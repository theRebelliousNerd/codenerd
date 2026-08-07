package config

import (
	"fmt"
	"time"
)

// LLMTimeoutsConfig is the `.nerd/config.json` surface for LLMTimeouts.
//
// Why this type exists separately from LLMTimeouts: the runtime struct uses
// time.Duration, which encoding/json marshals as an integer count of
// NANOSECONDS ("shard_execution_timeout": 1800000000000). That is unreadable
// and effectively unwritable by hand, so the config surface takes strings that
// time.ParseDuration understands ("30m", "90s", "1500ms").
//
// Before this existed, every one of the ~25 GetLLMTimeouts() call sites — the
// OODA loop, shard execution, campaign phases, articulation, follow-up,
// document ingestion — ran on a package-level singleton with no way to change
// it. Worse, those defaults are documented as "calibrated for the Z.AI API"
// and "GLM-4.7", so anyone on a different provider inherited another vendor's
// latency profile: a hung call blocked for 30 minutes instead of failing fast.
// FastLLMTimeouts and AggressiveLLMTimeouts existed but were reachable only
// from tests; Profile makes them selectable.
type LLMTimeoutsConfig struct {
	// Profile picks the baseline before per-field overrides are applied:
	// "default" (Z.AI/GLM calibrated, generous), "fast", or "aggressive".
	// Empty means "default".
	Profile string `json:"profile,omitempty"`

	// --- Tier 1: per-call ---

	// HTTPClientTimeout bounds a single HTTP request to the provider.
	HTTPClientTimeout string `json:"http_client_timeout,omitempty"`
	// SlotAcquisitionTimeout bounds the wait for an APIScheduler slot.
	SlotAcquisitionTimeout string `json:"slot_acquisition_timeout,omitempty"`
	// PerCallTimeout bounds one logical LLM call including retries.
	PerCallTimeout string `json:"per_call_timeout,omitempty"`
	// StreamingTimeout bounds a streaming response end to end.
	StreamingTimeout string `json:"streaming_timeout,omitempty"`
	// RetryBackoffBase is the first retry delay; it grows toward the max.
	RetryBackoffBase string `json:"retry_backoff_base,omitempty"`
	// RetryBackoffMax caps the retry delay.
	RetryBackoffMax string `json:"retry_backoff_max,omitempty"`
	// RateLimitDelay is the pause applied after a 429.
	RateLimitDelay string `json:"rate_limit_delay,omitempty"`
	// MaxRetries is a pointer so an explicit 0 ("never retry") is
	// distinguishable from "key absent, use the profile value".
	MaxRetries *int `json:"max_retries,omitempty"`

	// --- Tier 2: operation ---

	// ShardExecutionTimeout bounds one shard's full run.
	ShardExecutionTimeout string `json:"shard_execution_timeout,omitempty"`
	// ArticulationTimeout bounds the articulation transducer.
	ArticulationTimeout string `json:"articulation_timeout,omitempty"`
	// FollowUpTimeout bounds a follow-up turn.
	FollowUpTimeout string `json:"follow_up_timeout,omitempty"`
	// OuroborosTimeout bounds the tool-generation pipeline.
	OuroborosTimeout string `json:"ouroboros_timeout,omitempty"`
	// DocumentProcessingTimeout bounds document ingestion/refresh.
	DocumentProcessingTimeout string `json:"document_processing_timeout,omitempty"`

	// --- Tier 3: campaign ---

	// CampaignPhaseTimeout bounds one campaign phase.
	CampaignPhaseTimeout string `json:"campaign_phase_timeout,omitempty"`
	// OODALoopTimeout bounds one full Observe-Orient-Decide-Act loop.
	OODALoopTimeout string `json:"ooda_loop_timeout,omitempty"`
}

// baseProfile returns the starting timeouts for the named profile.
func baseProfile(name string) (LLMTimeouts, error) {
	switch name {
	case "", "default":
		return DefaultLLMTimeouts(), nil
	case "fast":
		return FastLLMTimeouts(), nil
	case "aggressive":
		return AggressiveLLMTimeouts(), nil
	default:
		return LLMTimeouts{}, fmt.Errorf(
			"llm_timeouts.profile %q is not recognised (want \"default\", \"fast\", or \"aggressive\")", name)
	}
}

// applyDuration parses raw into dst when raw is non-empty. A malformed value is
// an error rather than a silent fallback: a timeout that quietly stays at 30
// minutes because of a typo is exactly the failure this config exists to fix.
func applyDuration(field, raw string, dst *time.Duration) error {
	if raw == "" {
		return nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return fmt.Errorf("llm_timeouts.%s: %q is not a duration (want e.g. \"30m\", \"90s\", \"1500ms\"): %w", field, raw, err)
	}
	if d <= 0 {
		return fmt.Errorf("llm_timeouts.%s: %q must be positive", field, raw)
	}
	*dst = d
	return nil
}

// Resolve builds the runtime timeouts: the chosen profile, with every
// explicitly set field overlaid on top.
func (c *LLMTimeoutsConfig) Resolve() (LLMTimeouts, error) {
	if c == nil {
		return DefaultLLMTimeouts(), nil
	}
	t, err := baseProfile(c.Profile)
	if err != nil {
		return LLMTimeouts{}, err
	}

	fields := []struct {
		name string
		raw  string
		dst  *time.Duration
	}{
		{"http_client_timeout", c.HTTPClientTimeout, &t.HTTPClientTimeout},
		{"slot_acquisition_timeout", c.SlotAcquisitionTimeout, &t.SlotAcquisitionTimeout},
		{"per_call_timeout", c.PerCallTimeout, &t.PerCallTimeout},
		{"streaming_timeout", c.StreamingTimeout, &t.StreamingTimeout},
		{"retry_backoff_base", c.RetryBackoffBase, &t.RetryBackoffBase},
		{"retry_backoff_max", c.RetryBackoffMax, &t.RetryBackoffMax},
		{"rate_limit_delay", c.RateLimitDelay, &t.RateLimitDelay},
		{"shard_execution_timeout", c.ShardExecutionTimeout, &t.ShardExecutionTimeout},
		{"articulation_timeout", c.ArticulationTimeout, &t.ArticulationTimeout},
		{"follow_up_timeout", c.FollowUpTimeout, &t.FollowUpTimeout},
		{"ouroboros_timeout", c.OuroborosTimeout, &t.OuroborosTimeout},
		{"document_processing_timeout", c.DocumentProcessingTimeout, &t.DocumentProcessingTimeout},
		{"campaign_phase_timeout", c.CampaignPhaseTimeout, &t.CampaignPhaseTimeout},
		{"ooda_loop_timeout", c.OODALoopTimeout, &t.OODALoopTimeout},
	}
	for _, f := range fields {
		if err := applyDuration(f.name, f.raw, f.dst); err != nil {
			return LLMTimeouts{}, err
		}
	}

	if c.MaxRetries != nil {
		if *c.MaxRetries < 0 {
			return LLMTimeouts{}, fmt.Errorf("llm_timeouts.max_retries: %d must be >= 0", *c.MaxRetries)
		}
		t.MaxRetries = *c.MaxRetries
	}
	return t, nil
}
