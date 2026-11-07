package core

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

// deltaPolicy has every shape the delta pass must get right or hand to the
// replace set: recursion, a multi-premise join, a let transform, a negation of
// a predicate no write touches (delta-safe), a negation of one a write does
// touch (replaced), an aggregation (replaced) and a rule downstream of the
// replaced head (replaced with it), a predicate both asserted and derived,
// and a program fact.
const deltaPolicy = `
	Decl dl_edge(From, To).
	Decl dl_reach(From, To).
	Decl dl_owner(Node, Who).
	Decl dl_owned_reach(Who, To).
	Decl dl_label(Node, Label).
	Decl dl_banned(Node).
	Decl dl_clean(Node).
	Decl dl_quarantine(Node).
	Decl dl_exposed(Node).
	Decl dl_exposed_count(N).
	Decl dl_alarm().
	Decl dl_alarm_owner(Who).
	Decl dl_mixed(Node).
	Decl dl_mixed_seen(Node).
	Decl dl_frozen(Node).

	dl_reach(X, Y) :- dl_edge(X, Y).
	dl_reach(X, Z) :- dl_reach(X, Y), dl_edge(Y, Z).

	dl_owned_reach(W, T) :- dl_owner(N, W), dl_reach(N, T).
	dl_label(N, L) :- dl_owner(N, W) |> let L = fn:list(N, W).

	dl_clean(N) :- dl_owner(N, _), !dl_frozen(N).

	dl_exposed(N) :- dl_reach(_, N), !dl_quarantine(N).
	dl_exposed_count(C) :- dl_exposed(_) |> do fn:group_by(), let C = fn:count().
	dl_alarm() :- dl_exposed_count(C), C >= 3.
	dl_alarm_owner(W) :- dl_alarm(), dl_owner(_, W).

	dl_mixed(N) :- dl_banned(N).
	dl_mixed_seen(N) :- dl_mixed(N), dl_owner(N, _).

	dl_edge(/root, /a).
`

var dredWritable = []string{"dl_edge", "dl_owner", "dl_banned", "dl_quarantine", "dl_mixed", "dl_frozen"}

// TestDRedMatchesFullFixpoint drives random batches of asserts and retracts
// through the lazy path and compares the whole store with a fresh full
// fixpoint after each one. Most batches must be DRed evaluations, and over
// the walk DRed must over-delete, re-derive and insert, or it passed
// vacuously.
func TestDRedMatchesFullFixpoint(t *testing.T) {
	for _, seed := range []int64{1, 42, 20261106, 7} {
		t.Run(fmt.Sprintf("seed-%d", seed), func(t *testing.T) {
			rng := rand.New(rand.NewSource(seed))
			k := newPolicyKernel(t, newRealKernelBase(), deltaPolicy)
			nodes := []string{"/root", "/a", "/b", "/c", "/d", "/e", "/f"}
			pick := func() string { return nodes[rng.Intn(len(nodes))] }
			who := []string{"/ana", "/bo", "/cy"}
			var held []Fact

			dreds, inserted, overDeleted, rederived := 0, 0, 0, 0
			for step := 0; step < 200; step++ {
				batch := 1 + rng.Intn(4)
				for i := 0; i < batch; i++ {
					if len(held) > 0 && rng.Intn(3) == 0 {
						j := rng.Intn(len(held))
						f := held[j]
						held = append(held[:j], held[j+1:]...)
						if err := k.RetractExactFact(f); err != nil {
							t.Fatalf("step %d: retract %s: %v", step, f.String(), err)
						}
						continue
					}
					var f Fact
					switch pred := dredWritable[rng.Intn(len(dredWritable))]; pred {
					case "dl_edge":
						f = Fact{Predicate: pred, Args: []any{MangleAtom(pick()), MangleAtom(pick())}}
					case "dl_owner":
						f = Fact{Predicate: pred, Args: []any{MangleAtom(pick()), MangleAtom(who[rng.Intn(len(who))])}}
					default:
						f = Fact{Predicate: pred, Args: []any{MangleAtom(pick())}}
					}
					if err := k.AssertWithoutEval(f); err != nil {
						t.Fatalf("step %d: assert %s: %v", step, f.String(), err)
					}
					held = append(held, f)
				}
				if err := k.EvaluatePending(); err != nil {
					t.Fatalf("step %d: EvaluatePending: %v", step, err)
				}
				// A batch that writes only predicates every consumer of which
				// is replaced (dl_quarantine feeds only the aggregate's side)
				// has nothing to maintain and is a plain cone; never full.
				ev := k.LastEvaluation()
				if ev.Mode != "dred" && ev.Mode != "cone" {
					t.Fatalf("step %d: batch evaluated as %q (fallback %q), want dred or cone", step, ev.Mode, ev.ConeFallback)
				}
				if ev.Mode == "dred" {
					dreds++
					inserted += ev.DeltaFacts
					overDeleted += ev.OverDeleted
					rederived += ev.Rederived
				}
				requireSameStore(t, fmt.Sprintf("seed %d step %d", seed, step), dumpStore(t, k), fullOracle(t, k))
			}
			if dreds < 100 || inserted == 0 || overDeleted == 0 || rederived == 0 {
				t.Fatalf("walk: %d dred evaluations of 200; inserted %d, over-deleted %d, re-derived %d; want most batches by dred and all three kinds of work",
					dreds, inserted, overDeleted, rederived)
			}
		})
	}
}

