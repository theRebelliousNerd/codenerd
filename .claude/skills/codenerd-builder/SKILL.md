---
name: codenerd-builder
description: Build the codeNERD Logic-First Neuro-Symbolic coding agent framework. This skill should be used when implementing components of the codeNERD architecture including the Mangle kernel, Perception/Articulation Transducers, JIT Clean Loop, SubAgents, Modular Tool Registry, Virtual Predicates, TDD loops, Piggyback Protocol, Quiescent Boot, Dream State, Dreamer (Precog Safety), Legislator, Ouroboros Loop, and ConfigFactory. Use for tasks involving Google Mangle logic, Go runtime integration, or any neuro-symbolic agent development following the Creative-Executive Partnership pattern.
---

# codeNERD Builder

Build the codeNERD high-assurance Logic-First CLI coding agent.

> **Architecture Update (Dec 2024):** codeNERD now uses a **JIT Clean Loop** architecture. Domain shards (coder, tester, reviewer, researcher) have been **deleted** and replaced by JIT-driven SubAgents. All persona/identity comes from prompt atoms compiled at runtime. Tools are now modular via `internal/tools/` and any agent can use any tool via JIT selection.
>
> **Quiescent Boot (Dec 2024):** Sessions start fresh. Ephemeral facts (`user_intent`, `next_action`, etc.) are filtered at kernel boot. Use `/sessions` to load previous sessions explicitly.
>
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
[ Perception Transducer (LLM) ] --> [ Mangle Atoms (user_intent) ]
       |
[ Session Executor - The Clean Loop ]
       |
       +-> [ JIT Prompt Compiler ] --> [ Persona Atoms + Context ]
       +-> [ ConfigFactory ] --> [ Tools + Policies from Intent ]
       +-> [ LLM.CompleteWithTools() ]
       +-> [ Constitutional Gate ] --> [ Safety Check ]
       +-> [ Virtual Store (FFI) ] --> [ Tool Execution ]
       |
       +-> [ Spawner ] --> [ SubAgents (JIT-configured) ]
             +-> [ Autopoiesis ] → Ouroboros, Thunderdome
       |
[ Articulation Transducer (LLM) ] --> [ User Response ]
```

### The Clean Execution Loop (replaces old shard spawning)

```go
// internal/session/executor.go - ~50 lines replacing 5000+
func (e *Executor) Process(ctx context.Context, input string) (string, error) {
    // 1. Transducer: NL → intent
    intent := e.transducer.Transduce(ctx, input)
    e.kernel.Assert(intent.ToFact())

    // 2. JIT: Compile prompt (persona + skills + context)
    prompt := e.jitCompiler.Compile(ctx, e.buildContext(intent))

    // 3. JIT: Compile config (tools, policies)
    config := e.configFactory.Generate(ctx, prompt.Result, intent.Verb)

    // 4. LLM: Generate response with tool calls
    response, err := e.llm.CompleteWithTools(ctx, prompt.Prompt, input, config.Tools)

    // 5. Execute: Route tool calls through VirtualStore
    for _, call := range response.ToolCalls {
        if e.constitutionalGate.Permits(call) {
            e.virtualStore.Execute(ctx, call)
        }
    }

    // 6. Articulate: Response to user
    return e.articulator.Emit(response)
}
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

### SubAgents (JIT-Driven)

> **Dec 2024:** Domain shards (CoderShard, TesterShard, etc.) have been **deleted**. SubAgents are now JIT-configured via persona atoms and `ConfigFactory`.

| Type | Constant | Description | Memory |
|------|----------|-------------|--------|
| **Ephemeral** | `SubAgentTypeEphemeral` | Spawn → Execute → Die | RAM |
| **Persistent** | `SubAgentTypePersistent` | User-defined specialists | SQLite |
| **System** | `SubAgentTypeSystem` | Long-running services | RAM |

**Key Files:**

| File | Purpose |
|------|---------|
| `internal/session/spawner.go` | JIT-driven SubAgent spawning |
| `internal/session/subagent.go` | SubAgent lifecycle management |
| `internal/prompt/config_factory.go` | Intent → tools/policies mapping |
| `internal/prompt/atoms/identity/*.yaml` | Persona atoms (coder, tester, reviewer, researcher) |
| `internal/mangle/intent_routing.mg` | Mangle rules for persona selection |

See [references/shard-agents.md](references/shard-agents.md) for legacy context.

### Modular Tool Registry

> **Dec 2024:** Tools are modular and any agent can use any tool via JIT selection.

**Location:** `internal/tools/`

| Package | Tools | Purpose |
|---------|-------|---------|
| `core/` | read_file, write_file, glob, grep | Filesystem operations |
| `shell/` | run_command, bash, run_build | Shell execution |
| `codedom/` | get_elements, edit_lines | Semantic code operations |
| `research/` | web_search, web_fetch, context7 | Research tools |

Tools are routed via Mangle rules in `intent_routing.mg`:

```mangle
modular_tool_allowed(/read_file, Intent) :- user_intent(_, _, Intent, _, _).
modular_tool_allowed(/web_search, Intent) :- intent_category(Intent, /research).
```

### Quiescent Boot & Sessions

> **Dec 2024:** Sessions start fresh. Previous sessions loaded explicitly.

**Key Components:**

| File | Purpose |
|------|---------|
| `internal/core/fact_categories.go` | Defines ephemeral vs persistent predicates |
| `internal/core/kernel_init.go` | `filterBootFacts()` removes ephemeral facts at boot |
| `cmd/nerd/chat/session.go` | Fresh session generation, `/sessions` command |

**Ephemeral Predicates** (filtered at boot):
- `user_intent` - Current turn's intent
- `pending_action` - Actions awaiting execution
- `next_action` - Derived next action
- `active_tool` - Currently executing tool

**Session Commands:**
- `/sessions` - List and select previous sessions
- `/load-session <id>` - Load specific session
- `/new-session` - Start fresh (preserves old)

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
| **Session Executor** | `internal/session/executor.go` (The Clean Loop) |
| **Spawner** | `internal/session/spawner.go` (JIT-driven spawning) |
| **SubAgent** | `internal/session/subagent.go` (Context-isolated execution) |
| **ConfigFactory** | `internal/prompt/config_factory.go` (Intent → tools/policies) |
| **Persona Atoms** | `internal/prompt/atoms/identity/*.yaml` (coder, tester, reviewer, researcher) |
| **Intent Routing** | `internal/mangle/intent_routing.mg` (Mangle routing rules) |
| Kernel | `internal/core/kernel.go` (modularized into 8 files) |
| VirtualStore | `internal/core/virtual_store.go` |
| Dreamer | `internal/core/dreamer.go` |
| Transducer | `internal/perception/transducer.go` |
| SemanticClassifier | `internal/perception/semantic_classifier.go` |
| Emitter | `internal/articulation/emitter.go` |
| JITPromptCompiler | `internal/prompt/compiler.go` |
| OuroborosLoop | `internal/autopoiesis/ouroboros.go` |
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
