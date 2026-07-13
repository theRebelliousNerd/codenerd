# 12 — Failure Modes: JIT config

> Last verified against codebase: **2026-07-13**  
> Concrete failures involving `EffectiveAgentRuntimeConfig` production/consumption

## FM1 — Validate rejects identity

| | |
|--|--|
| **Symptom** | `config validation failed: identity_prompt is required` |
| **Cause** | Empty or whitespace-only `IdentityPrompt` |
| **Where** | `Validate()`; factory tests; if consumers start validating YAML |
| **Mitigation** | Always set identity from compilation result or specialist YAML; use `GenerateFallback` with explicit fallback string |
| **Recovery** | Fix producer; do not strip identity |

## FM2 — Validate rejects policies

| | |
|--|--|
| **Symptom** | `config validation failed: at least one policy file is required` |
| **Cause** | `Policies` nil or empty |
| **Where** | `Validate()` |
| **Mitigation** | Factory uses `defaultBasePolicies` (`base.mg`); custom atoms must include ≥1 policy |
| **Recovery** | Register ConfigAtom with policies; never ship zero-policy personas |

## FM3 — ConfigFactory finds no ConfigAtoms

| | |
|--|--|
| **Symptom** | `no config atoms found for intents: …` |
| **Cause** | Unknown intent verb; provider empty |
| **Where** | `ConfigFactory.Generate` |
| **Mitigation** | Default provider covers common verbs; `/consult/*` falls back to `/general`; executor may empty-fallback |
| **Recovery** | Register atom; normalize intent; ensure boot uses `NewDefaultConfigFactory` |

## FM4 — Empty config fallback after generate/compile failure

| | |
|--|--|
| **Symptom** | Turn continues with 0 tools; weak identity; Validate would fail |
| **Cause** | Session/spawner catch errors and set `&EffectiveAgentRuntimeConfig{}` |
| **Where** | `executor.compileConfig` / execute path; `spawner.generateConfig` |
| **Mitigation** | Logs warn; LLM may still answer text-only |
| **Risk** | Side-effecting intents complete without tools (orchestrator false complete) — partially mitigated by `intent_requires_tool_call` + no-tool nudge |
| **Recovery** | Fix JIT compile / factory; treat empty cfg as hard fail for side-effect intents |

## FM5 — Tool name not in allowlist

| | |
|--|--|
| **Symptom** | Tool call rejected / not executed |
| **Cause** | Model requests tool outside `AllowedTools` |
| **Where** | `session.isToolAllowed` / executeToolCall |
| **Mitigation** | Persona tools include needed set; nudge atoms list available tools |
| **Recovery** | Expand ConfigAtom tools or correct model behavior |

## FM6 — Allowlist name not in registry

| | |
|--|--|
| **Symptom** | Fewer tool definitions than allowlist length; silent drop |
| **Cause** | Typo / renamed tool in registry vs ConfigAtom |
| **Where** | `buildToolDefinitions` |
| **Mitigation** | Keep ConfigAtom lists aligned with `internal/tools` registration |
| **Recovery** | Log missing tools (if not already) and fix names |

## FM7 — Specialist YAML parse failure

| | |
|--|--|
| **Symptom** | Spawn specialist fails with unmarshal error |
| **Cause** | Invalid YAML / wrong types |
| **Where** | `loadSpecialistConfig` |
| **Mitigation** | Follow flat schema in `06-PUBLIC-API-AND-TYPES.md` |
| **Recovery** | Fix file; fall back only if file missing (not on parse error) |

## FM8 — Specialist YAML oversized

| | |
|--|--|
| **Symptom** | Error: exceeds maximum size |
| **Cause** | File > 1 MiB |
| **Where** | `loadSpecialistConfig` |
| **Mitigation** | Keep configs small; identity should reference atoms when possible |

## FM9 — Path traversal specialist name

| | |
|--|--|
| **Symptom** | `invalid specialist name … path traversal` |
| **Cause** | Name contains `..` or separators |
| **Where** | `loadSpecialistConfig` |
| **Mitigation** | Always sanitize names at API edge |

## FM10 — Dead ToolLoop / Safety fields

| | |
|--|--|
| **Symptom** | Operator sets `tool_loop.max_iterations: 2` in YAML; executor still uses 8 |
| **Cause** | Field not wired to `runToolLoop` |
| **Where** | Schema vs `ExecutorConfig` |
| **Mitigation** | Document in gap analysis; wire or remove |
| **Risk** | False sense of control |

## FM11 — Concurrent slice mutation

| | |
|--|--|
| **Symptom** | Flaky tool lists across agents |
| **Cause** | Shared slice mutated after Generate |
| **Where** | Callers; partially tested in prompt gaps |
| **Mitigation** | Clone AllowedTools/Policies when injecting per SubAgent |

## FM12 — Dual identity sources

| | |
|--|--|
| **Symptom** | System prompt ≠ `cfg.IdentityPrompt` |
| **Cause** | Executor often uses `compileResult.Prompt` for LLM system text; cfg identity may match only if factory used same result |
| **Where** | `runToolLoop(systemPrompt, …)` vs cfg fields |
| **Mitigation** | Keep factory IdentityPrompt = result.Prompt; SubAgent inject paths should stay consistent |

## Failure class summary

| Class | Primary owner |
|-------|---------------|
| Schema validation | `internal/jit/config` |
| Atom coverage | `internal/prompt` |
| Empty fallback policy | `internal/session` |
| Tool registry alignment | `internal/tools` + ConfigAtoms |
| Constitutional permission | `internal/core` policy |
