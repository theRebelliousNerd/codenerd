# Cortex 1.6.0 Policy Rules (IDB)
# Version: 1.6.0
# Philosophy: Logic determines Reality; the Model merely describes it.

# Modular Policy: MCP Tool Selection
# Section: 50

# =============================================================================
# SECTION 50: MCP TOOL SELECTION POLICY
# =============================================================================
# Rules for intelligent MCP tool serving via hybrid logic+vector selection.
# Implements JIT Tool Compiler skeleton/flesh pattern.

# -----------------------------------------------------------------------------
# 50.1 Tool Availability
# -----------------------------------------------------------------------------

# Tool is available if its server is connected
mcp_tool_available(ToolID) :-
    mcp_tool_registered(ToolID, ServerID, _),
    mcp_server_status(ServerID, /connected).

# Tool is available but offline (cached) if server disconnected
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

# Tool gets intent boost when capability matches current intent verb
mcp_tool_intent_boost(ToolID, 30) :-
    current_intent(IntentID),
    user_intent(IntentID, _, Verb, _, _),
    intent_requires_capability(Verb, Cap, _),
    mcp_tool_capability(ToolID, Cap).

# Zero boost if no matching capability
mcp_tool_intent_boost(ToolID, 0) :-
    mcp_tool_registered(ToolID, _, _),
    mcp_tool_available(ToolID).

# -----------------------------------------------------------------------------
# 50.4 Domain Boost
# -----------------------------------------------------------------------------

# Tool gets domain boost when domain matches target file language
mcp_tool_domain_boost(ToolID, 20) :-
    current_intent(IntentID),
    user_intent(IntentID, _, _, Target, _),
    file_topology(Target, _, Lang, _, _),
    mcp_tool_domain(ToolID, Lang).

# General domain tools get reduced boost (10)
mcp_tool_domain_boost(ToolID, 10) :-
    mcp_tool_domain(ToolID, /general),
    mcp_tool_available(ToolID).

# -----------------------------------------------------------------------------
# 50.5 Combined Relevance Score
# -----------------------------------------------------------------------------

# Combined score = (BaseScore + IntentBoost + DomainBoost) * 0.7 + VectorScore * 0.3
# Note: In Mangle, we compute this approximately using integer math

# Full combined score when vector score exists
mcp_tool_relevance(ShardType, ToolID, CombinedScore) :-
    mcp_tool_base_relevance(ShardType, ToolID, BaseScore),
    mcp_tool_intent_boost(ToolID, IntentBoost),
    mcp_tool_domain_boost(ToolID, DomainBoost),
    mcp_tool_vector_score(ToolID, VectorScore),
    LogicPart = fn:plus(fn:plus(BaseScore, IntentBoost), DomainBoost),
    WeightedLogic = fn:div(fn:mult(LogicPart, 7), 10),
    WeightedVector = fn:div(fn:mult(VectorScore, 3), 10),
    CombinedScore = fn:plus(WeightedLogic, WeightedVector).

# Fallback when no vector score (use logic only)
mcp_tool_relevance(ShardType, ToolID, LogicScore) :-
    mcp_tool_base_relevance(ShardType, ToolID, BaseScore),
    mcp_tool_intent_boost(ToolID, IntentBoost),
    mcp_tool_domain_boost(ToolID, DomainBoost),
    LogicScore = fn:plus(fn:plus(BaseScore, IntentBoost), DomainBoost).

# -----------------------------------------------------------------------------
# 50.6 Tool Selection (Render Mode Assignment)
# -----------------------------------------------------------------------------

# Full render for high relevance (score >= 70)
mcp_tool_selected(ShardType, ToolID, /full) :-
    mcp_tool_relevance(ShardType, ToolID, Score),
    Score >= 70.

# Condensed render for medium relevance (40 <= score < 70)
mcp_tool_selected(ShardType, ToolID, /condensed) :-
    mcp_tool_relevance(ShardType, ToolID, Score),
    Score >= 40,
    Score < 70.

# Minimal render for low relevance (20 <= score < 40)
mcp_tool_selected(ShardType, ToolID, /minimal) :-
    mcp_tool_relevance(ShardType, ToolID, Score),
    Score >= 20,
    Score < 40.

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

# Skeleton tools always get full render
mcp_tool_selected(_, ToolID, /full) :-
    mcp_tool_skeleton(ToolID),
    mcp_tool_available(ToolID).

# -----------------------------------------------------------------------------
# 50.8 Intent-Capability Mapping (EDB - Static Data)
# -----------------------------------------------------------------------------

# Read/Analyze intents require read capability
intent_requires_capability(/read, /read, 100).
intent_requires_capability(/view, /read, 100).
intent_requires_capability(/show, /read, 100).
intent_requires_capability(/analyze, /analyze, 100).
intent_requires_capability(/inspect, /analyze, 80).

# Write/Create intents require write capability
intent_requires_capability(/write, /write, 100).
intent_requires_capability(/create, /write, 100).
intent_requires_capability(/add, /write, 80).
intent_requires_capability(/update, /write, 80).
intent_requires_capability(/modify, /write, 80).

# Search/Find intents require search capability
intent_requires_capability(/search, /search, 100).
intent_requires_capability(/find, /search, 100).
intent_requires_capability(/grep, /search, 80).
intent_requires_capability(/locate, /search, 80).

# Execute intents require execute capability
intent_requires_capability(/run, /execute, 100).
intent_requires_capability(/execute, /execute, 100).
intent_requires_capability(/test, /execute, 80).
intent_requires_capability(/build, /execute, 80).

# Transform intents require transform capability
intent_requires_capability(/format, /transform, 100).
intent_requires_capability(/convert, /transform, 100).
intent_requires_capability(/refactor, /transform, 80).

# Delete intents require delete capability
intent_requires_capability(/delete, /delete, 100).
intent_requires_capability(/remove, /delete, 100).
intent_requires_capability(/clear, /delete, 80).

# =============================================================================
# END SECTION 50
# =============================================================================
