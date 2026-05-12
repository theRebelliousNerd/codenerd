---
remediated: false
---
# Northstar Guardian Boundary Value and Negative Testing Analysis
**Date/Time:** 2026-04-07 00:24:45 EST
**Subsystem Reviewed:** Northstar Guardian (`internal/northstar/guardian.go`, `internal/northstar/guardian_test.go`)

## 1. Subsystem Overview and Purpose

The Northstar Guardian subsystem is designed to act as an alignment gatekeeper during the agent's campaign execution. In the context of codeNERD's Creative-Executive Partnership model, the Guardian validates whether the subagent's actions, plans, and task completions are drifting away from the project's core "Vision" (Mission, Problem Statement, Objectives).

It leverages an external LLM for semantic evaluation (`CheckAlignment`) and utilizes an internal state store (`Store`) to keep a historical record of drift events, alignment checks, and task completions. It implements a fallback rule-based threshold evaluation when explicit alignment verdicts are missing.

This review focuses on the current test suite, identifying critical blind spots (Test Gaps) across four primary edge-case vectors: Null/Undefined/Empty Inputs, Type Coercion, User Request Extremes, and State Conflicts. The system's ability to gracefully handle these conditions is paramount to preventing total system failure or silent alignment drift.

---

## 2. Deep Dive: Null / Undefined / Empty Vector

The current test suite (`guardian_test.go`) covers "happy paths" for initialization with and without a Vision, but fails to deeply probe missing internal states and empty payload fields. In a Logic-First neuro-symbolic framework, missing facts or empty parameters can lead to Mangle assertion panics or subtle execution drifts.

### Identified Gaps & Risks

#### A. The Empty Configuration Vulnerability
The `NewGuardian` constructor accepts a `Store` and a `GuardianConfig`. The current test `TestNewGuardian` only passes a well-formed `DefaultGuardianConfig()`.
**Test Gap:** If the configuration loaded from disk is malformed, returning a completely empty `GuardianConfig` struct:
- All thresholds (`WarningThreshold`, `FailureThreshold`, `BlockThreshold`) default to `0.0`.
- The `classifyScore` function:
  ```go
  func (g *Guardian) classifyScore(score float64) AlignmentResult {
      if score >= g.config.WarningThreshold { return AlignmentPassed }
      // ...
  }
  ```
  Since `0.0` is the default threshold, *any* score returned by the LLM (even 0.1 for catastrophic drift) will result in `AlignmentPassed`. This is a silent failure mode that bypasses the entire safety constitutional gate.

#### B. The Null Store Scenario
**Test Gap:** Passing a `nil` store pointer to `NewGuardian`.
While Go is statically typed, nil pointers to interfaces or structs can sneak in during dynamic JIT dependency injection inside `internal/prompt/config_factory.go`.
If `g.store` is nil, calling `g.store.LoadVision()` inside `Initialize()` will trigger a nil pointer dereference, causing a runtime panic that crashes the codeNERD session loop. The test suite needs a test proving that `NewGuardian(nil, config)` either safely rejects creation or `Initialize()` returns a clear architectural error instead of crashing.

#### C. Empty Task Observations
The method `ObserveTaskCompletion(sessionID, taskType, taskDesc, result string)` writes to the underlying store.
**Test Gap:**
If an exhausted or malfunctioning subagent emits an empty task summary due to context window truncation, it sends empty strings for `taskDesc` and `result`.
```go
obs := &Observation{
    Content: fmt.Sprintf("Task: %s\nResult: %s", taskDesc, truncate(result, 500)),
}
```
This generates effectively blank records. The lack of `len(taskDesc) > 0` validation before persisting to `Store` means an agent caught in an empty output loop (a known hallucination failure mode) will flood the SQLite database with useless observations, rapidly bloating the session memory and slowing down historical paging.

#### D. The Empty LLM Response
**Test Gap:** The mock LLM in tests only returns perfectly formatted `SCORE: X` strings.
What if the LLM backend times out but returns a HTTP 200 with an empty body, or simply outputs `\n\n`?
In `parseAlignmentResponse`:
```go
lines := strings.Split(response, "\n")
```
An empty string splits to `[]string{""}`. The parser loops, finds no prefixes, and retains its default initialized values:
`check.Score = 0.7` and `check.Result = AlignmentWarning`.
While defaulting to a warning is safe, it obfuscates the root cause (an LLM failure). The Guardian should ideally distinguish between "LLM evaluated this as a warning" and "LLM failed to respond, defaulting to warning."

