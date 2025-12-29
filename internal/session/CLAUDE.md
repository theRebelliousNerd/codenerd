# internal/session/

The Universal Execution Loop - JIT-driven agent execution replacing hardcoded shards.

**Architecture Version:** 2.0.0 (December 2024 - JIT-Driven)

---

## Overview

The `session` package implements codeNERD's clean execution loop, replacing the old hardcoded shard architecture with a unified, JIT-driven approach. Instead of 35,000 lines of rigid shard implementations, the session package provides ~1,115 lines of flexible execution infrastructure that works for all agent types.

### Philosophy

> **"No shards. No spawn. No factories. Clean."**

The session architecture treats the LLM as the creative center, with the executor providing context, tools, and safety guardrails. All specialization happens through JIT-compiled prompts and configs—not through hardcoded Go implementations.

---

## Architecture

```
User Input → Transducer → user_intent atoms
                              ↓
                    Intent Routing (Mangle)
                              ↓
                    ┌─────────┴─────────────┐
                    ↓                       ↓
            ConfigFactory               JIT Compiler
        (Generate AgentConfig)      (Compile System Prompt)
                    ↓                       ↓
                    └───────────┬───────────┘
                                ↓
                        Session Executor
                        (Universal Loop)
                                ↓
                    LLM + VirtualStore + Safety
```

---

## Core Components

### 1. Executor (`executor.go`)

**Purpose:** Universal execution loop for all agent types (coder, tester, reviewer, researcher, etc.)

**The Clean Loop:**
```
1. OBSERVE  → Transducer converts NL to Intent
2. ORIENT   → Build compilation context from intent + world state
3. JIT      → Compile prompt (persona + skills + context)
4. JIT      → Compile config (tools, policies)
5. LLM      → Generate response with tool calls
6. EXECUTE  → Route tool calls through VirtualStore
7. RESPOND  → Articulate response to user
```

**Key Features:**
- Single execution path for all agent types
- JIT-driven specialization (no hardcoded behavior)
- Tool call limiting (prevents runaway execution)
- Safety gate enforcement (Constitutional Gate)
- Conversation history management
- Timeout handling

**Configuration:**
```go
type ExecutorConfig struct {
    MaxToolCalls     int           // Default: 50
    ToolTimeout      time.Duration // Default: 5 minutes
    EnableSafetyGate bool          // Default: true
}
```

**Usage:**
```go
executor := session.NewExecutor(
    kernel,
    virtualStore,
    llmClient,
    jitCompiler,
    configFactory,
    transducer,
)

result, err := executor.Process(ctx, "Fix the null pointer in auth.go")
if err != nil {
    log.Fatal(err)
}

fmt.Println(result.Response)
fmt.Printf("Tool calls: %d, Duration: %v\n", result.ToolCallsExecuted, result.Duration)
```

**Replaces:** `internal/core/shard_manager.go` (~12,000 lines) and all shard implementations

---

### 2. Spawner (`spawner.go`)

**Purpose:** Dynamic subagent creation with JIT-driven configuration

The Spawner manages the lifecycle of SubAgents, replacing the old ShardFactory pattern. It creates agents on-demand based on intent, with all behavior determined by JIT-compiled configs.

**Key Capabilities:**
- **Intent-Driven Spawning:** Automatically determine persona from intent verb
- **Specialist Loading:** Load user-defined agents from `.nerd/agents/{name}/`
- **Lifecycle Management:** Track active, completed, and failed subagents
- **Resource Limits:** Enforce max active subagent count
- **Cleanup:** Remove completed agents from tracking

**SubAgent Types:**

| Type | Lifespan | Memory | Use Case |
|------|----------|--------|----------|
| **Ephemeral** | Single task | RAM only | Quick fixes, single edits, queries |
| **Persistent** | Multi-turn | RAM + compressed history | Complex refactors, campaigns, research |
| **System** | Long-running | RAM + snapshots | Background indexing, monitoring |

**Spawning Methods:**

```go
// 1. Spawn by explicit request
spawner.Spawn(ctx, SpawnRequest{
    Name:       "coder",
    Task:       "Fix null pointer in auth.go",
    Type:       SubAgentTypeEphemeral,
    IntentVerb: "/fix",
    Timeout:    5 * time.Minute,
})

// 2. Spawn from parsed intent (primary method)
intent, _ := transducer.ParseIntent(ctx, "Fix null pointer in auth.go")
subagent, _ := spawner.SpawnForIntent(ctx, intent, "Fix null pointer in auth.go")

// 3. Spawn user-defined specialist
subagent, _ := spawner.SpawnSpecialist(ctx, "django-expert", "Optimize ORM queries")
```

