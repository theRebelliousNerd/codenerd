# codeNERD

A high-assurance Logic-First CLI coding agent built on the Neuro-Symbolic architecture.

## PUSH TO GITHUB ALL THE TIME

## FOR ALL NEW LLM SYSTEMS, JIT IS THE STANDARD, ALWAYS CREATE NEW PROMPT ATOMS AND USE THE JIT SYSTEM. IT IS THE CENTRAL PARADIGM OF THE SYSTEM. JIT, PIGGYBACKING, CONTROL PACKETS, MANGLE, THAT IS THE NAME OF THE GAME!

> ## ⚠️ MAJOR ARCHITECTURE UPDATE (Dec 2024): JIT Clean Loop
>
> **Domain shards have been DELETED** (~35,000 lines removed). The following directories no longer exist:
>
> - `internal/shards/coder/`, `internal/shards/tester/`, `internal/shards/reviewer/`
> - `internal/shards/researcher/`, `internal/shards/nemesis/`, `internal/shards/tool_generator/`
>
> **Replaced by JIT Clean Loop:**
>
> | New File | Purpose |
> |----------|---------|
> | `internal/session/executor.go` | The Clean Execution Loop (~50 lines) |
> | `internal/session/spawner.go` | JIT-driven SubAgent spawning |
> | `internal/session/subagent.go` | Context-isolated SubAgent implementation |
> | `internal/prompt/config_factory.go` | Intent → tools/policies mapping |
> | `internal/prompt/atoms/identity/*.yaml` | Persona atoms (coder, tester, reviewer, researcher) |
> | `internal/mangle/intent_routing.mg` | Mangle routing rules for persona selection |
>
> **Philosophy:** The LLM doesn't need 5000+ lines of Go code telling it how to be a coder/tester/reviewer. It needs JIT-compiled prompts with persona atoms, tool access via VirtualStore, and safety via Constitutional Gate.

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
├── nerd/               # CLI entrypoint (80+ Go files)
│   ├── chat/           # TUI chat interface (Elm architecture, modularized)
│   └── ui/             # UI components
├── query-kb/           # Knowledge base query tool
├── test-research/      # Research testing tool
└── tools/              # Build tools (corpus_builder, mangle_check, etc.)

internal/               # 32 packages, ~105K LOC (after JIT refactor)
├── session/            # **NEW** Clean execution loop, Spawner, SubAgents
├── tools/              # **NEW** Modular tool registry (core/, shell/, codedom/, research/)
├── core/               # Kernel, VirtualStore (modularized)
├── perception/         # NL → Mangle atom transduction
├── articulation/       # Mangle atom → NL transduction
├── autopoiesis/        # Self-modification: Ouroboros, Thunderdome, tool learning, prompt evolution
├── mcp/                # MCP (Model Context Protocol) integration, JIT Tool Compiler
├── prompt/             # JIT Prompt Compiler, ConfigFactory, persona atoms, context-aware assembly
├── shards/             # **REDUCED** System shards only (system/), domain shards DELETED
├── mangle/             # .mg schema/policy files + intent_routing.mg + feedback/, transpiler/
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

**SubAgents:** See "SubAgent Architecture (JIT Clean Loop)" section below. Domain shards (coder/tester/reviewer/researcher) have been deleted and replaced by JIT-configured SubAgents.

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

## SubAgent Architecture (JIT Clean Loop)

> **Dec 2024:** Domain shards (CoderShard, TesterShard, etc.) have been **deleted**. SubAgents are now JIT-configured via persona atoms and `ConfigFactory`.

### The Clean Execution Loop

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

### SubAgent Types

| Type | Constant | Description | Memory |
|------|----------|-------------|--------|
| **Ephemeral** | `SubAgentTypeEphemeral` | Spawn → Execute → Die | RAM only |
| **Persistent** | `SubAgentTypePersistent` | User-defined specialists | SQLite-backed |
| **System** | `SubAgentTypeSystem` | Long-running services | RAM |

### Key Implementation Files

| File | Purpose |
|------|---------|
| `internal/session/executor.go` | The Clean Execution Loop |
| `internal/session/spawner.go` | JIT-driven SubAgent spawning |
| `internal/session/subagent.go` | SubAgent lifecycle management |
| `internal/session/task_executor.go` | TaskExecutor interface and JITExecutor |
| `internal/prompt/config_factory.go` | Intent → tools/policies mapping |
| `internal/prompt/atoms/identity/*.yaml` | Persona atoms (coder, tester, reviewer, researcher) |
| `internal/mangle/intent_routing.mg` | Mangle routing rules for persona selection |

