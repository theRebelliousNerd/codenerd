# Progress — shards architecture corpus

## 2026-07-13 — Full rebuild (SUBAGENT_INSTRUCTIONS)

- Rebuilt `Docs/architecture/shards/` against living code under `internal/shards/` + `internal/shards/system/`.  
- Flagship: dense `IMPLEMENTED_SPEC.md` covering registration, system shards, OODA pipeline, lifecycle, specialist libs, dual-boot wiring, gaps.  
- Produced full doc set per rebuild contract: README, 00–12, TODO, OPEN-QUESTIONS, this file.  
- Corrected stale narrative: domain Go shards removed, but system shards + registration are production-live (package README still migration-era).  
- Quality bar: `Docs/architecture/cli/` (path citations, mermaid/ASCII flows, honest partials).  
- **No code changes** outside `Docs/architecture/shards/`.

### Research sources (code)

- `internal/shards/registration.go`, `matching.go`, `consultation.go`, `observer_manager.go`, `requirements_interrogator.go`  
- `internal/shards/system/{base,perception,executive,constitution,router,world_model,planner,campaign_runner,legislator,mangle_repair,payloads}.go`  
- Wiring: `internal/system/factory.go`, `cmd/nerd/chat/session_boot.go`, consumers under `cmd/nerd/chat/*`, `cmd/nerd/cmd_campaign.go`  

### Superseded earlier corpus shape

Prior auto-inventory files with names like `01-DOMAIN-MODEL.md` / `02-CURRENT-STATE-SHARDS.md` may still exist beside this set; **authoritative map is README.md document table**. Prefer the numbered files listed there.
