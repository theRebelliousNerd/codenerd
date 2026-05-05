# Write Set Lock Manager Boundary Analysis
*Date:* 2026-03-10 04:18:30 EST

## 1. System Overview & Architecture Context

The `write_set_lock_manager.go` subsystem provides deterministic, file-level mutual exclusion for the campaign orchestrator within codeNERD. In a concurrent execution environment (the "Clean Execution Loop"), multiple JIT-spawned agents might attempt to modify the same source file simultaneously. The `writeSetLockManager` coordinates this by mapping normalized absolute paths to the task ID that currently holds the lock lease.

It is designed to ensure that:
1. Deadlocks are prevented by sorting paths lexicographically before attempting acquisition.
2. A lease requires a heartbeat context, releasing safely if the underlying task times out.
3. Case-sensitivity semantics on Windows are normalized.

### 1.1 The Mutex Bottleneck
The system relies on a central `sync.Mutex` (`m.mu`) that guards a simple `map[string]string` (`owners`). The locking strategy is coarse-grained: to acquire locks for a set of files, the system locks the global mutex, iterates over all requested paths to check availability, and if all are free, assigns the owner to all of them before releasing the mutex. This implies O(N) time complexity *under lock* per acquisition request, where N is the size of the write set.

## 2. Missing Edge Cases Identified

### Vector 1: Null/Undefined/Empty Strings and Slices

#### 1.1: `nil` Context Fallback Behavior
The implementation explicitly falls back to `context.Background()` if `ctx == nil`:
```go
if ctx == nil {
    ctx = context.Background()
}
```
**Test Gap:** The test suite does not explicitly test that calling `acquire` with a `nil` context succeeds. Furthermore, falling back to `context.Background()` completely nullifies the timeout protection mechanisms inherent in the `select { case <-ctx.Done(): ... }` block at the bottom of the acquire loop. If a task passes `nil` and the files are permanently locked by a crashed task, this loop will block indefinitely (a goroutine leak). The tests must simulate `acquire(nil, ...)` and verify the risk profile of this indefinite block, or the code should enforce a hard maximum deadline if `nil` is passed.

#### 1.2: `nil` or Empty `writeSet` Slice
If `writeSet` is `nil` or `[]string{}`, the `normalizeWriteSetPaths` function correctly returns `nil`. Subsequently, `acquire` returns `nil, nil`.
**Test Gap:** The tests do not explicitly verify that an empty request correctly returns a `nil` lease and no error without attempting to lock any underlying maps. This is an important zero-value boundary. If a JIT agent decides it needs to execute a tool that doesn't mutate files (a read-only action) but the orchestrator statically wraps the action in an `acquire` call, this empty boundary must be provably safe and non-blocking.

#### 1.3: Empty Strings Inside `writeSet`
If `writeSet` contains `[]string{"", "  ", "\t"}`, the normalization loop relies on `strings.TrimSpace(rawPath)` and drops empty strings:
```go
if path == "" {
    continue
}
```
**Test Gap:** The test suite does not include a test case passing slice elements containing pure whitespace. We must verify that `[]string{"a.go", " "}` successfully acquires `a.go` and ignores the whitespace string, rather than returning an error or asserting a lock against an empty string key in the map.

#### 1.4: Empty `taskID` Validation
The `acquire` method checks `if taskID == ""` and returns an error:
```go
if taskID == "" {
    return nil, fmt.Errorf("write_set lock acquisition requires non-empty task id")
}
```
**Test Gap:** There is no explicit negative test asserting this validation logic in the test suite. An empty task ID could easily result from an uninitialized task struct in the `Orchestrator` package. If this check were ever accidentally removed, `owners[p] = ""` would represent an active lock held by "nobody", making debug tracing impossible. A strict unit test enforcing this rejection is required.

#### 1.5: `nil` Lease Release Idempotency
The `writeSetLockLease.release()` function contains defensive checks:
```go
if l == nil || l.manager == nil {
    return
}
```
**Test Gap:** There is no test explicitly calling `(*writeSetLockLease)(nil).release()`. In Go, method calls on nil pointers are valid but often panic if fields are accessed. Because of the defensive `l == nil` check, this is safe, but it must be unit-tested to prevent regression. If a task fails to acquire a lease, it might defer a call to `lease.release()` where `lease` is nil.

