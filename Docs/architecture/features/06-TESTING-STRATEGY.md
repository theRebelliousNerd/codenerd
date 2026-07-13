# features — Testing Strategy

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/features/` (complete internal coverage)
> **Implementation: `internal/features/` — 1 non-test .go, 3 tests, 0 .mg**


## Current inventory

| Kind | Count |
|------|------:|
| Source files | 1 |
| Test files | 3 |

## Recommended focus

1. **Unit** — table-driven tests for exported funcs in largest files (see 02-CURRENT-STATE).
2. **Race** — any goroutine/shared state paths under `internal/features/`.
3. **Integration** — callers from session/shards/core as applicable.
4. **Mangle** — if package ships `.mg` or feeds kernel facts, validate Decl + load.

## Commands

```powershell
go test ./internal/features/...
go test -race ./internal/features/...
```

## Sample existing tests

| Path | Lines |
|------|------:|
| `internal/features/config_roundtrip_test.go` | 192 |
| `internal/features/features_test.go` | 153 |
| `internal/features/features_defaults_test.go` | 42 |
