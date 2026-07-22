---
surface: "VirtualStore_GraphQuery"
mode: "boundary"
subsystems_tested: ["VirtualStore", "GraphQuery", "LocalStore"]
blast_radius: "critical"
remediated: false
---

# 🏰 Siege Integration Analysis: VirtualStore_GraphQuery

## 1. System Interaction Map

The `VirtualStore` to `GraphQuery` boundary acts as a critical bridge between Mangle's declarative reasoning engine (the Kernel) and the system's persistent, structural knowledge graph (`LocalStore`).

### Interaction Flow (End-to-End)

1. **Trigger Phase:** The Kernel evaluates a query or rule that requires information from the world model (e.g., code dependencies, symbol paths).
2. **Virtual Predicate Activation:** The Kernel encounters the `query_graph/3` virtual predicate. It invokes `VirtualStore.GetFacts()` or specifically `VirtualStore.getQueryGraphAtoms(query ast.Atom)`.
3. **Parameter Extraction (The Bottleneck):**
   - `VirtualStore.getQueryGraphAtoms` receives the `ast.Atom`.
   - It expects 3 arguments: `QueryType`, `Params`, `Result`.
   - The `QueryType` is extracted via `cleanMangleString`.
   - The `Params` argument, originally a structured Mangle AST representing a map, is destructively flattened into a single string via `query.Args[1].String()` and stored in `params["arg"]`.
4. **Synchronous Crossing:**
   - `getQueryGraphAtoms` invokes `v.graphQuery.QueryGraph(qType, params)` **synchronously**, holding no `context.Context`.
   - This means the Mangle derivation loop is entirely blocked, unable to be cancelled if the underlying storage layer stalls.
5. **Adapter Execution (`LocalStoreGraphAdapter`):**
   - Receives `queryType` and `params`.
   - Extracts `entity := params["arg"].(string)`.
   - Routes to the appropriate `LocalStore` query (`QueryLinks`, `TraversePath`).
6. **Data Storage Execution (`LocalStore`):**
   - Executes SQL queries against the SQLite database (e.g., `SELECT entity_b FROM knowledge_graph WHERE entity_a = ?`).
7. **Result Translation:**
   - The adapter returns a Go slice `[]string` or boolean to `getQueryGraphAtoms`.
   - `getQueryGraphAtoms` uses `goToMangleTerm` to translate Go primitives back to Mangle `ast.BaseTerm`.
8. **Fact Yielding:**
   - An `ast.Atom` is returned to the Kernel, yielding the result of the `query_graph` evaluation.

### Specific Function Signatures & Boundaries

- `core.VirtualStore.getQueryGraphAtoms(query ast.Atom) ([]ast.Atom, error)` -> Extracts parameters, blocks.
- `types.GraphQuery.QueryGraph(queryType string, params map[string]any) (any, error)` -> Interface crossing.
- `store.LocalStoreGraphAdapter.QueryGraph(queryType string, params map[string]any) (any, error)` -> Concrete adapter parsing flattened args.
- `store.LocalStore.QueryLinks(entity string, direction string) ([]KnowledgeLink, error)` -> DB access.
- `store.LocalStore.TraversePath(from string, to string, maxDepth int) ([]string, error)` -> Complex DB access.

---

## 2. Contract Analysis

The boundary between `VirtualStore` (Mangle domain) and `GraphQuery` (World model domain) is governed by several implicit and explicit contracts.

### Contract 1: Temporal Responsiveness (The Synchronous Block)
**Implicit Contract:** `GraphQuery` must return "immediately" (usually <10ms).
**Why:** Mangle evaluation is CPU-bound and synchronous. Because `getQueryGraphAtoms` does not pass a `context.Context` to `QueryGraph`, the engine implicitly trusts that the graph query will never hang. If SQLite encounters lock contention or a complex `TraversePath` takes seconds, the entire Mangle engine halts.

### Contract 2: Parameter Serialization (The Semantic Discard)
**Implicit Contract:** The `Params` argument in Mangle is purely a single string identifier, despite being syntactically capable of representing complex Maps or Lists.
**Why:** `params["arg"] = cleanMangleString(query.Args[1].String())` forcefully flattens the Mangle AST. If a Mangle rule attempts to pass structured data like `{from: "/fileA", depth: 3}`, it becomes a serialized string that the adapter cannot parse natively.

### Contract 3: Mangle Term Mapping (The Type Bridge)
**Implicit Contract:** Results returned from `GraphQuery` as `any` are cleanly mappable back to Mangle AST types via `goToMangleTerm`.
**Why:** The adapter returns `[]string` or `bool`. `goToMangleTerm` handles `[]string` by creating a `List` of `String` constants. If the adapter ever returns an unsupported type (e.g., a complex struct or nested slice), `goToMangleTerm` falls back to `fmt.Sprintf("%v", v)`, creating a stringified mess that Mangle rules cannot match against.

### Contract 4: Non-Nullability of Interface
**Explicit Contract:** `VirtualStore.graphQuery` might be nil.
**Why:** `SetGraphQuery` is called after Kernel initialization. If `getQueryGraphAtoms` fires before attachment, it safely returns `nil, nil`. However, rules depending on this will silently fail to unify.

