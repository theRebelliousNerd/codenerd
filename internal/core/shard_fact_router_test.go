package core

import (
	"fmt"
	"testing"

	"codenerd/internal/types"
)

// =============================================================================
// SHARD FACT ROUTER TESTS (Track D)
// =============================================================================
//
// These tests cover the per-shard fact-store partition coordinator. They do
// NOT depend on the per-shard-facts feature flag (which currently hard-codes
// to false). Instead they install the router by hand via SetFactRouter, the
// same hook that the cortex uses internally when the flag flips on.

// makeShardForRouter builds a KernelShard whose inner *RealKernel has been
// declared to own a small set of test predicates. We declare each predicate
// in the inner kernel so Mangle's analyzer accepts asserts and queries
// against them. We append (not replace) so the embedded constitution stays
// intact — the embedded policy references predicates like new_fact and a
// raw LoadSchemas would clobber those decls.
func makeShardForRouter(t *testing.T, domain string, owned []string) *KernelShard {
	t.Helper()

	shard, err := NewKernelShard(KernelShardConfig{
		Domain:          domain,
		OwnedPredicates: owned,
	})
	if err != nil {
		t.Fatalf("NewKernelShard(%s) failed: %v", domain, err)
	}

	for _, p := range owned {
		shard.kernel.AppendPolicy(fmt.Sprintf("Decl %s(Value).", p))
	}
	if err := shard.Evaluate(); err != nil {
		t.Fatalf("shard(%s) initial Evaluate failed: %v", domain, err)
	}

	return shard
}

// TestShardFactRouter_TwoShard_FanOut is the core acceptance test from the
// Track D plan: two shards A and B, A owns pred_a, B owns pred_b. Asserting
// pred_b on shard A MUST land in shard B's store (and vice versa) when the
// router is installed. Without the router, assert-on-wrong-shard would
// silently corrupt the partition.
func TestShardFactRouter_TwoShard_FanOut(t *testing.T) {
	shardA := makeShardForRouter(t, "shard_a", []string{"pred_a"})
	shardB := makeShardForRouter(t, "shard_b", []string{"pred_b"})

	router := NewShardFactRouter()
	if err := router.RegisterOwner(shardA, []string{"pred_a"}); err != nil {
		t.Fatalf("RegisterOwner(shardA) failed: %v", err)
	}
	if err := router.RegisterOwner(shardB, []string{"pred_b"}); err != nil {
		t.Fatalf("RegisterOwner(shardB) failed: %v", err)
	}

	shardA.SetRouter(router)
	shardB.SetRouter(router)

	// Assert pred_a on shardA — local path, no routing.
	if err := shardA.Assert(types.Fact{Predicate: "pred_a", Args: []any{"alpha"}}); err != nil {
		t.Fatalf("shardA.Assert(pred_a) failed: %v", err)
	}

	// Assert pred_b on shardA — MUST route to shardB.
	if err := shardA.Assert(types.Fact{Predicate: "pred_b", Args: []any{"beta"}}); err != nil {
		t.Fatalf("shardA.Assert(pred_b) routed to shardB failed: %v", err)
	}

	// Assert pred_a on shardB — MUST route to shardA.
	if err := shardB.Assert(types.Fact{Predicate: "pred_a", Args: []any{"gamma"}}); err != nil {
		t.Fatalf("shardB.Assert(pred_a) routed to shardA failed: %v", err)
	}

	// Validate: shardA's store should contain BOTH pred_a facts; shardB
	// should contain the single pred_b fact.
	gotA, err := shardA.Query("pred_a")
	if err != nil {
		t.Fatalf("shardA.Query(pred_a) failed: %v", err)
	}
	if len(gotA) != 2 {
		t.Errorf("shardA pred_a count = %d, want 2 (alpha + gamma)", len(gotA))
	}

	gotB, err := shardB.Query("pred_b")
	if err != nil {
		t.Fatalf("shardB.Query(pred_b) failed: %v", err)
	}
	if len(gotB) != 1 {
		t.Errorf("shardB pred_b count = %d, want 1 (beta)", len(gotB))
	}

	// Cross-shard query: shardA.Query(pred_b) should route to shardB and
	// return its single fact, NOT an empty result from shardA's store.
	routed, err := shardA.Query("pred_b")
	if err != nil {
		t.Fatalf("shardA.Query(pred_b) routed failed: %v", err)
	}
	if len(routed) != 1 {
		t.Errorf("shardA.Query(pred_b) routed = %d facts, want 1", len(routed))
	}

	// Sanity: the router should have recorded at least 3 routed hits
	// (assert pred_b A->B, assert pred_a B->A, query pred_b A->B).
	m := router.Metrics()
	if m.HitCount < 3 {
		t.Errorf("router HitCount = %d, want >= 3", m.HitCount)
	}
}

