# mangle corpus rebuild — progress

| Date | Action |
|------|--------|
| 2026-07-13 | Full corpus rebuild from `internal/mangle/` source (docs-only). Flagship `IMPLEMENTED_SPEC.md` rewritten with dense engine / differential / feedback deep dives. Canonical doc set per `Docs/architecture/_rebuild/SUBAGENT_INSTRUCTIONS.md`. Quality bar: `Docs/architecture/cli/`. |

## Files produced (canonical set)

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

## Research basis

- Read: `engine.go`, `differential.go`, `parse_lock.go`, `schema_validator.go`, `grammar.go`, `proof_tree.go`, `lsp.go`, `feedback/*`, `synth/*`, `transpiler/sanitizer.go`
- Kernel wiring: `internal/core/kernel_eval.go`, `kernel_init.go`, `kernel_policy.go`, `parse_serial.go`
- Downstream: `internal/shards/system/{legislator,executive,constitution,mangle_repair}.go`, `cmd/nerd/cmd_mangle_*.go`
- Reverse imports grepped for `codenerd/internal/mangle`
