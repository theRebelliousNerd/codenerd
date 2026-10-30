# Campaign Rules - Advanced Campaign Orchestration Logic
# Version: 2.0.0
# =============================================================================
# CAMPAIGN INTELLIGENCE LAYER
# This file contains advanced campaign execution rules that complement the
# base policy (policy.mg Section 19) and topology (build_topology.mg).
#
# Architecture:
#   policy.mg        → Base campaign state machine, task selection
#   build_topology.mg → Architectural layer enforcement
#   campaign_rules.mg → THIS FILE: Advanced scheduling, quality, learning
# =============================================================================

# =============================================================================
# SECTION 1: CAMPAIGN PLANNING INTELLIGENCE
# =============================================================================
# Rules for intelligent campaign creation and phase decomposition.

# -----------------------------------------------------------------------------
# 1.1 Goal Complexity Analysis
# -----------------------------------------------------------------------------

# Goal requires campaign (not single-shot) if it mentions multiple components
goal_requires_campaign(Goal) :-
    goal_topic_count(CampaignID, Count),
    Count >= 2,
    campaign_goal(CampaignID, Goal).

# Count goal topics per campaign
goal_topic_count(CampaignID, Count) :-
    goal_topic(CampaignID, _)
    |> do fn:group_by(CampaignID), let Count = fn:count().

# Goal requires campaign if it involves known complex patterns
goal_requires_campaign(Goal) :-
    campaign_goal(CampaignID, Goal),
    goal_topic(CampaignID, /migration).

goal_requires_campaign(Goal) :-
    campaign_goal(CampaignID, Goal),
    goal_topic(CampaignID, /refactor).

goal_requires_campaign(Goal) :-
    campaign_goal(CampaignID, Goal),
    goal_topic(CampaignID, /greenfield).

# Simple goals that DON'T need campaigns
simple_goal(Goal) :-
    campaign_goal(CampaignID, Goal),
    !goal_requires_campaign(Goal).

# Recommend downgrade to single-shot for simple goals
recommend_downgrade(CampaignID) :-
    campaign(CampaignID, _, _, _, /planning),
    campaign_goal(CampaignID, Goal),
    simple_goal(Goal).

# -----------------------------------------------------------------------------
# 1.2 Phase Count Heuristics
# -----------------------------------------------------------------------------

# Campaign seems too ambitious (> 6 phases)
campaign_too_ambitious(CampaignID) :-
    campaign_metadata(CampaignID, _, EstPhases, _),
    EstPhases > 6.

# Campaign seems trivial (< 2 phases for complex goal)
campaign_too_trivial(CampaignID) :-
    campaign_metadata(CampaignID, _, EstPhases, _),
    EstPhases < 2,
    campaign_goal(CampaignID, Goal),
    goal_requires_campaign(Goal).

# Warning for LLM to reconsider decomposition
decomposition_warning(CampaignID, "too_many_phases") :-
    campaign_too_ambitious(CampaignID).

decomposition_warning(CampaignID, "too_few_phases") :-
    campaign_too_trivial(CampaignID).

# -----------------------------------------------------------------------------
# 1.3 Confidence-Based Planning
# -----------------------------------------------------------------------------

# Low confidence plan needs user review
plan_needs_review(CampaignID) :-
    campaign_metadata(CampaignID, _, _, Confidence),
    Confidence < 70.

# High confidence allows auto-start
plan_can_autostart(CampaignID) :-
    campaign_metadata(CampaignID, _, _, Confidence),
    Confidence >= 85,
    !plan_needs_review(CampaignID).

# Trigger user clarification for low-confidence plans
next_action(/campaign_clarify) :-
    campaign(CampaignID, _, _, _, /planning),
    plan_needs_review(CampaignID).

# =============================================================================
# SECTION 2: INTELLIGENT TASK SCHEDULING
# =============================================================================
# Advanced task scheduling beyond basic priority ordering.

# -----------------------------------------------------------------------------
# 2.1 Parallel Task Detection
# -----------------------------------------------------------------------------

# Tasks are parallelizable if they don't share artifacts
tasks_parallelizable(TaskA, TaskB) :-
    campaign_task(TaskA, PhaseID, _, /pending, _),
    campaign_task(TaskB, PhaseID, _, /pending, _),
    TaskA != TaskB,
    !task_dependency(TaskA, TaskB),
    !task_dependency(TaskB, TaskA),
    !tasks_share_artifact(TaskA, TaskB).

# Helper: check if tasks touch the same file
tasks_share_artifact(TaskA, TaskB) :-
    task_artifact(TaskA, _, Path, _),
    task_artifact(TaskB, _, Path, _).

# Batch of parallelizable tasks (first 3)
parallel_batch_task(TaskID) :-
    eligible_task(TaskID),
    tasks_parallelizable(TaskID, _).

# Count parallelizable tasks (heuristic via multiple bindings)
has_parallel_opportunity(PhaseID) :-
    campaign_task(T1, PhaseID, _, /pending, _),
    campaign_task(T2, PhaseID, _, /pending, _),
    tasks_parallelizable(T1, T2).

# -----------------------------------------------------------------------------
# 2.2 Task Complexity Estimation
# -----------------------------------------------------------------------------

# High complexity tasks get extra context
task_is_complex(TaskID) :-
    phase_estimate(PhaseID, _, /high),
    campaign_task(TaskID, PhaseID, _, _, _).

