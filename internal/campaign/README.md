# internal/campaign/

Long-horizon, multi-phase goal execution: the executive that runs work too large
for a single OODA turn.

**Last verified against the code:** 2026-08-15
**Scale:** 49 non-test sources (~22.2k lines), 59 test files
**Corpus:** the authoritative architecture lives in `Docs/architecture/campaign/`;
this file is the operator/maintainer map of the package itself.

## What this package is

A campaign is a durable plan — phases, tasks, dependencies, write sets — that the
**Mangle kernel schedules** and Go executes. The split matters:

| Concern | Owner |
|---------|-------|
| Proposing a plan, replanning, summarising | LLM (`Decomposer`, `Replanner`) |
| Deciding what runs next, what is blocked, what stops a campaign | Mangle (`policy.mg` §19, `campaign_rules.mg`, `build_topology.mg`) |
| Running effectful work | `session.TaskExecutor`, `tactile.Executor` |
| Durability | JSON snapshot + append-only journal under `.nerd/campaigns/` |

Constitutional safety is **not** re-implemented here. Mutating tools and shards
still pass through VirtualStore / `permitted(...)`.

## Modular map

```
internal/campaign/
  orchestrator.go                  # package marker + module map only
  orchestrator_types.go            # Orchestrator, OrchestratorConfig, events
  orchestrator_init.go             # NewOrchestrator, validation, default wiring
  orchestrator_lifecycle.go        # Load/Set/save, resetInProgress
  orchestrator_execution.go        # Run loop, heartbeat, risk preflight gate
  orchestrator_control.go          # Pause/Resume/Stop, GetProgress
  orchestrator_phases.go           # phase queries, start/complete transitions
  orchestrator_tasks.go            # runPhase scheduler, rolling wave, runSingleTask
  orchestrator_task_handlers.go    # TaskType routing (incl. /campaign_ref)
  orchestrator_task_results.go     # result cache + context injection
  orchestrator_task_transaction.go # per-task fact transactions + rollback
  orchestrator_failure.go          # retries, escalation, repro tasks
  orchestrator_journal.go          # journal append + atomic snapshot
  orchestrator_events.go           # CLOSED set of OrchestratorEvent.Type values
  orchestrator_utils.go            # checkpoints, event/progress emit

  types.go                         # domain model + ToFacts (kernel transduction)
  campaign_fact_sync.go            # incremental fact retract/assert on transitions
  normalization.go / utils.go / errors.go / task_mutation_types.go

  decomposer.go                    # Decompose pipeline
  decomposer_documents.go          # ingest + classify + knowledge DB
  decomposer_requirements.go       # requirement extraction
  decomposer_planning.go           # LLM propose / build / validate / refine
  replan.go                        # adaptive replan + rolling-wave refinement
  context_pager.go                 # phase token budget, prefetch, compression
  checkpoint.go                    # verification runners (tests/build/review/nemesis)
  micro_checkpoint.go              # intra-task verification
  write_set_lock_manager.go        # deterministic file ownership for parallel tasks

  risk_scoring.go                  # deterministic risk score + strict gates
  risk_gate_contract.go            # HARD vs SOFT contract; kernel decides, Go enforces
  intelligence_gatherer.go         # multi-system pre-planning intelligence
  intelligence_gathering_methods.go / intelligence_formatting.go
  shard_advisory_board.go          # advisor consultation + vote synthesis
  edge_case_detector.go            # create/extend/modularize file decisions
  tool_pregenerator.go             # fill tool gaps before execution
  specialist_knowledge.go          # specialist DB atoms into task context

  assault_campaign.go              # deterministic 4-phase assault plan
  assault_types.go                 # AssaultConfig, stages, scope
  assault_tasks.go                 # discover / batch / triage handlers
  assault_prompts.go               # assault-specific prompt text
  assault_report.go                # aggregate results -> summary.md / summary.json

  journal_ops.go                   # verify + replay operator API
  metrics.go                       # optional backend-agnostic MetricsSink
  campaign_prompts.go              # PromptProvider + role -> atom family map
  prompts.go                       # frozen last-resort fallback prompts
  document_ingestor.go             # source document ingestion
```

Largest units, for orientation: `replan.go` (1201), `risk_scoring.go` (1169),
`assault_tasks.go` (1160), `decomposer.go` (1079), `prompts.go` (1072),
`orchestrator_task_handlers.go` (1060), `edge_case_detector.go` (1057).

## Required wiring

`NewOrchestrator` refuses to build without **all** of:

