# Current state: the live Cortex composition root

> **VERIFIED CURRENT** on 2026-07-13 at
> `c8f21b46ec4b28529953094e0c18dac4dfd0c8eb`. The worktree was dirty; the
> final fingerprint and commands are recorded in [_progress.md](_progress.md).

## Scope and inventory

`internal/system` contains five non-test Go files, 19 Go test files, and 61
named tests.

| Production file | Reviewed role |
|---|---|
| `internal/system/factory.go` | Cortex cache, boot stages, kernel/VirtualStore/JIT/shard/session assembly, hybrid prompt ingest, maintenance |
| `internal/system/factory_adapters.go` | trace, prompt-kernel, MCP-kernel, session-kernel, session-VirtualStore, and LLM adapters |
| `internal/system/agent_registry.go` | `.nerd/agents` discovery and `agents.json` mirror |
| `internal/system/holographic_code_scope.go` | `world.FileScope` bridge and deep-fact refresh without a core/world cycle |
| `internal/system/cortex_close.go` | bounded teardown and cache eviction |

`internal/system/debug_program_ERROR.mg` is a 657,945-byte crash artifact, not
owned runtime policy. No `.mg` file in this package is loaded as package-owned
production logic.

## Realized behavior

| Component | Claim | Live evidence |
|---|---|---|
| Keyed factory | **VERIFIED CURRENT** for accepted parameters, **PARTIAL** for engine identity | `getOrBootCortex` hashes workspace/provider/key/model plus the normalized disabled-shard set, never caches errors, and has reuse/split/retry regressions; separate engine/provider mode is not keyed |
| Boot state machine | **VERIFIED CURRENT** | `BootCortexWithConfig` executes nine named `defaultBootSteps`; any stage error calls `rollbackBootContext`; full boot and forced late-failure paths are tested |
| Configuration and LLM roles | **VERIFIED CURRENT** | `initCoreComponents` applies scheduler policy; `initPerceptionLayer` resolves override/config/env/legacy/missing clients and schedules main, shard, and image roles |
| Kernel domains | **VERIFIED CURRENT** | `defaultKernelShardConfigs` consumes `shards.DefaultShardPredicateManifests`; unique ownership and the complete policy envelope have focused regressions |
| Authorization ownership | **VERIFIED CURRENT** | Policy exclusively owns the complete four-predicate envelope; `TestDefaultCortexPermissionEnvelopeRoutesToPolicyShard` proves exact match behavior |
| VirtualStore executive wiring | **VERIFIED CURRENT** | `initExecutionLayer` attaches the CortexKernel, graph/store adapters, DreamRouter, DreamPlanManager, transaction manager, tools, code scope, and file editor |
| Primary RealKernel/Dreamer | **VERIFIED CURRENT** | Cortex obtains `GetPrimaryRealKernel`; `VirtualStore.SetKernel` exposes that RealKernel and `getDreamer` creates the Dreamer lazily outside the store mutex |
| JIT compiler | **VERIFIED CURRENT** | `initIntelligenceLayer` requires embedded atoms, materializes the project DB, applies budgets/vector search, and attaches PromptAssembler |
| Prompt fact isolation | **VERIFIED CURRENT** | `KernelAdapter.NewCompilationScope` clones the live RealKernel; four tests cover concurrent language/retry contexts, budget error, cancellation, and cache-key behavior |
| Agent and shard wiring | **VERIFIED CURRENT** | user agent DBs become JIT sources and TypeUser profiles; all shard factories register before disabled-set application and `StartSystemShards` |
| Task execution | **VERIFIED CURRENT** | `initFinalExecutors` builds Executor, Spawner, JITExecutor, and VirtualStore task delegator; `Cortex.SpawnTask*` keeps system/image shards on ShardManager |
| Maintenance | **VERIFIED CURRENT** | `StartMaintenanceSchedule` waits one full interval, stores cancel/done, and Close stops it before LocalDB; three focused tests pass |
| Close | **VERIFIED CURRENT** for enumerated ownership, **PARTIAL** for typed registry | idempotent Close covers maintenance, shard admission/workers, MCP, browser, closable embedding, JIT, LocalDB, LearningStore, perception, and cache; exact reverse-order metadata and all-resource fault injection remain open |
| Session VirtualStore adapter | **PARTIAL** | `Exec` and `ReadRaw` delegate to VirtualStore; `ReadFile`/`WriteFile` use raw OS access |

