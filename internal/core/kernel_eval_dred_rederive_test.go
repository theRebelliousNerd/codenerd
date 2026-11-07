package core

import (
	"fmt"
	"testing"
	"time"
)

func dlEdge(a, b string) Fact {
	return Fact{Predicate: "dl_edge", Args: []any{MangleAtom(a), MangleAtom(b)}}
}

// reachHolds reports whether the store holds dl_reach(a, b).
func reachHolds(t *testing.T, k *RealKernel, a, b string) bool {
	t.Helper()
	rows, err := k.Query("dl_reach")
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if len(row.Args) == 2 && fmt.Sprint(row.Args[0]) == a && fmt.Sprint(row.Args[1]) == b {
			return true
		}
	}
	return false
}

// retractAndEvaluate retracts f lazily and evaluates, requiring a DRed pass.
func retractAndEvaluate(t *testing.T, k *RealKernel, f Fact) EvaluationStats {
	t.Helper()
	if err := k.RetractExactFact(f); err != nil {
		t.Fatal(err)
	}
	if err := k.EvaluatePending(); err != nil {
		t.Fatal(err)
	}
	ev := k.LastEvaluation()
	if ev.Mode != "dred" || ev.OverDeleted == 0 {
		t.Fatalf("retract of %s evaluated as %q over-deleting %d (fallback %q); want dred over-deleting", f.String(), ev.Mode, ev.OverDeleted, ev.ConeFallback)
	}
	return ev
}

// Two reach facts that support only each other, entered through one edge, go
// when that edge goes: a recursive cycle never restores itself, because a
// re-derivation only grows from facts that already have support.
func TestDRedDeletingTheLastExternalSupportOfACycleRemovesIt(t *testing.T) {
	k := newPolicyKernel(t, newRealKernelBase(), deltaPolicy)
	for _, f := range []Fact{dlEdge("/x", "/a"), dlEdge("/a", "/b"), dlEdge("/b", "/a")} {
		if err := k.Assert(f); err != nil {
			t.Fatal(err)
		}
	}
	if !reachHolds(t, k, "/x", "/a") || !reachHolds(t, k, "/x", "/b") {
		t.Fatal("setup: dl_reach(/x,/a) and dl_reach(/x,/b) must hold through the entry edge")
	}
	ev := retractAndEvaluate(t, k, dlEdge("/x", "/a"))
	if reachHolds(t, k, "/x", "/a") || reachHolds(t, k, "/x", "/b") {
		t.Fatalf("the cycle dl_reach(/x,/a) <-> dl_reach(/x,/b) survived the loss of its entry edge (re-derived %d)", ev.Rederived)
	}
	if !reachHolds(t, k, "/a", "/a") || !reachHolds(t, k, "/a", "/b") {
		t.Fatal("dl_reach(/a,/a) and dl_reach(/a,/b) have their own edges and must stay")
	}
	requireSameStore(t, "after the entry edge went", dumpStore(t, k), fullOracle(t, k))
}

// Of two alternative supports, losing one keeps the fact, and the over-delete
// takes it only to bring it back.
func TestDRedDeletingOneOfTwoAlternativeSupportsKeepsTheFact(t *testing.T) {
	k := newPolicyKernel(t, newRealKernelBase(), deltaPolicy)
	for _, f := range []Fact{dlEdge("/p", "/m1"), dlEdge("/m1", "/q"), dlEdge("/p", "/m2"), dlEdge("/m2", "/q")} {
		if err := k.Assert(f); err != nil {
			t.Fatal(err)
		}
	}
	if !reachHolds(t, k, "/p", "/q") {
		t.Fatal("setup: dl_reach(/p,/q) must hold")
	}
	ev := retractAndEvaluate(t, k, dlEdge("/m1", "/q"))
	if !reachHolds(t, k, "/p", "/q") {
		t.Fatal("dl_reach(/p,/q) lost although /p -> /m2 -> /q remains")
	}
	if ev.Rederived == 0 {
		t.Fatalf("re-derived %d, want dl_reach(/p,/q) restored from the other path", ev.Rederived)
	}
	requireSameStore(t, "after one of two supports went", dumpStore(t, k), fullOracle(t, k))
}

