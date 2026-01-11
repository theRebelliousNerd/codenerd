# Coder Policy - Next Action Workflow
# Version: 1.0.0
# Extracted from coder.mg Section 7

# =============================================================================
# SECTION 7: NEXT ACTION DERIVATION
# =============================================================================
# Derive the next coder action based on current state.

# -----------------------------------------------------------------------------
# 7.1 State Machine Actions
# -----------------------------------------------------------------------------

# Helper: file has content loaded (for safe negation)
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

# Complete if build passed
next_coder_action(/complete) :-
    coder_state(/build_passed),
    !edit_needs_tests(_).

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
    pending_edit(File, _),
    critical_impact_edit(File),
    retry_count(N),
    N >= 2.

# Decompose complex task on failure
next_coder_action(/decompose_task) :-
    coder_state(/build_failed),
    task_complexity(/complex),
    retry_count(N),
    N >= 2.
