# DEPRECATED: Shard Agents (Replaced by JIT-Driven Execution)

**Status:** DEPRECATED as of December 2024
**Replaced By:** [JIT-Driven Execution Model](./jit-execution-model.md)

---

## ⚠️ This Document is Obsolete

The **hardcoded shard architecture** described in this document has been **completely replaced** by the **JIT-driven execution model**.

### What Changed?

**December 2024:** codeNERD underwent a fundamental architectural refactor:

- **Removed:** 35,000 lines of hardcoded shard implementations (CoderShard, TesterShard, ReviewerShard, etc.)
- **Added:** 1,600 lines of JIT-driven execution infrastructure (Session Executor, ConfigFactory, Intent Routing)
- **Result:** 95% code reduction, runtime configurability, zero boilerplate for new agents

### Where to Find Current Documentation

For up-to-date information on how codeNERD agents work, see:

1. **[JIT-Driven Execution Model](./jit-execution-model.md)** - Complete guide to the new architecture
2. **[Architecture Changes](../../../conductor/tracks/jit_refactor_20251226/ARCHITECTURE_CHANGES.md)** - Migration guide and metrics
3. **[Main README](../../../README.md)** - Updated architecture overview
4. **[CLAUDE.md](../../../CLAUDE.md)** - Project context with new architecture

---

## Quick Migration Guide

### Old Concept → New Equivalent

| Old (Deprecated) | New (Current) |
|------------------|---------------|
| **CoderShard** (3,500 lines) | Intent Routing rule: `persona(/coder)` + prompt atoms |
| **TesterShard** (2,800 lines) | Intent Routing rule: `persona(/tester)` + prompt atoms |
| **ReviewerShard** (8,500 lines) | Intent Routing rule: `persona(/reviewer)` + prompt atoms |
| **ResearcherShard** (5,200 lines) | Intent Routing rule: `persona(/researcher)` + prompt atoms |
| **ShardManager** (12,000 lines) | Session Executor (391 lines) + Spawner (385 lines) |
| **Type 1: System Shards** | System SubAgents (persistent background) |
| **Type 2: Ephemeral Shards** | Ephemeral SubAgents (single-task) |
| **Type 3: Persistent Shards** | Persistent SubAgents (multi-turn) |
| **Type 4: User-Configured** | Custom ConfigAtoms + prompt atoms |

### Key Implementation Changes

| Component | Old Location | New Location |
|-----------|-------------|--------------|
| Shard execution logic | `internal/shards/*/shard.go` | **DELETED** |
| Shard Manager | `internal/core/shard_manager.go` | **DELETED** |
| Universal Executor | N/A | `internal/session/executor.go` |
| Subagent Management | N/A | `internal/session/spawner.go` |
| Intent Routing | Hardcoded in ShardManager | `internal/mangle/intent_routing.mg` |
| Configuration | Hardcoded in each shard | `internal/prompt/config_factory.go` |

---

## Architecture Comparison

### Old Architecture (Pre-Dec 2024)

```text
User Input → Perception → user_intent
                              ↓
                       ShardManager
                              ↓
            ┌─────────────────┼─────────────────┐
            ↓                 ↓                 ↓
       CoderShard        TesterShard      ReviewerShard
     (3,500 lines)      (2,800 lines)    (8,500 lines)
        (hardcoded         (hardcoded       (hardcoded
         logic)             logic)           logic)
```

### New Architecture (Post-Dec 2024)

```text
User Input → Perception → user_intent atoms
                              ↓
                    Intent Routing (Mangle)
                              ↓
               ┌──────────────┴──────────────┐
               ↓                             ↓
       ConfigFactory                  JIT Compiler
   (Generate AgentConfig)         (Compile Prompt)
               ↓                             ↓
               └──────────────┬──────────────┘
                              ↓
                      Session Executor
                      (391 lines)
                              ↓
                  LLM + VirtualStore + Safety
```

---

## Why the Change?

### Problems with Hardcoded Shards

1. **Massive boilerplate:** 35,000 lines of redundant code
2. **Tight coupling:** Behavior changes required code modifications
3. **High maintenance:** Each shard needed separate testing and updates
4. **Inflexibility:** Adding new personas required 500-2000 lines of Go code

### Benefits of JIT-Driven Execution

1. **95% code reduction:** 35,000 → 1,600 lines
2. **Runtime configurability:** Update .mg files or YAML, no recompile
3. **Zero boilerplate:** New personas via declarative config only
4. **Cleaner separation:** LLM (creative) vs Logic (executive) vs Go (infrastructure)

---

## For Historical Reference Only

The remainder of this document describes the **old shard architecture** for historical purposes only. This architecture **no longer exists** in the codebase.

<details>
<summary>Historical Documentation (Deprecated)</summary>

## 1. The Hypervisor Pattern (DEPRECATED)

codeNERD **used to implement** Fractal Cognition through ShardAgents—miniature, hyper-focused agent kernels that spawned, executed specific sub-tasks in total isolation, and returned distilled logical results.

**This is no longer how the system works.** See [JIT-Driven Execution Model](./jit-execution-model.md) for current architecture.

## 2. The Four Shard Types (DEPRECATED)

The old system had four shard types:
- **Type 1: System Level** (permanent background shards)
- **Type 2: Ephemeral** (spawn → die, RAM only)
- **Type 3: Persistent** (long-running, SQLite-backed)
- **Type 4: User Configured** (specialist shards with domain knowledge)

**These types have been replaced** by three SubAgent types in the new architecture:
- **Ephemeral SubAgents** (single-task, RAM only)
- **Persistent SubAgents** (multi-turn, conversation history)
- **System SubAgents** (long-running background services)

## 3. Built-in Shard Implementations (DELETED)

The following implementations **no longer exist**:
- `internal/shards/coder/` - **DELETED**
- `internal/shards/tester/` - **DELETED**
- `internal/shards/reviewer/` - **DELETED**
- `internal/shards/researcher/` - **DELETED**
- `internal/shards/nemesis/` - **DELETED**
- `internal/shards/tool_generator/` - **DELETED**
- `internal/core/shard_manager.go` - **DELETED**

Their functionality has been replaced by:
- Intent routing rules in `internal/mangle/intent_routing.mg`
- Prompt atoms in `internal/prompt/atoms/identity/*.yaml`
- ConfigFactory-generated configurations
- Universal Session Executor

</details>

---

## Next Steps

1. Read [JIT-Driven Execution Model](./jit-execution-model.md) for complete current architecture
2. Review [Architecture Changes](../../../conductor/tracks/jit_refactor_20251226/ARCHITECTURE_CHANGES.md) for migration details
3. Check [Intent Routing Rules](../../../internal/mangle/intent_routing.mg) for persona mapping logic
4. Explore [Session Executor](../../../internal/session/executor.go) for execution implementation

**Do not** reference this document for current system behavior. It is preserved only for historical context.

---

**Last Updated:** December 27, 2024
**Deprecated Since:** December 26, 2024
**Replacement:** [JIT-Driven Execution Model](./jit-execution-model.md)
