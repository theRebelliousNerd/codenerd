# QA Boundary Value Analysis & Negative Testing Journal
## Target Subsystem: Perception (`internal/perception`)
## Date & Time: 2026-08-25 00:22:53 EDT

## 1. Executive Summary

This journal entry captures a deep-dive Boundary Value Analysis (BVA) and negative testing evaluation of the `internal/perception` subsystem within the codeNERD architecture. The perception layer is critical as it acts as the transducer between raw, unstructured natural language input (and potentially malicious payloads) and the highly structured, logic-driven Mangle inference kernel.

The current test suite (`tests/e2e/perception_adversarial_e2e_test.go`, `tests/e2e/perception_stateful_e2e_test.go`, `tests/e2e/perception_contract_e2e_test.go`) demonstrates a strong baseline for functional correctness and basic adversarial resilience (e.g., decoy JSON poisoning, input truncation). However, a rigorous application of Boundary Value Analysis reveals significant gaps in how the system handles extreme edge cases, structural malformations, type coercions, and concurrent state mutations.

The objective of this analysis is to identify these gaps, document their potential impact, and provide concrete architectural and testing recommendations to harden the perception layer against a broader spectrum of failure modes.

## 2. Methodology & Subsystem Context

The analysis follows these core testing vectors:
1. **Null/Undefined/Empty**: Evaluating the system's resilience against the absence of expected data structures.
2. **Type Coercion**: Probing the boundaries of dynamic typing and JSON unmarshaling where types diverge from the strict schema.
3. **User Request Extremes**: Simulating operational boundary conditions (e.g., massive context windows, ultra-high complexity instructions).
4. **State Conflicts**: Investigating race conditions and temporal inconsistencies in the stateful perception tracking.
5. **System Resource Exhaustion**: Probing the limits of transducer allocations.
6. **Mangle Syntactic Collision**: Exploring how edge characters affect rule parsing.
7. **Encoding Boundaries**: Testing limits of UTF-8, UTF-16, and non-printable structures.

### Subsystem Understanding: The Transducer Model

The `internal/perception` package implements a `Transducer` pattern. It takes natural language string inputs and "transduces" them into strongly typed `Understanding` structures (via LLM JSON output), which are then converted into Mangle `Fact` assertions (e.g., `user_intent`, `focus_resolution`).

The critical path:
1. `ParseIntentWithContext(input, history)` -> calls LLM client.
2. LLM responds with JSON containing an `Understanding` block.
3. JSON is unmarshaled into the Go `Understanding` struct.
4. Validation occurs.
5. The struct is mapped into Mangle Atoms via `.ToFact()`.
6. Facts are asserted into the kernel.

Mangle's type system is disjoint. An atom (e.g., `/fix`) is fundamentally different from a string (`"fix"`). The transducer's role is to safely bridge the permissive world of LLM text generation and the rigid, typed world of the Mangle engine. Failure to do so results in silent evaluation failures (empty join results) or, worse, logical corruption (Mangle injection).

## 3. Vector Analysis: Null / Undefined / Empty

The current test suite focuses heavily on malformed or malicious content. It lacks rigorous testing for the *absence* of content.

### 3.1 Empty Input & Blank Contexts

**Scenario:** The user submits a completely empty string, a string containing only whitespace/newlines, or a completely empty conversation history array.

**Current Coverage:** Minimal. Tests assume *some* semantic input is provided.

**Risk:**
- If an empty string bypasses the LLM call and defaults to a zero-value `Understanding`, what does the routing logic do with an empty `PrimaryIntent`?
- If the LLM generates a valid JSON envelope but the inner `understanding` object is entirely null or missing mandatory fields.

**BVA Edge Cases to Implement:**
1. `input = ""` (Absolute empty)
2. `input = " \n \t \r "` (Whitespace only)
3. `history = []ConversationTurn{}` (Explicitly empty history vs. `nil` history)
4. LLM Response: `{ "surface_response": "Hello" }` (Missing `understanding` block entirely).

**Architectural Consideration:** The transducer must implement strict schema validation *before* attempting to construct a Mangle Fact. A zero-value `Understanding` struct should never reach `.ToFact()`. If the LLM fails to provide the block, a safe default fallback (e.g., `ActionType: "explain", Domain: "general"`) must be enforced, or an explicit `ParsingError` must be returned to trigger a re-prompt.

### 3.2 Null Fields in JSON Parsing

