---

remediated: false
subsystem: mcp
---
# Boundary Value Analysis and Negative Testing Journal: MCP Tool Store
# Date: 2026-04-25 00:30:00 EST
# Subsystem: internal/mcp/store.go (MCP Tool Store)

## Executive Summary
This document outlines a thorough boundary value analysis and negative testing strategy for the MCP Tool Store in the codeNERD architecture. The MCP Tool Store acts as the SQLite-backed persistence layer for Model Context Protocol (MCP) integrations, caching servers, and tools, as well as maintaining vectors for semantic search. Because it interacts with the database layer, it is susceptible to null inputs, malformed types, scale limits (large objects), and concurrent execution.

## 1. Null / Undefined / Empty Inputs

### 1.1 Nil Structs and Pointers
- **Scenario:** Calling `SaveServer` or `SaveTool` with a `nil` pointer.
- **Expected Behavior:** The methods should immediately return an error (e.g., `fmt.Errorf("server cannot be nil")`) instead of panicking.
- **Current Gap:** `store_test.go` exclusively tests happy paths where valid struct pointers are provided.
- **Performance Impact:** Fast failure.

### 1.2 Empty Required Fields
- **Scenario:** A valid `MCPServer` pointer is provided, but its `ID` or `Endpoint` is an empty string. Or an `MCPTool` with an empty `ToolID`.
- **Expected Behavior:** SQLite might technically accept empty strings unless there are `NOT NULL` constraints paired with application-level validation. The application should reject empty IDs before hitting the database to prevent orphaned records.
- **Current Gap:** No tests verify that empty IDs or critical fields trigger errors.

### 1.3 Empty Slices and Maps
- **Scenario:** Saving a tool where `Categories`, `Capabilities`, or `UseCases` are empty `[]string{}`, or `nil`.
- **Expected Behavior:** The serialization logic should convert empty slices to `[]` in JSON instead of `null` if required, and correctly reconstruct them.
- **Current Gap:** No tests verify behavior with entirely empty array/slice configurations.

### 1.4 Nil Embeddings
- **Scenario:** `SemanticSearch` is called with a `nil` or empty `[]float32{}` query vector.
- **Expected Behavior:** The system must gracefully reject the search since vector distance calculation requires matching dimensions.
- **Current Gap:** No tests on `SemanticSearch` with empty embeddings.


## 3. User Request Extremes

### 3.1 Massive Tool Count
- **Scenario:** `GetAllTools` is called when there are 10,000+ tools in the database.
- **Expected Behavior:** The query should execute, but pulling 10,000 JSON schemas and vectors into memory at once might cause an OOM event.
- **Current Gap:** No performance benchmarks or pagination in the retrieval functions.

### 3.2 Massive Schema Size
- **Scenario:** `SaveTool` is called with an `InputSchema` that is 50MB in size.
- **Expected Behavior:** SQLite handles large BLOB/TEXT fields well, but the Go application memory will spike during JSON serialization and deserialization.
- **Current Gap:** Tests use tiny schemas. Extreme schemas could choke the system.

### 3.3 Massive Embedding Dimensions
- **Scenario:** `SaveTool` is passed a float array of 1,000,000 dimensions instead of the typical 768 or 1536.
- **Expected Behavior:** `sqlite-vec` likely has dimension limits. The system should enforce maximum dimension constraints.
- **Current Gap:** Tests do not explore extreme dimension lengths.


## 4. State Conflicts and Concurrency

### 4.1 Concurrent Writes
- **Scenario:** Multiple goroutines call `RecordToolUsage` simultaneously for the same tool ID.
- **Expected Behavior:** SQLite is capable of handling concurrent writes with WAL mode enabled. However, if the logic relies on read-modify-write in Go space, it will cause a race condition. If it is an atomic SQL `UPDATE`, it is safe.
- **Current Gap:** The test suite completely lacks `t.Parallel()` and `-race` coverage for concurrent database interactions.

### 4.2 Concurrent Saves and Reads
- **Scenario:** One goroutine calls `SaveServer` while another calls `GetAllServers`.
- **Expected Behavior:** SQLite should handle this via read/write locking. The system must not crash.
- **Current Gap:** Missing concurrency stress tests.

## Recommendations for Improvement
1. **Nil Pointer Guards:** Add `if server == nil { return error }` guards to all public store methods.
2. **Atomic Updates:** Ensure `RecordToolUsage` uses `UPDATE tools SET usage_count = usage_count + 1` instead of pulling the record, mutating, and saving.
3. **Pagination:** Implement `GetTools(offset, limit)` to prevent OOM when the tool registry grows indefinitely.
4. **Concurrency Tests:** Adopt a parallel test pattern to hit the store with hundreds of concurrent readers and writers to validate SQLite WAL stability.