### Vector 2: Type Coercion and Normalization Boundaries

#### 2.1: OS Case Sensitivity (`runtime.GOOS == "windows"`)
The `normalizeAbsolutePath` function forces `strings.ToLower(normalized)` only if `runtime.GOOS == "windows"`.
**Test Gap:** The test suite lacks OS-specific boundary tests. On Windows, acquiring `File.go` and `file.go` concurrently must block each other because the OS treats them as the same file. On Linux, they must map to distinct locks. The current tests don't simulate or mock the `runtime.GOOS` constraint to ensure case-insensitive file systems don't experience TOCTOU or dual-locking race conditions. We need a way to inject or mock `runtime.GOOS` during tests to prove that `File.go` and `file.go` collide correctly on simulated Windows environments.

#### 2.2: Extremely Deep Nested Paths (Path Traversal Escapes)
The path normalizer relies on `filepath.Abs` and `filepath.Clean`.
**Test Gap:** Negative testing should inject paths like `a/b/c/../../b/c/d/../../../../escape.go` containing excessive dot-dot directories. While `TestNormalizeWriteSetPaths_RejectsOutsideWorkspace` tests a basic `../escape.go`, it does not stress the path resolver with pathological depth (e.g., 500 levels deep). This could trigger CPU spikes or expose buffer limits in `filepath` during normalization, allowing an adversarial sub-agent to mount a denial-of-service attack on the locking manager.

#### 2.3: Unicode Normalization Vulnerabilities
The manager uses strict string equality for path mapping: `m.owners[p] = taskID`. Go strings are immutable byte slices (UTF-8).
**Test Gap:** What happens if a file is named with combining Unicode characters (e.g., the letter `e` + an acute accent `´` vs the precomposed character `é`)? If the workspace contains visually identical but byte-differing un-normalized Unicode paths, the system will lock them as distinct entries. If two tasks request the same file using different normalization forms, the `writeSetLockManager` will grant both locks concurrently, leading to race conditions in the underlying file system. A boundary test mapping normalized Unicode forms (NFC/NFD) is missing, and `golang.org/x/text/unicode/norm` should likely be applied during `normalizeAbsolutePath`.

#### 2.4: Path Separator Coercion
The normalizer uses `filepath.ToSlash(filepath.Clean(abs))`.
**Test Gap:** Does it handle mixed delimiters on all platforms? E.g., `pkg\dir/file.go`. The test `TestNormalizeWriteSetPaths_SortsAndDedupes` tests `pkg\\other.go`, but testing boundary mixed-slashes across simulated OS environments is sparse.

### Vector 3: User Request Extremes

#### 3.1: Massive Write Sets (The "Brownfield Monorepo" Vector)
If a user requests a massive campaign on a 50 million line monorepo, a generated task might declare a `write_set` spanning 100,000 files (e.g., a global rename or formatting pass).
**Analysis:**
The normalizer loop allocates `normalized := make(map[string]struct{}, len(writeSet))`, inserts elements, and then sorts them: `sort.Strings(out)`. Sorting 100,000 strings takes several milliseconds. This is done *outside* the global mutex, which is architecturally sound.
However, `tryAcquirePaths` iterates over the 100,000 strings *inside* the global mutex (`m.mu.Lock()`):
```go
for _, p := range paths {
    owner, held := m.owners[p]
    if held && owner != taskID {
        return false
    }
}
for _, p := range paths {
    m.owners[p] = taskID
}
```
**Test Gap:** A performance boundary test simulating 10,000 to 100,000 paths in a single `acquire` call is missing. This test would likely reveal that holding `m.mu` for O(N) map lookups and O(N) map insertions blocks the entire campaign orchestrator for dozens of milliseconds. Any other concurrent JIT task attempting to lock a single unrelated file (e.g., `README.md`) will stall completely. This is a severe scalability boundary.

