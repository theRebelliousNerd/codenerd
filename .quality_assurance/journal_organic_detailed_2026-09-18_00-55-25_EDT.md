# QA Automation Journal Entry: SchemaValidator Exhaustive Boundary Analysis
**Date and Time:** 2026-09-18 00:55:25 EDT
**Subsystem Reviewed:** `internal/mangle/schema_validator.go`

## 1. Executive Summary and Strategic Context

In this comprehensive review, I am acting as a Principal QA Automation Engineer specializing in Boundary Value Analysis (BVA), Negative Testing, and State Conflict Resolution.
My objective is to deeply evaluate the `SchemaValidator` component of the Mangle engine within the CodeNERD framework.
The `SchemaValidator` is arguably the most critical defensive layer in the system because it enforces schema compliance, preventing the system from hallucinating predicates or executing rules that have no valid data sources.

This subsystem acts as a fundamental 'drift prevention' mechanism.
CodeNERD utilizes generative AI models to construct plans, emit code, and synthesize Mangle facts to track system state and user intent. Because AI models are inherently non-deterministic and prone to hallucination, the `SchemaValidator` ensures that the facts and rules generated remain strictly grounded in reality.
Without robust validation, the engine could execute arbitrary derived facts leading to unpredictable state mutations.

By meticulously analyzing the production code and the corresponding tests in `internal/mangle/schema_validator_test.go`, my aim is to transcend standard 'Happy Path' and nominal error scenarios.
Instead, I will identify critical edge cases, structural constraints, performance boundaries, state conflicts, and other sophisticated negative testing vectors that could lead to system failure, silent misrouting, memory exhaustion, or security policy bypasses.
The analysis is structured to provide actionable vectors for enhancing the test suite and hardening the production code.

## 2. Methodology

The testing methodology employed in this analysis focuses on identifying systemic vulnerabilities in parsing, state management, and memory architecture.
Rather than just analyzing simple input variations, we evaluate the interaction between the Go memory model, the regular expression engine, the garbage collector, and the concurrency primitives.
The findings highlight areas where valid configurations can still lead to catastrophic failure due to resource exhaustion or race conditions.
The ultimate goal is to provide a rigorous, verifiable blueprint for fortifying the system.
## 3. Null, Undefined, and Empty Input Boundary Vectors

The system must remain deterministic and resilient when faced with the absolute absence of expected structural data.
While Go as a language ensures strings cannot be `nil`, they can manifest as:
- Zero-length strings (`""`)
- Strings containing only whitespace characters
- Strings containing non-printing byte sequences like null terminators (`\x00`)

### 3.1. Complete Program Evaluation Void (`ValidateProgram`)

When the primary entry point `ValidateProgram` is invoked with an entirely empty string, we must question how the underlying `ParseUnit` handles this absolute absence of data.
Questions to address in testing:
1. Does it return a graceful error indicating 'empty input'?
2. Does it construct an empty AST?
3. If an empty AST is constructed, does that subsequently cause a nil pointer dereference or index-out-of-bounds panic when passed into `AnalyzeOneUnit`?

The current test suite validates:
- A 'valid program'
- A 'parse error' explicitly caused by a missing parenthesis
- An 'unbound variable'
- An 'undefined predicate'
- A program consisting solely of comments

However, it completely lacks a baseline test for `ValidateProgram("")` or `ValidateProgram("   \n\t  ")`.
This is a fundamental BVA failure at the zero boundary. We must assert that passing an empty string definitively returns a benign 'no operations' state or a clearly typed error, rather than cascading into a panic.

### 3.2. Null Byte Injection in Lexical Analysis

If dynamic rule loading via `ValidateRule` or `HotLoadRule` receives a string maliciously or accidentally injected with null bytes, e.g., `candidate_action(/x) :- \x00user_intent(...)`, the behavior of `splitRuleBody` and the standard regex matchers becomes unpredictable.
While the Go `regexp` package can theoretically handle null bytes, string termination semantics in C-based systems (which might be relevant if SQLite or another CGO dependency processes these strings downstream for storage or query evaluation) might prematurely truncate the string.
This would cause a dangerous discrepancy where the Go validator approves a truncated, seemingly safe rule, but the underlying execution engine evaluates something entirely different.

