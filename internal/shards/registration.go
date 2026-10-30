// Package shards implements specialized ShardAgent types.
// This file provides registration helpers for the shard manager.
package shards

import (
	"sort"
	"time"

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

// ShardPredicateManifest names the predicates a shard authoritatively owns
// and any bounded EDB copies it consumes. It is consumed by the cortex /
// shard-fact-router (Track D) so that when features.IsPerShardFactsEnabled()
// is true, a fact asserted on the wrong shard gets routed to its owner
// instead of silently landing in a non-authoritative store.
//
// The manifest is intentionally a flat data structure with no behavior.
// internal/system/factory.go consumes it when constructing the production
// Cortex and its explicit consumer-copy routes, making this the single source
// of predicate ownership and bounded replica truth.
type ShardPredicateManifest struct {
	Domain             string   // Shard domain name, matches core.KernelShardConfig.Domain
	OwnedPredicates    []string // Predicates this shard is authoritative for
	ConsumerPredicates []string // Bounded EDB copies this shard needs for local rules
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
				"code_implements", "code_calls", "code_defines", "dependency_link",
				"file_dir", "file_metadata_only", "treatment_scope", "modified", "active_file", "in_scope",
				"churn_rate",
				"test_coverage",
				"test_failed", "modified_interface", "impact_graph",

				// CodeDOM element facts and the per-language surface
				// (codedom_*.mg, coder_*.mg, reviewer.mg, tester.mg).
				"element_parent", "element_signature", "code_interactable",
				"generated_code", "cgo_code", "parse_error", "file_hash_mismatch",
				"file_modified_externally", "is_test_function", "test_file_for",
				"file_imports", "file_package", "file_in_scope",
				"imports_project_file", "import_cycle_member",

				"api_handler_function", "api_client_function", "api_dependency",
				"go_struct", "py_class", "py_decorator", "py_typed_function",
				"has_pydantic_base", "rs_struct", "rs_derive", "rs_serde_rename",
				"rs_unsafe_block", "ts_interface", "ts_interface_prop", "ts_class",
				"mg_negation_rule", "mg_recursive_rule",
				"cyclomatic_complexity", "git_history", "review_finding",
				"pytest_failure",
				// Edit plans and pending mutations are judged against the
				// dependency graph (coder_impact.mg, coder_safety.mg,
				// commit_gate.mg).
				"pending_edit", "pending_mutation",
				"modified_function", "modified_file",
				// Tool domain facts are joined only against file_topology
				// (tool_routing.mg, policy_mcp.mg).
				"mcp_tool_domain", "mcp_tool_registered", "mcp_server_status",
				"mcp_tool_capability", "mcp_tool_shard_affinity", "mcp_tool_avg_latency",
				"mcp_tool_usage", "mcp_tool_vector_score", "mcp_tool_category",
				// Control-plane classification belongs with the catalog it
				// classifies: policy_mcp.mg joins facet and risk against
				// mcp_tool_available, which is derived from mcp_tool_registered
				// above. Homing them anywhere else splits those joins across
				// shards and the rules simply never fire — mcp_tool_browsable
				// and mcp_server_facet_available would derive nothing, silently.
				// risk_source travels with risk for the same reason: they are
				// joined together in mcp_tool_gated.
				"mcp_tool_facet", "mcp_tool_risk", "mcp_tool_risk_source",
				// Coder-shard working state judged against the world model
				// (coder_*.mg): the file under edit, its diagnostics, tests.
				"coder_state", "diagnostic_count",

				"file_edited", "failing_test", "pytest_root_cause",
				"is_test_file", "project_forbidden_path",
				"retry_count",
				"traceback_frame", "assertion_mismatch",

				// Facts negated by coder/commit rules whose positive side is
				// world-model data; a negation evaluated in a shard that never
				// holds the fact always succeeds (blind negation).

				"suppression", "file_content", "entry_point",
				// The commit gate (commit_gate.mg, reviewer.mg): safe_to_commit
				// and review_passed negate has_block_commit, which is derived
				// from block_commit rules reading build_state, modified,
				// pending_mutation and review_finding (all here) and the build
				// result and coverage facts. A catch-all home for those last
				// three left safe_to_commit evaluating in a shard that never
				// saw a broken build or a modified file, so the negation always
				// succeeded and a commit was never blocked.
				"build_result",
				// The polyglot async facts codedom_safety.mg turns into
				// deny_edit beside the structure facts it reads here;
				// safe_to_edit negates has_deny_edit in this shard, and the
				// async deny rules fired only in the catch-all.
				"go_goroutine", "go_uses_context", "py_async_def", "rs_async_fn", "ts_async_function",
				"go_returns_error", "rs_returns_result", "rs_uses_unwrap", "ts_component", "ts_hook",
				"mg_aggregation_rule",
				// The reviewer's remaining facts (reviewer.mg). Its findings,
				// complexity and history facts are world-owned; the approval
				// history, suppression inputs and the review verdict were left
				// in the catch-all, so review_passed negated has_block_commit
				// where no block reason could derive, and active_finding
				// negated is_finding_suppressed without false_positive_pattern.
				// The turn's plan steps (turn_steps.mg): turn_steps_verdict
				// negates turn_has_unresolved_step, whose step_execution and
				// step_file_net inputs are here; step_plan_remaining was in
				// the catch-all, so a plan with steps left could still verdict
				// as done in the shard that never saw it.
				"step_plan_remaining",
				"approval_count", "approved_pattern", "review_accuracy", "user_rejected_finding",
				// The tool registry (tool_registry.go) is joined against
				// file_topology by tool_routing.mg and against the shared
				// per-turn facts by stage_context.mg.
				// tool_capability is NOT here: it is shared (SharedPredicates), because
				// has_capability negations fire in every shard.
				"tool_registered", "tool_hash", "tool_source",
				// What .nerd/agents.json declared (registerUserAgentConfigAtoms).
				// turn_tool_allowed joins it with persona_tool_allowed, which
				// every shard holds, so the rule fires on this owner. The
				// tool registry those names were checked against lives here.
				"user_agent_declared_tool",
				"tool_exists", "tool_usage_stats", "tool_description",
				"tool_available", "tool_binary_path",

				// tool_generation_blocked is derived (autopoiesis.mg) and no Go
				// asserts it, so it is not owned: an owner would make the
				// derivation look incomplete in every other shard.
				"tool_lifecycle", "tool_known_issue",

				"tool_learning", "tool_executed", "tool_exec_success",
				"tool_exec_failed", "tool_not_found", "tool_execution_count",
				"tool_last_execution", "tool_execution", "tool_compilation_failed",
				"tool_generated", "tool_trace", "tool_generation_failed",
				"tool_issue_pattern", "tool_hot_loaded", "tool_version",

				"active_refinement",
				// Hypotheses are about files (prioritization.mg joins them
				// against test_coverage).
				"active_hypothesis", "unsafe_deref", "unchecked_error",

				// Data-flow facts (data_flow.mg): assignments, uses, guards.
				"assigns", "uses", "guards_block", "guards_return",
				"error_checked_block", "error_checked_return", "function_scope",

				// Turn verdict inputs (coder_safety.mg turn_evidence /
				// hollow_success / turn_done) sit beside the diagnostics and
				// build state their negations read.
				"turn_evidence", "turn_acceptance", "turn_created_source", "build_state",
				"turn_created_test", "turn_written", "turn_changed_element", "turn_element_uncovered", "turn_element_measured", "turn_doc_write", "turn_verb",
				// This turn's test run, beside the elements a witness joins
				// it to. A catch-all home would split that join.
				"turn_test_case", "turn_test_failure_at", "turn_test_build_failure", "turn_test_output_repeat", "turn_failing_test",
				// The campaign check this turn was handed, and each run_check
				// receipt. turn_owes_gate(/check) and turn_catalog join them
				// with turn_verb here; a catch-all home would split the join
				// and the gate would never fire.
				"turn_declared_check", "turn_check_run",
				// The turn's own report admitting unfinished work
				// (session/admission_audit.go), negated by turn_verified,
				// and Jev's reading of it with the push-back's idle mark
				// (turn_finish.mg), which the /finish round joins.
				"turn_self_reported_incomplete", "turn_report_reading", "turn_finish_idle",
				// Each write's effect receipt and validation reading
				// (tool_effects.mg). turn_verified negates
				// turn_has_rejected_effect / turn_has_unmeasured_effect,
				// derived from these; homed elsewhere, that negation is
				// blind and a rejected write still verifies the turn.
				"turn_tool_effect", "turn_tool_validation", "turn_tool_effect_superseded",
				// Declared code families and the turn's no-change decisions
				// (code_families.mg) join turn_changed_element here, and
				// turn_verified negates the open reviews they derive.
				"code_family", "code_family_member", "code_family_requirement", "code_family_member_unresolved", "turn_family_no_change",
				// The edit sites a brief names, beside turn_verb:
				// turn_needs_step_plan (turn_steps.mg) joins the two.
				"turn_brief_site",
				// What each planned step's pass measured (turn_steps.mg).
				// step_next_action and the coverage join read them; a
				// catch-all home would split that join and the commit
				// retry would never fire. step_file_net is the same
				// measurement at verdict time: step_reverted joins it
				// with step_execution, and a split join would leave a
				// reverted step complete on the shard that ships.
				"step_execution", "step_no_change_evidence", "step_retried", "step_file_net",
				// A repair episode's attempts (repair_episode.mg), with the
				// turn facts they are cleaned up with.
				"repair_attempt",
				// A repair episode's restart (repair_episode.mg /restart),
				// joined against its attempts.
				"repair_restart",
				"turn_gate",
				// Measurements the /build gate is derived from, and the
				// closure it joins (coder_safety.mg). turn_written is already
				// on this shard; a catch-all home for the import edges would
				// split that join and a transitive importer the turn broke
				// would never be charged.
				"turn_build_measured", "turn_build_diagnostic", "turn_pkg_imports",
				"turn_written_package", "turn_build_graph_unknown",
				// A written path put back as it was (turn_net_written joins
				// it against turn_written, negated: it must live beside it).
				"turn_write_reverted", "turn_net_written", "has_turn_net_written",
				"turn_pkg_depends", "turn_build_depends_on_written",
				"turn_build_has_diagnostic", "turn_build_failure_attributed",
				"turn_build_failure_foreign", "turn_has_build_failure_foreign",
				// Measurements the /test, /vet, /check, /test_run and
				// /test_retention gates are derived from (coder_safety.mg).
				// They join turn_failing_test, turn_check_run and turn_gate,
				// which this shard owns; a catch-all home would split those
				// joins and the gate would never fire. The importer run's
				// rows join the same /test gate.
				"turn_test_failed_before", "turn_test_measured",
				"turn_importer_failed_before", "turn_importer_failing_test", "turn_importer_measured",
				"turn_write_seq", "turn_test_run",
				"turn_test_obligation", "turn_test_obligation_run",
				"turn_vet_ran", "turn_vet_finding", "turn_vet_before",
				"turn_removed_test_ran", "turn_removed_test",
				// The critic's on-change findings, triaged by the same
				// policy (turn_needs_uplift); the uplift prompt joins the
				// actionable ones back by index.
				"turn_critic_finding",
				// The post-edit rounds a turn ran, and its write-tool count
				// (turn_rounds.mg), beside the gates they schedule.
				"turn_round_ran", "turn_write_tools",
				// The pin gate's survivor count, which owes the /survivors
				// round.
				"turn_pin_survivors",
				// What the /unwired round measured and Jev's reading of the
				// model's reasons (turn_unwired.mg). turn_verified negates
				// has_red_gate, which they derive; homed elsewhere, that
				// negation is blind and unwired code still verifies.
				"turn_new_func", "turn_new_func_entry", "turn_new_func_generated",
				"turn_new_func_unobserved", "turn_func_use", "turn_method_mentioned",
				"turn_unused_reason_reading", "turn_unused_reason_unread", "turn_unwired_idle",
				"turn_untested",
				"turn_uncovered",
				// Shard experts (shard_experts.mg): which shard agent a task
				// needs joins the task's targets with file_extension,
				// file_dir, file_topology and dependency_link, all homed
				// here; a catch-all home would split the join and no task
				// would ever need an expert. task_complexity rides with the
				// task for strategic_advisor_required.
				"shard_agent", "shard_agent_domain", "technology_import_exact", "technology_import_prefix",
				"shard_agent_recorded", "expert_task", "expert_task_target", "expert_task_extension",
				"expert_task_speaker", "task_complexity",
				// The domains an agent's name states (shard_agent_domain,
				// /agent_name) join the agent with the words of its name and
				// the concept vocabulary, and a concept's verb joins
				// expert_task: one home, or the concept expert is never needed.
				"shard_agent_name_part", "domain_concept_term", "domain_concept_verb",
				// The turn's shard-agent critique (turn_rounds.mg), beside
				// turn_wrote, which the round's owing joins.
				"turn_expert_critic", "turn_expert_finding",
			},
		},
		{
			Domain: "tools",
			// tool_capabilities is not owned: it is the built-in tools' program
			// facts (delegation.mg), every shard holds them, and no Go asserts a
			// row. An owner made has_capability look incomplete in every other
			// shard.
			OwnedPredicates: []string{"shard_lifecycle", "shell_exec_result"},
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
				// Homed on purpose, and it is the same kind of decision the
				// note below defers: it makes a permission path live. The
				// difference is that this one was asked for. The executor
				// asserts file_recoverable(Target) only for a /delete_file
				// whose target git tracks with nothing staged and nothing
				// modified, and constitution.mg joins it against
				// pending_action, so it has to live in this shard or the rule
				// cannot fire -- which the shard-join audit caught before this
				// shipped, as a "split join [file_recoverable@cortex,
				// pending_action@policy]".
				"file_recoverable",
				// The same shape, for the same reason: the executor asserts
				// touches_secret_path(Target) beside the pending_action it
				// measures, and constitution.mg joins the two into
				// dangerous_content. Homed anywhere else, the secret-file
				// refusal never fires on the production kernel (the audit
				// caught it: "split join [pending_action@policy,
				// touches_secret_path@cortex]", 2026-09-23).
				"touches_secret_path",
				// The host asserts the exact human approval beside the
				// pending_action it admits, and constitution.mg joins the
				// two (permitted, human_approval_required). Homed anywhere
				// else the join splits and no approval could ever admit a
				// call -- or, worse, a stale one could outlive its tuple.
				"human_approval_granted",
				// The turn's restriction (constitution.mg section 7D): the
				// executor asserts these beside the pending_action they bind,
				// and restricted_tuple / consent_gated_tuple join them with
				// it, so they live where pending_action does or the
				// restriction never bars anything on the production kernel.
				"turn_restriction",
				"restriction_tool_effect",
				"pending_action_turn",
				"pending_action_effect",
				// The turn's authorization and each gated shell call
				// (constitution.mg section 7E): shell_call_authorized negates
				// turn_bars_effects and turn_owes_confirmation, derived here,
				// so its inputs live here too. Homed elsewhere the negations
				// are blind and a read-only turn could be authorized to commit.
				"turn_authorization",
				"shell_call",
				"shell_call_in_write_set",
				"consent_given",
				"blocked",
				"constitution",
				"commit_barrier",
				"dangerous_action",
				// Routing consults the action type and the tool allowlist
				// against the pending action (system_routing.mg).
				"action_type",
				"tool_allowlist",
				"routing_result",
				// NOT moved here, deliberately: appeal_granted,
				// temporary_override and candidate_action feed the
				// constitution's appeal rules. Those rules have never fired on
				// the production kernel because the appeal facts land in the
				// catch-all while pending_action lives here, and nothing in
				// production grants an appeal. The broad signed_approval /
				// admin_override arm was replaced by the request-bound
				// human_approval_granted above (2026-10-27).
			},
			// These are the base facts policy_mcp.mg joins into the remote
			// authority decision. Their existing owners remain canonical; only
			// these bounded EDB predicates get an additional policy copy.
			ConsumerPredicates: []string{
				"mcp_tool_registered", "mcp_tool_risk", "mcp_tool_risk_source",
				"mcp_server_status", "mcp_tool_name", "mcp_tool_schema_hash",
				"mcp_server_registered", "mcp_remote_request", "mcp_remote_reviewed",
				"mcp_remote_operation", "mcp_remote_resource", "mcp_resource_registered",
				"mcp_remote_prompt", "mcp_prompt_registered",
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
				// Cleanup completion is joined (and negated) beside campaign
				// in campaign_rules.mg; a catch-all owner splits both rules.
				"campaign_cleaned_up",
				// The acceptance witness (Acceptance.ToFacts): its rules join
				// current_campaign and has_incomplete_phase, which derive here.
				"campaign_acceptance",
				"campaign_acceptance_result",
				"campaign_dependency",
				"campaign_goal",
				"campaign_heartbeat",
				"campaign_metadata",
				"campaign_phase",
				"campaign_progress",
				"campaign_task",
				// The failed-snapshot observation eligible_task and
				// has_next_campaign_task (policy/campaign_tasks.mg) negate and
				// join beside campaign_task; homed elsewhere the hold is blind.
				"campaign_storage_uncertain",
				"checkpoint_verdict",
				"context_compression",
				"context_profile",
				"current_phase",
				"doc_layer",
				"doc_metadata",
				"doc_tag",
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
				// The typed signals of a failed attempt and the one-replan
				// marker: task_next_move (policy/campaign_decisions.mg) joins
				// them with task_attempt, so homed anywhere else the move never
				// sees a signal on the production kernel.
				"task_attempt_signal",
				"task_replanned_at_cap",
				// A phase's failed checkpoint runs, with the campaign rows
				// phase_ckpt_move (campaign_decisions.mg) is read beside.
				"phase_checkpoint_failure",
				"task_dependency",
				"task_context_from",
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
				"task_write_ext",
				"generated_output_vocab",
				// What the orchestrator measured for task_evidence and
				// verify_report_hollow (policy/campaign_evidence.mg), joined
				// with campaign_task, task_dependency and phase_dependency.
				"task_artifact_on_disk",
				"task_brief_names",
				"task_output_path",
				"task_brief_file",
				"artifact_cites",
				// And for task_preload (policy/campaign_preload.mg).
				"code_outline",
				"task_brief_element",
				// Runtime facts campaign rules join against the family above:
				// shard profiles,
				// context pressure, milestones, remediation, document
				// references, phase tool lists and the decomposer's
				// intelligence_* facts.
				"shard_profile",
				"shard_performing_well",

				"context_pressure_high",
				"context_pressure_critical",

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

				"intelligence_missing_tests",
				"intelligence_high_priority_file",

				"knowledge_ingested",
				// Traces are scored against the campaign task that produced
				// them (trace_logic.mg, campaign_autopoiesis.mg).
				"reasoning_trace",

				"session_state", "shard_error",
			},
		},
		{
			Domain: "prompts",
			// The JIT selection facts join prompt_atom (jit_selection.mg,
			// jit_logic.mg), so they live with it.
			OwnedPredicates: []string{
				"prompt_atom", "atom_selection_score", "shard_prompt_base",
				"atom_tag", "atom_conflicts",

				"atom_requires", "is_mandatory", "vector_hit",
				"current_context", "atom_importance", "vector_slot", "atom_relevance",
				"atom_relevance_threshold", "flesh_carried",
			},
		},
		{
			Domain:          "cortex",
			OwnedPredicates: nil, // Catch-all for unowned predicates
		},
	}
}

