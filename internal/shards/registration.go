// Package shards implements specialized ShardAgent types.
// This file provides registration helpers for the shard manager.
package shards

import (
	"codenerd/internal/articulation"
	"codenerd/internal/config"
	"codenerd/internal/core"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	// Domain shards removed - JIT clean loop handles these via prompt atoms
	// "codenerd/internal/shards/coder"
	// "codenerd/internal/shards/nemesis"
	// "codenerd/internal/shards/researcher"
	// "codenerd/internal/shards/reviewer"
	// "codenerd/internal/shards/tester"
	// "codenerd/internal/shards/tool_generator"
	"codenerd/internal/shards/system"
	"codenerd/internal/store"
	"codenerd/internal/types"
	// "codenerd/internal/world" // Only used by reviewer holographic - removed
)

// RegistryContext holds dependencies for shard dependency injection.
// This solves the "hollow shard" problem by ensuring factories have access
// to the kernel and LLM client at instantiation time.
type RegistryContext struct {
	Kernel       types.Kernel
	LLMClient    perception.LLMClient
	VirtualStore *core.VirtualStore
	Workspace    string
	JITCompiler  *prompt.JITPromptCompiler
	JITConfig    config.JITConfig
}

// learningStoreAdapter adapts store.LearningStore to core.LearningStore
type learningStoreAdapter struct {
	store *store.LearningStore
}

func (a *learningStoreAdapter) Save(shardType, factPredicate string, factArgs []any, sourceCampaign string) error {
	return a.store.Save(shardType, factPredicate, factArgs, sourceCampaign)
}

func (a *learningStoreAdapter) Load(shardType string) ([]types.ShardLearning, error) {
	learnings, err := a.store.Load(shardType)
	if err != nil {
		return nil, err
	}
	// Map store.Learning to types.ShardLearning
	result := make([]types.ShardLearning, len(learnings))
	for i, l := range learnings {
		result[i] = types.ShardLearning{
			FactPredicate: l.FactPredicate,
			FactArgs:      l.FactArgs,
			Confidence:    l.Confidence,
		}
	}
	return result, nil
}

func (a *learningStoreAdapter) DecayConfidence(shardType string, decayFactor float64) error {
	return a.store.DecayConfidence(shardType, decayFactor)
}

func (a *learningStoreAdapter) LoadByPredicate(shardType, predicate string) ([]types.ShardLearning, error) {
	learnings, err := a.store.LoadByPredicate(shardType, predicate)
	if err != nil {
		return nil, err
	}
	// Map store.Learning to types.ShardLearning
	result := make([]types.ShardLearning, len(learnings))
	for i, l := range learnings {
		result[i] = types.ShardLearning{
			FactPredicate: l.FactPredicate,
			FactArgs:      l.FactArgs,
			Confidence:    l.Confidence,
		}
	}
	return result, nil
}

func (a *learningStoreAdapter) Close() error {
	return a.store.Close()
}

// NOTE: holographicAdapter and reviewerFeedbackAdapter removed.
// Domain shards (coder, reviewer, tester, researcher, nemesis, tool_generator)
// are replaced by the JIT clean loop architecture. Their functionality is now
// provided by JIT-compiled prompts with persona atoms and ConfigFactory.

