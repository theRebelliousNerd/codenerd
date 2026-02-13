# internal/shards/

**Status:** LEGACY - Implementations removed (December 2024)
**Current Architecture:** See [JIT-Driven Execution Model](../../.claude/skills/codenerd-builder/references/jit-execution-model.md)

---

## ⚠️ Major Architectural Change

As of **December 2024**, this directory no longer contains shard implementations. The **hardcoded shard architecture has been completely replaced** by the **JIT-driven execution model**.

### What Changed?

- **Removed:** All shard implementations (~35,000 lines)
  - `internal/shards/coder/` - DELETED
  - `internal/shards/tester/` - DELETED
  - `internal/shards/reviewer/` - DELETED
  - `internal/shards/researcher/` - DELETED
  - `internal/shards/nemesis/` - DELETED
  - `internal/shards/tool_generator/` - DELETED

- **Replaced By:** JIT-driven session-based execution (~1,600 lines)
  - `internal/session/executor.go` - Universal execution loop
  - `internal/session/spawner.go` - Dynamic subagent creation
  - `internal/session/subagent.go` - Execution context
  - `internal/mangle/intent_routing.mg` - Declarative routing logic
  - `internal/prompt/config_factory.go` - AgentConfig generation

### Current Directory Structure

```
shards/
├── registration.go      # Legacy command mapping to intents
└── system/             # System utilities (not shard implementations)
    └── CLAUDE.md       # System component docs
```

**Note:** `registration.go` remains only to map legacy `/coder`, `/tester` commands to intents for backward compatibility.

---

## Migration Guide: Old Shards → New System

### Old Shard Types → New SubAgent Types

| Old (Deprecated) | New (Current) |
|------------------|---------------|
| **Type A: Ephemeral Generalists** | **Ephemeral SubAgents** |
| Spawn → Execute → Die, RAM only | Same concept, but configured via JIT |
| CoderShard, TesterShard, ReviewerShard | persona(/coder), persona(/tester), persona(/reviewer) |
| | |
| **Type B: Persistent Specialists** | **Persistent SubAgents** |
| Pre-populated with knowledge, SQLite | Multi-turn conversation, maintains history |
| ResearcherShard | persona(/researcher) with custom ConfigAtoms |
| | |
| **Type S: System Shards** | **System SubAgents** |
| Built-in capabilities | Long-running background services |
| FileShard, ShellShard, GitShard | VirtualStore predicates + system tools |
| | |
| **Type O: Ouroboros** | **Autopoiesis + Prompt Evolution** |
| ToolGeneratorShard | Remains in `internal/autopoiesis/` |

### How Tasks Were Routed: Old vs New

**Old (Hardcoded in ShardManager):**
```go
// internal/core/shard_manager.go (DELETED)
func (sm *ShardManager) Route(intent UserIntent) Shard {
    switch intent.Verb {
    case "/fix", "/implement", "/refactor":
        return sm.GetShard("coder")
    case "/test":
        return sm.GetShard("tester")
    case "/review":
        return sm.GetShard("reviewer")
    // ... 200 more lines of hardcoded routing
    }
}
```

**New (Declarative Mangle Logic):**
```mangle
# internal/mangle/intent_routing.mg
persona(/coder) :- user_intent(_, _, /fix, _, _).
persona(/coder) :- user_intent(_, _, /implement, _, _).
persona(/coder) :- user_intent(_, _, /refactor, _, _).

persona(/tester) :- user_intent(_, _, /test, _, _).

persona(/reviewer) :- user_intent(_, _, /review, _, _).
```

### How Configurations Were Set: Old vs New

**Old (Hardcoded in Each Shard):**
```go
// internal/shards/coder/coder.go (DELETED)
type CoderShard struct {
    // Hardcoded tools
    allowedTools []string{"file_read", "file_write", "shell_exec", "git"}

    // Hardcoded policies
    policies []string{"code_safety.mg", "git_workflow.mg"}

    // Hardcoded system prompt
    systemPrompt = "You are a code generation expert..."
}
```

**New (JIT-Compiled Configuration):**
```go
// internal/prompt/config_factory.go
ConfigAtom{
    Tools:    ["file_read", "file_write", "shell_exec", "git"],
    Policies: ["code_safety.mg", "git_workflow.mg"],
    Priority: 10,
}
// Merged at runtime + JIT-compiled system prompt
```

---

## Current Architecture

### Intent → Persona → Configuration Flow

```
User Input: "Fix the bug in auth.go"
     ↓
Perception Transducer
     ↓
user_intent("id", /command, /fix, "auth.go", /none)
     ↓
Intent Routing (Mangle Logic)
     Query: persona(P) :- user_intent(_, _, /fix, _, _).
     Result: persona(/coder)
     ↓
ConfigFactory
     GetAtom("/coder") → ConfigAtom{
       Tools: ["file_read", "file_write", "shell_exec", "git"],
       Policies: ["code_safety.mg", "git_workflow.mg"]
     }
     ↓
JIT Compiler
     Compile system prompt from atoms in internal/prompt/atoms/identity/coder.yaml
     ↓
Session Executor
     Execute(ctx, AgentConfig{
       IdentityPrompt: "You are a code fixer...",
       Tools: [...],
       Policies: [...]
     })
     ↓
LLM + VirtualStore + Safety Gates
     ↓
Response
```

### Available Personas

