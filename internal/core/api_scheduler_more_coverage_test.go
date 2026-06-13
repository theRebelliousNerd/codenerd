package core

import (
	"context"
	"testing"
	"time"

	"codenerd/internal/types"
)

func TestAPIScheduler_PhaseStrings(t *testing.T) {
	phases := []ShardPhase{
		PhaseInitializing,
		PhaseWaitingForSlot,
		PhaseExecutingAPI,
		PhaseProcessingResult,
		PhaseCompleted,
		PhaseFailed,
		ShardPhase(999),
	}
	expected := []string{
		"initializing",
		"waiting_for_slot",
		"executing_api",
		"processing_result",
		"completed",
		"failed",
		"unknown(999)",
	}
	for i, phase := range phases {
		if s := phase.String(); s != expected[i] {
			t.Errorf("Expected phase string %q, got %q", expected[i], s)
		}
	}
}

func TestAPIScheduler_AcquireUnregistered(t *testing.T) {
	s := NewAPIScheduler(DefaultAPISchedulerConfig())
	ctx := context.Background()
	err := s.AcquireAPISlot(ctx, "unregistered")
	if err == nil {
		t.Fatal("expected error when acquiring slot for unregistered shard")
	}
}

func TestAPIScheduler_PriorityContext(t *testing.T) {
	s := NewAPIScheduler(DefaultAPISchedulerConfig())
	s.RegisterShard("test_prio", "test")
	defer s.UnregisterShard("test_prio")

	ctx := context.WithValue(context.Background(), types.CtxKeyPriority, types.PriorityHigh)
	err := s.AcquireAPISlot(ctx, "test_prio")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s.ReleaseAPISlot("test_prio")
}

func TestAPIScheduler_StopInterrupted(t *testing.T) {
	s := NewAPIScheduler(APISchedulerConfig{
		MaxConcurrentAPICalls: 1,
		SlotAcquireTimeout:    time.Second,
	})
	s.RegisterShard("shard1", "test")
	s.RegisterShard("shard2", "test")
	defer s.UnregisterShard("shard1")
	defer s.UnregisterShard("shard2")

	// Acquire slot 1
	ctx := context.Background()
	if err := s.AcquireAPISlot(ctx, "shard1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Try to acquire slot 2 (blocks)
	errChan := make(chan error, 1)
	go func() {
		errChan <- s.AcquireAPISlot(ctx, "shard2")
	}()

	time.Sleep(50 * time.Millisecond) // let it start waiting
	s.Stop()                          // interrupt wait

	err := <-errChan
	if err == nil || err.Error() != "scheduler stopped" {
		t.Errorf("expected 'scheduler stopped' error, got: %v", err)
	}

	// Cleanup slot1
	s.ReleaseAPISlot("shard1")
}

func TestAPIScheduler_ReleaseWithoutAcquire(t *testing.T) {
	s := NewAPIScheduler(DefaultAPISchedulerConfig())
	// Should not panic, should log error
	s.ReleaseAPISlot("nonexistent")
}

func TestAPIScheduler_GetStateNonexistent(t *testing.T) {
	s := NewAPIScheduler(DefaultAPISchedulerConfig())
	_, ok := s.GetShardState("nonexistent")
	if ok {
		t.Error("expected false for nonexistent shard state")
	}
}

func TestAPIScheduler_MetricsString(t *testing.T) {
	s := NewAPIScheduler(DefaultAPISchedulerConfig())
	s.RegisterShard("test", "test")
	ctx := context.Background()
	_ = s.AcquireAPISlot(ctx, "test")
	s.ReleaseAPISlot("test")

	metrics := s.GetMetrics()
	mStr := metrics.String()
	if mStr == "" {
		t.Error("expected non-empty metrics string")
	}

	// Test zero API calls metrics
	emptyMetrics := APISchedulerMetrics{
		TotalAPICalls: 0,
	}
	if emptyMetrics.String() == "" {
		t.Error("expected non-empty zero calls metrics string")
	}
}
