# 05 — Internal Architecture: JIT config

> Last verified against codebase: **2026-07-13**  
> Source of truth: `internal/jit/config/types.go`

## 1. Component model

This package has a single component: **runtime configuration value types**.

```
┌─────────────────────────────────────────────────────────┐
│                  internal/jit/config                    │
│                                                         │
│  EffectiveAgentRuntimeConfig                            │
│    ├── IdentityPrompt, IntentVerb, Persona, Model       │
│    ├── AllowedTools []string                            │
│    ├── Policies []string                                │
│    ├── ToolLoopConfig                                   │
│    ├── SafetyConfig                                     │
│    └── WorkspaceConfig                                  │
│                                                         │
│  Validate() → error                                     │
└─────────────────────────────────────────────────────────┘
```

No state machines, caches, or goroutines.

## 2. Type graph

```
EffectiveAgentRuntimeConfig
├── string fields: IdentityPrompt, IntentVerb, Persona, Model
├── []string: AllowedTools, Policies
├── ToolLoopConfig
│   ├── MaxIterations int
│   ├── MaxTotalCalls int
│   └── FailOnToolError bool
├── SafetyConfig
│   └── RequirePolicyEnforcement bool
└── WorkspaceConfig
    └── RootPath string
```

## 3. Data flow (ecosystem, not local)

### 3.1 Happy path — interactive / task executor

```
user input
  → perception.Intent{Verb}
  → prompt.JITPromptCompiler.Compile → CompilationResult{Prompt}
  → prompt.ConfigFactory.Generate(result, intentVerb)
       → EffectiveAgentRuntimeConfig{
            IdentityPrompt: result.Prompt,
            IntentVerb: intentVerb,
            AllowedTools: atom.Tools,
            Policies: atom.Policies,
            ToolLoop: {5, 50, false},
            Safety: {RequirePolicyEnforcement: true},
          }
  → session.Executor.runToolLoop(..., cfg)
       → buildToolDefinitions(cfg.AllowedTools)
       → isToolAllowed(name, cfg) before execute
```

### 3.2 Precompiled / SubAgent path

```
Spawner.generateConfig / loadSpecialistConfig
  → SubAgentConfig.EffectiveAgentRuntimeConfig
  → SubAgent.Execute → executor.SetAgentConfig(cfg)
  → compileConfig short-circuits to injected cfg
```

### 3.3 Specialist YAML path

```
.nerd/agents/<name>/config.yaml
  → VirtualStore.ReadRaw / os.ReadFile
  → yaml.Unmarshal → EffectiveAgentRuntimeConfig
  → Validate (path-qualified rejection on failure)
  → SubAgent
```

### 3.4 Compiler optional attach

When `JITPromptCompiler` has a non-nil `configFactory`, Compile may set:

```
CompilationResult.EffectiveAgentRuntimeConfig = agentCfg
```

Session may still call ConfigFactory again via `compileConfig` unless a SubAgent injected config is present.

## 4. Validation state machine (conceptual)

```
                ┌──────────────┐
                │  Constructed │
                └──────┬───────┘
                       │
          ┌────────────┼────────────┐
          ▼                         ▼
   Validate() == nil         Validate() != nil
   "full / acceptable"       "reject or degrade"
          │                         │
          ▼                         ▼
   Executor with tools        Empty fallback /
   + identity                 explicit error
```

Specialist YAML follows the reject branch. Generated and compiler fallback paths
can still jump to an empty fallback **without** `Validate`; that value is deny-all
at session's capability gate rather than unrestricted.

## 5. Serialization contract

The YAML below illustrates the **current serialized shape and canonical factory
paths**, not a selectively loaded per-agent constitution. Default factories
resolve stable core set IDs to members of the embedded boot inventory.

| Tag style | Example |
|-----------|---------|
| YAML | `identity_prompt: "…"` |
| JSON | `"identity_prompt":"…"` |

Nested:

```yaml
identity_prompt: "You are a careful reviewer."
intent_verb: "/review"
allowed_tools:
  - read_file
  - grep
policies:
  - policy/constitution.mg
  - policy/validation.mg
  - reviewer.mg
tool_loop:
  max_iterations: 5
  max_total_calls: 50
  fail_on_tool_error: false
safety:
  require_policy_enforcement: true
workspace:
  root_path: ""
```

## 6. Design rationale (from comments)

1. Maps **JIT compiler output** to **Universal Executor** input.  
2. Snake_case YAML for natural specialist files.  
3. Validate identity plus non-empty, unique, canonical embedded policies; other
   fields have “safe zero values or downstream defaults.”

## 7. Boundary with sibling systems

| Boundary | Rule |
|----------|------|
| → prompt | Factory owns ConfigAtoms and defaults |
| → session | Executor owns loop limits today; should converge on ToolLoop or document split |
| → tools | Names in AllowedTools must match registry keys; nil/empty/unlisted deny before modular or Ouroboros execution |
| → mangle | Default policy strings are canonical embedded paths resolved from stable registry set IDs; they identify global boot-corpus members but are not selectively loaded here |
| → core | Constitutional `permitted(...)` still applies after tool allowlist |
