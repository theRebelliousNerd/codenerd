# codeNERD

A high-assurance Logic-First CLI coding agent built on the Neuro-Symbolic architecture.

## PUSH TO GITHUB ALL THE TIME 

## FOR ALL NEW LLM SYSTEMS, JIT IS THE STANDARD, ALWAYS CREATE NEW PROMPT ATOMS AND USE THE JIT SYSTEM. IT IS THE CENTRAL PARADIGM OF THE SYSTEM. JIT, PIGGYBACKING, CONTROL PACKETS, MANGLE, THAT IS THE NAME OF THE GAME!

all prompt atoms from internal go into centralized category folders in C:\CodeProjects\codeNERD\internal\prompt\atoms

all prompt atoms from project specific shards go C:\CodeProjects\codeNERD\.nerd\agents in the agents subdirectory. 


**Kernel:** Google Mangle (Datalog) | **Runtime:** Go | **Philosophy:** Logic determines Reality; the Model merely describes it.

> [!IMPORTANT]
> **Build Instruction for Vector DB Support**
> To enable `sqlite-vec` mappings, you MUST use the following build command:
>
>rm c:/CodeProjects/codeNERD/nerd.exe 2>/dev/null; cd c:/ CodeProjects/codeNERD && CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers" go build -o nerd.exe ./cmd/nerd 2>&1 | grep -v "warning:" | grep -v note:

## Vision

The current generation of AI coding agents makes a category error: they ask LLMs to handle everything—creativity AND planning, insight AND memory, problem-solving AND self-correction—when LLMs excel at the former but struggle with the latter. codeNERD separates these concerns: the LLM remains the creative center where problems are understood, solutions are synthesized, and novel approaches emerge, while a deterministic Mangle kernel handles the executive functions that LLMs cannot reliably perform—planning, long-term memory, skill retention, and self-reflection. This architecture liberates the LLM to focus purely on what it does best while the harness ensures those creative outputs are channeled safely and consistently. The north star is an autonomous agent that pairs unbounded creative problem-solving with formal correctness guarantees: months-long sessions without context exhaustion, learned preferences without retraining, and parallel sub-agents—all orchestrated by logic, not luck. We are building the first coding agent where creative power and deterministic safety coexist by design.

## Core Principle: Inversion of Control

codeNERD inverts the traditional agent hierarchy:

- **LLM as Creative Center** - Problem-solving, solution synthesis, goal-crafting, and insight remain with the model
- **Logic as Executive** - Planning, memory, orchestration, and safety derive from deterministic Mangle rules
- **Transduction Interface** - NL↔Logic atom conversion channels creativity through formal structure

## Project Structure