#### 3.2: Path Length Maximums (OS Max Path boundaries)
If a generated task outputs a path exceeding 255 bytes (Linux) or 260 characters (Windows `MAX_PATH`), `filepath.Abs` or subsequent OS interactions may behave unexpectedly.
**Test Gap:** The system needs negative tests ensuring ultra-long paths (e.g., 4096 bytes) are either rejected gracefully by `normalizeAbsolutePath` or handled strictly. Currently, `filepath.Clean` might accept an arbitrarily long string, polluting the `owners` map with massive strings and consuming excessive memory if generated in a loop by an errant Ouroboros sub-agent.

#### 3.3: High Frequency Polling Contention
`acquire` loops based on `pollInterval` (default 10ms) via a `time.Ticker`. If 500 tasks are concurrently waiting for overlapping locks, they will all wake up every 10ms, grab the global mutex, check their arrays, and release.
**Test Gap:** This creates a "thundering herd" problem. The test `TestWriteSetLockManager_ConcurrentMutualExclusion` tests 20 goroutines over 3 seconds, which masks the issue. A boundary test spanning 1000 goroutines acquiring deeply overlapping sets (A+B, B+C, C+A) will likely show significant CPU starvation due to mutex contention.

### Vector 4: State Conflicts and Race Conditions

#### 4.1: Task ID Re-entrancy (Idempotency of `tryAcquirePaths`)
`tryAcquirePaths` contains a subtle state handling detail:
```go
owner, held := m.owners[p]
if held && owner != taskID {
    return false
}
```
This logic explicitly implies that if `owner == taskID`, it considers the path unblocked. The task re-acquires its own lock. This makes the lock *re-entrant* at the task level.
**Test Gap:** The test suite does not explicitly test a single task ID attempting to call `acquire` twice on the same file concurrently or sequentially.
The critical failure mode: if Task 1 calls `acquire("A")` (returns Lease X) and then calls `acquire("A")` again (returns Lease Y), both succeed. But if Lease X calls `release()`, it executes:
```go
if owner, held := m.owners[p]; held && owner == taskID {
    delete(m.owners, p)
}
```
This deletes the lock entirely. Now, Lease Y thinks it holds a lock, but the global manager considers "A" unlocked. Task 2 can now acquire "A", violating mutual exclusion. This is a massive state conflict race condition.

#### 4.2: Double Release Indempotency
The `release()` function uses `sync.Once` to guarantee idempotency.
```go
l.once.Do(func() {
    l.manager.releasePaths(l.taskID, l.paths)
})
```
**Test Gap:** A test must verify that calling `release()` multiple times on the same lease doesn't panic. The current unit tests implicitly rely on `defer lease.release()` but don't explicitly hammer the `release()` function from multiple goroutines simultaneously to verify the strict `sync.Once` guarantee.

#### 4.3: Workspace Path Prefix Spoofing
`normalizeAbsolutePath` verifies the path is inside the workspace:
```go
if workspace != "" && !isPathWithinWorkspace(workspace, normalized) {
    return ""
}
```
**Test Gap:** The `isPathWithinWorkspace` utility is not visible in this file, but boundary testing must verify that a workspace of `/app/repo` correctly rejects `/app/repo_secret/file.go`. If `strings.HasPrefix` is used naively without path separator checks, a state conflict allows tasks to escape the intended sandbox and lock files in adjacent directories sharing a string prefix.

## 3. Performance Viability Assessment

**Is the system performant enough?**

The viability depends entirely on the campaign scale defined by the user request.

### Small to Medium Campaigns (The Happy Path)
For standard codebase refactoring (1-50 files, 1-10 concurrent JIT tasks), this implementation is **highly performant**.
- The `owners` map uses `O(1)` lookups.
- Path normalization sorts `O(N log N)` where `N` < 50, resulting in sub-millisecond overhead.
- Memory overhead is bounded: 50 strings stored in a map per active lock is less than 5KB RAM. It perfectly meets the constraint of operating efficiently on an 8GB laptop.

### Massive Monorepo Campaigns (The Edge Case)
For extreme scale (100,000 file write_sets, 500 concurrent sub-agents), the system will **fail to scale** due to the Global Mutex Bottleneck.
- **CPU Starvation:** Holding `m.mu` for 100,000 sequential map lookups and writes takes measurable milliseconds. 500 agents polling every 10ms means the mutex queue will infinitely back up. The system will spend >90% of its CPU time context-switching and waiting on `m.mu` rather than executing LLM tasks.
- **Thundering Herd:** The 10ms polling interval forces continuous wakeups.
- **Recommendation for Performance:** To survive this boundary, the `owners` map must be sharded (e.g., `sync.Map` or a slice of 32 mutex-guarded maps hashed by file path), and the polling mechanism should be replaced with a `sync.Cond` or channel-based notification system to wake tasks *only* when a file they need is released, rather than blind 10ms polling.

