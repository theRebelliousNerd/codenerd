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
Decl working_stale(ID) bound [/string].
Decl working_superseded(ID) bound [/string].
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
# is a threshold, not a resource. It was a config key
# (core_limits.tool_loop_repeat_threshold) until 2026-09-18, removed because a
# user could tune it into a count ceiling; it is working.repeat_threshold now,
# and the checker refuses a value below 2, which is where it would stop being
# a repeat detector.
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

# The spans. Rounds, not tool calls: a model that batches ten reads in one
# response and one that reads one file per response get the same span. Each is
# the working section of .nerd/config.json (internal/config/working.go), as a
# config_param row the working set asserts before its first evaluation; it
# refuses to run while one is missing (config_param_missing). They were
# literals here until 2026-09-23 (sweep finding F8).
working_nudge_rounds(N) :- config_param(/working_nudge_rounds, N).
config_param_required(/working, /working_nudge_rounds).
working_commit_rounds(N) :- config_param(/working_commit_rounds, N).
config_param_required(/working, /working_commit_rounds).
working_finalize_rounds(N) :- config_param(/working_finalize_rounds, N).
config_param_required(/working, /working_finalize_rounds).
working_stall_rounds(N) :- config_param(/working_stall_rounds, N).
config_param_required(/working, /working_stall_rounds).
working_repeat_threshold(N) :- config_param(/working_repeat_threshold, N).
config_param_required(/working, /working_repeat_threshold).

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

# A repair attempt (internal/session/repair_loop.go) runs under /repair: its
# brief is a failure the harness already measured, with the failing output in
# hand, so one round of reading is the diagnosis and reading then closes for
# the rest of the attempt (F-REPAIR-1, pinned by
# TestRepairLoop_ReadThenEditConverges). The attempt ends at its first write,
# at a working_stop, or when the model stops calling tools -- not at a count of
# model calls (sweep finding F10: the six-call ceiling it replaces).
Decl working_repair_read_rounds(N) bound [/number].
working_repair_read_rounds(N) :- config_param(/working_repair_read_rounds, N).
config_param_required(/working, /working_repair_read_rounds).
working_regime(/commit) :-
    working_regime_now(/repair),
    working_progress(/write, Rounds, 0, _, _),
    working_repair_read_rounds(N), Rounds >= N.

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
working_structural_trials(N) :- config_param(/working_structural_trials, N).
config_param_required(/working, /working_structural_trials).
working_structural_miss_limit(N) :- config_param(/working_structural_miss_limit, N).
config_param_required(/working, /working_structural_miss_limit).
working_search_open() :-
    working_structural(Attempts, _), working_structural_trials(N), Attempts >= N.
working_search_open() :-
    working_structural(_, Misses), working_structural_miss_limit(N), Misses >= N.

# A repeated trace on a change task that has written nothing is a stall, and
# stops the turn. After a write it is the model re-checking work it has already
# made, and it finalizes instead (working_finalize(/repeat_after_write), below).
# Stopping it failed the task, and the campaign's task transaction then restored
# the pre-task snapshot: observed 2026-09-21 on campaign 7b853890, documents were
# written, read back and recalled until this rule fired, failed, deleted and
# rewritten from scratch -- 03-GAP-ANALYSIS.md three times -- while the
# policy's own verify nudge was asking for exactly that reading. A read task
# that repeats has finished reading, and finalizes too
# (working_finalize(/repeat_after_reading)).
working_stop(/repeated_cycle) :- working_control(/yes, _), working_progress(/write, _, 0, _, _).
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
# A turn that has written and then repeats itself is done: the harness collects
# the conclusion and runs the gates itself, with the write kept. The gates, not
# the repetition, decide whether the work stands.
working_finalize(/repeat_after_write) :-
    working_control(/yes, _),
    working_progress(_, _, Writes, _, _), Writes > 0.
