# Intent System - Inference Rules

pattern_verb_pair(Pattern, Verb1, Verb2) :-
    multistep_verb_pair(Pattern, Verb1, Verb2).

pattern_relation(Pattern, Relation) :-
    multistep_pattern(Pattern, _, Relation, _).
