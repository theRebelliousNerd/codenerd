---
surface: "Session_Kernel"
mode: "boundary"
subsystems_tested: ["Session Executor", "Kernel"]
blast_radius: "critical"
remediated: false
---

# 1. System Interaction Map
The integration between the Session Executor (`internal/session`) and the Kernel (`internal/core/kernel.go`) involves several key interactions:

- `JITExecutor.Execute` starts a turn and calls into the prompt compilation.
- The intent, configuration, and tools are resolved.
- Tool proposals from the LLM require explicit validation via the VirtualStore and the Mangle Kernel.
- Specifically, `VirtualStore.ValidateAction` pushes facts into the kernel and runs a query for `permitted/3` or equivalent security predicates.
- The `kernel.Assert` method is called to inject facts representing user inputs, tool results, and environmental states.
- The `kernel.Analyze` and `kernel.Query` functions determine if actions are allowed.
- SubAgents (`internal/session/subagent.go`) may hold their own isolated state or interact with shared kernel state.
- Channel communication exists if shards are spawned concurrently, but the primary boundary is synchronous kernel evaluation during a session turn.
- A failed validation (empty result set) means rejection.
- A malformed payload or garbage control packet in Piggyback processing leads to dropped actions.

# 2. Contract Analysis
Implicit contracts between Session Executor and Kernel:

1. **Monotonicity & Isolation**: The session executor assumes that facts asserted during evaluation do not leak between turns or tasks unless explicitly committed to a shared store. If `NewRealKernel()` is used without proper scoping, facts leak, causing "ghost" permissions.
2. **Type Strictness**: The session executor (often dealing in strings from LLMs) must properly format arguments as `ast.Atom` or `ast.String`. If the session sends `ast.String("edit")` but the kernel expects `ast.Name("edit")`, the join fails silently, resulting in denied permissions.
3. **Execution Safety**: The session executor expects the kernel to halt (reach fixpoint) in a reasonable time. If rules are written poorly or inputs are adversarial (e.g., highly cyclical graphs), the kernel must time out, and the session must gracefully recover.
4. **Error Handling**: The kernel returns empty sets for missing permissions, not errors. If the session executor incorrectly interprets `len(results) == 0` vs `err != nil`, unauthorized actions might proceed, or safe actions might crash the session.
5. **Context Cancellation**: Long-running queries must respect the session's context. If a user cancels a turn, the kernel evaluation must abort immediately without leaking goroutines.

# 3. Failure Mode Enumeration
For each contract:

- **Temporal**: The Kernel rule evaluation takes too long due to combinatorial explosion. The Session times out, but does it leave the VirtualStore in a locked state?
- **Semantic**: The Session passes a perfectly valid JSON payload as a string, but the Kernel expects atoms. The rule `permitted(User, Action, Payload) :- valid_json(Payload).` fails because `valid_json` is not a registered virtual predicate, or types mismatch.
- **Ordering**: The Session asserts `tool_result` before asserting the matching `tool_call`. The Kernel's monotonic logic might miss the join if the evaluation order is strict, though Datalog usually handles this, specific stratification could cause issues.
- **Partial**: The Session asserts 5 out of 10 necessary facts, gets interrupted (context cancelled), and the next turn re-uses the same kernel instance, mixing facts.
- **Corruption**: A concurrent SubAgent asserts a global `override_permission(true)` fact while the main executor is running a query, granting unauthorized access mid-flight.

# 4. Adversarial Scenario Design

Scenario 01: The String/Atom Dissonance
- **Violated Contract**: Type Strictness
- **Injection**: Inject `ast.String` for an action type when `ast.Name` is expected by the schema.
- **Expected Behavior**: Kernel yields 0 results. Session should interpret as "not permitted", not a hard failure, and Articulation should report the rejection.
- **Severity**: P1

Scenario 02: Ghost Facts from Previous Turn
- **Violated Contract**: Monotonicity & Isolation
- **Injection**: Run Turn 1 asserting `is_admin(user)`. End turn without cleanup. Run Turn 2 with different user intent.
- **Expected Behavior**: Turn 2 should NOT have `is_admin(user)` if isolation is proper. If isolation fails, it's a critical security bug.
- **Severity**: P0

Scenario 03: Combinatorial Explosion (Time Exhaustion)
- **Violated Contract**: Execution Safety
- **Injection**: Inject a highly cyclical graph of facts (e.g., `A -> B -> C -> A`) into a transitive closure rule.
- **Expected Behavior**: Kernel hits the context timeout. Session catches the deadline exceeded error and fails the turn cleanly without crashing the whole application.
- **Severity**: P1

Scenario 04: Concurrent State Corruption
- **Violated Contract**: Concurrency Safety
- **Injection**: Run two session executors concurrently sharing the same kernel instance, one asserting permissions, one querying them.
- **Expected Behavior**: Race condition detected (via `-race`) or logic leak. The system must use isolated kernel instances per session or lock properly.
- **Severity**: P0