#### E. Missing Vision Struct Sub-fields
**Test Gap:** The test suite assumes the Vision struct is perfectly populated.
If `Personas` is an empty slice but the LLM tries to reason about user needs, what does it hallucinate? If `Capabilities` is null, does the LLM fall back to an internal default model of "capabilities" that clashes with the `Mission`? The test suite must mock empty sub-fields to prove `buildAlignmentSystemPrompt` does not generate a malformed prompt (e.g., generating empty headers like `## Target Users\n\n## Planned Capabilities\n\n`).

---

## 3. Deep Dive: Type Coercion Vector

The Northstar Guardian must parse unstructured text from a non-deterministic LLM back into strict Go types (floats, enums). This is a highly vulnerable boundary in any neuro-symbolic system, where string-to-atom dissonance is a known anti-pattern.

### Identified Gaps & Risks

#### A. Numerical Type Coercion in `parseAlignmentResponse`
**Test Gap:** The test suite provides perfect floats (`SCORE: 0.95`).
However, modern LLMs are conversational and may output:
- `SCORE: 9.5e-1` (scientific notation)
- `SCORE: 0.95/1.0`
- `SCORE: ~0.9`
- `SCORE: 95%`
- `SCORE: I give it a 0.8`
The current parsing implementation:
```go
fmt.Sscanf(strings.TrimPrefix(line, "SCORE:"), "%f", &score)
```
`Sscanf` is highly rigid. If it attempts to parse "95%", it will fail to match the float format and leave the `score` variable untouched (it starts at 0.0 before the scan). The check then defaults to `0.7` because the scanned `score` is deemed invalid or skipped.
This silent parsing failure means a heavily drifting subagent returning "SCORE: 10%" might receive a passing mark of 0.7 simply because the LLM formatted the output with a percentage sign.

#### B. JSON Hallucination Coercion
**Test Gap:** Format Coercion (JSON instead of Plain Text).
Newer LLMs (especially GPT-4 or tool-tuned local models) often default to outputting JSON even when asked for plain text, especially if they are system-prompted heavily for JSON in other parts of the codeNERD stack.
If the LLM returns:
```json
{
  "SCORE": 0.5,
  "RESULT": "failed",
  "EXPLANATION": "Drift detected"
}
```
The line-by-line prefix check (`strings.HasPrefix(line, "SCORE:")`) will fail entirely because the line begins with `"SCORE":` or `{"SCORE":`.
The system completely ignores the failure and defaults to `0.7/Warning`.
**Improvement Plan:** The parser should use regex extraction `(?i)SCORE["\s:]*([0-9]*\.?[0-9]+)` or, better yet, the `buildAlignmentSystemPrompt` should leverage the codeNERD Piggyback Protocol to enforce strict JSON schemas, allowing Go to natively `json.Unmarshal` the response.

#### C. Enum Coercion for AlignmentResult
**Test Gap:** The LLM is instructed: `RESULT: <passed|warning|failed|blocked>`.
What if it outputs `RESULT: Pass` or `RESULT: FAIL` or `RESULT: Neutral`?
The code does a strict lowercase comparison. If the LLM says "pass" instead of "passed", the explicit result flag is ignored, and the system falls back to calculating the result from the score. If the score parser also failed, a critical failure might be masked as a warning.

#### D. String Coercion of Suggestions
**Test Gap:** The `SUGGESTIONS` line is split by commas.
```go
for _, s := range strings.Split(sugStr, ",") { ... }
```
If an LLM provides a numbered list, or uses semicolons, the entire list of suggestions becomes a single giant string in the array. This breaks any downstream UI elements that try to render bullet points based on the array length. The test suite must test format-coercion on the suggestion block.

---

## 4. Deep Dive: User Request Extremes Vector

Extremes test the system's performance boundaries, memory utilization, and string manipulation efficiency when faced with massive monorepos or unhinged user prompts.

### Identified Gaps & Risks

