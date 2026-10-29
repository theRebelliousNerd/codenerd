# Autopoiesis Integration for System Shards
# Extracted from system.mg

# Decl imports
# Moved to schemas_shards.mg
# Decl unhandled_case_count(ShardName, Count).
# Decl system_shard(ShardName, Type).
# Decl unhandled_case_count_computed(ShardName, Count).
# Decl propose_new_rule(ShardName).
# Decl rule_needs_approval(RuleID).
# Decl proposed_rule(RuleID, ShardName, MangleCode, Confidence).
# Decl auto_apply_rule(RuleID).
# Decl rule_applied(RuleID).
# Decl applied_rule(RuleID, Timestamp).
# Decl learning_signal(SignalType, RuleID).
# Decl rule_outcome(RuleID, Outcome, Details).

# Unhandled case tracking (for rule learning)
unhandled_case_count(ShardName, Count) :-
    system_shard(ShardName, _),
    unhandled_case_count_computed(ShardName, Count).

# Trigger LLM for rule proposal when threshold reached
propose_new_rule(ShardName) :-
    unhandled_case_count(ShardName, Count),
    Count >= 3.

# Proposed rule needs human approval if low confidence (confidence on 0-100 scale)
rule_needs_approval(RuleID) :-
    proposed_rule(RuleID, _, _, Confidence),
    Confidence < 80.

# Auto-apply rule if high confidence (confidence on 0-100 scale)
auto_apply_rule(RuleID) :-
    proposed_rule(RuleID, _, _, Confidence),
    Confidence >= 80,
    !rule_applied(RuleID).

# Helper for safe negation
rule_applied(RuleID) :-
    applied_rule(RuleID, _).

# Learn from successful rule applications
learning_signal(/rule_success, RuleID) :-
    applied_rule(RuleID, Timestamp),
    rule_outcome(RuleID, /success, _).
