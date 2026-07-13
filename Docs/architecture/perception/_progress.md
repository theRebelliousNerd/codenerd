# _progress — perception architecture corpus

| Date | Action |
|------|--------|
| **2026-07-13** | Full rebuild to SUBAGENT_INSTRUCTIONS + cli quality bar. Research: listed `internal/perception/` (~50 non-test Go + ~48 tests + xaioauth), read transducer/understanding/semantic/factory/clients/taxonomy/learning/tracing/transport sources, grepped exports and reverse deps. Replaced thin inventory stubs with dense narrative corpus. Flagship: `IMPLEMENTED_SPEC.md` (transducer, semantic classifier, LLM clients). |
| 2026-07-13 (earlier) | Thin auto-inventory stubs (domain-model naming) — **superseded**. |

## Canonical file set (this rebuild)

- README.md  
- IMPLEMENTED_SPEC.md  
- 00-ALIGNMENT-VISION-REVIEW.md  
- 01-VISION.md  
- 02-CURRENT-STATE.md  
- 03-GAP-ANALYSIS.md  
- 04-ARCHITECTURAL-PRINCIPLES.md  
- 05-INTERNAL-ARCHITECTURE.md  
- 06-PUBLIC-API-AND-TYPES.md  
- 07-DEPENDENCY-MAP.md  
- 08-WIRING-AND-INTEGRATION.md  
- 09-SAFETY-AND-INVARIANTS.md  
- 10-TESTING-ALIGNMENT.md  
- 11-OBSERVABILITY.md  
- 12-FAILURE-MODES.md  
- TODO.md  
- OPEN-QUESTIONS.md  
- _progress.md  

Legacy-named redirects retained so old links resolve (see redirect stubs).

## Scope discipline

- **Docs only** under `Docs/architecture/perception/`.  
- No Go/Mangle/test/config edits.  
