# internal/northstar - Northstar Vision Guardian

The Northstar package implements the permanent system guardian that monitors project activity for alignment with the defined vision. Unlike user-defined specialists in `.nerd/agents/`, Northstar is a core codeNERD system component.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     NORTHSTAR GUARDIAN                          │
│                   (Permanent System Agent)                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────────────┐  ┌──────────────────┐  ┌───────────────┐ │
│  │ northstar_       │  │  Guardian        │  │  Observers    │ │
│  │ knowledge.db     │  │  (Core Logic)    │  │  (Campaign/   │ │
│  │                  │  │                  │  │   Task)       │ │
│  └────────┬─────────┘  └────────┬─────────┘  └───────┬───────┘ │
│           │                     │                     │         │
│           └─────────────────────┼─────────────────────┘         │
│                                 ▼                               │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │                    Integration Points                     │  │
│  ├──────────────────────────────────────────────────────────┤  │
│  │ • /init          → Creates DB, schema                    │  │
│  │ • /northstar     → Populates vision via wizard           │  │
│  │ • TUI boot       → Loads vision into kernel              │  │
│  │ • Campaign       → CampaignObserver + phase gates        │  │
│  │ • Task execution → TaskObserver + periodic checks        │  │
│  │ • /alignment     → On-demand check                       │  │
│  └──────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

## File Index

| File | Description |
|------|-------------|
| `types.go` | Core type definitions: Vision, Observation, AlignmentCheck, DriftEvent, GuardianConfig, GuardianState |
| `store.go` | Store for northstar_knowledge.db with CRUD operations for vision, observations, checks, drift events |
| `guardian.go` | Guardian agent with alignment checking, observation recording, and periodic check logic |
| `observer.go` | CampaignObserver, TaskObserver, and BackgroundEventHandler for integration with campaigns, tasks, and BackgroundObserverManager |

## Storage

The Northstar store uses a dedicated SQLite database separate from the general knowledge base:

```
.nerd/
├── northstar.mg              # Mangle facts (vision definition)
├── northstar.json            # JSON backup
├── northstar_knowledge.db    # Dedicated Northstar KB (this package)
└── knowledge.db              # General project knowledge
```

### Database Schema

| Table | Purpose |
|-------|---------|
| `vision` | Single-row table with the complete vision definition |
| `observations` | Things Northstar noticed during sessions |
| `alignment_checks` | History of alignment validations |
| `drift_events` | Detected drift from vision (unresolved/resolved) |
| `guardian_state` | Runtime state (last check, task count, alignment score) |
| `ingested_docs` | Documents ingested for vision context |

## Key Types

### Vision
```go
type Vision struct {
    Mission      string       // One-line mission statement
    Problem      string       // Problem being solved
    VisionStmt   string       // Success state description
    Personas     []Persona    // Target users
    Capabilities []Capability // Planned features
    Risks        []Risk       // Identified risks
    Requirements []Requirement// Formal requirements
    Constraints  []string     // Project constraints
}
```

### AlignmentCheck
```go
type AlignmentCheck struct {
    ID          string           // Unique check ID
    Timestamp   time.Time        // When check was performed
    Trigger     AlignmentTrigger // What triggered the check
    Subject     string           // What was checked
    Result      AlignmentResult  // passed/warning/failed/blocked
    Score       float64          // 0.0-1.0 alignment score
    Explanation string           // LLM explanation
    Suggestions []string         // Improvement suggestions
}
```

### Alignment Triggers

| Trigger | When |
|---------|------|
| `TriggerManual` | User ran `/alignment` |
| `TriggerPhaseGate` | Campaign phase transition |
| `TriggerPeriodic` | Every N tasks (default: 5) |
| `TriggerHighImpact` | High-impact files modified |
| `TriggerTaskComplete` | After significant task |
| `TriggerCampaignStart` | New campaign started |

### Alignment Results

