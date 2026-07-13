# 05 — Internal Architecture: embedding

> Last verified against codebase: 2026-07-13  
> Components, data flow, key types, state machines

## 1. Component diagram

```
                    ┌────────────────────────────┐
                    │         Config             │
                    │  Provider, Ollama*, GenAI* │
                    └─────────────┬──────────────┘
                                  │ NewEngine
                    ┌─────────────▼──────────────┐
                    │      Factory switch        │
                    └──────┬──────────────┬──────┘
                           │              │
              ┌────────────▼──┐    ┌──────▼────────────┐
              │ OllamaEngine  │    │   GenAIEngine     │
              │ http.Client   │    │   *genai.Client   │
              │ ensureMu state│    │   model, taskType │
              └───────┬───────┘    └─────────┬─────────┘
                      │                      │
         /api/embeddings  /api/tags  EmbedContent  Batches.CreateEmbeddings
         /api/pull
                      │                      │
                      └──────────┬───────────┘
                                 │ []float32 / [][]float32
                    ┌────────────▼──────────────┐
                    │ CosineSimilarity / FindTopK│
                    │ task_selector helpers      │
                    └───────────────────────────┘
```

## 2. Layering inside the package

| Layer | Files | Role |
|-------|-------|------|
| Contract | `engine.go` | Interfaces, config, factory, top-K |
| Provider A | `ollama.go` | Local HTTP lifecycle |
| Provider B | `genai.go` | Cloud SDK lifecycle |
| Semantics | `task_selector.go` | Content → GenAI TaskType |
| Math | `math_*.go` | Similarity primitive |

No cyclic internal deps; math and task_selector are independent of providers.

## 3. Key types (behavioral)

### 3.1 `EmbeddingEngine`

Minimum contract for all callers. Implementations:

- `*OllamaEngine`
- `*GenAIEngine`
- test mocks in other packages

### 3.2 Optional capability interfaces

| Interface | Implementers | Purpose |
|-----------|--------------|---------|
| `TaskTypeAwareEngine` | GenAI | Per-call task type |
| `TaskTypeBatchAwareEngine` | GenAI | Per-batch task type |
| `HealthChecker` | Ollama | Fast availability probe |

### 3.3 `OllamaEngine` fields

| Field | Role |
|-------|------|
| `endpoint` | Trimmed base URL |
| `model` | May change after resolve/pull |
| `client` | 60s timeout HTTP |
| `ensureMu` | Serializes ensure state |
| `modelReady` | Skip re-list when true |
| `pullAttempted` | One pull per engine instance |

### 3.4 `GenAIEngine` fields

| Field | Role |
|-------|------|
| `client` | SDK client |
| `model` | Embedding model id |
| `taskType` | Default when call omits |

## 4. State machines

### 4.1 Ollama model readiness

```
        ┌────────────┐
        │  unknown   │
        └─────┬──────┘
              │ EnsureModel
              ▼
        list tags OK?
         /         \
       no           yes
        │            │
        ▼            ▼
      error     resolve exact/alias?
                 /            \
               yes             no
                │               │
                ▼               ▼
            modelReady    prefer known family?
                           /            \
                         yes             no
                          │               │
                          ▼               ▼
                      modelReady    pullAttempted?
                                     /          \
                                   yes           no
                                    │             │
                                    ▼             ▼
                                 error      pullModel
                                              /    \
                                           ok       fail
                                            │         │
                                            ▼         ▼
                                       modelReady  fallback nomic?
                                                    /        \
                                                  ok        fail
                                                   │          │
                                                   ▼          ▼
                                              modelReady    error
```

### 4.2 Ollama Embed retry

```
attempt 1..3:
  if ctx done → return
  POST embed
  network/5xx/decode → backoff, continue
  model missing + !pulledOn404 → EnsureModel, reset attempt, continue
  other 4xx → fail
  200 → return embedding
exhausted → fail
```

### 4.3 GenAI multi-batch

```
texts empty → nil,nil
len ≤ 100 → embedBatchChunk
else:
  split chunks of 100
  errgroup limit 6
  each: check gctx → chunk → slot[i]
  Wait → flatten slots → return
```

## 5. Data shapes

### 5.1 Ollama wire

```json
// request
{"model":"embeddinggemma:300m","prompt":"..."}
// response
{"embedding":[0.1, 0.2, ...]}
```

### 5.2 GenAI wire (SDK)

- Content: user-role text parts
- Config: `TaskType`, `OutputDimensionality=3072`
- Result: `Embeddings[i].Values []float32`

### 5.3 Task type strings

Normalized with `strings.ToUpper(strings.TrimSpace)`. Callers should pass canonical Google task type names.

## 6. Concurrency model

| Path | Concurrency |
|------|-------------|
| Ollama Embed | Single request; retries sequential |
| Ollama EmbedBatch | Sequential loop |
| Ollama EnsureModel | Mutex; safe concurrent callers |
| GenAI Embed | Single RPC |
| GenAI EmbedBatch >100 | Up to 6 concurrent RPCs; shared engine (SDK client) |
| CosineSimilarity / FindTopK | Pure functions; no shared state |
| FindTopK | Single-threaded scan/sort |

**Thread safety:** Engine instances are intended for concurrent Embed if the underlying client allows. Ollama ensure path is mutex-safe. GenAI client concurrency relies on SDK + shared HTTP transport elsewhere (comment references `internal/perception/transport.go` for 429 risk).

## 7. Error taxonomy (internal)

| Class | Examples |
|-------|----------|
| Config | missing GenAI key; unsupported provider |
| Network | dial errors; timeouts |
| Protocol | non-200; decode failures |
| Model lifecycle | missing model; pull failed |
| API semantic | no embeddings returned |
| Math | dimension mismatch |

All public methods wrap with `%w` where appropriate for caller inspection.

## 8. Interaction with task selector

Task selector is **pure** (no I/O). Providers do not call it internally. Callers compose:

```
task := GetOptimalTaskType(text, meta, isQuery)
if ta, ok := eng.(TaskTypeAwareEngine); ok {
  ta.EmbedWithTask(ctx, text, task)
} else {
  eng.Embed(ctx, text)
}
```

This keeps Ollama free of unused task parameters.

## 9. Build-tag math selection

```
go build                  → math_generic.go (almost always)
go build -tags simd       → math_amd64.go on amd64
```

Both export identical signatures. Tests use whichever is selected by the build.

## 10. Extension points (if growing the package)

Safe extensions:

- New provider implementing `EmbeddingEngine` (+ optional ifaces) registered in `NewEngine` switch.
- Config fields for dimensions, timeout, parallelism (with DefaultConfig updates).
- Shared retry helper extracted from Ollama (not required yet).

Unsafe extensions:

- Importing store/core into embedding.
- Embedding business rules about which vectors are “permitted”.
- Auto-switching providers at runtime without reembed.
