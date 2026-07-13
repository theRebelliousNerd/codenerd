# 09 — Safety and Invariants (perception)

> Last verified: **2026-07-13**

## Constitutional boundary

Perception **does not** implement `permitted(...)`. It produces facts and descriptions. The Cortex policy corpus enforces default-deny on actions. Do not “secure” the system by blocking HTTP inside Gemini/ZAI unless product requires network egress policy elsewhere.

## Invariants

### I1 — Fact argument hygiene

Any string from user/LLM that becomes a Mangle fact argument is length-capped and stripped of injection-prone control characters (`sanitizeFactArg` in `transducer.go`).  
**Tests:** `break_test.go` Mangle injection / control char / UTF-8 truncation cases.

### I2 — Degraded parse never panics the chat loop

`ParseIntentWithContext` returns a usable `Intent` (often `/explain`) with **nil error** when classification fails, unless a lower layer panics (should not).  
Transient provider failures set `TransientFailure=true` so firewall asserts `/llm_unavailable` not `/heuristic_low`.

### I3 — Config honesty

If user selects a provider without a key, factory errors. No silent swap to another vendor’s key.

### I4 — Learning never blocks OODA

`ConsolidationWorker.Enqueue` is non-blocking; full queue drops with warn log. Stop is idempotent (`stopOnce`).

### I5 — Input size bounds

| Stage | Bound |
|-------|------:|
| ParseIntentWithContext | 50_000 chars |
| ClassifyWithoutInjection | 32_768 bytes |
| Regex candidates | 2_000 chars |
| Fact args | 2_048 chars |
| JSON candidates retained | 1_000 spans |
| Intent hydrate timeout | 60s |
| Consolidation learn timeout | 2m |

### I6 — Taxonomy isolation

Do not load Cortex `learned.mg` into TaxonomyEngine. Only `learned_taxonomy.mg`.

### I7 — Concurrent corpus access

Verb corpus mutations go through `GetVerbCorpus` / `SetVerbCorpus`. Break tests include data-race scenarios.

### I8 — Shared transport safety

Process-wide `http.Transport` is immutable after init; clients create `http.Client` with timeouts via `NewSharedHTTPClient`.

### I9 — Classification model independence

`ClassificationModel` never silently inherits `Model`. Missing fast tier → nil client, not wrong model assignment.

### I10 — JIT prompt contract for understanding

JIT prompts for perception-transducer must include Understanding fields and must **not** include Piggyback control_packet or legacy category/verb-only schema fields.

## Concurrency risks

| Resource | Protection |
|----------|------------|
| verbCorpus | RWMutex |
| TaxonomyEngine | Mutex |
| Semantic stores | RWMutex |
| SharedSemanticClassifier init | sharedClassifierMu |
| Tracing context | Mutex |
| metrics map | Mutex |
| Consolidation quit | sync.Once |

## Trust boundaries

| Boundary | Trust assumption |
|----------|------------------|
| User NL | Untrusted → sanitize before facts |
| LLM JSON | Untrusted structure → parse + normalize + defaults |
| Config file | Trusted user machine config |
| CLI subprocess stdout | Semi-trusted; parse carefully (Claude/Codex) |
| OAuth store | Local secret; protect file perms OS-side |
| Embedding model | Integrity of vectors affects ranking only, not authz |

## Logging safety

Perception logs truncate inputs for display (`truncateForLog` / rune-safe). Avoid logging full API keys. Prefer model names and lengths.

## What is deliberately out of scope

- File write authorization  
- Network allowlists for tools  
- Prompt injection defenses for **downstream** tool-using agents (shard prompts / JIT atoms)

Those belong to kernel policy, tools, and articulation layers.
