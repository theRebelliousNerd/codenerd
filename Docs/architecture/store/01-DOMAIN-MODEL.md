# store — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/store/` (complete internal coverage)
> **Implementation: `internal/store/` — 39 non-test .go, 44 tests, 0 .mg**


## Package

`internal/store/`

## Exported types (sampled, up to 40)

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

## Exported functions/methods (sampled, up to 30)

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

## Mangle surface

| Artifact | Count |
|----------|------:|
| Package-local `.mg` | 0 |

| Path | Lines |
|------|------:|
| — | 0 |

Global kernel schemas/policy (when this package participates):

- `internal/core/defaults/schemas.mg`
- `internal/core/defaults/policy/`

## Fact-flow placement

```
user input → perception → user_intent → kernel(core/mangle) → next_action
  → VirtualStore / shards / tools → articulation
```

This package: **Memory tiers and durable store implementations**
