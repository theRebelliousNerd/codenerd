package xaioauth

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Sentinel / typed errors for SuperGrok OAuth.

var (
	// ErrAuthRequired means the user must run device-code login again.
	ErrAuthRequired = errors.New("xai-oauth: authentication required (run: nerd auth grok)")

	// ErrNoCredentials means no usable tokens were found in store or Grok CLI import.
	ErrNoCredentials = errors.New("xai-oauth: no credentials found")

	// ErrQuarantined means a prior refresh failed terminally; force re-login.
	ErrQuarantined = errors.New("xai-oauth: credentials quarantined; re-authenticate")

	// ErrTierForbidden means OAuth inference is not entitled for this subscription tier.
	ErrTierForbidden = errors.New("xai-oauth: subscription tier does not allow OAuth API access (try engine=api with xai_api_key)")
)

// AuthRequiredError wraps login-required failures with optional detail.
type AuthRequiredError struct {
	Detail string
}

func (e *AuthRequiredError) Error() string {
	base := ErrAuthRequired.Error()
	if e.Detail == "" {
		return base + "\n" + AuthRecoveryHelp("")
	}
	return fmt.Sprintf("%s: %s\n%s", base, e.Detail, AuthRecoveryHelp(e.Detail))
}

func (e *AuthRequiredError) Unwrap() error { return ErrAuthRequired }

// AuthRecoveryHelp returns actionable recovery steps for SuperGrok OAuth failures.
func AuthRecoveryHelp(detail string) string {
	revoked := strings.Contains(strings.ToLower(detail), "invalid_grant") ||
		strings.Contains(strings.ToLower(detail), "revoked") ||
		strings.Contains(strings.ToLower(detail), "quarantine")
	var b strings.Builder
	b.WriteString("  Fix: run `nerd auth grok` (device login or re-import from ~/.grok/auth.json).\n")
	if revoked {
		b.WriteString("  Quarantined/revoked tokens: delete ~/.nerd/xai_oauth.json then re-auth,\n")
		b.WriteString("  or log in again with the Grok CLI (`grok login`) and re-run `nerd auth grok`.\n")
	}
	b.WriteString("  Or: set engine=api with xai_api_key (metered) if SuperGrok OAuth is unavailable.")
	return b.String()
}

// TierForbiddenError indicates HTTP 403 entitlement/tier gating.
type TierForbiddenError struct {
	StatusCode int
	Body       string
}

func (e *TierForbiddenError) Error() string {
	return fmt.Sprintf("%s (status=%d)", ErrTierForbidden.Error(), e.StatusCode)
}

func (e *TierForbiddenError) Unwrap() error { return ErrTierForbidden }

// RateLimitedError indicates subscription/API rate limiting.
type RateLimitedError struct {
	RetryAfter time.Duration
	Body       string
}

func (e *RateLimitedError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("xai-oauth: rate limit exceeded, retry after %v", e.RetryAfter)
	}
	return "xai-oauth: rate limit exceeded"
}

// RefreshFailedError is returned when token refresh fails.
type RefreshFailedError struct {
	Terminal bool
	Detail   string
}

func (e *RefreshFailedError) Error() string {
	kind := "transient"
	if e.Terminal {
		kind = "terminal"
	}
	return fmt.Sprintf("xai-oauth: token refresh failed (%s): %s", kind, e.Detail)
}

// IsAuthRequired reports whether err requires re-login.
func IsAuthRequired(err error) bool {
	return errors.Is(err, ErrAuthRequired) || errors.Is(err, ErrNoCredentials) || errors.Is(err, ErrQuarantined)
}

// IsTierForbidden reports whether err is a subscription tier gate.
func IsTierForbidden(err error) bool {
	return errors.Is(err, ErrTierForbidden)
}

// IsTerminalRefreshFailure reports invalid_grant / revoked-style OAuth failures.
func IsTerminalRefreshFailure(detail string) bool {
	d := strings.ToLower(detail)
	return strings.Contains(d, "invalid_grant") ||
		strings.Contains(d, "revoked") ||
		strings.Contains(d, "invalid_token") ||
		strings.Contains(d, "expired_token")
}
