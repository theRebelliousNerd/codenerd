package shards

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"codenerd/internal/types"
	"codenerd/internal/usage"
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
	sm.SetJITRegistrar(func(name, path string) (func(), error) {
		if path != dbPath {
			t.Errorf("JIT registration path = %q, want %q", path, dbPath)
		}
		registered <- name
		return func() { unregistered <- name }, nil
	})
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

func TestShardManager_JITAcquisitionFailureStopsBeforeExecution(t *testing.T) {
	sm := NewShardManager()
	nerdDir := t.TempDir()
	dbPath := types.AgentKnowledgeDBPath(nerdDir, "expert")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dbPath, []byte("marker"), 0o600); err != nil {
		t.Fatal(err)
	}
	sm.SetNerdDir(nerdDir)
	acquisitionErr := errors.New("knowledge database unavailable")
	sm.SetJITRegistrar(func(string, string) (func(), error) { return nil, acquisitionErr })
	started := make(chan struct{})
	sm.RegisterShard("expert", func(id string, cfg types.ShardConfig) types.ShardAgent {
		return &cancellationIgnoringAgent{BaseShardAgent: NewBaseShardAgent(id, cfg), started: started, release: started}
	})
	sm.DefineProfile("expert", types.ShardConfig{Name: "expert", Type: types.ShardTypeUser})
	if _, err := sm.Spawn(t.Context(), "expert", "consult"); !errors.Is(err, acquisitionErr) {
		t.Fatalf("Spawn = %v, want the acquisition failure", err)
	}
	select {
	case <-started:
		t.Fatal("shard executed after database acquisition failed")
	default:
	}
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if len(sm.activeJITDBs) != 0 || len(sm.shards) != 0 || len(sm.results) != 0 {
		t.Fatal("failed acquisition left active execution state")
	}
}

// usageReportingAgent reports at the provider boundary using only the context
// delivered by the manager, so the test checks spawn wiring, not tracker setup.
type usageReportingAgent struct {
	*BaseShardAgent
	failure error
}

func (a *usageReportingAgent) Execute(ctx context.Context, task string) (string, error) {
	usage.RecordFromContext(ctx, usage.Report{
		Model: "gpt-4o", Provider: "openai", Operation: "completion",
		Usage: types.UsageMetadata{InputTokens: 11, OutputTokens: 2},
	})
	return "answer", a.failure
}

func TestShardManager_UsageIdentifiesPersonaNotLifecycle(t *testing.T) {
	for _, tc := range []struct {
		name      string
		lifecycle types.ShardType
		fail      bool
	}{
		{"ephemeral", types.ShardTypeEphemeral, false},
		{"persistent", types.ShardTypePersistent, false},
		{"user", types.ShardTypeUser, false},
		{"system", types.ShardTypeSystem, false},
		{"failed call", types.ShardTypeEphemeral, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ws := t.TempDir()
			tr, err := usage.NewTracker(ws)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := tr.Close(); err != nil {
					t.Error(err)
				}
			})
			providerErr := errors.New("provider failed after reporting usage")
			sm := NewShardManager()
			sm.SetSessionID("manager-session")
			sm.RegisterShard("goexpert", func(id string, cfg types.ShardConfig) types.ShardAgent {
				a := &usageReportingAgent{BaseShardAgent: NewBaseShardAgent(id, cfg)}
				if tc.fail {
					a.failure = providerErr
				}
				return a
			})
			sm.DefineProfile("goexpert", types.ShardConfig{Name: "Go expert", Type: tc.lifecycle, Timeout: 2 * time.Second})
			ctx, cancel := context.WithTimeout(usage.NewContext(context.Background(), tr), 5*time.Second)
			defer cancel()
			ctx = usage.WithSessionID(ctx, "caller-session")
			usage.RecordFromContext(ctx, usage.Report{Model: "gpt-4o", Provider: "openai", Operation: "outside", Usage: types.UsageMetadata{InputTokens: 3}})
			parent := usage.WithShardContext(ctx, "parent-coder", "coder", "caller-session")
			_, err = sm.SpawnWithContext(parent, "goexpert", "review the change", nil)
			if tc.fail {
				if !errors.Is(err, providerErr) {
					t.Fatalf("error=%v, want provider failure", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if err := tr.Flush(); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(ws, ".nerd", "usage.json"))
			if err != nil {
				t.Fatal(err)
			}
			var persisted usage.UsageData
			if err := json.Unmarshal(data, &persisted); err != nil {
				t.Fatal(err)
			}
			a := persisted.Aggregate
			if a.ByShardType["goexpert"].Total != 13 || a.ByShardName["Go expert"].Total != 13 {
				t.Fatalf("child identity lost: types=%+v names=%+v", a.ByShardType, a.ByShardName)
			}
			if a.ByShardType[string(tc.lifecycle)].Total != 0 || a.ByShardName["parent-coder"].Total != 0 {
				t.Fatal("charged lifecycle or parent rather than the active shard")
			}
			if a.ByShardType["unknown"].Total != 3 || a.ByShardName["unknown"].Total != 3 || a.TotalProject.Total != 16 {
				t.Fatalf("outside usage mixed with shard work: %+v", a)
			}
			if a.BySession["caller-session"].Total != 16 || a.BySession["manager-session"].Total != 0 {
				t.Fatal("caller session identity replaced")
			}
			if name, typ := usage.ShardFromContext(parent); name != "parent-coder" || typ != "coder" {
				t.Fatal("spawn mutated parent attribution")
			}
		})
	}
}
