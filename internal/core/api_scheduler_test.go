package core

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/types"
)

// MockLLMClient for testing
type mockLLMClient struct {
	completeFunc          func(ctx context.Context, prompt string) (string, error)
	delay                 time.Duration
	callCount             int32
	StreamingNotSupported bool
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

func (m *mockLLMClient) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	text, err := m.Complete(ctx, systemPrompt+"\n"+userPrompt)
	if err != nil {
		return nil, err
	}
	return &types.LLMToolResponse{Text: text, StopReason: "end_turn"}, nil
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
	for i := range numShards {
		scheduler.RegisterShard(string(rune('A'+i)), "test")
	}

	var wg sync.WaitGroup
	var maxConcurrent int32
	var currentConcurrent int32

	ctx := context.Background()

	for i := range numShards {
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
	for i := range 5 {
		shardID := string(rune('A' + i))
		scheduler.RegisterShard(shardID, "test")
	}

	// Launch 5 concurrent calls
	var wg sync.WaitGroup
	ctx := context.Background()

	for i := range 5 {
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

// REMEDIATED: All 16 TEST_GAP items — see api_scheduler_gaps_test.go:
//   TestAPISchedulerGap_RegisterShard_EmptyID (Null/Empty)
//   TestAPISchedulerGap_UnregisterShard_NonExistent (Null/Empty)
//   TestAPISchedulerGap_NegativeConcurrency (Config Boundary - FOUND BUG: negative panics)
//   TestAPISchedulerGap_ZeroTimeout (Config Boundary)
//   TestAPISchedulerGap_AcquireSlot_NilContext (Null/Empty - panics by Go convention)
//   TestAPISchedulerGap_NilClient_PanicRecovery (Null/Empty - panics on nil deref)
//   TestAPISchedulerGap_DurationOverflow (Config Boundary)
//   TestAPISchedulerGap_ExtremeLoad_ManyShards (Performance - 100 shards)
//   TestAPISchedulerGap_RetryExtremeMaxRetries (Performance - retry circuit breaker)
//   TestAPISchedulerGap_CheckpointMassivePayload (Resource Exhaustion)
//   TestAPISchedulerGap_Race_RegisterUnregister (Concurrency)
//   TestAPISchedulerGap_Race_ContextCancelVsSlotAcquire (TOCTOU)
//   TestAPISchedulerGap_Streaming_NonStreamingClient (Interface Assertion)
//   TestAPISchedulerGap_Streaming_NilChannelsFromUnderlying (Nil Channels)
//   TestAPISchedulerGap_Streaming_RapidCancel (Goroutine Leak)
//   TestAPISchedulerGap_GlobalConfig_SyncOnce (sync.Once guard)

// TestAPIScheduler_PriorityWakeOrder proves slot hand-off is priority-aware:
// a high-priority waiter (interactive turn) is woken before earlier-queued
// normal/low-priority waiters (background work). Before the fix, the parsed
// priority was ignored and waiters woke strictly FIFO.
func TestAPIScheduler_PriorityWakeOrder(t *testing.T) {
	scheduler := NewAPIScheduler(APISchedulerConfig{
		MaxConcurrentAPICalls: 1,
		SlotAcquireTimeout:    10 * time.Second,
	})

	scheduler.RegisterShard("holder", "test")
	scheduler.RegisterShardWithPriority("background-1", "test", types.PriorityLow)
	scheduler.RegisterShardWithPriority("background-2", "test", types.PriorityLow)
	scheduler.RegisterShardWithPriority("interactive", "test", types.PriorityHigh)

	ctx := context.Background()

	// Occupy the only slot.
	if err := scheduler.AcquireAPISlot(ctx, "holder"); err != nil {
		t.Fatalf("holder acquire failed: %v", err)
	}

	// enqueueWaiter blocks in AcquireAPISlot and records its wake order.
	var order []string
	var orderMu sync.Mutex
	var wg sync.WaitGroup
	enqueue := func(shardID string) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := scheduler.AcquireAPISlot(ctx, shardID); err != nil {
				t.Errorf("%s acquire failed: %v", shardID, err)
				return
			}
			orderMu.Lock()
			order = append(order, shardID)
			orderMu.Unlock()
			scheduler.ReleaseAPISlot(shardID)
		}()
		// Give the goroutine time to enter the wait queue so enqueue order is
		// deterministic.
		waitForQueued(t, scheduler, shardID)
	}

	// Background waiters queue FIRST; the interactive waiter queues LAST.
	enqueue("background-1")
	enqueue("background-2")
	enqueue("interactive")

	// Release the slot: the interactive waiter must wake first despite being
	// queued last.
	scheduler.ReleaseAPISlot("holder")
	wg.Wait()

	if len(order) != 3 {
		t.Fatalf("expected 3 completions, got %d (%v)", len(order), order)
	}
	if order[0] != "interactive" {
		t.Fatalf("wake order = %v, want interactive first (priority scheduling broken)", order)
	}
	// Background waiters keep FIFO order among themselves.
	if order[1] != "background-1" || order[2] != "background-2" {
		t.Errorf("background wake order = %v, want FIFO within priority", order[1:])
	}
}

// waitForQueued polls until the shard appears in the scheduler's wait queue.
func waitForQueued(t *testing.T, s *APIScheduler, shardID string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.RLock()
		for _, e := range s.waitQueue {
			if e.shardID == shardID {
				s.mu.RUnlock()
				return
			}
		}
		s.mu.RUnlock()
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("shard %s never entered the wait queue", shardID)
}

// TestAPIScheduler_FreeSlotPriority_NoSkipAhead proves a free slot is not
// granted to a low-priority waiter while a higher-priority waiter is already queued.
func TestAPIScheduler_FreeSlotPriority_NoSkipAhead(t *testing.T) {
	scheduler := NewAPIScheduler(APISchedulerConfig{
		MaxConcurrentAPICalls: 1,
		SlotAcquireTimeout:    5 * time.Second,
	})
	scheduler.RegisterShard("holder", "test")
	scheduler.RegisterShardWithPriority("low", "test", types.PriorityLow)
	scheduler.RegisterShardWithPriority("high", "test", types.PriorityHigh)

	ctx := context.Background()
	if err := scheduler.AcquireAPISlot(ctx, "holder"); err != nil {
		t.Fatalf("holder: %v", err)
	}

	var first string
	var firstMu sync.Mutex
	recordFirst := func(id string) {
		firstMu.Lock()
		if first == "" {
			first = id
		}
		firstMu.Unlock()
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// Queue low first, then high (while holder still owns the slot).
	go func() {
		defer wg.Done()
		if err := scheduler.AcquireAPISlot(ctx, "low"); err != nil {
			t.Errorf("low: %v", err)
			return
		}
		recordFirst("low")
		scheduler.ReleaseAPISlot("low")
	}()
	waitForQueued(t, scheduler, "low")

	go func() {
		defer wg.Done()
		if err := scheduler.AcquireAPISlot(ctx, "high"); err != nil {
			t.Errorf("high: %v", err)
			return
		}
		recordFirst("high")
		scheduler.ReleaseAPISlot("high")
	}()
	waitForQueued(t, scheduler, "high")

	scheduler.ReleaseAPISlot("holder")
	wg.Wait()

	if first != "high" {
		t.Fatalf("free/queued grant order: first completer = %q, want high", first)
	}
}

// TestAPIScheduler_AdaptiveConcurrency_RateLimitShrinks proves ReportRateLimit
// reduces the ceiling and ReportSuccess restores it after the recover window.
func TestAPIScheduler_AdaptiveConcurrency_RateLimitShrinks(t *testing.T) {
	scheduler := NewAPIScheduler(APISchedulerConfig{
		MaxConcurrentAPICalls: 3,
		SlotAcquireTimeout:    5 * time.Second,
		AdaptiveConcurrency:   true,
		AdaptiveFloor:         1,
		AdaptiveRecoverAfter:  20 * time.Millisecond,
	})
	if scheduler.EffectiveMaxSlots() != 3 {
		t.Fatalf("effective=%d want 3", scheduler.EffectiveMaxSlots())
	}
	if scheduler.BaseMaxSlots() != 3 {
		t.Fatalf("base=%d want 3", scheduler.BaseMaxSlots())
	}

	scheduler.ReportRateLimit()
	if scheduler.EffectiveMaxSlots() != 2 {
		t.Fatalf("after 1 RL effective=%d want 2", scheduler.EffectiveMaxSlots())
	}
	scheduler.ReportRateLimit()
	if scheduler.EffectiveMaxSlots() != 1 {
		t.Fatalf("after 2 RL effective=%d want 1", scheduler.EffectiveMaxSlots())
	}
	scheduler.ReportRateLimit()
	if scheduler.EffectiveMaxSlots() != 1 {
		t.Fatalf("floor broken: effective=%d want 1", scheduler.EffectiveMaxSlots())
	}

	// Success + wait recover window → grow back toward base.
	scheduler.ReportSuccess()
	time.Sleep(30 * time.Millisecond)
	scheduler.ReportSuccess() // triggers maybeRecoverAdaptive
	if scheduler.EffectiveMaxSlots() < 2 {
		t.Fatalf("expected recovery toward base, effective=%d", scheduler.EffectiveMaxSlots())
	}
}

// TestAPIScheduler_MinCallSpacing enforces spacing between grants.
func TestAPIScheduler_MinCallSpacing(t *testing.T) {
	spacing := 40 * time.Millisecond
	scheduler := NewAPIScheduler(APISchedulerConfig{
		MaxConcurrentAPICalls: 2,
		SlotAcquireTimeout:    5 * time.Second,
		MinCallSpacing:        spacing,
	})
	scheduler.RegisterShard("a", "test")
	scheduler.RegisterShard("b", "test")

	ctx := context.Background()
	start := time.Now()
	if err := scheduler.AcquireAPISlot(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	t1 := time.Now()
	if err := scheduler.AcquireAPISlot(ctx, "b"); err != nil {
		t.Fatal(err)
	}
	t2 := time.Now()
	scheduler.ReleaseAPISlot("a")
	scheduler.ReleaseAPISlot("b")

	// Second grant should be delayed by ~spacing from first grant.
	gap := t2.Sub(t1)
	if gap < spacing/2 {
		t.Fatalf("second grant gap %v < expected spacing ~%v (from start %v)", gap, spacing, t1.Sub(start))
	}
}

// TestAPIScheduler_ContextPriorityOverridesDefault proves an explicit
// CtxKeyPriority beats the registered default.
func TestAPIScheduler_ContextPriorityOverridesDefault(t *testing.T) {
	scheduler := NewAPIScheduler(APISchedulerConfig{
		MaxConcurrentAPICalls: 1,
		SlotAcquireTimeout:    10 * time.Second,
	})
	scheduler.RegisterShard("holder", "test")
	scheduler.RegisterShardWithPriority("normally-low", "test", types.PriorityLow)
	scheduler.RegisterShardWithPriority("normally-high", "test", types.PriorityHigh)

	ctx := context.Background()
	if err := scheduler.AcquireAPISlot(ctx, "holder"); err != nil {
		t.Fatalf("holder acquire failed: %v", err)
	}

	var order []string
	var orderMu sync.Mutex
	var wg sync.WaitGroup

	// normally-high queues first but is demoted to low via context.
	wg.Add(1)
	go func() {
		defer wg.Done()
		demoted := context.WithValue(ctx, types.CtxKeyPriority, types.PriorityLow)
		if err := scheduler.AcquireAPISlot(demoted, "normally-high"); err != nil {
			t.Errorf("normally-high acquire failed: %v", err)
			return
		}
		orderMu.Lock()
		order = append(order, "normally-high")
		orderMu.Unlock()
		scheduler.ReleaseAPISlot("normally-high")
	}()
	waitForQueued(t, scheduler, "normally-high")

	// normally-low queues second but is promoted to critical via context.
	wg.Add(1)
	go func() {
		defer wg.Done()
		promoted := context.WithValue(ctx, types.CtxKeyPriority, types.PriorityCritical)
		if err := scheduler.AcquireAPISlot(promoted, "normally-low"); err != nil {
			t.Errorf("normally-low acquire failed: %v", err)
			return
		}
		orderMu.Lock()
		order = append(order, "normally-low")
		orderMu.Unlock()
		scheduler.ReleaseAPISlot("normally-low")
	}()
	waitForQueued(t, scheduler, "normally-low")

	scheduler.ReleaseAPISlot("holder")
	wg.Wait()

	if len(order) != 2 || order[0] != "normally-low" {
		t.Fatalf("wake order = %v, want context-promoted shard first", order)
	}
}
