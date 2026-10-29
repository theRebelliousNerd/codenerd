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
	return newPolicyKernel(t, setupMockKernel(t), conePolicy)
}

// newPolicyKernel appends policy to k and evaluates it.
func newPolicyKernel(t *testing.T, k *RealKernel, policy string) *RealKernel {
	t.Helper()
	k.AppendPolicy(policy)
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	return k
}

// differentialKernels names the two programs a differential walk runs on.
// The walk writes and compares only its own predicates, which read nothing of
// the production corpus, so most seeds run on the policy alone; one runs
// with the whole corpus loaded, where the cone is cut from the real
// program's dependency index and layers. Every full evaluation of the corpus
// is a real fixpoint (18 ms), which was the whole cost of these tests.
func differentialKernels(t *testing.T, policy string, withCorpus bool) (lazy, full *RealKernel) {
	t.Helper()
	if withCorpus {
		return newPolicyKernel(t, setupMockKernel(t), policy), newPolicyKernel(t, setupMockKernel(t), policy)
	}
	return newPolicyKernel(t, newRealKernelBase(), policy), newPolicyKernel(t, newRealKernelBase(), policy)
}

// TestConeEvaluationMatchesFullEvaluation is the differential check: one
// kernel answers every query lazily, so its writes are re-derived by cone; the
// other is forced through the full fixpoint after the same write. After every
// one of several hundred random asserts and retracts the two must agree on
// every predicate.
func TestConeEvaluationMatchesFullEvaluation(t *testing.T) {
	for _, run := range []struct {
		seed   int64
		corpus bool
	}{{1, false}, {7, false}, {20260921, false}, {20260921, true}} {
		seed := run.seed
		t.Run(fmt.Sprintf("seed-%d-corpus-%v", seed, run.corpus), func(t *testing.T) {
			rng := rand.New(rand.NewSource(seed))
			lazy, full := differentialKernels(t, conePolicy, run.corpus)
			ids := []string{"/a", "/b", "/c", "/d"}
			pick := func() string { return ids[rng.Intn(len(ids))] }
			unary := []string{"cn_raised", "cn_discharged", "cn_mixed", "cn_seed", "cn_inert", "cn_island_in"}

			cones := 0
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
				if lazy.LastEvaluation().Mode == "cone" {
					cones++
				}
			}
			if cones == 0 {
				t.Fatal("no lazy evaluation took the cone path: the differential passed vacuously")
			}
		})
	}
}