**Scenario:** The LLM client returns valid JSON, but specific scalar fields are explicitly set to `null` rather than omitted or empty strings.

**Current Coverage:** The test suite tests "Decoy JSON" but does not test schema adherence regarding nullability.

**Risk:**
- Go's `encoding/json` handles `null` differently depending on the struct field type. If a field like `Scope.Target` is parsed as `null`, it becomes `""`. However, if custom unmarshaling is used later, or if a slice (like `UserConstraints`) is `null` vs `[]`, it can lead to nil pointer panics during iteration.

**BVA Edge Cases to Implement:**
1. `{ "understanding": { "primary_intent": null } }`
2. `{ "understanding": { "scope": null } }`
3. `{ "understanding": { "signals": null } }`
4. `{ "understanding": { "implicit_assumptions": null } }`
5. `{ "understanding": { "user_constraints": null } }`
6. `{ "understanding": { "suggested_approach": { "tools_needed": null } } }`
7. `{ "understanding": { "suggested_approach": { "context_needed": null } } }`
8. `{ "understanding": { "suggested_approach": { "supporting_shards": null } } }`

## 4. Vector Analysis: Type Coercion

The `Understanding` schema expects specific types (strings, booleans, floats, arrays of strings). LLMs, especially under adversarial load or systemic drift, can hallucinate incorrect JSON types.

### 4.1 Type Mismatch in Schema

**Scenario:** The LLM provides an integer where a boolean is expected, or an object where an array is expected.

**Current Coverage:** The `TestE2E_Perception_Adversarial_OutOfVocabulary` test checks for unknown string values (OOV strings), but not fundamental JSON type mismatches.

**Risk:**
- Standard Go `json.Unmarshal` will throw an error if a field defined as `[]string` receives an object `{}`.
- If the parsing fails, does the perception layer crash, or does it recover gracefully? If it fails, does it retry the LLM call with a schema enforcement prompt?

**BVA Edge Cases to Implement:**
1. `confidence: "high"` (String instead of Float64)
2. `signals.is_question: "true"` (String instead of Boolean)
3. `user_constraints: "no modifications"` (String instead of Array of Strings)
4. `scope.level: 1` (Integer instead of String)
5. `primary_intent: ["implement", "debug"]` (Array instead of String)
6. `signals.is_multi_step: 1` (Integer instead of Boolean)
7. `confidence: 1` (Integer instead of Float64)

**Architectural Consideration:** The transducer should utilize robust JSON schema validation (perhaps via `jsonschema` or strict unmarshaling settings) and implement a bounded retry loop when type coercion fails, prompting the LLM to correct its formatting.

### 4.2 Mangle Atom/String Dissonance

**Scenario:** The `ToFact()` method converts fields like `Category` and `Verb` into Mangle Atoms (`core.MangleAtom`).

**Current Coverage:** `TestE2E_Perception_Adversarial_MangleInjection` tests injecting rules into the `Target` string.

**Risk:**
- If the LLM produces a `Verb` that contains characters invalid for a Mangle Atom identifier (e.g., spaces, punctuation), `core.MangleAtom(i.Verb)` might produce an invalid representation or panic depending on the kernel's enforcement.

**BVA Edge Cases to Implement:**
1. `Verb: "fix bug"` (Contains space)
2. `Verb: "/fix"` (Already contains slash, leading to double-slash if not normalized)
3. `Category: "mutat!on"` (Invalid characters)
4. `Verb: ""` (Empty string becoming a zero-length atom)
5. `Category: "   "` (Whitespace-only atom)

## 5. Vector Analysis: User Request Extremes

The perception layer must remain performant and stable under extreme load, both in terms of input size and conceptual complexity.

### 5.1 Maximum Payload Thresholds

**Scenario:** The user submits a prompt approaching or exceeding the LLM's context window, or a campaign plan that requires decomposing into thousands of tasks.

**Current Coverage:** `TestE2E_Perception_Adversarial_InputTruncation` covers a 100KB input string and ensures truncation happens.

**Risk:**
- While truncation works for the user string, what about the `history` array? If the history contains 10,000 turns, does the memory footprint of marshalling that history cause an OOM?
- What if a single turn in the history has a 10MB payload?

**BVA Edge Cases to Implement:**
1. `history` array with 50,000 extremely short turns (Array length extreme).
2. `history` array with 1 turn containing 50MB of text (String length extreme).
3. `Target` reference containing 10,000 comma-separated filenames.
4. `Target` reference exceeding maximum filesystem path length constraints.