```text
cmd/nerd/           # CLI entrypoint
internal/
├── core/           # Kernel, VirtualStore, ShardManager
├── perception/     # NL → Mangle atom transduction
├── articulation/   # Mangle atom → NL transduction
├── autopoiesis/    # Self-modification: Ouroboros, Thunderdome, tool learning
├── prompt/         # JIT Prompt Compiler, atoms, context-aware assembly
├── shards/         # CoderShard, TesterShard, ReviewerShard, ResearcherShard, NemesisShard
├── mangle/         # .gl schema and policy files
├── store/          # Memory tiers (RAM, Vector, Graph, Cold)
├── campaign/       # Multi-phase goal orchestration
└── world/          # Filesystem, AST projection, multi-lang data flow, holographic context

codeNERD Architecture and Philosophy Report

  1. Architecture

  codeNERD utilizes a Neuro-Symbolic architecture designed to bridge the gap between probabilistic Large Language Models (LLMs) and deterministic execution environments. It functions as a
  high-assurance coding agent framework.

   * Neuro-Symbolic & Creative-Executive Partnership:
      The system fundamentally separates concerns into two distinct domains:
       * Creative Center (LLM): Responsible for problem-solving, solution synthesis, and insight generation. It handles ambiguity and creativity.
       * Executive (Logic/Mangle): Responsible for planning, memory, orchestration, and safety. It uses deterministic rules to harness the LLM's output.

   * Perception Transducer:
       * Implementation: internal/perception/transducer.go
       * Function: Converts unstructured natural language user input into formal logic "atoms" (e.g., user_intent). It grounds fuzzy references to concrete file paths and symbols, calculating       
         confidence scores to trigger clarification loops if necessary.

   * Articulation Transducer:
       * Implementation: internal/articulation/emitter.go
       * Function: Converts internal logic states and atomic facts back into natural language for the user. It ensures that the user sees a helpful response while the system maintains precise       
         logical state.

   * World Model (Extensional Database - EDB):
       * Implementation: internal/world/fs.go, internal/world/ast.go
       * Function: Maintains the "Ground Truth" of the project. It projects the filesystem and abstract syntax trees (AST) into logic facts (e.g., file_topology, symbol_graph, dependency_link),     
         allowing the logic engine to reason about the codebase structure and state.

   * Executive Policy (Intensional Database - IDB):
       * Implementation: internal/mangle/policy.gl, internal/core/kernel.go
       * Function: A collection of deductive rules that derive the system's next_action. It encodes workflows like TDD repair loops (test_state -> next_action) and enforces safety constraints.      

   * Virtual Predicates:
       * Implementation: internal/core/virtual_store.go
       * Function: Serves as a Foreign Function Interface (FFI) that abstracts external APIs (filesystem, shell, MCP tools) into logic predicates. When the logic engine queries a virtual predicate  
         (e.g., file_content), it triggers the actual underlying system call.

   * Shard Agents:
       * Implementation: internal/shards/ (coder.go, tester.go, reviewer.go, researcher.go)
       * Function: Ephemeral, specialized sub-kernels spawned for parallel task execution.
           * Coder: Code generation and refactoring.
           * Tester: Test execution and coverage analysis.
           * Reviewer: Security scans and code review.
           * Researcher: Knowledge gathering and documentation.

   * Memory Tiers:
       * RAM: Short-term working memory (FactStore) for the current session.
       * Vector: Persistent semantic memory (SQLite + embeddings) for similar content retrieval.
       * Cold: Permanent storage (cold_storage table) for learned preferences and patterns.

   * Piggyback Protocol:
       * Implementation: internal/articulation/emitter.go
       * Function: A dual-channel communication protocol. The agent outputs a JSON object containing a visible surface_response for the user and a hidden control_packet for the kernel. This allows  
         the agent to update its internal logical state (e.g., task_status) independently of the conversational text.

  2. Core Philosophy

   * Logic-First CLI:
      Unlike chat-first agents, codeNERD is driven by a logic kernel. Text generation is a side effect of logical processes, not the primary driver. The state of the system is defined by facts, not 
  conversation history.

   * Separation of Concerns:
      By decoupling creativity (LLM) from execution (Mangle Engine), codeNERD prevents the LLM from hallucinating actions or violating safety protocols. The LLM suggests; the Kernel executes.       

   * Deterministic Safety:
      Safety is not a prompt instruction but a logic rule. The "Constitutional Gate" (permitted(Action) :- safe_action(Action)) ensures that dangerous actions (like rm -rf) are blocked
  deterministically unless specific override conditions are met.

   * LLM as Creative Center:
      The architecture acknowledges that LLMs excel at synthesis and pattern matching. codeNERD leverages this by feeding the LLM highly specific, logic-derived context ("Context Atoms") and asking 
  it to solve specific problems, rather than asking it to manage the entire workflow.

  3. Implementation Patterns

   * Hallucination Firewall:
       * Pattern: permitted(Action) check.
       * Details: Every action proposed by the Transducer or a Shard is validated against the Mangle logic policy. If the logic cannot derive a permission rule for the action, it is strictly        
         blocked, preventing the execution of hallucinated or malicious commands.

   * Grammar-Constrained Decoding:
       * Pattern: Schema validation and recovery.
       * Details: Output from the LLM is forced to conform to strict Mangle syntax and JSON schemas. This ensures that the "thoughts" of the agent can be parsed and executed reliably by the
         deterministic kernel.

   * OODA Loop:
       * Pattern: Observe -> Orient -> Decide -> Act.
       * Details: The system cycles through:
           1. Observe: Transducer converts input to atoms.
           2. Orient: Spreading Activation selects relevant context facts based on logical dependencies.
           3. Decide: Mangle Engine derives the single best next_action.
           4. Act: Virtual Store executes the tool or command.

   * Autopoiesis (Self-Learning):
       * Pattern: Runtime feedback loops (internal/autopoiesis/).
       * Details: The system tracks rejection and acceptance of its actions. Repeated rejections of a specific pattern trigger a preference_signal, which promotes a new rule to long-term memory. The
         Ouroboros Loop detects missing capabilities and can trigger a generate_tool action to self-implement missing functionality.

   * Campaign Orchestration:
       * Pattern: Context Paging and Multi-Phase Goals (internal/campaign/).
       * Details: For complex goals (e.g., migrations), the system breaks the work into phases. It uses "Context Paging" to manage token budget, loading only the context relevant to the current     
         phase while keeping core facts and working memory available.



  Adversarial Engineering Report: Nemesis & Panic Maker

  codeNERD employs an Adversarial Co-Evolution strategy. Instead of relying solely on passive testing, it actively attempts to break its own code using two distinct but related components: the Panic
  Maker (tactical tool breaker) and the Nemesis Shard (strategic system breaker).

  1. Panic Maker (The Tactical Breaker)
   * Implementation: internal/autopoiesis/panic_maker.go
   * Scope: Micro-level. Focused on breaking individual tools and functions during the generation phase (Ouroboros loop).
   * Workflow:
       1. Static Analysis: Analyzes the generated tool's source code to identify specific vulnerability patterns (e.g., pointer dereferences, channel operations).
       2. Attack Vector Generation: Uses the LLM to craft targeted JSON inputs designed to trigger crashes.
       3. Thunderdome: Executes the attacks against the tool. If the tool crashes (panics, OOMs, deadlocks), it is rejected and sent back for hardening.
   * Attack Categories:
       * nil_pointer: Exploits unchecked pointer dereferences.
       * boundary: Max int, negative indices, empty slices.
       * resource: Massive allocations to trigger OOM (Out of Memory).
       * concurrency: Race conditions and channel deadlocks.
       * format: Malformed JSON/XML inputs.
  2. Nemesis Shard (The Strategic Adversary)
   * Implementation: internal/shards/nemesis/nemesis.go
   * Scope: System-level. A persistent "Type B" Specialist Shard that acts as a gatekeeper for code changes.
   * Philosophy: "The Nemesis does not seek destruction - it seeks truth." It acts as a hostile sparring partner for the Coder Shard.
   * Core Capabilities:
       * The Gauntlet: A required pipeline phase. A patch is only "battle-hardened" if it survives the Nemesis.
       * Attack Tool Generation: Unlike Panic Maker (which sends inputs), Nemesis writes and compiles full Go attack binaries (nemesis_attack_tool) to exploit logic flaws or race conditions in the
         target system.
       * Lazy Pattern Detection: actively scans for "lazy" coding patterns (e.g., return nil, generic error messages) that signal weakness.
       * The Armory: (internal/shards/nemesis/armory.go) A persistent store where Nemesis remembers effective attack strategies, allowing it to get smarter over time.

  Summary of Differences


  ┌─────────────┬──────────────────────┬────────────────────────────┬────────────────────────────────────┐
  │ Feature     │ Scope                │ Method                     │ Goal                               │
  ├─────────────┼──────────────────────┼────────────────────────────┼────────────────────────────────────┤
  │ Panic Maker │ Single Function/Tool │ Malformed Inputs (Fuzzing) │ Ensure tool robustness before use. │
  │ Nemesis     │ Full System/Patch    │ Compilable Attack Programs │ Reject weak architecture & logic.  │
  └─────────────┴──────────────────────┴────────────────────────────┴────────────────────────────────────┘


✦ Thunderdome: The Adversarial Battleground

  Thunderdome is the conceptual and operational environment within codeNERD where adversarial attacks are executed against generated code and submitted patches. It serves as the ultimate proving    
  ground for code resilience, feeding back results that drive the autopoietic self-improvement loops.

  Role and Functionality

   * Adversarial Testing Environment: Thunderdome is where the offensive capabilities of the PanicMaker and the Nemesis Shard are unleashed.
   * Code Hardening: Its primary purpose is to expose weaknesses (panics, deadlocks, OOMs, logic flaws) in code and patches, thereby driving their regeneration and improvement. Code that survives   
     Thunderdome is considered "battle-hardened."
   * Feedback Loop: The outcomes from Thunderdome (whether code "survived" or was "defeated") are crucial feedback for the ToolGenerator (when PanicMaker is involved) and for the overall patch      
     review process (when Nemesis is involved), dictating whether code needs further refinement or can proceed.

  Integration

   * Panic Maker: Attacks generated by the PanicMaker (adversarial inputs designed to break individual tools) are run within the Thunderdome context. The results (THUNDERDOME RESULT: SURVIVED or
     THUNDERDOME RESULT: DEFEATED) directly inform the tool generation and regeneration process.
   * Nemesis Shard: The Nemesis Shard orchestrates its comprehensive adversarial analysis, often referred to as "The Gauntlet," within the conceptual framework of Thunderdome. This involves
     deploying and executing Go attack binaries against a target patch to find systemic vulnerabilities.

  Key Aspects

   * Outcome-Oriented: Thunderdome is not just about running tests; it's about definitively determining if the code can withstand targeted attacks.
   * Deterministic Feedback: It provides clear, actionable results that the logical kernel can process, ensuring that the system learns from its vulnerabilities.
   * Foundational to Autopoiesis: The continuous cycle of attack and defense within Thunderdome is fundamental to codeNERD's self-learning and self-improving capabilities.

> are there any other features like:
  - Contextual awareness
  - Dynamic adaptation
  - Multi-modal input
  - Real-time feedback
  - Advanced analytics
  - Cross-platform sync
  - Customizable workflows
  - Predictive suggestions
  - Security enhancements
  - Performance optimization

  
✦ Contextual Awareness

  codeNERD exhibits a highly advanced and multi-layered approach to contextual awareness, a cornerstone of its neuro-symbolic architecture. It moves beyond simple chat history to a dynamic,
  logic-driven context management system.

   * `CompilationContext` (internal/prompt/context.go): This central structure encapsulates up to 10 distinct contextual dimensions, including:
       * Operational Mode: (e.g., /debugging, /dream)
       * Campaign Phase: For multi-phase goal management.
       * Shard Type: (e.g., /coder, /reviewer)
       * Language & Framework: (e.g., /go, /bubbletea)
       * Intent: User's current verb and target.
       * World States: Real-time conditions like failing tests, active diagnostics, security issues, new files, and code churn.
       * Token Budget: Managed dynamically to optimize LLM interactions.
      This CompilationContext is critical for JIT Prompt Compilation, ensuring that only the most relevant "prompt atoms" are selected for LLM injection.

   * `SessionContext` (internal/types/types.go, internal/core/shard_manager.go): Implementing a "Blackboard Pattern," the SessionContext acts as a shared working memory across different shards and
     turns. It provides a comprehensive snapshot of the current operational state, including:
       * Compressed History: Semantically condensed past interactions.
       * Current Diagnostics & Test State: Immediate feedback on code health.
       * Active Files, Symbols, and Dependencies: A view into the code being worked on.
       * Git Context: Branch, modified files, recent commits for historical awareness ("Chesterton's Fence").
       * Campaign Context: Details of active campaigns, phases, and goals.
       * Prior Shard Outputs: Results from other shards to enable cross-shard collaboration.
       * Knowledge Atoms & Specialist Hints: Retrieved relevant knowledge.
       * Allowed/Blocked Actions & Safety Warnings: Enforced constitutional guards.

   * Memory Tiers: codeNERD utilizes a tiered memory system:
       * RAM (FactStore): The kernel's immediate working memory for the current session.
       * Vector Store: Used for semantic retrieval, particularly in prompt atom selection and research.
       * Knowledge Graph: A persistent, relational store of facts, symbols, and relationships within the codebase, crucial for logical reasoning and long-term memory.
       * Cold Storage: For permanent learned patterns and preferences.

   * Spreading Activation (internal/context/activation.go, internal/core/defaults/policy.mg): This is a core mechanism for Logical Context Selection, replacing traditional vector-based RAG. It
     operates on the Mangle knowledge graph, dynamically identifying and retrieving facts structurally relevant to the current user intent and system state. Key aspects include:
       * Context-Directed Spreading Activation (CDSA): Dynamically adjusts activation flow based on logical rules (e.g., prioritizing dependency links in a "security" context).
       * Activation Scores: Facts are assigned scores based on recency, relevance, dependency, and campaign/issue context, ensuring that only high-priority information enters the LLM context window.

   * Context Paging & Compression (internal/campaign/context_pager.go, internal/context/compressor.go):
       * Context Paging: Manages the context window during long-running campaigns by activating context for the current phase, compressing completed phase contexts, and prefetching for upcoming
         tasks.
       * Semantic Compression: The Compressor achieves "Infinite Context" by transforming verbose conversational history into concise, logical Mangle facts. This dramatically reduces token usage,
         allowing for extended, multi-turn interactions without LLM context window exhaustion.

   * `context.Context` (Go Standard Library): Go's native context.Context is ubiquitous throughout the codebase, providing robust support for:
       * Cancellation: Propagating cancellation signals across goroutines and operations.
       * Timeouts: Enforcing execution limits on various tasks.
       * Request-Scoped Values: Passing metadata throughout a request's lifecycle.

   * Context7 (internal/shards/researcher/tools.go): An integrated research tool specifically designed to fetch curated, LLM-optimized documentation for libraries and frameworks, enriching the
     contextual knowledge available to the agent.

  In summary, codeNERD's contextual awareness is deeply integrated into its architecture, ensuring that the agent always operates with a precise, relevant, and budget-conscious understanding of its
  environment, tasks, and history.

✦ Dynamic Adaptation is a foundational and deeply integrated feature within codeNERD, primarily implemented through its Autopoiesis (self-creation) system.
  This goes beyond simple learning to enable the agent to self-modify and evolve its capabilities based on experience.

  Dynamic Adaptation

  codeNERD’s dynamic adaptation is primarily driven by its Autopoiesis system, which allows the agent to learn from its interactions, failures, and successes, and to adapt its behavior and even its
  own tools over time.

   * Autopoiesis Orchestrator (internal/autopoiesis/autopoiesis.go): This central component coordinates all self-modification capabilities. It monitors the agent's performance, detects needs for new
     tools or rule adjustments, and manages the entire learning lifecycle.

   * Self-Learning from Experience:
       * LearningStore (internal/store/learning.go): This dedicated persistence layer records successful and failed patterns across sessions for each shard. Learnings are stored in SQLite databases
         per shard type.
       * Feedback & Learning System (internal/autopoiesis/feedback.go): This closes the autopoiesis loop by evaluating tool quality, recording patterns of success and failure, and using this
         feedback to refine existing tools or generate new ones.
       * Rejection/Acceptance Tracking (internal/shards/coder/autopoiesis.go, internal/shards/tester/autopoiesis.go): Individual shards track the outcomes of their actions. For instance, the Coder
         shard tracks rejected code edits, and the Tester shard tracks test failures and successes. Recurring patterns (e.g., 3 rejections of a specific code change) trigger learning.
       * Decay Confidence: The LearningStore implements a "forgetting" mechanism (DecayConfidence) where the confidence of old learnings reduces over time if they are not reinforced.

   * Ouroboros Loop (internal/autopoiesis/ouroboros.go): This is codeNERD's self-correction and tool-generation engine. It's a transactional state machine that attempts to fulfill a ToolNeed (a
     detected capability gap). The loop involves:
       1. Specification: Defining the new tool's purpose and interface.
       2. Generation: Producing the tool's code and tests.
       3. Safety Check: Ensuring the generated code meets constitutional safety standards.
       4. Thunderdome: Adversarial testing against the PanicMaker and Nemesis Shard to identify vulnerabilities.
       5. Simulation: Testing the tool in a simulated environment (Dream Mode).
       6. Compilation & Registration: Making the new tool available to the system.
      This iterative process ensures that new capabilities are robust and validated before integration.

   * Dream State Learning (internal/core/dream_learning.go): This is a multi-agent simulation mode (DreamMode) where the agent can explore hypothetical scenarios, consult with specialist shards, and
     extract learnable insights without affecting the live operational state. User confirmation of these learnings then routes them to appropriate persistence stores.

   * Dynamic Policy Adjustment:
       * FeedbackLoop (internal/mangle/feedback/loop.go): This system uses Mangle logic and LLM interaction to propose and validate new policy rules based on observed behavior and system
         performance. This allows the core policy to adapt dynamically.
       * Legislator Shard (internal/shards/system/legislator.go): Can compile and incorporate new Mangle rules into the system's policy, enabling runtime adaptation of its executive functions.

   * Adaptive Workflows:
       * Campaign Replanning (internal/campaign/replan.go): Campaigns are long-running, multi-phase goals that can dynamically adapt their plans in response to failures, new requirements, or        
         changing conditions. The "Replanning Engine" is an "Adaptive Controller" that takes corrective actions based on current status.
       * Adaptive Batch Sizing (internal/shards/researcher/researcher.go): The Researcher shard can dynamically adjust the size of its research batches based on topic complexity and historical      
         performance, optimizing its approach to knowledge gathering.

  In essence, codeNERD is designed to be a continuously improving system, actively seeking out its own limitations, learning from every interaction, and dynamically adapting its capabilities and    
  internal policies to become more effective over time.

✦ The JIT (Just-In-Time) Prompt Compiler is a core component of codeNERD, replacing static system prompts with dynamically assembled ones. It represents a paradigm shift from fixed instructions to
  fluid, context-aware prompt engineering.

  JIT System Architecture

   1. Atom-Based Architecture:
       * System prompts are not stored as monolithic strings. Instead, they are broken down into thousands of atomic units called Prompt Atoms (stored in internal/prompt/atoms/ as YAML files).
       * Each atom has metadata: id, category (e.g., identity, capability, context), content, and Contextual Selectors (rules for when to include it).

   2. Compilation Process (internal/prompt/compiler.go):
      When a shard (like CoderShard or ReviewerShard) needs to interact with an LLM, the JIT compiler executes the following pipeline:
       * Context Gathering: Collects the current CompilationContext (Operational Mode, Campaign Phase, Shard Type, Language, Intent, World State, Token Budget).
       * Skeleton Selection: Uses Mangle logic (e.g., jit_compiler.mg) to select mandatory atoms that define the shard's core identity and mission.
       * Flesh Selection: Uses vector search and context matching to select optional, relevant atoms (e.g., specific framework documentation, project-specific domain knowledge, or recent error
         patterns).
       * Budgeting: Fits the selected atoms into the available token budget, prioritizing high-value information.
       * Assembly: Concatenates the selected atoms into a coherent system prompt string.

   3. Key Benefits:
       * Infinite Effective Prompt Length: The system can draw from a corpus of millions of tokens but only sends the relevant ~20k tokens to the LLM for any given task.
       * Contextual Specialization: A "Coder" shard working on a Python/Django project in a "Debugging" phase receives a drastically different prompt than one working on a Go/Mangle project in a
         "Planning" phase.
       * Dynamic Evolution: New atoms (learnings, new tool definitions) can be added to the corpus at runtime and immediately become available for future compilations.

   4. Integration:
       * Shard Integration: Shards use the PromptAssembler (backed by the JIT compiler) to generate their system prompts (AssembleSystemPrompt).
       * Autopoiesis: The Autopoiesis system leverages JIT to inject learned patterns and tool usage instructions dynamically.
       * Observability: The /jit command in the CLI allows users to inspect the last compiled prompt and view compilation statistics (atoms selected, tokens used, etc.).

  In summary, the JIT Prompt Compiler acts as a dynamic "knowledge hypervisor," ensuring that the LLM is always primed with the exact instructions and context needed for the specific millisecond of 
  execution, maximizing performance and minimizing hallucination.

```

