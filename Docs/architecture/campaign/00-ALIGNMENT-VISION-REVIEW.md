# 00 — Alignment / Vision Review: campaign

> Last verified: **2026-07-13**  
> Package: `internal/campaign`  
> Rubric: codeNERD north star (LLM creative / Mangle executive / constitutional safety / JIT prompts)

## Scoring scale

| Score | Meaning |
|------:|---------|
| 5 | Strongly aligned; evidence in source |
| 4 | Aligned with minor gaps |
| 3 | Partial / dual path |
| 2 | Weak alignment |
| 1 | Contradicts north star |

## Dimension scores

| Dimension | Score | Evidence |
|-----------|------:|----------|
| Logic as executive | **5** | `Run` queries `current_phase`, `eligible_task`, `next_campaign_task`, `campaign_blocked`, `phase_eligible`; plans become facts via `ToFacts`; policy in `campaign_rules.mg` + base policy |
| LLM as creative center | **5** | Decompose propose, replan deltas, phase compression summaries, optional assault remediation LLM, specialist shards |
| Transduction interface | **5** | Domain structs ↔ kernel facts; status updates retract/assert; intel seeding |
| Constitutional safety | **4** | Risk preflight, write-set locks, path guards on file fallback, northstar hooks; still depends on VirtualStore/`permitted` for tool effects — orchestrator is not a second permission kernel |
| JIT-first prompts | **4** | `PromptProvider` + CLI `CampaignJITProvider`; static prompts remain default fallback (`prompts.go` large) |
| Long-horizon durability | **5** | Atomic snapshots, journal event-before-ack, heartbeat autosave, retry `NextRetryAt`, assault on-disk batches |
| Parallel specialists | **4** | Bounded parallel tasks, advisory board, explicit `task.Shard`, TaskExecutor intents |
| Fail closed | **4** | Config validation, risk force-block mode, checkpoint fail → replan not silent continue; some intel/advisory non-fatal by design |
| Wiring honesty | **3** | Optional DI components easy to leave unwired; package README stale; dual TaskExecutor/ShardManager transition still documented |
| Scope discipline (general substrate) | **5** | Campaign types are general (greenfield/feature/audit/…); assault is general stress, not a client-app feature |

**Weighted read:** campaign is one of the strongest north-star embodiments in the repo — multi-hour goals with **logic scheduling** and **model proposal**. Main drag is optional intelligence wiring and residual static prompt mass.

## What “done” looks like for this package

1. Every campaign start path wires TaskExecutor + JIT PromptProvider + risk gates.  
2. Advisory/edge/northstar gates have clear hard-block vs soft-log contracts.  
3. Assault and feature campaigns share the same durability/journal story (already largely true).  
4. Docs and README match modular orchestrator reality (this corpus).

## Anti-alignment risks (watchlist)

| Risk | Why it matters |
|------|----------------|
| Growing `prompts.go` instead of atoms | Violates JIT-first repo contract |
| Scheduling work without kernel eligibility | Would make Go the executive |
| Direct tactile/LLM mutations without permission facts | Bypasses constitution |
| Dropping journal for “speed” | Breaks long-horizon resume trust |
| Per-app campaign types in core package | Violates general-use substrate rule |

## Summary

**Aligned and production-shaped.** Treat gaps as wiring and prompt-path hygiene, not as “campaign not implemented.”
