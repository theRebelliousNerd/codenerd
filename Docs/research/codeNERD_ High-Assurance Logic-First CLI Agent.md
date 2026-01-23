# **codeNERD: High-Assurance Logic-First CLI Agent**

System Specification & Architectural Blueprint  
Version: 1.4.0 (The "Deep Dive" Release)  
Architecture: Neuro-Symbolic / Inversion of Control  
Kernel: Google Mangle (Datalog)  
Runtime: Golang  
Philosophy: Logic determines Reality; the Model merely describes it.

## **1.0 THE PERCEPTION TRANSDUCER (INPUT LAYER)**

Macro-Objective: To act as a "Semantic Firewall" that converts high-entropy, unstructured Natural Language (NL) from the user into low-entropy, structurally rigid Logic Atoms.  
Theoretical Imperative: The current generation of agents suffers from the "Stochastic Crisis" where LLMs attempt to reason via token prediction. In codeNERD, the LLM is strictly forbidden from "thinking," "planning," or "executing" in this layer; it functions solely as a probabilistic parser (a "Transducer") that maps vague human intent to precise logical form.

### **1.1 Micro-Goal: Intent Disambiguation & Classification**

The system must mathematically distinguish between three fundamental categories of interaction:

1. **Query (Read-Only):** Requests for information that change no state (e.g., "How does auth work?").  
2. **Mutation (Write):** Requests that alter the state of the codebase (e.g., "Refactor the login handler").  
3. **Instruction (Axiomatic):** Provision of new rules or preferences (e.g., "Never use unwrap() in Rust").

It is critical that the system does not conflate "Search for X" (Query) with "Delete X" (Mutation).

* **Schema Requirement (user\_intent\_schema):**  
  * **Goal:** Capture the user's desire without executing it. This atom serves as the "Seed" for the spreading activation network in the Logic Kernel.  
  * **Fields Needed:**  
    * Verb: A controlled vocabulary string. Must be mapped to an enum in the Ontology (e.g., refactor, debug, explain, generate\_test, scaffold, define\_agent). **Crucially**, generic verbs like "fix" must be disambiguated here or flagged for clarification.  
    * PrimaryTarget: The abstract noun being acted upon (e.g., "auth system", "login button", "RustExpert"). This is the "subject" of the logical proposition.  
    * ConstraintVector: A list of negative constraints (e.g., "no external libs", "keep backward compatibility", "max\_cyclomatic\_complexity \< 10"). These become "Hard Constraints" in the Mangle derivation process.  
    * ContextWindowStrategy: A hint for the memory pager (e.g., "Broad" for architectural questions, "Narrow" for bug fixes).  
  * **Logic Constraint (Polymorphism):** The schema must support "polymorphic intents." The verb fix applied to a File triggers the tdd\_repair\_loop. The verb fix applied to a DatabaseSchema triggers the migration\_generation\_loop. The Logic Kernel resolves this dispatch, not the LLM.  
* **Grammar Constraint (GCD \- Grammar Constrained Decoding):**  
  * The LLM's output logits must be masked to strictly enforce that the Verb field matches a pre-compiled enum list defined in the Mangle ontology.  
  * **Mechanism:** If the Mangle schema defines Verb ::= 'refactor' | 'debug', and the LLM attempts to generate 'speculate', the logit for 's' is masked to $-\\infty$, forcing the model to choose a valid token. This prevents the "Hallucination of Agency."

### **1.2 Micro-Goal: High-Precision Entity Extraction (The "Focus" Mechanism)**

Users rarely use absolute file paths. They use fuzzy semantic references like "that function we touched yesterday" or "the user handler." The system must resolve these into concrete, absolute file paths and symbol names to anchor the logic.

* **Schema Requirement (focus\_resolution\_schema):**  
  * **Goal:** Map a distinct logical atom to a physical location on the disk (The "Grounding" process).  
  * **Fields Needed:**  
    * RawReference: The original text snippet (e.g., "the auth thing").  
    * ResolvedPath: The absolute file path (e.g., /src/auth/handler.go).  
    * SymbolName: The specific function, struct, or interface name within the file.  
    * Span: Start and End line numbers (critical for partial file editing).  
    * ConfidenceScore: A float (0.0-1.0) indicating the model's certainty in this mapping.  
  * **Logic Constraint (The Clarification Threshold):**  
    * Rule: clarification\_needed(Ref) :- focus\_resolution(Ref, \_, \_, Score), Score \< 0.85.  
    * **Behavior:** If the logic derives clarification\_needed, the Executive Policy is **blocked**. The system enters a "Clarification Loop," asking the user: "By 'auth thing', did you mean AuthHandler in user.go or LoginMiddleware in middleware.go?"  
    * **Justification:** It is better to halt and ask than to hallucinate a file modification on the wrong target.

