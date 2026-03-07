# Routing Table (Tactile Router)
# Extracted from system.mg

# Decl imports
# Moved to schemas_shards.mg
# Decl routing_table(ActionType, Tool, RiskLevel).
# Decl tool_allowed(Tool, ActionType).
# Decl route_action(ActionID, Tool).
# Decl action_ready_for_routing(ActionID).
# Decl action_type(ActionID, ActionType).
# Decl tool_allowlist(Tool, Timestamp).
# Decl routing_blocked(ActionID, Reason).
# Decl has_tool_for_action(ActionType).
# Decl next_action(Action).
# Decl routing_failed(ActionID, Error).
# Decl escalation_needed(Target, Subject, Reason).

# Default routing table entries aligned with the live Go router.
# Tool names here should mirror router.go's ToolRoute.ToolName values.
routing_table(/fs_read, /fs_read, /low).
routing_table(/fs_write, /fs_write, /medium).
routing_table(/exec_cmd, /shell_exec, /high).
routing_table(/browser, /browser_tool, /high).
routing_table(/code_graph, /code_search, /low).
routing_table(/delegate_reviewer, /shard_manager, /low).
routing_table(/delegate_coder, /shard_manager, /medium).
routing_table(/delegate_researcher, /shard_manager, /low).
routing_table(/delegate_tester, /shard_manager, /low).
routing_table(/delegate_tool_generator, /shard_manager, /medium).

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
