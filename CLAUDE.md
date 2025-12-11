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