# system — Internal Architecture

> Last verified: **2026-07-13**

## 1. Component map

```
┌──────────────────────────────────────────────────────────────────┐
│                        package system                            │
│                                                                  │
│  cortexCache ── GetOrBootCortex ──► BootCortex ──► BootCortex…Cfg│
│       ▲                                    │                     │
│       │ cortexKey                          ▼                     │
│  Cortex.Close ◄──────────────────── bootContext stages           │
│                                                                  │
│  agent_registry │ holographic_code_scope │ factory_adapters      │
└──────────────────────────────────────────────────────────────────┘
```

## 2. GetOrBootCortex control flow

```
GetOrBootCortex(ctx, workspace, apiKey, disableSystemShards)
  │
  ├─ resolveWorkspaceRoot(workspace)
  ├─ resolveProviderModelForKey(ws)  // best-effort from .nerd/config.json
  ├─ disabled = normalizeDisableSystemShards(input)
  ├─ key = SHA256(length-delimited ws/provider/apiKey/model/disabled...)
  │
  ├─ RLock → cache hit? return existing
  │
  ├─ Lock (write)  // serializes ALL first-boots
  │    ├─ re-check cache
  │    ├─ BootCortex(...)
  │    ├─ error? return (no cache write)
  │    ├─ cortex.cortexKey = key
  │    ├─ cortexCache[key] = cortex
  │    └─ StartMaintenanceSchedule(Background)  // cancel/done stored on Cortex
  └─ return cortex
```

### Cache helpers

| Func | Behavior |
|------|----------|
| `normalizeDisableSystemShards` | trim, discard empty, deduplicate, sort |
| `cortexKey` | SHA-256 hex of length-delimited identity components |
| `resolveWorkspaceRoot` | arg → FindWorkspaceRoot → cwd |
| `resolveProviderModelForKey` | LoadUserConfig; empty on error |
| `ResetGlobalCortex` | replace map (no Close) |
| `ResetCortexForWorkspace` | delete entries matching Workspace |
| `evictCortexByKey` | used by Close |

**VERIFIED CURRENT:** equal normalized disabled sets reuse one Cortex and
different sets split. **PARTIAL:** engine/provider mode remains outside the key.

## 3. Boot stage machine

`BootCortexWithConfig` builds a private `bootContext` and runs named
`defaultBootSteps` **in order** through `bootCortexWithSteps`:

```
1. initCoreComponents
2. initPerceptionLayer
3. initStorageLayer
4. initKernel
5. initExecutionLayer
6. initAutopoiesisAndBrowser
7. initIntelligenceLayer
8. initShardManagement
9. initFinalExecutors
→ assemble Cortex struct
```

### Stage details

#### 1. `initCoreComponents`

- Resolve workspace; set `perception.SharedTaxonomy` workspace  
- `logging.Initialize`  
- `usage.NewTracker`  
- Load `UserConfig` (or override / DefaultUserConfig)  
- Effective JIT config  
- Configure **global API scheduler** from `GetEffectiveAPISchedulerPolicy`

#### 2. `initPerceptionLayer`

LLM resolution order:

1. `LLMClientOverride`  
2. `perception.LoadConfigJSON` + `NewClientFromConfig` (engine wins over ambient keys)  
3. `NewClientFromEnv`  
4. Legacy `NewZAIClient(apiKey)` if apiKey non-empty  
5. Else `missingLLMClient` (boot continues)

Then:

- Open `.nerd/knowledge.db` → optional `TracingLLMClient`  
- Schedule main client as `"main"`  
- Optional worker LLM for shards (`"shards"`)  
- Optional image LLM (`"image_generator"`)  
- Taxonomy client/store hydrate  
- `NewUnderstandingTransducer(main)`

#### 3. `initStorageLayer`

- `store.NewLearningStore(.nerd/shards)`

#### 4. `initKernel`

- Override or `core.NewCortexKernel("cortex")`  
- Convert `shards.DefaultShardPredicateManifests` into KernelShardConfig for
  routing, world, tools, policy, campaign, prompts, and cortex
- Co-locate `pending_action`, `permitted_action`,
  `permission_check_result`, and `permitted` in the policy shard
- `cortex.Evaluate()` — **hard fail** on error  
- `perception.InitPerceptionLayer`  
- Load world facts: LocalDB cache `"fast"` else `.nerd/mangle/scan.mg`

#### 5. `initExecutionLayer`

- `tactile.NewDirectExecutor`  
- `core.NewVirtualStoreWithConfig` + SetKernel, DisableBootGuard  
- Wire LocalDB, graph adapter, LearningStore  
- Resolve the primary RealKernel from RealKernel or CortexKernel
- Lazily create the Dreamer on that RealKernel, then attach DreamRouter and DreamPlanManager
- TransactionManager when RealKernel available
- HydrateModularTools  
- **HolographicCodeScope** as CodeScope  
- FileEditor adapter

