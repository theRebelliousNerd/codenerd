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

// TestRouterActivationEndToEnd proves the second on-demand rule with real
// components: asserting action_ready_for_routing/1 derives
// activate_shard(/tactile_router), the watcher spawns the router, and the
// running router deterministically answers an unroutable permitted action
// with a routing_result failure fact — the no-handler path needs no tools,
// so the test is hermetic.
func TestRouterActivationEndToEnd(t *testing.T) {
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
	sm.RegisterShard("tactile_router", func(id string, cfg types.ShardConfig) types.ShardAgent {
		shard := shardsystem.NewTactileRouterShard()
		shard.SetParentKernel(kernel)
		return shard
	})
	sm.DefineProfile("tactile_router", types.ShardConfig{
		Name:        "tactile_router",
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

	if err := kernel.Assert(core.Fact{Predicate: "action_ready_for_routing", Args: []any{"act-1"}}); err != nil {
		t.Fatalf("assert trigger: %v", err)
	}
	if err := kernel.Assert(core.Fact{Predicate: "permitted_action", Args: []any{
		"act-1", types.MangleAtom("/no_such_action_xyz"), "tgt", "", int64(0),
	}}); err != nil {
		t.Fatalf("assert permitted_action: %v", err)
	}

	deadline := time.Now().Add(60 * time.Second)
	for {
		rows, err := kernel.Query("routing_result")
		if err != nil {
			t.Fatalf("query routing_result: %v", err)
		}
		for _, f := range rows {
			if len(f.Args) >= 3 && f.Args[0] == "act-1" {
				if atom, ok := f.Args[1].(types.MangleAtom); ok && string(atom) == "/failure" && f.Args[2] == "no_handler" {
					return // Chain complete: trigger spawned a router that answered.
				}
				if s, ok := f.Args[1].(string); ok && s == "/failure" && f.Args[2] == "no_handler" {
					return
				}
			}
		}
		if time.Now().After(deadline) {
			active := sm.GetActiveShards()
			names := make([]string, 0, len(active))
			for _, a := range active {
				if a != nil {
					names = append(names, a.GetConfig().Name)
				}
			}
			activated, qerr := kernel.Query("activate_shard")
			t.Fatalf("no routing_result for act-1 within 60s (active=%v activate_shard=%v qerr=%v rows=%d)",
				names, activated, qerr, len(rows))
		}
		time.Sleep(200 * time.Millisecond)
	}
}
