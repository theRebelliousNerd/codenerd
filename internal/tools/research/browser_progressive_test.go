package research

import (
	"testing"

	"codenerd/internal/browser"
)

func TestPageStepResults_WhenWindowCoversAll_ShouldReturnWhole(t *testing.T) {
	t.Parallel()
	steps := make([]browser.ActionStepResult, 25)
	got, end := pageStepResults(steps, 0, 20)
	if len(got) != 20 || end != 20 {
		t.Fatalf("window = %d/%d, want 20/20", len(got), end)
	}
	// The old hard clamp at 25 is gone: a wide window reaches every step.
	got, end = pageStepResults(steps, 0, 50)
	if len(got) != 25 || end != 25 {
		t.Fatalf("wide window = %d/%d, want 25/25", len(got), end)
	}
}

func TestPageStepResults_WhenOffsetSet_ShouldPageWithoutLoss(t *testing.T) {
	t.Parallel()
	steps := make([]browser.ActionStepResult, 25)
	first, end := pageStepResults(steps, 0, 20)
	second, end2 := pageStepResults(steps, end, 20)
	if len(first)+len(second) != 25 {
		t.Fatalf("paged windows cover %d steps, want 25", len(first)+len(second))
	}
	if end2 != 25 {
		t.Fatalf("second page end = %d, want 25", end2)
	}
}

func TestPageStepResults_WhenOffsetPastEnd_ShouldReturnEmpty(t *testing.T) {
	t.Parallel()
	steps := make([]browser.ActionStepResult, 3)
	got, end := pageStepResults(steps, 99, 20)
	if len(got) != 0 || end != 3 {
		t.Fatalf("past-end window = %d/%d, want 0/3", len(got), end)
	}
}
