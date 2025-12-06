# internal/

The brain of codeNERD. This package contains all core logic, transducers, and shard implementations.

## Package Overview

```
internal/
├── core/           # The Cortex - kernel, memory, orchestration
├── perception/     # Input Transducer - NL → Mangle atoms
├── articulation/   # Output Transducer - Mangle atoms → NL
├── shards/         # ShardAgents - parallel task executors
├── mangle/         # Logic files - schemas.gl, policy.gl
├── store/          # Memory tiers - RAM, Vector, Graph, Cold
├── campaign/       # Multi-phase goal orchestration
├── browser/        # Rod-based browser automation
├── world/          # Filesystem and AST projection
├── context/        # Context management and paging
├── config/         # Configuration management
├── init/           # Workspace initialization
├── tactile/        # Tool execution layer
└── autopoiesis/    # Self-learning subsystem
```

## Data Flow

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
│                              ▼                              │
│                     next_action(/spawn_coder)               │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│  SHARDS (shards/)                                           │
│  CoderShard spawns → generates fix → returns result         │
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

The Cortex kernel - the decision-making center.

| File | Purpose |
|------|---------|
| `kernel.go` | Mangle engine wrapper, fact management |
| `virtual_store.go` | FFI gateway to external systems |
| `shard_manager.go` | Shard lifecycle and orchestration |
| `learning.go` | Autopoiesis pattern tracking |

### perception/

Input transduction - converts natural language to logic atoms.

| File | Purpose |
|------|---------|
| `transducer.go` | NL → Mangle atom conversion |
| `intent.go` | Intent classification (query/mutation/instruction) |
| `focus.go` | Reference resolution with confidence scoring |

### articulation/

Output transduction - converts logic atoms back to natural language.

| File | Purpose |
|------|---------|
| `emitter.go` | Mangle atoms → NL + Piggyback Protocol |
| `formatter.go` | Response formatting and styling |

### shards/

ShardAgents for parallel task execution.

| File | Purpose |
|------|---------|
| `coder.go` | Code generation, refactoring, fixes |
| `tester.go` | Test creation, TDD loops |
| `reviewer.go` | Code review, security audit |
| `researcher/` | Deep research subsystem |
| `system/` | Built-in system shards |
| `tool_generator.go` | Ouroboros Loop - self-generating tools |

### mangle/

Logic files that define the policy.

| File | Purpose |
|------|---------|
| `schemas.gl` | EDB declarations (predicates) |
| `policy.gl` | IDB rules (20 sections of logic) |
| `coder.gl` | Coder-specific rules |
| `tester.gl` | Tester-specific rules |
| `reviewer.gl` | Reviewer-specific rules |

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

### browser/

Rod-based browser automation for web interaction.

| File | Purpose |
|------|---------|
| `session.go` | Browser session management |
| `dom.go` | DOM projection and interaction |
| `snapshot.go` | Page state capture |

### world/

Filesystem and AST projection - the agent's view of the codebase.

| File | Purpose |
|------|---------|
| `fs.go` | Filesystem facts (file_topology) |
| `ast.go` | AST projection (symbol_graph) |
| `scanner.go` | Codebase scanning |

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

### Shard

```go
type Shard interface {
    ID() string
    Config() ShardConfig
    State() ShardState
    Execute(ctx context.Context, task string) (string, error)
    Shutdown() error
}
```

### Transducer

```go
type Transducer interface {
    Transduce(ctx context.Context, input string) ([]core.Fact, error)
}
```

## Design Principles

1. **Logic First** - All decisions derive from Mangle rules
2. **Fact Immutability** - Facts are append-only with explicit retraction
3. **Shard Isolation** - Each shard has its own kernel instance
4. **Constitutional Safety** - Every action requires `permitted(Action)`
5. **Glass Box** - All derivations are traceable via `nerd why`
