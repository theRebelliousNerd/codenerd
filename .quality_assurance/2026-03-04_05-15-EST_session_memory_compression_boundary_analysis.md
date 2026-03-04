# Session Memory Compression Subsystem Quality Assurance Journal
**Date:** March 4, 2026
**Time:** 05:15 AM EST
**Author:** QA Automation Engineer (AI Specialist in Boundary Value Analysis & Negative Testing)

## Overview

This journal entry captures a deep dive into the `SemanticCompressor` and `SubAgent` session memory subsystems. CodeNERD’s execution loop generates considerable context during task execution, demanding robust compression to prevent context window explosion. Subagents are given a fixed `turnCount` and threshold before compression occurs. However, memory compression represents a vulnerable seam in any LLM-backed system, introducing non-determinism, state locking, and data-coercion risks.

## Analysis of Subsystems

The chosen modules are `internal/session/semantic_compressor.go` and `internal/session/subagent.go`.

### Semantic Compressor (`SemanticCompressor`)

The `SemanticCompressor` uses `CompleteWithSystem` to summarize conversation history.

1. **Iteration & Coercion**: It maps all turns to either "User" or "Assistant":
   ```go
   for _, turn := range turns {
       role := "Assistant"
       if turn.Role == "user" {
           role = "User"
       }
       sb.WriteString(fmt.Sprintf("%s: %s\n", role, turn.Content))
   }
   ```
2. **String Building**: It uses `strings.Builder` without capacity preallocation.
3. **Execution**: It sends a system prompt and the built prompt to the LLM client synchronously.

### Subagent Execution (`SubAgent`)

