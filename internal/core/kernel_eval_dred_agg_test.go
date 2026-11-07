package core

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

// aggPolicy has every aggregate shape the delta path maintains (single-atom
// and multi-premise bodies, count, sum, both, a negation in the body, no group
// key) beside the shapes that keep the group recompute (max, a float sum, a
// transform with a let over a reducer, a count that never reaches the head),
// and a rule downstream of a maintained one.
const aggPolicy = `
	Decl ag_item(Owner, Id, V).
	Decl ag_link(Importer, File, W).
	Decl ag_file(File, Kind).
	Decl ag_dead(Id).
	Decl ag_reading(Sensor, X).

	Decl ag_stats(Owner, N, S).
	Decl ag_total(Owner, S).
	Decl ag_members(Owner, N).
	Decl ag_dependents(File, N).
	Decl ag_net(File, N, S).
	Decl ag_live_total(Owner, S).
	Decl ag_all_count(N).
	Decl ag_busy(Owner).
	Decl ag_range(Owner, Hi).
	Decl ag_float_total(Sensor, T).
	Decl ag_bumped(Owner, M).
	Decl ag_hidden(Owner, S).

	ag_stats(O, N, S) :- ag_item(O, _, V) |> do fn:group_by(O), let N = fn:count(), let S = fn:sum(V).
	ag_total(O, S) :- ag_item(O, _, V) |> do fn:group_by(O), let S = fn:sum(V).
	ag_members(O, N) :- ag_item(O, _, _) |> do fn:group_by(O), let N = fn:count().
	ag_dependents(F, N) :- ag_link(I, F, _), ag_file(F, _) |> do fn:group_by(F), let N = fn:count().
	ag_net(F, N, S) :- ag_link(I, F, W), ag_file(F, _) |> do fn:group_by(F), let N = fn:count(), let S = fn:sum(W).
	ag_live_total(O, S) :- ag_item(O, K, V), !ag_dead(K) |> do fn:group_by(O), let S = fn:sum(V).
	ag_all_count(N) :- ag_item(_, _, _) |> do fn:group_by(), let N = fn:count().
	ag_busy(O) :- ag_stats(O, N, _), N >= 3.

	ag_range(O, Hi) :- ag_item(O, _, V) |> do fn:group_by(O), let Hi = fn:max(V).
	ag_float_total(S, T) :- ag_reading(S, X) |> do fn:group_by(S), let T = fn:float:sum(X).
	ag_bumped(O, M) :- ag_item(O, _, _) |> do fn:group_by(O), let N = fn:count(), let M = fn:plus(N, 1).
	ag_hidden(O, S) :- ag_item(O, _, V) |> do fn:group_by(O), let N = fn:count(), let S = fn:sum(V).
`

func agItem(o, id string, v int64) Fact {
	return Fact{Predicate: "ag_item", Args: []any{MangleAtom(o), MangleAtom(id), v}}
}
func agLink(i, f string, w int64) Fact {
	return Fact{Predicate: "ag_link", Args: []any{MangleAtom(i), MangleAtom(f), w}}
}
func agFile(f, kind string) Fact {
	return Fact{Predicate: "ag_file", Args: []any{MangleAtom(f), MangleAtom(kind)}}
}
func agDead(id string) Fact { return Fact{Predicate: "ag_dead", Args: []any{MangleAtom(id)}} }
func agReading(s string, x float64) Fact {
	return Fact{Predicate: "ag_reading", Args: []any{MangleAtom(s), x}}
}

// aggKernel is a kernel over aggPolicy, evaluated once.
func aggKernel(t *testing.T) *RealKernel {
	t.Helper()
	k := newRealKernelBase()
	k.AppendPolicy(aggPolicy)
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	return k
}

// aggWrite is a batch of writes: asserts, then retracts.
type aggWrite struct {
	assert, retract []Fact
}

