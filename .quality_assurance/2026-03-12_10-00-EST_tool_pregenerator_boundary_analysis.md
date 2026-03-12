# Boundary Value Analysis and Negative Testing: Tool Pregenerator Subsystem

**Date:** 2026-03-12
**Time:** 10:00 AM EST
**Subsystem:** Tool Pregenerator (`internal/campaign/tool_pregenerator.go` & `tool_pregenerator_test.go`)
**Analyst:** codeNERD QA Automation Engineer

## 1. Executive Summary

This journal entry documents a comprehensive boundary value analysis and negative testing review of the `ToolPregenerator` subsystem in the codeNERD framework's campaign orchestrator. The `ToolPregenerator` plays a critical role in the OODA loop by pre-generating tools via Ouroboros integration to ensure capabilities are met prior to execution. By proactively finding capability gaps, it prevents downstream campaign failures.

However, after a rigorous review of the subsystem's logic and the existing test suite, several material gaps have been identified. The current test suite focuses heavily on "Happy Path" scenarios and idealized configurations. This document categorizes and details the identified missing test cases across four major edge-case vectors:
1. Null/Undefined/Empty Values
2. Type Coercion and Data Malformation
3. User Request Extremes
4. State Conflicts and Race Conditions

Each section contains a detailed analysis, including the hypothetical behavior of the current code, potential mitigation strategies, and draft code for tests that should be implemented.

At the conclusion of the vector analysis, this journal evaluates the performance capabilities of the system concerning its ability to withstand extreme boundary conditions.

---

## 2. Null/Undefined/Empty Input Vectors

### 2.1 The `DetectGaps` Function and Null Task Definitions

**Analysis:**
The `DetectGaps` function currently has a single test for empty tasks (`TestToolPregenerator_DetectGaps_EmptyTasks`), which successfully proves that passing an empty slice `[]TaskInfo{}` does not panic. However, it fails to evaluate passing explicitly `nil` as the slice. Go's slice iteration handles `nil` seamlessly, but evaluating explicitly passing `nil` ensures that future developers do not add `len(tasks)` calculations prior to nil-checks that would trigger panics.

Furthermore, there is a gap concerning how `DetectGaps` handles tasks that are not nil, but are empty. Specifically, `TaskInfo.Description`. The function `analyzeTaskForGaps` heavily relies on:
`descLower := strings.ToLower(task.Description)`

If `task.Description` is entirely whitespace, empty `""`, or contains only control characters, the current pattern-matching falls through, and returns an empty slice. While this behavior is logically correct (no gaps detected), it must be formally encoded in tests to prevent regressions where a blank description causes an index out-of-bounds error in future heuristics.

**Proposed Test Implementations:**
```go
func TestToolPregenerator_DetectGaps_NilTasks(t *testing.T) {
    pregenerator := NewToolPregenerator(nil, nil, nil)
    ctx := context.Background()

    // Explicitly test nil slice
    var nilTasks []TaskInfo
    gaps, err := pregenerator.DetectGaps(ctx, "Test goal", nilTasks, nil)

    if err != nil {
        t.Errorf("DetectGaps with nil tasks should not error: %v", err)
    }
    if gaps == nil {
        t.Error("Expected non-nil empty slice for nil tasks")
    }
}

func TestToolPregenerator_DetectGaps_EmptyDescription(t *testing.T) {
    pregenerator := NewToolPregenerator(nil, nil, nil)
    ctx := context.Background()

    tasks := []TaskInfo{
        {
            ID: "empty-desc-task",
            Description: "",
            Type: "implement",
        },
        {
            ID: "whitespace-desc-task",
            Description: "   \t\n  ",
            Type: "implement",
        },
    }

    gaps, err := pregenerator.DetectGaps(ctx, "Test goal", tasks, nil)
    if err != nil {
        t.Errorf("DetectGaps failed on empty descriptions: %v", err)
    }
    if len(gaps) > 0 {
        t.Errorf("Expected no gaps for empty descriptions, got %d", len(gaps))
    }
}
```

