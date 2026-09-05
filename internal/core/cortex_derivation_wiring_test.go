package core

import (
	"testing"

	"codenerd/internal/types"
)

// Wiring policy for the derivation-map tests: R1 joins the shared
// user_intent with the world-owned file_topology (so it derives in world and
// world consumes user_intent); R2 joins the shared current_time with the
// unowned wiring_loose fact (so it derives in the catch-all and the catch-all
// consumes current_time); nothing reads derived_mode, so routing consumes
// neither shared predicate.
const derivationWiringPolicy = `
# Policy Module: wiring.mg
Decl user_intent(X).
Decl current_time(X).
Decl derived_mode(X).
Decl file_topology(X).
Decl wiring_loose(X).
Decl wiring_r1_head(X).
Decl wiring_r2_head(X).

wiring_r1_head(X) :- user_intent(X), file_topology(X).
wiring_r2_head(X) :- current_time(X), wiring_loose(X).
`

func wiringOwners() map[string]string {
	return map[string]string{"derived_mode": "routing", "file_topology": "world"}
}

func wiringShared() map[string]struct{} {
	return map[string]struct{}{"user_intent": {}, "current_time": {}}
}

func buildWiringMap(t *testing.T) *DerivationMap {
	t.Helper()
	m, err := BuildDerivationMap(derivationWiringPolicy, nil, wiringOwners(), wiringShared(), "cortex")
	if err != nil {
		t.Fatalf("BuildDerivationMap: %v", err)
	}
	if _, ok := m.Consumes["world"]["user_intent"]; !ok {
		t.Fatalf("world must consume user_intent through wiring_r1_head: %+v", m.Consumes)
	}
	if _, ok := m.Consumes["cortex"]["current_time"]; !ok {
		t.Fatalf("catch-all must consume current_time through wiring_r2_head: %+v", m.Consumes)
	}
	if len(m.Consumes["routing"]) != 0 {
		t.Fatalf("routing must consume neither shared predicate: %+v", m.Consumes["routing"])
	}
	if got := m.ShardsFor("wiring_r1_head", []string{"cortex", "routing", "world"}); len(got) != 1 || got[0] != "world" {
		t.Fatalf("ShardsFor(wiring_r1_head) = %v, want [world]", got)
	}
	return m
}

// wiringCortex builds the three-shard shape the task pins: routing owns
// derived_mode, world owns file_topology, cortex is the catch-all. The wiring
// heads and the unknown probe are declared in every shard so fan-out queries
// resolve without declaration warnings. When m is non-nil it is installed
// after the shared predicates, matching the production wiring order.
func wiringCortex(t *testing.T, m *DerivationMap) (*CortexKernel, map[string]*KernelShard) {
	t.Helper()
	cortex := NewCortexKernel("cortex")
	if err := cortex.SetSharedPredicates([]string{"user_intent", "current_time"}); err != nil {
		t.Fatalf("SetSharedPredicates: %v", err)
	}
	routing, err := NewKernelShard(KernelShardConfig{Domain: "routing", OwnedPredicates: []string{"derived_mode"}})
	if err != nil {
		t.Fatalf("routing shard: %v", err)
	}
	world, err := NewKernelShard(KernelShardConfig{Domain: "world", OwnedPredicates: []string{"file_topology"}})
	if err != nil {
		t.Fatalf("world shard: %v", err)
	}
	catchAll, err := NewKernelShard(KernelShardConfig{Domain: "cortex"})
	if err != nil {
		t.Fatalf("cortex shard: %v", err)
	}
	shards := map[string]*KernelShard{"routing": routing, "world": world, "cortex": catchAll}
	for _, s := range shards {
		if err := cortex.RegisterShard(s); err != nil {
			t.Fatalf("register %s: %v", s.Domain(), err)
		}
	}
	for _, s := range shards {
		s.kernel.AppendPolicy("Decl wiring_r1_head(X).\nDecl wiring_r2_head(X).\nDecl wiring_loose(X).\nDecl wiring_mystery(X).")
		if err := s.Evaluate(); err != nil {
			t.Fatalf("shard %s declare wiring preds: %v", s.Domain(), err)
		}
	}
	// Pin the local-query path regardless of the per-shard-facts flag so the
	// queryCount assertions below observe fan-out scope, not router dispatch.
	cortex.SetFactRouter(nil)
	if m != nil {
		cortex.SetDerivationMap(m)
	}
	return cortex, shards
}

