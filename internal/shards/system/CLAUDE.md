# internal/shards/system - Type S Permanent System Shards

This package implements Type S (System) permanent shards that provide core OODA loop functionality for codeNERD. These shards run continuously in the background, managing perception, decision-making, safety enforcement, and action routing.

**Related Packages:**
- [internal/core](../../core/CLAUDE.md) - Kernel, VirtualStore, ShardManager consumed by all shards
- [internal/perception](../../perception/CLAUDE.md) - Intent parsing used by PerceptionFirewallShard
- [internal/articulation](../../articulation/CLAUDE.md) - Piggyback Protocol for LLM response handling
- [internal/mangle/feedback](../../mangle/feedback/CLAUDE.md) - FeedbackLoop for rule generation

## Architecture

System shards implement the OODA loop (Observe → Orient → Decide → Act):
- **Perception** (Observe): NL → atoms transduction
- **Executive** (Orient/Decide): Strategy → action derivation via Mangle
- **Constitution** (Filter): Safety gate before action execution
- **Router** (Act): Action → tool mapping and execution
- **World Model**: Fact maintenance for codebase state
- **Planner**: Session/campaign orchestration

### Startup Modes
- **StartupAuto**: Starts when application initializes (Constitution, Executive, Perception)
- **StartupOnDemand**: Starts only when explicitly requested (Router, Planner, Legislator)

## File Index

| File | Description |
|------|-------------|
| `base.go` | BaseSystemShard embedding with common lifecycle and cost control. Exports StartupMode enum, CostGuard for LLM rate limiting (per-minute/session), and NewBaseSystemShard() constructor used by all system shards. |
| `constitution.go` | ConstitutionGateShard enforcing safety-critical permitted(Action) checks. Exports ConstitutionConfig with dangerous patterns (rm -rf, sudo, etc.), SecurityViolation/AppealRequest/AppealDecision types, and domain allowlisting. |
| `executive.go` | ExecutivePolicyShard as core OODA decision-maker deriving next_action from Mangle rules. Exports ExecutiveConfig, Strategy, ActionDecision types, and pattern tracking for Autopoiesis with FeedbackLoop integration. |
| `router.go` | TactileRouterShard mapping permitted actions to tools/virtual predicates. Exports ToolRoute with action patterns, timeouts, rate limits, and RequiresSafe flag, plus DefaultRouterConfig with 40+ built-in routes. |
| `perception.go` | PerceptionFirewallShard transducing user NL input to structured intent atoms. Exports PerceptionConfig with confidence/ambiguity thresholds, Intent and FocusResolution re-exports, and fallback regex patterns. |
| `world_model.go` | WorldModelIngestorShard maintaining file_topology, diagnostic, symbol_graph facts. Exports FileInfo, Diagnostic, Symbol, Dependency types and WorldModelConfig with include/exclude patterns for workspace scanning. |
| `planner.go` | SessionPlannerShard managing session agenda and multi-phase campaigns. Exports AgendaItem, Checkpoint, PlanView types for progress tracking, AddTask(), StartTask(), CompleteTask(), and GetCurrentPlan() methods. |
| `legislator.go` | LegislatorShard translating corrective feedback into durable Mangle policy rules. Exports llmClientAdapter wrapping types.LLMClient for FeedbackLoop, Piggyback Protocol processing, and hot-loading to learned.mg. |
| `mangle_repair.go` | MangleRepairShard validating and LLM-repairing Mangle rules before persistence. Exports RepairResult with attempts/fixes tracking, 4-stage validation (syntax, safety, schema, stratification), and max 3 repair retries. |
| `campaign_runner.go` | CampaignRunnerShard supervising long-horizon campaigns across process restarts. Exports CampaignRunnerConfig, SetWorkspaceRoot(), and automatic campaign resumption with exponential backoff on failure. |
| `learning_test.go` | Unit tests for autopoiesis learning infrastructure with LearningStoreAdapter. Tests Save/Load/LoadByPredicate interface compliance between store.LearningStore and core.LearningStore. |
| `planner_test.go` | Unit tests for PlanView progress tracking and agenda management. Tests AddTask(), task status transitions, and ProgressPct calculation. |
| `mangle_repair_test.go` | Unit tests for MangleRepairShard PredicateSelector wiring and repair prompts. Tests buildRepairPrompt() output structure with and without corpus. |
| `router_route_selection_test.go` | Unit tests for TactileRouterShard.findRoute() matching logic. Tests exact match preference, prefix vs contains, and action normalization (stripping /). |
| `action_pipeline_test.go` | Integration tests for pending_action → routing_result pipeline. Tests full Constitution → Router → VirtualStore flow with real kernel. |
| `policy_action_routes_test.go` | Unit tests verifying DefaultRouterConfig covers all policy-mapped actions. Tests /analyze_code, /fs_read, /delegate_* and other policy actions have routes. |

## Key Types

### BaseSystemShard
```go
type BaseSystemShard struct {
    ID           string
    Config       types.ShardConfig
    Kernel       *core.RealKernel
    LLMClient    types.LLMClient
    CostGuard    *CostGuard
    StartupMode  StartupMode
    learningStore core.LearningStore
}
```

### CostGuard
```go
type CostGuard struct {
    MaxLLMCallsPerMinute  int           // Default: 10
    MaxLLMCallsPerSession int           // Default: 100
    IdleTimeout           time.Duration // Auto-stop after inactivity
    MaxValidationRetries  int           // Per rule (default: 3)
    ValidationBudget      int           // Session-wide (default: 20)
}
```

### SecurityViolation
```go
type SecurityViolation struct {
    Timestamp    time.Time
    ActionType   string
    Target       string
    Reason       string
    WasEscalated bool
    ActionID     string // For appeal tracking
}
```

## System Shard Lifecycle

```
Application Start
    |
    +--[StartupAuto]
    |      |
    |      v
    |   Constitution.Start()  ─────┐
    |   Executive.Start()    ────┐ │
    |   Perception.Start()  ───┐ │ │
    |                          │ │ │
    +--[StartupOnDemand]       │ │ │
           |                   │ │ │
           v                   v v v
    On pending_action:     Continuous Loop:
    Router.Start()         - Tick every 50-100ms
    Planner.Start()        - Query kernel for facts
                           - Derive/emit new facts
                           - Track patterns
```

## OODA Loop Integration

```
User Input
    |
    v
PerceptionFirewallShard
    |
    v
user_intent fact → Kernel
    |
    v
ExecutivePolicyShard
    |
    v
next_action fact → Kernel
    |
    v
ConstitutionGateShard
    |
    +--[blocked]──> SecurityViolation
    |
    +--[permitted]
           |
           v
    TactileRouterShard
           |
           v
    VirtualStore.RouteAction()
           |
           v
    routing_result fact → Kernel
```

## Dependencies

- `internal/core` - Kernel, VirtualStore, PredicateCorpus
- `internal/perception` - Intent, FocusResolution types
- `internal/articulation` - PromptAssembler, ProcessLLMResponse
- `internal/mangle/feedback` - FeedbackLoop for rule generation
- `internal/campaign` - Orchestrator for campaign supervision
- `internal/world` - Scanner for world model ingestion

## Testing

```bash
go test ./internal/shards/system/...
```
