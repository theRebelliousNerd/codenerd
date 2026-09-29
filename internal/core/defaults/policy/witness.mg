# Witness Obligations: Which Tests Execute Which Changed Elements
# Version: 1.0.0
# Philosophy: a turn that changed behaviour owes a test that executes the change
#
# Step 1 of the witness rules (K1). test_executes derives which tests execute
# which code elements; witness_owed and witness_candidate derive which changed
# elements of a writing turn are owed a witness, and which tests could serve.
#
# Step 6 (W6) arms the verdict: witness_met discharges an owed element when
# the turn's own coverage run executed it and the turn's /test gate passed,
# and turn_unwitnessed names what is still owed. The static test_executes
# edge stays a CANDIDATE only -- an import edge is not execution -- so
# witness_candidate still has no verdict consumer.
#
# test_executes has two producers by design: the static rule below, from the
# test_impact.mg chain, and later Go-asserted dynamic rows from coverage. Both
# spell the callee as the code_element ref the chain joins on
# (fn:<pkg>.<Name>; commit b80adca9).

Decl test_executes(TestRef, Ref) bound [/string, /string].
Decl witness_owed(Turn, Ref) bound [/name, /string].
Decl witness_candidate(Turn, Ref, TestRef) bound [/name, /string, /string].
Decl witness_executed(Turn, Ref) bound [/name, /string].
Decl witness_met(Turn, Ref) bound [/name, /string].
Decl turn_unwitnessed(Turn, Ref) bound [/name, /string].
Decl turn_has_unwitnessed(Turn) bound [/name].

# Static execution: a test executes what it statically depends on, directly or
# through the call/method/embed closure. Reuses test_depends_on_transitive/2
# (test_impact.mg) rather than recursing again.
test_executes(TestRef, Ref) :-
    test_depends_on_transitive(TestRef, Ref).

# A writing turn owes a witness for every element it changed or added.
# turn_changed_element is the executor's measurement at turn closure
# (session/turn_elements.go, Decl in coder_safety.mg); turn_wrote is the
# policy's "this turn made a claim about the workspace" (coder_safety.mg), so
# a changed element without a writing turn behind it owes nothing.
witness_owed(Turn, Ref) :-
    turn_changed_element(Turn, Ref),
    turn_wrote(Turn).

# The candidate witnesses for an owed element: the tests that execute it. An
# owed element no test executes derives no candidate. Candidates rank which
# tests to run; they never discharge the witness (X2 F2: the static edge
# overclaims execution).
witness_candidate(Turn, Ref, TestRef) :-
    witness_owed(Turn, Ref),
    test_executes(TestRef, Ref).

# Step 6: only execution evidence discharges the witness. witness_executed is
# the turn's own coverage run: the element's span holds a statement block the
# profile measured, and none of its blocks went unexecuted
# (session/turn_element_coverage.go, from the same `go test -coverprofile`
# run whose verdict is turn_gate(Turn, /test, _)).
witness_executed(Turn, Ref) :-
    turn_element_measured(Turn, Ref),
    !turn_element_uncovered(Turn, Ref).

# Met when the measured run passed: execution under a red gate proves the
# code ran, not that it works.
witness_met(Turn, Ref) :-
    witness_owed(Turn, Ref),
    witness_executed(Turn, Ref),
    turn_gate(Turn, /test, /passing),
    !turn_red_gate(Turn, /test).

# Owed but not met. coder_safety.mg withholds turn_verified while any holds
# and names /change_unwitnessed; the executor lists these refs to the model.
# Not while the turn's /test gate is red: the red gate already withholds the
# verdict (/tests_not_green), and naming every changed element as unexecuted
# on top of a failing run points the model away from the failure it must fix.
# A run that never happened (gate unmet) still owes every witness.
turn_unwitnessed(Turn, Ref) :-
    witness_owed(Turn, Ref),
    !witness_met(Turn, Ref),
    !turn_red_gate(Turn, /test).

turn_has_unwitnessed(Turn) :- turn_unwitnessed(Turn, _).
