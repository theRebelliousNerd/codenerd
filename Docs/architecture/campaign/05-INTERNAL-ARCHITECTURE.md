# 05 — Internal Architecture: campaign

> Last verified: **2026-07-13**  
> Companion: [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md)

## Component diagram

```
┌──────────────────────────────────────────────────────────────────┐
│                         Orchestrator                             │
│  state: campaign*, isRunning, isPaused, pauseCh, journalSeq      │
│  mu / resultsMu / writeSetLocks / riskGateState                  │
├──────────────┬──────────────┬──────────────┬─────────────────────┤
│ ContextPager │ Checkpoint   │ Replanner    │ Decomposer          │
│              │ Runner       │              │  (+ optional intel  │
│              │              │              │   board/edge/tools) │
├──────────────┴──────────────┴──────────────┴─────────────────────┤
│ TaskExecutor (session) │ tactile.Executor │ VirtualStore         │
│ Kernel (core)          │ LLMClient        │ ShardManager (mon)   │
└──────────────────────────────────────────────────────────────────┘
         │ Assert/Query/LoadFacts              │ Execute / RouteAction
         ▼                                     ▼
   Mangle program (policy + campaign_rules)   tools / FS / tests
```

## Data flow — decompose

```
DecomposeRequest{Goal, SourcePaths, Type, Hints, MaxPhases, ContextBudget}
  → [optional] IntelligenceGatherer.Gather → seedIntelligenceFacts
  → ingestSourceDocuments → FileMetadata + SourceDocument
  → seedDocFacts / ingestIntoKnowledgeStore
  → extractRequirementsSmart
  → llmProposePlan → RawPlan
  → buildCampaign → Campaign
  → [optional] AdvisoryBoard.ConsultAdvisors
  → kernel.LoadFacts(ToFacts)
  → validatePlan → issues / refine
  → DecomposeResult{Campaign, Issues, Requirements, ...}
```

## Data flow — execute one phase

```
runPhase(phase):
  loop:
    if complete && idle:
      checkpoint → replan|compress+completePhase+rollingWave
      return
    eligible = kernel eligible_task ∩ phase.Tasks \ backoff
    for task in eligible (until concurrency):
      lease = writeSetLocks.acquire(task.WriteSet)
      update status in_progress (facts)
      go runSingleTask:
        executeTask by type/shard
        success → store result, complete, micro-checkpoint?
        fail → handleTaskFailure (retry/backoff/replan path)
    wait results | 200ms | cancel | resume from pause
```

## State machine — campaign status

```
                 ┌──────────────┐
                 │  /planning   │  (decomposition epoch)
                 └──────┬───────┘
                        ▼
                 ┌──────────────┐
                 │ /decomposing │
                 └──────┬───────┘
                        ▼
                 ┌──────────────┐
                 │ /validating  │
                 └──────┬───────┘
                        ▼
            ┌───────────────────────┐
     ┌─────►│       /active         │◄────┐
     │      └───────────┬───────────┘     │
     │ Pause            │                 │ Resume
     │      ┌───────────▼───────────┐     │
     └──────┤       /paused         ├─────┘
            └───────────┬───────────┘
                        │ complete / fail / block
            ┌───────────┴───────────┐
            ▼                       ▼
     ┌────────────┐          ┌────────────┐
     │ /completed │          │  /failed   │
     └────────────┘          └────────────┘
```

Runtime `Run` primarily drives `/active` ⇄ `/paused` and terminal states. Decomposition statuses are set around plan creation in caller/decomposer flows.

## State machine — phase & task

Phases advance only when all tasks are completed or skipped, then checkpoint passes. Tasks:

```
pending ──schedule──► in_progress ──ok──► completed
    ▲                      │
    │         fail+retries │
    └── NextRetryAt ───────┘
                           └── max retries ──► failed
 soft skip paths ──► skipped
 deps unmet ──► blocked (kernel)
```

Restart: `resetInProgress` maps `in_progress` → `pending` for both levels.

## Key types (structural)

| Type | File | Responsibility |
|------|------|----------------|
| `Campaign` | `types.go` | Root plan + progress + optional Assault |
| `Phase` | `types.go` | Ordered unit + objectives + tasks + compression |
| `Task` | `types.go` | Atomic work + deps + write_set + attempts |
| `Orchestrator` | `orchestrator_types.go` | Runtime shell |
| `OrchestratorConfig` | same | DI + knobs |
| `ContextPager` | `context_pager.go` | Token reserves + activation |
| `Decomposer` | `decomposer.go` | Plan synthesis |
| `Replanner` | `replan.go` | Plan mutation |
| `CheckpointRunner` | `checkpoint.go` | Verification |
| `AssaultConfig` | `assault_types.go` | Stress campaign params |
| `CampaignRiskDecision` | `risk_scoring.go` | Gate inputs/outputs |
| `IntelligenceReport` | `intelligence_gatherer.go` | Pre-plan sensors |
| `Progress` / `OrchestratorEvent` | types / orchestrator_types | UI telemetry |

## Kernel query surface used at runtime

| Query predicate | Consumer | Purpose |
|-----------------|----------|---------|
| `current_phase` | phases | Active phase id |
| `eligible_task` | phases | Runnable tasks |
| `next_campaign_task` | phases | Preferential next (serial helper) |
| `phase_eligible` | phases | Start next phase |
| `campaign_blocked` | execution/tasks | Hard stop reason |
| `phase_context_atom` | pager | Phase context set |
| `replan_trigger` | failure/checkpoint | Replan signal |

Asserts include: `campaign_*`, `campaign_task`, `task_error`, `campaign_heartbeat`, `replan_trigger`, activation facts.

## Concurrency sketch

```
main Run loop
  ├─ heartbeat goroutine (tick progress/autosave)
  └─ runPhase
       ├─ task goroutine 1 ──┐
       ├─ task goroutine 2 ──┼──► results chan
       └─ task goroutine N ──┘
 writeSetLocks serializes overlapping paths across goroutines
 o.mu protects campaign mutation / pause / running flags
```

## Persistence protocol

```
mutate campaign in memory
  → appendJournal(snapshot_write_requested, checksum)
  → write temp file + sync + verify checksum
  → atomic rename to <id>.json
  → appendJournal(snapshot_write_committed)
```

## Extension points

| Hook | Setter / config | Effect |
|------|-----------------|--------|
| PromptProvider | `SetPromptProvider` | JIT system prompts for plan/replan |
| TaskExecutor | `SetTaskExecutor` / config | Clean execution loop |
| NorthstarObserver | `SetNorthstarObserver` | Alignment checks |
| IntelligenceGatherer | config/setter | Pre-plan sensors |
| AdvisoryBoard | config/setter | Expert votes |
| EdgeCaseDetector | config/setter | File actions |
| ToolPregenerator | config/setter | Tool synthesis |
| SpecialistKnowledgeProvider | setter | Domain atoms in task input |
| ProgressChan / EventChan | config | Live UI |

## Assault internal architecture

```
NewAdversarialAssaultCampaign
  → 4 phases hard-coded
executeAssaultDiscoverTask
  → discover targets by scope
  → write targets.json + batch_*.json
  → append /assault_batch tasks to phase 1
executeAssaultBatchTask
  → for each target × stage: runCommand / go test / nemesis
  → results + logs under assault/
executeAssaultTriageTask
  → summarize → remediation tasks on phase 3
```

Determinism at plan build; dynamism at discover/triage only.
