# Coder Policy - Build State & Diagnostics
# Version: 1.0.0
# Extracted from coder.mg Section 6

# =============================================================================
# SECTION 6: BUILD STATE & DIAGNOSTICS
# =============================================================================
# Track build state and diagnostics.

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
    diagnostic(/warning, DiagID, _, _, _).

# Lint issues can be deferred
can_defer_lint(DiagID) :-
    diagnostic(/lint, DiagID, _, _, _).

# -----------------------------------------------------------------------------
# 6.3 Diagnostic Prioritization
# -----------------------------------------------------------------------------

# High priority: errors in impacted files
priority_diagnostic(DiagID, 80) :-
    diagnostic(/error, DiagID, File, _, _),
    coder_impacted(File).

# Low priority: lint issues
priority_diagnostic(DiagID, 20) :-
    diagnostic(/lint, DiagID, _, _, _).
