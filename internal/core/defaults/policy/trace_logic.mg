# Trace Logic
# Defines metadata for tracing and debugging tools.
# Extracted from internal/core/trace.go

Decl rule_metadata(Predicate, RuleName).

# IDB Rules Metadata
# Mapping Predicate -> Rule Name for trace visualization
rule_metadata("next_action", "strategy_selector").
rule_metadata("permitted", "permission_gate").
rule_metadata("block_commit", "commit_barrier").
rule_metadata("impacted", "transitive_impact").
rule_metadata("clarification_needed", "focus_threshold").
rule_metadata("unsafe_to_refactor", "refactoring_guard").
rule_metadata("test_state", "tdd_loop").
rule_metadata("context_atom", "spreading_activation").
rule_metadata("missing_hypothesis", "abductive_repair").
rule_metadata("delegate_task", "shard_delegation").
rule_metadata("activation", "activation_rules").
rule_metadata("derived_context", "context_inference").

# Rule Descriptions for Trace Visualization
Decl rule_description(Predicate, Text).
rule_description("next_action", "next_action(X) :- user_intent(_, Verb, _), action_for_verb(Verb, X).").
rule_description("impacted", "impacted(X) :- dependency_link(X, Y, _), modified(Y).").
rule_description("clarification_needed", "clarification_needed(Ref) :- focus_resolution(Ref, _, _, Score), Score < 85.").
rule_description("block_commit", "block_commit(Reason) :- diagnostic(/error, _, _, _, _).").
rule_description("permitted", "permitted(Action) :- safe_action(Action).").
rule_description("context_to_inject", "context_to_inject(Fact) :- activation(Fact, Score), Score > 30.").
rule_description("unsafe_to_refactor", "unsafe_to_refactor(Target) :- impacted(Dep), not test_coverage(Dep).").
rule_description("needs_research", "needs_research(Agent) :- shard_profile(Agent, _, Topics, _), not knowledge_ingested(Agent).").

# EDB Predicates Metadata (Base Facts)
Decl is_edb_predicate(Predicate).

is_edb_predicate("file_topology").
is_edb_predicate("file_content").
is_edb_predicate("symbol_graph").
is_edb_predicate("dependency_link").
is_edb_predicate("diagnostic").
is_edb_predicate("observation").
is_edb_predicate("user_intent").
is_edb_predicate("focus_resolution").
is_edb_predicate("preference").
is_edb_predicate("shard_profile").
is_edb_predicate("knowledge_atom").
is_edb_predicate("workspace_fact").
is_edb_predicate("current_time").
