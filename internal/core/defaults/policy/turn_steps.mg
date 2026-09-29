# Planned steps: whether a change task is divided before it runs
#
# A write-oriented turn can be divided into edit steps, each run as its own
# pass with the file named (internal/session/work_steps.go). Dividing costs a
# planning call, and it pays only when the brief names several places to
# change. Measured 2026-09-21: one campaign run made 13 planning calls, every
# one on a brief that named one site, and every one came back "one step";
# three other runs lost about 4 minutes each to planning calls that timed out
# before the task ran as one pass anyway.
#
# The executor measures the edit sites the brief names (a path with a
# directory, or a file that exists, with its line when the brief gives one)
# and asserts turn_brief_site; this decides. The threshold is the user's:
# session.step_plan_min_sites in .nerd/config.json.

Decl turn_brief_site(Turn, Path, Line) bound [/name, /string, /number].
Decl turn_brief_site_count(Turn, N) bound [/name, /number].
Decl turn_needs_step_plan(Turn) bound [/name].

config_param_required(/session, /session_step_plan_min_sites).

turn_brief_site_count(Turn, N) :-
    turn_brief_site(Turn, Path, Line)
    |> do fn:group_by(Turn), let N = fn:count().

turn_needs_step_plan(Turn) :-
    turn_verb(Turn, Verb),
    write_oriented_intent(Verb),
    turn_brief_site_count(Turn, N),
    config_param(/session_step_plan_min_sites, K),
    N >= K.

# What one planned step's pass did, and what the plan owes next.
#
# The executor measures; these rules decide (internal/session/work_steps.go).
# step_execution is one pass's outcome for one step: how many writes and tool
# calls it added. step_retried is asserted when the commit-regime pass is
# entered, so the retry is owed at most once. step_no_change_evidence is the
# text after a "NO CHANGE NEEDED:" line — Go reads the marker, because that
# is text, and asserts the evidence only when there is some. The marker with
# nothing after it is not evidence.
#
# A step that wrote nothing and has not been retried owes one more pass with
# reading closed (step_next_action /retry_commit). Coverage is per file: a
# step that still wrote nothing is covered by the earliest step that did
# write the same file, or by its own no-change evidence. Anything else is
# unresolved, and the plan's verdict is /incomplete. The error text stays in
# Go; the verdict is the rule.
#
# Negation goes through a projection. !step_execution(Turn, Step, _, 0, _)
# does not see the ground row (the same failure as !turn_failing_test in
# coder_safety.mg), and a wildcard in the negated atom is the same class of
# miss.

Decl step_execution(Turn, Step, File, Writes, Calls) bound [/name, /number, /string, /number, /number].
Decl step_no_change_evidence(Turn, Step, Evidence) bound [/name, /number, /string].
Decl step_retried(Turn, Step) bound [/name, /number].

Decl step_cover_candidate(Turn, Step, By) bound [/name, /number, /number].
Decl step_file_covered(Turn, Step, By) bound [/name, /number, /number].
Decl step_has_retried(Turn, Step) bound [/name, /number].
Decl step_has_no_change(Turn, Step) bound [/name, /number].
Decl step_has_cover(Turn, Step) bound [/name, /number].
Decl step_unresolved(Turn, Step, File) bound [/name, /number, /string].
Decl step_next_action(Turn, Step, Action) bound [/name, /number, /name].
Decl turn_has_step(Turn) bound [/name].
Decl turn_has_unresolved_step(Turn) bound [/name].
Decl turn_steps_verdict(Turn, Verdict) bound [/name, /name].

step_has_retried(Turn, Step) :- step_retried(Turn, Step).

step_next_action(Turn, Step, /retry_commit) :-
    step_execution(Turn, Step, _, 0, _),
    !step_has_retried(Turn, Step).

# The earliest other step that wrote this step's file. The candidate is its
# own predicate so the aggregate's body is one atom: a transform whose body
# had a second atom dropped the row (engine truth 020 item 7).
step_cover_candidate(Turn, Step, By) :-
    step_execution(Turn, Step, File, 0, _),
    step_execution(Turn, By, File, ByWrites, _),
    ByWrites > 0,
    By != Step.

step_file_covered(Turn, Step, Earliest) :-
    step_cover_candidate(Turn, Step, By)
    |> do fn:group_by(Turn, Step), let Earliest = fn:min(By).

step_has_no_change(Turn, Step) :- step_no_change_evidence(Turn, Step, _).
step_has_cover(Turn, Step) :- step_file_covered(Turn, Step, _).

step_unresolved(Turn, Step, File) :-
    step_execution(Turn, Step, File, 0, _),
    !step_has_no_change(Turn, Step),
    !step_has_cover(Turn, Step).

turn_has_step(Turn) :- step_execution(Turn, _, _, _, _).
turn_has_unresolved_step(Turn) :- step_unresolved(Turn, _, _).

turn_steps_verdict(Turn, /incomplete) :- turn_has_unresolved_step(Turn).
turn_steps_verdict(Turn, /complete) :-
    turn_has_step(Turn),
    !turn_has_unresolved_step(Turn).