**Performance Consideration:** The `ParseIntentWithContext` must implement strict token budgeting *before* serialization. The history window trimming must act on token counts (or byte heuristics), not just a static number of turns. The current system might crash on serialization before truncation can even happen if the structs themselves become too large in memory.

### 5.2 Conceptual Extremes (The "Turing Tar-pit")

**Scenario:** The user asks a question so deeply convoluted, layered with double-negatives and hypothetical abstractions, that the LLM's classification logic breaks down.

**Current Coverage:** None explicitly testing conceptual overload.

**Risk:**
- The LLM might output conflicting signals (e.g., `is_question: true`, `action_type: "modify"`, `confidence: 0.1`).
- The `confidence` score might drop near zero. Does the system handle low-confidence transductions differently?

**BVA Edge Cases to Implement:**
1. "If I were to theoretically not want to refrain from undeleting the database, how wouldn't I do it?"
2. Input in an unsupported, extremely obscure language, or raw binary representation.
3. A prompt containing a valid Python script that *prints* a request, rather than stating the request.
4. "I need you to write a poem about how you will refactor the codebase to use rust." (Conflicting domains).

## 6. Vector Analysis: State Conflicts and Race Conditions

The `UnderstandingTransducer` is a stateful component. It tracks conversation history and "current understanding".

### 6.1 Concurrent Mutations

**Scenario:** Multiple overlapping requests are sent to the same session/transducer instance simultaneously (e.g., via rapid UI interaction or API scripting).

**Current Coverage:** `TestE2E_Perception_Stateful_ConcurrentIsolation` tests 100 concurrent calls and checks for panics and target mismatches.

**Risk:**
- While the test checks for panics, it acknowledges target mismatches are "expected under queue contention". This is a significant design flaw if a user's intent is routed based on a different user's concurrent request.
- The `lastUnderstanding` internal state might be overwritten non-deterministically, causing the *next* turn in the conversation to inherit context from a completely different concurrent request.

**BVA Edge Cases to Implement:**
1. Two rapid-fire requests: Req A modifies state, Req B reads state. Ensure Req B does not read intermediate or dirty state from Req A.
2. Context cancellation mid-flight during a state update. Does it leave the transducer in a corrupted state?
3. What happens if multiple goroutines trigger the stability bypass simultaneously?

**Architectural Consideration:** The Transducer should ideally be stateless per request, or state must be heavily compartmentalized per correlation ID / request ID. Relying on a shared `lastUnderstanding` in a concurrent environment violates boundary isolation principles.

### 6.2 Temporal Inconsistencies (Ghost Facts)

**Scenario:** As documented in the stateful tests, "routing facts accumulate". This is a critical state conflict.

**Current Coverage:** The test logs "DOCUMENTED GAP: X current_understanding facts accumulated".

**Risk:**
- Mangle is monotonic within an evaluation step. If `current_understanding` facts accumulate across conversational turns without retraction, the ruleset will eventually trigger multiple conflicting derivations (e.g., both "tester shard" and "coder shard" are derived as primary).
- The "stability bypass" bug noted in the tests shows that stale state prevents the system from recognizing a context shift.

**BVA Edge Cases to Implement:**
1. Force an explicit rollback of a conversational turn. Does the state revert?
2. Submit a request that generates an error (e.g., API timeout). Does the failed attempt pollute the history or fact store?
3. Inject two contradictory intents consecutively and ensure only the latter influences routing derivations.

## 7. Extended Analysis: Edge Constraints & Limits

### 7.1 Encoding & Serialization Faults
**Scenario:** Input contains non-UTF8 bytes or complex surrogate pairs.
**Current Coverage:** Unchecked.
**Risk:** JSON serialization failure leading to dropped queries.
**BVA Edge Cases:**
1. Input with raw binary `\x00\xFF`.
2. Emojis and complex Unicode planes in scope targets.
3. Overly long surrogate pairs attempting to overflow string parsers.

### 7.2 The Ouroboros Feedback Loop
**Scenario:** System generates a response that immediately triggers itself.
**Current Coverage:** Autopoiesis tests handle this partially.
**Risk:** Infinite loops during perception transduction if internal logic reflects state continuously.
**BVA Edge Cases:**
1. A target reference that points to the log file being actively written.
2. An intent that recursively triggers a nested intent transduction.

