package northstar

import (
	"context"
	"testing"
	"time"
)

// =============================================================================
// CAMPAIGN OBSERVER TESTS
// =============================================================================

func TestNewCampaignObserver(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	guardian := NewGuardian(store, DefaultGuardianConfig())
	guardian.Initialize()

	observer := NewCampaignObserver(guardian)

	if observer == nil {
		t.Fatal("expected non-nil observer")
	}
	if observer.guardian != guardian {
		t.Error("guardian not set correctly")
	}
}

func TestCampaignObserver_StartCampaign(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	guardian := NewGuardian(store, DefaultGuardianConfig())
	guardian.Initialize()

	observer := NewCampaignObserver(guardian)

	err := observer.StartCampaign(context.Background(), "campaign-1", "Build feature X")
	if err != nil {
		t.Fatalf("StartCampaign error: %v", err)
	}

	observer.mu.RLock()
	campID := observer.campaignID
	observer.mu.RUnlock()

	if campID != "campaign-1" {
		t.Errorf("campaignID not set: got %q", campID)
	}
}

func TestCampaignObserver_OnPhaseStart(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	guardian := NewGuardian(store, DefaultGuardianConfig())
	guardian.Initialize()

	observer := NewCampaignObserver(guardian)
	observer.StartCampaign(context.Background(), "campaign-1", "Build feature")

	check, err := observer.OnPhaseStart(context.Background(), "planning", "Define requirements")
	if err != nil {
		t.Fatalf("OnPhaseStart error: %v", err)
	}

	// Check may be nil if no vision
	_ = check

	observer.mu.RLock()
	phase := observer.currentPhase
	observer.mu.RUnlock()

	if phase != "planning" {
		t.Errorf("currentPhase not set: got %q", phase)
	}
}

func TestCampaignObserver_OnPhaseComplete(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	guardian := NewGuardian(store, DefaultGuardianConfig())
	guardian.Initialize()

	observer := NewCampaignObserver(guardian)
	observer.StartCampaign(context.Background(), "campaign-1", "Build feature")
	observer.OnPhaseStart(context.Background(), "planning", "Define requirements")

	err := observer.OnPhaseComplete(context.Background(), "planning", true, "Requirements defined")
	if err != nil {
		t.Fatalf("OnPhaseComplete error: %v", err)
	}
}

func TestCampaignObserver_OnTaskComplete(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	guardian := NewGuardian(store, DefaultGuardianConfig())
	guardian.Initialize()

	observer := NewCampaignObserver(guardian)
	observer.StartCampaign(context.Background(), "campaign-1", "Build feature")
	observer.OnPhaseStart(context.Background(), "implementation", "Write code")

	check, err := observer.OnTaskComplete(context.Background(), "task-1", "Implement function", "success", []string{"file.go"})
	if err != nil {
		t.Fatalf("OnTaskComplete error: %v", err)
	}

	// Check may be nil without vision/enough tasks
	_ = check

	observer.mu.RLock()
	tasks := observer.tasksInPhase
	observer.mu.RUnlock()

	if tasks != 1 {
		t.Errorf("expected 1 task, got %d", tasks)
	}
}

func TestCampaignObserver_EndCampaign(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	guardian := NewGuardian(store, DefaultGuardianConfig())
	guardian.Initialize()

	observer := NewCampaignObserver(guardian)
	observer.StartCampaign(context.Background(), "campaign-1", "Build feature")

	err := observer.EndCampaign(context.Background(), true, "Campaign completed successfully")
	if err != nil {
		t.Fatalf("EndCampaign error: %v", err)
	}
}

func TestCampaignObserver_GetPhaseCheck(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	guardian := NewGuardian(store, DefaultGuardianConfig())
	guardian.Initialize()

	// Add vision so checks happen
	vision := &Vision{Mission: "Test", Problem: "Test", VisionStmt: "Test"}
	guardian.UpdateVision(vision)

	observer := NewCampaignObserver(guardian)
	observer.StartCampaign(context.Background(), "campaign-1", "Build feature")
	observer.OnPhaseStart(context.Background(), "planning", "Define requirements")

	check := observer.GetPhaseCheck("planning")
	// Check may be nil if LLM is not set
	_ = check

	// Verify non-existent phase returns nil
	nilCheck := observer.GetPhaseCheck("nonexistent")
	if nilCheck != nil {
		t.Error("expected nil for nonexistent phase")
	}
}

