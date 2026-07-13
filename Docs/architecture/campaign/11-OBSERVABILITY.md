# 11 — Observability: campaign

> Last verified: **2026-07-13**

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

### OrchestratorEvent types (string field)

Observed in code paths:

- `task_started` / `task_completed` / `task_failed`  
- `phase_started` / `phase_completed`  
- `checkpoint` / `checkpoint_failed`  
- `replan` / `replan_failed`  
- `campaign_completed`  
- `campaign_blocked`  
- `context_error` / `compression_error`  
- learning-related events when wired  

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

## Gaps

1. No first-class metrics registry (Prometheus-style) inside package — logs/channels only.  
2. Event type strings not a closed Go enum (typo risk).  
3. Journal replay CLI not in package.  
4. Glass-box integration depends on CLI transparency features, not campaign-internal exporters.

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
