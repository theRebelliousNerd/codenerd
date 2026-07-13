# 03 — Gap Analysis: Context (`internal/context`)

> Last verified against codebase: 2026-07-13  
> Status: Living gap matrix — honest partials

## 1. Spec vs reality

| Capability | Spec / comment intent | Reality | Gap? |
|------------|----------------------|---------|------|
| Infinite context via atoms | Discard surface; retain logic | `CompressedTurn` has no surface text; chat switches on `IsCompressionActive` | **Low** — works when compressor wired |
| 100:1 compression | `TargetCompressionRatio: 100` | Heuristic token ratio; simple summary may not hit 100:1 | **Medium** — target is soft |
| Spreading activation | Energy through fact graph | Implemented + SpreadFromSeeds | **Low** |
| Kernel-primary scoring | NERD-EVOLVE replace 9-component Go | Hybrid: kernel if `should_include_context` non-empty, else Go | **Medium** |
| Observation masking C3 | Mask old observations | Age categories asserted; summary path does not fully query `should_mask_observation` for selective field drop | **Medium** |
| LLM summarization | `generateSummary` present | `compress()` uses `generateSimpleSummary` | **Non-gap** if intentional; document as design choice |
| Constitutional core | Always include safety facts | `getCoreFacts` with warn on Query error | **Low** |
| Corpus SSOT priorities | predicate_corpus.db | Loaded at boot; fallback hardcoded map | **Low** |
| Feedback loop | Learn useful predicates | Store + scoring; min 10 samples | **Low** (cold start) |
| Concurrent safety | No map races | Mutex + race test | **Low** |
| Accurate tokens | Budget management | 4 chars/token estimate | **Medium** for hard provider limits |
| JIT activation scores | Feed prompt compiler | `GetActivationScores` exported; wiring must be audited per consumer | **Partial** |
| Package README accuracy | Living docs | Defaults still mention 128k / Dec 2024 | **Low** (docs debt) |

## 2. Priority backlog (from gaps)

| Priority | Item | Why |
|----------|------|-----|
| P1 | Complete kernel scoring path coverage | Reduce dual-system drift |
| P1 | Ensure every chat path honors compression once active | Avoid raw history leaks |
| P2 | Wire / verify observation mask consumption in Go | C3 schema exists |
| P2 | Optional real tokenizer plug-in | Hard limits closer to provider |
| P3 | Align package README with `DefaultConfig` | Operator confusion |
| P3 | Publish inclusion-source metrics (kernel vs Go) | Observability |

## 3. Non-gaps (do not “fix”)

| Apparent issue | Why not a gap |
|----------------|---------------|
| “No Mangle in package dir” | By design; rules live under core defaults |
| “LLM summary unused” | C3 prefers atom summary without extra LLM cost |
| “Activation threshold high (105)” | Intentional: filters recency-only facts |
| “Feedback score 0 with few samples” | minSamples=10 prevents noisy early bias |
| “Issue weights capped” | Safety against context domination |

## 4. Cross-system gap risks

| Risk | Packages | Mitigation |
|------|----------|------------|
| ProcessTurn async lag | chat process vs BuildContext | Accept eventual consistency; document |
| `should_include_context` empty early | core policy load | Fallback Go path |
| Compressed state not rehydrated | session_persistence | Audit LoadState call sites |
| Feedback DB open fail | boot | Warn + continue without feedback |

## 5. Related

- [TODO.md](TODO.md)  
- [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md)  
- [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) §19 honesty notes  
