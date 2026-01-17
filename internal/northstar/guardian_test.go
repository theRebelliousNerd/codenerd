package northstar

import (
	"context"
	"testing"
	"time"
)

// =============================================================================
// MOCK LLM CLIENT
// =============================================================================

type mockLLMClient struct {
	response string
	err      error
}

func (m *mockLLMClient) CompleteWithSystem(_ context.Context, _, _ string) (string, error) {
	return m.response, m.err
}

// =============================================================================
// GUARDIAN CREATION TESTS
// =============================================================================

func TestNewGuardian(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	config := DefaultGuardianConfig()

	guardian := NewGuardian(store, config)

	if guardian == nil {
		t.Fatal("expected non-nil guardian")
	}
	if guardian.store != store {
		t.Error("store not set correctly")
	}
}

func TestGuardian_SetLLMClient(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	guardian := NewGuardian(store, DefaultGuardianConfig())

	client := &mockLLMClient{response: "test"}
	guardian.SetLLMClient(client)

	// Verify through behavior - no direct access to llm field
	if guardian.llm != client {
		t.Error("LLM client not set")
	}
}

// =============================================================================
// INITIALIZATION TESTS
// =============================================================================

func TestGuardian_Initialize_NoVision(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	guardian := NewGuardian(store, DefaultGuardianConfig())

	if err := guardian.Initialize(); err != nil {
		t.Fatalf("Initialize error: %v", err)
	}

	if guardian.HasVision() {
		t.Error("expected HasVision to be false")
	}
	if guardian.GetState() == nil {
		t.Error("expected state to be loaded")
	}
}

func TestGuardian_Initialize_WithVision(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	// Save vision first
	vision := &Vision{
		Mission:    "Test mission",
		Problem:    "Test problem",
		VisionStmt: "Test vision",
	}
	if err := store.SaveVision(vision); err != nil {
		t.Fatalf("SaveVision error: %v", err)
	}

	guardian := NewGuardian(store, DefaultGuardianConfig())
	if err := guardian.Initialize(); err != nil {
		t.Fatalf("Initialize error: %v", err)
	}

	if !guardian.HasVision() {
		t.Error("expected HasVision to be true")
	}
	if guardian.GetVision() == nil {
		t.Error("expected vision to be loaded")
	}
	if guardian.GetVision().Mission != "Test mission" {
		t.Errorf("mission mismatch: got %q", guardian.GetVision().Mission)
	}
}

// =============================================================================
// VISION ACCESS TESTS
// =============================================================================

func TestGuardian_HasVision_False(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	guardian := NewGuardian(store, DefaultGuardianConfig())
	guardian.Initialize()

	if guardian.HasVision() {
		t.Error("expected HasVision to return false")
	}
}

func TestGuardian_GetVision_Nil(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	guardian := NewGuardian(store, DefaultGuardianConfig())
	guardian.Initialize()

	vision := guardian.GetVision()
	if vision != nil {
		t.Error("expected nil vision")
	}
}

func TestGuardian_GetState(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	guardian := NewGuardian(store, DefaultGuardianConfig())
	guardian.Initialize()

	state := guardian.GetState()
	if state == nil {
		t.Fatal("expected non-nil state")
	}
	if state.VisionDefined {
		t.Error("VisionDefined should be false")
	}
}

// =============================================================================
// UPDATE VISION TESTS
// =============================================================================

func TestGuardian_UpdateVision(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	guardian := NewGuardian(store, DefaultGuardianConfig())
	guardian.Initialize()

	vision := &Vision{
		Mission:    "New mission",
		Problem:    "New problem",
		VisionStmt: "New vision",
	}

	if err := guardian.UpdateVision(vision); err != nil {
		t.Fatalf("UpdateVision error: %v", err)
	}

	if !guardian.HasVision() {
		t.Error("expected HasVision to return true after update")
	}

	loaded := guardian.GetVision()
	if loaded == nil {
		t.Fatal("expected non-nil vision")
	}
	if loaded.Mission != "New mission" {
		t.Errorf("mission not updated: got %q", loaded.Mission)
	}
}

