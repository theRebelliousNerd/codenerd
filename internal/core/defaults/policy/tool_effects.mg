# Tool Effect Receipts: What a Call Changed, and What Its Validation Established
# Version: 1.0.0
# Philosophy: admitted is not executed; executed is not changed; changed is not correct
#
# A validator can reject a result after its bytes have landed. The session
# executor (internal/session/tool_effects.go) observes every write call's
# targets before and after the call and asserts, per turn:
#
#   turn_tool_effect(Turn, CallID, Path, Seq, State)  - the call left Path
#       /changed, or its state could not be read back (/unknown), at write
#       clock Seq (the same clock as turn_write_seq). Asserted whether the
#       handler or its validator then failed: a failed postcondition is not
#       proof of no mutation.
#   turn_tool_validation(Turn, CallID, Round, State)  - what validation round
#       Round established about the call's result: /verified, /rejected,
#       /incomplete, /unvalidated or /not_run. Append-only: a revalidation is
#       a later round, never an edit of the earlier one.
#   turn_tool_effect_superseded(Turn, CallID, Path)   - at the closure, Path no
#       longer holds what the call left: a later writer the receipts did not
#       observe replaced it, so the call's validation describes a revision
#       that is gone.
#
# Go observes; these rules decide which effect is current, whether the turn
# may claim its changes were measured, and what recovery the call owes.

Decl turn_tool_effect(Turn, CallID, Path, Seq, State) bound [/name, /string, /string, /number, /name].
Decl turn_tool_validation(Turn, CallID, Round, State) bound [/name, /string, /number, /name].
Decl turn_tool_effect_superseded(Turn, CallID, Path) bound [/name, /string, /string].
Decl turn_effect_last_seq(Turn, Path, Seq) bound [/name, /string, /number].
Decl turn_effect_current(Turn, Path, CallID) bound [/name, /string, /string].
Decl turn_tool_validated(Turn, CallID) bound [/name, /string].
Decl turn_tool_validation_round(Turn, CallID, Round) bound [/name, /string, /number].
Decl turn_tool_validation_state(Turn, CallID, State) bound [/name, /string, /name].
Decl turn_effect_unsettled(Turn, Path, Why) bound [/name, /string, /name].
Decl turn_has_rejected_effect(Turn) bound [/name].
Decl turn_has_unmeasured_effect(Turn) bound [/name].
Decl tool_effect_recovery(Turn, CallID, Move) bound [/name, /string, /name].

# The current effect on a path is the last call that changed it. An earlier
# call's validation says nothing about bytes a later call replaced.
turn_effect_last_seq(Turn, Path, Last) :-
    turn_tool_effect(Turn, CallID, Path, Seq, State)
    |> do fn:group_by(Turn, Path), let Last = fn:max(Seq).

turn_effect_current(Turn, Path, CallID) :-
    turn_effect_last_seq(Turn, Path, Last),
    turn_tool_effect(Turn, CallID, Path, Last, State),
    !turn_tool_effect_superseded(Turn, CallID, Path).

# The latest validation round decides; the earlier rounds stay as evidence.
turn_tool_validated(Turn, CallID) :-
    turn_tool_validation(Turn, CallID, Round, State).

turn_tool_validation_round(Turn, CallID, Last) :-
    turn_tool_validation(Turn, CallID, Round, State)
    |> do fn:group_by(Turn, CallID), let Last = fn:max(Round).

turn_tool_validation_state(Turn, CallID, State) :-
    turn_tool_validation_round(Turn, CallID, Last),
    turn_tool_validation(Turn, CallID, Last, State).

# A current change the turn cannot claim was measured. /unvalidated (no
# validator holds an opinion about the tool) is not here: it is visible on the
# receipt and the closing gates still measure the tree, but an absent optional
# validator is not a defect. Unknown never becomes passed.
turn_effect_unsettled(Turn, Path, /rejected) :-
    turn_effect_current(Turn, Path, CallID),
    turn_tool_validation_state(Turn, CallID, /rejected).
turn_effect_unsettled(Turn, Path, /incomplete) :-
    turn_effect_current(Turn, Path, CallID),
    turn_tool_validation_state(Turn, CallID, /incomplete).
turn_effect_unsettled(Turn, Path, /incomplete) :-
    turn_effect_current(Turn, Path, CallID),
    turn_tool_validation_state(Turn, CallID, /not_run).
turn_effect_unsettled(Turn, Path, /incomplete) :-
    turn_effect_current(Turn, Path, CallID),
    !turn_tool_validated(Turn, CallID).
turn_effect_unsettled(Turn, Path, /unknown) :-
    turn_effect_current(Turn, Path, CallID),
    turn_tool_effect(Turn, CallID, Path, Seq, /unknown).

turn_has_rejected_effect(Turn) :- turn_effect_unsettled(Turn, Path, /rejected).
turn_has_unmeasured_effect(Turn) :- turn_effect_unsettled(Turn, Path, /incomplete).
turn_has_unmeasured_effect(Turn) :- turn_effect_unsettled(Turn, Path, /unknown).

# What a call whose change is current owes. Never "repeat the call": the
# change is on disk, and a blind retry stacks a second effect on the first.
#   /revalidate - the handler failed after changing the file, so no validator
#                 measured the change; the host measures it once (round 0 only).
#   /repair     - a validator rejected the bytes the call left: inspect and
#                 repair the changed artifact.
#   /inspect    - validation could not complete: read the file before relying
#                 on it.
#   /reconcile  - the call's effect could not be read back: establish the
#                 file's state first.
tool_effect_recovery(Turn, CallID, /revalidate) :-
    turn_effect_current(Turn, Path, CallID),
    turn_tool_validation_state(Turn, CallID, /not_run),
    turn_tool_validation_round(Turn, CallID, 0).
tool_effect_recovery(Turn, CallID, /repair) :-
    turn_effect_current(Turn, Path, CallID),
    turn_tool_validation_state(Turn, CallID, /rejected).
tool_effect_recovery(Turn, CallID, /inspect) :-
    turn_effect_current(Turn, Path, CallID),
    turn_tool_validation_state(Turn, CallID, /incomplete).
tool_effect_recovery(Turn, CallID, /reconcile) :-
    turn_effect_current(Turn, Path, CallID),
    turn_tool_effect(Turn, CallID, Path, Seq, /unknown).
