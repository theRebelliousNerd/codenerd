package chat

import (
	"context"
	"testing"
	"time"

	"codenerd/internal/northstar"
	"codenerd/internal/shards"
)

// northstarHandlerAdapter is the only thing standing between the Guardian's
// verdict and the background observer manager that acts on it. A field dropped
// or a level mistranslated here turns a "block" into a silent "proceed", so
// these tests pin the whole conversion rather than spot-checking it.

type scriptedAlignmentClient struct {
	response string
	err      error
}

func (c *scriptedAlignmentClient) CompleteWithSystem(ctx context.Context, system, user string) (string, error) {
	if c.err != nil {
		return "", c.err
	}
	return c.response, nil
}

func newTestAdapter(t *testing.T, response string) *northstarHandlerAdapter {
	t.Helper()
	t.Cleanup(northstar.ResetGuardianRegistry)

	nerdDir := t.TempDir()
	guardian, err := northstar.AcquireGuardian(nerdDir, northstar.DefaultGuardianConfig())
	if err != nil {
		t.Fatalf("AcquireGuardian: %v", err)
	}
	t.Cleanup(func() { _ = northstar.ReleaseGuardian(guardian) })

	if err := guardian.UpdateVision(&northstar.Vision{
		Mission:    "Keep the kernel the executive",
		Problem:    "Improvised control flow",
		VisionStmt: "Facts decide",
	}); err != nil {
		t.Fatalf("UpdateVision: %v", err)
	}
	guardian.SetLLMClient(&scriptedAlignmentClient{response: response})

	return &northstarHandlerAdapter{handler: northstar.NewBackgroundEventHandler(guardian, "session-test")}
}

func testEvent() shards.ObserverEvent {
	return shards.ObserverEvent{
		Type:      shards.ObserverEventType("task_completed"),
		Source:    "coder",
		Target:    "internal/core/kernel.go",
		Details:   map[string]string{"task_description": "rewrite the evaluator"},
		Timestamp: time.Now(),
	}
}

func TestNorthstarHandlerAdapter_WhenGuardianBlocks_ShouldReturnBlockAssessment(t *testing.T) {
	adapter := newTestAdapter(t, "SCORE: 0.1\nRESULT: blocked\nEXPLANATION: contradicts the mission\nSUGGESTIONS: revert, rescope")

	got, err := adapter.HandleEvent(context.Background(), testEvent())
	if err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if got == nil {
		t.Fatal("adapter returned no assessment for a blocked check")
	}
	if got.Level != shards.AssessmentLevel("block") {
		t.Errorf("Level = %q, want block", got.Level)
	}
	if got.Score != 10 {
		t.Errorf("Score = %d, want 10 (0.1 scaled to 0-100)", got.Score)
	}
	if got.ObserverName != "northstar" {
		t.Errorf("ObserverName = %q, want northstar", got.ObserverName)
	}
	if got.VisionMatch != "contradicts the mission" {
		t.Errorf("VisionMatch = %q, want the guardian's explanation", got.VisionMatch)
	}
	if len(got.Suggestions) != 2 {
		t.Errorf("Suggestions = %v, want both parsed suggestions", got.Suggestions)
	}
	if got.EventID == "" {
		t.Error("EventID is empty; the assessment cannot be traced back to its alignment check")
	}
	if got.Metadata["trigger"] != string(northstar.TriggerTaskComplete) {
		t.Errorf("Metadata[trigger] = %q, want %q", got.Metadata["trigger"], northstar.TriggerTaskComplete)
	}
	if got.Timestamp.IsZero() {
		t.Error("Timestamp not carried through the adapter")
	}
}

func TestNorthstarHandlerAdapter_WhenGuardianPasses_ShouldReturnProceed(t *testing.T) {
	adapter := newTestAdapter(t, "SCORE: 0.95\nRESULT: passed\nEXPLANATION: on mission\nSUGGESTIONS: none")

	got, err := adapter.HandleEvent(context.Background(), testEvent())
	if err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if got == nil {
		t.Fatal("adapter returned no assessment")
	}
	if got.Level != shards.AssessmentLevel("proceed") {
		t.Errorf("Level = %q, want proceed", got.Level)
	}
	if got.Score != 95 {
		t.Errorf("Score = %d, want 95", got.Score)
	}
	if len(got.Suggestions) != 0 {
		t.Errorf("Suggestions = %v, want none", got.Suggestions)
	}
}

func TestNorthstarHandlerAdapter_WhenNoVisionDefined_ShouldReturnNilAssessment(t *testing.T) {
	t.Cleanup(northstar.ResetGuardianRegistry)

	nerdDir := t.TempDir()
	guardian, err := northstar.AcquireGuardian(nerdDir, northstar.DefaultGuardianConfig())
	if err != nil {
		t.Fatalf("AcquireGuardian: %v", err)
	}
	t.Cleanup(func() { _ = northstar.ReleaseGuardian(guardian) })
	if err := guardian.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	adapter := &northstarHandlerAdapter{handler: northstar.NewBackgroundEventHandler(guardian, "session-test")}

	got, err := adapter.HandleEvent(context.Background(), testEvent())
	if err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if got != nil {
		t.Errorf("assessment = %+v, want nil when no vision is defined", got)
	}
}

func TestNorthstarHandlerAdapter_WhenEventTypeIsFileModified_ShouldUseHighImpactTrigger(t *testing.T) {
	adapter := newTestAdapter(t, "SCORE: 0.8\nRESULT: passed\nEXPLANATION: fine\nSUGGESTIONS: none")

	event := testEvent()
	event.Type = shards.ObserverEventType("file_modified")

	got, err := adapter.HandleEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if got == nil {
		t.Fatal("adapter returned no assessment")
	}
	if got.Metadata["trigger"] != string(northstar.TriggerHighImpact) {
		t.Errorf("Metadata[trigger] = %q, want %q", got.Metadata["trigger"], northstar.TriggerHighImpact)
	}
}

// The adapter must satisfy the interface the observer manager stores, or the
// wiring in session_boot.go silently degrades to no northstar handler at all.
var _ shards.NorthstarHandler = (*northstarHandlerAdapter)(nil)
