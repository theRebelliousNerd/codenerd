# shards — Implemented Spec (Deep-Dive)

> Last verified against codebase: 2026-07-13  
> Status: Living Reference Document  
> Language: Go  
> Primary sources: `internal/shards/`, `internal/shards/system/`  
> Scale (approx): **18** non-test Go sources; **24** test files; **0** local `.mg`  
> Related: `internal/core/shards` (manager), `internal/system/factory.go` (Cortex boot), `cmd/nerd/chat/session_boot.go` (interactive boot)

## 1. Overview

`internal/shards` is the **implementation home for Type-1 system shards** and **specialist orchestration libraries** that sit between the Mangle kernel and the rest of the runtime.

### Historical inversion (read this first)

| Era | Shape |
|-----|--------|
| Pre-JIT | Large Go domain shards (`coder/`, `tester/`, `reviewer/`, `researcher/`, `nemesis/`, `tool_generator/`) with hard-coded prompts and routing |
| Current | Domain work = **JIT personas** + `session.Executor` + Mangle `intent_routing`; this package keeps **system OODA shards** + matching/consultation/observer helpers |

The package-local `README.md` still markets “implementations removed.” That is **only half-true**: domain packages were deleted; **system shards, registration, matching, consultation, and observer manager are live and large**.

### What this package owns

1. **Factory registration** — `RegisterAllShardFactories`, profiles, predicate ownership manifests  
2. **System shards** (`system/`) — perception, executive, constitution, router, world model, planner, campaign runner, legislator, mangle repair  
3. **Ephemeral special-purpose** — `requirements_interrogator` (Socratic clarify)  
4. **Specialist libraries** — technology matching, consultation protocol, background observer manager  

### What this package does **not** own

- `ShardManager` lifecycle engine → `internal/core/shards`  
- Session clean-loop LLM execution → `internal/session`  
- Policy corpus Decl/rules → `internal/core/defaults/policy/`  
- VirtualStore tool implementations → `internal/core` / `internal/tactile`  

### High-level fact pipeline (OODA)

```
user NL / CLI input
        │
        ▼
perception_firewall          (LLM-primary NL → user_intent atoms)
        │
        ▼
Mangle policy evaluation     (next_action, barriers, strategies)
        │
        ▼
executive_policy             (query next_action → pending_action; boot guard)
        │
        ▼
constitution_gate            (default deny; permitted_action | security_violation)
        │
        ▼
tactile_router               (action → ToolRoute → exec_request / tool call)
        │
        ▼
VirtualStore / tools         (effectful execution)
        │
        ▼
articulation / TUI           (surface response)
```

Parallel / on-demand support shards:

| Shard | Role |
|-------|------|
| `world_model_ingestor` | file_topology / symbols / diagnostics |
| `session_planner` | agenda decomposition & checkpoints |
| `campaign_runner` | supervise long-horizon campaigns on disk |
| `legislator` | synthesize & ratify learned Mangle constraints |
| `mangle_repair` | intercept invalid learned rules before persist |
| `requirements_interrogator` | ephemeral Socratic questions |

---

## 2. Implementation status

| Component | Status | Notes |
|-----------|--------|-------|
| `RegisterAllShardFactories` + profiles | **Implemented** | `registration.go` |
| `ShardPredicateManifest` table | **Implemented, partial wire** | Exported; factory still hard-codes owned preds in places |
| System shard base (`BaseSystemShard`, `CostGuard`, `AutopoiesisLoop`) | **Implemented** | `system/base.go` |
| Perception firewall | **Implemented** | Transducer + JIT + classification tiering |
| Executive policy | **Implemented** | Event-driven + boot guard + autopoiesis |
| Constitution gate | **Implemented** | StrictMode, appeals, dangerous patterns |
| Tactile router | **Implemented** | Large default route table, rate limits |
| World model ingestor | **Implemented** | Hybrid scan + AST |
| Session planner | **Implemented** | LLM agenda + checkpoints |
| Campaign runner | **Implemented** | On-demand supervisor |
| Legislator | **Implemented** | FeedbackLoop + synth modes |
| Mangle repair | **Implemented** | Kernel `SetRepairInterceptor` |
| Requirements interrogator | **Implemented** | JIT-required when LLM present |
| Specialist matching | **Implemented** | Pattern-based (not embedding) |
| Consultation manager | **Implemented** | Spawner-backed |
| Background observers | **Implemented** | Northstar direct handler path |
| Domain Go shards | **Removed** | JIT personas only |
| Dual registration paths | **Implemented / drift risk** | factory.go vs session_boot.go |

