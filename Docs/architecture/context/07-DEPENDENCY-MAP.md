# 07 — Dependency Map: Context

> Last verified against codebase: 2026-07-13  
> Package: `internal/context`  
> Status: Living Reference Document

## 1. Upstream (what context imports)

| Package | Usage |
|---------|--------|
| `codenerd/internal/core` | `Fact`, `RealKernel`, `PredicateCorpus`, assert/query |
| `codenerd/internal/perception` | `ControlPacket`, `MemoryOperation`, `LLMClient`, system LLM context |
| `codenerd/internal/store` | `LocalStore` for compressed state, activation logs, memory ops |
| `codenerd/internal/config` | `ContextWindowConfig` in `NewCompressorWithConfig` only |
| `codenerd/internal/logging` | `CategoryContext`, timers, Context/ContextDebug helpers |
| `codenerd/internal/sqlpragmas` | Feedback DB pragmas (`ProfileHot`) |
| stdlib | `sync`, `context`, `database/sql`, `encoding/json`, `time`, … |

**Does not import:** `cmd/nerd`, `internal/prompt` (scores are *exported for* prompt), shards, campaign packages (consumes campaign *facts* only).

## 2. Downstream (who imports context)

| Consumer | Evidence |
|----------|----------|
| `cmd/nerd/chat/session_boot.go` | `NewCompressorWithParams`, feedback store |
| `cmd/nerd/chat/session_shared_boot.go` | Shared boot path |
| `cmd/nerd/chat/process.go` | Active compression + `ProcessTurn` |
| `cmd/nerd/chat/model_session_context.go` | `GetContextString` |
| `cmd/nerd/chat/session_persistence.go` | `GetState` / marshal |
| `cmd/nerd/chat/model_types.go` | Holds compressor + feedbackStore |
| `cmd/nerd/chat/view.go` | Budget UI |
| `cmd/nerd/chat/persistence.go` | Turn type |
| `cmd/nerd/cmd_test_context.go` | CLI context testing |
| `internal/testing/context_harness/*` | Real/mock engines |

Indirect / soft:

| Consumer | Notes |
|----------|-------|
| `internal/prompt/context.go` | Comment references `GetActivationScores` |
| `internal/session/subagent.go` | Explicitly avoids import via interface |

## 3. Diagram

```
internal/config ──┐
internal/core ────┤
internal/perception┤──► internal/context ──► cmd/nerd/chat/*
internal/store ───┤                 └──► cmd/nerd/cmd_test_context.go
internal/logging ─┤                 └──► internal/testing/context_harness
internal/sqlpragmas┘

internal/core/defaults/policy/context_compilation.mg  (queried via kernel)
internal/core/defaults/schemas_context.mg             (Decl surface)
```

## 4. Fact-flow position

```
user_intent ──► kernel ──► next_action ──► VirtualStore
                  │
                  │ GetAllFacts / Query
                  ▼
            internal/context
                  │
                  ▼
         articulation / perception history
```

Context is **orthogonal** to action selection: it shapes **what the model sees**, not what the kernel permits.

## 5. Data store edges

| Store API | Called from |
|-----------|-------------|
| `StoreCompressedState` | `ProcessTurn` |
| `LogActivation` | `ProcessTurn` (top hot facts) |
| `StoreFact` | memory op `promote_to_long_term` |
| `StoreVector` | memory op `store_vector` |
| `kernel.Retract` | memory op `forget` |

## 6. Cyclic risk

No import cycle with core/perception/store expected: context sits “above” them. Chat imports context; context does not import chat.
