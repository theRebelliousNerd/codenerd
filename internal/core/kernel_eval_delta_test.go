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

var deltaWritable = []string{"dl_edge", "dl_owner", "dl_banned", "dl_quarantine", "dl_mixed"}

// TestDeltaEvaluationMatchesFullFixpoint drives insert-only batches through
// the lazy path and compares the whole store with a fresh full fixpoint after
// each one. Most batches must be delta evaluations, and the walk must both
// propagate (derive new rows by delta) and replace (re-derive a negating or
// aggregating head) or it passed vacuously.
func TestDeltaEvaluationMatchesFullFixpoint(t *testing.T) {
	for _, seed := range []int64{1, 42, 20261106} {
		t.Run(fmt.Sprintf("seed-%d", seed), func(t *testing.T) {
			rng := rand.New(rand.NewSource(seed))
			k := newPolicyKernel(t, newRealKernelBase(), deltaPolicy)
			nodes := []string{"/root", "/a", "/b", "/c", "/d", "/e", "/f"}
			pick := func() string { return nodes[rng.Intn(len(nodes))] }
			who := []string{"/ana", "/bo", "/cy"}

			propagated, replaced, deltas := 0, 0, 0
			for step := 0; step < 120; step++ {
				batch := 1 + rng.Intn(4)
				for i := 0; i < batch; i++ {
					var f Fact
					switch pred := deltaWritable[rng.Intn(len(deltaWritable))]; pred {
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
				}
				if err := k.EvaluatePending(); err != nil {
					t.Fatalf("step %d: EvaluatePending: %v", step, err)
				}
				// A batch that writes only predicates every consumer of which
				// is replaced (dl_quarantine is only negated) has nothing to
				// propagate and is a plain cone; never a full fixpoint.
				ev := k.LastEvaluation()
				if ev.Mode != "delta" && ev.Mode != "cone" {
					t.Fatalf("step %d: insert-only batch evaluated as %q (fallback %q), want delta or cone", step, ev.Mode, ev.ConeFallback)
				}
				if ev.Mode == "delta" {
					deltas++
				}
				if ev.DeltaFacts > 0 {
					propagated++
				}
				if ev.Rules > 0 {
					replaced++
				}
				requireSameStore(t, fmt.Sprintf("seed %d step %d", seed, step), dumpStore(t, k), fullOracle(t, k))
			}
			if propagated == 0 || replaced == 0 || deltas < 60 {
				t.Fatalf("walk: %d delta evaluations of 120, propagated %d and replaced %d times; want most batches by delta, both kinds of work", deltas, propagated, replaced)
			}
		})
	}
}

// TestDeltaEvaluationRetractTakesCone pins the boundary: a write set with a
// retraction is not delta-safe and re-derives its cone.
func TestDeltaEvaluationRetractTakesCone(t *testing.T) {
	k := newPolicyKernel(t, newRealKernelBase(), deltaPolicy)
	e := Fact{Predicate: "dl_edge", Args: []any{MangleAtom("/a"), MangleAtom("/b")}}
	if err := k.Assert(e); err != nil {
		t.Fatal(err)
	}
	if _, err := k.Query("dl_reach"); err != nil {
		t.Fatal(err)
	}
	if err := k.RetractExactFact(e); err != nil {
		t.Fatal(err)
	}
	if _, err := k.Query("dl_reach"); err != nil {
		t.Fatal(err)
	}
	if ev := k.LastEvaluation(); ev.Mode != "cone" {
		t.Fatalf("retract evaluated as %q, want cone", ev.Mode)
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

// TestDeltaEvaluationMatchesFullFixpoint_ProductionWorld runs the delta pass
// over the embedded production policy: a world, then insert-only batches of
// new files, each compared on every predicate with a full fixpoint.
func TestDeltaEvaluationMatchesFullFixpoint_ProductionWorld(t *testing.T) {
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
		if ev.Mode != "delta" || ev.DeltaFacts == 0 {
			t.Fatalf("batch %d evaluated as %q with %d derived (fallback %q), want delta deriving rows", step, ev.Mode, ev.DeltaFacts, ev.ConeFallback)
		}
		requireSameStore(t, fmt.Sprintf("batch %d", step), dumpStore(t, k), fullOracle(t, k))
	}
}

// BenchmarkProductionWorldInsertPending measures the lazy path for an
// insert-only batch over a 48K-file world with the production policy: the
// write that cost 8-16 s per boot through the cone path.
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
			b.ReportMetric(float64(ev.DeltaFacts), "derived")
			b.Logf("mode=%s delta=%v replace(prepare=%v engine=%v rules=%d) fallback=%q",
				ev.Mode, ev.DeltaDuration, ev.PrepareDuration, ev.EngineDuration, ev.Rules, ev.ConeFallback)
		})
	}
}

// A delta pass that fails part-way has already added rows to the store. Its
// retry must not trust them as "already present": the store is tainted and
// only a full evaluation, which builds a fresh one, runs next.
func TestFailedDeltaTaintsTheStoreUntilAFullEvaluation(t *testing.T) {
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
	if ev := k.LastEvaluation(); ev.Mode != "delta" {
		t.Fatalf("after a full repair the next insert evaluated as %q, want delta again", ev.Mode)
	}
	requireSameStore(t, "after the next insert", dumpStore(t, k), fullOracle(t, k))
}
