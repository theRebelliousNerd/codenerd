# _progress — logging architecture corpus

## 2026-07-13 — Full rebuild (SUBAGENT_INSTRUCTIONS)

- **Mode:** Docs only under `Docs/architecture/logging/`  
- **Source researched:** `internal/logging/` (4 non-test Go, 5 tests, 0 `.mg`)  
- **Reverse deps:** `cmd/nerd`, chat boot, and many `internal/*` consumers  
- **Quality bar:** `Docs/architecture/cli/` depth  
- **Produced full set:**

| File | Notes |
|------|-------|
| `README.md` | Scope, map, verify |
| `IMPLEMENTED_SPEC.md` | Flagship deep-dive |
| `00-ALIGNMENT-VISION-REVIEW.md` | Scored dimensions |
| `01-VISION.md` | Target vision |
| `02-CURRENT-STATE.md` | Inventory |
| `03-GAP-ANALYSIS.md` | Gaps + non-gaps |
| `04-ARCHITECTURAL-PRINCIPLES.md` | 12 principles |
| `05-INTERNAL-ARCHITECTURE.md` | Components + flows |
| `06-PUBLIC-API-AND-TYPES.md` | API surface |
| `07-DEPENDENCY-MAP.md` | Import graph |
| `08-WIRING-AND-INTEGRATION.md` | Boot + call patterns |
| `09-SAFETY-AND-INVARIANTS.md` | Safety / privacy |
| `10-TESTING-ALIGNMENT.md` | Tests |
| `11-OBSERVABILITY.md` | Operator guide |
| `12-FAILURE-MODES.md` | FM1–FM15 |
| `TODO.md` | Backlog |
| `OPEN-QUESTIONS.md` | Design questions |
| `_progress.md` | This file |

- **Earlier thin stubs** (DOMAIN-MODEL, CROSS-SYSTEM-WIRING naming, etc.) may still exist as orphans; authoritative set is the table above linked from `README.md`.  
- **No code changes.**