// =============================================================================
// ALIGNMENT CHECK TESTS
// =============================================================================

func TestGuardian_CheckAlignment_NoVision(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	guardian := NewGuardian(store, DefaultGuardianConfig())
	guardian.Initialize()

	check, err := guardian.CheckAlignment(context.Background(), TriggerManual, "test subject", "test context")
	if err != nil {
		t.Fatalf("CheckAlignment error: %v", err)
	}

	if check == nil {
		t.Fatal("expected non-nil check")
	}
	if check.Result != AlignmentSkipped {
		t.Errorf("expected Skipped result, got %s", check.Result)
	}
	if check.Score != 1.0 {
		t.Errorf("expected score 1.0, got %f", check.Score)
	}
}

func TestGuardian_CheckAlignment_NoLLM(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	guardian := NewGuardian(store, DefaultGuardianConfig())
	guardian.Initialize()

	// Set vision but no LLM
	vision := &Vision{
		Mission:    "Test",
		Problem:    "Test",
		VisionStmt: "Test",
	}
	guardian.UpdateVision(vision)

	check, err := guardian.CheckAlignment(context.Background(), TriggerManual, "test subject", "")
	if err != nil {
		t.Fatalf("CheckAlignment error: %v", err)
	}

	if check.Result != AlignmentPassed {
		t.Errorf("expected Passed result without LLM, got %s", check.Result)
	}
	if check.Score != 0.8 {
		t.Errorf("expected score 0.8 without LLM, got %f", check.Score)
	}
}

func TestGuardian_CheckAlignment_WithLLM(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	guardian := NewGuardian(store, DefaultGuardianConfig())
	guardian.Initialize()

	vision := &Vision{
		Mission:    "Build the best app",
		Problem:    "Users need help",
		VisionStmt: "A great solution",
	}
	guardian.UpdateVision(vision)

	// Mock LLM response
	mockLLM := &mockLLMClient{
		response: "SCORE: 0.95\nRESULT: passed\nEXPLANATION: Fully aligned\nSUGGESTIONS: none",
	}
	guardian.SetLLMClient(mockLLM)

	check, err := guardian.CheckAlignment(context.Background(), TriggerManual, "Add user feature", "New feature")
	if err != nil {
		t.Fatalf("CheckAlignment error: %v", err)
	}

	if check.Result != AlignmentPassed {
		t.Errorf("expected Passed, got %s", check.Result)
	}
	if check.Score != 0.95 {
		t.Errorf("expected score 0.95, got %f", check.Score)
	}
}

func TestGuardian_CheckAlignment_Warning(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	config := DefaultGuardianConfig()
	guardian := NewGuardian(store, config)
	guardian.Initialize()

	vision := &Vision{
		Mission:    "Test",
		Problem:    "Test",
		VisionStmt: "Test",
	}
	guardian.UpdateVision(vision)

	mockLLM := &mockLLMClient{
		response: "SCORE: 0.65\nRESULT: warning\nEXPLANATION: Minor drift detected\nSUGGESTIONS: Review approach",
	}
	guardian.SetLLMClient(mockLLM)

	check, err := guardian.CheckAlignment(context.Background(), TriggerManual, "Test", "")
	if err != nil {
		t.Fatalf("CheckAlignment error: %v", err)
	}

	if check.Result != AlignmentWarning {
		t.Errorf("expected Warning, got %s", check.Result)
	}
}

