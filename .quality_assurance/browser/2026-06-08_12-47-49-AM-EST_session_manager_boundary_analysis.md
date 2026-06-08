# Boundary Value Analysis & Negative Testing: Browser Session Manager

**Date:** 2026-06-08 00:47:49 EST
**Subsystem:** `internal/browser/session_manager.go`

## 1. System Overview & Context
The `internal/browser` package provides an integration layer between CodeNERD's reasoning engine (Mangle) and a headless Chrome instance via the `go-rod/rod` library. It acts as the physical actuator for web-based actions. The `SessionManager` struct acts as the core orchestrator, responsible for launching Chrome (`EnsureStarted`), attaching to Chrome CDP targets, managing isolated `Session` records (which contain references to `rod.Page`), routing DOM events (throttled) into the Mangle engine, and serializing session metadata to disk for persistence.

The system relies heavily on `go-rod`'s built-in timeout and cancellation primitives, but the wrapper layer (`SessionManager`) introduces its own state map (`m.sessions`) and synchronization (`m.mu`). Because this subsystem bridges a deterministic logic engine with an inherently unpredictable, asynchronous, and stateful external process (a web browser), boundary value analysis and negative testing are critical.

## 2. Evaluation of Existing Tests

The existing test suite (`session_manager_coverage_test.go`, `start_coverage_test.go`, `lifecycle_coverage_test.go`) is quite extensive, sitting at ~1100 lines. It achieves high statement coverage by explicitly verifying:
- Happy paths for JSON serialization/deserialization of config and session state.
- Throttling logic (allowing vs denying requests based on timestamps).
- Coalescing console arguments and identifying internal vs external scripts.
- Graceful degradation when `sink` (engine) is nil.

However, the suite is highly mocked and focused on structural unit tests. It completely ignores chaotic environmental factors, concurrent stress, boundary inputs from Mangle, and edge cases inherent to browser automation. The tests ensure the Go code doesn't panic when an internal map is empty, but they do not ensure the system survives malicious or malformed inputs from the LLM or network disconnects.

## 3. Identified Gaps & Boundary Vectors

### Vector A: Null / Undefined / Empty Inputs
The system must safely handle malformed configurations or empty strings passed to critical functions, especially since LLMs often hallucinate missing arguments.

1. **Empty/Malformed Debugger URL:** What happens if `Config.DebuggerURL` is passed as a string containing non-URL bytes or control characters? The `EnsureStarted` path might crash the underlying WebSocket dialer instead of returning a clean error.
2. **Nil Context in Core Methods:** Methods like `Start`, `CreateSession`, and `Attach` accept a `context.Context`. If `nil` is passed (perhaps due to a bug in the virtual store execution layer), `rod` operations or HTTP requests will panic.
3. **Empty Target ID / URL:** `Attach(ctx, targetID)` and `CreateSession(ctx, url)` with empty strings `""`. Creating a session on `""` might succeed in Chrome (defaulting to `about:blank`), but it's an edge case that should be explicitly tested and normalized. Attaching to an empty target ID will likely fail, but does the error bubble up correctly without leaving dangling state?
4. **Empty Config Arrays:** `Config.Launch` being nil or an empty slice. Will the default `rod` launcher inject necessary flags (like `--no-sandbox` if in a container), or will it fail to boot?

### Vector B: Type Coercion & Malformed Data
Data crossing the boundary from disk or Mangle into the session manager.

1. **Corrupted Session Store JSON:** `loadSessions` reads from `cfg.SessionStore`. What if the file exists but contains a massive 5GB JSON array? Or JSON that uses numbers where strings are expected (e.g., `{"id": 123}` instead of `{"id": "123"}`)? The standard `json.Unmarshal` might fail, but does it truncate the file, crash the boot sequence, or just silently return an empty session list?
2. **Invalid Port Numbers:** If a user specifies a massive integer for `NavigationTimeoutMs` that overflows standard duration conversions, or negative numbers for viewport dimensions.
3. **Mangle Type Dissonance:** The engine adapter takes `[]mangle.Fact`. If the LLM generates a fact using string types instead of required Atom types (e.g., `"active"` instead of `/active`), does the adapter safely reject it, or does it pollute the knowledge base?

### Vector C: User Request Extremes (Load & Stress)
Evaluating the system's ability to handle frontier-level workloads or adversarial load.

