# Inference Logic for Intent Refinement (Simplified)
# This module takes raw intent candidates (from regex/LLM) and refines them
# using contextual logic and safety constraints.

Decl candidate_intent(Verb, RawScore).
Decl context_token(Token).

# Decl system_state(Key, Value).

# Declare Boost and Penalty predicates
Decl boost(Verb, Amount).
Decl penalty(Verb, Amount).

# Output: Refined Score
Decl refined_score(Verb, Score).

# Base score from candidate
refined_score(Verb, Score) :-
    candidate_intent(Verb, Score).

# -----------------------------------------------------------------------------
# CONTEXTUAL BOOSTING
# -----------------------------------------------------------------------------

# Security Boost: If "security" or "vuln" appears, boost /security
boost(Verb, 0.3) :-
    candidate_intent(Verb, _),
    Verb = /security,
    context_token("security").

boost(Verb, 0.3) :-
    candidate_intent(Verb, _),
    Verb = /security,
    context_token("vulnerability").

# Testing Boost: If "coverage" appears, prefer /test over /review
boost(Verb, 0.2) :-
    candidate_intent(Verb, _),
    Verb = /test,
    context_token("coverage").

# Debugging Boost: If "error" or "panic" appears, prefer /debug over /fix
# fixing is the goal, but debugging is the immediate action.
boost(Verb, 0.15) :-
    candidate_intent(Verb, _),
    Verb = /debug,
    context_token("panic").

boost(Verb, 0.15) :-
    candidate_intent(Verb, _),
    Verb = /debug,
    context_token("stacktrace").

# -----------------------------------------------------------------------------
# SAFETY CONSTRAINTS (Penalties)
# -----------------------------------------------------------------------------

# Safety: Don't /delete if we are in a "learning" mode or context implies "safe"
penalty(Verb, 0.5) :-
    candidate_intent(Verb, _),
    Verb = /delete,
    context_token("safe").

# Ambiguity: If "fix" and "test" both appear, "fix" usually dominates,
# but if "verify" is present, "test" should win.
boost(Verb, 0.2) :-
    candidate_intent(Verb, _),
    Verb = /test,
    context_token("verify").

# -----------------------------------------------------------------------------
# FINAL SCORE CALCULATION (Relational Max, No Pipes)
# -----------------------------------------------------------------------------

# Generate potential scores by applying single boosts.
Decl potential_score(Verb, Score).

# 1. Base Score is a potential score
potential_score(Verb, Score) :- candidate_intent(Verb, Score).

# 2. Boosted Scores (Apply Boost)
# S = Base + Amount
potential_score(Verb, S) :-
    candidate_intent(Verb, Base),
    boost(Verb, Amount),
    S = fn:plus(Base, Amount).

# 3. Penalized Scores (Apply Penalty)
# S = Base - Amount (using minus 0, Amount)
potential_score(Verb, S) :-
    candidate_intent(Verb, Base),
    penalty(Verb, Amount),
    Neg = fn:minus(0, Amount),
    S = fn:plus(Base, Neg).

# 4. Relational Max Logic
# Find scores that are NOT max
Decl has_greater_score(Score).
has_greater_score(S) :-
    potential_score(_, S),
    potential_score(_, Other),
    Other > S.

# Define max score as one that has no greater score
Decl best_score(MaxScore).
best_score(S) :-
    potential_score(_, S),
    !has_greater_score(S).

# Select verb matching the max score
Decl selected_verb(Verb).
selected_verb(Verb) :-
    potential_score(Verb, S),
    best_score(Max),
    S = Max.