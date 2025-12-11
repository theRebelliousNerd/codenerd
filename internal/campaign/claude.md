# internal/campaign - Multi-Phase Campaign Orchestration

This package implements long‑running campaign orchestration for complex, multi‑phase development tasks.
Campaigns are logic‑validated plans (LLM + Mangle) that execute through shards with phase‑aware context paging.

## Architecture

Campaigns break down goals into Phases and Tasks, then the Orchestrator executes tasks with bounded parallelism,
checkpoints, rolling‑wave replanning, and context paging/compression.
Runtime config is asserted into the kernel so Mangle can derive replanning/checkpoint signals.

## File Structure

| File | Purpose |
|------|---------|
| `orchestrator.go` | Main execution engine: phases, tasks, checkpoints, replanning |
| `decomposer.go` | Goal → requirements → phases/tasks (LLM + kernel validation) |
| `context_pager.go` | Phase‑aware context activation, compression, prefetching |
| `replan.go` | Adaptive replanning + rolling‑wave refinement |
| `checkpoint.go` | Verification checkpoints (tests/build/reviewer) |
| `types.go` | Campaign/Phase/Task models and `ToFacts()` |
| `campaign_prompts.go` | PromptProvider abstraction for JIT prompts |

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
State persists to `.nerd/campaigns/<id>.json`.

## Dependencies

- `internal/core` (RealKernel, ShardManager, VirtualStore)
- `internal/perception` (LLMClient)
- `internal/store` / `internal/embedding` (campaign KB and doc ingestion)

## Testing

Run with sqlite headers available:
```bash
CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers" go test ./internal/campaign/...
```
