# 09 — Safety and Invariants (perception)

## Compatible streaming setup uncertainty

Compatible Chat setup transport errors may hide received inference requests.
Notify enclosing incomplete observers before retry/cancellation; later stream
success retains known usage but cannot resolve the earlier unknown cost. The
stream may still finish successfully: answer completion and billing certainty
are distinct. Preserve content, typed cancellation, cache counts, HTTP errors,
retry bounds and the existing rule that a started stream is not replayed.
Require actual local-server broker and mission stop/clean controls. This is
conservative uncertainty, not an assertion that every connect error was billed.

## Compatible and Responses transport uncertainty

A lost response in compatible Chat, shared non-streaming Responses or explicit
grounded search may hide endpoint work. Notify existing incomplete captures at
the HTTP transport-error boundary before retry or cancellation. Known reports
remain lower bounds; later success cannot make the missing exchange free. Do
not invent a token report. Keep error identity and the grounded transport's
sanitized messages intact: uncertainty signaling must not expose vendor text,
keys or new payloads. Clean responses remain certain. Actual local-server,
broker and real-kernel mission controls must cover all three funnels, with
cancellation, bounded exhaustion and known-budget positive controls. Other
transports and non-200 billing classification are outside this bounded target.

## Anthropic transport-error uncertainty

A failed Messages transport exchange may have reached the endpoint and lost
its response. Notify existing incomplete observers before retry or cancellation
returns; later success cannot establish that the failed exchange was free.
Keep its cost unknown rather than inventing counts, and preserve every known
report. This conservative boundary does not assert that every transport error
was billed. Clean success remains certain; retries, cancellation identity,
stream content and existing mission policy retain their behavior. Verify with
an actual local server that reads the inference body before closing without
headers, plus broker receipts and real-kernel mission stop/clean controls.

## Anthropic decoded-attempt accounting

Observe each decoded non-streaming Messages attempt exactly once before
validating its envelope, including incomplete messages and paused turns.
Missing token metadata notifies enclosing incomplete observers; known counts
remain a lower bound. A send/read/decode failure must retain uncertainty even
after earlier reports. Do not also report the accepted exchange's aggregate.
Keep its content, returned usage and pause/resume limits intact, including
refusal before sending an inadmissible continuation and typed cancellation.
Streaming keeps its own accounting boundary. Verify exact report counts through
the broker and tracker, plus the mission kernel's existing unknown-spend and
spend stops with clean positive controls. This contract does not change retry
limits, permissions, recovery, model selection or provider billing policy.

## Accepted response read failures

An unreadable HTTP 200 body is an accepted attempt with unaccounted usage.
Notify the existing incomplete-usage observer before retry or cancellation
returns; a successful retry cannot restore certainty about the failed read.
Preserve known reports, retry limits, typed cancellation and body closure. This
contract covers the compatible chat transport's accepted-response read boundary;
it does not classify pre-connect failures or other transports as billed or free.

## Retry usage uncertainty

Each completed provider attempt without token metadata must notify enclosing
`usage.IncompleteObserver` captures before an empty usage report is discarded.
Reports from other attempts do not resolve that uncertainty. Preserve known
counts, category-presence semantics and normal answer/retry behavior; do not
invent zero-cost reports. The adapter supplies observations and the mission's
existing kernel policy decides whether further work may run.

The regression must exercise the existing reasoning-only retry through the
broker, nested observers and tracker, plus mission admission with the real
kernel. A fully reported retry is the positive control; observed zero remains
distinct from missing metadata. Execution evidence is required before closing
GAP-PERCEPTION-RETRY-USAGE.

## Accepted constructor cancellation invariant

**PROPOSED UPLIFT:** every context-aware classifier constructor distinguishes parent cancellation from optional service failure/local hydration cap. Failed or canceled constructors release owned stores/engines and never publish a partial shared classifier. Completed cache writes remain reusable; no timer or detached goroutine stands in for joining work. Compatibility constructors may use Background only when no caller context exists. GAP-PERCEPTION-BOOT-CONTEXT owns the pending exit controls.

> Last verified: **2026-07-13**

## Constitutional boundary

Perception **does not** implement `permitted(...)`. It produces facts and descriptions. The Cortex policy corpus enforces default-deny on actions. Do not “secure” the system by blocking HTTP inside Gemini/ZAI unless product requires network egress policy elsewhere.

## Invariants

### I1 — Fact argument hygiene

Any string from user/LLM that becomes a Mangle fact argument is length-capped and stripped of injection-prone control characters (`sanitizeFactArg` in `transducer.go`).  
**Tests:** `break_test.go` Mangle injection / control char / UTF-8 truncation cases.

### I2 — Degraded parse never panics the chat loop

`ParseIntentWithContext` returns a usable `Intent` (often `/explain`) with **nil error** when classification fails, unless a lower layer panics (should not).  
A failed classification carries `Intent.Outcome`; the firewall asserts `classification_outcome`, never `/heuristic_low`, and the kernel derives the disposition (`perception_outcome.mg`).

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
