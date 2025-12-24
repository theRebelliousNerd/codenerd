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