#### A. Massive Context Payloads in `CheckAlignment`
**Test Gap:** Testing CheckAlignment with a massive string (e.g. 50MB).
The `subject` and `context` parameters are provided by the JIT tool executor or Transducer. If an agent passes a 50MB log file or an entire 10,000-line source file as the `context`, the `buildAlignmentUserPrompt` uses a `strings.Builder`:
```go
sb.WriteString("## Additional Context\n")
sb.WriteString(context)
```
Concatenating massive strings is generally safe and efficient in Go using `strings.Builder`. However, immediately passing a 50MB string to the `LLMClient.CompleteWithSystem()` will cause severe downstream issues:
1. It will exhaust the Token Budget Manager instantly.
2. It will cause a prolonged HTTP timeout or an immediate HTTP 413 Payload Too Large from the LLM provider.
The Guardian lacks a `max_context_bytes` truncation before dispatch. A test must feed an excessively large string and verify the Guardian defensively truncates it before invoking the LLM.

#### B. Algorithmic Complexity in `calculateRelevance`
**Test Gap:** DoS via contiguous string processing.
`calculateRelevance` implements a naive keyword matching algorithm.
```go
words := strings.Fields(strings.ToLower(source)) // source is Vision
for _, word := range words {
    if strings.Contains(textLower, word) { ... } // textLower is task result
}
```
If the `text` (task result) is extremely large (e.g., a massive minified JS bundle), and the Vision contains 500 words, this is an $O(N \times M)$ operation where M is the byte length of the task.
Furthermore, if the task result is a malicious or coincidental string containing millions of characters *without* spaces, `strings.Fields()` isn't the problem, but running 500 parallel `strings.Contains` over a massive contiguous byte slice is computationally heavy.
Since this function runs synchronously during `ObserveTaskCompletion`, under heavy JIT loop load, this naive nested loop will spike CPU usage and delay the critical path. Tests must provide massive continuous strings to benchmark this latency.

#### C. The Vision Schema Explosion
**Test Gap:** The `buildAlignmentSystemPrompt` iterates over `vision.Personas`, `vision.Capabilities`, `vision.Requirements`, `vision.Risks`, and `vision.Constraints`.
If a user loads a massive enterprise architecture document containing 1,000 capabilities and 500 risks, `strings.Builder` will construct a system prompt that exceeds the context window of most models *before* the user input is even added.
The test suite needs a boundary test that generates a `Vision` object with 10,000 items in the slices, proving that `CheckAlignment` either handles it safely, paginates it, or the Guardian enforces structural limits on the `Vision` object at `UpdateVision` time.

#### D. Extremely High Config Thresholds
**Test Gap:** Boundary testing for configuration limits.
If a user edits the JSON configuration and accidentally sets `PeriodicCheckInterval` to `9999999999999999999` (overflowing an `int64` or `int`), how does `NewGuardian` handle it?
Does it silently overflow to a negative number, causing `ShouldCheckNow` to trigger on every single task because `-123456 <= 0`? The test suite must push extreme types into the `GuardianConfig` to ensure bounds are respected.

---

## 5. Deep Dive: State Conflicts Vector

codeNERD's JIT Clean Loop architecture relies heavily on context isolation, where ephemeral facts are purged between turns. However, the `Guardian` is a persistent sub-system maintaining shared state (`g.state`, `g.vision`) protected by a `sync.RWMutex`.

### Identified Gaps & Risks

#### A. Concurrent Mutex Locking Race Conditions
**Test Gap:** The test suite has a `TestGuardian_ConcurrentAccess` that spins up 10 goroutines doing `HasVision()`, `GetVision()`, and `GetState()`. However, it *only* tests concurrent reads.
It completely fails to test concurrent execution of `CheckAlignment`, `UpdateVision`, and `OnTaskComplete` interleaving together.

**Specific Race Condition in `OnTaskComplete`:**
```go
// 1. Database Update (Outside Lock)
count, err := g.store.IncrementTaskCount()

// ... OS thread swapped here ...

// 2. Memory State Update (Inside Lock)
g.mu.Lock()
if g.state != nil {
    g.state.TasksSinceCheck = count
}
g.mu.Unlock()
```
If two goroutine SubAgents complete tasks simultaneously:
1. Goroutine A calls `IncrementTaskCount` -> DB returns 5.
2. Goroutine B calls `IncrementTaskCount` -> DB returns 6.
3. Goroutine B gets the lock -> sets memory `TasksSinceCheck` to 6.
4. Goroutine A gets the lock -> sets memory `TasksSinceCheck` to 5.
The memory state is now officially lagging behind the Database state. The next periodic check will trigger late. A negative test must simulate high-concurrency writes to expose this state drift between the persistence layer and the in-memory cache.

