# 06 — Public API and Types: Context

> Last verified against codebase: 2026-07-13  
> Package: `codenerd/internal/context`  
> Status: Living export reference (not exhaustive godoc)

## 1. Configuration

### `CompressorConfig` — `types.go`

| Field | Meaning |
|-------|---------|
| `TotalBudget` | Total token budget |
| `CoreReserve` / `AtomReserve` / `HistoryReserve` / `WorkingReserve` | Category budgets |
| `RecentTurnWindow` | Recent full-atom turns |
| `CompressionThreshold` | Utilization trigger (0–1) |
| `TargetCompressionRatio` | Soft target for segment size |
| `ActivationThreshold` | Min score to include |
| `PredicatePriorities` | **Deprecated** fallback map |

Constructors:

- `DefaultConfig() CompressorConfig`  
- `NewConfigWithBudget(totalBudget int) CompressorConfig`

### Related config outside package

`config.ContextWindowConfig` mapped by `NewCompressorWithConfig`.

## 2. Compressor

### Construction

```go
NewCompressor(kernel, localStore, llmClient) *Compressor
NewCompressorWithConfig(kernel, localStore, llmClient, cfg config.ContextWindowConfig) *Compressor
NewCompressorWithParams(kernel, localStore, llmClient,
    maxTokens, corePct, atomPct, historyPct, workingPct,
    recentWindow int,
    compressionThreshold, targetRatio, activationThreshold float64) *Compressor
```

### Lifecycle / wiring

| Method | Role |
|--------|------|
| `SetSessionID` / `GetSessionID` | Persistence identity |
| `SetFeedbackStore` | Wire learning into activation |
| `LoadPrioritiesFromCorpus` | Priority SSOT |
| `Reset` | Clear state, new session id |
| `GetState` / `LoadState` | Snapshot / restore |
| `RefreshBudget` | Recalc after load |

### Turn + build

| Method | Role |
|--------|------|
| `ProcessTurn(ctx, Turn) (*TurnResult, error)` | Main compression loop |
| `BuildContext(ctx) (*CompressedContext, error)` | Build structured context |
| `GetContextString(ctx) (string, error)` | Serialize for LLM |
| `IsCompressionActive() bool` | Chat path switch |
| `GetRecentTurnWindow() int` | Window size for chat |

### Metrics / scores

| Method | Role |
|--------|------|
| `GetMetrics() map[string]any` | Ratio, segments, turns |
| `GetCompressionRatio() float64` | Original/compressed |
| `GetBudgetUtilization() float64` | 0–1 for UI |
| `GetBudgetUsage() (used, total int)` | Gauge |
| `GetActivationScores() map[string]float64` | Normalized 0–1 for JIT |
| `GetHighActivationFactKeys(threshold) []string` | Hot keys |

## 3. ActivationEngine

```go
NewActivationEngine(config CompressorConfig) *ActivationEngine
```

| Method | Role |
|--------|------|
| `ScoreFacts` / `ScoreFactsWithKernelOverride` | Rank facts |
| `FilterByThreshold` / `SelectWithinBudget` | Gate + pack |
| `ApplyIntentActivation` / `GetHighActivationFacts` | High-level select |
| `SpreadFromSeeds` | Multi-hop energy |
| `Set/ClearCampaignContext` | Campaign boosts |
| `Set/ClearIssueContext` | Issue boosts |
| `Set/ClearBackReferenceContext` | Follow-up boosts |
| `SetCorpusPriorities` / `LoadPrioritiesFromCorpus` | Priorities |
| `SetFeedbackStore` | Learning |
| `UpdateFocusedPaths` / `RecordFactTimestamp` / `AddDependency` | Graph inputs |
| `MarkNewFacts` / `DecayRecency` / `NewSession` | Lifecycle |
| `GetState` / `SetState` / `ClearState` | ActivationState |
| `GetSessionStats() map[string]any` | Debug |

### Context structs

- `CampaignActivationContext`  
- `IssueActivationContext`  
- `BackReferenceActivationContext`  

Defined in `activation.go`.

## 4. Domain DTOs — `types.go`

| Type | Role |
|------|------|
| `CompressedContext` | Built context for one LLM call |
| `CompressedTurn` | Surface-free turn |
| `TokenUsage` | Component token accounting |
| `ScoredFact` | Fact + component scores |
| `ActivationState` | Engine snapshot |
| `Turn` / `TurnResult` | ProcessTurn I/O |
| `HistorySegment` / `RollingSummary` | Compression history |
| `CompressedState` | Persistence payload |

## 5. Serialization — `serializer.go`

```go
NewFactSerializer() *FactSerializer
NewContextBlockBuilder() *ContextBlockBuilder
```

| Symbol | Role |
|--------|------|
| `SerializeFacts` / `SerializeScoredFacts` | Atom text |
| `SerializeCompressedTurn` / `SerializeCompressedContext` | Blocks |
| `LoadSerializationOrderFromCorpus` / `SetCorpusOrder` | Order SSOT |
| `ExtractAtomsFromControlPacket` | Piggyback → facts |
| `ParseMangleAtom` | String → Fact |
| `MarshalCompressedState` / `UnmarshalCompressedState` | JSON |
| `ContextBlockBuilder.Build` | Compose CompressedContext |

## 6. Tokens — `tokens.go`

```go
NewTokenCounter() *TokenCounter
NewTokenBudget(config CompressorConfig) *TokenBudget
var ErrContextWindowExceeded error
```

| API | Role |
|-----|------|
| `CountString` / `CountFact` / `CountTurn(s)` / `CountCompressedContext` | Estimates |
| `Allocate` / `AllocateWithError` / `Release` | Category budgets |
| `CheckTotalBudget` / `MustFitWithinBudget` | Hard limits |
| `Utilization` / `ShouldCompress` / `GetUsage` / `Reset` | Tracking |
| `EstimateCompressionRatio` | Helper |

## 7. Feedback store — `feedback_store.go`

```go
NewContextFeedbackStore(dbPath string) (*ContextFeedbackStore, error)
```

| API | Role |
|-----|------|
| `StoreFeedback(...)` | Persist turn ratings |
| `GetPredicateUsefulness` / `ForIntent` | Activation inputs |
| `GetPredicateFeedback` | Detail stats |
| `GetTopHelpfulPredicates` / `GetTopNoisePredicates` | Analytics |
| `GetOverallStats` | Aggregate |
| `Close` | Shutdown |

Types: `StoredFeedback`, `PredicateFeedback`.

## 8. Errors

| Error | When |
|-------|------|
| `ErrContextWindowExceeded` | Total or category hard limit |

Wrapped with `fmt.Errorf("%w: ...")` in budget methods.

## 9. Import pattern

Chat code commonly aliases:

```go
ctxcompress "codenerd/internal/context"
```

Session package may avoid direct import via interfaces (`internal/session/subagent.go` comment).
