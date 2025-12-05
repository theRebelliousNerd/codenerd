# codeNERD Architecture Reference

## The Creative-Executive Partnership: Theoretical Foundations

### The Category Error

Current AI agents make a fundamental mistake: they ask LLMs to handle everything—creativity AND planning, insight AND memory, problem-solving AND self-correction. But LLMs excel at the former while struggling with the latter. This creates:

- **Unfaithful Reasoning**: LLMs generate convincing Chain-of-Thought that concludes with hallucinated actions contradicting their own reasoning
- **Context Window Exhaustion**: Planning, memory, and state management consume tokens that should be reserved for creative problem-solving
- **Hallucination Cascades**: Without executive infrastructure, creative genius devolves into confident nonsense

### The Creative-Executive Partnership

codeNERD separates concerns to **liberate** each component for what it does best:

| Component | Strength | Role in codeNERD |
|-----------|----------|------------------|
| **LLM (Creative Center)** | Problem-solving, insight, synthesis, novel approaches | Perception (understanding), Articulation (explanation), Solution generation |
| **Mangle Kernel (Executive)** | Consistency, memory, planning, safety | State management, orchestration, permission enforcement, learning persistence |

This isn't about limiting the LLM—it's about freeing it from tasks it's bad at so it can focus purely on creative work.

### Why Mangle (Datalog) as Executive

The executive layer must be deterministic, auditable, and reliable—the opposite of stochastic generation. Mangle provides:

**Deterministic Guarantees**:

- Bottom-up evaluation guarantees termination (no infinite loops)
- Order of rules does not affect outcome (composable, modular policies)
- Perfect for dynamically generated rules from LLM-proposed improvements

**Expressive Power**:

- Native recursion for transitive closures (impact analysis, dependency graphs)
- Aggregation (sum, count, max) for budget tracking and preference signals
- Namespaces for safe self-compiling tool generation

**Safety by Design**:

- If `permitted(Action)` cannot be derived, the action is blocked—0% probability
- Constitutional rules cannot be overridden by prompt injection
- All decisions are auditable and reproducible

## The Hollow Kernel Pattern

The kernel does not load massive datasets into RAM or use the LLM context as database. It operates as a high-speed Logic Router mounting external data sources as Virtual Predicates.

```
To the Mangle engine:
  - Querying a 10-million-vector database
  - Querying a local variable
Look identical. The FFI abstracts all complexity.
```

## The OODA Loop (Lifecycle of Interaction)

### 1. Observe (Semantic Firewall)

The Transducer receives user input and parses it into observation atoms:
```mangle
observation(time, user, "server is down")
```

This phase strips rhetorical flourishes and emotional manipulation, passing only the logical kernel.

### 2. Orient (Attention Mechanism)

The Kernel:
1. Accepts atoms into FactStore
2. Triggers Spreading Activation for context retrieval
3. Energy flows from observations through the fact graph
4. High-energy atoms paged into RAM, low-energy pruned
5. Checks Shard Delegation rules

### 3. Decide (Logic Engine)

Mangle runs IDB rules against EDB:
- Derives `next_action` atoms from intent + policy + state
- Derives `abductive_hypothesis` if data missing
- Derives `delegate_task` for sub-agent spawning

This phase is fully deterministic, repeatable, and auditable.

### 4. Act (Kinetic Interface)

Virtual Store intercepts `next_action` atoms:
- Routes to appropriate driver (Bash, MCP, File IO)
- Captures side-effects as new `execution_result` facts
- Closes the loop

## Logical Context Selection (Spreading Activation)

### Why Not Vector RAG

Vector retrieval is structurally blind:
- Cannot distinguish "A supervises B" from "B supervises A"
- Provides no provenance for why a document was retrieved
- Struggles with multi-hop reasoning

### Context-Directed Spreading Activation (CDSA)

1. **Seed Identification**: LLM identifies entities in query -> Seed Nodes
2. **Energy Injection**: Initial activation (1.0) assigned to seeds
3. **Propagation**: Activation flows from node i to neighbor j:
   ```
   A_j(t+1) = A_j(t) + W_ij * A_i(t) * Decay
   ```
4. **Logical Gating**: Edge weights dynamically adjusted by context:
   ```mangle
   weight(From, To) = 0.9 :- context("security"), edge_type(From, To, "dependency").
   weight(From, To) = 0.1 :- context("security"), edge_type(From, To, "authored_by").
   ```
5. **Retrieval**: Select subgraph with Activation > threshold

### The Anti-Vector Advantage: Provenance