task_is_complex(TaskID) :-
    phase_estimate(PhaseID, _, /critical),
    campaign_task(TaskID, PhaseID, _, _, _).

# Simple tasks can run with minimal context
task_is_simple(TaskID) :-
    phase_estimate(PhaseID, _, /low),
    campaign_task(TaskID, PhaseID, _, _, _).

# Which shard agent a campaign task needs is expert_needed and
# specialist_should_execute / specialist_should_advise
# (policy/shard_experts.mg, policy/shards.mg), asked per task through
# internal/experts by the orchestrator (internal/campaign/orchestrator_experts.go).
# The campaign's own rules for it (prefer_specialist_for_task here, a
# delegate_task in 5.3, the 8.2 override) joined shard_can_handle, which nothing
# produced, and were deleted on 2026-10-29.

# -----------------------------------------------------------------------------
# 2.3 Task Retry Strategy
# -----------------------------------------------------------------------------
# What a failed task does next is task_next_move (policy/campaign_decisions.mg),
# over typed attempt signals and the campaign's configured attempt cap. The
# rules that stood here are deleted: task_retry_exhausted said "exhausted" at a
# literal attempt 3 while the orchestrator retried to 4, and the enrichment
# strategy (task_needs_enrichment, specific_enrichment, enrichment_strategy)
# selected on task_error values nothing produced and was read by nothing.

# =============================================================================
# SECTION 3: QUALITY ENFORCEMENT
# =============================================================================
# Rules to ensure campaign outputs meet quality standards.

# -----------------------------------------------------------------------------
# 3.1 Task Output Verification
# -----------------------------------------------------------------------------

# Task output needs verification if it produces code
task_needs_verification(TaskID) :-
    campaign_task(TaskID, _, _, /completed, /file_create).

task_needs_verification(TaskID) :-
    campaign_task(TaskID, _, _, /completed, /file_modify).

task_needs_verification(TaskID) :-
    campaign_task(TaskID, _, _, /completed, /test_write).

# Task passes verification if checkpoint succeeded
task_verified(TaskID) :-
    task_needs_verification(TaskID),
    campaign_task(TaskID, PhaseID, _, _, _),
    phase_checkpoint(PhaseID, _, /true, _, _).

# Unverified completed task (quality risk)
task_unverified(TaskID) :-
    task_needs_verification(TaskID),
    !task_verified(TaskID).

# Block phase completion if unverified tasks exist
has_unverified_task(PhaseID) :-
    campaign_task(TaskID, PhaseID, _, /completed, _),
    task_unverified(TaskID).

phase_blocked(PhaseID, /unverified_tasks) :-
    has_unverified_task(PhaseID).

# -----------------------------------------------------------------------------
# 3.2 Code Quality Gates
# -----------------------------------------------------------------------------

# What a completed task's files measurably carry. These read the plan's
# artifacts and the world model (placeholder_site, detected_security_flow,
# unchecked_error, test_file_for), so they run in the world shard; a task that is
# itself a remediation is never judged again, so a fix that falls short is
# reported, not re-spawned forever.
Decl quality_is_remediation(TaskID) bound [/string].
quality_is_remediation(TaskID) :-
    task_remediation_target(TaskID, _, _).

Decl quality_has_test(Path) bound [/string].
quality_has_test(Path) :- test_file_for(_, Path).

quality_violation(TaskID, /placeholder) :-
    task_artifact(TaskID, /source_file, Path, _),
    placeholder_site(Path, _, _),
    !quality_is_remediation(TaskID).

quality_violation(TaskID, /hardcoded) :-
    task_artifact(TaskID, /source_file, Path, _),
    detected_security_flow(Path, _, /hardcoded_secret, _, _),
    !quality_is_remediation(TaskID).

quality_violation(TaskID, /missing_errors) :-
    task_artifact(TaskID, /source_file, Path, _),
    unchecked_error(Path, _, _),
    !quality_is_remediation(TaskID).

quality_violation(TaskID, /no_tests) :-
    task_artifact(TaskID, /source_file, Path, _),
    file_topology(Path, _, /go, _, /false),
    !quality_has_test(Path),
    !quality_is_remediation(TaskID).

quality_violation_detected(TaskID, ViolationType) :-
    quality_violation(TaskID, ViolationType).

# Task with quality violations needs remediation
task_needs_remediation(TaskID) :-
    quality_violation_detected(TaskID, _).

# A remediation is owed once per task and violation; the orchestrator appends
# the task and records what it targets, which retires the debt here.
remediation_task_needed(TaskID, ViolationType) :-
    quality_violation_detected(TaskID, ViolationType),
    !remediation_task_exists(TaskID, ViolationType).

Decl remediation_task_exists(OriginalTaskID, ViolationType) bound [/string, /name].
remediation_task_exists(OriginalTaskID, ViolationType) :-
    task_remediation_target(_, OriginalTaskID, ViolationType).

# -----------------------------------------------------------------------------
# 3.3 Build & Test Gates
# -----------------------------------------------------------------------------

# Phase cannot complete without passing build
phase_requires_build_pass(PhaseID) :-
    campaign_phase(PhaseID, _, _, _, _, _),
    phase_objective(PhaseID, /create, _, /builds).

