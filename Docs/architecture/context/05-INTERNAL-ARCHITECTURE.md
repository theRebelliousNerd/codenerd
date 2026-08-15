# 05 — Internal Architecture: Context

> Last verified against codebase: 2026-08-15  
> Source: `internal/context/`  
> Status: Living Reference Document

## 1. Component map

```
                    ┌──────────────────────┐
                    │   Compressor         │
                    │  (orchestrator + mu) │
                    └──────────┬───────────┘
           ┌───────────────────┼───────────────────┐
           ▼                   ▼                   ▼
  ActivationEngine      TokenBudget         FactSerializer
  (+ scoring.go)        TokenCounter        ContextBlockBuilder
           │                   │                   │
           ▼                   │                   ▼
  ContextFeedbackStore         │            CompressedContext
  (optional SQLite)            │            string for LLM
                               ▼
                        RollingSummary
                        recentTurns[]
```

| Component | Files | Responsibility |
|-----------|-------|----------------|
| Compressor | `compressor.go`, `_turns.go`, `_metrics.go` | Lifecycle, build, compress, persist |
| ActivationEngine | `activation.go`, `activation_scoring.go` | Ranking and selection |
| TokenBudget / Counter | `tokens.go` | Estimates + limits |
| FactSerializer / Builder | `serializer.go` | LLM-facing text |
| ContextFeedbackStore | `feedback_store.go` | Learned usefulness |
| Types | `types.go` | Config + DTOs |

## 2. Key data structures

### Compressor state

| Field | Meaning |
|-------|---------|
| `kernel` | Source of facts / assert target |
| `store` | Persistence + memory ops |
| `llmClient` | Optional LLM summary (mostly unused by compress path) |
| `activation` | Scoring engine |
| `budget` | Current usage |
| `recentTurns` | Sliding atom turns |
| `rollingSummary` | Compressed segments + text |
| `turnNumber` / `sessionID` | Bookkeeping |
| `totalOriginal/CompressedTokens` | Ratio metrics |

### ActivationEngine state

| Field | Meaning |
|-------|---------|
| `factTimestamps` | Recency |
| `dependencies` / `reverseDependencies` | Explicit + graph edges |
| `symbolGraph` | Caller→callee from facts |
| `campaignContext` / `issueContext` / `backReferenceContext` | Boost domains |
| `sessionFacts` | Session-local membership |
| `corpusPriorities` | SSOT priorities |
| `feedbackStore` | Optional learning |
| `state` | ActiveIntent, focused paths/symbols, hot facts |

## 3. State machines

### Compression readiness

```
[Empty] --ProcessTurn*--> [Accumulating]
   Accumulating --utilization >= threshold--> [Compressing]
   Compressing --segments written--> [CompressedActive]
   CompressedActive --IsCompressionActive true--> inject GetContextString
   any --Reset--> [Empty]
```

`IsCompressionActive` also true when budget threshold hit **even without** segments (prevents rehydrate dump).

### Fact selection path

```
[Kernel facts]
    │
    ├─(prefer) should_include_context → parse /pN → resolve entity → facts
    │             → SelectWithinBudgetPreFiltered
    │             (empty resolution falls through to the fallback)
    │
    └─(fallback) ScoreFacts → FilterByThreshold → SelectWithinBudget

Which branch ran is recorded in SelectionStats (GetSelectionStats / GetMetrics).
```

### Turn age categories (C3)

| Age (currentTurn − turnNum) | Category |
|-----------------------------|----------|
| ≤ 3 | `/recent` |
| ≤ 8 | `/mid` |
| ≤ 15 | `/old` |
| > 15 | `/ancient` |

`/old` and `/ancient` derive `should_mask_observation(TurnID)`; every categorized
turn derives `should_preserve_reasoning(TurnID)`. `maskedObservationTurns()` reads
both back and masks only their intersection, so drifted rules fail toward keeping
more history rather than less.

## 4. Data flow: one assistant turn

```
Chat process completes articulation
        │
        ▼
ControlPacket + surface response + user input
        │
        ▼
Turn{ Number, Role, UserInput, SurfaceResponse, ControlPacket }
        │
        ▼
ProcessTurn (often async goroutine with timeout)
        │
        ├─ atoms → kernel
        ├─ CompressedTurn (atoms only)
        ├─ budget / compress
        └─ store state
```

## 5. Data flow: next LLM call

```
IsCompressionActive?
  false → chat history as-is
  true  → recentTurns(window) + GetContextString
              BuildContext
              SerializeCompressedContext
```

Also: `model_session_context.go` may inject `CompressedHistory` independently via `GetContextString`.

## 6. Scoring pipeline detail

```
for each fact:
  computeScore:
    base, recency, relevance, dependency,
    campaign, session, issue, feedback, backReference
  Total = sum
sort desc
filter score >= threshold
greedy pack until AtomReserve tokens exhausted
```

Verb→predicate boosts for relevance live only in `computeRelevanceScore` (map keyed by intent verb like `/fix`, `/debug`, …).

## 7. Serialization pipeline

```
[]core.Fact / []ScoredFact
    → group by predicate
    → sort by corpus order or predicateSortOrder fallback
    → optional comments + line truncation
    → Mangle text lines
```

`SerializeCompressedContext` wraps four sections with ASCII headers for the LLM.

## 8. Feedback pipeline

```
StoreFeedback(helpful[], noise[])
    → SQL rows
GetPredicateUsefulness[ForIntent]
    → decay-weighted score if samples >= minSamples
computeFeedbackScore
    → ×20 into activation
```

## 9. Locking model

| Lock | Protects |
|------|----------|
| `Compressor.mu` | recentTurns, rollingSummary, session, budget access patterns in methods |
| `ActivationEngine.mu` | maps, contexts, score mutations |
| `ContextFeedbackStore.mu` | write transactions |
| `ContextFeedbackStore.cacheMu` | usefulness cache |

`RefreshBudget` carefully unlocks before `recalcBudget` to avoid deadlock with kernel/activation.

## 10. Extension points

| Hook | Purpose |
|------|---------|
| `SetCampaignContext` / Clear | Manual campaign boosts |
| `SetIssueContext` / Clear | Issue-driven work |
| `SetBackReferenceContext` / Clear | Follow-ups |
| `SetFeedbackStore` | Wire learning |
| `LoadPrioritiesFromCorpus` | SSOT priorities |
| `SetSessionID` | Align persistence key with chat session |
| `ScoreFactsWithKernelOverride` | Inject kernel map |
| Memory ops in ProcessTurn | Bridge to store/kernel retract |
