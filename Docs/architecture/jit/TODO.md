# TODO — JIT config package / consumers

> Last verified: **2026-07-13**  
> Docs-only corpus rebuild. Items below are **engineering backlog**, not claims of completed work.

## P0 — Safety / correctness

- [ ] Call `Validate()` after specialist YAML unmarshal in `session.Spawner.loadSpecialistConfig`; fail spawn on invalid configs.
- [ ] Define and implement **empty-config policy**: refuse side-effecting intents when config is zero-value, or mark explicit degraded mode with hard logging.
- [ ] Either **wire** `Safety.RequirePolicyEnforcement` (refuse empty Policies when true) or **remove** the flag and factory default.

## P1 — Schema honesty

- [ ] Wire `ToolLoop` into `session.runToolLoop` **or** stop populating it in `ConfigFactory` and document executor-only limits in specialist YAML docs.
- [ ] Document or implement application of `Policies` to kernel (load/assert); avoid cargo-cult Validate requirement.
- [ ] Align dual identity sources: system prompt vs `cfg.IdentityPrompt` must stay consistent on SubAgent inject paths.

## P2 — Hygiene

- [ ] Rename `TestAgentConfigValidation` → `TestEffectiveAgentRuntimeConfigValidation`.
- [ ] Update skill `jit-execution-model.md` snippets to flat `EffectiveAgentRuntimeConfig` (no nested Tools/Policies wrappers, no stale `AgentConfig` where it misleads).
- [ ] Optional `doc.go` under `internal/jit/config` clarifying schema-only role and link to prompt+session.
- [ ] Consider `Validate()` at end of `ConfigFactory.Generate` (defense in depth).
- [ ] Log when allowlist names miss the tool registry during `buildToolDefinitions`.

## P3 — Product extensions (only if needed)

- [ ] Per-agent `Model` routing for multi-engine specialists.
- [ ] Per-agent `Workspace.RootPath` sandboxing.
- [ ] Metrics: generate/validate/empty-fallback counters (session/prompt, not schema package).

## Explicit non-goals

- Do not grow orchestration into `internal/jit`.
- Do not reintroduce per-persona Go agent types.
- Do not invent Vectryx product surfaces here.
