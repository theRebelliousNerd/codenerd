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

// formatFloat used to add the integer part to '0' as a rune: single digits
// happened to print, 20 printed as "D" and 1.5 as "1". Validation errors carry
// boost floors such as 20, so the message named the wrong number.
func TestFormatFloat(t *testing.T) {
	for in, want := range map[float64]string{5: "5.00", 3: "3.00", 20: "20.00", 1.5: "1.50"} {
		if got := formatFloat(in); got != want {
			t.Errorf("formatFloat(%g)=%q, want %q", in, got, want)
		}
	}
}

func TestActivationValidationErrorMessage(t *testing.T) {
	err := &ActivationValidationError{Component: "score", Expected: 5.0, Actual: 3.0}
	msg := err.Error()
	if !strings.Contains(msg, "score") || !strings.Contains(msg, "validation failed") {
		t.Errorf("unexpected error message: %q", msg)
	}
}
