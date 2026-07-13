# 06 — Public API and Types: MCP

> Last verified against codebase: 2026-07-13  
> Package: `codenerd/internal/mcp`

## 1. Construction entry points

| Func | File | Role |
|------|------|------|
| `NewMCPIntegrationBridge(workspace, kernel, embedder, llm, configs)` | `integration.go` | Preferred system entry; owns store path `.nerd/mcp_tools.db` |
| `NewMCPClientManager(store, analyzer, config)` | `client.go` | Multi-server manager |
| `NewMCPToolStore(dbPath, embedder)` | `store.go` | SQLite store |
| `NewToolAnalyzer(llm, embedder)` | `analyzer.go` | Metadata analysis |
| `NewJITToolCompiler(store, embedder, kernel)` | `compiler.go` | Selection pipeline |
| `NewToolRenderer()` | `renderer.go` | LLM formatting |
| `NewHTTPTransport(baseURL, timeout)` | `transport_http.go` | HTTP |
| `NewStdioTransport(endpoint)` | `transport_stdio.go` | Command line split on whitespace |
| `NewSSETransport(baseURL, timeout)` | `transport_sse.go` | SSE |
| `NewIntegrationAdapter(manager, serverID)` | `integration.go` | Per-server VS adapter |
| `DefaultToolSelectionConfig()` | `types.go` | Thresholds |

## 2. Interfaces

### `MCPTransport` (`types.go`)

```text
Connect(ctx) error
Disconnect() error
ListTools(ctx) ([]MCPToolSchema, error)
CallTool(ctx, name, args) (*MCPCallResult, error)
GetCapabilities(ctx) (*MCPCapabilities, error)
Ping(ctx) error
IsConnected() bool
```

### `LLMClient` (`analyzer.go`)

```text
Complete(ctx, prompt string) (string, error)
```

### `ToolAnalyzerInterface` (`client.go`)

```text
Analyze(ctx, schema MCPToolSchema) (*ToolAnalysis, error)
```

### `KernelInterface` (`compiler.go`)

```text
Assert(fact string) error
Retract(fact string) error
Query(query string) ([]map[string]any, error)
```

### `IntegrationClient` (`integration.go`)

```text
CallTool(ctx, tool string, args map[string]any) (any, error)
```

Mirrored by `core.IntegrationClient` for VirtualStore.

## 3. Domain types (selection)

| Type | Purpose |
|------|---------|
| `ServerStatus` | Connection lifecycle strings |
| `Protocol` | `http` / `stdio` / `sse` |
| `RenderMode` | `full` / `condensed` / `minimal` / `excluded` |
| `MCPServerConfig` | Per-server config from integrations |
| `MCPServer` | Runtime/persisted server record |
| `MCPTool` | Full tool with metadata, embedding, usage |
| `MCPToolSchema` | Wire schema from tools/list |
| `MCPCapabilities` | tools/resources/prompts/logging flags |
| `ToolAnalysis` | Analyzer output |
| `ToolCompilationContext` | Shard/task/intent/budget inputs |
| `CompiledToolSet` | Full + condensed + minimal + stats |
| `ToolSummary` | Condensed entry |
| `ToolCompilationStats` | Timing and counts |
| `ToolSelectionConfig` | Thresholds and weights |
| `SelectedTool` | Intermediate selection record |
| `MCPCallResult` | Success, output, error, latency |
| `ToolAvailableEntry` | Hybrid available_tools.json entry (`Type=="mcp"`) |
| `ToolSearchResult` | Semantic search hit |
| `ToolJSONEntry` | Renderer JSON projection |

## 4. Manager API (behavioral)

| Method | Behavior |
|--------|----------|
| `ConnectAll` | Connect enabled+auto_connect; last error returned |
| `Connect` | Create transport, connect, save server, optional async discover |
| `Disconnect` / `DisconnectAll` | Tear down transport(s) |
| `DiscoverTools` | List + process schemas |
| `CallTool` | Route by toolID, safety checks, usage stats |
| `GetServer` / `GetConnectedServers` / `GetAllTools` / `ListTools` | Inspection |
| `SetOnToolDiscovered` / `SetOnServerStatus` | Callbacks |
| `SetToolSelectionConfig` | Manager-held selection config (compiler has own SetConfig) |

## 5. Store API (behavioral)

| Method | Behavior |
|--------|----------|
| `SaveServer` / `GetServer` / `GetAllServers` / `UpdateServerStatus` | Server CRUD |
| `SaveTool` / `GetTool` / `GetAllTools` / `GetToolsByServer` | Tool CRUD |
| `RecordToolUsage` | Counters + avg latency |
| `SemanticSearch` | vec0 or brute cosine |
| `Close` | Close DB |

## 6. Compiler API

| Method | Behavior |
|--------|----------|
| `Compile(ctx, ToolCompilationContext)` | Full pipeline |
| `CompileForShard(ctx, shardType, task)` | Convenience budget default |
| `SetConfig(ToolSelectionConfig)` | Thresholds |

## 7. Bridge API

| Method | Behavior |
|--------|----------|
| `GetManager/Store/Compiler/Renderer` | Accessors |
| `GetAdapter(serverID)` | Lazy per-server adapter |
| `ConnectServer` / `ConnectAll` / `Close` | Lifecycle |
| `CompileToolsForShard` | Compile + Render markdown string |
| `DiscoverAndAnalyzeTools` | Proxy to manager |

## 8. Renderer API

| Method | Output |
|--------|--------|
| `Render` | Markdown sections Primary / Secondary / Additional |
| `RenderCompact` | Single-line summary |
| `RenderJSON` | Structured JSON |
| `RenderForInvocation` | Call-oriented minimal text |
| `SetIncludeSchemas` / `SetMaxSchemaLen` | Full-tool schema controls |

## 9. Notable helpers

| Symbol | Role |
|--------|------|
| `ToolAvailableEntry.IsMCPTool` | `Type == "mcp"` |
| `parseToolID` | Last-slash split (unexported; tested via export_test / coverage) |
| Default shard affinities | coder/tester/reviewer/researcher maps in analyzer |

## 10. Import contract for callers

Preferred external callers today:

- `internal/system` — constructs bridge, registers adapters  
- `internal/config` — converts YAML/JSON to `MCPServerConfig`  
- `internal/campaign` — holds `*MCPToolStore` for affinity/gap logic  
- `internal/core` — does **not** import mcp; uses its own `IntegrationClient` interface
