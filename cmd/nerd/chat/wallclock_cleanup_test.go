package chat

import (
	"context"
	"testing"
	"time"

	"codenerd/internal/config"
)

// TestDialogueCompressionContext_NoWallClock pins that background turn
// compression is not cut off when the turn returns. The session's shutdown
// context is the stop; it carries no deadline of its own.
func TestDialogueCompressionContext_NoWallClock(t *testing.T) {
	detached := dialogueCompressionContext(nil)
	if err := detached.Err(); err != nil {
		t.Fatalf("nil shutdown context is already done: %v", err)
	}
	if _, ok := detached.Deadline(); ok {
		t.Fatal("nil shutdown context gained a deadline")
	}

	shutdown, cancel := context.WithCancel(context.Background())
	got := dialogueCompressionContext(shutdown)
	if _, ok := got.Deadline(); ok {
		t.Fatal("compression context has a deadline; it stops when the session shuts down")
	}
	cancel()
	if err := got.Err(); err == nil {
		t.Fatal("cancelling session shutdown left compression running")
	}
}

// TestReviewNarrativeContext_UsesArticulationTimeout pins the one narrative
// completion to llm_timeouts.articulation_timeout. The review command's
// context stays the parent, so a shorter caller deadline still wins.
func TestReviewNarrativeContext_UsesArticulationTimeout(t *testing.T) {
	prev := config.GetLLMTimeouts()
	t.Cleanup(func() { config.SetLLMTimeouts(prev) })

	configured := prev
	configured.ArticulationTimeout = 37 * time.Second
	config.SetLLMTimeouts(configured)

	parent, cancel := context.WithCancel(context.Background())
	ctx, stop := reviewNarrativeContext(parent)
	t.Cleanup(stop)

	dl, ok := ctx.Deadline()
	if !ok {
		t.Fatal("narrative context has no deadline")
	}
	rem := time.Until(dl)
	if rem < 30*time.Second || rem > 37*time.Second {
		t.Fatalf("narrative deadline remaining %s, want articulation_timeout (37s)", rem)
	}
	cancel()
	select {
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("cancelling the review context left the narrative call running")
	}

	short, shortCancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(shortCancel)
	child, childStop := reviewNarrativeContext(short)
	t.Cleanup(childStop)
	childDL, ok := child.Deadline()
	if !ok {
		t.Fatal("narrative context dropped the review command's deadline")
	}
	if rem := time.Until(childDL); rem > 6*time.Second {
		t.Fatalf("review deadline remaining %s; the caller context must still bound the call", rem)
	}

	configured.ArticulationTimeout = 0
	config.SetLLMTimeouts(configured)
	open, openStop := reviewNarrativeContext(context.Background())
	t.Cleanup(openStop)
	if _, ok := open.Deadline(); ok {
		t.Fatal("non-positive articulation_timeout added a deadline")
	}
}
