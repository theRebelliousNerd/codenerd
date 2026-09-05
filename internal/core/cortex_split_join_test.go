package core

import (
	"testing"

	"codenerd/internal/types"
)

// productionShapedCortex builds a CortexKernel the way the factory does: the
// routing shard owns routing_result, the catch-all "cortex" shard receives
// every unowned runtime fact, and the per-turn facts are shared. Each shard
// evaluates the full policy corpus over its own facts only
// (cortex_kernel.go EvaluateAll), so without sharing, a rule joining
// user_intent with a catch-all fact has no shard in which both exist.
//
// The manifests live in internal/shards (which imports core), so this test
// mirrors the two entries it needs rather than importing them; the
// internal/system boot tests cover the real table.
func productionShapedCortex(t *testing.T, shared []string) *CortexKernel {
	t.Helper()
	cortex := NewCortexKernel("cortex")
	if err := cortex.SetSharedPredicates(shared); err != nil {
		t.Fatalf("SetSharedPredicates: %v", err)
	}
	routing, err := NewKernelShard(KernelShardConfig{
		Domain:          "routing",
		OwnedPredicates: []string{"routing_result", "derived_mode"},
	})
	if err != nil {
		t.Fatalf("routing shard: %v", err)
	}
	catchAll, err := NewKernelShard(KernelShardConfig{Domain: "cortex"})
	if err != nil {
		t.Fatalf("cortex shard: %v", err)
	}
	for _, s := range []*KernelShard{routing, catchAll} {
		if err := cortex.RegisterShard(s); err != nil {
			t.Fatalf("register %s: %v", s.Domain(), err)
		}
	}
	return cortex
}

func cortexAssert(t *testing.T, k *CortexKernel, pred string, args ...any) {
	t.Helper()
	if err := k.Assert(types.Fact{Predicate: pred, Args: args}); err != nil {
		t.Fatalf("assert %s%v: %v", pred, args, err)
	}
}

func cortexQuery(t *testing.T, k *CortexKernel, query string) []types.Fact {
	t.Helper()
	if err := k.Evaluate(); err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	facts, err := k.Query(query)
	if err != nil {
		t.Fatalf("query %s: %v", query, err)
	}
	return facts
}

var turnShared = []string{"user_intent", "executive_processed_intent", "active_shard", "current_time"}

// TestCortexSplitJoin_SharedIntentReachesEveryShard: with user_intent
// unshared and owned by routing (the pre-item-55 shape), stage guidance never
// reaches injectable_context because active_shard lives in the catch-all.
// Shared, the rule fires in the catch-all and the guidance is admitted once.
func TestCortexSplitJoin_SharedIntentReachesEveryShard(t *testing.T) {
	single := setupMockKernel(t)
	mustAssert(t, single, "active_shard", "shard-1", types.MangleAtom("/coder"))
	assertIntent(t, single, "/mutation", "/fix", "auth middleware")
	if stageGuidanceCount(mustQuery(t, single, "injectable_context")) == 0 {
		t.Fatal("precondition: single-store kernel admits stage guidance into injectable_context")
	}

	// Pre-fix shape: user_intent owned by routing, nothing shared.
	legacy := NewCortexKernel("cortex")
	routing, _ := NewKernelShard(KernelShardConfig{Domain: "routing", OwnedPredicates: []string{"user_intent"}})
	catchAll, _ := NewKernelShard(KernelShardConfig{Domain: "cortex"})
	for _, s := range []*KernelShard{routing, catchAll} {
		if err := legacy.RegisterShard(s); err != nil {
			t.Fatal(err)
		}
	}
	cortexAssert(t, legacy, "active_shard", "shard-1", types.MangleAtom("/coder"))
	cortexAssert(t, legacy, "user_intent", "/current_intent", types.MangleAtom("/mutation"), types.MangleAtom("/fix"), "auth middleware", "none")
	if n := stageGuidanceCount(cortexQuery(t, legacy, "injectable_context")); n != 0 {
		t.Fatalf("legacy shape must split the join (documents the defect), got %d guidance facts", n)
	}

	// Production shape: user_intent shared.
	cortex := productionShapedCortex(t, turnShared)
	cortexAssert(t, cortex, "active_shard", "shard-1", types.MangleAtom("/coder"))
	cortexAssert(t, cortex, "user_intent", "/current_intent", types.MangleAtom("/mutation"), types.MangleAtom("/fix"), "auth middleware", "none")
	if n := stageGuidanceCount(cortexQuery(t, cortex, "injectable_context")); n != 1 {
		t.Fatalf("shared intent must admit exactly one stage guidance fact (deduped across shards), got %d", n)
	}
	// task_stage derives in both shards now; the fan-out query dedupes it.
	if n := len(cortexQuery(t, cortex, "task_stage")); n != 1 {
		t.Fatalf("task_stage must be reported once, got %d", n)
	}
	// A shared predicate reads once, not once per replica.
	if n := len(cortexQuery(t, cortex, "user_intent")); n != 1 {
		t.Fatalf("shared user_intent must be reported once, got %d", n)
	}
}

