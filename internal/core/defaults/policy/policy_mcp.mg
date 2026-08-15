# Cortex 1.6.0 Policy Rules (IDB)
# Version: 1.6.0
# Philosophy: Logic determines Reality; the Model merely describes it.

# Modular Policy: MCP Tool Selection
# Section: 50
#
# Relocated from internal/mcp/policy_mcp.mg. The kernel policy loader walks
# defaults/policy/*.mg; a package-local .mg file was never in that walk, so
# every mcp_tool_selected query returned nothing and the JIT tool compiler
# always fell back to the Go affinity heuristic.

# =============================================================================
# SECTION 50: MCP TOOL SELECTION POLICY
# =============================================================================
# Rules for intelligent MCP tool serving via hybrid logic+vector selection.
# Implements JIT Tool Compiler skeleton/flesh pattern.
#
# EDB inputs (asserted by internal/mcp on connect / discover / usage):
#   mcp_server_registered, mcp_server_status, mcp_server_capabilities,
#   mcp_server_name, mcp_tool_registered, mcp_tool_name, mcp_tool_description,
#   mcp_tool_condensed, mcp_tool_capability, mcp_tool_category,
#   mcp_tool_domain, mcp_tool_shard_affinity, mcp_tool_analyzed,
#   mcp_tool_usage, mcp_tool_last_used, mcp_tool_avg_latency
# Transient EDB (asserted per compile, retracted after):
#   mcp_tool_vector_score

# -----------------------------------------------------------------------------
# 50.0 Shard Type Universe
# -----------------------------------------------------------------------------
# Skeleton tools are selected for every shard. A head variable must be bound,
# so shards are enumerated positively rather than written as a wildcard.

mcp_shard_type(/coder).
mcp_shard_type(/tester).
mcp_shard_type(/reviewer).
mcp_shard_type(/researcher).
mcp_shard_type(/generalist).
mcp_shard_type(/specialist).

# Any shard the static tool-routing table knows about is also a valid MCP
# selection target, so a new shard type only has to be declared in one place.
mcp_shard_type(ShardType) :-
    shard_capability_affinity(ShardType, _, _).

# Any shard an MCP tool was scored against is selectable even if it is not in
# either table above (LLM analysis may invent specialist shard names).
mcp_shard_type(ShardType) :-
    mcp_tool_shard_affinity(_, ShardType, _).

# -----------------------------------------------------------------------------
# 50.1 Tool Availability
# -----------------------------------------------------------------------------

# Tool is available if its server is connected
mcp_tool_available(ToolID) :-
    mcp_tool_registered(ToolID, ServerID, _),
    mcp_server_status(ServerID, /connected).

# Tool is available but offline (cached) if server disconnected. Offline tools
# stay selectable so the LLM can still reason about the catalog; the call path
# returns a soft failure rather than a hallucinated success.
mcp_tool_available(ToolID) :-
    mcp_tool_registered(ToolID, ServerID, _),
    mcp_server_status(ServerID, /disconnected).

# -----------------------------------------------------------------------------
# 50.2 Base Relevance (from Shard Affinity)
# -----------------------------------------------------------------------------

# Base relevance from shard affinity (must have affinity >= 30 to be relevant)
mcp_tool_base_relevance(ShardType, ToolID, Score) :-
    mcp_tool_shard_affinity(ToolID, ShardType, Score),
    Score >= 30,
    mcp_tool_available(ToolID).

# -----------------------------------------------------------------------------
# 50.3 Intent Boost
# -----------------------------------------------------------------------------
# Candidates are collected, then reduced with fn:max. Emitting the boost
# directly produced one relevance row per matching capability, and the tool was
# then selected at several render modes at once.

# MCP-local verb -> capability mapping
mcp_tool_intent_boost_candidate(ToolID, 30) :-
    current_intent(IntentID),
    user_intent(IntentID, _, Verb, _, _),
    mcp_intent_requires_capability(Verb, Cap),
    mcp_tool_capability(ToolID, Cap),
    mcp_tool_available(ToolID).

# Shared verb -> capability-category mapping (policy/tool_routing.mg), so an
# MCP tool tagged with a coarse category still benefits from the global table.
mcp_tool_intent_boost_candidate(ToolID, 30) :-
    current_intent(IntentID),
    user_intent(IntentID, _, Verb, _, _),
    intent_requires_capability(Verb, Cap, _),
    mcp_tool_capability(ToolID, Cap),
    mcp_tool_available(ToolID).

# Floor so every available tool has exactly one boost value.
mcp_tool_intent_boost_candidate(ToolID, 0) :-
    mcp_tool_available(ToolID).

