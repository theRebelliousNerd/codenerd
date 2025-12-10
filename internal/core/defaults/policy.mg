# Cortex 1.5.0 Executive Policy (IDB)
# Version: 1.5.0
# Philosophy: Logic determines Reality; the Model merely describes it.

# =============================================================================
# SECTION 1: SPREADING ACTIVATION (Context Selection)
# =============================================================================
# Per §8.1: Energy flows from the user's intent through the graph of known facts

# 1. Base Activation (Recency) - High priority for new facts
activation(Fact, 100) :- new_fact(Fact).

# 2. Spreading Activation (Dependency)
# Energy flows from goals to required tools
activation(Tool, 80) :-
    active_goal(Goal),
    tool_capabilities(Tool, Cap),
    goal_requires(Goal, Cap).

# 3. Intent-driven activation
activation(Target, 90) :-
    user_intent(_, _, _, Target, _).

# 4. File modification spreads to dependents
activation(Dep, 70) :-
    modified(File),
    dependency_link(Dep, File, _).

# 5. Context Pruning - Only high-activation facts enter working memory
context_atom(Fact) :-
    activation(Fact, Score),
    Score > 30.

# =============================================================================
# SECTION 2: STRATEGY SELECTION (§3.1)
# =============================================================================
# Different coding tasks require different logical loops

# TDD Repair Loop for bug fixes
active_strategy(/tdd_repair_loop) :-
    user_intent(_, _, /fix, _, _),
    diagnostic(/error, _, _, _, _).

active_strategy(/tdd_repair_loop) :-
    user_intent(_, _, /debug, _, _).

# Exploration for queries
active_strategy(/breadth_first_survey) :-
    user_intent(_, /query, /explore, _, _).

active_strategy(/breadth_first_survey) :-
    user_intent(_, /query, /explain, _, _).

# Code generation for scaffolding
active_strategy(/project_init) :-
    user_intent(_, /mutation, /scaffold, _, _).

active_strategy(/project_init) :-
    user_intent(_, /mutation, /init, _, _).

# Refactor guard for modifications
active_strategy(/refactor_guard) :-
    user_intent(_, /mutation, /refactor, _, _).

# =============================================================================
# SECTION 3: TDD REPAIR LOOP (§3.2)
# =============================================================================
# State machine: Write -> Test -> Analyze -> Fix

# State Transitions
next_action(/read_error_log) :-
    test_state(/failing),
    retry_count(N), N < 3.

next_action(/analyze_root_cause) :-
    test_state(/log_read).

next_action(/generate_patch) :-
    test_state(/cause_found).

next_action(/run_tests) :-
    test_state(/patch_applied).

next_action(/run_tests) :-
    test_state(/unknown),
    user_intent(_, _, /test, _, _).

# Surrender Logic - Escalate after 3 retries
next_action(/escalate_to_user) :-
    test_state(/failing),
    retry_count(N), N >= 3.

# Success state
next_action(/complete) :-
    test_state(/passing).

# =============================================================================
# SECTION 4: FOCUS RESOLUTION & CLARIFICATION (§1.2)
# =============================================================================

# Clarification threshold - block execution if confidence < 85 (on 0-100 scale)
clarification_needed(Ref) :-
    focus_resolution(Ref, _, _, Score),
    Score < 85.

# Block action derivation when clarification is needed
next_action(/interrogative_mode) :-
    clarification_needed(_).

# Ambiguity detection
ambiguity_detected(Param) :-
    ambiguity_flag(Param, _, _).

next_action(/interrogative_mode) :-
    ambiguity_detected(_).

# =============================================================================
# SECTION 5: IMPACT ANALYSIS & REFACTORING GUARD (§3.3)
# =============================================================================

# Direct impact
impacted(X) :-
    dependency_link(X, Y, _),
    modified(Y).

# Transitive closure (recursive impact)
impacted(X) :-
    dependency_link(X, Z, _),
    impacted(Z).

# Unsafe to refactor if impacted code lacks test coverage
unsafe_to_refactor(Target) :-
    impacted(Target),
    !test_coverage(Target).

# Block refactoring when unsafe
block_refactor(Target, "uncovered_dependency") :-
    unsafe_to_refactor(Target).

# =============================================================================
# SECTION 6: COMMIT BARRIER (§2.2)
# =============================================================================

# Cannot commit if there are errors
block_commit("Build Broken") :-
    diagnostic(/error, _, _, _, _).

block_commit("Tests Failing") :-
    test_state(/failing).

# Fix Bug #10: The "Timeout = Permission" Trap
# Require explicit positive confirmation that checks actually ran
checks_passed() :-
    build_result(/true, _),
    test_state(/passing).

# Helper for safe negation
has_block_commit() :-
    block_commit(_).

# Safe to commit ONLY if checks passed AND no blocks exist
safe_to_commit() :-
    checks_passed(),
    !has_block_commit().

# =============================================================================
# SECTION 7: CONSTITUTIONAL LOGIC / SAFETY (§5.0)
# =============================================================================

# Default deny - permitted must be positively derived
permitted(Action) :-
    safe_action(Action).

permitted(Action) :-
    dangerous_action(Action),
    admin_override(User),
    signed_approval(Action).

# Fix Bug #12: The "Silent Join" (Shadow Rules)
# Explain WHY permission was denied to aid debugging/feedback
permission_denied(Action, "Dangerous Action") :-
    dangerous_action(Action),
    !admin_override(_).

permission_denied(Action, "Dangerous Action") :-
    dangerous_action(Action),
    !signed_approval(Action).

# Dangerous action patterns - marked by explicit facts
# dangerous_action is derived from danger_marker facts
# (String matching to be implemented via custom builtins)

# =============================================================================
# SAFE ACTIONS - Permitted by default for all shards
# =============================================================================
# These actions are constitutionally permitted without special approval.
# Dangerous actions (rm -rf, etc.) require admin_override + signed_approval.

# File operations (read-only and basic writes)
safe_action(/read_file).
safe_action(/fs_read).
safe_action(/write_file).
safe_action(/fs_write).
safe_action(/search_files).
safe_action(/glob_files).
safe_action(/analyze_code).

# Code analysis operations
safe_action(/parse_ast).
safe_action(/query_symbols).
safe_action(/check_syntax).
safe_action(/code_graph).

# Review operations
safe_action(/review).
safe_action(/lint).
safe_action(/check_security).

# Test operations (running tests is safe)
safe_action(/run_tests).
safe_action(/test_single).
safe_action(/coverage).

# Knowledge operations
safe_action(/vector_search).
safe_action(/knowledge_query).
safe_action(/embed_text).

# Browser operations (read-only)
safe_action(/browser_navigate).
safe_action(/browser_screenshot).
safe_action(/browser_read_dom).

# Network policy - allowlist approach
allowed_domain("github.com").
allowed_domain("pypi.org").
allowed_domain("crates.io").
allowed_domain("npmjs.com").
allowed_domain("pkg.go.dev").

# Note: network_permitted and security_violation require string matching
# which will be implemented via custom Go builtins at runtime

# =============================================================================
# SECTION 7B: STRATIFIED TRUST - AUTOPOIESIS SAFETY (Bug #15 Fix)
# =============================================================================
# This implements the "Stratified Trust" architecture to prevent jailbreak.
# Learned logic (from learned.gl) can ONLY propose candidate_action/1.
# The Constitution validates all candidates before they become final_action/1.
#
# SECURITY INVARIANT:
#   final_action(X) ⊆ candidate_action(X) ∩ permitted(X)
#
# This ensures learned rules can suggest, but never execute without approval.

# The Bridge Rule: Learned suggestions must pass constitutional checks
final_action(Action) :-
    candidate_action(Action),
    permitted(Action).

# Safety check predicate for runtime validation
safety_check(Action) :-
    permitted(Action).

# Deny actions that are candidates but not permitted
action_denied(Action, "Not constitutionally permitted") :-
    candidate_action(Action),
    !permitted(Action).

# Track learned rule proposals for auditing
learned_proposal(Action) :-
    candidate_action(Action).

# Metrics: Count how many learned rules are being blocked
# TODO: Re-enable when aggregation functions are fully implemented
blocked_learned_action_count(0) :-
    action_denied(_, _).

# =============================================================================
# SECTION 7C: APPEAL MECHANISM (Constitutional Appeals)
# =============================================================================
# Allows users to appeal blocked actions with justification.
# Appeals can be granted permanently or as temporary overrides.

# Actions that were blocked and have appeal available can be reconsidered
# if the appeal provides valid justification

# Suggest appeal for ambiguous blocks (not dangerous patterns)
suggest_appeal(ActionID) :-
    appeal_available(ActionID, ActionType, Target, Reason),
    !dangerous_action(ActionType).

# Track appeals that need user review
appeal_needs_review(ActionID, ActionType, Justification) :-
    appeal_pending(ActionID, ActionType, Justification, _).

# Helper: check if an action type has a temporary override configured
has_temporary_override(ActionType) :-
    temporary_override(ActionType, _).

# Override is currently active for an action type
# NOTE: temporary_override now stores ExpirationTimestamp directly (no arithmetic needed)
has_active_override(ActionType) :-
    appeal_granted(_, ActionType, _, _),
    temporary_override(ActionType, Expiration),
    current_time(Now),
    Now < Expiration.

# Permanent override (no expiration) - use helper to avoid unbound variable in negation
has_active_override(ActionType) :-
    appeal_granted(_, ActionType, _, _),
    !has_temporary_override(ActionType).

# Appeal granted should permit the action
permitted(ActionType) :-
    has_active_override(ActionType).

# Alert if too many appeals are being denied
excessive_appeal_denials() :-
    appeal_denied(_, _, _, _),
    appeal_denied(_, _, _, _),
    appeal_denied(_, _, _, _).

# Signal need for policy review if appeals frequently granted
appeal_pattern_detected(ActionType) :-
    appeal_granted(_, ActionType, _, _),
    appeal_granted(_, ActionType, _, _).

# =============================================================================
# SECTION 8: SHARD DELEGATION (§7.0)
# =============================================================================

# Delegate to researcher for init/explore
delegate_task(/researcher, "Initialize codebase analysis", /pending) :-
    user_intent(_, _, /init, _, _).

delegate_task(/researcher, Task, /pending) :-
    user_intent(_, _, /research, Task, _).

delegate_task(/researcher, Task, /pending) :-
    user_intent(_, /query, /explore, Task, _).

# Delegate to coder for coding tasks
delegate_task(/coder, Task, /pending) :-
    user_intent(_, /mutation, /implement, Task, _).

# Note: Negation with unbound variables is unsafe in Datalog
# Delegate refactoring task only when block_refactor facts don't exist
# This is handled at runtime by checking block_refactor before delegation
delegate_task(/coder, Task, /pending) :-
    user_intent(_, /mutation, /refactor, Task, _).

# Delegate to tester for test tasks
delegate_task(/tester, Task, /pending) :-
    user_intent(_, _, /test, Task, _).

delegate_task(/tester, "Generate tests for impacted code", /pending) :-
    impacted(File),
    !test_coverage(File).

# Delegate to reviewer for review tasks
delegate_task(/reviewer, Task, /pending) :-
    user_intent(_, _, /review, Task, _).

# =============================================================================
# SECTION 9: BROWSER PHYSICS (§9.0)
# =============================================================================

# Spatial reasoning - element to the left (constrained to interactable elements to avoid O(N²))
left_of(A, B) :-
    interactable(A, _),
    interactable(B, _),
    geometry(A, Ax, _, _, _),
    geometry(B, Bx, _, _, _),
    Ax < Bx.

# Element above another (constrained to interactable elements)
above(A, B) :-
    interactable(A, _),
    interactable(B, _),
    geometry(A, _, Ay, _, _),
    geometry(B, _, By, _, _),
    Ay < By.

# Honeypot detection via CSS properties
honeypot_detected(ID) :-
    computed_style(ID, "display", "none").

honeypot_detected(ID) :-
    computed_style(ID, "visibility", "hidden").

honeypot_detected(ID) :-
    computed_style(ID, "opacity", "0").

honeypot_detected(ID) :-
    geometry(ID, _, _, 0, _).

honeypot_detected(ID) :-
    geometry(ID, _, _, _, 0).

