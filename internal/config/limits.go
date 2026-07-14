package config

import (
	"fmt"
	"time"
)

// CoreLimits enforces system-wide resource constraints.
type CoreLimits struct {
	MaxTotalMemoryMB      int `yaml:"max_total_memory_mb" json:"max_total_memory_mb"`           // Total RAM limit
	MaxConcurrentShards   int `yaml:"max_concurrent_shards" json:"max_concurrent_shards"`       // Max parallel shards
	MaxConcurrentAPICalls int `yaml:"max_concurrent_api_calls" json:"max_concurrent_api_calls"` // Max simultaneous LLM API calls
	MaxSessionDurationMin int `yaml:"max_session_duration_min" json:"max_session_duration_min"` // Auto-save interval
	MaxFactsInKernel      int `yaml:"max_facts_in_kernel" json:"max_facts_in_kernel"`           // EDB size limit
	MaxDerivedFactsLimit  int `yaml:"max_derived_facts_limit" json:"max_derived_facts_limit"`   // Mangle gas limit (Bug #17)
}

// APISchedulerPolicy is user-facing configuration for the cooperative LLM API
// scheduler (priority queue, spacing, adaptive concurrency on rate limits).
// All pointer fields mean "use engine default when omitted".
//
// Example (.nerd/config.json):
//
//	"api_scheduler": {
//	  "min_call_spacing_ms": 150,
//	  "adaptive_concurrency": true,
//	  "adaptive_floor": 1,
//	  "adaptive_recover_after_sec": 30,
//	  "slot_acquire_timeout_sec": 300
//	}
//
// Concurrency ceiling still comes from core_limits.max_concurrent_api_calls
// (and engine overrides like xai_oauth.max_concurrent_calls / codex_cli.max_concurrent_calls).
type APISchedulerPolicy struct {
	// MinCallSpacingMs is the minimum gap between successive slot grants.
	// Subscription engines default to 150; api engine defaults to 0.
	MinCallSpacingMs *int `json:"min_call_spacing_ms,omitempty" yaml:"min_call_spacing_ms,omitempty"`

	// AdaptiveConcurrency enables shrink-on-429 / recover-after-success.
	// Subscription engines default to true; api defaults to false.
	AdaptiveConcurrency *bool `json:"adaptive_concurrency,omitempty" yaml:"adaptive_concurrency,omitempty"`

	// AdaptiveFloor is the minimum slots when throttled (default 1).
	AdaptiveFloor *int `json:"adaptive_floor,omitempty" yaml:"adaptive_floor,omitempty"`

	// AdaptiveRecoverAfterSec is quiet time without rate limits before restoring
	// one slot toward the configured max (default 30).
	AdaptiveRecoverAfterSec *int `json:"adaptive_recover_after_sec,omitempty" yaml:"adaptive_recover_after_sec,omitempty"`

	// SlotAcquireTimeoutSec is max wait for an API slot (default from LLM timeouts / 300).
	SlotAcquireTimeoutSec *int `json:"slot_acquire_timeout_sec,omitempty" yaml:"slot_acquire_timeout_sec,omitempty"`
}

// EffectiveAPISchedulerPolicy is the fully resolved scheduler policy ready for
// the core APIScheduler.
type EffectiveAPISchedulerPolicy struct {
	MaxConcurrentAPICalls int
	MinCallSpacing        time.Duration
	AdaptiveConcurrency   bool
	AdaptiveFloor         int
	AdaptiveRecoverAfter  time.Duration
	SlotAcquireTimeout    time.Duration
}

// Default subscription-engine spacing / adaptive knobs.
const (
	DefaultSubscriptionMinCallSpacingMs = 150
	DefaultAdaptiveFloor                = 1
	DefaultAdaptiveRecoverAfterSec      = 30
	DefaultSlotAcquireTimeoutSec        = 300
)

// ValidateCoreLimits checks that core limits are within acceptable ranges.
func (c *Config) ValidateCoreLimits() error {
	if c.CoreLimits.MaxTotalMemoryMB < 512 {
		return fmt.Errorf("max_total_memory_mb must be >= 512 MB")
	}
	if c.CoreLimits.MaxConcurrentShards < 1 {
		return fmt.Errorf("max_concurrent_shards must be >= 1")
	}
	if c.CoreLimits.MaxFactsInKernel < 1000 {
		return fmt.Errorf("max_facts_in_kernel must be >= 1000")
	}
	if c.CoreLimits.MaxDerivedFactsLimit < 1000 {
		return fmt.Errorf("max_derived_facts_limit must be >= 1000")
	}
	return nil
}

// EnforceCoreLimits returns enforcement parameters for the kernel.
// This ensures config values are actually used, not just stored.
func (c *Config) EnforceCoreLimits() map[string]int {
	return map[string]int{
		"max_facts":        c.CoreLimits.MaxFactsInKernel,
		"max_derived":      c.CoreLimits.MaxDerivedFactsLimit,
		"max_shards":       c.CoreLimits.MaxConcurrentShards,
		"max_memory_mb":    c.CoreLimits.MaxTotalMemoryMB,
		"session_duration": c.CoreLimits.MaxSessionDurationMin,
	}
}

// DefaultCoreLimits returns a CoreLimits with sensible defaults.
func DefaultCoreLimits() *CoreLimits {
	return &CoreLimits{
		MaxTotalMemoryMB:      12288,
		MaxConcurrentShards:   12,
		MaxConcurrentAPICalls: 5,
		MaxSessionDurationMin: 120,
		MaxFactsInKernel:      250000,
		MaxDerivedFactsLimit:  100000,
	}
}