phase_requires_build_pass(PhaseID) :-
    campaign_phase(PhaseID, _, _, _, _, _),
    phase_objective(PhaseID, /modify, _, /builds).

# Phase build check failed
phase_build_failed(PhaseID) :-
    phase_requires_build_pass(PhaseID),
    phase_checkpoint(PhaseID, /build, /false, _, _).

# Block phase on build failure
phase_blocked(PhaseID, /build_failed) :-
    phase_build_failed(PhaseID).

# Phase test check failed
phase_tests_failed(PhaseID) :-
    phase_checkpoint(PhaseID, /tests, /false, _, _).

phase_blocked(PhaseID, /tests_failed) :-
    phase_tests_failed(PhaseID).

# =============================================================================
# SECTION 4: CONTEXT BUDGET MANAGEMENT
# =============================================================================
# Rules for managing LLM context window during campaigns.

# -----------------------------------------------------------------------------
# 4.1 Context Pressure Detection
# -----------------------------------------------------------------------------

# Context pressure is a measurement judged here. The context pager reports the
# tokens its phase context uses and the budget (campaign_context_use); the
# thresholds are policy facts; integer cross-multiplication does the percentage
# (Used*100 >= Budget*Percent), so no ratio is computed in Go and no virtual
# predicate stands in for one. A level at or above a threshold is reached, so a
# critical campaign is also high.
Decl campaign_context_threshold(Level, Percent) bound [/name, /number].
campaign_context_threshold(/high, 80).
campaign_context_threshold(/critical, 95).

context_pressure_level(CampaignID, Level) :-
    campaign_context_use(CampaignID, Used, Budget),
    Budget > 0,
    campaign_context_threshold(Level, Percent),
    Scaled = fn:mult(Used, 100),
    Floor = fn:mult(Budget, Percent),
    Scaled >= Floor.

context_pressure_high(CampaignID) :-
    context_pressure_level(CampaignID, /high).

context_pressure_critical(CampaignID) :-
    context_pressure_level(CampaignID, /critical).

# What the pager does about it: how many upcoming tasks it prefetches context
# for. Prefetching spends the budget the running phase is already short of.
# (The orchestrator reads this instead of a constant.)
Decl campaign_prefetch_limit(CampaignID, Tasks) bound [/string, /number].
campaign_prefetch_limit(CampaignID, 0) :-
    context_pressure_critical(CampaignID).
campaign_prefetch_limit(CampaignID, 1) :-
    context_pressure_high(CampaignID),
    !context_pressure_critical(CampaignID).
campaign_prefetch_limit(CampaignID, 3) :-
    campaign_context_use(CampaignID, _, _),
    !context_pressure_high(CampaignID).

# -----------------------------------------------------------------------------
# 4.2 Phase Context Isolation
# -----------------------------------------------------------------------------

# Helper to identify phases that already have compression
has_compression(PhaseID) :-
    context_compression(PhaseID, _, _, _).

# Context atoms from completed phases should be compressed
phase_context_stale(PhaseID) :-
    campaign_phase(PhaseID, _, _, _, /completed, _),
    !has_compression(PhaseID).

# Trigger compression for stale phases
should_compress_phase(PhaseID) :-
    phase_context_stale(PhaseID).

# Priority: compress oldest completed phases first
compress_priority(PhaseID, Order) :-
    phase_context_stale(PhaseID),
    campaign_phase(PhaseID, _, _, Order, /completed, _).

# -----------------------------------------------------------------------------
# 4.3 Focus Amplification
# -----------------------------------------------------------------------------

# Current task artifacts get maximum activation
activation(Path, 200) :-
    next_campaign_task(TaskID),
    task_artifact(TaskID, _, Path, _).

# Current phase context atoms get high activation
activation(Atom, 150) :-
    current_phase(PhaseID),
    phase_context_atom(PhaseID, Atom, _).

# Previous phase summaries get moderate activation
# NOTE: Mangle can't compute CurrentOrder - 1; simplified to all completed phases
# in same campaign get activation (Go can refine with previous_phase predicate)
activation(Summary, 80) :-
    current_phase(CurrentPhaseID),
    campaign_phase(CurrentPhaseID, CampaignID, _, _, _, _),
    campaign_phase(PhaseID, CampaignID, _, _, /completed, _),
    PhaseID != CurrentPhaseID,
    context_compression(PhaseID, Summary, _, _).

# =============================================================================
# SECTION 5: CROSS-PHASE LEARNING
# =============================================================================
# Rules for learning patterns across campaign execution.

# -----------------------------------------------------------------------------
# 5.1 Success Pattern Detection
# -----------------------------------------------------------------------------

# Phase had no failures (success pattern)
phase_zero_failures(PhaseID) :-
    campaign_phase(PhaseID, _, _, _, /completed, _),
    !phase_had_failure(PhaseID).

# Helper: check if phase had any task failures
phase_had_failure(PhaseID) :-
    campaign_task(_, PhaseID, _, /failed, _).

# Learn from zero-failure phases
promote_to_long_term(/phase_pattern, PhaseType) :-
    phase_zero_failures(PhaseID),
    phase_objective(PhaseID, PhaseType, _, _).

# -----------------------------------------------------------------------------
# 5.2 Failure Pattern Detection
# -----------------------------------------------------------------------------