### 2.2 The `PregenerateTools` Function and Nil Gaps

**Analysis:**
Similar to `DetectGaps`, `PregenerateTools` tests for an empty `[]ToolGap{}` but does not test for `nil`. More concerningly, it does not test for a `ToolGap` that exists but contains a null-equivalent or empty `Capability` string.
If a capability is empty, `checkBuiltinTools` will likely not match anything, but when `generateTool` is called, it might result in generating a tool with an empty name, or ID like `gen--1234567890`.

**Proposed Test Implementation:**
```go
func TestToolPregenerator_PregenerateTools_NilGaps(t *testing.T) {
    pregenerator := NewToolPregenerator(nil, nil, nil)
    ctx := context.Background()

    var nilGaps []ToolGap
    result, err := pregenerator.PregenerateTools(ctx, nilGaps)

    if err != nil {
        t.Errorf("PregenerateTools with nil gaps should not error: %v", err)
    }
}

func TestToolPregenerator_PregenerateTools_EmptyCapability(t *testing.T) {
    pregenerator := NewToolPregenerator(nil, nil, nil)
    ctx := context.Background()

    gaps := []ToolGap{
        {
            ID: "gap-empty-cap",
            Capability: "",
            Description: "Valid description",
            Confidence: 0.9,
            Priority: 0.9,
        },
    }

    // Test behavior when Capability is empty string
    // Ideally, the system should reject this before Ouroboros call
    result, err := pregenerator.PregenerateTools(ctx, gaps)
    // Add assertions based on expected graceful degradation
}
```

---

## 3. Type Coercion / Data Malformation Vectors

### 3.1 Invalid Characters in Tool Capability Identifiers

**Analysis:**
Tool gaps are primarily identified by their `Capability` strings. These strings eventually feed into system IDs and potentially filesystem paths or shell commands during Thunderdome testing or code execution.
If a `Capability` string contains malformed data such as newlines `\n`, null bytes `\x00`, shell injection sequences like `$(rm -rf /)`, or complex unicode control characters, it could lead to severe consequences downstream.

In `generateTool`, the ID is formatted as:
`ID: fmt.Sprintf("gen-%s-%d", gap.Capability, time.Now().Unix())`
If `gap.Capability` contains newlines, the ID will span multiple lines. This breaks structured logging and JSON formatting if not properly escaped, and can cause Mangle rule evaluation to fail if the ID is cast to a Mangle atom and violates schema constraints.

**Proposed Mitigation:**
The system must sanitize or validate `Capability` strings when constructing `ToolGap` objects.

**Proposed Test Implementation:**
```go
func TestGeneratedTool_MaliciousCapabilityFormatting(t *testing.T) {
    pregenerator := NewToolPregenerator(nil, nil, nil)
    // Mock the ouroboros loop to return success

    maliciousCap := "tool\nwith\nnewlines"
    gap := ToolGap{
        ID: "gap-1",
        Capability: maliciousCap,
        Confidence: 0.9,
    }

    // The test should ensure that the resulting tool ID does not contain newlines,
    // or that generateTool throws a validation error for malformed capabilities.
    ctx := context.Background()
    result, err := pregenerator.PregenerateTools(ctx, []ToolGap{gap})

    if err == nil {
        // If it succeeds, verify the ID was sanitized
        for _, tool := range result.ToolsGenerated {
            if strings.Contains(tool.ID, "\n") {
                t.Errorf("Tool ID contains unescaped newlines: %s", tool.ID)
            }
        }
    }
}
```

### 3.2 Mangle Type Coercion Vulnerabilities

**Analysis:**
Because the `ToolPregenerator` interfaces with the Mangle kernel (via `ToolGenerator` and `autopoiesis`), string-to-atom conversion must be rigorously checked. If a string capability matches a Mangle string, it might not match the required Atom type for intent mapping. Passing raw Go strings where Mangle Atoms are required frequently causes zero results (empty joins), leading the Pregenerator to falsely assume a tool does not exist and trigger unnecessary generation loops.

Tests should assert that capability names conform to valid Atom constraints `[a-z][a-zA-Z0-9_]*`.

