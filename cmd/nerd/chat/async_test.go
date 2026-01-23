// Package chat provides tests for async operations and goroutine safety.
// This file tests command functions, cancellation, and resource cleanup.
package chat

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"
)

// =============================================================================
// SHUTDOWN TESTS
// =============================================================================

func TestShutdown_Idempotent(t *testing.T) {
	t.Parallel()
	m := NewTestModel()

	// Should not panic on multiple shutdowns
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Panic on multiple shutdowns: %v", r)
		}
	}()

	m.Shutdown()
	m.Shutdown()
	m.Shutdown()
}

func TestShutdown_Timeout(t *testing.T) {
	t.Parallel()
	m := NewTestModel()

	// Shutdown should complete within timeout
	done := make(chan struct{})
	go func() {
		m.Shutdown()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Error("Shutdown timed out")
	}
}

func TestShutdown_CancelsContext(t *testing.T) {
	t.Parallel()
	m := NewTestModel()

	// Store the context before shutdown
	ctx := m.shutdownCtx

	m.Shutdown()

	// Context should be cancelled
	select {
	case <-ctx.Done():
		// Success - context was cancelled
	case <-time.After(time.Second):
		t.Error("Shutdown did not cancel context")
	}
}

func TestShutdown_ClosesStatusChannel(t *testing.T) {
	t.Parallel()
	m := NewTestModel()

	// Get reference to status channel before shutdown
	statusChan := m.statusChan

	m.Shutdown()

	// Channel should be closed
	select {
	case _, ok := <-statusChan:
		if ok {
			// Got a value, try reading again
			_, ok = <-statusChan
		}
		if ok {
			t.Error("Status channel not closed after shutdown")
		}
	case <-time.After(time.Second):
		t.Error("Status channel read timed out")
	}
}

// =============================================================================
// CONTEXT CANCELLATION TESTS
// =============================================================================

func TestContextCancellation_Respected(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Operations should respect cancelled context
	select {
	case <-ctx.Done():
		// Expected
	default:
		t.Error("Context should be done")
	}
}

func TestContextCancellation_WithTimeout(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	// Wait for context to timeout
	<-ctx.Done()

	if ctx.Err() != context.DeadlineExceeded {
		t.Errorf("Expected DeadlineExceeded, got %v", ctx.Err())
	}
}

// =============================================================================
// GOROUTINE SAFETY TESTS
// =============================================================================

func TestConcurrentUpdates(t *testing.T) {
	t.Parallel()

	m := NewTestModel()
	var wg sync.WaitGroup
	iterations := 100

	// Simulate concurrent updates
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Panic in concurrent update %d: %v", i, r)
				}
			}()

			// Note: Bubbletea normally serializes updates, but this tests
			// that our code doesn't have obvious race conditions
			msg := statusMsg("status update")
			_, _ = m.Update(msg)
		}(i)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Error("Concurrent updates timed out")
	}
}

func TestConcurrentViews(t *testing.T) {
	t.Parallel()

	m := NewTestModel(
		WithHistory(
			Message{Role: "user", Content: "test", Time: time.Now()},
		),
	)

	var wg sync.WaitGroup
	iterations := 50

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Panic in concurrent view %d: %v", i, r)
				}
			}()

			_ = m.View()
		}(i)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Error("Concurrent views timed out")
	}
}

func TestGoroutineCount_AfterOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping goroutine count test in short mode")
	}
	t.Parallel()

	// Wait for any previous goroutines to settle
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	before := runtime.NumGoroutine()

	// Create and use multiple models
	for i := 0; i < 10; i++ {
		m := NewTestModel()

		// Perform some operations that don't require kernel
		m, _ = SimulateInput(m, "/help")
		m, _ = SimulateInput(m, "/usage")

		// Cleanup
		m.Shutdown()
	}

	// Wait for cleanup
	time.Sleep(500 * time.Millisecond)
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	after := runtime.NumGoroutine()

	// Allow some slack for runtime goroutines
	if after > before+10 {
		t.Errorf("Possible goroutine leak: before=%d after=%d (diff=%d)", before, after, after-before)
	}
}

// =============================================================================
// WAITGROUP TESTS
// =============================================================================

func TestWaitGroup_TracksGoroutines(t *testing.T) {
	t.Parallel()

	m := NewTestModel()
	wg := m.goroutineWg

	// Add some work
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond)
	}()

	// Wait should complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(time.Second):
		t.Error("WaitGroup.Wait timed out")
	}
}

