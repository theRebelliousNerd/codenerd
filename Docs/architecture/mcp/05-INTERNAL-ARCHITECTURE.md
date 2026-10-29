# 05 — Internal Architecture: MCP

> Last verified against codebase: 2026-07-13  
> Primary sources: `integration.go`, `client.go`, `compiler.go`, `store.go`, transports

## 1. Component diagram

```
                    ┌─────────────────────────────┐
                    │   MCPIntegrationBridge      │
                    │  manager store compiler     │
                    │  renderer adapters{}        │
                    └──────────┬──────────────────┘
           ┌───────────────────┼───────────────────┐
           ▼                   ▼                   ▼
   MCPClientManager     JITToolCompiler      ToolRenderer
           │                   │
     ┌─────┴─────┐             ├─ MCPToolStore
     │ per server│             ├─ EmbeddingEngine
     │ connection│             └─ KernelInterface
     ▼           ▼
 MCPTransport   ToolAnalyzer
 (http|stdio|sse)   (LLMClient + embedder)
```

## 2. Data flow — discovery

```
Connect(serverID)
  → new transport from MCPServerConfig.Protocol
  → transport.Connect
  → GetCapabilities → MCPServer record → store.SaveServer
  → if AutoDiscoverTools: go DiscoverTools
        → ListTools
        → for each schema: processToolSchema
              → store.GetTool (skip re-analyze if AnalyzedAt set)
              → analyzer.Analyze → metadata + embedding
              → store.SaveTool (+ vec index)
              → onToolDiscovered callback
```

## 3. Data flow — JIT compile

```
Compile(ToolCompilationContext)
  → store.GetAllTools
  → vectorSearch(task) → map[toolID]score
  → kernel.Assert mcp_tool_vector_score for each
  → selectTools:
        try mangleSelect: Query mcp_tool_selected(ShardType, ToolID, RenderMode)
        else fallbackSelect: affinity*0.7 + vector*0.3 → thresholds
  → buildToolSet (full / condensed / minimal buckets)
  → fitBudget demotions
  → kernel.Retract temporary vector scores
  → return CompiledToolSet + Stats
```

## 4. Data flow — invoke

```
IntegrationAdapter.CallTool(ctx, toolName, args)
  → toolID = serverID + "/" + toolName
  → MCPClientManager.CallTool
        → clone args + json.Marshal check
        → parseToolID; reject traversal in name
        → transport.CallTool
        → truncate output > 500KiB
        → go store.RecordToolUsage
  → return Output or error

VirtualStore.GetMCPClient(id) wraps client in mcpClientProxy
  → sanitizeArgs (primitives only)
  → CallTool
  → sanitizeResult (null-byte strip, stringify unknowns)
```

## 5. State machines

### Server connection (`ServerStatus`)

```
unknown → connecting → connected
                    ↘ error
connected → disconnected (explicit Disconnect)
```

Statuses: `unknown`, `connecting`, `connected`, `disconnected`, `error` (`types.go`).

### Render mode

```
score ≥ FullThreshold     → full
score ≥ Condensed         → condensed
score ≥ Minimal           → minimal
else                      → excluded (not selected)
```

Defaults: 70 / 40 / 20 (`DefaultToolSelectionConfig`).

### Store vector mode

```
embedder nil            → no vec table; brute path if embeddings exist
vec0 create fails       → vectorExt=false; brute force cosine
vec0 ok                 → mcp_tool_vec virtual table
```

## 6. Key types (structural)

| Type | File | Responsibility |
|------|------|----------------|
| `MCPServerConfig` | types | Config from integrations map |
| `MCPTool` | types | Full tool record + stats + embedding |
| `MCPTransport` | types | Protocol interface |
| `MCPClientManager` | client | Multi-server orchestration |
| `MCPToolStore` | store | SQLite persistence |
| `ToolAnalyzer` | analyzer | Metadata extraction |
| `JITToolCompiler` | compiler | Selection pipeline |
| `ToolRenderer` | renderer | LLM-facing formatting |
| `MCPIntegrationBridge` | integration | Lifecycle façade |
| `IntegrationAdapter` | integration | Per-server CallTool |
| `CompiledToolSet` | types | Compile output |
| `ToolSelectionConfig` | types | Thresholds/weights |

## 7. Concurrency model

| Site | Mechanism |
|------|-----------|
| Manager maps | `sync.RWMutex` |
| Store DB ops | `sync.RWMutex` around SQL |
| Bridge adapters | `sync.RWMutex` |
| Transports | per-transport mutex + stdio wait groups |
| Discover after connect | detached goroutine + panic recover |
| Usage / status updates | detached goroutine + recover |

CallTool clones the args map before transport use to reduce race exposure.

## 8. Persistence schema (SQLite)

**`mcp_servers`:** server_id PK, endpoint, protocol, name, version, status, capabilities JSON, timestamps, config.

**`mcp_tools`:** tool_id PK, server_id, name, schemas, categories/capabilities/domain/affinities/use_cases JSON, condensed, embedding BLOB, usage counters, timestamps; UNIQUE(server_id, name).

**`mcp_tool_vec`:** optional vec0 virtual table keyed by tool_id.

## 9. Mangle touchpoints (runtime)

| Direction | Predicate / action |
|-----------|-------------------|
| Go → kernel | `Assert mcp_tool_vector_score(ToolID, Score)` |
| Go → kernel | `Retract mcp_tool_vector_score(ToolID, _)` after compile |
| Go ← kernel | `Query mcp_tool_selected(ShardType, ToolID, RenderMode)` |
| Schema Decl | `internal/core/defaults/schemas_mcp.mg` |
| Policy IDB | `internal/mcp/policy_mcp.mg` (intended; load status see gap analysis) |

## 10. Relationship to prompt JIT

| Prompt JIT | Tool JIT |
|------------|----------|
| Atom loader + compiler | Tool store + JITToolCompiler |
| Skeleton / flesh atoms | Skeleton / flesh tools |
| Token budget demotion | fitBudget demotion |
| Render modes for context | Full / Condensed / Minimal |
| Kernel selection queries | `mcp_tool_selected` |

They are **sibling patterns**, not the same package.
