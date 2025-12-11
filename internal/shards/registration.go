// Package shards implements specialized ShardAgent types.
// This file provides registration helpers for the shard manager.
package shards

import (
	"codenerd/internal/articulation"
	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/shards/coder"
	"codenerd/internal/shards/nemesis"
	"codenerd/internal/shards/researcher"
	"codenerd/internal/shards/reviewer"
	"codenerd/internal/shards/system"
	"codenerd/internal/shards/tester"
	"codenerd/internal/shards/tool_generator"
	"codenerd/internal/store"
	"codenerd/internal/world"
)

// RegistryContext holds dependencies for shard dependency injection.
// This solves the "hollow shard" problem by ensuring factories have access
// to the kernel and LLM client at instantiation time.
type RegistryContext struct {
	Kernel       core.Kernel
	LLMClient    perception.LLMClient
	VirtualStore *core.VirtualStore
	Workspace    string
	JITCompiler  *prompt.JITPromptCompiler
}

// learningStoreAdapter adapts store.LearningStore to core.LearningStore
type learningStoreAdapter struct {
	store *store.LearningStore
}

func (a *learningStoreAdapter) Save(shardType, factPredicate string, factArgs []any, sourceCampaign string) error {
	return a.store.Save(shardType, factPredicate, factArgs, sourceCampaign)
}

