---
remediated: false
---
# codeNERD QA Journal - PromptEvolver Boundary Value Analysis

**Date:** 2026-03-28 10:00:00 EST
**Engineer:** Jules, QA Automation Engineer
**Subsystem:** Autopoiesis / PromptEvolver (`internal/autopoiesis/prompt_evolution/evolver.go`, `atom_generator.go`, `judge.go`, `prompt_evolution_test.go`)

---

## 1. Executive Summary

This journal entry details a comprehensive boundary value analysis and negative testing review of the `PromptEvolver` subsystem in the codeNERD framework. This subsystem plays a critical role in the autopoiesis (self-improving) loop by recording task execution histories, employing an LLM-as-Judge to score outcomes, analyzing failures to extract improvement rules, and auto-generating new prompt "atoms" via the `AtomGenerator` that guide future behavior.

The core observation driving this analysis is that the `PromptEvolver` manages an enormous amount of concurrent state (from parallel `ExecutionRecords`) and directly orchestrates synchronous, high-latency LLM network requests. A failure in this subsystem—either through a state conflict lockup, an out-of-bounds input payload, or a schema parsing error from an LLM hallucination—has catastrophic cascading effects on the agent's ability to operate and learn.

We evaluate the system along four vectors:
1. **Null/Undefined/Empty**
2. **Type Coercion**
3. **User Request Extremes**
4. **State Conflicts**

For each vector, we explicitly identify edge cases that are **currently missing** from the test suite, assess the performance impact, and propose architectural recommendations.

---

## 2. Deep Dive: Null/Undefined/Empty Inputs

### Edge Case 2.1: Nil Elements in Slices (`GenerateFromFailures`)
In `internal/autopoiesis/prompt_evolution/atom_generator.go`, the method `buildMetaPrompt` iterates over the `failures` slice (type `[]*JudgeVerdict`). If the upstream system passes a slice containing a mix of valid pointers and `nil` pointers (e.g., due to a failed or cancelled evaluation step that blindly appended to a result slice), the `buildMetaPrompt` method will attempt to dereference `f.Category` and `f.Explanation` without checking for `nil`.

**System Performance & Handling:**
The system is currently NOT robust against this. A nil pointer dereference inside the background evolution loop (`RunEvolutionCycle`) will panic the goroutine. Because `RunEvolutionCycle` is called under a locked mutex (`pe.mu.Lock()`), the panic will crash the entire subsystem, and if `RunEvolutionCycle` was spawned directly, it will crash the application. The `TestPromptEvolver_EmptyFailures` test passes an empty slice, but no test passes a slice *containing* a nil pointer.

**Proposed Test:**
Inject a slice with a nil element into `GenerateFromFailures` and verify it gracefully ignores it.

```go
// Example test code to add:
func TestAtomGenerator_NilElements(t *testing.T) {
	ag := NewAtomGenerator(&mockLLMClient{}, nil)
	failures := []*JudgeVerdict{
		nil,
		{Verdict: "FAIL", Explanation: "real failure", Category: CategoryLogicError},
		nil,
	}
	// Should not panic
	atoms, err := ag.GenerateFromFailures(context.Background(), failures, "/coder", string(ProblemDebugging))
	if err != nil {
		t.Fatalf("GenerateFromFailures failed: %v", err)
	}
}
```

### Edge Case 2.2: Empty Configuration Values & Zero Defaults
The `validateEvolverConfig` method prevents configurations like `MinFailuresForEvolution = 0`, but what about missing values in the LLM response during atom generation?
The LLM generates YAML. If the LLM generates a valid YAML block but leaves the `content:` field empty or entirely missing, the `convertToPromptAtom` method has a check: `if def.Content == "" { return nil }`.
However, the caller `parseGeneratedAtoms` loops over `atomDefs`, maps them, and ignores `nil` atoms. If *all* atoms map to `nil`, it returns an empty slice. This is handled safely, but there are no tests ensuring an empty YAML definition doesn't cause a panic or log warnings properly.