// A chain whose only restorable fact is its head: restoring dl_reach(/s,/n0)
// is what supports dl_reach(/s,/n1), and that one dl_reach(/s,/n2), and so on.
// Every over-deleted fact is tested for support once, not once per pass.
func TestDRedARestoredFactEnablesTheNextWithoutRescans(t *testing.T) {
	const links = 40
	k := newPolicyKernel(t, newRealKernelBase(), deltaPolicy)
	facts := []Fact{dlEdge("/s", "/n0"), dlEdge("/s", "/t"), dlEdge("/t", "/n0")}
	for i := 0; i < links; i++ {
		facts = append(facts, dlEdge(fmt.Sprintf("/n%d", i), fmt.Sprintf("/n%d", i+1)))
	}
	for _, f := range facts {
		if err := k.AssertWithoutEval(f); err != nil {
			t.Fatal(err)
		}
	}
	if err := k.EvaluatePending(); err != nil {
		t.Fatal(err)
	}
	ev := retractAndEvaluate(t, k, dlEdge("/s", "/n0"))
	for i := 0; i <= links; i++ {
		if !reachHolds(t, k, "/s", fmt.Sprintf("/n%d", i)) {
			t.Fatalf("dl_reach(/s,/n%d) lost although /s -> /t -> /n0 -> ... remains", i)
		}
	}
	if ev.Rederived < links+1 {
		t.Fatalf("re-derived %d, want at least the %d chain facts", ev.Rederived, links+1)
	}
	if ev.SupportChecks > ev.OverDeleted {
		t.Fatalf("%d support checks for %d over-deleted facts; each must be tested once, not once per pass", ev.SupportChecks, ev.OverDeleted)
	}
	requireSameStore(t, "after the chain head was re-derived", dumpStore(t, k), fullOracle(t, k))
}

const ringPolicy = `
	Decl rg_edge(From, To).
	Decl rg_reach(From, To).
	rg_reach(X, Y) :- rg_edge(X, Y).
	rg_reach(X, Z) :- rg_reach(X, Y), rg_edge(Y, Z).
`

// BenchmarkDRedRederiveDenseReach removes one edge of a dense ring (each node
// reaches the next two) so that most of the closure is over-deleted and nearly
// all of it comes back through another path: the re-derivation is the work.
func BenchmarkDRedRederiveDenseReach(b *testing.B) {
	for _, n := range []int{40, 80} {
		b.Run(fmt.Sprintf("nodes=%d", n), func(b *testing.B) {
			k := newRealKernelBase()
			k.AppendPolicy(ringPolicy)
			node := func(i int) any { return MangleAtom(fmt.Sprintf("/n%d", i%n)) }
			edge := func(i, j int) Fact { return Fact{Predicate: "rg_edge", Args: []any{node(i), node(j)}} }
			for i := 0; i < n; i++ {
				for _, j := range []int{i + 1, i + 2} {
					if err := k.AssertWithoutEval(edge(i, j)); err != nil {
						b.Fatal(err)
					}
				}
			}
			if err := k.Evaluate(); err != nil {
				b.Fatal(err)
			}
			var total time.Duration
			var checks, rounds, over, back, variants int
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := k.RetractExactFact(edge(0, 1)); err != nil {
					b.Fatal(err)
				}
				started := time.Now()
				if err := k.EvaluatePending(); err != nil {
					b.Fatal(err)
				}
				total += time.Since(started)
				ev := k.LastEvaluation()
				if ev.Mode != "dred" {
					b.Fatalf("retract evaluated as %q (fallback %q), want dred", ev.Mode, ev.ConeFallback)
				}
				checks += ev.SupportChecks
				rounds += ev.RederiveRounds
				over += ev.OverDeleted
				back += ev.Rederived
				variants += ev.VariantsBuilt
				b.StopTimer()
				if err := k.AssertWithoutEval(edge(0, 1)); err != nil {
					b.Fatal(err)
				}
				if err := k.EvaluatePending(); err != nil {
					b.Fatal(err)
				}
				b.StartTimer()
			}
			b.StopTimer()
			per := func(v int) float64 { return float64(v) / float64(b.N) }
			b.ReportMetric(float64(total.Microseconds())/float64(b.N), "eval-us/op")
			b.ReportMetric(per(checks), "support-checks/op")
			b.ReportMetric(per(rounds), "rounds/op")
			b.ReportMetric(per(over), "overdeleted/op")
			b.ReportMetric(per(back), "rederived/op")
			b.ReportMetric(per(variants), "variants/op")
		})
	}
}