mcp_tool_intent_boost(ToolID, Boost) :-
    mcp_tool_intent_boost_candidate(ToolID, Candidate) |>
    do fn:group_by(ToolID),
    let Boost = fn:max(Candidate).

# -----------------------------------------------------------------------------
# 50.4 Domain Boost
# -----------------------------------------------------------------------------

# Tool gets domain boost when domain matches target file language
mcp_tool_domain_boost_candidate(ToolID, 20) :-
    current_intent(IntentID),
    user_intent(IntentID, _, _, Target, _),
    file_topology(Target, _, Lang, _, _),
    mcp_tool_domain(ToolID, Lang),
    mcp_tool_available(ToolID).

# General domain tools get reduced boost (10)
mcp_tool_domain_boost_candidate(ToolID, 10) :-
    mcp_tool_domain(ToolID, /general),
    mcp_tool_available(ToolID).

mcp_tool_domain_boost_candidate(ToolID, 0) :-
    mcp_tool_available(ToolID).

mcp_tool_domain_boost(ToolID, Boost) :-
    mcp_tool_domain_boost_candidate(ToolID, Candidate) |>
    do fn:group_by(ToolID),
    let Boost = fn:max(Candidate).

# -----------------------------------------------------------------------------
# 50.5 Usage Feedback
# -----------------------------------------------------------------------------
# Success rate is derived here rather than in Go: the kernel owns the formula,
# Go only reports the raw counters.

mcp_tool_success_rate(ToolID, Rate) :-
    mcp_tool_usage(ToolID, Used, Succeeded),
    Used > 0,
    Rate = fn:div(fn:mult(Succeeded, 100), Used).

# Proven tools (>= 3 calls, >= 80% success) get a boost.
mcp_tool_usage_boost_candidate(ToolID, 15) :-
    mcp_tool_success_rate(ToolID, Rate),
    Rate >= 80,
    mcp_tool_usage(ToolID, Used, _),
    Used >= 3.

mcp_tool_usage_boost_candidate(ToolID, 0) :-
    mcp_tool_available(ToolID).

mcp_tool_usage_boost(ToolID, Boost) :-
    mcp_tool_usage_boost_candidate(ToolID, Candidate) |>
    do fn:group_by(ToolID),
    let Boost = fn:max(Candidate).

# Unreliable tools (>= 3 calls, < 50% success) are demoted.
mcp_tool_usage_penalty_candidate(ToolID, 20) :-
    mcp_tool_success_rate(ToolID, Rate),
    Rate < 50,
    mcp_tool_usage(ToolID, Used, _),
    Used >= 3.

# Consistently slow tools (>= 5s average) are demoted; latency crowds out the
# rest of the shard's turn budget.
mcp_tool_usage_penalty_candidate(ToolID, 10) :-
    mcp_tool_avg_latency(ToolID, LatencyMs),
    LatencyMs >= 5000,
    mcp_tool_available(ToolID).

mcp_tool_usage_penalty_candidate(ToolID, 0) :-
    mcp_tool_available(ToolID).

mcp_tool_usage_penalty(ToolID, Penalty) :-
    mcp_tool_usage_penalty_candidate(ToolID, Candidate) |>
    do fn:group_by(ToolID),
    let Penalty = fn:max(Candidate).

# -----------------------------------------------------------------------------
# 50.6 Combined Relevance Score
# -----------------------------------------------------------------------------

# Pure-logic score: base affinity plus boosts, minus penalties.
mcp_tool_logic_score(ShardType, ToolID, Score) :-
    mcp_tool_base_relevance(ShardType, ToolID, Base),
    mcp_tool_intent_boost(ToolID, IntentBoost),
    mcp_tool_domain_boost(ToolID, DomainBoost),
    mcp_tool_usage_boost(ToolID, UsageBoost),
    mcp_tool_usage_penalty(ToolID, Penalty),
    Rewarded = fn:plus(Base, IntentBoost, DomainBoost, UsageBoost),
    Score = fn:minus(Rewarded, Penalty).

mcp_tool_has_vector_score(ToolID) :-
    mcp_tool_vector_score(ToolID, _).

# Combined score = Logic * 0.7 + Vector * 0.3, in integer arithmetic.
mcp_tool_relevance(ShardType, ToolID, CombinedScore) :-
    mcp_tool_logic_score(ShardType, ToolID, LogicScore),
    mcp_tool_vector_score(ToolID, VectorScore),
    WeightedLogic = fn:div(fn:mult(LogicScore, 7), 10),
    WeightedVector = fn:div(fn:mult(VectorScore, 3), 10),
    CombinedScore = fn:plus(WeightedLogic, WeightedVector).

