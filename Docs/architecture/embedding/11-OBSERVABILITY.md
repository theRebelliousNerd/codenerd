# 11 — Observability: embedding

> Last verified against codebase: 2026-07-13  
> Logging categories, timers, debug hooks

## 1. Logging category

| Constant | Value | File |
|----------|-------|------|
| `logging.CategoryEmbedding` | `"embedding"` | `internal/logging/logger.go` |

Convenience helpers (`internal/logging/logger_convenience.go`):

| Helper | Level |
|--------|-------|
| `logging.Embedding` | Info |
| `logging.EmbeddingDebug` | Debug |
| `logging.EmbeddingWarn` | Warn |
| `logging.EmbeddingError` | Error |

Package code also uses `logging.Get(logging.CategoryEmbedding).Warn/Error` directly.

## 2. Timed operations

`logging.StartTimer(logging.CategoryEmbedding, opName)` is used for:

| Op name | Where |
|---------|-------|
| `NewEngine` | factory |
| `NewOllamaEngine` | construct |
| `NewGenAIEngine` | construct |
| `Ollama.Embed` | single embed (`defer Stop`, including failure/cancel paths) |
| `Ollama.EmbedBatch` | batch |
| `Ollama.HealthCheck` | health |
| `GenAI.Embed` | single |
| `GenAI.EmbedBatch` | batch |
| `GenAI.EmbedBatchJob` | async submit |
| `FindTopK` | utility |

Timers emit duration via the logging subsystem’s timer implementation (see logging corpus).

## 3. What is logged (by phase)

### 3.1 Construction

- Provider choice and non-secret config summary (endpoint, model, task type).
- GenAI API key **length** at debug (not value).
- Client create latency.
- Ollama EnsureModel init warnings if 8s budget fails.

### 3.2 Embed (Ollama)

- Text length, model, attempt numbers.
- POST URL (endpoint + `/api/embeddings`).
- API latency on success.
- Retry warnings with backoff.
- Invalid provider-vector warnings followed by bounded retry.
- Auto-pull warnings on model missing.
- Final dimensions + latency at Info on success.

### 3.3 Embed (GenAI)

- Text length / total chars for batch.
- Model, task type.
- Chunk indices for multi-batch.
- Parallel wall time, dimensions.
- EmbedBatchJob job name + submit latency.

### 3.4 Ensure / pull

- Resolve remap messages (`configured → installed`).
- Pull start/finish with duration.
- Fallback to nomic-embed-text warnings.
- Context-cancelled pull failure is visible and a future ensure remains eligible.

### 3.5 Math

- Debug: dimension of cosine inputs and result.
- Error: dimension mismatch.
- Warn: zero magnitude; FindTopK skipped vectors.

## 4. Operator surfaces outside the package

| Surface | What you see |
|---------|----------------|
| Boot stderr | “Warning: Embedding engine unavailable…” / “Failed to init…” |
| Boot logs | `logging.Boot` lines with provider/model from session_boot |
| `nerd embedding stats` | total vectors, with/without embeddings, engine name, dimensions |
| `nerd embedding reembed` | progress callback lines + summary counts |
| Glass box / TUI | may surface reembed/reflection activity (CLI corpus) |

## 5. Metrics

**None in-package.** No Prometheus counters, no OpenTelemetry spans.

If a future metrics layer is added, recommended series:

- `embedding_embed_latency_seconds` (labels: provider, model, op=single|batch)
- `embedding_embed_errors_total`
- `embedding_ensure_pull_total` / `embedding_ensure_pull_seconds`
- `embedding_batch_chunks_total`

## 6. Debug playbook

| Symptom | What to enable / check |
|---------|------------------------|
| Semantic search dead | Boot logs for engine init; `embedding stats`; provider in config.json |
| Slow first request | EnsureModel / pull logs; Ollama model load |
| 429 / rate limit GenAI | batchParallelism=6 + shared transport comments; reduce concurrent reembeds |
| Dim mismatch errors | stats dimensions vs stored vectors; reembed after provider change |
| Infinite pull attempts | should not happen (`pullAttempted`); check logs for “still unavailable after prior pull” |
| Repeated malformed vectors | inspect invalid-vector warning/error and provider model health; no malformed value is returned |

Enable debug for category `embedding` via the logging configuration used by the process (`-v` / config log levels — see logging/CLI docs).

## 7. Privacy notes

- Prefer logging **lengths** and **counts**, not full user text (current code largely follows this).
- Do not raise log level of raw prompts in shared multi-tenant logs without review.
- API keys: never Info-log full key.

## 8. Relation to glass box

Embedding package does not emit glass-box events itself. Higher layers (chat reflection, reembed progress, perception) may narrate embedding activity to the user.

A remaining north-star gap is one redacted semantic-capability receipt that
joins provider identity, declared/observed width, health/degradation state, and
last validation result without logging text or secrets.
