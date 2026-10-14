# Quality Assurance Journal: Mangle Engine Tests
Date/Time: 2026-10-14 00:47:57 EDT

## 1. Introduction and Subsystem Selection
I chose to review `internal/mangle/engine.go` and its corresponding test suite, particularly focusing on `internal/mangle/engine_test.go` and `internal/mangle/engine_clear_query_test.go`.
The Mangle engine represents a high-leverage and high-criticality component of the codeNERD architecture.
It is the neuro-symbolic logic kernel that evaluates system constraints, agent state, tool permissions, and task verdicts.
This subsystem is the backbone of the JIT Clean Loop architecture introduced in Dec 2024.
Every subagent relies on this engine for routing, permissions, and knowledge synthesis.
Any failure here compromises the entire platform's integrity.

## 2. Methodology: Boundary Value and Negative Testing
My evaluation methodology specifically eschews 'Happy Path' scenarios.
The goal is to discover where the deterministic logic of the Go engine breaks down when subjected to the non-deterministic, hallucinatory outputs of the Large Language Model (LLM) agents that interact with it.
I focused on four primary vectors: Null/Empty, Type Coercion, Extremes (Arity/Length), and State Conflicts (Concurrency).

## 3. Vector Analysis: Null, Undefined, and Empty States
The system must gracefully handle states where facts, schemas, or queries are absent or structurally incomplete.

### 3.1 Empty Schema Loading
Currently, `TestEngineClear` covers the basic reset path but fails to probe empty schema strings.
`LoadSchemaString("")` or `LoadSchemaString("   \n\t")`.
Does the parser return an empty AST, nil, or does it panic?
The tests must assert that a syntactically empty schema results in a predictable, non-fatal error or a successful no-op, rather than crashing the evaluation loop.

### 3.2 Null Byte and Whitespace Queries
When an LLM hallucinates a query and returns only whitespace or unprintable characters, `Query(ctx, "   ")` is invoked.
The regex and tokenization in `parseQueryShape` must handle this.
A negative test must explicitly pass `"\x00"`, `"\n\n"`, and `"    "` and assert that a `ParseError` is generated.
A panic here could kill a long-running subagent.

### 3.3 Zero-Argument Facts for Arity>0 Predicates
If a schema defines `Decl active_user(Name) bound [/string]`, what happens if the system attempts `AddFact("active_user")` with an empty argument list?
The `AddFactsContext` logic must enforce arity at runtime.
The tests currently lack explicit coverage for this arity mismatch.
A test must try to insert zero arguments where one is expected, and two where one is expected, verifying strict rejection.

### 3.4 Empty String Coercion
When a predicate expects a name constant (e.g., `/user`), and receives an empty string `""`, the coercion logic in `types.Fact.ToAtom` must be robust.
Does it convert it to a nameless constant `/`? Does it reject it?
The boundary between an empty string and a valid Mangle name constant must be codified in tests to prevent silent data corruption.

## 4. Vector Analysis: Type Coercion Boundaries
Mangle enforces strict types (`/string`, `/number`, `/name`), but the Go host environment passes loosely typed `any` or `string` values.
This boundary is highly vulnerable to LLM hallucinations.

### 4.1 String-to-Number Overflows
An LLM might generate a massive number to fulfill a prompt requirement.
`AddFact("score", "999999999999999999999999999999")`. The schema expects `/number`.
Go's `strconv.ParseFloat` or `ParseInt` will fail.
The negative test must ensure this failure bubbles up cleanly as a type mismatch error, rather than panicking or inserting a corrupted zero value into the fact store.

### 4.2 The /etc/passwd Name Coercion Trap
As documented in the Mangle architectural audits, `types.Fact.ToAtom` contains heuristics that coerce strings into name constants if they look like names.
For instance, starting with a slash, no spaces. `"/etc/passwd"` might become a name, while `"/etc/passwd/x"` remains a string.
This is a massive vulnerability.
We must write tests that explicitly assert the exact boundary of this regex: a path with 2 slashes vs 3 slashes, spaces vs underscores.
Tests must prove that `AddFact` behaves deterministically based on this undocumented heuristic.

