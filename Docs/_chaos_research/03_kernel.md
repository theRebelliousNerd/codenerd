# 03 - Kernel & Fact Store

## 1. RealKernel Structure & Locking

The `RealKernel` struct (`kernel_types.go:40-66`) is the Mangle inference engine wrapper. Fields relevant to chaos testing:

| Field | Type | Chaos Relevance |
|-------|------|-----------------|
| `mu` | `sync.RWMutex` | **Single lock for ALL kernel state** - every Assert, Query, Evaluate, Retract contends on this one mutex |
| `facts` | `[]Fact` | Unbounded slice - no cap enforcement at the kernel level |
| `cachedAtoms` | `[]ast.Atom` | Parallel array that must stay in sync with `facts`; desync detected at `kernel_eval.go:128` |
| `factIndex` | `map[string]struct{}` | Dedup index; lazy-initialized (`kernel_facts.go:336-340`), can be nil |
| `store` | `factstore.FactStore` | **Rebuilt from scratch on every evaluate() call** (`kernel_eval.go:125`) |
| `derivedFactLimit` | `int` | Gas limit for Mangle inference, defaults to 500K (`kernel_eval.go:149-150`) |
| `policyDirty` | `bool` | Triggers full reparse of schemas+policy+learned on next evaluate |

### The Single-Mutex Pattern

Every public method (`Assert`, `AssertBatch`, `Retract`, `Query`, `Evaluate`, `Clear`, `Reset`) acquires `k.mu`. There is no lock striping, no reader-writer separation for facts vs. policy, and no lock-free reads. Under high shard concurrency (12 parallel shards), this becomes a serialization bottleneck. Read-path (`Query`) uses `RLock`, but any write (`Assert`) escalates to full `Lock`, blocking all readers.

### derivedFactLimit Enforcement

The limit is passed to `engine.WithCreatedFactLimit(derivedFactLimit)` at `kernel_eval.go:156`. If exceeded, the Mangle engine returns an error containing "limit" or "exceeded" (`kernel_eval.go:187`). However, the **EDB fact count (`len(k.facts)`) is never checked against any limit** - only derived/IDB facts have a gas cap. The `LimitsEnforcer.GetMaxFactsInKernel()` exists but is never called inside `addFactIfNewLocked` or `Assert`.

## 2. Assert() - The Input Gate

### Validation Path (kernel_facts.go:378-396)

`Assert(fact)` performs the following steps:
1. `sanitizeFactForNumericPredicates(fact)` - Coerces priority atoms to numbers for `agenda_item`, `prompt_atom`, `atom_priority` predicates only (`kernel_facts.go:1114-1133`). Unknown priority strings pass through unchanged with a warning log (`kernel_facts.go:1166`).
2. `addFactIfNewLocked(fact)` - Deduplication check via `canonFact()` string key in `factIndex` map.
3. `evaluate()` - Full Mangle fixpoint re-evaluation.

**What Assert does NOT validate:**
- No schema validation (the `schemaValidator` field exists but is not called in `Assert`)
- No predicate existence check (any arbitrary predicate name is accepted)
- No argument count/type validation
- No fact count limit check against `MaxFactsInKernel`
- No size limit on individual fact argument values (a 10MB string argument is accepted)

### sanitizeFactForNumericPredicates (kernel_facts.go:1114-1133)

Only targets 3 predicates (`agenda_item`, `prompt_atom`, `atom_priority`). All other predicates pass through unmodified. The `coercePriorityAtomToNumber` function maps string atoms like `/critical` -> `int64(100)`, `/high` -> `int64(80)`, etc. Unknown atoms log a warning but are **not rejected** - they pass through as-is, potentially causing Mangle type mismatches downstream.

### addFactIfNewLocked Dedup Logic (kernel_facts.go:354-375)

Deduplication uses `canonFact(f)` which produces a canonical string representation. Key behavior:
- If `ToAtom()` fails on a fact, the fact is **still added** to `k.facts` and `k.factIndex` but **not** to `k.cachedAtoms` (`kernel_facts.go:363-368`). This creates a **cache desync** where `len(k.cachedAtoms) != len(k.facts)`, which is detected and recovered from in `evaluate()` at `kernel_eval.go:128-138` by rebuilding the entire cache.
- The dedup key is based on Go `fmt.Sprint` serialization of arguments, which means two semantically identical facts with different Go types (e.g., `int(42)` vs `int64(42)`) could be treated as distinct.