#### B. Timestamp Collision on Check IDs
**Test Gap:** ID Generation Collisions in High-Throughput Scenarios.
`CheckAlignment` generates IDs using:
```go
ID: fmt.Sprintf("check-%d", time.Now().UnixNano()),
```
While nanosecond collisions are rare in simple CLI usage, codeNERD's Thunderdome testing mode or autopoiesis loops can run massive batch testing across 64+ core machines. In such environments, two subagents might request an alignment check at the exact same nanosecond.
If this occurs, `store.RecordAlignmentCheck` might fail due to a SQLite `UNIQUE` constraint violation on the ID primary key, causing a legitimate alignment check to be silently discarded. The ID generation should be tested with mocked deterministic time, and the code should be upgraded to use UUIDv4 or KSUID to guarantee global uniqueness regardless of clock precision.

#### C. Pointer Bleed in `GetState()`
**Test Gap:** `GetState()` returns a copy of the state using `cloneGuardianState()`, but does it deep copy nested elements?
If `GuardianState` is updated in the future to hold slices or maps (e.g., `RecentDriftIDs []string`), the current clone implementation:
```go
func cloneGuardianState(state *GuardianState) *GuardianState {
    clone := *state
    return &clone
}
```
This is a shallow copy. Any caller receiving the state could append to `RecentDriftIDs`, inadvertently modifying the locked internal memory of the Guardian from an external thread, violating the `sync.RWMutex` contract. The tests must verify that mutating the returned object from `GetState()` has zero impact on the internal state.

#### D. Vision Pointer Mutation in Map/Slice Copying
**Test Gap:** The `cloneVision` function makes an attempt to deep-copy slices:
```go
clone.Personas = make([]Persona, len(v.Personas))
for i, persona := range v.Personas {
    clone.Personas[i] = Persona{
        Name:       persona.Name,
        PainPoints: append([]string(nil), persona.PainPoints...),
        Needs:      append([]string(nil), persona.Needs...),
    }
}
```
This is generally good, but if a developer adds a new pointer field to the `Vision` struct (e.g., `*BusinessMetrics`) and forgets to update `cloneVision()`, the resulting clone will share the pointer. Concurrent threads reading `GetVision()` could mutate `BusinessMetrics` under the lock's nose. A reflection-based test that walks the `Vision` struct and panics if any field is an uncopied pointer is necessary for long-term safety.

---

## 6. Integrating Guardian with codeNERD Piggyback Protocol

Currently, the Guardian interacts with the LLM via `CompleteWithSystem(ctx, systemPrompt, userPrompt)`. This returns unstructured text that we then manually parse.
However, codeNERD's core philosophy (as seen in the `Articulation Transducer`) involves the **Piggyback Protocol** - where the LLM is instructed to return dual-channel output: a surface response for humans, and a control packet for the machine.

### Proposal for Replacing the Fragile Parser
Instead of using `fmt.Sscanf`, the `buildAlignmentSystemPrompt` should be modified to request output matching a specific schema:
```json
{
  "surface_response": "I have evaluated the drift. The subagent is veering off course by focusing on backend optimization instead of the requested frontend styling.",
  "control_packet": {
    "northstar_updates": {
      "score": 0.45,
      "result_enum": "failed",
      "suggestions": ["Pause execution", "Re-prompt subagent with mission statement"]
    }
  }
}
```
This completely eliminates the "Type Coercion" vector. The Guardian would invoke the LLM using codeNERD's native `json.Unmarshal` flow inside the internal `llm` wrapper, instantly failing with a strong schema error if the LLM hallucinates, rather than silently assigning `0.7` and calling it a warning.

---

## 7. Architecture & Performance Recommendations

The Guardian subsystem is architecturally sound in its core intent—acting as a decoupled observer that intercepts task telemetry without blocking the main creative generation loop unless a hard gate is triggered. The use of `sync.RWMutex` combined with `cloneVision()` ensures that readers are not exposed to half-written state mutations.

However, based on the boundary analysis, the performance and safety are vulnerable, requiring the following architectural upgrades:

### A. Pre-computed System Prompts (CPU Optimization)
The system prompt builder (`buildAlignmentSystemPrompt`) dynamically concatenates potentially large slices on *every single alignment check*.
**Recommendation:** Since the Vision rarely changes compared to the frequency of alignment checks (which happen constantly in the background), the base System Prompt string should be cached inside `g.vision` upon initialization and during `UpdateVision`. Rebuilding it via `strings.Builder` on every check wastes garbage collection cycles and CPU time.

