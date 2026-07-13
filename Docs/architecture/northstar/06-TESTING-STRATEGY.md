# northstar — Testing Strategy

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/northstar/` (complete internal coverage)
> **Implementation: `internal/northstar/` — 4 non-test .go, 6 tests, 0 .mg**


## Current inventory

| Kind | Count |
|------|------:|
| Source files | 4 |
| Test files | 6 |

## Recommended focus

1. **Unit** — table-driven tests for exported funcs in largest files (see 02-CURRENT-STATE).
2. **Race** — any goroutine/shared state paths under `internal/northstar/`.
3. **Integration** — callers from session/shards/core as applicable.
4. **Mangle** — if package ships `.mg` or feeds kernel facts, validate Decl + load.

## Commands

```powershell
go test ./internal/northstar/...
go test -race ./internal/northstar/...
```

## Sample existing tests

| Path | Lines |
|------|------:|
| `internal/northstar/guardian_test.go` | 1103 |
| `internal/northstar/store_test.go` | 710 |
| `internal/northstar/types_test.go` | 623 |
| `internal/northstar/observer_test.go` | 514 |
| `internal/northstar/types_facts_test.go` | 114 |
| `internal/northstar/guardian_warn_test.go` | 71 |