### TaskExecutor (Unified Task Interface)

The `TaskExecutor` interface provides a unified API for task execution, abstracting both the JIT architecture and legacy ShardManager:

```go
// internal/session/task_executor.go
type TaskExecutor interface {
    Execute(ctx context.Context, intent string, task string) (string, error)
    ExecuteAsync(ctx context.Context, intent string, task string) (taskID string, err error)
    GetResult(taskID string) (result string, done bool, err error)
    WaitForResult(ctx context.Context, taskID string) (string, error)
}
```

**JITExecutor** implements this interface using the clean execution loop:

- Simple intents → `Executor.Process()` → Direct LLM call
- Complex intents → `Spawner.Spawn()` → Isolated SubAgent

**Intent Mapping** converts legacy shard names to intent verbs:

| Legacy Shard | Intent Verb |
|--------------|-------------|
| `coder` | `/fix` |
| `tester` | `/test` |
| `reviewer` | `/review` |
| `researcher` | `/research` |

### Persona Atoms (Replace Hardcoded Shard Prompts)

All persona/identity now comes from YAML atoms in `internal/prompt/atoms/identity/`:

```yaml
# internal/prompt/atoms/identity/coder.yaml
- id: "identity/coder/mission"
  category: "identity"
  priority: 100
  is_mandatory: true
  intent_verbs: ["/fix", "/implement", "/refactor", "/create"]
  content: |
    You are the Coder Shard of codeNERD, the execution arm for code generation.
    ## Core Responsibilities
    1. Generate new code following project patterns
    2. Modify existing code to fix bugs or add features
    ...
```

### ConfigFactory (Intent → Tools Mapping)

```go
// internal/prompt/config_factory.go
// Maps intent verbs to allowed tools
provider.atoms["/fix"] = ConfigAtom{
    Tools: []string{"read_file", "write_file", "edit_file", "run_build", "git_operation"},
    Priority: 100,
}
```

### System Shards (Still Active)

Long-running background services that maintain system state:

| Shard | Purpose |
|-------|---------|
| `world_model_ingestor` | file_topology, symbol_graph maintenance |
| `constitution_gate` | Safety enforcement |
| `session_planner` | Agenda/campaign orchestration |

## Quiescent Boot & Session Management

> **Dec 2024:** Implemented clean session architecture with proper ephemeral fact filtering.

### The Problem: Stale Facts Triggering Actions at Boot

Old `user_intent` facts persisting from previous sessions would trigger action derivation at boot via:
```mangle
next_action(/execute_intent) :- user_intent(_, _, _, _, _), not tdd_state(_).
```

### Solution: Ephemeral Fact Filtering

**Location:** `internal/core/fact_categories.go`, `internal/core/kernel_init.go`

Facts are categorized as ephemeral (session-scoped) or persistent:

```go
// Ephemeral predicates - filtered at boot
var ephemeralPredicates = map[string]bool{
    "user_intent":      true,  // Current turn's intent
    "pending_action":   true,  // Actions awaiting execution
    "next_action":      true,  // Derived next action
    "active_tool":      true,  // Currently executing tool
    "turn_context":     true,  // Per-turn context
}
```

At kernel boot, `filterBootFacts()` removes ephemeral facts before loading:
```go
// kernel_init.go
if len(k.bootFacts) > 0 {
    k.facts = append(k.facts, filterBootFacts(k.bootFacts)...)
}
```

### Clean Session Architecture

**Philosophy:** Boot always starts fresh. Previous sessions are explicitly resumed.

| Command | Purpose |
|---------|---------|
| `/sessions` | List and interactively select previous sessions |
| `/load-session <id>` | Load a specific session by ID |
| `/new-session` | Start a fresh session (preserves old in history) |

**Startup Behavior:**
1. Generate fresh session ID
2. Filter ephemeral facts from kernel boot
3. Display hint: "Use `/sessions` to load previous sessions (N available)"

**Location:** `cmd/nerd/chat/session.go`, `cmd/nerd/chat/commands.go`

### Defense-in-Depth: Boot Guard

As additional safety, a boot guard blocks `RouteAction()` until first user interaction:

```go
// virtual_store.go
if v.bootGuardActive {
    return "", fmt.Errorf("boot guard active: action routing blocked")
}
```

The guard is released when the first real user message arrives (not rehydrated history).

## Modular Tool Registry

> **Dec 2024:** Tools are now modular and any agent can use any tool via JIT selection.

### The Problem: Tools Embedded in Shards

Old architecture embedded tools inside domain shards. When shards were deleted, tools were lost. The JIT architecture separates:

| Component | Purpose |
|-----------|---------|
| **Personas** (prompt atoms) | Define *who* the agent is |
| **Tools** (registry) | Define *what* the agent can do |
| **Policies** (Mangle rules) | Define *when* tools are allowed |
| **Intent routing** (ConfigFactory) | Define *which* tools for *which* intent |

### Tool Registry Architecture

**Location:** `internal/tools/`

```
internal/tools/
├── registry.go       # Central tool registry
├── types.go          # Tool, ToolSchema, ToolCategory types
├── core/             # Filesystem tools (read_file, write_file, glob, grep)
├── shell/            # Execution tools (run_command, bash, run_build)
├── codedom/          # Semantic code tools (get_elements, edit_lines)
└── research/         # Research tools (web_search, web_fetch, context7_fetch)
```

### Tool Categories

| Category | Tools | Available To |
|----------|-------|--------------|
| `/core` | read_file, write_file, glob, grep, list_files | All personas |
| `/shell` | run_command, bash, run_build, run_tests | Coder, Tester |
| `/codedom` | get_elements, get_element, edit_lines | All personas |
| `/research` | context7_fetch, web_search, web_fetch | Researcher |

### Mangle Tool Routing

Tools are routed via intent in `internal/mangle/intent_routing.mg`:

```mangle
# Core tools available to all intents
modular_tool_allowed(/read_file, Intent) :- user_intent(_, _, Intent, _, _).

# Write tools - available for code intents
modular_tool_allowed(/write_file, Intent) :- intent_category(Intent, /code).

# Research tools - available for /research intent
modular_tool_allowed(/web_search, Intent) :- intent_category(Intent, /research).
```

### Tool Hydration at Boot

`VirtualStore.HydrateModularTools()` registers all tools at boot:

```go
// virtual_store.go
func (v *VirtualStore) HydrateModularTools() error {
    core.RegisterAll(v.toolRegistry)
    shell.RegisterAll(v.toolRegistry)
    codedom.RegisterAll(v.toolRegistry)
    research.RegisterAll(v.toolRegistry)
}
```

### VirtualStore Tool Routing

`handleModularTool()` routes tool calls through the registry:

```go
case ActionListFiles, ActionGlob, ActionGrep:
    return v.handleModularTool(ctx, req)
case ActionRunCommand, ActionBash, ActionRunBuild:
    return v.handleModularTool(ctx, req)
```

## Adversarial Engineering: Thunderdome & Panic Maker

codeNERD employs an Adversarial Co-Evolution strategy. Instead of relying solely on passive testing, it actively attempts to break its own code using two distinct but related components: the Panic Maker (tactical tool breaker) and the Nemesis Shard (strategic system breaker).

### Panic Maker (The Tactical Breaker)

- *Implementation:* `internal/autopoiesis/panic_maker.go`
- *Scope:* Micro-level. Focused on breaking individual tools and functions during the generation phase (Ouroboros loop).

### Nemesis Shard (The Strategic Adversary)

- *Implementation:* `internal/shards/nemesis/nemesis.go`
- *Scope:* System-level. A persistent "Type B" Specialist Shard that acts as a gatekeeper for code changes.
- *Philosophy:* "The Nemesis does not seek destruction - it seeks truth." It acts as a hostile sparring partner for the Coder Shard.

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

**SessionContext** (`internal/types/types.go`): Implements a "Blackboard Pattern" as shared working memory across SubAgents and turns:
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


## Documentation

The codebase includes **51 CLAUDE.md files** providing AI-readable documentation with standardized File Index tables.

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
- **Legislator** (`internal/shards/system/legislator.go`): System shard that compiles new Mangle rules at runtime

**Adaptive Workflows:**
- **Campaign Replanning** (`internal/campaign/replan.go`): Dynamically adapts plans in response to failures or new requirements
- **Adaptive Execution**: SubAgents adjust behavior based on task complexity via JIT-compiled ConfigAtoms

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