func TestGuardian_CheckAlignment_Failed(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	config := DefaultGuardianConfig()
	guardian := NewGuardian(store, config)
	guardian.Initialize()

	vision := &Vision{
		Mission:    "Test",
		Problem:    "Test",
		VisionStmt: "Test",
	}
	guardian.UpdateVision(vision)

	mockLLM := &mockLLMClient{
		response: "SCORE: 0.4\nRESULT: failed\nEXPLANATION: Significant drift\nSUGGESTIONS: Change approach",
	}
	guardian.SetLLMClient(mockLLM)

	check, err := guardian.CheckAlignment(context.Background(), TriggerManual, "Test", "")
	if err != nil {
		t.Fatalf("CheckAlignment error: %v", err)
	}

	// Score 0.4 is between failure (0.5) and block (0.3) thresholds
	if check.Result != AlignmentFailed {
		t.Errorf("expected Failed, got %s", check.Result)
	}
}

// =============================================================================
// OBSERVATION TESTS
// =============================================================================

func TestGuardian_ObserveTaskCompletion(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	guardian := NewGuardian(store, DefaultGuardianConfig())
	guardian.Initialize()

	err := guardian.ObserveTaskCompletion("session-1", "code-edit", "Edit file", "Success")
	if err != nil {
		t.Fatalf("ObserveTaskCompletion error: %v", err)
	}

	obs, _ := store.GetRecentObservations(1)
	if len(obs) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(obs))
	}
	if obs[0].Type != ObsTaskCompleted {
		t.Errorf("expected task_completed type, got %s", obs[0].Type)
	}
}

func TestGuardian_ObserveFileChange(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	guardian := NewGuardian(store, DefaultGuardianConfig())
	guardian.Initialize()

	err := guardian.ObserveFileChange("session-1", "internal/core/main.go", "modified")
	if err != nil {
		t.Fatalf("ObserveFileChange error: %v", err)
	}

	obs, _ := store.GetRecentObservations(1)
	if len(obs) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(obs))
	}
	if obs[0].Type != ObsFileChanged {
		t.Errorf("expected file_changed type, got %s", obs[0].Type)
	}
}

func TestGuardian_ObserveDecision(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	guardian := NewGuardian(store, DefaultGuardianConfig())
	guardian.Initialize()

	err := guardian.ObserveDecision("session-1", "Use SQLite", "Simple and portable")
	if err != nil {
		t.Fatalf("ObserveDecision error: %v", err)
	}

	obs, _ := store.GetRecentObservations(1)
	if len(obs) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(obs))
	}
	if obs[0].Type != ObsDecisionMade {
		t.Errorf("expected decision_made type, got %s", obs[0].Type)
	}
	if obs[0].Relevance != 0.8 {
		t.Errorf("expected decision relevance 0.8, got %f", obs[0].Relevance)
	}
}

// =============================================================================
// SHOULD CHECK NOW TESTS
// =============================================================================

func TestGuardian_ShouldCheckNow_NoVision(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	guardian := NewGuardian(store, DefaultGuardianConfig())
	guardian.Initialize()

	if guardian.ShouldCheckNow(TriggerManual, nil) {
		t.Error("should not check without vision")
	}
}

func TestGuardian_ShouldCheckNow_Manual(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	guardian := NewGuardian(store, DefaultGuardianConfig())
	guardian.Initialize()

	vision := &Vision{Mission: "Test", Problem: "Test", VisionStmt: "Test"}
	guardian.UpdateVision(vision)

	if !guardian.ShouldCheckNow(TriggerManual, nil) {
		t.Error("manual trigger should always check with vision")
	}
}

func TestGuardian_ShouldCheckNow_PhaseGate(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	config := DefaultGuardianConfig()
	config.EnablePhaseGates = true
	guardian := NewGuardian(store, config)
	guardian.Initialize()

	vision := &Vision{Mission: "Test", Problem: "Test", VisionStmt: "Test"}
	guardian.UpdateVision(vision)

	if !guardian.ShouldCheckNow(TriggerPhaseGate, nil) {
		t.Error("phase gate should trigger when enabled")
	}
}