# Same error type across multiple tasks (systemic issue)
campaign_task_error(CampaignID, ErrorType) :-
    task_error(TaskID, ErrorType, _),
    campaign_task(TaskID, PhaseID, _, /failed, _),
    campaign_phase(PhaseID, CampaignID, _, _, _, _).

campaign_task_error_count(CampaignID, ErrorType, Count) :-
    campaign_task_error(CampaignID, ErrorType)
    |> do fn:group_by(CampaignID, ErrorType), let Count = fn:count().

systemic_error_detected(CampaignID, ErrorType) :-
    campaign_task_error_count(CampaignID, ErrorType, Count),
    Count >= 3.

# Systemic error should trigger investigation
next_action(/investigate_systemic) :-
    current_campaign(CampaignID),
    systemic_error_detected(CampaignID, ErrorType),
    !systemic_error_investigated(CampaignID, ErrorType).

# Helper for safe negation
systemic_error_investigated(CampaignID, ErrorType) :-
    campaign_learning(CampaignID, /failure_pattern, ErrorType, _, _).

# Learn to avoid systemic errors
learning_signal(/avoid_pattern, ErrorType) :-
    systemic_error_detected(_, ErrorType).

# -----------------------------------------------------------------------------
# 5.3 Shard Performance in Campaigns
# -----------------------------------------------------------------------------

# The reliability rules that stood here (shard_campaign_success, _failure,
# _reliable, shard_has_many_failures) fed one consumer, a delegate_task that
# joined shard_can_handle, which nothing produced. All were deleted on
# 2026-10-29; see 2.2 for where a task's shard agent comes from.

# =============================================================================
# SECTION 6: RESILIENCE & RECOVERY
# =============================================================================
# Rules for handling failures and recovering campaigns.

# -----------------------------------------------------------------------------
# 6.1 Cascading Failure Detection
# -----------------------------------------------------------------------------

# Multiple consecutive task failures in same phase
phase_failure_cascade(PhaseID) :-
    phase_failed_task_count(PhaseID, Count),
    Count >= 3.

# Count failed tasks per phase
phase_failed_task_count(PhaseID, Count) :-
    campaign_task(_, PhaseID, _, /failed, _)
    |> do fn:group_by(PhaseID), let Count = fn:count().

# Cascade triggers phase pause
phase_blocked(PhaseID, /failure_cascade) :-
    phase_failure_cascade(PhaseID).

# Cascade triggers replan consideration
replan_needed(CampaignID, /phase_failure_cascade) :-
    current_phase(PhaseID),
    phase_failure_cascade(PhaseID),
    campaign_phase(PhaseID, CampaignID, _, _, _, _),
    current_campaign(CampaignID).

# -----------------------------------------------------------------------------
# 6.2 Stuck Detection
# -----------------------------------------------------------------------------

# Phase is stuck: in-progress but no runnable tasks
phase_stuck(PhaseID) :-
    current_phase(PhaseID),
    !has_runnable_task(PhaseID),
    campaign_phase(PhaseID, _, _, _, /in_progress, _),
    has_pending_tasks(PhaseID).

# Helper: phase has runnable tasks
has_runnable_task(PhaseID) :-
    eligible_task(TaskID),
    campaign_task(TaskID, PhaseID, _, _, _).

# Helper: phase has pending tasks
has_pending_tasks(PhaseID) :-
    campaign_task(_, PhaseID, _, /pending, _).

# Helper: phase has running tasks
has_running_tasks(PhaseID) :-
    campaign_task(_, PhaseID, _, /in_progress, _).

# Stuck phase needs intervention
escalation_needed(/campaign, PhaseID, "phase_stuck") :-
    phase_stuck(PhaseID).

# Debug helper: why is task blocked?
debug_why_blocked(TaskID, DepID) :-
    task_dependency(TaskID, DepID),
    campaign_task(DepID, _, _, Status, _),
    /completed != Status,
    /skipped != Status.

# -----------------------------------------------------------------------------
# 6.3 Recovery Actions
# -----------------------------------------------------------------------------

# Skip a non-critical task that has used its attempts (task_exhausted)
task_can_skip(TaskID) :-
    task_exhausted(TaskID),
    task_priority(TaskID, /low).

task_can_skip(TaskID) :-
    task_exhausted(TaskID),
    !task_blocks_others(TaskID),
    task_priority(TaskID, /normal).

# Helper: task blocks other tasks
task_blocks_others(TaskID) :-
    task_dependency(_, TaskID).

# Critical task failure requires escalation
escalation_required(TaskID, "critical_task_failed") :-
    task_exhausted(TaskID),
    task_priority(TaskID, /critical).

escalation_required(TaskID, "high_priority_failed") :-
    task_exhausted(TaskID),
    task_priority(TaskID, /high),
    task_blocks_others(TaskID).

# An escalation is put to the user. Until 2026-09-23 this rule sat in
# policy/verification.mg, whose own verification-loop rules never fired
# (nothing asserted their inputs); it is the one rule of that file a live
# input reached, and moved here with the escalations it serves.
next_action(/escalate_to_user) :-
    escalation_required(_, _).

# Auto-skip suggestion for non-blocking failed tasks
suggest_skip_task(TaskID) :-
    task_can_skip(TaskID),
    !task_blocks_others(TaskID).

# =============================================================================
# SECTION 7: MILESTONE & PROGRESS TRACKING
# =============================================================================
# Rules for tracking campaign progress and milestones.

