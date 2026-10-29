package core

import (
	"strings"
	"testing"

	"codenerd/internal/types"
)

// A nil *RealKernel stored in an interface is a non-nil interface, so a
// caller's nil check does not fire. Every types.Kernel method must return an
// error through that trap instead of panicking (which would take the agent
// down from a fail-closed safety check).
func TestNilKernel_InterfaceMethodsError(t *testing.T) {
	var k *RealKernel
	var iface types.Kernel = k
	if iface == nil {
		t.Fatal("test setup broken: typed nil must be a non-nil interface")
	}
	cases := []struct {
		name string
		call func() error
	}{
		{"Query", func() error { _, err := iface.Query("p"); return err }},
		{"QueryAll", func() error { _, err := iface.QueryAll(); return err }},
		{"Assert", func() error { return iface.Assert(Fact{Predicate: "p"}) }},
		{"AssertBatch", func() error { return iface.AssertBatch(nil) }},
		{"LoadFacts", func() error { return iface.LoadFacts(nil) }},
		{"Retract", func() error { return iface.Retract("p") }},
		{"RetractFact", func() error { return iface.RetractFact(Fact{Predicate: "p"}) }},
		{"RetractExactFactsBatch", func() error { return iface.RetractExactFactsBatch(nil) }},
		{"RemoveFactsByPredicateSet", func() error { return iface.RemoveFactsByPredicateSet(nil) }},
		{"UpdateSystemFacts", func() error { return iface.UpdateSystemFacts() }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panicked on nil kernel: %v", r)
				}
			}()
			if err := tc.call(); err == nil {
				t.Fatal("expected kernel-is-nil error, got nil")
			} else if !strings.Contains(err.Error(), "kernel is nil") {
				t.Fatalf("error does not name the problem: %v", err)
			}
		})
	}
	// Same-file siblings with the same trap shape.
	t.Run("QueryCallback", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("panicked on nil kernel: %v", r)
			}
		}()
		if err := k.QueryCallback("p", func(Fact) error { return nil }); err == nil {
			t.Fatal("expected kernel-is-nil error, got nil")
		}
	})
	t.Run("GetDerivedFacts", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("panicked on nil kernel: %v", r)
			}
		}()
		if _, err := k.GetDerivedFacts(); err == nil {
			t.Fatal("expected kernel-is-nil error, got nil")
		}
	})
}

// Void and pointer-returning members cannot carry an error; they must still
// not panic. Voids log loudly and no-op; getters return nil.
func TestNilKernel_VoidAndGetterMethodsSafe(t *testing.T) {
	var k *RealKernel
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panicked on nil kernel: %v", r)
		}
	}()
	k.Reset()
	k.AppendPolicy("Decl dropped(X).")
	if k.GetProgramInfo() != nil {
		t.Fatal("GetProgramInfo on nil kernel should be nil")
	}
	if k.GetBaseFacts() != nil {
		t.Fatal("GetBaseFacts on nil kernel should be nil")
	}
}

// Fractional patterns must not match their integer neighbors: 3.14 is not 3.
// The normalizer used to truncate every float, so Query("p(3.14)") matched
// stored p(3) and vice versa.
func TestQueryPattern_FloatPrecision(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("boot: %v", err)
	}
	const pred = "uplift_float_probe"
	k.AppendPolicy("Decl uplift_float_probe(X).")
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	for _, v := range []any{3.14, int64(3), 3.0} {
		if err := k.Assert(Fact{Predicate: pred, Args: []any{v}}); err != nil {
			t.Fatalf("Assert %v: %v", v, err)
		}
	}
	// 3.0 folds to 3 (dedup), so exactly two distinct rows exist.
	rows, err := k.Query(pred)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 distinct rows (3.14 and 3), got %d", len(rows))
	}
	frac, err := k.Query(pred + "(3.14)")
	if err != nil {
		t.Fatalf("Query 3.14: %v", err)
	}
	if len(frac) != 1 {
		t.Fatalf("Query(3.14) matched %d rows, want exactly 1", len(frac))
	}
	whole, err := k.Query(pred + "(3)")
	if err != nil {
		t.Fatalf("Query 3: %v", err)
	}
	if len(whole) != 1 {
		t.Fatalf("Query(3) matched %d rows, want exactly 1", len(whole))
	}
}

// In a directory that is not a git checkout, UpdateSystemFacts must assert
// the clock but no git facts — never a fabricated unstaged_count("0").
func TestUpdateSystemFacts_NonGitDir(t *testing.T) {
	k, err := NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("boot: %v", err)
	}
	if err := k.UpdateSystemFacts(); err != nil {
		t.Fatalf("UpdateSystemFacts: %v", err)
	}
	gitRows, err := k.Query("git_state")
	if err != nil {
		t.Fatalf("Query git_state: %v", err)
	}
	if len(gitRows) != 0 {
		t.Fatalf("asserted %d git_state facts without a repo: %v", len(gitRows), gitRows)
	}
	clockRows, err := k.Query("current_time")
	if err != nil {
		t.Fatalf("Query current_time: %v", err)
	}
	if len(clockRows) != 1 {
		t.Fatalf("want exactly 1 current_time fact, got %d", len(clockRows))
	}
}

// parseQueryPattern unit pins: bare names, patterns, and the historical
// fall back to predicate-only on unparseable input.
func TestParseQueryPattern(t *testing.T) {
	name, _, has, arity := parseQueryPattern("user_intent")
	if name != "user_intent" || has || arity != 0 {
		t.Fatalf("bare name: got %q %v %d", name, has, arity)
	}
	name, pat, has, arity := parseQueryPattern(`next_action(/generate_tool)`)
	if name != "next_action" || !has || arity != 1 {
		t.Fatalf("pattern: got %q %v %d", name, has, arity)
	}
	if len(pat.Args) != 1 || pat.Args[0] != "/generate_tool" {
		t.Fatalf("pattern args = %v", pat.Args)
	}
	name, _, has, _ = parseQueryPattern("broken_pred(")
	if name != "broken_pred" || has {
		t.Fatalf("unparseable: got %q %v, want predicate-only fallback", name, has)
	}
}
