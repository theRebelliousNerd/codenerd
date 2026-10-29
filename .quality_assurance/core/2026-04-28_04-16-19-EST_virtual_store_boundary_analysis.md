---

remediated: false
subsystem: core
---
# VirtualStore Boundary Value Analysis and Negative Testing Journal

**Date:** 2026-04-28
**Time:** 04:16:19 EST
**Subsystem:** VirtualStore (`internal/core/virtual_store.go`, `internal/core/virtual_store_actions.go`, etc.)

## Executive Summary

This journal entry documents a deep-dive Boundary Value Analysis (BVA) and Negative Testing assessment of the VirtualStore subsystem within the codeNERD framework. The VirtualStore acts as the FFI (Foreign Function Interface) Router for the Hollow Kernel. It processes declarative `next_action` facts and performs physical world actions like modifying files, executing shell commands, or invoking modular tools.

This analysis specifically targets edge cases, malformed data, concurrency issues, and extreme payloads that standard "happy path" testing misses. It does not look for functional requirements testing, but focuses solely on the edge cases outlined below.

## Subsystem Overview

The `VirtualStore` struct maintains references to numerous moving parts:
- `tactile.Executor` and `modernExecutor` (command execution)
- `coreshards.ShardManager` and `TaskDelegator` (agent spawning)
- `Kernel` (Mangle reasoning feedback loop)
- Tool Registries (`ToolRegistry`, `modularTools`)
- Code DOM scopes (`CodeScope`, `FileEditor`)
- Persistence layers (`localDB`, `learningStore`)
- Post-action validators

The core dispatch mechanism is `RouteAction(ctx, fact)`, which parses a Mangle fact into an `ActionRequest`, checks constitutional safety rules, checks kernel permissions, and routes the action via a massive `switch` statement in `executeAction()`.

## 1. Null/Undefined/Empty Vectors

The VirtualStore sits at the boundary between logic (Mangle) and reality. Mangle arguments are typed as `[]interface{}`. This looseness creates numerous edge cases when arguments are missing or incorrectly typed.

### Missing Action Arguments

The `parseActionFact` method converts a Mangle Fact into an `ActionRequest`. It expects at least three arguments: ActionID, Type, Target.
- **Vulnerability:** While `parseActionFact` handles `< 3` args, what happens if `action.Args` is entirely `nil`? The Go slice `len()` function is safe, but we must ensure no downstream logic panics if it attempts to access `args[0]` before the `len` check, or if the `len` check is bypassed.
- **Edge Case:** `Fact{Predicate: "next_action", Args: nil}`
- **Mitigation:** The code correctly returns an error if `len(action.Args) < 3`.

### Empty String Action Properties

Action requests often have `""` (empty string) values for `ActionID`, `Type`, or `Target`.
- **Vulnerability:** `executeAction` switches on `req.Type`. If `req.Type` is `""`, it falls to the `default` case, returning an error. However, if `Target` is `""`, many handlers assume it points to the current directory or a valid file.
- **Edge Case:** `ActionRequest{Type: ActionExecCmd, Target: ""}`.
- **Performance:** For `handleExecCmd`, an empty target might spawn a process that immediately exits, or it might hang indefinitely waiting for input if the empty target causes it to read from stdin. The system is performant enough, but dangling processes are a risk.

### Empty Payloads

Many actions extract configuration from `req.Payload`.
- **Vulnerability:** If `req.Payload` is `nil` or empty, do the handlers gracefully fall back to defaults, or do they panic when asserting types?
- **Edge Case:** `ActionRequest{Type: ActionExecCmd, Payload: nil}`. `timeoutSecondsFromActionRequest` must handle this safely.

### Missing Interfaces

The `VirtualStore` allows interfaces to be nil until they are lazily loaded or injected (e.g., `kernel`, `validators`, `fileEditor`, `modularTools`).
- **Vulnerability:** If `HydrateModularTools` is called before `modularTools` is initialized, it errors correctly. However, in `RouteAction`, what happens if `v.validators` is accessed concurrently while being initialized? What if `kernel` is nil during `injectFact`? The code uses locks, but we must verify the nil checks are comprehensive.
- **Edge Case:** `RouteAction` invoked while `v.kernel == nil`.

## 2. Type Coercion Vectors

