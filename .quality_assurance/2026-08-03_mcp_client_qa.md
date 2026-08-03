Journal Entry: QA Boundary Value Analysis & Negative Testing
Date: 2026-08-03 00:36:31 EST
System: codeNERD MCP Client Integration (internal/mcp)
Author: QA Automation Engineer

## 1. Executive Summary

This journal entry documents a deep dive into the Model Context Protocol (MCP) client module within the codeNERD architecture. As part of a Boundary Value Analysis and Negative Testing mandate, we are shifting away from traditional "Happy Path" scenarios. Instead, we systematically push the `internal/mcp` package to its limits across four specific vectors: Null/Undefined/Empty inputs, Type Coercion boundaries, User Request Extremes, and State Conflicts.

The `internal/mcp` package serves as a critical bridge. It is responsible for dynamically loading external tool capabilities via HTTP, Stdio, or SSE, compiling them into JIT (Just-In-Time) execution contexts, and handling the network boundary. A failure here could lead to context poisoning, memory exhaustion, or race conditions during multi-agent interactions.

This review will dissect each vector, identify gaps in the current testing strategy, and propose rigorous automated testing architectures to fortify the framework.

## 2. Methodology & Subsystem Evaluation

To evaluate this, I performed a static analysis of `internal/mcp/client.go`, `internal/mcp/mcp_client_integration_test.go`, and `internal/mcp/client_coverage_test.go`. I mapped out the state machine of the `MCPClientManager` and examined how it handles data ingested from external boundaries (servers) and internal boundaries (SubAgent / JIT Executor requests).

### 2.1 System Architecture Review
The `MCPClientManager` relies heavily on Go's `sync.RWMutex` for connection state and concurrent map operations.
- **Read Operations:** Lookups for tool definitions or server connections acquire a read lock, returning almost instantly.
- **Network Boundaries:** Context timeouts are correctly enforced, preventing unbounded hanging on external server failures.
- **Marshalling:** `json.Marshal` operations run sequentially per request but are fast for typical payloads.

However, vulnerabilities frequently arise during complex state transitions—such as `Connect` or `Disconnect` events—happening concurrently with high-throughput tool calls or massive ingestion phases.

## 3. Vector Analysis: Null / Undefined / Empty

In Go, "null" manifests as `nil` interfaces or pointers, uninitialized maps/slices, and empty strings. Testing must ensure that the boundary gracefully rejects or coerces these values before they propagate into panics.

### 3.1. Empty Server ID in Connect
- **Scenario:** A misconfigured intent, a corrupted `config.json`, or a malfunctioning JIT config factory passes `""` to `Connect(ctx, serverID)`.
- **System Behavior:** The system explicitly checks `if serverID == ""` in `client.go` and returns an error immediately. It does not panic. This is highly performant.
- **Test Gap:** While unit tests cover this, the integration wrapper (`IntegrationAdapter`) lacks a strict boundary test asserting this rejection behavior across the API boundary.
- **Proposed Improvement:** Introduce a table-driven test suite specifically for boundary rejections in `Connect`, ensuring empty, whitespace-only, and extremely long invalid IDs fail fast.

### 3.2. CallTool with Nil Arguments
- **Scenario:** A tool is invoked without any arguments (e.g., `s.client.CallTool(ctx, "test-server/ping", nil)`). This is common when LLMs omit the parameter object entirely.
- **System Behavior:** `client.go` handles this gracefully:
  ```go
  if args == nil {
      args = make(map[string]any)
  }
  ```
  It then clones the map (`maps.Copy(clonedArgs, args)`) to prevent race conditions during transport execution. The system handles this seamlessly and efficiently.
- **Test Gap:** `mcp_client_integration_test.go` exercises `CallTool` with `nil`, but a dedicated test validating that `nil` does not trigger map assignment panics under load is required.
- **Proposed Improvement:** Build a fuzzer that blasts `CallTool` with `nil`, empty maps, and deeply nested empty maps concurrently to guarantee structural integrity.

