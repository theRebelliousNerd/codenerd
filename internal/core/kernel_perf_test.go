package core

import (
	"fmt"
	"math"
	"reflect"
	"slices"
	"sort"
	"testing"
	"time"

	"codeberg.org/TauCeti/mangle-go/analysis"
	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/factstore"
)

// fullOracle evaluates a clone of k from scratch: every rule over a fresh
// store built from the EDB. It is the reference a cone evaluation must equal.
func fullOracle(t *testing.T, k *RealKernel) []string {
	t.Helper()
	ref := k.Clone()
	if err := ref.Evaluate(); err != nil {
		t.Fatalf("oracle Evaluate: %v", err)
	}
	return dumpStore(t, ref)
}

func requireSameStore(t *testing.T, stage string, got, want []string) {
	t.Helper()
	if slices.Equal(got, want) {
		return
	}
	i := 0
	for i < len(got) && i < len(want) && got[i] == want[i] {
		i++
	}
	g, w := "<end>", "<end>"
	if i < len(got) {
		g = got[i]
	}
	if i < len(want) {
		w = want[i]
	}
	t.Fatalf("%s: store has %d facts, full fixpoint %d; first difference at %d: %q vs %q", stage, len(got), len(want), i, g, w)
}

// A clone always gets the simple store (BenchmarkCloneStoreKind ruled the
// indexed copy out), whatever its parent's kind. On either store kind a cone
// evaluation holds exactly what a full fixpoint holds -- through asserts, a
// negated premise turning true, and retractions -- and a clone's writes never
// reach its parent.
func TestCloneStoreKindAndConeEquivalence(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		edges      int
		parentKind string
	}{{8, "simple"}, {600, "indexed"}} {
		t.Run(fmt.Sprintf("edges=%d", tc.edges), func(t *testing.T) {
			parent := newPerfKernel(t, tc.edges)
			before := dumpStore(t, parent)
			if got := storeKind(parent.store); got != tc.parentKind {
				t.Fatalf("parent store kind = %s, want %s", got, tc.parentKind)
			}
			c := parent.Clone()
			steps := []struct {
				name  string
				write func(*RealKernel) error
			}{
				{"stage quiet", func(k *RealKernel) error {
					return k.AssertWithoutEval(Fact{Predicate: "perf_quiet", Args: []any{"n1"}})
				}},
				{"stage probe (negation turns)", func(k *RealKernel) error {
					return k.AssertWithoutEval(Fact{Predicate: "perf_probe", Args: []any{"n1"}})
				}},
				{"retract an edge", func(k *RealKernel) error {
					return k.RetractExactFact(Fact{Predicate: "perf_edge", Args: []any{"n1", fmt.Sprintf("n%d", 8%tc.edges)}})
				}},
				{"retract probe", func(k *RealKernel) error { return k.Retract("perf_probe") }},
			}
			apply := func(who string, k *RealKernel, kind string) {
				for _, step := range steps {
					if err := step.write(k); err != nil {
						t.Fatalf("%s %s: %v", who, step.name, err)
					}
					if err := k.EvaluatePending(); err != nil {
						t.Fatalf("%s %s: EvaluatePending: %v", who, step.name, err)
					}
					if ev := k.LastEvaluation(); ev.Mode != "cone" || ev.StoreKind != kind {
						t.Fatalf("%s %s: evaluation mode=%q store=%q, want cone over %s (fallback %q)", who, step.name, ev.Mode, ev.StoreKind, kind, ev.ConeFallback)
					}
					requireSameStore(t, who+" "+step.name, dumpStore(t, k), fullOracle(t, k))
				}
			}
			apply("clone", c, "simple")
			requireSameStore(t, "parent after clone writes", dumpStore(t, parent), before)
			apply("parent", parent, tc.parentKind)
		})
	}
}

// The store still holds the atom of a row retracted since the last
// evaluation. When that row was the last of an undeclared predicate at its
// arity, only the EDB index's arity history names the symbol to remove.
func TestConeRemovesRetractedUndeclaredArity(t *testing.T) {
	k := newPerfKernel(t, 8)
	odd := Fact{Predicate: "perf_undeclared", Args: []any{"a", "b", "c"}}
	if err := k.Assert(odd); err != nil {
		t.Fatal(err)
	}
	if _, err := k.Query("perf_hit"); err != nil {
		t.Fatal(err)
	}
	if err := k.RetractExactFact(odd); err != nil {
		t.Fatal(err)
	}
	if _, err := k.Query("perf_hit"); err != nil {
		t.Fatal(err)
	}
	if ev := k.LastEvaluation(); ev.Mode != "cone" || ev.RemovedFacts != 1 {
		t.Fatalf("retraction evaluated as %q removing %d rows, want a cone removing 1", ev.Mode, ev.RemovedFacts)
	}
	requireSameStore(t, "after retract", dumpStore(t, k), fullOracle(t, k))
}

