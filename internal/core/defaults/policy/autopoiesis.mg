# Autopoiesis & Learning
# Section 12, 12B, 12C of Cortex Executive Policy


# Detect repeated rejection pattern
preference_signal(Pattern) :-
    rejection_count(Pattern, N),
    N >= 3.

# Promote to long-term memory
promote_to_long_term(FactType, FactValue) :-
    preference_signal(Pattern),
    derived_rule(Pattern, FactType, FactValue).

# Autopoiesis: Missing Tool Detection
# Helper: derive when we HAVE a capability (for safe negation)
has_capability(Cap) :-
    tool_capability(_, Cap).

has_capability(Cap) :-
    tool_capabilities(_, Cap).

# Derive missing_tool_for if user intent requires a capability we don't have
missing_tool_for(/current_intent, Cap) :-
    user_intent(/current_intent, _, _, _, _),
    goal_requires(_, Cap),
    !has_capability(Cap).

# Trigger tool generation if tool is missing
next_action(/generate_tool) :-
    missing_tool_for(_, Cap),
    tool_generation_permitted(Cap).

# SECTION 12B: OUROBOROS LOOP - TOOL SELF-GENERATION

# Tool exists in registry
tool_exists(ToolName) :-
    tool_registered(ToolName, _).

# Tool is ready for execution (compiled and registered)
tool_ready(ToolName) :-
    tool_exists(ToolName),
    tool_hash(ToolName, _).

# Tool is available (registered and ready)
tool_available(ToolName) :-
    registered_tool(ToolName, _, _).

# Capability is available if any tool provides it
capability_available(Cap) :-
    tool_capability(_, Cap).

# Need new tool when capability missing and user explicitly requests it
explicit_tool_request(Cap) :-
    user_intent(/current_intent, /mutation, /generate_tool, Cap, _).

# Tool generation is permitted (safety gate)
tool_generation_permitted(Cap) :-
    missing_tool_for(_, Cap),
    !tool_generation_blocked(Cap).

# Why a capability is not generated. /dangerous: it must never be. /provided:
# a registered tool already provides it. The missing_tool_for a turn asserts
# in its control packet reaches delegate_task(/tool_generator, ...) directly
# (delegation.mg), past the !has_capability guard of the intent rule above, so
# a turn with no vet tool in view sent autopoiesis to build /go_vet while
# run_lint existed. A campaign tool-create task reads the reason and resolves
# at once: to the providing tool, or failed with the reason
# (internal/campaign/orchestrator_tool_create.go).
tool_generation_block_reason(Cap, /dangerous) :-
    dangerous_capability(Cap).

tool_generation_block_reason(Cap, /provided) :-
    has_capability(Cap).

tool_generation_blocked(Cap) :-
    tool_generation_block_reason(Cap, _).

# Define dangerous capabilities that should never be auto-generated
dangerous_capability(/exec_arbitrary).
dangerous_capability(/network_unconstrained).
dangerous_capability(/system_admin).
dangerous_capability(/credential_access).

next_action(/ouroboros_generate) :-
    tool_generation_permitted(_).

# Helper for safe negation - true if tool is registered
is_tool_registered(ToolName) :-
    tool_registered(ToolName, _).

# Tool lifecycle states
tool_lifecycle(ToolName, /detected) :-
    missing_tool_for(_, ToolName).

tool_lifecycle(ToolName, /ready) :-
    tool_ready(ToolName).

# SECTION 12C: TOOL LEARNING AND OPTIMIZATION

# Tool quality tracking (quality on 0-100 scale).
# Thresholds must stay INTEGER: schemas_tools.mg declares AvgQuality /number,
# and this fork's comparison builtins are int64-only — a float operand aborts
# the whole kernel fixpoint, not just this rule.
tool_quality_poor(ToolName) :-
    tool_learning(ToolName, Executions, _, AvgQuality),
    Executions >= 3,
    AvgQuality < 50.

tool_quality_acceptable(ToolName) :-
    tool_learning(ToolName, Executions, _, AvgQuality),
    Executions >= 3,
    AvgQuality >= 50,
    AvgQuality < 80.

tool_quality_good(ToolName) :-
    tool_learning(ToolName, Executions, _, AvgQuality),
    Executions >= 3,
    AvgQuality >= 80.

# Trigger refinement for poor quality tools
tool_needs_refinement(ToolName) :-
    tool_quality_poor(ToolName).

tool_needs_refinement(ToolName) :-
    tool_known_issue(ToolName, /pagination),
    tool_learning(ToolName, Executions, _, _),
    Executions >= 2.

tool_needs_refinement(ToolName) :-
    tool_known_issue(ToolName, /incomplete),
    tool_learning(ToolName, Executions, _, _),
    Executions >= 2.

# Next action for refinement
next_action(/refine_tool) :-
    tool_needs_refinement(_).

# Section 49: Suppression Rules / Autopoiesis (ReviewerShard Beyond-SOTA)


# Active Hypothesis Filtering

# Active unsafe dereference hypotheses
active_hypothesis(/unsafe_deref, File, Line, Var) :-
    unsafe_deref(File, Var, Line).

# Active unchecked error hypotheses
active_hypothesis(/unchecked_error, File, Line, Var) :-
    unchecked_error(File, Var, Line).