```go
// Example test code to add:
func TestAtomGenerator_EmptyYAMLContent(t *testing.T) {
	// ... Test that a YAML block without a content field is gracefully ignored.
	ag := NewAtomGenerator(&emptyYAMLLLMClient{}, nil)
	// Execute and assert no panic, returns empty atoms
}
```

### Edge Case 2.3: Nil ExecutionRecords Passed to FeedbackCollector
The `FeedbackCollector.Record` method handles large payloads, but does it panic on empty/nil `ExecutionRecord.PromptManifest.Selected` arrays?
The `ExecutionRecord` contains a `PromptManifest` pointer. If an execution failed so early that `PromptManifest` was never initialized (nil), does storing it panic the SQLite layer, or does JSON marshaling safely convert it to SQL `NULL`? We need explicit tests simulating extremely early aborts where half the `ExecutionRecord` is zero-valued.

```go
// Example test code to add:
func TestFeedbackCollector_ZeroValueExecution(t *testing.T) {
	// ... Test storing an execution record that has zero values for almost all fields
}
```

### Edge Case 2.4: Empty LLM Response Handling in Generation
The `CompleteWithSystem` method could return an empty string (`""`) and `nil` error if the LLM API drops the connection unexpectedly or hits a filter. The `extractYAMLBlock` might return an empty string. The current logic safely handles this by returning `fmt.Errorf("no YAML content found in response")`. However, there is no test proving that an empty LLM string response correctly triggers the specific error.

```go
// Example test code to add:
func TestAtomGenerator_EmptyLLMResponse(t *testing.T) {
	// Create an LLM client that returns "" and verify error bubbling.
}
```

### Edge Case 2.5: Zero Byte Input Files to Extractor
The extraction methods that parse generated YAML content could be provided with zero byte inputs if a file read failed or returned empty content. Currently, `extractYAMLBlock` and `extractJSONObject` will gracefully return empty strings. We should assert this behavior.

```go
// Example test code to add:
func TestExtractYAMLBlock_ZeroByte(t *testing.T) {
	// Test zero byte string on extractYAMLBlock
}
```

### Edge Case 2.6: Null Pointer dereference on Strategy Store Initialization
If the configuration for `EnableStrategies` is set to true, but `NewStrategyStore(nerdDir)` fails, it returns an error. The `PromptEvolver` returns that error. But what if it succeeded, and later the `StrategyStore` pointer becomes nil somehow or isn't mocked during a test?

```go
// Example test code to add:
func TestPromptEvolver_NilStrategyStore(t *testing.T) {
    // Assert behavior when store initialization leaves a nil pointer
}
```

### Edge Case 2.7: Missing Output Directory Initialization
If the provided `nerdDir` string is missing (`""`), does `NewPromptEvolver` fail cleanly or attempt to write to the current working directory? The path resolution might resolve to `prompts/evolved/` locally, bypassing the expected `.nerd/` boundary.

```go
// Example test code to add:
func TestPromptEvolver_EmptyDirectory(t *testing.T) {
    // Assert behavior when nerDir is empty
}
```

### Edge Case 2.8: Uninitialized `evolvedAtoms` Map
If the Evolver struct is created directly bypassing `NewPromptEvolver`, the `evolvedAtoms` map will be nil. Does `RecordAtomUsage` panic? This is typical for struct initialization but should be asserted in a negative test.

---

## 3. Deep Dive: Type Coercion Vulnerabilities

