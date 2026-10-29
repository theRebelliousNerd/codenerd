# Repair episodes: what a red build or test run gets next
#
# When a turn's edits leave the build or the tests red, the executor runs a
# repair episode (internal/session/repair_loop.go): attempts, each a
# read-diagnose-edit pass on the failing output followed by a recheck. What
# happens after a failed recheck was decided in Go -- a for-loop bound, and
# `if !wrote { commit }` -- beside the working policy that decides the same
# kinds of question for the main loop (sweep finding F10, 2026-09-22). The
# executor now records each attempt and asks here.
#
# repair_attempt(Episode, Attempt, Wrote, Failure): attempt number Attempt
# (from 1) of the episode, whether it made a durable write (/true or /false),
# and the digest of the failure its recheck reported (durations removed, so
# the same failure hashes the same). A passing recheck ends the episode in Go;
# only failed attempts are recorded.

Decl repair_attempt(Episode, Attempt, Wrote, Failure) bound [/name, /number, /name, /string].
Decl repair_attempt_count(Episode, N) bound [/name, /number].
Decl repair_exhausted(Episode) bound [/name].
Decl repair_not_converging(Episode) bound [/name].
Decl repair_gives_up(Episode) bound [/name].
Decl repair_move(Episode, Move) bound [/name, /name].
Decl repair_closed(Episode) bound [/name].
# repair_restart(Episode, Attempt): the episode restarted after attempt
# Attempt -- its edits undone, the next attempt told to name a different cause.
# Asserted by the executor when it carries out the /restart move.
Decl repair_restart(Episode, Attempt) bound [/name, /number].
Decl repair_has_restarted(Episode) bound [/name].
Decl repair_should_restart(Episode) bound [/name].
Decl repair_not_converging_since_restart(Episode) bound [/name].

config_param_required(/session, /session_repair_max_attempts).

repair_attempt_count(E, N) :-
    repair_attempt(E, A, W, D)
    |> do fn:group_by(E), let N = fn:count().

# The user's cap on attempts (session.repair_max_attempts).
repair_exhausted(E) :-
    repair_attempt_count(E, N),
    config_param(/session_repair_max_attempts, M),
    N >= M.

# An edit that leaves exactly the failure an earlier attempt already saw has
# not moved the episode: two different edits, the same red run. That repeated
# failure ends the loop, where before only the count did. The failure that
# opened the episode is not an attempt's: a first edit that changes nothing
# gets another try.
repair_not_converging(E) :-
    repair_attempt(E, A1, _, D),
    repair_attempt(E, A2, /true, D),
    A1 < A2.

# A repeated failure first restarts the episode, once. Two edits that leave
# exactly the same red run are two edits aimed at a cause that is not the
# cause: grinding a third attempt from the same state repeats the mistake, and
# giving up throws away an attempt the cap still allows. R1-12 spent 746.7k
# input tokens across three attempts with no edit that moved the failure. The
# restart undoes the episode's edits and asks for a different cause; a
# failure that repeats again after it gives up, as before.
repair_has_restarted(E) :- repair_restart(E, _).
repair_should_restart(E) :-
    repair_not_converging(E), !repair_has_restarted(E), !repair_exhausted(E).
repair_not_converging_since_restart(E) :-
    repair_restart(E, R),
    repair_attempt(E, A1, _, D),
    repair_attempt(E, A2, /true, D),
    R < A1, A1 < A2.

repair_gives_up(E) :- repair_exhausted(E).
repair_gives_up(E) :- repair_not_converging_since_restart(E).

repair_move(E, /give_up) :- repair_gives_up(E).
repair_move(E, /restart) :- repair_should_restart(E).
repair_move(E, /retry) :- repair_attempt_count(E, _), !repair_gives_up(E), !repair_should_restart(E).

# An attempt that made no edit read instead of repairing; every later attempt
# of the episode runs with reading closed, carrying the failing output.
repair_closed(E) :- repair_attempt(E, _, /false, _).
