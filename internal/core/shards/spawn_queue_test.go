package shards

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"codenerd/internal/types"
)

type stubLimits struct {
	slots int
}

func (s stubLimits) CheckShardLimit(activeCount int) error { return nil }
func (s stubLimits) CheckMemory() error                    { return nil }
func (s stubLimits) GetAvailableShardSlots(activeCount int) int {
	return s.slots
}

func TestSpawnQueueStartStop(t *testing.T) {
	cfg := SpawnQueueConfig{
		WorkerCount:  1,
		DrainTimeout: 50 * time.Millisecond,
	}
	sq := NewSpawnQueue(nil, nil, cfg)
	if sq.IsRunning() {
		t.Fatalf("expected queue to be stopped initially")
	}
	if err := sq.Start(); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	if !sq.IsRunning() {
		t.Fatalf("expected queue to be running")
	}
	if err := sq.Stop(); err != nil {
		t.Fatalf("stop failed: %v", err)
	}
	if sq.IsRunning() {
		t.Fatalf("expected queue to be stopped")
	}
}

func TestSpawnQueueSubmitWhenStopped(t *testing.T) {
	sq := NewSpawnQueue(nil, nil, SpawnQueueConfig{})
	_, err := sq.Submit(context.Background(), "coder", "task", nil, types.PriorityNormal, time.Time{}, false)
	if !errors.Is(err, ErrQueueStopped) {
		t.Fatalf("expected ErrQueueStopped, got %v", err)
	}
}

func TestSpawnQueueCanAccept(t *testing.T) {
	cfg := SpawnQueueConfig{
		MaxQueueSize:        2,
		MaxQueuePerPriority: 2,
		HighWaterMark:       0.5,
	}
	sq := NewSpawnQueue(nil, nil, cfg)

	sq.queues[types.PriorityNormal] <- &spawnRequestWrapper{ID: "a"}
	sq.queues[types.PriorityHigh] <- &spawnRequestWrapper{ID: "b"}

	ok, reason := sq.CanAccept(types.PriorityNormal)
	if ok || !strings.Contains(reason, "total queue capacity") {
		t.Fatalf("expected queue full rejection, got ok=%v reason=%q", ok, reason)
	}
}

func TestSpawnQueueBackpressureHighWaterMark(t *testing.T) {
	cfg := SpawnQueueConfig{
		MaxQueueSize:        4,
		MaxQueuePerPriority: 4,
		HighWaterMark:       0.5,
	}
	sq := NewSpawnQueue(nil, stubLimits{slots: 0}, cfg)

	for i := 0; i < 3; i++ {
		sq.queues[types.PriorityNormal] <- &spawnRequestWrapper{ID: "req"}
	}

	status := sq.GetBackpressureStatus()
	if status.Accepting {
		t.Fatalf("expected backpressure to reject when slots are zero at high water mark")
	}
	if !strings.Contains(status.Reason, "no slots available") {
		t.Fatalf("expected backpressure reason, got %q", status.Reason)
	}
}
