package northstar

import (
	"testing"
	"time"
)

// =============================================================================
// STORE CREATION AND LIFECYCLE TESTS
// =============================================================================

func TestNewStore(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	if store == nil {
		t.Fatal("expected non-nil store")
	}

	path := store.Path()
	if path == "" {
		t.Error("expected non-empty path")
	}
}

func TestStore_Path(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	path := store.Path()
	if path == "" {
		t.Error("Path should not be empty")
	}
}

func TestStore_Close(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	if err := store.Close(); err != nil {
		t.Errorf("Close error: %v", err)
	}
}

// =============================================================================
// VISION OPERATION TESTS
// =============================================================================

func TestStore_VisionRoundTrip(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	vision := &Vision{
		Mission:    "Test mission",
		Problem:    "Test problem",
		VisionStmt: "Test vision statement",
		Personas: []Persona{
			{Name: "Developer", PainPoints: []string{"complexity"}, Needs: []string{"simplicity"}},
		},
		Capabilities: []Capability{
			{ID: "cap-1", Description: "Core feature", Timeline: "now", Priority: "critical"},
		},
		Risks: []Risk{
			{ID: "risk-1", Description: "Technical debt", Likelihood: "medium", Impact: "high", Mitigation: "Regular refactoring"},
		},
		Requirements: []Requirement{
			{ID: "req-1", Type: "functional", Description: "Must do X", Priority: "must_have"},
		},
		Constraints: []string{"No external dependencies"},
	}

	if err := store.SaveVision(vision); err != nil {
		t.Fatalf("SaveVision error: %v", err)
	}

	loaded, err := store.LoadVision()
	if err != nil {
		t.Fatalf("LoadVision error: %v", err)
	}

	if loaded == nil {
		t.Fatal("expected non-nil vision")
	}
	if loaded.Mission != vision.Mission {
		t.Errorf("Mission mismatch: got %q, want %q", loaded.Mission, vision.Mission)
	}
	if loaded.Problem != vision.Problem {
		t.Errorf("Problem mismatch: got %q, want %q", loaded.Problem, vision.Problem)
	}
	if len(loaded.Personas) != len(vision.Personas) {
		t.Errorf("Personas length mismatch: got %d, want %d", len(loaded.Personas), len(vision.Personas))
	}
	if len(loaded.Capabilities) != len(vision.Capabilities) {
		t.Errorf("Capabilities length mismatch: got %d, want %d", len(loaded.Capabilities), len(vision.Capabilities))
	}
}

func TestStore_HasVision_False(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	if store.HasVision() {
		t.Error("expected HasVision to return false when no vision saved")
	}
}

func TestStore_HasVision_True(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	vision := &Vision{
		Mission:    "Test",
		Problem:    "Test",
		VisionStmt: "Test",
	}
	if err := store.SaveVision(vision); err != nil {
		t.Fatalf("SaveVision error: %v", err)
	}

	if !store.HasVision() {
		t.Error("expected HasVision to return true after saving vision")
	}
}

func TestStore_LoadVision_NoVision(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	vision, err := store.LoadVision()
	if err != nil {
		t.Fatalf("LoadVision error: %v", err)
	}

	if vision != nil {
		t.Error("expected nil vision when none saved")
	}
}

func TestStore_SaveVision_UpdatesTimestamps(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	vision := &Vision{
		Mission:    "Test",
		Problem:    "Test",
		VisionStmt: "Test",
	}

	if err := store.SaveVision(vision); err != nil {
		t.Fatalf("SaveVision error: %v", err)
	}

	if vision.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if vision.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set")
	}
}

