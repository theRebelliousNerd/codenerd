# codeNERD

A high-assurance Logic-First CLI coding agent built on the Neuro-Symbolic architecture.

## PUSH TO GITHUB ALL THE TIME

## FOR ALL NEW LLM SYSTEMS, JIT IS THE STANDARD, ALWAYS CREATE NEW PROMPT ATOMS AND USE THE JIT SYSTEM. IT IS THE CENTRAL PARADIGM OF THE SYSTEM. JIT, PIGGYBACKING, CONTROL PACKETS, MANGLE, THAT IS THE NAME OF THE GAME!

All prompt atoms from internal go into: `C:\CodeProjects\codeNERD\internal\prompt\atoms`

All prompt atoms from project-specific shards go to: `C:\CodeProjects\codeNERD\.nerd\agents`

**Kernel:** Google Mangle (Datalog) | **Runtime:** Go | **Philosophy:** Logic determines Reality; the Model merely describes it.

> [!IMPORTANT]
> **Build Instruction for Vector DB Support**
> To enable `sqlite-vec` mappings, you MUST use the following build command:
>
> ```bash
> rm c:/CodeProjects/codeNERD/nerd.exe 2>/dev/null; cd c:/CodeProjects/codeNERD && CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers" go build -tags=sqlite_vec -o nerd.exe ./cmd/nerd 2>&1 | grep -v "warning:" | grep -v note:
> ```

## Vision

The current generation of AI coding agents makes a category error: they ask LLMs to handle everything—creativity AND planning, insight AND memory, problem-solving AND self-correction—when LLMs excel at the former but struggle with the latter. codeNERD separates these concerns: the LLM remains the creative center where problems are understood, solutions are synthesized, and novel approaches emerge, while a deterministic Mangle kernel handles the executive functions that LLMs cannot reliably perform—planning, long-term memory, skill retention, and self-reflection. This architecture liberates the LLM to focus purely on what it does best while the harness ensures those creative outputs are channeled safely and consistently. The north star is an autonomous agent that pairs unbounded creative problem-solving with formal correctness guarantees: months-long sessions without context exhaustion, learned preferences without retraining, and parallel sub-agents—all orchestrated by logic, not luck. We are building the first coding agent where creative power and deterministic safety coexist by design.

## Core Principle: Inversion of Control

codeNERD inverts the traditional agent hierarchy:

- **LLM as Creative Center** - Problem-solving, solution synthesis, goal-crafting, and insight remain with the model
- **Logic as Executive** - Planning, memory, orchestration, and safety derive from deterministic Mangle rules
- **Transduction Interface** - NL↔Logic atom conversion channels creativity through formal structure

## Project Structure

```text
cmd/
├── nerd/               # CLI entrypoint (67 Go files)
│   ├── chat/           # TUI chat interface (Elm architecture)
│   └── ui/             # UI components
├── query-kb/           # Knowledge base query tool
├── test-research/      # Research testing tool
└── tools/              # Build tools (corpus_builder, mangle_check, etc.)

internal/               # 30 packages, ~128K LOC
├── core/               # Kernel, VirtualStore, ShardManager (modularized)
├── perception/         # NL → Mangle atom transduction
├── articulation/       # Mangle atom → NL transduction
├── autopoiesis/        # Self-modification: Ouroboros, Thunderdome, tool learning
├── prompt/             # JIT Prompt Compiler, atoms, context-aware assembly
├── shards/             # Shard implementations (coder/, tester/, reviewer/, researcher/, nemesis/, system/, tool_generator/)
├── mangle/             # .mg schema/policy files + feedback/, transpiler/
├── store/              # Memory tiers (modularized into 10 files)
├── campaign/           # Multi-phase goal orchestration (25+ files)
├── world/              # Filesystem, AST projection, multi-lang data flow
├── context/            # Spreading activation, semantic compression
├── embedding/          # Vector database operations
├── browser/            # Browser automation via Rod
├── tactile/            # Motor cortex: shell execution, sandboxing, SWE-bench
├── testing/            # Test infrastructure (context_harness/)
├── transparency/       # Operation visibility layer
├── ux/                 # User experience management
├── config/             # Configuration management
├── init/               # Workspace initialization
├── logging/            # Structured logging (22 categories)
├── types/              # Shared type definitions
├── system/             # System shards and utilities
├── usage/              # Usage tracking
├── retrieval/          # Semantic retrieval
├── regression/         # Regression testing
├── build/              # Build environment management
└── verification/       # Code verification
```

## Architecture and Philosophy

### 1. Architecture

codeNERD utilizes a Neuro-Symbolic architecture designed to bridge the gap between probabilistic Large Language Models (LLMs) and deterministic execution environments. It functions as a high-assurance coding agent framework.

**Neuro-Symbolic & Creative-Executive Partnership:**
The system fundamentally separates concerns into two distinct domains:
- **Creative Center (LLM):** Responsible for problem-solving, solution synthesis, and insight generation. It handles ambiguity and creativity.
- **Executive (Logic/Mangle):** Responsible for planning, memory, orchestration, and safety. It uses deterministic rules to harness the LLM's output.

