# embedding — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/embedding/` (complete internal coverage)
> **Implementation: `internal/embedding/` — 6 non-test .go, 7 tests, 0 .mg**


## Package

`internal/embedding/`

## Exported types (sampled, up to 40)

| Type | Location |
|------|----------|
| `EmbeddingEngine` | `internal/embedding/engine.go:18` |
| `TaskTypeAwareEngine` | `internal/embedding/engine.go:33` |
| `TaskTypeBatchAwareEngine` | `internal/embedding/engine.go:40` |
| `HealthChecker` | `internal/embedding/engine.go:49` |
| `Config` | `internal/embedding/engine.go:60` |
| `SimilarityResult` | `internal/embedding/engine.go:213` |
| `GenAIEngine` | `internal/embedding/genai.go:35` |
| `OllamaEngine` | `internal/embedding/ollama.go:33` |
| `ContentType` | `internal/embedding/task_selector.go:14` |

## Exported functions/methods (sampled, up to 30)

| Symbol | Location |
|--------|----------|
| `DefaultConfig` | `internal/embedding/engine.go:77` |
| `NewEngine` | `internal/embedding/engine.go:92` |
| `FindTopK` | `internal/embedding/engine.go:146` |
| `NewGenAIEngine` | `internal/embedding/genai.go:42` |
| `Embed` | `internal/embedding/genai.go:89` |
| `EmbedWithTask` | `internal/embedding/genai.go:94` |
| `EmbedBatch` | `internal/embedding/genai.go:148` |
| `EmbedBatchWithTask` | `internal/embedding/genai.go:153` |
| `Dimensions` | `internal/embedding/genai.go:287` |
| `Name` | `internal/embedding/genai.go:292` |
| `Close` | `internal/embedding/genai.go:297` |
| `EmbedBatchJob` | `internal/embedding/genai.go:317` |
| `CosineSimilarity` | `internal/embedding/math_amd64.go:15` |
| `CosineSimilarity` | `internal/embedding/math_generic.go:14` |
| `NewOllamaEngine` | `internal/embedding/ollama.go:44` |
| `Embed` | `internal/embedding/ollama.go:81` |
| `EmbedBatch` | `internal/embedding/ollama.go:211` |
| `Dimensions` | `internal/embedding/ollama.go:249` |
| `Name` | `internal/embedding/ollama.go:254` |
| `Model` | `internal/embedding/ollama.go:259` |
| `HealthCheck` | `internal/embedding/ollama.go:268` |
| `EnsureModel` | `internal/embedding/ollama.go:301` |
| `SelectTaskType` | `internal/embedding/task_selector.go:36` |
| `DetectContentType` | `internal/embedding/task_selector.go:84` |
| `GetOptimalTaskType` | `internal/embedding/task_selector.go:183` |

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

This package: **Embedding engines (including Ollama) and vector generation**
