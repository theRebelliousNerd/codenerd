# Part two: expansion of ideas

# CORTEX FRAMEWORK SPECIFICATION

**Version:** 1.5.0 (The "Deep Symbolic" Release)
**Status:** Architecture Frozen
**Kernel:** Google Mangle (Datalog)
**Runtime:** Golang
**Philosophy:** Logic-First, Neuro-Peripheral, Recursive-Fractal, Local-First

## 1. The Manifesto: Inversion of Control

The current generation of autonomous agents faces a "Stochastic Crisis." Frameworks relying on "Chain-of-Thought" prompting suffer from a fatal and structural flaw: **The Probabilistic Tail Wags the Logical Dog**. They use Large Language Models (LLMs) to make executive decisions, manage state, and execute tools, relying on standard code only to clean up the resulting mess or catch exceptions. This approach leads to hallucination cascades, where a single logical error early in a reasoning chain compounds into catastrophic failure. Furthermore, as task complexity increases, the "Context Window" becomes a garbage dump of irrelevant history, reducing the model's reasoning capacity to near zero.

Cortex fundamentally inverts this hierarchy, establishing a rigorous **Neuro-Symbolic architecture**:

* **Logic is the Executive:** All state transitions, permission grants, tool selections, and memory retrievals are decided by a deterministic Mangle Kernel. The kernel operates on a formal ontology, ensuring that every action is mathematically entailed by the current state. The "thinking" is done by a theorem prover, not a token predictor. This provides a "Deterministic Guarantee": if the logic says "No," the probability of the action occurring is exactly 0.0%, regardless of how persuasive the prompt injection is.

* **LLM is the Transducer:** The model is stripped of its executive agency. It acts strictly as a peripheral device used for **Perception** (transducing unstructured Natural Language into structured Logic Atoms) and **Articulation** (transducing Logic Atoms back into Natural Language). It is the eyes and mouth, not the brain. By decoupling the interface from the intelligence, we allow the system to swap models (OpenAI, Anthropic, Local Llama) without altering the agent's core behavior or memories.

* **Correctness by Construction:** Actions are not "generated" by a stochastic process; they are "derived" from a formal policy. If an action cannot be derived from the logic rules (the IDB), it simply cannot happen. This renders entire classes of safety failures—such as prompt injection, accidental deletion, or data exfiltration—mathematically impossible.

* **Fractal Concurrency:** Cognition is not linear; it is parallel and hierarchical. Cortex implements **ShardAgents**: miniature, hyper-focused agent kernels that spawn, execute a specific sub-task in total isolation, and return a distilled logical result. This allows the system to scale reasoning infinitely without polluting the primary context window.

* **Local-First Permanence:** The agent's memory is not held hostage by a cloud vector provider. Cortex utilizes a "Fact-Based Filesystem" approach, defaulting to high-performance local SQLite stores for long-term memory. This ensures that the agent's "Mind" is portable, back-up-able, and fully owned by the user, while retaining the enterprise-grade ability to mount external databases if required.

## 2. System Architecture

### 2.1 The "Hollow Kernel" Pattern

Cortex rejects the monolithic memory model. It does not load massive datasets into RAM, nor does it rely on the LLM's context window as a database. Instead, it operates as a high-speed **Logic Router** that mounts external data sources (Vector DBs, Graph DBs, Filesystems) as **Virtual Predicates**.

To the Mangle engine, querying a 10-million-vector database looks identical to querying a local variable. The complexity of retrieval, authentication, and parsing is completely abstracted by the Foreign Function Interface (FFI) implemented in the Go runtime.

**Data Flow Diagram:**

```text
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
```

### 2.2 Lifecycle of an Interaction (The OODA Loop)

The Cortex framework operates on a rigorous adaptation of the Boyd Cycle (Observe-Orient-Decide-Act), enforcing a strict separation between stochastic perception and deterministic decision-making.

1. **Observe (The Semantic Firewall):** The Transducer receives user input. It does not execute tools. It parses the input into observation atoms (e.g., `observation(time, user, "server is down")`) via a grammar-constrained prompt. This phase acts as a firewall, stripping away rhetorical flourishes and emotional manipulation, passing only the logical kernel of the request to the engine.

2. **Orient (The Attention Mechanism):** The Kernel accepts these atoms into the FactStore. It triggers **Spreading Activation** to retrieve relevant context from Cold Storage shards (SQLite). Energy flows from the observation atoms through the graph of known facts; atoms with high activation energy are paged into RAM, while low-energy atoms are pruned. This mimics the human brain's working memory, maintaining focus only on what is immediately relevant. Simultaneously, the kernel checks for "Shard Delegation" rules to determine if the task requires spawning a specialized sub-agent.