## 4. Recommended Fixes

1. **Fix Re-entrancy Bug:** Modify `tryAcquirePaths` to implement reference counting per task, or reject re-entrant lock requests outright (`if held && owner == taskID { return false }`).
2. **Handle Unicode Normalization:** Integrate `golang.org/x/text/unicode/norm.NFC.String(path)` before checking against the `owners` map to prevent un-normalized bypasses.
3. **Thundering Herd Mitigation:** Implement a `sync.Cond` to broadcast state changes instead of `time.NewTicker(pollInterval)`.
4. **Context Deadline Validation:** Add a circuit breaker to reject `acquire` if `ctx == nil` or if it lacks a deadline to prevent goroutine leaks.
5. **Add Comprehensive Table-Driven Tests:** Cover all missing gaps identified in section 2 (nulls, empty strings, extremely long paths).

## 5. Extensive Concurrency Modeling

To understand the boundaries of the `write_set_lock_manager`, we must model its behavior under extreme load, characteristic of a high-throughput, multi-agent AI system operating on a massive codebase. The system employs a coarse-grained `sync.Mutex` and a polling loop to coordinate access to a shared resource (the `owners` map). This model is susceptible to several failure modes that become apparent only when boundary values are pushed.

### 5.1 The Polling Loop and Context Contention

The core acquisition logic relies on a tight loop governed by a `time.Ticker`:

```go
ticker := time.NewTicker(pollInterval)
defer ticker.Stop()

for {
    if ok := m.tryAcquirePaths(taskID, paths); ok {
        return ...
    }
    select {
    case <-ctx.Done():
        return ...
    case <-ticker.C:
    }
}
```

**Boundary Case:** What happens when `pollInterval` is exceedingly small (e.g., 1 nanosecond) or the system is under extreme load with many goroutines?
*   **Thundering Herd:** If 1000 tasks are waiting for locks, and `pollInterval` is 10ms, every 10ms, 1000 goroutines wake up and attempt to acquire the global mutex `m.mu`.
*   **Mutex Starvation:** Go's `sync.Mutex` in normal mode does not guarantee strict FIFO ordering. A newly arriving task might steal the lock from a task that has been waiting. While Go 1.9+ introduced starvation mode to mitigate this, the sheer volume of wakeups will cause significant CPU churn (context switching overhead).
*   **Context Cancellation Latency:** If `tryAcquirePaths` is slow (e.g., due to a massive `writeSet`), the `select` block checking `ctx.Done()` is delayed. A task might hold onto resources longer than its intended timeout because it's stuck waiting for the mutex to even *check* its status, reducing overall system responsiveness.

### 5.2 Lexicographical Sorting and Deadlock Prevention

Deadlocks are a primary concern when acquiring multiple locks. The classical solution, implemented here, is to impose a strict total order on the resources and acquire them in that order.

```go
sort.Strings(out) // In normalizeWriteSetPaths
```

By sorting the paths lexicographically, the system ensures that if Task A needs `[file1, file2]` and Task B needs `[file2, file1]`, they will both attempt to acquire `file1` first. The one that succeeds proceeds; the other waits.

**Boundary Case:** What if the paths are not perfectly normalized before sorting?
*   **Case Sensitivity (Revisited):** On Windows, `File.txt` and `file.txt` are the same file. If normalization fails to lowercase them, they might sort differently relative to other files (e.g., `A.txt`, `File.txt`, `file.txt`). If Task A requests `[A.txt, file.txt]` and Task B requests `[File.txt, Z.txt]`, the sorting order might not prevent a logical deadlock at the OS level if the system attempts to open the files concurrently, even if the manager grants the locks. The manager *must* guarantee canonical representation.
*   **Symlinks and Hardlinks:** If `a.go` is a symlink to `b.go`, they are the same physical resource. Lexicographical sorting of paths `["a.go"]` and `["b.go"]` does not recognize this equivalence. Task A locks `a.go`, Task B locks `b.go`. They both proceed and concurrently modify the same physical file, resulting in corruption.
    *   **Test Gap:** The system lacks boundary tests for file system links. `filepath.EvalSymlinks` should ideally be part of the normalization process to resolve paths to their physical canonical form before sorting and locking.

