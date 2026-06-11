package transparency

import (
	"errors"
	"strings"
	"testing"
)

func TestClassifyErrorSafety(t *testing.T) {
	err := errors.New("permission denied by policy")
	classified := ClassifyError(err)
	if classified.Category != ErrorCategorySafety {
		t.Fatalf("expected safety category, got %v", classified.Category)
	}
	if !strings.Contains(classified.Format(), "[SAFETY]") {
		t.Fatalf("expected safety prefix in formatted output")
	}
}

func TestClassifyErrorTimeout(t *testing.T) {
	err := errors.New("context deadline exceeded")
	classified := ClassifyError(err)
	if classified.Category != ErrorCategoryTimeout {
		t.Fatalf("expected timeout category, got %v", classified.Category)
	}
	if len(classified.Remediation) == 0 {
		t.Fatalf("expected remediation guidance")
	}
}

func TestGetRecoveryGuideUnknown(t *testing.T) {
	guide := GetRecoveryGuide(ErrorCategoryUnknown)
	if len(guide) == 0 {
		t.Fatalf("expected fallback recovery guide")
	}
}

// TODO: TEST_GAP: [Null/Undefined/Empty] Verify ClassifyError handles custom errors where Error() returns an empty string "".
// TODO: TEST_GAP: [Type Coercion] Verify GetRecoveryGuide, Prefix, and String handle out-of-bounds ErrorCategory values (e.g., ErrorCategory(-1), ErrorCategory(999)) without panicking.
// TODO: TEST_GAP: [User Request Extremes] Verify ClassifyError handles extremely large error strings (e.g., 50MB of stack trace) efficiently without causing an OOM during strings.ToLower allocation.
// TODO: TEST_GAP: [User Request Extremes] Verify ClassifyError behavior when the error string contains null bytes, invalid UTF-8, or non-English characters.
// TODO: TEST_GAP: [Type Coercion] Verify ClassifyError correctly classifies Go 1.20+ joined errors (errors.Join) based on the concatenated string representation.
