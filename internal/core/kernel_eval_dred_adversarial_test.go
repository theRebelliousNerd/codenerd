package core

import (
	"fmt"
	"math/rand"
	"testing"
)

// axPolicy stresses the places DRed's soundness argument leans on: a predicate
// both asserted and derived and negated from above, program facts of derived
// and of underived predicates, a program fact beside an aggregation, a
// recursion through a negation boundary, and numeric atoms.
const axPolicy = `
	Decl ax_src(X).
	Decl ax_cand(X).
	Decl ax_blk(X).
	Decl ax_e(X, Y).
	Decl ax_u(X).
	Decl ax_d(X).
	Decl ax_open(X).
	Decl ax_reach(X, Y).
	Decl ax_iso(X).
	Decl ax_has_out(X).
	Decl ax_ucand(X).
	Decl ax_cnt(F, N).
	Decl ax_cnt_hi(F).
	Decl ax_num(N).
	Decl ax_big(N).
	Decl ax_sum(X, Z).

	ax_d(X) :- ax_src(X).
	ax_d(/pf).
	ax_open(X) :- ax_cand(X), !ax_d(X).

	ax_reach(X, Y) :- ax_e(X, Y), !ax_blk(X).
	ax_reach(X, Z) :- ax_reach(X, Y), ax_e(Y, Z), !ax_blk(Z).
	ax_has_out(X) :- ax_reach(X, _).
	ax_iso(X) :- ax_cand(X), !ax_has_out(X).

	ax_u(/pf).
	ax_ucand(X) :- ax_cand(X), !ax_u(X).

	ax_cnt(/root, 7).
	ax_cnt(F, N) :- ax_e(F, _) |> do fn:group_by(F), let N = fn:count().
	ax_cnt_hi(F) :- ax_cnt(F, N), N >= 2.

	ax_big(N) :- ax_num(N), N > 2.
	ax_sum(X, Z) :- ax_num(X), ax_num(Y), Z = fn:plus(X, Y).
`

var axNodes = []string{"/root", "/pf", "/a", "/b", "/c", "/d"}

func axFact(rng *rand.Rand, preds []string) Fact {
	pick := func() MangleAtom { return MangleAtom(axNodes[rng.Intn(len(axNodes))]) }
	switch pred := preds[rng.Intn(len(preds))]; pred {
	case "ax_e":
		return Fact{Predicate: pred, Args: []any{pick(), pick()}}
	case "ax_reach":
		return Fact{Predicate: pred, Args: []any{pick(), pick()}}
	case "ax_cnt":
		return Fact{Predicate: pred, Args: []any{pick(), int64(rng.Intn(4))}}
	case "ax_num":
		return Fact{Predicate: pred, Args: []any{int64(rng.Intn(5))}}
	default:
		return Fact{Predicate: pred, Args: []any{pick()}}
	}
}

// axWalk drives random batches through the lazy path and compares each with a
// full fixpoint.
func axWalk(t *testing.T, policy string, preds []string, seeds []int64, steps int) {
	for _, seed := range seeds {
		t.Run(fmt.Sprintf("seed-%d", seed), func(t *testing.T) {
			rng := rand.New(rand.NewSource(seed))
			k := newPolicyKernel(t, newRealKernelBase(), policy)
			var held []Fact
			for step := 0; step < steps; step++ {
				batch := 1 + rng.Intn(4)
				var log []string
				for i := 0; i < batch; i++ {
					switch {
					case len(held) > 0 && rng.Intn(3) == 0:
						j := rng.Intn(len(held))
						f := held[j]
						held = append(held[:j], held[j+1:]...)
						log = append(log, "retract "+f.String())
						if err := k.RetractExactFact(f); err != nil {
							t.Fatalf("step %d: retract %s: %v", step, f.String(), err)
						}
						if rng.Intn(5) == 0 { // and put it back in the same batch
							log = append(log, "reassert "+f.String())
							if err := k.AssertWithoutEval(f); err != nil {
								t.Fatal(err)
							}
							held = append(held, f)
						}
					default:
						f := axFact(rng, preds)
						log = append(log, "assert "+f.String())
						if err := k.AssertWithoutEval(f); err != nil {
							t.Fatalf("step %d: assert %s: %v", step, f.String(), err)
						}
						held = append(held, f)
					}
				}
				if err := k.EvaluatePending(); err != nil {
					t.Fatalf("step %d: EvaluatePending: %v", step, err)
				}
				requireSameStore(t, fmt.Sprintf("seed %d step %d mode %s batch %v", seed, step, k.LastEvaluation().Mode, log), dumpStore(t, k), fullOracle(t, k))
			}
		})
	}
}

