# 04 — Architectural Principles: JIT config

> Binding principles for `internal/jit` and any code that constructs or consumes `EffectiveAgentRuntimeConfig`.  
> Last verified: **2026-07-13**

## P1 — Schema plus deterministic inventory validation; no orchestration

`internal/jit/config` is a **pure data contract** with one deliberate project
dependency: core's read-only embedded policy inventory. It must not import
session, prompt, tools, or LLM clients. Side effects, I/O, loading, and routing
belong to producers and consumers; the core import may validate membership but
must not boot or mutate a kernel.

## P2 — One type for all personas

Never reintroduce per-persona Go agent structs. New behavior = ConfigAtoms + prompt atoms + `.mg` policies + tool registry entries. The runtime config type stays shared.

## P3 — Identity is mandatory for “full” agents

An agent without an identity prompt is not a specialized agent. `Validate()` rejects blank/whitespace `IdentityPrompt`. Callers that intentionally run degraded mode must not pretend to have validated.

## P4 — Policies anchor the executive layer

At least one unique canonical embedded policy reference is required by
`Validate`. Treat empty, aliased, traversal-shaped, duplicate, or missing
`Policies` as invalid for any config that claims to be production-ready. Default
providers must resolve stable core registry set IDs rather than invent paths.
Canonical membership does not itself prove selective per-agent enforcement.

## P5 — Tools are an allowlist, not a denylist

`AllowedTools` enumerates what the model may request. Session code checks membership before modular or Ouroboros execution (`isToolAllowed`); nil and empty lists are deny-all. Registry membership proves handler availability, never capability.

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
2. YAML unmarshal of specialist configs (**does** Validate)
3. External API injection if ever exposed  

Generated and fallback configs still need a uniform validation/degradation
contract. Skipping `Validate` must never grant a tool; current zero-value
fallbacks satisfy that narrower rule because session capability checks deny-all.
