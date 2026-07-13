# transparency — Public API and Types

> Last verified: 2026-07-13  
> Package: `codenerd/internal/transparency`  
> Line refs are approximate anchors from source reads.

## 1. Constructors

| Func | File | Returns |
|------|------|---------|
| `NewTransparencyManager(cfg *config.TransparencyConfig)` | `transparency.go` | `*TransparencyManager` |
| `NewShardObserver()` | `shard_observer.go` | `*ShardObserver` |
| `NewSafetyReporter()` | `safety_reporter.go` | `*SafetyReporter` |
| `NewExplainer()` | `explainer.go` | `*Explainer` |
| `NewGlassBoxEventBus()` | `event_bus.go` | `*GlassBoxEventBus` |
| `NewToolEventBus()` | `glass_box_events.go` | `*ToolEventBus` |

## 2. TransparencyManager

```go
type TransparencyManager struct { /* unexported fields */ }
```

| Method | Behavior |
|--------|----------|
| `Enable()` | Master on; cascade observer (if ShardPhases) + safety |
| `Disable()` | Master off; disable subcomponents |
| `Toggle() bool` | Flip; returns new state |
| `IsEnabled() bool` | |
| `GetConfig() *config.TransparencyConfig` | Pointer to held config |
| `ShardObserver() *ShardObserver` | |
| `SafetyReporter() *SafetyReporter` | |
| `StartShard(id, type, task string)` | Gated |
| `UpdateShardPhase(id string, phase ShardPhase, message string)` | Gated |
| `EndShard(id string, failed bool)` | Gated |
| `ReportSafetyViolation(action, target, rule string) *SafetyViolation` | Gated; may nil |
| `GetStatus() string` | Markdown status |
| `FormatError(err error) string` | Classifies; verbosity gated |

## 3. Shard observation

### Types

| Type | Kind | Notes |
|------|------|-------|
| `ShardPhase` | int enum | Idle…Failed |
| `ShardExecution` | struct | Live state; `Duration`, `PhaseDuration`, `StatusLine` |
| `PhaseUpdate` | struct | Old/New phase + message + time |
| `PhaseObserver` | interface | `OnPhaseChange(PhaseUpdate)` |
| `ShardObserver` | struct | Tracker |

### ShardObserver methods

| Method | Notes |
|--------|-------|
| `Enable` / `Disable` / `IsEnabled` | |
| `AddObserver(PhaseObserver)` | |
| `StartExecution` / `UpdatePhase` / `SetProgress` / `EndExecution` | |
| `GetExecution` | Returns copy or nil |
| `GetActiveExecutions` | Non-terminal copies |
| `GetPhaseHistory(limit)` | Recent |
| `ClearHistory` | |
| `FormatExecutionSummary` | Joined status lines |

### ShardPhase values

`PhaseIdle`, `PhaseInitializing`, `PhaseLoading`, `PhaseAnalyzing`, `PhaseGenerating`, `PhaseExecuting`, `PhaseComplete`, `PhaseFailed` — each with `String()`.

## 4. Safety reporting

| Type | Notes |
|------|-------|
| `SafetyViolationType` | enum + `String()` |
| `SafetyViolation` | full context fields |
| `SafetyReporter` | history + classify |

Methods: `Enable`/`Disable`, `ReportViolation`, `GetRecentViolations`, `GetViolation`, `FormatViolation`, `ClearHistory`.

Free function:

| Func | Purpose |
|------|---------|
| `ExplainSafetyAction(action string) string` | Hypothetical risk markdown |

Violation types: `ViolationDestructiveAction`, `ViolationProtectedPath`, `ViolationSecretExposure`, `ViolationResourceLimit`, `ViolationPolicyRule`, `ViolationUnauthorized`, `ViolationUnknown`.

## 5. Error classification

| Type | Notes |
|------|-------|
| `ErrorCategory` | enum + `Prefix()` + `String()` |
| `ClassifiedError` | `Error()`, `Unwrap()`, `Format()` |