## Full Specifications

For detailed architecture and implementation specs, see:

- [.claude/skills/codenerd-builder/references/](.claude/skills/codenerd-builder/references/) - Full architecture docs
- [.claude/skills/mangle-programming/references/](.claude/skills/mangle-programming/references/) - Mangle language reference


## Notice on unused wiring... investigate consuming unused methods and parameters and code before removing it... ultrathink on it even... this is a living codebase and we forget to wire things up all the time... 

## Skills

Use skills to get specialized knowledge for different tasks. Invoke with `/skill:<name>`.

### codenerd-builder

**When:** Implementing codeNERD components - kernel, transducers, shards, virtual predicates, TDD loops, Piggyback Protocol, or any neuro-symbolic architecture work.

### mangle-programming

**When:** Writing or debugging Mangle logic - schemas, rules, queries, aggregations, recursive closures, or understanding Datalog semantics. Complete language reference from basics to production optimization.

### research-builder

**When:** Building knowledge ingestion systems - ResearcherShard, llms.txt parsing, Context7-style processing, knowledge atom extraction, 4-tier memory persistence, or specialist hydration.

### rod-builder

**When:** Implementing browser automation - web scraping, CDP event handling, session management, DOM projection, or the semantic browser peripheral.

### skill-creator

**When:** Creating or updating skills - designing SKILL.md structure, bundled resources, reference organization, or skill metadata.

