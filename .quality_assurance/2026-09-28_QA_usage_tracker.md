# QA Journal Entry: Boundary Value & Negative Testing Analysis
**Date:** 2026-09-28
**Time:** 00:18:41 EST
**Subsystem Analyzed:** `internal/usage` (Usage Tracking System)
**Reviewer:** QA Automation Engineer (Jules)

## 1. Executive Summary

This journal entry documents a deep-dive Quality Assurance review of the `internal/usage` subsystem in the codeNERD
framework, with a specific focus on Boundary Value Analysis (BVA) and Negative Testing.

The `internal/usage` package is responsible for accurately tracking, aggregating, and persisting token usage and cost
metrics across various models, providers, and shard types. Given that this subsystem directly handles financial
approximations (via token-to-cost conversions) and system-wide telemetry, its robustness against edge cases is critical.
A failure here could lead to silently dropped metrics, panics, or memory leaks that could destabilize long-running agent
campaigns.

The current test suite (`internal/usage/usage_tracker_test.go` and peers) provides reasonable coverage for the "Happy
Path"—verifying that standard token inputs are correctly aggregated and saved. However, it lacks comprehensive negative
testing and stress testing across multiple failure vectors.

This analysis evaluates the subsystem against four primary vectors:
1. Null/Undefined/Empty Inputs
2. Type Coercion / Precision Loss
3. User Request Extremes / Stress Testing
4. State Conflicts & Concurrency

## 2. Analysis by Vector

### 2.1 Null / Undefined / Empty Strings and Arrays

The `Track` method takes multiple strings as context (`model`, `provider`, `operation`). The system uses these strings
as keys in Go maps (e.g., `ByProvider`, `ByModel`).

**Current State & Risks:**
- **Empty Strings:** If `provider` or `model` is passed as an empty string `""`, Go will happily insert it into the map. Over time, an aggregation under `""` might obscure real data.
- **Missing Context Keys:** The function `shardMetaFromContext` extracts values like `shard_type` and `session_id`. If the context is missing these keys (e.g., a background goroutine calls `TrackFromContext` with `context.Background()`), it likely falls back to empty strings. The system currently handles this without panicking, but it might pollute the `""` bucket.
- **Nil Contexts:** While `context.WithValue` on a nil context panics, `TrackFromContext` safely handles `ctx == nil` by returning early. However, passing a nil context to `shardMetaFromContext` directly (if exposed) could be a vector.
- **Empty Arrays/Slices:** The events ring buffer (`t.data.Events`) handles empty slices well during JSON marshalling, but we must ensure that operations on a 0-length ring buffer don't cause index-out-of-bounds panics if `WithEventLog` is not used.

**Recommendations:**
- Add tests that explicitly inject `""` for all string parameters to ensure the aggregation maps handle them predictably (or reject them, depending on the business logic).
- Test with contexts that lack the specific `shardMetaKey` values.

### 2.2 Type Coercion and Precision Loss

Go is strictly typed, so we don't have JavaScript-style string-to-number coercions. However, we do have precision issues
when moving between `int64` token counts and `float64` cost estimations.

**Current State & Risks:**
- **Float64 Precision Degradation:** Cost is accumulated in a `float64`. When aggregating millions of tiny fractional token costs (e.g., $0.0000001 per token) over a long-running campaign, `float64` can suffer from precision loss. Adding a tiny float to a very large float causes the tiny addition to be lost entirely due to the mantissa limits of IEEE 754.
- **Integer Overflow:** Token counts are accumulated as `int64`. While 9 quintillion tokens is practically unreachable today, long-running systems aggregating across many nodes might theoretically hit boundaries. More realistically, intermediate 32-bit `int` inputs (from the signature `input, output int`) could overflow on 32-bit architectures if a massive chunk of text is processed (e.g., a massive log file).
- **Negative Tokens:** The code explicitly checks `if input < 0 || output < 0` and logs a warning, returning early. This is a good defense against negative values reducing total counts.

**Recommendations:**
- Add tests to simulate millions of microscopic cost additions to verify if the final `float64` total drifts significantly from the true mathematical sum. If the drift is unacceptable, consider using a `big.Rat` or scaled integer approach (e.g., micro-cents) for internal accumulation.
- Ensure tests verify the `int` to `int64` conversion boundaries.

### 2.3 User Request Extremes & Stress Testing

The usage tracker is a shared resource (`mu sync.Mutex`) that receives concurrent hits from multiple shards.