# -----------------------------------------------------------------------------
# 7.1 Progress Calculation Helpers
# -----------------------------------------------------------------------------

# Phase is partially complete
phase_partial_complete(PhaseID) :-
    campaign_phase(PhaseID, _, _, _, /in_progress, _),
    campaign_task(_, PhaseID, _, /completed, _).

# Phase has high completion rate (75%+)
phase_nearly_complete(PhaseID) :-
    campaign_phase(PhaseID, _, _, _, /in_progress, _),
    all_phase_tasks_complete(PhaseID).

# Campaign is past halfway point: at least half its phases are done
# (Done*2 >= Total, integer arithmetic; no Go-computed flag).
campaign_past_halfway(CampaignID) :-
    campaign_progress(CampaignID, Done, Total, _, _),
    Total > 0,
    Twice = fn:mult(Done, 2),
    Twice >= Total.

# -----------------------------------------------------------------------------
# 7.2 Milestone Detection
# -----------------------------------------------------------------------------

# First phase complete = "foundation" milestone
milestone_reached(CampaignID, /foundation) :-
    campaign_phase(PhaseID, CampaignID, _, 1, /completed, _).

# All phases complete = "completion" milestone
milestone_reached(CampaignID, /completion) :-
    campaign_complete(CampaignID).

# Halfway milestone
milestone_reached(CampaignID, /halfway) :-
    campaign_past_halfway(CampaignID).

# Integration phase complete = "integrated" milestone
milestone_reached(CampaignID, /integrated) :-
    campaign_phase(PhaseID, CampaignID, _, _, /completed, _),
    phase_category(PhaseID, /integration).

# A milestone is announced once: the orchestrator emits an event for each
# pending one and records it as campaign_milestone, which retires it here.
Decl milestone_recorded(CampaignID, Milestone) bound [/string, /name].
milestone_recorded(CampaignID, Milestone) :-
    campaign_milestone(CampaignID, Milestone, _, _).

Decl milestone_pending(CampaignID, Milestone) bound [/string, /name].
milestone_pending(CampaignID, Milestone) :-
    milestone_reached(CampaignID, Milestone),
    !milestone_recorded(CampaignID, Milestone).

# -----------------------------------------------------------------------------
# 7.3 Progress Events
# -----------------------------------------------------------------------------

# Trigger progress update on task completion
progress_changed(CampaignID) :-
    campaign_task(_, PhaseID, _, /completed, _),
    campaign_phase(PhaseID, CampaignID, _, _, _, _),
    current_campaign(CampaignID).

progress_changed(CampaignID) :-
    campaign_phase(_, CampaignID, _, _, /completed, _),
    current_campaign(CampaignID).

# =============================================================================
# SECTION 8: SHARD SELECTION FOR CAMPAIGNS
# =============================================================================
# Rules for choosing the right shard for campaign tasks.

# -----------------------------------------------------------------------------
# 8.1 Task Type to Persona
# -----------------------------------------------------------------------------
# One table: the persona a task type runs as. task_persona and task_delegation
# (8.3, the verb a task's turn runs) read it. It replaced nine rules of this section
# and the orchestrator's own copy, inferShardFromTaskType, which nothing called.

task_type_persona(/file_create, /coder).
task_type_persona(/file_modify, /coder).
task_type_persona(/test_write, /tester).
task_type_persona(/test_run, /tester).
task_type_persona(/research, /researcher).
task_type_persona(/verify, /reviewer).
# Refactoring with reviewer support; integration and documentation.
task_type_persona(/refactor, /coder).
task_type_persona(/integrate, /coder).
task_type_persona(/document, /coder).
task_type_persona(/shard_spawn, /coder).

# -----------------------------------------------------------------------------
# 8.2 Shard agents
# -----------------------------------------------------------------------------
# A task's shard agent is the kernel's expert decision (see 2.2). The override
# that stood here (campaign_task_shard_override, task_has_shard_override,
# final_shard_for_task and its default, campaign_task_shard) joined
# shard_can_handle, which nothing produced, and no consumer read its answer.

# -----------------------------------------------------------------------------
# 8.3 The verb a task's turn runs (sweep finding F6)
# -----------------------------------------------------------------------------
# The orchestrator's handlers named the verb themselves: every file, refactor,
# shard-spawn and generic task ran as /fix, and a .md deliverable ran as
# "/fix create file:..." with /fix's atoms. The verb is the task type's
# persona through the one persona table (persona_verb, policy/delegation.mg),
# except that a new document is created, not fixed. A task type with no
# persona runs as the coder, as the generic handler always did. The rules do
# not read the status: the handler asks while the task is in progress.

task_type_known(TaskType) :-
    task_type_persona(TaskType, Persona).

task_persona(TaskID, Persona) :-
    campaign_task(TaskID, _, _, _, TaskType),
    task_type_persona(TaskType, Persona).

task_persona(TaskID, /coder) :-
    campaign_task(TaskID, _, _, _, TaskType),
    !task_type_known(TaskType).

task_creates_doc(TaskID) :-
    campaign_task(TaskID, _, _, _, /file_create),
    task_write_ext(TaskID, Ext),
    write_class(Ext, /doc).

task_delegation(TaskID, /create) :-
    task_creates_doc(TaskID).

task_delegation(TaskID, Verb) :-
    task_persona(TaskID, Persona),
    persona_verb(Persona, Verb),
    !task_creates_doc(TaskID).

