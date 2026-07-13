# system — Public API and Types

> Last verified: **2026-07-13**  
> Only **exported** symbols that matter for consumers. Unexported adapters are noted under “internal adapters.”

## 1. Core factory types

### `SystemKernel` — `factory.go`

```go
type SystemKernel interface {
    core.Kernel
    Evaluate() error
    LoadFactsFromFile(path string) error
    ConsumeBootPrompts() []core.HybridPrompt
}
```

Used for hybrid prompt ingest and kernel overrides in tests (`BootConfig.KernelOverride`).

### `BootConfig` — `factory.go`

| Field | Purpose |
|-------|---------|
| `Workspace` | Project root |
| `APIKey` | Legacy Z.AI / GenAI fallback key |
| `DisableSystemShards` | Names to `DisableSystemShard` before start |
| `UserConfigOverride` | Test/TUI inject config |
| `LLMClientOverride` | Test inject LLM |
| `KernelOverride` | Test inject SystemKernel |

### `Cortex` — `factory.go`

Fully initialized system instance. Public fields:

| Field | Type | Role |
|-------|------|------|
| `Kernel` | `core.Kernel` | Executive interface |
| `RealKernel` | `*core.RealKernel` | Concrete kernel when available |
| `LLMClient` | `perception.LLMClient` | Main scheduled client |
| `ShardManager` | `*coreshards.ShardManager` | Profiles + system shards |
| `TaskExecutor` | `session.TaskExecutor` | Non-system task execution |
| `SessionExecutor` | `*session.Executor` | Clean session loop |
| `SessionSpawner` | `*session.Spawner` | Subagent spawn |
| `VirtualStore` | `*core.VirtualStore` | Effectful action router |
| `Executor` | `tactile.Executor` | Direct tactile executor |
| `Transducer` | `perception.Transducer` | NL → facts |
| `Orchestrator` | `*autopoiesis.Orchestrator` | Self-modification orchestration |
| `BrowserManager` | `*browser.SessionManager` | Browser sessions |
| `Scanner` | `*world.Scanner` | World scan |
| `UsageTracker` | `*usage.Tracker` | Usage accounting |
| `LocalDB` | `*store.LocalStore` | Knowledge DB |
| `LearningStore` | `*store.LearningStore` | Shard learning |
| `EmbeddingEngine` | `embedding.EmbeddingEngine` | Vectors (may be nil) |
| `Workspace` | `string` | Resolved root |
| `JITCompiler` | `*prompt.JITPromptCompiler` | Prompt JIT |
| `PromptAssembler` | `*articulation.PromptAssembler` | Runtime assembly |

Unexported: `cortexKey string` for cache eviction.

## 2. Factory functions

| Symbol | File | Contract |
|--------|------|----------|
| `GetOrBootCortex(ctx, workspace, apiKey, disableSystemShards)` | factory.go | Cache get-or-create; starts maintenance |
| `BootCortex(ctx, workspace, apiKey, disableSystemShards)` | factory.go | → BootCortexWithConfig |
| `BootCortexWithConfig(ctx, BootConfig)` | factory.go | Full DI boot |
| `ResetGlobalCortex()` | factory.go | Clear entire cache; does **not** Close |
| `ResetCortexForWorkspace(workspace)` | factory.go | Evict by Workspace path |
| `IngestHybridPrompts(ctx, workspace, kernel, atomLoader)` | factory.go | Hybrid PROMPT → corpus.db |

## 3. Cortex methods

| Method | File | Contract |
|--------|------|----------|
| `SpawnTask(ctx, shardType, task)` | factory.go | System → ShardManager; else TaskExecutor |
| `SpawnTaskWithContext(ctx, shardType, task, sessionCtx, priority)` | factory.go | Same split + priority |
| `StartMaintenanceSchedule(ctx)` | factory.go | 30m LocalDB maintenance; returns cancel |
| `Close()` | cortex_close.go | Stop shards/queue; close JIT/DB/learning; perception; evict cache |

## 4. Agent registry

| Symbol | File | Contract |
|--------|------|----------|
| `AgentOnDisk` | agent_registry.go | `{ID, DBPath}` |
| `DiscoverAgentsOnDisk(workspace)` | agent_registry.go | Scan `.nerd/agents/*/prompts.yaml` |
| `SyncAgentRegistryFromDisk(workspace)` | agent_registry.go | Discover + sync |
| `SyncAgentRegistryFromDiscovered(workspace, discovered)` | agent_registry.go | Upsert `.nerd/agents.json`; returns modified? |

## 5. Kernel adapter (prompt)

| Symbol | File | Contract |
|--------|------|----------|
| `KernelAdapter` | factory_adapters.go | `core.Kernel` → `prompt.KernelQuerier` |
| `NewKernelAdapter(kernel)` | factory_adapters.go | Constructor |
| `(KernelAdapter).Query` | factory_adapters.go | core.Fact → prompt.Fact |
| `(KernelAdapter).AssertBatch` | factory_adapters.go | core.Fact or Mangle string facts |

## 6. Trace adapter

| Symbol | File | Notes |
|--------|------|-------|
| `LocalStoreTraceAdapter` | factory_adapters.go | Implements perception.TraceStore |
| `StoreReasoningTrace` | | Forwards to LocalStore |
| `LoadReasoningTrace` | | **Stub** — returns nil, nil |

## 7. Holographic code scope

| Symbol | File | Contract |
|--------|------|----------|
| `HolographicCodeScope` | holographic_code_scope.go | Implements VirtualStore CodeScope surface |
| `NewHolographicCodeScope(root, kernel, localDB, deepWorkers)` | | deepWorkers ≤0 → CPU-based default [2,8] |
| `Open`, `Refresh`, `Close` | | Scope + deep facts |
| `GetCoreElement`, `GetElementBody`, `GetCoreElementsByFile` | | Forward FileScope |
| `IsInScope`, `ScopeFacts`, `GetActiveFile`, `GetInScopeFiles` | | |
| `VerifyFileHash`, `RefreshWithRetry` | | |

## 8. Internal adapters (not exported; documented for wiring audits)

| Type | Bridges |
|------|---------|
| `missingLLMClient` | Always-error LLM so boot continues |
| `taskDelegatorAdapter` | session.TaskExecutor → VS task interface |
| `perceptionLLMAdapter` | perception → mcp.LLMClient |
| `mcpKernelAdapter` | core.Kernel → mcp Assert/Query/Retract |
| `sessionKernelAdapter` | core/types.Kernel for session |
| `sessionVirtualStoreAdapter` | VS + **os fallback** Read/WriteFile |
| `sessionLLMAdapter` | perception → types.LLMClient incl. ToolResults |

## 9. Consumer cheat-sheet

```go
// Preferred in Cobra handlers:
cortex, err := system.GetOrBootCortex(ctx, workspace, apiKey, disableSystemShards)
defer cortex.Close() // only if process-owned singleton policy allows

// Tests / DI:
cortex, err := system.BootCortexWithConfig(ctx, system.BootConfig{
    Workspace: ws,
    UserConfigOverride: cfg,
    LLMClientOverride: mock,
    KernelOverride: mockKernel,
    DisableSystemShards: []string{"campaign_runner", /* ... */},
})
```
