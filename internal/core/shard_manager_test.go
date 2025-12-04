package core

import (
	"context"
	"testing"
	"time"
)

func TestNewShardManager(t *testing.T) {
	sm := NewShardManager()
	if sm == nil {
		t.Fatal("NewShardManager() returned nil")
	}

	// Should have default factories registered
	activeShards := sm.GetActiveShards()
	if activeShards == nil {
		t.Error("GetActiveShards() returned nil")
	}
}

func TestShardManagerSpawn(t *testing.T) {
	sm := NewShardManager()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Spawn a generalist shard
	result, err := sm.Spawn(ctx, "ephemeral", "test task")
	if err != nil {
		t.Fatalf("Spawn() error = %v", err)
	}

	if result == "" {
		t.Error("Spawn() returned empty result")
	}
}

func TestShardManagerSpawnCoder(t *testing.T) {
	sm := NewShardManager()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := sm.Spawn(ctx, "coder", "write a function")
	if err != nil {
		t.Fatalf("Spawn(coder) error = %v", err)
	}

	if result == "" {
		t.Error("Spawn(coder) returned empty result")
	}
}

func TestShardManagerSpawnResearcher(t *testing.T) {
	sm := NewShardManager()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := sm.Spawn(ctx, "researcher", "research a topic")
	if err != nil {
		t.Fatalf("Spawn(researcher) error = %v", err)
	}

	if result == "" {
		t.Error("Spawn(researcher) returned empty result")
	}
}

func TestShardManagerSpawnUnknownType(t *testing.T) {
	sm := NewShardManager()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := sm.Spawn(ctx, "nonexistent", "task")
	if err == nil {
		t.Error("Spawn(nonexistent) should return error")
	}
}

func TestShardManagerSpawnAsync(t *testing.T) {
	sm := NewShardManager()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	id, err := sm.SpawnAsync(ctx, "ephemeral", "async task")
	if err != nil {
		t.Fatalf("SpawnAsync() error = %v", err)
	}

	if id == "" {
		t.Error("SpawnAsync() returned empty id")
	}

	// Wait a bit for the shard to complete
	time.Sleep(100 * time.Millisecond)

	// Check result
	result, ok := sm.GetResult(id)
	if !ok {
		t.Log("Result not yet available (expected for async)")
	} else if result.Error != nil {
		t.Errorf("SpawnAsync shard failed: %v", result.Error)
	}
}

func TestShardManagerDefineProfile(t *testing.T) {
	sm := NewShardManager()

	config := DefaultSpecialistConfig("TestExpert", "memory/test.db")
	sm.DefineProfile("TestExpert", config)

	retrieved, ok := sm.GetProfile("TestExpert")
	if !ok {
		t.Fatal("GetProfile() returned false for defined profile")
	}

	if retrieved.Name != "TestExpert" {
		t.Errorf("Profile name = %q, want %q", retrieved.Name, "TestExpert")
	}
}

func TestShardManagerGetActiveShards(t *testing.T) {
	sm := NewShardManager()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Spawn async to keep it active longer
	_, _ = sm.SpawnAsync(ctx, "ephemeral", "long task")

	// Check active shards
	active := sm.GetActiveShards()
	// May or may not have active shards depending on timing
	t.Logf("Active shards: %d", len(active))
}

func TestShardManagerStopAll(t *testing.T) {
	sm := NewShardManager()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Spawn some async shards
	_, _ = sm.SpawnAsync(ctx, "ephemeral", "task1")
	_, _ = sm.SpawnAsync(ctx, "ephemeral", "task2")

	// Stop all
	sm.StopAll()

	// All should be stopped (or completing)
	time.Sleep(50 * time.Millisecond)
}

func TestShardManagerSetParentKernel(t *testing.T) {
	sm := NewShardManager()
	kernel := NewRealKernel()

	sm.SetParentKernel(kernel)

	// Should not panic
}

func TestShardManagerToFacts(t *testing.T) {
	sm := NewShardManager()

	// Define a profile
	config := DefaultSpecialistConfig("Expert1", "memory/expert1.db")
	sm.DefineProfile("Expert1", config)

	facts := sm.ToFacts()
	if len(facts) == 0 {
		t.Error("ToFacts() returned empty slice, expected at least profile facts")
	}
}

func TestDefaultGeneralistConfig(t *testing.T) {
	config := DefaultGeneralistConfig("TestGen")

	if config.Name != "TestGen" {
		t.Errorf("Name = %q, want %q", config.Name, "TestGen")
	}
	if config.Type != ShardTypeEphemeral {
		t.Errorf("Type = %v, want %v", config.Type, ShardTypeEphemeral)
	}
	if config.Timeout != 5*time.Minute {
		t.Errorf("Timeout = %v, want 5m", config.Timeout)
	}
}

func TestDefaultSpecialistConfig(t *testing.T) {
	config := DefaultSpecialistConfig("TestSpec", "memory/test.db")

	if config.Name != "TestSpec" {
		t.Errorf("Name = %q, want %q", config.Name, "TestSpec")
	}
	if config.Type != ShardTypeUser {
		t.Errorf("Type = %v, want %v", config.Type, ShardTypeUser)
	}
	if config.KnowledgePath != "memory/test.db" {
		t.Errorf("KnowledgePath = %q, want %q", config.KnowledgePath, "memory/test.db")
	}
	if config.Timeout != 30*time.Minute {
		t.Errorf("Timeout = %v, want 30m", config.Timeout)
	}
}

func TestBaseShardAgent(t *testing.T) {
	config := DefaultGeneralistConfig("TestAgent")
	agent := NewBaseShardAgent("test-001", config)

	if agent.GetID() != "test-001" {
		t.Errorf("GetID() = %q, want %q", agent.GetID(), "test-001")
	}

	if agent.GetState() != ShardStateIdle {
		t.Errorf("GetState() = %v, want %v", agent.GetState(), ShardStateIdle)
	}

	cfg := agent.GetConfig()
	if cfg.Name != "TestAgent" {
		t.Errorf("GetConfig().Name = %q, want %q", cfg.Name, "TestAgent")
	}
}

func TestBaseShardAgentPermissions(t *testing.T) {
	config := DefaultGeneralistConfig("TestAgent")
	agent := NewBaseShardAgent("test-001", config)

	if !agent.HasPermission(PermissionReadFile) {
		t.Error("Should have ReadFile permission")
	}
	if !agent.HasPermission(PermissionWriteFile) {
		t.Error("Should have WriteFile permission")
	}
	if agent.HasPermission(PermissionBrowser) {
		t.Error("Should NOT have Browser permission")
	}
}

func TestBaseShardAgentExecute(t *testing.T) {
	config := DefaultGeneralistConfig("TestAgent")
	agent := NewBaseShardAgent("test-001", config)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := agent.Execute(ctx, "test task")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result == "" {
		t.Error("Execute() returned empty result")
	}
}

func TestBaseShardAgentStop(t *testing.T) {
	config := DefaultGeneralistConfig("TestAgent")
	agent := NewBaseShardAgent("test-001", config)

	err := agent.Stop()
	if err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	if agent.GetState() != ShardStateCompleted {
		t.Errorf("GetState() after Stop() = %v, want %v", agent.GetState(), ShardStateCompleted)
	}
}
