# system — Wiring and Integration

> Last verified: **2026-07-13**

## 1. Why this package exists

codeNERD’s features are spread across many packages. Without a composition root, every CLI command would re-wire kernel + VS + LLM + JIT differently. `internal/system` is the **canonical wiring journal** enacted as code.

## 2. CLI wiring (GetOrBootCortex)

Pattern repeated across Cobra handlers:

```go
coresys "codenerd/internal/system"
// ...
cortex, err := coresys.GetOrBootCortex(ctx, workspace, key, disableSystemShards)
// use cortex.Kernel, cortex.VirtualStore, cortex.SpawnTask, ...
```

| Command area | File | Notes |
|--------------|------|-------|
| Advanced / multi tools | `cmd/nerd/cmd_advanced.go` | Multiple GetOrBoot sites |
| Knowledge | `cmd_knowledge.go` | |
| Interactive refine | `cmd_interactive.go` | |
| Instruction | `cmd_instruction.go` | May pass disableSystemShards |
| Direct actions | `cmd_direct_actions.go` | |
| Systems dashboard-ish | `cmd_systems.go` | Many call sites |
| Spawn / create | `cmd_spawn.go` | disableSystemShards supported |
| Query | `cmd_query.go` | |
| Transparency | `cmd_transparency.go` | |
| Campaign | `cmd_campaign.go` | |
| DOM | `dom_cmd.go` | |
| Test context | `cmd_test_context.go` | |

**Contract comment in factory.go:**

> IMPORTANT: This function should be used instead of BootCortex() in all command handlers.

## 3. TUI wiring (BootCortexWithConfig)

`cmd/nerd/chat/session_shared_boot.go` → `performSystemBootShared`:

```go
cortex, err := nerdsystem.BootCortexWithConfig(context.Background(), nerdsystem.BootConfig{
    Workspace:           workspace,
    DisableSystemShards: disableSystemShards,
    UserConfigOverride:  appCfg,
})
```

Then the chat layer **unpacks** Cortex fields into local session state (kernel, shardMgr, VS, JIT, etc.) and adds TUI-only layers:

- transparency manager  
- sparse retriever  
- preferences  
- additional learning-store adapters on shard manager  

This path **does not** call `GetOrBootCortex`, so:

- no process cache registration (`cortexKey` empty)  
- no automatic maintenance schedule  
- Close still works for resources but does not evict cache  

## 4. Kernel domain wiring

Boot consumes `shards.DefaultShardPredicateManifests` and registers these
CortexKernel domains (`initKernel`):

| Domain | Owned predicates (excerpt) |
|--------|----------------------------|
| routing | user_intent, next_action, routing_result, derived_mode |
| world | file_topology, symbol_graph, diagnostic, project_profile |
| tools | tool_capabilities, shard_lifecycle, shell_exec_result |
| policy | pending_action, permitted_action, permission_check_result, permitted, blocked, constitution, commit_barrier, dangerous_action |
| campaign | campaign, campaign_phase, campaign_task, campaign_dependency |
| prompts | prompt_atom, atom_selection_score, shard_prompt_base |
| cortex | (empty ownership set) |

This is how system participates in fact-flow without owning rule text:

```
user_intent (routing) → next_action(ActionID, Type, Target, Payload)
pending_action (policy) → permitted(Action, Target, Payload)
→ VirtualStore.RouteAction preserves ActionID → execution_result
```

## 5. VirtualStore wiring checklist

Set during boot:

| Setter | Source |
|--------|--------|
| SetKernel | boot kernel |
| DisableBootGuard | always |
| SetLocalDB / SetGraphQuery | if LocalDB |
| SetLearningStore | if present |
| SetDreamRouter / SetDreamPlanManager | constructed against boot kernel; VirtualStore lazily binds Dreamer to primary RealKernel |
| SetTransactionManager | if RealKernel |
| HydrateModularTools | best-effort |
| SetCodeScope | HolographicCodeScope |
| SetFileEditor | tactile adapter |
| SetShardManager | after intelligence init |
| SetToolGenerator / SetToolExecutor | Ouroboros |
| SetMCPClient(serverID, …) | per integration |
| SetTaskExecutor | taskDelegatorAdapter → JITExecutor |

## 6. Shard registration wiring

```
RegisterAllShardFactories(shardManager, RegistryContext{
  Kernel, LLMClient(shard), VirtualStore, Workspace, JITCompiler, JITConfig,
  ClassificationClient? })
+ explicit tactile_router, campaign_runner factories
+ user agents as TypeUser profiles from disk
+ DisableSystemShards + StartSystemShards
```

## 7. Prompt / JIT wiring

```
AtomLoader(+embedding)
→ AgentSynchronizer.SyncAll
→ LoadEmbeddedCorpus (hard)
→ MaterializeDefaultPromptCorpus(.nerd/prompts/corpus.db)
→ NewJITPromptCompiler(kernel adapter, corpus, vector searcher, project DB)
   → per Compile: KernelAdapter.NewCompilationScope → private RealKernel.Clone
→ PromptAssembler.EnableJIT + budgets from UserConfig
→ transducer.SetPromptAssembler
→ IngestHybridPrompts from kernel.ConsumeBootPrompts
→ RegisterAgentDBWithJIT for each discovered agent
```

## 8. Integration map (package ↔ package)

```
system ──boots──► core.CortexKernel
      ──wires──► core.VirtualStore ◄── session / shards / tools
      ──wires──► perception.Transducer + LLM clients
      ──wires──► prompt.JIT + articulation.PromptAssembler
      ──wires──► session.Executor/Spawner/JITExecutor
      ──wires──► store.LocalStore/LearningStore
      ──wires──► world.Scanner + HolographicCodeScope
      ──wires──► shards.RegisterAll + system shards
      ──wires──► mcp bridge, browser, autopoiesis, usage, embedding
```

## 9. Invalidation / reset wiring

| API | Callers today | Intended use |
|-----|---------------|--------------|
| `ResetGlobalCortex` | tests only (no prod grep hits outside factory) | test isolation |
| `ResetCortexForWorkspace` | none outside factory | after config/provider switch |
| `Cortex.Close` | CLI handlers and tests | cancel maintenance, release covered handles, evict if cached |

**Gaps:** config reload still needs an explicit reset-and-close contract; current
Reset APIs evict without closing. Disabled-shard identity is repaired and
tested; separately configured engine/provider mode remains outside the key.

## 10. Fail-closed authorization wiring

The effect path requires two independent positive conditions:

1. the policy shard derives exact `permitted(Action, Target, Payload)` from the
   pending envelope;
2. mapped destructive actions obtain a usable Dreamer and pass simulation.

`safe_action/1` does not satisfy the first condition. Missing kernel or Dreamer
does not degrade to allow. `internal/system/cortex_permission_routing_test.go#TestDefaultCortexPermissionEnvelopeRoutesToPolicyShard`
guards ownership and exact mismatch denial; core routing tests guard the effect
boundary and correlated result facts.

## 11. Teardown and dormant paths

Normal Close is idempotent and bounded. It stops maintenance, shard admission
and workers, cancels/closes/joins MCP work, shuts down the browser, closes a
closable embedding engine, then closes JIT, LocalDB, LearningStore, initialized
perception, and evicts the cache entry. Stage failures call the same aggregate
path through `rollbackBootContext`; an untransferred project DB is closed first.

**PARTIAL:** this is still an enumerated cleanup path rather than a typed
reverse-order acquisition registry. Caller-owned DI override semantics,
all-resource close-order fault injection, and a cleanup receipt remain open.
Chat's direct-boot path intentionally remains outside the cache and automatic
maintenance lifecycle.