---

## 4. User Request Extremes Vectors

### 4.1 Massive Task Lists

**Analysis:**
What happens if the system is fed a campaign with an absurdly large number of tasks? Consider a "brownfield request to work on a 50 million line monorepo," resulting in the Campaign Orchestrator decomposing the goal into 100,000 micro-tasks.

The `DetectGaps` function iterates over all tasks:
```go
for _, task := range tasks {
    taskGaps := p.analyzeTaskForGaps(ctx, task, intel)
    // ... array appends ...
}
```
For 100,000 tasks, this loop performs `strings.ToLower(task.Description)` 100,000 times, along with several `strings.Contains` checks. Go's garbage collector will be heavily pressured by the allocations from `strings.ToLower` for each description string.

**Proposed Test Implementation:**
```go
func TestToolPregenerator_DetectGaps_MassiveTasks(t *testing.T) {
    pregenerator := NewToolPregenerator(nil, nil, nil)
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    const numTasks = 100000
    tasks := make([]TaskInfo, numTasks)
    for i := 0; i < numTasks; i++ {
        tasks[i] = TaskInfo{
            ID: fmt.Sprintf("task-%d", i),
            Description: "Implement API parser and fetch data from database",
        }
    }

    start := time.Now()
    gaps, err := pregenerator.DetectGaps(ctx, "Massive Goal", tasks, nil)
    duration := time.Since(start)

    if err != nil && err != context.DeadlineExceeded {
        t.Errorf("Unexpected error: %v", err)
    }

    // Verify performance bounds
    if duration > 3*time.Second {
        t.Logf("Warning: DetectGaps took %v for %d tasks. Consider optimization.", duration, numTasks)
    }
}
```

### 4.2 Massive Task Description String

**Analysis:**
If a single task description contains an extremely long string (e.g., a user pasting a 50MB log file directly into the prompt, resulting in a single massive task description), `strings.ToLower(task.Description)` will allocate a contiguous 50MB byte array. In a memory-constrained environment (like an 8GB laptop running codeNERD locally with heavy LLMs), this could cause an Out-Of-Memory (OOM) panic.

**Mitigation:**
Before calling `strings.ToLower()`, the system should truncate task descriptions to a sensible maximum length (e.g., 10KB), as tool gaps are usually identifiable from the first few paragraphs.

**Proposed Test Implementation:**
```go
func TestToolPregenerator_DetectGaps_OOM_Prevention(t *testing.T) {
    pregenerator := NewToolPregenerator(nil, nil, nil)
    ctx := context.Background()

    // Create a 50MB string
    massiveDesc := strings.Repeat("A", 50*1024*1024)
    tasks := []TaskInfo{
        {
            ID: "massive-task",
            Description: massiveDesc,
        },
    }

    // This should run without OOMing the test runner
    gaps, err := pregenerator.DetectGaps(ctx, "Goal", tasks, nil)
    if err != nil {
        t.Errorf("Error processing massive string: %v", err)
    }
}
```

### 4.3 MaxToolsToGenerate Extremes

**Analysis:**
`MaxToolsToGenerate` limits generation loops. What if `config.MaxToolsToGenerate` is initialized to 0, negative, or `math.MaxInt32`? A value of `0` or negative should result in no tools generated, bypassing the generation loop safely. `math.MaxInt32` could theoretically lead to extreme long runtimes if the system thinks there are billions of gaps (unlikely, but possible through duplicate gap bugs).

Tests should verify how `PregenerateTools` honors mathematical boundaries for its configuration limits.

---

## 5. State Conflicts Vectors

### 5.1 Concurrent Execution / Race Conditions

**Analysis:**
The `ToolPregenerator` instance is constructed via `NewToolPregenerator`. While its primary fields are configuration pointers, what happens if `DetectGaps` or `PregenerateTools` are called concurrently by different Campaign Orchestrator phases or subagents on the same struct instance?

Currently, `p.config` is read concurrently without locks. If `WithConfig()` is called while another goroutine is running `DetectGaps`, this introduces a classic Go race condition on the `config` struct.