# Safe interactive elements (not honeypots)
safe_interactable(ID) :-
    interactable(ID, _),
    !honeypot_detected(ID).

# Target checkbox to the left of label text
target_checkbox(CheckID, LabelText) :-
    dom_node(CheckID, /input, _),
    attr(CheckID, "type", "checkbox"),
    visible_text(TextID, LabelText),
    left_of(CheckID, TextID).

# =============================================================================
# SECTION 10: TOOL CAPABILITY MAPPING & ACTION MAPPING
# =============================================================================

# Tool capabilities for spreading activation
tool_capabilities(/fs_read, /read).
tool_capabilities(/fs_write, /write).
tool_capabilities(/exec_cmd, /execute).
tool_capabilities(/browser, /navigate).
tool_capabilities(/browser, /click).
tool_capabilities(/browser, /type).
tool_capabilities(/code_graph, /analyze).
tool_capabilities(/code_graph, /dependencies).

# Goal capability requirements
goal_requires(Goal, /read) :-
    user_intent(_, /query, _, Goal, _).

goal_requires(Goal, /write) :-
    user_intent(_, /mutation, _, Goal, _).

goal_requires(Goal, /execute) :-
    user_intent(_, _, /run, Goal, _).

goal_requires(Goal, /analyze) :-
    user_intent(_, _, /explain, Goal, _).

# Action Mappings: Map intent verbs to executable actions
# Core actions
action_mapping(/explain, /analyze_code).
action_mapping(/read, /fs_read).
action_mapping(/search, /search_files).
action_mapping(/run, /exec_cmd).
action_mapping(/test, /run_tests).

# Code review & analysis actions (delegate to reviewer shard)
action_mapping(/review, /delegate_reviewer).
action_mapping(/security, /delegate_reviewer).
action_mapping(/analyze, /delegate_reviewer).

# Code mutation actions (delegate to coder shard)
action_mapping(/fix, /delegate_coder).
action_mapping(/refactor, /delegate_coder).
action_mapping(/create, /delegate_coder).
action_mapping(/delete, /delegate_coder).
action_mapping(/write, /fs_write).
action_mapping(/document, /delegate_coder).
action_mapping(/commit, /delegate_coder).

# Debug actions
action_mapping(/debug, /delegate_coder).

# Research actions (delegate to researcher shard)
action_mapping(/research, /delegate_researcher).
action_mapping(/explore, /delegate_researcher).

# Autopoiesis/Tool generation actions (delegate to tool_generator shard)
action_mapping(/generate_tool, /delegate_tool_generator).
action_mapping(/refine_tool, /delegate_tool_generator).
action_mapping(/list_tools, /delegate_tool_generator).
action_mapping(/tool_status, /delegate_tool_generator).

# Diff actions
action_mapping(/diff, /show_diff).

# Derive next_action from intent and mapping
next_action(Action) :-
    user_intent(_, _, Verb, _, _),
    action_mapping(Verb, Action).

# Specific file system actions
next_action(/fs_read) :-
    user_intent(_, _, /read, _, _).

next_action(/fs_write) :-
    user_intent(_, _, /write, _, _).

# Review delegation - high confidence triggers immediate delegation
delegate_task(/reviewer, Target, /pending) :-
    user_intent(_, _, /review, Target, _).

delegate_task(/reviewer, Target, /pending) :-
    user_intent(_, _, /security, Target, _).

delegate_task(/reviewer, Target, /pending) :-
    user_intent(_, _, /analyze, Target, _).

# Tool generator delegation - autopoiesis operations
delegate_task(/tool_generator, Target, /pending) :-
    user_intent(_, _, /generate_tool, Target, _).

delegate_task(/tool_generator, Target, /pending) :-
    user_intent(_, _, /refine_tool, Target, _).

delegate_task(/tool_generator, "", /pending) :-
    user_intent(_, _, /list_tools, _, _).

delegate_task(/tool_generator, Target, /pending) :-
    user_intent(_, _, /tool_status, Target, _).

# Auto-delegate when missing capability detected (implicit tool generation)
delegate_task(/tool_generator, Cap, /pending) :-
    missing_tool_for(_, Cap),
    !tool_generation_blocked(Cap).

# =============================================================================
# SECTION 11: ABDUCTIVE REASONING (§8.2)
# =============================================================================

# Abductive reasoning: missing hypotheses are symptoms without known causes
# This rule requires all variables to be bound in the negated atom
# Implementation: We use a helper predicate has_known_cause to track which symptoms have causes
# Then negate against that helper

# Mark symptoms that have known causes
has_known_cause(Symptom) :-
    known_cause(Symptom, _).

# Symptoms without causes need investigation
# Note: Using has_known_cause helper to ensure safe negation
missing_hypothesis(Symptom) :-
    symptom(_, Symptom),
    !has_known_cause(Symptom).

# Trigger clarification for missing hypotheses
next_action(/interrogative_mode) :-
    missing_hypothesis(_).

# =============================================================================
# SECTION 12: AUTOPOIESIS / LEARNING (§8.3)
# =============================================================================

# Detect repeated rejection pattern
preference_signal(Pattern) :-
    rejection_count(Pattern, N),
    N >= 3.

# Promote to long-term memory
promote_to_long_term(FactType, FactValue) :-
    preference_signal(Pattern),
    derived_rule(Pattern, FactType, FactValue).

# Autopoiesis: Missing Tool Detection
# Helper: derive when we HAVE a capability (for safe negation)
has_capability(Cap) :-
    tool_capabilities(_, Cap).

# Derive missing_tool_for if user intent requires a capability we don't have
missing_tool_for(IntentID, Cap) :-
    user_intent(IntentID, _, _, _, _),
    goal_requires(_, Cap),
    !has_capability(Cap).

# Trigger tool generation if tool is missing
next_action(/generate_tool) :-
    missing_tool_for(_, _).

# =============================================================================
# SECTION 12B: OUROBOROS LOOP - TOOL SELF-GENERATION
# =============================================================================
# The Ouroboros Loop: Detection → Specification → Safety → Compile → Register → Execute
# Named after the ancient symbol of a serpent eating its own tail.

# Tool exists in registry
tool_exists(ToolName) :-
    tool_registered(ToolName, _).

# Tool is ready for execution (compiled and registered)
tool_ready(ToolName) :-
    tool_exists(ToolName),
    tool_hash(ToolName, _).

# Tool is available (registered and ready)
tool_available(ToolName) :-
    registered_tool(ToolName, _, _).

# Capability is available if any tool provides it
capability_available(Cap) :-
    tool_capability(_, Cap).

# Need new tool when capability missing and user explicitly requests it
explicit_tool_request(Cap) :-
    user_intent(_, /mutation, /generate_tool, Cap, _).

# Need new tool when repeated failures suggest capability gap
capability_gap_detected(Cap) :-
    task_failure_reason(_, "missing_capability", Cap),
    task_failure_count(Cap, N),
    N >= 2.

# Tool generation is permitted (safety gate)
tool_generation_permitted(Cap) :-
    missing_tool_for(_, Cap),
    !tool_generation_blocked(Cap).

# Block tool generation for dangerous capabilities
tool_generation_blocked(Cap) :-
    dangerous_capability(Cap).

# Define dangerous capabilities that should never be auto-generated
dangerous_capability(/exec_arbitrary).
dangerous_capability(/network_unconstrained).
dangerous_capability(/system_admin).
dangerous_capability(/credential_access).

# Ouroboros next actions
next_action(/ouroboros_detect) :-
    capability_gap_detected(_).

next_action(/ouroboros_generate) :-
    tool_generation_permitted(_),
    !has_active_generation().

next_action(/ouroboros_compile) :-
    tool_source_ready(ToolName),
    tool_safety_verified(ToolName),
    !tool_compiled(ToolName).

next_action(/ouroboros_register) :-
    tool_compiled(ToolName),
    !is_tool_registered(ToolName).

# Track active tool generation (prevent parallel generations)
active_generation(ToolName) :-
    generation_state(ToolName, /in_progress).

# Helper for safe negation - true if any generation is in progress
has_active_generation() :-
    active_generation(_).

# Helper for safe negation - true if tool is registered
is_tool_registered(ToolName) :-
    tool_registered(ToolName, _).

# Tool lifecycle states
tool_lifecycle(ToolName, /detected) :-
    missing_tool_for(_, ToolName).

tool_lifecycle(ToolName, /generating) :-
    generation_state(ToolName, /in_progress).

tool_lifecycle(ToolName, /safety_check) :-
    tool_source_ready(ToolName),
    !tool_safety_verified(ToolName).

tool_lifecycle(ToolName, /compiling) :-
    tool_safety_verified(ToolName),
    !tool_compiled(ToolName).

tool_lifecycle(ToolName, /ready) :-
    tool_ready(ToolName).

# =============================================================================
# SECTION 12C: TOOL LEARNING AND OPTIMIZATION
# =============================================================================
# Learning from tool executions to improve future generations.

# Tool quality tracking (quality on 0-100 scale)
tool_quality_poor(ToolName) :-
    tool_learning(ToolName, Executions, _, AvgQuality),
    Executions >= 3,
    AvgQuality < 50.

tool_quality_acceptable(ToolName) :-
    tool_learning(ToolName, Executions, _, AvgQuality),
    Executions >= 3,
    AvgQuality >= 50,
    AvgQuality < 80.

tool_quality_good(ToolName) :-
    tool_learning(ToolName, Executions, _, AvgQuality),
    Executions >= 3,
    AvgQuality >= 80.

# Trigger refinement for poor quality tools
tool_needs_refinement(ToolName) :-
    tool_quality_poor(ToolName).

tool_needs_refinement(ToolName) :-
    tool_known_issue(ToolName, /pagination),
    tool_learning(ToolName, Executions, _, _),
    Executions >= 2.

tool_needs_refinement(ToolName) :-
    tool_known_issue(ToolName, /incomplete),
    tool_learning(ToolName, Executions, _, _),
    Executions >= 2.

# Next action for refinement
next_action(/refine_tool) :-
    tool_needs_refinement(_),
    !has_active_refinement().

# Prevent parallel refinements
active_refinement(ToolName) :-
    refinement_state(ToolName, /in_progress).

# Helper for safe negation - true if any refinement is in progress
has_active_refinement() :-
    active_refinement(_).

# Learning pattern signals
learning_pattern_detected(ToolName, IssueType) :-
    tool_known_issue(ToolName, IssueType),
    issue_occurrence_count(ToolName, IssueType, Count),
    Count >= 3.

# Promote learnings to tool generation hints
tool_generation_hint(Capability, "add_pagination") :-
    learning_pattern_detected(_, /pagination),
    capability_similar_to(Capability, _).

tool_generation_hint(Capability, "increase_limits") :-
    learning_pattern_detected(_, /incomplete),
    capability_similar_to(Capability, _).

tool_generation_hint(Capability, "add_retry") :-
    learning_pattern_detected(_, /rate_limit),
    capability_similar_to(Capability, _).

# Track refinement success
refinement_effective(ToolName) :-
    tool_refined(ToolName, OldVersion, NewVersion),
    version_quality(ToolName, OldVersion, OldQuality),
    version_quality(ToolName, NewVersion, NewQuality),
    NewQuality > OldQuality.

# Escalate if refinement didn't help
escalate_to_user(ToolName, "refinement_ineffective") :-
    tool_refined(ToolName, _, _),
    tool_quality_poor(ToolName),
    refinement_count(ToolName, Count),
    Count >= 2.

# =============================================================================
# SECTION 13: GIT-AWARE SAFETY / CHESTERTON'S FENCE (§21)
# =============================================================================

# Recent change by another author (within 2 days)
recent_change_by_other(File) :-
    git_history(File, _, Author, Age, _),
    current_user(CurrentUser),
    Author != CurrentUser,
    Age < 2.

# Chesterton's Fence warning - warn before deleting recently-changed code
chesterton_fence_warning(File, "recent_change_by_other") :-
    user_intent(_, /mutation, /delete, File, _),
    recent_change_by_other(File).

chesterton_fence_warning(File, "high_churn_file") :-
    user_intent(_, /mutation, /refactor, File, _),
    churn_rate(File, Freq),
    Freq > 5.

# Trigger clarification for Chesterton's Fence
clarification_needed(File) :-
    chesterton_fence_warning(File, _).