### 7.3 Deep Nesting & Recursion Limits
**Scenario:** A scope target that contains recursive or excessively deeply nested path references.
**Risk:** Stack overflows during resolution.
**BVA Edge Cases:**
1. Path resolution for `a/b/c/d/...` exceeding 1024 levels.
2. A JSON response with extremely deep nested arrays for `user_constraints`.

### 7.4 Network Interruption Constraints
**Scenario:** Connection terminates mid-stream during LLM chunking.
**Risk:** Partial JSON parsing creating invalid Mangle facts.
**BVA Edge Cases:**
1. TCP termination exactly halfway through the `understanding` JSON payload.
2. Connection timeout during a long streaming intent response.

## 8. Deep Dive: Memory Profiling Needs

While the functional bounds of the transducer are partially tested, the mechanical bounds (memory and CPU) are largely unverified. A transducer allocating massive strings on the heap continuously can trigger severe GC pauses in a long-running codeNERD daemon.

### 8.1 Heap Escape Velocity
Every time `ParseIntentWithContext` runs, the JSON string must be unmarshaled into a large struct array. If the history is long, does the Go runtime escape these allocations to the heap?
- Negative test: Run 1,000 rapid ParseIntent calls and profile the heap. Does memory plateau, or does it steadily leak?

### 8.2 Fact Store Eviction
When `current_understanding` accumulates, how does the Mangle Fact store react? If 10,000 ghost facts sit in the graph, does query resolution time increase linearly, or exponentially?
- Negative test: Assert 10,000 overlapping routing facts and benchmark query resolution.

## 9. Contextual Integrity under Stress

The transducer must resolve references (e.g., "fix *that* bug").

### 9.1 Ambiguity Saturation
**Scenario:** The user says "update the file" in a repository with 5,000 files modified recently.
**Risk:** The LLM's `FocusResolution` might return an enormous array, overwhelming the Mangle logic phase.
**BVA Case:** Simulate a focus context where `Candidates` array equals the entire 50,000 file monorepo.

## 10. Conclusion and Actionable Roadmap

The Boundary Value Analysis highlights that while the perception layer handles "happy path" and basic adversarial attacks well, it is vulnerable to structural malformations (nulls, type coercion) and complex state management issues. By implementing the identified edge case tests and addressing the architectural recommendations, the system can achieve a significantly higher degree of resilience and determinism.

**Action Items:**
1. Implement rigorous structural fuzzing tests for the JSON payload.
2. Introduce typed coercion guards that prevent Mangle injection.
3. Migrate state tracking from shared pointers to context-isolated scopes to prevent cross-turn contamination.

## 11. Final Verification Matrix
- Null Strings: [ ] To Do
- Null Arrays: [ ] To Do
- Type Int->String: [ ] To Do
- Type String->Bool: [ ] To Do
- Context Overflow: [ ] To Do
- State Contention: [ ] To Do
- Unicode Planes: [ ] To Do
- Zero-Value Defaults: [ ] To Do
- Missing Top-Level JSON Keys: [ ] To Do
- Malformed Escape Characters: [ ] To Do
- Unbounded Fact Accumulation: [ ] To Do
- Floating Point Threshold Boundaries (0.0, 0.999999, 1.0, 1.000001): [ ] To Do


## 12. Extended Test Case Definitions (For Automated QA)

### 12.1 Negative Test Case: The Empty Array Coercion
**Preconditions:** Transducer initialized, mock LLM configured.
**Input payload:** `{"understanding": {"primary_intent": "implement", "user_constraints": [""], "implicit_assumptions": null}}`
**Expected Behavior:** System gracefully defaults `implicit_assumptions` to an empty slice, and filters out the empty string from `user_constraints`.
**Failure Mode:** `panic: runtime error: index out of range` or nil pointer dereference during constraint iteration.

### 12.2 Boundary Test Case: Confidence Threshold Precision
**Preconditions:** Ruleset requires confidence > 0.8.
**Input payload:** `{"understanding": {"primary_intent": "implement", "confidence": 0.8000000000000001}}`
**Expected Behavior:** The float64 value retains precision across JSON unmarshaling and into Mangle fact assertion.
**Failure Mode:** The value rounds down to `0.8`, failing a strict inequality check in the kernel rules.

### 12.3 State Conflict: Rapid Turnover
**Preconditions:** Transducer instance shared across two goroutines simulating a rapid user double-click.
**Action:** Goroutine A submits Intent 1. Goroutine B submits Intent 2 exactly 1ms later.
**Expected Behavior:** Transducer either explicitly locks and processes sequentially, or rejects Request B with an in-flight error.
**Failure Mode:** Request B's understanding overwrites Request A's state mid-processing, causing Request A to be routed according to Request B's instructions.

