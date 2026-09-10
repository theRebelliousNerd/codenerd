# Per-task working context. Loaded beside the canonical context_compilation.mg
# in a private evaluation scope; observations can never authorize an action.
Decl working_observation(ID, Entity, Revision, Kind, Step) bound [/string, /string, /string, /string, /number].
Decl working_revision(Entity, Revision) bound [/string, /string].
Decl working_recent(ID) bound [/string].
Decl working_stale(ID) bound [/string].
Decl working_superseded(ID) bound [/string].
Decl working_selected(ID, Priority) bound [/string, /name].
Decl working_control(Cycle, FailedRounds) bound [/name, /number].
Decl working_stop(Reason) bound [/name].
Decl working_stopped() bound [].
Decl working_continue() descr [doc("Continuation requires no observed stall; it never means task completion.")].

working_stop(/repeated_cycle) :- working_control(/yes, _).
working_stop(/tool_failures) :- working_control(_, Failed), Failed >= 3.
# Negation needs a bound literal, and a wildcard in a negated atom does not
# exclude in this Mangle; project the stop reasons to arity 0 first.
working_stopped() :- working_stop(_).
working_continue() :- working_control(_, _), !working_stopped().

working_stale(ID) :-
    working_observation(ID, Entity, Old, _, _),
    working_revision(Entity, Current), Old != Current.

working_superseded(ID) :-
    working_observation(ID, Entity, Revision, Kind, Step),
    working_observation(_, Entity, Revision, Kind, Later), Step < Later.

working_selected(ID, Priority) :-
    working_observation(ID, Entity, _, _, _),
    should_include_context(Entity, Priority),
    !working_stale(ID), !working_superseded(ID).

working_selected(ID, /p100) :-
    working_observation(ID, _, _, _, _), working_recent(ID),
    !working_stale(ID), !working_superseded(ID).
