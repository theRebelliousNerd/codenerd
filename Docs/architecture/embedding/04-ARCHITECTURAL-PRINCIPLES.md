# 04 — Architectural Principles: embedding

> Binding design principles for `internal/embedding`.  
> Violations should be called out in review.  
> Last verified: 2026-07-13

## P1 — Leaf substrate, not executive

This package must not import `internal/core`, `internal/mangle`, or own `permitted` / `next_action` logic. It produces vectors; other layers decide what retrieval means for policy and planning.

## P2 — Interface-first providers

All production code depends on `EmbeddingEngine` (and optional type assertions). Do not scatter raw HTTP or GenAI SDK calls outside this package for embedding use cases.

## P3 — Config is authoritative; code defaults are fallbacks

Model names, endpoints, and providers come from `.nerd/config.json` via `config.GetEmbeddingConfig` at boot. Do not hardcode model strings in chat boot or shards. Package defaults exist only when config is empty.

## P4 — Task type is part of the semantic contract (GenAI)

When the engine is task-aware, callers **must** embed queries and documents with appropriate task types (`SelectTaskType` / `GetOptimalTaskType`). Treating task type as optional garnish degrades retrieval.

## P5 — Known-family remaps only

Ollama auto-prefer / fallback applies only to `knownEmbedFamilies`. Arbitrary configured names pull as-is or fail. Never “helpfully” rewrite unknown models.

## P6 — Fail soft at boot; fail loud at batch

- Boot may warn and continue without embeddings.
- Large batch/reembed paths should HealthCheck (when available) and surface errors rather than hang for minutes on a dead daemon.

## P7 — Dimensions are a contract

`Dimensions()` and GenAI `OutputDimensionality` must stay aligned with store schemas. Changing dimensions is a **migration** (reembed), not a silent code tweak.

Provider output width is validated for internal consistency, but it is not
blindly compared with the hardcoded Ollama `Dimensions()` value: configured
Ollama models can legitimately use another width. Cross-run compatibility
belongs to an explicit vector-space identity contract with store.

## P8 — Context cancellation is honored

Embed retries, batch loops, and EnsureModel must respect `ctx`. Do not sleep-ignore cancellation.

## P9 — Observability via CategoryEmbedding

Every public operation of consequence logs through `logging.CategoryEmbedding` (or convenience helpers) with latency and dimensions. Silent network failures are defects.

## P10 — Optional interfaces stay optional

Callers must type-assert `TaskTypeAwareEngine`, `TaskTypeBatchAwareEngine`, and `HealthChecker`. The base interface remains implementable by mocks and minimal engines.

## P11 — Wiring audit before deletion

Reverse deps span store, prompt, perception, mcp, campaign, system, init, CLI, and tools. “Unused” is almost certainly wrong without `rg` evidence.

## P12 — No time/cost estimates in architecture docs

Roadmaps and TODOs use priority and dependency order only (project-wide rule).

## P13 — Provider responses are untrusted input

No provider vector reaches a caller unless it is non-empty and finite. Batch
responses must also match input cardinality and use one width. A malformed
success payload is an error, not a degraded success.

## P14 — Mutable provider state has one lock owner

Every read or mutation of Ollama `model`, `modelReady`, and `pullAttempted`
must occur under `ensureMu` (directly or through a locking accessor). A pull
cancelled by its caller re-arms the attempt flag so a short boot context cannot
poison the engine for the session.
