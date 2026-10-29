package shards

import (
	"context"
	"runtime"
	"testing"

	"codenerd/internal/types"
	"codenerd/internal/types/typestest"
)

// Each on-demand activation is a spawn nobody reads the result of. SpawnAsync
// retained every finished shard's result for a GetResult that never came, so
// every activation for the life of the process left an entry behind. A
// detached spawn's outcome is audited and shown; it is not retained.
func TestOnDemandActivationRetainsNoResult(t *testing.T) {
	sm := NewShardManager()
	kernel := typestest.NewMockKernel()
	if err := kernel.LoadFacts([]types.Fact{activateFact("/world_model_ingestor")}); err != nil {
		t.Fatalf("LoadFacts: %v", err)
	}
	sm.SetParentKernel(kernel)
	sm.RegisterShard("world_model_ingestor", func(id string, cfg types.ShardConfig) types.ShardAgent {
		return &onDemandFakeAgent{id: id, config: cfg}
	})
	sm.DefineProfile("world_model_ingestor", types.ShardConfig{
		Name: "world_model_ingestor", Type: types.ShardTypeSystem, StartupMode: types.StartupOnDemand,
	})

	for range 3 {
		if spawned := sm.EnsureOnDemandShards(context.Background()); len(spawned) != 1 {
			t.Fatalf("an activation spawned %v, want one shard", spawned)
		}
		// The agent returns at once; the manager drops it from the active set
		// in the same critical section that records (or skips) its result.
		for sm.hasActiveShardOfType("world_model_ingestor") {
			runtime.Gosched()
		}
	}

	sm.mu.RLock()
	retained, marked := len(sm.results), len(sm.detached)
	sm.mu.RUnlock()
	if retained != 0 || marked != 0 {
		t.Fatalf("after 3 finished activations the manager retains %d result(s) and %d detached mark(s), want 0 and 0", retained, marked)
	}
}

// A spawn someone waits on keeps its result for them.
func TestAttachedSpawnKeepsItsResult(t *testing.T) {
	sm := NewShardManager()
	sm.RegisterShard("worker", func(id string, cfg types.ShardConfig) types.ShardAgent {
		return &onDemandFakeAgent{id: id, config: cfg}
	})
	sm.DefineProfile("worker", types.ShardConfig{Name: "worker", Type: types.ShardTypeEphemeral})

	id, err := sm.SpawnAsync(context.Background(), "worker", "task")
	if err != nil {
		t.Fatalf("SpawnAsync: %v", err)
	}
	for {
		if res, ok := sm.GetResult(id); ok {
			if res.Result != "on-demand work" {
				t.Fatalf("result = %q", res.Result)
			}
			return
		}
		runtime.Gosched()
	}
}