# Logic only when Go could not produce an embedding score for this tool.
mcp_tool_relevance(ShardType, ToolID, LogicScore) :-
    mcp_tool_logic_score(ShardType, ToolID, LogicScore),
    !mcp_tool_has_vector_score(ToolID).

# -----------------------------------------------------------------------------
# 50.7 Skeleton Tools (Always Selected)
# -----------------------------------------------------------------------------

# Filesystem read tools are skeleton tools (always needed)
mcp_tool_skeleton(ToolID) :-
    mcp_tool_category(ToolID, /filesystem),
    mcp_tool_capability(ToolID, /read).

# Search tools are skeleton tools
mcp_tool_skeleton(ToolID) :-
    mcp_tool_category(ToolID, /search),
    mcp_tool_capability(ToolID, /search).

# Skeleton tools always get full render, for every shard.
mcp_tool_selected(ShardType, ToolID, /full) :-
    mcp_shard_type(ShardType),
    mcp_tool_skeleton(ToolID),
    mcp_tool_available(ToolID).

# -----------------------------------------------------------------------------
# 50.8 Tool Selection (Render Mode Assignment)
# -----------------------------------------------------------------------------
# Skeleton tools are excluded from the scored tiers: without the guard a
# skeleton tool was emitted at /full *and* at its score tier, and the compiler
# rendered it twice.

# Full render for high relevance (score >= 70)
mcp_tool_selected(ShardType, ToolID, /full) :-
    mcp_tool_relevance(ShardType, ToolID, Score),
    Score >= 70,
    !mcp_tool_skeleton(ToolID).

# Condensed render for medium relevance (40 <= score < 70)
mcp_tool_selected(ShardType, ToolID, /condensed) :-
    mcp_tool_relevance(ShardType, ToolID, Score),
    Score >= 40,
    Score < 70,
    !mcp_tool_skeleton(ToolID).

# Minimal render for low relevance (20 <= score < 40)
mcp_tool_selected(ShardType, ToolID, /minimal) :-
    mcp_tool_relevance(ShardType, ToolID, Score),
    Score >= 20,
    Score < 40,
    !mcp_tool_skeleton(ToolID).

# -----------------------------------------------------------------------------
# 50.9 Intent-Capability Mapping (EDB - Static Data)
# -----------------------------------------------------------------------------
# MCP-local: maps intent verbs onto the fine-grained capability atoms produced
# by internal/mcp/analyzer.go (/read, /write, /delete, /search, /transform,
# /execute, /analyze, /validate).

# Read/Analyze intents require read capability
mcp_intent_requires_capability(/read, /read).
mcp_intent_requires_capability(/view, /read).
mcp_intent_requires_capability(/show, /read).
mcp_intent_requires_capability(/explain, /read).
mcp_intent_requires_capability(/analyze, /analyze).
mcp_intent_requires_capability(/review, /analyze).
mcp_intent_requires_capability(/inspect, /analyze).
mcp_intent_requires_capability(/debug, /analyze).

# Write/Create intents require write capability
mcp_intent_requires_capability(/write, /write).
mcp_intent_requires_capability(/create, /write).
mcp_intent_requires_capability(/add, /write).
mcp_intent_requires_capability(/update, /write).
mcp_intent_requires_capability(/modify, /write).
mcp_intent_requires_capability(/implement, /write).
mcp_intent_requires_capability(/fix, /write).

# Search/Find intents require search capability
mcp_intent_requires_capability(/search, /search).
mcp_intent_requires_capability(/find, /search).
mcp_intent_requires_capability(/grep, /search).
mcp_intent_requires_capability(/locate, /search).
mcp_intent_requires_capability(/research, /search).
mcp_intent_requires_capability(/explore, /search).

# Execute intents require execute capability
mcp_intent_requires_capability(/run, /execute).
mcp_intent_requires_capability(/execute, /execute).
mcp_intent_requires_capability(/test, /execute).
mcp_intent_requires_capability(/build, /execute).

# Verification intents require validate capability
mcp_intent_requires_capability(/test, /validate).
mcp_intent_requires_capability(/verify, /validate).
mcp_intent_requires_capability(/validate, /validate).

# Transform intents require transform capability
mcp_intent_requires_capability(/format, /transform).
mcp_intent_requires_capability(/convert, /transform).
mcp_intent_requires_capability(/refactor, /transform).

# Delete intents require delete capability
mcp_intent_requires_capability(/delete, /delete).
mcp_intent_requires_capability(/remove, /delete).
mcp_intent_requires_capability(/clear, /delete).

# =============================================================================
# END SECTION 50
# =============================================================================