**Current State & Risks:**
- **Extreme String Lengths:** What if a rogue prompt or malformed context injects a 100MB string as the `provider` name? The system will use it as a map key. In Go, map keys are hashed, which is relatively fast, but storing millions of massive string keys will cause severe memory bloat (OOM).
- **Extreme Ring Buffer Load:** The `maxEvents` is hardcoded to 1000. If an agent generates 10,000 tool calls in a few seconds, the ring buffer wraps rapidly. The test must ensure the slice slicing `append(t.data.Events[1:], event)` doesn't degrade linearly to O(N) where N is large, though N=1000 is small enough to be fine.
- **High Turn Volumes:** The `recordTurnLocked` function uses a `map` and a separate `turnOrder` slice to track up to `maxTurns` (1024). When the limit is reached, it deletes the oldest turn and shifts the slice: `t.turnOrder = t.turnOrder[1:]`. This slice shift is O(N). Doing this thousands of times per second could cause slight CPU spikes under extreme load.
- **Session Pruning Boundary:** `pruneSessionsLocked` kicks in at `maxSessions` (500). It sorts the map by cost. Sorting a map of size 500 is fast, but doing it on *every* `Track` call once the boundary is crossed is an O(N log N) operation held *under a global mutex*. This is a massive bottleneck.

**Recommendations:**
- Write stress tests that inject strings of extreme lengths.
- Write tests that cross the `maxSessions` threshold and measure the latency of `Track`. If latency spikes due to repeated sorting, suggest a debounced pruning mechanism or a min-heap structure.
- Write tests pushing 10,000+ unique turn IDs to observe the behavior of the slice shift.

### 2.4 State Conflicts and Race Conditions

The tracker acts as a funnel for highly concurrent subsystems.

**Current State & Risks:**
- **Mutex Contention:** `Track` locks `t.mu` for the entirety of its execution. Under a heavy 100-agent thunderdome scenario, thousands of goroutines will contend for this single mutex. Since `pruneSessionsLocked` runs under this same lock and does O(N log N) work when triggered, it could cause severe starvation and latency spikes across the entire system.
- **File System Races:** `Save()` is debounced using an `AfterFunc`. If the process is terminated via SIGKILL, in-flight data is lost. If a file lock fails (e.g., in Windows), the system might race to write `usage.json`. The `atomicfile` package mitigates partial writes, but tests need to ensure two concurrent tracker instances (which is considered a bug, but could happen via API misuse) don't silently overwrite each other.

**Recommendations:**
- Implement a test that spins up 10,000 goroutines concurrently calling `Track` to measure mutex contention and look for potential deadlocks or starvation.
- Verify the debounce timer logic properly flushes if `Close()` is called concurrently with incoming tracks.

## 3. Performance Capability Assessment

Can the `internal/usage` system handle these vectors performantly?

- **Handling Null/Empty:** Yes. Go maps handle `""` in O(1) time. It is highly performant, though semantically undesirable.
- **Handling Coercion/Precision:** Yes, `float64` addition is O(1) and CPU-bound. However, precision accuracy will degrade. It is performant but potentially mathematically inaccurate over extreme time horizons.
- **Handling Extremes (The Pruning Bottleneck):** **No.** The current implementation of `pruneSessionsLocked` is a performance risk. When sessions exceed 500, every single subsequent call to `Track` triggers an O(N log N) sort of the map under a global lock. If 1,000 agents are active, the system will spend significant CPU time just re-sorting the same pruned map. This must be refactored to a background routine or a more efficient data structure (like a priority queue or heap).
- **Handling Concurrency:** **Moderate.** The global `sync.Mutex` is a classic bottleneck. A sharded mutex array (e.g., `sync.Map` or an array of locks hashed by session ID) would drastically improve throughput under extreme load.

## 4. Conclusion & Action Items

The `internal/usage` package is structurally sound for standard codenerd operation but exhibits architectural stress
points under high-concurrency and long-tail boundary conditions.

The immediate action item is to expand `usage_tracker_test.go` to explicitly document these gaps using `// TODO:`
markers within the primary testing function, ensuring future developers address the precision loss and O(N log N)
pruning bottleneck under the global mutex lock.

By systematically applying Boundary Value Analysis and Negative Testing methodologies, we can preemptively identify
these performance regressions before they manifest in production as mysterious latency spikes during massive campaign
executions.


## 5. Detailed Edge Case Scenarios

To further flesh out the negative testing surface area, let's explore specific hypothetical scenarios that the system
might encounter during rigorous operations.

### Scenario A: The Infinite Prompt Attack
A malicious or malfunctioning external system pipes an infinitely long string into the `provider` field.
*   **Trigger:** `Track(ctx, "model_x", <1GB string>, 10, 10, "chat")`
*   **Expected Outcome:** The system should ideally truncate strings that exceed a reasonable length (e.g., 255 characters) before using them as map keys to prevent OOM errors.
*   **Actual Outcome (Current):** Go will attempt to hash the 1GB string and store it. Memory usage will spike linearly with the size of the string.
*   **Remediation:** Introduce a sanitization layer at the entry of `Track` that enforces length boundaries on all string inputs.

