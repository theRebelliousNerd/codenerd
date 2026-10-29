package core

import (
	"testing"

	"codenerd/internal/features"
	"codenerd/internal/types"
)

const consumerRouteTestPolicy = `
Decl consumer_route_source(Value) bound [/string].
Decl consumer_route_aux(Value) bound [/string].
Decl consumer_route_policy(Value) bound [/string].
Decl consumer_route_gate(Value) bound [/string].
consumer_route_gate(Value) :-
    consumer_route_source(Value),
    consumer_route_aux(Value),
    consumer_route_policy(Value).
`

func newConsumerRouteTestShard(t *testing.T, domain string, owned []string) *KernelShard {
	t.Helper()
	shard, err := NewKernelShard(KernelShardConfig{Domain: domain, OwnedPredicates: owned})
	if err != nil {
		t.Fatalf("NewKernelShard(%s): %v", domain, err)
	}
	shard.AppendPolicy(consumerRouteTestPolicy)
	if err := shard.Evaluate(); err != nil {
		t.Fatalf("Evaluate(%s): %v", domain, err)
	}
	return shard
}

func newConsumerRouteTestCortex(t *testing.T) (*CortexKernel, map[string]*KernelShard, []PredicateConsumerRoute) {
	t.Helper()
	world := newConsumerRouteTestShard(t, "world", []string{"consumer_route_source"})
	policy := newConsumerRouteTestShard(t, "policy", []string{"consumer_route_policy"})
	cortexShard := newConsumerRouteTestShard(t, "cortex", nil)
	kernel := NewCortexKernel("cortex")
	for _, shard := range []*KernelShard{world, policy, cortexShard} {
		if err := kernel.RegisterShard(shard); err != nil {
			t.Fatalf("RegisterShard(%s): %v", shard.Domain(), err)
		}
	}
	routes := []PredicateConsumerRoute{
		{Predicate: "consumer_route_source", Consumer: "policy"},
		{Predicate: "consumer_route_aux", Consumer: "policy"},
	}
	if err := kernel.SetPredicateConsumerRoutes(routes); err != nil {
		t.Fatalf("SetPredicateConsumerRoutes: %v", err)
	}
	owners := map[string]string{"consumer_route_source": "world", "consumer_route_policy": "policy"}
	dm, err := BuildDerivationMapWithConsumerRoutes(consumerRouteTestPolicy, nil, owners, nil, routes, "cortex")
	if err != nil {
		t.Fatalf("BuildDerivationMapWithConsumerRoutes: %v", err)
	}
	kernel.SetDerivationMap(dm)
	return kernel, map[string]*KernelShard{"world": world, "policy": policy, "cortex": cortexShard}, routes
}

func assertConsumerRouteFact(t *testing.T, kernel *CortexKernel, predicate, value string) {
	t.Helper()
	if err := kernel.Assert(types.Fact{Predicate: predicate, Args: []any{value}}); err != nil {
		t.Fatalf("Assert(%s(%q)): %v", predicate, value, err)
	}
}

func assertConsumerCopies(t *testing.T, shards map[string]*KernelShard, predicate, value, owner string, want bool) {
	t.Helper()
	for _, domain := range []string{"world", "policy", "cortex"} {
		shard := shards[domain]
		facts, err := shard.kernel.Query(predicate)
		if err != nil {
			t.Fatalf("%s query %s: %v", domain, predicate, err)
		}
		found := false
		for _, fact := range facts {
			if len(fact.Args) == 1 && fact.Args[0] == value {
				found = true
			}
		}
		shouldHave := want && (domain == owner || domain == "policy")
		if found != shouldHave {
			t.Errorf("%s has %s(%q) = %t, want %t", domain, predicate, value, found, shouldHave)
		}
	}
}

