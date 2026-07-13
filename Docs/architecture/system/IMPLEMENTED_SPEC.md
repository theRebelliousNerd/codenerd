# system — Implemented Spec (Motherboard / Cortex Factory)

> Last verified against codebase: **2026-07-13**  
> Status: Living reference — **code-grounded full corpus**  
> Mode: 1:1 with `internal/system/`  
> Implementation: **5** non-test `.go`, **11** tests, **0** intentional package `.mg`  
> Scale: ~2,100 lines of source; dominated by `factory.go` (~1,150 lines)

---

## 1. Purpose

`internal/system` is codeNERD’s **composition root** — the “Motherboard” that wires every major runtime subsystem into a single `Cortex` value.

It answers one operational question:

> Given a workspace (and optional API key / shard disable list), what is the fully initialized agent runtime I can call from a CLI command or interactive session?

It does **not** run the OODA loop. It **builds** the graph that `session`, VirtualStore, shards, perception, and articulation use once the user speaks.

### North-star placement

| Role | Owner |
|------|--------|
| LLM = creative center | Wired here as main/shard/image clients; used elsewhere |
| Mangle kernel = executive | Booted and domain-registered here |
| Constitutional safety | Policy domain registered; rules in `core/defaults/policy` |
| JIT prompts | Compiler + assembler + corpora wired here |

### Fact-flow position

```
user input
  → perception.Transducer          (on Cortex)
  → user_intent / next_action      (Cortex.Kernel)
  → VirtualStore / TaskExecutor    (on Cortex)
  → articulation / session         (on Cortex)
  → stdout / TUI
```

`system` constructs every arrow’s endpoints; execution packages fire them.

---

## 2. Implementation status

| Component | Status | Notes |
|-----------|--------|-------|
| `GetOrBootCortex` keyed cache | **VERIFIED CURRENT** for accepted inputs, **PARTIAL** for engine mode | SHA-256 identity includes normalized disabled-shard set; failure-safe reuse/split/retry tests pass; separate engine mode omitted |
| `BootCortex` / `BootCortexWithConfig` | **VERIFIED CURRENT** | Full 9-stage named pipeline with aggregate error rollback |
| `Cortex` aggregate | **VERIFIED CURRENT** | Public fields for all major deps |
| Soft-fail missing LLM | **VERIFIED CURRENT** | `missingLLMClient` |
| Multi LLM roles (main/shards/image) | **VERIFIED CURRENT** | Scheduled via `core.NewScheduledLLMCall` |
| Kernel multi-domain CortexKernel | **VERIFIED CURRENT** | routing/world/tools/policy/campaign/prompts/cortex |
| VirtualStore full wire | **VERIFIED CURRENT** | dream, tx, tools, MCP, code scope, task exec |
| JIT + PromptAssembler | **VERIFIED CURRENT** | Hard-fail if embedded corpus missing |
| Hybrid PROMPT ingest | **VERIFIED CURRENT** | `IngestHybridPrompts` |
| User agent discovery + registry | **VERIFIED CURRENT** | disk + `agents.json` + JIT + profiles |
| HolographicCodeScope | **VERIFIED CURRENT** | core↔world cycle breaker |
| System shard start | **VERIFIED CURRENT** | RegisterAll + tactile_router + campaign_runner |
| Maintenance schedule | **VERIFIED CURRENT** | Starts on GetOrBoot, waits one interval, stores cancel/done, stops before LocalDB |
| `Cortex.Close` | **VERIFIED CURRENT** for enumerated owners, **PARTIAL** for typed registry | Idempotent bounded maintenance/shard/MCP/browser/embedding/JIT/DB/perception/cache cleanup |
| Exact policy authorization envelope | **VERIFIED CURRENT** | Complete envelope owned by policy shard; exact mismatch denial tested |
| Prompt compilation isolation | **VERIFIED CURRENT** | Private RealKernel clone per compilation; error/cancel/concurrency tested |
| Boot failure rollback | **VERIFIED CURRENT** for forced late failure | Shared aggregate cleanup closes verified owned resources and joins cleanup error with primary stage error |
| TUI cache integration | **PARTIAL** | Uses BootCortexWithConfig directly |
| Trace load adapter | **PARTIAL** | Store only; load returns nil, nil |
| GetOrBoot unit tests | **VERIFIED CURRENT** for reuse/split/retry | Close eviction, Reset semantics, and engine identity remain open |

