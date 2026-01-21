# JIT-Driven Agent Execution: Universal Executor & Dynamic Configuration

**Version:** 2.0.0 (December 2024 - Major Refactor)
**Replaces:** Shard Agents (v1.0.0)
**Implementation:** `internal/session/*.go`, `internal/jit/config/*.go`, `internal/mangle/intent_routing.mg`

---

## Overview: From Hardcoded Shards to JIT-Driven Execution

**December 2024** marked a fundamental architectural shift in codeNERD. The previous **hardcoded shard architecture** (35,000+ lines of Go implementing CoderShard, TesterShard, ReviewerShard, etc.) has been completely replaced by a **JIT-driven universal executor** (~1,600 lines) that dynamically generates agent configurations at runtime.

### The Core Problem with Hardcoded Shards

The old shard system required:
- Separate Go struct implementations for each agent type
- Hardcoded tool/policy selection in each shard
- 500-2,000 lines of boilerplate per new agent type
- Recompilation for behavior changes
- Significant code duplication across shards

### The New Solution: JIT Configuration + Universal Executor

The new architecture separates concerns:
- **Mangle logic** determines persona and routing (declarative `.mg` rules)
- **JIT Compiler** assembles context-aware system prompts (prompt atoms)
- **ConfigFactory** generates AgentConfig (tools + policies per intent)
- **Session Executor** provides universal execution loop (works for all personas)

**Result:** 95% code reduction, zero boilerplate for new personas, runtime configurability.

---

## 1. Architecture Overview

### 1.1 The New Execution Flow

```text
                    ┌──────────────────────┐
                    │   User Input         │
                    │   "Fix the bug in    │
                    │    auth.go"          │
                    └──────────┬───────────┘
                               │
                               ▼
                    ┌──────────────────────┐
                    │ Perception Transducer│
                    │ (LLM)                │
                    └──────────┬───────────┘
                               │
                               ▼
           user_intent("id", /command, /fix, "auth.go", /none)
                               │
                               ▼
           ┌───────────────────────────────────────┐
           │   Intent Routing (Mangle Logic)      │
           │   internal/mangle/intent_routing.mg   │
           └───────────────┬───────────────────────┘
                           │
      ┌────────────────────┴────────────────────┐
      │                                         │
      ▼                                         ▼
┌─────────────────┐                  ┌─────────────────┐
│  ConfigFactory  │                  │  JIT Compiler   │
│  Generate       │                  │  Compile System │
│  AgentConfig    │                  │  Prompt         │
└────────┬────────┘                  └────────┬────────┘
         │                                    │
         │  AgentConfig {                     │  System Prompt
         │    Tools: [...]                    │  (context-aware)
         │    Policies: [...]                 │
         │    IdentityPrompt: ...             │
         │  }                                 │
         └────────────────┬───────────────────┘
                          │
                          ▼
           ┌──────────────────────────────┐
           │     Session Executor         │
           │  (Universal Execution Loop)  │
           └──────────────┬───────────────┘
                          │
          ┌───────────────┼───────────────┐
          │               │               │
          ▼               ▼               ▼
    ┌─────────┐   ┌─────────────┐  ┌──────────┐
    │   LLM   │   │VirtualStore │  │  Safety  │
    │         │   │  (Tools)    │  │  Gates   │
    └─────────┘   └─────────────┘  └──────────┘
                          │
                          ▼
           ┌──────────────────────────────┐
           │   User Response              │
           │   "Fixed null pointer in     │
           │    auth.go:142"              │
           └──────────────────────────────┘
```

### 1.2 Component Responsibilities

| Component | Responsibility | Declarative/Imperative |
|-----------|----------------|----------------------|
| **Intent Routing** | Map intent verbs → persona | Declarative (Mangle) |
| **ConfigFactory** | Generate AgentConfig for persona | Imperative (Go) |
| **JIT Compiler** | Assemble system prompt from atoms | Imperative (Go) |
| **Session Executor** | Execute LLM interaction + tools | Imperative (Go) |
| **VirtualStore** | Provide tool implementations | Imperative (Go) |

**Key Insight:** Behavior is determined by declarative logic and composition, not hardcoded implementations.

---

## 2. Intent Routing System