Scenario 05: Malformed Piggyback Payload
- **Violated Contract**: Error Handling
- **Injection**: Send garbage JSON as the payload for an action.
- **Expected Behavior**: VirtualStore parsing fails, Kernel is not even called, or Kernel virtual predicate for parsing fails cleanly. Session loops back to LLM with an error.
- **Severity**: P1

Scenario 06: Context Cancellation Leak
- **Violated Contract**: Context Cancellation
- **Injection**: Cancel the context exactly when `kernel.Query` is invoked.
- **Expected Behavior**: Query aborts immediately. No goroutine leak in the evaluation engine.
- **Severity**: P2

Scenario 07: Massive Fact Injection (OOM Check)
- **Violated Contract**: Resource Constraints
- **Injection**: Assert 100,000 distinct facts into the kernel before a query.
- **Expected Behavior**: Memory grows but stays bounded. Query takes longer but completes or times out. No panic.
- **Severity**: P1

Scenario 08: Missing Schema Defs
- **Violated Contract**: Initialization requirements
- **Injection**: Query a predicate that hasn't been declared.
- **Expected Behavior**: Kernel returns a specific schema error. Session handles it as an internal error, not an authorization grant.
- **Severity**: P2

Scenario 09: Virtual Predicate Panic
- **Violated Contract**: FFI Stability
- **Injection**: A virtual predicate (e.g., checking file existence) panics.
- **Expected Behavior**: Kernel recovers from panic (or session catches it) and fails the action.
- **Severity**: P0

Scenario 10: Empty Result Set Assumption
- **Violated Contract**: Error Handling
- **Injection**: Inject a scenario where an action is denied, so results are empty.
- **Expected Behavior**: Session correctly denies the action, does not assume `err == nil` means success.
- **Severity**: P0

Scenario 11: Duplicate Fact Assertion
- **Violated Contract**: Monotonicity
- **Injection**: Assert the exact same fact 1000 times.
- **Expected Behavior**: Kernel normalizes it to a single fact (set semantics). No performance degradation.
- **Severity**: P3

Scenario 12: Invalid Fact Types
- **Violated Contract**: Type Strictness
- **Injection**: Assert a fact with 5 arguments into a predicate defined with 2.
- **Expected Behavior**: Kernel returns an arity mismatch error during assertion. Session handles it gracefully.
- **Severity**: P2

Scenario 13: Transitive Delegation Denial
- **Violated Contract**: Logic Correctness
- **Injection**: A delegates to B, B delegates to C. C requests action.
- **Expected Behavior**: Kernel must correctly trace the delegation chain up to the budget/limit. If limit exceeded, deny.
- **Severity**: P1

Scenario 14: Conflicting Policies
- **Violated Contract**: Consistency
- **Injection**: Load two rules: one permits X, one explicitly denies X (if such negative facts exist, or via negation).
- **Expected Behavior**: Mangle stratification handles it or rejects the program during analysis. Session must catch the analysis error before runtime.
- **Severity**: P1

Scenario 15: Session Disconnect Mid-Turn
- **Violated Contract**: Lifecycle Management
- **Injection**: Terminate the session (simulate crash) immediately after a fact is asserted but before execution.
- **Expected Behavior**: Any temporary facts or locks held in external systems are released (or designed to be stateless).
- **Severity**: P2

# 5. Cascading Failure Analysis
If the Session-Kernel boundary fails:
- **Ghost Facts (P0)**: If facts leak across turns, a benign agent (e.g., a documentation writer) could inherit the permissions of an executive agent from the previous turn, leading to unauthorized file deletion or API calls.
- **Combinatorial Explosion (P1)**: If the kernel hangs, the session hangs. Since sessions are often driven by API requests or CLI loops, this leads to starvation of worker threads or CLI freezing, requiring a hard kill.
- **Virtual Predicate Panic (P0)**: If a panic in a virtual predicate (e.g., file system check) is not caught, it brings down the entire Cortex process, killing all running sessions and campaigns instantly.



# 6. Detailed Cascading Failure Traces

## Failure Trace 1: The Monotonic Privilege Escalation
1. **Trigger**: An executive agent (high privilege) completes a complex campaign involving filesystem writes.
2. **Boundary Failure**: The Session Executor recycles the `RealKernel` for performance reasons instead of tearing it down.
3. **Propagation**: The kernel retains the EDB fact `permitted(/executive, /write, /workspace)`.
4. **Impact**: The next turn is initiated by a low-privileged Documentation agent. The documentation agent hallucinated a write command.
5. **Exploit**: The Session Executor queries the dirty kernel for permission. The Mangle engine evaluates the logic against the retained facts. Because the context is corrupted, the rule evaluates to true.
6. **Result**: The VirtualStore executes the write command on behalf of the Documentation agent. A critical security boundary is breached due to lack of state isolation.