| Func | Purpose |
|------|---------|
| `ClassifyError(err error) *ClassifiedError` | nil-safe |
| `GetRecoveryGuide(category ErrorCategory) []string` | remediation steps |

Categories: Safety, Config, API, Kernel, Shard, Filesystem, Network, Timeout, Unknown.

## 6. Explainer

| Method / Func | Purpose |
|---------------|---------|
| `SetMaxDepth` / `SetShowDetails` | Config |
| `ExplainTrace(*mangle.DerivationTrace) string` | Full tree markdown |
| `ExplainFact(trace, predicate) string` | Filter by predicate |
| `ExplainDecision(action, trace) string` | Narrative; prefers next_action |
| `QuickExplain(predicate string, args []any) string` | One-liner |
| `FormatOperationSummary(*OperationSummary) string` | Post-op markdown |

```go
type OperationSummary struct {
    Operation, Duration, Outcome, Details string
    FilesAffected, RulesApplied, NextSteps []string
}
```

## 7. Glass Box events

### Categories

```go
const (
    CategoryPerception GlassBoxCategory = "perception"
    CategoryKernel     GlassBoxCategory = "kernel"
    CategoryJIT        GlassBoxCategory = "jit"
    CategoryShard      GlassBoxCategory = "shard"
    CategoryControl    GlassBoxCategory = "control"
    CategoryRouting    GlassBoxCategory = "routing"
)
```

| API | Purpose |
|-----|---------|
| `(GlassBoxCategory).String` / `DisplayPrefix` | Display |
| `AllCategories()` | Slice of all |
| `ValidCategory(s string) bool` | Membership |

```go
type GlassBoxEvent struct {
    ID uint64
    Timestamp time.Time
    Category GlassBoxCategory
    Summary, Details string
    TurnID int
    Duration time.Duration
    Source string
}
// String(), HasDetails()
```

## 8. GlassBoxEventBus

| Method | Purpose |
|--------|---------|
| `Enable` / `Disable` / `IsEnabled` | Master bus |
| `SetVerbose` / `IsVerbose` | Immediate path |
| `SetCategories([]GlassBoxCategory)` | Empty = all |
| `Subscribe() <-chan GlassBoxEvent` | Buffer 512 |
| `Unsubscribe(ch <-chan GlassBoxEvent)` | Close + remove |
| `Emit` / `EmitImmediate` | Producers |
| `Flush` | Force batch out |
| `ClearTurn(turnID int)` | Buffer filter |
| `Close` | Shutdown all |
| `Stats() GlassBoxBusStats` | Snapshot |

```go
type GlassBoxBusStats struct {
    Enabled bool
    SubscriberCount, BufferedEvents, CategoryCount int
    TotalEmitted uint64
    Verbose bool
}
```

## 9. Tool events

```go
type ToolEvent struct {
    ToolName, Result string
    Success bool
    Duration time.Duration
    Timestamp time.Time
}

type ToolEventBus struct { /* chan */ }
// NewToolEventBus, Emit, Subscribe, Close
```

## 10. Stability notes for consumers

**Relatively stable contracts**

- Category string values  
- ToolEvent always-on semantics  
- Non-blocking emit  
- Manager Enable/Disable/Toggle/IsEnabled  

**More likely to evolve**

- Extra Glass Box categories  
- ClassifiedError heuristics  
- Safety violation taxonomy  
- Stats fields (drop counts)  
- OperationSummary producer conventions  

**Breaking-change caution**

- Renaming category strings breaks filters and tests in chat  
- Changing `Subscribe` buffer semantics under load affects TUI lag  
- Closing bus channels requires coordinated chat teardown  

## 11. What is intentionally unexported

- `containsAny`, `containsAnyWord`, `boolToStatus`, `formatFactForHuman`, `explainRule`, `notifyObservers`, `flushLocked`, `classifyViolation`  

Consumers should not fork classification by reimplementing these; extend exported APIs instead.
