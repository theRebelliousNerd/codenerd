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
    {  
      "surface\_response": "Text visible to the user (e.g., 'I fixed the bug.').",  
      "control\_packet": {  
        "reasoning\_trace": "Transient Chain-of-Thought (discarded after verification).",  
        "mangle\_updates": \["atom(/task\_status, /complete)", "atom(/file\_state, /modified)"\],  
        "memory\_operations": \[  
          {"operation": "promote\_to\_long\_term", "fact": "preference(/user, /concise)"},  
          {"operation": "forget", "target": "error(/transient\_network\_fail)"}  
        \],  
        "self\_correction": {"trigger": "failure", "hypothesis": "missing\_dependency"}  
      }  
    }

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

  # Part two: expansion of ideas

  CORTEX FRAMEWORK SPECIFICATION

Version: 1.5.0 (The "Deep Symbolic" Release)
Status: Architecture Frozen
Kernel: Google Mangle (Datalog)
Runtime: Golang
Philosophy: Logic-First, Neuro-Peripheral, Recursive-Fractal, Local-First

1. The Manifesto: Inversion of Control

The current generation of autonomous agents faces a "Stochastic Crisis." Frameworks relying on "Chain-of-Thought" prompting suffer from a fatal and structural flaw: The Probabilistic Tail Wags the Logical Dog. They use Large Language Models (LLMs) to make executive decisions, manage state, and execute tools, relying on standard code only to clean up the resulting mess or catch exceptions. This approach leads to hallucination cascades, where a single logical error early in a reasoning chain compounds into catastrophic failure. Furthermore, as task complexity increases, the "Context Window" becomes a garbage dump of irrelevant history, reducing the model's reasoning capacity to near zero.

Cortex fundamentally inverts this hierarchy, establishing a rigorous Neuro-Symbolic architecture:

Logic is the Executive: All state transitions, permission grants, tool selections, and memory retrievals are decided by a deterministic Mangle Kernel. The kernel operates on a formal ontology, ensuring that every action is mathematically entailed by the current state. The "thinking" is done by a theorem prover, not a token predictor. This provides a "Deterministic Guarantee": if the logic says "No," the probability of the action occurring is exactly 0.0%, regardless of how persuasive the prompt injection is.

LLM is the Transducer: The model is stripped of its executive agency. It acts strictly as a peripheral device used for Perception (transducing unstructured Natural Language into structured Logic Atoms) and Articulation (transducing Logic Atoms back into Natural Language). It is the eyes and mouth, not the brain. By decoupling the interface from the intelligence, we allow the system to swap models (OpenAI, Anthropic, Local Llama) without altering the agent's core behavior or memories.

Correctness by Construction: Actions are not "generated" by a stochastic process; they are "derived" from a formal policy. If an action cannot be derived from the logic rules (the IDB), it simply cannot happen. This renders entire classes of safety failures—such as prompt injection, accidental deletion, or data exfiltration—mathematically impossible.

Fractal Concurrency: Cognition is not linear; it is parallel and hierarchical. Cortex implements ShardAgents: miniature, hyper-focused agent kernels that spawn, execute a specific sub-task in total isolation, and return a distilled logical result. This allows the system to scale reasoning infinitely without polluting the primary context window.

Local-First Permanence: The agent's memory is not held hostage by a cloud vector provider. Cortex utilizes a "Fact-Based Filesystem" approach, defaulting to high-performance local SQLite stores for long-term memory. This ensures that the agent's "Mind" is portable, back-up-able, and fully owned by the user, while retaining the enterprise-grade ability to mount external databases if required.

2. System Architecture

2.1 The "Hollow Kernel" Pattern

Cortex rejects the monolithic memory model. It does not load massive datasets into RAM, nor does it rely on the LLM's context window as a database. Instead, it operates as a high-speed Logic Router that mounts external data sources (Vector DBs, Graph DBs, Filesystems) as Virtual Predicates.

To the Mangle engine, querying a 10-million-vector database looks identical to querying a local variable. The complexity of retrieval, authentication, and parsing is completely abstracted by the Foreign Function Interface (FFI) implemented in the Go runtime.

Data Flow Diagram:

[ Terminal / User ]
       ↕
[ Transducer (LLM Client) ] <==> [ API: Anthropic/OpenAI ]
       ↓ (Control Stream: JSON Atoms)
[ Cortex Kernel (Primary) ]
       |
       +--> [ FactStore (RAM) ]: "Working Memory" (Variables, Current Intent, Active DOM)
       |
       +--> [ Mangle Engine ]: "The CPU" (Runs .mg logic to fixpoint)
       |         ↕ (Query / Derivation)
       |
       +--> [ Virtual Store (FFI Router) ]
                 |
                 +--> [ Shard A: Filesystem ] (Tools, Project Code, Logs)
                 +--> [ Shard B: Vector DB ] (Long-term Associative Context)
                 +--> [ Shard C: Graph DB ] (Relational Knowledge / Ontology)
                 +--> [ Shard D: Native Peripherals ] (Baked-in Browser, Docker)
                 |
                 +--> [ SHARD MANAGER ] (The Hypervisor)
                           |
                           +--> [ ShardAgent 1: Researcher ] (Own Kernel + Context)
                           +--> [ ShardAgent 2: Coder ] (Own Kernel + Context)

2.2 Lifecycle of an Interaction (The OODA Loop)

The Cortex framework operates on a rigorous adaptation of the Boyd Cycle (Observe-Orient-Decide-Act), enforcing a strict separation between stochastic perception and deterministic decision-making.

Observe (The Semantic Firewall): The Transducer receives user input. It does not execute tools. It parses the input into observation atoms (e.g., observation(time, user, "server is down")) via a grammar-constrained prompt. This phase acts as a firewall, stripping away rhetorical flourishes and emotional manipulation, passing only the logical kernel of the request to the engine.

Orient (The Attention Mechanism): The Kernel accepts these atoms into the FactStore. It triggers Spreading Activation to retrieve relevant context from Cold Storage shards (SQLite). Energy flows from the observation atoms through the graph of known facts; atoms with high activation energy are paged into RAM, while low-energy atoms are pruned. This mimics the human brain's working memory, maintaining focus only on what is immediately relevant. Simultaneously, the kernel checks for "Shard Delegation" rules to determine if the task requires spawning a specialized sub-agent.

Decide (The Logic Engine): The Mangle Engine runs the Intensional Database (IDB) rules against the updated FactStore (EDB). It derives next_action atoms based on the intersection of user intent, system policy, and current state. If necessary data is missing, it derives abductive_hypothesis atoms to guide information gathering. If a sub-task is identified, it derives delegate_task atoms. This phase is fully deterministic, repeatable, and auditable.

Act (The Kinetic Interface): The Virtual Store intercepts next_action atoms. It routes them to the appropriate driver (Bash, MCP, File IO). The side-effects of these actions (stdout, file changes, API responses) are captured and injected back into the FactStore as new execution_result facts, closing the loop.

3. The Taxonomy of Intent (Input Layer)

To prevent category errors—like "deleting" a file when the user only asked to "read" it—Cortex enforces a strict taxonomic classification of user intent at the Perception layer.

3.1 Intent Schema

The user_intent atom is the seed of all subsequent logic. It must be mathematically disambiguated into one of three fundamental categories:

Query (Read-Only): Requests for information that change no state (e.g., "How does auth work?").

Mutation (Write): Requests that alter the state of the codebase or environment (e.g., "Refactor the login handler").

Instruction (Axiomatic): Provision of new rules or preferences (e.g., "Never use unwrap() in Rust").

Mangle Schema:

Decl user_intent(
    ID.Type<n>,
    Category.Type<n>,      # /query, /mutation, /instruction
    Verb.Type<n>,          # /explain, /refactor, /debug, /generate
    Target.Type<string>,   # "auth system", "login button"
    Constraint.Type<string> # "no external libs"
).

3.2 Focus Resolution (Grounding)

Users rarely use absolute file paths. They use fuzzy semantic references like "that function we touched yesterday." Cortex creates a "Focus" layer to map these fuzzy references to concrete logical atoms.

Mangle Schema:

Decl focus_resolution(
    RawReference.Type<string>,  # "the auth thing"
    ResolvedPath.Type<string>,  # "/src/auth/handler.go"
    SymbolName.Type<string>,    # "AuthHandler"
    Confidence.Type<float>
).

# Logic Constraint: The Clarification Threshold

# If the system is unsure what the user means, it MUST ask

clarification_needed(Ref) :-
    focus_resolution(Ref, *,*, Score),
    Score < 0.85.

4. The Domain Modules (Specialized Shards)

Cortex is not just a generic agent; it includes specialized modules for high-value domains. This section details the Coder Shard, which integrates the "Software Engineering Physics" from the codeNERD architecture.

4.1 The Coder Shard: Fact-Based Filesystem

The Coder Shard does not view the filesystem as a hierarchy of text strings. It projects the codebase into a relational database of content, capabilities, and metadata.

Topology Schema:

Decl file_topology(
    Path.Type<string>,
    Hash.Type<string>,       # SHA-256 for change detection
    Language.Type<n>,        # /go, /python, /ts
    LastModified.Type<int>,
    IsTestFile.Type<bool>    # Critical for TDD loops
).

AST Projection (Symbol Graph):
To reason about code structure (inheritance, dependency, scope), the Go runtime parses the code and projects the Abstract Syntax Tree (AST) into Mangle atoms.

Decl symbol_graph(
    SymbolID.Type<string>,   # "func:main:AuthHandler"
    Type.Type<n>,            # /function, /class, /interface
    Visibility.Type<n>,      # /public, /private
    DefinedAt.Type<string>,  # Path
    Signature.Type<string>
).

Decl dependency_link(
    CallerID.Type<string>,
    CalleeID.Type<string>,
    ImportPath.Type<string>
).

# Logic: Transitive Impact Analysis

# "If I modify X, what else breaks?"

impacted(X) :- dependency_link(X, Y,*), modified(Y).
impacted(X) :- dependency_link(X, Z,*), impacted(Z). # Recursive Closure

4.2 The Coder Shard: Linter-Logic Bridge

In standard agents, compiler errors are just text in the context window. In Cortex, errors, warnings, and panics are logical constraints that drive the TDD loop.

Diagnostic Schema:

Decl diagnostic(
    Severity.Type<n>,      # /panic, /error, /warning
    FilePath.Type<string>,
    Line.Type<int>,
    ErrorCode.Type<string>,
    Message.Type<string>
).

# Logic Constraint: The Commit Barrier

# "You cannot commit code if there are errors."

block_commit("Build Broken") :-
    diagnostic(/error,_, *,*, _).

4.3 The Coder Shard: TDD Repair Loop

The Coder Shard operates on a rigorous state machine: Write -> Test -> Analyze -> Fix.

# State Transitions

next_action(/read_error_log) :-
    test_state(/failing),
    retry_count(N), N < 3.

next_action(/analyze_root_cause) :-
    test_state(/log_read).

next_action(/generate_patch) :-
    test_state(/cause_found).

next_action(/run_tests) :-
    test_state(/patch_applied).

# Surrender Logic

next_action(/escalate_to_user) :-
    test_state(/failing),
    retry_count(N), N >= 3.

5. ShardAgents: Recursive Cognition

This is the framework's capability to spawn Ephemeral Sub-Kernels. A ShardAgent is a completely self-contained Cortex instance with its own Mangle engine, its own (empty) FactStore, and its own context window.

5.1 The "Hypervisor" Pattern

The main Kernel acts as a Hypervisor. It does not think about how to solve a sub-problem; it only thinks about who to assign it to. This mirrors the human organizational model of delegation.

The Mangle Interface:
Decl delegate_task(ShardType.Type<n>, TaskDescription.Type<string>, Result.Type<string>).

The Workflow:

Derivation: The Main Kernel derives: delegate_task(/researcher, "Find the pricing of OpenAI API", Result).

Interception: The Virtual Store intercepts this predicate.

Spawn: The Go Runtime spins up a new AgentKernel struct.

Logic: Loads /logic/shards/researcher.mg (Specialized logic).

Memory: Starts Blank (Zero context pollution).

Input: The TaskDescription is injected as the initial user_intent.

