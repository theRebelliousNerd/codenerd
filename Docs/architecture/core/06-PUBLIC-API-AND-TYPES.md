# core — Public API and Types

> Last verified: **2026-07-13**  
> Scope: symbols other packages are expected to use. Internal helpers omitted unless critical.

## 1. Type aliases (cycle break)

| Alias | Target | File |
|-------|--------|------|
| `Fact` | `types.Fact` | `kernel_types.go` |
| `MangleAtom` | `types.MangleAtom` | `kernel_types.go` |
| `Kernel` | `types.Kernel` | `kernel_types.go` |
| `LLMClient` | `types.LLMClient` | `llm_client.go` |

Prefer `types.Kernel` in new cross-package APIs; core aliases exist for historical call sites.

## 2. Kernel construction

| Func | File | Notes |
|------|------|-------|
| `NewRealKernel() (*RealKernel, error)` | `kernel_init.go` | Default boot |
| `NewRealKernelWithWorkspace(root string)` | `kernel_init.go` | Stable `.nerd` |
| `NewRealKernelWithPath(manglePath string)` | `kernel_init.go` | Explicit mangle root |
| `NewCortexKernel(cortexDomain string)` | `cortex_kernel.go` | Multi-domain hub |
| `NewKernelShard(config KernelShardConfig)` | `kernel_shard.go` | Domain wrapper |

### RealKernel methods (selected)

| Method | File | Role |
|--------|------|------|
| `SetWorkspace` / `GetWorkspace` | `kernel_init.go` | Path root |
| `SetMaxFacts` / `GetMaxFacts` | `kernel_init.go` | EDB cap |
| `SetDerivedFactLimit` | `kernel_init.go` | IDB cap |
| `LoadFacts` / `LoadFactsSeq` | `kernel_facts.go` | Bulk EDB |
| `Assert` / `AssertBatch` / `AssertWithoutEval` | `kernel_facts.go` | Mutations |
| `Evaluate` | `kernel_facts.go` | Explicit eval |
| `Retract` / `RetractExactFact` / batches | `kernel_facts.go` | Removal |
| `Query` / `QueryCallback` / `QueryAll` | `kernel_query.go` | Read IDB/EDB |
| `SetPolicy` / `AppendPolicy` / `LoadPolicyFile` | `kernel_policy.go` | Policy |
| `HotLoadRule` / `HotLoadLearnedRule` | `kernel_policy.go` | Autopoiesis |
| `Clone` | `kernel_eval.go` | Sandbox snapshot |
| `Clear` / `Reset` | `kernel_eval.go` | Wipe |
| `EnableProvenance` / `Explain` | `kernel_provenance.go` | Why |
| `GetEventBus` | `kernel_init.go` | Pub/sub |
| `SetVirtualStore` / `GetVirtualStore` | `kernel_virtual.go` | VS bind |
| `GetPredicateCorpus` | `kernel_init.go` | Validation corpus |
| `Transaction` | `kernel_transactions.go` | Kernel tx |
| `SetRepairInterceptor` | `kernel_init.go` | Learned repair hook |

### CortexKernel methods (selected)

| Method | Role |
|--------|------|
| `RegisterShard` | Add domain + ownership index |
| `GetShard` / `GetPrimaryRealKernel` | Access |
| `FactRouter` / `SetFactRouter` | Per-shard routing |
| `GetEventBus` | Aggregated events |
| Implements `types.Kernel` | Drop-in for consumers |

## 3. VirtualStore

| Func | File |
|------|------|
| `NewVirtualStore(executor tactile.Executor)` | `virtual_store.go` |
| `NewVirtualStoreWithConfig(executor, VirtualStoreConfig)` | `virtual_store.go` |
| `DefaultVirtualStoreConfig()` | `virtual_store.go` |

### Lifecycle / DI

`SetKernel`, `SetShardManager`, `SetTaskExecutor`, `SetMCPClient`, `SetCodeScope`, `SetFileEditor`, `SetToolExecutor`, `SetToolGenerator`, `SetTransactionManager`, `SetDreamRouter`, `SetDreamPlanManager`, `SetGlassBoxBus`, `SetToolEventBus`, `EnableModernExecutor`, `DisableBootGuard`, `Close`.

### Effect entry points

| Method | Role |
|--------|------|
| `RouteAction(ctx, Fact) (string, error)` | Primary policy-derived path |
| `Exec(ctx, cmd, env)` | Direct shell with limited gates |
| `ReadFile` / `WriteFile` / `ReadRaw` | Convenience FS |
| `QueryPermitted` / `CheckKernelPermitted` | Permission probes |
| `GetDreamer` | Access speculative engine |
| `GetStrategicSummary` | Summary helper |

### Core types (`virtual_store_types.go`)

