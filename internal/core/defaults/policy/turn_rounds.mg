# Post-edit rounds -- which a turn owes, and in what order (sweep finding F4)
#
# verifyCompletedToolTurn ran eleven rounds in a fixed Go order, and six of
# them decided for themselves whether they applied, from the written paths'
# extensions (touchedGoFiles): a second answer to turn_owes_gate, and not the
# same one -- a write the policy classes as owing /build, but with no recorded
# path, never built. The executor now asks turn_next_round, runs the round it
# names, records turn_round_ran, and asks again until none derives. A round
# the policy owes and the executor has no driver for is an error, so a new
# round is a rule here and a driver there, not a pipeline edit.
#
# turn_write_tools is the executor's count of this turn's successful write
# tools, asserted before the first question: it makes turn_wrote hold during
# the turn, which turn_evidence -- asserted at the closure -- could not.

Decl round_order(Round, Order) bound [/name, /number].
Decl turn_round_owed(Turn, Round) bound [/name, /name].
Decl turn_round_ran(Turn, Round) bound [/name, /name].
Decl turn_round_pending(Turn, Round, Order) bound [/name, /name, /number].
Decl turn_next_round_order(Turn, Order) bound [/name, /number].
Decl turn_next_round(Turn, Round) bound [/name, /name].
Decl turn_write_tools(Turn, Count) bound [/name, /number].

# The order carries what the rounds depend on: the tests after the build
# (compiler errors are a better repair signal than test output wrapped
# around them); the critic once the change compiles and passes, and before
# the rounds that answer for its edits; coverage before pinning, which counts
# the tests coverage asks for, and before vet, which vets them; removed tests
# after the test gate; the test run last, because a write after it would
# reset the run it asks for.
round_order(/build, 1).
round_order(/test, 2).
round_order(/critic, 3).
round_order(/coverage, 4).
round_order(/pinned, 5).
round_order(/vet, 6).
round_order(/removed_tests, 7).
round_order(/test_run, 8).

turn_round_owed(Turn, /build) :- turn_owes_gate(Turn, /build).
turn_round_owed(Turn, /test) :- turn_owes_gate(Turn, /test).
turn_round_owed(Turn, /critic) :- turn_write_class(Turn, /go).
# Non-Go source is reviewed too, grounded by its language server's
# diagnostics (session/lsp_diagnostics.go). Until 2026-09-25 only a Go write
# owed the critic, so a Python or TypeScript turn was never reviewed. A write
# the critic cannot read (a YAML file, say) costs nothing: it offers no file
# and the round ends without a call.
turn_round_owed(Turn, /critic) :- turn_write_class(Turn, /other).
turn_round_owed(Turn, /coverage) :- turn_owes_gate(Turn, /test).
turn_round_owed(Turn, /pinned) :- turn_owes_gate(Turn, /pinned).
turn_round_owed(Turn, /vet) :- turn_write_class(Turn, /go).
turn_round_owed(Turn, /removed_tests) :- turn_wrote(Turn).
turn_round_owed(Turn, /test_run) :- turn_owes_gate(Turn, /test_run).

turn_round_pending(Turn, Round, Order) :-
    turn_round_owed(Turn, Round),
    round_order(Round, Order),
    !turn_round_ran(Turn, Round).

turn_next_round_order(Turn, Min) :-
    turn_round_pending(Turn, Round, Order)
    |> do fn:group_by(Turn), let Min = fn:min(Order).

turn_next_round(Turn, Round) :-
    turn_next_round_order(Turn, Order),
    turn_round_pending(Turn, Round, Order).

turn_wrote(Turn) :- turn_write_tools(Turn, Count), Count > 0.
