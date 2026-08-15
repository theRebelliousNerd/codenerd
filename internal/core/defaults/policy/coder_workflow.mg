# Coder Shard Policy - Workflow, Context, Campaign & Observability
# Description: Logic for next actions, context gathering, campaign integration, and observability.

# =============================================================================
# SECTION 7: NEXT ACTION DERIVATION
# =============================================================================

# -----------------------------------------------------------------------------
# 7.1 State Machine Actions
# -----------------------------------------------------------------------------

# Helper: file has content loaded (for safe negation)
Decl has_file_content(File).
has_file_content(File) :-
    file_content(File, _).

# Read context if needed
next_coder_action(/read_context) :-
    coder_state(/idle),
    coder_task(_, _, Target, _),
    !has_file_content(Target).

# Generate code when context ready
next_coder_action(/generate_code) :-
    coder_state(/context_ready),
    coder_task(_, _, _, _).

# Apply edit when code generated
next_coder_action(/apply_edit) :-
    coder_state(/code_generated),
    pending_edit(File, _),
    coder_safe_to_write(File).

# Request review for high-impact edits
next_coder_action(/request_review) :-
    coder_state(/code_generated),
    pending_edit(File, _),
    high_impact_edit(File).

# Run build after edit applied
next_coder_action(/run_build) :-
    coder_state(/edit_applied).

# Run tests if build passes
next_coder_action(/run_tests) :-
    coder_state(/build_passed),
    edit_needs_tests(_).

any_edit_needs_tests(/yes) :-
    edit_needs_tests(FilePath).

# Complete if build passed.
# `!edit_needs_tests(_)` excluded nothing, so the coder was told to complete
# even while edits were still waiting on tests.
next_coder_action(/complete) :-
    coder_state(/build_passed),
    !any_edit_needs_tests(/yes).

next_coder_action(/complete) :-
    coder_state(/tests_passed).

# Escalate after too many retries
next_coder_action(/escalate) :-
    coder_state(/build_failed),
    retry_count(N),
    N >= 3.

# Retry on build failure with fewer retries
next_coder_action(/retry_with_diagnostics) :-
    coder_state(/build_failed),
    retry_count(N),
    N < 3.

# -----------------------------------------------------------------------------
# 7.2 Recovery Actions
# -----------------------------------------------------------------------------

# Roll back if critical edit fails
next_coder_action(/rollback) :-
    coder_state(/build_failed),
    retry_count(N),
    N >= 2,
    pending_edit(File, _),
    critical_impact_edit(File).

# Decompose complex task on failure
next_coder_action(/decompose_task) :-
    coder_state(/build_failed),
    retry_count(N),
    N >= 2,
    coder_task_complexity(/complex).

# =============================================================================
# SECTION 8: CONTEXT GATHERING INTELLIGENCE
# =============================================================================

# -----------------------------------------------------------------------------
# 8.1 Context Priority
# -----------------------------------------------------------------------------

# Current target has highest priority
coder_context_priority(File, 100) :-
    coder_target(File).

# Direct dependencies have high priority
coder_context_priority(File, 80) :-
    coder_target(Target),
    dependency_link(Target, File, _).

# Files that import target have medium priority
coder_context_priority(File, 60) :-
    coder_target(Target),
    dependency_link(File, Target, _).

# Test files for target have high priority
coder_context_priority(File, 75) :-
    coder_target(Target),
    test_file_for(File, Target).

# Interface definitions have high priority
coder_context_priority(File, 70) :-
    coder_target(Target),
    is_interface_file(File),
    same_package(File, Target).

# -----------------------------------------------------------------------------
# 8.2 Context Selection
# -----------------------------------------------------------------------------

# Include file in context if priority is high enough
include_in_context(File) :-
    coder_context_priority(File, P),
    file_in_project(File),
    P >= 50.

# Include related test files
include_in_context(File) :-
    coder_target(Target),
    test_file_for(File, Target).

# Include type definitions
include_in_context(File) :-
    coder_target(Target),
    same_package(File, Target),
    type_definition_file(File).

# -----------------------------------------------------------------------------
# 8.3 Context Exclusion
# -----------------------------------------------------------------------------