### 4.3 Query Matching with Type Discrepancies
When calling `QueryFacts("score", "100.0")`, the internal fact might be stored as the integer `100`.
The matching logic in `QueryFacts` (using `types.ExtractString`) might fail to match them, or it might incorrectly match them.
A negative test must insert a float and query for an int (and vice versa), verifying that strict Mangle type semantics are upheld.
Go's stringification must not mask the type difference.

### 4.4 Boolean Coercion Ambiguity
While Mangle lacks a native boolean, facts often use `/true` and `/false`.
If the host Go code passes the literal boolean `true`, how is it handled? Does it panic?
A test must attempt to pass a Go boolean and ensure it is properly rejected or correctly coerced into a `/name` constant.
This ensures type safety at the boundary.

## 5. Vector Analysis: User Request Extremes (LLM Hallucinations)
The "Creative Center" (LLM) operates without strict mechanical bounds.
The engine must defend itself against extreme structural inventions.

### 5.1 Extreme Arity Inventions
An LLM might invent a new schema on the fly to capture complex state: `p(a, b, c, ...)` with 10,000 arguments.
`AddFact` must allocate a slice for these arguments.
A malicious or hallucinated request could trigger an Out Of Memory (OOM) error.
We need a negative test that attempts to add a fact with 50,000 arguments.
It must assert that the engine rejects it with a specific length limit error rather than crashing.

### 5.2 Megabyte-Scale Query Strings
Similarly, a generated query string might be megabytes long, perhaps containing thousands of chained constraints.
`Query(ctx, string_of_10MB)`. The parser (`Mangle.g4` implementation) might exhibit non-linear time complexity.
For example, exponential backtracking on certain regexes.
The test must supply a massive, deeply nested query and assert that the context deadline or a parser limit aborts the operation safely.

### 5.3 Mangle Explosion and the Fact Limit
The Mangle engine is vulnerable to infinite fixpoints or massive Cartesian products during evaluation.
The architecture mitigates this with `defaultDerivedFactLimit` (500k).
A critical negative test must construct a program that deliberately exceeds this limit (e.g., recursive self-joins).
The test must assert that the evaluation aborts with `EVAL ERROR: fact size limit reached` AND that the engine state is not corrupted.
This means the partial facts don't leak into subsequent queries.

### 5.4 Extreme Depth of Stratification
Beyond the number of facts, the number of logical strata (dependency layers) is a vulnerability.
The current corpus has ~913 strata.
An LLM might generate a recursive program with 10,000 strata.
The analysis phase (`AnalyzeOneUnit` -> `Stratify`) must handle this without stack overflow.
We need a test that generates a chain of 10,000 interdependent rules and verifies that the analyzer either processes it or rejects it gracefully.

## 6. Vector Analysis: State Conflicts and Race Conditions
The engine operates in a highly concurrent environment, managing the JIT Clean Loop for multiple subagents.

### 6.1 Query vs. Clear Race Condition
The `Clear()` function is invoked heavily during JIT resets.
The `engine.go` implementation uses a `sync.RWMutex` and clones `e.queryContext`.
A crucial negative test must simulate a race: Goroutine A starts a long-running query.
Goroutine B calls `Clear()`.
The test must prove that Goroutine A completes its query against the *old* state without panicking.
It must also prove that Goroutine B's clear takes effect for all subsequent queries. The lock topology must be stressed.

### 6.2 Concurrent Schema Mutations
If two subagents attempt to load conflicting schemas simultaneously (`LoadSchemaString`), how does the lock handle it?
The parser modifies internal maps (`PredToRules`, `PredToDecl`).
The test must spin up 50 goroutines, each loading slightly different schemas.
It must verify that the resulting engine state is consistent and no map concurrent write panics occur.

### 6.3 Reset During Active Query
Unlike `Clear()`, the `Reset()` function completely nils out `e.queryContext`.
If a query is paused just after acquiring the RLock, and `Reset()` is called, what happens?
We need a negative test that deliberately delays the query execution while calling `Reset()`.
This can be done using hooks if available, or sheer concurrency volume.
It must assert that the engine does not dereference a nil pointer.

