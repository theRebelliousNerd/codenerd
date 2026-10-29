package chat

import (
	"context"
	"errors"
	"strings"
	"testing"

	"codenerd/internal/observation"
	"codenerd/internal/verification"
)

// A failed delegation tells the user what the worker had already changed and
// still reads as the failure it was: the cause is findable through the wrap.
func TestFailedDelegation_CarriesWhatTheWorkerDid(t *testing.T) {
	ret := observation.Return{
		Changed: []string{"internal/x/x.go"},
		Build:   &observation.Verification{Kind: "build", Ran: false, Source: observation.SourceObserved},
		Outcome: "/failed",
	}
	err := failedDelegation("shard delegation failed", context.Canceled, ret)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("the cause is no longer findable: %v", err)
	}
	for _, want := range []string{"shard delegation failed: context canceled", "internal/x/x.go", "Build did not run"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message lacks %q:\n%s", want, err)
		}
	}
}

func TestFailedDelegation_UnmeasuredReturnIsJustTheError(t *testing.T) {
	err := failedDelegation("shard delegation failed", errors.New("boom"), observation.Return{Output: "I edited things"})
	if err.Error() != "shard delegation failed: boom" {
		t.Errorf("claimed a state the producer did not measure: %q", err)
	}
}

// The verified lane gets the observation out of the verifier's typed error.
func TestFailedVerifiedDelegation_ReadsTheExecutionError(t *testing.T) {
	cause := errors.New("provider unavailable")
	err := failedVerifiedDelegation("verified execution failed", &verification.ExecutionError{
		Attempt: 1,
		Return:  observation.Return{Changed: []string{"a.go", "b.go"}},
		Err:     cause,
	})
	if !errors.Is(err, cause) {
		t.Fatalf("lost the cause: %v", err)
	}
	var executed *verification.ExecutionError
	if !errors.As(err, &executed) {
		t.Errorf("lost the verifier's typed error")
	}
	if !strings.Contains(err.Error(), "a.go, b.go") {
		t.Errorf("the changed files are not named:\n%s", err)
	}

	plain := failedVerifiedDelegation("verified execution failed", verification.ErrVerificationUnavailable)
	if strings.Contains(plain.Error(), "Partial state") {
		t.Errorf("a failure with no worker observation invented one:\n%s", plain)
	}
}
