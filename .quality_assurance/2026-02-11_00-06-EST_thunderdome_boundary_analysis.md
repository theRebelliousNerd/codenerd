# Thunderdome: Adversarial Co-Evolution Boundary Analysis
**Date:** 2026-02-11 00:06 EST
**Author:** QA Automation Engineer (Jules)
**Component:** `internal/autopoiesis/thunderdome.go`
**Scope:** Boundary Value Analysis, Negative Testing, and Performance Review

---

## 1. Executive Summary

This document details a comprehensive boundary value analysis of the `Thunderdome` component within the `autopoiesis` subsystem. Thunderdome serves as the "Adversarial Co-Evolution" arena where generated tools are subjected to attack vectors (malformed inputs, resource exhaustion attempts, etc.) to verify their robustness before deployment.

The analysis reveals significant gaps in the current testing strategy. While the *generation* of the test harness is unit-tested (`thunderdome_harness_test.go`), the *execution* of attacks (`Battle` method) and the *runtime behavior* of the harness itself lack dedicated integration tests. This leaves the system vulnerable to several critical failure modes, particularly regarding input size limits, binary data handling, and resource constraints.

## 2. Current Testing State

The current test suite for Thunderdome consists primarily of:
- `internal/autopoiesis/thunderdome_harness_test.go`: A targeted unit test verifying that the generated Go test harness correctly compiles and passes input to the tool function. This was introduced to fix the "Phantom Punch" bug where inputs were discarded.
- `internal/autopoiesis/ouroboros_test.go`: Integration tests for the Ouroboros loop, but notably, `EnableThunderdome` is set to `false` in the happy path test to simplify execution.

**Crucially, there is no `thunderdome_test.go` that actually runs a `Battle` simulation with a mock tool.** The logic relies entirely on the correctness of the `os/exec` calls and the generated harness code, which is generated as a string literal. This "stringly typed" code generation is fragile and prone to subtle bugs that only manifest at runtime during an attack.

## 3. Boundary Value Analysis Vectors

We have identified four primary vectors for potential failure, categorized by the standard QA taxonomy:

### 3.1 Null / Undefined / Empty Inputs

