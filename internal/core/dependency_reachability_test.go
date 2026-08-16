package core

import (
	"fmt"
	"testing"

	"codenerd/internal/types"
)

// denseImportGraph builds a layered file->file import graph of the shape real
// resolution produces: package-level fan-out, every file of a layer importing
// every file of the next. layers*width*width edges.
func denseImportGraph(layers, width int) []types.Fact {
	facts := make([]types.Fact, 0, layers*width*width)
	for p := range layers {
		for a := range width {
			for b := range width {
				facts = append(facts, types.Fact{
					Predicate: "dependency_link",
					Args: []any{
						fmt.Sprintf("p%d/f%d.go", p, a),
						fmt.Sprintf("p%d/f%d.go", p+1, b),
						types.MangleAtom("/import"),
					},
				})
			}
		}
	}
	return facts
}

// TestDependencyReachability_WhenGraphIsDense_ShouldNotKillTheKernel is the
// regression test for a whole-program failure.
//
// symbol_reachable/2 and path_of_length/3 were eager closures over
// dependency_link. Resolved import edges are dense — package-level fan-out gives
// this repository's own tree ~33,000 file->file edges from ~2,100 import
// statements — and an unbounded transitive closure plus a depth-15 path
// enumeration over that exhausted the kernel's 500,000 derived-fact ceiling.
//
// The failure is not confined to the predicate that overflows. Once the ceiling
// trips, evaluation of the ENTIRE program fails, so every unrelated query
// returns zero rows. On this repo, `nerd query safe_action` went from 120 rows
// to an error. That makes it a whole-product outage triggered by nothing worse
// than opening a normal-sized codebase.
//
// safe_action is the canary on purpose: it has nothing to do with dependency
// links, which is exactly why its collapse is the thing worth asserting.
func TestDependencyReachability_WhenGraphIsDense_ShouldNotKillTheKernel(t *testing.T) {
	k, err := NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("NewRealKernelWithWorkspace: %v", err)
	}

	facts := denseImportGraph(40, 25) // 25,000 edges; the real repo produces ~33,000
	if err := k.AssertFactBatch(facts); err != nil {
		t.Fatalf("asserting %d dependency_link facts: %v", len(facts), err)
	}

	rows, err := k.Query("safe_action(A)")
	if err != nil {
		t.Fatalf("a dense import graph broke whole-program evaluation: %v\n"+
			"Every query in the kernel fails when the derived-fact ceiling trips, "+
			"not just the one that overflowed.", err)
	}
	if len(rows) != 120 {
		t.Errorf("safe_action = %d rows, want 120 — the constitution stopped deriving "+
			"under an unrelated fact load", len(rows))
	}
}

// TestDependencyReachability_WhenUnseeded_ShouldDeriveNothing pins the mechanism
// that makes the above affordable: the closures are demand-driven. Without a
// reachability_query seed they cost nothing, which is what lets a dense graph
// sit in the EDB harmlessly.
func TestDependencyReachability_WhenUnseeded_ShouldDeriveNothing(t *testing.T) {
	k, err := NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("NewRealKernelWithWorkspace: %v", err)
	}
	if err := k.AssertFactBatch(denseImportGraph(10, 10)); err != nil {
		t.Fatalf("assert: %v", err)
	}

	for _, q := range []string{"symbol_reachable(F, T)", "path_of_length(F, T, L)", "symbol_reachable_safe(F, T)"} {
		rows, err := k.Query(q)
		if err != nil {
			t.Fatalf("Query(%s): %v", q, err)
		}
		if len(rows) != 0 {
			t.Errorf("Query(%s) = %d rows with no reachability_query asserted; "+
				"an eager closure over import edges is what took the kernel down", q, len(rows))
		}
	}
}

// TestDependencyReachability_WhenSeeded_ShouldAnswer is the half that stops the
// fix from being a silent removal. Making a closure derive nothing is trivial
// and would have passed both tests above; the capability has to still work.
func TestDependencyReachability_WhenSeeded_ShouldAnswer(t *testing.T) {
	k, err := NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("NewRealKernelWithWorkspace: %v", err)
	}

	// A simple chain: a.go -> b.go -> c.go -> d.go.
	chain := []types.Fact{
		{Predicate: "dependency_link", Args: []any{"a.go", "b.go", types.MangleAtom("/import")}},
		{Predicate: "dependency_link", Args: []any{"b.go", "c.go", types.MangleAtom("/import")}},
		{Predicate: "dependency_link", Args: []any{"c.go", "d.go", types.MangleAtom("/import")}},
		{Predicate: "dependency_link", Args: []any{"unrelated.go", "other.go", types.MangleAtom("/import")}},
	}
	if err := k.AssertFactBatch(chain); err != nil {
		t.Fatalf("assert chain: %v", err)
	}
	if err := k.AssertFact(types.Fact{Predicate: "reachability_query", Args: []any{"a.go"}}); err != nil {
		t.Fatalf("assert seed: %v", err)
	}

	rows, err := k.Query("symbol_reachable(F, T)")
	if err != nil {
		t.Fatalf("Query(symbol_reachable): %v", err)
	}
	// From a.go the closure must reach b, c and d — transitively, not just the
	// direct edge — and must not wander into the unseeded component.
	if len(rows) != 3 {
		t.Errorf("symbol_reachable from the seed = %d rows, want 3 (b, c, d): %v", len(rows), rows)
	}

	bounded, err := k.Query("symbol_reachable_safe(F, T)")
	if err != nil {
		t.Fatalf("Query(symbol_reachable_safe): %v", err)
	}
	if len(bounded) != 3 {
		t.Errorf("symbol_reachable_safe from the seed = %d rows, want 3: %v", len(bounded), bounded)
	}

	// The unseeded component must stay invisible; a seed that does not actually
	// bound the search is the same eager closure with extra steps.
	for _, r := range rows {
		if len(r.Args) > 0 && fmt.Sprint(r.Args[0]) == "unrelated.go" {
			t.Errorf("the seed did not bound the search; unrelated.go appeared: %v", r)
		}
	}
}
