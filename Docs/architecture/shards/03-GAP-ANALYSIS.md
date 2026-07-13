# 03 — Gap Analysis: shards

> Last verified against codebase: 2026-07-13  
> Vision: [01-VISION.md](01-VISION.md) · Reality: [02-CURRENT-STATE.md](02-CURRENT-STATE.md)

## 1. Spec vs reality matrix

| Capability | Vision | Reality | Gap severity |
|------------|--------|---------|--------------|
| System OODA spine | Full continuous pipeline | Implemented (exec/const/router/perception) | **None** (ops polish only) |
| Domain Go shards | Absent | Absent | **None** (intentional) |
| Single registration path | One factory for all boots | Factory + inline session_boot | **High** |
| Predicate manifests | Authoritative ownership | Exported; partial consume | **Medium** |
| Event-driven shards | Event bus first | Implemented with poll fallback | **Low** |
| Boot guard | No stale auto-run | Implemented executive + VS | **Low** (must call DisableBootGuard) |
| CostGuard | Bound LLM cost | Implemented on base | **Low** |
| JIT for system LLM | Assembler everywhere | Present; some fallbacks | **Low–Med** |
| Specialist matching | Accurate mesh | Path/import heuristics | **Medium** |
| Package README accuracy | Live docs | Migration-only README | **Medium** (docs debt) |
| Manifest ↔ factory owned preds | Identical | Comments claim mirror | **Medium** (manual sync) |
| Campaign runner ShardManager | Always injected | Factory re-register sets it; RegisterAll path may not | **Med** |
| GlassBox on all boots | Consistent | Chat boot richer than factory | **Low–Med** |

## 2. Non-gaps (do not “fix”)

| Observation | Why not a gap |
|-------------|----------------|
| Domain shards “missing” | Replaced by JIT + session by design |
| Constitution without LLM model | Required for deterministic safety |
| Campaign runner OnDemand not Auto | Prevents boot-time campaign execution (intentional profile comment) |
| `AllowUnmappedActions=false` | Safety default |
| Requirements interrogator fails without JIT atoms | JIT-first contract |

## 3. Priority backlog (from gaps)

### P0 — Correctness / safety wiring

1. Ensure every production boot disables boot guard only after intentional user turn.  
2. Keep constitution StrictMode true unless explicitly configuring open mode.  
3. Keep mangle_repair interceptor wired on all boots that load learned rules.

### P1 — Drift elimination

1. Unify session_boot factories with `RegisterAllShardFactories` + extension hooks.  
2. Wire `DefaultShardPredicateManifests` into factory KernelShard construction.  
3. Single test asserting registered factory name set equality across boots.

### P2 — Capability

1. Optional embedding-assisted specialist matching (retrieve then structure).  
2. Expand integration tests for full pending→permitted→route pipeline (exists partially as `action_pipeline_test.go`).  
3. Refresh `internal/shards/README.md` to match this corpus.

### P3 — Polish

1. Remove or ignore historical `*_learnings.db` in package tree if unused.  
2. Align legislator profile Model with constructor ModelConfig.

## 4. Dependency gaps elsewhere that surface here

| External gap | Symptom in shards |
|--------------|-------------------|
| Event bus nil | Fallback tickers; higher kernel evaluate load |
| Missing JIT atoms for system personas | Interrogator / perception JIT fails |
| Policy missing `permitted` rules | Constitution denies everything (correct default) |
| VirtualStore route incomplete | Router emits exec_request that never completes |

## 5. Measurement

When closing a gap, require:

- Path citation in this doc or IMPLEMENTED_SPEC update  
- Targeted `go test ./internal/shards/...` green  
- If registration changed: exercise both factory and chat boot smoke paths when feasible  