### 12.4 Negative Test Case: Mangle Syntactic Poisoning via Target
**Preconditions:** LLM generates a target intended to break parsing.
**Input payload:** `{"understanding": {"scope": {"target": "auth.go). evil_rule(X) :- fact(X)."}}}`
**Expected Behavior:** `sanitizeFactArg` explicitly strips `.`, `)`, and `:-` or fully escapes the string, preventing the string from breaking out of the fact declaration syntax.
**Failure Mode:** The kernel parses `evil_rule` as valid IDB logic and executes it during the next query phase.

### 12.5 Boundary Test Case: Maximum Scope Candidates
**Preconditions:** File candidate list passed to `ResolveFocus` contains 1,000,000 entries.
**Action:** User inputs "refactor the auth flow".
**Expected Behavior:** The transducer restricts the number of candidates passed to the LLM prompt to a safe maximum (e.g., 100) based on token budget.
**Failure Mode:** The system attempts to format 1,000,000 candidates into the system prompt, causing an Out Of Memory panic or exceeding the LLM API token limit, resulting in a dropped request.

### 12.6 Boundary Test Case: Deeply Nested History Context
**Preconditions:** Conversation history contains 100 turns.
**Action:** User inputs a new request.
**Expected Behavior:** `ParseIntentWithContext` successfully truncates the history array to the maximum allowed window (e.g., 5 turns) before processing.
**Failure Mode:** The full 100 turns are serialized, exceeding the prompt token budget or severely degrading processing latency.

### 12.7 Negative Test Case: Unrecognized Semantic Types
**Preconditions:** LLM hallucinated a semantic type not in the vocabulary.
**Input payload:** `{"understanding": {"semantic_type": "quantum_entanglement"}}`
**Expected Behavior:** The parsing layer flags the unrecognized vocabulary term and applies a safe fallback (e.g., `general`) or triggers a re-prompt.
**Failure Mode:** The unrecognized string is asserted as a fact, but no Mangle rules match it, resulting in a silent failure where the intent is simply ignored by the router.

### 12.8 Negative Test Case: Extreme Urgency Escalation
**Preconditions:** User bypasses system urgency checks via prompt injection.
**Input payload:** `{"understanding": {"signals": {"urgency": "EXTREME_CRITICAL_BYPASS"}}}`
**Expected Behavior:** Urgency values are strongly typed or clamped to known constants (low, normal, high, critical).
**Failure Mode:** The unknown string bypasses urgency filters, or worse, triggers unintended edge-case logic that allows the request to bypass safety gates.

### 12.9 Boundary Test Case: Zero-Length Verb Fact
**Preconditions:** LLM returns an empty string for the Verb field.
**Input payload:** `{"understanding": {"primary_intent": "implement"}}` (with Verb missing/empty)
**Expected Behavior:** The transducer populates Verb based on `primary_intent` if missing, or explicitly sets it to `/unknown`.
**Failure Mode:** `core.MangleAtom("")` creates an invalid or zero-length atom, which causes the Mangle engine parser to crash upon ingestion.

### 12.10 Resource Exhaustion: The infinite array
**Preconditions:** An attacker crafts a massive array within constraints.
**Input payload:** `{"understanding": {"user_constraints": ["A", "B", ... 10,000 items]}}`
**Expected Behavior:** A hard limit on slice length during unmarshaling or validation prevents excessive memory allocation.
**Failure Mode:** The system allocates large continuous memory blocks, leading to GC thrashing and eventual OOM.

### 12.11 Temporal Conflict: Stale Focus Resolution
**Preconditions:** A user asks to modify a file, then deletes the file via external tools, then asks a follow up question.
**Action:** The focus resolution attempts to lock onto a file that no longer exists but is present in the `lastUnderstanding` context.
**Expected Behavior:** The transducer validates the existence of the focus target before asserting it, or the kernel handles the missing file gracefully.
**Failure Mode:** The system asserts a `focus_resolution` on a missing file, causing the coder shard to crash when attempting to read it.

### 12.12 Type Coercion: The "False" String
**Preconditions:** LLM outputs a string representation of a boolean.
**Input payload:** `{"understanding": {"signals": {"is_question": "false"}}}`
**Expected Behavior:** The JSON parser rejects the type mismatch, or a custom unmarshaler explicitly handles the string coercion safely.
**Failure Mode:** The struct field remains at its default value (false), which might be correct in this specific instance, but would fail silently if the input was `"true"`, leading to incorrect routing.

