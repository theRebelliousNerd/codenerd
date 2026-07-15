# Shard Delegation & Tool Mapping
# Section 8 and 10 of Cortex Executive Policy


# Section 8: Shard Delegation
#
# Every user_intent-driven delegation is guarded by !wants_direct_answer()
# (policy/routing_arbitration.mg): when the user is asking a QUESTION, the
# kernel must not derive shard delegations from the same intent — the turn
# terminates in prose. Without this guard, "what is X?" classified as
# /research or /analyze spawned researcher/reviewer shards through the
# handleSystemDelegations path even after routing chose a direct answer.

# Delegate to researcher for init/explore
delegate_task(/researcher, "Initialize codebase analysis", /pending) :-
    user_intent(/current_intent, _, /init, _, _),
    !wants_direct_answer().

delegate_task(/researcher, Task, /pending) :-
    user_intent(/current_intent, _, /research, Task, _),
    !wants_direct_answer().

delegate_task(/researcher, Task, /pending) :-
    user_intent(/current_intent, /query, /explore, Task, _),
    !wants_direct_answer().

# Delegate to coder for coding tasks
delegate_task(/coder, Task, /pending) :-
    user_intent(/current_intent, /mutation, /implement, Task, _),
    !wants_direct_answer().

delegate_task(/coder, Task, /pending) :-
    user_intent(/current_intent, /mutation, /refactor, Task, _),
    !wants_direct_answer().

# /create, /fix, /write, /delete, /debug, /commit, /git also map to
# next_action(/delegate_coder) via action_mapping. Without matching
# delegate_task rules, `nerd run` printed "Next action: /delegate_coder"
# and exited 0 without ever spawning the coder (hollow success).
delegate_task(/coder, Task, /pending) :-
    user_intent(/current_intent, _, /create, Task, _),
    !wants_direct_answer().

delegate_task(/coder, Task, /pending) :-
    user_intent(/current_intent, _, /fix, Task, _),
    !wants_direct_answer().

delegate_task(/coder, Task, /pending) :-
    user_intent(/current_intent, _, /write, Task, _),
    !wants_direct_answer().

delegate_task(/coder, Task, /pending) :-
    user_intent(/current_intent, _, /delete, Task, _),
    !wants_direct_answer().

delegate_task(/coder, Task, /pending) :-
    user_intent(/current_intent, _, /debug, Task, _),
    !wants_direct_answer().

delegate_task(/coder, Task, /pending) :-
    user_intent(/current_intent, _, /commit, Task, _),
    !wants_direct_answer().

delegate_task(/coder, Task, /pending) :-
    user_intent(/current_intent, _, /git, Task, _),
    !wants_direct_answer().

# Delegate to tester for test tasks
delegate_task(/tester, Task, /pending) :-
    user_intent(/current_intent, _, /test, Task, _),
    !wants_direct_answer().

delegate_task(/tester, "Generate tests for impacted code", /pending) :-
    impacted(File),
    !test_coverage(File).

# Delegate to reviewer for review tasks
delegate_task(/reviewer, Task, /pending) :-
    user_intent(/current_intent, _, /review, Task, _),
    !wants_direct_answer().

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
# /audit is a workhorse_verb (routing_arbitration.mg) — the taxonomy maps
# "analyze/audit X" to /audit — but it had no action_mapping, so the nerd-run
# one-shot path derived no next_action and hard-failed ("no action derived from
# policy"). It is an inspection verb like /review and /analyze; route it to the
# reviewer. Non-side-effecting (a prose audit is a valid terminal answer), so it
# stays out of side_effecting_action below, matching /analyze.
action_mapping(/audit, /delegate_reviewer).

