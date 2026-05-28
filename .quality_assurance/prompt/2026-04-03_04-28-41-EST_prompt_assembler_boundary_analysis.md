---

remediated: false
subsystem: prompt
---
# Prompt Assembler Subsystem - Boundary Value Analysis & Negative Testing Journal
**Date/Time:** 2026-04-03 04:28:41 EST

## Subsystem Overview
The `PromptAssembler` subsystem (`internal/articulation/prompt_assembler.go`) is responsible for dynamically assembling system prompts from the Mangle kernel state. It queries the kernel for context atoms and shard templates, combining them with session context and user intents to compose fully dynamic system prompts for LLM execution. This mechanism connects the logic core (Mangle) to the LLM (Perception/Articulation layer) and plays a key role in context selection and prompt injection.

It uses a fallback strategy: it attempts JIT prompt compilation via `JITPromptCompiler` when enabled, but can fall back to legacy, hard-coded baseline templates.

## Architectural & Semantic Context
The prompt construction builds an environment that dictates LLM capabilities and boundaries.
If the assembly fails or produces malformed text, the LLM will hallucinate, execute incorrect tools, or bypass safety mechanisms.
This analysis evaluates `prompt_assembler.go` across four major vectors: Null/Undefined/Empty inputs, Type Coercion, User Request Extremes, and State Conflicts.

## 1. Null/Undefined/Empty Inputs

### 1.1 Nil KernelQuerier in `NewPromptAssembler`
**Analysis:** `NewPromptAssembler` checks for a nil `KernelQuerier` and returns an error. However, `NewPromptAssemblerWithJIT` also does this. The check is present and handles `nil` appropriately. But what happens if the kernel queried exists but the underlying DB is closed or returns zero rows for every single query? The `AssembleSystemPrompt` handles zero rows gracefully for templates (using fallbacks), but if all context atoms are missing, it just proceeds with an empty prompt block. Testing must verify that the resulting prompt structure is still valid for LLMs (e.g. no hanging `// KERNEL-INJECTED CONTEXT` headers without content).

### 1.2 Nil/Empty `PromptContext` in `AssembleSystemPrompt`
**Analysis:** The `AssembleSystemPrompt` method checks for a nil input, but the interface accepts `interface{}`. If a `map[string]interface{}` is provided but missing `shard_id` or `shard_type`, it returns an error `fmt.Errorf("missing required fields shard_id or shard_type")`. But what if `shard_id` is an empty string within `PromptContext` directly? `AssembleSystemPrompt` doesn't explicitly fail; it continues. `queryContextAtoms` will query with an empty string, potentially matching `*` or `/_all` wildcards, but missing specific atoms. The stable shard ID logic in `toCompilationContext` handles empty `instanceID` by falling back to `pc.ShardType`. This is fairly resilient, but tests are completely lacking to verify this resilience against empty string inputs directly embedded in the struct.

### 1.3 Empty Fact Arguments
**Analysis:** In `queryShardTemplate` and `queryContextAtoms`, facts are queried and iterated.
`queryShardTemplate` checks `len(fact.Args) < 2`.
`queryContextAtoms` checks `len(fact.Args) < 2`.
`queryAndFormatSpecialistKnowledge` checks `len(fact.Args) < 3`.
What if `fact.Args` elements are empty strings? If the template is `""`, `queryShardTemplate` might return an empty template, causing `AssembleSystemPrompt` to fall back to the hard-coded template. If a context atom is an empty string, it is appended to the prompt as `- \n`. This could confuse the LLM or cause whitespace bloat. A test must verify how the assembler behaves when the Mangle engine returns empty strings.

### 1.4 Missing JIT Compiler but JIT Flag Enabled
**Analysis:** The feature flag `useJIT` can be true while `jitCompiler` is nil (e.g., initialized via `NewPromptAssembler` which sets `useJIT` to `defaultUseJIT` but `jitCompiler` is nil). `JITReady()` correctly checks `pa.useJIT && pa.jitCompiler != nil`. This prevents nil panics. However, testing should explicitly set up this contradicting state and ensure it gracefully degrades to legacy templates without logging spurious panics.

### 1.5 Missing Piggyback Envelope Requirements
**Analysis:** The fallback template is appended with `PiggybackProtocolSuffix`. If JIT is used, `shouldAppendPiggybackProtocol` checks if it's missing `"control_packet"`. If missing, it appends the suffix. But what if the JIT prompt contains `"control_packet"` as part of a comment or a user input string, but lacks the actual schema? The check `strings.Contains(result.Prompt, "\"control_packet\"")` is very naive. Tests must provide edge case prompts that contain `"control_packet"` out of context and verify the behavior.

## 2. Type Coercion