**Location:** `internal/mangle/intent_routing.mg`
**Lines:** 228 lines of declarative Mangle rules
**Replaces:** ~12,000 lines of ShardManager routing logic

### 2.1 Persona Selection Rules

Intent routing determines which "persona" the agent should embody based on the user's intent verb:

```mangle
# Persona selection based on intent verbs
persona(/coder) :- user_intent(_, _, /fix, _, _).
persona(/coder) :- user_intent(_, _, /implement, _, _).
persona(/coder) :- user_intent(_, _, /refactor, _, _).
persona(/coder) :- user_intent(_, _, /create, _, _).
persona(/coder) :- user_intent(_, _, /modify, _, _).

persona(/tester) :- user_intent(_, _, /test, _, _).
persona(/tester) :- user_intent(_, _, /cover, _, _).
persona(/tester) :- user_intent(_, _, /verify, _, _).

persona(/reviewer) :- user_intent(_, _, /review, _, _).
persona(/reviewer) :- user_intent(_, _, /audit, _, _).
persona(/reviewer) :- user_intent(_, _, /check, _, _).
persona(/reviewer) :- user_intent(_, _, /analyze, _, _).

persona(/researcher) :- user_intent(_, _, /research, _, _).
persona(/researcher) :- user_intent(_, _, /learn, _, _).
persona(/researcher) :- user_intent(_, _, /document, _, _).
persona(/researcher) :- user_intent(_, _, /explore, _, _).

# Default to coder for unmatched intents
persona(/coder) :- user_intent(_, _, V, _, _), not persona_matched(V).
persona_matched(V) :- persona(P), P != /coder, user_intent(_, _, V, _, _).
```

### 2.2 Action Type Derivation

```mangle
# Action types for different operations
action_type(/create) :- user_intent(_, /command, /create, _, _).
action_type(/create) :- user_intent(_, /command, /implement, _, _).
action_type(/create) :- user_intent(_, /command, /add, _, _).
action_type(/create) :- user_intent(_, /command, /generate, _, _).

action_type(/modify) :- user_intent(_, /command, /fix, _, _).
action_type(/modify) :- user_intent(_, /command, /refactor, _, _).
action_type(/modify) :- user_intent(_, /command, /update, _, _).
action_type(/modify) :- user_intent(_, /command, /change, _, _).

action_type(/delete) :- user_intent(_, /command, /remove, _, _).
action_type(/delete) :- user_intent(_, /command, /delete, _, _).

action_type(/query) :- user_intent(_, /question, _, _, _).
action_type(/query) :- user_intent(_, /command, /find, _, _).
action_type(/query) :- user_intent(_, /command, /search, _, _).
```

### 2.3 Test Framework Detection

```mangle
# Automatically detect test frameworks based on files
test_framework(/go_test) :- file_exists("go.mod").

test_framework(/jest) :- file_exists("jest.config.js").
test_framework(/jest) :- file_exists("jest.config.ts").
test_framework(/vitest) :- file_exists("vitest.config.js").

test_framework(/pytest) :- file_exists("pytest.ini").
test_framework(/pytest) :- file_exists("pyproject.toml"), file_contains("pyproject.toml", "pytest").
test_framework(/pytest) :- file_exists("conftest.py").

test_framework(/cargo_test) :- file_exists("Cargo.toml").
```

**Why This Matters:** Test framework detection was previously hardcoded in `TesterShard.detectFramework()`. Now it's declarative Mangle logic.

---

## 3. JIT Configuration System

**Location:** `internal/prompt/config_factory.go`
**Lines:** 205 lines
**Replaces:** Hardcoded shard configs in each shard implementation

### 3.1 ConfigAtom: The Building Block

```go
// ConfigAtom represents a configuration fragment for an intent
type ConfigAtom struct {
    Tools    []string  // Tools required for this intent
    Policies []string  // Mangle policy files to load
    Priority int       // Merge priority
}
```

**Example ConfigAtoms:**

```go
// Coder persona
ConfigAtom{
    Tools:    ["file_read", "file_write", "shell_exec", "git", "build_check"],
    Policies: ["code_safety.mg", "git_workflow.mg"],
    Priority: 10,
}

// Tester persona
ConfigAtom{
    Tools:    ["test_exec", "coverage_analyzer", "file_read"],
    Policies: ["test_strategy.mg", "framework_detection.mg"],
    Priority: 10,
}

// Reviewer persona
ConfigAtom{
    Tools:    ["file_read", "hypothesis_gen", "impact_analysis"],
    Policies: ["review_policy.mg", "hypothesis_rules.mg"],
    Priority: 10,
}
```

