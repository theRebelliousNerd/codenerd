# store — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/store/` (39 non-test .go, 44 tests, 0 .mg)**


## Source package

`internal/store/`

## Exported / primary types (sampled)

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

## Planned vs actual Mangle surface

| Artifact | Count | Notes |
|----------|-------|-------|
| `.mg` files under package | 0 | See inventory |
| Core schemas (global) | shared | `internal/core/defaults/schemas.mg` when kernel-touching |
| Policy modules (global) | shared | `internal/core/defaults/policy/` |

### Package-local Mangle inventory (top)

| Path | Lines |
|------|-------|
| — | 0 |

## Fact-flow placement

```
user input → perception → user_intent → kernel(core/mangle) → next_action
  → VirtualStore / shards / tools → articulation
```

This package's position: **Memory tiers / persistence stores**

## Data & control concepts

- Primary language surface: Go under `internal/store/`
- Logic surface: Mangle where listed above or via kernel defaults
- External effects: VirtualStore / tools / CLI depending on package