### B. Transition from Keyword to Vector Relevance (Memory Optimization)
`calculateRelevance` is explicitly marked as a naive placeholder ("In production, this would use embeddings"). As it stands, its performance is strictly $O(V \times T)$ where $V$ is the number of words in the Vision and $T$ is the byte length of the task result.
**Recommendation:** Until `sqlite-vec` embeddings are integrated, the system should pre-compile the Vision words into a `map[string]struct{}` (a set) during `UpdateVision`. Then, the system only needs to tokenize the incoming text once and perform $O(1)$ map lookups, drastically reducing the CPU penalty of monitoring large file changes.

### C. Defending the LLM Interface (Robustness)
The parsing logic is too optimistic. Relying on `strings.HasPrefix` and `fmt.Sscanf` for LLM outputs guarantees failure when transitioning to different models (e.g., from Claude 3.5 Sonnet to an open-weights model like Llama 3).
**Recommendation:**
- Implement fallback regex parsers that scan for keywords anywhere in the output block.
- Upgrade the alignment check prompt to utilize the codeNERD JSON schema framework, forcing the LLM to return a structured JSON object that can be safely unmarshaled.

### D. Mangle Logic Integration
The Guardian currently checks drift via LLM semantics. However, in codeNERD, safety and execution gating is driven by Mangle facts (e.g., `permitted(Action)`).
**Recommendation:** When the Guardian detects a `DriftCritical` event, it should not merely write to the database. It should emit a Mangle fact back into the `VirtualStore` kernel instance.
```go
if check.Result == AlignmentBlocked {
    g.kernel.Assert(ast.Fact("panic_state", ast.String("Guardian blocked execution due to extreme drift.")))
}
```
This allows the Mangle Constitutional rules (`internal/core/defaults/policy.mg`) to instantly revoke tool permissions from the drifting subagent.

---

## 8. Implementation Blueprint for Tests

To fully close the test gaps documented above, the following specific test cases must be written into `internal/northstar/guardian_test.go`:

1.  **`TestGuardian_EmptyConfig_SilentPass`**: Instantiate the Guardian with a completely zeroed configuration and prove that it dangerously allows all checks to pass. (Fix requires adding minimum floor values to config initialization).
2.  **`TestGuardian_ParseAlignment_ScientificNotation`**: Feed `SCORE: 9.9e-1` into the parser and assert that it correctly decodes to 0.99 instead of failing to 0.7.
3.  **`TestGuardian_ParseAlignment_JSONHallucination`**: Feed a fully structured JSON payload and assert the parser can extract the score and result despite the lack of plain-text prefixes.
4.  **`TestGuardian_ParseAlignment_EmptyResponse`**: Send `""` and verify it falls back safely to `0.7/Warning`.
5.  **`TestGuardian_CalculateRelevance_ExtremeString`**: Pass a 10MB string without spaces to `calculateRelevance` and use `testing.T.Deadline()` or explicit timeouts to prove the function does not cause a CPU denial of service.
6.  **`TestGuardian_Concurrent_TaskCompletion_Race`**: Spawn 50 goroutines that simultaneously call `OnTaskComplete` on a slow mock Store, and assert that `g.state.TasksSinceCheck` perfectly matches the DB counter at the end of the test, proving the memory/DB synchronization is thread-safe.
7.  **`TestGuardian_BuildPrompt_EmptyVisionStructs`**: Provide a Vision struct with entirely nil/empty slices, and verify the resulting prompt doesn't have broken markdown formatting.
8.  **`TestGuardian_ContextTruncation_Limits`**: Pass a 100MB string into `CheckAlignment` and verify the internal LLM call only receives the first N tokens/bytes.

Implementing these test bounds will ensure the Northstar Guardian remains a robust, fail-safe mechanism against agentic drift, rather than a fragile bottleneck that collapses under unpredicted string formats or massive monorepo payloads.
## 9. Concrete Implementation Blueprints for `internal/northstar/guardian_test.go`

To provide immediate actionable value for the engineering team, below are the specific, copy-paste-ready implementations of the required test boundaries. Adding these will resolve the Gaps identified in Sections 2 through 5.

