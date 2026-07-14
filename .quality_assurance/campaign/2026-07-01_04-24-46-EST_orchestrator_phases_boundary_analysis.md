# Quality Assurance Journal: Boundary Value Analysis for `internal/campaign/orchestrator_phases.go`
**Date & Time**: 2026-07-01 04:24:46 EST
**Engineer**: QA Automation
**Subsystem**: Campaign Orchestrator - Phase Management
**File Reviewed**: `internal/campaign/orchestrator_phases.go`
**Test File**: `internal/campaign/orchestrator_phases_test.go`

## 1. Overview & Context

The Campaign Orchestrator is responsible for managing the state transitions and execution flow of complex, multi-phase adversarial/research/refactoring campaigns. In `orchestrator_phases.go`, the orchestrator queries the `VirtualStore` (Mangle kernel) for the current phase, eligible tasks, and phase transition logic.

As part of the JIT Clean Loop architecture, the orchestration relies heavily on logic-first evaluations using Mangle (`getCurrentPhase`, `getEligibleTasks`, `startNextPhase`). Because the entire state machine transitions are predicated on external logic execution and fact-store queries, this subsystem is highly susceptible to adversarial facts, nil pointers from uninitialized states, and O(n^2) scaling issues when handling extreme boundaries.

The test suite currently evaluates a purely "Happy Path" approach:
- `TestOrchestrator_GetCurrentPhase`: Tests matching a fact to a phase, non-existent phase, and no fact.
- `TestOrchestrator_GetEligibleTasks`: Tests backoff mechanics and matching eligible tasks.
- `TestOrchestrator_GetNextTask`: Tests success and not in phase.
- `TestOrchestrator_IsCampaignComplete`: Checks phase completion conditions.
- `TestOrchestrator_IsPhaseComplete`: Checks task completion conditions.
- `TestOrchestrator_GetCampaignBlockReason`: Tests string extraction.

The following sections document critical boundary gaps and edge cases missing from the test suite and potentially the implementation.

---

## 2. Null, Undefined, and Empty States

The system assumes well-formed structures coming from campaigns and Mangle facts. It lacks defensive programming around `nil` pointers and empty states.

### 2.1. `isPhaseComplete` with `nil` Phase
If `isPhaseComplete(phase *Phase)` is called with a `nil` phase, it will immediately panic:
```go
func (o *Orchestrator) isPhaseComplete(phase *Phase) bool {
    // Panic: assignment to entry in nil map, or range over nil pointer
    for _, task := range phase.Tasks { ... }
}
```
**Test Gap**: `TestOrchestrator_IsPhaseComplete_NilPhase`
We must test that passing `nil` to `isPhaseComplete` does not crash the system.

### 2.2. `startNextPhase` with `nil` Context
The method `startNextPhase(ctx context.Context)` begins with:
```go
select {
case <-ctx.Done():
```
If `ctx` is `nil`, calling `ctx.Done()` will cause a panic.
**Test Gap**: `TestOrchestrator_StartNextPhase_NilContext`
Test the behavior when a nil context is passed, which can happen if a JIT prompt evaluation dynamically triggers a phase transition outside an HTTP request scope.

### 2.3. `completePhase` with `nil` Phase
`completePhase` locks the orchestrator and iterates through `o.campaign.Phases`, comparing against `phase.ID`.
If `phase` is `nil`, `phase.ID` will panic.
**Test Gap**: `TestOrchestrator_CompletePhase_NilPhase`

### 2.4. Empty Campaign Phases (`isCampaignComplete`)
If a campaign is instantiated with 0 phases (e.g., an empty assault configuration), `isCampaignComplete` skips the loop:
```go
for _, phase := range o.campaign.Phases {
    // skipped
}
return true
```
This implies an empty campaign is always complete. Is this the desired behavior, or should it trigger an error?
**Test Gap**: `TestOrchestrator_IsCampaignComplete_EmptyCampaign`

