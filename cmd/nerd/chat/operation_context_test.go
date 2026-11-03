package chat
// TODO: Add negative test for null m.usageTracker behavior
// TODO: Add boundary test for empty string m.sessionID
// TODO: Add test verifying fallback to context.Background() when m.shutdownCtx is nil
// TODO: Add adversarial test coercing extreme string lengths into m.sessionID (100MB string)
// TODO: Add adversarial test for non-UTF8 / null byte injection in m.sessionID
// TODO: Add test verifying context key collision resistance (are keys unexported types or strings?)
// TODO: Add stress test simulating 1,000,000 concurrent context creations (Goroutine leak detection)
// TODO: Add long-running duration simulation to test memory usage of un-cancelled contexts over 45 days
// TODO: Add `-race` enabled concurrency test with 100 goroutines mutating the usage tracker on the same session
// TODO: Add context cancellation race condition test (simultaneous cancel and tracker write)

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
	// Close before t.TempDir is removed: the auto-save timer otherwise
	// fires into a deleted directory and logs a warning into whatever test
	// runs next (it broke TestDebugSelf_ProjectsProblemsLogIntoKernel).
	t.Cleanup(func() { _ = tracker.Close() })
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