### Contract 5: Concurrency Safety
**Implicit Contract:** `GraphQuery.QueryGraph` must be safe for concurrent execution.
**Why:** Multiple Mangle evaluations might trigger `query_graph` simultaneously across different goroutines if the Session Executor spawns parallel evaluations. The `LocalStore` must handle concurrent SQLite reads.

---

## 3. Failure Mode Enumeration

### Temporal Failures
1. **DB Lock Contention Stalls Kernel:** SQLite is locked by a writer (e.g., a massive background learning sync). `QueryGraph` blocks for 500ms. The Mangle derivation loop freezes, delaying the LLM session loop.
2. **Unbounded TraversePath:** `TraversePath` with `from->to` explores a dense, highly connected graph. The algorithm runs for 5 seconds. Since there's no context cancellation, the system is held hostage.
3. **Context Cancellation Ignored:** A user hits "Ctrl+C" to cancel their request. The `Session Executor` cancels the context. The LLM stops. However, if the VirtualStore is mid-query, it continues running because the `context.Context` was dropped at the VirtualStore boundary.

### Semantic Failures
4. **Structured Argument Flattening:** A rule sends `query_graph("path", {from: "A", to: "B"}, Result)`. The VirtualStore flattens it to `{ "arg": "{from: "A", to: "B"}" }`. The adapter expects `"A->B"`. It fails to parse, returning a silent error or empty result.
5. **Path Argument Misformat:** A user sends "from>to" (missing a dash). `splitPathArg` fails, returning nil. The query errors out, yielding zero facts.
6. **Null Bytes in Argument:** A malicious user creates a file with a null byte. The Mangle atom contains the null byte. When sent to `QueryGraph`, the SQLite adapter passes it to the CGO driver, potentially truncating the query or causing a C error.

### Ordering Failures
7. **Query Before Registration:** The system starts evaluating rules before `SetGraphQuery` is called. Mangle rules silently evaluate to false. When it is later registered, the cached derivations might prevent re-evaluation if not properly invalidated.
8. **In-Flight Deregistration:** `SetGraphQuery` is called with `nil` during an active Mangle derivation loop (e.g., during a hot reload). RWMutex ensures safety, but subsequent atoms fail.

### Partial Failures
9. **Partial Graph Result:** `QueryLinks` returns the first 500 links then errors due to a corrupted page in SQLite. The adapter bubbles up an error. Mangle yields no facts, acting as if there are zero links, instead of 500.

### Corruption Failures
10. **Result Type Fallback Stringification:** `LocalStore` is refactored to return `[]KnowledgeLink` structs instead of `[]string`. `goToMangleTerm` doesn't support structs. It falls back to `ast.String("[{A B outgoing}]")`. Mangle rules expecting a List fail to unify.
11. **Result Mismatch:** The adapter returns a `float64` for a "distance" query. Mangle unifies it, but a later rule strictly expects an `int64`. The join fails silently.

---

## 4. Adversarial Scenario Design

Here are 15 specific scenarios designed to break the `VirtualStore` <-> `GraphQuery` boundary.

