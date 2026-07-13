# store — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/store/` (complete internal coverage)
> **Implementation: `internal/store/` — 39 non-test .go, 44 tests, 0 .mg**


## 1. Purpose

Memory tiers and durable store implementations

## 2. Source paths (1:1)

| Path | Role |
|------|------|
| `internal/store/` | Primary implementation |
| `Docs/architecture/store/` | This full corpus |

## 3. Implementation Status

> Living code status — **not** pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
| Source package tree | Implemented | **90%** |
| Exported types (sampled) | Implemented | **80%** |
| Tests | Implemented | **90%** |
| Mangle local sources | N/A or global-only | **n/a** |
| Full architecture corpus | Implemented | **100%** |

**Overall (heuristic): 90%** as living package (39 src / 44 tests)

## 4. Public surface inventory

### Largest files

| Path | Lines |
|------|------:|
| `internal/store/vector_store.go` | 1009 | source |
| `internal/store/migrations.go` | 811 | source |
| `internal/store/trace_store.go` | 710 | source |
| `internal/store/local_core.go` | 689 | source |
| `internal/store/reflection_worker.go` | 651 | source |
| `internal/store/learned_store.go` | 571 | source |
| `internal/store/local_cold.go` | 544 | source |
| `internal/store/tool_cleanup.go` | 464 | source |
| `internal/store/embedded_store.go` | 444 | source |
| `internal/store/local_knowledge.go` | 426 | source |
| `internal/store/reflection_search.go` | 405 | source |
| `internal/store/learning.go` | 386 | source |
| `internal/store/tool_store.go` | 373 | source |
| `internal/store/vector_store_reembed.go` | 344 | source |
| `internal/store/local_prompt.go` | 334 | source |
| `internal/store/trace_reflection.go` | 324 | source |
| `internal/store/learning_reflection.go` | 317 | source |
| `internal/store/local_world.go` | 313 | source |
| `internal/store/local_verification.go` | 264 | source |
| `internal/store/vector_store_bruteforce.go` | 257 | source |

### Types (sampled)

| Type | Location |
|------|----------|
| `SemanticMatch` | `internal/store/embedded_store.go:19` |
| `EmbeddedCorpusStore` | `internal/store/embedded_store.go:32` |
| `LearnedPattern` | `internal/store/learned_store.go:19` |
| `LearnedCorpusStore` | `internal/store/learned_store.go:32` |
| `Learning` | `internal/store/learning.go:21` |
| `LearningStore` | `internal/store/learning.go:40` |
| `LearningCandidate` | `internal/store/learning_candidates.go:10` |
| `LearningEmbeddingCandidate` | `internal/store/learning_reflection.go:14` |
| `LearningEmbeddingUpdate` | `internal/store/learning_reflection.go:31` |
| `StoredFact` | `internal/store/local_cold.go:14` |
| `ArchivedFact` | `internal/store/local_cold.go:27` |
| `MaintenanceConfig` | `internal/store/local_cold.go:41` |
| `MaintenanceStats` | `internal/store/local_cold.go:50` |
| `LocalStore` | `internal/store/local_core.go:45` |
| `KnowledgeLink` | `internal/store/local_graph.go:15` |
| `LocalStoreGraphAdapter` | `internal/store/local_graph_query.go:13` |
| `KnowledgeAtom` | `internal/store/local_knowledge.go:18` |
| `KnowledgeStore` | `internal/store/local_knowledge.go:29` |
| `PromptAtom` | `internal/store/local_prompt.go:17` |
| `StoredReviewFinding` | `internal/store/local_review.go:13` |
| `VectorEntry` | `internal/store/local_vector.go:16` |
| `VerificationRecord` | `internal/store/local_verification.go:15` |
| `WorldFileMeta` | `internal/store/local_world.go:12` |
| `FileUpdates` | `internal/store/local_world.go:25` |
| `WorldFactInput` | `internal/store/local_world.go:29` |
| `MigrationResult` | `internal/store/migrations.go:28` |
| `Migration` | `internal/store/migrations.go:39` |
| `PragmaProfile` | `internal/store/pragmas.go:17` |
| `ReembedResult` | `internal/store/reembed_all.go:17` |
| `ReembedProgressFn` | `internal/store/reembed_all.go:28` |
| `TraceRecallHit` | `internal/store/reflection_search.go:15` |
| `LearningRecallHit` | `internal/store/reflection_search.go:28` |
| `CleanupConfig` | `internal/store/tool_cleanup.go:14` |
| `CleanupStats` | `internal/store/tool_cleanup.go:32` |
| `ToolStatsSummary` | `internal/store/tool_cleanup.go:40` |
| `LLMCleanupRecommendation` | `internal/store/tool_cleanup.go:264` |
| `ToolStore` | `internal/store/tool_store.go:25` |
| `ToolExecution` | `internal/store/tool_store.go:32` |
| `ToolStoreStats` | `internal/store/tool_store.go:52` |
| `TraceEmbeddingCandidate` | `internal/store/trace_reflection.go:14` |

### Functions (sampled)

| Symbol | Location |
|--------|----------|
| `NewEmbeddedCorpusStore` | `internal/store/embedded_store.go:41` |
| `Search` | `internal/store/embedded_store.go:115` |
| `SearchByPredicate` | `internal/store/embedded_store.go:194` |
| `SearchByCategory` | `internal/store/embedded_store.go:270` |
| `GetStats` | `internal/store/embedded_store.go:345` |
| `Close` | `internal/store/embedded_store.go:403` |
| `NewLearnedCorpusStore` | `internal/store/learned_store.go:45` |
| `AddPattern` | `internal/store/learned_store.go:156` |
| `Search` | `internal/store/learned_store.go:230` |
| `GetAllPatterns` | `internal/store/learned_store.go:316` |
| `GetPatternsByVerb` | `internal/store/learned_store.go:360` |
| `DecayConfidence` | `internal/store/learned_store.go:400` |
| `DeletePattern` | `internal/store/learned_store.go:447` |
| `GetStats` | `internal/store/learned_store.go:469` |
| `SetEmbeddingEngine` | `internal/store/learned_store.go:529` |
| `Close` | `internal/store/learned_store.go:555` |
| `NewLearningStore` | `internal/store/learning.go:51` |
| `Save` | `internal/store/learning.go:141` |
| `Load` | `internal/store/learning.go:178` |
| `LoadByPredicate` | `internal/store/learning.go:224` |
| `DecayConfidence` | `internal/store/learning.go:271` |
| `Delete` | `internal/store/learning.go:312` |
| `GetStats` | `internal/store/learning.go:333` |
| `Close` | `internal/store/learning.go:373` |
| `RecordLearningCandidate` | `internal/store/learning_candidates.go:23` |
| `ListLearningCandidates` | `internal/store/learning_candidates.go:60` |
| `ConfirmLearningCandidate` | `internal/store/learning_candidates.go:104` |
| `RejectLearningCandidate` | `internal/store/learning_candidates.go:109` |
| `ConfirmLearningCandidateMatch` | `internal/store/learning_candidates.go:114` |
| `RejectLearningCandidateMatch` | `internal/store/learning_candidates.go:119` |

## 5. Integration relevance

| Surface | Relevance |
|---------|-----------|
| Kernel | Related |
| VirtualStore | Consumer if effectful |
| Shards | Related |
| Prompt JIT | Optional |
| CLI | Related via `cmd/nerd` |
| Config | Reader |

## 6. Non-goals of this corpus revision

- Full prose rewrite of every function body
- Docs/Spec 18-file product templates (`spec-doc-sprint`)
- Implementing missing features (corpus-build implementation mode)
