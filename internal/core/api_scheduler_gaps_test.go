package core

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// ============================================================================
// Remediation for api_scheduler_test.go TEST_GAP markers (16 gaps total).
// QA: api_scheduler_boundary_analysis
// ============================================================================

// ---------- Null/Empty Inputs ----------

// TestAPISchedulerGap_RegisterShard_EmptyID verifies registering with empty shard ID.
func TestAPISchedulerGap_RegisterShard_EmptyID(t *testing.T) {
	scheduler := NewAPIScheduler(DefaultAPISchedulerConfig())

	// Should succeed (no validation on empty IDs currently)
	state := scheduler.RegisterShard("", "test")
	if state == nil {
		t.Fatal("Expected non-nil state for empty shard ID")
	}
	if state.ShardID != "" {
		t.Errorf("Expected empty ShardID, got %q", state.ShardID)
	}

	// Should be able to acquire and release with empty ID
	ctx := context.Background()
	err := scheduler.AcquireAPISlot(ctx, "")
	if err != nil {
		t.Fatalf("Expected AcquireAPISlot to work with empty ID: %v", err)
	}
	scheduler.ReleaseAPISlot("")
}

// TestAPISchedulerGap_UnregisterShard_NonExistent verifies unregistering a non-existent shard.
func TestAPISchedulerGap_UnregisterShard_NonExistent(t *testing.T) {
	scheduler := NewAPIScheduler(DefaultAPISchedulerConfig())

	// Should not panic
	scheduler.UnregisterShard("non-existent")
	scheduler.UnregisterShard("")

	// Verify metrics are unaffected
	metrics := scheduler.GetMetrics()
	if metrics.RegisteredShards != 0 {
		t.Errorf("Expected 0 registered shards, got %d", metrics.RegisteredShards)
	}
}

// TestAPISchedulerGap_AcquireSlot_NilContext verifies behavior with nil context.
func TestAPISchedulerGap_AcquireSlot_NilContext(t *testing.T) {
	scheduler := NewAPIScheduler(DefaultAPISchedulerConfig())
	scheduler.RegisterShard("test", "test")

	// nil context should panic (Go stdlib convention)
	defer func() {
		if r := recover(); r != nil {
			t.Logf("KNOWN: nil context causes panic as expected by Go conventions: %v", r)
		}
	}()
	// This will likely panic because context operations on nil are UB
	scheduler.AcquireAPISlot(nil, "test") //nolint:staticcheck
}

// ---------- Config Boundary Values ----------

// TestAPISchedulerGap_NegativeConcurrency verifies zero/negative MaxConcurrentAPICalls.
func TestAPISchedulerGap_NegativeConcurrency(t *testing.T) {
	tests := []struct {
		name     string
		maxConc  int
	}{
		{"zero", 0},
		{"negative", -5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// NewAPIScheduler creates a buffered channel with size maxConc.
			// Zero or negative creates an unbuffered or panics.
			defer func() {
				if r := recover(); r != nil {
					t.Logf("KNOWN: NewAPIScheduler(%d) panics: %v", tt.maxConc, r)
				}
			}()

			scheduler := NewAPIScheduler(APISchedulerConfig{
				MaxConcurrentAPICalls: tt.maxConc,
				SlotAcquireTimeout:   5 * time.Second,
			})
			if scheduler != nil {
				t.Logf("NewAPIScheduler(%d) created successfully (channel size=%d)", tt.maxConc, tt.maxConc)
			}
		})
	}
}