**Overall:** production OODA substrate — **not** pre-implementation.

---

## 3. Source inventory

### 3.1 Package layout

```
internal/shards/
  registration.go              # factories + profiles + predicate manifests
  matching.go                  # specialist/tech matching + classifications
  consultation.go              # cross-specialist consultation
  observer_manager.go          # background observer events/assessments
  requirements_interrogator.go # ephemeral Socratic shard
  *_test.go
  system/
    base.go                    # BaseSystemShard, CostGuard, AutopoiesisLoop
    perception.go              # PerceptionFirewallShard
    executive.go               # ExecutivePolicyShard loop
    executive_intent.go        # intent hydration / next_action query
    executive_autopoiesis.go   # strategy-gap rule proposals
    constitution.go            # ConstitutionGateShard
    router.go                  # TactileRouterShard + ToolRoute tables
    world_model.go             # WorldModelIngestorShard
    planner.go                 # SessionPlannerShard
    campaign_runner.go         # CampaignRunnerShard
    legislator.go              # LegislatorShard
    mangle_repair.go           # MangleRepairShard
    payloads.go                # pending/permitted payload encode/decode
    *_test.go
```

### 3.2 Largest non-test sources (approx lines)

| Path | Lines | Purpose |
|------|------:|---------|
| `system/planner.go` | ~1108 | Agenda, checkpoints, plan views |
| `system/mangle_repair.go` | ~1061 | Validate/repair learned rules |
| `system/constitution.go` | ~1038 | Safety gate + appeals |
| `system/router.go` | ~1030 | Action→tool routes + exec loop |
| `system/perception.go` | ~953 | NL→intent firewall |
| `system/base.go` | ~779 | Shared system shard infrastructure |
| `system/world_model.go` | ~748 | Workspace fact ingestion |
| `system/executive.go` | ~660 | OODA decision loop |
| `matching.go` | ~649 | Specialist match engine |
| `system/executive_intent.go` | ~563 | Intent/action binding helpers |
| `observer_manager.go` | ~542 | Background observers |
| `registration.go` | ~534 | Factory registration |
| `system/campaign_runner.go` | ~471 | Campaign supervision |
| `system/legislator.go` | ~457 | Learned constraint synthesis |
| `consultation.go` | ~406 | Consultation protocol |
| `system/executive_autopoiesis.go` | ~269 | Executive rule proposals |
| `requirements_interrogator.go` | ~190 | Clarify questions |
| `system/payloads.go` | ~69 | Payload codecs |

---

## 4. Registration deep dive

### 4.1 `RegistryContext`

```go
// internal/shards/registration.go
type RegistryContext struct {
    Kernel               types.Kernel
    LLMClient            perception.LLMClient
    VirtualStore         *core.VirtualStore
    Workspace            string
    JITCompiler          *prompt.JITPromptCompiler
    JITConfig            config.JITConfig
    ClassificationClient perception.LLMClient // optional cheaper model
}
```

Factories **must** receive kernel + LLM + VirtualStore at registration time (“hollow shard” fix). Optional `ClassificationClient` enables perception model tiering.

### 4.2 `RegisterAllShardFactories`

Entry point used by Cortex boot (`internal/system/factory.go` → `initShardManagement`):

1. `sm.SetVirtualStore(ctx.VirtualStore)`  
2. `registerEphemeralShards` → `requirements_interrogator`  
3. `registerSystemShards` → `perception_firewall`, `world_model_ingestor`  
4. `registerLogicShards` → `executive_policy`, `constitution_gate`, `legislator`, `mangle_repair`  
5. `registerPlanningShards` → `tactile_router`, `campaign_runner`, `session_planner`  
6. `defineShardProfiles` / `defineSystemShardProfiles`  

Each factory:

- constructs the concrete `system.New…Shard()`  
- `SetParentKernel`, `SetLLMClient`, `SetVirtualStore` as needed  
- attaches `PromptAssembler` via `createAssembler()` when kernel present  
- wires learning store from VirtualStore for perception + executive  
- for `mangle_repair`, extracts `*core.RealKernel` (or cortex primary) and `SetRepairInterceptor(shard)`  

### 4.3 Profiles (startup + permissions)

