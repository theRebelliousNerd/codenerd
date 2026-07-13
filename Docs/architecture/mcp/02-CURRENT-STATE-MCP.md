# mcp — Current State

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/mcp/` (complete internal coverage)
> **Implementation: `internal/mcp/` — 10 non-test .go, 16 tests, 1 .mg**


## 1. Source location

- Primary package: `internal/mcp/` (exists; 10 non-test Go files)
- 1:1 mapping: `Docs/architecture/mcp/` ↔ `internal/mcp/`

## 2. Largest source files

| Path | Lines | Kind |
|------|------:|------|
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

## 3. Complete source inventory (capped)

| Path | Lines |
|------|------:|
| `internal/mcp/analyzer.go` | 523 |
| `internal/mcp/client.go` | 545 |
| `internal/mcp/compiler.go` | 363 |
| `internal/mcp/integration.go` | 183 |
| `internal/mcp/renderer.go` | 227 |
| `internal/mcp/store.go` | 683 |
| `internal/mcp/transport_http.go` | 291 |
| `internal/mcp/transport_sse.go` | 439 |
| `internal/mcp/transport_stdio.go` | 478 |
| `internal/mcp/types.go` | 267 |

## 4. Tests (sample)

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

## 5. Behavior summary

Package **mcp** is a living codeNERD subsystem: MCP server/client integration surfaces.

APIs and control flow are defined by the cited source files — this corpus does not invent signatures.

## 6. Known limitations

- Inventory/type extraction is automated; deepen narrative when this package is the active design target.
- Completeness heuristic (90%) is not go cover %.
- Cross-package wiring claims require grep of registration hubs before treating as live.