**Perception Transducer:**
- *Implementation:* `internal/perception/transducer.go`
- *Function:* Converts unstructured natural language user input into formal logic "atoms" (e.g., user_intent). It grounds fuzzy references to concrete file paths and symbols, calculating confidence scores to trigger clarification loops if necessary.

**Articulation Transducer:**
- *Implementation:* `internal/articulation/emitter.go`
- *Function:* Converts internal logic states and atomic facts back into natural language for the user. It ensures that the user sees a helpful response while the system maintains precise logical state.

**World Model (Extensional Database - EDB):**
- *Implementation:* `internal/world/fs.go`, `internal/world/ast.go`
- *Function:* Maintains the "Ground Truth" of the project. It projects the filesystem and abstract syntax trees (AST) into logic facts (e.g., file_topology, symbol_graph, dependency_link), allowing the logic engine to reason about the codebase structure and state.

**Executive Policy (Intensional Database - IDB):**
- *Implementation:* `internal/mangle/policy.mg`, `internal/core/kernel.go`
- *Function:* A collection of deductive rules that derive the system's next_action. It encodes workflows like TDD repair loops (test_state → next_action) and enforces safety constraints.

**Virtual Predicates:**
- *Implementation:* `internal/core/virtual_store.go`
- *Function:* Serves as a Foreign Function Interface (FFI) that abstracts external APIs (filesystem, shell, MCP tools) into logic predicates. When the logic engine queries a virtual predicate (e.g., file_content), it triggers the actual underlying system call.

**Shard Agents:** See dedicated "Shard Architecture" section below for comprehensive coverage of shard types, lifecycle, and implementations.

**Memory Tiers:** See "Memory Tiers" table in Nomenclature section below.

**Piggyback Protocol:**
- *Implementation:* `internal/articulation/emitter.go`
- *Function:* A dual-channel communication protocol. The agent outputs a JSON object containing a visible surface_response for the user and a hidden control_packet for the kernel. This allows the agent to update its internal logical state (e.g., task_status) independently of the conversational text.

### 2. Core Philosophy

**Logic-First CLI:**
Unlike chat-first agents, codeNERD is driven by a logic kernel. Text generation is a side effect of logical processes, not the primary driver. The state of the system is defined by facts, not conversation history.

**Separation of Concerns:**
By decoupling creativity (LLM) from execution (Mangle Engine), codeNERD prevents the LLM from hallucinating actions or violating safety protocols. The LLM suggests; the Kernel executes.

**Deterministic Safety:**
Safety is not a prompt instruction but a logic rule. The "Constitutional Gate" (`permitted(Action) :- safe_action(Action)`) ensures that dangerous actions (like `rm -rf`) are blocked deterministically unless specific override conditions are met.

**LLM as Creative Center:**
The architecture acknowledges that LLMs excel at synthesis and pattern matching. codeNERD leverages this by feeding the LLM highly specific, logic-derived context ("Context Atoms") and asking it to solve specific problems, rather than asking it to manage the entire workflow.

### 3. Implementation Patterns

**Hallucination Firewall:**
- *Pattern:* `permitted(Action)` check
- *Details:* Every action proposed by the Transducer or a Shard is validated against the Mangle logic policy. If the logic cannot derive a permission rule for the action, it is strictly blocked, preventing the execution of hallucinated or malicious commands.

**Grammar-Constrained Decoding:**
- *Pattern:* Schema validation and recovery
- *Details:* Output from the LLM is forced to conform to strict Mangle syntax and JSON schemas. This ensures that the "thoughts" of the agent can be parsed and executed reliably by the deterministic kernel.

**OODA Loop:**
- *Pattern:* Observe → Orient → Decide → Act
- *Details:* The system cycles through:
  1. **Observe:** Transducer converts input to atoms
  2. **Orient:** Spreading Activation selects relevant context facts based on logical dependencies
  3. **Decide:** Mangle Engine derives the single best next_action
  4. **Act:** Virtual Store executes the tool or command

**Autopoiesis (Self-Learning):**
- *Pattern:* Runtime feedback loops (`internal/autopoiesis/`)
- *Details:* The system tracks rejection and acceptance of its actions. Repeated rejections of a specific pattern trigger a preference_signal, which promotes a new rule to long-term memory. The Ouroboros Loop detects missing capabilities and can trigger a generate_tool action to self-implement missing functionality.

**Campaign Orchestration:**
- *Pattern:* Context Paging and Multi-Phase Goals (`internal/campaign/`)
- *Details:* For complex goals (e.g., migrations), the system breaks the work into phases. It uses "Context Paging" to manage token budget, loading only the context relevant to the current phase while keeping core facts and working memory available.

## Shard Architecture

Shards are specialized sub-agents that handle domain-specific tasks in parallel. The ShardManager (`internal/core/shard_manager.go`, modularized into 5 files) orchestrates their lifecycle.

### Lifecycle Types

| Type | Constant | Description | Memory | Creation |
|------|----------|-------------|--------|----------|
| **Type A** | `ShardTypeEphemeral` | Generalist agents. Spawn → Execute → Die. | RAM only | `/review`, `/test`, `/fix` |
| **Type B** | `ShardTypePersistent` | Domain specialists with pre-loaded knowledge. | SQLite-backed | `/init` project setup |
| **Type U** | `ShardTypeUser` | User-defined specialists via wizard. | SQLite-backed | `/define-agent` |
| **Type S** | `ShardTypeSystem` | Long-running system services. | RAM | Auto-start |