### 2.1 Interface Type Assertions in Facts
**Analysis:** The code frequently asserts types from Mangle facts:
```go
factType, ok := fact.Args[0].(string)
```
If `fact.Args[0]` is a `MangleAtom` instead of a string, this assertion fails, and the fact is silently skipped. Mangle facts often use atoms (e.g., `/coder`). If `KernelQuerier` returns atoms as custom types rather than strings, `PromptAssembler` will silently fail to inject context or templates. This is a critical dissonance risk. `internal/core/kernel.go` and `internal/core/types` define facts. We need tests that mock the kernel returning `types.MangleAtom` instead of `string` to expose these silent drops.

### 2.2 Map Type Assertions in `mapToPromptContext`
**Analysis:** `mapToPromptContext` converts `map[string]interface{}`. It asserts `string`, `int`, `*types.SessionContext`, `*types.StructuredIntent`. If an upstream caller passes an integer for `shard_id`, it will be ignored, leading to an empty `shard_id` and an error. If `semantic_top_k` is passed as a float64 (common when unmarshaling JSON), the `int` assertion fails silently, causing the fallback to default values. Tests must verify how `mapToPromptContext` handles these implicit type conversions from external data sources.

### 2.3 `SessionContext.ExtraContext` Map
**Analysis:** The `ExtraContext` handles frameworks:
```go
rawFws := strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ';' || r == ' ' })
```
If `v` contains strange characters, null bytes, or is poorly formatted, it might produce garbage tags. Tests should verify that garbage characters do not crash the assembler or produce malformed tags that could cause Mangle syntax errors downstream.

## 3. User Request Extremes

### 3.1 Massive Context Injection (OOM/Token Exhaustion)
**Analysis:** The system queries `injectable_context` and `specialist_knowledge` and appends ALL matching atoms to the prompt string. If the kernel contains 10,000 context atoms for `*`, the prompt string will grow massive, causing token exhaustion during LLM inference or an OOM during prompt string building. There is NO hard cap on the number of atoms appended in the legacy assembler (unlike the JIT compiler which has budget limits).
`queryContextAtoms` should have a cap. Tests must simulate a kernel returning 100,000 context atoms and measure if memory usage spikes catastrophically.

### 3.2 Extremely Long Strings in Context
**Analysis:** User intent targets, diagnostic errors, or reflection hits might contain massive strings (e.g., a 10MB base64 string in an error log). `buildSessionContext` appends these directly to the string builder without length limits. This can cause prompt explosion.
`ctx.CompressedHistory` has a check `len(ctx.CompressedHistory) < 1500`, but other fields like `ctx.CurrentDiagnostics` or `ctx.RecentFindings` do not. Tests must inject multi-megabyte strings into these fields and verify if the assembler truncates them or causes an OOM.

### 3.3 Pathological `SemanticQuery`
**Analysis:** A user might provide a 1GB string as a semantic query. `pc.SemanticQuery` is passed to the JIT compiler. The JIT compiler handles token budgets, but if JIT is off, it bypasses it. Actually, `SemanticQuery` isn't used in legacy assembly, so this is mostly a JIT concern. But tests should verify that the assembler structure handles a massive string in `pc.SemanticQuery` without crashing before handing it to JIT.

### 3.4 Infinite Recursion in Template Variables
**Analysis:** Although the current template logic doesn't explicitly do variable substitution (like `{{ .Var }}`), if any future changes or Mangle templates include self-referencing variables, the assembler must not hang. The JIT compiler handles this, but the legacy assembler just concatenates strings. We must test the string concatenation limits to ensure it completes in O(1) time relative to the number of sections.

## 4. State Conflicts

### 4.1 Concurrent Prompt Assembly
**Analysis:** `AssembleSystemPrompt` doesn't mutate `PromptAssembler` state except for telemetry (which uses `_ = pa.jitCompiler.AssertFacts(...)`). However, the `PromptContext` could be mutated concurrently by other subsystems if it contains pointers like `SessionCtx` and `UserIntent`. The assembler reads these without locking. If another goroutine modifies `SessionCtx.CurrentDiagnostics` while `buildSessionContext` is iterating over it, a data race and panic will occur. Tests must be written to run `AssembleSystemPrompt` concurrently while another goroutine mutates the `SessionCtx`.

### 4.2 JIT Compiler Race Condition
**Analysis:** `EnableJIT` and `SetJITCompiler` use `pa.mu.Lock()`. `JITReady` uses `pa.mu.RLock()`.
In `AssembleSystemPrompt`:
```go
if pa.JITReady() {
    cc := pa.toCompilationContext(pc)
    result, err := pa.jitCompiler.Compile(ctx, cc)
```
Between `pa.JITReady()` (which releases the RLock) and `pa.jitCompiler.Compile()`, another goroutine could call `SetJITCompiler(nil)`. This would cause a nil pointer dereference on `pa.jitCompiler.Compile()`. Tests must be written to intentionally trigger this race condition (e.g., using `time.Sleep` or tight loops) to prove the panic exists, so it can be fixed.

