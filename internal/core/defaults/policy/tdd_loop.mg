# TDD Repair Loop
# Section 3 of Cortex Executive Policy


# State Transitions
next_action(/read_error_log) :-
    test_state(/failing),
    retry_count(N), N < 3.

next_action(/analyze_root_cause) :-
    test_state(/log_read).

next_action(/generate_patch) :-
    test_state(/cause_found).

next_action(/run_tests) :-
    test_state(/patch_applied).

next_action(/run_tests) :-
    test_state(/unknown),
    user_intent(/current_intent, _, /test, _, _).

# Surrender Logic - Escalate after 3 retries
next_action(/escalate_to_user) :-
    test_state(/failing),
    retry_count(N), N >= 3.

# Success state
next_action(/complete) :-
    test_state(/passing).
