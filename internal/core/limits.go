// Package core provides the limits enforcement system for codeNERD.
// This file implements hard enforcement of CoreLimits from config.
package core

import (
	"fmt"
	"runtime"
	"sync"

	"codenerd/internal/logging"
)

// =============================================================================
// LIMITS ENFORCER
// =============================================================================
// Enforces CoreLimits from config with hard errors, not just warnings.
// Previously these limits were defined but never checked at runtime.

// LimitsConfig holds the enforcement parameters.
type LimitsConfig struct {
	MaxTotalMemoryMB     int // Total RAM limit in MB
	MaxConcurrentShards  int // Max parallel shards
	MaxFactsInKernel     int // EDB size limit
	MaxDerivedFactsLimit int // Mangle gas limit for inference
}

// DefaultLimitsConfig returns production defaults matching config.go.
func DefaultLimitsConfig() LimitsConfig {
	return LimitsConfig{
		MaxTotalMemoryMB:     12288,   // 12GB RAM limit
		MaxConcurrentShards:  12,      // Max 12 parallel shards (7 system + 5 user)
		MaxFactsInKernel:     2000000, // Out-of-memory backstop, not a working budget
		MaxDerivedFactsLimit: 5000000, // Runaway-rule backstop; a real repository's world derives past 500k
	}
}

// LimitsEnforcer tracks resource usage and enforces hard limits.
type LimitsEnforcer struct {
	mu sync.RWMutex

	config LimitsConfig

	// Callbacks for violation handling
	onMemoryViolation func(usedMB, limitMB int)
	onShardViolation  func(active, limit int)
}

// NewLimitsEnforcer creates a new enforcer with the given config.
func NewLimitsEnforcer(cfg LimitsConfig) *LimitsEnforcer {
	logging.Kernel("LimitsEnforcer initialized: memory=%dMB, shards=%d, facts=%d, derived=%d",
		cfg.MaxTotalMemoryMB, cfg.MaxConcurrentShards,
		cfg.MaxFactsInKernel, cfg.MaxDerivedFactsLimit)

	return &LimitsEnforcer{
		config: cfg,
	}
}

// OnMemoryViolation sets the callback for memory limit violations.
func (le *LimitsEnforcer) OnMemoryViolation(fn func(usedMB, limitMB int)) {
	le.mu.Lock()
	defer le.mu.Unlock()
	le.onMemoryViolation = fn
}

// OnShardViolation sets the callback for shard limit violations.
func (le *LimitsEnforcer) OnShardViolation(fn func(active, limit int)) {
	le.mu.Lock()
	defer le.mu.Unlock()
	le.onShardViolation = fn
}

// =============================================================================
// MEMORY ENFORCEMENT
// =============================================================================

// ErrMemoryLimitExceeded is returned when memory usage exceeds the limit.
var ErrMemoryLimitExceeded = fmt.Errorf("memory limit exceeded")

