# sqlpragmas — Testing Alignment

> Last verified: **2026-07-13**

## Existing tests

### Unit — `pragmas_test.go`

| Test | Intent |
|------|--------|
| `TestApplyDefaultPragmas_ProfileHot` | cache, temp_store, journal, sync, busy, mmap soft |
| `TestApplyDefaultPragmas_ProfileBulkBuild` | bulk cache + wal_autocheckpoint |
| `TestApplyDefaultPragmas_ProfileQuery` | query cache |
| `TestApplyDefaultPragmas_ProfileReadOnly_NoJournalChange` | must not flip to WAL |
| `TestApplyDefaultPragmas_NilDB_NoPanic` | nil safety |

Helpers: `openTempDB`, `pragmaInt`, `pragmaString`, `assertMmapEnabled`.

### Integration — `pragma_integration_test.go`

| Test | Intent |
|------|--------|
| `TestApplyDefaultPragmas_AllProfilesIntegration` | Single table of documented expects for all profiles |
| `TestApplyDefaultPragmas_Idempotent` | Re-apply Hot does not drift |

Helpers: `openFreshTempDB`, `readPragmaInt`, `readPragmaString` (testify).

## Test environment constraints

| Constraint | Rationale |
|------------|-----------|
| Tempfile DSN, not `:memory:` | mmap_size reports 0 on memory DBs |
| `SetMaxOpenConns(1)` / idle 1 | PRAGMAs are per-connection |
| Driver: mattn/go-sqlite3 | CGO; matches primary Windows/dev path |
| mmap assert is `> 0` | OS/SQLite caps below requested GiB |

## Coverage map vs principles

| Principle / invariant | Covered? |
|-----------------------|----------|
| Hot values | Yes |
| Bulk checkpoint | Yes |
| Query cache | Yes |
| ReadOnly no WAL | Yes |
| Nil | Yes |
| Idempotent Hot | Yes |
| Order of PRAGMAs | Indirect (end state) |
| modernc driver | **No** |
| Idempotent other profiles | **No** |
| Multi-conn pool behavior | **No** |
| Partial failure mid-list | **No** (would need mock DB) |

## Commands

```powershell
# Package tests
go test ./internal/sqlpragmas/...

# Verbose
go test ./internal/sqlpragmas/... -v

# With repo CGO headers (when needed for other packages; sqlpragmas tests use mattn driver)
$env:CGO_CFLAGS = "-IC:/CodeProjects/codeNERD/sqlite_headers"
go test ./internal/sqlpragmas/...

# Race (optional)
go test ./internal/sqlpragmas/... -race
```

**Note:** Without a working CGO sqlite toolchain, tests will fail to build due to `mattn/go-sqlite3`.

## Gaps and recommendations

| Gap | Priority | Idea |
|-----|----------|------|
| modernc build tag test | P2 | `//go:build modernc` file opening pure-Go driver |
| Bulk/Query/ReadOnly idempotency | P3 | Extend integration test |
| Mock `sql.DB` failure injection | P3 | Ensure loop continues after first error |
| Call-site wiring test | Process | Grep audit in CI, not unit test |

## Alignment judgment

For a ~125 LOC package, test density is **high and appropriate**. The important product risk is **call-site omission** (not tested inside this package) and **driver divergence** (under-tested).
