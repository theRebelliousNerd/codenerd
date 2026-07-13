# core — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/core/` (78 non-test .go, 107 tests, 129 .mg)**


## 1. Purpose

Mangle kernel, VirtualStore, Dreamer, fact store, shard manager plumbing

## 2. Source paths

| Path | Role |
|------|------|
| `internal/core/` | Primary implementation |
| `Docs/architecture/core/` | This corpus |

## 3. Implementation Status

> Status reflects **living code**, not pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
| Source package tree | Implemented | **88%** |
| Exported types (sampled) | Implemented | **80%** |
| Tests | Implemented | **88%** |
| Mangle local sources | Implemented | **85%** |
| Docs corpus (this) | Implemented | **100%** |

**Overall (heuristic): 88% complete as living package**

## 4. Public surface (inventory-driven)

### Largest implementation files

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

### Sampled types

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

## 5. Integration points (codeNERD spine)

| Surface | Relevance |
|---------|-----------|
| Kernel / facts | Primary |
| VirtualStore | Owner |
| Shards | Related |
| Prompt JIT | Optional |
| CLI | Invokes via cmd/nerd |
| Config | Reads global config |

## 6. Non-goals for this corpus revision

- Full behavioral prose rewrite of every function
- Filling Docs/Spec 18-file product templates (use spec-doc-sprint)
- Implementing missing runtime features (use corpus-build implementation mode)
