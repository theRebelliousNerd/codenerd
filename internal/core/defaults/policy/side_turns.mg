# Side turns: a message typed while a turn runs work (chat side_turn.go). The
# running turn keeps its own per-turn facts, so a side turn is read into
# these predicates only, and the kernel decides what it is:
#   /stop      cancel the running turn (Jev: the user asks to stop);
#   /later     a request for work of its own, run as a full turn when the
#              running one ends (perception reads a mutation, or Jev says
#              the user asks for files to change);
#   /converse  a reply, nothing more.

Decl side_turn_intent(Category, Verb) bound [/name, /name].
Decl side_turn_judgment(Name, Percent) bound [/string, /number].
Decl side_turn_wants(Action) bound [/name].
Decl side_turn_action(Action) bound [/name].

side_turn_wants(/stop) :-
    side_turn_judgment("wants_to_stop", Percent),
    config_param(/jev_yes_percent, Yes),
    Percent >= Yes.

side_turn_wants(/later) :-
    side_turn_intent(/mutation, Verb).
side_turn_wants(/later) :-
    side_turn_judgment("changes_files", Percent),
    config_param(/jev_yes_percent, Yes),
    Percent >= Yes.

side_turn_action(/stop) :-
    side_turn_wants(/stop).
side_turn_action(/later) :-
    side_turn_wants(/later),
    !side_turn_wants(/stop).
side_turn_action(/converse) :-
    side_turn_intent(Category, Verb),
    !side_turn_wants(/stop),
    !side_turn_wants(/later).