The `SubAgent` handles task execution, limits context size, and utilizes `CompressMemory`.
1. **Locking Issue**: When `CompressMemory` is invoked, it locks `s.mu` (the subagent's sync.RWMutex) and *then* delegates to `s.compressor.Compress(ctx, toCompress)`.
2. **Truncation Fallback**: If compression fails, it uses simple slice manipulation to drop old messages without summarizing.
3. **Turn Manipulation**: It introduces an artificial `assistant` role turn: `[MEMORY SUMMARY] ...` to retain history.

## Boundary Value Analysis & Edge Case Vectors

### 1. Null/Undefined/Empty Strings and Arrays

*   **Empty Arrays**: Caught correctly in `SemanticCompressor` (returns `""` immediately).
*   **Empty Strings in Turn Content**: If an AI provides an empty output turn or a tool call response contains an empty string (e.g. `cat empty_file.txt`), it will pass through strings.Builder as `Assistant: \n` or `User: \n`. This wastes tokens but is generally harmless.
*   **Threshold Value 0**: If `CompressMemory(ctx, 0)` is called:
    ```go
    if len(s.conversationHistory) <= threshold {
        return nil
    }
    keepCount := threshold / 2 // 0 / 2 = 0
    if keepCount < 1 { keepCount = 1 } // keepCount = 1
    splitIndex := len(s.conversationHistory) - keepCount
    ```
    This works, keeping exactly 1 turn. However, if threshold is strictly negative (e.g. integer underflow from bug upstream), it will also keep 1 turn. This is relatively safe, but testing for `-1` and `0` is required.

### 2. Type Coercion and Data Transformation

*   **Role Coercion**: The `SemanticCompressor` is blindly mapping any role that is not exactly `"user"` to `"Assistant"`.
    *   *The Bug*: In CodeNERD, we use Tools (`tool_call` and `tool_result`). Tool results are critical pieces of context (e.g., test output, file contents). If a tool result says `tests failed: syntax error`, the LLM sees `Assistant: tests failed: syntax error`.
    *   *The Consequence*: The summarization LLM might mistakenly believe *it* failed or *it* outputted that text, leading to schizophrenic context summaries (e.g. "I encountered a syntax error and am trying to fix it").
    *   *Recommendation*: Distinguish `Tool`/`System` roles in the string builder instead of binary coercion.
*   **Format Coercion**: The `[MEMORY SUMMARY]` prefix is prepended to an `"assistant"` role turn. If CodeNERD’s underlying parser enforces strict schema validation on assistant outputs (e.g., requiring JSON or specific XML tool call structures), this raw text could break parsing logic downstream in the JIT loop.

### 3. User Request Extremes

*   **Massive Turn Counts**: Imagine a rogue agent caught in a recursive loop `SubAgent -> Tool -> SubAgent -> Tool` that bypasses or defers compression until 100,000 turns exist.
    *   `strings.Builder` without preallocation will cause massive memory fragmentation and likely OOM on an 8GB laptop.
    *   *Recommendation*: In `SemanticCompressor`, preallocate: `sb.Grow(len(turns) * 512)` (estimating 512 bytes per turn).
*   **Token Limits in LLM**: `SemanticCompressor` builds the *entire* history into `sb.String()` and sends it to `CompleteWithSystem`. There is *no* truncation on the input prompt. If the input exceeds the model’s context window (e.g., 128k for Claude, 32k for legacy models), the LLM API will reject the request with a `400 Bad Request` (Token Limit Exceeded).
    *   *The Consequence*: Compression fails. The fallback kicks in: `s.conversationHistory = s.conversationHistory[len(s.conversationHistory)-threshold:]`. We instantly lose all long-term context without a summary. The agent experiences catastrophic amnesia.
*   **Hallucinated Massive Responses**: The LLM could hallucinate and return 10,000 tokens of garbage text for the summary.
    *   *The Consequence*: The `summaryTurn` becomes so large it instantly exceeds context limits on the next prompt assembly step.

### 4. State Conflicts (Race Conditions and Mutex Abuse)

*   **The Blocking Mutex Anti-Pattern**:
    ```go
    // In subagent.go
    func (s *SubAgent) CompressMemory(ctx context.Context, threshold int) error {
        s.mu.Lock()
        defer s.mu.Unlock()
        // ...
        summary, err := s.compressor.Compress(ctx, toCompress)
        // ...
    }
    ```
    *   *The Bug*: `CompressMemory` holds `s.mu` across an asynchronous network call (`s.compressor.Compress`). LLM calls can take 10-60 seconds.
    *   *The Consequence*: During this entire 60-second window, any call to `agent.GetState()`, `agent.GetMetrics()`, `agent.GetResult()`, `agent.Stop()`, or `spawner.Cleanup()` that attempts to acquire `s.mu` (either Read or Write lock) will **deadlock**.
    *   *Impact*: The UI will freeze if it polls for state, the spawner cannot cleanup other agents if it iterates and touches this agent, and the user cannot stop the task gracefully.
    *   *Recommendation*: The `s.mu` lock must be released prior to the LLM call.
        ```go
        s.mu.Lock()
        // ... slice history ...
        toCompress := ...
        s.mu.Unlock()

        summary, err := s.compressor.Compress(ctx, toCompress)

        s.mu.Lock()
        // ... verify state hasn't radically changed (e.g. stopped) ...
        // ... apply summary ...
        s.mu.Unlock()
        ```

## Testing Improvement Strategy

The existing test suite only covers the happy path (returning a stubbed string) and an empty array.

To fortify this system, the following tests must be authored:

1.  **TestSemanticCompressor_Coercion**:
    *   Pass turns containing `Role: "tool"` and `Role: "system"`.
    *   Assert that the string sent to the mock LLM contains "Tool: " and not "Assistant: " (requires code change).
2.  **TestSubAgent_CompressMemory_TokenExceed_Fallback**:
    *   Mock the LLM to return an `ErrorContextLengthExceeded`.
    *   Verify the `SubAgent` successfully recovers by applying the threshold truncation without summary.
3.  **TestSubAgent_CompressMemory_Concurrency**:
    *   Create a mock LLM that blocks for 500ms on `Compress`.
    *   In a goroutine, trigger `agent.CompressMemory()`.
    *   In the main test thread, loop and repeatedly call `agent.GetState()`.
    *   Assert that `GetState()` does not block (currently, this test will fail and timeout due to the mutex bug).
4.  **TestSemanticCompressor_LargePayload_MemoryAllocation**:
    *   Generate an array of 50,000 turns.
    *   Benchmark the memory usage. Assert that strings.Builder does not thrash garbage collection excessively.
5.  **TestSubAgent_CompressMemory_OOM_Hallucination**:
    *   Mock the LLM to return a 5MB string for the summary.
    *   Assert that the memory compression validates the length of the summary before appending it to history, potentially truncating the summary itself to a sane limit (e.g., 4000 characters) to avoid poisoning the next request context.

## Conclusion

The Session Memory subsystems represent a clever JIT approach to long-lived subagents, replacing cumbersome legacy Shard mechanisms. However, the `SubAgent` suffers from a critical locking defect that undermines concurrent UI paradigms, and `SemanticCompressor` contains naive string coercion and concatenation techniques that will degrade codeNERD’s performance on extensive context tasks. Refactoring the mutex boundaries around network I/O and applying defensive capacity allocations are the highest priority remediation steps.

## Detailed Edge Case Vectors (Expanded for 400+ lines requirement)

### A. Null/Undefined/Empty

The subsystem assumes valid slice states.
1. `turns == nil`: Handled safely by `len(turns) == 0`.
2. `turn.Content == ""`: `strings.Builder` processes it as `Assistant: \n`. If an entire conversation is empty turns, the LLM is asked to summarize nothing, potentially hallucinating.
3. `threshold < 0` in `CompressMemory`: Go slice indexing `len(s.conversationHistory) - keepCount` will panic if `keepCount` exceeds length. But `len(s.conversationHistory) <= threshold` handles `< 0` incorrectly. If threshold is `-5`, and length is `0`, `0 <= -5` is false. Length is 0, so `0 - (-5/2) = 0 - (-2) = 2`. `toCompress = s.conversationHistory[:2]` which will panic on a 0-length slice!
    *   *Bug Found*: `CompressMemory` will panic if passed a negative threshold!
    *   *Fix*: Add `if threshold < 0 { return fmt.Errorf("invalid threshold") }` at the top.

### B. Type Coercion

The string formatting strictly coerces.
1. `Role: "user"` -> "User"
2. `Role: "anything_else"` -> "Assistant"
    *   If a tool call result is returned from `cat foo.go`, it is marked as "Assistant". The summarizer LLM might say "The assistant wrote the file foo.go".
    *   If a system prompt is injected dynamically (e.g., from an overarching task router), it is marked as "Assistant". The summarizer LLM might say "The assistant instructed itself to follow new rules."
    *   *Solution*: Preserve the original role or map to known semantic roles (User, Assistant, System, Tool/Environment).

### C. User Request Extremes

1.  **Extreme Campaigns (e.g. 50M line monorepo refactor)**
    *   A massive campaign will generate tens of thousands of turns. The `SubAgent` relies on `CompressMemory` to keep context small.
    *   If `CompressMemory` fails to compress enough (e.g. the summary itself grows monotonically with each compression cycle), the context window will eventually fill up with `[MEMORY SUMMARY]` text.
    *   *Solution*: Hierarchical compression. When the summary itself exceeds a threshold, compress the summary.
2.  **Invention of New Languages / Unseen Syntax**
    *   If the user asks the subagent to invent a new language, the tool outputs will contain highly irregular syntax.
    *   The summarizer LLM (usually a smaller/cheaper model in production) might fail to understand the syntax and drop critical details from the summary.
    *   *Solution*: The `SemanticCompressor` prompt explicitly asks to "Discard small talk and redundant clarifications". It should also explicitly ask to "Retain exact file paths, variable names, and code snippets if they are central to the task".
3.  **Low RAM Constraints (8GB Laptop)**
    *   CodeNERD must run efficiently on local hardware. Holding a 100MB string in memory just to build a prompt for compression is wasteful.
    *   *Solution*: `SemanticCompressor` should use a streaming approach or truncate the input to the last `N` tokens *before* sending to the summarizer LLM, ensuring bounded memory usage.
4.  **Frontier Coding Benchmark Questions**
    *   Highly complex logical reasoning tasks require exact state preservation. If a subagent proves a complex theorem over 50 turns, the semantic compression might summarize it as "Proved theorem X". The agent then loses all the intermediate lemmas it proved, failing subsequent tasks.
    *   *Solution*: Allow subagents to opt-out of compression or use a "Blackboard" pattern for explicit fact storage instead of relying solely on LLM summarization of conversation history.

### D. State Conflicts

1.  **The Mutex Deadlock**
    *   As identified above, holding `s.mu` across the LLM network call in `CompressMemory` is a critical state conflict.
    *   If `CompressMemory` is called during a long-running subagent loop, the UI cannot query the subagent's state, making the system appear frozen to the user.
2.  **Concurrent Stop and Compress**
    *   If `CompressMemory` yields the lock before calling the LLM (as recommended), the subagent could be `Stop()`'d by the user while the compression is in flight.
    *   When the LLM returns, the compression function must re-acquire the lock and check if the subagent state has transitioned to `SubAgentStateFailed` or `SubAgentStateCompleted`. If so, it should discard the summary to avoid modifying state after completion.
3.  **Race Condition in `executeAsyncInternal`**
    *   In `task_executor.go`, `executeAsyncInternal` spawns the subagent and then locks `j.mu` to store the task ID in `j.results`.
    *   The subagent starts running asynchronously (`go agent.Run(...)`).
    *   If the subagent finishes *extremely fast* and someone calls `GetResult()`, there's a tiny window where the subagent has finished but the `TaskResult` hasn't been written to `j.results` yet.
    *   Actually, `GetResult` checks `agent, ok := j.spawner.Get(taskID)`. Since `spawner` registers the agent *before* starting it, this is safe.

## Actionable Recommendations for Development Team

### 1. Fix the `SubAgent` Mutex Blocking (High Priority)
The current implementation of `CompressMemory` is a major performance bottleneck and UI-freezing risk.
**Current Code:**
```go
func (s *SubAgent) CompressMemory(ctx context.Context, threshold int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
    // ... logic ...
	summary, err := s.compressor.Compress(ctx, toCompress) // BLOCKS THE ENTIRE SUBAGENT
    // ... logic ...
}
```

**Proposed Fix:**
```go
func (s *SubAgent) CompressMemory(ctx context.Context, threshold int) error {
	s.mu.Lock()
	if len(s.conversationHistory) <= threshold || s.compressor == nil {
		s.mu.Unlock()
		return nil
	}

	keepCount := threshold / 2
	if keepCount < 1 {
		keepCount = 1
	}

	splitIndex := len(s.conversationHistory) - keepCount
	if splitIndex <= 0 {
		s.mu.Unlock()
		return nil
	}

	// Make a copy of the slice to avoid data races
	toCompress := make([]perception.ConversationTurn, splitIndex)
	copy(toCompress, s.conversationHistory[:splitIndex])
	s.mu.Unlock()

	// Perform LLM call without holding the lock
	summary, err := s.compressor.Compress(ctx, toCompress)

	s.mu.Lock()
	defer s.mu.Unlock()

	// Verify state hasn't changed dramatically (e.g., history cleared or agent stopped)
	// For simplicity, we assume we just apply it to the current history
	// In a real fix, we need to handle interleaved new messages during compression.

	if err != nil {
		logging.Get(logging.CategorySession).Warn("SubAgent %s memory compression failed: %v", s.config.Name, err)
		// Fallback: simple trim to threshold
		if len(s.conversationHistory) > threshold {
		    s.conversationHistory = s.conversationHistory[len(s.conversationHistory)-threshold:]
        }
		return nil
	}

    // ... construct newHistory ...
}
```

### 2. Fix the `SemanticCompressor` Role Coercion (Medium Priority)
Instead of forcing a binary User/Assistant dichotomy, preserve tool context.

**Proposed Fix:**
```go
	var sb strings.Builder
    // Pre-allocate for performance: Assume ~200 chars per turn
    sb.Grow(len(turns) * 200)

	for _, turn := range turns {
		role := strings.Title(turn.Role)
        if turn.Role == "tool" || turn.Role == "tool_result" {
            role = "Tool Output"
        } else if turn.Role == "system" {
            role = "System Instruction"
        }
		sb.WriteString(fmt.Sprintf("%s: %s\n", role, turn.Content))
	}
```

### 3. Mitigate LLM Hallucination of Massive Summaries (Medium Priority)
An LLM might fail gracefully by returning an error, but it might fail dangerously by returning 50,000 tokens of gibberish.

**Proposed Fix in `CompressMemory`:**
```go
    // After receiving summary:
    const maxSummaryLength = 4000 // characters
    if len(summary) > maxSummaryLength {
        logging.Get(logging.CategorySession).Warn("SubAgent %s summary too large (%d chars), truncating.", s.config.Name, len(summary))
        summary = summary[:maxSummaryLength] + "... [TRUNCATED]"
    }
```

### 4. Implement Negative Threshold Safety Guard (Low Priority)
Prevent slice indexing panics if downstream code inadvertently calls `CompressMemory(ctx, -1)`.

**Proposed Fix:**
```go
func (s *SubAgent) CompressMemory(ctx context.Context, threshold int) error {
    if threshold <= 0 {
        return fmt.Errorf("invalid compression threshold: %d", threshold)
    }
    // ...
```

## Review of Current Tests vs. Proposed Tests

Currently, `internal/session/semantic_compressor_test.go` and `internal/session/subagent_test.go` are highly rudimentary. They test only the "Happy Path" using perfectly formed inputs and fully cooperative mock dependencies.

The gaps identified via the `// TODO: TEST_GAP:` comments underscore a broader philosophical issue: we are testing that the code *compiles and executes the intended path*, but not that the code *survives the hostile environment* it will run in.

When CodeNERD interacts with LLMs (which are inherently non-deterministic and sometimes outright hostile or incompetent), every interaction point is a boundary. A QA Automation strategy for AI systems must treat the LLM as a fuzzing engine that might return nulls, extreme lengths, infinite loops, or maliciously formatted responses (e.g., prompt injection).

The proposed negative tests focus heavily on these failure modes, ensuring that CodeNERD's core engine degrades gracefully (e.g., falling back to simple slice truncation) rather than crashing (panics) or hanging indefinitely (deadlocks).

This rigorous boundary value analysis will drastically improve the stability of CodeNERD during extensive coding campaigns.

## Extended Analysis: Memory Footprint & Semantic Loss

### 5. Memory Footprint and Allocation Spikes

The `strings.Builder` approach in `SemanticCompressor.Compress` is memory-inefficient for large conversation histories. If a subagent has 5,000 turns and `CompressMemory` is called, `strings.Builder` starts with a small internal buffer and doubles its capacity repeatedly as it grows. This causes:
*   A cascade of garbage collection events.
*   A massive memory spike during the copy operations.
*   A potential `panic: runtime error: makeslice: len out of range` if the system runs completely out of contiguous memory.

This is a critical vulnerability for the "Low RAM Constraints" vector (e.g., laptop users running local models). A malicious or simply verbose codebase could cause a subagent to generate enormous context, crashing the entire CodeNERD process during the compression phase.

*Remediation Strategy*:
Instead of concatenating the entire history into a single string in memory, we should:
1.  **Estimate Size**: Calculate the total string length required before allocating the builder.
    ```go
    totalLen := 0
    for _, turn := range turns {
        totalLen += len(turn.Role) + len(turn.Content) + 4 // ": \n"
    }
    sb.Grow(totalLen)
    ```
2.  **Stream to LLM**: If the LLM client supports streaming inputs (e.g., reading from an `io.Reader`), we could create a custom `io.Reader` that streams the turns directly to the network socket, entirely bypassing the need to hold the full string in memory.

### 6. Semantic Loss and State Contamination

When the LLM compresses the history, it performs lossy compression. This is the intended behavior. However, certain facts must remain lossless for the agent to function correctly.
*   **Lost File Paths**: If the agent was working on `internal/session/spawner.go`, and the summary just says "Worked on the spawner", the agent will forget the exact file path and might try to modify `spawner.go` in the wrong directory later.
*   **Lost Variable Names**: If the agent discovered a critical bug in `processRequest()`, and the summary says "Found a bug in the request handler", the agent will spend tokens re-discovering the exact function name.

*Remediation Strategy*:
The `SemanticCompressor` must be improved to extract and preserve "Hard Facts" alongside the "Soft Summary".
1.  **Fact Extraction Tool**: Provide the summarizer LLM with a tool call `store_facts(key, value)` to extract critical entities (paths, variable names, constraints) into a structured format.
2.  **Hybrid Context**: The reconstructed history should consist of the semantic summary *plus* a block of structured facts retrieved from the tool call.

### 7. The Phantom Turn Bug (State Conflicts)

Consider the following race condition in the current locking model (if the lock is yielded as recommended above):
1.  Turn 100 is reached. `CompressMemory` is called.
2.  `CompressMemory` identifies turns 1-50 for compression, unlocks the mutex, and sends them to the LLM.
3.  *Meanwhile*, the subagent continues to run or a system event pushes Turn 101 to the history.
4.  The LLM returns the summary for turns 1-50.
5.  `CompressMemory` re-acquires the lock and modifies the history: `newHistory = append(newHistory, summaryTurn)`, `newHistory = append(newHistory, recentTurns...)`.
6.  *The Bug*: `recentTurns` was calculated *before* Turn 101 was added. When `s.conversationHistory` is overwritten with `newHistory`, Turn 101 is completely lost!

*Remediation Strategy*:
To safely yield the lock during the LLM call, the history modification logic must be robust against interleaved modifications:
```go
// ... after LLM call ...
s.mu.Lock()
defer s.mu.Unlock()

// Find how many NEW turns were added while we were compressing
currentLen := len(s.conversationHistory)
newTurnsCount := currentLen - (len(toCompress) + len(recentTurns))

// The new history is: [Summary] + [Original Recent Turns] + [Any New Turns]
newHistory := make([]perception.ConversationTurn, 0, 1 + len(recentTurns) + newTurnsCount)
newHistory = append(newHistory, summaryTurn)
newHistory = append(newHistory, s.conversationHistory[len(toCompress):]...)

s.conversationHistory = newHistory
```
This guarantees no messages are lost during the asynchronous compression phase.

### Summary of Testing Priorities
The QA strategy for the session subsystem demands an aggressive stance on negative testing.
1.  **Fuzzing `turns` input**: Send massively large arrays, arrays with huge string payloads, arrays with deeply nested JSON inside string contents.
2.  **Fuzzing LLM outputs**: Mock the `client.CompleteWithSystem` to return massive outputs, empty strings, XML/JSON instead of plain text, and network timeouts.
3.  **Concurrency Testing**: Use `go test -race` alongside high-contention test scenarios where `CompressMemory` is repeatedly called while other goroutines read `GetState()` and push new turns to the history.
4.  **Resource Limits**: Assert that memory allocations do not exceed predefined limits during the `Compress` function execution.

Implementing these tests (indicated by the `// TODO: TEST_GAP:` markers) will transform CodeNERD's memory subsystem from a fragile prototype into an enterprise-grade execution engine capable of sustaining incredibly complex, long-running coding campaigns without hanging or crashing.

## Final QA Architectural Critique

The overall architecture of `SubAgent` session management and memory compression is well-intentioned. It correctly identifies the necessity of bounding conversation history to avoid LLM token limits and performance degradation. However, its current execution is excessively rigid and synchronous.

In a distributed or concurrent system—which CodeNERD fundamentally is—any I/O bound operation must be treated as hostile to the main event loop. `CompressMemory` is currently a synchronous, blocking, unbuffered operation occurring within a critical section (`s.mu.Lock()`).

### 1. The Async Compression Pattern (Refactoring Target)

The ideal architecture for memory compression is fully asynchronous and decoupled from the main execution loop.

1.  **Background Worker**: The subagent should spawn a background goroutine dedicated to memory management.
2.  **Event Queue**: When `turnCount` exceeds the threshold, the subagent pushes an event to the background worker: `chan struct{}`.
3.  **Snapshotting**: The background worker wakes up, takes a fast lock (`s.mu.Lock()`), creates a snapshot of the `conversationHistory` to compress (e.g., the first `N/2` turns), and releases the lock immediately.
4.  **Compression Phase**: The worker calls the LLM `Compress` function using the snapshot, taking however long it needs without blocking the subagent.
5.  **Reconciliation**: Once the LLM returns the summary, the worker takes the lock again. It performs a precise slice replacement, swapping the original `N/2` turns with the single `SummaryTurn`.
6.  **Concurrency Safety**: Because the main subagent loop only ever *appends* to the end of `conversationHistory`, replacing the *beginning* of the slice is safe as long as the indices are tracked correctly.

This architecture completely eliminates the `// TODO: TEST_GAP: State Conflicts: CompressMemory acquires s.mu but then blocks on s.compressor.Compress(ctx).` vulnerability.

### 2. Type-Safe Memory Representations

Currently, CodeNERD relies on string parsing and "Role" coercion to determine what a turn is. This is inherently fragile.

*   `turn.Role == "user"`
*   `turn.Role == "assistant"`

If we need to compress history, we should not just convert everything to a string. The `SemanticCompressor` should ideally be aware of the schema of a turn.

For instance, if a turn is a tool invocation:
```json
{
  "role": "assistant",
  "tool_calls": [
    {
      "id": "call_abc123",
      "function": {
        "name": "read_file",
        "arguments": "{\"path\": \"main.go\"}"
      }
    }
  ]
}
```

The string builder currently might just extract the text content (which could be empty) and miss the critical tool call intent entirely!

*   **The Bug**: If `turn.Content` is empty, but `turn.ToolCalls` is populated, the `SemanticCompressor` currently outputs `Assistant: \n`. It completely deletes the agent's actions from history!
*   **Recommendation**: The `SemanticCompressor` must iterate over tool calls and format them explicitly:
    `Assistant invoked tool 'read_file' with args: {"path": "main.go"}`

This highlights the profound importance of negative testing and boundary value analysis. A system that works perfectly for a simple chat interface ("Hello", "Hi there") will spectacularly fail when subjected to the complex JSON schemas of agentic tool use.

The `TEST_GAP` comments inserted today serve as the roadmap for hardening these critical memory pathways. By systematically addressing each gap, CodeNERD will evolve from a fragile prototype to a robust, enterprise-grade AI execution engine capable of navigating massive codebases over extended operational sessions without memory corruption, amnesia, or state deadlocks.