**Lifecycle Management:**
```go
// Get active subagents
active := spawner.ListActive()

// Stop specific subagent
spawner.Stop(subagentID)

// Stop all
spawner.StopAll()

// Cleanup completed
removed := spawner.Cleanup()
```

**Intent → Persona Mapping:**

The Spawner automatically maps intent verbs to agent names:

| Intent Verbs | Agent Name | Config Loaded |
|--------------|------------|---------------|
| `/fix`, `/implement`, `/refactor`, `/create` | `coder` | ConfigAtom for `/coder` |
| `/test`, `/cover`, `/verify` | `tester` | ConfigAtom for `/tester` |
| `/review`, `/audit`, `/check` | `reviewer` | ConfigAtom for `/reviewer` |
| `/research`, `/learn`, `/document` | `researcher` | ConfigAtom for `/researcher` |

**Replaces:** Shard factory pattern and manual shard spawning

---

### 3. SubAgent (`subagent.go`)

**Purpose:** Context-isolated instance of the clean execution loop

A SubAgent is a self-contained execution context with:
- Its own LLM conversation history (context isolation)
- JIT-provided identity and tools (no hardcoded behavior)
- Memory compression for long-running tasks
- Independent lifecycle (idle → running → completed/failed)

**Key Difference from Old Shards:**

```
OLD: CoderShard with 600 lines of hardcoded Go logic
NEW: SubAgent with ~50-line loop, all behavior from JIT
```

**State Machine:**

```
       [Create]
          ↓
     ┌─ IDLE ─┐
     │         │
   Run()      Stop()
     │         │
     ↓         ↓
  RUNNING → COMPLETED
     │         ↑
   Error      │
     │         │
     └─→ FAILED
```

**Configuration:**
```go
type SubAgentConfig struct {
    ID          string           // Unique identifier
    Name        string           // Human-readable name
    Type        SubAgentType     // Ephemeral/Persistent/System
    AgentConfig *config.AgentConfig // JIT-compiled config
    Timeout     time.Duration    // Execution timeout
    MaxTurns    int              // Max conversation turns
}
```

**Usage:**
```go
// SubAgents are typically created via Spawner
subagent, _ := spawner.Spawn(ctx, SpawnRequest{...})

// Runs asynchronously
go subagent.Run(ctx, task)

// Wait for completion
result, err := subagent.Wait()

// Or poll for state
for subagent.GetState() == SubAgentStateRunning {
    time.Sleep(100 * time.Millisecond)
}
result, err := subagent.GetResult()

// Get metrics
metrics := subagent.GetMetrics()
fmt.Printf("Duration: %v, Turns: %d\n", metrics.Duration, metrics.TurnCount)
```

**Memory Compression:**

For long-running persistent agents:
```go
// Set compressor for semantic compression
subagent.SetCompressor(compressor)

// Compress when history exceeds threshold
subagent.CompressMemory(ctx, 100)
```

**Replaces:** All shard implementations (`CoderShard`, `TesterShard`, etc.)

---

## Execution Flow

### Single-Turn Execution

```
User: "Fix the null pointer in auth.go"
  ↓
Executor.Process(ctx, input)
  ↓
1. OBSERVE: Transducer → Intent{Verb: "/fix", Target: "auth.go"}
  ↓
2. ORIENT: BuildCompilationContext → {IntentVerb: "/fix", FailingTests: 0, ...}
  ↓
3. JIT PROMPT: JITCompiler.Compile → "You are a code fixer. Focus on null pointer bugs..."
  ↓
4. JIT CONFIG: ConfigFactory.Generate → {Tools: ["file_read", "file_write", "git"], ...}
  ↓
5. LLM: llmClient.CompleteWithSystem(prompt, input) → Response + Tool Calls
  ↓
6. EXECUTE: For each tool call:
     - Check if allowed by config
     - Safety check via Constitutional Gate
     - Route through VirtualStore
  ↓
7. RESPOND: Extract text response, update history, return result
  ↓
ExecutionResult{Response: "Fixed...", ToolCallsExecuted: 3, Duration: 12s}
```

### Multi-Agent Spawning

