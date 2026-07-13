# Progress — `Docs/architecture/build`

| Date | Change |
|------|--------|
| 2026-07-13 | **Full corpus rebuild** to cli-quality bar per `Docs/architecture/_rebuild/SUBAGENT_INSTRUCTIONS.md`. Replaced thin auto-inventory stubs with narrative docs grounded in `internal/build/env.go` (+ tests) and reverse deps in `internal/autopoiesis`. |
| 2026-07-13 | Prior thin corpus existed (tier-2 auto gen); superseded by this rewrite. |

## Rebuild checklist

- [x] README.md with full doc map  
- [x] IMPLEMENTED_SPEC.md (flagship narrative)  
- [x] 00–12 series (alignment through failure modes)  
- [x] TODO.md, OPEN-QUESTIONS.md, _progress.md  
- [x] Real paths only; no pre-impl 0% claims  
- [x] Honest adoption gaps (autopoiesis-only, nil config)  
- [x] Docs only under `Docs/architecture/build/`  

## Package facts at rebuild

- Production: `internal/build/env.go` (~312 lines)  
- Tests: `env_test.go`, `env_gaps_test.go`  
- Mangle: none  
- Live importers: `tool_compiler.go`, `thunderdome.go` under `internal/autopoiesis`  