func TestStore_SaveVision_UpdateExisting(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	// Save initial vision
	vision1 := &Vision{
		Mission:    "Original mission",
		Problem:    "Original problem",
		VisionStmt: "Original statement",
	}
	if err := store.SaveVision(vision1); err != nil {
		t.Fatalf("first SaveVision error: %v", err)
	}

	// Update vision
	vision2 := &Vision{
		Mission:    "Updated mission",
		Problem:    "Updated problem",
		VisionStmt: "Updated statement",
	}
	if err := store.SaveVision(vision2); err != nil {
		t.Fatalf("second SaveVision error: %v", err)
	}

	// Load and verify update
	loaded, err := store.LoadVision()
	if err != nil {
		t.Fatalf("LoadVision error: %v", err)
	}

	if loaded.Mission != "Updated mission" {
		t.Errorf("Mission not updated: got %q", loaded.Mission)
	}
}

// =============================================================================
// OBSERVATION OPERATION TESTS
// =============================================================================

func TestStore_RecordObservation(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	obs := &Observation{
		SessionID: "session-1",
		Type:      ObsTaskCompleted,
		Subject:   "Test task",
		Content:   "Task completed successfully",
		Relevance: 0.8,
		Tags:      []string{"test"},
		Metadata:  map[string]string{"key": "value"},
	}

	if err := store.RecordObservation(obs); err != nil {
		t.Fatalf("RecordObservation error: %v", err)
	}

	if obs.ID == "" {
		t.Error("expected ID to be assigned")
	}
	if obs.Timestamp.IsZero() {
		t.Error("expected Timestamp to be assigned")
	}
}

func TestStore_GetRecentObservations(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	// Record multiple observations
	for i := 0; i < 5; i++ {
		obs := &Observation{
			SessionID: "session-1",
			Type:      ObsTaskCompleted,
			Subject:   "Task",
			Content:   "Content",
			Relevance: 0.5,
		}
		if err := store.RecordObservation(obs); err != nil {
			t.Fatalf("RecordObservation error: %v", err)
		}
		time.Sleep(time.Millisecond) // Ensure different timestamps
	}

	// Get with limit
	observations, err := store.GetRecentObservations(3)
	if err != nil {
		t.Fatalf("GetRecentObservations error: %v", err)
	}

	if len(observations) != 3 {
		t.Errorf("expected 3 observations, got %d", len(observations))
	}
}

func TestStore_GetRecentObservations_Empty(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	observations, err := store.GetRecentObservations(10)
	if err != nil {
		t.Fatalf("GetRecentObservations error: %v", err)
	}

	if observations == nil {
		// nil is okay for empty
	} else if len(observations) != 0 {
		t.Errorf("expected 0 observations, got %d", len(observations))
	}
}

// =============================================================================
// ALIGNMENT CHECK OPERATION TESTS
// =============================================================================

func TestStore_RecordAlignmentCheck(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	check := &AlignmentCheck{
		Trigger:     TriggerManual,
		Subject:     "Test subject",
		Context:     "Test context",
		Result:      AlignmentPassed,
		Score:       0.95,
		Explanation: "All aligned",
		Suggestions: []string{"Keep going"},
		Duration:    5 * time.Second,
	}

	if err := store.RecordAlignmentCheck(check); err != nil {
		t.Fatalf("RecordAlignmentCheck error: %v", err)
	}

	if check.ID == "" {
		t.Error("expected ID to be assigned")
	}
	if check.Timestamp.IsZero() {
		t.Error("expected Timestamp to be assigned")
	}
}

func TestStore_GetAlignmentHistory(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	// Record multiple checks
	for i := 0; i < 5; i++ {
		check := &AlignmentCheck{
			Trigger: TriggerPeriodic,
			Subject: "Periodic check",
			Result:  AlignmentPassed,
			Score:   0.9,
		}
		if err := store.RecordAlignmentCheck(check); err != nil {
			t.Fatalf("RecordAlignmentCheck error: %v", err)
		}
		time.Sleep(time.Millisecond)
	}

	history, err := store.GetAlignmentHistory(3)
	if err != nil {
		t.Fatalf("GetAlignmentHistory error: %v", err)
	}

	if len(history) != 3 {
		t.Errorf("expected 3 checks, got %d", len(history))
	}
}

