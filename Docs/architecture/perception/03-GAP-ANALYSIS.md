# 03 — Gap Analysis (perception)

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