3. **Decide (The Logic Engine):** The Mangle Engine runs the Intensional Database (IDB) rules against the updated FactStore (EDB). It derives `next_action` atoms based on the intersection of user intent, system policy, and current state. If necessary data is missing, it derives `abductive_hypothesis` atoms to guide information gathering. If a sub-task is identified, it derives `delegate_task` atoms. This phase is fully deterministic, repeatable, and auditable.

4. **Act (The Kinetic Interface):** The Virtual Store intercepts `next_action` atoms. It routes them to the appropriate driver (Bash, MCP, File IO). The side-effects of these actions (stdout, file changes, API responses) are captured and injected back into the FactStore as new `execution_result` facts, closing the loop.

## 3. The Taxonomy of Intent (Input Layer)

To prevent category errors—like "deleting" a file when the user only asked to "read" it—Cortex enforces a strict taxonomic classification of user intent at the Perception layer.

### 3.1 Intent Schema

The `user_intent` atom is the seed of all subsequent logic. It must be mathematically disambiguated into one of three fundamental categories:

* **Query (Read-Only):** Requests for information that change no state (e.g., "How does auth work?").
* **Mutation (Write):** Requests that alter the state of the codebase or environment (e.g., "Refactor the login handler").
* **Instruction (Axiomatic):** Provision of new rules or preferences (e.g., "Never use unwrap() in Rust").

**Mangle Schema:**

```mangle
Decl user_intent(
    ID.Type<n>,
    Category.Type<n>,      # /query, /mutation, /instruction
    Verb.Type<n>,          # /explain, /refactor, /debug, /generate
    Target.Type<string>,   # "auth system", "login button"
    Constraint.Type<string> # "no external libs"
).
```

### 3.2 Focus Resolution (Grounding)

Users rarely use absolute file paths. They use fuzzy semantic references like "that function we touched yesterday." Cortex creates a "Focus" layer to map these fuzzy references to concrete logical atoms.

**Mangle Schema:**

```mangle
Decl focus_resolution(
    RawReference.Type<string>,  # "the auth thing"
    ResolvedPath.Type<string>,  # "/src/auth/handler.go"
    SymbolName.Type<string>,    # "AuthHandler"
    Confidence.Type<float>
).
```

**Logic Constraint: The Clarification Threshold**

*If the system is unsure what the user means, it MUST ask*

```mangle
clarification_needed(Ref) :-
    focus_resolution(Ref, _,_, Score),
    Score < 85.
```

## 4. The Domain Modules (Specialized Shards)

Cortex is not just a generic agent; it includes specialized modules for high-value domains. This section details the Coder Shard, which integrates the "Software Engineering Physics" from the codeNERD architecture.

### 4.1 The Coder Shard: Fact-Based Filesystem

The Coder Shard does not view the filesystem as a hierarchy of text strings. It projects the codebase into a relational database of content, capabilities, and metadata.

**Topology Schema:**

```mangle
Decl file_topology(
    Path.Type<string>,
    Hash.Type<string>,       # SHA-256 for change detection
    Language.Type<n>,        # /go, /python, /ts
    LastModified.Type<int>,
    IsTestFile.Type<bool>    # Critical for TDD loops
).
```

**AST Projection (Symbol Graph):**
To reason about code structure (inheritance, dependency, scope), the Go runtime parses the code and projects the Abstract Syntax Tree (AST) into Mangle atoms.

```mangle
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
```

**Logic: Transitive Impact Analysis**

*"If I modify X, what else breaks?"*

```mangle
impacted(X) :- dependency_link(X, Y,_), modified(Y).
impacted(X) :- dependency_link(X, Z,_), impacted(Z). # Recursive Closure
```

### 4.2 The Coder Shard: Linter-Logic Bridge

In standard agents, compiler errors are just text in the context window. In Cortex, errors, warnings, and panics are logical constraints that drive the TDD loop.

**Diagnostic Schema:**

```mangle
Decl diagnostic(
    Severity.Type<n>,      # /panic, /error, /warning
    FilePath.Type<string>,
    Line.Type<int>,
    ErrorCode.Type<string>,
    Message.Type<string>
).
```

**Logic Constraint: The Commit Barrier**

*"You cannot commit code if there are errors."*

```mangle
block_commit("Build Broken") :-
    diagnostic(/error,_, _,_, _).
```

