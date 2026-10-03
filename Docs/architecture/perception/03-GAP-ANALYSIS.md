# 03 — Gap Analysis (perception)

## Accepted cold-classifier lifetime target (2026-10-02)

**VERIFIED CURRENT — bounded implementation/live slice:** strict constructor ownership/cancellation, cache preservation/retry, optional fallback and shared admission controls plus focused race pass (`artifact:.corpus-build/runs/all-features-20261002/round6-coldboot-corrected.receipt.json`, `round6-coldboot-race.receipt.json`). Fresh normal-entry timeout20s now exits21.6602s and preserves448 cache rows with integrity OK, before later prompt initialization. This supersedes the contextless production constructor observation below. The learned-store backend constructor still has contextless SQL and requires a separate store-owned spec/packet; complete suites, global isolation and model-turn/world-model proof remain open.

**PARTIAL — GAP-PERCEPTION-BOOT-CONTEXT:** `transducer.go#InitPerceptionLayer`, `semantic_classifier.go#InitSemanticClassifier` and `#NewSemanticClassifierFromConfig` lose the calling boot context; cold hydration uses a Background-derived cap. The root normal-entry 20-second probe continued hydration and exited after 58.4495 seconds. Evidence: `artifact:.corpus-build/runs/all-features-20261002/02-execution/handoff-5.json`.

**PROPOSED UPLIFT:** introduce context-aware construction and keep compatibility wrappers. Derive optional hydration limits from the caller. Parent cancellation aborts construction and prevents shared-classifier publication; an optional local cap/service failure with a live parent may retain existing degraded behavior. Cache admission/traversal, SQL operations and embedding requests observe cancellation. Close constructor-owned handles on failure, preserve completed cache writes, and permit a later healthy retry. Do not introduce a detached hydration goroutine, change routing/permissions or discard the cache. Exit: real blocked embedding HTTP cancellation, typed parent cancellation, local-cap/outage fallback, partial-cache reopening/retry, constructor ownership and shared-publication controls, plus the system normal-entry discriminator. Shared-context isolation remains separately open.

## Accepted JIT envelope target (2026-10-02)

**PARTIAL — GAP-PERCEPTION-JIT-CONTRACT:** `internal/perception/understanding_adapter.go#UnderstandingTransducer.getSystemPrompt` compiles perception at line 117 but `#isValidUnderstandingPromptContract` at line 135 rejects any occurrence of legacy field names, including legitimate nested scope.target from `internal/prompt/atoms/system/perception.yaml#system/perception/output_format`. Root inspected the source; artifact:.corpus-build/runs/all-features-20261002/03-prompt/handoff-2.json records the normal-entry fallback observation. The July narrative below is historical and is not current implementation evidence.

**PROPOSED UPLIFT:** the actual perception model request uses the canonical UnderstandingEnvelope JIT atoms, retains legitimate nested fields and methodology, and excludes competing Piggyback envelopes. Validate the owning output schema rather than blacklisting words anywhere in the complete prompt. Expose bounded fallback cause/actual contract diagnostics without leaking full prompts. Preserve intentional fallback on genuinely missing, malformed or conflicting contracts. Root tests require actual production assembly and captured model requests plus missing/conflicting negative controls; a mocked valid string or successful compile alone cannot close the gap. No routing/permission changes or new control fields are part of this packet. Companion GAP-PROMPT-PERCEPTION-CONTRACT owns atom selection. Accepted, not yet implemented or verified.

The dependency contract also requires `perception/understanding.yaml` schema and examples to nest `signals` and `suggested_approach` under `understanding`. These fields remain meaningful typed input, not extra top-level envelopes. Capture a real full-corpus request and validate that shape without weakening missing/malformed/conflicting controls.

**VERIFIED CURRENT — bounded production-bridge receipt:** `artifact:.corpus-build/runs/all-features-20261002/round4-perception.receipt.json` records a passing `TestPerceptionJIT_` gate on the modified checkout. Real compiler/assembler/transducer request captures cover canonical perception/firewall contracts, dependency closure, missing/malformed/conflicting fallback, planner campaign phases and ordinary conversational Piggyback. Fixtures use the production campaign context shape; no control assertion was weakened. This supersedes the accepted-not-implemented sentence for these named behaviors only. Full-package, shipped-seed parity, fresh binary and live provider/world-model consumption remain separate unclosed obligations.

Focused production-bridge race controls also pass in `artifact:.corpus-build/runs/all-features-20261002/round4-world-perception-race.receipt.json`; this is not full-package race or live-provider qualification. The subsequent shared reconciler correction requires fresh gates before claiming its dependent paths qualified.

> Last verified: **2026-07-13**
> Compare vision/north star vs **actual code** in `internal/perception/`.

## Spec vs reality matrix