# Exclude generated files from context
exclude_from_context(File) :-
    is_generated_file(File).

# Exclude vendor files
exclude_from_context(File) :-
    is_vendor_file(File).

# Exclude binary files
exclude_from_context(File) :-
    is_binary_file(File).

# Final context decision
final_context_include(File) :-
    include_in_context(File),
    !exclude_from_context(File).

# =============================================================================
# SECTION 12: CAMPAIGN INTEGRATION
# =============================================================================

# -----------------------------------------------------------------------------
# 12.1 Campaign Context Awareness
# -----------------------------------------------------------------------------

# Coder is operating within campaign
in_campaign_context() :-
    current_campaign(_).

# Current phase objectives affect coder strategy
campaign_coder_focus(Objective) :-
    in_campaign_context(),
    current_phase(PhaseID),
    phase_objective(PhaseID, _, Objective, _).

# -----------------------------------------------------------------------------
# 12.2 Campaign Quality Requirements
# -----------------------------------------------------------------------------

# Campaign phase requires tests
campaign_requires_tests() :-
    in_campaign_context(),
    current_phase(PhaseID),
    phase_checkpoint(PhaseID, /tests, _, _, _).

# Campaign phase requires build pass
campaign_requires_build() :-
    in_campaign_context(),
    current_phase(PhaseID),
    phase_checkpoint(PhaseID, /build, _, _, _).

# Stricter quality during campaigns
coder_quality_mode(/strict) :-
    in_campaign_context().

coder_quality_mode(/normal) :-
    !in_campaign_context().

# -----------------------------------------------------------------------------
# 12.3 Campaign Progress Reporting
# -----------------------------------------------------------------------------

# Report coder completion to campaign
coder_task_completed(TaskID) :-
    coder_task(TaskID, _, _, _),
    coder_state(/build_passed).

coder_task_completed(TaskID) :-
    coder_task(TaskID, _, _, _),
    coder_state(/tests_passed).

# Report coder failure to campaign
coder_task_failed(TaskID, Reason) :-
    coder_state(/build_failed),
    retry_count(N),
    N >= 3,
    Reason = "max_retries_exceeded",
    coder_task(TaskID, _, _, _).

# =============================================================================
# SECTION 13: OBSERVABILITY & DEBUGGING
# =============================================================================

# -----------------------------------------------------------------------------
# 13.1 State Queries
# -----------------------------------------------------------------------------

# Current coder status (bounded: all three predicates are singletons 1×1×1)
# Acceptable cross-product: max 1 fact each for coder_state, coder_target, coder_strategy.
coder_status(State, Target, Strategy) :-
    coder_state(State),
    coder_target(Target),
    coder_strategy(Strategy).

# Why is coder blocked?
coder_blocked_reason(File, Reason) :-
    coder_block_write(File, Reason).

# Helper: an edit action is currently blocked
Decl has_edit_block(Reason).
has_edit_block(Reason) :- coder_block_action(/edit, Reason).

coder_blocked_reason(File, Reason) :-
    pending_edit(File, _),
    has_edit_block(Reason).

# -----------------------------------------------------------------------------
# 13.2 Diagnostic Summary
# -----------------------------------------------------------------------------

# Count errors for current target
target_error_count(Count) :-
    coder_target(Target),
    diagnostic_count(Target, /error, Count).

# Count warnings for current target
target_warning_count(Count) :-
    coder_target(Target),
    diagnostic_count(Target, /warning, Count).

# -----------------------------------------------------------------------------
# 13.3 Performance Metrics
# -----------------------------------------------------------------------------

# Helper: previous state differs from current
Decl has_state_change(Current, Previous).
has_state_change(Current, Previous) :-
    coder_state(Current),
    previous_coder_state(Previous),
    Current != Previous.

# Coder is making progress
coder_progressing() :-
    has_state_change(_, _).

# Helper: state has been unchanged too long
Decl state_is_stuck() bound [].
state_is_stuck() :-
    state_unchanged_count(N),
    N >= 3.

# Coder is stuck (state unchanged AND count exceeds threshold)
coder_stuck() :-
    coder_state(State),
    previous_coder_state(State),
    state_is_stuck().
