package core

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/types"

	"codeberg.org/TauCeti/mangle-go/ast"
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

// axGen builds a random fact of a predicate; a predicate absent from it is
// unary over axNodes.
var axGen = map[string]func(rng *rand.Rand) []any{}

func axFact(rng *rand.Rand, preds []string) Fact {
	pick := func() MangleAtom { return MangleAtom(axNodes[rng.Intn(len(axNodes))]) }
	pred := preds[rng.Intn(len(preds))]
	if gen, ok := axGen[pred]; ok {
		return Fact{Predicate: pred, Args: gen(rng)}
	}
	switch pred {
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
			modes := map[string]int{}
			defer func() { t.Logf("modes %v", modes) }()
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
				modes[k.LastEvaluation().Mode]++
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

// bxPolicy: let transforms, aggregates of every function and shape, numeric
// atoms and expressions.
const bxPolicy = `
	Decl bx_a(X).
	Decl bx_b(X, Y).
	Decl bx_n(N).
	Decl bx_v(K, V).
	Decl bx_lab(N, L).
	Decl bx_nolab(N).
	Decl bx_has_lab(N).
	Decl bx_inc(M).
	Decl bx_cnt(F, N).
	Decl bx_cnt_swapped(N, F).
	Decl bx_stats(K, C, M, S).
	Decl bx_global(C).
	Decl bx_glob_neg(C).
	Decl bx_self(A, C).
	Decl bx_const(N).
	Decl bx_rep(F, N).
	Decl bx_chain(K, M2).
	Decl bx_last(M).

	bx_lab(N, L) :- bx_b(N, W) |> let L = fn:pair(N, W).
	bx_has_lab(N) :- bx_lab(N, _).
	bx_nolab(N) :- bx_a(N), !bx_has_lab(N).
	bx_inc(M) :- bx_n(X), M = fn:plus(X, 1).

	bx_cnt(F, N) :- bx_b(F, _) |> do fn:group_by(F), let N = fn:count().
	bx_cnt_swapped(N, F) :- bx_b(F, _) |> do fn:group_by(F), let N = fn:count().
	bx_stats(K, C, M, S) :- bx_v(K, V) |> do fn:group_by(K), let C = fn:count(), let M = fn:max(V), let S = fn:sum(V).
	bx_global(C) :- bx_n(_) |> do fn:group_by(), let C = fn:count().
	bx_glob_neg(C) :- bx_n(X), !bx_a(X) |> do fn:group_by(), let C = fn:count().
	bx_self(A, C) :- bx_b(A, B), bx_b(B, _) |> do fn:group_by(A), let C = fn:count().
	bx_const(N) :- bx_b(/a, _) |> do fn:group_by(), let N = fn:count().
	bx_rep(F, N) :- bx_b(F, F) |> do fn:group_by(F), let N = fn:count().
	bx_chain(K, M2) :- bx_stats(K, _, M, _), M2 = fn:plus(M, 1).
	bx_last(M) :- bx_cnt(_, C) |> do fn:group_by(), let M = fn:max(C).
`

func init() {
	axGen["bx_n"] = func(rng *rand.Rand) []any { return []any{int64(rng.Intn(5))} }
	axGen["bx_v"] = func(rng *rand.Rand) []any {
		return []any{MangleAtom(axNodes[rng.Intn(3)]), int64(rng.Intn(5))}
	}
	axGen["bx_b"] = func(rng *rand.Rand) []any {
		return []any{MangleAtom(axNodes[rng.Intn(4)]), MangleAtom(axNodes[rng.Intn(4)])}
	}
}

func TestDRedAdversarialWalkAggregatesAndLets(t *testing.T) {
	axWalk(t, bxPolicy, []string{"bx_a", "bx_b", "bx_n", "bx_v"}, []int64{21, 22, 23, 24, 25, 26}, 150)
}

// cxPolicy: an average. (fn:collect and fn:float:sum are left out: their value
// depends on the order rows arrive, which a maintained store cannot share with
// a fresh one.)
const cxPolicy = `
	Decl cx_v(K, V).
	Decl cx_avg(K, S).

	cx_avg(K, S) :- cx_v(K, V) |> do fn:group_by(K), let S = fn:avg(V).
`

func init() {
	axGen["cx_v"] = func(rng *rand.Rand) []any {
		return []any{MangleAtom(axNodes[rng.Intn(2)]), int64(rng.Intn(6))}
	}
}

func TestDRedAdversarialWalkAverage(t *testing.T) {
	axWalk(t, cxPolicy, []string{"cx_v"}, []int64{31, 32, 33, 34}, 150)
}

// axKernelWalk is one kernel's state in a clone walk.
type axKernelWalk struct {
	k    *RealKernel
	held []Fact
	name string
}

func (w *axKernelWalk) step(t *testing.T, rng *rand.Rand, preds []string, stage string) {
	t.Helper()
	for i, n := 0, 1+rng.Intn(4); i < n; i++ {
		if len(w.held) > 0 && rng.Intn(3) == 0 {
			j := rng.Intn(len(w.held))
			f := w.held[j]
			w.held = append(w.held[:j:j], w.held[j+1:]...)
			if err := w.k.RetractExactFact(f); err != nil {
				t.Errorf("%s %s: retract %s: %v", stage, w.name, f.String(), err)
				return
			}
			continue
		}
		f := axFact(rng, preds)
		if err := w.k.AssertWithoutEval(f); err != nil {
			t.Errorf("%s %s: assert %s: %v", stage, w.name, f.String(), err)
			return
		}
		w.held = append(w.held, f)
	}
	if err := w.k.EvaluatePending(); err != nil {
		t.Errorf("%s %s: EvaluatePending: %v", stage, w.name, err)
	}
}

// Clones written on both sides, and clones of clones, each match their own
// full fixpoint, and a write on one never reaches another.
func TestDRedClonesWrittenOnBothSidesMatchFullFixpoint(t *testing.T) {
	preds := []string{"ax_src", "ax_cand", "ax_blk", "ax_e", "ax_u", "ax_d", "ax_num"}
	for _, seed := range []int64{101, 102, 103} {
		t.Run(fmt.Sprintf("seed-%d", seed), func(t *testing.T) {
			rng := rand.New(rand.NewSource(seed))
			root := &axKernelWalk{k: newPolicyKernel(t, newRealKernelBase(), axPolicy), name: "root"}
			walks := []*axKernelWalk{root}
			frozen := map[*axKernelWalk][]string{}
			for step := 0; step < 120; step++ {
				switch rng.Intn(6) {
				case 0:
					src := walks[rng.Intn(len(walks))]
					if len(walks) < 6 {
						c := &axKernelWalk{k: src.k.Clone(), held: append([]Fact(nil), src.held...), name: fmt.Sprintf("%s>c%d", src.name, step)}
						walks = append(walks, c)
						requireSameStore(t, "fresh clone "+c.name, dumpStore(t, c.k), dumpStore(t, src.k))
					}
				default:
					w := walks[rng.Intn(len(walks))]
					w.step(t, rng, preds, fmt.Sprintf("seed %d step %d", seed, step))
					if t.Failed() {
						return
					}
					requireSameStore(t, fmt.Sprintf("seed %d step %d %s mode %s", seed, step, w.name, w.k.LastEvaluation().Mode), dumpStore(t, w.k), fullOracle(t, w.k))
					frozen[w] = dumpStore(t, w.k)
					// Every kernel not written this step still holds what it held.
					for _, o := range walks {
						if o != w && frozen[o] != nil {
							requireSameStore(t, fmt.Sprintf("seed %d step %d: %s changed by a write to %s", seed, step, o.name, w.name), dumpStore(t, o.k), frozen[o])
						}
					}
				}
			}
		})
	}
}

// Concurrent clones, each written and evaluated on its own goroutine while the
// parent keeps writing (run with -race).
func TestDRedConcurrentClonesMatchFullFixpoint(t *testing.T) {
	preds := []string{"ax_src", "ax_cand", "ax_blk", "ax_e", "ax_u", "ax_d", "ax_num"}
	parent := &axKernelWalk{k: newPolicyKernel(t, newRealKernelBase(), axPolicy), name: "parent"}
	rng := rand.New(rand.NewSource(7))
	for i := 0; i < 20; i++ {
		parent.step(t, rng, preds, "warm")
	}
	var wg sync.WaitGroup
	results := make(chan string, 64)
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			grng := rand.New(rand.NewSource(int64(g) + 900))
			for round := 0; round < 10; round++ {
				c := &axKernelWalk{k: parent.k.Clone(), name: fmt.Sprintf("g%d-r%d", g, round)}
				c.held = nil
				for s := 0; s < 6; s++ {
					c.step(t, grng, preds, c.name)
					if t.Failed() {
						return
					}
					got, want := dumpStoreNoT(c.k), oracleNoT(c.k)
					if strings.Join(got, "\n") != strings.Join(want, "\n") {
						results <- fmt.Sprintf("%s step %d mode %s: store has %d facts, full fixpoint %d", c.name, s, c.k.LastEvaluation().Mode, len(got), len(want))
						return
					}
				}
			}
		}(g)
	}
	for s := 0; s < 40; s++ {
		parent.step(t, rng, preds, "parent")
	}
	wg.Wait()
	close(results)
	for msg := range results {
		t.Error(msg)
	}
	requireSameStore(t, "parent after the clones", dumpStore(t, parent.k), fullOracle(t, parent.k))
}

