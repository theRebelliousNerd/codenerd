# store — Implemented Spec (Deep-Dive)

> Last verified against codebase: **2026-07-13**  
> Status: Living Reference Document — **code-grounded full corpus**  
> Language: Go · Package: `codenerd/internal/store`  
> Mode: 1:1 with `internal/store/`  
> Scale: **~39** non-test `.go` · **~44** test files · **0** `.mg`  
> Flagship focus: **memory tiers** (vector, graph, cold/archival, session, world, traces, learnings, tools, corpora)

---

## 1. Purpose

`internal/store` is codeNERD’s **durable multi-tier memory substrate**. It does not decide actions (that is the Mangle kernel) and does not invent prose (that is the LLM). It **persists, retrieves, archives, embeds, and rehydrates** the facts and artifacts that give the agent long-horizon continuity:

| Responsibility | What “success” looks like |
|----------------|---------------------------|
| Multi-tier SQLite memory | One `knowledge.db` hosts vector, graph, cold, session, world, prompt, verification, and trace tables |
| Semantic recall | Real embeddings + optional sqlite-vec ANN, with brute-force and keyword fallbacks |
| Autopoietic learning | Per-shard `*_learnings.db` plus confidence decay and reflection re-embed |
| World model cache | Fast/deep AST projections survive restarts without full rescan |
| Tool / trace audit | Separate tool DB + reasoning traces for self-learning |
| Schema evolution | Column-additive migrations + versioned knowledge-base upgrades |

### North-star placement

```
user input → perception → user_intent → kernel next_action
  → VirtualStore / shards / tools
  → store (read/write durable memory)
  → articulation
```

Store is the **disk-backed half of VirtualStore’s knowledge surface**. Logic remains executive; store is the substrate that outlives a process.

---

## 2. Implementation status

| Component | Status | Evidence |
|-----------|--------|----------|
| `LocalStore` core + schema init | **Implemented** | `local_core.go` |
| Vector tier (Shard B) + semantic path | **Implemented** | `local_vector.go`, `vector_store.go`, `vector_store_bruteforce.go` |
| Knowledge graph (Shard C) | **Implemented** | `local_graph.go`, `local_graph_query.go` |
| Cold + archival (Shard D) | **Implemented** | `local_cold.go` |
| Session / activation / compressed state | **Implemented** | `local_session.go` |
| World model cache | **Implemented** | `local_world.go` |
| Knowledge atoms + semantic bridge | **Implemented** | `local_knowledge.go` |
| Prompt atoms (JIT corpus on disk) | **Implemented** | `local_prompt.go` |
| Verification + reasoning traces | **Implemented** | `local_verification.go`, `trace_store.go` |
| Learning candidates (taxonomy staging) | **Implemented** | `learning_candidates.go` |
| Review findings | **Implemented** | `local_review.go` |
| `LearningStore` (autopoiesis §8.3) | **Implemented** | `learning.go`, `learning_reflection.go` |
| `ToolStore` + cleanup strategies | **Implemented** | `tool_store.go`, `tool_cleanup.go` |
| Embedded intent corpus (read-only) | **Implemented** | `embedded_store.go` |
| Learned pattern corpus | **Implemented** | `learned_store.go` |
| Reflection worker (async re-embed) | **Implemented** | `reflection_worker.go`, `*_reembed.go` |
| Migrations + pragma profiles | **Implemented** | `migrations.go`, `pragmas.go` → `sqlpragmas` |
| sqlite-vec build tags | **Implemented** | `init_vec.go`, `vec_support_*.go` |
| Fact arg codec (typed Mangle args) | **Implemented** | `fact_codec.go` |
| Force re-embed across DBs | **Implemented** | `reembed_all.go` |
| Mangle local sources | **N/A** | 0 `.mg` in package |
| Cross-DB transactional coordination | **Partial / absent** | Separate DBs; no 2PC |
| Unified store interface for all tiers | **Partial** | Multiple concrete types, not one interface |

**Overall:** living production package — **not** pre-implementation. Heuristic completeness **~90%** for declared surfaces; densest remaining gaps are operational (ANN drift hygiene, re-embed ops UX, multi-DB consistency).

---

## 3. Source inventory

### 3.1 Modular layout (`local.go` index)

`local.go` is a package map only. Implementation is split so each file stays under ~1000 lines:

| File | Role |
|------|------|
| `local_core.go` | `LocalStore`, `NewLocalStore`, schema create, stats, vec probe |
| `local_world.go` | World file/fact cache (fast + deep) |
| `local_vector.go` | Keyword vector CRUD (`StoreVector`, `VectorRecall`) |
| `local_graph.go` | Graph edges, BFS path, kernel hydrate |
| `local_cold.go` | Cold facts, archive, purge, maintenance |
| `local_session.go` | Activation log, session turns, compressed states |
| `local_verification.go` | Task verifications + trace facades |
| `local_knowledge.go` | Knowledge atoms + dual-write semantic bridge |
| `local_prompt.go` | Prompt atom CRUD for JIT compiler |
| `local_review.go` | Review findings |

### 3.2 Largest non-test sources (approx lines)

| Path | Lines | Purpose |
|------|------:|---------|
| `vector_store.go` | ~1009 | Embeddings, batch store, semantic recall, ANN path |
| `migrations.go` | ~811 | Column migrations + versioned KB upgrade |
| `trace_store.go` | ~710 | Reasoning-trace persistence / queries |
| `local_core.go` | ~689 | Core LocalStore + full table create |
| `reflection_worker.go` | ~651 | Background descriptor/embedding refresh |
| `learned_store.go` | ~571 | Learned pattern corpus + vec search |
| `local_cold.go` | ~544 | Cold/archival lifecycle |
| `tool_cleanup.go` | ~464 | Tool retention policies |
| `embedded_store.go` | ~444 | Baked-in intent corpus |
| `local_knowledge.go` | ~426 | Knowledge atoms |
| `reflection_search.go` | ~405 | Trace/learning recall by embed or lexical |
| `learning.go` | ~386 | Per-shard learnings DBs |
| `tool_store.go` | ~373 | Tool execution journal |
| `vector_store_reembed.go` | ~344 | Vector re-embed helpers |
| `local_prompt.go` | ~334 | Prompt atoms |
| `trace_reflection.go` | ~324 | Trace embedding candidates/updates |
| `learning_reflection.go` | ~317 | Learning embedding backlog |
| `local_world.go` | ~313 | World model tables API |
| `local_verification.go` | ~264 | Verification + LocalStore→TraceStore bridge |
| `vector_store_bruteforce.go` | ~257 | Cosine brute-force search |

### 3.3 Supporting / build-tag files

| Path | Role |
|------|------|
| `init_sqlite.go` | Blank import `mattn/go-sqlite3` |
| `init_vec.go` | `//go:build cgo` — registers sqlite-vec via `vec.Auto()` |
| `vec_support_enabled.go` | `//go:build sqlite_vec && cgo` → `defaultRequireVec = true` |
| `vec_support_disabled.go` | Default builds → `defaultRequireVec = false` |
| `pragmas.go` | Re-exports `sqlpragmas` profiles (`ProfileHot`, `ProfileBulkBuild`, …) |
| `indexes.go` | Conditional index ensure (esp. reasoning_traces) |
| `fact_codec.go` | Typed encode/decode for cold fact args |
| `vector_utils.go` | Blob/float helpers for vec paths |
| `reflection_utils.go` | Shared reflection constants/helpers |
| `prompt_reembed.go` / `reflection_reembed.go` | Force re-embed surfaces |

---

## 4. Memory tiers (deep dive)

This is the package’s conceptual spine. Comments in `local_core.go` name **Shards B/C/D** (vector / graph / cold). Runtime practice extends far beyond those three labels.

### 4.1 Tier map

