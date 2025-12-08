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
    campaign_goal(CampaignID, Goal),
    goal_topic(CampaignID, Topic1),
    goal_topic(CampaignID, Topic2),
    Topic1 != Topic2.

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
    goal_requires_campaign(_).

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
    T1 != T2,
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

# Complex tasks should spawn specialist shards if available
prefer_specialist_for_task(TaskID, SpecialistName) :-
    task_is_complex(TaskID),
    campaign_task(TaskID, _, Description, _, TaskType),
    shard_profile(SpecialistName, /specialist, _),
    shard_can_handle(SpecialistName, TaskType).

# -----------------------------------------------------------------------------
# 2.3 Task Retry Strategy
# -----------------------------------------------------------------------------

# Task has exhausted basic retries
task_retry_exhausted(TaskID) :-
    task_attempt(TaskID, 3, /failure, _).

# Task should try with enriched context
task_needs_enrichment(TaskID) :-
    task_attempt(TaskID, AttemptNum, /failure, _),
    AttemptNum >= 1,
    AttemptNum < 3,
    !task_retry_exhausted(TaskID).

# Specific enrichment strategies based on failure type (computed in stratum 0)
# Note: /research for /unknown_api is a specific strategy, not the default
specific_enrichment(TaskID, /research) :-
    task_needs_enrichment(TaskID),
    task_error(TaskID, /unknown_api, _).

specific_enrichment(TaskID, /documentation) :-
    task_needs_enrichment(TaskID),
    task_error(TaskID, /missing_context, _).

specific_enrichment(TaskID, /decompose) :-
    task_needs_enrichment(TaskID),
    task_error(TaskID, /too_complex, _).

specific_enrichment(TaskID, /specialist) :-
    task_needs_enrichment(TaskID),
    task_error(TaskID, /domain_specific, _).

# Helper: task has a specific enrichment strategy (for safe negation in stratum 1)
has_specific_enrichment(TaskID) :-
    specific_enrichment(TaskID, _).

# Final enrichment_strategy: either specific or default to /research (stratum 1)
enrichment_strategy(TaskID, Strategy) :-
    specific_enrichment(TaskID, Strategy).

enrichment_strategy(TaskID, /research) :-
    task_needs_enrichment(TaskID),
    !has_specific_enrichment(TaskID).

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
    phase_checkpoint(PhaseID, _, /true, _, _),
    campaign_task(TaskID, PhaseID, _, _, _).

# Unverified completed task (quality risk)
task_unverified(TaskID) :-
    task_needs_verification(TaskID),
    !task_verified(TaskID).

# Block phase completion if unverified tasks exist
has_unverified_task(PhaseID) :-
    campaign_task(TaskID, PhaseID, _, /completed, _),
    task_unverified(TaskID).

phase_blocked(PhaseID, "unverified_tasks") :-
    has_unverified_task(PhaseID).

# -----------------------------------------------------------------------------
# 3.2 Code Quality Gates
# -----------------------------------------------------------------------------

# Quality violation patterns (detected by shards)
quality_violation_detected(TaskID, /no_error_handling) :-
    campaign_task(TaskID, _, _, /completed, /file_create),
    task_artifact(TaskID, /source_file, Path, _),
    file_topology(Path, _, /go, _, _),
    quality_violation(TaskID, /missing_errors).

quality_violation_detected(TaskID, /no_tests) :-
    campaign_task(TaskID, _, _, /completed, /file_create),
    task_artifact(TaskID, /source_file, Path, _),
    !test_coverage(Path).

# Task with quality violations needs remediation
task_needs_remediation(TaskID) :-
    quality_violation_detected(TaskID, _).

# Auto-spawn remediation task
remediation_task_needed(TaskID, ViolationType) :-
    quality_violation_detected(TaskID, ViolationType),
    !remediation_task_exists(TaskID, ViolationType).

# Helper to check if remediation already spawned
# Note: ViolationType bound via task_remediation_target which tracks what we're fixing
remediation_task_exists(OriginalTaskID, ViolationType) :-
    campaign_task(RemediationID, _, _, _, /fix),
    task_dependency(RemediationID, OriginalTaskID),
    task_remediation_target(RemediationID, OriginalTaskID, ViolationType).

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
phase_blocked(PhaseID, "build_failed") :-
    phase_build_failed(PhaseID).