// coneScanReference is the cone preparation the plan replaced: a scan of
// every rule and stratum for members of the cone.
func coneScanReference(k *RealKernel, cone map[string]struct{}) ([]ast.Clause, []analysis.Nodeset) {
	var rules []ast.Clause
	for _, rule := range k.programInfo.Rules {
		if _, ok := cone[rule.Head.Predicate.Symbol]; ok {
			rules = append(rules, rule)
		}
	}
	var strata []analysis.Nodeset
	for _, stratum := range k.strata {
		for sym := range stratum {
			if _, ok := cone[sym.Symbol]; ok {
				strata = append(strata, stratum)
				break
			}
		}
	}
	return rules, strata
}

// On the production corpus, a cone plan selects exactly the rules and strata,
// in exactly the order, that a scan of the whole program does; it names every
// symbol the store holds under its touched predicates; and it is cached.
func TestConePlanMatchesProgramScanOnProductionCorpus(t *testing.T) {
	t.Parallel()
	k, err := NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	if err := k.Assert(Fact{Predicate: "user_intent", Args: []any{"/current_intent", "/mutation", "/fix", "x.go", ""}}); err != nil {
		t.Fatal(err)
	}
	if _, err := k.Query("next_action"); err != nil {
		t.Fatal(err)
	}
	k.mu.RLock()
	defer k.mu.RUnlock()
	inStore := make(map[string][]ast.PredicateSym)
	for _, sym := range k.store.ListPredicates() {
		inStore[sym.Symbol] = append(inStore[sym.Symbol], sym)
	}
	for _, written := range []map[string]struct{}{
		{"user_intent": {}},
		{"pending_action": {}, "action_verified": {}},
		{"file_topology": {}},
		{"tool_execution": {}},
		{"world_model_heartbeat": {}},
	} {
		plan := k.cone.plan(written)
		if again := k.cone.plan(written); again != plan {
			t.Fatalf("%v: plan was not cached", written)
		}
		rules, strata := coneScanReference(k, plan.cone)
		if !reflect.DeepEqual(plan.rules, rules) && !(len(plan.rules) == 0 && len(rules) == 0) {
			t.Fatalf("%v: plan selected %d rules, the program scan %d (or a different order)", written, len(plan.rules), len(rules))
		}
		if !reflect.DeepEqual(plan.strata, strata) && !(len(plan.strata) == 0 && len(strata) == 0) {
			t.Fatalf("%v: plan selected %d strata, the program scan %d (or a different order)", written, len(plan.strata), len(strata))
		}
		for _, pred := range plan.touched {
			named := k.cone.symbolsOf(pred, k.factIndex.aritiesOf(pred))
			for _, sym := range inStore[pred] {
				if !slices.Contains(named, sym) {
					t.Fatalf("%v: store holds %v, which the plan cannot name for removal", written, sym)
				}
			}
		}
	}
}

