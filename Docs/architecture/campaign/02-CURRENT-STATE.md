# 02 — Current State: campaign

> Last verified: **2026-07-13**  
> Source of truth: `internal/campaign/` on disk

## Package summary

| Metric | Value (approx.) |
|--------|-----------------|
| Non-test `.go` sources | ~45 |
| `*_test.go` files | ~29 |
| Local `.mg` in package | 0 production rules (debug dump may exist: `debug_program_ERROR.mg`) |
| Policy companions | `internal/core/defaults/campaign_rules.mg` (+ policy Section 19, topology) |
| Primary binary consumers | `cmd/nerd` campaign cmds, chat/UI |

## File inventory by subsystem

### Orchestrator core

| File | Role |
|------|------|
| `orchestrator.go` | Package marker + modular map only |
| `orchestrator_types.go` | `Orchestrator`, `OrchestratorConfig`, events |
| `orchestrator_init.go` | `NewOrchestrator`, validation, setters, defaults |
| `orchestrator_lifecycle.go` | Load/Set/save/resetInProgress |
| `orchestrator_execution.go` | `Run`, heartbeat/autosave |
| `orchestrator_control.go` | Pause/Resume/Stop/GetProgress |
| `orchestrator_phases.go` | Kernel phase/task queries, start/complete phase |
| `orchestrator_tasks.go` | `runPhase`, rolling-wave, `runSingleTask` |
| `orchestrator_task_handlers.go` | Type routing + shard/file/etc handlers |
| `orchestrator_task_results.go` | Result cache + context injection |
| `orchestrator_task_transaction.go` | Task transaction helpers |
| `orchestrator_failure.go` | Retries, logic escalation, repro tasks |
| `orchestrator_journal.go` | Journal + atomic snapshot |
| `orchestrator_utils.go` | Checkpoints helper, events, concurrency limits |

### Planning

| File | Role |
|------|------|
| `decomposer.go` | `Decompose` pipeline + intel seed |
| `decomposer_documents.go` | Ingest/classify/knowledge DB |
| `decomposer_requirements.go` | Requirements extraction |
| `decomposer_planning.go` | LLM plan, build, validate, refine |
| `replan.go` | Adaptive replan + refine next phase |
| `document_ingestor.go` | Embedding-backed ingest helper |
| `campaign_prompts.go` | Roles + PromptProvider |
| `prompts.go` | Large static prompt body |

### Context & verification

| File | Role |
|------|------|
| `context_pager.go` | Budget reserves, activate/compress/prefetch |
| `checkpoint.go` | Verification methods |
| `micro_checkpoint.go` | Per-task micro checks |
| `campaign_fact_sync.go` | Fact sync helpers |

### Safety / intelligence

| File | Role |
|------|------|
| `risk_scoring.go` | Deterministic score + gate evaluation |
| `write_set_lock_manager.go` | Parallel write locking |
| `intelligence_gatherer.go` | Report types + gather entry |
| `intelligence_gathering_methods.go` | Per-system gather methods |
| `intelligence_formatting.go` | Prompt formatting |
| `shard_advisory_board.go` | Advisor consult/synthesize |
| `edge_case_detector.go` | File action decisions |
| `tool_pregenerator.go` | Tool gap pregen |
| `specialist_knowledge.go` | Specialist DB injection |

### Assault

| File | Role |
|------|------|
| `assault_types.go` | Scope/stage/config |
| `assault_campaign.go` | Deterministic campaign builder |
| `assault_tasks.go` | Discover/batch/triage handlers |
| `assault_prompts.go` | Assault-related prompts |

### Domain & support

| File | Role |
|------|------|
| `types.go` | Campaign/Phase/Task + ToFacts |
| `task_mutation_types.go` | Mutation classification helpers |
| `normalization.go` / `utils.go` | Path/string helpers |
| `errors.go` | Sentinels |
| `README.md` | Human package overview (partially stale) |

### Tests (representative)

| Pattern | Coverage focus |
|---------|----------------|
| `orchestrator_*_test.go` | DI, phases, failure, journal, write-set gating, behavior |
| `decomposer*_test.go` | Planning/helpers |
| `context_pager_test.go` | Budget/activate (includes thread-safe mock kernel) |
| `replan_test.go` | Replan paths |
| `risk_scoring_test.go` | Gate math |
| `assault_*_test.go` | Helpers/tasks |
| `intelligence_*_test.go` / `edge_case_*_test.go` | Gather/edge gaps |
| `checkpoint*_test.go` | Parsers/integration |
| `write_set_lock_manager_test.go` | Lock concurrency |
| `mocks_test.go` / `main_test.go` | Shared fakes |

## Hotspots (behavioral)

1. **`orchestrator_tasks.go` + `orchestrator_execution.go`** — live scheduler; most runtime complexity.  
2. **`decomposer.go` + planning/requirements** — plan quality bottleneck; many soft dependencies.  
3. **`assault_tasks.go`** — durable I/O heavy; long-running.  
4. **`risk_scoring.go`** — gate behavior operators may not see without logs.  
5. **`prompts.go`** — large static surface competing with JIT atoms.

## On-disk runtime layout

```
.nerd/campaigns/
  <campaignID>.json              # snapshot
  <campaignID>.journal.jsonl     # event journal
  <slug>/
    knowledge.db                 # per-campaign knowledge (when used)
    assault/                     # assault artifacts only
      targets.json
      batches/
      results/
      logs/
      triage/
```

## What is *not* in this package

- Base campaign state-machine rules (kernel defaults / policy)  
- Cobra command definitions (cmd)  
- Session single-turn executor implementation (session owns TE)  
- UI rendering (cmd/nerd/ui)

## Currency notes

- Modular orchestrator split is real; `orchestrator.go` is intentionally empty of logic.  
- README architecture version “2.0.0 (December 2024)” is historical — code has advanced (risk, journal, assault, intelligence). Prefer this corpus for currency.
