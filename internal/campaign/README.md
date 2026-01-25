# internal/campaign/

Multi-Phase Campaign Orchestration for complex, long-running development tasks.

**Architecture Version:** 2.0.0 (December 2024 - JIT-Driven)

## Overview

The campaign package implements long-running campaign orchestration for complex, multi-phase development tasks. Campaigns are logic-validated plans (LLM + Mangle) that execute through shards with phase-aware context paging.

## Architecture

```text
Goal → Decomposer → Campaign Plan → Orchestrator
                                         ↓
              ┌──────────────────────────┴──────────────────────────┐
              ↓                          ↓                          ↓
         Phase 1                    Phase 2                    Phase 3
    ┌─────┴─────┐              ┌─────┴─────┐              ┌─────┴─────┐
  Tasks    Checkpoint        Tasks    Checkpoint        Tasks    Checkpoint
```

## Structure

```text
campaign/
├── orchestrator.go            # Package marker (modularized)
├── orchestrator_*.go          # Modular orchestrator files (11 files)
├── decomposer.go              # Goal → Campaign plan
├── types.go                   # Campaign, Phase, Task types
├── context_pager.go           # Token budget management
├── checkpoint.go              # Phase verification
├── replan.go                  # Adaptive replanning
├── assault_*.go               # Adversarial assault campaigns
├── document_ingestor.go       # Knowledge ingestion
└── prompts.go                 # God Tier campaign prompts
```

## Key Concepts

| Concept | Description |
|---------|-------------|
| **Campaign** | Directed graph of Phases with token budget |
| **Phase** | Ordered unit with objectives, tasks, dependencies |
| **Task** | Atomic work item routed to shards |
| **Checkpoint** | Verification before phase completion |

## Orchestrator Flow

1. **Decompose**: Ingest docs → extract requirements → LLM proposes plan → kernel validates
2. **Execute**: Phase-by-phase, task-by-task with bounded parallelism
3. **Context Paging**: Activate per phase, prefetch upcoming, compress completed
4. **Checkpoint**: Verify objectives (tests/build/reviewer)
5. **Replan**: Kernel derives `replan_needed/2`; Replanner updates

## Pause/Resume

```go
orch.Pause()   // Pause campaign execution
orch.Resume()  // Resume from saved state
orch.Stop()    // Stop and cleanup
```

State persists to `.nerd/campaigns/<id>.json`.

## Context Paging

Token budget allocation per phase:

| Reserve | Purpose |
|---------|---------|
| Core | Essential facts and schemas |
| Phase | Current phase context |
| History | Compressed previous phases |
| Working | Current task processing |
| Prefetch | Upcoming task hints |

## Adversarial Assault Campaigns

Special campaigns for systematic security testing:

| Stage | Description |
|-------|-------------|
| Discovery | Find attack targets |
| Batch Execution | Run attacks in batches |
| Triage | Classify and prioritize findings |

```go
campaign := NewAdversarialAssaultCampaign(config)
```

## Directory Structure

```text
.nerd/campaigns/
├── feature-auth.json         # Campaign state
└── feature-auth/
    └── assault/              # Adversarial artifacts
```

## Testing

```bash
CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers" go test ./internal/campaign/...
```

---

**Last Updated:** December 2024


> *[Archived & Reviewed by The Librarian on 2026-01-25]*