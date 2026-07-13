# core — Current State

> Last verified: **2026-07-13**  
> Source of truth: filesystem under `C:\CodeProjects\codeNERD\internal\core\`

## 0. Evidence status

- **VERIFIED CURRENT:** safety simulations now keep `hypothetical/1` inside the
  sandbox projection. Evidence:
  `internal/core/dreamer.go#SimulateAction`,
  `internal/core/dreamer.go#projectEffects`, and
  `internal/core/dreamer_test.go#TestDreamer_SimulateAction_DoesNotMutateLiveHypotheticals`.
  The focused package receipt is recorded in [_progress.md](_progress.md).
- **PARTIAL:** Dreamer has a bounded result cache, but mutation-epoch invalidation
  across every fact and policy path has not yet been proven. The proven slice is
  `internal/core/dreamer.go#DreamCache`; the absent seam is a single kernel-state
  epoch or complete invalidation contract.
- **VERIFIED CURRENT:** projected facts are staged with
  `assertWithoutEvalChecked`; invalid encoding and fact-capacity exhaustion return
  an unsafe result. `safe_action/1` may accelerate classification but cannot
  authorize: `CheckKernelPermitted` requires an exact
  `permitted(ActionType, Target, CanonicalPayload)` derivation.
- **VERIFIED CURRENT:** destructive `RouteAction` and interactive tool preflight
  deny when no Dreamer-capable real kernel exists. `CortexKernel` supplies its
  primary real kernel, and the default policy shard owns every predicate in the
  pending-action permission join.
- **VERIFIED CURRENT:** failure and recovery seams now preserve declared shapes:
  `security_violation/3`, `execution_error/2`, post-validation errors, execution
  result timestamps for pruning, and concrete CodeDOM file metadata.
- **VERIFIED CURRENT:** predicate-corpus I/O is lazy (`sync.Once`), Unicode-blank
  RuleCourt candidates short-circuit before kernel access, and full/differential
  evaluation share the same effective derived-fact ceiling.

## 1. Package role (as implemented)

Executive runtime: Mangle kernel + VirtualStore + Dreamer + embedded constitution + shard plumbing + executive helpers (scheduler, validators, TDD, shadow, tools).

## 2. Inventory counts (approximate)

| Class | Location | Notes |
|-------|----------|-------|
| Non-test Go | `internal/core/*.go`, `shards/*.go` | ~78 production `.go` files (order of magnitude) |
| Tests | `*_test.go` in core + shards + defaults/policy | Very high coverage density |
| Mangle | `defaults/**/*.mg` | Schemas modular + policy modular (~70+ policy files) |
| Embed | `//go:embed defaults/*.mg defaults/schema/*.mg defaults/policy/*.mg` | Boot constitution |
| Corpora | `*.db` under defaults | predicate / intent / prompt |
| Docs in package | `internal/core/README.md` | Migration narrative (Dec 2024 JIT-driven) |

Exact file enumeration drifts; use `Get-ChildItem -Recurse` if you need a hard count for a PR.

## 3. Subtree map

### 3.1 Kernel modularization

| File | Role |
|------|------|
| `kernel.go` | Marker + modularization notice |
| `kernel_types.go` | `RealKernel`, aliases, embed FS |
| `kernel_init.go` | Constructors, `loadMangleFiles`, workspace |
| `kernel_facts.go` | Load/Assert/Retract, dedup, scrub |
| `kernel_query.go` | Query paths, file load facts |
| `kernel_eval.go` | rebuildProgram, evaluate, Clone, diff |
| `kernel_policy.go` | Policy load, HotLoadRule |
| `kernel_validation.go` | Learned validation / heal |
| `kernel_provenance.go` | Explain proofs |
| `kernel_transactions.go` | Kernel-level transactions |
| `kernel_virtual.go` | VS attachment |
| `kernel_accessors.go` | GetBaseFacts, ProgramInfo |
| `kernel_utils.go` | Autopoiesis bridge helpers |
| `kernel_shard.go` | Domain KernelShard |
| `cortex_kernel.go` | Multi-domain hub |

### 3.2 VirtualStore modularization

