# System Shard Coordination - Autopoiesis Logic
# Domain: System Self-Correction and Rule Learning

Decl unhandled_case_count(ShardName.Type<string>, Count.Type<int64>).
Decl unhandled_case_count_computed(ShardName.Type<string>, Count.Type<int64>).
Decl propose_new_rule(ShardName.Type<string>).
Decl rule_needs_approval(RuleID.Type<string>).
Decl proposed_rule(RuleID.Type<string>, RuleText.Type<string>, Source.Type<string>, Confidence.Type<int64>).
Decl auto_apply_rule(RuleID.Type<string>).
Decl rule_applied(RuleID.Type<string>).
Decl applied_rule(RuleID.Type<string>, Timestamp.Type<int64>).
Decl learning_signal(SignalType.Type<atom>, RuleID.Type<string>).
Decl rule_outcome(RuleID.Type<string>, Outcome.Type<atom>, Info.Type<string>).
Decl system_shard(ShardName.Type<atom>, Status.Type<atom>).

# Autopoiesis Integration for System Shards

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