func TestCampaignObserver_GetAllPhaseChecks(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	guardian := NewGuardian(store, DefaultGuardianConfig())
	guardian.Initialize()

	vision := &Vision{Mission: "Test", Problem: "Test", VisionStmt: "Test"}
	guardian.UpdateVision(vision)

	observer := NewCampaignObserver(guardian)
	observer.StartCampaign(context.Background(), "campaign-1", "Build feature")
	observer.OnPhaseStart(context.Background(), "phase1", "Goal 1")
	observer.OnPhaseStart(context.Background(), "phase2", "Goal 2")

	checks := observer.GetAllPhaseChecks()
	if checks == nil {
		t.Error("expected non-nil map")
	}
}

// =============================================================================
// TASK OBSERVER TESTS
// =============================================================================

func TestNewTaskObserver(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	guardian := NewGuardian(store, DefaultGuardianConfig())
	guardian.Initialize()

	observer := NewTaskObserver(guardian, "session-123")

	if observer == nil {
		t.Fatal("expected non-nil observer")
	}
	if observer.sessionID != "session-123" {
		t.Errorf("sessionID not set: got %q", observer.sessionID)
	}
}

func TestTaskObserver_OnTaskStart(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	guardian := NewGuardian(store, DefaultGuardianConfig())
	guardian.Initialize()

	observer := NewTaskObserver(guardian, "session-1")

	// Should not panic
	observer.OnTaskStart("code-edit", "Edit file.go")
}

func TestTaskObserver_OnTaskComplete(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	guardian := NewGuardian(store, DefaultGuardianConfig())
	guardian.Initialize()

	observer := NewTaskObserver(guardian, "session-1")
	observer.OnTaskStart("code-edit", "Edit file.go")

	check, err := observer.OnTaskComplete(
		context.Background(),
		"code-edit",
		"Edit file.go",
		"success",
		[]string{"file.go"},
	)
	if err != nil {
		t.Fatalf("OnTaskComplete error: %v", err)
	}

	// Check may be nil without vision
	_ = check

	// Verify observation was recorded
	obs, _ := store.GetRecentObservations(1)
	if len(obs) != 1 {
		t.Errorf("expected 1 observation, got %d", len(obs))
	}
}

func TestTaskObserver_OnError(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	guardian := NewGuardian(store, DefaultGuardianConfig())
	guardian.Initialize()

	observer := NewTaskObserver(guardian, "session-1")

	// Should not panic
	observer.OnError("code-edit", "Edit file.go", "file not found")
}

// =============================================================================
// BACKGROUND EVENT HANDLER TESTS
// =============================================================================

func TestNewBackgroundEventHandler(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	guardian := NewGuardian(store, DefaultGuardianConfig())
	guardian.Initialize()

	handler := NewBackgroundEventHandler(guardian, "session-1")

	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
	if handler.sessionID != "session-1" {
		t.Errorf("sessionID not set: got %q", handler.sessionID)
	}
}

func TestBackgroundEventHandler_HandleEvent(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	guardian := NewGuardian(store, DefaultGuardianConfig())
	guardian.Initialize()

	// Add vision so handler doesn't return nil
	vision := &Vision{Mission: "Test", Problem: "Test", VisionStmt: "Test"}
	guardian.UpdateVision(vision)

	handler := NewBackgroundEventHandler(guardian, "session-1")

	assessment, err := handler.HandleEvent(
		context.Background(),
		"task_completed",
		"executor",
		"file.go",
		map[string]string{"result": "success"},
		time.Now(),
	)
	if err != nil {
		t.Fatalf("HandleEvent error: %v", err)
	}

	if assessment == nil {
		// Without LLM, guardian returns a default check
		return
	}

	if assessment.ObserverName != "northstar" {
		t.Errorf("expected observer name 'northstar', got %q", assessment.ObserverName)
	}
}

