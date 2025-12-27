# internal/jit/

JIT (Just-In-Time) Configuration - The bridge between declarative config and runtime execution.

**Architecture Version:** 2.0.0 (December 2024 - JIT-Driven)

---

## Overview

The `jit` package defines the core configuration schema for codeNERD's JIT-driven agent architecture. It provides the `AgentConfig` type—the unified configuration object that bridges the gap between:

1. **Declarative Configuration** (Mangle rules + YAML atoms)
2. **Runtime Execution** (Session Executor + SubAgents)

### Philosophy

> **"The config is the agent."**

In the new architecture, there is no `CoderShard` class or `TesterShard` struct. Instead, an `AgentConfig` object defines everything about an agent's behavior:
- **What it is** (identity prompt)
- **What it can do** (tools)
- **What it must obey** (policies)
- **How it operates** (mode)

---

## Architecture Context

```
User Input → Intent Routing → ConfigFactory → JIT Compiler
                                    ↓              ↓
                              ConfigAtom    Prompt Atoms
                                    ↓              ↓
                              Tools +        Identity +
                              Policies       Capabilities
                                    ↓              ↓
                                    └──────┬───────┘
                                           ↓
                                    **AgentConfig**
                                           ↓
                                   Session Executor
                                           ↓
                                   LLM + VirtualStore
```

The `AgentConfig` is the output of the JIT compilation process and the input to the Session Executor. It's the "compiled contract" for an agent's behavior.

---

## Core Types

### AgentConfig

**Definition:**
```go
type AgentConfig struct {
    // IdentityPrompt is the system prompt that defines the agent's persona and mission.
    IdentityPrompt string `json:"identity_prompt"`

    // Tools defines the set of tools this agent is permitted to use.
    Tools ToolSet `json:"tools"`

    // Policies defines the Mangle logic files that govern this agent's behavior.
    Policies PolicySet `json:"policies"`

    // Mode defines the execution mode (e.g., "SingleTurn", "Campaign").
    Mode string `json:"mode,omitempty"`
}
```

**Purpose:**
- Represents the complete configuration for a dynamically-created agent
- Replaces hardcoded shard implementations with declarative config
- Ensures deterministic, logic-driven agent behavior

**Fields:**

| Field | Type | Purpose | Example |
|-------|------|---------|---------|
| `IdentityPrompt` | `string` | JIT-compiled system prompt defining persona | "You are a code fixer specialized in null pointer bugs..." |
| `Tools` | `ToolSet` | Allowed tools (enforced by executor) | `["file_read", "file_write", "git"]` |
| `Policies` | `PolicySet` | Mangle policy files to load | `["code_safety.mg", "git_workflow.mg"]` |
| `Mode` | `string` | Execution mode | `"SingleTurn"`, `"Campaign"` |

**Validation:**
```go
func (c AgentConfig) Validate() error {
    if strings.TrimSpace(c.IdentityPrompt) == "" {
        return fmt.Errorf("identity_prompt is required")
    }

    if len(c.Policies.Files) == 0 {
        return fmt.Errorf("at least one policy file is required")
    }

    return nil
}
```

**Usage:**
```go
config := &config.AgentConfig{
    IdentityPrompt: "You are a code reviewer...",
    Tools: config.ToolSet{
        AllowedTools: []string{"file_read", "git", "hypothesis_gen"},
    },
    Policies: config.PolicySet{
        Files: []string{"review_policy.mg", "code_safety.mg"},
    },
    Mode: "SingleTurn",
}

if err := config.Validate(); err != nil {
    log.Fatal(err)
}

// Pass to executor
executor.ProcessWithConfig(ctx, input, config)
```

---

### ToolSet

**Definition:**
```go
type ToolSet struct {
    AllowedTools []string `json:"allowed_tools"`
}
```

**Purpose:**
- Defines which tools an agent is permitted to use
- Enforced at execution time by the Session Executor
- Prevents tool hallucination and unauthorized operations

**Tool Enforcement:**
```go
// In executor.go
func (e *Executor) isToolAllowed(toolName string, cfg *config.AgentConfig) bool {
    if cfg == nil || len(cfg.Tools.AllowedTools) == 0 {
        return true // No restrictions
    }

    for _, allowed := range cfg.Tools.AllowedTools {
        if allowed == toolName {
            return true
        }
    }
    return false // Blocked
}
```

**Example Tool Sets:**

| Persona | Tools |
|---------|-------|
| **Coder** | `file_read`, `file_write`, `shell_exec`, `git`, `build_check` |
| **Tester** | `file_read`, `test_exec`, `coverage_analyzer`, `mock_gen` |
| **Reviewer** | `file_read`, `hypothesis_gen`, `impact_analysis`, `preflight` |
| **Researcher** | `web_fetch`, `doc_parse`, `kb_ingest`, `context7` |

