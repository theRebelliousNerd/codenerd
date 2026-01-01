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

# =============================================================================
# INTENT UNKNOWN / UNMAPPED CLARIFICATION
# =============================================================================

next_action(/interrogative_mode) :-
    intent_unknown(_, _),
    !any_awaiting_clarification(/yes).

next_action(/interrogative_mode) :-
    intent_unmapped(_, _),
    !any_awaiting_clarification(/yes).

clarification_question(/current_intent, Question) :-
    intent_unmapped(Verb, /unknown_verb),
    Question = fn:string_concat("I don't recognize the action '", Verb, "'. What would you like me to do?").

clarification_question(/current_intent, "Could you rephrase what you'd like me to do?") :-
    intent_unknown(_, /llm_failed).

clarification_question(/current_intent, "I'm not confident I understood correctly. Could you clarify?") :-
    intent_unknown(_, /heuristic_low).

clarification_question(/current_intent, "I couldn't identify the action you want. What would you like me to do?") :-
    intent_unknown(_, /no_verb_match).

clarification_question(/current_intent, "I recognize the action but don't have a mapping for it. Which action should I take instead?") :-
    intent_unmapped(_, /no_action_mapping).

clarification_option(/current_intent, /explain, "Explain or describe something") :-
    intent_unmapped(_, _).
clarification_option(/current_intent, /fix, "Fix a bug or issue") :-
    intent_unmapped(_, _).
clarification_option(/current_intent, /review, "Review code for issues") :-
    intent_unmapped(_, _).
clarification_option(/current_intent, /search, "Search the codebase") :-
    intent_unmapped(_, _).
clarification_option(/current_intent, /test, "Run or generate tests") :-
    intent_unmapped(_, _).
clarification_option(/current_intent, /create, "Create new code or files") :-
    intent_unmapped(_, _).

# =============================================================================
# NO ACTION REASON HANDLING
# =============================================================================

next_action(/interrogative_mode) :-
    no_action_reason(_, /no_route),
    !any_awaiting_clarification(/yes).

clarification_question(IntentID, "I don't have a tool to handle this action. Would you like me to try a different approach?") :-
    no_action_reason(IntentID, /no_route).

next_action(/interrogative_mode) :-
    no_action_reason(_, /no_action_derived),
    !any_awaiting_clarification(/yes).

clarification_question(IntentID, "I'm not sure which action to take for this request. Could you clarify?") :-
    no_action_reason(IntentID, /no_action_derived),
    !learning_confirmation_active(/yes).

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
