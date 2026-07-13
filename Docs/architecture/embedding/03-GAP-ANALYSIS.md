# 03 — Gap Analysis: embedding

> Last verified against codebase: 2026-07-13  
> Status: Spec/vision vs reality matrix

## 1. How to read this doc

- **Gap** = vision or production need not fully met by current code.
- **Non-gap** = intentional limit or already solved.
- Priorities: P0 operational correctness, P1 quality/throughput, P2 polish.

## 2. Matrix

| Desired capability | Reality | Status | Priority |
|--------------------|---------|--------|----------|
| Unified EmbeddingEngine API | Interfaces + two engines | **Met** | — |
| Local default + cloud option | DefaultConfig ollama; genai path | **Met** | — |
| Auto-install local model | EnsureModel resolve/pull/fallback | **Met** | — |
| Task-typed GenAI embeds | EmbedWithTask + SelectTaskType | **Met** | — |
| Task-typed Ollama embeds | Ollama API has no task type | **Non-gap** (backend limit) | — |
| Fail closed on malformed provider output | Single and batch validators enforce finite, non-empty, cardinality-consistent output | **Met** | — |
| Concurrent Ollama model state | Public reads, updates, and invalidation share `ensureMu` | **Met** | — |
| Bootstrap pull retry after cancellation | Cancelled/deadline pull re-arms `pullAttempted` | **Met** | — |
| Retry cancellation | Backoff selects on `ctx.Done()` | **Met** | — |
| Accurate Dimensions() | Hardcoded 768 / 3072 | **Gap** | P0 |
| Safe provider switch | CLI set + reembed; no auto-migrate | **Partial** | P0 |
| Consistent boot health policy | factory drops unhealthy Ollama; chat boot keeps engine | **Gap** | P1 |
| GenAI preflight health | No HealthChecker on GenAI | **Gap** | P2 |
| Large-batch cloud path | Sync parallel + EmbedBatchJob submit | **Partial** (no poll helper) | P1 |
| Large-batch local path | Sequential EmbedBatch | **Gap** | P1 |
| FindTopK production ANN | Bubble partial sort utility only | **Non-gap** (store owns search) | — |
| SIMD cosine buildability | Current opaque-vector API compiles and passes with experiment + tag | **Met** | — |
| SIMD cosine in default release | Explicit experiment + tag required | **Partial** | P2 |
| Live CI against Ollama/GenAI | Unit mocks only | **Gap** | P2 |
| Metrics / RED signals | Logs only | **Gap** | P2 |
| Cache identical embeds | None | **Gap** | P2 |
| EmbedBatchJob end-to-end | Submit only | **Gap** | P1 |
| Documented dim migration | Operator must reembed | **Partial** (docs now) | P1 |

## 3. P0 gaps (correctness)

The prior provider-output trust, Ollama state-race, cancelled-bootstrap poisoning,
and non-cancellable retry gaps are **VERIFIED CLOSED** by focused and race tests.
The remaining P0 concerns cross package boundaries and require coordinated store
migration rather than a package-local hardcoded-width check.

### G-P0-1 — Dimension contract vs reality

`OllamaEngine.Dimensions()` always 768; `GenAIEngine.Dimensions()` always 3072; GenAI always requests `OutputDimensionality: 3072`.

**Risk:** User configures an Ollama model with different width, or Google changes defaults; sqlite-vec tables and brute-force cosine assume one width.

**Mitigation today:** Operator discipline + reembed.  
**Better:** Probe first embed length, or config field `dimensions` validated at boot.

### G-P0-2 — Cross-provider vector pollution

Switching `provider` in config without reembed leaves incompatible vectors. CosineSimilarity errors on length mismatch (FindTopK skips; store paths vary).

**Mitigation today:** `nerd embedding reembed` / chat reembed; reflection workers check expected task/model/dim in store layer.

## 4. P1 gaps (quality / ops)

### G-P1-1 — Boot asymmetry

| Path | Unhealthy Ollama |
|------|------------------|
| `system.factory` `initIntelligenceLayer` | Engine not attached |
| `chat.session_boot` | Engine attached if NewEngine succeeds |

Operators get different semantic capability depending on entrypoint.

### G-P1-2 — Ollama batch throughput

Large reembeds on Ollama are O(n) serial HTTP. Fine for small workspaces; painful for multi-DB force reembed.

### G-P1-3 — EmbedBatchJob incomplete productization

Submit is implemented; polling, result extraction, and retry policy live outside the package (or nowhere yet for some tools).

## 5. P2 gaps (polish)

- GenAI HealthCheck (cheap Embed of `"ping"` or models list if API allows).
- Decide whether release builds opt into Go's experimental SIMD feature.
- Optional embed cache keyed by hash(model, task, text).
- Metrics counters under logging or future telemetry package.
- Nightly optional integration job with real Ollama.

## 6. Non-gaps (do not “fix”)

| Item | Why it is fine |
|------|----------------|
| No Mangle in package | Correct leaf layering |
| Ollama ignores task type | Provider API |
| FindTopK not ANN | Store + sqlite-vec is real search path |
| Sequential Ollama by default | Correctness/simplicity before premature pools |
| Auto-pull may take minutes | Explicit DX tradeoff; 30m pull client, bounded by caller context |
| knownEmbedFamilies limited | Protects custom models and tests |

## 7. Spec vs inventory honesty

Earlier stub corpus claimed “1:1 complete internal coverage” while remaining thin. This rebuild documents **behavior**, not only symbols. Gap count above is intentional residual work, not evidence the package is unbuilt.

## 8. Recommended order of closure

1. Introduce vector-space identity (provider/model/task/dimensions) coordinated with store.
2. Document + optionally enforce reembed on provider/model change (G-P0-2).  
3. Align factory vs chat health policy (G-P1-1).  
4. Productize async GenAI job helper if corpus tools need it (G-P1-3).  
5. Ollama parallel batch if profiling demands (G-P1-2).  
6. P2 polish as capacity allows.
