# Progress — Docs/architecture/jit

| Date | Change |
|------|--------|
| 2026-07-13 | Full corpus **rebuild** to SUBAGENT_INSTRUCTIONS full set (cli quality bar). Replaced thin auto-inventory stubs with code-grounded deep docs. Scope: `internal/jit/config` as schema contract; producers/consumers mapped in prompt+session. |
| 2026-07-13 | Prior thin generation noted (1:1 inventory, ~90% heuristics) — superseded. |

## Rebuild checklist (2026-07-13)

- [x] README.md
- [x] IMPLEMENTED_SPEC.md (flagship narrative)
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
- [x] Supersession notes for old thin filenames

## Evidence base

- Read `internal/jit/config/types.go`, `types_test.go`
- Grepped reverse imports (`internal/prompt`, `internal/session`, `tests/e2e`)
- Read producer/consumer hotspots: `config_factory.go`, `compiler.go` (attach), `executor.go`, `executor_tools.go`, `spawner.go`, `subagent.go`
- Skilled narrative: `.claude/skills/codenerd-builder/references/jit-execution-model.md` (names may lag)

## Out of scope

- No Go/Mangle/test code changes
- No files outside `Docs/architecture/jit/`