# =============================================================================
# SECTION 14: SHADOW MODE / COUNTERFACTUAL REASONING (§22)
# =============================================================================

# Helper for safe negation
has_projection_violation(ActionID) :-
    projection_violation(ActionID, _).

# Safe projection - action passes safety checks in shadow simulation
safe_projection(ActionID) :-
    shadow_state(_, ActionID, /valid),
    !has_projection_violation(ActionID).

# Projection violation detection
projection_violation(ActionID, "test_failure") :-
    simulated_effect(ActionID, "diagnostic", _),
    simulated_effect(ActionID, "diagnostic_severity", /error).

projection_violation(ActionID, "security_violation") :-
    simulated_effect(ActionID, "security_violation", _).

# Block action if projection fails
block_commit("shadow_simulation_failed") :-
    pending_mutation(MutationID, _, _, _),
    !safe_projection(MutationID).

# =============================================================================
# SECTION 15: INTERACTIVE DIFF APPROVAL (§23)
# =============================================================================

# Require approval for dangerous mutations
requires_approval(MutationID) :-
    pending_mutation(MutationID, File, _, _),
    chesterton_fence_warning(File, _).

requires_approval(MutationID) :-
    pending_mutation(MutationID, File, _, _),
    impacted(File).

# Helper for safe negation
is_mutation_approved(MutationID) :-
    mutation_approved(MutationID, _, _).

# Block mutation without approval
next_action(/ask_user) :-
    pending_mutation(MutationID, _, _, _),
    requires_approval(MutationID),
    !is_mutation_approved(MutationID).

# =============================================================================
# SECTION 16: SESSION STATE / CLARIFICATION LOOP (§20)
# =============================================================================

# Resume from clarification
next_action(/resume_task) :-
    session_state(_, /suspended, _),
    focus_clarification(_).

# Clear clarification when answered
# (Handled at runtime - logic marks session as active)

# =============================================================================
# SECTION 17: KNOWLEDGE ATOM INTEGRATION (§24)
# =============================================================================

# When high-confidence knowledge about the domain exists
# Knowledge atoms inform strategy selection (confidence on 0-100 scale)
active_strategy(/domain_expert) :-
    knowledge_atom(_, _, _, Confidence),
    Confidence > 80,
    user_intent(_, _, _, _, _).

# =============================================================================
# SECTION 17B: LEARNED KNOWLEDGE APPLICATION
# =============================================================================
# These rules leverage facts hydrated from knowledge.db by HydrateLearnings().
# The hydration happens during OODA Observe phase.

# 1. User preferences influence tool selection
# If user prefers a language, boost activation for related tools
activation(Tool, 85) :-
    learned_preference(/prefer_language, _),
    tool_capabilities(Tool, /code_generation),
    tool_language(Tool, _).

# 2. Learned constraints become safety checks
# Constraints from knowledge.db feed into constitutional logic
constraint_violation(Action, Reason) :-
    learned_constraint(Predicate, Args),
    action_violates(Action, Predicate, Args),
    Reason = Args.

# 3. User facts inform context
# Facts about the user/project activate relevant context
context_atom(fn:pair(Pred, Args)) :-
    learned_fact(Pred, Args),
    relevant_to_intent(Pred, Intent),
    user_intent(_, _, _, _, Intent).

# 4. Knowledge graph links spread activation
# Entity relationships from knowledge_graph propagate energy
activation(EntityB, 60) :-
    knowledge_link(EntityA, /related_to, EntityB),
    activation(EntityA, Score),
    Score > 50.

activation(EntityB, 70) :-
    knowledge_link(EntityA, /depends_on, EntityB),
    activation(EntityA, Score),
    Score > 40.

# 5. High-activation facts boost related content
# Recent activations from activation_log inform focus
context_priority(FactID, /high) :-
    activation(FactID, Score),
    Score > 70.

# 6. Session continuity - recent turns inform context
# Session history provides conversational context
context_atom(UserInput) :-
    session_turn(_, TurnNum, UserInput, _),
    TurnNum > 0.

# 7. Similar content retrieval for semantic search
# Vector recall results inform related context
related_context(Content) :-
    similar_content(Rank, Content),
    Rank < 5.

# =============================================================================
# SECTION 18: SHARD TYPE CLASSIFICATION (§6.1 Taxonomy)
# =============================================================================

# Type 1: System Level - Always on, high reliability
shard_type(/system, /permanent, /high_reliability).

# Type 2: Ephemeral - Fast spawning, RAM only
shard_type(/ephemeral, /spawn_die, /speed_optimized).

# Type 3: Persistent LLM-Created - Background tasks, SQLite
shard_type(/persistent, /long_running, /adaptive).

# Type 4: User Configured - Deep domain knowledge
shard_type(/user, /explicit, /user_defined).

# Model capability mapping for shards
shard_model_config(/system, /high_reasoning).
shard_model_config(/ephemeral, /high_speed).
shard_model_config(/persistent, /balanced).
shard_model_config(/user, /high_reasoning).

# =============================================================================
# SECTION 19: CAMPAIGN ORCHESTRATION POLICY
# =============================================================================
# Long-running, multi-phase goal execution with context management

# -----------------------------------------------------------------------------
# 19.1 Campaign State Machine
# -----------------------------------------------------------------------------

# Current campaign is the one that's active
current_campaign(CampaignID) :-
    campaign(CampaignID, _, _, _, /active).

# Campaign execution strategy activates when a campaign is active
active_strategy(/campaign_execution) :-
    current_campaign(_).

# -----------------------------------------------------------------------------
# 19.2 Phase Eligibility & Sequencing
# -----------------------------------------------------------------------------

# Helper: check if a phase has incomplete hard dependencies
has_incomplete_hard_dep(PhaseID) :-
    phase_dependency(PhaseID, DepPhaseID, /hard),
    campaign_phase(DepPhaseID, _, _, _, Status, _),
    /completed != Status.

# A phase is eligible when all hard dependencies are complete
phase_eligible(PhaseID) :-
    campaign_phase(PhaseID, CampaignID, _, _, /pending, _),
    current_campaign(CampaignID),
    !has_incomplete_hard_dep(PhaseID).

# Helper: check if there's an earlier eligible phase
# Note: Order is bound by looking up PhaseID's order within the rule
has_earlier_phase(PhaseID) :-
    campaign_phase(PhaseID, _, _, Order, _, _),
    phase_eligible(OtherPhaseID),
    OtherPhaseID != PhaseID,
    campaign_phase(OtherPhaseID, _, _, OtherOrder, _, _),
    OtherOrder < Order.

# Current phase: lowest order eligible phase, or the one in progress
current_phase(PhaseID) :-
    campaign_phase(PhaseID, CampaignID, _, _, /in_progress, _),
    current_campaign(CampaignID).

current_phase(PhaseID) :-
    phase_eligible(PhaseID),
    !has_earlier_phase(PhaseID),
    !has_in_progress_phase().

# Helper: check if any phase is in progress
has_in_progress_phase() :-
    campaign_phase(_, CampaignID, _, _, /in_progress, _),
    current_campaign(CampaignID).

# Phase is blocked if it has incomplete hard dependencies
phase_blocked(PhaseID, "hard_dependency_incomplete") :-
    campaign_phase(PhaseID, CampaignID, _, _, /pending, _),
    current_campaign(CampaignID),
    has_incomplete_hard_dep(PhaseID).

# -----------------------------------------------------------------------------
# 19.3 Task Selection & Execution
# -----------------------------------------------------------------------------

# Helper: check if task has blocking dependencies
has_blocking_task_dep(TaskID) :-
    task_dependency(TaskID, BlockerID),
    campaign_task(BlockerID, _, _, Status, _),
    /completed != Status,
    /skipped != Status.

# Helper: check if task conflicts with an in-progress task
task_conflict_active(TaskID) :-
    task_conflict(TaskID, OtherTaskID),
    campaign_task(OtherTaskID, _, _, /in_progress, _).

task_conflict_active(TaskID) :-
    task_conflict(OtherTaskID, TaskID),
    campaign_task(OtherTaskID, _, _, /in_progress, _).

# Optional conflict heuristic: same artifact path -> conflict
task_conflict(TaskID, OtherTaskID) :-
    TaskID != OtherTaskID,
    task_artifact(TaskID, _, Path, _),
    task_artifact(OtherTaskID, _, Path, _).

# Helper: check if there's an earlier pending task
has_earlier_task(TaskID, PhaseID) :-
    campaign_task(OtherTaskID, PhaseID, _, /pending, _),
    OtherTaskID != TaskID,
    task_priority(OtherTaskID, OtherPriority),
    task_priority(TaskID, Priority),
    priority_higher(OtherPriority, Priority).

# Priority ordering helper (will be implemented as Go builtin)
# For now, we use simple rules
priority_higher(/critical, /high).
priority_higher(/critical, /normal).
priority_higher(/critical, /low).
priority_higher(/high, /normal).
priority_higher(/high, /low).
priority_higher(/normal, /low).

# Eligible tasks: highest-priority pending tasks in the current phase without blockers or conflicts
eligible_task(TaskID) :-
    current_phase(PhaseID),
    campaign_task(TaskID, PhaseID, _, /pending, _),
    !has_blocking_task_dep(TaskID),
    !has_earlier_task(TaskID, PhaseID),
    !task_conflict_active(TaskID).

# Next task remains available for single-dispatch clients
next_campaign_task(TaskID) :-
    eligible_task(TaskID).

# Derive next_action based on campaign task type
next_action(/campaign_create_file) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, _, _, /file_create).

next_action(/campaign_modify_file) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, _, _, /file_modify).

next_action(/campaign_write_test) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, _, _, /test_write).

next_action(/campaign_run_test) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, _, _, /test_run).

next_action(/campaign_research) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, _, _, /research).

next_action(/campaign_verify) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, _, _, /verify).

next_action(/campaign_document) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, _, _, /document).

next_action(/campaign_refactor) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, _, _, /refactor).

next_action(/campaign_integrate) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, _, _, /integrate).

# Auto-spawn researcher shard for research tasks
delegate_task(/researcher, Description, /pending) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, Description, _, /research).

# Auto-spawn coder shard for file creation/modification
delegate_task(/coder, Description, /pending) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, Description, _, /file_create).

delegate_task(/coder, Description, /pending) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, Description, _, /file_modify).

# Auto-spawn tester shard for test tasks
delegate_task(/tester, Description, /pending) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, Description, _, /test_write).

delegate_task(/tester, Description, /pending) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, Description, _, /test_run).

# -----------------------------------------------------------------------------
# 19.4 Context Paging (Phase-Aware Spreading Activation)
# -----------------------------------------------------------------------------

# Boost activation for current phase context
activation(Fact, 150) :-
    current_phase(PhaseID),
    phase_context_atom(PhaseID, Fact, _).

# Boost files matching current task's target
activation(Target, 140) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, _, _, _),
    task_artifact(TaskID, _, Target, _).

# Suppress context from completed phases
activation(Fact, -50) :-
    context_compression(PhaseID, _, _, _),
    phase_context_atom(PhaseID, Fact, _).

# -----------------------------------------------------------------------------
# 19.5 Checkpoint & Verification
# -----------------------------------------------------------------------------

# Helper: check if phase has pending checkpoint
has_pending_checkpoint(PhaseID) :-
    phase_objective(PhaseID, _, _, VerifyMethod),
    /none != VerifyMethod,
    !has_passed_checkpoint(PhaseID, VerifyMethod).

has_passed_checkpoint(PhaseID, CheckType) :-
    phase_checkpoint(PhaseID, CheckType, /true, _, _).

# Helper: check if all phase tasks are complete
has_incomplete_phase_task(PhaseID) :-
    campaign_task(_, PhaseID, _, Status, _),
    /completed != Status,
    /skipped != Status.

all_phase_tasks_complete(PhaseID) :-
    campaign_phase(PhaseID, _, _, _, _, _),
    !has_incomplete_phase_task(PhaseID).

# Trigger checkpoint when all tasks complete but checkpoint pending
next_action(/run_phase_checkpoint) :-
    current_phase(PhaseID),
    all_phase_tasks_complete(PhaseID),
    has_pending_checkpoint(PhaseID).