When the agent retrieves a fact, it can trace the activation path:
```
"I retrieved 'Vulnerability CVE-2024' because:
 'Project X' depends on 'Lib Y' which has 'Vulnerability CVE-2024'"
```

This explainability is mathematically impossible with vector similarity scores.

## Virtual Predicates (FFI)

### Abstracting Tools into Logic

- **Standard Predicate**: `file_size("report.txt", 1024)` (stored)
- **Virtual Predicate**: `current_weather("London", Temp)` (computed on demand)

From the Mangle rules perspective, there is no difference:
```mangle
safe_to_fly(City) :- current_weather(City, "Sunny").
```

### FFI Mechanism

1. **Declaration**:
   ```
   @virtual(handler="weather_api")
   current_weather(City: string, Condition: string).
   ```

2. **Query Execution**:
   - Suspends logical derivation
   - Collects bound arguments
   - Invokes registered handler
   - Injects result as temporary fact
   - Resumes derivation

### Piggybacked Safety

Before exec_cmd is triggered, Mangle requires proof of safety:
```mangle
exec_cmd(Cmd) :- authorized(User), in_allowlist(Cmd).
```

If `authorized(User)` cannot be proven, the virtual predicate is never reached.

## Autopoiesis (Self-Compiling Tools)

### Neural-Symbolic ILP

The agent can write new Mangle rules using Inductive Logic Programming:

1. **Intent Perception**: User asks for capability agent lacks
2. **Hypothesis Generation**: LLM proposes new rule
3. **Formal Verification**:
   - Syntax Check: Valid Mangle?
   - Schema Check: Predicates exist?
   - Stratification Check: No negation cycles?
4. **Adoption**: Valid rule hot-loaded into IDB

### Grey Goo Prevention

- **Constitutional Logic**: IDB divided into Mutable/Immutable strata
- **Sandboxing**: New rules simulated against historical EDB before promotion

## The Piggyback Protocol (Steganographic Control)

### The Hidden Control Channel

The system maintains a parallel Control Channel hidden in the LLM interaction:

**Input Stream**:
- Component A: User text
- Component B: Mangle Context Block (current truth)
- Component C: Hidden Directive (forces dual payload schema)

**Output Stream**:
```json
{
  "surface_response": "User-visible text",
  "control_packet": {
    "mangle_updates": ["atom(/status, /complete)"],
    "memory_operations": [{"op": "promote", "fact": "..."}],
    "abductive_hypothesis": null
  }
}
```

### Infinite Context via Semantic Compression

1. User says "Fix server" -> Agent emits `task_status(/server, /fixing)`
2. Kernel commits to FactStore
3. Kernel deletes text "Fixing..." from history
4. Next turn: Transducer sees only the atom

Target compression ratio: >100:1 compared to raw token history.

## Handling Negation (Stratified Negation)

Mangle employs Stratified Negation to prevent logical paradoxes:

```mangle
# This would oscillate in naive Datalog:
act(X) :- !act(X).  # REJECTED by stratification check

# This is allowed (negated predicate in lower stratum):
allowed(X) :- requested(X), not blocked(X).
# blocked() must be fully evaluated before allowed()
```

If the agent attempts to compile a self-contradictory rule, the Mangle compiler rejects it.

## Abductive Reasoning (The Detective)

When data is missing, derive hypotheses:

```mangle
missing_hypothesis(RootCause) :-
    symptom(Server, Symptom),
    not known_cause(Symptom, _).

clarification_needed(Symptom) :- missing_hypothesis(Symptom).
```

The agent then actively investigates rather than guessing.

## Safety Architecture

### The Hallucination Firewall

The Kernel acts as final gate:
```go
if !Mangle.Query("permitted(?)", action) {
    return AccessDenied
}
```

Even if the LLM hallucinates `rm -rf /`, the rule `permitted(Action)` fails to derive.

### Default Deny

The system's base state contains zero `permitted` atoms. They must be positively derived from `user_intent + safety_checks`.

### Network Policy

```mangle
security_violation(URL) :-
    next_action(/network_request, URL),
    not in_allowlist(URL).
```

Obfuscated exfiltration attempts detected by analyzing argument entropy.

## Comparison with Industry

| Framework | Approach | Limitation |
|-----------|----------|------------|
| LangChain/CrewAI | LLM-as-Controller | LLM wastes capacity on planning/memory |
| Palantir AIP | Ontology + Visual Programming | No self-modification or learning |
| Adept ACT-1 | Neural-First DOM grounding | No formal safety guarantees |
| **codeNERD** | Creative-Executive Partnership | LLM focused on creativity, harness on infrastructure |
