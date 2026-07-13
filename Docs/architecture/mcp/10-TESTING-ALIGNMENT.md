# 10 — Testing Alignment: MCP

> Last verified against codebase: 2026-07-13

## 1. Commands

```powershell
go test ./internal/mcp/...
go test ./internal/mcp/... -count=1
go test ./internal/mcp/... -run Compiler -v
go test ./tests/e2e/ -run MCP -count=1
```

CGO + sqlite-vec builds (for real vec0 paths) use root `AGENTS.md` flags:

```powershell
$env:CGO_CFLAGS = "-IC:/CodeProjects/codeNERD/sqlite_headers"
go test ./internal/mcp/...
```

## 2. Test inventory

| File | Intent |
|------|--------|
| `analyzer_test.go` | JSON extraction, normalize*, infer*, analyze edges |
| `analyzer_coverage_test.go` | formatJSON, categories, build prompt/embed text |
| `client_boundary_test.go` | Empty config, bad protocol, malformed tool IDs, extreme durations |
| `client_coverage_test.go` | Broad manager surface with mock transport |
| `compiler_test.go` | fallback modes, budget demotion, duplicates, mangle case, extremes |
| `compiler_select_test.go` | Tier ordering, shard slash strip, SSE URL helper |
| `store_test.go` | Server/tool lifecycle, nulls, coercion, conflicts |
| `store_coverage_test.go` | float bytes, cosine similarity |
| `store_query_test.go` | Query paths on temp DB |
| `transport_http_test.go` | HTTP lifecycle + connect failure |
| `renderer_coverage_test.go` | All render modes and JSON validity |
| `types_coverage_test.go` | Enums + defaults + IsMCPTool |
| `integration_coverage_test.go` | Adapter + bridge accessors |
| `integration_bridge_test.go` | NewMCPIntegrationBridge temp workspace |
| `mcp_client_integration_test.go` | Suite-style client integration |
| `export_test.go` | Test exports of unexported helpers |

External:

| File | Intent |
|------|--------|
| `tests/e2e/mcp_virtualstore_integration_test.go` | VS proxy contracts |
| `internal/config/config_defaults_test.go` | `ToMCPServerConfigs` |
| `internal/system/factory_adapters_test.go` | MCP kernel adapter |

## 3. Coverage strengths

- Selection math and budget demotion well unit-tested without live MCP servers.  
- Manager edges (nil configs, disconnected call, discover failures) covered with mocks.  
- Store lifecycle and embedding math helpers covered.  
- Renderer output structure covered.  
- VirtualStore security proxy has dedicated e2e suite.

## 4. Coverage gaps

| Gap | Risk |
|-----|------|
| Live HTTP/SSE/stdio against real MCP servers | Protocol drift undetected |
| End-to-end: discover → assert facts → mangleSelect path | Policy currently unloaded; no golden |
| Boot factory MCP block integration test | Wiring regressions |
| Concurrent discover + call under load | Race beyond unit mocks |
| sqlite-vec path on CI without CGO | Always brute force in some envs |
| Campaign store affinity behavior | Lives under campaign package tests (if any) |

## 5. Recommended additions

1. **Mangle golden**: EDB fixture of registered tools + vector scores → expect `mcp_tool_selected` rows (after policy is loadable).  
2. **Fact emitter unit**: SaveTool → Assert strings (when implemented).  
3. **Fake MCP server** httptest for HTTP transport list/call.  
4. **Bridge retain/compile** smoke: factory-style construct → CompileToolsForShard non-empty after seeding store.  
5. **Race detector**: `go test -race ./internal/mcp/...` on manager+store.

## 6. Alignment to quality bar

Package tests are **above average** for unit density relative to source size (16 test files / 10 sources). Residual risk is **integration truth** (policy + boot + real transports), not missing table-driven helpers.