// applyAgg writes the batch, evaluates it lazily and requires the store to
// equal a full fixpoint's. It returns the evaluation's stats.
func applyAgg(t *testing.T, k *RealKernel, name string, w aggWrite) EvaluationStats {
	t.Helper()
	for _, f := range w.retract {
		if err := k.RetractExactFact(f); err != nil {
			t.Fatalf("%s: retract %s: %v", name, f.String(), err)
		}
	}
	for _, f := range w.assert {
		if err := k.AssertWithoutEval(f); err != nil {
			t.Fatalf("%s: assert %s: %v", name, f.String(), err)
		}
	}
	if err := k.EvaluatePending(); err != nil {
		t.Fatalf("%s: EvaluatePending: %v", name, err)
	}
	ev := k.LastEvaluation()
	requireSameStore(t, name, dumpStore(t, k), fullOracle(t, k))
	return ev
}

// rowsOf returns the rows of a predicate, as strings.
func rowsOf(t *testing.T, k *RealKernel, pred string) []string {
	t.Helper()
	facts, err := k.Query(pred)
	if err != nil {
		t.Fatalf("Query %s: %v", pred, err)
	}
	out := make([]string, len(facts))
	for i, f := range facts {
		out[i] = strings.TrimSuffix(f.String(), ".")
	}
	return out
}

func requireRows(t *testing.T, k *RealKernel, pred string, want ...string) {
	t.Helper()
	got := rowsOf(t, k, pred)
	if !sameSet(got, want) {
		t.Fatalf("%s = %v; want %v", pred, got, want)
	}
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]int, len(a))
	for _, x := range a {
		seen[x]++
	}
	for _, x := range b {
		seen[x]--
	}
	for _, n := range seen {
		if n != 0 {
			return false
		}
	}
	return true
}

// requireDelta fails unless the evaluation was a DRed pass that kept at least
// one group by delta.
func requireDelta(t *testing.T, name string, ev EvaluationStats) {
	t.Helper()
	if ev.Mode != "dred" || ev.AggByDelta == 0 {
		t.Fatalf("%s: evaluated as %q with %d groups by delta (fallback %q); want a dred pass maintaining groups by delta", name, ev.Mode, ev.AggByDelta, ev.ConeFallback)
	}
}

// The reducers the delta path takes, and the shapes that keep the group
// recompute, read off the production index built for the policy.
func TestDRedAggregateShapesChooseDeltaOrRecompute(t *testing.T) {
	k := aggKernel(t)
	idx := k.cone.dredIndex(k.programInfo)
	byHead := func(head string) *dredAggregate {
		for _, ri := range idx.byHead[head] {
			return idx.rules[ri].agg
		}
		t.Fatalf("no rule for %s", head)
		return nil
	}
	for _, head := range []string{"ag_stats", "ag_total", "ag_members", "ag_dependents", "ag_net", "ag_live_total", "ag_all_count"} {
		if agg := byHead(head); agg == nil || len(agg.reducers) == 0 {
			t.Errorf("%s is counts and integer sums and must be maintained by delta", head)
		}
	}
	for _, head := range []string{"ag_range", "ag_float_total", "ag_bumped", "ag_hidden"} {
		if agg := byHead(head); agg != nil && len(agg.reducers) > 0 {
			t.Errorf("%s needs its group's rows and must keep the recompute", head)
		}
	}
}

