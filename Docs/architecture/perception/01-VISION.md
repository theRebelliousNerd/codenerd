# 01 — Vision (perception)

> Last verified: **2026-07-13**

## Target product role

Perception is the **sensory cortex** of codeNERD:

- Hear the user in natural language (and ambient editor context).  
- **Describe** intent with high structure and confidence.  
- Ground description with **vector memory** and **Mangle vocabulary**.  
- Hand the executive kernel a clean `user_intent` (and optional routing facts).  
- Supply **reliable multi-provider LLM transport** for every other subsystem that needs a model.

It must **never** become the executive: no silent policy override of `permitted(...)`, no long-lived agent planning inside client factories.

## Target architecture

```
                    ┌─────────────────────────────┐
                    │   Ambient + history + strat │
                    └─────────────┬───────────────┘
                                  ▼
┌──────────────┐   exemplars   ┌──────────────────┐   Understanding   ┌─────────────┐
│ Embed + dual │──────────────►│ LLM classifier   │──────────────────►│ deriveRoute │
│ corpus search│               │ (fast tier)      │                   │ (Mangle)    │
└──────┬───────┘               └──────────────────┘                   └──────┬──────┘
       │ semantic_match facts                                                 │
       ▼                                                                      ▼
┌──────────────┐                                                     Intent + facts
│ Taxonomy /   │──────────────────────────────────────────────────► user_intent
│ verb corpus  │  (legacy / hybrid signals)
└──────────────┘
```

## Non-goals

- Fuzzy synonym banks as the primary NL understanding path (embeddings + LLM first).  
- Provider-specific product features that cannot map to `LLMClient` capabilities.  
- Replacing articulation’s Piggyback emission contract.  
- Embedding campaign orchestration logic (campaigns **use** clients; they do not live here).

## Success criteria

1. Every interactive turn produces a structured Intent with honest confidence / failure flags.  
2. Classification stays on a **cheap/fast** model by default.  
3. Kernel can consume routing facts without re-parsing NL.  
4. Provider swap (API key ↔ CLI ↔ OAuth) is config-level, not code-fork.  
5. Learning improves corpus without blocking the hot path.

## Evolution trajectory

| Phase | State |
|-------|--------|
| Verb-regex + taxonomy | Still present for hybrid/legacy |
| LLM-first Understanding | **Current canonical** |
| Fast classification tier | **Implemented** |
| Full Mangle vocabulary validation on every field | Partial (deriveRouting; validate dead on hot path) |
| Unified capability matrix per client | Partial / aspirational |

Related: [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md), [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md).