### 3.2 AgentConfig: The Complete Configuration

```go
// AgentConfig is the final configuration passed to Session Executor
type AgentConfig struct {
    IdentityPrompt string      // JIT-compiled system prompt
    Tools          ToolSet      // Allowed tools for this task
    Policies       PolicySet    // Mangle policy files to load
    Mode           string       // "SingleTurn", "Campaign", etc.
}

// ToolSet specifies allowed tools
type ToolSet struct {
    AllowedTools []string
}

// PolicySet specifies Mangle policies
type PolicySet struct {
    Files []string  // Paths to .mg files
}
```

### 3.3 ConfigFactory.Generate()

```go
func (f *ConfigFactory) Generate(
    ctx context.Context,
    result *CompilationResult,
    intents ...string,
) (*config.AgentConfig, error) {
    var finalAtom ConfigAtom
    found := false

    // Merge all ConfigAtoms for provided intents
    for _, intent := range intents {
        if atom, ok := f.provider.GetAtom(intent); ok {
            finalAtom = finalAtom.Merge(atom)
            found = true
        }
    }

    if !found {
        return nil, fmt.Errorf("no config atoms found for intents: %v", intents)
    }

    // Construct AgentConfig
    cfg := &config.AgentConfig{
        IdentityPrompt: result.Prompt,
        Tools: config.ToolSet{
            AllowedTools: finalAtom.Tools,
        },
        Policies: config.PolicySet{
            Files: finalAtom.Policies,
        },
    }

    return cfg, nil
}
```

### 3.4 ConfigAtom Merging

```go
func (c ConfigAtom) Merge(other ConfigAtom) ConfigAtom {
    merged := ConfigAtom{
        Tools:    uniqueStrings(append(c.Tools, other.Tools...)),
        Policies: uniqueStrings(append(c.Policies, other.Policies...)),
        Priority: c.Priority,
    }

    if other.Priority > c.Priority {
        merged.Priority = other.Priority
    }

    return merged
}
```

**Example:** If intent verbs suggest both "coder" and "tester" personas (e.g., "implement and test feature X"), the ConfigAtoms merge to provide tools and policies for both.

---

## 4. Session-Based Execution

**Location:** `internal/session/`
**Files:** `executor.go` (391 lines), `spawner.go` (385 lines), `subagent.go` (339 lines)
**Total:** ~1,115 lines
**Replaces:** ~35,000 lines of shard implementations

### 4.1 Session Executor

The **Session Executor** is the universal execution loop for all agent types.

```go
// Executor implements the clean execution loop.
type Executor struct {
    mu sync.RWMutex

    // Core dependencies
    kernel       types.Kernel
    virtualStore types.VirtualStore
    llmClient    types.LLMClient

    // JIT components
    jitCompiler   *prompt.JITPromptCompiler
    configFactory *prompt.ConfigFactory

    // Perception
    transducer perception.Transducer

    // Context management
    conversationHistory []perception.ConversationTurn
    sessionContext      *types.SessionContext

    // Configuration
    config ExecutorConfig
}
```

**Key Methods:**

```go
// Execute runs a single turn of the execution loop
func (e *Executor) Execute(ctx context.Context, task string) (string, error)

// ExecuteWithConfig uses a pre-built AgentConfig
func (e *Executor) ExecuteWithConfig(ctx context.Context, agentConfig *config.AgentConfig, task string) (string, error)

// SetSessionContext injects shared context
func (e *Executor) SetSessionContext(sc *types.SessionContext)
```

**Execution Flow:**