func dumpStoreNoT(k *RealKernel) []string {
	k.mu.RLock()
	defer k.mu.RUnlock()
	var out []string
	for _, sym := range k.store.ListPredicates() {
		if strings.HasSuffix(sym.Symbol, ast.InternalPredicateSuffix) {
			continue
		}
		_ = k.store.GetFacts(ast.NewQuery(sym), func(a ast.Atom) error {
			out = append(out, a.String())
			return nil
		})
	}
	sort.Strings(out)
	return out
}

func oracleNoT(k *RealKernel) []string {
	ref := k.Clone()
	if err := ref.Evaluate(); err != nil {
		return []string{"oracle error: " + err.Error()}
	}
	return dumpStoreNoT(ref)
}

// dxPolicy: recursion shapes (non-linear, mutual) with negation and
// aggregation above them, and aggregate heads of unusual shape.
const dxPolicy = `
	Decl dx_e(X, Y).
	Decl dx_off(X).
	Decl dx_v(K, V).
	Decl dx_tc(X, Y).
	Decl dx_even(X, Y).
	Decl dx_odd(X, Y).
	Decl dx_unlinked(X, Y).
	Decl dx_node(X).
	Decl dx_tc_count(X, N).
	Decl dx_const_head(K, N).
	Decl dx_twice(F, G, N).
	Decl dx_filtered(K, N).
	Decl dx_eq(K, N).
	Decl dx_big_hubs(X).

	dx_tc(X, Y) :- dx_e(X, Y).
	dx_tc(X, Z) :- dx_tc(X, Y), dx_tc(Y, Z).

	dx_odd(X, Y) :- dx_e(X, Y), !dx_off(X).
	dx_even(X, Z) :- dx_odd(X, Y), dx_e(Y, Z).
	dx_odd(X, Z) :- dx_even(X, Y), dx_e(Y, Z).

	dx_node(X) :- dx_e(X, _).
	dx_node(X) :- dx_e(_, X).
	dx_unlinked(X, Y) :- dx_node(X), dx_node(Y), !dx_tc(X, Y).

	dx_tc_count(X, N) :- dx_tc(X, _) |> do fn:group_by(X), let N = fn:count().
	dx_big_hubs(X) :- dx_tc_count(X, N), N >= 3.

	dx_const_head(/k, N) :- dx_v(_, V) |> do fn:group_by(), let N = fn:count().
	dx_twice(F, F, N) :- dx_v(F, _) |> do fn:group_by(F), let N = fn:count().
	dx_filtered(K, N) :- dx_v(K, V), V > 2 |> do fn:group_by(K), let N = fn:count().
	dx_eq(K, N) :- dx_v(K, V), W = fn:plus(V, 1), W > 3 |> do fn:group_by(K), let N = fn:count().
`

