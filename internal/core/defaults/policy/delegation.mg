# Shard Delegation & Tool Mapping
# Section 8 and 10 of Cortex Executive Policy


# Section 8: Shard Delegation

# Delegate to researcher for init/explore
delegate_task(/researcher, "Initialize codebase analysis", /pending) :-
    user_intent(/current_intent, _, /init, _, _).

delegate_task(/researcher, Task, /pending) :-
    user_intent(/current_intent, _, /research, Task, _).

delegate_task(/researcher, Task, /pending) :-
    user_intent(/current_intent, /query, /explore, Task, _).

# Delegate to coder for coding tasks
delegate_task(/coder, Task, /pending) :-
    user_intent(/current_intent, /mutation, /implement, Task, _).

delegate_task(/coder, Task, /pending) :-
    user_intent(/current_intent, /mutation, /refactor, Task, _).

# Delegate to tester for test tasks
delegate_task(/tester, Task, /pending) :-
    user_intent(/current_intent, _, /test, Task, _).

delegate_task(/tester, "Generate tests for impacted code", /pending) :-
    impacted(File),
    !test_coverage(File).

# Delegate to reviewer for review tasks
delegate_task(/reviewer, Task, /pending) :-
    user_intent(/current_intent, _, /review, Task, _).

# Section 10: Tool Capability Mapping & Action Mapping

# Tool capabilities for spreading activation
tool_capabilities(/fs_read, /read).
tool_capabilities(/fs_write, /write).
tool_capabilities(/exec_cmd, /execute).
tool_capabilities(/browser, /navigate).
tool_capabilities(/browser, /click).
tool_capabilities(/browser, /type).
tool_capabilities(/code_graph, /analyze).
tool_capabilities(/code_graph, /dependencies).

# Goal capability requirements
goal_requires(Goal, /read) :-
    user_intent(/current_intent, /query, _, Goal, _).

goal_requires(Goal, /write) :-
    user_intent(/current_intent, /mutation, _, Goal, _).

goal_requires(Goal, /execute) :-
    user_intent(/current_intent, _, /run, Goal, _).

goal_requires(Goal, /analyze) :-
    user_intent(/current_intent, _, /explain, Goal, _).

# Action Mappings: Map intent verbs to executable actions
# Core actions
action_mapping(/explain, /analyze_code).
action_mapping(/read, /fs_read).
action_mapping(/search, /search_files).
action_mapping(/run, /exec_cmd).
action_mapping(/test, /run_tests).

# Code review & analysis actions (delegate to reviewer shard)
action_mapping(/review, /delegate_reviewer).
action_mapping(/review_enhance, /delegate_reviewer).
action_mapping(/security, /delegate_reviewer).
action_mapping(/analyze, /delegate_reviewer).

# Code mutation actions (delegate to coder shard)
action_mapping(/fix, /delegate_coder).
action_mapping(/refactor, /delegate_coder).
action_mapping(/create, /delegate_coder).
action_mapping(/delete, /delegate_coder).
action_mapping(/write, /fs_write).
# /document is documentation/prose, not code generation. Earlier this
# routed to /delegate_coder which contradicted persona(/researcher) :-
# user_intent(_, _, /document, _, _) in internal/mangle/intent_routing.mg
# and produced a planning-only failure: the coder shard tried to "plan
# how to document" a non-existent file instead of writing the markdown.
# Routing to /delegate_researcher matches the persona rule and uses the
# researcher shard's write_file allowance for prose output.
action_mapping(/document, /delegate_researcher).
action_mapping(/commit, /delegate_coder).

# Debug actions
action_mapping(/debug, /delegate_coder).

# Git actions (delegate to coder shard for safe git operations)
action_mapping(/git, /delegate_coder).

# Research actions (delegate to researcher shard)
action_mapping(/research, /delegate_researcher).
action_mapping(/explore, /delegate_researcher).

