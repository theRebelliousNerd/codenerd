package verification

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// =============================================================================
// parseVerificationResponse TESTS
// =============================================================================

func TestParseVerificationResponse_WhenValidJSON_ShouldParse(t *testing.T) {
	response := `{"success":true,"confidence":0.95,"reason":"all good","quality_violations":[],"evidence":[],"suggestions":[]}`

	parsed, err := parseVerificationResponse(response)
	if err != nil {
		t.Fatalf("parseVerificationResponse error: %v", err)
	}
	if !parsed.Success {
		t.Error("Success should be true")
	}
	if parsed.Confidence != 0.95 {
		t.Errorf("Confidence = %f, want 0.95", parsed.Confidence)
	}
	if parsed.Reason != "all good" {
		t.Errorf("Reason = %q, want 'all good'", parsed.Reason)
	}
}

func TestParseVerificationResponse_WhenCodeFenced_ShouldStrip(t *testing.T) {
	response := "```json\n{\"success\":false,\"confidence\":0.3,\"reason\":\"failed\"}\n```"

	parsed, err := parseVerificationResponse(response)
	if err != nil {
		t.Fatalf("parseVerificationResponse error: %v", err)
	}
	if parsed.Success {
		t.Error("Success should be false")
	}
}

func TestParseVerificationResponse_WhenPlainCodeFenced_ShouldStrip(t *testing.T) {
	response := "```\n{\"success\":true,\"confidence\":0.8,\"reason\":\"ok\"}\n```"

	parsed, err := parseVerificationResponse(response)
	if err != nil {
		t.Fatalf("parseVerificationResponse error: %v", err)
	}
	if !parsed.Success {
		t.Error("Success should be true")
	}
}

