# embedding — trustworthy semantic transduction

> **VERIFIED CURRENT** on 2026-07-13 against `internal/embedding` and its live
> system, store, prompt, perception, MCP, campaign, init, and CLI consumers.
> `corpus.toml` owns the source boundary; [_progress.md](_progress.md) owns the
> test and review receipts.

## In one minute

codeNERD needs fuzzy meaning before its deterministic kernel can reason over
facts. `internal/embedding` turns text into vectors through local Ollama or Google
GenAI, chooses retrieval task types when the provider supports them, and supplies
cosine/top-K helpers. A user sees the result as better intent recognition, prompt
atom selection, knowledge recall, tool discovery, and document grounding.

This package is the trust boundary between remote or local provider responses and
vector persistence. It now rejects empty, non-finite, truncated, or internally
inconsistent results before callers can store them. It does not decide what an
action means or whether that action is permitted.

## Its place in codeNERD

The LLM remains the creative center and the Mangle kernel remains the executive.
Embeddings are semantic evidence: perception or retrieval code may translate
similarity hits into structured candidates, but only kernel policy can derive an
executive decision.

```text
text/config -> embedding.NewEngine -> validated vector
                                         |
            perception / prompt / store / MCP / campaign
                                         |
                    structured candidate facts or context
                                         |
          Mangle decision -> permitted effect -> articulation
```

The package owns provider construction, provider-specific requests, task-type
selection, vector response validation, similarity math, and bounded local-model
recovery. Stores own persistence and reembedding; system/chat own boot policy;
core owns permission; articulation owns the response.

## A representative journey

Consider a user asking, “Which rule prevents this write?”

1. `internal/perception/semantic_classifier.go#Classify` or a JIT/store search
   asks the configured engine for a query vector.
2. `internal/embedding/task_selector.go#GetOptimalTaskType` selects a query-side
   task. A task-aware GenAI engine uses it; Ollama uses its provider-native path.
3. `internal/embedding/genai.go#embedBatchChunk` or
   `internal/embedding/ollama.go#Embed` obtains a provider response. The shared
   validators enforce vector cardinality, finiteness, non-emptiness, and uniform
   batch width.
4. Store or prompt code compares the query with indexed vectors. Matching
   patterns become context or structured semantic candidates; an allow score is
   not permission.
5. The kernel derives the next action under `permitted/3`; normal execution and
   articulation produce the visible answer.

If Ollama is unavailable, boot may leave semantic features disabled or an embed
call returns a bounded error, depending on entrypoint. Transient request retries
observe cancellation. A short bootstrap model-pull timeout is rearmed so the
first real request can retry rather than leaving the engine permanently poisoned.

## What exists today

| Claim | Status | Evidence |
|---|---|---|
| One provider-neutral base interface supports Ollama and GenAI | **VERIFIED CURRENT** | `internal/embedding/engine.go#EmbeddingEngine`; `internal/embedding/engine.go#NewEngine`; package tests |
| Default configuration is local Ollama with `embeddinggemma:300m` | **VERIFIED CURRENT** | `internal/embedding/engine.go#DefaultConfig`; `internal/embedding/engine_coverage_test.go#TestDefaultConfig_WhenCalled_ShouldReturnSensibleDefaults` |
| Provider output is rejected when empty, non-finite, truncated, or mixed-width | **VERIFIED CURRENT** | `internal/embedding/engine.go#validateEmbeddingVector`; `internal/embedding/engine.go#validateEmbeddingBatchResponse`; `internal/embedding/engine_coverage_test.go#TestValidateEmbeddingBatchResponseEnforcesCardinalityAndShape`; `internal/embedding/ollama_coverage_test.go#TestOllamaEngine_Embed_WhenServerReturnsEmptyEmbedding_ShouldRetryAndFail` |
| Ollama model/readiness state is concurrency-safe at public read and invalidation sites | **VERIFIED CURRENT** | `internal/embedding/ollama.go#Model`; `internal/embedding/ollama.go#invalidateModel`; `internal/embedding/ollama_coverage_test.go#TestOllamaEngineModelAccessIsConcurrentSafe`; race receipt in [_progress.md](_progress.md) |
| A bootstrap pull deadline allows a later request-scoped retry | **VERIFIED CURRENT** | `internal/embedding/ollama.go#EnsureModel`; `internal/embedding/ollama_ensure_test.go#TestEnsureModel_BootstrapDeadlineDoesNotPoisonLaterPull` |
| GenAI batches preserve input order and reject a partial response | **VERIFIED CURRENT** | `internal/embedding/genai.go#embedBatchWithTask`; `internal/embedding/genai.go#embedBatchChunk`; response-contract tests |
| `Dimensions()` always describes the configured model's observed output | **PARTIAL** | GenAI requests/reports 3072 and Ollama reports 768, but alternate Ollama widths are not discovered; [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) |
| Every boot surface applies one health/degradation policy | **PARTIAL** | `internal/system/factory.go#initIntelligenceLayer` health-checks optional engines; `cmd/nerd/chat/session_boot.go` attaches after construction without the same probe |
| The optional SIMD implementation is buildable on the current toolchain | **VERIFIED CURRENT** | `internal/embedding/math_amd64.go#CosineSimilarity`; `GOEXPERIMENT=simd go test -tags simd` receipt in [_progress.md](_progress.md) |

