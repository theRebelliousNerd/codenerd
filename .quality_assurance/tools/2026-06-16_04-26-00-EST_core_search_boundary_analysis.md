---
date: 2026-06-16 04:26:00 EST
author: Jules (QA Automation Engineer)
subsystem: Core Search Tools (glob, grep)
focus: Boundary Value Analysis and Negative Testing
---

# Boundary Value Analysis: Core Search Tools

This journal documents the boundary value analysis and negative testing evaluation for the `internal/tools/core/search.go` subsystem, specifically focusing on the `glob` and `grep` tool execution functions (`executeGlob`, `executeGrep`, and `searchFile`). The core search tools are responsible for allowing the codeNERD agent to accurately locate and index files and code snippets across arbitrarily large monorepos. Failures or silent coercion errors in this system drastically inhibit the agent's contextual awareness and execution capabilities.

## 1. Null, Undefined, and Empty Inputs

### The `args` Map Nil Pointer
Currently, the tests check for missing parameters by passing an empty map, like so:
`executeGlob(context.Background(), map[string]any{})`

While this tests that missing a specific key like `"pattern"` safely returns an error, it fails to account for the scenario where the entire `args` map is strictly `nil`.
In Go, reading from a `nil` map (e.g., `args["pattern"]`) does not panic; it simply returns the zero value for the type interface. However, a robust test suite should explicitly assert that a nil map does not cause downstream nil pointer dereferences or panics, particularly when passed from the transducer subsystem where tool calls might occasionally omit the `arguments` field entirely.

### Pure Whitespace Patterns
Another significant gap is the handling of strings that are purely whitespace (`"   "`).
- In `executeGlob`, a whitespace pattern passed to `filepath.Glob` or `filepath.Match` might yield unexpected matches or no matches, but it doesn't fail gracefully.
- In `executeGrep`, passing whitespace into `regexp.Compile("(?i)   ")` successfully compiles into a valid regular expression that matches any sequence of three spaces. While technically correct behavior for a grep tool, in the context of an autonomous LLM agent, a pure whitespace query is almost always an accidental hallucination or formatting error that will return thousands of useless context lines, wasting precious Token Budget and filling the `Context` array with garbage.

## 2. Type Coercion and JSON Unmarshaling

### The `float64` to `int` Silent Failure
This is one of the most critical vulnerabilities in the codeNERD tool registry architecture when interacting with JSON-based LLM outputs.
Both `executeGlob` and `executeGrep` retrieve integer configurations using type assertions:

```go
if mr, ok := args["max_results"].(int); ok && mr > 0 {
    maxResults = mr
}
```

When an LLM generates a JSON tool call, standard Go JSON unmarshaling (e.g., into `map[string]any`) unmarshals all numbers as `float64`.
Because the types are strictly distinct in Go, the assertion `args["max_results"].(int)` will **always be false** if the value originated from standard JSON unmarshaling.

The result is a silent failure. The system will discard the LLM's explicit instruction (e.g., `max_results: 5` or `context_lines: 10`) and fall back to the hardcoded defaults (`maxResults = 50`, `contextLines = 0`).
The agent will be completely blind to surrounding context lines it explicitly requested because `contextLines` will remain 0.

**Remediation in Tests:**
The test suite explicitly passes explicit integers in the mock maps: `float64(1)` was seen in `TestGrepTool_Execute_WithContext`, but it relies on an explicit mock configuration that masks the real-world JSON integration issue. Tests must be added that inject `float64` values and verify that the system either correctly coerces them using type switching (e.g., `case float64: maxResults = int(v)`) or fails loudly.

## 3. User Request Extremes

### Catastrophic Backtracking (ReDoS)
The `grep` tool blindly accepts user-provided (or LLM-provided) regex patterns and passes them to `regexp.Compile`.
While Go's `regexp` package guarantees linear time execution and is generally immune to traditional Catastrophic Backtracking (ReDoS) because it uses RE2 instead of PCRE, extremely complex or deeply nested patterns can still consume significant memory during compilation or matching on massive files. A test should verify the system's performance boundaries with maximum length regex strings.

### Unbounded Execution and Context Cancellation
Both `executeGlob` and `executeGrep` take a `context.Context`, but they never actually check `ctx.Done()`.

```go
err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
    // Missing: if ctx.Err() != nil { return ctx.Err() }
    // ...
```

In a brownfield project involving a 50 million line monorepo, a query for `**/*.js` or a wide grep pattern could take several minutes. If the user cancels the session, or the `TokenBudgetManager` / `OuroborosLoop` times out the task, the internal `filepath.Walk` loop will continue traversing millions of files in the background, consuming IO and CPU resources until completion.

The same applies to `bufio.Scanner` in the `searchFile` function:

```go
for scanner.Scan() {
    // Missing: if ctx.Err() != nil { return nil, ctx.Err() }
    lineNum++
    // ...
```

Tests must be added that pass a context with a timeout (e.g., 1 millisecond), spawn a massively recursive mock directory structure, and assert that the function exits immediately with a `context.DeadlineExceeded` error rather than finishing the traversal.

### Memory Exhaustion via Context Lines
If an agent hallucinates and requests `context_lines: 1000000`, the `searchFile` function will buffer millions of strings into memory:
```go
if contextLines > 0 && len(lines) > contextLines+1 {
    lines = lines[1:]
}
```
A test should verify that absurdly large integer requests for context lines are clamped to a safe maximum (e.g., 100) to prevent OOM panics in the Mangle virtual store.

## 4. State Conflicts

### Time-of-Check to Time-of-Use (TOCTOU) Races
The `executeGrep` function operates in two distinct phases:
1. Discovery Phase: It uses `filepath.Walk` or `os.Stat` to build a flat `files []string` slice of all target files.
2. Execution Phase: It iterates over the `files` array, calling `searchFile`, which sequentially calls `os.Open(path)`.

In a highly concurrent development environment (or when other agents/tools are actively modifying the workspace), a file discovered in step 1 might be deleted, moved, or have its permissions revoked (`chmod 000`) before step 2 executes.

Currently, `executeGrep` handles this reasonably well by continuing on error:
```go
fileMatches, err := searchFile(file, re, contextLines, maxResults-len(matches))
if err != nil {
    continue // Skip files with errors
}
```
However, **there are no tests explicitly proving this safety mechanism.** A state conflict test should be introduced that uses a goroutine to delete a file immediately after the `filepath.Walk` phase but before `searchFile` is invoked, asserting that the tool gracefully skips the file and returns the remaining matches without returning an I/O error to the LLM.

## Conclusion

The `core` search tools are the eyes of the codeNERD agent. While the happy path testing is thorough, the lack of boundary testing around type coercion (`float64` vs `int`) is likely causing silent failures in production where the LLM is requesting context lines but receiving none. Furthermore, the absence of `ctx.Done()` checks in the deep I/O loops is a significant availability vulnerability for large scale monorepos. Addressing these gaps will vastly improve the reliability of the core tooling substrate.
