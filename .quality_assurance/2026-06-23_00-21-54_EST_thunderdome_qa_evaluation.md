# Thunderdome QA Evaluation Journal
## Date: 2026-06-23 00:21:54 EST

### Introduction
The codeNERD Thunderdome system acts as the execution battleground for generated autopoiesis tools. Its core responsibility is compiling untrusted or unpredictable generated Go functions into binary wrappers and blasting them with user-supplied inputs and edge-case execution environments. The goal is to evaluate generated tools for Out of Memory (OOM) constraints, memory leaks, system isolation integrity, or panic behaviors.

A deep code-level dive of `internal/autopoiesis/thunderdome.go` and its associated `thunderdome_harness_test.go` reveals a moderately well-tested system. It currently covers common happy-path attacks such as newline errors, simple binary injections, basic memory explosion detection, large input lengths, and basic environment isolation.

However, from an adversarial QA, Boundary Value, and Negative Testing perspective, the Thunderdome test suite has critical, exploitable gaps across numerous edge-case axes. The underlying `thunderdome.go` implementation contains some robustness against these, but the current test assertions are entirely blind to them, leaving the door open to future regressions.

The evaluation below splits these findings by attack vector.

### 1. Null, Undefined, and Empty Strings and Types

**Analysis of Gaps:**
The simplest way to break poorly typed input systems is removing the input entirely, passing `nil`, or executing against empty interfaces. Thunderdome executes generated tools in a harness wrapper.
1. **The Nil Tool Problem:** The `Battle()` function signature is `(ctx context.Context, tool *GeneratedTool, attacks []AttackVector)`. If the `tool` argument is passed as `nil`, there are no current tests evaluating whether the system returns a safe operational error. If a transducer or generation loop glitches and passes `nil`, `t.prepareArena(ctx, tool)` will immediately panic with a `nil` pointer dereference on `tool.Name`. The system's resilience here is completely untested.
2. **The Empty Attack Vector:** The `attacks` array slice can be initialized but left completely empty (`[]AttackVector{}`). What happens? Current tests always pass a slice of size >= 1. The Thunderdome logic currently iterates via a `for _, attack := range attacks` loop, which gracefully handles 0 length by skipping and returning a survival status. However, there is no test verifying that this behavior hasn't broken or won't break in the future if initialization logic requires at least one execution to set baseline stats.
3. **The Zero-Byte Input Attack:** Empty input string attacks (`""`). Many string parsers, lexers, or split functions (like `strings.Split()`) behave oddly or panic when receiving a length zero string. If Thunderdome injects this empty slice through `stdin` via `io.LimitReader`, does the generated harness correctly execute the tool's entrypoint? A test must explicitly provide `Input: ""` and verify that a tool designed to panic on zero-length strings gets correctly killed, proving the harness invokes the underlying tool even with 0 bytes over stdin.

**Proposed Solution For Tests:**
- Add `TestThunderdome_NilToolHandling`: Verify `td.Battle(ctx, nil, attacks)` fails cleanly with an error and does NOT panic the main codeNERD execution thread.
- Add `TestThunderdome_EmptyAttacksArray`: Verify providing an empty slice array to `td.Battle` acts neutrally and returns survival without error.
- Add `TestThunderdome_EmptyStringInputAttack`: Test an attack vector with an empty string against a tool designed to read it to ensure the harness invokes the underlying tool even with 0 bytes over stdin.

### 2. Type Coercion Constraints

**Analysis of Gaps:**
Thunderdome's test harness strictly executes the string representation of attack vectors over standard input.
1. **Malformed Attack Vectors:** What occurs if `AttackVector` parameters are improperly hydrated due to a malformed deserialization upstream in the campaign orchestrator? If the `Name`, `Category`, or `Description` parameters of an `AttackVector` are missing, Thunderdome should still safely log or execute them. Currently, `runAttack` formats these for debug logging. A malformed struct could cause unpredictable string formatting issues.
2. **Invalid Function Signatures:** What if the Thunderdome tries to battle a generated tool whose function signature requires structured types (like a `struct` or `[]byte`) instead of `string`? In `findEntryPointCall`, the AST parsing heavily favors `string` inputs. But a tool could technically generate a different parameter type. The compilation step `cmd.Run()` inside `prepareArena()` should catch this type dissonance and report it as a tool failure instead of a Thunderdome kernel failure. However, there are zero negative tests covering tools with invalid or un-coercible function signatures.

**Proposed Solution For Tests:**
- Add `TestThunderdome_MalformedAttackVector`: Supply an attack vector struct with zeroed out inner fields to ensure Thunderdome executes it safely.
- Add `TestThunderdome_InvalidToolSignature`: Supply a `GeneratedTool` that expects `(ctx context.Context, data []byte)` instead of `(ctx context.Context, input string)` and verify Thunderdome handles the compilation failure correctly and rejects the tool instead of erroring out the main orchestration loop.