### 3.3. Nil Configuration Objects
- **Scenario:** The manager is initialized with `nil` maps for configurations or `nil` callback functions.
- **System Behavior:** Go permits reads from `nil` maps, and the manager defensive-copies or avoids writing to them blindly. Callbacks are typically checked for nil before execution.
- **Test Gap:** Lifecycle callbacks (like `onToolDiscovered`) might panic if triggered when set to `nil`.
- **Proposed Improvement:** Ensure integration tests assert that no panics occur during initialization and teardown with completely zeroed dependencies.

## 4. Vector Analysis: Type Coercion

Type coercion in Go often revolves around JSON unmarshalling interfaces (`any`) or invalid struct casting. Since MCP relies heavily on JSON-RPC, this is a prime attack surface.

### 4.1. Unserializable Tool Arguments
- **Scenario:** A higher-level subsystem passes an argument map containing a type that cannot be serialized to JSON (e.g., `make(chan int)`, or a complex unexported struct) to `CallTool`.
- **System Behavior:**
  ```go
  if _, err := json.Marshal(clonedArgs); err != nil {
      return nil, fmt.Errorf("invalid arguments: cannot serialize to JSON: %w", err)
  }
  ```
  The system proactively marshals the arguments before passing them to the transport layer. This fails fast and returns a clean error, preventing network layers from hanging or panicking.
- **Test Gap:** The integration tests do not verify this fail-fast mechanism. A test must supply unmarshallable constructs to prove the JSON check triggers appropriately.
- **Proposed Improvement:** Create a test suite `TestCallTool_TypeCoercionBoundary` that attempts to push function pointers, channels, and recursive structs through the API.

### 4.2. Schema Type Mismatches
- **Scenario:** A server returns an integer where a string description is expected in the JSON schema, or an array where an object is expected.
- **System Behavior:** The Go `json.Decoder` strictly enforces types based on the `MCPToolSchema` struct. It will fail to decode.
- **Test Gap:** Malformed schemas returned by rogue servers could result in partial parsing or unexpected zero values.
- **Proposed Improvement:** Inject a mock HTTP transport that returns deeply corrupted JSON schema payloads and assert the client cleanly ignores or errors without state corruption.

## 5. Vector Analysis: User Request Extremes

This vector tests the system's resilience against massive payloads, frontier-level scaling, and denial-of-service style responses from rogue servers.

### 5.1. Massive Tool Lists (DiscoverTools)
- **Scenario:** A rogue or misconfigured MCP server returns a payload containing 1,000,000 tool schemas during the `ListTools` protocol exchange.
- **System Behavior:** The transport reads the JSON body. If the JSON is valid, Go will allocate memory for 1,000,000 structs. The manager then iterates over them, truncating descriptions:
  ```go
  if tool.Condensed == "" && tool.Description != "" {
      tool.Condensed = truncate(tool.Description, 80)
  }
  ```
  While descriptions are truncated, the volume of tools could cause a significant GC (Garbage Collection) spike or even an Out-Of-Memory (OOM) panic.
- **Performance Impact:** The system does not currently enforce a hard pagination limit or maximum tool count limit *during ingestion* (though it does during JIT Selection). This is a potential vulnerability.
- **Test Gap:** There is no integration test simulating a massive response payload to verify if the transport timeout fires first, or if the memory spikes uncontrollably.
- **Proposed Improvement:** Implement a custom memory-bound testing harness that spins up a server emitting infinite JSON arrays. The test must assert that the client `DiscoverTools` context cancels appropriately before an OOM event.

### 5.2. Extreme Context Output Windows
- **Scenario:** A tool executes successfully but returns a 50MB string or binary blob as output.
- **System Behavior:** `client.go` includes a defensive truncation mechanism:
  ```go
  const maxContextWindowBytes = 500 * 1024
  if len(result.Output) > maxContextWindowBytes {
      truncMsg := []byte("\n...[output truncated due to MCP context memory window limit]")
      result.Output = append(result.Output[:maxContextWindowBytes], truncMsg...)
  }
  ```
  This is highly performant and successfully protects the LLM context window.
