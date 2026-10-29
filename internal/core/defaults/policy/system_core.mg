# System Shard Coordination
# Extracted from system.mg

# Decl imports (local definition of internal helpers)
# Intent Processing
# Moved to schemas_shards.mg
# Decl pending_intent(IntentID).
# Decl focus_needs_resolution(Ref).
# Decl intent_ready_for_executive(IntentID).

# Action Flow
# Moved to schemas_shards.mg
# Decl pending_permission_check(ActionID).
# Decl action_pending_permission(ActionID).
# Decl permission_checked(ActionID).
# Decl action_permitted(ActionID).
# Decl action_blocked(ActionID, Reason).
# Decl action_ready_for_routing(ActionID).
# Decl ready_for_routing(ActionID).
# Decl action_routed(ActionID).
# Decl routing_succeeded(ActionID).
# Decl routing_failed(ActionID, Error).

# System Health
# Moved to schemas_shards.mg
# Decl system_shard_healthy(ShardName).
# Decl shard_heartbeat_stale(ShardName).
# Decl escalation_needed(Target, Subject, Reason).
# Decl shard_startup(ShardName, Mode).

# Safety
# Moved to schemas_shards.mg
# Decl block_all_actions(Reason).
# Decl next_action(Action).
# Decl anomaly_investigated(AnomalyID).
# Decl repeated_violation_pattern(Pattern).
# Decl propose_safety_rule(Pattern).
# Decl activate_shard(ShardName).

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

# System Shard Health Monitoring

# System shard is healthy if heartbeat within threshold (30 seconds)
system_shard_healthy(ShardName) :-
    system_heartbeat(ShardName, _).

# Helper: check if shard has no recent heartbeat
shard_heartbeat_stale(ShardName) :-
    system_shard(ShardName, _),
    !system_shard_healthy(ShardName).

# Escalate if critical system shard is unhealthy
escalation_needed(/system_health, ShardName, "heartbeat_timeout") :-
    shard_heartbeat_stale(ShardName),
    shard_startup(ShardName, /auto).

# System shards that must auto-start
shard_startup(/perception_firewall, /auto).
shard_startup(/executive_policy, /auto).
shard_startup(/constitution_gate, /auto).
shard_startup(/world_model_ingestor, /on_demand).
shard_startup(/tactile_router, /on_demand).
shard_startup(/session_planner, /on_demand).

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

# On-Demand Shard Activation

# Activate world_model_ingestor when files change
activate_shard(/world_model_ingestor) :-
    modified(_),
    !system_shard_healthy(/world_model_ingestor).

# Activate tactile_router when actions are pending
activate_shard(/tactile_router) :-
    action_ready_for_routing(_),
    !system_shard_healthy(/tactile_router).

# Activate session_planner for campaigns or complex goals
activate_shard(/session_planner) :-
    current_campaign(_),
    !system_shard_healthy(/session_planner).

activate_shard(/session_planner) :-
    user_intent(/current_intent, _, /plan, _, _),
    !system_shard_healthy(/session_planner).
