---
surface: "PromptCompiler_LLMClient"
mode: "boundary"
subsystems_tested: ["prompt", "perception"]
blast_radius: "critical"
remediated: false
---

# Prompt Compiler ↔ LLM Client Boundary Analysis

## 1. System Interaction Map

*   `prompt.JITPromptCompiler.Compile(ctx, ...)`: Generates the `CompilationResult` containing atoms and tool definitions.
*   `prompt.TokenBudgetManager.Fit(atoms, totalBudget)`: Truncates and selects atoms to fit within the `totalBudget`.
*   `perception.LLMClient.Complete(ctx, prompt)` / `CompleteWithTools(ctx, prompt, ...)`: Accepts the compiled string and tool definitions.
*   `perception.LLMClient.Stream(ctx, ...)`: Asynchronously yields chunks via a channel.

## 2. Contract Analysis

*   **Budgeting vs Model Limits:** The `TokenBudgetManager` enforces a token budget. The `LLMClient` relies entirely on this manager to ensure the resulting string does not exceed the model's actual context window.
*   **UTF-8 Integrity:** The `prompt.FinalAssembler` (or downstream truncators) MUST produce valid UTF-8 strings. The `LLMClient` assumes valid UTF-8 and may fail catastrophically (e.g., HTTP 400 Bad Request) if sent malformed byte sequences.
*   **Protocol Overhead Reservation:** The system relies on the Piggyback Protocol (JSON). The `TokenBudgetManager` must reserve sufficient headroom for the LLM to generate the Piggyback JSON structure. If it over-allocates the input context, the LLM might run out of output tokens, truncating the JSON and causing `processPiggybackControlPacket` to fail, dropping critical `next_action` state.
*   **Streaming Lifecycle:** The `LLMClient.Stream` method returns a channel. If the context `ctx` is canceled, the goroutine feeding that channel must exit immediately, without leaking.

## 3. Failure Mode Enumeration

*   **Temporal (Streaming Leak):**
    *   **Failure:** `ctx` is canceled mid-stream during network I/O, but the `LLMClient` goroutine doesn't check `ctx.Done()` alongside the slow network read.
    *   **Result:** Goroutine leak. Repeated failures exhaust system memory.
*   **Semantic (UTF-8 Corruption):**
    *   **Failure:** `TokenBudgetManager` or `FinalAssembler` truncates a massive context string exactly in the middle of a multi-byte UTF-8 character (e.g., an emoji or CJK text).
    *   **Result:** `LLMClient` sends malformed JSON to the provider API, resulting in a 400 error. The session crashes.
*   **Ordering / Resource Exhaustion (Protocol Truncation):**
    *   **Failure:** `TokenBudgetManager` uses the entire token budget for the input prompt, leaving insufficient `max_tokens` for the LLM output to complete the Piggyback JSON.
    *   **Result:** The LLM output is truncated mid-JSON. `articulation.ProcessLLMResponseAllowPlain` fails to parse the control packet. `next_action` facts are lost. The orchestrator hangs or fails to progress the TDD loop.
*   **Partial (OOM on Pathological Input):**
    *   **Failure:** 1,000,000 atoms are fed into `TokenBudgetManager.Fit()`.
    *   **Result:** Sorting or allocating slices for this massive input causes an OOM panic before the LLM is even called.
*   **Corruption (Concurrent Map Read/Write in Compiler):**
    *   **Failure:** Two concurrent executor turns attempt to compile prompts using the same `JITPromptCompiler` instance.
    *   **Result:** Data race in `JITPromptCompiler`'s cache or internal state if mutexes aren't held, causing a panic.

## 4. Adversarial Scenario Design

1.  **Scenario: UTF-8 Mid-Character Truncation**
    *   **Contract Violated:** UTF-8 Integrity.
    *   **Mechanism:** Feed atoms containing only 4-byte emojis, set a budget that precisely splits an emoji.
    *   **Expected:** `FinalAssembler` cleanly bounds the string to a valid rune boundary.
    *   **Severity:** P0.
2.  **Scenario: Streaming Goroutine Leak on Cancel**
    *   **Contract Violated:** Streaming Lifecycle.
    *   **Mechanism:** Start a stream, read 1 chunk, cancel the context, wait 100ms, and verify no goroutines are blocked sending to the channel.
    *   **Expected:** The streaming goroutine exits immediately upon `ctx.Done()`.
    *   **Severity:** P1.
3.  **Scenario: Piggyback Protocol Starvation**
    *   **Contract Violated:** Protocol Overhead Reservation.
    *   **Mechanism:** Set `totalBudget` to the exact hardware limit, provide a massive prompt, and force a small `max_tokens` for the response.
    *   **Expected:** `TokenBudgetManager` reserves mandatory headroom; the prompt is truncated to ensure JSON can complete.
    *   **Severity:** P0.
