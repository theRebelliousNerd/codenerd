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
	"codenerd/internal/shards/system"
	"codenerd/internal/store"
	"codenerd/internal/types"
)

// ShardPredicateManifest is the per-shard contract that names every predicate
// the shard is the authoritative owner of. It is consumed by the cortex /
// shard-fact-router (Track D) so that when features.IsPerShardFactsEnabled()
// is true, a fact asserted on the wrong shard gets routed to its owner
// instead of silently landing in a non-authoritative store.
//
// The manifest is intentionally a flat data structure with no behavior. It
// is constructed once at startup and handed to the kernel/cortex layer; the
// shard factories below remain unchanged so the off-flag path is byte-
// identical to today.
//
// Wiring note: the production construction of *core.KernelShard happens in
// internal/system/factory.go, which is owned by a separate marathon track
// and is OUTSIDE this file's edit lane. Code that consumes this manifest
// will land in that file in a subsequent pass. Until then, the manifest is
// exported so other lanes can pick it up without bouncing through here.
type ShardPredicateManifest struct {
	Domain          string   // Shard domain name, matches core.KernelShardConfig.Domain
	OwnedPredicates []string // Predicates this shard is authoritative for
}

// DefaultShardPredicateManifests returns the canonical predicate ownership
// table for codeNERD's domain shards. It mirrors the OwnedPredicates lists
// currently hard-coded in internal/system/factory.go so that the two
// converge once factory.go is wired to read from here.
//
// Ordering matches the registration order in factory.go for readability;
// the router does not depend on order.
func DefaultShardPredicateManifests() []ShardPredicateManifest {
	return []ShardPredicateManifest{
		{
			Domain:          "routing",
			OwnedPredicates: []string{"user_intent", "next_action", "routing_result", "derived_mode"},
		},
		{
			Domain:          "world",
			OwnedPredicates: []string{"file_topology", "symbol_graph", "diagnostic", "project_profile"},
		},
		{
			Domain:          "tools",
			OwnedPredicates: []string{"tool_capabilities", "shard_lifecycle", "shell_exec_result"},
		},
		{
			Domain:          "policy",
			OwnedPredicates: []string{"permitted", "blocked", "constitution", "commit_barrier", "dangerous_action"},
		},
		{
			Domain:          "campaign",
			OwnedPredicates: []string{"campaign", "campaign_phase", "campaign_task", "campaign_dependency"},
		},
		{
			Domain:          "prompts",
			OwnedPredicates: []string{"prompt_atom", "atom_selection_score", "shard_prompt_base"},
		},
		{
			Domain:          "cortex",
			OwnedPredicates: nil, // Catch-all for unowned predicates
		},
	}
}

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
	// NERD-EVOLVE-START: P1P2-model-tiering
	// ClassificationClient, when non-nil, is used by the PerceptionFirewallShard
	// for intent classification calls instead of the main LLMClient.
	// This enables routing perception to a faster/cheaper model (e.g. Haiku, Gemini Flash)
	// while keeping the main generation model for longer, higher-quality responses.
	// When nil, the perception shard falls back to LLMClient.
	ClassificationClient perception.LLMClient
	// NERD-EVOLVE-END: P1P2-model-tiering
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

type shardFactoryRegistrar struct {
	sm  *coreshards.ShardManager
	ctx RegistryContext
}

func (r *shardFactoryRegistrar) getLearningStore() core.LearningStore {
	if r.ctx.VirtualStore != nil {
		ls := r.ctx.VirtualStore.GetLearningStore()
		if ls != nil {
			return &learningStoreAdapter{store: ls}
		}
	}
	return nil
}

func (r *shardFactoryRegistrar) createAssembler() *articulation.PromptAssembler {
	if r.ctx.Kernel == nil {
		return nil
	}
	pa, err := articulation.NewPromptAssembler(r.ctx.Kernel)
	if err != nil {
		return nil
	}
	if r.ctx.JITCompiler != nil {
		jitCfg := r.ctx.JITConfig
		if jitCfg.TokenBudget == 0 && jitCfg.ReservedTokens == 0 && jitCfg.SemanticTopK == 0 && !jitCfg.Enabled && !jitCfg.FallbackEnabled {
			jitCfg = config.DefaultJITConfig()
		}
		pa.SetJITCompiler(r.ctx.JITCompiler)
		pa.SetJITBudgets(jitCfg.TokenBudget, jitCfg.ReservedTokens, jitCfg.SemanticTopK, jitCfg.ReservedTokensFallbackRatio)
		pa.EnableJIT(jitCfg.Enabled)
	}
	return pa
}

