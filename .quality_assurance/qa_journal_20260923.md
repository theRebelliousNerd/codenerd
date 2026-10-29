# QA Automation Journal: Boundary Value Analysis and Negative Testing
Date: 2026-09-23
Time: 00:49:40 EST
Subsystem: Usage Tracker (`internal/usage`)
Auditor: codeNERD QA Automation Engineer

## 1. Executive Summary
The `internal/usage` package tracks token usage and costs across the codeNERD framework. This is a
critical component for tracking expenditures, enforcing quota limits, and preventing unbounded
financial drain from LLM operations. The current testing strategy, particularly in
`usage_tracker_test.go`, focuses heavily on the "Happy Path" – normal operations where inputs are
well-formed, numeric values fall within expected bounds, and filesystems behave predictably.
However, considering the high-assurance nature of codeNERD, this component must be robust against a
variety of failure modes, malicious inputs, extreme environmental conditions, and sophisticated
state conflicts.

This document serves as a comprehensive Negative Testing and Boundary Value Analysis (BVA) audit of
the usage tracker. It includes not only functional edge cases, but also extensive architectural
stress points identified during analysis.

## 2. Architectural Overview
Before detailing the edge cases, it's crucial to understand the component's architecture:
*   **State Management:** The tracker relies on a central `Tracker` struct containing a `sync.Mutex`
(`mu`) to guard access to an in-memory representation of usage data (`UsageData`).
*   **Aggregation:** Data is aggregated across multiple dimensions (Provider, Model, ShardType,
ShardName, Operation, Session) utilizing Go maps.
*   **Persistence:** A background goroutine periodically flushes the aggregated state to disk
(`usage.json`) via the `atomicfile` package to ensure transactional integrity.
*   **Bounded Ring Buffer:** A fixed-size ring buffer retains the most recent raw usage events for
operational visibility.

## 3. Negative Testing Vectors

### A. Null, Undefined, and Empty Vectors
The current tests do not adequately probe how the system handles the absence of data or semantically
meaningless empty states.

1.  **Empty Strings for Taxonomic Keys:**
    *   *Scenario:* `Track` is called with empty strings for `model`, `provider`, `shardType`, or
`operation`.
    *   *Current Behavior:* The system appears to accept them, creating empty keys (`""`) in the
aggregated maps.
    *   *Impact:* Over time, this pollutes the usage data, breaks downward aggregation tools that
expect valid strings, and potentially masks real usage if default/empty values are inadvertently
passed due to upstream bugs in the perception or transducer layers.
    *   *Recommendation:* The tracker should validate these inputs and either reject them (returning
an error, which `Track` currently does not support) or bucket them into explicitly named "unknown"
categories rather than literal empty strings.

2.  **Missing Context Keys (Context Degradation):**
    *   *Scenario:* The `Track` function extracts metadata (`shardType`, `shardName`, `sessionID`)
from the `context.Context`. If the context lacks these values (e.g., initialized without
`WithShardContext`), the extraction yields empty strings.
    *   *Impact:* Similar to above, but this points to an integration failure where an
uninstrumented context is passed down the call stack.
    *   *Recommendation:* Add negative tests explicitly passing `context.Background()` to `Track`
and asserting that the failure mode is safe and logged.

3.  **Zero and Negative Token Counts:**
    *   *Scenario:* A downstream API reports 0 tokens (perhaps due to a cached response) or a
negative token count (due to an integer overflow in a faulty upstream parser before the data reaches
the tracker).
    *   *Impact:* Negative tokens will silently subtract from the total usage, artificially lowering
the recorded cost and bypassing billing guardrails.
    *   *Recommendation:* Add an explicit boundary test for `-1` and `0` input/output tokens. The
tracker must panic, log a severe error, or clamp the values to `0`.

4.  **Empty Workspace Path Initialization:**
    *   *Scenario:* `NewTracker` is initialized with an empty string (`""`).
    *   *Impact:* The tracker attempts to create a `.nerd` directory in the current working
directory, which might be read-only, leading to immediate panic or silent failure of the background
save goroutine.

### B. Type Coercion and Boundary Value Analysis
Given Go's static typing, pure type coercion (e.g., passing a string to an int parameter) is caught
at compile time. However, logical boundaries within those types are critical vectors.

1.  **Integer Overflow at the 64-bit Boundary:**
    *   *Vector:* The `TokenCounts` struct uses `int64`. The maximum value is
