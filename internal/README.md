# internal/

The brain of codeNERD. This package contains all core logic, transducers, and the JIT-driven execution system.

**Architecture Version:** 2.0.0 (December 2024 - JIT-Driven)

## Package Overview

```
internal/
├── core/           # The Cortex - kernel, VirtualStore, fact management
├── session/        # **NEW** Session Executor, Spawner, SubAgents (JIT execution)
├── jit/            # **NEW** JIT configuration types and validation
├── tools/          # **NEW** Modular tool registry (core/, shell/, codedom/, research/)
├── perception/     # Input Transducer - NL → Mangle atoms + LLM providers
├── articulation/   # Output Transducer - Mangle atoms → NL + Piggyback
├── prompt/         # JIT Prompt Compiler, ConfigFactory, prompt atoms
├── mcp/            # Model Context Protocol integration, JIT Tool Compiler
├── shards/         # **REDUCED** Registration only (implementations removed)
├── mangle/         # Logic files - schemas.mg, policy.mg, intent_routing.mg
├── store/          # Memory tiers - RAM, Vector, Graph, Cold
├── campaign/       # Multi-phase goal orchestration + assault campaigns
├── browser/        # Rod-based browser automation
├── world/          # Filesystem, AST projection, holographic context
├── context/        # Context management, spreading activation, compression
├── config/         # Configuration management + LLM timeout consolidation
├── init/           # Workspace initialization
├── tactile/        # Motor cortex - sandboxed execution layer
├── autopoiesis/    # Self-learning: Ouroboros, Thunderdome, prompt evolution
├── embedding/      # Vector database operations, embedding engines
├── northstar/      # North Star goal tracking and alignment
├── testing/        # Test infrastructure (context_harness/)
├── retrieval/      # Semantic retrieval
├── regression/     # Regression testing
├── verification/   # Code verification
├── build/          # Build environment management
├── logging/        # Structured logging (22 categories)
├── types/          # Shared type definitions
├── system/         # System utilities
├── usage/          # Usage tracking
├── ux/             # User experience & journey tracking
└── transparency/   # Operation visibility & explanations
```

## Data Flow (JIT-Driven Architecture)

```
User Input
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│  PERCEPTION (perception/)                                   │
│  "Fix the auth bug" → user_intent(/mutation, /fix, "auth")  │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│  CORE (core/)                                               │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐   │
│  │ Kernel       │───▶│ Mangle Engine│───▶│ VirtualStore │   │
│  │ (facts)      │    │ (rules)      │    │ (FFI)        │   │
│  └──────────────┘    └──────────────┘    └──────────────┘   │
│                              │                              │
│                    intent_routing.mg                        │
│                              │                              │
│                              ▼                              │
│                     persona(/coder) derived                 │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│  JIT COMPILATION (prompt/ + jit/)                           │
│  ┌──────────────┐    ┌──────────────┐                       │
│  │ JIT Compiler │───▶│ ConfigFactory│                       │
│  │ (prompts)    │    │ (tools/cfg)  │                       │
│  └──────────────┘    └──────────────┘                       │
│           ↓                  ↓                              │
│    System Prompt      AgentConfig                           │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│  SESSION EXECUTOR (session/)                                │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐   │
│  │ Executor     │───▶│ SubAgent     │───▶│ Tool Router  │   │
│  │ (loop)       │    │ (isolated)   │    │ (tools/)     │   │
│  └──────────────┘    └──────────────┘    └──────────────┘   │
│                                                             │
│  LLM call with JIT-compiled prompt + tool access            │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│  ARTICULATION (articulation/)                               │
│  result atoms → "I fixed the auth bug in auth.go:42"        │
│  + Piggyback: task_status(/auth_fix, /complete)             │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
User Output
```

## Package Details

### core/

The Cortex kernel - fact management and VirtualStore FFI gateway.

| File | Purpose |
|------|---------|
| `kernel_*.go` | Mangle engine wrapper (modularized into 8 files) |
| `virtual_store*.go` | FFI gateway to external systems |
| `fact_categories.go` | Ephemeral vs persistent fact categorization |
| `learning.go` | Autopoiesis pattern tracking |