### Scenario B: The Micro-Transaction Drain
A fast-looping agent process generates 100,000 sub-agent queries per second, each using a microscopic token count that
rounds down to $0.00000001.
*   **Trigger:** Rapid, continuous calls to `Track` with costs near the float64 epsilon.
*   **Expected Outcome:** Total cost should accurately reflect the mathematical sum of all transactions.
*   **Actual Outcome (Current):** Due to IEEE 754 limitations, once the accumulator `TotalProject.Cost` becomes large (e.g., $1,000.00), adding $0.00000001 might result in zero change, effectively giving away compute for free.
*   **Remediation:** Implement Kahan summation or switch to fixed-point integer arithmetic (e.g., tracking micro-cents) for internal accumulation.

### Scenario C: Context Cancellation Mid-Track
An agent's context is abruptly cancelled via `context.WithTimeout` exactly while `TrackFromContext` is executing.
*   **Trigger:** `ctx.Done()` fires while waiting for `t.mu.Lock()`.
*   **Expected Outcome:** The system should gracefully handle the cancellation and either abandon the track or ensure it completes safely.
*   **Actual Outcome (Current):** `Track` does not check `ctx.Done()` while waiting for the mutex. It blocks uninterruptibly. This could delay context teardown.
*   **Remediation:** Consider using `select` with a channel-based concurrency model, or at least check `ctx.Err()` after acquiring the lock to decide whether to proceed.

### Scenario D: Time Warp (Clock Skew)
The host operating system experiences a significant NTP clock skew backwards while the `autoSaveTimer` is active.
*   **Trigger:** `time.Now()` returns a value in the past.
*   **Expected Outcome:** The `autoSaveDelay` (5 seconds) should still fire relatively reliably based on monotonic clocks.
*   **Actual Outcome (Current):** Go's `time.Timer` uses monotonic clocks under the hood, so it is mostly immune to wall-clock jumps. However, the `Timestamp` in `UsageEvent` relies on `time.Now().UTC()`, meaning the events ring buffer could end up with events appearing out of order chronologically.
*   **Remediation:** Ensure any downstream consumers of `UsageEvent` do not implicitly rely on strict chronological ordering without sorting, or use a purely monotonic sequence ID.

### Scenario E: Unpriced Token Explosion
An experimental model is added without a price table entry. It is used heavily, generating billions of tokens.
*   **Trigger:** `priced` returns `false` from `EstimateCost`.
*   **Expected Outcome:** `UnpricedTokens` should accumulate without wrapping or overflowing.
*   **Actual Outcome (Current):** `UnpricedTokens` is an `int64`. It can hold up to 9 quintillion tokens. This is safe from overflow under practical conditions.
*   **Remediation:** The current implementation is sufficient, but tests should explicitly verify the transition boundary where a model is added to the price table mid-flight.

### Scenario F: The Pruned Bucket Collision
The system reaches `maxSessions` (500). Subsequent sessions are pruned into the `(pruned)` bucket. A legitimate session
accidentally generates an ID of `(pruned)`.
*   **Trigger:** `Track(ctx, "model", "provider", 10, 10, "chat")` where `sessionID == "(pruned)"`.
*   **Expected Outcome:** The system should differentiate between the internal bucket and a user-supplied ID, or reject the reserved name.
*   **Actual Outcome (Current):** The user's tokens will be seamlessly merged into the pruning bucket, obscuring their actual origin.
*   **Remediation:** Enforce a strict prefix format for generated session IDs (e.g., `sess_...`) and sanitize incoming IDs that attempt to use reserved internal keywords.

### Scenario G: Concurrent Close and Track
The application begins shutdown, calling `Tracker.Close()`, while lingering goroutines are still flushing final `Track`
calls.
*   **Trigger:** `Close()` executes concurrently with `Track()`.
*   **Expected Outcome:** The system flushes pending writes and rejects new tracks gracefully.
*   **Actual Outcome (Current):** `Track` checks `if t.closed { return }` inside the mutex. This is thread-safe. However, if `Close()` flushes the data and sets `closed`, any tracks waiting on the mutex will simply return, losing their tokens.
*   **Remediation:** During graceful shutdown, upstream systems must ensure all tracking operations have completed before invoking `Close()`.

### Scenario H: Massive Event Ring Allocation
`WithEventLog` is enabled, and a user attempts to track 1,000,000 events instantly.
*   **Trigger:** High burst rate with event logging enabled.
*   **Expected Outcome:** The ring buffer manages memory smoothly.
*   **Actual Outcome (Current):** `t.data.Events = append(t.data.Events, event)` is used. When capacity is exceeded, `append(t.data.Events[1:], event)` creates a new slice header. Under massive load, this continuous reallocation and garbage collection of the underlying array can cause GC pressure.
*   **Remediation:** Implement a true circular buffer (ring buffer) using a fixed array and head/tail pointers to eliminate GC pressure entirely.

## 6. Testing Strategy Framework

To implement the identified edge cases, the QA framework must adopt a multi-tiered approach:

1.  **Fuzz Testing:** Introduce Go fuzz targets (`func FuzzTrack(...)`) to blast random strings and integers into the
tracking system to uncover hidden panics.
2.  **Property-Based Testing:** Assert that regardless of the order of operations, the total cost equals the sum of its
parts (commutativity and associativity).
3.  **Concurrency Benchmarks:** Write `BenchmarkTrackConcurrent` tests using `b.RunParallel` to empirically measure the
lock contention and O(N log N) sorting degradation.
4.  **Chaos Engineering:** Introduce random disk write failures (mocking `atomicfile.Write`) to ensure the debounce
timer recovers gracefully and memory state remains coherent.

## 7. Final Assessment

The boundary value analysis reveals that while the core logic is mathematically sound for standard operational bounds,
the system is exposed to performance degradation under extreme concurrency and massive campaign scenarios. By
strategically layering the identified negative tests into the test suite, we can harden the framework to support the
next generation of codeNERD agent workloads.


## 8. Architectural Reflections on Telemetry Subsystems

The design of a robust telemetry subsystem requires balancing throughput with accuracy. In high-concurrency environments like codeNERD, the observer effect must be minimized—tracking usage should not slow down the actual usage.

### 8.1 The Lock-Free Ideal
Consider migrating the core counters to atomic variables (e.g., `sync/atomic.Int64`). While this handles total counts beautifully, it struggles with the multi-dimensional mapping required by `ByProvider` and `ByModel`. A lock-free skip list or concurrent hash map could bridge this gap, though Go's lack of a natively typed concurrent map (prior to generics, and even now `sync.Map` has overhead) makes this complex.

### 8.2 The Sharding Approach
To alleviate mutex contention, the `tracker` struct could internally shard its state. Instead of one global mutex, it could have an array of 256 mutexes and maps, hashing the `sessionID` to determine which shard to lock. This would immediately reduce contention by two orders of magnitude while preserving the simplicity of the map data structures.

### 8.3 The Event Stream Model
Currently, `usage.json` is a snapshot of state. This is vulnerable to corruption if the atomic swap fails in catastrophic OS failures. An append-only Write-Ahead Log (WAL) or Event Sourcing model would provide superior durability. Each `Track` simply appends an event to a fast log. Background workers periodically compact the log into the aggregated state. This isolates the hot path from complex calculations like `pruneSessionsLocked`.

### 8.4 Observability vs. Actionability
Why do we track this data? If it is purely for operator visibility, eventual consistency is acceptable. If it is used for hard rate-limiting (e.g., stopping a campaign when cost > $50), strict consistency is required. Clarifying this boundary will guide how aggressively we optimize the locking strategy.

### 8.5 Future-Proofing for 64-bit Costs
As models become cheaper and context windows become massive, the risk of `float64` precision loss increases. We may need to adopt a standard financial practice: storing all monetary values as integers representing the smallest logical unit (e.g., micro-cents). This eliminates float drift entirely and makes comparisons deterministic.

### 8.6 The Cost of Pruning
The `pruneSessionsLocked` function represents a classic memory vs. CPU tradeoff. We bound memory by setting `maxSessions=500`, but pay a severe CPU tax to maintain that bound under pressure. Using a Min-Heap based on cost would allow us to identify and prune the cheapest session in O(log N) time instead of O(N log N) time, drastically reducing the time spent holding the global lock.

### 8.7 Integration with the JIT Clean Loop
With the December 2024 architecture update moving to the JIT Clean Loop, the concept of a "session" is more ephemeral. The tracking system must gracefully handle highly volatile session IDs generated dynamically by the Ouroboros loop. The pruning logic will be exercised far more frequently in this new paradigm.

### 8.8 Conclusion on Architecture
The `internal/usage` package stands as a critical pillar of the codeNERD framework. By continuously subjecting it to rigorous boundary value analysis and negative testing, we ensure it remains an invisible, highly performant telemetry engine capable of supporting the most demanding neuro-symbolic workloads.


## 9. Expanded Vector Analysis: The "Thunderdome" SubAgent Edge Cases

As codeNERD evolves towards highly autonomous multi-agent swarms (the "Thunderdome" testing environment), the usage tracking system will face unprecedented stress. We must consider edge cases where the core `internal/usage` system interacts with chaotic, self-replicating subagent architectures.

### 9.1 The Ouroboros Loop Consumption Rate
In the Ouroboros Loop pattern, an agent repeatedly spawns subagents to evaluate its own output, which then spawn further subagents. If this recursive loop lacks a strict depth limit or if the stop condition (the fixpoint in Mangle logic) is never reached, the token consumption rate becomes exponential.

*   **Trigger:** A recursive subagent chain generating maximum context window calls at a rate of 10Hz per subagent.
*   **System Impact:** The `Track` method will be hit with massive, simultaneous spikes. The `BySession` map will expand with hundreds of unique ephemeral session IDs.
*   **Test Case Design:** We must write a synthetic stress test that mimics a 5-deep Ouroboros loop. It should verify that `Track` correctly captures the total cost, but more importantly, we must verify that the `pruneSessionsLocked` bottleneck does not cause a cascading failure where the locking delays the Ouroboros loop itself, resulting in context deadline exceeded errors across the swarm.

