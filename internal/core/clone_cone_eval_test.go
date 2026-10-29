package core

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"codeberg.org/TauCeti/mangle-go/ast"
)

// dumpStore renders every atom of a kernel's store, sorted, for comparison.
func dumpStore(t *testing.T, k *RealKernel) []string {
	t.Helper()
	k.mu.RLock()
	defer k.mu.RUnlock()
	var out []string
	for _, sym := range k.store.ListPredicates() {
		if err := k.store.GetFacts(ast.NewQuery(sym), func(a ast.Atom) error {
			out = append(out, a.String())
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	sort.Strings(out)
	return out
}

// A clone of an evaluated kernel re-derives only the cone of what is staged
// on it. That must hold exactly the facts a full fixpoint holds: a dream
// safety verdict is read off this store.
func TestCloneConeEvaluationMatchesFullFixpoint(t *testing.T) {
	d, k := setupTestDreamer(t)
	var seed []Fact
	for i := 0; i < 40; i++ {
		seed = append(seed,
			Fact{Predicate: "code_defines", Args: []any{fmt.Sprintf("internal/core/f%d.go", i), fmt.Sprintf("Sym%d", i), MangleAtom("/function"), int64(i), int64(i + 5)}},
			Fact{Predicate: "code_calls", Args: []any{fmt.Sprintf("Sym%d", i), fmt.Sprintf("Sym%d", (i+1)%40)}},
		)
	}
	if err := k.LoadFacts(seed); err != nil {
		t.Fatalf("LoadFacts: %v", err)
	}
	if _, err := k.Query("panic_state"); err != nil {
		t.Fatalf("evaluate parent: %v", err)
	}

	for _, req := range []ActionRequest{
		{Type: ActionEditFile, Target: "internal/core/f3.go"},
		{Type: ActionDeleteFile, Target: "internal/core/f7.go"},
		{Type: ActionExecCmd, Target: "rm -rf internal/core"},
	} {
		projected := d.projectEffects(k, "dream:test", req)

		cone := k.Clone()
		full := k.Clone()
		for _, f := range projected {
			if err := cone.AssertWithoutEval(f); err != nil {
				t.Fatalf("stage on cone clone: %v", err)
			}
			if err := full.AssertWithoutEval(f); err != nil {
				t.Fatalf("stage on full clone: %v", err)
			}
		}
		if err := cone.EvaluatePending(); err != nil {
			t.Fatalf("EvaluatePending: %v", err)
		}
		if err := full.Evaluate(); err != nil {
			t.Fatalf("Evaluate: %v", err)
		}
		got, want := dumpStore(t, cone), dumpStore(t, full)
		if len(got) != len(want) {
			t.Fatalf("%s: cone clone holds %d facts, full fixpoint %d", req.Type, len(got), len(want))
		}
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("%s: first difference at %d: cone %q, full %q", req.Type, i, got[i], want[i])
			}
		}
	}
}

// The point of the cone path, measured: a clone's staged change costs far
// less than a full fixpoint of the same clone.
func TestCloneConeEvaluationIsCheaperThanFullFixpoint(t *testing.T) {
	d, k := setupTestDreamer(t)
	if _, err := k.Query("panic_state"); err != nil {
		t.Fatalf("evaluate parent: %v", err)
	}
	projected := d.projectEffects(k, "dream:test", ActionRequest{Type: ActionEditFile, Target: "internal/core/x.go"})
	timeIt := func(eval func(*RealKernel) error) time.Duration {
		c := k.Clone()
		for _, f := range projected {
			if err := c.AssertWithoutEval(f); err != nil {
				t.Fatal(err)
			}
		}
		start := time.Now()
		if err := eval(c); err != nil {
			t.Fatal(err)
		}
		return time.Since(start)
	}
	cone := timeIt((*RealKernel).EvaluatePending)
	full := timeIt((*RealKernel).Evaluate)
	t.Logf("staged-change evaluation: cone %v, full fixpoint %v", cone, full)
	if cone >= full {
		t.Errorf("cone evaluation (%v) is not cheaper than the full fixpoint (%v)", cone, full)
	}
}
