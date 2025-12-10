# internal/mangle/build_topology.mg
# =========================================================
# BUILD TOPOLOGY ENFORCEMENT
# Enforces architectural ordering between phases using explicit categories.
# =========================================================

# ----------------------------------------------------------------------------- 
# 1. Canonical Build Layers
# -----------------------------------------------------------------------------

build_phase_type(/scaffold, 10).     # Config, env, bootstrapping
build_phase_type(/domain_core, 20).  # Interfaces, types, constants
build_phase_type(/data_layer, 30).   # Schemas, repositories, migrations
build_phase_type(/service, 40).      # Business logic, state machines
build_phase_type(/transport, 50).    # HTTP, gRPC, CLI, UI endpoints
build_phase_type(/integration, 60).  # Wiring, main, E2E, deploy

# Natural language aliases to improve LLM classification resilience
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

# Gap warning: phases skip more than one layer
# NOTE: Uses transform pipeline for arithmetic per Mangle spec
suspicious_gap(Downstream, Upstream) :-
    phase_dependency(Downstream, Upstream, _),
    phase_precedence(Downstream, ScoreDown),
    phase_precedence(Upstream, ScoreUp)
    |> let Gap = fn:minus(ScoreDown, ScoreUp)
    |> do fn:filter(fn:gt(Gap, 20)).

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