### charm-tui

**When:** Building terminal user interfaces with Bubbletea and Lipgloss - interactive CLI apps, forms, tables, lists, spinners, progress bars, styled output, or any TUI component using the Charm ecosystem. Includes stability patterns, goroutine safety, and the complete Bubbles component library.

### prompt-architect

**When:** Writing or auditing shard prompts - static vs dynamic prompt layers, Piggyback Protocol compliance, context injection patterns, tool steering, specialist knowledge hydration, or debugging LLM behavior. Essential for creating "God Tier" maximalist prompts that leverage codeNERD's 100:1 semantic compression.

### integration-auditor

**When:** Verifying system integration - debugging "code exists but doesn't run" issues, pre-commit wiring checks, new feature integration, or shard lifecycle verification. Covers all 39+ codeNERD integration systems.

### stress-tester

**When:** Live stress testing codeNERD - pre-release stability verification, finding panics and edge cases, validating resource limits, testing system recovery. Includes 27 workflows across 8 categories with 4 severity levels (conservative, aggressive, chaos, hybrid). Integrates with log-analyzer for post-test Mangle queries.

## FOR ALL NEW LLM SYSTEMS, JIT IS THE STANDARD, ALWAYS CREATE NEW PROMPT ATOMS AND USE THE JIT SYSTEM

