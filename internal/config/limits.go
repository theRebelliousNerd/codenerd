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

	// MaxToolCalls caps tool executions in a single turn. 0 uses the code
	// default (50).
	MaxToolCalls int `yaml:"max_tool_calls" json:"max_tool_calls,omitempty"`

	// MaxToolIterations caps LLM -> tools -> LLM round trips in a single turn.
	// 0 uses the code default (8).
	//
	// This is the knob that decides how much investigation a turn may do before
	// the executor forces a conclusion. Both limits were hardcoded in
	// session.DefaultExecutorConfig with no way to raise them, and 8 is low for
	// real work: a `nerd create <architecture doc>` turn spent its whole budget
	// reading source and hit the ceiling before writing anything. Raise these
	// for research-heavy or documentation work; lower them to cap spend.
	MaxToolIterations int `yaml:"max_tool_iterations" json:"max_tool_iterations,omitempty"`

	// AdaptiveToolBudget lets the session orchestrator extend the iteration
	// ceiling when deterministic telemetry shows novel successful work and no
	// repeated tool-call cycle. Nil means enabled.
	AdaptiveToolBudget *bool `yaml:"adaptive_tool_budget" json:"adaptive_tool_budget,omitempty"`

	// ToolIterationExtensionSize is the number of additional LLM -> tool rounds
	// in one progress extension. Zero uses the code default (8).
	ToolIterationExtensionSize int `yaml:"tool_iteration_extension_size" json:"tool_iteration_extension_size,omitempty"`

	// MaxToolIterationExtensions caps progress extensions per turn. Zero uses
	// the code default (2). Set AdaptiveToolBudget=false to disable extensions.
	MaxToolIterationExtensions int `yaml:"max_tool_iteration_extensions" json:"max_tool_iteration_extensions,omitempty"`

	// ToolLoopRepeatThreshold is the number of identical deterministic trace
	// cycles that marks a loop and blocks further extensions. Zero uses 2.
	ToolLoopRepeatThreshold int `yaml:"tool_loop_repeat_threshold" json:"tool_loop_repeat_threshold,omitempty"`
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
	if c.MaxToolCalls < 0 {
		return fmt.Errorf("max_tool_calls must be >= 0")
	}
	if c.MaxToolIterations < 0 {
		return fmt.Errorf("max_tool_iterations must be >= 0")
	}
	if c.ToolIterationExtensionSize < 0 || c.ToolIterationExtensionSize > 64 {
		return fmt.Errorf("tool_iteration_extension_size must be between 0 and 64")
	}
	if c.MaxToolIterationExtensions < 0 || c.MaxToolIterationExtensions > 8 {
		return fmt.Errorf("max_tool_iteration_extensions must be between 0 and 8")
	}
	if threshold := c.ToolLoopRepeatThreshold; threshold != 0 && (threshold < 2 || threshold > 8) {
		return fmt.Errorf("tool_loop_repeat_threshold must be 0 or between 2 and 8")
	}
	return nil
}

// DefaultCoreLimits returns a CoreLimits with sensible defaults.
func DefaultCoreLimits() *CoreLimits {
	adaptiveToolBudget := true
	return &CoreLimits{
		MaxTotalMemoryMB:           12288,
		MaxConcurrentShards:        12,
		MaxConcurrentAPICalls:      5,
		MaxSessionDurationMin:      120,
		MaxFactsInKernel:           250000,
		MaxDerivedFactsLimit:       100000,
		AdaptiveToolBudget:         &adaptiveToolBudget,
		ToolIterationExtensionSize: 8,
		MaxToolIterationExtensions: 2,
		ToolLoopRepeatThreshold:    2,
	}
}
