# Delivering the whole request (the chat's delivery rounds)
#
# A delegated or multi-step chat turn used to end "Ran N step(s); the work is
# not verified ... (no acceptance contract)" and stop, even when the steps had
# done half of what was asked: a shard can say "done" and miss a part, and the
# /finish round (turn_finish.mg) reads only the shard's own admission.
#
# When the continuation has no obligation left (codedom_continuation.mg), the
# chat splits the person's request into candidate fragments and Jev judges,
# per fragment, whether it was asked for and whether the steps' reports and
# changed files deliver it, and whether the reports name an obstacle codeNERD
# cannot clear by itself (learnjudge.CheckDelivery). Go asserts the reading as
# round R of the request; this file decides. A missing part is handed back as
# one more step (the offer Go builds from delivery_fragment_missing), until
# nothing is missing, a blocker is named, or a round resolves none of the
# parts the round before left missing. An unsettled part is reported, never
# pushed: the state did not settle it, and another step would not either.
# Only the request the chat is working on (delivery_active) can raise a step.

Decl delivery_active(Request) bound [/name].
Decl delivery_reading(Request, Round, Fragment, Asked, Delivered) bound [/name, /number, /string, /number, /number].
Decl delivery_blocker_reading(Request, Round, Blocked) bound [/name, /number, /number].
Decl delivery_result_cut(Request, Round) bound [/name, /number].
Decl delivery_step_offer(Request, Round, TaskID, Description, ShardType) bound [/name, /number, /string, /string, /name].

Decl delivery_round(Request, Round) bound [/name, /number].
Decl delivery_asked(Request, Round, Fragment) bound [/name, /number, /string].
Decl delivery_fragment_delivered(Request, Round, Fragment) bound [/name, /number, /string].
Decl delivery_fragment_missing(Request, Round, Fragment) bound [/name, /number, /string].
Decl delivery_fragment_unsettled(Request, Round, Fragment) bound [/name, /number, /string].
Decl delivery_gap(Request, Round) bound [/name, /number].
Decl delivery_has_unsettled(Request, Round) bound [/name, /number].
Decl delivery_blocked(Request, Round) bound [/name, /number].
Decl delivery_resolved_some(Request, Round) bound [/name, /number].
Decl delivery_stalled(Request, Round) bound [/name, /number].
Decl delivery_pushback(Request, Round) bound [/name, /number].
Decl delivery_step_taken(Description) bound [/string].
Decl delivery_outcome(Request, Round, Outcome) bound [/name, /number, /name].

# A round exists once Jev's reading of it is asserted; the blocker reading is
# asserted for every round.
delivery_round(Request, Round) :-
    delivery_blocker_reading(Request, Round, _).

# --- Each fragment ---

delivery_asked(Request, Round, Fragment) :-
    delivery_reading(Request, Round, Fragment, Asked, _),
    config_param(/jev_no_percent, No),
    Asked > No.

delivery_fragment_delivered(Request, Round, Fragment) :-
    delivery_asked(Request, Round, Fragment),
    delivery_reading(Request, Round, Fragment, _, Delivered),
    config_param(/jev_yes_percent, Yes),
    Delivered >= Yes.

# A part the reports were cut to fit cannot be read as missing: the cut may
# hold it.
delivery_fragment_missing(Request, Round, Fragment) :-
    delivery_asked(Request, Round, Fragment),
    delivery_reading(Request, Round, Fragment, _, Delivered),
    config_param(/jev_no_percent, No),
    Delivered <= No,
    !delivery_result_cut(Request, Round).

delivery_fragment_unsettled(Request, Round, Fragment) :-
    delivery_asked(Request, Round, Fragment),
    !delivery_fragment_delivered(Request, Round, Fragment),
    !delivery_fragment_missing(Request, Round, Fragment).

# --- Each round ---

delivery_gap(Request, Round) :-
    delivery_fragment_missing(Request, Round, _).

delivery_has_unsettled(Request, Round) :-
    delivery_fragment_unsettled(Request, Round, _).

delivery_blocked(Request, Round) :-
    delivery_blocker_reading(Request, Round, Blocked),
    config_param(/jev_yes_percent, Yes),
    Blocked >= Yes.

# Progress: a part the previous round left missing is no longer missing.
delivery_resolved_some(Request, Round) :-
    delivery_round(Request, Round),
    Previous = fn:minus(Round, 1),
    delivery_fragment_missing(Request, Previous, Fragment),
    !delivery_fragment_missing(Request, Round, Fragment).

# A step that resolved none of what it was handed changes nothing by running
# again.
delivery_stalled(Request, Round) :-
    delivery_gap(Request, Round),
    Previous = fn:minus(Round, 1),
    delivery_gap(Request, Previous),
    !delivery_resolved_some(Request, Round).

delivery_pushback(Request, Round) :-
    delivery_gap(Request, Round),
    !delivery_blocked(Request, Round),
    !delivery_stalled(Request, Round).

# --- The step that hands the gap back ---

delivery_step_taken(Description) :-
    delivery_step_offer(_, _, _, Description, _),
    shard_result(_, _, _, Description, _).

has_pending_subtask(TaskID, Description, ShardType) :-
    delivery_step_offer(Request, Round, TaskID, Description, ShardType),
    delivery_active(Request),
    delivery_pushback(Request, Round),
    !delivery_step_taken(Description).

# --- What the round ended as, for the summary ---

delivery_outcome(Request, Round, /delivered) :-
    delivery_round(Request, Round),
    !delivery_gap(Request, Round),
    !delivery_has_unsettled(Request, Round).

delivery_outcome(Request, Round, /unsettled) :-
    delivery_has_unsettled(Request, Round),
    !delivery_gap(Request, Round).

delivery_outcome(Request, Round, /blocked) :-
    delivery_gap(Request, Round),
    delivery_blocked(Request, Round).

delivery_outcome(Request, Round, /no_progress) :-
    delivery_stalled(Request, Round),
    !delivery_blocked(Request, Round).

delivery_outcome(Request, Round, /pushback) :-
    delivery_pushback(Request, Round).