---

### PolicySet

**Definition:**
```go
type PolicySet struct {
    Files []string `json:"files"`
}
```

**Purpose:**
- Specifies which Mangle policy files govern this agent's behavior
- Loaded into the kernel at runtime
- Provides declarative safety and workflow rules

**Policy File Examples:**

| Policy File | Purpose |
|-------------|---------|
| `code_safety.mg` | Prevents destructive operations (rm -rf, etc.) |
| `git_workflow.mg` | Enforces proper git usage (branch checks, commit rules) |
| `test_strategy.mg` | TDD repair loop rules (test → fail → fix → retest) |
| `review_policy.mg` | Code review workflow (hypothesis generation, verification) |
| `research_strategy.mg` | Knowledge ingestion and quality scoring |

**Policy Loading:**
```go
// In kernel initialization
for _, policyFile := range config.Policies.Files {
    content, _ := os.ReadFile("internal/mangle/" + policyFile)
    kernel.LoadPolicy(string(content))
}
```

---

## How AgentConfig is Created

### 1. Intent Routing (Mangle Logic)

When a user provides input like "Fix the bug in auth.go", the system:

```mangle
# internal/mangle/intent_routing.mg
persona(/coder) :- user_intent(_, _, /fix, _, _).
```

Result: `persona(/coder)` fact is derived.

### 2. ConfigAtom Retrieval

The `ConfigFactory` queries for the ConfigAtom associated with `/coder`:

```go
// internal/prompt/config_factory.go
type ConfigAtom struct {
    Tools    []string  // Tools needed for this intent
    Policies []string  // Policy files to load
    Priority int       // Merge priority
}

// Retrieval:
atom := provider.GetAtom("/coder")
// Returns: ConfigAtom{
//     Tools: ["file_read", "file_write", "shell_exec", "git"],
//     Policies: ["code_safety.mg", "git_workflow.mg"],
//     Priority: 10,
// }
```

### 3. JIT Prompt Compilation

The `JITPromptCompiler` assembles the identity prompt:

```go
// internal/prompt/compiler.go
compileResult, _ := jitCompiler.Compile(ctx, compilationCtx)

// Returns:
// CompilationResult{
//     Prompt: "You are a code fixer. Your mission is to...",
//     SkeletonAtoms: [...],
//     FleshAtoms: [...],
// }
```

### 4. Final Assembly

The `ConfigFactory` merges everything into an `AgentConfig`:

```go
agentConfig := &config.AgentConfig{
    IdentityPrompt: compileResult.Prompt,
    Tools: config.ToolSet{
        AllowedTools: configAtom.Tools,
    },
    Policies: config.PolicySet{
        Files: configAtom.Policies,
    },
    Mode: "SingleTurn",
}
```

### Complete Flow

```
User: "Fix the bug in auth.go"
  ↓
Transducer → user_intent(_, _, /fix, "auth.go", _)
  ↓
Intent Routing → persona(/coder)
  ↓
ConfigFactory.GetAtom("/coder") → ConfigAtom{Tools: [...], Policies: [...]}
  ↓
JITCompiler.Compile(ctx, {IntentVerb: "/fix"}) → "You are a code fixer..."
  ↓
ConfigFactory.Generate(ctx, compileResult, "/fix") → **AgentConfig**
  ↓
Executor.ProcessWithConfig(ctx, input, agentConfig)
```

---

## Usage Patterns

### 1. Single-Turn Execution

```go
// Standard flow: JIT generates config
result, err := executor.Process(ctx, "Fix the null pointer in auth.go")
```

Internally, this:
1. Parses intent
2. Generates `AgentConfig` via JIT
3. Executes with config

### 2. Explicit Config Override

```go
// User wants to force specific tools
customConfig := &config.AgentConfig{
    IdentityPrompt: "You are a read-only auditor.",
    Tools: config.ToolSet{
        AllowedTools: []string{"file_read"}, // No write tools
    },
    Policies: config.PolicySet{
        Files: []string{"audit_only.mg"},
    },
    Mode: "SingleTurn",
}

result, err := executor.ProcessWithConfig(ctx, input, customConfig)
```

### 3. Persistent Agent with Custom Config

```go
// Load specialist config from filesystem
config, _ := loadSpecialistConfig(ctx, "django-expert")

// Spawn with loaded config
spawner.Spawn(ctx, SpawnRequest{
    Name:       "django-expert",
    Task:       "Optimize ORM queries",
    Type:       SubAgentTypePersistent,
    AgentConfig: config, // Pre-loaded
})
```

---

## Comparison: Old vs New

### Old Architecture (Hardcoded Shards)

