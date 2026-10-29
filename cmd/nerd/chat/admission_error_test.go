package chat

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/broker"
)

func refusal(code broker.DecisionCode, tokens, window int, reason string) *broker.AdmissionError {
	return &broker.AdmissionError{
		Purpose: broker.PurposePerception,
		Decision: broker.Decision{
			Code:   code,
			Reason: reason,
			Count:  broker.Count{Tokens: tokens, Confidence: broker.ConfidenceExact},
			Window: window,
		},
	}
}

func TestExplainAdmissionErrorPassesOtherFailuresThrough(t *testing.T) {
	// Rewriting an ordinary failure into a budget explanation would send the
	// reader looking for a problem that is not there.
	for _, err := range []error{
		errors.New("connection reset by peer"),
		fmt.Errorf("observation failed: %w", errors.New("context deadline exceeded")),
	} {
		if got := explainAdmissionError(err); got != err {
			t.Errorf("explainAdmissionError(%v) rewrote a non-refusal into %q", err, got)
		}
	}
	if got := explainAdmissionError(nil); got != nil {
		t.Errorf("explainAdmissionError(nil) = %v, want nil", got)
	}
}

func TestExplainAdmissionErrorReachesThroughWrapping(t *testing.T) {
	// The path a refusal really travels: raised inside perception, wrapped by
	// the executor, wrapped again by the turn.
	wrapped := fmt.Errorf("turn failed: %w",
		fmt.Errorf("observation failed: %w", refusal(broker.DecisionWindowExceeded, 210000, 200000, "")))

	got := explainAdmissionError(wrapped).Error()
	if !strings.Contains(got, "did not fit the model's context window") {
		t.Fatalf("a wrapped refusal was not recognized:\n%s", got)
	}
}

func TestWindowExceededSaysWhatToDoAndKeepsTheNumbers(t *testing.T) {
	got := explainAdmissionError(refusal(broker.DecisionWindowExceeded, 210000, 200000, "")).Error()

	// The action, so the message is usable.
	for _, want := range []string{"did not fit", "Nothing was billed", "Start a new session"} {
		if !strings.Contains(got, want) {
			t.Errorf("message is missing %q:\n%s", want, got)
		}
	}
	// The numbers, because an exact count is the difference between a
	// diagnosis and a guess, and the arithmetic must be right.
	for _, want := range []string{"210000", "200000", "10000", "exact"} {
		if !strings.Contains(got, want) {
			t.Errorf("message is missing the figure %q:\n%s", want, got)
		}
	}
}

func TestBudgetExhaustedNamesThePurpose(t *testing.T) {
	got := explainAdmissionError(refusal(broker.DecisionBudgetExhausted, 5000, 200000, "")).Error()
	if !strings.Contains(got, string(broker.PurposePerception)) {
		t.Errorf("message does not say which budget ran out:\n%s", got)
	}
	if !strings.Contains(got, "Nothing was billed") {
		t.Errorf("message does not say the request was never sent:\n%s", got)
	}
}

func TestCountUnavailableSaysItFailedClosed(t *testing.T) {
	got := explainAdmissionError(
		refusal(broker.DecisionCountUnavailable, 0, 200000, "count_tokens: dial tcp: i/o timeout")).Error()

	// Failing closed is a deliberate policy, not an outage. A message that
	// leaves it looking like a provider problem invites the wrong fix.
	if !strings.Contains(got, "failing closed") {
		t.Errorf("message does not explain that the refusal was deliberate:\n%s", got)
	}
	if !strings.Contains(got, "i/o timeout") {
		t.Errorf("message drops the underlying reason:\n%s", got)
	}
}

func TestUnknownDecisionCodeIsLeftAlone(t *testing.T) {
	// A code this function does not know how to explain must pass through with
	// its original text rather than be flattened into a wrong explanation.
	orig := refusal(broker.DecisionCode("something_new"), 100, 200000, "")
	if got := explainAdmissionError(orig); got.Error() != orig.Error() {
		t.Errorf("an unrecognized refusal code was rewritten:\n%s", got)
	}
}