// =============================================================================
// CHANNEL SAFETY TESTS
// =============================================================================

func TestStatusChannel_NonBlocking(t *testing.T) {
	t.Parallel()

	m := NewTestModel()

	// ReportStatus should not block even if channel is full
	for i := 0; i < 100; i++ {
		m.ReportStatus("status update")
	}

	// Should complete without blocking
}

func TestStatusChannel_ClosedSafety(t *testing.T) {
	t.Parallel()

	m := NewTestModel()
	m.Shutdown() // This closes statusChan

	// ReportStatus should not panic on closed channel
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Panic on ReportStatus after shutdown: %v", r)
		}
	}()

	m.ReportStatus("after shutdown")
}

// =============================================================================
// TEA.CMD TESTS
// =============================================================================

func TestTeaCmd_BatchSafety(t *testing.T) {
	t.Parallel()

	m := NewTestModel()

	// Create multiple commands
	cmd1 := m.tickMemory()
	cmd2 := m.waitForStatus()

	// Batch should work
	if cmd1 == nil || cmd2 == nil {
		t.Log("Some commands are nil (expected in test environment)")
	}
}

func TestTeaCmd_NilHandling(t *testing.T) {
	t.Parallel()

	m := NewTestModel()

	// Update should handle nil commands gracefully
	newModel, cmd := m.Update(statusMsg("test"))
	_ = newModel

	// Don't execute commands that may block (like waitForStatus)
	// Just verify the update worked
	if cmd != nil {
		t.Log("Update returned a command")
	}
}

// =============================================================================
// INTERRUPT HANDLING TESTS
// =============================================================================

func TestInterrupt_SetsFlag(t *testing.T) {
	t.Parallel()

	m := NewTestModel(WithLoading(true))

	newModel, _ := m.Update(TestMessages.KeyCtrlX)
	result := newModel.(Model)

	if !result.isInterrupted {
		t.Error("Expected isInterrupted to be true after Ctrl+X")
	}
}

func TestInterrupt_StopsLoading(t *testing.T) {
	t.Parallel()

	m := NewTestModel(WithLoading(true))

	newModel, _ := m.Update(TestMessages.KeyCtrlX)
	result := newModel.(Model)

	if result.isLoading {
		t.Error("Expected isLoading to be false after Ctrl+X")
	}
}

func TestInterrupt_ClearsOnContinue(t *testing.T) {
	t.Parallel()

	m := NewTestModel(
		WithPendingSubtasks(Subtask{ID: "1", Description: "test", ShardType: "coder"}),
	)
	m.isInterrupted = true

	newModel, _ := m.handleCommand("/continue")
	result := newModel.(Model)

	if result.isInterrupted {
		t.Error("Expected isInterrupted to be cleared after /continue")
	}
}

// =============================================================================
// RESOURCE CLEANUP TESTS
// =============================================================================

func TestResourceCleanup_OnShutdown(t *testing.T) {
	t.Parallel()

	m := NewTestModel()

	// Ensure resources are initialized
	if m.shutdownCtx == nil {
		t.Error("shutdownCtx should be initialized")
	}
	if m.goroutineWg == nil {
		t.Error("goroutineWg should be initialized")
	}
	if m.statusChan == nil {
		t.Error("statusChan should be initialized")
	}

	m.Shutdown()

	// Context should be cancelled
	select {
	case <-m.shutdownCtx.Done():
		// Expected
	default:
		t.Error("shutdownCtx should be cancelled after shutdown")
	}
}

// =============================================================================
// SHARD RESULT HISTORY TESTS
// =============================================================================

func TestShardResultHistory_SlidingWindow(t *testing.T) {
	t.Parallel()

	m := NewTestModel()

	// Add more than maxHistorySize results
	for i := 0; i < 15; i++ {
		m.storeShardResult("test", "task", "result", nil)
	}

	// Should be capped at maxHistorySize (10)
	if len(m.shardResultHistory) > 10 {
		t.Errorf("Expected max 10 results, got %d", len(m.shardResultHistory))
	}
}

func TestShardResultHistory_LastResult(t *testing.T) {
	t.Parallel()

	m := NewTestModel()

	m.storeShardResult("coder", "write code", "result1", nil)
	m.storeShardResult("tester", "run tests", "result2", nil)

	if m.lastShardResult == nil {
		t.Error("Expected lastShardResult to be set")
		return
	}

	if m.lastShardResult.ShardType != "tester" {
		t.Errorf("Expected last shard type 'tester', got '%s'", m.lastShardResult.ShardType)
	}
}

