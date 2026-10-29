# sqlpragmas — Corpus Progress

## 2026-07-13 — Full architecture rebuild (new doc set)

**Mode:** Docs only under `Docs/architecture/sqlpragmas/`.  
**Source of truth:** `internal/sqlpragmas/` (1 production file, 2 tests) + reverse-dep grep + `internal/store/pragmas.go` façade.

### Produced (SUBAGENT_INSTRUCTIONS full set)

| File | Status |
|------|--------|
| README.md | Rebuilt |
| IMPLEMENTED_SPEC.md | Rebuilt (flagship narrative) |
| 00-ALIGNMENT-VISION-REVIEW.md | Rebuilt |
| 01-VISION.md | New name/content |
| 02-CURRENT-STATE.md | New name/content |
| 03-GAP-ANALYSIS.md | New name/content |
| 04-ARCHITECTURAL-PRINCIPLES.md | New name/content |
| 05-INTERNAL-ARCHITECTURE.md | New |
| 06-PUBLIC-API-AND-TYPES.md | New |
| 07-DEPENDENCY-MAP.md | Rebuilt |
| 08-WIRING-AND-INTEGRATION.md | New (replaces thin cross-system stub role) |
| 09-SAFETY-AND-INVARIANTS.md | New |
| 10-TESTING-ALIGNMENT.md | New |
| 11-OBSERVABILITY.md | New |
| 12-FAILURE-MODES.md | New |
| TODO.md | Rebuilt |
| OPEN-QUESTIONS.md | Rebuilt |
| _progress.md | This file |

### Research performed

- Read `pragmas.go`, `pragmas_test.go`, `pragma_integration_test.go` end-to-end  
- Read `internal/store/pragmas.go` re-export  
- Grep all `sqlpragmas` / `ApplyDefaultPragmas` / `store.Profile*` call sites  
- Confirmed no Mangle / no kernel registration  

### Superseded thin stubs (old names)

Earlier auto-inventory files used DOMAIN-MODEL / CURRENT-STATE-SQLPRAGMAS / INVARIANTS-AND-GATES naming. The authoritative set is the table above (per `_rebuild/SUBAGENT_INSTRUCTIONS.md`). Orphan old-named files, if still present, are **not** maintained.

### Quality bar

- Package-specific (pragma profiles, leaf cycle story, FK omission)  
- Real paths only  
- No “0% pre-implementation” claims  
- CLI-depth narrative scaled to small package (≥150-line IMPLEMENTED_SPEC target)  

### Not done (by design)

- No Go / test / config changes  
- No GitHub push from this subagent  