**Note:** `shard_manager.go` has been **removed** and replaced by `internal/session/`.

### session/ (**NEW** - JIT Execution)

The universal execution loop replacing hardcoded shards.

| File | Purpose |
|------|---------|
| `executor.go` | Universal execution loop (~391 lines) |
| `spawner.go` | Dynamic SubAgent creation (~385 lines) |
| `subagent.go` | Execution context and lifecycle (~339 lines) |
| `task_executor.go` | TaskExecutor interface |

### jit/ (**NEW**)

JIT configuration types and validation.

| File | Purpose |
|------|---------|
| `config/types.go` | AgentConfig schema |
| `config/validation.go` | Configuration validation |

### tools/ (**NEW** - Modular Tool Registry)

Any agent can use any tool via JIT selection.

| Directory | Purpose |
|-----------|---------|
| `core/` | Filesystem tools (read_file, write_file, glob, grep) |
| `shell/` | Execution tools (run_command, bash, run_build) |
| `codedom/` | Semantic code tools (get_elements, edit_lines) |
| `research/` | Research tools (web_search, web_fetch, context7_fetch) |

### perception/

Input transduction + multi-provider LLM client.

| File | Purpose |
|------|---------|
| `transducer.go` | NL → Mangle atom conversion |
| `intent.go` | Intent classification (query/mutation/instruction) |
| `focus.go` | Reference resolution with confidence scoring |
| `client_*.go` | Multi-provider LLM clients (7 providers) |

### articulation/

Output transduction - converts logic atoms back to natural language.

| File | Purpose |
|------|---------|
| `emitter.go` | Mangle atoms → NL + Piggyback Protocol |
| `formatter.go` | Response formatting and styling |
| `prompt_assembler.go` | Semantic Knowledge Bridge |

### prompt/

JIT Prompt Compiler and ConfigFactory.

| File | Purpose |
|------|---------|
| `compiler.go` | JIT prompt compilation |
| `config_factory.go` | Intent → tools/policies mapping |
| `atoms/` | Prompt atom library (identity, protocol, safety, etc.) |
| `selector.go` | Skeleton/flesh atom selection |
| `budget.go` | Token budget management |

### mcp/

Model Context Protocol integration.

| File | Purpose |
|------|---------|
| `compiler.go` | JIT Tool Compiler |
| `store.go` | Tool storage with embeddings |
| `analyzer.go` | LLM-based metadata extraction |
| `renderer.go` | Three-tier tool rendering |

### shards/ (**REDUCED**)

Legacy registration only - implementations removed.

| File | Purpose |
|------|---------|
| `registration.go` | Legacy command mapping to intents |
| `system/` | System utilities (not shard implementations) |

### mangle/

Logic files that define the policy.

| File | Purpose |
|------|---------|
| `schemas.mg` | EDB declarations (predicates) |
| `policy.mg` | IDB rules (20 sections of logic) |
| `intent_routing.mg` | **NEW** Declarative persona/action routing |
| `feedback/` | Feedback loop rules |
| `transpiler/` | Mangle transpilation |

### store/

Memory tiers for different persistence needs.

| Tier | Backing | Use Case |
|------|---------|----------|
| RAM | In-memory | Working memory, current session |
| Vector | Embeddings | Semantic search, similar code |
| Graph | Neo4j-style | Dependency relationships |
| Cold | SQLite | Long-term specialist knowledge |

### campaign/

Multi-phase goal orchestration for complex tasks.

| File | Purpose |
|------|---------|
| `orchestrator.go` | Main campaign loop |
| `decomposer.go` | Plan decomposition |
| `context_pager.go` | Phase-aware context management |
| `types.go` | Campaign, Phase, Task types |
| `assault_campaign.go` | Adversarial assault campaign builder |
| `assault_types.go` | Assault config, scopes, and stages |

### browser/

Rod-based browser automation for web interaction.

