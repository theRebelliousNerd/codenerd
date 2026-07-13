# 08 — Wiring and Integration: Context

> Last verified against codebase: 2026-07-13  
> Status: Living wiring journal — evidence-based

## 1. Boot wiring (interactive chat)

**File:** `cmd/nerd/chat/session_boot.go` (approx lines 910–939)

Steps:

1. Read `appCfg.GetContextWindowConfig()`.  
2. `ctxcompress.NewCompressorWithParams(kernel, localDB, llmClient, maxTokens, reserve %, window, thresholds…)`.  
3. `compressor.LoadPrioritiesFromCorpus(kernel.GetPredicateCorpus())` (warn on failure).  
4. Open `.nerd/context_feedback.db` via `NewContextFeedbackStore`.  
5. `compressor.SetFeedbackStore(feedbackStore)`.  
6. Attach compressor + feedbackStore to chat `Model` fields.

**Also:** `cmd/nerd/chat/session_shared_boot.go` mirrors the same construction for shared boot.

## 2. Turn processing wiring

**File:** `cmd/nerd/chat/process.go`

| Site | Behavior |
|------|----------|
| Perception history (~168) | If `IsCompressionActive()`, use recent window only |
| Articulation context (~964) | If active: `GetContextString` + recent turns; else full history |
| After assistant response (~1129–1143) | Build `ctxcompress.Turn`, `go ProcessTurn` with 2m timeout |

**Important:** ProcessTurn is **async**. Next-turn build may not see the just-written turn if racing; eventual consistency.

## 3. Session context injection

**File:** `cmd/nerd/chat/model_session_context.go` (~51–55)

```
compressor.GetContextString → sessionCtx.CompressedHistory
```

Parallel path to process.go’s articulation CompressedCtx.

## 4. Persistence

**File:** `cmd/nerd/chat/session_persistence.go`

- `compressor.GetState()`  
- `MarshalCompressedState` → store  

Load path should call `LoadState` + `RefreshBudget` (audit consumers when changing schema).

## 5. UI

**File:** `cmd/nerd/chat/view.go`

- `GetBudgetUsage()` for token gauge display.

## 6. Model type fields

**File:** `cmd/nerd/chat/model_types.go`

- `compressor *ctxcompress.Compressor`  
- `feedbackStore *ctxcompress.ContextFeedbackStore`  
- Boot result structs also carry Compressor / FeedbackStore.

## 7. CLI test entry

**File:** `cmd/nerd/cmd_test_context.go`

Imports package for context stress / harness-style CLI testing.

## 8. Testing harness

**Package:** `internal/testing/context_harness/`

Wraps real compressor (`real_engine.go`) and mocks for integration tests without full chat UI.

## 9. Kernel policy wiring (Mangle)

Not registered by context package. Loaded with core defaults:

| Artifact | Path |
|----------|------|
| Schemas | `internal/core/defaults/schemas_context.mg` |
| Rules | `internal/core/defaults/policy/context_compilation.mg` |

Consumed when `BuildContext` does `kernel.Query("should_include_context")` and when compress asserts `turn_age_category`.

## 10. Prompt / JIT soft wiring

`GetActivationScores` / `GetHighActivationFactKeys` exist for JIT boosts.  
`internal/prompt/context.go` documents population by compression system.  
**Wiring audit required** before claiming all JIT paths live-call these methods every turn.

## 11. Memory ops bridge

From control packet via `processMemoryOperation`:

| Op | Effect |
|----|--------|
| `promote_to_long_term` | `store.StoreFact` preference |
| `forget` | `kernel.Retract` |
| `store_vector` | `store.StoreVector` |

## 12. Wiring checklist (for reviewers)

- [ ] Compressor constructed with workspace-config budgets  
- [ ] Corpus priorities loaded  
- [ ] Feedback store optional-fail is logged  
- [ ] ProcessTurn called after control packet available  
- [ ] IsCompressionActive branches for both perception and articulation  
- [ ] Session persistence save/load pair  
- [ ] UI budget not panicking on nil compressor  
