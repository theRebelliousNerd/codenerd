# 03 — Gap Analysis: campaign

> Last verified: **2026-07-13**  
> Compares vision ([01-VISION.md](01-VISION.md)) and north star to **implemented** code.

## Legend

| Status | Meaning |
|--------|---------|
| Met | Present and usable |
| Partial | Exists but incomplete/unwired/soft |
| Missing | Not found in source |
| Non-gap | Intentional out-of-scope |

## Spec vs reality matrix

| Vision / requirement | Status | Evidence / notes |
|----------------------|--------|------------------|
| Multi-phase campaigns with durable state | **Met** | JSON + journal; Load/Set/Run |
| LLM plan proposal | **Met** | `Decomposer.llmProposePlan` |
| Mangle plan validation | **Met** | `validatePlan`, LoadFacts |
| Kernel-driven task eligibility | **Met** | `eligible_task`, `next_campaign_task` queries |
| Context paging + compression | **Met** | `ContextPager` |
| Phase checkpoints | **Met** | `CheckpointRunner` |
| Adaptive replan | **Met** | `Replanner` + triggers |
| Rolling-wave refinement | **Met** | `triggerRollingWave` |
| Pause / resume / stop | **Met** | control + status |
| Bounded parallel tasks | **Met** | maxParallel + write-set locks |
| Adversarial assault | **Met** | assault_* files |
| Risk gating | **Met** | `risk_scoring.go` preflight |
| JIT prompts | **Partial** | Interface + CLI adapter; static default still primary mass |
| Intelligence from many systems | **Partial** | Implemented but optional DI |
| Advisory hard-block | **Partial** | Synthesis logs; not always abortive |
| Edge detector always on | **Partial** | Optional; risk auto-wire when present |
| Tool pregeneration always on | **Partial** | Optional |
| Nested campaign refs | **Met** (types/handlers) | Needs integration testing density |
| Cobra full assault control | **Partial** | Chat/docs emphasize assault; Cobra tree is lifecycle cmds |
| Package README accuracy | **Partial** | Stale version/date |
| Campaign rules in-package | **Non-gap** | Correctly live under `internal/core/defaults` |
| Replace constitutional kernel | **Non-gap** | Must not; delegates TE/VS |

## Priority backlog (docs → engineering)

### P0 — Correctness / safety

1. Confirm every production start path sets `TaskExecutor` (not shard-only).  
2. Document and enforce risk gate outcomes in operator UX (not only logs).  
3. Ensure file-task fallback path remains path-traversal safe (covered) and permissioned.

### P1 — Wiring completeness

1. Default-on intelligence gatherer when Cortex has world/git/MCP handles.  
2. Map advisory `BlockingConcern` → hard fail vs user prompt contract.  
3. Parity: Cobra flags for assault config or explicit “chat-only” documentation.

### P2 — JIT / maintainability

1. Migrate high-value `prompts.go` roles into prompt atoms under `internal/prompt/atoms/`.  
2. Shrink static provider to thin fallback.  
3. Refresh `internal/campaign/README.md` to modular map.

### P3 — Observability / ops

1. First-class progress channel consumers in all UIs.  
2. Journal tooling (replay/verify CLI).  
3. Assault summary export command.

## Non-gaps (do not “fix”)

| Item | Why |
|------|-----|
| No per-file assault tasks for huge repos | Intentional batching |
| No busy-wait pause | `pauseCh` design |
| Soft intel failures | Planning should not hard-fail on optional sensors |
| Empty `orchestrator.go` logic | Modularization by design |

## Test gaps (see also [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md))

- End-to-end multi-phase run with real kernel program load is heavier than unit mocks.  
- Assault discover on non-Go monorepos less covered.  
- Nested `campaign_ref` failure policies need more scenario tests.

## Bottom line

Campaign is **feature-complete as an engine**. Gaps are primarily **wiring defaults**, **prompt path migration**, and **operator-facing gate clarity** — not missing orchestrator fundamentals.
