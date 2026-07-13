# 02 — Current State: Context (`internal/context`)

> Last verified against codebase: 2026-07-13  
> Status: Precise inventory — code-grounded

## 1. Package stats

| Kind | Count | Notes |
|------|------:|-------|
| Non-test `.go` | **9** | See table below |
| Test `.go` | **11** | Including race + mocks |
| Package `.mg` | **0** design sources | `debug_program_ERROR.mg` is a crash dump |
| Package `README.md` | **1** | Overview; defaults lag `types.go` slightly |

Approximate non-test lines: **~4,700**.

## 2. File inventory (non-test)

| File | Approx lines | Role |
|------|-------------:|------|
| `compressor.go` | 749 | Compressor type, constructors, `BuildContext`, activation context refresh from kernel |
| `activation.go` | 701 | `ActivationEngine`, graph, score orchestration, spread, session stats |
| `activation_scoring.go` | 635 | Nine scoring functions + `ScoreFactsWithKernelOverride` |
| `serializer.go` | 553 | Fact/context serialization, control-packet atom parse, JSON state |
| `compressor_metrics.go` | 493 | Metrics, Get/Load state, activation score export, C1/C3 helpers |
| `compressor_turns.go` | 439 | `ProcessTurn`, compress, summaries, prune |
| `feedback_store.go` | 424 | SQLite usefulness learning |
| `types.go` | 360 | Config + domain types (`CompressedContext`, `ScoredFact`, …) |
| `tokens.go` | 347 | `TokenCounter`, `TokenBudget`, `ErrContextWindowExceeded` |

## 3. File inventory (tests)

| File | Role |
|------|------|
| `activation_test.go` | Core activation unit suite |
| `activation_race_test.go` | Concurrent map race |
| `activation_setters_test.go` | Context setter coverage |
| `compressor_test.go` | Turn/compress/build/persist |
| `compressor_accessors_test.go` | Params, metrics, trim |
| `budget_helpers_test.go` | Budget allocate + hard enforcement |
| `token_counter_extra_test.go` | Count helpers + `NewConfigWithBudget` |
| `serializer_test.go` | Serialization + corpus order |
| `feedback_store_test.go` | Persist + locking |
| `feedback_store_scoring_test.go` | Usefulness scoring |
| `mocks_test.go` | Shared test fakes |

## 4. Hotspots

| Hotspot | Why |
|---------|-----|
| `BuildContext` | Dual path kernel/Go; every LLM injection depends on it |
| `ProcessTurn` | Side effects: assert, store, compress, analytics |
| `computeRelevanceScore` | Large verb→predicate boost map; maintenance surface |
| `refreshActivationContextsLocked` | Large QueryAll fan-out; campaign/issue/back-ref coupling |
| `computeIssueScore` | Cap + clamp critical for safety of window |
| `SelectWithinBudget` | Gate that must always filter first |

## 5. External Mangle surface (not in package)

| Path | Role |
|------|------|
| `internal/core/defaults/schemas_context.mg` | Declarations for inclusion + masking |
| `internal/core/defaults/policy/context_compilation.mg` | C1/C3/C4 derivation rules |

## 6. Runtime artifacts (workspace)

| Path | Role |
|------|------|
| `.nerd/context_feedback.db` | Feedback store (boot path) |
| Store tables via `LocalStore` | Compressed state + activation logs |

## 7. Status summary

| Area | State |
|------|-------|
| Activation scoring | Mature, tested, mutexed |
| Compression loop | Mature; simple summary path active |
| Kernel co-derivation | Present with fallback |
| Feedback learning | Present; needs samples to affect scores |
| Docs (package README) | Partially stale vs code |
| Architecture corpus | This rebuild (2026-07-13) |
