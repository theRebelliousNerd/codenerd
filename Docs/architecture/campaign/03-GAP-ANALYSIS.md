# 03 — Gap Analysis: campaign

> Last verified: **2026-08-15**  
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
| JIT prompts | **Met** | All 7 roles covered by `internal/prompt/atoms/campaign/*`; static provider warns when used |
| Intelligence from many systems | **Met** | Default-wired from kernel + workspace; explicit config still wins |
| Advisory hard-block | **Met** | Kernel-derived hard/soft contract; hard path aborts `Run` with a renderable error |
| Edge detector always on | **Met** | Default-wired; edge risk gate follows availability |
| Tool pregeneration always on | **Partial** | Still optional; needs an Ouroboros handle the orchestrator does not own |
| Nested campaign refs | **Met** | e2e through `runPhase` with a real kernel, all three policies + defaults |
| Cobra full assault control | **Met** | `campaign assault` with flag coverage enforced by test |
| Package README accuracy | **Met** | Rewritten 2026-08-15 |
| Journal operator tooling | **Met** | `campaign journal verify\|replay` |
| Assault summary export | **Met** | `campaign report` |
| Closed event enum / metrics hooks | **Met** | AST-enforced set; nil-safe `MetricsSink` |
| Campaign rules in-package | **Non-gap** | Correctly live under `internal/core/defaults` |
| Replace constitutional kernel | **Non-gap** | Must not; delegates TE/VS |

## Priority backlog (docs → engineering)

### Closed 2026-08-15

P0 1–2, P1 1–3, P2 1–3, P3 2–3 are implemented and guarded by tests; see
[TODO.md](TODO.md) for the per-item evidence.

### Still open

1. File-task fallback path: confirmed path-traversal safe, but the direct-LLM
   fallback still bypasses the preferred shard route by design. Worth a
   dedicated permission audit rather than a status line.
2. Tool pregeneration is still opt-in; it needs an Ouroboros handle the
   orchestrator does not own.
3. First-class progress channel consumers in all UIs (`cmd/nerd/ui` still
   renders a subset of the closed event set).

## Non-gaps (do not “fix”)

| Item | Why |
|------|-----|
| No per-file assault tasks for huge repos | Intentional batching |
| No busy-wait pause | `pauseCh` design |
| Soft intel failures | Planning should not hard-fail on optional sensors |
| Empty `orchestrator.go` logic | Modularization by design |

## Test gaps (see also [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md))

- End-to-end multi-phase run with real kernel program load is heavier than unit mocks; `campaign_ref_e2e_test.go` and `risk_gate_contract_test.go` now do it for their slices.  
- Assault discover on non-Go monorepos still less covered.  
- ~~Nested `campaign_ref` failure policies need more scenario tests~~ — covered 2026-08-15.

## Bottom line

Campaign is **feature-complete as an engine**, and as of 2026-08-15 the wiring
defaults, prompt path and operator-facing gate clarity are closed too. What
remains is depth: permission auditing of the direct-LLM file fallback, an
Ouroboros handle for tool pregeneration, and UI consumption of the full event
set.
