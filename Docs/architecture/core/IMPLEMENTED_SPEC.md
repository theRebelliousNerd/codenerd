# core — Implemented Spec (Deep-Dive)

> Last verified against codebase: **2026-07-13**  
> Status: Living Reference Document  
> Language: Go + embedded Mangle  
> Primary sources: `internal/core/`, `internal/core/defaults/`, `internal/core/shards/`  
> Scale (approx): **~78** non-test Go files; **~100+** test files; **~130** `.mg` sources under `defaults/`  
> Philosophy: *Logic determines Reality; the Model merely describes it.*

---

## 1. Overview

`internal/core` is the **executive runtime** of codeNERD. It owns:

1. **`RealKernel`** — EDB/IDB separation over the Codeberg `mangle-go` engine; embeds constitution (schemas + stratified policy + learned rules); lazy/differential evaluation; fact bus.
2. **`VirtualStore`** — FFI / effect gateway: `next_action` → tactile shell/files, MCP, CodeDOM, tools, campaigns, python env; multi-layer safety gates.
3. **`Dreamer`** — speculative pre-execution safety: clone kernel, project effects, query `panic_state` (fail-closed).
4. **`CortexKernel` / `KernelShard`** — optional hierarchical multi-domain kernel federation with predicate ownership routing.
5. **Embedded Mangle corpus** — `//go:embed defaults/*.mg defaults/schema/*.mg defaults/policy/*.mg` (schemas, constitution, dreamer, campaign, JIT, coder, …).
6. **Shard manager plumbing** — `internal/core/shards` still provides spawn/queue/agent base used by boot and VirtualStore, while primary OODA execution has moved toward `internal/session`.
7. **Supporting executive systems** — API scheduler, scheduled LLM client, validators, shadow mode, TDD loop, tool registry, transactions, rule court, self-healing, limits, provenance.

### Fact-flow (always)

```
user input
  → perception (user_intent facts)
  → RealKernel / CortexKernel assert + evaluate
  → IDB derives next_action / permitted / persona / …
  → VirtualStore.RouteAction (boot guard → Dreamer → constitution → permitted → handler)
  → result facts injected (execution_result, …)
  → articulation / session / TUI
```

### High-level component map

```
┌──────────────────────────────────────────────────────────────────┐
│                         internal/core                            │
│  ┌─────────────┐   ┌──────────────┐   ┌─────────────────────┐   │
│  │ RealKernel  │◄──┤ CortexKernel │   │ VirtualStore        │   │
│  │ + embed.FS  │   │ + KernelShard│──►│ RouteAction/handlers│   │
│  │ EDB facts   │   └──────────────┘   │ + Dreamer gate      │   │
│  │ programInfo │          ▲           │ + validators        │   │
│  └──────┬──────┘          │           └──────────┬──────────┘   │
│         │            ShardFactRouter             │              │
│         ▼                                        ▼              │
│  mangle-go engine                         tactile / MCP / tools │
│  stratified eval                          transaction manager   │
│  provenance (opt)                         shards.ShardManager   │
└──────────────────────────────────────────────────────────────────┘
```

---

## 2. Implementation status

| Component | Status | Evidence |
|-----------|--------|----------|
| `RealKernel` boot + evaluate | **Implemented** | `kernel_init.go`, `kernel_eval.go` |
| Modular schemas + policy embed | **Implemented** | `//go:embed` + `loadMangleFiles` |
| Fact assert/retract/query | **Implemented** | `kernel_facts.go`, `kernel_query.go` |
| Lazy eval + optional diff eval | **Implemented** | `factsDirty`, `CODENERD_DIFF_EVAL` / features |
| Provenance / Explain | **Implemented** | `kernel_provenance.go` (off by default) |
| VirtualStore action routing | **Implemented** | `virtual_store_routing.go`, handlers |
| Constitutional Go rules | **Implemented** | `virtual_store_constitution.go` |
| Mangle `permitted(...)` gate | **Implemented** | `CheckKernelPermitted`, `constitution.mg` |
| Dreamer safety gate | **Implemented** | `dreamer.go`, `policy/dreamer.mg` |
| Post-action validators | **Implemented** | `action_validator.go`, `validator_*.go` |
| Boot guard | **Implemented** | `bootGuardActive` until first user interaction |
| CortexKernel multi-domain | **Implemented** (feature-flagged per-shard facts) | `cortex_kernel.go` |
| ShardManager plumbing | **Implemented** (partial deprecation vs session) | `shards/manager.go` |
| APIScheduler / ScheduledLLM | **Implemented** | `api_scheduler.go`, `scheduled_llm_client.go` |
| Shadow mode | **Implemented** | `shadow_mode.go`, `policy/shadow_mode.mg` |
| TDD loop | **Implemented** | `tdd_loop.go`, `policy/tdd_*.mg` |
| Tool registry | **Implemented** | `tool_registry.go` |
| Transaction manager (2PC shadow) | **Implemented** | `transaction_manager.go` |
| Predicate corpus validation | **Implemented** | `predicate_corpus.go`, baked DB |
| Hybrid `.mg` loader | **Implemented** | `hybrid_loader.go` |
| HotLoadRule + sandbox | **Implemented** | `kernel_policy.go` |
| Full domain-shard isolation | **Partial / evolved** | session executor preferred for agent loop |
| Diff eval production default | **Partial / cautioned** | env/feature flag; known caveats in comments |

