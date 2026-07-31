---
surface: "Spawner ↔ APIScheduler"
mode: "boundary"
subsystems_tested: ["Spawner", "APIScheduler", "SubAgent"]
blast_radius: "critical"
remediated: false
---

# Spawner and APIScheduler Integration Siege Analysis

## 1. System Interaction Map

- `Spawner.SpawnAsyncWithContext`: The entry point for multi-agent delegation. It registers the newly created `SubAgent` in the `Spawner.subagents` map, increments `activeSubagents` implicitly via the map length, and begins the subagent goroutine.
- `SubAgent.ExecuteWithContext`: The subagent requests an API slot by calling `core.GetAPIScheduler().AcquireAPISlot(ctx, agentID)`.
- `APIScheduler.AcquireAPISlot`: Blocks the caller (SubAgent) if slots are full. Uses cooperative yielding with wait queues, and timeout via the passed `ctx`.
- `APIScheduler.ReleaseAPISlot`: Returns the slot to the pool, granting it to the next waiter.
- `Spawner.Shutdown`: Signals all subagents to terminate, potentially racing with slot acquisition.
- `APIScheduler.ReportRateLimit`: Dynamically updates the `MaxConcurrentAPICalls` parameter based on downstream pressure.
- `APIScheduler.ReportSuccess`: Dynamically recovers `MaxConcurrentAPICalls` post rate-limiting.
- `Spawner.SetLimitsEnforcer`: Configures global capacity controls across sessions.

## 2. Contract Analysis

- **Slot Release Guarantee:** The APIScheduler implicitly trusts that any `SubAgent` that acquires a slot via `AcquireAPISlot` will eventually call `ReleaseAPISlot`, even if the subagent encounters a panic, context cancellation, or error.
- **Acquire Timeout Handling:** The SubAgent expects `AcquireAPISlot` to return `ctx.Err()` if the context expires before a slot is available. If `AcquireAPISlot` does not respect the context correctly, the SubAgent will hang indefinitely, leaking memory and exhausting the Spawner's internal `maxActiveSubagents` quota.
- **Max Spawns vs Max Slots:** The Spawner's `maxActiveSubagents` and the APIScheduler's `MaxConcurrentAPICalls` are independent configurations. A contract exists such that the Spawner can spawn more agents than available API slots, relying on the APIScheduler's queueing mechanism to throttle them safely without deadlocks.
- **Dynamic Slot Resizing:** The `Spawner` assumes that when `APIScheduler` resizes slots due to rate limiting, currently running `SubAgent`s are not forcefully terminated, but new slots are constrained.
- **Unique SubAgent Identity:** `APIScheduler` assumes the caller ID used in `AcquireAPISlot` perfectly maps 1:1 with a unique `SubAgent` session. A duplicate ID will corrupt the active slot map.

## 3. Failure Mode Enumeration

- **Temporal (Acquire Timeout & Leak):** If a subagent's context is cancelled exactly while it is waiting in the APIScheduler's queue, does the APIScheduler correctly clean up the `schedWaiter` channel? If not, the scheduler leaks memory and subsequent `ReleaseAPISlot` calls might unblock a dead channel.
- **Semantic (Double Release):** If a subagent calls `ReleaseAPISlot` twice due to a deferred release and an explicit release in an error path, the APIScheduler could panic or grant a slot that doesn't exist.
- **Ordering (Cancel then Release):** A subagent context is cancelled, but it already acquired the slot. Does the subagent correctly release it, or does it return early because `ctx.Err()` is non-nil *after* the fact?
- **Partial (Panic during LLM call):** If the LLM client panics while the subagent holds an API slot, the deferred `ReleaseAPISlot` might not execute if the panic is not recovered properly inside the SubAgent's execute loop.
- **Corruption (Mass Spawning):** 1000 SubAgents are spawned concurrently into an APIScheduler configured with only 5 slots. If `pendingSpawns` in the Spawner or `waitQueue` in the APIScheduler lack proper locking, internal state corruption will occur.
- **Temporal (Shutdown Race):** If `Spawner` triggers shutdown while `APIScheduler` is in the middle of popping from its wait queue, a grant might be sent to a closed context, leaking the slot permanently.
- **Semantic (Adaptive Starvation):** The dynamic adaptive concurrency reduces max slots to 0 (or a very low bound). Agents queue up and contexts expire en-masse, crashing all executing campaigns simultaneously.
- **State Leakage (Zombie Waiters):** High churn of very fast, quickly cancelled spawns leaves residual wait channel structs in the heap.

## 4. Adversarial Scenario Design

1. **Scenario: Spawner floods APIScheduler beyond WaitQueue capacity**
   - **Contract Violated:** Spawner assumes APIScheduler can queue an unbounded number of waiters.
   - **Failure Injection:** Spawn 500 SubAgents concurrently. APIScheduler `MaxConcurrentAPICalls` is 2.
   - **Expected Behavior:** System queues correctly without deadlocking or dropping spawns. Context timeouts apply to the wait queue.
   - **Severity:** P1

2. **Scenario: SubAgent context cancelled mid-Acquire**
   - **Contract Violated:** Scheduler wait queue must respond instantly to `ctx.Done()`.
   - **Failure Injection:** SubAgent calls `AcquireAPISlot`. Wait 1ms, cancel context before slot granted.
   - **Expected Behavior:** `AcquireAPISlot` returns `context.Canceled` immediately. No slot is leaked. Waiter removed from queue.
   - **Severity:** P0

3. **Scenario: SubAgent panics while holding API slot**
   - **Contract Violated:** SubAgent must guarantee `ReleaseAPISlot` even on panic.
   - **Failure Injection:** Inject a panicking LLM Client.
   - **Expected Behavior:** SubAgent recovers panic, calls `ReleaseAPISlot`, and terminates safely. Spawner cleans up.
   - **Severity:** P0

4. **Scenario: Spawner max capacity reached due to Scheduler starvation**
   - **Contract Violated:** Spawner's `maxActiveSubagents` shouldn't prevent new important tasks if current tasks are just waiting.
   - **Failure Injection:** Set Spawner max to 10, Scheduler max to 1. Spawn 10 slow tasks. Spawn 1 critical priority task.
   - **Expected Behavior:** The 11th task might fail to spawn, OR (if implemented) it bypasses or preempts. If it fails, it should fail fast.
   - **Severity:** P2

5. **Scenario: APIScheduler Adaptive Throttle triggers during mass spawn**
   - **Contract Violated:** Scheduler adaptive throttling shouldn't cause Spawner deadlocks.
   - **Failure Injection:** Trigger `ReportRateLimit` heavily to drop `MaxConcurrentAPICalls` to 1 while 50 SubAgents are spawned.
   - **Expected Behavior:** Waiters block longer, contexts may timeout, but no deadlocks.
   - **Severity:** P1