// RegisterAllShardFactories registers all specialized shard factories with the shard manager.
// This should be called during application initialization after creating the shard manager.
func RegisterAllShardFactories(sm *coreshards.ShardManager, ctx RegistryContext) {
	// Ensure ShardManager has the VirtualStore for dynamic injection
	if ctx.VirtualStore != nil {
		sm.SetVirtualStore(ctx.VirtualStore)
	}

	// Helper to safely get LearningStore as interface (all shards use core.LearningStore)
	getLearningStore := func() core.LearningStore {
		if ctx.VirtualStore != nil {
			ls := ctx.VirtualStore.GetLearningStore()
			if ls != nil {
				return &learningStoreAdapter{store: ls}
			}
		}
		return nil
	}

	// Helper to create PromptAssembler with JIT support
	createAssembler := func() *articulation.PromptAssembler {
		if ctx.Kernel == nil {
			return nil
		}
		// Assuming core.Kernel satisfies articulation.KernelQuerier (Fact types act effectively aliased)
		// We might need an adapter if strict go interfaces complain, but for now we try direct.
		// If direct fails compilation, we'll wrap it.
		// prompt.NewPromptAssembler takes articulation.KernelQuerier.
		pa, err := articulation.NewPromptAssembler(ctx.Kernel)
		if err != nil {
			return nil
		}
		if ctx.JITCompiler != nil {
			jitCfg := ctx.JITConfig
			if jitCfg.TokenBudget == 0 && jitCfg.ReservedTokens == 0 && jitCfg.SemanticTopK == 0 && !jitCfg.Enabled && !jitCfg.FallbackEnabled {
				jitCfg = config.DefaultJITConfig()
			}
			pa.SetJITCompiler(ctx.JITCompiler)
			pa.SetJITBudgets(jitCfg.TokenBudget, jitCfg.ReservedTokens, jitCfg.SemanticTopK)
			pa.EnableJIT(jitCfg.Enabled)
		}
		return pa
	}

	// =========================================================================
	// DOMAIN SHARDS REMOVED - JIT CLEAN LOOP ARCHITECTURE
	// =========================================================================
	// The following domain shards have been replaced by the JIT clean loop:
	// - coder: Now handled by session.Executor with /coder persona atoms
	// - reviewer: Now handled by session.Executor with /reviewer persona atoms
	// - tester: Now handled by session.Executor with /tester persona atoms
	// - researcher: Now handled by session.Executor with /researcher persona atoms
	// - tool_generator: Now handled by Ouroboros via VirtualStore
	// - nemesis: Now handled by Thunderdome adversarial testing
	//
	// The JIT prompt compiler assembles the appropriate persona, skills, and
	// context based on user intent. ConfigFactory provides tool sets per intent.
	// =========================================================================

	// Register Requirements Interrogator (Socratic clarifier) - still needed for clarification
	sm.RegisterShard("requirements_interrogator", func(id string, config types.ShardConfig) types.ShardAgent {
		shard := NewRequirementsInterrogatorShard()
		shard.SetLLMClient(ctx.LLMClient)
		shard.SetParentKernel(ctx.Kernel)
		return shard
	})

	// =========================================================================
	// Type 1: System Shards (Permanent, Continuous)
	// =========================================================================

	// Register Perception Firewall - AUTO-START, LLM-primary
	sm.RegisterShard("perception_firewall", func(id string, config types.ShardConfig) types.ShardAgent {
		shard := system.NewPerceptionFirewallShard()
		shard.SetParentKernel(ctx.Kernel)
		shard.SetLLMClient(ctx.LLMClient)
		shard.SetVirtualStore(ctx.VirtualStore)    // FIX: Enable .gitignore/safety rules access
		shard.SetLearningStore(getLearningStore()) // FIX: Enable learning persistence
		shard.SetPromptAssembler(createAssembler())
		return shard
	})

	// Register World Model Ingestor - ON-DEMAND, Hybrid
	sm.RegisterShard("world_model_ingestor", func(id string, config types.ShardConfig) types.ShardAgent {
		shard := system.NewWorldModelIngestorShard()
		shard.SetParentKernel(ctx.Kernel)
		shard.SetVirtualStore(ctx.VirtualStore)
		shard.SetLLMClient(ctx.LLMClient)
		shard.SetPromptAssembler(createAssembler())
		return shard
	})

	// Register Executive Policy - AUTO-START, Logic-primary
	sm.RegisterShard("executive_policy", func(id string, config types.ShardConfig) types.ShardAgent {
		shard := system.NewExecutivePolicyShard()
		shard.SetParentKernel(ctx.Kernel)
		shard.SetVirtualStore(ctx.VirtualStore)
		shard.SetLLMClient(ctx.LLMClient)
		shard.SetLearningStore(getLearningStore()) // FIX: Enable strategy pattern learning
		shard.SetPromptAssembler(createAssembler())
		return shard
	})

	// Register Constitution Gate - AUTO-START, Logic-primary (SAFETY-CRITICAL)
	sm.RegisterShard("constitution_gate", func(id string, config types.ShardConfig) types.ShardAgent {
		shard := system.NewConstitutionGateShard()
		shard.SetParentKernel(ctx.Kernel)
		shard.SetVirtualStore(ctx.VirtualStore)
		shard.SetLLMClient(ctx.LLMClient)
		shard.SetPromptAssembler(createAssembler())
		return shard
	})

	// Register Legislator - ON-DEMAND, Logic-primary (learned constraints)
	sm.RegisterShard("legislator", func(id string, config types.ShardConfig) types.ShardAgent {
		shard := system.NewLegislatorShard()
		shard.SetParentKernel(ctx.Kernel)
		shard.SetVirtualStore(ctx.VirtualStore)
		shard.SetLLMClient(ctx.LLMClient)
		shard.SetPromptAssembler(createAssembler())
		return shard
	})

	// Register Mangle Repair - AUTO-START, Logic-primary (self-healing rules)
	sm.RegisterShard("mangle_repair", func(id string, config types.ShardConfig) types.ShardAgent {
		shard := system.NewMangleRepairShard()
		shard.SetParentKernel(ctx.Kernel)
		shard.SetLLMClient(ctx.LLMClient)
		shard.SetPromptAssembler(createAssembler())
		// Wire the predicate corpus from kernel for schema validation
		// Also wire the shard as the kernel's learned rule interceptor
		if realKernel, ok := ctx.Kernel.(*core.RealKernel); ok {
			if corpus := realKernel.GetPredicateCorpus(); corpus != nil {
				shard.SetCorpus(corpus)
			}
			// Wire repair interceptor for learned rule validation/repair before persistence
			realKernel.SetRepairInterceptor(shard)
		}
		return shard
	})

	// Register Tactile Router - ON-DEMAND, Logic-primary
	sm.RegisterShard("tactile_router", func(id string, config types.ShardConfig) types.ShardAgent {
		shard := system.NewTactileRouterShard()
		shard.SetParentKernel(ctx.Kernel)
		shard.SetVirtualStore(ctx.VirtualStore)
		shard.SetLLMClient(ctx.LLMClient)
		shard.SetPromptAssembler(createAssembler())
		// BrowserManager will be injected separately if available
		return shard
	})

	// Register Campaign Runner - AUTO-START supervisor for long-horizon campaigns
	sm.RegisterShard("campaign_runner", func(id string, config types.ShardConfig) types.ShardAgent {
		shard := system.NewCampaignRunnerShard()
		shard.SetParentKernel(ctx.Kernel)
		shard.SetVirtualStore(ctx.VirtualStore)
		shard.SetLLMClient(ctx.LLMClient)
		shard.SetWorkspaceRoot(ctx.Workspace)
		shard.SetPromptAssembler(createAssembler())
		// Shared ShardManager is injected in system factory to avoid cycles.
		return shard
	})

	// Register Session Planner - ON-DEMAND, LLM-primary
	sm.RegisterShard("session_planner", func(id string, config types.ShardConfig) types.ShardAgent {
		shard := system.NewSessionPlannerShard()
		shard.SetParentKernel(ctx.Kernel)
		shard.SetLLMClient(ctx.LLMClient)
		shard.SetVirtualStore(ctx.VirtualStore) // FIX: Enable codebase scanning for planning
		shard.SetPromptAssembler(createAssembler())
		return shard
	})

	// Define shard profiles with proper configurations
	defineShardProfiles(sm)
}