## Key Implementation Files

| Component | Location | Purpose |
|-----------|----------|---------|
| Kernel | [internal/core/kernel.go](internal/core/kernel.go) | Mangle engine + fact management |
| Policy | [internal/mangle/policy.gl](internal/mangle/policy.gl) | IDB rules (20 sections) |
| Schemas | [internal/mangle/schemas.gl](internal/mangle/schemas.gl) | EDB declarations |
| VirtualStore | [internal/core/virtual_store.go](internal/core/virtual_store.go) | FFI to external systems |
| ShardManager | [internal/core/shard_manager.go](internal/core/shard_manager.go) | Shard lifecycle |
| Transducer | [internal/perception/transducer.go](internal/perception/transducer.go) | NL→Atoms |
| Emitter | [internal/articulation/emitter.go](internal/articulation/emitter.go) | Atoms→NL (Piggyback) |
| JIT Compiler | [internal/prompt/compiler.go](internal/prompt/compiler.go) | Runtime prompt assembly |
| Nemesis | [internal/shards/nemesis/nemesis.go](internal/shards/nemesis/nemesis.go) | Adversarial patch analysis |
| Thunderdome | [internal/autopoiesis/thunderdome.go](internal/autopoiesis/thunderdome.go) | Attack vector arena |
| DataFlow | [internal/world/dataflow_multilang.go](internal/world/dataflow_multilang.go) | Multi-language taint analysis |
| Holographic | [internal/world/holographic.go](internal/world/holographic.go) | Impact-aware context builder |
| Hypotheses | [internal/shards/reviewer/hypotheses.go](internal/shards/reviewer/hypotheses.go) | Mangle→LLM verification |