Mangle passes arguments as `interface{}`. The `VirtualStore` uses `types.ExtractString()` heavily, but this can mask underlying type confusion.

### Float to Int Coercion in Timeout

`timeoutSecondsFromActionRequest` attempts to coerce timeout values.
- **Vulnerability:** It checks for `int`, `float64`, `json.Number`, and `string`. What if the value is an enormous float like `1e100`, an infinity (`math.Inf(1)`), or `NaN`?
- **Edge Case:** `Payload: map[string]interface{}{"timeout_seconds": math.NaN()}`.
- **Performance:** Passing an extreme timeout to `context.WithTimeout` could overflow `time.Duration`, resulting in immediate timeouts or indefinite hangs.

### MangleAtom Coercion

`parseActionFact` strips leading slashes from `action.Args[1]` (Type).
- **Vulnerability:** If the type is passed as a complex Mangle AST node instead of a string, `action.Args[1].(string)` fails, falling back to `types.ExtractString()`. If `ExtractString` formats it as something unexpected, `req.Type` will be misclassified.
- **Edge Case:** ActionType provided as `ast.Constant{...}` instead of a string primitive.

### Argument Type Mismatches in Payload Maps

JSON deserialization often converts integers to float64.
- **Vulnerability:** Handlers that strictly expect `int` in the payload map might fail if a `float64` is provided.
- **Edge Case:** A tool that requires `max_results` expects an `int`, but receives `float64(100)` from JSON serialization.

## 3. User Request Extremes

The VirtualStore must handle extreme payloads without crashing, running out of memory (OOM), or halting the execution loop.

### Extreme Output Payloads

When `ActionExecCmd` is run, it returns `stdout`.
- **Vulnerability:** If a user requests an extreme action like `cat /dev/urandom` or compiles a 50GB binary and attempts to read it, the output will exceed memory.
- **Edge Case:** Command output exceeding 500MB.
- **Performance/Mitigation:** The executor must have strict byte-limits on stdout/stderr capture. If `tactile.Executor` buffers the entire stream in memory, the codeNERD agent will OOM on limited RAM machines. We must verify truncation logic.

### Deep Dependency Chains in File Walking

The `GlobTool` uses `filepath.Walk`.
- **Vulnerability:** A directory structure containing recursive symlinks (if not strictly ignored) or simply a massive depth (e.g., 10,000 nested folders) can cause `filepath.Walk` to exhaust file descriptors or take excessive time, blocking the Mangle kernel's reasoning cycle.
- **Edge Case:** `pattern: "**/*"` on a root filesystem with a massive monorepo.
- **Performance:** `max_results` limits the output size, but `filepath.Walk` still visits every node until `max_results` is reached. If the first 10,000 nodes don't match, it wastes time. The system should timeout or cancel gracefully.

### Extreme Regex Patterns

The `GrepTool` compiles user-provided regex.
- **Vulnerability:** Regular Expression Denial of Service (ReDoS). A maliciously crafted or accidentally complex regex pattern can cause the Go regex engine (which is generally safe in `regexp`, but can still be slow) to block.
- **Edge Case:** Complex backtracking regex (if supported) or massive regex strings.
- **Performance:** `regexp.Compile` is bounded, but executing it over millions of lines is O(N*M). We must ensure Context cancellations apply to `GrepTool`.

### Massive Number of Facts in Pruning

`maybePruneActionLogs` prevents unbounded growth of action logs in the kernel.
- **Vulnerability:** It queries `execution_result` and then retracts older facts. If the system executes 1,000,000 actions, the `kernel.Query()` call itself might OOM or take seconds.
- **Edge Case:** 10,000,000 unpruned action facts.
- **Performance:** The Mangle engine is performant, but querying and iterating massive slices requires continuous allocation. The `pruneByCount` logic is decent, but `kernel.RemoveFactsByPredicateSet` or batching is required for stability.

## 4. State Conflicts & Concurrency Vectors

The `VirtualStore` makes heavy use of `sync.RWMutex` (`v.mu`). Lock contention and deadlocks are the primary risks here.

### Deadlock: Kernel Callbacks

