package campaign

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"codenerd/internal/broker"
)

func refusal() error {
	return &broker.AdmissionError{
		Purpose: "/reasoning",
		Decision: broker.Decision{
			Allowed: false,
			Code:    broker.DecisionBudgetExhausted,
			Reason:  "purpose cap spent",
		},
	}
}

// A broker refusal is not a bug in the task, and the difference is expensive.
//
// A task classified /logic retries on a backoff capped at 30 seconds and, after
// enough attempts, gets a repro-diagnostic task inserted so the agent can debug
// it. Applied to a budget refusal that is exactly backwards: the broker has
// just said there are no tokens to spend, and the response would be to schedule
// a fresh unit of work that needs tokens to investigate why the model would not
// answer. There is nothing to reproduce.
func TestBrokerRefusalIsNotClassifiedAsALogicBug(t *testing.T) {
	if got := classifyTaskError(refusal()); got != errorTypeRefused {
		t.Errorf("classifyTaskError(refusal) = %q, want %q", got, errorTypeRefused)
	}
}

// Every path a refusal actually travels wraps it, so the check has to reach
// through. This is the same defect that made IsAdmissionError itself useless
// before it moved to errors.As.
func TestWrappedRefusalIsStillRecognised(t *testing.T) {
	wrapped := fmt.Errorf("observation failed: %w", fmt.Errorf("shard turn: %w", refusal()))
	if got := classifyTaskError(wrapped); got != errorTypeRefused {
		t.Errorf("classifyTaskError(wrapped refusal) = %q, want %q — a refusal is "+
			"never raised bare, so a classifier that only sees the outermost error "+
			"sees a code bug on every real path", got, errorTypeRefused)
	}
}

// The existing classifications must not move.
func TestClassifyTaskErrorKeepsItsExistingVerdicts(t *testing.T) {
	cases := map[string]struct {
		err  error
		want string
	}{
		"nil":              {nil, "/logic"},
		"deadline":         {context.DeadlineExceeded, "/transient"},
		"cancelled":        {context.Canceled, "/transient"},
		"wrapped deadline": {fmt.Errorf("task: %w", context.DeadlineExceeded), "/transient"},
		"rate limit text":  {errors.New("429 rate limit exceeded"), "/transient"},
		"connection reset": {errors.New("read tcp: connection reset by peer"), "/transient"},
		"plain failure":    {errors.New("compilation failed: undefined variable"), "/logic"},
	}
	for name, tc := range cases {
		if got := classifyTaskError(tc.err); got != tc.want {
			t.Errorf("%s: classifyTaskError = %q, want %q", name, got, tc.want)
		}
	}
}

// A refusal must keep the full exponential backoff rather than the shortened
// one /logic gets. Retrying quickly against a broker that just declined spends
// attempts against a limit that has not moved; the only useful thing a retry
// can do is arrive later.
func TestRefusalBacksOffLongerThanALogicError(t *testing.T) {
	o := &Orchestrator{config: OrchestratorConfig{
		RetryBackoffBase: 5 * time.Second,
		RetryBackoffMax:  5 * time.Minute,
	}}

	const attempt = 5 // 5s << 4 = 80s, above the 30s /logic cap
	logic := o.computeRetryBackoff("/logic", attempt)
	refused := o.computeRetryBackoff(errorTypeRefused, attempt)

	if logic != 30*time.Second {
		t.Fatalf("the /logic cap changed (%v); this test's premise no longer holds", logic)
	}
	if refused <= logic {
		t.Errorf("refusal backoff %v is not longer than the /logic backoff %v: "+
			"a refusal would retry against an unchanged limit as fast as a code bug", refused, logic)
	}
}
