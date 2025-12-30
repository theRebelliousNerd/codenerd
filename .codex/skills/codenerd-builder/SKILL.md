---
name: codenerd-builder
description: Build the codeNERD Logic-First Neuro-Symbolic coding agent framework. This skill should be used when the user asks to implement or modify core codeNERD architecture (Mangle kernel, schemas/policy, perception or articulation transducers, shards, virtual predicates, JIT prompt compiler, autopoiesis, campaigns, memory tiers) or any neuro-symbolic agent logic in Go that follows the Creative-Executive Partnership pattern.
---

# codeNERD Builder

Build the codeNERD high-assurance Logic-First CLI coding agent.

> **Stability Notice:** This codebase is under active development and not yet stable. Code snippets in this skill illustrate architectural patterns but may not match current implementations exactly. When implementing, always read the actual source files to verify current APIs and signatures. The architecture and concepts are stable; specific implementations are evolving. please make additions to this skill as architecture changes. Also, the tests rapidly become stale in this codebase to always be sure to check if the tests are valid or if they need refactoring before trusting output.

## Build Instructions

**IMPORTANT: To enable sqlite-vec for vector DB support, use this build command:**

```powershell
$env:CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers"; go build ./cmd/nerd
```

## Integrated Skills Ecosystem

codeNERD development leverages a constellation of specialized skills. Each skill handles a specific domain; together they form a complete development toolkit.

### Skill Index

Quick reference with links to detailed registry entries. For complete documentation on each skill including trigger conditions, bundled resources, and integration points, see [references/skill-registry.md](references/skill-registry.md).