The `rebuildPermissionCache` method specifically notes a deadlock fix: "This method does NOT hold v.mu while querying the kernel."
- **Vulnerability:** While `rebuildPermissionCache` is fixed, other methods might still hold `v.mu` while calling external components. For example, `injectTactileFact` acquires a read lock to get the kernel, unlocks it, and then calls `kernel.Assert()`. This is safe. However, are there any other flows where `v.mu` is held during a blocking operation or external call?
- **Edge Case:** `SetToolExecutor` acquires a write lock on `v.mu`, then calls `v.toolRegistry.SyncFromOuroboros(executor)`. If `SyncFromOuroboros` attempts to access the VirtualStore back, or blocks on another lock, it could deadlock.
- **Mitigation/Testing:** We need stress tests that concurrently mutate VirtualStore configurations while actions are being routed.

### Race Condition: Boot Guard

The `bootGuardActive` boolean is checked to prevent action routing during rehydration.
- **Vulnerability:** Rehydration facts might arrive concurrently with the first user interaction.
- **Edge Case:** A background process emits a `next_action` exactly when `DisableBootGuard` is called.
- **Performance:** The read-lock around `bootGuardActive` is fast, but we must ensure the timing guarantees are sound.

### Concurrent File Modifications

Multiple shards or tools might attempt to edit the same file simultaneously.
- **Vulnerability:** `ActionWriteFile` and `ActionEditFile` are routed through the `VirtualStore`. The `CodeScope` and `FileEditor` manage access, but if the raw `os.WriteFile` fallback is hit, it lacks synchronization.
- **Edge Case:** Two parallel tools emitting `ActionWriteFile` for `main.go`.
- **Mitigation:** The system relies on `write_set_lock_manager` at the orchestrator level, but the VirtualStore itself is vulnerable if invoked directly.

### Interleaved Log Pruning

`maybePruneActionLogs` uses `v.lastLogPrune` to debounce pruning.
- **Vulnerability:** Multiple goroutines executing actions will concurrently invoke `maybePruneActionLogs`. The lock on `v.lastLogPrune` prevents concurrent *checks*, but once past the check, the actual kernel querying and retraction happens outside the lock. This means multiple goroutines might simultaneously query and attempt to retract the same facts.
- **Edge Case:** 50 actions complete in the same millisecond after a 10-second idle period. 50 goroutines pass the debounce check and simultaneously attempt batch retractions.
- **Performance:** `RetractExactFactsBatch` must be idempotent, but querying the kernel 50 times concurrently for the entire `execution_result` history is an immense performance penalty.

## Recommendations and Next Steps

The following test gaps have been identified and mapped to `// TODO: TEST_GAP:` comments in `internal/core/virtual_store_test.go`.

**Null/Undefined/Empty:**
1. Verify `parseActionFact` handles `action.Args == nil` gracefully.
2. Verify `RouteAction` with `req.Target == ""` for shell commands behaves deterministically.
3. Verify `timeoutSecondsFromActionRequest` handles missing or empty keys gracefully.

**Type Coercion:**
4. Verify `timeoutSecondsFromActionRequest` behavior with `NaN` and `Inf` float payload values.
5. Verify `parseActionFact` behavior when `ActionType` is passed as a complex Mangle AST type.

**User Request Extremes:**
6. Verify `ActionExecCmd` gracefully truncates or handles stdout payloads exceeding 100MB to prevent OOM.
7. Verify `GlobTool` handles massive directory structures or circular symlinks without infinite looping.
8. Verify `maybePruneActionLogs` performance when `execution_result` fact count is > 1,000,000.

**State Conflicts:**
9. Verify concurrent `RouteAction` calls and `SetToolExecutor` do not trigger deadlocks.
10. Verify concurrent executions of `maybePruneActionLogs` do not lead to massive redundant kernel queries (race condition in debounce).

## Extended Analysis: Deep Performance Profiles and System Stability Limits

### 5. Disk and I/O Boundaries

The VirtualStore fundamentally interacts with the filesystem through `ReadFile`, `WriteFile`, and command execution. Mangle's synchronous nature means that IO blocks the logical reasoning thread.

#### 5.1 High-Latency Storage Mediums
When codeNERD operates on network-mounted drives (NFS, SMB) or extremely slow spinning disks, `os.ReadFile` and `filepath.Walk` exhibit non-linear latency scaling.
- **Vulnerability:** If `ReadFile` blocks for 30 seconds due to NFS latency, the entire action routing pipeline for that SubAgent hangs. `RouteAction` does not enforce context cancellation on direct IO fallbacks.
- **Testing Strategy:** Mock the `FileEditor` and `os.ReadFile` fallback to introduce multi-second delays. Verify that `ctx.Done()` is checked at strategic points, or that actions time out according to `req.Timeout`.