| Persona | Intent Verbs | Tools | Policies |
|---------|--------------|-------|----------|
| **Coder** | fix, implement, refactor, create, modify | file_write, shell_exec, git, build | code_safety.mg |
| **Tester** | test, cover, verify, validate | test_exec, coverage_analyzer | test_strategy.mg |
| **Reviewer** | review, audit, check, analyze | hypothesis_gen, impact_analysis | review_policy.mg |
| **Researcher** | research, learn, document, explore | web_fetch, doc_parse, kb_ingest | research_strategy.mg |

---

## Adding a New "Shard" (Persona)

**Old Way (Required 500-2000 lines of Go):**
1. Implement `Shard` interface
2. Create struct with kernel, virtualStore, llmClient
3. Implement `Execute()` method with task parsing, LLM calls, etc.
4. Register in ShardManager
5. Write tests
6. Recompile binary

**New Way (Requires ~20 lines total):**

### Step 1: Add Intent Routing Rule

```mangle
# File: internal/mangle/intent_routing.mg
persona(/my_new_agent) :- user_intent(_, _, /my_verb, _, _).
```

### Step 2: Add Prompt Atoms

```yaml
# File: internal/prompt/atoms/identity/my_new_agent.yaml
id: my_new_agent_identity
category: identity
content: |
  You are a specialized agent for [purpose].
  Your capabilities include:
  - [capability 1]
  - [capability 2]
```

### Step 3: (Optional) Define ConfigAtom

```go
// File: internal/prompt/config_defaults.go
// Or use default provider which auto-detects from persona name
```

**That's it!** The Session Executor will automatically:
- Route intents with `/my_verb` to `persona(/my_new_agent)`
- Load the identity prompt atoms
- Generate AgentConfig with appropriate tools and policies
- Execute the LLM interaction

**No Go code. No recompilation. No tests beyond existing executor tests.**

---

## Where Did Shard Logic Go?

### CoderShard Logic → Multiple Places

| Old CoderShard Method | New Location |
|----------------------|--------------|
| `parseTask()` | `internal/mangle/intent_routing.mg` (declarative rules) |
| `buildContext()` | `internal/session/executor.go` (JIT Compiler integration) |
| `generateCode()` | LLM with JIT-compiled prompt |
| `applyEdits()` | VirtualStore tool execution |
| `runBuild()` | VirtualStore `build_check` tool |
| `trackLearning()` | `internal/autopoiesis/` (unchanged) |

### TesterShard Logic → Multiple Places

| Old TesterShard Method | New Location |
|----------------------|--------------|
| `detectFramework()` | `internal/mangle/intent_routing.mg` (test_framework rules) |
| `generateTests()` | LLM with JIT-compiled prompt |
| `runTests()` | VirtualStore `test_exec` tool |
| `parseCoverage()` | VirtualStore tool |
| `tddRepairLoop()` | `internal/mangle/policy.mg` (TDD rules unchanged) |

### ReviewerShard Logic → Multiple Places

| Old ReviewerShard Method | New Location |
|----------------------|--------------|
| `preflightChecks()` | VirtualStore `build_check`, `vet_check` tools |
| `generateHypotheses()` | `internal/mangle/policy.mg` (hypothesis rules) |
| `impactAnalysis()` | `internal/world/holographic.go` (unchanged) |
| `verifyWithLLM()` | LLM with JIT-compiled prompt |
| `scoreReview()` | LLM response parsing |

---

## Backward Compatibility

### Legacy Commands Still Work

The `/coder`, `/tester`, `/reviewer` commands still function via `registration.go`:

```go
// internal/shards/registration.go
func MapLegacyCommand(cmd string) string {
    switch cmd {
    case "/coder":   return "fix"
    case "/tester":  return "test"
    case "/reviewer": return "review"
    default:         return cmd
    }
}
```

### Migration Path for Users

Users can continue using:
- `/coder "fix bug in auth.go"` → routes to `persona(/coder)`
- `/tester "run tests"` → routes to `persona(/tester)`
- `/reviewer "check for issues"` → routes to `persona(/reviewer)`

Or use natural language:
- "Fix bug in auth.go" → auto-routes to `persona(/coder)`
- "Run tests" → auto-routes to `persona(/tester)`
- "Review this code" → auto-routes to `persona(/reviewer)`

---

## Key Benefits

### 1. Massive Code Reduction
- **95% less code** (35,000 → 1,600 lines)
- Easier to maintain and debug
- Fewer tests needed

### 2. Runtime Configurability
- Update behavior via `.mg` files or YAML
- No recompilation required
- Hot-reload friendly (future)

### 3. Zero Boilerplate for New Personas
- Add Mangle rule + prompt atoms
- No Go implementation required
- Immediate availability

### 4. Cleaner Architecture
- Declarative routing (Mangle logic)
- Composable configurations (ConfigAtoms)
- Universal execution (Session Executor)

---

## See Also

- [JIT-Driven Execution Model](../../.claude/skills/codenerd-builder/references/jit-execution-model.md) - Complete guide to new architecture
- [Intent Routing Rules](../mangle/intent_routing.mg) - Declarative routing logic
- [Session Executor](../session/executor.go) - Universal execution loop
- [ConfigFactory](../prompt/config_factory.go) - AgentConfig generation
- [Architecture Changes](../../conductor/tracks/jit_refactor_20251226/ARCHITECTURE_CHANGES.md) - Migration metrics

---

**Last Updated:** December 27, 2024
**Architecture Version:** 2.0.0 (JIT-Driven)
