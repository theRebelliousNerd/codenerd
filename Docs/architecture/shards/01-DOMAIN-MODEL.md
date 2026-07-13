# shards — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/shards/` (complete internal coverage)
> **Implementation: `internal/shards/` — 18 non-test .go, 24 tests, 1 .mg**


## Package

`internal/shards/`

## Exported types (sampled, up to 40)

| Type | Location |
|------|----------|
| `ConsultationRequest` | `internal/shards/consultation.go:21` |
| `ConsultationResponse` | `internal/shards/consultation.go:33` |
| `ConsultPriority` | `internal/shards/consultation.go:47` |
| `ConsultationSpawner` | `internal/shards/consultation.go:59` |
| `ConsultationManager` | `internal/shards/consultation.go:64` |
| `AgentRegistry` | `internal/shards/matching.go:19` |
| `RegisteredAgent` | `internal/shards/matching.go:26` |
| `SpecialistMatch` | `internal/shards/matching.go:38` |
| `TechnologyPattern` | `internal/shards/matching.go:49` |
| `SpecialistExecutionMode` | `internal/shards/matching.go:170` |
| `SpecialistKnowledgeTier` | `internal/shards/matching.go:184` |
| `SpecialistClassification` | `internal/shards/matching.go:198` |
| `ExecutionMode` | `internal/shards/matching.go:337` |
| `VerbSpecialistConfig` | `internal/shards/matching.go:361` |
| `ObserverEvent` | `internal/shards/observer_manager.go:20` |
| `ObserverEventType` | `internal/shards/observer_manager.go:29` |
| `ObserverAssessment` | `internal/shards/observer_manager.go:44` |
| `AssessmentLevel` | `internal/shards/observer_manager.go:57` |
| `ObserverCallback` | `internal/shards/observer_manager.go:81` |
| `NorthstarHandler` | `internal/shards/observer_manager.go:85` |
| `BackgroundObserverManager` | `internal/shards/observer_manager.go:91` |
| `ObserverState` | `internal/shards/observer_manager.go:127` |
| `ObserverSpawner` | `internal/shards/observer_manager.go:137` |
| `ShardPredicateManifest` | `internal/shards/registration.go:33` |
| `RegistryContext` | `internal/shards/registration.go:81` |
| `RequirementsInterrogatorShard` | `internal/shards/requirements_interrogator.go:17` |
| `StartupMode` | `internal/shards/system/base.go:32` |
| `CostGuard` | `internal/shards/system/base.go:42` |
| `UnhandledCase` | `internal/shards/system/base.go:179` |
| `ProposedRule` | `internal/shards/system/base.go:187` |
| `AutopoiesisLoop` | `internal/shards/system/base.go:196` |
| `BaseSystemShard` | `internal/shards/system/base.go:263` |
| `CampaignRunnerConfig` | `internal/shards/system/campaign_runner.go:30` |
| `CampaignRunnerShard` | `internal/shards/system/campaign_runner.go:42` |
| `ConstitutionConfig` | `internal/shards/system/constitution.go:33` |
| `SecurityViolation` | `internal/shards/system/constitution.go:73` |
| `AppealRequest` | `internal/shards/system/constitution.go:84` |
| `AppealDecision` | `internal/shards/system/constitution.go:95` |
| `ConstitutionGateShard` | `internal/shards/system/constitution.go:107` |
| `Strategy` | `internal/shards/system/executive.go:29` |

## Exported functions/methods (sampled, up to 30)

| Symbol | Location |
|--------|----------|
| `NewConsultationManager` | `internal/shards/consultation.go:82` |
| `RequestConsultation` | `internal/shards/consultation.go:94` |
| `RequestBatchConsultation` | `internal/shards/consultation.go:138` |
| `GetStrategicAdvisorsFor` | `internal/shards/consultation.go:177` |
| `ShouldConsultBeforeExecution` | `internal/shards/consultation.go:191` |
| `FormatConsultationAdvice` | `internal/shards/consultation.go:381` |
| `GetSpecialistClassification` | `internal/shards/matching.go:286` |
| `CanSpecialistExecute` | `internal/shards/matching.go:294` |
| `IsExecutorSpecialist` | `internal/shards/matching.go:303` |
| `IsStrategicAdvisor` | `internal/shards/matching.go:312` |
| `ShouldSpecialistExecuteTask` | `internal/shards/matching.go:322` |
| `GetExecutionMode` | `internal/shards/matching.go:423` |
| `MatchSpecialistsForTask` | `internal/shards/matching.go:434` |
| `ShouldIncludeGenericShard` | `internal/shards/matching.go:638` |
| `GetAllPatterns` | `internal/shards/matching.go:647` |
| `GetAssessmentLevel` | `internal/shards/observer_manager.go:67` |
| `NewBackgroundObserverManager` | `internal/shards/observer_manager.go:142` |
| `Start` | `internal/shards/observer_manager.go:157` |
| `Stop` | `internal/shards/observer_manager.go:178` |
| `RegisterObserver` | `internal/shards/observer_manager.go:189` |
| `UnregisterObserver` | `internal/shards/observer_manager.go:216` |
| `GetActiveObservers` | `internal/shards/observer_manager.go:223` |
| `SendEvent` | `internal/shards/observer_manager.go:237` |
| `AddCallback` | `internal/shards/observer_manager.go:256` |
| `SetNorthstarHandler` | `internal/shards/observer_manager.go:264` |
| `GetRecentAssessments` | `internal/shards/observer_manager.go:271` |
| `GetLastAssessment` | `internal/shards/observer_manager.go:287` |
| `FormatAssessment` | `internal/shards/observer_manager.go:515` |
| `DefaultShardPredicateManifests` | `internal/shards/registration.go:45` |
| `Save` | `internal/shards/registration.go:103` |

## Mangle surface

| Artifact | Count |
|----------|------:|
| Package-local `.mg` | 1 |

| Path | Lines |
|------|------:|
| `internal/shards/system/debug_program_ERROR.mg` | 16308 |

Global kernel schemas/policy (when this package participates):

- `internal/core/defaults/schemas.mg`
- `internal/core/defaults/policy/`

## Fact-flow placement

```
user input → perception → user_intent → kernel(core/mangle) → next_action
  → VirtualStore / shards / tools → articulation
```

This package: **Domain and system shard implementations + registration**