# Block phase completion if checkpoint failed
phase_blocked(PhaseID, "checkpoint_failed") :-
    phase_checkpoint(PhaseID, _, /false, _, _).

# -----------------------------------------------------------------------------
# 19.6 Replanning Triggers
# -----------------------------------------------------------------------------

# Helper: identify failed tasks (for counting in Go runtime)
failed_campaign_task(CampaignID, TaskID) :-
    current_campaign(CampaignID),
    campaign_task(TaskID, PhaseID, Desc, /failed, TaskType),
    campaign_phase(PhaseID, CampaignID, PhaseName, Seq, Status, Profile).

# Trigger replan on repeated failures (threshold checked in Go runtime)
# The Go runtime counts failed_campaign_task facts and triggers replan if > 3
replan_needed(CampaignID, "task_failure_cascade") :-
    current_campaign(CampaignID),
    failed_campaign_task(CampaignID, TaskID1),
    failed_campaign_task(CampaignID, TaskID2),
    failed_campaign_task(CampaignID, TaskID3),
    TaskID1 != TaskID2,
    TaskID2 != TaskID3,
    TaskID1 != TaskID3.

# Trigger replan if user provides new instruction during campaign
replan_needed(CampaignID, "user_instruction") :-
    current_campaign(CampaignID),
    user_intent(_, /instruction, _, _, _).

# Trigger replan if explicit trigger exists
replan_needed(CampaignID, Reason) :-
    replan_trigger(CampaignID, Reason, _).

# Pause and replan action
next_action(/pause_and_replan) :-
    replan_needed(_, _).

# -----------------------------------------------------------------------------
# 19.7 Campaign Helpers for Safe Negation
# -----------------------------------------------------------------------------

# Helper: true if any phase is eligible to start
has_eligible_phase() :-
    phase_eligible(_).

# Helper: true if any phase is in progress
has_in_progress_phase() :-
    campaign_phase(_, _, _, _, /in_progress, _).

# Helper: true if there's a next campaign task available
has_next_campaign_task() :-
    next_campaign_task(_).

# Helper: check if any phase is not complete
has_incomplete_phase(CampaignID) :-
    campaign_phase(_, CampaignID, _, _, Status, _),
    /completed != Status,
    /skipped != Status.

# Campaign complete when all phases complete
campaign_complete(CampaignID) :-
    current_campaign(CampaignID),
    !has_incomplete_phase(CampaignID).

next_action(/campaign_complete) :-
    campaign_complete(_).

# -----------------------------------------------------------------------------
# 19.8 Campaign Blocking Conditions
# -----------------------------------------------------------------------------

# Campaign blocked if no eligible phases and none in progress
campaign_blocked(CampaignID, "no_eligible_phases") :-
    current_campaign(CampaignID),
    !has_eligible_phase(),
    !has_in_progress_phase(),
    has_incomplete_phase(CampaignID).

# Campaign blocked if all remaining tasks are blocked
campaign_blocked(CampaignID, "all_tasks_blocked") :-
    current_campaign(CampaignID),
    current_phase(PhaseID),
    !has_next_campaign_task(),
    has_incomplete_phase_task(PhaseID).

# -----------------------------------------------------------------------------
# 19.9 Autopoiesis During Campaign
# -----------------------------------------------------------------------------

# Track successful phase types for learning (Go runtime extracts from kernel)
phase_success_pattern(PhaseType) :-
    campaign_phase(PhaseID, CampaignID, PhaseName, Seq, /completed, Profile),
    phase_objective(PhaseID, PhaseType, Desc, Priority),
    phase_checkpoint(PhaseID, CheckpointID, /true, ValidatedAt, ValidatorShard).

# Learn from phase completion - promotes success pattern for phase type
promote_to_long_term(/phase_success, PhaseType) :-
    phase_success_pattern(PhaseType).

# Learn from task failures for future avoidance
campaign_learning(CampaignID, /failure_pattern, TaskType, ErrorMsg, Now) :-
    current_campaign(CampaignID),
    campaign_task(TaskID, _, _, /failed, TaskType),
    task_error(TaskID, _, ErrorMsg),
    current_time(Now).

# -----------------------------------------------------------------------------
# 19.10 Campaign-Aware Tool Permissions
# -----------------------------------------------------------------------------

# During campaigns, only permit tools in the phase's context profile
phase_tool_permitted(Tool) :-
    current_phase(PhaseID),
    campaign_phase(PhaseID, _, _, _, _, ContextProfile),
    context_profile(ContextProfile, _, RequiredTools, _),
    tool_in_list(Tool, RequiredTools).

# Block tools not in phase profile during active campaign
# (This is advisory - Go runtime can override for safety)
# Note: Tool is bound via tool_capabilities before negation check
tool_advisory_block(Tool, "not_in_phase_profile") :-
    current_campaign(_),
    current_phase(_),
    tool_capabilities(Tool, _),
    !phase_tool_permitted(Tool).

# =============================================================================
# SECTION 20: CAMPAIGN START TRIGGER
# =============================================================================

# Trigger campaign mode when user wants to start a campaign
active_strategy(/campaign_planning) :-
    user_intent(_, /mutation, /campaign, _, _).

# Alternative triggers for campaign-like requests
active_strategy(/campaign_planning) :-
    user_intent(_, /mutation, /build, Target, _),
    target_is_large(Target).

active_strategy(/campaign_planning) :-
    user_intent(_, /mutation, /implement, Target, _),
    target_is_complex(Target).

# Heuristics for complexity (implemented in Go builtins)
# target_is_large(Target) - true if target references multiple files/features
# target_is_complex(Target) - true if target requires multiple phases

# =============================================================================
# SECTION 21: SYSTEM SHARD COORDINATION
# =============================================================================
# Coordinates the 6 system shards: perception_firewall, executive_policy,
# constitution_gate, world_model_ingestor, tactile_router, session_planner.
# These are Type 1 (permanent/continuous) shards that form the OODA loop.

# -----------------------------------------------------------------------------
# 21.1 Intent Processing Flow (Perception → Executive)
# -----------------------------------------------------------------------------

# A user_intent is pending if not yet processed by executive
pending_intent(IntentID) :-
    user_intent(IntentID, _, _, _, _),
    !intent_processed(IntentID).

# Helper for safe negation
intent_processed(IntentID) :-
    processed_intent(IntentID).

# Focus needs resolution if confidence is low (score on 0-100 scale)
focus_needs_resolution(Ref) :-
    focus_resolution(Ref, _, _, Score),
    Score < 70.

# Intent ready for executive processing
intent_ready_for_executive(IntentID) :-
    user_intent(IntentID, _, _, Target, _),
    !focus_needs_resolution(Target).

# -----------------------------------------------------------------------------
# 21.2 Action Flow (Executive → Constitution → Router)
# -----------------------------------------------------------------------------

# Action is pending permission check from constitution gate
action_pending_permission(ActionID) :-
    pending_permission_check(ActionID),
    !permission_checked(ActionID).

# Helper for safe negation
permission_checked(ActionID) :-
    permission_check_result(ActionID, _, _).

# Action is permitted by constitution gate
action_permitted(ActionID) :-
    permission_check_result(ActionID, /permit, _).

# Action is blocked by constitution gate
action_blocked(ActionID, Reason) :-
    permission_check_result(ActionID, /deny, Reason).

# Action ready for routing (permitted and not yet routed)
action_ready_for_routing(ActionID) :-
    action_permitted(ActionID),
    !action_routed(ActionID).

# Helper for safe negation
action_routed(ActionID) :-
    ready_for_routing(ActionID),
    routing_result(ActionID, _, _).

# Derive routing result success
routing_succeeded(ActionID) :-
    routing_result(ActionID, /success, _).

# Derive routing result failure
routing_failed(ActionID, Error) :-
    routing_result(ActionID, /failure, Error).

# -----------------------------------------------------------------------------
# 21.3 System Shard Health Monitoring
# -----------------------------------------------------------------------------

# System shard is healthy if heartbeat within threshold (30 seconds)
# Note: Using fn:minus directly since time_diff would need bound variables.
# Assumes Now >= Timestamp (current time always after heartbeat time).
system_shard_healthy(ShardName) :-
    system_heartbeat(ShardName, Timestamp),
    current_time(Now),
    Now >= Timestamp,
    Diff = fn:minus(Now, Timestamp),
    Diff < 30.

# Helper: check if shard has no recent heartbeat
shard_heartbeat_stale(ShardName) :-
    system_shard(ShardName, _),
    !system_shard_healthy(ShardName).

# Escalate if critical system shard is unhealthy
escalation_needed(/system_health, ShardName, "heartbeat_timeout") :-
    shard_heartbeat_stale(ShardName),
    system_startup(ShardName, /auto).

# System shards that must auto-start
system_startup(/perception_firewall, /auto).
system_startup(/executive_policy, /auto).
system_startup(/constitution_gate, /auto).
system_startup(/world_model_ingestor, /on_demand).
system_startup(/tactile_router, /on_demand).
system_startup(/session_planner, /on_demand).

# -----------------------------------------------------------------------------
# 21.4 Safety Violation Handling (Constitution Gate)
# -----------------------------------------------------------------------------

# A safety violation blocks all further actions
block_all_actions("safety_violation") :-
    safety_violation(_, _, _, _).

# Security anomaly triggers investigation
# Need to check if there are uninvestigated anomalies
next_action(/investigate_anomaly) :-
    security_anomaly(AnomalyID, _, _),
    !anomaly_investigated(AnomalyID).

# Helper for safe negation
anomaly_investigated(AnomalyID) :-
    security_anomaly(AnomalyID, _, _),
    investigation_result(AnomalyID, _).

# Pattern recognition for repeated violations
repeated_violation_pattern(Pattern) :-
    safety_violation(_, Pattern, _, _),
    violation_count(Pattern, Count),
    Count >= 3.

# Propose rule when pattern detected (Autopoiesis)
propose_safety_rule(Pattern) :-
    repeated_violation_pattern(Pattern).

# -----------------------------------------------------------------------------
# 21.5 World Model Updates (World Model Ingestor)
# -----------------------------------------------------------------------------

# File change triggers world model update
# Note: Using fn:minus directly for time difference calculation.
world_model_stale(File) :-
    modified(File),
    file_topology(File, _, _, LastUpdate, _),
    current_time(Now),
    Now >= LastUpdate,
    Diff = fn:minus(Now, LastUpdate),
    Diff > 5.

# Trigger ingestor when world model is stale
next_action(/update_world_model) :-
    world_model_stale(_),
    system_shard_healthy(/world_model_ingestor).

# File topology derived from filesystem
file_in_project(File) :-
    file_topology(File, _, _, _, _).

# Symbol graph connectivity (uses dependency_link for edges)
symbol_reachable(From, To) :-
    dependency_link(From, To, _).

symbol_reachable(From, To) :-
    dependency_link(From, Mid, _),
    symbol_reachable(Mid, To).

# -----------------------------------------------------------------------------
# 21.6 Routing Table (Tactile Router)
# -----------------------------------------------------------------------------

# Default routing table entries (can be extended via Autopoiesis)
routing_table(/fs_read, /read_file, /low).
routing_table(/fs_write, /write_file, /medium).
routing_table(/exec_cmd, /execute_command, /high).
routing_table(/browser, /browser_action, /high).
routing_table(/code_graph, /analyze_code, /low).

# Tool is allowed for action type
tool_allowed(Tool, ActionType) :-
    routing_table(ActionType, Tool, _),
    tool_allowlist(Tool, _).

# Route action to appropriate tool
route_action(ActionID, Tool) :-
    action_ready_for_routing(ActionID),
    action_type(ActionID, ActionType),
    tool_allowed(Tool, ActionType).

# Routing blocked if no tool available
routing_blocked(ActionID, "no_tool_available") :-
    action_ready_for_routing(ActionID),
    action_type(ActionID, ActionType),
    !has_tool_for_action(ActionType).

# Helper for safe negation
has_tool_for_action(ActionType) :-
    tool_allowed(_, ActionType).

# -----------------------------------------------------------------------------
# 21.7 Session Planning (Session Planner)
# -----------------------------------------------------------------------------

# Agenda item is ready when dependencies complete
agenda_item_ready(ItemID) :-
    agenda_item(ItemID, _, _, /pending, _),
    !has_incomplete_dependency(ItemID).

