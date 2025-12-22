# System Shard Coordination - Routing Logic
# Domain: Action Routing and Tools

Decl routing_table(ActionType.Type<atom>, Tool.Type<atom>, Priority.Type<atom>).
Decl tool_allowed(Tool.Type<atom>, ActionType.Type<atom>).
Decl tool_allowlist(Tool.Type<atom>, Status.Type<atom>).
Decl route_action(ActionID.Type<string>, Tool.Type<atom>).
Decl action_ready_for_routing(ActionID.Type<string>).
Decl action_type(ActionID.Type<string>, Type.Type<atom>).
Decl routing_blocked(ActionID.Type<string>, Reason.Type<string>).
Decl has_tool_for_action(ActionType.Type<atom>).
Decl next_action(Action.Type<atom>).
Decl routing_failed(ActionID.Type<string>, Error.Type<string>).
Decl activate_shard(ShardName.Type<atom>).
Decl system_shard_healthy(ShardName.Type<atom>).

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

# Activate tactile_router when actions are pending
activate_shard(/tactile_router) :-
    action_ready_for_routing(_),
    !system_shard_healthy(/tactile_router).