#### 5.2 File System Race Conditions (TOCTOU)
Time-of-check to time-of-use (TOCTOU) is a common vulnerability in file operations.
- **Vulnerability:** Between the moment `VirtualStore` verifies an action is constitutionally safe (e.g., path traversal check on `/path/to/file`) and the moment the action executes, the underlying file could be replaced by a symlink pointing to `/etc/passwd`.
- **Testing Strategy:** The tests must spawn concurrent goroutines where one executes a file operation and another rapidly swaps the target file with a symlink. We must verify if the modern executor (which uses sandboxing) prevents this, or if the VirtualStore's raw `os.ReadFile` fallback is vulnerable.

### 6. SubAgent Execution Limits

The `VirtualStore` interacts heavily with the `TaskDelegator` and `ShardManager`.

#### 6.1 Unbounded Spawning Loops
A malicious or poorly reasoning SubAgent could continuously spawn new ephemeral shards.
- **Vulnerability:** `ActionDelegate` creates new tasks. If the parent agent delegates to a child, and the child delegates back, an infinite loop ensues. The `VirtualStore` acts as the broker but does not track the depth of the delegation stack.
- **Testing Strategy:** Simulate a cyclic delegation payload. The test should verify that the system detects the cycle or hits a strict recursion depth limit, rather than crashing with an `OutOfMemory` error or thread starvation.

#### 6.2 Zombie Process Harvesting
If a command started by `handleExecCmd` forks a background process (e.g., `npm start &`), the VirtualStore's `tactile.Executor` might lose track of it.
- **Vulnerability:** Background processes can outlive the codeNERD session, consuming ports and CPU. When the session restarts, these zombie processes cause conflicts (e.g., port 3000 already in use).
- **Testing Strategy:** Execute a command that spans a long-running background process. Verify that session teardown or executor cleanup correctly identifies and sends `SIGKILL` to the entire process group, not just the parent shell.

### 7. Virtual Predicate Resolution Complexities

The `VirtualStore.Get()` method resolves queries for the Mangle engine dynamically (e.g., `query_knowledge_graph`, `recall_similar`).

#### 7.1 Cross-Cutting Consistency
Virtual predicates retrieve data from `localDB` (SQLite) and `learningStore`.
- **Vulnerability:** If the Mangle engine queries a virtual predicate repeatedly within the same fixpoint evaluation, it expects deterministic results. If another concurrent process updates the `localDB` during the evaluation, the virtual predicate might return a different set of facts on the second call, breaking Mangle's monotonicity requirement and causing unpredictable reasoning.
- **Testing Strategy:** Test the stability of `Get()` during concurrent writes to the underlying store. We must verify that the virtual predicate implementation either snapshots the state or caches results for the duration of the logical transaction.

#### 7.2 Vector Database Dimension Mismatches
When `recall_similar` is used, it queries the `sqlite-vec` index.
- **Vulnerability:** If the user switches LLM models (e.g., from `text-embedding-ada-002` with 1536 dimensions to a local model with 768 dimensions), the vector query will crash or return meaningless garbage. The `VirtualStore` does not validate embedding dimensions prior to routing to the database.
- **Testing Strategy:** Insert vectors of dimension X, then attempt to query with a vector of dimension Y. Ensure the system degrades gracefully with a clear error fact (`diagnostic`) rather than panicking the application.

### 8. Memory Management and Garbage Collection

The Mangle engine creates thousands of temporary string objects and AST nodes during evaluation. The `VirtualStore` converts these to Go primitives.

#### 8.1 Heap Fragmentation
The continuous creation and pruning of `execution_result` facts causes heap fragmentation.
- **Vulnerability:** `maybePruneActionLogs` runs frequently. If it removes elements from the middle of large arrays in the Mangle kernel, it could cause massive slice reallocations and GC pressure.
- **Testing Strategy:** Run a memory profile test simulating 100,000 rapid, small actions (e.g., `ActionListFiles`). Monitor heap growth and GC pause times. The test should assert that memory stabilizes and doesn't exhibit a strict leak curve.