### **1.3 Micro-Goal: Semantic Ambiguity & Missing Data Detection**

The system must detect when a request is logically incomplete (e.g., "Refactor the function" without specifying which one) or physically impossible (e.g., "Edit the PNG file" \- which codeNERD cannot do).

* **Schema Requirement (ambiguity\_flag\_schema):**  
  * **Goal:** A predicate that represents a "hole" or "vacuum" in the current knowledge graph.  
  * **Fields Needed:**  
    * MissingParam: The name of the required argument that was not found (e.g., TargetFunction).  
    * ContextClue: The part of the user prompt that *hinted* at the missing data.  
    * Hypothesis: The LLM's best guess at what might fill the hole (used for suggesting options to the user).  
  * **Logic Constraint:** The existence of an ambiguity\_flag atom in the EDB effectively "short-circuits" the Executive Policy. The derivation of next\_action switches from execute\_tool to interrogative\_mode.

## **2.0 THE WORLD MODEL (THE EXTENSIONAL DATABASE \- EDB)**

Macro-Objective: To maintain a real-time, relational "Twin" of the physical codebase, file system, and git history.  
Theoretical Imperative: Logic cannot operate on "strings." It operates on relations. Therefore, the physical world must be constantly "ingested" and "projected" into the FactStore (EDB) as structured atoms. This is the "Ground Truth."

### **2.1 Micro-Goal: The "Fact-Based Filesystem"**

The file system must not be treated as a hierarchy of strings, but as a relational database of content, capabilities, and metadata.

* **Schema Requirement (file\_topology\_schema):**  
  * **Goal:** Represent the "Physics" of files.  
  * **Fields Needed:**  
    * Path: Unique identifier.  
    * Hash: SHA-256 for change detection. This allows the system to skip re-indexing files that haven't changed (Incremental Compilation).  
    * Language: Enum (go, python, ts, rust).  
    * LastModified: Timestamp for recency bias calculations.  
    * IsTestFile: Boolean flag. Critical for TDD loops—we treat test code differently from production code.  
    * Size: Used to determine if the file fits in the "Hot" context window or requires "Windowing."  
  * **Logic Constraint (Windowing):** Requires a "Windowing" schema to handle large files. Content cannot simply be a raw string; it must be indexed by line ranges or semantic blocks (e.g., file\_chunk(Path, StartLine, EndLine, Content)). The Logic Kernel pages these chunks in and out based on focus\_resolution.

### **2.2 Micro-Goal: The "Linter-Logic" Bridge**

In standard agents, compiler errors are just text in the context window. In codeNERD, errors, warnings, and panics are **logical constraints** that drive the TDD loop.

* **Schema Requirement (diagnostic\_schema):**  
  * **Goal:** Convert stderr into actionable atoms.  
  * **Fields Needed:**  
    * Severity: Enum (Panic, Error, Warning, Info).  
    * FilePath: The culprit file.  
    * LineNumber: Precise location.  
    * ErrorCode: The compiler code (e.g., E0308 in Rust, TS2322 in TypeScript). This key allows the agent to look up specific documentation in its Knowledge Shard.  
    * Message: The human-readable error string.  
  * **Logic Constraint (The Barrier):**  
    * Rule: block\_commit :- diagnostic(\_, \_, \_, \_, Severity), Severity \== 'Error'.  
    * **Behavior:** The presence of a single diagnostic atom with Severity=Error mathematically inhibits the git\_commit predicate. The system is physically incapable of committing broken code.

### **2.3 Micro-Goal: Abstract Syntax Tree (AST) Projection**

To reason about code structure (inheritance, dependency, scope), the Go runtime must parse the code and project the AST into Mangle atoms. Text search (grep) is insufficient for semantic reasoning.

* **Schema Requirement (symbol\_graph\_schema):**  
  * **Goal:** A graph representation of the code structure.  
  * **Fields Needed:**  
    * SymbolID: Unique ID (e.g., func:main:AuthHandler).  
    * Type: (Function, Class, Interface, Variable, Constant).  
    * Visibility: (Public, Private, Protected).  
    * DefinedAt: Link to file\_topology\_schema (Path \+ Line Number).  
    * Signature: The type signature (e.g., (User) \-\> Result\<bool\>).  
