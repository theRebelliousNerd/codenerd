package core

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"codenerd/internal/types"
)

// The witness chain (policy/witness.mg) derives which tests execute which
// code elements (test_executes, from the test_impact.mg chain) and which
// changed elements of a writing turn are owed a witness (witness_owed, with
// witness_candidate naming the tests that could serve). Nothing reads these
// conclusions yet -- results, witness_met and the verdict arm land in a later
// lane -- so these tests prove the derivations on the real corpus, from facts
// spelled exactly as their producers emit them, not hand spellings that are
// self-consistent and wrong (internal/mangle/agents.md: a Decl is a contract).

// witnessSeedFacts is one writing turn over a two-function package: TestTarget
// calls Target, Helper is called by nothing, and the turn changed both. Each
// fact mirrors its producer's arg types: code_element/is_test_function/code_calls
// as world emits them (code_elements.go ToFacts: plain strings, int64 lines;
// cartographer.go: plain strings, bare plus dual fn:-prefixed rows),
// turn_evidence as the executor asserts it (ints for the /number slots, as in
// turn_write_class_test.go), turn_changed_element as session/assertTurnElements
// does (MangleAtom turn, MangleString ref).
func witnessSeedFacts(turn types.MangleAtom) []types.Fact {
	return []types.Fact{
		{Predicate: "code_element", Args: []any{"fn:p.TestTarget", "/function", "p/app_test.go", int64(1), int64(9)}},
		{Predicate: "code_element", Args: []any{"fn:p.Target", "/function", "p/app.go", int64(1), int64(5)}},
		{Predicate: "code_element", Args: []any{"fn:p.Helper", "/function", "p/app.go", int64(7), int64(9)}},
		{Predicate: "is_test_function", Args: []any{"fn:p.TestTarget"}},
		{Predicate: "code_calls", Args: []any{"p.TestTarget", "p.Target"}},
		{Predicate: "code_calls", Args: []any{"fn:p.TestTarget", "fn:p.Target"}},
		{Predicate: "turn_evidence", Args: []any{
			turn, types.MangleAtom("/fix"), 2, 1, 0, types.MangleAtom("/false"), types.MangleAtom("/false"),
		}},
		{Predicate: "turn_changed_element", Args: []any{turn, types.MangleString("fn:p.Target")}},
		{Predicate: "turn_changed_element", Args: []any{turn, types.MangleString("fn:p.Helper")}},
	}
}

// newWitnessCortex builds a sharded kernel shaped like production for the
// witness join: the world shard owns every predicate the fixture asserts,
// because production's world entry owns the whole family together -- the
// world-model facts and the turn-verdict inputs beside them
// (internal/shards/registration.go, world OwnedPredicates). The owned list
// mirrors those entries narrowed to what the fixture asserts, the same way
// productionShapedCortex mirrors the entries it needs (core cannot import
// shards: shards imports core). The witness join needs no shared predicates:
// every input lives on the world shard.
func newWitnessCortex(t *testing.T) *CortexKernel {
	t.Helper()
	cortex := NewCortexKernel("cortex")
	if err := cortex.SetSharedPredicates(nil); err != nil {
		t.Fatalf("SetSharedPredicates: %v", err)
	}
	world, err := NewKernelShard(KernelShardConfig{
		Domain: "world",
		OwnedPredicates: []string{
			"code_element", "code_calls", "is_test_function",
			"turn_evidence", "turn_changed_element",
		},
	})
	if err != nil {
		t.Fatalf("world shard: %v", err)
	}
	catchAll, err := NewKernelShard(KernelShardConfig{Domain: "cortex"})
	if err != nil {
		t.Fatalf("cortex shard: %v", err)
	}
	for _, s := range []*KernelShard{world, catchAll} {
		if err := cortex.RegisterShard(s); err != nil {
			t.Fatalf("register %s: %v", s.Domain(), err)
		}
	}
	return cortex
}

// witnessRow encodes one expected derived row for set comparison. fmt.Sprint
// renders a queried argument however the kernel typed it (string or atom),
// so the expectation is about the value, not the Go type.
func witnessRow(pred string, args ...string) string {
	return pred + "\x00" + strings.Join(args, "\x00")
}

func witnessRowSet(facts []types.Fact) map[string]struct{} {
	out := make(map[string]struct{}, len(facts))
	for _, f := range facts {
		parts := make([]string, 0, len(f.Args)+1)
		parts = append(parts, f.Predicate)
		for _, a := range f.Args {
			parts = append(parts, fmt.Sprint(a))
		}
		out[strings.Join(parts, "\x00")] = struct{}{}
	}
	return out
}

func witnessSortedKeys(set map[string]struct{}) []string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// assertWitnessRows compares a derived relation against its exact expected
// rows. Exact, not subset: a chain that derives extra rows (a junk callee
// joining something it should not) is as broken as one that derives too few.
func assertWitnessRows(t *testing.T, what string, got []types.Fact, want ...string) {
	t.Helper()
	set := witnessRowSet(got)
	if len(set) != len(want) {
		t.Fatalf("%s: got %d rows %q, want %d rows %q", what, len(set), witnessSortedKeys(set), len(want), want)
	}
	for _, w := range want {
		if _, ok := set[w]; !ok {
			t.Errorf("%s: missing row %q (got %q)", what, w, witnessSortedKeys(set))
		}
	}
}

