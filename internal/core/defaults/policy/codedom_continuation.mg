# Code DOM Continuation Logic
# Multi-step task execution and progress tracking
#
# The continuation is a fixpoint over obligations, not a loop with a step
# ceiling. An obligation is RAISED by evidence the host observed about a shard
# result (Go source written with no test beside it, structured findings) and
# DISCHARGED by a shard result for the same description: the tester's result
# for a test obligation, the coder's for a fix obligation. A shard that
# reported its own step incomplete owes the rest of it once; a second identical
# attempt is a repeat, not progress. Nothing here joins on a status word to
# decide that work is owed -- until 2026-09-18 the tester step was reachable
# only from shard_result(_, /code_generated, /coder, _, _), which a verified
# turn was never written as, so a pending_test asserted on a /complete turn
# was an obligation no rule could reach (REVIEW-wave1 F8, contract C-03).

# --- Discharge ---

# The tester acted on this test obligation.
test_obligation_discharged(Description) :-
    shard_result(_, _, /tester, Description, _).

# The coder acted on this fix obligation.
fix_obligation_discharged(Description) :-
    shard_result(_, _, /coder, Description, _).

# The same shard has already produced two results for the same work: the
# incomplete step was retried once. A third attempt is the S3 repeat rule in
# continuation form -- the same call again is a stall, not progress.
incomplete_step_retried(ShardType, Description) :-
    shard_result(First, _, ShardType, Description, _),
    shard_result(Second, _, ShardType, Description, _),
    First != Second.

# --- Pending Subtask Detection ---

# Code was written with no test beside it -> the tester owes one, whatever
# verdict the kernel reached about the turn that wrote it.
has_pending_subtask(TaskID, Description, /tester) :-
    pending_test(TaskID, Description),
    !test_obligation_discharged(Description).

# Structured findings were reported against the change -> the coder owes a
# fix. The findings themselves reach the coder through the prior shard
# context; the description names the work they were found in.
has_pending_subtask(TaskID, Description, /coder) :-
    pending_fix(TaskID, Description),
    !fix_obligation_discharged(Description).

# Shard execution was incomplete -> continue with the same shard, once.
has_pending_subtask(TaskID, Description, ShardType) :-
    shard_result(TaskID, /incomplete, ShardType, Description, _),
    !incomplete_step_retried(ShardType, Description).

# --- Continuation Blocking Conditions ---

# User pressed Ctrl+X
continuation_blocked(/user_interrupted) :-
    interrupt_requested().

# Clarification is pending
continuation_blocked(/needs_clarification) :-
    pending_clarification(_, _, _).

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