### 9.2 The "Precog" Dream State Hallucination
The Dreamer (Precog Safety) component runs hypothetical scenarios in a "Dream State" to evaluate safety. These runs use real tokens but should logically be separated from actual project expenditure, or at least categorized under a specific ephemeral shard type.

*   **Trigger:** The Dreamer generates a massive hallucinated context (e.g., simulating a complex multi-file attack vector) to test the Legislator's response.
*   **System Impact:** The system logs millions of tokens under `shard_type="dreamer"`.
*   **Test Case Design:** If a user cancels a Dream State run midway, does the usage tracker record partial usage? We must write a test where `context.WithCancel` is fired *during* the generation phase of a large streaming response. The test must assert that `Track` captures the exact number of tokens processed up to the millisecond of cancellation, preventing silent token leakage where an aborted run's tokens are lost to the ether.

### 9.3 The ConfigFactory JIT Context Reload
Under the new JIT Clean Loop architecture (Dec 2024), the session starts fresh every turn. The ConfigFactory rapidly rebuilds tools and policies based on the parsed `user_intent`.

*   **Trigger:** A user intent fluctuates rapidly between `multi_step` and `clarify`, causing the JIT compiler to repeatedly swap out massive tool registries (e.g., loading the entire `rod-builder` schema then swapping to `mangle-programming`).
*   **System Impact:** The system generates a high volume of small, disjointed `usage_event` entries. If `WithEventLog` is enabled, the 1000-event ring buffer will wrap in minutes.
*   **Test Case Design:** We must ensure the ring buffer wraps safely under concurrent load. A test should concurrently push 5,000 events across 50 goroutines and assert that the final length of `t.data.Events` is exactly 1000, and that no panic occurred due to the slice reallocation (`append(t.data.Events[1:], event)`).

## 10. Memory Leak Vectors in Telemetry

Telemetry systems are notorious for introducing subtle memory leaks, especially when dealing with bounded maps that rely on garbage collection.

### 10.1 The Zombie Turn IDs
The `turns` map is bounded by `maxTurns` (1024). When the limit is reached, the oldest turn is deleted: `delete(t.turns, t.turnOrder[0])`.

*   **Analysis:** In Go, deleting an item from a map does not immediately shrink the map's underlying memory allocation. If the system tracks 1024 extremely large values (though here they are small `TokenCounts` structs), the map's memory footprint grows. While `TokenCounts` is small, what if `turnID` strings become massive due to a bug in the context generator?
*   **Test Case Design:** We should construct a test that generates 100,000 unique `turnID` strings, each 1KB in size, and processes them through `recordTurnLocked`. We then measure the `runtime.MemStats` before and after. If the memory footprint grows substantially and does not decrease after forced GC, we have a leak due to the map's backing array retaining the string keys.

### 10.2 Shared Ownership Pointer Retention
The `Tracker` uses a reference count (`refs`) to manage shared ownership across multiple shards (e.g., Cortex and interactive chat).

*   **Analysis:** The `Close()` method decrements the reference count and unregisters the shared key when `refs` hits 0. However, if a single client forgets to call `Close()`, the `Tracker` remains registered in the global `sharedTrackers` map indefinitely.
*   **Test Case Design:** This is a classic leak vector. We must write a negative test where `Shared(ws)` is called 5 times, but `Close()` is only called 4 times. We then verify that a subsequent `Shared(ws)` returns the exact same memory pointer, proving it was not evicted. Finally, we must ensure that the framework's overarching shutdown mechanism has a failsafe to forcefully purge abandoned trackers, or we risk OOM in long-running daemon mode.

## 11. Final Recommendations for the JIT Era

The December 2024 migration to the JIT Clean Loop architecture radically changes the expected load profile on the `internal/usage` system. Instead of long-lived, stable shards trickling usage data, we now face ephemeral, highly concurrent, JIT-compiled subagents blasting the tracker simultaneously.

The immediate priority for the QA team is to transition from static unit testing to dynamic fuzzing and concurrent stress testing. The O(N log N) pruning sort under a global mutex is the most critical vulnerability identified in this analysis and must be refactored before deploying the Thunderdome multi-agent swarm capability in production.

## 12. Cross-Process and IPC Durability

In addition to concurrent in-process load, the usage tracker must withstand extreme conditions when multiple processes attempt to access the same workspace data simultaneously. This is a common scenario in the codeNERD ecosystem, where a background daemon might be running campaigns while a user simultaneously invokes a one-shot CLI command against the same repository.

### 12.1 The File Lock Contention Scenario
The `filelock` mechanism is designed to prevent two processes from corrupting `usage.json`. However, negative testing must evaluate what happens when the lock is violently held or abruptly orphaned.

