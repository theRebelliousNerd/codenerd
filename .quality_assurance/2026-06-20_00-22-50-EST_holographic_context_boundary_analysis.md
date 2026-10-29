# Holographic Context Boundary Value Analysis & Negative Testing Journal
## Target Subsystem: `internal/world` - Holographic Context Provider

### Executive Summary

The Holographic Context system (`holographic.go`, `holographic_impact.go`) is responsible for constructing multi-dimensional architectural views of the codebase. By generating context based on sibling files, imports, and symbol signatures, it allows LLM agents to possess "x-ray vision" into the surrounding package. However, boundary condition assessment highlights key areas of fragility, particularly around the processing of invalid or edge-case AST files, extreme concurrency boundaries, and data race potentials.

This journal outlines a comprehensive negative testing and boundary value analysis to strengthen these components against failure vectors.

---

### Section 1: Null, Undefined, and Empty State Handling

When dealing with dynamically scanned file systems, "empty" states are surprisingly common and challenging.

1.  **Zero-Byte Files:** A 0-byte Go file is perfectly legal in the filesystem but technically not a valid complete package in Go unless there's at least a package declaration. Testing `h.extractGoSignatures` with a 0-byte file (and purely whitespace files like a file with only `\n` or ` `) should be strictly enforced to ensure no panic occurs within the TreeSitter/`go/ast` parsers.
2.  **Nil vs. Empty Array Serialization:** Go's `encoding/json` distinguishes between a `nil` slice (serializes to `null`) and an empty initialized slice `make([]string, 0)` (serializes to `[]`). In `extractTypeDefinition`, we append to `typeDef.Fields` or `typeDef.Methods`. If a struct or interface has zero fields/methods, these stay `nil`. We need tests to confirm the resulting LLM context output handles this correctly (e.g., if a prompt expects `[]` and gets `null`).
3.  **Empty Prioritized Callers:** The `HasPrioritizedCallers` method explicitly checks `len(hc.PrioritizedCallers) > 0`. However, formatting functions like `FormatWithPriorities` might have untested behavior if `hc.PrioritizedCallers` is an initialized but empty slice compared to a purely `nil` slice.

### Section 2: Type Coercion and Invalid Data Formats

The `HolographicContext` heavily relies on `filepath.Ext` to deduce language types, which opens up avenues for type coercion vulnerabilities or incorrect processing logic if file extensions mask their contents.

1.  **Malformed Sibling AST Disruption:** In `buildGoContextWithContext`, the system iterates over all sibling Go files. If `parser.ParseFile` throws an error on *one* malformed sibling file, the entire context generation process is currently prone to returning that error or dropping the entire package's context. A test explicitly adding a syntactically invalid Go file to a directory containing 9 valid ones must be added to ensure graceful degradation.
2.  **Extension Spoofing / Binary Files:** A binary file renamed to `.go` will be fed into `parser.ParseFile`. While `go/ast` is reasonably robust against crashing on binary data, attempting to read a 4.9MB compiled binary (just under the 5MB limit) into memory as strings is incredibly inefficient. Tests should feed a small binary file to ensure the system correctly errors on the AST parse and falls back cleanly without allocating excessive string memory.
3.  **Non-UTF8 Encoding:** Go's `ast` assumes UTF-8 source. A test should feed valid Go code encoded in UTF-16 or Shift-JIS to observe the failure mode.

### Section 3: User Request Extremes

The holographic context builder represents a significant attack surface for memory and CPU exhaustion if unconstrained.

1.  **Massive Directory Breadth:** The system currently utilizes `os.ReadDir` before capping files at 100 via `maxPackageFilesToParse`. While the parsing is capped, reading a directory with 500,000 files (e.g., an accidental `node_modules` equivalent scan) will still buffer 500,000 `DirEntry` objects into memory.
2.  **Deep Call Hierarchies:** `PrioritizedCaller` uses `Depth`. A test should mock the impact analysis engine to return an artificially deep call graph (e.g., Depth=1000) to ensure there are no recursive loop failures in formatting or stack overflows in recursive prioritization rules.
3.  **Extremely Long Lines:** A file consisting of a single line of 4.5 million characters (e.g., a minified or obfuscated string literal within a Go file) could cause line-by-line parsers or regex implementations to hit complexity limits. The `findFunctionEnd` helper logic must be validated against files lacking newlines entirely.