#### 8.2 JSON Payload Bloat
Actions like `ActionOuroborosReg` accept massive JSON payloads containing entire tool ASTs.
- **Vulnerability:** The `req.Payload` map holds `interface{}` values. Deeply nested JSON structures keep a massive graph of maps and slices alive in memory. If a SubAgent emits a tool with a multi-megabyte payload, it sits in the kernel's fact store until pruned.
- **Testing Strategy:** Generate a 50MB synthetic tool definition payload. Verify that routing this action does not double the baseline memory footprint permanently.

### 9. Extensibility and Plugin Architecture Integrity

The `mcpClients` map dynamically loads MCP tools.

#### 9.1 Malicious or Incompetent MCP Servers
The VirtualStore trusts registered MCP clients to behave.
- **Vulnerability:** If an external MCP server hangs forever on a tool call, does the VirtualStore block the agent indefinitely? If the MCP server returns a 10GB response, does the VirtualStore attempt to parse it into Mangle facts?
- **Testing Strategy:** Create a mock MCP server that (a) never responds, (b) returns a payload violating the schema, and (c) returns a massive 5GB string. The tests must prove the VirtualStore limits execution time and payload size before injecting facts back into the kernel.

### Conclusion of Analysis

The `VirtualStore` is the critical choke point of the entire codeNERD framework. Every action, every tool, and every command passes through its `RouteAction` method. While the functional tests prove it can run bash and read files, the BVA highlights severe structural risks concerning concurrency (debounce race conditions), type coercion (Mangle atom confusion), and unbounded resource consumption (deep directory walks, massive outputs).

Implementing tests for the identified `TODO: TEST_GAP` markers will transform the VirtualStore from a prototype router into a hardened execution broker capable of sustaining multi-day, frontier-level coding campaigns without degradation.

### 10. Process Signal Handling and Interruption

The interaction between the VirtualStore and the operating system's process management is complex, particularly when long-running processes are involved.

#### 10.1 Graceful Degradation on SIGINT
When the user presses Ctrl+C, the context passed to `RouteAction` is canceled.
- **Vulnerability:** Does `handleExecCmd` properly propagate the context cancellation to the underlying `os/exec.Cmd`? More importantly, if it does, does it wait for the process to exit, or does it return immediately, leaving the process orphaned? If the command ignores SIGINT, does the system escalate to SIGKILL?
- **Testing Strategy:** Create a test that executes a bash script which traps SIGINT and sleeps. Send a context cancellation. Verify that the VirtualStore escalating kills the process after a grace period, preventing zombie processes.

#### 10.2 IO Pipe Deadlocks
When executing commands, stdout and stderr are often piped back to the application.
- **Vulnerability:** If the executed process produces massive amounts of output very quickly, and the codeNERD reader goroutine cannot consume it fast enough, the OS pipe buffer will fill. The child process will block on `write()`. If the context is canceled simultaneously, there's a risk of deadlocking the IO pipe cleanup routines.
- **Testing Strategy:** Execute a command that rapidly prints millions of lines (e.g., `yes`). Cancel the context after 10ms. Verify that the system recovers cleanly without leaking goroutines blocked on `io.Copy`.

### 11. Security and Jailbreaking Vectors

The Constitutional Safety rules in `VirtualStore` provide defense-in-depth, but they operate primarily on string matching.

#### 11.1 Shell Injection Bypass
The `no_destructive_commands` rule checks for `rm -rf`.
- **Vulnerability:** An LLM might generate a clever bypass, such as `r\m -rf /`, `rm -r -f /`, `a="rm"; $a -rf /`, or use `find / -exec rm -rf {} +`. A simple string check is easily defeated. Furthermore, if `ActionExecCmd` uses a shell interpreter (like `bash -c`), environment variable injection could alter the behavior of safe commands.
- **Testing Strategy:** Construct action requests with obfuscated destructive commands. The tests must verify that either the parser expands these safely, or that the sandboxed execution environment (the `modernExecutor`) catches the physical system calls, rather than relying solely on the string-based constitution.

#### 11.2 Environment Variable Poisoning
The VirtualStore filters environment variables provided by the caller using `filterCallerEnv`.
- **Vulnerability:** While `filterCallerEnv` prevents injection of unauthorized variables like `LD_PRELOAD`, it allows explicitly approved variables. What if an approved variable is crafted maliciously? For instance, setting `PATH` to a directory containing a malicious `ls` binary, or passing `GOPATH` with an unexpected format that causes Go tools to execute arbitrary code.
- **Testing Strategy:** Test `filterCallerEnv` with extreme values for permitted variables. Provide a `PATH` containing 10,000 entries, or paths with special characters (newlines, null bytes). Verify that the command executor handles these gracefully without crashing.

