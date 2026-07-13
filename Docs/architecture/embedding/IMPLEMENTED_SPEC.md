# embedding — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/embedding/` (complete internal coverage)
> **Implementation: `internal/embedding/` — 6 non-test .go, 7 tests, 0 .mg**


## 1. Purpose

Embedding engines (including Ollama) and vector generation

## 2. Source paths (1:1)

| Path | Role |
|------|------|
| `internal/embedding/` | Primary implementation |
| `Docs/architecture/embedding/` | This full corpus |

## 3. Implementation Status

> Living code status — **not** pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
| Source package tree | Implemented | **90%** |
| Exported types (sampled) | Implemented | **80%** |
| Tests | Implemented | **90%** |
| Mangle local sources | N/A or global-only | **n/a** |
| Full architecture corpus | Implemented | **100%** |

**Overall (heuristic): 90%** as living package (6 src / 7 tests)

## 4. Public surface inventory

### Largest files

| Path | Lines |
|------|------:|
| `internal/embedding/ollama.go` | 611 | source |
| `internal/embedding/genai.go` | 373 | source |
| `internal/embedding/engine.go` | 216 | source |
| `internal/embedding/task_selector.go` | 198 | source |
| `internal/embedding/math_amd64.go` | 57 | source |
| `internal/embedding/math_generic.go` | 37 | source |

### Types (sampled)

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

### Functions (sampled)

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