func TestStore_GetAlignmentHistory_Empty(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	history, err := store.GetAlignmentHistory(10)
	if err != nil {
		t.Fatalf("GetAlignmentHistory error: %v", err)
	}

	if history != nil && len(history) != 0 {
		t.Errorf("expected 0 checks, got %d", len(history))
	}
}

// =============================================================================
// DRIFT EVENT OPERATION TESTS
// =============================================================================

func TestStore_RecordDriftEvent(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	drift := &DriftEvent{
		Severity:    DriftMajor,
		Category:    "architecture",
		Description: "Test drift",
		Evidence:    []string{"evidence-1"},
	}

	if err := store.RecordDriftEvent(drift); err != nil {
		t.Fatalf("RecordDriftEvent error: %v", err)
	}

	if drift.ID == "" {
		t.Error("expected ID to be assigned")
	}
	if drift.Timestamp.IsZero() {
		t.Error("expected Timestamp to be assigned")
	}
}

func TestStore_GetActiveDriftEvents(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	// Record drift event
	drift := &DriftEvent{
		Severity:    DriftMinor,
		Category:    "style",
		Description: "Minor drift",
	}
	if err := store.RecordDriftEvent(drift); err != nil {
		t.Fatalf("RecordDriftEvent error: %v", err)
	}

	events, err := store.GetActiveDriftEvents()
	if err != nil {
		t.Fatalf("GetActiveDriftEvents error: %v", err)
	}

	if len(events) != 1 {
		t.Errorf("expected 1 active drift event, got %d", len(events))
	}
}

func TestStore_ResolveDriftEvent(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	// Record drift event
	drift := &DriftEvent{
		Severity:    DriftMinor,
		Category:    "style",
		Description: "Minor drift",
	}
	if err := store.RecordDriftEvent(drift); err != nil {
		t.Fatalf("RecordDriftEvent error: %v", err)
	}

	// Resolve it
	if err := store.ResolveDriftEvent(drift.ID, "Fixed the issue"); err != nil {
		t.Fatalf("ResolveDriftEvent error: %v", err)
	}

	// Verify no more active events
	events, err := store.GetActiveDriftEvents()
	if err != nil {
		t.Fatalf("GetActiveDriftEvents error: %v", err)
	}

	if len(events) != 0 {
		t.Errorf("expected 0 active events after resolution, got %d", len(events))
	}
}

func TestStore_ResolveDriftEvent_NonExistent(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	// Should not error for non-existent ID
	if err := store.ResolveDriftEvent("non-existent-id", "resolution"); err != nil {
		t.Errorf("ResolveDriftEvent on non-existent ID should not error: %v", err)
	}
}

// =============================================================================
// GUARDIAN STATE OPERATION TESTS
// =============================================================================

func TestStore_GetState_Initial(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	state, err := store.GetState()
	if err != nil {
		t.Fatalf("GetState error: %v", err)
	}

	if state == nil {
		t.Fatal("expected non-nil state")
	}
	if state.VisionDefined {
		t.Error("VisionDefined should be false initially")
	}
	if state.TasksSinceCheck != 0 {
		t.Errorf("TasksSinceCheck should be 0, got %d", state.TasksSinceCheck)
	}
	if state.ActiveDriftCount != 0 {
		t.Errorf("ActiveDriftCount should be 0, got %d", state.ActiveDriftCount)
	}
	if state.OverallAlignment != 1.0 {
		t.Errorf("OverallAlignment should be 1.0, got %f", state.OverallAlignment)
	}
}

func TestStore_IncrementTaskCount(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	count1, err := store.IncrementTaskCount()
	if err != nil {
		t.Fatalf("first IncrementTaskCount error: %v", err)
	}
	if count1 != 1 {
		t.Errorf("expected count 1, got %d", count1)
	}

	count2, err := store.IncrementTaskCount()
	if err != nil {
		t.Fatalf("second IncrementTaskCount error: %v", err)
	}
	if count2 != 2 {
		t.Errorf("expected count 2, got %d", count2)
	}
}