A robust negative test must explicitly construct and pass strings with `\x00` embedded within:
- Predicate names
- Arguments
- Structural tokens (`:-`, `.`)
This ensures the parser explicitly rejects them with a syntax error, rather than silently truncating or accepting them.

### 3.3. Zero-State Schemas Initialization

The `NewSchemaValidator` constructor and subsequently `LoadDeclaredPredicates` might be initialized with a zero-length schema: `schemasText = ""`.
If the agent engine is mistakenly spun up during a cold start without properly loading the system schemas, the `declaredPredicates` map remains empty.
Consequently, when `ValidateRule` checks body predicates, it will flag absolutely everything as undefined.
While this fail-closed behavior is technically safe, we must explicitly test this boundary condition.

What if `schemasText` consists exclusively of whitespace?
What if it contains syntactically complete but functionally empty `Decl` statements like `Decl () bound [].`?
Would an empty declaration break the regex `declHeadPattern` or cause a panic in `countTopLevelArgs`?
These zero-boundary initializations require dedicated test cases to prove that the validation subsystem degrades safely.

### 3.4. Trailing and Leading Whitespace Erosion

Often, data pipelined from large language models includes inconsistent spacing.
A negative test must explore what happens when `HotLoadRule` receives strings with enormous amounts of leading or trailing whitespace.
For example, consider a 10MB string that is entirely spaces followed by a single valid rule.
Does the `strings.TrimSpace` call allocate a massive new string in memory, causing a memory spike?
We should test feeding strings heavily biased with:
- Non-breaking spaces (U+00A0)
- Zero-width spaces
- Other Unicode whitespace variants
This verifies if `strings.TrimSpace` handles them, or if they evade trimming and break the `learnedHeadPattern` regex which relies on specific whitespace boundaries.

## 4. Type Coercion, Arity Manipulation, and Integer Boundaries

Although Go is a strongly, statically typed language preventing the direct passing of an integer where a string is expected at compile time, the semantics of Mangle introduce dynamic typing logic.
Within Mangle, types are declared dynamically in bounds, and arity is represented as a runtime integer within the Go maps.

### 4.1. Negative Arity Bound Exploitation

The function `SetPredicateArity` accepts a `string` predicate name and an `int` arity value.
What happens if an external source or a misconfigured schema definition passes a negative arity? Consider `SetPredicateArity("foo", -5)`.
Later during evaluation, `CheckArity` retrieves this `-5`.
The validation logic in `CheckArity` explicitly states:
```go
expectedArity := sv.GetArity(predicate)
if expectedArity < 0 {
    return nil // Unknown arity - skip check
}
```
Thus, a negative expected arity *completely bypasses the arity check mechanism*.
An attacker, or a hallucinated capability generation, could intentionally define a predicate with a negative arity to silently disable validation for that specific critical predicate.

We urgently need test coverage to ensure:
1. `SetPredicateArity` explicitly rejects negative values (perhaps panicking or logging a fatal error during schema load).
2. Alternatively, that `CheckArity` handles negative numbers securely rather than treating them as 'unknown and therefore valid'.

### 4.2. Extreme Upper Arity Limits and Parsing Exhaustion

If we manipulate the schema to set the expected arity to `math.MaxInt32`, what happens during the evaluation phase?
When `countTopLevelArgs` encounters an actual rule attempting to pass 10,000 or 1,000,000 arguments, it iterates over every single character in the argument string to count commas and track quote depth.
There is currently no maximum arity limit enforced at the schema validation layer.
The system could easily run out of memory or consume excessive CPU time (a classic Denial of Service vector) just parsing excessively long, hallucinated argument lists.
We must establish a clear upper boundary test: verifying how the validator behaves when fed rules with 1,000, 10,000, and 100,000 arguments.
The system should reject unrealistic arities immediately during parsing rather than attempting to evaluate them.

