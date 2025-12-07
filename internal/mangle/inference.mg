# Inference Logic for Intent Refinement
# This module takes raw intent candidates (from regex/LLM) and refines them
# using contextual logic and safety constraints.

Decl candidate_intent(Verb.Type<n>, RawScore.Type<float>).
Decl context_token(Token.Type<string>).
Decl system_state(Key.Type<string>, Value.Type<string>).

# Output: Refined Score
Decl refined_score(Verb.Type<n>, Score.Type<float>).

# Base score from candidate
refined_score(Verb, Score) :-
    candidate_intent(Verb, Score).

# -----------------------------------------------------------------------------
# CONTEXTUAL BOOSTING
# -----------------------------------------------------------------------------

# Security Boost: If "security" or "vuln" appears, boost /security
# Even if regex matched /review, context implies /security is better.
boost(Verb, 0.3) :-
    candidate_intent(Verb, _),
    Verb == /security,
    context_token("security").

boost(Verb, 0.3) :-
    candidate_intent(Verb, _),
    Verb == /security,
    context_token("vulnerability").

# Testing Boost: If "coverage" appears, prefer /test over /review
boost(Verb, 0.2) :-
    candidate_intent(Verb, _),
    Verb == /test,
    context_token("coverage").

# Debugging Boost: If "error" or "panic" appears, prefer /debug over /fix
# fixing is the goal, but debugging is the immediate action.
boost(Verb, 0.15) :-
    candidate_intent(Verb, _),
    Verb == /debug,
    context_token("panic").

boost(Verb, 0.15) :-
    candidate_intent(Verb, _),
    Verb == /debug,
    context_token("stacktrace").

# -----------------------------------------------------------------------------
# SAFETY CONSTRAINTS (Penalties)
# -----------------------------------------------------------------------------

# Safety: Don't /delete if we are in a "learning" mode or context implies "safe"
penalty(Verb, 0.5) :-
    candidate_intent(Verb, _),
    Verb == /delete,
    context_token("safe").

# Ambiguity: If "fix" and "test" both appear, "fix" usually dominates,
# but if "verify" is present, "test" should win.
boost(Verb, 0.2) :-
    candidate_intent(Verb, _),
    Verb == /test,
    context_token("verify").

# -----------------------------------------------------------------------------
# FINAL AGGREGATION
# -----------------------------------------------------------------------------

Decl final_adjustment(Verb.Type<n>, Delta.Type<float>).

final_adjustment(Verb, D) :-
    boost(Verb, Amount) |>
    do fn:group_by(Verb),
    let D = fn:Sum(Amount).

final_adjustment(Verb, D) :-
    penalty(Verb, Amount) |>
    do fn:group_by(Verb),
    let P = fn:Sum(Amount) |>
    let D = fn:negate(P). # Negative delta

# Calculate Final Score
Decl final_intent_score(Verb.Type<n>, Score.Type<float>).

final_intent_score(Verb, Final) :-
    candidate_intent(Verb, Base),
    !final_adjustment(Verb, _) |> # No adjustments
    let Final = Base.

final_intent_score(Verb, Final) :-
    candidate_intent(Verb, Base),
    final_adjustment(Verb, Delta) |>
    let Final = fn:plus(Base, Delta).

# Select Best (Max)
Decl best_intent_score(MaxScore.Type<float>).
best_intent_score(M) :-
    final_intent_score(_, S) |>
    do fn:group_by(),
    let M = fn:Max(S).

Decl selected_verb(Verb.Type<n>).
selected_verb(Verb) :-
    final_intent_score(Verb, Score),
    best_intent_score(Max),
    Score == Max.
