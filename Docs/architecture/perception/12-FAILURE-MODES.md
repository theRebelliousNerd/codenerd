# 12 — Failure Modes (perception)

> Last verified: **2026-07-13**

## Catalog

### F1 — LLM complete failure (network / 5xx / timeout)

| | |
|--|--|
| **Symptoms** | Degraded `/explain` intent; user sees trouble message or llm_unavailable clarification |
| **Detection** | `[Understand] LLM FAILED`; `TransientFailure` when sentinel present |
| **Mitigation** | Provider retries (Gemini/ZAI); classification on fast tier reduces load; user retry |
| **Code** | `understanding_adapter.go`, `client_gemini.go` `ErrLLMUnavailable` |

### F2 — JSON parse failure (model free-text / markdown)

| | |
|--|--|
| **Symptoms** | Same degraded path; parse FAILED with response preview |
| **Detection** | `no JSON found` / `JSON parse failed` |
| **Mitigation** | `ExtractCleanJSON` last-valid-object; Gemini schema path; prompt contract |
| **Code** | `transducer_llm.go` |

### F3 — Schema/prompt contract mismatch (JIT)

| | |
|--|--|
| **Symptoms** | Silent fallback to embedded prompt; unexpected fields |
| **Detection** | `JIT perception prompt rejected` debug |
| **Mitigation** | `isValidUnderstandingPromptContract`; keep atoms aligned |
| **Code** | `understanding_adapter.go` |

### F4 — Semantic classifier unavailable

| | |
|--|--|
| **Symptoms** | No exemplars; no semantic_match facts; logs `SharedSemanticClassifier=nil` or embed fail |
| **Detection** | Boot warn on embed engine; empty matches |
| **Mitigation** | Regex/taxonomy + LLM-only path continues |
| **Code** | `semantic_classifier.go` |

### F5 — Cold embed hydrate hang (historical)

| | |
|--|--|
| **Symptoms** | TUI freeze minutes on first boot |
| **Mitigation** | `intentHydrateTimeout` 60s + chunked cache writes |
| **Code** | `semantic_classifier.go` |

### F6 — Provider misconfiguration

| | |
|--|--|
| **Symptoms** | Factory error; chat cannot boot LLM |
| **Detection** | `no API key found`; provider key missing with named field |
| **Mitigation** | Fix config; subscription engines without keys |
| **Code** | `client_factory.go` |

### F7 — Rate limits (CLI / API)

| | |
|--|--|
| **Symptoms** | RateLimitError; probe rate limited; retries exhaust |
| **Mitigation** | Backoff; fallback models on some CLI clients; user waits |
| **Code** | `claude_cli_client.go`, probe files, ZAI retry |

### F8 — OAuth token expiry (xai-oauth)

| | |
|--|--|
| **Symptoms** | 401; auth probe fail |
| **Mitigation** | `EnsureValid` refresh; re-device login; import from grok CLI |
| **Code** | `xaioauth/token.go`, `auth_device.go` |

### F9 — Mangle fact injection attempt

| | |
|--|--|
| **Symptoms** | Attempted `). malicious` in target |
| **Mitigation** | `sanitizeFactArg` strips syntactic chars |
| **Tests** | `break_test.go` |
| **Code** | `transducer.go` |

### F10 — Pathological JSON responses (memory)

| | |
|--|--|
| **Symptoms** | Huge responses with thousands of braces |
| **Mitigation** | maxJSONCandidates, break-tested extract |
| **Code** | `ExtractCleanJSON` |

### F11 — Learning queue full

| | |
|--|--|
| **Symptoms** | Patterns not learned |
| **Detection** | `ConsolidationWorker queue full, dropping` |
| **Mitigation** | Accept drop; drain on stop; increase buffer only if justified |
| **Code** | `consolidation.go` |

### F12 — Wrong verb inheritance (historical)

| | |
|--|--|
| **Symptoms** | “thanks” → coder shard |
| **Mitigation** | Removed stability-bypass; every turn reclassifies |
| **Code** | comments in `understanding_adapter.go` |

### F13 — Classification on huge main model (historical)

| | |
|--|--|
| **Symptoms** | Minutes before any action each turn |
| **Mitigation** | `NewClassificationClientFromConfig` ignores main Model |
| **Code** | `client_factory.go` |

### F14 — Taxonomy schema reload cost (historical bug #18)

| | |
|--|--|
| **Symptoms** | Slow ClassifyInput every call |
| **Mitigation** | One-shot `schemasLoaded` static load |
| **Code** | `taxonomy.go` |

### F15 — Double-stop ConsolidationWorker panic (historical bug #17)

| | |
|--|--|
| **Mitigation** | `stopOnce` around quit close |
| **Code** | `consolidation.go` |

### F16 — Campaign LLM calls serialized

| | |
|--|--|
| **Symptoms** | Parallel shards wait on HTTP idle conn limit |
| **Mitigation** | Shared transport MaxIdleConnsPerHost=64 |
| **Code** | `transport.go` |

## Severity matrix

| ID | Severity | User-visible? | Silent degrade? |
|----|----------|---------------|-----------------|
| F1 | High | Yes | Partial |
| F2 | High | Yes | Partial |
| F3 | Medium | Rare | **Yes** (fallback prompt) |
| F4 | Medium | Quality | **Yes** |
| F6 | High | Boot fail | No |
| F8 | High | Auth fail | No |
| F11 | Low | Delayed learning | **Yes** |

## First response checklist

1. Read Perception COMPLETE lines for last turn.  
2. Check TransientFailure / llm_unavailable path.  
3. Confirm engine/provider in DetectProvider logs.  
4. Confirm semantic classifier init.  
5. Confirm classification model, not main model, on hot path.
