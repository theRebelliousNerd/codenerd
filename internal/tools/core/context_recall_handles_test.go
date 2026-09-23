package core

import (
	"context"
	"strings"
	"testing"
)

// A model holding a subagent handle reaches for the one recall verb, and it is
// answered with the transcript. It used to be told "no archived observation
// has id obs:sa:..." (campaign 7b853890, 2026-09-22), with no working-context
// recall even consulted for the handle's kind.
func TestRecallContext_RedeemsASubagentHandle(t *testing.T) {
	handle, buried := seedRetainedReturn(t)

	// No working-context recall in the context: a retained handle does not
	// need one.
	out, err := RecallContextTool().Execute(context.Background(), map[string]any{"id": handle})
	if err != nil {
		t.Fatalf("recall_context(%s): %v", handle, err)
	}
	if !strings.Contains(out, buried) {
		t.Fatalf("recall_context did not return the retained transcript; got:\n%s", out)
	}
}

// An observation id still needs the working context, and says so when it is
// absent: the handle routing takes only the handle kinds.
func TestRecallContext_AnObservationIDStillNeedsTheWorkingContext(t *testing.T) {
	_, err := RecallContextTool().Execute(context.Background(), map[string]any{"id": "obs-123"})
	if err == nil || !strings.Contains(err.Error(), "working context recall unavailable") {
		t.Fatalf("err = %v, want the working-context error", err)
	}
}
