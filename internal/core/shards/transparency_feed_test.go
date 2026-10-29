package shards

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"codenerd/internal/types"
)

// fakeTransparencyManager records the lifecycle calls ShardManager makes.
type fakeTransparencyManager struct {
	mu         sync.Mutex
	started    []string
	phases     []types.ShardPhase
	ended      map[string]bool
	operations []types.OperationRecord
}

func newFakeTransparencyManager() *fakeTransparencyManager {
	return &fakeTransparencyManager{ended: map[string]bool{}}
}

func (f *fakeTransparencyManager) IsEnabled() bool { return true }

func (f *fakeTransparencyManager) StartShard(shardID, _, _ string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.started = append(f.started, shardID)
}

func (f *fakeTransparencyManager) UpdateShardPhase(_ string, phase types.ShardPhase, _ string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.phases = append(f.phases, phase)
}

func (f *fakeTransparencyManager) EndShard(shardID string, failed bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.ended == nil {
		f.ended = map[string]bool{}
	}
	f.ended[shardID] = failed
}

func (f *fakeTransparencyManager) RecordOperation(rec types.OperationRecord) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.operations = append(f.operations, rec)
}

func (f *fakeTransparencyManager) snapshot() ([]string, []types.ShardPhase, map[string]bool, []types.OperationRecord) {
	f.mu.Lock()
	defer f.mu.Unlock()
	ended := make(map[string]bool, len(f.ended))
	for k, v := range f.ended {
		ended[k] = v
	}
	return append([]string(nil), f.started...),
		append([]types.ShardPhase(nil), f.phases...),
		ended,
		append([]types.OperationRecord(nil), f.operations...)
}

// TestSpawn_WhenTransparencyManagerAttached_ShouldReportShardLifecycle closes
// the split-brain the corpus flagged: ShardManager emitted Glass Box shard
// lines but never called StartShard/UpdateShardPhase/EndShard, so
// `/transparency` rendered "Active Operations" from a ShardObserver that
// nothing fed. If these calls are removed the list silently goes empty again,
// which no other test would notice.
func TestSpawn_WhenTransparencyManagerAttached_ShouldReportShardLifecycle(t *testing.T) {
	fake := newFakeTransparencyManager()
	sm := NewShardManager()
	// The monotonic clock granularity is around half a millisecond, so an instantly-returning delegator makes a correctly-measured duration read as zero.
	sm.SetTaskDelegator(&recordingDelegator{result: "work done", delay: 2 * time.Millisecond})
	sm.SetTransparencyManager(fake)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := sm.SpawnWithContext(ctx, "coder", "fix the bug", nil); err != nil {
		t.Fatalf("spawn failed: %v", err)
	}

	started, phases, ended, ops := fake.snapshot()
	if len(started) != 1 {
		t.Fatalf("expected exactly one StartShard call, got %d", len(started))
	}
	if len(phases) == 0 || phases[0] != types.PhaseExecuting {
		t.Fatalf("expected an Executing phase update, got %v", phases)
	}
	failed, ok := ended[started[0]]
	if !ok {
		t.Fatalf("expected EndShard for %s, got %v", started[0], ended)
	}
	if failed {
		t.Error("a successful shard must not be reported as failed")
	}
	if len(ops) != 1 {
		t.Fatalf("expected one operation summary, got %d", len(ops))
	}
	if ops[0].Outcome != "Success" || ops[0].Source != started[0] {
		t.Errorf("unexpected operation record: %+v", ops[0])
	}
	if ops[0].Duration <= 0 {
		t.Error("operation summary should carry a measured duration")
	}
}

func TestSpawn_WhenShardFails_ShouldReportFailedPhase(t *testing.T) {
	fake := newFakeTransparencyManager()
	sm := NewShardManager()
	sm.SetTaskDelegator(&recordingDelegator{err: errors.New("model refused")})
	sm.SetTransparencyManager(fake)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := sm.SpawnWithContext(ctx, "reviewer", "review it", nil); err == nil {
		t.Fatal("expected the failing delegation to surface as an error")
	}

	started, _, ended, ops := fake.snapshot()
	if len(started) != 1 {
		t.Fatalf("expected one StartShard call, got %d", len(started))
	}
	if failed, ok := ended[started[0]]; !ok || !failed {
		t.Fatalf("expected EndShard(failed=true) for %s, got %v", started[0], ended)
	}
	if len(ops) != 1 || ops[0].Outcome != "Failed" {
		t.Fatalf("expected a Failed operation summary, got %+v", ops)
	}
}

func TestSpawn_WhenNoTransparencyManager_ShouldStillSpawn(t *testing.T) {
	sm := NewShardManager()
	sm.SetTaskDelegator(&recordingDelegator{result: "work done"})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := sm.SpawnWithContext(ctx, "coder", "fix the bug", nil); err != nil {
		t.Fatalf("spawn must not depend on transparency being wired: %v", err)
	}
}
