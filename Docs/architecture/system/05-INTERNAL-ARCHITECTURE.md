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
  ├─ key = SHA256(ws \0 provider \0 apiKey \0 model)
  │
  ├─ RLock → cache hit? return existing
  │
  ├─ Lock (write)  // serializes ALL first-boots
  │    ├─ re-check cache
  │    ├─ BootCortex(...)
  │    ├─ error? return (no cache write)
  │    ├─ cortex.cortexKey = key
  │    ├─ cortexCache[key] = cortex
  │    └─ StartMaintenanceSchedule(Background)  // cancel discarded
  └─ return cortex
```

### Cache helpers

| Func | Behavior |
|------|----------|
| `cortexKey` | SHA-256 hex of identity string |
| `resolveWorkspaceRoot` | arg → FindWorkspaceRoot → cwd |
| `resolveProviderModelForKey` | LoadUserConfig; empty on error |
| `ResetGlobalCortex` | replace map (no Close) |
| `ResetCortexForWorkspace` | delete entries matching Workspace |
| `evictCortexByKey` | used by Close |

## 3. Boot stage machine

`BootCortexWithConfig` builds a private `bootContext` and runs stages **in order**:

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
- Register domains: routing, world, tools, policy, campaign, prompts, cortex  
- `cortex.Evaluate()` — **hard fail** on error  
- `perception.InitPerceptionLayer`  
- Load world facts: LocalDB cache `"fast"` else `.nerd/mangle/scan.mg`

#### 5. `initExecutionLayer`

- `tactile.NewDirectExecutor`  
- `core.NewVirtualStoreWithConfig` + SetKernel, DisableBootGuard  
- Wire LocalDB, graph adapter, LearningStore  
- DreamRouter + DreamPlanManager  
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
- MCP integration bridge (async ConnectAll)  
- IngestHybridPrompts  
- AgentSynchronizer.SyncAll  
- Load embedded prompt corpus (**hard fail** if missing)  
- Materialize + open project corpus.db  
- NewJITPromptCompiler  
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
└── cortexKey (cache only)
```

## 5. SpawnTask routing

```
SpawnTask(shardType, task)
  normalize name (trim, strip leading /)
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