# Phase test check failed
phase_tests_failed(PhaseID) :-
    phase_checkpoint(PhaseID, /tests, /false, _, _).

phase_blocked(PhaseID, "tests_failed") :-
    phase_tests_failed(PhaseID).

# =============================================================================
# SECTION 4: CONTEXT BUDGET MANAGEMENT
# =============================================================================
# Rules for managing LLM context window during campaigns.

# -----------------------------------------------------------------------------
# 4.1 Context Pressure Detection
# -----------------------------------------------------------------------------

# Context is under pressure (> 80% utilized)
context_pressure_high(CampaignID) :-
    context_window_state(CampaignID, Used, Total, _),
    Utilization = fn:mult(100, fn:div(Used, Total)),
    Utilization > 80.

# Context is critical (> 95% utilized)
context_pressure_critical(CampaignID) :-
    context_window_state(CampaignID, Used, Total, _),
    Utilization = fn:mult(100, fn:div(Used, Total)),
    Utilization > 95.

# Trigger compression when pressure is high
next_action(/compress_context) :-
    current_campaign(CampaignID),
    context_pressure_high(CampaignID),
    !context_pressure_critical(CampaignID).

# Emergency compression when critical
next_action(/emergency_compress) :-
    current_campaign(CampaignID),
    context_pressure_critical(CampaignID).

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
activation(Summary, 80) :-
    campaign_phase(PhaseID, CampaignID, _, Order, /completed, _),
    current_phase(CurrentPhaseID),
    campaign_phase(CurrentPhaseID, CampaignID, _, CurrentOrder, _, _),
    PrevOrder = fn:minus(CurrentOrder, 1),
    Order = PrevOrder,
    context_compression(PhaseID, Summary, _, _).

# =============================================================================
# SECTION 5: CROSS-PHASE LEARNING
# =============================================================================
# Rules for learning patterns across campaign execution.

# -----------------------------------------------------------------------------
# 5.1 Success Pattern Detection
# -----------------------------------------------------------------------------

# Phase completed quickly (success pattern)
phase_completed_fast(PhaseID) :-
    campaign_phase(PhaseID, _, _, _, /completed, _),
    campaign_milestone(_, PhaseID, _, CompletedAt),
    phase_estimate(PhaseID, EstTasks, _),
    EstTasks > 0.

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
systemic_error_detected(ErrorType) :-
    task_error(T1, ErrorType, _),
    task_error(T2, ErrorType, _),
    task_error(T3, ErrorType, _),
    T1 != T2,
    T2 != T3,
    T1 != T3.

# Systemic error should trigger investigation
next_action(/investigate_systemic) :-
    current_campaign(_),
    systemic_error_detected(ErrorType),
    !systemic_error_investigated(ErrorType).

# Helper for safe negation
systemic_error_investigated(ErrorType) :-
    campaign_learning(_, /failure_pattern, ErrorType, _, _).

# Learn to avoid systemic errors
learning_signal(/avoid_pattern, ErrorType) :-
    systemic_error_detected(ErrorType).

# -----------------------------------------------------------------------------
# 5.3 Shard Performance in Campaigns
# -----------------------------------------------------------------------------

# Track which shard types perform well in campaigns
shard_campaign_success(ShardType) :-
    campaign_shard(_, ShardID, ShardType, _, /completed),
    shard_success(ShardID).

# Track shard failures in campaigns
shard_campaign_failure(ShardType) :-
    campaign_shard(_, ShardID, ShardType, _, /failed),
    shard_error(ShardID, _).

# Shard is reliable for campaigns (3+ successes, < 2 failures)
shard_campaign_reliable(ShardType) :-
    shard_campaign_success(ShardType),
    !shard_has_many_failures(ShardType).

# Helper: shard has 2+ failures
shard_has_many_failures(ShardType) :-
    campaign_shard(_, S1, ShardType, _, /failed),
    campaign_shard(_, S2, ShardType, _, /failed),
    S1 != S2.