func TestGuardian_ShouldCheckNow_PhaseGate_Disabled(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	config := DefaultGuardianConfig()
	config.EnablePhaseGates = false
	guardian := NewGuardian(store, config)
	guardian.Initialize()

	vision := &Vision{Mission: "Test", Problem: "Test", VisionStmt: "Test"}
	guardian.UpdateVision(vision)

	if guardian.ShouldCheckNow(TriggerPhaseGate, nil) {
		t.Error("phase gate should not trigger when disabled")
	}
}

func TestGuardian_ShouldCheckNow_Periodic(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	config := DefaultGuardianConfig()
	config.PeriodicCheckInterval = 3
	guardian := NewGuardian(store, config)
	guardian.Initialize()

	vision := &Vision{Mission: "Test", Problem: "Test", VisionStmt: "Test"}
	guardian.UpdateVision(vision)

	// Should not trigger initially
	if guardian.ShouldCheckNow(TriggerPeriodic, nil) {
		t.Error("periodic should not trigger with 0 tasks")
	}

	// Increment tasks
	for i := 0; i < 3; i++ {
		store.IncrementTaskCount()
	}
	guardian.state.TasksSinceCheck = 3

	if !guardian.ShouldCheckNow(TriggerPeriodic, nil) {
		t.Error("periodic should trigger after reaching interval")
	}
}

func TestGuardian_ShouldCheckNow_HighImpact(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	config := DefaultGuardianConfig()
	config.EnableHighImpact = true
	config.HighImpactPaths = []string{"internal/core/"}
	guardian := NewGuardian(store, config)
	guardian.Initialize()

	vision := &Vision{Mission: "Test", Problem: "Test", VisionStmt: "Test"}
	guardian.UpdateVision(vision)

	// Non-high-impact path
	if guardian.ShouldCheckNow(TriggerHighImpact, []string{"internal/utils/helper.go"}) {
		t.Error("should not trigger for non-high-impact path")
	}

	// High-impact path
	if !guardian.ShouldCheckNow(TriggerHighImpact, []string{"internal/core/main.go"}) {
		t.Error("should trigger for high-impact path")
	}
}

// =============================================================================
// ON TASK COMPLETE TESTS
// =============================================================================

func TestGuardian_OnTaskComplete(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	config := DefaultGuardianConfig()
	config.PeriodicCheckInterval = 100 // High so it doesn't trigger
	guardian := NewGuardian(store, config)
	guardian.Initialize()

	check, err := guardian.OnTaskComplete(context.Background(), "Some task")
	if err != nil {
		t.Fatalf("OnTaskComplete error: %v", err)
	}

	// No check should be returned (not enough tasks)
	if check != nil {
		t.Error("expected nil check when not enough tasks")
	}

	// Verify task count incremented
	state, _ := store.GetState()
	if state.TasksSinceCheck != 1 {
		t.Errorf("expected 1 task, got %d", state.TasksSinceCheck)
	}
}

func TestGuardian_OnTaskComplete_TriggersCheck(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	config := DefaultGuardianConfig()
	config.PeriodicCheckInterval = 2
	guardian := NewGuardian(store, config)
	guardian.Initialize()

	vision := &Vision{Mission: "Test", Problem: "Test", VisionStmt: "Test"}
	guardian.UpdateVision(vision)

	// First task
	guardian.OnTaskComplete(context.Background(), "Task 1")

	// Second task - should trigger check
	check, err := guardian.OnTaskComplete(context.Background(), "Task 2")
	if err != nil {
		t.Fatalf("OnTaskComplete error: %v", err)
	}

	if check == nil {
		t.Error("expected check to be triggered at interval")
	}
}

// =============================================================================
// UTILITY FUNCTION TESTS
// =============================================================================

func TestTruncate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"short", "hello", 10, "hello"},
		{"exact", "hello", 5, "hello"},
		{"truncate", "hello world", 5, "hello..."},
		{"empty", "", 5, ""},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := truncate(tc.input, tc.maxLen)
			if got != tc.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tc.input, tc.maxLen, got, tc.want)
			}
		})
	}
}

