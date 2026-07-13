# shards — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/shards/` (18 non-test .go, 24 tests, 1 .mg)**


## Source package

`internal/shards/`

## Exported / primary types (sampled)

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

## Planned vs actual Mangle surface

| Artifact | Count | Notes |
|----------|-------|-------|
| `.mg` files under package | 1 | See inventory |
| Core schemas (global) | shared | `internal/core/defaults/schemas.mg` when kernel-touching |
| Policy modules (global) | shared | `internal/core/defaults/policy/` |

### Package-local Mangle inventory (top)

| Path | Lines |
|------|-------|
| `internal/shards/system/debug_program_ERROR.mg` | 16308 |

## Fact-flow placement

```
user input → perception → user_intent → kernel(core/mangle) → next_action
  → VirtualStore / shards / tools → articulation
```

This package's position: **Domain/system shard implementations and registration**

## Data & control concepts

- Primary language surface: Go under `internal/shards/`
- Logic surface: Mangle where listed above or via kernel defaults
- External effects: VirtualStore / tools / CLI depending on package
