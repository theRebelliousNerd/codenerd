package core

import (
	"sort"
	"strings"
	"testing"

	"codeberg.org/TauCeti/mangle-go/analysis"
	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/engine"
	"codeberg.org/TauCeti/mangle-go/factstore"
)

func storeDump(s factstore.FactStore) string {
	var lines []string
	_ = factstore.GetAllFacts(s, func(a ast.Atom) error {
		lines = append(lines, a.String())
		return nil
	})
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// The kernel evaluates the production corpus in layers of independent
// components (mangle.Stratify), not one stratum per component: the engine's
// per-call cost grows with strata x predicates, and the corpus has 1,315
// components. A full fixpoint over the layers derives exactly what the
// component strata analysis.Stratify returns derive.
func TestProductionCorpusEvaluatesInLayers(t *testing.T) {
	k := setupMockKernel(t)
	for _, f := range []Fact{
		{Predicate: "user_intent", Args: []any{"/current_intent", "/mutation", "/fix", "x.go", ""}},
		{Predicate: "file_topology", Args: []any{"x.go", "h1", "/go", int64(1), "/false"}},
	} {
		if err := k.Assert(f); err != nil {
			t.Fatal(err)
		}
	}
	if err := k.Evaluate(); err != nil {
		t.Fatal(err)
	}
	k.mu.RLock()
	defer k.mu.RUnlock()

	components, componentOf, err := analysis.Stratify(analysis.Program{
		EdbPredicates: k.programInfo.EdbPredicates,
		IdbPredicates: k.programInfo.IdbPredicates,
		Rules:         k.programInfo.Rules,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(k.strata)*10 > len(components) {
		t.Fatalf("the kernel evaluates %d strata for %d components: they were not layered", len(k.strata), len(components))
	}
	reference := factstore.NewSimpleInMemoryStore()
	for _, atom := range k.cachedAtoms {
		reference.Add(atom)
	}
	if _, err := engine.EvalStratifiedProgramWithStats(k.programInfo, components, componentOf, reference,
		engine.WithCreatedFactLimit(k.effectiveDerivedFactLimitLocked())); err != nil {
		t.Fatal(err)
	}
	got, want := storeDump(k.store), storeDump(reference)
	if got != want {
		t.Fatalf("layered evaluation diverged from the component strata: %d vs %d lines",
			strings.Count(got, "\n"), strings.Count(want, "\n"))
	}
	if strings.Count(want, "\n") < 1000 {
		t.Fatalf("the reference derived only %d facts: the comparison is not exercising the corpus", strings.Count(want, "\n"))
	}
}