## Development Guidelines

### Mangle Rules

- All predicates require `Decl` in schemas.gl before use
- Variables are UPPERCASE, constants are `/lowercase`
- Negation requires all variables bound elsewhere (safety)
- End every statement with `.`

### Go Patterns

- Shards implement the `Shard` interface with `Execute(ctx, task) (string, error)`
- Facts use `ToAtom()` to convert Go structs to Mangle AST
- Virtual predicates abstract external APIs into logic queries

### Testing

- Run `go test ./...` before committing
- Build with `go build -o nerd.exe ./cmd/nerd`

### Git

- Push to GitHub regularly
- Use conventional commits

### Model Configuration

- Config file: `.nerd/config.json`
- Gemini 3 Pro model ID: `gemini-3-pro-preview` (yes, Gemini 3 exists as of Dec 2024)

## Nomenclature

### Shard Lifecycle Types

| Type | Constant | Description | Memory | Creation |
|------|----------|-------------|--------|----------|
| **Type A** | `ShardTypeEphemeral` | Generalist agents. Spawn → Execute → Die. | RAM only | `/review`, `/test`, `/fix` |
| **Type B** | `ShardTypePersistent` | Domain specialists with pre-loaded knowledge. | SQLite-backed | `/init` project setup |
| **Type U** | `ShardTypeUser` | User-defined specialists via wizard. | SQLite-backed | `/define-agent` |
| **Type S** | `ShardTypeSystem` | Long-running system services. | RAM | Auto-start |

