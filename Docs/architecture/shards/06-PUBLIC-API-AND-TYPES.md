# 06 — Public API and Types: shards

> Last verified against codebase: 2026-07-13  
> Exported surface that matters for integrators

## 1. Package `codenerd/internal/shards`

### Registration

| Symbol | Kind | File | Notes |
|--------|------|------|-------|
| `ShardPredicateManifest` | type | registration.go | Domain + OwnedPredicates |
| `DefaultShardPredicateManifests` | func | registration.go | Canonical ownership table |
| `RegistryContext` | type | registration.go | DI for factories |
| `RegisterAllShardFactories` | func | registration.go | Main registration entry |
| `RegisterSystemShardProfiles` | func | registration.go | Profiles only (manual factory path) |

### Matching

| Symbol | Kind | Notes |
|--------|------|-------|
| `AgentRegistry` / `RegisteredAgent` | types | `.nerd/agents.json` shape |
| `SpecialistMatch` | type | Match result + classification |
| `TechnologyPattern` | type | Pattern definition |
| `CoreTechnologyPatterns` | var | Built-in tech patterns |
| `AgentPatternMapping` | var | tech → expert name |
| `SpecialistExecutionMode` | type | executor/advisor/observer |
| `SpecialistKnowledgeTier` | type | technical/strategic/domain |
| `SpecialistClassification` | type | Capability flags |
| `DefaultSpecialistClassifications` | var | Built-in class map |
| `GetSpecialistClassification` | func | Lookup |
| `CanSpecialistExecute` / `IsExecutorSpecialist` / `IsStrategicAdvisor` | funcs | Predicates |
| `ShouldSpecialistExecuteTask` | func | conf-gated execute |
| `ExecutionMode` / `ModeParallel`… | types | Verb orchestration modes |
| `VerbSpecialistConfig` / `DefaultVerbConfigs` | types/var | Per-verb config |
| `GetExecutionMode` | func | Verb → mode |
| `MatchSpecialistsForTask` | func | Core matcher |
| `ShouldIncludeGenericShard` | func | Include generic persona |
| `GetAllPatterns` | func | Export patterns |

### Consultation

| Symbol | Kind | Notes |
|--------|------|-------|
| `ConsultationRequest` / `ConsultationResponse` | types | Protocol messages |
| `ConsultPriority` | type | Background/Normal/Urgent |
| `ConsultationSpawner` | interface | `SpawnConsultation` |
| `ConsultationManager` | type | Manager |
| `NewConsultationManager` | ctor | |
| `GetStrategicAdvisorsFor` | func | Advisors for executor |
| `ShouldConsultBeforeExecution` | func | Complexity gate |
| `FormatConsultationAdvice` | func | Prompt formatting |

### Observers

| Symbol | Kind | Notes |
|--------|------|-------|
| `ObserverEvent` / `ObserverEventType` | types | Event stream |
| `ObserverAssessment` / `AssessmentLevel` | types | Score outcomes |
| `GetAssessmentLevel` | func | Score → level |
| `ObserverCallback` | func type | Assessment handler |
| `NorthstarHandler` | interface | Direct Northstar |
| `BackgroundObserverManager` | type | Manager |
| `ObserverState` | type | Per-observer state |
| `ObserverSpawner` | interface | Spawn observer task |
| `NewBackgroundObserverManager` | ctor | |
| `FormatAssessment` | func | Display helper |

### Requirements interrogator

| Symbol | Kind | Notes |
|--------|------|-------|
| `RequirementsInterrogatorShard` | type | Ephemeral agent |
| `NewRequirementsInterrogatorShard` | ctor | |
| Methods | `SetLLMClient`, `SetPromptAssembler`, `Execute` | ShardAgent surface |

## 2. Package `codenerd/internal/shards/system`

### Base

| Symbol | Kind | Notes |
|--------|------|-------|
| `StartupMode` / `StartupAuto` / `StartupOnDemand` | type/const | Lifecycle mode |
| `CostGuard` | type | LLM rate limits |
| `NewCostGuard` | ctor | |
| `UnhandledCase` / `ProposedRule` / `AutopoiesisLoop` | types | Autopoiesis buffer |
| `NewAutopoiesisLoop` | ctor | |
| `BaseSystemShard` | type | Embeddable base |
| `NewBaseSystemShard` | ctor | |

