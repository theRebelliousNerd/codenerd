package campaign

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"codenerd/internal/broker"
	"codenerd/internal/config"
	"codenerd/internal/session"
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

// A refusal is read off its type, through every wrapping it travels in: a
// refusal is never raised bare.
func TestFailureSignals_ARefusalIsReadOffItsType(t *testing.T) {
	for name, err := range map[string]error{
		"bare":    refusal(),
		"wrapped": fmt.Errorf("observation failed: %w", fmt.Errorf("shard turn: %w", refusal())),
	} {
		if got := failureSignals(err); !slices.Contains(got, "/refused") {
			t.Errorf("%s: signals %v, want /refused", name, got)
		}
	}
}

func TestFailureSignals_AnEndedContextIsReadOffItsType(t *testing.T) {
	cases := map[string]struct {
		err  error
		want string
	}{
		"deadline":         {context.DeadlineExceeded, "/deadline"},
		"wrapped deadline": {fmt.Errorf("task: %w", context.DeadlineExceeded), "/deadline"},
		"canceled":         {context.Canceled, "/canceled"},
	}
	for name, tc := range cases {
		if got := failureSignals(tc.err); !slices.Contains(got, tc.want) {
			t.Errorf("%s: signals %v, want %s", name, got, tc.want)
		}
	}
}

// The error's wording is never read. The substring classifier this replaced
// called anything mentioning "connection", "eof", "i/o" or "network" a
// transient fault, so a test named TestConnectionPool failing was retried as a
// network blip (2026-09-22 sweep, F2).
func TestFailureSignals_TheErrorsWordingIsNeverRead(t *testing.T) {
	for _, text := range []string{
		"--- FAIL: TestConnectionPool (0.01s): read tcp: connection reset by peer",
		"unexpected EOF in parser; i/o timeout; network unavailable",
		"429 rate limit exceeded",
	} {
		if got := failureSignals(errors.New(text)); !slices.Equal(got, []string{signalUnclassified}) {
			t.Errorf("%q: signals %v, want only %s", text, got, signalUnclassified)
		}
	}
}

// A turn that ran but did not end done carries its verdict and every evidence
// atom the kernel found missing, and post-edit verification is its own type.
func TestFailureSignals_ATurnCarriesItsVerdictAndMissingEvidence(t *testing.T) {
	err := withSignals(fmt.Errorf("task t1: %w", ErrTaskNotDone), "/hollow", "/tests_not_green")
	got := failureSignals(err)
	for _, want := range []string{"/turn_not_done", "/hollow", "/tests_not_green"} {
		if !slices.Contains(got, want) {
			t.Errorf("signals %v, want %s", got, want)
		}
	}
	if got := failureSignals(fmt.Errorf("edit: %w", session.ErrVerificationFailed)); !slices.Contains(got, "/verification_failed") {
		t.Errorf("signals %v, want /verification_failed", got)
	}
}

// Every attempt has at least one signal, so the policy always has something
// to decide on.
func TestFailureSignals_NoErrorIsUnclassifiedNotEmpty(t *testing.T) {
	if got := failureSignals(nil); !slices.Equal(got, []string{signalUnclassified}) {
		t.Errorf("failureSignals(nil) = %v, want [%s]", got, signalUnclassified)
	}
}

// A refusal waits out the full exponential backoff; a retry with a reason to
// change something is capped lower. Retrying quickly against a broker that just
// declined spends attempts against a limit that has not moved.
func TestComputeRetryBackoff_OnlyWaitingOutKeepsTheFullBackoff(t *testing.T) {
	o := &Orchestrator{policy: testPolicy(func(c *config.CampaignConfig) {
		c.RetryBackoffBase = "5s"
		c.RetryBackoffMax = "5m"
		c.RetryWithReasonBackoffMax = "30s"
	})}
	const attempt = 5 // 5s << 4 = 80s: above the 30s cap, under the 5m max
	if got := o.computeRetryBackoff(attempt, false); got != 30*time.Second {
		t.Errorf("a retry with a reason waited %v, want the 30s cap", got)
	}
	if got := o.computeRetryBackoff(attempt, true); got != 80*time.Second {
		t.Errorf("waiting out waited %v, want the full 80s", got)
	}
}