### 4.3 Conflicting Kernel Facts
**Analysis:** If the kernel contains multiple `shard_prompt_base` facts for the same shard type, `queryShardTemplate` returns the first one it finds:
```go
if template, ok := fact.Args[1].(string); ok {
    return template, nil
}
```
Mangle evaluation is unordered. The "first" fact is non-deterministic. If an attacker injects a rogue `shard_prompt_base`, the assembler might select it over the legitimate one randomly, leading to prompt hijacking. Tests must populate the kernel with multiple conflicting base templates and assert that the assembler either consistently picks the safest one or logs a warning about ambiguity.

### 4.4 Budget Clamping Race
**Analysis:** In `SetJITBudgets`, the code locks, checks, and updates budgets. However, if the fallback ratio calculations wrap around or go negative (e.g. if `tokenBudget` is set to `math.MinInt64`), the logic:
```go
if pa.tokenBudget > 0 && pa.reservedTokens >= pa.tokenBudget {
    // Clamp to a safe fallback
```
might misbehave. Tests must verify the clamping behavior with extreme edge cases (negative numbers, zero, MaxInt).

## Summary of Actionable Recommendations

1.  **JIT Compiler Data Race Fix**: In `AssembleSystemPrompt`, acquire the `RLock` to retrieve the `jitCompiler` pointer safely, check if it's nil, and then use it. Do not rely on `JITReady()` sequentially before accessing `pa.jitCompiler`.
2.  **Mangle Fact Type Coercion**: Update the type assertions in `queryShardTemplate`, `queryContextAtoms`, and `queryAndFormatSpecialistKnowledge` to handle both `string` and custom atom types (e.g., using `types.ExtractString`).
3.  **Caps on Injected Content**: Implement caps on the number of `injectable_context` atoms and `specialist_knowledge` blocks appended to the string builder. Implement length limits on string fields like `ctx.CurrentDiagnostics` to prevent token exhaustion.
4.  **Deterministic Template Selection**: If multiple `shard_prompt_base` templates exist, implement a conflict resolution strategy (e.g., sorting, or flagging an error/warning) instead of relying on map iteration order.
5.  **Robust Piggyback Check**: Replace `strings.Contains(result.Prompt, "\"control_packet\"")` with a more robust regex or check to ensure the LLM receives the schema, not just a passing mention of the key.
6.  **JSON Unmarshal Float Coercion**: In `mapToPromptContext`, gracefully handle float64 for integer fields (like `semantic_top_k`).

## Testing Strategy for Gaps

To implement the `TEST_GAP`s added to the code, the following test structures should be created:

### `TestPromptAssembler_EmptyInputs`
Create a test case where `shard_id` is `""`. Create another where `fact.Args` is `["", ""]`. Assert that the resulting prompt string is not malformed and that fallbacks are triggered correctly without panicking.

### `TestPromptAssembler_TypeCoercion`
Create a test using `mapToPromptContext` where `map[string]interface{}{"semantic_top_k": float64(10.5)}`. Assert that it handles the coercion gracefully, or fails gracefully without defaulting to zero silently. Similarly, mock a `KernelQuerier` that returns a custom `MangleAtom` type for the first argument of `shard_prompt_base` and verify it doesn't drop the fact.

### `TestPromptAssembler_HugeContextStrings`
Create a `SessionContext` with a 50MB string in `CurrentDiagnostics[0]`. Run `AssembleSystemPrompt` and measure if it completes under 100ms. If it OOMs or hangs, the gap is proven.

### `TestPromptAssembler_RaceCondition`
Spawn two goroutines. Goroutine A calls `AssembleSystemPrompt` in a tight loop. Goroutine B calls `SetJITCompiler(nil)` and `SetJITCompiler(&JITPromptCompiler{})` in a tight loop. This will quickly expose the pointer dereference race.

### `TestPromptAssembler_ConflictingFacts`
Mock a kernel returning 10 different `shard_prompt_base` facts for `/coder`. Ensure the logic doesn't randomly fail or succeed based on Go's internal map iteration randomness.

By addressing these test gaps, the `PromptAssembler` subsystem will become significantly more robust against adversarial or chaotic inputs, ensuring the LLM is always provided with a sane, safe, and constrained environment to operate within.

## Additional Negative Test Considerations

*   **Invalid JIT Compilation Context**: Ensure `toCompilationContext` handles malformed or extremely large `OperationalMode` or `Language` strings derived from user intent, which could cause downstream Mangle issues.
*   **Corrupted Base Templates**: If the fallback string constants are accidentally corrupted during a bad build, we should have a test confirming they are valid schemas or parsable text.
*   **Memory Exhaustion on Telemetry**: The telemetry call `_ = pa.jitCompiler.AssertFacts([]string{...})` might buffer facts infinitely if the underlying DB transaction hangs. A negative test simulating a stuck DB should ensure telemetry doesn't block prompt assembly.

## Final Review Comments

This document serves as the formal boundary value analysis and negative test gap journal for the `PromptAssembler` subsystem. The gaps identified here have been mapped to `// TODO: TEST_GAP:` comments in `internal/articulation/prompt_assembler_test.go` and represent critical areas for future test-driven hardening efforts.

---
End of Journal Entry.
































































































































































































































































\n\n
