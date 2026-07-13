# store — Failure Modes

> Last verified: **2026-07-13**  
> Package: `internal/store/`

## Catalog

### F1 — Cannot create/open knowledge.db

| | |
|--|--|
| **Symptom** | Boot without LocalStore; no durable memory |
| **Causes** | Permissions, full disk, bad path |
| **Code** | `NewLocalStore` MkdirAll / sql.Open / initialize errors |
| **Mitigation** | Ensure `.nerd` writable; disk free space; surface factory error |

### F2 — Schema initialize / migration failure

| | |
|--|--|
| **Symptom** | NewLocalStore returns error; DB closed |
| **Causes** | Corrupt file, locked DB, unexpected schema |
| **Code** | `initialize`, `RunMigrations` |
| **Mitigation** | Backup; RunAllMigrations path; restore from backup |

### F3 — sqlite-vec required but missing

| | |
|--|--|
| **Symptom** | Fail-fast open error mentioning rebuild with vec0 |
| **Causes** | `defaultRequireVec` true without extension |
| **Code** | `vec_support_enabled.go`, `NewLocalStore` |
| **Mitigation** | Build with CGO + headers + `sqlite_vec` tag; or use default optional build |

### F4 — ANN drift

| | |
|--|--|
| **Symptom** | Brute-force finds vectors; ANN returns empty/partial |
| **Causes** | vec_index insert failed; backfill incomplete; dim mismatch |
| **Code** | `StoreVectorWithEmbedding` warns; `backfillVecIndex` |
| **Mitigation** | Re-set embedding engine (triggers backfill); force re-embed; rebuild vec_index |

### F5 — Embedding engine failures

| | |
|--|--|
| **Symptom** | StoreVectorWithEmbedding errors; semantic recall fails |
| **Causes** | Ollama/API down, dim change mid-run |
| **Mitigation** | Keyword fallback when engine nil; when engine set but Embed fails, return error (caller retries) |

### F6 — Lock contention / slow TUI

| | |
|--|--|
| **Symptom** | UI stalls on store ops |
| **Causes** | Holding mutex across slow embeds; single writer; huge batch |
| **Mitigation** | Batch APIs; background backfill design; avoid large sync re-embeds on hot path |

### F7 — Reflection worker stuck or leaking

| | |
|--|--|
| **Symptom** | Goroutine leak after Close; or backlog never drains |
| **Causes** | Stop timeout 2s; embed hangs |
| **Code** | `stopReflectionWorker` |
| **Mitigation** | Close engine first; context-aware embeds; monitor backlog counts |

### F8 — Cold archival data loss

| | |
|--|--|
| **Symptom** | Facts missing after maintenance |
| **Causes** | Aggressive purge days; mistaken Archive+Purge |
| **Mitigation** | Conservative MaintenanceConfig; restore API; backups |

### F9 — World fact staleness

| | |
|--|--|
| **Symptom** | Kernel world model wrong after file edit |
| **Causes** | Fingerprint not updated; partial UpdateWorldFilesAndFacts failure |
| **Mitigation** | Rescan; delete path + rewrite; consumer integrity checks |

### F10 — Session turn collisions

| | |
|--|--|
| **Symptom** | Unexpected overwrite of session_history row |
| **Causes** | Reuse of (session_id, turn_number) intentionally upserts via UNIQUE |
| **Mitigation** | Treat as idempotent sync contract; callers allocate turn numbers carefully |

### F11 — Learning confidence decay erases value

| | |
|--|--|
| **Symptom** | Autopoiesis learnings stop applying |
| **Causes** | DecayConfidence over-aggressive |
| **Mitigation** | Tune decay; re-learn from campaigns; inspection GetStats |

### F12 — Tool DB unbounded growth

| | |
|--|--|
| **Symptom** | Disk fill; slow tool queries |
| **Causes** | No cleanup; large Result bodies |
| **Mitigation** | CleanupConfig runtime hours / size FIFO / smart cleanup |

### F13 — Embedded corpus unavailable

| | |
|--|--|
| **Symptom** | NewEmbeddedCorpusStore error in dev |
| **Causes** | intent corpus not baked |
| **Mitigation** | Run corpus build pipeline; fall back to other classifiers |

### F14 — Cross-DB inconsistency

| | |
|--|--|
| **Symptom** | Learnings exist but knowledge graph missing related edges |
| **Causes** | Separate DBs; no 2PC; partial campaign failure |
| **Mitigation** | Application-level retry; accept eventual consistency |

### F15 — Predicate vector duplicates historically

| | |
|--|--|
| **Symptom** | Bloated vectors for same predicate content |
| **Mitigation** | Init-time dedupe + partial unique index |

## Severity ranking (ops)

| Mode | Severity |
|------|----------|
| F1, F2 | High (no store) |
| F3 | High in tagged prod builds |
| F4, F5 | Medium (search quality) |
| F6 | Medium (UX) |
| F8, F9 | Medium–High (wrong memory) |
| F12 | Medium (disk) |
| F7, F10, F11, F13–F15 | Low–Medium situational |