* **Schema Requirement (dependency\_link\_schema):**  
  * **Goal:** Capture the "Call Graph" (A calls B).  
  * **Fields:** CallerID, CalleeID, ImportPath.  
  * **Logic Constraint (Transitive Impact):**  
    * Rule: impacted(X) :- depends\_on(X, Y), modified(Y).  
    * Rule: depends\_on(X, Z) :- depends\_on(X, Y), depends\_on(Y, Z). (Recursive Closure)  
    * **Application:** "If I modify Function A, the logic engine instantly identifies all Functions B, C, and D that rely on A (even indirectly) and marks them as needs\_testing."

## **3.0 THE EXECUTIVE POLICY (THE INTENSIONAL DATABASE \- IDB)**

**Macro-Objective:** The "Brain." A collection of pure logic rules (Horn clauses) that derive the *next action* based on the intersection of the World Model (EDB) and User Intent. This layer is deterministic.

### **3.1 Micro-Goal: The Strategy Selector**

Different coding tasks require different logical loops (algorithms). The kernel must derive which "Strategy Module" to load and execute.

* **Schema Requirement (strategy\_selection\_schema):**  
  * **Goal:** Dynamic dispatch of logical workflows.  
  * **Logic Needed:** Rules that map user\_intent \+ file\_topology to a strategy.  
    * *Scenario A:* If intent is fix\_bug AND file has IsTestFile=true, derive strategy tdd\_repair\_loop.  
    * *Scenario B:* If intent is explore AND file count \> 100, derive strategy breadth\_first\_survey.  
    * *Scenario C:* If intent is scaffold AND project is empty, derive strategy project\_init.  
  * **Constraint:** Only one strategy can be active per shard to prevent "schizophrenic" behavior (trying to explore and fix simultaneously).

### **3.2 Micro-Goal: The TDD Repair Loop (The "OODA" Loop)**

This is the core coding engine. It is a rigorous state machine: Write \-\> Test \-\> Analyze \-\> Fix.

* **Schema Requirement (test\_state\_schema):**  
  * **Goal:** Track the redness/greenness of the build.  
  * **States:** Passing, Failing, Compiling, Unknown.  
* **Schema Requirement (repair\_action\_schema):**  
  * **Goal:** Define the valid moves in a repair game (The "Game Theory" of coding).  
  * **Logic Transition Table:**  
    1. **State:** Failing \+ Retry \< Max \-\> **Action:** read\_error\_log.  
    2. **State:** LogRead \-\> **Action:** analyze\_root\_cause (Abductive Reasoning).  
    3. **State:** CauseFound \-\> **Action:** generate\_patch.  
    4. **State:** PatchApplied \-\> **Action:** run\_tests.  
    5. **State:** Failing \+ Retry \>= Max \-\> **Action:** escalate\_to\_user (Surrender).  
  * **Constraint (Termination):** This loop must be mathematically proven to terminate. Infinite retry loops are forbidden by the schema's attempt\_count check, which decrements a "Logical Budget."

### **3.3 Micro-Goal: The Refactoring Guard & Impact Analysis**

Refactoring is dangerous; it can break hidden dependencies. Logic must enforce safety before a single byte is changed.

* **Schema Requirement (impact\_radius\_schema):**  
  * **Goal:** Calculate the "Blast Radius" of a change.  
  * **Logic Needed:** A recursive rule that transitively closes over dependency\_link\_schema.  
  * **Derivation:** unsafe\_to\_refactor(Target) is derived IF impacted(Dependent) exists AND test\_coverage(Dependent) is FALSE.  
  * **Behavior:** The write\_file action is blocked if unsafe\_to\_refactor is true. The system forces the user to either:  
    1. Authorize the risk explicitly (override).  
    2. Write tests for the uncovered dependents first (the "Safety First" path).

## **4.0 THE TACTILE INTERFACE (VIRTUAL PREDICATES)**

Macro-Objective: The interface between the Logic Kernel and the Operating System. Mangle decides what to do; Go decides how to do it safely.  
Theoretical Imperative: The Logic Kernel is "Hollow"—it has no side effects. It "hallucinates" an action atom, and the Go Runtime (The Virtual Store) observes this atom and reifies it into a syscall.

### **4.1 Micro-Goal: Safe Execution Sandbox**

Shell commands are the most dangerous tool. They must be wrapped in a rigid schema, not free-form text.