**Overall:** living production executive — **not** pre-implementation.

---

## 3. Source inventory

### 3.1 Package layout

```
internal/core/
  kernel.go                 # package marker / modularization map
  kernel_types.go           # RealKernel struct, Fact aliases, embed.FS
  kernel_init.go            # NewRealKernel*, loadMangleFiles
  kernel_facts.go           # Assert/Retract/LoadFacts, limits
  kernel_query.go           # Query / QueryAll / callbacks
  kernel_eval.go            # rebuildProgram, evaluate, Clone, diff eval
  kernel_policy.go          # Set/Append/Load policy, HotLoadRule
  kernel_validation.go      # learned rule validation / self-heal
  kernel_provenance.go      # Explain / proof recorder
  kernel_transactions.go    # KernelTransaction
  kernel_virtual.go         # kernel ↔ VirtualStore binding
  kernel_shard.go           # KernelShard domain wrapper
  cortex_kernel.go          # hierarchical kernel hub
  virtual_store*.go         # FFI router + handlers
  dreamer.go + dream_*.go   # speculative safety + plans/learning
  action_validator*.go      # post-action validation
  validator_*.go            # file/exec/syntax/codedom validators
  api_scheduler.go          # concurrent LLM slot scheduling
  scheduled_llm_client.go   # LLM client wrapped by scheduler
  shards/                   # ShardManager, spawn queue, base agents
  defaults/                 # embedded schemas, policy, corpora
```

### 3.2 Largest non-test sources (approximate lines)

| Path | ~Lines | Role |
|------|-------:|------|
| `kernel_facts.go` | 1255 | EDB mutations, batch ops, scrub, schema load helpers |
| `virtual_store.go` | 1077 | VS struct, DI, inject facts, permitted cache, boot guard |
| `virtual_store_workflows.go` | 1029 | multi-step / workflow action handlers |
| `virtual_store_actions.go` | 1008 | exec/tests/git/delegate/research handlers |
| `virtual_store_predicates.go` | 993 | virtual predicate GetFacts surface |
| `api_scheduler.go` | 871 | shard-phase aware API concurrency |
| `scheduled_llm_client.go` | 854 | scheduled LLM calls |
| `tdd_loop.go` | 833 | TDD state machine over VS + kernel |
| `dreamer.go` | 753 | SimulateAction, projectEffects, panic_state |
| `cortex_kernel.go` | 731 | multi-domain routing |
| `kernel_eval.go` | 730 | program rebuild + stratified eval |
| `virtual_store_codedom.go` | 706 | CodeDOM / element edit handlers |
| `predicate_corpus.go` | 636 | baked predicate metadata |
| `shards/manager_spawn.go` | 622 | spawn lifecycle |
| `kernel_query.go` | 595 | query paths |
| `kernel_init.go` | 591 | boot + mangle load order |
| `dream_learning.go` | 585 | dream consultation learnings |
| `tool_registry.go` | 582 | static + dynamic tools |
| `shards/spawn_queue.go` | 581 | spawn backpressure |

### 3.3 Critical types

| Type | File | Role |
|------|------|------|
| `RealKernel` | `kernel_types.go` | Primary Mangle executive |
| `Fact` | alias → `types.Fact` | EDB atom |
| `Kernel` | alias → `types.Kernel` | Interface for consumers |
| `VirtualStore` | `virtual_store.go` | Effect gateway |
| `ActionRequest` / `ActionResult` | `virtual_store_types.go` | Action envelope |
| `ActionType` | `virtual_store_types.go` | Large enum of verbs |
| `Dreamer` / `DreamResult` | `dreamer.go` | Speculative safety |
| `CortexKernel` | `cortex_kernel.go` | Domain federation |
| `KernelShard` | `kernel_shard.go` | Domain-owned kernel |
| `ShardManager` | `shards/manager.go` | Agent lifecycle plumbing |
| `APIScheduler` | `api_scheduler.go` | LLM concurrency |
| `ValidatorRegistry` | `action_validator.go` | Post-action checks |
| `TransactionManager` | `transaction_manager.go` | Multi-file shadow commits |
| `ShadowMode` | `shadow_mode.go` | Simulation of action sets |
| `TDDLoop` | `tdd_loop.go` | Repair loop |
| `ToolRegistry` | `tool_registry.go` | Tool defs for agents |
| `FactEventBus` | `fact_event_bus.go` | Pub/sub on fact mutations |
| `PredicateCorpus` | `predicate_corpus.go` | Schema knowledge DB |