**Overall:** production-used and package-tested. The disabled-shard identity and
late-failure leak are repaired; engine identity, typed cleanup registry, and the
P1 policy-bypass adapter remain explicitly open. Percentage-complete is
not used because those risks are not interchangeable with low-impact polish.

---

## 3. Source inventory

### 3.1 Layout

```
internal/system/
  factory.go                 # cache + boot pipeline + Cortex + hybrid ingest
  factory_adapters.go        # boundary adapters (kernel/session/mcp/trace)
  agent_registry.go          # .nerd/agents discovery + agents.json
  holographic_code_scope.go  # CodeScope + deep facts
  cortex_close.go            # Close lifecycle
  *_test.go                  # 19 test files, 61 named tests
  debug_program_ERROR.mg     # crash dump artifact (not source)
```

### 3.2 Non-test files by size

| Path | Lines (approx.) | Role |
|------|----------------:|------|
| `internal/system/factory.go` | 1280 | Flagship |
| `internal/system/factory_adapters.go` | 480 | Adapters |
| `internal/system/agent_registry.go` | 284 | Agents |
| `internal/system/holographic_code_scope.go` | 172 | World bridge |
| `internal/system/cortex_close.go` | 104 | Teardown |

### 3.3 Related docs in this corpus

See [README.md](README.md) document map for 00–12 + governance files.

---

## 4. Public surface (authoritative summary)

### 4.1 Types

| Type | File | Role |
|------|------|------|
| `SystemKernel` | factory.go | `core.Kernel` + Evaluate / LoadFactsFromFile / ConsumeBootPrompts |
| `BootConfig` | factory.go | Workspace, APIKey, DisableSystemShards, DI overrides |
| `Cortex` | factory.go | Fully initialized runtime aggregate |
| `AgentOnDisk` | agent_registry.go | `{ID, DBPath}` |
| `KernelAdapter` | factory_adapters.go | core → prompt.KernelQuerier |
| `LocalStoreTraceAdapter` | factory_adapters.go | perception.TraceStore (load stub) |
| `HolographicCodeScope` | holographic_code_scope.go | CodeScope + deep fact maintenance |

### 4.2 Functions

| Symbol | Contract |
|--------|----------|
| `GetOrBootCortex` | Preferred process entry; cache + maintenance |
| `BootCortex` | Thin wrapper → WithConfig |
| `BootCortexWithConfig` | Full DI boot |
| `ResetGlobalCortex` | Clear map; no Close |
| `ResetCortexForWorkspace` | Evict by Workspace path |
| `IngestHybridPrompts` | Hybrid PROMPT → corpus.db |
| `DiscoverAgentsOnDisk` | Scan agents dirs |
| `SyncAgentRegistryFromDisk` / `FromDiscovered` | Upsert agents.json |
| `NewKernelAdapter` | Constructor |
| `NewHolographicCodeScope` | Constructor |

### 4.3 Cortex methods

| Method | Contract |
|--------|----------|
| `SpawnTask` | System profiles → ShardManager; else TaskExecutor |
| `SpawnTaskWithContext` | Same + priority/session |
| `StartMaintenanceSchedule` | 30m LocalDB archival loop; returns cancel |
| `Close` | Idempotent bounded shard/MCP/browser/embedding/JIT/DB/perception close; cache evict |

Full API tables: [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md).

---

## 5. Deep dive: GetOrBootCortex factory

This is the **primary production entry** for Cobra handlers.

