# Per-task working context. Loaded beside the canonical context_compilation.mg
# in a private evaluation scope; observations can never authorize an action.
Decl working_observation(ID, Entity, Revision, Kind, Step) bound [/string, /string, /string, /string, /number].
Decl working_revision(Entity, Revision) bound [/string, /string].
# Digest of the observation body. Two requests can differ in their arguments
# and still return the same observation (a read whose range snaps to the same
# code element), so identity of what came back is tracked beside identity of
# what was asked.
Decl working_digest(ID, Digest) bound [/string, /string].
# The line span a content read covered; only reads with a span assert one.
Decl working_span(ID, Start, End) bound [/string, /number, /number].
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
# How many rounds beyond working_transcript_rounds the transcript may grow
# before it is cut back to that count. A provider prefix cache covers a request
# only up to its first changed byte, and a window that drops its oldest round
# every round changes the first byte after the anchor every round, so no round
# of the transcript was ever served from cache: measured 2026-09-21, 767 of 854
# follow-up calls changed the prefix and 47% of their input was cached, against
# 83% where it held. With slack S the transcript is append-only for S rounds in
# every S+1 and the cut happens once. Zero restores the every-round slide.
Decl working_transcript_slack(N) bound [/number].
working_transcript_slack(3).
# The most the observations section of one working request may carry, in
# bytes. Within it working_selected/2 chooses what is shown; everything it
# leaves out stays recallable by id. It was a Go constant equal to the old
# transcript cap (256 KiB, ~64k tokens) until 2026-09-18, when one nerd fix run
# was measured sending 32k tokens on its first call and 80-103k by its last:
# the growth was this section filling with every file the turn had read. The
# window is not a bucket; half of that is room for a one-file change and the
# files around it, and a read that falls out is one recall away.
Decl working_section_ceiling(Bytes) bound [/number].
working_section_ceiling(131072).
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
# How many identical deterministic trace cycles make a loop. The loop measures
# the cycle (it is the only side that can see the tool trace) and reports the
# verdict as working_control/2; this is the span it measures against, and it
# lives here because it is a threshold, not a resource. It was a config key
# (core_limits.tool_loop_repeat_threshold) until 2026-09-18, which made a
# policy constant something a user could tune into a count ceiling.
Decl working_repeat_threshold(N) bound [/number].
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
working_repeat_threshold(2).

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

# Structural-first search. The structure index answers "where is X", "what is
# in this package", "who calls X" and "what does nothing use" in one call;
# grep answers them in a call per guess plus a read per hit. Measured
# 2026-09-21 on campaign 440585a6: 166 raw filesystem calls against 14
# structural ones, 5.5M input tokens for 45 KB of cited documents. So the raw
# search tools (grep, glob, list_files, search_code) stay withheld until the
# structural ones have been given a real trial, or have shown twice that they
# have no answer (a workspace the index cannot parse, a question about prose).
# read_file is never withheld: it is how a file with no element model is read.
Decl working_structural(Attempts, Misses) bound [/number, /number].
Decl working_structural_trials(N) bound [/number].
Decl working_structural_miss_limit(N) bound [/number].
Decl working_search_open() descr [doc("The raw search tools are offered. Underivable at the start of a loop, so search opens only on evidence.")].
working_structural_trials(4).
working_structural_miss_limit(2).
working_search_open() :-
    working_structural(Attempts, _), working_structural_trials(N), Attempts >= N.
working_search_open() :-
    working_structural(_, Misses), working_structural_miss_limit(N), Misses >= N.

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
# A later read of the same file at the same revision that covers an earlier
# read's span replaces it. Observed 2026-09-11: one region read five times as
# 760-830, 700-850, 768-815, 740-830 and 690-850 was five observations, and
# with two other files treated the same the section ran to 146 KB a round.
working_superseded(ID) :-
    working_observation(ID, Entity, Revision, _, Step), working_span(ID, Start, End),
    working_observation(Other, Entity, Revision, _, Later), working_span(Other, OtherStart, OtherEnd),
    Step < Later, OtherStart <= Start, OtherEnd >= End.

working_selected(ID, Priority) :-
    working_observation(ID, Entity, _, _, _),
    should_include_context(Entity, Priority),
    !working_stale(ID), !working_superseded(ID), !working_in_transcript(ID).

working_selected(ID, /p100) :-
    working_observation(ID, _, _, _, _), working_recent(ID),
    !working_stale(ID), !working_superseded(ID), !working_in_transcript(ID).
