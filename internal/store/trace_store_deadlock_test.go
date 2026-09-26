package store

import (
	"sync/atomic"
	"testing"
	"time"
)

// GetLearningInsights holds the trace store's read lock while it collects
// failure patterns. It used to collect them through GetFailurePatterns, which
// took the read lock again, and sync.RWMutex readers must not recurse: a
// writer queued between the two RLocks blocks the second, the writer waits on
// the first, and both hang. Writers hammer the lock here so one lands in that
// window; a single uncontended call never deadlocks, which is why the first
// version of this test passed against the bug.
func TestTraceStore_GetLearningInsightsDoesNotRecurseOnItsReadLock(t *testing.T) {
	local, err := NewLocalStore(t.TempDir() + "/trace-lock.db")
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}
	defer local.Close()

	ts := local.GetTraceStore()
	if err := ts.StoreReasoningTrace(&ReasoningTrace{
		ID: "failed-trace", ShardID: "coder-1", ShardType: "coder",
		SystemPrompt: "system", UserPrompt: "user", Response: "response",
		Success: false, ErrorMessage: "compile failed", CreatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("StoreReasoningTrace: %v", err)
	}

	var stop atomic.Bool
	defer stop.Store(true)
	go func() {
		for !stop.Load() {
			ts.mu.Lock()
			ts.mu.Unlock() //nolint:staticcheck // an empty critical section is the point: a queued writer
		}
	}()

	done := make(chan error, 1)
	go func() {
		for i := 0; i < 200; i++ {
			if _, err := ts.GetLearningInsights("coder", 7); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("GetLearningInsights: %v", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("GetLearningInsights deadlocked: it re-acquired the read lock it holds while a writer was queued")
	}
}