### Edge Case 3.1: YAML Type Coercion from LLM Hallucinations
The `AtomGenerator` relies on `yaml.Unmarshal([]byte(yamlContent), &atomDefs)` where `atomDefinition` expects strictly typed fields:
```go
type atomDefinition struct {
	Priority    int      `yaml:"priority"`
	IsMandatory bool     `yaml:"is_mandatory"`
}
```
If the LLM generates a string for Priority (e.g., `priority: "High"`) instead of an integer, or a string for IsMandatory (e.g., `is_mandatory: "Yes"`), the strict `gopkg.in/yaml.v3` unmarshaler will return an error (`*yaml.TypeError`).
Because `parseGeneratedAtoms` checks `if err := yaml.Unmarshal(...); err != nil`, the ENTIRE batch of atoms is discarded if even one field in one atom has a type mismatch.

**System Performance & Handling:**
The system is safe from crashing, but the performance is poor: an expensive LLM call is completely wasted due to a minor formatting hallucination. A more performant and robust system would use a loose unmarshaling struct (e.g., `interface{}`) and gracefully try to coerce "High" -> 80, or "Yes" -> true, salvaging the LLM's valuable output.
Currently, this type coercion failure vector is completely untested.

```go
// Example test code to add:
func TestAtomGenerator_TypeCoercion(t *testing.T) {
	// Need a mock that returns YAML like:
	// priority: "High"
	// is_mandatory: "True" (string)
	// And verify it recovers gracefully instead of discarding the atom.
}
```

### Edge Case 3.2: LLM JSON Response Parsing in Judge
The `TaskJudge.parseVerdict` expects:
```json
{
  "confidence": 0.85
}
```
If the LLM returns `"confidence": "85%"` or `"confidence": "High"`, `json.Unmarshal` will fail. Again, an expensive LLM call is wasted. The test `TestTaskJudge_EvaluateBatchPreservesOrdering` uses a dummy client, but we need a test that explicitly injects mismatched JSON types to verify the system's resilience and error reporting.

```go
// Example test code to add:
func TestTaskJudge_TypeCoercion(t *testing.T) {
	// Need a mock that returns JSON with a string instead of a float64 for confidence
	// And verify the judge recovers instead of completely failing the evaluation.
}
```

### Edge Case 3.3: Invalid Category Strings to `mapToErrorCategory`
The `mapToErrorCategory` function relies on `strings.Contains` for fallback cases (e.g., if it contains "LOGIC", return `CategoryLogicError`).
What if the LLM hallucinates an empty string, or an extremely long string with null characters? Or what if it hallucinates `LOGICSYNTAX`? The string handling is somewhat robust but edge cases are not explicitly covered in tests.

```go
// Example test code to add:
func TestTaskJudge_InvalidCategory(t *testing.T) {
    // Assert how mapToErrorCategory handles extreme invalid inputs
    // e.g. "LOGICSYNTAX", "\x00", ""
}
```

### Edge Case 3.4: Float to Int Coercion in YAML Priority
The `yaml` package can coerce a float like `50.0` into an integer field `Priority` but if it receives `50.5` it will fail. Mangle Atoms use numeric ranking, and an LLM might attempt to assign `priority: 88.5`. We need tests ensuring `yaml.Unmarshal` handles this without discarding the whole atom payload.

```go
// Example test code to add:
func TestAtomGenerator_FloatPriority(t *testing.T) {
    // Test parsing a YAML payload containing priority: 88.5
}
```

### Edge Case 3.5: JSON Boolean Coercion
If the LLM returns `"success": "true"` instead of `"success": true` in a JSON block, does `json.Unmarshal` silently fail? Yes, the Go standard library requires exact type mapping unless a custom unmarshaler is used. This is missing from testing.

```go
// Example test code to add:
func TestTaskJudge_JSONBooleanCoercion(t *testing.T) {
    // Verify JSON parsing failure recovery
}
```

### Edge Case 3.6: String coercion from integer lists
What if `languages: [123]` is returned instead of strings? YAML handles this sometimes but strong type assertion might fail.

---

## 4. Deep Dive: User Request Extremes

