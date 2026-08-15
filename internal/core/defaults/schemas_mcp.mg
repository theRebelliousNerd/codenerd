# Cortex 1.6.0 Schemas (EDB Declarations)
# Version: 1.6.0
# Philosophy: Logic determines Reality; the Model merely describes it.

# Modular Schema: MCP (Model Context Protocol) Integration
# Section: 50

# =============================================================================
# SECTION 50: MCP TOOL INTEGRATION (JIT Tool Compiler)
# =============================================================================
# Predicates for MCP server connections and tool management.
# Enables intelligent tool serving via hybrid logic+vector selection.

# -----------------------------------------------------------------------------
# 50.1 MCP Server Registration
# -----------------------------------------------------------------------------

# mcp_server_registered(ServerID, Endpoint, Protocol, RegisteredAt)
# Records an MCP server connection.
# Protocol: /http, /stdio, /sse
Decl mcp_server_registered(ServerID, Endpoint, Protocol, RegisteredAt) bound [/string, /string, /name, /number].

# mcp_server_status(ServerID, Status)
# Current connection status.
# Status: /connected, /disconnected, /connecting, /error
Decl mcp_server_status(ServerID, Status) bound [/string, /name].

# mcp_server_capabilities(ServerID, Capability)
# Capabilities advertised by the server.
# Capability: /tools, /resources, /prompts, /logging
Decl mcp_server_capabilities(ServerID, Capability) bound [/string, /name].

# mcp_server_name(ServerID, Name)
# Human-readable server name.
Decl mcp_server_name(ServerID, Name) bound [/string, /string].

# -----------------------------------------------------------------------------
# 50.2 MCP Tool Registration
# -----------------------------------------------------------------------------

# mcp_tool_registered(ToolID, ServerID, RegisteredAt)
# Records an MCP tool discovered from a server.
# ToolID format: "server_id/tool_name"
Decl mcp_tool_registered(ToolID, ServerID, RegisteredAt) bound [/string, /string, /number].

# mcp_tool_name(ToolID, Name)
# The tool's name as advertised by the server.
Decl mcp_tool_name(ToolID, Name) bound [/string, /string].

# mcp_tool_description(ToolID, Description)
# The tool's description from the server.
Decl mcp_tool_description(ToolID, Description) bound [/string, /string].

# mcp_tool_condensed(ToolID, Condensed)
# LLM-generated one-line description (max 80 chars) for condensed rendering.
Decl mcp_tool_condensed(ToolID, Condensed) bound [/string, /string].

# -----------------------------------------------------------------------------
# 50.3 MCP Tool Metadata (LLM-Extracted)
# -----------------------------------------------------------------------------

# mcp_tool_capability(ToolID, Capability)
# Capabilities extracted by LLM analysis.
# Capability: /read, /write, /delete, /search, /transform, /execute, /analyze, /validate
Decl mcp_tool_capability(ToolID, Capability) bound [/string, /name].

# mcp_tool_category(ToolID, Category)
# Categories extracted by LLM analysis.
# Category: /filesystem, /code_analysis, /code_generation, /shell, /git, /web, /database, /api, /testing, /documentation, /search
Decl mcp_tool_category(ToolID, Category) bound [/string, /name].

# mcp_tool_domain(ToolID, Domain)
# Primary language/framework domain.
# Domain: /go, /python, /typescript, /rust, /java, /general
Decl mcp_tool_domain(ToolID, Domain) bound [/string, /name].

# mcp_tool_shard_affinity(ToolID, ShardType, Score)
# LLM-determined affinity score (0-100) for each shard type.
# ShardType: /coder, /tester, /reviewer, /researcher
Decl mcp_tool_shard_affinity(ToolID, ShardType, Score) bound [/string, /name, /number].

# mcp_tool_analyzed(ToolID)
# Indicates tool has been analyzed by LLM.
Decl mcp_tool_analyzed(ToolID) bound [/string].

# -----------------------------------------------------------------------------
# 50.4 MCP Tool Selection (Derived Predicates)
# -----------------------------------------------------------------------------

# mcp_tool_available(ToolID)
# Tool is available (server is connected).
Decl mcp_tool_available(ToolID) bound [/string].

# mcp_tool_vector_score(ToolID, Score)
# Semantic similarity score (0-100) from vector search.
# Asserted by Go code after embedding-based search.
Decl mcp_tool_vector_score(ToolID, Score) bound [/string, /number].

# mcp_tool_base_relevance(ShardType, ToolID, Score)
# Base relevance from shard affinity.
Decl mcp_tool_base_relevance(ShardType, ToolID, Score) bound [/name, /string, /number].

# mcp_tool_intent_boost(ToolID, Score)
# Bonus score when tool capability matches intent verb.
Decl mcp_tool_intent_boost(ToolID, Score) bound [/string, /number].

# mcp_tool_domain_boost(ToolID, Score)
# Bonus score when tool domain matches target language.
Decl mcp_tool_domain_boost(ToolID, Score) bound [/string, /number].

# mcp_tool_relevance(ShardType, ToolID, Score)
# Combined relevance score (logic * 0.7 + vector * 0.3).
Decl mcp_tool_relevance(ShardType, ToolID, Score) bound [/name, /string, /number].

# mcp_tool_selected(ShardType, ToolID, RenderMode)
# Final tool selection with render mode.
# RenderMode: /full, /condensed, /minimal
Decl mcp_tool_selected(ShardType, ToolID, RenderMode) bound [/name, /string, /name].

