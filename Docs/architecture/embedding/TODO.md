# embedding — architecture uplifts

> Sole authority for embedding `NERD_FEATURE` cards. Current truth remains in
> [02-CURRENT-STATE.md](02-CURRENT-STATE.md).

## Truth-gap repair

<!-- NERD_FEATURE
id: embedding-provider-contract-v1
owner: embedding
status: verified
kind: truth-gap
depends_on: []
affects: [embedding, store, prompt, perception]
-->

### Fail closed on malformed vectors and recoverable Ollama lifecycle faults

- **Value:** provider faults cannot silently become persisted empty/NaN vectors,
  truncated batches, stuck retries, or racy model identities.
- **Current evidence:** `internal/embedding/engine.go#validateEmbeddingVector` and
  `validateEmbeddingBatchResponse` gate provider output;
  `internal/embedding/ollama.go#Model`, `invalidateModel`, `waitForRetry`, and
  `EnsureModel` own model state, cancellation, and bootstrap retry rearming.
- **Observed gap closed:** prior code accepted empty Ollama vectors, trusted GenAI
  batch cardinality/shape, read mutable Ollama model state outside its lock,
  slept through cancellation, and permanently marked a deadline-cancelled boot
  pull as attempted.
- **Desired behavior:** each non-empty input yields exactly one finite,
  non-empty, batch-consistent vector or a contextual error; concurrent callers
  observe synchronized model state; request cancellation stops retry delay; a
  short bootstrap failure never disables a later request-scoped pull.
- **Non-goals:** enforcing hardcoded `Dimensions()` against alternate provider
  output, changing store schemas, parallelizing Ollama batch, or changing boot
  health policy.
- **Affected contracts:** provider response boundary, `EmbeddingEngine` order and
  cardinality, Ollama ensure/pull lifecycle, logging timers, race behavior, and
  optional SIMD compilation.
- **Positive acceptance:** valid single/batch vectors preserve order; a cancelled
  boot pull retries later; concurrent `Name`/`Model` reads remain race-free;
  generic and experimental SIMD cosine suites pass.
- **Negative acceptance:** empty/non-finite vectors, truncated/mixed-width
  batches, and permanent pull poisoning are rejected by named regressions before
  persistence.
- **Rollback:** revert validators and state helpers with their focused tests as
  one packet. Do not retain the old accept-malformed behavior behind a default-on
  compatibility flag.

## Safe leverage uplift

<!-- NERD_FEATURE
id: embedding-vector-space-identity-v1
owner: embedding
status: proposed
kind: leverage
depends_on: [embedding-provider-contract-v1]
affects: [embedding, store, config, cli]
-->

### Make vector-space identity an enforced cross-store contract

- **Value:** switching provider, model, task type, or dimensions cannot mix
  incompatible vectors or quietly degrade every semantic consumer.
- **Current evidence:** `EmbeddingEngine.Name` and `Dimensions` expose only part
  of identity; store metadata persists model/task details on several paths; CLI
  exposes manual stats/reembed. Alternate Ollama dimensions remain assumed.
- **Observed gap:** there is no single versioned identity value or boot-time gate
  proving every opened vector store matches the active engine and task policy.
- **Desired behavior:** define a stable identity containing provider, resolved
  model, task-space version, dimensions, and schema version. Store opens either
  prove parity, enter explicit read-only/degraded mode, or require reembed.
- **Non-goals:** automatic destructive migration, moving sqlite schemas into
  embedding, one vector width for every model, or using identity as permission.
- **Affected contracts:** embedding config, provider model resolution, vector
  metadata, store open/reembed, CLI stats, reflection, and prompt corpus sync.
- **Positive acceptance:** same-space stores open normally; a completed reembed
  atomically updates vectors and identity; every write path persists the same
  identity.
- **Negative acceptance:** seeded provider/model/task/dimension drift refuses
  mixed-space search and cannot be bypassed by a partial batch or process restart.
- **Rollback:** keep legacy metadata readable and disable only enforcement; retain
  mismatch telemetry and never auto-delete vectors.

## North-star uplift

<!-- NERD_FEATURE
id: embedding-semantic-health-receipt-v1
owner: embedding
status: proposed
kind: north-star
depends_on: [embedding-vector-space-identity-v1]
affects: [embedding, system, cli, observability, prompt, perception]
-->

### Publish one redacted semantic-capability health receipt

- **Value:** operators and agents can tell whether semantic retrieval is active,
  degraded, stale, or disabled and why, regardless of entrypoint.
- **Current evidence:** system factory health-checks Ollama before attachment;
  chat boot attaches after construction; `logging.CategoryEmbedding` and CLI
  stats expose fragments but no shared state machine.
- **Observed gap:** entrypoints disagree on health policy, feature loss is
  reconstructed from logs, and no receipt correlates engine identity, vector
  space parity, latest error, or recovery action.
- **Desired behavior:** one bounded receipt reports state, redacted engine
  identity, store parity, last transition/reason, and recovery command. System,
  chat, CLI, prompt, and perception consume the same state semantics.
- **Non-goals:** recording raw text/API keys, making telemetry authorize actions,
  constant health probes, or forcing boot failure for optional semantics.
- **Affected contracts:** system/chat boot, Cortex exposure, embedding timers,
  CLI stats/reembed, JIT/perception degradation, redaction, and retention.
- **Positive acceptance:** healthy and deliberately degraded boots emit the same
  schema and downstream features expose the matching state.
- **Negative acceptance:** secret-bearing, unbounded, contradictory, or
  receipt-authorized behavior fails; a health failure cannot masquerade as
  semantic-ready.
- **Rollback:** stop emitting the receipt and preserve existing warnings; vector
  validation and identity mismatch gates remain active.

## Bounded moonshot

<!-- NERD_FEATURE
id: embedding-shadow-retrieval-lab-v1
owner: embedding
status: deferred
kind: moonshot
depends_on: [embedding-semantic-health-receipt-v1]
affects: [embedding, campaign, evaluation, observability]
-->

### Compare candidate embedding spaces without mutating production indexes

- **Value:** codeNERD can measure retrieval quality, latency, denial quality, and
  resource cost before changing the semantic substrate for every agent.
- **Current evidence:** both providers, task selection, corpus/reembed tools, and
  deterministic top-K helpers exist; there is no versioned evaluation corpus or
  isolated shadow index.
- **Observed gap:** provider/model decisions rely on ad-hoc benchmarks and live
  reembedding rather than reproducible relevance cases and failure budgets.
- **Desired behavior:** replay redacted/versioned query-document relevance cases
  into disposable candidate indexes, label nondeterminism, compare against the
  active identity, and produce an evidence receipt with no production writes.
- **Non-goals:** storing user prompts, automatic provider rollout, online shadow
  network calls without budget, or model-selected authorization.
- **Affected contracts:** evaluation corpus, campaign sampling, provider budgets,
  disposable stores, relevance metrics, redaction, and artifact retention.
- **Positive acceptance:** a seeded candidate produces deterministic structural
  receipts and measured deltas while production store hashes remain unchanged.
- **Negative acceptance:** any production write, secret retention, unbounded
  sample, missing baseline, or automatic adoption rejects the run.
- **Rollback:** delete disposable indexes and receipts and disable the lab;
  production embedding, identity, and health paths remain unchanged.