// A retract is maintained too: the reachability it supported is over-deleted
// and what has another path is re-derived.
func TestDRedRetractReDerivesAlternativePaths(t *testing.T) {
	k := newPolicyKernel(t, newRealKernelBase(), deltaPolicy)
	edge := func(a, b string) Fact { return Fact{Predicate: "dl_edge", Args: []any{MangleAtom(a), MangleAtom(b)}} }
	for _, f := range []Fact{edge("/a", "/b"), edge("/b", "/c"), edge("/a", "/c")} {
		if err := k.Assert(f); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := k.Query("dl_reach"); err != nil {
		t.Fatal(err)
	}
	if err := k.RetractExactFact(edge("/b", "/c")); err != nil {
		t.Fatal(err)
	}
	if _, err := k.Query("dl_reach"); err != nil {
		t.Fatal(err)
	}
	ev := k.LastEvaluation()
	if ev.Mode != "dred" || ev.OverDeleted == 0 || ev.Rederived == 0 {
		t.Fatalf("retract evaluated as %q, over-deleted %d, re-derived %d; want dred re-deriving reach(/a,/c) through the direct edge",
			ev.Mode, ev.OverDeleted, ev.Rederived)
	}
	requireSameStore(t, "after retract", dumpStore(t, k), fullOracle(t, k))
}

// worldBatch asserts n files' world facts: topology, two symbols, a call
// between them, an import of the previous file and a definition.
func worldBatch(t testing.TB, k *RealKernel, prefix string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		path := fmt.Sprintf("%s/f%d.go", prefix, i)
		fn := fmt.Sprintf("%s.F%d", prefix, i)
		gn := fmt.Sprintf("%s.G%d", prefix, i)
		facts := []Fact{
			{Predicate: "file_topology", Args: []any{path, "h", MangleAtom("/go"), int64(1), MangleAtom("/false")}},
			{Predicate: "symbol_graph", Args: []any{fn, MangleAtom("/function"), MangleAtom("/public"), path, "func()"}},
			{Predicate: "symbol_graph", Args: []any{gn, MangleAtom("/function"), MangleAtom("/private"), path, "func()"}},
			{Predicate: "dependency_link", Args: []any{fn, gn, ""}},
			{Predicate: "code_defines", Args: []any{path, fn, MangleAtom("/function"), int64(1), int64(9)}},
		}
		if i > 0 {
			facts = append(facts, Fact{Predicate: "file_imports", Args: []any{path, fmt.Sprintf("%s/f%d.go", prefix, i-1)}})
		}
		for _, f := range facts {
			if err := k.AssertWithoutEval(f); err != nil {
				t.Fatalf("assert %s: %v", f.String(), err)
			}
		}
	}
}

// TestDRedMatchesFullFixpoint_ProductionWorld runs DRed over the
// embedded production policy: a world, then insert-only batches of
// new files, each compared on every predicate with a full fixpoint.
func TestDRedMatchesFullFixpoint_ProductionWorld(t *testing.T) {
	k := setupMockKernel(t)
	worldBatch(t, k, "base", 300)
	if err := k.Evaluate(); err != nil {
		t.Fatal(err)
	}
	for step := 0; step < 4; step++ {
		worldBatch(t, k, fmt.Sprintf("batch%d", step), 25)
		if err := k.EvaluatePending(); err != nil {
			t.Fatalf("batch %d: %v", step, err)
		}
		ev := k.LastEvaluation()
		if ev.Mode != "dred" || ev.DeltaFacts == 0 {
			t.Fatalf("batch %d evaluated as %q with %d inserted (fallback %q), want dred inserting rows", step, ev.Mode, ev.DeltaFacts, ev.ConeFallback)
		}
		requireSameStore(t, fmt.Sprintf("batch %d", step), dumpStore(t, k), fullOracle(t, k))
	}
}

// BenchmarkProductionWorldInsertPending measures the lazy path for an
// insert-only batch over a 48K-file world with the production policy: the
// write that cost 4-16 s through the cone path.
func BenchmarkProductionWorldInsertPending(b *testing.B) {
	for _, batch := range []int{1, 2000} {
		b.Run(fmt.Sprintf("batch=%d", batch), func(b *testing.B) {
			k, err := NewRealKernel()
			if err != nil {
				b.Fatal(err)
			}
			worldBatch(b, k, "base", 48000)
			if err := k.Evaluate(); err != nil {
				b.Fatal(err)
			}
			var total time.Duration
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				worldBatch(b, k, fmt.Sprintf("d%d", i), batch)
				started := time.Now()
				if err := k.EvaluatePending(); err != nil {
					b.Fatal(err)
				}
				total += time.Since(started)
			}
			b.StopTimer()
			ev := k.LastEvaluation()
			b.ReportMetric(float64(total.Microseconds())/float64(b.N), "eval-us/op")
			b.ReportMetric(float64(ev.DeltaFacts), "inserted")
			b.Logf("mode=%s dred=%v replace(prepare=%v engine=%v rules=%d) fallback=%q",
				ev.Mode, ev.DeltaDuration, ev.PrepareDuration, ev.EngineDuration, ev.Rules, ev.ConeFallback)
		})
	}
}