# mcp_tool_skeleton(ToolID)
# Tool is a skeleton tool (always selected for certain categories).
Decl mcp_tool_skeleton(ToolID) bound [/string].

# -----------------------------------------------------------------------------
# 50.5 MCP Tool Usage Statistics
# -----------------------------------------------------------------------------

# mcp_tool_usage(ToolID, UsageCount, SuccessCount)
# Aggregated usage statistics.
Decl mcp_tool_usage(ToolID, UsageCount, SuccessCount) bound [/string, /number, /number].

# mcp_tool_last_used(ToolID, Timestamp)
# Last usage timestamp.
Decl mcp_tool_last_used(ToolID, Timestamp) bound [/string, /number].

# mcp_tool_success_rate(ToolID, Rate)
# Success rate (0-100) derived from usage statistics.
Decl mcp_tool_success_rate(ToolID, Rate) bound [/string, /number].

# mcp_tool_avg_latency(ToolID, LatencyMs)
# Rolling average call latency in milliseconds.
Decl mcp_tool_avg_latency(ToolID, LatencyMs) bound [/string, /number].

# -----------------------------------------------------------------------------
# 50.6 Intent-Capability Mapping (for tool selection)
# -----------------------------------------------------------------------------

# NOTE: intent_requires_capability/3 is declared in schemas_tools.mg
# We use it here for MCP tool selection but don't re-declare it.

# mcp_intent_requires_capability(Verb, Capability)
# MCP-local verb -> tool-capability mapping. Deliberately NOT folded into the
# shared intent_requires_capability/3 table: that table is keyed on coarse
# capability *categories* (/generation, /validation, ...) consumed by static
# tool routing in policy/tool_routing.mg, whereas MCP capabilities are fine
# grained verbs (/read, /write, ...). Adding MCP rows to the shared table would
# silently widen static tool relevance for every shard.
Decl mcp_intent_requires_capability(Verb, Capability) bound [/name, /name].

# -----------------------------------------------------------------------------
# 50.7 Selection Support Predicates (IDB scaffolding)
# -----------------------------------------------------------------------------

# mcp_shard_type(ShardType)
# The universe of shard types tool selection may be asked about. Needed because
# a rule head may not contain an unbound wildcard: skeleton tools are selected
# for every shard, which requires enumerating shards positively.
Decl mcp_shard_type(ShardType) bound [/name].

# mcp_tool_intent_boost_candidate(ToolID, Score)
# Per-match intent boost candidates; the winning boost is the max (see policy).
Decl mcp_tool_intent_boost_candidate(ToolID, Score) bound [/string, /number].

# mcp_tool_domain_boost_candidate(ToolID, Score)
# Per-match domain boost candidates; the winning boost is the max.
Decl mcp_tool_domain_boost_candidate(ToolID, Score) bound [/string, /number].

# mcp_tool_usage_boost(ToolID, Score) / _candidate
# Reward for tools with a proven success record.
Decl mcp_tool_usage_boost(ToolID, Score) bound [/string, /number].
Decl mcp_tool_usage_boost_candidate(ToolID, Score) bound [/string, /number].

# mcp_tool_usage_penalty(ToolID, Score) / _candidate
# Penalty for tools that fail often or are consistently slow.
Decl mcp_tool_usage_penalty(ToolID, Score) bound [/string, /number].
Decl mcp_tool_usage_penalty_candidate(ToolID, Score) bound [/string, /number].

# mcp_tool_logic_score(ShardType, ToolID, Score)
# Pure-logic score before vector blending: base + boosts - penalties.
Decl mcp_tool_logic_score(ShardType, ToolID, Score) bound [/name, /string, /number].

# mcp_tool_has_vector_score(ToolID)
# Positive guard so the vector-free relevance rule can use safe negation.
Decl mcp_tool_has_vector_score(ToolID) bound [/string].

# -----------------------------------------------------------------------------
# 50.8 MCP Lifecycle
# -----------------------------------------------------------------------------

# mcp_integration_ready(ServerCount, ToolCount)
# Asserted once boot-time ConnectAll and the initial tool discovery finish.
# Absence means the catalog is still filling in, not that MCP is unavailable.
Decl mcp_integration_ready(ServerCount, ToolCount) bound [/number, /number].

# -----------------------------------------------------------------------------
# 50.9 MCP Resources and Prompts
# -----------------------------------------------------------------------------
# MCP servers expose three primitive kinds; tools are only one of them. These
# make the other two visible so planning can ask "is there already a resource
# that answers this?" before spending a tool call.

# mcp_resource_registered(ServerID, URI)
Decl mcp_resource_registered(ServerID, URI) bound [/string, /string].

# mcp_resource_mime(URI, MimeType)
Decl mcp_resource_mime(URI, MimeType) bound [/string, /string].

# mcp_resource_name(URI, Name)
Decl mcp_resource_name(URI, Name) bound [/string, /string].

# mcp_prompt_registered(ServerID, PromptName)
Decl mcp_prompt_registered(ServerID, PromptName) bound [/string, /string].

# mcp_prompt_argument(PromptName, ArgumentName, Required)
# Required: /true, /false
Decl mcp_prompt_argument(PromptName, ArgumentName, Required) bound [/string, /string, /name].

# =============================================================================
# END SECTION 50
# =============================================================================
