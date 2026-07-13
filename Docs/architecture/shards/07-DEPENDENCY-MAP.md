# 07 — Dependency Map: shards

> Last verified against codebase: 2026-07-13  
> Upstream (what shards imports) and downstream (who imports shards)

## 1. Upstream dependencies (this package imports)

### Root `internal/shards`

| Import | Why |
|--------|-----|
| `internal/articulation` | PromptAssembler for factories / interrogator |
| `internal/config` | JITConfig |
| `internal/core` | VirtualStore, LearningStore adapter, RealKernel/Cortex types |
| `internal/core/shards` | ShardManager Register/DefineProfile |
| `internal/perception` | LLMClient types for RegistryContext |
| `internal/prompt` | JITPromptCompiler |
| `internal/shards/system` | Concrete system constructors |
| `internal/store` | LearningStore adapter |
| `internal/types` | ShardAgent, permissions, facts |

### `internal/shards/system`

| Import | Why |
|--------|-----|
| `internal/articulation` | JIT + piggyback |
| `internal/browser` | Router browser manager |
| `internal/campaign` | CampaignRunner orchestrator |
| `internal/config` | JIT config |
| `internal/core` | Kernel, VirtualStore, PredicateCorpus |
| `internal/core/shards` | DefaultSystemConfig |
| `internal/logging` | CategorySystemShards |
| `internal/mangle/feedback` | Validation/autopoiesis loops |
| `internal/mangle/synth` | Legislator/repair synthesis constraints |
| `internal/perception` | Transducer, Intent types |
| `internal/prompt` | PredicateSelector |
| `internal/store` | ToolStore |
| `internal/tactile` | Campaign tactile hooks |
| `internal/transparency` | GlassBox, ToolEventBus |
| `internal/types` | Facts, permissions, agents |
| `internal/world` | ASTParser, Scanner concepts |

**Does not import:** `cmd/*`, `internal/session` (avoids cycle; session consumes manager/factories higher up).

## 2. Downstream consumers (import `codenerd/internal/shards` or `.../system`)

| Consumer | Path evidence | Use |
|----------|---------------|-----|
| Cortex factory | `internal/system/factory.go` | `RegisterAllShardFactories`, re-register router/campaign |
| Interactive chat boot | `cmd/nerd/chat/session_boot.go` | Inline system factories + profiles |
| Chat process | `cmd/nerd/chat/process.go` | Boot guard; perception_firewall lookup |
| Delegation | `cmd/nerd/chat/delegation.go`, `delegation_modes.go` | Matching / consultation |
| Campaign consultation adapter | `cmd/nerd/chat/campaign_consultation_adapter.go` | Consultation types |
| Model types | `cmd/nerd/chat/model_types.go`, `model_update.go` | Hold managers |
| Campaign CLI | `cmd/nerd/cmd_campaign.go` | Register factories |
| Init scanner | `internal/init/scanner.go` | Register factories |
| Action linter tool | `cmd/tools/action_linter` | system package routes/actions |
| Tests | various e2e / package tests | Boot guard, shards |

Commented historical imports of deleted domain packages remain in several chat files for migration notes only.

## 3. Runtime dependency graph (fact-time)

```
perception → kernel facts → policy (core defaults .mg)
    → executive (this pkg) → constitution (this pkg)
    → router (this pkg) → VirtualStore (core) → tools/tactile/browser
```

## 4. Sibling packages (related but not owned)

| Package | Relationship |
|---------|--------------|
| `internal/core/shards` | Lifecycle engine; this package supplies factories/agents |
| `internal/session` | Persona execution replaces domain shards |
| `internal/campaign` | Orchestrator used by campaign_runner |
| `internal/features` | `IsSystemShardsEnabled` gates chat StartSystemShards |
| `internal/northstar` | May implement NorthstarHandler for observers |

## 5. Circular import risks

| Risk | Mitigation in code |
|------|--------------------|
| system → articulation → core → system | PromptAssembler stored as `any` on base; adapters |
| shards → core/shards → shards | Manager does not import this package; factories injected from outside |
| system → session | Avoided; campaign uses orchestrator only |

## 6. Feature flags / env

| Flag / env | Effect |
|------------|--------|
| `features.IsSystemShardsEnabled()` | Skip StartSystemShards in chat boot |
| `NERD_DISABLE_SYSTEM_SHARDS` | Comma-separated disable list (chat) |
| `BootConfig.DisableSystemShards` / CLI `--disable-system-shard` | Factory disable set |
| `features.IsPerShardFactsEnabled()` | Future consumer of predicate manifests |