// A walk of random writes over every shape: after each batch the store equals
// a full fixpoint, and over the walk most groups are kept by delta while the
// shapes that need rows are recomputed.
func TestDRedAggregateDeltaMatchesFullFixpoint(t *testing.T) {
	for _, seed := range []int64{1, 7, 42, 20261106} {
		t.Run(fmt.Sprintf("seed-%d", seed), func(t *testing.T) {
			rng := rand.New(rand.NewSource(seed))
			k := aggKernel(t)
			owners := []string{"/o1", "/o2", "/o3"}
			files := []string{"/f1", "/f2", "/f3"}
			importers := []string{"/i1", "/i2", "/i3", "/i4"}
			ids := []string{"/a", "/b", "/c", "/d", "/e", "/f"}
			sensors := []string{"/s1", "/s2"}
			pick := func(xs []string) string { return xs[rng.Intn(len(xs))] }
			var held []Fact
			delta, recomputed, dreds := 0, 0, 0
			for step := 0; step < 250; step++ {
				var w aggWrite
				for i, n := 0, 1+rng.Intn(4); i < n; i++ {
					if len(held) > 0 && rng.Intn(3) == 0 {
						j := rng.Intn(len(held))
						w.retract = append(w.retract, held[j])
						held = append(held[:j], held[j+1:]...)
						continue
					}
					var f Fact
					switch rng.Intn(5) {
					case 0, 1:
						f = agItem(pick(owners), pick(ids), int64(rng.Intn(7)-3))
					case 2:
						f = agLink(pick(importers), pick(files), int64(1+rng.Intn(2)))
					case 3:
						if rng.Intn(2) == 0 {
							f = agFile(pick(files), pick([]string{"/go", "/py"}))
						} else {
							f = agDead(pick(ids))
						}
					default:
						f = agReading(pick(sensors), float64(rng.Intn(4))/2)
					}
					w.assert = append(w.assert, f)
					held = append(held, f)
				}
				// A fact asserted twice in one batch is one row; a retract of
				// a row asserted in the same batch is not a write order the
				// walk models, so retracts go first.
				ev := applyAgg(t, k, fmt.Sprintf("seed %d step %d", seed, step), w)
				if ev.Mode == "dred" {
					dreds++
					delta += ev.AggByDelta
					recomputed += ev.AggRecomputed
				}
			}
			if dreds < 150 || delta < 100 || recomputed == 0 {
				t.Fatalf("walk: %d dred evaluations of 250, %d groups by delta, %d recomputed; want most batches by dred, most groups by delta and the recompute exercised",
					dreds, delta, recomputed)
			}
		})
	}
}

// A row with two derivations is one row: losing one derivation of it leaves
// the count alone, and only losing the last removes it.
func TestDRedAggregateDuplicateSupportsMoveTheCountOnce(t *testing.T) {
	k := aggKernel(t)
	steps := []struct {
		name string
		w    aggWrite
		want []string // ag_dependents(/f, N), nothing: no row
	}{
		{"two files (kinds) and two importers", aggWrite{assert: []Fact{agFile("/f", "/go"), agFile("/f", "/py"), agLink("/i1", "/f", 1), agLink("/i2", "/f", 1)}}, []string{"ag_dependents(/f, 2)"}},
		{"i1 links again with another weight: the same row", aggWrite{assert: []Fact{agLink("/i1", "/f", 2)}}, []string{"ag_dependents(/f, 2)"}},
		{"one of i1's two links goes", aggWrite{retract: []Fact{agLink("/i1", "/f", 1)}}, []string{"ag_dependents(/f, 2)"}},
		{"one of the two file kinds goes", aggWrite{retract: []Fact{agFile("/f", "/go")}}, []string{"ag_dependents(/f, 2)"}},
		{"i1's last link goes", aggWrite{retract: []Fact{agLink("/i1", "/f", 2)}}, []string{"ag_dependents(/f, 1)"}},
		{"i2 goes: the group is empty", aggWrite{retract: []Fact{agLink("/i2", "/f", 1)}}, nil},
		{"links return, the last file kind goes", aggWrite{assert: []Fact{agLink("/i3", "/f", 1)}, retract: []Fact{agFile("/f", "/py")}}, nil},
		{"the file returns", aggWrite{assert: []Fact{agFile("/f", "/rb")}}, []string{"ag_dependents(/f, 1)"}},
	}
	for _, s := range steps {
		ev := applyAgg(t, k, s.name, s.w)
		if ev.Mode != "dred" {
			t.Fatalf("%s: evaluated as %q", s.name, ev.Mode)
		}
		requireRows(t, k, "ag_dependents", s.want...)
	}
	if ev := k.LastEvaluation(); ev.AggRecomputed != 0 {
		t.Fatalf("a maintained count recomputed %d groups", ev.AggRecomputed)
	}
}