// defineShardProfiles registers shard profiles with appropriate configurations.
// NOTE: Domain shard profiles (coder, reviewer, tester, researcher, tool_generator, nemesis)
// have been removed. These shards are now handled by the JIT clean loop via session.Executor.
// Only system shard profiles remain.
func defineShardProfiles(sm *coreshards.ShardManager) {
	// Requirements Interrogator profile - Socratic clarification specialist
	// (Kept because it has unique ask-user interaction pattern not covered by JIT)
	sm.DefineProfile("requirements_interrogator", types.ShardConfig{
		Name: "requirements_interrogator",
		Type: types.ShardTypeEphemeral,
		Permissions: []types.ShardPermission{
			types.PermissionAskUser,
			types.PermissionReadFile,
		},
		Timeout:     5 * 60 * 1000000000, // 5 minutes
		MemoryLimit: 6000,
		Model: types.ModelConfig{
			Capability: types.CapabilityBalanced,
		},
	})

	// Define system shard profiles
	defineSystemShardProfiles(sm)
}

// RegisterSystemShardProfiles registers Type 1 system shard profiles.
// This is exported for use by session initialization when factories are
// registered manually with dependency injection.
func RegisterSystemShardProfiles(sm *coreshards.ShardManager) {
	defineSystemShardProfiles(sm)
}

