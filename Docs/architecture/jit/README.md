# jit — the capability envelope for a freshly configured agent

> **VERIFIED CURRENT** on 2026-07-13 against `internal/jit/config` and its
> prompt, session, and system consumers. `corpus.toml` owns the source boundary;
> [_progress.md](_progress.md) owns test and review receipts.

## In one minute

When codeNERD turns a request such as “review this patch” into a temporary
reviewer, it needs a typed handoff between the creative instructions selected for
the model and the capabilities the runtime will enforce. `jit` owns that handoff:
`internal/jit/config/types.go#EffectiveAgentRuntimeConfig` carries identity text,
intent, tools, policy references, model and workspace hints, and tool-loop and
safety settings.

The package is deliberately tiny: one production file and one test file. Its
value comes from fan-out. Prompt code produces the value; session code consumes
it; system boot wires both sides. The visible outcome should be a specialist whose
role and capability envelope are chosen for this turn without adding another
hard-coded Go agent type.

## Its place in codeNERD

The model remains the creative center: compiled prompt atoms describe how a
reviewer, coder, or researcher should think. The Mangle kernel and effect gates
remain the executive: an allowlist never replaces `permitted/3`. `jit` is the
typed seam that lets those responsibilities meet without collapsing into each
other.

```text
perceived intent -> prompt/config selection -> EffectiveAgentRuntimeConfig
                                               |
                                               v
kernel permission <- session tool loop <- universal executor -> response
```

`internal/jit` does not own atom selection, Mangle policy content, provider
routing, tool implementations, the executor loop, or response articulation.
Those live in the `prompt`, `core`/`mangle`, `config`, `tools`, `session`, and
`articulation` corpora respectively.

## A representative journey

Consider a user asking for a code review:

1. Perception supplies a `/review` intent to the session executor.
2. `internal/prompt/compiler.go#Compile` assembles identity and methodology atoms
   under the compilation context's token budget. The config factory separately
   resolves `/review` to a tool list and policy-reference list through
   `internal/prompt/config_factory.go#Generate`.
3. Both values meet in `EffectiveAgentRuntimeConfig`. A spawned specialist may
   instead load the same YAML shape from `.nerd/agents/<name>/config.yaml` through
   `internal/session/spawner.go#loadSpecialistConfig`.
4. `internal/session/executor.go#compileConfig` accepts an injected specialist
   config or asks the factory for one. `buildToolDefinitions` presents registered
   allowlisted tools to the model.
5. Requested tools enter `internal/session/executor_tools.go#executeToolCall`,
   which checks the capability list, constitutional safety, destructive-action
   preflight, timeout, execution, and postcondition validation before returning a
   result.
6. The tool loop returns results to the model and the executor processes the
   Piggyback control packet before recording the turn.

Failure is not yet uniform. Compiler or factory failure can degrade to a baseline
prompt or empty config; specialist YAML is now bounded, path-contained, and
validated before use, and nil/empty capability lists deny execution. Several
effective fields and the factory/fallback validation contract remain incomplete.
These are current gaps, not properties of the north star.

## What exists today

| Claim | Status | Evidence |
|---|---|---|
| The schema rejects blank identity, empty policies, noncanonical/missing paths, traversal/whitespace variants, and duplicates when callers invoke validation | **VERIFIED CURRENT** | `internal/jit/config/types.go#Validate`; `internal/jit/config/types_test.go#TestAgentConfigValidation`; focused test receipt in [_progress.md](_progress.md) |
| Default prompt config atoms create one flat runtime config whose policy paths resolve against core's embedded boot inventory | **VERIFIED CURRENT** | `internal/core/policy_inventory.go#DefaultAgentPolicySetFiles`; `internal/prompt/config_factory.go#Generate`; `internal/prompt/config_policy_registry_test.go` |
| The normal modular-tool catalog is built from `AllowedTools` | **VERIFIED CURRENT** | `internal/session/executor.go#buildToolDefinitions`; owning session tests |
| Nil/empty and unlisted capabilities fail closed across modular and Ouroboros execution and catalog paths | **VERIFIED CURRENT** | `internal/session/executor_tools.go#isToolAllowed`; `internal/session/executor_tools.go#executeToolCall`; `internal/session/executor_capability_test.go#TestExecutorToolCapabilityEnvelopeFailsClosed`; `internal/session/executor_capability_test.go#TestExecutorOuroborosRegistryDoesNotGrantCapability`; focused race receipt in [_progress.md](_progress.md) |
| Specialist YAML loading has size and path-containment checks | **VERIFIED CURRENT** | `internal/session/spawner.go#loadSpecialistConfig`; `internal/session/spawner_config_test.go#TestLoadSpecialistConfigPreservesBoundaryGates` |
| Specialist YAML obeys `EffectiveAgentRuntimeConfig.Validate` | **VERIFIED CURRENT** | `internal/session/spawner.go#loadSpecialistConfig`; blank-identity and missing-policy regressions in `internal/session/spawner_config_test.go#TestLoadSpecialistConfigRejectsInvalidRuntimeConfig` |
| Policy references identify live Mangle policy members and apply them selectively per agent | **PARTIAL** | default references now resolve and validate against the global boot inventory; session does not selectively load them or carry set identity/version; `artifact:.corpus-build/findings/jit-policy-reference-drift.md` |
| Per-agent loop, failure, model, workspace, and policy-enforcement settings affect runtime | **PARTIAL** | fields exist in `types.go`; session loop bounds come from `internal/session/executor.go#ExecutorConfig`, while the remaining settings lack hot-path consumers |