#### 6. `initAutopoiesisAndBrowser`

- `autopoiesis.NewOrchestrator` + AutopoiesisBridge  
- Ouroboros as tool generator/executor on VirtualStore  
- Browser SessionManager (needs mangle engine)

#### 7. `initIntelligenceLayer`

- Embedding engine (+ health check)  
- AtomLoader  
- Reflect embedding into LocalDB/LearningStore  
- MCP integration bridge; retain bridge plus connect cancel/done on bootContext
- IngestHybridPrompts  
- AgentSynchronizer.SyncAll  
- Load embedded prompt corpus (**hard fail** if missing)  
- Materialize + open project corpus.db  
- NewJITPromptCompiler with `KernelAdapter`; every production Compile obtains a
  private RealKernel clone through `NewCompilationScope`
- PromptAssembler + wire transducer (+ poiesis if present)  
- Sync agents.json from discovered  
- NewShardManager; register discovered user agents as TypeUser profiles  
- SetShardManager on VirtualStore

#### 8. `initShardManagement`

- LimitsEnforcer + SpawnQueue  
- `shards.RegisterAllShardFactories`  
- JIT DB registrar/unregistrar  
- Explicit factories: `tactile_router`, `campaign_runner`  
- Apply `DisableSystemShards`  
- `StartSystemShards` — **hard fail**

#### 9. `initFinalExecutors`

- `world.NewScannerWithConfig`  
- session adapters (kernel, VS, LLM=task/shard client)  
- `session.NewExecutor` + persister  
- `session.NewSpawner`  
- `session.NewJITExecutor` as TaskExecutor  
- VirtualStore.SetTaskExecutor(taskDelegatorAdapter)

## 4. Cortex post-boot structure

```
Cortex
├── Kernel / RealKernel
├── LLMClient (main scheduled)
├── ShardManager / TaskExecutor
├── SessionExecutor / SessionSpawner
├── VirtualStore / Executor (tactile)
├── Transducer
├── Orchestrator (autopoiesis)
├── BrowserManager
├── Scanner
├── UsageTracker
├── LocalDB / LearningStore
├── EmbeddingEngine
├── Workspace
├── JITCompiler / PromptAssembler
├── cortexKey (cache only)
└── private lifecycle: MCP bridge/cancel/done, perception flag, close mutex/state
```

## 5. SpawnTask routing

```
SpawnTask(shardType, task)
  normalize name (trim, strip leading /)
  if image shard
      → ShardManager.Spawn using dedicated image client
  if ShardManager has profile with Type == System
      → ShardManager.Spawn
  else
      → TaskExecutor.Execute(TaskRequest{IntentVerb, Task})
```

Same pattern for `SpawnTaskWithContext` with priority/session context on system path.

## 6. HolographicCodeScope flow

```
Open/Refresh path(s)
  → world.FileScope
  → ensureDeepFacts(paths)
       if LocalDB: world.EnsureDeepFacts → RetractExact + LoadFacts
       else: in-memory fingerprint cache + Cartographer.MapFile for .go
```

## 7. Agent registry flow

```
.nerd/agents/<id>/prompts.yaml  →  DiscoverAgentsOnDisk
                                    DB: .nerd/shards/<id>_knowledge.db
SyncAgentRegistryFromDiscovered → .nerd/agents.json
Boot also: AgentSynchronizer + JIT RegisterAgentDB + DefineProfile
```

## 8. State machines (package-local)

### Cortex cache entry lifecycle

```
absent → (successful boot) present → (Close or Reset*) absent
              │
              └── failed boot stays absent
```

No “failed” entry state exists by design.

### Boot acquisition lifecycle

```text
stage 1 acquisition -> stage 2 acquisition -> ... -> assembled Cortex -> Close
                               |
                               +-- stage error -> rollbackBootContext
                                      |-- close untransferred project DB
                                      |-- cortexFromBootContext(...).Close
                                      `-- primary error + joined cleanup errors
```

The error edge is **VERIFIED CURRENT** for the forced late-failure slice.
`Cortex.Close` is idempotent and owns maintenance, shard admission/workers, MCP,
browser, closable embeddings, JIT, LocalDB, LearningStore, and initialized
perception. The residual is architectural: cleanup is an enumerated aggregate,
not a typed registry that proves exact reverse acquisition order or caller-owned
override semantics.

### Prompt compilation lifecycle

```text
live CortexKernel
  -> GetPrimaryRealKernel
  -> Clone per Compile
  -> assert/query selector facts in clone
  -> close scope by dropping clone
  -> live executive unchanged
```

Concurrent compiles therefore do not share transient selector facts. Error and
cancellation paths discard the same private scope.