### 4.3. Arity Mismatch Type Coercion Simulation

What happens if a schema declares an arity of 3, but the engine is fed an array or structurally distinct object simulating 3 arguments?
For instance, if the validation relies strictly on comma counting within parentheses, `countTopLevelArgs` might be fooled by nested strings or functions.
The current test suite must add tests validating that complex arguments (e.g., nested functions or tuple values) correctly preserve the top-level arity count without causing off-by-one errors in the schema validator.

### 4.4. Struct and Map Arity Extraction

If Mangle evolves to support struct or map literals as arguments, how does `countTopLevelArgs` behave when commas exist inside `{}` brackets instead of `()`?
Currently, the parser only tracks depth for `(` and `)`.
A test should verify that injecting JSON-like dictionary payloads into arguments does not break the arity count by falsely identifying commas inside a dictionary as top-level argument separators.

## 5. User Request Extremes (Stress, Scale, and Frontier Benchmarks)

The CodeNERD framework is designed to handle immense, brownfield monorepos and extremely long, context-heavy conversations.
This necessitates the generation and validation of massive, complex Mangle programs.

### 5.1. The Ten-Million Line Program Validation Vector

If a user request requires the agent to understand a 50 million line monorepo, the resultant Mangle fact generation phase might produce a program containing tens of millions of lines of `file_topology` facts.
When `ValidateProgram` is called, it loads this entire massive text block into memory as a single contiguous string, and then calls `ParseUnit`.
Furthermore, the regular expressions used in `extractDeclsFromText` execute over the entire string simultaneously (`FindAllStringSubmatchIndex`).
This is a massive `O(N)` operation that could easily require gigabytes of RAM.
On a resource-constrained device (e.g., an 8GB RAM laptop), this will inevitably cause an Out-Of-Memory (OOM) panic, abruptly terminating the agent.
We must implement a stringent boundary stress test that programmatically generates a 100MB string of syntactically valid facts and ensures the `SchemaValidator` processes it within an acceptable timeframe, or fails gracefully without crashing the host process.
Memory profiling during this test is essential.

### 5.2. Deep Parenthesis Nesting and Stack Overflow Vulnerabilities

Within the `validateHeadArity` and `balancedArgs` utility functions, parsing depth is tracked using simple integer counters:
- `depth++` when encountering `(`
- `depth--` when encountering `)`

If a malicious input, or a heavily hallucinated rule string, contains extreme nesting—e.g., `foo((((((((((...))))))))))` with 100,000 nested parentheses—the initial string parsing might survive because it relies on a flat `for` loop rather than recursion.
However, what happens when this deeply nested string is subsequently passed to `ParseUnit` to build the actual Abstract Syntax Tree (AST)?
Does the AST builder rely on recursive descent parsing?
If so, nesting 10,000 parentheses will immediately cause a stack overflow panic, crashing the entire Go runtime.
We require a dedicated negative test that feeds an artificially deep structure like `((...))` nested 10,000 times directly to `ValidateLearnedRule` to categorically verify that the parser handles depth limits safely, likely by returning a 'max depth exceeded' parse error rather than panicking.

### 5.3. Predicate Name Length Extremes and Memory Duplication

The regular expression `(?m)^([a-z_][a-z0-9_]*)\s*\(` bounds valid predicate names to alphanumeric characters and underscores.
However, it notably places absolutely no limit on the length of that name.
An AI hallucinating a predicate name consisting of 1 million characters will successfully have that name stored in the validation map.
While Go map keys are memory pointers, allocating the 1MB string itself consumes significant memory.
More critically, when validation fails and the system builds an error message using `fmt.Errorf('rule uses undefined predicates: %v', undefined)`, that entire 1MB string will be duplicated in memory and potentially flushed to application logs, causing severe log bloat and further memory pressure.
We need a negative boundary test asserting that predicate names exceeding a reasonable threshold (e.g., > 255 characters) are immediately rejected by the lexer or schema loader.

