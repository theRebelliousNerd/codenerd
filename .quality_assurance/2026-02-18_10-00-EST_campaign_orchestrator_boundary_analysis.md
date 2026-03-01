# QA Journal Entry: Campaign Orchestrator Task Handlers Boundary Analysis
**Date:** 2026-02-18 10:00 EST
**Author:** Jules (QA Automation Engineer)
**Subsystem:** Internal Campaign Orchestrator (Task Handlers)
**File:** `internal/campaign/orchestrator_task_handlers.go`

## 1. Subsystem Overview

The `orchestrator_task_handlers.go` file is a critical component of the Codenerd Campaign Orchestrator. It is responsible for dispatching tasks to appropriate execution handlers based on the `TaskType`. This dispatch mechanism acts as the central router for all campaign activities, delegating work to specialized "Shards" (e.g., Coder, Tester, Researcher) or executing fallback logic when shards are unavailable or fail.

The file implements the following key functionalities:
-   **Task Dispatch (`executeTask`)**: Routes tasks to specific handler methods based on `TaskType`.
-   **Shard Delegation (`spawnTask`)**: Invokes external agents (shards) to perform complex tasks.
-   **Fallback Mechanisms**: Provides direct LLM-based execution for file creation/modification when shards fail.
-   **Verification**: Performs post-execution checks (e.g., `os.Stat` to confirm file creation).
-   **Helper Functions**: Utilities for extracting code blocks and paths from LLM output.

## 2. Test Evaluation (Current State)

The current test suite for this subsystem (`internal/campaign/orchestrator_task_handlers_test.go`) is critically deficient. It contains a single benchmark: `BenchmarkExtractPathFromDescription`.

**Gaps Identified:**
-   **Zero Functional Coverage**: There are no unit tests for `executeTask`, `spawnTask`, or any of the specific task handlers (`executeFileTask`, `executeTestRunTask`, etc.).
-   **No Error Handling Tests**: The robust error handling logic (fallbacks, retries, logging) is completely untested.
-   **Missing Integration Tests**: The interaction between the Orchestrator and the Shard Executor is not verified.
-   **Helper Function Logic**: Critical helpers like `extractCodeBlock` (which parses LLM output) are untested, leaving the system vulnerable to hallucinated or malformed LLM responses.

## 3. Boundary Value Analysis & Negative Testing Vectors

To bring this subsystem up to "PhD level" quality, we must rigorous test edge cases. Below is a detailed breakdown of missing vectors.

### 3.1 Null / Undefined / Empty Inputs

**Vector:** `Task` struct fields being empty or nil.
-   **Empty `Task.Description`**: Many handlers rely on `Description` to generate prompts or shard inputs. An empty description could lead to empty prompts or confused shards.
    -   *Impact*: Shard failure, garbage file generation.
    -   *Test Case*: `executeFileTask` with empty description.
-   **Nil `Task.Artifacts`**: Handlers like `executeFileTask` check `len(task.Artifacts) > 0`. If `Artifacts` is nil, it falls back to `extractPathFromDescription`.
    -   *Impact*: Path extraction might fail or return an incorrect path.
    -   *Test Case*: `executeFileTask` with nil `Artifacts` and a description with no path.
-   **Empty `Task.Shard`**: `executeTask` checks `task.Shard != ""`. If empty, it falls back to type-based routing.
    -   *Impact*: Implicit routing might be incorrect if `TaskType` is ambiguous.
    -   *Test Case*: `executeWithExplicitShard` called with empty `Shard` string (should default or error).
-   **Nil `Orchestrator.taskExecutor`**: `spawnTask` checks for nil `te`.
    -   *Impact*: Panic or error return.
    -   *Test Case*: `spawnTask` with uninitialized `taskExecutor`.

### 3.2 Type Coercion & Data Integrity

**Vector:** Mismatched types or unexpected data formats in string fields.
-   **Path Injection in `Artifacts[0].Path`**:
    -   *Scenario*: Path contains `../`, absolute paths, or invalid characters.
    -   *Impact*: Writing files outside the workspace (security vulnerability).
    -   *Test Case*: `executeFileTask` with path `../../../../etc/passwd`.
-   **Numeric Strings in LLM Output**:
    -   *Scenario*: LLM returns a file content that looks like a number or boolean (e.g., "true").
    -   *Impact*: `extractCodeBlock` might mishandle it or downstream parsers might fail.
    -   *Test Case*: `extractCodeBlock` with input "12345" or "true".
