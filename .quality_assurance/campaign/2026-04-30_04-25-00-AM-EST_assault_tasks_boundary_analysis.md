---

remediated: false
subsystem: campaign
---
# Quality Assurance Journal: Assault Tasks Boundary Value Analysis and Negative Testing

**Date:** April 30, 2026
**Time:** 04:25:00 AM EST
**Engineer:** Jules, QA Automation Engineer
**Target Subsystem:** Campaign Assault Tasks (`internal/campaign/assault_tasks.go`)

---

## 1. Executive Summary

The `assault_tasks.go` module implements adversarial campaign management for the codeNERD framework, providing functions to discover targets, execute batches of commands, record findings, and generate LLM-guided remediation plans. A preliminary review of the corresponding test suite (`assault_tasks_test.go`) reveals massive test gaps. Given the sensitive nature of adversarial sweeps and their high likelihood of encountering chaotic state changes in large monorepos, robust negative testing and boundary value analysis are critical.

Currently, the test coverage focuses heavily on the "happy path" and minimal configuration states. This journal evaluates the resilience of the assault tasks engine across four primary attack vectors: Null/Undefined/Empty, Type Coercion, User Request Extremes, and State Conflicts.

For each vector, I assess both the necessary test coverage additions and the performance implications, specifically judging whether the current JIT-driven codeNERD architecture can handle these extremes. I also detail the architectural patterns required to test these vectors safely within the Mangle logic programming environment, avoiding common pitfalls such as ghost facts and unbound variables.

---

## 2. Vector 1: Null / Undefined / Empty Inputs

### Overview
Assault tasks process file paths, configuration fields, and Mangle rules. Null, undefined, or empty inputs frequently cause panic states or infinite loops in unhardened software. This is especially true when passing facts derived from empty strings into the Mangle engine, which may violate predicate schemas.

### Identified Edge Cases

#### 2.1. Nil Orchestrator or Missing Context
- **Scenario:** The orchestrator pointer (`*Orchestrator`) is nil, or the `campaign` pointer within the orchestrator is nil during the execution of `executeAssaultDiscoverTask`, `executeAssaultBatchTask`, or `executeAssaultTriageTask`.
- **Current Mitigation:** The code checks `if o.campaign == nil { return nil, fmt.Errorf("no campaign loaded") }` at the start of these functions.
- **Test Gap:** There are no tests explicitly calling these methods with a nil campaign to verify safe error returns without panics. Furthermore, `getAssaultConfig()` requires a non-nil `o.campaign.Assault`, returning defaults if missing, which is untested.
- **Performance Evaluation:** The nil check is O(1) and extremely performant. It effectively short-circuits execution before any state allocation occurs.
- **Mangle Integration Risk:** If an error occurs, does the orchestrator correctly derive a failure fact `task_failed(TaskID, ErrorString)`? We must test that a nil context gracefully fails and updates the logical state appropriately.

#### 2.2. Empty Command Templates
- **Scenario:** An `AssaultStage` is defined with `Kind == /command` but the `Command` string is empty (`""`) or purely whitespace.
- **Current Mitigation:** `runCommandStage` doesn't seem to explicitly check for an empty command string before passing it to `shellForCommand`.
- **Test Gap:** Passing an empty command template could result in unexpected shell behavior (`bash -c ""` or `powershell -Command ""`). The engine should catch this early and fail the stage immediately, logging the misconfiguration.
- **Performance Evaluation:** Running an empty shell command creates unnecessary OS process overhead (spawning bash/powershell). Pre-validation would improve performance and reduce system noise.

#### 2.3. Empty Discovery Configurations
- **Scenario:** The target discovery configuration uses `Include` or `Exclude` arrays that are nil or contain empty string elements (`["", "   "]`).
- **Current Mitigation:** `normalizePrefix` and `matchesInclude`/`matchesExclude` handle empty prefixes cleanly, generally ignoring them.
- **Test Gap:** We must explicitly test `discoverGoTargets` with configurations containing malformed or empty path prefixes to ensure we don't accidentally match the root directory unexpectedly, or infinite-loop on root traversal if `filepath.Walk` interprets an empty string as `.`.
- **Performance Evaluation:** Checking empty prefixes adds negligible overhead but prevents catastrophic full-disk traversals.

#### 2.4. Missing Artifacts in Triage
- **Scenario:** `executeAssaultTriageTask` is called but the task contains no artifacts, or the artifacts have empty paths. This happens if the batch phase failed entirely or was skipped.
- **Current Mitigation:** `findArtifactPath` checks if `task == nil` and safely iterates through the artifacts slice.
- **Test Gap:** The test suite does not mock triage operations where `results.jsonl` is entirely missing or has a zero-byte length. We must test that the triage task outputs a summary indicating "No data collected" rather than failing ungracefully.
- **Performance Evaluation:** Graceful exit on missing artifacts is highly performant.

