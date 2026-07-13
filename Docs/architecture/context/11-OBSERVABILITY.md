# 11 — Observability: Context

> Last verified against codebase: 2026-07-13  
> Package: `internal/context`  
> Status: Living Reference Document

## 1. Logging category

Primary: `logging.CategoryContext`

Helpers used throughout package:

- `logging.Context(...)` — info-level operational messages  
- `logging.ContextDebug(...)` — detailed scoring/budget traces  
- `logging.Get(logging.CategoryContext).Warn/Error(...)` — failures  

## 2. Timers

`logging.StartTimer(logging.CategoryContext, name)` sites include:

| Timer name | Where |
|------------|-------|
| `ScoreFacts` | `activation.go` |
| `GetHighActivationFacts` | `activation.go` |
| `SpreadFromSeeds` | `activation.go` |
| `BuildContext` | `compressor.go` |
| `ActivationScoring` | nested in BuildContext |
| `ProcessTurn[N]` | `compressor_turns.go` |
| `Compression` | compress path |
| `RecalcBudget` | budget recompute |
| `GenerateObservationMaskedSummary` | simple summary path |

## 3. Key log events

| Event | Level | Meaning |
|-------|-------|---------|
| Compressor initialized… | Context | Budget/window at construct |
| Building context: N facts | Debug | Build start |
| C1+C4 kernel context… / Go activation fallback… | Debug | Selection path |
| Context built: tokens used… | Context | Build complete |
| Processing turn N | Context | ProcessTurn start |
| COMPRESSION TRIGGERED | Context | Threshold crossed |
| COMPRESSION COMPLETE | Context | Segment written |
| CONTEXT WINDOW EXCEEDED | Error | Hard limit |
| safety-predicate query failed | Warn | Core facts partial |
| QueryAll failed (refresh) | Warn | Campaign/issue context stale |
| Stored context feedback… | Debug | Feedback write |

## 4. Metrics APIs (programmatic)

| Method | Returns |
|--------|---------|
| `GetMetrics` | turn_number, recent_turns, segments, ratios, token totals |
| `GetCompressionRatio` | float |
| `GetBudgetUtilization` | 0–1 |
| `GetBudgetUsage` | (used, total) |
| `ActivationEngine.GetSessionStats` | session_id, fact counts, graph sizes, campaign/issue flags |
| `ContextFeedbackStore.GetOverallStats` | total feedback, avg usefulness |
| `GetTopHelpfulPredicates` / `GetTopNoisePredicates` | analytics |

## 5. Persistence analytics

On `ProcessTurn`, when store non-nil:

- `StoreCompressedState(sessionID, turn, json, ratio)`  
- `LogActivation(factString, score)` for up to 50 hot facts  

Useful for post-hoc long-horizon analysis.

## 6. UI surface

Chat `view.go` reads `GetBudgetUsage` to display window pressure to the operator.

## 7. Debug dump caveat

`internal/context/debug_program_ERROR.mg` appears when the **kernel program** fails elsewhere and dumps sources — not a structured context metric. Treat as crash forensics only.

## 8. Recommended operator filters

When diagnosing context issues, filter logs by CategoryContext and look for:

1. Which selection path (kernel vs Go)  
2. above_threshold counts  
3. COMPRESSION TRIGGERED timing  
4. Budget utilization vs threshold  
5. Core query warnings  