func TestDRedAdversarialWalk(t *testing.T) {
	axWalk(t, axPolicy, []string{"ax_src", "ax_cand", "ax_blk", "ax_e", "ax_u", "ax_d", "ax_num"},
		[]int64{1, 2, 3, 4, 5, 6, 7, 8}, 150)
}

func TestDRedAdversarialWalkAssertedDerived(t *testing.T) {
	axWalk(t, axPolicy, []string{"ax_src", "ax_cand", "ax_blk", "ax_e", "ax_u", "ax_d", "ax_reach", "ax_cnt", "ax_open", "ax_num"},
		[]int64{11, 12, 13, 14, 15, 16}, 150)
}

// axApply writes, evaluates lazily and requires the maintained store to equal
// the full fixpoint, by the named mode.
func axApply(t *testing.T, k *RealKernel, stage, wantMode string, write func()) {
	t.Helper()
	write()
	if err := k.EvaluatePending(); err != nil {
		t.Fatalf("%s: %v", stage, err)
	}
	if got := k.LastEvaluation().Mode; wantMode != "" && got != wantMode {
		t.Fatalf("%s: evaluated as %q (fallback %q), want %q", stage, got, k.LastEvaluation().ConeFallback, wantMode)
	}
	requireSameStore(t, stage, dumpStore(t, k), fullOracle(t, k))
}

func axAssert(t *testing.T, k *RealKernel, facts ...Fact) {
	t.Helper()
	for _, f := range facts {
		if err := k.AssertWithoutEval(f); err != nil {
			t.Fatalf("assert %s: %v", f.String(), err)
		}
	}
}

func axRetract(t *testing.T, k *RealKernel, facts ...Fact) {
	t.Helper()
	for _, f := range facts {
		if err := k.RetractExactFact(f); err != nil {
			t.Fatalf("retract %s: %v", f.String(), err)
		}
	}
}

func axFactOf(pred string, args ...any) Fact { return Fact{Predicate: pred, Args: args} }

// A program fact of an aggregated head is not the group's output: the engine
// adds it before any rule runs and a recomputed group must leave it alone.
// The group recompute used to remove it with the group's stale row.
func TestDRedAggregateKeepsAProgramFactOfItsHead(t *testing.T) {
	k := newPolicyKernel(t, newRealKernelBase(), `
		Decl pf_e(X, Y).
		Decl pf_cnt(F, N).
		pf_cnt(/root, 7).
		pf_cnt(F, N) :- pf_e(F, _) |> do fn:group_by(F), let N = fn:count().
	`)
	e := func(a, b string) Fact { return axFactOf("pf_e", MangleAtom(a), MangleAtom(b)) }
	axApply(t, k, "first row of the group", "dred", func() { axAssert(t, k, e("/root", "/a")) })
	axApply(t, k, "second row of the group", "dred", func() { axAssert(t, k, e("/root", "/b")) })
	axApply(t, k, "a row leaves", "dred", func() { axRetract(t, k, e("/root", "/a")) })
	axApply(t, k, "the group empties", "dred", func() { axRetract(t, k, e("/root", "/b")) })
	rows, err := k.Query("pf_cnt")
	if err != nil || len(rows) != 1 {
		t.Fatalf("pf_cnt = %v, %v; want the program fact alone", rows, err)
	}
}