#### 2.5. Null LLM Client during Triage
- **Scenario:** `llmAssaultRemediationPlan` is called to generate fixes, but the orchestrator's `llmClient` is nil (e.g., offline mode, or configuration error).
- **Current Mitigation:** The code checks `if o.llmClient == nil { return nil }`.
- **Test Gap:** Ensure that when `llmClient` is nil, the system gracefully falls back to `deterministicRemediationTasks` (if applicable) or safely creates a dummy plan without crashing.
- **Performance Evaluation:** Bypassing the LLM client is an O(1) operation.

### Proposed Test Additions
- `TestExecuteAssaultDiscoverTask_NilCampaign`
- `TestGetAssaultConfig_NilConfig_ReturnsDefaults`
- `TestRunCommandStage_EmptyCommand_FailsGracefully`
- `TestDiscoverGoTargets_EmptyIncludesExcludes_Ignored`
- `TestExecuteAssaultTriageTask_MissingArtifacts_HandlesEmptyLog`
- `TestLLMAssaultRemediationPlan_NilClient_ReturnsEmpty`

---

## 3. Vector 2: Type Coercion & Data Malformation

### Overview
JSON serialization/deserialization boundaries are primary vulnerability points. The assault system heavily relies on `json.Marshal`/`Unmarshal` for tracking batch tasks, target states, and results across Mangle cycles. Additionally, interacting with the file system and Mangle AST requires careful type handling.

### Identified Edge Cases

#### 3.1. Invalid JSON in Results Log
- **Scenario:** `readAssaultResults` attempts to parse `results.jsonl`. What happens if a line is truncated, corrupted due to a sudden power loss, or contains types that do not map to the `assaultResult` struct (e.g., a string instead of an integer for `DurationMs`)?
- **Current Mitigation:** `json.Unmarshal` will return an error, but the `readAssaultResults` method currently seems to return `nil, nil` on `IsNotExist` but might fail completely on parse errors.
- **Test Gap:** No tests exist to simulate a corrupted `results.jsonl` file. The triage parser should ideally skip corrupted lines and log a warning, rather than failing the entire campaign.
- **Performance Evaluation:** Failing on the first error is fast but brittle. Skipping lines requires slightly more logic (looping and accumulating errors) but guarantees continuous operation of long-running campaigns.

#### 3.2. Extreme Timeout Values (Integer Coercion)
- **Scenario:** A user configures `DefaultTimeoutSeconds` to a negative number, or a number exceeding the maximum allowed size (e.g., `math.MaxInt64`), resulting in overflow when multiplied by `time.Second`.
- **Current Mitigation:** `newAssaultExecutor` caps `MaxTimeout` at `2*time.Hour`, but negative values might not be sanitized properly before being converted to a duration.
- **Test Gap:** Test negative and overflow timeout values being passed down to the tactile executor. Verify they clamp to a safe minimum (e.g., 1 second) rather than causing instantaneous timeouts.
- **Performance Evaluation:** Converting and validating timeout integers is trivial. Failing to do so could result in immediate process cancellation or hanging indefinitely.

#### 3.3. Stage Kind Type Mis-match
- **Scenario:** An external API or modified campaign injects an unrecognized `AssaultStageKind` string that bypasses standard Mangle atom schema checks (e.g., `/unknown_attack`).
- **Current Mitigation:** `runAssaultStage` uses a switch statement and falls back to a default error: `unknown assault stage kind: %s`.
- **Test Gap:** Test with random strings, empty strings, and special characters passed as `AssaultStageKind`. Verify the error propagates back to the orchestrator and marks the specific target attempt as failed.
- **Performance Evaluation:** The switch statement routing is extremely performant.

#### 3.4. Malformed Pathing Strings
- **Scenario:** `targetToDir` processes Windows-style paths (`C:\foo\bar`) in a Unix environment or vice versa, or handles malicious input from a compromised workspace.
- **Current Mitigation:** `targetToDir` strips prefixes like `./` but doesn't do deep path sanitization.
- **Test Gap:** Introduce null bytes (`\x00`), path traversal strings (`../../../etc/passwd`), and extreme directory lengths into `targetToDir`. Ensure it resolves to a safe, jailed path relative to the workspace.
- **Performance Evaluation:** Deep sanitization adds string manipulation cost, but is necessary for cross-platform stability. The `filepath.Clean` and `filepath.Rel` methods should be rigorously tested here.

#### 3.5. Mangle Fact Atom Injection
- **Scenario:** A directory name contains special characters (spaces, parenthesis, commas) that interfere with Mangle's Datalog syntax when converted to facts.
- **Current Mitigation:** Assumed `internal/types.ExtractString` or Mangle serialization quotes strings automatically.
- **Test Gap:** Test discovery on a directory named `dir with spaces and (brackets),oh no`. Verify that when the orchestrator creates facts for this target, the Mangle parser does not throw syntax errors.
- **Performance Evaluation:** Proper quoting/serialization is essential and incurs minimal performance overhead compared to query failure.

### Proposed Test Additions
- `TestReadAssaultResults_CorruptedJSONL_SkipsLine`
- `TestNewAssaultExecutor_NegativeTimeout_ClampsToDefault`
- `TestRunAssaultStage_InvalidStageKind_ReturnsError`
- `TestTargetToDir_PathTraversalAndNullBytes_Sanitized`
- `TestAssaultTarget_SpecialCharacters_MangleSafe`