// A DRed pass that fails part-way has already changed rows of the store. Its
// retry must not trust them: the store is tainted and only a full evaluation,
// which builds a fresh one, runs next.
func TestFailedDRedTaintsTheStoreUntilAFullEvaluation(t *testing.T) {
	k := newPolicyKernel(t, newRealKernelBase(), deltaPolicy)
	k.SetDerivedFactLimit(3)
	for i := 0; i < 12; i++ {
		if err := k.AssertWithoutEval(Fact{Predicate: "dl_edge", Args: []any{MangleAtom(fmt.Sprintf("/n%d", i)), MangleAtom(fmt.Sprintf("/n%d", i+1))}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := k.EvaluatePending(); err == nil {
		t.Fatal("the chain closure must exceed the derived-fact limit")
	}
	if why := k.coneIneligibilityLocked(); why != "store left by a failed incremental evaluation" {
		t.Fatalf("after a failed delta the incremental paths say %q; want them refused", why)
	}
	k.SetDerivedFactLimit(0)
	if _, err := k.Query("dl_reach"); err != nil {
		t.Fatal(err)
	}
	if ev := k.LastEvaluation(); ev.Mode != "full" {
		t.Fatalf("retry evaluated as %q, want full", ev.Mode)
	}
	requireSameStore(t, "after recovery", dumpStore(t, k), fullOracle(t, k))
	if err := k.Assert(Fact{Predicate: "dl_edge", Args: []any{MangleAtom("/n12"), MangleAtom("/n13")}}); err != nil {
		t.Fatal(err)
	}
	if _, err := k.Query("dl_reach"); err != nil {
		t.Fatal(err)
	}
	if ev := k.LastEvaluation(); ev.Mode != "dred" {
		t.Fatalf("after a full repair the next insert evaluated as %q, want dred again", ev.Mode)
	}
	requireSameStore(t, "after the next insert", dumpStore(t, k), fullOracle(t, k))
}

// A row retracted and asserted again in one batch is no change. Read as a
// removal plus an insert, "!dl_frozen(/d)" looked true and DRed inserted
// dl_clean(/d) beside a frozen node.
func TestDRedRetractAndReassertIsNoChange(t *testing.T) {
	k := newPolicyKernel(t, newRealKernelBase(), deltaPolicy)
	frozen := Fact{Predicate: "dl_frozen", Args: []any{MangleAtom("/d")}}
	for _, f := range []Fact{{Predicate: "dl_owner", Args: []any{MangleAtom("/d"), MangleAtom("/ana")}}, frozen} {
		if err := k.Assert(f); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := k.Query("dl_clean"); err != nil {
		t.Fatal(err)
	}
	if err := k.RetractExactFact(frozen); err != nil {
		t.Fatal(err)
	}
	if err := k.AssertWithoutEval(frozen); err != nil {
		t.Fatal(err)
	}
	clean, err := k.Query("dl_clean")
	if err != nil {
		t.Fatal(err)
	}
	if len(clean) != 0 {
		t.Fatalf("dl_clean = %v after retracting and re-asserting dl_frozen(/d); want none", clean)
	}
	requireSameStore(t, "after retract and reassert", dumpStore(t, k), fullOracle(t, k))
}

// BenchmarkProductionWorldRetractPending measures the lazy path for retracting
// one file's world facts from a 48K-file world: through the cone it removed and
// re-derived every downstream predicate; DRed over-deletes what the file
// supported.
func BenchmarkProductionWorldRetractPending(b *testing.B) {
	k, err := NewRealKernel()
	if err != nil {
		b.Fatal(err)
	}
	worldBatch(b, k, "base", 48000)
	if err := k.Evaluate(); err != nil {
		b.Fatal(err)
	}
	var total time.Duration
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		path := fmt.Sprintf("base/f%d.go", 100+i)
		if err := k.RetractExactFact(Fact{Predicate: "file_topology", Args: []any{path, "h", MangleAtom("/go"), int64(1), MangleAtom("/false")}}); err != nil {
			b.Fatal(err)
		}
		started := time.Now()
		if err := k.EvaluatePending(); err != nil {
			b.Fatal(err)
		}
		total += time.Since(started)
	}
	b.StopTimer()
	ev := k.LastEvaluation()
	b.ReportMetric(float64(total.Microseconds())/float64(b.N), "eval-us/op")
	b.Logf("mode=%s dred=%v overdeleted=%d rederived=%d replace(prepare=%v engine=%v rules=%d) fallback=%q",
		ev.Mode, ev.DeltaDuration, ev.OverDeleted, ev.Rederived, ev.PrepareDuration, ev.EngineDuration, ev.Rules, ev.ConeFallback)
}
