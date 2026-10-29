package shards

import (
	"context"
	"testing"

	"codenerd/internal/types"
	"codenerd/internal/types/typestest"
)

type onDemandFakeAgent struct {
	id     string
	config types.ShardConfig
	// release, when set, holds Execute until the test ends, the way a live
	// on-demand system shard runs until it is stopped.
	release <-chan struct{}
}

// Execute returning at once made "while the shard is active" a race between
// the agent's goroutine and the test's second ensure: the manager drops a
// finished agent from its active set, and once, under a full `go test ./...`
// (2026-09-19), the goroutine won and the second ensure spawned again.
func (a *onDemandFakeAgent) Execute(ctx context.Context, _ string) (string, error) {
	if a.release != nil {
		select {
		case <-a.release:
		case <-ctx.Done():
		}
	}
	return "on-demand work", nil
}
func (a *onDemandFakeAgent) GetID() string                           { return a.id }
func (a *onDemandFakeAgent) GetState() types.ShardState              { return types.ShardStateRunning }
func (a *onDemandFakeAgent) GetConfig() types.ShardConfig            { return a.config }
func (a *onDemandFakeAgent) Stop() error                             { return nil }
func (a *onDemandFakeAgent) SetParentKernel(types.Kernel)            {}
func (a *onDemandFakeAgent) SetLLMClient(types.LLMClient)            {}
func (a *onDemandFakeAgent) SetSessionContext(*types.SessionContext) {}

func activateFact(atom string) types.Fact {
	return types.Fact{Predicate: "activate_shard", Args: []any{atom}}
}

func onDemandTestManager(t *testing.T, activate []types.Fact) *ShardManager {
	t.Helper()
	sm := NewShardManager()
	kernel := typestest.NewMockKernel()
	if err := kernel.LoadFacts(activate); err != nil {
		t.Fatalf("LoadFacts: %v", err)
	}
	sm.SetParentKernel(kernel)
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	sm.RegisterShard("world_model_ingestor", func(id string, cfg types.ShardConfig) types.ShardAgent {
		return &onDemandFakeAgent{id: id, config: cfg, release: release}
	})
	sm.DefineProfile("world_model_ingestor", types.ShardConfig{
		Name:        "world_model_ingestor",
		Type:        types.ShardTypeSystem,
		StartupMode: types.StartupOnDemand,
	})
	return sm
}

func TestEnsureOnDemandShardsSpawnsDerived(t *testing.T) {
	sm := onDemandTestManager(t, []types.Fact{activateFact("/world_model_ingestor")})

	spawned := sm.EnsureOnDemandShards(context.Background())

	if len(spawned) != 1 || spawned[0] != "world_model_ingestor" {
		t.Fatalf("expected [world_model_ingestor], got %v", spawned)
	}
	if !sm.hasActiveShardOfType("world_model_ingestor") {
		t.Fatal("shard was reported spawned but is not active")
	}
}

func TestEnsureOnDemandShardsNeverDoubleSpawns(t *testing.T) {
	sm := onDemandTestManager(t, []types.Fact{activateFact("/world_model_ingestor")})

	first := sm.EnsureOnDemandShards(context.Background())
	second := sm.EnsureOnDemandShards(context.Background())

	if len(first) != 1 {
		t.Fatalf("first ensure must spawn once, got %v", first)
	}
	if len(second) != 0 {
		t.Fatalf("second ensure must be a no-op while the shard is active, got %v", second)
	}
	if n := len(sm.GetActiveShards()); n != 1 {
		t.Fatalf("expected exactly 1 active shard, got %d", n)
	}
}

func TestEnsureOnDemandShardsSkipsAutoProfiles(t *testing.T) {
	sm := NewShardManager()
	kernel := typestest.NewMockKernel()
	if err := kernel.LoadFacts([]types.Fact{activateFact("/auto_shard")}); err != nil {
		t.Fatal(err)
	}
	sm.SetParentKernel(kernel)
	sm.RegisterShard("auto_shard", func(id string, cfg types.ShardConfig) types.ShardAgent {
		return &onDemandFakeAgent{id: id, config: cfg}
	})
	sm.DefineProfile("auto_shard", types.ShardConfig{
		Name:        "auto_shard",
		Type:        types.ShardTypeSystem,
		StartupMode: types.StartupAuto,
	})

	if spawned := sm.EnsureOnDemandShards(context.Background()); len(spawned) != 0 {
		t.Fatalf("auto profiles belong to the boot path, must not spawn here: %v", spawned)
	}
}

func TestEnsureOnDemandShardsSkipsDisabled(t *testing.T) {
	sm := onDemandTestManager(t, []types.Fact{activateFact("/world_model_ingestor")})
	sm.DisableSystemShard("world_model_ingestor")

	if spawned := sm.EnsureOnDemandShards(context.Background()); len(spawned) != 0 {
		t.Fatalf("disabled shard must not spawn: %v", spawned)
	}
}

func TestEnsureOnDemandShardsNilKernelIsNoop(t *testing.T) {
	sm := NewShardManager()
	if spawned := sm.EnsureOnDemandShards(context.Background()); spawned != nil {
		t.Fatalf("nil kernel must yield nil, got %v", spawned)
	}
}

func TestEnsureOnDemandShardsUnknownNameIsSkipped(t *testing.T) {
	sm := onDemandTestManager(t, []types.Fact{activateFact("/no_such_shard")})
	if spawned := sm.EnsureOnDemandShards(context.Background()); len(spawned) != 0 {
		t.Fatalf("unknown shard names must be skipped, got %v", spawned)
	}
}
