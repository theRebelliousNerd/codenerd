# internal/campaign - Multi-Phase Campaign Orchestration

This package implements long‑running campaign orchestration for complex, multi‑phase development tasks.
Campaigns are logic‑validated plans (LLM + Mangle) that execute through shards with phase‑aware context paging.

## Architecture

Campaigns break down goals into Phases and Tasks, then the Orchestrator executes tasks with bounded parallelism,
checkpoints, rolling‑wave replanning, and context paging/compression.
Runtime config is asserted into the kernel so Mangle can derive replanning/checkpoint signals.

## File Index

| File | Description |
|------|-------------|
| `assault_campaign.go` | Builds deterministic adversarial assault campaigns that sweep repos in stages without LLM decomposition. Exports `NewAdversarialAssaultCampaign()` for long-horizon stress testing with batched execution. |
| `assault_prompts.go` | Static fallback prompt for the assault planner role when JIT is unavailable. Exports `AssaultLogic` const with Piggyback Protocol compliance for remediation task generation. |
| `assault_tasks.go` | Target discovery, batched execution, and triage for assault campaigns. Implements `executeAssaultDiscoverTask()`, `executeAssaultBatchTask()`, and `executeAssaultTriageTask()` handlers. |
| `assault_types.go` | Configuration types for adversarial assault campaigns including scopes and stages. Exports `AssaultConfig`, `AssaultScope` (repo/module/subsystem/package), and `AssaultStage` types. |
| `campaign_prompts.go` | Defines the `PromptProvider` interface for JIT prompt injection into campaign roles. Exports `CampaignRole` constants and `StaticPromptProvider` as fallback. |
| `checkpoint.go` | Runs verification checkpoints between campaign phases using tests, builds, or shards. Exports `CheckpointRunner` with handlers for `VerifyTestsPass`, `VerifyBuilds`, and `VerifyNemesisGauntlet`. |
| `context_pager.go` | Manages context window budgets during campaign execution with phase-aware activation. Exports `ContextPager` with token budgeting (core/phase/history/working/prefetch reserves). |
| `decomposer.go` | Decomposes high-level goals into structured campaign plans via LLM + Mangle collaboration. Exports `Decomposer` with `DecomposeRequest` and `DecomposeResult` for spec-to-plan transformation. |
| `document_ingestor.go` | Writes campaign source documents into campaign-scoped knowledge stores with embeddings. Exports `DocumentIngestor` for vector storage and knowledge graph linking. |
| `orchestrator.go` | Package marker documenting orchestrator modularization across 11 files. Points to orchestrator_types, init, lifecycle, execution, control, phases, tasks, handlers, results, failure, utils. |
| `orchestrator_control.go` | Pause/Resume/Stop controls and progress reporting for campaign execution. Implements `Pause()`, `Resume()`, `Stop()`, `GetProgress()`, and channel management. |
| `orchestrator_execution.go` | Main campaign execution loop with cancellation, timeout, and heartbeat support. Implements `Run()` with phase iteration, autosave, and status updates. |
| `orchestrator_failure.go` | Task failure handling with retry logic, backoff, and error classification. Implements `handleTaskFailure()` with attempt tracking and `classifyTaskError()`. |
| `orchestrator_init.go` | Orchestrator constructor with configuration defaults for timeouts, retries, and heartbeat. Exports `NewOrchestrator()` that wires kernel, shard manager, and context pager. |
| `orchestrator_lifecycle.go` | Campaign loading, saving, and state management. Implements `LoadCampaign()`, `SetCampaign()`, `saveCampaign()`, and kernel fact synchronization. |
| `orchestrator_phases.go` | Phase management and Mangle-based phase queries. Implements `getCurrentPhase()`, `getEligibleTasks()`, and phase status updates. |
| `orchestrator_task_handlers.go` | Individual task type execution handlers routing to shards or shell commands. Implements `executeTask()` dispatcher and type-specific handlers (file, test, research, assault). |
| `orchestrator_task_results.go` | Task result caching for context injection between dependent tasks. Implements `storeTaskResult()` with LRU pruning and `computeNeededResultIDs()` for retention. |
| `orchestrator_tasks.go` | Phase execution with bounded parallelism and rolling-wave refinement. Implements `runPhase()` coordinating concurrent task execution with checkpoint triggers. |
| `orchestrator_types.go` | Core type definitions including `Orchestrator` struct with all dependencies. Defines `OrchestratorConfig`, parallelism settings, and component references. |
| `orchestrator_utils.go` | Campaign configuration fact assertion and failure tracking utilities. Implements `assertCampaignConfigFacts()` and `updateFailedTaskCount()` for kernel sync. |
| `prompts.go` | God Tier static prompts for campaign roles (Librarian, Extractor, Taxonomy, Planner). Exports `LibrarianLogic` and role-specific prompts with Piggyback Protocol and hallucination prevention. |
| `replan.go` | Adaptive replanning when campaigns encounter failures or new requirements. Exports `Replanner` with `Replan()` returning `ReplanResult` containing added/removed/modified tasks. |
| `types.go` | Core domain models for campaigns, phases, and tasks with Mangle fact conversion. Exports `Campaign`, `Phase`, `Task`, status constants, and `ToFacts()` methods. |
| `utils.go` | Utility functions for text chunking, campaign ID sanitization, and file extension filtering. Exports `chunkText()`, `sanitizeCampaignID()`, and `isSupportedDocExt()`. |

## Key Concepts

- **Campaign**: Directed graph of Phases with token budget and learnings.
- **Phase**: Ordered unit with objectives, tasks, dependencies, context profile, and compressed summary.
- **Task**: Atomic work item routed to shards (or explicit shard) with deps and artifacts.
- **PromptProvider**: Lets campaign roles use JIT prompt atoms via an external adapter.

## Orchestrator Flow

1. **Decompose**: ingest docs → extract requirements → LLM proposes plan → kernel validates → refine.
2. **Execute**: phase‑by‑phase, task‑by‑task with bounded parallelism.
3. **Context Paging**: Activate once per phase, prefetch upcoming tasks, compress completed phases.
4. **Checkpoint**: Verify objectives (tests/build/reviewer) before phase completion.
5. **Replan**: Kernel derives `replan_needed/2`; Replanner updates downstream phases.

## Pause/Resume

Campaigns can be paused/resumed via:
```go
orch.Pause()
orch.Resume()
```
State persists to `.nerd/campaigns/<id>.json`. Adversarial assault campaigns also persist artifacts under `.nerd/campaigns/<id>/assault/`.

## Dependencies

- `internal/core` (RealKernel, ShardManager, VirtualStore)
- `internal/perception` (LLMClient)
- `internal/store` / `internal/embedding` (campaign KB and doc ingestion)

## Testing

Run with sqlite headers available:
```bash
CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers" go test ./internal/campaign/...
```

---

**Remember: Push to GitHub regularly!**


> *[Archived & Reviewed by The Librarian on 2026-01-25]*