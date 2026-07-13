# core — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/core/` (78 non-test .go, 107 tests, 129 .mg)**


## Source package

`internal/core/`

## Exported / primary types (sampled)

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

## Planned vs actual Mangle surface

| Artifact | Count | Notes |
|----------|-------|-------|
| `.mg` files under package | 129 | See inventory |
| Core schemas (global) | shared | `internal/core/defaults/schemas.mg` when kernel-touching |
| Policy modules (global) | shared | `internal/core/defaults/policy/` |

### Package-local Mangle inventory (top)

| Path | Lines |
|------|-------|
| `internal/core/debug_program_ERROR.mg` | 17733 |
| `internal/core/defaults/schema/intent_campaign.mg` | 1203 |
| `internal/core/defaults/schema/intent_queries.mg` | 1152 |
| `internal/core/defaults/campaign_rules.mg` | 922 |
| `internal/core/defaults/schemas_shards.mg` | 635 |
| `internal/core/defaults/schema/intent_operations.mg` | 633 |
| `internal/core/defaults/reviewer.mg` | 610 |
| `internal/core/defaults/taxonomy.mg` | 568 |

## Fact-flow placement

```
user input → perception → user_intent → kernel(core/mangle) → next_action
  → VirtualStore / shards / tools → articulation
```

This package's position: **Mangle kernel, VirtualStore, Dreamer, fact store, shard manager plumbing**

## Data & control concepts

- Primary language surface: Go under `internal/core/`
- Logic surface: Mangle where listed above or via kernel defaults
- External effects: VirtualStore / tools / CLI depending on package