- `ActionType` — extensive string enum of verbs  
- `ActionRequest` — ActionID, Type, Target, Payload, Timeout, SessionID, RetryCount  
- `ActionResult` — Success, Output, Error, Metadata, FactsToAdd  
- `ConstitutionalRule` — Name, Description, Check  
- `IntegrationClient` — `CallTool(ctx, tool, args)`  
- CodeScope / FileEditor / ToolExecutor / ToolGenerator interfaces (in VS files)

## 4. Dreamer family

| Symbol | File |
|--------|------|
| `NewDreamer(kernel *RealKernel)` | `dreamer.go` |
| `(*Dreamer).SimulateAction` | `dreamer.go` |
| `(*Dreamer).InvalidateCache` | `dreamer.go` |
| `NewDreamRouter(...)` | `dream_router.go` |
| `NewDreamPlanManager(kernel)` | `dream_plan_manager.go` |
| `NewDreamPlan` / `DreamPlan` / `DreamSubtask` | `dream_plan.go` |
| `NewDreamLearningCollector` | `dream_learning.go` |

## 5. Validators

| Symbol | File |
|--------|------|
| `ActionValidator` interface | `action_validator.go` |
| `NewValidatorRegistry` | `action_validator.go` |
| `NewFileWriteValidator` / Edit / Delete | `validator_file.go` |
| `NewExecutionValidator` / Build / Test | `validator_exec.go` |
| `NewSyntaxValidator` / MangleSyntax | `validator_syntax.go` |
| `NewCodeDOMValidator` / LineEdit | `validator_codedom.go` |
| `NewParanoidFileValidator` | `validator_paranoid.go` |
| `NewEnhancedEditValidator` | `validator_edit_enhanced.go` |
| `NewDirectoryValidator` | `validator_dir.go` |

## 6. Scheduling / tools / limits

| Symbol | File |
|--------|------|
| `NewAPIScheduler(config)` | `api_scheduler.go` |
| `NewScheduledLLMCall` / `WithPriority` | `scheduled_llm_client.go` |
| `NewToolRegistry(workDir)` | `tool_registry.go` |
| `NewLimitsEnforcer(cfg)` | `limits.go` |
| `NewShadowMode(parent *RealKernel)` | `shadow_mode.go` |
| `NewTDDLoop` / `WithConfig` | `tdd_loop.go` |
| `NewTransactionManager(kernel, root)` | `transaction_manager.go` |
| `NewSelfHealer(...)` | `self_healing.go` |
| `NewRuleCourt(kernel)` | `rule_court.go` |
| `NewMangleWatcher(workspace, kernel)` | `mangle_watcher.go` |
| `NewFactEventBus` | `fact_event_bus.go` |
| `NewPredicateCorpus` / `FromPath` | `predicate_corpus.go` |
| `NewShardFactRouter` | `shard_fact_router.go` |
| `NewAutopoiesisBridge` | `kernel_utils.go` |

## 7. core/shards public surface

| Symbol | File |
|--------|------|
| `NewShardManager` | `manager.go` |
| `SetParentKernel` / `SetVirtualStore` / `SetLLMClient` / `SetImageLLMClient` | `manager.go` |
| `SetPromptLoader` / JIT register hooks | `manager.go` |
| `SetGlassBoxBus` / `PostSpawnHook` | `manager.go` |
| Spawn APIs | `manager_spawn.go` |
| `NewSpawnQueue` | `spawn_queue.go` |
| `NewBaseShardAgent` / `NewSystemShard` | `agents.go` |

Interfaces: `VirtualStoreConsumer`, `ReviewerFeedbackProvider`.

## 8. Embed helpers

| Symbol | Role |
|--------|------|
| `GetDefaultContent(path string)` | Read embedded defaults relative path |
| `LoadHybridMangleFile` | Hybrid logic/data split (`hybrid_loader.go`) |

## 9. Interfaces consumers implement

| Interface | Expected implementer |
|-----------|----------------------|
| `types.Kernel` | RealKernel, CortexKernel |
| `TaskDelegator` | `session.TaskExecutor` |
| `IntegrationClient` | MCP HTTP clients |
| `LearnedRuleInterceptor` | Mangle repair shard |
| `LearningStoreSaver` / `ColdStoreSaver` | Dream router persistence |
| `ActionExecutor` | Self-healer action apply |
| `CodeScope` / `FileEditor` | world/tactile adapters |

## 10. What not to call from outside casually

| Symbol | Reason |
|--------|--------|
| `evaluate` (unexported) | Use `Evaluate` / Query ensure path |
| `rebuildProgram` | Internal |
| `simulateCommitErr` | Test-only field on RealKernel |
| Direct mutation of `facts` slice | Breaks indexes/diff |

## 11. Error / dump conventions

- Kernel boot/eval analysis failure → error + optional `debug_program_ERROR.mg`  
- RouteAction denials → `error` return + `security_violation` facts  
- Dreamer blocks → error text includes “dreamer safety gate”
