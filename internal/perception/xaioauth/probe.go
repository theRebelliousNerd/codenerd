package xaioauth

import (
	"context"
	"fmt"
	"time"
)

// ProbeClassification is the auth/status taxonomy for SuperGrok OAuth.
type ProbeClassification string

const (
	ProbeReady          ProbeClassification = "ready"
	ProbeLoginRequired  ProbeClassification = "login_required"
	ProbeTierForbidden  ProbeClassification = "tier_forbidden"
	ProbeRateLimited    ProbeClassification = "rate_limited"
	ProbeFailed         ProbeClassification = "failed"
)

// ProbeResult captures health probe outcome for nerd auth status.
type ProbeResult struct {
	Classification ProbeClassification
	Message        string
	Model          string
	CredentialPath string
	Source         string
	ExpiresAt      time.Time
	RawError       string
}

// RunHealthProbe verifies credentials and a minimal chat completion.
func (c *Client) RunHealthProbe(ctx context.Context) *ProbeResult {
	result := &ProbeResult{
		Model:          c.cfg.Model,
		CredentialPath: c.cfg.CredentialPath,
	}

	if err := c.tokens.Load(); err != nil {
		result.Classification = ProbeLoginRequired
		result.Message = "no SuperGrok OAuth credentials; run: nerd auth grok"
		result.RawError = err.Error()
		return result
	}

	creds := c.tokens.Credentials()
	if creds != nil {
		result.Source = creds.Source
		result.ExpiresAt = creds.ExpiresAt
		if creds.Quarantined {
			result.Classification = ProbeLoginRequired
			result.Message = "credentials quarantined; re-run: nerd auth grok"
			result.RawError = creds.QuarantineReason
			return result
		}
	}

	// Token resolution (refresh if needed)
	if _, err := c.tokens.AccessToken(ctx); err != nil {
		if IsAuthRequired(err) {
			result.Classification = ProbeLoginRequired
			result.Message = err.Error()
			result.RawError = err.Error()
			return result
		}
		result.Classification = ProbeFailed
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
			result.Message = "authentication required"
		case IsTierForbidden(err):
			result.Classification = ProbeTierForbidden
			result.Message = "OAuth API not entitled for this subscription tier; use engine=api with xai_api_key"
		case isRateLimited(err):
			result.Classification = ProbeRateLimited
			result.Message = "rate limited during probe"
		default:
			result.Classification = ProbeFailed
			result.Message = fmt.Sprintf("probe completion failed: %v", err)
		}
		return result
	}

	result.Classification = ProbeReady
	result.Message = "SuperGrok OAuth ready"
	return result
}

func isRateLimited(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*RateLimitedError)
	return ok
}