### 6.4 Context Cancellation Mid-Evaluation
The `Query` function takes a `context.Context`.
If the query involves a massive self-join and the context is cancelled, the engine must halt evaluation immediately.
A test must construct an expensive query, cancel the context 5 milliseconds later.
It must assert that the query returns `context.Canceled` promptly, proving that goroutines are not leaked.

## 7. Performance and System Implications
The identified boundary conditions have significant implications for system performance and stability.

### 7.1 Memory Allocation during Extreme Arity
If the LLM generates a predicate with high arity, allocating the `[]any` slice is fast.
However, if this occurs in a loop (e.g., thousands of facts), memory pressure spikes.
The garbage collector will struggle to keep up.
The negative tests for extreme arity protect the system from memory exhaustion DOS attacks generated by the LLM.

### 7.2 CPU Starvation during Parser Backtracking
The megabyte-scale query strings risk CPU starvation.
If the parser is not strictly bounded, a single hallucinated query could pin a CPU core at 100% for minutes.
The tests enforcing length limits or context timeouts ensure the codeNERD runtime remains responsive.

### 7.3 Lock Contention in the JIT Loop
The race condition tests (Query vs Clear) highlight a potential bottleneck.
If `Clear()` must wait for all long-running queries to finish (due to the RLock), a single slow query will stall the entire JIT Clean Loop.
This prevents new subagents from spinning up.
The test design ensures we verify that `Clear()` merely swaps the pointer, allowing concurrent execution, rather than blocking.

### 7.4 Garbage Collection Spikes
When `Clear()` discards the old `factstore`, a massive amount of memory becomes eligible for garbage collection.
If the LLM just triggered a 'Mangle Explosion' that hit the 500k limit, clearing that store will cause a noticeable GC pause.
While tests cannot easily assert GC pause times, the architecture must acknowledge that frequent clears of bloated engines are expensive.

## 8. Detailed Scenario: The 'Ghost Fact' Vulnerability
A critical vulnerability identified in the architectural audits is the persistence of 'Ghost Facts'.
Because Mangle evaluation is monotonic, a derived fact from a previous program might remain in the store.

### 8.1 The Stale Positive Fact Defeating Negation
If program A derives `q(1)`, and program B uses the rule `s(X) :- p(X), !q(X)`.
The stale `q(1)` will silently defeat the negation in program B, resulting in missing facts (`s(1)` is never derived).
This is insidious because it doesn't cause a panic; it causes a silent logical failure.

### 8.2 Testing the Differential Re-evaluation
The `codeNERD` engine attempts to solve this via the differential path (`internal/mangle/differential.go`).
However, tests must explicitly assert that `Clear()` or the differential update mechanism genuinely eradicates all traces of IDB (derived) facts before re-evaluating.
The test must execute program A, run `Clear()`, execute program B, and assert that `s(1)` IS derived.
This proves idempotency.

## 9. Detailed Scenario: The `fn:collect` Aggregation Flaw
Mangle's `fn:collect` aggregation function has documented quirks regarding map-iteration order and duplicates.

### 9.1 Non-Deterministic Ordering
As noted in the audits, `fn:collect` without `WithDeterministicOrder` results in unpredictable list ordering.
A negative test must run a `fn:collect` query hundreds of times and assert that the resulting order fluctuates.
This proves the necessity of the deterministic order flag in production rules.

### 9.2 Duplicate Accumulation
`fn:collect` keeps duplicates (one list entry per input row).
If an LLM incorrectly assumes it creates a set (like `fn:collect_distinct`), logical errors occur.
We need a test that intentionally feeds duplicate rows into an aggregation.
It must verify that `fn:collect` produces a larger-than-expected list, documenting this behavior for future QA automation scripts.

## 10. Architectural Deep Dive: Moded Externals and Panics
The engine supports 'externals' (Go functions called from Mangle).
These externals have modes (inputs vs outputs).

### 10.1 Unbound Variable Panics
If an external expects an input (mode `+`), but the rule provides an unbound variable, the engine panics.
This was documented as 'engine surprise 2'.
A negative test MUST construct this exact scenario: `q(X,Y) :- dbl_ext(X,Y), p(X).` where `dbl_ext` is called before `X` is bound.
The test must assert that `AnalyzeOneUnit` catches this and returns an error, rather than allowing the engine to panic during evaluation.

