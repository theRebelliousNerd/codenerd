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
    tool_capabilities(_, Cap).

# Derive missing_tool_for if user intent requires a capability we don't have
missing_tool_for(/current_intent, Cap) :-
    user_intent(/current_intent, _, _, _, _),
    goal_requires(_, Cap),
    !has_capability(Cap).

# Trigger tool generation if tool is missing
next_action(/generate_tool) :-
    missing_tool_for(_, _).

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

# Need new tool when repeated failures suggest capability gap
capability_gap_detected(Cap) :-
    task_failure_reason(_, "missing_capability", Cap),
    task_failure_count(Cap, N),
    N >= 2.

# Tool generation is permitted (safety gate)
tool_generation_permitted(Cap) :-
    missing_tool_for(_, Cap),
    !tool_generation_blocked(Cap).

# Block tool generation for dangerous capabilities
tool_generation_blocked(Cap) :-
    dangerous_capability(Cap).

# Define dangerous capabilities that should never be auto-generated
dangerous_capability(/exec_arbitrary).
dangerous_capability(/network_unconstrained).
dangerous_capability(/system_admin).
dangerous_capability(/credential_access).

# Ouroboros next actions
next_action(/ouroboros_detect) :-
    capability_gap_detected(_).

next_action(/ouroboros_generate) :-
    tool_generation_permitted(_),
    !has_active_generation().

next_action(/ouroboros_compile) :-
    tool_source_ready(ToolName),
    tool_safety_verified(ToolName),
    !tool_compiled(ToolName),
    !tool_ready(ToolName).

next_action(/ouroboros_register) :-
    tool_compiled(ToolName),
    !is_tool_registered(ToolName),
    !tool_ready(ToolName).

# Track active tool generation (prevent parallel generations)
active_generation(ToolName) :-
    generation_state(ToolName, /in_progress).

# Helper for safe negation - true if any generation is in progress
has_active_generation() :-
    active_generation(_).

# Helper for safe negation - true if tool is registered
is_tool_registered(ToolName) :-
    tool_registered(ToolName, _).

# Tool lifecycle states
tool_lifecycle(ToolName, /detected) :-
    missing_tool_for(_, ToolName).

tool_lifecycle(ToolName, /generating) :-
    generation_state(ToolName, /in_progress).

tool_lifecycle(ToolName, /safety_check) :-
    tool_source_ready(ToolName),
    !tool_safety_verified(ToolName).

tool_lifecycle(ToolName, /compiling) :-
    tool_safety_verified(ToolName),
    !tool_compiled(ToolName).

tool_lifecycle(ToolName, /ready) :-
    tool_ready(ToolName).

# SECTION 12C: TOOL LEARNING AND OPTIMIZATION

# Tool quality tracking (quality on 0-100 scale)
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
    tool_needs_refinement(_),
    !has_active_refinement().

# Prevent parallel refinements
active_refinement(ToolName) :-
    refinement_state(ToolName, /in_progress).

# Helper for safe negation - true if any refinement is in progress
has_active_refinement() :-
    active_refinement(_).

# Learning pattern signals
learning_pattern_detected(ToolName, IssueType) :-
    tool_known_issue(ToolName, IssueType),
    issue_occurrence_count(ToolName, IssueType, Count),
    Count >= 3.

# Promote learnings to tool generation hints
tool_generation_hint(Capability, "add_pagination") :-
    learning_pattern_detected(_, /pagination),
    capability_similar_to(Capability, _).

tool_generation_hint(Capability, "increase_limits") :-
    learning_pattern_detected(_, /incomplete),
    capability_similar_to(Capability, _).

tool_generation_hint(Capability, "add_retry") :-
    learning_pattern_detected(_, /rate_limit),
    capability_similar_to(Capability, _).

# Track refinement success
refinement_effective(ToolName) :-
    tool_refined(ToolName, OldVersion, NewVersion),
    version_quality(ToolName, OldVersion, OldQuality),
    version_quality(ToolName, NewVersion, NewQuality),
    NewQuality > OldQuality.

# Escalate if refinement didn't help
escalate_to_user(ToolName, "refinement_ineffective") :-
    tool_refined(ToolName, _, _),
    tool_quality_poor(ToolName),
    refinement_count(ToolName, Count),
    Count >= 2.

# Section 49: Suppression Rules / Autopoiesis (ReviewerShard Beyond-SOTA)


# A finding is suppressed if there's a suppression rule for it
is_suppressed(Type, File, Line) :-
    suppressed_rule(Type, File, Line, _).

# Also suppress based on confidence score (learned from user feedback)
is_suppressed(Type, File, Line) :-
    suppression_confidence(Type, File, Line, Score),
    Score >= 80.

# Active Hypothesis Filtering

# Helper predicates for safe negation
has_suppression_unsafe_deref(File, Line) :-
    is_suppressed(/unsafe_deref, File, Line).

has_suppression_unchecked_error(File, Line) :-
    is_suppressed(/unchecked_error, File, Line).

# Active unsafe dereference hypotheses
active_hypothesis(/unsafe_deref, File, Line, Var) :-
    unsafe_deref(File, Var, Line),
    !has_suppression_unsafe_deref(File, Line).

# Active unchecked error hypotheses
active_hypothesis(/unchecked_error, File, Line, Var) :-
    unchecked_error(File, Var, Line),
    !has_suppression_unchecked_error(File, Line).