### 5.3 The Re-entrancy Vulnerability Deep Dive

The logic in `tryAcquirePaths` is fundamentally flawed regarding re-entrancy, representing a critical state conflict boundary.

```go
func (m *writeSetLockManager) tryAcquirePaths(taskID string, paths []string) bool {
    // ...
    for _, p := range paths {
        owner, held := m.owners[p]
        if held && owner != taskID {
            return false // Blocked by someone else
        }
    }
    // ... grant locks
}
```

**Scenario:**
1.  **Task T1** acquires lock on `file.go`. `m.owners["file.go"] = "T1"`. It receives `Lease L1`.
2.  **Task T1** (perhaps a sub-routine or a retried operation within the same context) attempts to acquire `file.go` again.
3.  `tryAcquirePaths` checks `m.owners["file.go"]`. `held` is true. `owner` ("T1") == `taskID` ("T1"). The condition `held && owner != taskID` is *false*.
4.  The loop completes. `tryAcquirePaths` grants the lock again. `m.owners["file.go"] = "T1"` (overwriting the same value). T1 receives `Lease L2`.
5.  **Task T1** finishes its first operation and calls `L1.release()`.
6.  `releasePaths` executes:
    ```go
    for _, p := range paths {
        if owner, held := m.owners[p]; held && owner == taskID {
            delete(m.owners, p)
        }
    }
    ```
7.  `m.owners["file.go"]` is deleted.
8.  **CRITICAL FAILURE:** `Lease L2` is still active and believes it holds the lock. However, the global manager considers `file.go` available.
9.  **Task T2** requests `file.go`. `tryAcquirePaths` sees it's available and grants it.
10. **Task T1** (via L2) and **Task T2** now concurrently modify `file.go`. Data corruption ensues.

**Resolution:** The manager must either strictly forbid re-entrancy (returning an error if a task tries to lock what it already owns, forcing proper task scope management), or implement a reference counter per lock per task (`map[string]*lockInfo` where `lockInfo` tracks the owning task and an integer count of active leases). Given the simplicity of the current design, forbidding re-entrancy is the safest boundary enforcement.

### 5.4 Path Normalization Boundaries

The `normalizeAbsolutePath` function acts as the gatekeeper for the `owners` map. Its behavior at boundaries defines the security and reliability of the locking mechanism.

```go
func normalizeAbsolutePath(workspace, rawPath string) string {
    // ...
    abs, err := filepath.Abs(path)
    if err != nil {
        abs = filepath.Clean(path)
    }
    normalized := filepath.ToSlash(filepath.Clean(abs))
    // ...
}
```

**Boundary Case:** Invalid Paths and `filepath.Abs` Failure
*   If `filepath.Abs` fails (e.g., due to an unresolvable working directory or path length issues), it falls back to `filepath.Clean(path)`.
*   If the input was a relative path (e.g., `../../secret.txt`), and `Abs` failed, the resulting `normalized` path might remain relative.
*   If `workspace` is set, the `isPathWithinWorkspace` check (assumed to exist) might fail to correctly evaluate a relative path against an absolute workspace path, potentially allowing an escape.
    *   **Test Gap:** Negative tests must inject environments where `filepath.Abs` intentionally fails (e.g., by manipulating `os.Getwd` or passing exceptionally malformed strings) to ensure the fallback logic securely rejects the path rather than granting an ambiguous lock.

**Boundary Case:** The "Clean" Illusion
*   `filepath.Clean` removes `.` and `..` elements syntactically. However, it does not evaluate the physical file system.
*   If a workspace contains a directory that is actually a symlink pointing outside the workspace, `filepath.Clean` will produce a path that *looks* like it's inside the workspace, bypassing `isPathWithinWorkspace`.
    *   **Test Gap:** This is a classic directory traversal vulnerability. The normalizer must be tested against symlink attacks to ensure tasks cannot lock (and subsequently modify) files outside their designated sandbox.

