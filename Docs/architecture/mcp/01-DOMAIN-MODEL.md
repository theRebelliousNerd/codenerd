# mcp — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/mcp/` (complete internal coverage)
> **Implementation: `internal/mcp/` — 10 non-test .go, 16 tests, 1 .mg**


## Package

`internal/mcp/`

## Exported types (sampled, up to 40)

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

## Exported functions/methods (sampled, up to 30)

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

## Mangle surface

| Artifact | Count |
|----------|------:|
| Package-local `.mg` | 1 |

| Path | Lines |
|------|------:|
| `internal/mcp/policy_mcp.mg` | 177 |

Global kernel schemas/policy (when this package participates):

- `internal/core/defaults/schemas.mg`
- `internal/core/defaults/policy/`

## Fact-flow placement

```
user input → perception → user_intent → kernel(core/mangle) → next_action
  → VirtualStore / shards / tools → articulation
```

This package: **MCP server/client integration surfaces**
