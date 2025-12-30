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
projection_violation(ActionID, "test_failure") :-
    simulated_effect(ActionID, "diagnostic", _),
    simulated_effect(ActionID, "diagnostic_severity", /error).

projection_violation(ActionID, "security_violation") :-
    simulated_effect(ActionID, "security_violation", _).

# Block action if projection fails
block_commit("shadow_simulation_failed") :-
    pending_mutation(MutationID, _, _, _),
    !safe_projection(MutationID).
