# campaign architecture corpus — rebuild progress

| Date | Action |
|------|--------|
| 2026-07-13 | Full rebuild against `internal/campaign/` (orchestrator modular split, decomposer, context pager, assault, risk gates, journal, write-set locks). Flagship `IMPLEMENTED_SPEC.md` rewritten to CLI quality bar. Document set aligned to `Docs/architecture/_rebuild/SUBAGENT_INSTRUCTIONS.md` naming. Legacy filenames left as redirects. |

**Sources consulted (non-exhaustive):**

- `internal/campaign/*.go` (orchestrator_*, decomposer*, context_pager, assault_*, checkpoint, replan, risk_scoring, intelligence_*, edge_case_*, tool_pregenerator, write_set_lock_manager, types, journal, README)
- `internal/core/defaults/campaign_rules.mg`
- `cmd/nerd/cmd_campaign.go`, `cmd/nerd/campaign_jit_provider.go`
- Reverse deps: chat/ui/e2e importers of `codenerd/internal/campaign`

**Not modified:** any Go, Mangle, tests, or code outside `Docs/architecture/campaign/`.