// A group that loses its last member has no output: never a zero count, and
// never a zero sum (a sum alone does not know its row count).
func TestDRedAggregateEmptyGroupHasNoOutput(t *testing.T) {
	k := aggKernel(t)
	ev := applyAgg(t, k, "two members", aggWrite{assert: []Fact{agItem("/o1", "/a", 2), agItem("/o1", "/b", -2), agItem("/o2", "/a", 5)}})
	requireDelta(t, "two members", ev)
	requireRows(t, k, "ag_stats", "ag_stats(/o1, 2, 0)", "ag_stats(/o2, 1, 5)")
	// The sum is zero with rows left: an output of 0, not a vanished group.
	requireRows(t, k, "ag_total", "ag_total(/o1, 0)", "ag_total(/o2, 5)")
	requireRows(t, k, "ag_all_count", "ag_all_count(3)")

	ev = applyAgg(t, k, "one leaves", aggWrite{retract: []Fact{agItem("/o1", "/a", 2)}})
	requireDelta(t, "one leaves", ev)
	requireRows(t, k, "ag_total", "ag_total(/o1, -2)", "ag_total(/o2, 5)")

	ev = applyAgg(t, k, "the last leaves", aggWrite{retract: []Fact{agItem("/o1", "/b", -2)}})
	requireDelta(t, "the last leaves", ev)
	requireRows(t, k, "ag_stats", "ag_stats(/o2, 1, 5)")
	requireRows(t, k, "ag_total", "ag_total(/o2, 5)")
	requireRows(t, k, "ag_members", "ag_members(/o2, 1)")
	requireRows(t, k, "ag_all_count", "ag_all_count(1)")

	// A member whose value is zero leaves no sum behind either.
	applyAgg(t, k, "zero member", aggWrite{assert: []Fact{agItem("/o3", "/z", 0)}})
	requireRows(t, k, "ag_total", "ag_total(/o2, 5)", "ag_total(/o3, 0)")
	applyAgg(t, k, "zero member leaves", aggWrite{retract: []Fact{agItem("/o3", "/z", 0)}})
	requireRows(t, k, "ag_total", "ag_total(/o2, 5)")

	applyAgg(t, k, "everything leaves", aggWrite{retract: []Fact{agItem("/o2", "/a", 5)}})
	requireRows(t, k, "ag_all_count")
	requireRows(t, k, "ag_stats")
}

// A sum goes up and down, through zero and back, one write at a time and in
// batches that add and remove members of one group together.
func TestDRedAggregateSumGoesUpAndDown(t *testing.T) {
	k := aggKernel(t)
	steps := []struct {
		name string
		w    aggWrite
		want string
	}{
		{"first", aggWrite{assert: []Fact{agItem("/o1", "/a", 5)}}, "ag_total(/o1, 5)"},
		{"up", aggWrite{assert: []Fact{agItem("/o1", "/b", 7)}}, "ag_total(/o1, 12)"},
		{"down by a negative member", aggWrite{assert: []Fact{agItem("/o1", "/c", -20)}}, "ag_total(/o1, -8)"},
		{"a member leaves", aggWrite{retract: []Fact{agItem("/o1", "/a", 5)}}, "ag_total(/o1, -13)"},
		{"swap members in one batch", aggWrite{assert: []Fact{agItem("/o1", "/d", 13)}, retract: []Fact{agItem("/o1", "/c", -20)}}, "ag_total(/o1, 20)"},
		{"same value in, same value out", aggWrite{assert: []Fact{agItem("/o1", "/e", 4)}, retract: []Fact{agItem("/o1", "/b", 7), agItem("/o1", "/d", 13)}}, "ag_total(/o1, 4)"},
		{"to zero with one member left", aggWrite{assert: []Fact{agItem("/o1", "/f", -4)}}, "ag_total(/o1, 0)"},
	}
	for _, s := range steps {
		ev := applyAgg(t, k, s.name, s.w)
		requireDelta(t, s.name, ev)
		requireRows(t, k, "ag_total", s.want)
		requireRows(t, k, "ag_stats", strings.Replace(s.want, "ag_total(/o1, ", "ag_stats(/o1, "+fmt.Sprint(len(rowsOf(t, k, "ag_item")))+", ", 1))
	}
	if ev := k.LastEvaluation(); ev.AggRecomputed != 0 && ev.Mode == "dred" {
		// ag_range and the other recompute shapes read these items too.
		if ev.AggByDelta == 0 {
			t.Fatalf("no group by delta in the last pass")
		}
	}
}