# =============================================================================
# SECTION 9: CAMPAIGN INTENT HANDLING
# =============================================================================
# Rules for handling user intents during active campaigns.

# -----------------------------------------------------------------------------
# 9.1 Intent Interruption
# -----------------------------------------------------------------------------

# User mutation intent during campaign needs routing decision
campaign_intent_conflict(IntentID) :-
    user_intent(IntentID, /mutation, _, _, _),
    !intent_is_campaign_related(IntentID),
    current_campaign(_).

# Intent is campaign-related if it matches current phase
intent_is_campaign_related(IntentID) :-
    current_phase(PhaseID),
    phase_objective(PhaseID, _, _, _),
    campaign_phase(PhaseID, CampaignID, _, _, _, _),
    current_campaign(CampaignID),
    user_intent(IntentID, _, _, _, _).

# Route conflict to user for decision
next_action(/ask_campaign_interrupt) :-
    campaign_intent_conflict(_).

# -----------------------------------------------------------------------------
# 9.2 Campaign Queries
# -----------------------------------------------------------------------------

# Status query during campaign
# Target is user_intent's /string slot — it normally holds a path or a noun
# phrase lifted from the user's words. These two rules were the only place in
# the corpus that matched it with a name constant, so they could never fire; the
# sub-command rules in policy/capabilities.mg use the quoted form throughout.
next_action(/show_campaign_status) :-
    user_intent(/current_intent, /query, _, "status", _),
    current_campaign(_).

# Progress query during campaign
next_action(/show_campaign_progress) :-
    user_intent(/current_intent, /query, _, "progress", _),
    current_campaign(_).

# =============================================================================
# SECTION 10: CAMPAIGN COMPLETION & CLEANUP
# =============================================================================
# Rules for finalizing and learning from completed campaigns.

# -----------------------------------------------------------------------------
# 10.1 Completion Verification
# -----------------------------------------------------------------------------

# Campaign ready for completion verification
campaign_completion_ready(CampaignID) :-
    current_campaign(CampaignID),
    !has_incomplete_phase(CampaignID),
    !has_unverified_task_in_campaign(CampaignID).

# Helper: any unverified task in campaign
has_unverified_task_in_campaign(CampaignID) :-
    campaign_phase(PhaseID, CampaignID, _, _, _, _),
    has_unverified_task(PhaseID).

# Final verification action
next_action(/campaign_final_verify) :-
    campaign_completion_ready(_).

# -----------------------------------------------------------------------------
# 10.2 Post-Campaign Learning
# -----------------------------------------------------------------------------

# Extract success patterns from completed campaign
campaign_success_pattern(CampaignID, Pattern) :-
    campaign_metadata(CampaignID, _, _, Confidence),
    Confidence > 80,
    campaign(CampaignID, Type, _, _, /completed),
    Pattern = Type.

# Promote campaign success to long-term memory
promote_to_long_term(/campaign_success, Pattern) :-
    campaign_success_pattern(_, Pattern).

# Track campaign type effectiveness
campaign_type_effective(Type) :-
    campaign_type_count(Type, Count),
    Count >= 2.

campaign_type_count(Type, Count) :-
    campaign(_, Type, _, _, /completed)
    |> do fn:group_by(Type), let Count = fn:count().

# Learn from campaign failures
campaign_failure_pattern(CampaignID, Reason) :-
    campaign(CampaignID, _, _, _, /failed),
    campaign_blocked(CampaignID, Reason).

learning_signal(/campaign_failure, Reason) :-
    campaign_failure_pattern(_, Reason).

# -----------------------------------------------------------------------------
# 10.3 Cleanup Actions
# -----------------------------------------------------------------------------

# Compress all context after campaign completion
next_action(/campaign_cleanup) :-
    campaign(CampaignID, _, _, _, /completed),
    !campaign_cleaned(CampaignID).

# Helper for safe negation. The cleanup handler
# (VirtualStore.handleCampaignCleanup) asserts campaign_cleaned_up when it
# has run. This read campaign_learning(_, /cleanup, ...), which nothing
# produces, so the cleanup action derived again after it had run.
campaign_cleaned(CampaignID) :-
    campaign_cleaned_up(CampaignID).

# Archive campaign artifacts
next_action(/archive_campaign) :-
    campaign(CampaignID, _, _, _, /completed),
    campaign_cleaned(CampaignID).

# =============================================================================
# SECTION 11: OBSERVABILITY & DEBUGGING
# =============================================================================
# Rules for campaign monitoring and debugging.

# -----------------------------------------------------------------------------
# 11.1 Health Indicators
# -----------------------------------------------------------------------------

campaign_is_blocked(CampaignID) :-
    campaign_blocked(CampaignID, Reason).

any_phase_stuck(/yes) :-
    phase_stuck(PhaseID).

# Campaign is healthy (making progress).
# Both `!campaign_blocked(CampaignID, _)` and `!phase_stuck(_)` excluded
# nothing, so a blocked campaign with a stuck phase still derived healthy —
# and campaign_has_issues, its negation, could never fire.
campaign_healthy(CampaignID) :-
    current_campaign(CampaignID),
    !campaign_is_blocked(CampaignID),
    !any_phase_stuck(/yes),
    !context_pressure_critical(CampaignID).