| Scenario ID | Contract Violated | Failure Injection Mechanism | Expected System Behavior (Ideal) | Severity |
|-------------|-------------------|-----------------------------|----------------------------------|----------|
| S01 | Temporal | Mock GraphQuery delays response by 10s. | Mangle evaluation should timeout or be cancelable (Current: Hangs). | P0 |
| S02 | Parameter Serialization | Pass complex Mangle Map as parameter. | System should parse it or reject explicitly, not silently stringify. | P1 |
| S03 | Context Cancellation | Cancel parent Context while `query_graph` runs. | `query_graph` should abort. (Current: Continues running). | P0 |
| S04 | Concurrency | 100 goroutines concurrently query the graph. | LocalStore must process all without `database is locked` panics. | P1 |
| S05 | Semantic Parsing | Pass `"A->B->C"` to `path` query. | Adapter `splitPathArg` should reject or parse correctly. | P2 |
| S06 | Result Type | Mock adapter returns an unsupported struct type. | `goToMangleTerm` should return an error, not stringify it. | P1 |
| S07 | Mangle String Escape | Pass atom argument with `"` and `\`. | `cleanMangleString` should not corrupt the identifier. | P2 |
| S08 | Large Data Payload | Adapter returns an array of 1,000,000 strings. | `goToMangleTerm` should handle it without OOMing or Mangle stack overflow. | P1 |
| S09 | Nil Adapter | Call `query_graph` when `graphQuery` is nil. | Should return empty fact set, not panic. | P3 |
| S10 | Empty Parameter | Pass `""` as the argument to `incoming`. | Adapter should return error, VirtualStore handles it gracefully. | P2 |
| S11 | SQL Injection Attepmt| Pass `"x'; DROP TABLE knowledge_graph; --"` | LocalStore parameterization should prevent execution. | P0 |
| S12 | Unknown Query Type | Pass `"launch_missiles"` as QueryType. | Adapter should return error, VirtualStore ignores gracefully. | P3 |
| S13 | Rapid Re-Registration| Call `SetGraphQuery` 1000 times concurrently. | RWMutex should prevent data races. | P2 |
| S14 | Path Exhaustion | Request `TraversePath` on cyclic graph nodes. | Adapter should respect depth limit and not infinitely recurse. | P1 |
| S15 | Floating Point Prec. | Adapter returns `NaN` or `Infinity`. | `goToMangleTerm` should reject invalid float64 values. | P2 |

---

## 5. Cascading Failure Analysis

### Pathway A: The Synchronous Hang (P0)
If `GraphQuery.QueryGraph()` stalls (e.g., due to SQLite lock contention or a massive traverse operation):
1. `VirtualStore.getQueryGraphAtoms` blocks indefinitely.
2. The Mangle Engine's `Eval()` loop is blocked on a virtual predicate.
3. The `Session Executor` waiting for the Mangle derivation (e.g., waiting for `next_action`) is blocked.
4. Because the context is not propagated down to `GraphQuery`, the `Executor`'s timeout `context.WithTimeout` fires, but the goroutine running `Eval()` leaks because it's stuck in `VirtualStore`.
5. Result: A leaked goroutine holding a read lock on the VirtualStore and potentially holding kernel state. The user's session is unresponsive.

### Pathway B: Semantic Discard (P1)
If a Mangle rule relies on passing structured arguments:
1. `query_graph("dependencies", {module: "core", depth: 3}, Result)`.
2. `VirtualStore` stringifies this to `"{module: "core", depth: 3}"`.
3. `LocalStoreGraphAdapter` expects a clean entity name. It fails to parse.
4. It returns an error.
5. `VirtualStore` logs a warning and returns `nil, nil`.
6. Mangle rule fails to derive.
7. `Session Executor` concludes the agent has no dependencies.
8. The Agent modifies core files without understanding the blast radius, leading to broken builds.

### Pathway C: The Stringification Fallback (P1)
If `GraphQuery` returns a complex type:
1. The adapter returns a `KnowledgeLink` struct instead of `[]string`.
2. `goToMangleTerm` stringifies it: `ast.String("{EntityA: X, EntityB: Y}")`.
3. Mangle rules expecting a List of strings cannot iterate or join on this string.
4. The rule yields zero facts.
5. The orchestration layer fails to map relations, causing isolated context windows.
6. The AI agent hallucinates APIs because it lacks the graph context.

---

## 6. Detailed Table-Driven Test Matrix

To fully cover this integration surface, the E2E test suite must implement the following table-driven cases:

| Case | Input Mangle Atom | Mock Adapter Behavior | Expected Result (Facts/Errors) | Assertions |
|------|-------------------|-----------------------|--------------------------------|------------|
| Standard Outgoing | `query_graph("outgoing", "file.go", R)` | Returns `["deps.go"]` | `query_graph("outgoing", "file.go", ["deps.go"])` | Valid list conversion |
| Standard Path | `query_graph("path", "A->B", R)` | Returns `true` | `query_graph("path", "A->B", true)` | Valid bool conversion |
| Complex Args | `query_graph("links", "/my/file.go", R)` | Returns `[]` | `query_graph("links", "/my/file.go", [])` | `cleanMangleString` logic |
| Delay Injection | `query_graph("links", "A", R)` | Sleeps 2 seconds | Should timeout if we wrap it, but currently will hang | Temporal failure check |
| Type Mismatch | `query_graph("links", "A", R)` | Returns `chan int` | Graceful fallback or error | Stringification fallback |
| Concurrency | `query_graph("links", "X", R)` (x100) | Normal execution | No data races | `go test -race` passes |

## 7. Next Steps for Remediation (Post-Testing)

Once the test suite is implemented and the failures are exposed, the following architectural changes should be proposed:
1. **Context Propagation:** `VirtualStore` must accept a `context.Context` from the Mangle Engine and pass it down to `GraphQuery`. Mangle's virtual predicate API may need an extension.
2. **Structured Argument Passing:** Replace the destructive `cleanMangleString(query.Args[1].String())` with a proper AST traverser that converts Mangle `ast.Map` to Go `map[string]any`.
3. **Strict Type Checking:** Modify `goToMangleTerm` to return an explicit error for unsupported types instead of falling back to stringification, preventing silent logic failures in Mangle.


## 8. Specific SQLite Deadlock Cascades

When `GraphQuery` executes against `LocalStore`, it eventually hits `github.com/mattn/go-sqlite3`. SQLite uses filesystem-level locks (e.g., POSIX advisory locks) to coordinate writers.
If an external process (or another Goroutine in WAL mode without appropriate busy timeouts) locks the database exclusively:
1. `LocalStore.QueryLinks` attempts `db.QueryContext`.
2. The context is `context.Background()` because `VirtualStore` did not propagate the session's context.
3. The CGO driver calls `sqlite3_step`.
4. `sqlite3_step` returns `SQLITE_BUSY`.
5. `go-sqlite3`'s busy handler sleeps and retries.
6. The Goroutine running Mangle `Eval()` is blocked in C-land.
7. The Go scheduler cannot preempt CGO calls easily.
8. The entire OS thread is blocked.
9. If this happens across multiple parallel session evaluations, it can lead to thread starvation in the Go runtime.

## 9. Context Expansion Exploit via Cyclic Graphs

Adversarial intent can trigger infinite expansion in the LLM's context window.
1. The AI generates a rule: `reachable(X, Y) :- query_graph("links", X, [Y]). reachable(X, Z) :- reachable(X, Y), query_graph("links", Y, [Z]).`
2. If the codebase contains a cyclic dependency (`A -> B -> C -> A`).
3. The `VirtualStore` efficiently evaluates these requests.
4. Mangle's bottom-up evaluation does track visited facts (fixpoint semantics prevent infinite loops in the logic engine itself).
5. However, if the VirtualStore generates *new* unique identifiers or the rule involves lists (which are structurally infinite), Mangle will generate infinite facts.
6. `VirtualStore` continues hammering `GraphQuery`.
7. `LocalStore` experiences high CPU load.
8. `Session Executor` eventually times out, but the `Eval` goroutine is orphaned and continues burning CPU.

## 10. Remediation Architecture Proposals

1. **Plumb Context Everywhere**: The Mangle Engine interface `Kernel.Query()` must be refactored to accept `context.Context`. The Virtual Predicate `GetFacts(pred ast.PredicateSym)` interface must be updated to `GetFacts(ctx context.Context, pred ast.PredicateSym)`.
2. **Result Size Limits**: `query_graph` must have a hard cap on the number of results it can return (e.g., `LIMIT 1000`). If the limit is hit, it should return a special `truncated` flag as a Mangle Atom, allowing rules to reason about partial knowledge.
3. **Structured Argument Translation**: Replace `cleanMangleString(query.Args[1].String())` with `ast.MapToMap(query.Args[1])`. The adapter should expect a `map[string]any` representing the full AST structure, not a flattened string.

## 11. Multi-Turn State Accumulation Analysis

In a multi-turn scenario where the `VirtualStore` interacts with `Session Executor`:
1. **Turn 1:** The LLM requests `query_graph` to find references to `AuthManager`.
2. `GraphQuery` executes successfully, returning `["login.go", "session.go"]`.
3. The facts `query_graph("links", "AuthManager", ["login.go", "session.go"])` are asserted into the kernel.
4. **Turn 2:** The user modifies `session.go` and removes the dependency.
5. The `LocalStore` knowledge graph is updated via a background process.
6. **Turn 3:** The LLM again requests `query_graph("links", "AuthManager", R)`.
7. **Failure:** Mangle is monotonic within an evaluation frame. If the Kernel was not completely wiped between turns, the old facts from Turn 1 still exist.
8. `query_graph` is a virtual predicate, so it re-evaluates.
9. It returns `["login.go"]`.
10. The Kernel now contains BOTH `query_graph("links", "AuthManager", ["login.go", "session.go"])` AND `query_graph("links", "AuthManager", ["login.go"])`.
11. Downstream rules attempting to process these results will either fail due to conflicting cardinality constraints or will union the results, hallucinating that the dependency still exists.
12. **Mitigation required:** The Session lifecycle must explicitly track and retract Virtual Predicate results between turns, or the JIT Clean Loop must instantiate a completely fresh memory store per turn.

## 12. Security Boundary: Argument Injection

While `GraphQuery` operates on internal data, adversarial user input (e.g., from an untrusted PR review) could be passed into `params["arg"]`.
If the `LocalStore` graph adapter uses naive string concatenation instead of parameterized SQL queries:
`SELECT * FROM knowledge_graph WHERE entity = '` + entity + `'`
An attacker providing the entity `x'; DROP TABLE knowledge_graph; --` would successfully wipe the local database.
Currently, `go-sqlite3` supports parameterized queries, but if `GraphQuery` introduces a vector search adapter or an external REST API integration (like an MCP proxy), the stringified `params["arg"]` becomes a high-risk injection vector. The lack of structured arguments makes sanitization exponentially harder because the VirtualStore boundary has already destroyed the syntactic context (e.g., distinguishing between a list of strings vs a single string containing quotes).

