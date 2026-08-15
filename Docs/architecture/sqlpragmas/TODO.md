# sqlpragmas — TODO

> Last verified: **2026-08-15**  
> Prioritized backlog.

## P0 — Keep true

These were prose rules. Prose rules decay on the first PR that does not read
this file, so each one is now an executable check in `internal/sqlpragmas`.
The doc entry below records *which test owns it* — the test is the authority.

- [x] When changing `pragmasFor`, update unit + integration tests in the same PR.  
      → `TestPragmasFor_WhenPresetsChange_ShouldMatchGolden` (`golden_test.go`).
      The full profile × host-class matrix lives in `testdata/pragmas.golden`;
      any edit to a value, an order, or a profile fails until the author
      regenerates it (`-update`) and reads the diff.
- [x] When adding a profile, update this corpus (`IMPLEMENTED_SPEC`, API, failure modes).  
      → `TestProfileConstants_WhenProfileAdded_ShouldAppearInCorpus` and
      `TestExportedAPI_WhenSymbolAdded_ShouldAppearInPublicAPIDoc`
      (`corpus_coverage_test.go`). Both read the package AST, so they cannot
      fall behind the code.
- [x] Never add mid-layer imports to this package.  
      → `TestPackageImports_WhenNewImportAdded_ShouldStayLeaf`
      (`imports_test.go`). Allowed: stdlib + `codenerd/internal/logging`.
      Fails naming the offending import; a stale allowlist entry fails too.

## P1 — Process / hygiene

- [x] Periodic audit: product `sql.Open` sites without `ApplyDefaultPragmas` (or explicit exception comment).  
      → `TestSQLOpenSites_WhenOpeningSQLite_ShouldApplyPragmasOrBeExempt`
      (`open_site_audit_test.go`), modelled on
      `internal/build/go_invocation_inventory_test.go`. Repo-wide AST scan of
      non-test files; every `sql.Open`/`sql.OpenDB` must apply a profile (or a
      connector hook) or be listed in `unpragmaedOpens` with a reason. Stale
      exemptions fail. **Audit result at time of writing: all 32 production
      open sites apply a profile; the exemption map is empty.**
- [x] Prefer `sqlpragmas` import in new mid-layer packages that must not touch `store`.  
      → `TestPragmaSurface_WhenNewPackageAppliesPragmas_ShouldPreferTheLeaf`.
      The four packages that reach pragmas via `store.ApplyDefaultPragmas`
      (`cmd/query-kb`, `cmd/tools/prompt_builder`,
      `cmd/tools/predicate_corpus_builder`) are pinned with reasons; a new one
      fails until it imports the leaf or justifies itself.

## P2 — Product quality

- [x] Optional modernc.org/sqlite integration test (build tag) to catch reject sets early.  
      → `modernc_integration_test.go`, `go test -tags sqlite_modernc ./internal/sqlpragmas/`.
      **Measured result: at modernc.org/sqlite v1.50.1 the reject set is
      empty** — the pure-Go driver accepts the entire preset table, contrary to
      the long-standing package comment. The comment has been corrected. The
      reject set is now pinned in `knownModerncRejects`, so both a new
      rejection and a fixed one fail the test.
- [x] Document multi-conn pool guidance next to major store openers (or shared helper).  
      → Shared helper, per the "or": `OpenWithPragmas` / `NewConnector`
      (`connector.go`). Guidance is in the package comment and
      06-PUBLIC-API-AND-TYPES.md. Prose next to 32 openers would have been 32
      copies of a rule to decay.
- [x] Consider Debug log including profile name on failure only.  
      → Done. `pragma %q failed (profile %s): %v`; success stays silent.
      `PragmaProfile.String()` also keys the failure metrics.

## P3 — Ergonomics

- [x] `EnableForeignKeys(db *sql.DB)` helper for schemas ready to enforce FKs.  
      → Returns an error (unlike `ApplyDefaultPragmas`) and verifies the
      read-back, because SQLite silently ignores the pragma on builds without
      FK support.
- [x] Idempotency tests for BulkBuild / Query / ReadOnly.  
      → `TestApplyDefaultPragmas_WhenReapplied_ShouldBeIdempotent`, comparing a
      snapshot of all seven pragmas rather than a chosen subset.
- [x] Named Go constants for cache/mmap sizes (readability) without changing values.  
      → `kib`/`mib`/`gib`, `mmapHot`, `cacheHotKiB`, `busyTimeoutMS`,
      `walCheckpointBulk`, … Values unchanged; the golden file is the proof.

## P4 — Aspirational

- [x] Config/env overrides for host class (laptop vs workstation).  
      → `HostClass` + `SetHostClass` / `NERD_SQL_HOST_CLASS`. Scales cache and
      mmap only. Config is pushed down rather than imported, which is what
      keeps the leaf a leaf.
- [x] `database/sql` connector hook helper for per-connection apply.  
      → `OpenWithPragmas` / `NewConnector`. A companion test pins the defect it
      fixes, so the connector cannot be deleted as redundant.
- [x] Metrics counter for pragma failures (behind observability flag).  
      → `metrics.go`. Off by default; `SetMetricsEnabled` or
      `NERD_SQL_PRAGMA_METRICS`. Per-profile and per-statement; the
      per-statement view is the driver reject-set view.

## Still open

- Adoption of `OpenWithPragmas` at the 32 existing call sites. Not urgent:
  their pools are effectively single-connection, which is why the current
  apply-once pattern has held. Any site that raises `MaxOpenConns` must switch.
- Automatic host-class detection from available RAM. Deliberately not done —
  a declared host class keeps behavior reproducible across machines.
- Which schemas opt into `EnableForeignKeys`, and after what orphan-repair
  migration (OPEN-QUESTIONS Q3).

## Explicitly not planned

- Returning `error` from `ApplyDefaultPragmas` without a repo-wide migration.  
- Mangle predicates for pragma application.  
- Automatic profile selection from file path heuristics.