### 5.1 Why it exists

Early code used a process-wide singleton Cortex. Switching workspace, provider, API key, or model mid-process returned a Cortex wired to the **wrong** context (**Bug #15**). The fix is a **keyed multi-instance cache**.

### 5.2 Identity model

```
disabled = normalizeDisableSystemShards(input)
key = SHA256_hex(length_delimited(workspace, provider, apiKey, model, disabled...))
```

- Workspace resolved via `resolveWorkspaceRoot` (arg → FindWorkspaceRoot → cwd).  
- Provider/model from best-effort `config.LoadUserConfig(.nerd/config.json)`.  
- Disabled names are trimmed, empty entries removed, deduplicated, and sorted.
- Hash avoids leaking API keys as map keys; length delimiters prevent component ambiguity.

**VERIFIED CURRENT** for workspace/provider/key/model/disabled set.
**PARTIAL:** separately configured engine/provider mode remains outside the key.

### 5.3 Algorithm

1. Compute key.  
2. **RLock** fast path — return cached if present.  
3. **Lock** write path (serializes *all* concurrent first-boots, even different keys — intentional simplicity).  
4. Re-check map.  
5. `BootCortex(...)` — on error, return without insert.  
6. Guard against nil cortex without error.  
7. Set `cortex.cortexKey`, insert map.  
8. `StartMaintenanceSchedule(context.Background())` — cancel/done ownership is stored on Cortex.
9. Return cortex.

### 5.4 Invalidation

| API | Behavior |
|-----|----------|
| `ResetGlobalCortex` | New empty map |
| `ResetCortexForWorkspace` | Delete entries where `c.Workspace == resolved ws` |
| `Cortex.Close` | `evictCortexByKey(cortexKey)` if non-empty |

**Important:** Reset APIs do **not** call Close. Holding references after Reset can leak SQLite handles.

### 5.5 Contract for handlers

Source comment:

> IMPORTANT: This function should be used instead of BootCortex() in all command handlers.

Grep evidence (2026-07-13): `cmd_advanced`, `cmd_knowledge`, `cmd_interactive`, `cmd_instruction`, `cmd_direct_actions`, `cmd_test_context`, `cmd_systems`, `cmd_spawn`, `cmd_query`, `cmd_transparency`, campaign/DOM paths import `internal/system` and call GetOrBootCortex (or related).

**Exception:** interactive chat uses `BootCortexWithConfig` in `cmd/nerd/chat/session_shared_boot.go` — **not** GetOrBootCortex.

### 5.6 What GetOrBoot adds beyond BootCortex

| Concern | BootCortex alone | GetOrBootCortex |
|---------|------------------|-----------------|
| Caching | None | Keyed |
| Maintenance | No | Yes (30m) |
| cortexKey | empty | set |
| Concurrent first-boot | Uncoordinated | Serialized under write lock |

---

## 6. Deep dive: BootCortexWithConfig pipeline

### 6.1 Entry points

```go
BootCortex(ctx, workspace, apiKey, disable) 
  → BootCortexWithConfig(ctx, BootConfig{...})

BootCortexWithConfig(ctx, BootConfig)
  → bootContext + defaultBootSteps
  → success: cortexFromBootContext
  → error: rollbackBootContext + named primary error
```

### 6.2 Stage order (binding)

| # | Function | Hard-fail examples | Soft-fail examples |
|---|----------|--------------------|--------------------|
| 1 | `initCoreComponents` | — | logging/usage init warnings |
| 2 | `initPerceptionLayer` | — | missing LLM → stub client |
| 3 | `initStorageLayer` | — | learning store fail |
| 4 | `initKernel` | Evaluate, register shard | perception layer, scan load |
| 5 | `initExecutionLayer` | — | modular tools hydrate |
| 6 | `initAutopoiesisAndBrowser` | — | browser engine fail silent-ish |
| 7 | `initIntelligenceLayer` | embedded corpus, JIT New | embedding, MCP, hybrid, agent sync |
| 8 | `initShardManagement` | StartSystemShards | — |
| 9 | `initFinalExecutors` | — | — |

Each stage runs through `bootCortexWithSteps`. A failure closes an
untransferred project DB, constructs the partial aggregate, calls idempotent
`Cortex.Close`, and joins cleanup errors without replacing the named stage error.

Order note: **Autopoiesis runs before Intelligence**, so `PromptAssembler` is attached to poiesis when created later (`SetPromptAssembler` if poiesis non-nil).

### 6.3 LLM resolution order (perception)

1. `LLMClientOverride`  
2. Config JSON engine/provider via `NewClientFromConfig` (**wins over ambient ZAI_API_KEY**)  
3. `NewClientFromEnv`  
4. Legacy `NewZAIClient(apiKey)` if non-empty  
5. `missingLLMClient`

Then wrap with tracing if LocalDB opens, then schedule:

- `"main"` — primary  
- `"shards"` — worker if configured, else same raw path  
- `"image_generator"` — Gemini image family only  

Global concurrency: `core.ConfigureGlobalAPIScheduler` from user config policy in stage 1.

### 6.4 Kernel domain ownership (canonical manifest → boot)

`internal/shards/registration.go#DefaultShardPredicateManifests` is the typed
source; `internal/system/factory.go#defaultKernelShardConfigs` converts it into
the production Cortex configs. The uniqueness regression prevents a predicate
from acquiring two owners.

| Domain | Owned predicates |
|--------|------------------|
| routing | user_intent, next_action, routing_result, derived_mode |
| world | file_topology, symbol_graph, diagnostic, project_profile |
| tools | tool_capabilities, shard_lifecycle, shell_exec_result |
| policy | pending_action, permitted_action, permission_check_result, permitted, blocked, constitution, commit_barrier, dangerous_action |
| campaign | campaign, campaign_phase, campaign_task, campaign_dependency |
| prompts | prompt_atom, atom_selection_score, shard_prompt_base |
| cortex | (none listed) |

World facts load preference: LocalDB `LoadAllWorldFacts("fast")` → else `.nerd/mangle/scan.mg`.

### 6.5 VirtualStore construction

- WorkingDir = workspace  
- Kernel attached; boot guard disabled  
- LocalDB + graph adapter + LearningStore  
- Resolve primary RealKernel from direct or Cortex kernel
- Lazily bind Dreamer to that RealKernel; attach DreamRouter + DreamPlanManager
- TransactionManager when RealKernel available
- Modular tools hydrate  
- **HolographicCodeScope** as CodeScope (deepWorkers from world config)  
- FileEditor tactile adapter  
- Later: ShardManager, Ouroboros tools, MCP clients, TaskExecutor  

### 6.6 Intelligence / JIT

- Embedding engine optional health-check  
- AtomLoader  
- MCP bridge from integrations config; async `ConnectAll` with retained
  bridge/cancel/done ownership
- Hybrid prompt ingest  
- AgentSynchronizer  
- Embedded corpus **required** for JIT  
- Project corpus materialize at `.nerd/prompts/corpus.db`  
- JIT compiler with kernel adapter, vector searcher, project DB
- One private cloned RealKernel compilation scope per Compile
- PromptAssembler budgets from JIT config; EnableJIT flag  
- User agents: JIT DB register + ShardManager TypeUser profiles  

### 6.7 Shard management

- LimitsEnforcer from core limits config  
- SpawnQueue started  
- `shards.RegisterAllShardFactories` with RegistryContext (kernel, **shard** LLM, VS, workspace, JIT, optional classification client)  
- Explicit `tactile_router` and `campaign_runner` factories with browser/assembler hooks  
- DisableSystemShards map applied  
- `StartSystemShards` hard-fail  

### 6.8 Final executors

- World Scanner from world config  
- session adapters for kernel / VS / LLM (**task LLM prefers shard worker**)  
- Executor + Spawner + `NewJITExecutor` as TaskExecutor  
- VS task delegator adapter  

### 6.9 Cortex assembly fields

All major public fields assigned from `bootContext`; `cortexKey` left empty unless GetOrBoot sets it afterward.

---

## 7. Deep dive: SpawnTask routing

```
normalizeShardTypeName: trim space, strip leading '/'

if image shard:
    ShardManager.Spawn / SpawnWithPriority using image client
if ShardManager.GetProfile(name).Type == ShardTypeSystem:
    ShardManager.Spawn / SpawnWithPriority
else:
    TaskExecutor.Execute / ExecuteWithContext
```

**Intent verbs are not rewritten** (tests assert `/test` passes through). System lifecycle stays on ShardManager; user/domain work uses session JIT executor path.

---

## 8. Deep dive: HolographicCodeScope

### Problem

`core.VirtualStore` needs deep code facts (`code_defines`, `code_calls`, …) for holographic retrieval policies, but **core cannot import world** (cycle).

### Solution

`system` constructs `HolographicCodeScope` implementing the CodeScope surface:

- Wraps `world.FileScope`  
- On Open/Refresh: `ensureDeepFacts`  
  - With LocalDB: `world.EnsureDeepFacts` → retract old + load new into kernel  
  - Without: in-memory fingerprint cache + `Cartographer.MapFile` for `.go` files  

deepWorkers ≤ 0 → clamp to CPU-based default between 2 and 8.

---

## 9. Deep dive: Agent registry

| Path | Meaning |
|------|---------|
| `.nerd/agents/<id>/prompts.yaml` | Discovery gate |
| `.nerd/shards/<id>_knowledge.db` | Canonical knowledge path |
| `.nerd/agents.json` | Registry mirror (version 1.5.0) |

Sync is **best-effort** (must not block boot). Status:

- `ready` if DB exists (kb_size from atom tables when possible)  
- `missing_db` if knowledge file absent  

Boot additionally uses `prompt/sync.AgentSynchronizer` and registers agents with JIT + shard profiles (richer path than registry JSON alone).

---

## 10. Deep dive: Adapters

### 10.1 Why adapters live here

Composition roots absorb impedance mismatches so domain packages stay cycle-free.

### 10.2 Catalog

| Adapter | From → To | Notes |
|---------|-----------|-------|
| `KernelAdapter` | core.Kernel → prompt querier | Fact conversion; string AssertBatch parse; Retract; private cloned compilation scopes |
| `mcpKernelAdapter` | core.Kernel → mcp | Assert/Query/Retract; Retract trims dots carefully |
| `perceptionLLMAdapter` | perception → mcp LLM | Tool complete optional |
| `sessionKernelAdapter` | kernel → types.Kernel | Full method set |
| `sessionVirtualStoreAdapter` | VS → types.VS | **Read/WriteFile use os fallback** |
| `sessionLLMAdapter` | perception → types LLM | ToolResults + streaming shim |
| `LocalStoreTraceAdapter` | LocalStore → TraceStore | Load stub |
| `taskDelegatorAdapter` | TaskExecutor → VS task iface | Intent/task pack |
| `missingLLMClient` | — | Boot without credentials |

### 10.3 Known adapter debt

- Session file I/O may bypass VirtualStore constitutional routing.  
- Trace load unimplemented.  
- Streaming on session adapter is complete-then-channel, not true stream, unless underlying provides better.

---

## 11. Deep dive: Close lifecycle

```
Close():
  lock; return if already closed
  stopMaintenanceSchedule before LocalDB
  ShardManager.StopSpawnQueue(5s) + StopAll
  cancel/close/bounded-wait MCP
  BrowserManager.Shutdown
  JITCompiler.Close
  closable EmbeddingEngine.Close
  perception.ClosePerceptionLayer when initialized
  LocalDB.Close
  LearningStore.Close
  evictCortexByKey if cortexKey set
  errors.Join
```

Boot rollback reuses this aggregate through `cortexFromBootContext`; an
untransferred project DB is closed first. The forced late-failure regression
proves LocalDB, Learning/JIT DB ownership, and a closable embedding are released.

**Residual:** cleanup is enumerated, not a typed exact-reverse-order acquisition
registry. Caller-owned override semantics and queue/MCP/browser failure
injection are not yet proven. This remains critical for Windows TempDir + SQLite
behavior.

---

## 12. Integration map

### 12.1 Downstream consumers

| Surface | Entry |
|---------|-------|
| Cobra commands under `cmd/nerd` | `GetOrBootCortex` |
| Interactive chat | `BootCortexWithConfig` |
| Tests | Boot* + overrides |

No other `internal/*` package imports `system` (correct composition-root shape).

### 12.2 Upstream fan-out

Boots and wires: core, core/shards, session, perception, prompt, articulation, store, embedding, world, shards (+ system shards), tactile, browser, mcp, autopoiesis, config, logging, usage, sqlpragmas, mangle, types.

### 12.3 Config / disk touchpoints

| Artifact | Role |
|----------|------|
| `.nerd/config.json` | Provider, model, JIT, embedding, MCP, limits, world |
| `.nerd/knowledge.db` | LocalStore |
| `.nerd/shards/` | Learning + agent DBs |
| `.nerd/prompts/corpus.db` | Project JIT corpus |
| `.nerd/agents/**` | User agents |
| `.nerd/agents.json` | Registry |
| `.nerd/mangle/scan.mg` | Cold world facts |
| `.nerd/browser/sessions.json` | Browser store path |

---

## 13. Observability (package)

Categories: Session, Store, Perception, Context, Embedding, Tools, World.  
No metrics registry. Maintenance emits archival counts.  
See [11-OBSERVABILITY.md](11-OBSERVABILITY.md).

---

## 14. Testing (package)

| Strength | Evidence |
|----------|----------|
| Full boot e2e | `TestBootCortexEndToEnd` (skip short) |
| DI + no-LLM | `factory_boot_test.go` |
| Spawn routing | `factory_test.go` |
| Agent registry | large coverage file |
| MCP adapter | unit tests |
| Exact policy ownership and permission | `cortex_permission_routing_test.go` |
| Prompt compile isolation | `prompt_kernel_scope_test.go` |
| Maintenance shutdown | `maintenance_schedule_test.go` |
| Cache identity/retry | `factory_cache_test.go`, `factory_helpers_test.go` |
| Late rollback/idempotence | `factory_rollback_test.go` |
| Canonical predicate ownership | `internal/shards/registration_manifest_test.go` plus policy routing test |

| Weakness | Impact |
|----------|--------|
| No engine/provider-mode identity test | Same provider/model with different engine mode can alias |
| No explicit Close-eviction/Reset contract tests | Long-lived invalidation behavior can drift |
| No all-resource rollback fault injection | Exact reverse order, queue/MCP/browser failure, and caller-owned overrides remain unproven |
| No policy-preserving session file adapter test | Raw OS bypass remains |

Commands: `go test ./internal/system/...` — see [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md).

---

## 15. Failure modes (index)

| ID | Summary |
|----|---------|
| FM1 | Stale identity without reset / dual boot |
| FM2 | Failure poison (**prevented**) |
| FM3 | Kernel evaluate hard-fail |
| FM4 | JIT/corpus hard-fail |
| FM5 | System shards hard-fail |
| FM6 | Missing LLM soft |
| FM7 | Embedding soft |
| FM8 | LocalDB soft |
| FM9 | MCP soft |
| FM10 | Maintenance vs Close race (mitigated) |
| FM11 | CLI+TUI double Cortex |
| FM12 | Reset without Close |
| FM13 | Session file policy bypass |
| FM14 | Hybrid atom partial store |
| FM15 | Image/worker LLM mis-route |
| FM16 | Late boot failure leaks acquisitions (mitigated; registry residual) |
| FM17 | Prompt compilation contaminates executive state (mitigated) |

Details: [12-FAILURE-MODES.md](12-FAILURE-MODES.md).

---

## 16. Invariants (index)

- C1–C5 cache identity and no failure poison  
- F1 missing LLM soft; F2 kernel hard  
- S1 policy domain present on real boot  
- L1 maintenance stops before SQLite; L2 Close bounded/idempotent/enumerated
- S1 exact envelope owner; S2 default deny; S3 action correlation; S4 Dreamer fail closed
- S5 prompt scopes never mutate the live executive
- M1 Retract dot-trimming  
- R1 Close releases SQLite  

Details: [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md).

---

## 17. Gaps pointer

Prioritized work lives in [TODO.md](TODO.md) and [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md). Headline gaps:

1. Cache identity omits separately configured engine/provider mode
2. sessionVirtualStoreAdapter file I/O bypasses VirtualStore
3. Typed reverse-order ownership, override policy, and boot receipt are incomplete
4. Close eviction/Reset behavior needs a pinned API and tests
5. TUI remains direct boot

---

## 18. Principles for changers

When editing this package, obey [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md):

1. Motherboard not actor  
2. Full identity tuple  
3. Never cache failures  
4. Prefer GetOrBoot at edges  
5. Soft periphery / hard core  
6. Role-split scheduled LLMs  
7. Thin adapters  
8. Wiring audit before delete  
9. Close covers new resources  
10. Maintenance policy conscious  

---

## 19. Worked example: `nerd query` path (conceptual)

```
cmd_query.RunE
  → GetOrBootCortex(ctx, workspace, apiKey, disable?)
      → cache miss → BootCortexWithConfig stages 1–9
      → insert cache, start maintenance
  → use cortex.Kernel.Query / cortex.VirtualStore / ...
  → process exit (Cortex often left cached)
```

Chat path differs: BootCortexWithConfig once into session model fields; no cache key; Close on session end is chat’s responsibility.

---

## 20. Non-goals of this package

- Implementing tool handlers or Mangle policy text  
- Owning Cobra trees or TUI widgets  
- Multi-process Cortex sharing  
- Vectryx / product-specific memory substrates  
- Fuzzy NL matching (belongs in retrieval/embedding + structured assert)

---

## 21. Non-goals of this corpus revision

- Implementing any of the TODOs in Go  
- Docs/Spec product template sets  
- Exhaustive line-by-line commentary of every adapter method body  
- Claiming pre-implementation zero status  

---

## 22. Cross-links

| Need | Doc |
|------|-----|
| Vision | [01-VISION.md](01-VISION.md) |
| Alignment scores | [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) |
| File inventory | [02-CURRENT-STATE.md](02-CURRENT-STATE.md) |
| Architecture diagrams | [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) |
| Wiring journal | [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) |
| Dependencies | [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) |
| Core kernel detail | `Docs/architecture/core/` |
| CLI consumers | `Docs/architecture/cli/` |
| Session execution | `Docs/architecture/session/` |
| Prompt JIT | `Docs/architecture/prompt/` |

---

## 23. Verify commands

```powershell
go test ./internal/system/...
go test ./internal/system/ -run TestBootCortexEndToEnd -count=1
rg "GetOrBootCortex|BootCortexWithConfig" -g "*.go" cmd internal/system
```

---

## 24. Change log (corpus)

| Date | Change |
|------|--------|
| 2026-07-13 | Reconciled normalized disabled-shard identity, transactional aggregate rollback, idempotent optional-resource Close, canonical manifest consumption, and focused regressions |
| 2026-07-13 | Full rebuild per `_rebuild/SUBAGENT_INSTRUCTIONS.md`; deep GetOrBootCortex + boot pipeline coverage |

---

*End of IMPLEMENTED_SPEC — living document; re-verify against `internal/system/` when factory stages change.*