## Failure Trace 2: The Silent Denial of Service
1. **Trigger**: An LLM agent generates a malformed Piggyback packet that parses as valid JSON but contains primitive strings instead of expected Enum Atoms (e.g., `"read"` instead of `/read`).
2. **Boundary Failure**: The Session Executor fails to validate the type constraints before asserting the fact into the Kernel.
3. **Propagation**: The Kernel accepts the fact (as it is a valid `ast.String`), but during evaluation, the rule `is_valid(Action) :- action(Action.Type<Atom>)` fails to match the string.
4. **Impact**: The query returns an empty result set (0 facts).
5. **Exploit**: The Session Executor correctly interprets `len(results) == 0` as a denial. However, it returns an ambiguous "Permission Denied" error to the agent.
6. **Result**: The agent enters a retry loop, continually generating the same primitive string, failing, and retrying until it exhausts its LLM API budget and the session is killed.

## Failure Trace 3: The Context Leak
1. **Trigger**: A user inputs a highly adversarial prompt designed to create an infinite loop in the agent's reasoning (e.g., defining a circular import structure).
2. **Boundary Failure**: The Session Executor translates this into a cyclic graph of facts and initiates a Kernel Query without a tight context timeout.
3. **Propagation**: The Mangle engine attempts to compute the transitive closure of the cyclic graph. Due to a bug in stratification (or missing limits), the evaluation hangs.
4. **Impact**: The user clicks "Cancel" in the UI. The Session Executor receives the context cancellation.
5. **Exploit**: The Executor aborts its wait, but the `Kernel.Query` function was not properly bound to the context. The Mangle engine goroutine continues spinning indefinitely.
6. **Result**: Repeated attacks lead to goroutine leaks, CPU starvation, and eventual OOM crash of the codeNERD background daemon.

## Failure Trace 4: The Shared Cache Poisoning
1. **Trigger**: Two parallel shards are spawned to analyze different files. Shard A analyzes a safe file, Shard B analyzes a malware file.
2. **Boundary Failure**: Both shards use the same `FactEventBus` or shared kernel instance to assert intermediate findings.
3. **Propagation**: Shard B asserts `contains_malware(/file_b)`.
4. **Impact**: Shard A queries `contains_malware(X)`. Due to lack of namespace isolation in the query, Shard A retrieves the fact from Shard B.
5. **Exploit**: Shard A incorrectly classifies its safe file as malware and halts execution.
6. **Result**: Parallel task execution yields non-deterministic, flaky results, destroying user trust in the system's reliability.

# 7. Remediation Recommendations
To address these vulnerabilities, the following architectural invariants must be enforced in code:

- **Enforce Kernel Ephemerality**: `JITExecutor` must instantiate a fresh `RealKernel` (or a cloned instance using `Kernel.Clone()`) at the start of every distinct task turn.
- **Implement Strict Type Transducers**: The Perception layer must include a schema-aware transducer that automatically maps LLM JSON string primitives to Mangle `ast.Name` Atoms based on the target predicate's signature.
- **Context-Bound Evaluations**: All FFI calls to `Kernel.Query` and `Kernel.Analyze` must be wrapped in a `context.WithTimeout` that derives from the session's parent context.
- **Result Set Validation**: Audit all `Executor` calls to the Kernel to ensure they evaluate `len(results) > 0` rather than just `err == nil`.











## Failure Trace 5: The Conflicting Permission Bypass
1. **Trigger**: An agent's persona atoms define it as a "reviewer" (read-only), but it loads a specific custom tool atom that asserts a `permitted` fact for editing.
2. **Boundary Failure**: The Session Executor asserts both the read-only limitations and the custom tool permissions into the same Kernel instance.
3. **Propagation**: The Kernel contains conflicting facts: one derived rule explicitly blocking edits based on the persona, and one base fact permitting it based on the tool atom.
4. **Impact**: Because Mangle rules evaluate to the union of satisfied facts, the `permitted` fact exists in the final set.
5. **Exploit**: The Session Executor queries `permitted(Agent, /edit, Target)`. The query returns results.
6. **Result**: The agent successfully bypasses its intended read-only persona limitation and modifies the repository, violating the security model.

## Failure Trace 6: The Ephemeral State Accumulation
1. **Trigger**: The Campaign Orchestrator runs a long multi-phase campaign spanning hundreds of turns.
2. **Boundary Failure**: The Session Executor creates a new `RealKernel` for each turn, but uses a shared `VirtualStore` that caches facts and re-injects them into the new Kernel on every turn to maintain context.
3. **Propagation**: The VirtualStore injection grows linearly. By turn 100, the VirtualStore is injecting 10,000+ historical, irrelevant facts into the JIT compiler and Kernel.
4. **Impact**: The Mangle engine spends 95% of its execution time computing the fixpoint of historical facts that have no bearing on the current turn's rules.
5. **Exploit**: The agent experiences massive latency (30+ seconds per turn). The LLM context budget is completely exhausted by the Piggyback protocol attempting to process the bloated control packets.
6. **Result**: The campaign crashes with a `context deadline exceeded` error or an LLM context window overflow.