1. **Massive DOM Ingestion:** If `EnableDOMIngestion` is true, and the session navigates to an infinitely scrolling page or a maliciously crafted 500MB DOM tree. The event throttler limits event frequency, but the payload size of a single `page.HTML()` call could OOM the Go process before it even reaches the Mangle engine.
2. **Connection Leak Exhaustion:** Creating 10,000 concurrent `Session` instances via `CreateSession` in a tight loop. Chrome CDP has limits. Does `go-rod` panic, or does `SessionManager` safely return an error, and are partially created sessions cleaned up?
3. **Event Throttler Hash Collision / Map Bloat:** The `eventThrottler` uses a `map[string]time.Time`. If an adversarial script logs millions of unique strings to the console, the `last` map will grow unbounded, causing a slow memory leak over the lifetime of the application.
4. **Rapid Mount/Dismount:** Constantly calling `Start` and `Shutdown` in parallel across multiple goroutines to trigger race conditions in the `EnsureStarted` singleton logic.

### Vector D: State Conflicts & Race Conditions
The browser is an independent process. State can diverge from the `SessionManager`'s internal map.

1. **Zombie Sessions:** If a user manually closes a tab in the headful Chrome instance (or a script calls `window.close()`), the `rod.Page` is destroyed. The `SessionManager`'s `sessions` map will still contain the `sessionRecord`. A subsequent call to `Page(sessionID)` will return a dead page, and attempting to use it will result in a `rod.ErrInvisible` or broken pipe. The system needs a heartbeat or a target-destroyed event listener to prune dead sessions.
2. **Network Partitions (CDP Disconnect):** If the headless Chrome process crashes (e.g., OOM killer) or the WebSocket connection drops silently, `IsConnected()` might return false, but are the sessions evicted? Will `Shutdown` block forever trying to close already-dead targets?
3. **Concurrent Mutation of Session Metadata:** `UpdateMetadata` uses the `SessionManager`'s `mu` (RWMutex). However, the `Session` struct is passed by value. If a massive burst of events attempts to update metadata simultaneously, the mutex prevents data races, but the order of updates is non-deterministic.
4. **Race Condition in Persistence:** If `Shutdown` (which calls `persistSessions`) races with another goroutine calling `CreateSession`, the final written JSON might omit the newly created session.

## 4. Performance Implications

