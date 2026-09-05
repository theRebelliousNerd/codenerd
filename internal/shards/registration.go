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
// The manifest is intentionally a flat data structure with no behavior.
// internal/system/factory.go consumes it when constructing the production
// Cortex shards, making this the single source of predicate ownership truth.
type ShardPredicateManifest struct {
	Domain          string   // Shard domain name, matches core.KernelShardConfig.Domain
	OwnedPredicates []string // Predicates this shard is authoritative for
}

// DefaultShardPredicateManifests returns the canonical predicate ownership
// table for codeNERD's domain shards.
//
// Ordering matches the registration order in factory.go for readability;
// the router does not depend on order.
func DefaultShardPredicateManifests() []ShardPredicateManifest {
	return []ShardPredicateManifest{
		{
			Domain: "routing",
			// user_intent and next_action are deliberately NOT owned here.
			// user_intent is shared (SharedPredicates) because rules in
			// every domain join it; next_action is derived wherever its
			// rule's other facts live, so a query must fan out.
			// routing_result is asserted by the constitution and router
			// system shards and joined against ready_for_routing (policy),
			// so it lives in the policy shard.
			OwnedPredicates: []string{"derived_mode"},
		},
		{
			Domain: "world",
			// The whole world-model family. Reviewer, CodeDOM, impact and
			// context-priority rules join these against file_topology and
			// symbol_graph; a split (dependency_link in the catch-all,
			// symbol_graph here) silently killed every such rule.
			OwnedPredicates: []string{
				"file_topology", "symbol_graph", "diagnostic", "project_profile",
				"code_element", "element_visibility", "element_modified",
				"code_implements", "code_calls", "dependency_link",
				"file_dir", "modified", "active_file", "in_scope",
				"churn_rate", "same_package", "imports", "file_contains",
				"file_has_public_api", "package_has_dep", "test_coverage",
				"test_failed", "modified_interface", "impact_graph",
				"recent_change_by_other", "coder_context_priority",
				// CodeDOM element facts and the per-language surface
				// (codedom_*.mg, coder_*.mg, reviewer.mg, tester.mg).
				"element_parent", "element_signature", "code_interactable",
				"generated_code", "cgo_code", "parse_error", "file_hash_mismatch",
				"file_modified_externally", "is_test_function", "test_file_for",
				"file_imports", "file_package", "file_in_scope", "type_embeds",
				"is_interface_file", "type_definition_file",
				"api_handler_function", "api_client_function", "api_dependency",
				"go_struct", "py_class", "py_decorator", "py_typed_function",
				"has_pydantic_base", "rs_struct", "rs_derive", "rs_serde_rename",
				"rs_unsafe_block", "ts_interface", "ts_interface_prop", "ts_class",
				"mg_negation_rule", "mg_recursive_rule",
				"cyclomatic_complexity", "git_history", "review_finding",
				"pytest_failure", "reachability_query",
				// Edit plans and pending mutations are judged against the
				// dependency graph (coder_impact.mg, coder_safety.mg,
				// commit_gate.mg).
				"coder_target", "plan_edit", "pending_edit", "pending_mutation",
				"modified_function", "modified_file",
				// Tool domain facts are joined only against file_topology
				// (tool_routing.mg, policy_mcp.mg).
				"tool_domain", "mcp_tool_domain", "mcp_tool_registered", "mcp_server_status",
				"mcp_tool_capability", "mcp_tool_shard_affinity", "mcp_tool_avg_latency",
				"mcp_tool_usage", "mcp_tool_vector_score", "mcp_tool_category",
				// Coder-shard working state judged against the world model
				// (coder_*.mg): the file under edit, its diagnostics, tests.
				"coder_state", "coder_task", "active_review", "diagnostic_count",
				"file_extension", "path_contains", "dependent_count", "edit_analysis",
				"edit_operation", "file_edited", "failing_test", "pytest_root_cause",
				"is_core_file", "is_test_file", "is_binary_file", "project_forbidden_path",
				"high_element_count_flag", "interface_definition", "retry_count",
				"symbol_verified_exists", "traceback_frame", "assertion_mismatch",
				"is_public_api",
				// Facts negated by coder/commit rules whose positive side is
				// world-model data; a negation evaluated in a shard that never
				// holds the fact always succeeds (blind negation).
				"path_in_workspace", "doc_exists_for", "created_source",
				"suppression", "file_content", "entry_point",
				// The tool registry (tool_registry.go) is joined against
				// file_topology by tool_routing.mg and against the shared
				// per-turn facts by stage_context.mg.
				"tool_registered", "tool_capability", "tool_hash", "tool_source",
				"tool_exists", "tool_usage_stats", "tool_description",
				"tool_available", "tool_binary_path", "tool_source_ready",
				"tool_safety_verified", "tool_compiled",
				"tool_generation_blocked", "tool_lifecycle", "tool_known_issue",
				"capability_similar_to", "tool_refined", "refinement_count",
				"tool_learning", "tool_executed", "tool_exec_success",
				"tool_exec_failed", "tool_not_found", "tool_execution_count",
				"tool_last_execution", "tool_execution", "tool_compilation_failed",
				"tool_generated", "tool_trace", "tool_generation_failed",
				"tool_issue_pattern", "tool_hot_loaded", "tool_version",
				"task_failure_reason", "task_failure_count",
				"issue_occurrence_count", "version_quality", "active_refinement",
				// Hypotheses are about files (prioritization.mg joins them
				// against test_coverage).
				"active_hypothesis", "unsafe_deref", "unchecked_error",
				"refinement_state",
				// Data-flow facts (data_flow.mg): assignments, uses, guards.
				"assigns", "uses", "guards_block", "guards_return",
				"error_checked_block", "error_checked_return", "same_scope",
				"suppressed_rule", "suppression_confidence", "bug_history",
				// Turn verdict inputs (coder_safety.mg turn_evidence /
				// hollow_success / turn_done) sit beside the diagnostics and
				// build state their negations read.
				"turn_evidence", "turn_created_source", "build_state",
			},
		},
		{
			Domain:          "tools",
			OwnedPredicates: []string{"tool_capabilities", "shard_lifecycle", "shell_exec_result"},
		},
		{
			Domain: "policy",
			// Keep the complete authorization envelope in one shard. Splitting
			// these predicates prevents policy rules from joining the exact
			// action, target, and payload submitted by the executive.
			OwnedPredicates: []string{
				"pending_action",
				"permitted_action",
				"permission_check_result",
				"permitted",
				"blocked",
				"constitution",
				"commit_barrier",
				"dangerous_action",
				// Routing consults the action type and the tool allowlist
				// against the pending action (system_routing.mg).
				"action_type",
				"tool_allowlist",
				"routing_result",
				// NOT moved here, deliberately: signed_approval, admin_override,
				// appeal_granted, temporary_override and candidate_action feed
				// the constitution's override rules (permitted :- ...
				// admin_override ...). Those rules have never fired on the
				// production kernel because the override facts land in the
				// catch-all while pending_action lives here. Homing them would
				// make a dormant permission path live; that is the
				// architect's call, not a routing fix.
			},
		},
		{
			Domain: "campaign",
			// Complete campaign fact family. Rules evaluate per shard, so every
			// predicate campaign rules join must live on the campaign shard or
			// per-shard routing splits the join (e.g. campaign_phase in the
			// campaign shard with phase_category in the catch-all makes
			// build_topology.mg derive missing_category for every phase).
			// Sources: Campaign/Phase/Task/ContextProfile ToFacts emits
			// (pinned by TestToFacts_GoldenFixture_ShouldExerciseEveryEmitBranch),
			// campaign_fact_sync retracts, and runtime facts asserted by the
			// campaign package (orchestrator, decomposer, checkpoint, pager).
			OwnedPredicates: []string{
				"campaign",
				"campaign_config",
				"campaign_dependency",
				"campaign_goal",
				"campaign_heartbeat",
				"campaign_metadata",
				"campaign_phase",
				"campaign_progress",
				"campaign_task",
				"checkpoint_verdict",
				"context_compression",
				"context_profile",
				"current_phase",
				"doc_layer",
				"doc_metadata",
				"doc_tag",
				"failed_campaign_task_count_computed",
				"goal_topic",
				"phase_category",
				"phase_checkpoint",
				"phase_context_atom",
				"phase_dependency",
				"phase_estimate",
				"phase_objective",
				"plan_revision",
				"replan_trigger",
				"requirement_coverage",
				"requires_resource",
				"source_document",
				"task_artifact",
				"task_attempt",
				"task_dependency",
				"task_error",
				"task_inference",
				"task_order",
				"task_priority",
				"task_result",
				"task_retry_at",
				"task_soft_dependency",
				"task_sub_campaign",
				"task_verification",
				"task_write_target",
				// Runtime facts campaign rules join against the family above:
				// shard profiles (delegate_task, specialist preference),
				// context pressure, milestones, remediation, document
				// references, phase tool lists and the decomposer's
				// intelligence_* facts.
				"shard_profile",
				"shard_can_handle",
				"shard_performing_well",
				"shard_campaign_reliable",
				"campaign_milestone",
				"campaign_progress_over_50",
				"context_pressure_high",
				"context_pressure_critical",
				"task_remediation_target",
				"quality_violation",
				"doc_reference",
				"tool_in_list",
				"intelligence_world_fact",
				"intelligence_churn_hotspot",
				"intelligence_learning_pattern",
				"intelligence_safety_warning",
				"intelligence_tool_gap",
				"intelligence_mcp_tool",
				"intelligence_shard_advice",
				"intelligence_test_coverage",
				"intelligence_code_pattern",
				"intelligence_previous_campaign",
				"intelligence_strategic_knowledge",
				"intelligence_file_action",
				"intelligence_high_impact",
				"intelligence_missing_tests",
				"intelligence_high_priority_file",
				"intelligence_file_depends",
				"intelligence_file_topology",
				"context_pressure_level",
				"knowledge_ingested",
				// Traces and verification attempts are scored against the
				// campaign task that produced them (trace_logic.mg,
				// campaign_autopoiesis.mg).
				"reasoning_trace", "trace_quality", "trace_task_type",
				"verification_attempt", "campaign_shard",
				"corrective_action_taken", "session_state", "shard_error",
				"trace_error", "trace_pattern",
			},
		},
		{
			Domain: "prompts",
			// The JIT selection facts join prompt_atom (jit_selection.mg,
			// jit_logic.mg), so they live with it.
			OwnedPredicates: []string{
				"prompt_atom", "atom_selection_score", "shard_prompt_base",
				"atom_tag", "atom_conflict", "atom_conflicts", "atom_exclusion_group",
				"atom_dependency", "atom_context_boost", "atom_final_order",
				"prompt_exemplar", "atom_requires", "is_mandatory", "vector_hit",
				"current_context", "atom_priority",
			},
		},
		{
			Domain:          "cortex",
			OwnedPredicates: nil, // Catch-all for unowned predicates
		},
	}
}