Execution: The ShardAgent runs its own OODA loop. It browses 15 pages, summarizes data, and iterates. It burns its own token budget, completely separate from the parent.

Termination: The ShardAgent derives task_complete(Summary).

Return: The Go Runtime destroys the ShardAgent and returns the Summary to the Main Kernel as the value of Result.

5.2 Shard Taxonomy (The "Crew")

Shards are classified by their lifecycle and persistence.

Type A: Ephemeral Generalists ("Interns")

Lifecycle: Spawn -> Execute Task -> Die.

Memory: Starts Blank. RAM only.

Use Case: Quick refactoring, writing a single unit test, fixing a typo.

Identity: Generic (e.g., Shard-Gen-001).

Type B: Persistent Specialists ("Experts")

Lifecycle: Defined -> Hydrated -> Sleeping -> Waking -> Sleeping.

Memory: Starts Pre-Populated. Mounts a read-only Knowledge Shard (SQLite) containing deep domain knowledge (e.g., K8s docs, Rust compiler internals).

Use Case: Architectural review, complex debugging, framework migration.

Identity: Named (e.g., RustExpert, SecurityAuditor).

6. The Transducer & Control Plane

6.1 The Piggyback Protocol (Steganographic Control)

Standard agents expose their internal reasoning trace to the user, breaking immersion and cluttering the interface. Cortex maintains a parallel "Control Channel" hidden inside the LLM's system prompt loop. This effectively creates a stateless state machine where the state is carried in the conversation history but invisible to the end-user.

Protocol Specification:

Input: User text + Mangle Context Block (a carefully selected subset of atoms representing the "current truth").

Output: Strict JSON containing both the surface-level conversational response and the deep-level state updates.

JSON Schema:

{
  "surface_response": "I have updated the server configuration and restarted the service.",
  "control_packet": {
    "mangle_updates": [
      "user_intent(/update_config, \"prod_server\")",
      "observation(/server_status, /healthy)",
      "tool_result(/exec_cmd, \"success\")"
    ],
    "memory_operations": [
      { "op": "promote_to_long_term", "fact": "preference(/user, /concise)" }
    ],
    "abductive_hypothesis": "missing_fact(/sudo_access)"
  }
}

6.2 Grammar-Constrained Decoding (GCD)

The reliability of the system depends on the Transducer outputting valid Mangle syntax. To prevent syntax errors in the mangle_updates field, the Transducer employs Grammar-Constrained Decoding (GCD).

Mechanism: At the inference level (logits), the Transducer applies a mask derived from the Mangle EBNF grammar. This forces the LLM to output valid atoms (e.g., ensuring predicates are lowercase, variables are uppercase, and parentheses are balanced).

Fallback: If the provider does not support logit masking, a strict "Repair Loop" is used: malformed atoms trigger an automatic, invisible retry where the error message Invalid Mangle Syntax: [Error Detail] is fed back to the model to force self-correction.

7. Memory Architecture: "Cognitive Sharding"

To solve the context window bottleneck, Cortex shards memory by retrieval method. The Kernel acts as the Memory Management Unit (MMU), paging data in and out of the "CPU" (Logic Engine) as needed.

7.1 Shard A: Working Memory (RAM)

Backend: factstore.SimpleInMemoryStore

Content: The "Hot" state. Current turn atoms, active variables, the DOM tree of the currently viewed webpage.

Lifecycle: Pruned every turn based on Spreading Activation.

7.2 Shard B: Associative Memory (Vector)

Backend: Local SQLite with sqlite-vec extension (Default).

Mangle Interface: Decl vector_recall(Query.Type<string>, Content.Type<string>, Score.Type<float>).

Use Case: Fuzzy matching and thematic recall.

7.3 Shard C: Relational Memory (Graph)

Backend: Local SQLite (using recursive CTEs) or ArangoDB.

Mangle Interface: Decl knowledge_link(EntityA.Type<n>, Relation.Type<n>, EntityB.Type<n>).

Use Case: Structured, multi-hop knowledge.

7.4 Shard D: Cold Storage (The Fact Archive)

Backend: Local SQLite.

Mechanism: When the agent learns a preference or configuration, it promotes the fact from RAM to SQLite.