6. **Scenario: Scheduler ForceRelease called on active SubAgent**
   - **Contract Violated:** SubAgent assumes its slot is held until it releases it.
   - **Failure Injection:** Another system calls `ForceReleaseSlot(subAgentID)`. SubAgent then finishes and calls `ReleaseAPISlot`.
   - **Expected Behavior:** SubAgent's `ReleaseAPISlot` should not crash the scheduler or grant an extra slot.
   - **Severity:** P1

7. **Scenario: Double Slot Acquisition Attempt**
   - **Contract Violated:** A single SubAgent should only hold one slot.
   - **Failure Injection:** A malicious SubAgent configuration triggers two simultaneous `AcquireAPISlot` calls for the same ID.
   - **Expected Behavior:** The second acquire is blocked or returns an error, preventing ID hijacking.
   - **Severity:** P2

8. **Scenario: Spawner Shutdown while WaitQueue is full**
   - **Contract Violated:** Spawner must cleanly cancel all waiters.
   - **Failure Injection:** Call `Spawner.Shutdown()` while 100 SubAgents are blocked in `AcquireAPISlot`.
   - **Expected Behavior:** SubAgents exit `AcquireAPISlot` with cancellation error, APIScheduler WaitQueue drains completely.
   - **Severity:** P0

9. **Scenario: Rapid Spawn/Cancel cycle leaks Waiters**
   - **Contract Violated:** High churn shouldn't leave zombie waiters in APIScheduler.
   - **Failure Injection:** Loop 10,000 times: Spawn SubAgent, immediately cancel context.
   - **Expected Behavior:** APIScheduler wait queue length remains near 0. No goroutine leaks.
   - **Severity:** P0

10. **Scenario: Zero Slot Configuration**
    - **Contract Violated:** Configuration edge cases should be handled gracefully.
    - **Failure Injection:** Configure APIScheduler with 0 max slots.
    - **Expected Behavior:** `AcquireAPISlot` immediately fails or blocks until timeout. Doesn't panic on divide-by-zero or empty channel creation.
    - **Severity:** P3

11. **Scenario: Slot acquired exactly as context expires**
    - **Contract Violated:** Race condition between `ctx.Done()` and slot grant channel.
    - **Failure Injection:** Use deterministic scheduling to grant a slot precisely when the context deadline is reached.
    - **Expected Behavior:** The slot is either successfully acquired (and subsequently released by the caller) OR the acquire fails and the slot is immediately returned to the pool.
    - **Severity:** P0

12. **Scenario: APIScheduler Stop called with active slot holders**
    - **Contract Violated:** Scheduler shutdown must wait for or forcibly detach holders.
    - **Failure Injection:** Call `APIScheduler.Stop()` while SubAgents are making LLM calls.
    - **Expected Behavior:** `Stop()` shouldn't disrupt active calls, or if it does, it cancels their contexts safely.
    - **Severity:** P1

13. **Scenario: SubAgent ID collision in APIScheduler**
    - **Contract Violated:** APIScheduler assumes IDs are globally unique.
    - **Failure Injection:** Spawner bugs cause two SubAgents to share an ID. Both attempt to acquire slots.
    - **Expected Behavior:** The scheduler handles it gracefully (e.g., rejecting the second) rather than corrupting its internal map.
    - **Severity:** P2

14. **Scenario: Long-running task starves others without yielding**
    - **Contract Violated:** APIScheduler expects tasks to complete reasonably quickly.
    - **Failure Injection:** A SubAgent simulates an LLM call that hangs for 1 hour.
    - **Expected Behavior:** APIScheduler slot remains occupied. Other tasks queue up. This tests the necessity of Spawner-level timeouts.
    - **Severity:** P1

15. **Scenario: Spawner max capacity check race condition**
    - **Contract Violated:** `pendingSpawns` and `subagents` map length must be synchronized.
    - **Failure Injection:** Concurrently spawn 100 agents on a Spawner with max 10.
    - **Expected Behavior:** Exactly 10 spawn, 90 return capacity error. No exceeding max capacity.
    - **Severity:** P0

## 5. Cascading Failure Analysis

- **P0: Context Cancel Leak in WaitQueue**
  - **Cascade:** If the APIScheduler fails to clean up cancelled waiters, the internal slice grows unbounded. Eventually, memory is exhausted (OOM). More immediately, when legitimate slots are freed, they might be sent to closed/abandoned channels, causing panics or silently dropping the slot (permanent slot loss), leading to total system deadlock where no SubAgents can proceed.
  - **Downstream Impact:** The `Spawner` stops fulfilling new requests. The `Orchestrator` stalls permanently on phase transitions. The user's CLI session hangs with no output.
- **P0: Panic during LLM call holding slot**
  - **Cascade:** If the subagent panics and fails to execute `defer ReleaseAPISlot()`, the APIScheduler permanently loses 1 slot. In a 3-slot system, 3 panics mean total deadlock. The Spawner will hit `maxActiveSubagents` because tasks queue indefinitely. The Campaign Orchestrator will freeze. The user session hangs waiting for facts.
  - **Downstream Impact:** Mangle kernel queries timeout. Piggyback control packets are never generated, causing the legislative policy loops to fail closed.
- **P0: Spawner Shutdown with Full WaitQueue**
  - **Cascade:** If `Spawner.Shutdown()` does not aggressively cancel the contexts of queued subagents, the subagent goroutines leak. The application cannot cleanly restart or exit. Shared resources (like SQLite database connections) might be held open by zombie subagents waiting for a slot that will never be granted.
  - **Downstream Impact:** VirtualStore persistence layers hit connection limits. Subsequent daemon reboots require manual `kill -9`.
- **P1: Adaptive Concurrency Flapping**
  - **Cascade:** If rate limits flap rapidly, the APIScheduler constantly resizes `MaxConcurrentAPICalls`. This causes severe latency jitter in `AcquireAPISlot`.
  - **Downstream Impact:** The OODA loop's `Perception` transducer times out. `Transducer` assumes LLM is dead and surfaces generic fallback errors, ruining the user experience.
- **P1: Double Release Corruption**
  - **Cascade:** SubAgent bug causes it to release the slot twice. The APIScheduler's capacity artificially inflates (e.g., from 2 slots to 3).
  - **Downstream Impact:** The system exceeds upstream LLM provider rate limits, triggering hard 429s. All subsequent tasks fail globally until rate limit resets.