```go
func (e *Executor) Execute(ctx context.Context, task string) (string, error) {
    // 1. Transduce input to intent atoms
    intents := e.transducer.TransduceToIntent(task)

    // 2. Query Mangle for persona
    persona := e.kernel.Query("persona(P)")

    // 3. Generate AgentConfig via ConfigFactory
    compilationResult := e.jitCompiler.Compile(ctx, persona)
    agentConfig, err := e.configFactory.Generate(ctx, compilationResult, persona)

    // 4. Execute LLM interaction with configured tools
    response := e.llmClient.CompleteWithSystem(ctx, agentConfig.IdentityPrompt, task)

    // 5. Route tool calls through VirtualStore
    for _, toolCall := range parsedToolCalls {
        if !e.isToolAllowed(toolCall.Name, agentConfig.Tools) {
            return "", fmt.Errorf("tool %s not allowed", toolCall.Name)
        }
        e.virtualStore.ExecuteTool(ctx, toolCall)
    }

    // 6. Apply safety gates
    if !e.kernel.Query("permitted(Action)") {
        return "", fmt.Errorf("action blocked by safety gates")
    }

    // 7. Return response
    return response, nil
}
```

### 4.2 Spawner: Dynamic SubAgent Creation

The **Spawner** manages creation and lifecycle of SubAgents.

```go
// Spawner manages JIT-driven subagent creation and lifecycle.
type Spawner struct {
    mu sync.RWMutex

    // Core dependencies (shared with all spawned subagents)
    kernel        types.Kernel
    virtualStore  types.VirtualStore
    llmClient     types.LLMClient
    jitCompiler   *prompt.JITPromptCompiler
    configFactory *prompt.ConfigFactory
    transducer    perception.Transducer

    // Active subagents
    subagents map[string]*SubAgent

    // Configuration
    maxActiveSubagents int
}
```

**SpawnRequest:**

```go
type SpawnRequest struct {
    Name       string         // Subagent name
    Task       string         // Initial task
    Type       SubAgentType   // Ephemeral, Persistent, or System
    IntentVerb string         // Used for JIT config generation
    Timeout    time.Duration  // Execution timeout
}
```

**Spawn Method:**

```go
func (s *Spawner) Spawn(ctx context.Context, req SpawnRequest) (*SubAgent, error) {
    // 1. Check active limit
    activeCount := s.countActive()
    if activeCount >= s.maxActiveSubagents {
        return nil, fmt.Errorf("max active subagents reached: %d", s.maxActiveSubagents)
    }

    // 2. Generate AgentConfig for this subagent
    compilationResult := s.jitCompiler.Compile(ctx, req.IntentVerb)
    agentConfig, err := s.configFactory.Generate(ctx, compilationResult, req.IntentVerb)

    // 3. Create SubAgent with config
    subagent := NewSubAgent(SubAgentConfig{
        ID:          generateID(),
        Name:        req.Name,
        Type:        req.Type,
        AgentConfig: agentConfig,
        Kernel:      s.kernel,
        VirtualStore: s.virtualStore,
        LLMClient:   s.llmClient,
    })

    // 4. Start execution
    go subagent.Run(ctx, req.Task)

    // 5. Track subagent
    s.subagents[subagent.ID] = subagent

    return subagent, nil
}
```

### 4.3 SubAgent: Execution Context

**SubAgents** are dynamically created execution contexts with configurable lifecycle.

```go
// SubAgent represents a spawned execution context.
type SubAgent struct {
    mu sync.RWMutex

    ID   string
    Name string
    Type SubAgentType

    // JIT-compiled configuration
    agentConfig *config.AgentConfig

    // Execution context
    kernel       types.Kernel
    virtualStore types.VirtualStore
    llmClient    types.LLMClient

    // State
    state             SubAgentState
    conversationHist  []ConversationTurn
    createdAt         time.Time
    completedAt       time.Time
}
```

**SubAgent Types:**

```go
type SubAgentType string

const (
    Ephemeral  SubAgentType = "ephemeral"  // Single task, RAM only
    Persistent SubAgentType = "persistent" // Multi-turn, maintains history
    System     SubAgentType = "system"     // Long-running background service
)
```

**Lifecycle:**

| Type | Creation | Execution | Termination |
|------|----------|-----------|-------------|
| **Ephemeral** | Spawn on demand | Execute task | Terminate immediately |
| **Persistent** | Spawn on demand | Multi-turn conversation | Terminate on idle or explicit stop |
| **System** | Auto-start on init | Continuous background loop | Shutdown on session end |

---

## 5. Intent → Persona → Config Mapping

### 5.1 Complete Routing Table

