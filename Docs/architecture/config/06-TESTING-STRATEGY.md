# config — Testing Strategy

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/config/` (complete internal coverage)
> **Implementation: `internal/config/` — 17 non-test .go, 5 tests, 0 .mg**


## Current inventory

| Kind | Count |
|------|------:|
| Source files | 17 |
| Test files | 5 |

## Recommended focus

1. **Unit** — table-driven tests for exported funcs in largest files (see 02-CURRENT-STATE).
2. **Race** — any goroutine/shared state paths under `internal/config/`.
3. **Integration** — callers from session/shards/core as applicable.
4. **Mangle** — if package ships `.mg` or feeds kernel facts, validate Decl + load.

## Commands

```powershell
go test ./internal/config/...
go test -race ./internal/config/...
```

## Sample existing tests

| Path | Lines |
|------|------:|
| `internal/config/config_comprehensive_test.go` | 885 |
| `internal/config/config_test.go` | 386 |
| `internal/config/env_override_test.go` | 289 |
| `internal/config/config_defaults_test.go` | 170 |
| `internal/config/ollama_worker_config_test.go` | 33 |