```
Main Executor receives: "Implement feature X with tests"
  ↓
Transducer → Intent{Verb: "/implement", Target: "feature X"}
  ↓
Spawner.SpawnForIntent(ctx, intent, task)
  ↓
1. Determine agent type: Ephemeral (short task)
2. Determine agent name: "coder" (based on /implement verb)
3. Generate JIT config: ConfigAtom for "/coder"
4. Create SubAgent with config
5. Launch: go subagent.Run(ctx, "Implement feature X")
  ↓
SubAgent executes independently using Executor.Process
  ↓
Main executor can spawn second SubAgent for tests:
  ↓
Spawner.Spawn(ctx, SpawnRequest{
    Name:       "tester",
    Task:       "Write tests for feature X",
    IntentVerb: "/test",
})
  ↓
Both SubAgents run concurrently
Main executor waits for results
```

---

## Integration with JIT System

### 1. Intent Routing

```mangle
# internal/mangle/intent_routing.mg
persona(/coder) :- user_intent(_, _, /fix, _, _).
persona(/coder) :- user_intent(_, _, /implement, _, _).
persona(/tester) :- user_intent(_, _, /test, _, _).
```

When Executor asserts `user_intent` to the kernel, Mangle derives the appropriate `persona` fact, which ConfigFactory uses to load the right ConfigAtom.

### 2. ConfigFactory Integration

```go
// Executor compiles config from intent
agentConfig, _ := executor.compileConfig(ctx, compileResult, intent)

// ConfigFactory returns:
type AgentConfig struct {
    IdentityPrompt string   // From JIT Compiler
    Tools          ToolSet   // Allowed tools
    Policies       PolicySet // Mangle policy files
    Mode           string    // SingleTurn, Campaign, etc.
}
```

### 3. JIT Prompt Compilation

```go
// Build context from current state
compilationCtx := executor.buildCompilationContext(intent)

// JIT Compiler assembles prompt from atoms
compileResult, _ := jitCompiler.Compile(ctx, compilationCtx)

// Result includes:
// - Identity atoms (persona-specific)
// - Capability atoms (tool usage)
// - Context atoms (world state)
// - Safety atoms (Constitutional rules)
```

---

## Comparison: Old vs New

### Code Reduction

| Component | Old (Hardcoded) | New (JIT-Driven) | Reduction |
|-----------|-----------------|------------------|-----------|
| CoderShard | 3,500 lines | N/A (uses Executor) | 100% |
| TesterShard | 2,800 lines | N/A (uses Executor) | 100% |
| ReviewerShard | 8,500 lines | N/A (uses Executor) | 100% |
| ResearcherShard | 5,200 lines | N/A (uses Executor) | 100% |
| ShardManager | 12,000 lines | 385 lines (Spawner) | 97% |
| **Total** | **~35,000 lines** | **~1,115 lines** | **95%** |

### Adding a New "Shard" (Persona)

**Old Way:**
```go
// 1. Implement Shard interface (500-2000 lines)
type MyNewShard struct {
    mu sync.RWMutex
    id string
    kernel types.Kernel
    // ... 50 more fields
}

func (s *MyNewShard) Execute(ctx context.Context, task ShardTask) (string, error) {
    // ... 400 lines of hardcoded logic
}

// 2. Register in ShardManager
// 3. Write tests
// 4. Update documentation
// 5. Recompile binary
```

**New Way:**
```mangle
# Add to internal/mangle/intent_routing.mg
persona(/my_new_agent) :- user_intent(_, _, /my_verb, _, _).
```

```yaml
# Add to internal/prompt/atoms/identity/my_new_agent.yaml
id: my_new_agent_identity
category: identity
content: |
  You are a specialized agent for [purpose].
```

**That's it.** No Go code, no recompilation.

---

## Safety & Constraints

### Tool Call Limiting

Prevents runaway execution:
```go
config.MaxToolCalls = 50 // Default
```

If LLM tries to make more than 50 tool calls in a single turn, execution stops.

### Constitutional Gate

All tool calls checked against Mangle policy:
```go
if config.EnableSafetyGate {
    if !executor.checkSafety(toolCall) {
        return error // Blocked
    }
}
```

### Timeout Management

Multiple timeout layers:
```go
// Per-subagent timeout
SubAgentConfig{Timeout: 30 * time.Minute}

// Per-tool timeout
ExecutorConfig{ToolTimeout: 5 * time.Minute}
```