### Applicability matrix

| Lane | Embedding contract and evidence |
|---|---|
| Mangle | **N-A locally:** `internal/embedding` contains no `.mg` and imports no core package. Semantic consumers such as `internal/perception/semantic_classifier.go#Classify` may assert candidates; core policy owns their executive use. |
| Permission and safety | Provider/network access is configured and bounded, vector responses are validated, and model pulls are explicit side effects. Embedding similarity never grants `permitted/3`. |
| Fact flow | Text becomes a vector, then retrieval evidence or prompt context. Perception/kernel/session/articulation own intent, decision, effect, and response. |
| JIT and agents | `internal/prompt/vector_searcher.go#CompilerVectorSearcher` and atom loaders consume the engine; this package owns no prompt prose, token budget, or agent lifecycle. |
| Wiring | System factory, chat boot, init, campaign, CLI, tools, MCP, prompt, perception, and store import the package. Health policy is intentionally documented as inconsistent rather than silently normalized. |
| State and concurrency | GenAI chunk slots are disjoint; Ollama mutable model state is mutex-owned; retry waits observe context. Vector persistence and deduplication are caller-owned. |
| Recovery | Ollama has bounded retries, missing-model ensure/pull, context-aware backoff, and bootstrap retry rearming. GenAI parallel batches cancel siblings and discard partial package results. |
| Observability | `logging.CategoryEmbedding` timers and bounded metadata cover construction, retries, pulls, dimensions, and failures. No correlated metrics/receipt exists. |
| Testing | Mock HTTP, provider/config, response-contract, task selection, math, race, and experimental SIMD gates are local. Live GenAI/Ollama integration remains opt-in. |

## North star

Every semantic consumer should receive a validated, identity-bearing vector from
one shared engine, with provider/model/task/dimension provenance sufficient to
prevent mixed-space retrieval. Provider switches should make required reembedding
obvious, and all boot paths should report the same machine-readable semantic
capability state.

Non-goals:

- moving sqlite-vec, ANN indexing, or database migration into this package;
- treating similarity as constitutional authorization;
- importing the kernel or asserting facts directly;
- silently remapping unknown custom Ollama models;
- building another chat-completion client beside perception;
- retaining raw text or API secrets in telemetry.

## Improvement frontier

The authoritative feature cards are in [TODO.md](TODO.md):

1. **Truth-gap repair (verified):** provider outputs fail closed; Ollama model
   state, cancellation, and bootstrap retry semantics are regression-tested.
2. **Safe leverage:** make vector-space identity (`provider/model/task/dimension`)
   a checked contract across embedding and store, with an explicit reembed gate.
3. **North-star advance:** expose one semantic-capability health receipt shared by
   system factory, chat boot, CLI stats, and recovery paths.
4. **Bounded moonshot:** shadow-evaluate candidate embedding spaces on redacted,
   versioned retrieval cases with no automatic provider or index mutation.

## Choose a reading route

### 90-second orientation

Read this page and the four feature cards in [TODO.md](TODO.md).

### 10-minute tour

Read [02-CURRENT-STATE.md](02-CURRENT-STATE.md),
[03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md), and
[08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md).

### Deep implementation and assurance

- Runtime contracts: [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md),
  [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md), and
  [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md).
- Safety and operations: [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md),
  [11-OBSERVABILITY.md](11-OBSERVABILITY.md), and
  [12-FAILURE-MODES.md](12-FAILURE-MODES.md).
- Verification and change evidence: [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md)
  and [_progress.md](_progress.md).