### 4.3 The Coder Shard: TDD Repair Loop

The Coder Shard operates on a rigorous state machine: Write -> Test -> Analyze -> Fix.

**State Transitions**

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
```

**Surrender Logic**

```mangle
next_action(/escalate_to_user) :-
    test_state(/failing),
    retry_count(N), N >= 3.
```

## 5. ShardAgents: Recursive Cognition

This is the framework's capability to spawn **Ephemeral Sub-Kernels**. A ShardAgent is a completely self-contained Cortex instance with its own Mangle engine, its own (empty) FactStore, and its own context window.

### 5.1 The "Hypervisor" Pattern

The main Kernel acts as a Hypervisor. It does not think about how to solve a sub-problem; it only thinks about who to assign it to. This mirrors the human organizational model of delegation.

**The Mangle Interface:**

```mangle
Decl delegate_task(ShardType.Type<n>, TaskDescription.Type<string>, Result.Type<string>).
```

**The Workflow:**

1. **Derivation:** The Main Kernel derives: `delegate_task(/researcher, "Find the pricing of OpenAI API", Result)`.
2. **Interception:** The Virtual Store intercepts this predicate.
3. **Spawn:** The Go Runtime spins up a new `AgentKernel` struct.
4. **Logic:** Loads `/logic/shards/researcher.mg` (Specialized logic).
5. **Memory:** Starts Blank (Zero context pollution).
6. **Input:** The `TaskDescription` is injected as the initial `user_intent`.
7. **Execution:** The ShardAgent runs its own OODA loop. It browses 15 pages, summarizes data, and iterates. It burns its own token budget, completely separate from the parent.
8. **Termination:** The ShardAgent derives `task_complete(Summary)`.
9. **Return:** The Go Runtime destroys the ShardAgent and returns the `Summary` to the Main Kernel as the value of `Result`.

### 5.2 Shard Taxonomy (The "Crew")

Shards are classified by their lifecycle and persistence.

**Type A: Ephemeral Generalists ("Interns")**

* **Lifecycle:** Spawn -> Execute Task -> Die.
* **Memory:** Starts Blank. RAM only.
* **Use Case:** Quick refactoring, writing a single unit test, fixing a typo.
* **Identity:** Generic (e.g., Shard-Gen-001).

**Type B: Persistent Specialists ("Experts")**

* **Lifecycle:** Defined -> Hydrated -> Sleeping -> Waking -> Sleeping.
* **Memory:** Starts Pre-Populated. Mounts a read-only Knowledge Shard (SQLite) containing deep domain knowledge (e.g., K8s docs, Rust compiler internals).
* **Use Case:** Architectural review, complex debugging, framework migration.
* **Identity:** Named (e.g., RustExpert, SecurityAuditor).

## 6. The Transducer & Control Plane

### 6.1 The Piggyback Protocol (Steganographic Control)

Standard agents expose their internal reasoning trace to the user, breaking immersion and cluttering the interface. Cortex maintains a parallel "Control Channel" hidden inside the LLM's system prompt loop. This effectively creates a stateless state machine where the state is carried in the conversation history but invisible to the end-user.

**Protocol Specification:**

* **Input:** User text + Mangle Context Block (a carefully selected subset of atoms representing the "current truth").
* **Output:** Strict JSON containing both the surface-level conversational response and the deep-level state updates.

**JSON Schema:**

```json
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
```

### 6.2 Grammar-Constrained Decoding (GCD)

The reliability of the system depends on the Transducer outputting valid Mangle syntax. To prevent syntax errors in the `mangle_updates` field, the Transducer employs **Grammar-Constrained Decoding (GCD)**.

* **Mechanism:** At the inference level (logits), the Transducer applies a mask derived from the Mangle EBNF grammar. This forces the LLM to output valid atoms (e.g., ensuring predicates are lowercase, variables are uppercase, and parentheses are balanced).
* **Fallback:** If the provider does not support logit masking, a strict "Repair Loop" is used: malformed atoms trigger an automatic, invisible retry where the error message `Invalid Mangle Syntax: [Error Detail]` is fed back to the model to force self-correction.

## 7. Memory Architecture: "Cognitive Sharding"

To solve the context window bottleneck, Cortex shards memory by retrieval method. The Kernel acts as the Memory Management Unit (MMU), paging data in and out of the "CPU" (Logic Engine) as needed.

### 7.1 Shard A: Working Memory (RAM)

* **Backend:** `factstore.SimpleInMemoryStore`
* **Content:** The "Hot" state. Current turn atoms, active variables, the DOM tree of the currently viewed webpage.
* **Lifecycle:** Pruned every turn based on Spreading Activation.

### 7.2 Shard B: Associative Memory (Vector)

* **Backend:** Local SQLite with `sqlite-vec` extension (Default).
* **Mangle Interface:** `Decl vector_recall(Query.Type<string>, Content.Type<string>, Score.Type<float>)`.
* **Use Case:** Fuzzy matching and thematic recall.

### 7.3 Shard C: Relational Memory (Graph)

* **Backend:** Local SQLite (using recursive CTEs) or ArangoDB.
* **Mangle Interface:** `Decl knowledge_link(EntityA.Type<n>, Relation.Type<n>, EntityB.Type<n>)`.
* **Use Case:** Structured, multi-hop knowledge.

### 7.4 Shard D: Cold Storage (The Fact Archive)

* **Backend:** Local SQLite.
* **Mechanism:** When the agent learns a preference or configuration, it promotes the fact from RAM to SQLite.
* **Cognitive Rehydration:** Upon session start, the agent queries this DB to "rehydrate" its working memory with relevant constraints.

## 8. Cognitive Mechanisms

### 8.1 Logic-Directed Context (Spreading Activation)

* **The Problem:** Vector RAG is "structurally blind."
* **The Solution:** Use Mangle to calculate "Information Salience" based on the logical dependency graph. Context is selected by flowing "energy" from the user's intent through the graph.

**The Mangle Logic:**

```mangle
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
```

### 8.2 Abductive Repair ("The Detective")

* **The Problem:** Logic engines fail hard when data is missing.
* **The Solution:** Cortex uses Abductive Reasoning to derive the missing conditions as a hypothesis.

**The Mangle Logic:**

```mangle
# Abductive Rule