- **Test Gap:** Ensure the truncation operates correctly at exact byte boundaries (e.g., exactly `500 * 1024` bytes) without off-by-one slice bounds panics.
- **Proposed Improvement:** Write table-driven tests checking `len(result.Output)` at `Max - 1`, `Max`, and `Max + 1`.

## 6. Vector Analysis: State Conflicts

State conflicts involve race conditions, deadlocks, and stale data when multiple goroutines interact with the manager simultaneously.

### 6.1. Concurrent CallTool and Disconnect
- **Scenario:** Goroutine A calls `CallTool("srv/tool")`. Goroutine B calls `Disconnect("srv")` at the exact same millisecond.
- **System Behavior:**
  The manager acquires an RLock to look up the connection, then releases it before network I/O:
  ```go
  m.mu.RLock()
  conn, ok := m.servers[serverID]
  m.mu.RUnlock()
  ```
  Goroutine B can then acquire the WLock, remove the server, and call `conn.Transport.Close()`.
  Meanwhile, Goroutine A invokes `CallTool` on a transport that is actively being closed.
- **Performance/Safety:** Because Goroutine A holds a reference to `conn`, it will not nil-dereference. The transport implementations (HTTP, Stdio, SSE) must handle this by returning a context cancellation or closed network connection error.
- **Test Gap:** The integration test suite lacks a rigorous stress test that spans `CallTool` and `Disconnect` concurrently to prove that the transports cleanly return `context.Canceled` or similar.
- **Proposed Improvement:** Utilize Go's `testing.T.Run` with `t.Parallel()` and a `sync.WaitGroup` to spawn 1,000 goroutines hammering `CallTool` while a single goroutine violently disconnects the server.

### 6.2. Double Connect / Rapid Reconnect
- **Scenario:** Two goroutines attempt to `Connect` to the same server simultaneously, or a disconnect and reconnect happen back-to-back.
- **System Behavior:** The `Connect` method acquires a WLock early. The first goroutine establishes the connection. The second checks `m.servers[serverID]` and sees it is already connected, returning `nil` gracefully. This is safe.
- **Test Gap:** Verifying that a rapid disconnect followed instantly by a reconnect properly flushes stale tool definitions from the cache.

## 7. Future Architectural Recommendations

To further harden the codeNERD MCP client against extreme boundary conditions, I recommend the following architectural enhancements:

1. **Strict Transport Ingestion Limits:** Implement an `io.LimitReader` on the HTTP and Stdio transport layers during `ListTools`. If a server attempts to return more than 10MB of JSON schemas, the connection should be forcefully severed to prevent memory exhaustion on resource-constrained devices.
2. **Context-Aware Transports:** Ensure that all transport implementations rigorously respect `context.Context` cancellation during blocking I/O (e.g., reading from Stdio pipes or waiting for SSE events), which is vital for the `State Conflicts` mitigation outlined above.
3. **Fuzz Testing:** Introduce Go Fuzz testing (`go test -fuzz`) for the `CallTool` argument marshalling and the `parseToolID` routing logic.
4. **Rate Limiting:** Implement token bucket rate limiting on `CallTool` per server ID to prevent a runaway agent loop from saturating local network ports or remote APIs.
5. **Circuit Breakers:** Introduce a circuit breaker pattern on transports. If an MCP server returns 5 consecutive `context.DeadlineExceeded` errors, automatically trip the breaker and fail subsequent calls fast for a cooling-off period.

## 8. Detailed Test Implementation Strategies

Below is a proposed structure for implementing the missing tests identified during this analysis.

### 8.1 Implementing TestConnect_EmptyServerID
```go
func (s *MCPClientIntegrationSuite) TestConnect_EmptyServerID() {
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()
    err := s.client.Connect(ctx, "")
    s.Require().Error(err)
    s.Require().Contains(err.Error(), "server ID cannot be empty")
}
```

