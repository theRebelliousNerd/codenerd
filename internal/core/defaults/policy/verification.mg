# Verification Loop
# Section 23 of Cortex Executive Policy


# Verification State Management

# Task has any quality violation
has_quality_violation(TaskID) :-
    quality_violation(TaskID, _).

# Task passed verification (successful with no violations)
verification_succeeded(TaskID) :-
    verification_attempt(TaskID, _, /success),
    !has_quality_violation(TaskID).

# Block further execution after max retries (3 attempts)
verification_blocked(TaskID) :-
    verification_attempt(TaskID, 3, /failure).

# Corrective Action Triggers

# Task needs corrective action if verification failed and not blocked
needs_corrective_action(TaskID) :-
    verification_attempt(TaskID, AttemptNum, /failure),
    AttemptNum < 3,
    !verification_blocked(TaskID).

# Specific corrective action based on violation type

# Mock code → needs research for real implementation
next_action(/corrective_research) :-
    current_task(TaskID),
    quality_violation(TaskID, /mock_code),
    needs_corrective_action(TaskID).

# Placeholder code → needs research for proper implementation
next_action(/corrective_research) :-
    current_task(TaskID),
    quality_violation(TaskID, /placeholder),
    needs_corrective_action(TaskID).

# Hallucinated API → needs documentation lookup
next_action(/corrective_docs) :-
    current_task(TaskID),
    quality_violation(TaskID, /hallucinated_api),
    needs_corrective_action(TaskID).

# Incomplete implementation → may need decomposition
next_action(/corrective_decompose) :-
    current_task(TaskID),
    quality_violation(TaskID, /incomplete),
    needs_corrective_action(TaskID).

# Missing error handling → needs docs/examples
next_action(/corrective_docs) :-
    current_task(TaskID),
    quality_violation(TaskID, /missing_errors),
    needs_corrective_action(TaskID).

# Fake tests → needs research on testing patterns
next_action(/corrective_research) :-
    current_task(TaskID),
    quality_violation(TaskID, /fake_tests),
    needs_corrective_action(TaskID).

# Escalation Logic

# Escalation required when max retries exceeded
escalation_required(TaskID, "max_retries_exceeded") :-
    verification_blocked(TaskID),
    current_task(TaskID).

# Escalation triggers next_action
next_action(/escalate_to_user) :-
    escalation_required(_, _).

# Block all other actions during escalation
block_all_actions("verification_escalation") :-
    escalation_required(_, _).

# Learning Signals from Quality Violations

# Learn to avoid mock code patterns
quality_signal(/avoid_mock_code) :-
    quality_violation(_, /mock_code).

# Learn to avoid placeholder patterns
quality_signal(/avoid_placeholders) :-
    quality_violation(_, /placeholder).

# Learn to avoid hallucinated APIs
quality_signal(/avoid_hallucinated_api) :-
    quality_violation(_, /hallucinated_api).

# Learn to avoid incomplete implementations
quality_signal(/avoid_incomplete) :-
    quality_violation(_, /incomplete).

# Learn to avoid fake tests
quality_signal(/avoid_fake_tests) :-
    quality_violation(_, /fake_tests).

# Learn to include error handling
quality_signal(/require_error_handling) :-
    quality_violation(_, /missing_errors).

# Promote learning signals to long-term memory after repeated violations
promote_to_long_term(/quality_pattern, ViolationType) :-
    quality_violation(Task1, ViolationType),
    quality_violation(Task2, ViolationType),
    quality_violation(Task3, ViolationType),
    Task1 != Task2,
    Task2 != Task3,
    Task1 != Task3.

# Shard Selection for Retries

# Prefer specialist shards for complex tasks after failure
delegate_task(/specialist, TaskID, /pending) :-
    current_task(TaskID),
    verification_attempt(TaskID, AttemptNum, /failure),
    AttemptNum >= 1,
    shard_profile(_, /specialist, _).

# Delegate to researcher when documentation is needed
delegate_task(/researcher, Query, /pending) :-
    current_task(TaskID),
    corrective_query(TaskID, _, Query),
    corrective_action_taken(TaskID, /research).

# Delegate to researcher when docs lookup needed
delegate_task(/researcher, Query, /pending) :-
    current_task(TaskID),
    corrective_query(TaskID, _, Query),
    corrective_action_taken(TaskID, /docs).

# Verification Strategy Selection

# Activate verification strategy when task has been executed
active_strategy(/verification_loop) :-
    current_task(TaskID),
    verification_attempt(TaskID, _, _).

# Block normal task execution when in verification failure state
executive_blocked("verification_in_progress", Now) :-
    current_task(TaskID),
    needs_corrective_action(TaskID),
    current_time(Now).

# Quality Gate Integration with Commit Barrier

# Block commit if any task has unresolved quality violations
block_commit("Quality Violations") :-
    has_quality_violation(_).

# Block commit if verification is blocked (max retries reached)
block_commit("Verification Failed") :-
    verification_blocked(_).

# Context Enrichment Tracking

# Track successful corrective actions for learning
corrective_action_effective(TaskID, ActionType) :-
    corrective_action_taken(TaskID, ActionType),
    verification_attempt(TaskID, AttemptNum, /failure),
    verification_attempt(TaskID, NextAttempt, /success),
    NextAttempt > AttemptNum.

# Learn from effective corrective actions
learning_signal(/effective_correction, ActionType) :-
    corrective_action_effective(_, ActionType).

# Verification Metrics (for monitoring)

# Helper: track tasks that succeeded on first attempt
first_attempt_success(TaskID) :-
    verification_attempt(TaskID, 1, /success),
    !has_quality_violation(TaskID).

# Helper: track tasks that required retries
required_retry(TaskID) :-
    verification_attempt(TaskID, AttemptNum, /success),
    AttemptNum > 1.

# Helper: track specific violation types for analytics
violation_type_count_high(ViolationType) :-
    quality_violation(T1, ViolationType),
    quality_violation(T2, ViolationType),
    quality_violation(T3, ViolationType),
    quality_violation(T4, ViolationType),
    quality_violation(T5, ViolationType),
    T1 != T2, T2 != T3, T3 != T4, T4 != T5.

# Trigger rule proposal for high-frequency violations
propose_new_rule(/verification_policy) :-
    violation_type_count_high(_).
