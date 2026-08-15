# _progress — context architecture corpus

| Date | Change |
|------|--------|
| 2026-07-13 | Full corpus **rebuild** to CLI quality bar per `Docs/architecture/_rebuild/SUBAGENT_INSTRUCTIONS.md`. Replaced thin inventory stubs with deep narrative: activation scoring, window management, compression loop, kernel hybrid path, wiring journal, safety invariants. Flagship `IMPLEMENTED_SPEC.md` expanded. Document set standardized to required filenames (`01-VISION.md`, `02-CURRENT-STATE.md`, `05-INTERNAL-ARCHITECTURE.md`, etc.). Sources read: all 9 non-test Go files under `internal/context/`, plus `schemas_context.mg`, `context_compilation.mg`, chat boot/process wiring. |
| 2026-08-15 | Backlog implementation pass (code + docs). C3 masking finished: `assertTurnAgeCategories` had appended a clause terminator `ParseFactString` adds itself, so every assertion failed to parse under a discarded error and masking had never fired; Go now consumes `should_mask_observation` ∩ `should_preserve_reasoning`. Kernel context selection fixed (entity→fact resolution + the fallback its own doc comment promised) and instrumented via `SelectionStats`. `LoadState` now refreshes the budget itself; `RefreshBudget` no longer drops the lock. Long-session gate found and fixed silent history truncation and unbounded rolling-summary growth. Corpus updated from test evidence. |
| 2026-07-13 | Earlier auto-inventory pass (superseded by rebuild above). |

## Verification notes

- 2026-07-13 pass: docs only, no Go/Mangle/test modifications.  
- 2026-08-15 pass: Go + tests under `internal/context/`, new `cmd/nerd/cmd_context_stats.go`.
  No `.mg` files changed. Verified with `go build ./...`,
  `go test ./internal/context/...` and `go test -race ./internal/context/...`.  
- Paths cited against live tree on 2026-08-15.  