### Edge Case 4.1: Massive Context Window Exhaustion in Meta-Prompt
The `buildMetaPrompt` method groups failures by category and limits them:
```go
for i, f := range categoryFailures {
    if i >= 3 {
        sb.WriteString(fmt.Sprintf("... and %d more\n", len(categoryFailures)-3))
        break
    }
    // ... string builder ...
}
```
This protects against a single category having thousands of failures. However, what if there are 100 *different* categories?
`mapToErrorCategory` maps strings to an `ErrorCategory` enum, but if the LLM hallucinates categories, or if we define 50 unique sub-categories in the future, the map `byCategory` could contain 50 entries. 50 entries * 3 failures each = 150 failure explanations injected into the prompt.
For a complex codebase, a single failure explanation might include a large stack trace or 1,000+ characters of output. Injecting 150 of these into `buildMetaPrompt` will silently build a string that is 150,000+ characters long. When sent to the LLM via `CompleteWithSystem`, it will likely hit a `400 Bad Request: Token limit exceeded`.

**System Performance & Handling:**
The system handles the HTTP 400 error safely (returns err, logs it, skips evolution). However, the system is fundamentally stuck. The next time `RunEvolutionCycle` runs, it will select the exact same 150 failures, build the same massive prompt, and hit the same 400 error. The `PromptEvolver` is permanently paralyzed for that Shard/ProblemType combination.
We need negative tests that feed an extremely high number of distinct categories and massive explanation strings to verify truncation logic (which currently does not exist for the total prompt size).

```go
// Example test code to add:
func TestAtomGenerator_MassiveFailureCategories(t *testing.T) {
	// Generate 100 unique categories with 10 failures each.
	// Ensure the resulting prompt length is strictly capped.
}
```

### Edge Case 4.2: Extremely Long Evolved Atom Content
What if the LLM generates an atom whose `content` is 5MB of text?
The `convertToPromptAtom` function accepts the content and computes the `TokenCount`. Later, `storeEvolvedAtom` writes this to disk. When the system subsequently loads the atom and passes it to the `TokenBudgetManager` (not tested here, but downstream), it will consume the entire budget.
While this isn't a crash in the `PromptEvolver`, it's an extreme user edge case. The Evolver should probably reject newly generated atoms that exceed a certain size limit (e.g., 4000 tokens) to protect the downstream system.

```go
// Example test code to add:
func TestAtomGenerator_ExtremelyLongContent(t *testing.T) {
	// Generate an atom with 5MB of text.
	// Ensure it is safely rejected to prevent downstream OOM or budget exhaustion.
}
```

### Edge Case 4.3: Deeply Nested JSON/YAML responses
An adversarial LLM output could return deeply nested YAML:
```yaml
a:
  b:
    c: ...
```
If the nesting exceeds the YAML parser's recursion depth, it could cause a stack overflow. We need to ensure that the regex extraction `extractYAMLBlock` doesn't hang on pathological backtracking (ReDoS) and that the parser handles depth safely. Currently `extractYAMLBlock` uses fast string operations (`strings.Index`), avoiding ReDoS, but the YAML parser's behavior under nested extreme depth is not tested.

```go
// Example test code to add:
func TestAtomGenerator_DeeplyNestedYAML(t *testing.T) {
	// Feed deeply nested YAML string to parseGeneratedAtoms
	// Ensure it does not cause a stack overflow or OOM
}
```

### Edge Case 4.4: Maximum Atoms Per Evolution Threshold Extreme
If `MaxAtomsPerEvolution` is configured to `math.MaxInt32`, the slicing `atoms = atoms[:pe.config.MaxAtomsPerEvolution]` will panic with an out-of-bounds index error because `len(atoms)` might be smaller than `MaxAtomsPerEvolution`. Wait, the code reads:
```go
if len(atoms) > pe.config.MaxAtomsPerEvolution {
    atoms = atoms[:pe.config.MaxAtomsPerEvolution]
}
```
So it is actually safe. However, there's no test to assert this boundary condition holds explicitly.

