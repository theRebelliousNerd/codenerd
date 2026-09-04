# Cortex 1.5.0 Schemas (EDB Declarations)
# Version: 1.5.0
# Philosophy: Logic determines Reality; the Model merely describes it.

# Modular Schema: EXECUTION
# Sections: 9, 10

# =============================================================================
# SECTION 9: TDD REPAIR LOOP STATE (§3.2)
# =============================================================================

# test_state(State)
# State: /failing, /log_read, /cause_found, /patch_applied, /passing, /unknown
# Priority: 95
# SerializationOrder: 4
Decl test_state(State) bound [/name].

# test_type(Type)
# Type: /unit, /integration, /e2e (detected from test file patterns)
Decl test_type(Type) bound [/name].

# retry_count(Count)
Decl retry_count(Count) bound [/number].

# task_status(TaskID, Status)
# Status: /pending, /in_progress, /completed, /blocked, /failed
Decl task_status(TaskID, Status) bound [/string, /name].

# =============================================================================
# SECTION 10: ACTION & EXECUTION (§4.0)
# =============================================================================

# next_action(ActionType)
# ActionType: /read_error_log, /analyze_root_cause, /generate_patch, /run_tests,
#             /escalate_to_user, /complete, /interrogative_mode
# Priority: 70
# SerializationOrder: 5
Decl next_action(ActionType) bound [/name].

# Strategy-Specific Next Actions (derived by strategy policy rules)
# tdd_next_action(ActionType) - TDD repair loop derived action
Decl tdd_next_action(ActionType) bound [/name].

# campaign_next_action(ActionType) - Campaign orchestration derived action
Decl campaign_next_action(ActionType) bound [/name].

# repair_next_action(ActionType) - Repair strategy derived action
Decl repair_next_action(ActionType) bound [/name].

# Blocking Conditions (derived by policy rules)
# block_action(Reason) - General action blocking condition
Decl block_action(Reason) bound [/name].

# test_state_blocking(Reason) - Test state prevents action
Decl test_state_blocking(Reason) bound [/name].

# action_details(ActionType, Payload)
Decl action_details(ActionType, Payload) bound [/name, /string].

# safe_action(ActionType)
Decl safe_action(ActionType) bound [/name].

# action_mapping(IntentVerb, ActionType) - maps intent verbs to executable actions
# IntentVerb: /explain, /read, /search, /run, /test, /review, /fix, /refactor, etc.
# ActionType: /analyze_code, /fs_read, /search_files, /exec_cmd, /run_tests, etc.
Decl action_mapping(IntentVerb, ActionType) bound [/name, /name].

# side_effecting_action(ActionType) - actions that mutate file system, process state,
# or otherwise require a real tool invocation to complete. Used by
# intent_requires_tool_call/1 to decide whether a narrative-only LLM response
# is acceptable for a given intent verb.
Decl side_effecting_action(ActionType) bound [/name].

# intent_requires_tool_call(IntentVerb) - derived predicate: true when the
# intent's verb maps to a side-effecting action. The session executor queries
# this after the first LLM turn — when true and zero tool_calls were emitted,
# it recompiles the prompt with the system/tool_nudge/no_tool_call_retry
# atom (selected by world state "no_tool_call_retry") and retries once.
Decl intent_requires_tool_call(IntentVerb) bound [/name].

# write_oriented_intent(IntentVerb) - verbs whose terminal contract requires a
# durable file mutation, not merely any successful tool invocation. The session
# executor uses this to retain write tools during forced finalization and to
# reject hollow prose-only completion.
Decl write_oriented_intent(IntentVerb) bound [/name].

# reasoning_intensive_action(ActionType) - actions whose OUTPUT QUALITY dominates
# their token cost: adversarial review, static analysis, policy authorship,
# planning. Distinct from side_effecting_action/1, which is about whether prose
# alone can complete the turn. An action can be one, both, or neither.
Decl reasoning_intensive_action(ActionType) bound [/name].

# reasoning_intensive_verb(IntentVerb) - verbs that are reasoning-intensive on
# their own, independent of any action_mapping. Orchestration verbs (/campaign,
# /assault) and speculative verbs (/dream, /shadow) live here because they plan
# rather than delegate to a single action.
Decl reasoning_intensive_verb(IntentVerb) bound [/name].

# intent_requires_reasoning_model(IntentVerb) - derived predicate: true when the
# turn should be served by the high-reasoning planner LLM slot rather than the
# cheap bulk worker slot. The session executor queries this once per turn and
# routes the whole tool loop accordingly. When no planner slot is configured the
# answer is ignored and everything stays on the worker.
Decl intent_requires_reasoning_model(IntentVerb) bound [/name].

# tool_invocation(ToolName, Input, Timestamp) - Tool execution record
# Records tool invocations for transparency/observability display
Decl tool_invocation(ToolName, Input, Timestamp) bound [/name, /string, /number].

# =============================================================================
# SECTION 11: POST-ACTION VALIDATION (§4.1)
# =============================================================================
# These predicates track the verification of action outcomes.
# Every action executed by VirtualStore is verified after execution
# to ensure it actually succeeded, not just returned without error.