```
┌──────────────────────────────────────────────────────────────────────────┐
│                     LocalStore  (.nerd/knowledge.db)                     │
│  ┌─────────────┐ ┌──────────────┐ ┌─────────────┐ ┌──────────────────┐  │
│  │ B: vectors  │ │ C: knowledge │ │ D: cold_    │ │ archival:        │  │
│  │  + vec_index│ │    _graph    │ │    storage  │ │  archived_facts  │  │
│  └─────────────┘ └──────────────┘ └─────────────┘ └──────────────────┘  │
│  ┌─────────────┐ ┌──────────────┐ ┌─────────────┐ ┌──────────────────┐  │
│  │ session_    │ │ compressed_  │ │ activation_ │ │ task_            │  │
│  │  history    │ │  states     │ │  log        │ │  verifications   │  │
│  └─────────────┘ └──────────────┘ └─────────────┘ └──────────────────┘  │
│  ┌─────────────┐ ┌──────────────┐ ┌─────────────┐ ┌──────────────────┐  │
│  │ world_files │ │ world_facts  │ │ knowledge_  │ │ prompt_atoms     │  │
│  │             │ │ fast|deep    │ │  atoms      │ │ (JIT on disk)    │  │
│  └─────────────┘ └──────────────┘ └─────────────┘ └──────────────────┘  │
│  ┌─────────────┐ ┌──────────────┐ ┌─────────────┐                        │
│  │ reasoning_  │ │ learning_    │ │ review_     │                        │
│  │  traces     │ │  candidates  │ │  findings   │                        │
│  └─────────────┘ └──────────────┘ └─────────────┘                        │
└──────────────────────────────────────────────────────────────────────────┘

Separate DBs (same package, different constructors)
  LearningStore → .nerd/shards/<shard>_learnings.db
  ToolStore     → .nerd/tools.db
  EmbeddedCorpusStore → temp extract of go:embed intent_corpus.db (RO)
  LearnedCorpusStore  → path-configured learned patterns DB
```

### 4.2 Hot path concurrency model

Every `LocalStore` holds:

- `*sql.DB` with **`SetMaxOpenConns(1)`** (SQLite writer safety)
- `sync.RWMutex` around most public methods
- Optional `embedding.EmbeddingEngine`
- `vectorExt bool` from live probe (`CREATE VIRTUAL TABLE … USING vec0`)
- `requireVec` from build tags
- Embedded `*TraceStore` sharing the same `*sql.DB`
- Reflection worker channels when config enables background re-embed

`ApplyDefaultPragmas(db, ProfileHot)` runs at open for writable DBs; embedded corpus uses `ProfileReadOnly`.

### 4.3 Shard B — Vector / associative memory

**Tables**

- `vectors(id, content, embedding TEXT/JSON, metadata, created_at)`
- Optional `vec_index` virtual table (sqlite-vec) when extension present

**Write paths**

| API | Behavior |
|-----|----------|
| `StoreVector` | Keyword-era path; metadata JSON; no engine required (`local_vector.go`) |
| `StoreVectorWithEmbedding` | Embed via engine + task type; dual-write `vectors` and `vec_index` |
| `StoreVectorBatchWithEmbedding` | Batch embed (uniform task optimized); transactional insert |

**Read paths**

| API | Behavior |
|-----|----------|
| `VectorRecall` | Keyword / LIKE-style recall |
| `VectorRecallSemantic` | Embed query → ANN if `vectorExt` else brute cosine |
| `VectorRecallSemanticByPaths` | Metadata path filter |
| `VectorRecallSemanticFiltered` | Arbitrary metadata key/value filter |

**ANN drift pathology (documented in code):** if `vectors` row lands but `vec_index` insert fails, ANN returns empty while brute-force still finds content. Inserts **must** check `LastInsertId` and vec Exec errors; failures are **warned**, not silent.

**Predicate vector hygiene:** on init, `dedupePredicateVectors` + partial unique index on `content` where `metadata` looks like `"kind":"predicate"`.

**Fallback ladder**

```
engine set + vec0 ok  → ANN (vectorRecallVec)
engine set + no vec0  → brute-force cosine
engine nil            → keyword VectorRecall
```

### 4.4 Shard C — Knowledge graph

**Table:** `knowledge_graph(entity_a, relation, entity_b, weight, metadata)` UNIQUE on triple.

**API**

- `StoreLink` — upsert edge
- `QueryLinks(entity, direction)` — out / in / both
- `TraversePath(from, to, maxDepth)` — BFS path of edges
- `HydrateKnowledgeGraph(assertFunc)` — emit durable edges into kernel as facts
- `LocalStoreGraphAdapter.QueryGraph` — map-style adapter for VirtualStore/system factory

### 4.5 Shard D — Cold storage + archival

**Tables**

- `cold_storage` — active durable facts with `last_accessed`, `access_count`, priority
- `archived_facts` — demoted cold rows with `archived_at`

**Codec:** `fact_codec.go` preserves typed args (`nil`, `atom`→`types.MangleAtom`, string/int64/float64/bool) as tagged JSON so Mangle atoms round-trip.

**Lifecycle**

```
StoreFact ──► cold_storage (ON CONFLICT update type/priority)
LoadFacts ──► SELECT + bump last_accessed / access_count
ArchiveOldFacts(olderThanDays, maxAccessCount) ──► move cold → archived
RestoreArchivedFact ──► archived → cold
PurgeOldArchivedFacts ──► hard delete archived
MaintenanceCleanup ──► archive + purge + activation log clean + optional VACUUM
```