func TestGuardian_ScoreToSeverity(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	guardian := NewGuardian(store, DefaultGuardianConfig())

	cases := []struct {
		score    float64
		expected DriftSeverity
	}{
		{0.9, DriftMinor},
		{0.7, DriftMinor},
		{0.6, DriftModerate},
		{0.5, DriftModerate},
		{0.4, DriftMajor},
		{0.3, DriftMajor},
		{0.2, DriftCritical},
		{0.0, DriftCritical},
	}

	for _, tc := range cases {
		got := guardian.scoreToSeverity(tc.score)
		if got != tc.expected {
			t.Errorf("scoreToSeverity(%f) = %s, want %s", tc.score, got, tc.expected)
		}
	}
}

func TestGuardian_CalculateRelevance_NoVision(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	guardian := NewGuardian(store, DefaultGuardianConfig())
	guardian.Initialize()

	relevance := guardian.calculateRelevance("any text")
	if relevance != 0.5 {
		t.Errorf("expected 0.5 without vision, got %f", relevance)
	}
}

func TestGuardian_CalculatePathRelevance_HighImpact(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	config := DefaultGuardianConfig()
	config.HighImpactPaths = []string{"internal/core/"}
	guardian := NewGuardian(store, config)

	relevance := guardian.calculatePathRelevance("internal/core/main.go")
	if relevance != 0.9 {
		t.Errorf("expected 0.9 for high impact path, got %f", relevance)
	}

	relevance = guardian.calculatePathRelevance("internal/utils/helper.go")
	if relevance != 0.5 {
		t.Errorf("expected 0.5 for normal path, got %f", relevance)
	}
}

// =============================================================================
// PARSE ALIGNMENT RESPONSE TESTS
// =============================================================================

func TestGuardian_ParseAlignmentResponse(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	guardian := NewGuardian(store, DefaultGuardianConfig())

	cases := []struct {
		name        string
		response    string
		wantScore   float64
		wantResult  AlignmentResult
		wantSuggNil bool
	}{
		{
			name:       "full_response",
			response:   "SCORE: 0.95\nRESULT: passed\nEXPLANATION: All good\nSUGGESTIONS: keep going, stay focused",
			wantScore:  0.95,
			wantResult: AlignmentPassed,
		},
		{
			name:       "warning_response",
			response:   "SCORE: 0.65\nRESULT: warning\nEXPLANATION: Minor issue\nSUGGESTIONS: none",
			wantScore:  0.65,
			wantResult: AlignmentWarning,
		},
		{
			name:        "no_suggestions",
			response:    "SCORE: 0.9\nRESULT: passed\nEXPLANATION: Fine\nSUGGESTIONS: none",
			wantScore:   0.9,
			wantResult:  AlignmentPassed,
			wantSuggNil: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			check := &AlignmentCheck{}
			guardian.parseAlignmentResponse(tc.response, check)

			if check.Score != tc.wantScore {
				t.Errorf("score: got %f, want %f", check.Score, tc.wantScore)
			}
			if check.Result != tc.wantResult {
				t.Errorf("result: got %s, want %s", check.Result, tc.wantResult)
			}
			if tc.wantSuggNil && len(check.Suggestions) > 0 {
				t.Errorf("expected no suggestions, got %v", check.Suggestions)
			}
		})
	}
}

// =============================================================================
// CONCURRENT ACCESS TESTS
// =============================================================================

func TestGuardian_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	guardian := NewGuardian(store, DefaultGuardianConfig())
	guardian.Initialize()

	vision := &Vision{Mission: "Test", Problem: "Test", VisionStmt: "Test"}
	guardian.UpdateVision(vision)

	done := make(chan bool)

	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func() {
			guardian.HasVision()
			guardian.GetVision()
			guardian.GetState()
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for concurrent access")
		}
	}
}
