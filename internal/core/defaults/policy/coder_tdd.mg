# Coder Policy - Test-Driven Development
# What the test state tells the coder. tdd_state is derived from the workspace's
# own test results (intent_routing_rules.mg): /red while a test fails, /green
# once the failures are fixed and code changed. These rows say what each state
# asks of the coder and are told to it through injectable_context.
#
# Intent conflict, dropped: the original rules also marked an implementation
# edit made in the red phase, and a test edit made in the green phase, as TDD
# violations. tdd_state is derived from "some test fails", not from a declared
# red-then-green workflow, so a bug fix (a failing test and the production code
# that must change) would have been a violation by construction. The edit
# classification those rules needed went with them.

tdd_active() :-
    tdd_state(_).

tdd_red_phase() :-
    tdd_state(/red).

tdd_green_phase() :-
    tdd_state(/green).

tdd_refactor_phase() :-
    tdd_state(/refactor).

injectable_context("coder", "Tests are failing: the failures define the work. Make them pass without weakening or deleting a test.") :-
    tdd_red_phase().

injectable_context("coder", "Tests pass: keep the implementation minimal and do not change behaviour a test relies on.") :-
    tdd_green_phase().

injectable_context("coder", "Tests pass and the code can be improved: refactor in small steps and run the tests after each.") :-
    tdd_refactor_phase().
