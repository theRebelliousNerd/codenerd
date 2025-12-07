// Package shards implements specialized ShardAgent types.
// This file provides registration helpers for the shard manager.
package shards

import (
	"codenerd/internal/core"
	"codenerd/internal/shards/coder"
	"codenerd/internal/shards/researcher"
	"codenerd/internal/shards/reviewer"
	"codenerd/internal/shards/system"
	"codenerd/internal/shards/tester"
)

// RegisterAllShardFactories registers all specialized shard factories with the shard manager.
// This should be called during application initialization after creating the shard manager.
func RegisterAllShardFactories(sm *core.ShardManager) {
	// Register Coder shard factory
	sm.RegisterShard("coder", func(id string, config core.ShardConfig) core.ShardAgent {
		shard := coder.NewCoderShard()
		return shard
	})

	// Register Reviewer shard factory
	sm.RegisterShard("reviewer", func(id string, config core.ShardConfig) core.ShardAgent {
		shard := reviewer.NewReviewerShard()
		return shard
	})

	// Register Tester shard factory
	sm.RegisterShard("tester", func(id string, config core.ShardConfig) core.ShardAgent {
		shard := tester.NewTesterShard()
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

	// =========================================================================
	// Type 1: System Shards (Permanent, Continuous)
	// =========================================================================

	// Register Perception Firewall - AUTO-START, LLM-primary
	sm.RegisterShard("perception_firewall", func(id string, config core.ShardConfig) core.ShardAgent {
		return system.NewPerceptionFirewallShard()
	})

	// Register World Model Ingestor - ON-DEMAND, Hybrid
	sm.RegisterShard("world_model_ingestor", func(id string, config core.ShardConfig) core.ShardAgent {
		return system.NewWorldModelIngestorShard()
	})

	// Register Executive Policy - AUTO-START, Logic-primary
	sm.RegisterShard("executive_policy", func(id string, config core.ShardConfig) core.ShardAgent {
		return system.NewExecutivePolicyShard()
	})

	// Register Constitution Gate - AUTO-START, Logic-primary (SAFETY-CRITICAL)
	sm.RegisterShard("constitution_gate", func(id string, config core.ShardConfig) core.ShardAgent {
		return system.NewConstitutionGateShard()
	})

	// Register Tactile Router - ON-DEMAND, Logic-primary
	sm.RegisterShard("tactile_router", func(id string, config core.ShardConfig) core.ShardAgent {
		return system.NewTactileRouterShard()
	})

	// Register Session Planner - ON-DEMAND, LLM-primary
	sm.RegisterShard("session_planner", func(id string, config core.ShardConfig) core.ShardAgent {
		return system.NewSessionPlannerShard()
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

	// Define system shard profiles
	defineSystemShardProfiles(sm)
}

// RegisterSystemShardProfiles registers Type 1 system shard profiles.
// This is exported for use by session initialization when factories are
// registered manually with dependency injection.
func RegisterSystemShardProfiles(sm *core.ShardManager) {
	defineSystemShardProfiles(sm)
}

// defineSystemShardProfiles registers Type 1 system shard profiles.
func defineSystemShardProfiles(sm *core.ShardManager) {
	// Perception Firewall - AUTO-START, LLM for NL understanding
	sm.DefineProfile("perception_firewall", core.ShardConfig{
		Name: "perception_firewall",
		Type: core.ShardTypeSystem,
		Permissions: []core.ShardPermission{
			core.PermissionReadFile,
			core.PermissionAskUser,
		},
		Timeout:     24 * 60 * 60 * 1000000000, // 24 hours (permanent)
		MemoryLimit: 5000,
		Model: core.ModelConfig{
			Capability: core.CapabilityBalanced,
		},
	})

	// World Model Ingestor - ON-DEMAND, Hybrid
	sm.DefineProfile("world_model_ingestor", core.ShardConfig{
		Name: "world_model_ingestor",
		Type: core.ShardTypeSystem,
		Permissions: []core.ShardPermission{
			core.PermissionReadFile,
			core.PermissionExecCmd,
			core.PermissionCodeGraph,
		},
		Timeout:     24 * 60 * 60 * 1000000000, // 24 hours
		MemoryLimit: 10000,
		Model: core.ModelConfig{
			Capability: core.CapabilityHighSpeed,
		},
	})

	// Executive Policy - AUTO-START, Pure logic (no LLM by default)
	sm.DefineProfile("executive_policy", core.ShardConfig{
		Name: "executive_policy",
		Type: core.ShardTypeSystem,
		Permissions: []core.ShardPermission{
			core.PermissionReadFile,
			core.PermissionCodeGraph,
			core.PermissionAskUser,
		},
		Timeout:     24 * 60 * 60 * 1000000000, // 24 hours
		MemoryLimit: 3000,
		Model:       core.ModelConfig{}, // No LLM needed for core logic
	})

	// Constitution Gate - AUTO-START, Pure logic (SAFETY-CRITICAL)
	sm.DefineProfile("constitution_gate", core.ShardConfig{
		Name: "constitution_gate",
		Type: core.ShardTypeSystem,
		Permissions: []core.ShardPermission{
			core.PermissionAskUser, // Only for escalation
		},
		Timeout:     24 * 60 * 60 * 1000000000, // 24 hours
		MemoryLimit: 1000,
		Model:       core.ModelConfig{}, // No LLM - safety MUST be deterministic
	})

	// Tactile Router - ON-DEMAND, Pure logic
	sm.DefineProfile("tactile_router", core.ShardConfig{
		Name: "tactile_router",
		Type: core.ShardTypeSystem,
		Permissions: []core.ShardPermission{
			core.PermissionExecCmd,
			core.PermissionNetwork,
			core.PermissionBrowser,
		},
		Timeout:     24 * 60 * 60 * 1000000000, // 24 hours
		MemoryLimit: 2000,
		Model:       core.ModelConfig{}, // No LLM needed
	})

	// Session Planner - ON-DEMAND, LLM for goal decomposition
	sm.DefineProfile("session_planner", core.ShardConfig{
		Name: "session_planner",
		Type: core.ShardTypeSystem,
		Permissions: []core.ShardPermission{
			core.PermissionAskUser,
			core.PermissionReadFile,
		},
		Timeout:     24 * 60 * 60 * 1000000000, // 24 hours
		MemoryLimit: 8000,
		Model: core.ModelConfig{
			Capability: core.CapabilityHighReasoning,
		},
	})
}