// A negation in the body moves rows in and out of the group: a freshly dead id
// stops counting, a revived one counts again, and a row's other support (the
// same id under another owner) is untouched.
func TestDRedAggregateNegatedBodyMovesRows(t *testing.T) {
	k := aggKernel(t)
	applyAgg(t, k, "items", aggWrite{assert: []Fact{agItem("/o1", "/a", 3), agItem("/o1", "/b", 4), agItem("/o2", "/a", 10)}})
	requireRows(t, k, "ag_live_total", "ag_live_total(/o1, 7)", "ag_live_total(/o2, 10)")
	ev := applyAgg(t, k, "a dies", aggWrite{assert: []Fact{agDead("/a")}})
	requireDelta(t, "a dies", ev)
	requireRows(t, k, "ag_live_total", "ag_live_total(/o1, 4)")
	applyAgg(t, k, "b dies: both groups empty", aggWrite{assert: []Fact{agDead("/b")}})
	requireRows(t, k, "ag_live_total")
	applyAgg(t, k, "a revives", aggWrite{retract: []Fact{agDead("/a")}})
	requireRows(t, k, "ag_live_total", "ag_live_total(/o1, 3)", "ag_live_total(/o2, 10)")
}

// A multi-premise count and sum together: rows are distinct tuples of the
// body's variables, so a weight difference makes a distinct row even where
// the importer is the same.
func TestDRedAggregateMultiPremiseCountAndSum(t *testing.T) {
	k := aggKernel(t)
	applyAgg(t, k, "file and links", aggWrite{assert: []Fact{agFile("/f", "/go"), agLink("/i1", "/f", 2), agLink("/i1", "/f", 5), agLink("/i2", "/f", 5)}})
	requireRows(t, k, "ag_net", "ag_net(/f, 3, 12)")
	requireRows(t, k, "ag_dependents", "ag_dependents(/f, 2)")
	ev := applyAgg(t, k, "a second file kind doubles the support of every row, not the rows", aggWrite{assert: []Fact{agFile("/f", "/py")}})
	if ev.Mode != "dred" {
		t.Fatalf("evaluated as %q", ev.Mode)
	}
	requireRows(t, k, "ag_net", "ag_net(/f, 3, 12)")
	applyAgg(t, k, "an importer leaves", aggWrite{retract: []Fact{agLink("/i1", "/f", 5)}})
	requireRows(t, k, "ag_net", "ag_net(/f, 2, 7)")
	requireRows(t, k, "ag_dependents", "ag_dependents(/f, 2)")
	applyAgg(t, k, "both kinds leave", aggWrite{retract: []Fact{agFile("/f", "/go"), agFile("/f", "/py")}})
	requireRows(t, k, "ag_net")
	requireRows(t, k, "ag_dependents")
}