### Shard Implementations

| Shard | Location | Purpose |
|-------|----------|---------|
| **CoderShard** | `internal/shards/coder/` | Code generation, file edits, refactoring |
| **TesterShard** | `internal/shards/tester/` | Test execution, coverage analysis |
| **ReviewerShard** | `internal/shards/reviewer/` | Code review, pre-flight checks, hypothesis verification |
| **ResearcherShard** | `internal/shards/researcher/` | Knowledge gathering, documentation ingestion |
| **NemesisShard** | `internal/shards/nemesis/` | Adversarial testing, patch breaking (see Adversarial Engineering) |
| **ToolGenerator** | `internal/shards/tool_generator/` | Ouroboros: self-generating tools |
| **Legislator** | `internal/shards/system/legislator.go` | Compiles new Mangle rules at runtime |

### System Shards (Type S)

Long-running background services that maintain system state:

| Shard | Purpose |
|-------|---------|
| `perception_firewall` | NL → atoms transduction |
| `world_model_ingestor` | file_topology, symbol_graph maintenance |
| `executive_policy` | next_action derivation |
| `constitution_gate` | Safety enforcement |
| `tactile_router` | Action → tool routing |
| `session_planner` | Agenda/campaign orchestration |
| `nemesis` | Adversarial co-evolution, patch breaking |

### Go Interface

All shards implement the `Shard` interface:

```go
type Shard interface {
    Execute(ctx context.Context, task ShardTask) (string, error)
}
```

### ShardManager Modularization

| File | Purpose |
|------|---------|
| `shard_manager_core.go` | ShardManager struct, core operations |
| `shard_manager_spawn.go` | Shard spawning and execution |
| `shard_manager_tools.go` | Intelligent tool routing |
| `shard_manager_facts.go` | Fact conversion utilities |
| `shard_manager_feedback.go` | Reviewer feedback interface |

## Adversarial Engineering: Nemesis & Panic Maker

codeNERD employs an Adversarial Co-Evolution strategy. Instead of relying solely on passive testing, it actively attempts to break its own code using two distinct but related components: the Panic Maker (tactical tool breaker) and the Nemesis Shard (strategic system breaker).

### Panic Maker (The Tactical Breaker)

- *Implementation:* `internal/autopoiesis/panic_maker.go`
- *Scope:* Micro-level. Focused on breaking individual tools and functions during the generation phase (Ouroboros loop).
- *Workflow:*
  1. **Static Analysis:** Analyzes the generated tool's source code to identify specific vulnerability patterns (e.g., pointer dereferences, channel operations)
  2. **Attack Vector Generation:** Uses the LLM to craft targeted JSON inputs designed to trigger crashes
  3. **Thunderdome:** Executes the attacks against the tool. If the tool crashes (panics, OOMs, deadlocks), it is rejected and sent back for hardening
- *Attack Categories:*
  - `nil_pointer`: Exploits unchecked pointer dereferences
  - `boundary`: Max int, negative indices, empty slices
  - `resource`: Massive allocations to trigger OOM
  - `concurrency`: Race conditions and channel deadlocks
  - `format`: Malformed JSON/XML inputs

### Nemesis Shard (The Strategic Adversary)

- *Implementation:* `internal/shards/nemesis/nemesis.go`
- *Scope:* System-level. A persistent "Type B" Specialist Shard that acts as a gatekeeper for code changes.
- *Philosophy:* "The Nemesis does not seek destruction - it seeks truth." It acts as a hostile sparring partner for the Coder Shard.
- *Core Capabilities:*
  - **The Gauntlet:** A required pipeline phase. A patch is only "battle-hardened" if it survives the Nemesis.
  - **Attack Tool Generation:** Unlike Panic Maker (which sends inputs), Nemesis writes and compiles full Go attack binaries to exploit logic flaws or race conditions.
  - **Lazy Pattern Detection:** Actively scans for "lazy" coding patterns (e.g., `return nil`, generic error messages) that signal weakness.
  - **The Armory:** (`internal/shards/nemesis/armory.go`) A persistent store where Nemesis remembers effective attack strategies.

### Comparison

| Feature | Scope | Method | Goal |
|---------|-------|--------|------|
| Panic Maker | Single Function/Tool | Malformed Inputs (Fuzzing) | Ensure tool robustness before use |
| Nemesis | Full System/Patch | Compilable Attack Programs | Reject weak architecture & logic |

## Thunderdome: The Adversarial Battleground

Thunderdome is the conceptual and operational environment within codeNERD where adversarial attacks are executed against generated code and submitted patches. It serves as the ultimate proving ground for code resilience, feeding back results that drive the autopoietic self-improvement loops.

**Role and Functionality:**
- **Adversarial Testing Environment:** Thunderdome is where the offensive capabilities of the PanicMaker and the Nemesis Shard are unleashed.
- **Code Hardening:** Its primary purpose is to expose weaknesses (panics, deadlocks, OOMs, logic flaws) in code and patches, thereby driving their regeneration and improvement. Code that survives Thunderdome is considered "battle-hardened."
- **Feedback Loop:** The outcomes from Thunderdome (whether code "survived" or was "defeated") are crucial feedback for the ToolGenerator and the overall patch review process.

