package core

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// MockLLMClient for testing
type mockLLMClient struct {
	completeFunc func(ctx context.Context, prompt string) (string, error)
	delay        time.Duration
	callCount    int32
}

func (m *mockLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	atomic.AddInt32(&m.callCount, 1)
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	if m.completeFunc != nil {
		return m.completeFunc(ctx, prompt)
	}
	return "mock response", nil
}

func (m *mockLLMClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return m.Complete(ctx, systemPrompt+"\n"+userPrompt)
}

// TestAPIScheduler_AcquireRelease tests basic slot acquisition and release
func TestAPIScheduler_AcquireRelease(t *testing.T) {
	config := APISchedulerConfig{
		MaxConcurrentAPICalls: 2,
		SlotAcquireTimeout:    5 * time.Second,
	}
	scheduler := NewAPIScheduler(config)

	// Register shards
	scheduler.RegisterShard("shard-1", "test")
	scheduler.RegisterShard("shard-2", "test")
	scheduler.RegisterShard("shard-3", "test")

	ctx := context.Background()

	// Acquire 2 slots - should succeed immediately
	err := scheduler.AcquireAPISlot(ctx, "shard-1")
	if err != nil {
		t.Fatalf("Failed to acquire slot 1: %v", err)
	}

	err = scheduler.AcquireAPISlot(ctx, "shard-2")
	if err != nil {
		t.Fatalf("Failed to acquire slot 2: %v", err)
	}

	// Try to acquire 3rd slot with short timeout - should timeout
	shortCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	err = scheduler.AcquireAPISlot(shortCtx, "shard-3")
	if err != context.DeadlineExceeded {
		t.Fatalf("Expected DeadlineExceeded, got: %v", err)
	}

	// Release slot 1
	scheduler.ReleaseAPISlot("shard-1")

	// Now slot 3 should be able to acquire
	err = scheduler.AcquireAPISlot(ctx, "shard-3")
	if err != nil {
		t.Fatalf("Failed to acquire slot 3 after release: %v", err)
	}

	// Cleanup
	scheduler.ReleaseAPISlot("shard-2")
	scheduler.ReleaseAPISlot("shard-3")
}

// TestAPIScheduler_ContextCancellation tests context cancellation while waiting
func TestAPIScheduler_ContextCancellation(t *testing.T) {
	config := APISchedulerConfig{
		MaxConcurrentAPICalls: 1,
		SlotAcquireTimeout:    5 * time.Second,
	}
	scheduler := NewAPIScheduler(config)

	scheduler.RegisterShard("shard-1", "test")
	scheduler.RegisterShard("shard-2", "test")

	ctx := context.Background()

	// Fill the only slot
	err := scheduler.AcquireAPISlot(ctx, "shard-1")
	if err != nil {
		t.Fatalf("Failed to acquire slot: %v", err)
	}

	// Try to acquire with cancellable context
	cancelCtx, cancel := context.WithCancel(ctx)

	done := make(chan error)
	go func() {
		done <- scheduler.AcquireAPISlot(cancelCtx, "shard-2")
	}()

	// Wait a bit then cancel
	time.Sleep(50 * time.Millisecond)
	cancel()

	// Should get context cancelled error
	err = <-done
	if err != context.Canceled {
		t.Fatalf("Expected context.Canceled, got: %v", err)
	}

	scheduler.ReleaseAPISlot("shard-1")
}

// TestAPIScheduler_Checkpoint tests checkpoint save/load with deep copy
func TestAPIScheduler_Checkpoint(t *testing.T) {
	scheduler := NewAPIScheduler(DefaultAPISchedulerConfig())
	scheduler.RegisterShard("shard-1", "test")

	// Save checkpoint
	scheduler.SaveCheckpoint("shard-1", "stage", 1)
	scheduler.SaveCheckpoint("shard-1", "data", map[string]string{"key": "value"})

	// Load checkpoint
	stage, ok := scheduler.LoadCheckpoint("shard-1", "stage")
	if !ok {
		t.Fatal("Failed to load stage checkpoint")
	}
	if stage.(int) != 1 {
		t.Fatalf("Expected stage=1, got %v", stage)
	}

	// Get shard state (should be deep copy)
	state, ok := scheduler.GetShardState("shard-1")
	if !ok {
		t.Fatal("Failed to get shard state")
	}

	// Modify the returned checkpoint map
	state.Checkpoint["modified"] = true

	// Verify original is not modified
	_, exists := scheduler.LoadCheckpoint("shard-1", "modified")
	if exists {
		t.Fatal("Deep copy failed - original checkpoint was modified")
	}
}

