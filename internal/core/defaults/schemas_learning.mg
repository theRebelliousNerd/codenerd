# Schema for Self-Learned Patterns
# Pattern: The user input string to match
# Verb: The mapped action (e.g. /delete)
# Target: The target of the action (e.g. "database")
# Constraint: Additional instructions (e.g. "ensure: dry run")
# Confidence: 0-100 (integer scale, NOT 0.0-1.0 float)
# Note: Go code converts float confidence (0.0-1.0) to int (0-100) before storing
Decl learned_exemplar(Pattern, Verb, Target, Constraint, Confidence) bound [/string, /name, /string, /string, /number].

# Rule: Learned patterns override standard inference
# If we match a learned exemplar with high confidence, we skip the standard classifier
# Note: In the Go code, we will likely query this separately or integrate it.
# For now, we define the valid_intent override.

Decl valid_intent(Verb, Target) bound [/name, /string].

# learned_pattern(Category, Pattern) - Patterns learned from repeated outcomes
# Stored by autopoiesis feedback loop; queried by /systems status display
# Category: pattern category (e.g., "bug_fix", "common_error")
# Pattern: the learned pattern description
Decl learned_pattern(Category, Pattern) bound [/string, /string].

# Note: Conf is integer 0-100, not float 0.0-1.0
valid_intent(Verb, Target) :-
    learned_exemplar(Pattern, Verb, Target, _, Conf),
    Conf > 80.
