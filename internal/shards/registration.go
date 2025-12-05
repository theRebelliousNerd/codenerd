// Package shards implements specialized ShardAgent types.
// This file provides registration helpers for the shard manager.
package shards

import (
	"codenerd/internal/core"
	"codenerd/internal/shards/researcher"
)

// RegisterAllShardFactories registers all specialized shard factories with the shard manager.
// This should be called during application initialization after creating the shard manager.
func RegisterAllShardFactories(sm *core.ShardManager) {
	// Register Coder shard factory
	sm.RegisterShard("coder", func(id string, config core.ShardConfig) core.ShardAgent {
		shard := NewCoderShard()
		return shard
	})

	// Register Reviewer shard factory
	sm.RegisterShard("reviewer", func(id string, config core.ShardConfig) core.ShardAgent {
		shard := NewReviewerShard()
		return shard
	})

	// Register Tester shard factory
	sm.RegisterShard("tester", func(id string, config core.ShardConfig) core.ShardAgent {
		shard := NewTesterShard()
		return shard
	})

	// Register Researcher shard factory
	sm.RegisterShard("researcher", func(id string, config core.ShardConfig) core.ShardAgent {
		shard := researcher.NewResearcherShard()
		return shard
	})

	// Register ToolGenerator shard factory (autopoiesis)
	sm.RegisterShard("tool_generator", func(id string, config core.ShardConfig) core.ShardAgent {
		shard := NewToolGeneratorShard(id, config)
		return shard
	})

	// Define shard profiles with proper configurations
	defineShardProfiles(sm)
}

// defineShardProfiles registers shard profiles with appropriate configurations.
func defineShardProfiles(sm *core.ShardManager) {
	// Coder profile - code generation specialist
	sm.DefineProfile("coder", core.ShardConfig{
		Name: "coder",
		Type: core.ShardTypeEphemeral,
		Permissions: []core.ShardPermission{
			core.PermissionReadFile,
			core.PermissionWriteFile,
			core.PermissionExecCmd,
			core.PermissionCodeGraph,
		},
		Timeout:     10 * 60 * 1000000000, // 10 minutes
		MemoryLimit: 5000,
		Model: core.ModelConfig{
			Capability: core.CapabilityHighReasoning,
		},
	})

	// Reviewer profile - code review specialist
	sm.DefineProfile("reviewer", core.ShardConfig{
		Name: "reviewer",
		Type: core.ShardTypeEphemeral,
		Permissions: []core.ShardPermission{
			core.PermissionReadFile,
			core.PermissionCodeGraph,
		},
		Timeout:     5 * 60 * 1000000000, // 5 minutes
		MemoryLimit: 3000,
		Model: core.ModelConfig{
			Capability: core.CapabilityHighReasoning,
		},
	})

	// Tester profile - testing specialist
	sm.DefineProfile("tester", core.ShardConfig{
		Name: "tester",
		Type: core.ShardTypeEphemeral,
		Permissions: []core.ShardPermission{
			core.PermissionReadFile,
			core.PermissionWriteFile,
			core.PermissionExecCmd,
		},
		Timeout:     15 * 60 * 1000000000, // 15 minutes
		MemoryLimit: 3000,
		Model: core.ModelConfig{
			Capability: core.CapabilityBalanced,
		},
	})

	// Researcher profile - research specialist
	sm.DefineProfile("researcher", core.ShardConfig{
		Name: "researcher",
		Type: core.ShardTypeEphemeral,
		Permissions: []core.ShardPermission{
			core.PermissionReadFile,
			core.PermissionNetwork,
			core.PermissionResearch,
		},
		Timeout:     10 * 60 * 1000000000, // 10 minutes
		MemoryLimit: 5000,
		Model: core.ModelConfig{
			Capability: core.CapabilityBalanced,
		},
	})

	// ToolGenerator profile - autopoiesis specialist
	sm.DefineProfile("tool_generator", core.ShardConfig{
		Name: "tool_generator",
		Type: core.ShardTypePersistent,
		Permissions: []core.ShardPermission{
			core.PermissionReadFile,
			core.PermissionWriteFile,
			core.PermissionExecCmd,
			core.PermissionCodeGraph,
		},
		Timeout:     30 * 60 * 1000000000, // 30 minutes (tool generation can take time)
		MemoryLimit: 10000,
		Model: core.ModelConfig{
			Capability: core.CapabilityHighReasoning,
		},
	})
}