# A read task's product is its conclusion, so a repeat means reading is done:
# the harness asks for the conclusion from what was gathered. Stopping it failed
# the task and threw the reading away: observed 2026-09-22 on campaign
# 7b853890, a /research task made 21 reads and recalls, repeated, was stopped,
# failed, and was retried from nothing.
# Only a repeat that read something: when every call in the repeating round
# failed (working_control(/yes, Failed), Failed > 0) nothing was gathered to
# conclude from, and the finalize asked the model for a conclusion one round
# before working_stop(/tool_failures) could name the failure -- the turn ended
# quietly with no error (e2e InfiniteToolLoop, 2026-09-23).
working_finalize(/repeat_after_reading) :-
    working_control(/yes, 0),
    working_progress(/read, _, 0, _, _).

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

# The context ledger. A working request carries every round of native
# call/result pairs since the last compaction, whole and byte-identical from one
# round to the next, so a provider's prefix cache covers all of it but the
# newest round. Until 2026-09-23 a request carried a sliding window of rounds
# and, at its tail, a section of selected observations regenerated every round;
# measured on 481 rounds (2026-09-22): 81% of each section was the round
# before's observations, each observation was sent 5.6 times, and none of the
# section was ever served from cache (22.3k of 55.4k input tokens a round
# uncached).
#
# working_ledger(Call, ID, Bytes, Round): a tool result the ledger carries whole
# -- the call it answers, the observation it holds, its size with any harness
# text appended to it, and the round it entered. working_round_now(N): the
# latest round in the ledger.
Decl working_ledger(Call, ID, Bytes, Round) bound [/string, /string, /number, /number].
Decl working_round_now(N) bound [/number].
Decl working_ledger_ceiling(Bytes) bound [/number].
working_ledger_ceiling(N) :- config_param(/working_ledger_ceiling, N).
config_param_required(/working, /working_ledger_ceiling).
Decl working_ledger_keep_rounds(N) bound [/number].
working_ledger_keep_rounds(N) :- config_param(/working_ledger_keep_rounds, N).
config_param_required(/working, /working_ledger_keep_rounds).

Decl working_ledger_bytes(Total) bound [/number].
working_ledger_bytes(Total) :-
    working_ledger(_, _, Bytes, _)
    |> do fn:group_by(), let Total = fn:sum(Bytes).

# Compaction is one step that moves results out behind their recall handles, so
# the cache breaks once an epoch instead of once a round.
Decl working_compact() descr [doc("The ledger outgrew its ceiling: this request moves results out behind recall handles.")].
working_compact() :-
    working_ledger_bytes(Total), working_ledger_ceiling(Ceiling), Total > Ceiling.

# What a compaction moves out: every result older than the kept rounds, and any
# stale or superseded observation -- stale evidence does not stand as current,
# and a covered read is carried by the read that covers it.
Decl working_evict(Call) bound [/string].
working_evict(Call) :-
    working_compact(), working_ledger(Call, _, _, Round),
    working_round_now(Now), working_ledger_keep_rounds(Keep),
    Cut = fn:minus(Now, Keep), Round <= Cut.
working_evict(Call) :-
    working_compact(), working_ledger(Call, ID, _, _), working_stale(ID).
working_evict(Call) :-
    working_compact(), working_ledger(Call, ID, _, _), working_superseded(ID).

# Restatement. An observation still in the ledger whose file changed after it
# was made is not rewritten -- that would break the cache from its round on --
# and is not left standing as current either: the harness appends the current
# text once, at the round the change is seen, and again only at a later
# revision. working_restated(ID, Revision) is what the harness has appended.
Decl working_restated(ID, Revision) bound [/string, /string].
Decl working_restated_now(ID) bound [/string].
working_restated_now(ID) :-
    working_restated(ID, Revision),
    working_observation(ID, Entity, _, _, _), working_revision(Entity, Revision).
Decl working_kept(ID) bound [/string].
working_kept(ID) :- working_ledger(Call, ID, _, _), !working_evict(Call).
Decl working_restate(ID) bound [/string].
working_restate(ID) :-
    working_kept(ID), working_stale(ID), !working_restated_now(ID).
