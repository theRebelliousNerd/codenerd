# Focus Resolution & Clarification
# Section 4 of Cortex Executive Policy


# Clarification threshold - block execution if confidence < 85 (on 0-100 scale)
clarification_needed(Ref) :-
    focus_resolution(Ref, _, _, Score),
    Score < 85.

# Helpers for safe negation
any_awaiting_clarification(/yes) :- awaiting_clarification(_).

# Block action derivation when clarification is needed
next_action(/interrogative_mode) :-
    clarification_needed(_),
    !any_awaiting_clarification(/yes).

# Ambiguity detection
ambiguity_detected(Param) :-
    ambiguity_flag(Param, _, _).

next_action(/interrogative_mode) :-
    ambiguity_detected(_),
    !any_awaiting_clarification(/yes).

# Section 11: Abductive Reasoning

# Abductive reasoning: missing hypotheses are symptoms without known causes
# This rule requires all variables to be bound in the negated atom
# Implementation: We use a helper predicate has_known_cause to track which symptoms have causes
# Then negate against that helper

# Mark symptoms that have known causes
has_known_cause(Symptom) :-
    known_cause(Symptom, _).

# Symptoms without causes need investigation
# Note: Using has_known_cause helper to ensure safe negation
missing_hypothesis(Symptom) :-
    symptom(_, Symptom),
    !has_known_cause(Symptom).

# Trigger clarification for missing hypotheses
next_action(/interrogative_mode) :-
    missing_hypothesis(_).

# Section 16: Session State

# Resume from clarification
next_action(/resume_task) :-
    session_state(_, /suspended, _),
    focus_clarification(_).
