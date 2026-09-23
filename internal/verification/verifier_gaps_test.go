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
	v := NewTaskVerifier(nil, nil)
	if v == nil {
		t.Fatal("NewTaskVerifier should not return nil")
	}
}

func TestNewTaskVerifier_ShouldStoreFields(t *testing.T) {
	v := NewTaskVerifier(nil, nil)
	if v.client != nil {
		t.Error("client should be nil")
	}
	if v.localDB != nil {
		t.Error("localDB should be nil")
	}
}

// =============================================================================
// SetSessionContext TESTS
// =============================================================================

func TestSetSessionContext_ShouldStoreValues(t *testing.T) {
	v := NewTaskVerifier(nil, nil)
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
// retryTask TESTS
// =============================================================================

func TestRetryTask_WithoutAVerdictIsTheTask(t *testing.T) {
	if got := retryTask("original task", nil); got != "original task" {
		t.Errorf("retryTask = %q, want the task unchanged", got)
	}
}

func TestRetryTask_CarriesWhyTheAttemptFailed(t *testing.T) {
	verification := &VerificationResult{
		Success:           false,
		Reason:            "Code has quality issues",
		QualityViolations: []QualityViolation{MockCode, PlaceholderCode},
		Evidence:          []string{"line 5: mock", "line 10: TODO"},
	}

	result := retryTask("fix the code", verification)

	for _, want := range []string{"fix the code", "Previous Attempt Failed", "Code has quality issues", "mock_code", "line 5: mock"} {
		if !strings.Contains(result, want) {
			t.Errorf("retry task does not carry %q:\n%s", want, result)
		}
	}
}

// The judge's corrective action reaches the retry as advice. Until 2026-09-23
// the verifier ran it: a specialist chosen by a keyword table, or a tool
// generated through autopoiesis, on the judge's word -- and the judge can only
// withhold (F5).
func TestRetryTask_TheJudgesCorrectiveActionIsAdviceNotAnAction(t *testing.T) {
	verification := &VerificationResult{
		Reason:           "the scraper calls an API that does not exist",
		CorrectiveAction: &CorrectiveAction{Type: CorrectiveTool, Query: "a rod page scraper", Reason: "no tool reads the page"},
	}

	result := retryTask("scrape the page", verification)

	for _, want := range []string{"Suggested Next Step", "tool: a rod page scraper", "no tool reads the page"} {
		if !strings.Contains(result, want) {
			t.Errorf("retry task does not carry %q:\n%s", want, result)
		}
	}
}

// =============================================================================
// VerifyWithRetry TESTS
// =============================================================================

func TestVerifyWithRetry_WhenNoExecutor_ShouldError(t *testing.T) {
	v := NewTaskVerifier(nil, nil)

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
// storeVerification TESTS
// =============================================================================

func TestStoreVerification_WhenNoLocalDB_ShouldNotPanic(t *testing.T) {
	v := NewTaskVerifier(nil, nil)
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
	v := NewTaskVerifier(nil, nil)

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
