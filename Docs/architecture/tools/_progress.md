# tools — Corpus Rebuild Progress

| Date | Action |
|------|--------|
| 2026-07-13 | Full architecture corpus rebuilt per `Docs/architecture/_rebuild/SUBAGENT_INSTRUCTIONS.md`. Source researched: `internal/tools/**` (25 non-test Go, 21 tests, 0 local mg), plus wiring in `virtual_store_tools.go`, `executor_tools.go`, `intent_routing.mg`, `schemas_tools.mg`, system factory hydrate. Replaced thin inventory stubs with dense living set: README, IMPLEMENTED_SPEC (flagship), 00–12 series, TODO, OPEN-QUESTIONS, this progress log. No code changes. |

## Document checklist

- [x] README.md  
- [x] IMPLEMENTED_SPEC.md  
- [x] 00-ALIGNMENT-VISION-REVIEW.md  
- [x] 01-VISION.md  
- [x] 02-CURRENT-STATE.md  
- [x] 03-GAP-ANALYSIS.md  
- [x] 04-ARCHITECTURAL-PRINCIPLES.md  
- [x] 05-INTERNAL-ARCHITECTURE.md  
- [x] 06-PUBLIC-API-AND-TYPES.md  
- [x] 07-DEPENDENCY-MAP.md  
- [x] 08-WIRING-AND-INTEGRATION.md  
- [x] 09-SAFETY-AND-INVARIANTS.md  
- [x] 10-TESTING-ALIGNMENT.md  
- [x] 11-OBSERVABILITY.md  
- [x] 12-FAILURE-MODES.md  
- [x] TODO.md  
- [x] OPEN-QUESTIONS.md  
- [x] _progress.md  

## Notes

- Older filenames from prior auto-corpus (`01-DOMAIN-MODEL.md`, `02-CURRENT-STATE-TOOLS.md`, `03-GAP-ANALYSIS-TOOLS.md`, `04-INVARIANTS-AND-GATES.md`, `05-CROSS-SYSTEM-WIRING.md`, `06-TESTING-STRATEGY.md`, `08-FAILURE-MODES.md`) redirected to supersession stubs pointing at the canonical set.  
- No modifications under `internal/`, `cmd/`, or elsewhere.  
