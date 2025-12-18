# System Routing Logic
# Section 21 of Cortex Executive Policy

# Routing Table (Tactile Router)

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