---

## 4. Vector 3: User Request Extremes & System Stress

### Overview
codeNERD is designed for complex, high-performance tasks. We must evaluate how the system handles ridiculous constraints: massive monorepos, infinite outputs, huge arrays, and adversarial code that actively fights the executor.

### Identified Edge Cases

#### 4.1. The 50-Million Line Monorepo
- **Scenario:** The orchestrator is pointed at a repository with 500,000 `.go` files across 50,000 directories (e.g., the Kubernetes or Chromium monorepos).
- **Current Mitigation:** `discoverGoTargets` uses concurrent discovery or falls back to basic directory walks. `chunkStrings` groups targets to avoid monolithic executions.
- **Test Gap:** Simulate a filesystem tree with 1,000,000 dummy nodes and measure `discoverAssaultTargets` CPU/Memory usage. If the list of targets exceeds available RAM, the process will OOM. We must test if `BatchSize` and pagination limits apply correctly during discovery.
- **Performance Evaluation:** The system currently loads all targets into memory: `targets := make([]string, 0, len(in))`. In an extreme monorepo, an array of 500k strings will consume tens of megabytes. While manageable on modern hardware, generating JSON payloads from this array could spike RAM usage heavily. We need streaming discovery to ensure linear memory scaling.

#### 4.2. "Infinite" Shell Output (Stdout/Stderr Flooding)
- **Scenario:** A command stage (`Kind == /command`) runs a test that gets stuck in an infinite loop, spewing gigabytes of log data to stdout (e.g., `while true; do echo "spam"; done`).
- **Current Mitigation:** `newAssaultExecutor` caps `MaxOutputBytes`. The tactile executor should terminate the process when the limit is reached.
- **Test Gap:** Mock an `AssaultStage` that invokes an infinite-output script. Verify that the orchestrator truncates the output, records `Truncated: true` in the `assaultResult`, and does not crash the host machine with disk I/O or memory exhaustion.
- **Performance Evaluation:** Output truncation at the Tactile layer is performant as it prevents the Go runtime from allocating massive internal string buffers. Disk space limits must also be tested.

#### 4.3. Massive Triage Summaries for LLMs
- **Scenario:** 5,000 targets fail an assault stage, generating 5,000 `assaultFailure` structs.
- **Current Mitigation:** `buildAssaultSummary` iterates through failures to build a prompt context. It truncates using `cfg.MaxRemediationTasks`.
- **Test Gap:** Test `buildAssaultSummary` with 10,000 failed targets and `MaxRemediationTasks = 500`. Ensure the string builder enforces strict token limits or byte limits before sending to the LLM for remediation planning, to avoid "Context Window Exceeded" API errors.
- **Performance Evaluation:** String builder concatenation is fast, but sending a 2MB string to an LLM will result in expensive API rejections and massive latency.

#### 4.4. Extreme Batch Sizes
- **Scenario:** `BatchSize` is set to `math.MaxInt32`.
- **Current Mitigation:** `chunkStrings` uses `size`. If `size` is absurdly large, the loop still behaves correctly for the available array length.
- **Test Gap:** Test `chunkStrings` with size `0`, `-1`, and `math.MaxInt32`.
- **Performance Evaluation:** Simple math boundaries. O(N) array slicing is highly performant.

#### 4.5. Infinite Remediation Loops
- **Scenario:** The LLM consistently generates remediation tasks that fail the exact same way, causing an infinite loop of `assault -> failure -> triage -> remediation -> assault`.
- **Current Mitigation:** The system tracks `attempt` counters.
- **Test Gap:** Ensure the orchestrator hard-stops after a maximum number of cycles (e.g., `AssaultConfig.Cycles`). Simulate a cycle where the code never compiles and verify the campaign eventually yields a terminal failure rather than running forever.
- **Performance Evaluation:** Cycle tracking requires minimal overhead but prevents expensive infinite loops with the LLM API.

### Proposed Test Additions
- `TestDiscoverAssaultTargets_MassiveScaleOOMPrevention`
- `TestRunCommandStage_InfiniteStdout_TruncatesCleanly`
- `TestBuildAssaultSummary_TokenLimitEnforcement_MassiveFailures`
- `TestChunkStrings_ExtremeBatchSizes_BoundsCheck`
- `TestExecuteAssaultTriage_InfiniteLoopPrevention_MaxCycles`

---

## 5. Vector 4: State Conflicts & Concurrency

### Overview
Assault tasks run in an highly concurrent environment where the orchestrator manages multiple phases. Files can be deleted mid-run, execution results can be interlaced, and race conditions can occur when logging to shared files.

### Identified Edge Cases

#### 5.1. Target Deletion Mid-Assault
- **Scenario:** A batch task is scheduled for target `internal/foo`, but an external process (or another campaign phase acting asynchronously) deletes `internal/foo` before the assault executor acts on it.
- **Current Mitigation:** Go test tools will simply return a failure if the directory is missing.
- **Test Gap:** We must explicitly test target deletion between the discovery phase and the execution phase. The system should gracefully handle "directory not found" errors, log them as a specific failure type (e.g., `TargetMissing`), and proceed without crashing the batch.
- **Performance Evaluation:** The OS disk lookup handles the missing file efficiently. No major overhead.