| Aspiration | Reality | Gap? |
|------------|---------|------|
| LLM describes, harness decides | `deriveRouting` + Intent facts; LLM suggestions overridable | **Non-gap** |
| Fast path classification | `NewClassificationClientFromConfig` | **Non-gap** (watch nil fallback to main model) |
| Semantic + Mangle grounding | Dual stores + inject + taxonomy | **Partial** when embed boot fails |
| JIT classification prompts | Assembler + contract check + embedded fallback | **Partial** (string snippet contract) |
| Uniform LLMClient capabilities | Base interface thin; tools/stream/schema optional | **Gap** (callers type-assert) |
| Full field vocabulary validation | `validate()` exists but unused on hot Understand path | **Gap / intentional dead code** |
| Piggyback for intent | UnderstandingEnvelope is classification contract | **Non-gap** (evolution; Piggyback for emission) |
| Never block chat on learning | ConsolidationWorker drop-on-full | **Non-gap** |
| Honest outage messaging | TransientFailure + clarification path | **Non-gap** (Gemini-origin sentinel; others may not wrap) |
| Multi-workspace taxonomy isolation | Shared globals + SetWorkspace | **Partial** |
| Provider README accuracy | Operator README somewhat stale | **Doc gap** (package README, not arch corpus) |
| Complete e2e for every provider | Mock-heavy + gated live | **Partial** |
| Nil provider construction | `NewClientFromConfig` rejects nil with a tested error | **Closed 2026-07-13** |

## Priority ranking

### P0 — correctness / safety

| Item | Notes |
|------|-------|
| Ensure all durable 5xx paths wrap `ErrLLMUnavailable` | Gemini does; audit ZAI/OpenAI/Anthropic/CLI |
| Keep fact sanitization on every ToFact path | Covered for Intent; watch new fact writers |
| Config-is-boss no silent provider swap | Already in factory; preserve under new engines |

### P1 — product latency / quality

| Item | Notes |
|------|-------|
| Always wire classification client in boot | Avoid accidental main-model classification |
| Ensure SharedSemanticClassifier init when embeddings available | Silent nil loses neuro-symbolic path |
| Align JIT understanding atoms with `isValidUnderstandingPromptContract` | Prevent silent fallback |

### P2 — architecture cleanliness

| Item | Notes |
|------|-------|
| Capability interface matrix (ToolsClient, SchemaClient, StreamClient) | Reduce type asserts |
| Remove or re-enable `validate()` with clear policy | Dead code confuses readers |
| Unify dual classification paths documentation in runtime | When is corpus path still called? |

### P3 — polish

| Item | Notes |
|------|-------|
| Refresh `internal/perception/README.md` defaults | Match factory models |
| Metrics export beyond process map | Hook observability package |
| Multi-workspace SharedTaxonomy isolation | Tests vs concurrent workspaces |

## Wave-3 closures (2026-09-25)

- P0 "all durable 5xx paths wrap ErrLLMUnavailable": superseded by the shared
  provider-failure contract (`provider_failure.go`), which classifies every
  HTTP adapter's failures; the degraded turn reads the class.
- P2 "remove or re-enable `validate()`": removed from Go; the vocabulary check
  is a kernel derivation (`perception_routing.mg` `understanding_vocab_miss`),
  consumed by `RealKernelRouter.VocabularyMisses` onto `Routing.VocabularyMisses`.
- P2 "when is the corpus path still called?": `matchVerbFromCorpus` is reached
  only by `DebugTaxonomy` (cmd/tools/verify_taxonomy); the regex NLU helpers,
  `DualPayloadTransducer`, the pull-style `ShardTraceStore` and the second
  `MangleRoutingKernel` adapter were removed as superseded.
- Factory: OpenAI-compatible vendors dispatch from the vendor table alone.

## Explicit non-gaps

- “No code exists” — **false**; large mature package.  
- “Only regex NLU” — **false**; LLM-first is canonical.  
- “No multi-provider” — **false**; seven API + three engines.  
- “Learning blocks OODA” — **false**; async worker with drop.  
- “Constitutional deny inside HTTP client” — **not required** by north star; kernel owns permitted.

## Debt register (honest)

1. Dual mental model: Understanding vs VerbEntry taxonomy.  
2. Optional interfaces explosion without formal capability discovery.  
3. Global singleton classifiers complicate pure DI.  
4. Live tests depend on secrets / network; CI may skip real coverage.

The authoritative implementation contracts for these gaps are the feature cards
in [TODO](TODO.md). In particular, outage normalization is not closed merely by
Gemini's sentinel, and optional Go interfaces are not yet a typed capability or
workspace-ownership contract.

See [TODO.md](TODO.md) for actionable backlog.

### Learned-corpus startup context reaches SQL

The cold-bootstrap cancellation requirement includes the actual writable learned
corpus, not just the wrapper in `semantic_classifier.go`. Its context-aware
constructor must call a context-aware store backend through readiness, profile,
and schema operations. Couple GAP-STORE-LEARNED-CONSTRUCTOR-CONTEXT to this route:
parent cancellation escapes optional degradation, closes owned resources, and
does not publish a successful classifier. Prove actual blocked SQL cancellation,
drain, preserved existing data, and successful retry alongside existing coldboot
and optional-outage controls.