`9,223,372,036,854,775,807`.
    *   *Scenario:* An upstream bug or malicious payload forces the system to track `math.MaxInt64 -
10` tokens. A subsequent `Track` call adds `20` tokens.
    *   *Impact:* Go does not panic on integer overflow. The value wraps around to a massive
negative number (e.g., `-9223372036854775799`), completely destroying the integrity of the project's
accounting ledger.
    *   *Test Requirement:* Add a test that initializes the tracker with `math.MaxInt64 - 1`, adds 2
tokens, and asserts the behavior (preferably capping at MaxInt64 or returning an overflow error).

2.  **Float64 Precision Loss in Financial Accumulators:**
    *   *Vector:* `CostUSD` is tracked as a `float64`.
    *   *Scenario:* The system processes millions of micro-transactions, each costing fractions of a
cent (e.g., $0.00001).
    *   *Impact:* Due to IEEE 754 floating-point limitations, accumulating millions of tiny floats
into a large float results in precision loss (swamping). The total cost will begin to drift from the
true arithmetic sum.
    *   *Test Requirement:* Create a loop adding $0.0000001 ten million times and compare it against
the expected result multiplied by an arbitrary precision integer. For financial data, integer
arithmetic (representing tenths of a micro-cent) is architecturally superior to float accumulation.

3.  **Unbounded String Lengths:**
    *   *Vector:* The taxonomic keys (SessionID, Model, Provider).
    *   *Scenario:* A sophisticated attack involves passing a 1GB string as the `sessionID`.
    *   *Impact:* The tracker stores this string as a key in the `BySession` map. This causes an
immediate OOM (Out of Memory) panic, crashing the agent.
    *   *Test Requirement:* Pass extremely long strings (e.g., `strings.Repeat("a", 10*1024*1024)`)
to `Track` and ensure it gets truncated to a safe length (e.g., 256 characters) before being used as
a map key.

### C. State Conflicts and Concurrency Vectors
The tracker operates in a highly concurrent environment where dozens of subagents might be spinning
up and tearing down simultaneously.

1.  **Massive Concurrency and Mutex Contention (The "Thundering Herd"):**
    *   *Scenario:* codeNERD executes a massive parallel map-reduce over a 50-million line monorepo.
1,000 subagents finish their LLM calls at the exact same millisecond and attempt to call
`usage.Track()`.
    *   *Impact:* All 1,000 goroutines block on the single `sync.Mutex` (`mu`) in the tracker. The
resulting lock contention causes a massive latency spike, starving the Go scheduler and potentially
causing timeouts in the core orchestrator.
    *   *Test Requirement:* A stress test spawning 10,000 goroutines calling `Track` concurrently.
    *   *Architectural Implication:* Is `sync.Mutex` performant enough for this? At high scale, the
tracker must move to a lock-free architecture using `sync/atomic` for the global counters, or shard
the mutexes (e.g., a slice of 64 mutexes hashed by SessionID) to distribute the contention.

2.  **Save Race Conditions (Disk I/O Latency):**
    *   *Scenario:* The background autosave triggers. It locks the mutex, serializes the JSON, and
calls `atomicfile.Write`. The underlying disk is an overloaded networked drive, and the write takes
5 seconds to complete.
    *   *Impact:* Because the write happens (presumably) while holding the lock, or shortly after,
any subagent trying to track usage during those 5 seconds is frozen.
    *   *Test Requirement:* Mock the filesystem to introduce a 2-second sleep during `Save()`, and
assert that concurrent `Track` calls do not block for 2 seconds. The serialization must happen under
the lock, but the I/O must happen outside the lock.

3.  **Session Map Pruning Boundary (`maxSessions = 500`):**
    *   *Vector:* The `BySession` map is capped to prevent unbound JSON growth.
    *   *Scenario:* Exactly 500 unique sessions are created. Then the 501st session is created.
    *   *Impact:* The pruning logic must execute perfectly. It must identify the lowest-spend
sessions, aggregate them into the `(pruned)` bucket, and delete the original keys. If this fails,
the map grows forever.
    *   *Test Requirement:* A boundary test explicitly generating 501 unique sessions and asserting
that `len(tracker.data.Aggregate.BySession)` remains exactly at the limit, and the `(pruned)` bucket
exists with the correct aggregate totals.

