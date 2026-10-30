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

# Apply edit when code generated
next_coder_action(/apply_edit) :-
    coder_state(/code_generated),
    pending_edit(File, _),
    coder_safe_to_write(File).

# Run build after edit applied
next_coder_action(/run_build) :-
    coder_state(/edit_applied).

# Complete if build passed.
# `!edit_needs_tests(_)` excluded nothing, so the coder was told to complete
# even while edits were still waiting on tests.
next_coder_action(/complete) :-
    coder_state(/build_passed).

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

# =============================================================================
# SECTION 8: CONTEXT GATHERING INTELLIGENCE
# =============================================================================

# -----------------------------------------------------------------------------
# 8.1 Context Priority
# -----------------------------------------------------------------------------

# -----------------------------------------------------------------------------
# 8.2 Context Selection
# -----------------------------------------------------------------------------

# -----------------------------------------------------------------------------
# 8.3 Context Exclusion
# -----------------------------------------------------------------------------

# Final context decision
final_context_include(File) :-
    include_in_context(File).

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

# =============================================================================
# SECTION 13: OBSERVABILITY & DEBUGGING
# =============================================================================

# -----------------------------------------------------------------------------
# 13.1 State Queries
# -----------------------------------------------------------------------------

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

# -----------------------------------------------------------------------------
# 13.3 Performance Metrics
# -----------------------------------------------------------------------------