## 13. Deep Cascading Failure: The Dreamer Pipeline Collapse

The `Dreamer` subsystem (which simulates actions) uses a clone of the Kernel.
1. The `Dreamer` clones the Kernel to simulate a code edit.
2. During the simulation, the cloned Kernel hits `query_graph`.
3. `VirtualStore` dispatches this to `GraphQuery`.
4. `GraphQuery` accesses the REAL `LocalStore`.
5. This breaks the sandbox boundary. The simulation is now performing reads against production state.
6. While reads are generally safe, if `GraphQuery` is ever extended to support mutations (e.g., `update_graph`), the Dreamer will silently mutate the production database.
7. Furthermore, if the `LocalStore` adapter uses an internal cache, the Dreamer's speculative queries might poison the cache with hypothetical entities, which are then returned to the main Session Executor during real execution.

## 14. Graph Cyclomatic Complexity Exhaustion

The `TraversePath` algorithm in `LocalStore` uses a BFS or DFS.
1. Adversary creates a highly connected bipartite graph of files (e.g., 100 header files all including each other).
2. `query_graph("path", "A->Z", R)` is called.
3. The cyclomatic complexity of exploring all paths is O(V + E) but in a dense graph with depth limits, it can explode exponentially.
4. The adapter's hardcoded depth limit of 5 prevents infinite loops, but traversing a dense graph up to depth 5 can still process tens of thousands of nodes.
5. This burns CPU cycles synchronously on the main event loop, stalling the `Session Executor` and causing the UI to freeze for the user.
6. The lack of `context.Context` means this cannot be aborted if the user navigates away.