| Profile name | Type | Startup | LLM model hint | Notable permissions |
|--------------|------|---------|----------------|---------------------|
| `perception_firewall` | System | **Auto** | Balanced | ReadFile, AskUser |
| `world_model_ingestor` | System | OnDemand | HighSpeed | ReadFile, ExecCmd, CodeGraph |
| `executive_policy` | System | **Auto** | none (logic) | ReadFile, CodeGraph, AskUser |
| `constitution_gate` | System | **Auto** | none (logic) | AskUser only |
| `mangle_repair` | System | **Auto** | HighReasoning | ReadFile |
| `tactile_router` | System | OnDemand | none | ExecCmd, Network, Browser |
| `session_planner` | System | OnDemand | HighReasoning | AskUser, ReadFile |
| `campaign_runner` | System | OnDemand | Balanced | Read/Write/Exec |
| `legislator` | System | OnDemand | none in profile* | ReadFile, CodeGraph |
| `requirements_interrogator` | Ephemeral | n/a | Balanced | AskUser, ReadFile |

\*Legislator constructor still sets HighReasoning when creating the shard; profile Model is empty for “logic-primary” marketing.

Timeouts for system shards are typically 24h (permanent). Ephemeral interrogator: 5 minutes.

### 4.4 Predicate ownership manifests

`DefaultShardPredicateManifests()` declares domains (`routing`, `world`, `tools`, `policy`, `campaign`, `prompts`, `cortex` catch-all) with `OwnedPredicates` lists. Intended consumer: per-shard fact router when `features.IsPerShardFactsEnabled()`. **Wiring note in source:** production `KernelShard` construction lives in `internal/system/factory.go`; manifest is exported for convergence — do not assume every assert path uses it yet.

### 4.5 Dual boot registration (drift surface)

| Path | Behavior |
|------|----------|
| `internal/system/factory.go` `initShardManagement` | Calls `RegisterAllShardFactories`, then **re-registers** `tactile_router` / `campaign_runner` with browser + shard manager extras; applies `DisableSystemShards`; `StartSystemShards` |
| `cmd/nerd/chat/session_boot.go` | Registers each system shard **inline** with chat-specific wiring (GlassBox, ToolEventBus, ToolStore, classification client, learning candidates); `RegisterSystemShardProfiles`; feature flag `features.IsSystemShardsEnabled()`; env `NERD_DISABLE_SYSTEM_SHARDS` |

Both paths must stay semantically aligned. Prefer fixing wiring gaps over assuming one path is dead.

---

## 5. System shard deep dives

### 5.1 Base infrastructure (`system/base.go`)

`BaseSystemShard` embeds identity, config, state, kernel, LLM, VirtualStore, GlassBox, ToolEventBus, ToolStore, JIT assembler (as `any`), CostGuard, AutopoiesisLoop, learning maps, StopCh, event subscription channel.

**StartupMode:**

- `StartupAuto` — start with application  
- `StartupOnDemand` — start when needed / idle-timeout eligible  

**CostGuard defaults:**

| Limit | Default |
|-------|---------|
| MaxLLMCallsPerMinute | 10 |
| MaxLLMCallsPerSession | 100 |
| IdleTimeout | 5m (overridable per shard) |
| CooldownAfterError | exponential 1s…60s |
| MaxValidationRetries | 3 |
| ValidationBudget | 20 |

**AutopoiesisLoop:** after `UnhandledThreshold` (default 3) unhandled Mangle cases, shards may propose rules (`RuleConfidence` 0.8).

**Key methods:** `SubscribeToFacts`, `SetParentKernel` (unwraps `CortexKernel` → primary RealKernel), `TryJITPrompt`, `GuardedLLMCall`, learning load/persist on stop, `EmitHeartbeat`.

### 5.2 Perception Firewall (`system/perception.go`)

- **Mode:** Auto-start, **LLM-primary**  
- Queues inputs; confidence/ambiguity thresholds (0.85 / 0.7)  
- Canonical `perception.Transducer` for NL → piggyback → intent  
- Optional `ClassificationClient` for cheaper/faster classification  
- Learning candidates via `LearningCandidateStore`  
- Regex verb fallback when LLM fails (`UseFallbackParsing`)  
- Emits `user_intent`, focus resolution, ambiguity flags (via kernel asserts through transducer path)

Interactive turns often also drive a transducer outside the shard; the firewall remains the long-running system perception agent and classification-tier entry.

### 5.3 Executive Policy (`system/executive.go` + helpers)

- **Mode:** Auto-start, **logic-primary**  
- Subscribes to: `user_intent`, `next_action`, `delegate_task`, `tdd_next_action`, `campaign_next_action`, `repair_next_action`  
- Heartbeat every **15s**; fallback poll **2s** if no event bus  
- On start: **retracts stale** `user_intent`, `processed_intent`, `executive_processed_intent`, `pending_action` (infinite-loop prevention)  
- `evaluatePolicy`: strategies → barriers (`block_commit`) → `queryNextActions` → emit `pending_action`  
- **Boot guard** (`bootGuardActive=true` until first user interaction) suppresses derived actions during rehydration  
- Max **5** actions/tick (storm prevention)  
- Hydrates target/payload from latest `user_intent` (`executive_intent.go`)  
- Autopoiesis via `feedback.FeedbackLoop` + `executive_autopoiesis.go` (waited via `autopoiesisWg` before stop)  
- OODA stall tracking emits `ooda_timeout` after ~30s with pending intent and no actions  