// TestCortexSplitJoin_SharedNegationIsNotBlind: executive_processed_intent is
// asserted to stop next_action from re-deriving. Owned nowhere, it used to
// land only in the catch-all while the next_action rules evaluated in the
// routing shard, so the negation always held. Shared, it reaches the rule.
func TestCortexSplitJoin_SharedNegationIsNotBlind(t *testing.T) {
	cortex := productionShapedCortex(t, turnShared)
	cortexAssert(t, cortex, "user_intent", "/current_intent", types.MangleAtom("/mutation"), types.MangleAtom("/fix"), "auth middleware", "none")
	before := len(cortexQuery(t, cortex, "next_action"))
	if before == 0 {
		t.Fatal("precondition: a /fix intent derives at least one next_action")
	}
	cortexAssert(t, cortex, "executive_processed_intent", types.MangleAtom("/current_intent"))
	if after := len(cortexQuery(t, cortex, "next_action")); after >= before {
		t.Fatalf("executive_processed_intent must retire next_action: %d before, %d after", before, after)
	}
}

// TestCortexSplitJoin_SharedRetractReachesEveryReplica: retracting a shared
// predicate must clear every replica, or a stale intent lingers in the
// shards the retract never visited.
func TestCortexSplitJoin_SharedRetractReachesEveryReplica(t *testing.T) {
	cortex := productionShapedCortex(t, turnShared)
	cortexAssert(t, cortex, "user_intent", "/current_intent", types.MangleAtom("/mutation"), types.MangleAtom("/fix"), "x", "none")
	if err := cortex.Retract("user_intent"); err != nil {
		t.Fatal(err)
	}
	for _, domain := range cortex.ShardDomains() {
		shard, _ := cortex.GetShard(domain)
		facts, err := shard.queryLocal("user_intent")
		if err != nil {
			t.Fatal(err)
		}
		if len(facts) != 0 {
			t.Fatalf("shard %s still holds %d user_intent facts after Retract", domain, len(facts))
		}
	}

	// Transaction path: retract + assert in one commit reaches every replica.
	tx := cortex.Transaction()
	tx.Assert(types.Fact{Predicate: "user_intent", Args: []any{"/current_intent", types.MangleAtom("/query"), types.MangleAtom("/explain"), "y", "none"}})
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	for _, domain := range cortex.ShardDomains() {
		shard, _ := cortex.GetShard(domain)
		facts, _ := shard.queryLocal("user_intent")
		if len(facts) != 1 {
			t.Fatalf("shard %s holds %d user_intent facts after transaction, want 1", domain, len(facts))
		}
	}
}

func TestCortexSplitJoin_SharedCannotBeOwned(t *testing.T) {
	cortex := NewCortexKernel("cortex")
	if err := cortex.SetSharedPredicates([]string{"user_intent"}); err != nil {
		t.Fatal(err)
	}
	owner, _ := NewKernelShard(KernelShardConfig{Domain: "routing", OwnedPredicates: []string{"user_intent"}})
	if err := cortex.RegisterShard(owner); err == nil {
		t.Fatal("registering a shard that owns a shared predicate must fail")
	}
	fresh := NewCortexKernel("cortex")
	if err := fresh.RegisterShard(owner); err != nil {
		t.Fatal(err)
	}
	if err := fresh.SetSharedPredicates([]string{"user_intent"}); err == nil {
		t.Fatal("sharing an owned predicate must fail")
	}
}

func stageGuidanceCount(facts []types.Fact) int {
	n := 0
	for _, f := range facts {
		if len(f.Args) == 2 {
			if s, ok := f.Args[1].(string); ok && len(s) > 7 && s[:7] == "stage /" {
				n++
			}
		}
	}
	return n
}

func mustQuery(t *testing.T, k *RealKernel, pred string) []types.Fact {
	t.Helper()
	facts, err := k.Query(pred)
	if err != nil {
		t.Fatalf("query %s: %v", pred, err)
	}
	return facts
}
