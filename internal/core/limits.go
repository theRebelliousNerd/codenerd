// Package core provides the limits enforcement system for codeNERD.
// This file implements hard enforcement of CoreLimits from config.
package core

import (
	"fmt"
	"runtime"
	"sync"
	"time"

	"codenerd/internal/logging"
)

// =============================================================================
// LIMITS ENFORCER
// =============================================================================
// Enforces CoreLimits from config with hard errors, not just warnings.
// Previously these limits were defined but never checked at runtime.

// LimitsConfig holds the enforcement parameters.
type LimitsConfig struct {
	MaxTotalMemoryMB      int // Total RAM limit in MB
	MaxConcurrentShards   int // Max parallel shards
	MaxSessionDurationMin int // Auto-save interval / max session time
	MaxFactsInKernel      int // EDB size limit
	MaxDerivedFactsLimit  int // Mangle gas limit for inference
}

// DefaultLimitsConfig returns production defaults matching config.go.
func DefaultLimitsConfig() LimitsConfig {
	return LimitsConfig{
		MaxTotalMemoryMB:      12288,  // 12GB RAM limit
		MaxConcurrentShards:   12,     // Max 12 parallel shards (7 system + 5 user)
		MaxSessionDurationMin: 120,    // 2 hour sessions
		MaxFactsInKernel:      250000, // Increase working-set ceiling with larger RAM
		MaxDerivedFactsLimit:  100000, // Mangle gas limit scales with fact budget
	}
}

// LimitsEnforcer tracks resource usage and enforces hard limits.
type LimitsEnforcer struct {
	mu sync.RWMutex

	config       LimitsConfig
	sessionStart time.Time

	// Callbacks for violation handling
	onMemoryViolation  func(usedMB, limitMB int)
	onSessionTimeout   func(elapsed, limit time.Duration)
	onShardViolation   func(active, limit int)
}

// NewLimitsEnforcer creates a new enforcer with the given config.
func NewLimitsEnforcer(cfg LimitsConfig) *LimitsEnforcer {
	logging.Kernel("LimitsEnforcer initialized: memory=%dMB, shards=%d, session=%dmin, facts=%d, derived=%d",
		cfg.MaxTotalMemoryMB, cfg.MaxConcurrentShards, cfg.MaxSessionDurationMin,
		cfg.MaxFactsInKernel, cfg.MaxDerivedFactsLimit)

	return &LimitsEnforcer{
		config:       cfg,
		sessionStart: time.Now(),
	}
}

// SetSessionStart sets the session start time (for resumed sessions).
func (le *LimitsEnforcer) SetSessionStart(t time.Time) {
	le.mu.Lock()
	defer le.mu.Unlock()
	le.sessionStart = t
	logging.KernelDebug("Session start time set to: %v", t)
}

// OnMemoryViolation sets the callback for memory limit violations.
func (le *LimitsEnforcer) OnMemoryViolation(fn func(usedMB, limitMB int)) {
	le.mu.Lock()
	defer le.mu.Unlock()
	le.onMemoryViolation = fn
}

// OnSessionTimeout sets the callback for session timeout.
func (le *LimitsEnforcer) OnSessionTimeout(fn func(elapsed, limit time.Duration)) {
	le.mu.Lock()
	defer le.mu.Unlock()
	le.onSessionTimeout = fn
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
// SESSION DURATION ENFORCEMENT
// =============================================================================

// ErrSessionTimeout is returned when session duration exceeds the limit.
var ErrSessionTimeout = fmt.Errorf("session timeout")

// CheckSessionDuration checks if session duration is within limits.
// Returns error if limit is exceeded.
func (le *LimitsEnforcer) CheckSessionDuration() error {
	if le.config.MaxSessionDurationMin <= 0 {
		return nil // No limit configured
	}

	le.mu.RLock()
	start := le.sessionStart
	le.mu.RUnlock()

	elapsed := time.Since(start)
	limit := time.Duration(le.config.MaxSessionDurationMin) * time.Minute

	if elapsed > limit {
		logging.Get(logging.CategoryKernel).Warn("SESSION TIMEOUT: %v elapsed > %v limit",
			elapsed.Round(time.Second), limit)

		le.mu.RLock()
		callback := le.onSessionTimeout
		le.mu.RUnlock()

		if callback != nil {
			callback(elapsed, limit)
		}

		return fmt.Errorf("%w: %v elapsed exceeds %v limit", ErrSessionTimeout,
			elapsed.Round(time.Second), limit)
	}

	return nil
}

// GetSessionDuration returns elapsed session time.
func (le *LimitsEnforcer) GetSessionDuration() time.Duration {
	le.mu.RLock()
	defer le.mu.RUnlock()
	return time.Since(le.sessionStart)
}

// GetSessionUtilization returns session duration utilization (0.0-1.0).
func (le *LimitsEnforcer) GetSessionUtilization() float64 {
	if le.config.MaxSessionDurationMin <= 0 {
		return 0.0
	}
	elapsed := le.GetSessionDuration()
	limit := time.Duration(le.config.MaxSessionDurationMin) * time.Minute
	return float64(elapsed) / float64(limit)
}

// RemainingSessionTime returns how much time is left in the session.
func (le *LimitsEnforcer) RemainingSessionTime() time.Duration {
	if le.config.MaxSessionDurationMin <= 0 {
		return time.Duration(1<<63 - 1) // Max duration (effectively unlimited)
	}
	elapsed := le.GetSessionDuration()
	limit := time.Duration(le.config.MaxSessionDurationMin) * time.Minute
	remaining := limit - elapsed
	if remaining < 0 {
		return 0
	}
	return remaining
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
		slots = slots / 2
		if slots < 1 {
			slots = 1
		}
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
	if err := le.CheckSessionDuration(); err != nil {
		return err
	}
	if err := le.CheckShardLimit(activeShards); err != nil {
		return err
	}
	return nil
}

// GetStatus returns a summary of current limit utilization.
func (le *LimitsEnforcer) GetStatus() map[string]interface{} {
	return map[string]interface{}{
		"memory_mb":             le.GetMemoryUsage(),
		"memory_limit_mb":       le.config.MaxTotalMemoryMB,
		"memory_utilization":    le.GetMemoryUtilization(),
		"session_elapsed":       le.GetSessionDuration().String(),
		"session_limit":         time.Duration(le.config.MaxSessionDurationMin) * time.Minute,
		"session_remaining":     le.RemainingSessionTime().String(),
		"session_utilization":   le.GetSessionUtilization(),
		"shard_limit":           le.config.MaxConcurrentShards,
		"max_facts_in_kernel":   le.config.MaxFactsInKernel,
		"max_derived_facts":     le.config.MaxDerivedFactsLimit,
	}
}