- `Kernel`, `LLMClient`, `Executor` (tactile), `VirtualStore`, non-empty `Workspace`
- **`TaskExecutor`** — required, not "TaskExecutor or ShardManager". `ShardManager`
  is monitoring only; without a task executor nothing can run and both
  verification checkpoints have nothing to run on. Typed nils are rejected too.

Optional collaborators (`IntelligenceGatherer`, `AdvisoryBoard`,
`EdgeCaseDetector`, `ToolPregenerator`, `NorthstarObserver`, `MetricsSink`,
`PromptProvider`) are dependency-injected. When the kernel is a
`*core.RealKernel` and a workspace is set, `IntelligenceGatherer` and
`EdgeCaseDetector` are **default-wired** from those two ingredients so a caller
cannot silently lose pre-planning intelligence and the edge risk gate.

## Risk gating: hard vs soft

Go measures the preflight and asserts what it saw; `campaign_rules.mg` §13
decides what stops the campaign.

| Finding | Effect |
|---------|--------|
| Any blocked gate on a **protected surface** (`internal/core`, `internal/mangle`, `internal/campaign`, `internal/perception`, `internal/articulation`) | **HARD** — campaign refuses to start |
| Northstar alignment blocked, anywhere | **HARD** |
| Critical advisor voted REJECT | **HARD** |
| `--risk-gate force_block` | **HARD** |
| Blocked gate while over threshold **and** carrying safety warnings / blocked actions | **HARD** |
| Anything else blocked (edge prework, requested changes, no consensus, consultation failure) | **SOFT** — recorded, emitted as `risk_gate_advisory`, campaign proceeds |

A hard finding returns a `*RiskBlockedError`; `campaign.FormatRiskBlock(err)`
renders the full gate report for CLI and chat. `Orchestrator.LastRiskEvaluation()`
exposes the soft findings on a campaign that did start.

If `campaign_rules.mg` is not loaded, Go falls back to a mirror of the same
contract rather than treating "no rules" as "no blocks";
`TestRiskClassification_KernelAndMirror_ShouldAgree` keeps the two in step.

## Durability

- `.nerd/campaigns/<id>.json` — snapshot, written to a temp file, checksum-verified, then renamed
- `.nerd/campaigns/<id>.journal.jsonl` — append-only, per-event SHA-256, event-before-ack

The snapshot rename never deletes the committed file before the replacement
lands: it moves it aside and restores it if the replacement fails.

Operator surface:

```bash
nerd campaign journal verify              # integrity + snapshot consistency, non-zero exit on defects
nerd campaign journal replay --limit 20   # reconstructed progress history
```

## Adversarial assault

Deterministic (no LLM decomposition): discover targets → batch → run stages →
triage → remediation. Artifacts under `.nerd/campaigns/<slug>/assault/`.

```bash
nerd campaign assault                                   # subsystem scope, go test + nemesis
nerd campaign assault package --include internal/core --cycles 3
nerd campaign assault --stages command --command "golangci-lint run {{target}}"
nerd campaign assault --dry-run
nerd campaign report                                    # aggregate results -> summary.md
```

Chat parity: `/campaign assault …`. Every `AssaultConfig` field is settable from
a Cobra flag, enforced by `TestCampaignAssaultFlags_CoverEveryAssaultConfigField`.

## Prompts

`PromptProvider` has two implementations. Production wires
`CampaignJITProvider`, which assembles `internal/prompt/atoms/campaign/*`
against the live campaign. `StaticPromptProvider` (`prompts.go`) is a
**last-resort fallback** that logs a warning naming the atom family it stood in
for. `CampaignRoleAtomFamily` maps each role to its atoms and
`TestCampaignRoles_HaveAtomCoverage` fails if a role has none.

## Observability

- `OrchestratorEvent.Type` is a closed set (`orchestrator_events.go`), enforced
  against the emit sites by `TestOrchestratorEventTypes_AreClosedSet`.
- `SetMetricsSink` accepts any `MetricsSink`: task durations by type and
  outcome, phase wall time, checkpoint results, risk preflight outcomes. No
  backend dependency; nil sink costs nothing. `InMemoryMetrics` is included.

## Testing

```bash
go test ./internal/campaign/...
go test ./internal/campaign/ -run ToFacts -update-golden   # regenerate the fact-shape golden
```

`testdata/tofacts_predicates.golden` pins the predicate/arity/type set emitted
into the kernel. Arity drift there does not fail loudly — it just stops Mangle
rules from matching — so the golden is the guard.

---

See `Docs/architecture/campaign/IMPLEMENTED_SPEC.md` for the full architecture,
failure modes and wiring journal.
