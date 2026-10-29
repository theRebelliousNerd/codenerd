# store — Observability

> Last verified: **2026-07-13**  
> Package: `internal/store/`

## Logging category

Primary category: **`logging.CategoryStore`**

Helpers used throughout:

| Helper | Use |
|--------|-----|
| `logging.Store(...)` | Info-level store events |
| `logging.StoreDebug(...)` | Verbose path details |
| `logging.Get(CategoryStore).Error/Warn` | Failures / non-fatal issues |
| `logging.StartTimer(CategoryStore, op)` | Latency of hot ops |

## Timed operations (representative)

| Op name | Where |
|---------|-------|
| `NewLocalStore` | `local_core.go` |
| `RunMigrations` | `migrations.go` |
| `StoreFact` / `LoadFacts` / … | `local_cold.go` |
| `StoreVectorWithEmbedding` / batch / semantic recall | `vector_store.go` |
| `NewEmbeddedCorpusStore` / Search* | `embedded_store.go` |
| Knowledge dual-write / semantic search | `local_knowledge.go` |

## Notable log signals

| Signal | Severity | Meaning |
|--------|----------|---------|
| sqlite-vec extension detected and enabled | Info | ANN available |
| sqlite-vec extension not available; continuing… | Warn | Degraded search |
| sqlite-vec extension not available (require) | Error + fail open | Tagged build fail-fast |
| vec_index insert failed … (ANN drift) | Warn | Dual-write broken |
| Background vector index backfill starting/completed | Info | Boot non-block backfill |
| Content hash backfill had issues | Warn | Non-fatal maintenance |
| Deduped N predicate vectors | Info | Hygiene on init |
| Failed to create * indexes | Warn | Perf only |
| LearningStore / ToolStore init paths | Info/Debug | Satellite open |

## Stats APIs

| API | Returns |
|-----|---------|
| `LocalStore.GetStats` | map of table → row count (subset) |
| `LearningStore.GetStats(shard)` | learning stats map |
| `TraceStore` / `GetTraceStats` | via LocalStore facade |
| `ToolStoreStats` | executions, sizes, runtime hours, success/fail |
| `EmbeddedCorpusStore.GetStats` | corpus stats map |
| `LearnedCorpusStore.GetStats` | pattern stats |
| `MaintenanceStats` | archive/purge/vacuum outcomes |
| `CleanupStats` | tool cleanup outcomes |
| `ReembedResult` | multi-DB reembed summary |
| `MigrationResult` | version transition + backup path |

## Debug hooks

- `GetDB()` for ad-hoc inspection (tests / advanced tooling) — treat as escape hatch.
- `GetTraceStore()` for direct self-learning queries.
- Progress callback on `ReembedAllDBsForce` for CLI progress lines.

## Gaps

1. No Prometheus/OpenTelemetry counters — logging-only.
2. `GetStats` incomplete vs full schema.
3. ANN drift not elevated to a first-class metric/counter.
4. Reflection backlog depth only via candidate count APIs — not periodic emission.

## Operator playbook (observability)

1. On “semantic search empty”: check warn logs for ANN drift / missing engine / vec extension.  
2. On slow boot: look for backfill timers and embedding engine init outside store.  
3. On disk growth: tool store stats + cold maintenance stats + vacuum flag.  
4. After model change: run re-embed and inspect `ReembedResult` fields.
