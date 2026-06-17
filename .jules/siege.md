## 2024-12-27 - Prompt Compiler ↔ LLM Client Boundary
**Learning:** The `TokenBudgetManager` acts as the critical throttle between the JIT-assembled atoms and the LLM's physical context window limit. An oversight here (such as truncating a string halfway through a UTF-8 character or misallocating resources in concurrent calls) directly causes the downstream `LLMClient` to panic or receive a `400 Bad Request`.
**Action:** Enforce strict UTF-8 validity checks and bounds truncation at the very edge of the prompt assembler, treating the TokenBudgetManager as a defensive firewall rather than a simple string trimmer.

## 2024-12-27 - Stale Facts in Cross-Boundary Pipelines
**Learning:** If an LLM call fails mid-stream in the pipeline, the facts asserted into the kernel during the earlier JIT intent classification may linger, causing "ghost context" in subsequent interactive turns.
**Action:** The pipeline MUST include an explicit rollback or retracting deferred function that clears transient `next_action` or `current_intent` facts if the downstream LLM boundary encounters an error.

## 2024-05-26 - VirtualStore FFI and Mangle Type Mismatches
**Learning:** When the `Session Executor` requests tool execution via the `VirtualStore`, the `VirtualStore` must translate tool arguments into Mangle Atoms to assert facts in the kernel. If a tool argument is passed as a generic string instead of a strict Mangle `ast.String` or `ast.Name`, the kernel join silently fails (returns 0 results) rather than erroring out, leading the Executor to believe an action is permitted when it isn't (or vice versa).
**Action:** Write boundary tests that explicitly inject Mangle type confusion (e.g., passing `/string` instead of `"string"`) at the VirtualStore FFI layer to ensure the type-checker catches the error before policy evaluation.

## 2026-06-17 - Prompt Compiler and LLM Client Streaming Lifecycles
**Learning:** The boundary between the `Prompt Compiler` and `LLM Client` involves passing heavily constructed, massive strings (`CompiledContext`). However, if the network stream breaks or the context times out during the `LLMClient.Stream` phase, goroutines can leak if they don't explicitly listen to `ctx.Done()` alongside the slow network channel. A single streaming failure can leave the `Compiler` state hanging or leak resources across multiple turns.
**Action:** When testing the `Compiler` -> `LLMClient` pipeline, always inject simulated network latency and test `context.Cancel()` exactly mid-stream to ensure the boundary guarantees fast teardown.