-   **Invalid `TaskType`**:
    -   *Scenario*: `executeTask` called with a `TaskType` not in the switch statement.
    -   *Impact*: Falls through to `executeGenericTask` (default). Is this always safe?
    -   *Test Case*: `executeTask` with `TaskType("INVALID_TYPE")`.

### 3.3 User Request Extremes

**Vector:** Handling massive, complex, or nonsensical requests.
-   **Massive File Content**:
    -   *Scenario*: LLM generates a 100MB file content string.
    -   *Impact*: `executeFileTaskFallback` loads this into memory. `extractCodeBlock` processes it. Potential OOM.
    -   *Test Case*: `executeFileTaskFallback` with 500MB mocked LLM response.
-   **Deeply Nested Paths**:
    -   *Scenario*: Path depth exceeds OS limits (e.g., 1000 directories deep).
    -   *Impact*: `os.MkdirAll` failure.
    -   *Test Case*: `executeFileTask` with path `a/b/c/.../z.go` (length > 4096).
-   **Infinite Loop in Generated Code**:
    -   *Scenario*: `executeTestRunTask` runs a test that loops forever.
    -   *Impact*: Orchestrator hangs indefinitely if timeout is not enforced.
    -   *Test Case*: Mock `executor.Execute` to hang; verify `TimeoutMs` in `tactile.ResourceLimits` works.
