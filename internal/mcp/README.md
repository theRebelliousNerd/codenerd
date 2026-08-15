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
internal/mcp/
├── types.go             # Core type definitions, ToolSchemaHash
├── client.go            # MCPClientManager: connect / discover / call
├── transport_http.go    # HTTP JSON-RPC transport
├── transport_stdio.go   # Subprocess JSON-RPC transport
├── transport_sse.go     # Server-sent-events transport
├── store.go             # SQLite storage with embeddings (sqlite-vec optional)
├── analyzer.go          # LLM + heuristic tool analysis
├── facts.go             # FactEmitter: MCP state -> kernel EDB
├── compiler.go          # JITToolCompiler: vector + Mangle selection
├── renderer.go          # Tool set rendering for LLM context
├── integration.go       # MCPIntegrationBridge + IntegrationAdapter
└── testdata/            # Golden fixtures for the selection policy
```

The Mangle files are **not** in this package. The kernel only loads what is
embedded under `internal/core/defaults/`:

| File | Role |
|------|------|
| `internal/core/defaults/schemas_mcp.mg` | Decls (EDB + IDB names), loaded by `kernel_init.go` |
| `internal/core/defaults/policy/policy_mcp.mg` | Section 50 selection rules, loaded by the `defaults/policy/*.mg` sweep |

A package-local `policy_mcp.mg` used to live here; nothing loaded it, so
`mcp_tool_selected` was always empty and selection silently fell back to the Go
heuristic in `compiler.go`.

## Key Concepts

### Skeleton/Flesh Bifurcation

Mirrors the JIT Prompt Compiler pattern:

| Type | Selection | Description |
|------|-----------|-------------|
| **Skeleton** | `mcp_tool_skeleton/1` | Mandatory: filesystem+read, search+search. Always full render, for every shard. |
| **Flesh** | Hybrid scoring | Context-dependent, (Logic × 0.7) + (Vector × 0.3) |

`ToolCompilationStats.SkeletonTools` / `.FleshTools` count these policy classes,
not render tiers.

### Fact emission

`FactEmitter` (`facts.go`) mirrors runtime MCP state into the kernel so policy
has something to reason over:

| Event | Facts |
|-------|-------|
| Connect | `mcp_server_registered`, `mcp_server_name`, `mcp_server_capabilities`, `mcp_server_status(/connected)` |
| Status change / disconnect | `mcp_server_status` replaced in place (cached tools stay available) |
| Discover / analyze | `mcp_tool_registered`, `_name`, `_description`, `_condensed`, `_capability`, `_category`, `_domain`, `_shard_affinity`, `_analyzed` |
| Tool call | `mcp_tool_usage`, `mcp_tool_last_used`, `mcp_tool_avg_latency` |
| ConnectAll finished | `mcp_integration_ready(ServerCount, ToolCount)` |

Emission is subject-keyed and replaces rather than appends: re-analysis of a
tool retracts its previous facts first. The kernel adapter retracts by *exact*
fact, so the emitter remembers the exact strings it asserted.

### Analysis cache invalidation

`ToolSchemaHash` fingerprints the server-advertised name, description, and input
/output schemas. Rediscovery with a different hash invalidates the cached LLM
analysis and re-runs it; an unchanged hash reuses it. Rows written before the
hash column existed are backfilled on first sight rather than re-analyzed.

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

1. **Startup**: `MCPIntegrationBridge.ConnectAll` connects configured servers
2. **Discovery**: `ListTools()` called, new tools sent to ToolAnalyzer
3. **Analysis**: LLM extracts metadata, embeddings generated (heuristics if no LLM)
4. **Storage**: Tools persisted to SQLite with embeddings; facts emitted to the kernel
5. **Readiness**: `ConnectAll` returns only after initial discovery; `Ready()` /
   `WaitReady(ctx)` expose the same signal, `mcp_integration_ready/2` exposes it
   to Mangle
6. **Compilation**: JITToolCompiler called during shard spawn
7. **Selection**: vector scores asserted, `mcp_tool_selected/3` queried; Go
   fallback only when the kernel derives nothing (logged at Info/Warn, and
   reported as `ToolCompilationStats.SelectionPath`)
8. **Rendering**: ToolRenderer produces LLM-consumable output

## Operational notes

### stdio sandbox expectations

A stdio server's `endpoint` is split on whitespace and executed with
`exec.Command`, inheriting this process's environment, working directory, and
privileges. There is **no sandbox**. Treat `integrations.*.endpoint` as
equivalent to a shell entry in the config: only configure commands you would run
yourself. Prefer `http`/`sse` for anything not vendored with the workspace, and
run untrusted servers inside an external sandbox (container, jail) whose entry
command is what you configure here.

### Secret redaction

Tool arguments and outputs may carry credentials. `RedactSecrets` scrubs common
token shapes (`Authorization` headers, `api_key`/`token`/`password`/`secret`
fields, bearer tokens, AWS keys, PEM blocks, long high-entropy hex strings)
before anything reaches a log line. Call sites that log call results must route
through it — never log a raw `MCPCallResult.Output`.

## Testing

```bash
go test ./internal/mcp/...
go test -race ./internal/mcp/ -run 'Concurrent|Race'   # manager + store races
```

`policy_golden_test.go` runs the selection policy against
`testdata/mcp_selection.edb` and compares to a golden file;
`kernel_integration_test.go` boots a real kernel and proves the embedded policy
is loaded and driven by emitted facts.

---

**Last Updated:** August 2026