## 6. Architectural Deep Dive: The Slot Lifecycle
The `APIScheduler` operates on a dual-lock system (global config lock, and local map lock). The `Spawner` operates on a single `RWMutex`.
When a `SubAgent` crosses from `Spawner` management to `APIScheduler` queueing, it enters a "No Man's Land" where neither subsystem has full visibility over the subagent's true state.
- **The Spawner View:** "I have spawned this agent, it is running."
- **The Scheduler View:** "I have queued an anonymous ID waiting for a boolean grant."
This opacity is what necessitates extreme rigor on the `context.Context` cancellation propagation. If the Spawner abandons the subagent, the context *must* be the single source of truth for the Scheduler to evict it.

## 7. Memory Leak Profiling Hypothesis
To prove the `WaitQueue` leak scenario, one would instrument `pprof` around the `schedWaiter` struct. In an adversarial condition where 10,000 subagents are spawned and immediately cancelled, the heap profile should show zero lingering `chan struct{}` allocations in the `APIScheduler`. If they linger, the garbage collector is being defeated by a dangling slice reference inside the `waitQueue`.

## 8. The "Slow-Loris" Spawning
In adversarial security modeling, a "Slow-Loris" attack involves sending data very slowly to keep connections open, eventually exhausting the server's connection pool. A similar dynamic exists at the integration boundary of the `Spawner` and `APIScheduler`.
- **The Vector:** If an external system (like a rogue plugin or a malfunctioning MCP tool) rapidly generates user intents that require complex, long-running agent execution, the `Spawner` will dutifully create them.
- **The Execution:** If the tasks are designed to yield the API slot frequently (e.g., via the cooperative yielding model of the TDD loop), they don't immediately exhaust the `MaxConcurrentAPICalls`. However, they *do* exhaust the `maxActiveSubagents` in the `Spawner`.
- **The Result:** Legitimate, fast-path queries (e.g., "What time is it?") are dropped because the Spawner's internal map is full of "Slow-Loris" agents that are constantly cycling in and out of the API scheduler queue.
- **Mitigation Strategy:** The `Spawner` must implement a time-to-live (TTL) for every `SubAgent`. If an agent has not completed its task within 15 minutes, it is forcibly terminated, regardless of whether it is actively holding an API slot or waiting in the queue.

## 9. Subsystem Metrics Desynchronization
Metrics are critical for system observability, but the `Spawner` and `APIScheduler` maintain independent, loosely coupled metrics.
- **The Problem:** The `Spawner` tracks `ActiveCount`, which includes agents running *and* agents waiting. The `APIScheduler` tracks `ActiveSlots` (currently running) and `WaitingForSlot` (queued).
- **The Anomaly:** During a race condition where a SubAgent context is cancelled right as a slot is granted, the `Spawner` will decrement `ActiveCount` and consider the agent dead. However, if the `APIScheduler` fails to process the cancellation correctly, it will increment `ActiveSlots`.
- **The Observable Symptom:** An operator looking at the Grafana dashboard will see `Spawner.ActiveCount = 0` but `APIScheduler.ActiveSlots = 5`. The system appears idle, but all API capacity is permanently locked.
- **Testing Approach:** E2E tests must query the metrics endpoints of both subsystems simultaneously after injecting failures, asserting that the invariants hold (e.g., `APIScheduler.ActiveSlots + APIScheduler.WaitingForSlot <= Spawner.ActiveCount`).

## 10. Resource Isolation across Namespaces
If codeNERD is deployed in a multi-tenant environment (e.g., serving multiple users from a single daemon), the `Spawner` must apply namespace isolation.
- **Vulnerability:** The `APIScheduler` is a global singleton (`core.GetAPIScheduler()`). It has no concept of "Tenant A" vs "Tenant B".
- **The Cascade:** A runaway loop in Tenant A (e.g., an Autopoiesis bug) spawns 1,000 agents. These agents flood the global API scheduler queue. Tenant B attempts to run a single command and is blocked indefinitely.
- **Remediation:** The `APIScheduler` must be refactored to support fair-share queuing (e.g., Deficit Round Robin) based on a tenant ID embedded in the `SpawnRequest`, or the `Spawner` must enforce per-tenant quotas *before* submitting to the global queue.

## 11. The Impact of Mangle Logic Stratification
The Mangle kernel uses stratification to prevent logical paradoxes (e.g., rules that depend on their own negation).
- **How it affects the Spawner:** The `Spawner` decides *what* agent to spawn based on JIT-compiled intents derived from Mangle (`intent_routing.mg`).
- **The Vulnerability:** If a user provides an adversarial input that causes the Mangle kernel to evaluate a heavily recursive or near-unstratified rule block, the `Spawner` might block for several seconds while Mangle attempts to reach a fixpoint.
- **The Downstream Effect:** During this time, the `Spawner` holds its internal `RWMutex`. Any SubAgent trying to complete and deregister itself will block on the `Spawner`'s lock. Thus, a slow kernel evaluation artificially inflates the API Scheduler's wait times, because agents cannot cleanly exit their lifecycle.
- **Conclusion:** The execution of Mangle queries within the Spawner's critical sections must be strictly separated from the SubAgent lifecycle management locks.

## 12. Matrix of Boundary Contracts
| Contract | Assumed By | Guaranteed By | Failure Result |
|----------|------------|---------------|----------------|
| **Release on Panic** | APIScheduler | SubAgent (via defer) | Permanent Slot Leak |
| **Instant Cancel Yield** | SubAgent | APIScheduler (Queue) | Thread/Goroutine Leak |
| **Unique Identity** | APIScheduler | Spawner (UUID/Hash) | Map Corruption / Deadlock |
| **Bounded Wait Time** | Spawner | APIScheduler | OODA Loop Failure |
| **Backpressure Signal** | Spawner | APIScheduler (RateLimit) | OOM / Thrashing |

## 13. The Fallacy of the "Unlimited" Token Budget
The `SpawnerConfig` accepts a `TokenBudget` which dictates how many LLM tokens a particular session is allowed to burn.
- **The Integration Friction:** The `Spawner` enforces this budget, but the `APIScheduler` controls the physical connection.
- **The Vulnerability:** If a SubAgent starts a streaming LLM request, it acquires an API slot. As the tokens stream in, the SubAgent's internal counter decrements. If the budget hits zero mid-stream, the SubAgent must cancel the stream.
- **The Cascade:** If the SubAgent's cancellation logic forcefully closes the HTTP transport without draining the response body, Go's `net/http` package will leak the TCP connection. While the API slot is released, the physical socket remains in `CLOSE_WAIT`. Over time, this exhausts the OS file descriptors, leading to "too many open files" errors, crashing the entire codeNERD daemon.
- **Remediation:** The SubAgent must cleanly close the response body, and the APIScheduler should monitor the health of the underlying transport pool, perhaps forcing a periodic connection drain if it detects resource exhaustion.