-   **Non-Existent Languages**:
    -   *Scenario*: `getLangFromPath` called with `.xyz` extension.
    -   *Impact*: `extractCodeBlock` defaults to searching for ` ```xyz `, which might fail to find the block.
    -   *Test Case*: `executeFileTask` for `unknown.xyz`.

### 3.4 State Conflicts & Concurrency

**Vector:** Race conditions and invalid state access.
-   **Concurrent Task Execution**:
    -   *Scenario*: Two tasks modify the same file concurrently.
    -   *Impact*: Last write wins, or file corruption.
    -   *Test Case*: Parallel execution of `executeFileTask` on same path.
-   **Workspace Deletion**:
    -   *Scenario*: Workspace directory is deleted while a task is running.
    -   *Impact*: `os.WriteFile` or `os.Stat` fails.
    -   *Test Case*: `executeFileTask` where `workspace` dir is removed before write.
-   **Orchestrator Lifecycle**:
    -   *Scenario*: `Stop()` is called while a task is in `spawnTask`.
    -   *Impact*: Context cancellation should propagate, but does it verify cleanup?
    -   *Test Case*: Cancel context during `executeTask`.

## 4. Performance & Scalability Considerations

-   **Synchronous `os.Stat`**: `executeFileTask` performs a synchronous filesystem check. For campaigns with thousands of file tasks, this adds latency.
    -   *Improvement*: Batch verification or async checks.
-   **LLM Latency**: `executeFileTaskFallback` blocks on `llmClient.Complete`. If the LLM is slow, the orchestrator thread is blocked.
    -   *Improvement*: Async execution model for fallbacks.
-   **Memory Usage**: `extractCodeBlock` creates new strings. For large files, this generates garbage.
    -   *Improvement*: Stream processing or slice manipulation without allocation.

## 5. Improvement Plan: Required Tests

To address these gaps, I will add the following tests to `internal/campaign/orchestrator_task_handlers_test.go`:

1.  **`TestExecuteTask_Dispatch`**: Verify `TaskType` routing to correct internal methods.
2.  **`TestExecuteFileTask_ShardFailure_Fallback`**:
    -   Mock `spawnTask` to return error.
    -   Verify `executeFileTaskFallback` is called.
    -   Verify file is written from LLM output.
3.  **`TestExecuteFileTask_VerificationFailure`**:
    -   Mock `spawnTask` to return success (but do not write file).
    -   Verify `os.Stat` fails.
    -   Verify fallback is triggered.
4.  **`TestExtractCodeBlock_EdgeCases`**:
    -   Input: "Raw code" (no fences).
    -   Input: "```go\ncode\n```".
    -   Input: "Text ```code``` text".
    -   Input: Empty string.
5.  **`TestExecuteTask_ContextCancellation`**:
    -   Cancel context before execution.
    -   Verify immediate return with error.
6.  **`TestExecuteTestRunTask_Timeout`**:
    -   Mock `executor.Execute` to exceed timeout.
    -   Verify error handling.
7.  **`TestExecuteToolCreateTask_Timeout`**:
    -   Mock `kernel.Query` to never return `tool_registered`.
    -   Verify timeout handling and partial success return.
8.  **`TestPathTraversals`**:
    -   Attempt to write to `../outside.go`.
    -   Verify sandbox containment (if implemented) or flag as security risk.

## 6. Journal Summary

The `orchestrator_task_handlers.go` module contains high-value logic for campaign execution but currently relies entirely on integration/system tests (if any) or manual verification. The lack of unit tests for fallback logic and error handling is a significant risk, especially for a system designed to be autonomous (Autopoiesis). The proposed tests will secure the boundaries of the orchestrator and ensure robust operation under failure conditions.

Signed,
Jules
QA Automation Engineer
2026-02-18 10:00 EST

# Detailed Improvement Logic

## I. Mangle & Logic Integrity
The orchestrator relies on `LegacyShardNameToIntent` (from `internal/core`) to map string shard names to Mangle intents.
*   **Gap**: If `core` changes the mapping, `spawnTask` might dispatch to the wrong intent.
*   **Fix**: Add a test verifying `shardType` to `Intent` mapping consistency.

## II. File System Safety
The fallback mechanism (`executeFileTaskFallback`) writes files directly.
*   **Gap**: No check for `..` traversals in `targetPath` derived from LLM.
*   **Fix**: Add `filepath.Clean` and prefix verification before `os.WriteFile`.

## III. Autopoiesis Loop
`executeToolCreateTask` interacts with the kernel to assert `missing_tool_for`.
*   **Gap**: If the kernel is read-only or the assertion fails, the task hangs or fails silently (error is returned but logic flow is unclear).
*   **Fix**: Test kernel assertion failure paths.

## IV. LLM Output Parsing
`extractCodeBlock` is heuristic-based.
*   **Gap**: It picks the *first* block. If the LLM outputs "Here is the old code: ```...``` and here is the new code: ```...```", it extracts the old code.
*   **Fix**: Test with multi-block responses. Consider logic to pick the *last* block or a block labeled "new".

# Hypothetical Test Implementation Examples

To illustrate the rigor required, here are hypothetical implementations of the missing tests.

### Example 1: `TestExecuteFileTask_Fallback`

```go
func TestExecuteFileTask_Fallback(t *testing.T) {
    // Setup mocks
    mockShardExecutor := &MockShardExecutor{
        ExecuteFunc: func(ctx context.Context, intent string, task string) (string, error) {
            return "", fmt.Errorf("shard dead")
        },
    }
    mockLLM := &MockLLM{
        CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
            return "```go\npackage main\nfunc main() {}\n```", nil
        },
    }
    orch := NewOrchestrator(..., mockShardExecutor, mockLLM)

    // Execute task
    task := &Task{
        ID: "task-1",
        Type: TaskTypeFileCreate,
        Description: "Create main.go",
        Artifacts: []TaskArtifact{{Path: "main.go"}},
    }

    res, err := orch.executeFileTask(context.Background(), task)
    if err != nil {
        t.Fatalf("Expected success via fallback, got error: %v", err)
    }

    // Verify file written
    content, _ := os.ReadFile(filepath.Join(orch.workspace, "main.go"))
    if string(content) != "package main\nfunc main() {}" {
        t.Errorf("File content mismatch: %s", content)
    }
}
```

### Example 2: `TestExtractCodeBlock_Complex`

```go
func TestExtractCodeBlock_Complex(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        lang     string
        expected string
    }{
        {
            name: "Standard fence",
            input: "Here is the code:\n```go\nfunc foo() {}\n```",
            lang: "go",
            expected: "func foo() {}",
        },
        {
            name: "No fence",
            input: "func foo() {}",
            lang: "go",
            expected: "func foo() {}",
        },
        {
            name: "Wrong language fence",
            input: "```python\ndef foo(): pass\n```",
            lang: "go",
            expected: "def foo(): pass", // Current implementation returns raw if fence doesn't match
        },
        {
            name: "Multiple blocks",
            input: "Old:\n```go\nold()\n```\nNew:\n```go\nnew()\n```",
            lang: "go",
            expected: "old()", // Current implementation picks first
        },
    }

    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            got := extractCodeBlock(tc.input, tc.lang)
            if got != tc.expected {
                t.Errorf("Expected %q, got %q", tc.expected, got)
            }
        })
    }
}
```

# Deep Dive: Mangle Intent Mapping Risks

The file `internal/campaign/orchestrator_task_handlers.go` uses `core.LegacyShardNameToIntent` to bridge the gap between legacy shard names (like "coder", "tester") and Mangle intents (like `/code/modify`, `/test/run`).

This mapping is brittle. If the Mangle ontology evolves (as seen in `internal/mangle/intent_routing.mg`), the hardcoded mapping in `core` might become stale.

**Risk Scenario:**
1.  Mangle introduces a new intent structure: `/agent/coder/v2`.
2.  `LegacyShardNameToIntent` still returns `/agent/coder/v1`.
3.  The `Orchestrator` calls `spawnTask("coder")`.
4.  The `TaskExecutor` (which uses JIT compilation) fails to find a matching agent for `/agent/coder/v1` because the rules have changed.
5.  Result: Silent failure or confusing error "No agent found for intent".

**Mitigation Strategy:**
*   **Contract Tests**: Write a test in `orchestrator_task_handlers_test.go` that explicitly verifies the mapping against the current loaded Mangle rules.
*   **Dynamic Resolution**: Instead of hardcoding "coder", query Mangle for `persona(/coder)` to get the correct intent.

# Performance Analysis: Synchronous I/O

The `executeFileTask` function performs the following operations synchronously:
1.  `spawnTask` (RPC/LLM call) - High latency (seconds).
2.  `os.Stat` (Syscall) - Low latency (microseconds).
3.  `executeFileTaskFallback` (LLM call + Disk I/O) - High latency.

While the Go runtime schedules goroutines efficiently, the orchestrator's throughput is limited by the serial nature of these operations *per task*.

**Bottleneck Analysis:**
*   If a campaign has 100 file tasks, and we run them with parallelism=10.
*   Each worker is blocked for the duration of the LLM call.
*   The fallback logic is particularly expensive because it invokes the LLM *again*.

**Recommendation:**
*   Implement "Speculative Execution": While the shard is working, start pre-generating the fallback content in a low-priority background thread if the shard is known to be flaky.
*   Implement "Batched Verification": Instead of `os.Stat` per file, run a bulk verify step after a batch of tasks completes.

# Security Audit: Path Traversal

The function `executeFileTaskFallback` takes `targetPath` either from `task.Artifacts` or `extractPathFromDescription`.

```go
fullPath := filepath.Join(o.workspace, targetPath)
```

`filepath.Join` cleans the path, but it doesn't prevent `../` from escaping the root if the joined path resolves outside. However, `filepath.Join("/workspace", "../etc/passwd")` results in `/etc/passwd` (on Unix) or `/workspace/../etc/passwd` which simplifies to `/etc/passwd`.

**Vulnerability:**
If `targetPath` is `../../etc/passwd`, `filepath.Join("/app/workspace", targetPath)` becomes `/app/etc/passwd` (if it stays within) or just `/etc/passwd` if it goes up enough.
Actually, `filepath.Join` handles `..` by moving up the directory tree. It does *not* enforce a jail.

**Proof of Concept:**
```go
workspace := "/tmp/workspace"
target := "../../etc/passwd"
joined := filepath.Join(workspace, target)
// joined is "/etc/passwd"
```

**Fix Required:**
We must verify that `joined` starts with `workspace` and a separator.

```go
if !strings.HasPrefix(joined, workspace + string(os.PathSeparator)) {
    return fmt.Errorf("security violation: path traversal detected")
}
```

This check is currently MISSING in `orchestrator_task_handlers.go`. This is a critical security finding.

# Conclusion

This extensive analysis highlights that while the `Orchestrator` is functionally capable, it lacks the defensive depth required for a high-reliability, autonomous system. The missing tests, brittle Mangle integration, and potential security vulnerabilities (Path Traversal) must be addressed immediately. The provided test plan and hypothetical implementations offer a clear path forward.

---
**End of Expanded Journal Entry**

# Detailed Breakdown of Missing Test Cases

## 1. Task Dispatch Logic (`TestExecuteTask`)
| Test Name | Scenario | Expected Outcome |
| :--- | :--- | :--- |
| `TestExecuteTask_UnknownType` | `TaskType` is "unknown_shard" | Logs warning, executes generic task |
| `TestExecuteTask_EmptyType` | `TaskType` is "" | Logs warning, executes generic task |
| `TestExecuteTask_ExplicitShard` | `Task.Shard` is set to "custom_shard" | Calls `executeWithExplicitShard` |
| `TestExecuteTask_NilTask` | `task` is nil | Panics or returns error (check implementation) |

## 2. File Task Execution (`TestExecuteFileTask`)
| Test Name | Scenario | Expected Outcome |
| :--- | :--- | :--- |
| `TestExecuteFileTask_Success` | Shard returns success, file exists | Returns success map |
| `TestExecuteFileTask_ShardFail` | Shard returns error | Calls fallback |
| `TestExecuteFileTask_VerifyFail` | Shard returns success, file missing | Calls fallback |
| `TestExecuteFileTask_NoPath` | No artifacts, description has no path | Returns error |
| `TestExecuteFileTask_PathTraversal` | Path is `../../etc/passwd` | Returns security error (after fix) |

## 3. Fallback Logic (`TestExecuteFileTaskFallback`)
| Test Name | Scenario | Expected Outcome |
| :--- | :--- | :--- |
| `TestFallback_Success` | LLM returns valid code block | Writes file, returns success |
| `TestFallback_LLMFail` | LLM returns error | Returns error |
| `TestFallback_WriteFail` | Disk full / permission denied | Returns error |
| `TestFallback_Garbage` | LLM returns "I can't do that" | Writes garbage (needs validation?) |

## 4. Test Execution (`TestExecuteTestRunTask`)
| Test Name | Scenario | Expected Outcome |
| :--- | :--- | :--- |
| `TestTestRun_Success` | Shard runs tests successfully | Returns parsed results |
| `TestTestRun_ShardFail` | Shard fails | Falls back to direct execution |
| `TestTestRun_DirectFail` | Direct execution fails (compilation error) | Returns failure map |
| `TestTestRun_Timeout` | Tests hang | Context timeout triggers |

## 5. Tool Creation (`TestExecuteToolCreateTask`)
| Test Name | Scenario | Expected Outcome |
| :--- | :--- | :--- |
| `TestToolCreate_Success` | Kernel asserts `tool_registered` | Returns success |
| `TestToolCreate_Timeout` | `tool_registered` never appears | Returns "pending" status |
| `TestToolCreate_KernelFail` | `kernel.Assert` fails | Returns error immediately |

## 6. Helper Functions
| Test Name | Scenario | Expected Outcome |
| :--- | :--- | :--- |
| `TestExtractCode_Go` | ` ```go ... ``` ` | Extracts content |
| `TestExtractCode_NoFence` | Raw code | Returns raw code |
| `TestExtractCode_Empty` | "" | "" |
| `TestGetLang_Go` | `foo.go` | "go" |
| `TestGetLang_Unknown` | `foo.bar` | "bar" |

## 7. Integration Scenarios
| Test Name | Scenario | Expected Outcome |
| :--- | :--- | :--- |
| `TestIntegration_Replan` | Task fails, triggers replan | Kernel receives failure fact |
| `TestIntegration_Context` | Context cancellation propagates | All operations stop immediately |

# Final Recommendation

The `internal/campaign/orchestrator_task_handlers.go` file is a robust implementation of the Strategy pattern for task execution. However, the lack of unit tests makes it fragile to refactoring. The fallback logic, which is crucial for resilience (Autopoiesis), is completely unverified.

By implementing the test suite outlined in this document, the Codenerd team can ensure that the Campaign Orchestrator remains a reliable engine for autonomous software development. The "PhD level" rigor demanded by this analysis will prevent critical failures in production and ensure that edge cases (like path traversal and infinite loops) are handled gracefully.

This concludes the QA analysis.

# Glossary

*   **Shard**: An external agent responsible for a specific domain (Coder, Tester, etc.).
*   **Mangle**: The logic programming language used for intent routing and state management.
*   **Orchestrator**: The central component managing the campaign lifecycle.
*   **Autopoiesis**: The system's ability to self-create and self-repair (e.g., creating missing tools).
*   **JIT Compilation**: Just-In-Time compilation of Mangle intents to executable plans.
*   **Context Pager**: Manages the token budget for LLM contexts.
*   **Decomposer**: Breaks down high-level goals into actionable phases and tasks.
*   **Assault Campaign**: A specialized campaign type for adversarial testing.
*   **OODA Loop**: Observe-Orient-Decide-Act loop implemented by the Orchestrator.
*   **Kernel**: The central knowledge base storing Mangle facts.
*   **Fact**: A unit of knowledge in Mangle (e.g., `file_exists("foo.go")`).
*   **Predicate**: A relation definition in Mangle (e.g., `file_exists/1`).
*   **Atom**: A symbolic constant in Mangle (e.g., `/coder`).
*   **Variable**: A placeholder in Mangle rules (e.g., `Path`).
*   **Rule**: A logical derivation in Mangle (e.g., `p(X) :- q(X).`).
*   **Query**: A request for information from the Kernel.
*   **Assertion**: Adding a new fact to the Kernel.
*   **Derivation**: Inferring new facts from existing ones based on rules.
*   **Stratification**: Avoiding negation cycles in logic rules.
*   **Monotonicity**: Facts accumulate and are not retracted during a derivation step.
*   **Fixpoint**: The state where no further facts can be derived.
