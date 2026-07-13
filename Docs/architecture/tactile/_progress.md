# tactile architecture corpus — progress

| Date | Note |
|------|------|
| 2026-07-13 | Superstar reconciliation completed at Git checkpoint `cfc537e9` plus the recorded dirty-tree packet. Strict corpus validation and fixed verification profile pass with 18 canonical Markdown documents, 4 feature cards, zero redirects, broken links, unresolved source references, or missing README sections. Product uplift: explicit unavailable sandboxes now fail closed and `ExecutorFactory.CreateDocker` preserves caller configuration. Full `go test -count=1 ./internal/tactile/...` and full `-race` both pass. Signed human score: **40/42** — orientation 3, north-star 3, evidence 3, architecture 3, data/logic 3, lifecycle 3, safety 3, JIT/agents 2 (evidenced N-A boundary), wiring 3, operations 3, verification 3, uplift 3, navigation 3, consistency 2. |
| 2026-07-13 | Fixed Docker availability probe amplification and configuration drift in `docker.go`/`factory.go`. Receipt: `go test -count=1 -timeout=30s ./internal/tactile -run 'TestDockerDetectionCachesUnavailableProbe|TestCompositeExecutorPassesConfigToDockerProbe'` passed in 0.026s; dependent `TestKnowledgeGraphNoDatabase` passed in 5.329s. |
| 2026-07-13 | Full corpus **rebuilt** from `internal/tactile/` source (not thin auto-inventory). New doc set per `Docs/architecture/_rebuild/SUBAGENT_INSTRUCTIONS.md`. Flagship `IMPLEMENTED_SPEC.md` + numbered 00–12 + governance. Supersedes earlier domain-model stub set. |
| 2026-07-13 | Prior pass: thin 1:1 inventory stub generation (tier 2). |

## Sources consulted (rebuild)

- `internal/tactile/*.go` (all non-test + tests)
- `internal/tactile/python/environment.go`
- `internal/tactile/swebench/harness.go`
- `internal/tactile/swebench/instance.go`
- `internal/tactile/README.md`
- Reverse deps: `internal/core/virtual_store.go`, `virtual_store_codedom.go`, `cmd/nerd/chat/session_boot.go`, `cmd/nerd/dom_*.go`, `cmd/nerd/cmd_campaign.go`, `internal/campaign/*`, `tests/e2e/*`
- Mangle decls: `internal/core/defaults/schemas_shards.mg`, `schemas_codedom.mg`, `schemas_execution.mg`
- Logging: `internal/logging/logger.go`, `logger_convenience.go`
