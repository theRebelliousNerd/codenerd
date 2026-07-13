# JIT capability envelope — current state

> **VERIFIED CURRENT** on 2026-07-13. This is a source-grounded field report,
> not the broader prompt compiler design.

## Package boundary and inventory

`corpus.toml` owns `internal/jit`. There is no package at the directory root;
production imports `codenerd/internal/jit/config`.

| Path | Role | State |
|---|---|---|
| `internal/jit/config/types.go` | flat YAML/JSON runtime config, nested loop/safety/workspace values, and validation | implemented |
| `internal/jit/config/types_test.go` | table tests for identity, empty/nil policies, missing aliases, traversal, and duplicates | implemented |

The package has one non-test Go file, one test file, no Mangle source, no I/O,
no constructor, no goroutine, and no package-owned persistent state.

## Exported data contract

`internal/jit/config/types.go#EffectiveAgentRuntimeConfig` holds identity prompt,
intent verb, persona, allowed tools, policy references, model hint, tool-loop
settings, a policy-enforcement flag, and workspace root. Its nested values are
`ToolLoopConfig`, `SafetyConfig`, and `WorkspaceConfig`.

`internal/jit/config/types.go#Validate` proves these policy and identity
invariants:

1. identity is non-empty after whitespace trimming;
2. at least one string appears in `Policies`;
3. every policy is a canonical path in core's embedded default policy inventory;
4. policy references are unique.

It deliberately does not validate tools, model, limits, safety flag, workspace,
intent, persona, or prove selective per-agent policy loading. It does reject
aliases, missing modules, traversal, whitespace variants, and duplicates through
`internal/core/policy_inventory.go#IsDefaultPolicyFile`. Validation still has no
effect unless a caller invokes it.

## Producers

| Producer | Current behavior | Evidence status |
|---|---|---|
| Default config factory | merges intent atoms, assigns prompt identity, tools, canonical policy paths resolved from stable set IDs, loop defaults, and `RequirePolicyEnforcement=true` | **VERIFIED CURRENT** — `internal/prompt/config_factory.go#Generate`; `NewDefaultConfigAtomProvider` |
| Compiler attachment | may attach a generated config to `CompilationResult` when a ConfigFactory is registered | **VERIFIED CURRENT** — `internal/prompt/compiler.go#Compile` |
| Specialist YAML | path-contained, one-megabyte-bounded YAML unmarshal followed by `Validate` from `.nerd/agents/<name>/config.yaml` | **VERIFIED CURRENT** — `internal/session/spawner.go#loadSpecialistConfig`; `internal/session/spawner_config_test.go#TestLoadSpecialistConfigRejectsInvalidRuntimeConfig`; `internal/session/spawner_config_test.go#TestLoadSpecialistConfigPreservesBoundaryGates` |
| Fallbacks | missing compiler/factory or failed generation can return baseline prompt or zero-value config | **VERIFIED CURRENT** — `internal/session/executor.go#compileConfig`; `internal/session/spawner.go#generateConfig` |

## Consumers and actual field use

| Field | Live consumer | Current verdict |
|---|---|---|
| `IdentityPrompt` | factory sets it and SubAgent injects config, but executor still passes the compiler result prompt separately | **PARTIAL** dual-source contract |
| `IntentVerb` | factory metadata and SubAgent preset intent | **PARTIAL** routing support, not an authority gate |
| `AllowedTools` | modular and Ouroboros catalog/execution allowlist check | **VERIFIED CURRENT** nil/empty/unlisted deny before registry execution; listed tools continue to downstream safety gates |
| `Policies` | validated canonical members of core's globally loaded default inventory | **PARTIAL** references resolve, but no set ID/version or selective per-agent loader reaches session |
| `ToolLoop` | populated by factory | **PARTIAL** `runToolLoop` uses `ExecutorConfig` limits instead |
| `Safety.RequirePolicyEnforcement` | populated true by factory | **PARTIAL** no session branch reads it |
| `Model` | no primary session consumer | **PROPOSED UPLIFT** if activated later |
| `Workspace.RootPath` | no primary capability/sandbox consumer | **PROPOSED UPLIFT** if activated later |
| `Persona` | optional YAML metadata | **PARTIAL** no enforcement consumer |