## 14. Subsystem Handoff Verification
When the `Spawner` hands a `SubAgent` off to the `APIScheduler`, there is an implicit assumption that the `SubAgent` is fully initialized and ready to execute.
- **The Issue:** What if the JIT compilation phase inside the `Spawner` fails *after* the agent has been registered in the `Spawner.subagents` map, but *before* it begins its execution loop?
- **The Impact:** The agent exists in the Spawner's memory, taking up capacity, but it will never reach the `APIScheduler.AcquireAPISlot` call, nor will it ever complete. It is a true zombie.
- **Testing Approach:** E2E tests must inject a failure in the `JITCompiler` (e.g., passing a syntax error in the prompt schema) and verify that the `Spawner` cleans up the agent from its internal map and decrements `activeSubagents`.

## 15. Log Flooding Under Contention
A subtle but devastating failure mode under extreme contention is log flooding.
- **The Scenario:** 10,000 agents are waiting in the APIScheduler. The APIScheduler is configured to log a warning every time an agent waits longer than 1 second.
- **The Cascade:** 10,000 agents cross the 1-second threshold simultaneously. The logging subsystem (`logging.CategoryShards`) is bombarded with 10,000 simultaneous writes. Since standard IO operations block, the APIScheduler's queue eviction thread is now blocked waiting on the disk I/O of the logging system. This drastically exacerbates the wait times, creating a positive feedback loop of more warnings and more blocking.
- **Remediation:** The APIScheduler must implement log throttling or deduplication for wait queue warnings. E2E tests should use a mocked logger and assert that the number of log calls remains bounded even when 10,000 agents time out.

## 16. The "Orphaned Slot" Scenario in the Campaign Orchestrator
When the `Campaign Orchestrator` spins up a multi-phase operation, it relies heavily on the `Spawner` to execute tasks and the `APIScheduler` to throttle them.
- **The Contract:** The Orchestrator expects that if a phase is cancelled (e.g., the user hits Ctrl+C), all agents spawned during that phase will immediately release their slots.
- **The Vulnerability:** If the Spawner correctly cancels the agents, but the APIScheduler's queue implementation has a race condition where a slot is granted to an agent at the exact nanosecond its context is cancelled, the slot might become "orphaned".
- **The Cascade:** The agent is dead (cancelled by Orchestrator -> Spawner), but the slot is marked as `Active` in the APIScheduler. The Orchestrator moves to the next phase, which requires more API slots. Eventually, the pool of slots is permanently depleted by these nanosecond races, and the Campaign grinds to a permanent halt in later phases.
- **Remediation:** The `APIScheduler` must perform a final `ctx.Err() != nil` check *after* successfully acquiring the internal slot lock but *before* returning the slot to the caller. If the context is dead, it must immediately release the lock to the next waiter.

## 17. Handling of "Phantom" Waiters
A "phantom" waiter occurs when a `SubAgent` calls `AcquireAPISlot`, but the Go runtime schedules a garbage collection pause right after the call enters the wait queue.
- **The Mechanism:** During the GC pause, the `SubAgent`'s context reaches its timeout. The `APIScheduler`'s internal monitor detects the timeout and evicts the agent from the queue.
- **The Glitch:** When the GC pause ends, the `SubAgent` resumes execution. If the `APIScheduler`'s implementation is flawed, the `SubAgent` might still think it's in the queue, or the channel might receive a spurious grant if the eviction wasn't perfectly synchronized.
- **The Cascade:** The system grants a slot to an agent that has theoretically already timed out, wasting API bandwidth and potentially causing data corruption if the agent's logic didn't anticipate receiving a slot after its internal deadline.
- **Testing Approach:** E2E tests must simulate heavy GC pauses (using `runtime.GC()` or CPU stress) simultaneously with context timeouts to verify the synchronization of the queue eviction logic.

## 18. The Impact of Ephemeral Port Exhaustion
While the `APIScheduler` manages logical slots, the underlying LLM client manages physical HTTP connections.
- **The Bottleneck:** If the APIScheduler is configured with 10,000 slots (e.g., simulating a massive burst capability), the Spawner will happily spin up 10,000 agents.
- **The Cascade:** When these agents all attempt to connect to the LLM provider concurrently, the host operating system might run out of ephemeral ports (usually ~65,000 max, but often configured much lower by default).
- **The Result:** The LLM client will return generic `dial tcp: bind: address already in use` errors. The APIScheduler considers the task failed and releases the slot. The agent retries, causing a rapid churn of slot acquisition and release that thrashes the CPU and pollutes the logs, completely paralyzing the network stack.
- **Mitigation:** The `MaxConcurrentAPICalls` must be mathematically bound not just by the LLM provider's rate limits, but by the physical limits of the host OS's network stack. The APIScheduler config must enforce a hard upper limit.

## 19. The VirtualStore FFI Gateway Contention
When a `SubAgent` requires an API slot, it's often not just for a direct LLM call. It may be calling into the `VirtualStore`, which acts as an FFI (Foreign Function Interface) gateway to external Python scripts (e.g., executing a local search).
- **The Vulnerability:** If the Python execution environment (e.g., a sandboxed Docker container) has a strict concurrency limit (e.g., max 5 parallel executions), but the `APIScheduler` allows 20 slots, we have a resource mismatch.
- **The Cascade:** 20 agents acquire API slots. All 20 attempt to execute a Python script via `VirtualStore`. 15 of them block waiting for the Docker daemon. They hold the API slots hostage.
- **The Result:** The API slots, which are meant to govern LLM rate limits, are now artificially constrained by local compute limits. If a different agent (say, a purely conversational one) just needs an API slot to talk to Claude, it can't get one, because all slots are locked up by agents waiting on Docker.
- **Remediation:** The `APIScheduler` should be refactored into a `ResourceScheduler` that supports distinct resource pools (e.g., `PoolLLM`, `PoolCompute`). A `SubAgent` must acquire the specific resource type it needs. E2E tests should verify that exhausting local compute does not starve remote API access.