---

## 4. RealKernel deep dive

### 4.1 Construction

| Constructor | Behavior |
|-------------|----------|
| `NewRealKernel()` | Default: load embed + CWD-relative `.nerd` |
| `NewRealKernelWithWorkspace(root)` | Stable `.nerd` resolution even if CWD differs |
| `NewRealKernelWithPath(manglePath)` | Explicit mangle search root |

Boot sequence (`kernel_init.go`):

1. Allocate EDB slices, `factstore.NewSimpleInMemoryStore()`, `FactEventBus`.
2. `loadMangleFiles()` — concatenate schemas, policy modules, optional core modules, learned.
3. Leave the baked predicate corpus unloaded; `GetPredicateCorpus` initializes it
   once on first use so ordinary kernel construction does not open an unused
   SQLite-backed corpus.
4. Inject **non-ephemeral** boot facts from hybrid files (quiescent boot).
5. Force `evaluate()` — **hard fail** if embedded constitution does not compile.

### 4.2 Program composition (stratified trust)

`rebuildProgram()` (`kernel_eval.go`) concatenates, in order:

1. **schemas** (Decl surface — physics)
2. **policy** (rules — executive + constitution)
3. **learned** (autopoiesis layer — *after* constitution)

Then: parse → `analysis.AnalyzeOneUnit` → `Stratify` → cache `programInfo`, `strata`, `predToStratum`.

On analysis failure: dump combined source to `debug_program_ERROR.mg` in CWD for debugging.

### 4.3 Evaluation modes

| Mode | When | Notes |
|------|------|-------|
| Full stratified eval | Default / policyDirty / retract / provenance on | Rebuild store from EDB atoms |
| Lazy eval | `factsDirty` atomic | Query path calls `ensureEvaluated()` |
| Differential eval | Feature / env gated | `diffEngine.ApplyDelta` for assert-only deltas; invalidated on retract/clear/policy change |

Default EDB cap: **250_000** facts (`defaultMaxFacts`). The effective derived
fact limit defaults to **500_000** in both full and differential evaluation;
zero configuration does not create divergent ceilings.

### 4.4 Fact API (behavioral)

| Method | Semantics |
|--------|-----------|
| `LoadFacts` | Bulk EDB add + eager evaluate (boot path) |
| `Assert` | Add if new; mark dirty / stratum dirty; may eval |
| `AssertWithoutEval` | Stage fact; caller must `Evaluate()` |
| `AssertBatch` | Batch insert |
| `Retract(predicate)` | Remove all facts with predicate; force full rebuild path |
| `RetractExactFact` / batch | Precise removal |
| `Query` / `QueryCallback` / `QueryAll` | Ensure evaluated then scan store |
| `Clone` | Deep-ish snapshot for Dreamer sandboxes |
| `HotLoadRule` | Sandbox-validate then append to learned |
| `EnableProvenance` / `Explain` | Optional derivation proofs |

Dedup: `factIndex` canonical string keys. Atom cache avoids repeated `ToAtom()` conversions.

### 4.5 Ephemeral vs persistent

Boot filters ephemeral predicates (`filterBootFacts` / `IsEphemeral`) so session rehydration cannot re-fire `user_intent` / pending actions before a real user turn.

### 4.6 Event bus

`GetEventBus()` exposes `FactEventBus` for system shards / subscribers that previously polled the EDB.

---

## 5. VirtualStore deep dive

### 5.1 Role

`VirtualStore` is the **hollow-kernel FFI router**: logic decides `next_action`; VS performs the side effect and writes result facts back.

Dependencies injected at boot/runtime:

- `tactile.Executor` (+ modern audited executor)
- `Kernel` (+ lazy Dreamer from `*RealKernel` or Cortex primary real kernel)
- MCP `IntegrationClient` map
- `TaskDelegator` (session executor — avoids import cycle)
- CodeScope / FileEditor / GraphQuery
- ToolExecutor / ToolGenerator / `tools.Registry`
- `store.LocalStore` / `LearningStore`
- GlassBox + Tool event buses
- `TransactionManager`, validators, constitution rules

### 5.2 `RouteAction` pipeline

`virtual_store_routing.go`:

```
RouteAction(fact)
  1. bootGuardActive? → deny (session rehydration safety)
  2. parseActionFact → ActionRequest
  3. if destructive → require Dreamer → SimulateAction (fail-closed)
  4. checkConstitution (Go hard rules)
  5. CheckKernelPermitted (exact Mangle permitted/3; cache classifies only)
  6. executeAction → type-specific handler
  7. post-action validators (confidence ≥ 0.8 returns validation failure)
  8. inject execution_result + FactsToAdd
  9. audit + Tool/GlassBox events
```