### Applicability matrix

| Lane | JIT contract and evidence |
|---|---|
| Mangle | `Policies` carries canonical members resolved from stable core set IDs, but this package declares no predicates and session does not selectively load those references. The live executive still uses the global core corpus and `permitted/3`; set identity/version and selective semantics remain gap J3 in [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md). |
| Permission and safety | `AllowedTools` is a fail-closed capability gate, not authorization. Nil/empty and unlisted modular or Ouroboros requests are denied before handler execution; listed requests still pass the constitutional and interactive executive gates. |
| Fact flow | Intent is produced outside the package, selects prompt/config atoms, reaches the executor, and may cause permitted effects and articulated results. The schema itself owns no fact store or side effect. |
| JIT and agents | `EffectiveAgentRuntimeConfig` is the handoff. Prompt budgets and truncation live in `internal/prompt/context.go#AvailableTokens` and the compiler; spawned specialists inject this config through `internal/session/subagent.go#execute`. |
| Wiring | `internal/system/factory.go#initIntelligenceLayer` constructs the compiler and prompt assembler; `initFinalExecutors` constructs the default config factory, executor, and spawner. Missing compiler/factory paths degrade rather than abort. |
| State and concurrency | Config values are per agent/task and copied by pointer into an executor. `SetAgentConfig` is mutex-protected, but no immutable snapshot or version ID proves which producer made a value. The schema has no persistence of its own. |
| Recovery | JIT compilation has baseline fallback; spawner retries compilation with a reduced context; contexts cancel tool execution. Invalid specialist YAML and invalid specialist contract values return errors. Factory/compile zero-value fallback remains implicit, but it is deny-all at the tool capability gate. |
| Observability | JIT and session log compilation/config summaries and the compiler exposes stats/manifests. There is no correlated “effective capability” receipt proving field consumers, policy-set identity/version, degradation reason, or redaction. |
| Testing | Unit validation, core-inventory/provider parity, hostile spawner boundaries, fail-closed modular/Ouroboros execution and catalog paths, focused race, and broader session tests exist. Missing gates include full config-replacement snapshot semantics, policy-set-to-turn semantics, and a complete intent-to-effect integration oracle. |

## North star

Every agent turn should receive a validated, versioned, explainable capability
envelope whose identity comes from JIT prompt atoms and whose executable grants
come from deterministic executive policy. Every field must either alter a named
runtime gate or be absent; every degradation must be explicit; every named policy
must resolve to a loaded policy-set identity.

Non-goals:

- moving prompt compilation, tool implementations, or policy source into this
  schema package;
- treating the model's identity prompt or a YAML file as permission;
- adding a Go type per persona;
- enabling per-agent model/workspace overrides before precedence, containment,
  and cancellation contracts are pinned;
- retaining prompt bodies or tool payloads in observability receipts.

## Improvement frontier

The authoritative cards are in [TODO.md](TODO.md):

1. **Truth-gap repair (in progress):** specialist YAML validation and fail-closed
   modular/Ouroboros capability enforcement are verified; generated/fallback
   config validation and a typed degraded-mode contract remain.
2. **Safe leverage (in progress):** canonical policy-set registry and provider
   parity are verified; finish schema-to-consumer parity and set/version-to-turn
   semantics so no field or reference only looks enforced.
3. **North-star advance:** emit a bounded effective-capability receipt that proves
   which config version, grants, policy set, limits, and fallback reached a turn.
4. **Deferred moonshot:** compare alternative capability envelopes in a no-effect
   shadow to measure utility and denial changes before rollout.

The remaining validation boundary is first because later enforcement and
experimentation cannot be trusted while invalid generated values can cross the
seam, even though empty values can no longer grant a tool capability.

## Choose a reading route

### 90-second orientation

Read this page and the four feature cards in [TODO.md](TODO.md).

### 10-minute tour

Read [02-CURRENT-STATE.md](02-CURRENT-STATE.md),
[03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md), and
[08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md).

### Deep implementation and assurance

- Schema and field behavior: [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) and
  [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md).
- Safety and operations: [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md),
  [11-OBSERVABILITY.md](11-OBSERVABILITY.md), and
  [12-FAILURE-MODES.md](12-FAILURE-MODES.md).
- Verification and change evidence: [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md)
  and [_progress.md](_progress.md).
