# internal/mangle/build_topology.mg
# =========================================================
# BUILD TOPOLOGY ENFORCEMENT
# Enforces architectural ordering between phases using explicit categories.
# =========================================================

# ----------------------------------------------------------------------------- 
# 1. Canonical Build Layers
# -----------------------------------------------------------------------------

# NOTE: this table must cover EVERY category the Go normalizer can emit
# (internal/campaign/normalization.go: allowedPhaseCategories). A category with no
# entry here derives no phase_precedence, which makes has_phase_category/1 false and
# raises a bogus "missing_category" validation error on an otherwise valid plan.
# internal/campaign/topology_contract_test.go pins the two tables together.
build_phase_type(/research, 5).      # Discovery, analysis, planning (precedes all build work)
build_phase_type(/scaffold, 10).     # Config, env, bootstrapping
build_phase_type(/domain_core, 20).  # Interfaces, types, constants
build_phase_type(/data_layer, 30).   # Schemas, repositories, migrations
build_phase_type(/service, 40).      # Business logic, state machines
build_phase_type(/transport, 50).    # HTTP, gRPC, CLI, UI endpoints
build_phase_type(/integration, 60).  # Wiring, main, E2E, deploy
build_phase_type(/test, 70).         # Verification, QA, review (follows integration)
build_phase_type(/ops, 80).          # Release, deploy, monitoring (last)

# Natural language aliases to improve LLM classification resilience.
# Mirrored in Go by phaseCategorySynonyms so that an alias survives normalization —
# without the Go half these facts are unreachable, because the Go normalizer collapses
# any unrecognized string to the fallback before the kernel ever sees it.
phase_synonym(/research, "planning").
phase_synonym(/research, "discovery").
phase_synonym(/research, "analysis").
phase_synonym(/service, "implementation").
phase_synonym(/service, "remediation").
phase_synonym(/scaffold, "setup").
phase_synonym(/scaffold, "config").
phase_synonym(/scaffold, "bootstrap").
phase_synonym(/domain_core, "types").
phase_synonym(/domain_core, "interfaces").
phase_synonym(/domain_core, "entities").
phase_synonym(/data_layer, "database").
phase_synonym(/data_layer, "storage").
phase_synonym(/service, "logic").
phase_synonym(/service, "processor").
phase_synonym(/transport, "api").
phase_synonym(/transport, "frontend").
phase_synonym(/integration, "wiring").
phase_synonym(/integration, "main").
phase_synonym(/test, "testing").
phase_synonym(/test, "qa").
phase_synonym(/test, "verification").
phase_synonym(/test, "review").
phase_synonym(/ops, "deploy").
phase_synonym(/ops, "release").
phase_synonym(/ops, "monitoring").

# ----------------------------------------------------------------------------- 
# 2. Phase Precedence
# -----------------------------------------------------------------------------

# Derive precedence score from explicit category
# Derive precedence score from explicit category
phase_precedence(PhaseID, Score) :-
    phase_category(PhaseID, Category),
    build_phase_type(Category, Score).

# If category provided via synonym, map it
phase_precedence(PhaseID, Score) :-
    phase_category(PhaseID, Alias),
    phase_synonym(Category, Alias),
    build_phase_type(Category, Score).

# ----------------------------------------------------------------------------- 
# 3. Violations & Warnings
# -----------------------------------------------------------------------------

# Architectural inversion: downstream depends on upstream with higher precedence score
architectural_violation(Downstream, Upstream, "inverted_dependency") :-
    phase_dependency(Downstream, Upstream, _),
    phase_precedence(Downstream, ScoreDown),
    phase_precedence(Upstream, ScoreUp),
    ScoreUp > ScoreDown.

# Gap warning: a dependency skips two or more intervening build layers.
#
# Mangle has no inline arithmetic, and the earlier "score difference > 20" shortcut was
# both unexpressible and, as written (ScoreDown > ScoreUp), the definition of a CORRECT
# dependency -- so every well-ordered phase raised task_topology_warning "skips_layer".
# Stated declaratively instead: a gap exists when at least two distinct canonical layers
# sit strictly between the two phases. This is also robust to non-uniform scores.
suspicious_gap(Downstream, Upstream) :-
    phase_dependency(Downstream, Upstream, _),
    phase_precedence(Downstream, ScoreDown),
    phase_precedence(Upstream, ScoreUp),
    build_phase_type(_, Mid1),
    build_phase_type(_, Mid2),
    ScoreUp < Mid1,
    Mid1 < Mid2,
    Mid2 < ScoreDown.

# Helper to check if a phase has any precedence derived
has_phase_category(PhaseID) :-
    phase_precedence(PhaseID, _).

# Validation surface for the decomposer/validator
validation_error(PhaseID, /topology, "inverted_dependency") :-
    architectural_violation(PhaseID, _, _).

validation_error(PhaseID, /topology, "inverted_dependency") :-
    architectural_violation(_, PhaseID, _).

validation_error(PhaseID, /topology, "missing_category") :-
    campaign_phase(PhaseID, _, _, _, _, _),
    !has_phase_category(PhaseID).