Disable boot guard: `DisableBootGuard()` on executive **and** typically `VirtualStore.DisableBootGuard()` from chat/CLI after user message.

### 5.4 Constitution Gate (`system/constitution.go`)

- **Mode:** Auto-start, **safety-critical**, pure logic by default  
- Subscribes to `pending_action`  
- `checkPermitted` order:  
  1. Active appeal overrides  
  2. Dangerous regex patterns on target (`rm -rf`, `mkfs`, `curl|sh`, …)  
  3. Network domain allowlist for network/fetch/browse  
  4. Mangle `permitted(...)` query — **default deny** in StrictMode  
- On permit: assert `permitted_action` + `permission_check_result(/permit)`  
- On deny: `permission_check_result(/deny)`, `routing_result(/failure)`, `security_violation`, `appeal_available`; optional user escalate  
- Appeal API: `SubmitAppeal`, `HandleAppeal`, temporary overrides  
- Prunes old `permission_check_result` facts (>15m)  

### 5.5 Tactile Router (`system/router.go`)

- **Mode:** On-demand, logic-primary  
- Subscribes to `permitted_action`  
- Maps action pattern → `ToolRoute{ToolName, Timeout, RateLimit, RequiresSafe}`  
- Default tables: system/control, file/code, execution/env (incl. python/swebench), network/git/browser, agentic/campaign/ouroboros  
- `AllowUnmappedActions` default **false**  
- Emits `exec_request` for VirtualStore async processing; tracks ToolCall lifecycle  
- Optional BrowserManager, GlassBox, ToolEventBus, ToolStore  
- IdleTimeout default 30s  

### 5.6 World Model Ingestor (`system/world_model.go`)

- On-demand hybrid: scan workspace → `file_topology`, symbols via `world.ASTParser`, diagnostics, dependency links  
- Include/exclude globs; max files/scan; large-file hash-only mode  

### 5.7 Session Planner (`system/planner.go`)

- On-demand, LLM-primary for goal decomposition  
- Agenda items, checkpoints, retry budgets, `PlanView` JSON surface  
- Idle timeout 10m; tick 5s  

### 5.8 Campaign Runner (`system/campaign_runner.go`)

- On-demand supervisor (explicit start; **not** auto-run campaigns on boot)  
- Scans workspace campaigns; manages `campaign.Orchestrator` lifecycle with restart backoff  
- Emits `campaign_runner_heartbeat`  
- Needs workspace root + optional ShardManager injection  

### 5.9 Legislator (`system/legislator.go`)

- On-demand; translates corrective feedback into durable Mangle rules  
- Uses `feedback.FeedbackLoop` with `SynthModeRequire` single-clause options  
- Schema-capable LLM path when available; piggyback surface extraction  

### 5.10 Mangle Repair (`system/mangle_repair.go`)

- Auto-start gatekeeper for **learned rule persistence**  
- Pipeline: syntax → safety (unbound vars / unsafe negation) → schema (corpus Decl) → stratification  
- LLM repair loop, max 3 attempts; then reject  
- Wired as `RealKernel.SetRepairInterceptor`  

### 5.11 Payloads (`system/payloads.go`)

Shared codecs for action payloads between executive/constitution/router:

- `encodeActionPayload` / `decodeActionPayload`  
- Handles JSON maps, JSON strings, Go `map[...]` pseudo-string dumps  
- Extracts `intent_id`  

---

## 6. Package-level specialist systems

### 6.1 Matching (`matching.go`)

- `CoreTechnologyPatterns` — rod, golang, react, mangle, sql, api, testing, bubbletea, cobra, security, concurrency, grpc  
- `AgentPatternMapping` → expert names (`GoExpert`, …)  
- Classifications: executor / advisor / observer × technical / strategic / domain  
- Verb configs (`DefaultVerbConfigs`): `/review` parallel, `/fix` advisory+critique, `/create` advisory, etc.  
- `MatchSpecialistsForTask` scores files via path/import/content hints (filesystem read; **not** embeddings)  
- `ShouldSpecialistExecuteTask` requires executor + high confidence (>0.8)  

Used by chat delegation / campaign advisory paths (`cmd/nerd/chat/delegation.go`, campaign intelligence).