func init() {
	axGen["dx_e"] = func(rng *rand.Rand) []any {
		return []any{MangleAtom(axNodes[rng.Intn(5)]), MangleAtom(axNodes[rng.Intn(5)])}
	}
	axGen["dx_v"] = func(rng *rand.Rand) []any {
		return []any{MangleAtom(axNodes[rng.Intn(3)]), int64(rng.Intn(6))}
	}
}

func TestDRedAdversarialWalkRecursionShapes(t *testing.T) {
	axWalk(t, dxPolicy, []string{"dx_e", "dx_e", "dx_e", "dx_off", "dx_v"}, []int64{41, 42, 43, 44, 45, 46}, 150)
}

// A row retracted and a row of the same canonical key asserted in its place
// can admit to two atoms that print alike (int64(1) and float64(1)). The
// string is types.MangleString, so the retraction takes the alias-support
// path, which used to treat the float as the removed int because both print
// as 1. The derived copy and the rule that matches only the float then
// disappeared.
func TestDRedAliasPredicateSwapsANumberForAnEqualFloat(t *testing.T) {
	k := newPolicyKernel(t, newRealKernelBase(), `
		Decl al_p(S, N) bound [/string, /any].
		Decl al_q(S, N).
		Decl al_seen(S).
		al_q(S, N) :- al_p(S, N).
		al_seen(S) :- al_q(S, N), N = 1.0.
	`)
	as := func(n any) Fact { return axFactOf("al_p", types.MangleString("x"), n) }
	axApply(t, k, "number", "dred", func() { axAssert(t, k, as(int64(1))) })
	axApply(t, k, "swap for a float", "dred", func() {
		axRetract(t, k, as(int64(1)))
		axAssert(t, k, as(float64(1)))
	})
	axApply(t, k, "swap back", "dred", func() {
		axRetract(t, k, as(float64(1)))
		axAssert(t, k, as(int64(1)))
	})
}