| Intent Verbs | Persona | Tools | Policies | Use Case |
|--------------|---------|-------|----------|----------|
| fix, implement, refactor, create, modify, add, update | **Coder** | file_write, shell_exec, git, build_check | code_safety.mg, git_workflow.mg | Code generation, bug fixes, refactoring |
| test, cover, verify, validate | **Tester** | test_exec, coverage_analyzer, file_read | test_strategy.mg, framework_detection.mg | Test creation, TDD loops, coverage analysis |
| review, audit, check, analyze, inspect | **Reviewer** | file_read, hypothesis_gen, impact_analysis, preflight | review_policy.mg, hypothesis_rules.mg | Code review, security audit, quality checks |
| research, learn, document, explore, find | **Researcher** | web_fetch, doc_parse, kb_ingest, context7 | research_strategy.mg | Knowledge gathering, documentation |

### 5.2 Multi-Intent Merging

If an intent suggests multiple personas (e.g., "implement feature X and write tests"), ConfigAtoms merge:

```
Intent: "implement feature X and write tests"
  ↓
Detected Personas: /coder, /tester
  ↓
ConfigAtom Merging:
  Coder ConfigAtom:
    Tools: [file_write, shell_exec, git, build_check]
    Policies: [code_safety.mg, git_workflow.mg]
  +
  Tester ConfigAtom:
    Tools: [test_exec, coverage_analyzer, file_read]
    Policies: [test_strategy.mg, framework_detection.mg]
  =
  Merged AgentConfig:
    Tools: [file_write, shell_exec, git, build_check, test_exec, coverage_analyzer, file_read]
    Policies: [code_safety.mg, git_workflow.mg, test_strategy.mg, framework_detection.mg]
```

---

## 6. Migration from Hardcoded Shards

### 6.1 What Was Removed

**Deleted Implementations:**
- `internal/shards/coder/` - 3,500 lines (11 files)
- `internal/shards/tester/` - 2,800 lines (12 files)
- `internal/shards/reviewer/` - 8,500 lines (25 files)
- `internal/shards/researcher/` - 5,200 lines (11 files)
- `internal/shards/nemesis/` - 2,000 lines (7 files)
- `internal/shards/tool_generator/` - 1,000 lines (3 files)
- `internal/core/shard_manager.go` - 12,000 lines (modularized into 5 files, now deleted)

**Total Removed:** ~35,000 lines

### 6.2 What Was Added

**New Implementations:**
- `internal/session/executor.go` - 391 lines (universal execution loop)
- `internal/session/spawner.go` - 385 lines (dynamic subagent creation)
- `internal/session/subagent.go` - 339 lines (execution context)
- `internal/mangle/intent_routing.mg` - 228 lines (declarative routing)
- `internal/prompt/config_factory.go` - 205 lines (AgentConfig generation)
- `internal/jit/config/types.go` - 46 lines (configuration schema)

**Total Added:** ~1,594 lines

**Net Reduction:** 33,406 lines (95% reduction)

### 6.3 Old vs New: Adding a New Agent

**Old Way (Hardcoded Shard):**

```go
// Step 1: Implement Shard interface (500-2000 lines)
type MyNewShard struct {
    mu sync.RWMutex
    id string
    config types.ShardConfig
    state  types.ShardState
    kernel types.Kernel
    virtualStore types.VirtualStore
    llmClient types.LLMClient
    // ... more boilerplate ...
}

func (s *MyNewShard) Execute(ctx context.Context, task ShardTask) (string, error) {
    // Hardcoded logic for this shard type
    // 100-500 lines of task parsing, context building, LLM calls, etc.
}

// Step 2: Register in ShardManager
// Step 3: Write tests
// Step 4: Update documentation
// Step 5: Recompile binary
```

**Estimated Effort:** 4-8 hours, 500-2000 lines of Go code

**New Way (JIT-Driven):**

```mangle
# Step 1: Add intent routing rule (1 line)
persona(/my_new_agent) :- user_intent(_, _, /my_verb, _, _).
```

```yaml
# Step 2: Add prompt atom (10-20 lines)
# File: internal/prompt/atoms/identity/my_new_agent.yaml
id: my_new_agent_identity
category: identity
content: |
  You are a specialized agent for [purpose].
  Your goals are:
  - [goal 1]
  - [goal 2]
```

