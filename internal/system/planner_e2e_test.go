package system

import (
	"context"
	"testing"
	"time"

	"codenerd/internal/core"
	coreshards "codenerd/internal/core/shards"
	shardsystem "codenerd/internal/shards/system"
	"codenerd/internal/types"
)

// TestPlannerActivationEndToEnd proves the third on-demand rule: asserting
// current_campaign/1 derives activate_shard(/session_planner) and the
// watcher spawns a planner that enters its await-goals loop instead of
// decomposing the activation label. No LLM client is attached, so any
// attempt to decompose would error the shard out — sustained activity
// proves the label path.
func TestPlannerActivationEndToEnd(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	bus := kernel.GetEventBus()
	if bus == nil {
		t.Fatal("real kernel must expose a fact event bus")
	}

	sm := coreshards.NewShardManager()
	sm.SetParentKernel(kernel)
	sm.RegisterShard("session_planner", func(id string, cfg types.ShardConfig) types.ShardAgent {
		shard := shardsystem.NewSessionPlannerShard()
		shard.SetParentKernel(kernel)
		return shard
	})
	sm.DefineProfile("session_planner", types.ShardConfig{
		Name:        "session_planner",
		Type:        types.ShardTypeSystem,
		StartupMode: types.StartupOnDemand,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stop := core.StartOnDemandWatcher(ctx, bus, sm.EnsureOnDemandShards)
	defer stop()
	defer sm.StopAll()

	subDeadline := time.Now().Add(5 * time.Second)
	for bus.SubscriberCount() == 0 && time.Now().Before(subDeadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if bus.SubscriberCount() == 0 {
		t.Fatal("watcher never subscribed within 5s")
	}

	if err := kernel.Assert(core.Fact{Predicate: "current_campaign", Args: []any{"c1"}}); err != nil {
		t.Fatalf("assert trigger: %v", err)
	}

	activeWithName := func() bool {
		for _, a := range sm.GetActiveShards() {
			if a != nil && a.GetConfig().Name == "session_planner" {
				return true
			}
		}
		return false
	}
	deadline := time.Now().Add(30 * time.Second)
	for !activeWithName() && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
	if !activeWithName() {
		activated, qerr := kernel.Query("activate_shard")
		t.Fatalf("planner never spawned within 30s (activate_shard=%v qerr=%v)", activated, qerr)
	}

	// The planner must stay up awaiting goals. A shard that tried to
	// decompose the label without an LLM client would error out and be
	// reaped within milliseconds.
	time.Sleep(2 * time.Second)
	if !activeWithName() {
		t.Fatal("planner exited after spawn; it must await goals on the activation label")
	}
}