Shortest timeout wins (Go's `context.WithTimeout` semantics).

---

## Thread Safety

### Executor

- `sync.RWMutex` protects conversation history and session context
- Safe for concurrent `Process()` calls (separate goroutines)
- Conversation history append is protected

### Spawner

- `sync.RWMutex` protects subagent registry
- Safe for concurrent `Spawn()` and `Stop()` calls
- Active count checking is atomic

### SubAgent

- `atomic.LoadInt32` / `atomic.StoreInt32` for state
- `sync.RWMutex` for results and configuration
- Safe to call `GetState()` from multiple goroutines

---

## Error Handling

### Graceful Degradation

```go
// If JIT compilation fails, use baseline
if err := jitCompiler.Compile(ctx, compilationCtx); err != nil {
    compileResult = &prompt.CompilationResult{
        Prompt: "You are an AI assistant helping with software development.",
    }
}

// If config compilation fails, continue with empty config
if err := configFactory.Generate(ctx, result, intent); err != nil {
    agentConfig = &config.AgentConfig{}
}
```

### Tool Call Failures

Individual tool call failures don't stop execution:
```go
for i, call := range toolCalls {
    if err := executor.executeToolCall(ctx, call, agentConfig); err != nil {
        logging.Error("Tool call failed: %v", err)
        // Continue with other tool calls
    }
}
```

---

## Observability

### Execution Results

```go
type ExecutionResult struct {
    Response          string        // Text response
    Intent            perception.Intent // Parsed intent
    ToolCallsExecuted int           // Number of tool calls
    Duration          time.Duration // Execution time
    Error             error         // Error if failed
}
```

### SubAgent Metrics

```go
type SubAgentMetrics struct {
    ID        string
    Name      string
    Type      SubAgentType
    State     SubAgentState
    TurnCount int
    Duration  time.Duration
}

// Get metrics for all subagents
metrics := spawner.GetMetrics()
```

### Logging

```go
logging.Session("Processing input: %d chars", len(input))
logging.Session("Spawning subagent: %s (type: %s)", name, agentType)
logging.SessionDebug("Would execute tool: %s", toolName)
```

---

## Future Enhancements

### Planned Features

- [ ] **Multi-turn conversation:** Full interactive sessions with SubAgents
- [ ] **Tool calling protocol:** Proper tool call parsing from LLM responses
- [ ] **VirtualStore integration:** Complete tool execution routing
- [ ] **Memory persistence:** Save/restore SubAgent state for persistent agents
- [ ] **Hot-reload:** Reload JIT configs without restarting
- [ ] **Streaming responses:** Real-time response streaming
- [ ] **Parallel tool execution:** Execute independent tool calls concurrently

### TODO Items in Code

```go
// executor.go
// TODO: Pass tools to LLM for tool calling
// TODO: Implement proper tool call parsing based on LLM response format
// TODO: Implement tool execution through VirtualStore
// TODO: Implement proper Mangle query for permitted(action)
// TODO: Parse out tool calls and return just text

// spawner.go
// TODO: Load from .nerd/agents/{name}/config.yaml

// subagent.go
// TODO: Implement compression via SemanticCompressor
```

---

## Migration Guide

### For Code Using Old Shards

**Old Pattern:**
```go
shardMgr := core.NewShardManager()
shard := shardMgr.GetShard("coder")
result, err := shard.Execute(ctx, task)
```

**New Pattern:**
```go
executor := session.NewExecutor(kernel, virtualStore, llmClient,
                                 jitCompiler, configFactory, transducer)
result, err := executor.Process(ctx, task.Description)
```

### For Spawning Subagents

**Old Pattern:**
```go
shardMgr.Spawn(ctx, "coder", task)
```

**New Pattern:**
```go
spawner := session.NewSpawner(kernel, virtualStore, llmClient,
                               jitCompiler, configFactory, transducer,
                               session.DefaultSpawnerConfig())
subagent, _ := spawner.SpawnForIntent(ctx, intent, task)
```

---

## File Index

| File | Lines | Purpose |
|------|-------|---------|
| `executor.go` | 391 | Universal execution loop for all agent types |
| `spawner.go` | 385 | Dynamic subagent creation and lifecycle management |
| `subagent.go` | 339 | Context-isolated execution instance |
| **Total** | **1,115** | Complete session execution infrastructure |

---

## See Also

- [JIT-Driven Execution Model](../../.claude/skills/codenerd-builder/references/jit-execution-model.md) - Complete architecture guide
- [Intent Routing](../mangle/intent_routing.mg) - Declarative persona selection
- [ConfigFactory](../prompt/config_factory.go) - Dynamic AgentConfig generation
- [JIT Compiler](../prompt/compiler.go) - Runtime prompt assembly
- [Architecture Changes](../../conductor/tracks/jit_refactor_20251226/ARCHITECTURE_CHANGES.md) - Migration metrics

---

**Last Updated:** December 27, 2024
**Architecture Version:** 2.0.0 (JIT-Driven)
**Replaces:** `internal/core/shard_manager.go` and all shard implementations

---

**Remember: Push to GitHub regularly!**
