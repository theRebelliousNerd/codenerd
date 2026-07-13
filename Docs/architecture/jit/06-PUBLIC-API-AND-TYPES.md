# 06 — Public API and Types: `internal/jit/config`

> Last verified against codebase: **2026-07-13**  
> Import: `codenerd/internal/jit/config`  
> Source: `internal/jit/config/types.go`

## 1. Package identity

| Item | Value |
|------|-------|
| Path | `internal/jit/config` |
| Go package name | `config` |
| Parent dir | `internal/jit/` (no root package files) |
| External deps | `fmt`, `strings` only |

**Caution:** Other packages also use the name `config` (e.g. `internal/config` for user/engine configuration). Always import with a path and alias if both are needed:

```go
import (
    usercfg "codenerd/internal/config"
    jitcfg "codenerd/internal/jit/config"
)
```

Session and prompt import as:

```go
"codenerd/internal/jit/config"
// then config.EffectiveAgentRuntimeConfig
```

## 2. Types

### 2.1 `EffectiveAgentRuntimeConfig`

Primary DTO for a JIT-driven dynamic agent.

| Field | Type | Tags | Required by Validate? |
|-------|------|------|------------------------|
| `IdentityPrompt` | `string` | `yaml:"identity_prompt" json:"identity_prompt"` | **Yes** (non-whitespace) |
| `IntentVerb` | `string` | `yaml:"intent_verb" json:"intent_verb"` | No |
| `Persona` | `string` | `yaml:"persona" json:"persona"` | No |
| `AllowedTools` | `[]string` | `yaml:"allowed_tools" json:"allowed_tools"` | No |
| `Policies` | `[]string` | `yaml:"policies" json:"policies"` | **Yes** (len ≥ 1) |
| `Model` | `string` | `yaml:"model" json:"model"` | No |
| `ToolLoop` | `ToolLoopConfig` | `yaml:"tool_loop" json:"tool_loop"` | No |
| `Safety` | `SafetyConfig` | `yaml:"safety" json:"safety"` | No |
| `Workspace` | `WorkspaceConfig` | `yaml:"workspace" json:"workspace"` | No |

Comment (source): maps JIT compiler output to Universal Executor; YAML tags for `.nerd/agents/<name>/config.yaml`.

### 2.2 `ToolLoopConfig`

| Field | Type | Tags | Factory default (prompt) |
|-------|------|------|---------------------------|
| `MaxIterations` | `int` | `max_iterations` | `5` |
| `MaxTotalCalls` | `int` | `max_total_calls` | `50` |
| `FailOnToolError` | `bool` | `fail_on_tool_error` | `false` |

**Runtime note:** `session.Executor` currently uses `ExecutorConfig.MaxToolIterations` / `MaxToolCalls` (defaults 8 / budget), not these fields.

### 2.3 `SafetyConfig`

| Field | Type | Tags | Factory default |
|-------|------|------|-----------------|
| `RequirePolicyEnforcement` | `bool` | `require_policy_enforcement` | `true` |

**Runtime note:** No session branch observed on this flag as of 2026-07-13.

### 2.4 `WorkspaceConfig`

| Field | Type | Tags |
|-------|------|------|
| `RootPath` | `string` | `root_path` |

Optional specialist scoping; process workspace is usually set by CLI.

## 3. Methods

### 3.1 `func (c EffectiveAgentRuntimeConfig) Validate() error`

**Value receiver** — does not mutate.

**Errors (exact prefixes):**

- `"config validation failed: identity_prompt is required"`  
- `"config validation failed: at least one policy file is required"`  

**Success:** `nil`.

**Not checked:** nil vs empty `AllowedTools`, tool name syntax, policy file existence on disk, model validity, tool loop bounds, workspace path existence.

## 4. Non-API (intentionally absent)

| Missing API | Rationale |
|-------------|-----------|
| `New…` constructors | Zero value + factory + YAML |
| `Merge` | Merging lives on `prompt.ConfigAtom` |
| `Clone` | Callers copy fields if needed; slices share if not careful |
| Interfaces | Consumers define `ConfigFactory` interface in session package |
| Context parameters | Pure validation |

## 5. Consumer-facing interface (defined elsewhere)

`internal/session` defines:

```go
type ConfigFactory interface {
    Generate(ctx context.Context, result *prompt.CompilationResult, intents ...string) (*config.EffectiveAgentRuntimeConfig, error)
}
```

Concrete implementation: `*prompt.ConfigFactory` (`internal/prompt/config_factory.go`).

E2E mocks sometimes also declare `RegisterSpecialist` — **not** on the production `prompt.ConfigFactory` API as the core path; treat as test interface widening.

## 6. Related result type (prompt package)

```go
// internal/prompt/compiler.go
type CompilationResult struct {
    // ...
    EffectiveAgentRuntimeConfig *config.EffectiveAgentRuntimeConfig
}
```

Optional attachment when compiler is constructed with `WithConfigFactory`.

## 7. Stability guidance

| Change | Compatibility impact |
|--------|----------------------|
| Add optional field with zero default | Low |
| Rename YAML tags | **Breaking** for specialist files |
| Stricter Validate | May break custom YAML agents |
| Nesting under Tools/Policies wrappers | **Breaking** (already removed once) |
