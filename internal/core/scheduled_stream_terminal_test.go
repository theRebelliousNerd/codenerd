package core

import (
	"context"
	"testing"
	"time"

	"codenerd/internal/types"
)

// The scheduled stream holds its API slot for the whole stream and releases it
// only after the underlying terminal result is in, and it forwards that
// terminal — stop reason and usage included — unchanged. A wrapper that
// released early or rewrote the terminal would let the scheduler overcommit
// the provider, or erase the accounting the broker settles on.
func TestScheduledLLMCall_CompleteWithStreaming_HoldsSlotAndForwardsTerminal(t *testing.T) {
	scheduler := NewAPIScheduler(APISchedulerConfig{
		MaxConcurrentAPICalls: 1,
		SlotAcquireTimeout:    5 * time.Second,
	})
	scheduler.RegisterShard("stream", "test")

	want := types.StreamResult{
		StopReason: "end_turn",
		Usage: types.UsageMetadata{
			InputTokens: 120, OutputTokens: 30, TotalTokens: 150,
			CachedContentTokens: 80, CacheWriteTokens: 10,
			Known: types.UsageTokens | types.UsageCacheRead | types.UsageCacheWrite,
		},
	}
	gate := make(chan struct{})
	mock := &mockStreamingClient{
		streamFunc: func(ctx context.Context, sys, usr string, think bool) (<-chan string, <-chan types.StreamResult) {
			w, content, done := types.NewStream()
			go func() {
				defer w.Close()
				<-gate
				if err := w.Content(ctx, "hello"); err != nil {
					w.Fail(err)
					return
				}
				w.Finish(want)
			}()
			return content, done
		},
	}
	call := &ScheduledLLMCall{Scheduler: scheduler, ShardID: "stream", Client: mock}

	ctx := context.Background()
	content, done := call.CompleteWithStreaming(ctx, "sys", "usr", false)

	// While the underlying stream is open, the single slot is held.
	if got := scheduler.GetMetrics().ActiveSlots; got != 1 {
		t.Fatalf("ActiveSlots during stream = %d, want 1", got)
	}

	close(gate)
	text, res := types.CollectStream(ctx, content, done, nil)
	if text != "hello" {
		t.Fatalf("content = %q, want %q", text, "hello")
	}
	if res.Err != nil || res.StopReason != want.StopReason || res.Usage != want.Usage {
		t.Fatalf("terminal = %+v, want %+v forwarded unchanged", res, want)
	}

	// After the terminal has been read the slot comes back.
	deadline := time.Now().Add(2 * time.Second)
	for scheduler.GetMetrics().ActiveSlots != 0 {
		if time.Now().After(deadline) {
			t.Fatal("API slot still held after the stream's terminal was read")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