#### 5.2. Concurrent JSONL Appends
- **Scenario:** `appendJSONL` writes to `results.jsonl`. In a highly concurrent scenario where multiple batches run in parallel, multiple goroutines might try to write simultaneously.
- **Current Mitigation:** `appendJSONL` opens the file with `os.O_CREATE|os.O_APPEND|os.O_WRONLY`. POSIX guarantees atomic appends for small writes (under `PIPE_BUF`), but this is less reliable on Windows or for large JSON objects that exceed the buffer.
- **Test Gap:** Run a stress test with `-race` firing 500 goroutines calling `appendJSONL` on the same file. Read the file back and verify that no JSON lines are intertwined, truncated, or corrupted.
- **Performance Evaluation:** Relying on OS-level append locks is fast, but if the byte slice exceeds the limit, interleaving *will* occur. A `sync.Mutex` inside the orchestrator for file writes might be required for guaranteed cross-platform safety.

#### 5.3. Locked Workspace Files (Windows Sharing Violations)
- **Scenario:** On Windows, `go build` or `go test` locks compiled artifacts or database files. If the assault task tries to read, write, or clear these files concurrently, it encounters a sharing violation.
- **Current Mitigation:** Tactile executor isolates environments to some extent, but running commands locally against the active workspace could trigger locks.
- **Test Gap:** Mock a locked file in a target directory and verify the system handles the permissions error gracefully, perhaps retrying with exponential backoff or logging a specific error.
- **Performance Evaluation:** Disk I/O blocks are fundamentally slow. The system must use context timeouts to break out of locked-file hangs.

#### 5.4. Double Triage Phase (Idempotency)
- **Scenario:** A network error causes the orchestrator to retry the triage phase. Does it append duplicate remediation tasks to the next phase?
- **Current Mitigation:** `executeAssaultTriageTask` appends to phase tasks using `appendTasksToPhase`.
- **Test Gap:** Call `executeAssaultTriageTask` twice in a row. Verify whether it is idempotent or if it causes duplicate tasks to be spawned in the Mangle fact store and the campaign state.
- **Performance Evaluation:** Idempotency checks require a small overhead (checking if tasks with specific hashes already exist), but save massive amounts of duplicate LLM calls and processing time downstream.

#### 5.5. Context Cancellation During Heavy I/O
- **Scenario:** The user hits Ctrl+C or a global timeout fires while `executeAssaultBatchTask` is writing huge amounts of data or iterating through a thousand targets.
- **Current Mitigation:** Contexts are passed down, but loops might not check `ctx.Done()`.
- **Test Gap:** Inside the batch processing loop, simulate context cancellation. Verify the function exits immediately, cleans up any intermediate file handles, and returns `context.Canceled`.
- **Performance Evaluation:** Checking `ctx.Err() != nil` inside tight loops adds slight overhead but is absolutely mandatory for responsive CLI applications.

### Proposed Test Additions
- `TestAssaultBatchTask_TargetDeletedMidFlight_GracefulSkip`
- `TestAppendJSONL_ConcurrencyStress_NoInterleaving`
- `TestLockedWorkspaceFiles_HandlesSharingViolations`
- `TestExecuteAssaultTriageTask_Idempotency_NoDuplicateTasks`
- `TestExecuteAssaultBatchTask_ContextCancellation_ImmediateExit`

---

## 6. Conclusion & Action Items

The `assault_tasks.go` implementation contains sophisticated logic for managing adversarial coding loops, but the lack of boundary testing exposes it to OOM risks on massive repositories, JSON corruption during concurrent appends, and state conflicts during restarts.

### Implementation Guide for Mangle-based Testing
As this subsystem interacts with the orchestrator, and thus the Mangle engine, any tests added must follow the guidelines:
1. **Clean Slate:** Always instantiate a `factstore.NewSimpleInMemoryStore()` per test. Do not reuse global stores to prevent ghost facts from contaminating the fixpoint.
2. **Type Strictness:** Use strict AST helpers (`ast.Name`, `ast.String`) when asserting state. Passing raw Go strings where Atoms are expected will lead to silent failures.
3. **Goroutine Leaks:** Ensure context cancellation (`defer cancel()`) to avoid Goroutine leaks from tactile executors and Mangle streaming APIs.
4. **Golden Files:** For massive triage summary outputs, prefer writing the expected output to a `.golden` file rather than hardcoding massive strings in the test file.

### TODO Tracker Integration
I will be injecting explicit `// TODO: TEST_GAP:` markers directly into `internal/campaign/assault_tasks_test.go` to flag these missing scenarios for the rest of the engineering team, organized by the vector classifications discussed above.

**End of Journal Entry**

## 7. Deep Dive: Architectural Resilience of codeNERD against Assault Extremes

