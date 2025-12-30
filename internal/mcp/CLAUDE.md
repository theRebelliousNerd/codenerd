# MCP (Model Context Protocol) Integration

JIT Tool Compiler for intelligent MCP tool serving based on task context.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                     JIT TOOL COMPILER                           │
├─────────────────────────────────────────────────────────────────┤
│  MCPClientManager → ToolAnalyzer → MCPToolStore → Mangle Facts │
│                                          ↓                      │
│  TaskContext → Vector Search + Mangle Logic → Tool Selection   │
│                                          ↓                      │
│  ToolRenderer → Full/Condensed/Minimal → LLM Context           │
└─────────────────────────────────────────────────────────────────┘
```

## Core Components

| Component | File | Purpose |
|-----------|------|---------|
| Types | `types.go` | Core type definitions (MCPServer, MCPTool, etc.) |
| Client Manager | `client.go` | Server connections, protocol negotiation |
| HTTP Transport | `transport_http.go` | HTTP-based MCP communication |
| Tool Store | `store.go` | SQLite storage with embeddings |
| Tool Analyzer | `analyzer.go` | LLM-based metadata extraction |
| JIT Compiler | `compiler.go` | Tool selection pipeline |
| Renderer | `renderer.go` | Tool set rendering for LLM |

## Design Principles

### 1. Skeleton/Flesh Bifurcation

Mirrors the JIT Prompt Compiler pattern:

- **Skeleton Tools**: Always available (filesystem read, shell exec)
  - Selected via Mangle logic rules
  - Never omitted regardless of context

- **Flesh Tools**: Context-dependent
  - Hybrid scoring: (Logic × 0.7) + (Vector × 0.3)
  - Rendered in three tiers based on relevance

### 2. Three-Tier Rendering

| Tier | Threshold | Content |
|------|-----------|---------|
| Full | ≥70 | Complete JSON schema, description, examples |
| Condensed | 40-69 | Name + one-line description |
| Minimal | 20-39 | Name only (available on request) |
| Excluded | <20 | Not sent to LLM |

### 3. LLM-Powered Tool Analysis

On first discovery, each tool is analyzed by LLM to extract:
- Categories (filesystem, code_analysis, web, etc.)
- Capabilities (/read, /write, /search, /transform)
- Domain affinity (/go, /python, /general)
- Shard affinities with scores 0-100
- Condensed description for medium-relevance rendering

### 4. Graceful Fallback

When MCP servers disconnect:
- Tools remain cached with `status: offline`
- Selection continues (may select offline tools)
- Execution fails gracefully with clear error

## Configuration

### config.json integrations section

```json
{
  "integrations": {
    "code_graph": {
      "enabled": true,
      "protocol": "http",
      "base_url": "http://localhost:8080",
      "timeout": "30s",
      "auto_connect": true,
      "auto_discover_tools": true
    }
  },
  "tool_selection": {
    "full_threshold": 70,
    "condensed_threshold": 40,
    "minimal_threshold": 20,
    "logic_weight": 0.7,
    "vector_weight": 0.3,
    "max_full_tools": 10,
    "token_budget": 4000
  }
}
```

### available_tools.json MCP entries

```json
{
  "name": "semantic_search",
  "type": "mcp",
  "mcp_server": "code_graph",
  "mcp_tool": "search",
  "category": "code_analysis",
  "shard_affinity": "CoderShard"
}
```

## Mangle Integration

### EDB Predicates (Facts)

```mangle
mcp_server_registered(ServerID, Endpoint, Protocol, RegisteredAt).
mcp_server_status(ServerID, Status).
mcp_tool_registered(ToolID, ServerID, RegisteredAt).
mcp_tool_capability(ToolID, Capability).
mcp_tool_category(ToolID, Category).
mcp_tool_shard_affinity(ToolID, ShardType, Score).
mcp_tool_vector_score(ToolID, Score).  # Asserted by Go after vector search
```

### IDB Predicates (Derived)

```mangle
mcp_tool_available(ToolID).           # Server connected
mcp_tool_relevance(ShardType, ToolID, Score).  # Combined score
mcp_tool_selected(ShardType, ToolID, RenderMode).  # Final selection
```

## Usage Flow

1. **Startup**: MCPClientManager connects to configured servers
2. **Discovery**: ListTools() called, new tools sent to ToolAnalyzer
3. **Analysis**: LLM extracts metadata, embeddings generated
4. **Storage**: Tools persisted to SQLite with embeddings
5. **Fact Injection**: Mangle facts asserted for each tool
6. **Compilation**: JITToolCompiler called during shard spawn
7. **Selection**: Vector search + Mangle logic determines tools
8. **Rendering**: ToolRenderer produces LLM-consumable output

## SQLite Schema

```sql
-- mcp_servers: Server connection state
-- mcp_tools: Tool definitions with LLM-extracted metadata
-- mcp_tool_vec: Vector index for semantic search (sqlite-vec)
```

## File Index

| File | Lines | Purpose |
|------|-------|---------|
| `CLAUDE.md` | - | This documentation |
| `types.go` | ~200 | Core type definitions |
| `client.go` | ~300 | Server connection management |
| `transport_http.go` | ~150 | HTTP transport implementation |
| `store.go` | ~400 | SQLite storage with embeddings |
| `analyzer.go` | ~200 | LLM tool analysis |
| `compiler.go` | ~300 | JIT tool selection |
| `renderer.go` | ~100 | Output formatting |
| `schemas_mcp.mg` | ~50 | Mangle schema declarations |
| `policy_mcp.mg` | ~100 | Mangle selection rules |

---

**Remember: Push to GitHub regularly!**