func TestParseVerificationResponse_WhenInvalidJSON_ShouldError(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"garbage", "not json at all"},
		{"incomplete", `{"success": true`},
		{"wrong_type", `{"success": "yes"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseVerificationResponse(tc.input)
			if err == nil {
				t.Error("Should error on invalid JSON")
			}
		})
	}
}

func TestParseVerificationResponse_WithQualityViolations_ShouldParseAll(t *testing.T) {
	response := `{
		"success": false,
		"confidence": 0.2,
		"reason": "multiple issues",
		"quality_violations": ["mock_code", "placeholder", "incomplete"],
		"evidence": ["line 5: TODO", "line 10: Mock"],
		"suggestions": ["fix it"],
		"corrective_action": {
			"type": "research",
			"query": "how to implement",
			"reason": "need real API"
		}
	}`

	parsed, err := parseVerificationResponse(response)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if parsed.Success {
		t.Error("Success should be false")
	}
	if len(parsed.QualityViolations) != 3 {
		t.Errorf("Expected 3 violations, got %d", len(parsed.QualityViolations))
	}
	if len(parsed.Evidence) != 2 {
		t.Errorf("Expected 2 evidence items, got %d", len(parsed.Evidence))
	}
	if parsed.CorrectiveAction == nil {
		t.Fatal("CorrectiveAction should not be nil")
	}
	if parsed.CorrectiveAction.Type != CorrectiveResearch {
		t.Errorf("CorrectiveAction.Type = %q, want %q", parsed.CorrectiveAction.Type, CorrectiveResearch)
	}
}

// =============================================================================
// NewTaskVerifier TESTS
// =============================================================================

func TestNewTaskVerifier_WhenAllNil_ShouldNotPanic(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, nil)
	if v == nil {
		t.Fatal("NewTaskVerifier should not return nil")
	}
}

func TestNewTaskVerifier_ShouldStoreFields(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, nil)
	if v.client != nil {
		t.Error("client should be nil")
	}
	if v.localDB != nil {
		t.Error("localDB should be nil")
	}
	if v.shardMgr != nil {
		t.Error("shardMgr should be nil")
	}
	if v.autopoiesis != nil {
		t.Error("autopoiesis should be nil")
	}
}

// =============================================================================
// SetSessionContext TESTS
// =============================================================================

func TestSetSessionContext_ShouldStoreValues(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, nil)
	v.SetSessionContext("session123", 5)

	v.mu.RLock()
	defer v.mu.RUnlock()
	if v.sessionID != "session123" {
		t.Errorf("sessionID = %q, want 'session123'", v.sessionID)
	}
	if v.turnCount != 5 {
		t.Errorf("turnCount = %d, want 5", v.turnCount)
	}
}

// =============================================================================
// spawnTask TESTS
// =============================================================================

func TestSpawnTask_WhenNoExecutor_ShouldError(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, nil)

	_, err := v.spawnTask(context.Background(), "test", "task")
	if err == nil {
		t.Fatal("spawnTask with no executor should error")
	}
	if !strings.Contains(err.Error(), "no executor available") {
		t.Errorf("Error should mention no executor: %v", err)
	}
}

// =============================================================================
// enrichTaskWithContext TESTS
// =============================================================================

func TestEnrichTaskWithContext_WhenNoVerification_ShouldAddContextOnly(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, nil)

	result := v.enrichTaskWithContext("original task", "extra context", nil)

	if !strings.Contains(result, "original task") {
		t.Error("Should contain original task")
	}
	if !strings.Contains(result, "extra context") {
		t.Error("Should contain extra context")
	}
	if !strings.Contains(result, "IMPORTANT") {
		t.Error("Should contain quality reminder")
	}
}

func TestEnrichTaskWithContext_WhenVerificationFailed_ShouldAddFailureInfo(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, nil)

	verification := &VerificationResult{
		Success:           false,
		Reason:            "Code has quality issues",
		QualityViolations: []QualityViolation{MockCode, PlaceholderCode},
		Evidence:          []string{"line 5: mock", "line 10: TODO"},
	}

	result := v.enrichTaskWithContext("fix the code", "", verification)

	if !strings.Contains(result, "Previous Attempt Failed") {
		t.Error("Should contain failure header")
	}
	if !strings.Contains(result, "Code has quality issues") {
		t.Error("Should contain failure reason")
	}
	if !strings.Contains(result, "mock_code") {
		t.Error("Should list violations")
	}
	if !strings.Contains(result, "line 5: mock") {
		t.Error("Should list evidence")
	}
}

func TestEnrichTaskWithContext_WhenEmptyContext_ShouldStillAddReminder(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, nil)

	result := v.enrichTaskWithContext("task", "", nil)

	if !strings.Contains(result, "Do NOT use mock") {
		t.Error("Should contain anti-mock reminder")
	}
	if !strings.Contains(result, "Do NOT use TODO") {
		t.Error("Should contain anti-TODO reminder")
	}
}

// =============================================================================
// VerifyWithRetry TESTS
// =============================================================================

func TestVerifyWithRetry_WhenNoExecutor_ShouldError(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, nil)

	_, _, err := v.VerifyWithRetry(context.Background(), Delegation{Task: "test task", Persona: "coder", MaxAttempts: 1})
	if err == nil {
		t.Fatal("VerifyWithRetry should error when no executor available")
	}
}

// A delegation with no attempt cap is refused before any attempt runs: the
// cap is the persona's shard_profiles.<persona>.max_retries, never a
// constant of the verifier's.
func TestVerifyWithRetry_WhenNoAttemptCap_ShouldRefuse(t *testing.T) {
	exec := &stubTaskExecutor{outcome: "/done", result: "out"}
	v := newDelegationVerifier(t, nil, nil, exec)

	_, _, err := v.VerifyWithRetry(context.Background(), Delegation{Task: "test", Persona: "coder", MaxAttempts: 0})
	if err == nil || !strings.Contains(err.Error(), "max_retries") {
		t.Fatalf("VerifyWithRetry = %v, want a refusal naming max_retries", err)
	}
	if exec.calls != 0 {
		t.Fatalf("ran %d attempts with no cap", exec.calls)
	}
}

// =============================================================================
// applyCorrectiveAction TESTS
// =============================================================================

func TestApplyCorrectiveAction_WhenNil_ShouldReturnEmpty(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, nil)
	result := v.applyCorrectiveAction(context.Background(), nil)
	if result != "" {
		t.Errorf("Expected empty string for nil action, got %q", result)
	}
}

func TestApplyCorrectiveAction_WhenDecompose_ShouldReturnHint(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, nil)
	action := &CorrectiveAction{
		Type:  CorrectiveDecompose,
		Query: "break into smaller steps",
	}

	result := v.applyCorrectiveAction(context.Background(), action)
	if !strings.Contains(result, "Task Decomposition") {
		t.Error("Should contain decomposition hint")
	}
	if !strings.Contains(result, "break into smaller steps") {
		t.Error("Should contain the query")
	}
}

func TestApplyCorrectiveAction_WhenToolNoAutopoiesis_ShouldReturnEmpty(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, nil)
	action := &CorrectiveAction{
		Type:   CorrectiveTool,
		Query:  "generate_tool",
		Reason: "need tool",
	}

	result := v.applyCorrectiveAction(context.Background(), action)
	if result != "" {
		t.Errorf("Expected empty with no autopoiesis, got %q", result)
	}
}

// =============================================================================
// findMatchingSpecialist TESTS
// =============================================================================

func TestFindMatchingSpecialist_WhenNoShardMgr_ShouldReturnEmpty(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, nil)
	result := v.findMatchingSpecialist("hint", "query")
	if result != "" {
		t.Errorf("Expected empty with no shard manager, got %q", result)
	}
}

// =============================================================================
// storeVerification TESTS
// =============================================================================

func TestStoreVerification_WhenNoLocalDB_ShouldNotPanic(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, nil)
	verification := &VerificationResult{
		Success:    true,
		Confidence: 0.9,
		Reason:     "ok",
	}

	// Should not panic
	v.storeVerification("task", "shard", verification, 0, true)
}

// =============================================================================
// QualityViolation / CorrectiveType constants TESTS
// =============================================================================

func TestQualityViolationConstants_ShouldHaveExpectedValues(t *testing.T) {
	tests := []struct {
		violation QualityViolation
		expected  string
	}{
		{MockCode, "mock_code"},
		{PlaceholderCode, "placeholder"},
		{HallucinatedAPI, "hallucinated_api"},
		{IncompleteImpl, "incomplete"},
		{HardcodedValues, "hardcoded"},
		{EmptyFunction, "empty_function"},
		{MissingErrors, "missing_errors"},
		{FakeTests, "fake_tests"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			if string(tc.violation) != tc.expected {
				t.Errorf("QualityViolation = %q, want %q", tc.violation, tc.expected)
			}
		})
	}
}

func TestCorrectiveTypeConstants_ShouldHaveExpectedValues(t *testing.T) {
	tests := []struct {
		ct       CorrectiveType
		expected string
	}{
		{CorrectiveResearch, "research"},
		{CorrectiveDocs, "docs"},
		{CorrectiveTool, "tool"},
		{CorrectiveDecompose, "decompose"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			if string(tc.ct) != tc.expected {
				t.Errorf("CorrectiveType = %q, want %q", tc.ct, tc.expected)
			}
		})
	}
}

// =============================================================================
// ErrMaxRetriesExceeded TESTS
// =============================================================================

func TestErrMaxRetriesExceeded_ShouldBeDescriptive(t *testing.T) {
	if ErrMaxRetriesExceeded == nil {
		t.Fatal("ErrMaxRetriesExceeded should not be nil")
	}
	msg := ErrMaxRetriesExceeded.Error()
	if !strings.Contains(msg, "max retries") {
		t.Errorf("Error message should mention max retries: %q", msg)
	}
}

// =============================================================================
// verifyTask edge case TESTS
// =============================================================================

func TestVerifyTask_WhenNilClient_ShouldReturnUnavailable(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, nil)

	result, err := v.verifyTask(context.Background(), "task", "result", "/implementation")
	if err == nil {
		t.Fatal("verifyTask with nil client should error fail-closed")
	}
	if !errors.Is(err, ErrVerificationUnavailable) {
		t.Fatalf("verifyTask error = %v, want it to wrap ErrVerificationUnavailable", err)
	}
	if result != nil {
		t.Fatalf("verifyTask result = %#v, want nil when verification could not run", result)
	}
}
