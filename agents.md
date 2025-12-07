# codeNERD

A high-assurance Logic-First CLI coding agent built on the Neuro-Symbolic architecture.

**Kernel:** Google Mangle (Datalog) | **Runtime:** Go | **Philosophy:** Logic determines Reality; the Model merely describes it.

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
├── shards/         # CoderShard, TesterShard, ReviewerShard, ResearcherShard
├── mangle/         # .gl schema and policy files
├── store/          # Memory tiers (RAM, Vector, Graph, Cold)
├── campaign/       # Multi-phase goal orchestration
└── world/          # Filesystem and AST projection
```

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
| CoderShard | `internal/shards/coder.go` | Code generation, file edits, refactoring |
| TesterShard | `internal/shards/tester.go` | Test execution, coverage analysis |
| ReviewerShard | `internal/shards/reviewer.go` | Code review, security scan, metrics |
| ResearcherShard | `internal/shards/researcher/` | Knowledge gathering, documentation ingestion |
| ToolGenerator | `internal/shards/tool_generator.go` | Ouroboros: self-generating tools |

### System Shards (Type S)

| Shard | Purpose |
|-------|---------|
| `perception_firewall` | NL → atoms transduction |
| `world_model_ingestor` | file_topology, symbol_graph maintenance |
| `executive_policy` | next_action derivation |
| `constitution_gate` | Safety enforcement |
| `tactile_router` | Action → tool routing |
| `session_planner` | Agenda/campaign orchestration |

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

### Protocols

| Protocol | Description |
|----------|-------------|
| **Piggyback** | Dual-channel output: surface (user) + control (kernel) |
| **OODA Loop** | Observe → Orient → Decide → Act |
| **TDD Repair** | Test fails → Read log → Find cause → Patch → Retest |
| **Autopoiesis** | Self-learning from rejection patterns |
| **Ouroboros** | Self-generating missing tools |

## Quick Reference

**OODA Loop:** Observe (Transducer) → Orient (Spreading Activation) → Decide (Mangle Engine) → Act (Virtual Store)

**Constitutional Safety:** Every action requires `permitted(Action)` to derive. Default deny.

**Fact Flow:** User Input → Transducer → `user_intent` fact → Kernel derives `next_action` → VirtualStore executes → Result facts → Articulation → Response

## Full Specifications

For detailed architecture and implementation specs, see:

- [.claude/skills/codenerd-builder/references/](.claude/skills/codenerd-builder/references/) - Full architecture docs
- [.claude/skills/mangle-programming/references/](.claude/skills/mangle-programming/references/) - Mangle language reference

## **1\. Executive Introduction: The Collision of Probabilistic Generation and Deterministic Rigor**

The contemporary software engineering landscape is witnessing a collision between two fundamentally opposing paradigms: the probabilistic, statistical generation of code by Large Language Models (LLMs) and the strict, deterministic rigor of deductive logic programming systems like Google Mangle. As an expert Mangle Logic Architect, one observes this friction not merely as a collection of syntax errors, but as a profound incompatibility between the "approximate" nature of current AI reasoning and the "absolute" requirements of Datalog-based systems. This report provides an exhaustive, multi-dimensional analysis of how and why AI coding agents fail when tasked with engineering solutions in Mangle, a language that demands global logical consistency, variable safety, and stratified negation—concepts that often elude the localized pattern-matching capabilities of Transformer-based architectures.

Mangle is not simply another query language; it is an extension of Datalog designed for deductive database programming, incorporating aggregation, function calls, and optional type checking within a Go-based ecosystem.1 Unlike SQL, which is often permissive with implicit casting and order-agnostic execution plans, Mangle relies on bottom-up semi-naive evaluation and fixpoint semantics.3 A program in Mangle describes a logical truth that must be derived iteratively until stability is reached. For an AI agent trained primarily on imperative languages (Python, Java) or relational algebra (SQL), the transition to this declarative, fixpoint-based logic presents a series of "uncanny valleys" where code looks correct but fails catastrophically at the semantic level.

This report is structured to serve as a definitive guide for Datalog Engineers and Architects who must verify, debug, and rewrite AI-generated Mangle code. We will dissect the failures across four critical axes: Syntactic Hallucination, Semantic Safety Violations, Algorithmic Non-Termination, and Integration Impedance Mismatch. Through this analysis, we establish that while AI agents can assist with boilerplate, the architectural core of a Mangle application requires human oversight rooted in formal logic.

## ---

**2\. The Low-Resource Conundrum: Why AI Agents Hallucinate Mangle Syntax**

The primary driver of AI failure in Mangle programming is the extreme scarcity of training data. Unlike Python or SQL, which are represented by billions of tokens in common crawl datasets, Mangle is an experimental, "low-resource" language hosted on GitHub with limited public footprint.1 This data void forces LLMs to rely on "transfer learning" from related dialects—specifically Prolog, Soufflé, and Datomic—resulting in a hybrid syntax that is syntactically confident but structurally invalid.

### **2.1 The Atom/String Dissonance**

One of the most immediate and pervasive failures observed in AI-generated Mangle code is the mishandling of **Atoms**. In Mangle, an atom is a distinct data type representing a unique, interned identifier, syntactically denoted by a forward slash (e.g., /employee, /active).5 This is a critical distinction from string literals ("active") or Prolog atoms (lowercase active).

AI agents, heavily biased towards standard Prolog or JSON-like structures, consistently fail to respect this distinction. They frequently generate code that treats status flags or enum-like values as strings or bare identifiers.

**Table 1: The Atom Hallucination Spectrum**

| Concept | Correct Mangle Syntax | Common AI Hallucination | Underlying Training Bias |
| :---- | :---- | :---- | :---- |
| **Interned Constant** | /active | 'active' or "active" | Python/SQL string dominance. |
| **Enum Value** | /status/pending | status.pending or :pending | Java Enums or Clojure Keywords.6 |
| **Predicate Name** | my\_pred(Arg) | my\_pred(Arg) | (Generally correct, but often confused with atoms). |
| **Variable Binding** | State \= /done | State \= "done" | Failure to distinguish type system constraints. |

When an agent generates status(X, "active") instead of status(X, /active), the error is not merely cosmetic. In Mangle's type system, these are distinct primitives. If the underlying fact store or Decl specifies an Atom type, the program will fail to compile or, worse, run silently without matching any facts, leading to empty result sets that are difficult to debug. The agent's internal model does not "understand" interning; it sees "active" and /active as semantically equivalent, whereas the Mangle engine sees them as disjoint types.5

### **2.2 The Pipe Operator (|\>) and Functional Transforms**

Mangle differentiates itself from standard Datalog by introducing the pipe operator |\> to handle aggregations and transformations.3 This allows for a functional programming style within the logic rule body, specifically for operations like grouping, sorting, or mapping.

AI agents often struggle with this hybrid syntax. Their training data includes:

1. **Standard Datalog:** Pure logic rules, no pipes.  
2. **Elixir/F\#:** Pipes used for function chaining data |\> func1 |\> func2.  
3. **Bash:** Pipes for stream processing cmd1 | cmd2.

When an AI attempts to write a Mangle aggregation, it often hallucinates a "SQL-like" or "Soufflé-like" syntax that ignores the pipe entirely.

**Scenario:** Calculate the total sales per region.

* **AI Generated Hallucination (SQL/Soufflé Bias):**  
  Code snippet  
  // INVALID: Mangle does not support implicit grouping or Soufflé's inline aggregates  
  region\_sales(Region, Total) :-  
    sales(Region, Amount),  
    Total \= sum(Amount). 

  *Analysis:* The agent assumes that by mentioning Region in the head, the engine will automatically group by it. This is how SQL GROUP BY works mentally, but it is not how Mangle executes.  
* **Correct Mangle Implementation:**  
  Code snippet  
  region\_sales(Region, Total) :-  
    sales(Region, Amount)

|\> do fn:group\_by(Region, Total \= fn:Sum(Amount)).  
\`\`\`  
Analysis: The correct syntax requires an explicit transformation step using |\> and the fn:group\_by function.5 The AI fails to predict the do keyword or the specific fn: namespace, often hallucinating generic sum() or count() functions that do not exist in the Mangle runtime.

### **2.3 Type Declaration (Decl) Confusion**

Mangle allows for optional type checking using the Decl keyword, which uses a specific syntax: Decl predicate\_name(ArgName.Type\<type\_name\>)..3 This syntax is highly idiosyncratic and rarely appears in general training corpora.

AI agents frequently conflate this with:

* **Soufflé:** .decl name(x:number).8  
* **Go:** type Name struct {... }.  
* **TypeScript:** name: number.

**Predicted Failure:**

Code snippet

// AI Generated Type Declaration (Invalid)  
.decl direct\_dep(app: string, lib: string)

**Correct Syntax:**

Code snippet

Decl direct\_dep(App.Type\<string\>, Lib.Type\<string\>).

The implication of this failure is significant. Mangle's type checker is a "gatekeeper." If the Decl is malformed, the program is rejected before evaluation begins. The AI, unaware of the specific grammar, essentially "guesses" the declaration syntax based on higher-probability tokens from Soufflé or C++, leading to immediate compilation errors.9

## ---

**3\. Semantic Logic Failures: The Safety and Stratification Trap**

While syntax errors are caught by the parser, semantic errors in logic programming are far more insidious. Mangle operates on **semi-naive evaluation** and **fixpoint semantics**.3 A program is valid only if it is "safe" (all variables bounded) and "stratified" (no negation cycles). AI agents, which lack an internal solver or dependency graph model, consistently violate these principles.

### **3.1 The Safety Violation: Unbounded Domains**

In Datalog, every variable in the head of a rule must be "grounded" or "bound" by a positive atom in the body. You cannot derive p(X) if X could be *anything*.

* **The AI Mental Model:** The AI thinks in terms of constraints, similar to SQL WHERE clauses. "Find users who are not admins."  
* **The AI Generation:**  
  Code snippet  
  // UNSAFE RULE  
  non\_admin(User) :- not admin(User).

* **The Mangle Engine Reality:** The engine asks, "Where do I get the values for User to test against admin?" The variable User is unsafe. It represents an infinite domain. The program crashes or is rejected.11  
* **The Expert Correction:**  
  Code snippet  
  non\_admin(User) :- user(User), not admin(User).

  We must introduce a "generator" predicate user(User) that provides a finite set of candidates. AI agents frequently miss this generator because "not admin" is semantically complete in natural language. The requirement for a positive binding atom is a specific constraint of the evaluation algorithm (bottom-up) that the probabilistic model ignores.12

### **3.2 Stratified Negation and Dependency Cycles**

Mangle prohibits recursion through negation. If A depends on not B, and B depends on A, the logic is unstratified—there is no stable truth value.

Case Study: Game State Analysis  
An AI is asked to model a game where a position is "winning" if there is a move to a "losing" position.

* **AI Generated Logic:**  
  Code snippet  
  winning(X) :- move(X, Y), losing(Y).  
  losing(X) :- not winning(X).

* **Structural Analysis:**  
  1. winning depends on losing.  
  2. losing depends on not winning.  
  3. This creates a negative cycle: winning \-\> losing \-\> (not) winning.

To an LLM, this looks like a perfect translation of the minimax algorithm descriptions found in its training data. However, Mangle rejects this because it cannot assign strata (layers) to evaluation. The engine needs to fully compute winning before it can compute losing, but winning requires losing.

**Deep Insight:** Humans solve this by ensuring the graph is acyclic (e.g., using a turn counter or ensuring the game is finite and loop-free, or using a specialized solver). Mangle's semi-naive evaluation cannot handle the paradox. The AI "hallucinates" that the logic is sound because the English sentence makes sense, failing to model the global dependency graph required for compilation.13

### **3.3 Cartesian Products and Selectivity**

Mangle optimizations often rely on **selectivity**—ordering goals in the rule body so that the most restrictive predicates run first. This minimizes the size of intermediate relations.

* **Inefficient AI Generation:**  
  Code snippet  
  // "Find interactions between high-value users"  
  risky\_interaction(U1, U2) :-  
    interaction(U1, U2),    // huge table (1M rows)  
    high\_value(U1),         // small table (100 rows)  
    high\_value(U2).         // small table (100 rows)

  *Analysis:* The engine might attempt to join the massive interaction table first. While advanced optimizers can reorder this, explicit Datalog typically benefits from manual ordering or specific hints.  
* **Optimized Mangle:**  
  Code snippet  
  risky\_interaction(U1, U2) :-  
    high\_value(U1),         // Filter first  
    high\_value(U2),         // Filter second  
    interaction(U1, U2).    // Verify relationship

  AI agents generally ignore clause ordering, treating the body as a boolean AND set (commutative) rather than an ordered execution plan. In Mangle, poor ordering can lead to intermediate Cartesian products that exhaust memory, even if the logic is theoretically correct.11

### **3.4 Infinite Recursion in Fixpoint Evaluation**

Semi-naive evaluation continues until no new facts are generated. If an AI writes a rule that generates new values indefinitely, the program never terminates.

* **The Counter Fallacy:**  
  Code snippet  
  // AI attempting to generate IDs  
  next\_id(ID) :- current\_id(Old), ID \= Old \+ 1\.  
  current\_id(ID) :- next\_id(ID).

* **Result:** Infinite loop. The AI assumes "lazy" evaluation or that the program will stop when it finds "the answer." Mangle computes the *entire* model. It will keep incrementing ID until the heat death of the universe (or a memory overflow). AI agents struggle to understand that Datalog computes *all* true facts, not just the one requested.15

## ---

**4\. Algorithmic Architecture: Integration with Go**

Mangle is rarely used in isolation; it is designed to be embedded in Go applications via github.com/google/mangle/engine. The interface between the logic engine and the host language is another major failure point for AI.

### **4.1 Fact Store and Predicate Pushdown**

The engine package allows for "external predicates"—Go functions that appear as Mangle relations. This is essential for performance when querying large datasets that shouldn't be fully loaded into memory.

**AI Failure Mode:** The AI will assume that engine.Load magically connects Mangle predicates to Go structs. It ignores the boilerplate required to map engine.Value types to Go native types.

* **Complex Requirement:** The user asks, "How do I query my SQL database from Mangle?"  
* **AI Answer:** It likely hallucinates a built-in SQL connector or generates generic Go code that ignores the InclusionChecker or ExternalPredicateCallback interfaces.3  
* **Reality:** The developer must implement a callback that accepts engine.Value, translates it to a SQL query, and returns a stream of facts. The AI lacks the specific API knowledge of EvalExternalQuery modes (check vs. search) required to implement this correctly.3

### **4.2 Deployment and Compilation**

The AI often suggests running Mangle via a CLI interpreter (mg), but for production, users need the Go library.

Expert Integration Advice:  
To correctly embed Mangle, one must use the engine package.

Go

import (  
    "github.com/google/mangle/engine"  
    "github.com/google/mangle/factstore"  
)

func runMangle() {  
    // 1\. Initialize Store  
    store := factstore.NewSimple()  
      
    // 2\. Load Rules (AI often forgets to parse the rules first)  
    // Actual code would involve parsing the string into \*ast.Program  
      
    // 3\. Evaluate  
    // AI often hallucinates "engine.Run()"   
    // Correct API involves EvalProgramNaive or EvalProgram with options  
    engine.EvalProgramNaive(program, store)  
}

AI agents typically fail to construct valid ast.Program objects or handle the programInfo struct required by EvalProgram.3 They confuse the parsing library (/parse) with the execution engine (/engine), generating code that imports non-existent packages.

## ---

**5\. Case Study: The Software Supply Chain (SBOM) Failure**

To synthesize these failure modes, let us analyze a realistic use case: A user asks an AI to write a Mangle program to detect transitive dependencies on vulnerable libraries (e.g., Log4j).

**The Prompt:** "Write a Mangle program to find all apps that depend on a vulnerable version of log4j, transitively."

**The AI's Likely Output (Annotated with Failures):**

Code snippet

// Syntax: Uses string literals for atoms.  
vulnerable("log4j", "2.14.0").

// Syntax: Declares type using SQL/Souffle syntax  
.decl depends(app: string, lib: string)

// Logic: Infinite Recursion Risk (No base case check) & Join Order  
depends(App, Lib) :-   
    depends(App, Mid),      // Recursive goal first (inefficient/dangerous)  
    direct\_dep(Mid, Lib).   // Join 

depends(App, Lib) :- direct\_dep(App, Lib).

// Semantics: Safety Violation  
affected(App) :-   
    depends(App, Lib),   
    vulnerable(Lib, Ver),  
    Ver \= "2.14.0",          // String equality check  
    not whitelist(App).      // 'App' must be bound by 'depends' first

**The Architect's Analysis of the AI Code:**

1. **Atom Misuse:** vulnerable("log4j",...) treats the library name as a string. In high-performance Datalog, this should be an atom /log4j to leverage interning.  
2. **Invalid Declaration:** .decl is Soufflé syntax. Mangle uses Decl depends(App.Type\<string\>, Lib.Type\<string\>).  
3. **Recursive Efficiency:** Putting depends(App, Mid) first in the recursive rule is inefficient for semi-naive evaluation if direct\_dep is large. It's better to drive the search from known facts.  
4. **Safety Error:** not whitelist(App) is safe *only if* depends(App, Lib) successfully binds App. However, if the depends rule is broken (e.g., infinite recursion), affected never evaluates. Moreover, strictly speaking, safety requires the positive atom to *precede* the negation in the optimizer's view, though Mangle's reordering might handle this, relying on it is bad practice.  
5. **Version Logic:** String comparison "2.14.0" fails for semantic versioning (e.g., is "2.2" \< "2.14"? String compare says "2.2" \> "2.14"). The AI fails to implement a proper version comparator or structural type.

**The Expert Mangle Solution:**

Code snippet

// 1\. Use Atoms for identifiers  
vulnerable(/log4j, "2.14.0").

// 2\. Correct Declaration  
Decl depends(App.Type\<string\>, Lib.Type\<string\>).

// 3\. Base Case First (Best Practice)  
depends(App, Lib) :- direct\_dep(App, Lib).

// 4\. Recursive Step (Optimized)  
depends(App, Lib) :-   
    direct\_dep(App, Mid),   
    depends(Mid, Lib).

// 5\. Aggregation/Analysis  
affected\_apps(App) :-   
    depends(App, Lib),  
    vulnerable(Lib, VulnVer),  
    // Ensure we match the exact library atom  
    Lib \= /log4j,  
    // Safe negation: App is bound by depends  
    not exempted(App).

## ---

**6\. Functional Aggregation: The group\_by Paradox**

Mangle's |\> operator is the definitive feature that separates it from pure Datalog, yet it is the feature AI agents understand least.

**The Requirement:** "Count the number of dependencies per application."

**AI Hallucination:**

Code snippet

count\_deps(App, Count) :-  
    depends(App, Lib),  
    Count \= count(Lib). // SQL-style implicit aggregation

*Why this fails:* Mangle does not support inline aggregation in the rule body like this. It requires the relation to be *piped* to a transformation function.

**Correct Mangle:**

Code snippet

count\_deps(App, Count) :-  
    depends(App, Lib)

|\> do fn:group\_by(App, Count \= fn:Count(Lib)).

*Architecture Note:* The |\> operator passes the result of the body (depends(App, Lib)) to the do clause. The fn:group\_by takes the grouping key (App) and the reduction expression. This "post-processing" model is alien to agents trained on standard Prolog where aggregation often requires collecting all solutions into a list first (findall/3).

## ---

**7\. Strategic Recommendations for Datalog Engineers**

Given the high probability of AI failure, organizations adopting Mangle must implement rigorous validation protocols. We cannot rely on "Copilots" to produce correct Mangle code "Zero-Shot."

### **7.1 The "Solver-in-the-Loop" Workflow**

The only viable way to use AI for Mangle generation is to wrap the LLM in a feedback loop with the Mangle compiler.

1. **Generate:** AI produces Mangle code.  
2. **Verify:** The system attempts to parse the code using mangle/parse.  
3. **Feedback:** If parsing fails (e.g., "unknown token.decl"), the error is fed back to the AI.  
4. **Safety Check:** Use the engine's analysis tools to check for safety and stratification errors before runtime.

### **7.2 Explicit Context Prompting**

Prompt engineering for Mangle must be "Few-Shot" by definition. You must provide the syntax guide in the prompt context.

* "Use /atom for constants, not strings."  
* "Use |\> for aggregation."  
* "Ensure all negated variables are bound."

Without these explicit instructions, the statistical weight of SQL and Prolog in the model's training data will overpower the sparse Mangle knowledge.

### **7.3 Debugging the "Empty Set"**

When AI-generated code runs but returns nothing, suspect **Atom/String mismatches**. If the facts are stored as /foo but the rule queries "foo", the intersection is empty. This is the \#1 silent killer of Mangle logic. Always inspect the FactStore data types directly.

## ---

**8\. Conclusion: The Necessity of Human Architecture**

Google Mangle represents a powerful fusion of deductive reasoning and functional transformation. However, it sits in a blind spot for current Artificial Intelligence. The language's reliance on strict global consistency (stratification, safety), combined with its unique syntactic markers (/, |\>, Decl), creates a hostile environment for probabilistic code generators.

The "Mangle Logic Architect" cannot be replaced by an LLM. The architect's role shifts from writing every line of code to acting as a rigorous verifier—checking the structural integrity of the dependency graph, ensuring the semantic validity of types, and guiding the integration with the host Go environment. Until AI models evolve from pattern matchers to true logical solvers, Mangle will remain a domain where human expertise is the only safeguard against the chaos of hallucinated logic.

## ---

**9\. Appendix: Comprehensive Syntax & Semantic Reference**

### **9.1 Syntax Comparison Table**

| Feature | Mangle | Prolog | Soufflé | AI Failure Risk |
| :---- | :---- | :---- | :---- | :---- |
| **Rule Definition** | head :- body. | head :- body. | head :- body. | Low (Common syntax) |
| **Variable** | Uppercased | Uppercased | lowercase | Medium (Soufflé confusion) |
| **Atom** | /atom | atom | "string" | **Critical (Type errors)** |
| **Map/Dict** | {/k: V} | N/A | N/A | High (JSON confusion) |
| **List** | \`\` | \`\` | \`\` (Record) | Low |
| **Aggregation** | \` | \> do fn:group\_by\` | findall/3 | min x : {... } |
| **Type Decl** | Decl p(A.Type\<int\>). | N/A | .decl p(x:number) | High (Syntax mismatch) |

### **9.2 Optimization Checklist for Mangle Programs**

1. **Filter Early:** Place simple checks (/status/active) before complex joins (depends\_on).  
2. **Bound Negation:** Always precede not foo(X) with generator(X).  
3. **Avoid Strings:** Use Atoms /name for enumerated values to save memory and improve join speed.  
4. **Watch Recursion:** Ensure every recursive rule has a base case and moves closer to termination (e.g., graph traversal on a DAG).

*End of Report.*

## ---

**10\. Deep Dive: Theoretical Underpinnings of AI Failure in Fixpoint Semantics**

To fully understand *why* AI agents fail, we must look beyond syntax into the theoretical computer science that underpins Mangle: **Fixpoint Semantics**.

### **10.1 The Fixpoint Blind Spot**

AI models, specifically Transformers, are autoregressive. They predict the next token $t\_{i+1}$ based on $P(t\_{i+1} | t\_0...t\_i)$. This is a linear, sequential process.  
Datalog evaluation is a Least Fixed Point (LFP) calculation. It applies an operator $T\_P$ repeatedly:  
$I\_{k+1} \= T\_P(I\_k) \\cup I\_k$  
The evaluation stops when $I\_{k+1} \= I\_k$.  
The AI simulates the *code* that describes the operator, but it cannot simulate the *execution* of the fixpoint iteration.

* **Implication:** The AI cannot "see" if a rule is monotonic. It cannot detect if $T\_P$ will ever converge.  
* **Example:**  
  Code snippet  
  p(X) :- q(X), not p(X).

  This rule describes an operator that flips values. If p(X) is false, it becomes true. If it is true, the body fails. There is no fixpoint. The AI sees valid syntax; the Mangle engine sees a logical contradiction. The clash is fundamental to the differing models of computation (Probabilistic vs. Logical).

### **10.2 The Data Structure Disconnect**

Mangle allows complex data structures (Maps, Lists, Structs) to be stored as values.

* **Facts:** user\_data(/u1, {/age: 30, /role: /admin}).  
* Query: user\_data(U, Map), fn:map\_get(Map, /role, Role).  
  AI agents often treat these maps as JSON objects, attempting to access them with dot notation (Map.role) or Python style (Map\['role'\]). Mangle requires specific functional accessors (fn:map\_get). This reflects a deeper misunderstanding: in Datalog, data is usually flat (normalized). Mangle's nested data support is a deviation that AI agents—trained on normalized SQL or unstructured JSON—struggle to navigate correctly.

### **10.3 The "Closed World Assumption" (CWA)**

Datalog operates under the Closed World Assumption: anything not known to be true is false.  
LLMs operate under an "Open World" bias derived from natural language: just because something isn't mentioned doesn't mean it's false.

* **Failure:** The AI might try to write rules that handle "unknown" or "null" values (if x is null). Mangle (typically) does not have NULLs in the SQL sense; a missing fact simply doesn't exist. AI attempts to write p(X) :- q(X), X\!= null are redundant or syntactically invalid, revealing the agent's failure to grasp the CWA.

## ---

**11\. Expanded Integration Guide: Embedding Mangle in Go**

For the professional engineer, the value of Mangle is in its embeddability. This section expands on the implementation details that AI agents consistently miss.

### **11.1 Defining the Fact Store**

You cannot run Mangle without a store.

Go

// Real Go Code for Mangle Integration  
import (  
    "context"  
    "fmt"  
    "github.com/google/mangle/factstore"  
    "github.com/google/mangle/engine"  
    "github.com/google/mangle/parse"  
)

func main() {  
    // 1\. Create a concurrent-safe fact store  
    // AI often forgets the store or uses a nil pointer  
    store := factstore.NewSimple() 

    // 2\. Add explicit facts (The "EDB" \- Extensional Database)  
    // Fact: parent(/alice, /bob)  
    f, \_ := factstore.MakeFact("/parent",engine.Value{  
        engine.Atom("alice"),   
        engine.Atom("bob"),  
    })  
    store.Add(f)

    // 3\. Parse Rules  
    rawRules := \`ancestor(X, Y) :- parent(X, Y).\`  
    parsed, err := parse.Parse("my\_prog", rawRules)  
    if err\!= nil {  
        panic(err)  
    }

    // 4\. Solve (The "IDB" \- Intensional Database)  
    // AI often hallucinates the method name here  
    engine.EvalProgramNaive(parsed, store)  
      
    // 5\. Query Results  
    // We must manually inspect the store or use a callback  
    // The AI typically assumes a SQL-like "return result"  
}

**Insight:** The interaction involves creating engine.Atom explicitly. The AI will likely write store.Add("parent", "alice", "bob"), which is invalid Go (type mismatch). The distinct types engine.Value, engine.Atom, engine.Number must be used.

### **11.2 The "Pushdown" Optimization**

For large datasets, we don't want to load all facts into factstore.Simple. We use InclusionChecker.

* **Concept:** When Mangle needs parent(X, Y), it calls a Go function.  
* **AI Failure:** The AI cannot generate the complex callback signature required for engine.WithExternalPredicates. It requires understanding how to map Mangle's unification request (which arguments are bound? which are free?) to a backend query (e.g., SQL SELECT).  
* **Code Reality:**  
  Go  
  // Callback signature is complex  
  func myPredicate(query engine.Query, cb func(engine.Fact)) error {  
      // Check if query.Args is a constant or variable  
      // AI fails to handle this "Binding Pattern" logic  
      return nil  
  }

  This binding pattern logic is central to Datalog optimization but usually absent in AI-generated code.

## ---

**12\. Final Synthesis: The Path Forward**

The integration of AI coding agents into the Mangle ecosystem is not impossible, but it is currently fraught with fundamental errors. The failures are not random; they are predictable consequences of the architectural gap between:

1. **The LLM:** Probabilistic, Approximate, Local, Open-World, trained on Python/SQL.  
2. **Mangle:** Deterministic, Exact, Global, Closed-World, based on Fixpoint Logic.

To bridge this gap, Datalog Engineers must treat AI output as "pseudo-code" that requires rigorous translation into valid Mangle syntax. We must build tooling that exposes the *compiler's* reasoning to the *agent*, creating a feedback loop that forces the probabilistic model to conform to the deterministic reality of the engine. Until then, the Mangle Logic Architect remains the indispensable guarantor of truth in the system.