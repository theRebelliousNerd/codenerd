# 08 — Wiring and Integration: JIT config

> Last verified against codebase: **2026-07-13**  
> How `EffectiveAgentRuntimeConfig` is produced, injected, and enforced

## 1. Boot wiring (CLI / chat)

**Source:** `cmd/nerd/chat/session_boot.go` (and shared boot paths)

Pattern:

1. Construct `prompt.NewDefaultConfigFactory()` (default ConfigAtoms).  
2. Pass factory into session Executor / Spawner / task executor construction.  
3. Optional: attach factory to JIT compiler via `prompt.WithConfigFactory`.

Campaign CLI also constructs factories (`cmd/nerd/cmd_campaign.go`).

Domain shards are **not** registered for coder/tester/etc.; comments in boot note migration to JIT clean loop.

## 2. Production producers

### 2.1 `prompt.ConfigFactory.Generate`

**File:** `internal/prompt/config_factory.go`

| Step | Behavior |
|------|----------|
| Nil result | Error |
| Empty intents | Error |
| Lookup ConfigAtoms by intent | Merge tools/policies; `/consult/*` falls back to `/general` |
| No atoms found | Error |
| Success | Fill IdentityPrompt from `result.Prompt`, tools/policies from atom, ToolLoop/Safety defaults |

`GenerateFallback` builds a minimal config when JIT compilation fails; still prefers intent atom or `/general`.

### 2.2 `prompt.JITPromptCompiler` (optional)

**File:** `internal/prompt/compiler.go`

If `configFactory != nil` and intents present on context, sets `result.EffectiveAgentRuntimeConfig`.

### 2.3 Specialist YAML

**File:** `internal/session/spawner.go` → `loadSpecialistConfig`

| Step | Behavior |
|------|----------|
| Name validation | Reject `..` and `/` `\` |
| Path | `.nerd/agents/<name>/config.yaml` |
| Size cap | 1 MiB (`maxSpecialistConfigSize`) |
| Parse | `yaml.Unmarshal` into `EffectiveAgentRuntimeConfig` |
| Validate | Called after unmarshal; failures include the specialist config path |
| Missing file | Fall back to ConfigFactory with `"/"+name` and empty prompt shell |

## 3. Production consumers

### 3.1 `session.Executor`

| Method / site | Use of config |
|---------------|---------------|
| `SetAgentConfig` | Inject precompiled config (SubAgent) |
| `compileConfig` | Prefer injected; else `configFactory.Generate` |
| Execute failure path | Empty `&EffectiveAgentRuntimeConfig{}` on error |
| `runToolLoop` | Pass cfg for tool defs + allowlist |
| `isToolAllowed` | Membership in `AllowedTools`; nil/empty/unlisted deny |
| `buildToolDefinitions` | Resolve names via tool registry |
| `buildToolCatalogForPiggyback` | Catalog filtered by allowlist |

**Not consumed from cfg:** ToolLoop limits (uses `ExecutorConfig`), Policies application, Model, Workspace, Safety flag.

### 3.2 `session.SubAgent`

Holds `EffectiveAgentRuntimeConfig` on `SubAgentConfig`; injects into executor before process.

### 3.3 `session.Spawner`

`generateConfig` compiles prompt then Generate; on failures returns empty config after warns.

### 3.4 `session` task executor / JIT executor naming

`TaskRequest.IntentVerb` drives spawn/inline execution which eventually hits the same config path.

## 4. Fact-flow placement

```
user input
  → perception (user_intent facts / Intent)
  → kernel (persona / next_action / permitted — executive)
  → JIT prompt compile (creative framing)
  → ConfigFactory → EffectiveAgentRuntimeConfig   ← THIS PACKAGE
  → VirtualStore / tools.Global (effect)
  → articulation / response
```

Constitutional safety is **not** replaced by this schema. Tool allowlist is an **additional** gate before effectful tools. `permitted(...)` and Dreamer interactive gates remain in core/session.

## 5. Registration hooks

| Hook | Present? |
|------|----------|
| Mangle Decl for this package | No (no local .mg) |
| VirtualStore action route | No |
| Shard registration | No (replaced by JIT) |
| CLI cobra subcommand solely for schema | No (specialists via spawn/consult) |

## 6. Integration test entry points

| Test area | Path pattern |
|-----------|--------------|
| Specialist YAML boundary | `tests/e2e/specialist_config_boundary_test.go` |
| Session clean loop | `tests/e2e/session_clean_loop_integration_test.go` |
| Orchestrator + race | `tests/e2e/orchestrator_executor_*` |
| Campaign | `tests/e2e/campaign_session_integration_test.go` |
| Cross boundary | `tests/e2e/cross_boundary_integration_test.go` |

## 7. Wiring gaps (actionable)

1. Factory/generated fallback → uniform validation or an explicit typed degraded state
2. `ToolLoop` → executor loop  
3. `Policies` → carry stable set identity/version and define global membership
   versus selective per-agent loading; canonical inventory resolution is verified
4. `RequirePolicyEnforcement` → give the flag a real session meaning; canonical
   reference validation currently applies regardless of the flag
5. Deny-all empty fallback → structured reason/degrade mode + metrics

Specialist YAML validation and modular/Ouroboros capability intersection are
verified current and are no longer wiring gaps.

See `03-GAP-ANALYSIS.md` and `TODO.md`.
