## 2024-12-27 - Prompt Compiler ↔ LLM Client Boundary
**Learning:** The `TokenBudgetManager` acts as the critical throttle between the JIT-assembled atoms and the LLM's physical context window limit. An oversight here (such as truncating a string halfway through a UTF-8 character or misallocating resources in concurrent calls) directly causes the downstream `LLMClient` to panic or receive a `400 Bad Request`.
**Action:** Enforce strict UTF-8 validity checks and bounds truncation at the very edge of the prompt assembler, treating the TokenBudgetManager as a defensive firewall rather than a simple string trimmer.

## 2024-12-27 - Stale Facts in Cross-Boundary Pipelines
**Learning:** If an LLM call fails mid-stream in the pipeline, the facts asserted into the kernel during the earlier JIT intent classification may linger, causing "ghost context" in subsequent interactive turns.
**Action:** The pipeline MUST include an explicit rollback or retracting deferred function that clears transient `next_action` or `current_intent` facts if the downstream LLM boundary encounters an error.

## 2024-12-27 - Prompt Compiler ↔ LLM Client Boundary
**Learning:** The `TokenBudgetManager` acts as the critical throttle between the JIT-assembled atoms and the LLM's physical context window limit. An oversight here (such as truncating a string halfway through a UTF-8 character or misallocating resources in concurrent calls) directly causes the downstream `LLMClient` to panic or receive a `400 Bad Request`.
**Action:** Enforce strict UTF-8 validity checks and bounds truncation at the very edge of the prompt assembler, treating the TokenBudgetManager as a defensive firewall rather than a simple string trimmer.

## 2024-12-27 - Stale Facts in Cross-Boundary Pipelines
**Learning:** If an LLM call fails mid-stream in the pipeline, the facts asserted into the kernel during the earlier JIT intent classification may linger, causing "ghost context" in subsequent interactive turns.
**Action:** The pipeline MUST include an explicit rollback or retracting deferred function that clears transient `next_action` or `current_intent` facts if the downstream LLM boundary encounters an error.