# Campaign has issues
campaign_has_issues(CampaignID) :-
    current_campaign(CampaignID),
    !campaign_healthy(CampaignID).

# -----------------------------------------------------------------------------
# 11.2 Diagnostic Queries
# -----------------------------------------------------------------------------

# Why is campaign blocked? (for debugging)
campaign_block_reason(CampaignID, Reason) :-
    campaign_blocked(CampaignID, Reason).

# Why is phase blocked? (for debugging)
phase_block_reason(PhaseID, Reason) :-
    phase_blocked(PhaseID, Reason).

# Task failure summary (for debugging)
task_failure_summary(TaskID, ErrorType, ErrorMsg) :-
    campaign_task(TaskID, _, _, /failed, _),
    task_error(TaskID, ErrorType, ErrorMsg).

# -----------------------------------------------------------------------------
# 11.3 Metrics Derivation
# -----------------------------------------------------------------------------

# Helper for task counts (simplified - exact counting in Go)
campaign_has_tasks(CampaignID) :-
    campaign_phase(PhaseID, CampaignID, _, _, _, _),
    campaign_task(_, PhaseID, _, _, _).

# Phases per campaign (simplified heuristic)
campaign_has_multiple_phases(CampaignID) :-
    campaign_phase_count(CampaignID, Count),
    Count >= 2.

campaign_phase_count(CampaignID, Count) :-
    campaign_phase(_, CampaignID, _, _, _, _)
    |> do fn:group_by(CampaignID), let Count = fn:count().

# =============================================================================
# SECTION 12: INTEGRATION WITH BUILD TOPOLOGY
# =============================================================================
# Rules that connect campaign execution with build topology validation.

# -----------------------------------------------------------------------------
# 12.1 Topology Violation Handling
# -----------------------------------------------------------------------------

# Block task if it would create architectural violation
task_blocked_by_topology(TaskID) :-
    campaign_task(TaskID, PhaseID, _, /pending, _),
    architectural_violation(PhaseID, _, _).

# Task warning for suspicious gaps
task_topology_warning(TaskID, "skips_layer") :-
    campaign_task(TaskID, PhaseID, _, /pending, _),
    suspicious_gap(PhaseID, _).

# Block campaign start if topology violations exist
campaign_blocked(CampaignID, /topology_violations) :-
    campaign(CampaignID, _, _, _, /validating),
    campaign_phase(PhaseID, CampaignID, _, _, _, _),
    architectural_violation(PhaseID, _, _).

# -----------------------------------------------------------------------------
# 12.2 Layer-Aware Scheduling
# -----------------------------------------------------------------------------

# Prefer lower-layer phases first (enforces build order)
phase_layer_priority(PhaseID, Priority) :-
    phase_precedence(PhaseID, Priority).

# Earlier layers should complete before later layers start
layer_sequencing_correct(PhaseA, PhaseB) :-
    phase_layer_priority(PhaseA, PriorityA),
    phase_dependency(PhaseB, PhaseA, _),
    phase_layer_priority(PhaseB, PriorityB),
    PriorityA < PriorityB.

# Warning if layer sequencing is wrong
layer_sequencing_warning(PhaseA, PhaseB) :-
    phase_layer_priority(PhaseA, PriorityA),
    phase_dependency(PhaseB, PhaseA, _),
    phase_layer_priority(PhaseB, PriorityB),
    PriorityA >= PriorityB.

# =============================================================================
# SECTION 13: RISK PREFLIGHT CLASSIFICATION (hard vs soft advisory contract)
# =============================================================================
# Go measures; the kernel decides. internal/campaign/risk_scoring.go runs the
# strict gates, asserts what they observed, and then asks THIS section whether
# any of it stops the campaign. Go never decides that on its own: it enforces
# campaign_risk_block and surfaces campaign_risk_warning.
#
# The distinction exists because "an advisor is unhappy" and "an advisor
# rejected work on the logic kernel" are not the same event, and treating them
# identically means operators learn to ignore both.

# Readiness canary. A kernel booted without campaign_rules.mg derives no blocks
# at all, which would read as "everything passed". Go checks this predicate to
# tell "no blocks" apart from "no rules", and falls back to its own mirror of
# this contract when the rules are absent.
campaign_risk_classification_ready(CampaignID) :-
    campaign_risk_gate_outcome(CampaignID, _, _).

# Safety-critical evidence escalates any blocked gate to a hard stop.
campaign_risk_critical_signal(CampaignID) :-
    campaign_risk_signal(CampaignID, /safety_warnings, Count),
    Count > 0.

campaign_risk_critical_signal(CampaignID) :-
    campaign_risk_signal(CampaignID, /blocked_actions, Count),
    Count > 0.

# HARD 1: any blocked gate while the campaign targets a protected surface
# (kernel, mangle, campaign, perception, articulation). These are the surfaces
# that can disable the safety machinery itself, so advice is not optional there.
campaign_risk_block(CampaignID, Gate, /protected_surface) :-
    campaign_risk_gate_outcome(CampaignID, Gate, /blocked),
    campaign_protected_surface(CampaignID, _).

# HARD 1b: a gate that ran but produced no verdict (/inconclusive: e.g. the
# northstar alignment check had no LLM or got a malformed answer) on a
# protected surface. "Nobody assessed this" is not "assessed and fine" where
# the change can disable the safety machinery itself.
campaign_risk_block(CampaignID, Gate, /protected_surface_unassessed) :-
    campaign_risk_gate_outcome(CampaignID, Gate, /inconclusive),
    campaign_protected_surface(CampaignID, _).

