# world — Corpus Rebuild Progress

| Date | Action |
|------|--------|
| 2026-07-13 | Full architecture corpus rebuild per `Docs/architecture/_rebuild/SUBAGENT_INSTRUCTIONS.md` |
| 2026-07-13 | Research: listed `internal/world/` + `lsp/`; read fs, incremental, deep, cartographer, dataflow, scope, holographic, CodeDOM, predicates, schemas_world, wiring callers |
| 2026-07-13 | Flagship `IMPLEMENTED_SPEC.md` rewritten dense (FS topology + AST/holographic deep dives) |
| 2026-07-13 | New document set: VISION, CURRENT-STATE, GAP, PRINCIPLES, INTERNAL ARCH, PUBLIC API, DEPS, WIRING, SAFETY, TESTING, OBSERVABILITY, FAILURE MODES, MANGLE SURFACE |
| 2026-07-13 | Superseded old thin names with redirect stubs where conflicting |

## Produced files (canonical)

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
- 13-MANGLE-SURFACE.md
- TODO.md
- OPEN-QUESTIONS.md
- _progress.md

## Source snapshot

- Package: `internal/world/` (~37 non-test Go, ~31 tests)
- Subpackage: `internal/world/lsp/`
- Schemas: `internal/core/defaults/schemas_world.mg`
- Heuristic completeness: ~85–90% living implementation

## Out of scope (per instructions)

- No Go/Mangle/code changes
- No files outside `Docs/architecture/world/`
