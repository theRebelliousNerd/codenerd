# northstar — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/northstar/` (complete internal coverage)
> **Implementation: `internal/northstar/` — 4 non-test .go, 6 tests, 0 .mg**


## Package

`internal/northstar/`

## Exported types (sampled, up to 40)

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

## Exported functions/methods (sampled, up to 30)

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

## Mangle surface

| Artifact | Count |
|----------|------:|
| Package-local `.mg` | 0 |

| Path | Lines |
|------|------:|
| — | 0 |

Global kernel schemas/policy (when this package participates):

- `internal/core/defaults/schemas.mg`
- `internal/core/defaults/policy/`

## Fact-flow placement

```
user input → perception → user_intent → kernel(core/mangle) → next_action
  → VirtualStore / shards / tools → articulation
```

This package: **North-star goal tracking and alignment helpers**