// SharedPredicates names the per-turn context facts replicated into every
// kernel shard. Each shard evaluates the policy program over its own facts
// only, so a rule joining user_intent (one fact per turn) with file_topology
// (world shard) or campaign_task (campaign shard) can fire only if the intent
// is present in that shard too. Everything here is tiny and turn-scoped;
// never add a large fact family (world model, campaign tasks) to this list —
// replication multiplies its cost by the shard count.
//
// Owned and shared are disjoint by construction: CortexKernel rejects a
// shard that owns a shared predicate, and the manifest test pins the rule.
func SharedPredicates() []string {
	return []string{
		// The turn
		"user_intent",
		"current_intent",
		"executive_processed_intent",
		"intent_signal",
		"delegation_candidate",
		"is_multi_step",
		"focus_needs_resolution",
		"active_goal",
		// The moment
		"current_time",
		"ooda_timeout",
		// The shards and their compile context
		"active_shard",
		"context_budget",
		"compile_context",
		"compile_shard",
		"system_shard_healthy",
		"generation_state",
		"validation_max_retries_reached",
		// Small runtime state joined by routing rules
		"test_state",
		"test_scope",
		"review_type",
		// Per-turn task and focus markers joined by both campaign and
		// prompt rules (current_task, shard_success) or by world rules
		// (focus_resolution); one or a handful of facts each.
		"current_task",
		"shard_success",
		"focus_resolution",
		"system_heartbeat",
		"current_user",
		// The instruction itself and the coder loop's counters
		"instruction_contains",
		"instruction_contains_write",
		"previous_coder_state",
		"state_unchanged_count",
		"tdd_retry_count",
		"max_retries",
		"corrective_query",
		"focus_clarification",
		"awaiting_user_input",
		"awaiting_clarification",
		// Small gate facts read under negation by rules whose positive side
		// is shared. A negation over a fact that lives in one shard is
		// vacuous in every other shard the rule fires in, so the negated
		// side must be visible everywhere the positive side is.
		"gauntlet_result",
		"mutation_approved",
		"action_verified",
		"shard_context_refreshed",
		"shadow_state",
		"simulated_effect",
		"current_shard_type",
		// Per-action validation outcomes (validation.mg): one fact per
		// action, negated by the unvalidated_side_effect verdict.
		"action_validation_failed",
		"action_escalated",
		"action_validated",
		"critical_action_resolved",
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

func (a *learningStoreAdapter) SaveBatch(shardType string, learnings []types.ShardLearning, sourceCampaign string) error {
	return a.store.SaveBatch(shardType, learnings, sourceCampaign)
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

	// Image generation (Gemini Nano Banana 2). Client is injected at spawn via
	// ShardManager.clientForShardType — never the worker/Ollama LLM.
	registerImageGenerator := func(id string, cfg types.ShardConfig) types.ShardAgent {
		return coreshards.NewImageGeneratorAgent(id, cfg)
	}
	for _, name := range []string{
		"image_generator", "image-generator", "imagegenerator",
		"imagen", "image", "nano_banana", "nanobanana",
	} {
		r.sm.RegisterShard(name, registerImageGenerator)
	}
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
	// NOTE: "tactile_router" and "campaign_runner" are registered in
	// internal/system/factory.go because they need boot-time collaborators
	// (task executor, browser manager, JIT config) that RegistryContext does not carry.
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

	// Image generator profile (Gemini Nano Banana 2 / gemini-3.1-flash-image)
	imgCfg := coreshards.DefaultImageGeneratorConfig("image_generator")
	sm.DefineProfile("image_generator", imgCfg)
	for _, alias := range []string{"image-generator", "imagegenerator", "imagen", "image", "nano_banana", "nanobanana"} {
		aliasCfg := imgCfg
		aliasCfg.Name = alias
		sm.DefineProfile(alias, aliasCfg)
	}

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
