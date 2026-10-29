# store — TODO

> Last verified: **2026-09-25** (lane B wave 2, against the code)
> Prioritized backlog for `internal/store` (and docs-only notes where code change is optional).

> **Status, 2026-09-25.** Every item is resolved.
>
> | Item | Resolution | Evidence |
> |---|---|---|
> | 1. ANN drift healer | closed `2acad15` | `LocalStore.ReconcileVecIndex` (`internal/store/vector_store.go`) re-indexes embedded rows of the index's dimension that `vec_index` lacks, after any pending backfill; `MaintenanceConfig.ReconcileVecIndex`, turned on in `Cortex.runMaintenance` (`internal/system/factory.go`). The drift is the `vec_index_missing` gauge in `GetStats`. Startup was already healed: every `SetEmbeddingEngine` drops and rebuilds the index (`initVecIndex` + `backfillVecIndex`). Tests: `TestReconcileVecIndex_HealsTheDriftAFailedInsertLeaves`, `TestRunMaintenance_HealsVecIndexDrift` |
> | 2. Failure-injection tests for vec_index insert failures | closed `2acad15` | `TestReconcileVecIndex_HealsTheDriftAFailedInsertLeaves` drops the index under a write: the write survives, the drift is measured, maintenance heals it |
> | 3. Dim-change procedure | documented here | after an embedding model swap, the next engine attach recreates `vec_index` at the new dimension (`initVecIndex`); the backfill and `ReconcileVecIndex` skip rows of another dimension (`TestReconcileVecIndex_IgnoresRowsOfAnotherDimension`), which stay findable by brute force only until `nerd embedding reembed` (`store.ReembedAllDBsForce`, `cmd/nerd/embedding_cmd.go`) re-embeds them |
> | 4. Expand `GetStats` | closed `2acad15` | `statsTables` (`internal/store/local_core.go`) counts all fifteen tables, adding reasoning_traces, prompt_atoms, task_verifications, review_findings, archived_facts; `nerd memory` prints them by name (`renderStoreStats`, `cmd/nerd/cmd_systems.go`) instead of summing every table under "Vector (Embeddings)". `TestGetStats_CountsEveryTierAndTheReflectionBacklog`, `TestRenderStoreStats_NamesEachTierAndTheGauges` |
> | 5. Reflection backlog gauge | closed `2acad15` | `reflection_trace_backlog` in `GetStats` (`CountTraceEmbeddingBacklog`, which the worker computed and nothing surfaced) |
> | 6. Thin interfaces | declined | this item says not to force it, and no cross-package mock needs one: consumers take narrow interfaces at their own edges (`retrieval.FactSink`, `session.SessionPersister`) |
> | 7. "Shard B/C/D" comments | closed `2acad15` | `LocalStore`'s doc comment maps the legacy labels onto the tables; the per-file headers keep the labels the map explains |
> | 8. Prompt atom round-trip density | closed `17f7efd` | `TestPromptAtom_EverySliceFieldRoundTrips` walks `PromptAtom` by reflection, so a selector added without its column fails (the four-field `TestScanPromptAtoms_SelectorsRoundTrip` already existed) |
> | 9. World fingerprint race tests | closed `17f7efd` | `TestWorldCache_FingerprintAndFactsNeverTear`: two writers, four readers, clean under `-race` |
> | 10-12. Remote cold tier, multi-reader profile, OpenTelemetry | declined | horizon items with no consumer; the TODO says not to start them without need |
> | (08) tools.db never cleaned automatically | closed `2acad15` | `ToolStore.AutoCleanup` had no caller; `autoCleanupToolStore` runs it at boot (`initFactoryToolStore`) and each maintenance cycle. `TestInitFactoryToolStore_AutoCleansOverBudgetJournal` |
> | (08) tool smart-cleanup LLM path | declined `17f7efd` | `CleanupIntelligent` asks a model which executions to delete; retention is the budgets' decision, not a model's. `/cleanup-tools --smart` now says so instead of "not yet implemented" (`TestCleanupTools_PreviewsTheEnforcedBudgetAndDeclinesSmart`) |
> | (found) empty tools.db stats failed to scan | closed `17f7efd` | `SUM` over no rows is NULL; `COALESCE` in `getStatsLocked`. `TestToolStore_GetStatsOnAnEmptyJournal` |

## P1 — Correctness / search quality

1. **ANN drift healer** — periodic or startup reconcile of `vectors` vs `vec_index` rowids; surface count of drifted rows in stats/logs.
2. **Failure-injection tests** for vec_index insert failures (assert warning path + recovery).
3. **Documented dim-change procedure** — force re-embed after embedding model swap (ops runbook; partially covered by `ReembedAllDBsForce`).

## P2 — API / observability

4. **Expand `GetStats`** to include `reasoning_traces`, `prompt_atoms`, `task_verifications`, `review_findings`, `archived_facts`.
5. **Reflection backlog gauge** — log or stats for pending trace/learning embedding candidates.
6. **Optional thin interfaces** at consumer edges if cross-package mocks become painful (do not force premature abstraction).

## P3 — Ergonomics / hygiene

7. **Rename/clarify “Shard B/C/D” comments** in `local_core.go` to match expanded tier map (docs done; code comments lag).
8. **Prompt atom test density** for polymorphism columns and selector JSON round-trips.
9. **World fingerprint race tests** under concurrent scan + read.

## P4 — Horizon (do not start without need)

10. Remote/blob archival tier for huge cold sets.
11. Multi-reader pragma profile for read-mostly analytics processes.
12. OpenTelemetry metrics export.

## Explicitly not TODO

- Implementing `permitted(...)` inside store.
- Merging tools.db into knowledge.db.
- Adding package-local Mangle `.mg` files.
