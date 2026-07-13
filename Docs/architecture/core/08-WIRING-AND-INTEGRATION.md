# core — Wiring and Integration

> Last verified: **2026-07-13**  
> How core is constructed and bound into a running nerd process.

## 1. Boot sequence (typical interactive)

Conceptual order (see `cmd/nerd/chat/session_boot.go` and `internal/system` for exact calls):

```
1. Resolve workspace → .nerd/
2. NewRealKernelWithWorkspace(workspace)  OR  CortexKernel + shards
3. loadMangleFiles already ran inside constructor
4. NewVirtualStore(tactileExecutor) / WithConfig(workingDir, allowlists)
5. vs.SetKernel(kernel)  → clears stale Dreamer; lazily attaches to RealKernel or Cortex primary
6. kernel.SetVirtualStore(vs)  → virtual predicates
7. Chat: keep guard through rehydration; disable on first user message
   Command BootCortex: disable while booting an already requested command
8. Register domain shard factories on ShardManager
9. SetLLMClient / APIScheduler / ScheduledLLM wrappers
10. session.Executor or TaskDelegator wired for delegate actions
11. Optional: GlassBox + Tool buses, transaction manager, learning store
12. Perception asserts user_intent → evaluate → next_action → RouteAction
```

## 2. Kernel ↔ VirtualStore mutual binding

```
vs.SetKernel(k)
  - stores Kernel interface
  - clears any Dreamer bound to the prior kernel
  - lazy Dreamer resolves *RealKernel or CortexKernel.GetPrimaryRealKernel
  - rebuilds safe_action classification cache

k.SetVirtualStore(vs)
  - enables query_* / virtual predicate resolution during eval
```

Order matters: attach before routing actions. Missing kernel or missing Dreamer on
a destructive path denies. The boot-guard lifetime is mode-specific: chat is
quiescent through rehydration; command boot represents an already requested action.

The default Cortex configuration routes the full permission envelope to the
policy shard: `pending_action`, `permitted_action`, `permission_check_result`,
`permitted`, `blocked`, `constitution`, `commit_barrier`, and `dangerous_action`.
Splitting these predicates across shards breaks the join and must remain a test
failure.

## 3. Session integration

| Session piece | Core touchpoint |
|---------------|-----------------|
| `Executor.Process` | Asserts intents/facts; may query next_action |
| Tool execution | Prefer VS RouteAction / Exec with tool allowlists |
| `TaskExecutor` | Implements `TaskDelegator` for `handleDelegate*` |
| Spawner | May use types.Kernel + VS without owning constitution |

Package README: orchestration moved toward session; core still supplies executive truth + effects.

## 4. Shard manager plumbing

On `NewVirtualStoreWithConfig`:

```go
shardManager: coreshards.NewShardManager()
// ...
vs.shardManager.SetVirtualStore(vs)
```

Boot then:

1. `SetParentKernel`  
2. `SetLLMClient` / `SetImageLLMClient`  
3. Register factories (from `internal/shards/registration.go` typically)  
4. Optional `SpawnQueue` + `LimitsEnforcer`  
5. `PostSpawnHook` for chat-only DI (glass box, stores)  

Delegate actions in VS call `taskDelegator` when set; otherwise may fall back to manager-related paths depending on handler code.

## 5. CLI wiring surfaces

| CLI area | Core use |
|----------|----------|
| Interactive chat | Real-kernel session boot; guard released at first user turn |
| Command-oriented system boot | `BootCortex`; guard released during explicit command boot |
| `query` / `why` | Kernel Query / Explain |
| `dream` / `shadow` / `whatif` | Dreamer / ShadowMode |
| `run` one-shot | Boot + single OODA |
| mangle-check | Related to mangle package; may share schemas |

## 6. Autopoiesis wiring

- `HotLoadRule` / learned file append from self-modifying tools  
- `LearnedRuleInterceptor` set for repair shard validation  
- `SelfHealer` + validators for bad rule/text recovery  
- Tool generation actions (`ouroboros_*`, `generate_tool`) handled in VS workflows/tools  

## 7. Persistence wiring

| Store | Set on | Purpose |
|-------|--------|---------|
| `store.LocalStore` | VS | knowledge queries / virtual preds |
| `store.LearningStore` | VS / ShardManager | cross-session patterns |
| DreamRouter savers | Dreamer | confirmed dream learnings |

## 8. Transparency wiring

```
vs.SetGlassBoxBus(bus)
vs.SetToolEventBus(toolBus)
sm.SetGlassBoxBus(bus)
```

RouteAction emits routing + tool events; spawn emits shard lifecycle events.

## 9. Feature-flag wiring

| Flag | Construction impact |
|------|---------------------|
| Per-shard facts | `NewCortexKernel` builds `ShardFactRouter`; `RegisterShard` registers owners |
| Diff eval | `evaluate()` chooses `evaluateDiffLocked` when eligible |

Boot should log flag state for supportability.

## 10. Test / e2e wiring patterns

E2E packages construct minimal cores:

- RealKernel + in-memory facts  
- VirtualStore with fake tactile executor  
- Dreamer clone safety  
- Shadow commit boundaries  

See: `tests/e2e/session_executor_kernel_integration_test.go`, `dreamer_kernelclone_integration_test.go`, `mcp_virtualstore_integration_test.go`, `shadowmode_commit_safety_boundary_test.go`.

## 11. Wiring anti-patterns

| Anti-pattern | Why bad |
|--------------|---------|
| RouteAction with boot guard still on after user typed | All tools blocked |
| DisableBootGuard before rehydrate complete | Stale next_action fires |
| Kernel without VS when virtual preds required | Incomplete eval |
| VS without kernel | Exact permission gate denies; indicates incomplete DI |
| Register two Cortex domains claiming same predicate | Last-wins warning only |
| HotLoad invalid rule without sandbox | Mitigated by HotLoadRule — do not bypass |

## 12. Registration checklist for a new action

1. Add `ActionType` constant  
2. Handler in appropriate `virtual_store_*.go` + switch in `executeAction`  
3. `safe_action`/dangerous classification and exact `permitted/3` envelope in policy
4. Mark `isDestructiveAction` if irreversible  
5. Decl any new result predicates in schemas  
6. Validator if success is non-obvious  
7. Unit + preferably e2e test  
8. Document in ActionType table if public
