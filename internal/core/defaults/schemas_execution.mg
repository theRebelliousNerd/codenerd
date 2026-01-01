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
Decl test_state(State).

# test_type(Type)
# Type: /unit, /integration, /e2e (detected from test file patterns)
Decl test_type(Type).

# retry_count(Count)
Decl retry_count(Count).

# task_status(TaskID, Status)
# Status: /pending, /in_progress, /completed, /blocked, /failed
Decl task_status(TaskID, Status).

# =============================================================================
# SECTION 10: ACTION & EXECUTION (§4.0)
# =============================================================================

# next_action(ActionType)
# ActionType: /read_error_log, /analyze_root_cause, /generate_patch, /run_tests,
#             /escalate_to_user, /complete, /interrogative_mode
# Priority: 70
# SerializationOrder: 5
Decl next_action(ActionType).

# Strategy-Specific Next Actions (derived by strategy policy rules)
# tdd_next_action(ActionType) - TDD repair loop derived action
Decl tdd_next_action(ActionType).

# campaign_next_action(ActionType) - Campaign orchestration derived action
Decl campaign_next_action(ActionType).

# repair_next_action(ActionType) - Repair strategy derived action
Decl repair_next_action(ActionType).

# Blocking Conditions (derived by policy rules)
# block_action(Reason) - General action blocking condition
Decl block_action(Reason).

# test_state_blocking(Reason) - Test state prevents action
Decl test_state_blocking(Reason).

# action_details(ActionType, Payload)
Decl action_details(ActionType, Payload).

# safe_action(ActionType)
Decl safe_action(ActionType).

# action_mapping(IntentVerb, ActionType) - maps intent verbs to executable actions
# IntentVerb: /explain, /read, /search, /run, /test, /review, /fix, /refactor, etc.
# ActionType: /analyze_code, /fs_read, /search_files, /exec_cmd, /run_tests, etc.
Decl action_mapping(IntentVerb, ActionType).

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
Decl action_verified(ActionID, ActionType, Method, Confidence, Timestamp).

# action_validation_failed(ActionID, ActionType, Reason, Details, Timestamp)
# Emitted when post-action validation fails.
# Triggers self-healing or escalation.
Decl action_validation_failed(ActionID, ActionType, Reason, Details, Timestamp).

# validation_method_used(ActionID, Method)
# Tracks which validation method was applied to each action.
Decl validation_method_used(ActionID, Method).

# action_pre_state(ActionID, StateHash)
# Captures state before action execution (for rollback).
Decl action_pre_state(ActionID, StateHash).

# action_post_state(ActionID, StateHash)
# Captures state after action execution (for verification).
Decl action_post_state(ActionID, StateHash).

# action_state_delta(ActionID, PreHash, PostHash)
# Records the change in state from action execution.
Decl action_state_delta(ActionID, PreHash, PostHash).

# validation_attempt(ActionID, AttemptNum, Success, Timestamp)
# Tracks validation retry attempts.
Decl validation_attempt(ActionID, AttemptNum, Success, Timestamp).

# validation_max_retries_reached(ActionID)
# Indicates self-healing exhausted retry budget.
Decl validation_max_retries_reached(ActionID).

# needs_self_healing(ActionID, HealingType)
# Triggers automatic recovery when validation fails.
# HealingType: /retry, /rollback, /escalate, /alternative_approach
Decl needs_self_healing(ActionID, HealingType).

# healing_attempt(ActionID, HealingType, Success, ErrorMsg, Timestamp)
# Records a self-healing attempt and its outcome.
Decl healing_attempt(ActionID, HealingType, Success, ErrorMsg, Timestamp).

# action_escalated(ActionID, Reason, Timestamp)
# Indicates an action was escalated to user for manual intervention.
Decl action_escalated(ActionID, Reason, Timestamp).

