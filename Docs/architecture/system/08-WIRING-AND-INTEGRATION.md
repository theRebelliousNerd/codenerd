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

Boot registers these CortexKernel domains (`initKernel`):

| Domain | Owned predicates (excerpt) |
|--------|----------------------------|
| routing | user_intent, next_action, routing_result, derived_mode |
| world | file_topology, symbol_graph, diagnostic, project_profile |
| tools | tool_capabilities, shard_lifecycle, shell_exec_result |
| policy | permitted, blocked, constitution, commit_barrier, dangerous_action |
| campaign | campaign, campaign_phase, campaign_task, campaign_dependency |
| prompts | prompt_atom, atom_selection_score, shard_prompt_base |
| cortex | (empty ownership set) |

This is how system participates in fact-flow without owning rule text:

```
user_intent (routing) → next_action → VirtualStore.HandleAction
permitted (policy) gates effectful work
```

## 5. VirtualStore wiring checklist

Set during boot:

| Setter | Source |
|--------|--------|
| SetKernel | boot kernel |
| DisableBootGuard | always |
| SetLocalDB / SetGraphQuery | if LocalDB |
| SetLearningStore | if present |
| SetDreamRouter / SetDreamPlanManager | always constructed |
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
| `Cortex.Close` | tests, careful callers | release handles; evict if cached |

**Gap:** auth/config reload commands should call `ResetCortexForWorkspace` then `GetOrBootCortex` to pick up new provider/model.
