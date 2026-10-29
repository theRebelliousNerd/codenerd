# sqlpragmas — Architecture Corpus

> Last verified: **2026-07-13**  
> Source: [`internal/sqlpragmas/`](../../../internal/sqlpragmas/)  
> Status: Living reference — code-grounded full corpus  
> Scale: **1** non-test Go file (~125 lines), **2** test files, **0** Mangle sources  

## Role

`sqlpragmas` is the **cycle-free leaf** that applies curated SQLite `PRAGMA` presets after `sql.Open`. It exists so every long-lived agent store, bulk builder, short query tool, and read-only open can share one tuned profile table without pulling `internal/store` into import cycles (notably `internal/mcp` and other mid-layer packages).

It sits **below** fact-flow — not on the OODA path itself — but every durable SQLite surface the agent touches depends on it for WAL, cache, mmap, and busy-timeout posture.

```
sql.Open(...)
    → sqlpragmas.ApplyDefaultPragmas(db, Profile*)
    → schema / first query
```

## Scope

| In scope | Out of scope |
|----------|--------------|
| Profile enum + ordered PRAGMA lists | Schema design, migrations |
| `ApplyDefaultPragmas` contract (best-effort, no error return) | Connection pool sizing (`SetMaxOpenConns`) |
| Driver coexistence (mattn CGO vs modernc pure-Go) | Embedding / sqlite-vec loading |
| `store` re-export façade documentation | Foreign-key enforcement policy (intentionally omitted) |
| Call-site wiring map | DSN construction, backup, vacuum |

## Document map

| Doc | Purpose |
|-----|---------|
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | **Flagship** — inventory, profiles, flows, integration map |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment scores with evidence |
| [01-VISION.md](01-VISION.md) | Target architecture for pragma governance |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | Precise file/symbol inventory and hotspots |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Spec vs reality, priorities, non-gaps |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Binding design principles for this package |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Components, data flow, profile matrix |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | Exported API with file refs |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Upstream / downstream imports with evidence |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | Boot, CLI, store re-export, call sites |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Safety, concurrency, FK omission |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Existing tests, gaps, commands |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | Logging category and debug hooks |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Concrete failures + mitigations |
| [TODO.md](TODO.md) | Prioritized backlog |
| [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) | Real open design questions |
| [_progress.md](_progress.md) | Rebuild progress log |

## Verify

```powershell
# Unit + integration tests (requires CGO sqlite; uses mattn/go-sqlite3)
go test ./internal/sqlpragmas/...

# With project sqlite headers (repo-standard CGO path)
$env:CGO_CFLAGS = "-IC:/CodeProjects/codeNERD/sqlite_headers"
go test ./internal/sqlpragmas/...

# Reverse-dep audit (who still opens SQLite without this helper)
rg "ApplyDefaultPragmas|sqlpragmas" -g "*.go"
```

## North-star placement

| Dimension | Placement |
|-----------|-----------|
| LLM creative center | **None** — pure infrastructure |
| Mangle executive | **None** — no predicates, no policy |
| Constitutional safety | **Indirect** — durable stores must open reliably; FK default left off |
| JIT prompt atoms | **None** |
| Wiring-before-delete | **Critical** — widely re-exported via `store` and imported by mcp/prompt/core/system |

## Quick facts

| Fact | Value |
|------|-------|
| Package path | `codenerd/internal/sqlpragmas` |
| Single entrypoint | `ApplyDefaultPragmas(*sql.DB, PragmaProfile)` |
| Profiles | `Hot`, `BulkBuild`, `Query`, `ReadOnly` |
| Error policy | Per-pragma failures → `logging.CategoryStore` Debug; **never** fail open |
| Re-export | `internal/store/pragmas.go` type/const/func aliases |
| Leaf deps | `database/sql`, `fmt`, `internal/logging` only |