This section elaborates on the system-level performance implications of the identified vectors, particularly focusing on how codeNERD's JIT Clean Loop architecture handles these stresses.

### 7.1. JIT Clean Loop vs. Memory Bloat
The codeNERD architecture recently shifted to a JIT Clean Loop. This means sessions start fresh, and ephemeral facts are filtered at kernel boot.
- **Advantage in Assaults:** Adversarial sweeps generate massive amounts of ephemeral data (e.g., thousands of test failures, command outputs, compiler warnings). The JIT architecture prevents this data from permanently polluting the Mangle fact store across sessions.
- **Vulnerability:** During a single long-running campaign (e.g., a 24-hour adversarial assault), the fact store can still bloat significantly if intermediate results aren't pruned.
- **Testing Implication:** We need a stress test that runs 100 assault cycles and verifies that memory usage remains stable (garbage collection of old facts or truncating history). The orchestrator currently relies on writing to `results.jsonl`, but if it also asserts facts for every single failure into the kernel, we could hit OOM within a single session.

### 7.2. Piggyback Protocol Dual-Channel Output Handling
codeNERD uses a dual-channel output protocol (Piggyback) for NL↔Logic conversion.
- **Scenario:** Triage phase generates a remediation plan that is logically sound (Mangle rules) but the NL description is extremely long, truncated, or contains formatting errors (like unbalanced code blocks).
- **Vulnerability:** If `llmAssaultRemediationPlan` fails to cleanly separate the intent from the logic due to a bloated prompt context (Vector 4.3), the JIT compiler will fail to assemble the remediation task.
- **Testing Implication:** Negative tests must feed garbled, extremely long, and malformed strings into the triage summary parser to ensure the Piggyback transducer safely rejects or repairs the output.

### 7.3. The Tactile Legacy Executor and Kernel Synchronization
The assault system uses `tactile.Executor` to run shell commands.
- **Scenario:** Concurrent assault batches fire off multiple `go test` commands.
- **Vulnerability:** The tactile executor might not be fully synchronized with the Mangle kernel's locking mechanism. If an assault command modifies the workspace (e.g., generating a coverage file) while another orchestrator phase is evaluating facts about that file, we have a classic race condition.
- **Testing Implication:** The `TestAppendJSONL_ConcurrencyStress` is just the start. We need tests that trigger assault commands that intentionally manipulate file locks while the orchestrator reads them.

### 7.4. Mangle Evaluation Finite Verification
Mangle evaluations are guaranteed to terminate *if* the rules are safe and stratified.
- **Scenario:** An assault task introduces a cyclic dependency in the remediation plan (e.g., Task A requires fixing Task B, Task B requires fixing Task A).
- **Vulnerability:** While Mangle inherently prevents infinite loops in logic derivation, the Go code orchestrating the cycles (`AssaultConfig.Cycles`) could infinite loop if it continually feeds new, slightly altered facts into the engine based on LLM hallucinations.
- **Testing Implication:** The `TestExecuteAssaultTriage_InfiniteLoopPrevention_MaxCycles` must specifically check that the orchestrator's state machine accurately increments the cycle counter and enforces the hard limit, regardless of what the Mangle derivation outputs.

## 8. Final QA Sign-off Requirements

Before any major refactor of the `assault_tasks.go` subsystem, the following conditions must be met:
1. All `// TODO: TEST_GAP:` items marked in this journal and the codebase must be implemented.
2. The tests must run in CI under `go test -race` to catch any concurrent JSON append issues.
3. The monorepo scale test must pass within reasonable memory limits (e.g., under 1GB RAM for 500k files).
4. All Mangle-interacting tests must explicitly use `analysis.Analyze()` to prevent unsafe derivations from being introduced via remediation plans.

## 9. Expanded Vector Analysis: Extended State Conflicts

### 9.1. Cross-Session Phantom States
- **Scenario:** A user cancels an assault campaign mid-way, and restarts codenerd. The `results.jsonl` file and the `targets.json` file remain on disk in the `.nerd/campaigns/` directory.
- **Vulnerability:** When the user starts a new assault or resumes, the orchestrator might read stale state that contradicts the current reality of the codebase (e.g., target files have been deleted by the user manually while codenerd was offline).
- **Test Gap:** We need a test that simulates an interrupted assault, manually deletes files in the workspace, and then restarts the orchestrator. The `discoverAssaultTargets` phase must reconcile the existing `targets.json` against the current filesystem and purge orphaned targets before executing the batch.
- **Performance Evaluation:** Checking `os.Stat` for every target before execution adds IO overhead. For a 50k target monorepo, this could delay batch execution significantly. We must test if `executeAssaultBatchTask` handles missing targets lazily (during execution) rather than upfront validation.

### 9.2. Concurrent Modifying Assaults
- **Scenario:** The user triggers two separate campaigns (Campaign A and Campaign B) that both run assault tasks against the same sub-package `internal/core`.
- **Vulnerability:** Both campaigns attempt to write remediation tasks that modify the same files. Since assaults are adversarial, Campaign A's remediation might conflict with Campaign B's analysis.
- **Test Gap:** The `write_set_lock_manager` should theoretically protect against this, but assault tasks are read-heavy and execute `go test` which creates temporary artifacts. We must test concurrent orchestrators running assaults on the same workspace.
- **Performance Evaluation:** This is an integration-level test that will be extremely slow. It is necessary to prove the robustness of the system.

