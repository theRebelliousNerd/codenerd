# store — Public API and Types

> Last verified: **2026-08-09**
> Package: `codenerd/internal/store`  
> Note: Go exports all capitalized symbols; this lists **architecturally meaningful** ones with file refs.

## Constructors

| Symbol | File | Purpose |
|--------|------|---------|
| `NewLocalStore(path)` | `local_core.go` | Open/create knowledge.db multi-tier store |
| `NewTraceStore(db, path)` | `trace_store.go` | Trace layer on shared or dedicated DB |
| `NewLearningStore(basePath)` | `learning.go` | Per-shard learnings root |
| `NewToolStore(dbPath)` | `tool_store.go` | Tool execution journal |
| `NewKnowledgeStore(dbPath)` | `local_knowledge.go` | Knowledge-oriented open helper |
| `NewEmbeddedCorpusStore()` | `embedded_store.go` | RO baked intent corpus |
| `NewLearnedCorpusStore(path, engine)` | `learned_store.go` | RW learned patterns |
| `NewLocalStoreGraphAdapter(store)` | `local_graph_query.go` | Graph query adapter |

## Core LocalStore configuration

| Symbol | File |
|--------|------|
| `(*LocalStore) SetEmbeddingEngine` | `vector_store.go` |
| `(*LocalStore) SetReflectionConfig` | `reflection_worker.go` |
| `(*LocalStore) GetTraceStore` | `local_core.go` |
| `(*LocalStore) GetDB` | `local_core.go` |
| `(*LocalStore) Close` | `local_core.go` |
| `(*LocalStore) GetStats` | `local_core.go` |

## Types — LocalStore domain

| Type | File | Notes |
|------|------|-------|
| `LocalStore` | `local_core.go` | Central multi-tier store |
| `StoredFact` / `ArchivedFact` | `local_cold.go` | Cold tier rows |
| `MaintenanceConfig` / `MaintenanceStats` | `local_cold.go` | Cleanup knobs/results |
| `KnowledgeLink` | `local_graph.go` | Graph edge |
| `LocalStoreGraphAdapter` | `local_graph_query.go` | Adapter |
| `VectorEntry` | `local_vector.go` | Vector hit |
| `KnowledgeAtom` / `KnowledgeStore` | `local_knowledge.go` | Concept atoms |
| `PromptAtom` | `local_prompt.go` | JIT atom disk form |
| `StoredReviewFinding` | `local_review.go` | Review row |
| `VerificationRecord` | `local_verification.go` | Task verification |
| `WorldFileMeta` / `FileUpdates` / `WorldFactInput` | `local_world.go` | World cache |
| `LearningCandidate` | `learning_candidates.go` | Taxonomy staging |

## Types — traces & reflection

| Type | File |
|------|------|
| `TraceStore` | `trace_store.go` |
| `ReasoningTrace` | `trace_store.go` |
| `TraceTypeStats` | `trace_store.go` | Exact one-shard total/success/fail/mean-duration aggregate |
| `TraceEmbeddingCandidate` / `TraceEmbeddingUpdate` | `trace_reflection.go` |
| `TraceRecallHit` / `LearningRecallHit` | `reflection_search.go` |
| `LearningEmbeddingCandidate` / `LearningEmbeddingUpdate` | `learning_reflection.go` |

## Types — learning / tools / corpora

| Type | File |
|------|------|
| `Learning` / `LearningStore` | `learning.go` |
| `ToolStore` / `ToolExecution` / `ToolStoreStats` | `tool_store.go` |
| `CleanupConfig` / `CleanupStats` / `ToolStatsSummary` | `tool_cleanup.go` |
| `LLMCleanupRecommendation` | `tool_cleanup.go` |
| `SemanticMatch` | `embedded_store.go` (shared shape) |
| `EmbeddedCorpusStore` | `embedded_store.go` |
| `LearnedPattern` / `LearnedCorpusStore` | `learned_store.go` |

## Types — migrations / ops

| Type | File |
|------|------|
| `Migration` / `MigrationResult` | `migrations.go` |
| `CurrentSchemaVersion` | `migrations.go` (const = 4) |
| `ReembedResult` / `ReembedProgressFn` | `reembed_all.go` |
| `PragmaProfile` + `Profile*` constants | `pragmas.go` |

## Cold / graph / world / session APIs (selected)

| Group | Methods (file) |
|-------|----------------|
| Cold | `StoreFact`, `LoadFacts`, `LoadAllFacts`, `DeleteFact`, `ArchiveOldFacts`, `GetArchivedFacts`, `RestoreArchivedFact`, `PurgeOldArchivedFacts`, `MaintenanceCleanup` (`local_cold.go`) |
| Graph | `StoreLink`, `QueryLinks`, `TraversePath`, `HydrateKnowledgeGraph` (`local_graph.go`) |
| Vector | `StoreVector`, `VectorRecall` (`local_vector.go`); `StoreVectorWithEmbedding`, `StoreVectorBatchWithEmbedding`, `VectorRecallSemantic*` (`vector_store.go`) |
| Session | `LogActivation`, `GetRecentActivations`, `StoreSessionTurn`, `GetSessionHistory`, `StoreCompressedState`, `LoadLatestCompressedState` (`local_session.go`) |
| Trace statistics | `GetTraceStats` for broad observability; `GetTraceStatsForType` for exact policy-facing one-shard values (`trace_store.go`, facade in `local_verification.go`) |
| World | `UpsertWorldFile`, `DeleteWorldFile`, `ReplaceWorldFactsForFile`, `LoadWorldFactsForFile`, `LoadAllWorldFacts`, `UpdateWorldFilesAndFacts`, `DeleteWorldFiles` (`local_world.go`) |
| Knowledge | `StoreKnowledgeAtom`, `GetKnowledgeAtoms*`, `StoreKnowledgeAtomWithEmbedding`, `SearchKnowledgeAtomsSemantic` (`local_knowledge.go`) |
| Prompt | `StorePromptAtom`, `LoadPromptAtoms*`, `GetPromptAtom`, `DeletePromptAtom` (`local_prompt.go`) |
| Learning candidates | `RecordLearningCandidate`, `ListLearningCandidates`, `Confirm*`, `Reject*` (`learning_candidates.go`) |

## Free functions

| Symbol | File |
|--------|------|
| `CosineSimilarity` | `local_core.go` |
| `RunMigrations` / `RunAllMigrations` | `migrations.go` |
| `ReembedAllDBsForce` | `reembed_all.go` |
| `ApplyDefaultPragmas` | `pragmas.go` (var re-export) |

## Unexported but important

| Symbol | Why it matters |
|--------|----------------|
| `encodeFactArgs` / `decodeFactArgs` | Contract for cold storage fidelity |
| `defaultRequireVec` | Build-tag gate for ANN requirement |
| `detectVecExtension` | Runtime capability probe |
| `processReflectionCycle` | Background maintenance heart |
| `pendingMigrations` | Source of truth for additive columns |
