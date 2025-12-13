package config

import "fmt"

// CoreLimits enforces system-wide resource constraints.
type CoreLimits struct {
	MaxTotalMemoryMB      int `yaml:"max_total_memory_mb" json:"max_total_memory_mb"`           // Total RAM limit
	MaxConcurrentShards   int `yaml:"max_concurrent_shards" json:"max_concurrent_shards"`       // Max parallel shards
	MaxConcurrentAPICalls int `yaml:"max_concurrent_api_calls" json:"max_concurrent_api_calls"` // Max simultaneous LLM API calls
	MaxSessionDurationMin int `yaml:"max_session_duration_min" json:"max_session_duration_min"` // Auto-save interval
	MaxFactsInKernel      int `yaml:"max_facts_in_kernel" json:"max_facts_in_kernel"`           // EDB size limit
	MaxDerivedFactsLimit  int `yaml:"max_derived_facts_limit" json:"max_derived_facts_limit"`   // Mangle gas limit (Bug #17)
}

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
