# Perception: turning a request into grounded intent

> Corpus: `perception` | Live owner: `internal/perception` | Verified: 2026-07-13

## In one minute

Perception is codeNERD's sensory ingress. It turns a user's natural-language
request into a structured `Understanding`, derives a kernel-facing `Intent`, and
can enrich that decision with semantic examples and a learned verb taxonomy. The
visible outcome is that “review this API for authorization bugs” becomes a typed
review/security request with target, scope, confidence, routing suggestions, and
an explainable degraded result when the model is unavailable.

`VERIFIED CURRENT` — the canonical interactive route is
`internal/perception/understanding_adapter.go#UnderstandingTransducer.ParseIntentWithContext`.
It invokes `internal/perception/transducer_llm.go#LLMTransducer.Understand`, maps
the result to `internal/perception/transducer.go#Intent`, and the latter emits
`user_intent` through `internal/perception/transducer.go#Intent.ToFact`.

## Its place in codeNERD

The LLM is the creative interpreter: it proposes semantics, scope, and an
approach. Perception's Go harness normalizes and sanitizes that proposal. Mangle
remains the executive: perception does not authorize tools, mutate files, or
replace `permitted(Action, Target, Payload)`.

```text
user text + recent conversation + workspace context
  -> semantic exemplars (optional) + Understanding prompt
  -> LLM proposes Understanding
  -> Go normalizes fields and derives routing with Mangle affinities
  -> Intent.ToFact -> user_intent(...)
  -> kernel derives next_action and permitted/3
  -> VirtualStore/shard effect -> articulation -> user
```

The package also owns provider clients used beyond classification. That is a
large transport surface, but it does not make perception the owner of session
execution, prompt policy, or constitutional safety.

## A representative journey

Suppose a user says, “Assault the parser with malformed Unicode, but do not edit
production code.”

1. `UnderstandingTransducer.ParseIntentWithContext` caps input size and asks the
   optional `SharedSemanticClassifier` for nearby examples.
2. `LLMTransducer.Understand` builds the classification request, parses a bounded
   JSON object, normalizes vocabulary, and derives routing. Mangle affinity rules
   may override the model's suggested shard or mode.
3. The adapter maps the result to an `/assault` `Intent`; `Intent.ToFact` strips
   Mangle-significant characters and emits a bounded `user_intent` fact.
4. The core kernel decides what may happen. The “do not edit” constraint is not
   merely trusted as prose; downstream permission policy must deny mutations.
5. If the model has a durable transient failure, the adapter preserves its
   nil-error compatibility contract but sets `TransientFailure`, allowing the
   session firewall to explain the outage instead of treating it as ambiguity.

`VERIFIED CURRENT` — hostile fact arguments are exercised by
`internal/perception/break_test.go#TestBreak_SanitizeFactArg_MangleInjection`;
transient degradation is exercised by
`internal/perception/understanding_adapter_transient_test.go#TestParseIntentWithContext_TransientFailureMarksIntent`.

## What exists today

| Applicability lane | Evidence-backed answer |
|---|---|
| Mangle | `VERIFIED CURRENT` — perception produces `user_intent` and routing facts through `Intent.ToFact` and `LLMTransducer.assertRoutingFacts`; taxonomy classification uses a dedicated `mangle.Engine` in `internal/perception/taxonomy.go#TaxonomyEngine.ClassifyInput`. It consumes embedded intent/taxonomy declarations; it does not own `permitted/3`. |
| Permission and safety | `VERIFIED CURRENT` — `sanitizeFactArg` bounds and cleans fact arguments, provider configuration refuses unknown engines/providers, and a nil `ProviderConfig` now fails closed. `PARTIAL` — durable 5xx outage classification is explicit for Gemini but not proven uniform across every provider. |
| Fact flow | `VERIFIED CURRENT` — text enters `ParseIntentWithContext`, becomes `Understanding`, then `Intent`, then `user_intent`; kernel/session/action/articulation are downstream owners. The legacy regex/taxonomy route remains available as a degraded or alternate classifier. |
| JIT and agents | `PARTIAL` — the adapter accepts a `PromptAssembler` and validates the Understanding prompt contract before use, with an embedded fallback. Classification-client tiering is live. The provider-capability matrix remains implicit in Go type assertions rather than a typed JIT capability receipt. |
| Wiring | `VERIFIED CURRENT` — config detection constructs API, Claude CLI, Codex CLI, or xAI OAuth clients; chat/session boot constructs the transducer; semantic classification and taxonomy are process-shared optional enrichments. `PARTIAL` — globals complicate simultaneous workspace isolation. |
| State and concurrency | `VERIFIED CURRENT` — verb corpus access is guarded and the consolidation queue is bounded/drop-on-full; `internal/perception/transducer_unit_test.go#TestVerbCorpus_ConcurrentAccess_ShouldNotRace` covers the corpus. `PARTIAL` — taxonomy workspace and process-global classifier lifecycle are not an explicit per-session ownership model. |
| Recovery | `VERIFIED CURRENT` — embedding failure degrades to non-semantic classification, learning never blocks chat, transient Gemini failures carry `ErrLLMUnavailable`, and OAuth can loudly fall back when configured. `PARTIAL` — outage identity is not normalized across all provider transports. |
| Observability | `VERIFIED CURRENT` — perception stage timing, bottleneck labels, process LLM metrics, reasoning traces, and auth probes exist. `PARTIAL` — no durable turn receipt joins semantic candidates, LLM proposal, normalization, derived routing, final intent, and later kernel outcome. |
| Testing | `VERIFIED CURRENT` — unit, adversarial, race-oriented, benchmark, provider mock, taxonomy persistence, and gated live tests exist. `go test -count=1 -timeout=240s ./internal/perception/...` passed on 2026-07-13. `PARTIAL` — real provider coverage remains secret/network gated. |