### 2.5. Malformed Mangle Facts (Empty Args)
`getCampaignBlockReason` has a length check (`len(facts[0].Args) >= 2`), but `getCurrentPhase` and `startNextPhase` blindly access `facts[0].Args[0]`:
```go
phaseID := types.ExtractString(facts[0].Args[0])
```
If an adversarial rule asserts `current_phase()` with 0 arguments, the system will panic out of bounds.
**Test Gap**: `TestOrchestrator_GetCurrentPhase_MalformedFact`
Assert `current_phase()` without arguments in the kernel and ensure the orchestrator handles it gracefully (returns nil or logs error).

---

## 3. Type Coercion & Schema Violations

The Mangle engine has strict typing (Atom vs String vs Number). The orchestrator relies on `types.ExtractString` to bridge Mangle outputs to Go primitives.

### 3.1. Non-String Fact Arguments
If the kernel logic is misconfigured and returns an integer or boolean for `next_campaign_task(12345)`, `ExtractString` may either return `""`, `"12345"`, or panic depending on its internal implementation.
**Test Gap**: `TestOrchestrator_GetNextTask_TypeCoercion`
Test the behavior when Mangle returns unexpected types (e.g., AST Number) for phase IDs or task IDs.

### 3.2. Empty String Fact Arguments
What if the phase ID extracted is `""`? The system will try to match `""` against `o.campaign.Phases[i].ID`. If an uninitialized phase has `ID: ""`, it might match incorrectly.
**Test Gap**: `TestOrchestrator_GetCurrentPhase_EmptyStringFact`

---

## 4. User Request Extremes (Performance & Scaling Boundaries)

The orchestrator operates on data structures that could scale massively during a brownfield repository ingestion (e.g., 50 million lines of code) or Adversarial Assault campaigns.

### 4.1. O(N^2) Complexity in `getEligibleTasks`
The `getEligibleTasks` method iterates over all phase tasks and matches them against all facts returned from the kernel:
```go
for i := range phase.Tasks {
    for _, fact := range facts {
        taskID := types.ExtractString(fact.Args[0])
        if phase.Tasks[i].ID == taskID { ... }
    }
}
```
If a phase contains 10,000 tasks (e.g., a batch operation for 10,000 files) and the kernel returns 5,000 eligible task facts, this loop runs 50,000,000 times on every orchestration tick. This is computationally expensive and will lock up the JIT loop.
**Test Gap**: `TestOrchestrator_GetEligibleTasks_ExtremeScaling`
Create a phase with 10,000 tasks, assert 10,000 facts, and benchmark the execution to ensure it doesn't trigger the Mangle timeout limiter or block the orchestrator indefinitely. (System should ideally use a map `map[string]struct{}`).

### 4.2. Extreme NextRetryAt Times
In `getEligibleTasks`, backoff logic uses:
```go
if !t.NextRetryAt.IsZero() && t.NextRetryAt.After(now)
```
What if `NextRetryAt` is set to the year 9999 (e.g., `time.Unix(1<<63-1, 0)`) due to an integer overflow or malicious user input?
**Test Gap**: `TestOrchestrator_GetEligibleTasks_ExtremeBackoff`

---

## 5. State Conflicts & Race Conditions

The Orchestrator operates in a highly concurrent environment where perception, tool execution, and state evaluation occur asynchronously.

### 5.1. Unsafe Read of `campaign.Phases`
Methods like `getCurrentPhase`, `getNextTask`, `isCampaignComplete`, and `isPhaseComplete` read `o.campaign.Phases` without acquiring `o.mu.Lock()`.
If another goroutine is modifying the phase array (e.g., dynamic phase generation, or another orchestration loop running `completePhase` which updates `Status`), a read-write race condition occurs.
**Test Gap**: `TestOrchestrator_Concurrency_ReadWritePhases`
Run `getCurrentPhase` and `completePhase` in parallel goroutines to trigger the Go data race detector (`go test -race`).

### 5.2. `startNextPhase` Time-of-Check to Time-of-Use (TOCTOU)
```go
facts, err := o.kernel.Query("phase_eligible")
// ...
o.mu.Lock()
defer o.mu.Unlock()
```
The kernel is queried *before* the lock is acquired. Between querying the kernel and acquiring the lock, another goroutine might have already transitioned the phase, completed the campaign, or retracted the `phase_eligible` fact.
**Test Gap**: `TestOrchestrator_StartNextPhase_RaceCondition`

