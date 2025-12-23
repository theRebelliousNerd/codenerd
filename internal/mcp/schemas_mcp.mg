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
Decl mcp_server_registered(ServerID, Endpoint, Protocol, RegisteredAt).

# mcp_server_status(ServerID, Status)
# Current connection status.
# Status: /connected, /disconnected, /connecting, /error
Decl mcp_server_status(ServerID, Status).

# mcp_server_capabilities(ServerID, Capability)
# Capabilities advertised by the server.
# Capability: /tools, /resources, /prompts, /logging
Decl mcp_server_capabilities(ServerID, Capability).

# mcp_server_name(ServerID, Name)
# Human-readable server name.
Decl mcp_server_name(ServerID, Name).

# -----------------------------------------------------------------------------
# 50.2 MCP Tool Registration
# -----------------------------------------------------------------------------

# mcp_tool_registered(ToolID, ServerID, RegisteredAt)
# Records an MCP tool discovered from a server.
# ToolID format: "server_id/tool_name"
Decl mcp_tool_registered(ToolID, ServerID, RegisteredAt).

# mcp_tool_name(ToolID, Name)
# The tool's name as advertised by the server.
Decl mcp_tool_name(ToolID, Name).

# mcp_tool_description(ToolID, Description)
# The tool's description from the server.
Decl mcp_tool_description(ToolID, Description).

# mcp_tool_condensed(ToolID, Condensed)
# LLM-generated one-line description (max 80 chars) for condensed rendering.
Decl mcp_tool_condensed(ToolID, Condensed).

# -----------------------------------------------------------------------------
# 50.3 MCP Tool Metadata (LLM-Extracted)
# -----------------------------------------------------------------------------

# mcp_tool_capability(ToolID, Capability)
# Capabilities extracted by LLM analysis.
# Capability: /read, /write, /delete, /search, /transform, /execute, /analyze, /validate
Decl mcp_tool_capability(ToolID, Capability).

# mcp_tool_category(ToolID, Category)
# Categories extracted by LLM analysis.
# Category: /filesystem, /code_analysis, /code_generation, /shell, /git, /web, /database, /api, /testing, /documentation, /search
Decl mcp_tool_category(ToolID, Category).

# mcp_tool_domain(ToolID, Domain)
# Primary language/framework domain.
# Domain: /go, /python, /typescript, /rust, /java, /general
Decl mcp_tool_domain(ToolID, Domain).

# mcp_tool_shard_affinity(ToolID, ShardType, Score)
# LLM-determined affinity score (0-100) for each shard type.
# ShardType: /coder, /tester, /reviewer, /researcher
Decl mcp_tool_shard_affinity(ToolID, ShardType, Score).

# mcp_tool_analyzed(ToolID)
# Indicates tool has been analyzed by LLM.
Decl mcp_tool_analyzed(ToolID).

# -----------------------------------------------------------------------------
# 50.4 MCP Tool Selection (Derived Predicates)
# -----------------------------------------------------------------------------

# mcp_tool_available(ToolID)
# Tool is available (server is connected).
Decl mcp_tool_available(ToolID).

# mcp_tool_vector_score(ToolID, Score)
# Semantic similarity score (0-100) from vector search.
# Asserted by Go code after embedding-based search.
Decl mcp_tool_vector_score(ToolID, Score).

# mcp_tool_base_relevance(ShardType, ToolID, Score)
# Base relevance from shard affinity.
Decl mcp_tool_base_relevance(ShardType, ToolID, Score).

# mcp_tool_intent_boost(ToolID, Score)
# Bonus score when tool capability matches intent verb.
Decl mcp_tool_intent_boost(ToolID, Score).

# mcp_tool_domain_boost(ToolID, Score)
# Bonus score when tool domain matches target language.
Decl mcp_tool_domain_boost(ToolID, Score).

# mcp_tool_relevance(ShardType, ToolID, Score)
# Combined relevance score (logic * 0.7 + vector * 0.3).
Decl mcp_tool_relevance(ShardType, ToolID, Score).

# mcp_tool_selected(ShardType, ToolID, RenderMode)
# Final tool selection with render mode.
# RenderMode: /full, /condensed, /minimal
Decl mcp_tool_selected(ShardType, ToolID, RenderMode).

# mcp_tool_skeleton(ToolID)
# Tool is a skeleton tool (always selected for certain categories).
Decl mcp_tool_skeleton(ToolID).

# -----------------------------------------------------------------------------
# 50.5 MCP Tool Usage Statistics
# -----------------------------------------------------------------------------

# mcp_tool_usage(ToolID, UsageCount, SuccessCount)
# Aggregated usage statistics.
Decl mcp_tool_usage(ToolID, UsageCount, SuccessCount).

# mcp_tool_last_used(ToolID, Timestamp)
# Last usage timestamp.
Decl mcp_tool_last_used(ToolID, Timestamp).

# mcp_tool_success_rate(ToolID, Rate)
# Success rate (0-100) derived from usage statistics.
Decl mcp_tool_success_rate(ToolID, Rate).

# -----------------------------------------------------------------------------
# 50.6 Intent-Capability Mapping (for tool selection)
# -----------------------------------------------------------------------------

# intent_requires_capability(Verb, Capability, Weight)
# Maps intent verbs to required capabilities with weights.
# Used by tool selection to boost tools matching current intent.
Decl intent_requires_capability(Verb, Capability, Weight).

# =============================================================================
# END SECTION 50
# =============================================================================