func TestCortexPredicateConsumerRoutes_PlaceRuleAndMirrorEveryMutationPath(t *testing.T) {
	kernel, shards, routes := newConsumerRouteTestCortex(t)
	dm := kernel.derivationMap
	for _, predicate := range []string{"consumer_route_source", "consumer_route_aux"} {
		presence := dm.Presence[predicate]
		if !presence.Equals(Presence{Shards: map[string]struct{}{"world": {}, "policy": {}}}) && predicate == "consumer_route_source" {
			t.Fatalf("source presence = %+v, want world + policy", presence)
		}
		if predicate == "consumer_route_aux" && !presence.Equals(Presence{Shards: map[string]struct{}{"cortex": {}, "policy": {}}}) {
			t.Fatalf("aux presence = %+v, want cortex + policy", presence)
		}
	}
	if !dm.QueryTargets["consumer_route_gate"].Equals(SingleShardPresence("policy")) {
		t.Fatalf("consumer_route_gate query targets = %+v, want policy", dm.QueryTargets["consumer_route_gate"])
	}
	if !dm.ConsumerReplicas["consumer_route_source"].Equals(SingleShardPresence("policy")) {
		t.Fatalf("source replica placement = %+v, want policy", dm.ConsumerReplicas["consumer_route_source"])
	}
	if len(routes) != 2 {
		t.Fatalf("route count = %d, want 2", len(routes))
	}

	// Positive authority requires the complete evidence set on the policy
	// consumer. An incomplete set remains absent there and cannot derive a gate.
	assertConsumerRouteFact(t, kernel, "consumer_route_source", "allow")
	assertConsumerRouteFact(t, kernel, "consumer_route_aux", "allow")
	assertConsumerCopies(t, shards, "consumer_route_source", "allow", "world", true)
	assertConsumerCopies(t, shards, "consumer_route_aux", "allow", "cortex", true)
	rows, err := kernel.Query("consumer_route_gate")
	if err != nil || len(rows) != 0 {
		t.Fatalf("gate without policy fact = %v, err %v; want no allow", rows, err)
	}
	assertConsumerRouteFact(t, kernel, "consumer_route_policy", "allow")
	rows, err = kernel.Query("consumer_route_gate")
	if err != nil || len(rows) != 1 {
		t.Fatalf("gate with complete evidence = %v, err %v; want one policy-derived allow", rows, err)
	}

	// Assert, LoadFacts, AssertBatch, and transaction assertions all write the
	// source and consumer copy. Exact, first-argument, predicate-set, and
	// transaction retractions remove both copies.
	assertConsumerRouteFact(t, kernel, "consumer_route_source", "assert")
	assertConsumerCopies(t, shards, "consumer_route_source", "assert", "world", true)
	if err := kernel.RetractFact(types.Fact{Predicate: "consumer_route_source", Args: []any{"assert"}}); err != nil {
		t.Fatalf("RetractFact: %v", err)
	}
	assertConsumerCopies(t, shards, "consumer_route_source", "assert", "world", false)

	if err := kernel.LoadFacts([]types.Fact{{Predicate: "consumer_route_source", Args: []any{"load"}}}); err != nil {
		t.Fatalf("LoadFacts: %v", err)
	}
	assertConsumerCopies(t, shards, "consumer_route_source", "load", "world", true)
	if err := kernel.RetractExactFactsBatch([]types.Fact{{Predicate: "consumer_route_source", Args: []any{"load"}}}); err != nil {
		t.Fatalf("RetractExactFactsBatch: %v", err)
	}
	assertConsumerCopies(t, shards, "consumer_route_source", "load", "world", false)

	if err := kernel.AssertBatch([]types.Fact{{Predicate: "consumer_route_source", Args: []any{"batch"}}}); err != nil {
		t.Fatalf("AssertBatch: %v", err)
	}
	assertConsumerCopies(t, shards, "consumer_route_source", "batch", "world", true)
	if err := kernel.RemoveFactsByPredicateSet(map[string]struct{}{"consumer_route_source": {}}); err != nil {
		t.Fatalf("RemoveFactsByPredicateSet: %v", err)
	}
	assertConsumerCopies(t, shards, "consumer_route_source", "batch", "world", false)

	tx := kernel.Transaction()
	tx.Assert(types.Fact{Predicate: "consumer_route_source", Args: []any{"transaction"}})
	if err := tx.Commit(); err != nil {
		t.Fatalf("transaction Assert Commit: %v", err)
	}
	assertConsumerCopies(t, shards, "consumer_route_source", "transaction", "world", true)
	tx = kernel.Transaction()
	tx.RetractExactFact(types.Fact{Predicate: "consumer_route_source", Args: []any{"transaction"}})
	if err := tx.Commit(); err != nil {
		t.Fatalf("transaction RetractExactFact Commit: %v", err)
	}
	assertConsumerCopies(t, shards, "consumer_route_source", "transaction", "world", false)

	assertConsumerRouteFact(t, kernel, "consumer_route_source", "transaction-first-arg")
	tx = kernel.Transaction()
	tx.RetractFact(types.Fact{Predicate: "consumer_route_source", Args: []any{"transaction-first-arg"}})
	if err := tx.Commit(); err != nil {
		t.Fatalf("transaction RetractFact Commit: %v", err)
	}
	assertConsumerCopies(t, shards, "consumer_route_source", "transaction-first-arg", "world", false)

	assertConsumerRouteFact(t, kernel, "consumer_route_source", "transaction-predicate-set")
	tx = kernel.Transaction()
	tx.RetractPredicateSet(map[string]struct{}{"consumer_route_source": {}})
	if err := tx.Commit(); err != nil {
		t.Fatalf("transaction RetractPredicateSet Commit: %v", err)
	}
	assertConsumerCopies(t, shards, "consumer_route_source", "transaction-predicate-set", "world", false)

	assertConsumerRouteFact(t, kernel, "consumer_route_source", "retract-all")
	if err := kernel.Retract("consumer_route_source"); err != nil {
		t.Fatalf("Retract: %v", err)
	}
	assertConsumerCopies(t, shards, "consumer_route_source", "retract-all", "world", false)

	assertConsumerRouteFact(t, kernel, "consumer_route_source", "transaction-retract-all")
	tx = kernel.Transaction()
	tx.Retract("consumer_route_source")
	if err := tx.Commit(); err != nil {
		t.Fatalf("transaction Retract Commit: %v", err)
	}
	assertConsumerCopies(t, shards, "consumer_route_source", "transaction-retract-all", "world", false)
}