## Failure Trace 7: VirtualStore Asynchronous Desync
1. **Trigger**: A background shard (e.g., `perception_firewall`) completes an analysis and asserts a risk fact into the `FactEventBus`.
2. **Boundary Failure**: The Session Executor initiates its rule evaluation loop before the `FactEventBus` completely flushes its queues to the Kernel's EDB.
3. **Propagation**: The Mangle engine computes the fixpoint without the critical risk fact.
4. **Impact**: The query `blocked_by_policy(Action)` returns empty because the risk fact was missing.
5. **Exploit**: The Session Executor permits the execution of an unsafe action (e.g., deleting a critical system file).
6. **Result**: A race condition leads to intermittent security bypasses that are impossible to reproduce deterministically in unit tests.

## Failure Trace 8: The Arity Mismatch Swallow
1. **Trigger**: The codeNERD ruleset is updated, changing the arity of the `permitted` predicate from 3 arguments `(User, Action, Target)` to 4 arguments `(User, Action, Target, Context)`.
2. **Boundary Failure**: The Session Executor code is not updated and continues to query `permitted(U, A, T)`.
3. **Propagation**: The Mangle engine rejects the query due to a signature mismatch.
4. **Impact**: The query returns an error. The Session Executor checks `err != nil` but logs it as a "warning" and proceeds with fallback logic that defaults to "allow" for local executions.
5. **Exploit**: Every action is permitted because the authorization query fundamentally fails to execute.
6. **Result**: Complete circumvention of the authorization model due to improper error handling at the FFI boundary.

## Failure Trace 9: The Invalid Predicate Name Crash
1. **Trigger**: An LLM agent hallucinates a tool call that generates a predicate name starting with an uppercase letter, e.g., `Permitted(...)`.
2. **Boundary Failure**: The Session Executor blindly constructs an `ast.Atom` using the hallucinated predicate name and attempts to assert it.
3. **Propagation**: The Mangle parser strictly enforces that predicate names must begin with a lowercase letter. It throws a syntax validation error during the EDB insertion.
4. **Impact**: The assertion function panics or returns a hard error.
5. **Exploit**: The Session Executor does not recover from the assertion failure.
6. **Result**: The session crashes abruptly. All accumulated state for the task is lost, requiring a complete restart.

## Failure Trace 10: The Massive String Payload OOM
1. **Trigger**: An agent uses the `read_file` tool to ingest a 50MB log file, and the payload is returned as a single massive string inside a tool result fact.
2. **Boundary Failure**: The Session Executor asserts this massive string into the Kernel to track tool execution history.
3. **Propagation**: The Mangle engine creates multiple copies of the 50MB string during its fixpoint derivation and join operations.
4. **Impact**: The Go garbage collector thrashes, and memory consumption spikes exponentially.
5. **Exploit**: The Cortex process exceeds its container memory limits.
6. **Result**: The OS Out-Of-Memory (OOM) killer terminates the codeNERD process, causing a hard outage.

## Failure Trace 11: Malformed Piggyback Recursion
1. **Trigger**: The LLM output contains nested JSON structures within a Piggyback packet that exceed the predefined depth limit.
2. **Boundary Failure**: The perception layer fails to flatten or sanitize the JSON before passing it to the Session Executor.
3. **Propagation**: The Session Executor attempts to convert the deeply nested JSON into Mangle nested atoms (e.g., `fn:List(...)`).
4. **Impact**: The Mangle AST constructor hits a stack overflow or exceeds AST depth limits.
5. **Exploit**: The Kernel evaluation engine halts abruptly.
6. **Result**: The session hangs indefinitely because the AST construction failure was not caught by the executor's panic recovery mechanism.

## Failure Trace 12: Rule Corruption via EDB Injection
1. **Trigger**: A malicious user intent string contains syntax that resembles Mangle logic clauses: `delete_file) :- true. permitted(hacker, `.
2. **Boundary Failure**: The Session Executor asserts the user intent string directly into a dynamic `user_input` fact without proper escaping.
3. **Propagation**: If the Mangle engine dynamically evaluates or parses this string (e.g., through a virtual predicate that interprets rules), the injected logic alters the IDB.
4. **Impact**: The injected rule `permitted(...) :- true.` becomes active.
5. **Exploit**: The attacker gains unconditional permission to execute any action.
6. **Result**: A classic injection attack successfully corrupts the declarative logic engine.

## Failure Trace 13: Timeout Masking
1. **Trigger**: A complex query takes 4.9 seconds, just under the 5-second context timeout.
2. **Boundary Failure**: The Session Executor receives the result, but the total session turn budget (including LLM generation and tool execution) was also 5 seconds.
3. **Propagation**: The Executor proceeds to execute the action via the VirtualStore.
4. **Impact**: The VirtualStore action initiates, but the parent context expires immediately.
5. **Exploit**: The action completes partially (e.g., a file is truncated but not written).
6. **Result**: Data corruption due to mismatched timing bounds between the Kernel query and the broader session context.

