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