### AssertBatch (kernel_facts.go:401-434)

Optimized O(N) path: adds all facts, then evaluates once. No additional validation beyond single Assert. The `AssertWithoutEval` variant (`kernel_facts.go:451-459`) is even more permissive - no evaluation at all, facts accumulate silently.

## 3. evaluate() - Mangle Engine

### The Full Store Rebuild Pattern (kernel_eval.go:107-208)

Every call to `evaluate()`:
1. Checks if `policyDirty` or `programInfo == nil` - if so, calls `rebuildProgram()` which re-parses the entire schemas+policy+learned string from scratch (`kernel_eval.go:113-120`)
2. Creates a **brand new** `factstore.NewSimpleInMemoryStore()` (`kernel_eval.go:125`) - the old store is discarded
3. Iterates over ALL cached atoms and adds them to the new store (`kernel_eval.go:142-144`)
4. Runs `engine.EvalStratifiedProgramWithStats()` to fixpoint (`kernel_eval.go:180-181`)
5. Replaces `k.store` with the new store (`kernel_eval.go:193`)

This means every `Assert()` call triggers a complete re-evaluation of all rules against all facts. With 100K EDB facts and complex policy rules, this is O(N * R) per assertion where N is fact count and R is rule complexity.

### Cache Desync Recovery (kernel_eval.go:128-138)

If `len(k.cachedAtoms) != len(k.facts)`, evaluate() rebuilds the entire atom cache by calling `ToAtom()` on every fact. If any individual `ToAtom()` fails during this rebuild, evaluate() **returns an error** (`kernel_eval.go:134-135`), which is stricter than `addFactIfNewLocked` which silently accepts broken facts. This creates a scenario where Assert succeeds but the next evaluate fails.

### Derived Fact Limit (kernel_eval.go:148-157)

Default 500K derived facts. Passed as `engine.WithCreatedFactLimit(derivedFactLimit)`. If breached, the error is caught at `kernel_eval.go:184-191` and logged as a possible "FACT EXPLOSION". The evaluation **fails** and the kernel's `k.store` is NOT updated (the old store remains).

### debug_program_ERROR.mg Dump (kernel_eval.go:70-74)

When `rebuildProgram()` fails analysis, the entire program string (schemas+policy+learned concatenated) is dumped to `debug_program_ERROR.mg` in the CWD. This is a **blocking synchronous file write** with no size limit. With large policy files (50K+ lines), this dump can be significant. The dump path is hardcoded and not sanitized.

## 4. Limits - Defined But Not Enforced

### LimitsEnforcer (limits.go:40-51)

The `LimitsEnforcer` provides check methods but **does not actively enforce** - callers must invoke checks.

| Limit | Default | Enforcement Status |
|-------|---------|-------------------|
| `MaxTotalMemoryMB` | 12,288 (12GB) | `CheckMemory()` exists (`limits.go:103-131`) but must be called explicitly. **Not called in Assert/Evaluate paths.** |
| `MaxConcurrentShards` | 12 | `CheckShardLimit()` exists (`limits.go:230-252`), called by ShardManager spawn logic |
| `MaxSessionDurationMin` | 120 (2hr) | `CheckSessionDuration()` exists (`limits.go:158-187`), callback-based |
| `MaxFactsInKernel` | 250,000 | `GetMaxFactsInKernel()` is a **getter only** (`limits.go:303-304`). **No enforcement function exists.** No code anywhere compares `len(k.facts)` to this limit. |
| `MaxDerivedFactsLimit` | 100,000 | `GetMaxDerivedFactsLimit()` is a **getter only** (`limits.go:308-309`). The kernel uses its own `derivedFactLimit` field (default 500K, not 100K). |

**Critical gap**: The `MaxFactsInKernel = 250,000` is defined in config but has zero enforcement anywhere in the codebase. `Assert()` will happily add fact #250,001 with no warning. Similarly, `MaxDerivedFactsLimit = 100,000` in config disagrees with the kernel's hardcoded default of `500,000` at `kernel_eval.go:150`.

### Memory Check (limits.go:108-112)

Uses `runtime.ReadMemStats()` which only measures Go heap allocation (`m.Alloc`), not total process RSS. CGO allocations (sqlite-vec) are invisible. The 12GB limit could be exceeded by native memory without triggering.