# Prefer reliable shards for campaign tasks
delegate_task(ShardType, Task, /pending) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, Task, _, TaskType),
    shard_profile(ShardType, _, _),
    shard_campaign_reliable(ShardType),
    shard_can_handle(ShardType, TaskType).

# =============================================================================
# SECTION 6: RESILIENCE & RECOVERY
# =============================================================================
# Rules for handling failures and recovering campaigns.

# -----------------------------------------------------------------------------
# 6.1 Cascading Failure Detection
# -----------------------------------------------------------------------------

# Multiple consecutive task failures in same phase
phase_failure_cascade(PhaseID) :-
    campaign_task(T1, PhaseID, _, /failed, _),
    campaign_task(T2, PhaseID, _, /failed, _),
    campaign_task(T3, PhaseID, _, /failed, _),
    T1 != T2,
    T2 != T3,
    T1 != T3.

# Cascade triggers phase pause
phase_blocked(PhaseID, "failure_cascade") :-
    phase_failure_cascade(PhaseID).

# Cascade triggers replan consideration
replan_needed(CampaignID, "phase_failure_cascade") :-
    current_campaign(CampaignID),
    current_phase(PhaseID),
    phase_failure_cascade(PhaseID).

# -----------------------------------------------------------------------------
# 6.2 Stuck Detection
# -----------------------------------------------------------------------------

# Phase is stuck: in-progress but no runnable tasks
phase_stuck(PhaseID) :-
    campaign_phase(PhaseID, CampaignID, _, _, /in_progress, _),
    current_campaign(CampaignID),
    !has_runnable_task(PhaseID),
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

# Skip non-critical failed task after 3 retries
task_can_skip(TaskID) :-
    task_retry_exhausted(TaskID),
    task_priority(TaskID, /low).

task_can_skip(TaskID) :-
    task_retry_exhausted(TaskID),
    task_priority(TaskID, /normal),
    !task_blocks_others(TaskID).

# Helper: task blocks other tasks
task_blocks_others(TaskID) :-
    task_dependency(_, TaskID).

# Critical task failure requires escalation
escalation_required(TaskID, "critical_task_failed") :-
    task_retry_exhausted(TaskID),
    task_priority(TaskID, /critical).

escalation_required(TaskID, "high_priority_failed") :-
    task_retry_exhausted(TaskID),
    task_priority(TaskID, /high),
    task_blocks_others(TaskID).

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

# Campaign is past halfway point
campaign_past_halfway(CampaignID) :-
    campaign_progress(CampaignID, Completed, Total, _, _),
    Total > 0,
    Progress = fn:mult(100, fn:div(Completed, Total)),
    Progress >= 50.

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

# -----------------------------------------------------------------------------
# 7.3 Progress Events
# -----------------------------------------------------------------------------

# Trigger progress update on task completion
progress_changed(CampaignID) :-
    current_campaign(CampaignID),
    campaign_task(_, _, _, /completed, _).

progress_changed(CampaignID) :-
    current_campaign(CampaignID),
    campaign_phase(_, CampaignID, _, _, /completed, _).

# =============================================================================
# SECTION 8: SHARD SELECTION FOR CAMPAIGNS
# =============================================================================
# Rules for choosing the right shard for campaign tasks.

# -----------------------------------------------------------------------------
# 8.1 Task Type to Shard Mapping
# -----------------------------------------------------------------------------

# File creation → Coder
campaign_task_shard(TaskID, /coder) :-
    campaign_task(TaskID, _, _, /pending, /file_create).

# File modification → Coder
campaign_task_shard(TaskID, /coder) :-
    campaign_task(TaskID, _, _, /pending, /file_modify).

# Test writing → Tester
campaign_task_shard(TaskID, /tester) :-
    campaign_task(TaskID, _, _, /pending, /test_write).

# Test running → Tester
campaign_task_shard(TaskID, /tester) :-
    campaign_task(TaskID, _, _, /pending, /test_run).

# Research → Researcher
campaign_task_shard(TaskID, /researcher) :-
    campaign_task(TaskID, _, _, /pending, /research).

# Verification → Reviewer
campaign_task_shard(TaskID, /reviewer) :-
    campaign_task(TaskID, _, _, /pending, /verify).

