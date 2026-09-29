package session

import (
	"context"
	"testing"
	"time"
)

// A verification budget of zero means the command is not bounded by the
// harness. Steve, 2026-09-19: "there should not be timeouts like that... some
// agentic runs are like hours long." The four-minute ceiling was shorter than
// the suite it gated -- internal/session's own tests take 259 s -- so any turn
// touching a package it imports could never have its importers verified.
//
// context.WithTimeout(ctx, 0) is already expired, so an unbounded budget needs
// its own path: without one, "no budget" would mean "no time at all" and every
// verification would come back indeterminate before the command started.
func TestRunVerificationCommand_ZeroBudgetIsUnbounded(t *testing.T) {
	ran := false
	runner := func(ctx context.Context, _ string, _ []string, _ string, _ []string) ([]byte, error) {
		// Long enough that any accidental sub-second budget would cut it,
		// short enough to keep the test quick.
		select {
		case <-time.After(150 * time.Millisecond):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		ran = true
		return []byte("ok"), nil
	}
	out, outcome, reason := runVerificationCommand(context.Background(), t.TempDir(), nil, 0, "go", []string{"test"}, runner)
	if !ran {
		t.Fatal("an unbounded verification never ran its command")
	}
	if outcome != VerifyPassed {
		t.Fatalf("outcome = %s (%s), want passed; out=%q", outcome, reason, out)
	}
}
