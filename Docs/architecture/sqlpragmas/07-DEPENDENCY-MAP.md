# sqlpragmas — Dependency Map

> Last verified: **2026-07-13**  
> Evidence from source imports and repo-wide grep.

## Upstream (what sqlpragmas imports)

```
sqlpragmas
  ├── database/sql          (stdlib)
  ├── fmt                   (stdlib)
  └── codenerd/internal/logging
```

| Dependency | Symbols used | Why |
|------------|--------------|-----|
| `database/sql` | `*sql.DB`, `Exec` | Apply PRAGMAs |
| `fmt` | `Sprintf` | `mmap_size` byte values |
| `internal/logging` | `Get`, `CategoryStore`, `Debug` | Soft-fail logging |

**Does not import:** core, store, mangle, prompt, session, config, features, mcp, perception, init, system, embedding, tools.

## Downstream (who imports sqlpragmas)

### Direct import `codenerd/internal/sqlpragmas`

| Importer | Path evidence | Typical profile |
|----------|---------------|-----------------|
| store (façade) | `internal/store/pragmas.go` | re-export only |
| mcp | `internal/mcp/store.go` | Hot |
| prompt | `internal/prompt/loader.go`, `loader_embedding.go`, `compiler_db.go` | Hot / BulkBuild |
| prompt/sync | `internal/prompt/sync/synchronizer.go` | Hot |
| system | `internal/system/factory.go`, `agent_registry.go` | Hot / BulkBuild / Query |
| init | `internal/init/profile.go`, `validation.go` | BulkBuild / Query |
| core | `internal/core/predicate_corpus.go` | ReadOnly |
| northstar | `internal/northstar/store.go` | Hot |
| context | `internal/context/feedback_store.go` | Hot |
| perception | `internal/perception/semantic_classifier.go` | Hot |
| autopoiesis | `internal/autopoiesis/prompt_evolution/strategy_store.go`, `feedback_collector.go` | Hot |
| chat CLI | `cmd/nerd/chat/session_boot.go`, `ingest.go` | Hot / BulkBuild |
| corpus_builder | `cmd/tools/corpus_builder/main.go` | BulkBuild |

### Via store re-export (same function)

| Importer | Path evidence | Typical profile |
|----------|---------------|-----------------|
| store internals | `local_core.go`, `learned_store.go`, `learning.go`, `tool_store.go`, `embedded_store.go`, `migrations.go` | Hot / ReadOnly / Bulk / Query |
| query-kb | `cmd/query-kb/main.go`, `deep_query.go` | Query |
| prompt_builder | `cmd/tools/prompt_builder/main.go` | BulkBuild |
| predicate_corpus_builder | `cmd/tools/predicate_corpus_builder/main.go` | BulkBuild |

## Dependency direction diagram

```
                    ┌──────────────┐
                    │   logging    │
                    └──────▲───────┘
                           │
                    ┌──────┴───────┐
                    │  sqlpragmas  │  ◄── leaf
                    └──────▲───────┘
           ┌───────────────┼────────────────────────┐
           │               │                        │
    ┌──────┴──────┐  ┌─────┴─────┐          ┌───────┴────────┐
    │    store    │  │    mcp    │   …      │ prompt, system │
    │ (re-export) │  │ (direct)  │          │ core, init, …  │
    └──────▲──────┘  └───────────┘          └────────────────┘
           │
    ┌──────┴──────┐
    │ query-kb,   │
    │ builders    │
    │ (via store) │
    └─────────────┘
```

## Why mcp cannot use store for pragmas

Historical / structural: `internal/mcp` sits such that importing full `store` risks cycles or heavy coupling. Package comments on both `sqlpragmas` and `store/pragmas.go` call out mcp (and similar) as **direct leaf consumers**.

## Test-only dependencies

| Import | File |
|--------|------|
| `github.com/mattn/go-sqlite3` | both test files |
| `github.com/stretchr/testify/require` | integration test |
| `path/filepath`, `testing` | tests |

Production package does **not** import a SQLite driver — callers register/open with their chosen driver.

## Layer placement

| Layer | Packages |
|-------|----------|
| Stdlib / logging | below |
| **sqlpragmas** | infrastructure leaf |
| store, mcp, prompt, … | consumers |
| cmd/nerd, cmd/tools | top-level openers |

## Audit commands

```powershell
# Upstream
go list -f "{{.Imports}}" codenerd/internal/sqlpragmas

# Downstream
rg "codenerd/internal/sqlpragmas" -g "*.go" --glob "!*_test.go"

# Re-export users
rg "store\.ApplyDefaultPragmas|store\.Profile" -g "*.go"
```

## Circular dependency risk

| Pair | Risk |
|------|------|
| sqlpragmas → store | **Must stay zero** |
| store → sqlpragmas | Allowed (façade) |
| sqlpragmas → logging | Allowed; logging must not import sqlpragmas |