# Helper for dependency checking
has_incomplete_dependency(ItemID) :-
    agenda_dependency(ItemID, DepID),
    agenda_item(DepID, _, _, Status, _),
    /completed != Status.

# Next agenda item: highest priority ready item
next_agenda_item(ItemID) :-
    agenda_item_ready(ItemID),
    !has_higher_priority_item(ItemID).

# Helper for priority ordering
has_higher_priority_item(ItemID) :-
    agenda_item(ItemID, _, Priority, _, _),
    agenda_item_ready(OtherID),
    OtherID != ItemID,
    agenda_item(OtherID, _, OtherPriority, _, _),
    OtherPriority > Priority.

# Checkpoint needed based on time or completion (10 minutes = 600 seconds)
checkpoint_due() :-
    last_checkpoint_time(LastTime),
    current_time(Now),
    Now >= LastTime,
    Diff = fn:minus(Now, LastTime),
    Diff > 600.

next_action(/create_checkpoint) :-
    checkpoint_due().

# Blocked item triggers escalation after retries
agenda_item_escalate(ItemID, "max_retries_exceeded") :-
    agenda_item(ItemID, _, _, /blocked, _),
    item_retry_count(ItemID, Count),
    Count >= 3.

escalation_needed(/session_planner, ItemID, Reason) :-
    agenda_item_escalate(ItemID, Reason).

# -----------------------------------------------------------------------------
# 21.8 On-Demand Shard Activation
# -----------------------------------------------------------------------------

# Activate world_model_ingestor when files change
activate_shard(/world_model_ingestor) :-
    modified(_),
    !system_shard_healthy(/world_model_ingestor).

# Activate tactile_router when actions are pending
activate_shard(/tactile_router) :-
    action_ready_for_routing(_),
    !system_shard_healthy(/tactile_router).

# Activate session_planner for campaigns or complex goals
activate_shard(/session_planner) :-
    current_campaign(_),
    !system_shard_healthy(/session_planner).

activate_shard(/session_planner) :-
    user_intent(_, _, /plan, _, _),
    !system_shard_healthy(/session_planner).

# -----------------------------------------------------------------------------
# 21.9 Autopoiesis Integration for System Shards
# -----------------------------------------------------------------------------

# Unhandled case tracking (for rule learning)
unhandled_case_count(ShardName, Count) :-
    system_shard(ShardName, _),
    unhandled_cases(ShardName, Cases),
    list_length(Cases, Count).

# Trigger LLM for rule proposal when threshold reached
propose_new_rule(ShardName) :-
    unhandled_case_count(ShardName, Count),
    Count >= 3.

# Proposed rule needs human approval if low confidence (confidence on 0-100 scale)
rule_needs_approval(RuleID) :-
    proposed_rule(RuleID, _, _, Confidence),
    Confidence < 80.

# Auto-apply rule if high confidence (confidence on 0-100 scale)
auto_apply_rule(RuleID) :-
    proposed_rule(RuleID, _, _, Confidence),
    Confidence >= 80,
    !rule_applied(RuleID).

# Helper for safe negation
rule_applied(RuleID) :-
    applied_rule(RuleID, _).

# Learn from successful rule applications
learning_signal(/rule_success, RuleID) :-
    applied_rule(RuleID, Timestamp),
    rule_outcome(RuleID, /success, _).

# -----------------------------------------------------------------------------
# 21.10 OODA Loop Coordination
# -----------------------------------------------------------------------------

# OODA phases: Observe → Orient → Decide → Act
ooda_phase(/observe) :-
    pending_intent(IntentID),
    !intent_ready_for_executive(IntentID).

ooda_phase(/orient) :-
    intent_ready_for_executive(IntentID),
    pending_intent(IntentID),
    !has_next_action().

ooda_phase(/decide) :-
    has_next_action(),
    next_action(ActionID),
    !action_permitted(ActionID).

ooda_phase(/act) :-
    action_ready_for_routing(_).

# Helper for OODA phase detection
has_next_action() :-
    next_action(_).

# Current OODA state for debugging/monitoring
current_ooda_phase(Phase) :-
    ooda_phase(Phase).

# OODA loop stalled detection (30 second threshold)
ooda_stalled(Reason) :-
    pending_intent(_),
    !has_next_action(),
    current_time(Now),
    last_action_time(LastTime),
    Now >= LastTime,
    Diff = fn:minus(Now, LastTime),
    Diff > 30,
    Reason = "no_action_derived".

# Escalate stalled OODA loop
escalation_needed(/ooda_loop, "stalled", Reason) :-
    ooda_stalled(Reason).

# =============================================================================
# SECTION 22: CODE DOM RULES
# =============================================================================
# Rules for semantic code element operations - treating code like a DOM.
# These rules coordinate file scope, element queries, and edit tracking.

# -----------------------------------------------------------------------------
# 22.1 File Scope Rules
# -----------------------------------------------------------------------------

# A file is in scope if it's the active file
in_scope(File) :- active_file(File).

# A file is in scope if the active file imports it
in_scope(File) :-
    active_file(ActiveFile),
    dependency_link(ActiveFile, File, _).

# A file is in scope if it imports the active file
in_scope(File) :-
    active_file(ActiveFile),
    dependency_link(File, ActiveFile, _).

# -----------------------------------------------------------------------------
# 22.2 Element Accessibility Rules
# -----------------------------------------------------------------------------

# An element is editable if its file is in scope and it has replace action
editable(Ref) :-
    code_element(Ref, _, File, _, _),
    in_scope(File),
    code_interactable(Ref, /replace).

# All functions in scope (for querying)
function_in_scope(Ref, File, Sig) :-
    code_element(Ref, /function, File, _, _),
    in_scope(File),
    element_signature(Ref, Sig).

# All methods in scope
method_in_scope(Ref, File, Sig) :-
    code_element(Ref, /method, File, _, _),
    in_scope(File),
    element_signature(Ref, Sig).

# Method belongs to struct
method_of(MethodRef, StructRef) :- element_parent(MethodRef, StructRef).

# -----------------------------------------------------------------------------
# 22.3 Transitive Element Containment
# -----------------------------------------------------------------------------

# Direct containment
code_contains(Parent, Child) :- element_parent(Child, Parent).

# Transitive containment
code_contains(Ancestor, Descendant) :-
    element_parent(Mid, Ancestor),
    code_contains(Mid, Descendant).

# -----------------------------------------------------------------------------
# 22.4 Safety and Complexity Rules
# -----------------------------------------------------------------------------

# Safe to modify: element is in scope (implicitly has context)
safe_to_modify(Ref) :-
    code_element(Ref, _, File, _, _),
    in_scope(File).

# Helper: count elements for complexity analysis (evaluated in Go runtime)
# Note: Mangle doesn't have != operator, so we use virtual predicate for counting
element_count_high() :-
    code_element(Ref1, _, _, _, _),
    code_element(Ref2, _, _, _, _),
    code_element(Ref3, _, _, _, _),
    code_element(Ref4, _, _, _, _),
    code_element(Ref5, _, _, _, _).

# Trigger campaign for complex refactors affecting many elements
requires_campaign(Intent) :-
    user_intent(Intent, /mutation, _, Target, _),
    in_scope(Target),
    element_count_high().

# -----------------------------------------------------------------------------
# 22.5 Next Action Derivation for Code DOM
# -----------------------------------------------------------------------------

# Open file when targeting a file that's not yet in scope
next_action(/open_file) :-
    user_intent(_, _, _, Target, _),
    file_topology(Target, _, /go, _, _),
    !active_file(Target),
    !in_scope(Target).

# Query elements when file is open and we need to find something
next_action(/query_elements) :-
    active_file(_),
    user_intent(_, /query, _, _, _).

# Edit element when mutation targets a known element
next_action(/edit_element) :-
    user_intent(_, /mutation, _, Ref, _),
    code_element(Ref, _, File, _, _),
    in_scope(File).

# Refresh scope after external changes
next_action(/refresh_scope) :-
    active_file(File),
    modified(File),
    !scope_refreshed(File).

# Helper for safe negation
scope_refreshed(File) :-
    file_in_scope(File, _, _, _),
    !modified(File).

# -----------------------------------------------------------------------------
# 22.6 Learning From Code Edits
# -----------------------------------------------------------------------------

# Track edit outcomes for learning
successful_edit(Ref, EditType) :-
    code_edit_outcome(Ref, EditType, /true, _).

failed_edit(Ref, EditType) :-
    code_edit_outcome(Ref, EditType, /false, _).

# Proven safe: edit pattern has succeeded multiple times (3+)
# Note: Exact counting done in Go runtime; this is a heuristic trigger
proven_safe_edit(Ref, EditType) :-
    code_edit_outcome(Ref, EditType, /true, _),
    code_edit_outcome(Ref2, EditType, /true, _),
    code_edit_outcome(Ref3, EditType, /true, _),
    Ref != Ref2, Ref2 != Ref3, Ref != Ref3.

# Promote to long-term memory when edit pattern is proven
promote_to_long_term(/edit_pattern, EditType) :-
    proven_safe_edit(_, EditType).

# -----------------------------------------------------------------------------
# 22.7 Code DOM Activation Rules
# -----------------------------------------------------------------------------

# Boost activation for elements in the active file
activation(Ref, 100) :-
    code_element(Ref, _, File, _, _),
    active_file(File).

# Boost activation for elements matching current intent target
activation(Ref, 120) :-
    code_element(Ref, _, _, _, _),
    user_intent(_, _, _, Ref, _).

# Suppress activation for elements in files outside scope
activation(Ref, -50) :-
    code_element(Ref, _, File, _, _),
    file_topology(File, _, _, _, _),
    !in_scope(File).

# -----------------------------------------------------------------------------
# 22.8 Edit Safety & Risk Assessment Rules
# -----------------------------------------------------------------------------

# Public function has external callers (exported = can be called from outside)
has_external_callers(Ref) :-
    code_element(Ref, /function, _, _, _),
    element_visibility(Ref, /public).

has_external_callers(Ref) :-
    code_element(Ref, /method, _, _, _),
    element_visibility(Ref, /public).

# Breaking change risk: HIGH for public API functions
breaking_change_risk(Ref, /high, "public_api") :-
    has_external_callers(Ref),
    api_handler_function(Ref, _, _).

# Breaking change risk: HIGH for public interface methods
breaking_change_risk(Ref, /high, "interface_contract") :-
    code_element(Ref, /method, _, _, _),
    element_visibility(Ref, /public),
    element_parent(Ref, InterfaceRef),
    code_element(InterfaceRef, /interface, _, _, _).

# Breaking change risk: MEDIUM for public functions in libraries
breaking_change_risk(Ref, /medium, "public_function") :-
    has_external_callers(Ref),
    !api_handler_function(Ref, _, _).

# Breaking change risk: LOW for private elements
breaking_change_risk(Ref, /low, "private") :-
    code_element(Ref, _, _, _, _),
    element_visibility(Ref, /private).

# Breaking change risk: CRITICAL for generated code
breaking_change_risk(Ref, /critical, "generated_will_be_overwritten") :-
    code_element(Ref, _, File, _, _),
    generated_code(File, _, _).

# Edit unsafe: generated code will be overwritten
edit_unsafe(Ref, "generated_code") :-
    code_element(Ref, _, File, _, _),
    generated_code(File, _, _).

# Edit unsafe: CGo requires special handling
edit_unsafe(Ref, "cgo_code") :-
    code_element(Ref, _, File, _, _),
    cgo_code(File).

# Edit unsafe: file has hash mismatch (concurrent modification)
edit_unsafe(Ref, "concurrent_modification") :-
    code_element(Ref, _, File, _, _),
    file_hash_mismatch(File, _, _).

# Edit unsafe: element is stale
edit_unsafe(Ref, "stale_reference") :-
    element_stale(Ref, _).

# -----------------------------------------------------------------------------
# 22.9 Mock & Interface Rules
# -----------------------------------------------------------------------------

# Struct implements interface if it has methods matching interface methods
# (Simplified: if struct has methods and interface exists in same package)
interface_impl(StructRef, InterfaceRef) :-
    code_element(StructRef, /struct, File, _, _),
    code_element(InterfaceRef, /interface, File, _, _),
    element_parent(MethodRef, StructRef),
    code_element(MethodRef, /method, _, _, _).