// PredicateConsumerRoutes returns the canonical bounded EDB replica routes
// declared by the shard manifests. The consumer gets a copy while the
// manifest's OwnedPredicates continue to name the authoritative source.
func PredicateConsumerRoutes() []core.PredicateConsumerRoute {
	var routes []core.PredicateConsumerRoute
	for _, manifest := range DefaultShardPredicateManifests() {
		for _, predicate := range manifest.ConsumerPredicates {
			routes = append(routes, core.PredicateConsumerRoute{
				Predicate: predicate,
				Consumer:  manifest.Domain,
			})
		}
	}
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Predicate != routes[j].Predicate {
			return routes[i].Predicate < routes[j].Predicate
		}
		return routes[i].Consumer < routes[j].Consumer
	})
	return routes
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
		// Configuration: config_param(Key, Value) rows from .nerd/config.json
		// (internal/config/params.go). Every domain's rules read their
		// thresholds from it -- the campaign's caps, the session's spans --
		// so it lives in every shard.
		"config_param",
		// The turn
		"user_intent",
		"current_intent",
		"executive_processed_intent",
		"intent_signal",
		"delegation_candidate",
		// multi_step_signal feeds is_multi_step and multi_step_plan_step
		// feeds multi_step_plan_ready, and both feed multi_step_lane(), which
		// the delegate and clarify lanes negate (routing_arbitration.mg): a
		// negation is only sound where the negated side can derive.
		"multi_step_signal",
		"multi_step_plan_step",
		"is_multi_step",
		"focus_needs_resolution",
		"active_goal",
		// The moment
		"current_time",
		// The shards and their compile context
		"active_shard",
		"context_budget",
		"compile_context",
		"compile_shard",
		"system_shard_healthy",

		// Small runtime state joined by routing rules
		"test_state",

		// Per-turn task and focus markers joined by both campaign and
		// prompt rules (current_task, shard_success) or by world rules
		// (focus_resolution); one or a handful of facts each.
		"current_task",
		"shard_success",
		"focus_resolution",
		"system_heartbeat",

		// The instruction itself and the coder loop's counters
		"instruction_contains",
		"instruction_contains_write",
		"previous_coder_state",
		"state_unchanged_count",
		"tdd_retry_count",
		"max_retries",
		"focus_clarification",
		"awaiting_user_input",
		"awaiting_clarification",
		// Small gate facts read under negation by rules whose positive side
		// is shared. A negation over a fact that lives in one shard is
		// vacuous in every other shard the rule fires in, so the negated
		// side must be visible everywhere the positive side is.

		"yolo_mode",
		// The work mode and the brief (brief.mg) gate every lane that starts
		// work (routing_arbitration.mg), and the lanes derive in every shard.
		"work_mode",
		"work_proposal_pending",
		// The operating state the main agent is told (routing_arbitration.mg)
		// reads debug_self_active under negation: "Debug-self is off" would
		// derive in every shard that never saw it.
		"debug_self_active",
		"brief_goal",
		"brief_constraint",
		"brief_decision",
		"brief_open_question",
		"brief_marked_ready",
		// Jev's turn judgments (jev_evidence.mg) feed the clarify, approve and
		// self-diagnosis lanes, which derive in every shard; background_job
		// feeds stop_background_requested beside them. jev_choice carries the
		// conversation act (conversation_acts.mg), read under negation there.
		"jev_judgment",
		"jev_judgment_about",
		"jev_choice",
		"background_job",
		"action_verified",
		"shard_context_refreshed",
		"shadow_state",
		"simulated_effect",
		"current_shard_type",
		// Per-action validation outcomes (validation.mg): one fact per
		// action, negated by the unvalidated_side_effect verdict.
		"action_validation_failed",

		"action_validated",
		"critical_action_resolved",
		// The tool registry's capabilities (autopoiesis_kernel.go: one row per
		// generated tool). has_capability reads it under !has_capability in
		// missing_tool_for, tool_generation_permitted and delegate_task, which
		// derive in every shard: homed in one, the others saw no generated
		// tool and derived "missing tool" for a capability already provided.
		"tool_capability",
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
	// RolePrompts compiles the system prompt of a role that is not a persona
	// shard turn (the image generator); nil fails that role's requests.
	RolePrompts types.RolePromptCompiler
	// World is the loaded world section. The world-model ingestor reads
	// MaxFilesPerScan from it at construction. The zero value resolves to
	// the default cap (100); a non-positive cap stays unbounded.
	World config.WorldConfig
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

