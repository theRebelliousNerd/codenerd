# 02 — Current State: `internal/jit`

> Last verified against codebase: **2026-07-13**  
> Status: Living inventory — code-grounded

## 1. Package layout

```
internal/jit/
  config/
    types.go       # EffectiveAgentRuntimeConfig + nested configs + Validate
    types_test.go  # Validate table tests
```

There is **no** Go package at `internal/jit` root. Import path used in production:

```go
import "codenerd/internal/jit/config"
```

## 2. File inventory

| Path | Lines (approx) | Role |
|------|---------------:|------|
| `internal/jit/config/types.go` | 59 | All types + `Validate` |
| `internal/jit/config/types_test.go` | 67 | Unit tests for `Validate` |

**Counts:** 1 non-test source · 1 test · 0 `.mg` · 0 YAML · 0 README in package.

## 3. Exported surface (complete)

| Symbol | Kind | File |
|--------|------|------|
| `EffectiveAgentRuntimeConfig` | struct | `types.go` |
| `ToolLoopConfig` | struct | `types.go` |
| `SafetyConfig` | struct | `types.go` |
| `WorkspaceConfig` | struct | `types.go` |
| `(EffectiveAgentRuntimeConfig).Validate` | method | `types.go` |

No constructors, no interfaces, no package-level vars, no init.

## 4. Field inventory with live consumption

| Field | YAML / JSON | Produced by | Consumed by (hot path) |
|-------|-------------|-------------|-------------------------|
| `IdentityPrompt` | `identity_prompt` | ConfigFactory from `CompilationResult.Prompt`; specialist YAML | SubAgent logs tool count; YAML round-trip tests; **system prompt often uses `compileResult.Prompt` separately in executor** |
| `IntentVerb` | `intent_verb` | ConfigFactory primary intent | Metadata; not gate for tools |
| `Persona` | `persona` | Optional YAML | No strong session enforcement found |
| `AllowedTools` | `allowed_tools` | ConfigAtom tools | **`isToolAllowed`, `buildToolDefinitions`, piggyback catalog** |
| `Policies` | `policies` | ConfigAtom policies (+ `base.mg`) | Required by `Validate`; factory always sets; **kernel load-from-this-slice not in session hot path** |
| `Model` | `model` | Optional YAML | Not primary session model selection (global LLM client) |
| `ToolLoop.MaxIterations` | `tool_loop.max_iterations` | Factory default `5` | **Executor uses `ExecutorConfig.MaxToolIterations` (default 8)** |
| `ToolLoop.MaxTotalCalls` | `tool_loop.max_total_calls` | Factory default `50` | **Executor uses `ExecutorConfig.MaxToolCalls`** |
| `ToolLoop.FailOnToolError` | `tool_loop.fail_on_tool_error` | Factory default `false` | Not read by `runToolLoop` (heuristic on empty response + tool errs) |
| `Safety.RequirePolicyEnforcement` | `safety.require_policy_enforcement` | Factory default `true` | No direct branch found on this flag in session |
| `Workspace.RootPath` | `workspace.root_path` | Optional YAML | Workspace selection is primarily CLI/`--workspace` / process CWD |

## 5. Validation rules (as implemented)

From `Validate()` in `types.go`:

1. `strings.TrimSpace(IdentityPrompt) != ""` — else error  
2. `len(Policies) > 0` — else error  

**Explicitly not validated:** `AllowedTools`, `Model`, `ToolLoop`, `Safety`, `Workspace`, `IntentVerb`, `Persona`.

## 6. Downstream import graph (non-test production)

| Importer | Role |
|----------|------|
| `internal/prompt` | Constructs configs (`config_factory.go`, attaches on `CompilationResult` in `compiler.go`) |
| `internal/session` | Holds, injects, filters tools (`executor.go`, `executor_tools.go`, `spawner.go`, `subagent.go`) |

## 7. Downstream import graph (tests / e2e)

| Importer | Role |
|----------|------|
| `internal/jit/config/types_test.go` | Package unit tests |
| `internal/prompt/*_test.go` | Factory Validate pass-through |
| `internal/session/*_test.go` | Mocks returning empty or partial configs |
| `tests/e2e/*` | Integration boundaries (specialist YAML, orchestrator, campaign, piggyback, race) |

## 8. Hotspots

| Hotspot | Why |
|---------|-----|
| `Validate` vs empty fallbacks | Session/spawner frequently return `&EffectiveAgentRuntimeConfig{}` on errors — fails Validate if ever checked |
| Schema / executor loop knobs | Dual sources of truth for iteration limits |
| Policy list | Present for constitution narrative; runtime enforcement is kernel-global + tool gates, not config.Policies loader |
| Skill docs | `jit-execution-model.md` still shows older nested `AgentConfig` shapes in places |

## 9. Status summary

| Aspect | State |
|--------|-------|
| Schema package | **Implemented / stable** |
| Factory population | **Implemented** (`internal/prompt`) |
| Tool allowlist enforcement | **Implemented** (`internal/session`) |
| Full field enforcement | **Partial** |
| Local Mangle | **N/A** (policy *names* only) |
| Pre-implementation | **No** — living code since domain-shard removal |