* **Schema Requirement (shell\_exec\_request\_schema):**  
  * **Goal:** A standardized request for execution.  
  * **Fields Needed:**  
    * Binary: The command (e.g., go, npm, grep, cargo).  
    * Arguments: A strict list of strings. **Shell injection prevention:** Arguments are passed as an array to exec.Command, never concatenated into a string.  
    * WorkingDirectory: Where to run it.  
    * TimeoutSeconds: Hard limit to prevent hanging (e.g., npm install hanging on network).  
    * EnvironmentVars: Allowed variables (e.g., preventing access to AWS\_SECRET\_KEY unless authorized).  
  * **Logic Constraint:** The Binary must be in a predefined allowlist (Constitution). No rm, mkfs, nc, or dd allowed without explicit override.

### **4.2 Micro-Goal: The "Semantic Grep"**

Regex is insufficient for high-level coding. We need logical search.

* **Schema Requirement (structural\_search\_schema):**  
  * **Goal:** Allow logic to ask the runtime to search the AST structure.  
  * **Fields Needed:**  
    * Pattern: The code pattern (e.g., try { ... } catch { ... } or func $NAME($ARGS) error).  
    * Language: To select the correct parser (Tree-sitter).  
    * Scope: File, Directory, or Repo.  
  * **Logic Constraint:** Returns a list of file\_topology atoms matching the structure, ignoring whitespace/formatting differences.

## **5.0 THE CONSTITUTION (SAFETY LAYER)**

Macro-Objective: A "Grey Goo" prevention layer. These rules take precedence over all others and act as a compile-time check on the agent's derived actions.  
Theoretical Imperative: Safety cannot be "prompted" into an LLM (Probabilistic Safety is not Safety). Safety must be "derived" from axioms (Deterministic Safety).

### **5.1 Micro-Goal: The "Iron Law" of Permission**

* **Schema Requirement (permission\_gate\_schema):**  
  * **Goal:** The master gatekeeper.  
  * **Logic Needed:** perform\_action(Action) is ONLY valid if permitted(Action) is derivable.  
  * **Default Deny:** The system's logical base state contains zero permitted atoms. They must be positively derived from user\_intent \+ safety\_checks.  
  * **Mechanism:** The Go Runtime's Execute() function begins with if \!Mangle.Query("permitted(?)") { return AccessDenied }.

### **5.2 Micro-Goal: Data Exfiltration Prevention**

* **Schema Requirement (network\_policy\_schema):**  
  * **Goal:** Define the allowed internet surface area.  
  * **Fields Needed:**  
    * AllowList: List of domains (e.g., github.com, pypi.org, crates.io).  
    * Protocol: (HTTPS, SSH).  
  * **Logic Constraint:** If next\_action involves a network request to a URL *not* in the allowlist, the Constitution derives security\_violation.  
  * **Heuristic Detection:** The system detects obfuscated exfiltration attempts (e.g., base64 encoded data in URL parameters) by analyzing the argument entropy of the curl/wget command structure.

## **6.0 THE ARTICULATION TRANSDUCER (OUTPUT LAYER)**

**Macro-Objective:** Convert the dry, logical state of the kernel into a helpful, human-readable status update.

### **6.1 Micro-Goal: State-Aware Reporting**

The user shouldn't just see "Done." They should see *why*, *how*, and *what's next*.

* **Schema Requirement (report\_template\_schema):**  
  * **Goal:** Logic that selects the tone and detail level.  
  * **Fields Needed:**  
    * TaskComplexity: Integer score derived from the number of actions taken.  
    * SuccessState: Boolean.  
    * ArtifactsCreated: List of files touched.  
  * **Logic Constraint:**  
    * If TaskComplexity is High (e.g., multi-file refactor), select a "Summary" template ("I modified 14 files...").  
    * If Low (e.g., single line fix), select a "Detailed" template showing the diff.  
  * **Input:** The set of next\_action atoms and exec\_result atoms from the last turn.

### **6.2 Micro-Goal: The Dual-Payload Emitter (Piggyback Interface)**

The Transducer must emit two distinct streams: one for the user (Natural Language) and one for the Kernel (Logic).

* **Schema Requirement (dual\_payload\_schema):**  
  * **Goal:** Enforce strict separation of "Presentation" vs. "State".  
  * **Structure:** A JSON object mandated by the Grammar-Constrained Decoder.  
  * **Logic Constraint:** The control\_packet field must contain valid, parseable Mangle atoms that mathematically map to the next\_state of the system.  
  * **Failure Mode:** If the control\_packet fails Mangle syntax validation, the system treats it as a "transduction failure" and triggers a silent retry with an error-correction prompt, invisible to the user.