This is **access-tracked durable logic memory**, not a generic K/V cache.

### 4.6 Session / activation / compressed state

| Table | Role |
|-------|------|
| `activation_log` | Spreading-activation scores for facts |
| `session_history` | Per-turn user/intent/response/atoms; UNIQUE(session_id, turn_number) for idempotent sync |
| `compressed_states` | Semantic compressor snapshots for infinite-context rehydration |

APIs: `LogActivation`, `GetRecentActivations`, `StoreSessionTurn`, `GetSessionHistory`, `StoreCompressedState`, `LoadLatestCompressedState` (`local_session.go`). Consumers include `internal/context` compressor and session persistence paths.

### 4.7 World model cache

| Table | Role |
|-------|------|
| `world_files` | Path primary key + lang/size/modtime/hash/fingerprint |
| `world_facts` | Per `(path, depth, predicate, args)`; depth `fast` \| `deep` |

APIs: upsert/delete files, replace facts for file, load by file or all for depth, batch `UpdateWorldFilesAndFacts`, bulk delete. Used by `internal/world` (`persist.go`, scans) so boot and incremental scan skip unchanged files.

### 4.8 Knowledge atoms (Type-3 / agent KB)

**Table:** `knowledge_atoms(concept, content, confidence, content_hash, source, tags, …)`

- Exact: `StoreKnowledgeAtom`, `GetKnowledgeAtoms`, prefix queries
- Semantic dual-write: `StoreKnowledgeAtomWithEmbedding` also writes `vectors` with `content_type=knowledge_atom`
- Recall: `SearchKnowledgeAtomsSemantic` → filtered vector recall
- Init backfills missing `content_hash` via `ensureContentHashes`

`KnowledgeStore` is a thin path-based constructor wrapper around the same idea for standalone KB open.

### 4.9 Prompt atoms (disk side of JIT)

**Table:** `prompt_atoms` with polymorphism (`content` / `content_concise` / `content_min`), selector JSON columns (modes, phases, verbs, languages, …), embedding BLOB + task, priority/mandatory/exclusive/depends/conflicts.

CRUD in `local_prompt.go`. Re-embed: `ReembedAllPromptAtomsForce`. Used by `internal/prompt` compiler / DB-backed atom load — **disk persistence for prompt atoms**, not compilation policy (policy lives in `internal/prompt`).

### 4.10 Verification + reasoning traces

**Tables**

- `task_verifications` — retry-loop learning (success, confidence, quality_violations, corrective_action, …)
- `reasoning_traces` — full shard LLM interaction capture + optional reflection embedding fields

`TraceStore` owns durable trace ops; `LocalStore` wraps with `StoreReasoningTrace(any)` (reflection-friendly) and query facades. Wired from `internal/system/factory.go` via `createTraceStoreAdapter` + `perception.NewTracingLLMClient`.

Reflection columns (`summary_descriptor`, `embedding*`) feed `reflection_worker` and `RecallTracesByEmbedding` / lexical variants.

### 4.11 Learning candidates + review findings

- `learning_candidates` — staged taxonomy learnings (`pending` / confirm / reject); perception taxonomy can promote them
- `review_findings` — static review history for analysis

### 4.12 LearningStore (Autopoiesis §8.3)

**Not** inside `knowledge.db`. One SQLite file per shard type under `.nerd/shards/<shard>_learnings.db`.

| Concern | API |
|---------|-----|
| Persist learning | `Save(shardType, predicate, args, sourceCampaign)` |
| Load | `Load`, `LoadByPredicate` |
| Confidence | `DecayConfidence` |
| Semantic reflection | candidate list + `ApplyLearningEmbeddingUpdates` |
| Recall | `RecallLearningsByEmbedding` / `RecallLearningsLexical` |

Confidence decay is first-class: learnings are **soft**, not permanent gospel.

### 4.13 ToolStore

**Path:** `.nerd/tools.db` (separate to avoid bloating knowledge base).

Stores full tool I/O (no truncation in `Result`), duration, usefulness score, reference counts. Cleanup (`tool_cleanup.go`): runtime-hours retention, size FIFO, optional LLM-smart cleanup recommendations.

### 4.14 Embedded + learned corpora

