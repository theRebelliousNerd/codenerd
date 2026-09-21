package core

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"testing"
)

// conePolicy exercises everything the S23 differential path got wrong, plus
// the shapes a cone has to respect: negation, aggregation, recursion, a
// predicate that is both asserted and derived, a program fact of a written
// predicate, and a predicate no rule reads.
const conePolicy = `
	Decl cn_edge(From, To).
	Decl cn_reach(From, To).
	Decl cn_raised(Id).
	Decl cn_discharged(Id).
	Decl cn_open(Id).
	Decl cn_open_count(N).
	Decl cn_busy().
	Decl cn_mixed(Id).
	Decl cn_seed(Id).
	Decl cn_downstream(Id).
	Decl cn_inert(Id).
	Decl cn_island_in(Id).
	Decl cn_island_out(Id).

	cn_reach(X, Y) :- cn_edge(X, Y).
	cn_reach(X, Z) :- cn_edge(X, Y), cn_reach(Y, Z).

	cn_open(X) :- cn_raised(X), !cn_discharged(X).
	cn_open_count(N) :- cn_open(_) |> do fn:group_by(), let N = fn:count().
	cn_busy() :- cn_open_count(N), N >= 2.

	cn_mixed(X) :- cn_seed(X).
	cn_downstream(X) :- cn_mixed(X), !cn_open(X).

	cn_seed(/program_fact).

	cn_island_out(X) :- cn_island_in(X).
`

var conePredicates = []string{
	"cn_edge", "cn_reach", "cn_raised", "cn_discharged", "cn_open", "cn_open_count", "cn_busy",
	"cn_mixed", "cn_seed", "cn_downstream", "cn_inert", "cn_island_in", "cn_island_out",
}

func coneSnapshot(t *testing.T, k *RealKernel) string {
	t.Helper()
	var lines []string
	for _, pred := range conePredicates {
		facts, err := k.Query(pred)
		if err != nil {
			t.Fatalf("Query %s: %v", pred, err)
		}
		for _, f := range facts {
			lines = append(lines, f.String())
		}
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

func newConeKernel(t *testing.T) *RealKernel {
	t.Helper()
	k := setupMockKernel(t)
	k.AppendPolicy(conePolicy)
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	return k
}

// TestConeEvaluationMatchesFullEvaluation is the differential check: one
// kernel answers every query lazily, so its writes are re-derived by cone; the
// other is forced through the full fixpoint after the same write. After every
// one of several hundred random asserts and retracts the two must agree on
// every predicate.
func TestConeEvaluationMatchesFullEvaluation(t *testing.T) {
	for _, seed := range []int64{1, 7, 20260921} {
		t.Run(fmt.Sprintf("seed-%d", seed), func(t *testing.T) {
			rng := rand.New(rand.NewSource(seed))
			lazy, full := newConeKernel(t), newConeKernel(t)
			ids := []string{"/a", "/b", "/c", "/d"}
			pick := func() string { return ids[rng.Intn(len(ids))] }
			unary := []string{"cn_raised", "cn_discharged", "cn_mixed", "cn_seed", "cn_inert", "cn_island_in"}

			for step := 0; step < 300; step++ {
				var fact Fact
				if rng.Intn(4) == 0 {
					fact = Fact{Predicate: "cn_edge", Args: []any{pick(), pick()}}
				} else {
					fact = Fact{Predicate: unary[rng.Intn(len(unary))], Args: []any{pick()}}
				}
				op := "assert"
				apply := func(k *RealKernel) error { return k.Assert(fact) }
				switch rng.Intn(5) {
				case 0:
					op = "retract-exact"
					apply = func(k *RealKernel) error { return k.RetractExactFact(fact) }
				case 1:
					op = "transaction"
				}
				// The transaction draws from rng, and both kernels must replay the
				// same draw: capture it once, outside the closure.
				if op == "transaction" {
					extra := Fact{Predicate: "cn_inert", Args: []any{pick()}}
					apply = func(k *RealKernel) error {
						tx := k.Transaction()
						tx.RetractExactFact(fact)
						tx.Assert(extra)
						return tx.Commit()
					}
				}
				if err := apply(lazy); err != nil {
					t.Fatalf("step %d %s %s on lazy: %v", step, op, fact.String(), err)
				}
				if err := apply(full); err != nil {
					t.Fatalf("step %d %s %s on full: %v", step, op, fact.String(), err)
				}
				if err := full.Evaluate(); err != nil {
					t.Fatalf("step %d: full Evaluate: %v", step, err)
				}
				if got, want := coneSnapshot(t, lazy), coneSnapshot(t, full); got != want {
					t.Fatalf("step %d, after %s %s: cone evaluation diverged from the full fixpoint\n--- cone\n%s\n--- full\n%s",
						step, op, fact.String(), got, want)
				}
			}
		})
	}
}

// TestConeEvaluationIsActuallyTaken guards the test above against passing
// vacuously: if every lazy evaluation fell back to the full path, the two
// kernels would agree for the wrong reason.
func TestConeEvaluationIsActuallyTaken(t *testing.T) {
	k := newConeKernel(t)
	if _, err := k.Query("cn_open"); err != nil {
		t.Fatalf("Query: %v", err)
	}
	before := k.store
	if err := k.Assert(Fact{Predicate: "cn_raised", Args: []any{"/a"}}); err != nil {
		t.Fatalf("Assert: %v", err)
	}
	open, err := k.Query("cn_open")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(open) != 1 {
		t.Fatalf("cn_open = %v, want exactly one fact", open)
	}
	if k.store != before {
		t.Fatalf("the write was evaluated on the full path (a fresh store was built), so the cone path is not being exercised")
	}
}

// TestConeLeavesUnrelatedPredicatesAlone pins the point of the exercise: a
// write re-derives its own cone and nothing else.
func TestConeLeavesUnrelatedPredicatesAlone(t *testing.T) {
	k := newConeKernel(t)
	k.mu.Lock()
	cone := k.cone.closure(map[string]struct{}{"cn_island_in": {}})
	inert := k.cone.closure(map[string]struct{}{"cn_inert": {}})
	k.mu.Unlock()
	if _, ok := cone["cn_island_out"]; !ok || len(cone) != 1 {
		t.Fatalf("cone of cn_island_in = %v, want exactly {cn_island_out}", cone)
	}
	if len(inert) != 0 {
		t.Fatalf("cone of a predicate no rule reads = %v, want empty", inert)
	}
}

// TestConeRestoresProgramFactOfWrittenPredicate: a fact written in the program
// text is removed with the rest of its predicate and must come back.
func TestConeRestoresProgramFactOfWrittenPredicate(t *testing.T) {
	k := newConeKernel(t)
	if err := k.Assert(Fact{Predicate: "cn_seed", Args: []any{"/a"}}); err != nil {
		t.Fatalf("Assert: %v", err)
	}
	mixed, err := k.Query("cn_mixed")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(mixed) != 2 {
		t.Fatalf("cn_mixed = %v, want the program fact's derivation and the asserted one", mixed)
	}
}
