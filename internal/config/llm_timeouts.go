package config

import "time"

// LLMTimeouts centralizes all timeout configuration for LLM operations.
// This ensures consistency across the codebase and prevents timeout conflicts.
//
// KEY INSIGHT: In Go, the SHORTEST timeout in the chain wins.
// If you have a 10-minute HTTP client but wrap the call in a 90-second context,
// the context wins and the call fails after 90 seconds.
//
// This configuration provides canonical timeouts that all LLM operations should use.
type LLMTimeouts struct {
	// HTTPClientTimeout is the maximum time for HTTP operations including
	// connection, TLS handshake, and full response body read.
	// GLM-4.7 with 160K+ context windows can take 2-4+ minutes.
	HTTPClientTimeout time.Duration `json:"http_client_timeout"`

	// SlotAcquisitionTimeout is the max time to wait for an API scheduler slot.
	// This should be at least as long as HTTPClientTimeout since a call
	// might need to wait for another call to complete.
	SlotAcquisitionTimeout time.Duration `json:"slot_acquisition_timeout"`

	// PerCallTimeout is the default timeout for a single LLM call context.
	// This wraps the actual API call and should match HTTPClientTimeout.
	PerCallTimeout time.Duration `json:"per_call_timeout"`

	// StreamingTimeout is the timeout for streaming LLM calls.
	// Streaming calls may take longer as they receive data incrementally.
	StreamingTimeout time.Duration `json:"streaming_timeout"`

	// RetryBackoffBase is the base duration for exponential backoff between retries.
	RetryBackoffBase time.Duration `json:"retry_backoff_base"`

	// RetryBackoffMax is the maximum backoff duration.
	RetryBackoffMax time.Duration `json:"retry_backoff_max"`

	// MaxRetries is the default number of retry attempts for transient failures.
	MaxRetries int `json:"max_retries"`

	// RateLimitDelay is the minimum delay between consecutive API calls.
	// Z.AI recommends 600ms between requests.
	RateLimitDelay time.Duration `json:"rate_limit_delay"`
}

// DefaultLLMTimeouts returns sensible defaults for GLM-4.7 with large context windows.
// These values are calibrated for the Z.AI API with 200K context and 128K output tokens.
func DefaultLLMTimeouts() LLMTimeouts {
	return LLMTimeouts{
		HTTPClientTimeout:      10 * time.Minute, // GLM-4.7 needs extended timeout
		SlotAcquisitionTimeout: 10 * time.Minute, // Wait for slow calls to complete
		PerCallTimeout:         10 * time.Minute, // Match HTTP timeout to avoid conflicts
		StreamingTimeout:       15 * time.Minute, // Streaming needs extra time
		RetryBackoffBase:       1 * time.Second,
		RetryBackoffMax:        30 * time.Second,
		MaxRetries:             3,
		RateLimitDelay:         600 * time.Millisecond,
	}
}

// FastLLMTimeouts returns shorter timeouts for quick operations.
// Use this for simple prompts with small context.
func FastLLMTimeouts() LLMTimeouts {
	return LLMTimeouts{
		HTTPClientTimeout:      2 * time.Minute,
		SlotAcquisitionTimeout: 3 * time.Minute,
		PerCallTimeout:         2 * time.Minute,
		StreamingTimeout:       3 * time.Minute,
		RetryBackoffBase:       500 * time.Millisecond,
		RetryBackoffMax:        10 * time.Second,
		MaxRetries:             2,
		RateLimitDelay:         600 * time.Millisecond,
	}
}

// AggressiveLLMTimeouts returns timeouts for time-sensitive operations.
// Use sparingly - may cause failures with large context.
func AggressiveLLMTimeouts() LLMTimeouts {
	return LLMTimeouts{
		HTTPClientTimeout:      1 * time.Minute,
		SlotAcquisitionTimeout: 2 * time.Minute,
		PerCallTimeout:         1 * time.Minute,
		StreamingTimeout:       2 * time.Minute,
		RetryBackoffBase:       250 * time.Millisecond,
		RetryBackoffMax:        5 * time.Second,
		MaxRetries:             1,
		RateLimitDelay:         600 * time.Millisecond,
	}
}

// Global singleton for consistent timeout access.
var globalLLMTimeouts = DefaultLLMTimeouts()

// GetLLMTimeouts returns the global LLM timeout configuration.
func GetLLMTimeouts() LLMTimeouts {
	return globalLLMTimeouts
}

// SetLLMTimeouts updates the global LLM timeout configuration.
// This should be called early in application startup.
func SetLLMTimeouts(t LLMTimeouts) {
	globalLLMTimeouts = t
}