func (r *shardFactoryRegistrar) withJITConfig(agent types.ShardAgent) types.ShardAgent {
	if setter, ok := agent.(interface{ SetJITConfig(config.JITConfig) }); ok {
		setter.SetJITConfig(r.ctx.JITConfig)
	}
	return agent
}

func (r *shardFactoryRegistrar) registerEphemeralShards() {
	r.sm.RegisterShard("requirements_interrogator", func(id string, config types.ShardConfig) types.ShardAgent {
		shard := NewRequirementsInterrogatorShard()
		shard.SetLLMClient(r.ctx.LLMClient)
		shard.SetParentKernel(r.ctx.Kernel)
		return r.withJITConfig(shard)
	})
}

func (r *shardFactoryRegistrar) registerSystemShards() {
	r.sm.RegisterShard("perception_firewall", func(id string, config types.ShardConfig) types.ShardAgent {
		shard := system.NewPerceptionFirewallShard()
		shard.SetParentKernel(r.ctx.Kernel)
		shard.SetLLMClient(r.ctx.LLMClient)
		shard.SetVirtualStore(r.ctx.VirtualStore)    // FIX: Enable .gitignore/safety rules access
		shard.SetLearningStore(r.getLearningStore()) // FIX: Enable learning persistence
		shard.SetPromptAssembler(r.createAssembler())
		// NERD-EVOLVE-START: P1P2-model-tiering
		if r.ctx.ClassificationClient != nil {
			shard.SetClassificationClient(r.ctx.ClassificationClient)
		}
		// NERD-EVOLVE-END: P1P2-model-tiering
		return r.withJITConfig(shard)
	})

	r.sm.RegisterShard("world_model_ingestor", func(id string, config types.ShardConfig) types.ShardAgent {
		shard := system.NewWorldModelIngestorShard()
		shard.SetParentKernel(r.ctx.Kernel)
		shard.SetVirtualStore(r.ctx.VirtualStore)
		shard.SetLLMClient(r.ctx.LLMClient)
		shard.SetPromptAssembler(r.createAssembler())
		return r.withJITConfig(shard)
	})
}

func (r *shardFactoryRegistrar) registerLogicShards() {
	r.sm.RegisterShard("executive_policy", func(id string, config types.ShardConfig) types.ShardAgent {
		shard := system.NewExecutivePolicyShard()
		shard.SetParentKernel(r.ctx.Kernel)
		shard.SetVirtualStore(r.ctx.VirtualStore)
		shard.SetLLMClient(r.ctx.LLMClient)
		shard.SetLearningStore(r.getLearningStore()) // FIX: Enable strategy pattern learning
		shard.SetPromptAssembler(r.createAssembler())
		return r.withJITConfig(shard)
	})

	r.sm.RegisterShard("constitution_gate", func(id string, config types.ShardConfig) types.ShardAgent {
		shard := system.NewConstitutionGateShard()
		shard.SetParentKernel(r.ctx.Kernel)
		shard.SetVirtualStore(r.ctx.VirtualStore)
		shard.SetLLMClient(r.ctx.LLMClient)
		shard.SetPromptAssembler(r.createAssembler())
		return r.withJITConfig(shard)
	})

	r.sm.RegisterShard("legislator", func(id string, config types.ShardConfig) types.ShardAgent {
		shard := system.NewLegislatorShard()
		shard.SetParentKernel(r.ctx.Kernel)
		shard.SetVirtualStore(r.ctx.VirtualStore)
		shard.SetLLMClient(r.ctx.LLMClient)
		shard.SetPromptAssembler(r.createAssembler())
		return r.withJITConfig(shard)
	})

	r.sm.RegisterShard("mangle_repair", func(id string, config types.ShardConfig) types.ShardAgent {
		shard := system.NewMangleRepairShard()
		shard.SetParentKernel(r.ctx.Kernel)
		shard.SetLLMClient(r.ctx.LLMClient)
		shard.SetPromptAssembler(r.createAssembler())

		var realKernel *core.RealKernel
		if rk, ok := r.ctx.Kernel.(*core.RealKernel); ok {
			realKernel = rk
		} else if ck, ok := r.ctx.Kernel.(*core.CortexKernel); ok {
			realKernel = ck.GetPrimaryRealKernel()
		}
		if realKernel != nil {
			if corpus := realKernel.GetPredicateCorpus(); corpus != nil {
				shard.SetCorpus(corpus)
			}
			realKernel.SetRepairInterceptor(shard)
		}
		return r.withJITConfig(shard)
	})
}