Destructive types include shell, file mutations, line/DOM edits, git, campaign file ops, python env exec/patch (`isDestructiveAction`).

### 5.3 Action type surface

`virtual_store_types.go` defines a large `ActionType` vocabulary, including:

- Filesystem: `read_file`, `write_file`, `edit_file`, `delete_file`, `fs_read`, `fs_write`, `glob`, `grep`, …
- Shell: `exec_cmd`, `run_command`, `bash`, `run_build`, …
- CodeDOM: `open_file`, `edit_element`, `edit_lines`, …
- Delegation: `delegate`, `delegate_coder`, …
- Campaign: `campaign_*`, archive/status/checkpoint
- Ouroboros: `generate_tool`, `ouroboros_*`
- Python / SWE-bench env actions
- Research: `context7_fetch`, `web_search`, browser_* 

Handlers live across `virtual_store_actions.go`, `_file_actions.go`, `_codedom.go`, `_workflows.go`, `_python.go`, `_mcp_proxy.go`, etc.

### 5.4 Permission model (defense in depth)

| Layer | Location | Behavior |
|-------|----------|----------|
| Boot guard | VS | No routing until `DisableBootGuard()` |
| Dreamer | VS + `dreamer.go` | Speculative `panic_state` |
| Go constitution | `virtual_store_constitution.go` | Destructive cmd, secret exfil, traversal, system paths |
| Mangle constitution | `defaults/policy/constitution.mg` | `permitted` default-deny |
| Binary allowlist | VS config | `bash`, `go`, `git`, … |
| Env allowlist | VS config | PATH/HOME/GOPATH/…; caller env filtered |
| Post validators | `validator_*.go` | Success verification |

`CheckKernelPermitted` canonicalizes payload JSON and accepts only an exact
`permitted(ActionType, Target, Payload)` derivation. The `safe_action` cache is a
classification hint only and cannot return allow.

### 5.5 Result feedback

Successful/failed actions inject:

- handler-specific `FactsToAdd`
- `execution_result(ActionID, Type, Target, Success, Output, UnixTime)`
- on deny: `security_violation(ActionType, Reason, UnixTime)`, sometimes `dream_blocked_action`
- on exec fail: `execution_error(RequestID, ErrorMessage)`

Action log pruning (`maybePruneActionLogs`) reads the timestamp from slot 5 of
`execution_result/6`; output text is never parsed as time.

---

## 6. Dreamer deep dive

### 6.1 Components

| Piece | File | Role |
|-------|------|------|
| `Dreamer` | `dreamer.go` | SimulateAction entry |
| `DreamCache` | `dreamer.go` | Cache verdicts (max 256) |
| `DreamRouter` | `dream_router.go` | Persist confirmed learnings |
| `DreamPlanManager` | `dream_plan_manager.go` | Multi-step dream plans |
| `DreamLearningCollector` | `dream_learning.go` | Extract consultation learnings |

### 6.2 Simulation algorithm

1. Validate target length (≤4096) and non-empty type (else unsafe).
2. Cache hit → return cached unsafe/safe verdict.
3. `projectEffects` builds sandbox-only `hypothetical("type:target")`,
   `projected_action`, and type-specific `projected_fact` values (for example
   `/file_missing` and `/critical_path_hit`) without mutating the live kernel.
4. Clone the kernel; stage every projection through the checked insertion path;
   reject the simulation if capacity or Mangle conversion drops any fact.
5. Evaluate the complete sandbox projection.
6. Require `panic_state` Decl present (else fail-closed).
7. Query `panic_state`; match action ID → unsafe + reason.
8. Cache verdict.

Critical path prefixes asserted into kernel: `.git`, `.nerd`, `internal/mangle`, `internal/core`, `cmd/nerd`.

Policy (`defaults/policy/dreamer.mg`) defines `panic_state` for critical files, dangerous exec, tested-symbol deletion, critical path hits.

### 6.3 Fail-closed invariants

- Nil kernel → unsafe  
- Nil/canceled context → unsafe  
- Eval/query failure → unsafe  
- Projected-fact capacity or encoding failure → unsafe
- Missing `panic_state` Decl → unsafe  
- Missing Dreamer on a destructive `RouteAction` or interactive preflight → deny

---

## 7. CortexKernel & shards

### 7.1 CortexKernel

Federates `KernelShard` instances:

- Mutations/queries routed by `predicateOwner` map.
- Unowned predicates → `cortexDomain` catch-all.
- Optional `ShardFactRouter` when `features.IsPerShardFactsEnabled()`.
- Aggregated `FactEventBus`.