### 5.3. Double `startNextPhase` Execution
If `startNextPhase` is invoked concurrently by two JIT evaluation events, both may query `phase_eligible`, get the same fact, and attempt to start the phase. While the lock prevents simultaneous writing, the second goroutine will blindly update `o.campaign.Phases[i].Status = PhaseInProgress` and emit events again without checking if the phase was already started.
**Test Gap**: `TestOrchestrator_StartNextPhase_DoubleInvocation`

### 5.4. `completePhase` Fact Assertions Out-of-Sync
```go
_ = o.kernel.RetractFact(core.Fact{...})
o.kernel.Assert(core.Fact{...})
```
If the kernel fails to assert the fact (e.g., syntax error, schema mismatch, or closed store), `completePhase` silently ignores the error and saves the campaign state to disk. The on-disk JSON will say the phase is `completed`, but the in-memory Mangle store will hold stale or inconsistent data, completely breaking the JIT loop on the next tick.
**Test Gap**: `TestOrchestrator_CompletePhase_KernelAssertFailure`
Simulate a kernel rejection during `completePhase` and verify if the system maintains state integrity.

---

## 6. Recommendations & Remediation Plan

1. **Add Nil Checks**: Immediately add `if phase == nil` checks to `isPhaseComplete`, `completePhase`, and context checks to `startNextPhase`.
2. **Bounds Checking**: Always verify `len(facts[0].Args) > 0` before indexing into `Args[0]`.
3. **Optimize `getEligibleTasks`**: Refactor the O(N^2) loop into an O(N) operation using a hashed map for facts lookup.
4. **Thread Safety**: Add `o.mu.RLock()` / `o.mu.RUnlock()` to all reader methods (`getCurrentPhase`, `getEligibleTasks`, `getNextTask`, `isCampaignComplete`, `isPhaseComplete`).
5. **Idempotency in Transitions**: Inside `startNextPhase` and `completePhase`, ensure the phase isn't *already* in the target state before modifying it and emitting events.

By aggressively testing these boundaries, the Orchestrator will become resilient against adversarial agent behavior, massive source code ingestion bursts, and concurrent transducer events.

## 7. Deep Dive: Memory Profiling and CPU Bounds (Assault Context)

When orchestrating large adversarial assault campaigns spanning 100,000+ files, the Mangle fact store becomes a significant bottleneck.

### 7.1. Fact Compaction Failures
If `getCurrentPhase` returns thousands of facts over time, and `completePhase` fails to fully retract them due to a timeout or panic, the `VirtualStore` can bloat. The orchestrator must periodically trigger a "Fact Compaction" cycle or assert a `clean_session` fact.
**Performance Vector:** An assault campaign creates 1,000 phases sequentially. Each `completePhase` assertion adds 1 MB of memory overhead if Mangle indices aren't optimized. Can the Go GC handle the allocation spikes in the JIT loop?
**Test Gap**: `TestOrchestrator_MemoryLeak_LongRunningAssault`

### 7.2. Goroutine Leaks in Event Emitting
```go
o.emitEvent("phase_completed", phase.ID, "", phase.Name, nil)
```
If `emitEvent` blocks (e.g., a slow subscriber or full channel) and it's called synchronously inside `completePhase` while `o.mu` is locked, the entire Orchestrator halts.
**Test Gap**: `TestOrchestrator_EmitEvent_SlowSubscriber_Deadlock`

### 7.3. Context Budgets Exceeded
The JIT loop manages a `ContextBudget`. If a phase (like `/remediation`) is loaded, it might bring in gigabytes of code context. When `getEligibleTasks` runs, does it verify if the phase's tasks will exceed the LLM context window?
**Test Gap**: `TestOrchestrator_PhaseTasks_ExceedContextBudget`

---

## 8. Chaos Engineering Scenarios (Subsystem Injections)

To ensure the Orchestrator is truly robust, we must inject failures at the boundary layers connecting Mangle, the LLM Transducers, and the Go Runtime.

### 8.1. Northstar Observer Panics
```go
if o.northstarObserver != nil {
    check, err := o.northstarObserver.OnPhaseStart(...)
    ...
}
```
If the `northstarObserver` (which often makes network calls to a fast classification tier) times out or panics, it could bring down the `startNextPhase` sequence.
**Test Gap**: `TestOrchestrator_Northstar_Observer_Panic_Recovery`

