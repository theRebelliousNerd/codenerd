# retrieval — Corpus Rebuild Progress

| Date | Change |
|------|--------|
| 2026-07-13 | **Full rewrite** to SUBAGENT_INSTRUCTIONS quality bar (cli-depth). Code-grounded against `internal/retrieval/` (4 src, 6 tests), chat boot/seed wiring, `schemas_knowledge.mg`, context compressor consumers. Replaced thin auto-inventory stubs with full doc set: README, IMPLEMENTED_SPEC, 00–12 series, TODO, OPEN-QUESTIONS. Honest gaps: dormant `Model.Retriever`, extract-only seed, T4 placeholder, ripgrep comment drift. |
| 2026-07-13 | Prior stub generation (tier-2 inventory) superseded by this rewrite. |

## Produced set (this rebuild)

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

## Superseded filenames (legacy thin stubs)

These older names were inventory stubs from an earlier generator and are **not** part of the required contract. Prefer the numbered set above; remove leftovers if still present:

- `01-DOMAIN-MODEL.md`  
- `02-CURRENT-STATE-RETRIEVAL.md`  
- `03-GAP-ANALYSIS-RETRIEVAL.md`  
- `04-INVARIANTS-AND-GATES.md`  
- `05-CROSS-SYSTEM-WIRING.md`  
- `06-TESTING-STRATEGY.md`  