func TestCortexPredicateConsumerRoutes_ShardRouterMirrorsOwnerAndCatchAllWrites(t *testing.T) {
	kernel, shards, _ := newConsumerRouteTestCortex(t)
	kernel.SetFactRouter(NewShardFactRouter())

	// The per-shard router must notice consumer routes even when the caller is
	// already the canonical owner. Unowned MCP evidence routes from catch-all.
	if err := shards["world"].Assert(types.Fact{Predicate: "consumer_route_source", Args: []any{"owner"}}); err != nil {
		t.Fatalf("owner direct Assert: %v", err)
	}
	assertConsumerCopies(t, shards, "consumer_route_source", "owner", "world", true)
	if err := shards["cortex"].Assert(types.Fact{Predicate: "consumer_route_aux", Args: []any{"catchall"}}); err != nil {
		t.Fatalf("catch-all direct Assert: %v", err)
	}
	assertConsumerCopies(t, shards, "consumer_route_aux", "catchall", "cortex", true)

	if err := shards["world"].AssertBatch([]types.Fact{
		{Predicate: "consumer_route_source", Args: []any{"batch-a"}},
		{Predicate: "consumer_route_source", Args: []any{"batch-b"}},
	}); err != nil {
		t.Fatalf("owner direct AssertBatch: %v", err)
	}
	assertConsumerCopies(t, shards, "consumer_route_source", "batch-a", "world", true)
	assertConsumerCopies(t, shards, "consumer_route_source", "batch-b", "world", true)
	if err := shards["world"].RetractFact(types.Fact{Predicate: "consumer_route_source", Args: []any{"owner"}}); err != nil {
		t.Fatalf("owner direct RetractFact: %v", err)
	}
	assertConsumerCopies(t, shards, "consumer_route_source", "owner", "world", false)
	if err := shards["cortex"].Retract("consumer_route_aux"); err != nil {
		t.Fatalf("catch-all direct Retract: %v", err)
	}
	assertConsumerCopies(t, shards, "consumer_route_aux", "catchall", "cortex", false)
}

func TestCortexPredicateConsumerRoutes_FeatureFlagInstallsDirectRouter(t *testing.T) {
	features.SetActive(nil)
	t.Setenv("CODENERD_PER_SHARD_FACTS", "1")
	t.Cleanup(func() { features.SetActive(nil) })

	kernel, shards, _ := newConsumerRouteTestCortex(t)
	if kernel.FactRouter() == nil {
		t.Fatal("per-shard-facts flag did not install the direct shard router")
	}
	if err := shards["world"].Assert(types.Fact{Predicate: "consumer_route_source", Args: []any{"flag-owner"}}); err != nil {
		t.Fatalf("owner direct Assert with feature flag: %v", err)
	}
	assertConsumerCopies(t, shards, "consumer_route_source", "flag-owner", "world", true)
}

func TestCortexPredicateConsumerRoutes_RejectInvalidDestinations(t *testing.T) {
	kernel, _, _ := newConsumerRouteTestCortex(t)
	if err := kernel.SetPredicateConsumerRoutes([]PredicateConsumerRoute{{Predicate: "consumer_route_source", Consumer: "missing"}}); err == nil {
		t.Fatal("missing consumer was accepted")
	}
	if err := kernel.SetPredicateConsumerRoutes([]PredicateConsumerRoute{{Predicate: "consumer_route_source", Consumer: "world"}}); err == nil {
		t.Fatal("canonical owner was accepted as its own consumer")
	}
	if err := kernel.SetSharedPredicates([]string{"consumer_route_shared"}); err != nil {
		t.Fatalf("SetSharedPredicates: %v", err)
	}
	if err := kernel.SetPredicateConsumerRoutes([]PredicateConsumerRoute{{Predicate: "consumer_route_shared", Consumer: "policy"}}); err == nil {
		t.Fatal("shared predicate was accepted as consumer-routed")
	}
}
