# 05 — Internal Architecture: Northstar

> Last verified against codebase: 2026-07-13  
> Source: `internal/northstar/{types,store,guardian,observer}.go`

## 1. Component diagram

```mermaid
flowchart TB
  subgraph surfaces [External surfaces]
    Wizard[Chat /northstar wizard]
    CLI[nerd northstar]
    Camp[Campaign Orchestrator]
    BG[BackgroundObserverManager]
    Align[/alignment slash]
  end

  subgraph northstar_pkg [internal/northstar]
    Store[(Store SQLite)]
    Guardian[Guardian]
    CampObs[CampaignObserver]
    TaskObs[TaskObserver]
    BGHandler[BackgroundEventHandler]
  end

  subgraph platform [Platform]
    Kernel[Mangle Kernel via KernelClient]
    LLM[LLMClient]
  end

  Wizard -.->|JSON/MG today| FS[(.nerd/northstar.json)]
  CLI --> FS
  Align --> Store
  Align --> Guardian
  BG --> BGHandler
  Camp --> CampObs
  CampObs --> Guardian
  TaskObs --> Guardian
  BGHandler --> Guardian
  Guardian --> Store
  Guardian --> LLM
  Guardian --> Kernel
```

## 2. Data flow: alignment check

```
CheckAlignment(ctx, trigger, subject, context)
  │
  ├─ RLock clone vision + llm
  ├─ no vision  → Skipped / 1.0 → persist → return
  ├─ no llm     → Passed / 0.8  → persist → return
  ├─ build system + user prompts from vision
  ├─ llm.CompleteWithSystem
  │     └─ error → Warning / 0.7 → persist → return
  ├─ parseAlignmentResponse (SCORE/RESULT/EXPLANATION/SUGGESTIONS)
  │     └─ if no RESULT, classifyScore(score)
  └─ persistAlignmentOutcome
        ├─ RecordAlignmentCheck → EWMA overall_alignment (0.8 old + 0.2 new)
        ├─ refreshState
        └─ if Failed|Blocked → RecordDriftEvent + refreshState
```

## 3. Data flow: kernel projection

```
Initialize / UpdateVision
  → store load/save
  → in-memory vision/state (cloned)
  → refreshKernelFacts:
       for p in northstar_* predicates: Retract(p)
       if vision != nil: Assert each Vision.ToFacts()
```

Predicates retracted/asserted:

`northstar_mission`, `northstar_problem`, `northstar_vision`, `northstar_persona`, `northstar_pain_point`, `northstar_need`, `northstar_capability`, `northstar_risk`, `northstar_mitigation`, `northstar_requirement`, `northstar_constraint`, `northstar_defined`.

## 4. Observer state machines

### 4.1 CampaignObserver

| Event | Behavior |
|-------|----------|
| `StartCampaign` | Reset phase maps; if vision, alignment with `TriggerCampaignStart`; **blocked → error** |
| `OnPhaseStart` | First phase skips (already checked at start); else phase-gate check; **blocked → error** |
| `OnPhaseComplete` | Observation only |
| `OnTaskComplete` | Observe task + files; every N tasks or high-impact paths → check |
| `EndCampaign` | Terminal observation |

Config: copies `EnablePhaseGates` and `PeriodicCheckInterval` from guardian at construction.

### 4.2 TaskObserver

| Event | Behavior |
|-------|----------|
| `OnTaskStart` | Debug log only |
| `OnTaskComplete` | Observe + `Guardian.OnTaskComplete` (task counter / periodic) |
| `OnError` | Observation type `pattern_detected` |

### 4.3 BackgroundEventHandler

Maps event types → triggers:

| Event type | Trigger |
|------------|---------|
| `task_completed` | `TriggerTaskComplete` |
| `campaign_phase` | `TriggerPhaseGate` |
| `file_modified` | `TriggerHighImpact` |
| `alignment_check` / default | `TriggerPeriodic` |

Result → level: passed→`proceed`, warning→`note`, failed→`clarify`, blocked→`block`.

## 5. GuardianConfig defaults

From `DefaultGuardianConfig()`:

| Field | Default |
|-------|---------|
| `PeriodicCheckInterval` | 5 tasks |
| `EnablePhaseGates` | true |
| `EnablePeriodicCheck` | true |
| `EnableHighImpact` | true |
| `HighImpactPaths` | `internal/core/`, `internal/session/`, `internal/perception/`, `cmd/nerd/`, `*.mg` |
| `WarningThreshold` | 0.7 |
| `FailureThreshold` | 0.5 |
| `BlockThreshold` | 0.3 |
| `AlignmentModel` | `""` (unused by Guardian) |

Score bands (`classifyScore`):  
`score ≥ warning → passed`; `≥ failure → warning`; `≥ block → failed`; else `blocked`.

## 6. Store rollups (`guardian_state`)

| Event | State effect |
|-------|----------------|
| Save vision | `vision_defined = 1` |
| Record observation | `session_observations++` |
| Record alignment check | `last_check`, `tasks_since_check=0`, EWMA alignment |
| Record drift | `active_drift_count++` |
| Resolve drift | `active_drift_count--` (if was unresolved) |
| IncrementTaskCount | `tasks_since_check++` |

## 7. Concurrency model

- `Guardian.mu` — RWMutex around vision, state, llm, kernel pointers.
- `Store.mu` — RWMutex around DB access (serialize writers).
- `CampaignObserver.mu` / `TaskObserver.mu` — local counters and maps.
- Clone-on-read prevents callers from racing the Guardian’s vision pointer.

## 8. Relevance heuristics

- **Text**: fraction of vision mission/problem/vision words (len>3) contained in subject text.
- **Paths**: 0.9 if matches high-impact pattern, else 0.5.
- Comments in code mark this as placeholder for embeddings.