**Proposed Mitigation:**
Either `ToolPregenerator` state must be immutable once started, `WithConfig()` should return a deeply cloned copy of the struct rather than modifying a pointer in place (it currently seems to mutate `p.config = config` but returns `p`), or a `sync.RWMutex` is required.
Actually, looking at `WithConfig`:
```go
func (p *ToolPregenerator) WithConfig(config PregeneratorConfig) *ToolPregenerator {
	p.config = config
	return p
}
```
This absolutely causes a data race if `WithConfig` is chained while another agent uses `p`.

**Proposed Test Implementation:**
```go
func TestToolPregenerator_ConcurrentAccess(t *testing.T) {
    pregenerator := NewToolPregenerator(nil, nil, nil)

    var wg sync.WaitGroup
    wg.Add(2)

    go func() {
        defer wg.Done()
        // Continuously read config by calling DetectGaps
        ctx := context.Background()
        for i := 0; i < 1000; i++ {
            pregenerator.DetectGaps(ctx, "Goal", []TaskInfo{}, nil)
        }
    }()

    go func() {
        defer wg.Done()
        // Continuously write config
        for i := 0; i < 1000; i++ {
            pregenerator.WithConfig(PregeneratorConfig{
                MaxToolsToGenerate: i,
            })
        }
    }()

    wg.Wait()
    // Run this test with 'go test -race' to detect the issue.
}
```

### 5.2 Context Cancellation Timing

**Analysis:**
The `GenerationLoop` in `PregenerateTools` checks `ctx.Done()`, but it does so *between* tool generations.
```go
select {
case <-ctx.Done():
    result.Errors = append(result.Errors, "Context cancelled during generation")
    break GenerationLoop
default:
}
tool, err := p.generateTool(ctx, gap)
```
If context is cancelled exactly while `p.generateTool` is blocking (which could take minutes waiting for Ouroboros LLM calls), `generateTool` will theoretically return an error from the underlying network call. The loop will record a failed tool, increment `FailedTools`, append to `UnresolvedGaps`, and *then* the next loop iteration will break.
This means partial state mapping could be misleading (it reports the tool failed validation, rather than explicitly citing context timeout).

Testing the precise boundary where the context times out mid-generation ensures that partial results are correctly formed and goroutines are not leaked.

---

## 6. Performance & Scalability Evaluation

Based on the analysis of the edge cases, the current system is **moderately performant but highly vulnerable to specific load profiles.**

**Strengths:**
- Deduplication logic in `deduplicateGaps` uses an `O(N)` mapping strategy (`seen := make(map[string]*ToolGap)`), which easily scales to thousands of detected gaps without exponential slowdown.
- Preallocation is used appropriately in some areas (`result := make([]ToolGap, 0, len(seen))`), avoiding continuous reallocation penalties during slice growth.

**Weaknesses / Bottlenecks:**
- **String Manipulations:** `strings.ToLower(task.Description)` in `analyzeTaskForGaps` is the primary CPU and memory bottleneck. Go string strings are immutable, meaning this allocation happens entirely newly for every task description. On a monorepo decomposition resulting in tens of thousands of tasks, this will trigger aggressive garbage collection sweeps, resulting in CPU throttling and potential OOM issues on 8GB machines.
- **Data Races:** The `WithConfig` pattern mutating the shared state makes the subsystem dangerous to scale out to concurrent orchestrator shards. This must be refactored to a functional option pattern or return a new struct value.
- **Pattern Matching Inefficiency:** The repetitive `strings.Contains` checks over `descLower` could be optimized by tokenizing the description once or using an Aho-Corasick automaton for multi-pattern searching, which is drastically faster than running `strings.Contains` 20+ times sequentially over a large string.

**Is it performant enough to handle each edge case vector?**
No. It will currently fail (OOM or extreme slowdown) on the "Massive Task Description String" vector and fail (panic via race condition) on the "State Conflicts" concurrent execution vector. For standard, human-sized prompts (5-10 tasks), it is highly performant. But to meet codeNERD's high-assurance, frontier-scale coding goals, the strings allocations and data races must be resolved.