### 8.2 Implementing TestCallTool_NilArgs
```go
func (s *MCPClientIntegrationSuite) TestCallTool_NilArgs() {
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()

    err := s.client.Connect(ctx, "test-server")
    s.Require().NoError(err)

    result, err := s.client.CallTool(ctx, "test-server/ping", nil)
    s.Require().NoError(err)
    s.Require().True(result.Success)
}
```

### 8.3 Implementing TestCallTool_InvalidArgsTypes
```go
func (s *MCPClientIntegrationSuite) TestCallTool_InvalidArgsTypes() {
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()

    err := s.client.Connect(ctx, "test-server")
    s.Require().NoError(err)

    badArgs := map[string]any{
        "input": make(chan int),
    }

    _, err = s.client.CallTool(ctx, "test-server/ping", badArgs)
    s.Require().Error(err)
    s.Require().Contains(err.Error(), "cannot serialize to JSON")
}
```

### 8.4 Implementing TestDiscoverTools_UserExtremes
```go
func (s *MCPClientIntegrationSuite) TestDiscoverTools_UserExtremes() {
    // Setup a custom mock server that streams 1,000,000 tools
    // Must assert timeout or clean cancellation without panicking
    ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
    defer cancel()

    err := s.client.DiscoverTools(ctx, "rogue-server")
    s.Require().ErrorIs(err, context.DeadlineExceeded)
}
```

### 8.5 Implementing TestCallToolConcurrentDisconnect
```go
func (s *MCPClientIntegrationSuite) TestCallToolConcurrentDisconnect() {
    ctx := context.Background()
    err := s.client.Connect(ctx, "test-server")
    s.Require().NoError(err)

    var wg sync.WaitGroup
    wg.Add(2)

    go func() {
        defer wg.Done()
        for i := 0; i < 1000; i++ {
            _, _ = s.client.CallTool(ctx, "test-server/ping", nil)
        }
    }()

    go func() {
        defer wg.Done()
        time.Sleep(5 * time.Millisecond)
        _ = s.client.Disconnect("test-server")
    }()

    wg.Wait()
}
```

## 9. Conclusion

The `internal/mcp` client is generally robust and handles basic edge cases well, leveraging Go's strong typing and JSON unmarshalling guarantees. However, relying purely on implicit system behavior (like map copies not panicking on nil) is a risk. By solidifying these boundaries with explicit integration tests (marked by the new `TODO` gaps), we ensure the framework can safely scale to frontier coding workloads without subtle regression panics.

## 10. Further In-Depth Analysis on Type Safety and Memory Limits

### 10.1 Schema Validation Depth
When an MCP server provides tools, the schemas are heavily relied upon by the LLM to generate proper JSON inputs. If the internal system does not rigorously validate the structural integrity of these JSON schemas upon ingestion:
- The `analyzer.go` module might attempt to compile embeddings for malformed descriptions.
- The `renderer.go` module could emit invalid markdown or JSON back to the LLM, breaking the AI's generation capability.
Testing MUST include injecting schemas with deeply nested recursive `$ref` loops. If a server malicious provides a recursive schema, does the `json.Marshal` on the outgoing render step cause a stack overflow?

### 10.2 Connection State Matrix
The `ServerStatus` enum transitions through `Unknown`, `Connecting`, `Connected`, `Disconnected`, and `Error`.
Negative testing should forcibly inject network faults during the exact moment a transition from `Connecting` to `Connected` occurs to ensure the `onServerStatus` callback does not receive contradictory events or panic if the map entry is simultaneously wiped.

### 10.3 Transport Concurrency
Each transport (Stdio, HTTP, SSE) implements its own reading and writing goroutines.
- For Stdio: If the child process is killed (`SIGKILL`) externally, does the reader goroutine leak, or does it cleanly detect `EOF`?
- For SSE: If the server keeps the connection open but sends malformed data frames indefinitely, does the parsing loop consume 100% CPU, or does it abort?
These require explicit chaos-engineering style tests to simulate operating system level failures.

