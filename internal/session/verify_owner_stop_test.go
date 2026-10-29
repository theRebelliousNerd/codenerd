package session

import (
	"context"
	"testing"
	"time"
)

// A cancel that lands after the parent's soft deadline still stops
// verification. The parent's deadline fires while the command runs
// (verification keeps its own budget), then the operation's owner -- the
// process's signal handling -- is canceled. The supervisor used to poll the
// parent for Canceled, but a context's first error is permanent, so it kept
// reading DeadlineExceeded and the command ran on (audit
// verification-cancellation-after-deadline).
func TestVerification_OwnerCancelAfterTheParentDeadlineStopsIt(t *testing.T) {
	t.Parallel()
	owner, stopOwner := context.WithCancel(context.Background())
	defer stopOwner()
	timed, cancelTimed := context.WithTimeout(owner, 20*time.Millisecond)
	defer cancelTimed()
	parent := WithOwnerStop(timed, owner)

	started := make(chan struct{})
	runner := func(ctx context.Context, _ string, _ []string, _ string, _ []string) ([]byte, error) {
		close(started)
		<-ctx.Done() // a long build: ends only when its context is canceled
		return []byte("partial output"), ctx.Err()
	}
	type result struct {
		out     []byte
		outcome VerifyOutcome
	}
	got := make(chan result, 1)
	go func() {
		out, outcome, _ := runVerificationCommand(parent, t.TempDir(), nil, 0, "go", []string{"build", "./..."}, runner)
		got <- result{out, outcome}
	}()
	<-started
	<-timed.Done() // the parent's deadline fires first
	time.Sleep(30 * time.Millisecond)
	stopOwner() // then SIGTERM reaches the owner

	select {
	case r := <-got:
		if r.outcome != VerifyCanceled || string(r.out) != "partial output" {
			t.Fatalf("outcome = %v (%q), want canceled with the partial output", r.outcome, r.out)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("verification kept running after the owner was canceled past the parent's deadline")
	}
}

// The parent's soft deadline alone does not stop verification: it keeps its
// own budget.
func TestVerification_ParentDeadlineAloneDoesNotStopIt(t *testing.T) {
	t.Parallel()
	timed, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	runner := func(ctx context.Context, _ string, _ []string, _ string, _ []string) ([]byte, error) {
		time.Sleep(60 * time.Millisecond)
		return []byte("ok"), ctx.Err()
	}
	if _, outcome, reason := runVerificationCommand(timed, t.TempDir(), nil, 0, "go", []string{"build"}, runner); outcome != VerifyPassed {
		t.Fatalf("outcome = %v (%s), want passed: a soft deadline does not abort verification", outcome, reason)
	}
}
