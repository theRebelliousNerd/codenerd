# campaign architecture corpus — rebuild progress

| Date | Action |
|------|--------|
| 2026-08-15 | Backlog pass: all 18 TODO items closed. Kernel-decided hard/soft risk contract (`campaign_rules.mg` §13 + `risk_gate_contract.go`), TaskExecutor made a hard requirement, `ToFacts` golden + Decl cross-check (found 3 undeclared predicates and a float-into-`/number` slot), checkpoint-fail regression suite, default intelligence wiring, `campaign_ref` e2e, journal verify/replay, assault summary export, closed event enum, metrics hooks, Cobra assault parity. Snapshot rename defect found by the new chaos test and fixed. Corpus reconciled from test evidence. |
| 2026-07-13 | Full rebuild against `internal/campaign/` (orchestrator modular split, decomposer, context pager, assault, risk gates, journal, write-set locks). Flagship `IMPLEMENTED_SPEC.md` rewritten to CLI quality bar. Document set aligned to `Docs/architecture/_rebuild/SUBAGENT_INSTRUCTIONS.md` naming. Legacy filenames left as redirects. |

**Sources consulted (non-exhaustive):**

- `internal/campaign/*.go` (orchestrator_*, decomposer*, context_pager, assault_*, checkpoint, replan, risk_scoring, intelligence_*, edge_case_*, tool_pregenerator, write_set_lock_manager, types, journal, README)
- `internal/core/defaults/campaign_rules.mg`
- `cmd/nerd/cmd_campaign.go`, `cmd/nerd/campaign_jit_provider.go`
- Reverse deps: chat/ui/e2e importers of `codenerd/internal/campaign`

**Not modified (2026-07-13 rebuild):** any Go, Mangle, tests, or code outside `Docs/architecture/campaign/`.

**Modified (2026-08-15 backlog pass):** `internal/campaign/**`,
`cmd/nerd/cmd_campaign.go` (one injection point), `cmd/nerd/cmd_campaign_assault.go`,
`cmd/nerd/cmd_campaign_journal.go`, `cmd/nerd/chat/model_update.go` (error
rendering), and additive edits to `internal/core/defaults/schemas_campaign.mg`
and `internal/core/defaults/campaign_rules.mg`.
