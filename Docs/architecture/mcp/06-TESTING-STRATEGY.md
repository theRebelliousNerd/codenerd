# mcp — Testing Strategy

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/mcp/` (complete internal coverage)
> **Implementation: `internal/mcp/` — 10 non-test .go, 16 tests, 1 .mg**


## Current inventory

| Kind | Count |
|------|------:|
| Source files | 10 |
| Test files | 16 |

## Recommended focus

1. **Unit** — table-driven tests for exported funcs in largest files (see 02-CURRENT-STATE).
2. **Race** — any goroutine/shared state paths under `internal/mcp/`.
3. **Integration** — callers from session/shards/core as applicable.
4. **Mangle** — if package ships `.mg` or feeds kernel facts, validate Decl + load.

## Commands

```powershell
go test ./internal/mcp/...
go test -race ./internal/mcp/...
```

## Sample existing tests

| Path | Lines |
|------|------:|
| `internal/mcp/client_coverage_test.go` | 686 |
| `internal/mcp/mcp_client_integration_test.go` | 357 |
| `internal/mcp/renderer_coverage_test.go` | 348 |
| `internal/mcp/store_test.go` | 298 |
| `internal/mcp/analyzer_test.go` | 270 |
| `internal/mcp/compiler_test.go` | 269 |
| `internal/mcp/analyzer_coverage_test.go` | 182 |
| `internal/mcp/integration_coverage_test.go` | 169 |
| `internal/mcp/client_boundary_test.go` | 152 |
| `internal/mcp/transport_http_test.go` | 135 |