# Test file mocks source file if it's a _test.go in same package
mock_file(TestFile, SourceFile) :-
    file_topology(TestFile, _, /go, _, _),
    file_topology(SourceFile, _, /go, _, _),
    TestFile != SourceFile.
    # Note: Actual mock detection needs content analysis in Go runtime

# Suggest updating mocks when source function signature changes
suggest_update_mocks(Ref) :-
    code_element(Ref, /function, File, _, _),
    element_visibility(Ref, /public),
    element_modified(Ref, _, _),
    mock_file(TestFile, File).

suggest_update_mocks(Ref) :-
    code_element(Ref, /method, File, _, _),
    element_visibility(Ref, /public),
    element_modified(Ref, _, _),
    mock_file(TestFile, File).

# -----------------------------------------------------------------------------
# 22.10 Scope Staleness Detection
# -----------------------------------------------------------------------------

# File modified externally: hash doesn't match what we loaded
file_modified_externally(Path) :-
    file_hash_mismatch(Path, _, _).

# Scope needs refresh when any in-scope file was modified
needs_scope_refresh() :-
    active_file(ActiveFile),
    in_scope(File),
    modified(File).

needs_scope_refresh() :-
    file_modified_externally(_).

# Element edit blocked due to concurrent modification
element_edit_blocked(Ref, "concurrent_modification") :-
    code_element(Ref, _, File, _, _),
    file_modified_externally(File).

# Element edit blocked due to generated code
element_edit_blocked(Ref, "generated_code") :-
    code_element(Ref, _, File, _, _),
    generated_code(File, _, _).

# Element edit blocked due to parse error
element_edit_blocked(Ref, "parse_error") :-
    code_element(Ref, _, File, _, _),
    parse_error(File, _, _).

# -----------------------------------------------------------------------------
# 22.11 API Client/Handler Awareness
# -----------------------------------------------------------------------------

# API client functions should be tested carefully
requires_integration_test(Ref) :-
    api_client_function(Ref, _, _).

# API handlers need contract validation
requires_contract_check(Ref) :-
    api_handler_function(Ref, _, _),
    element_modified(Ref, _, _).

# Warn when editing API client without corresponding test
api_edit_warning(Ref, "no_integration_test") :-
    api_client_function(Ref, _, _),
    element_modified(Ref, _, _).

# =============================================================================
# SECTION 23: VERIFICATION LOOP POLICY (Post-Execution Quality Enforcement)
# =============================================================================
# Implements the quality-enforcing verification loop that ensures tasks are
# completed PROPERLY - no shortcuts, no mock code, no corner-cutting.
# Per plan: Execute → Verify → Corrective Action → Retry (max 3)

# -----------------------------------------------------------------------------
# 23.1 Verification State Management
# -----------------------------------------------------------------------------

# Task has any quality violation
has_quality_violation(TaskID) :-
    quality_violation(TaskID, _).

# Task passed verification (successful with no violations)
verification_succeeded(TaskID) :-
    verification_attempt(TaskID, _, /success),
    !has_quality_violation(TaskID).

# Block further execution after max retries (3 attempts)
verification_blocked(TaskID) :-
    verification_attempt(TaskID, 3, /failure).

# -----------------------------------------------------------------------------
# 23.2 Corrective Action Triggers
# -----------------------------------------------------------------------------

# Task needs corrective action if verification failed and not blocked
needs_corrective_action(TaskID) :-
    verification_attempt(TaskID, AttemptNum, /failure),
    AttemptNum < 3,
    !verification_blocked(TaskID).

# Specific corrective action based on violation type

# Mock code → needs research for real implementation
next_action(/corrective_research) :-
    current_task(TaskID),
    quality_violation(TaskID, /mock_code),
    needs_corrective_action(TaskID).

# Placeholder code → needs research for proper implementation
next_action(/corrective_research) :-
    current_task(TaskID),
    quality_violation(TaskID, /placeholder),
    needs_corrective_action(TaskID).

# Hallucinated API → needs documentation lookup
next_action(/corrective_docs) :-
    current_task(TaskID),
    quality_violation(TaskID, /hallucinated_api),
    needs_corrective_action(TaskID).

# Incomplete implementation → may need decomposition
next_action(/corrective_decompose) :-
    current_task(TaskID),
    quality_violation(TaskID, /incomplete),
    needs_corrective_action(TaskID).

# Missing error handling → needs docs/examples
next_action(/corrective_docs) :-
    current_task(TaskID),
    quality_violation(TaskID, /missing_errors),
    needs_corrective_action(TaskID).

# Fake tests → needs research on testing patterns
next_action(/corrective_research) :-
    current_task(TaskID),
    quality_violation(TaskID, /fake_tests),
    needs_corrective_action(TaskID).

# -----------------------------------------------------------------------------
# 23.3 Escalation Logic
# -----------------------------------------------------------------------------

# Escalation required when max retries exceeded
escalation_required(TaskID, "max_retries_exceeded") :-
    verification_blocked(TaskID),
    current_task(TaskID).

# Escalation triggers next_action
next_action(/escalate_to_user) :-
    escalation_required(_, _).

# Block all other actions during escalation
block_all_actions("verification_escalation") :-
    escalation_required(_, _).

# -----------------------------------------------------------------------------
# 23.4 Learning Signals from Quality Violations
# -----------------------------------------------------------------------------

# Learn to avoid mock code patterns
learning_signal(/avoid_mock_code) :-
    quality_violation(_, /mock_code).

# Learn to avoid placeholder patterns
learning_signal(/avoid_placeholders) :-
    quality_violation(_, /placeholder).

# Learn to avoid hallucinated APIs
learning_signal(/avoid_hallucinated_api) :-
    quality_violation(_, /hallucinated_api).

# Learn to avoid incomplete implementations
learning_signal(/avoid_incomplete) :-
    quality_violation(_, /incomplete).

# Learn to avoid fake tests
learning_signal(/avoid_fake_tests) :-
    quality_violation(_, /fake_tests).

# Learn to include error handling
learning_signal(/require_error_handling) :-
    quality_violation(_, /missing_errors).

# Promote learning signals to long-term memory after repeated violations
promote_to_long_term(/quality_pattern, ViolationType) :-
    quality_violation(Task1, ViolationType),
    quality_violation(Task2, ViolationType),
    quality_violation(Task3, ViolationType),
    Task1 != Task2,
    Task2 != Task3,
    Task1 != Task3.

# -----------------------------------------------------------------------------
# 23.5 Shard Selection for Retries
# -----------------------------------------------------------------------------

# Prefer specialist shards for complex tasks after failure
delegate_task(/specialist, TaskID, /pending) :-
    current_task(TaskID),
    verification_attempt(TaskID, AttemptNum, /failure),
    AttemptNum >= 1,
    shard_profile(_, /specialist, _).

# Delegate to researcher when documentation is needed
delegate_task(/researcher, Query, /pending) :-
    current_task(TaskID),
    corrective_query(TaskID, _, Query),
    corrective_action_taken(TaskID, /research).

# Delegate to researcher when docs lookup needed
delegate_task(/researcher, Query, /pending) :-
    current_task(TaskID),
    corrective_query(TaskID, _, Query),
    corrective_action_taken(TaskID, /docs).

# -----------------------------------------------------------------------------
# 23.6 Verification Strategy Selection
# -----------------------------------------------------------------------------

# Activate verification strategy when task has been executed
active_strategy(/verification_loop) :-
    current_task(TaskID),
    verification_attempt(TaskID, _, _).

# Block normal task execution when in verification failure state
execution_blocked("verification_in_progress") :-
    current_task(TaskID),
    needs_corrective_action(TaskID).

# -----------------------------------------------------------------------------
# 23.7 Quality Gate Integration with Commit Barrier
# -----------------------------------------------------------------------------

# Block commit if any task has unresolved quality violations
block_commit("Quality Violations") :-
    has_quality_violation(_).

# Block commit if verification is blocked (max retries reached)
block_commit("Verification Failed") :-
    verification_blocked(_).

# -----------------------------------------------------------------------------
# 23.8 Context Enrichment Tracking
# -----------------------------------------------------------------------------

# Track successful corrective actions for learning
corrective_action_effective(TaskID, ActionType) :-
    corrective_action_taken(TaskID, ActionType),
    verification_attempt(TaskID, AttemptNum, /failure),
    verification_attempt(TaskID, NextAttempt, /success),
    NextAttempt > AttemptNum.

# Learn from effective corrective actions
learning_signal(/effective_correction, ActionType) :-
    corrective_action_effective(_, ActionType).

# -----------------------------------------------------------------------------
# 23.9 Verification Metrics (for monitoring)
# -----------------------------------------------------------------------------

# Helper: track tasks that succeeded on first attempt
first_attempt_success(TaskID) :-
    verification_attempt(TaskID, 1, /success),
    !has_quality_violation(TaskID).

# Helper: track tasks that required retries
required_retry(TaskID) :-
    verification_attempt(TaskID, AttemptNum, /success),
    AttemptNum > 1.

# Helper: track specific violation types for analytics
violation_type_count_high(ViolationType) :-
    quality_violation(T1, ViolationType),
    quality_violation(T2, ViolationType),
    quality_violation(T3, ViolationType),
    quality_violation(T4, ViolationType),
    quality_violation(T5, ViolationType),
    T1 != T2, T2 != T3, T3 != T4, T4 != T5.

# Trigger rule proposal for high-frequency violations
propose_new_rule(/verification_policy) :-
    violation_type_count_high(_).

# =============================================================================
# SECTION 24: REASONING TRACE POLICY (Shard Learning from Traces)
# =============================================================================
# Rules for analyzing shard LLM traces and deriving learning signals.
# Covers all 4 shard types: system, ephemeral, LLM-created, user-created specialists.

# -----------------------------------------------------------------------------
# 24.1 Trace Quality Tracking
# -----------------------------------------------------------------------------

# Low quality trace (needs review) - score on 0-100 scale
low_quality_trace(TraceID) :-
    trace_quality(TraceID, Score),
    Score < 50.

# High quality trace (good for learning) - score on 0-100 scale
high_quality_trace(TraceID) :-
    trace_quality(TraceID, Score),
    Score >= 80.

# -----------------------------------------------------------------------------
# 24.2 Shard Performance Patterns
# -----------------------------------------------------------------------------

# Shard has high failure rate (3+ consecutive failures)
shard_struggling(ShardType) :-
    reasoning_trace(T1, ShardType, _, _, /false, _),
    reasoning_trace(T2, ShardType, _, _, /false, _),
    reasoning_trace(T3, ShardType, _, _, /false, _),
    T1 != T2,
    T2 != T3,
    T1 != T3.

# Shard is performing well (5+ consecutive successes)
shard_performing_well(ShardType) :-
    reasoning_trace(T1, ShardType, _, _, /true, _),
    reasoning_trace(T2, ShardType, _, _, /true, _),
    reasoning_trace(T3, ShardType, _, _, /true, _),
    reasoning_trace(T4, ShardType, _, _, /true, _),
    reasoning_trace(T5, ShardType, _, _, /true, _),
    T1 != T2,
    T2 != T3,
    T3 != T4,
    T4 != T5.

# Detect slow reasoning (> 30 seconds)
slow_reasoning_detected(ShardType) :-
    reasoning_trace(_, ShardType, _, _, _, DurationMs),
    DurationMs > 30000.

# -----------------------------------------------------------------------------
# 24.3 Learning Signals from Traces
# -----------------------------------------------------------------------------

# Learn from repeated failures - shard needs help
learning_from_traces(/shard_needs_help, ShardType) :-
    shard_struggling(ShardType).

# Learn from success patterns
learning_from_traces(/success_pattern, ShardType) :-
    shard_performing_well(ShardType).

# Learn from slow traces (performance issue)
learning_from_traces(/slow_reasoning, ShardType) :-
    slow_reasoning_detected(ShardType).

# Promote learning signals to long-term memory
promote_to_long_term(/shard_pattern, ShardType) :-
    learning_from_traces(_, ShardType).

# -----------------------------------------------------------------------------
# 24.4 Cross-Shard Learning (Specialist vs Ephemeral)
# -----------------------------------------------------------------------------