// int64(1) and float64(1) are different atoms that print as 1. The engine's
// group_by folds them into one group, so the count is 2 and the rule on that
// count fires. A numeric group whose values are one type stays maintained.
func TestDRedAggregateGroupsANumberWithTheFloatThatPrintsLikeIt(t *testing.T) {
	plain := newPolicyKernel(t, newRealKernelBase(), `
		Decl ng_p(K, V).
		Decl ng_h(K, C).
		ng_h(K, C) :- ng_p(K, V) |> do fn:group_by(K), let C = fn:count().
	`)
	row := func(k, v any) Fact { return axFactOf("ng_p", k, v) }
	axApply(t, plain, "two ints in one group", "dred", func() {
		axAssert(t, plain, row(MangleAtom("/a"), int64(1)), row(MangleAtom("/a"), int64(2)))
	})
	axApply(t, plain, "one int leaves", "dred", func() {
		axRetract(t, plain, row(MangleAtom("/a"), int64(1)))
	})

	policy := `
		Decl ag_a(N) bound [/any].
		Decl ag_b(N) bound [/any].
		Decl ag_e(N, X).
		Decl ag_p(N).
		Decl ag_h(N, C).
		Decl ag_big(N).
		Decl ag_hj(N, C).
		Decl ag_hj_big(N).
		ag_p(N) :- ag_a(N).
		ag_p(N) :- ag_b(N).
		ag_h(N, C) :- ag_p(N) |> do fn:group_by(N), let C = fn:count().
		ag_big(N) :- ag_h(N, C), C >= 2.
		ag_hj(N, C) :- ag_p(N), ag_e(N, X) |> do fn:group_by(N), let C = fn:count().
		ag_hj_big(N) :- ag_hj(N, C), C >= 2.
	`
	k := newPolicyKernel(t, newRealKernelBase(), policy)
	axApply(t, k, "int and the float that prints as it", "", func() {
		axAssert(t, k,
			axFactOf("ag_a", int64(1)), axFactOf("ag_b", float64(1)),
			axFactOf("ag_e", int64(1), MangleAtom("/a")), axFactOf("ag_e", float64(1), MangleAtom("/b")))
	})
	axApply(t, k, "the float leaves", "", func() {
		axRetract(t, k, axFactOf("ag_b", float64(1)), axFactOf("ag_e", float64(1), MangleAtom("/b")))
	})
	axApply(t, k, "the float returns", "", func() {
		axAssert(t, k, axFactOf("ag_b", float64(1)), axFactOf("ag_e", float64(1), MangleAtom("/b")))
	})

	// The float already in the store, and the number asserted after it, are
	// still one group. Which of the two atoms stands for the group follows
	// the store's iteration order; the count does not.
	k2 := newPolicyKernel(t, newRealKernelBase(), policy)
	axApply(t, k2, "float first", "", func() {
		axAssert(t, k2, axFactOf("ag_b", float64(1)), axFactOf("ag_e", float64(1), MangleAtom("/b")))
	})
	axApply(t, k2, "the number joins it", "", func() {
		axAssert(t, k2, axFactOf("ag_a", int64(1)), axFactOf("ag_e", int64(1), MangleAtom("/a")))
	})
	axApply(t, k2, "the number leaves", "", func() {
		axRetract(t, k2, axFactOf("ag_a", int64(1)), axFactOf("ag_e", int64(1), MangleAtom("/a")))
	})

	// The group variable is computed (N = V), so no body atom carries it. A
	// later row of the number, beside a float that prints as that number,
	// still counts toward the same group.
	computed := newPolicyKernel(t, newRealKernelBase(), `
		Decl cg_src(Id, V) bound [/name, /any].
		Decl cg_h(N, C).
		Decl cg_big(N).
		cg_h(N, C) :- cg_src(_, V), N = V |> do fn:group_by(N), let C = fn:count().
		cg_big(N) :- cg_h(N, C), C >= 2.
	`)
	src := func(id string, v any) Fact { return axFactOf("cg_src", MangleAtom(id), v) }
	axApply(t, computed, "computed key, number and float", "", func() {
		axAssert(t, computed, src("/a", int64(1)), src("/b", float64(1)))
	})
	axApply(t, computed, "computed key, another number joins the group", "", func() {
		axAssert(t, computed, src("/c", int64(1)))
	})
	axApply(t, computed, "computed key, the float leaves", "", func() {
		axRetract(t, computed, src("/b", float64(1)))
	})
}

