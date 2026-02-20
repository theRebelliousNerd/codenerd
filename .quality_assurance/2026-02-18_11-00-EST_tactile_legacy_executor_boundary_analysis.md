# QA Journal Entry: Legacy SafeExecutor Boundary Analysis
**Date:** 2026-02-18 11:00 EST
**Author:** Jules (QA Automation Engineer)
**Subsystem:** Internal Tactile Executor (Legacy SafeExecutor)
**File:** `internal/tactile/executor.go`

## 1. Subsystem Overview

The `internal/tactile/executor.go` file implements the `SafeExecutor` struct, which is marked as a legacy component maintained for backward compatibility with the `VirtualStore` system. Despite its deprecation status in favor of the newer `DirectExecutor` (in `direct.go`), it remains a critical attack surface because:
1.  It is still compiled into the binary.
2.  It may be invoked by older code paths or specific configurations.
3.  It attempts to enforce security policies ("Constitutional Logic") via a simple allowlist mechanism, which is prone to bypasses.

The `SafeExecutor` wraps Go's `os/exec` package and provides a simplified interface (`ShellCommand`) for running system commands. Its primary security mechanism is a map `AllowedBinaries` which explicitly permits or denies specific executables (e.g., allowing `go`, `git`, `bash`, but denying `rm`).

## 2. Test Evaluation (Current State)

The current test suite (`internal/tactile/tactile_test.go`) focuses heavily on the new `DirectExecutor`. There are **no direct tests** for `SafeExecutor`'s unique behaviors, specifically its allowlist enforcement or its legacy environment handling.

