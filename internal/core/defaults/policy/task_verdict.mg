# What a caller does with a delegated task whose turn did not end /done
#
# A delegated task's turn ends with a verdict (the executor's /done, /hollow,
# /failed, /unverified) and, when it is not /done, the evidence the kernel
# found missing (turn_missing_evidence: /tests_not_green, /self_reported_incomplete,
# ...). Callers used to take the result as done anyway: the chat's background
# jobs printed "finished", a consultation returned the prose, `nerd spawn`
# printed it. Each now runs the task through session.RunUntilVerified, which
# records every dispatch here and asks this file what to do with the last one.
#
# task_dispatch(Loop, N, Outcome, Changed): dispatch N (from 1) of the loop
# ended with Outcome (/done, /unverified, ... or /no_verdict when the executor
# took none) and wrote Changed files. task_dispatch_gap(Loop, N, Gap) is one
# unmet evidence atom of that dispatch. task_dispatch_blocked(Loop, N, Pct) is
# Jev's reading of the dispatch's final report: how sure it is that the report
# names an obstacle codeNERD cannot clear (a permission denial, a missing
# credential or input, a choice only the user can make).
#
# task_loop_verdict(Loop, Verdict) is the one decision about the last dispatch:
#
#   /verified    it ended /done
#   /blocked     it did not, and its report names an obstacle: the person is told
#                what it is, and the task is not sent again
#   /unjudged    it did not, and the verdict names nothing to finish (the executor
#                took no verdict, or the unmet evidence is empty): there is nothing
#                to hand back
#   /stalled     it did not, and it is not the first: it wrote nothing and cleared
#                no gap the dispatch before it left
#   /redispatch  it did not, and the work is moving: the task goes out again with
#                what is missing
#
# There is no count: a loop ends when the work is verified, a blocker is named,
# a dispatch makes no progress, or the caller's own context ends.

Decl task_dispatch(Loop, N, Outcome, Changed) bound [/name, /number, /name, /number].
Decl task_dispatch_gap(Loop, N, Gap) bound [/name, /number, /name].
Decl task_dispatch_blocked(Loop, N, Pct) bound [/name, /number, /number].
Decl task_dispatch_seq(Loop, N) bound [/name, /number].
Decl task_dispatch_last(Loop, N) bound [/name, /number].
Decl task_dispatch_has_gap(Loop, N) bound [/name, /number].
Decl task_dispatch_verified(Loop) bound [/name].
Decl task_dispatch_open(Loop) bound [/name].
Decl task_dispatch_blocker(Loop) bound [/name].
Decl task_dispatch_unjudged(Loop) bound [/name].
Decl task_dispatch_moved(Loop) bound [/name].
Decl task_dispatch_stalled(Loop) bound [/name].
Decl task_loop_verdict(Loop, Verdict) bound [/name, /name].

task_dispatch_seq(Loop, N) :- task_dispatch(Loop, N, _, _).
task_dispatch_last(Loop, Last) :-
    task_dispatch_seq(Loop, N)
    |> do fn:group_by(Loop), let Last = fn:max(N).
task_dispatch_has_gap(Loop, N) :- task_dispatch_gap(Loop, N, _).

task_dispatch_verified(Loop) :-
    task_dispatch_last(Loop, N),
    task_dispatch(Loop, N, /done, _).
task_dispatch_open(Loop) :-
    task_dispatch_last(Loop, N),
    !task_dispatch_verified(Loop).

task_dispatch_blocker(Loop) :-
    task_dispatch_last(Loop, N),
    task_dispatch_blocked(Loop, N, Pct),
    config_param(/jev_yes_percent, Yes),
    Pct >= Yes.

task_dispatch_unjudged(Loop) :-
    task_dispatch_last(Loop, N),
    task_dispatch(Loop, N, /no_verdict, _).
task_dispatch_unjudged(Loop) :-
    task_dispatch_last(Loop, N),
    !task_dispatch_has_gap(Loop, N).

# Progress between two dispatches: the later one wrote, or cleared a gap the
# earlier one left.
task_dispatch_moved(Loop) :-
    task_dispatch_last(Loop, N),
    task_dispatch(Loop, N, _, Changed),
    Changed > 0.
task_dispatch_moved(Loop) :-
    task_dispatch_last(Loop, N),
    Before = fn:minus(N, 1),
    task_dispatch_gap(Loop, Before, Gap),
    !task_dispatch_gap(Loop, N, Gap).
task_dispatch_stalled(Loop) :-
    task_dispatch_last(Loop, N),
    N > 1,
    !task_dispatch_moved(Loop).

task_loop_verdict(Loop, /verified) :- task_dispatch_verified(Loop).
task_loop_verdict(Loop, /blocked) :-
    task_dispatch_open(Loop),
    task_dispatch_blocker(Loop).
task_loop_verdict(Loop, /unjudged) :-
    task_dispatch_open(Loop),
    !task_dispatch_blocker(Loop),
    task_dispatch_unjudged(Loop).
task_loop_verdict(Loop, /stalled) :-
    task_dispatch_open(Loop),
    !task_dispatch_blocker(Loop),
    !task_dispatch_unjudged(Loop),
    task_dispatch_stalled(Loop).
task_loop_verdict(Loop, /redispatch) :-
    task_dispatch_open(Loop),
    !task_dispatch_blocker(Loop),
    !task_dispatch_unjudged(Loop),
    !task_dispatch_stalled(Loop).