### 9.3. The "PanicMaker" Subagent Edge Case
- **Scenario:** The adversarial assault is configured to use the `PanicMaker` or `Thunderdome` subagent to actively try to break the codebase by injecting faults.
- **Vulnerability:** If the subagent injects a fault that breaks the Go compiler globally (e.g., modifying `go.mod` to an unresolvable state), all subsequent assault batches will fail immediately.
- **Test Gap:** Simulate a scenario where a stage corrupts the build environment. The orchestrator must recognize a global failure cascade (e.g., 50 consecutive `go build` failures with the exact same error) and halt the assault campaign early to save compute time, rather than mindlessly iterating through 10,000 targets.
- **Performance Evaluation:** Implementing a circuit breaker pattern in `executeAssaultBatchTask` based on sequential identical failures would drastically improve efficiency in catastrophic failure scenarios.

## 10. Expanded Vector Analysis: Malformed Output from LLMs

### 10.1. Remediation Plan JSON Injection
- **Scenario:** The LLM is prompted to output a JSON remediation plan during `llmAssaultRemediationPlan`. It instead outputs a string containing a JSON injection attack, or deeply nested JSON that causes stack overflow during parsing.
- **Vulnerability:** The JIT compiler and standard `json.Unmarshal` might choke, or worse, execute malicious task structures.
- **Test Gap:** Feed massive, deeply nested JSON (e.g., 10,000 levels deep) into the LLM mock response for `llmAssaultRemediationPlan`. Ensure the JSON parser handles it safely without crashing the orchestrator.
- **Performance Evaluation:** Deeply nested JSON parsing can be an attack vector (Billion Laughs attack equivalent for JSON). A custom decoder with depth limits is highly recommended.

### 10.2. Remediation Plan Hallucination of Non-Existent Files
- **Scenario:** The LLM correctly formats the JSON but hallucinates a remediation task for `internal/core/ghost_file.go`.
- **Vulnerability:** The orchestrator appends this task to the phase. When the phase executes, the tactile executor will fail to find the file.
- **Test Gap:** The assault triage phase test must mock an LLM response containing invalid paths. The system must filter out remediation tasks for files that do not exist, or at least handle the execution failure gracefully without failing the entire remediation phase.
- **Performance Evaluation:** Path validation before task appending is fast and prevents wasted cycles in the executor.

## 11. Expanded Vector Analysis: Host OS Environment Extremes

### 11.1. Out of Disk Space (ENOSPC)
- **Scenario:** The host system runs out of disk space while `appendJSONL` is writing the results of a massive monorepo assault.
- **Vulnerability:** The `os.OpenFile` or `f.Write` will return an `ENOSPC` error. If unhandled, the orchestrator might panic or silently drop results, corrupting the campaign state.
- **Test Gap:** Mock a failing filesystem interface (or use a tiny loopback partition on Linux) to trigger `ENOSPC` during `executeAssaultBatchTask`. The system must detect the lack of space, pause the campaign, and alert the user, rather than entering a failure loop.
- **Performance Evaluation:** This requires robust error handling at the tactile boundary.

### 11.2. Memory Limits and Cgroups
- **Scenario:** codeNERD is running inside a Docker container with a strict 256MB RAM limit.
- **Vulnerability:** The `chunkStrings` function combined with `executeAssaultBatchTask` might attempt to spawn too many concurrent `go test` processes, breaching the cgroup limit and causing the OOM Killer to terminate codeNERD.
- **Test Gap:** We must test the concurrency controls. `AssaultConfig.BatchSize` must be respected. We need a test that sets batch size to 100, and verifies that the executor semaphore or worker pool strictly limits active OS processes to the available CPU/Memory resources.
- **Performance Evaluation:** Uncontrolled process spawning is the most common cause of agent death in CI environments. A strict worker pool is essential.

## 12. Conclusion Summary
This journal entry has outlined over 30 distinct test gaps across four major vectors. The `assault_tasks.go` subsystem is powerful but currently lacks the defensive programming required for enterprise-grade monorepo integration. The corresponding TODOs will be placed in `assault_tasks_test.go` to guide future development.

## 13. Deep Dive: Mangle Rules & Logic Boundary Analysis

As codeNERD is a Neuro-Symbolic architecture, the ultimate source of truth is the Mangle fact store. The assault subsystem bridges imperative Go execution with declarative Mangle logic. This section explores boundary conditions at the transduction layer between Go and Mangle.