*   **Trigger:** Process A acquires the lock and begins writing. Process A is suddenly terminated (SIGKILL) before releasing the lock. Process B attempts to initialize a Tracker on the same workspace.
*   **System Impact:** If the lock implementation relies on a stale PID file or a non-atomic file creation mechanism, Process B might hang indefinitely, waiting for a lock that will never be released, resulting in a silent failure of the CLI command.
*   **Test Case Design:** Construct a multi-process test where a child process acquires the usage lock and is immediately `syscall.SIGKILL`'ed. The parent process must then attempt to acquire the lock. The test asserts that the system correctly identifies the orphaned lock (e.g., using `F_SETLK` on Unix or `LockFileEx` on Windows, which are released by the OS on process death) and successfully recovers without hanging.

### 12.2 Atomic File Swap Interruption
The `atomicfile.Write` mechanism writes to a temporary file and renames it to `usage.json` to ensure atomic updates.

*   **Trigger:** A hard power failure or kernel panic occurs exactly after the `write` syscall to the temporary file finishes, but before the `rename` syscall completes.
*   **System Impact:** On reboot, the `usage.json` might be stale, and a stray `.tmp` file exists in the directory.
*   **Test Case Design:** While difficult to simulate a true power failure in standard unit tests, we can write a test that intentionally mocks the file system interface to fail the `rename` operation. The system must verify that the original `usage.json` remains completely intact and readable, ensuring that at worst, we lose the most recent 5 seconds of tracking data, rather than corrupting the entire historical ledger.

## 13. Deep Dive: Precision Loss over Extended Campaigns

Returning to the issue of `float64` precision loss, a deeper mathematical analysis is required to truly understand the boundary limits of the current implementation.

### 13.1 IEEE 754 Mantissa Exhaustion
A `float64` has 53 bits of precision in its mantissa. This allows it to represent integers exactly up to 2^53 (approx. 9 quadrillion). However, when adding very small fractional numbers (like $0.000001 per token) to a moderately large number (like $10,000.00), the precision limits are hit much earlier.

*   **Mathematical Boundary:** If the total cost reaches $131,072 (2^17), the smallest change it can register is approximately $0.00000001. If a model costs less than this per token, the addition becomes a mathematical no-op. The tokens are consumed, but the cost never increments.
*   **System Impact:** For extremely long-running campaigns utilizing vast swarms of highly optimized, cheap sub-models, the overall project cost will silently decouple from reality, underreporting the actual API spend.
*   **Test Case Design:** A specific negative test must initialize `TotalProject.Cost` to $200,000.00. It must then loop 1,000,000 times, adding $0.000000001 (representing an ultra-cheap micro-model). The test must assert that the final cost is exactly $200,000.001. Under the current `float64` implementation, this test will fail, proving the architectural flaw.

### 13.2 Remediation: The Big-Integer Shift
To definitively resolve the precision loss, the core data structures (`TokenCounts` and `AggregatedStats`) should be migrated from `float64` to exact-precision representations.

*   **Proposal:** Introduce a `MicroCent` type alias for `int64`. One MicroCent represents $0.000001.
*   **Implementation:** All internal accumulation happens via `atomic.AddInt64`. When a user requests the stats via the CLI, the display layer divides by 1,000,000 and formats it as a standard USD string.
*   **Benefits:** This completely eliminates IEEE 754 floating-point drift, guarantees absolute accuracy regardless of scale, and allows for lock-free atomic accumulation across highly concurrent agent swarms, simultaneously solving the precision issue and alleviating the global mutex contention problem.

## 14. Holistic Subsystem Synthesis

The `internal/usage` tracker is deceptively simple. It appears as a basic metrics accumulator, but in the context of the codeNERD Logic-First framework, it is the central nervous system for budget awareness and rate-limiting. As the system scales to support the "Thunderdome" multi-agent concurrency model, the tracker transforms from a simple logging utility into a critical, high-throughput synchronization point.

By systematically applying Boundary Value Analysis, we have uncovered two primary existential threats to the subsystem's future viability:
1.  **The Mutex/Pruning Bottleneck:** The O(N log N) sorting logic within a global mutex will inevitably strangle the Ouroboros Loop under high concurrency.
2.  **The Floating-Point Drift:** The inevitable precision loss when tracking massive volumes of cheap tokens will lead to fundamentally inaccurate financial reporting over long time horizons.

The recommended architectural shifts—migrating to atomic fixed-point arithmetic and implementing lock-free sharded accumulators—will robust the system against these negative boundaries, ensuring the codeNERD framework remains performant, accurate, and stable regardless of the scale of the cognitive workload it is tasked to execute.

## 15. The Role of Mangle Logic in Usage Tracking

While the `internal/usage` system is currently implemented in imperative Go, its role in the broader codeNERD architecture must interface with the Mangle logic kernel. The executive logic of the system (e.g., stopping a campaign if budget is exceeded) is declarative.

### 15.1 Atom/String Dissonance in Budget Rules
A critical failure vector occurs at the boundary where Go tracking data is converted into Mangle facts for the logic engine to evaluate.