### 3. User Request Extremes

**Analysis of Gaps:**
The Thunderdome must handle massive payloads and system pressure.
1. **Massive Output Generation (The 1GB Attack):** While Thunderdome protects its internal harness execution memory via `debug.SetMemoryLimit`, the `cmd.Run()` step in the main process's `runAttack` redirects the child process output to `&stdout` and `&stderr` via unbounded `bytes.Buffer` objects. If the executed tool prints 1GB of text to stdout in a tight loop, the `bytes.Buffer` array living inside the main codeNERD host process will inflate infinitely and OOM the ENTIRE kernel, bypassing the Thunderdome containerization limit. This is a critical security and boundary testing gap.
2. **Configuration Bounds:** `NewThunderdomeWithConfig` can receive user-supplied boundary overrides for memory limits and timeouts. What if a user forces `config.MaxMemoryMB = 0` or `config.Timeout = 0`? The current Thunderdome tests do not evaluate these bounds. Inside the harness generator, setting the timeout to `0` may cause instant timeouts, and setting memory limit to `0` will force the GC to run instantly or crash the harness binary on startup.
3. **Infinite Compilation Attacks:** Thunderdome invokes the Go compiler via `exec.CommandContext(ctx, "go", "test", "-c", "-o", binaryPath, ".")`. A malicious tool could use advanced generic recursive types to force the Go compiler to hang infinitely. The test suite needs to ensure `prepareArena` uses a strict context timeout to kill runaway compilation.

**Proposed Solution For Tests:**
- Add `TestThunderdome_MassiveStdoutOutput`: Generate a tool that continuously dumps 100MB of string data to `fmt.Println()`. Verify the host system does not OOM and `runAttack()` gracefully caps the buffer via an `io.LimitReader` on the output pipes (this test will currently fail until the kernel is patched).
- Add `TestThunderdome_ZeroBoundsConfiguration`: Instantiate the system with zeroed out `ThunderdomeConfig` variables and test to see if it safely defaults them, executes instantly, or errors gracefully instead of causing undefined behavior.
- Add `TestThunderdome_CompilationTimeoutEnforcement`: Supply a tool that deliberately hangs the Go compiler and verify Thunderdome kills it gracefully.

### 4. State Conflicts and Race Conditions

**Analysis of Gaps:**
Thunderdome uses `t.config.ParallelAttacks` but tests primarily execute sequentially.
1. **Directory Collision:** `prepareArena()` generates temp directories using `tool.Name` and `time.Now().UnixNano()`. Under extremely high parallel load, or low-resolution system clocks, it is possible for two tools to collision-generate the same temporary directory. This is an OS level boundary check failure. It should use `crypto/rand` or a UUID system to guarantee mathematical uniqueness.
2. **Panic Swallowing:** What if a generated tool explicitly uses `defer func() { recover() }()` to hide its own panic and prevent Thunderdome from classifying it as dead? Thunderdome currently looks for "PANIC:" in stderr. If the tool eats its own panic and exits `0`, Thunderdome marks it as survived! A test must check if tools that swallow their own panics can mask test failures.
3. **Deep Recursion and Stack Overflows:** A tool might cause a stack overflow through infinite recursion. Go's runtime prints a very specific panic trace for this, which can sometimes bypass simple string matching if the output buffer is truncated.

**Proposed Solution For Tests:**
- Add `TestThunderdome_ConcurrentDirectoryCollision`: Stub out `time.Now()` or brute force parallel directory generations to prove uniqueness failure.
- Add `TestThunderdome_PanicSwallowingTool`: Generate a tool that panics, but intercepts it using `recover()`, to test if Thunderdome can detect these evasion tactics.
- Add `TestThunderdome_StackOverflowDetection`: Supply a deeply recursive tool and ensure Thunderdome accurately classifies the failure.

### Performance Analysis
Can Thunderdome handle these extremes?
Currently: No. The most critical vulnerability is the `stdout` buffering inside `runAttack()`. Because the `cmd.Stdout` is tied directly to a dynamically expanding `bytes.Buffer` on the parent process, a rogue tool generating continuous string output can trivially OOM the entire codeNERD system. The test suite needs to be updated to capture these failures so they can be patched upstream in the Thunderdome execution package. The boundary values on configurations (0 or negative ints) will also currently cascade unchecked into the Go text template for the test harness wrapper.

### Implementation Action Plan
To remediate this, I will add these gaps as `// TODO: TEST_GAP:` comments in `internal/autopoiesis/thunderdome_harness_test.go` so that the engineering team can begin developing specific assertions for them.

This journal entry serves as the official artifact for the QA automation audit of the Thunderdome system, validating that the tests must be expanded beyond happy-path scenarios.
