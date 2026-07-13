# 09 — Safety and Invariants: embedding

> Last verified against codebase: 2026-07-13  
> Safety, concurrency, dimensions, and constitutional posture

## 1. Constitutional posture

codeNERD safety is **`permitted(...)` default deny** in the Mangle kernel.  
`internal/embedding` does **not** evaluate permission. It must not become a side channel that executes host actions beyond:

1. HTTP to configured Ollama endpoint.
2. HTTPS to Google GenAI (with API key).
3. Disk growth via Ollama model pulls initiated by EnsureModel.

Any tool/action that *uses* embeddings remains gated by VirtualStore + policy.

## 2. Invariants

### I1 — Provider set is closed

`NewEngine` accepts only `ollama` and `genai`. Unknown provider → error. No silent fallback provider.

### I2 — GenAI requires a key

Empty API key → construct error. No anonymous cloud embed.

### I3 — Dimensions are stable per engine family

| Engine | Dimensions() | Requested width |
|--------|-------------:|-----------------|
| Ollama | 768 | model-defined (assumed 768) |
| GenAI | 3072 | `OutputDimensionality=3072` |

Callers may treat these as declared schema constraints, but an alternate Ollama
model may return another valid width. The package enforces response-internal
shape; store must enforce vector-space identity across persisted generations.

### I4 — Batch order preservation

`EmbedBatch` results index `i` corresponds to input text `i` (GenAI parallel path uses slot array; Ollama sequential).

### I5 — Empty batch is nil result

Both engines: `len(texts)==0` → `nil, nil`.

### I6 — Context cancellation

Embed loops check `ctx.Err()`; retry delays select on cancellation; errgroup
cancels siblings; EnsureModel uses request contexts.

### I7 — Known-family remap only

Ollama remaps/prefers only models in `knownEmbedFamilies` (plus exact/alias resolve of configured base). Custom names never silently become `nomic-embed-text` unless they are already in the known-family fallback path for failed pulls of known families.

### I8 — Bounded pull attempts with cancellation recovery

`pullAttempted` prevents infinite pull loops after a non-context pull failure.
If the pull failed because the caller context ended, the flag is reset so the
8-second bootstrap budget cannot poison a later request-scoped ensure.

### I9 — Cosine length safety

`CosineSimilarity` errors on dim mismatch. `FindTopK` skips mismatches rather than panicking.

### I10 — Leaf import graph

No dependency on core/mangle/store — prevents policy cycles and accidental store-from-embed recursion.

### I11 — Provider response contract

Every successful single result is non-empty and finite. Every successful batch
contains exactly one vector per input, uses one non-zero width, and contains no
NaN or infinity. Provider protocol success without this contract is an error.

## 3. Concurrency invariants

| Component | Safety claim |
|-----------|--------------|
| `OllamaEngine.ensureMu` | Serialize every `modelReady` / `model` / `pullAttempted` read and update |
| `OllamaEngine.Model()` | Lock-protected read |
| GenAI parallel batch | Each goroutine writes distinct `chunkResults[batchIdx]` |
| Pure math | Reentrant |

Do not copy an `OllamaEngine` by value (mutex). Always share pointer.

## 4. Secret handling

| Secret | Handling |
|--------|----------|
| GenAI API key | Required at construct; debug may log **length**, not full key |
| Ollama | Typically no auth in default local setup; if remote Ollama is used, endpoint is config-controlled |

Do not log full request bodies that may contain user secrets mixed into embed text at Info level; debug logs text **length**, not content (GenAI/Ollama Embed paths log lengths).

## 5. Resource safety

| Resource | Bound |
|----------|-------|
| Ollama HTTP timeout | 60s |
| Ollama pull timeout | 30m |
| NewEngine EnsureModel | 8s |
| HealthCheck | 2s |
| GenAI batch parallelism | 6 |
| GenAI max batch size | 100 |

Unbounded risk: **caller** may pass huge `texts` slices; GenAI will issue many parallel chunk groups of 6×100. Callers of mega-reembed should chunk further if needed.

## 6. Failure isolation

| Failure | Isolation |
|---------|-----------|
| Engine construct fail | Boot continues; semantic off |
| HealthCheck fail (factory) | Engine not attached |
| Mid-batch embed fail | Error returned; partial results discarded (no half-batch commit in package) |
| Cosine mismatch | Skip/error local to helper |

Persistence partial writes are **store** concerns (transactions), not embedding package.

## 7. Auto-pull policy (DX vs safety)

Auto-pull improves first-run UX but:

- Can consume significant disk and bandwidth.
- Can block first Embed for a long time (pull client 30m).
- Is limited to configured name + known-family fallbacks.

Operators on restricted networks should pre-pull models or use GenAI.

## 8. SIMD build tag safety

`math_amd64.go` uses Go's experimental `simd/archsimd` opaque-vector API. It is
only built on amd64 with both `GOEXPERIMENT=simd` and `-tags simd`; the generic
path remains the default. The experimental command is part of the verification
receipt so API drift is detected.

## 9. Cross-provider safety

Mixing vector widths in one DB is **unsafe for retrieval quality** even if code does not crash. Operational invariant:

> After changing `provider`, `ollama_model`, or GenAI model family, run reembed and verify `nerd embedding stats` dimensions.

## 10. What this package must never do

- Shell out to arbitrary commands.
- Write to workspace source trees.
- Assert Mangle facts.
- Bypass VirtualStore for tool execution.
- Log raw API keys.
- Remap unknown model names to “something that works”.
