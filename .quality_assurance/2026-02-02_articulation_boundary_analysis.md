# QA Journal: Deep Boundary Value Analysis of Articulation Subsystem
**Date:** 2026-02-02 10:00 EST
**Author:** Jules (QA Automation Engineer)
**Target:** `internal/articulation/emitter.go` (Piggyback Protocol)
**Version:** 1.2.0 (Thought-First Architecture)
**Review Status:** DRAFT (Expansion Phase)

## 1. Executive Summary

This journal entry documents a Deep Boundary Value Analysis (BVA) and Negative Testing strategy for the `internal/articulation` subsystem. This subsystem is the critical "Transducer" that converts raw LLM text output into structured actions (Mangle atoms, memory operations, tool calls) via the Piggyback Protocol.

**Critical Finding:** The current implementation relies heavily on `json.Unmarshal` and regex extraction (`embeddedJSONPattern`) but lacks rigorous defense against:
1.  **Type Confusion Attacks:** Where an LLM (or adversary) emits incorrect JSON types (e.g., strings instead of arrays), which `json.Unmarshal` will reject, causing a fallback to raw text. This fails-safe but degrades the user experience by showing raw JSON to the user instead of executing actions.
2.  **Resource Exhaustion:** The regex pattern `\{[\s\S]*...\}` is potentially vulnerable to catastrophic backtracking on malicious or extremely long inputs.
3.  **Partial Failures:** No mechanism exists to recover partial data if one field (e.g., `mangle_updates`) is malformed but others are valid.
4.  **Unbounded Growth:** The `ReasoningTrace` field is not subject to any size caps, leading to potential OOM scenarios.

## 2. System Overview & Architecture Fit

The `articulation` package serves as the "mouth" of the agent. In the JIT Clean Loop architecture, it is the final step before the `kernel` receives instructions.

### 2.1 The Piggyback Protocol
The protocol defines a dual-channel communication format:
-   **Surface Channel (`surface_response`)**: What the user sees.
-   **Control Channel (`control_packet`)**: Instructions for the kernel.

The robust parsing of this protocol is the single point of failure for the agent's agency. If parsing fails, the agent becomes a chat-bot, unable to use tools or remember facts.

### 2.2 Component Analysis: ResponseProcessor

