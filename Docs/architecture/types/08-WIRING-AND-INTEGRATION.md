# 08 — Wiring and Integration: `internal/types`

> Last verified: **2026-07-13**  
> `types` is not registered at boot like a shard. Wiring means **who satisfies and injects** its contracts.

## 1. Boot / composition root

Interactive and Cobra boot paths (`cmd/nerd/chat/session_boot.go`, `session_shared_boot.go`, system Cortex boot) construct:

| Concrete | Satisfies / carries |
|----------|---------------------|
| `*core.RealKernel` / Cortex wrappers | `types.Kernel`, often `KernelTransactor` + `KernelInterface` methods |
| Perception LLM clients | `types.LLMClient` (+ optional interfaces) |
| `*core.VirtualStore` | `types.VirtualStore` methods |
| Shard manager + factories | Produce `types.ShardAgent` |
| Session context builders | Fill `types.SessionContext` |

Adapters:

- `sessionKernelAdapter` (`cmd/nerd/chat/session_adapters.go`) — adapts core kernel to `types.Kernel` for session package
- `campaignKernelAdapter` (`cmd/nerd/cmd_campaign.go`) — same pattern for campaign CLI
- `core.AutopoiesisBridge` — adapts RealKernel to `types.KernelInterface`

## 2. Fact-flow wiring

```
user text
  → perception (LLMClient) → StructuredIntent
  → assert user_intent facts (Fact.ToAtom via kernel)
  → kernel derives next_action (Mangle policy)
  → VirtualStore routes action
  → ShardAgent.Execute with SessionContext
  → articulation reads facts + SessionContext → response
```

Every dashed boundary above is typed by `internal/types` even when implementers live elsewhere.

## 3. Kernel wiring details

### Full `types.Kernel`

- Compile-time checks: `var _ types.KernelTransactor = (*CortexKernel)(nil)` in `internal/core/cortex_kernel.go`
- Alias: `type Kernel = types.Kernel` in `kernel_types.go`
- Batch retracts used by world scanner paths (`RetractExactFactsBatch`, `RemoveFactsByPredicateSet`)

### Narrow `types.KernelInterface`

- `AutopoiesisBridge` + `var _ types.KernelInterface = (*AutopoiesisBridge)(nil)` in `kernel_utils.go`
- Autopoiesis mocks implement `KernelInterface` for unit tests

### Transactions

Call sites (examples):

- `internal/campaign/campaign_fact_sync.go` — `NewKernelTx`
- `internal/campaign/context_pager.go`
- `internal/core/tdd_loop.go`
- `internal/core/cortex_kernel_transaction_test.go`

## 4. Shard wiring

- Registration uses `ShardConfig` / factories (`internal/init/agents_registration.go`, `internal/shards/registration.go`)
- `BaseShardAgent.SetParentKernel(k types.Kernel)` injects kernel without importing concrete only at base layer
- Policy string on `ShardConfig` → `Kernel.AppendPolicy` before execution (power-user path)

## 5. LLM wiring

- Shared client instance may receive per-shard hints via context values `CtxKeyModelCapability` / `CtxKeyModelName`
- Scheduler / tool loops type-assert `ToolResultsProvider`, `PiggybackToolProvider`, etc.
- Gemini piggyback path depends on `ShouldUsePiggybackTools()` rather than native function calling when grounding is on

## 6. SessionContext wiring

| Producer | Consumer |
|----------|----------|
| Chat `model_session_context.go`, process pipeline | Shards via `SetSessionContext` |
| Campaign context pager | Kernel facts + context |
| Prompt assembler | `PromptContext.SessionCtx` |

Constitutional fields (`AllowedActions`, …) are **data for prompts/guards** — not auto-enforced in `types`.

## 7. Persistence wiring

- `store/fact_codec.go` encodes/decodes `types.Fact`
- `persist/factsnap` snapshots `[]types.Fact` for restore

## 8. What is *not* wired

- No CLI command dedicated solely to `types`
- No Mangle `Decl` files owned by this package
- No automatic registration of optional LLM interfaces (assertion at use site)

## 9. Wiring audit checklist (before claiming “unused”)

If a type or interface in `types` looks unused:

1. Grep `types.Symbol` across `internal/` and `cmd/`
2. Grep aliases (`core.Kernel`, `core.Fact`, `perception.LLMClient`)
3. Check e2e mocks
4. Check `var _ types.X =` compliance assertions

Deleting an “unused” interface often breaks a dormant or test-only implementer — prefer wiring audit (repo rule).
