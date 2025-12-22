# System Shard Coordination - Core Logic
# Domain: System Health, Security, and Core Action Flow

Decl pending_intent(IntentID.Type<atom>).
Decl user_intent(IntentID.Type<atom>, Goal.Type<string>, Verb.Type<atom>, Target.Type<string>, Args.Type<string>).
Decl executive_processed_intent(IntentID.Type<atom>).
Decl focus_needs_resolution(Ref.Type<string>).
Decl focus_resolution(Ref.Type<string>, Res.Type<string>, Context.Type<string>, Score.Type<int64>).
Decl intent_ready_for_executive(IntentID.Type<atom>).

Decl pending_permission_check(ActionID.Type<string>).
Decl pending_action(ActionID.Type<string>, Type.Type<atom>, Args.Type<string>, Status.Type<atom>, Ts.Type<int64>).
Decl action_pending_permission(ActionID.Type<string>).
Decl permission_checked(ActionID.Type<string>).
Decl permission_check_result(ActionID.Type<string>, Decision.Type<atom>, Reason.Type<string>, Meta.Type<string>).
Decl action_permitted(ActionID.Type<string>).
Decl action_blocked(ActionID.Type<string>, Reason.Type<string>).
Decl action_ready_for_routing(ActionID.Type<string>).
Decl action_routed(ActionID.Type<string>).
Decl ready_for_routing(ActionID.Type<string>).
Decl routing_result(ActionID.Type<string>, Status.Type<atom>, Error.Type<string>, Tool.Type<atom>).
Decl routing_succeeded(ActionID.Type<string>).
Decl routing_failed(ActionID.Type<string>, Error.Type<string>).

Decl system_shard_healthy(ShardName.Type<atom>).
Decl system_heartbeat(ShardName.Type<atom>, Timestamp.Type<int64>).
Decl shard_heartbeat_stale(ShardName.Type<atom>).
Decl system_shard(ShardName.Type<atom>, Status.Type<atom>).
Decl escalation_needed(Domain.Type<atom>, Entity.Type<atom>, Reason.Type<string>).
Decl shard_startup(ShardName.Type<atom>, Mode.Type<atom>).

Decl block_all_actions(Reason.Type<string>).
Decl safety_violation(ID.Type<string>, Pattern.Type<string>, Severity.Type<string>, Ctx.Type<string>).
Decl next_action(Action.Type<atom>).
Decl security_anomaly(AnomalyID.Type<string>, Type.Type<string>, Info.Type<string>).
Decl anomaly_investigated(AnomalyID.Type<string>).
Decl investigation_result(AnomalyID.Type<string>, Result.Type<string>).
Decl repeated_violation_pattern(Pattern.Type<string>).
Decl violation_count(Pattern.Type<string>, Count.Type<int64>).
Decl propose_safety_rule(Pattern.Type<string>).

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

# Catch-all escalation handler for system signals
next_action(/escalate_to_user) :-
    escalation_needed(_, _, _).
