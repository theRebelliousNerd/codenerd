# articulation corpus rebuild — progress

| Date | Action |
|------|--------|
| **2026-07-13** | Full architecture corpus rebuilt under `Docs/architecture/articulation/` against `internal/articulation/`. Research: all 8 non-test Go sources + package README; reverse importers (`cmd/nerd/chat`, `internal/session`, `internal/system/factory`, `internal/shards/*`, `internal/perception`). Flagship `IMPLEMENTED_SPEC.md` densified to CLI depth. Document set aligns with `_rebuild/SUBAGENT_INSTRUCTIONS.md` naming (01-VISION, 05-INTERNAL-ARCHITECTURE, 06-PUBLIC-API, 08-WIRING, etc.). No Go/Mangle/test code modified. |

## Deliverables checklist

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

## Source snapshot at rebuild

| Metric | Value |
|--------|------:|
| Non-test `.go` | 8 |
| Test `.go` | 7 |
| `.mg` | 0 |
| Largest sources | `prompt_assembler.go` (~1165), `emitter.go` (~1103) |