### 13.1. Atom Truncation & Stringification
- **Scenario:** An assault target has a path of `internal/core/very_long_directory_name_that_exceeds_normal_limits/...`. When this target fails, the failure fact `assault_failure(Target, Error)` is generated.
- **Vulnerability:** Mangle string length limits or atom naming conventions might cause silent rejection of the fact. If the fact is rejected, the orchestrator loses track of the failure, and the remediation phase never triggers.
- **Test Gap:** We need a test that passes a 1MB string as the `Error` field to `kernel.Assert()`. We must verify if the string is truncated safely by the Go layer before insertion, or if Mangle handles it gracefully.
- **Performance Evaluation:** Inserting massive strings into a Datalog engine will bloat memory and severely slow down unification algorithms during queries. The `executeAssaultTriageTask` should hash or truncate massive compiler errors before asserting them as facts.

### 13.2. Unbound Variables in Remediation Rules
- **Scenario:** The `llmAssaultRemediationPlan` generates a plan, which the system then tries to convert into Mangle rules to guide the next phase.
- **Vulnerability:** If the LLM generates a rule with an unbound variable (e.g., `remediate(X) :- target_failed(Y).` where `X` is never defined), the Mangle analysis phase will fail.
- **Test Gap:** Mock an LLM response that successfully parses as JSON but contains a structurally unsafe Mangle rule in the remediation metadata. Verify that `analysis.Analyze()` catches the safety error and prevents the rule from being added to the kernel, falling back to a default plan.
- **Performance Evaluation:** The `analysis.Analyze()` call is fast but computationally non-trivial. It must be called exactly once per new rule set to prevent runtime crashes during `Eval()`.

### 13.3. Negation Cycles & Stratification
- **Scenario:** The LLM devises a complex remediation plan involving negation: "Do not fix A if B is failing, and do not fix B if A is failing."
- **Vulnerability:** If this translates to Mangle rules `fix(a) :- not fix(b). fix(b) :- not fix(a).`, this is unstratified. The Mangle engine cannot compute a fixpoint.
- **Test Gap:** We must explicitly test that the orchestrator safely handles `ErrUnstratified` from the Mangle kernel when processing LLM-generated logic.
- **Performance Evaluation:** Stratification checks happen during analysis. If it fails, the system must recover gracefully without entering an infinite loop of trying to re-compile the rules.

### 13.4. Ghost Facts from Interrupted Sweeps
- **Scenario:** An assault cycle completes batch 1 and 2, writing `assault_success` facts. The user kills the process during batch 3. Upon restart, the orchestrator boots in Quiescent mode (filtering ephemeral facts).
- **Vulnerability:** Are `assault_success` facts considered ephemeral? If they are filtered, the orchestrator will re-run batch 1 and 2. If they are persistent, how does the orchestrator know they correspond to the *current* state of the code, rather than the state 5 hours ago?
- **Test Gap:** Test the Quiescent Boot sequence with an existing assault campaign. Verify that target hashes or timestamps are used to invalidate old facts.
- **Performance Evaluation:** Recalculating hashes for thousands of files on boot is slow. The system should use an indexed caching mechanism.

## 14. Real-World Execution Scenarios (Stress Testing)

To truly harden the assault subsystem, we must simulate the environment of a 50-million line monorepo on constrained hardware.

### 14.1. The "Laptop with 8GB RAM" Scenario
- **Scenario:** The user is running codeNERD locally on a laptop to assault a massive codebase.
- **Vulnerability:** CPU thrashing and Out-Of-Memory exceptions.
- **Mitigation Strategy Required:**
    1. **Streaming JSONL:** `readAssaultResults` currently reads the entire file into memory `results = append(results, r)`. This must be refactored to use a streaming channel pattern if the file exceeds 50MB.
    2. **Backpressure:** The orchestrator dispatch loop must use a semaphore channel to limit concurrent `tactile.Executor` runs.
    3. **Lazy Discovery:** Instead of `filepath.Walk` collecting 500,000 strings upfront, the discovery phase should yield targets via a channel, writing them directly to `targets.json` in chunks.

### 14.2. The "Frontier Benchmark" Problem
- **Scenario:** The LLM is tasked with solving a frontier-level coding benchmark that requires 50 steps of logical deduction across 20 files. The assault task is validating intermediate steps.
- **Vulnerability:** Context window exhaustion. The triage summary will grow linearly with the complexity of the failure.
- **Mitigation Strategy Required:** The `buildAssaultSummary` function must implement semantic compression. Instead of concatenating raw errors, it should use a vector embedding search to cluster similar errors and only present representatives to the LLM.

## 15. Final Summary
This document provides a comprehensive blueprint for hardening the `assault_tasks.go` module against the rigors of real-world adversarial testing. By addressing these 40+ boundary conditions, type coercion risks, and performance bottlenecks, the codeNERD framework will achieve the enterprise-grade stability required for its neuro-symbolic architecture.

## 16. Expanded Edge Cases: Network and Dependency Failures

### 16.1. Flaky Network Connections during LLM Triage
- **Scenario:** The triage phase relies heavily on the `llmClient` to generate remediation plans based on assault failures. During a massive campaign, the network connection to the LLM provider drops or experiences severe latency.
- **Vulnerability:** If `llmAssaultRemediationPlan` blocks indefinitely or crashes on the first timeout, the entire campaign halts.
- **Test Gap:** Mock a failing `llmClient` that returns `context.DeadlineExceeded` or `io.ErrUnexpectedEOF`. Verify that the orchestrator implements exponential backoff with jitter, and eventually falls back to a deterministic remediation strategy (or safely pauses the campaign) instead of abandoning the task.
- **Performance Evaluation:** Proper retry mechanisms with timeouts are crucial to avoid zombie processes waiting on dead network sockets.