Implements `types.Kernel` / transaction interfaces for drop-in use.

The default system factory co-locates `pending_action`, `permitted_action`,
`permission_check_result`, `permitted`, `blocked`, `constitution`,
`commit_barrier`, and `dangerous_action` on the policy shard. This is required for
the permission join. System routing preserves the executive `actionID` through
the result fact.

### 7.2 KernelShard

Wraps a `RealKernel` with domain name + owned predicate set; supports router forwarding so Assert lands in authoritative store.

### 7.3 `internal/core/shards.ShardManager`

Still live for:

- Register factories / profiles
- Spawn agents with kernel/LLM/VS injection
- SpawnQueue backpressure + limits enforcer
- Image-shard special LLM client
- JIT DB register/unregister hooks
- Glass Box lifecycle events
- PostSpawnHook for chat-layer DI without import cycles

**Architectural note:** Package README documents migration of *orchestration* toward `internal/session` Executor/Spawner; domain agent loop is no longer a 12k-line core ShardManager monofile. The `shards` package remains **plumbing**, not the sole OODA brain.

---

## 8. Embedded Mangle corpus

### 8.1 Schemas (Decl physics)

Index: `defaults/schemas.mg` (modularization map).

Loaded modules (from `loadMangleFiles`):  
`schemas_intent`, `_world`, `_execution`, `_browser`, `_project`, `_dreamer`, `_memory`, `_knowledge`, `_learning`, `_state`, `chaos`, `_safety`, `_analysis`, `_misc`, `_codedom`, `_codedom_polyglot`, `_testing`, `_campaign`, `_intelligence`, `_tools`, `_mcp`, `_prompts`, `_reviewer`, `_shards`, `_coder`, `_context`.

Core Decl examples (`schemas_safety.mg`):

- `Decl permitted(ActionType, Target, Payload) bound [/name, /string, /string].`
- `dangerous_action`, `dangerous_content`, appeal/override family
- Git safety / shadow / diff approval decls in same module family

### 8.2 Policy (executive rules)

All `defaults/policy/*.mg` are concatenated at boot. Major families:

| Family | Files (examples) |
|--------|------------------|
| Safety | `constitution.mg`, `git_safety.mg`, `codedom_safety.mg`, `dreamer.mg`, `shadow_mode.mg` |
| System OODA | `system_core.mg`, `system_ooda.mg`, `system_routing.mg`, `system_session.mg`, `system_shards.mg` |
| Campaign | `campaign_*.mg` |
| Coder | `coder_*.mg` |
| JIT / prompts | `jit_logic.mg`, `jit_selection.mg`, `jit_config.mg`, `prompt_*.mg` |
| Tools / routing | `tool_routing.mg`, `routing_arbitration.mg`, `delegation.mg` |
| TDD | `tdd_logic.mg`, `tdd_loop.mg` |
| Browser | `browser.mg`, `browser_honeypot.mg` |
| Autopoiesis | `autopoiesis.mg`, `system_autopoiesis.mg`, `learning.mg` |

Additional root modules folded into policy: `doc_taxonomy.mg`, `topology_planner.mg`, `build_topology.mg`, `campaign_rules.mg`, `selection_policy.mg`, `taxonomy.mg`, `inference.mg`, `jit_compiler.mg`, `reviewer.mg`, `tester.mg`, `go_safety.mg`, `benchmarks.mg`.

### 8.3 Constitution rule (canonical)

From `defaults/policy/constitution.mg`:

```mangle
permitted(Action, Target, Payload) :-
    safe_action(Action),
    pending_action(_, Action, Target, Payload, _),
    !dangerous_content(Action, Payload),
    !dangerous_content(Action, Target).
```

Plus dangerous-action + signed approval paths and executor bridge via `permitted_action` / `permission_check_result`.

### 8.4 Intent schema modules

`defaults/schema/` holds modular intent corpus pieces (`intent_*.mg`, `prompts.mg`) plus hybrid loading paths for routing/classification.

### 8.5 Corpora

- `predicate_corpus.db` / `predicate_corpus.go` — Decl knowledge for validation,
  opened lazily once by `GetPredicateCorpus`
- `intent_corpus.db` / `prompt_corpus.db` — baked corpora (placeholders present for build hygiene)

---

## 9. Supporting executive subsystems

