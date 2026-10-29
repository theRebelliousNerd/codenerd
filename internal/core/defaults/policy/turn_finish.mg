# Finishing what the report says is unfinished (the /finish round)
#
# A turn whose own final report said part of the task was not done used to
# close "not verified" and stop: turn_self_reported_incomplete withheld /done,
# and nothing sent the admission back to the model. In the 2026-10-28 dogfood
# run a coder said "no .nerd/agents files are checked in, so remediation runs
# on the next init" (they existed, gitignored) and the turn ended there.
#
# Jev reads the report (learnjudge.JudgeReport): does it say part of the task
# is not done, and does it name an obstacle codeNERD cannot clear by itself (a
# permission denial, a missing credential or input, a choice only the user can
# make)? The executor asserts the reading; this file decides. A report that
# admits unfinished work and names no such blocker is pushed back, until it no
# longer admits it, names a blocker, or a push-back changes nothing
# (turn_finish_idle: the attempt wrote nothing). An unsettled blocker reading
# is not a blocker: the next attempt can name it plainly.

Decl turn_report_reading(Turn, Unfinished, Blocked) bound [/name, /number, /number].
Decl turn_finish_idle(Turn) bound [/name].
Decl turn_report_unfinished(Turn) bound [/name].
Decl turn_report_blocked(Turn) bound [/name].
Decl turn_finish_pushback(Turn) bound [/name].

turn_report_unfinished(Turn) :-
    turn_report_reading(Turn, Unfinished, _),
    config_param(/jev_yes_percent, Yes),
    Unfinished >= Yes.

turn_report_blocked(Turn) :-
    turn_report_reading(Turn, _, Blocked),
    config_param(/jev_yes_percent, Yes),
    Blocked >= Yes.

turn_finish_pushback(Turn) :-
    turn_report_unfinished(Turn),
    !turn_report_blocked(Turn),
    !turn_finish_idle(Turn).