// TestShardFactRouter_OffPath_NilRouter verifies that a KernelShard without
// a router still behaves exactly like the legacy single-store kernel: it
// happily asserts whatever predicate it's given into its inner kernel, no
// dispatch. This is the byte-identical "flag off" guarantee.
func TestShardFactRouter_OffPath_NilRouter(t *testing.T) {
	shard := makeShardForRouter(t, "solo", []string{"pred_x", "pred_y"})

	// No SetRouter call — router stays nil.
	if got := shard.getRouter(); got != nil {
		t.Fatalf("expected nil router on unconfigured shard, got %v", got)
	}

	// Asserting an UN-owned predicate on a router-less shard must NOT error
	// (legacy behavior is "delegate blindly to the inner kernel"). We can't
	// test that pred_z actually lands without declaring it; instead we
	// confirm an owned predicate lands as before — that's the path the off
	// path is meant to preserve.
	if err := shard.Assert(types.Fact{Predicate: "pred_x", Args: []any{"v1"}}); err != nil {
		t.Fatalf("Assert on owned predicate without router failed: %v", err)
	}
	got, err := shard.Query("pred_x")
	if err != nil {
		t.Fatalf("Query(pred_x) failed: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("Query(pred_x) = %d facts, want 1", len(got))
	}
}

// TestShardFactRouter_UnknownPredicate_FallsBackLocal documents the chosen
// fallback semantics: when the router has no registered owner for a
// predicate, the call lands in the caller's local kernel. This preserves
// availability — facts are never silently dropped because the manifest is
// incomplete.
//
// Setup: we declare TWO predicates on the shard but only mark ONE as owned.
// The non-owned-but-declared predicate is what triggers the router; since
// no owner is registered with the router, the assert falls back to local.
func TestShardFactRouter_UnknownPredicate_FallsBackLocal(t *testing.T) {
	shard := makeShardForRouter(t, "lonely", []string{"owned_pred"})
	// Declare a second predicate that the shard does NOT claim ownership of.
	shard.kernel.AppendPolicy("Decl unowned_pred(Value).")
	if err := shard.Evaluate(); err != nil {
		t.Fatalf("evaluate after extra decl failed: %v", err)
	}

	router := NewShardFactRouter() // empty router — no owners registered
	shard.SetRouter(router)

	// Asserting the unowned-but-declared predicate forces the router path.
	// Since no owner is registered, MissCount must increment and the fact
	// must land in this shard's local store as a fallback.
	if err := shard.Assert(types.Fact{Predicate: "unowned_pred", Args: []any{"v"}}); err != nil {
		t.Fatalf("Assert(unowned_pred) failed: %v", err)
	}
	if m := router.Metrics(); m.MissCount == 0 {
		t.Errorf("router MissCount = 0, want >= 1")
	}
	got, err := shard.Query("unowned_pred")
	if err != nil {
		t.Fatalf("Query(unowned_pred) failed: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("Query(unowned_pred) = %d, want 1 (local fallback)", len(got))
	}
}

// TestShardFactRouter_BatchPartitions verifies that AssertBatch with mixed
// predicates is split per owner. We assert two facts (one for shardA, one
// for shardB) in a single AssertBatch on shardA; both must land in their
// correct stores.
func TestShardFactRouter_BatchPartitions(t *testing.T) {
	shardA := makeShardForRouter(t, "batch_a", []string{"ba_pred"})
	shardB := makeShardForRouter(t, "batch_b", []string{"bb_pred"})

	router := NewShardFactRouter()
	_ = router.RegisterOwner(shardA, []string{"ba_pred"})
	_ = router.RegisterOwner(shardB, []string{"bb_pred"})
	shardA.SetRouter(router)
	shardB.SetRouter(router)

	batch := []types.Fact{
		{Predicate: "ba_pred", Args: []any{"local"}},
		{Predicate: "bb_pred", Args: []any{"remote"}},
	}
	if err := shardA.AssertBatch(batch); err != nil {
		t.Fatalf("shardA.AssertBatch failed: %v", err)
	}

	gotA, err := shardA.Query("ba_pred")
	if err != nil {
		t.Fatalf("shardA.Query(ba_pred) failed: %v", err)
	}
	if len(gotA) != 1 {
		t.Errorf("ba_pred count in shardA = %d, want 1", len(gotA))
	}

	gotB, err := shardB.Query("bb_pred")
	if err != nil {
		t.Fatalf("shardB.Query(bb_pred) failed: %v", err)
	}
	if len(gotB) != 1 {
		t.Errorf("bb_pred count in shardB = %d, want 1 (routed via batch)", len(gotB))
	}
}

// TestShardFactRouter_CortexAutoWiring confirms that SetFactRouter on the
// cortex propagates to already-registered shards and re-registers owners.
// This is the path the cortex itself would use if the feature flag flipped
// at runtime, and it's what the production wiring will rely on.
func TestShardFactRouter_CortexAutoWiring(t *testing.T) {
	cortex := NewCortexKernel("cortex")

	shardA := makeShardForRouter(t, "ca_a", []string{"ca_pred_a"})
	shardB := makeShardForRouter(t, "ca_b", []string{"ca_pred_b"})

	if err := cortex.RegisterShard(shardA); err != nil {
		t.Fatalf("RegisterShard(A) failed: %v", err)
	}
	if err := cortex.RegisterShard(shardB); err != nil {
		t.Fatalf("RegisterShard(B) failed: %v", err)
	}

	// Flag is hard-coded false in features.IsPerShardFactsEnabled(), so the
	// cortex created its router as nil. Install one by hand.
	if got := cortex.FactRouter(); got != nil {
		t.Fatalf("unexpected router on flag-off cortex: %v", got)
	}
	cortex.SetFactRouter(NewShardFactRouter())
	if cortex.FactRouter() == nil {
		t.Fatal("FactRouter still nil after SetFactRouter")
	}

	// Now asserting ca_pred_b on shardA must route to shardB.
	if err := shardA.Assert(types.Fact{Predicate: "ca_pred_b", Args: []any{"cross"}}); err != nil {
		t.Fatalf("shardA.Assert(ca_pred_b) routed failed: %v", err)
	}
	gotB, err := shardB.Query("ca_pred_b")
	if err != nil {
		t.Fatalf("shardB.Query failed: %v", err)
	}
	if len(gotB) != 1 {
		t.Errorf("shardB ca_pred_b count = %d, want 1 (auto-wired route)", len(gotB))
	}
}
