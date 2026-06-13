package context_harness

import (
	"strings"
	"testing"

	"codenerd/internal/core"
)

func TestEstimateFactTokens(t *testing.T) {
	// 5 base tokens + 5 per arg.
	if got := estimateFactTokens(core.Fact{Predicate: "p", Args: []any{"a", "b", "c"}}); got != 20 {
		t.Errorf("estimateFactTokens(3 args)=%d, want 20", got)
	}
	if got := estimateFactTokens(core.Fact{Predicate: "p"}); got != 5 {
		t.Errorf("estimateFactTokens(0 args)=%d, want 5", got)
	}
}

func TestFormatFloat(t *testing.T) {
	if got := formatFloat(5.0); got != "5" {
		t.Errorf("formatFloat(5.0)=%q, want 5", got)
	}
	if got := formatFloat(3.0); got != "3" {
		t.Errorf("formatFloat(3.0)=%q, want 3", got)
	}
}

func TestActivationValidationErrorMessage(t *testing.T) {
	err := &ActivationValidationError{Component: "score", Expected: 5.0, Actual: 3.0}
	msg := err.Error()
	if !strings.Contains(msg, "score") || !strings.Contains(msg, "validation failed") {
		t.Errorf("unexpected error message: %q", msg)
	}
}
