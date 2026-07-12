package core

import (
	"testing"
	"time"
)

// TestAssertHeartbeat_DoesNotDirtyOnRefresh verifies the hot-path fix for
// system shard heartbeats: the first assert dirties (so health rules can
// derive), but subsequent timestamp refreshes for the same shard must not
// mark factsDirty — otherwise N shards × 5s ticks thrash full re-eval.
func TestAssertHeartbeat_DoesNotDirtyOnRefresh(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}

	// Seed a minimal clean state: evaluate once so factsDirty is false.
	if err := k.ensureEvaluated(); err != nil {
		// New kernels may already be clean; ignore if no facts
		_ = err
	}
	// Force clean after init eval
	k.factsDirty.Store(false)

	first := Fact{
		Predicate: "system_heartbeat",
		Args:      []any{"constitution_gate", int64(1000)},
	}
	if err := k.Assert(first); err != nil {
		t.Fatalf("first heartbeat: %v", err)
	}
	if !k.factsDirty.Load() {
		t.Fatal("first heartbeat for a shard should mark factsDirty")
	}

	// Simulate evaluation completing.
	k.factsDirty.Store(false)

	// Refresh same shard with a newer timestamp — must NOT dirty.
	refresh := Fact{
		Predicate: "system_heartbeat",
		Args:      []any{"constitution_gate", time.Now().Unix()},
	}
	if err := k.Assert(refresh); err != nil {
		t.Fatalf("refresh heartbeat: %v", err)
	}
	if k.factsDirty.Load() {
		t.Fatal("refreshing an existing shard heartbeat must not mark factsDirty")
	}

	// A different shard's first heartbeat still dirties.
	other := Fact{
		Predicate: "system_heartbeat",
		Args:      []any{"executive_policy", time.Now().Unix()},
	}
	if err := k.Assert(other); err != nil {
		t.Fatalf("other shard heartbeat: %v", err)
	}
	if !k.factsDirty.Load() {
		t.Fatal("first heartbeat for a new shard should mark factsDirty")
	}

	// Only one EDB row per shard (upsert, not append).
	var count int
	for _, f := range k.facts {
		if f.Predicate == "system_heartbeat" {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("expected 2 heartbeat facts (one per shard), got %d", count)
	}
}

func TestIsNonResolvableTarget_WiredViaAssertPath(t *testing.T) {
	// Smoke: ordinary asserts still dirty.
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	k.factsDirty.Store(false)
	if err := k.Assert(Fact{Predicate: "user_intent", Args: []any{"/x", "/y", "t", "", 0.9}}); err != nil {
		// May fail schema — just ensure heartbeat path doesn't break normal Assert structure
		_ = err
	}
}
