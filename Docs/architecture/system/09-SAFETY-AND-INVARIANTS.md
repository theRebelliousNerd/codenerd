# system — Safety and Invariants

> Last verified: **2026-07-13**

## 1. Constitutional safety (wiring role)

`system` does not evaluate `permitted(...)`. It **registers** the policy domain so the kernel can:

```
permitted / blocked / constitution / commit_barrier / dangerous_action
```

VirtualStore is constructed with a real tactile executor and attached kernel — effectful actions later go through kernel-backed gates implemented in `core` + policy `.mg`.

### Invariant S1 — Policy domain present on production boot

Unless `KernelOverride` supplies a mock kernel without domains, a normal boot includes the policy ownership set in `initKernel`.

### Invariant S2 — Default deny remains kernel-side

Disabling system shards (e.g. `constitution_gate`) is allowed via `DisableSystemShards` for tests. Production CLI should not casually disable constitutional system shards.

## 2. Cache safety

### Invariant C1 — Identity completeness

Key includes workspace, provider, apiKey, **and** model. Omitting any reintroduces Bug #15 class failures (wrong Cortex after mid-session switch).

### Invariant C2 — No failure poisoning

```
Boot err → return err  // cache untouched
```

### Invariant C3 — Double-check under write lock

Concurrent first-boots cannot insert duplicates for the same key.

### Invariant C4 — Close evicts only cached instances

`cortexKey == ""` (direct Boot*) → Close does not touch map.  
`cortexKey != ""` → `evictCortexByKey`.

### Invariant C5 — Reset does not Close

`ResetGlobalCortex` / `ResetCortexForWorkspace` only delete map entries. Callers holding pointers must Close themselves or risk leaks.

## 3. Concurrency

| Resource | Protection |
|----------|------------|
| `cortexCache` | `sync.RWMutex` |
| Boot under GetOrBoot | Write lock held for entire BootCortex (serializes all first boots) |
| Holographic memCache | `sync.Mutex` on no-LocalDB path |
| Maintenance | Single goroutine per fresh GetOrBoot insert |
| MCP ConnectAll | Background goroutine; logs on failure |

### Invariant K1 — Maintenance and Close race

`runMaintenance` assumes `LocalDB` non-nil without re-check. Close sets LocalDB nil. **Potential panic** if ticker fires during/after Close. Treat as known hazard (see gaps).

## 4. Credential handling

### Invariant P1 — API keys not stored in plain cache keys

Keys are hashed (SHA-256 hex). Cache map keys do not leak raw API keys in logs of map iteration.

### Invariant P2 — Provider resolution is best-effort for keying

If config is unreadable, provider/model strings are empty for the key; Boot will hit the same failure mode later. Consistent empty components keep retries aligned.

## 5. Soft-fail vs hard-fail

See principle P5 in [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md). Summary invariant:

### Invariant F1 — Missing LLM must not hard-fail boot

`missingLLMClient` ensures non-LLM commands work. First Complete* returns a clear error.

### Invariant F2 — Kernel Evaluate hard-fails boot

A non-evaluating cortex is not returned as a half-built success.

## 6. Mangle surface (package-local)

No package-owned `.mg` policy. Mangle interactions:

| Site | Behavior |
|------|----------|
| KernelAdapter.AssertBatch | parse.Unit on string facts; NameType → `core.MangleAtom` |
| mcpKernelAdapter | Assert/Query/Retract with careful trailing-dot rules |
| Hybrid prompts | ConsumeBootPrompts → atom store (not rule evaluation) |
| Holographic deep facts | LoadFacts / RetractExactFactsBatch |

### Invariant M1 — Retract must not double-append dots

`mcpKernelAdapter.Retract` trims trailing `.` before `core.ParseFactString` (which appends its own). Regression-sensitive.

## 7. Resource / Windows invariant

### Invariant R1 — Close releases SQLite

Tests on Windows fail TempDir cleanup if knowledge/learning DBs remain open. `Close` closes LocalDB, LearningStore, JITCompiler, and `perception.ClosePerceptionLayer`.

## 8. System shard disable list

Names in `DisableSystemShards` are applied via `ShardManager.DisableSystemShard` before `StartSystemShards`. Empty list = all registered system shards start.

## 9. Safety checklist for PR reviewers

- [ ] New boot resource has Close path  
- [ ] New cache field is part of identity or intentionally excluded  
- [ ] No new process-global unkeyed Cortex  
- [ ] Soft-fail vs hard-fail choice documented  
- [ ] Adapter fact parsing handles Mangle name constants  
- [ ] DisableSystemShards not expanded in production defaults  
