# internal/mcp/

MCP (Model Context Protocol) Integration - JIT Tool Compiler for intelligent tool serving.

**Architecture Version:** 2.0.0 (December 2024 - JIT-Driven)

## Overview

The MCP package provides a JIT Tool Compiler for intelligent MCP tool serving based on task context. It enables dynamic tool discovery, analysis, and selection from MCP servers.

## Architecture

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

## Structure

```
mcp/
├── types.go            # Core type definitions
├── client.go           # Server connection management
├── transport_http.go   # HTTP transport implementation
├── store.go            # SQLite storage with embeddings
├── analyzer.go         # LLM-based tool analysis
├── compiler.go         # JIT tool selection pipeline
├── renderer.go         # Tool set rendering for LLM
├── schemas_mcp.mg      # Mangle schema declarations
└── policy_mcp.mg       # Mangle selection rules
```

## Key Concepts

### Skeleton/Flesh Bifurcation

Mirrors the JIT Prompt Compiler pattern:

| Type | Selection | Description |
|------|-----------|-------------|
| **Skeleton** | Mangle logic | Always available (filesystem, shell) |
| **Flesh** | Hybrid scoring | Context-dependent, (Logic × 0.7) + (Vector × 0.3) |

### Three-Tier Rendering

| Tier | Score | Content |
|------|-------|---------|
| Full | ≥70 | Complete JSON schema, description, examples |
| Condensed | 40-69 | Name + one-line description |
| Minimal | 20-39 | Name only (available on request) |
| Excluded | <20 | Not sent to LLM |

## Mangle Integration

```mangle
# EDB (Facts)
mcp_server_registered(ServerID, Endpoint, Protocol, RegisteredAt).
mcp_tool_capability(ToolID, Capability).
mcp_tool_shard_affinity(ToolID, ShardType, Score).

# IDB (Derived)
mcp_tool_available(ToolID).
mcp_tool_selected(ShardType, ToolID, RenderMode).
```

## Configuration

```json
{
  "integrations": {
    "code_graph": {
      "enabled": true,
      "protocol": "http",
      "base_url": "http://localhost:8080",
      "auto_discover_tools": true
    }
  },
  "tool_selection": {
    "full_threshold": 70,
    "condensed_threshold": 40,
    "logic_weight": 0.7,
    "vector_weight": 0.3,
    "max_full_tools": 10
  }
}
```

## Usage Flow

1. **Startup**: MCPClientManager connects to configured servers
2. **Discovery**: `ListTools()` called, new tools sent to ToolAnalyzer
3. **Analysis**: LLM extracts metadata, embeddings generated
4. **Storage**: Tools persisted to SQLite with embeddings
5. **Compilation**: JITToolCompiler called during shard spawn
6. **Selection**: Vector search + Mangle logic determines tools
7. **Rendering**: ToolRenderer produces LLM-consumable output

## Testing

```bash
go test ./internal/mcp/...
```

---

**Last Updated:** December 2024