// defineSystemShardProfiles registers Type 1 system shard profiles.
func defineSystemShardProfiles(sm *coreshards.ShardManager) {
	// Perception Firewall - AUTO-START, LLM for NL understanding
	sm.DefineProfile("perception_firewall", types.ShardConfig{
		Name:        "perception_firewall",
		Type:        types.ShardTypeSystem,
		StartupMode: types.StartupAuto,
		Permissions: []types.ShardPermission{
			types.PermissionReadFile,
			types.PermissionAskUser,
		},
		Timeout:     24 * 60 * 60 * 1000000000, // 24 hours (permanent)
		MemoryLimit: 9000,
		Model: types.ModelConfig{
			Capability: types.CapabilityBalanced,
		},
	})

	// World Model Ingestor - ON-DEMAND, Hybrid
	sm.DefineProfile("world_model_ingestor", types.ShardConfig{
		Name:        "world_model_ingestor",
		Type:        types.ShardTypeSystem,
		StartupMode: types.StartupOnDemand,
		Permissions: []types.ShardPermission{
			types.PermissionReadFile,
			types.PermissionExecCmd,
			types.PermissionCodeGraph,
		},
		Timeout:     24 * 60 * 60 * 1000000000, // 24 hours
		MemoryLimit: 20000,
		Model: types.ModelConfig{
			Capability: types.CapabilityHighSpeed,
		},
	})

	// Executive Policy - AUTO-START, Pure logic (no LLM by default)
	sm.DefineProfile("executive_policy", types.ShardConfig{
		Name:        "executive_policy",
		Type:        types.ShardTypeSystem,
		StartupMode: types.StartupAuto,
		Permissions: []types.ShardPermission{
			types.PermissionReadFile,
			types.PermissionCodeGraph,
			types.PermissionAskUser,
		},
		Timeout:     24 * 60 * 60 * 1000000000, // 24 hours
		MemoryLimit: 7000,
		Model:       types.ModelConfig{}, // No LLM needed for core logic
	})

	// Constitution Gate - AUTO-START, Pure logic (SAFETY-CRITICAL)
	sm.DefineProfile("constitution_gate", types.ShardConfig{
		Name:        "constitution_gate",
		Type:        types.ShardTypeSystem,
		StartupMode: types.StartupAuto,
		Permissions: []types.ShardPermission{
			types.PermissionAskUser, // Only for escalation
		},
		Timeout:     24 * 60 * 60 * 1000000000, // 24 hours
		MemoryLimit: 2500,
		Model:       types.ModelConfig{}, // No LLM - safety MUST be deterministic
	})

	// Mangle Repair - AUTO-START, learned rule validation/repair
	sm.DefineProfile("mangle_repair", types.ShardConfig{
		Name:        "mangle_repair",
		Type:        types.ShardTypeSystem,
		StartupMode: types.StartupAuto,
		Permissions: []types.ShardPermission{
			types.PermissionReadFile,
		},
		Timeout:     24 * 60 * 60 * 1000000000, // 24 hours
		MemoryLimit: 6000,
		Model: types.ModelConfig{
			Capability: types.CapabilityHighReasoning,
		},
	})

	// Tactile Router - ON-DEMAND, Pure logic
	sm.DefineProfile("tactile_router", types.ShardConfig{
		Name:        "tactile_router",
		Type:        types.ShardTypeSystem,
		StartupMode: types.StartupOnDemand,
		Permissions: []types.ShardPermission{
			types.PermissionExecCmd,
			types.PermissionNetwork,
			types.PermissionBrowser,
		},
		Timeout:     24 * 60 * 60 * 1000000000, // 24 hours
		MemoryLimit: 6000,
		Model:       types.ModelConfig{}, // No LLM needed
	})

	// Session Planner - ON-DEMAND, LLM for goal decomposition
	sm.DefineProfile("session_planner", types.ShardConfig{
		Name:        "session_planner",
		Type:        types.ShardTypeSystem,
		StartupMode: types.StartupOnDemand,
		Permissions: []types.ShardPermission{
			types.PermissionAskUser,
			types.PermissionReadFile,
		},
		Timeout:     24 * 60 * 60 * 1000000000, // 24 hours
		MemoryLimit: 16000,
		Model: types.ModelConfig{
			Capability: types.CapabilityHighReasoning,
		},
	})

	// Campaign Runner - ON-DEMAND, supervisor (uses orchestrator + shards)
	// NOTE: Changed to ON-DEMAND to prevent automatic campaign execution on boot
	sm.DefineProfile("campaign_runner", types.ShardConfig{
		Name:        "campaign_runner",
		Type:        types.ShardTypeSystem,
		StartupMode: types.StartupOnDemand,
		Permissions: []types.ShardPermission{
			types.PermissionReadFile,
			types.PermissionWriteFile,
			types.PermissionExecCmd,
		},
		Timeout:     24 * 60 * 60 * 1000000000, // 24 hours
		MemoryLimit: 6000,
		Model: types.ModelConfig{
			Capability: types.CapabilityBalanced,
		},
	})

	// Legislator - ON-DEMAND, Logic-primary for learned constraints
	sm.DefineProfile("legislator", types.ShardConfig{
		Name:        "legislator",
		Type:        types.ShardTypeSystem,
		StartupMode: types.StartupOnDemand,
		Permissions: []types.ShardPermission{
			types.PermissionReadFile,
			types.PermissionCodeGraph,
		},
		Timeout:     24 * 60 * 60 * 1000000000, // 24 hours
		MemoryLimit: 4000,
		Model:       types.ModelConfig{}, // No LLM - constraint synthesis is logic-primary
	})
}