4.  **Ring Buffer Wraparound (`maxEvents = 1000`):**
    *   *Vector:* The transient event log.
    *   *Scenario:* The 1,000th event is recorded, filling the slice. The 1,001st event arrives.
    *   *Impact:* Off-by-one errors in modulo arithmetic or slice appending can cause Panics (`index
out of range`) or dropped events.
    *   *Test Requirement:* Loop `Track` 1,005 times. Assert that the length of the slice is exactly
1,000, and that the 5 oldest events were successfully overwritten by the 5 newest events,
maintaining strict temporal ordering.

### D. User Request Extremes (The "Frontier Benchmark" Vectors)
How does the tracker handle truly bizarre inputs generated by extreme prompts?

1.  **Hallucinated Model Identifiers:**
    *   *Scenario:* A user asks the agent to use a non-existent, hallucinated coding language, and
the internal logic glitches, passing that hallucinated language string as the `model` parameter to
the usage tracker.
    *   *Impact:* The tracker faithfully records "Blargh++" in the `ByModel` map. If this happens
thousands of times uniquely, the map bloats.
    *   *Recommendation:* The tracker should cross-reference models against a known registry of
valid models (or a dynamic list populated by the `ConfigFactory`). Unrecognized models should
trigger a warning and be bucketed into an "unknown-model" key to prevent map bloat.

2.  **Time-Travel Events (Clock Skew):**
    *   *Scenario:* An event is generated by a remote subagent or a distributed shard where the
system clock is drastically skewed (e.g., 2 years in the past, or 10 years in the future).
    *   *Impact:* When added to the `Events` ring buffer, it breaks chronological assumptions.
Downstream tooling reading `usage.json` might crash if they assume strict monotonicity in
timestamps.
    *   *Test Requirement:* Inject an event with `time.Now().Add(-8760 * time.Hour)` and ensure the
tracker handles it gracefully (either rejecting it, clamping it, or at least not crashing).

## 4. Deep Architectural Considerations and Secondary Vectors

### Subsystem Interoperability and Memory Constraints
When codeNERD runs on constrained hardware (e.g., a laptop with 8GB RAM), the `UsageData` struct
footprint matters. While a single JSON blob seems small, during a massive brownfield refactoring
campaign involving 50 million lines of code, the system might spawn thousands of ephemeral
subagents.

*   **Garbage Collection Pressure:** Every call to `Track` allocates new strings for map keys if
they aren't interned. The `context.Context` extraction also involves interface conversions. In a
tight loop, this generates significant garbage, forcing the Go GC to run more frequently and
stealing CPU cycles from the main logic evaluation engine.
*   **JSON Serialization Overhead:** As the `AggregatedStats` maps grow (even within their pruned
limits), the `json.Marshal` call in the background save routine takes longer and allocates more
memory. This is a CPU-bound operation happening in the background.

### Mangle Kernel Interactions
The codeNERD architecture uses a Mangle kernel for executive control. The usage tracker sits
alongside this, but how do they interact?

*   **Logic Rule Re-evaluation:** If a Mangle logic rule fails and the engine re-evaluates a state,
does it double-charge the token usage for the LLM synthesis that occurred during the failed attempt?
The tracker must ensure that idempotent operations or retries are tracked accurately according to
the business logic, not just mechanically.
*   **JIT SubAgent Telemetry:** As JIT Clean Loop architecture spawns subagents dynamically based on
runtime prompt compilation, the `ShardName` and `ShardType` become highly dynamic. The tracker must
handle high cardinality in these dimensions without blowing up the memory budget.

### Resilience Against Malformed Persistence State
What happens if the `usage.json` file on disk becomes corrupted during a hard power loss, despite
the use of `atomicfile`?
*   **Initialization Recovery:** The `NewTracker` must be resilient. If `json.Unmarshal` fails due
to a truncated file, it should ideally backup the corrupted file and start fresh, logging a critical
error, rather than crashing the entire codeNERD startup sequence. The current tests evaluate
`ReadError` and `JSONUnmarshalError`, but do they verify the recovery path?