### Section 4: State Conflicts and Race Conditions

The holographic context provider maintains internal state caches and is built for concurrent use across multiple subagents in a live session.

1.  **Concurrent Regex Cache Access:** The system employs a `regexCache` map guarded by a `sync.RWMutex` (`regexCacheMu`). A dedicated negative test must spawn 100+ goroutines simultaneously attempting to read and write context for the *same* file, forcing high lock contention to detect any read-after-write anomalies or deadlocks during context cancellation.
2.  **Ephemeral Filesystem Operations:** Between the `os.ReadDir` call and the subsequent `os.Stat` or `parser.ParseFile` execution, a file might be deleted or locked by an external process. The system must degrade gracefully (e.g., logging a warning) rather than propagating an `fs.ErrNotExist` up the chain and failing the entire context generation.
3.  **Context Cancellation Latency:** `getContextInternal` checks `ctx.Err()`. A negative test should simulate extremely high latency in AST parsing and trigger context cancellation mid-parse to verify that resources are promptly freed and goroutines don't leak.

---

### Expanded Test Plan Blueprint

The following specific tests need to be implemented within `holographic_test.go` to cover these vectors:

#### Test Vector: Malformed AST Impact Isolation
```go
func TestHolographicContext_MalformedGoFile(t *testing.T) {
    // 1. Create a TempDir.
    // 2. Write valid1.go (valid package main)
    // 3. Write invalid.go (contains "func { broken { code")
    // 4. Write valid2.go (valid package main)
    // 5. Run buildGoContext.
    // 6. Assert that hc.PackageSiblings contains valid1.go and valid2.go.
    // 7. Assert that no fatal error is returned for the whole package.
}
```

#### Test Vector: Zero-Byte Boundaries
```go
func TestHolographicContext_EmptyFile(t *testing.T) {
    // 1. Create TempDir with a 0-byte file empty.go.
    // 2. Run extractGoSignatures.
    // 3. Assert it returns a clean error or nil without panicking.
    // 4. Verify that generated Context fields are correctly initialized.
}
```

#### Test Vector: Concurrency & Lock Contention
```go
func TestHolographicContext_ConcurrentReadWrite(t *testing.T) {
    // 1. Initialize HolographicProvider.
    // 2. Use sync.WaitGroup to spawn 50 goroutines.
    // 3. Each goroutine requests GetContext for the same file.
    // 4. Run with -race flag to detect regexCacheMu contention/leaks.
}
```

#### Test Vector: Mid-Flight Deletion
```go
func TestHolographicContext_DeletedFileMidFlight(t *testing.T) {
    // 1. Mock os.ReadDir to return a file list.
    // 2. Delete the physical file before parser.ParseFile is called.
    // 3. Ensure the error is swallowed/logged, and the context builder continues.
}
```

#### Test Vector: Fallback Robustness
```go
func TestHolographicContext_BinaryFileFallback(t *testing.T) {
    // 1. Create a file containing randomly generated bytes (e.g., crypto/rand).
    // 2. Name it binary_data.bin.
    // 3. Call GetContext.
    // 4. Ensure it routes to buildBasicContext and doesn't attempt AST parsing.
    // 5. Ensure memory usage remains flat (use runtime.ReadMemStats).
}
```

#### Test Vector: Edge-Case Structs/Interfaces
```go
func TestHolographicContext_EmptyTypeDefinitions(t *testing.T) {
    // 1. Write file with `type EmptyStruct struct{}` and `type EmptyInterface interface{}`
    // 2. Extract context.
    // 3. Verify Fields and Methods properties on TypeDefinition handle empty appropriately.
}
```

### Performance Assessment Against Vectors
The `HolographicProvider` is reasonably well-optimized via early escapes (e.g., skipping files >5MB). However, its handling of *directory scans* relies on standard `os.ReadDir`, which buffers in memory. For standard workspaces (<10,000 files per dir), this is highly performant. For pathological workspaces (100,000+ files in a single flat directory), it will suffer latency spikes before the 100-file cap is applied.

The most critical fragility is its current "fail-fast" disposition when encountering a single `parser.ParseFile` failure during sibling iteration. A single broken Go file in a developer's workspace can blind the agent to the entire architectural context of that directory. By shifting this to a "log and continue" pattern, the agent's resilience against live, under-development workspaces will be drastically improved.
