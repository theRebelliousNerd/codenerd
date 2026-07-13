# northstar — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/northstar/` (complete internal coverage)
> **Implementation: `internal/northstar/` — 4 non-test .go, 6 tests, 0 .mg**


## 1. Purpose

North-star goal tracking and alignment helpers

## 2. Source paths (1:1)

| Path | Role |
|------|------|
| `internal/northstar/` | Primary implementation |
| `Docs/architecture/northstar/` | This full corpus |

## 3. Implementation Status

> Living code status — **not** pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
| Source package tree | Implemented | **90%** |
| Exported types (sampled) | Implemented | **80%** |
| Tests | Implemented | **90%** |
| Mangle local sources | N/A or global-only | **n/a** |
| Full architecture corpus | Implemented | **100%** |

**Overall (heuristic): 90%** as living package (4 src / 6 tests)

## 4. Public surface inventory

### Largest files

| Path | Lines |
|------|------:|
| `internal/northstar/store.go` | 732 | source |
| `internal/northstar/guardian.go` | 677 | source |
| `internal/northstar/observer.go` | 482 | source |
| `internal/northstar/types.go` | 305 | source |

### Types (sampled)

| Type | Location |
|------|----------|
| `KernelClient` | `internal/northstar/guardian.go:17` |
| `Guardian` | `internal/northstar/guardian.go:24` |
| `LLMClient` | `internal/northstar/guardian.go:37` |
| `CampaignObserver` | `internal/northstar/observer.go:16` |
| `TaskObserver` | `internal/northstar/observer.go:225` |
| `BackgroundEventHandler` | `internal/northstar/observer.go:289` |
| `ObserverAssessment` | `internal/northstar/observer.go:303` |
| `ObserverEvent` | `internal/northstar/observer.go:316` |
| `Store` | `internal/northstar/store.go:17` |
| `Vision` | `internal/northstar/types.go:22` |
| `Persona` | `internal/northstar/types.go:115` |
| `Capability` | `internal/northstar/types.go:122` |
| `Risk` | `internal/northstar/types.go:130` |
| `Requirement` | `internal/northstar/types.go:139` |
| `Observation` | `internal/northstar/types.go:151` |
| `ObservationType` | `internal/northstar/types.go:164` |
| `AlignmentCheck` | `internal/northstar/types.go:181` |
| `AlignmentTrigger` | `internal/northstar/types.go:195` |
| `AlignmentResult` | `internal/northstar/types.go:208` |
| `DriftEvent` | `internal/northstar/types.go:223` |
| `DriftSeverity` | `internal/northstar/types.go:237` |
| `GuardianConfig` | `internal/northstar/types.go:251` |
| `GuardianState` | `internal/northstar/types.go:298` |

### Functions (sampled)

| Symbol | Location |
|--------|----------|
| `NewGuardian` | `internal/northstar/guardian.go:42` |
| `SetLLMClient` | `internal/northstar/guardian.go:51` |
| `SetParentKernel` | `internal/northstar/guardian.go:58` |
| `Initialize` | `internal/northstar/guardian.go:65` |
| `HasVision` | `internal/northstar/guardian.go:134` |
| `GetVision` | `internal/northstar/guardian.go:141` |
| `GetState` | `internal/northstar/guardian.go:148` |
| `UpdateVision` | `internal/northstar/guardian.go:155` |
| `CheckAlignment` | `internal/northstar/guardian.go:185` |
| `ObserveTaskCompletion` | `internal/northstar/guardian.go:396` |
| `ObserveFileChange` | `internal/northstar/guardian.go:413` |
| `ObserveDecision` | `internal/northstar/guardian.go:428` |
| `ShouldCheckNow` | `internal/northstar/guardian.go:490` |
| `OnTaskComplete` | `internal/northstar/guardian.go:534` |
| `NewCampaignObserver` | `internal/northstar/observer.go:32` |
| `StartCampaign` | `internal/northstar/observer.go:53` |
| `OnPhaseStart` | `internal/northstar/observer.go:77` |
| `OnPhaseComplete` | `internal/northstar/observer.go:112` |
| `OnTaskComplete` | `internal/northstar/observer.go:133` |
| `EndCampaign` | `internal/northstar/observer.go:178` |
| `GetPhaseCheck` | `internal/northstar/observer.go:204` |
| `GetAllPhaseChecks` | `internal/northstar/observer.go:211` |
| `NewTaskObserver` | `internal/northstar/observer.go:232` |
| `OnTaskStart` | `internal/northstar/observer.go:240` |
| `OnTaskComplete` | `internal/northstar/observer.go:246` |
| `OnError` | `internal/northstar/observer.go:267` |
| `NewBackgroundEventHandler` | `internal/northstar/observer.go:295` |
| `HandleEvent` | `internal/northstar/observer.go:325` |
| `NewStore` | `internal/northstar/store.go:24` |
| `Close` | `internal/northstar/store.go:59` |

## 5. Integration relevance

| Surface | Relevance |
|---------|-----------|
| Kernel | Related |
| VirtualStore | Consumer if effectful |
| Shards | Related |
| Prompt JIT | Optional |
| CLI | Related via `cmd/nerd` |
| Config | Reader |

## 6. Non-goals of this corpus revision

- Full prose rewrite of every function body
- Docs/Spec 18-file product templates (`spec-doc-sprint`)
- Implementing missing features (corpus-build implementation mode)
