# Coder Shard Policy - Impact Analysis
# Description: Analyze impact of changes before making them.

# =============================================================================
# SECTION 4: IMPACT ANALYSIS
# =============================================================================

# -----------------------------------------------------------------------------
# 4.1 Bounded Impact Propagation
# -----------------------------------------------------------------------------
# "Recursive Logic Bomb" Fix: Bounded recursion depth (3 levels)

# Each hop leads with the impacted side and looks its importers up:
# dependency_link holds every file import edge, the frontier a handful.

# Direct impact (1-hop)
coder_impacted_1(X) :- modified(Y), dependency_link(X, Y, _).
coder_impacted_1(X) :- coder_target(Y), dependency_link(X, Y, _).

# Transitive impact (2-hop)
coder_impacted_2(X) :- coder_impacted_1(Z), dependency_link(X, Z, _).

# Deep impact (3-hop)
coder_impacted_3(X) :- coder_impacted_2(Z), dependency_link(X, Z, _).

# Union of all levels
coder_impacted(X) :- coder_impacted_1(X).
coder_impacted(X) :- coder_impacted_2(X).
coder_impacted(X) :- coder_impacted_3(X).

# -----------------------------------------------------------------------------
# 4.2 Impact Classification
# -----------------------------------------------------------------------------

# High impact: many dependents affected
high_impact_edit(File) :-
    coder_target(File),
    dependent_count(File, N),
    N > 5.

# Critical impact: core files or interfaces
critical_impact_edit(File) :-
    coder_target(File),
    is_core_file(File).

critical_impact_edit(File) :-
    coder_target(File),
    is_interface_file(File).

# Cross-package impact
cross_package_impact(File) :-
    coder_target(File),
    file_package(File, Pkg1),
    dependency_link(Dependent, File, _),
    file_package(Dependent, Pkg2),
    coder_impacted(Dependent),
    Pkg1 != Pkg2.

# -----------------------------------------------------------------------------
# 4.3 Impact Warnings
# -----------------------------------------------------------------------------

# Warn about high-impact edits
# WarningType is a closed 3-value vocabulary (schemas_coder.mg declares /name).
impact_warning(File, /high_dependent_count) :-
    high_impact_edit(File).

impact_warning(File, /critical_file) :-
    critical_impact_edit(File).

impact_warning(File, /cross_package) :-
    cross_package_impact(File).
