package perception

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ProviderFailureClass is the provider-independent class of a failed model
// call. Every adapter reports HTTP failures in one of a few message shapes
// ("API request failed with status N", "transient server error (N)", "rate
// limit exceeded (429)", "retryable status N"); classifying those shapes in
// one place is what lets a Gemini outage and an Anthropic outage produce the
// same degraded turn. TestProviderFailureConformance drives every HTTP adapter
// through the same responses and pins that each lands in the same class, so an
// adapter that changes its message shape fails there, not in production.
type ProviderFailureClass string

const (
	// FailureTransient: the provider was unavailable (5xx, 408) after the
	// adapter's own retries. Retryable later.
	FailureTransient ProviderFailureClass = "transient"
	// FailureRateLimited: 429 after the adapter's own retries. Retryable later.
	FailureRateLimited ProviderFailureClass = "rate_limited"
	// FailureAuth: 401/403. Retrying cannot help; the credentials must change.
	FailureAuth ProviderFailureClass = "auth"
	// FailureInvalidRequest: 400/404/413/422. The request itself is wrong.
	FailureInvalidRequest ProviderFailureClass = "invalid_request"
	// FailureCanceled: the caller's context was canceled or timed out.
	FailureCanceled ProviderFailureClass = "canceled"
	// FailurePermanent: anything else the adapter reported.
	FailurePermanent ProviderFailureClass = "permanent"
)

// Retryable reports whether the same request may succeed later unchanged.
func (c ProviderFailureClass) Retryable() bool {
	return c == FailureTransient || c == FailureRateLimited
}

var providerStatusPattern = regexp.MustCompile(`(?:status|error|exceeded) \(?(\d{3})\)?`)

// providerFailureStatus extracts the HTTP status an adapter reported, or 0.
func providerFailureStatus(err error) int {
	if err == nil {
		return 0
	}
	m := providerStatusPattern.FindStringSubmatch(err.Error())
	if m == nil {
		return 0
	}
	code, convErr := strconv.Atoi(m[1])
	if convErr != nil {
		return 0
	}
	return code
}

// ClassifyProviderFailure maps a model-call error to its class.
func ClassifyProviderFailure(err error) ProviderFailureClass {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return FailureCanceled
	}
	var rateLimitErr *RateLimitError
	if errors.As(err, &rateLimitErr) && rateLimitErr != nil {
		return FailureRateLimited
	}
	switch status := providerFailureStatus(err); {
	case status == 429:
		return FailureRateLimited
	case status == 401 || status == 403:
		return FailureAuth
	case status == 400 || status == 404 || status == 413 || status == 422:
		return FailureInvalidRequest
	case status == 408 || (status >= 500 && status != 501):
		return FailureTransient
	}
	if errors.Is(err, ErrLLMUnavailable) {
		return FailureTransient
	}
	return FailurePermanent
}

// IsTransientProviderFailure reports whether err is an outage the user should
// be told to retry rather than a failure to understand them. It holds for
// every adapter, not only for errors that wrap ErrLLMUnavailable.
func IsTransientProviderFailure(err error) bool {
	return ClassifyProviderFailure(err).Retryable()
}

// SafeProviderFailureMessage describes a failure for a user or a log line
// without the provider's response body, which can echo request content,
// account identifiers or keys. It names the class and, when known, the
// status -- nothing the provider wrote.
func SafeProviderFailureMessage(err error) string {
	class := ClassifyProviderFailure(err)
	if class == "" {
		return ""
	}
	return safeFailureMessage(class, providerFailureStatus(err))
}

// safeFailureMessage is SafeProviderFailureMessage for a failure already
// reduced to its class and status, the form a ClassificationOutcome keeps.
func safeFailureMessage(class ProviderFailureClass, status int) string {
	switch class {
	case FailureCanceled:
		return "the model call was canceled before it finished"
	case FailureAuth:
		return fmt.Sprintf("the model provider rejected the credentials (HTTP %d); check the configured API key", status)
	case FailureRateLimited:
		return "the model provider is rate limiting requests (HTTP 429); try again shortly"
	case FailureTransient:
		if status != 0 {
			return fmt.Sprintf("the model provider is temporarily unavailable (HTTP %d)", status)
		}
		return "the model provider is temporarily unavailable"
	case FailureInvalidRequest:
		return fmt.Sprintf("the model provider rejected the request (HTTP %d)", status)
	default:
		if status != 0 {
			return fmt.Sprintf("the model call failed (HTTP %d)", status)
		}
		return "the model call failed"
	}
}

// UserFailureText is err's text for a user: the message of err with the
// provider's own diagnostic -- the error type, message and request id a pooled
// or Anthropic failure carries, which can name request content -- replaced by
// SafeProviderFailureMessage, and everything the caller wrapped around it kept.
// The full text stays on the error for logs and problems.log; this is the form
// a terminal, a chat surface or a CLI result prints.
func UserFailureText(err error) string {
	if err == nil {
		return ""
	}
	text := err.Error()
	var detailed error
	var pooled *pooledCallError
	var anthropic *AnthropicAPIError
	switch {
	case errors.As(err, &pooled):
		detailed = pooled
	case errors.As(err, &anthropic):
		detailed = anthropic
	default:
		return text
	}
	return strings.Replace(text, detailed.Error(), SafeProviderFailureMessage(detailed), 1)
}