func wiringLocalCount(t *testing.T, shards map[string]*KernelShard, domain, pred string) int {
	t.Helper()
	facts, err := shards[domain].queryLocal(pred)
	if err != nil {
		t.Fatalf("shard %s queryLocal %s: %v", domain, pred, err)
	}
	return len(facts)
}

func TestCortexDerivationWiring_SharedReplicationNarrowed(t *testing.T) {
	m := buildWiringMap(t)
	cortex, shards := wiringCortex(t, m)

	cortexAssert(t, cortex, "user_intent", "/current_intent", types.MangleAtom("/mutation"), types.MangleAtom("/fix"), "wiring probe", "none")
	if n := wiringLocalCount(t, shards, "world", "user_intent"); n != 1 {
		t.Fatalf("world holds %d user_intent facts, want 1", n)
	}
	if n := wiringLocalCount(t, shards, "cortex", "user_intent"); n != 1 {
		t.Fatalf("catch-all holds %d user_intent facts, want 1", n)
	}
	if n := wiringLocalCount(t, shards, "routing", "user_intent"); n != 0 {
		t.Fatalf("routing holds %d user_intent facts, want 0 (consumes neither shared predicate)", n)
	}

	if err := cortex.Assert(types.Fact{Predicate: "current_time", Args: []any{int64(12345)}}); err != nil {
		t.Fatalf("assert current_time: %v", err)
	}
	if n := wiringLocalCount(t, shards, "cortex", "current_time"); n != 1 {
		t.Fatalf("catch-all holds %d current_time facts, want 1", n)
	}
	if n := wiringLocalCount(t, shards, "world", "current_time"); n != 0 {
		t.Fatalf("world holds %d current_time facts, want 0", n)
	}
	if n := wiringLocalCount(t, shards, "routing", "current_time"); n != 0 {
		t.Fatalf("routing holds %d current_time facts, want 0", n)
	}

	if err := cortex.Retract("user_intent"); err != nil {
		t.Fatalf("retract user_intent: %v", err)
	}
	for _, domain := range []string{"world", "cortex", "routing"} {
		if n := wiringLocalCount(t, shards, domain, "user_intent"); n != 0 {
			t.Fatalf("shard %s still holds %d user_intent facts after Retract", domain, n)
		}
	}
}

func TestCortexDerivationWiring_QueryFanoutNarrowed(t *testing.T) {
	m := buildWiringMap(t)
	cortex, shards := wiringCortex(t, m)

	before := map[string]int64{}
	for domain, s := range shards {
		before[domain] = s.Metrics().QueryCount
	}
	if _, err := cortex.Query("wiring_r1_head"); err != nil {
		t.Fatalf("query wiring_r1_head: %v", err)
	}
	if got := shards["world"].Metrics().QueryCount; got != before["world"]+1 {
		t.Fatalf("world queryCount = %d, want %d (r1_head derives only in world)", got, before["world"]+1)
	}
	if got := shards["routing"].Metrics().QueryCount; got != before["routing"] {
		t.Fatalf("routing queryCount = %d, want %d (fan-out must skip routing)", got, before["routing"])
	}
	if got := shards["cortex"].Metrics().QueryCount; got != before["cortex"] {
		t.Fatalf("catch-all queryCount = %d, want %d (fan-out must skip the catch-all)", got, before["cortex"])
	}

	for domain, s := range shards {
		before[domain] = s.Metrics().QueryCount
	}
	if _, err := cortex.Query("wiring_mystery"); err != nil {
		t.Fatalf("query wiring_mystery: %v", err)
	}
	for domain, s := range shards {
		if got := s.Metrics().QueryCount; got != before[domain]+1 {
			t.Fatalf("shard %s queryCount = %d, want %d (unknown predicate must fan out everywhere)", domain, got, before[domain]+1)
		}
	}
}

func TestCortexDerivationWiring_NilMapReplicatesEverywhere(t *testing.T) {
	cortex, shards := wiringCortex(t, nil)

	cortexAssert(t, cortex, "user_intent", "/current_intent", types.MangleAtom("/mutation"), types.MangleAtom("/fix"), "nil-map probe", "none")
	for _, domain := range []string{"routing", "world", "cortex"} {
		if n := wiringLocalCount(t, shards, domain, "user_intent"); n != 1 {
			t.Fatalf("nil-map path: shard %s holds %d user_intent facts, want 1 (today's replicate-everywhere behaviour)", domain, n)
		}
	}
}
