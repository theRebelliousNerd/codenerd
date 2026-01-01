# Learning Candidate Policy
# Staged learning candidates are surfaced for confirmation but never auto-applied.

learning_candidate_ready(Phrase, Verb) :-
    learning_candidate(Phrase, Verb, _, _).
