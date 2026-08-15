# 11 — Observability: campaign

> Last verified: **2026-08-15**

## Logging

Primary category: `logging.CategoryCampaign` via helpers:

| Helper | Use |
|--------|-----|
| `logging.Campaign(...)` | Info-level campaign narrative |
| `logging.CampaignDebug(...)` | Verbose scheduling/context |
| `logging.CampaignWarn` / package Error through `logging.Get(CategoryCampaign)` | Failures |
| `logging.StartTimer(CategoryCampaign, name)` | Timed spans (`NewOrchestrator`, `Run`, `Decompose`, `runPhase`, activate/compress, etc.) |

Operators should enable campaign category in `.nerd/logs/` configuration (see CLI telemetry docs).

### High-signal log moments

- Campaign start banner with type/phases/tasks  
- Phase started / completed  
- Task schedule lines (truncated description)  
- Checkpoint pass/fail  
- Replan applied with revision  
- Risk gate block  
- Assault discovery summary (targets/batches)  
- Journal/snapshot errors  

## Progress and events

| Channel | Type | Payload |
|---------|------|---------|
| `ProgressChan` | `chan Progress` | Overall/phase progress, context usage, errors |
| `EventChan` | `chan OrchestratorEvent` | typed events |

### OrchestratorEvent types — CLOSED SET

Since 2026-08-15 the type strings are constants in
`internal/campaign/orchestrator_events.go`, and
`TestOrchestratorEventTypes_AreClosedSet` parses every `emitEvent` /
`emitRiskAudit` call in the package to prove no bare literal escapes the set. A
typo used to produce an event that every consumer dropped through its default
branch: the campaign still ran, the operator just never saw the step.

`OrchestratorEventTypes()` returns the set; `IsKnownOrchestratorEventType`
tests membership. UIs should assert against it rather than hand-maintaining a
switch.

| Group | Types |
|-------|-------|
| Task | `task_started`, `task_completed`, `task_failed` |
| Phase | `phase_started`, `phase_completed` |
| Campaign | `campaign_completed`, `campaign_blocked` |
| Verification | `checkpoint_failed`, `checkpoint_exhausted` |
| Planning | `replan`, `replan_triggered`, `replan_failed`, `new_requirement_received`, `new_requirement_integrated`, `new_requirement_failed` |
| Context | `context_error`, `compression_error` |
| Scheduling | `task_lock_timeout`, `task_write_set_missing`, `artifact_persisted` |
| Diagnostics | `diagnostic_task_inserted`, `logic_failure_escalated`, `generation_degraded`, `research_empty`, `shard_result_empty`, `tool_generation_requested`, `sub_campaign_referenced` |
| Risk preflight | `risk_snapshot_pinned`, `risk_score_computed`, `risk_gate_result`, `risk_gate_skipped`, `risk_gate_passed`, `risk_gate_advisory`, `risk_gate_blocked`, `risk_intelligence_error` |

`risk_gate_blocked` carries the `*RiskGateEvaluation` in `Data`;
`risk_gate_advisory` carries the `RiskFinding`. Both are renderable with
`campaign.FormatRiskBlock` / `Orchestrator.LastRiskEvaluation()`.

`emitProgress` on heartbeat and control surfaces feeds TUI/CLI status.

## Kernel-visible telemetry

| Fact | Meaning |
|------|---------|
| `campaign_heartbeat(CampaignID, Unix)` | Liveness while Run active |
| `campaign_progress(...)` | Durable progress snapshot facts |
| `task_error(...)` | Failure taxonomy for policy learning |
| `replan_trigger(...)` | Why replan fired |

## Durable audit surfaces

| Artifact | Location |
|----------|----------|
| Campaign JSON | `.nerd/campaigns/<id>.json` |
| Journal JSONL | `.nerd/campaigns/<id>.journal.jsonl` |
| Assault targets/batches/results/logs/triage | `.nerd/campaigns/<slug>/assault/` |
| Knowledge DB | `.nerd/campaigns/<slug>/knowledge.db` |
| Debug program dump | `debug_program_ERROR.mg` on kernel crash |

Journal events include sequence, checksums, and snapshot checksums for forensic reconstruction.

## Northstar observations

When observer set, alignment checks emit campaign logs with scores; blocking returns errors on phase start.

## Metrics hooks (`metrics.go`)

`SetMetricsSink(MetricsSink)` accepts any implementation of a four-method,
primitive-argument interface:

| Method | Observed at |
|--------|-------------|
| `ObserveTaskDuration(campaign, phase, taskType, outcome, d)` | every task completion and failure |
| `ObservePhaseDuration(campaign, phase, d)` | phase completion (start time held in memory, not in the snapshot schema) |
| `ObserveCheckpoint(campaign, phase, method, passed, d)` | every verification run |
| `ObserveRiskPreflight(campaign, score, allowed, hard, soft)` | once per `Run` |

A nil sink observes nothing and allocates nothing, so the engine carries no
metrics dependency and no consumer inherits a backend choice. `InMemoryMetrics`
is provided for tests and for `status`-style summaries. The sink is guarded by
its own mutex rather than `o.mu`: risk preflight observes while `Run` holds
`o.mu`, and `sync.RWMutex` is not reentrant.

## Journal operator tooling

```text
nerd campaign journal verify [--campaign ID] [--json]   # non-zero exit on defects
nerd campaign journal replay [--limit N] [--json]
nerd campaign report [--stdout|--json]                  # assault aggregate
```

`verify` checks per-event checksums, sequence continuity, campaign-id match,
unpaired snapshot writes, and whether the snapshot on disk hashes to what the
last commit event recorded.

## Gaps

1. Glass-box integration still depends on CLI transparency features rather than
   campaign-internal exporters.  
2. `cmd/nerd/ui/campaign_page.go` renders a subset of the closed event set; it
   should assert against `OrchestratorEventTypes()`.  
3. No per-task token/cost attribution in the metrics sink (usage tracking lives
   in `internal/usage`).

## Operator playbook

```text
# While running
tail campaign logs; nerd campaign status

# After failure
inspect .nerd/campaigns/<id>.json Status/LastError fields
read <id>.journal.jsonl last events
for assault: open assault/triage and results/

# Kernel policy debug
query campaign_blocked, task_error, replan_trigger via nerd query tools
```