## End-to-end current route

1. `internal/system/factory.go#initIntelligenceLayer` constructs the JIT prompt
   compiler and connects it to the prompt assembler and shard-manager lifecycle.
2. `internal/system/factory.go#initFinalExecutors` constructs one default config
   factory and injects it with the compiler into Executor and Spawner.
3. Executor builds a compilation context from the perceived intent and session
   context, compiles prompt atoms, then calls `compileConfig`.
4. `buildToolDefinitions` resolves allowlisted modular tools for the model.
5. `runToolLoop` executes bounded turns using `ExecutorConfig`, not `ToolLoop`.
6. `executeToolCall` checks the JIT allowlist condition, constitutional gate,
   destructive preflight, timeout, registry, and postcondition validator.
7. Articulation processes the returned Piggyback envelope; JIT config does not
   author the response.

## Applicability and limitations

| Lane | Current state |
|---|---|
| Mangle | **PARTIAL:** schema carries canonical members of core's embedded boot inventory; no local predicates, set identity/version, or per-agent loading. Core policy remains global. |
| Permission and safety | **VERIFIED CURRENT for the capability slice:** nil/empty/unlisted modular and Ouroboros tools fail closed; constitutional checks remain independently downstream. Policy-reference enforcement remains partial. |
| Fact flow | **VERIFIED CURRENT:** intent feeds config selection and executor; package itself owns no facts or effects. |
| JIT and agents | **VERIFIED CURRENT:** shared config reaches spawned agents; token fitting belongs to prompt compiler. |
| Wiring | **VERIFIED CURRENT:** Cortex boot wires compiler, factory, executor, and spawner; degraded paths remain. |
| State and concurrency | **PARTIAL:** executor setter is locked, but config provenance/version and immutable turn snapshot are not explicit. |
| Recovery | **PARTIAL:** invalid specialist configs fail with path-qualified errors and empty fallbacks deny tools, but compile retries and baseline/zero-value fallbacks are not represented as typed degradation. |
| Observability | **PARTIAL:** logs and compiler stats exist; no effective-capability receipt spans producer to effect. |
| Testing | **PARTIAL:** strong unit/factory coverage plus hostile specialist, policy-inventory/provider parity, fail-closed modular/Ouroboros, catalog, and focused race regressions. Complete field-consumer and intent-to-effect coverage remain absent. |

## Repaired findings and remaining technical debt

- `artifact:.corpus-build/findings/jit-specialist-config-validation-bypass.md`
  records the now-repaired loader bug; current code calls `Validate` and has
  negative blank-identity and missing-policy regressions.
- `artifact:.corpus-build/findings/jit-empty-config-capability-bypass.md` records
  the now-repaired capability bug; nil/empty lists deny and Ouroboros registry
  membership is not a grant.
- `artifact:.corpus-build/findings/jit-policy-reference-drift.md` is **RESOLVED**
  for canonical references and validation. The explicitly retained residual is
  selective per-agent loading and turn-level policy-set identity/version.
- Config generation and compiler attachment can both produce a config, while
  executor may independently call the factory; precedence is implicit.
- Existing skill prose still describes historical prompt and config shapes. It
  is design input, not evidence of live behavior.

## Current test evidence

The package test and focused session consumers, including race instrumentation,
passed in the bounded receipt recorded in [_progress.md](_progress.md). That
receipt proves the repaired specialist and capability slices. Core-inventory and
prompt-provider tests separately prove default policy resolution; the combined
evidence does not prove uniform factory/fallback validation, selective per-agent
loading, or typed degradation.
