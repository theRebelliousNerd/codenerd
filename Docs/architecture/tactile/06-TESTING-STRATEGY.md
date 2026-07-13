# tactile — Testing Strategy

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/tactile/` (complete internal coverage)
> **Implementation: `internal/tactile/` — 16 non-test .go, 12 tests, 0 .mg**


## Current inventory

| Kind | Count |
|------|------:|
| Source files | 16 |
| Test files | 12 |

## Recommended focus

1. **Unit** — table-driven tests for exported funcs in largest files (see 02-CURRENT-STATE).
2. **Race** — any goroutine/shared state paths under `internal/tactile/`.
3. **Integration** — callers from session/shards/core as applicable.
4. **Mangle** — if package ships `.mg` or feeds kernel facts, validate Decl + load.

## Commands

```powershell
go test ./internal/tactile/...
go test -race ./internal/tactile/...
```

## Sample existing tests

| Path | Lines |
|------|------:|
| `internal/tactile/audit_test.go` | 1113 |
| `internal/tactile/coverage_boost_test.go` | 922 |
| `internal/tactile/tactile_test.go` | 726 |
| `internal/tactile/docker_platform_test.go` | 577 |
| `internal/tactile/types_coverage_test.go` | 300 |
| `internal/tactile/docker_platform_windows_test.go` | 246 |
| `internal/tactile/swebench/coverage_boost_test.go` | 213 |
| `internal/tactile/files_test.go` | 209 |
| `internal/tactile/swebench/instance_test.go` | 118 |
| `internal/tactile/python/environment_test.go` | 78 |