### Shard Implementations

| Shard | File | Purpose |
|-------|------|---------|
| CoderShard | `internal/shards/coder/` | Code generation, file edits, refactoring |
| TesterShard | `internal/shards/tester/` | Test execution, coverage analysis |
| ReviewerShard | `internal/shards/reviewer/` | Code review, pre-flight checks, hypothesis verification |
| ResearcherShard | `internal/shards/researcher/` | Knowledge gathering, documentation ingestion |
| NemesisShard | `internal/shards/nemesis/` | Adversarial testing, patch breaking, chaos tools |
| ToolGenerator | `internal/shards/tool_generator/` | Ouroboros: self-generating tools |

### System Shards (Type S)

| Shard | Purpose |
|-------|---------|
| `perception_firewall` | NL → atoms transduction |
| `world_model_ingestor` | file_topology, symbol_graph maintenance |
| `executive_policy` | next_action derivation |
| `constitution_gate` | Safety enforcement |
| `tactile_router` | Action → tool routing |
| `session_planner` | Agenda/campaign orchestration |
| `nemesis` | Adversarial co-evolution, patch breaking |

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

## Full Specifications

For detailed architecture and implementation specs, see:

- [.claude/skills/codenerd-builder/references/](.claude/skills/codenerd-builder/references/) - Full architecture docs
- [.claude/skills/mangle-programming/references/](.claude/skills/mangle-programming/references/) - Mangle language reference

here is the definitive list of the Top 30 Common Errors AI coding agents make when writing Mangle code.


These are categorized by the layer of the stack where the "Stochastic Gap" occurs: Syntax, Logic/Safety, Data Structures, and Integration.

I. Syntactic Hallucinations (The "Soufflé/SQL" Bias)
AI models trained on SQL, Prolog, and Soufflé often force those syntaxes into Mangle.


Atom vs. String Confusion 


Error: Using "active" when the schema requires /active.

Correction: Use /atom for enums/IDs. Mangle treats these as disjoint types; they will never unify.


Soufflé Declarations 


Error: .decl edge(x:number, y:number).

Correction: Decl edge(X.Type<int>, Y.Type<int>). (Note uppercase Decl and type syntax).


Lowercase Variables 

Error: ancestor(x, y) :- parent(x, y). (Prolog style).

Correction: ancestor(X, Y) :- parent(X, Y). Variables must be UPPERCASE.


Inline Aggregation (SQL Style) 


Error: total(Sum) :- item(X), Sum = sum(X).

Correction: Use the pipe operator: ... |> do fn:group_by(), let Sum = fn:Sum(X).


Implicit Grouping 


Error: Assuming variables in the head automatically trigger GROUP BY (like SQL).

Correction: Grouping is explicit in the do fn:group_by(...) transform step.


Missing Periods 

Error: Ending a rule with a newline instead of ..

Correction: Every clause must end with a period ..


Comment Syntax 

Error: // This is a comment or /* ... */.

Correction: Use # This is a comment.


Assignment vs. Unification 

