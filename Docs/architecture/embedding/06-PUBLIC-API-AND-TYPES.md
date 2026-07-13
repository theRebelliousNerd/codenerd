# 06 — Public API and Types: embedding

> Last verified against codebase: 2026-07-13  
> Exported symbols that matter, with file references

## 1. Interfaces

### `EmbeddingEngine` — `engine.go`

```go
type EmbeddingEngine interface {
    Embed(ctx context.Context, text string) ([]float32, error)
    EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
    Dimensions() int
    Name() string
}
```

| Method | Semantics |
|--------|-----------|
| `Embed` | One text → one finite, non-empty vector; malformed provider output is an error |
| `EmbedBatch` | N texts → exactly N uniform-width vectors in order; empty → `nil, nil` |
| `Dimensions` | Declared width of vectors (not necessarily probed) |
| `Name` | Stable-ish identifier for logs/stats (`provider:model`) |

### `TaskTypeAwareEngine` — `engine.go`

```go
type TaskTypeAwareEngine interface {
    EmbeddingEngine
    EmbedWithTask(ctx context.Context, text string, taskType string) ([]float32, error)
}
```

Implemented by: `*GenAIEngine`.

### `TaskTypeBatchAwareEngine` — `engine.go`

```go
type TaskTypeBatchAwareEngine interface {
    EmbeddingEngine
    EmbedBatchWithTask(ctx context.Context, texts []string, taskType string) ([][]float32, error)
}
```

Implemented by: `*GenAIEngine`.

### `HealthChecker` — `engine.go`

```go
type HealthChecker interface {
    HealthCheck(ctx context.Context) error
}
```

Implemented by: `*OllamaEngine` (GET `/api/tags`, 2s timeout).

---

## 2. Configuration

### `Config` — `engine.go`

| Field | JSON tag | Notes |
|-------|----------|-------|
| `Provider` | `provider` | `"ollama"` or `"genai"` |
| `OllamaEndpoint` | `ollama_endpoint` | default localhost:11434 |
| `OllamaModel` | `ollama_model` | default embeddinggemma:300m |
| `GenAIAPIKey` | `genai_api_key` | required for genai |
| `GenAIModel` | `genai_model` | default gemini-embedding-001 |
| `TaskType` | `task_type` | default SEMANTIC_SIMILARITY |

### `DefaultConfig() Config` — `engine.go`

Returns local-first defaults (provider ollama, no GenAI key).

### `NewEngine(cfg Config) (EmbeddingEngine, error)` — `engine.go`

| Provider | Construct | Post |
|----------|-----------|------|
| `ollama` | `NewOllamaEngine` | EnsureModel 8s best-effort |
| `genai` | `NewGenAIEngine` | requires API key |
| other / empty | error | unsupported message |

---

## 3. Ollama API

### `NewOllamaEngine(endpoint, model string) (*OllamaEngine, error)`

Always succeeds for config (no dial). Defaults endpoint/model as documented.

### Methods on `*OllamaEngine`

| Method | Notes |
|--------|-------|
| `Embed` | Retries network/protocol/malformed-vector failures with cancellable backoff; auto-pull on missing model |
| `EmbedBatch` | Sequential |
| `Dimensions` | 768 |
| `Name` | synchronized `ollama:<model>` snapshot |
| `Model` | Current model (mutex); may differ post-ensure |
| `HealthCheck` | 2s `/api/tags` |
| `EnsureModel` | Serialized ensure/pull; a context-cancelled pull can be retried later |

Does **not** implement task-aware interfaces.

---

## 4. GenAI API

### `NewGenAIEngine(apiKey, model, taskType string) (*GenAIEngine, error)`

Fails if `apiKey == ""`. Defaults model and task type.

### Methods on `*GenAIEngine`

| Method | Notes |
|--------|-------|
| `Embed` | Uses engine default task type; validates first result and nil entries |
| `EmbedWithTask` | Explicit task |
| `EmbedBatch` | Chunk ≤100; parallel if larger; exact cardinality and uniform shape required |
| `EmbedBatchWithTask` | Explicit task for batch |
| `Dimensions` | 3072 |
| `Name` | `genai:<model>` |
| `Close` | No-op |
| `EmbedBatchJob` | Async submit; returns `*genai.BatchJob` |

`EmbedBatchJob` is exported but not part of `EmbeddingEngine`. Callers needing it must use `*GenAIEngine` concrete type or add a new interface.

---

## 5. Task selection API — `task_selector.go`

### `ContentType`

String alias with constants:

`code`, `documentation`, `conversation`, `knowledge_atom`, `prompt_atom`, `query`, `fact`, `question`, `answer`, `classification`, `clustering`.

### `SelectTaskType(contentType ContentType, isQuery bool) string`

Maps content type (+ query flag for code) → Google task type string.

### `DetectContentType(text string, metadata map[string]any) ContentType`

Metadata first, then heuristics.

### `GetOptimalTaskType(text string, metadata map[string]any, isQuery bool) string`

Detect + optional query remapping + Select.

---

## 6. Math API

### `CosineSimilarity(a, b []float32) (float64, error)` — `math_*.go`

- Error if lengths differ.
- Zero magnitude → `(0, nil)` (not error).
- Else cosine in [-1, 1].

### `FindTopK(query []float32, corpus [][]float32, k int) ([]SimilarityResult, error)` — `engine.go`

- `k <= 0` → 10.
- Skips corpus entries that error on cosine (dim mismatch); warns if any skipped.
- Partial selection-style sort for top k.
- Returns `[]SimilarityResult{Index, Similarity}`.

### `SimilarityResult`

```go
type SimilarityResult struct {
    Index      int
    Similarity float64
}
```

---

## 7. Unexported helpers (for implementers)

Not public API, but important for reviewers:

| Helper | File | Role |
|--------|------|------|
| `normalizeTaskType` | `task_selector.go` | Upper/trim |
| `embedWithTask` / `embedBatchWithTask` / `embedBatchChunk` | `genai.go` | Internal GenAI path |
| `resolveInstalledModel` | `ollama.go` | Tag matching |
| `preferInstalledEmbeddingModel` | `ollama.go` | Family prefer |
| `pullTargetFor` / `modelBase` | `ollama.go` | Pull naming |
| `isModelNotFoundStatus` | `ollama.go` | 404/400 body detect |
| `knownEmbedFamilies` | `ollama.go` | Safe remap set |
| `validateEmbeddingVector` | `engine.go` | Reject empty, NaN, and infinite vectors |
| `validateEmbeddingBatchResponse` | `engine.go` | Enforce N-to-N cardinality and one finite width |
| `waitForRetry` | `ollama.go` | Context-aware retry delay |
| `invalidateModel` | `ollama.go` | Locked readiness invalidation |

---

## 8. Compatibility notes for callers

1. **Never assume** concrete type unless you need `EmbedBatchJob` or `Model()`.
2. **Always** prefer task-aware assert when selecting GenAI task types.
3. **Empty batch** returns `nil, nil` (not empty slice) for both engines — check both `err` and length.
4. **Name()** may change for Ollama after ensure (model remapped); re-read after first Embed if logging identity.
5. Mocks for tests need only `EmbeddingEngine` unless the unit under test asserts optional interfaces — see `internal/store` mock engines.
6. Do not persist a result merely because the provider returned HTTP success;
   implementations now turn malformed single and batch payloads into errors.
