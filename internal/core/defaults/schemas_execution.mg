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
Decl test_state(State).

# test_type(Type)
# Type: /unit, /integration, /e2e (detected from test file patterns)
Decl test_type(Type).

# retry_count(Count)
Decl retry_count(Count).

# =============================================================================
# SECTION 10: ACTION & EXECUTION (§4.0)
# =============================================================================

# next_action(ActionType)
# ActionType: /read_error_log, /analyze_root_cause, /generate_patch, /run_tests,
#             /escalate_to_user, /complete, /interrogative_mode
Decl next_action(ActionType).

# action_details(ActionType, Payload)
Decl action_details(ActionType, Payload).

# safe_action(ActionType)
Decl safe_action(ActionType).

# action_mapping(IntentVerb, ActionType) - maps intent verbs to executable actions
# IntentVerb: /explain, /read, /search, /run, /test, /review, /fix, /refactor, etc.
# ActionType: /analyze_code, /fs_read, /search_files, /exec_cmd, /run_tests, etc.
Decl action_mapping(IntentVerb, ActionType).