| ID | Skill | Domain | When to Use |
|----|-------|--------|-------------|
| [SK-001](references/skill-registry.md#sk-001-codenerd-builder) | codenerd-builder | Architecture | Kernel, schemas/policy, transducers, shards, virtual predicates, JIT, autopoiesis, memory tiers |
| [SK-002](references/skill-registry.md#sk-002-mangle-programming) | mangle-programming | Logic | Schemas, predicates, policies, rules, queries, aggregation, safety |
| [SK-003](references/skill-registry.md#sk-003-go-architect) | go-architect | Go Patterns | Writing/refactoring Go, concurrency, context, errors |
| [SK-004](references/skill-registry.md#sk-004-charm-tui) | charm-tui | Terminal UI | TUI/terminal UI, MVU, forms, lists, tables |
| [SK-005](references/skill-registry.md#sk-005-research-builder) | research-builder | Knowledge | ResearcherShard, llms.txt/Context7, knowledge atoms, 4-tier memory |
| [SK-006](references/skill-registry.md#sk-006-rod-builder) | rod-builder | Browser | Rod/CDP automation, scraping, E2E, screenshots/PDFs |
| [SK-007](references/skill-registry.md#sk-007-log-analyzer) | log-analyzer | Debugging | logquery + Mangle facts, cross-system tracing |
| [SK-008](references/skill-registry.md#sk-008-skill-creator) | skill-creator | Tooling | Create/update/package skills, frontmatter, registry |
| [SK-010](references/skill-registry.md#sk-010-cli-engine-integration) | cli-engine-integration | LLM Integration | Claude/Codex CLI backends, LLMClient, auth/streaming |
| [SK-011](references/skill-registry.md#sk-011-prompt-architect) | prompt-architect | Prompt Engineering | Prompt atoms, JIT injection, Piggyback, tool steering |

### Skill Map

```text
┌─────────────────────────────────────────────────────────────────────────────┐
│                         codeNERD Development Skills                          │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌─────────────────────┐     ┌─────────────────────┐                        │
│  │   codenerd-builder  │────▶│  mangle-programming │                        │
│  │   (Architecture)    │     │  (Logic Language)   │                        │
│  └─────────────────────┘     └─────────────────────┘                        │
│           │                           │                                      │
│           │  ┌────────────────────────┘                                      │
│           │  │                                                               │
│           ▼  ▼                                                               │
│  ┌─────────────────────┐     ┌─────────────────────┐                        │
│  │    go-architect     │────▶│     charm-tui       │                        │
│  │   (Go Patterns)     │     │    (Terminal UI)    │                        │
│  └─────────────────────┘     └─────────────────────┘                        │
│           │                                                                  │
│           │                                                                  │
│           ▼                                                                  │
│  ┌─────────────────────┐     ┌─────────────────────┐                        │
│  │  research-builder   │────▶│    rod-builder      │                        │
│  │ (Knowledge Systems) │     │ (Browser Automation)│                        │
│  └─────────────────────┘     └─────────────────────┘                        │
│           │                                                                  │
│           │                                                                  │
│           ▼                                                                  │
│  ┌─────────────────────┐                                                    │
│  │    log-analyzer     │                                                    │
│  │   (Debug + Mangle)  │                                                    │
│  └─────────────────────┘                                                    │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### When to Use Each Skill

| Skill | When to Use | Key Capabilities |
|-------|-------------|------------------|
| **codenerd-builder** | Core architecture: kernel, schemas/policy, transducers, shards, virtual predicates, JIT, autopoiesis, campaigns, memory tiers | Architecture patterns, component structure, JIT prompt compilation |
| **mangle-programming** | Schemas, predicates, policies, rules, queries, aggregations, safety/stratification | Mangle syntax, aggregation, stratification |
| **go-architect** | Write/review/refactor Go; concurrency, context, errors, memory safety | Goroutine safety, context, testing patterns |
| **charm-tui** | TUI/terminal UI, MVU, forms, lists, tables, spinners, progress bars, markdown | Bubbletea, Lipgloss, forms, lists |
| **research-builder** | ResearcherShard, llms.txt/Context7 ingestion, knowledge atoms, quality scoring, 4-tier memory | llms.txt, Context7, 4-tier memory |
| **rod-builder** | Rod/CDP automation, scraping, E2E, sessions, screenshots/PDFs | CDP, session management, screenshots |
| **log-analyzer** | Debugging with logs, cross-system tracing | Log → Mangle facts, query patterns |
| **prompt-architect** | Prompt atoms, shard prompts, JIT injection, Piggyback, tool steering | God Tier templates, JIT compilation, Piggyback protocol |
| **integration-auditor** | End-to-end wiring checks, missing hooks, code exists but doesn't run | 39-system audit, shard type matrix, diagnostics |

### Skill Interactions by Component

#### Kernel Development (internal/core/)

```text
codenerd-builder    →  Architecture, kernel patterns
mangle-programming  →  Schema declarations, policy rules
go-architect        →  Concurrency safety, error handling
log-analyzer        →  Debug kernel derivations
```

#### Shard Development (internal/shards/)

```text
codenerd-builder    →  Shard lifecycle, execution patterns
go-architect        →  Goroutine management, context propagation
research-builder    →  ResearcherShard implementation
log-analyzer        →  Shard execution tracing
```

#### CLI Development (cmd/nerd/)

```text
charm-tui           →  Interactive terminal UI
go-architect        →  Main loop, signal handling
codenerd-builder    →  Chat processing, session management
```

#### Browser Integration (internal/browser/)

```text
rod-builder         →  CDP automation, page interaction
go-architect        →  Connection pooling, timeouts
research-builder    →  Web scraping for knowledge
```

### Skill Invocation Examples

**Writing a new Mangle schema:**

```bash
/skill mangle-programming
# Then write Decl statements, rules with aggregation, etc.
```

**Implementing a new shard:**

```bash
/skill codenerd-builder
/skill go-architect
# Use codenerd-builder for shard pattern, go-architect for Go safety
```

**Debugging a derivation failure:**

```bash
/skill log-analyzer
# Parse logs, query for kernel errors, find root cause
```

**Building the CLI:**

```bash
/skill charm-tui
/skill go-architect
# Use charm-tui for UI components, go-architect for lifecycle
```

**Implementing ResearcherShard knowledge gathering:**

```bash
/skill research-builder
/skill rod-builder
# research-builder for llms.txt parsing, rod-builder for scraping
```

### Cross-Skill Patterns

#### Pattern: Mangle-Governed Go Component

Used throughout codeNERD - Go code that is controlled by Mangle rules.

```go
// go-architect: proper error handling and context
func (k *Kernel) Execute(ctx context.Context, action string) error {
    // mangle-programming: query for permission
    permitted, err := k.Query(ctx, "permitted(?)", action)
    if err != nil {
        // log-analyzer: will capture this error
        logging.KernelError("Permission query failed: %v", err)
        return err
    }
    if !permitted {
        return ErrAccessDenied
    }
    return k.dispatch(ctx, action)
}
```

#### Pattern: Shard with TUI Feedback

```go
// charm-tui: spinner component
// go-architect: goroutine lifecycle
// codenerd-builder: shard execution pattern
func (s *CoderShard) ExecuteWithProgress(ctx context.Context, task string) (string, error) {
    spinner := spinner.New()
    go spinner.Run()  // go-architect: ensure cleanup
    defer spinner.Stop()

    // codenerd-builder: shard execution
    result, err := s.Execute(ctx, task)

    // log-analyzer: will capture timing
    logging.Coder("Task completed: %s", task)
    return result, err
}
```

#### Pattern: Research → Knowledge → Mangle

```go
// research-builder: fetch documentation
// mangle-programming: convert to facts
// codenerd-builder: persist to kernel
func (r *ResearcherShard) IngestKnowledge(ctx context.Context, topic string) error {
    // research-builder: llms.txt pattern
    docs, err := r.FetchLLMSTxt(ctx, topic)
    if err != nil {
        return err
    }

    // mangle-programming: create knowledge atoms
    facts := r.ExtractFacts(docs)

    // codenerd-builder: assert to kernel
    return r.kernel.LoadFacts(facts)
}
```

### Related Skill Documentation

- [mangle-programming/SKILL.md](../mangle-programming/SKILL.md) - Complete Mangle reference with AI failure mode prevention
- [go-architect/SKILL.md](../go-architect/SKILL.md) - Production Go patterns, concurrency safety
- [charm-tui/SKILL.md](../charm-tui/SKILL.md) - Terminal UI with Bubbletea/Lipgloss
- [research-builder/SKILL.md](../research-builder/SKILL.md) - Knowledge ingestion, llms.txt, 4-tier memory
- [rod-builder/SKILL.md](../rod-builder/SKILL.md) - Browser automation with Rod
- [log-analyzer/SKILL.md](../log-analyzer/SKILL.md) - Mangle-based log analysis and debugging
- [prompt-architect/SKILL.md](../prompt-architect/SKILL.md) - Prompt engineering and protocol auditing

## Core Philosophy

Current AI agents make a category error: they ask LLMs to handle everything—creativity AND planning, insight AND memory, problem-solving AND self-correction—when LLMs excel at the former but struggle with the latter. codeNERD separates these concerns through a **Creative-Executive Partnership**:

- **LLM as Creative Center**: The model is the source of problem-solving, solution synthesis, goal-crafting, and insight. It understands problems deeply, generates novel approaches, and crafts creative solutions.
- **Logic as Executive**: Planning, long-term memory, orchestration, skill retention, and safety derive from deterministic Mangle rules. The harness handles what LLMs cannot reliably perform.
- **Transduction Interface**: NL↔Logic atom conversion channels the LLM's creative outputs through formal structure, ensuring creativity flows safely into execution.

This architecture **liberates** the LLM to focus purely on what it does best, while the harness ensures those creative outputs are channeled safely and consistently. The result: creative power and deterministic safety coexist by design.

## Neuro-Symbolic Design Principles

### The "Mangle as HashMap" Anti-Pattern

A critical lesson learned during development: **Mangle is for deduction, not data lookup**.

**The Mistake**: Storing 400+ `intent_definition` facts hoping Mangle would fuzzy-match user input:

```mangle
# WRONG - treating Mangle as a lookup table
intent_definition("review my code", /review, /codebase).
intent_definition("check my code", /review, /codebase).
intent_definition("audit my code", /security, /codebase).
# User says "inspect my code" → NO MATCH (exact strings only!)
```

**Why It Fails**: Mangle performs **exact structural matching**. It has no fuzzy matching, no similarity scoring, no `fn:string_contains` (that function doesn't exist!).

**The Neuro-Symbolic Solution**:

| Layer | Tool | Handles |
|-------|------|---------|
| **Neural (Stochastic)** | Vector embeddings | Fuzzy semantic similarity |
| **Symbolic (Deterministic)** | Mangle rules | Deductive reasoning over matches |

```text
"inspect my code" → Embedding → Vector Search → semantic_match facts → Mangle → /review
```

### The "DSL Trap"

**Root Cause**: Developers treat `.mg` files as general design documents—mixing **Taxonomy** (Data), **Intents** (Configuration), and **Rules** (Logic).

**Mangle is a strict compiler, not a notebook.** It will panic on lines like `Taxonomy: Vehicle > Car`.

| Category | Example | Correct Home |
|----------|---------|--------------|
| **Taxonomy** | `/vehicle > /car` | Mangle facts via Go pre-processor |
| **Intents** | `"I need help" -> /support` | Vector DB (fuzzy matching) |
| **Rules** | `permitted(X) :- safe(X).` | Mangle engine (real logic) |

**Salvage Strategy**: Use a "Split-Brain Loader" in Go that routes content:
- Lines starting with `INTENT:` → Vector DB
- Lines starting with `TAXONOMY:` → Inject as Mangle facts
- Real Mangle code → Pass to compiler

See [go-architect skill](../go-architect/SKILL.md) for the Go implementation pattern.

### When to Use Each Tool

| Need | Wrong Tool | Right Tool |
|------|------------|------------|
| "Does X mean Y?" (semantic) | Mangle facts | Vector embeddings |
| "If X then Y" (deduction) | ML classifier | Mangle rules |
| "All paths from A to B" | Graph traversal code | Mangle transitive closure |
| "Count items by category" | SQL | Mangle aggregation |
| "Find similar documents" | Mangle | Vector search |

### Functions That Don't Exist in Mangle

AI agents frequently hallucinate these. They will silently fail or cause compile errors:

- `fn:string_contains` ❌
- `fn:substring` ❌
- `fn:match` / `fn:regex` ❌
- `fn:like` ❌
- `fn:lower` / `fn:upper` ❌

**Valid built-ins**: `fn:plus`, `fn:minus`, `fn:mult`, `fn:div`, `fn:Count`, `fn:Sum`, `fn:Max`, `fn:Min`, `fn:group_by`, `fn:collect`, `fn:list`, `fn:len`, `fn:concat`, `fn:pair`, `fn:tuple`.

For complete documentation, see [mangle-programming skill Section 11](../mangle-programming/references/150-AI_FAILURE_MODES.md).

### JIT Prompt Compiler

The same neuro-symbolic pattern that powers intent classification applies to **prompt engineering**. Instead of monolithic 20,000-character prompts, the system compiles context-appropriate prompts at runtime from atomic components.

**Architecture:**

```text
CompilationContext (Task + Intent + Language)
    ↓
┌─────────────────────────────────────────┐
│  VECTOR DB (Search Engine)              │
│  - Semantic similarity search           │
│  - Find relevant atomic prompts         │
│  - Returns: candidate atoms             │
└─────────────────────────────────────────┘
    ↓
┌─────────────────────────────────────────┐
│  MANGLE KERNEL (Linker)                 │
│  - Resolve dependencies                 │
│  - Detect conflicts                     │
│  - Apply phase gating                   │
│  - Priority ordering                    │
│  - Derive: selected_prompts/1           │
└─────────────────────────────────────────┘
    ↓
┌─────────────────────────────────────────┐
│  TOKEN BUDGET MANAGER                   │
│  - Allocate budget per category         │
│  - Trim low-priority atoms if needed    │
│  - Ensure mandatory atoms included      │
└─────────────────────────────────────────┘
    ↓
┌─────────────────────────────────────────┐
│  GO RUNTIME (Assembler)                 │
│  - Concatenate selected atoms           │
│  - Inject dynamic context               │
│  - Output: Final prompt string          │
└─────────────────────────────────────────┘
```

**Key Components:**

| Component | File | Responsibility |
|-----------|------|----------------|
| JITPromptCompiler | `internal/prompt/compiler.go` | Main orchestration |
| PromptAtom | `internal/prompt/atoms.go` | Atom type definitions |
| AtomSelector | `internal/prompt/selector.go` | Vector + Mangle selection |
| DependencyResolver | `internal/prompt/resolver.go` | Auto-inject dependencies |
| TokenBudgetManager | `internal/prompt/budget.go` | Budget allocation |
| PromptLoader | `internal/prompt/loader.go` | YAML→SQLite loading |
| PromptAssembler | `internal/articulation/prompt_assembler.go` | Shard integration |

**Key Benefits:**

- **Dependency Resolution**: SQL capability auto-injects safety constraints
- **Conflict Detection**: "verbose" and "concise" atoms can't coexist
- **Phase Gating**: Planning/Coding/Testing phases get different personas
- **Token Efficiency**: Only relevant atoms, not full 20K prompt
- **Context-Aware**: Language/framework atoms selected dynamically
- **Unified Storage**: Prompts stored in agent knowledge DBs

**Storage Architecture (Unified Model):**

| Store | Location | Purpose |
|-------|----------|---------|
| Build-time source | `build/prompt_atoms/**/*.yaml` | 50+ atomic prompt definitions |
| Agent prompts | `.nerd/agents/{name}/prompts.yaml` | Human-editable YAML source |
| Unified DB | `.nerd/shards/{name}_knowledge.db` | SQLite with knowledge_atoms + prompt_atoms |
| Baked-in corpus | Embedded in binary | Default atoms for each shard type |

**Prompt Atom Schema:**

```sql
CREATE TABLE prompt_atoms (
    atom_id TEXT UNIQUE,           -- e.g. "identity/coder/mission"
    category TEXT,                 -- identity, protocol, safety, methodology, etc.
    content TEXT,                  -- Actual prompt text
    token_count INTEGER,

    -- Contextual Selectors (JSON arrays)
    operational_modes TEXT,        -- ["/active", "/debugging"]
    campaign_phases TEXT,          -- ["/planning", "/coding"]
    intent_verbs TEXT,             -- ["/refactor", "/generate"]
    languages TEXT,                -- ["/go", "/python"]
    frameworks TEXT,               -- ["/bubbletea", "/gin"]

    -- Composition
    priority INTEGER DEFAULT 50,
    is_mandatory BOOLEAN,
    depends_on TEXT,               -- JSON array of atom IDs
    conflicts_with TEXT,           -- JSON array of atom IDs

    embedding BLOB                 -- For semantic search
);
```

**Example Atom Categories:**

- `identity/*` - Shard personas (coder, tester, reviewer)
- `protocol/*` - Piggyback protocol, output formatting
- `safety/*` - Constitutional constraints
- `methodology/*` - TDD, debugging strategies
- `language/*` - Go, Python, TypeScript patterns
- `framework/*` - Bubbletea, Rod, Gin specifics
- `hallucination/*` - Anti-patterns for each shard type

For complete specification and God Tier prompt patterns, see [prompt-architect skill](../prompt-architect/SKILL.md).

## Architecture Overview

```text
[ Terminal / User ]
       |
[ Perception Transducer (LLM) ] --> [ Mangle Atoms ]
       |
       +-> [ SemanticClassifier ]
       |     +-> [ Embedded Corpus (intent_corpus.db) ]: Baked-in vectors
       |     +-> [ Learned Corpus (.nerd/learned_corpus.db) ]: Dynamic patterns
       |     +-> [ Embedding Engine (GenAI/Ollama) ]: RETRIEVAL_QUERY
       |
[ Cortex Kernel ]
       |
       +-> [ FactStore (RAM) ]: Working Memory
       +-> [ Mangle Engine ]: Logic CPU
       |     +-> [ DifferentialEngine ]: Incremental Evaluation
       |     +-> semantic_match facts --> selected_verb derivation
       +-> [ JIT Prompt Compiler ]: Dynamic prompt assembly
       |     +-> [ PromptAtomSelector ]: Vector + Mangle selection
       |     +-> [ DependencyResolver ]: Auto-inject safety constraints
       |     +-> [ TokenBudgetManager ]: Context-aware allocation
       |     +-> [ Unified Storage ]: .nerd/shards/*_knowledge.db
       +-> [ Dreamer ]: Precog Safety Simulation
       +-> [ Virtual Store (FFI) ]
             +-> Filesystem Shard
             +-> Vector DB Shard
             +-> MCP/A2A Adapters
             +-> [ Shard Manager ]
                   +-> Type A (Ephemeral): Coder, Tester, Reviewer, Researcher
                   +-> Type B/U (Persistent): User-defined specialists
                   +-> Type S (System): Legislator, Perception, Executive, Router
             +-> [ Autopoiesis ]
                   +-> SafetyChecker (AST + Mangle policy)
                   +-> OuroborosLoop (Tool self-generation)
                   +-> [ Adversarial Co-Evolution ]
                         +-> NemesisShard (Type B adversary)
                         +-> Thunderdome (Battle arena)
                         +-> PanicMaker (Attack generation)
                         +-> Armory (Attack persistence)
       |
[ Articulation Transducer (LLM) ] --> [ User Response ]
       |
       +-> [ PromptAssembler ]: Injects compiled prompts
```

## Implementation Workflow

### 1. Perception Transducer Implementation

The Perception Transducer converts user input into Mangle atoms. Key schemas:

**user_intent** - The seed for all logic:

```mangle
Decl user_intent(
    ID.Type<n>,
    Category.Type<n>,      # /query, /mutation, /instruction
    Verb.Type<n>,          # /explain, /refactor, /debug, /generate
    Target.Type<string>,
    Constraint.Type<string>
).
```

**focus_resolution** - Ground fuzzy references to concrete paths:

```mangle
Decl focus_resolution(
    RawReference.Type<string>,
    ResolvedPath.Type<string>,
    SymbolName.Type<string>,
    Confidence.Type<float>
).

# Clarification threshold - blocks execution if uncertain
clarification_needed(Ref) :-
    focus_resolution(Ref, _, _, Score),
    Score < 0.85.
```

Implementation location: [internal/perception/transducer.go](internal/perception/transducer.go)

### 1.5. Semantic Classification (Neuro-Symbolic Intent)

The Semantic Classifier implements the **neuro-symbolic** approach to intent classification:

- **Vector embeddings** handle fuzzy semantic matching (what Mangle can't do)
- **Mangle rules** handle deductive reasoning over match results (what Mangle excels at)

This replaces the "Mangle as HashMap" anti-pattern where 400+ `intent_definition` facts were used only for documentation generation.

**Architecture:**

```text
User Input: "check my code for security issues"
         ↓
    EMBEDDING LAYER (RETRIEVAL_QUERY via Gemini)
         ↓
    Vector Search (embedded + learned stores)
         ↓
    semantic_match facts → Mangle Kernel
         ↓
    Deductive inference (boosts/overrides)
         ↓
    Selected Verb: /security (confidence: 0.92)
```

**Key Schema (schemas.mg Section 44):**

```mangle
# Semantic match facts injected by SemanticClassifier
Decl semantic_match(UserInput, CanonicalSentence, Verb, Target, Rank, Similarity).
Decl semantic_suggested_verb(Verb, MaxSimilarity).
Decl compound_suggestion(Verb1, Verb2).
```

**Inference Rules (taxonomy.go):**

```mangle
# HIGH-CONFIDENCE OVERRIDE: similarity >= 85 → max score
potential_score(Verb, 100.0) :-
    semantic_match(_, _, Verb, _, 1, Similarity),
    Similarity >= 85.

# MEDIUM-CONFIDENCE BOOST: 70-84 → +30
potential_score(Verb, NewScore) :-
    candidate_intent(Verb, Base),
    semantic_match(_, _, Verb, _, Rank, Similarity),
    Rank <= 3, Similarity >= 70, Similarity < 85,
    NewScore = fn:plus(Base, 30.0).

# LEARNED PATTERN PRIORITY: +40 boost for user-specific patterns
potential_score(Verb, NewScore) :-
    semantic_match(_, Sentence, Verb, _, 1, Similarity),
    Similarity >= 70,
    learned_exemplar(Sentence, Verb, _, _, _),
    candidate_intent(Verb, Base),
    NewScore = fn:plus(Base, 40.0).
```

**Two Vector Stores:**

| Store | Location | Purpose | Mode |
|-------|----------|---------|------|
| Embedded Corpus | `internal/core/defaults/intent_corpus.db` | Baked-in patterns (~1400 entries) | Read-only |
| Learned Corpus | `.nerd/learned_corpus.db` | User-specific patterns | Read-write |

**Corpus Builder** (development-time):

```powershell
$env:CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers"
go run ./cmd/tools/corpus_builder --api-key=$env:GEMINI_API_KEY
```

**Graceful Degradation:** If embedding fails, the system continues with regex-only classification.

**Implementation locations:**

- [internal/perception/semantic_classifier.go](internal/perception/semantic_classifier.go) - Classifier + stores
- [internal/perception/transducer.go](internal/perception/transducer.go) - Integration point
- [internal/core/defaults/schemas.mg](internal/core/defaults/schemas.mg) - Schema declarations (Section 44)
- [internal/perception/taxonomy.go](internal/perception/taxonomy.go) - Inference rules
- [cmd/tools/corpus_builder/main.go](cmd/tools/corpus_builder/main.go) - Build-time corpus generator

For complete specification, see [references/semantic-classification.md](references/semantic-classification.md)

### 2. World Model (EDB) Implementation

The Extensional Database maintains the "Ground Truth" of the codebase:

**file_topology** - Fact-Based Filesystem:

```mangle
Decl file_topology(
    Path.Type<string>,
    Hash.Type<string>,       # SHA-256
    Language.Type<n>,        # /go, /python, /ts
    LastModified.Type<int>,
    IsTestFile.Type<bool>
).
```

**symbol_graph** - AST Projection:

```mangle
Decl symbol_graph(
    SymbolID.Type<string>,
    Type.Type<n>,            # /function, /class, /interface
    Visibility.Type<n>,
    DefinedAt.Type<string>,
    Signature.Type<string>
).

Decl dependency_link(
    CallerID.Type<string>,
    CalleeID.Type<string>,
    ImportPath.Type<string>
).

# Transitive Impact Analysis
impacted(X) :- dependency_link(X, Y, _), modified(Y).
impacted(X) :- dependency_link(X, Z, _), impacted(Z).
```

**diagnostic** - Linter-Logic Bridge:

```mangle
Decl diagnostic(
    Severity.Type<n>,      # /panic, /error, /warning
    FilePath.Type<string>,
    Line.Type<int>,
    ErrorCode.Type<string>,
    Message.Type<string>
).

# The Commit Barrier - blocks git commit if errors exist
block_commit("Build Broken") :-
    diagnostic(/error, _, _, _, _).
```

Implementation locations:

- [internal/world/fs.go](internal/world/fs.go) - Filesystem projection
- [internal/world/ast.go](internal/world/ast.go) - AST projection

### 3. Executive Policy (IDB) Implementation

The Intensional Database contains rules that derive next actions:

**TDD Repair Loop**:

```mangle
next_action(/read_error_log) :-
    test_state(/failing),
    retry_count(N), N < 3.

next_action(/analyze_root_cause) :-
    test_state(/log_read).

next_action(/generate_patch) :-
    test_state(/cause_found).

next_action(/run_tests) :-
    test_state(/patch_applied).

# Surrender after max retries
next_action(/escalate_to_user) :-
    test_state(/failing),
    retry_count(N), N >= 3.
```

**Constitutional Safety**:

```mangle
permitted(Action) :- safe_action(Action).

permitted(Action) :-
    dangerous_action(Action),
    admin_override(User),
    signed_approval(Action).

dangerous_action(Action) :-
    action_type(Action, /exec_cmd),
    cmd_string(Action, Cmd),
    fn:string_contains(Cmd, "rm").
```

Implementation location: [internal/core/kernel.go](internal/core/kernel.go)

### 4. Virtual Predicates (FFI) Implementation

Virtual Predicates abstract external APIs into logic:

```go
// In virtual_store.go
func (s *VirtualStore) GetFacts(pred ast.PredicateSym) []ast.Atom {
    switch pred.Symbol {
    case "mcp_tool_result":
        return s.callMCPTool(pred)
    case "file_content":
        return s.readFileContent(pred)
    case "shell_exec_result":
        return s.executeShell(pred)
    default:
        return s.MemStore.GetFacts(pred)
    }
}
```

Implementation location: [internal/core/virtual_store.go](internal/core/virtual_store.go)

### 5. ShardAgent Implementation

ShardAgents are modular execution units for parallel task execution:

**Shard Types**:

- **Type A (Ephemeral)**: `ShardTypeEphemeral` - Spawn → Execute → Die. RAM only. Used for `/review`, `/test`, `/fix`.
- **Type B (Persistent)**: `ShardTypePersistent` - Pre-populated with domain knowledge. SQLite-backed.
- **Type U (User)**: `ShardTypeUser` - User-defined specialists via `/define-agent` wizard.
- **Type S (System)**: `ShardTypeSystem` - Long-running services. Auto-start.

**Built-in System Shards (Type S)**:

- `legislator` - Translates feedback into Mangle rules
- `perception_firewall` - NL → atoms transduction
- `executive_policy` - `next_action` derivation
- `world_model_ingestor` - File topology, symbol graph maintenance
- `tactile_router` - Action → tool routing
- `requirements_interrogator` - Socratic clarification (via `/clarify`)

**Modular Shard Structure** (subdirectories):

```text
internal/shards/
├── coder/          # apply.go, autopoiesis.go, build.go, coder.go, context.go, facts.go, generation.go
├── tester/         # Modularized test execution
├── reviewer/       # checks.go, custom_rules.go, dependencies.go, metrics.go
├── researcher/     # analyze.go, extract.go, scraper.go, tools.go
├── nemesis/        # nemesis.go, armory.go, attack_runner.go (Adversarial Co-Evolution)
├── tool_generator/ # Ouroboros tool generation
└── system/         # base.go, constitution.go, executive.go, legislator.go, perception.go, planner.go
```

```mangle
Decl delegate_task(
    ShardType.Type<n>,
    TaskDescription.Type<string>,
    Result.Type<string>
).

Decl shard_lifecycle(
    ShardID.Type<n>,
    ShardType.Type<n>,       # /generalist, /specialist
    MountStrategy.Type<n>,   # /ram, /sqlite
    KnowledgeBase.Type<string>,
    Permissions.Type<string>
).
```

Implementation locations:

- [internal/core/shard_manager.go](internal/core/shard_manager.go)
- [internal/shards/coder.go](internal/shards/coder.go)
- [internal/shards/tester.go](internal/shards/tester.go)
- [internal/shards/reviewer.go](internal/shards/reviewer.go)
- [internal/shards/researcher.go](internal/shards/researcher.go)
- [internal/shards/nemesis/nemesis.go](internal/shards/nemesis/nemesis.go)

### 6. Piggyback Protocol Implementation

The **Piggyback Protocol** is the Corpus Callosum of the Neuro-Symbolic architecture—the invisible bridge between what the agent says and what it truly believes.

**The Dual-Channel Architecture:**

- **Surface Stream**: Natural language for the user (visible)
- **Control Stream**: Logic atoms for the kernel (hidden)

```json
{
  "surface_response": "I fixed the authentication bug.",
  "control_packet": {
    "intent_classification": {
      "category": "mutation",
      "confidence": 0.95
    },
    "mangle_updates": [
      "task_status(/auth_fix, /complete)",
      "file_state(/auth.go, /modified)",
      "diagnostic(/error, \"auth.go\", 42, \"E001\", \"fixed\")"
    ],
    "memory_operations": [
      { "op": "promote_to_long_term", "key": "preference:code_style", "value": "concise" }
    ],
    "self_correction": null
  }
}
```

**Key Capabilities:**

1. **Semantic Compression** - Chat history compressed to atoms (>100:1 ratio)
2. **Constitutional Override** - Kernel can block/rewrite unsafe surface responses
3. **Grammar-Constrained Decoding** - Forces valid JSON at inference level
4. **Self-Correction** - Abductive hypotheses trigger automatic recovery

**The Hidden Injection:**

```text
CRITICAL PROTOCOL:
You must NEVER output raw text. You must ALWAYS output a JSON object
containing surface_response and control_packet.
Your control_packet must reflect the true state of the world, even if the
surface_response is polite.
```

For complete specification, see [references/piggyback-protocol.md](references/piggyback-protocol.md)

Implementation location: [internal/articulation/emitter.go](internal/articulation/emitter.go)

### 7. Spreading Activation (Context Selection)

Replace vector RAG with logic-directed context:

```mangle
# Base Activation (Recency)
activation(Fact, 100) :- new_fact(Fact).

# Spreading Activation (Dependency)
activation(Tool, 80) :-
    active_goal(Goal),
    tool_capabilities(Tool, Cap),
    goal_requires(Goal, Cap).

# Recursive Spread
activation(FileB, 50) :-
    activation(FileA, Score),
    Score > 40,
    dependency_link(FileA, FileB, _).

# Context Pruning
context_atom(Fact) :-
    activation(Fact, Score),
    Score > 30.
```

## Dream State and Precog Safety

### Dream State (`/dream` command)

The Dream State is a multi-agent simulation/learning mode that consults ALL available shards in parallel WITHOUT executing anything. Used for "what if" scenarios, hypothetical planning, and learning.

**Trigger:** Intent verb `/dream` or phrases like "what if", "imagine", "hypothetically"

**Implementation:**

```go
// cmd/nerd/chat/process.go

// DreamConsultation holds a shard's perspective on a hypothetical task.
type DreamConsultation struct {
    ShardName   string // e.g., "coder", "my-go-expert"
    ShardType   string // e.g., "ephemeral", "persistent", "system"
    Perspective string
    Error       error
}

// SessionContext includes DreamMode flag
type SessionContext struct {
    DreamMode bool // When true, shard should ONLY describe what it would do, not execute
    // ...
}
```

**Workflow:**

1. Parse intent as `/dream` verb
2. Query ShardManager for ALL available shards (Type A, B, U, and selected Type S)
3. Spawn each shard with `DreamMode: true` in context
4. Aggregate `DreamConsultation` results
5. Assert `dream_state(Hypothetical, Timestamp)` fact to kernel for learning

Implementation location: [cmd/nerd/chat/process.go:607](cmd/nerd/chat/process.go#L607)

### Dreamer (Precog Safety System)

The Dreamer simulates the impact of actions BEFORE execution using sandboxed kernel cloning. It projects effects and checks for `panic_state` derivations to prevent catastrophic actions.

**Key Components:**

```go
// internal/core/dreamer.go

type Dreamer struct {
    kernel *RealKernel
}

type DreamResult struct {
    ActionID       string
    Request        ActionRequest
    ProjectedFacts []Fact
    Unsafe         bool
    Reason         string
}

type DreamCache struct {
    mu      sync.RWMutex
    results map[string]DreamResult
}
```

**Simulation Flow:**

1. `SimulateAction(ctx, ActionRequest)` called before execution
2. `projectEffects()` generates projected facts based on action type:
   - `/file_missing`, `/modified`, `/file_exists` for file ops
   - `/critical_path_hit` for protected directories
   - `/exec_cmd`, `/exec_danger` for shell commands
   - `/touches_symbol`, `/impacts_test` from code graph
3. `evaluateProjection()` clones kernel, asserts projections, evaluates
4. Query `panic_state` - if derived, action is blocked

**Critical Paths Protected:**

- `.git`, `.nerd`, `internal/mangle`, `internal/core`, `cmd/nerd`

**Dangerous Commands Detected:**

- `rm -rf`, `rm -r`, `git reset --hard`, `terraform destroy`, `dd if=`

Implementation location: [internal/core/dreamer.go](internal/core/dreamer.go)

## Autopoiesis System (Refactored)

The autopoiesis system has been significantly refactored into a production-ready architecture.

### SafetyChecker (AST + Mangle Policy)

Validates generated tool code using embedded Mangle policy (`go_safety.mg`):

```go
// internal/autopoiesis/checker.go

type SafetyChecker struct {
    config      OuroborosConfig
    policy      string          // Embedded go_safety.mg
    allowedPkgs []string
}

type SafetyReport struct {
    Safe           bool
    Violations     []SafetyViolation
    ImportsChecked int
    CallsChecked   int
    Score          float64 // 0.0 = unsafe, 1.0 = perfectly safe
}

type ViolationType int
const (
    ViolationForbiddenImport ViolationType = iota
    ViolationDangerousCall
    ViolationUnsafePointer
    ViolationReflection
    ViolationCGO
    ViolationExec
    ViolationPanic
    ViolationGoroutineLeak
    ViolationParseError
    ViolationPolicy
)
```

**AST Fact Extraction:**

- `ast_import(File, Package)` - Import statements
- `ast_call(Function, Callee)` - Function calls
- `ast_goroutine_spawn(Target, Line)` - Goroutine spawns
- `ast_uses_context_cancellation(Line)` - Context usage in goroutines

Implementation location: [internal/autopoiesis/checker.go](internal/autopoiesis/checker.go)

### Ouroboros Loop (Transactional State Machine)

The Ouroboros Loop has been rewritten as a Mangle-governed transactional state machine:

**4-Phase Protocol:**

1. **Proposal** - Generate & sanitize tool code (with retry feedback if previous failed)
2. **Audit** - Safety check with retry loop on violations
3. **Simulation** - DifferentialEngine analysis & transition validation
4. **Commit** - Compile, register & hot-reload

```go
// internal/autopoiesis/ouroboros.go

type OuroborosLoop struct {
    toolGen       *ToolGenerator
    safetyChecker *SafetyChecker
    compiler      *ToolCompiler
    registry      *RuntimeRegistry
    sanitizer     *transpiler.Sanitizer
    engine        *mangle.Engine  // Mangle governs the loop
    config        OuroborosConfig
}

type LoopStage int
const (
    StageDetection LoopStage = iota
    StageSpecification
    StageSafetyCheck
    StageCompilation
    StageRegistration
    StageExecution
    StageComplete
    StageSimulation
    StagePanic
)
```

**State Machine Rules** (`internal/autopoiesis/state.mg`):

```mangle
# Stability transition - must not degrade
valid_transition(Next) :-
    state(Curr, _, _),
    proposed(Next),
    effective_stability(Curr, CurrEff),
    effective_stability(Next, NextEff),
    NextEff >= CurrEff.

# Halting Oracle - detect stagnation
stagnation_detected() :-
    history(StepA, Hash),
    history(StepB, Hash),
    StepA != StepB.

# Termination conditions
should_halt(StepID) :- max_iterations_exceeded(StepID).
should_halt(StepID) :- max_retries_exceeded(StepID).
should_halt(StepID) :- stagnation_detected().
should_halt(StepID) :- stability_degrading(StepID).
```

**Stability Penalties:**

- Panic: -0.2 stability
- Retry (>=2 attempts): -0.1 stability
- Both: -0.3 stability

Implementation locations:

- [internal/autopoiesis/ouroboros.go](internal/autopoiesis/ouroboros.go)
- [internal/autopoiesis/state.mg](internal/autopoiesis/state.mg)

## Adversarial Co-Evolution System

The Adversarial Co-Evolution System is codeNERD's immune system—a sophisticated battle-testing infrastructure that actively tries to break generated tools and patches before they reach production. It embodies the principle: "Code that survives Nemesis survives reality."

### Adversarial Architecture

```text
┌─────────────────────────────────────────────────────────────────────────────┐
│                    Adversarial Co-Evolution System                          │
├─────────────────────────────────────────────────────────────────────────────┤
│   ┌─────────────┐    generates     ┌─────────────────┐                     │
│   │ PanicMaker  │───────attacks───▶│   Thunderdome   │                     │
│   │  (LLM-based)│                  │  (Battle Arena) │                     │
│   └─────────────┘                  └────────┬────────┘                     │
│         ▲                           survives│fails                         │
│         │                                   ▼                              │
│   ┌─────────────┐    analyzes      ┌─────────────────┐                     │
│   │   Nemesis   │◀────patches──────│   CoderShard    │                     │
│   │ (Adversary) │                  │   (Creates)     │                     │
│   └──────┬──────┘                  └─────────────────┘                     │
│          │ successful attacks                                              │
│          ▼                                                                 │
│   ┌─────────────┐    regression    ┌─────────────────┐                     │
│   │   Armory    │───────tests─────▶│  Future Builds  │                     │
│   └─────────────┘                  └─────────────────┘                     │
└─────────────────────────────────────────────────────────────────────────────┘
```

### NemesisShard (Type B Persistent Adversary)

The NemesisShard is a **persistent adversarial specialist** that opposes the CoderShard. While Coder creates, Nemesis destroys—ensuring only robust code survives.

**Task Formats:**

- `analyze:<patch_id>` - Analyze a patch for weaknesses
- `gauntlet:<patch_id>` - Run full adversarial gauntlet
- `review:<target>` - Adversarial review of a target file
- `anti_autopoiesis:<patch_id>` - Detect lazy fix patterns

```go
// internal/shards/nemesis/nemesis.go
type NemesisShard struct {
    id          string
    config      core.ShardConfig
    kernel      *core.RealKernel
    llmClient   LLMClient
    armory      *Armory           // Persisted attack tools
    vulnDB      *VulnerabilityDB  // Tracks discovered weaknesses
    thunderdome *autopoiesis.Thunderdome
}
```

Implementation: [internal/shards/nemesis/nemesis.go](internal/shards/nemesis/nemesis.go)

### Thunderdome (Battle Arena)

The Thunderdome runs attack vectors against compiled tools in isolated sandboxes with memory/timeout limits.

```go
// internal/autopoiesis/thunderdome.go
type ThunderdomeConfig struct {
    Timeout         time.Duration  // Max time per attack (default: 5s)
    MaxMemoryMB     int            // Memory limit (default: 100MB)
    ParallelAttacks int            // Concurrent attacks (default: 1)
}
```

**Battle Flow:** Prepare Arena → Generate Harness → Compile → Run Attacks → Analyze → Fail Fast

Implementation: [internal/autopoiesis/thunderdome.go](internal/autopoiesis/thunderdome.go)

### PanicMaker (Attack Vector Generator)

Analyzes tool source code and generates targeted attacks using LLM analysis.

**Attack Categories:**

| Category | Description | Expected Failure |
|----------|-------------|------------------|
| `nil_pointer` | Pass nil where non-nil expected | panic |
| `boundary` | Max int, empty slices, negative indices | panic |
| `resource` | Memory exhaustion, huge allocations | OOM |
| `concurrency` | Race conditions, deadlocks | race/deadlock |
| `format` | Invalid UTF-8, special chars, injection | panic |

Implementation: [internal/autopoiesis/panic_maker.go](internal/autopoiesis/panic_maker.go)

### Armory (Attack Persistence)

Persists successful attacks as regression tests. "Just as codeNERD learns preferences, Nemesis learns weaknesses."

```go
// internal/shards/nemesis/armory.go
type ArmoryAttack struct {
    ID            string    // Unique attack ID
    Name          string    // Attack name
    Category      string    // concurrency, resource, logic, integration
    Vulnerability string    // What invariant it violates
    SuccessCount  int       // How many bugs it's found
    LastSuccess   time.Time // Last successful break
}
```

**Rules:** Add on break → Update on success → Prune stale (30 days) → Protect effective (3+ successes)

Implementation: [internal/shards/nemesis/armory.go](internal/shards/nemesis/armory.go)

### AttackRunner (Sandboxed Execution)

Executes attack scripts in isolated Go test environments with race detection (`-race`).

Implementation: [internal/shards/nemesis/attack_runner.go](internal/shards/nemesis/attack_runner.go)

### Chaos Schema (chaos.mg)

Mangle predicates for adversarial tracking:

```mangle
# Attack tracking
Decl attack_vector(AttackID, Name, Category, ToolName).
Decl panic_maker_verdict(ToolName, Verdict, Timestamp).  # /survived or /defeated
Decl battle_hardened(ToolName, Timestamp).

# Nemesis tracking
Decl nemesis_victory(PatchID).  # Nemesis broke the patch

# System invariants
Decl system_invariant_violated(InvariantID, Timestamp).

# Lazy pattern detection (anti-autopoiesis)
lazy_pattern_detected(/timeout_lazy, /timeout_increase) :-
    fix_pattern(_, /timeout_increase, Count, _), Count >= 3.
```

**System Invariants:** HTTP 500 rate, deadlock, memory, goroutine leak, latency (p99)

Implementation: [internal/autopoiesis/chaos.mg](internal/autopoiesis/chaos.mg)

### Adversarial Component Locations

| Component | File | Purpose |
|-----------|------|---------|
| NemesisShard | `internal/shards/nemesis/nemesis.go` | Adversarial specialist |
| Armory | `internal/shards/nemesis/armory.go` | Attack persistence |
| AttackRunner | `internal/shards/nemesis/attack_runner.go` | Sandboxed execution |
| Thunderdome | `internal/autopoiesis/thunderdome.go` | Battle arena |
| PanicMaker | `internal/autopoiesis/panic_maker.go` | Attack generation |
| Chaos Schema | `internal/autopoiesis/chaos.mg` | Mangle rules |

### DifferentialEngine (Incremental Evaluation)

Optimizes Mangle evaluation for growing world models:

```go
// internal/mangle/differential.go

type DifferentialEngine struct {
    baseEngine   *Engine
    programInfo  *analysis.ProgramInfo
    strataStores []*KnowledgeGraph  // Store per stratum
    predStratum  map[ast.PredicateSym]int
    strataRules  [][]ast.Clause
}
```

**Features:**

- **Stratum-Aware Caching** - EDB (stratum 0) vs IDB (stratum 1+)
- **Delta Propagation** - Only re-evaluate affected strata
- **Snapshot Isolation (COW)** - Concurrent simulation branches
- **ChainedFactStore** - Layered queries across strata
- **Virtual Predicate Lazy Loading** - On-demand fact loading

```go
// Incremental fact addition
diffEngine.AddFactIncremental(mangle.Fact{
    Predicate: "state",
    Args:      []interface{}{stepID, stability, loc},
})

// Snapshot for simulation
snapshot := diffEngine.Snapshot()
```

Implementation location: [internal/mangle/differential.go](internal/mangle/differential.go)

### Mangle Generation Feedback Loop

When LLMs generate Mangle rules (for autopoiesis, legislator proposals, or constitution self-improvement), the generated code frequently contains syntax errors. The Feedback Loop system provides automatic validation, error classification, and retry with structured feedback.

**Architecture:**

```text
┌──────────────────────────────────────────────────────────────────────┐
│                        MangleFeedbackLoop                             │
│                                                                       │
│  LLM Output ──▶ Sanitizer ──▶ PreValidator ──▶ HotLoadRule (sandbox) │
│                 (auto-fix)    (regex checks)    (Mangle compile)     │
│                     │              │                  │               │
│                     └──────────────┴──────────────────┘               │
│                                    │                                  │
│                        ┌───────────┴───────────┐                      │
│                        ▼                       ▼                      │
│                    SUCCESS                 FAILURE                    │
│                        │                       │                      │
│                        │     ErrorClassifier → FeedbackPrompt         │
│                        │                       │                      │
│                        │     CostGuard.CanRetry? ──▶ RETRY            │
│                        ▼                       ▼                      │
│                     RETURN                   FAIL                     │
└──────────────────────────────────────────────────────────────────────┘
```

**Key Components:**

| Component | Location | Purpose |
|-----------|----------|---------|
| FeedbackLoop | [internal/mangle/feedback/loop.go](internal/mangle/feedback/loop.go) | Main orchestrator |
| PreValidator | [internal/mangle/feedback/pre_validator.go](internal/mangle/feedback/pre_validator.go) | Fast regex-based AI error detection |
| ErrorClassifier | [internal/mangle/feedback/error_classifier.go](internal/mangle/feedback/error_classifier.go) | Parses Mangle compiler errors |
| PromptBuilder | [internal/mangle/feedback/prompt_builder.go](internal/mangle/feedback/prompt_builder.go) | Progressive feedback prompts |
| ValidationBudget | [internal/shards/system/base.go](internal/shards/system/base.go) | CostGuard rate limiting for retries |

**Common AI Error Patterns Detected:**

- **Atom/String confusion**: Detects `"active"` and converts to `/active` - Auto-repairable (Sanitizer)
- **Prolog negation**: Detects `\+` via regex and converts to Mangle negation - Auto-repairable
- **Missing period**: Detects `:-` without terminating `.` - Auto-repairable
- **Aggregation syntax**: Detects SQL-style aggregation - Auto-repairable (Sanitizer)
- **Unbound negation**: Binding analysis - Partially auto-repairable
- **Undeclared predicate**: SchemaValidator - Not auto-repairable (feedback only)
- **Stratification**: Mangle analysis - Not auto-repairable (feedback only)

**Progressive Retry Strategy:**

| Attempt | Feedback Content |
|---------|------------------|
| 1 | Original prompt + syntax reminders |
| 2 | Original + specific error + WRONG/CORRECT example |
| 3 | Original + all errors + "ONLY use these predicates" + simplify hint |

**Integration Points:**

```go
// Executive shard autopoiesis (internal/shards/system/executive.go)
result, err := e.feedbackLoop.GenerateAndValidate(
    ctx,
    llmAdapter,
    e.Kernel,  // Kernel satisfies RuleValidator interface
    executiveAutopoiesisPrompt,
    userPrompt,
    "executive",  // Domain for valid examples
)

// Constitution shard (internal/shards/system/constitution.go)
result, err := c.feedbackLoop.GenerateAndValidate(ctx, llmAdapter, c.Kernel,
    constitutionAutopoiesisPrompt, userPrompt, "constitution")

// Legislator shard (internal/shards/system/legislator.go)
result, err := l.feedbackLoop.GenerateAndValidate(ctx, adapter, l.Kernel,
    legislatorSystemPrompt, buildLegislatorPrompt(directive), "legislator")
```

**RuleValidator Interface:**

The Kernel satisfies this interface for sandbox validation:

```go
type RuleValidator interface {
    HotLoadRule(rule string) error      // Sandbox compile check
    GetDeclaredPredicates() []string    // Available predicates for feedback
}
```

**ValidationBudget in CostGuard:**

```go
// internal/shards/system/base.go
type CostGuard struct {
    // ... existing fields ...
    MaxValidationRetries  int // Default: 3 per rule
    ValidationBudget      int // Default: 20 per session
    validationRetriesUsed int
}

// Methods
func (g *CostGuard) CanRetryValidation() (bool, string)
func (g *CostGuard) RecordValidationRetry()
func (g *CostGuard) ResetValidationBudget()
func (g *CostGuard) ValidationStats() (used, budget int)
```

**Success Metrics:**

- Target: Reduce "rule rejected by sandbox" errors by 70%
- Average retries needed: < 1.5 per rule generation
- Pre-validator catches ~80% of errors before compilation
- Target success rate: 40% → 85%

## Logging System

codeNERD uses a config-driven categorized logging system for debugging and diagnostics. Logging is completely disabled in production mode for zero overhead.

### Configuration

Logging is configured in `.nerd/config.json`:

```json
{
  "logging": {
    "level": "info",
    "format": "text",
    "debug_mode": true,
    "categories": {
      "boot": true,
      "session": true,
      "kernel": true,
      "api": true,
      "perception": true,
      "articulation": true,
      "routing": true,
      "tools": true,
      "virtual_store": true,
      "shards": true,
      "coder": true,
      "tester": true,
      "reviewer": true,
      "researcher": true,
      "system_shards": true,
      "dream": true,
      "autopoiesis": true,
      "campaign": true,
      "context": true,
      "world": true,
      "embedding": true,
      "store": true
    }
  }
}
```

**Key Settings:**

- `debug_mode`: Master toggle. `false` = production mode (no logging, zero overhead)
- `categories`: Per-category toggles. Omitted categories default to enabled in debug mode
- `level`: Minimum level (`debug`, `info`, `warn`, `error`)

### The 22 Log Categories

| Category | System | Key Events |
|----------|--------|------------|
| `boot` | Initialization | Config loading, startup sequence |
| `session` | Session Management | Turn processing, persistence |
| `kernel` | Mangle Engine | Fact assertion, rule derivation, queries |
| `api` | LLM Calls | Requests, responses, token counts |
| `perception` | Transducer | NL parsing, intent extraction, atom generation |
| `articulation` | Emitter | Response generation, Piggyback protocol |
| `routing` | Action Router | Tool selection, delegation decisions |
| `tools` | Tool Execution | Invocations, results, errors |
| `virtual_store` | FFI Layer | External API calls, fact loading |
| `shards` | Shard Manager | Spawn, execute, destroy lifecycle |
| `coder` | CoderShard | Code generation, edits, Ouroboros routing |
| `tester` | TesterShard | Test execution, coverage analysis |
| `reviewer` | ReviewerShard | Code review, security checks |
| `researcher` | ResearcherShard | Knowledge gathering, extraction |
| `system_shards` | System Shards | Legislator, Constitution, Executive |
| `dream` | Dream State | What-if simulations, Precog safety |
| `autopoiesis` | Self-Improvement | Learning patterns, Ouroboros loop |
| `campaign` | Campaign System | Multi-phase orchestration |
| `context` | Context Compression | Window management, token budgets |
| `world` | World Scanner | File topology, AST projection |
| `embedding` | Vector Operations | Embedding generation, similarity |
| `store` | Memory Tiers | CRUD across RAM/Vector/Graph/Cold |

### Usage in Go Code

```go
import "codenerd/internal/logging"

// Initialize at startup (in cmd/nerd/main.go)
logging.Initialize(workspacePath)
defer logging.CloseAll()

// Category-specific logging
logging.Kernel("Asserting fact: %s", factString)
logging.KernelDebug("Detailed derivation: %v", result)

// Get logger instance for structured logging
logger := logging.Get(logging.CategoryShards)
logger.Info("Spawning shard: %s", shardID)
logger.Warn("Shard execution slow: %v", duration)
logger.Error("Shard failed: %v", err)

// Context logging (key-value pairs)
ctx := logger.WithContext(map[string]interface{}{
    "shard_id": shardID,
    "task":     taskName,
})
ctx.Info("Starting execution")

// Performance timing
timer := logging.StartTimer(logging.CategoryKernel, "Query evaluation")
// ... operation ...
timer.Stop()  // Logs: "Query evaluation completed in 45ms"

// With threshold warning
timer.StopWithThreshold(100 * time.Millisecond)  // Warns if >100ms
```

### Log File Structure

Logs are written to `.nerd/logs/` with date-prefixed category files:

```
.nerd/logs/
├── 2025-12-08_kernel.log
├── 2025-12-08_shards.log
├── 2025-12-08_perception.log
├── 2025-12-08_articulation.log
└── ...
```

**Log Format:**

```
2025/12/08 10:30:45.123456 [INFO] Asserting fact: user_intent(/id1, /query, /read, "foo", _)
2025/12/08 10:30:45.124000 [DEBUG] Derived 15 facts from rule next_action
2025/12/08 10:30:45.125000 [WARN] Query took 2.3s (threshold: 1s)
2025/12/08 10:30:45.126000 [ERROR] Failed to derive permitted(X): no matching rules
```

### Integration with log-analyzer Skill

The logging system integrates with the `log-analyzer` skill for Mangle-based debugging:

```bash
# 1. Enable debug mode in config
# .nerd/config.json: "debug_mode": true

# 2. Run codeNERD session
./nerd chat

# 3. Parse logs to Mangle facts
python .claude/skills/log-analyzer/scripts/parse_log.py .nerd/logs/*.log > session.mg

# 4. Analyze with Mangle queries
python .claude/skills/log-analyzer/scripts/analyze_logs.py session.mg --builtin errors
python .claude/skills/log-analyzer/scripts/analyze_logs.py session.mg --builtin root_cause
```

**Example Mangle queries for debugging:**

```mangle
# Find all errors
?error_entry(Time, Category, Message).

# Error count by category
?error_count(Category, Count).

# Error context (what happened before each error)
?error_context(ErrorTime, ErrorCat, PriorTime, PriorCat, PriorMsg).

# Cross-category correlations
?correlated(Time1, Cat1, Time2, Cat2).
```

For complete Mangle patterns, see the `log-analyzer` skill at [.claude/skills/log-analyzer/SKILL.md](.claude/skills/log-analyzer/SKILL.md).

Implementation location: [internal/logging/logger.go](internal/logging/logger.go)

## Key Implementation Patterns

### Pattern 1: The Hallucination Firewall

Every action must be logically permitted:

```go
func (k *Kernel) Execute(action Action) error {
    if !k.Mangle.Query("permitted(?)", action.Name) {
        return ErrAccessDenied
    }
    return k.VirtualStore.Dispatch(action)
}
```

### Pattern 2: Grammar-Constrained Decoding

Force valid Mangle syntax output:

- Use structured output schemas
- Implement repair loops for malformed atoms
- Validate against Mangle EBNF before committing

### Pattern 3: The OODA Loop

```
Observe -> Orient -> Decide -> Act
   |          |         |        |
   v          v         v        v
Transducer  Spreading  Mangle   Virtual
 (LLM)     Activation  Engine    Store
```

### Pattern 4: Autopoiesis (Self-Learning)

The system learns from user interactions without retraining:

**Rejection Tracking:**

```go
// In shard execution
if err := c.applyEdits(ctx, result.Edits); err != nil {
    c.trackRejection(coderTask.Action, err.Error())  // Track pattern
    return "", err
}
c.trackAcceptance(coderTask.Action)  // Track success
```

**Mangle Pattern Detection:**

```mangle
# 3 rejections = pattern signal
preference_signal(Pattern) :-
    rejection_count(Pattern, N), N >= 3.

# Promote to long-term memory
promote_to_long_term(FactType, FactValue) :-
    preference_signal(Pattern),
    derived_rule(Pattern, FactType, FactValue).
```

**The Ouroboros Loop (Tool Self-Generation):**

```mangle
# Detect missing capability
missing_tool_for(IntentID, Cap) :-
    user_intent(IntentID, _, _, _, _),
    goal_requires(_, Cap),
    !has_capability(Cap).

# Trigger tool generation
next_action(/generate_tool) :-
    missing_tool_for(_, _).
```

For complete specification, see [references/autopoiesis.md](references/autopoiesis.md)

## Mangle Logic Files

Core logic files have been reorganized to `internal/core/defaults/`:

**Default Policy Files:**

- [internal/core/defaults/schemas.mg](internal/core/defaults/schemas.mg) - Core schema declarations (78KB)
- [internal/core/defaults/policy.mg](internal/core/defaults/policy.mg) - Constitutional rules (81KB)
- [internal/core/defaults/coder.mg](internal/core/defaults/coder.mg) - Coder shard logic (35KB)
- [internal/core/defaults/tester.mg](internal/core/defaults/tester.mg) - Tester shard logic
- [internal/core/defaults/reviewer.mg](internal/core/defaults/reviewer.mg) - Reviewer shard logic
- [internal/core/defaults/campaign_rules.mg](internal/core/defaults/campaign_rules.mg) - Campaign orchestration (33KB)
- [internal/core/defaults/inference.mg](internal/core/defaults/inference.mg) - Inference rules
- [internal/core/defaults/taxonomy.mg](internal/core/defaults/taxonomy.mg) - Category taxonomies
- [internal/core/defaults/schema/intent.mg](internal/core/defaults/schema/intent.mg) - Intent schema (1.7MB)

**Autopoiesis Logic:**

- [internal/autopoiesis/state.mg](internal/autopoiesis/state.mg) - Ouroboros state machine rules
- [internal/autopoiesis/go_safety.mg](internal/autopoiesis/go_safety.mg) - Go code safety policy

**User Overrides (`.nerd/` directory):**

- `.nerd/mangle/extensions.mg` - User-defined schema extensions
- `.nerd/mangle/policy_overrides.mg` - Custom policy rules
- `.nerd/profile.mg` - User preferences

## 8. Campaign Orchestration (Multi-Phase Goals)

Campaigns handle long-running, multi-phase goal execution:

**Campaign Types**:

- `/greenfield` - Build from scratch
- `/feature` - Add major feature
- `/audit` - Stability/security audit
- `/migration` - Technology migration
- `/remediation` - Fix issues across codebase

**The Decomposer** - LLM + Mangle collaboration:

1. Ingest source documents (specs, requirements)
2. Extract requirements via LLM
3. Propose phases and tasks
4. Validate via Mangle (circular deps, unreachable tasks)
5. Refine if issues found
6. Link requirements to tasks

**Context Pager** - Phase-aware context management:

```go
// Budget allocation
totalBudget:     100000 // 100k tokens
coreReserve:     5000   // 5% - core facts
phaseReserve:    30000  // 30% - current phase
historyReserve:  15000  // 15% - compressed history
workingReserve:  40000  // 40% - working memory
prefetchReserve: 10000  // 10% - upcoming tasks
```

**Campaign Policy Rules** ([internal/mangle/policy.gl:479](internal/mangle/policy.gl#L479)):

```mangle
# Phase eligibility - all hard deps complete
phase_eligible(PhaseID) :-
    campaign_phase(PhaseID, CampaignID, _, _, /pending, _),
    current_campaign(CampaignID),
    !has_incomplete_hard_dep(PhaseID).

# Next task - highest priority without blockers
next_campaign_task(TaskID) :-
    current_phase(PhaseID),
    campaign_task(TaskID, PhaseID, _, /pending, _),
    !has_blocking_task_dep(TaskID).

# Replan on cascade failures
replan_needed(CampaignID, "task_failure_cascade") :-
    failed_campaign_task(CampaignID, TaskID1),
    failed_campaign_task(CampaignID, TaskID2),
    failed_campaign_task(CampaignID, TaskID3),
    TaskID1 != TaskID2, TaskID2 != TaskID3, TaskID1 != TaskID3.
```

Implementation locations:

- [internal/campaign/orchestrator.go](internal/campaign/orchestrator.go) - Main orchestration loop
- [internal/campaign/decomposer.go](internal/campaign/decomposer.go) - Plan decomposition
- [internal/campaign/context_pager.go](internal/campaign/context_pager.go) - Context management
- [internal/campaign/types.go](internal/campaign/types.go) - All campaign types with ToFacts()

## 9. Actual Kernel Implementation

The kernel is implemented in [internal/core/kernel.go](internal/core/kernel.go):

```go
type RealKernel struct {
    mu          sync.RWMutex
    facts       []Fact
    store       factstore.FactStore
    programInfo *analysis.ProgramInfo
    schemas     string  // From schemas.gl
    policy      string  // From policy.gl
}

// Key methods:
// - LoadFacts(facts []Fact) - Add to EDB and rebuild
// - Query(predicate string) - Query derived facts
// - Assert(fact Fact) - Add single fact dynamically
// - Retract(predicate string) - Remove all facts of predicate
```

**Fact struct with ToAtom()**:

```go
type Fact struct {
    Predicate string
    Args      []interface{}
}

func (f Fact) ToAtom() (ast.Atom, error) {
    // Converts Go types to Mangle AST terms
    // Handles: strings, name constants (/foo), ints, floats, bools
}
```

## 10. Shard Implementation Pattern

Each shard follows this pattern (see [internal/shards/coder.go](internal/shards/coder.go)):

```go
type CoderShard struct {
    id           string
    config       core.ShardConfig
    state        core.ShardState
    kernel       *core.RealKernel      // Own kernel instance
    llmClient    perception.LLMClient
    virtualStore *core.VirtualStore

    // Autopoiesis tracking
    rejectionCount  map[string]int
    acceptanceCount map[string]int
}

func (c *CoderShard) Execute(ctx context.Context, task string) (string, error) {
    // 1. Load shard-specific policy
    c.kernel.LoadPolicyFile("coder.gl")

    // 2. Parse task into structured form
    coderTask := c.parseTask(task)

    // 3. Assert task facts to kernel
    c.assertTaskFacts(coderTask)

    // 4. Check impact via Mangle query
    if blocked, reason := c.checkImpact(target); blocked {
        return "", fmt.Errorf("blocked: %s", reason)
    }

    // 5. Generate code via LLM
    result := c.generateCode(ctx, coderTask, fileContext)

    // 6. Apply edits via VirtualStore
    c.applyEdits(ctx, result.Edits)

    // 7. Generate facts for propagation back to parent
    result.Facts = c.generateFacts(result)
}
```

## 11. Policy File Structure

The policy file ([internal/mangle/policy.gl](internal/mangle/policy.gl)) has 20 sections:

1. **Spreading Activation** (§1) - Context selection
2. **Strategy Selection** (§2) - Dynamic workflow dispatch
3. **TDD Repair Loop** (§3) - Write→Test→Analyze→Fix
4. **Focus Resolution** (§4) - Clarification threshold
5. **Impact Analysis** (§5) - Refactoring guard
6. **Commit Barrier** (§6) - Block commit on errors
7. **Constitutional Safety** (§7) - Permission gates
8. **Shard Delegation** (§8) - Task routing
9. **Browser Physics** (§9) - DOM spatial reasoning
10. **Tool Capability Mapping** (§10) - Action mappings
11. **Abductive Reasoning** (§11) - Missing hypothesis detection
12. **Autopoiesis** (§12) - Learning patterns
13. **Git-Aware Safety** (§13) - Chesterton's Fence
14. **Shadow Mode** (§14) - Counterfactual reasoning
15. **Interactive Diff** (§15) - Mutation approval
16. **Session State** (§16) - Clarification loop
17. **Knowledge Atoms** (§17) - Domain expertise
18. **Shard Types** (§18) - Classification
19. **Campaign Orchestration** (§19) - Multi-phase execution
20. **Campaign Triggers** (§20) - Campaign start detection

## Resources

For detailed specifications, consult the reference documentation:

- [references/architecture.md](references/architecture.md) - Theoretical foundations and neuro-symbolic principles
- [references/mangle-schemas.md](references/mangle-schemas.md) - Complete Mangle schema reference
- [references/implementation-guide.md](references/implementation-guide.md) - Go implementation patterns and component details
- [references/piggyback-protocol.md](references/piggyback-protocol.md) - Dual-channel steganographic control protocol specification
- [references/campaign-orchestrator.md](references/campaign-orchestrator.md) - Multi-phase goal execution and context paging system
- [references/autopoiesis.md](references/autopoiesis.md) - Self-creation, runtime learning, Ouroboros state machine, SafetyChecker, and DifferentialEngine
- [references/shard-agents.md](references/shard-agents.md) - All four shard types, ShardManager API, and built-in implementations
- [references/logging-system.md](references/logging-system.md) - Config-driven categorized logging and log-analyzer integration
- [references/skill-registry.md](references/skill-registry.md) - Complete skill ecosystem documentation with trigger conditions and integration points
- [references/semantic-classification.md](references/semantic-classification.md) - Neuro-symbolic intent classification with baked-in vector database

## Key Implementation Locations

| Component | Location | Purpose |
|-----------|----------|---------|
| Kernel | [internal/core/kernel.go](internal/core/kernel.go) | Mangle engine + fact management |
| Dreamer | [internal/core/dreamer.go](internal/core/dreamer.go) | Precog safety simulation |
| VirtualStore | [internal/core/virtual_store.go](internal/core/virtual_store.go) | FFI to external systems |
| ShardManager | [internal/core/shard_manager.go](internal/core/shard_manager.go) | Shard lifecycle |
| DifferentialEngine | [internal/mangle/differential.go](internal/mangle/differential.go) | Incremental Mangle evaluation |
| FeedbackLoop | [internal/mangle/feedback/loop.go](internal/mangle/feedback/loop.go) | LLM Mangle generation with retry |
| Transducer | [internal/perception/transducer.go](internal/perception/transducer.go) | NL→Atoms |
| SemanticClassifier | [internal/perception/semantic_classifier.go](internal/perception/semantic_classifier.go) | Vector-based intent classification |
| Emitter | [internal/articulation/emitter.go](internal/articulation/emitter.go) | Atoms→NL (Piggyback) |
| PromptAssembler | [internal/articulation/prompt_assembler.go](internal/articulation/prompt_assembler.go) | JIT prompt compilation integration |
| JITPromptCompiler | [internal/prompt/compiler.go](internal/prompt/compiler.go) | Dynamic prompt assembly from atoms |
| PromptAtomSelector | [internal/prompt/selector.go](internal/prompt/selector.go) | Mangle + Vector atom selection |
| PromptDependencyResolver | [internal/prompt/resolver.go](internal/prompt/resolver.go) | Dependency resolution |
| TokenBudgetManager | [internal/prompt/budget.go](internal/prompt/budget.go) | Token budget allocation |
| PromptLoader | [internal/prompt/loader.go](internal/prompt/loader.go) | YAML→SQLite prompt loading |
| OuroborosLoop | [internal/autopoiesis/ouroboros.go](internal/autopoiesis/ouroboros.go) | Tool self-generation state machine |
| SafetyChecker | [internal/autopoiesis/checker.go](internal/autopoiesis/checker.go) | AST + Mangle safety validation |
| Legislator | [internal/shards/system/legislator.go](internal/shards/system/legislator.go) | Rule synthesis and hot-loading |
| CoderShard | [internal/shards/coder/coder.go](internal/shards/coder/coder.go) | Code generation |
| ResearcherShard | [internal/shards/researcher/researcher.go](internal/shards/researcher/researcher.go) | Knowledge gathering |
| NemesisShard | [internal/shards/nemesis/nemesis.go](internal/shards/nemesis/nemesis.go) | Adversarial testing specialist |
| Thunderdome | [internal/autopoiesis/thunderdome.go](internal/autopoiesis/thunderdome.go) | Battle arena for tools |
| PanicMaker | [internal/autopoiesis/panic_maker.go](internal/autopoiesis/panic_maker.go) | Attack vector generation |
| Armory | [internal/shards/nemesis/armory.go](internal/shards/nemesis/armory.go) | Attack persistence for regression |
| Dream State | [cmd/nerd/chat/process.go:607](cmd/nerd/chat/process.go#L607) | Multi-agent simulation mode |
| Logging | [internal/logging/logger.go](internal/logging/logger.go) | Config-driven categorized logging |