# Specialist outperforms ephemeral for same task type
specialist_outperforms(SpecialistName, TaskType) :-
    reasoning_trace(T1, SpecialistName, /specialist, _, /true, _),
    reasoning_trace(T2, /coder, /ephemeral, _, /false, _),
    trace_task_type(T1, TaskType),
    trace_task_type(T2, TaskType).

# Suggest using specialist instead of ephemeral
suggest_use_specialist(TaskType, SpecialistName) :-
    specialist_outperforms(SpecialistName, TaskType).

# Suggest switching shard when current one struggles
shard_switch_suggestion(TaskType, CurrentShard, AlternateShard) :-
    shard_struggling(CurrentShard),
    shard_performing_well(AlternateShard),
    shard_can_handle(AlternateShard, TaskType).

# -----------------------------------------------------------------------------
# 24.5 Trace-Based Context Enhancement
# -----------------------------------------------------------------------------

# Boost activation for successful trace patterns in current session
activation(TraceID, 80) :-
    high_quality_trace(TraceID),
    reasoning_trace(TraceID, ShardType, _, SessionID, /true, _),
    session_state(SessionID, /active, _).

# Suppress failed trace patterns
activation(TraceID, -30) :-
    low_quality_trace(TraceID),
    reasoning_trace(TraceID, _, _, _, /false, _).

# -----------------------------------------------------------------------------
# 24.6 Corrective Actions Based on Traces
# -----------------------------------------------------------------------------

# Escalate if multiple shards struggling
escalation_needed(/system_health, /shard_performance, "Multiple shards struggling") :-
    shard_struggling(Shard1),
    shard_struggling(Shard2),
    Shard1 != Shard2.

# Suggest spawning researcher for failed traces with unknown errors
delegate_task(/researcher, TaskContext, /pending) :-
    reasoning_trace(TraceID, _, _, _, /false, _),
    trace_error(TraceID, /unknown),
    trace_task_type(TraceID, TaskContext).

# -----------------------------------------------------------------------------
# 24.7 System Shard Trace Monitoring
# -----------------------------------------------------------------------------

# System shard traces get special attention
activation(TraceID, 90) :-
    reasoning_trace(TraceID, _, /system, _, _, _).

# Alert on system shard failures
escalation_needed(/system_health, ShardType, "System shard failure") :-
    reasoning_trace(_, ShardType, /system, _, /false, _).

# -----------------------------------------------------------------------------
# 24.8 Specialist Knowledge Hydration from Traces
# -----------------------------------------------------------------------------

# Specialist with good traces should be preferred for similar tasks
delegate_task(SpecialistName, Task, /pending) :-
    shard_performing_well(SpecialistName),
    shard_profile(SpecialistName, /specialist, _),
    trace_task_type(_, TaskType),
    shard_can_handle(SpecialistName, TaskType),
    user_intent(_, _, _, Task, _).

# Learn which tasks specialists handle well
shard_can_handle(ShardType, TaskType) :-
    reasoning_trace(TraceID, ShardType, /specialist, _, /true, _),
    trace_task_type(TraceID, TaskType),
    high_quality_trace(TraceID).

# =============================================================================
# SECTION 25: HOLOGRAPHIC RETRIEVAL (Cartographer)
# =============================================================================

# "X-Ray Vision": Find context relevant to the target file/symbol

# 1. Callers of the target symbol
relevant_context(File) :-
    user_intent(_, _, _, TargetSymbol, _),
    code_calls(Caller, TargetSymbol),
    code_defines(File, Caller, _, _, _).

# 2. Definitions in the target file
relevant_context(Symbol) :-
    user_intent(_, _, _, TargetFile, _),
    code_defines(TargetFile, Symbol, _, _, _).

# 3. Implementations of target interface
relevant_context(Struct) :-
    user_intent(_, _, _, Interface, _),
    code_implements(Struct, Interface).

# 4. Structs implementing the target interface (if target is interface)
relevant_context(StructFile) :-
    user_intent(_, _, _, Interface, _),
    code_implements(Struct, Interface),
    code_defines(StructFile, Struct, _, _, _).

# Boost activation for holographic matches
activation(Ctx, 85) :-
    relevant_context(Ctx).

# =============================================================================
# SECTION 26: SPECULATIVE DREAMER (Precognition Layer)
# =============================================================================

# Enumerate critical files that must never disappear
critical_file("go.mod").
critical_file("go.sum").

# Panic if a projected action would remove a critical file
panic_state(Action, "critical_file_missing") :-
    projected_fact(Action, /file_missing, File),
    critical_file(File).

# Panic on obviously dangerous exec commands
panic_state(Action, "dangerous_exec") :-
    projected_fact(Action, /exec_danger, _).

# Panic when deleting a file whose symbols are covered by tests
panic_state(Action, "deletes_tested_symbol") :-
    projected_fact(Action, /file_missing, _),
    projected_fact(Action, /impacts_test, _).

# Panic when Dreamer flags critical path hits
panic_state(Action, "critical_path_missing") :-
    projected_fact(Action, /critical_path_hit, _).

# Block actions the Dreamer marks as panic states
dream_block(Action, Reason) :-
    panic_state(Action, Reason).

# =============================================================================
# SECTION 40: INTELLIGENT TOOL ROUTING (§40)
# =============================================================================
# Routes Ouroboros-generated tools to shards based on capabilities, intent,
# domain matching, and usage history. Enables context-window-aware injection.

# -----------------------------------------------------------------------------
# 40.1 Base Shard-Capability Affinities (EDB)
# -----------------------------------------------------------------------------
# Score 0-100 (integer scale) indicating how relevant a capability is to each shard type
# NOTE: Must use integers because Mangle comparison operators don't support floats

# CoderShard affinities
shard_capability_affinity(/coder, /generation, 100).
shard_capability_affinity(/coder, /debugging, 90).
shard_capability_affinity(/coder, /transformation, 80).
shard_capability_affinity(/coder, /inspection, 50).
shard_capability_affinity(/coder, /validation, 40).
shard_capability_affinity(/coder, /execution, 60).

# TesterShard affinities
shard_capability_affinity(/tester, /validation, 100).
shard_capability_affinity(/tester, /execution, 90).
shard_capability_affinity(/tester, /inspection, 70).
shard_capability_affinity(/tester, /debugging, 60).
shard_capability_affinity(/tester, /analysis, 50).

# ReviewerShard affinities
shard_capability_affinity(/reviewer, /inspection, 100).
shard_capability_affinity(/reviewer, /analysis, 90).
shard_capability_affinity(/reviewer, /validation, 60).
shard_capability_affinity(/reviewer, /debugging, 40).

# ResearcherShard affinities
shard_capability_affinity(/researcher, /knowledge, 100).
shard_capability_affinity(/researcher, /analysis, 80).
shard_capability_affinity(/researcher, /inspection, 60).

# Generalist affinities (moderate across all)
shard_capability_affinity(/generalist, /generation, 50).
shard_capability_affinity(/generalist, /validation, 50).
shard_capability_affinity(/generalist, /inspection, 50).
shard_capability_affinity(/generalist, /analysis, 50).
shard_capability_affinity(/generalist, /execution, 50).
shard_capability_affinity(/generalist, /knowledge, 50).
shard_capability_affinity(/generalist, /debugging, 50).
shard_capability_affinity(/generalist, /transformation, 50).

# -----------------------------------------------------------------------------
# 40.2 Intent-Capability Mappings (EDB)
# -----------------------------------------------------------------------------
# Maps user intent verbs to required capabilities with importance weights (0-100 scale)

# Mutation intents
intent_requires_capability(/implement, /generation, 100).
intent_requires_capability(/implement, /validation, 50).
intent_requires_capability(/refactor, /transformation, 100).
intent_requires_capability(/refactor, /analysis, 70).
intent_requires_capability(/fix, /debugging, 100).
intent_requires_capability(/fix, /validation, 80).
intent_requires_capability(/generate, /generation, 100).
intent_requires_capability(/scaffold, /generation, 90).
intent_requires_capability(/init, /generation, 80).

# Query intents
intent_requires_capability(/test, /validation, 100).
intent_requires_capability(/test, /execution, 90).
intent_requires_capability(/review, /inspection, 100).
intent_requires_capability(/review, /analysis, 80).
intent_requires_capability(/explain, /analysis, 100).
intent_requires_capability(/explain, /knowledge, 70).
intent_requires_capability(/debug, /debugging, 100).
intent_requires_capability(/debug, /inspection, 80).

# Research intents
intent_requires_capability(/research, /knowledge, 100).
intent_requires_capability(/research, /analysis, 60).
intent_requires_capability(/explore, /inspection, 90).
intent_requires_capability(/explore, /analysis, 80).
intent_requires_capability(/explore, /knowledge, 50).

# Run intents
intent_requires_capability(/run, /execution, 100).
intent_requires_capability(/run, /validation, 40).

# -----------------------------------------------------------------------------
# 40.3 Tool Relevance Derivation Rules (IDB)
# -----------------------------------------------------------------------------

# 40.3.1 Base Relevance: Tool matches shard's capability affinity
tool_base_relevance(ShardType, ToolName, AffinityScore) :-
    tool_capability(ToolName, Cap),
    shard_capability_affinity(ShardType, Cap, AffinityScore),
    tool_registered(ToolName, _).

# 40.3.2 Intent Boost: Tool matches current intent's required capabilities
tool_intent_relevance(ToolName, Weight) :-
    current_intent(IntentID),
    user_intent(IntentID, _, Verb, _, _),
    intent_requires_capability(Verb, Cap, Weight),
    tool_capability(ToolName, Cap).

# No current intent = no boost (fallback rule)
# Uses helper predicate for safe negation
tool_intent_relevance(ToolName, 0) :-
    tool_registered(ToolName, _),
    !has_current_intent().

# 40.3.3 Domain Boost: Tool matches target file's language/domain
# Score: 30 (out of 100)
tool_domain_relevance(ToolName, 30) :-
    current_intent(IntentID),
    user_intent(IntentID, _, _, Target, _),
    file_topology(Target, _, Lang, _, _),
    tool_domain(ToolName, Lang).

tool_domain_relevance(ToolName, 0) :-
    tool_registered(ToolName, _),
    !has_tool_domain(ToolName).

# 40.3.4 Success History Boost: Tool succeeded in similar contexts
# Note: Uses simplified scoring - full implementation would compute rate
# Score: 20 (out of 100)
tool_success_relevance(ToolName, 20) :-
    tool_usage_stats(ToolName, ExecCount, SuccessCount, _),
    ExecCount > 0,
    SuccessCount > 0.

tool_success_relevance(ToolName, 0) :-
    tool_registered(ToolName, _),
    !has_tool_usage(ToolName).

# 40.3.5 Recency Boost: Recently used tools likely still relevant
# Note: Full implementation would check timestamp difference
# Score: 15 (out of 100)
tool_recency_relevance(ToolName, 15) :-
    tool_usage_stats(ToolName, _, _, LastUsed),
    current_time(Now),
    LastUsed > 0.

tool_recency_relevance(ToolName, 0) :-
    tool_registered(ToolName, _),
    !has_tool_usage(ToolName).

# 40.3.6 Combined Score: Weighted sum of all relevance factors
# Base relevance weighted at 40%, intent at 30%, domain/success/recency fill rest
# Note: Mangle doesn't support arithmetic in rule bodies, so we use approximation
#       The Go implementation will compute exact scores

# Simplified relevance threshold: tool is relevant if it has base affinity >= 30
relevant_tool(ShardType, ToolName) :-
    tool_base_relevance(ShardType, ToolName, BaseScore),
    BaseScore >= 30.

# Also relevant if intent matches strongly (>= 70)
relevant_tool(ShardType, ToolName) :-
    tool_intent_relevance(ToolName, IntentScore),
    IntentScore >= 70,
    tool_registered(ToolName, _),
    current_shard_type(ShardType).

# System shards see all tools (Type S gets full visibility)
relevant_tool(/system, ToolName) :-
    tool_registered(ToolName, _).

# -----------------------------------------------------------------------------
# 40.4 Helper Predicates
# -----------------------------------------------------------------------------

# has_current_intent() - helper for safe negation
has_current_intent() :- current_intent(_).

