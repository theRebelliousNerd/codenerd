# 11 — Observability: shards

> Last verified against codebase: 2026-07-13

## 1. Logging categories

Primary: **`logging.CategorySystemShards`** via helpers:

| Helper | Use |
|--------|-----|
| `logging.SystemShards(...)` | Info-level system shard events |
| `logging.SystemShardsDebug(...)` | Verbose transitions, config, hydration |
| `logging.Get(logging.CategorySystemShards).Warn/Error` | Failures, blocks, budget exhaustion |
| `logging.StartTimer(CategorySystemShards, ...)` | Learning load / LLM timing |

Related categories used nearby:

- `logging.Routing` — router poll fallback  
- `logging.Shards` — ShardManager (core package)  
- `logging.CategoryKernel` — stale cleanup failures  
- `logging.CategoryBoot` — chat/factory boot steps  

## 2. Structured / glass box

| Mechanism | Wiring | What you see |
|-----------|--------|--------------|
| GlassBoxEventBus | Chat tactile_router `SetGlassBox` | Debug visibility events |
| ToolEventBus | Chat tactile_router | Always-visible tool execution |
| ToolStore | Chat tactile_router | Persisted full tool results |
| ShardManager glass bus | core manager | Spawn/completion overlays |
| `jitConfig.TraceLLMIO` | Base `TraceLLMIOEnabled` | Legislator/repair raw LLM I/O structured logs |

## 3. Kernel-visible health facts

| Fact | Meaning |
|------|---------|
| Heartbeats via `EmitHeartbeat` | Shard liveness for policy |
| `campaign_runner_heartbeat` | Campaign supervisor tick |
| `executive_error` | Policy evaluation failure |
| `executive_blocked` | Barrier active |
| `ooda_timeout` | Pending intent without actions |
| `permission_check_result` | /permit or /deny audit stream |
| `security_violation` | Blocked action audit |
| `strategy_activated` | Strategy change |

Pruning: constitution prunes old `permission_check_result` after ~15 minutes (throttled every 10s).

## 4. Metrics held in memory

| Shard | Metrics |
|-------|---------|
| Executive | decisionsCount, blockCount, strategyChanges, pending/blocked lists |
| Perception | intentsProcessed, clarifications |
| Constitution | violations, permitted list |
| Router | pendingCalls, completedCalls |
| Planner | tasksCompleted, tasksBlocked |
| CostGuard | callsThisMinute/Session, validationRetriesUsed |

Access via getters (`GetMetrics`, `GetViolations`, etc.) for UI/status commands.

## 5. Debug dumps

If the process crashes during Mangle evaluation, a combined program dump may land as `debug_program_ERROR.mg` (observed under `internal/shards/system/` in tree). Use for post-mortem; do not treat as source of truth.

## 6. Operator tips

```text
# When OODA seems stuck
- Check boot guard still active?
- Query ooda_timeout / pending_intent / next_action / pending_action
- Check executive_blocked / block_commit barriers

# When tools never run
- Is permitted_action asserted?
- Is tactile_router running (OnDemand)?
- AllowUnmappedActions? action pattern missing?

# When LLM storm
- CostGuard cooldowns in system shard logs
- MaxLLMCallsPerMinute/Session
```

## 7. Gaps

- No centralized Prometheus-style metrics exporter from system shards  
- Factory boot path has less GlassBox/ToolStore wiring than chat  
- Heartbeat semantics depend on policy rules outside this package  
