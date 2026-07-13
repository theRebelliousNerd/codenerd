# core — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/core/` (complete internal coverage)
> **Implementation: `internal/core/` — 78 non-test .go, 107 tests, 129 .mg**


## Package

`internal/core/`

## Exported types (sampled, up to 40)

| Type | Location |
|------|----------|
| `ValidationResult` | `internal/core/action_validator.go:16` |
| `ActionValidator` | `internal/core/action_validator.go:60` |
| `ValidatorRegistry` | `internal/core/action_validator.go:78` |
| `AggregateResult` | `internal/core/action_validator.go:241` |
| `ShardPhase` | `internal/core/api_scheduler.go:33` |
| `ShardExecutionState` | `internal/core/api_scheduler.go:70` |
| `APISchedulerConfig` | `internal/core/api_scheduler.go:94` |
| `APIScheduler` | `internal/core/api_scheduler.go:128` |
| `APISchedulerMetrics` | `internal/core/api_scheduler.go:644` |
| `CortexKernel` | `internal/core/cortex_kernel.go:39` |
| `CortexTransaction` | `internal/core/cortex_kernel.go:476` |
| `DreamLearningType` | `internal/core/dream_learning.go:26` |
| `DreamLearning` | `internal/core/dream_learning.go:72` |
| `DreamLearningCollector` | `internal/core/dream_learning.go:91` |
| `DreamConsultation` | `internal/core/dream_learning.go:578` |
| `DreamPlanStatus` | `internal/core/dream_plan.go:9` |
| `DreamPlan` | `internal/core/dream_plan.go:22` |
| `DreamSubtask` | `internal/core/dream_plan.go:39` |
| `SubtaskStatus` | `internal/core/dream_plan.go:56` |
| `DreamPlanManager` | `internal/core/dream_plan_manager.go:14` |
| `DreamRouter` | `internal/core/dream_router.go:18` |
| `LearningStoreSaver` | `internal/core/dream_router.go:27` |
| `ColdStoreSaver` | `internal/core/dream_router.go:32` |
| `ToolNeed` | `internal/core/dream_router.go:38` |
| `RouteResult` | `internal/core/dream_router.go:66` |
| `DreamResult` | `internal/core/dreamer.go:17` |
| `DreamCache` | `internal/core/dreamer.go:27` |
| `Dreamer` | `internal/core/dreamer.go:91` |
| `FactCategory` | `internal/core/fact_categories.go:10` |
| `FactEvent` | `internal/core/fact_event_bus.go:10` |
| `FactEventBus` | `internal/core/fact_event_bus.go:25` |
| `HybridIntent` | `internal/core/hybrid_loader.go:14` |
| `HybridPrompt` | `internal/core/hybrid_loader.go:23` |
| `HybridLoadResult` | `internal/core/hybrid_loader.go:33` |
| `ExplainOptions` | `internal/core/kernel_provenance.go:59` |
| `KernelShard` | `internal/core/kernel_shard.go:22` |
| `KernelShardConfig` | `internal/core/kernel_shard.go:53` |
| `ShardMetrics` | `internal/core/kernel_shard.go:372` |
| `KernelTransaction` | `internal/core/kernel_transactions.go:25` |
| `Fact` | `internal/core/kernel_types.go:24` |

## Exported functions/methods (sampled, up to 30)

| Symbol | Location |
|--------|----------|
| `NewValidatorRegistry` | `internal/core/action_validator.go:86` |
| `Register` | `internal/core/action_validator.go:95` |
| `Validate` | `internal/core/action_validator.go:147` |
| `ValidateAll` | `internal/core/action_validator.go:206` |
| `FirstFailure` | `internal/core/action_validator.go:216` |
| `HighestConfidence` | `internal/core/action_validator.go:226` |
| `Aggregate` | `internal/core/action_validator.go:265` |
| `ToFacts` | `internal/core/action_validator.go:304` |
| `String` | `internal/core/api_scheduler.go:50` |
| `DefaultAPISchedulerConfig` | `internal/core/api_scheduler.go:117` |
| `NewAPIScheduler` | `internal/core/api_scheduler.go:216` |
| `RegisterShard` | `internal/core/api_scheduler.go:242` |
| `RegisterShardWithPriority` | `internal/core/api_scheduler.go:249` |
| `UnregisterShard` | `internal/core/api_scheduler.go:273` |
| `AcquireAPISlot` | `internal/core/api_scheduler.go:292` |
| `ReleaseAPISlot` | `internal/core/api_scheduler.go:542` |
| `SaveCheckpoint` | `internal/core/api_scheduler.go:575` |
| `LoadCheckpoint` | `internal/core/api_scheduler.go:589` |
| `GetShardState` | `internal/core/api_scheduler.go:602` |
| `GetMetrics` | `internal/core/api_scheduler.go:619` |
| `String` | `internal/core/api_scheduler.go:656` |
| `Stop` | `internal/core/api_scheduler.go:667` |
| `ConfigureGlobalAPIScheduler` | `internal/core/api_scheduler.go:687` |
| `UpdateMaxConcurrentAPICalls` | `internal/core/api_scheduler.go:727` |
| `ReportRateLimit` | `internal/core/api_scheduler.go:759` |
| `ReportSuccess` | `internal/core/api_scheduler.go:793` |
| `EffectiveMaxSlots` | `internal/core/api_scheduler.go:842` |
| `BaseMaxSlots` | `internal/core/api_scheduler.go:849` |
| `GetAPIScheduler` | `internal/core/api_scheduler.go:859` |
| `NewCortexKernel` | `internal/core/cortex_kernel.go:70` |

## Mangle surface

| Artifact | Count |
|----------|------:|
| Package-local `.mg` | 129 |

| Path | Lines |
|------|------:|
| `internal/core/debug_program_ERROR.mg` | 17733 |
| `internal/core/defaults/schema/intent_campaign.mg` | 1203 |
| `internal/core/defaults/schema/intent_queries.mg` | 1152 |
| `internal/core/defaults/campaign_rules.mg` | 922 |
| `internal/core/defaults/schemas_shards.mg` | 635 |
| `internal/core/defaults/schema/intent_operations.mg` | 633 |
| `internal/core/defaults/reviewer.mg` | 610 |
| `internal/core/defaults/taxonomy.mg` | 568 |
| `internal/core/defaults/schema/intent_mutations.mg` | 526 |
| `internal/core/defaults/policy/constitution.mg` | 460 |
| `internal/core/defaults/schema/intent_qualifiers.mg` | 452 |
| `internal/core/defaults/schemas_reviewer.mg` | 437 |
| `internal/core/defaults/schemas_prompts.mg` | 425 |
| `internal/core/defaults/schemas_tools.mg` | 413 |
| `internal/core/defaults/policy/intelligence.mg` | 402 |

Global kernel schemas/policy (when this package participates):

- `internal/core/defaults/schemas.mg`
- `internal/core/defaults/policy/`

## Fact-flow placement

```
user input → perception → user_intent → kernel(core/mangle) → next_action
  → VirtualStore / shards / tools → articulation
```

This package: **Mangle kernel, VirtualStore, Dreamer, facts, API scheduler, shard manager plumbing**