# Code mutation actions (delegate to coder shard)
action_mapping(/fix, /delegate_coder).
action_mapping(/refactor, /delegate_coder).
# F-ROUTE-3: /optimize, /migrate, /format, /scaffold are the /mutation verbs
# perception routes to the coder (taxonomy.go: /migrate:755, /optimize:760,
# /scaffold:785, /format:795 — all Category /mutation, ShardType /coder), yet
# none had an action_mapping. So `nerd run "optimize/migrate/format/scaffold
# ..."` derived no next_action and died with "no action derived from policy"
# (exit 1) — the same F-ROUTE gap /audit had. (/document is /mutation /coder
# too but is intentionally mapped to /delegate_researcher above.) Each routes
# to the coder via the next_action handoff (cmd_instruction.go:223-237) and,
# because /delegate_coder is a side_effecting_action, makes
# intent_requires_tool_call true so the coder must emit a tool_call rather than
# prose-only (anti-hollow).
action_mapping(/optimize, /delegate_coder).
action_mapping(/migrate, /delegate_coder).
action_mapping(/format, /delegate_coder).
action_mapping(/scaffold, /delegate_coder).
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

# Step 6: TOOL-LOOP TERMINATION stays in Go — deliberately NOT migrated.
# runToolLoop's three control branch points are all mechanical loop counters,
# not policy decisions:
#   (1) len(ToolCalls)==0        -> model-driven natural exit (final answer);
#   (2) iter < MaxToolIterations -> the iteration ceiling (Go for-loop bound);
#   (3) ToolCallsExecuted >= MaxToolCalls -> the per-call budget.
# Encoding (2)/(3) as a should_retry_tool_loop(Attempt, Max) rule would force a
# fast-changing counter fact (tool_loop_state) into the deductive store. That is
# the classic Datalog counter fallacy (unbounded/monotonic store growth) AND it
# is unsafe here specifically: SubAgents (internal/session/subagent.go) each
# build their own Executor over the SINGLE shared kernel passed by the Spawner
# (internal/session/spawner.go), and run their tool loops concurrently. A 0-ary
# should_retry_tool_loop() gate cannot discriminate which loop a tool_loop_state
# fact belongs to, so concurrent loops would clobber each other's counter — a
# cross-goroutine race that retract-before-assert cannot fix.
# The ONLY genuine policy decision in runToolLoop — whether a no-tool first turn
# is acceptable — is already migrated above (intent_requires_tool_call/1,
# queried at executor.go:intentRequiresToolCall). The Go `for iter < maxIter`
# bound remains the hard, race-free termination backstop. There is nothing
# sound left to migrate, so no rule is written (per the repo guardrail against
# unbounded recursion / counter facts, and to avoid a consumer-less dead rule).

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

# Step 5: MULTI-STEP CLASSIFICATION (decision moves to policy; extraction stays
# in Go). Go's detectMultiStepTask computes the individual signals from the
# (quote-stripped) input — campaign verb, multi-step keyword match, verb-count
# >= 3, compound-pattern regex — and asserts one multi_step_signal fact per
# detected signal. The regex/keyword/verb-count MATCHING itself stays in Go
# (Mangle is not used for fuzzy/NL pattern banks per the repo guardrails); only
# the combination DECISION lives here. Go queries is_multi_step and falls back
# to the legacy boolean if the kernel is unavailable or returns nothing.
#
# Tuned (routing reliability): the original rule ORed ALL signals, so a single
# weak keyword hit ("also", "then ", a numbered list in pasted text) decomposed
# trivial requests into multi-step shard pipelines. Strong signals (an explicit
# campaign verb, a compound action pattern like "fix X and test it") decide
# alone; the weak keyword signal only counts when corroborated by a high verb
# count.
is_multi_step() :-
    multi_step_signal(/campaign_verb).

is_multi_step() :-
    multi_step_signal(/compound_pattern).

is_multi_step() :-
    multi_step_signal(/keyword_match),
    multi_step_signal(/verb_count_high).

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
    user_intent(/current_intent, _, /review, Target, _),
    !wants_direct_answer().

delegate_task(/reviewer, Target, /pending) :-
    user_intent(/current_intent, _, /security, Target, _),
    !wants_direct_answer().

delegate_task(/reviewer, Target, /pending) :-
    user_intent(/current_intent, _, /analyze, Target, _),
    !wants_direct_answer().

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
