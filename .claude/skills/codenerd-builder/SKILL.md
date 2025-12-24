---
name: codenerd-builder
description: Build the codeNERD Logic-First Neuro-Symbolic coding agent framework. This skill should be used when implementing components of the codeNERD architecture including the Mangle kernel, Perception/Articulation Transducers, ShardAgents, Virtual Predicates, TDD loops, Piggyback Protocol, Dream State, Dreamer (Precog Safety), Legislator, Ouroboros Loop, and DifferentialEngine. Use for tasks involving Google Mangle logic, Go runtime integration, or any neuro-symbolic agent development following the Creative-Executive Partnership pattern.
---

# codeNERD Builder

Build the codeNERD high-assurance Logic-First CLI coding agent.

> **Stability Notice:** This codebase is under active development. Code snippets illustrate architectural patterns but may not match current implementations exactly. Always read the actual source files.

## Build Instructions

**IMPORTANT: To enable sqlite-vec for vector DB support:**

```powershell
$env:CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers"; go build ./cmd/nerd
```

## Integrated Skills Ecosystem

codeNERD development leverages a constellation of specialized skills:

| Skill | Domain | When to Use |
|-------|--------|-------------|
| **codenerd-builder** | Architecture | Kernel, transducers, shards, campaigns |
| **mangle-programming** | Logic | Schemas, rules, queries, aggregation |
| **go-architect** | Go Patterns | All Go code, concurrency, errors |
| **charm-tui** | Terminal UI | CLI interface, forms, lists |
| **research-builder** | Knowledge | ResearcherShard, llms.txt, 4-tier memory |
| **rod-builder** | Browser | Web scraping, CDP, automation |
| **log-analyzer** | Debugging | Log analysis, cross-system tracing |
| **prompt-architect** | Prompts | Shard prompts, JIT compilation, Piggyback |

For complete skill documentation with trigger conditions and integration points, see [references/skill-registry.md](references/skill-registry.md).

## Core Philosophy

Current AI agents make a category error: they ask LLMs to handle everything—creativity AND planning. codeNERD separates these concerns through a **Creative-Executive Partnership**:

- **LLM as Creative Center**: Problem-solving, solution synthesis, goal-crafting
- **Logic as Executive**: Planning, memory, orchestration, safety derive from Mangle rules
- **Transduction Interface**: NL↔Logic atom conversion channels creativity through formal structure

## Architecture Overview

```text
[ Terminal / User ]
       |
[ Perception Transducer (LLM) ] --> [ Mangle Atoms ]
       |
[ Cortex Kernel ]
       |
       +-> [ FactStore (RAM) ]
       +-> [ Mangle Engine ]
       +-> [ JIT Prompt Compiler ]
       +-> [ Dreamer (Precog Safety) ]
       +-> [ Virtual Store (FFI) ]
             +-> [ Shard Manager ] → Type A/B/U/S Shards
             +-> [ Autopoiesis ] → Ouroboros, Nemesis, Thunderdome
       |
[ Articulation Transducer (LLM) ] --> [ User Response ]
```

For detailed architecture, see [references/architecture.md](references/architecture.md).

## Neuro-Symbolic Design Principles

### The "Mangle as HashMap" Anti-Pattern

**CRITICAL**: Mangle is for deduction, not data lookup. It has no fuzzy matching.

```mangle
# WRONG - expecting fuzzy matching
intent_definition("review my code", /review, /codebase).
# User says "inspect my code" → NO MATCH!
```

**Solution**: Use vector embeddings for semantic matching, then let Mangle reason over the results.

| Need | Wrong Tool | Right Tool |
|------|------------|------------|
| Semantic matching | Mangle facts | Vector embeddings |
| If X then Y | ML classifier | Mangle rules |
| All paths A→B | Graph code | Mangle transitive closure |

### Functions That Don't Exist

These are commonly hallucinated and will fail silently:

- `fn:string_contains` ❌
- `fn:substring` ❌
- `fn:regex` ❌
- `fn:like` ❌

**Valid**: `fn:plus`, `fn:minus`, `fn:Count`, `fn:Sum`, `fn:group_by`, `fn:collect`, `fn:list`, `fn:len`.

See [mangle-programming skill](../mangle-programming/SKILL.md) for complete reference.

## Key Components

### Perception Transducer

Converts user input to Mangle atoms (`user_intent`, `focus_resolution`).

```mangle
Decl user_intent(ID, Category, Verb, Target, Constraint).
```

See [references/semantic-classification.md](references/semantic-classification.md).

### Articulation & Piggyback Protocol

Dual-channel output: visible `surface_response` for user + hidden `control_packet` for kernel.

```json
{
  "surface_response": "I fixed the bug.",
  "control_packet": {
    "mangle_updates": ["task_status(/fix, /complete)"]
  }
}
```

See [references/piggyback-protocol.md](references/piggyback-protocol.md).

### JIT Prompt Compiler

Dynamic prompt assembly from atomic components:

| Component | Location | Purpose |
|-----------|----------|---------|
| JITPromptCompiler | `internal/prompt/compiler.go` | Orchestration |
| AtomSelector | `internal/prompt/selector.go` | Vector + Mangle selection |
| TokenBudgetManager | `internal/prompt/budget.go` | Budget allocation |

