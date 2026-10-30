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

# Why is coder blocked?
coder_blocked_reason(File, Reason) :-
    coder_block_write(File, Reason).

coder_blocked_reason(File, Reason) :-
    coder_block_action(/edit, Reason),
    pending_edit(File, _).

# -----------------------------------------------------------------------------
# 13.2 Diagnostic Summary
# -----------------------------------------------------------------------------

# -----------------------------------------------------------------------------
# 13.3 Performance Metrics
# -----------------------------------------------------------------------------