| File | Purpose |
|------|---------|
| `session.go` | Browser session management |
| `session_manager.go` | Multi-session management |
| `dom.go` | DOM projection and interaction |
| `snapshot.go` | Page state capture |

### world/

Filesystem and AST projection - the agent's view of the codebase.

| File | Purpose |
|------|---------|
| `fs.go` | Filesystem facts (file_topology) |
| `ast.go` | AST projection (symbol_graph) |
| `scanner.go` | Codebase scanning |
| `holographic.go` | Impact-aware context builder |
| `dataflow_multilang.go` | Multi-language taint analysis |
| `lsp/` | LSP integration |

### context/

Context management and spreading activation.

| File | Purpose |
|------|---------|
| `activation.go` | Spreading activation engine |
| `compressor.go` | Semantic compression |
| `feedback_store.go` | Feedback tracking |

### autopoiesis/

Self-learning and self-modification.

| File | Purpose |
|------|---------|
| `ouroboros.go` | Self-generating tools |
| `thunderdome.go` | Adversarial testing arena |
| `panic_maker.go` | Tactical tool breaker |
| `feedback.go` | Learning from experience |
| `prompt_evolution/` | System prompt learning |

### embedding/

Vector database operations.

| File | Purpose |
|------|---------|
| `engine.go` | Embedding engine interface |
| `genai.go` | Google GenAI embeddings |
| `task_selector.go` | Task-based embedding selection |

### testing/

Test infrastructure.

| Directory | Purpose |
|-----------|---------|
| `context_harness/` | Infinite context validation framework |

## Key Interfaces

### Kernel

```go
type Kernel interface {
    LoadFacts(facts []Fact) error
    Query(predicate string) ([]Fact, error)
    QueryAll() (map[string][]Fact, error)
    Assert(fact Fact) error
    Retract(predicate string) error
}
```

### TaskExecutor (NEW - replaces Shard interface)

```go
type TaskExecutor interface {
    Execute(ctx context.Context, intent string, task string) (string, error)
    ExecuteAsync(ctx context.Context, intent string, task string) (taskID string, err error)
    GetResult(taskID string) (result string, done bool, err error)
    WaitForResult(ctx context.Context, taskID string) (string, error)
}
```

### Transducer

```go
type Transducer interface {
    Transduce(ctx context.Context, input string) ([]core.Fact, error)
}
```

### LLMClient

```go
type LLMClient interface {
    Complete(ctx context.Context, prompt string) (string, error)
    CompleteWithSystem(ctx context.Context, system, user string) (string, error)
}
```

### ux/

User experience and journey tracking for adaptive interfaces.

| File | Purpose |
|------|---------|
| `manager.go` | Central UX coordinator |
| `user_state.go` | Journey state definitions (New → Learning → Productive → Power) |
| `journey_tracker.go` | State machine for user progression |
| `preferences.go` | Extended preferences with journey tracking |
| `help_triggers.go` | Contextual help detection |
| `onboarding.go` | First-run onboarding flow |

### transparency/

Operation visibility and explanation system.

| File | Purpose |
|------|---------|
| `transparency.go` | Main TransparencyManager coordinator |
| `shard_observer.go` | Tracks execution phases |
| `safety_reporter.go` | Reports and explains safety gate blocks |
| `explainer.go` | Builds human-readable explanations from proof trees |
| `error_classifier.go` | Categorizes errors with remediation suggestions |

## Design Principles

1. **Logic First** - All decisions derive from Mangle rules
2. **Fact Immutability** - Facts are append-only with explicit retraction
3. **JIT Configuration** - Agent behavior configured at runtime via ConfigFactory
4. **Constitutional Safety** - Every action requires `permitted(Action)`
5. **Glass Box** - All derivations are traceable via `nerd why`
6. **Progressive Disclosure** - UX adapts to user experience level
7. **Transparency on Demand** - Operations can be made visible via `/transparency`
8. **Modular Tools** - Any agent can use any tool via JIT selection

---

**Last Updated:** December 2024
**Architecture Version:** 2.0.0 (JIT-Driven)