See [prompt-architect skill](../prompt-architect/SKILL.md).

### Shard Agents

| Type | Constant | Description | Memory |
|------|----------|-------------|--------|
| **A** | `ShardTypeEphemeral` | Spawn → Execute → Die | RAM |
| **B** | `ShardTypePersistent` | Domain specialists | SQLite |
| **U** | `ShardTypeUser` | User-defined specialists | SQLite |
| **S** | `ShardTypeSystem` | Long-running services | RAM |

Built-in shards: CoderShard, TesterShard, ReviewerShard, ResearcherShard, NemesisShard, Legislator.

See [references/shard-agents.md](references/shard-agents.md).

### Dreamer (Precog Safety)

Simulates action impact before execution, checking for `panic_state` derivations.

```go
result := dreamer.SimulateAction(ctx, ActionRequest{...})
if result.Unsafe {
    return errors.New(result.Reason)
}
```

### Autopoiesis System

Self-modification via:

- **OuroborosLoop**: Tool self-generation with safety checks
- **Thunderdome**: Adversarial battle arena for generated tools
- **NemesisShard**: Strategic adversary that breaks patches

See [references/autopoiesis.md](references/autopoiesis.md).

### Campaign Orchestration

Multi-phase goal execution with context paging:

| Phase | Budget % |
|-------|----------|
| Core reserve | 5% |
| Current phase | 30% |
| History | 15% |
| Working memory | 40% |
| Prefetch | 10% |

See [references/campaign-orchestrator.md](references/campaign-orchestrator.md).

## Key Implementation Patterns

### Pattern 1: Hallucination Firewall

Every action requires `permitted(Action)` to derive:

```go
if !k.Mangle.Query("permitted(?)", action.Name) {
    return ErrAccessDenied
}
```

### Pattern 2: OODA Loop

```text
Observe → Orient → Decide → Act
Transducer → Spreading Activation → Mangle Engine → Virtual Store
```

### Pattern 3: TDD Repair Loop

```mangle
next_action(/read_error_log) :- test_state(/failing), retry_count(N), N < 3.
next_action(/analyze_root_cause) :- test_state(/log_read).
next_action(/generate_patch) :- test_state(/cause_found).
```

## Mangle Logic Files

Core logic files in `internal/core/defaults/`:

- `schemas.mg` - Core schema declarations (78KB)
- `policy.mg` - Constitutional rules (81KB)
- `coder.mg`, `tester.mg`, `reviewer.mg` - Shard logic
- `campaign_rules.mg` - Campaign orchestration

## Logging System

22 categories in `.nerd/config.json`:

| Category | System | Key Events |
|----------|--------|------------|
| `kernel` | Mangle Engine | Fact assertion, queries |
| `shards` | Shard Manager | Spawn, execute, destroy |
| `perception` | Transducer | Intent extraction |
| `autopoiesis` | Self-Improvement | Learning, Ouroboros |

See [references/logging-system.md](references/logging-system.md).

## Key Implementation Locations

| Component | Location |
|-----------|----------|
| Kernel | `internal/core/kernel.go` (modularized into 8 files) |
| VirtualStore | `internal/core/virtual_store.go` |
| ShardManager | `internal/core/shard_manager.go` |
| Dreamer | `internal/core/dreamer.go` |
| Transducer | `internal/perception/transducer.go` |
| SemanticClassifier | `internal/perception/semantic_classifier.go` |
| Emitter | `internal/articulation/emitter.go` |
| JITPromptCompiler | `internal/prompt/compiler.go` |
| OuroborosLoop | `internal/autopoiesis/ouroboros.go` |
| NemesisShard | `internal/shards/nemesis/nemesis.go` |
| Thunderdome | `internal/autopoiesis/thunderdome.go` |

## Reference Documentation

| Reference | Contents |
|-----------|----------|
| [architecture.md](references/architecture.md) | Theoretical foundations, neuro-symbolic principles |
| [mangle-schemas.md](references/mangle-schemas.md) | Complete Mangle schema reference |
| [implementation-guide.md](references/implementation-guide.md) | Go implementation patterns |
| [piggyback-protocol.md](references/piggyback-protocol.md) | Dual-channel control protocol |
| [campaign-orchestrator.md](references/campaign-orchestrator.md) | Multi-phase execution, context paging |
| [autopoiesis.md](references/autopoiesis.md) | Self-creation, Ouroboros, DifferentialEngine |
| [shard-agents.md](references/shard-agents.md) | Shard types, ShardManager API |
| [logging-system.md](references/logging-system.md) | 22-category logging system |
| [skill-registry.md](references/skill-registry.md) | Complete skill ecosystem |
| [semantic-classification.md](references/semantic-classification.md) | Neuro-symbolic intent classification |

## Quick Reference

**OODA Loop:** Observe (Transducer) → Orient (Spreading Activation) → Decide (Mangle) → Act (VirtualStore)

**Constitutional Safety:** Every action requires `permitted(Action)` to derive. Default deny.

**Fact Flow:** User → Transducer → `user_intent` → Kernel derives `next_action` → VirtualStore executes → Response