// A named write that cannot take the cone path says why.
func TestEvaluationStatsNameTheConeFallback(t *testing.T) {
	k := newPerfKernel(t, 8)
	if ev := k.LastEvaluation(); ev.Mode != "full" || ev.EngineDuration <= 0 || ev.Rules != 3 {
		t.Fatalf("explicit Evaluate recorded %+v, want a full evaluation of 3 rules", ev)
	}
	k.EnableProvenance()
	if err := k.Assert(Fact{Predicate: "perf_quiet", Args: []any{"n2"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := k.Query("perf_quiet_seen"); err != nil {
		t.Fatal(err)
	}
	if ev := k.LastEvaluation(); ev.Mode != "full" || ev.ConeFallback != "provenance recording" {
		t.Fatalf("provenance-recording write recorded mode=%q fallback=%q", ev.Mode, ev.ConeFallback)
	}
}

type namedText string

// retractionFixture stores every value of a zoo whose equality argsEqual
// decides across Go types (or refuses to), keyed by its position so values the
// dedupe index would collapse are all held.
func retractionFixture() []Fact {
	values := []any{
		"a", MangleAtom("a"), "/b", MangleAtom("/b"),
		int(1), int64(1), int32(1), uint(1), uint64(1), uint32(1), int8(1),
		float64(0), math.Copysign(0, -1), float64(1), math.NaN(), float32(1),
		true, false, nil,
		[]any{"x", int64(1)}, map[string]any{"k": "v"}, namedText("a"),
		time.Duration(5),
	}
	var facts []Fact
	for i, v := range values {
		facts = append(facts,
			Fact{Predicate: "zoo2", Args: []any{v, i}},
			Fact{Predicate: "zoo_other", Args: []any{v, i}},
		)
		for j, w := range values {
			if (i+j)%4 == 0 {
				facts = append(facts, Fact{Predicate: "zoo3", Args: []any{v, w, i*100 + j}})
			}
		}
	}
	return facts
}

// sequentialRetractReference applies retractions the way Commit did before
// they were compiled: one filter pass per retraction, in phase order.
func sequentialRetractReference(facts []Fact, preds []string, set map[string]struct{}, first, exact []Fact) []Fact {
	keep := func(cur []Fact, pred func(Fact) bool) []Fact {
		var out []Fact
		for _, f := range cur {
			if pred(f) {
				out = append(out, f)
			}
		}
		return out
	}
	cur := slices.Clone(facts)
	for _, p := range preds {
		cur = keep(cur, func(f Fact) bool { return f.Predicate != p })
	}
	cur = keep(cur, func(f Fact) bool { _, ok := set[f.Predicate]; return !ok })
	for _, rf := range first {
		cur = keep(cur, func(f Fact) bool {
			return f.Predicate != rf.Predicate || len(f.Args) == 0 || !argsEqual(f.Args[0], rf.Args[0])
		})
	}
	for _, rf := range exact {
		cur = keep(cur, func(f Fact) bool { return f.Predicate != rf.Predicate || !argsSliceEqual(f.Args, rf.Args) })
	}
	return cur
}

// A compiled transaction removes exactly the facts the per-retraction passes
// removed, including every cross-type equality argsEqual draws (int with
// int64, string with MangleAtom, 0 with -0) and none it refuses (int32 with
// int64, NaN with NaN, a named string with a string), and leaves the dedupe
// index equal to a rebuilt one.
func TestCompiledRetractionsMatchSequentialPasses(t *testing.T) {
	facts := retractionFixture()
	k := newRealKernelBase()
	k.mu.Lock()
	for _, f := range facts {
		if k.factIndex.insert(k.canonFact(f), f) {
			k.facts = append(k.facts, f)
		}
	}
	stored := slices.Clone(k.facts)
	k.mu.Unlock()

	first := []Fact{
		{Predicate: "zoo3", Args: []any{int64(1)}},             // int and int64 rows, not int32 or uint
		{Predicate: "zoo3", Args: []any{MangleAtom("/b")}},     // both spellings
		{Predicate: "zoo3", Args: []any{math.Copysign(0, -1)}}, // 0 and -0
		{Predicate: "zoo3", Args: []any{[]any{"x", int64(1)}}}, // structural, unkeyed
		{Predicate: "zoo3", Args: []any{math.NaN()}},           // equals nothing
		{Predicate: "zoo3", Args: []any{namedText("a")}},       // named type, unkeyed
	}
	exact := []Fact{
		{Predicate: "zoo2", Args: []any{MangleAtom("a"), int64(0)}},    // cross-type in both positions
		{Predicate: "zoo2", Args: []any{int64(1), int64(4)}},           // stored as int
		{Predicate: "zoo2", Args: []any{int64(1), int64(6)}},           // stored as int32: kept
		{Predicate: "zoo2", Args: []any{math.NaN(), 14}},               // kept
		{Predicate: "zoo2", Args: []any{float64(0), 12}},               // stored as -0: removed
		{Predicate: "zoo2", Args: []any{map[string]any{"k": "v"}, 20}}, // unkeyed
		{Predicate: "zoo2", Args: []any{namedText("a"), 21}},           // unkeyed
		{Predicate: "zoo2", Args: []any{"a", 21}},                      // string is not namedText: kept
		{Predicate: "zoo2", Args: []any{nil, 18}},
		{Predicate: "zoo2", Args: []any{"a"}}, // wrong arity: nothing
		{Predicate: "zoo_absent", Args: []any{"a"}},
	}
	set := map[string]struct{}{"zoo_none": {}}
	preds := []string{"zoo_other"}

	tx := k.Transaction().(*KernelTransaction)
	for _, rf := range first {
		tx.RetractFact(rf)
	}
	for _, rf := range exact {
		tx.RetractExactFact(rf)
	}
	tx.RetractPredicateSet(set)
	for _, p := range preds {
		tx.Retract(p)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	want := sequentialRetractReference(stored, preds, set, first, exact)
	k.mu.Lock()
	got := slices.Clone(k.facts)
	gotIndex, gotSize := k.factIndex.buckets, k.factIndex.len()
	k.rebuildFactIndexLocked()
	wantIndex, wantSize := k.factIndex.buckets, k.factIndex.len()
	k.mu.Unlock()
	if len(got) == len(stored) || len(got) == 0 {
		t.Fatalf("fixture removed %d of %d facts; it must remove some and keep some", len(stored)-len(got), len(stored))
	}
	if len(got) != len(want) {
		t.Fatalf("compiled retraction kept %d facts, sequential passes %d", len(got), len(want))
	}
	for i := range got {
		if g, w := fmt.Sprintf("%s%#v", got[i].Predicate, got[i].Args), fmt.Sprintf("%s%#v", want[i].Predicate, want[i].Args); g != w {
			t.Fatalf("fact %d: compiled kept %v, sequential kept %v", i, got[i], want[i])
		}
	}
	held := func(args ...any) bool {
		for _, f := range got {
			if f.Predicate == "zoo2" && fmt.Sprintf("%#v", f.Args) == fmt.Sprintf("%#v", args) {
				return true
			}
		}
		return false
	}
	if !held(int32(1), 6) || held(math.Copysign(0, -1), 12) || held("a", 0) || held(namedText("a"), 21) {
		t.Fatal("fixture did not exercise the cross-type cases it names")
	}
	if gotSize != wantSize || !reflect.DeepEqual(keysOf(gotIndex), keysOf(wantIndex)) {
		t.Fatalf("dedupe index after commit has %d facts, a rebuild %d", gotSize, wantSize)
	}
}

func keysOf(buckets map[string]map[string]Fact) []string {
	var out []string
	for _, bucket := range buckets {
		for key := range bucket {
			out = append(out, key)
		}
	}
	sort.Strings(out)
	return out
}

// Through the production path: a transaction of retractions and asserts on
// the perf kernel leaves the same derived world as a fresh full evaluation.
func TestTransactionRetractionsKeepFixpointEquivalence(t *testing.T) {
	k := newPerfKernel(t, 64)
	tx := k.Transaction()
	for j := 0; j < 20; j++ {
		m := (j * 5) % 64
		tx.RetractExactFact(Fact{Predicate: "perf_edge", Args: []any{fmt.Sprintf("n%d", m), fmt.Sprintf("n%d", (m*7+1)%64)}})
	}
	tx.RetractFact(Fact{Predicate: "perf_label", Args: []any{"n8"}})
	tx.Retract("perf_bulk")
	tx.Assert(Fact{Predicate: "perf_probe", Args: []any{"n3"}})
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err := k.Query("perf_hit"); err != nil {
		t.Fatal(err)
	}
	if ev := k.LastEvaluation(); ev.Mode != "cone" {
		t.Fatalf("transaction evaluated as %q (fallback %q), want cone", ev.Mode, ev.ConeFallback)
	}
	requireSameStore(t, "after transaction", dumpStore(t, k), fullOracle(t, k))
	if got := k.FactCount(); got != 64-20+64-1+1 {
		t.Fatalf("EDB holds %d facts after the transaction, want %d", got, 64-20+64-1+1)
	}
}

// A clone's index and EDB are its own: removing on one side leaves the other.
func TestCloneSharesFactsButNotEDBStructure(t *testing.T) {
	parent := newPerfKernel(t, 8)
	c := parent.Clone()
	if err := c.Retract("perf_edge"); err != nil {
		t.Fatal(err)
	}
	if err := c.Assert(Fact{Predicate: "perf_bulk", Args: []any{"clone-only", int64(1)}}); err != nil {
		t.Fatal(err)
	}
	parent.mu.RLock()
	defer parent.mu.RUnlock()
	if len(parent.factIndex.bucket("perf_edge")) != 8 || parent.factIndex.has("perf_bulk", parent.canonFact(Fact{Predicate: "perf_bulk", Args: []any{"clone-only", int64(1)}})) {
		t.Fatal("clone writes reached the parent's EDB index")
	}
	if len(parent.facts) != 24 {
		t.Fatalf("parent EDB holds %d facts, want 24", len(parent.facts))
	}
	if _, ok := parent.store.(factstore.FactStoreWithRemove); !ok {
		t.Fatal("parent store lost removal support")
	}
}
