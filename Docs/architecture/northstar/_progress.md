# Progress — Northstar architecture corpus

| Date | Change |
|------|--------|
| 2026-07-13 | **Full rebuild** to SUBAGENT_INSTRUCTIONS + `Docs/architecture/cli/` quality bar: replaced thin auto-inventory stubs with code-grounded full set (README, IMPLEMENTED_SPEC, 00–12, TODO, OPEN-QUESTIONS). Sourced from `internal/northstar/*`, reverse deps in chat/campaign/init, adjacent CLI/wizard/atoms. |
| 2026-07-13 | Prior thin corpus generated (type/file tables only) — superseded. |

## Rebuild checklist

- [x] No files outside `Docs/architecture/northstar/` modified  
- [x] Paths cited exist under `internal/northstar/` and known integrators  
- [x] IMPLEMENTED_SPEC is dense living spec  
- [x] No pre-impl “no code exists” banners  
- [x] README links full doc map  
- [x] Honest dual-store + boot kernel wiring gaps  

## Package stats (at rebuild)

- 4 non-test Go ≈ 2196 lines  
- 6 test Go ≈ 3135 lines  
- 0 package `.mg`  