### 10.2 Constant Filters on Outputs
If an external is all-output, passing a constant to it acts as a filter.
However, the cache key behavior means that two rules calling the same external with different constants will result in two full invocations.
A test should verify this cache-busting behavior.
This documents the performance implications of how LLMs construct these calls.

## 11. Security and Authorization Bounds
The Mangle engine is deeply tied to the system's authorization model.
LLMs might attempt to generate rules that bypass security constraints.

### 11.1 Implicit Security Bypass
If a rule relies on a negation `!unauthorized(User, Action)` and the system fails to evaluate `unauthorized` correctly due to type issues (e.g., passing a string instead of a name), the system might implicitly allow the action.
Tests must specifically construct boundary cases where the authorization check relies on negation.
We must inject malformed data to ensure the system fails closed (denies access) rather than failing open.

### 11.2 Re-entrant Rule Evaluations
What happens if a rule's execution triggers an external Go function that attempts to re-enter the Mangle engine by adding a fact or running a sub-query?
The `sync.RWMutex` might deadlock, or the engine state might become corrupted.
A negative test must implement an external that calls back into the engine and verify whether the system deadlocks, panics, or handles it safely.

## 12. Generative Rule Evaluation and State Poisoning
When the JIT engine generates rules dynamically, there is a risk of evaluating malformed predicates.
If an LLM generates a rule that uses an undeclared predicate, `AnalyzeOneUnit` will catch it.
However, what happens if the rule uses a predicate that is declared but has a mismatching mode declaration?
For instance, a predicate declared as `bound[/string]` but used in a context expecting a `/number`.
The negative test must inject a perfectly structured rule with a subtle mode mismatch and assert that the parser or analyzer rejects it.
If it passes analysis and fails at runtime, this is a 'poisoned state' vulnerability.

### 12.1 Wildcard Bounds Mismatch
A related vulnerability occurs when wildcards (`_`) are used in predicates with strict bounds.
Does the analyzer correctly infer the bound of the wildcard, or does it bypass the check?
A negative test must instantiate a wildcard in a position where the bound is `/name` and attempt to join it with a `/string` column.
If the analyzer allows this, the join will silently fail at runtime, leading to empty result sets.

### 12.2 Silent Deletion of Unsafe Negations
As documented in M0 §4, an unbound named variable in a negated literal is deleted exactly like a wildcard.
`q(X) :- p(X), !r(X,Y).` becomes `q(X) :- p(X).`
This is a severe logical vulnerability.
A negative test must construct this exact pattern and assert that the resulting AST is flagged as unsafe.
If the engine still silently deletes the negation, the test suite must at least contain a test that verifies this deletion behavior so that future QA tools can scan the AST for it.

## 13. Deep Recursive Structures and Stack Limits
Mangle rules can be recursive. What happens when the recursion depth exceeds typical bounds?
If a rule is defined as `p(X) :- p(Y), X = Y + 1`, and the base fact is `p(1)`.
The evaluation will continue indefinitely until `defaultDerivedFactLimit` is hit.
However, what if the structure is not linear but tree-like?
`p(X, Y) :- p(X-1, Y), p(X, Y-1)`.
The number of facts grows exponentially.
A negative test must construct an exponential recursion and assert that the engine can halt it via context cancellation before it consumes all available system memory.

## 14. Complex Struct Comparison and Hashing
Mangle's `factstore` uses hashing to deduplicate facts.
If facts contain complex nested structures (e.g., lists of lists), the hashing algorithm must be robust.
A negative test must construct two identical, deeply nested facts and ensure they hash to the same value.
Conversely, it must construct two facts that are identical except for a single integer deep in the nested structure and ensure they hash differently.
A collision here would cause silent data loss.

## 15. Handling of the `/rule/...` ID generation
The `/rule/<hash>` IDs are generated based on the analyzed, rewritten clause.
If an LLM modifies the whitespace or reorders positive atoms, the hash might change or stay the same.
A negative test must supply two semantically identical rules with different formatting and assert whether the hashes match.
This is crucial for the `nerd why` provenance tool.
If the hashes don't match, the provenance tool might fail to find the rule source.

