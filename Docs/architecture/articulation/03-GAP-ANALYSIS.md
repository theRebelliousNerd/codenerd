# 03 — Gap Analysis: articulation

> Last verified: 2026-07-13  
> Gaps are **spec/vision vs code + wiring**, not “no code exists.”

## Matrix

| Area | Vision / intent | Reality | Gap? | Priority |
|------|-----------------|---------|------|----------|
| Dual-channel parse | Always structured envelope | Full pipeline + fallback | **Partial** — fallback loses control | P2 ops |
| Thought-first ordering | Control before surface | Struct tags + prompt suffix; streaming parser waits for surface key | **Non-gap** for design; models still violate occasionally | P3 |
| Truncation UX | Never show raw control | `looksLikePartialEnvelope` salvage | **Non-gap** (implemented) | — |
| JIT default | All prompts JIT | JIT first; multi-level fallback | **Minor** — legacy templates remain | P3 |
| Mangle injection defense | Only safe atoms reach kernel | Syntax/metachar/size filters in `applyCaps` | **Partial** — not full semantic/policy check | P1 shared w/ core |
| Constitutional override | Kernel-mandated rewrites | Helper mutates surface + filters atoms | **Partial** — few call sites; reason string only | P2 |
| Memory operations | Cold storage directives | Parsed + helpers; application is caller | **Wiring gap outside package** | P2 |
| Context feedback | Learning signal every turn | Typed + schema; application caller-side | **Wiring gap outside package** | P2 |
| Knowledge requests | Specialist re-entry | Typed; chat implements re-invoke | **Partial** path coverage | P2 |
| Tool requests | Structured tools vs native FC | Session executor path live | **Non-gap** for protocol; provider variance | P3 |
| Strict schema mode | Provider-guaranteed shape | Schema constant + optional `RequireValidJSON` | **Partial** — default is tolerant | P3 |
| StreamParser concurrency | Safe multi-writer | Single-threaded design | **Documented gap** in test TODO | P3 |
| Processor stats export | Ops dashboards | In-memory `GetStats` only | **Gap** — no persistence/metrics wire | P3 |
| Package README accuracy | File map complete | README omits scanner/stream/adapter | **Doc gap** | P4 |

## Priority definitions

| Pri | Meaning |
|-----|---------|
| P1 | Safety / injection correctness |
| P2 | Product completeness / silent feature loss |
| P3 | Hardening / ergonomics |
| P4 | Docs polish |

## Detailed gaps

### G1 — Fallback strips executive channel

When parse method is `fallback`, `ProcessLLMResponse` sets `Control = nil`. Chat still shows text, but no `mangle_updates`, tools, or feedback for that turn.

**Mitigation today:** error-level log when `LogFallbackAsError`; salvage surface for partial envelopes.  
**Desired:** higher structured recovery (repair models) or automatic re-prompt with schema.

### G2 — Semantic safety of mangle_updates is not articulation’s job alone

`applyCaps` rejects obviously bad strings; it does **not** prove `Decl` existence or `permitted` for resulting effects. Session executor applies additional blocking before assert.

**Non-gap if** callers always re-validate. **Gap if** a new caller asserts raw updates without the session path.

### G3 — Memory ops and context feedback application

Types and protocol are complete. Durable application lives outside this package. Risk: fields appear in envelopes and logs without changing long-term memory or retrieval weights.

### G4 — Constitutional override under-used

`ApplyConstitutionalOverride` is tested and used from session mangle filtering; it is not a universal post-processor on every chat surface path. Surface safety notices may be inconsistent across entry points.

### G5 — Hard-coded shard fallback templates

`coderFallbackTemplate` etc. duplicate identity that should live in atoms. Acceptable emergency path; drift risk vs JIT atoms.

### G6 — Stats not operationalized

`ProcessorStats` exists but is not known to be scraped into glass-box / telemetry products. Operators infer health from log lines instead.

## Non-gaps (do not “fix” as missing features)

| Claim | Why it is not a gap |
|-------|---------------------|
| “No streaming support” | `StreamParser` + chat streaming integration |
| “No schema” | `schema.go` full draft-07 schema |
| “No decoy defense” | Last-match-wins embedded extraction + tests |
| “No JIT” | `NewPromptAssemblerWithJIT` / factory wiring |
| “No tests” | Extensive unit/boundary/fuzz suite |
| “Pre-implementation 0%” | ~3.2k LoC production package |

## Recommended sequencing (no time estimates)

1. Ensure **all** mangle assert paths share the same filter + constitutional gate (core/session).  
2. Wire **context_feedback** and **memory_operations** end-to-end or explicitly demote them in protocol docs.  
3. Reduce fallback rate (schema-capable clients, tighter prompts).  
4. Retire or atomize hard-coded templates.  
5. Expose parse stats to glass-box / session metrics.