---

## 7. Concrete Implementation Steps

1. **Refactor `WithConfig`:** Modify `WithConfig` to return a new `ToolPregenerator` value pointer to eliminate data races, or use functional options (`func(p *ToolPregenerator)`).
2. **Implement Truncation:** Add a length check before `strings.ToLower()` in `analyzeTaskForGaps`.
3. **Add Validation:** Add regex or string validation to `ToolGap.Capability` during gap formulation to prevent invalid IDs and Mangle-breaking control characters.
4. **Merge Test Gaps:** Implement the missing unit tests directly into `internal/campaign/tool_pregenerator_test.go` to provide continuous negative testing coverage.

## 8. Mangle Logic Re-evaluation

In addition to Go-side testing, the neuro-symbolic boundaries between codeNERD and the Mangle runtime must be tested. The Ouroboros tool generation loop relies on logic facts to orchestrate tool synthesis.

### 8.1 Ghost Facts and Datalog Monotonicity

When pre-generating tools in a loop, if the system does not properly clear the Mangle `factstore` between generations, "ghost facts" from a previous tool generation failure could pollute the current fixpoint logic.

**Analysis:**
If tool A generation writes a fact `tool_status(/toolA, /failed)`, and the engine context is reused for tool B generation, rules evaluating global readiness might trip. A test should verify the isolation of the reasoning contexts across multiple generation iterations within the same orchestrator loop.

**Proposed Mitigation:**
Ensure that each call to `generateTool` instantiates a fresh evaluation context or explicitly filters out ephemeral generation-state facts from the base store.

**Proposed Test Implementation:**
```go
func TestToolPregenerator_MangleContextIsolation(t *testing.T) {
    // This test ensures that generating a failed tool does not pollute
    // the logic state for subsequent tool generations.

    pregenerator := NewToolPregenerator(nil, nil, nil)
    ctx := context.Background()

    gaps := []ToolGap{
        {
            ID: "gap-1",
            Capability: "impossible_tool",
            Confidence: 0.9,
        },
        {
            ID: "gap-2",
            Capability: "simple_tool",
            Confidence: 0.9,
        },
    }

    // Execute pregenerator
    result, _ := pregenerator.PregenerateTools(ctx, gaps)

    // Assert that the simple_tool succeeded even if impossible_tool failed.
    // Assert that the Mangle knowledge graph for simple_tool does not contain
    // artifacts from impossible_tool.
}
```

## 9. Hardware Constraints and Graceful Degradation

### 9.1 Network Disconnects during Pre-generation

**Analysis:**
The `ToolPregenerator` attempts to call an LLM (via `autopoiesis`) to synthesize tools. If the local network disconnects or the LLM API rate limits the system (e.g., HTTP 429 Too Many Requests), the `PregenerateTools` loop could either panic, hang forever, or incorrectly mark all tools as `failed`.

**Mitigation:**
Implement exponential backoff and retry limits within `generateTool`. If a hard network error occurs, `PregenerateTools` should return the specific network error rather than a generic validation error, allowing the orchestrator to pause the campaign rather than aborting it due to missing tools.

### 9.2 File System Permissions for MCP Fallback

**Analysis:**
The `checkExistingResolutions` relies on checking MCP tools (`EnableMCPFallback`). If the filesystem containing the MCP configurations or binaries is read-restricted, the system will silently fail to find fallbacks and attempt to generate a tool that already exists, wasting budget.

**Mitigation:**
Tests should mock an `os.ErrPermission` when querying the `mcpStore` to verify that the system surfaces the error clearly instead of treating it as a standard "tool not found" state.

## 10. Conclusion

By addressing the gaps identified across the four major edge case vectors, along with the Mangle logic isolation and hardware constraint scenarios, the `ToolPregenerator` subsystem can be hardened to meet codeNERD's strict reliability requirements. The implementation of the proposed tests will lock down these boundaries, preventing regressions as the Ouroboros loop logic evolves.
