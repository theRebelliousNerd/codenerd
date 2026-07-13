# regression — Corpus Progress

| Date | Change |
|------|--------|
| 2026-07-13 | Initial thin auto-inventory corpus (tier-2 stubs: domain model tables, generic gates). |
| 2026-07-13 | **Full rebuild** per `Docs/architecture/_rebuild/SUBAGENT_INSTRUCTIONS.md`. Replaced stubs with code-grounded narrative set (README, IMPLEMENTED_SPEC flagship, 00–12 series, TODO, OPEN-QUESTIONS). Old stub filenames kept as short “Moved” redirects. Evidence: full read of `battery.go` + `battery_test.go`; reverse-import grep (zero importers). |

## Rebuild checklist

- [x] No files outside `Docs/architecture/regression/` modified for this task  
- [x] Paths cited exist on disk  
- [x] IMPLEMENTED_SPEC is dense living spec  
- [x] No pre-impl “no code exists” banners  
- [x] README links every produced doc  
- [x] Last-verified date 2026-07-13  
