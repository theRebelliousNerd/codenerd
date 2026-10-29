# Shadow Mode / Counterfactual Reasoning
# Section 14 of Cortex Executive Policy

# Default implication: echo hypothetical input into derived implications
derives_from_hypothetical(Change) :-
    hypothetical(Change).


# Helper for safe negation
has_projection_violation(ActionID) :-
    projection_violation(ActionID, _).

# Safe projection - action passes safety checks in shadow simulation
safe_projection(ActionID) :-
    shadow_state(_, ActionID, /valid),
    !has_projection_violation(ActionID).

# Projection violation detection
# simulated_effect(ActionID, FactPredicate, FactArgs) is bound
# [/string, /string, /string] and FactArgs is the only slot that can hold a
# value: ShadowMode.projectEffects renders the projected fact's arguments with
# fmt.Sprintf("%v", effect.Args) (internal/core/shadow_mode.go), which is always
# a plain Go string. An atom in that slot never unified with anything.
projection_violation(ActionID, /test_failure) :-
    simulated_effect(ActionID, "diagnostic", _),
    simulated_effect(ActionID, "diagnostic_severity", "error").

projection_violation(ActionID, /security_violation) :-
    simulated_effect(ActionID, "security_violation", _).

# Block action if projection fails
block_commit("shadow_simulation_failed") :-
    pending_mutation(MutationID, _, _, _),
    !safe_projection(MutationID).