### 5.4. Extreme Rule Body Densities

While arity deals with arguments per predicate, rule density deals with predicates per rule.
A rule like `action(X) :- pred1(X), pred2(X), ..., pred10000(X).` might parse correctly, but how does `ValidateProgram` perform when calling `unsourcedPremisePredicates` on a rule with 10,000 premises?
The `seen` map allocations and iterations will scale linearly per rule.
If the program has 100,000 rules, each with 1,000 premises, the validation overhead becomes astronomical.
A negative test must benchmark the compilation and validation of ultra-dense rule bodies to identify acceptable thresholds.

### 5.5. Massive Variable Binding

Similarly, rules can bind massive numbers of variables across premises: `result(X1, ..., X1000) :- query(X1), ..., query(X1000).`
The validator parses these variables as symbols.
Testing the memory limits when thousands of distinct variable symbols are instantiated per rule body is crucial for proving the engine's scalability against complex logical derivations.

## 6. State Conflicts, Concurrency, and Race Conditions

The most severe and glaring gap in the current `SchemaValidator` test suite is the complete lack of concurrency testing.
The CodeNERD architecture extensively utilizes asynchronous workers, multiple agent shards (e.g., perception, tactile, execution), and concurrent evaluation loops.
The validator is a shared resource.

### 6.1. Concurrent Map Access Panics in SchemaValidator

The `SchemaValidator` struct maintains state using two fundamental Go maps:
1. `declaredPredicates`
2. `predicateArities`

By definition in the Go language specification, maps are absolutely *not* thread-safe for concurrent read/write operations.
If one goroutine (e.g., the execution pipeline) is validating a rule via `sv.ValidateRule`, it performs a read operation on `sv.declaredPredicates`.
If, precisely at that moment, another concurrent goroutine (e.g., the feedback loop or dynamic schema updater) is hot-loading a new learned schema or rule via `sv.HotLoadRule` or `sv.LoadDeclaredPredicates`, it performs a write operation to that exact same map.
This will trigger Go's built-in race detector and cause an immediate, unrecoverable, fatal panic:
```
fatal error: concurrent map read and map write
```

The current unit tests execute strictly sequentially.
They never invoke `LoadDeclaredPredicates` and `ValidateRule` on the same shared `sv` instance from multiple goroutines simultaneously.
We must urgently add a state conflict test utilizing `sync.WaitGroup` to fire 1,000 concurrent read operations and 1,000 concurrent write operations.
If this subsystem is instantiated once and shared across worker threads, the struct must be refactored to encapsulate the maps behind a `sync.RWMutex` to guarantee thread safety.

### 6.2. Double Loading, Idempotency, and Phantom State Persistence

If `LoadDeclaredPredicates` is invoked multiple times on the same validator instance, it currently simply appends new discoveries to the existing state maps.
If the underlying `schemas.mg` file is updated on disk (e.g., a deprecated predicate is removed), and the validator is told to reload, the validator will incorrectly still believe the removed predicate is valid because the internal map retains the stale, phantom state.
There is no `sv.Reset()` method, nor is there map clearing logic at the beginning of `LoadDeclaredPredicates`.
A critical test scenario should verify idempotency: asserting that removing a `Decl` statement and reloading the schemas definitively invalidates the removed predicate.
As currently implemented, the system fails this state transition.

### 6.3. Interleaved Schema Updates and Validation Inconsistencies

Without a mutex, even if map access doesn't panic (e.g., due to specific memory interleaving), validating a large program while the schema is updating can lead to partial validations.
Half the rules might be validated against Schema V1, and the other half against Schema V2.
This non-deterministic validation state can allow invalid rules to pass or valid rules to be rejected based purely on timing.
A test must simulate this interleaving to enforce that validations are atomic transactions against a frozen snapshot of the schema.

### 6.4. Unsafe Cross-Instance Data Bleeding

If `NewSchemaValidator` copies maps rather than instantiating them, or if global state like `mangleBuiltins` is modified concurrently (although it is declared as `var` and seemingly read-only), data could bleed between isolated agent sessions.
Tests must construct multiple concurrent instances of the validator with entirely disjoint schemas to ensure zero crosstalk.