## Failure Trace 14: Conflicting Shard Delegation
1. **Trigger**: A user command requires both coding and testing. The Campaign Orchestrator delegates to both a `coder` shard and a `tester` shard concurrently.
2. **Boundary Failure**: Both shards utilize the same Session Executor loop, competing for the same LLM client connection and Kernel mutex locks.
3. **Propagation**: Lock contention slows down both shards. The JIT compiler experiences thrashing as it switches context between the coder and tester prompt sets.
4. **Impact**: Both shards exceed their latency budgets.
5. **Exploit**: The Orchestrator assumes both shards failed and attempts to restart them, compounding the contention.
6. **Result**: A livelock scenario where no forward progress is made, and API budgets are wasted on failed retries.

## Failure Trace 15: Autopoiesis Loop Failure
1. **Trigger**: The agent successfully fixes a bug and triggers the Autopoiesis (self-learning) loop to derive a new rule.
2. **Boundary Failure**: The JIT compiler generates a new `.mg` rule file based on the successful trace, but the rule contains a subtle syntax error (e.g., unstratified negation).
3. **Propagation**: The Session Executor attempts to reload the `RealKernel` with the newly learned rule.
4. **Impact**: The `analysis.Analyze` phase fails during Kernel boot.
5. **Exploit**: The Kernel refuses to boot, returning an error.
6. **Result**: The entire codeNERD system becomes bricked for that project until a human manually deletes the corrupted `.mg` file from the `.nerd/rules` directory.


# 8. Architectural Implications


# 9. Extended Technical Subsystem Review

## Subsystem 1: The `JITExecutor`
The `JITExecutor` bridges the gap between raw intent (`fix_bug`) and actual execution. It relies on the `PromptCompiler` to synthesize the system prompt, but crucially, it does not hold persistent state itself. The problem is that it assumes the `VirtualStore` it passes to the LLM agent and the `Kernel` it uses for authorization are perfectly synchronized.

When an LLM proposes a tool call, the `JITExecutor`:
1. Parses the JSON.
2. Checks if the tool is in the active persona's capability list.
3. If yes, it delegates validation to the `VirtualStore`.
4. The `VirtualStore` asserts facts to the `Kernel` and runs queries.

If step 3 fails because the LLM generated arguments that violate constraints, the VirtualStore returns an error, which the `JITExecutor` reflects back to the LLM. This loop is the primary mechanism for self-correction.

## Subsystem 2: The `VirtualStore`
The `VirtualStore` is the FFI gateway. It maps logic programming constraints (Mangle) to imperative system calls (Go). It maintains its own internal cache to avoid hitting the actual filesystem or APIs repeatedly during rule evaluation.

### Cache Coherency Problems
The biggest risk at this boundary is cache coherency. The `VirtualStore` asserts facts into the `Kernel` representing the world state (e.g., `file_exists(/etc/passwd)`). If the agent deletes the file, the `VirtualStore` must perform two distinct operations atomically:
1. Execute the system call (`os.Remove`).
2. Retract the fact from the `Kernel`.

Because Mangle is strictly monotonic within a single evaluation cycle, retraction is practically impossible mid-turn. The only safe way to reflect the new world state is to tear down the entire `RealKernel` instance and boot a new one, re-evaluating the world state from scratch. If the `VirtualStore` fails to do this, subsequent queries in the same turn will operate on stale facts, leading to cascading failures.

## Subsystem 3: The Mangle `RealKernel`
The `RealKernel` is the source of truth for authorization. It takes the JIT-compiled `.mg` rules and the EDB facts provided by the `VirtualStore` and computes the least fixed point (the IDB).

### Stratification and Negation
Mangle handles negation via stratification. If a rule relies on the *absence* of a fact (e.g., `permitted(...) :- not blocked(...)`), it must be in a higher stratum than the rule defining `blocked`. The `Session Executor` is largely ignorant of these mechanics. If it attempts to dynamically inject a rule that creates a cyclic dependency involving negation, the `RealKernel`'s `analysis.Analyze` phase will fail.

The failure mode here is that the `Session Executor` often treats analysis failures as fatal system crashes, rather than catching them and telling the JIT compiler to rewrite the broken rule.

## Deep Dive: The Articulation Phase
After the LLM completes its execution loop, the `Session Executor` calls the Articulation layer to summarize the actions. The Articulation layer reads the `VirtualStore`'s transaction log.

If a boundary failure caused the `VirtualStore` to execute an action without proper `Kernel` authorization (e.g., due to the `err == nil` swallowing bug), the Articulation layer will dutifully report the unauthorized action to the user as a success. This breaks the transparency guarantee of the codeNERD architecture. The Articulation layer must independently cross-reference its transaction log with the `Kernel`'s authorization derivations to detect discrepancies before presenting them to the user.










## Extended System Boundary Analysis: Transducer to Mangle Graph Projection

The integration between the Perception system and the Kernel relies heavily on accurate graph projection.
When the codeNERD transducer processes an LLM response containing a Piggyback control packet, it often
encounters arrays of strings or nested objects. The mapping from these dynamic JSON structures to the static,
type-checked Mangle AST represents one of the most fragile seams in the architecture.

### Vulnerability: Object Graph Cycle Injection
A primary failure mode identified during integration reconnaissance is the potential for the JIT compiler to accept
a JSON object with recursive references (e.g., self-referencing nodes in a filesystem tree representation). When the
Perception layer projects this into Mangle facts (like `parent(A, B)`), a malicious prompt can create a tight cycle.

