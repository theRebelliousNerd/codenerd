# store — Corpus Rebuild Progress

## 2026-07-13 — Full rebuild (SUBAGENT contract)

**Mode:** Docs only under `Docs/architecture/store/`  
**Source:** `internal/store/` (~39 non-test Go, ~44 tests, 0 `.mg`)  
**Reference bar:** CLI deep-dive density; flagship memory-tier focus

### Produced (new contract file set)

| File | Status |
|------|--------|
| README.md | Rebuilt |
| IMPLEMENTED_SPEC.md | Rebuilt — dense memory tiers flagship |
| 00-ALIGNMENT-VISION-REVIEW.md | Rebuilt |
| 01-VISION.md | Created (replaces domain-model naming) |
| 02-CURRENT-STATE.md | Created |
| 03-GAP-ANALYSIS.md | Created |
| 04-ARCHITECTURAL-PRINCIPLES.md | Created |
| 05-INTERNAL-ARCHITECTURE.md | Created |
| 06-PUBLIC-API-AND-TYPES.md | Created |
| 07-DEPENDENCY-MAP.md | Rebuilt |
| 08-WIRING-AND-INTEGRATION.md | Created |
| 09-SAFETY-AND-INVARIANTS.md | Created |
| 10-TESTING-ALIGNMENT.md | Created |
| 11-OBSERVABILITY.md | Created |
| 12-FAILURE-MODES.md | Rebuilt |
| TODO.md | Rebuilt |
| OPEN-QUESTIONS.md | Rebuilt |
| _progress.md | This file |

### Research performed

- Listed `internal/store` modular layout (`local.go` map)
- Read core: `local_core.go`, `vector_store.go`, `migrations.go`, `local_cold.go`, `learning.go`, `trace_store.go`, `tool_store.go`, `embedded_store.go`, `reflection_worker.go`, pragmas/vec tags/fact_codec
- Grepped exports, constructors, reverse imports
- Traced boot wiring via `internal/system/factory.go` and VirtualStore fields

### Intentionally not done

- No Go/Mangle/test/code changes
- No edits outside `Docs/architecture/store/`
- Legacy-named stubs from prior corpus generation may still exist beside the new set; README maps the authoritative contract set only

### Next (if continued)

- Optionally delete or redirect pre-contract filenames (`01-DOMAIN-MODEL.md`, `02-CURRENT-STATE-STORE.md`, …)
- Cross-link from `Docs/architecture/INDEX.md` when index pass runs
