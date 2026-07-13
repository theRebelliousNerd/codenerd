# context — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/context/` (complete internal coverage)
> **Implementation: `internal/context/` — 9 non-test .go, 11 tests, 1 .mg**


## 1. Purpose

Context activation, scoring, and window management

## 2. Source paths (1:1)

| Path | Role |
|------|------|
| `internal/context/` | Primary implementation |
| `Docs/architecture/context/` | This full corpus |

## 3. Implementation Status

> Living code status — **not** pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
| Source package tree | Implemented | **90%** |
| Exported types (sampled) | Implemented | **80%** |
| Tests | Implemented | **90%** |
| Mangle local sources | Implemented | **85%** |
| Full architecture corpus | Implemented | **100%** |

**Overall (heuristic): 90%** as living package (9 src / 11 tests)

## 4. Public surface inventory

### Largest files

| Path | Lines |
|------|------:|
| `internal/context/compressor.go` | 748 | source |
| `internal/context/activation.go` | 700 | source |
| `internal/context/activation_scoring.go` | 634 | source |
| `internal/context/serializer.go` | 552 | source |
| `internal/context/compressor_metrics.go` | 492 | source |
| `internal/context/compressor_turns.go` | 438 | source |
| `internal/context/feedback_store.go` | 423 | source |
| `internal/context/types.go` | 359 | source |
| `internal/context/tokens.go` | 346 | source |

### Types (sampled)

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

### Functions (sampled)

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
