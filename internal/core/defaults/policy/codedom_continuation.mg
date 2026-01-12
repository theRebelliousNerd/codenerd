# Code DOM Continuation Logic
# Multi-step task execution and progress tracking

# --- Pending Subtask Detection ---

# Code was generated but no tests exist → need tests
has_pending_subtask(TaskID, Description, /tester) :-
    shard_result(_, /code_generated, /coder, Task, _),
    pending_test(TaskID, Description).

# Changes were made but not reviewed → need review
has_pending_subtask(TaskID, Description, /reviewer) :-
    shard_result(_, /code_generated, /coder, Task, _),
    pending_review(TaskID, Description).

# Shard execution was incomplete → continue with same shard
has_pending_subtask(TaskID, Description, ShardType) :-
    shard_result(TaskID, /incomplete, ShardType, Description, _).

# Tests needed status → trigger tester
has_pending_subtask(TaskID, Description, /tester) :-
    shard_result(TaskID, /tests_needed, _, Description, _).

# Review needed status → trigger reviewer
has_pending_subtask(TaskID, Description, /reviewer) :-
    shard_result(TaskID, /review_needed, _, Description, _).

# --- Continuation Blocking Conditions ---

# User pressed Ctrl+X
continuation_blocked(/user_interrupted) :-
    interrupt_requested().

# Clarification is pending
continuation_blocked(/needs_clarification) :-
    pending_clarification(_, _, _).

# Max steps reached (safety limit)
continuation_blocked(/max_steps_reached) :-
    continuation_step(Current, _),
    max_continuation_steps(Limit),
    Current >= Limit.

# --- Auto-Continue Signal ---

# Should continue if there's pending work and not blocked
should_auto_continue() :-
    has_pending_subtask(_, _, _),
    !has_continuation_block().

# --- Helpers ---

# Helper: check if any blocking condition exists
has_continuation_block() :-
    continuation_blocked(_).

# Helper: check if we have any blocking condition
has_blocking_condition() :-
    continuation_blocked(_).

# Helper: count pending subtasks (for progress display)
pending_subtask_count(Count) :-
    pending_subtask_count_computed(Count).