| File | Role |
|------|------|
| `virtual_store.go` | Struct, constructors, DI, inject, permitted cache |
| `virtual_store_types.go` | ActionType enum, Request/Result |
| `virtual_store_routing.go` | RouteAction pipeline |
| `virtual_store_constitution.go` | Go constitutional rules + destructive list |
| `virtual_store_actions.go` | Shell, tests, git, delegate, research |
| `virtual_store_file_actions.go` | Read/write/edit/delete/search |
| `virtual_store_codedom.go` | Element/line DOM ops |
| `virtual_store_workflows.go` | Multi-step workflows |
| `virtual_store_predicates.go` | Virtual GetFacts |
| `virtual_store_python.go` | Python env / SWE-bench style |
| `virtual_store_mcp_proxy.go` | MCP routing |
| `virtual_store_tools.go` | Tool execution glue |
| `virtual_store_graph.go` | Graph query integration |
| `virtual_store_interactive_gate.go` | Interactive gate helpers |

### 3.3 Dreamer family

`dreamer.go`, `dream_plan.go`, `dream_plan_manager.go`, `dream_plan_extractor.go`, `dream_router.go`, `dream_learning.go`.

### 3.4 Safety / quality helpers

`action_validator.go`, `validator_{file,dir,exec,syntax,codedom,edit_enhanced,paranoid}.go`, `shadow_mode.go`, `self_healing.go`, `rule_court.go`, `transaction_manager.go`, `limits.go`.

### 3.5 Scheduling / LLM

`api_scheduler.go`, `llm_client.go`, `scheduled_llm_client.go`.

### 3.6 shards subpackage

`manager.go`, `manager_spawn.go`, `manager_tools.go`, `spawn_queue.go`, `agents.go`, `config.go`.

### 3.7 defaults

| Path | Content |
|------|---------|
| `defaults/schemas.mg` | Index / docs for modular schemas |
| `defaults/schemas_*.mg` | Decl modules by domain |
| `defaults/policy/*.mg` | Executive + safety + domain rules |
| `defaults/schema/` | Intent modularization |
| `defaults/*.mg` (root) | Extra modules (jit_compiler, taxonomy, …) |
| `defaults/policy/testdata/` | Golden EDB tests |
| `defaults/*_corpus*` | Baked DBs + loaders |

## 4. Hotspots (change risk)

| Hotspot | Why high risk |
|---------|----------------|
| `loadMangleFiles` order | Boot correctness; Decl collisions |
| `constitution.mg` / safety schemas | Global permit surface |
| `RouteAction` | Every effect |
| `kernel_eval.evaluate` | Latency + correctness |
| `Dreamer.projectEffects` | Safety false negatives/positives |
| `ActionType` enum growth | Handler switch completeness |
| `ShardManager` spawn | Concurrency + resource limits |

## 5. Behavioral snapshot (what works today)

1. Kernel boots from embed; fails hard on bad constitution.  
2. User/session facts assert; IDB derives actions.  
3. VS routes under exact, target/payload-bound multi-layer safety.
4. Destructive routing requires a usable Dreamer; its checked sandbox blocks on
   critical-path rules or projection staging/evaluation failure.
5. Results re-enter kernel as facts; counterfactual safety inputs do not.
6. Optional Cortex multi-domain + fact router under feature flag.  
7. Policy goldens cover safety/campaign/jit/tdd subsets.
8. Router result facts preserve the executive action ID rather than minting a
   second correlation identity.

## 6. Dual-path reality (important)

| Concern | Path A | Path B |
|---------|--------|--------|
| Agent execution | `internal/session.Executor` | Residual shard agent run via `core/shards` |
| LLM concurrency | APIScheduler | Direct client |
| Kernel shape | Single `RealKernel` | `CortexKernel` federation |
| Eval | Full stratified | Differential (flagged) |
| Boot guard | Chat rehydration holds until first user input | Explicit command `BootCortex` disables during requested-command boot |

Docs and boot code must name which path is preferred for a given binary mode.

## 7. Package README staleness

`internal/core/README.md` correctly describes JIT-driven direction and VirtualStore role but:

- Overstates “ShardManager removed” relative to live `shards/` package.  
- Shows simplified VirtualStore `GetFacts` switch that is illustrative, not the full modular handler surface.  
- Dated “Architecture Version 2.0.0 (December 2024)”.

Prefer this architecture corpus for accurate 2026-07 inventory.

## 8. Related systems outside package

Boot wiring: `cmd/nerd/chat/session_boot.go`, `internal/system/*`.  
Domain shards: `internal/shards/registration.go`.  
Session loop: `internal/session/executor.go`.
