# store — Internal Architecture

> Last verified: **2026-07-13**  
> Package: `internal/store/`

## Component diagram

```
                    ┌─────────────────────┐
                    │  embedding.Engine   │
                    │  (optional inject)  │
                    └──────────┬──────────┘
                               │ SetEmbeddingEngine
                               ▼
┌──────────────────────────────────────────────────────────────┐
│ LocalStore                                                   │
│  db *sql.DB (MaxOpenConns=1)  mu RWMutex  vectorExt require  │
│  ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌───────────┐ │
│  │ vector_*   │ │ graph_*    │ │ cold_*     │ │ world_*   │ │
│  └────────────┘ └────────────┘ └────────────┘ └───────────┘ │
│  ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌───────────┐ │
│  │ session_*  │ │ knowledge  │ │ prompt     │ │ verify    │ │
│  └────────────┘ └────────────┘ └────────────┘ └───────────┘ │
│  ┌────────────────────────────────────────────────────────┐ │
│  │ TraceStore (same *sql.DB)                              │ │
│  └────────────────────────────────────────────────────────┘ │
│  reflection worker goroutine (optional)                      │
└──────────────────────────────────────────────────────────────┘

┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐
│ LearningStore    │  │ ToolStore        │  │ Corpus stores    │
│ map[shard]*sql.DB│  │ tools.db         │  │ Embedded RO      │
│ + reflection     │  │ + cleanup        │  │ Learned RW+vec   │
└──────────────────┘  └──────────────────┘  └──────────────────┘
```

## Key types (internal roles)

| Type | Owns | Lifetime |
|------|------|----------|
| `LocalStore` | knowledge.db multi-tier | Process / Cortex boot |
| `TraceStore` | reasoning_traces | Nested in LocalStore |
| `LearningStore` | shard learnings map | Process |
| `ToolStore` | tool_executions | Process / on demand |
| `EmbeddedCorpusStore` | temp RO corpus | Lazy, Close cleans temp |
| `LearnedCorpusStore` | patterns DB | Optional feature path |
| `LocalStoreGraphAdapter` | query facade | Thin wrap |
| `KnowledgeStore` | convenience open | Legacy/helper path |

## Data flows

### A. Cold fact write/read

```
caller → StoreFact(predicate, args, type, priority)
       → encodeFactArgs → INSERT cold_storage ON CONFLICT UPDATE
caller → LoadFacts(predicate)
       → SELECT … ORDER BY priority
       → decodeFactArgs
       → UPDATE last_accessed, access_count++
```

### B. Semantic vector path

```
SetEmbeddingEngine → initVecIndex → go backfillVecIndex
StoreVectorWithEmbedding → Embed* → vectors + vec_index
VectorRecallSemantic → Embed query → ANN | brute | keyword
```

### C. World incremental update

```
world scan → UpdateWorldFilesAndFacts / ReplaceWorldFactsForFile
boot rehydrate → LoadAllWorldFacts(depth) → assert into kernel (caller)
```

### D. Reflection cycle (state machine-ish)

```
[idle] --SetReflectionConfig(enabled)+engine--> [worker started]
[worker] --ticker 45s--> processReflectionCycle
  ├─ list candidates (missing/stale embeddings)
  ├─ embed batch ≤32
  └─ Apply*EmbeddingUpdates
[worker] --stop / Close / engine nil--> [stopped]
```

### E. Archival lifecycle

```
[cold active]
   --ArchiveOldFacts(age, maxAccess)--> [archived]
   --RestoreArchivedFact--> [cold active]
[archived]
   --PurgeOldArchivedFacts(age)--> [deleted]
```

## Mutex / locking notes

- Most LocalStore methods take `mu.Lock` or `RLock`.
- Background backfill and reflection must coordinate with the same mutex patterns used in `vector_store.go` / `reflection_worker.go`.
- LearningStore locks its own `mu` around the DB map; per-shard `*sql.DB` also single-writer friendly via pragmas.

## Schema ownership

LocalStore.initialize creates tables; TraceStore.ensureSchema is defensive if table missing; LearningStore.initializeSchema per shard DB; ToolStore.initialize own schema. Migrations shared via `RunMigrations` for overlapping column sets (cold, archived, knowledge_atoms, prompt_atoms, reasoning_traces, learnings).

## Build-time architecture

```
cgo build
  └─ init_vec.go registers sqlite-vec Auto()

tags: sqlite_vec && cgo
  └─ defaultRequireVec=true → NewLocalStore fails without vec0

default / !sqlite_vec
  └─ defaultRequireVec=false → warn and continue without ANN
```