// =============================================================================
// RENDERING CACHE TESTS
// =============================================================================

func TestRenderingCache_AddMessage(t *testing.T) {
	t.Parallel()

	m := NewTestModel()

	msg := Message{Role: "user", Content: "test", Time: time.Now()}
	m = m.addMessage(msg)

	if len(m.renderedCache) != 1 {
		t.Errorf("Expected 1 cached message, got %d", len(m.renderedCache))
	}
}

func TestRenderingCache_BulkAdd(t *testing.T) {
	t.Parallel()

	m := NewTestModel()

	msgs := []Message{
		{Role: "user", Content: "test1", Time: time.Now()},
		{Role: "assistant", Content: "test2", Time: time.Now()},
		{Role: "user", Content: "test3", Time: time.Now()},
	}
	m = m.addMessages(msgs...)

	if len(m.renderedCache) != 3 {
		t.Errorf("Expected 3 cached messages, got %d", len(m.renderedCache))
	}
}

func TestRenderingCache_InvalidationIndex(t *testing.T) {
	t.Parallel()

	m := NewTestModel()

	// Add messages
	for i := 0; i < 5; i++ {
		msg := Message{Role: "user", Content: "test", Time: time.Now()}
		m = m.addMessage(msg)
	}

	// Cache invalidation index should be at the end
	if m.cacheInvalidFrom != 5 {
		t.Errorf("Expected cacheInvalidFrom=5, got %d", m.cacheInvalidFrom)
	}
}

// =============================================================================
// INPUT HISTORY TESTS
// =============================================================================

func TestInputHistory_Preserved(t *testing.T) {
	t.Parallel()

	m := NewTestModel()
	m.inputHistory = []string{"first", "second", "third"}

	// History should be accessible
	if len(m.inputHistory) != 3 {
		t.Errorf("Expected 3 history items, got %d", len(m.inputHistory))
	}

	if m.inputHistory[0] != "first" {
		t.Errorf("Expected 'first', got '%s'", m.inputHistory[0])
	}
}

// =============================================================================
// KNOWLEDGE HISTORY TESTS
// =============================================================================

func TestKnowledgeHistory_Format(t *testing.T) {
	t.Parallel()

	kr := KnowledgeResult{
		Query:      "test query",
		Specialist: "researcher",
		Response:   "test response",
		Timestamp:  time.Now(),
	}

	if kr.Query != "test query" {
		t.Errorf("Expected 'test query', got '%s'", kr.Query)
	}
	if kr.Specialist != "researcher" {
		t.Errorf("Expected 'researcher', got '%s'", kr.Specialist)
	}
	if kr.Response != "test response" {
		t.Errorf("Expected 'test response', got '%s'", kr.Response)
	}
	if kr.Timestamp.IsZero() {
		t.Errorf("Expected non-zero timestamp")
	}
}

// =============================================================================
// CONTINUATION STATE TESTS
// =============================================================================

func TestContinuationMode_Cycle(t *testing.T) {
	t.Parallel()

	modes := []ContinuationMode{
		ContinuationModeAuto,
		ContinuationModeConfirm,
		ContinuationModeBreakpoint,
	}

	for i, mode := range modes {
		next := (mode + 1) % 3
		expected := modes[(i+1)%3]

		if next != expected {
			t.Errorf("Mode cycle broken: %v + 1 = %v, expected %v", mode, next, expected)
		}
	}
}

func TestContinuationMode_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mode     ContinuationMode
		expected string
	}{
		{ContinuationModeAuto, "Auto"},
		{ContinuationModeConfirm, "Confirm"},
		{ContinuationModeBreakpoint, "Breakpoint"},
	}

	for _, tc := range tests {
		if tc.mode.String() != tc.expected {
			t.Errorf("Expected '%s', got '%s'", tc.expected, tc.mode.String())
		}
	}
}

func TestSubtask_Fields(t *testing.T) {
	t.Parallel()

	st := Subtask{
		ID:          "task-1",
		Description: "Do something",
		ShardType:   "coder",
		IsMutation:  true,
	}

	if st.ID != "task-1" {
		t.Errorf("Expected ID 'task-1', got '%s'", st.ID)
	}
	if !st.IsMutation {
		t.Error("Expected IsMutation to be true")
	}
	if st.Description != "Do something" {
		t.Errorf("Unexpected description: %s", st.Description)
	}
	if st.ShardType != "coder" {
		t.Errorf("Unexpected shard type: %s", st.ShardType)
	}
}
