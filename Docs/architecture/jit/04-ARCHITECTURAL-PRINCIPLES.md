# 04 — Architectural Principles: JIT config

> Binding principles for `internal/jit` and any code that constructs or consumes `EffectiveAgentRuntimeConfig`.  
> Last verified: **2026-07-13**

## P1 — Schema only; no orchestration

`internal/jit/config` must not import session, core, prompt, tools, or LLM clients. It is a **pure data contract**. Side effects, I/O, and routing belong to producers and consumers.

## P2 — One type for all personas

Never reintroduce per-persona Go agent structs. New behavior = ConfigAtoms + prompt atoms + `.mg` policies + tool registry entries. The runtime config type stays shared.

## P3 — Identity is mandatory for “full” agents

An agent without an identity prompt is not a specialized agent. `Validate()` rejects blank/whitespace `IdentityPrompt`. Callers that intentionally run degraded mode must not pretend to have validated.

## P4 — Policies anchor the executive layer

At least one policy reference is required by `Validate`. Treat empty `Policies` as unconstitutional for any config that claims to be production-ready. Prefer `base.mg` + persona overlays (factory convention).

## P5 — Tools are an allowlist, not a denylist

`AllowedTools` enumerates what the model may request. Session code must check membership before execute (`isToolAllowed`). Empty list must not be silently treated as “all tools” without an explicit product decision.

## P6 — YAML and factory share the same shape

Specialist configs under `.nerd/agents/<name>/config.yaml` use **flat** snake_case fields matching struct tags (`identity_prompt`, `allowed_tools`, `policies`, nested `tool_loop` / `safety` / `workspace`). Do not reintroduce nested `Tools.AllowedTools` or `Policies.Files` wrappers without a versioned migration.

## P7 — Fail closed on path traversal for specialists

Consumers loading YAML by agent name must reject `..` and path separators (already in `Spawner.loadSpecialistConfig`). Schema package does not own paths, but consumers of the type must.

## P8 — Do not invent dead knobs

If a field is not read by any consumer, either wire it or remove it. `ToolLoop` and `RequirePolicyEnforcement` currently risk **false authority** in YAML/docs.

## P9 — JIT-first for LLM-facing text

Identity and behavioral prose come from prompt compilation / atoms, not hard-coded shard strings. The schema carries the **result** (`IdentityPrompt`), not the atom library.

## P10 — Wiring audit before deletion

If a field looks unused, grep session/prompt/e2e before deleting. This codebase has dormant integration points; the schema may be a foreshadow for incomplete wiring rather than dead code.

## P11 — No time/cost estimates in architecture docs

Roadmaps use dependency gates and ordering only.

## P12 — Validate at trust boundaries

Trust boundaries for this type:

1. Factory generation (optional defense-in-depth)  
2. YAML unmarshal of specialist configs (**should** Validate)  
3. External API injection if ever exposed  

Skipping Validate is only acceptable for explicitly degraded internal fallbacks with logging.
