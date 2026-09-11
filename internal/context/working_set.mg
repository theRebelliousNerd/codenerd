# Per-task working context. Loaded beside the canonical context_compilation.mg
# in a private evaluation scope; observations can never authorize an action.
Decl working_observation(ID, Entity, Revision, Kind, Step) bound [/string, /string, /string, /string, /number].
Decl working_revision(Entity, Revision) bound [/string, /string].
# Digest of the observation body. Two requests can differ in their arguments
# and still return the same observation (a read whose range snaps to the same
# code element), so identity of what came back is tracked beside identity of
# what was asked.
Decl working_digest(ID, Digest) bound [/string, /string].
Decl working_recent(ID) bound [/string].
# Observations whose native call/result pair is in the provider transcript
# this round. They are the model's own recent turns and are not repeated in
# the selected state.
Decl working_in_transcript(ID) bound [/string].
# Rounds of native call/result pairs kept in the provider transcript. The
# current round is always kept. Observed 2026-09-11: with only the current
# pair kept, every earlier read lived in the system prompt as an observation
# block and the model, seeing no turn of its own before this one, "located
# the insertion point" afresh on every round of a three-fact insertion and
# never wrote; three runs stalled at the read-only ceiling.
Decl working_transcript_rounds(N) bound [/number].
working_transcript_rounds(3).
Decl working_stale(ID) bound [/string].
Decl working_superseded(ID) bound [/string].
Decl working_selected(ID, Priority) bound [/string, /name].
Decl working_control(Cycle, FailedRounds) bound [/name, /number].
# The loop's report at each round boundary. Intent is /write for a
# write-oriented verb and /read otherwise; Rounds counts completed rounds;
# Writes counts durable writes so far; SinceWrite and SinceVerify count rounds
# since the last durable write and the last focused verification (equal to
# Rounds when there has been none).
Decl working_progress(Intent, Rounds, Writes, SinceWrite, SinceVerify) bound [/name, /number, /number, /number, /number].
Decl working_nudge_rounds(N) bound [/number].
Decl working_stall_rounds(N) bound [/number].
Decl working_stop(Reason) bound [/name].
Decl working_finalize(Reason) descr [doc("Exploration is over; the harness asks for the conclusion and runs verification. Not a stop and not a completion witness.")].
Decl working_nudge(Kind) descr [doc("Steering the loop appends to the round's last tool result: /implement, /verify or /conclude.")].
# The regime the loop runs the next round under. /commit: the model has had
# the implement nudge for a whole span and only read; the catalog offered to
# it narrows to the tools that make and verify a change (plus recall of what
# it already gathered), so its legal moves are to make the change, verify it,
# or conclude. Observed 2026-09-11: four runs of one insertion brief re-read
# the same four facts for 24 rounds each with the nudge in hand.
Decl working_regime(Regime) descr [doc("The loop's regime for the next round; /commit closes exploration on a change task that ignored the implement nudge for a span, or that wrote and then only read for a span, and stays until a verification.")].
Decl working_regime_now(Regime) bound [/name].
Decl working_commit_rounds(N) bound [/number].
Decl working_finalize_rounds(N) bound [/number].
Decl working_stopped() bound [].
Decl working_continue() descr [doc("Continuation requires no observed stall; it never means task completion.")].

# Policy constants. Rounds, not tool calls: a model that batches ten reads in
# one response and one that reads one file per response get the same span.
working_nudge_rounds(8).
working_commit_rounds(16).
working_finalize_rounds(16).
working_stall_rounds(24).

working_regime(/commit) :-
    working_progress(/write, Rounds, 0, _, _),
    working_commit_rounds(N), Rounds >= N.
# A change task that wrote and then neither wrote nor verified for a nudge
# span is exploring again; its reading closes the same way. Observed
# 2026-09-11: one edit, then sixteen reads of the same three files while the
# model announced the next edit every round, until the turn was finalized
# with two of the brief's three changes unmade.
working_regime(/commit) :-
    working_progress(/write, _, Writes, SinceWrite, SinceVerify), Writes > 0,
    working_nudge_rounds(N), SinceWrite >= N, SinceVerify >= N.
# Once closed, reading stays closed until a verification brings evidence the
# model has not seen; a write by itself lifts nothing. Observed 2026-09-11
# with the regime lifting on a write: a twelve-site signature change went
# one edit per cycle, each cycle eight rounds of re-reading, closure, three
# recalls and one edit.
working_regime(/commit) :-
    working_regime_now(/commit),
    working_progress(/write, _, _, _, SinceVerify), SinceVerify > 0.

working_stop(/repeated_cycle) :- working_control(/yes, _).
working_stop(/tool_failures) :- working_control(_, Failed), Failed >= 3.
# A change task that has only read for the whole stall span never started.
# Observed 2026-09-11: 300 reads in 25 minutes before a one-line edit the
# brief had named by file and line.
working_stop(/read_only_stall) :-
    working_progress(/write, Rounds, 0, _, _),
    working_stall_rounds(N), Rounds >= N.
# Negation needs a bound literal, and a wildcard in a negated atom does not
# exclude in this Mangle; project the stop reasons to arity 0 first.
working_stopped() :- working_stop(_).
working_continue() :- working_control(_, _), !working_stopped().

# A change task that wrote and then neither wrote nor verified for the
# finalize span, a whole commit span past the point its reading closed, is
# done exploring: the harness collects the conclusion and runs the build/test
# gate itself instead of waiting for the model to get round to it.
working_finalize(/verify_after_write) :-
    working_progress(/write, _, Writes, SinceWrite, SinceVerify), Writes > 0,
    working_finalize_rounds(N), SinceWrite >= N, SinceVerify >= N.

# Steering, well before the stop and finalize thresholds.
working_nudge(/implement) :-
    working_progress(/write, Rounds, 0, _, _),
    working_nudge_rounds(N), Rounds >= N.
working_nudge(/verify) :-
    working_progress(/write, _, Writes, SinceWrite, SinceVerify), Writes > 0,
    SinceWrite >= 3, SinceVerify >= 3.
working_nudge(/conclude) :-
    working_progress(/read, Rounds, _, _, _),
    working_nudge_rounds(N), Rounds >= N.

working_stale(ID) :-
    working_observation(ID, Entity, Old, _, _),
    working_revision(Entity, Current), Old != Current.

working_superseded(ID) :-
    working_observation(ID, Entity, Revision, Kind, Step),
    working_observation(_, Entity, Revision, Kind, Later), Step < Later.
# The same body observed again later is one observation, whatever the
# request looked like. Observed 2026-09-11: five copies of one 389-line
# region of a file, requested with ranges that all snapped to the same
# projection, filled a third of the working section.
working_superseded(ID) :-
    working_observation(ID, Entity, Revision, _, Step), working_digest(ID, Digest),
    working_observation(Other, Entity, Revision, _, Later), working_digest(Other, Digest),
    Step < Later.

working_selected(ID, Priority) :-
    working_observation(ID, Entity, _, _, _),
    should_include_context(Entity, Priority),
    !working_stale(ID), !working_superseded(ID), !working_in_transcript(ID).

working_selected(ID, /p100) :-
    working_observation(ID, _, _, _, _), working_recent(ID),
    !working_stale(ID), !working_superseded(ID), !working_in_transcript(ID).