// TestConeEvaluationIsActuallyTaken guards the test above against passing
// vacuously: if every lazy evaluation fell back to the full path, the two
// kernels would agree for the wrong reason.
func TestConeEvaluationIsActuallyTaken(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

// latticePolicy is the shape the cone planner once missed: a merge lattice
// whose merge predicate reads a ranking table, and a rule whose premise is a
// deferred predicate. Both are evaluated top-down, so their rules must reach
// the engine with the cone that reads them (coneIndex.topDown); without them
// every lattice row was kept as incomparable and the deferred premise had no
// answer, on the cone path only.
const latticePolicy = `
	Decl cl_score(Key, Option, Score).
	Decl cl_option(Option).
	Decl cl_promoted(Option).
	Decl cl_rank(Option, Rank).
	Decl cl_best(Key, Best)
		descr [fundep([Key], [Best]), merge([Best], "cl_better")].
	Decl cl_better(A, B, Best) descr [mode('+', '+', '-'), deferred()].
	Decl cl_outranks(A, B) descr [mode('+', '+'), deferred()].
	Decl cl_pick(Key, Option).
	Decl cl_flagged(Option).
	Decl cl_ranked_pair(A, B).

	cl_option(/a). cl_option(/b). cl_option(/c).
	# One rank per option, so the merge is a total preorder (a lattice): a
	# promoted option outranks the rest, and two promoted ones tie.
	cl_rank(O, 0) :- cl_option(O), cl_promoted(O).
	cl_rank(O, 1) :- cl_option(O), !cl_promoted(O).

	# The body names cl_rank, which the merge reads: analysis orders strata by
	# rule bodies only, never by a merge predicate, so without it cl_best can
	# run before cl_rank exists and keep every tied row (mangle-go v0.7).
	cl_best(K, B) :- cl_score(K, O, S), cl_rank(O, _), B = fn:pair(O, S).
	cl_better(A, B, A) :- :match_pair(A, OA, SA), :match_pair(B, OB, SB), SA > SB.
	cl_better(A, B, B) :- :match_pair(A, OA, SA), :match_pair(B, OB, SB), SB > SA.
	cl_better(A, B, A) :- :match_pair(A, OA, SA), :match_pair(B, OB, SB), SA = SB, cl_rank(OA, RA), cl_rank(OB, RB), RA < RB.
	cl_better(A, B, B) :- :match_pair(A, OA, SA), :match_pair(B, OB, SB), SA = SB, cl_rank(OA, RA), cl_rank(OB, RB), RB < RA.
	cl_pick(K, O) :- cl_best(K, B), :match_pair(B, O, S).

	cl_outranks(A, B) :- cl_rank(A, RA), cl_rank(B, RB), RA < RB.
	cl_ranked_pair(A, B) :- cl_flagged(A), cl_flagged(B), cl_outranks(A, B).
`

var latticePredicates = []string{"cl_score", "cl_promoted", "cl_rank", "cl_best", "cl_pick", "cl_flagged", "cl_ranked_pair"}

func latticeSnapshot(t *testing.T, k *RealKernel) string {
	t.Helper()
	var lines []string
	for _, pred := range latticePredicates {
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

func TestConeEvaluationMatchesFullEvaluation_LatticeAndDeferred(t *testing.T) {
	for _, run := range []struct {
		seed   int64
		corpus bool
	}{{3, false}, {20261028, false}, {20261028, true}} {
		seed := run.seed
		t.Run(fmt.Sprintf("seed-%d-corpus-%v", seed, run.corpus), func(t *testing.T) {
			rng := rand.New(rand.NewSource(seed))
			lazy, full := differentialKernels(t, latticePolicy, run.corpus)
			keys := []string{"/k1", "/k2"}
			options := []string{"/a", "/b", "/c"}
			pick := func(xs []string) string { return xs[rng.Intn(len(xs))] }
			var lattices int
			for step := 0; step < 60; step++ {
				var fact Fact
				switch rng.Intn(3) {
				case 0:
					fact = Fact{Predicate: "cl_score", Args: []any{pick(keys), pick(options), int64(rng.Intn(4))}}
				case 1:
					fact = Fact{Predicate: "cl_promoted", Args: []any{pick(options)}}
				default:
					fact = Fact{Predicate: "cl_flagged", Args: []any{pick(options)}}
				}
				apply := func(k *RealKernel) error { return k.Assert(fact) }
				if rng.Intn(3) == 0 {
					apply = func(k *RealKernel) error { return k.RetractExactFact(fact) }
				}
				for _, k := range []*RealKernel{lazy, full} {
					if err := apply(k); err != nil {
						t.Fatalf("step %d %s: %v", step, fact.String(), err)
					}
				}
				if err := full.Evaluate(); err != nil {
					t.Fatalf("step %d: full Evaluate: %v", step, err)
				}
				got, want := latticeSnapshot(t, lazy), latticeSnapshot(t, full)
				if got != want {
					t.Fatalf("step %d, after %s: cone evaluation diverged from the full fixpoint\n--- cone\n%s\n--- full\n%s", step, fact.String(), got, want)
				}
				best, _ := full.Query("cl_best")
				scores, _ := full.Query("cl_score")
				if len(scores) > len(best) && len(best) > 0 {
					lattices++ // the lattice kept fewer rows than it was given
				}
			}
			if lattices == 0 {
				t.Fatal("the lattice never replaced a row: the differential passed vacuously")
			}
		})
	}
}
