package context

import (
	"path/filepath"
	"testing"

	"codenerd/internal/core"
)

// TestActivationEngineContextSetters verifies the issue / back-reference /
// feedback-store setters install and clear their state, and that SetState /
// GetState round-trip the activation state.
func TestActivationEngineContextSetters(t *testing.T) {
	ae := NewActivationEngine(DefaultConfig())

	issue := &IssueActivationContext{IssueID: "GH-1", IssueText: "panic on nil map", Source: "github"}
	ae.SetIssueContext(issue)
	if ae.issueContext != issue {
		t.Error("SetIssueContext did not install the issue context")
	}
	ae.ClearIssueContext()
	if ae.issueContext != nil {
		t.Error("ClearIssueContext did not clear the issue context")
	}

	back := &BackReferenceActivationContext{ReferencedTurnIDs: []int{3, 5}, ReferenceStrength: 0.9}
	ae.SetBackReferenceContext(back)
	if ae.backReferenceContext != back {
		t.Error("SetBackReferenceContext did not install the back-reference context")
	}
	ae.ClearBackReferenceContext()
	if ae.backReferenceContext != nil {
		t.Error("ClearBackReferenceContext did not clear the back-reference context")
	}

	store, err := NewContextFeedbackStore(filepath.Join(t.TempDir(), "fb.db"))
	if err != nil {
		t.Fatalf("NewContextFeedbackStore: %v", err)
	}
	defer store.Close()
	ae.SetFeedbackStore(store)
	if ae.feedbackStore != store {
		t.Error("SetFeedbackStore did not install the feedback store")
	}

	// SetState / GetState round-trip; ClearState wipes it back to empty.
	want := ActivationState{
		ActiveIntent: &core.Fact{Predicate: "user_intent", Args: []any{"fix the bug"}},
		FocusedPaths: []string{"main.go"},
	}
	ae.SetState(want)
	got := ae.GetState()
	if got.ActiveIntent == nil || got.ActiveIntent.Predicate != "user_intent" {
		t.Errorf("GetState after SetState lost the active intent: %+v", got)
	}
	if len(got.FocusedPaths) != 1 || got.FocusedPaths[0] != "main.go" {
		t.Errorf("GetState after SetState lost focused paths: %+v", got.FocusedPaths)
	}
	ae.ClearState()
	if ae.GetState().ActiveIntent != nil {
		t.Error("ClearState should reset the active intent to nil")
	}
}