Common methods on base: `GetID`, `GetState`, `SetState`, `GetConfig`, `Stop`, `SubscribeToFacts`, `SetLLMClient`, `SetParentKernel`, `SetVirtualStore`, `SetGlassBox`, `SetToolEventBus`, `SetToolStore`, `SetPromptAssembler`, `SetJITConfig`, `TryJITPrompt`, `SetLearningStore`, `GuardedLLMCall`, `EmitHeartbeat`.

### Perception

| Symbol | Kind | Notes |
|--------|------|-------|
| `Intent` / `FocusResolution` | type aliases | Re-export perception types |
| `PerceptionConfig` / `DefaultPerceptionConfig` | config | Thresholds |
| `LearningCandidateStore` | interface | Candidate persistence |
| `PerceptionFirewallShard` | type | Shard |
| `NewPerceptionFirewallShard` / `WithConfig` | ctors | |
| `SetClassificationClient` | method | Model tiering |

### Executive

| Symbol | Kind | Notes |
|--------|------|-------|
| `Strategy` / `ActionDecision` | types | Decision records |
| `ExecutiveConfig` / `DefaultExecutiveConfig` | config | Tick, barriers, max actions |
| `ExecutivePolicyShard` | type | Shard |
| `NewExecutivePolicyShard` / `WithConfig` | ctors | |
| `DisableBootGuard` / `IsBootGuardActive` | methods | Boot safety |
| `ResetValidationBudget` | method | Autopoiesis resume |
| `SetLearningStore` / `SetLearningCandidateStore` | methods | |
| `RecordActionOutcome` / `GetMetrics` | methods | Feedback |

### Constitution

| Symbol | Kind | Notes |
|--------|------|-------|
| `ConstitutionConfig` / `DefaultConstitutionConfig` | config | Strict, domains, patterns |
| `SecurityViolation` / `AppealRequest` / `AppealDecision` | types | Audit/appeal |
| `ConstitutionGateShard` | type | Shard |
| `NewConstitutionGateShard` / `WithConfig` | ctors | |
| `SubmitAppeal` / `HandleAppeal` / `GetViolations` | methods | Human override path |
| `AddAllowedDomain` / `AddDangerousPattern` | methods | Runtime config |

### Router

| Symbol | Kind | Notes |
|--------|------|-------|
| `ToolRoute` / `RouterConfig` / `ToolCall` | types | Routing |
| `DefaultRouterConfig` | func | Built-in routes |
| `TactileRouterShard` | type | Shard |
| `NewTactileRouterShard` / `WithConfig` | ctors | |
| `SetBrowserManager` | method | Browser tools |

### World / Planner / Campaign / Legislator / Repair

| Symbol | File | Key exports |
|--------|------|-------------|
| World model | world_model.go | `WorldModelIngestorShard`, configs, `FileInfo`, `Diagnostic`, `Symbol` |
| Planner | planner.go | `SessionPlannerShard`, `AgendaItem`, `Checkpoint`, `PlanView` |
| Campaign | campaign_runner.go | `CampaignRunnerShard`, `SetWorkspaceRoot`, `SetShardManager` |
| Legislator | legislator.go | `LegislatorShard`, `NewLegislatorShard` |
| Repair | mangle_repair.go | `MangleRepairShard`, `RepairResult`, `SetCorpus` |

### Non-exported but load-bearing

`encodeActionPayload` / `decodeActionPayload` in `payloads.go` — shared across executive/constitution/router; not part of public API but critical to cross-shard contracts.

## 3. Interfaces consumers implement

| Interface | Package | Purpose |
|-----------|---------|---------|
| `ConsultationSpawner` | shards | Spawn consult tasks |
| `ObserverSpawner` | shards | Spawn observer tasks |
| `NorthstarHandler` | shards | Direct alignment checks |
| `LearningCandidateStore` | system | Persist learning candidates |
| `types.ShardAgent` | types | Implemented by all shards |

## 4. Integration constraints

- System shards expect `*core.RealKernel` (or CortexKernel unwrap). Wrong kernel type logs error and runs without kernel.  
- VirtualStore injection type-asserts to `*core.VirtualStore`.  
- Prompt assembler often `*articulation.PromptAssembler`; base stores `any` to avoid cycles; perception prefers concrete type.  
