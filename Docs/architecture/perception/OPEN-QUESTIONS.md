# OPEN QUESTIONS — perception

> Last verified: **2026-07-13**  
> Real open questions only (not fake “should we write docs?”).

## Q1 — Dual classification paths: how long both?

**Question:** Is the regex+taxonomy `matchVerbFromCorpus` path long-term, or should all production paths go exclusively through UnderstandingTransducer?

**Why it matters:** Two vocabularies (VerbEntry vs action_type mapping) can drift.

**Evidence:** Both exist; Understanding is documented as canonical in adapter comments.

## Q2 — Should vocabulary validation be hard-fail?

**Question:** Re-enable `validate()` against RoutingKernel and reject out-of-vocab Understanding fields, or keep soft normalize + harness defaults?

**Tradeoff:** Hard-fail increases clarification rate; soft path increases silent misroute risk.

## Q3 — Classification on subscription engines

**Question:** Should claude-cli / codex-cli / xai-oauth ever support a cheap classification mode (local model / haiku API key sidecar)?

**Current:** Factory returns nil → main engine classifies every turn (expensive/slow for CLI engines).

## Q4 — Global singletons vs injected instances

**Question:** Keep `SharedTaxonomy` / `SharedSemanticClassifier` or require explicit construction in Cortex boot only?

**Tradeoff:** Globals simplify transducers; break multi-workspace purity and tests.

## Q5 — Error contract for ParseIntent*

**Question:** Should `ParseIntentWithContext` start returning errors for non-degradable failures, or keep nil-error forever for chat stability?

**Current:** Explicit degraded Intent + nil error; TransientFailure as side channel.

## Q6 — Provider capability discovery

**Question:** Introduce a single `Capabilities() ClientCaps` method vs continue type asserts?

**Consumers:** shards, tools, streaming UI.

## Q7 — Learning critic model

**Question:** Should consolidation always use worker LLM (cheap) rather than main client on TaxonomyEngine?

**Current:** Depends on how TaxonomyEngine.client is set by boot (audit when changing learning quality).

## Q8 — Semantic MinSimilarity / LearnedBoost defaults

**Question:** Are 0.5 / 0.1 still correct after corpus growth, or should they be config.json knobs always set from UserConfig?

**Current:** `DefaultSemanticConfig` with SetConfig available.

## Resolved / closed (keep for history)

| Item | Resolution |
|------|------------|
| Stability bypass | **Removed** — misclassified short turns |
| Classification inherits main model | **Bug fixed** via ClassificationModel tiering |
| Taxonomy reload every ClassifyInput | **Fixed** one-shot schema load |
