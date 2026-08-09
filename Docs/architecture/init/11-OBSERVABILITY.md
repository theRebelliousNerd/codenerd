# init — Observability

> Last verified: 2026-08-09

## Logging categories

| Category | Typical events |
|----------|----------------|
| `logging.CategoryBoot` | Phase milestones, agent KB creates, JIT compile failures, migrations, prompt sync, grounding source counts |
| `logging.CategoryStore` | `ValidateAgentDB` timers and validity summaries |
| `logging.Get(CategoryBoot).Debug` | Doc relevance counts, failed doc fact asserts |

Helpers:

- `logging.StartTimer(CategoryStore|Boot, name)` around validation and shared pool creation.
- `logging.Boot("…")` formatted boot messages throughout init.

## Stdout operator UX

Init is **chatty on stdout** by design:

- Phase banners (`📊 Phase 2`, `🤖 Phase 6`, …)
- Per-agent research lines
- Final framed summary (project stats, required failures, warnings, duration)
- LLM enrichment attempts/successes/failures with provider/model and bounded last error
- Recommendations (max 4)
- Validation section + backup cleanup hint
- Next-step slash commands

This is primary observability for interactive CLI users (not only log files).

## Progress channel

`InitConfig.ProgressChan chan InitProgress` carries:

| Field | Meaning |
|-------|---------|
| Phase | Phase name string |
| Message | Human status |
| Percent | 0.0–1.0 |
| IsError | Error marker |
| AgentUpdate | Nested agent creation details |
| ETARemaining / ElapsedTime | From `ETATracker` |
| CurrentPhaseNo / TotalPhases | Phase index |

Consumers (TUI) can attach a buffered channel. CLI `runInit` currently often leaves it nil → no channel metrics, only stdout.

## ETA tracker

`DefaultPhaseDurations` seeds expected seconds per phase; actuals overwrite on `CompletePhase`. Remaining estimate sums remaining phase map entries.

## Metrics

`InitResult.LLMMetrics` is machine-readable per-run telemetry. It distinguishes
provider failures from atom-count population and drives the degraded summary.
The package still has no Prometheus/OTel exporter.

## Debug artifacts

| Artifact | Meaning |
|----------|---------|
| `debug_program_ERROR.mg` in package dir | Kernel crash dump residue (repo hygiene issue if committed) |
| `InitResult.Warnings` | Machine-usable list of soft failures |
| `InitResult.Failures` | Machine-usable required failures; non-empty means `Success=false` |
| `InitResult.LLMMetrics` | Provider/model and enrichment outcome counts |
| `InitResult.Validation` | Per-shard structural results used before `Success` is derived |
| `InitResult.GroundingSources` | Gemini URL sources when enabled |
| `doc_ingestion_state.json` | Resume state for doc campaign |

## Glass box / transparency

Init does not integrate glass-box UI pages directly. Downstream chat may surface agent KBs and profile; init only creates them.