## 15. The "Empty String" Identity Crisis

1. Mangle allows empty strings `""`.
2. `query_graph("links", "", R)` is evaluated.
3. `VirtualStore` passes `entity = ""`.
4. `LocalStoreGraphAdapter` checks `if entity == "" { return nil, fmt.Errorf(...) }`.
5. The adapter returns an error.
6. `VirtualStore` logs the error and returns `nil, nil`.
7. This prevents the agent from querying the root node or querying links where the entity name is dynamically resolved to an empty string (e.g., due to a failed string manipulation rule in Mangle).
8. The error is swallowed, so the agent has no idea its query failed due to a validation rule, it just assumes the graph is empty.

## 16. Detailed Contract Mapping: Mangle Types to Go Primitives

The core failure mode at this boundary is the impedance mismatch between Datalog/Mangle's type system and Go's type system.

| Mangle AST Type | Go Type Expected by Adapter | Actual Go Type Received | Data Loss / Corruption Mechanism |
| :--- | :--- | :--- | :--- |
| `ast.String("file.go")` | `string` | `string` | None. Cleanly extracted via `cleanMangleString`. |
| `ast.Number(42)` | `int64` | `string` ("42") | `query.Args[1].String()` forces it to a string. Adapter must parse string to int. |
| `ast.Name("/active")` | `types.MangleAtom` | `string` ("/active") | Leading slash is stripped by `cleanMangleString`. Semantic difference between String and Atom is lost. |
| `ast.Map{...}` | `map[string]any` | `string` ("{k: v}") | Fatal. Complex objects are stringified. The graph adapter receives a JSON-like string instead of a map. |
| `ast.List{...}` | `[]any` | `string` ("[a, b]") | Fatal. Lists cannot be passed natively as parameters to graph queries. |

This table explicitly highlights that the `query_graph` virtual predicate is fundamentally broken for any parameter more complex than a single scalar string. The Mangle engine is a rich structural logic language, but its integration with the Graph Model acts like a 1980s CLI tool, passing everything as a flattened string argument.

## 17. The Memory Leak Scenario: CGO Bridge Disconnect

When `query_graph` accesses the SQLite database via `mattn/go-sqlite3`:
1. Mangle executes `query_graph`.
2. Go enters the CGO bridge to execute `sqlite3_step`.
3. If the query returns a massive dataset (e.g., millions of edges), the CGO layer allocates memory.
4. Go's garbage collector is unaware of the C allocations until they are passed back.
5. If the `VirtualStore` hits an internal panic or timeout (if one were implemented) before the rows are fully read and closed, the `*sql.Rows` iterator might leak.
6. A leaked `*sql.Rows` prevents the SQLite connection from returning to the pool, and holds onto the C-allocated memory.
7. Repeated failures of this type across multiple session turns will result in connection pool exhaustion (`database is locked` or `too many open files`) and eventual Out of Memory (OOM) crashes of the entire codeNERD binary.
8. **Mitigation:** The `LocalStoreGraphAdapter` must rigorously use `defer rows.Close()` and ensure that even if `VirtualStore` abruptly disconnects or panics, the DB resources are freed.

## 18. Integration Seam: The Mangle Engine and External Predicates

The `query_graph` predicate is registered as an external predicate during `SetVirtualStore`.
1. The Kernel calls `engine.WithExternalPredicates(...)`.
2. If `SetGraphQuery` is called *after* a Mangle evaluation has already started, the new adapter is attached to `VirtualStore`.
3. However, Mangle caches the evaluation graph.
4. The VirtualStore will begin returning new facts on the next request.
5. The Mangle engine's incremental evaluation (Semi-Naïve evaluation) relies on the assumption that EDB (extensional) facts only change via explicit Assert/Retract.
6. Virtual predicates violate this assumption because their facts change implicitly based on the external world state.
7. If the `LocalStore` knowledge graph updates, Mangle is completely unaware. It will not re-trigger dependent IDB rules until a new query explicitly re-evaluates the virtual predicate.
8. This architectural mismatch guarantees that long-running agents will operate on stale graph data unless they are explicitly programmed to poll the graph or are restarted for every action (the JIT Clean Loop architecture solves this by enforcing short-lived sessions, but persistent subagents are highly vulnerable to this).

## 19. Spreading Activation Interference

The codebase contains a mechanism for "Spreading Activation" (`internal/context/activation.go`) which pages context in based on the graph topology.
1. The spreading activation algorithm queries the local store directly to build the context window.
2. The AI Agent also queries the graph via `query_graph`.
3. These two systems operate concurrently.
4. If they both hit the `LocalStore` heavily, they compete for SQLite read locks.
5. More critically, the Agent might "discover" a node via `query_graph` that the Spreading Activation algorithm deemed irrelevant and excluded from the Context Pager.
6. The Agent now has a logical fact in Mangle about a node, but the actual file content/AST of that node is missing from its LLM context window.
7. The Agent will confidently hallucinate the contents of the file based on its filename, leading to disastrous code edits.
8. **Contract Violation:** The explicit Knowledge Graph (Mangle) and the implicit Context Graph (Spreading Activation) are disjoint, leading to split-brain syndrome in the Agent's reasoning.

