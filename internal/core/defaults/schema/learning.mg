# Schema for Self-Learned Patterns
# Pattern: The user input string to match
# Verb: The mapped action (e.g. /delete)
# Target: The target of the action (e.g. "database")
# Constraint: Additional instructions (e.g. "ensure: dry run")
# Confidence: 0.0-1.0
Decl learned_exemplar(Pattern, Verb, Target, Constraint, Confidence).

# Rule: Learned patterns override standard inference
# If we match a learned exemplar with high confidence, we skip the standard classifier
# Note: In the Go code, we will likely query this separately or integrate it.
# For now, we define the valid_intent override.

Decl valid_intent(Verb, Target).

# Note: Conf is integer 0-100, not float 0.0-1.0
valid_intent(Verb, Target) :-
    learned_exemplar(Pattern, Verb, Target, _, Conf),
    Conf > 80.
