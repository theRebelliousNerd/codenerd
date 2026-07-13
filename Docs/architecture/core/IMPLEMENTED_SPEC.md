# core — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/core/` (complete internal coverage)
> **Implementation: `internal/core/` — 78 non-test .go, 107 tests, 129 .mg**


## 1. Purpose

Mangle kernel, VirtualStore, Dreamer, facts, API scheduler, shard manager plumbing

## 2. Source paths (1:1)

| Path | Role |
|------|------|
| `internal/core/` | Primary implementation |
| `Docs/architecture/core/` | This full corpus |

## 3. Implementation Status

> Living code status — **not** pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
| Source package tree | Implemented | **88%** |
| Exported types (sampled) | Implemented | **80%** |
| Tests | Implemented | **88%** |
| Mangle local sources | Implemented | **85%** |
| Full architecture corpus | Implemented | **100%** |

**Overall (heuristic): 88%** as living package (78 src / 107 tests)

## 4. Public surface inventory

### Largest files

| Path | Lines |
|------|------:|
| `internal/core/kernel_facts.go` | 1255 | source |
| `internal/core/virtual_store.go` | 1077 | source |
| `internal/core/virtual_store_workflows.go` | 1029 | source |
| `internal/core/virtual_store_actions.go` | 1008 | source |
| `internal/core/virtual_store_predicates.go` | 993 | source |
| `internal/core/api_scheduler.go` | 871 | source |
| `internal/core/scheduled_llm_client.go` | 854 | source |
| `internal/core/tdd_loop.go` | 833 | source |
| `internal/core/dreamer.go` | 753 | source |
| `internal/core/cortex_kernel.go` | 731 | source |
| `internal/core/kernel_eval.go` | 730 | source |
| `internal/core/virtual_store_codedom.go` | 706 | source |
| `internal/core/predicate_corpus.go` | 636 | source |
| `internal/core/shards/manager_spawn.go` | 622 | source |
| `internal/core/kernel_query.go` | 595 | source |
| `internal/core/kernel_init.go` | 591 | source |
| `internal/core/dream_learning.go` | 585 | source |
| `internal/core/tool_registry.go` | 582 | source |
| `internal/core/shards/spawn_queue.go` | 581 | source |
| `internal/core/virtual_store_python.go` | 573 | source |

### Types (sampled)

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

### Functions (sampled)

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

## 5. Integration relevance

| Surface | Relevance |
|---------|-----------|
| Kernel | Primary |
| VirtualStore | Owner |
| Shards | Related |
| Prompt JIT | Optional |
| CLI | Related via `cmd/nerd` |
| Config | Reader |

## 6. Non-goals of this corpus revision

- Full prose rewrite of every function body
- Docs/Spec 18-file product templates (`spec-doc-sprint`)
- Implementing missing features (corpus-build implementation mode)
