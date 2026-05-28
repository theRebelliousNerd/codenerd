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

// TestAPISchedulerGap_RegisterShard_EmptyID verifies registering with empty shard ID is rejected.
func TestAPISchedulerGap_RegisterShard_EmptyID(t *testing.T) {
	scheduler := NewAPIScheduler(DefaultAPISchedulerConfig())

	state := scheduler.RegisterShard("", "test")
	if state != nil {
		t.Fatal("Expected nil state when registering empty shard ID")
	}

	// Should fail to acquire
	ctx := context.Background()
	err := scheduler.AcquireAPISlot(ctx, "")
	if err == nil {
		t.Fatal("Expected AcquireAPISlot to fail with unregistered empty ID")
	}
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

	err := scheduler.AcquireAPISlot(nil, "test") //nolint:staticcheck
	if err == nil {
		t.Fatal("Expected AcquireAPISlot to fail with nil context")
	}
}

// ---------- Config Boundary Values ----------

// TestAPISchedulerGap_NegativeConcurrency verifies zero/negative MaxConcurrentAPICalls are sanitized.
func TestAPISchedulerGap_NegativeConcurrency(t *testing.T) {
	tests := []struct {
		name    string
		maxConc int
	}{
		{"zero", 0},
		{"negative", -5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheduler := NewAPIScheduler(APISchedulerConfig{
				MaxConcurrentAPICalls: tt.maxConc,
				SlotAcquireTimeout:    5 * time.Second,
			})
			if scheduler == nil {
				t.Fatal("Expected scheduler to be non-nil")
			}
			metrics := scheduler.GetMetrics()
			if metrics.MaxSlots <= 0 {
				t.Errorf("Expected sanitized MaxSlots > 0, got %d", metrics.MaxSlots)
			}
		})
	}
}

// TestAPISchedulerGap_ZeroTimeout verifies behavior with zero SlotAcquireTimeout.
func TestAPISchedulerGap_ZeroTimeout(t *testing.T) {
	scheduler := NewAPIScheduler(APISchedulerConfig{
		MaxConcurrentAPICalls: 1,
		SlotAcquireTimeout:    0, // Zero timeout
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
	_, err := call.Complete(ctx, "test")
	if err == nil {
		t.Fatal("Expected Complete to fail when Client is nil")
	}
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
		SlotAcquireTimeout:    5 * time.Second,
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
		SlotAcquireTimeout:    30 * time.Second,
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
		SlotAcquireTimeout:    time.Duration(1<<62 - 1), // Near-max Duration
	})

	scheduler.RegisterShard("test", "test")

	ctx := context.Background()
	err := scheduler.AcquireAPISlot(ctx, "test")
	if err != nil {
		t.Fatalf("Expected slot acquisition with large timeout: %v", err)
	}
	scheduler.ReleaseAPISlot("test")
}

// ---------- Streaming Gaps ----------

// mockStreamingClient implements llmStreamingChannels for streaming tests.
type mockStreamingClient struct {
	mockLLMClient
	streamFunc func(ctx context.Context, sys, usr string, think bool) (<-chan string, <-chan error)
}

func (m *mockStreamingClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error) {
	if m.streamFunc != nil {
		return m.streamFunc(ctx, systemPrompt, userPrompt, enableThinking)
	}
	// Default: return immediately-closed channels
	content := make(chan string)
	errCh := make(chan error)
	close(content)
	close(errCh)
	return content, errCh
}

// TestAPISchedulerGap_Streaming_NonStreamingClient tests streaming on a client that doesn't support it.
func TestAPISchedulerGap_Streaming_NonStreamingClient(t *testing.T) {
	scheduler := NewAPIScheduler(DefaultAPISchedulerConfig())
	scheduler.RegisterShard("test", "test")

	// mockLLMClient does NOT implement streaming
	call := &ScheduledLLMCall{
		Scheduler: scheduler,
		ShardID:   "test",
		Client:    &mockLLMClient{StreamingNotSupported: true},
	}

	ctx := context.Background()
	contentCh, errCh := call.CompleteWithStreaming(ctx, "sys", "usr", false)

	// Content channel should be closed immediately
	_, ok := <-contentCh
	if ok {
		t.Error("Expected content channel to be closed for non-streaming client")
	}

	// Error channel should contain ErrStreamingNotSupported
	err, ok := <-errCh
	if !ok {
		t.Fatal("Expected error channel to have an error before closing")
	}
	if err != ErrStreamingNotSupported {
		t.Errorf("Expected ErrStreamingNotSupported, got: %v", err)
	}
}

