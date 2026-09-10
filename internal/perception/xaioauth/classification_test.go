package xaioauth

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// The probe's classification switch decides what a user is told when auth
// fails, and one arm of it used a bare type assertion while its two neighbours
// reached through a wrapping chain. That inconsistency is the tell: a rate
// limit hidden by a wrapper is classified as a generic failure, and the agent
// retries immediately against the thing that just told it to slow down.

func TestRateLimitIsRecognizedThroughWrapping(t *testing.T) {
	base := &RateLimitedError{RetryAfter: 30 * time.Second}

	for _, tc := range []struct {
		name string
		err  error
	}{
		{"bare", base},
		{"wrapped once", fmt.Errorf("completion failed: %w", base)},
		{"wrapped twice", fmt.Errorf("probe: %w", fmt.Errorf("completion failed: %w", base))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !isRateLimited(tc.err) {
				t.Fatalf("isRateLimited(%v) = false; the agent would retry into a rate limit", tc.err)
			}
		})
	}
}

func TestRateLimitClassificationRejectsOtherFailures(t *testing.T) {
	for _, err := range []error{
		nil,
		errors.New("connection reset"),
		fmt.Errorf("completion failed: %w", errors.New("timeout")),
		&TierForbiddenError{StatusCode: 403},
	} {
		if isRateLimited(err) {
			t.Errorf("isRateLimited(%v) = true; an unrelated failure would be reported as a rate limit", err)
		}
	}
}

func TestLoginMessageUsesDetailThroughWrapping(t *testing.T) {
	base := &AuthRequiredError{Detail: "device authorization timed out"}
	wrapped := fmt.Errorf("probe completion: %w", base)

	got := formatLoginRequiredMessage(wrapped)
	// Without errors.As this falls through to the generic "no credentials"
	// message, which sends the user to re-run login when the real problem was
	// that they never finished the device flow.
	if !strings.Contains(got, "device authorization timed out") {
		t.Fatalf("message = %q, want the wrapped error's detail", got)
	}
	if !strings.Contains(got, "nerd auth grok") {
		t.Errorf("message = %q, want the remediation command", got)
	}
}

func TestLoginMessageNamesARevokedRefreshSpecifically(t *testing.T) {
	// "Revoked" and "never logged in" need different actions from the user, and
	// the generic message describes only the second.
	revoked := fmt.Errorf("wrapped: %w", &AuthRequiredError{Detail: "invalid_grant"})
	got := formatLoginRequiredMessage(revoked)
	if !strings.Contains(got, "revoked") {
		t.Errorf("message = %q, want it to name the revocation", got)
	}
}

func TestLoginMessageHandlesNilAndUndetailedErrors(t *testing.T) {
	if got := formatLoginRequiredMessage(nil); !strings.Contains(got, "nerd auth grok") {
		t.Errorf("nil message = %q, want the remediation command", got)
	}
	// An AuthRequiredError with no detail carries nothing extra to say, so the
	// generic message is the correct answer rather than a fallthrough bug.
	got := formatLoginRequiredMessage(&AuthRequiredError{})
	if !strings.Contains(got, "no SuperGrok OAuth credentials") {
		t.Errorf("undetailed message = %q", got)
	}
}

func TestQuarantineMessageDistinguishesARevokedRefresh(t *testing.T) {
	if got := formatQuarantineMessage(""); !strings.Contains(got, "re-run") {
		t.Errorf("empty-reason message = %q, want the remediation", got)
	}

	revoked := formatQuarantineMessage("invalid_grant")
	if !strings.Contains(revoked, "invalid_grant") {
		t.Errorf("message = %q, want the reason quoted", revoked)
	}

	other := formatQuarantineMessage("clock skew")
	if !strings.Contains(other, "clock skew") {
		t.Errorf("message = %q, want the reason quoted", other)
	}
	// The two forms differ: a revoked refresh is named up front, anything else
	// is parenthetical. Collapsing them loses the distinction between "your
	// grant is gone" and "something transient happened".
	if revoked == other {
		t.Error("a revoked refresh and an unrelated reason produced the same message")
	}
}

func TestAuthAndTierHelpersStillReachThroughWrapping(t *testing.T) {
	// The two arms that were already correct, pinned so the switch stays
	// consistent rather than drifting back one arm at a time.
	authWrapped := fmt.Errorf("outer: %w", &AuthRequiredError{Detail: "x"})
	if !IsAuthRequired(authWrapped) {
		t.Error("IsAuthRequired lost a wrapped AuthRequiredError")
	}
	tierWrapped := fmt.Errorf("outer: %w", &TierForbiddenError{StatusCode: 403})
	if !IsTierForbidden(tierWrapped) {
		t.Error("IsTierForbidden lost a wrapped TierForbiddenError")
	}
	// And they must not answer for each other.
	if IsTierForbidden(authWrapped) || IsAuthRequired(tierWrapped) {
		t.Error("the auth and tier classifications overlap")
	}
}

func TestTerminalRefreshFailureRecognisesTheRevocationVocabulary(t *testing.T) {
	for _, detail := range []string{"invalid_grant", "INVALID_GRANT", "the grant was revoked"} {
		if !IsTerminalRefreshFailure(detail) {
			t.Errorf("IsTerminalRefreshFailure(%q) = false; a revoked grant would be retried forever", detail)
		}
	}
	for _, detail := range []string{"", "connection reset", "temporarily unavailable"} {
		if IsTerminalRefreshFailure(detail) {
			t.Errorf("IsTerminalRefreshFailure(%q) = true; a transient failure would be reported as terminal", detail)
		}
	}
}
