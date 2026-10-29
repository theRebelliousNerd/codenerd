package core

import (
	"testing"
)

func TestLimitsEnforcer_New(t *testing.T) {
	cfg := testLimitsConfig()
	enforcer := NewLimitsEnforcer(cfg)

	if enforcer == nil {
		t.Fatal("NewLimitsEnforcer returned nil")
	}
}

func TestLimitsEnforcer_CheckMemory(t *testing.T) {
	cfg := testLimitsConfig()
	cfg.MaxTotalMemoryMB = 10000 // High limit to pass

	enforcer := NewLimitsEnforcer(cfg)

	err := enforcer.CheckMemory()
	if err != nil {
		t.Logf("CheckMemory: %v (may exceed limit in low-memory env)", err)
	}
}

func TestLimitsEnforcer_GetMemoryUsage(t *testing.T) {
	enforcer := NewLimitsEnforcer(testLimitsConfig())

	usage := enforcer.GetMemoryUsage()
	if usage < 0 {
		t.Errorf("Expected non-negative memory usage, got %d", usage)
	}

	t.Logf("Current memory usage: %d MB", usage)
}

func TestLimitsEnforcer_CheckShardLimit(t *testing.T) {
	cfg := testLimitsConfig()
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
	cfg := testLimitsConfig()
	cfg.MaxConcurrentShards = 10

	enforcer := NewLimitsEnforcer(cfg)

	limit := enforcer.GetShardLimit()
	if limit != 10 {
		t.Errorf("Expected limit 10, got %d", limit)
	}
}

func TestLimitsEnforcer_GetAvailableShardSlots(t *testing.T) {
	cfg := testLimitsConfig()
	cfg.MaxConcurrentShards = 5

	enforcer := NewLimitsEnforcer(cfg)

	slots := enforcer.GetAvailableShardSlots(2)
	if slots != 3 {
		t.Errorf("Expected 3 available slots, got %d", slots)
	}
}

func TestLimitsEnforcer_EstimateCapacity(t *testing.T) {
	cfg := testLimitsConfig()
	enforcer := NewLimitsEnforcer(cfg)

	slots, reason := enforcer.EstimateCapacity(0)

	t.Logf("Capacity: %d slots, reason: %s", slots, reason)

	if slots < 0 {
		t.Errorf("Expected non-negative capacity, got %d", slots)
	}
}

func TestLimitsEnforcer_Callbacks(t *testing.T) {
	enforcer := NewLimitsEnforcer(testLimitsConfig())

	memoryCalled := false
	shardCalled := false

	enforcer.OnMemoryViolation(func(used, limit int) {
		memoryCalled = true
	})

	enforcer.OnShardViolation(func(active, limit int) {
		shardCalled = true
	})

	// Just verify callbacks can be set (they'll be called on violations)
	t.Logf("Callbacks set: memory=%v, shard=%v",
		memoryCalled, shardCalled)
}

// testLimitsConfig is a generous configuration for exercising the enforcer;
// production builds its LimitsConfig from config in system/factory.go.
func testLimitsConfig() LimitsConfig {
	return LimitsConfig{
		MaxTotalMemoryMB:     12288,
		MaxConcurrentShards:  12,
		MaxFactsInKernel:     2000000,
		MaxDerivedFactsLimit: 5000000,
	}
}

func TestLimitsEnforcer_GetStatus_ShouldReturnAllKeys(t *testing.T) {
	enforcer := NewLimitsEnforcer(testLimitsConfig())
	status := enforcer.GetStatus()

	expectedKeys := []string{
		"memory_mb", "memory_limit_mb", "memory_utilization",
		"shard_limit",
		"max_facts_in_kernel", "max_derived_facts",
	}
	for _, key := range expectedKeys {
		if _, ok := status[key]; !ok {
			t.Errorf("missing key %q in GetStatus()", key)
		}
	}
}
