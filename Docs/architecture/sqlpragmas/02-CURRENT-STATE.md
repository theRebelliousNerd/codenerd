# sqlpragmas — Current State

> Last verified: **2026-07-13**  
> Precise inventory of `internal/sqlpragmas/` as it exists on disk.

## Package stats

| Metric | Value |
|--------|------:|
| Non-test Go files | 1 |
| Test Go files | 2 |
| Mangle (`.mg`) | 0 |
| YAML/JSON config | 0 |
| Package README / agents.md | 0 (docs live under `Docs/architecture/sqlpragmas/`) |
| Production LOC (approx) | ~125 |
| Test LOC (approx) | ~287 |

## File inventory

### `internal/sqlpragmas/pragmas.go`

| Region | Lines (approx) | Content |
|--------|----------------|---------|
| Package doc | 1–16 | Leaf intent, driver coexistence, workstation tuning |
| Imports | 18–23 | `database/sql`, `fmt`, `internal/logging` |
| `PragmaProfile` + consts | 25–51 | Hot / BulkBuild / Query / ReadOnly with workload comments |
| `ApplyDefaultPragmas` | 53–70 | nil guard, logger, loop, Debug on error |
| `pragmasFor` | 72–124 | switch with ordered PRAGMA lists; FK omission note |

**Hotspots:** entire file is the hotspot — no subpackages.

### `internal/sqlpragmas/pragmas_test.go`

| Symbol | Role |
|--------|------|
| `openTempDB` | Tempfile sqlite3, MaxOpen/Idle=1 |
| `pragmaInt` / `pragmaString` | Read-back helpers |
| `assertMmapEnabled` | Soft mmap assert (`> 0`) |
| `TestApplyDefaultPragmas_ProfileHot` | Full Hot matrix |
| `TestApplyDefaultPragmas_ProfileBulkBuild` | Bulk cache + checkpoint |
| `TestApplyDefaultPragmas_ProfileQuery` | Query cache |
| `TestApplyDefaultPragmas_ProfileReadOnly_NoJournalChange` | No WAL |
| `TestApplyDefaultPragmas_NilDB_NoPanic` | nil safety |

### `internal/sqlpragmas/pragma_integration_test.go`

| Symbol | Role |
|--------|------|
| `TestApplyDefaultPragmas_AllProfilesIntegration` | Table-driven all profiles |
| `TestApplyDefaultPragmas_Idempotent` | Triple apply Hot |
| `openFreshTempDB` | Integration open helper |
| `readPragmaInt` / `readPragmaString` | testify-backed readers |

## Exported surface (complete)

| Kind | Name | File |
|------|------|------|
| type | `PragmaProfile` | `pragmas.go` |
| const | `ProfileHot` | `pragmas.go` |
| const | `ProfileBulkBuild` | `pragmas.go` |
| const | `ProfileQuery` | `pragmas.go` |
| const | `ProfileReadOnly` | `pragmas.go` |
| func | `ApplyDefaultPragmas` | `pragmas.go` |

Unexported: `pragmasFor`.

## Profile → PRAGMA matrix (source truth)

| PRAGMA | Hot | BulkBuild | Query | ReadOnly |
|--------|:---:|:---------:|:-----:|:--------:|
| `journal_mode=WAL` | ✓ | ✓ | ✓ | — |
| `synchronous=NORMAL` | ✓ | ✓ | ✓ | — |
| `temp_store=MEMORY` | ✓ | ✓ | ✓ | ✓ |
| `busy_timeout=10000` | ✓ | ✓ | ✓ | ✓ |
| `mmap_size` | 8 GiB | 16 GiB | 4 GiB | 4 GiB |
| `cache_size` | 2 GiB | 4 GiB | 512 MiB | 512 MiB |
| `wal_autocheckpoint` | 10000 | 20000 | — | — |

## Façade state (`internal/store/pragmas.go`)

| Re-export | Maps to |
|-----------|---------|
| `store.PragmaProfile` | `sqlpragmas.PragmaProfile` (type alias) |
| `store.ProfileHot` … `ProfileReadOnly` | matching consts |
| `store.ApplyDefaultPragmas` | function variable equal to `sqlpragmas.ApplyDefaultPragmas` |

## Call-site census (2026-07-13)

### Direct `sqlpragmas.ApplyDefaultPragmas`

Counted via repo grep (non-test product code):

| Area | Count (approx) | Dominant profile |
|------|---------------:|------------------|
| prompt (+ sync) | 5 | Hot, BulkBuild |
| system | 3 | Hot, BulkBuild, Query |
| store (re-export file only) | 1 file | — |
| core | 2 | ReadOnly |
| mcp, northstar, context, perception | 1 each | Hot |
| autopoiesis prompt_evolution | 2 | Hot |
| init | 2 | Query, BulkBuild |
| cmd/nerd/chat | 2 | Hot, BulkBuild |
| cmd/tools/corpus_builder | 1 | BulkBuild |

### Via `store.ApplyDefaultPragmas` / package-local alias

| Area | Profiles |
|------|----------|
| `internal/store` open paths | Hot, ReadOnly, BulkBuild, Query |
| `cmd/query-kb` | Query |
| `cmd/tools/prompt_builder`, `predicate_corpus_builder` | BulkBuild |

## Runtime behavior notes

1. **Silent success** — no log on full apply.
2. **Partial apply** — possible if mid-list PRAGMA fails; later pragmas still attempted.
3. **Default profile** — unknown `PragmaProfile` int values use Hot list.
4. **No mutex** — pure apply; thread-safety is that of `*sql.DB.Exec`.

## What is *not* in tree

- Config keys under `.nerd/` for pragma sizes
- Feature flags for “aggressive mmap”
- Metrics counters for pragma failures
- Second implementation (all paths go through this leaf or its re-export)

## Health assessment

| Aspect | State |
|--------|-------|
| Completeness of stated API | Done |
| Adoption | High across product SQLite |
| Documentation (prior corpus) | Thin stubs — **this rebuild replaces** |
| Risk of regression | Low if tests stay green; high blast radius if defaults change |