4.  **Scenario: 1,000,000 Atom Pathological Input**
    *   **Contract Violated:** Resource Exhaustion.
    *   **Mechanism:** Call `Fit()` with 1M atoms.
    *   **Expected:** The `maxAtomsInput` limit truncates it quickly without OOM or CPU spinning.
    *   **Severity:** P2.
5.  **Scenario: Zero Token Budget Panic**
    *   **Contract Violated:** Budgeting.
    *   **Mechanism:** Call `Fit()` with 0 or negative budget.
    *   **Expected:** Graceful error, no panic.
    *   **Severity:** P2.
6.  **Scenario: Extreme Concurrent Compilation**
    *   **Contract Violated:** State Corruption.
    *   **Mechanism:** 100 goroutines call `Compile()` simultaneously on the same `JITPromptCompiler`.
    *   **Expected:** No `-race` panics; cache handles contention gracefully.
    *   **Severity:** P1.
7.  **Scenario: All Atoms Mandatory Exceeds Budget**
    *   **Contract Violated:** Budgeting.
    *   **Mechanism:** Provide more `PriorityMandatory` atoms than the budget allows.
    *   **Expected:** `Fit()` returns an error rather than silently dropping mandatory atoms or exceeding the budget.
    *   **Severity:** P1.
8.  **Scenario: Provider Switch Mid-Session**
    *   **Contract Violated:** LLM Client Consistency.
    *   **Mechanism:** Swap the `types.LLMClient` implementation from OpenAI to Gemini mid-session and pass the same compiled prompt.
    *   **Expected:** Tool call schemas are parsed correctly by both providers without panic.
    *   **Severity:** P2.
9.  **Scenario: Malformed JSON from LLM**
    *   **Contract Violated:** Protocol parsing.
    *   **Mechanism:** Mock LLM to return `{"control_packet": { "mangle_updates": [ "missing_quote(/bad) ]}}`.
    *   **Expected:** `processPiggybackControlPacket` degrades to raw surface text gracefully without crashing.
    *   **Severity:** P1.
10. **Scenario: Invalid Tool Call Names**
    *   **Contract Violated:** LLM tool adherence.
    *   **Mechanism:** Mock LLM to invoke tool `doesntexist`.
    *   **Expected:** Executor logs error, does not panic, and returns error to LLM.
    *   **Severity:** P1.
11. **Scenario: Token Counter Discrepancy**
    *   **Contract Violated:** Budget precision.
    *   **Mechanism:** `getTokenCount` vastly underestimates true tokens.
    *   **Expected:** The LLM client API returns HTTP 413 (Payload Too Large). The integration test must assert the system handles this HTTP error gracefully and doesn't crash the session.
    *   **Severity:** P1.
12. **Scenario: Network Timeout During Compilation**
    *   **Contract Violated:** Context propagation.
    *   **Mechanism:** Cancel context while `Compile()` is performing vector search.
    *   **Expected:** `Compile()` returns `context.Canceled` immediately.
    *   **Severity:** P2.
13. **Scenario: Negative Headroom Reservation**
    *   **Contract Violated:** Budget constraints.
    *   **Mechanism:** Call `SetReservedHeadroom(-5000)`.
    *   **Expected:** Clamped to 0; no silent budget inflation.
    *   **Severity:** P3.
14. **Scenario: Huge Single Atom**
    *   **Contract Violated:** Fit algorithm.
    *   **Mechanism:** Provide one `PriorityHigh` atom that is 5x the total budget.
    *   **Expected:** Atom is skipped (or truncated), other atoms are processed.
    *   **Severity:** P2.
15. **Scenario: Stream Channel Deadlock**
    *   **Contract Violated:** Channel operations.
    *   **Mechanism:** Client doesn't read from stream channel, provider keeps sending.
    *   **Expected:** The stream goroutine drops messages or blocks safely without leaking if canceled.
    *   **Severity:** P1.

## 5. Cascading Failure Analysis

*   **P0 (Piggyback Protocol Starvation):** If the JSON is truncated, the orchestrator loses the `task_status` updates. The campaign orchestrator spins, waiting for a phase transition that never arrives. The shard manager keeps the executor alive, starving API slots for other requests. The system eventually deadlocks or hits a hard max-turn limit.
*   **P1 (Streaming Goroutine Leak):** If the network connection is flaky and we leak one goroutine per retry, over a 1,000-turn automated TDD loop, we leak 1,000 goroutines and their associated massive `CompiledContext` string buffers. This causes a silent OOM kill of the `codeNERD` process.
