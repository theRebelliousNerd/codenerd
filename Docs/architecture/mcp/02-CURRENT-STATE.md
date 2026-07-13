# 02 — Current State: MCP (`internal/mcp`)

> Last verified against codebase: 2026-07-13  
> Status: Living inventory (code-grounded)

## 1. Package identity

| Property | Value |
|----------|-------|
| Import path | `codenerd/internal/mcp` |
| Package comment | MCP client + JIT-style tool compilation (`types.go`) |
| Local README | `internal/mcp/README.md` (architecture 2.0.0 JIT-driven) |
| Workspace DB default | `{workspace}/.nerd/mcp_tools.db` (`integration.go`) |

## 2. File inventory

### 2.1 Non-test sources

| Path | ~Lines | Role |
|------|------:|------|
| `internal/mcp/store.go` | 683 | SQLite tool/server store, embeddings, semantic search |
| `internal/mcp/client.go` | 545 | Multi-server manager: connect, discover, call |
| `internal/mcp/analyzer.go` | 523 | LLM + heuristic tool metadata + embeddings |
| `internal/mcp/transport_stdio.go` | 478 | Subprocess JSON-RPC over stdio |
| `internal/mcp/transport_sse.go` | 439 | SSE + POST JSON-RPC transport |
| `internal/mcp/compiler.go` | 363 | JIT selection pipeline + budget fit |
| `internal/mcp/transport_http.go` | 291 | HTTP JSON-RPC transport |
| `internal/mcp/types.go` | 267 | Domain types, config defaults, MCPTransport iface |
| `internal/mcp/renderer.go` | 227 | Markdown / compact / JSON / invocation render |
| `internal/mcp/integration.go` | 183 | Bridge + IntegrationAdapter for VirtualStore |
| `internal/mcp/export_test.go` | (test helpers export) | White-box test hooks |
| `internal/mcp/policy_mcp.mg` | 177 | Section 50 selection rules (package-local) |
| `internal/mcp/README.md` | ~114 | Package overview |

**Note:** Package README still lists `schemas_mcp.mg` under `internal/mcp/`. On disk, the **boot-loaded** schema module is `internal/core/defaults/schemas_mcp.mg`. There is no `internal/mcp/schemas_mcp.mg` in the tree; `cmd/nerd/cmd_mangle_check.go` still references that missing path in one place.

### 2.2 Tests (16 files)

| Path | Focus |
|------|--------|
| `analyzer_test.go` / `analyzer_coverage_test.go` | JSON extract, normalize, analyze paths |
| `client_boundary_test.go` / `client_coverage_test.go` | Connect edges, call tool, discover, parseToolID |
| `compiler_test.go` / `compiler_select_test.go` | Fallback select, budget, buildToolSet, mangle case |
| `store_test.go` / `store_coverage_test.go` / `store_query_test.go` | Lifecycle, vectors helpers, queries |
| `transport_http_test.go` | HTTP transport lifecycle |
| `renderer_coverage_test.go` | Render tiers + JSON |
| `types_coverage_test.go` | Enums + defaults |
| `integration_coverage_test.go` / `integration_bridge_test.go` | Adapter + bridge constructors |
| `mcp_client_integration_test.go` | Heavier client suite |

### 2.3 Related sources **outside** package (wiring)

| Path | Role |
|------|------|
| `internal/core/defaults/schemas_mcp.mg` | Decl surface loaded by kernel_init |
| `internal/system/factory.go` | Boot: bridge + SetMCPClient + ConnectAll |
| `internal/system/factory_adapters.go` | `mcpKernelAdapter`, `perceptionLLMAdapter` |
| `internal/config/integrations.go` | `ToMCPServerConfigs()` |
| `internal/core/virtual_store.go` | mcpClients map, Set/GetMCPClient |
| `internal/core/virtual_store_mcp_proxy.go` | Arg/result sanitization proxy |
| `internal/core/virtual_store_actions.go` | e.g. `code_graph` impact-analysis call |
| `internal/core/virtual_store_workflows.go` | e.g. `scraper` client usage |
| `internal/campaign/tool_pregenerator.go` | Optional MCPToolStore fallback |
| `internal/campaign/intelligence_gatherer.go` | Affinity over MCP tools |
| `tests/e2e/mcp_virtualstore_integration_test.go` | Proxy / contract e2e |

## 3. Component status (honest)

| Component | Status | Notes |
|-----------|--------|-------|
| Types + defaults | **Implemented** | Full domain model in `types.go` |
| HTTP transport | **Implemented** | Connect via capabilities, tools/list, tools/call |
| Stdio transport | **Implemented** | Subprocess + reader loop + pending requests |
| SSE transport | **Implemented** | Endpoint event + dual channel |
| Client manager | **Implemented** | Multi-server, discover, call, callbacks |
| Tool analyzer | **Implemented** | LLM + heuristic + embed |
| Tool store | **Implemented** | SQLite + optional vec0 |
| JIT compiler | **Implemented** | Vector + mangle query + fallback + budget |
| Renderer | **Implemented** | Multiple formats |
| Integration bridge | **Implemented** | High-level façade |
| Boot wiring adapters | **Implemented** | When integrations config non-empty |
| Assert tool EDB on discover | **Partial / missing** | No store→kernel fact emitter found |
| Load `policy_mcp.mg` | **Not wired** | Rules file present; kernel loads schemas only |
| Retain bridge for compile | **Partial** | Adapters registered; bridge local to factory block |
| MCP server role | **Out of scope** | Client only |

**Overall:** living package ~**85–90%** of designed client surface; **Mangle executive path incomplete**.

## 4. Hotspots

1. **`store.go`** — largest file; schema + vector dual path; lock discipline.
2. **`client.go`** — connection lifecycle, background discover, usage stats goroutines.
3. **`compiler.go`** — selection semantics and budget demotion.
4. **Transports** — protocol correctness and concurrency (stdio/SSE especially).
5. **Wiring gap** between rich policy file and runtime kernel program.

## 5. Config surface (as consumed)

From `MCPServerConfig` / `IntegrationsConfig`:

| Field | Meaning |
|-------|---------|
| `id` / map key | Server ID |
| `enabled` | Include in ToMCPServerConfigs |
| `protocol` | `http` \| `stdio` \| `sse` (default http) |
| `base_url` | HTTP/SSE base |
| `endpoint` | Stdio command line (on MCPServerConfig) |
| `timeout` | Parsed duration; default 30s (scraper/browser specials in config) |
| `auto_connect` | ConnectAll eligibility |
| `auto_discover_tools` | Background DiscoverTools after connect |

Selection defaults (`DefaultToolSelectionConfig`): full≥70, condensed≥40, minimal≥20, logic 0.7, vector 0.3, max full 10, max condensed 20, token budget 4000.
