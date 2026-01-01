# Learning Candidate Policy
# Staged learning candidates are surfaced for confirmation but never auto-applied.
learning_candidate_ready(Phrase, Verb) :-
    learning_candidate(Phrase, Verb, _, _).

# Surface confirmation for explicit learning candidates.
learning_confirmation_needed(Phrase, Verb, Target, /no_action_derived) :-
    learning_candidate(Phrase, Verb, Target, /no_action_derived).
learning_confirmation_needed(Phrase, Verb, Target, /critic_autolearn) :-
    learning_candidate(Phrase, Verb, Target, /critic_autolearn).
learning_confirmation_needed(Phrase, Verb, Target, /critic_manual) :-
    learning_candidate(Phrase, Verb, Target, /critic_manual).

learning_confirmation_active(/yes) :-
    learning_confirmation_needed(_, _, _, _).

next_action(/interrogative_mode) :-
    learning_confirmation_needed(_, _, _, _),
    !any_awaiting_clarification(/yes).

clarification_question(/current_intent, Question) :-
    learning_confirmation_needed(_, _, _, _),
    Question = "I can learn this mapping. Should I learn it?".

clarification_option(/current_intent, /learn_yes, "Yes, learn this mapping") :-
    learning_confirmation_needed(_, _, _, _).

clarification_option(/current_intent, /learn_no, "No, do not learn this") :-
    learning_confirmation_needed(_, _, _, _).