### 5.5 Conclusion on Performance and Scalability

The `write_set_lock_manager` is a pragmatic, simple implementation suited for the common case (tens of files, tens of tasks). It prioritizes correctness (via lexicographical sorting to prevent deadlocks) over extreme scalability.

However, the boundary analysis reveals that it is susceptible to performance degradation under heavy load (the thundering herd effect on the global mutex) and possesses critical state conflict vulnerabilities regarding task re-entrancy and symlink equivalence.

For codeNERD to operate reliably on massive codebases ("brownfield monorepos") with hundreds of concurrent JIT agents, this subsystem requires architectural reinforcement:
1.  **Replace Polling with Signaling:** Transition from `time.NewTicker` to `sync.Cond` or channel-based queues to eliminate CPU churn and provide strict FIFO fairness for waiting tasks.
2.  **Enforce Canonical Paths:** Integrate `filepath.EvalSymlinks` and robust Unicode normalization to guarantee that the string keys in the `owners` map represent a 1:1 mapping with physical OS file handles.
3.  **Resolve Re-entrancy:** Explicitly track and reject re-entrant lock requests from the same `taskID` to prevent premature lock deletion and subsequent data corruption.

## 6. Granular Analysis of Negative Testing Vectors

To rigorously harden the `write_set_lock_manager`, the following negative testing strategies must be implemented. Each strategy targets a specific boundary identified in the previous sections.

### 6.1 Vector: Null/Undefined/Empty Inputs

The system must prove resilient against absence of data, specifically checking the robust handling of zero values in Go.

*   **Test Case 6.1.1: `nil` Context Fallback**
    *   **Input:** `manager.acquire(nil, "task-1", []string{"file.go"}, 10*time.Millisecond)`
    *   **Action:** The system should replace `nil` with `context.Background()`.
    *   **Expected Result:** The lock is acquired successfully if available. If the lock is held indefinitely by another task, the `nil` context call should block indefinitely. This is a crucial negative behavior to verify, as it proves that passing `nil` defeats the timeout safety net.

*   **Test Case 6.1.2: `nil` and Empty `writeSet`**
    *   **Input:** `manager.acquire(ctx, "task-1", nil, 10*time.Millisecond)` and `manager.acquire(ctx, "task-1", []string{}, 10*time.Millisecond)`
    *   **Action:** `normalizeWriteSetPaths` should return `nil`.
    *   **Expected Result:** The function must return a `nil` lease (`*writeSetLockLease(nil)`) and `nil` error immediately, without acquiring the global mutex or interacting with the `owners` map.

*   **Test Case 6.1.3: Empty Strings in `writeSet`**
    *   **Input:** `manager.acquire(ctx, "task-1", []string{"", "  ", "\t\n"}, 10*time.Millisecond)`
    *   **Action:** The normalization loop should trim these strings to `""` and `continue`.
    *   **Expected Result:** Same as 6.1.2. The system should gracefully ignore invalid paths and return a `nil` lease if no valid paths remain.

*   **Test Case 6.1.4: Empty `taskID`**
    *   **Input:** `manager.acquire(ctx, "", []string{"file.go"}, 10*time.Millisecond)`
    *   **Action:** The guard clause `if taskID == ""` should trigger.
    *   **Expected Result:** The function must return an error explicitly stating that a non-empty task ID is required.

*   **Test Case 6.1.5: `nil` Lease Release**
    *   **Input:** `var l *writeSetLockLease; l.release()`
    *   **Action:** The defensive check `if l == nil || l.manager == nil` should trigger.
    *   **Expected Result:** The function returns silently without panicking.

### 6.2 Vector: Type Coercion and OS State

The system must handle the impedance mismatch between Go strings (byte slices) and the underlying operating system's file system constraints.

*   **Test Case 6.2.1: Case Sensitivity (Windows)**
    *   **Setup:** Mock `runtime.GOOS` to "windows" (if possible via build tags or internal interfaces) or run the test exclusively on a Windows runner.
    *   **Input:** Task A acquires `File.go`. Task B attempts to acquire `file.go`.
    *   **Action:** `normalizeAbsolutePath` lowercases the paths.
    *   **Expected Result:** Task B must be blocked until Task A releases the lock, proving that case-insensitive collisions are correctly handled.