| Subsystem | File(s) | Role |
|-----------|---------|------|
| APIScheduler | `api_scheduler.go` | Phase-aware concurrent LLM slots |
| ScheduledLLMCall | `scheduled_llm_client.go` | Queue LLM work through scheduler |
| Action validators | `action_validator.go`, `validator_*.go` | File/exec/syntax/codedom/paranoid |
| ShadowMode | `shadow_mode.go` | Multi-action simulation + projection violations |
| TDDLoop | `tdd_loop.go` | Red/green/refactor driven by diagnostics |
| TransactionManager | `transaction_manager.go` | Shadow-validated multi-file edits |
| ToolRegistry | `tool_registry.go` | Tool defs + registration |
| RuleCourt | `rule_court.go` | Rule quality adjudication hooks |
| SelfHealer | `self_healing.go` | Repair invalid learned rules |
| LimitsEnforcer | `limits.go` | Resource ceilings for spawn/etc. |
| MangleWatcher | `mangle_watcher.go` | Watch workspace mangle for reloads |
| Hybrid loader | `hybrid_loader.go` | Split logic vs data sections in `.mg` |
| External predicates | `external_predicates.go` | Virtual/external predicate glue |
| Trace | `trace.go` | Execution tracing helpers |
| Intent inference | `intent_inference.go`, `intent_loader.go` | Intent helpers / defaults |

RuleCourt rejects candidates made only of Unicode whitespace or Unicode format
characters before requiring a kernel, so visually blank learned policy cannot
enter adjudication. TDD loop mocks use the same exact `/run_tests`, target, and
canonical payload permission envelope as production.

---

## 10. Integration map

| Peer package | Relationship |
|--------------|--------------|
| `internal/types` | Fact, Kernel, LLMClient, ShardAgent interfaces (cycle break) |
| `internal/mangle` | SchemaValidator, DifferentialEngine, feedback normalize |
| `internal/session` | Executor / Spawner / TaskExecutor consume Kernel + VS |
| `internal/system` | Boot / Cortex assembly, adapters |
| `internal/perception` | Asserts `user_intent` into kernel |
| `internal/articulation` | Reads derived state for responses |
| `internal/prompt` | JIT atoms; kernel holds prompt facts / selection rules |
| `internal/shards` (domain) | Registration of factories into core shards manager |
| `internal/tactile` | Safe command/file execution |
| `internal/store` | LocalStore / LearningStore backends for VS |
| `internal/tools` | Modular tool registry |
| `internal/transparency` | Glass Box + tool events |
| `internal/features` | Diff eval, per-shard facts flags |
| `internal/logging` | CategoryKernel, CategoryVirtualStore, CategoryDream, … |
| `cmd/nerd` | Boots cortex; CLI query/why/dream/shadow |
| `tests/e2e/*` | Cross-boundary integration tests |

Import cycle control: `Fact`/`Kernel` live in `types`; core aliases them. Session `TaskDelegator` interface defined in core so session can implement without core→session import.

---

## 11. Concurrency & safety notes

- `RealKernel.mu` RWMutex; `factsDirty` atomic for query fast-path.  
- `evalSingleflight` serializes lazy evaluate.  
- Dreamer uses kernel `Clone` sandboxes — do not share mutable store across simulations.  
- VS `mu` guards DI fields; handlers should not hold lock across long I/O where avoidable.  
- APIScheduler coordinates multi-shard LLM pressure.  
- Fail-closed dreamer + default-deny permitted are non-negotiable product invariants.

---

## 12. Observability hooks

| Channel | Use |
|---------|-----|
| `logging.CategoryKernel` | Boot, eval, parse failures |
| `logging.CategoryVirtualStore` | RouteAction, denies, exec |
| `logging.CategoryDream` | Simulation, blocks |
| `logging.Audit()` | ActionRoute / ActionComplete |
| Glass Box bus | Routing + shard lifecycle |
| Tool event bus | Per-tool TUI milestones |
| `debug_program_ERROR.mg` | Failed program dump |
| Provenance Explain | Optional why-derived |

---

## 13. Testing surface (summary)

Dense tests across:

- kernel facts/query/eval/features/provenance/transactions  
- virtual store routing, actions, codedom, python, workflows  
- dreamer + plans + learning + router  
- validators (file, exec, syntax, paranoid)  
- shadow mode, TDD loop, API scheduler  
- cortex + shard fact router  
- policy package golden tests under `defaults/policy/testdata/`  
- e2e under `tests/e2e/*kernel*`, `*virtualstore*`, `*dreamer*`, `*shadowmode*`

Commands:

```powershell
go test ./internal/core/...
go test ./internal/core/defaults/policy/...
go test ./tests/e2e/ -run 'Kernel|VirtualStore|Dreamer|Shadow'
```

---

## 14. Virtual predicates (`VirtualStore.Get`)

Besides effect routing, VS answers **on-demand atoms** during Mangle evaluation via `Get(query ast.Atom)` (`virtual_store.go`):

| Predicate symbol | Purpose (handler family) |
|------------------|--------------------------|
| `query_learned` | Learned store lookup |
| `query_session` | Session history / session-scoped knowledge |
| `recall_similar` | Similarity recall against stores |
| `query_knowledge_graph` | Local knowledge graph query |
| `query_activations` | Spreading-activation style scores |
| `has_learned` | Boolean existence of learned material |
| `query_traces` / `query_trace_stats` | Reasoning / execution traces |
| `query_strategic` | Strategic summary atoms |
| `query_graph` | World-model graph query |

