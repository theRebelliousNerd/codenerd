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

# -----------------------------------------------------------------------------
# 6.2 Diagnostic Classification
# -----------------------------------------------------------------------------

# -----------------------------------------------------------------------------
# 6.3 Diagnostic Prioritization
# -----------------------------------------------------------------------------

# The id of a diagnostic here is its file: diagnostic/5 is (Severity, File, Line, Code,
# Message). The rules once read the file out of the third column, which is the line.
# Highest priority: errors in current file
priority_diagnostic(File, 100) :-
    diagnostic(/error, File, _, _, _),
    coder_target(File).

# High priority: errors in impacted files
priority_diagnostic(File, 80) :-
    diagnostic(/error, File, _, _, _),
    coder_impacted(File).

# Medium priority: warnings in current file
priority_diagnostic(File, 50) :-
    diagnostic(/warning, File, _, _, _),
    coder_target(File).

# Low priority: lint issues
priority_diagnostic(DiagID, 20) :-
    diagnostic(/lint, DiagID, _, _, _).
