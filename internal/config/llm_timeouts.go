package config

import "time"

// LLMTimeouts centralizes the timeouts that bound a single LLM request.
// This ensures consistency across the codebase and prevents timeout conflicts.
//
// KEY INSIGHT: In Go, the SHORTEST timeout in the chain wins.
// If you have a 10-minute HTTP client but wrap the call in a 90-second context,
// the context wins and the call fails after 90 seconds.
//
// Every field bounds one request (a call, its retries, a slot wait, a single
// articulation or follow-up call). Nothing here bounds a run: until 2026-09-19
// this struct also carried shard-execution, OODA-loop, campaign-phase,
// document-processing and Ouroboros ceilings of 10-30 minutes ("calibrated for
// GLM-4.7"), which cut hours-long agentic work that was still progressing. A
// run now stops when the working policy derives a stall or a repeated failure,
// or when the user's own --timeout expires.
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

	// ArticulationTimeout bounds one articulation transducer call, which
	// converts internal state to user-facing natural language.
	ArticulationTimeout time.Duration `json:"articulation_timeout"`

	// FollowUpTimeout bounds one quick follow-up call (a clarification
	// question or a simple response).
	FollowUpTimeout time.Duration `json:"follow_up_timeout"`
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
		ArticulationTimeout:    5 * time.Minute, // Articulation transducer
		FollowUpTimeout:        5 * time.Minute, // ZAI simple prompts: 150s+
	}
}

// FastLLMTimeouts returns shorter timeouts for quick operations.
// Use this for simple prompts with small context.
// NOTE: Even "fast" operations need 150+ seconds minimum for ZAI simple prompts.
func FastLLMTimeouts() LLMTimeouts {
	return LLMTimeouts{
		HTTPClientTimeout:      5 * time.Minute,
		SlotAcquisitionTimeout: 6 * time.Minute,
		PerCallTimeout:         5 * time.Minute,
		StreamingTimeout:       6 * time.Minute,
		RetryBackoffBase:       500 * time.Millisecond,
		RetryBackoffMax:        10 * time.Second,
		MaxRetries:             2,
		RateLimitDelay:         600 * time.Millisecond,
		ArticulationTimeout:    5 * time.Minute,
		FollowUpTimeout:        5 * time.Minute,
	}
}

// AggressiveLLMTimeouts returns minimal timeouts while respecting ZAI floor.
// ZAI simple prompts take 150s+ minimum, so 5 min floor gives buffer for variance.
func AggressiveLLMTimeouts() LLMTimeouts {
	return LLMTimeouts{
		HTTPClientTimeout:      5 * time.Minute,
		SlotAcquisitionTimeout: 6 * time.Minute,
		PerCallTimeout:         5 * time.Minute,
		StreamingTimeout:       5 * time.Minute,
		RetryBackoffBase:       250 * time.Millisecond,
		RetryBackoffMax:        5 * time.Second,
		MaxRetries:             1,
		RateLimitDelay:         600 * time.Millisecond,
		ArticulationTimeout:    5 * time.Minute,
		FollowUpTimeout:        5 * time.Minute,
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