# Autopoiesis/Tool generation actions (delegate to tool_generator shard)
action_mapping(/generate_tool, /delegate_tool_generator).
action_mapping(/refine_tool, /delegate_tool_generator).
action_mapping(/list_tools, /delegate_tool_generator).
action_mapping(/tool_status, /delegate_tool_generator).

# Diff actions
action_mapping(/diff, /show_diff).

# =============================================================================
# Side-effecting actions: producing file/process/system mutations or requiring
# real tool execution to make any observable progress. The executor uses this
# set (via intent_requires_tool_call/1) to decide whether the LLM's
# narrative-only first turn is acceptable. Delegation actions are included
# because the delegated shard's purpose is to mutate state — the orchestrating
# turn must still emit a tool_call (write_file / edit_file / run_command /
# delegate_*). Pure analysis verbs (/explain, /analyze, /research) intentionally
# omitted: a prose answer there is a valid terminal response.
side_effecting_action(/delegate_coder).
side_effecting_action(/delegate_researcher).
side_effecting_action(/delegate_tool_generator).
side_effecting_action(/fs_write).
side_effecting_action(/exec_cmd).
side_effecting_action(/run_tests).
side_effecting_action(/show_diff).

# Derive whether the current intent's verb requires a tool_call this turn.
# Used by internal/session/executor.go:runToolLoop to gate the no-tool retry.
intent_requires_tool_call(Verb) :-
    action_mapping(Verb, Action),
    side_effecting_action(Action).

# Step 4: DELEGATION DECISION (confidence gate moved from Go to policy).
# Go computes the verb->shard lookup and the perception confidence and asserts
# delegation_candidate(/current_intent, ShardType, Conf). This rule applies the
# gate: delegate only to a real shard (not /none) when confidence is at least 50
# (== the legacy intent.Confidence >= 0.5 boundary). The verb->shard LOOKUP
# itself stays in Go (GetShardTypeForVerb) because its source data (verb_def) is
# siloed in the perception taxonomy engine, not this kernel; only the DECISION
# migrates. Go queries should_delegate and falls back to the legacy boolean if
# the kernel is unavailable or returns nothing.
should_delegate(ShardType) :-
    delegation_candidate(/current_intent, ShardType, Conf),
    /none != ShardType,
    Conf >= 50.

# Derive next_action from intent and mapping
# Guard: Only derive if intent hasn't been processed by executive (prevents infinite loop)
next_action(Action) :-
    user_intent(/current_intent, _, Verb, _, _),
    action_mapping(Verb, Action),
    !executive_processed_intent(/current_intent).

# Specific file system actions (with same guard)
next_action(/fs_read) :-
    user_intent(/current_intent, _, /read, _, _),
    !executive_processed_intent(/current_intent).

next_action(/fs_write) :-
    user_intent(/current_intent, _, /write, _, _),
    !executive_processed_intent(/current_intent).

# Review delegation - high confidence triggers immediate delegation
delegate_task(/reviewer, Target, /pending) :-
    user_intent(/current_intent, _, /review, Target, _).

delegate_task(/reviewer, Target, /pending) :-
    user_intent(/current_intent, _, /security, Target, _).

delegate_task(/reviewer, Target, /pending) :-
    user_intent(/current_intent, _, /analyze, Target, _).

# Tool generator delegation - autopoiesis operations
delegate_task(/tool_generator, Target, /pending) :-
    user_intent(/current_intent, _, /generate_tool, Target, _).

delegate_task(/tool_generator, Target, /pending) :-
    user_intent(/current_intent, _, /refine_tool, Target, _).

delegate_task(/tool_generator, "", /pending) :-
    user_intent(/current_intent, _, /list_tools, _, _).

delegate_task(/tool_generator, Target, /pending) :-
    user_intent(/current_intent, _, /tool_status, Target, _).

# Auto-delegate when missing capability detected (implicit tool generation)
delegate_task(/tool_generator, Cap, /pending) :-
    missing_tool_for(_, Cap),
    !tool_generation_blocked(Cap).
