# shards — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/shards/` (18 non-test .go, 24 tests, 1 .mg)**


## 1. Purpose

Domain/system shard implementations and registration

## 2. Source paths

| Path | Role |
|------|------|
| `internal/shards/` | Primary implementation |
| `Docs/architecture/shards/` | This corpus |

## 3. Implementation Status

> Status reflects **living code**, not pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
| Source package tree | Implemented | **90%** |
| Exported types (sampled) | Implemented | **80%** |
| Tests | Implemented | **90%** |
| Mangle local sources | Implemented | **85%** |
| Docs corpus (this) | Implemented | **100%** |

**Overall (heuristic): 90% complete as living package**

## 4. Public surface (inventory-driven)

### Largest implementation files

| Path | Lines |
|------|------:|
| `internal/shards/system/planner.go` | 1108 | source |
| `internal/shards/system/mangle_repair.go` | 1061 | source |
| `internal/shards/system/constitution.go` | 1038 | source |
| `internal/shards/system/router.go` | 1030 | source |
| `internal/shards/system/perception.go` | 953 | source |
| `internal/shards/system/base.go` | 779 | source |
| `internal/shards/system/world_model.go` | 748 | source |
| `internal/shards/system/executive.go` | 660 | source |
| `internal/shards/matching.go` | 649 | source |
| `internal/shards/system/executive_intent.go` | 563 | source |
| `internal/shards/observer_manager.go` | 542 | source |
| `internal/shards/registration.go` | 534 | source |

### Sampled types

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

## 5. Integration points (codeNERD spine)

| Surface | Relevance |
|---------|-----------|
| Kernel / facts | Consumer/producer |
| VirtualStore | User if effectful |
| Shards | Owner |
| Prompt JIT | Optional |
| CLI | Invokes via cmd/nerd |
| Config | Reads global config |

## 6. Non-goals for this corpus revision

- Full behavioral prose rewrite of every function
- Filling Docs/Spec 18-file product templates (use spec-doc-sprint)
- Implementing missing runtime features (use corpus-build implementation mode)