| Type | Mutability | Role |
|------|------------|------|
| `EmbeddedCorpusStore` | Read-only | Baked intent classification vectors from `defaults.IntentCorpusDB` extracted to temp file |
| `LearnedCorpusStore` | Read-write | Runtime-learned intent/pattern rows with vec search, confidence decay |

Both return `SemanticMatch` (predicate/verb/target/category/similarity).

### 4.15 Cross-DB re-embed operations

`ReembedAllDBsForce` walks roots for `*.db`, opens as LocalStore where possible, re-embeds vectors / prompt atoms / traces, then learning stores under discovered shard roots. Progress callback for CLI/ops tooling.

---

## 5. Schema & migrations

### 5.1 Init order (`LocalStore.initialize`)

1. CREATE base tables (vectors, graph, world, cold, archived, activation, session, compressed, verification, traces, review, learning_candidates, prompt_atoms, knowledge_atoms)
2. `RunMigrations` — additive `ALTER TABLE … ADD COLUMN` for existing DBs (`pendingMigrations`)
3. Ensure reasoning-trace indexes (only if columns exist)
4. Create indexes that depend on migrated columns
5. Dedupe predicate vectors + unique partial index

### 5.2 Versioned KB migration

`CurrentSchemaVersion = 4`:

| Version | Meaning |
|--------:|---------|
| 1 | Basic `knowledge_atoms` |
| 2 | Embedding column |
| 3 | `vec_index` ANN virtual table |
| 4 | `content_hash` dedupe |

`RunAllMigrations` can backup and upgrade knowledge bases (hash computation included in `MigrationResult`).

### 5.3 Pragmas

Implementation lives in `internal/sqlpragmas`; `store` re-exports for historical call sites. Profiles: Hot, BulkBuild, Query, ReadOnly.

---

## 6. Embedding & reflection subsystem

```
SetEmbeddingEngine(engine)
  ├─ initVecIndex(dims)
  ├─ background backfillVecIndex (non-blocking boot)
  └─ startReflectionWorker if reflectionCfg.Enabled

SetReflectionConfig(cfg)
  └─ start/stop worker (45s interval, batch 32)

processReflectionCycle
  ├─ trace descriptor + embedding backlog
  └─ (LearningStore has parallel reflection surfaces)
```

Task typing uses `embedding.GetOptimalTaskType` / `TaskTypeAwareEngine` / batch variants so document vs query embeddings stay aligned with the embedding package contract.

---

## 7. Control-flow diagrams

### 7.1 Boot wiring (system factory)

```
system.Factory boot
  │
  ├─ NewLocalStore(.nerd/knowledge.db)
  │     ├─ schema + migrations
  │     ├─ detectVecExtension
  │     └─ NewTraceStore(shared db)
  │
  ├─ NewLearningStore(.nerd/shards)
  │
  ├─ createTraceStoreAdapter(localDB) → TracingLLMClient
  │
  ├─ embedding engine init
  │     ├─ localDB.SetEmbeddingEngine
  │     ├─ localDB.SetReflectionConfig
  │     ├─ learningStore.SetEmbeddingEngine
  │     └─ learningStore.SetReflectionConfig
  │
  └─ NewLocalStoreGraphAdapter(localDB) → graph query surface
```

### 7.2 Semantic knowledge write

```
StoreKnowledgeAtomWithEmbedding
  ├─ StoreKnowledgeAtom → knowledge_atoms (+ content_hash)
  └─ if engine: StoreVectorWithEmbedding
        ├─ EmbedWithTask / Embed
        ├─ INSERT vectors
        └─ INSERT vec_index (if vectorExt)  // warn on drift
```

### 7.3 Cold maintenance

```
MaintenanceCleanup(cfg)
  ├─ ArchiveOldFacts
  ├─ PurgeOldArchivedFacts
  ├─ clean activation_log older than N days
  └─ optional VACUUM
```

---

## 8. Integration map (consumers)

| Consumer package | How store is used |
|------------------|-------------------|
| `internal/system` | Constructs LocalStore + LearningStore; wires embeddings, graph adapter, trace adapter |
| `internal/core` | VirtualStore holds `*store.LocalStore` and `*store.LearningStore` |
| `internal/world` | World file/fact persistence for scans |
| `internal/prompt` | Prompt atoms / predicate selection DB access |
| `internal/init` | Shared KB, strategic docs, agents knowledge, validation |
| `internal/perception` | Taxonomy store over LocalStore; semantic classifier |
| `internal/campaign` | Specialist knowledge, intelligence, document ingest |
| `internal/context` | Compressed state / session bridge |
| `internal/verification` | Verification history |
| `internal/shards/system` | Base shard access to store |
| `cmd/nerd`, `cmd/query-kb`, tools | Ops, re-embed, KB query |

