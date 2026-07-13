# core — Observability

> Last verified: **2026-07-13**

## 1. Logging categories

Core uses `internal/logging` categories (helpers often wrap these):

| Category / helper | Typical events |
|-------------------|----------------|
| `CategoryKernel` / `logging.Kernel` | Boot, load modules, eval, parse errors |
| `logging.KernelDebug` | Fact samples, rebuild sizes, Decl diagnostics |
| `CategoryVirtualStore` / `logging.VirtualStore` | RouteAction, denies, handler outcomes |
| `CategoryDream` / `logging.Dream` | SimulateAction start/block/cache |
| `logging.Shards` | ShardManager lifecycle |
| `logging.Audit().ActionRoute/Complete` | Structured action audit trail |

Timers: `logging.StartTimer(category, name)` around NewRealKernel, rebuildProgram, RouteAction, SimulateAction, handlers.

## 2. Audit trail

Successful routing path:

1. `ActionRoute(type, target)`  
2. Execute  
3. `ActionComplete(type, target, durationMs, success, err)`  

Use audit logs when user-visible TUI only shows summaries.

## 3. Glass Box & tool buses

| Bus | Emitter | Event kind |
|-----|---------|------------|
| `GlassBoxEventBus` | VS routing, ShardManager spawn | CategoryRouting / CategoryShard |
| `ToolEventBus` | VS after action | ToolEvent name/duration/success |

Both are nil-safe and drop-on-full (non-blocking). Wired from chat boot for TUI activity line / scrollback badges.

## 4. Fact-level observability

| Mechanism | Use |
|-----------|-----|
| `execution_result/6` facts | Logical postcondition; executive action ID and timestamp |
| `security_violation/3` | Action, bounded reason, and Unix timestamp for later policy/UI |
| `execution_error/2` | Request/action correlation and error text |
| `dream_blocked_action` | Speculative blocks |
| `FactEventBus` | Push model for system shards vs polling |
| `Query` / CLI `why` | Interactive inspection |
| Provenance `Explain` | Derivation trees when enabled |

## 5. Debug dumps

| Artifact | Trigger |
|----------|---------|
| `debug_program_ERROR.mg` | `rebuildProgram` analysis failure |

Contains concatenated schemas+policy+learned. **Sensitive:** may include user overrides if loaded — treat as support artifact, not always shareable publicly.

Duplicate `Decl permitted(` lines are logged at debug with surrounding context during rebuild (schema inconsistency hunting).

## 6. Metrics (in-process)

| Struct | File | Fields (conceptually) |
|--------|------|------------------------|
| `APISchedulerMetrics` | `api_scheduler.go` | Slot usage / scheduling stats |
| `ShardMetrics` | `kernel_shard.go` | Per-domain counters |
| `ShardRouterMetrics` | `shard_fact_router.go` | Forward counts |
| Cortex `routeHitCount` / `routeMissCount` | `cortex_kernel.go` | Routing efficacy |
| tactile `ExecutionMetricsSnapshot` | via VS | Executor stats |

No first-class OTEL exporter in core; consumers may scrape these structs.

## 7. Performance observability

- Kernel timers on rebuild vs evaluate  
- Dreamer timers + cache hit logs  
- Diff-eval path selection (feature/env) — log when debugging latency  
- Derived fact / EDB counts via `FactCount` and logs on LoadFacts  

## 8. Operator playbooks

### “Action blocked” with little UI detail

1. VirtualStore warn logs: `policy DENY` includes payload **keys** only.  
2. Query kernel for schema-correct `security_violation/3` / `permission_denied`.
3. Check boot guard active.  
4. Check Dreamer logs for `ACTION BLOCKED`.

### Kernel boot failure

1. Read process error.  
2. Open `debug_program_ERROR.mg` if present.  
3. Search for Decl conflicts / syntax near last modules loaded.  
4. Bisect user `.nerd/mangle/*` overrides.

### Slow campaigns

1. Measure rebuildProgram frequency (policyDirty thrash).  
2. Check fact counts vs maxFacts.  
3. Diff-eval flag status and retract rate.  
4. APIScheduler metrics for LLM queue wait.

## 9. What not to log

- Full secret payloads (VS deliberately logs keys not values on deny)  
- Entire file contents on every read (prefer lengths / paths)  
- Full program source on every successful rebuild (debug only)

## 10. Related corpus

CLI glass box UX: `Docs/architecture/cli/12-TELEMETRY-OBSERVABILITY.md`.  
Logging package: `Docs/architecture/logging/` if present.