# action_verified(ActionID, ActionType, Method, Confidence, Timestamp)
# Emitted when post-action validation succeeds.
# Method: /hash, /syntax, /existence, /content_check, /output_scan, /codedom_refresh, /paranoid_validation, /enhanced_edit_validation
# Confidence: 0-100 integer score (Go scales 0.0-1.0 to 0-100)
Decl action_verified(ActionID, ActionType, Method, Confidence, Timestamp) bound [/string, /name, /name, /number, /number].

# action_validation_failed(ActionID, ActionType, Reason, Details, Timestamp)
# Emitted when post-action validation fails.
# Triggers self-healing or escalation.
Decl action_validation_failed(ActionID, ActionType, Reason, Details, Timestamp) bound [/string, /name, /name, /string, /number].

# validation_method_used(ActionID, Method)
# Tracks which validation method was applied to each action.
Decl validation_method_used(ActionID, Method) bound [/string, /name].

# action_pre_state(ActionID, StateHash)
# Captures state before action execution (for rollback).
Decl action_pre_state(ActionID, StateHash) bound [/string, /string].

# action_post_state(ActionID, StateHash)
# Captures state after action execution (for verification).
Decl action_post_state(ActionID, StateHash) bound [/string, /string].

# action_state_delta(ActionID, PreHash, PostHash)
# Records the change in state from action execution.
Decl action_state_delta(ActionID, PreHash, PostHash) bound [/string, /string, /string].

# validation_attempt(ActionID, AttemptNum, Success, Timestamp)
# Tracks validation retry attempts.
Decl validation_attempt(ActionID, AttemptNum, Success, Timestamp) bound [/string, /number, /name, /number].

# validation_max_retries_reached(ActionID)
# Indicates self-healing exhausted retry budget.
Decl validation_max_retries_reached(ActionID) bound [/string].

# needs_self_healing(ActionID, HealingType)
# Triggers automatic recovery when validation fails.
# HealingType: /retry, /rollback, /escalate, /alternative_approach
Decl needs_self_healing(ActionID, HealingType) bound [/string, /name].

# Bound-negation helper. A negated literal containing an anonymous wildcard
# excludes nothing in this Mangle build (see internal/core/bound_negation_test.go);
# projecting the wildcard away makes the negation actually filter.
Decl action_needs_self_healing(ActionID) bound [/string].

# healing_attempt(ActionID, HealingType, Success, ErrorMsg, Timestamp)
# Records a self-healing attempt and its outcome.
Decl healing_attempt(ActionID, HealingType, Success, ErrorMsg, Timestamp) bound [/string, /name, /name, /string, /number].

# action_escalated(ActionID, Reason, Timestamp)
# Indicates an action was escalated to user for manual intervention.
Decl action_escalated(ActionID, Reason, Timestamp) bound [/string, /name, /number].

# --- Derived Validation Predicates (used in policy/validation.mg) ---

# action_failed_validation(ActionID) - derived: true when any validator reported failure
Decl action_failed_validation(ActionID) bound [/string].

# validation_syntax_error(ActionID) - derived: syntax validation failed (code corruption)
Decl validation_syntax_error(ActionID) bound [/string].

# unresolved_failure(ActionID) - derived: failed validation not yet resolved or escalated
Decl unresolved_failure(ActionID) bound [/string].


# =============================================================================
# SECTION 12: EXECUTION OUTCOMES (Integration Gaps)
# =============================================================================

# cmd_succeeded(Binary, Output)
Decl cmd_succeeded(Binary, Output) bound [/string, /string].

# cmd_failed(Binary, Error)
Decl cmd_failed(Binary, Error) bound [/string, /string].

# file_read_error(Path, Error)
Decl file_read_error(Path, Error) bound [/string, /string].

# file_write_error(Path, Error)
Decl file_write_error(Path, Error) bound [/string, /string].

# file_truncated(Path, Limit)

# dir_read(Path, Count)
Decl dir_read(Path, Count) bound [/string, /number].

# dir_read_error(Path, Error)
Decl dir_read_error(Path, Error) bound [/string, /string].

# edit_failed(Path, Reason)
Decl edit_failed(Path, Reason) bound [/string, /name].

# file_edited(Path)
Decl file_edited(Path) bound [/string].

# delete_blocked(Path, Reason)
Decl delete_blocked(Path, Reason) bound [/string, /name].

# file_deleted(Path)
Decl file_deleted(Path) bound [/string].

# file_read(Path, SessionID, Timestamp) - matches VirtualStore usage
# Override/Supplement schemas_codedom.mg usage
# Decl file_read(Path, SessionID, Timestamp).
# NOTE: file_read is already declared in schemas_codedom.mg as (Path, SessionID, Timestamp)
# But virtual_store_actions.go uses (Path, Size). We will fix the Go code.


# =============================================================================
# SECTION 13: AUDIT LOGGING (Tactile Executor)
# =============================================================================

# Duplicates removed (in schemas_shards.mg):
# execution_started, execution_command, execution_working_dir, execution_completed,
# execution_output, execution_success, execution_nonzero, execution_failure,
# execution_resource_usage, execution_io, execution_sandbox, execution_killed,
# execution_error, execution_blocked

Decl execution_sandboxed(RequestID, SandboxMode) bound [/string, /name].
Decl execution_tag(RequestID, Key, Value) bound [/string, /string, /string].


# Turn evidence is owned by policy/coder_safety.mg (Decls + derived
# hollow_success/turn_done); the session executor asserts it per turn.