# Refactoring → Coder (with reviewer support)
campaign_task_shard(TaskID, /coder) :-
    campaign_task(TaskID, _, _, /pending, /refactor).

# Integration → Coder
campaign_task_shard(TaskID, /coder) :-
    campaign_task(TaskID, _, _, /pending, /integrate).

# Documentation → Coder
campaign_task_shard(TaskID, /coder) :-
    campaign_task(TaskID, _, _, /pending, /document).

# -----------------------------------------------------------------------------
# 8.2 Specialist Override
# -----------------------------------------------------------------------------

# If specialist available and performs well, prefer it
campaign_task_shard_override(TaskID, SpecialistName) :-
    campaign_task(TaskID, _, _, /pending, TaskType),
    shard_profile(SpecialistName, /specialist, _),
    shard_campaign_reliable(SpecialistName),
    shard_can_handle(SpecialistName, TaskType).

# Final shard selection (specialist override or default)
final_shard_for_task(TaskID, SpecialistName) :-
    campaign_task_shard_override(TaskID, SpecialistName).

final_shard_for_task(TaskID, ShardType) :-
    campaign_task_shard(TaskID, ShardType),
    !campaign_task_shard_override(TaskID, _).

# =============================================================================
# SECTION 9: CAMPAIGN INTENT HANDLING
# =============================================================================
# Rules for handling user intents during active campaigns.

# -----------------------------------------------------------------------------
# 9.1 Intent Interruption
# -----------------------------------------------------------------------------

# User mutation intent during campaign needs routing decision
campaign_intent_conflict(IntentID) :-
    current_campaign(_),
    user_intent(IntentID, /mutation, _, _, _),
    !intent_is_campaign_related(IntentID).

# Intent is campaign-related if it matches current phase
intent_is_campaign_related(IntentID) :-
    current_campaign(CampaignID),
    current_phase(PhaseID),
    user_intent(IntentID, _, _, Target, _),
    phase_objective(PhaseID, _, Description, _).

# Route conflict to user for decision
next_action(/ask_campaign_interrupt) :-
    campaign_intent_conflict(_).

# -----------------------------------------------------------------------------
# 9.2 Campaign Queries
# -----------------------------------------------------------------------------

# Status query during campaign
next_action(/show_campaign_status) :-
    current_campaign(_),
    user_intent(_, /query, _, /status, _).

# Progress query during campaign
next_action(/show_campaign_progress) :-
    current_campaign(_),
    user_intent(_, /query, _, /progress, _).

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
    campaign(CampaignID, Type, _, _, /completed),
    campaign_metadata(CampaignID, _, _, Confidence),
    Confidence > 80,
    Pattern = Type.

# Promote campaign success to long-term memory
promote_to_long_term(/campaign_success, Pattern) :-
    campaign_success_pattern(_, Pattern).

# Track campaign type effectiveness
campaign_type_effective(Type) :-
    campaign(C1, Type, _, _, /completed),
    campaign(C2, Type, _, _, /completed),
    C1 != C2.

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

# Helper for safe negation
campaign_cleaned(CampaignID) :-
    campaign_learning(CampaignID, /cleanup, _, _, _).

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

# Campaign is healthy (making progress)
campaign_healthy(CampaignID) :-
    current_campaign(CampaignID),
    !campaign_blocked(CampaignID, _),
    !phase_stuck(_),
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
    campaign_phase(P1, CampaignID, _, _, _, _),
    campaign_phase(P2, CampaignID, _, _, _, _),
    P1 != P2.

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
campaign_blocked(CampaignID, "topology_violations") :-
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
    phase_layer_priority(PhaseB, PriorityB),
    phase_dependency(PhaseB, PhaseA, _),
    PriorityA < PriorityB.

# Warning if layer sequencing is wrong
layer_sequencing_warning(PhaseA, PhaseB) :-
    phase_dependency(PhaseB, PhaseA, _),
    phase_layer_priority(PhaseA, PriorityA),
    phase_layer_priority(PhaseB, PriorityB),
    PriorityA >= PriorityB.

# =============================================================================
# END OF CAMPAIGN RULES
# =============================================================================