Unknown symbols return `nil, nil` (no atoms). External/virtual predicates force **full** evaluate when present: differential path deliberately skips when `hasExternalPredicatesLocked()` is true because diff eval does not yet forward external-predicate options (documented in `kernel_eval.go`).

Kernel bind: `RealKernel.SetVirtualStore` / VS `SetKernel` mutual wire so callbacks exist during fixpoint.

---

## 15. Permission cache & CheckKernelPermitted

### 15.1 Cache rebuild

`rebuildPermissionCache` (`virtual_store.go`):

1. Queries kernel for `safe_action` **without** holding VS write lock (deadlock fix vs virtual predicates re-entering VS).  
2. Builds `map[string]bool` storing both `/name` and bare `name` keys.  
3. Swaps cache under lock.

Triggered from `SetKernel`. A stale entry may affect classification diagnostics or
performance but not authorization because the exact kernel query always runs.

### 15.2 Runtime permit check

`CheckKernelPermitted(actionType, target, payload)` (same file family as RouteAction):

- Canonicalizes payload JSON and queries exact `permitted/3`.
- Default deny when the kernel is nil, query fails, or action/target/payload do
  not match.
- On deny, logs **payload keys only** (not values) for secret safety.

`QueryPermitted(ActionRequest)` is the request-shaped convenience wrapper.

---

## 16. Dreamer lifecycle details

| Behavior | Implementation detail |
|----------|----------------------|
| Reset | `SetKernel` clears the prior Dreamer |
| Lazy create | serialized `getDreamer()` constructs from `*RealKernel` or Cortex primary |
| Cortex kernels | `GetPrimaryRealKernel()` is the supported Dreamer backing seam |
| Cache key | `type:target` string |
| Eviction | At 256 entries, delete ~half (unordered map iteration) |
| Critical paths | Asserted as `critical_path_prefix` + `Evaluate` |
| Learning | Optional `DreamRouter` + `DreamLearningCollector` for confirmed consultations |
| Plans | Optional `DreamPlanManager` for multi-step dream execution state |

Construction avoids holding the VirtualStore mutex during Dreamer evaluation and
retries if `SetKernel` races the lazy bind. Critical-path facts are asserted into
the selected real kernel.

---

## 17. Evaluation path decision table

| Condition | Path |
|-----------|------|
| `policyDirty` or `programInfo == nil` | `rebuildProgram` then full or diff |
| Diff flag on + no provenance + no external preds + engine valid | `evaluateDiffLocked` |
| Diff returns not-done / error | Fall through / error |
| Otherwise | `evaluateFullLocked` — fresh SimpleInMemoryStore + stratified eval |
| Retract / Clear / Reset | Invalidate diff engine |
| Provenance enabled | Full path so every derivation is recorded |
| VS external predicates active | Full path (options not forwarded on diff) |