## 11. Edge Cases in Argument Passing

### 11.1 The Null Byte Injection
A critical boundary value test involves passing strings containing null bytes (` `) within the `CallTool` argument map. While Go handles null bytes in strings natively, many underlying libraries or CGO bridges (such as SQLite) or even remote MCP servers written in C/C++ or older Node.js versions might truncate the string or crash.
- **Scenario:** `s.client.CallTool(ctx, "tool", map[string]any{"path": "file.txt malicious"})`
- **System Behavior:** The Go client will marshal this JSON successfully. However, the downstream effect on the server is unknown.
- **Proposed Improvement:** We should sanitize string inputs at the client boundary, or at least have a test verifying that null bytes are transmitted as valid JSON-escaped sequences (`\u0000`) rather than raw bytes that could corrupt the transport stream.

### 11.2 Massive Number Values
JSON does not specify limits for numeric precision, but Go's `any` unmarshalling defaults to `float64`.
- **Scenario:** An LLM generates a tool call with an integer exceeding a 64-bit boundary (e.g., `999999999999999999999999999999`).
- **System Behavior:** If passed directly as a string or a `json.Number`, it might be safe. If unmarshalled into an `interface{}` intermediate, precision might be lost or rounded unexpectedly.
- **Test Gap:** The client must have a negative test verifying that massive numbers do not cause the `json.Marshal` step to panic or drop data silently.

## 12. Security Boundary Considerations

### 12.1 Path Traversal in Tool IDs
The client parses tool IDs using `serverID/toolName`.
- **Scenario:** A malicious server attempts to register a tool named `../../etc/passwd` or similar. Or, an LLM attempts to call `srv/../../../system/execute`.
- **System Behavior:** The `client.go` code contains a specific check:
  ```go
  if strings.Contains(toolName, "..") || strings.ContainsAny(toolName, "/\") {{
      return nil, fmt.Errorf("invalid tool name: directory traversal detected")
  }}
  ```
  This is excellent defensive programming.
- **Test Gap:** The coverage tests check basic slash parsing, but the integration suite lacks a penetration-testing style case specifically hitting this directory traversal block.

### 12.2 Arbitrary Protocol Injection
When connecting to a server, the protocol is inferred or provided.
- **Scenario:** `Connect(ctx, "http://localhost:8080/mcp
Host: malicious.com

")`
- **System Behavior:** The Go `net/http` library typically defends against CRLF injection, but raw Stdio or SSE transports might not.
- **Proposed Improvement:** Inject CRLF characters into the server configuration URLs and verify they are rejected during the `Connect` phase.

## 13. Summary of Action Items

1. Update `internal/mcp/mcp_client_integration_test.go` with the 5 new `TODO: TEST_GAP` markers (completed).
2. Implement the 5 missing integration tests to cover Null, Type Coercion, Extremes, and State Conflicts.
3. Review `DiscoverTools` to implement a hard cap (e.g., 5,000 tools max) to prevent memory exhaustion from rogue servers.
4. Add fuzzing for the `CallTool` JSON marshalling pipeline to catch edge cases with numeric precision and null bytes.
5. Create a chaos engineering suite that randomly kills Stdio subprocesses and terminates SSE TCP connections to ensure the `MCPClientManager` recovers cleanly without stale lock states.

This comprehensive QA analysis ensures that the Model Context Protocol integration within codeNERD remains a robust, enterprise-grade component capable of handling extreme scaling and adversarial inputs without compromising the overarching neuro-symbolic kernel.

## 14. Deep Dive into the Stdio Transport Vulnerabilities

The Stdio transport (`transport_stdio.go`) is particularly vulnerable to edge cases because it relies on OS-level process management and pipes, rather than standard network sockets.

