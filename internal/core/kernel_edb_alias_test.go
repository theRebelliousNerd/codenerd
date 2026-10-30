package core

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"codenerd/internal/types"
)

// Two supported host representations can have different canonical keys and
// still admit to one store atom: under a narrow /string declaration a plain
// "/tmp" (key /tmp, a name-shaped string) and types.MangleString("/tmp") (key
// "/tmp", quoted) are the same String constant. The store is a set; the EDB
// held two rows. The cone evaluator's extensional delta removed the shared atom
// when one row was retracted, though the other still supported it, and only a
// full evaluation put it back. A store atom must stay while any retained EDB
// row admits to it.
const aliasPolicy = `
Decl alias_probe(X) bound [/string].
Decl alias_visible(X) bound [/string].
alias_visible(X) :- alias_probe(X).
`

func aliasKernel(t *testing.T, facts ...Fact) *RealKernel {
	t.Helper()
	k := newRealKernelBase()
	k.AppendPolicy(aliasPolicy)
	if err := k.Evaluate(); err != nil {
		t.Fatal(err)
	}
	for _, f := range facts {
		if err := k.Assert(f); err != nil {
			t.Fatal(err)
		}
	}
	// Settle: the writes above are folded into one evaluation, so the retraction
	// under test is the first delta against an evaluated store.
	if _, err := k.Query("alias_visible"); err != nil {
		t.Fatal(err)
	}
	return k
}

func aliasAnswers(t *testing.T, k *RealKernel) string {
	t.Helper()
	var out []string
	for _, pred := range []string{"alias_probe", "alias_visible"} {
		rows, err := k.Query(pred)
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range rows {
			out = append(out, pred+":"+types.ExtractString(f.Args[0]))
		}
	}
	sort.Strings(out)
	return strings.Join(out, " ")
}

// fullAnswers is the reference: the same EDB evaluated from scratch.
func fullAnswers(t *testing.T, k *RealKernel) string {
	t.Helper()
	if err := k.Evaluate(); err != nil {
		t.Fatal(err)
	}
	return aliasAnswers(t, k)
}

var (
	aliasPlain  = Fact{Predicate: "alias_probe", Args: []any{"/tmp"}}
	aliasString = Fact{Predicate: "alias_probe", Args: []any{types.MangleString("/tmp")}}
)

func TestExtensionalDelta_RemovingOneAliasKeepsTheAtomTheOtherSupports(t *testing.T) {
	for _, tc := range []struct {
		name          string
		first, second Fact
		retract       Fact
	}{
		{"retract the plain string", aliasPlain, aliasString, aliasPlain},
		{"retract the explicit string", aliasPlain, aliasString, aliasString},
		{"other insertion order", aliasString, aliasPlain, aliasPlain},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k := aliasKernel(t, tc.first, tc.second)
			if err := k.RetractExactFact(tc.retract); err != nil {
				t.Fatal(err)
			}
			got := aliasAnswers(t, k)
			if mode := k.LastEvaluation().Mode; mode != "cone" {
				t.Fatalf("evaluation mode %q, want cone: the witness is the delta path", mode)
			}
			if !strings.Contains(got, "alias_probe:/tmp") || !strings.Contains(got, "alias_visible:/tmp") {
				t.Fatalf("a retained EDB row still supports the atom, but the cone answer is %q", got)
			}
			if want := fullAnswers(t, k); got != want {
				t.Fatalf("cone and full evaluation disagree:\n cone: %s\n full: %s", got, want)
			}
		})
	}
}

func TestExtensionalDelta_RemovingBothAliasesRemovesTheAtom(t *testing.T) {
	k := aliasKernel(t, aliasPlain, aliasString)
	if err := k.RetractExactFact(aliasPlain); err != nil {
		t.Fatal(err)
	}
	if err := k.RetractExactFact(aliasString); err != nil {
		t.Fatal(err)
	}
	if got := aliasAnswers(t, k); got != "" {
		t.Fatalf("no support remains, but the cone answer is %q", got)
	}
}

func TestExtensionalDelta_RetractAndReassertBeforeEvaluation(t *testing.T) {
	k := aliasKernel(t, aliasPlain, aliasString)
	if err := k.RetractExactFact(aliasPlain); err != nil {
		t.Fatal(err)
	}
	if err := k.Assert(aliasPlain); err != nil {
		t.Fatal(err)
	}
	got := aliasAnswers(t, k)
	if want := fullAnswers(t, k); got != want || !strings.Contains(got, "alias_visible") {
		t.Fatalf("cone %q vs full %q", got, want)
	}
}

