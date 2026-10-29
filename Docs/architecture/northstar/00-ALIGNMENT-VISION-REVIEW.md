# 00 — Alignment & Vision Review: Northstar (`internal/northstar`)

> Last verified against codebase: 2026-07-13  
> Status: Living Reference Document — code-grounded  
> Source: `internal/northstar/` (4 non-test Go ≈ 2.2k lines; 6 tests ≈ 3.1k lines)

## 1. North-star statement (package-local)

Northstar is the **permanent vision guardian**: it holds a project’s mission/problem/vision, watches work (tasks, campaigns, background events), scores alignment with an optional LLM, records drift, and projects vision into the **Mangle kernel** as structured facts so policy and prompt selection can reason over “what this project is for.”

It embodies the platform inversion of control at a strategic layer:

- **LLM as creative center** — articulates vision (wizard) and judges alignment (`LLMClient.CompleteWithSystem`).
- **Logic as executive** — kernel predicates (`northstar_*`, `northstar_defined`) and campaign risk gates decide whether work may proceed.
- **Transduction** — natural-language vision becomes `types.Fact` via `Vision.ToFacts()` and durable rows in SQLite.

## 2. Alignment dimensions

| Dimension | Score (0–5) | Evidence |
|-----------|-------------|----------|
| Creative/executive split | **4** | Alignment judgment is LLM-shaped (`guardian.go` `CheckAlignment`); hard campaign block only when result is `AlignmentBlocked` (`observer.go` `StartCampaign` / `OnPhaseStart`). Soft defaults when no LLM (score 0.8 pass). |
| Fact-flow fidelity | **3** | Facts can be asserted via `KernelClient` on init/update (`refreshKernelFacts`). Boot path in `session_shared_boot.go` calls `SetParentKernel`; primary `session_boot.go` path currently does **not** wire the kernel into the guardian (handler only). |
| Dual-store honesty | **2** | Package owns SQLite (`northstar_knowledge.db`); wizard/CLI read/write `.nerd/northstar.json` / `northstar.mg`. No in-package sync. |
| Campaign integration | **4** | `CampaignObserver` wired into orchestrator risk gate (`risk_scoring.go` `runNorthstarRiskGate`), phase start/complete, task complete, campaign end. |
| Test grounding | **5** | ~114 test functions; guardian/store/observer/types covered including concurrency, parse edge cases, drift refresh. |
| Observability | **3** | Dedicated `CategoryNorthstar`; Info on checks/init, Debug on assert/record failures. No metrics/export of alignment history via CLI. |
| Safety / default deny | **3** | No vision → skip (score 1.0); failed LLM → warning not block; only `blocked` stops campaign start/phase. Not constitutional `permitted(...)` — orthogonal safety layer. |
| JIT / atom discipline | **4** | Package itself does not hardcode wizard prose into shards; wizard/atoms live under `internal/prompt/atoms/northstar/` and chat. Guardian alignment prompts are **inline strings** in `buildAlignmentSystemPrompt` (not atoms) — intentional library-level tradeoff, still a discipline gap. |
| Wiring completeness | **3** | Background observer + manual `/alignment` + campaign observer exist; `TaskObserver` and several store APIs are library-ready but lightly called outside tests. |

**Overall alignment: 3.4 / 5** — solid living guardian library with strong unit tests and real campaign hooks; residual risk is **dual persistence**, uneven kernel wiring across boot paths, and soft enforcement outside campaign risk gates.

## 3. What “good” looks like (Northstar-specific)

| Good | Bad |
|------|-----|
| Vision loaded once, cloned under RLock for checks | Callers mutate `GetVision()` result and expect store update |
| Failed/blocked checks create drift events | Alignment failures only log and vanish |
| Campaign goal fails closed when blocked | Campaign starts regardless of vision conflict |
| Kernel retract+assert on vision change | Stale `northstar_*` facts after vision edit |
| Single source of truth for vision content | JSON wizard file and SQLite diverge silently |

## 4. Related corpora

- `Docs/architecture/cli/` — wizard, `nerd northstar`, `/alignment`
- `Docs/architecture/campaign/` — risk gates, phase hooks
- `Docs/architecture/prompt/` — `internal/prompt/atoms/northstar/`
- `Docs/architecture/core/` — fact assert/retract, schemas for `northstar_*` Decls
- `Docs/architecture/shards/` — `BackgroundObserverManager`, `NorthstarHandler`