### Cross-Process Lock File Contention
The tracker relies on `.nerd/usage.json`. If multiple instances of codeNERD are executed against the
same workspace simultaneously (e.g., a user runs two CLI commands in different terminals), there is
a race condition on the file.
*   **File Locking Limitations:** Even with `atomicfile` ensuring write atomicity, the Read-Modify-
Write cycle is not atomic across processes. Instance A reads the file, Instance B reads the file,
Instance A writes, Instance B writes (overwriting Instance A's usage).
*   **Test Requirement:** A multi-process test simulating this exact scenario to ensure the usage
tracker correctly utilizes file-level advisory locks (e.g., `flock`) or downgrades to a safe
degraded mode.

### Advanced Negative Test: The "Poison Pill" Event
Consider an event specifically crafted to exploit the JSON parser upon reload.
*   **Scenario:** An operation name contains null bytes (`\x00`), unescaped control characters, or
deeply nested structures (if the schema evolved to allow it).
*   **Impact:** When the tracker is restarted and attempts to load `usage.json`, the Go JSON parser
might fail, throwing a syntax error.
*   **Test Requirement:** Inject control characters into the `operation` string, ensure it is
written to disk successfully, and then ensure a subsequent `NewTracker` instantiation can parse it
without failing.

### Advanced Negative Test: Disk Quota Exhaustion
*   **Scenario:** The user's hard drive is 100% full.
*   **Impact:** The `atomicfile.Write` will fail with `ENOSPC`. The background save goroutine will
likely log the error, but the in-memory state will continue to accumulate.
*   **Test Requirement:** Mock the disk interface to return `ENOSPC` on write. Verify that the
tracker does not panic, continues to aggregate in memory, and successfully flushes to disk once
space is available (or upon shutdown).

### The Concurrency/Consistency Trade-off
The `sync.Mutex` approach prioritizes strong consistency over raw throughput. Every event is
strictly serialized. For financial data, this is often the correct choice. However, if performance
testing reveals that the mutex is a bottleneck during parallel code generation (e.g., generating 100
files concurrently), we must reconsider.
*   **Alternative Architecture:** A lock-free ring buffer per goroutine/subagent that flushes to the
central aggregator asynchronously. This would require significantly more complex tests to ensure no
data is lost during the flush process or during a sudden shutdown.

## 5. Specific Test Additions Proposed
Based on the analysis, the following specific `// TODO:` comments have been added to
`internal/usage/usage_tracker_test.go` in the `TestTracker_TrackAggregatesAndPersists` function to
mandate test coverage for these critical vectors:

1.  `// TODO: Test \`Track\` with empty string inputs for metadata.`
2.  `// TODO: Test \`Track\` with negative token counts.`
3.  `// TODO: Test \`Track\` with missing context keys.`
4.  `// TODO: Test integer overflow on token accumulation.`
5.  `// TODO: Test extreme concurrency (e.g., 1000 goroutines calling \`Track\`).`
6.  `// TODO: Test the \`maxSessions\` pruning logic exactly at the boundary (500, 501 sessions).`
7.  `// TODO: Test the \`maxEvents\` ring buffer wraparound.`

Implementing these tests is crucial for achieving the high assurance required by the codeNERD
architecture.

## 6. Detailed Action Plan for Implementation
The implementation of the above tests should follow a phased rollout to prevent destabilizing the
core agent architecture.

### Phase 1: Foundational Boundary Validations
*   **Task 1.1:** Implement the negative bounds tests for token counts (`< 0`) and verify the
clamping logic or error handling correctly fires without panic.
*   **Task 1.2:** Introduce the null byte (`\x00`) poisoning test to validate JSON serialization
resilience. Ensure the Go JSON encoder handles or strips invalid UTF-8 without entering a crash
loop.
*   **Task 1.3:** Test the integer overflow scenario. Create a mocked accumulator initialized at
`math.MaxInt64 - 100` and force multiple rapid additions to confirm the wrapping behavior and
implement a safety cap.

### Phase 2: Concurrency and Load Hardening
*   **Task 2.1:** Develop the "Thundering Herd" benchmark. This requires a new file
(`usage_tracker_bench_test.go`) utilizing `testing.B` with `RunParallel` to hammer the `Track`
method from thousands of goroutines.
*   **Task 2.2:** Analyze the profiling data from the benchmark. If lock contention exceeds
acceptable thresholds (e.g., p99 latency > 100ms), architect a proposed fix (such as sharded
mutexes) and document the design in the repository's architecture logs.
*   **Task 2.3:** Implement the race condition test simulating slow disk I/O. Use a mock
implementation of the filesystem interface (if one exists in the project) to introduce an artificial
2-second delay during `atomicfile.Write`, then verify that concurrent readers/writers in `Track` are
not blocked during this I/O phase.

### Phase 3: Extreme Bounds and Edge Cases
*   **Task 3.1:** Write the time-travel test injecting timestamps from years in the past and future.
Ensure the ring buffer maintains stability and doesn't sort or drop items unexpectedly.
*   **Task 3.2:** Write the boundary test for `maxSessions` exactly at `500` and `501`. This
requires programmatic generation of unique UUIDs for session strings to force the pruning logic to
activate.
*   **Task 3.3:** Validate the OOM resilience by attempting to inject massive strings (100MB+) as
keys. Ensure the system either rejects the payload or truncates it before map insertion.

## 7. Deep Dive: Agentic State Traces and Fault Injection
Beyond standard unit tests, the codeNERD environment demands rigorous integration testing of the
usage tracker within the Mangle fixpoint loop.
The Mangle kernel evaluates rules to a fixpoint, determining the agent's next action. If a rule
derivation relies on current token budget constraints (e.g., `budget_available(T) :- max_tokens(M),
usage(U), U < M`), any latency or inaccuracy in the usage tracker directly impacts the agent's
autonomy.

### Trace Injection Scenario A: The 'Phantom Budget'
*   **Setup:** The agent is nearing its token quota limit (e.g., 999,000 out of 1,000,000 tokens).
*   **Action:** The agent initiates a highly parallelized task (e.g., recursive directory
summarization).
*   **Fault Injection:** We intercept the `Track` function and delay the update of the global atomic
counter by 500 milliseconds (simulating high mutex contention or a slow background sync).
*   **Expected Result (Failure Mode):** The Mangle kernel, evaluating the budget continuously, reads
the stale (delayed) usage data. It incorrectly concludes the budget is still available and
authorizes 10 more parallel LLM calls. The true usage spikes to 1,050,000, violating the hard quota
constraint.
*   **Remediation & Testing Strategy:** This highlights a fundamental flaw in asynchronous budget
tracking. The `Track` operation must either strictly block budget evaluations until synchronized, or
the kernel must operate on a pessimistic projection of in-flight token requests, reconciling with
the tracker post-execution. The test suite must mock the Mangle evaluation cycle and assert that
budget violations cannot occur even under maximum simulated tracker latency.

### Trace Injection Scenario B: The 'Atomic Desync'
*   **Setup:** The `atomicfile` package is responsible for durably writing the usage JSON.
*   **Action:** The background goroutine initiates a save. It writes to a temporary file, e.g.,
`.nerd/usage.json.tmp`.
*   **Fault Injection:** We simulate a power failure or process termination (SIGKILL) *exactly*
after the temporary file is completely written, but *before* the atomic `rename` operation swaps it
into place as `.nerd/usage.json`.
*   **Expected Result (Recovery Mode):** Upon reboot, codeNERD initialization should detect the
orphaned `.tmp` file and the older `.json` file.
*   **Testing Strategy:** The `NewTracker` initialization sequence must be tested with various
combinations of valid, invalid, and orphaned lock/tmp files. It must deterministically recover the
most recent valid state. If the `.tmp` file is fully valid JSON (can be unmarshaled), it represents
the true most recent state and should be recovered. If it is truncated, it must be discarded in
favor of the older `.json` file. This logic requires explicit testing via file system mocking in
`usage_tracker_test.go`.

### Trace Injection Scenario C: Mangle Atom Type Dissonance
*   **Setup:** The usage tracker interacts with the Mangle kernel by providing facts (e.g.,
`usage_stats(/session_1, /chat, 1500)`).
*   **Action:** The usage tracker's Go code serializes the usage data into Mangle atoms.
*   **Fault Injection:** A new operation type is introduced (e.g., "tool_generation") but is
inadvertently passed as a raw string instead of a registered Mangle atom type.
*   **Expected Result (Failure Mode):** The Mangle engine expects `Atom` types for relational joins.
If the usage data is inserted as a `String` type, a join like `billing_rate(Op, Cost) :-
usage_stats(_, Op, _), rate_table(Op, Cost)` will yield zero results because `/chat` (Atom) does not
match `"chat"` (String). The billing calculation silently fails.
*   **Testing Strategy:** Following the established Mangle testing patterns documented in the
codebase, the usage tracker's test suite must utilize the `analysis.Analyze(program)` logic to parse
rules interacting with the usage data, and rigorously verify that the `usage_stats` facts generated
by the Go code are strictly typed as `ast.Name("...")` (Atoms) and not `ast.String("...")`. This
requires an integration test bridging the `internal/usage` and `internal/mangle` packages,
confirming the safety and stratification of the rules governing token budgets.

## 8. Formal Proof of Properties and Test Coverage Maps
In high-assurance logic systems, empirical testing is often insufficient. We must complement
boundary analysis with formal or semi-formal methods to prove safety invariants. For the
`internal/usage` system, these properties are critical:

1.  **Monotonicity of Usage Counters:** The total project tokens must monotonically increase or
remain constant over time, assuming no manual intervention via administration tools. The test suite
must formally verify that `new_usage >= old_usage` under all concurrency models. Any reduction in
token counts represents a critical breach of accounting integrity.
2.  **Referential Integrity of Taxonomic Buckets:** The sum of all values in the `BySession` map
(including the `(pruned)` bucket) must exactly equal the `TotalProject` count, provided no events
are silently dropped or double-counted during the pruning phase. The tests must enforce this
accounting ledger constraint after every simulated batch of events.
3.  **Strict Serializability of the File Save Operation:** The background save must capture a
consistent, point-in-time snapshot of the tracker's memory. It must not capture partial updates
(e.g., capturing the increment of the Provider map but missing the increment of the TotalProject map
because a concurrent thread was midway through its update). The tests must artificially interleave
`Track` and `Save` goroutines, asserting that the resulting JSON always maintains referential
integrity.

## 9. Next Steps and Prioritization
The proposed tests range in complexity from trivial boundary checks to complex concurrency
simulations. To integrate these efficiently into the codeNERD continuous integration pipeline, I
propose the following priority queue:

*   **Priority 1 (Critical Path - P0):** Implement the `maxSessions` pruning boundary test and the
zero/negative token validation. These directly protect the application from memory leaks and silent
billing circumvention, respectively.
*   **Priority 2 (High Risk - P1):** Implement the "Thundering Herd" concurrency stress test. This
is vital to validate the architecture's suitability for extreme parallelization scenarios, which is
a core value proposition of codeNERD's multi-agent design.
*   **Priority 3 (Medium Risk - P2):** Implement the `maxEvents` ring buffer wraparound and the
string limits for taxonomic keys. These prevent localized panics but are less likely to occur than
typical concurrency issues.
*   **Priority 4 (Low Risk / Formal - P3):** Develop the formal property verifications and the
Mangle Atom Type Dissonance integration tests. These provide the highest level of assurance but
require the most engineering effort to build and maintain.

## 13. Deep Integration with Token Budget Manager
A critical aspect of negative testing is evaluating how this subsystem interacts with its closest neighbor: the Token Budget Manager. The Usage Tracker records the past; the Budget Manager enforces the future. If the tracker fails, the budget manager is blind.

### 13.1 Stale Read Vulnerabilities
*   **Vector:** The Budget Manager reads `usage.json` to determine remaining quota, rather than querying the `Tracker` instance in memory.
*   **Test Scenario:** If the architecture permits external or out-of-band reads of the JSON file by other components, a race condition exists. We must test the scenario where 50,000 tokens are consumed rapidly, but the background save hasn't flushed yet.
*   **Expected Result:** The test must prove that either the Budget Manager queries the in-memory state directly (via `tracker.Stats()`) or that the system enforces a synchronous flush before any critical budget evaluation. Relying on an asynchronous disk write for real-time quota enforcement is a catastrophic architectural flaw that BVA must uncover and prevent.

### 13.2 The "Infinite Loop" Ouroboros Cost Drain
*   **Vector:** The Autopoiesis system (Ouroboros Loop) generates a new subagent or tool, but due to a logic flaw, it enters an infinite generation loop.
*   **Test Scenario:** The usage tracker is hammered with thousands of high-cost generation events per minute.
*   **Expected Result:** While the tracker's job is simply to record, the BVA must ensure the tracker's API allows for real-time threshold alerts. We must test a "Circuit Breaker" pattern: if the tracker observes an anomalous spike in `ByOperation["tool_gen"]`, it must be able to trigger a synchronous panic or an interrupt signal to the Mangle kernel, halting the run before the API bill reaches thousands of dollars. The test should simulate a loop and verify the circuit breaker fires accurately.
