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
# WARNING: This unbounded version can loop forever if dependency_link has cycles.
# Use symbol_reachable_bounded/3 with explicit depth limit for safety.
symbol_reachable(From, To) :-
    dependency_link(From, To, _).

symbol_reachable(From, To) :-
    dependency_link(From, Mid, _),
    symbol_reachable(Mid, To).

# Depth-bounded variant to prevent infinite recursion in cyclic graphs.
# MaxDepth should typically be 10-20 for most codebases.
symbol_reachable_bounded(From, To, MaxDepth) :-
    MaxDepth > 0,
    dependency_link(From, To, _).

symbol_reachable_bounded(From, To, MaxDepth) :-
    MaxDepth > 0,
    dependency_link(From, Mid, _),
    NextDepth = fn:minus(MaxDepth, 1),
    symbol_reachable_bounded(Mid, To, NextDepth).

# Convenience predicate with default depth limit of 15.
# Safe to use in place of symbol_reachable.
symbol_reachable_safe(From, To) :-
    symbol_reachable_bounded(From, To, 15).

# Routing Table (Tactile Router)

# Default routing table entries (can be extended via Autopoiesis)
routing_table(/fs_read, /read_file, /low).
routing_table(/fs_write, /write_file, /medium).
routing_table(/exec_cmd, /execute_command, /high).
routing_table(/browser, /browser_action, /high).
routing_table(/code_graph, /analyze_code, /low).

# Tool is allowed for action type
tool_allowed(Tool, ActionType) :-
    routing_table(ActionType, Tool, _),
    tool_allowlist(Tool, _).

# Route action to appropriate tool
route_action(ActionID, Tool) :-
    action_ready_for_routing(ActionID),
    action_type(ActionID, ActionType),
    tool_allowed(Tool, ActionType).

# Routing blocked if no tool available
routing_blocked(ActionID, "no_tool_available") :-
    action_ready_for_routing(ActionID),
    action_type(ActionID, ActionType),
    !has_tool_for_action(ActionType).

# Helper for safe negation
has_tool_for_action(ActionType) :-
    tool_allowed(_, ActionType).

# Recovery from routing failures
next_action(/pause_and_replan) :-
    routing_failed(_, "rate_limit_exceeded").

next_action(/escalate_to_user) :-
    routing_failed(_, "no_handler").

# Catch-all escalation handler for system signals
next_action(/escalate_to_user) :-
    escalation_needed(_, _, _).

# Session Planning (Session Planner)

# Agenda item is ready when dependencies complete
agenda_item_ready(ItemID) :-
    agenda_item(ItemID, _, _, /pending, _),
    !has_incomplete_dependency(ItemID).

# Helper for dependency checking
has_incomplete_dependency(ItemID) :-
    agenda_dependency(ItemID, DepID),
    agenda_item(DepID, _, _, Status, _),
    /completed != Status.

# Next agenda item: highest priority ready item
next_agenda_item(ItemID) :-
    agenda_item_ready(ItemID),
    !has_higher_priority_item(ItemID).

# Helper for priority ordering
has_higher_priority_item(ItemID) :-
    agenda_item(ItemID, _, Priority, _, _),
    agenda_item_ready(OtherID),
    OtherID != ItemID,
    agenda_item(OtherID, _, OtherPriority, _, _),
    priority_higher(OtherPriority, Priority).

# Checkpoint needed based on time or completion (10 minutes = 600 seconds)
checkpoint_due() :-
    checkpoint_needed().

next_action(/create_checkpoint) :-
    checkpoint_due().

# Blocked item triggers escalation after retries
agenda_item_escalate(ItemID, "max_retries_exceeded") :-
    agenda_item(ItemID, _, _, /blocked, _),
    item_retry_count(ItemID, Count),
    Count >= 3.

escalation_needed(/session_planner, ItemID, Reason) :-
    agenda_item_escalate(ItemID, Reason).

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

# Autopoiesis Integration for System Shards

# Unhandled case tracking (for rule learning)
unhandled_case_count(ShardName, Count) :-
    system_shard(ShardName, _),
    unhandled_case_count_computed(ShardName, Count).

# Trigger LLM for rule proposal when threshold reached
propose_new_rule(ShardName) :-
    unhandled_case_count(ShardName, Count),
    Count >= 3.

# Proposed rule needs human approval if low confidence (confidence on 0-100 scale)
rule_needs_approval(RuleID) :-
    proposed_rule(RuleID, _, _, Confidence),
    Confidence < 80.

# Auto-apply rule if high confidence (confidence on 0-100 scale)
auto_apply_rule(RuleID) :-
    proposed_rule(RuleID, _, _, Confidence),
    Confidence >= 80,
    !rule_applied(RuleID).

# Helper for safe negation
rule_applied(RuleID) :-
    applied_rule(RuleID, _).

# Learn from successful rule applications
learning_signal(/rule_success, RuleID) :-
    applied_rule(RuleID, Timestamp),
    rule_outcome(RuleID, /success, _).

# OODA Loop Coordination

# OODA phases: Observe → Orient → Decide → Act
ooda_phase(/observe) :-
    pending_intent(IntentID),
    !intent_ready_for_executive(IntentID).

ooda_phase(/orient) :-
    intent_ready_for_executive(IntentID),
    pending_intent(IntentID),
    !has_next_action().

ooda_phase(/decide) :-
    pending_intent(IntentID),
    intent_ready_for_executive(IntentID),
    has_next_action().

ooda_phase(/act) :-
    action_pending_permission(_).

ooda_phase(/act) :-
    action_ready_for_routing(_).

# Helper for OODA phase detection
has_next_action() :-
    next_action(_).

# Current OODA state for debugging/monitoring.
# Prefer the most advanced phase when multiple are simultaneously true.
current_ooda_phase(/act) :-
    ooda_phase(/act).

current_ooda_phase(/decide) :-
    ooda_phase(/decide),
    !ooda_phase(/act).

current_ooda_phase(/orient) :-
    ooda_phase(/orient),
    !ooda_phase(/act),
    !ooda_phase(/decide).

current_ooda_phase(/observe) :-
    ooda_phase(/observe),
    !ooda_phase(/act),
    !ooda_phase(/decide),
    !ooda_phase(/orient).

# OODA loop stalled detection (30 second threshold)
ooda_stalled("no_action_derived") :-
    pending_intent(_),
    ooda_timeout().

# Escalate stalled OODA loop
escalation_needed(/ooda_loop, "stalled", Reason) :-
    ooda_stalled(Reason).
