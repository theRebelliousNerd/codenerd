package xaioauth

import (
	"errors"
	"testing"
)

func TestTypedErrors(t *testing.T) {
	auth := &AuthRequiredError{Detail: "expired"}
	if !IsAuthRequired(auth) {
		t.Error("expected IsAuthRequired")
	}
	if !errors.Is(auth, ErrAuthRequired) {
		t.Error("expected unwrap to ErrAuthRequired")
	}

	tier := &TierForbiddenError{StatusCode: 403, Body: "no subscription"}
	if !IsTierForbidden(tier) {
		t.Error("expected IsTierForbidden")
	}

	rl := &RateLimitedError{RetryAfter: 0}
	if rl.Error() == "" {
		t.Error("empty rate limit message")
	}
}

func TestClassifyHTTPError(t *testing.T) {
	err := classifyHTTPError(401, []byte("unauthorized"), nil)
	if !IsAuthRequired(err) {
		t.Errorf("401 => %v", err)
	}
	err = classifyHTTPError(403, []byte("no active Grok subscription"), nil)
	if !IsTierForbidden(err) {
		t.Errorf("403 => %v", err)
	}
	err = classifyHTTPError(429, []byte("slow down"), nil)
	if _, ok := err.(*RateLimitedError); !ok {
		t.Errorf("429 => %T %v", err, err)
	}
}
