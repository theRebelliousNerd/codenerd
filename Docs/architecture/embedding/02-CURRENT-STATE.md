# 02 — Current State: embedding

> Last verified against codebase: 2026-07-13  
> Status: Precise inventory of what is on disk today

## 1. Package existence

| Fact | Value |
|------|-------|
| Path | `C:\CodeProjects\codeNERD\internal\embedding\` |
| Non-test `.go` | 6 |
| Test `.go` | 7 |
| `.mg` / Mangle | 0 |
| Scoped guidance | `internal/embedding/agents.md` |
| Package clause | `package embedding` |
| Module import | `codenerd/internal/embedding` |

**Implementation exists.** Do not treat as pre-implementation 0%.

## 2. File inventory (sources)

| File | ≈Lines | Build tag | Responsibility |
|------|-------:|-----------|----------------|
| `ollama.go` | 657 | (default) | OllamaEngine, validated HTTP API, synchronized model lifecycle, context-aware retry, request DTOs |
| `genai.go` | 386 | (default) | GenAIEngine, validated Embed/Batch/BatchJob, 3072 dims, errgroup parallelism |
| `engine.go` | 249 | (default) | Interfaces, Config, NewEngine, FindTopK, provider-response validators |
| `task_selector.go` | 198 | (default) | ContentType, SelectTaskType, DetectContentType, GetOptimalTaskType, normalizeTaskType |
| `math_amd64.go` | 65 | `amd64 && simd` | Experimental SIMD CosineSimilarity via opaque `simd/archsimd` vectors |
| `math_generic.go` | 37 | `!amd64 \|\| !simd` | Scalar CosineSimilarity |

**Total non-test = 1,592 lines.**

## 3. File inventory (tests)

| File | ≈Lines | Notes |
|------|-------:|-------|
| `ollama_coverage_test.go` | 561 | httptest API, retry, malformed vectors, concurrent model state |
| `engine_coverage_test.go` | 494 | Config/factory, cosine/top-K, provider response contracts |
| `task_selector_coverage_test.go` | 409 | content/task matrices |
| `ollama_ensure_test.go` | 223 | resolution/pull helpers and cancelled-bootstrap re-arm |
| `genai_coverage_test.go` | 124 | construct validation; limited API surface |
| `task_selector_test.go` | 60 | smaller focused cases |
| `genai_bench_test.go` | 44 | `BenchmarkEmbedBatchParallel` (live) |

**Total test = 1,915 lines** (tests exceed production, with HTTP and concurrency edge coverage).

## 4. Exported surface inventory

### Types

| Name | Kind | File |
|------|------|------|
| `EmbeddingEngine` | interface | `engine.go` |
| `TaskTypeAwareEngine` | interface | `engine.go` |
| `TaskTypeBatchAwareEngine` | interface | `engine.go` |
| `HealthChecker` | interface | `engine.go` |
| `Config` | struct | `engine.go` |
| `SimilarityResult` | struct | `engine.go` |
| `GenAIEngine` | struct | `genai.go` |
| `OllamaEngine` | struct | `ollama.go` |
| `ContentType` | string alias + consts | `task_selector.go` |

### Functions / methods (exported)

| Name | File |
|------|------|
| `DefaultConfig` | `engine.go` |
| `NewEngine` | `engine.go` |
| `FindTopK` | `engine.go` |
| `NewGenAIEngine` | `genai.go` |
| `(*GenAIEngine) Embed`, `EmbedWithTask`, `EmbedBatch`, `EmbedBatchWithTask`, `Dimensions`, `Name`, `Close`, `EmbedBatchJob` | `genai.go` |
| `NewOllamaEngine` | `ollama.go` |
| `(*OllamaEngine) Embed`, `EmbedBatch`, `Dimensions`, `Name`, `Model`, `HealthCheck`, `EnsureModel` | `ollama.go` |
| `CosineSimilarity` | `math_*.go` |
| `SelectTaskType`, `DetectContentType`, `GetOptimalTaskType` | `task_selector.go` |

Ollama does **not** export task-aware methods (does not implement `TaskTypeAwareEngine`).

## 5. Constants and tunables (source)

| Symbol / value | Location | Meaning |
|----------------|----------|---------|
| `defaultOllamaEmbedModel = "embeddinggemma:300m"` | `ollama.go` | Default model tag |
| `maxBatchSize = 100` | `genai.go` | GenAI API batch ceiling |
| `batchParallelism = 6` | `genai.go` | Concurrent chunk calls |
| OutputDimensionality `3072` | `genai.go` | Requested embed size |
| Ollama client Timeout `60s` | `ollama.go` | Per-request HTTP |
| Pull client Timeout `30m` | `ollama.go` | Model download |
| EnsureModel at NewEngine `8s` | `engine.go` | Boot budget |
| HealthCheck timeout `2s` | `ollama.go` | Fast fail |
| Embed maxRetries `3` | `ollama.go` | Transient recovery |
| knownEmbedFamilies map | `ollama.go` | Safe remap set |
| Provider vector validation | `engine.go` | Reject empty/non-finite vectors and invalid batch shape |

## 6. Hotspots (complexity / risk)

| Hotspot | Why |
|---------|-----|
| `EnsureModel` + pull | Process-wide side effects (disk, time); mutex state; cancelled bootstrap re-arm |
| GenAI parallel `EmbedBatch` | Concurrency, 429 risk, order preservation |
| Hardcoded `Dimensions()` | Schema coupling to store |
| Task type heuristics | False positives (codeScore, conversation markers) |
| Dual build-tag cosine | Experimental path requires `GOEXPERIMENT=simd` and `-tags simd` |

## 7. Reverse dependency snapshot

Heavy importers (non-test):

- `internal/store/*` (vector, reembed, reflection, learning, learned_store)
- `internal/prompt/*` (loader, vector_searcher)
- `internal/perception/semantic_classifier.go`
- `internal/mcp/*` (store, compiler, analyzer, integration)
- `internal/system/factory.go`
- `internal/campaign/document_ingestor.go`, `decomposer_documents.go`
- `internal/init/initializer.go`
- `cmd/nerd/embedding_cmd.go`, `cmd/nerd/chat/*` (session_boot, reembed, reflection, ingest, model_types)
- `cmd/tools/corpus_builder`, `cmd/tools/prompt_builder`

This package is **load-bearing**, not experimental.

## 8. Configuration sources of truth

| Layer | Location |
|-------|----------|
| Package defaults | `embedding.DefaultConfig()` |
| User config struct | `config.EmbeddingConfig` / `UserConfig.GetEmbeddingConfig()` |
| On-disk | `.nerd/config.json` Embedding section |
| CLI mutation | `nerd embedding set …` |
| Boot mapping | factory + session_boot map UserConfig → `embedding.Config` |

GenAI key fallback in factory: if provider is genai and GenAIAPIKey empty, may use Cortex `apiKey` (`system/factory.go`).

## 9. What is deliberately absent

- No caching layer for identical texts.
- No rate-limit token bucket.
- No dimension negotiation RPC.
- No Mangle Decl of embed predicates.
- No OpenTelemetry spans.
- No Vertex AI backend configuration on GenAI client.
- No vector-space identity object covering provider, model, task, dimensions, and normalization.

## 10. Obsolescence notes for prior corpus

Earlier auto-stub docs under this folder used inventory-only tables and generic filler. This rebuild (2026-07-13) replaces that quality level with the CLI-bar corpus. Prefer the numbered files linked from `README.md`.