func TestBackgroundEventHandler_BuildSubject(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	guardian := NewGuardian(store, DefaultGuardianConfig())
	handler := NewBackgroundEventHandler(guardian, "session-1")

	subject := handler.buildSubject("task_completed", "executor", "file.go", nil)
	if subject == "" {
		t.Error("expected non-empty subject")
	}
}

func TestBackgroundEventHandler_BuildEventContext(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	guardian := NewGuardian(store, DefaultGuardianConfig())
	handler := NewBackgroundEventHandler(guardian, "session-1")

	ctx := handler.buildEventContext("task_completed", "executor", "file.go", map[string]string{"key": "value"})
	if ctx == "" {
		t.Error("expected non-empty context")
	}
}

func TestBackgroundEventHandler_ResultToLevel(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	guardian := NewGuardian(store, DefaultGuardianConfig())
	handler := NewBackgroundEventHandler(guardian, "session-1")

	cases := []struct {
		result   AlignmentResult
		expected string
	}{
		{AlignmentPassed, "proceed"},
		{AlignmentWarning, "note"},
		{AlignmentFailed, "clarify"},
		{AlignmentBlocked, "block"},
		{AlignmentSkipped, "note"},
	}

	for _, tc := range cases {
		got := handler.resultToLevel(tc.result)
		if got != tc.expected {
			t.Errorf("resultToLevel(%s) = %q, want %q", tc.result, got, tc.expected)
		}
	}
}

// =============================================================================
// OBSERVER ASSESSMENT STRUCT TESTS
// =============================================================================

func TestObserverAssessment_Fields(t *testing.T) {
	t.Parallel()

	assessment := ObserverAssessment{
		ObserverName: "test",
		EventID:      "event-1",
		Score:        85,
		Level:        "info",
		VisionMatch:  "aligned",
		Deviations:   []string{"minor issue"},
		Suggestions:  []string{"fix it"},
		Metadata:     map[string]string{"key": "value"},
	}

	// Verify all fields are properly set
	if assessment.ObserverName != "test" {
		t.Errorf("ObserverName mismatch: got %q", assessment.ObserverName)
	}
	if assessment.EventID != "event-1" {
		t.Errorf("EventID mismatch: got %q", assessment.EventID)
	}
	if assessment.Score != 85 {
		t.Errorf("Score mismatch: got %d", assessment.Score)
	}
	if assessment.Level != "info" {
		t.Errorf("Level mismatch: got %q", assessment.Level)
	}
	if assessment.VisionMatch != "aligned" {
		t.Errorf("VisionMatch mismatch: got %q", assessment.VisionMatch)
	}
	if len(assessment.Deviations) != 1 || assessment.Deviations[0] != "minor issue" {
		t.Errorf("Deviations mismatch: got %v", assessment.Deviations)
	}
	if len(assessment.Suggestions) != 1 || assessment.Suggestions[0] != "fix it" {
		t.Errorf("Suggestions mismatch: got %v", assessment.Suggestions)
	}
	if assessment.Metadata["key"] != "value" {
		t.Errorf("Metadata mismatch: got %v", assessment.Metadata)
	}
}

// =============================================================================
// OBSERVER EVENT STRUCT TESTS
// =============================================================================

func TestObserverEvent_Fields(t *testing.T) {
	t.Parallel()

	now := time.Now()
	event := ObserverEvent{
		Type:      "task_completed",
		Source:    "executor",
		Target:    "file.go",
		Details:   map[string]string{"result": "success"},
		Timestamp: now,
	}

	// Verify all fields are properly set
	if event.Type != "task_completed" {
		t.Errorf("Type mismatch: got %q", event.Type)
	}
	if event.Source != "executor" {
		t.Errorf("Source mismatch: got %q", event.Source)
	}
	if event.Target != "file.go" {
		t.Errorf("Target mismatch: got %q", event.Target)
	}
	if event.Details["result"] != "success" {
		t.Errorf("Details mismatch: got %v", event.Details)
	}
	if event.Timestamp != now {
		t.Errorf("Timestamp mismatch: got %v, want %v", event.Timestamp, now)
	}
}