## **7.0 SHARDING (SCALABILITY LAYER)**

**Macro-Objective:** Handle massive tasks by spawning "Fractal Agents" (ShardAgents). To support both rapid scaling and deep expertise, the system distinguishes between **Ephemeral Generalists** and **Persistent Specialists**.

### **7.1 Taxonomy of Shards**

* **Type A: Ephemeral Generalists (The "Interns")**  
  * **Lifecycle:** Spawn $\\to$ Execute Task $\\to$ Die.  
  * **Memory:** Starts Blank (RAM only). Zero context pollution from previous tasks.  
  * **Use Case:** Refactoring a file, writing a unit test, fixing a typo.  
  * **Identity:** Generic (e.g., Shard-Gen-001).  
  * **Cost:** Low latency, low token usage.  
* **Type B: Persistent Specialists (The "Experts")**  
  * **Lifecycle:** Defined $\\to$ Hydrated (Research) $\\to$ Sleeping $\\to$ Waking (Task) $\\to$ Sleeping.  
  * **Memory:** Starts Pre-Populated. Mounts a read-only "Knowledge Shard" (SQLite) containing deep domain knowledge (e.g., K8s docs, Rust compiler internals, AWS API specs).  
  * **Use Case:** Architectural review, complex debugging, framework migration.  
  * **Identity:** Named (e.g., RustExpert, SecurityAuditor, K8sArchitect).

### **7.2 Micro-Goal: Lifecycle Management Schema**

The Logic Kernel must treat these two types differently during instantiation.

* **Schema Requirement (shard\_lifecycle\_schema):**  
  * **Goal:** Define how a shard is born and configured.  
  * **Fields:**  
    * ShardType: Enum (Generalist | Specialist).  
    * MountStrategy: (RamDisk | PersistentSQLite).  
    * KnowledgeBase: Path to .db file (Null for Generalists).  
    * Permissions: Subset of parent permissions (e.g., Read-Only, Network-Isolated).  
  * **Logic Constraint:**  
    * IF ShardType \== Generalist: FactStore \= NewInMemoryStore().  
    * IF ShardType \== Specialist: FactStore \= Mount(KnowledgeBase).  
    * **Isolation:** A Shard cannot escalate its own permissions. A Network-Isolated shard can never derive permitted(net\_request).

## **8.0 THE PIGGYBACK PROTOCOL (THE BICAMERAL MIND)**

**Macro-Objective:** Solve the "Monologue Problem" where agents pollute their context with transient reasoning. This protocol creates a separate, high-speed "system bus" for the agent's subconscious, decoupling "Thought" from "Speech."

### **8.1 Protocol Specification**

The protocol defines the wire-level standard for the Input and Output streams of the Transducer.

* **Input Stream (The Synthesized Reality):**  
  * The Transducer **never** receives the raw conversation history.  
  * **Component A (User Input):** Raw text from the operator.  
  * **Component B (Mangle Context Block):** A logic-dense block of atoms representing the *Current Truth* (fixpoint), derived dynamically via Spreading Activation. This ensures the model sees the *State*, not the *Talk*.  
  * **Component C (Hidden Directive):** A system-level instruction injected at the end of the prompt to force adherence to the Dual Payload schema.  
* **Output Stream (The Dual Payload):**  
  * The Transducer outputs a strict JSON object containing two isomorphic payloads.  
  * **JSON Schema:**  
```json
    {  
      "surface_response": "Text visible to the user (e.g., 'I fixed the bug.').",
      "control_packet": {
        "reasoning_trace": "Transient Chain-of-Thought (discarded after verification).",
        "mangle_updates": ["atom(/task_status, /complete)", "atom(/file_state, /modified)"],
        "memory_operations": [
          {"operation": "promote_to_long_term", "fact": "preference(/user, /concise)"},
          {"operation": "forget", "target": "error(/transient_network_fail)"}
        ],
        "self_correction": {"trigger": "failure", "hypothesis": "missing_dependency"}
      }  
    }
```

### **8.2 Mechanism: Infinite Context via Semantic Compression**

The system achieves "Infinite Context" by continuously discarding surface text and retaining only logical state.

* **Compression Loop:**  
  1. **Interaction:** User says "Fix server." Agent replies "Fixing..." (Surface) and emits task\_status(/server, /fixing) (Control).  
  2. **Commit:** Kernel commits task\_status to FactStore (The Logical Twin updates).  
  3. **Pruning:** Kernel **deletes** the text "Fixing..." from the sliding window history.  
  4. **Next Turn:** Transducer sees *only* the atom task\_status(/server, /fixing) in the Context Block.  