// CheckMemory checks if current memory usage is within limits.
// Returns error if limit is exceeded.
func (le *LimitsEnforcer) CheckMemory() error {
	if le.config.MaxTotalMemoryMB <= 0 {
		return nil // No limit configured
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Alloc is bytes of allocated heap objects
	usedMB := int(m.Alloc / 1024 / 1024)

	if usedMB > le.config.MaxTotalMemoryMB {
		logging.Get(logging.CategoryKernel).Error("MEMORY LIMIT EXCEEDED: %dMB used > %dMB limit",
			usedMB, le.config.MaxTotalMemoryMB)

		le.mu.RLock()
		callback := le.onMemoryViolation
		le.mu.RUnlock()

		if callback != nil {
			callback(usedMB, le.config.MaxTotalMemoryMB)
		}

		return fmt.Errorf("%w: %dMB used exceeds %dMB limit", ErrMemoryLimitExceeded,
			usedMB, le.config.MaxTotalMemoryMB)
	}

	return nil
}

// GetMemoryUsage returns current memory usage in MB.
func (le *LimitsEnforcer) GetMemoryUsage() int {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int(m.Alloc / 1024 / 1024)
}

// GetMemoryUtilization returns memory utilization as a percentage (0.0-1.0).
func (le *LimitsEnforcer) GetMemoryUtilization() float64 {
	if le.config.MaxTotalMemoryMB <= 0 {
		return 0.0
	}
	usedMB := le.GetMemoryUsage()
	return float64(usedMB) / float64(le.config.MaxTotalMemoryMB)
}

// =============================================================================
// CONCURRENT SHARDS ENFORCEMENT
// =============================================================================

// ErrTooManyShards is returned when trying to spawn more shards than allowed.
var ErrTooManyShards = fmt.Errorf("concurrent shard limit exceeded")

// CheckShardLimit checks if spawning another shard would exceed the limit.
// activeCount is the current number of active shards.
// Returns error if limit would be exceeded.
func (le *LimitsEnforcer) CheckShardLimit(activeCount int) error {
	if le.config.MaxConcurrentShards <= 0 {
		return nil // No limit configured
	}

	if activeCount >= le.config.MaxConcurrentShards {
		logging.Get(logging.CategoryKernel).Warn("SHARD LIMIT REACHED: %d active >= %d limit",
			activeCount, le.config.MaxConcurrentShards)

		le.mu.RLock()
		callback := le.onShardViolation
		le.mu.RUnlock()

		if callback != nil {
			callback(activeCount, le.config.MaxConcurrentShards)
		}

		return fmt.Errorf("%w: %d active shards equals limit of %d", ErrTooManyShards,
			activeCount, le.config.MaxConcurrentShards)
	}

	return nil
}

// GetShardLimit returns the max concurrent shards limit.
func (le *LimitsEnforcer) GetShardLimit() int {
	return le.config.MaxConcurrentShards
}

// GetAvailableShardSlots returns how many more shards can be spawned.
func (le *LimitsEnforcer) GetAvailableShardSlots(activeCount int) int {
	if le.config.MaxConcurrentShards <= 0 {
		return 100 // Effectively unlimited
	}
	available := le.config.MaxConcurrentShards - activeCount
	if available < 0 {
		return 0
	}
	return available
}

// EstimateCapacity returns a capacity estimate considering memory and shards.
// Returns the number of available slots and a reason if capacity is reduced.
func (le *LimitsEnforcer) EstimateCapacity(activeShards int) (slots int, reason string) {
	// Check memory first - critical constraint
	memUtil := le.GetMemoryUtilization()
	if memUtil > 0.9 {
		return 0, "memory utilization critical (>90%)"
	}

	// Check shard slots
	slots = le.GetAvailableShardSlots(activeShards)
	if slots == 0 {
		return 0, "shard limit reached"
	}

	// If memory is high, reduce effective capacity
	if memUtil > 0.7 {
		slots = max(slots/2, 1)
		return slots, "reduced due to high memory (>70%)"
	}

	return slots, ""
}

// =============================================================================
// KERNEL FACT LIMITS
// =============================================================================

// GetMaxFactsInKernel returns the max facts limit for the kernel.
func (le *LimitsEnforcer) GetMaxFactsInKernel() int {
	return le.config.MaxFactsInKernel
}

// GetMaxDerivedFactsLimit returns the gas limit for Mangle inference.
func (le *LimitsEnforcer) GetMaxDerivedFactsLimit() int {
	return le.config.MaxDerivedFactsLimit
}

// =============================================================================
// AGGREGATE CHECK
// =============================================================================

// CheckAll runs all limit checks and returns the first error encountered.
// activeShards is the current number of active shards.
func (le *LimitsEnforcer) CheckAll(activeShards int) error {
	if err := le.CheckMemory(); err != nil {
		return err
	}
	if err := le.CheckShardLimit(activeShards); err != nil {
		return err
	}
	return nil
}

// GetStatus returns a summary of current limit utilization.
func (le *LimitsEnforcer) GetStatus() map[string]any {
	return map[string]any{
		"memory_mb":           le.GetMemoryUsage(),
		"memory_limit_mb":     le.config.MaxTotalMemoryMB,
		"memory_utilization":  le.GetMemoryUtilization(),
		"shard_limit":         le.config.MaxConcurrentShards,
		"max_facts_in_kernel": le.config.MaxFactsInKernel,
		"max_derived_facts":   le.config.MaxDerivedFactsLimit,
	}
}
