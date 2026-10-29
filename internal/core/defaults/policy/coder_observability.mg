# Coder Policy - Observability
# Version: 1.0.0
# Extracted from coder.mg Section 13

# =============================================================================
# SECTION 13: OBSERVABILITY & DEBUGGING
# =============================================================================
# Rules for monitoring coder behavior.

# -----------------------------------------------------------------------------
# 13.1 State Queries
# -----------------------------------------------------------------------------

# Current coder status (all singletons - 1×1×1, no actual explosion)
coder_status(State, Target, Strategy) :-
    coder_state(State),
    coder_target(Target),
    coder_strategy(Strategy).

# Why is coder blocked?
coder_blocked_reason(File, Reason) :-
    coder_block_write(File, Reason).

coder_blocked_reason(File, Reason) :-
    coder_block_action(/edit, Reason),
    pending_edit(File, _).

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

# Coder is making progress
coder_progressing() :-
    coder_state(S1),
    previous_coder_state(S2),
    S1 != S2.

# Coder is stuck
coder_stuck() :-
    coder_state(State),
    previous_coder_state(State),
    state_unchanged_count(N),
    N >= 3.