// A clone starts from its parent's store and keeps nothing else of the
// aggregates: its own writes are maintained exactly, and neither side's
// groups move for the other's writes.
func TestDRedAggregateCloneMaintainsItsOwnGroups(t *testing.T) {
	k := aggKernel(t)
	applyAgg(t, k, "parent world", aggWrite{assert: []Fact{
		agItem("/o1", "/a", 1), agItem("/o1", "/b", 2), agFile("/f", "/go"), agLink("/i1", "/f", 1), agLink("/i2", "/f", 1),
	}})
	before := dumpStore(t, k)

	clone := k.Clone()
	ev := applyAgg(t, clone, "clone grows", aggWrite{assert: []Fact{agItem("/o1", "/c", 10), agLink("/i3", "/f", 1)}})
	requireDelta(t, "clone grows", ev)
	requireRows(t, clone, "ag_total", "ag_total(/o1, 13)")
	requireRows(t, clone, "ag_dependents", "ag_dependents(/f, 3)")
	ev = applyAgg(t, clone, "clone shrinks", aggWrite{retract: []Fact{agItem("/o1", "/a", 1), agItem("/o1", "/b", 2), agItem("/o1", "/c", 10)}})
	requireDelta(t, "clone shrinks", ev)
	requireRows(t, clone, "ag_total")

	requireSameStore(t, "the parent after its clone's writes", dumpStore(t, k), before)
	requireRows(t, k, "ag_total", "ag_total(/o1, 3)")

	// The parent keeps maintaining its own, over a store its clone wrapped.
	ev = applyAgg(t, k, "parent moves on", aggWrite{assert: []Fact{agItem("/o1", "/d", 100)}, retract: []Fact{agLink("/i1", "/f", 1)}})
	requireDelta(t, "parent moves on", ev)
	requireRows(t, k, "ag_total", "ag_total(/o1, 103)")
	requireRows(t, k, "ag_dependents", "ag_dependents(/f, 1)")
	requireRows(t, clone, "ag_dependents", "ag_dependents(/f, 3)")

	// A clone of a clone, taken after both moved.
	grand := clone.Clone()
	applyAgg(t, grand, "grandchild", aggWrite{assert: []Fact{agItem("/o9", "/x", 9)}, retract: []Fact{agLink("/i2", "/f", 1)}})
	requireRows(t, grand, "ag_total", "ag_total(/o9, 9)")
	requireRows(t, grand, "ag_dependents", "ag_dependents(/f, 2)")
	requireRows(t, clone, "ag_dependents", "ag_dependents(/f, 3)")
}

// A policy change replaces the store; the groups the next write touches are
// maintained over the rebuilt one, and so are they after an evaluation that
// held a row beside an aggregate (which only the cone path takes).
func TestDRedAggregateSurvivesPolicyChangeAndConeEvaluation(t *testing.T) {
	k := aggKernel(t)
	applyAgg(t, k, "world", aggWrite{assert: []Fact{agItem("/o1", "/a", 1), agItem("/o1", "/b", 2), agFile("/f", "/go"), agLink("/i1", "/f", 1)}})
	applyAgg(t, k, "a write", aggWrite{assert: []Fact{agItem("/o1", "/c", 4)}})

	k.AppendPolicy(`
		Decl ag_late(Owner, N).
		ag_late(O, N) :- ag_item(O, _, _) |> do fn:group_by(O), let N = fn:count().
	`)
	if err := k.Evaluate(); err != nil {
		t.Fatal(err)
	}
	ev := applyAgg(t, k, "after a policy change", aggWrite{assert: []Fact{agItem("/o1", "/d", 8)}, retract: []Fact{agItem("/o1", "/a", 1)}})
	requireDelta(t, "after a policy change", ev)
	requireRows(t, k, "ag_late", "ag_late(/o1, 3)")
	requireRows(t, k, "ag_total", "ag_total(/o1, 14)")
	// A row asserted beside a maintained aggregate sends every pass that
	// reaches that head to the cone; the other heads stay maintained, and the
	// head the cone re-derives (its scratch relation rebuilt by the engine)
	// agrees with the full fixpoint through writes and the row's retraction.
	applyAgg(t, k, "link by delta", aggWrite{assert: []Fact{agLink("/i2", "/f", 1)}})
	stray := Fact{Predicate: "ag_dependents", Args: []any{MangleAtom("/zz"), int64(99)}}
	ev = applyAgg(t, k, "a row beside an aggregate", aggWrite{assert: []Fact{stray, agLink("/i3", "/f", 1)}})
	if ev.Mode == "dred" {
		t.Fatalf("a row asserted beside an aggregate evaluated as dred")
	}
	applyAgg(t, k, "cone again", aggWrite{assert: []Fact{agLink("/i4", "/f", 1)}, retract: []Fact{agLink("/i2", "/f", 1)}})
	requireRows(t, k, "ag_dependents", "ag_dependents(/f, 3)", "ag_dependents(/zz, 99)")
	applyAgg(t, k, "the stray goes", aggWrite{retract: []Fact{stray}})
	requireRows(t, k, "ag_dependents", "ag_dependents(/f, 3)")
	ev = applyAgg(t, k, "other heads stay maintained", aggWrite{assert: []Fact{agItem("/o1", "/g", 1)}, retract: []Fact{agItem("/o1", "/b", 2)}})
	requireDelta(t, "other heads stay maintained", ev)
	requireRows(t, k, "ag_total", "ag_total(/o1, 13)")
}

