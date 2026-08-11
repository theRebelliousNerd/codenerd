package core

import (
	"testing"

	"codenerd/internal/types"
)

// TestCortexKernel_Query_FanOut_UnownedPredicate verifies the bug fix for
// CortexKernel.Query returning zero facts for unowned predicates.
//
// The CortexKernel federates multiple KernelShards. Before the fix, Query
// called routeToShard which on a miss fell back to the single cortex
// catch-all shard (empty), while facts for that predicate sat in the shard
// that derived them. QueryAll merges every shard and therefore saw the
// facts. After the fix, Query fans out to every shard when the predicate
// has no registered owner and concatenates the results, matching QueryAll.
//
// Setup (mirrors the production topology described in the bug):
//   - CortexKernel with cortexDomain = "shard_a" (so the miss path is
//     deterministic: it returns shard_a, not a random map iteration).
//   - shard_a owns "owned_pred" (to exercise the still-routed owned path).
//   - shard_b owns "other_pred" but also declares "unowned_pred" without
//     claiming ownership. A fact for "unowned_pred" is asserted directly
//     into shard_b's local store, bypassing cortex routing.
//
// Before the fix, cortex.Query("unowned_pred") hits shard_a -> 0 facts.
// After the fix it fans out to shard_a and shard_b -> 1 fact, and
// len(Query(p)) == len(QueryAll()[p]).
func TestCortexKernel_Query_FanOut_UnownedPredicate(t *testing.T) {
	cortex := NewCortexKernel("shard_a")

	// shard_a owns "owned_pred"
	shardA := setupTestShard(t, "shard_a", []string{"owned_pred"})
	// shard_b owns a different predicate; it will hold the unowned facts.
	shardB := setupTestShard(t, "shard_b", []string{"other_pred"})

	// Declare the unowned predicate in shard_b without claiming ownership,
	// so a query for it is an unowned query but shard_b's store can hold it.
	shardB.kernel.AppendPolicy("Decl unowned_pred(Value).")
	if err := shardB.Evaluate(); err != nil {
		t.Fatalf("shard_b Evaluate after Decl unowned_pred failed: %v", err)
	}

	if err := cortex.RegisterShard(shardA); err != nil {
		t.Fatalf("RegisterShard(shard_a) failed: %v", err)
	}
	if err := cortex.RegisterShard(shardB); err != nil {
		t.Fatalf("RegisterShard(shard_b) failed: %v", err)
	}

	// Load a fact for the unowned predicate directly into shard_b's local
	// store (bypassing cortex routing). With the router disabled (feature flag
	// off) this lands in shard_b unconditionally.
	unownedFact := types.Fact{Predicate: "unowned_pred", Args: []any{"hello"}}
	if err := shardB.Assert(unownedFact); err != nil {
		t.Fatalf("shard_b.Assert(unowned_pred) failed: %v", err)
	}

	// Sanity: shard_b's local query sees the fact.
	if got, err := shardB.Query("unowned_pred"); err != nil || len(got) != 1 {
		t.Fatalf("shard_b.Query(unowned_pred) = %d, err %v, want 1", len(got), err)
	}
	// Sanity: shard_a does not have it.
	if got, err := shardA.Query("unowned_pred"); err != nil || len(got) != 0 {
		t.Fatalf("shard_a.Query(unowned_pred) should be empty, got %d err %v", len(got), err)
	}

	// The bug: before the fix this returned 0 ("No facts found").
	results, err := cortex.Query("unowned_pred")
	if err != nil {
		t.Fatalf("cortex.Query(unowned_pred) failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("cortex.Query(unowned_pred) = %d facts, want 1 (fan-out to shard_b); bug: routed only to cortex/shard_a and missed shard_b", len(results))
	}
	if len(results) > 0 && results[0].Args[0] != "hello" {
		t.Fatalf("unowned fact arg = %v, want hello", results[0].Args[0])
	}

	// QueryAll merges every shard; for this predicate it should agree exactly
	// with the fan-out Query.
	all, err := cortex.QueryAll()
	if err != nil {
		t.Fatalf("cortex.QueryAll failed: %v", err)
	}
	if len(results) != len(all["unowned_pred"]) {
		t.Fatalf("fan-out mismatch: Query(unowned_pred)=%d vs QueryAll[unowned_pred]=%d", len(results), len(all["unowned_pred"]))
	}
}

// TestCortexKernel_Query_OwnedPredicateStillRoutes ensures the fix does not
// break the fast path: predicates that DO have a registered owner must still
// route to that single owner, not fan out.
func TestCortexKernel_Query_OwnedPredicateStillRoutes(t *testing.T) {
	cortex := NewCortexKernel("shard_a")

	shardA := setupTestShard(t, "shard_a", []string{"owned_pred"})
	shardB := setupTestShard(t, "shard_b", []string{"other_pred"})

	if err := cortex.RegisterShard(shardA); err != nil {
		t.Fatalf("RegisterShard(shard_a) failed: %v", err)
	}
	if err := cortex.RegisterShard(shardB); err != nil {
		t.Fatalf("RegisterShard(shard_b) failed: %v", err)
	}

	// Assert via the cortex — must route to shard_a because owned_pred is
	// owned by shard_a.
	fact := types.Fact{Predicate: "owned_pred", Args: []any{"owned_value"}}
	if err := cortex.Assert(fact); err != nil {
		t.Fatalf("cortex.Assert(owned_pred) failed: %v", err)
	}

	// cortex.Query must return the single routed fact.
	got, err := cortex.Query("owned_pred")
	if err != nil {
		t.Fatalf("cortex.Query(owned_pred) failed: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("cortex.Query(owned_pred) = %d, want 1 (routed to owner shard_a)", len(got))
	}

	// shard_a must have it; shard_b must not.
	if q, _ := shardA.Query("owned_pred"); len(q) != 1 {
		t.Fatalf("shard_a.Query(owned_pred) = %d, want 1", len(q))
	}
	if q, _ := shardB.Query("owned_pred"); len(q) != 0 {
		t.Fatalf("shard_b.Query(owned_pred) = %d, want 0 (must not be fanned out)", len(q))
	}

	// QueryAll should agree with Query for the owned predicate as well.
	all, err := cortex.QueryAll()
	if err != nil {
		t.Fatalf("QueryAll failed: %v", err)
	}
	if len(got) != len(all["owned_pred"]) {
		t.Fatalf("owned predicate mismatch: Query=%d vs QueryAll=%d", len(got), len(all["owned_pred"]))
	}
}

// TestCortexKernel_Query_FanOut_MultipleShards verifies that fan-out
// concatenates across more than one holder of the same unowned predicate,
// just as QueryAll does. Before the fix only the cortex shard was consulted.
func TestCortexKernel_Query_FanOut_MultipleShards(t *testing.T) {
	cortex := NewCortexKernel("shard_a")

	shardA := setupTestShard(t, "shard_a", []string{"owned_pred"})
	shardB := setupTestShard(t, "shard_b", []string{"other_pred"})

	// Both shards declare the same unowned predicate.
	for _, s := range []*KernelShard{shardA, shardB} {
		s.kernel.AppendPolicy("Decl shared_unowned(Value).")
		if err := s.Evaluate(); err != nil {
			t.Fatalf("Evaluate Decl shared_unowned on %s failed: %v", s.Domain(), err)
		}
	}

	if err := cortex.RegisterShard(shardA); err != nil {
		t.Fatalf("RegisterShard(shard_a) failed: %v", err)
	}
	if err := cortex.RegisterShard(shardB); err != nil {
		t.Fatalf("RegisterShard(shard_b) failed: %v", err)
	}

	// Put one distinct fact in each shard's local store.
	if err := shardA.Assert(types.Fact{Predicate: "shared_unowned", Args: []any{"from_a"}}); err != nil {
		t.Fatalf("shardA.Assert failed: %v", err)
	}
	if err := shardB.Assert(types.Fact{Predicate: "shared_unowned", Args: []any{"from_b"}}); err != nil {
		t.Fatalf("shardB.Assert failed: %v", err)
	}

	results, err := cortex.Query("shared_unowned")
	if err != nil {
		t.Fatalf("cortex.Query(shared_unowned) failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("cortex.Query(shared_unowned) = %d, want 2 (one from each shard) after fan-out", len(results))
	}

	all, err := cortex.QueryAll()
	if err != nil {
		t.Fatalf("QueryAll failed: %v", err)
	}
	if len(results) != len(all["shared_unowned"]) {
		t.Fatalf("fan-out mismatch: Query=%d vs QueryAll=%d", len(results), len(all["shared_unowned"]))
	}
}