// learningCandidateStore is where the executive and the perception firewall
// stage learning candidates: the workspace's local store, which /learn lists
// and confirms from. Nothing used to wire it, so every candidate those
// shards staged was dropped. nil without a local store.
func (r *shardFactoryRegistrar) learningCandidateStore() system.LearningCandidateStore {
	if r.ctx.VirtualStore == nil {
		return nil
	}
	db := r.ctx.VirtualStore.GetLocalDB()
	if db == nil {
		return nil
	}
	return db
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
	// ShardManager.clientForShardTypeLocked — never the worker/Ollama LLM.
	registerImageGenerator := func(id string, cfg types.ShardConfig) types.ShardAgent {
		return coreshards.NewImageGeneratorAgent(id, cfg, r.ctx.RolePrompts)
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
		if candidates := r.learningCandidateStore(); candidates != nil {
			shard.SetLearningCandidateStore(candidates)
		}
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
		shard := system.NewWorldModelIngestorShardWithConfig(system.WorldModelConfigFor(r.ctx.World))
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
		if candidates := r.learningCandidateStore(); candidates != nil {
			shard.SetLearningCandidateStore(candidates)
		}
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
		Timeout:     5 * time.Minute,
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
		Timeout:     24 * time.Hour,
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
		Timeout:     24 * time.Hour,
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
		Timeout:     24 * time.Hour,
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
		Timeout:     24 * time.Hour,
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
		Timeout:     24 * time.Hour,
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
		Timeout:     24 * time.Hour,
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
		Timeout:     24 * time.Hour,
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
		Timeout:     24 * time.Hour,
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
		Timeout:     24 * time.Hour,
		MemoryLimit: 4000,
		Model:       types.ModelConfig{}, // No LLM - constraint synthesis is logic-primary
	})
}