## 7. Architectural Guide to High-Leverage Mangle Tests in Go

Based on deep analysis of 'AI Failure Modes', here is how we must structurally test logic like the SchemaValidator and broader Mangle constraints within CodeNERD.

### 7.1. The 'Clean Slate' Fact Store Requirement

Mangle’s evaluation is monotonic and stateful. Reusing a store across tests leads to 'ghost facts' from previous runs contaminating the current fixpoint.
**Requirement:** Always instantiate a `factstore.NewSimpleInMemoryStore()` (or equivalent) inside your test loop.
**Why:** Ensures idempotency. A test checking for 'empty results' will fail if the store retains facts from a prior 'success' test.

### 7.2. The Analysis Phase (`analysis.Analyze`)

Do not just run `Eval`. You must explicitly test the safety of your logic.
**Requirement:** Parse the rules, then run `analysis.Analyze(program)`.
**Why:** This catches Stratification Errors (negation cycles like `p :- not p.`) and Safety Errors (unbound variables) before execution. A test that skips this might pass on a permissive engine configuration but fail in strict production environments.

### 7.3. Type-Strict AST Helpers

The most common AI failure is the 'Atom/String Dissonance.' Mangle treats `/active` and `"active"` as disjoint types.
**Requirement:** Use helpers that force you to choose the type.
- `ast.Name("active")` corresponds to `/active`.
- `ast.String("active")` corresponds to `"active"`.
**Why:** Passing raw Go strings often defaults to Mangle strings, causing joins to fail silently (zero results) when the schema expects Atoms.

### 7.4. Avoiding 'Stringly Typed' Assertions

Avoid converting results to strings for comparison (e.g., `res.String() == "p(/a)"`).
**Why:** Datalog sets are unordered. `[A, B]` is logically identical to `[B, A]`, but their string representations differ. String matching leads to flaky tests.
**Do:** Use set membership checks (`store.Read(...)`) to verify specific facts exist.

### 7.5. Avoiding 'Empty' Result False Positives

Avoid assuming that `err == nil` equals success.
**Why:** In logic programming, a bug often manifests as an empty result set, not an exception. A join between disjoint types (Atom vs String) produces zero tuples, which is a valid but incorrect state.
**Do:** Always assert that expected facts are present.

### 7.6. Golden File Testing for Logic

For complex recursive rules (e.g., transitive dependencies or ACLs), hardcoding Go structs is brittle.
**Focus:** Store the expected IDB (derived facts) in a `.golden` file.
**Method:** Serialize the store content after evaluation and compare it against the file. This detects subtle regressions in join ordering or derivation limits.

### 7.7. Termination Verification

Focus on recursive rules involving constructors or arithmetic.
**Test:** Feed a cyclic graph into your recursive rules.
**Verify:** The engine halts (reaches a fixpoint) within a strict timeout (`context.WithTimeout`). This proves your logic does not contain infinite generation loops (e.g., `p(X+1) :- p(X).`).

## 8. Detailed Analysis of Regular Expression Dependencies

The validator's logic is heavily dependent on regular expressions to extract structural information before handing it off to the formal parser. This reliance introduces several subtle parsing vulnerabilities.

### 8.1. `declHeadPattern` Bypass via Whitespace Indentation

The core schema extraction pattern is defined as `(?m)^Decl\s+([a-z_][a-z0-9_]*)\s*\(`. The inclusion of the `(?m)` flag dictates that the `^` anchor matches only the absolute start of a line.
Consequently, if a `Decl` statement is perfectly valid Mangle syntax but happens to be indented with a single space or tab (e.g., `    Decl foo() bound [].`), the regex will completely fail to match it.
This means perfectly valid semantic definitions will be silently ignored by the `SchemaValidator`.
Later, when rules attempt to use these predicates, they will inexplicably fail validation, leading to highly confusing debugging sessions.
A negative test must supply a schema containing heavily indented, syntactically valid `Decl` statements and verify that they are successfully loaded. Under current logic, this test will fail.