# has_tool_domain(ToolName) - helper for safe negation
has_tool_domain(ToolName) :- tool_domain(ToolName, _).

# has_tool_usage(ToolName) - helper for safe negation
has_tool_usage(ToolName) :- tool_usage_stats(ToolName, _, _, _).

# =============================================================================
# SECTION 41: DYNAMIC PROMPT COMPOSITION (Spreading Activation Extension)
# =============================================================================
# Rules for selecting context atoms to inject into shard system prompts.
# Implements spreading activation from user_intent to relevant facts.
# Per codeNERD architecture: facts flow through the kernel to shape LLM context.

# -----------------------------------------------------------------------------
# 41.1 Shard-Specific Context Relevance (3-arity extension)
# -----------------------------------------------------------------------------
# shard_context_atom(ShardID, Atom, Relevance) - context relevance per shard
# Relevance is integer 0-100 scale (Mangle doesn't support floats)

# Context relevance based on intent match - HIGH relevance (90)
# When shard type matches intent category, target is highly relevant
shard_context_atom(ShardID, Target, 90) :-
    active_shard(ShardID, ShardType),
    user_intent(_, ShardType, _, Target, _).

# Propagate specialist knowledge to context - HIGH relevance (80)
shard_context_atom(ShardID, Knowledge, 80) :-
    active_shard(ShardID, _),
    specialist_knowledge(ShardID, _, Knowledge).

# Include campaign constraints in context - MEDIUM relevance (70)
shard_context_atom(ShardID, Constraint, 70) :-
    active_shard(ShardID, ShardType),
    campaign_active(CampaignID),
    campaign_prompt_policy(CampaignID, ShardType, Constraint).

# Include learned exemplars - MEDIUM relevance (60)
shard_context_atom(ShardID, Exemplar, 60) :-
    active_shard(ShardID, ShardType),
    user_intent(_, Category, _, _, _),
    prompt_exemplar(ShardType, Category, Exemplar).

# Include relevant tool descriptions - MEDIUM relevance (65)
shard_context_atom(ShardID, ToolDesc, 65) :-
    active_shard(ShardID, ShardType),
    relevant_tool(ShardType, ToolName),
    tool_description(ToolName, ToolDesc).

# Include recent successful trace patterns - LOW relevance (50)
shard_context_atom(ShardID, TracePattern, 50) :-
    active_shard(ShardID, ShardType),
    high_quality_trace(TraceID),
    reasoning_trace(TraceID, ShardType, _, _, /true, _),
    trace_pattern(TraceID, TracePattern).

# -----------------------------------------------------------------------------
# 41.2 Injectable Context Selection (Threshold Filtering)
# -----------------------------------------------------------------------------

# Select injectable context based on relevance threshold (> 50)
injectable_context(ShardID, Atom) :-
    shard_context_atom(ShardID, Atom, Relevance),
    Relevance > 50.

# High-priority injectable context (relevance >= 80)
injectable_context_priority(ShardID, Atom, /high) :-
    shard_context_atom(ShardID, Atom, Relevance),
    Relevance >= 80.

# Medium-priority injectable context (60 <= relevance < 80)
injectable_context_priority(ShardID, Atom, /medium) :-
    shard_context_atom(ShardID, Atom, Relevance),
    Relevance >= 60,
    Relevance < 80.

# Low-priority injectable context (50 < relevance < 60)
injectable_context_priority(ShardID, Atom, /low) :-
    shard_context_atom(ShardID, Atom, Relevance),
    Relevance > 50,
    Relevance < 60.

# -----------------------------------------------------------------------------
# 41.3 Context Budget Awareness (for context window management)
# -----------------------------------------------------------------------------

# Helper: shard has injectable context
has_injectable_context(ShardID) :-
    injectable_context(ShardID, _).

# Helper: shard has high-priority context
has_high_priority_context(ShardID) :-
    injectable_context_priority(ShardID, _, /high).

# When context budget is limited, only inject high-priority items
context_budget_constrained(ShardID) :-
    active_shard(ShardID, _),
    context_budget(ShardID, Budget),
    Budget < 5000.

# Full context injection allowed when budget is sufficient
context_budget_sufficient(ShardID) :-
    active_shard(ShardID, _),
    context_budget(ShardID, Budget),
    Budget >= 5000.

# Final injectable set: all items when budget sufficient
final_injectable(ShardID, Atom) :-
    context_budget_sufficient(ShardID),
    injectable_context(ShardID, Atom).

# Final injectable set: only high priority when budget constrained
final_injectable(ShardID, Atom) :-
    context_budget_constrained(ShardID),
    injectable_context_priority(ShardID, Atom, /high).

# -----------------------------------------------------------------------------
# 41.4 Spreading Activation Integration
# -----------------------------------------------------------------------------

# Boost activation for atoms selected as injectable context
activation(Atom, 95) :-
    final_injectable(_, Atom).

# Boost activation for specialist knowledge atoms
activation(Knowledge, 85) :-
    specialist_knowledge(_, _, Knowledge).

# Boost activation for campaign prompt policy atoms
activation(Constraint, 75) :-
    campaign_active(_),
    campaign_prompt_policy(_, _, Constraint).

# Boost activation for learned exemplars
activation(Exemplar, 70) :-
    prompt_exemplar(_, _, Exemplar).

# -----------------------------------------------------------------------------
# 41.5 Context Staleness Detection
# -----------------------------------------------------------------------------

# Context atom is stale if it references a modified file
context_stale(ShardID, Atom) :-
    shard_context_atom(ShardID, Atom, _),
    modified(Atom).

# Context atom is stale if specialist knowledge was updated
context_stale(ShardID, Knowledge) :-
    shard_context_atom(ShardID, Knowledge, _),
    specialist_knowledge(ShardID, _, Knowledge),
    specialist_knowledge_updated(ShardID).

# Helper: shard has stale context
has_stale_context(ShardID) :-
    context_stale(ShardID, _).

# Trigger context refresh when stale atoms detected
next_action(/refresh_shard_context) :-
    active_shard(ShardID, _),
    has_stale_context(ShardID).

# -----------------------------------------------------------------------------
# 41.6 Learning Signals from Context Usage
# -----------------------------------------------------------------------------

# Track when injected context leads to successful task completion
context_injection_effective(ShardID, Atom) :-
    final_injectable(ShardID, Atom),
    shard_executed(ShardID, _, /success, _).

# Learn from effective context injections
learning_signal(/effective_context, Atom) :-
    context_injection_effective(_, Atom).

# Promote frequently effective context to long-term memory
promote_to_long_term(/context_pattern, Atom) :-
    context_injection_effective(S1, Atom),
    context_injection_effective(S2, Atom),
    context_injection_effective(S3, Atom),
    S1 != S2,
    S2 != S3,
    S1 != S3.

# =============================================================================
# 42. NORTHSTAR VISION REASONING
# =============================================================================
# Rules for reasoning over northstar facts defined via /northstar command.
# These rules derive strategic insights from vision, personas, capabilities,
# risks, and requirements.

# -----------------------------------------------------------------------------
# 42.1 Critical Path Derivation
# -----------------------------------------------------------------------------

# Derive critical capabilities (priority = /critical)
critical_capability(CapID) :-
    northstar_capability(CapID, _, _, /critical).

# Derive high-risk items (both likelihood AND impact are high)
high_risk(RiskID) :-
    northstar_risk(RiskID, _, /high, /high).

# Helper: risk has at least one mitigation
has_mitigation(RiskID) :-
    northstar_mitigation(RiskID, _).

# Derive unmitigated risks (high risk without any mitigation)
unmitigated_risk(RiskID) :-
    high_risk(RiskID),
    !has_mitigation(RiskID).

# -----------------------------------------------------------------------------
# 42.2 Alignment Analysis
# -----------------------------------------------------------------------------

# Capability addresses persona need when serves relationship exists
capability_addresses_need(CapID, PersonaID, Need) :-
    northstar_serves(CapID, PersonaID),
    northstar_need(PersonaID, Need).

# Helper: persona is served by at least one capability
is_served_persona(PersonaID) :-
    northstar_serves(_, PersonaID).

# Helper: capability serves at least one persona
capability_is_linked(CapID) :-
    northstar_serves(CapID, _).

# Unserved persona - has needs but no capability serves them
unserved_persona(PersonaID, Name) :-
    northstar_persona(PersonaID, Name),
    northstar_need(PersonaID, _),
    !is_served_persona(PersonaID).

# Orphan capability - not linked to any persona
orphan_capability(CapID, Desc) :-
    northstar_capability(CapID, Desc, _, _),
    !capability_is_linked(CapID).

# -----------------------------------------------------------------------------
# 42.3 Requirements Traceability
# -----------------------------------------------------------------------------

# Must-have requirements (priority = /must_have)
must_have_requirement(ReqID, Desc) :-
    northstar_requirement(ReqID, _, Desc, /must_have).

# Helper: requirement is supported by at least one capability
is_supported_req(ReqID) :-
    northstar_supports(ReqID, _).

# Orphan requirement - not linked to any capability
orphan_requirement(ReqID, Desc) :-
    northstar_requirement(ReqID, _, Desc, _),
    !is_supported_req(ReqID).

# Risk-addressing requirement
risk_addressing_requirement(ReqID, RiskID) :-
    northstar_addresses(ReqID, RiskID),
    high_risk(RiskID).

# Helper: risk is addressed by at least one requirement
risk_is_addressed(RiskID) :-
    northstar_addresses(_, RiskID).

# Unaddressed high risk - no requirement addresses it
unaddressed_high_risk(RiskID, Desc) :-
    high_risk(RiskID),
    northstar_risk(RiskID, Desc, _, _),
    !risk_is_addressed(RiskID).

# -----------------------------------------------------------------------------
# 42.4 Timeline Planning
# -----------------------------------------------------------------------------

# Immediate work (timeline = /now)
immediate_capability(CapID, Desc) :-
    northstar_capability(CapID, Desc, /now, _).

# Near-term work (timeline = /6mo)
near_term_capability(CapID, Desc) :-
    northstar_capability(CapID, Desc, /6mo, _).

# Long-term work (timeline = /1yr or /3yr)
long_term_capability(CapID, Desc) :-
    northstar_capability(CapID, Desc, /1yr, _).

long_term_capability(CapID, Desc) :-
    northstar_capability(CapID, Desc, /3yr, _).

# Moonshot capabilities (timeline = /moonshot)
moonshot_capability(CapID, Desc) :-
    northstar_capability(CapID, Desc, /moonshot, _).

# -----------------------------------------------------------------------------
# 42.5 Strategic Warnings
# -----------------------------------------------------------------------------

# Warning: critical capability with unmitigated high risk
strategic_warning(/critical_unmitigated_risk, CapID, RiskID) :-
    critical_capability(CapID),
    northstar_supports(ReqID, CapID),
    northstar_addresses(ReqID, RiskID),
    unmitigated_risk(RiskID).

# Warning: immediate work depends on unaddressed risk
strategic_warning(/immediate_risk_gap, CapID, RiskID) :-
    immediate_capability(CapID, _),
    unaddressed_high_risk(RiskID, _).

# -----------------------------------------------------------------------------
# 42.6 Context Injection for Northstar
# -----------------------------------------------------------------------------

# Inject mission when planning or deciding actions
injectable_context(/northstar_mission, Mission) :-
    northstar_defined(),
    northstar_mission(_, Mission),
    active_shard(ShardID, _),
    shard_family(ShardID, /planner).

injectable_context(/northstar_mission, Mission) :-
    northstar_defined(),
    northstar_mission(_, Mission),
    active_shard(ShardID, _),
    shard_family(ShardID, /coder).

# Inject critical capabilities during planning
injectable_context(/critical_cap, Desc) :-
    northstar_defined(),
    critical_capability(CapID),
    northstar_capability(CapID, Desc, _, _),
    active_shard(ShardID, _),
    shard_family(ShardID, /planner).

# Inject unmitigated risks as warnings
injectable_context(/unmitigated_risk_warning, Desc) :-
    northstar_defined(),
    unmitigated_risk(RiskID),
    northstar_risk(RiskID, Desc, _, _).

# Inject constraints always
injectable_context(/constraint, Desc) :-
    northstar_defined(),
    northstar_constraint(_, Desc).
