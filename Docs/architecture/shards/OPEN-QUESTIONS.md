# OPEN QUESTIONS — shards

> Last verified against codebase: 2026-07-13  
> Real open questions (not rhetorical)

## Q1 — Single registration owner

Should **chat boot** stop hand-registering system shards and only call `RegisterAllShardFactories`, with all chat-only DI moved to `ShardManager.PostSpawnHook` / `RegistryContext` fields?

**Tradeoff:** less drift vs harder-to-see chat-specific wiring.

## Q2 — Who owns predicate routing?

When `IsPerShardFactsEnabled` is true, is ownership enforced in cortex KernelShard construction only, or also at Assert time in RealKernel? Manifest lives in this package today.

## Q3 — Perception dual path

Interactive turns often use a chat-side transducer **and** may involve `perception_firewall`. Is the firewall authoritative continuous agent, a backup, or path-dependent? Clarify product contract to avoid double-assert of `user_intent`.

## Q4 — Router vs VirtualStore direct tools

Session executor can invoke tools without the tactile_router fact path. Is the permitted-stream pipeline **mandatory** for all effectful tools, or only for Mangle-derived `next_action`?

## Q5 — Specialist matching future

Keep heuristic patterns as source of truth, or relegate them to fallback behind embedding retrieval over agent knowledge bases?

## Q6 — Legislator LLM vs “logic-primary” profile

Profile markets no LLM; constructor sets HighReasoning and uses LLM adapters. Should profile Model reflect reality or should legislator become pure-logic with LLM only via explicit autopoiesis feature flag?

## Q7 — Observer enforcement

AssessmentLevel `block` — does any runtime enforce a hard stop, or is it advisory for TUI/northstar only?

## Q8 — Disable matrix UX

Three mechanisms (feature flag, env, CLI list) — should they merge into one config surface under `.nerd/config.json`?

## Q9 — Package boundary with core/shards

Long-term: keep factories here and manager in core, or move system shard types next to manager? Import cycles currently prevent a simple merge.
