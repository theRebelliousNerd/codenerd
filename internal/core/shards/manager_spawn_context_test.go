package shards

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"codenerd/internal/types"
)

type capabilityCheckAgent struct {
	*BaseShardAgent
	want types.ModelCapability
}

func (a *capabilityCheckAgent) Execute(ctx context.Context, task string) (string, error) {
	got, ok := types.ModelCapabilityFromContext(ctx)
	if !ok {
		return "", fmt.Errorf("missing model capability context value")
	}
	if got != a.want {
		return "", fmt.Errorf("model capability hint = %q, want %q", got, a.want)
	}
	return "ok", nil
}

type modelNameCheckAgent struct {
	*BaseShardAgent
	want string
}

func (a *modelNameCheckAgent) Execute(ctx context.Context, task string) (string, error) {
	got, ok := types.ModelNameFromContext(ctx)
	if !ok {
		return "", fmt.Errorf("missing model name context value")
	}
	if got != a.want {
		return "", fmt.Errorf("model name hint = %q, want %q", got, a.want)
	}
	return "ok", nil
}

func TestShardManager_ModelCapabilityContextHint(t *testing.T) {
	sm := NewShardManager()

	want := types.CapabilityHighReasoning
	sm.RegisterShard("cap_test", func(id string, cfg types.ShardConfig) types.ShardAgent {
		base := NewBaseShardAgent(id, cfg)
		return &capabilityCheckAgent{
			BaseShardAgent: base,
			want:           want,
		}
	})
	sm.DefineProfile("cap_test", types.ShardConfig{
		Name:    "cap_test",
		Type:    types.ShardTypeEphemeral,
		Timeout: 2 * time.Second,
		Model: types.ModelConfig{
			Capability: want,
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := sm.SpawnWithContext(ctx, "cap_test", "task", nil)
	if err != nil {
		t.Fatalf("SpawnWithContext error: %v", err)
	}
	if res != "ok" {
		t.Fatalf("result=%q, want ok", res)
	}
}

func TestShardManager_ModelNameContextHint(t *testing.T) {
	sm := NewShardManager()

	want := "gpt-5.4"
	sm.RegisterShard("model_name_test", func(id string, cfg types.ShardConfig) types.ShardAgent {
		base := NewBaseShardAgent(id, cfg)
		return &modelNameCheckAgent{
			BaseShardAgent: base,
			want:           want,
		}
	})
	sm.DefineProfile("model_name_test", types.ShardConfig{
		Name:    "model_name_test",
		Type:    types.ShardTypeEphemeral,
		Timeout: 2 * time.Second,
		Model: types.ModelConfig{
			Name:       want,
			Capability: types.CapabilityBalanced,
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := sm.SpawnWithContext(ctx, "model_name_test", "task", nil)
	if err != nil {
		t.Fatalf("SpawnWithContext error: %v", err)
	}
	if res != "ok" {
		t.Fatalf("result=%q, want ok", res)
	}
}

type cancellationIgnoringAgent struct {
	*BaseShardAgent
	started chan struct{}
	release chan struct{}
}

func (a *cancellationIgnoringAgent) Execute(context.Context, string) (string, error) {
	close(a.started)
	<-a.release
	return "completed after waiter left", nil
}

func TestShardManager_CanceledSynchronousWaiterDoesNotRetainResult(t *testing.T) {
	sm := NewShardManager()
	started := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseShard := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(releaseShard)
	registered := make(chan string, 1)
	unregistered := make(chan string, 1)
	nerdDir := t.TempDir()
	dbPath := filepath.Join(nerdDir, "shards", "cancel_test_knowledge.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatalf("create shard data directory: %v", err)
	}
	if err := os.WriteFile(dbPath, []byte("test"), 0o600); err != nil {
		t.Fatalf("create shard database marker: %v", err)
	}
	sm.SetNerdDir(nerdDir)
	sm.SetJITRegistrar(func(name, path string) error {
		if path != dbPath {
			t.Errorf("JIT registration path = %q, want %q", path, dbPath)
		}
		registered <- name
		return nil
	})
	sm.SetJITUnregistrar(func(name string) { unregistered <- name })
	sm.RegisterShard("cancel_test", func(id string, cfg types.ShardConfig) types.ShardAgent {
		return &cancellationIgnoringAgent{
			BaseShardAgent: NewBaseShardAgent(id, cfg),
			started:        started,
			release:        release,
		}
	})
	sm.DefineProfile("cancel_test", types.ShardConfig{
		Name: "cancel_test",
		Type: types.ShardTypePersistent,
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	waitResult := make(chan error, 1)
	go func() {
		_, err := sm.SpawnWithContext(ctx, "cancel_test", "task", nil)
		waitResult <- err
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("shard did not start")
	}
	select {
	case name := <-registered:
		if name != "cancel_test" {
			t.Fatalf("registered shard = %q, want cancel_test", name)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("persistent shard JIT database was not registered")
	}
	cancel()
	select {
	case err := <-waitResult:
		if err != context.Canceled {
			t.Fatalf("SpawnWithContext error = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("synchronous waiter did not return after cancellation")
	}
	sm.mu.RLock()
	active, results, detached := len(sm.shards), len(sm.results), len(sm.detached)
	sm.mu.RUnlock()
	if active != 1 || results != 0 || detached != 1 {
		t.Fatalf("while canceled shard is still executing: active=%d results=%d detached=%d, want 1, 0, 1", active, results, detached)
	}

	// Let Execute finish after the synchronous caller has abandoned its result.
	releaseShard()
	select {
	case name := <-unregistered:
		if name != "cancel_test" {
			t.Fatalf("unregistered shard = %q, want cancel_test", name)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("late shard completion did not unregister its JIT database")
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		sm.mu.RLock()
		active, results, detached, jitDBs := len(sm.shards), len(sm.results), len(sm.detached), len(sm.activeJITDBs)
		sm.mu.RUnlock()
		if active == 0 && results == 0 && detached == 0 && jitDBs == 0 {
			return
		}
		runtime.Gosched()
	}
	sm.mu.RLock()
	active, results, detached, jitDBs := len(sm.shards), len(sm.results), len(sm.detached), len(sm.activeJITDBs)
	sm.mu.RUnlock()
	t.Fatalf("after canceled waiter and completed shard: active=%d results=%d detached=%d JITDBs=%d, want all zero", active, results, detached, jitDBs)
}
