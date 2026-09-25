package config

import (
	"fmt"
	"time"

	"codenerd/internal/sqlpragmas"
)

// CoreLimits enforces system-wide resource constraints.
type CoreLimits struct {
	MaxTotalMemoryMB      int `yaml:"max_total_memory_mb" json:"max_total_memory_mb"`           // Total RAM limit
	MaxConcurrentShards   int `yaml:"max_concurrent_shards" json:"max_concurrent_shards"`       // Max parallel shards
	MaxConcurrentAPICalls int `yaml:"max_concurrent_api_calls" json:"max_concurrent_api_calls"` // Max simultaneous LLM API calls
	MaxFactsInKernel      int `yaml:"max_facts_in_kernel" json:"max_facts_in_kernel"`           // EDB size limit
	MaxDerivedFactsLimit  int `yaml:"max_derived_facts_limit" json:"max_derived_facts_limit"`   // Mangle gas limit (Bug #17)

	// SQLHostClass declares the machine class SQLite's page cache and mmap
	// window are sized for: "workstation" (the default), "laptop" (1/4) or
	// "micro" (1/16; containers, CI). internal/sqlpragmas owns the scaling and
	// does not import config, so boot pushes this value down
	// (internal/system configureSQLPragmas). The NERD_SQL_HOST_CLASS
	// environment variable, when set, wins over this key. A declared class
	// rather than one detected from free RAM keeps SQLite's behaviour the same
	// on every machine that shares a config.
	SQLHostClass string `yaml:"sql_host_class,omitempty" json:"sql_host_class,omitempty"`

	// There is deliberately no tool-call, tool-round or session-time limit
	// here. Until 2026-09-18 this struct carried max_tool_calls,
	// max_tool_iterations, adaptive_tool_budget, tool_iteration_extension_size,
	// max_tool_iteration_extensions and tool_loop_repeat_threshold, and until
	// 2026-09-19 max_session_duration_min (a 2-hour session ceiling that
	// nothing called yet). A turn continues while the working policy
	// (internal/context/working_set.mg) derives no working_stop over the facts
	// the tool loop asserts; the only wall clock on a run is the user's own
	// --timeout. removedCoreLimitKeys (removed_keys.go) rejects the old keys by
	// name so a config that still sets one fails to load instead of silently
	// meaning nothing.
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
func (c *CoreLimits) ValidateCoreLimits() error {
	if c.MaxTotalMemoryMB < 512 {
		return fmt.Errorf("max_total_memory_mb must be >= 512 MB")
	}
	if c.MaxConcurrentShards < 1 {
		return fmt.Errorf("max_concurrent_shards must be >= 1")
	}
	if c.MaxFactsInKernel < 1000 {
		return fmt.Errorf("max_facts_in_kernel must be >= 1000")
	}
	if c.MaxDerivedFactsLimit < 1000 {
		return fmt.Errorf("max_derived_facts_limit must be >= 1000")
	}
	if c.SQLHostClass != "" {
		if _, ok := sqlpragmas.ParseHostClass(c.SQLHostClass); !ok {
			return fmt.Errorf("sql_host_class %q is not a host class (workstation, laptop or micro)", c.SQLHostClass)
		}
	}
	return nil
}

// DefaultCoreLimits returns a CoreLimits with sensible defaults.
func DefaultCoreLimits() *CoreLimits {
	return &CoreLimits{
		MaxTotalMemoryMB:      12288,
		MaxConcurrentShards:   12,
		MaxConcurrentAPICalls: 5,
		MaxFactsInKernel:      2000000,
		MaxDerivedFactsLimit:  5000000,
	}
}