**Vector:** Empty Strings and Nil Slices.
- **Scenario:** An `AttackVector` is provided with an empty `Input` string.
- **Current Behavior:** The harness uses `bufio.Scanner`. `Scan()` returns `true` for non-empty lines (mostly). If input is truly empty (0 bytes), `Scan()` returns `false` immediately, and the `input` variable in the harness remains at its zero value (`""`).
- **Risk:** Low. The tool receives an empty string, which is a valid Go string.
- **Gap:** However, what if the `AttackVector` itself is `nil` in the slice passed to `Battle`? The code iterates `range attacks`. If `attacks` contains a nil pointer (if it were a slice of pointers, but it's a slice of structs `[]AttackVector`), this is impossible. But if `Input` is somehow uninitialized in a way that causes `cmd.Stdin` to receive `nil`, `os/exec` might panic or hang.
- **Test Gap:** No test verifies that an empty input string is correctly passed as `""` vs `nil` to the tool function.

### 3.2 Type Coercion (Text vs Binary)

**Vector:** Binary Data as Input.
- **Scenario:** An attack vector contains raw binary data (e.g., a fuzzing payload with null bytes `\x00`, invalid UTF-8, or control characters).
- **Current Behavior:** The harness uses `bufio.NewScanner(os.Stdin)`.
    - `scanner.Scan()` defaults to `ScanLines`, which splits on `\n`.
    - If the binary input contains `\n`, it will be split. The tool will only receive the *first line*.
    - If the binary input does *not* contain `\n` but is very large, it might be treated as one token.
    - **CRITICAL:** `scanner.Text()` returns a `string`. If the input is not valid UTF-8, Go strings can still hold arbitrary bytes, but subsequent processing might assume valid UTF-8.
- **Risk:** High. Tools designed to handle binary formats (e.g., image parsers, proto decoders) might receive truncated input if the fuzzer generates a newline character early in the payload.
- **Gap:** The harness logic `if scanner.Scan() { input = scanner.Text() }` inherently assumes line-oriented text input. This defeats the purpose of fuzzing with arbitrary binary data.

### 3.3 User Request Extremes (Size Limits & OOM)

**Vector:** Massive Input Size (> 10MB).
- **Scenario:** An attack vector sends a 50MB string (e.g., "Billion Laughs" XML payload or just garbage).
- **Current Behavior:**
    - The harness explicitly sets `scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)`.
    - This sets the *maximum* token size to 10MB.
    - If the input is > 10MB and does not contain a newline, `scanner.Scan()` will return `false`, and `scanner.Err()` will return `bufio.ErrTooLong`.
    - **The Bug:** The harness code checks `if scanner.Scan() { ... }` but *ignores* the `else` case and does *not* check `scanner.Err()`.
    - **Result:** If input > 10MB, the tool is called with an **empty string**. The attack fails to deliver the payload, and the tool "survives" essentially doing nothing.
- **Risk:** Critical. The system believes it is robust against large inputs, but in reality, it is silently dropping them.
- **Performance:** Allocating a 1MB buffer (`make([]byte, 1024*1024)`) inside the harness for *every* execution adds overhead.

**Vector:** Memory Exhaustion (OOM).
- **Scenario:** The tool allocates 200MB of RAM. Config `MaxMemoryMB` is 100.
- **Current Behavior:**
    - The harness uses `debug.SetMemoryLimit`.
    - It *also* runs a goroutine: `for { runtime.ReadMemStats(&m); if m.Alloc > limit { exit(3) } time.Sleep(100ms) }`.
    - **Race Condition:** The polling interval is 100ms. A tool can allocate 1GB, crash the OS (or container), and exit *before* the 100ms tick catches it.
    - `debug.SetMemoryLimit` (Go 1.19+) helps by triggering GC more aggressively, but it doesn't strictly *kill* the process on overflow; it just makes the GC work harder until it potentially panics or the OS kills it.
- **Risk:** Medium. The OS OOM killer is the ultimate backstop, but the harness's manual check is flaky.

### 3.4 State Conflicts (Concurrency)

**Vector:** Parallel Execution Races.
- **Scenario:** `ThunderdomeConfig` has `ParallelAttacks int`.
- **Current Behavior:** The `Battle` method iterates sequentially: `for i, attack := range attacks`.
- **Gap:** If `ParallelAttacks > 1` is ever implemented (e.g., via `errgroup`), we must ensure that:
    - `prepareArena` (compilation) is done *once* or is thread-safe. (Currently done once before the loop, which is correct).
    - `runAttack` uses unique, isolated resources. It uses `exec.CommandContext`, which is safe.
    - **Logging:** `logging.Autopoiesis` is used heavily. If multiple attacks run, logs will be interleaved, making debugging hard.
    - **Stats:** `t.stats` is protected by `t.mu`, so it is thread-safe.
- **Risk:** Low (currently), High (future). If parallelism is added, the log interleaving will be the primary pain point.

## 4. Detailed Analysis: The Scanner Buffer Limit

The most critical finding is the `bufio.Scanner` limitation in the generated harness.

```go
    // From thunderdome.go
    scanner := bufio.NewScanner(os.Stdin)
    // Use larger buffer for potential attack payloads
    scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)
    var input string
    if scanner.Scan() {
        input = scanner.Text()
    }
```

This pattern is fundamentally flawed for a general-purpose tool harness because:
1.  **Truncation:** It stops at the first newline. Many attacks (JSON payloads, multi-line scripts) contain newlines.
2.  **Silent Failure:** As noted in 3.3, exceeding the buffer size results in an empty input, not an error passed to the tool.
3.  **Type Confusion:** It forces the input to be a string.

**Recommendation:**
Replace `bufio.Scanner` with `io.ReadAll(os.Stdin)`.
- **Pros:** Handles binary data, handles newlines, no artificial 10MB token limit (limited only by available memory), returns error on failure.
- **Cons:** Loads entire input into memory. Given `MaxMemoryMB` is 100MB, reading a 50MB input is risky but acceptable (it will trigger OOM if too large, which is a valid failure mode).

**Proposed Harness Change:**
```go
    inputBytes, err := io.ReadAll(os.Stdin)
    if err != nil {
        fmt.Fprintf(os.Stderr, "HARNESS_ERROR: Failed to read stdin: %v\n", err)
        os.Exit(1)
    }
    input := string(inputBytes) // Explicit conversion, still allows binary strings in Go
```

## 5. Detailed Analysis: Compilation & Environment

The `prepareArena` function compiles the tool *once* and then runs it multiple times.
- **Pros:** Efficient. Compilation is slow (~1-2s).
- **Cons:** If the tool has global state (package-level variables), that state is *reset* for each attack because `exec.Command` spawns a new process. This is **GOOD**. It ensures isolation.
- **Risk:** `prepareArena` modifies the source code (`normalizePackage`, `generateTestHarness`).
    - **Edge Case:** What if the user code has `package main` and a `func main()`?
    - The compiler logic in `ToolCompiler` handles this, but `Thunderdome` has its own logic (`normalizePackage`).
    - If `normalizePackage` blindly replaces `package main` with `package tools`, the original `func main()` remains. Go allows a `main` function in a non-main package, but it won't be the entry point. This is acceptable.

**Environment Variable Leaks:**
- The harness runs with `build.GetBuildEnv`.
- It disables CGO (`CGO_ENABLED=0`).
- **Gap:** Does it inherit `os.Environ()`?
- `cmd.Env` logic in `prepareArena`:
  ```go
  cmd.Env = build.MergeEnv(build.GetBuildEnv(nil, arenaDir), "CGO_ENABLED=0")
  ```
- If `build.GetBuildEnv` merges with `os.Environ`, then the tool might inherit sensitive env vars from the host (API keys, etc.).
- **Security Risk:** High. The tool is untrusted (generated by AI). It should run in a clean environment.

## 6. Performance & Resource Consumption

### 6.1 Process Spawning
Each attack spawns a new process (`arena.test`).
- **Overhead:** ~10-20ms per fork/exec on Linux.
- **Impact:** Negligible for 5-10 attacks.
- **Scalability:** If we run 1000 fuzzing iterations, this becomes a bottleneck.
- **Mitigation:** For high-volume fuzzing, we would need in-process fuzzing (`testing.F`), but that risks crashing the host process (the Ouroboros loop). Isolation is worth the cost.

### 6.2 Disk I/O
- `prepareArena` writes files to `/tmp/thunderdome/...`.
- **Cleanup:** `defer os.RemoveAll(arenaDir)` is used (unless `KeepArtifacts`).
- **Risk:** If the process panics *before* the defer (e.g., OOM in the parent), artifacts accumulate.
- **Mitigation:** A cron job or startup cleaner for `/tmp/thunderdome` is recommended.

## 7. Journal of Identified Test Gaps

The following specific gaps must be addressed in `thunderdome_harness_test.go` and future `thunderdome_test.go`:

1.  **TEST_GAP_1: Large Input Handling**
    - **Location:** `internal/autopoiesis/thunderdome_harness_test.go`
    - **Description:** Verify behavior when input exceeds the scanner's 10MB buffer. Currently, we expect it to fail silently (bug), but the test should assert the *correct* behavior (which requires a code fix first, or at least documenting the failure).

2.  **TEST_GAP_2: Binary Input Integrity**
    - **Location:** `internal/autopoiesis/thunderdome_harness_test.go`
    - **Description:** Verify that inputs with newlines (`\n`) are not truncated. The current harness uses `Scan()`, which *will* truncate at newline. This is a logic bug in the harness.

3.  **TEST_GAP_3: OOM Detection Reliability**
    - **Location:** `internal/autopoiesis/thunderdome.go` (Logic)
    - **Description:** The 100ms polling interval for memory usage is too loose. A test case should inject a tool that allocates memory rapidly to see if the harness catches it or if the OS kills it first.

4.  **TEST_GAP_4: Environment Isolation**
    - **Location:** `internal/autopoiesis/thunderdome.go`
    - **Description:** Verify that the attacked tool cannot access host environment variables (e.g., `AWS_SECRET_KEY`).

## 8. Recommendations for Improvement

1.  **Rewrite Harness Input Reading:**
    - Switch from `bufio.Scanner` to `io.ReadAll`. This solves the truncation (newline) and size limit (scanner buffer) issues in one go.

2.  **Implement `internal/autopoiesis/thunderdome_test.go`:**
    - Create a true integration test that:
        - Creates a dummy `GeneratedTool`.
        - Calls `Battle()`.
        - Asserts that known-bad inputs cause expected failures.
        - Asserts that resource hogs are killed.

3.  **Harden Environment:**
    - Explicitly clear `cmd.Env` in `runAttack` to only include necessary vars (`PATH`, `GOCACHE`, etc.), removing host secrets.

4.  **Structure `AttackVector`:**
    - Add `ExpectedOutcome` to `AttackVector`. Some attacks *should* cause errors (e.g., "invalid input"). We need to distinguish "handled error" from "panic/crash".

## 9. Conclusion

The `Thunderdome` component is conceptually sound but implementation-wise fragile due to its reliance on `bufio.Scanner` for IPC. This introduces arbitrary limits and corrupts binary inputs. The testing strategy is currently minimal, focusing on the *syntax* of the generated harness rather than its *semantics* under load.

By addressing the gaps identified above—specifically replacing the input reading mechanism and adding a dedicated integration test suite—the robustness of the self-correction loop can be significantly improved. The "Phantom Punch" bug was just one symptom of this broader "stringly typed" fragility.

---
*End of Entry*
