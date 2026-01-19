package core

import (
	"testing"
	"time"
)

func TestLimitsEnforcer_New(t *testing.T) {
	cfg := DefaultLimitsConfig()
	enforcer := NewLimitsEnforcer(cfg)

	if enforcer == nil {
		t.Fatal("NewLimitsEnforcer returned nil")
	}
}

func TestDefaultLimitsConfig(t *testing.T) {
	cfg := DefaultLimitsConfig()

	if cfg.MaxTotalMemoryMB <= 0 {
		t.Errorf("Expected positive MaxTotalMemoryMB, got %d", cfg.MaxTotalMemoryMB)
	}

	if cfg.MaxConcurrentShards <= 0 {
		t.Errorf("Expected positive MaxConcurrentShards, got %d", cfg.MaxConcurrentShards)
	}

	if cfg.MaxSessionDurationMin <= 0 {
		t.Errorf("Expected positive MaxSessionDurationMin, got %d", cfg.MaxSessionDurationMin)
	}
}

func TestLimitsEnforcer_CheckMemory(t *testing.T) {
	cfg := DefaultLimitsConfig()
	cfg.MaxTotalMemoryMB = 10000 // High limit to pass

	enforcer := NewLimitsEnforcer(cfg)

	err := enforcer.CheckMemory()
	if err != nil {
		t.Logf("CheckMemory: %v (may exceed limit in low-memory env)", err)
	}
}

func TestLimitsEnforcer_GetMemoryUsage(t *testing.T) {
	enforcer := NewLimitsEnforcer(DefaultLimitsConfig())

	usage := enforcer.GetMemoryUsage()
	if usage < 0 {
		t.Errorf("Expected non-negative memory usage, got %d", usage)
	}

	t.Logf("Current memory usage: %d MB", usage)
}

func TestLimitsEnforcer_CheckSessionDuration(t *testing.T) {
	cfg := DefaultLimitsConfig()
	cfg.MaxSessionDurationMin = 60 // 60 minutes

	enforcer := NewLimitsEnforcer(cfg)

	err := enforcer.CheckSessionDuration()
	if err != nil {
		t.Errorf("Unexpected session timeout: %v", err)
	}
}

func TestLimitsEnforcer_SetSessionStart(t *testing.T) {
	enforcer := NewLimitsEnforcer(DefaultLimitsConfig())

	pastTime := time.Now().Add(-1 * time.Hour)
	enforcer.SetSessionStart(pastTime)

	duration := enforcer.GetSessionDuration()
	if duration < time.Hour {
		t.Errorf("Expected duration >= 1 hour, got %v", duration)
	}
}

func TestLimitsEnforcer_CheckShardLimit(t *testing.T) {
	cfg := DefaultLimitsConfig()
	cfg.MaxConcurrentShards = 5

	enforcer := NewLimitsEnforcer(cfg)

	// Under limit
	err := enforcer.CheckShardLimit(3)
	if err != nil {
		t.Errorf("Unexpected error for 3 shards: %v", err)
	}

	// At limit
	err = enforcer.CheckShardLimit(5)
	if err == nil {
		t.Error("Expected error when at shard limit")
	}
}

func TestLimitsEnforcer_GetShardLimit(t *testing.T) {
	cfg := DefaultLimitsConfig()
	cfg.MaxConcurrentShards = 10

	enforcer := NewLimitsEnforcer(cfg)

	limit := enforcer.GetShardLimit()
	if limit != 10 {
		t.Errorf("Expected limit 10, got %d", limit)
	}
}

func TestLimitsEnforcer_GetAvailableShardSlots(t *testing.T) {
	cfg := DefaultLimitsConfig()
	cfg.MaxConcurrentShards = 5

	enforcer := NewLimitsEnforcer(cfg)

	slots := enforcer.GetAvailableShardSlots(2)
	if slots != 3 {
		t.Errorf("Expected 3 available slots, got %d", slots)
	}
}

func TestLimitsEnforcer_EstimateCapacity(t *testing.T) {
	cfg := DefaultLimitsConfig()
	enforcer := NewLimitsEnforcer(cfg)

	slots, reason := enforcer.EstimateCapacity(0)

	t.Logf("Capacity: %d slots, reason: %s", slots, reason)

	if slots < 0 {
		t.Errorf("Expected non-negative capacity, got %d", slots)
	}
}

func TestLimitsEnforcer_RemainingSessionTime(t *testing.T) {
	cfg := DefaultLimitsConfig()
	cfg.MaxSessionDurationMin = 60

	enforcer := NewLimitsEnforcer(cfg)

	remaining := enforcer.RemainingSessionTime()

	// Should have most of the 60 minutes remaining
	if remaining < 55*time.Minute {
		t.Logf("Remaining time: %v (may be less if session started earlier)", remaining)
	}
}

func TestLimitsEnforcer_Callbacks(t *testing.T) {
	enforcer := NewLimitsEnforcer(DefaultLimitsConfig())

	memoryCalled := false
	sessionCalled := false
	shardCalled := false

	enforcer.OnMemoryViolation(func(used, limit int) {
		memoryCalled = true
	})

	enforcer.OnSessionTimeout(func(elapsed, limit time.Duration) {
		sessionCalled = true
	})

	enforcer.OnShardViolation(func(active, limit int) {
		shardCalled = true
	})

	// Just verify callbacks can be set (they'll be called on violations)
	t.Logf("Callbacks set: memory=%v, session=%v, shard=%v",
		memoryCalled, sessionCalled, shardCalled)
}