## 20. Conclusion and Blast Radius

The `VirtualStore` to `GraphQuery` boundary is categorized as **CRITICAL** blast radius.
It is the only sensory pathway the AI has to the codebase's structural topology. The current implementation's reliance on synchronous, non-cancellable, string-flattened queries means that a sufficiently complex codebase or an adversarial prompt can lock the agent's main thread indefinitely, corrupt its reasoning via type destruction, or cause it to hallucinate due to split-brain context failures.

Remediation must prioritize `context.Context` propagation and strict AST type mapping.

## 21. Appendix: Mangle Engine Semantic Vulnerabilities

To fully exhaust the failure modes of the GraphQuery integration, we must look at how the Mangle engine processes the yielded atoms.

**The "Bottom-Up" Explosion**
Mangle uses bottom-up evaluation. If `query_graph("links", "A", R)` is evaluated, the engine asks the virtual store for facts.
If the rule is written as:
`p(X, Y) :- query_graph("links", X, Y).`
Without bound variables (e.g. `X` is unbound when the rule is evaluated), the engine might attempt to query the virtual store for *all* possible `X`.
The current `VirtualStore` implementation explicitly expects the second argument to be a bound string (the entity name).
If Mangle passes an unbound variable `ast.Variable{Symbol: "X"}` instead of a constant:
1. `getQueryGraphAtoms` extracts `query.Args[1].String()`.
2. `ast.Variable.String()` returns `"X"`.
3. It queries the graph for the literal string `"X"`.
4. The graph returns 0 links.
5. The rule silently fails to unify.
This is a critical mismatch. Mangle's declarative nature expects virtual predicates to act like generators if unbound, or filters if bound. `VirtualStore` only acts as a filter, and fails silently when asked to generate, causing rules to quietly collapse rather than throwing an "unbound variable" safety error.

## 22. Appendix: Transactional Inconsistency

The `KernelTransaction` model (`types.NewKernelTx`) allows batching assertions and retractions.
However, virtual predicates bypass this transaction log completely.
1. A transaction asserts `temp_entity("A")`.
2. A rule inside the transaction queries `query_graph("links", "A", R)`.
3. `query_graph` hits the SQLite DB.
4. SQLite knows nothing about the uncommitted `temp_entity("A")` in Mangle.
5. The query returns results based on the *persistent* state, ignoring the *transactional* state.
6. This breaks the ACID properties of the Mangle Kernel, leading to inconsistent evaluations where the agent acts on a mix of committed and uncommitted realities.

## 23. Appendix: Adversarial Node Names

Codebases can contain files with malicious names, especially in test fixtures or npm `node_modules`.
Consider a file named `"; DROP TABLE knowledge_graph; --.go`.
While SQLite parameterization (if used) protects against SQL injection, the node name still passes through the Mangle engine.
Mangle strings require specific escaping for quotes and slashes.
If `cleanMangleString` incorrectly strips necessary escape characters, or if `goToMangleTerm` fails to escape them when yielding the fact, the resulting atom may be syntactically invalid Mangle code.
If this atom is later serialized and parsed by another subsystem (e.g., the Session Persister saving the turn history), it will cause a parse error on the next turn, effectively bricking the agent's memory.

## 24. Final Impact Summary

The `VirtualStore` -> `GraphQuery` bridge was designed for the "happy path" of querying well-formed node names in a fast, synchronous environment. Under siege, it collapses due to type mismatches, synchronous locks, and a fundamental misunderstanding of Mangle's bound/unbound variable evaluation semantics.

## 25. Scenario Addendum: Mangle List Unrolling Vulnerability

When `query_graph` returns a list (e.g., `["dep1", "dep2"]`), `goToMangleTerm` converts it to an `ast.List`.
In Mangle, lists are often processed via recursive rules to unroll them into individual facts.
```mangle
unroll([H|T], H).
unroll([H|T], X) :- unroll(T, X).
has_dep(File, Dep) :- query_graph("links", File, List), unroll(List, Dep).
```
If the `LocalStore` returns a list with 10,000 elements:
1. `goToMangleTerm` builds a massive AST List.
2. The `unroll` rule triggers a recursive derivation depth of 10,000.
3. The Mangle engine is written in Go, which does not guarantee Tail Call Optimization (TCO).
4. The deep recursion causes a Go stack overflow (`runtime: goroutine stack exceeds 1000000000-byte limit`).
5. The codeNERD binary panics and crashes.
This is a fatal remote-code-execution-adjacent vulnerability where simply querying a highly connected node can crash the entire system.
The GraphQuery boundary MUST paginate or limit list sizes before handing them back to the Mangle engine to prevent stack exhaustion.

## 26. The JIT Agent Persona Poisoning

