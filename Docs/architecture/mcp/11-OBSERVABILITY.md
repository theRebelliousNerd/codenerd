# 11 — Observability: MCP

> Last verified against codebase: 2026-07-13

## 1. Logging category

Primary category: **`logging.CategoryTools`**

Used across client, transports, analyzer, store, compiler, integration adapter.

| Level | Typical events |
|-------|----------------|
| **Info** | Connected to server; discovered N tools; JIT compile summary; vector index init |
| **Warn** | Connect/discover/analyze/persist failures; tool call failure; usage/status persist failure; embedding health adjacent at boot |
| **Debug** | Cached tool reuse; vector search fail; mangle selection fail → fallback; vec index update fail |
| **Error** | Panic recovered in background goroutines (discover, usage, status) |

Factory boot also logs Info `"Wired MCP integration: %s"` and Warn on bridge/connect failures (`system/factory.go`).

VirtualStore uses `logging.VirtualStoreDebug` for MCP client attach.

## 2. Compilation stats

`ToolCompilationStats` fields (returned on `CompiledToolSet` and logged):

| Field | Meaning |
|-------|---------|
| `Duration` | End-to-end compile time |
| `TotalTools` / `SelectedTools` | Catalog vs selected |
| `SkeletonTools` / `FleshTools` | Counters (mapped from full vs condensed/minimal in buildToolSet) |
| `VectorQueryMs` / `MangleQueryMs` | Phase timings |
| `TokensUsed` / `TokenBudget` | Budget fit |
| `CacheHit` | Field present (populate path limited) |

Info log format (compiler):

```text
JIT Tool Compiler: %dms | tools=%d (full=%d, condensed=%d, minimal=%d) | vec=%dms | budget=%d/%d
```

## 3. Persistent telemetry

Store counters on each tool:

- `usage_count`, `success_count`, `avg_latency_ms`, `last_used`

Updated asynchronously after CallTool. Failures are **Warn** (not silent) because they skew future affinity models.

Server status transitions persisted via `UpdateServerStatus`.

## 4. Callbacks for external observers

Manager:

- `SetOnToolDiscovered(func(*MCPTool))`  
- `SetOnServerStatus(func(serverID, ServerStatus))`

Useful for UI / systems panels if wired by boot (not required for core function).

## 5. CLI inspection hooks

`cmd_systems` kernel queries:

- `mcp_server_registered`  
- `mcp_tool_capability`  

These only show data if facts exist in the kernel (see Mangle surface gaps).

## 6. Missing observability

| Missing | Impact |
|---------|--------|
| Structured metrics (Prometheus/OTel) | No time-series of call latency/error rate |
| Correlation IDs per tool call | Hard to join logs across OODA turns |
| Explicit “selection path=mangle|fallback” Info flag | Operators cannot see executive path at a glance (only Debug on mangle fail) |
| Bridge lifecycle metrics (connected count) | Only GetConnectedServers API |
| Redaction of tool args in logs | Risk if Debug ever logs full payloads |

## 7. Operator playbook (minimal)

1. Enable tools-category logging at Info.  
2. Confirm boot lines: `Wired MCP integration: <id>`.  
3. On failure: `MCP auto-connect failed` / `Failed to discover tools`.  
4. On selection: look for `JIT Tool Compiler:` summary after compile path is exercised.  
5. Inspect `.nerd/mcp_tools.db` for catalog if kernel facts empty.