Error: X := 5 or let X = 5 inside a rule body (without pipe).

Correction: Use unification X = 5 inside the body, or let only within a transform block.

II. Semantic Safety & Logic (The "Datalog" Gap)
Mangle requires strict logical validity that probabilistic models often miss.


Unsafe Head Variables 


Error: result(X) :- other(Y). (X is unbounded).

Correction: Every variable in the head must appear in a positive atom in the body.


Unsafe Negation 


Error: safe(X) :- not distinct(X).

Correction: Variables in a negated atom must be bound first: safe(X) :- candidate(X), not distinct(X).


Stratification Cycles 


Error: p(X) :- not q(X). q(X) :- not p(X).

Correction: Ensure no recursion passes through a negation. Restructure logic into strict layers (strata).


Infinite Recursion (Counter Fallacy) 

Error: count(N) :- count(M), N = fn:plus(M, 1). (Unbounded generation).

Correction: Always bound recursion with a limit or a finite domain (e.g., N < 100).


Cartesian Product Explosion 

Error: Placing large tables before filters: res(X) :- huge_table(X), X = /specific_id.

Correction: Selectivity first: res(X) :- X = /specific_id, huge_table(X).


Null Checking (Open World Bias) 


Error: check(X) :- data(X), X != null.

Correction: Mangle follows the Closed World Assumption. If a fact exists, it is not null. "Missing" facts are simply not there.


Duplicate Rule Definitions 

Error: Thinking multiple rules overwrite each other.

Correction: Multiple rules create a UNION. p(x) :- a(x). and p(x) :- b(x). means p is true if a OR b is true.


Anonymous Variable Misuse 

Error: Using _ when the value is actually needed later in the rule.

Correction: Use _ only for values you truly don't care about. It never binds.

III. Data Types & Functions (The "JSON" Bias)
AI agents often hallucinate object-oriented accessors for Mangle's structured data.


Map Dot Notation 

Error: Val = Map.key or Map['key'].

Correction: Use :match_entry(Map, /key, Val) or :match_field(Struct, /key, Val).


List Indexing 

Error: Head = List[0].

Correction: Use :match_cons(List, Head, Tail) or fn:list:get(List, 0).


Type Mismatch (Int vs Float) 

Error: X = 5 when X is declared Type<float>.

Correction: Mangle is strict. Use 5.0 for floats, 5 for ints.


String Interpolation 

Error: msg("Error: $Code").

Correction: Use fn:string_concat or build list structures. Mangle has no string interpolation.


Hallucinated Functions 

Error: fn:split, fn:date, fn:substring (assuming StdLib parity with Python).

Correction: Verify function existence in builtin package. Mangle's standard library is minimal.


Aggregation Safety 

Error: ... |> do fn:group_by(UnboundVar) ...

Correction: Grouping variables must be bound in the rule body before the pipe |>.


Struct Syntax 

Error: {"key": "value"} (JSON style).

Correction: { /key: "value" } (Note the atom key and spacing).

IV. Go Integration & Architecture (The "API" Gap)
When embedding Mangle, AI agents fail to navigate the boundary between Go and Logic.


Fact Store Type Errors 


Error: store.Add("pred", "arg").

Correction: Must use engine.Atom, engine.Number types wrapped in engine.Value.


Incorrect Engine Entry Point 

Error: engine.Run() (Hallucination).

Correction: Use engine.EvalProgram or engine.EvalProgramNaive.


Ignoring Imports 

Error: Generating Mangle code without necessary package references or failing to import the Go engine package correctly.

Correction: Explicitly manage github.com/google/mangle/engine.


External Predicate Signature 

Error: Writing a Go function for a predicate that returns (interface{}, error).

Correction: External predicates require func(query engine.Query, cb func(engine.Fact)) error.


Parsing vs. Execution 

Error: Passing raw strings to EvalProgram.

Correction: Code must be parsed (parse.Unit) and analyzed (analysis.AnalyzeOneUnit) before evaluation.


Assuming IO access 

Error: read_file(Path, Content).

Correction: Mangle is pure. IO must happen in Go before execution (loading facts) or via external predicates.


Package Hallucination (Slopsquatting) 


Error: Importing non-existent Mangle libraries (use /std/date).

Correction: Verify imports. Mangle has a very small, specific ecosystem.

How to Avoid These Mistakes (For the Mangle Architect)
Feed the Grammar: Provide the "Complete Syntax Reference" (File 200) in the prompt context.

Solver-in-the-Loop: Do not trust "Zero-Shot" code. Run a loop: Generate -> Parse (with mangle/parse) -> Feed Errors back to LLM -> Regenerate.

Explicit Typing: Force the AI to declare types (Decl) first. This forces it to decide between /atoms and "strings" early.


Review for Liveness: Manually audit recursive rules for termination conditions.