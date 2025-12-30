# Intelligent Tool Routing
# Section 40 of Cortex Executive Policy

# Base Shard-Capability Affinities (EDB)
# Score 0-100 (integer scale) indicating how relevant a capability is to each shard type

# CoderShard affinities
shard_capability_affinity(/coder, /generation, 100).
shard_capability_affinity(/coder, /debugging, 90).
shard_capability_affinity(/coder, /transformation, 80).
shard_capability_affinity(/coder, /inspection, 50).
shard_capability_affinity(/coder, /validation, 40).
shard_capability_affinity(/coder, /execution, 60).

# TesterShard affinities
shard_capability_affinity(/tester, /validation, 100).
shard_capability_affinity(/tester, /execution, 90).
shard_capability_affinity(/tester, /inspection, 70).
shard_capability_affinity(/tester, /debugging, 60).
shard_capability_affinity(/tester, /analysis, 50).

# ReviewerShard affinities
shard_capability_affinity(/reviewer, /inspection, 100).
shard_capability_affinity(/reviewer, /analysis, 90).
shard_capability_affinity(/reviewer, /validation, 60).
shard_capability_affinity(/reviewer, /debugging, 40).

# ResearcherShard affinities
shard_capability_affinity(/researcher, /knowledge, 100).
shard_capability_affinity(/researcher, /analysis, 80).
shard_capability_affinity(/researcher, /inspection, 60).

# Generalist affinities (moderate across all)
shard_capability_affinity(/generalist, /generation, 50).
shard_capability_affinity(/generalist, /validation, 50).
shard_capability_affinity(/generalist, /inspection, 50).
shard_capability_affinity(/generalist, /analysis, 50).
shard_capability_affinity(/generalist, /execution, 50).
shard_capability_affinity(/generalist, /knowledge, 50).
shard_capability_affinity(/generalist, /debugging, 50).
shard_capability_affinity(/generalist, /transformation, 50).

# Intent-Capability Mappings (EDB)

# Mutation intents
intent_requires_capability(/implement, /generation, 100).
intent_requires_capability(/implement, /validation, 50).
intent_requires_capability(/refactor, /transformation, 100).
intent_requires_capability(/refactor, /analysis, 70).
intent_requires_capability(/fix, /debugging, 100).
intent_requires_capability(/fix, /validation, 80).
intent_requires_capability(/generate, /generation, 100).
intent_requires_capability(/scaffold, /generation, 90).
intent_requires_capability(/init, /generation, 80).

# Query intents
intent_requires_capability(/test, /validation, 100).
intent_requires_capability(/test, /execution, 90).
intent_requires_capability(/review, /inspection, 100).
intent_requires_capability(/review, /analysis, 80).
intent_requires_capability(/explain, /analysis, 100).
intent_requires_capability(/explain, /knowledge, 70).
intent_requires_capability(/debug, /debugging, 100).
intent_requires_capability(/debug, /inspection, 80).

# Research intents
intent_requires_capability(/research, /knowledge, 100).
intent_requires_capability(/research, /analysis, 60).
intent_requires_capability(/explore, /inspection, 90).
intent_requires_capability(/explore, /analysis, 80).
intent_requires_capability(/explore, /knowledge, 50).

# Run intents
intent_requires_capability(/run, /execution, 100).
intent_requires_capability(/run, /validation, 40).

# Tool Relevance Derivation Rules (IDB)

# 40.3.1 Base Relevance: Tool matches shard's capability affinity
tool_base_relevance(ShardType, ToolName, AffinityScore) :-
    tool_capability(ToolName, Cap),
    shard_capability_affinity(ShardType, Cap, AffinityScore),
    tool_registered(ToolName, _).

# 40.3.2 Intent Boost: Tool matches current intent's required capabilities
tool_intent_relevance(ToolName, Weight) :-
    current_intent(IntentID),
    user_intent(IntentID, _, Verb, _, _),
    intent_requires_capability(Verb, Cap, Weight),
    tool_capability(ToolName, Cap).

# No current intent = no boost (fallback rule)
# Uses helper predicate for safe negation
tool_intent_relevance(ToolName, 0) :-
    tool_registered(ToolName, _),
    !has_current_intent().

# 40.3.3 Domain Boost: Tool matches target file's language/domain
# Score: 30 (out of 100)
tool_domain_relevance(ToolName, 30) :-
    current_intent(IntentID),
    user_intent(IntentID, _, _, Target, _),
    file_topology(Target, _, Lang, _, _),
    tool_domain(ToolName, Lang).

tool_domain_relevance(ToolName, 0) :-
    tool_registered(ToolName, _),
    !has_tool_domain(ToolName).

# 40.3.4 Success History Boost: Tool succeeded in similar contexts
# Note: Uses simplified scoring - full implementation would compute rate
# Score: 20 (out of 100)
tool_success_relevance(ToolName, 20) :-
    tool_usage_stats(ToolName, ExecCount, SuccessCount, _),
    ExecCount > 0,
    SuccessCount > 0.

tool_success_relevance(ToolName, 0) :-
    tool_registered(ToolName, _),
    !has_tool_usage(ToolName).

# 40.3.5 Recency Boost: Recently used tools likely still relevant
# Note: Full implementation would check timestamp difference
# Score: 15 (out of 100)
tool_recency_relevance(ToolName, 15) :-
    tool_usage_stats(ToolName, _, _, LastUsed),
    current_time(Now),
    LastUsed > 0.

tool_recency_relevance(ToolName, 0) :-
    tool_registered(ToolName, _),
    !has_tool_usage(ToolName).

# 40.3.6 Combined Score: Weighted sum of all relevance factors
# Base relevance weighted at 40%, intent at 30%, domain/success/recency fill rest
# Note: Mangle doesn't support arithmetic in rule bodies, so we use approximation
#       The Go implementation will compute exact scores

# Simplified relevance threshold: tool is relevant if it has base affinity >= 30
relevant_tool(ShardType, ToolName) :-
    tool_base_relevance(ShardType, ToolName, BaseScore),
    BaseScore >= 30.

# Also relevant if intent matches strongly (>= 70)
relevant_tool(ShardType, ToolName) :-
    tool_intent_relevance(ToolName, IntentScore),
    IntentScore >= 70,
    tool_registered(ToolName, _),
    current_shard_type(ShardType).

# System shards see all tools (Type S gets full visibility)
relevant_tool(/system, ToolName) :-
    tool_registered(ToolName, _).

# Helper Predicates

# has_current_intent() - helper for safe negation
has_current_intent() :- current_intent(_).

# has_tool_domain(ToolName) - helper for safe negation
has_tool_domain(ToolName) :- tool_domain(ToolName, _).

# has_tool_usage(ToolName) - helper for safe negation
has_tool_usage(ToolName) :- tool_usage_stats(ToolName, _, _, _).
