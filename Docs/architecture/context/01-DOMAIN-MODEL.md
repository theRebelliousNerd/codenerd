# context — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/context/` (complete internal coverage)
> **Implementation: `internal/context/` — 9 non-test .go, 11 tests, 1 .mg**


## Package

`internal/context/`

## Exported types (sampled, up to 40)

| Type | Location |
|------|----------|
| `ActivationEngine` | `internal/context/activation.go:31` |
| `CampaignActivationContext` | `internal/context/activation.go:78` |
| `IssueActivationContext` | `internal/context/activation.go:97` |
| `BackReferenceActivationContext` | `internal/context/activation.go:141` |
| `Compressor` | `internal/context/compressor.go:28` |
| `ContextFeedbackStore` | `internal/context/feedback_store.go:25` |
| `StoredFeedback` | `internal/context/feedback_store.go:40` |
| `PredicateFeedback` | `internal/context/feedback_store.go:51` |
| `FactSerializer` | `internal/context/serializer.go:19` |
| `ContextBlockBuilder` | `internal/context/serializer.go:506` |
| `TokenCounter` | `internal/context/tokens.go:19` |
| `TokenBudget` | `internal/context/tokens.go:147` |
| `CompressorConfig` | `internal/context/types.go:19` |
| `CompressedContext` | `internal/context/types.go:155` |
| `CompressedTurn` | `internal/context/types.go:179` |
| `TokenUsage` | `internal/context/types.go:201` |
| `ScoredFact` | `internal/context/types.go:219` |
| `ActivationState` | `internal/context/types.go:236` |
| `Turn` | `internal/context/types.go:259` |
| `TurnResult` | `internal/context/types.go:276` |
| `HistorySegment` | `internal/context/types.go:295` |
| `RollingSummary` | `internal/context/types.go:316` |
| `CompressedState` | `internal/context/types.go:340` |

## Exported functions/methods (sampled, up to 30)

| Symbol | Location |
|--------|----------|
| `NewActivationEngine` | `internal/context/activation.go:169` |
| `SetCampaignContext` | `internal/context/activation.go:183` |
| `ClearCampaignContext` | `internal/context/activation.go:190` |
| `SetIssueContext` | `internal/context/activation.go:198` |
| `ClearIssueContext` | `internal/context/activation.go:205` |
| `SetBackReferenceContext` | `internal/context/activation.go:213` |
| `ClearBackReferenceContext` | `internal/context/activation.go:220` |
| `SetCorpusPriorities` | `internal/context/activation.go:229` |
| `LoadPrioritiesFromCorpus` | `internal/context/activation.go:237` |
| `SetFeedbackStore` | `internal/context/activation.go:254` |
| `ScoreFacts` | `internal/context/activation.go:266` |
| `FilterByThreshold` | `internal/context/activation.go:389` |
| `SelectWithinBudget` | `internal/context/activation.go:411` |
| `UpdateFocusedPaths` | `internal/context/activation.go:435` |
| `RecordFactTimestamp` | `internal/context/activation.go:459` |
| `AddDependency` | `internal/context/activation.go:468` |
| `ApplyIntentActivation` | `internal/context/activation.go:517` |
| `GetHighActivationFacts` | `internal/context/activation.go:532` |
| `SpreadFromSeeds` | `internal/context/activation.go:548` |
| `GetState` | `internal/context/activation.go:612` |
| `SetState` | `internal/context/activation.go:619` |
| `ClearState` | `internal/context/activation.go:626` |
| `MarkNewFacts` | `internal/context/activation.go:640` |
| `DecayRecency` | `internal/context/activation.go:654` |
| `NewSession` | `internal/context/activation.go:678` |
| `GetSessionStats` | `internal/context/activation.go:687` |
| `Total` | `internal/context/activation_scoring.go:27` |
| `ScoreFactsWithKernelOverride` | `internal/context/activation_scoring.go:591` |
| `NewCompressor` | `internal/context/compressor.go:57` |
| `SetSessionID` | `internal/context/compressor.go:531` |

## Mangle surface

| Artifact | Count |
|----------|------:|
| Package-local `.mg` | 1 |

| Path | Lines |
|------|------:|
| `internal/context/debug_program_ERROR.mg` | 16308 |

Global kernel schemas/policy (when this package participates):

- `internal/core/defaults/schemas.mg`
- `internal/core/defaults/policy/`

## Fact-flow placement

```
user input → perception → user_intent → kernel(core/mangle) → next_action
  → VirtualStore / shards / tools → articulation
```

This package: **Context activation, scoring, and window management**