# HARD 2: northstar alignment is constitutional on every surface.
campaign_risk_block(CampaignID, /northstar, /vision_alignment) :-
    campaign_risk_gate_outcome(CampaignID, /northstar, /blocked).

# HARD 3: a critical advisor voting REJECT is a blocking concern, not a note.
campaign_risk_block(CampaignID, Gate, /critical_advisor_rejection) :-
    campaign_risk_concern(CampaignID, Gate, /blocking).

# HARD 4: explicit operator force-block.
campaign_risk_block(CampaignID, /override, /force_block) :-
    campaign_risk_override(CampaignID, /force_block).

# HARD 5: a blocked gate on a campaign already over the deterministic risk
# threshold AND carrying critical safety signals.
campaign_risk_block(CampaignID, Gate, /gated_with_critical_signals) :-
    campaign_risk_gate_outcome(CampaignID, Gate, /blocked),
    campaign_risk_posture(CampaignID, _, _, /true),
    campaign_risk_critical_signal(CampaignID).

# Fully-bound helper for safe negation. Negating campaign_risk_block directly
# with a wildcard reason (`!campaign_risk_block(C, G, _)`) does not exclude the
# blocked gate — the wildcard slot leaves the literal unbound rather than
# existentially quantified, so the soft rules fired alongside every hard one.
campaign_risk_blocked_gate(CampaignID, Gate) :-
    campaign_risk_block(CampaignID, Gate, _).

# SOFT: a blocked gate no hard rule claimed is advice. It is recorded and shown
# to the operator; the campaign proceeds.
campaign_risk_warning(CampaignID, Gate, /advisory_only) :-
    campaign_risk_gate_outcome(CampaignID, Gate, /blocked),
    !campaign_risk_blocked_gate(CampaignID, Gate).

# SOFT: an inconclusive gate on an ordinary surface is surfaced, never passed.
campaign_risk_warning(CampaignID, Gate, /inconclusive) :-
    campaign_risk_gate_outcome(CampaignID, Gate, /inconclusive),
    !campaign_risk_blocked_gate(CampaignID, Gate).

# SOFT: requested changes are advice unless a hard rule fired on the same gate.
campaign_risk_warning(CampaignID, Gate, /requires_changes) :-
    campaign_risk_concern(CampaignID, Gate, /requires_changes),
    !campaign_risk_blocked_gate(CampaignID, Gate).

# SOFT: an unapproved-but-not-rejected plan is advice on ordinary surfaces.
campaign_risk_warning(CampaignID, Gate, /unapproved) :-
    campaign_risk_concern(CampaignID, Gate, /unapproved),
    !campaign_risk_blocked_gate(CampaignID, Gate).

campaign_risk_preflight_blocked(CampaignID) :-
    campaign_risk_block(CampaignID, _, _).

# =============================================================================
# SECTION 14: RECOVERY ADMISSION (which durable generation may resume)
# =============================================================================
# Go observes storage; the kernel decides. internal/campaign/recovery_admission.go
# compares the snapshot (or a replacement candidate left by an interrupted
# rename) with the journal's last requested and last committed checksums and
# asserts ONE campaign_recovery_class. Parsing as JSON is not a verdict: only a
# provenance derived here lets a resume hydrate the kernel or schedule work.
#
# Provenance names how the generation earned trust:
#   /committed               the journal's last commit vouches for these bytes
#   /reconciled_publication  a verified snapshot was renamed into place but the
#                            process died before its commit record
#   /previous_generation     the last requested write never published; the
#                            previous committed generation is intact
#   /legacy_import           no journal ever existed: a new provenance boundary,
#                            not manufactured prior verification
#   /operator_adopted        the operator explicitly adopted a snapshot the
#                            journal does not vouch for (--adopt-snapshot)
# /storage_unreadable is never admitted, adoption or not: nothing was observed.

campaign_recovery_admitted(CampaignID, /committed) :-
    campaign_recovery_class(CampaignID, /committed_match).

campaign_recovery_admitted(CampaignID, /reconciled_publication) :-
    campaign_recovery_class(CampaignID, /prepared_snapshot_present).

campaign_recovery_admitted(CampaignID, /previous_generation) :-
    campaign_recovery_class(CampaignID, /previous_generation_present).

campaign_recovery_admitted(CampaignID, /legacy_import) :-
    campaign_recovery_class(CampaignID, /legacy_unjournaled).

campaign_recovery_admitted(CampaignID, /operator_adopted) :-
    campaign_recovery_class(CampaignID, /conflict_or_corruption),
    campaign_recovery_adopt(CampaignID).

# Fully-bound helper for safe negation (see campaign_risk_blocked_gate).
campaign_recovery_decided(CampaignID) :-
    campaign_recovery_admitted(CampaignID, _).

# Every observed class no admission rule claimed is refused, by name, so Go can
# tell "refused" apart from "rules not loaded" (neither predicate derives).
campaign_recovery_refused(CampaignID, Class) :-
    campaign_recovery_class(CampaignID, Class),
    !campaign_recovery_decided(CampaignID).

# =============================================================================
# END OF CAMPAIGN RULES
# =============================================================================
