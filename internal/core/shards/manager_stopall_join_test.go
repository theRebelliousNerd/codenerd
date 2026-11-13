package shards

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/types"
)

// lingeringAgent runs until it is stopped or its context ends, then keeps
// working for a while before it returns -- the way a system shard writes its
// closing state after it observes the stop.
type lingeringAgent struct {
	onDemandFakeAgent
	stopped  chan struct{}
	finished *atomic.Bool
}

func (a *lingeringAgent) Stop() error {
	select {
	case <-a.stopped:
	default:
		close(a.stopped)
	}
	return nil
}

func (a *lingeringAgent) Execute(ctx context.Context, _ string) (string, error) {
	select {
	case <-a.stopped:
	case <-ctx.Done():
	}
	time.Sleep(100 * time.Millisecond)
	a.finished.Store(true)
	return "closing state written", nil
}

// StopAll returned once Stop had been called, while the shard's goroutine was
// still finishing: Cortex.Close then returned with a system shard about to
// write its transcript under .nerd, and a test's TempDir cleanup raced it
// ("directory not empty"). StopAll joins what the manager started.
func TestStopAllJoinsRunningShards(t *testing.T) {
	sm := NewShardManager()
	var finished atomic.Bool
	sm.RegisterShard("resident", func(id string, cfg types.ShardConfig) types.ShardAgent {
		return &lingeringAgent{onDemandFakeAgent: onDemandFakeAgent{id: id, config: cfg}, stopped: make(chan struct{}), finished: &finished}
	})
	sm.DefineProfile("resident", types.ShardConfig{Name: "resident", Type: types.ShardTypeSystem})

	id, err := sm.SpawnAsync(context.Background(), "resident", "serve")
	if err != nil {
		t.Fatalf("SpawnAsync: %v", err)
	}
	sm.StopAll()
	if !finished.Load() {
		t.Fatal("StopAll returned while the shard's goroutine was still running")
	}
	sm.mu.RLock()
	_, recorded := sm.results[id]
	running := len(sm.runs)
	sm.mu.RUnlock()
	if !recorded {
		t.Fatal("StopAll returned before the shard recorded its result")
	}
	if running != 0 {
		t.Fatalf("%d shard run(s) still registered after StopAll", running)
	}
}

// A shard whose agent ignores Stop still ends: StopAll cancels its context.
func TestStopAllCancelsAShardThatIgnoresStop(t *testing.T) {
	sm := NewShardManager()
	release := make(chan struct{})
	defer close(release)
	sm.RegisterShard("worker", func(id string, cfg types.ShardConfig) types.ShardAgent {
		return &onDemandFakeAgent{id: id, config: cfg, release: release}
	})
	sm.DefineProfile("worker", types.ShardConfig{Name: "worker", Type: types.ShardTypeEphemeral})
	if _, err := sm.SpawnAsync(context.Background(), "worker", "task"); err != nil {
		t.Fatalf("SpawnAsync: %v", err)
	}
	done := make(chan struct{})
	go func() { sm.StopAll(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("StopAll did not end a shard that ignores Stop")
	}
}