The `ResponseProcessor` in `emitter.go` operates in stages:
1.  **Direct JSON**: Attempts to unmarshal the entire string.
2.  **Markdown Wrapped**: Strips ` ```json ` markers.
3.  **Embedded Extraction**: Uses regex to find JSON-like structures inside mixed text.
4.  **Fallback**: Returns the raw string as `Surface` response.

## 3. Deep Code Walkthrough & Audit

### 3.1 `parseJSON` (Line 214)
*   **Mechanism**: Uses `json.Unmarshal` and then `json.NewDecoder` for streaming.
*   **Vulnerability**: `json.Unmarshal` reads the *entire* byte slice into memory. For a 100MB input, this allocates >100MB.
*   **Validation**: It checks for `surface_response` and `control_packet` presence.
*   **Gap**: It does *not* validate the internal structure of `ControlPacket` beyond basic presence. `MangleUpdates` can be nil, `ToolRequests` can be nil.

### 3.2 `extractEmbeddedJSON` (Line 274)
*   **Mechanism**: `regexp.MustCompile` with `[\s\S]*`.
*   **Complexity**: The regex engine in Go (RE2) is linear time $O(n)$, which prevents catastrophic backtracking (exponential time) common in PCRE.
*   **Correction**: My previous fear of catastrophic backtracking was based on PCRE behavior. Go's `regexp` guarantees linear time. **However**, linear time on a 1GB string is still slow and CPU intensive. The memory overhead of finding all matches (`FindAllString`) is significant because it materializes the substrings.

### 3.3 `applyCaps` (Line 192)
*   **Mechanism**: Truncates strings and slices.
*   **Gap**: It explicitly checks `MangleUpdates` and `MemoryOperations`. It totally ignores `ReasoningTrace`, `ToolRequests`, and `KnowledgeRequests`.
*   **Risk**: An attacker (or confused LLM) could emit a `tool_requests` array with 1,000,000 items, causing the loop in the `executor` to spin for a long time before hitting a timeout, or OOMing during unmarshal.

## 4. Boundary Value Analysis (BVA) Vectors

### 4.1 Null / Undefined / Empty
**Vector:** The JSON specification allows `null`. Go's `encoding/json` unmarshals `null` to zero values for pointers/slices, but strict validation might panic or behave unexpectedly if fields are assumed to be non-nil.

*   **Case 4.1.1: Explicit Null Root**
    *   **Input**: `null`
    *   **Expected**: Error or empty envelope.
    *   **Current Code**: `json.Unmarshal` succeeds with empty struct. `envelope.Surface` is empty.
    *   **Risk**: `parseJSON` returns `fmt.Errorf("missing surface_response field")`. Safe.

*   **Case 4.1.2: Explicit Null Control Packet**
    *   **Input**: `{"surface_response": "hi", "control_packet": null}`
    *   **Expected**: Valid surface, empty control.
    *   **Current Code**: Unmarshals successfully. `Control` is zero value.
    *   **Gap**: If code downstream expects `Control` to be initialized (though it is a struct, not a pointer in `PiggybackEnvelope`), it's fine. However, `MangleUpdates` will be `nil`.
    *   **Risk**: Iterating over `nil` slice is safe in Go.

*   **Case 4.1.3: Null in Arrays**
    *   **Input**: `{"...": ..., "mangle_updates": ["a().", null, "b()."]}`
    *   **Expected**: Partial success or clean error.
    *   **Current Code**: `json.Unmarshal` will fail with `json: cannot unmarshal null into Go value of type string`.
    *   **Result**: Fallback to raw text. The user sees the JSON.
    *   **Improvement**: Custom unmarshaller to skip nulls?

*   **Case 4.1.4: Empty Strings in Critical Fields**
    *   **Input**: `{"tool_requests": [{"id": "", "tool_name": ""}]}`
    *   **Risk**: Tool router might panic or execute a "default" tool if empty name is passed.
    *   **Gap**: Validation only checks `IntentClassification`. `ToolRequest` validation is missing in `emitter.go`.

### 4.2 Type Coercion (The "Stringly Typed" LLM Problem)
LLMs often output numbers as strings or single items instead of arrays.

*   **Case 4.2.1: Stringified Confidence**
    *   **Input**: `"confidence": "0.9"` (String instead of Number)
    *   **Expected**: `0.9`
    *   **Current Code**: `json.Unmarshal` fails. Fallback to raw.
    *   **Impact**: Valid intent is lost.
    *   **Fix**: Use `json.Number` or a custom flexible float type.

*   **Case 4.2.2: Single String vs Array**
    *   **Input**: `"mangle_updates": "task_complete(/foo)."` (String instead of Array)
    *   **Expected**: `["task_complete(/foo)."]`
    *   **Current Code**: `json.Unmarshal` fails.
    *   **Impact**: Loss of logic updates. High probability with weaker models.

*   **Case 4.2.3: Object vs Array**
    *   **Input**: `"tool_requests": {"id": "1", ...}` (Object instead of Array of Objects)
    *   **Current Code**: Fails.

### 4.3 User Request Extremes (Size & Complexity)

*   **Case 4.3.1: Recursion Depth**
    *   **Input**: `{"surface_response": "...", "control_packet": {"tool_args": {"a": {"b": {"c": ...}}}}}` (10k deep)
    *   **Risk**: `json.Unmarshal` usually handles deep nesting until stack overflow.
    *   **Impact**: Panic in `ResponseProcessor`.

*   **Case 4.3.2: Massive Reasoning Trace**
    *   **Input**: `reasoning_trace` field is 50MB of text.
    *   **Gap**: `applyCaps` limits `Surface`, `MangleUpdates` (count), `MemoryOps` (count).
    *   **CRITICAL GAP**: There is **NO CAP** on `ReasoningTrace` string length.
    *   **Impact:** OOM (Out of Memory) if multiple shards emit massive traces simultaneously.

*   **Case 4.3.3: Massive Integer**
    *   **Input**: `"confidence": 1e1000`
    *   **Risk**: `float64` overflow to `+Inf`. Downstream logic might break.

### 4.4 State Conflicts & Structural Weirdness

*   **Case 4.4.1: Duplicate Keys**
    *   **Input**: `{"surface_response": "A", "surface_response": "B"}`
    *   **Behavior**: Go's `json` package keeps the *last* occurrence ("B").
    *   **Risk**: Adversarial manipulation. If the LLM writes "I will not delete files" then "I will delete files" in the same JSON object, strict parsing hides the first one.

*   **Case 4.4.2: Conflicting Signals**
    *   **Input**: `intent` says "read_file" but `tool_requests` contains "write_file".
    *   **Behavior**: `emitter.go` parses both. Conflict resolution is delegated to `kernel`.
    *   **Note**: This is acceptable separation of concerns, but `emitter` should perhaps flag it.

### 4.5 Performance & Regex

*   **Case 4.5.1: Catastrophic Backtracking**
    *   **Pattern**: `embeddedJSONPattern = regexp.MustCompile(\`\{[\s\S]*("surface_response"[\s\S]*"control_packet"|"control_packet"[\s\S]*"surface_response")[\s\S]*\}\`)`
    *   **Analysis**: The `[\s\S]*` is a greedy "match anything" including newlines.
    *   **Vulnerability**: If the text is massive (e.g., 10MB log file dump) and contains `{` at the start and `}` at the end, but *almost* matches the middle, the engine might hang.
    *   **Fix**: Use `regexp.QuoteMeta` or limit the scan window? Or better, find the first `{` and last `}` and try to unmarshal, avoiding regex for extraction if possible (which `parseJSON`'s streaming decoder attempts, but `extractEmbeddedJSON` relies on regex).

## 5. Risk Matrix

| ID | Vector | Likelihood | Severity | Impact | Mitigation Status |
|----|--------|------------|----------|--------|-------------------|
| 1  | Null fields | Low | Medium | Partial data loss | Unmitigated |
| 2  | String vs Array | High | High | Action failure (Fallback) | Unmitigated |
| 3  | ReasoningTrace OOM | Low | Critical | Service Crash | **MISSING** |
| 4  | Regex DOS | Medium | High | CPU spike / Hang | Partial (Fallback) |
| 5  | Malformed UTF-8 | Low | Low | Parse error | Handled by Go |
| 6  | ToolRequest Spam | Low | High | DOS / Resource exhaustion | **MISSING** |
| 7  | Duplicate Keys | Low | Medium | Logic confusion | Unmitigated |

## 6. Detailed Reproduction Steps for "ReasoningTrace OOM"

1.  **Context**: The `applyCaps` function (line 197) limits `Surface` length.
2.  **Context**: It limits `MangleUpdates` *count* and `MemoryOperations` *count*.
3.  **Observation**: `ControlPacket` struct (line 21) has `ReasoningTrace string`.
4.  **Observation**: `applyCaps` does **not** check `len(result.Control.ReasoningTrace)`.
5.  **Test Code**:
    ```go
    func TestReasoningTrace_Unbounded(t *testing.T) {
        // Allocate a 200MB string
        hugeString := strings.Repeat("A", 200*1024*1024)
        json := fmt.Sprintf(`{"surface_response":"hi", "control_packet": {"reasoning_trace": "%s"}}`, hugeString)

        rp := NewResponseProcessor()

        // This process call will attempt to duplicate the string in memory during unmarshal
        res, err := rp.Process(json)

        if err != nil {
            t.Fatalf("Process failed: %v", err)
        }

        // Check if the trace was truncated
        if len(res.Control.ReasoningTrace) > 100000 {
            t.Fatalf("ReasoningTrace was not capped! Length: %d", len(res.Control.ReasoningTrace))
        }
    }
    ```

## 7. Detailed Reproduction for Type Coercion

1.  **Test Code**:
    ```go
    func TestFlexibleParsing_MangleUpdates(t *testing.T) {
        // Mangle updates is a single string instead of an array
        json := `{"surface_response":"hi", "control_packet": {"mangle_updates": "single_atom()."}}`

        rp := NewResponseProcessor()
        res, err := rp.Process(json)

        if err != nil {
            t.Fatalf("Unexpected error: %v", err)
        }

        // Current implementation will fallback to raw text because unmarshal fails
        if res.ParseMethod != "json" {
             t.Logf("Correctly identified that current impl fails to parse single string mangle_update. Method: %s", res.ParseMethod)
        } else {
             t.Errorf("Unexpectedly succeeded? Did someone fix it?")
        }
    }
    ```

## 8. Proposed Test Improvements

The `emitter_test.go` file needs to be augmented with these negative test cases to ensure robustness.

1.  **TestMalformedJSONValues**: Inject `null`, `1.0` (for strings), `true` (for strings).
2.  **TestBoundarySizes**: Inject 100MB strings into `reasoning_trace`, `tool_args`, `purpose`.
3.  **TestRegexPerformance**: Benchmark `extractEmbeddedJSON` with 1MB of noise.
4.  **TestDuplicateKeys**: Verify which key takes precedence.
5.  **TestToolRequestSpam**: Inject 10,000 tool requests.

## 9. Mitigation Strategies (Code Snippets)

### 9.1 Flexible Array Unmarshalling
Use a custom type for `MangleUpdates` that implements `json.Unmarshaler`. This is a robust pattern for LLM integration.

```go
// Internal helper type
type FlexibleStringSlice []string

func (s *FlexibleStringSlice) UnmarshalJSON(data []byte) error {
    // 1. Try array
    var arr []string
    if err := json.Unmarshal(data, &arr); err == nil {
        *s = arr
        return nil
    }

    // 2. Try single string
    var str string
    if err := json.Unmarshal(data, &str); err == nil {
        *s = []string{str}
        return nil
    }

    // 3. Try null
    if string(data) == "null" {
        *s = []string{}
        return nil
    }

    return fmt.Errorf("invalid format for string slice")
}

// Update ControlPacket struct
type ControlPacket struct {
    MangleUpdates FlexibleStringSlice `json:"mangle_updates"`
    // ...
}
```

### 9.2 Capping Reasoning Trace & Tool Requests
Modify `applyCaps` in `emitter.go` to be comprehensive.

```go
func (rp *ResponseProcessor) applyCaps(result *ArticulationResult) {
    if result == nil {
        return
    }

    // 1. Surface Cap
    if rp.MaxSurfaceLength > 0 && len(result.Surface) > rp.MaxSurfaceLength {
        result.Surface = result.Surface[:rp.MaxSurfaceLength] + "\n\n[TRUNCATED]"
        result.Warnings = append(result.Warnings,
            fmt.Sprintf("Surface response truncated to %d chars", rp.MaxSurfaceLength))
    }

    // 2. Mangle Updates Cap
    const maxMangleUpdates = 2000
    if len(result.Control.MangleUpdates) > maxMangleUpdates {
        result.Control.MangleUpdates = result.Control.MangleUpdates[:maxMangleUpdates]
        result.Warnings = append(result.Warnings,
            fmt.Sprintf("Mangle updates truncated to %d atoms", maxMangleUpdates))
    }

    // 3. Memory Ops Cap
    const maxMemoryOps = 500
    if len(result.Control.MemoryOperations) > maxMemoryOps {
        result.Control.MemoryOperations = result.Control.MemoryOperations[:maxMemoryOps]
        result.Warnings = append(result.Warnings,
            fmt.Sprintf("Memory operations truncated to %d items", maxMemoryOps))
    }

    // 4. Reasoning Trace Cap (NEW)
    const maxTraceLength = 50000 // 50KB
    if len(result.Control.ReasoningTrace) > maxTraceLength {
        result.Control.ReasoningTrace = result.Control.ReasoningTrace[:maxTraceLength] + "[TRUNCATED]"
        result.Warnings = append(result.Warnings, "Reasoning trace truncated")
    }

    // 5. Tool Requests Cap (NEW)
    const maxToolRequests = 20
    if len(result.Control.ToolRequests) > maxToolRequests {
        result.Control.ToolRequests = result.Control.ToolRequests[:maxToolRequests]
        result.Warnings = append(result.Warnings, "Tool requests truncated")
    }
}
```

### 9.3 Robust Float Parsing
Use `json.Number` to handle "0.9" vs 0.9.

```go
type IntentClassification struct {
    Confidence json.Number `json:"confidence"`
    // ...
}

func (ic *IntentClassification) GetConfidence() float64 {
    f, err := ic.Confidence.Float64()
    if err != nil {
        return 0.0 // Default to low confidence
    }
    return f
}
```

### 9.4 Defensive Regex
Avoid `[\s\S]*` if possible. Use a streaming parser that looks for `{` and counts braces to find the matching `}`.

```go
func findJSONBlock(s string) (string, error) {
    // Simple state machine to find balanced braces
    start := strings.Index(s, "{")
    if start == -1 {
        return "", fmt.Errorf("no start brace")
    }

    balance := 0
    for i := start; i < len(s); i++ {
        if s[i] == '{' {
            balance++
        } else if s[i] == '}' {
            balance--
            if balance == 0 {
                return s[start : i+1], nil
            }
        }
    }
    return "", fmt.Errorf("unbalanced braces")
}
```
*Note: This naive implementation fails on braces inside strings, but it's $O(n)$ and memory efficient.*

## 10. Mangle Logic Implications

The interface between `articulation` and `kernel` is sensitive to format.

### 10.1 Atom Injection Vulnerability
Mangle atoms are strings starting with `/` or lowercase. If the `MangleUpdates` parser allows arbitrary strings, an LLM could inject:
`task_status(/id, "some text with ) inside to break parser").`

If the kernel uses string concatenation to build the program, this is a **SQL Injection** equivalent.
**Mitigation:** The `kernel` MUST parse these strings using `mangle.Parse` BEFORE adding them to the store. The `articulation` layer cannot guarantee validity, only structure.

### 10.2 Atom vs String Dissonance
The journal's introductory "Skill Review" highlighted the danger of `"active"` vs `/active`.
- If LLM emits: `["status(/user, \"active\")"]`
- But Rules expect: `status(U, /active)`
- Result: **Logic Failure (Silent)**.

**Mitigation:** The `articulation` layer could implement a "heuristic linter" that warns if it sees quoted strings where atoms are common (e.g., as second argument to `status`).

## 11. Fuzzing Corpus (Nasty JSONs)

To properly test `ResponseProcessor`, the following corpus should be used:

```json
[
  "null",
  "{}",
  "{\"control_packet\": null}",
  "{\"control_packet\": []}",
  "{\"control_packet\": \"string\"}",
  "{\"surface_response\": null}",
  "{\"mangle_updates\": [null]}",
  "{\"mangle_updates\": [123]}",
  "{\"mangle_updates\": {}}",
  "{\"intent_classification\": {\"confidence\": \"high\"}}",
  "{\"intent_classification\": {\"confidence\": true}}",
  "{\"tool_requests\": [{\"id\": 1}]}",
  "{\"tool_requests\": [{\"tool_args\": \"args\"}]}",
  "{\"reasoning_trace\": 12345}",
  "{\"unknown_field\": [recursive...]}",
  "{\"surface_response\": \"\\u0000\"}",
  "{\"key\": \"\\\"escaped\"}",
  "{\"duplicate\": 1, \"duplicate\": 2}",
  "{\"depth\": {\"a\": {\"b\": {\"c\": {}}}}}"
]
```

## 12. Conclusion and Recommendations

The `internal/articulation` subsystem is functional for "Happy Path" scenarios but fragile against LLM variability (type coercion) and potentially vulnerable to resource exhaustion (ReasoningTrace OOM, Regex DOS).

**Recommendation 1:** Implement the `FlexibleStringSlice` type immediately. This will reduce the "Fallback" rate significantly, as LLMs often hallucinate single strings for single-item arrays.

**Recommendation 2:** Implement the `ReasoningTrace` and `ToolRequests` caps in `applyCaps`. This is a critical safety fix to prevent memory exhaustion attacks or accidents.

**Recommendation 3:** Adopt a rigorous fuzzing strategy for `ResponseProcessor`. The existing fuzzer `FuzzResponseProcessor_Process` is good, but it should be run continuously in CI with a corpus of known "bad" JSONs.

**Recommendation 4:** Add structured logging for all truncation events. Currently, they are just warnings in the result object, which might be ignored by the caller. They should be logged to `logging.CategoryArticulation`.

This analysis concludes that while the "Clean Loop" architecture is sound, the "Mouth" (Articulation) needs strictly typed dental work to avoid choking on unexpected inputs.

---
*Generated by Jules, QA Automation Engineer, on 2026-02-02. Verified against v1.2.0.*