// TestAPIScheduler_ConcurrentAccess tests concurrent slot acquisition
func TestAPIScheduler_ConcurrentAccess(t *testing.T) {
	config := APISchedulerConfig{
		MaxConcurrentAPICalls: 3,
		SlotAcquireTimeout:    30 * time.Second,
	}
	scheduler := NewAPIScheduler(config)

	numShards := 10
	for i := 0; i < numShards; i++ {
		scheduler.RegisterShard(string(rune('A'+i)), "test")
	}

	var wg sync.WaitGroup
	var maxConcurrent int32
	var currentConcurrent int32

	ctx := context.Background()

	for i := 0; i < numShards; i++ {
		wg.Add(1)
		shardID := string(rune('A' + i))
		go func(id string) {
			defer wg.Done()

			err := scheduler.AcquireAPISlot(ctx, id)
			if err != nil {
				t.Errorf("Failed to acquire slot for %s: %v", id, err)
				return
			}

			// Track concurrent count
			current := atomic.AddInt32(&currentConcurrent, 1)
			for {
				old := atomic.LoadInt32(&maxConcurrent)
				if current <= old || atomic.CompareAndSwapInt32(&maxConcurrent, old, current) {
					break
				}
			}

			// Simulate work
			time.Sleep(10 * time.Millisecond)

			atomic.AddInt32(&currentConcurrent, -1)
			scheduler.ReleaseAPISlot(id)
		}(shardID)
	}

	wg.Wait()

	if maxConcurrent > 3 {
		t.Fatalf("Max concurrent exceeded limit: got %d, expected <=3", maxConcurrent)
	}

	metrics := scheduler.GetMetrics()
	if metrics.TotalAPICalls != int64(numShards) {
		t.Fatalf("Expected %d total API calls, got %d", numShards, metrics.TotalAPICalls)
	}
}

// TestAPIScheduler_WaitQueueCleanup tests wait queue cleanup on cancellation
func TestAPIScheduler_WaitQueueCleanup(t *testing.T) {
	config := APISchedulerConfig{
		MaxConcurrentAPICalls: 1,
		SlotAcquireTimeout:    5 * time.Second,
	}
	scheduler := NewAPIScheduler(config)

	scheduler.RegisterShard("holder", "test")
	scheduler.RegisterShard("waiter", "test")

	ctx := context.Background()

	// Fill the slot
	err := scheduler.AcquireAPISlot(ctx, "holder")
	if err != nil {
		t.Fatalf("Failed to acquire slot: %v", err)
	}

	// Start a waiter with cancellable context
	cancelCtx, cancel := context.WithCancel(ctx)

	done := make(chan struct{})
	go func() {
		scheduler.AcquireAPISlot(cancelCtx, "waiter")
		close(done)
	}()

	// Wait for waiter to be in queue
	time.Sleep(50 * time.Millisecond)

	// Verify waiter is in queue
	metrics := scheduler.GetMetrics()
	if metrics.WaitingShards != 1 {
		t.Fatalf("Expected 1 waiting shard, got %d", metrics.WaitingShards)
	}

	// Cancel the waiter
	cancel()
	<-done

	// Give time for cleanup
	time.Sleep(10 * time.Millisecond)

	// Verify waiter removed from queue
	metrics = scheduler.GetMetrics()
	if metrics.WaitingShards != 0 {
		t.Fatalf("Expected 0 waiting shards after cancel, got %d", metrics.WaitingShards)
	}

	scheduler.ReleaseAPISlot("holder")
}

// TestAPIScheduler_Metrics tests metrics accuracy
func TestAPIScheduler_Metrics(t *testing.T) {
	config := APISchedulerConfig{
		MaxConcurrentAPICalls: 2,
		SlotAcquireTimeout:    5 * time.Second,
	}
	scheduler := NewAPIScheduler(config)

	scheduler.RegisterShard("s1", "type-a")
	scheduler.RegisterShard("s2", "type-b")

	ctx := context.Background()

	// Make some API calls
	scheduler.AcquireAPISlot(ctx, "s1")
	scheduler.ReleaseAPISlot("s1")

	scheduler.AcquireAPISlot(ctx, "s2")
	scheduler.ReleaseAPISlot("s2")

	scheduler.AcquireAPISlot(ctx, "s1")
	scheduler.ReleaseAPISlot("s1")

	metrics := scheduler.GetMetrics()

	if metrics.TotalAPICalls != 3 {
		t.Fatalf("Expected 3 total API calls, got %d", metrics.TotalAPICalls)
	}

	if metrics.RegisteredShards != 2 {
		t.Fatalf("Expected 2 registered shards, got %d", metrics.RegisteredShards)
	}

	if metrics.MaxSlots != 2 {
		t.Fatalf("Expected max slots 2, got %d", metrics.MaxSlots)
	}

	// Cleanup
	scheduler.UnregisterShard("s1")
	scheduler.UnregisterShard("s2")

	metrics = scheduler.GetMetrics()
	if metrics.RegisteredShards != 0 {
		t.Fatalf("Expected 0 registered shards after unregister, got %d", metrics.RegisteredShards)
	}
}