| Result | Score Range | Meaning |
|--------|-------------|---------|
| `AlignmentPassed` | ≥0.7 | Fully aligned |
| `AlignmentWarning` | 0.5-0.7 | Minor drift detected |
| `AlignmentFailed` | 0.3-0.5 | Significant drift |
| `AlignmentBlocked` | <0.3 | Cannot proceed |

## Usage

### Creating the Store (at /init)

```go
store, err := northstar.NewStore(nerdDir)
if err != nil {
    return err
}
defer store.Close()
```

### Initializing the Guardian

```go
config := northstar.DefaultGuardianConfig()
guardian := northstar.NewGuardian(store, config)
guardian.SetLLMClient(llmClient)
if err := guardian.Initialize(); err != nil {
    return err
}
```

### Campaign Integration

```go
// Create observer for campaign
observer := northstar.NewCampaignObserver(guardian)

// Start campaign
if err := observer.StartCampaign(ctx, campaignID, goal); err != nil {
    // Campaign goal doesn't align - may be blocked
}

// On phase transition
check, err := observer.OnPhaseStart(ctx, "implementation", "Build core features")
if check != nil && check.Result == northstar.AlignmentBlocked {
    // Phase doesn't align - requires attention
}

// On task completion
check, err := observer.OnTaskComplete(ctx, taskID, taskDesc, result, modifiedFiles)
```

### Task Integration

```go
// Create observer for session
taskObserver := northstar.NewTaskObserver(guardian, sessionID)

// On task completion
check, err := taskObserver.OnTaskComplete(ctx, "coder", taskDesc, result, modifiedFiles)
if check != nil {
    // Periodic or high-impact check was performed
}
```

### On-Demand Alignment Check

```go
check, err := guardian.CheckAlignment(ctx, northstar.TriggerManual,
    "Proposed change to authentication system",
    "Adding OAuth2 support")
```

## Configuration

```go
type GuardianConfig struct {
    PeriodicCheckInterval int      // Tasks between checks (default: 5)
    EnablePhaseGates      bool     // Check at phase transitions (default: true)
    EnablePeriodicCheck   bool     // Check every N tasks (default: true)
    EnableHighImpact      bool     // Check high-impact changes (default: true)
    HighImpactPaths       []string // Paths that trigger checks
    WarningThreshold      float64  // Score for warning (default: 0.7)
    FailureThreshold      float64  // Score for failure (default: 0.5)
    BlockThreshold        float64  // Score for block (default: 0.3)
}
```

## Integration Points

### /init
- Creates `northstar_knowledge.db` with schema
- Initializes guardian state

### /northstar wizard
- Populates vision via `store.SaveVision()`
- Updates guardian state

### TUI boot
- Loads vision into kernel
- Initializes Guardian with LLM client

### Campaign orchestrator
- Uses CampaignObserver for alignment monitoring
- Phase gates can block progression

### Delegation/task execution
- Uses TaskObserver for periodic checks
- High-impact file changes trigger checks

### /alignment command
- Manual alignment check via `guardian.CheckAlignment()`

### Background Observer Manager
- Wired via `BackgroundEventHandler` implementing `shards.NorthstarHandler`
- Receives events from shard execution (task_completed, task_failed)
- Triggers periodic alignment checks every 5 minutes
- Uses Guardian for LLM-based alignment evaluation

## Prompt Atoms

Northstar prompt atoms are in `internal/prompt/atoms/northstar/`:
- `vision_alignment.yaml` - Core alignment guidance
- `goal_tracking.yaml` - Goal tracking atoms
- `requirements.yaml` - Requirements handling
- `doc_ingestion.yaml` - Document ingestion
- `architecture_roadmap_validation.yaml` - Architecture validation
- `alignment_check.yaml` - **NEW** Mandatory JIT atoms for campaign awareness, drift detection, high-impact gates

These define HOW to be a Northstar guardian. The project-specific WHAT is stored in the knowledge DB.

---

**Remember: Push to GitHub regularly!**