// A plain string and an unrelated value are distinct atoms: removing one never
// removes the other, and an ordinary single-representation predicate keeps its
// delta (no scan, no extra work: the aliasRows guard is zero).
func TestExtensionalDelta_OrdinaryRowsAreNotScanned(t *testing.T) {
	k := aliasKernel(t,
		Fact{Predicate: "alias_probe", Args: []any{"/a"}},
		Fact{Predicate: "alias_probe", Args: []any{"/b"}},
	)
	if err := k.RetractExactFact(Fact{Predicate: "alias_probe", Args: []any{"/a"}}); err != nil {
		t.Fatal(err)
	}
	got := aliasAnswers(t, k)
	if strings.Contains(got, ":/a") || !strings.Contains(got, "alias_visible:/b") {
		t.Fatalf("ordinary retraction: %q", got)
	}
	k.mu.RLock()
	rows := k.factIndex.aliasRows["alias_probe"]
	k.mu.RUnlock()
	if rows != 0 {
		t.Fatalf("a predicate with only plain rows carries %d alias-capable rows; its delta must stay O(delta)", rows)
	}
}

// The batch retraction the world scan uses takes the same delta path.
func TestExtensionalDelta_BatchRetractionKeepsSharedSupport(t *testing.T) {
	k := aliasKernel(t, aliasPlain, aliasString)
	if err := k.RetractExactFactsBatch([]Fact{aliasPlain}); err != nil {
		t.Fatal(err)
	}
	got := aliasAnswers(t, k)
	if want := fullAnswers(t, k); got != want || !strings.Contains(got, "alias_visible:/tmp") {
		t.Fatalf("batch: cone %q vs full %q", got, want)
	}
}

// A clone has its own delta and alias count: retracting in the clone leaves the
// parent's answers and support alone, and the clone's answer is the full one.
func TestExtensionalDelta_CloneKeepsItsOwnSupport(t *testing.T) {
	k := aliasKernel(t, aliasPlain, aliasString)
	c := k.Clone()
	if err := c.RetractExactFact(aliasPlain); err != nil {
		t.Fatal(err)
	}
	if got := aliasAnswers(t, c); !strings.Contains(got, "alias_visible:/tmp") {
		t.Fatalf("clone lost the atom its other row supports: %q", got)
	}
	if got := aliasAnswers(t, k); !strings.Contains(got, "alias_visible:/tmp") {
		t.Fatalf("parent changed by a clone's retraction: %q", got)
	}
	c.mu.RLock()
	rows := c.factIndex.aliasRows["alias_probe"]
	c.mu.RUnlock()
	k.mu.RLock()
	parent := k.factIndex.aliasRows["alias_probe"]
	k.mu.RUnlock()
	if rows != 1 || parent != 1 {
		t.Fatalf("alias-capable rows: clone %d, parent %d, want 1 and 1", rows, parent)
	}
}

// What the guard costs: one retraction from a large predicate. The ordinary
// predicate stays O(delta); the alias-capable one pays one pass over its rows.
func benchmarkRetract(b *testing.B, rows int, aliasCapable bool) {
	k := newRealKernelBase()
	k.AppendPolicy(aliasPolicy)
	if err := k.Evaluate(); err != nil {
		b.Fatal(err)
	}
	facts := make([]Fact, 0, rows)
	for i := 0; i < rows; i++ {
		facts = append(facts, Fact{Predicate: "alias_probe", Args: []any{fmt.Sprintf("/p/%d", i)}})
	}
	if aliasCapable {
		facts = append(facts, aliasString)
	}
	if err := k.AssertBatch(facts); err != nil {
		b.Fatal(err)
	}
	if _, err := k.Query("alias_visible"); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f := facts[i%rows]
		if err := k.RetractExactFact(f); err != nil {
			b.Fatal(err)
		}
		if _, err := k.Query("alias_visible"); err != nil {
			b.Fatal(err)
		}
		b.StopTimer()
		if err := k.Assert(f); err != nil {
			b.Fatal(err)
		}
		if _, err := k.Query("alias_visible"); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
	}
}

func BenchmarkExtensionalDelta_Ordinary20k(b *testing.B)     { benchmarkRetract(b, 20000, false) }
func BenchmarkExtensionalDelta_AliasCapable20k(b *testing.B) { benchmarkRetract(b, 20000, true) }