### 16.2. Dependency Corruption During Assault
- **Scenario:** An assault stage executing `go test` downloads external module dependencies. A man-in-the-middle or corrupted proxy cache results in invalid checksums in `go.sum`.
- **Vulnerability:** All subsequent Go-related assault stages will fail with module verification errors. The orchestrator might misinterpret this as a code failure rather than an environment failure.
- **Test Gap:** Introduce a mock failure where the output contains `verifying module: checksum mismatch`. Verify the orchestrator can parse this specific failure mode and trigger an environment reset (e.g., `go clean -modcache`) rather than just logging a generic failure.
- **Performance Evaluation:** Recognizing environment failures versus code failures saves tremendous amounts of compute by aborting pointless test runs.

## 17. Security and Permissions Boundaries

### 17.1. Malicious Directory Structures
- **Scenario:** In an adversarial test of codeNERD itself, a user creates a workspace with symlink loops, or extremely deep directory structures (`mkdir -p a/b/c/d/e...` repeating 1000 times).
- **Vulnerability:** `discoverAssaultTargets` using `filepath.Walk` will traverse the symlink loop or exceed the maximum call stack / path length limit, causing a panic or infinite loop.
- **Test Gap:** Test target discovery against a workspace containing a symlink loop and a deeply nested structure exceeding `PATH_MAX` (typically 4096 bytes on Linux). Ensure `filepath.WalkDir` detects the symlink loop or correctly handles the `path too long` error without crashing.
- **Performance Evaluation:** File system traversal is inherently risky. `filepath.WalkDir` is more efficient than `filepath.Walk` but still susceptible to pathological filesystem states.

### 17.2. Privilege Escalation Attempts in Command Stages
- **Scenario:** A user submits an `AssaultConfig` with a command stage designed to break out of the tactile executor's isolation: `sudo rm -rf /` or `chmod +s /bin/bash`.
- **Vulnerability:** If the tactile executor runs with high privileges, the command stage could destroy the host OS.
- **Test Gap:** Ensure that `newAssaultExecutor` drops privileges appropriately, or runs in a restricted container. While this is primarily a Tactile system concern, the Assault Orchestrator must test that it correctly handles permission denied errors returned by the executor when attempting privileged operations.
- **Performance Evaluation:** Security isolation (like Docker or seccomp) adds overhead but is non-negotiable for executing arbitrary commands.

## 18. Recommendations for Immediate Action

1.  **Refactor `assault_tasks_test.go`:** It currently only contains 3 tests. Expand it to include the 25+ specific `Test...` scenarios outlined in this journal.
2.  **Implement Streaming JSONL:** Replace the in-memory array slice in `readAssaultResults` with a line-by-line scanner to prevent OOM on massive campaigns.
3.  **Add Input Sanitization:** Implement rigorous path sanitization in `targetToDir` and `normalizePrefix` to defend against path traversal and null-byte injection.
4.  **Strengthen LLM Fallbacks:** Ensure `llmAssaultRemediationPlan` has robust error handling, retries, and fallback mechanisms when the API is unavailable or returns garbage.

---
**End of Journal Entry**

## 19. Addendum: Mangle Fixpoint Safety in Remediations

### 19.1. Non-Monotonic Deductions
- **Scenario:** The triage phase asserts a fact `assault_failure(Target)`. A remediation phase runs, and the orchestrator attempts to *retract* the fact using a non-monotonic update before starting the next cycle.
- **Vulnerability:** Mangle is strictly monotonic within a single evaluation. If the orchestrator tries to simulate state changes by retracting facts without creating a new clean store, it violates Datalog principles, leading to undefined behavior or ghost facts.
- **Test Gap:** Verify that the orchestrator creates a *brand new* `factstore.NewSimpleInMemoryStore()` at the start of every cycle, rather than attempting to mutate an existing one. This is the only safe way to represent time and state changes in pure Mangle.
- **Performance Evaluation:** Creating a new memory store is extremely fast. Trying to surgically remove facts from an existing store is both dangerous and slower.

### 19.2. Variable Binding Leakage
- **Scenario:** `buildAssaultSummary` generates a Mangle query to check for previous failures. `?Target` is unbound in a negative literal: `new_target(T) :- target(T), not failed(T, ?Reason).`
- **Vulnerability:** Mangle requires all variables in a negated literal to be bound by a positive literal in the same rule body. If `?Reason` is unbound, analysis fails.
- **Test Gap:** Provide the orchestrator with a complex, slightly malformed Mangle query during the triage setup. Verify that the analysis engine properly catches the "unbound variable in negative literal" error and reports it cleanly to the user log.
- **Performance Evaluation:** This is an essential safety check that must never be bypassed for performance reasons.

**Final End of Journal Entry**
