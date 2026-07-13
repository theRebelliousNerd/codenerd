# tactile architecture corpus — progress

| Date | Note |
|------|------|
| 2026-07-13 | Full corpus **rebuilt** from `internal/tactile/` source (not thin auto-inventory). New doc set per `Docs/architecture/_rebuild/SUBAGENT_INSTRUCTIONS.md`. Flagship `IMPLEMENTED_SPEC.md` + numbered 00–12 + governance. Supersedes earlier domain-model stub set. |
| 2026-07-13 | Prior pass: thin 1:1 inventory stub generation (tier 2). |

## Sources consulted (rebuild)

- `internal/tactile/*.go` (all non-test + tests)
- `internal/tactile/python/environment.go`
- `internal/tactile/swebench/{harness,instance}.go`
- `internal/tactile/README.md`
- Reverse deps: `internal/core/virtual_store.go`, `virtual_store_codedom.go`, `cmd/nerd/chat/session_boot.go`, `cmd/nerd/dom_*.go`, `cmd/nerd/cmd_campaign.go`, `internal/campaign/*`, `tests/e2e/*`
- Mangle decls: `internal/core/defaults/schemas_shards.mg`, `schemas_codedom.mg`, `schemas_execution.mg`
- Logging: `internal/logging/logger.go`, `logger_convenience.go`