// TestAPISchedulerGap_Streaming_NilChannelsFromUnderlying tests when the underlying
// streamer returns nil channels (edge case).
// FIX VERIFIED: The forwarding goroutine now treats nil channels as immediately closed,
// so it exits cleanly, closes output channels, and releases the API slot.
func TestAPISchedulerGap_Streaming_NilChannelsFromUnderlying(t *testing.T) {
	scheduler := NewAPIScheduler(DefaultAPISchedulerConfig())
	scheduler.RegisterShard("test", "test")

	mock := &mockStreamingClient{
		streamFunc: func(ctx context.Context, sys, usr string, think bool) (<-chan string, <-chan error) {
			return nil, nil
		},
	}

	call := &ScheduledLLMCall{
		Scheduler: scheduler,
		ShardID:   "test",
		Client:    mock,
	}

	ctx := context.Background()
	contentCh, errCh := call.CompleteWithStreaming(ctx, "sys", "usr", false)

	// With the fix, nil channels are treated as immediately closed.
	// The forwarding goroutine should exit cleanly and close output channels.

	// Content channel should be closed (drain it)
	for range contentCh {
		t.Error("Expected no content from nil underlying channel")
	}

	// Error channel should be closed (drain it)
	for range errCh {
		// No error expected — nil channels just mean empty stream
	}

	// Verify slot was released by acquiring it again
	slotCtx, slotCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer slotCancel()
	err := scheduler.AcquireAPISlot(slotCtx, "test")
	if err != nil {
		t.Fatalf("Slot was leaked after nil-channel streaming: %v", err)
	}
	scheduler.ReleaseAPISlot("test")
}

// TestAPISchedulerGap_Streaming_RapidCancel tests that the forwarding goroutine
// doesn't leak when context is cancelled while the underlying stream is active.
func TestAPISchedulerGap_Streaming_RapidCancel(t *testing.T) {
	scheduler := NewAPIScheduler(DefaultAPISchedulerConfig())
	scheduler.RegisterShard("test", "test")

	// Underlying stream that sends data slowly
	mock := &mockStreamingClient{
		streamFunc: func(ctx context.Context, sys, usr string, think bool) (<-chan string, <-chan error) {
			content := make(chan string)
			errCh := make(chan error)
			go func() {
				defer close(content)
				defer close(errCh)
				for i := 0; i < 100; i++ {
					select {
					case content <- fmt.Sprintf("chunk_%d", i):
						time.Sleep(10 * time.Millisecond)
					case <-ctx.Done():
						return
					}
				}
			}()
			return content, errCh
		},
	}

	call := &ScheduledLLMCall{
		Scheduler: scheduler,
		ShardID:   "test",
		Client:    mock,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	contentCh, errCh := call.CompleteWithStreaming(ctx, "sys", "usr", false)

	// Read a few chunks then cancel rapidly
	chunkCount := 0
	for chunk := range contentCh {
		chunkCount++
		_ = chunk
		if chunkCount >= 3 {
			cancel()
			break
		}
	}

	// Drain remaining to avoid goroutine leak
	for range contentCh {
	}
	for range errCh {
	}

	// Verify slot was released by acquiring it again
	freshCtx, freshCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer freshCancel()
	err := scheduler.AcquireAPISlot(freshCtx, "test")
	if err != nil {
		t.Fatalf("Slot was leaked after rapid cancel: %v", err)
	}
	scheduler.ReleaseAPISlot("test")

	t.Logf("Rapid cancel: read %d chunks before cancellation, slot released cleanly", chunkCount)
}

// TestAPISchedulerGap_GlobalConfig_SyncOnce tests that ConfigureGlobalAPIScheduler
// dynamically reconfigures the global instance even after GetAPIScheduler has been called.
func TestAPISchedulerGap_GlobalConfig_SyncOnce(t *testing.T) {
	scheduler := GetAPIScheduler()
	if scheduler == nil {
		t.Fatal("GetAPIScheduler returned nil")
	}

	originalMax := scheduler.config.MaxConcurrentAPICalls

	// Attempt to reconfigure — should be applied dynamically!
	ConfigureGlobalAPIScheduler(APISchedulerConfig{
		MaxConcurrentAPICalls: originalMax + 2,
		SlotAcquireTimeout:    99 * time.Second,
	})

	scheduler2 := GetAPIScheduler()
	if scheduler2 != scheduler {
		t.Error("GetAPIScheduler returned different instance after reconfigure attempt")
	}
	metrics := scheduler2.GetMetrics()
	if metrics.MaxSlots != originalMax+2 {
		t.Errorf("Config was not modified dynamically: expected %d, got %d",
			originalMax+2, metrics.MaxSlots)
	}

	// Restore original config for cleanliness
	ConfigureGlobalAPIScheduler(APISchedulerConfig{
		MaxConcurrentAPICalls: originalMax,
		SlotAcquireTimeout:    5 * time.Minute,
	})
}

func (m *mockLLMClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error) {
	contentChan := make(chan string, 1)
	errorChan := make(chan error, 1)
	if m.StreamingNotSupported {
		close(contentChan)
		errorChan <- ErrStreamingNotSupported
		close(errorChan)
		return contentChan, errorChan
	}
	go func() {
		defer close(contentChan)
		defer close(errorChan)
		res, err := m.CompleteWithSystem(ctx, systemPrompt, userPrompt)
		if err != nil {
			errorChan <- err
			return
		}
		contentChan <- res
	}()
	return contentChan, errorChan
}