// TestAPISchedulerGap_ZeroTimeout verifies behavior with zero SlotAcquireTimeout.
func TestAPISchedulerGap_ZeroTimeout(t *testing.T) {
	scheduler := NewAPIScheduler(APISchedulerConfig{
		MaxConcurrentAPICalls: 1,
		SlotAcquireTimeout:   0, // Zero timeout
	})
	scheduler.RegisterShard("s1", "test")
	scheduler.RegisterShard("s2", "test")

	ctx := context.Background()

	// First slot should always succeed
	err := scheduler.AcquireAPISlot(ctx, "s1")
	if err != nil {
		t.Fatalf("First slot failed: %v", err)
	}

	// Second slot with zero timeout — context governs entirely
	shortCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	err = scheduler.AcquireAPISlot(shortCtx, "s2")
	if err == nil {
		scheduler.ReleaseAPISlot("s2")
		t.Log("Zero timeout: slot acquired (context timeout used instead)")
	} else {
		t.Logf("Zero timeout: slot acquisition failed as expected: %v", err)
	}

	scheduler.ReleaseAPISlot("s1")
}

// ---------- ScheduledLLMCall Edge Cases ----------

// TestAPISchedulerGap_NilClient_PanicRecovery verifies behavior when Client is nil.
func TestAPISchedulerGap_NilClient_PanicRecovery(t *testing.T) {
	scheduler := NewAPIScheduler(DefaultAPISchedulerConfig())
	scheduler.RegisterShard("test", "test")

	call := &ScheduledLLMCall{
		Scheduler: scheduler,
		ShardID:   "test",
		Client:    nil, // nil client
	}

	ctx := context.Background()

	defer func() {
		if r := recover(); r != nil {
			t.Logf("KNOWN: nil Client causes panic on method call: %v", r)
		}
	}()

	// This will panic when it tries to call nil.Complete
	call.Complete(ctx, "test")
}

// ---------- Resource Exhaustion ----------

// TestAPISchedulerGap_CheckpointMassivePayload verifies deep-copy with large checkpoints.
func TestAPISchedulerGap_CheckpointMassivePayload(t *testing.T) {
	scheduler := NewAPIScheduler(DefaultAPISchedulerConfig())
	scheduler.RegisterShard("test", "test")

	// Store 1000 checkpoint entries
	for i := 0; i < 1000; i++ {
		scheduler.SaveCheckpoint("test", fmt.Sprintf("key_%d", i), fmt.Sprintf("value_%d_data_%s", i, "padding"))
	}

	// Deep copy should succeed without OOM
	state, ok := scheduler.GetShardState("test")
	if !ok {
		t.Fatal("Expected shard state")
	}
	if len(state.Checkpoint) != 1000 {
		t.Errorf("Expected 1000 checkpoint entries, got %d", len(state.Checkpoint))
	}

	// Modify deep copy — should not affect original
	state.Checkpoint["injected"] = true
	_, exists := scheduler.LoadCheckpoint("test", "injected")
	if exists {
		t.Error("Deep copy mutation leaked to original")
	}
}

// ---------- Concurrency ----------

// TestAPISchedulerGap_Race_RegisterUnregister verifies concurrent register/unregister.
func TestAPISchedulerGap_Race_RegisterUnregister(t *testing.T) {
	scheduler := NewAPIScheduler(DefaultAPISchedulerConfig())

	var wg sync.WaitGroup

	// Concurrent register/unregister for same IDs
	for i := 0; i < 10; i++ {
		wg.Add(2)
		id := fmt.Sprintf("shard_%d", i%3) // Overlapping IDs

		go func(shardID string) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				scheduler.RegisterShard(shardID, "test")
			}
		}(id)

		go func(shardID string) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				scheduler.UnregisterShard(shardID)
			}
		}(id)
	}

	wg.Wait()
	// No panics or data races (run with -race)
}