```go
// Example test code to add:
func TestPromptEvolver_MaxAtomsPerEvolutionExtreme(t *testing.T) {
    // Assert safe slicing behavior when config limit is much larger than generated atoms
}
```

### Edge Case 4.5: Exhaustive Shard Types List
What happens if the LLM generates an atom that applies to 1,000 distinct shard types? `shardTypes` slice parsing would allocate a massive string array, causing unnecessary memory allocation overhead. It's technically safe but indicates a user request extreme parameter. We should have a hard ceiling on parsed slice allocations for these meta fields.

```go
// Example test code to add:
func TestAtomGenerator_MassiveShardTypes(t *testing.T) {
	// Test generation where the LLM defines 10,000 shard_types in the YAML.
}
```

---

## 5. Deep Dive: State Conflicts and Concurrency

### Edge Case 5.1: Mutex Lock Held During Synchronous LLM Calls
This is the most critical architectural vulnerability identified in this analysis.
```go
func (pe *PromptEvolver) RunEvolutionCycle(ctx context.Context) (*EvolutionResult, error) {
	pe.mu.Lock()
	defer pe.mu.Unlock()
    // ...
    // Calls pe.judge.EvaluateBatch (Makes LLM Network Calls)
    // ...
    // Calls pe.atomGenerator.GenerateFromFailures (Makes LLM Network Call)
}
```
The `RunEvolutionCycle` method acquires an exclusive write lock (`pe.mu.Lock()`) on the `PromptEvolver` struct and holds it for the entire duration of the evolution cycle.
An evolution cycle includes network requests to `gemini-3-pro` for judging unevaluated executions, and another network request for generating atoms. This process can take anywhere from 5 seconds to 60+ seconds depending on API latency and batch size.

During this 60-second window, **no other goroutine can interact with the PromptEvolver**.
- `RecordExecution()` calls `pe.mu.Lock()`. It will block.
- `PromoteAtom()` calls `pe.mu.Lock()`. It will block.
- `RejectAtom()` calls `pe.mu.Lock()`. It will block.
- `RecordAtomUsage()` calls `pe.mu.Lock()`. It will block.

**System Performance & Handling:**
In a high-concurrency environment where codeNERD is executing dozens of subagents simultaneously (each trying to call `RecordExecution` upon completion), the entire execution pipeline will stall waiting on the `PromptEvolver`'s mutex. The system is NOT performant enough to handle this state conflict.
This is a classic "Lock held during I/O" anti-pattern. The lock should only protect the in-memory maps (`evolvedAtoms`) and state counters, not the synchronous external network calls. The test suite has a `TestPromptEvolver_ConcurrentAccess` test, but it uses a `mockLLMClient` that returns instantly! A test that introduces a 5-second sleep in the mock LLM will immediately reveal the starvation.

```go
// Example mock LLM that reveals the starvation:
type sleepingLLMClient struct{}

func (m *sleepingLLMClient) CompleteWithSystem(ctx context.Context, system, user string) (string, error) {
	time.Sleep(5 * time.Second)
	return "mock response", nil
}

// Example test code to add:
func TestPromptEvolver_LockStarvation(t *testing.T) {
	// Simulate RunEvolutionCycle with the sleepingLLMClient.
	// Try to RecordExecution concurrently and measure the delay.
	// The delay will be 5+ seconds, confirming the lock starvation.
}
```

