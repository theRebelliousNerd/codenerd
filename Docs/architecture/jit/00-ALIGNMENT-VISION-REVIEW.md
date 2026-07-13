# 00 — Alignment & Vision Review: JIT config (`internal/jit`)

> Last verified against codebase: **2026-07-13**  
> Status: Living Reference Document — code-grounded  
> Source: `internal/jit/config/` (1 non-test Go file, 59 lines; 1 test file, 67 lines)

## 1. North-star statement

codeNERD inverts control: the **LLM is the creative center**; the **Mangle kernel is the executive**. Domain agents (coder, tester, reviewer, …) must not be separate hardcoded Go shards. They must be **JIT-configured**: intent selects tools, policies, and identity at runtime; one universal executor runs every persona.

`internal/jit/config` is the **minimal shared type system** for that configuration. Alignment is judged by whether the schema:

1. Forces constitutional anchors (identity + at least one policy file).
2. Carries an explicit tool allowlist into the session tool loop.
3. Remains serializable (YAML/JSON) for specialist agents under `.nerd/agents/`.
4. Does **not** reintroduce per-persona Go agent types.

## 2. Alignment dimensions

| Dimension | Score (0–5) | Evidence |
|-----------|-------------|----------|
| Creative/executive split | **5** | Types encode identity (LLM) + policies/tools (executive bounds); no LLM call logic in this package (`types.go`) |
| Fact-flow fidelity | **4** | Config sits after intent/prompt compile and before VirtualStore tools (`session/executor.go` `compileConfig` → `runToolLoop`); perception→kernel still own intent |
| JIT-first discipline | **5** | Package exists *because* hardcoded domain shards were deleted; comment on `EffectiveAgentRuntimeConfig` maps JIT compiler → Universal Executor |
| Constitutional safety surface | **3** | `Validate()` requires `Policies` non-empty and non-blank identity; production paths often skip `Validate()` and fall back to empty configs |
| Schema completeness vs runtime | **2** | `ToolLoop`, `Safety.RequirePolicyEnforcement`, `Model`, `Persona`, `Workspace` are populated or loadable but **not** fully enforced by `session.Executor` (executor uses `ExecutorConfig.MaxToolIterations` instead of `cfg.ToolLoop`) |
| Test grounding (package) | **4** | `types_test.go` covers valid / missing identity / whitespace identity / empty & nil policies |
| Test grounding (system) | **4** | Heavy consumer coverage in `internal/prompt` ConfigFactory tests, session/spawner tests, `tests/e2e/*` |
| Dependency hygiene | **5** | Package imports only `fmt` + `strings`; zero reverse risk of cycles into core/session |
| Observability | **2** | No logging in package (by design); consumers log tool counts under `logging.CategorySession` / `CategoryJIT` |

**Overall alignment: 3.8 / 5** — the **type contract is correct and load-bearing**; residual risk is **partial consumption of schema fields** and **lenient empty-config fallbacks** in session/spawner hot paths.

## 3. What “good” looks like (JIT-schema-specific)

| Good | Bad |
|------|-----|
| Single `EffectiveAgentRuntimeConfig` for every persona | Reintroducing `CoderShard` / per-agent Go structs |
| `Validate()` as gate before executor accept | Silent empty `{}` configs with unrestricted or zero tools |
| `AllowedTools` enforced in tool loop | LLM free to name any registered tool |
| Policy file list non-empty | Agent with zero Mangle policy files |
| Schema fields that the executor actually honors | Dead YAML knobs that look authoritative but are ignored |
| YAML tags snake_case for `.nerd/agents/*/config.yaml` | Nested historical `Tools.AllowedTools` wrappers (removed) |

## 4. Related corpora

- `Docs/architecture/prompt/` — ConfigFactory + prompt JIT compiler  
- `Docs/architecture/session/` — clean executor / spawner / subagent  
- `Docs/architecture/core/` — VirtualStore, Dreamer, `permitted(...)`  
- `Docs/architecture/cli/` — boot wires default ConfigFactory  
- `.claude/skills/codenerd-builder/references/jit-execution-model.md` — historical narrative of shard → JIT cutover (names may lag: `AgentConfig` → `EffectiveAgentRuntimeConfig`)