### 8.2. `learnedHeadPattern` Bypass via Preceding Content

The pattern used for learned rules is `^([a-z_][a-z0-9_]*)\s*\(`, notably *without* the `(?m)` multiline flag.
It operates on strings that have been processed by `strings.TrimSpace`.
However, if a learned rule is dynamically generated across multiple lines without a clear starting boundary, or if there is a preceding comment block that the parser failed to strip entirely, it might successfully bypass the forbidden heads check.
For instance, the highly restricted forbidden head `permitted` could theoretically be masked if it is not positioned at the very first byte of the string after trimming.
The test suite needs to inject complex, multi-line strings with embedded comments before the rule head to ensure the security boundaries cannot be evaded.

### 8.3. Catastrophic Backtracking Risks in Parsing

While the current regexes are relatively straightforward, if the `[a-z_][a-z0-9_]*` pattern is evaluated against massive, malformed strings containing millions of characters that almost match but eventually fail, it might trigger regex catastrophic backtracking.
We need a negative test that feeds pathological strings engineered to maximize backtracking evaluation time, ensuring the Go standard library regex engine caps execution or processes it linearly without freezing the system.

### 8.4. Unicode Bypass in Regex Anchors

The `[a-z]` class in regex typically matches ASCII letters.
If an attacker uses Cyrillic characters that look identical to Latin `a-z` (homoglyphs), the regex will fail to match them.
If the underlying Mangle engine permits full UTF-8 predicate names, this discrepancy means the schema validator ignores Unicode names, while the engine evaluates them.
A test must verify how Unicode homoglyphs are processed by the validation pipeline.

## 9. Execution Context and Authorization Validation Constraints

The Mangle execution context within CodeNERD introduces complex security requirements.
The schema validator is tasked with preventing unauthorized mutation of core system state by enforcing a blacklist called `forbiddenLearnedHeads`.

### 9.1. Casing Subversion and Normalization Failures

The Mangle language specification dictates that uppercase letters represent variables, while lowercase strings represent constants or predicates.
If a malicious user or hallucinating AI bypasses the lowercase restriction—perhaps through a bug in the AST or string interpolation—could they assert a protected predicate using alternate casing, such as `PERMITTED(/dangerous)`?
The regex `[a-z_]` strictly expects lowercase.
If the AST parser subsequently normalizes casing, or if the underlying evaluation engine treats predicate names case-insensitively, a rule rejected by regex might slip through validation and execute.
We must introduce negative tests verifying that mixed-case predicate names are uniformly rejected or securely normalized across the entire validation pipeline.

### 9.2. System Shard State Spoofing via Arity Variance

The forbidden map explicitly prohibits learned rules from defining `system_shard_state`.
However, what happens regarding arity?
If the legitimate `system_shard_state` predicate requires an arity of 2, and an AI agent derives a novel fact `system_shard_state(A, B, C)` with an arity of 3, does the system treat it as a distinct, isolated predicate namespace, or does it attempt to merge the facts, potentially crashing the engine?
The arity validation logic executes, but crucially, it *only* enforces arity if the schema explicitly declares an expected arity for that predicate.
We need integration tests to confirm that forbidden heads are categorically rejected regardless of any arity variations attempted by the input string.

### 9.3. Cross-Namespace Collision Attacks

If the system introduces namespace prefixes (e.g., `core.permitted`), does the `forbiddenLearnedHeads` check handle fully qualified names?
A test must evaluate if asserting `core.permitted` bypasses the check for `permitted`, allowing an attacker to inject facts directly into core namespaces.
The validation logic must explicitly parse and evaluate fully qualified paths against the forbidden registry.

### 9.4. Bypass via Aliasing and Desugaring

Can an attacker define an alias or macro that bypasses the literal string check?
If the engine supports macro expansion, a rule like `macro_permit(X) :- ...` could expand into `permitted(X)` post-validation.
Tests must confirm that all AST transformations are applied *before* the forbidden heads check, or that the validation strictly analyzes the fully desugared AST rather than just the literal source string.