## MCP (Model Context Protocol) Integration

The MCP package provides a JIT Tool Compiler for intelligent MCP tool serving based on task context.

**Location:** `internal/mcp/`

### Architecture

```
MCPClientManager → ToolAnalyzer → MCPToolStore → Mangle Facts
                                      ↓
TaskContext → Vector Search + Mangle Logic → Tool Selection
                                      ↓
ToolRenderer → Full/Condensed/Minimal → LLM Context
```

### Components

| File | Purpose |
|------|---------|
| `types.go` | Core type definitions (MCPServer, MCPTool, etc.) |
| `client.go` | Server connections, protocol negotiation |
| `transport_http.go` | HTTP-based MCP communication |
| `store.go` | SQLite storage with embeddings |
| `analyzer.go` | LLM-based metadata extraction |
| `compiler.go` | JIT tool selection pipeline |
| `renderer.go` | Tool set rendering for LLM |

### Skeleton/Flesh Bifurcation

Mirrors the JIT Prompt Compiler pattern:
- **Skeleton Tools**: Always available (filesystem read, shell exec) — selected via Mangle logic
- **Flesh Tools**: Context-dependent — hybrid scoring: (Logic × 0.7) + (Vector × 0.3)

### Three-Tier Rendering

| Tier | Threshold | Content |
|------|-----------|---------|
| Full | ≥70 | Complete JSON schema, description, examples |
| Condensed | 40-69 | Name + one-line description |
| Minimal | 20-39 | Name only (available on request) |
| Excluded | <20 | Not sent to LLM |

### Mangle Integration

```mangle
mcp_server_registered(ServerID, Endpoint, Protocol, RegisteredAt).
mcp_tool_capability(ToolID, Capability).
mcp_tool_shard_affinity(ToolID, ShardType, Score).
mcp_tool_selected(ShardType, ToolID, RenderMode).
```

## Prompt Evolution System (System Prompt Learning)

The Prompt Evolution System implements Karpathy's "third paradigm" of LLM learning — automatic evolution of prompt atoms based on execution feedback.

**Location:** `internal/autopoiesis/prompt_evolution/`

### Architecture

```
Execute → Evaluate (LLM-as-Judge) → Evolve (Meta-Prompt) → Integrate (JIT Compiler)
```

### Components

| File | Purpose |
|------|---------|
| `types.go` | Core type definitions (ExecutionRecord, JudgeVerdict, Strategy) |
| `judge.go` | LLM-as-Judge evaluation with explanations |
| `feedback_collector.go` | Execution outcome recording and storage |
| `strategy_store.go` | Problem-type-specific strategy database |
| `classifier.go` | Problem type classification |
| `atom_generator.go` | Automatic atom creation from failures |
| `evolver.go` | Main evolution orchestrator |

### Error Categories

| Category | Description |
|----------|-------------|
| `LOGIC_ERROR` | Wrong approach or algorithm |
| `SYNTAX_ERROR` | Code syntax issues |
| `API_MISUSE` | Wrong API or library usage |
| `EDGE_CASE` | Missing edge case handling |
| `CONTEXT_MISS` | Missed relevant codebase context |
| `INSTRUCTION_MISS` | Didn't follow instructions |
| `HALLUCINATION` | Made up information |
| `CORRECT` | Task completed correctly |

### Storage

```
.nerd/prompts/
├── evolution.db        # Execution records, verdicts
├── strategies.db       # Strategy database
└── evolved/
    ├── pending/        # Awaiting promotion
    ├── promoted/       # Promoted to corpus
    └── rejected/       # User rejected
```

## LLM Provider System

The perception package implements a multi-provider LLM client factory supporting 7 providers with unified interface.

**Location:** `internal/perception/client*.go`

### Supported Providers

| Provider | Default Model | Config Key | Notes |
|----------|---------------|------------|-------|
| Z.AI | `glm-4.7` | `zai_api_key` | 200K context, 128K output |
| Anthropic | `claude-sonnet-4` | `anthropic_api_key` | Claude 4 series |
| OpenAI | `gpt-5.1-codex-max` | `openai_api_key` | Codex models supported |
| Gemini | `gemini-3-pro-preview` | `gemini_api_key` | Gemini 3 Flash/Pro |
| xAI | `grok-3-beta` | `xai_api_key` | Grok series |
| OpenRouter | (various) | `openrouter_api_key` | Multi-model routing |
| CLI Engines | - | `engine: claude-cli` | Claude Code / Codex CLI subprocess |