## 5. VirtualStore Constitutional Gate

### The 4 Hardcoded Constitutional Rules (virtual_store.go:834-911)

| # | Rule Name | Check Logic | Bypass Vector |
|---|-----------|-------------|---------------|
| 1 | `no_destructive_commands` | Checks `req.Target` against 5 substrings: `rm -rf`, `mkfs`, `dd if=`, `:(){`, `chmod 777` | Case-insensitive via `ToLower`, but `rm -r -f` (separate flags) bypasses. `del /s /q` (Windows) bypasses. |
| 2 | `no_secret_exfiltration` | Requires BOTH a secret keyword in `payload` AND a dangerous tool in `target`. Secret: `.env`, `credentials`, `secret`, `api_key`, `password`. Dangerous: `curl`, `wget`, `nc `, `netcat`. | Using only one (e.g., `curl` without `secret` in payload) bypasses. Base64-encoding secrets bypasses. `powershell Invoke-WebRequest` bypasses. |
| 3 | `path_traversal_protection` | Checks `req.Target` for `..` substring. Only applies to `ActionReadFile`, `ActionWriteFile`, `ActionDeleteFile`. | Symlink traversal bypasses. Absolute paths outside workspace bypass. `ActionEditFile` is excluded in the type check at `virtual_store.go:884` but then **included** at `virtual_store.go:897`. |
| 4 | `no_system_file_modification` | Checks `req.Target` prefix against 5 system paths: `/etc/`, `/usr/`, `/bin/`, `/sbin/`, `C:\Windows\`. | Case-sensitive on Windows (`c:\windows\` bypasses). Symlinks from workspace into system dirs bypass. No protection for `C:\Program Files\`. |

### CheckKernelPermitted Caching (virtual_store.go:1553-1612)

The permission cache (`permittedCache`) is a `map[string]bool` built from `safe_action` query results (`virtual_store.go:456-486`). Issues:

1. **Stale cache**: `rebuildPermissionCache()` is called lazily only when `cache == nil` (`virtual_store.go:1573`). There is no invalidation when new `safe_action` facts are asserted or policy changes. The comment at `virtual_store.go:966` acknowledges "Cache is deprecated for fine-grained permissions" but the cache is still used at `virtual_store.go:1580-1585`.
2. **Cache bypass fallback**: If cache misses, falls through to querying `permitted` facts directly (`virtual_store.go:1588`), which is correct but slower.
3. **Race window**: `rebuildPermissionCache` acquires `v.mu.Lock()` while the kernel query inside it acquires `k.mu.RLock()` - potential lock ordering issue if kernel operations also access VirtualStore.

### RouteAction Flow (virtual_store.go:924-1028)

1. Boot guard check (blocks all actions until first user interaction, `virtual_store.go:930-936`)
2. Parse action fact (`virtual_store.go:941`)
3. Constitutional check (`virtual_store.go:954`) - runs the 4 hardcoded rules
4. Kernel permission check (`virtual_store.go:964-978`) - queries Mangle `permitted` predicate
5. Execute action (`virtual_store.go:984`)
6. Post-action validation (`virtual_store.go:996-1011`)
7. Inject result facts back into kernel (`virtual_store.go:1013-1022`)

Step 7 creates a feedback loop: action results become kernel facts, which may trigger new `next_action` derivations on the next evaluate.

## 6. CHAOS FAILURE PREDICTIONS

| # | Prediction | Severity | Location | Rationale |
|---|-----------|----------|----------|-----------|
| 1 | **Unbounded EDB growth crashes kernel via OOM** | **CRITICAL** | `kernel_facts.go:354-375` | `addFactIfNewLocked` has no size cap. `MaxFactsInKernel=250K` is defined at `limits.go:35` but never enforced. A runaway transducer or result-injection loop (VirtualStore injecting `execution_result` facts at `virtual_store.go:1017-1020`) can grow `k.facts` unboundedly. Each `evaluate()` copies ALL facts into a new `SimpleInMemoryStore`, doubling peak memory. |
| 2 | **evaluate() full-rebuild causes OOM with large fact sets** | **CRITICAL** | `kernel_eval.go:125-144` | Every evaluate creates a fresh store and re-adds all atoms. With 200K facts, this means 200K atom copies plus fixpoint computation. Peak memory is ~3x the fact set (old store + new store + derivation working set). Combined with 500K derived fact limit, worst case is 700K facts in memory simultaneously. |
| 3 | **Malicious facts poison Mangle evaluation** | **HIGH** | `kernel_facts.go:378-396` | Assert performs no schema validation. Any predicate name is accepted. An attacker (or buggy transducer) can inject `permitted(/rm_rf_everything).` or `safe_action(/exec_cmd).` directly, bypassing constitutional checks. The kernel will happily derive `permitted` for destructive actions. |
| 4 | **Cache desync between facts[] and cachedAtoms[] causes silent data loss** | **HIGH** | `kernel_facts.go:363-368`, `kernel_eval.go:128-138` | When `ToAtom()` fails in `addFactIfNewLocked`, fact is added but atom is not. Next `evaluate()` detects desync and rebuilds cache - but if the rebuild's `ToAtom()` also fails, evaluate returns error and the kernel is stuck in a non-functional state with an uninitialized store. |
| 5 | **Concurrent Assert() calls serialize on single mutex, causing shard starvation** | **HIGH** | `kernel_types.go:41`, `kernel_facts.go:382-383` | With 12 concurrent shards each calling Assert, all contend on `k.mu.Lock()`. Each Assert triggers `evaluate()` which can take 100ms+ with large fact sets. 12 shards * 100ms = 1.2s round-trip per Assert cycle. Query operations (`RLock`) are blocked during any write. |
| 6 | **VirtualStore permission cache returns stale allow for revoked actions** | **HIGH** | `virtual_store.go:1573-1585` | `permittedCache` is built once lazily and never invalidated when policy changes or facts are retracted. If `safe_action(/exec_cmd)` is retracted, the cache still returns `true` for `exec_cmd` until the VirtualStore is restarted or cache is manually set to nil. |
| 7 | **Constitutional rules trivially bypassed on Windows** | **MEDIUM** | `virtual_store.go:900` | System path check uses case-sensitive `HasPrefix`. On Windows, `c:\windows\system32` bypasses `C:\Windows\` check. Also missing `C:\Program Files\`, `C:\ProgramData\`, and WSL paths. `rm -r -f` with separated flags bypasses `rm -rf` substring check. |
| 8 | **Lock ordering deadlock between VirtualStore.mu and Kernel.mu** | **MEDIUM** | `virtual_store.go:1574-1577`, `virtual_store.go:462` | `rebuildPermissionCache` holds `v.mu.Lock()` then calls `k.Query()` which acquires `k.mu.RLock()`. If another goroutine holds `k.mu.Lock()` (during Assert) and calls into VirtualStore (e.g., `injectFact`), a deadlock occurs. The VirtualStore's `injectFact` calls `k.Assert()` which needs `k.mu.Lock()`. |
| 9 | **debug_program_ERROR.mg dump leaks full policy to CWD** | **MEDIUM** | `kernel_eval.go:70-74` | When analysis fails, the entire program (schemas+policy+learned rules) is written to a predictable path. On multi-user systems, this could expose security policy. The file is never cleaned up. Repeated failures overwrite without warning. |
| 10 | **Derived fact limit mismatch: config says 100K, kernel defaults to 500K** | **MEDIUM** | `limits.go:36`, `kernel_eval.go:150` | `DefaultLimitsConfig().MaxDerivedFactsLimit = 100,000` but `evaluate()` defaults to `500,000` when `k.derivedFactLimit <= 0`. If the config value is never wired to `k.derivedFactLimit`, the kernel operates at 5x the intended gas limit, allowing fact explosions the operator thought were capped. |
| 11 | **Result fact injection creates unbounded feedback loop** | **HIGH** | `virtual_store.go:1013-1022` | After every action execution, RouteAction injects `execution_result` facts back into the kernel via `injectFacts`. These facts trigger `evaluate()`, which may derive new `next_action` atoms, which trigger more RouteAction calls, creating an unbounded cycle. No circuit breaker or depth counter exists. |
| 12 | **GoString canonicalization inconsistency causes dedup failure** | **MEDIUM** | `kernel_facts.go:354-356` | `canonFact()` uses `fmt.Sprint`-based serialization. Type-distinct but value-identical arguments (`int(42)` vs `int64(42)` vs `float64(42)`) produce different canonical keys, so the "same" fact can be added multiple times with slightly different Go types. This inflates the fact store silently. |
