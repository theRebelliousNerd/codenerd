package core

import (
	"testing"

	"codenerd/internal/types"
)

// W6 arms the witness verdict (policy/witness.mg + the turn_verified arm in
// coder_safety.mg): an owed element is met only when the turn's own coverage
// run executed it and the turn's /test gate passed. These tests prove the
// derivations on the real corpus, from facts spelled exactly as their
// producers emit them (internal/mangle/agents.md: a Decl is a contract).

// witnessStep6Seed is two writing turns over trivial packages. T1 changed
// Target and Helper and its passing run executed Target only; T2 changed
// Solo and its passing run executed it. No turn_verb: the pinning gate is a
// production-shaping concern the session tests cover, not this join.
func witnessStep6Seed() []types.Fact {
	t1 := types.MangleAtom("/turn_w6_gap")
	t2 := types.MangleAtom("/turn_w6_met")
	evidence := func(turn types.MangleAtom) types.Fact {
		return types.Fact{Predicate: "turn_evidence", Args: []any{
			turn, types.MangleAtom("/fix"), 2, 1, 1, types.MangleAtom("/false"), types.MangleAtom("/false"),
		}}
	}
	written := func(turn types.MangleAtom) types.Fact {
		return types.Fact{Predicate: "turn_written", Args: []any{turn, types.MangleString("p/app.go"), types.MangleString(".go")}}
	}
	gate := func(turn, name, verdict types.MangleAtom) types.Fact {
		return types.Fact{Predicate: "turn_gate", Args: []any{turn, name, verdict}}
	}
	changed := func(turn types.MangleAtom, ref string) types.Fact {
		return types.Fact{Predicate: "turn_changed_element", Args: []any{turn, types.MangleString(ref)}}
	}
	measured := func(turn types.MangleAtom, ref string) types.Fact {
		return types.Fact{Predicate: "turn_element_measured", Args: []any{turn, types.MangleString(ref)}}
	}
	return []types.Fact{
		evidence(t1), evidence(t2),
		written(t1), written(t2),
		gate(t1, "/build", "/passing"), gate(t1, "/test", "/passing"),
		gate(t2, "/build", "/passing"), gate(t2, "/test", "/passing"),
		changed(t1, "fn:p.Target"), changed(t1, "fn:p.Helper"), changed(t2, "fn:p.Solo"),
		measured(t1, "fn:p.Target"), measured(t1, "fn:p.Helper"), measured(t2, "fn:p.Solo"),
		{Predicate: "turn_element_uncovered", Args: []any{t1, types.MangleString("fn:p.Helper")}},
	}
}

// newWitnessStep6Cortex builds a sharded kernel shaped like production for
// the witness verdict: the world shard owns every predicate the fixture
// asserts, because production's world entry owns the whole turn family
// together (internal/shards/registration.go, world OwnedPredicates). The
// verdict join needs no shared predicates: every input lives on one shard.
func newWitnessStep6Cortex(t *testing.T) *CortexKernel {
	t.Helper()
	cortex := NewCortexKernel("cortex")
	if err := cortex.SetSharedPredicates(nil); err != nil {
		t.Fatalf("SetSharedPredicates: %v", err)
	}
	world, err := NewKernelShard(KernelShardConfig{
		Domain: "world",
		OwnedPredicates: []string{
			"turn_evidence", "turn_written", "turn_gate",
			"turn_changed_element", "turn_element_measured", "turn_element_uncovered",
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

// TestWitnessStep6_ShouldDeriveMetUnwitnessedAndVerdict proves the W6 chain:
// Target and Solo are met, Helper is owed but not met, T1 is unverified
// with /change_unwitnessed as its only missing evidence, and T2 verifies.
func TestWitnessStep6_ShouldDeriveMetUnwitnessedAndVerdict(t *testing.T) {
	single, err := NewRealKernel()
	if err != nil {
		t.Fatalf("the shipped corpus must load: %v", err)
	}
	for _, f := range witnessStep6Seed() {
		if err := single.Assert(f); err != nil {
			t.Fatalf("assert %s: %v", f.Predicate, err)
		}
	}
	query := func(pred string) []types.Fact {
		rows, err := single.Query(pred)
		if err != nil {
			t.Fatalf("query %s: %v", pred, err)
		}
		return rows
	}
	assertWitnessRows(t, "witness_executed", query("witness_executed"),
		witnessRow("witness_executed", "/turn_w6_gap", "fn:p.Target"),
		witnessRow("witness_executed", "/turn_w6_met", "fn:p.Solo"),
	)
	assertWitnessRows(t, "witness_met", query("witness_met"),
		witnessRow("witness_met", "/turn_w6_gap", "fn:p.Target"),
		witnessRow("witness_met", "/turn_w6_met", "fn:p.Solo"),
	)
	assertWitnessRows(t, "turn_unwitnessed", query("turn_unwitnessed"),
		witnessRow("turn_unwitnessed", "/turn_w6_gap", "fn:p.Helper"),
	)
	assertWitnessRows(t, "turn_has_unwitnessed", query("turn_has_unwitnessed"),
		witnessRow("turn_has_unwitnessed", "/turn_w6_gap"),
	)
	assertWitnessRows(t, "turn_verified", query("turn_verified"),
		witnessRow("turn_verified", "/turn_w6_met"),
	)
	assertWitnessRows(t, "turn_unverified", query("turn_unverified"),
		witnessRow("turn_unverified", "/turn_w6_gap"),
	)
	assertWitnessRows(t, "turn_missing_evidence", query("turn_missing_evidence"),
		witnessRow("turn_missing_evidence", "/turn_w6_gap", "/change_unwitnessed"),
	)
}

// TestWitnessStep6_WhenSharded_ShouldMatchSingleKernel is the split-join
// guard for the verdict join: every predicate its rules read is owned by
// the world shard in production, so the sharded kernel must derive exactly
// what the single-store kernel derives.
func TestWitnessStep6_WhenSharded_ShouldMatchSingleKernel(t *testing.T) {
	seed := witnessStep6Seed()
	single, err := NewRealKernel()
	if err != nil {
		t.Fatalf("the shipped corpus must load: %v", err)
	}
	for _, f := range seed {
		if err := single.Assert(f); err != nil {
			t.Fatalf("assert %s: %v", f.Predicate, err)
		}
	}
	sharded := newWitnessStep6Cortex(t)
	for _, f := range seed {
		if err := sharded.Assert(f); err != nil {
			t.Fatalf("assert %s: %v", f.Predicate, err)
		}
	}
	for _, pred := range []string{
		"witness_owed", "witness_executed", "witness_met",
		"turn_unwitnessed", "turn_has_unwitnessed",
		"turn_verified", "turn_unverified", "turn_missing_evidence",
	} {
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
	if rows, _ := single.Query("turn_unwitnessed"); len(rows) == 0 {
		t.Fatal("single kernel derived no turn_unwitnessed rows; the parity check is vacuous")
	}
	if rows, _ := single.Query("turn_verified"); len(rows) == 0 {
		t.Fatal("single kernel derived no turn_verified rows; the parity check is vacuous")
	}
}