If the rule executing over this data is `ancestor(X, Z) :- parent(X, Y), ancestor(Y, Z).`, Mangle's evaluation engine
is designed to find a fixed point safely. However, if the generated rules themselves are malformed and bypass stratification checks,
the fixpoint computation may fail or exceed memory bounds.

### Mitigation via Transaction Manager
To safeguard against this, the Transaction Manager subsystem must intercede. It should apply a cycle-detection pass on the projected graph *before* asserting the facts into the `RealKernel`. By intercepting the facts in the `VirtualStore`, the system can reject cyclic inputs at the FFI boundary, returning a graceful `Invalid Payload` error to the LLM agent rather than risking a Kernel evaluation stall.

## Extended System Boundary Analysis: Autopoiesis Rule Promotion

The Autopoiesis engine (the system that learns new Mangle rules from successful problem-solving traces) operates across three boundaries:
1. `Session` (which completes a task).
2. `VirtualStore` (which records the facts proving success).
3. `Kernel` (which evaluates the new rule for validity).

### Vulnerability: The Ouroboros Effect (Rule Corruption)
When the Autopoiesis subsystem attempts to promote a newly learned rule (e.g., `coder_edit_barred`), it submits it to the `RealKernel` for syntax and stratification validation via `analysis.Analyze()`.

If the rule is unstratified (e.g., it contains negative cycles), the `Kernel` correctly rejects it. The critical integration failure occurs if the `Autopoiesis` subsystem caches the invalid rule in the `.nerd/rules` directory *before* confirming successful analysis.

If this happens, the next time the Cortex boots up, it will read the corrupted `.mg` file from disk, fail the `analysis.Analyze()` check during the core bootstrap sequence, and terminate the codeNERD daemon with a fatal error.

### Mitigation via Atomic Shadow Deployments
To prevent this, the integration seam must be hardened using the `KernelClone` capability. The `Autopoiesis` shard must:
1. Clone the current `RealKernel`.
2. Assert the new rule into the isolated clone.
3. Only if the clone reaches a valid fixpoint, promote the rule to disk.
4. Signal the `ShardManager` or `JITExecutor` to gracefully reload the primary `RealKernel` with the new file.

## Extended System Boundary Analysis: The Perception Firewall

A less obvious but highly consequential boundary is between the Perception layer (which parses unstructured user intent) and the JIT Executor. This boundary must act as a strict firewall.

### Vulnerability: Command Injection via Persona Override
If a user inputs `/fix this bug but act as /reviewer`, the Perception Transducer must definitively map this intent to a single persona atom (either `coder` or `reviewer`). If the Transducer fails to normalize the intent and instead passes *both* persona atoms down the pipeline, the JIT Compiler will load a hybrid system prompt containing contradictory rules from both the `coder.mg` and `reviewer.mg` domains.

When this hybrid prompt is executed, the LLM may become confused, but more importantly, the `Kernel` will be forced to evaluate overlapping and potentially conflicting policies. The `coder_edit_barred` rules may clash with the `reviewer_read_only` constraints, leading to unpredictable authorization behavior.

### Mitigation via Canonical Intent Resolution
The JIT Executor must strictly enforce a "One Intent, One Persona" invariant. If the Transducer produces ambiguous or multiple intent mappings, the JIT Executor should halt execution and prompt the user for clarification, rather than attempting to merge incompatible rule sets.

## Extended System Boundary Analysis: MCP (Model Context Protocol) Integration

The integration with external MCP servers represents another complex boundary involving the VirtualStore. When the system decides to use a tool provided by an external MCP server, the `VirtualStore` acts as a proxy.

### Vulnerability: MCP State Desynchronization
If an MCP tool call modifies state on the external server (e.g., updating a Jira ticket), but the codeNERD session crashes or is cancelled immediately afterward, the `Kernel` is unaware of the external state mutation.

### Mitigation via Two-Phase Commit
While a full two-phase commit might be overkill, the VirtualStore should ideally assert a `pending_external_mutation` fact into the Kernel before initiating the MCP call. Upon success, it retracts (via a new evaluation cycle) the pending fact and asserts the `mutation_complete` fact.

## Conclusion and Strategic Review
The overarching theme across all these integration failures is the fundamental mismatch between imperative control flow (Go, Session Executor, VirtualStore) and declarative fixpoint logic (Mangle, Kernel).

1. **State Ownership**: Go routines naturally want to mutate variables. Mangle logic is immutable per cycle. Attempting to force mutable behavior into Mangle without proper tear-down boundaries results in Ghost Facts.
2. **Type Systems**: The LLM speaks in untyped strings; Mangle speaks in strict Atoms and Numbers. The Perception Transducer must be a rigorous enforcer of types, not just a passthrough.
3. **Error Paradigms**: Imperative systems throw exceptions or return `err != nil`. Declarative systems fail silently by returning empty sets. The FFI boundary must explicitly translate empty sets into meaningful access-denied errors.