Store does **not** import kernel or Mangle rule files. Upstream packages pass assert funcs / adapters.

---

## 9. Public surface (summary)

### Core constructors

- `NewLocalStore(path)`
- `NewTraceStore(db, path)`
- `NewLearningStore(basePath)`
- `NewToolStore(dbPath)`
- `NewKnowledgeStore(dbPath)`
- `NewEmbeddedCorpusStore()`
- `NewLearnedCorpusStore(dbPath, engine)`
- `NewLocalStoreGraphAdapter(store)`

### Notable free functions

- `CosineSimilarity`
- `RunMigrations` / `RunAllMigrations`
- `ReembedAllDBsForce`
- `ApplyDefaultPragmas` (re-export)
- `encodeFactArgs` / `decodeFactArgs` (package-private helpers)

Deep type tables live in `06-PUBLIC-API-AND-TYPES.md`.

---

## 10. Testing posture

Broad unit + integration coverage:

- Cold storage suite (`cold_storage_integration_test.go`)
- Graph integration + benchmarks
- Session integration
- Vector e2e, boundary, batch, brute, search, reembed tests
- Trace store unit + integration
- Migrations + benchmarks
- Tool cleanup / tool store
- Reflection worker / search / reembed
- Learning candidates, fact codec, archival

Command:

```powershell
go test ./internal/store/...
```

With sqlite-vec binary builds, use CGO flags from root `Agents.md`.

---

## 11. Observability

- Category: `logging.CategoryStore`
- Helpers: `logging.Store`, `logging.StoreDebug`, `logging.StartTimer(..., "OpName")`
- Hot ops timed: `NewLocalStore`, migrations, store/load fact, vector semantic paths, embedded corpus search
- Warnings for non-fatal: index create failures, ANN drift, content-hash backfill, vec backfill duration

See `11-OBSERVABILITY.md`.

---

## 12. Failure modes (pointer)

| Mode | Mitigation in code |
|------|--------------------|
| sqlite-vec missing | Optional unless `sqlite_vec` tag; warn / fail-fast |
| ANN drift | Explicit warn on vec insert failure |
| Embedding engine nil | Keyword fallback |
| Single-writer SQLite | MaxOpenConns=1 + package mutex |
| Migration on old DB | Additive columns; skip missing tables |
| Reflection backlog growth | Worker batching; force re-embed ops |
| Knowledge.db vs tools.db split | Intentional isolation; no cross-DB TX |

Full catalog: `12-FAILURE-MODES.md`.

---

## 13. Gaps (honest)

1. **No unified Store interface** covering Local/Learning/Tool/Corpus — consumers take concrete types.
2. **ANN drift** is logged, not auto-healed (no continuous reconcile job beyond backfill on engine set).
3. **Multi-process** access not supported (single-conn design).
4. **GetStats** covers a subset of tables (not traces/prompt_atoms/reviews).
5. **Mangle safety** is not enforced here — store will persist whatever callers write; `permitted(...)` is kernel-side.
6. **Legacy naming** “Shards B/C/D” in comments vs expanded table set can confuse newcomers (documented here).

Non-gaps: package is fully implemented for its stated memory roles; absence of local `.mg` is correct.

---

## 14. Non-goals of this corpus

- Implementing features or changing Go
- Product Spec templates under `Docs/Spec/`
- Documenting embedding engine internals (`internal/embedding/`)
- Documenting VirtualStore policy routing (`internal/core/`)

---

## 15. Related architecture corpora

| Corpus | Relationship |
|--------|--------------|
| `Docs/architecture/embedding/` | Engines store calls |
| `Docs/architecture/sqlpragmas/` | Pragma leaf |
| `Docs/architecture/world/` | World scan → world_* tables |
| `Docs/architecture/prompt/` | JIT atoms; disk side here |
| `Docs/architecture/core/` | VirtualStore owns store handles |
| `Docs/architecture/session/` | Session loop that writes turns/state |
| `Docs/architecture/persist/` | Factsnap helpers (adjacent, not this package) |

---

*End of IMPLEMENTED_SPEC — store memory tiers living reference (2026-07-13).*