### Provider Detection

```go
// Auto-detect provider from config/environment
client, err := perception.NewClientFromEnv()

// Each provider implements LLMClient interface
type LLMClient interface {
    Complete(ctx context.Context, prompt string) (string, error)
    CompleteWithSystem(ctx context.Context, system, user string) (string, error)
}
```

## LLM Timeout Consolidation

Centralized timeout configuration for all LLM operations, preventing timeout conflicts.

**Location:** `internal/config/llm_timeouts.go`

### Key Insight

In Go, the SHORTEST timeout in the chain wins. This configuration provides canonical timeouts that all LLM operations use.

### Timeout Tiers

| Tier | Purpose | Default |
|------|---------|---------|
| **Tier 1: Per-Call** | HTTP/API timeouts | 10 minutes |
| **Tier 2: Operation** | Multi-step operations | 5-20 minutes |
| **Tier 3: Campaign** | Long-running orchestration | 30 minutes |

### Key Timeouts

| Timeout | Default | Purpose |
|---------|---------|---------|
| `HTTPClientTimeout` | 10 min | Maximum HTTP operation time |
| `PerCallTimeout` | 10 min | Single LLM call context |
| `ShardExecutionTimeout` | 20 min | Shard spawn + research + LLM |
| `ArticulationTimeout` | 5 min | Transducer LLM calls |
| `CampaignPhaseTimeout` | 30 min | Full campaign phase |

### Usage

```go
timeouts := config.GetLLMTimeouts()
ctx, cancel := context.WithTimeout(ctx, timeouts.ShardExecutionTimeout)
```

## Glass-Box Tool Visibility

Real-time visibility into tool execution for transparency and debugging.

**Location:** `cmd/nerd/chat/glass_box.go`

### Features

- **Tool Execution Display**: Shows which tools are being invoked
- **Parameter Visibility**: Displays tool parameters in real-time
- **Result Streaming**: Shows tool output as it arrives
- **Error Context**: Provides detailed error information with remediation

### Integration

Wired into the TUI chat interface for real-time observation of tool invocations during shard execution.

## Knowledge Discovery System

LLM-First Knowledge Discovery enables shards to access specialized knowledge dynamically.

**Location:** `internal/articulation/prompt_assembler.go` (Semantic Knowledge Bridge)

### Semantic Knowledge Bridge

Bridges JIT Prompt Compiler and shard knowledge for context-aware knowledge injection:
1. **Query Time**: When JIT compiles a prompt, it queries the bridge for relevant knowledge
2. **Shard Context**: Knowledge atoms are filtered by shard type and current task
3. **Vector Search**: Semantic similarity finds relevant knowledge from specialist databases

### Document Ingestion

The `/refresh-docs` command enables runtime document ingestion:
- Scans markdown files for strategic knowledge
- Uses LLM filtering for relevance
- Persists knowledge atoms with vector embeddings

## Full Specifications

For detailed architecture and implementation specs, see:

- [.claude/skills/codenerd-builder/references/](.claude/skills/codenerd-builder/references/) - Full architecture docs
- [.claude/skills/mangle-programming/references/](.claude/skills/mangle-programming/references/) - Mangle language reference


## Notice on unused wiring... investigate consuming unused methods and parameters and code before removing it... ultrathink on it even... this is a living codebase and we forget to wire things up all the time...



## Key Implementation Files