Cognitive Rehydration: Upon session start, the agent queries this DB to "rehydrate" its working memory with relevant constraints.

8. Cognitive Mechanisms

8.1 Logic-Directed Context (Spreading Activation)

The Problem: Vector RAG is "structurally blind."
The Solution: Use Mangle to calculate "Information Salience" based on the logical dependency graph. Context is selected by flowing "energy" from the user's intent through the graph.

The Mangle Logic:

# 1. Base Activation (Recency)

activation(Fact, 100) :- new_fact(Fact).

# 2. Spreading Activation (Dependency)

# Energy flows from goals to required tools

activation(Tool, 80) :-
    active_goal(Goal),
    tool_capabilities(Tool, Cap),
    goal_requires(Goal, Cap).

# 3. Recursive Spread (Associations)

activation(FileB, 50) :-
    activation(FileA, Score),
    Score > 40,
    dependency_link(FileA, FileB).

# 4. Context Pruning

context_atom(Fact) :-
    activation(Fact, Score),
    Score > 30.

8.2 Abductive Repair ("The Detective")

The Problem: Logic engines fail hard when data is missing.
The Solution: Cortex uses Abductive Reasoning to derive the missing conditions as a hypothesis.

The Mangle Logic:

# Abductive Rule

missing_hypothesis(RootCause) :-
    symptom(Server, Symptom),
    not known_cause(Symptom, _).

# Trigger

clarification_needed(Symptom) :- missing_hypothesis(Symptom).

8.3 Autopoiesis (Self-Compiling Tools)

The Problem: The agent encounters a novel problem with no tools.
The Solution: The "Ouroboros" Loop. The agent acts as its own developer, writing, compiling, and binding new tools at runtime.

Trigger: Logic derives missing_tool_for(Intent).

Generation: Kernel prompts LLM for Go code.

Safety Verification: Static analysis checks for forbidden imports (e.g., os/exec in restricted shards).

Compilation: Go Runtime (Plugin/Yaegi) compiles code.

Registration: Tool is hot-patched into the Mangle engine.

9. Native Peripheral: The Browser Physics Engine

Cortex includes a "Baked-In" Semantic Browser. This is a headless browser instance capable of projecting the DOM directly into Mangle atoms, enabling Self-Healing Selectors and Spatial Reasoning.

The Schema:

Decl dom_node(ID.Type<n>, Tag.Type<n>, Parent.Type<n>).
Decl attr(ID.Type<n>, Key.Type<string>, Val.Type<string>).
Decl geometry(ID.Type<n>, X.Type<int>, Y.Type<int>, W.Type<int>, H.Type<int>).
Decl computed_style(ID.Type<n>, Prop.Type<string>, Val.Type<string>).
Decl interactable(ID.Type<n>, Type.Type<n>).

# Spatial Reasoning Logic

# "The checkbox to the left of 'Agree'"

target_checkbox(CheckID) :-
    dom_node(CheckID, /input, _),
    attr(CheckID, "type", "checkbox"),
    visible_text(TextID, "Agree"),
    geometry(CheckID, Cx, *,*, *),
    geometry(TextID, Tx,*, *,*),
    Cx < Tx.

10. Security & Deployment

10.1 The "Black Box" Binary

To protect the proprietary logic:

Embed: Use Go //go:embed to compile logic files into the binary.

Obfuscate: Strip symbols from the Go binary.

API: Expose only a gRPC/HTTP endpoint.

10.2 The "Hallucination Firewall" (Constitutional Logic)

The Kernel acts as a final gate. Even if the LLM hallucinates a command like rm -rf /, the Mangle rule permitted(Action) will fail to derive, and the Virtual Store will refuse to execute it.

Constitutional Logic:

permitted(Action) :- safe_action(Action).

permitted(Action) :-
    dangerous_action(Action),
    admin_override(User),
    signed_approval(Action).

dangerous_action(Action) :-
    action_type(Action, /exec_cmd),
    cmd_string(Action, Cmd),
    fn:string_contains(Cmd, "rm").

10.3 Shard Isolation

Each ShardAgent runs in its own memory space with inherited, immutable Constitutional constraints. A Shard cannot rewrite its own permissions.
