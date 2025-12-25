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

	// ============================================================================
	// Tier 2 - Operation Timeouts (multi-step operations that include LLM calls)
	// ============================================================================

	// ShardExecutionTimeout is the timeout for shard spawn and execution.
	// Includes research, context building, LLM call, and post-processing.
	ShardExecutionTimeout time.Duration `json:"shard_execution_timeout"`

	// ArticulationTimeout is the timeout for articulation transducer LLM calls.
	// These convert internal state to user-facing natural language.
	ArticulationTimeout time.Duration `json:"articulation_timeout"`

	// FollowUpTimeout is the timeout for quick follow-up responses.
	// Used for clarification questions and simple responses.
	FollowUpTimeout time.Duration `json:"follow_up_timeout"`

	// OuroborosTimeout is the timeout for the tool generation pipeline.
	// Includes detection, generation, safety check, and simulation stages.
	OuroborosTimeout time.Duration `json:"ouroboros_timeout"`

	// DocumentProcessingTimeout is the timeout for document ingestion and refresh.
	// Includes vector embedding, storage, and knowledge synthesis.
	DocumentProcessingTimeout time.Duration `json:"document_processing_timeout"`

	// ============================================================================
	// Tier 3 - Campaign Timeouts (long-running orchestration)
	// ============================================================================

	// CampaignPhaseTimeout is the timeout for a full campaign phase.
	// Campaign phases may include multiple shard executions.
	CampaignPhaseTimeout time.Duration `json:"campaign_phase_timeout"`

	// OODALoopTimeout is the timeout for the full OODA loop (input processing).
	// Covers Observe, Orient, Decide, Act cycle including perception and articulation.
	OODALoopTimeout time.Duration `json:"ooda_loop_timeout"`
}

// DefaultLLMTimeouts returns sensible defaults for GLM-4.7 with large context windows.
// These values are calibrated for the Z.AI API with 200K context and 128K output tokens.
func DefaultLLMTimeouts() LLMTimeouts {
	return LLMTimeouts{
		// Tier 1 - Per-Call
		HTTPClientTimeout:      10 * time.Minute, // GLM-4.7 needs extended timeout
		SlotAcquisitionTimeout: 10 * time.Minute, // Wait for slow calls to complete
		PerCallTimeout:         10 * time.Minute, // Match HTTP timeout to avoid conflicts
		StreamingTimeout:       15 * time.Minute, // Streaming needs extra time
		RetryBackoffBase:       1 * time.Second,
		RetryBackoffMax:        30 * time.Second,
		MaxRetries:             3,
		RateLimitDelay:         600 * time.Millisecond,

		// Tier 2 - Operation
		// NOTE: Z.AI responses take 150+ seconds minimum for SIMPLE prompts.
		// Complex prompts can take 5-10+ minutes. All values have generous buffers.
		// Bug #3 fix: Increased from 20 to 30 min to account for API slot contention during high load
		ShardExecutionTimeout:     30 * time.Minute, // Shard spawn includes research + LLM
		ArticulationTimeout:       5 * time.Minute,  // Articulation transducer
		FollowUpTimeout:           5 * time.Minute,  // ZAI simple prompts: 150s+
		OuroborosTimeout:          10 * time.Minute, // Tool generation pipeline
		DocumentProcessingTimeout: 20 * time.Minute, // Document ingestion and refresh

		// Tier 3 - Campaign
		CampaignPhaseTimeout: 30 * time.Minute, // Full campaign phase
		OODALoopTimeout:      30 * time.Minute, // Full OODA loop
	}
}

// FastLLMTimeouts returns shorter timeouts for quick operations.
// Use this for simple prompts with small context.
// NOTE: Even "fast" operations need 150+ seconds minimum for ZAI simple prompts.
func FastLLMTimeouts() LLMTimeouts {
	return LLMTimeouts{
		// Tier 1 - Per-Call (5 min for simple ZAI prompts)
		HTTPClientTimeout:      5 * time.Minute,
		SlotAcquisitionTimeout: 6 * time.Minute,
		PerCallTimeout:         5 * time.Minute,
		StreamingTimeout:       6 * time.Minute,
		RetryBackoffBase:       500 * time.Millisecond,
		RetryBackoffMax:        10 * time.Second,
		MaxRetries:             2,
		RateLimitDelay:         600 * time.Millisecond,

		// Tier 2 - Operation (ZAI simple prompts: 150s+)
		ShardExecutionTimeout:     7 * time.Minute,
		ArticulationTimeout:       5 * time.Minute,
		FollowUpTimeout:           5 * time.Minute,
		OuroborosTimeout:          7 * time.Minute,
		DocumentProcessingTimeout: 7 * time.Minute,

		// Tier 3 - Campaign
		CampaignPhaseTimeout: 15 * time.Minute,
		OODALoopTimeout:      15 * time.Minute,
	}
}

// AggressiveLLMTimeouts returns minimal timeouts while respecting ZAI floor.
// ZAI simple prompts take 150s+ minimum, so 5 min floor gives buffer for variance.
func AggressiveLLMTimeouts() LLMTimeouts {
	return LLMTimeouts{
		// Tier 1 - Per-Call (5 min floor = 2x ZAI simple prompt minimum)
		HTTPClientTimeout:      5 * time.Minute,
		SlotAcquisitionTimeout: 6 * time.Minute,
		PerCallTimeout:         5 * time.Minute,
		StreamingTimeout:       5 * time.Minute,
		RetryBackoffBase:       250 * time.Millisecond,
		RetryBackoffMax:        5 * time.Second,
		MaxRetries:             1,
		RateLimitDelay:         600 * time.Millisecond,

		// Tier 2 - Operation (5 min floor = 2x ZAI simple prompt minimum)
		ShardExecutionTimeout:     5 * time.Minute,
		ArticulationTimeout:       5 * time.Minute,
		FollowUpTimeout:           5 * time.Minute,
		OuroborosTimeout:          5 * time.Minute,
		DocumentProcessingTimeout: 5 * time.Minute,

		// Tier 3 - Campaign
		CampaignPhaseTimeout: 10 * time.Minute,
		OODALoopTimeout:      10 * time.Minute,
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
