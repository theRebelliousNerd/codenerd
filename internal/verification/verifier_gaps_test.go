package verification

import (
	"codenerd/internal/config"
	"context"
	"strings"
	"testing"
)

// =============================================================================
// parseVerificationResponse TESTS
// =============================================================================

// =============================================================================
// NewTaskVerifier TESTS
// =============================================================================

func TestNewTaskVerifier_WhenAllNil_ShouldNotPanic(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, config.JITConfig{})
	if v == nil {
		t.Fatal("NewTaskVerifier should not return nil")
	}
}

func TestNewTaskVerifier_ShouldStoreFields(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, config.JITConfig{})
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
	v := NewTaskVerifier(nil, nil, nil, config.JITConfig{})
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

// =============================================================================
// VerifyWithRetry TESTS
// =============================================================================

func TestVerifyWithRetry_WhenNoExecutor_ShouldError(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, config.JITConfig{})

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