* **Metric:** Target compression ratio of \>100:1 compared to raw token history. This allows the agent to work on a project for months without context window exhaustion.

### **8.3 Mechanism: Autopoiesis (The Learning Loop)**

The agent must be able to structurally alter its own behavior (Self-Creation/Self-Repair).

* **Trigger:** Kernel detects a repeated pattern (e.g., User rejects code style 3x in 10 minutes).  
* **Injection:** Kernel injects a meta-directive: "Review recent failures; propose persistent rule change to avoid future rejection."  
* **Proposal:** Transducer emits promote\_to\_long\_term operation in the Control Packet (e.g., rule: avoid\_pattern(unwrap)).  
* **Adoption:** Kernel validates the new rule against the Constitution (e.g., "Does this rule contradict safety?"). If valid, it writes it to **Shard D (Cold Storage)**.  
* **Result:** The agent has permanently learned a preference without fine-tuning, modifying its own Intensional Database (IDB).

## **9.0 DYNAMIC SHARD CONFIGURATION & DEEP RESEARCH**

**Macro-Objective:** Enable the user to define **Persistent Specialists** on the fly via CLI. This module handles the creation and "education" of Type B shards, preventing "Generic Coder Syndrome" where the agent guesses about new frameworks.

### **9.1 Micro-Goal: CLI Agent Definition**

The user must be able to describe a new agent type in natural language, which the system compiles into a Shard Profile.

* **CLI Command:** nerd define-agent \--name "RustExpert" \--topic "Tokio Async Runtime"  
* **Schema Requirement (shard\_profile\_schema):**  
  * **Goal:** Persist the definition of a specialist.  
  * **Fields Needed:**  
    * AgentName: Unique ID (e.g., /agent\_rust).  
    * Description: Natural language mission statement ("You are an expert in non-blocking I/O...").  
    * ResearchKeywords: List of topics to master (e.g., "tokio::select\!", "pinning", "async traits").  
    * AllowedTools: Subset of tools this agent can access (e.g., \["fs\_read", "cargo\_check"\] \- maybe no network allowed after research).  
  * **Logic Constraint:** Defining an agent does *not* spawn it. It creates a profile atom in the EDB and automatically triggers the Research phase.

### **9.2 Micro-Goal: The "Deep Research" Trigger**

Before a specialist can run, it must prove it knows the subject matter.

* **Logic Trigger:**  
  * Rule: needs\_research(Agent) :- shard\_profile(Agent, \_, Topics, \_), not knowledge\_ingested(Agent).  
  * **Effect:** The Kernel blocks instantiation of the Shard and triggers the perform\_deep\_research virtual predicate.  
* **Virtual Predicate (perform\_deep\_research):**  
  * **Input:** Topics list.  
  * **Action:** Go Runtime spawns a temporary "Research Shard" (browser-enabled) to search documentation, summarize best practices, and extract code patterns.  
  * **Workflow:**  
    1. Search Google/Docs for keywords.  
    2. Scrape relevant pages (using a headless browser).  
    3. Transduce content into knowledge\_atoms (summaries, snippets).  
  * **Output:** A stream of knowledge\_atoms (e.g., best\_practice("tokio", "avoid\_blocking\_threads")).

### **9.3 Micro-Goal: Knowledge Ingestion & Pre-Population**

The research results must be "burned" into the Shard's initial state so it starts with a "PhD" in the topic.

* **Schema Requirement (knowledge\_atom\_schema):**  
  * **Goal:** Structured representation of documentation.  
  * **Fields:** SourceURL, Concept, CodePattern, AntiPattern.  
* **Mechanism:**  
  1. **Research:** Researcher Shard yields 500+ atoms.  
  2. **Ingestion:** These atoms are written to a dedicated SQLite file: memory/shards/{AgentName}\_knowledge.db.  
  3. **Spawn:** When nerd spawn RustExpert is called, the Kernel mounts this DB as a read-only "Knowledge Shard" (Shard C) for the new agent, satisfying the **Type B** lifecycle defined in Section 7.0.  
  4. **Verification:** The Kernel runs a "viva voce" (oral exam) — generating synthetic questions from the knowledge base and verifying the Shard can answer them using Mangle queries before marking the Shard as ready.

> *[Archived & Reviewed by The Librarian on 2026-01-23]*