*   **Test Case 6.2.2: Path Length Maximums**
    *   **Input:** `manager.acquire(ctx, "task-1", []string{string(make([]byte, 4096))}, 10*time.Millisecond)` (a 4KB path string).
    *   **Action:** The system attempts to resolve the absolute path.
    *   **Expected Result:** The system should either successfully normalize and lock the path (if the OS supports it) or gracefully return an error indicating the path is too long, rather than panicking or truncating the path silently, which could lead to locking the wrong file.

*   **Test Case 6.2.3: Unicode Normalization**
    *   **Input:** Task A acquires `e`+`´` (NFD). Task B acquires `é` (NFC).
    *   **Action:** The strings are byte-wise different but represent the same character.
    *   **Expected Result:** Currently, this will likely fail (Task B will acquire the lock concurrently). A successful test *after* fixing the issue should demonstrate that Task B is blocked because the manager normalizes both paths to the same NFC representation before checking the `owners` map.

### 6.3 Vector: User Request Extremes

The system must remain responsive and robust when faced with absurdly large requests.

*   **Test Case 6.3.1: Massive Write Sets (Benchmark)**
    *   **Setup:** Generate a `writeSet` array of 100,000 unique paths.
    *   **Action:** Call `acquire`.
    *   **Expected Result:** The test should measure the duration of the `acquire` call. It should not panic or run out of memory. However, the benchmark should flag a warning if the lock hold time (the time spent iterating the `owners` map inside `m.mu.Lock()`) exceeds a reasonable threshold (e.g., 5ms), indicating a scalability bottleneck.

*   **Test Case 6.3.2: High Frequency Contention (Thundering Herd)**
    *   **Setup:** Spawn 1000 goroutines, all attempting to acquire the *same* lock `file.go` with a `pollInterval` of 1ms.
    *   **Action:** Monitor CPU usage and time to acquisition for the last goroutine.
    *   **Expected Result:** The test will likely show immense CPU churn. A successful fix (e.g., migrating to `sync.Cond`) should drastically reduce the CPU overhead while maintaining correctness.

### 6.4 Vector: State Conflicts and Race Conditions

The system must maintain absolute mutual exclusion under complex interleavings of tasks.

*   **Test Case 6.4.1: Task ID Re-entrancy Vulnerability**
    *   **Input:** Task 1 acquires `file.go` (Lease 1). Task 1 acquires `file.go` again (Lease 2). Lease 1 releases.
    *   **Action:** The system currently allows the re-entrant lock, then deletes the lock upon the first release.
    *   **Expected Result:** This test should currently fail (demonstrating the bug). A successful fix will either reject the second acquire attempt (returning `false` in `tryAcquirePaths`) or correctly track the lease count, ensuring `file.go` remains locked until Lease 2 also releases.

*   **Test Case 6.4.2: Double Release Idempotency**
    *   **Input:** Task 1 acquires `file.go`. `lease.release()` is called concurrently from 10 different goroutines.
    *   **Action:** The `sync.Once` primitive ensures the underlying `releasePaths` logic executes only once.
    *   **Expected Result:** The system should not panic, the `owners` map should be correctly cleared, and subsequent tasks should be able to acquire the lock.

*   **Test Case 6.4.3: Deadlock Prevention via Lexicographical Sorting**
    *   **Input:** Task A requests `[C, A, B]`. Task B requests `[B, C, A]`.
    *   **Action:** Both tasks normalize and sort their arrays to `[A, B, C]`.
    *   **Expected Result:** No deadlock occurs. Task A acquires `A`, then `B`, then `C`. Task B waits on `A`. This validates the core design principle of the manager.

## 7. Summary and Action Items

The `write_set_lock_manager` is a critical component for maintaining codebase integrity during multi-agent campaigns. While its foundational design (lexicographical sorting for deadlock prevention) is solid, boundary analysis exposes several critical vulnerabilities:

1.  **Re-entrancy Bug:** The most severe issue. Task ID-based checking in `tryAcquirePaths` creates a race condition where a single task can inadvertently unlock resources it still needs.
2.  **OS/Unicode Coercion:** The lack of strict Unicode normalization (NFC/NFD) and OS-agnostic path resolution (symlink evaluation) allows visually identical but byte-differing paths to bypass mutual exclusion.
3.  **Scalability Bottleneck:** The O(N) iteration inside the global mutex `m.mu` and the 10ms polling loop will cripple the orchestrator under extreme load (massive monorepos).

**Action Items:**
1.  Implement strict rejection of re-entrant locks in `tryAcquirePaths`.
2.  Integrate `golang.org/x/text/unicode/norm` into `normalizeAbsolutePath`.
3.  Add the negative test cases defined in Section 6 to `write_set_lock_manager_test.go` to explicitly enforce these boundaries.
4.  (Future Enhancement) Refactor the polling mechanism to use `sync.Cond` for efficient, starvation-free lock notifications.

## 8. Appendix: Trace Analysis of Lock Contention

To further illustrate the performance bottleneck identified in Vector 3 (User Request Extremes), consider the following hypothetical trace of lock contention when processing a massive `writeSet`.

Let $N$ be the number of files requested by a JIT subagent, and $M$ be the number of active locks currently held in the `owners` map.

### 8.1 The Acquisition Phase (`tryAcquirePaths`)

When a task calls `acquire`, it invokes `tryAcquirePaths(taskID, paths)`. This function is guarded by a single global mutex `m.mu`.

```go
m.mu.Lock()
defer m.mu.Unlock()

// Step 1: Verification (O(N) operations)
for _, p := range paths {
    owner, held := m.owners[p]
    if held && owner != taskID {
        return false // Lock is unavailable
    }
}

// Step 2: Assignment (O(N) operations)
for _, p := range paths {
    m.owners[p] = taskID
}
return true
```

*   **Time Complexity:** The total time complexity inside the critical section is strictly $O(N)$.
*   **Space Complexity:** $O(1)$ additional space, but the map itself grows to $O(M+N)$.

**The Bottleneck:** If $N = 100,000$ (a massive refactoring campaign on a monorepo), the goroutine must perform $100,000$ map lookups and $100,000$ map insertions *while holding the global mutex*. During this time, *no other JIT subagent can even check the status of a single file*. If the median map operation takes $20ns$, this critical section holds the lock for approximately $4ms$.

### 8.2 The Contention Spike

If 50 other subagents are currently polling for locks (because their needed files are held by someone else), they wake up every $10ms$ (the `pollInterval`).

If the global mutex is held for $4ms$ by the massive task, the other 50 subagents will queue up behind `m.mu.Lock()`. When the massive task finally releases the mutex, the Go runtime must schedule the waiting goroutines.
*   **Context Switching Overhead:** The sudden rush of 50 goroutines acquiring and immediately releasing the mutex (because their files are likely still locked) generates immense CPU context switching overhead, starving actual LLM processing or parsing work.
*   **Latency Amplification:** A task requesting a single, uncontended file (e.g., `README.md`) might wait $4ms + \text{queue\_delay}$ just to acquire it, violating the expectation of near-instantaneous acquisition for disjoint resources.

### 8.3 The Release Phase (`releasePaths`)

The release process suffers from a similar, though slightly less severe, bottleneck.

```go
m.mu.Lock()
defer m.mu.Unlock()

for _, p := range paths {
    if owner, held := m.owners[p]; held && owner == taskID {
        delete(m.owners, p)
    }
}
```

*   **Time Complexity:** $O(N)$ operations inside the critical section.
*   **The Bottleneck:** Releasing $100,000$ files requires $100,000$ map lookups and deletions. Again, this holds the global mutex for several milliseconds, blocking all other acquisition attempts.

### 8.4 Summary of Trace

The trace confirms that while the `write_set_lock_manager` provides strict safety (mutual exclusion and deadlock prevention via sorting), its coarse-grained locking strategy and active polling architecture will critically degrade performance when subjected to boundary value inputs (massive $N$ or high concurrency). The system trades extreme scalability for implementation simplicity, which is a calculated risk but must be explicitly documented as a boundary constraint for codeNERD campaigns.
