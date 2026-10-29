package mangle

import (
	"fmt"
	"sort"
	"testing"
)

// The merge comparator reads rank, an EDB relation no lattice rule body
// names. Replacing rank changes which row winner keeps, so winner and what
// was concluded from it (chosen) are downstream of rank through the merge
// Decl. The generic engine's invalidation graph only followed rule bodies:
// a rank-only replacement cleared {rank, better}, the lattice swapped its
// row, and chosen kept the old winner beside the new one (audit
// mangle-generic-merge-invalidation).
const rankedLattice = `
Decl candidate(K, O) bound [/name, /name].
Decl rank(O, R) bound [/name, /number].
Decl winner(K, O)
    descr [fundep([K], [O]), merge([O], "better")]
    bound [/name, /name].
Decl better(A, B, Best)
    descr [mode('+', '+', '-'), deferred()]
    bound [/name, /name, /name].
Decl chosen(K, O) bound [/name, /name].

winner(K, O) :- candidate(K, O).
better(A, B, A) :- rank(A, RA), rank(B, RB), RA <= RB.
better(A, B, B) :- rank(A, RA), rank(B, RB), RB < RA.
chosen(K, O) :- winner(K, O).
`

func rankedEngine(t *testing.T, autoEval bool, ranks []Fact) *Engine {
	t.Helper()
	cfg := DefaultConfig()
	cfg.AutoEval = autoEval
	eng, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := eng.LoadSchemaString(rankedLattice); err != nil {
		t.Fatal(err)
	}
	base := append([]Fact{
		{Predicate: "candidate", Args: []any{"/k", "/a"}},
		{Predicate: "candidate", Args: []any{"/k", "/b"}},
	}, ranks...)
	if err := eng.AddFacts(base); err != nil {
		t.Fatal(err)
	}
	if err := eng.Evaluate(); err != nil {
		t.Fatal(err)
	}
	return eng
}

func materialized(t *testing.T, eng *Engine, preds ...string) []string {
	t.Helper()
	var out []string
	for _, p := range preds {
		rows, err := eng.GetFacts(p)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range rows {
			out = append(out, fmt.Sprintf("%s%v", r.Predicate, r.Args))
		}
	}
	sort.Strings(out)
	return out
}

func TestEngine_ComparatorOnlyReplacementRederivesTheLatticeAndItsConsumers(t *testing.T) {
	first := []Fact{
		{Predicate: "rank", Args: []any{"/a", int64(1)}},
		{Predicate: "rank", Args: []any{"/b", int64(2)}},
	}
	second := []Fact{
		{Predicate: "rank", Args: []any{"/a", int64(3)}},
		{Predicate: "rank", Args: []any{"/b", int64(2)}},
	}
	for _, autoEval := range []bool{false, true} {
		t.Run(fmt.Sprintf("autoEval=%v", autoEval), func(t *testing.T) {
			eng := rankedEngine(t, autoEval, first)
			if got := materialized(t, eng, "winner", "chosen"); fmt.Sprint(got) != "[chosen[/k /a] winner[/k /a]]" {
				t.Fatalf("first epoch = %v, want /a winning", got)
			}
			if err := eng.ReplaceControlFacts(second, "rank"); err != nil {
				t.Fatal(err)
			}
			if err := eng.Evaluate(); err != nil {
				t.Fatal(err)
			}
			got := materialized(t, eng, "winner", "chosen")
			fresh := materialized(t, rankedEngine(t, autoEval, second), "winner", "chosen")
			if fmt.Sprint(got) != fmt.Sprint(fresh) {
				t.Fatalf("after replacing only rank: %v, a fresh engine over the same base facts: %v", got, fresh)
			}
		})
	}
}

// The graph edge itself: the merge predicate feeds its lattice head.
func TestBuildRuleGraph_MergePredicateFeedsItsLattice(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	eng, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := eng.LoadSchemaString(rankedLattice); err != nil {
		t.Fatal(err)
	}
	eng.mu.RLock()
	down := eng.rules.downstream(map[string]struct{}{"rank": {}})
	eng.mu.RUnlock()
	for _, want := range []string{"better", "winner", "chosen"} {
		if _, ok := down[want]; !ok {
			t.Errorf("downstream(rank) = %v, missing %s", down, want)
		}
	}
}
