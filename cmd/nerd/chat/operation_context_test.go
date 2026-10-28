package chat

import (
	"codenerd/internal/types"
	"testing"

	"codenerd/internal/usage"
)

// Every long-lived chat operation (init, scan, delegation, tool runs) runs
// under sessionOperationContext. It must carry the usage tracker and the
// session id: without them those operations' LLM calls record nothing —
// an entire /init run of planner-tier calls was invisible in usage.json —
// or land under BySession "unknown".
func TestSessionOperationContext_CarriesUsageTrackerAndSession(t *testing.T) {
	tracker, err := usage.NewTracker(t.TempDir())
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}
	m := NewTestModel()
	m.usageTracker = tracker
	m.sessionID = "session-op-test"

	ctx, cancel := m.sessionOperationContext()
	defer cancel()
	if usage.FromContext(ctx) == nil {
		t.Fatal("operation context carries no usage tracker")
	}
	usage.RecordFromContext(ctx, usage.Report{Model: "claude-opus-5-5", Provider: "anthropic", Operation: "chat", Usage: types.UsageMetadata{InputTokens: 10, OutputTokens: 5}})
	if got := tracker.Stats().BySession["session-op-test"].Input; got != 10 {
		t.Errorf("BySession[session-op-test].Input = %d, want 10 (recorded under the session, not unknown)", got)
	}
}