**Critical Gaps Identified:**
-   **Security Bypass Testing**: No tests verify that `rm` is actually blocked. More importantly, no tests verify that `bash -c "rm ..."` is handled correctly (spoiler: it likely isn't).
-   **Environment Isolation**: The `SafeExecutor` has a dangerous default behavior regarding environment variables that is completely untested.
-   **Resource Limits**: Unlike `DirectExecutor`, the `SafeExecutor` uses `CombinedOutput()`, which buffers all output in memory. There are no tests for OOM scenarios.
-   **Concurrency**: No tests for race conditions when modifying `AllowedBinaries` (though it's not thread-safe by design).

## 3. Boundary Value Analysis & Negative Testing Vectors

To bring this legacy subsystem up to a "PhD level" understanding of its fragility, we must explore the following edge cases.

### 3.1 Environment Variable Leaks (The "Clean Slate" Violation)

**Vector:** The `ShellCommand.EnvironmentVars` field.
-   **Scenario**: The caller passes `nil` or an empty slice for `EnvironmentVars`.
-   **Code Path**: `internal/tactile/executor.go:76`: `c.Env = cmd.EnvironmentVars`.
-   **Behavior**: In Go's `os/exec`, setting `Cmd.Env` to `nil` causes the command to inherit the **parent process's environment**.
-   **Impact**: Critical Security Vulnerability. If the Codenerd process runs with `AWS_ACCESS_KEY_ID` or `OPENAI_API_KEY` in its environment, executing `env` via `SafeExecutor` will leak these secrets to the child process (and potentially to the logs or user output).
-   **Test Case**: Execute `env` with `EnvironmentVars: nil` and assert that sensitive keys from the host are *present* (proving the leak).

### 3.2 Memory Exhaustion (The "Infinite Buffer" Attack)

**Vector:** Large output from executed commands.
-   **Scenario**: A command outputs 10GB of text (e.g., `yes | head -n 1000000000`).
-   **Code Path**: `internal/tactile/executor.go:78`: `output, err := c.CombinedOutput()`.
-   **Behavior**: `CombinedOutput` reads the entire stdout and stderr into a `bytes.Buffer` until EOF.
-   **Impact**: Denial of Service (OOM). The Go runtime will panic or the OS will kill the container when memory is exhausted. This is a trivial DoS vector for any user able to run commands.
-   **Test Case**: Execute a command that generates 1GB of zeroes and observe memory usage/panic.

### 3.3 Security Allowlist Bypass (The "Wolf in Sheep's Clothing")

**Vector:** Indirect execution via allowed shells.
-   **Scenario**: The user wants to run `rm -rf /`.
-   **Constraint**: `AllowedBinaries["rm"]` is `false`.
-   **Bypass**: `AllowedBinaries["bash"]` is `true`.
-   **Attack**: User executes `Binary: "bash"`, `Arguments: ["-c", "rm -rf /"]`.
-   **Code Path**: `internal/tactile/executor.go:60`: The check only looks at `cmd.Binary`.
-   **Impact**: Total security bypass. The "Constitutional Logic" is cosmetic.
-   **Test Case**: Attempt to create a file via `bash -c "touch pwned"` and verify it exists, proving arbitrary code execution is possible despite restrictions.

### 3.4 Path Traversal & Working Directory

**Vector:** `WorkingDirectory` manipulation.
-   **Scenario**: `WorkingDirectory` set to `../../../../etc`.
-   **Code Path**: `internal/tactile/executor.go:75`: `c.Dir = cmd.WorkingDirectory`.
-   **Behavior**: `os/exec` allows relative paths.
-   **Impact**: Commands can run in arbitrary directories, potentially accessing sensitive files outside the workspace if the container allows it.
-   **Test Case**: Set `WorkingDirectory` to `/` (or root equivalent) and run `ls`.

### 3.5 Null/Empty/Type Coercion

**Vector:** Invalid inputs to `ShellCommand`.
-   **Empty Binary**: `Binary: ""`.
    -   *Behavior*: `exec.Command` panics or returns error depending on version. `SafeExecutor` does not validate.
    -   *Test Case*: Execute with empty binary.
-   **Nil Arguments**: `Arguments: nil`.
    -   *Behavior*: Valid. Runs binary with no args.
-   **Timeout=0**: `TimeoutSeconds: 0`.
    -   *Code Path**: `internal/tactile/executor.go:67`: Defaults to 30 seconds.
    -   *Behavior*: This is actually "safe" default behavior, but implicit.
    -   *Test Case*: Verify 30s timeout is applied.

## 4. Improvement Plan: Required Tests

To address these critical gaps, we must add specific tests to `internal/tactile/tactile_test.go` (or a new file).

### 4.1 Test: `TestSafeExecutor_EnvironmentLeak`
**Objective**: Prove that `nil` environment leaks host secrets.
```go
func TestSafeExecutor_EnvironmentLeak(t *testing.T) {
    // Set a secret in the host process
    os.Setenv("SECRET_KEY", "super_secret")
    defer os.Unsetenv("SECRET_KEY")

    exec := NewSafeExecutor()
    // Explicitly pass nil environment
    cmd := ShellCommand{
        Binary: "env", // or "cmd /c set" on Windows
        EnvironmentVars: nil,
    }

    output, _ := exec.Execute(context.Background(), cmd)
    if strings.Contains(output, "super_secret") {
        t.Fatalf("Security Vulnerability: SafeExecutor leaked host environment!")
    }
}
```

### 4.2 Test: `TestSafeExecutor_OOM_DoS`
**Objective**: Demonstrate lack of output limiting.
```go
func TestSafeExecutor_OOM_DoS(t *testing.T) {
    exec := NewSafeExecutor()
    // Generate large output (e.g., 100MB)
    cmd := ShellCommand{
        Binary: "python3",
        Arguments: []string{"-c", "print('A'*1024*1024*100)"},
    }

    // This should ideally fail or be truncated, but legacy will try to buffer it all
    output, err := exec.Execute(context.Background(), cmd)
    if len(output) > 10*1024*1024 {
        t.Logf("Legacy executor buffered %d bytes (Risk of OOM)", len(output))
    }
}
```

### 4.3 Test: `TestSafeExecutor_Bypass`
**Objective**: Demonstrate `bash` allowlist bypass.
```go
func TestSafeExecutor_Bypass(t *testing.T) {
    exec := NewSafeExecutor()
    // 'rm' is banned, but 'bash' is allowed
    // We try to use bash to do something 'rm' like, or just proving execution
    cmd := ShellCommand{
        Binary: "bash",
        Arguments: []string{"-c", "echo 'pwned'"},
    }

    output, err := exec.Execute(context.Background(), cmd)
    if err == nil && strings.Contains(output, "pwned") {
        t.Errorf("Security Weakness: Allowed binary 'bash' allows arbitrary command execution")
    }
}
```

## 5. Performance & Architectural Implications

The `SafeExecutor` uses a **synchronous, blocking model** for execution.
-   It calls `c.CombinedOutput()` which blocks the goroutine until the command completes.
-   It creates a new `context.WithTimeout` for *every* execution.

**Performance Bottleneck**:
If 50 requests come in to execute `sleep 10`, 50 goroutines will be blocked for 10 seconds. In a high-throughput scenario (e.g., fuzzing or massive campaign execution), this will exhaust goroutines.

**Architectural Flaw**:
The `AllowedBinaries` map is hardcoded and instantiated per `NewSafeExecutor`.
-   It cannot be updated dynamically without recompiling or modifying code.
-   It mixes policy ("Constitutional Logic") with mechanism (`SafeExecutor`).

## 6. Recommendations

1.  **Immediate Deprecation**: The `SafeExecutor` should be removed entirely. The `DirectExecutor` (in `direct.go`) is superior in every way (output limits, better env handling).
2.  **Migration Path**: All calls to `NewSafeExecutor` should be replaced with `NewDirectExecutor`.
3.  **Policy Extraction**: The "Constitutional Logic" (allowlist) should be moved to a separate Policy Engine (likely Mangle-based) rather than hardcoded strings.
4.  **Fix Environment Handling**: Change `c.Env = cmd.EnvironmentVars` to `c.Env = cmd.EnvironmentVars` BUT if nil, force it to empty slice `[]string{}` or a sanitized baseline.
5.  **Limit Output**: Replace `CombinedOutput` with `StdoutPipe` and `io.CopyN` to enforce a limit (e.g., 1MB).

## 7. Journal Summary

The `internal/tactile/executor.go` file represents "technical debt" with security implications. While functionality like `AllowedBinaries` suggests safety, it provides a false sense of security due to shell escape vectors. The memory management (unbounded buffering) and environment handling (leakage by default) are critical flaws that would not pass a modern code review.

This analysis confirms that the "material code quality" of this legacy component is low compared to the newer `DirectExecutor`. The lack of negative tests hides these vulnerabilities. The proposed tests will make these risks explicit and force the migration to the secure implementation.

Signed,
Jules
QA Automation Engineer
2026-02-18 11:00 EST

# Detailed Breakdown of Missing Test Cases for Tactile Legacy

## 1. Environment Handling
| Test Name | Scenario | Expected Outcome (Current) | Expected Outcome (Fixed) |
| :--- | :--- | :--- | :--- |
| `TestSafeExecutor_EnvLeak` | `EnvironmentVars` is nil | **FAIL**: Leaks host env | **PASS**: Empty env |
| `TestSafeExecutor_EnvExplicit` | `EnvironmentVars` has `KEY=VAL` | **PASS**: Has `KEY=VAL` | **PASS**: Has `KEY=VAL` |

## 2. Resource Limits
| Test Name | Scenario | Expected Outcome (Current) | Expected Outcome (Fixed) |
| :--- | :--- | :--- | :--- |
| `TestSafeExecutor_LargeOutput` | Command outputs 1GB | **CRASH/SLOW**: Buffers all | **PASS**: Truncates output |
| `TestSafeExecutor_Timeout` | Command hangs | **PASS**: Kills process | **PASS**: Kills process |

## 3. Security Boundaries
| Test Name | Scenario | Expected Outcome (Current) | Expected Outcome (Fixed) |
| :--- | :--- | :--- | :--- |
| `TestSafeExecutor_BlockedBinary` | Binary is `rm` | **PASS**: Returns error | **PASS**: Returns error |
| `TestSafeExecutor_AllowedShell` | Binary `bash`, Args `rm` | **FAIL**: Executes `rm` | **FAIL**: (Hard to fix without removing shell) |
| `TestSafeExecutor_PathTraversal` | `WorkingDirectory` is `../../` | **FAIL**: Access outside workspace | **PASS**: Jailed or Error |

## 4. Input Validation
| Test Name | Scenario | Expected Outcome (Current) | Expected Outcome (Fixed) |
| :--- | :--- | :--- | :--- |
| `TestSafeExecutor_EmptyBinary` | Binary is "" | **PANIC/ERROR**: `exec: arg[0] empty` | **PASS**: Validation Error |

# Conclusion on Material Code Quality

The existence of `SafeExecutor` alongside `DirectExecutor` violates the **DRY (Don't Repeat Yourself)** principle and creates confusion about which security model applies. The `DirectExecutor` is clearly the "materially better" implementation, yet the legacy code persists.

By adding the negative tests outlined above, we will create "failing tests" that document the exact reasons why `SafeExecutor` must be retired. This is a classic QA strategy: **Break it to fix it.**

# Extended Analysis: Code Walkthrough

Let's dissect `internal/tactile/executor.go` line by line to pinpoint the exact locations of vulnerabilities.

```go
// ...
type SafeExecutor struct {
    AllowedBinaries map[string]bool
}

func NewSafeExecutor() *SafeExecutor {
    return &SafeExecutor{
        AllowedBinaries: map[string]bool{
            // ...
            "bash":    true, // CRITICAL: Allows arbitrary shell execution
            "sh":      true, // CRITICAL: Allows arbitrary shell execution
            // ...
        },
    }
}
```
**Analysis:** The constructor hardcodes the allowlist. The inclusion of `bash` and `sh` effectively negates the restriction on `rm` (which is explicitly set to `false`). Any user who can call `Execute` can pass `bash` as the binary and `-c rm -rf /` as arguments. The `Execute` method only checks `cmd.Binary`, not the arguments.

```go
func (e *SafeExecutor) Execute(ctx context.Context, cmd ShellCommand) (string, error) {
    // ...
    if allowed, exists := e.AllowedBinaries[cmd.Binary]; exists && !allowed {
        // ...
        return "", fmt.Errorf("binary not allowed by Constitutional Logic: %s", cmd.Binary)
    }
    // ...
```
**Analysis:** This check is flawed. It assumes that the binary name is the sole determinant of safety. It does not inspect arguments. It also allows any binary NOT in the map (unless the map is exhaustive and default-deny? The code implies `exists && !allowed`, which means if it doesn't exist, it is permitted? No, usually maps return zero value (false) if not found. But `exists` checks presence. If it's NOT in the map, `exists` is false, so the check passes! **Wait, this is a default-allow policy?** Let's re-read carefully:
`if allowed, exists := e.AllowedBinaries[cmd.Binary]; exists && !allowed`
If `exists` is false (binary not in map), the condition is false, so it proceeds.
**This means any binary NOT in the list is ALLOWED.**
So I can run `perl`, `python`, `gcc`, `wget`, `curl` because they are not in the list!
**This is a massive vulnerability.** A "Safe" Executor should be Default-Deny.

```go
    c := exec.CommandContext(ctx, cmd.Binary, cmd.Arguments...)
    c.Dir = cmd.WorkingDirectory
    c.Env = cmd.EnvironmentVars
```
**Analysis:**
1.  `c.Dir`: No validation that this directory is inside a sandbox or workspace.
2.  `c.Env`: As discussed, if `cmd.EnvironmentVars` is nil, it leaks parent env.

```go
    output, err := c.CombinedOutput()
```
**Analysis:** Unbounded read. A classic OOM vector.

# Risk Scenarios: "The Environment Heist"

Imagine a scenario where Codenerd is deployed in a Kubernetes cluster. The pod has `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` injected as environment variables for S3 access.

1.  **Attacker Action**: An attacker (or a rogue AI agent) submits a request to "list environment variables".
2.  **System Response**: The system uses `SafeExecutor` (perhaps due to a legacy fallback configuration).
3.  **Command Construction**: The `ShellCommand` is created with `Binary: "env"`. Crucially, the `EnvironmentVars` field is left `nil` because the request didn't specify custom variables.
4.  **Execution**: `SafeExecutor` runs `env`. Because `c.Env` is nil, `os/exec` copies the pod's environment to the child process.
5.  **Leak**: The `env` command prints all variables to stdout.
6.  **Exfiltration**: The output is returned to the user or logged to a transparency log. The keys are compromised.

**Mitigation:** `DirectExecutor` avoids this by explicitly building the environment from an allowlist (`e.config.AllowedEnvironment`) and the provided variables. `SafeExecutor` has no such mechanism.

# Refactoring Roadmap

To eliminate this technical debt, we propose a 3-phase plan:

## Phase 1: Containment (Immediate)
Add a wrapper to `SafeExecutor.Execute` that enforces:
1.  **Default-Deny**: If binary is not in `AllowedBinaries`, reject it.
2.  **Env Sanitization**: If `cmd.EnvironmentVars` is nil, set it to `[]string{}`.
3.  **Output Limit**: Wrap `CombinedOutput` with a size-limited reader.

## Phase 2: Migration (Short-term)
Identify all call sites of `NewSafeExecutor` using `grep`.
Replace them with `NewDirectExecutor`.
Note: This may require updating call sites to use the new `Command` struct instead of `ShellCommand`. The `ExecuteNew` bridge already exists on `SafeExecutor`, so we can flip the dependency injection.

## Phase 3: Elimination (Medium-term)
Delete `internal/tactile/executor.go`.
Remove `TestSafeExecutor` tests.
Remove `ShellCommand` struct (if no longer used).

# Hypothetical Mangle Policies

Instead of hardcoding `AllowedBinaries` in Go, we should define security policies in Mangle.

**Example `security.mg`:**
```mangle
Decl permitted(Action, Binary, Arguments).

permitted(run, "go", Args) :-
    not(contains(Args, "rm")).

permitted(run, "ls", _).

deny(run, Binary, _) :-
    not(permitted(run, Binary, _)).
```

The Executor would then query the Kernel:
`Query: permit(run, "bash", ["-c", "rm -rf /"])`
Result: `false` (because no rule permits `bash`).

This decouples policy from mechanism and allows dynamic updates.

# Comparative Analysis

| Feature | `SafeExecutor` (Legacy) | `DirectExecutor` (New) | `DockerExecutor` (Future) |
| :--- | :--- | :--- | :--- |
| **Default Policy** | Default-Allow (Insecure) | Default-Allow (relies on OS) | Isolated |
| **Env Handling** | Leaks Parent | Sanitized | Isolated |
| **Output Buffer** | Unbounded (OOM Risk) | Limited (Safe) | Limited |
| **Timeout** | Global Default | Configurable | Configurable |
| **Filesystem** | Host Access | Host Access | Containerized |
| **Maintainability** | Low (Hardcoded) | High (Config struct) | High |

**Verdict**: `SafeExecutor` is misnamed. It is the *least* safe executor available. Its continued existence is a liability. The negative tests we are adding will serve as the "nail in the coffin" to force its removal.

# Exploit Proof of Concepts (PoC)

## PoC 1: Environment Exfiltration

```go
package main

import (
	"context"
	"fmt"
	"os"
	"codenerd/internal/tactile"
)

func main() {
	// 1. Set sensitive environment in parent
	os.Setenv("AWS_SECRET_ACCESS_KEY", "AKIAIOSFODNN7EXAMPLE")

	// 2. Exploit SafeExecutor
	exec := tactile.NewSafeExecutor()
	cmd := tactile.ShellCommand{
		Binary: "env", // Dump all environment variables
		// EnvironmentVars: nil, // Implicitly inherits parent env
	}

	output, _ := exec.Execute(context.Background(), cmd)

	// 3. Check for leak
	if strings.Contains(output, "AKIAIOSFODNN7EXAMPLE") {
		fmt.Println("[CRITICAL] Leaked AWS Key!")
	}
}
```

## PoC 2: OOM DoS via Infinite Output

```go
package main

import (
	"context"
	"codenerd/internal/tactile"
)

func main() {
	exec := tactile.NewSafeExecutor()

	// 'yes' prints 'y' infinitely.
	// CombinedOutput will try to buffer infinite bytes.
	cmd := tactile.ShellCommand{
		Binary: "yes",
	}

	// This will block until memory exhaustion crashes the process
	exec.Execute(context.Background(), cmd)
}
```

## PoC 3: Filesystem Wipe via Bash Bypass

```go
package main

import (
	"context"
	"codenerd/internal/tactile"
)

func main() {
	exec := tactile.NewSafeExecutor()

	// Direct execution of 'rm' is blocked:
	// exec.Execute(ctx, ShellCommand{Binary: "rm", ...}) -> Error

	// Indirect execution via 'bash -c' is allowed:
	cmd := tactile.ShellCommand{
		Binary: "bash",
		Arguments: []string{"-c", "rm -rf /tmp/important_data"},
	}

	exec.Execute(context.Background(), cmd)
	// /tmp/important_data is now gone.
}
```

# Historical Context: Why is this here?

The `SafeExecutor` likely dates back to the project's inception ("VirtualStore"). At that time:
1.  **MVP Focus**: Speed of development was prioritized over rigorous security.
2.  **Assumption of Trust**: The system was likely designed for a single user or trusted environment where malicious inputs were not a threat model.
3.  **Go Stdlib Defaults**: `os/exec`'s default behavior (inherit env, unbound buffers) is convenient for scripting but dangerous for services.

As the system evolved to support Autopoiesis (self-modification) and multi-tenant campaigns, these assumptions became liabilities. The deprecation notice exists, but the code remains, likely due to deep dependencies in older test suites or legacy subsystems.

# Cross-Platform Considerations

## Windows vs Linux
*   **Environment**: Windows environment variables are case-insensitive. Leaking `Path` vs `PATH` is consistent.
*   **Shells**: `SafeExecutor` allows `cmd` and `powershell`. The bypass PoC becomes:
    *   `cmd /c del /F /Q C:\Windows\System32\drivers\etc\hosts`
*   **Filesystem**: Path traversal checks like `../../` work similarly, but Windows drive letters (`C:\`) introduce new bypass vectors if not handled (e.g. absolute paths).

## Mitigation Snippets (Immediate Patch)

If removal is impossible, apply this patch to `internal/tactile/executor.go`:

```go
func (e *SafeExecutor) Execute(ctx context.Context, cmd ShellCommand) (string, error) {
    // 1. Deny by default
    if allowed, exists := e.AllowedBinaries[cmd.Binary]; !exists || !allowed {
         return "", fmt.Errorf("binary not allowed")
    }

    // 2. Sanitize Environment
    if cmd.EnvironmentVars == nil {
        // Force empty, or copy only safe keys like PATH
        cmd.EnvironmentVars = []string{"PATH=" + os.Getenv("PATH")}
    }

    // ... setup command ...
    c.Env = cmd.EnvironmentVars

    // 3. Limit Output
    stdoutReader, _ := c.StdoutPipe()
    c.Start()

    // Read max 1MB
    limitedReader := io.LimitReader(stdoutReader, 1024*1024)
    output, _ := io.ReadAll(limitedReader)

    // Kill if still running
    c.Wait()
    return string(output), nil
}
```

This patch closes the most critical holes (Env Leak, Default Allow, OOM) while maintaining API compatibility.

---
**End of Expanded Journal Entry**