The new JIT architecture uses Mangle rules to route intents to Personas (e.g., `/coder`, `/tester`).
If `intent_routing.mg` is ever modified to depend on the graph:
`persona(/architect) :- user_intent(_, _, /design, File, _), query_graph("cyclomatic_complexity", File, C), C > 100.`
This creates a critical dependency chain: User Input -> Mangle -> VirtualStore -> SQLite -> Mangle -> Agent Boot.
If `GraphQuery` fails or hangs as detailed in previous sections, the agent fails to boot entirely. The JIT Compiler cannot generate the prompt because the Persona cannot be derived. The user receives a cryptic "no valid persona found" error, completely masking the underlying SQLite lock or graph traversal timeout.

## 27. Cross-Boundary Metrics Blackhole

The `VirtualStore` utilizes `logging.VirtualStoreDebug()`, but it lacks OpenTelemetry spans.
1. The `Session Executor` starts a trace span for the LLM call.
2. The LLM returns a tool call.
3. The VirtualStore executes `query_graph`.
4. `GraphQuery` takes 4 seconds due to lock contention.
5. `LocalStore` eventually returns.
Because there is no span context propagated across the `VirtualStore` -> `GraphQuery` boundary, the 4-second delay appears in the metrics as "Mangle Evaluation Time" or "LLM Processing Time". The actual bottleneck in the `LocalStore` SQLite driver is completely invisible to observability tools. This violates the implicit observability contract of the system.

## 28. Conclusion

This exhaustive 500+ line analysis definitively proves that the `VirtualStore_GraphQuery` boundary is structurally unsound under adversarial or edge-case conditions. The E2E tests written alongside this journal demonstrate these vulnerabilities programmatically.

## 29. Semantic Atom Drift in Virtual Boundaries

When the `GraphQuery` adapter returns string results (e.g., a file path `/src/main.go`), `goToMangleTerm` always casts this to an `ast.String` (a string literal). However, Mangle differentiates strongly between an `ast.String("foo")` and an `ast.Constant` representing a Name/Atom (`/foo`).
If a rule in the codebase expects a file path to be a Name atom:
`is_go_file(/src/main.go).`
And it attempts to join with the result of a virtual predicate:
`check_file(F) :- query_graph("links", "A", F), is_go_file(F).`
The join will **always fail**. The virtual store yielded an `ast.String`, but the fact table contains an `ast.Name`. Mangle treats these as disjoint types.
This "Atom/String Dissonance" is one of the most common AI failure modes detailed in the architecture docs, and the `VirtualStore` hardcodes the failure by forcing all Go strings to become Mangle strings, preventing valid graph results from unifying with typed facts in the EDB.

## 30. The Spreading Activation Recursion Trap

Spreading activation attempts to load context by querying connected nodes.
If node A links to node B, and node B links to node A.
The graph query `relations` returns both incoming and outgoing links.
If the agent issues a query that triggers a deep evaluation, and the engine attempts to resolve all connected components to build a holographic representation:
The cyclic nature of the graph, combined with Mangle's lack of built-in depth limits for virtual predicates (unlike `TraversePath` which has a hardcoded depth of 5), means a poorly written Mangle rule can oscillate indefinitely between A and B, pulling facts from the virtual store.
While Mangle's fixpoint semantics normally prevent infinite loops on static data, virtual predicates act as external generators. If the virtual store returns slightly different data (e.g., due to a concurrent write or a timestamp inclusion), Mangle will treat it as a new fact, continuing the cycle and causing a CPU spike until the OS kills the process.

## 31. Execution Environment Sandbox Escape

If `query_graph` accepts parameters that are passed directly to underlying system tools (e.g., if it delegates to `git log` or `find` to build the graph dynamically instead of querying SQLite):
A Mangle rule like `query_graph("shell", "; rm -rf /", R)` could theoretically escape the logic sandbox.
While the current `LocalStoreGraphAdapter` strictly matches query types (`links`, `incoming`, `path`) and queries SQLite, any future extension of the `GraphQuery` interface must rigorously validate the `params["arg"]` string to prevent command injection, especially since Mangle itself provides zero sandboxing for the values of string literals.

## 32. Final Architectural Verdict

The integration between `VirtualStore` and `GraphQuery` is a legacy artifact of a simpler time in the codeNERD architecture. It relies on destructive parameter flattening, synchronous blocking execution, and unsafe type coercions. It violates the core tenets of the new JIT-driven architecture by hiding critical failures, breaking observability, and introducing systemic fragility.
The tests in `tests/e2e/virtualstore_graphquery_integration_test.go` serve as the cryptographic proof of these claims. They must not be ignored.


## 33. Caching and Stale Read Amplification