### Edge Case 5.2: Concurrent File Access during Promotion/Rejection
`PromoteAtom` and `RejectAtom` use `os.Rename(srcPath, dstPath)` to move YAML files between directories.
If an administrator or another subsystem calls `PromoteAtom("id_1")` and `RejectAtom("id_1")` concurrently, both functions acquire the mutex at different times.
- Goroutine 1 gets the lock, renames the file from `pending/id_1` to `promoted/id_1`, and updates `ga.PromotedAt`. It releases the lock.
- Goroutine 2 gets the lock, attempts to rename from `pending/id_1` to `rejected/id_1`. But the file is no longer in `pending/id_1`!
`os.Rename` will return a `no such file or directory` error. The `RejectAtom` method catches this and returns the error, which is safe.
However, what if `RejectAtom` runs first? It moves the file to `rejected` and *deletes* the atom from the `pe.evolvedAtoms` map.
Then `PromoteAtom` runs. It attempts to look up `atomID` in the map, fails, and returns an error. This is also safe.
BUT, what if `RecordAtomUsage` runs concurrently while `RejectAtom` is executing?
`RecordAtomUsage` gets the atom from the map, increments `ga.UsageCount`, and calls `storeEvolvedAtom`.
If `RejectAtom` just moved the file to `rejected` and deleted the map entry, and then `RecordAtomUsage` somehow holds a reference to `ga`... wait, `RecordAtomUsage` locks the mutex. So they can't overlap. The locking is actually safe here, *except* for the massive starvation issue mentioned in 5.1.

```go
// Example test code to add:
func TestPromptEvolver_ConcurrentPromoteReject(t *testing.T) {
	// Spawn multiple goroutines that concurrently call PromoteAtom and RejectAtom on the same atomID.
	// Verify that the final state is consistent and no panics occur.
}
```

### Edge Case 5.3: Duplicate Atom ID Collision
If the LLM generates multiple atoms with the exact same `id` across different evaluation cycles, the `storeEvolvedAtom` method will overwrite the file.
`pe.evolvedAtoms[ga.Atom.ID] = ga` will simply overwrite the existing atom in the map. The usage counters (`UsageCount`, `SuccessCount`) for the previous version of that atom will be completely lost! We need tests verifying that ID collisions are either rejected or gracefully merged.

```go
// Example test code to add:
func TestPromptEvolver_AtomIDCollision(t *testing.T) {
    // Generate an atom with ID "duplicate", record usage on it,
    // then generate a new atom with ID "duplicate" and verify usage count behavior.
}
```

---

## 6. Performance Summary and Recommendations

### Performance Assessment
The `PromptEvolver` subsystem exhibits high algorithmic efficiency for in-memory operations (O(1) map lookups, fast YAML serialization). However, its **concurrent performance is highly degraded** by holding a global structure lock during blocking I/O (LLM API calls). The use of the file system as the primary persistence mechanism for atoms (using `os.Rename` and `os.WriteFile`) is acceptable for low-to-medium throughput, but at scale, it could become a bottleneck on restricted filesystems.

### Architectural Recommendations
1. **Finer-Grained Locking:** Refactor `RunEvolutionCycle`. Gather the necessary state while holding a read lock, release the lock, perform the LLM API calls, and then re-acquire the write lock to apply the resulting atoms to the map and filesystem.
2. **Defensive Slicing:** Add nil-checks when iterating over `[]*JudgeVerdict`.
3. **Resilient Parsing:** Implement a custom YAML unmarshaler that uses `map[string]interface{}` internally to extract fields, allowing for "soft" type coercion (e.g., converting the string "75" to the integer 75) to salvage LLM responses.
4. **Context Window Protection:** Add a hard cap on the total character length of the meta-prompt. If the grouped failure strings exceed ~60,000 characters, truncate the oldest or least confident failures to ensure the prompt always executes successfully.
5. **Worker Pools for Evaluation:** To avoid overwhelming the `TaskJudge` batch evaluator, implement a worker pool or strict semaphore that correctly respects the `LLM_RATE_LIMIT` environment variable across all concurrent evolver loops.

---

## 7. Action Plan

Based on this analysis, explicit `// TODO: TEST_GAP:` markers will be injected into `internal/autopoiesis/prompt_evolution/prompt_evolution_test.go` to flag these exact missing coverage vectors for the engineering team.

*End of Journal Entry.*