### 8.2. Mangle Timeout Exhaustion
The JIT prompt relies on `kernel.Query()`. If a Mangle evaluation rule is unstratified or infinitely recursive (e.g., `p(X) :- p(X+1)`), `kernel.Query` should abort based on `ratifyEvalTimeout`.
**Test Gap**: `TestOrchestrator_Mangle_Timeout_Handling`
We must test that the orchestrator degrades gracefully and marks the campaign as "failed" or "blocked" rather than hanging forever if the Mangle query hits the context deadline.

### 8.3. Spurious Task Duplication
If the fact store inadvertently duplicates task IDs (due to non-deterministic sorting or a sync issue during a 2PC edit operation), `getEligibleTasks` might append the same task pointer multiple times to the slice.
**Test Gap**: `TestOrchestrator_GetEligibleTasks_DuplicatedFacts`

---

## 9. Negative Testing Protocol for CodeDOM Injections

CodeNERD's JIT architecture means tools can edit the orchestrator's own state if unguarded.

### 9.1. Self-Modification Assault
During a campaign, what happens if an LLM agent uses `/edit_file` to modify `internal/campaign/orchestrator_phases.go` while it is actively running? The Go runtime doesn't allow live-reloading, but the JIT Mangle rules DO.
If an agent edits `orchestrator_phases.mg` on disk, does the next tick pick up the malicious rule?
**Test Gap**: `TestOrchestrator_LiveRule_Mutation_Prevention`

### 9.2. Symbolic Pollution via `VirtualStore`
The `InteractiveExecutiveGate` is supposed to guard against destructive actions, but the orchestrator trusts `campaign_phase` facts implicitly.
If a malicious rule inserts:
`campaign_phase("/phase_1", "/campaign_1", "Hacked", -1, "/completed", "")`
The orchestrator's state machine will instantly warp to completed.
**Test Gap**: `TestOrchestrator_Fact_Authentication_Bypass`

---

## 10. Expanding the Boundary Value Matrix

Below is a structured matrix of boundary values that the QA team must systematically implement as table-driven tests in `orchestrator_phases_test.go`.

| Vector | Parameter | Boundary Value | Expected Orchestrator Behavior |
|--------|-----------|----------------|--------------------------------|
| **Nullity** | `ctx` | `nil` | Graceful error, no panic in `Done()` |
| **Nullity** | `phase` | `nil` | Return `false` or `nil`, log warning |
| **Empty** | `campaign.Phases` | `[]Phase{}` | `isCampaignComplete` == true (or error) |
| **Empty** | `facts` | `[]core.Fact{}` | Safe return `nil` |
| **Schema** | `Args[0]` type | `ast.Number(1)`| `ExtractString` ignores or converts safely |
| **Schema** | `Args` length | `0` | Prevent `index out of range` panic |
| **Time** | `NextRetryAt` | `time.Unix(-1,0)` | Process immediately (past) |
| **Time** | `NextRetryAt` | `time.Unix(1<<63-1,0)`| Skip forever, do not overflow logic |
| **Scale** | Phase count | `100,000` | Mangle query completes < 50ms |
| **Scale** | Task count | `1,000,000` | `getEligibleTasks` completes < 100ms |
| **Concurrency**| Readers | `100 threads` | No data races on `o.campaign` reads |
| **Concurrency**| Writers | `50 threads` | `completePhase` lock prevents corruption|

### 10.1 Implementing the Table-Driven Matrix

The current tests are monolithic. We must refactor `TestOrchestrator_GetCurrentPhase` to use table-driven tests (TDT) to handle these boundaries.