// A group key can be a pair or a list. int64(1) and float64(1) still print as
// 1 inside it, and the engine's group_by folds the two keys together. The
// count is of the printed key, so one pair of (1, /k) from each side is a
// group of 2.
func TestDRedAggregateGroupsAPairThatHoldsTheFloatThatPrintsLikeItsNumber(t *testing.T) {
	names := newPolicyKernel(t, newRealKernelBase(), `
		Decl np_n(N) bound [/name].
		Decl np_p(P).
		Decl np_h(P, C).
		np_p(P) :- np_n(N), P = fn:pair(N, /k).
		np_h(P, C) :- np_p(P) |> do fn:group_by(P), let C = fn:count().
	`)
	axApply(t, names, "two name pairs", "dred", func() {
		axAssert(t, names, axFactOf("np_n", MangleAtom("/a")), axFactOf("np_n", MangleAtom("/b")))
	})

	const pairPolicy = `
		Decl pk_a(N) bound [/any].
		Decl pk_b(N) bound [/any].
		Decl pk_p(P).
		Decl pk_h(P, C).
		Decl pk_big(P).
		pk_p(P) :- pk_a(N), P = fn:pair(N, /k).
		pk_p(P) :- pk_b(N), P = fn:pair(N, /k).
		pk_h(P, C) :- pk_p(P) |> do fn:group_by(P), let C = fn:count().
		pk_big(P) :- pk_h(P, C), C >= 2.
	`
	k := newPolicyKernel(t, newRealKernelBase(), pairPolicy)
	axApply(t, k, "one number pair", "dred", func() {
		axAssert(t, k, axFactOf("pk_a", int64(1)))
	})
	axApply(t, k, "the float's pair joins it", "", func() {
		axAssert(t, k, axFactOf("pk_b", float64(1)))
	})
	axApply(t, k, "the float's pair leaves", "", func() {
		axRetract(t, k, axFactOf("pk_b", float64(1)))
	})

	lists := newPolicyKernel(t, newRealKernelBase(), `
		Decl lk_a(N) bound [/any].
		Decl lk_b(N) bound [/any].
		Decl lk_p(L).
		Decl lk_h(L, C).
		Decl lk_big(L).
		lk_p(L) :- lk_a(N), L = fn:list(N, /k).
		lk_p(L) :- lk_b(N), L = fn:list(N, /k).
		lk_h(L, C) :- lk_p(L) |> do fn:group_by(L), let C = fn:count().
		lk_big(L) :- lk_h(L, C), C >= 2.
	`)
	axApply(t, lists, "one number list", "dred", func() {
		axAssert(t, lists, axFactOf("lk_a", int64(1)))
	})
	axApply(t, lists, "the float's list joins it", "", func() {
		axAssert(t, lists, axFactOf("lk_b", float64(1)))
	})
	axApply(t, lists, "the float's list leaves", "", func() {
		axRetract(t, lists, axFactOf("lk_b", float64(1)))
	})

	// The pair is computed in the aggregate body, so no premise column carries
	// it. A later row of the number still belongs to the float's group.
	computed := newPolicyKernel(t, newRealKernelBase(), `
		Decl cpk_src(Id, N) bound [/name, /any].
		Decl cpk_h(P, C).
		Decl cpk_big(P).
		cpk_h(P, C) :- cpk_src(_, N), P = fn:pair(N, /k) |> do fn:group_by(P), let C = fn:count().
		cpk_big(P) :- cpk_h(P, C), C >= 2.
	`)
	src := func(id string, v any) Fact { return axFactOf("cpk_src", MangleAtom(id), v) }
	axApply(t, computed, "computed pair, number and float", "", func() {
		axAssert(t, computed, src("/a", int64(1)), src("/b", float64(1)))
	})
	axApply(t, computed, "computed pair, another number joins", "", func() {
		axAssert(t, computed, src("/c", int64(1)))
	})
	axApply(t, computed, "computed pair, the float leaves", "", func() {
		axRetract(t, computed, src("/b", float64(1)))
	})
}
