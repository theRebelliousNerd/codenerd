package session

import (
	"context"
	"strings"
	"testing"
	"time"
)

// A run the test gate stops because the turn's own tests failed first says
// so; any other cancel of the run's context is the operator's.
func TestRunVerificationCommand_NamesWhoCanceled(t *testing.T) {
	blocked := func(ctx context.Context, _ string, _ []string, _ string, _ []string) ([]byte, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	run := func(parent context.Context) string {
		_, outcome, reason := runVerificationCommand(parent, t.TempDir(), nil, time.Minute, "go", []string{"test"}, blocked)
		if outcome != VerifyCanceled {
			t.Fatalf("outcome = %s, want canceled", outcome)
		}
		return reason
	}

	gate, stopGate := context.WithCancelCause(context.Background())
	derived, cancelDerived := context.WithCancel(gate) // the runner sees a derived context
	defer cancelDerived()
	go func() { time.Sleep(20 * time.Millisecond); stopGate(errOwnTestsFailedFirst) }()
	if got := run(derived); !strings.Contains(got, "own tests failed first") || strings.Contains(got, "operator") {
		t.Fatalf("gate cancel reason = %q", got)
	}

	op, stopOp := context.WithCancel(context.Background())
	go func() { time.Sleep(20 * time.Millisecond); stopOp() }()
	if got := run(op); got != "operator canceled verification" {
		t.Fatalf("operator cancel reason = %q", got)
	}
}