## Boot order

The binding order is:

```text
initCoreComponents
  -> initPerceptionLayer
  -> initStorageLayer
  -> initKernel
  -> initExecutionLayer
  -> initAutopoiesisAndBrowser
  -> initIntelligenceLayer
  -> initShardManagement
  -> initFinalExecutors
  -> assemble Cortex

any stage error
  -> rollbackBootContext
  -> close an untransferred project DB
  -> cortexFromBootContext(...).Close()
  -> preserve the primary error; join cleanup errors
```

This order is load-bearing: policy exists before effect routing; VirtualStore
exists before shard factory registration; JIT exists before user-agent DB and
session executor registration; TaskExecutor is attached last.

## Exact authorization and correlation

The boot-owned policy shard contains:

```text
pending_action/5
permitted_action/5
permission_check_result/4
permitted/3
```

`internal/core/virtual_store_routing.go#VirtualStore.parseActionFact` reads the
executive-issued action ID from argument zero. `RouteAction` uses that ID in
`execution_error/2` and `execution_result/6`. Permission compares action type,
target, and canonical JSON payload; `safe_action/1` is classification only.
Mapped destructive actions also need a Dreamer. That is the realized
creative-center/executive split: models propose, policy authorizes, simulation
checks, effectors act, and results remain correlated.

## JIT and agent state

System owns JIT construction, not atom content. The compiler receives:

- the required embedded corpus;
- optional project SQLite corpus and vector search;
- LocalDB and LearningStore;
- user agent databases discovered under `.nerd/agents/<id>/`;
- budgets and enabled state from effective JIT config;
- `KernelAdapter`, whose production compilation scope is a private kernel clone.

The cloned scope discards selector facts on `Close`; the live Cortex remains
free of `compile_context`, `prompt_atom`, vector-hit, and related transient
facts, including concurrent, failure, and cancellation paths.

## State, concurrency, and lifetime

| State | Owner | Scope/lifetime | Current boundary |
|---|---|---|---|
| `cortexCache` | package global | process | RWMutex; global write lock spans boot; disabled-shard set normalized and keyed; engine identity residual |
| kernel shards | CortexKernel | Cortex | registered before Evaluate; policy envelope co-located |
| prompt compilation facts | cloned RealKernel | one Compile call | private scope; dropped on close |
| maintenance goroutine | Cortex | cached Cortex lifetime | cancel/done stored; Close waits up to two seconds |
| spawn queue/system shards | ShardManager | Cortex | started at boot; Close stops admission before workers; rollback uses the same path |
| SQLite resources | JIT/LocalDB/LearningStore | Cortex | ownership transfers explicitly; normal Close and forced late-boot rollback are tested |
| MCP bridge/connect work | bootContext → Cortex | Cortex | cancel/done/bridge retained; Close cancels, closes, then bounded-waits |
| browser / closable embedding | Cortex | Cortex | browser Shutdown and optional `Close() error` embedding release are bounded Close steps |

## Runtime artifacts

| Workspace path | Purpose |
|---|---|
| `.nerd/config.json` | provider, engine, scheduler, JIT, embedding, MCP, limits, world |
| `.nerd/knowledge.db` | LocalStore and traced reasoning data |
| `.nerd/shards/` | LearningStore and agent knowledge DBs |
| `.nerd/prompts/corpus.db` | project JIT corpus |
| `.nerd/agents/**` and `.nerd/agents.json` | user agents and best-effort registry mirror |
| `.nerd/mangle/scan.mg` | world-fact fallback when LocalDB has no fast snapshot |
| `.nerd/browser/sessions.json` | browser session store path |

## Verification receipt

```text
go test -count=1 ./internal/system/...
ok codenerd/internal/system 79.191s

go test -race -count=1 -timeout=180s -run \
  'Test(CortexKey|GetOrBootCortex|BootCortexWithConfigLateFailure|CortexCloseIsIdempotent)' \
  ./internal/system
ok codenerd/internal/system 9.091s
```

This does not claim network integration, every CLI consumer, a whole-package
race run, fuzzing, or campaign behavior. The risk-selected missing gates are in
[10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md).