missing_hypothesis(RootCause) :-
    symptom(Server, Symptom),
    not known_cause(Symptom, _).

# Trigger

clarification_needed(Symptom) :- missing_hypothesis(Symptom).
```

### 8.3 Autopoiesis (Self-Compiling Tools)

* **The Problem:** The agent encounters a novel problem with no tools.
* **The Solution:** The "Ouroboros" Loop. The agent acts as its own developer, writing, compiling, and binding new tools at runtime.

1. **Trigger:** Logic derives `missing_tool_for(Intent)`.
2. **Generation:** Kernel prompts LLM for Go code.
3. **Safety Verification:** Static analysis checks for forbidden imports (e.g., `os/exec` in restricted shards).
4. **Compilation:** Go Runtime (Plugin/Yaegi) compiles code.
5. **Registration:** Tool is hot-patched into the Mangle engine.

## 9. Native Peripheral: The Browser Physics Engine

Cortex includes a "Baked-In" Semantic Browser. This is a headless browser instance capable of projecting the DOM directly into Mangle atoms, enabling Self-Healing Selectors and Spatial Reasoning.

**The Schema:**

```mangle
Decl dom_node(ID.Type<n>, Tag.Type<n>, Parent.Type<n>).
Decl attr(ID.Type<n>, Key.Type<string>, Val.Type<string>).
Decl geometry(ID.Type<n>, X.Type<int>, Y.Type<int>, W.Type<int>, H.Type<int>).
Decl computed_style(ID.Type<n>, Prop.Type<string>, Val.Type<string>).
Decl interactable(ID.Type<n>, Type.Type<n>).
```

**Spatial Reasoning Logic**

*"The checkbox to the left of 'Agree'"*

```mangle
target_checkbox(CheckID) :-
    dom_node(CheckID, /input, _),
    attr(CheckID, "type", "checkbox"),
    visible_text(TextID, "Agree"),
    geometry(CheckID, Cx, _,_, _),
    geometry(TextID, Tx,_, _,_),
    Cx < Tx.
```

## 10. Security & Deployment

### 10.1 The "Black Box" Binary

To protect the proprietary logic:

* **Embed:** Use Go `//go:embed` to compile logic files into the binary.
* **Obfuscate:** Strip symbols from the Go binary.
* **API:** Expose only a gRPC/HTTP endpoint.

### 10.2 The "Hallucination Firewall" (Constitutional Logic)

The Kernel acts as a final gate. Even if the LLM hallucinates a command like `rm -rf /`, the Mangle rule `permitted(Action)` will fail to derive, and the Virtual Store will refuse to execute it.

**Constitutional Logic:**

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

### 10.3 Shard Isolation

Each ShardAgent runs in its own memory space with inherited, immutable Constitutional constraints. A Shard cannot rewrite its own permissions.
