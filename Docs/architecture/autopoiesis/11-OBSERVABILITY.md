# 11 — Observability: Autopoiesis

> Last verified against codebase: **2026-07-13**

## 1. Logging

| Facility | Usage |
|----------|--------|
| `logging.CategoryAutopoiesis` | Primary category |
| `logging.Autopoiesis(...)` | Info-level stage/events |
| `logging.AutopoiesisDebug(...)` | Verbose details |
| `logging.Get(CategoryAutopoiesis).Error/Warn` | Failures |
| `logging.StartTimer(CategoryAutopoiesis, name)` | Timed spans |

### Notable log moments

- Orchestrator init + config echo  
- Ouroboros `=== OUROBOROS LOOP START/END ===` with success/stage/duration  
- Per-stage banners: specification, safety, thunderdome, simulation, compilation  
- Safety violation dumps on failure  
- Thunderdome ENTER / KILLED / SURVIVED  
- Kernel delegation process counts  
- Pattern high-confidence transitions  
- Grounding enabled for Gemini  

## 2. Metrics / stats structs

| Struct | Fields of interest |
|--------|--------------------|
| `OuroborosStats` | ToolsGenerated/Compiled/Rejected, SafetyViolations, ExecutionCount, Panics, ThunderdomeRuns/Kills/Survived, LastGeneration |
| `ThunderdomeStats` | TotalBattles, ToolsSurvived/Defeated, AttacksRun/Failed, AverageTimeMS, LongestBattle, MostDeadlyType |

Exposed via `GetOuroborosStats()` and Thunderdome internal stats.

## 3. Reasoning traces

- `TraceCollector` persists under `.nerd/tools/.traces/`  
- `ReasoningTrace` includes prompts, CoT steps, success/failure  
- Orchestrator wrappers: `StartToolTrace`, `FinalizeTrace`, `AnalyzeGenerations`  

## 4. Learning artifacts

- `.learnings/` JSON via `LearningStore`  
- UI Patterns/Learnings tabs (`autopoiesis_page.go`)  
- Kernel `tool_learning` / `tool_known_issue` for logic-side observability  

## 5. Operator surfaces

| Surface | What you see |
|---------|----------------|
| Chat Alt+A / `/autopoiesis` | Dashboard |
| `cmd_systems` Autopoiesis section | Status narrative |
| Verbose `-v` | Debug category output |
| Filesystem | tools, compiled, traces, profiles |

## 6. Glass box / transparency

Autopoiesis does not own transparency manager; chat boot initializes transparency separately. Correlate by timestamped Autopoiesis logs during tool gen.

## 7. Debug playbook

| Symptom | Where to look |
|---------|----------------|
| Tool never appears | logs for safety/thunderdome; registry restore; kernel `tool_registered` |
| Infinite-looking gen | MaxIters/halt warn logs; retry counters |
| Slow sessions | timers: ToolGeneration, SafetyCheck, Thunderdome, Commit |
| Learning not applied | `RefreshLearningsContext` logs; `.learnings` files |
| Delegation no-ops | “No kernel attached”; delegate_task query count |

## 8. Gaps

- No first-class Prometheus/OpenTelemetry metrics exporter in-package.  
- Thunderdome artifacts optional (`KeepArtifacts`) — default off.  
- Dual engines make “one query for all autopoiesis state” impossible without bridging.
