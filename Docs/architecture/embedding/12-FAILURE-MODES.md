# 12 — Failure Modes: embedding

> Last verified against codebase: 2026-07-13  
> Concrete failures + mitigations grounded in source

## FM1 — Unsupported or empty provider

| | |
|--|--|
| **Trigger** | `NewEngine` with `Provider` not `ollama`/`genai`, or empty string |
| **Behavior** | Error; nil engine |
| **User impact** | Boot may continue without embeddings (caller-dependent) |
| **Mitigation** | Use `DefaultConfig()` or validate config UI; `nerd embedding set` |

## FM2 — GenAI missing API key

| | |
|--|--|
| **Trigger** | provider=genai, empty GenAIAPIKey (and no factory apiKey fallback) |
| **Behavior** | `NewGenAIEngine` error “GenAI API key is required” |
| **Mitigation** | Set key in config or `nerd embedding set genai <key>`; factory may inject cortex apiKey |

## FM3 — Ollama daemon down

| | |
|--|--|
| **Trigger** | No process on endpoint |
| **Behavior** | HealthCheck fails (2s); Embed retries then errors; EnsureModel list fails |
| **Factory impact** | Engine **not attached** after failed HealthCheck |
| **Chat boot impact** | Engine may still attach; first Embed fails loudly |
| **Mitigation** | Start Ollama; verify `curl endpoint/api/tags`; consider GenAI |

## FM4 — Model not installed

| | |
|--|--|
| **Trigger** | Configured model absent from `/api/tags` |
| **Behavior** | EnsureModel resolve → prefer family → pull; Embed 404 triggers ensure once |
| **Failure** | Pull fail + fallback fail → error; subsequent EnsureModel returns “still unavailable after prior pull” |
| **Mitigation** | Pre-pull; free disk; use known family names; check network for registry |

## FM5 — Pull takes too long / context cancelled

| | |
|--|--|
| **Trigger** | Large model download; short parent ctx |
| **Behavior** | pull client 30m but parent ctx can cancel earlier; context failure re-arms `pullAttempted` |
| **Mitigation** | A later request retries automatically; pre-pull offline for deterministic boot |

## FM6 — GenAI API error (4xx/5xx/network)

| | |
|--|--|
| **Trigger** | Invalid key, quota, outage, bad model name |
| **Behavior** | Embed/Batch returns wrapped error; parallel batch cancels remaining chunks on first failure |
| **Mitigation** | Check key/quota; reduce batch sizes; retry later; switch to Ollama |

## FM7 — GenAI rate limiting (429)

| | |
|--|--|
| **Trigger** | Parallelism 6 + other concurrent Gemini traffic |
| **Behavior** | Chunk failures; whole batch fails |
| **Mitigation** | Serialize reembeds; lower concurrency (code change); respect shared transport notes in genai.go |

## FM8 — Malformed provider response

| | |
|--|--|
| **Trigger** | Empty, nil, NaN, infinite, truncated, or mixed-width provider vectors |
| **Behavior** | Single or batch response validation returns an error; Ollama retries within its bound |
| **Mitigation** | Inspect provider/model health; no malformed vector reaches persistence through a successful engine call |

## FM9 — Dimension mismatch at similarity time

| | |
|--|--|
| **Trigger** | Query 3072 vs stored 768 (or partial reembed) |
| **Behavior** | CosineSimilarity error; FindTopK skips; store paths may drop candidates |
| **Mitigation** | Full reembed after provider change; verify stats dimensions |

## FM10 — Zero-magnitude vectors

| | |
|--|--|
| **Trigger** | All-zero embedding (pathological) |
| **Behavior** | CosineSimilarity returns 0, nil (not error) |
| **Impact** | Similarity ties at zero; ranking noise |
| **Mitigation** | Rare; investigate provider if common |

## FM11 — Mid-batch cancellation

| | |
|--|--|
| **Trigger** | User cancel / timeout during EmbedBatch |
| **Behavior** | Ollama returns error at current index; GenAI errgroup cancels |
| **Persistence** | Package returns error without partial slice guarantee for Ollama mid-loop (no results returned on error) |
| **Mitigation** | Callers should not commit partial DB batches without their own checkpointing |

## FM12 — Boot semantic feature loss (silent-ish)

| | |
|--|--|
| **Trigger** | Factory HealthCheck fail or NewEngine fail |
| **Behavior** | Cortex runs; AtomLoader may get nil engine; vector search degrades |
| **User impact** | Worse JIT selection / no semantic search without obvious hard fail |
| **Mitigation** | Read boot warnings; run stats; fix Ollama/key |

## FM13 — EmbedBatchJob experimental SDK break

| | |
|--|--|
| **Trigger** | SDK change; Vertex backend; empty texts |
| **Behavior** | Submit error; Vertex unsupported per comments |
| **Mitigation** | Prefer sync EmbedBatch for moderate sizes; pin SDK; poll carefully |

## FM14 — Wrong content-type detection

| | |
|--|--|
| **Trigger** | Heuristics mis-label code/docs |
| **Behavior** | Suboptimal GenAI task type → weaker retrieval |
| **Mitigation** | Pass explicit metadata `content_type` / `type` from callers (store already uses GetOptimalTaskType with metadata) |

## FM15 — Concurrent EnsureModel storms

| | |
|--|--|
| **Trigger** | Many goroutines Embed before modelReady |
| **Behavior** | Mutex serializes ensure; only one pull |
| **Mitigation** | Already handled; `Name`, `Model`, Embed snapshots, updates, and invalidation share the mutex and pass the race detector |

## FM16 — Experimental SIMD API or build-contract drift

| | |
|--|--|
| **Trigger** | Toolchain changes `simd/archsimd`, or build uses only `-tags simd` without enabling the experiment |
| **Behavior** | Optional accelerated build fails; default generic build is unaffected |
| **Mitigation** | Verify with `GOEXPERIMENT=simd go test -tags simd`; keep generic path as release fallback until policy is explicit |

## 3. Failure mode vs layer ownership

| Mode | Owned by embedding | Owned by caller/store |
|------|--------------------|------------------------|
| HTTP/API errors | **Yes** | surface to user |
| Schema/dim migration | contract only | **Yes** reembed |
| Permission to reembed | no | **Yes** policy/CLI |
| Partial DB write | no | **Yes** transactions |

## 4. Recovery cheat sheet

```
stats show 0 embeddings?
  → check config provider
  → check Ollama up / GenAI key
  → nerd embedding reembed

cosine errors after switching genai↔ollama?
  → force reembed all DBs

boot warning embedding unavailable?
  → factory HealthCheck failed; start Ollama or use genai

model pull loop failed?
  → manual `ollama pull embeddinggemma:300m`
  → if cancellation caused it, retry in the same engine instance
  → after a permanent pull failure, fix the root cause and restart or construct a new engine
```