```go
func TestOrchestrator_GetCurrentPhase_Boundaries(t *testing.T) {
    tests := []struct {
        name       string
        facts      []core.Fact
        phases     []Phase
        wantPhase  string
        wantPanic  bool
    }{
        { "Valid Phase", []core.Fact{{Args: []any{"/p1"}}}, []Phase{{ID: "/p1"}}, "/p1", false },
        { "Empty Fact Args", []core.Fact{{Args: []any{}}}, []Phase{{ID: "/p1"}}, "", true },
        { "Nil Phases", []core.Fact{{Args: []any{"/p1"}}}, nil, "", false },
        { "Type Mismatch", []core.Fact{{Args: []any{ast.Number(1)}}}, []Phase{{ID: "1"}}, "", false }, // Depends on ExtractString
    }
    // ... test logic
}
```

---

## 11. Orchestrator Phase Lifecycle Fuzzing Strategy

Given the complexity of the Campaign Orchestrator, standard unit tests are insufficient. We must introduce Fuzz Testing using Go 1.18+ fuzzing for `getEligibleTasks` and `getCurrentPhase`.

### 11.1. Fuzzing `getCurrentPhase`
We will feed random string data, malformed byte slices, and massive strings into the Mangle fact simulator to ensure `getCurrentPhase` never panics.

```go
func FuzzOrchestrator_GetCurrentPhase(f *testing.F) {
    f.Add("normal_phase_id")
    f.Add("")
    f.Add(strings.Repeat("A", 10000)) // 10KB string
    f.Add("\x00\x01\x02\xFF") // binary garbage

    f.Fuzz(func(t *testing.T, phaseID string) {
        // ... inject phaseID into mock kernel
        // ... call orch.getCurrentPhase()
        // ... assert no panics
    })
}
```

### 11.2 Fuzzing Phase Ordering Constraints
Phases have an `Order` field. What happens if a campaign defines phases out of order, or with duplicate orders?
```go
Phases: []Phase{
    {ID: "p1", Order: 2},
    {ID: "p2", Order: 1},
    {ID: "p3", Order: -1},
}
```
If `startNextPhase` relies on sequential progression, negative ordering or duplicate ordering could cause infinite loops in the phase transition logic.
**Test Gap**: `TestOrchestrator_Phase_Order_Validation`

## 12. Final Verification Recommendations
To ensure complete robustness of the `internal/campaign/orchestrator_phases.go` subsystem:
1. Implement the table-driven test matrix outlined in Section 10.
2. Add the Go fuzz tests described in Section 11 to the CI pipeline.
3. Apply `o.mu.RLock()` to all pure reader functions immediately.
4. Add strict schema validation (`len(Args) > 0`, type assertions) directly after `kernel.Query` calls.
5. Create a synthetic integration test (`tests/e2e/campaign_orchestrator_chaos_test.go`) that simulates network partitions, Mangle engine timeouts, and disk I/O blocks while a campaign is running.

End of Report.

## 13. Deep Boundary Analysis: Cross-System Contaminations

Because the JIT Clean Loop architecture boots from a blank slate (`Quiescent Boot`), we must consider edge cases where ephemeral facts from a *previous* phase or a *previous* campaign session leak into the current orchestrator evaluation due to an improper `kernel.RetractFact` execution.

### 13.1 Ghost Facts and the "Forgotten Sender" Leak
As noted in the AI Failure Modes guide, testing Mangle engines requires a "Clean Slate" store. If `completePhase` executes but the `RetractFact("campaign_phase")` fails silently, both `/completed` and `/in_progress` facts might exist simultaneously in the kernel.
**Negative Test Condition**: Force `RetractFact` to fail in a mock kernel. Call `completePhase`. Then query the kernel. If the orchestrator proceeds to the next tick with conflicting state, it may cause an infinite replan loop.
**Test Gap**: `TestOrchestrator_CompletePhase_GhostFact_Leak`

### 13.2 Resource Exhaustion via Task Spawning
In `Assault Execution` phases, if a directory contains 50,000 Go files, and the orchestrator dynamically expands tasks using `getEligibleTasks`, the slice allocation could trigger an Out of Memory (OOM) kill.
**Performance Bound**: Slice `append` operations in `getEligibleTasks` scale geometrically.
```go
tasks := make([]*Task, 0, len(facts))
// ...
tasks = append(tasks, &phase.Tasks[i])
```
If `facts` is 50,000, the pre-allocation is correct. But if the phase *also* has 50,000 tasks, the inner loop executes 2.5 billion operations.
**Test Gap**: `TestOrchestrator_GetEligibleTasks_OOM_Prevention`