## 10. String Splitting and Lexical Fragmentation Vectors

The custom `splitRuleBody` and `headText` functions perform manual string traversal to separate rules from facts, looking for the `:-` operator while attempting to ignore contents within string literals.

### 10.1. Escaped Quote Subversion

The loop tracks string state: `if c == '\\' { escaped = true } else if c == '"' { inString = false }`.
What happens with complex escape sequences, such as an escaped backslash preceding a quote: `"\\"`?
If the state machine misinterprets the escape sequence, it might incorrectly toggle the `inString` boolean.
This would cause the parser to treat subsequent actual rule structure as part of a string literal, or conversely, evaluate a string literal as executable rule code.
If an attacker crafts a string literal containing `:-`, and the quote tracking fails, `splitRuleBody` will fracture the rule in the wrong place, potentially exposing validation bypasses.
We require extensive fuzzer-style negative testing on the custom string traversal loops using highly complex, layered escape sequences to verify robustness.

### 10.2. Multi-byte Unicode Tokenization Failures

The custom parser explicitly loops over the string using byte indexing: `for i := 0; i+1 < len(ruleText); i++`, and compares against single-byte values like `'"'` and `':'`.
While Mangle syntax relies on ASCII control characters, user intents and constraints often contain multi-byte Unicode characters (e.g., emoji, complex language graphemes).
Because Go strings are UTF-8, iterating by byte index means that an index might point to the middle of a multi-byte sequence.
If an attacker intentionally crafts a multi-byte character where one of the continuation bytes happens to match the ASCII value of `:` or `-`, the byte-level comparison might mistakenly identify it as the rule separator `:-`.
We must construct negative tests embedding specific, adversarial Unicode sequences to prove that the byte-level tokenizer does not accidentally trigger on continuation bytes, ensuring complete Unicode safety.

### 10.3. Malformed String Termination

If a rule string contains an unclosed quote `"unterminated string literal... :- action(/run)`, the `splitRuleBody` function will never exit `inString` mode and will therefore fail to find the `:-` separator, treating the entire text as a single fact head.
This bypasses rule validation entirely.
We need tests ensuring that unterminated strings are appropriately flagged as fatal parse errors rather than quietly processed as facts.

### 10.4. Escape Sequences at String Boundaries

