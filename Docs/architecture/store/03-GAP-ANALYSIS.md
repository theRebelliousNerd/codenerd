# store — Gap Analysis

> Last verified: **2026-07-13**
> Method: vision / north-star claims vs `internal/store` reality.

> **Status, 2026-09-25.** The three "Yes" rows are resolved: ANN drift is
> measured (`vec_index_missing`) and healed by the maintenance cycle
> (`ReconcileVecIndex`, `2acad15`); `GetStats` counts all fifteen tables and
> the reflection backlog (`2acad15`); a unified Store interface is declined
> (no consumer needs one). The reflection-backlog row is now observable
> rather than silent. Evidence per item: `TODO.md`.

## Spec vs reality matrix

| Claim | Reality | Gap? |
|-------|---------|------|
| Multi-tier SQLite memory | Implemented in LocalStore schema | No |
| Semantic vector search | Engine + ANN/brute/keyword ladder | No |
| sqlite-vec optional/required by tag | `vec_support_*.go`, `init_vec.go` | No |
| Cold + archival lifecycle | Full API + maintenance | No |
| Knowledge graph + hydrate | Implemented | No |
| World fast/deep cache | Implemented | No |
| Session + compressed state | Implemented | No |
| Prompt atoms on disk | Implemented | No |
| Reasoning traces + reflection | Implemented + worker | No |
| Autopoiesis learnings | LearningStore separate DBs | No |
| Tool journal isolation | ToolStore + cleanup | No |
| Embedded intent corpus | Implemented when baked | Partial in dev without corpus build |
| Unified Store interface | Concrete types only | **Yes (soft)** |
| Auto-heal ANN drift | Warn only; backfill on engine set | **Yes** |
| Complete GetStats | Subset of tables | **Yes (minor)** |
| Cross-DB transactions | Not present | Intentional non-goal |
| Constitutional checks in store | Not present | Correct non-goal |
| Multi-process concurrent writers | Single conn | Intentional limit |
| Remote/blob cold tier | Local SQLite only | Horizon |

## Prioritized gaps

### P1 — Operational correctness

1. **ANN drift reconcile**  
   After warned vec_index failures, no continuous job rebuilds missing rowids. Background `backfillVecIndex` on engine attach helps but is not a periodic healer.

2. **Reflection backlog under heavy load**  
   45s / batch 32 may lag large campaign trace volumes; ops need force re-embed path (exists) but productized scheduling is thin.

### P2 — API ergonomics

3. **No single interface** for LocalStore + LearningStore + ToolStore — complicates tests/mocks outside package (package has mocks for embedding only).

4. **GetStats incomplete** — omit `reasoning_traces`, `prompt_atoms`, `review_findings`, `task_verifications`, etc.

### P3 — Documentation / naming

5. Comments still say “Shards B/C/D” while many more tables exist — mitigated by this corpus, not code rename.

6. Dual ownership of prompt atoms (store vs prompt package) can look like duplication without wiring docs.

## Non-gaps (do not “fix”)

| Observation | Why not a gap |
|-------------|----------------|
| No `.mg` in store | Store is not executive logic |
| Separate tools.db | Explicit design to avoid bloating KB |
| MaxOpenConns=1 | SQLite safety for this process model |
| Keyword fallback without engine | Graceful degrade |
| Embedded corpus missing in dev | Requires build pipeline; fails closed with error |

## Recommended sequencing

1. ANN drift audit command or startup reconcile (P1)  
2. Expand `GetStats` (P2)  
3. Optional thin interfaces at consumer edges only if tests demand (P2)  
4. Keep horizon remote cold store off the critical path

## Learned-corpus constructor cancellation

**GAP-STORE-LEARNED-CONSTRUCTOR-CONTEXT:** learned-corpus startup must inherit
the caller's context through connection readiness, profile application, and
schema creation. The contextless `NewLearnedCorpusStore` remains a compatibility
wrapper, not the production cold-bootstrap path. Checks before and after a
contextless constructor do not cancel its connection readiness or schema work.
Use context-aware SQL on the actual constructor path, with the shared hot profile
on every connection. Failed construction closes owned handles, joins cleanup
errors, preserves typed cancellation/deadline causes, and publishes no successful
store. Do not detach blocked constructor work into an orphan goroutine.

Acceptance requires synchronized real SQL contention/driver blocking, cancellation
and drain, successful retry/reopen, unchanged schema and existing vector data,
legacy wrapper compatibility, and no filesystem effects for pre-canceled calls.
The normal perception path must call the context-aware backend. Optional backend
outage with a live parent may degrade visibly; parent cancellation may not.

The production constructor is `internal/store/learned_store.go:58`. Focused
acceptance in `internal/store/learned_constructor_context_test.go:230` and
`internal/perception/learned_constructor_context_test.go:139` passes real SQL
contention, cancellation/drain, unchanged-data, retry, and perception-path
controls. This establishes the named bootstrap contract, not all storage or
whole-session acceptance.


## Required contract: Migration failure visibility

Existing-table schema inspection, row iteration and additive ALTER errors must propagate. Missing tables retain supported skip behavior. An add-column race may succeed only if reinspection proves the desired column exists. LearningStore must close and decline publication on migration failure.

| Gap ID | Capability | Current state / deficiency | Target | Severity | Phase | Dependencies | Exit criteria |
|---|---|---|---|---|---|---|---|
| GAP-STORE-MIGRATION-FAILURES | Migration failure visibility | `RunMigrations` returns schema and migration failures; LearningStore declines publication when migration fails (`internal/store/migrations.go:86`, `internal/store/learning.go:64`). | Required contract above | P1 | Implemented; focused tests passed; aggregate verification pending | Existing package interfaces | Real SQLite failure through read-only/query_only cannot yield success or an admitted partial store; legacy migration succeeds through close/reopen with fields usable. |

**Verified current:** `TestRunMigrations_WhenExistingTableCannotBeAltered_ShouldReturnError` (`internal/store/migrations_test.go:49`), `TestLearningStore_WhenMigrationFails_ShouldNotPublishShardDB` (`internal/store/learning_migration_failure_test.go:15`), and `TestLocalStore_MigratesPopulatedLegacyColdStorageAcrossReopen` (`internal/store/legacy_migration_test.go:12`) pass with migration-race and legacy-reopen controls in the focused store suite.


### Legacy-row compatibility of strict migration failure handling

GAP-STORE-MIGRATION-FAILURES must also preserve populated legacy cold_storage databases. SQLite cannot add a non-constant CURRENT_TIMESTAMP default to such a table: add the missing nullable last_accessed field, backfill missing values from existing update/create time with current-time fallback, and initialize last_accessed explicitly for new facts. Backfill must be restartable and errors visible. A populated legacy database must migrate, reopen, retrieve and store facts with usable access timestamps.

**Verified current:** `RunMigrations` performs restartable timestamp backfill (`internal/store/migrations.go:179`), while cold-storage writes initialize access time (`internal/store/local_cold.go:80`). The populated legacy close/reopen regression at `internal/store/legacy_migration_test.go:12` passes.
