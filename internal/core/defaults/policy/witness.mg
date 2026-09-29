# Witness Obligations: Which Tests Execute Which Changed Elements
# Version: 1.0.0
# Philosophy: a turn that changed behaviour owes a test that executes the change
#
# Step 1 of the witness rules (K1). test_executes derives which tests execute
# which code elements; witness_owed and witness_candidate derive which changed
# elements of a writing turn are owed a witness, and which tests could serve.
#
# Nothing reads these conclusions yet -- results, witness_met and the verdict
# arm land in a later lane, after the gates assert per-test results -- so this
# file cannot change any verdict today. It is derivation waiting for consumers.
#
# test_executes has two producers by design: the static rule below, from the
# test_impact.mg chain, and later Go-asserted dynamic rows from coverage. Both
# spell the callee as the code_element ref the chain joins on
# (fn:<pkg>.<Name>; commit b80adca9).

Decl test_executes(TestRef, Ref) bound [/string, /string].
Decl witness_owed(Turn, Ref) bound [/name, /string].
Decl witness_candidate(Turn, Ref, TestRef) bound [/name, /string, /string].

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
# owed element no test executes derives no candidate; that silence is the gap
# the later verdict lane reads.
witness_candidate(Turn, Ref, TestRef) :-
    witness_owed(Turn, Ref),
    test_executes(TestRef, Ref).
