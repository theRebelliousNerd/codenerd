# 04 — Architectural Principles (perception)

> Last verified: **2026-07-13**  
> Binding principles **specific to this package**. Violate only with explicit design notes.

---

### P1 — LLM describes; harness determines

Understanding fields are **suggestions**. Routing (`Mode`, shards, tool/context priorities) is derived by the harness/Mangle (`deriveRouting`). Never treat model output as authorization.

### P2 — Classification is a critical path; keep it cheap

Every interactive turn classifies. Use `NewClassificationClientFromConfig` fast tiers. **Do not** default classification to the user’s large main model.

### P3 — Prefer structured contracts over free text

Intent classification expects `UnderstandingEnvelope` / `Understanding` JSON. Reject JIT prompts that reintroduce Piggyback control_packet or legacy category/verb-only schemas for this path.

### P4 — Graceful degradation, honest signalling

Optional subsystems (embeddings, semantic store) may fail open. LLM outages must not be labelled as user ambiguity (`TransientFailure` / `ErrLLMUnavailable`). Prefer degraded Intent + nil error where the call contract requires it.

### P5 — Config is boss

When a provider is configured, missing keys error out. No silent env fallback that swaps engines under the user.

### P6 — Sanitize before Mangle

Any user- or model-derived string entering fact arguments passes length limits and control-character stripping (`sanitizeFactArg`).

### P7 — Never block the interactive loop on learning

Consolidation / critic work is async; full queues drop work with a warning.

### P8 — Provider modularity via factory

New backends register through engine/provider switches in `client_factory.go` (or xaioauth). Chat and shards depend on `LLMClient`, not concrete HTTP shapes.

### P9 — Schema single source of truth for Piggyback

Piggyback structured-output schemas must parse from `articulation.PiggybackEnvelopeSchema` (see `client_schema.go`), not a second hand-maintained copy.

### P10 — Parallel transport for parallel work

Shared HTTP transport pools must support campaign-scale concurrent calls; do not reintroduce default `MaxIdleConnsPerHost=2` assumptions.

### P11 — Taxonomy isolation

Taxonomy engine uses **its own** Mangle instance and `learned_taxonomy.mg`. Do not load the Cortex kernel’s general `learned.mg` into taxonomy.

### P12 — JIT-first for new LLM-facing prompt behavior

New classification/system prompt behavior lands as prompt atoms + assembler selection, with embedded fallback only as safety net (`internal/prompt/README.md` documents the live atom workflow).

---

## Anti-principles (do not)

- Grow synonym regex banks as primary NLU.  
- Assert untrusted strings into Mangle without sanitization.  
- Put campaign/product-specific verbs only in perception without general kernel schema.  
- Type-assert provider secrets into logs.