// A summand that is not an integer sends its group to the recompute, which
// fails as the full fixpoint does; the store is then tainted and the full
// path repairs it.
func TestDRedAggregateNonIntegerSummandIsLeftToTheRecompute(t *testing.T) {
	k := newRealKernelBase()
	k.AppendPolicy(`
		Decl nz_item(Owner, V).
		Decl nz_total(Owner, S).
		nz_total(O, S) :- nz_item(O, V) |> do fn:group_by(O), let S = fn:sum(V).
	`)
	if err := k.Evaluate(); err != nil {
		t.Fatal(err)
	}
	if err := k.Assert(Fact{Predicate: "nz_item", Args: []any{MangleAtom("/o"), int64(3)}}); err != nil {
		t.Fatal(err)
	}
	if err := k.AssertWithoutEval(Fact{Predicate: "nz_item", Args: []any{MangleAtom("/o"), "four"}}); err != nil {
		t.Fatal(err)
	}
	// The engine rejects the sum of an integer and a string; so must the pass.
	if err := k.EvaluatePending(); err == nil {
		t.Fatal("a sum over a string summand evaluated; the engine rejects it")
	}
}

// BenchmarkDRedAggregateOneChange's claim as a test: one change to a group of
// 10,000 members visits no more than the changed row, so the maintained
// pass's work does not follow the group's size.
func TestDRedAggregateOneChangeDoesNotVisitTheGroup(t *testing.T) {
	var variants [2]int
	for i, size := range []int{4, 2000} {
		k := newRealKernelBase()
		k.AppendPolicy(aggBenchPolicy)
		_ = k.AssertWithoutEval(agFileBench("/hot"))
		for m := 0; m < size; m++ {
			if err := k.AssertWithoutEval(Fact{Predicate: "ab_link", Args: []any{MangleAtom(fmt.Sprintf("/i%d", m)), MangleAtom("/hot"), "via"}}); err != nil {
				t.Fatal(err)
			}
			if err := k.AssertWithoutEval(Fact{Predicate: "ab_item", Args: []any{MangleAtom("/hot"), MangleAtom(fmt.Sprintf("/m%d", m)), int64(m % 5)}}); err != nil {
				t.Fatal(err)
			}
		}
		if err := k.Evaluate(); err != nil {
			t.Fatal(err)
		}
		if err := k.AssertWithoutEval(Fact{Predicate: "ab_link", Args: []any{MangleAtom("/new"), MangleAtom("/hot"), "via"}}); err != nil {
			t.Fatal(err)
		}
		if err := k.AssertWithoutEval(Fact{Predicate: "ab_item", Args: []any{MangleAtom("/hot"), MangleAtom("/new"), int64(1)}}); err != nil {
			t.Fatal(err)
		}
		if err := k.EvaluatePending(); err != nil {
			t.Fatal(err)
		}
		ev := k.LastEvaluation()
		if ev.Mode != "dred" || ev.AggByDelta != 2 || ev.AggRecomputed != 0 {
			t.Fatalf("size %d: %q, %d groups by delta, %d recomputed; want dred, 2 by delta, none recomputed", size, ev.Mode, ev.AggByDelta, ev.AggRecomputed)
		}
		variants[i] = ev.DeltaVariants
		rows := rowsOf(t, k, "ab_dependent_count")
		if want := fmt.Sprintf("ab_dependent_count(/hot, %d)", size+1); len(rows) != 1 || rows[0] != want {
			t.Fatalf("size %d: ab_dependent_count = %v; want %s", size, rows, want)
		}
	}
	if variants[0] != variants[1] {
		t.Fatalf("variants evaluated: %d for a group of 4, %d for one of 2000; want the same", variants[0], variants[1])
	}
}

func agFileBench(f string) Fact {
	return Fact{Predicate: "ab_file", Args: []any{MangleAtom(f), "go"}}
}