```go
// Step 3: Add ConfigAtom (optional, can use defaults)
provider.GetAtom("my_new_agent") // Returns ConfigAtom
```

**Estimated Effort:** 15-30 minutes, ~20 lines total (1 Mangle rule + 1 YAML file)

---

## 7. Benefits of JIT-Driven Architecture

### 7.1 Code Reduction

- **95% less code** in agent execution layer
- Fewer tests required (test executor once, not each shard)
- Easier to maintain and debug

### 7.2 Declarative Behavior

- Persona selection via Mangle logic (transparent, traceable)
- Tool/policy selection via ConfigAtoms (composable, mergeable)
- No hidden behavior in Go code

### 7.3 Zero Boilerplate for New Agents

- Add Mangle rule + prompt atoms
- No Go implementation required
- Immediate availability (no recompile)

### 7.4 Runtime Configurability

- Change behavior by updating .mg files or prompt atoms
- No recompilation needed
- Hot-reload friendly (future feature)

### 7.5 Cleaner Separation of Concerns

| Layer | Responsibility | Language |
|-------|----------------|----------|
| **Creative** | Problem solving, synthesis, insight | LLM |
| **Executive** | Planning, routing, safety | Mangle (declarative logic) |
| **Infrastructure** | Execution, FFI, persistence | Go (imperative code) |

**Old Architecture:** All three concerns mixed in hardcoded shard implementations.
**New Architecture:** Clean separation with clear boundaries.

---

## 8. Comparison Table: Old vs New

| Aspect | Old (Hardcoded Shards) | New (JIT-Driven) |
|--------|----------------------|------------------|
| **Lines of Code** | ~35,000 | ~1,600 |
| **New Agent Effort** | 4-8 hours, 500-2000 lines | 15-30 min, ~20 lines |
| **Behavior Change** | Recompile binary | Update .mg or YAML files |
| **Persona Selection** | Hardcoded in ShardManager | Declarative Mangle rules |
| **Tool Selection** | Hardcoded in each shard | ConfigAtom merging |
| **Code Duplication** | High (each shard reimplements) | Zero (single executor) |
| **Testing Burden** | Test each shard separately | Test executor + ConfigFactory |
| **Maintenance** | Multiple files per shard | Single executor |

---

## 9. Future Enhancements

### 9.1 Hot-Reload

- Watch `.mg` files and prompt atoms for changes
- Reload without restarting session
- Already partially supported (JIT compilation is runtime)

### 9.2 User-Defined ConfigAtoms

- Allow users to define custom ConfigAtoms via CLI
- Store in `.nerd/agents/custom_configs.yaml`
- Enable fully custom personas without touching codebase

### 9.3 Dynamic Persona Detection

- Use LLM to detect persona from natural language (no explicit verbs)
- "Can you help me understand this code?" → auto-detect `/reviewer` or `/explainer`

### 9.4 Mangle-Based Tool Dependency Resolution

- Automatically infer required tools from policy files
- Example: `review_policy.mg` uses `hypothesis_gen` predicate → auto-add tool

---

## 10. Summary

The JIT-driven execution architecture represents a fundamental reimagining of how codeNERD agents work:

**Core Philosophy:**
> "Logic determines Reality; the Model merely describes it."

**Old Paradigm:**
- Hardcoded Go implementations determine behavior
- LLM is constrained by shard logic
- Adding new agents requires code changes

**New Paradigm:**
- Declarative Mangle logic determines behavior
- LLM is the creative center, logic is the executive
- Adding new agents requires configuration changes

**Metrics:**
- 95% code reduction (35K → 1.6K lines)
- Zero boilerplate for new personas
- Runtime configurability (no recompile)
- Cleaner separation of concerns

This refactor embodies codeNERD's vision: a system where creative power (LLM) and deterministic control (Mangle logic) coexist through a clean, declarative architecture—not tangled in 35,000 lines of imperative shard implementations.

---

**See Also:**
- [Intent Routing Rules](../../internal/mangle/intent_routing.mg)
- [ConfigFactory Implementation](../../internal/prompt/config_factory.go)
- [Session Executor](../../internal/session/executor.go)
- [Architecture Changes Document](../../../../docs/conductor/tracks/jit_refactor_20251226/ARCHITECTURE_CHANGES.md)
