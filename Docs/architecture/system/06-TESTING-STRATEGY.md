# system — Testing Strategy

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/system/` (complete internal coverage)
> **Implementation: `internal/system/` — 5 non-test .go, 11 tests, 1 .mg**


## Current inventory

| Kind | Count |
|------|------:|
| Source files | 5 |
| Test files | 11 |

## Recommended focus

1. **Unit** — table-driven tests for exported funcs in largest files (see 02-CURRENT-STATE).
2. **Race** — any goroutine/shared state paths under `internal/system/`.
3. **Integration** — callers from session/shards/core as applicable.
4. **Mangle** — if package ships `.mg` or feeds kernel facts, validate Decl + load.

## Commands

```powershell
go test ./internal/system/...
go test -race ./internal/system/...
```

## Sample existing tests

| Path | Lines |
|------|------:|
| `internal/system/agent_registry_coverage_test.go` | 334 |
| `internal/system/dom_demo_test.go` | 205 |
| `internal/system/factory_boot_test.go` | 162 |
| `internal/system/factory_test.go` | 120 |
| `internal/system/dom_mangle_test.go` | 107 |
| `internal/system/tool_compilation_test.go` | 95 |
| `internal/system/mocks_test.go` | 87 |
| `internal/system/session_kernel_adapter_test.go` | 71 |
| `internal/system/factory_helpers_test.go` | 63 |
| `internal/system/factory_adapters_test.go` | 57 |
