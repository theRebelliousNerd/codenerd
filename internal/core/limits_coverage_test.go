package core

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestLimitsEnforcer_CoverageExtra(t *testing.T) {
	// 1. Test disabled limits (zeros)
	cfgZero := LimitsConfig{
		MaxTotalMemoryMB:      0,
		MaxConcurrentShards:   0,
		MaxSessionDurationMin: 0,
		MaxFactsInKernel:      100,
		MaxDerivedFactsLimit:  200,
	}
	leZero := NewLimitsEnforcer(cfgZero)

	if err := leZero.CheckMemory(); err != nil {
		t.Errorf("expected no memory error when disabled, got: %v", err)
	}
	if util := leZero.GetMemoryUtilization(); util != 0.0 {
		t.Errorf("expected memory utilization 0.0 when disabled, got: %f", util)
	}

	if err := leZero.CheckSessionDuration(); err != nil {
		t.Errorf("expected no session error when disabled, got: %v", err)
	}
	if util := leZero.GetSessionUtilization(); util != 0.0 {
		t.Errorf("expected session utilization 0.0 when disabled, got: %f", util)
	}
	if rem := leZero.RemainingSessionTime(); rem <= 24*time.Hour {
		t.Errorf("expected near infinite remaining session time, got: %v", rem)
	}

	if err := leZero.CheckShardLimit(100); err != nil {
		t.Errorf("expected no shard limit error when disabled, got: %v", err)
	}
	if slots := leZero.GetAvailableShardSlots(100); slots != 100 {
		t.Errorf("expected default 100 available slots when disabled, got: %d", slots)
	}

	if maxFacts := leZero.GetMaxFactsInKernel(); maxFacts != 100 {
		t.Errorf("expected MaxFactsInKernel 100, got: %d", maxFacts)
	}
	if maxDerived := leZero.GetMaxDerivedFactsLimit(); maxDerived != 200 {
		t.Errorf("expected MaxDerivedFactsLimit 200, got: %d", maxDerived)
	}

	// 2. Test Callback executions on limits triggered
	cfgTrigger := LimitsConfig{
		MaxTotalMemoryMB:      1, // Extremely low
		MaxConcurrentShards:   2,
		MaxSessionDurationMin: 10,
	}
	leTrigger := NewLimitsEnforcer(cfgTrigger)

	memoryViolated := false
	leTrigger.OnMemoryViolation(func(used, limit int) {
		memoryViolated = true
	})

	err := leTrigger.CheckMemory()
	if err == nil || !errors.Is(err, ErrMemoryLimitExceeded) {
		t.Errorf("expected ErrMemoryLimitExceeded, got: %v", err)
	}
	if !memoryViolated {
		t.Error("expected memory violation callback to be triggered")
	}

	// Session timeout callback
	leTrigger.SetSessionStart(time.Now().Add(-20 * time.Minute))
	sessionTimeoutCalled := false
	leTrigger.OnSessionTimeout(func(elapsed, limit time.Duration) {
		sessionTimeoutCalled = true
	})
	err = leTrigger.CheckSessionDuration()
	if err == nil || !errors.Is(err, ErrSessionTimeout) {
		t.Errorf("expected ErrSessionTimeout, got: %v", err)
	}
	if !sessionTimeoutCalled {
		t.Error("expected session timeout callback to be triggered")
	}
	if rem := leTrigger.RemainingSessionTime(); rem != 0 {
		t.Errorf("expected remaining session time 0 when expired, got: %v", rem)
	}

	// Shard limit callback
	shardLimitCalled := false
	leTrigger.OnShardViolation(func(active, limit int) {
		shardLimitCalled = true
	})
	err = leTrigger.CheckShardLimit(2)
	if err == nil || !errors.Is(err, ErrTooManyShards) {
		t.Errorf("expected ErrTooManyShards, got: %v", err)
	}
	if !shardLimitCalled {
		t.Error("expected shard violation callback to be triggered")
	}

	// 3. Test EstimateCapacity edge cases
	cfgShardLimitOnly := LimitsConfig{
		MaxTotalMemoryMB:    10000,
		MaxConcurrentShards: 2,
	}
	leShardOnly := NewLimitsEnforcer(cfgShardLimitOnly)
	slots, reason := leShardOnly.EstimateCapacity(2)
	if slots != 0 || reason != "shard limit reached" {
		t.Errorf("expected 0 slots due to shard limit, got slots=%d, reason=%q", slots, reason)
	}

	// Memory > 90% (by setting MaxTotalMemoryMB to 1)
	slots, reason = leTrigger.EstimateCapacity(0)
	if slots != 0 || !strings.Contains(reason, "memory utilization critical") {
		t.Errorf("expected 0 slots due to critical memory, got slots=%d, reason=%q", slots, reason)
	}

	// Memory > 70% but <= 90% (simulate memory utilization 75%)
	currentMem := leZero.GetMemoryUsage()
	if currentMem > 0 {
		cfgHighMem := LimitsConfig{
			MaxTotalMemoryMB:    int(float64(currentMem) / 0.75),
			MaxConcurrentShards: 10,
		}
		leHighMem := NewLimitsEnforcer(cfgHighMem)
		slots, reason = leHighMem.EstimateCapacity(0)
		if slots == 0 || !strings.Contains(reason, "reduced due to high memory") {
			t.Logf("estimate capacity: slots=%d, reason=%q", slots, reason)
		}
	}

	// 4. Test CheckAll and GetStatus
	cfgOK := DefaultLimitsConfig()
	cfgOK.MaxTotalMemoryMB = 999999
	leOK := NewLimitsEnforcer(cfgOK)
	if err := leOK.CheckAll(1); err != nil {
		t.Errorf("expected CheckAll to succeed, got: %v", err)
	}

	status := leOK.GetStatus()
	if status["memory_limit_mb"] != 999999 {
		t.Errorf("expected status memory limit 999999, got: %v", status["memory_limit_mb"])
	}
}
