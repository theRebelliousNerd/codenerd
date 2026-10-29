package system

import (
	"context"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/core"
)

// TestAutopoiesisLifetime_NoWallClock pins that tool generation is not
// parented on the boot caller's context and carries no deadline. Cortex.Close
// cancels it, which is the ouroborosCancel stored beside the context.
func TestAutopoiesisLifetime_NoWallClock(t *testing.T) {
	caller, cancelCaller := context.WithCancel(context.Background())
	bctx := &bootContext{ctx: caller}
	life := ensureOuroborosLifetime(bctx)
	if _, ok := life.Deadline(); ok {
		t.Fatal("autopoiesis lifetime has a deadline")
	}
	if again := ensureOuroborosLifetime(bctx); again != life {
		t.Fatal("ensureOuroborosLifetime created a second lifetime")
	}
	cancelCaller()
	if err := life.Err(); err != nil {
		t.Fatalf("boot caller cancellation stopped autopoiesis: %v", err)
	}
	bctx.ouroborosCancel()
	if err := life.Err(); err == nil {
		t.Fatal("ouroborosCancel left the lifetime running")
	}
	if detached := ensureOuroborosLifetime(nil); detached.Err() != nil {
		t.Fatalf("nil boot context = %v", detached.Err())
	}
}

// TestServeDreamToolNeeds_NoWallClock pins that one dream-queue generation
// inherits the lifetime and nothing else. Closing the queue ends the loop;
// cancelling the lifetime ends it even when a need is in hand.
func TestServeDreamToolNeeds_NoWallClock(t *testing.T) {
	lifetime, cancel := context.WithCancel(context.Background())
	needs := make(chan core.ToolNeed, 2)
	needs <- core.ToolNeed{Name: "formatter", Description: "format go", Priority: 1}
	close(needs)

	ran := make(chan context.Context, 1)
	done := make(chan struct{})
	go func() {
		serveDreamToolNeeds(lifetime, needs, func(ctx context.Context, need core.ToolNeed) {
			if need.Name != "formatter" {
				t.Errorf("need = %q", need.Name)
			}
			ran <- ctx
		})
		close(done)
	}()

	select {
	case ctx := <-ran:
		if _, ok := ctx.Deadline(); ok {
			t.Fatal("tool generation ran under a deadline")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("generation did not run")
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("serveDreamToolNeeds did not return after the queue closed")
	}
	cancel()

	blocked := make(chan context.Context, 1)
	open := make(chan core.ToolNeed, 1)
	stopped := make(chan struct{})
	life, lifeCancel := context.WithCancel(context.Background())
	go func() {
		serveDreamToolNeeds(life, open, func(ctx context.Context, need core.ToolNeed) {
			blocked <- ctx
			<-ctx.Done()
		})
		close(stopped)
	}()
	open <- core.ToolNeed{Name: "linter"}
	select {
	case <-blocked:
	case <-time.After(2 * time.Second):
		t.Fatal("second generation did not start")
	}
	lifeCancel()
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("cancelling the lifetime left generation running")
	}
}

// TestToolRefinementContext_UsesPerCallTimeout pins one refinement completion
// to llm_timeouts.per_call_timeout on the autopoiesis lifetime, not a
// detached five-minute clock.
func TestToolRefinementContext_UsesPerCallTimeout(t *testing.T) {
	prev := config.GetLLMTimeouts()
	t.Cleanup(func() { config.SetLLMTimeouts(prev) })
	configured := prev
	configured.PerCallTimeout = 37 * time.Second
	config.SetLLMTimeouts(configured)

	lifetime, cancel := context.WithCancel(context.Background())
	ctx, stop := toolRefinementContext(lifetime)
	t.Cleanup(stop)
	dl, ok := ctx.Deadline()
	if !ok {
		t.Fatal("refinement context has no deadline")
	}
	rem := time.Until(dl)
	if rem < 30*time.Second || rem > 37*time.Second {
		t.Fatalf("refinement deadline remaining %s, want per_call_timeout (37s)", rem)
	}

	detached, detachStop := toolRefinementContext(nil)
	t.Cleanup(detachStop)
	if _, ok := detached.Deadline(); !ok {
		t.Fatal("nil lifetime dropped the per-call bound")
	}

	cancel()
	select {
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("cancelling the autopoiesis lifetime left refinement running")
	}

	configured.PerCallTimeout = 0
	config.SetLLMTimeouts(configured)
	open, openStop := toolRefinementContext(context.Background())
	t.Cleanup(openStop)
	if _, ok := open.Deadline(); ok {
		t.Fatal("non-positive per_call_timeout added a deadline")
	}
}
