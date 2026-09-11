package chat

import (
	"fmt"

	"codenerd/internal/broker"
)

// explainAdmissionError turns a broker refusal into something a person can act
// on, and returns err unchanged when it is anything else.
//
// A refusal is not a failure of the model or the network. It is the meter
// declining to send a request it has counted and found too large, or a budget
// it cannot check. That distinction is invisible in the raw error: by the time
// it reaches the surface it reads as
//
//	observation failed: broker refused window_exceeded request for perception:
//	counted 210000 tokens, window 200000, headroom -10000 (exact)
//
// which is accurate and tells the reader nothing about what to do next. The
// numbers are worth keeping -- an exact count is the difference between a
// diagnosis and a guess -- so they stay, with the action in front of them.
func explainAdmissionError(err error) error {
	refusal, ok := broker.IsAdmissionError(err)
	if !ok {
		return err
	}
	if explained := explainRefusal(refusal); explained != nil {
		return explained
	}
	return err
}

// explainedRefusal is the user-facing text over the refusal it explains. It
// unwraps to the original error so errors.Is/As and broker.IsAdmissionError
// still find the refusal behind the explanation.
type explainedRefusal struct {
	text  string
	cause error
}

func (e *explainedRefusal) Error() string { return e.text }
func (e *explainedRefusal) Unwrap() error { return e.cause }

// explainRefusal returns nil for a decision code it has no explanation for.
func explainRefusal(refusal *broker.AdmissionError) error {
	text := refusalText(refusal)
	if text == "" {
		return nil
	}
	return &explainedRefusal{text: text, cause: refusal}
}

func refusalText(refusal *broker.AdmissionError) string {
	d := refusal.Decision
	switch d.Code {
	case broker.DecisionWindowExceeded:
		over := d.Count.Tokens - d.Window
		return fmt.Sprintf(
			"the request did not fit the model's context window, so it was not sent.\n\n"+
				"  counted   %d tokens (%s)\n"+
				"  window    %d tokens\n"+
				"  over by   %d tokens\n"+
				"  charged to %s\n\n"+
				"Nothing was billed. Start a new session, or reduce what this turn carries "+
				"— fewer attached files, a shorter instruction, or a smaller model context.",
			d.Count.Tokens, d.Count.Confidence, d.Window, over, refusal.Purpose)

	case broker.DecisionBudgetExhausted:
		return fmt.Sprintf(
			"the %s budget is used up, so the request was not sent.\n\n"+
				"  counted  %d tokens (%s)\n\n"+
				"Nothing was billed. Raise the cap for this purpose, or start a new session.",
			refusal.Purpose, d.Count.Tokens, d.Count.Confidence)

	case broker.DecisionCountUnavailable:
		// Failing closed here is deliberate: a budget that cannot be checked is
		// not a budget. Say so, rather than leaving it looking like a provider
		// outage.
		return fmt.Sprintf(
			"the request could not be measured, so it was refused rather than sent blind: %s\n\n"+
				"Nothing was billed. This is the meter failing closed — a budget that cannot be "+
				"checked is not being enforced — and usually means the token-counting endpoint "+
				"is unreachable.",
			d.Reason)
	}

	return ""
}