*   **Trigger:** The system asserts a fact like `usage_stat("glm-4.6", 500)`. A Mangle policy rule expects an Atom, such as `budget_limit(/glm-4.6, 1000)`.
*   **System Impact:** Due to the "Atom/String Dissonance," the logic engine will fail to join the string `"glm-4.6"` with the atom `/glm-4.6`. The rule will evaluate to an empty result set. The campaign will continue executing indefinitely, oblivious to the fact that it has exceeded its budget, resulting in a massive, uncontrolled API spend.
*   **Test Case Design:** We must implement a cross-boundary integration test. The test must mock a scenario where `Track` records 10,000 tokens. It must then invoke the `VirtualStore` FFI to bridge these stats into the Mangle `factstore`. Finally, it must run `analysis.Analyze` and execute a policy rule: `stop_campaign :- usage_stat(Model, Cost), budget(Model, Limit), Cost > Limit`. The test explicitly asserts that the generated facts use strict `ast.Name` constructors, preventing the silent string mismatch failure.

### 15.2 The Monotonicity of Spend
Mangle evaluation is strictly monotonic; facts can only be added during a fixpoint iteration, never removed. Usage tracking is also conceptually monotonic—tokens are only ever consumed, never refunded during a run.

*   **Trigger:** A bug in a provider's SDK causes it to return a negative token count (e.g., due to an integer underflow in their backend).
*   **System Impact:** If `Track` accepts a negative token count, the `TotalProject.Cost` would decrease. If this updated total is fed into Mangle in a subsequent turn, it violates the monotonic assumption of the budget rules. While Mangle's `factstore` resets between turns in the JIT architecture, a decreasing cost can cause cyclical logic instability in historical auditing rules.
*   **Test Case Design:** As noted in section 2.2, the Go code currently drops negative counts. The test suite must rigorously verify this guard rail. A negative test must inject `Track(ctx, "mod", "prov", -500, -100, "chat")` and assert via `t.Fatalf` if the total cost decreases, ensuring the monotonic nature of the spend ledger is mathematically guaranteed at the Go layer before it ever reaches the Mangle kernel.

## 16. Security and Injection Vectors in Telemetry

Telemetry data is often viewed as benign, but in a system that dynamically generates queries or interfaces with external dashboards, it can become an injection vector.

### 16.1 Malicious Provider Name Injection
When an agent dynamically loads a new external tool, the tool might specify its own LLM provider string.

*   **Trigger:** A rogue tool defines its provider name as `"ZAI_Provider

-- DROP TABLE usage;"`.
*   **System Impact:** If the `usage.json` is later parsed by an external analytics tool (e.g., a fragile bash script using `jq` and `sed`, or imported into a SQL database without parameterization), the newline and SQL injection payloads could trigger arbitrary command execution or data destruction on the analyst's machine.
*   **Test Case Design:** A security-focused negative test must inject known malicious payloads (SQLi, XSS, Path Traversal strings like `../../../etc/passwd`) into the `provider`, `model`, and `operation` fields. The test must verify that the `usage_tracker` safely escapes or sanitizes these strings before writing them to the `usage.json` file. While `encoding/json` handles most basic escaping, testing for extremely long strings or non-UTF8 byte sequences is critical to ensure the JSON encoder doesn't panic or produce invalid JSON.

### 16.2 Denial of Wallet (DoW) via Operation Spoofing
If an agent can maliciously spoof the `operation` string, it could hide expensive operations under cheaper categories.

*   **Trigger:** An agent performing an expensive `tool_gen` operation calls `Track` but hardcodes the operation string as `chat`.
*   **System Impact:** The telemetry data becomes polluted. Audits will show massive spend on `chat` and zero spend on `tool_gen`, leading to incorrect architectural decisions regarding which subsystems to optimize.
*   **Test Case Design:** This points to a deeper architectural boundary issue. The `operation` string should not be an arbitrary string passed by the caller. It should be a strictly typed enumeration enforcing an allowlist at compile time. A negative test should attempt to pass `"bitcoin_mining"` as an operation. The tracker should reject it or categorize it under `"unknown"`. The current implementation allows arbitrary strings, representing a minor validation boundary failure.

## 17. Final QA Sign-off Requirements

Before the `internal/usage` system can be considered fully hardened for the next release cycle, the following QA gates must be passed:

1.  **Unit Test Parity:** All `// TODO:` comments added in this cycle must be implemented with dedicated, isolated unit tests.
2.  **Concurrency Benchmark Passing:** `BenchmarkTrackConcurrent` must demonstrate a sub-millisecond p99 latency even when `maxSessions` is exceeded and 1,000 goroutines are contending for the lock.
3.  **Mangle Integration Verified:** At least one golden-file test must exist proving that usage metrics successfully cross the Go/Mangle FFI boundary using correct `ast.Name` types without dropping facts.
4.  **No-Drift Guarantee:** The `float64` precision loss test must be implemented and passing, either by proving the drift is negligible over a 10-year simulated campaign, or by migrating the core structures to integer-based `MicroCents`.