func (r *shardFactoryRegistrar) registerPlanningShards() {
	r.sm.RegisterShard("tactile_router", func(id string, config types.ShardConfig) types.ShardAgent {
		shard := system.NewTactileRouterShard()
		shard.SetParentKernel(r.ctx.Kernel)
		shard.SetVirtualStore(r.ctx.VirtualStore)
		shard.SetLLMClient(r.ctx.LLMClient)
		shard.SetPromptAssembler(r.createAssembler())
		return r.withJITConfig(shard)
	})

	r.sm.RegisterShard("campaign_runner", func(id string, config types.ShardConfig) types.ShardAgent {
		shard := system.NewCampaignRunnerShard()
		shard.SetParentKernel(r.ctx.Kernel)
		shard.SetVirtualStore(r.ctx.VirtualStore)
		shard.SetLLMClient(r.ctx.LLMClient)
		shard.SetWorkspaceRoot(r.ctx.Workspace)
		shard.SetPromptAssembler(r.createAssembler())
		return r.withJITConfig(shard)
	})

	r.sm.RegisterShard("session_planner", func(id string, config types.ShardConfig) types.ShardAgent {
		shard := system.NewSessionPlannerShard()
		shard.SetParentKernel(r.ctx.Kernel)
		shard.SetLLMClient(r.ctx.LLMClient)
		shard.SetVirtualStore(r.ctx.VirtualStore) // FIX: Enable codebase scanning for planning
		shard.SetPromptAssembler(r.createAssembler())
		return r.withJITConfig(shard)
	})
}

// RegisterAllShardFactories registers all specialized shard factories with the shard manager.
// This should be called during application initialization after creating the shard manager.
func RegisterAllShardFactories(sm *coreshards.ShardManager, ctx RegistryContext) {
	// Ensure ShardManager has the VirtualStore for dynamic injection
	if ctx.VirtualStore != nil {
		sm.SetVirtualStore(ctx.VirtualStore)
	}

	registrar := &shardFactoryRegistrar{
		sm:  sm,
		ctx: ctx,
	}

	registrar.registerEphemeralShards()
	registrar.registerSystemShards()
	registrar.registerLogicShards()
	registrar.registerPlanningShards()

	// Define shard profiles with proper configurations
	defineShardProfiles(sm)
}

// defineShardProfiles registers shard profiles with appropriate configurations.
// Domain shards (coder, reviewer, tester, researcher) are now handled by the
// JIT clean loop via session.Executor and prompt atoms. Only system shards
// and special-purpose shards (like requirements_interrogator) remain here.
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
	definePerceptionFirewallProfile(sm)
	defineWorldModelIngestorProfile(sm)
	defineExecutivePolicyProfile(sm)
	defineConstitutionGateProfile(sm)
	defineMangleRepairProfile(sm)
	defineTactileRouterProfile(sm)
	defineSessionPlannerProfile(sm)
	defineCampaignRunnerProfile(sm)
	defineLegislatorProfile(sm)
}

func definePerceptionFirewallProfile(sm *coreshards.ShardManager) {
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
}

func defineWorldModelIngestorProfile(sm *coreshards.ShardManager) {
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
}

func defineExecutivePolicyProfile(sm *coreshards.ShardManager) {
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
}

func defineConstitutionGateProfile(sm *coreshards.ShardManager) {
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
}

func defineMangleRepairProfile(sm *coreshards.ShardManager) {
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
}

func defineTactileRouterProfile(sm *coreshards.ShardManager) {
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
}

func defineSessionPlannerProfile(sm *coreshards.ShardManager) {
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
}

func defineCampaignRunnerProfile(sm *coreshards.ShardManager) {
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
}

func defineLegislatorProfile(sm *coreshards.ShardManager) {
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