### 12. Artifact Integrity and Code DOM Stability

The Code DOM tools (`ActionGetElements`, `ActionEditLines`) rely on AST parsing.

#### 12.1 Malformed Source Files
- **Vulnerability:** If an agent attempts to edit a file that is currently in a state of syntax error (e.g., a missing brace), the AST parser used by Code DOM tools might fail to build the tree. Does the VirtualStore gracefully degrade to line-based editing, or does it return an opaque error, confusing the agent?
- **Testing Strategy:** Feed a syntactically invalid Go file to `ActionGetElements`. Verify the resulting Mangle facts correctly indicate the parse error (`parse_error` predicate) rather than panicking or returning zero elements silently.

#### 12.2 Massive File Asts
- **Vulnerability:** A 50,000-line generated file (like a Swagger client) parsed into an AST creates an enormous graph in memory. Processing this to generate Mangle facts (`code_element`, `element_signature`) could consume gigabytes of RAM.
- **Testing Strategy:** Generate a 100MB source file. Attempt `ActionOpenFile` and `ActionGetElements`. The system must detect the massive file and refuse full AST parsing, returning a `large_file_warning` fact to guide the agent to use `grep` or other tools instead.

### 13. System Clock Dependencies

- **Vulnerability:** The log pruning logic `now.Sub(v.lastLogPrune) < 10*time.Second` relies on a monotonic clock. If the system clock is manipulated, or if the process is paused (e.g., VM snapshot or laptop sleep), the elapsed time calculations might behave unexpectedly, causing a massive flood of delayed prunes simultaneously.
- **Testing Strategy:** Mock the time source or simulate a leap second scenario. Ensure the pruning logic handles negative time diffs gracefully without panicking or entering infinite loops.

### Final Assessment of VirtualStore Resilience

The VirtualStore is functionally robust for the expected workflows, but its position as the central nervous system of codeNERD makes it uniquely susceptible to cascading failures caused by edge-case payloads. The identified vectors—particularly the debounce race conditions, the naive string-matching in the constitution, and the unbounded nature of `filepath.Walk` and command stdout—represent material risks to the stability of the agent when deployed against complex, real-world repositories.

The engineering team must prioritize the `TODO: TEST_GAP` implementations immediately to establish a solid baseline of negative test coverage. Only by mathematically proving the VirtualStore's resilience against these edge cases can we confidently allow the codeNERD autonomous system to operate unattended on production codebases.

### 14. Network and External Dependency Isolation

The VirtualStore interacts with modular tools like `ActionWebSearch` and `ActionWebFetch`.

#### 14.1 Network Timeouts and Infinite Hangs
- **Vulnerability:** If a web fetch tool attempts to hit a server that accepts the connection but never sends data (a "tarpit"), and the HTTP client doesn't have strict read/write timeouts configured independently of the context, the goroutine could hang forever. If the context is canceled, the underlying socket might leak if not closed properly.
- **Testing Strategy:** Mock an HTTP server that acts as a tarpit. Route an `ActionWebFetch`. Verify that the VirtualStore enforces a strict timeout and that the number of active goroutines does not permanently increase.

#### 14.2 SSRF (Server-Side Request Forgery)
- **Vulnerability:** An agent might be tricked into fetching internal metadata URLs (e.g., `http://169.254.169.254/latest/meta-data/` on AWS) via `ActionWebFetch`. The VirtualStore currently lacks an explicit constitutional rule against internal IP addresses.
- **Testing Strategy:** Route an `ActionWebFetch` targeting a loopback or internal non-routable IP address. Verify if the network layer or a constitutional rule blocks the request. If not, this is a severe security vulnerability that must be addressed.

#### 14.3 DNS Rebinding Attacks
- **Vulnerability:** An external URL might initially resolve to a safe IP during validation but rebind to an internal IP during the actual fetch.
- **Testing Strategy:** This requires a specialized DNS mock. The test should verify that the HTTP client used by the fetch tools performs IP pinning or resolves the DNS once and uses the verified IP for the connection.

