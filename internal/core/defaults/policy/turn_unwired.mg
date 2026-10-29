# New production code no production path reaches (the /unwired round)
#
# A shard agent could finish a turn with a function nothing in production
# calls, and the turn still read verified: on 2026-10-28 a coder added
# RepairExistingPromptAtoms to internal/init/agents.go, ran it once from a
# temporary test, deleted the test and reported done. Nothing called it
# until the root session wired it into boot.
#
# The executor measures, for each function and method the turn added to a
# non-test Go file (session/unwired_round.go): its exposure (/api when it is
# exported from a package another module can import, else /module), whether
# the toolchain or a directive runs it (main, //go:linkname, //export,
# //go:wasmexport), whether its file is generated, whether the index could
# observe every place a use could sit, every use of a function outside its
# own declaration with the declaration that holds it (the structure index's
# Uses), and, for a method, whether its name occurs in production or test
# code outside a declaration (an interface it satisfies, a call through a
# value of an interface type). This file decides which of them no
# production path reaches, and whether the turn is pushed back.
#
# Reachability is a closure: a use from code the turn did not add reaches
# the function; a use from a function the turn added reaches it only when
# that function is reached itself, so a new helper that only a new, uncalled
# function calls is not wired. A use from a test reaches nothing.
#
# The model answers a push-back by wiring the function, deleting it, or
# saying why it is meant to have no caller here. Jev reads that reason
# (learnjudge.JudgeUnusedReason): does it name a caller that exists today
# outside the code the index reads, and does the user's request ask for the
# function without asking the program to call it? Either settles the
# function. A reason Jev could not read (no judge, a failed call) settles it
# too, as the stated reason, unjudged: a failed call is never a "no". The
# round stops when every function is wired or settled, or when an attempt
# writes nothing (turn_unwired_idle). What is still open withholds the
# verdict, so a turn with unwired code no longer reads verified.

Decl turn_new_func(Turn, Ref, Exposure) bound [/name, /string, /name].
Decl turn_new_func_entry(Turn, Ref) bound [/name, /string].
Decl turn_new_func_generated(Turn, Ref) bound [/name, /string].
Decl turn_new_func_unobserved(Turn, Ref) bound [/name, /string].
Decl turn_func_use(Turn, Ref, From, Side) bound [/name, /string, /string, /name].
Decl turn_method_mentioned(Turn, Ref, Side) bound [/name, /string, /name].
Decl turn_unused_reason_reading(Turn, Ref, Caller, TaskAsks) bound [/name, /string, /number, /number].
Decl turn_unused_reason_unread(Turn, Ref) bound [/name, /string].
Decl turn_unwired_idle(Turn) bound [/name].

Decl turn_new_func_ref(Turn, Ref) bound [/name, /string].
Decl turn_func_exempt(Turn, Ref) bound [/name, /string].
Decl turn_func_reached(Turn, Ref) bound [/name, /string].
Decl turn_func_unwired(Turn, Ref) bound [/name, /string].
Decl turn_unused_reason_accepted(Turn, Ref) bound [/name, /string].
Decl turn_func_settled(Turn, Ref) bound [/name, /string].
Decl turn_func_unwired_open(Turn, Ref) bound [/name, /string].
Decl turn_func_test_only(Turn, Ref) bound [/name, /string].
Decl turn_unwired_open(Turn) bound [/name].
Decl turn_unwired_pushback(Turn) bound [/name].

turn_new_func_ref(Turn, Ref) :- turn_new_func(Turn, Ref, _).

# Not charged: another module may call an exported function of an importable
# package; generated code answers to its generator; and an absence the index
# could not observe everywhere is not proven.
turn_func_exempt(Turn, Ref) :- turn_new_func(Turn, Ref, /api).
turn_func_exempt(Turn, Ref) :- turn_new_func_generated(Turn, Ref).
turn_func_exempt(Turn, Ref) :- turn_new_func_unobserved(Turn, Ref).

turn_func_reached(Turn, Ref) :- turn_new_func_entry(Turn, Ref).
turn_func_reached(Turn, Ref) :- turn_method_mentioned(Turn, Ref, /production).
turn_func_reached(Turn, Ref) :-
    turn_func_use(Turn, Ref, From, /production),
    !turn_new_func_ref(Turn, From).
turn_func_reached(Turn, Ref) :-
    turn_func_use(Turn, Ref, From, /production),
    turn_func_reached(Turn, From).
turn_func_reached(Turn, Ref) :-
    turn_func_use(Turn, Ref, From, /production),
    turn_func_exempt(Turn, From).
turn_func_reached(Turn, Ref) :-
    turn_func_use(Turn, Ref, From, /production),
    turn_func_settled(Turn, From).

turn_func_unwired(Turn, Ref) :-
    turn_new_func(Turn, Ref, _),
    !turn_func_reached(Turn, Ref),
    !turn_func_exempt(Turn, Ref).

turn_unused_reason_accepted(Turn, Ref) :-
    turn_unused_reason_reading(Turn, Ref, Caller, _),
    config_param(/jev_yes_percent, Yes),
    Caller >= Yes.
turn_unused_reason_accepted(Turn, Ref) :-
    turn_unused_reason_reading(Turn, Ref, _, Asks),
    config_param(/jev_yes_percent, Yes),
    Asks >= Yes.

turn_func_settled(Turn, Ref) :- turn_unused_reason_accepted(Turn, Ref).
turn_func_settled(Turn, Ref) :- turn_unused_reason_unread(Turn, Ref).

turn_func_unwired_open(Turn, Ref) :-
    turn_func_unwired(Turn, Ref),
    !turn_func_settled(Turn, Ref).

# What the push-back says about a function: only tests call it.
turn_func_test_only(Turn, Ref) :-
    turn_func_unwired_open(Turn, Ref),
    turn_func_use(Turn, Ref, _, /test).
turn_func_test_only(Turn, Ref) :-
    turn_func_unwired_open(Turn, Ref),
    turn_method_mentioned(Turn, Ref, /test).

turn_unwired_open(Turn) :- turn_func_unwired_open(Turn, _).

turn_unwired_pushback(Turn) :-
    turn_unwired_open(Turn),
    !turn_unwired_idle(Turn).

# The verdict (coder_safety.mg): unwired code is a red gate, and the reason
# the turn is not verified.
has_red_gate(Turn) :- turn_unwired_open(Turn).
turn_missing_evidence(Turn, /code_unwired) :- turn_unverified(Turn), turn_unwired_open(Turn).
