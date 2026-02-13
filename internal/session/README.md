# internal/session/

The Universal Execution Loop - JIT-driven agent execution replacing hardcoded shards.

**Architecture Version:** 2.0.0 (December 2024 - JIT-Driven)

## Overview

The `session` package implements codeNERD's clean execution loop, replacing the old hardcoded shard architecture with a unified, JIT-driven approach. Instead of 35,000 lines of rigid shard implementations, the session package provides ~1,115 lines of flexible execution infrastructure.

> **"No shards. No spawn. No factories. Clean."**

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

## Components

### Executor (`executor.go`)

Universal execution loop for all agent types.

**The Clean Loop:**
1. OBSERVE  → Transducer converts NL to Intent
2. ORIENT   → Build compilation context
3. JIT      → Compile prompt + config
4. LLM      → Generate response with tool calls
5. EXECUTE  → Route through VirtualStore
6. RESPOND  → Articulate to user

```go
executor := session.NewExecutor(kernel, virtualStore, llmClient,
                                 jitCompiler, configFactory, transducer)
result, err := executor.Process(ctx, "Fix the null pointer in auth.go")
```

### Spawner (`spawner.go`)

Dynamic SubAgent creation with JIT-driven configuration.

**SubAgent Types:**

| Type | Lifespan | Memory | Use Case |
|------|----------|--------|----------|
| **Ephemeral** | Single task | RAM only | Quick fixes, queries |
| **Persistent** | Multi-turn | RAM + history | Complex refactors |
| **System** | Long-running | RAM + snapshots | Background services |

```go
spawner := session.NewSpawner(...)
subagent, _ := spawner.SpawnForIntent(ctx, intent, task)
```

### SubAgent (`subagent.go`)

Context-isolated execution instance with independent conversation history.

**State Machine:**
```
IDLE → RUNNING → COMPLETED
              └→ FAILED
```

## Intent → Persona Mapping

| Intent Verbs | Agent | ConfigAtom |
|--------------|-------|------------|
| `/fix`, `/implement`, `/refactor` | `coder` | `/coder` |
| `/test`, `/cover`, `/verify` | `tester` | `/tester` |
| `/review`, `/audit` | `reviewer` | `/reviewer` |
| `/research`, `/learn` | `researcher` | `/researcher` |

## Code Reduction

| Component | Old | New | Reduction |
|-----------|-----|-----|-----------|
| All Shards | 35,000 lines | N/A | 100% |
| ShardManager | 12,000 lines | 385 lines | 97% |
| **Total** | **~35,000** | **~1,115** | **95%** |

## Thread Safety

- `Executor`: `sync.RWMutex` for conversation history
- `Spawner`: `sync.RWMutex` for subagent registry
- `SubAgent`: `atomic` operations for state

## See Also

- [JIT Compiler](../prompt/compiler.go)
- [ConfigFactory](../prompt/config_factory.go)
- [Intent Routing](../mangle/intent_routing.mg)

---

**Last Updated:** December 2024
