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
| `GetMetrics` | turn_number, recent_turns, segments, ratios, token totals, masked turns, kernel/Go split |
| `GetCompressionRatio` | float |
| `GetBudgetUtilization` | 0–1 |
| `GetBudgetUsage` | (used, total) |
| `GetSelectionStats` | `SelectionStats`: kernel selections, Go fallbacks, inclusion rate, last reason |
| `GetFeedbackStats(topN)` | `FeedbackStats`: totals + helpful/noise predicate tables |
| `ActivationEngine.GetSessionStats` | session_id, fact counts, graph sizes, campaign/issue flags |
| `ContextFeedbackStore.GetOverallStats` | total feedback, avg usefulness |
| `GetTopHelpfulPredicates` / `GetTopNoisePredicates` | analytics |

### 4.1 Kernel-vs-Go selection (dual-path drift)

`SelectionStats.LastReason` is the diagnostic that matters:

| Reason | Meaning | What to do |
|--------|---------|------------|
| `kernel_selected` | `should_include_context` decided the block | healthy |
| `no_should_include_context_facts` | kernel had no opinion (cold session) | expected early in a session |
| `kernel_query_error` | the query itself failed | check schema load / kernel health |
| `kernel_facts_unresolved` | the kernel named entities no fact in the store mentions | C1/C4 rules and the fact store disagree about identity — investigate |

A `KernelInclusionRate()` that collapses toward 0 in a warm session means the
kernel path has stopped contributing and Go heuristics are silently running the
context window.

## 5. Persistence analytics

On `ProcessTurn`, when store non-nil:

- `StoreCompressedState(sessionID, turn, json, ratio)`  
- `LogActivation(factString, score)` for up to 50 hot facts  

Useful for post-hoc long-horizon analysis.

## 6. UI surface

Chat `view.go` reads `GetBudgetUsage` to display window pressure to the operator.

## 7. Debug dump caveat

`debug_program_ERROR.mg` appears when the **kernel program** fails analysis and dumps its sources — not a structured context metric. Treat as crash forensics only.

It is written by `core.writeFailedProgramDump` to `<cwd>/.nerd/debug/`, never into a package tree; the name is gitignored and `.nerd/` is excluded from world scans (an earlier version landed a 700 KB dump inside the scanned source tree and the scanner ingested it as real source). Older corpus text placing it at `internal/context/debug_program_ERROR.mg` is stale.

## 8. Recommended operator filters

When diagnosing context issues, filter logs by CategoryContext and look for:

1. Which selection path (kernel vs Go)  
2. above_threshold counts  
3. COMPRESSION TRIGGERED timing  
4. Budget utilization vs threshold  
5. Core query warnings  

## 9. Operator workflow: helpful vs noise predicates

Context learning is the third feedback loop: the LLM rates which predicates
helped, and `ContextFeedbackStore` turns that into a −20..+20 activation
adjustment. Inspecting it:

```bash
# Tables of the predicates the system learned to trust and distrust
nerd context-stats

# Wider tables, or machine-readable for diffing across runs
nerd context-stats --top 30
nerd context-stats --json > /tmp/ctx-before.json
```

Reading the output:

- **turns rated / avg usefulness** — whether the loop is receiving signal at all.
  Zero rated turns means feedback is not being written, not that context is perfect.
- **min samples/pred** — the trust floor (default 10). Predicates below it are
  excluded from both tables and have **no** effect on scoring. A near-empty report
  on a young workspace is normal.
- **SCORE** — weighted usefulness in [−1, +1] after 7-day half-life decay, scaled
  ×20 into the activation feedback component.

Acting on it:

1. A predicate parked in the noise table across sessions is a candidate for a
   lower corpus priority (`predicate_corpus.db`), not a code change — the corpus
   is the priority SSOT and activation reads it via `LoadPrioritiesFromCorpus`.
2. A helpful predicate that never reaches the sample floor is usually not being
   selected into context at all; check `GetSelectionStats().LastReason` first —
   if the kernel path is unresolved, no amount of feedback will surface it.
3. The feedback DB is per-workspace (`<workspace>/.nerd/context_feedback.db`);
   deleting it resets learning without touching session history.