### 14.1 Zombie Processes
When `Disconnect` is called, the transport must send a signal (e.g., `SIGKILL` or `SIGTERM`) to the underlying subprocess.
- **Scenario:** The subprocess ignores `SIGTERM` and continues running.
- **System Behavior:** The Go `exec.Cmd.Wait()` might hang indefinitely if the process does not exit, leading to a goroutine leak.
- **Test Gap:** There is no integration test verifying that a stubborn subprocess is forcefully reaped.

### 14.2 Pipe Buffer Deadlocks
- **Scenario:** The MCP server writes 5MB of log data to `stderr` rapidly, but the client only reads from `stdout`.
- **System Behavior:** If the OS pipe buffer for `stderr` fills up, the child process will block indefinitely on its `write()` system call. This will cause the entire MCP server to freeze, halting `stdout` responses as well.
- **Test Gap:** We must ensure the `StdioTransport` actively drains both `stdout` and `stderr` concurrently, even if `stderr` is simply discarded or logged.

## 15. The Impact of Mangle on the MCP Boundary

The `codeNERD` architecture relies heavily on Google's Mangle for logic programming and fixpoint evaluation. The MCP client must interact safely with the `Mangle` knowledge base.

### 15.1 Fact Assertion Limits
When a tool is discovered via MCP, its schema might be translated into Mangle facts (e.g., `mcp_tool(/test_server, /ping)`).
- **Scenario:** A server with 10,000 tools causes 10,000 facts to be asserted into the Mangle store.
- **System Behavior:** The Mangle engine is designed for high performance, but massive IDB (Intensional Database) derivations can slow down the JIT (Just-In-Time) execution loop.
- **Test Gap:** We must verify that asserting massive tool catalogs does not degrade the `Ouroboros Loop` evaluation times beyond acceptable thresholds (e.g., <50ms).

## 16. Final Sign-Off

The analysis confirms that while the `internal/mcp` package is well-structured and utilizes Go's concurrency primitives effectively, it lacks rigorous boundary testing at the integration layer. Implementing the 5 missing tests documented above, along with the proposed architectural hardening, will significantly elevate the system's reliability.

Analysis completed by QA Automation Engineer.

## 17. Expanded Analysis: The SSE Transport Layer

Server-Sent Events (SSE) provide a unidirectional event stream. The `transport_sse.go` handles this by maintaining an open HTTP connection.

### 17.1 Connection Drops and Reconnection Logic
- **Scenario:** The TCP connection for the SSE stream is abruptly severed by a middlebox or proxy.
- **System Behavior:** The `http.Client.Do` call will eventually return an error, terminating the reading goroutine.
- **Test Gap:** Does the `MCPClientManager` automatically attempt to reconnect, or does it mark the server as `ServerStatusDisconnected` and require user intervention? The expected behavior needs to be explicitly defined and tested.

### 17.2 Malformed SSE Framing
- **Scenario:** The server sends data that does not conform to the `data: ...

` SSE specification (e.g., missing double newlines, or binary data).
- **System Behavior:** The `bufio.Scanner` or custom parser might desync, misinterpreting event boundaries.
- **Test Gap:** We need negative tests feeding garbage bytes into the SSE transport to ensure it recovers or errors cleanly without corrupting the next valid JSON payload.

## 18. Expanded Analysis: Tool Selection Config

The `ToolSelectionConfig` struct dictates how tools are tiered (Full, Condensed, Minimal) based on token budgets.

### 18.1 Negative Token Budgets
- **Scenario:** The user configures a `TokenBudget` of `-1000`.
- **System Behavior:** The system should likely treat this as `0` and fallback to minimal renderings, or reject the configuration at startup.
- **Test Gap:** Ensure negative or absurdly large (`MAX_INT`) token budgets do not cause integer overflows or infinite loops during the `FitBudgetDemotesTools` algorithm.

## 19. Expanded Analysis: The Analyzer Component

The `analyzer.go` module uses NLP or embedding models to analyze tool schemas.