### 6.2 Consultation (`consultation.go`)

- `ConsultationManager` + `ConsultationSpawner` interface  
- Sync request, parallel batch, response cache (max 100), default 2m timeout  
- `GetStrategicAdvisorsFor` / `ShouldConsultBeforeExecution` from classifications  
- `FormatConsultationAdvice` for prompt injection  

### 6.3 Background observers (`observer_manager.go`)

- Event types: task/campaign lifecycle, file modified, user intent, alignment check  
- Assessment levels by score: proceed ≥80, note ≥60, clarify ≥40, else block  
- Event channel + periodic check (default 5m)  
- `NorthstarHandler` bypasses generic spawner for alignment efficiency  
- Atomic `EventsReceived` counter for race-free stats  

### 6.4 Requirements interrogator

Ephemeral `ShardAgent` (not `BaseSystemShard`):

- JIT system prompt required when LLM present (`internal/prompt/atoms/system/requirements_interrogator` expected)  
- Static question fallback only when LLM nil  
- Piggyback parse + question extraction  

---

## 7. Lifecycle model

```
                    ┌─────────────────────────────┐
                    │     ShardManager (core)     │
                    │  factories + profiles +     │
                    │  StartSystemShards / Spawn  │
                    └─────────────┬───────────────┘
                                  │
          ┌───────────────────────┼───────────────────────┐
          │                       │                       │
   StartupAuto              StartupOnDemand           Ephemeral
   perception               tactile_router            requirements_
   executive                world_model               interrogator
   constitution             session_planner
   mangle_repair            campaign_runner
                            legislator
          │                       │                       │
          ▼                       ▼                       ▼
   continuous loop         start on need;            Execute once
   until StopCh/ctx        idle timeout stop         spawn→die
```

State transitions (typical): Idle → Running → Completed (via `SetState`). Stop flushes learning and unsubscribes event bus.

---

## 8. Integration map

| Consumer | How |
|----------|-----|
| `internal/system/factory.go` | Primary Cortex boot registration + StartSystemShards |
| `cmd/nerd/chat/session_boot.go` | Interactive boot; richer DI for glass box / tools |
| `cmd/nerd/chat/process.go` | DisableBootGuard; perception_firewall lookup |
| `cmd/nerd/chat/delegation*.go` | matching + consultation |
| `cmd/nerd/cmd_campaign.go` | RegisterAllShardFactories for campaign CLI |
| `internal/init/scanner.go` | Registers factories during scan init |
| `internal/core` RealKernel | Repair interceptor; fact bus; permitted derivation |
| `internal/session` | Task execution replaces domain-shard Spawn for personas |
| `internal/campaign` | Orchestrator used by campaign_runner |
| `cmd/tools/action_linter` | Imports system package for route/action lint |

### Fact-flow ownership (with manifests)

| Domain | Owned predicates (manifest) |
|--------|----------------------------|
| routing | `user_intent`, `next_action`, `routing_result`, `derived_mode` |
| world | `file_topology`, `symbol_graph`, `diagnostic`, `project_profile` |
| tools | `tool_capabilities`, `shard_lifecycle`, `shell_exec_result` |
| policy | `permitted`, `blocked`, `constitution`, `commit_barrier`, `dangerous_action` |
| campaign | `campaign`, `campaign_phase`, `campaign_task`, `campaign_dependency` |
| prompts | `prompt_atom`, `atom_selection_score`, `shard_prompt_base` |
| cortex | catch-all (nil list) |

Runtime action stream (not all in manifest): `pending_action` → `permitted_action` → `exec_request` / `security_violation`.

---

## 9. North-star alignment (summary)

| Principle | How shards honor it |
|-----------|---------------------|
| LLM creative, logic executive | Perception/planner use LLM; executive/constitution/router logic-primary |
| `permitted(...)` default deny | Constitution StrictMode + query |
| JIT for LLM-facing behavior | PromptAssembler on system shards; interrogator JIT-required |
| Wiring before deletion | Dual registration preserved; domain shards removed only after JIT path |

---

## 10. Gaps pointer

See [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) and [TODO.md](TODO.md). Headline gaps:

1. Dual registration drift (factory vs session_boot)  
2. Predicate manifest not fully consumed  
3. Package README stale (“implementations removed”)  
4. Matching is heuristic/path-based, not embedding-aware  
5. Large system files still only partially tested relative to size  

---

## 11. Verify

```powershell
go test ./internal/shards/...
```

Related:

```powershell
go test ./internal/core/shards/...
go test ./internal/system/ -count=1 -run Boot
```
