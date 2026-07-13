# 08 — Wiring and Integration: shards

> Last verified against codebase: 2026-07-13  
> How shards are registered, started, and called

## 1. Cortex factory path (`internal/system/factory.go`)

### 1.1 `initShardManagement`

1. Attach limits enforcer + spawn queue to ShardManager  
2. Build `shards.RegistryContext` (kernel, LLM, VS, workspace, JIT, optional ClassificationClient)  
3. `shards.RegisterAllShardFactories(shardManager, regCtx)`  
4. Set JIT registrar/unregistrar on manager  
5. **Override** factories for:  
   - `tactile_router` — +BrowserManager, PromptAssembler adapter  
   - `campaign_runner` — +workspace, +ShardManager  
6. Apply `cfg.DisableSystemShards`  
7. `StartSystemShards(ctx)`  

### 1.2 When Cortex boots

`GetOrBootCortex` / `BootCortex` → init chain includes shard management. Cached Cortex reuses running system shards.

### 1.3 Boot guard on VirtualStore

Factory also calls `virtualStore.DisableBootGuard()` at a late stage in some boot paths — **verify** against current factory timing when debugging auto-actions. Executive still has its own boot guard until disabled.

## 2. Interactive chat path (`cmd/nerd/chat/session_boot.go`)

### 2.1 Inline factories

Registers each system shard by name with chat-specific wiring:

| Shard | Extra DI |
|-------|----------|
| perception_firewall | classification client, learning candidate store (LocalDB), PromptAssembler, learning thresholds from appCfg |
| world_model_ingestor | VS, PromptAssembler |
| executive_policy | learning candidates, appCfg threshold, PromptAssembler |
| constitution_gate | PromptAssembler |
| mangle_repair | corpus + `kernel.SetRepairInterceptor` |
| tactile_router | GlassBox, ToolEventBus, ToolStore, BrowserManager |
| session_planner | PromptAssembler |
| requirements_interrogator | LLM + kernel |
| legislator | VS + PromptAssembler |
| campaign_runner | workspace (ShardManager injection may differ from factory) |

Then `shards.RegisterSystemShardProfiles(shardMgr)`.

### 2.2 Feature flags

```
if !features.IsSystemShardsEnabled() { skip StartSystemShards }
else apply disable set from CLI + NERD_DISABLE_SYSTEM_SHARDS
     StartSystemShards(ctx)
```

### 2.3 Per-turn wiring (`process.go`)

- After user message: disable VirtualStore boot guard  
- May fetch running `perception_firewall` for input handling  
- Delegation paths use matching/consultation managers held on model  

## 3. Campaign CLI (`cmd/nerd/cmd_campaign.go`)

Calls `RegisterAllShardFactories` with a RegistryContext for campaign-oriented Cortex/setup. Disables VS boot guard for CLI-initiated commands.

## 4. Init scanner (`internal/init/scanner.go`)

Registers factories (possibly with empty RegistryContext for structural registration during scan) — hollow-context risk if scan path executes shards.

## 5. Kernel integration points

| Hook | Shard | Mechanism |
|------|-------|-----------|
| Fact event bus | exec, const, router | `SubscribeToFacts` |
| Repair interceptor | mangle_repair | `RealKernel.SetRepairInterceptor` |
| Predicate corpus | repair, legislator, exec feedback | `GetPredicateCorpus` + PredicateSelector |
| Learning store | perception, executive | VirtualStore → adapter |

## 6. Session / TaskExecutor relationship

```
User coding task
  → session.TaskExecutor / Executor (JIT persona)
  → tools via VirtualStore
  → NOT via Spawn("coder") domain factory

System OODA
  → StartSystemShards
  → continuous loops
```

Domain persona tools still may produce `next_action` / `delegate_task` facts that executive observes.

## 7. Disable matrix

| Mechanism | Scope |
|-----------|-------|
| `DisableSystemShard(name)` on manager | Per name |
| CLI disable list | BootConfig / chat args |
| Env `NERD_DISABLE_SYSTEM_SHARDS` | Chat boot only (comma list) |
| Feature flag system shards off | Chat skips start entirely |

## 8. Wiring checklist for a new system shard

1. Implement type embedding `*BaseSystemShard`  
2. Constructor + Execute loop with StopCh/ctx  
3. Factory in `register*` method in `registration.go`  
4. Profile in `define*Profile`  
5. Until unified: mirror factory in `session_boot.go` if chat needs extra DI  
6. Update `DefaultShardPredicateManifests` if new owned preds  
7. Tests for factory registration + happy-path Execute  
8. Document in this corpus  

## 9. Known wiring partials

| Item | Status |
|------|--------|
| Predicate manifests → factory KernelShard | Exported; production consume incomplete (source comment) |
| Campaign runner SetShardManager on all paths | Strong on factory re-register; weaker on pure RegisterAll |
| GlassBox/Tool buses | Strong on chat tactile_router; weaker on factory default factory from RegisterAll before override |
