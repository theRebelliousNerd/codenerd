# mcp — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/mcp/` (complete internal coverage)
> **Implementation: `internal/mcp/` — 10 non-test .go, 16 tests, 1 .mg**


## 1. Purpose

MCP server/client integration surfaces

## 2. Source paths (1:1)

| Path | Role |
|------|------|
| `internal/mcp/` | Primary implementation |
| `Docs/architecture/mcp/` | This full corpus |

## 3. Implementation Status

> Living code status — **not** pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
| Source package tree | Implemented | **90%** |
| Exported types (sampled) | Implemented | **80%** |
| Tests | Implemented | **90%** |
| Mangle local sources | Implemented | **85%** |
| Full architecture corpus | Implemented | **100%** |

**Overall (heuristic): 90%** as living package (10 src / 16 tests)

## 4. Public surface inventory

### Largest files

| Path | Lines |
|------|------:|
| `internal/mcp/store.go` | 683 | source |
| `internal/mcp/client.go` | 545 | source |
| `internal/mcp/analyzer.go` | 523 | source |
| `internal/mcp/transport_stdio.go` | 478 | source |
| `internal/mcp/transport_sse.go` | 439 | source |
| `internal/mcp/compiler.go` | 363 | source |
| `internal/mcp/transport_http.go` | 291 | source |
| `internal/mcp/types.go` | 267 | source |
| `internal/mcp/renderer.go` | 227 | source |
| `internal/mcp/integration.go` | 183 | source |

### Types (sampled)

| Type | Location |
|------|----------|
| `LLMClient` | `internal/mcp/analyzer.go:15` |
| `ToolAnalyzer` | `internal/mcp/analyzer.go:20` |
| `MCPClientManager` | `internal/mcp/client.go:16` |
| `MCPServerConnection` | `internal/mcp/client.go:31` |
| `ToolAnalyzerInterface` | `internal/mcp/client.go:38` |
| `KernelInterface` | `internal/mcp/compiler.go:15` |
| `JITToolCompiler` | `internal/mcp/compiler.go:23` |
| `IntegrationClient` | `internal/mcp/integration.go:15` |
| `IntegrationAdapter` | `internal/mcp/integration.go:21` |
| `MCPIntegrationBridge` | `internal/mcp/integration.go:65` |
| `ToolRenderer` | `internal/mcp/renderer.go:10` |
| `ToolJSONEntry` | `internal/mcp/renderer.go:193` |
| `MCPToolStore` | `internal/mcp/store.go:22` |
| `ToolSearchResult` | `internal/mcp/store.go:557` |
| `HTTPTransport` | `internal/mcp/transport_http.go:17` |
| `SSETransport` | `internal/mcp/transport_sse.go:20` |
| `StdioTransport` | `internal/mcp/transport_stdio.go:18` |
| `ServerStatus` | `internal/mcp/types.go:12` |
| `Protocol` | `internal/mcp/types.go:23` |
| `RenderMode` | `internal/mcp/types.go:32` |
| `MCPServerConfig` | `internal/mcp/types.go:42` |
| `MCPServer` | `internal/mcp/types.go:54` |
| `MCPTool` | `internal/mcp/types.go:70` |
| `MCPToolSchema` | `internal/mcp/types.go:108` |
| `MCPCapabilities` | `internal/mcp/types.go:116` |
| `ToolAnalysis` | `internal/mcp/types.go:124` |
| `ToolCompilationContext` | `internal/mcp/types.go:136` |
| `CompiledToolSet` | `internal/mcp/types.go:146` |
| `ToolSummary` | `internal/mcp/types.go:155` |
| `ToolCompilationStats` | `internal/mcp/types.go:162` |
| `ToolSelectionConfig` | `internal/mcp/types.go:176` |
| `SelectedTool` | `internal/mcp/types.go:204` |
| `MCPCallResult` | `internal/mcp/types.go:213` |
| `MCPTransport` | `internal/mcp/types.go:221` |
| `ToolAvailableEntry` | `internal/mcp/types.go:245` |

### Functions (sampled)

| Symbol | Location |
|--------|----------|
| `NewToolAnalyzer` | `internal/mcp/analyzer.go:26` |
| `Analyze` | `internal/mcp/analyzer.go:34` |
| `NewMCPClientManager` | `internal/mcp/client.go:43` |
| `SetToolSelectionConfig` | `internal/mcp/client.go:54` |
| `SetOnToolDiscovered` | `internal/mcp/client.go:61` |
| `SetOnServerStatus` | `internal/mcp/client.go:68` |
| `ConnectAll` | `internal/mcp/client.go:75` |
| `Connect` | `internal/mcp/client.go:96` |
| `Disconnect` | `internal/mcp/client.go:216` |
| `DisconnectAll` | `internal/mcp/client.go:240` |
| `DiscoverTools` | `internal/mcp/client.go:256` |
| `CallTool` | `internal/mcp/client.go:362` |
| `GetServer` | `internal/mcp/client.go:438` |
| `GetConnectedServers` | `internal/mcp/client.go:446` |
| `GetAllTools` | `internal/mcp/client.go:460` |
| `ListTools` | `internal/mcp/client.go:472` |
| `NewJITToolCompiler` | `internal/mcp/compiler.go:31` |
| `SetConfig` | `internal/mcp/compiler.go:41` |
| `Compile` | `internal/mcp/compiler.go:46` |
| `CompileForShard` | `internal/mcp/compiler.go:357` |
| `NewIntegrationAdapter` | `internal/mcp/integration.go:27` |
| `CallTool` | `internal/mcp/integration.go:36` |
| `NewMCPIntegrationBridge` | `internal/mcp/integration.go:76` |
| `GetManager` | `internal/mcp/integration.go:108` |
| `GetStore` | `internal/mcp/integration.go:113` |
| `GetCompiler` | `internal/mcp/integration.go:118` |
| `GetRenderer` | `internal/mcp/integration.go:123` |
| `GetAdapter` | `internal/mcp/integration.go:129` |
| `ConnectServer` | `internal/mcp/integration.go:143` |
| `ConnectAll` | `internal/mcp/integration.go:148` |

## 5. Integration relevance

| Surface | Relevance |
|---------|-----------|
| Kernel | Related |
| VirtualStore | Consumer if effectful |
| Shards | Related |
| Prompt JIT | Optional |
| CLI | Related via `cmd/nerd` |
| Config | Reader |

## 6. Non-goals of this corpus revision

- Full prose rewrite of every function body
- Docs/Spec 18-file product templates (`spec-doc-sprint`)
- Implementing missing features (corpus-build implementation mode)
