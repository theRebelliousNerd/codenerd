# 01 — Vision: campaign

> Last verified: **2026-07-13**  
> Scope: target architecture for long-horizon goal execution in codeNERD

## Product intent

codeNERD should sustain **hours-to-days** of development work without losing the plan, the why, or safety. Campaigns are the productized form of that promise:

- An operator states a goal (and optional specs).
- The system **proposes** a phased plan (LLM).
- The system **validates and schedules** that plan (Mangle).
- Work proceeds with **context paging**, **checkpoints**, **replan**, and **durable resume**.
- For stress validation, **adversarial assault** campaigns batch whole-repo probes without per-file task explosion.

## Design thesis

The category error in most coding agents is asking one model call stack to be both creative **and** executive across long horizons. Campaigns invert that:

```
Creative  : “What phases/tasks achieve this goal?”
Executive : “Which task is eligible now? Is the campaign blocked? Did checkpoint pass?”
Durable   : “What is true after a crash at hour 3?”
```

## Target user journeys

### J1 — Spec-driven feature

1. `nerd campaign start "Implement OAuth" --docs ./specs/`  
2. Decomposer ingests docs, extracts requirements, proposes phases.  
3. Kernel validates dependencies; operator may clarify low-confidence plans (`plan_needs_review` / `/campaign_clarify` in rules).  
4. Orchestrator executes; TUI shows progress; pause/resume across sessions.

### J2 — Greenfield from research corpus

1. Multi-doc knowledge base under `.nerd/campaigns/<id>/knowledge.db`.  
2. Topology-aware doc selection (`seedDocFacts`, layer classification).  
3. Rolling-wave refinement as early phases produce artifacts.

### J3 — Adversarial assault

1. Chat `/campaign assault` or programatic `NewAdversarialAssaultCampaign`.  
2. Deterministic discovery → batched stages → triage → remediation tasks.  
3. Artifacts under `.nerd/campaigns/<slug>/assault/` for audit.

### J4 — Nested campaigns

1. Parent task type `/campaign_ref` with inheritance scopes and failure policies (`/propagate`, `/absorb`, `/transform`).  
2. Child campaign lifecycle facts drive parent completion.

## Non-goals

- Replacing single-shot OODA for small tasks (policy should recommend downgrade for simple goals — `recommend_downgrade` in `campaign_rules.mg`).
- Owning constitutional permission (kernel + VirtualStore remain authority).
- Becoming a general workflow engine for non-code domains.
- Encoding client-app-specific campaign types into this package.

## Success metrics (qualitative)

| Metric | Signal |
|--------|--------|
| Resume integrity | Load after kill continues without phantom in_progress forever |
| Context stability | Token usage stays within budget reserves; compress after phases |
| Logic ownership | Task order changes only via facts/replans, not ad-hoc Go sorts alone |
| Operator trust | Checkpoints fail loud; journals reconstruct sequence |
| Safety | Risk gates block reckless protected-root campaigns when configured |

## Evolution trajectory

1. **Now:** Modular orchestrator + assault + risk + journal (implemented).  
2. **Next:** Universal JIT prompt atoms for all campaign roles; hard contracts for advisory blocks.  
3. **Later:** Deeper world-model / northstar coupling at every phase boundary without extra LLM calls when facts suffice.

See [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) for scored alignment and [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) for current reality.