### 19.1 Embedding API Failures
- **Scenario:** The external embedding API (e.g., OpenAI or local Ollama) is down or returns 500 Internal Server Error.
- **System Behavior:** The `Analyze` method should gracefully return a fallback analysis (e.g., zero-vector) rather than crashing the entire tool discovery process.
- **Test Gap:** Mock the embedding engine to always fail, and verify that `DiscoverTools` still completes successfully, albeit with degraded semantic search capabilities.

## 20. Conclusion of Deep Dive

This concludes the comprehensive 20-point analysis of the `internal/mcp` boundary. The system demonstrates excellent foundational architecture, but true production readiness requires these negative testing gaps to be closed. The `TODO` comments inserted into the codebase serve as the immediate action items for the development team.

## 21. Expanded Analysis: The Tool Store Component

The `store.go` component persists discovered tools to an SQLite database (`mcp_tools.db`).

### 21.1 Disk Full Errors
- **Scenario:** The host system's disk is completely full.
- **System Behavior:** The `SaveTool` method will attempt to execute an `INSERT` or `UPDATE` statement, which will fail with a `SQLITE_FULL` error.
- **Test Gap:** The client manager currently logs a warning `logging.Get(logging.CategoryTools).Warn("Failed to persist tool %s: %v", toolID, err)` but continues operation. We must verify via integration test that the system truly can operate "in-memory only" if the persistent store fails, ensuring resilience.

### 21.2 Database Lock Contention
- **Scenario:** Two separate codeNERD instances attempt to write to the same `mcp_tools.db` simultaneously.
- **System Behavior:** SQLite uses file locking. If the `PRAGMA busy_timeout` is exceeded, a `database is locked` error is returned.
- **Test Gap:** Simulate high write contention on the database file and verify that the `SaveTool` operations do not crash the agent, but rather degrade gracefully (perhaps caching in memory and retrying later).

## 22. Expanded Analysis: The Renderer Component

The `renderer.go` module converts the `CompiledToolSet` into markdown or JSON for the LLM.

### 22.1 Malformed Tool Descriptions
- **Scenario:** A tool description contains invalid markdown syntax, unescaped HTML, or control characters (e.g., ``, ``).
- **System Behavior:** The renderer passes strings mostly as-is. If rendered into a JSON payload for an API (like Claude or Gemini), invalid control characters will break the JSON marshalling at the *LLM Client* boundary, not the MCP boundary.
- **Test Gap:** We need negative tests ensuring that `renderer.go` sanitizes descriptions (e.g., stripping ANSI escape codes) before embedding them into the system prompt.

## 23. Final Metrics and Sign-Off

- **Total Vectors Analyzed:** 4 primary (Null, Coercion, Extremes, Conflicts) + 4 secondary (Transport, Config, Analyzer, Store).
- **Total Gaps Identified:** 15 distinct negative testing scenarios.
- **TODO Comments Inserted:** 5 critical gaps marked directly in `internal/mcp/mcp_client_integration_test.go`.

This document serves as the canonical reference for the upcoming Q3 resilience sprint.

## 24. Addendum: Reviewing the Integration Adapter

The `IntegrationAdapter` (in `integration.go`) acts as the facade for the rest of the codeNERD system (specifically the JIT Executor) to interact with the MCP layer.

### 24.1 Adapting Errors for LLM Consumption
When an MCP tool fails (e.g., a timeout or a schema mismatch), the error must be propagated back to the JIT Executor, which then formats it for the LLM to understand and correct its behavior.
- **Scenario:** An MCP server returns an obscure internal error trace (e.g., a Java stack trace from a Spring Boot backend).
- **System Behavior:** The `CallTool` method currently returns `MCP protocol error: <trace>`.
- **Test Gap:** We must ensure that the `IntegrationAdapter` does not leak sensitive information or massive stack traces directly into the LLM context window, which could blow the token budget. The errors should be sanitized and truncated.
- **Proposed Improvement:** Implement a test that asserts error string truncation occurs at the adapter boundary before it reaches the `session` package.