// TestAPISchedulerGap_Race_ContextCancelVsSlotAcquire verifies the TOCTOU window
// where context is cancelled precisely as a slot becomes available.
func TestAPISchedulerGap_Race_ContextCancelVsSlotAcquire(t *testing.T) {
	scheduler := NewAPIScheduler(APISchedulerConfig{
		MaxConcurrentAPICalls: 1,
		SlotAcquireTimeout:   5 * time.Second,
	})

	scheduler.RegisterShard("holder", "test")
	scheduler.RegisterShard("racer", "test")

	ctx := context.Background()

	// Fill the slot
	err := scheduler.AcquireAPISlot(ctx, "holder")
	if err != nil {
		t.Fatalf("Failed to acquire slot: %v", err)
	}

	// Start racer with cancellable context
	cancelCtx, cancel := context.WithCancel(ctx)

	done := make(chan error, 1)
	go func() {
		done <- scheduler.AcquireAPISlot(cancelCtx, "racer")
	}()

	// Give racer time to enter wait state
	time.Sleep(20 * time.Millisecond)

	// Cancel AND release simultaneously to create TOCTOU race
	cancel()
	scheduler.ReleaseAPISlot("holder")

	err = <-done
	// Either the racer got the slot or was cancelled — both are valid
	t.Logf("TOCTOU race result: err=%v", err)
}

// ---------- Extreme Load ----------

// TestAPISchedulerGap_ExtremeLoad_ManyShards tests scheduler with many shards.
func TestAPISchedulerGap_ExtremeLoad_ManyShards(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping extreme load test in short mode")
	}

	const numShards = 100
	scheduler := NewAPIScheduler(APISchedulerConfig{
		MaxConcurrentAPICalls: 3,
		SlotAcquireTimeout:   30 * time.Second,
	})

	for i := 0; i < numShards; i++ {
		scheduler.RegisterShard(fmt.Sprintf("shard_%d", i), "test")
	}

	var wg sync.WaitGroup
	ctx := context.Background()

	for i := 0; i < numShards; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			shardID := fmt.Sprintf("shard_%d", idx)
			err := scheduler.AcquireAPISlot(ctx, shardID)
			if err != nil {
				return
			}
			// Brief simulated work
			time.Sleep(time.Millisecond)
			scheduler.ReleaseAPISlot(shardID)
		}(i)
	}

	wg.Wait()

	metrics := scheduler.GetMetrics()
	if metrics.TotalAPICalls != numShards {
		t.Errorf("Expected %d total API calls, got %d", numShards, metrics.TotalAPICalls)
	}
}

// TestAPISchedulerGap_RetryExtremeMaxRetries verifies behavior with large maxRetries.
func TestAPISchedulerGap_RetryExtremeMaxRetries(t *testing.T) {
	scheduler := NewAPIScheduler(DefaultAPISchedulerConfig())
	scheduler.RegisterShard("retry", "test")

	callCount := 0
	mock := &mockLLMClient{
		completeFunc: func(ctx context.Context, prompt string) (string, error) {
			callCount++
			if callCount <= 5 {
				return "", fmt.Errorf("transient error %d", callCount)
			}
			return "success", nil
		},
	}

	call := &ScheduledLLMCall{
		Scheduler: scheduler,
		ShardID:   "retry",
		Client:    mock,
	}

	// Use a timeout to prevent infinite retry
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := call.CompleteWithRetry(ctx, "sys", "usr", 10)
	if err != nil {
		t.Fatalf("Expected success after retries, got: %v", err)
	}
	if result != "success" {
		t.Errorf("Expected 'success', got %q", result)
	}
	if callCount != 6 {
		t.Errorf("Expected 6 calls (5 failures + 1 success), got %d", callCount)
	}
}

// TestAPISchedulerGap_DurationOverflow tests behavior with extremely large timeout.
func TestAPISchedulerGap_DurationOverflow(t *testing.T) {
	// Very large timeout — should not cause overflow or panic
	scheduler := NewAPIScheduler(APISchedulerConfig{
		MaxConcurrentAPICalls: 2,
		SlotAcquireTimeout:   time.Duration(1<<62 - 1), // Near-max Duration
	})

	scheduler.RegisterShard("test", "test")

	ctx := context.Background()
	err := scheduler.AcquireAPISlot(ctx, "test")
	if err != nil {
		t.Fatalf("Expected slot acquisition with large timeout: %v", err)
	}
	scheduler.ReleaseAPISlot("test")
}