| Component | Location | Purpose |
|-----------|----------|---------|
| **Session Executor** | [internal/session/executor.go](internal/session/executor.go) | The Clean Execution Loop |
| **Spawner** | [internal/session/spawner.go](internal/session/spawner.go) | JIT-driven SubAgent spawning |
| **SubAgent** | [internal/session/subagent.go](internal/session/subagent.go) | Context-isolated SubAgents |
| **TaskExecutor** | [internal/session/task_executor.go](internal/session/task_executor.go) | Unified task execution interface |
| **ConfigFactory** | [internal/prompt/config_factory.go](internal/prompt/config_factory.go) | Intent → tools/policies mapping |
| **Persona Atoms** | [internal/prompt/atoms/identity/](internal/prompt/atoms/identity/) | Persona atoms (coder, tester, etc.) |
| **Intent Routing** | [internal/mangle/intent_routing.mg](internal/mangle/intent_routing.mg) | Mangle routing rules |
| **Tool Registry** | [internal/tools/registry.go](internal/tools/registry.go) | **NEW** Modular tool registry |
| **Core Tools** | [internal/tools/core/](internal/tools/core/) | **NEW** Filesystem tools (read, write, glob) |
| **Shell Tools** | [internal/tools/shell/](internal/tools/shell/) | **NEW** Execution tools (bash, run_command) |
| **CodeDOM Tools** | [internal/tools/codedom/](internal/tools/codedom/) | **NEW** Semantic code tools |
| **Research Tools** | [internal/tools/research/](internal/tools/research/) | **NEW** Research tools (web_search, context7) |
| **Fact Categories** | [internal/core/fact_categories.go](internal/core/fact_categories.go) | **NEW** Ephemeral vs persistent facts |
| Kernel | [internal/core/kernel.go](internal/core/kernel.go) | Mangle engine + fact management (modularized) |
| Policy | [internal/mangle/policy.mg](internal/mangle/policy.mg) | IDB rules (20 sections) |
| Schemas | [internal/mangle/schemas.mg](internal/mangle/schemas.mg) | EDB declarations |
| VirtualStore | [internal/core/virtual_store.go](internal/core/virtual_store.go) | FFI to external systems |
| Transducer | [internal/perception/transducer.go](internal/perception/transducer.go) | NL→Atoms |
| Emitter | [internal/articulation/emitter.go](internal/articulation/emitter.go) | Atoms→NL (Piggyback) |
| JIT Compiler | [internal/prompt/compiler.go](internal/prompt/compiler.go) | Runtime prompt assembly |
| LocalStore | [internal/store/local.go](internal/store/local.go) | 4-tier persistence (modularized into 10 files) |
| Thunderdome | [internal/autopoiesis/thunderdome.go](internal/autopoiesis/thunderdome.go) | Attack vector arena |
| DataFlow | [internal/world/dataflow_multilang.go](internal/world/dataflow_multilang.go) | Multi-language taint analysis |
| Holographic | [internal/world/holographic.go](internal/world/holographic.go) | Impact-aware context builder |
| Context Harness | [internal/testing/context_harness/](internal/testing/context_harness/) | Infinite context validation |
| Tactile | [internal/tactile/](internal/tactile/) | Motor cortex, sandboxed execution |
| Transparency | [internal/transparency/transparency.go](internal/transparency/transparency.go) | Operation visibility layer |
| UX | [internal/ux/](internal/ux/) | User journey, progressive disclosure |
| Activation | [internal/context/activation.go](internal/context/activation.go) | Spreading activation engine |
| Compressor | [internal/context/compressor.go](internal/context/compressor.go) | Semantic compression |
| MCP Compiler | [internal/mcp/compiler.go](internal/mcp/compiler.go) | JIT Tool Compiler |
| MCP Store | [internal/mcp/store.go](internal/mcp/store.go) | Tool storage with embeddings |
| Prompt Evolution | [internal/autopoiesis/prompt_evolution/evolver.go](internal/autopoiesis/prompt_evolution/evolver.go) | System Prompt Learning |
| LLM Timeouts | [internal/config/llm_timeouts.go](internal/config/llm_timeouts.go) | Centralized timeout config |
| Glass-Box | [cmd/nerd/chat/glass_box.go](cmd/nerd/chat/glass_box.go) | Tool execution visibility |

## Development Guidelines

### Mangle Rules

- All predicates require `Decl` in schema.mg before use
- Variables are UPPERCASE, constants are `/lowercase`
- Negation requires all variables bound elsewhere (safety)
- End every statement with `.`

### Go Patterns

- SubAgents use `session.Executor` and `session.Spawner` for JIT-driven execution
- System shards implement `types.ShardAgent` interface (perception_firewall, legislator, etc.)
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

Common errors when writing Mangle code, categorized by stack layer.

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

---

**Remember: Push to GitHub regularly!**

## Repository Hygiene

- **2026-01-27**: Moved `perception_output.txt` to `Docs/archive-candidates/` to maintain root directory cleanliness.
- **2026-01-27**: Established `Docs/maintenance/TODO.md` for tracking staleness and broken links.

> *[Archived & Reviewed by The Librarian on 2026-01-27]*