func (a *learningStoreAdapter) Load(shardType string) ([]core.ShardLearning, error) {
	learnings, err := a.store.Load(shardType)
	if err != nil {
		return nil, err
	}
	// Map store.Learning to core.ShardLearning
	result := make([]core.ShardLearning, len(learnings))
	for i, l := range learnings {
		result[i] = core.ShardLearning{
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

func (a *learningStoreAdapter) LoadByPredicate(shardType, predicate string) ([]core.ShardLearning, error) {
	learnings, err := a.store.LoadByPredicate(shardType, predicate)
	if err != nil {
		return nil, err
	}
	// Map store.Learning to core.ShardLearning
	result := make([]core.ShardLearning, len(learnings))
	for i, l := range learnings {
		result[i] = core.ShardLearning{
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

// holographicAdapter adapts world.HolographicProvider to reviewer.HolographicProvider
type holographicAdapter struct {
	provider *world.HolographicProvider
}

func (h *holographicAdapter) GetContext(filePath string) (*reviewer.HolographicContext, error) {
	ctx, err := h.provider.GetContext(filePath)
	if err != nil {
		return nil, err
	}

	// Convert world.HolographicContext to reviewer.HolographicContext
	result := &reviewer.HolographicContext{
		TargetFile:      ctx.TargetFile,
		TargetPkg:       ctx.TargetPkg,
		PackageSiblings: ctx.PackageSiblings,
		Layer:           ctx.Layer,
		Module:          ctx.Module,
		Role:            ctx.Role,
		SystemPurpose:   ctx.SystemPurpose,
		HasTests:        ctx.HasTests,
	}

	// Convert signatures
	for _, sig := range ctx.PackageSignatures {
		result.PackageSignatures = append(result.PackageSignatures, reviewer.SymbolSignature{
			Name:       sig.Name,
			Receiver:   sig.Receiver,
			Params:     sig.Params,
			Returns:    sig.Returns,
			File:       sig.File,
			Line:       sig.Line,
			Exported:   sig.Exported,
			DocComment: sig.DocComment,
		})
	}

	// Convert types
	for _, t := range ctx.PackageTypes {
		result.PackageTypes = append(result.PackageTypes, reviewer.TypeDefinition{
			Name:     t.Name,
			Kind:     t.Kind,
			Fields:   t.Fields,
			Methods:  t.Methods,
			File:     t.File,
			Line:     t.Line,
			Exported: t.Exported,
		})
	}

	return result, nil
}

// reviewerFeedbackAdapter adapts reviewer.ReviewerShard to core.ReviewerFeedbackProvider
type reviewerFeedbackAdapter struct {
	shard *reviewer.ReviewerShard
}

func (r *reviewerFeedbackAdapter) NeedsValidation(reviewID string) bool {
	return r.shard.NeedsValidation(reviewID)
}

func (r *reviewerFeedbackAdapter) GetSuspectReasons(reviewID string) []string {
	return r.shard.GetSuspectReasons(reviewID)
}

func (r *reviewerFeedbackAdapter) AcceptFinding(reviewID, file string, line int) {
	r.shard.AcceptFinding(reviewID, file, line)
}

func (r *reviewerFeedbackAdapter) RejectFinding(reviewID, file string, line int, reason string) {
	r.shard.RejectFinding(reviewID, file, line, reason)
}

func (r *reviewerFeedbackAdapter) GetAccuracyReport(reviewID string) string {
	return r.shard.GetAccuracyReport(reviewID)
}

// RegisterAllShardFactories registers all specialized shard factories with the shard manager.
// This should be called during application initialization after creating the shard manager.
func RegisterAllShardFactories(sm *core.ShardManager, ctx RegistryContext) {
	// Ensure ShardManager has the VirtualStore for dynamic injection
	if ctx.VirtualStore != nil {
		sm.SetVirtualStore(ctx.VirtualStore)
	}

	// Helper to safely get LocalDB from VirtualStore
	getLocalDB := func() *store.LocalStore {
		if ctx.VirtualStore != nil {
			return ctx.VirtualStore.GetLocalDB()
		}
		return nil
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
			pa.SetJITCompiler(ctx.JITCompiler)
			pa.EnableJIT(true) // Enable by default if compiler is present
		}
		return pa
	}

	// Be defensive about interface satisfaction - core.Kernel returns []core.Fact,
	// articulation.KernelQuerier expects []types.Fact. They are aliases but Go type system
	// might require specific casting. Let's see if compilation passes.

	// Register Coder shard factory
	sm.RegisterShard("coder", func(id string, config core.ShardConfig) core.ShardAgent {
		shard := coder.NewCoderShard()
		shard.SetParentKernel(ctx.Kernel)
		shard.SetVirtualStore(ctx.VirtualStore)
		shard.SetLLMClient(ctx.LLMClient)
		shard.SetLearningStore(getLearningStore()) // FIX: Enable learning persistence
		shard.SetPromptAssembler(createAssembler())
		return shard
	})

	// Register Reviewer shard factory
	sm.RegisterShard("reviewer", func(id string, config core.ShardConfig) core.ShardAgent {
		shard := reviewer.NewReviewerShard()
		shard.SetParentKernel(ctx.Kernel)
		shard.SetVirtualStore(ctx.VirtualStore)
		shard.SetLLMClient(ctx.LLMClient)
		shard.SetLearningStore(getLearningStore()) // FIX: Enable learning persistence

		// NEW: Inject holographic provider for package-aware code review
		if realKernel, ok := ctx.Kernel.(*core.RealKernel); ok {
			holoProvider := world.NewHolographicProvider(realKernel, ctx.Workspace)
			shard.SetHolographicProvider(&holographicAdapter{provider: holoProvider})
		}

		// NEW: Register as feedback provider for validation triggers
		core.SetReviewerFeedbackProvider(&reviewerFeedbackAdapter{shard: shard})

		shard.SetPromptAssembler(createAssembler())
		return shard
	})

	// Register Tester shard factory
	sm.RegisterShard("tester", func(id string, config core.ShardConfig) core.ShardAgent {
		shard := tester.NewTesterShard()
		shard.SetParentKernel(ctx.Kernel)
		shard.SetVirtualStore(ctx.VirtualStore)
		shard.SetLLMClient(ctx.LLMClient)
		shard.SetLearningStore(getLearningStore()) // FIX: Enable learning persistence
		shard.SetPromptAssembler(createAssembler())
		return shard
	})

	// Register Researcher shard factory
	sm.RegisterShard("researcher", func(id string, config core.ShardConfig) core.ShardAgent {
		shard := researcher.NewResearcherShard()
		shard.SetParentKernel(ctx.Kernel)
		shard.SetVirtualStore(ctx.VirtualStore)
		shard.SetLLMClient(ctx.LLMClient)
		shard.SetLocalDB(getLocalDB())             // FIX: Enable knowledge atom storage
		shard.SetLearningStore(getLearningStore()) // FIX: Enable learning persistence
		// Use Workspace from context
		shard.SetWorkspaceRoot(ctx.Workspace)
		shard.SetPromptAssembler(createAssembler())
		return shard
	})

	// Register Requirements Interrogator (Socratic clarifier)
	sm.RegisterShard("requirements_interrogator", func(id string, config core.ShardConfig) core.ShardAgent {
		shard := NewRequirementsInterrogatorShard()
		shard.SetLLMClient(ctx.LLMClient)
		shard.SetParentKernel(ctx.Kernel) // FIX: Enable kernel access
		// shard.SetVirtualStore(ctx.VirtualStore) // Removed: Interrogator doesn't support VirtualStore yet
		return shard
	})

	// Register ToolGenerator shard factory (autopoiesis)
	sm.RegisterShard("tool_generator", func(id string, config core.ShardConfig) core.ShardAgent {
		shard := tool_generator.NewToolGeneratorShard(id, config)
		shard.SetParentKernel(ctx.Kernel)
		shard.SetWorkspaceRoot(ctx.Workspace) // MUST be called before SetLLMClient
		shard.SetLLMClient(ctx.LLMClient)
		shard.SetLearningStore(getLearningStore()) // FIX: Enable learning persistence for Type B shard
		shard.SetVirtualStore(ctx.VirtualStore)    // FIX: Enable tool execution
		return shard
	})

	// Register Nemesis shard factory (adversarial co-evolution)
	sm.RegisterShard("nemesis", func(id string, config core.ShardConfig) core.ShardAgent {
		shard := nemesis.NewNemesisShard()
		shard.SetParentKernel(ctx.Kernel)
		shard.SetVirtualStore(ctx.VirtualStore)
		shard.SetLLMClient(ctx.LLMClient)
		shard.SetLearningStore(getLearningStore())
		// Initialize Armory for regression test persistence
		if ctx.Workspace != "" {
			armory := nemesis.NewArmory(ctx.Workspace + "/.nerd")
			shard.SetArmory(armory)
		}
		shard.SetPromptAssembler(createAssembler())
		return shard
	})

	// =========================================================================
	// Type 1: System Shards (Permanent, Continuous)
	// =========================================================================

	// Register Perception Firewall - AUTO-START, LLM-primary
	sm.RegisterShard("perception_firewall", func(id string, config core.ShardConfig) core.ShardAgent {
		shard := system.NewPerceptionFirewallShard()
		shard.SetParentKernel(ctx.Kernel)
		shard.SetLLMClient(ctx.LLMClient)
		shard.SetVirtualStore(ctx.VirtualStore)    // FIX: Enable .gitignore/safety rules access
		shard.SetLearningStore(getLearningStore()) // FIX: Enable learning persistence
		return shard
	})

	// Register World Model Ingestor - ON-DEMAND, Hybrid
	sm.RegisterShard("world_model_ingestor", func(id string, config core.ShardConfig) core.ShardAgent {
		shard := system.NewWorldModelIngestorShard()
		shard.SetParentKernel(ctx.Kernel)
		shard.SetVirtualStore(ctx.VirtualStore)
		shard.SetLLMClient(ctx.LLMClient)
		return shard
	})

	// Register Executive Policy - AUTO-START, Logic-primary
	sm.RegisterShard("executive_policy", func(id string, config core.ShardConfig) core.ShardAgent {
		shard := system.NewExecutivePolicyShard()
		shard.SetParentKernel(ctx.Kernel)
		shard.SetVirtualStore(ctx.VirtualStore)
		shard.SetLLMClient(ctx.LLMClient)
		shard.SetLearningStore(getLearningStore()) // FIX: Enable strategy pattern learning
		return shard
	})

	// Register Constitution Gate - AUTO-START, Logic-primary (SAFETY-CRITICAL)
	sm.RegisterShard("constitution_gate", func(id string, config core.ShardConfig) core.ShardAgent {
		shard := system.NewConstitutionGateShard()
		shard.SetParentKernel(ctx.Kernel)
		shard.SetVirtualStore(ctx.VirtualStore)
		shard.SetLLMClient(ctx.LLMClient)
		return shard
	})

	// Register Legislator - ON-DEMAND, Logic-primary (learned constraints)
	sm.RegisterShard("legislator", func(id string, config core.ShardConfig) core.ShardAgent {
		shard := system.NewLegislatorShard()
		shard.SetParentKernel(ctx.Kernel)
		shard.SetVirtualStore(ctx.VirtualStore)
		shard.SetLLMClient(ctx.LLMClient)
		return shard
	})

	// Register Mangle Repair - AUTO-START, Logic-primary (self-healing rules)
	sm.RegisterShard("mangle_repair", func(id string, config core.ShardConfig) core.ShardAgent {
		shard := system.NewMangleRepairShard()
		shard.SetParentKernel(ctx.Kernel)
		shard.SetLLMClient(ctx.LLMClient)
		// Wire the predicate corpus from kernel for schema validation
		if realKernel, ok := ctx.Kernel.(*core.RealKernel); ok {
			if corpus := realKernel.GetPredicateCorpus(); corpus != nil {
				shard.SetCorpus(corpus)
			}
		}
		return shard
	})

	// Register Tactile Router - ON-DEMAND, Logic-primary
	sm.RegisterShard("tactile_router", func(id string, config core.ShardConfig) core.ShardAgent {
		shard := system.NewTactileRouterShard()
		shard.SetParentKernel(ctx.Kernel)
		shard.SetVirtualStore(ctx.VirtualStore)
		shard.SetLLMClient(ctx.LLMClient)
		// BrowserManager will be injected separately if available
		return shard
	})

	// Register Session Planner - ON-DEMAND, LLM-primary
	sm.RegisterShard("session_planner", func(id string, config core.ShardConfig) core.ShardAgent {
		shard := system.NewSessionPlannerShard()
		shard.SetParentKernel(ctx.Kernel)
		shard.SetLLMClient(ctx.LLMClient)
		shard.SetVirtualStore(ctx.VirtualStore) // FIX: Enable codebase scanning for planning
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
		MemoryLimit: 12000,
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
		MemoryLimit: 8000,
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
		MemoryLimit: 8000,
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
		MemoryLimit: 12000,
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
		MemoryLimit: 20000,
		Model: core.ModelConfig{
			Capability: core.CapabilityHighReasoning,
		},
	})

	// Nemesis profile - adversarial co-evolution specialist
	sm.DefineProfile("nemesis", core.ShardConfig{
		Name: "nemesis",
		Type: core.ShardTypePersistent,
		Permissions: []core.ShardPermission{
			core.PermissionReadFile,
			core.PermissionExecCmd,
			core.PermissionCodeGraph,
		},
		Timeout:     20 * 60 * 1000000000, // 20 minutes (adversarial analysis can take time)
		MemoryLimit: 16000,
		Model: core.ModelConfig{
			Capability: core.CapabilityHighReasoning, // Needs reasoning to find weaknesses
		},
	})

	// Requirements Interrogator profile - Socratic clarification specialist
	sm.DefineProfile("requirements_interrogator", core.ShardConfig{
		Name: "requirements_interrogator",
		Type: core.ShardTypeEphemeral,
		Permissions: []core.ShardPermission{
			core.PermissionAskUser,
			core.PermissionReadFile,
		},
		Timeout:     5 * 60 * 1000000000, // 5 minutes
		MemoryLimit: 6000,
		Model: core.ModelConfig{
			Capability: core.CapabilityBalanced,
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
		MemoryLimit: 9000,
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
		MemoryLimit: 20000,
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
		MemoryLimit: 7000,
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
		MemoryLimit: 2500,
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
		MemoryLimit: 6000,
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
		MemoryLimit: 16000,
		Model: core.ModelConfig{
			Capability: core.CapabilityHighReasoning,
		},
	})

	// Legislator - ON-DEMAND, Logic-primary for learned constraints
	sm.DefineProfile("legislator", core.ShardConfig{
		Name: "legislator",
		Type: core.ShardTypeSystem,
		Permissions: []core.ShardPermission{
			core.PermissionReadFile,
			core.PermissionCodeGraph,
		},
		Timeout:     24 * 60 * 60 * 1000000000, // 24 hours
		MemoryLimit: 4000,
		Model:       core.ModelConfig{}, // No LLM - constraint synthesis is logic-primary
	})
}