## 16. Wildcard and Builtin Premise Interactions
A wildcard in a positive premise drops that premise's sub-proof and every leaf under it.
A comparison builtin premise marks the node `[partial]` without dropping any leaf.
A negative test must construct a proof tree that combines these two features.
It must assert that the resulting proof tree is correctly structured and doesn't panic when traversed by the provenance recorder.
This ensures the transparency of the logic engine remains intact.

## 17. The `MultiIndexedArrayInMemoryStore` Fallback
The engine uses `SimpleInMemoryStore` for < 1024 facts, and `MultiIndexedArrayInMemoryStore` for larger sets.
The `SimpleInMemoryStore` drops hash-colliding facts based on source insertion order.
A negative test must create a scenario with exact hash collisions (e.g., `0` vs `0.0` depending on internal representation).
It must verify which fact survives in the `SimpleInMemoryStore`.
Then, it must insert 1025 facts to trigger the transition to `MultiIndexedArrayInMemoryStore` and assert that the collision behavior changes.
This exposes a non-deterministic state transition in the engine's core memory model.

## 18. Clause-Final Name Constant Grammar Bug
As documented, a name constant immediately followed by `.` at the end of a clause after `=` or `!=` fails to parse.
`q(X) :- p(X), X != /b.` fails with `missing '.'`.
A negative test must explicitly supply this malformed string to `parseQueryShape` or `LoadSchemaString`.
It must assert that the parser returns the exact error string expected, documenting this language quirk so that the JIT compiler can avoid generating such structures.

## 19. Cross-Kernel Shared Predicate Leaks
The policy corpus has rules that join facts living in different shard kernels.
If the `engine.go` instance is reused or improperly cloned, facts from one shard might leak into another.
A negative test must initialize two distinct `Engine` instances, load the same schema, populate Engine A with sensitive facts, and execute a query on Engine B.
It must assert that Engine B returns an empty set, proving absolute isolation of the fact stores despite shared schemas.

## 20. Advanced State Evaluation: The Stratification Challenge
When dealing with Mangle's evaluation model, stratification is a key safety mechanism to prevent paradoxical queries.
A critical boundary condition arises when an LLM generates a rule that is technically stratified but functionally cyclic due to indirect recursion across multiple predicates.
A negative test must generate a chain of 15 predicates, where `p1` depends on `p2`, `p2` on `p3`, down to `p15`, which then subtly loops back to `p1` through a negation.
The analyzer must correctly identify this as a stratification error and reject the program.
If it fails to do so, the engine will likely infinite loop or crash during evaluation, exposing a critical flaw in the `AnalyzeOneUnit` step.

## 21. Advanced State Evaluation: Builtin Function Overload
The engine supports a variety of builtin functions, including string manipulation and mathematical operations.
If an LLM hallucinates a query that attempts to execute a computationally expensive builtin function (e.g., a regex match) on a massive dataset, the evaluation could stall.
A negative test must populate the fact store with 100,000 strings and execute a query that performs a complex regex match on every single one.
The test must assert that the `context.WithTimeout` correctly interrupts the evaluation, preventing the engine from consuming excessive CPU resources and impacting the JIT loop.

## 22. Advanced State Evaluation: The `__tmp` Relation Sprawl
As documented in the audits, the `__tmp` relation is a crucial mechanism for handling complex joins and aggregations.
However, if an LLM generates a query with a massive number of variables and a complex aggregation, the `__tmp` relation could balloon in size.
A negative test must construct a query that intentionally maximizes the size of the `__tmp` relation, perhaps by joining multiple large tables without sufficient filtering.
The test must assert that the engine handles this gracefully, either by hitting the memory limit and returning a structured error, or by completing the evaluation within a reasonable timeframe.

## 23. Advanced State Evaluation: The Differential Update Flaw
The differential update mechanism is designed to improve performance by only re-evaluating parts of the program that have changed.
However, if the LLM generates a rule that depends on a non-monotonic builtin function (e.g., a time-based function), the differential update might produce incorrect results.
A negative test must create a scenario where a rule's output depends on external state that changes between evaluations.
The test must assert that the engine either correctly invalidates the differential update cache or returns a deterministic result regardless of the external state change.