### 12.13 Memory Leak: Unbounded Subagent Spawning
**Preconditions:** The intent routing logic contains a cyclical reference.
**Action:** The user requests a task that requires a supporting shard, which in turn requests the primary shard.
**Expected Behavior:** The routing rules detect the cycle and terminate the request with an error.
**Failure Mode:** The system spawns an infinite tree of subagents, rapidly exhausting system memory and goroutines.

### 12.14 Type Coercion: The Numeric String
**Preconditions:** LLM outputs a string representation of a number.
**Input payload:** `{"understanding": {"confidence": "0.95"}}`
**Expected Behavior:** The JSON parser rejects the type mismatch or a custom unmarshaler handles the coercion.
**Failure Mode:** The field defaults to `0.0`, causing high-confidence requests to be treated as low-confidence and routed to a fallback path unnecessarily.

### 12.15 Boundary Test Case: Unicode Normalization
**Preconditions:** User requests focus on a file with decomposed Unicode characters in its name.
**Action:** The string "é" (e + acute accent) is passed.
**Expected Behavior:** The system normalizes the string to a composed character before comparing it against filesystem candidates.
**Failure Mode:** The string comparison fails, and the focus resolution returns nil, degrading the user experience.

### 12.16 State Conflict: The "Zombie" Intent
**Preconditions:** A request is cancelled mid-execution, but the intent facts were already asserted to the kernel.
**Action:** The user submits a new request.
**Expected Behavior:** The kernel's `current_understanding` is purged before the new request is processed.
**Failure Mode:** The old facts persist, causing the new request to be routed using a mixture of the old and new intents.

### 12.17 Boundary Test Case: The 64KB Token Limit
**Preconditions:** The LLM's context window is exactly 64KB tokens.
**Action:** The system attempts to assemble a prompt that is exactly 64,001 tokens.
**Expected Behavior:** The `PromptAssembler` correctly calculates the budget and truncates the context to fit.
**Failure Mode:** The request is sent to the LLM and rejected with a 400 Bad Request error.

### 12.18 Boundary Test Case: Zero-Length File
**Preconditions:** The focus resolution targets a file that exists but is completely empty (0 bytes).
**Action:** The user requests to "explain this file".
**Expected Behavior:** The transducer processes the intent normally, and the subsequent shard handles the empty file gracefully (e.g., by stating it is empty).
**Failure Mode:** The focus resolution logic assumes a non-zero length for valid targets and rejects the focus, or the resulting prompt crashes due to empty context blocks.

### 12.19 Boundary Test Case: Extremely Long Function Names
**Preconditions:** The focus resolution targets a function with a name exceeding 255 characters.
**Action:** The user requests to "refactor the function".
**Expected Behavior:** The transducer correctly identifies the target and limits the length of the string asserted as a Mangle atom.
**Failure Mode:** The long string is rejected by the Mangle kernel as an invalid atom identifier, or the resulting prompt token budget is skewed.

### 12.20 Negative Test Case: Missing Required JSON Keys
**Preconditions:** The LLM output is valid JSON but is missing the top-level `understanding` key entirely.
**Input payload:** `{"surface_response": "I cannot help with that."}`
**Expected Behavior:** The `json.Unmarshal` process yields a zero-value `Understanding` struct, which is then rejected by the `Validate()` method, triggering a graceful fallback or re-prompt.
**Failure Mode:** The system assumes the struct is valid and asserts empty/default Mangle facts, confusing the routing logic.

### 12.21 Negative Test Case: Malformed Escape Sequences
**Preconditions:** The LLM generates JSON with invalid string escape sequences.
**Input payload:** `{"understanding": {"primary_intent": "fix", "scope": {"target": "auth\x00go"}}}`
**Expected Behavior:** The JSON parser fails, and the system initiates a retry loop to correct the LLM's output format.
**Failure Mode:** The parser silently truncates the string at the null byte, or accepts it, leading to invalid file paths later in the pipeline.

### 12.22 Negative Test Case: Unexpected Top-Level Types
**Preconditions:** The LLM returns a JSON array instead of an object.
**Input payload:** `[{"understanding": {"primary_intent": "fix"}}]`
**Expected Behavior:** The parser detects the type mismatch and handles the error gracefully.
**Failure Mode:** A panic occurs during unmarshaling, crashing the perception transducer.
