package xaioauth

import (
	"context"
	"fmt"
	"time"
)

// ProbeClassification is the auth/status taxonomy for SuperGrok OAuth.
type ProbeClassification string

const (
	ProbeReady         ProbeClassification = "ready"
	ProbeLoginRequired ProbeClassification = "login_required"
	ProbeTierForbidden ProbeClassification = "tier_forbidden"
	ProbeRateLimited   ProbeClassification = "rate_limited"
	ProbeFailed        ProbeClassification = "failed"
)

// AuthStatus is the user-facing status label for nerd auth status.
type AuthStatus string

const (
	// AuthStatusOK means SuperGrok OAuth is ready for inference.
	AuthStatusOK AuthStatus = "ok"
	// AuthStatusNeedsReauth means credentials missing/revoked/quarantined.
	AuthStatusNeedsReauth AuthStatus = "needs_reauth"
	// AuthStatusAPIFallback means OAuth is unusable but an xAI API key can cover chat.
	AuthStatusAPIFallback AuthStatus = "api_fallback"
	// AuthStatusDegraded means transient probe failure (rate limit, network).
	AuthStatusDegraded AuthStatus = "degraded"
)

// ProbeResult captures health probe outcome for nerd auth status.
type ProbeResult struct {
	Classification ProbeClassification
	// Status is the simplified user-facing label (ok / needs_reauth / api_fallback / degraded).
	Status         AuthStatus
	Message        string
	Model          string
	CredentialPath string
	Source         string
	ExpiresAt      time.Time
	RawError       string
	// Quarantined is true when local credentials are marked invalid after terminal refresh.
	Quarantined bool
	// ReimportAttempted notes that Load tried Grok CLI import during this probe.
	ReimportAttempted bool
}

// RunHealthProbe verifies credentials and a minimal chat completion.
func (c *Client) RunHealthProbe(ctx context.Context) *ProbeResult {
	result := &ProbeResult{
		Model:          c.cfg.Model,
		CredentialPath: c.cfg.CredentialPath,
	}

	if err := c.tokens.Load(); err != nil {
		result.Classification = ProbeLoginRequired
		result.Status = AuthStatusNeedsReauth
		result.Message = formatLoginRequiredMessage(err)
		result.RawError = err.Error()
		if c.cfg.ImportGrokAuth {
			result.ReimportAttempted = true
		}
		return result
	}

	creds := c.tokens.Credentials()
	if creds != nil {
		result.Source = creds.Source
		result.ExpiresAt = creds.ExpiresAt
		result.Quarantined = creds.Quarantined
		if creds.Quarantined {
			result.Classification = ProbeLoginRequired
			result.Status = AuthStatusNeedsReauth
			result.Message = formatQuarantineMessage(creds.QuarantineReason)
			result.RawError = creds.QuarantineReason
			return result
		}
	}

	// Token resolution (refresh if needed; may re-import on invalid_grant)
	if _, err := c.tokens.AccessToken(ctx); err != nil {
		if IsAuthRequired(err) {
			result.Classification = ProbeLoginRequired
			result.Status = AuthStatusNeedsReauth
			result.Message = formatLoginRequiredMessage(err)
			result.RawError = err.Error()
			if c.cfg.ImportGrokAuth {
				result.ReimportAttempted = true
			}
			// Reflect post-refresh quarantine state
			if c2 := c.tokens.Credentials(); c2 != nil {
				result.Quarantined = c2.Quarantined
				result.Source = c2.Source
			}
			return result
		}
		result.Classification = ProbeFailed
		result.Status = AuthStatusDegraded
		result.Message = "token resolution failed"
		result.RawError = err.Error()
		return result
	}

	// Minimal completion
	probeCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	_, err := c.CompleteWithSystem(probeCtx, "Reply with exactly: ok", "health probe")
	if err != nil {
		result.RawError = err.Error()
		switch {
		case IsAuthRequired(err):
			result.Classification = ProbeLoginRequired
			result.Status = AuthStatusNeedsReauth
			result.Message = formatLoginRequiredMessage(err)
		case IsTierForbidden(err):
			result.Classification = ProbeTierForbidden
			result.Status = AuthStatusNeedsReauth
			result.Message = "OAuth API not entitled for this subscription tier; use engine=api with xai_api_key"
		case isRateLimited(err):
			result.Classification = ProbeRateLimited
			result.Status = AuthStatusDegraded
			result.Message = "rate limited during probe"
		default:
			result.Classification = ProbeFailed
			result.Status = AuthStatusDegraded
			result.Message = fmt.Sprintf("probe completion failed: %v", err)
		}
		return result
	}

	result.Classification = ProbeReady
	result.Status = AuthStatusOK
	result.Message = "SuperGrok OAuth ready"
	return result
}

// DeriveAuthStatus maps probe + API-key availability into a user-facing status.
// When OAuth needs reauth but an API key is present and fallback is enabled,
// reports AuthStatusAPIFallback so `nerd auth status` is honest about boot behavior.
func DeriveAuthStatus(probe *ProbeResult, apiKeyAvailable bool, fallbackEnabled bool) AuthStatus {
	if probe == nil {
		return AuthStatusNeedsReauth
	}
	if probe.Status == AuthStatusOK {
		return AuthStatusOK
	}
	if probe.Classification == ProbeLoginRequired || probe.Classification == ProbeTierForbidden {
		if apiKeyAvailable && fallbackEnabled {
			return AuthStatusAPIFallback
		}
		return AuthStatusNeedsReauth
	}
	if probe.Status != "" {
		return probe.Status
	}
	return AuthStatusDegraded
}

func formatLoginRequiredMessage(err error) string {
	if err == nil {
		return "no SuperGrok OAuth credentials; run: nerd auth grok"
	}
	detail := err.Error()
	// Prefer concise first line for status UI; full help remains in RawError.
	if ae, ok := err.(*AuthRequiredError); ok && ae.Detail != "" {
		if IsTerminalRefreshFailure(ae.Detail) {
			return fmt.Sprintf("refresh revoked (%s); run: nerd auth grok", ae.Detail)
		}
		return fmt.Sprintf("authentication required: %s; run: nerd auth grok", ae.Detail)
	}
	if IsTerminalRefreshFailure(detail) {
		return "refresh token revoked (invalid_grant); run: nerd auth grok"
	}
	return "no SuperGrok OAuth credentials; run: nerd auth grok"
}

func formatQuarantineMessage(reason string) string {
	if reason == "" {
		return "credentials quarantined; re-run: nerd auth grok"
	}
	if IsTerminalRefreshFailure(reason) {
		return fmt.Sprintf("credentials quarantined (%s); re-run: nerd auth grok", reason)
	}
	return fmt.Sprintf("credentials quarantined; re-run: nerd auth grok (%s)", reason)
}

func isRateLimited(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*RateLimitedError)
	return ok
}
