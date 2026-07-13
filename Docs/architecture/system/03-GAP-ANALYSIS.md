# system — Gap Analysis

> Last verified: **2026-07-13**

## 1. Spec vs reality matrix

| Expectation (vision / north star) | Reality in code | Gap? |
|-----------------------------------|-----------------|------|
| Single cache entry for all long-lived Cortex | CLI: `GetOrBootCortex`. TUI: `BootCortexWithConfig` (no cache) | **Yes** — dual identity models in one process |
| GetOrBootCortex for all command handlers | All sampled Cobra handlers use it | **No** for Cobra |
| Failed boots not cached | Explicit: error path never inserts | **No** |
| Config change → fresh Cortex | `ResetCortexForWorkspace` exists | **Partial** — no production callers found; TUI still separate |
| Close tears down everything | Shards, JIT, LocalDB, Learning, perception, cache | **Partial** — maintenance goroutine not cancelled; MCP bridge not closed here |
| Session file ops respect VirtualStore policy | `sessionVirtualStoreAdapter` uses `os.ReadFile`/`WriteFile` | **Yes** |
| LoadReasoningTrace via adapter | Returns `nil, nil` always | **Yes** — write path only |
| Embedding failure fails boot | Soft-warn; continue without engine | **No** (by design) |
| Missing LLM fails boot | Soft: `missingLLMClient`; boot continues | **No** (by design) |
| Hybrid PROMPT → JIT corpus | `IngestHybridPrompts` | **No** |
| User agents registered to JIT + ShardManager | Discover + DefineProfile TypeUser | **No** |
| Maintenance archival on schedule | 30m ticker when LocalDB present; only via GetOrBootCortex path | **Partial** — direct Boot* never starts it |
| `debug_program_ERROR.mg` not in package | Present as dump artifact | Hygiene gap only |

## 2. Prioritized gaps

### P0 — correctness / resources

1. **Maintenance cancel discarded**  
   `GetOrBootCortex` calls `StartMaintenanceSchedule` and ignores `context.CancelFunc`. `Close` does not stop the ticker. A closed-but-still-referenced LocalDB risk is mitigated by Close nil-ing LocalDB *if* the goroutine observes that — but the goroutine holds `*Cortex` and will call `runMaintenance` on a nil LocalDB path only after Close sets LocalDB nil (then `runMaintenance` panics on `c.LocalDB.MaintenanceCleanup` if called after Close).  
   - **Mitigation needed:** store cancel on Cortex; call from Close; guard `runMaintenance` for nil LocalDB.

2. **TUI vs CLI Cortex duality**  
   Two full boots possible in one process; no shared cache; double SQLite handles, double system shards.  
   - Prefer TUI adopt GetOrBootCortex or register into same cache after BootCortexWithConfig.

### P1 — policy / integrity

3. **sessionVirtualStoreAdapter file I/O bypass**  
   Session executor file tools may skip VirtualStore constitutional routing depending on call path.  
   - Route through VS when non-nil.

4. **ResetCortexForWorkspace unused**  
   Provider/key rotation mid-process may leave stale cache unless callers know to reset.  
   - Wire from config reload / auth commands.

### P2 — completeness

5. **Trace Load path**  
   `LocalStoreTraceAdapter.LoadReasoningTrace` stub.  
6. **MCP bridge lifetime**  
   Bridge created and ConnectAll async; Close does not disconnect.  
7. **BootCortex path vs GetOrBootCortex**  
   Direct BootCortex leaves `cortexKey` empty — Close skips cache eviction (correct for uncached), but no maintenance (correct). Document as intentional; ensure call sites choose deliberately.

### P3 — hygiene

8. Remove or gitignore `debug_program_ERROR.mg` from package tree.  
9. Expand `GetOrBootCortex` unit tests for cache hit / multi-key / failure non-cache.

## 3. Non-gaps (do not “fix”)

| Observation | Why not a gap |
|-------------|----------------|
| Boot continues without embedding | Designed soft-fail; retrieval degrades gracefully |
| Boot continues without LLM | Enables query/store CLI without credentials |
| Write lock serializes all first-boots | Comment in source: acceptable for rare heavy boot |
| System shards disable list | Test / diagnostic escape hatch |
| Package has no `.mg` policy sources | Policy lives in `core/defaults`; system only wires domains |

## 4. Completeness heuristic

| Component | Completeness |
|-----------|--------------|
| Boot pipeline | ~95% |
| Keyed cache | ~95% |
| Lifecycle Close | ~75% |
| Adapter purity | ~70% |
| Test coverage of cache | ~40% |
| **Overall package** | **~85–90%** |
