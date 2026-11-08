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

func TestClassifyError_WhenDeclaredCategory_ShouldBeatHeuristics(t *testing.T) {
	cause := errors.New("file not found: missing .json config")
	be := NewSafetyError("/exec", "/tmp/x", "permitted", cause)
	classified := ClassifyError(be)
	if classified == nil {
		t.Fatal("expected classified error for BoundaryError")
	}
	if classified.Category != ErrorCategorySafety {
		t.Fatalf("expected declared safety category, got %v", classified.Category)
	}
	if classified.Summary != summaryForCategory(ErrorCategorySafety) {
		t.Fatalf("expected declared summary %q, got %q", summaryForCategory(ErrorCategorySafety), classified.Summary)
	}
	if !errors.Is(classified, cause) {
		t.Fatalf("expected errors.Is to reach the BoundaryError cause")
	}
}

func TestClassifyError_WhenEmptyMessage_ShouldReturnUnknownWithoutPanic(t *testing.T) {
	classified := ClassifyError(errors.New(""))
	if classified == nil {
		t.Fatal("expected classified error for empty message")
	}
	if classified.Category != ErrorCategoryUnknown {
		t.Fatalf("expected unknown category for empty message, got %v", classified.Category)
	}
}

func TestClassifyError_WhenJoinedErrors_ShouldClassifyFromCombinedText(t *testing.T) {
	joined := errors.Join(errors.New("ok"), errors.New("connection refused"))
	classified := ClassifyError(joined)
	if classified == nil {
		t.Fatal("expected classified error for joined errors")
	}
	if classified.Category != ErrorCategoryNetwork {
		t.Fatalf("expected network category for joined error, got %v", classified.Category)
	}
}

func TestErrorCategory_WhenNegative_ShouldReturnFallback(t *testing.T) {
	if got := ErrorCategory(-1).Prefix(); got != "[ERROR]" {
		t.Fatalf("expected fallback prefix, got %q", got)
	}
	if got := ErrorCategory(-1).String(); got != "unknown" {
		t.Fatalf("expected fallback name, got %q", got)
	}
}

func TestBoundaryError_WhenNilOrEmpty_ShouldBeSafe(t *testing.T) {
	var nilBE *BoundaryError
	if got := nilBE.Error(); got != "" {
		t.Fatalf("expected empty Error() for nil BoundaryError, got %q", got)
	}
	if got := nilBE.TransparencyCategory(); got != ErrorCategoryUnknown {
		t.Fatalf("expected unknown category for nil BoundaryError, got %v", got)
	}
	empty := &BoundaryError{}
	if got := empty.Error(); got != "" {
		t.Fatalf("expected empty Error() for empty BoundaryError, got %q", got)
	}
	if empty.Unwrap() != nil {
		t.Fatalf("expected nil Unwrap for empty BoundaryError")
	}
	tagged := NewSafetyError("/exec", "/tmp/x", "permitted", errors.New("boom"))
	if got := tagged.TransparencyCategory(); got != ErrorCategorySafety {
		t.Fatalf("expected safety category from NewSafetyError, got %v", got)
	}
}

func TestErrorCategory_WhenValidPrefix_ShouldReturnDistinctValue(t *testing.T) {
	if got := ErrorCategorySafety.Prefix(); got != "[SAFETY]" {
		t.Fatalf("expected [SAFETY] for valid category, got %q", got)
	}
	if got := ErrorCategoryTimeout.Prefix(); got != "[TIMEOUT]" {
		t.Fatalf("expected [TIMEOUT] for valid category, got %q", got)
	}
}

func TestErrorCategory_WhenValidName_ShouldReturnDistinctValue(t *testing.T) {
	if got := ErrorCategorySafety.String(); got != "safety" {
		t.Fatalf("expected safety for valid category, got %q", got)
	}
	if got := ErrorCategoryTimeout.String(); got != "timeout" {
		t.Fatalf("expected timeout for valid category, got %q", got)
	}
}