// TestWitness_WhenTurnChangedElements_ShouldDeriveOwedAndCandidates proves the
// K1 chain on the production-shaped sharded kernel: the turn wrote, the impact
// edge derives through the test_impact chain, test_executes reuses it, both
// changed elements are owed a witness, and only Target -- the one a test
// executes -- gains a candidate. Helper is the gap case: owed, with no
// candidate, which is the silence the later verdict lane reads.
func TestWitness_WhenTurnChangedElements_ShouldDeriveOwedAndCandidates(t *testing.T) {
	turn := types.MangleAtom("/turn_witness")
	cortex := newWitnessCortex(t)
	for _, f := range witnessSeedFacts(turn) {
		if err := cortex.Assert(f); err != nil {
			t.Fatalf("assert %s: %v", f.Predicate, err)
		}
	}

	assertWitnessRows(t, "turn_wrote", cortexQuery(t, cortex, "turn_wrote"),
		witnessRow("turn_wrote", "/turn_witness"),
	)
	assertWitnessRows(t, "test_depends_on_transitive", cortexQuery(t, cortex, "test_depends_on_transitive"),
		witnessRow("test_depends_on_transitive", "fn:p.TestTarget", "fn:p.Target"),
	)
	assertWitnessRows(t, "test_executes", cortexQuery(t, cortex, "test_executes"),
		witnessRow("test_executes", "fn:p.TestTarget", "fn:p.Target"),
	)
	assertWitnessRows(t, "witness_owed", cortexQuery(t, cortex, "witness_owed"),
		witnessRow("witness_owed", "/turn_witness", "fn:p.Target"),
		witnessRow("witness_owed", "/turn_witness", "fn:p.Helper"),
	)
	assertWitnessRows(t, "witness_candidate", cortexQuery(t, cortex, "witness_candidate"),
		witnessRow("witness_candidate", "/turn_witness", "fn:p.Target", "fn:p.TestTarget"),
	)
}

// TestWitness_WhenTurnDidNotWrite_ShouldOweNothing pins the turn_wrote
// conjunct in witness_owed: a changed element with no writing turn behind it
// owes no witness.
func TestWitness_WhenTurnDidNotWrite_ShouldOweNothing(t *testing.T) {
	cortex := newWitnessCortex(t)
	cortexAssert(t, cortex, "code_element", "fn:p.Target", "/function", "p/app.go", int64(1), int64(5))
	cortexAssert(t, cortex, "turn_changed_element", types.MangleAtom("/turn_nowrite"), types.MangleString("fn:p.Target"))
	if rows := cortexQuery(t, cortex, "witness_owed"); len(rows) != 0 {
		t.Fatalf("witness_owed derived %v without turn_wrote", rows)
	}
}

// TestWitness_WhenSharded_ShouldMatchSingleKernel is the split-join guard for
// the witness chain: every predicate its rules join is owned by the world
// shard in production, so the sharded kernel must derive exactly what the
// single-store kernel derives. If a future manifest move strands any of them
// on another shard, the sharded side goes quiet here while the single side
// keeps deriving.
func TestWitness_WhenSharded_ShouldMatchSingleKernel(t *testing.T) {
	turn := types.MangleAtom("/turn_witness")
	seed := witnessSeedFacts(turn)
	single, err := NewRealKernel()
	if err != nil {
		t.Fatalf("the shipped corpus must load: %v", err)
	}
	for _, f := range seed {
		if err := single.Assert(f); err != nil {
			t.Fatalf("assert %s: %v", f.Predicate, err)
		}
	}
	sharded := newWitnessCortex(t)
	for _, f := range seed {
		if err := sharded.Assert(f); err != nil {
			t.Fatalf("assert %s: %v", f.Predicate, err)
		}
	}
	for _, pred := range []string{"test_executes", "witness_owed", "witness_candidate"} {
		want, err := single.Query(pred)
		if err != nil {
			t.Fatalf("single query %s: %v", pred, err)
		}
		got := cortexQuery(t, sharded, pred)
		wantSet, gotSet := witnessRowSet(want), witnessRowSet(got)
		if len(wantSet) != len(gotSet) {
			t.Errorf("%s: single derived %d rows %q, sharded derived %d rows %q",
				pred, len(wantSet), witnessSortedKeys(wantSet), len(gotSet), witnessSortedKeys(gotSet))
			continue
		}
		for k := range wantSet {
			if _, ok := gotSet[k]; !ok {
				t.Errorf("%s: sharded kernel missing row %q", pred, k)
			}
		}
	}
	// The parity above is only meaningful if the chain derived something.
	if rows, _ := single.Query("witness_candidate"); len(rows) == 0 {
		t.Fatal("single kernel derived no witness_candidate rows; the parity check is vacuous")
	}
}
