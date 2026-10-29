# Coder Shard Policy - Build State & Diagnostics
# Description: Track build state and diagnostics.

# =============================================================================
# SECTION 6: BUILD STATE & DIAGNOSTICS
# =============================================================================

# -----------------------------------------------------------------------------
# 6.1 Build State
# -----------------------------------------------------------------------------

# Block commit on build errors
block_commit("build_errors") :-
    diagnostic(/error, _, _, _, _).

block_commit("build_errors") :-
    build_state(/failing).

# Build is healthy
build_healthy() :-
    build_state(/passing),
    !has_errors().

has_errors() :-
    diagnostic(/error, _, _, _, _).

# -----------------------------------------------------------------------------
# 6.2 Diagnostic Classification
# -----------------------------------------------------------------------------

# Error requires immediate fix
requires_immediate_fix(DiagID) :-
    diagnostic(/error, DiagID, _, _, _).

# Warning should be addressed
should_address_warning(DiagID) :-
    diagnostic(/warning, DiagID, _, _, _),
    !warning_suppressed(DiagID).

# Lint issues can be deferred
can_defer_lint(DiagID) :-
    diagnostic(/lint, DiagID, _, _, _).

# Helper for suppressed warnings
warning_suppressed(DiagID) :-
    suppression(DiagID, _).

# -----------------------------------------------------------------------------
# 6.3 Diagnostic Prioritization
# -----------------------------------------------------------------------------

# Highest priority: errors in current file
priority_diagnostic(DiagID, 100) :-
    diagnostic(/error, DiagID, File, _, _),
    coder_target(File).

# High priority: errors in impacted files
priority_diagnostic(DiagID, 80) :-
    diagnostic(/error, DiagID, File, _, _),
    coder_impacted(File).

# Medium priority: warnings in current file
priority_diagnostic(DiagID, 50) :-
    diagnostic(/warning, DiagID, File, _, _),
    coder_target(File).

# Low priority: lint issues
priority_diagnostic(DiagID, 20) :-
    diagnostic(/lint, DiagID, _, _, _).
