package system

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/core"
	coreshards "codenerd/internal/core/shards"
	shardsystem "codenerd/internal/shards/system"
	"codenerd/internal/types"
)

// TestOnDemandActivationEndToEnd is the falsifiable proof for runtime
// on-demand activation, exercising the whole chain with real components and
// no scripted derivation:
//
//	kernel asserts modified/1 → policy derives activate_shard/1 → the
//	event-bus watcher wakes → EnsureOnDemandShards spawns the ingestor →
//	the ingestor scans and asserts file_topology/1.
//
// Every hop was verified in isolation elsewhere; this test proves the hops
// connect. If any link regresses (policy stops deriving, the watcher stops
// waking, the spawn stops landing, the scan stops asserting), no
// file_topology facts appear and the test fails on the deadline.
func TestOnDemandActivationEndToEnd(t *testing.T) {
	dir := t.TempDir()
	src := "package seed\n\nfunc SeedFunc() int { return 42 }\n"
	if err := os.WriteFile(filepath.Join(dir, "seed.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

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
	sm.RegisterShard("world_model_ingestor", func(id string, cfg types.ShardConfig) types.ShardAgent {
		shardCfg := shardsystem.DefaultWorldModelConfig()
		shardCfg.RootPath = dir
		shard := shardsystem.NewWorldModelIngestorShardWithConfig(shardCfg)
		shard.SetParentKernel(kernel)
		return shard
	})
	sm.DefineProfile("world_model_ingestor", types.ShardConfig{
		Name:        "world_model_ingestor",
		Type:        types.ShardTypeSystem,
		StartupMode: types.StartupOnDemand,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stop := core.StartOnDemandWatcher(ctx, bus, sm.EnsureOnDemandShards)
	defer stop()
	defer sm.StopAll()

	// Rendezvous with the subscription: publishing before the watcher's
	// goroutine subscribes loses the event by design (pub/sub has no
	// backlog), which would push this test onto the 30s fallback sweep
	// instead of the event path it means to exercise.
	subDeadline := time.Now().Add(5 * time.Second)
	for bus.SubscriberCount() == 0 && time.Now().Before(subDeadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if bus.SubscriberCount() == 0 {
		t.Fatal("watcher never subscribed within 5s")
	}

	// The trigger a file write would assert through the VirtualStore.
	if err := kernel.Assert(core.Fact{Predicate: "modified", Args: []any{"/some/changed.go"}}); err != nil {
		t.Fatalf("assert modified: %v", err)
	}

	deadline := time.Now().Add(60 * time.Second)
	for {
		facts, err := kernel.Query("file_topology")
		if err != nil {
			t.Fatalf("query file_topology: %v", err)
		}
		for _, f := range facts {
			for _, arg := range f.Args {
				if s, ok := arg.(string); ok && strings.Contains(s, "seed.go") {
					return // Chain complete: trigger became world facts.
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
			t.Fatalf("no file_topology for seed.go within 60s (active=%v activate_shard=%v qerr=%v facts=%d)",
				names, activated, qerr, len(facts))
		}
		time.Sleep(200 * time.Millisecond)
	}
}