By completing these rigorous negative tests, codeNERD's usage tracking will evolve from a simple logging utility into a mathematically sound, high-assurance ledger capable of orchestrating complex financial logic within the Mangle rule engine.

## 18. The Ephemeral Boot and Usage Persistence

The codeNERD framework's "Quiescent Boot" architecture (introduced in Dec 2024) dictates that every session starts with a clean slate. Ephemeral facts are filtered at kernel boot. However, usage tracking is an explicitly durable concern that spans across these ephemeral boots. This creates a fascinating boundary condition between ephemeral state and durable ledgering.

### 18.1 The "Ghost Fact" Collision
In a standard system, state is either entirely ephemeral or entirely durable. In codeNERD, they mix.

*   **Trigger:** A session boots, runs for 10 minutes, and tracks $5.00 of usage. The session undergoes a soft reset (Quiescent Boot) but the process remains alive. The `internal/usage` tracker remains in memory. The session re-authenticates and begins tracking again.
*   **System Impact:** The tracker must seamlessly aggregate the new tokens into the existing durable file, without allowing the "ghost facts" (ephemeral session state that was cleared) to corrupt the new session's baseline. The `baseline` field in `tracker` handles this merging logic, but it is highly complex.
*   **Test Case Design:** A test must simulate a Quiescent Boot sequence. It tracks tokens, calls `Save()`, simulates the boot (clearing the active turn context but keeping the `Tracker` instance alive), tracks more tokens, and calls `Save()` again. The test must assert that the final `usage.json` reflects the sum of both phases, and that the `turnOrder` map did not leak IDs from the pre-boot phase into the post-boot phase.

### 18.2 File System Race on Boot Merging
When the system boots, it reads `usage.json` to populate its `baseline` aggregates.

*   **Trigger:** Two separate codeNERD instances (e.g., CLI workspaces) boot simultaneously against the same repository. Both read `usage.json` into their `baseline`. Instance A tracks $1.00 and saves. Instance B tracks $2.00 and saves.
*   **System Impact:** The "Lost Update" problem. Because both instances read the same baseline, Instance B's save will overwrite Instance A's save, resulting in a total of $2.00 instead of $3.00.
*   **Test Case Design:** This is a distributed systems boundary failure. The test must simulate two `Tracker` instances instantiated in separate go-routines representing different processes (mocking the `filelock` to allow simultaneous reads). It asserts that the current architecture *fails* this test (resulting in lost updates). The remediation requires moving to a purely append-only WAL architecture, as previously discussed, or implementing optimistic concurrency control (e.g., an ETag or version number in `usage.json`) where `Save()` fails and retries if the file was modified by another process since the last read.

## 19. Conclusion of Expanded Analysis

This document has systematically torn down the `internal/usage` tracking subsystem through the lens of Boundary Value Analysis and Negative Testing. By pushing the boundaries of memory, concurrency, precision, distributed file locking, and logic engine integration, we have mapped the fragile edges of an otherwise solid implementation.

The discovery of the O(N log N) pruning bottleneck and the IEEE 754 float64 drift are critical findings that justify immediate architectural refactoring. Furthermore, the integration risks with the Mangle Logic engine and the JIT Clean Loop highlight the necessity of testing not just the Go code in isolation, but how its outputs behave when ingested as declarative facts by the broader neuro-symbolic framework.

This journal entry serves as the foundational roadmap for the next sprint of QA automation development, ensuring codeNERD's telemetry remains an unbreakable source of truth.

## 20. Addendum: Handling Vector Database Ingestion Spikes

The `internal/usage` tracker also needs to account for extreme token usage generated during bulk ingestion phases. When codeNERD builds its Vector DB (via `sqlite-vec`), it must embed massive amounts of text.

### 20.1 The Bulk Embedding Deluge
*   **Trigger:** A brownfield monorepo with 50 million lines of code is ingested. The `rod-builder` or embedding agent shards the codebase and blasts the embedding API.
*   **System Impact:** Millions of tokens are processed in a matter of minutes. The `Track` function is called recursively and continuously.
*   **Test Case Design:** We must simulate an embedding deluge. The test must mock an embedding operation that generates 10,000 requests per second, each consuming exactly 8,192 tokens (a standard max context window for small embedding models). The test must verify that the `ByOperation["embedding"]` aggregate correctly handles this massive, rapid influx without dropping counts or suffering from integer overflow on 32-bit architectures. This validates the system's readiness for massive brownfield onboarding tasks.

### 20.2 The Missing Cost Conundrum
*   **Trigger:** If the embedding model used during the deluge is unpriced or custom, `EstimateCost` returns 0 and `priced` is false.
*   **System Impact:** The `UnpricedTokens` integer will skyrocket. If it exceeds `int64` limits, it will wrap around to negative numbers.
*   **Test Case Design:** A test must simulate tracking `1<<62` tokens to an unpriced model multiple times to explicitly verify behavior at the 64-bit integer boundary.