Full path applies `WithCreatedFactLimit(derivedFactLimit)` gas cap (default 500k) to mitigate recursive learned-rule explosions (Bug #17 class).

Atom cache: if `cachedAtoms` length ≠ `facts` length, rebuild ToAtom conversions with warn log on desync.

---

## 18. APIScheduler (LLM concurrency executive)

File: `api_scheduler.go`.

**Problem solved:** Multi-shard / multi-agent LLM fan-out can stampede provider rate limits and starve interactive turns behind background jobs.

**Mechanics:**

- Buffered channel semaphore of size `MaxConcurrentAPICalls` (default 5).  
- Waiters sorted by **priority then FIFO sequence** (`schedWaiter`) — interactive work can leapfrog background.  
- Optional `MinCallSpacing` between grants (smooth SuperGrok/subscription bursts).  
- Optional adaptive concurrency: shrink slots after rate-limit events; recover toward base after quiet successes.  
- Per-shard `ShardExecutionState` / phase tracking for observability.  
- Metrics: total calls, wait time, currently waiting/executing, rate-limit events.

`ScheduledLLMCall` (`scheduled_llm_client.go`) wraps `types.LLMClient` so Complete/stream calls acquire/release slots.

---

## 19. Action handler map (by file)

| File | Handler themes |
|------|----------------|
| `virtual_store_actions.go` | `Exec`, exec_cmd, run_tests, build, git, show_diff, impact, browse, research, modular tools, delegate*, ask_user, escalate |
| `virtual_store_file_actions.go` | read/write/edit/delete, search_code, directory list |
| `virtual_store_codedom.go` | open/get/edit elements, line insert/delete, scope refresh/close |
| `virtual_store_workflows.go` | multi-step campaign / ouroboros / corrective workflows |
| `virtual_store_python.go` | python env + swebench-style actions |
| `virtual_store_mcp_proxy.go` | MCP tool proxying |
| `virtual_store_tools.go` | generated / modular tool execution |
| `virtual_store_graph.go` | graph query results |
| `virtual_store_interactive_gate.go` | interactive approval gates |

`executeAction` switch must stay in sync with `ActionType` constants,
destructive classification, and Mangle action policy. `safe_action` remains
classification only; it does not replace the exact pending envelope.

---

## 20. Validator matrix

Registered in `initValidators` (`virtual_store.go`):

| Validator | Typical actions |
|-----------|-----------------|
| File write/edit/delete | Mutation success + path sanity |
| Directory | Dir create/list outcomes |
| Execution / build / test | Exit codes, diagnostic parse |
| Syntax / Mangle syntax | Post-edit parseability |
| CodeDOM / line edit | Element bounds integrity; result metadata supplies concrete file |
| Enhanced edit | Richer edit success heuristics |
| Paranoid file | Aggressive post-condition checks |

`ValidateAll` + `FirstFailure` with confidence ≥ 0.8 returns a route error after
the handler claimed success — defense against “wrote zero bytes but reported OK.”
For CodeDOM edits, semantic reference validation prefers `result.Metadata["file"]`
over the symbolic element target.

---

## 21. Shadow mode vs Dreamer vs transactions

| Mechanism | Granularity | Mutates live FS? | Kernel |
|-----------|-------------|------------------|--------|
| Dreamer | Single action pre-route | No | Clone + projected facts |
| ShadowMode | Multi-action simulation | No (simulation types) | Parent clone / sim state |
| TransactionManager | Multi-file edit batch | Yes on commit after shadow validation | Coordinates with kernel facts |

Use Dreamer always on destructive RouteAction; ShadowMode for exploratory what-if / CLI shadow; TransactionManager for atomic multi-file coding edits.

---

## 22. Fact category taxonomy

`fact_categories.go` maps predicate names to categories for context systems:

| Category | Example predicates |
|----------|-------------------|
| Intent | `user_intent`, goals |
| World | `file_topology`, symbol graph |
| Diagnostic | test/build errors |
| Action | `next_action`, `permitted` |
| Context | context atoms / priority |
| Learning | learned patterns |
| Session | session phase / checkpoint |

Not enforced by the engine; consumers (campaign pager, activation, JIT) use the classifier.

---

## 23. Hybrid Mangle files

`hybrid_loader.go` splits workspace `.mg` files into:

- **Logic** — appended to schemas or policy  
- **Facts** — EDB boot facts (filtered for ephemeral)  
- **Intents** — `HybridIntent` consumed via `ConsumeBootIntents`  
- **Prompts** — `HybridPrompt` via `ConsumeBootPrompts`  

Paths: `.nerd/mangle/extensions.mg`, `policy_overrides.mg`, `learned.mg` (see `loadMangleFiles` user extension section).

---

## 24. End-to-end sequence (one user turn)

```
1. perception → Fact{user_intent, ...}
2. kernel.Assert → factsDirty
3. session/orchestrator Query("next_action") → ensureEvaluated → IDB atoms
4. for each next_action / pending envelope:
     a. (policy) permission_check_result / permitted
     b. VirtualStore.RouteAction
          bootGuard? dreamer? constitution? permitted? handler? validators?
     c. inject execution_result (+ domain facts)
5. re-evaluate as needed for follow-on next_action
6. articulation reads results / residual goals for user text
```

If step 4b denies: schema-correct `security_violation/3` facts remain for
transparency / later rules (`repeated_violation_pattern` family in system policy).

---

## 25. Gaps pointer

Honest gaps and partials: see [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md).

Highlights:

- Diff-eval production caution (external preds + provenance force full path)  
- Dual orchestration paths (session executor vs residual ShardManager)  
- Policy surface size → boot cost and `debug_program_ERROR.mg` risk  
- Dreamer cache identity/invalidation across fact and policy mutation
- Mid-evaluation context cancellation
- Dedicated CodeDOM metadata-to-validator negative regression
- ActionType / safe_action / isDestructive triple-list drift risk  
- Package README still understates live `shards/` plumbing  

---

## 26. Related corpus docs

| Doc | Content |
|-----|---------|
| [README.md](README.md) | Map + verify |
| [01-VISION.md](01-VISION.md) | Target architecture |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | Inventory |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Components & FSMs |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | Exported API |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | Boot wiring |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Safety layers |
| [13-MANGLE-SURFACE.md](13-MANGLE-SURFACE.md) | Schemas/policy deep-dive |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Concrete failures |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Tests |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | Logs / metrics |

---

**End of IMPLEMENTED_SPEC — core**  
Document intentionally dense; prefer editing this file when kernel/VS/Dreamer/policy load order change.
