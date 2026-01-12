# Coder Shard Policy - Impact Analysis
# Description: Analyze impact of changes before making them.

# =============================================================================
# SECTION 4: IMPACT ANALYSIS
# =============================================================================

# -----------------------------------------------------------------------------
# 4.1 Bounded Impact Propagation
# -----------------------------------------------------------------------------
# "Recursive Logic Bomb" Fix: Bounded recursion depth (3 levels)

# Direct impact (1-hop)
coder_impacted_1(X) :- dependency_link(X, Y, _), modified(Y).

# Transitive impact (2-hop)
coder_impacted_2(X) :- dependency_link(X, Z, _), coder_impacted_1(Z).

# Deep impact (3-hop)
coder_impacted_3(X) :- dependency_link(X, Z, _), coder_impacted_2(Z).

# Union of all levels
coder_impacted(X) :- coder_impacted_1(X).
coder_impacted(X) :- coder_impacted_2(X).
coder_impacted(X) :- coder_impacted_3(X).

# -----------------------------------------------------------------------------
# 4.2 Impact Classification
# -----------------------------------------------------------------------------

# High impact: many dependents affected
high_impact_edit(File) :-
    pending_edit(File, _),
    dependent_count(File, N),
    N > 5.

# Critical impact: core files or interfaces
critical_impact_edit(File) :-
    pending_edit(File, _),
    is_core_file(File).

critical_impact_edit(File) :-
    pending_edit(File, _),
    is_interface_file(File).

# Cross-package impact
cross_package_impact(File) :-
    pending_edit(File, _),
    file_package(File, Pkg1),
    dependency_link(Dependent, File, _),
    file_package(Dependent, Pkg2),
    coder_impacted(Dependent),
    Pkg1 != Pkg2.

# -----------------------------------------------------------------------------
# 4.3 Impact Warnings
# -----------------------------------------------------------------------------

# Warn about high-impact edits
impact_warning(File, "high_dependent_count") :-
    high_impact_edit(File).

impact_warning(File, "critical_file") :-
    critical_impact_edit(File).

impact_warning(File, "cross_package") :-
    cross_package_impact(File).