## 24. Advanced State Evaluation: The `AssertBatch` Failure Mode
The `executor.go` file utilizes `AssertBatch` to process multiple facts simultaneously.
If an LLM generates a batch of 500 facts, and fact #250 is malformed (e.g., incorrect arity), what happens to the batch?
A negative test must simulate this exact scenario and assert the transaction semantics of `AssertBatch`.
Does it roll back the entire batch? Does it partially commit? The system must ensure atomicity to prevent leaving the fact store in a corrupted state.

## 25. Advanced State Evaluation: The `ApplyConstitutionalOverride` Trap
The system includes constitutional overrides to block certain actions.
If an LLM discovers a way to structure a query that bypasses the override by using an alias or a deeply nested structure, the system's safety is compromised.
A negative test must construct an adversarial query designed to evade a known constitutional override.
The test must verify that the engine's evaluation logic correctly applies the override regardless of the query's obfuscation, ensuring the fail-safe mechanism remains robust.

## 26. Advanced State Evaluation: The `shard_status` Misinterpretation
The kernel derives shard status from tool traces, but LLMs might generate output that confuses the status logic.
If a coder subagent outputs text containing 'TODO' but also successfully generates code, the status might be misclassified as `/incomplete`.
A negative test must simulate this confusing output and verify that the engine's logic correctly prioritizes the actual tool trace (the generated code) over the heuristic text matching.
This ensures that the orchestrator receives an accurate representation of the subagent's progress.

## 27. Advanced State Evaluation: The `turn_failed` Verdict Rule
The kernel uses a `turn_failed` rule to determine if a subagent's action was unsuccessful.
If an LLM generates a read-only intent that is technically successful but logically flawed, the rule might exempt it from failure due to the read-only exception.
A negative test must construct a scenario where a read-only intent produces a 'hollow success' and assert that the engine correctly identifies the failure, rather than relying on a Go fallback.
This enforces the principle that all verdicts should be derived by the Mangle logic, not Go hardcoding.

## 28. Advanced System Limits: The Maximum Recursion Depth
We must address the engine's handling of deeply nested AST nodes.
If an LLM generates a query with thousands of nested `AND` or `OR` operators, the parser might exceed Go's maximum recursion depth, leading to a stack overflow panic.
A negative test must construct a programmatically generated query with 10,000 nested `OR` conditions and pass it to `parseQueryShape`.
The test must verify that the parser catches the recursion depth limit and returns a `ParseError` before the Go runtime panics, ensuring the system's stability against adversarial query generation.

## 29. Advanced System Limits: Extreme String Constants
Beyond the arity and depth, we must consider the size of individual string constants within a query.
If an LLM hallucinates a query that contains a single string constant that is 50MB in size, the parser's tokenization process might allocate excessive memory.
A negative test must inject a query with a massive string constant and assert that the parser either processes it without exceeding memory bounds or rejects it based on a predefined token length limit.
This protects the JIT engine from memory exhaustion attacks triggered by oversized string literals.

## 30. Advanced System Limits: The Combinatorial Explosion in `fn:group_by`
The `fn:group_by` aggregation function can be highly memory-intensive if applied to a dataset with extreme cardinality.
If an LLM generates a query that groups by a column containing millions of unique values, the resulting hash map will consume significant RAM.
A negative test must populate the fact store with a high-cardinality dataset and execute a `fn:group_by` query.
The test must assert that the engine handles the memory pressure gracefully, perhaps by utilizing disk-backed sorting or by aborting the evaluation with an explicit memory limit error, rather than crashing the process.

## 31. Advanced System Limits: The `AnalyzeOneUnit` Bottleneck
The `AnalyzeOneUnit` phase is crucial for ensuring the safety and stratification of Mangle rules.
However, if an LLM generates a schema with thousands of highly interdependent rules, the analysis phase itself might become a performance bottleneck.
A negative test must generate a massive, densely connected rule graph and pass it to the analyzer.
The test must assert that the analyzer completes within a reasonable timeframe (e.g., under 1 second) or aborts with a timeout error, preventing the JIT loop from stalling during the schema compilation phase.

