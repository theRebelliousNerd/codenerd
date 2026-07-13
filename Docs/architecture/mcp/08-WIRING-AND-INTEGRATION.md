# 08 — Wiring and Integration: MCP

> Last verified against codebase: 2026-07-13  
> Evidence-based; no invented routes

## 1. Boot path (live)

**Site:** `internal/system/factory.go` (embedding init region, ~lines 731–751)

```text
integrationsCfg := bctx.appCfg.GetIntegrations()
serverConfigs := integrationsCfg.ToMCPServerConfigs()
if len(serverConfigs) > 0:
  mcpLLMClient = perceptionLLMAdapter{bctx.llmClient} if present
  mcpBridge = NewMCPIntegrationBridge(
      workspace,
      newMCPKernelAdapter(kernel),
      embeddingEngine,
      mcpLLMClient,
      serverConfigs)
  for serverID := range serverConfigs:
      virtualStore.SetMCPClient(serverID, mcpBridge.GetAdapter(serverID))
  go mcpBridge.ConnectAll(context.Background())
```

### Properties of this wire

| Property | Observation |
|----------|-------------|
| Conditional | Only when at least one enabled server in config |
| Adapter registration | Synchronous before connect |
| Connect | Asynchronous; failures logged Warn |
| Bridge retention | **Local variable only** — not assigned to bctx field in this block |
| Kernel adapter | `newMCPKernelAdapter` in `factory_adapters.go` |
| LLM adapter | `perceptionLLMAdapter` maps perception client → `mcp.LLMClient` |

## 2. Config wire (live)

**Site:** `internal/config/integrations.go`

- `IntegrationsConfig.Servers map[string]MCPServerIntegration`
- `ToMCPServerConfigs()` skips disabled; defaults protocol `http` and timeout via `DefaultTimeout` (scraper 120s, browser 60s, else 30s)
- Produces `map[string]mcp.MCPServerConfig`

Config shape (conceptual JSON):

```json
{
  "integrations": {
    "servers": {
      "code_graph": {
        "enabled": true,
        "protocol": "http",
        "base_url": "http://localhost:8080",
        "auto_connect": true,
        "auto_discover_tools": true
      }
    }
  }
}
```

(Exact top-level key nesting follows app config schema; conversion lives in `ToMCPServerConfigs`.)

## 3. VirtualStore wire (live)

| API | File | Role |
|-----|------|------|
| `SetMCPClient(serverID, IntegrationClient)` | `virtual_store.go` | Register |
| `GetMCPClient(serverID)` | `virtual_store.go` | Returns **proxied** client |
| `GetMCPClientNames()` | `virtual_store.go` | Inspection |
| `mcpClientProxy` | `virtual_store_mcp_proxy.go` | Sanitize args/results, recover panics |

### Known call sites by server ID

| Server ID | File | Tool / use |
|-----------|------|------------|
| `code_graph` | `virtual_store_actions.go` | `impact-analysis` |
| `scraper` | `virtual_store_workflows.go` | workflow scraping path |

These are **hardcoded consumer IDs**, not package-level enums — they work only if config enables matching server keys and boot registered adapters.

## 4. Kernel schema wire (live Decl / partial rules)

| Artifact | Path | Loaded? |
|----------|------|---------|
| Schemas | `internal/core/defaults/schemas_mcp.mg` | **Yes** — `kernel_init.go` includes `"schemas_mcp.mg"` |
| Policy rules | `internal/mcp/policy_mcp.mg` | **No evidence** in kernel policy loader |
| Related tool routing | `internal/core/defaults/policy/tool_routing.mg` | Separate section 40 tool affinity (non-MCP-specific) |
| Intelligence | `policy/intelligence.mg` | References `intelligence_mcp_tool` Decl elsewhere |

## 5. Compiler ↔ kernel wire (runtime, partial)

On each `JITToolCompiler.Compile`:

1. Assert temporary `mcp_tool_vector_score("id", N)` for vector hits  
2. Query `mcp_tool_selected("shard", ToolID, RenderMode)`  
3. Retract vector scores  

Without EDB registration facts + loaded policy IDB, step 2 yields empty → **fallbackSelect**.

## 6. Campaign wire (optional consumer)

| Type | Field | File |
|------|-------|------|
| `ToolPregenerator` | `mcpStore *mcp.MCPToolStore` | `tool_pregenerator.go` |
| `IntelligenceGatherer` | `mcpStore *mcp.MCPToolStore` | `intelligence_gatherer.go` |

These expect a store instance injected by campaign setup; they do not construct the full bridge themselves.

## 7. CLI / systems inspection

`cmd/nerd/cmd_systems.go` queries kernel for `mcp_server_registered` and `mcp_tool_capability` — useful only if those facts were asserted into the kernel (currently not automatically by discover).

`cmd/nerd/cmd_mangle_check.go` references path `internal/mcp/schemas_mcp.mg` — **path drift** vs actual defaults location.

## 8. E2E wire

`tests/e2e/mcp_virtualstore_integration_test.go` registers mock `IntegrationClient`s via `SetMCPClient` and exercises proxy contracts (panic, races, sanitization, replacement). Validates VirtualStore edge, not full HTTP MCP servers.

## 9. Wiring journal — honest status

| Wire | Status |
|------|--------|
| Config → server configs | **Live** |
| Factory → bridge construct | **Live** (when configs non-empty) |
| Bridge → VS adapters | **Live** |
| VS → named tool calls | **Live** for specific workflows |
| Auto connect + discover | **Live** async |
| Discover → SQLite | **Live** |
| Discover → kernel EDB | **Missing** |
| Policy_mcp → kernel program | **Missing** |
| Bridge → retained compile API | **Missing / local only** |
| CompileToolsForShard → prompt assembly | **Not found as standard path** |
| Campaign store injection | **API present; depends on campaign setup** |

## 10. How to prove a wire before claiming it

```powershell
rg "NewMCPIntegrationBridge|SetMCPClient|CompileToolsForShard|policy_mcp" -g "*.go" -g "*.mg"
rg "mcp_tool_registered|mcp_tool_selected" -g "*.go"
go test ./internal/mcp/...
go test ./tests/e2e/ -run MCP -count=1
```