The current implementation has a critical performance flaw regarding memory unboundedness:
1. **Event Throttler Map Leak:** The `last map[string]time.Time` is never pruned. Long-running sessions (which is CodeNERD's default mode) will slowly bleed RAM.
2. **Missing Context Timeouts:** `EnsureStarted` and `Shutdown` rely on the user providing a context, but do not impose their own hard timeouts. If the LLM doesn't set a timeout, `Start()` could hang indefinitely waiting for the Chrome debugger URL to become available.

## 5. Required Test Additions

To address these gaps, the following tests must be implemented (some requiring an integration or heavy-mocking environment):

- `TestSessionManager_EmptyDebuggerURL_ShouldFailFast`
- `TestSessionManager_ConcurrentStartShutdown_ShouldNotRace`
- `TestSessionManager_ZombiePage_WhenBrowserClosesTab_ShouldEvict`
- `TestEventThrottler_PrunesOldEntries_ToPreventOOM`
- `TestSessionManager_MassiveConcurrentCreates_ShouldRespectLimits`
- `TestSessionManager_CorruptedSessionStore_ShouldRecoverGracefully`

## 6. Extended Analysis & Edge Case Mitigations

### 6.1 Elaborated Vector C (User Request Extremes) Analysis

#### Massive DOM Ingestion Mitigation
When the Mangle reasoning engine receives raw DOM representations, the cost is significantly amplified by the logic engine's internal evaluation overhead. A single `page.HTML()` returning 500MB of raw string data is bad, but translating 500MB of DOM into tens of thousands of `Mangle.Fact` structures via `AddFacts` is catastrophic.

**Proposed Solution Strategy:**
Instead of ingesting the entire DOM blindly, the `SessionManager` (or the underlying Honeypot detector) must enforce a hard token/byte limit on fact generation.

```go
// Example pseudo-code mitigation
const MaxDOMFacts = 5000
func (m *SessionManager) ingestDOM(ctx context.Context, page *rod.Page) error {
   // Wait for network idle or DOMContentLoaded
   // Traverse DOM and extract interactive elements ONLY
   // Stop and return partial if count > MaxDOMFacts
}
```

The corresponding negative test must generate a synthetic webpage containing 50,000 `<a>` tags and ensure that `AddFacts` is only called `MaxDOMFacts` times and that memory does not spike above expected thresholds.

#### Event Throttler Hash Collision Mitigation
The `eventThrottler` is vulnerable to a simple hash-collision or map-bloat denial of service. If an external webpage under CodeNERD's control dynamically generates randomized `console.log()` messages:
```javascript
setInterval(() => console.log(Math.random()), 10);
```
The `eventThrottler.Allow(key)` will add every unique random number as a key to `t.last`.

**Proposed Solution Strategy:**
1. Implement a Least Recently Used (LRU) cache instead of an unbounded map.
2. Or, periodically run a background goroutine to sweep and prune keys older than `t.interval * 10`.

The corresponding negative test must log 1,000,000 unique keys and assert that the internal map size does not exceed a hardcoded maximum (e.g., 10,000 keys) and that the test completes within a reasonable timeframe without triggering the Go runtime garbage collector death spiral.

### 6.2 Elaborated Vector D (State Conflicts) Analysis

#### Zombie Session Eviction
The `go-rod` library is fundamentally synchronous for many operations. If the underlying browser crashes, the CDP WebSocket connection is broken. The next time the `SessionManager` attempts to call `page.Info()` or evaluate a script, it will block until the context timeout (which might be infinite if `context.Background()` was used) or panic if errors are not explicitly checked.

**Proposed Solution Strategy:**
1. Hook into `go-rod`'s built-in event listener for target destruction (`rod.EventTargetTargetDestroyed`).
2. When this event fires, eagerly remove the `sessionRecord` from the `m.sessions` map and log a warning.

The negative test must use the `rod.Browser` to open a tab, record the session ID, manually kill the Chrome tab via the lower-level CDP API (`browser.MustClose()`), and assert that a subsequent call to `m.List()` does not include the dead session and that `m.Page(id)` returns false.

#### Race Condition in Persistence
`persistSessions` is called during `Shutdown`. However, what if CodeNERD is abruptly terminated (SIGKILL) or panics before `Shutdown` is reached? The internal state (`m.sessions`) is lost, and the persisted JSON file on disk becomes stale.

**Proposed Solution Strategy:**
1. Implement a Write-Ahead Log (WAL) or a periodic flush (e.g., every 5 seconds) to disk instead of solely relying on `Shutdown`.
2. Ensure the `sessionRecord` locks its own mutex when being serialized to prevent partial writes.

The test must spawn multiple goroutines calling `CreateSession` while another goroutine simultaneously calls `persistSessions`. It must then reload the JSON from disk and verify that the JSON structure is valid (no corrupted bytes) and contains a consistent subset of the created sessions.

## 7. Extended Architecture Validation

The `SessionManager` acts as the bridge between the non-deterministic world (the web) and the deterministic logic engine (Mangle).

To truly validate this architecture, we must write a fuzz test that feeds random, high-entropy inputs into the Mangle engine simulating "hallucinated" browser commands.

```go
func FuzzSessionManager(f *testing.F) {
    f.Add("invalid-url://", "", 0, -1)
    f.Fuzz(func(t *testing.T, url string, targetID string, timeoutMs int, viewportWidth int) {
        cfg := DefaultConfig()
        cfg.DebuggerURL = url
        cfg.NavigationTimeoutMs = timeoutMs
        cfg.ViewportWidth = viewportWidth

        manager := NewSessionManager(cfg, nil)
        ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*50)
        defer cancel()

        _ = manager.Start(ctx)
        _, _ = manager.CreateSession(ctx, url)
        _, _ = manager.Attach(ctx, targetID)
        _ = manager.Shutdown(context.Background())

        // Assert no panics occurred.
    })
}
```

This fuzzer ensures that regardless of how mangled the LLM's requests are, the `SessionManager` remains a stable and protective boundary layer.

## 8. Summary of Negative Testing Principles for Browsers

1. **Assume the Network is Hostile:** Every navigation, evaluate, or screenshot request might hang forever.
2. **Assume the DOM is Infinite:** Never iterate over the entire DOM without a hard token limit.
3. **Assume State is Stale:** A `rod.Page` reference might be dead milliseconds after it is retrieved.
4. **Assume the LLM is Drunk:** Configuration values and command parameters can and will be completely illogical.

By implementing these edge case tests, we transform the `SessionManager` from a functional wrapper into a robust, adversarial-resistant subsystem capable of surviving CodeNERD's most intense autonomous campaigns.