## 32. Advanced Integration: The `AssertBatch` Concurrency Conflict
While `AssertBatch` is designed for atomic insertions, what happens when multiple subagents attempt to execute `AssertBatch` concurrently on the same `Engine` instance?
If the locking mechanism in `NewConcurrentFactStore` is not optimized for batch operations, this could lead to severe lock contention and decreased throughput.
A negative test must spawn 50 goroutines, each repeatedly calling `AssertBatch` with 100 facts.
The test must assert that the overall throughput remains acceptable and that no deadlocks occur, validating the scalability of the engine's write path under high concurrent load.

## 33. Advanced Integration: The Context Revocation Scenario
In the JIT Clean Loop architecture, a subagent's execution context might be revoked unexpectedly (e.g., due to a higher-priority task preempting it).
If the subagent is in the middle of executing a complex Mangle query, the engine must immediately halt evaluation and release all resources.
A negative test must initiate a long-running query and asynchronously cancel the context from another goroutine.
The test must assert that the `Query` method returns immediately with `context.Canceled` and that all temporary memory allocated for the evaluation is promptly garbage collected, ensuring no resource leaks occur upon context revocation.

## 34. Advanced Integration: The Ghost Fact Re-emergence
Building upon the 'Ghost Fact' vulnerability discussed earlier, we must consider the scenario where a ghost fact re-emerges after a differential update.
If the differential update mechanism incorrectly assumes a fact is no longer relevant, it might fail to clear it from the IDB cache.
A negative test must construct a complex sequence of rule modifications and fact insertions designed to confuse the differential update logic.
The test must assert that, despite the adversarial sequence, the final query results exactly match the expected output of a full re-evaluation, proving the mathematical soundness of the differential update algorithm under extreme conditions.

## 35. Advanced Edge Cases: Engine Cleanup and Finalization
When an `Engine` instance is no longer needed, it should be properly closed or garbage collected.
However, if background goroutines (e.g., for file watching or asynchronous evaluation) are still active, they might prevent the engine from being fully reclaimed.
A negative test must initialize an engine, trigger several asynchronous operations, and then drop the reference to the engine.
It must assert that no goroutine leaks occur and that memory usage returns to a baseline level, confirming the robustness of the engine's lifecycle management.

## 36. Advanced Edge Cases: Invalid Configuration Parameters
The `EngineConfig` struct provides several parameters that dictate engine behavior.
If these parameters are initialized with extreme or invalid values (e.g., negative memory limits, empty logger instances), the engine initialization should fail safely.
A negative test must attempt to instantiate an engine with a configuration featuring zero limits across all constraints.
It must verify that the engine returns a precise validation error upon initialization rather than proceeding with an unstable configuration.

## 37. Final Summary of Quality Assurance Enhancements
The systematic application of boundary value and negative testing to the Mangle engine reveals numerous areas for improvement.
By addressing the gaps identified in this journal, the codeNERD framework will achieve a significantly higher level of robustness.
The inclusion of specific `// TODO:` comments in the test suite provides a clear roadmap for the engineering team to implement these critical safeguards.
This proactive approach to quality assurance is essential for building a reliable and secure neuro-symbolic platform.

## 38. Extrapolation: Schema Loading With Unicode Normalization
When loading schemas containing non-standard unicode characters, the parsing engine might incorrectly canonicalize identifiers.
A negative test must pass a schema with variables that utilize zero-width spaces or homoglyphs.
It must assert that these are either correctly identified as distinct variables or rejected entirely as invalid syntax.
Failing to do so exposes the system to injection and logic masking attacks.

## 39. Extrapolation: Malformed AST Injections During Debugging
During local debugging sessions, the tool allows developers to inject rules manually via the CLI.
If a malformed string is pasted directly into the rule evaluator bypassing the transducer, the parser must still catch structural errors.
A negative test must simulate this exact CLI bypass vector and assert that the internal error boundaries hold strong.
This guarantees that even privileged human intervention cannot inadvertently corrupt the state machine.

## 40. Extrapolation: Future Directions in Mangle Fuzzing
The final aspect of the testing strategy is the introduction of property-based fuzzing.
Instead of manually writing negative tests for arity and depth, a fuzzing harness should be constructed to bombard the engine with dynamically generated, structurally invalid rules.
The negative tests outlined in this journal will serve as the initial seed corpus for this fuzzing effort.
By doing so, we shift from reactive QA to proactive architectural defense, anticipating the infinite variability of generative logic.
