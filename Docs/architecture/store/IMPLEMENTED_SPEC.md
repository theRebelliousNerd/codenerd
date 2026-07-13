# store — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/store/` (39 non-test .go, 44 tests, 0 .mg)**


## 1. Purpose

Memory tiers / persistence stores

## 2. Source paths

| Path | Role |
|------|------|
| `internal/store/` | Primary implementation |
| `Docs/architecture/store/` | This corpus |

## 3. Implementation Status

> Status reflects **living code**, not pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
| Source package tree | Implemented | **90%** |
| Exported types (sampled) | Implemented | **80%** |
| Tests | Implemented | **90%** |
| Mangle local sources | N/A or global | **n/a** |
| Docs corpus (this) | Implemented | **100%** |

**Overall (heuristic): 90% complete as living package**

## 4. Public surface (inventory-driven)

### Largest implementation files

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

### Sampled types

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

## 5. Integration points (codeNERD spine)

| Surface | Relevance |
|---------|-----------|
| Kernel / facts | Consumer/producer |
| VirtualStore | User if effectful |
| Shards | Related |
| Prompt JIT | Optional |
| CLI | Invokes via cmd/nerd |
| Config | Reads global config |

## 6. Non-goals for this corpus revision

- Full behavioral prose rewrite of every function
- Filling Docs/Spec 18-file product templates (use spec-doc-sprint)
- Implementing missing runtime features (use corpus-build implementation mode)