By strictly adhering to these invariants—ephemeral kernel instances, rigorous type mapping, bounded context evaluation, and explicit result set checking—the codeNERD JIT architecture can achieve stable and secure orchestration across its subsystem boundaries.

## Deep Dive: The Interactive Gate Boundary

The Interactive Gate is responsible for halting automated execution and requesting human approval for sensitive actions (e.g., executing shell commands, deleting files). This creates a critical intersection point involving four distinct subsystems: the `Session Executor` (which pauses), the `VirtualStore` (which holds the pending action), the `Kernel` (which evaluates if the action requires approval based on the Constitution), and the `UI/Chat Layer` (which interacts with the user).

### Vulnerability: Asynchronous Approval Injection (The Confused Deputy)
When the `Kernel` determines an action requires approval, the `Session Executor` suspends the current turn and waits on a channel for the UI to provide the user's decision.

The integration failure mode occurs if the approval routing mechanism uses a weak identifier (like a simple integer ID or just the action name) to correlate the pending action with the incoming approval. If the user initiates *another* parallel session (e.g., a background task or another terminal window) that also requests approval, a race condition emerges.

If the user approves the benign background task, the UI might send the `Approval Granted` signal back to the central `FactEventBus` or directly to the awaiting Executor. If the identifier is ambiguous, the malicious or destructive action in the first session might intercept the approval signal. The `Session Executor` receives the signal, incorrectly believes the user authorized the destructive action, and proceeds to command the `VirtualStore` to execute it.

### Mitigation via Cryptographic Context Binding
To prevent this Confused Deputy attack, the system must employ strong cryptographic or UUID-based binding.
1. When the `Kernel` flags an action, the `VirtualStore` must generate a strong `ApprovalTicket` (a UUID).
2. The `Session Executor` asserts a `pending_approval(TicketID, Action, Context)` fact into the Kernel.
3. The UI presents the exact Context and Action tied to the TicketID.
4. The user's approval is sent back strictly carrying the TicketID.
5. The `Session Executor` verifies the TicketID against its local wait-state before proceeding.

## Deep Dive: The Dreamer Subsystem Simulator

The Dreamer subsystem acts as a sandbox simulator. Before taking a risky action, the `Session Executor` can invoke the Dreamer to simulate the outcome. The Dreamer clones the `RealKernel` and projects hypothetical facts into it to see if any safety invariants are violated.

### Vulnerability: Dream State Bleed
The isolation between the actual world state and the simulated dream state is entirely dependent on the fidelity of the `Kernel.Clone()` method.

If `Kernel.Clone()` performs a shallow copy of the underlying EDB (Extensional Database) slices or maps, any hypothetical fact asserted by the Dreamer (e.g., `hypothetical_file_deleted(/etc/passwd)`) will inadvertently mutate the primary `RealKernel`'s state.

When the simulation completes, the Dreamer is discarded. However, because of the shallow copy, the primary `Kernel` now believes `/etc/passwd` has actually been deleted. The next time the `Session Executor` queries the world state, it will act on this corrupted hallucination, leading to catastrophic logic failures.

### Mitigation via Deep Copy and Immutable Data Structures
The boundary between reality and simulation must be absolute. The `RealKernel` must implement a true deep copy mechanism for all fact storage, or better yet, utilize persistent, immutable data structures (like Hash Array Mapped Tries) where the Dreamer's clone is merely a new pointer to the root node. Any mutations by the Dreamer allocate new nodes without affecting the original tree. This guarantees O(1) cloning performance while mathematically ensuring state isolation.


## Deep Dive: The TDD Loop and Shard Delegation Boundary

The Test-Driven Development (TDD) loop represents a complex orchestration sequence managed by the `Session Executor`. When an agent is tasked with fixing a bug, the Orchestrator may delegate the work between a `coder` shard and a `tester` shard.

### Vulnerability: The Endless Ping-Pong Retry Loop
The boundary failure occurs when the interaction between the `coder` and the `tester` lacks a unified state machine or a hard escalation limit enforced by the `Kernel`.
1. The `coder` writes a patch.
2. The `Session Executor` delegates to the `tester`.
3. The `tester` runs `go test`, which fails.
4. The `VirtualStore` records the test failure as a fact.
5. The `Session Executor` routes back to the `coder`.
6. The `coder` makes a trivial or incorrect change.

If the `Kernel` rules do not track the number of retries (`retry_count(Task, Count)`) or the similarity of the patches, the system can enter an infinite ping-pong loop. The `Session Executor` keeps passing the baton between the two shards until the overall API budget is exhausted.

### Mitigation via Kernel Escalation Policies
The `Kernel` must define explicit escalation policies.
`escalate_to_human(Task) :- retry_count(Task, C), C > 3.`
The `VirtualStore` must track the retry count monotonically across JIT executions. When the limit is reached, the `Session Executor` must catch the derivation from the `Kernel` and gracefully halt the loop, presenting the partial work and the failure trace to the user for manual intervention.

## Deep Dive: The FileLock and Concurrent Modification Boundary