**Integration:**
- **Panic Maker:** Attacks are run within the Thunderdome context. Results (`THUNDERDOME RESULT: SURVIVED` or `THUNDERDOME RESULT: DEFEATED`) directly inform the tool generation and regeneration process.
- **Nemesis Shard:** Orchestrates its comprehensive adversarial analysis ("The Gauntlet") within Thunderdome, deploying and executing Go attack binaries against target patches.

**Key Aspects:**
- **Outcome-Oriented:** Definitively determining if code can withstand targeted attacks
- **Deterministic Feedback:** Clear, actionable results that the logical kernel can process
- **Foundational to Autopoiesis:** The continuous cycle of attack and defense is fundamental to codeNERD's self-learning capabilities

## Contextual Awareness

codeNERD exhibits a highly advanced and multi-layered approach to contextual awareness, a cornerstone of its neuro-symbolic architecture. It moves beyond simple chat history to a dynamic, logic-driven context management system.

**CompilationContext** (`internal/prompt/context.go`): Encapsulates up to 10 distinct contextual dimensions:
- Operational Mode (e.g., `/debugging`, `/dream`)
- Campaign Phase (for multi-phase goal management)
- Shard Type (e.g., `/coder`, `/reviewer`)
- Language & Framework (e.g., `/go`, `/bubbletea`)
- Intent (user's current verb and target)
- World States (failing tests, active diagnostics, security issues, new files, code churn)
- Token Budget (managed dynamically)

This CompilationContext is critical for JIT Prompt Compilation, ensuring that only the most relevant "prompt atoms" are selected for LLM injection.

**SessionContext** (`internal/types/types.go`, `internal/core/shard_manager.go`): Implements a "Blackboard Pattern" as shared working memory across shards and turns:
- Compressed History (semantically condensed past interactions)
- Current Diagnostics & Test State
- Active Files, Symbols, and Dependencies
- Git Context (branch, modified files, recent commits for "Chesterton's Fence")
- Campaign Context
- Prior Shard Outputs (cross-shard collaboration)
- Knowledge Atoms & Specialist Hints
- Allowed/Blocked Actions & Safety Warnings

**Memory Tiers:** See "Memory Tiers" table in Nomenclature section (4-tier system: RAM, Vector, Graph, Cold).

**Spreading Activation** (`internal/context/activation.go`, `internal/core/defaults/policy.mg`): Core mechanism for Logical Context Selection, replacing traditional vector-based RAG. Operates on the Mangle knowledge graph:
- **Context-Directed Spreading Activation (CDSA):** Dynamically adjusts activation flow based on logical rules
- **Activation Scores:** Facts are assigned scores based on recency, relevance, dependency, and campaign/issue context

**Context Paging & Compression** (`internal/campaign/context_pager.go`, `internal/context/compressor.go`):
- **Context Paging:** Manages the context window during long-running campaigns
- **Semantic Compression:** Achieves "Infinite Context" by transforming verbose history into concise Mangle facts

**context.Context** (Go Standard Library): Ubiquitous throughout the codebase for cancellation, timeouts, and request-scoped values.

**Context7** (`internal/shards/researcher/tools.go`): Integrated research tool for fetching curated, LLM-optimized documentation for libraries and frameworks.

## Context Test Harness

A comprehensive testing framework for validating codeNERD's infinite context system.

**Location:** `internal/testing/context_harness/`

**Architecture:**
- `harness.go` - Main orchestrator
- `simulator.go` - Session simulator with checkpoint validation
- `scenarios.go` - Pre-built test scenarios (4 scenarios, 50-100 turns each)
- `metrics.go` - Metrics collection (compression, retrieval, performance)
- `reporter.go` - Results reporting (console, JSON)

**Observability Components:**
- `activation_tracer.go` - Traces spreading activation through fact graph
- `jit_tracer.go` - JIT prompt compilation tracing
- `compression_viz.go` - Semantic compression visualization
- `inspector.go` - Deep inspection tools
- `integration.go` - Real codeNERD integration

**Pre-Built Scenarios:**
| Scenario | Turns | Tests |
|----------|-------|-------|
| Debugging Marathon | 50 | Long-term context retention, solution tracking |
| Feature Implementation | 75 | Multi-phase context paging (plan → implement → test) |
| Refactoring Campaign | 100 | Cross-file tracking, long-term stability |
| Research + Build | 80 | Cross-phase knowledge retrieval |

**Metrics:**
- **Compression Ratio**: Target >5:1 (short), >8:1 (long sessions)
- **Retrieval Precision/Recall/F1**: Spreading activation accuracy
- **Performance**: Latency, peak memory
- **Degradation**: Quality stability over 100+ turn sessions

**Usage:**
```bash
nerd test-context --scenario debugging-marathon
nerd test-context --all --format json > results.json
```

## Tactile: Motor Cortex

The tactile package is the motor cortex of the neuro-symbolic architecture, providing the lowest-level execution layer for physical world interaction.

**Location:** `internal/tactile/`

**Architecture:**
- **DirectExecutor**: Host execution via os/exec (no sandboxing)
- **DockerExecutor**: Ephemeral container isolation
- **PersistentDockerExecutor**: Stateful containers for SWE-bench workflows
- **NamespaceExecutor**: Linux namespace isolation (PID/Net/Mount)
- **CompositeExecutor**: Routes by sandbox mode

**Sandbox Modes:**
| Mode | Implementation | Use Case |
|------|---------------|----------|
| `none` | DirectExecutor | Trusted operations |
| `docker` | DockerExecutor | Isolated execution |
| `namespace` | NamespaceExecutor | Linux-only isolation |
| `firejail` | FirejailExecutor | Lightweight sandboxing |

**Audit Trail:** All executors emit Mangle facts:
```datalog
execution_started("session-123", "req-456", "go", 1703001234).
execution_completed("req-456", /success, 0, 2345).
file_written("/path/to/file.go", "abc123", "session-123", 1703001235).
```

**SWE-bench Integration:**
- `python/` - Python environment management
- `swebench/` - SWE-bench task orchestration with persistent containers

## Transparency & UX Layers

### Transparency (`internal/transparency/`)

Operation visibility layer making internal operations visible to users on demand.

**Components:**
- **ShardObserver**: Real-time shard execution phase tracking
- **SafetyReporter**: Explains constitutional gate blocks with remediation
- **Explainer**: Human-readable explanations from Mangle derivation traces
- **ErrorClassifier**: Categorizes errors (9 types) with remediation suggestions

**Design Principles:** Opt-in, non-intrusive, lazy (expensive ops only when requested), informative.

### UX (`internal/ux/`)

User experience management with progressive disclosure.

**User Journey States:**
| State | Trigger | Guidance Level |
|-------|---------|----------------|
| New | First run | Full onboarding |
| Onboarding | In wizard | Step-by-step |
| Learning | First 10-20 sessions | Contextual hints |
| Productive | 15+ sessions, <15% clarification | Minimal |
| Power | 50+ sessions, <5% clarification | None |

**Features:**
- Existing users skip onboarding (migrate to "productive")
- Commands revealed progressively as experience grows
- Metrics tracking (sessions, commands, clarifications)

## Modularization Patterns

Large files have been modularized for maintainability. The parent file serves as a package marker pointing to component files.

### Kernel Modularization (`internal/core/kernel.go` → 8 files)
| File | Purpose |
|------|---------|
| `kernel_types.go` | Core type definitions (RealKernel, Fact) |
| `kernel_init.go` | Constructor, Mangle engine boot |
| `kernel_facts.go` | LoadFacts, Assert, Retract |
| `kernel_query.go` | Query execution, pattern matching |
| `kernel_eval.go` | Policy evaluation, rule execution |
| `kernel_validation.go` | Schema validation, safety checks |
| `kernel_policy.go` | Policy/schema loading |
| `kernel_virtual.go` | Virtual predicate handling |

### LocalStore Modularization (`internal/store/local.go` → 10 files)
| File | Purpose |
|------|---------|
| `local_core.go` | Core struct, SQLite initialization |
| `local_vector.go` | Vector store (Shard B) |
| `local_graph.go` | Knowledge graph (Shard C) |
| `local_cold.go` | Cold storage/archival (Shard D) |
| `local_session.go` | Session management |
| `local_world.go` | World model cache |
| `local_knowledge.go` | Knowledge atoms |
| `local_prompt.go` | Prompt atoms for JIT |
| `local_review.go` | Review findings |
| `local_verification.go` | Verification records |

## Documentation

The codebase includes **46 CLAUDE.md files** providing AI-readable documentation with standardized File Index tables.

**Coverage:**
- Root `CLAUDE.md` - Architecture overview
- `cmd/nerd/`, `cmd/nerd/chat/`, `cmd/nerd/ui/` - CLI documentation
- All 30 `internal/` packages
- Build tools in `cmd/tools/`
- `.nerd/` workspace directory

**File Index Format:**
```markdown
| File | Description |
|------|-------------|
| `kernel.go` | Package marker documenting kernel modularization... |
| `kernel_init.go` | Constructor `NewRealKernel()` that boots... |
```

Each description includes: purpose, exported types/functions, key behaviors.

## Dynamic Adaptation

Dynamic Adaptation is a foundational feature implemented through codeNERD's Autopoiesis (self-creation) system. This goes beyond simple learning to enable the agent to self-modify and evolve its capabilities based on experience.

**Autopoiesis Orchestrator** (`internal/autopoiesis/autopoiesis.go`): Coordinates all self-modification capabilities—monitoring performance, detecting needs for new tools or rule adjustments, and managing the learning lifecycle.

**Self-Learning from Experience:**
- **LearningStore** (`internal/store/learning.go`): Records successful and failed patterns across sessions per shard type
- **Feedback & Learning System** (`internal/autopoiesis/feedback.go`): Closes the autopoiesis loop by evaluating tool quality and recording patterns
- **Rejection/Acceptance Tracking** (`internal/shards/coder/autopoiesis.go`, `internal/shards/tester/autopoiesis.go`): Recurring patterns (e.g., 3 rejections) trigger learning
- **Decay Confidence:** Old learnings reduce in confidence if not reinforced

**Ouroboros Loop** (`internal/autopoiesis/ouroboros.go`): Self-correction and tool-generation engine:
1. **Specification:** Defining the new tool's purpose and interface
2. **Generation:** Producing the tool's code and tests
3. **Safety Check:** Ensuring constitutional safety standards
4. **Thunderdome:** Adversarial testing
5. **Simulation:** Testing in Dream Mode
6. **Compilation & Registration:** Making the new tool available

**Dream State Learning** (`internal/core/dream_learning.go`): Multi-agent simulation mode where the agent explores hypothetical scenarios without affecting live state.

**Dynamic Policy Adjustment:**
- **FeedbackLoop** (`internal/mangle/feedback/loop.go`): Proposes and validates new policy rules
- **Legislator Shard**: Compiles and incorporates new Mangle rules at runtime (see Shard Architecture)

**Adaptive Workflows:**
- **Campaign Replanning** (`internal/campaign/replan.go`): Dynamically adapts plans in response to failures or new requirements
- **Adaptive Batch Sizing** (`internal/shards/researcher/researcher.go`): Adjusts research batch sizes based on complexity

## JIT (Just-In-Time) Prompt Compiler

The JIT Prompt Compiler is a core component replacing static system prompts with dynamically assembled ones. It represents a paradigm shift from fixed instructions to fluid, context-aware prompt engineering.

### Atom-Based Architecture

System prompts are broken down into thousands of atomic units called **Prompt Atoms** (stored in `internal/prompt/atoms/` as YAML files). Each atom has metadata: id, category (e.g., identity, capability, context), content, and Contextual Selectors (rules for when to include it).

### Compilation Process (`internal/prompt/compiler.go`)

When a shard needs to interact with an LLM, the JIT compiler executes:
1. **Context Gathering:** Collects the current CompilationContext
2. **Skeleton Selection:** Uses Mangle logic to select mandatory atoms defining core identity
3. **Flesh Selection:** Uses vector search to select optional, relevant atoms
4. **Budgeting:** Fits selected atoms into available token budget
5. **Assembly:** Concatenates atoms into a coherent system prompt

### Key Benefits

- **Infinite Effective Prompt Length:** Draw from millions of tokens, send only relevant ~20k
- **Contextual Specialization:** Python/Django debugging gets different prompt than Go/Mangle planning
- **Dynamic Evolution:** New atoms available immediately at runtime

### Integration

- **Shard Integration:** Shards use PromptAssembler (`AssembleSystemPrompt`)
- **Autopoiesis:** Injects learned patterns and tool usage instructions dynamically
- **Observability:** `/jit` command inspects last compiled prompt and statistics

## Full Specifications

For detailed architecture and implementation specs, see:

- [.claude/skills/codenerd-builder/references/](.claude/skills/codenerd-builder/references/) - Full architecture docs
- [.claude/skills/mangle-programming/references/](.claude/skills/mangle-programming/references/) - Mangle language reference


## Notice on unused wiring... investigate consuming unused methods and parameters and code before removing it... ultrathink on it even... this is a living codebase and we forget to wire things up all the time... 



## Key Implementation Files

| Component | Location | Purpose |
|-----------|----------|---------|
| Kernel | [internal/core/kernel.go](internal/core/kernel.go) | Mangle engine + fact management (modularized) |
| Policy | [internal/mangle/policy.mg](internal/mangle/policy.mg) | IDB rules (20 sections) |
| Schemas | [internal/mangle/schemas.mg](internal/mangle/schemas.mg) | EDB declarations |
| VirtualStore | [internal/core/virtual_store.go](internal/core/virtual_store.go) | FFI to external systems |
| ShardManager | [internal/core/shard_manager.go](internal/core/shard_manager.go) | Shard lifecycle (see Shard Architecture) |
| Transducer | [internal/perception/transducer.go](internal/perception/transducer.go) | NL→Atoms |
| Emitter | [internal/articulation/emitter.go](internal/articulation/emitter.go) | Atoms→NL (Piggyback) |
| JIT Compiler | [internal/prompt/compiler.go](internal/prompt/compiler.go) | Runtime prompt assembly |
| LocalStore | [internal/store/local.go](internal/store/local.go) | 4-tier persistence (modularized into 10 files) |
| Nemesis | [internal/shards/nemesis/nemesis.go](internal/shards/nemesis/nemesis.go) | Adversarial patch analysis |
| Thunderdome | [internal/autopoiesis/thunderdome.go](internal/autopoiesis/thunderdome.go) | Attack vector arena |
| DataFlow | [internal/world/dataflow_multilang.go](internal/world/dataflow_multilang.go) | Multi-language taint analysis |
| Holographic | [internal/world/holographic.go](internal/world/holographic.go) | Impact-aware context builder |
| Hypotheses | [internal/shards/reviewer/hypotheses.go](internal/shards/reviewer/hypotheses.go) | Mangle→LLM verification |
| Context Harness | [internal/testing/context_harness/](internal/testing/context_harness/) | Infinite context validation |
| Tactile | [internal/tactile/](internal/tactile/) | Motor cortex, sandboxed execution |
| Transparency | [internal/transparency/transparency.go](internal/transparency/transparency.go) | Operation visibility layer |
| UX | [internal/ux/](internal/ux/) | User journey, progressive disclosure |
| Activation | [internal/context/activation.go](internal/context/activation.go) | Spreading activation engine |
| Compressor | [internal/context/compressor.go](internal/context/compressor.go) | Semantic compression |

## Development Guidelines

### Mangle Rules

- All predicates require `Decl` in schema.mg before use
- Variables are UPPERCASE, constants are `/lowercase`
- Negation requires all variables bound elsewhere (safety)
- End every statement with `.`

### Go Patterns

- Shards implement the `Shard` interface (see Shard Architecture section)
- Facts use `ToAtom()` to convert Go structs to Mangle AST
- Virtual predicates abstract external APIs into logic queries

### Testing

- Run `go test ./...` before committing
- Build with `go build -o nerd.exe ./cmd/nerd`
- Context system validation: `nerd test-context --all`
- 30+ `*_test.go` files distributed across internal packages

### Git

- Use conventional commits

### Model Configuration

- Config file: `.nerd/config.json`
- Gemini 3 Pro model ID: `gemini-3-pro-preview` (yes, Gemini 3 exists as of Dec 2024)

## Nomenclature

### Memory Tiers

| Tier | Storage | Lifespan | Content |
|------|---------|----------|---------|
| **RAM** | In-memory FactStore | Session | Working facts, active context |
| **Vector** | SQLite + embeddings | Persistent | Semantic search, similar content |
| **Graph** | knowledge_graph table | Persistent | Entity relationships |
| **Cold** | cold_storage table | Permanent | Learned preferences, patterns |

### Key Predicates

| Predicate | Purpose |
|-----------|---------|
| `user_intent/5` | Seed for all logic (Category, Verb, Target, Constraint) |
| `next_action/1` | Derived action to execute |
| `permitted/1` | Constitutional safety gate |
| `context_atom/1` | Facts selected for LLM injection |
| `shard_executed/4` | Cross-turn shard result tracking |
| `hypothesis/4` | Mangle-derived issue candidates for LLM verification |
| `data_flow_sink/4` | Taint tracking for security analysis |
| `context_priority/3` | Impact-weighted context selection |
| `modified_function/3` | Changed functions for impact analysis |

### Protocols

| Protocol | Description |
|----------|-------------|
| **Piggyback** | Dual-channel output: surface (user) + control (kernel) |
| **OODA Loop** | Observe → Orient → Decide → Act |
| **TDD Repair** | Test fails → Read log → Find cause → Patch → Retest |
| **Autopoiesis** | Self-learning from rejection patterns |
| **Ouroboros** | Self-generating missing tools |
| **Thunderdome** | Adversarial arena: tools battle attack vectors in sandboxes |
| **The Gauntlet** | Nemesis adversarial review pipeline |
| **JIT Prompt Compile** | Runtime prompt assembly with token budget and context selectors |

## Quick Reference

**OODA Loop:** Observe (Transducer) → Orient (Spreading Activation) → Decide (Mangle Engine) → Act (Virtual Store)

**Constitutional Safety:** Every action requires `permitted(Action)` to derive. Default deny.

**Fact Flow:** User Input → Transducer → `user_intent` fact → Kernel derives `next_action` → VirtualStore executes → Result facts → Articulation → Response

## Top 30 Mangle Errors

Common errors AI coding agents make when writing Mangle code, categorized by stack layer.

### I. Syntactic Hallucinations (The "Soufflé/SQL" Bias)

**Atom vs. String Confusion**
- *Error:* Using `"active"` when the schema requires `/active`
- *Correction:* Use `/atom` for enums/IDs. Mangle treats these as disjoint types; they will never unify.

**Soufflé Declarations**
- *Error:* `.decl edge(x:number, y:number).`
- *Correction:* `Decl edge(X.Type<int>, Y.Type<int>).` (Note uppercase Decl and type syntax)

**Lowercase Variables**
- *Error:* `ancestor(x, y) :- parent(x, y).` (Prolog style)
- *Correction:* `ancestor(X, Y) :- parent(X, Y).` Variables must be UPPERCASE.

**Inline Aggregation (SQL Style)**
- *Error:* `total(Sum) :- item(X), Sum = sum(X).`
- *Correction:* Use the pipe operator: `... |> do fn:group_by(), let Sum = fn:Sum(X).`

**Implicit Grouping**
- *Error:* Assuming variables in the head automatically trigger GROUP BY (like SQL)
- *Correction:* Grouping is explicit in the `do fn:group_by(...)` transform step.

**Missing Periods**
- *Error:* Ending a rule with a newline instead of `.`
- *Correction:* Every clause must end with a period `.`

**Comment Syntax**
- *Error:* `// This is a comment` or `/* ... */`
- *Correction:* Use `# This is a comment`

**Assignment vs. Unification**
- *Error:* `X := 5` or `let X = 5` inside a rule body (without pipe)
- *Correction:* Use unification `X = 5` inside the body, or `let` only within a transform block.

### II. Semantic Safety & Logic (The "Datalog" Gap)

Mangle requires strict logical validity that probabilistic models often miss.

**Unsafe Head Variables**
- *Error:* `result(X) :- other(Y).` (X is unbounded)
- *Correction:* Every variable in the head must appear in a positive atom in the body.

**Unsafe Negation**
- *Error:* `safe(X) :- not distinct(X).`
- *Correction:* Variables in a negated atom must be bound first: `safe(X) :- candidate(X), not distinct(X).`

**Stratification Cycles**
- *Error:* `p(X) :- not q(X). q(X) :- not p(X).`
- *Correction:* Ensure no recursion passes through a negation. Restructure logic into strict layers (strata).

**Infinite Recursion (Counter Fallacy)**
- *Error:* `count(N) :- count(M), N = fn:plus(M, 1).` (Unbounded generation)
- *Correction:* Always bound recursion with a limit or a finite domain (e.g., `N < 100`).

**Cartesian Product Explosion**
- *Error:* Placing large tables before filters: `res(X) :- huge_table(X), X = /specific_id.`
- *Correction:* Selectivity first: `res(X) :- X = /specific_id, huge_table(X).`

**Null Checking (Open World Bias)**
- *Error:* `check(X) :- data(X), X != null.`
- *Correction:* Mangle follows the Closed World Assumption. If a fact exists, it is not null. "Missing" facts are simply not there.

**Duplicate Rule Definitions**
- *Error:* Thinking multiple rules overwrite each other
- *Correction:* Multiple rules create a UNION. `p(x) :- a(x).` and `p(x) :- b(x).` means p is true if a OR b is true.

**Anonymous Variable Misuse**
- *Error:* Using `_` when the value is actually needed later in the rule
- *Correction:* Use `_` only for values you truly don't care about. It never binds.

### III. Data Types & Functions (The "JSON" Bias)

AI agents often hallucinate object-oriented accessors for Mangle's structured data.

**Map Dot Notation**
- *Error:* `Val = Map.key` or `Map['key']`
- *Correction:* Use `:match_entry(Map, /key, Val)` or `:match_field(Struct, /key, Val)`.

**List Indexing**
- *Error:* `Head = List[0]`
- *Correction:* Use `:match_cons(List, Head, Tail)` or `fn:list:get(List, 0)`.

**Type Mismatch (Int vs Float)**
- *Error:* `X = 5` when X is declared `Type<float>`
- *Correction:* Mangle is strict. Use `5.0` for floats, `5` for ints.

**String Interpolation**
- *Error:* `msg("Error: $Code")`
- *Correction:* Use `fn:string_concat` or build list structures. Mangle has no string interpolation.

**Hallucinated Functions**
- *Error:* `fn:split`, `fn:date`, `fn:substring` (assuming StdLib parity with Python)
- *Correction:* Verify function existence in builtin package. Mangle's standard library is minimal.

**Aggregation Safety**
- *Error:* `... |> do fn:group_by(UnboundVar) ...`
- *Correction:* Grouping variables must be bound in the rule body before the pipe `|>`.

**Struct Syntax**
- *Error:* `{"key": "value"}` (JSON style)
- *Correction:* `{ /key: "value" }` (Note the atom key and spacing)

### IV. Go Integration & Architecture (The "API" Gap)

When embedding Mangle, AI agents fail to navigate the boundary between Go and Logic.

**Fact Store Type Errors**
- *Error:* `store.Add("pred", "arg")`
- *Correction:* Must use `engine.Atom`, `engine.Number` types wrapped in `engine.Value`.

**Incorrect Engine Entry Point**
- *Error:* `engine.Run()` (Hallucination)
- *Correction:* Use `engine.EvalProgram` or `engine.EvalProgramNaive`.

**Ignoring Imports**
- *Error:* Generating Mangle code without necessary package references or failing to import the Go engine package correctly
- *Correction:* Explicitly manage `github.com/google/mangle/engine`.

**External Predicate Signature**
- *Error:* Writing a Go function for a predicate that returns `(interface{}, error)`
- *Correction:* External predicates require `func(query engine.Query, cb func(engine.Fact)) error`.

**Parsing vs. Execution**
- *Error:* Passing raw strings to EvalProgram
- *Correction:* Code must be parsed (`parse.Unit`) and analyzed (`analysis.AnalyzeOneUnit`) before evaluation.

**Assuming IO Access**
- *Error:* `read_file(Path, Content).`
- *Correction:* Mangle is pure. IO must happen in Go before execution (loading facts) or via external predicates.

**Package Hallucination (Slopsquatting)**
- *Error:* Importing non-existent Mangle libraries (e.g., `use /std/date`)
- *Correction:* Verify imports. Mangle has a very small, specific ecosystem.

### How to Avoid These Mistakes

1. **Feed the Grammar:** Provide the "Complete Syntax Reference" in the prompt context
2. **Solver-in-the-Loop:** Don't trust "Zero-Shot" code. Run a loop: Generate → Parse (with `mangle/parse`) → Feed Errors back to LLM → Regenerate
3. **Explicit Typing:** Force the AI to declare types (`Decl`) first. This forces it to decide between `/atoms` and `"strings"` early
4. **Review for Liveness:** Manually audit recursive rules for termination conditions
