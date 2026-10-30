# Commit Barrier & Interactive Diff Approval
# Section 6 and 15 of Cortex Executive Policy


# Cannot commit if there are errors
block_commit("Build Broken") :-
    diagnostic(/error, _, _, _, _).

block_commit("Tests Failing") :-
    test_state(/failing).

# Bridge review findings to diagnostics (Fix 15.5)
diagnostic(Severity, File, Line, "REVIEW", Message) :-
    review_finding(File, Line, Severity, _, Message).

# Fix Bug #10: The "Timeout = Permission" Trap
# Require explicit positive confirmation that checks actually ran
checks_passed() :-
    build_result(/true, _),
    test_state(/passing).

# Helper for safe negation
has_block_commit() :-
    block_commit(_).

# Safe to commit ONLY if checks passed AND no blocks exist
safe_to_commit() :-
    checks_passed(),
    !has_block_commit().

# REMOVED: a "Gauntlet Not Passed" block rule. Nothing ever recorded a
# gauntlet_result, so with the negation intact it blocked every commit after any
# modification; the Nemesis gauntlet runs in Go (internal/autopoiesis) and
# decides there, not through this barrier.

# Section 15: Interactive Diff Approval

# Require approval for dangerous mutations
requires_approval(MutationID) :-
    pending_mutation(MutationID, File, _, _),
    chesterton_fence_warning(File, _).

requires_approval(MutationID) :-
    pending_mutation(MutationID, File, _, _),
    impacted(File).

# Ask the user about a mutation that needs approval, unless a question is
# already open. (No producer ever recorded a mutation approval, so the approval
# test the rule once carried is removed; the open question is what gates it.)
any_awaiting_user_input(/yes) :- awaiting_user_input(_).

next_action(/ask_user) :-
    pending_mutation(MutationID, _, _, _),
    requires_approval(MutationID),
    !any_awaiting_user_input(/yes).