// TestScheduledLLMCall_Complete tests the wrapper
func TestScheduledLLMCall_Complete(t *testing.T) {
	scheduler := NewAPIScheduler(APISchedulerConfig{
		MaxConcurrentAPICalls: 2,
		SlotAcquireTimeout:    5 * time.Second,
	})

	mock := &mockLLMClient{}
	scheduler.RegisterShard("test-shard", "test")

	call := &ScheduledLLMCall{
		Scheduler: scheduler,
		ShardID:   "test-shard",
		Client:    mock,
	}

	ctx := context.Background()
	result, err := call.Complete(ctx, "hello")
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	if result != "mock response" {
		t.Fatalf("Expected 'mock response', got '%s'", result)
	}

	if atomic.LoadInt32(&mock.callCount) != 1 {
		t.Fatalf("Expected 1 call to mock, got %d", mock.callCount)
	}

	// Verify slot was released
	metrics := scheduler.GetMetrics()
	if metrics.ActiveSlots != 0 {
		t.Fatalf("Expected 0 active slots after Complete, got %d", metrics.ActiveSlots)
	}
}

// TestScheduledLLMCall_RetryReleasesSlot tests slot release between retries
func TestScheduledLLMCall_RetryReleasesSlot(t *testing.T) {
	scheduler := NewAPIScheduler(APISchedulerConfig{
		MaxConcurrentAPICalls: 1, // Only 1 slot to verify release
		SlotAcquireTimeout:    5 * time.Second,
	})

	callCount := int32(0)
	mock := &mockLLMClient{
		completeFunc: func(ctx context.Context, prompt string) (string, error) {
			count := atomic.AddInt32(&callCount, 1)
			if count < 3 {
				return "", context.DeadlineExceeded // Fail first 2 attempts
			}
			return "success", nil
		},
	}

	scheduler.RegisterShard("retry-shard", "test")

	call := &ScheduledLLMCall{
		Scheduler: scheduler,
		ShardID:   "retry-shard",
		Client:    mock,
	}

	ctx := context.Background()
	result, err := call.CompleteWithRetry(ctx, "system", "user", 3)
	if err != nil {
		t.Fatalf("CompleteWithRetry failed: %v", err)
	}

	if result != "success" {
		t.Fatalf("Expected 'success', got '%s'", result)
	}

	if atomic.LoadInt32(&callCount) != 3 {
		t.Fatalf("Expected 3 calls (2 fails + 1 success), got %d", callCount)
	}

	// Verify API calls were tracked (each retry is a separate call)
	metrics := scheduler.GetMetrics()
	if metrics.TotalAPICalls != 3 {
		t.Fatalf("Expected 3 total API calls, got %d", metrics.TotalAPICalls)
	}
}

// TestNoDoubleLimiting verifies no double-limiting when semaphore disabled
func TestNoDoubleLimiting(t *testing.T) {
	scheduler := NewAPIScheduler(APISchedulerConfig{
		MaxConcurrentAPICalls: 5,
		SlotAcquireTimeout:    10 * time.Second,
	})

	// Track concurrent executions
	var maxConcurrent int32
	var currentConcurrent int32

	mock := &mockLLMClient{
		delay: 50 * time.Millisecond,
		completeFunc: func(ctx context.Context, prompt string) (string, error) {
			current := atomic.AddInt32(&currentConcurrent, 1)
			defer atomic.AddInt32(&currentConcurrent, -1)

			// Track max
			for {
				old := atomic.LoadInt32(&maxConcurrent)
				if current <= old || atomic.CompareAndSwapInt32(&maxConcurrent, old, current) {
					break
				}
			}

			time.Sleep(50 * time.Millisecond)
			return "ok", nil
		},
	}

	// Register 5 shards
	for i := 0; i < 5; i++ {
		shardID := string(rune('A' + i))
		scheduler.RegisterShard(shardID, "test")
	}

	// Launch 5 concurrent calls
	var wg sync.WaitGroup
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		wg.Add(1)
		shardID := string(rune('A' + i))
		go func(id string) {
			defer wg.Done()
			call := &ScheduledLLMCall{
				Scheduler: scheduler,
				ShardID:   id,
				Client:    mock,
			}
			call.Complete(ctx, "test")
		}(shardID)
	}

	wg.Wait()

	// Should have achieved 5 concurrent calls (no double-limiting)
	if maxConcurrent < 4 { // Allow some scheduling variance
		t.Fatalf("Expected near-5 concurrent calls, got %d (possible double-limiting)", maxConcurrent)
	}
}