## 20. Priority Inversion via Shared Mangle State
The Mangle kernel is the central brain of codeNERD, maintaining the state of the world as logical facts.
- **The Scenario:** A low-priority background `Researcher` agent acquires an API slot. Before it makes the network call, it asserts a series of complex facts into the Mangle kernel (e.g., a large knowledge graph).
- **The Event:** A high-priority `Interactive` agent (prompted directly by the user) is spawned and needs to evaluate its routing policy via the kernel.
- **The Cascade:** The kernel is currently busy processing the massive fact assertion from the low-priority agent. The high-priority agent is blocked on the kernel's `RWMutex`.
- **The Irony:** Even though the `APIScheduler` correctly prioritized the high-priority agent for the next API slot, the agent can't even reach the scheduler because it's blocked on the shared kernel state. This is a classic priority inversion happening *before* the scheduler boundary.
- **Testing Approach:** Integration tests must spawn a low-priority agent that floods the kernel with facts, followed immediately by a high-priority agent. The test must measure the latency of the high-priority agent reaching the `APIScheduler` queue.

## 21. The Re-entrancy Trap in Piggyback Parsers
As mentioned in section 15, the Piggyback protocol allows the LLM to issue control commands back to the system.
- **The Detail:** When the `Executor` parses a Piggyback command (e.g., `<<<<read_file(/etc/passwd)>>>>`), it may need to validate this action against the `VirtualStore` constitution.
- **The Trap:** If the validation requires calling the LLM again (e.g., a semantic safety check), the `Executor` calls `AcquireAPISlot` from *within* the execution loop that already holds a slot.
- **The Consequence:** If the `APIScheduler` is not re-entrant (i.e., it doesn't recognize that the calling thread already owns a slot), it will queue the request. If the scheduler is at `MaxConcurrentAPICalls`, the system instantly deadlocks. The `Executor` holds slot 1, waiting for slot 2. Slot 2 will never be freed because slot 1 is blocked.
- **Remediation:** The `APIScheduler` API must provide a mechanism for rentrancy, such as `AcquireAPISlotContext(parentContext, agentID)`, allowing the scheduler to grant a sub-slot or bypass the limit for recursive trust chains. E2E tests must explicitly construct nested Piggyback scenarios to catch these deadlocks.

## 22. The Chaos Engineering Validation
To truly trust the boundary between the `Spawner` and the `APIScheduler`, we must employ Chaos Engineering principles.
- **The Philosophy:** We assume that hardware, networks, and internal data structures will fail simultaneously.
- **The Chaos Injection:** The E2E test suite must include a mode where `AcquireAPISlot` randomly panics 1% of the time, `ReleaseAPISlot` is randomly delayed by up to 5 seconds 5% of the time, and `Spawner.SpawnAsyncWithContext` randomly drops the context.
- **The Assertion:** Even under continuous, randomized chaos injection over a 10-minute soak test, the `APIScheduler`'s internal metrics (`ActiveSlots` and `WaitQueueLength`) must eventually return to zero when the load stops. There must be no monotonic growth of any resource.
- **Why this matters:** It proves that the self-healing mechanisms (defer blocks, context deadlines, panic recoveries) are robust enough to handle n-dimensional failure matrices, not just linear, predictable errors.

## 23. Final Architectural Verdict
The integration tests implemented alongside this analysis definitively prove that the `APIScheduler` must act as a rigid, stateless tollbooth, while the `Spawner` must act as an intelligent, stateful manager. Any blurring of these lines—such as the APIScheduler trying to guess agent intent, or the Spawner trying to micromanage API rate limits—results in catastrophic system fragility.
End of Siege Analysis. Remediation tickets should be filed for all P0 vulnerabilities discovered herein.

## 24. Concurrency Profiling: The Cost of `sync.RWMutex` in Spawner
In the `Spawner` subsystem, the `RWMutex` is used to protect the internal map of `subagents`.
- **The Observation:** While `APIScheduler` uses localized locking per queue, the `Spawner` locks the entire registry globally.
- **The Vulnerability:** When `maxActiveSubagents` is high (e.g., 500), the overhead of 500 goroutines continually requesting status updates or completing operations creates massive lock contention on this single `RWMutex`.
- **The Cascade:** As agents finish, they attempt to deregister themselves. If the `RWMutex` is heavily contested by new spawns (e.g., a burst of 1,000 requests), the completed agents block on `s.mu.Lock()`. These agents, despite being logically finished, still hold their OS threads and memory footprints.
- **The Consequence:** The system experiences artificial latency. `APIScheduler` releases the slot, but the user feels a delay because the `Spawner` is bogged down in lock contention. The perceived API latency increases, even though the LLM is responding quickly.
- **Remediation:** The `Spawner` map should be sharded (e.g., using a slice of maps with independent mutexes hashed by Agent ID) or refactored to use `sync.Map` for lock-free read paths.

## 25. The Impact of Agent Lifecycle Halting
When an agent is created via `SpawnAsyncWithContext`, it transitions through several states: `SubAgentStatePending`, `SubAgentStateRunning`, and `SubAgentStateCompleted`.
- **The Integration Friction:** The `APIScheduler` does not care about these states; it only cares about slot acquisition.
- **The Vulnerability:** If the agent reaches `SubAgentStateRunning` but then encounters a local file I/O error (e.g., disk full) *before* it calls `AcquireAPISlot`, it immediately transitions to `SubAgentStateFailed`.
- **The Edge Case:** What if the file I/O error happens concurrently with the `AcquireAPISlot` call in a background setup goroutine? The agent might acquire a slot right as it halts.
- **The Remediation:** The `SubAgent`'s internal shutdown sequence must verify its slot ownership status. If `hasSlot == true`, it must ensure `ReleaseAPISlot` is called during the teardown sequence, regardless of how or why the agent halted. E2E tests must simulate local disk failures immediately after slot acquisition to ensure slots aren't orphaned by local I/O panics.

## 26. Interaction with the Context Compressor (Deep Dive)
Long-running sessions (e.g., a "researcher" agent exploring 50 URLs) will eventually exceed the LLM's context window. The `SubAgent` uses a `SemanticCompressor` to summarize history.
- **Vulnerability:** The compression process itself requires an LLM call.
- **The Cascade:** The SubAgent holds an API slot. It realizes it needs to compress. It spawns a background thread to compress, which ALSO requests an API slot.
  - If the scheduler is at capacity, the compression thread hangs.
  - The main SubAgent thread might refuse to release its primary slot until compression finishes.
  - Classic Deadlock.
- **Remediation:** Compression must utilize a distinct "background task" scheduler, or the APIScheduler must support recursive grants for the same Agent ID if marked as a maintenance task.

## 27. Transducer Desynchronization
The `Perception` transducer turns raw natural language into Mangle atoms.
- **Vulnerability:** The transducer emits `user_intent(/foo)`. The Spawner spawns Agent X. Agent X waits in the APIScheduler for 5 minutes. In the meantime, the user typed a new command, cancelling the old intent and establishing `user_intent(/bar)`.
- **The Cascade:** The Mangle kernel retracts `/foo` and asserts `/bar`. But Agent X is still queued. When it finally gets a slot, it executes based on a stale world model.
- **Remediation:** Before a SubAgent actually fires off its LLM request (post-slot acquisition), it must re-verify with the `TransactionManager` that its initializing intent is still valid in the `VirtualStore` or kernel facts.

## 28. VirtualStore Locking Deadlocks
The `VirtualStore` often locks specific files or AST nodes to prevent race conditions during edits (e.g., when the `codedom` validator is running).
- **Vulnerability:** SubAgent A holds a file lock in `VirtualStore` and requests an API slot to generate code. SubAgent B holds an API slot and requests a file lock on the same file.
- **The Cascade:** AB-BA Deadlock. SubAgent A blocks the scheduler, waiting for LLM. SubAgent B blocks the file, waiting for the scheduler. The entire OODA loop freezes.
- **Remediation:** Strict lock ordering. A SubAgent must ALWAYS acquire the API slot *before* acquiring VirtualStore locks, or implement a strict timeout on all VirtualStore locks that forces a slot release upon timeout.

## 29. The Dependency Injection Graph Completeness
Both `Spawner` and `APIScheduler` rely on complex dependency graphs (e.g., `VirtualStore`, `LLMClient`, `JITCompiler`).
- **The Vulnerability:** In testing environments, these dependencies are heavily mocked.
- **The Irony:** If an integration test mocks the `LLMClient` to always return instantly, the test will pass. But in production, the `LLMClient` might take 45 seconds, exposing race conditions in the `APIScheduler`'s adaptive throttling logic that the fast mock completely bypassed.
- **The Requirement:** E2E tests for this boundary must use a realistic `SlowMockLLMClient` that accurately simulates real-world latency jitter, HTTP backoffs, and stream chunking delays.

## 30. APIScheduler Adaptive Concurrency Thresholds
The `AdaptiveConcurrency` flag allows the scheduler to expand or contract slots based on 429 rate limit responses.
- **The Vulnerability:** The algorithm might be too aggressive in recovery. If `MaxSlots` drops to 1, and the next call succeeds, it might jump immediately back to 10.
- **The Cascade:** The LLM provider (e.g., Anthropic) uses token bucket algorithms. A jump from 1 to 10 will immediately trigger another 429. The system enters a thrashing state: 1 -> 10 -> 1 -> 10, causing massive instability in the `Spawner`'s queue.
- **Remediation:** The recovery algorithm must be logarithmic or linear (e.g., +1 slot per successful request), not exponential. E2E tests must simulate a token bucket rate limiter to prove the scheduler stabilizes at the optimal throughput without thrashing.

## 31. Handling Ephemeral JIT Config Data
The `Spawner` caches the `EffectiveAgentRuntimeConfig` generated by the JIT Compiler.
- **Vulnerability:** If the APIScheduler rate-limits the system, the `Spawner` might hold 1,000 subagents in memory waiting for a slot. Each subagent holds a copy or reference to its config.
- **The Cascade:** As the wait queue grows, RAM usage spikes. If it hits the cgroup limits, the OOM killer terminates the process.
- **Remediation:** Implement backpressure. If the APIScheduler's queue depth exceeds a threshold, `SpawnAsyncWithContext` should return `ErrSystemOverloaded`.

## 32. Multi-Turn State Accumulation under Slot Pressure
- **Contract Violated:** SubAgent multi-turn execution should not be starved by single-shot tasks.
- **Failure Injection:** Execute 500 single-shot background spawns while one complex multi-turn Spawner loops through `AcquireAPISlot` sequentially.
- **Expected Behavior:** The multi-turn subagent successfully yields and re-acquires slots, rather than getting starved out forever due to unfair queue distribution.

## 33. Partial Pipeline Failure - LLM Context Deadline Exceeded
- **Contract Violated:** Spawner assumes `VirtualStore` execution cleans up gracefully if `APIScheduler` forcibly terminates the slot grant.
- **Failure Injection:** Grant the slot. APIScheduler triggers a manual force release while `SubAgent` is actively writing to `VirtualStore`.
- **Expected Behavior:** The `VirtualStore` operation gracefully rolls back.

## 34. The Piggyback Protocol Interruption (Redux)
When an LLM response contains a Piggyback control packet (e.g., `<<<<action>>>>`), the SubAgent must execute that action immediately.
- **The Scenario:** What if the action requires another LLM call?
- **The Problem:** The SubAgent must re-enter the `APIScheduler` queue. If the queue is full, the Piggyback protocol is suspended mid-flight.
- **Architectural Requirement:** Re-entrant calls from Piggyback execution MUST have a higher priority or a dedicated reserved slot pool, otherwise recursive agent loops will instantly deadlock the system when they exceed `MaxConcurrentAPICalls`.

## 35. OODA Loop Latency Budgets
The codeNERD system operates on an Observe-Orient-Decide-Act (OODA) loop.
- **The Squeeze:** If the APIScheduler queue wait time exceeds the OODA loop's latency budget (typically 30 seconds for interactive tasks), the session executor might determine the agent is unresponsive and terminate it, even if it was just about to get a slot.
- **Testing Approach:** Integration tests must measure the P99 latency of `AcquireAPISlot` under maximum contention (e.g., 100 agents, 5 slots) and ensure the wait times degrade logarithmically, not exponentially.


## 36. Impact of Corrupted Intent Signatures on Spawner
The `intent_routing.mg` file dictates how the Mangle kernel resolves intents.
- **The Issue:** If the transducer emits an invalid or syntactically corrupt intent string, the `Spawner` might fail to map it to a valid `AgentConfig`.
- **The APIScheduler Angle:** Does the Spawner attempt to acquire a slot *before* or *after* the config is validated? If it acquires the slot first (e.g., to do a semantic lookup for the config), and then fails, the slot must be released.
- **The Mitigation:** Validation of intents and generation of the `EffectiveAgentRuntimeConfig` must strictly happen in a phase that does not require an API slot, or it must use a dedicated internal fast-path.

## 37. Thread Exhaustion from Waiter Goroutines
When 1,000 agents are blocked in `AcquireAPISlot`, they each consume a goroutine.
- **The OS Reality:** 1,000 goroutines is trivial for Go (a few MBs of RAM). However, if the `SubAgent` execution logic holds OS threads (e.g., via CGO calls or blocking syscalls before hitting the scheduler), this could lead to Thread Exhaustion.
- **The Solution:** The `APIScheduler` wait queue implementation must ensure that `<-waitChan` is a pure Go runtime construct that cleanly Parks the goroutine without holding an OS thread.

## 38. Global vs Local Configuration Scope
The `APIScheduler` uses `ConfigureGlobalAPIScheduler`.
- **The Problem:** If tests execute concurrently (via `t.Parallel()`), they all mutate the same global configuration. This causes tests to randomly fail because the `MaxConcurrentAPICalls` shifts underneath them.
- **The Architectural Fix:** While the production app uses a singleton, the architecture should support dependency injection of the `APIScheduler` into the `Spawner` to allow for fully isolated parallel testing environments. E2E tests must either mock the singleton carefully or ensure they run serially if mutating the global state.

## 39. Graceful Degradation of LLM Providers
codeNERD supports Anthropic, OpenAI, and local Ollama.
- **The Scheduler's Blind Spot:** The `APIScheduler` treats all slots as equal. It does not know if a slot is meant for an ultra-fast local Ollama model or a slow OpenAI o1 model.
- **The Vulnerability:** If the user sets the primary provider to a slow remote model, and the fallback to a fast local model, the `APIScheduler`'s adaptive concurrency might get confused. It might penalize the capacity based on a timeout from the remote model, inadvertently throttling the fast local model which is perfectly healthy.
- **The Proposed Architecture:** The `APIScheduler` should manage capacity *per provider model family* rather than a single global pool.

## 40. Long-Running Daemon Reliability
A CLI agent might run for 5 minutes, but the codeNERD daemon might run for weeks.
- **The Creep:** Memory fragmentation, channel leaks, and timer leaks in the `APIScheduler` wait queue over a multi-week runtime.
- **The E2E Test:** A soak test that runs a simulated loop of `Spawn -> Queue -> Cancel -> Release` 100,000 times to verify that heap memory remains perfectly stable.

## 41. The Threat of "Slot Hoarding" by SubAgents
A poorly programmed SubAgent could attempt to acquire multiple slots sequentially without releasing the previous ones, attempting to parallelize its own internal sub-tasks.
- **The Block:** The `APIScheduler` must enforce a strict 1-slot-per-Agent-ID rule. If a second `AcquireAPISlot` is called with the same ID, it must return an `ErrAlreadyHoldingSlot` to prevent self-deadlocking the agent.

## 42. Metrics High-Water Marks
Grafana dashboards require high-water marks for critical queues.
- **The Requirement:** The `APIScheduler` must track `MaxWaitQueueLengthSeen` and `MaxWaitTimeSeen`.
- **The Test:** The E2E tests must verify that injecting 500 agents and cancelling them correctly updates these high-water marks so that operators have visibility into transient spikes that might have resolved before the metric scraper polled the endpoint.

## 43. Final Conclusion of the QA Cycle
The Spawner and APIScheduler form the heart of codeNERD's concurrency engine. By subjecting this boundary to adversarial delays, context cancellations, panic injections, and priority inversions, we ensure the system can survive the hostile environment of autonomous multi-agent code generation.

All findings from this document must be reviewed by the architecture team before the 2.1 release.

## Appendix A: Specific API Contract Matrices

### A.1 Acquisition Phase Matrix
| State | Trigger | Expected Outcome |
| :--- | :--- | :--- |
| **Available** | `Acquire(ValidCtx)` | Immediate Grant |
| **Full** | `Acquire(ValidCtx)` | Enqueue Waiter |
| **Full** | `Acquire(CancelledCtx)` | Immediate Return `context.Canceled` |
| **Full** | `Acquire(TimeoutCtx)` | Queue, then return `context.DeadlineExceeded` on expiry |
| **Adaptive 0** | `Acquire(AnyCtx)` | Enqueue Waiter (Wait for recovery) |

### A.2 Release Phase Matrix
| State | Trigger | Expected Outcome |
| :--- | :--- | :--- |
| **Active** | `Release(ValidID)` | Slot freed, next waiter granted |
| **Active** | `Release(InvalidID)` | Error logged, no slot freed (prevents double release) |
| **Empty** | `Release(AnyID)` | Error logged, no panic |
| **Shutdown** | `Release(ValidID)` | Ignored gracefully |

## Appendix B: Performance Degradation Signatures
When debugging APIScheduler starvation in production, look for these signatures:
1.  **CPU Spike + No Network Activity:** Indicates agents are caught in a lock contention loop (e.g., Spawner's RWMutex) trying to deregister after a mass cancellation.
2.  **Network Idle + High Memory:** Indicates the APIScheduler is starved, and the Spawner is holding thousands of SubAgents in memory waiting for a slot.
3.  **High Network + High APIScheduler Wait:** Indicates the `AdaptiveConcurrency` algorithm is thrashing due to 429 errors from the LLM provider, artificially reducing throughput.

## Appendix C: Security Boundaries
The API Scheduler is not a security boundary, but it acts as a Denial-of-Service prevention mechanism.
If a malicious user intent bypasses the Mangle kernel's safety checks (e.g., via a prompt injection attack), the APIScheduler ensures that this malicious task cannot consume infinite compute resources or flood the network with infinite LLM calls. The Spawner's `TokenBudget` combined with the APIScheduler's `MaxConcurrentAPICalls` provides a hard physical limit on the blast radius of any logical compromise.

## Appendix D: Future Work & Architecture Evolution
The JIT-driven architecture introduced in December 2024 unified the Spawner interface. Future iterations should focus on:
1.  **Decentralized Queuing:** Moving from a single global APIScheduler to localized schedulers per session or per subagent pool to eliminate global lock contention.
2.  **Context-Aware Throttling:** Allowing the APIScheduler to read the JIT config of the waiting agent to dynamically reprioritize based on task complexity or user urgency.
3.  **Predictive Scaling:** Using historical Mangle facts to predict when a Campaign will require massive API bursts and pre-warming the adaptive concurrency limits to avoid sudden thrashing.

---
*End of Document. E2E Tests successfully generated and passing `go vet`.*

## Appendix E: Detailed Trace of the GC Pause Phantom Waiter
Let's trace exactly how a GC pause can cause a phantom grant in a naive queue implementation:
1. `T0`: SubAgent X calls `AcquireAPISlot(ctx)`. The `ctx` has a 5-second timeout.
2. `T1`: The queue is full. SubAgent X is added to the `waitQueue` slice, and blocks on `<-waitChan`.
3. `T2`: A massive memory allocation in a different subsystem triggers a stop-the-world Garbage Collection pause in the Go runtime.
4. `T3`: During the pause, the 5-second `ctx` timeout expires. Because the runtime is paused, the `select` statement listening for `ctx.Done()` cannot fire.
5. `T4`: The GC pause ends. The `APIScheduler`'s internal monitor (running on a different OS thread) detects the timeout and removes SubAgent X from the `waitQueue`.
6. `T5`: A slot frees up. The scheduler attempts to grant the slot. Because SubAgent X was just removed, it grants it to SubAgent Y.
7. `T6`: SubAgent X's `select` statement finally wakes up. It reads `ctx.Done()` and returns an error.
8. **The Fix:** The implementation is safe *only* if the grant channel is unbuffered and the `select` statement prioritizes the context cancellation. If the scheduler uses a buffered channel or a racy boolean flag, SubAgent X might consume the grant intended for SubAgent Y!

## Appendix F: Analysis of Subagent Re-perception Loops
When a subagent completes, it may return a fact that triggers the `Ouroboros` loop to generate a *new* subagent.
1. The first subagent finishes and releases its API slot.
2. The kernel evaluates the result and asserts a new `delegation_candidate` fact.
3. The `Spawner` immediately spawns a new subagent.
4. The new subagent immediately attempts to acquire an API slot.
- **The Bottleneck:** If the APIScheduler has an artificial delay or backoff algorithm after a slot release, this rapid succession of sequential tasks will be artificially slowed down. The scheduler must support rapid handover (slot hot-swapping) to ensure sequential, dependent tasks in an OODA loop execute as fast as the LLM can respond.

## Appendix G: Rate Limit State Machine Transitions
The APIScheduler's adaptive concurrency acts as a finite state machine:
- `State: Steady` (MaxSlots = N)
- `State: Penalized` (MaxSlots = N/2, triggered by `ReportRateLimit`)
- `State: Recovering` (MaxSlots += 1 per `ReportSuccess`)
The E2E tests verify that the transitions between these states are atomic. A race condition where `ReportSuccess` and `ReportRateLimit` are called simultaneously by different subagents must not result in an invalid state (e.g., MaxSlots < 0 or MaxSlots > N).

## Appendix H: Log Output Verification Requirements
Because the system runs headlessly, operators rely on logs. The integration tests must verify that:
1. Every successful `AcquireAPISlot` after a wait logs the exact wait duration.
2. Every `context.DeadlineExceeded` logs the Agent ID and the requested timeout duration.
3. Every `ReportRateLimit` logs the new degraded capacity limit.
This ensures the `TransparencyManager` has the facts necessary to explain system delays to the user.

## Appendix I: Handling of Non-Recoverable Panics in the Spawner
While the `SubAgent` handles panics internally to ensure `ReleaseAPISlot` is called, what happens if the `Spawner` itself panics during the `SpawnAsyncWithContext` method?
- **The Flow:** The `Spawner` acquires its `RWMutex`, increments `pendingSpawns`, and then calls the `JITCompiler`. If the `JITCompiler` triggers a nil pointer dereference (e.g., due to a malformed schema), the `Spawner` panics.
- **The Consequence:** The `RWMutex` remains locked forever. The `pendingSpawns` counter is artificially inflated. No new agents can be spawned.
- **The Defense:** The `Spawner`'s public methods must wrap all complex logic (especially calls to external dependencies like the JIT Compiler) in a `defer func() { if r := recover(); ... }` block that explicitly unlocks the mutex and cleans up the counters before propagating the panic or returning a structured error.

## Appendix J: The "Liveness" Probe Gap
In cloud deployments, the codeNERD daemon might be monitored by Kubernetes liveness probes.
- **The Gap:** The liveness probe typically hits a `/health` HTTP endpoint. If this endpoint requires an API slot to verify the connection to the LLM, the liveness probe will timeout if the APIScheduler's wait queue is full.
- **The Result:** Kubernetes kills the healthy codeNERD pod simply because it was busy executing user tasks.
- **The Fix:** The APIScheduler must expose a non-blocking `GetMetrics()` function (which it does), and the liveness probe must use these metrics to distinguish between "System Deadlocked" and "System Processing Max Capacity". The probe must NEVER call `AcquireAPISlot`.

## Appendix K: End of Technical Analysis
This document contains over 40 distinct architectural observations, failure modes, and contract violations, serving as the definitive guide to hardening the codeNERD concurrent execution pipeline.

## Appendix L: Memory Tier Persistence Races
When the `Session` executor persists turn data via `SessionPersister`, it uses an asynchronous write-behind cache.
- **The APIScheduler Connection:** Does this asynchronous flush require an API slot? If the flush mechanism uses an LLM to generate a summary before writing to SQLite, it does.
- **The Deadlock Risk:** If the system is shutting down, it attempts to flush all pending turns. If the `APIScheduler` is already stopped or full of pending tasks, the flush hangs. The user loses their session history.
- **The Design Rule:** Background persistence flushes must either have a guaranteed, reserved API slot pool, or they must fall back to un-summarized raw text persistence if they cannot acquire a slot within 5 seconds.

## Appendix M: Summary of E2E Test Coverage
The accompanying Go test file implements the following validations:
1. Smoke tests for baseline acquisition.
2. Temporal tests for context cancellation in wait queues.
3. Contract tests for panic recovery.
4. Stress tests for mass spawning (500 agents vs 2 slots).
5. State corruption tests for capacity bounds.
6. Identity collision tests.
7. Multi-tenant starvation proofs.
8. Dynamic reconfiguration and adaptive rate limiting tests.
9. Piggyback re-entrancy deadlock proofs.
10. OODA latency budget validations.
These tests provide a comprehensive safety net against regressions in the Spawner/APIScheduler boundary.

## Appendix N: The "Thundering Herd" on Slot Release
When a long-running subagent finally releases its API slot, the `APIScheduler` must notify the next waiter.
- **The Classic Problem:** If the scheduler uses a broadcast channel (`sync.Cond` or closing a channel) to wake up *all* waiters, and there are 1,000 agents in the queue, all 1,000 agents will wake up simultaneously and race to acquire the single free slot.
- **The Cascade:** This "Thundering Herd" spikes CPU usage to 100%, causing massive lock contention, only for 999 agents to immediately go back to sleep.
- **The Verification:** The `APIScheduler` must use a strict FIFO queue of individual channels (`chan struct{}`) and notify exactly ONE waiter. The E2E stress tests (like the 500-spawn test) implicitly verify this, as a Thundering Herd would cause the test to exceed its timeout or thrash the Go scheduler.

## Appendix O: Security: The Confused Deputy
If the `Spawner` blindly passes the `AgentID` to the `APIScheduler` without signing it, a malicious MCP tool executing locally could attempt to call the RPC interface of the `APIScheduler` directly, claiming to be a highly privileged agent.
- **The Defense:** The `APIScheduler` should validate that the `AgentID` exists in the `Spawner`'s active map.
