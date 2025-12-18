# System Shard Coordination
# Section 21 of Cortex Executive Policy

# Intent Processing Flow (Perception → Executive)

# A user_intent is pending if not yet processed by executive
pending_intent(/current_intent) :-
    user_intent(/current_intent, _, _, _, _),
    !executive_processed_intent(/current_intent).

# Focus needs resolution if confidence is low (score on 0-100 scale)
focus_needs_resolution(Ref) :-
    focus_resolution(Ref, _, _, Score),
    Score < 70.

# Intent ready for executive processing
intent_ready_for_executive(/current_intent) :-
    user_intent(/current_intent, _, _, Target, _),
    !focus_needs_resolution(Target).

# Action Flow (Executive → Constitution → Router)

# pending_permission_check/1 is derived from the executable action envelope.
pending_permission_check(ActionID) :-
    pending_action(ActionID, _, _, _, _).

# Action is pending permission check from constitution gate
action_pending_permission(ActionID) :-
    pending_permission_check(ActionID),
    !permission_checked(ActionID).

# Helper for safe negation
permission_checked(ActionID) :-
    permission_check_result(ActionID, _, _, _).

# Action is permitted by constitution gate
action_permitted(ActionID) :-
    permission_check_result(ActionID, /permit, _, _).

# Action is blocked by constitution gate
action_blocked(ActionID, Reason) :-
    permission_check_result(ActionID, /deny, Reason, _).

# Action ready for routing (permitted and not yet routed)
action_ready_for_routing(ActionID) :-
    action_permitted(ActionID),
    !action_routed(ActionID).

# ready_for_routing/1 is derived from a successful permission check.
ready_for_routing(ActionID) :-
    permission_check_result(ActionID, /permit, _, _).

# Helper for safe negation
action_routed(ActionID) :-
    ready_for_routing(ActionID),
    routing_result(ActionID, _, _, _).

# Derive routing result success
routing_succeeded(ActionID) :-
    routing_result(ActionID, /success, _, _).

# Derive routing result failure
routing_failed(ActionID, Error) :-
    routing_result(ActionID, /failure, Error, _).

# Safety Violation Handling (Constitution Gate)

# A safety violation blocks all further actions
block_all_actions("safety_violation") :-
    safety_violation(_, _, _, _).

# Security anomaly triggers investigation
# Need to check if there are uninvestigated anomalies
next_action(/investigate_anomaly) :-
    security_anomaly(AnomalyID, _, _),
    !anomaly_investigated(AnomalyID).

# Helper for safe negation
anomaly_investigated(AnomalyID) :-
    security_anomaly(AnomalyID, _, _),
    investigation_result(AnomalyID, _).

# Pattern recognition for repeated violations
repeated_violation_pattern(Pattern) :-
    safety_violation(_, Pattern, _, _),
    violation_count(Pattern, Count),
    Count >= 3.

# Propose rule when pattern detected (Autopoiesis)
propose_safety_rule(Pattern) :-
    repeated_violation_pattern(Pattern).

# World Model Updates (World Model Ingestor)

# File change triggers world model update
world_model_stale(File) :-
    modified(File),
    file_topology(File, _, _, _, _).

# Trigger ingestor when world model is stale
next_action(/update_world_model) :-
    world_model_stale(_),
    system_shard_healthy(/world_model_ingestor).

# File topology derived from filesystem
file_in_project(File) :-
    file_topology(File, _, _, _, _).

# Symbol graph connectivity (uses dependency_link for edges)
symbol_reachable(From, To) :-
    dependency_link(From, To, _).

symbol_reachable(From, To) :-
    dependency_link(From, Mid, _),
    symbol_reachable(Mid, To).
