# campaign — durable multi-phase execution

**Question answered here:** what is `internal/campaign` and how do I run it?
How it works internally is in [INTERNALS.md](INTERNALS.md);
what is wired, what is dormant, and what was never built is in
[WIRING-AND-NOT-BUILT.md](WIRING-AND-NOT-BUILT.md).

**Verified 2026-09-20 against the working tree at `refs/heads/main`.**
(No read-only tool was available to record the exact commit hash; re-pin on next touch.
The in-package map at `internal/campaign/README.md` still says 2026-08-15 — stale.)

## What it is

A campaign is a durable plan — `Campaign` → `Phase` → `Task`
(`internal/campaign/types.go:156`, `:235`, `:293`) — for work too large for a
single OODA turn. The split of responsibilities is the whole design:

| Concern | Owner |
|---------|-------|
| Proposing a plan, replanning, summarising | LLM: `Decomposer.Decompose` (`internal/campaign/decomposer.go:242`), `Replanner.Replan` (`internal/campaign/replan.go:169`) |
| Deciding what runs next, what is blocked, what stops a campaign | Mangle policy over facts emitted by `Campaign.ToFacts` / `Phase.ToFacts` / `Task.ToFacts` (`internal/campaign/types.go:496`, `:555`, `:618`) |
| Running effectful work | `session.TaskExecutor` + `tactile.Executor`, carried on `OrchestratorConfig` (`internal/campaign/orchestrator_types.go:126`) |
| Durability | JSON snapshot + append-only journal under `.nerd/campaigns/` (`internal/campaign/orchestrator_lifecycle.go:132`, `internal/campaign/journal_ops.go:96`) |

Constitutional safety is not re-implemented here: mutating tools and shards still
pass through VirtualStore / `permitted(...)`. Campaign types are `/greenfield`,
`/feature`, `/audit`, `/migration`, `/remediation`, `/adversarial_assault`,
`/recurse`, `/custom` (`internal/campaign/types.go:28`).

## Module map (filenames verified against the tree)

```
orchestrator.go / orchestrator_types.go   # Orchestrator, OrchestratorConfig, events
orchestrator_lifecycle.go                 # NewOrchestrator, LoadCampaign (:69), SetCampaign (:105), saveCampaign (:132), resetInProgress (:168)
orchestrator_execution.go                 # Run (:14), risk preflight gate (:37), sweepUndeclaredRootWrites (:198)
orchestrator_control.go                   # Pause/Resume/Stop, GetProgress
orchestrator_phases.go / orchestrator_tasks.go / orchestrator_task_handlers.go
orchestrator_task_results.go / orchestrator_task_transaction.go / orchestrator_failure.go
orchestrator_journal.go / orchestrator_events.go / orchestrator_utils.go
types.go                                  # domain model + ToFacts transduction
decomposer.go / decomposer_documents.go / decomposer_requirements.go / decomposer_planning.go
replan.go                                 # Replan (:169), ReplanForNewRequirement (:237), RefineNextPhase (:417)
context_pager.go                          # ContextPager (:18), NewContextPager (:37)
checkpoint.go                             # CheckpointRunner (:24), Run (:72)
risk_scoring.go / risk_gate_contract.go   # preflight measurement + hard/soft contract
intelligence_gatherer.go / intelligence_gathering_methods.go / intelligence_formatting.go
shard_advisory_board.go / edge_case_detector.go / tool_pregenerator.go / specialist_knowledge.go
assault_campaign.go (:19) / assault_types.go / assault_tasks.go / assault_prompts.go / assault_report.go
recurse_runner.go                         # RecurseLoop (:46), RecurseRunner (:117)
journal_ops.go                            # VerifyCampaignJournal (:208), ReplayCampaignJournal (:293)
metrics.go / campaign_prompts.go / prompts.go / document_ingestor.go
```

`TaskExecutor` on the config is **required, not optional**
(`internal/campaign/orchestrator_types.go:134`); `ShardManager` (`:135`) is
monitoring only. Without a task executor nothing runs — and both verification
checkpoints fail closed (see INTERNALS.md).

## Running a campaign

Build with `NewOrchestrator`, attach a campaign with `LoadCampaign` or
`SetCampaign`, call `Run`. `Run` gates on the risk preflight before any task
executes (`internal/campaign/orchestrator_execution.go:37`); a hard finding
aborts with `*RiskBlockedError` (`internal/campaign/risk_gate_contract.go:89`).
Resume re-arms checkpoints on phases left `/unverified`
(`internal/campaign/types.go:63`).

Durability operators should know:

```bash
nerd campaign journal verify              # integrity + snapshot consistency, non-zero exit on defects
nerd campaign journal replay --limit 20   # reconstructed progress history
```

Backed by `VerifyCampaignJournal` / `ReplayCampaignJournal` / `RenderJournalVerification` /
`RenderJournalReplay` (`internal/campaign/journal_ops.go:208`, `:293`, `:353`, `:390`).

## Adversarial assault

The deterministic (non-LLM-decomposed) campaign: entry point
`NewAdversarialAssaultCampaign(workspace, cfg)`
(`internal/campaign/assault_campaign.go:19`), executed through
`/assault_discover` → `/assault_batch` → `/assault_triage`
(`internal/campaign/types.go:96`). Artifacts land under
`.nerd/campaigns/<slug>/assault/`.

```bash
nerd campaign assault                                   # subsystem scope, go test + nemesis
nerd campaign assault package --include internal/core --cycles 3
nerd campaign assault --stages command --command "golangci-lint run {{target}}"
nerd campaign assault --dry-run
nerd campaign report                                    # aggregate results -> summary.md
```
