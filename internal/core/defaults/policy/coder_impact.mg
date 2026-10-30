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

impact_warning(File, /cross_package) :-
    cross_package_impact(File).