```go
// internal/shards/coder/coder.go (~600 lines)
type CoderShard struct {
    // Hardcoded tools
    allowedTools []string{"file_read", "file_write", "shell_exec", "git"}

    // Hardcoded policies
    policies []string{"code_safety.mg", "git_workflow.mg"}

    // Hardcoded system prompt
    systemPrompt = "You are a code generation expert..."
}

func (s *CoderShard) Execute(ctx context.Context, task ShardTask) (string, error) {
    // 400 lines of hardcoded logic...
}
```

**Problems:**
- Behavior changes require code modifications
- Adding new agent types requires 500-2000 lines of Go
- Tool/policy selection is rigid
- No runtime configurability

### New Architecture (JIT-Driven Config)

```go
// internal/jit/config/types.go (~46 lines total)
type AgentConfig struct {
    IdentityPrompt string   // JIT-compiled
    Tools          ToolSet   // Declarative
    Policies       PolicySet // Mangle files
    Mode           string    // Runtime-defined
}
```

**Benefits:**
- Behavior changes via YAML atoms or Mangle rules (no recompile)
- New agents via declarative config (zero Go code)
- Tool/policy selection via Mangle logic
- Full runtime configurability

---

## Validation & Safety

### Config Validation

```go
if err := agentConfig.Validate(); err != nil {
    return fmt.Errorf("invalid config: %w", err)
}
```

**Checks:**
- Identity prompt is not empty
- At least one policy file is specified

**Why validation matters:**
- Prevents agents from running without safety policies
- Ensures all agents have a defined identity
- Catches config errors early (fail-fast)

### Tool Enforcement

Tools in `AgentConfig.Tools.AllowedTools` are enforced at execution time:

```go
// Before executing any tool call
if !executor.isToolAllowed(toolCall.Name, agentConfig) {
    return fmt.Errorf("tool %s not allowed by config", toolCall.Name)
}
```

Even if the LLM hallucinates a tool call, the executor blocks it deterministically.

### Policy Enforcement

Policies in `AgentConfig.Policies.Files` are loaded into the Mangle kernel:

```go
// Load policies at agent initialization
for _, policyFile := range agentConfig.Policies.Files {
    kernel.LoadPolicy(policyFile)
}

// Later, check permissions
permitted, _ := kernel.Query("permitted(shell_exec(\"rm -rf /\"))")
if len(permitted) == 0 {
    return fmt.Errorf("action blocked by policy")
}
```

---

## Serialization & Persistence

`AgentConfig` is JSON-serializable for storage and transfer:

```go
// Serialize
configJSON, _ := json.Marshal(agentConfig)
os.WriteFile(".nerd/agents/my-specialist/config.json", configJSON, 0644)

// Deserialize
var loadedConfig config.AgentConfig
json.Unmarshal(configJSON, &loadedConfig)
```

**Use Cases:**
- **Specialist Agents:** Store custom configs in `.nerd/agents/{name}/config.json`
- **Audit Trail:** Log which config was used for each execution
- **Reproducibility:** Re-run agents with exact same config

---

## Future Enhancements

### Planned Features

- [ ] **Dynamic Tool Discovery:** Auto-populate tools based on available MCP servers
- [ ] **Policy Composition:** Merge multiple policy sets with priority
- [ ] **Config Inheritance:** Extend base configs for specialized agents
- [ ] **Capability Constraints:** Limit token usage, max file size, etc.
- [ ] **Multi-Mode Support:** Single config supporting multiple execution modes

### Extension Points

```go
// Future: Extended AgentConfig
type AgentConfig struct {
    IdentityPrompt string
    Tools          ToolSet
    Policies       PolicySet
    Mode           string

    // NEW: Additional constraints
    Constraints    ConstraintSet
    // NEW: Base config to extend
    Extends        string
    // NEW: Metadata
    Metadata       map[string]interface{}
}
```

---

## File Index

| File | Lines | Purpose |
|------|-------|---------|
| `config/types.go` | 46 | AgentConfig and related type definitions |
| `config/config_test.go` | ~50 | Validation tests |
| **Total** | **~96** | Complete JIT configuration schema |

---

## See Also

- [Session Executor](../session/CLAUDE.md) - Uses AgentConfig for execution
- [ConfigFactory](../prompt/config_factory.go) - Generates AgentConfig objects
- [JIT Compiler](../prompt/compiler.go) - Compiles identity prompts
- [Intent Routing](../mangle/intent_routing.mg) - Determines persona for config
- [JIT-Driven Execution Model](../../.claude/skills/codenerd-builder/references/jit-execution-model.md) - Complete architecture guide

---

**Last Updated:** December 27, 2024
**Architecture Version:** 2.0.0 (JIT-Driven)
**Purpose:** Bridge between declarative configuration and runtime execution