### Implementation: JSON Hallucination Test
This test ensures that the system handles LLM format drift.
```go
func TestGuardian_ParseAlignmentResponse_JSONHallucination(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	guardian := NewGuardian(store, DefaultGuardianConfig())

	// Simulated output from a tool-tuned model ignoring plain-text instructions
	jsonResponse := `{
		"SCORE": 0.35,
		"RESULT": "failed",
		"EXPLANATION": "Critical architectural drift",
		"SUGGESTIONS": ["Revert code", "Consult Vision"]
	}`

	check := &AlignmentCheck{}
	guardian.parseAlignmentResponse(jsonResponse, check)

	// If the parser fails to regex/unmarshal JSON, it currently defaults to 0.7/Warning
	// This test asserts the parser should correctly identify the 0.35 score.
	if check.Score != 0.35 {
		t.Errorf("expected JSON extraction of score 0.35, got %f", check.Score)
	}
	if check.Result != AlignmentFailed {
		t.Errorf("expected JSON extraction of result 'failed', got %s", check.Result)
	}
}
```

### Implementation: Scientific Notation & Percentage Test
This test ensures numerical formats from generative models are handled robustly.
```go
func TestGuardian_ParseAlignmentResponse_TypeCoercions(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	guardian := NewGuardian(store, DefaultGuardianConfig())

	cases := []struct {
		name       string
		response   string
		wantScore  float64
	}{
		{"Scientific", "SCORE: 9.5e-1\nRESULT: passed", 0.95},
		{"Percentage", "SCORE: 95%\nRESULT: passed", 0.95},
		{"Fraction", "SCORE: 0.95/1.0\nRESULT: passed", 0.95},
		{"Conversational", "SCORE: I give it a 0.95.\nRESULT: passed", 0.95},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			check := &AlignmentCheck{}
			guardian.parseAlignmentResponse(tc.response, check)
			if check.Score != tc.wantScore {
				t.Errorf("failed to coerce %s: got %f, want %f", tc.name, check.Score, tc.wantScore)
			}
		})
	}
}
```

### Implementation: Extreme Payload Truncation Test
This test guards against token exhaustion and DoS from unhinged agent actions.
```go
func TestGuardian_CheckAlignment_ExtremeContextLength(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	guardian := NewGuardian(store, DefaultGuardianConfig())
	guardian.Initialize()
	guardian.UpdateVision(&Vision{Mission: "Test"})

	// Create a 50MB string (typical for a massive log dump hallucination)
	massiveContext := strings.Repeat("A", 50*1024*1024)

	mockLLM := &mockLLMClient{
		response: "SCORE: 0.95\nRESULT: passed",
	}
	guardian.SetLLMClient(mockLLM)

	// We expect CheckAlignment to either truncate the context silently or return an error.
	// It should NOT attempt to concatenate and send the 50MB string.
	check, err := guardian.CheckAlignment(context.Background(), TriggerManual, "Massive Log Check", massiveContext)

	if err != nil {
		// Valid behavior: Reject massive payloads
		return
	}

	if check == nil {
		t.Fatal("expected non-nil check or error")
	}

	// We must inspect the prompt actually sent to the mock LLM (requires mockLLM upgrade)
	// to ensure it was truncated to max_context_bytes (e.g., 32KB).
}
```

### Implementation: Memory vs DB Synchronization Race Test
This test proves that `TasksSinceCheck` does not drift from the source of truth database.
```go
func TestGuardian_Concurrent_TaskCompletion_Race(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	config := DefaultGuardianConfig()
	config.PeriodicCheckInterval = 999999 // Prevent check triggering
	guardian := NewGuardian(store, config)
	guardian.Initialize()

	concurrency := 100
	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func(id int) {
			defer wg.Done()
			guardian.OnTaskComplete(context.Background(), fmt.Sprintf("Task %d", id))
		}(i)
	}

	wg.Wait()

	// The Database is the source of truth
	dbState, _ := store.GetState()

	// The Guardian memory state
	memState := guardian.GetState()

	if dbState.TasksSinceCheck != memState.TasksSinceCheck {
		t.Fatalf("CRITICAL RACE: DB has %d tasks, but Guardian memory has %d",
			dbState.TasksSinceCheck, memState.TasksSinceCheck)
	}
	if memState.TasksSinceCheck != 100 {
		t.Fatalf("Expected 100 tasks, got %d", memState.TasksSinceCheck)
	}
}
```

By merging these tests into `internal/northstar/guardian_test.go` and modifying `guardian.go` to make them pass, the alignment subsystem will transition from a prototype to an enterprise-grade safety mechanism capable of handling frontier model hallucinations.