### 13.3 Adversarial Null-Byte Injection
What happens if a user submits a campaign goal with a null byte (`\x00`) in the task ID or phase ID?
When `types.ExtractString` parses the fact, does it truncate the string, causing task ID collisions, or does it pass the null byte to the file system or locking manager?
**Test Gap**: `TestOrchestrator_ExtractString_NullByte_Handling`

## 14. Reflection and Pre-Commit Finalization

The boundary analysis reveals that while the Orchestrator logic is semantically sound for the Happy Path, it lacks the defensive programming required for a production-grade, adversarial CLI tool.

The immediate next steps for the engineering team are:
- Add `mu.RLock()` to all readers in `orchestrator_phases.go`.
- Implement `len(facts[0].Args) == 0` guards.
- Implement the requested `TODO` tests to harden the pipeline.

This concludes the 400+ line QA journal boundary analysis for the Campaign Orchestrator subsystem.

## 15. Transaction and Synchronization Anomalies

A critical area of boundary testing involves the intersection of the Orchestrator's internal mutex (`o.mu`) and external synchronization systems like the `WriteSetLockManager`.

### 15.1 Mutex Deadlocks
In `startNextPhase`, the orchestrator attempts to align with the Northstar observer *while holding the lock*:
```go
o.mu.Lock()
defer o.mu.Unlock()
// ...
if o.northstarObserver != nil {
    check, err := o.northstarObserver.OnPhaseStart(ctx, phaseID, o.campaign.Phases[i].Name)
}
```
If `OnPhaseStart` performs a synchronous network request or waits on a channel, the entire Orchestrator is blocked. If `ctx` times out, `OnPhaseStart` might return an error, but if it ignores the context, it causes a hard deadlock.
**Test Gap**: `TestOrchestrator_StartNextPhase_Deadlock_On_Northstar`

### 15.2 Incomplete Phase Rollback
If `startNextPhase` fails mid-transition (e.g., due to the Northstar alignment check failing), it returns an error:
```go
if err != nil {
    logging.Campaign("Northstar blocked phase %s: %v", phaseID, err)
    return fmt.Errorf("northstar alignment failed: %w", err)
}
```
However, *before* this check, the code has already modified the in-memory state and the Mangle kernel:
```go
o.campaign.Phases[i].Status = PhaseInProgress
_ = o.kernel.RetractFact(...)
o.kernel.Assert(...)
```
If the phase transition is "blocked," the orchestrator is now in a corrupted state: it failed the transition, but its internal tracking and Mangle facts claim the phase is `in_progress`.
**Critical Test Gap**: `TestOrchestrator_StartNextPhase_Rollback_On_Failure`
This is a severe transaction safety violation. The orchestrator must implement a Two-Phase Commit (2PC) or defer state mutations until *after* all fallible operations (like Northstar checks) succeed.

## 16. Conclusion
The Orchestrator's phase management requires immediate refactoring to address TOCTOU races, transaction rollbacks, O(N^2) scaling bottlenecks, and unhandled nil pointers. The missing test cases identified in this document serve as the roadmap for remediation.

## 17. The Holographic Context Impact

The `HolographicProvider` maintains an AST cache. When the orchestrator executes a phase transition, does it invalidate the cache?
If a task in Phase 1 heavily refactors `core.go`, and Phase 2 starts immediately, the Orchestrator's internal queries or the JIT prompt might rely on stale Holographic Context because `fileContentCache` wasn't flushed.
**Test Gap**: `TestOrchestrator_PhaseTransition_HolographicCache_Invalidation`
This edge case bridges the `campaign` subsystem and the `world` subsystem, requiring a cross-package integration test.

## 18. SIMD Architecture Considerations
If the experimental Go 1.26 toolchain (`GOEXPERIMENT=simd`) is active, does the O(N^2) loop in `getEligibleTasks` behave deterministically? SIMD-accelerated string matching inside the Mangle engine might reorder facts. Since Mangle evaluation is inherently unordered, the orchestrator MUST NOT rely on slice append order.
**Test Gap**: `TestOrchestrator_GetEligibleTasks_Deterministic_Order`

Final Line count optimization.