The deep inventory and contracts are in [Current State](02-CURRENT-STATE.md) and
[Implemented Spec](IMPLEMENTED_SPEC.md). The current package includes a broad
multi-provider client layer, LLM-first transduction, semantic classification,
Mangle taxonomy inference, asynchronous learning, tracing, and OAuth/CLI engines.

## North star

Perception should be a deterministic, inspectable firewall around creative model
interpretation: every turn produces a versioned receipt showing the context and
semantic evidence considered, the model proposal, harness normalization, Mangle
routing derivation, confidence/degradation status, and the final fact handed to
the executive. Identical typed inputs and model output should produce identical
facts; concurrent workspaces must not share mutable taxonomy state accidentally.

Non-goals:

- Perception never authorizes an effect or weakens default deny.
- Mangle is not used for fuzzy natural-language matching; embeddings/LLMs propose
  candidates, then typed facts enter logic.
- Provider clients do not decide routing policy merely because they transport the
  classification call.
- A model suggestion is evidence, not executive truth.

## Improvement frontier

1. `VERIFIED CURRENT` — reject nil provider configuration explicitly instead of
   panicking; the regression is
   `internal/perception/client_factory_test.go#TestNewClientFromConfig_NilConfig`.
2. `PROPOSED UPLIFT` — normalize retry exhaustion and durable provider outages
   into typed transient/permanent/auth/rate-limit classes, then prove each adapter
   against one shared conformance suite.
3. `PROPOSED UPLIFT` — replace optional-interface discovery with a typed provider
   capability receipt consumed by JIT prompt selection and session wiring. The
   receipt describes capabilities; it never grants permission.
4. `PROPOSED UPLIFT` — give taxonomy, semantic stores, caches, and learning queues
   explicit workspace/session ownership and teardown contracts.
5. `PROPOSED UPLIFT` — persist a redacted perception decision receipt correlated
   with the later prompt, permission decision, effect, and articulated response.
6. `DEFERRED` — use side-effect-free shadow transduction to compare candidate
   prompts/models/taxonomies on recorded redacted requests before promotion.

Machine-readable contracts, acceptance tests, and rollback are in [TODO](TODO.md).

## Choose a reading route

| Time | Route |
|---|---|
| 90 seconds | This README, then [Current State](02-CURRENT-STATE.md) and [Gap Analysis](03-GAP-ANALYSIS.md). |
| 10 minutes | [Internal Architecture](05-INTERNAL-ARCHITECTURE.md), [Wiring](08-WIRING-AND-INTEGRATION.md), [Safety](09-SAFETY-AND-INVARIANTS.md), and [Failure Modes](12-FAILURE-MODES.md). |
| Deep implementation | [Implemented Spec](IMPLEMENTED_SPEC.md), [Public API](06-PUBLIC-API-AND-TYPES.md), [Dependencies](07-DEPENDENCY-MAP.md), and [Testing](10-TESTING-ALIGNMENT.md). |
| Build or review an uplift | [Vision](01-VISION.md), [Gap Analysis](03-GAP-ANALYSIS.md), [TODO](TODO.md), [Open Questions](OPEN-QUESTIONS.md), then [_progress](_progress.md). |