When multiple agents (or a user and an agent) operate within the same workspace, concurrent file modifications are a primary source of integration failure.

### Vulnerability: The Phantom Edit
1. The `Session Executor` (driving the `coder` agent) reads `main.go`.
2. The agent takes 15 seconds to generate a patch.
3. During this time, the user manually edits `main.go` and saves it.
4. The agent finishes generating the patch and requests the `VirtualStore` to apply it.
5. The `VirtualStore` asserts the `pending_edit` fact to the `Kernel`.
6. The `Kernel` approves the edit because the agent has permission.
7. The `VirtualStore` executes the edit, overwriting the user's manual changes.

The integration failure lies in the lack of optimistic concurrency control or file locking between the perception of the file state and the articulation of the edit.

### Mitigation via Content-Addressable Hashes
The boundary between the perception (reading the file) and the action (writing the file) must be secured by requiring the `Session Executor` to provide the original file hash along with the patch.
The `VirtualStore` must implement an atomic Check-and-Set (CAS) operation. It verifies the current file hash against the provided original hash. If they mismatch, the `VirtualStore` rejects the edit, and the `Session Executor` is forced to re-read the file and attempt the patch again.


## Final Summary and Architecture Directives

The codeNERD architecture relies on the strict separation of imperative side effects (managed by the `Session Executor` and `VirtualStore`) and declarative authorization logic (managed by the Mangle `Kernel`). The integration tests designed in this exercise prove that the seams between these systems are highly vulnerable to type mismatches, state leakage, resource exhaustion, and concurrency bugs.

To ensure the long-term stability and security of the platform, the engineering team must adopt the following directives:
1. **Never share Kernel instances** across distinct session turns unless the state is explicitly designed to be monotonic and additive. Use `Kernel.Clone()` or boot fresh instances to guarantee isolation.
2. **Implement strict FFI boundary checks** for all data flowing from the JIT/LLM context into the Mangle engine. Rely on `types.MangleAtom` and `types.MangleString` with explicit validation rather than trusting raw strings.
3. **Always validate result set lengths** (`len(results) > 0`) instead of relying solely on `err == nil` when querying the Kernel for authorization, as Datalog natively uses empty sets to represent denied assertions.
4. **Wrap all evaluation calls in strict timeout contexts** to prevent poorly formed or adversarial logic from causing infinite loops or OOM crashes in the background daemon.

By adhering to these principles and running the integration suite established here, the codeNERD architecture can scale to support complex, multi-agent orchestrations without sacrificing reliability.

## Deep Dive: Shard Capability Scoping
Another crucial boundary that frequently fails is the mapping of JIT capabilities to Kernel permissions. The configuration of a SubAgent (e.g., granting it the `read_file` tool) does not automatically confer Mangle `permitted(/agent, /read_file, /target)` rights.

### Vulnerability: Capability/Permission Desync
If the `Session Executor` parses the tools correctly and the `VirtualStore` attempts execution, it will fail at the Kernel FFI if the JIT compiler forgot to include the corresponding policy rules in the compiled prompt. This leads to a state where the agent *knows* it has a tool, but is consistently denied usage, leading to a looping failure mode as the agent repeatedly tries and fails to use its authorized tool.

### Mitigation via Unified Configuration Synthesis
The configuration generation must be holistic. The boot sequence or task spawner must inject a single source of truth into both the JIT context and the Kernel EDB. If a tool is granted, the corresponding `permitted` fact must be asserted atomically.

## Deep Dive: The Role of the Spawner
The `Spawner` acts as the gatekeeper for dynamically instantiating user-defined agents. Its contract with the `Session Executor` guarantees that any agent instantiated possesses a valid JIT system prompt.

### Vulnerability: Hollow Spawns
If an agent is defined (e.g., `.nerd/agents/Hacker`) but lacks the required `prompts.yaml` structure, the Spawner might fallback to an empty or default context. When the `Session Executor` processes this hollow agent, it sends it to the LLM. The LLM, lacking guidance, hallucinates actions. The `Kernel`, receiving these hallucinations, either rejects them (safe but useless) or accidentally approves a benign-looking but destructive action.

### Mitigation via Strict Validation
The `Spawner` must assert a `valid_agent` fact into the Kernel upon successful validation of the agent's definitions. The execution loop should enforce a rule: `action_allowed(A) :- valid_agent(A), ...`. If the Spawner fails to produce a valid JIT context, it must return an explicit error and refuse instantiation, rather than allowing a hollow agent to enter the execution pipeline.

## Architectural Reflection and Next Steps
The tests implemented in this suite demonstrate that relying on `err == nil` is universally dangerous when interfacing with declarative logic engines. The codebase must undergo a systematic refactor of all `VirtualStore` and `Session Executor` calls to `Kernel.Query` and `Kernel.Analyze` to explicitly check result set cardinalities and appropriately map empty sets to authorization failures. Furthermore, the `FactEventBus` must be audited for cache coherency and flush behaviors to eliminate the pervasive race conditions observed during high-concurrency background operations.