If a string ends abruptly with a single trailing backslash (`"some string\`), does the tokenizer read past the end of the array seeking the escaped character, leading to an index-out-of-bounds panic?
The test suite must specifically inject trailing escapes to verify the loop termination conditions are robust against malformed syntax.

## 11. Subsystem Integration and Cross-Context Behaviors

The schema validator does not exist in a vacuum; it sits between the LLM output generator and the Mangle engine.

### 11.1. AST Transformation Desync

The validator uses regex to check predicates, but relies on `analysis.AnalyzeOneUnit(parsed, nil)` to validate entire programs.
If the AST desugaring logic applied during analysis (like transforming negations or aggregations) generates synthetic internal predicates that differ from the initial lexical scan, there is a risk of desynchronization.
The validator ignores `sym.IsInternalPredicate()`, but what if a user manually writes an internal predicate name like `__synthetic_1`?
We must test if manual injection of reserved internal predicate names can bypass the schema check or collide with actual generated symbols, causing evaluation panics.

### 11.2. CGO SQLite Dependency Bleed

While the validator operates in Go, the downstream Mangle evaluation may rely on SQLite via CGO for fact resolution.
If the validator passes a predicate name containing valid Go characters that happen to trigger SQL injection or reserved keyword errors in SQLite (e.g., predicate names like `SELECT` or `DROP`, which might bypass regex if casing is manipulated), the evaluation engine could crash.
A boundary test should encompass injecting SQL reserved keywords as valid predicate names to verify the entire stack remains isolated and safe.

### 11.3. Type Resolution across Contexts

When `ValidateProgram` calls `analysis.AnalyzeOneUnit`, does it correctly resolve temporal operators or aggregated types?
If the schema bounds define a predicate expecting a temporal atom, but the string provides a standard atom, does the validator flag it as a mismatch, or does it silently pass?
Integration tests must assert type bounds strictly across all validation checkpoints.

### 11.4. Built-in Function Spoofing

The system maintains a map of `mangleBuiltins`.
If an attacker defines a learned rule `count(X) :- ...`, overriding a core mathematical primitive, what is the sequence of resolution?
The validator should block the redefinition of builtins.
Tests must explicitly attempt to define rules for `count`, `sum`, `applyFn`, and `match` to verify they are permanently protected from mutation.

## 12. Memory Allocation and Lifecycle Metrics

A robust validation engine must not leak resources over long-running daemon processes.

### 12.1. Validator Instance Lifecycle

If `NewSchemaValidator` is called repeatedly for every single LLM interaction, the garbage collector will be heavily taxed allocating and destroying map instances.
Conversely, if it is maintained as a singleton, it becomes susceptible to the state conflicts outlined in Section 6.
Tests must profile memory allocation during rapid, continuous instantiation of `SchemaValidator` to guarantee it is optimized for high-throughput messaging environments.

### 12.2. String Pointer Retention

Go strings are immutable, and taking a substring (e.g., `text[m[2]:m[3]]` in `extractDeclsFromText`) does not copy the underlying byte array; it retains a pointer to the original memory block.
If `sv.schemasText` is a massive 100MB string read from disk, and `sv.declaredPredicates` stores a small extracted predicate name, the entire 100MB backing array is kept alive in memory indefinitely because the map key points into it.
This is a severe memory leak vector.
Tests must utilize `runtime.MemStats` to verify that massive schema strings are fully garbage collected after parsing, meaning the extracted string keys must be explicitly cloned (e.g., `strings.Clone`) before insertion into the map.

### 12.3. Map Deallocation Characteristics

As noted earlier, deleting keys from a Go map does not shrink its underlying allocated bucket size.
If a schema undergoes heavy churn, adding and deleting thousands of predicates over hours of operation, the map memory will grow monotonically.
A prolonged stress test must monitor map memory consumption and verify if periodically copying the map to a fresh instance is necessary to reclaim memory.

## 13. Conclusion and Strategic Recommendations

The `SchemaValidator` is structurally sound for nominal, well-formed Mangle syntax.
However, this exhaustive Boundary Value Analysis reveals significant exposure to extreme inputs, concurrent state mutation panics, and subtle bypasses related to whitespace, arity coercion, string pointer retention, and byte-level tokenization.

Immediate remediation should prioritize:

1. **Thread Safety:** Implement `sync.RWMutex` around the state maps to prevent concurrent read/write panics in distributed worker environments. This is critical for scaling.
2. **State Reset:** Ensure `LoadDeclaredPredicates` safely clears existing map state before loading to prevent phantom predicate persistence and guarantee idempotency.
3. **Input Bounding:** Implement hard upper limits on string lengths, arity counts, and parenthesis nesting depth to prevent memory and stack exhaustion Denial of Service.
4. **Regex Robustness:** Refactor regular expressions to handle valid Mangle whitespace variations, particularly indented `Decl` statements.
5. **Unicode Tokenization:** Update the lexical splitting functions to iterate over runes rather than raw bytes to completely eliminate multi-byte sequence collisions.
6. **Memory Leak Prevention:** Use `strings.Clone` when extracting predicate names from massive schema files to allow the Go garbage collector to free the underlying multi-megabyte string buffers.
7. **AST Desync Prevention:** Ensure that any AST desugaring transforms applied during `AnalyzeOneUnit` do not introduce vulnerabilities where validating the raw string allows execution of a malicious transformed rule.

Addressing these boundary conditions will elevate the `SchemaValidator` from a functional parser to a hardened, enterprise-grade security perimeter for the CodeNERD evaluation engine.

This concludes the deep-dive negative testing analysis.