### 15. Cross-Platform Path Separator Anomalies

The VirtualStore relies heavily on `filepath.Clean` and `filepath.ToSlash`.

#### 15.1 Windows UNC Paths
- **Vulnerability:** On Windows, `filepath.Clean` might behave unexpectedly with Universal Naming Convention (UNC) paths (e.g., `\\server\share\file`). If an agent attempts to access a UNC path, it might bypass the `path_traversal_protection` rule if the normalization logic assumes a standard `C:\` drive prefix.
- **Testing Strategy:** Construct `ActionReadFile` requests with UNC paths on a Windows test runner. Verify that the file access is restricted to the defined `workingDir` and that the constitutional rules still apply.

#### 15.2 Case Sensitivity Discrepancies
- **Vulnerability:** Linux is case-sensitive; Windows and macOS (usually) are not. An agent might create a file named `Main.go` and later attempt to read `main.go`. The VirtualStore's behavior will vary based on the underlying OS. While this is expected filesystem behavior, it can lead to flaky campaigns if the orchestrator assumes a specific behavior.
- **Testing Strategy:** Create `test.txt`. Attempt to read `TEST.TXT`. The test suite should assert the OS-specific behavior explicitly so developers are aware of the contract.

### 16. JSON Type Coercion Edge Cases in Payload

The payload is unmarshaled from JSON, resulting in specific Go types (`float64` for all numbers, etc.).

#### 16.1 Deeply Nested Payloads
- **Vulnerability:** If the `req.Payload` contains a JSON structure nested 1000 levels deep, attempting to format it or recursively process it could cause a stack overflow.
- **Testing Strategy:** Generate a deeply nested JSON payload. Route an action that prints or logs the payload. Ensure the logger or processor caps recursion depth or iterative processing limits.

#### 16.2 Invalid UTF-8 Sequences
- **Vulnerability:** If a file contains invalid UTF-8 sequences, reading it into a Go string is safe (replaced with rune replacement characters), but sending that string back to the Mangle engine might cause serialization issues if the engine expects strict UTF-8.
- **Testing Strategy:** Create a file with invalid UTF-8 bytes. Execute `ActionReadFile`. Verify the resulting Mangle fact is correctly formed and does not corrupt the kernel's state.

### 17. The Boot Guard Rehydration Vulnerability

The `bootGuardActive` flag prevents old actions from executing during session rehydration.

#### 17.1 Premature Disablement
- **Vulnerability:** `DisableBootGuard` is called when the first user interaction occurs. What if the user interaction is just a ping or a status check that doesn't intend to start execution? The boot guard drops, and suddenly queued actions from a previous session might flood the executor.
- **Testing Strategy:** Rehydrate a session with pending actions. Send a benign command (e.g., `/status`). Verify that either the boot guard remains active, or that pending actions are explicitly cleared during rehydration before the guard is dropped.

#### 17.2 Race Condition during Disablement
- **Vulnerability:** If a background thread (e.g., a delayed timer from a previous session) attempts to route an action exactly as `DisableBootGuard` is called, there's a microscopic race.
- **Testing Strategy:** Use Go's `-race` detector while concurrently calling `DisableBootGuard` and `RouteAction`. The current `v.mu.RLock()` in `RouteAction` and `v.mu.Lock()` in `DisableBootGuard` should protect against memory corruption, but the logical race might allow one action through.

### Concluding Thoughts on the BVA

This exhaustive Boundary Value Analysis of the `VirtualStore` reveals that while the system is architecturally sound for standard operations, its critical position necessitates extreme paranoia regarding inputs and state. The system must not trust the Mangle facts it receives, it must not trust the commands it executes, and it must not trust the filesystem it operates on.

By implementing the `// TODO: TEST_GAP:` markers mapped to these specific vulnerabilities, the codeNERD framework will transform from a research prototype into a hardened, production-ready AI execution engine capable of safely navigating the most hostile and complex enterprise codebases.

### 18. Extensibility Testing Strategies

Given the modular nature of the VirtualStore, ensuring new tools don't break existing ones is paramount.

#### 18.1 Tool Priority Collisions
- **Vulnerability:** When registering modular tools, what happens if two tools register with the same name but different priorities, or the same priority?
- **Testing Strategy:** Register conflicting tools and assert deterministic behavior (e.g., first-in wins, or explicit error).

