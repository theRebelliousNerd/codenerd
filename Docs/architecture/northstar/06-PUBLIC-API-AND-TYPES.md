# 06 — Public API and Types: Northstar

> Last verified against codebase: 2026-07-13  
> Package: `codenerd/internal/northstar`

## 1. Construction

| Symbol | File | Role |
|--------|------|------|
| `NewStore(nerdDir string) (*Store, error)` | `store.go` | Opens/creates `nerdDir/northstar_knowledge.db` |
| `NewGuardian(store *Store, config GuardianConfig) *Guardian` | `guardian.go` | In-memory guardian; call `Initialize` |
| `DefaultGuardianConfig() GuardianConfig` | `types.go` | Sensible thresholds and high-impact paths |
| `NewCampaignObserver(g *Guardian) *CampaignObserver` | `observer.go` | Campaign lifecycle alignment |
| `NewTaskObserver(g *Guardian, sessionID string) *TaskObserver` | `observer.go` | Non-campaign tasks |
| `NewBackgroundEventHandler(g *Guardian, sessionID string) *BackgroundEventHandler` | `observer.go` | BackgroundObserver integration |

## 2. Interfaces (dependency inversion)

### `KernelClient` (`guardian.go`)

```text
Assert(fact types.Fact) error
Retract(predicate string) error
```

Implemented by core kernel adapters at call sites; optional.

### `LLMClient` (`guardian.go`)

```text
CompleteWithSystem(ctx context.Context, system, user string) (string, error)
```

Any chat client with system+user completion (e.g. session LLM wrapper).

## 3. Guardian methods

| Method | Purpose |
|--------|---------|
| `SetLLMClient` | Inject judge |
| `SetParentKernel` | Inject fact sink |
| `Initialize` | Load vision/state; warn if no vision; refresh facts |
| `HasVision` / `GetVision` / `GetState` | Read accessors (cloned) |
| `UpdateVision` | Persist + memory + facts |
| `CheckAlignment` | Full alignment pipeline |
| `ObserveTaskCompletion` / `ObserveFileChange` / `ObserveDecision` | Record observations |
| `ShouldCheckNow` | Trigger policy |
| `OnTaskComplete` | Increment counter; maybe periodic check |

## 4. Store methods

| Method | Purpose |
|--------|---------|
| `Close` / `Path` | Lifecycle |
| `SaveVision` / `LoadVision` / `HasVision` | Vision singleton |
| `RecordObservation` / `GetRecentObservations` | Observation log |
| `RecordAlignmentCheck` / `GetAlignmentHistory` | Check history |
| `RecordDriftEvent` / `ResolveDriftEvent` / `GetActiveDriftEvents` | Drift lifecycle |
| `GetState` / `IncrementTaskCount` / `ResetSessionObservations` | Guardian state |

No public API for `ingested_docs`.

## 5. Domain types (`types.go`)

### Vision graph

| Type | Fields of note |
|------|----------------|
| `Vision` | Mission, Problem, VisionStmt, Personas, Capabilities, Risks, Requirements, Constraints, timestamps |
| `Persona` | Name, PainPoints, Needs |
| `Capability` | ID, Description, Timeline (`now`/`next`/`later`), Priority |
| `Risk` | ID, Description, Likelihood, Impact, Mitigation |
| `Requirement` | ID, Type, Description, Priority |

`Vision.ToFacts() []types.Fact` — Mangle projection (see §7).

### Runtime records

| Type | Role |
|------|------|
| `Observation` | Session event with relevance 0–1 |
| `ObservationType` | task_completed, file_changed, decision_made, pattern_detected, drift_warning, alignment_success, risk_triggered |
| `AlignmentCheck` | Trigger, subject, result, score, explanation, suggestions, duration |
| `AlignmentTrigger` | manual, phase_gate, periodic, high_impact, task_complete, session_start, campaign_start |
| `AlignmentResult` | passed, warning, failed, blocked, skipped |
| `DriftEvent` | Severity, category, evidence, related check, resolution |
| `DriftSeverity` | minor, moderate, major, critical |
| `GuardianConfig` | intervals, enables, paths, thresholds, AlignmentModel |
| `GuardianState` | vision_defined, last_check, tasks_since_check, active_drift_count, overall_alignment, session_observations |

## 6. Observer surface

### CampaignObserver

`StartCampaign`, `OnPhaseStart`, `OnPhaseComplete`, `OnTaskComplete`, `EndCampaign`, `GetPhaseCheck`, `GetAllPhaseChecks`.

### TaskObserver

`OnTaskStart`, `OnTaskComplete`, `OnError`.

### BackgroundEventHandler

`HandleEvent(ctx, eventType, source, target, details, timestamp) (*ObserverAssessment, error)`.

Local mirror types (not `shards.*`):

- `ObserverAssessment` — score 0–100, Level string, VisionMatch, Suggestions, …
- `ObserverEvent` — Type, Source, Target, Details, Timestamp (defined for documentation/tests; handler takes scalar args)

## 7. Fact shape reference (`ToFacts`)

| Predicate | Args (conceptually) |
|-----------|---------------------|
| `northstar_mission` | `"global"`, statement |
| `northstar_problem` | `"global"`, description |
| `northstar_vision` | `"global"`, statement |
| `northstar_persona` | `persona_<Name>`, name |
| `northstar_pain_point` | persona id, text |
| `northstar_need` | persona id, text |
| `northstar_capability` | id, description, `/timeline`, priority number |
| `northstar_risk` | id, description, `/likelihood`, impact number |
| `northstar_mitigation` | risk id, `/mitigation` (constant atom; free-text strategy not encoded) |
| `northstar_requirement` | id, `/type`, description, priority number |
| `northstar_constraint` | `constraint_N`, text |
| `northstar_defined` | (no args) |

Priority map: critical/must_have→100, high/should_have→80, medium→50, low/nice_to_have→20, default 50.  
Impact map: high→100, medium→50, low→20, default 50.

## 8. What callers outside the package should use

| Integrator goal | Preferred API |
|-----------------|---------------|
| Boot guardian | `NewStore` → `NewGuardian` → `SetLLMClient` → `SetParentKernel` → `Initialize` |
| Manual check | `CheckAlignment(..., TriggerManual, ...)` |
| Campaign | `NewCampaignObserver` + orchestrator `SetNorthstarObserver` |
| Background | `NewBackgroundEventHandler` + adapter to `shards.NorthstarHandler` |
| Mutate vision | `Guardian.UpdateVision` (not raw SQL) |