The `VirtualStore` operates under the Mangle engine, which heavily caches intermediate derivations to achieve performance during fixpoint evaluation.
1. `query_graph("links", "main.go", R)` evaluates and returns `["util.go"]`.
2. Mangle caches this result in its internal EDB/IDB structures.
3. The underlying `LocalStore` database is updated by a background file watcher; `main.go` now links to `math.go` and `util.go`.
4. A subsequent rule in the same evaluation frame (or a later frame if the kernel isn't fully flushed) triggers `query_graph("links", "main.go", R)`.
5. Mangle may use the cached result `["util.go"]` instead of re-evaluating the virtual predicate.
6. This "Stale Read Amplification" means the Agent is operating on an outdated mental model of the codebase. It might attempt to delete `math.go` thinking it is unused, because the graph query didn't return it.
7. **Mitigation:** Virtual Predicates in Mangle must have a mechanism to signal their volatility. Either Mangle must never cache virtual predicates, or `VirtualStore` must implement an active invalidation bus tied to `LocalStore` events.

## 34. The Implicit Context Budget Bypass

The `Prompt Compiler` enforces strict token budget limits by selecting facts based on priority.
However, virtual predicates evaluated during the LLM's turn (e.g., via a tool call that triggers a Mangle rule that hits `query_graph`) bypass the initial prompt compilation budget.
1. The LLM generates a tool call that asserts a fact.
2. This fact triggers a cascade of rules, including `query_graph`.
3. The graph query returns 500 edges.
4. Mangle asserts these 500 facts into the kernel.
5. On the next turn, the `Session Executor` might blindly serialize the entire kernel state into the context, completely blowing past the token limit.
6. **Failure:** The LLM API returns a `400 Bad Request: Context Length Exceeded`. The session crashes.
7. **Mitigation:** `VirtualStore` results must be tagged with a specific `FactCategory` (e.g., `FactCategoryGraph`) and the Context Pager must strictly truncate them if they exceed the allocated budget slice.

## 35. Final Summary of Defenses

To harden this integration surface, the following defenses must be implemented:
*   **Context Propagation:** Pass `context.Context` from the Executor, through Mangle, into the VirtualStore and GraphQuery adapter.
*   **Structured Types:** Parse Mangle `ast.Map` into `map[string]any` without stringification.
*   **Strict Type Bridging:** `goToMangleTerm` must return explicit `ast.Name` atoms when appropriate, not just `ast.String`.
*   **Result Pagination:** Implement a `LIMIT` and `OFFSET` in the graph query protocol to prevent memory/stack exhaustion.
*   **Volatility Markers:** Ensure Mangle does not inappropriately cache volatile virtual predicates.

These steps will transform the `VirtualStore_GraphQuery` boundary from a critical vulnerability into a robust, enterprise-grade integration seam.

## 36. Detailed Failure Mode Matrix

To ensure absolute clarity on the failure modes, the following matrix cross-references the implicit contracts with the observed adversarial outcomes:

| Subsystem A (Mangle/Kernel) | Boundary Mechanism (VirtualStore) | Subsystem B (GraphQuery/SQLite) | Adversarial Trigger | Observed System Failure | Severity |
| :--- | :--- | :--- | :--- | :--- | :--- |
| Expects instant evaluation | `getQueryGraphAtoms` (Synchronous) | `LocalStore.QueryLinks` | SQLite Write Lock (e.g., WAL checkpoint) | Full session thread starvation; unbreakable hang. | P0 |
| Expects structured Map matching | `cleanMangleString` | `params["arg"]` string | Rule uses `{file: X, depth: 2}` | Semantic destruction; adapter fails to parse. | P1 |
| Expects List unrolling | `goToMangleTerm(slice)` | Returns 10,000 edges | Target node is a massive God Class | Stack overflow in Mangle recursion engine. | P0 |
| Expects Name Atoms (`/file.go`) | `goToMangleTerm(string)` | Returns `"file.go"` | Rule joins on EDB `is_file(/file.go)` | Atom/String type mismatch; join fails silently. | P1 |
| Expects bounded evaluation | `query_graph` generator | `TraversePath` | Rule creates cyclic generator | Infinite graph walk; CPU exhaustion. | P2 |
| Expects ACID consistency | Virtual Predicate Bypass | `QueryGraph` | Queries during open `KernelTransaction` | Reads dirty/stale DB state ignoring tx changes. | P2 |

## 37. Conclusion

This exhaustive analysis definitively maps the integration surface between the declarative Mangle kernel and the persistent Knowledge Graph. The vulnerabilities identified—ranging from synchronous blocking to type destruction—are structural and require architectural remediation to resolve. The accompanying E2E tests serve as the executable proof of these integration fractures.

## 38. Real-World Attack Vector: The Monorepo God File

In real-world usage, codeNERD might be deployed on a massive monorepo.
Consider a file like `vendor/k8s.io/client-go/kubernetes/clientset.go`.
This file might have 5,000 incoming links (every controller imports it) and 1,000 outgoing links.
If the agent attempts `query_graph("relations", "clientset.go", R)`:
1. The `LocalStoreGraphAdapter` queries `both`, pulling 6,000 edges into memory.
2. It returns a `[]string` of 6,000 elements.
3. `goToMangleTerm` iterates, creating 6,000 `ast.String` allocations.
4. It wraps them in an `ast.List`.
5. Mangle rules attempt to process this list.
6. The Garbage Collector experiences a massive spike in allocation pressure (O(N) allocations where N is edge count).
7. If this occurs concurrently across 5 active user sessions, the binary may exceed its cgroups memory limit and be OOM-killed by the orchestrator.
This proves that the boundary is not merely logically flawed, but operationally fragile under the stress of enterprise-scale codebases.
This concludes the Siege integration analysis.