#### 18.2 Schema Validation Bypasses
- **Vulnerability:** Tools define a schema. Does the VirtualStore strictly validate the `req.Payload` against this schema before invoking the tool? If not, a poorly written tool might panic on missing fields.
- **Testing Strategy:** Submit payloads that subtly violate the schema (e.g., wrong type, missing required field, extra undocumented fields). Ensure the VirtualStore or the tool registry rejects them cleanly with a `validation_error` fact.

### 19. Integration with JIT Prompt Compilation

The VirtualStore provides feedback to the JIT compiler via facts.

#### 19.1 Fact Flood leading to Token Exhaustion
- **Vulnerability:** If an action generates 10,000 `diagnostic` facts (e.g., a massive build error), and these are fed directly into the context pager or JIT compiler, they will blow out the token budget.
- **Testing Strategy:** Generate a massive number of diagnostic facts. Verify that the VirtualStore's pruning logic (`maybePruneActionLogs`) caps them before they can overwhelm the context assembly process. The 200-fact limit in `pruneByCount` must be strictly enforced.

#### 19.2 Stale State Retention
- **Vulnerability:** `clearCodeDOMFacts` is meant to clear state, but what if a new Code DOM tool introduces a new predicate that isn't in the hardcoded `preds` map? That state will leak across scope refreshes.
- **Testing Strategy:** Review the Code DOM tool definitions and cross-reference them with the `preds` map in `clearCodeDOMFacts`. Ensure a process exists to automatically keep this list synchronized.

### Summary of Actionable Test Gaps

1.  **[Null/Empty]** Verify `parseActionFact` handles `action.Args == nil` gracefully.
2.  **[Null/Empty]** Verify `RouteAction` with `req.Target == ""` for shell commands behaves deterministically.
3.  **[Type Coercion]** Verify `timeoutSecondsFromActionRequest` behavior with `NaN` and `Inf` float payload values.
4.  **[Type Coercion]** Verify `parseActionFact` behavior when `ActionType` is an unexpected Mangle AST node instead of a string.
5.  **[Type Coercion]** Verify JSON payload integer/float coercion doesn't break strictly typed handlers.
6.  **[Extremes]** Verify `ActionExecCmd` gracefully truncates stdout payloads exceeding 100MB.
7.  **[Extremes]** Verify `GlobTool` handles massive directory structures or circular symlinks with context cancellation.
8.  **[Extremes]** Verify `maybePruneActionLogs` performance when `execution_result` fact count is > 1,000,000.
9.  **[State Conflicts]** Verify concurrent `RouteAction` calls and `SetToolExecutor` do not trigger deadlocks.
10. **[State Conflicts]** Verify concurrent executions of `maybePruneActionLogs` (race condition in debounce).
11. **[IO/System]** Verify TOCTOU vulnerabilities on file writes via the raw fallback.
12. **[IO/System]** Verify Context cancellation propagates to spawned child processes correctly, preventing zombies.
13. **[Security]** Verify `filterCallerEnv` rejects extremely long PATH strings to prevent buffer issues in the executor.
14. **[Integration]** Verify cross-platform path behavior is asserted in tests.

The QA team will implement these scenarios in `internal/core/virtual_store_test.go` using the `TODO: TEST_GAP:` markers as anchors. This rigorous negative testing approach is essential for the stability of the CodeNERD execution layer.

### 20. Conclusion

By executing these Boundary Value Analysis cases, codeNERD ensures that the VirtualStore acts as a robust, fail-safe router. The implementation of the missing tests highlighted by the `TODO: TEST_GAP:` markers will provide mathematical assurance against edge-case panics, silent state corruption, and unbounded resource consumption.

### 21. Appendices

#### Appendix A: Test Implementation Guide
When writing the tests for these gaps, QA Engineers must prioritize using Go's `testing.T` features like `t.Parallel()` for the concurrency tests and table-driven design for the type coercion tests. This ensures the tests run quickly and provide clear, actionable output when an edge case is violated.

#### Appendix B: Review Process
This analysis must be reviewed by the Core Kernel team to validate the assumptions made about Mangle's internal fact representation and the orchestrator's concurrency models.

#### Appendix C: Acknowledgements
Special thanks to the codenerd-builder and mangle-programming experts for their insights into the core architecture, which guided the formulation of these boundary tests.