func TestStore_ResetSessionObservations(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	// Record an observation (increments session count)
	obs := &Observation{
		SessionID: "session-1",
		Type:      ObsTaskCompleted,
		Subject:   "Test",
		Content:   "Content",
		Relevance: 0.5,
	}
	if err := store.RecordObservation(obs); err != nil {
		t.Fatalf("RecordObservation error: %v", err)
	}

	// Verify count increased
	state, _ := store.GetState()
	if state.SessionObservations != 1 {
		t.Errorf("expected 1 observation, got %d", state.SessionObservations)
	}

	// Reset
	if err := store.ResetSessionObservations(); err != nil {
		t.Fatalf("ResetSessionObservations error: %v", err)
	}

	// Verify reset
	state, _ = store.GetState()
	if state.SessionObservations != 0 {
		t.Errorf("expected 0 observations after reset, got %d", state.SessionObservations)
	}
}

func TestStore_SaveVision_UpdatesGuardianState(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	// Initial state
	state, _ := store.GetState()
	if state.VisionDefined {
		t.Error("VisionDefined should be false initially")
	}

	// Save vision
	vision := &Vision{
		Mission:    "Test",
		Problem:    "Test",
		VisionStmt: "Test",
	}
	if err := store.SaveVision(vision); err != nil {
		t.Fatalf("SaveVision error: %v", err)
	}

	// Check state updated
	state, _ = store.GetState()
	if !state.VisionDefined {
		t.Error("VisionDefined should be true after saving vision")
	}
}

func TestStore_RecordAlignmentCheck_UpdatesGuardianState(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	// Increment task count
	store.IncrementTaskCount()
	store.IncrementTaskCount()

	state, _ := store.GetState()
	if state.TasksSinceCheck != 2 {
		t.Errorf("expected 2 tasks, got %d", state.TasksSinceCheck)
	}

	// Record check (should reset counter)
	check := &AlignmentCheck{
		Trigger: TriggerManual,
		Subject: "Test",
		Result:  AlignmentPassed,
		Score:   0.9,
	}
	if err := store.RecordAlignmentCheck(check); err != nil {
		t.Fatalf("RecordAlignmentCheck error: %v", err)
	}

	state, _ = store.GetState()
	if state.TasksSinceCheck != 0 {
		t.Errorf("expected 0 tasks after check, got %d", state.TasksSinceCheck)
	}
	if state.LastCheck.IsZero() {
		t.Error("LastCheck should be set after alignment check")
	}
}

func TestStore_RecordDriftEvent_UpdatesGuardianState(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	// Initial state
	state, _ := store.GetState()
	if state.ActiveDriftCount != 0 {
		t.Errorf("expected 0 drifts initially, got %d", state.ActiveDriftCount)
	}

	// Record drift
	drift := &DriftEvent{
		Severity:    DriftMinor,
		Category:    "test",
		Description: "Test drift",
	}
	if err := store.RecordDriftEvent(drift); err != nil {
		t.Fatalf("RecordDriftEvent error: %v", err)
	}

	state, _ = store.GetState()
	if state.ActiveDriftCount != 1 {
		t.Errorf("expected 1 drift, got %d", state.ActiveDriftCount)
	}

	// Resolve drift
	if err := store.ResolveDriftEvent(drift.ID, "fixed"); err != nil {
		t.Fatalf("ResolveDriftEvent error: %v", err)
	}

	state, _ = store.GetState()
	if state.ActiveDriftCount != 0 {
		t.Errorf("expected 0 drifts after resolution, got %d", state.ActiveDriftCount)
	}
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

func newTestStore(t *testing.T) *Store {
	t.Helper()

	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	return store
}
