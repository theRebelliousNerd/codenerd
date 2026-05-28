---

remediated: true
remediated_date: 2026-05-28
subsystem: prompt
---
# Prompt Atoms Boundary Value & Negative Testing Analysis

## Introduction
Date: 2026-05-18
Time: 04:13:04 AM EST
System under analysis: `internal/prompt/atoms.go` and corresponding test suite `internal/prompt/atoms_test.go`
Scope: Boundary Value Analysis, Negative Testing, Extreme User Inputs, Type Coercion, State Conflicts, Performance Considerations

This journal entry serves as an in-depth analysis of the Prompt Atom generation, matching, hashing, and cloning logic within the `prompt` package of codeNERD. By shifting the focus away from "happy path" workflows and instead zeroing in on the system's resilience across boundary conditions, we aim to uncover blind spots and recommend targeted improvements.

---

## Part 1: Detailed Gap Analysis Vectors

### Vector 1: Null / Undefined / Empty Data (The "Void" Path)

1. **Empty Strings vs Missing Values in `MatchesContext`:**
   - *Observation:* The matching logic heavily relies on `matchSelector` which checks `value == ""` or slice length zero.
   - *Gap:* The current tests confirm that passing an empty string matches an empty selector list. However, we do not rigorously test what happens if `cc *CompilationContext` contains explicitly populated slices where one of the items is the empty string `""` or contains strings consisting entirely of whitespace.
   - *Remediation Strategy:* Introduce explicit tests that instantiate `CompilationContext` fields with `[]string{""}`, `[]string{" "}`, `[]string{"\x00"}`. Verify if Mangle engine queries correctly deal with zero-width or empty strings since these ultimately serialize to `prompt_atom` and `atom_selector` queries in the kernel.

2. **Nil Struct Pointers vs Zero Structs:**
   - *Observation:* `tt.context` being `nil` allows for an immediate exit in some conditions, but `NewPromptAtom` doesn't handle passing `nil` slices vs `[]string{}` well across all operations explicitly in tests.
   - *Gap:* When `Clone()` is called, it performs deep copying logic but does so via helper functions. Nil vs initialized-empty slice propagation could affect serialization or downstream cache hits.
   - *Remediation Strategy:* We must test `atoms.Clone()` output where the input atoms have fields instantiated explicitly to `[]string{}`. `Clone()` must ensure it doesn't inadvertently convert an empty slice into a `nil` slice, which could break JSON marshaling expectations if external downstream systems enforce strict types (`[]` vs `null`).

### Vector 2: Type Coercion and Formatting

1. **Rune Encoding and Malformed UTF-8:**
   - *Observation:* `HashContent` utilizes `sha256.Sum256([]byte(content))`.
   - *Gap:* We are testing string representations but not maliciously formed byte-sequences wrapped in string types. The system assumes prompt atom contents are valid text.
   - *Remediation Strategy:* Feed `NewPromptAtom` string literals crafted from invalid UTF-8 sequences (e.g. `"\xff\xfe\xfd"`). Check if the `TokenCount` estimator `EstimateTokens` panics or returns deeply skewed negative token counts when `len(content)` math is applied.

2. **Slash Normalization (Type Coercion via String Manipulation):**
   - *Observation:* `matchSelector` normalizes strings by dynamically inspecting `value[0] == '/'`.
   - *Gap:* The logic assumes standard path-like constants (e.g. `/coder`). But what if the input string is just `/`? What if it's `//coder`?
   - *Remediation Strategy:* We need negative tests where `matchSelector` is fed `"/"`, `""`, `"//"`, and `" /active"`. Will the first character check index out of bounds if the string isn't sanitized first, or does the logic bypass?

### Vector 3: User Request Extremes (The "Titan" Workloads)

1. **The 50-Million Line Monorepo Context (`content` Size Boundaries):**
   - *Observation:* `EstimateTokens` performs `(len(content) + 3) / 4`.
   - *Gap:* A prompt atom content theoretically can be injected via the dynamically generated file reading transits. If a user asks codeNERD to ingest a 5GB core dump or log file, and it is temporarily wrapped as a context atom, `HashContent` and `EstimateTokens` might consume excessive memory or even trigger integer overflow in 32-bit systems on `len(content)`.
   - *Remediation Strategy:* Test generating an atom with an extreme length string (e.g. `strings.Repeat("A", math.MaxInt32/2)`). Ensure that hashing this does not crash the test suite but successfully bounds memory usage.

2. **Benchmark Scale-Out (The "Frontier Coding" Limit):**
   - *Observation:* `BenchmarkMatchesContext` tests only tiny atom lists.
   - *Gap:* In a scenario where 10,000 contextual state variables are dynamically loaded as an explicit test (e.g. thousands of `failing_tests` contexts passed via `CompilationContext`), the `O(N*M)` nested loop logic inside `MatchContext` for Frameworks and WorldStates could cause dramatic CPU spin-locking.
   - *Remediation Strategy:* Profile the nested loops within `hasWorldState` and `matchSelector`. Introduce `TestProblemClassifier_HugeInput` equivalents specifically for JIT Prompt Compilation with 100,000+ framework matches to stress-test the `sync.RWMutex` if any are added, or simply the iteration performance.

### Vector 4: State Conflicts (The "Race" Conditions)

1. **Concurrency During Deep Cloning:**
   - *Observation:* The atom fields are pointers in `CompilationContext`.
   - *Gap:* JIT compilation can happen concurrently. If an `Atom` instance is somehow cached globally (like in `EmbeddedCorpus`) and something mistakenly attempts to mutate it without cloning (e.g., stripping slashes or expanding dependencies dynamically), read-write data races will occur.
   - *Remediation Strategy:* Create a test utilizing `errgroup.Group` and 100+ goroutines concurrently calling `MatchesContext` and `Clone` while a background goroutine simulates random context mutations. Detect race conditions with `go test -race`.

2. **Datalog Fact Pollution (The Mangle Boundary):**
   - *Observation:* `ToFact` creates `prompt_atom` facts and selector facts for Mangle.
   - *Gap:* If atoms share IDs but have different states or overlapping conflicts (e.g., an atom states it conflicts with `A`, but another atom dynamically generated re-asserts it doesn't), the Mangle engine fixpoint computation may oscillate or generate an empty set due to logical contradiction.
   - *Remediation Strategy:* Add an adversarial test where contradictory `ConflictsWith` and `DependsOn` arrays are formulated and fed to the `mockKernel`. Verify that `mockKernel` fails gracefully or Mangle explicitly blocks it during the Stratification phase.

---

## Part 2: Concrete Expansion Plan for `internal/prompt/atoms_test.go`

To address the gaps identified above, the following structural enhancements should be applied to the test suite:

### Section A: `TestMatchSelector_BoundaryValues`
```go
func TestMatchSelector_BoundaryValues(t *testing.T) {
	// Tests type coercion and zero-length boundaries for the internal slice matching logic.
	tests := []struct{
		name     string
		selector []string
		value    string
		expected bool
	}{
		{"Double slash value", []string{"/coder"}, "//coder", false}, // Evaluates to '/coder' vs 'coder' or similar string mismatch
		{"Just a slash", []string{"/"}, "/", true},
		{"Whitespace inside selector", []string{" /coder"}, " /coder", true},
		{"Null byte inclusion", []string{"\x00coder"}, "\x00coder", true},
	}
	// ... logic to execute test
}
```

### Section B: `TestPromptAtom_ExtremeLoad`
```go
func TestPromptAtom_ExtremeLoad(t *testing.T) {
    // Simulating a scenario where a sub-agent attempts to wrap an entire massive log file
    // into an ephemeral PromptAtom.
    // Ensure that token count math and SHA256 doesn't OOM or integer overflow.
    hugeSize := 1024 * 1024 * 50 // 50MB string
    hugeStr := strings.Repeat("a", hugeSize)

    // This should run quickly due to Go's optimized SHA256, but confirms memory bounds.
    atom := NewPromptAtom("extreme/load", CategoryContext, hugeStr)

    expectedTokens := (hugeSize + 3) / 4
    if atom.TokenCount != expectedTokens {
        t.Fatalf("Token count failed for massive load: expected %d, got %d", expectedTokens, atom.TokenCount)
    }
}
```

### Section C: `TestPromptAtom_ConcurrencyRace`
```go
func TestPromptAtom_ConcurrencyRace(t *testing.T) {
    // Tests for State Conflicts - verifies that matches against a read-only global corpus atom
    // are strictly thread-safe and no accidental state mutation occurs during 'NormalizeSelectors' or 'MatchesContext'
    atom := &PromptAtom{
        ID: "race/atom",
        Frameworks: []string{"/react", "/bubbletea"},
        WorldStates: []string{"diagnostics"},
    }

    var wg sync.WaitGroup
    for i := 0; i < 1000; i++ {
        wg.Add(1)
        go func(idx int) {
            defer wg.Done()
            cc := &CompilationContext{
                Frameworks: []string{"/react"},
                DiagnosticCount: idx, // Different integer each time
            }
            // If MatchContext mutates state, go test -race will catch it here.
            _ = atom.MatchesContext(cc)
            _ = atom.Clone()
        }(i)
    }
    wg.Wait()
}
```

### Section D: `TestPromptAtom_DatalogFactTranslation`
```go
func TestPromptAtom_DatalogFactTranslation(t *testing.T) {
    // Verifies Boundary / Type behaviors when generating Datalog facts
    atom := &PromptAtom{
        ID: "fact/test",
        Category: CategoryIdentity,
        IsMandatory: true,
    }

    fact := atom.ToFact()
    // Specifically test that the Mangle Engine expects "/true" instead of true or "true"
    // The Atom/String Dissonance is a primary AI failure mode in Mangle.
    if len(fact.Args) < 5 || fact.Args[4] != core.MangleAtom("/true") {
        t.Fatalf("Mangle Atom translation failed. Expected '/true', got %v", fact.Args[4])
    }
}
```

---

## Part 3: Architecture and Performance Implications

The codeNERD framework handles sub-agent context compilation via JIT. The atom definitions (`PromptAtom` struct) heavily dictate the performance baseline before the LLM takes over.

1. **Big O Complexity Analysis of Context Paging:**
   When an ephemeral sub-agent boots up during the JIT Clean Loop, it runs `MatchesContext` across hundreds of loaded atoms. The loops present in `hasWorldState` and `matchSelector` are currently `O(N*M)` where N is atom requirements and M is compilation context values. Given `N` and `M` are typically < 10, this operates basically in `O(1)` time. However, if a future feature introduces massive nested context selections (e.g., "Must NOT conflict with any of these 500 files"), the nested arrays will become a bottleneck. We might need to consider mapping slices to `map[string]struct{}`.

2. **Pointer Aliasing in Cache:**
   The `EmbeddedCorpus` utilizes caching. The `All()` and `GetByCategory()` methods correctly return deep copies (slices of pointers). However, the underlying structs are still pointers. If a consumer iterates over `GetByCategory` and modifies `atom.Content = "hacked"`, the cached global value is permanently modified, polluting subsequent queries. This is an extreme state conflict that isn't caught by simple `go test` runs unless specifically audited.

3. **Memory Backpressure vs Token Count Estimates:**
   The `TokenCount` estimation (`(len + 3) / 4`) is very rudimentary. Under extreme workloads, if we ingest Minified JavaScript vs Verbose XML, the token ratio varies wildly. If an atom bounds a system context limit strictly by this estimator, a mismatch of true BPE tokenization vs the rough formula could cause LLM inference calls to drop contexts or exceed context windows unexpectedly, violating the `ContextPager` thresholds defined in the budget logic.

## Conclusion

The core atoms structure is generally robust for happy paths. However, the system's reliance on rudimentary slice math, basic pointer distribution, and string token estimation leaves it vulnerable to data races, Mangle type dissonance, and memory issues under extreme load. The addition of the identified test gaps will secure the JIT JIT Prompt Compiler against unpredictable state anomalies during adversarial usage.

---

## Part 4: Advanced Mangle Type Coercion Vectors

### Vector 5: Atom/String Dissonance and Evaluation Errors
1. **The Mangle Tuple Construction Risk:**
   - *Observation:* `ToFact()` handles creating datalog propositions using `core.Fact`.
   - *Gap:* The current type checking inside Mangle does not perform deep validation at the Go boundary if strings are passed instead of explicit Atoms when an Atom is expected by a schema. If the `PromptAtom.Category` is empty (`""`), it prepends a slash resulting in `//`, which Mangle might evaluate as an invalid Name type, failing the entire compilation fixpoint silently.
   - *Remediation Strategy:* Introduce validation inside the test suite:
     - Instantiate an Atom with Category `""`.
     - Run `ToFact()`.
     - Parse the resulting fact using `mangle/parse.Parse(...)`.
     - Expect a syntax or type error rather than a runtime silent failure.

2. **Mangle Arithmetic and Recursive Limit Overflows:**
   - *Observation:* The `Priority` field in `PromptAtom` influences ordering or filtering via Mangle rules.
   - *Gap:* If Priority is passed as `math.MaxInt64`, does the Mangle execution environment properly handle integer precision, or does it silently cast to a lower precision float or integer bounds during arithmetic reductions? Negative testing must confirm bounds logic on integer properties exported to Mangle facts.

### Vector 6: Autopoiesis & Hallucinated States
1. **The "Ouroboros" Edge Case (Infinite Recursion):**
   - *Observation:* Atoms can define `DependsOn` and `ConflictsWith`.
   - *Gap:* There is no explicit prevention mechanism defined inside `atoms.go` for dependency cycles (e.g., A depends on B, B depends on A). The `Validate()` function only checks for *self-dependency* (`A depends on A`).
   - *Remediation Strategy:* The test suite must mock a cycle and pass it to the JIT Compiler test suite to ensure that the compiler's topological sort or Mangle Stratification phase catches the `not P :- P` or cycle evaluation without deadlocking.

2. **Cross-Subsystem Memory Leak via Pointer Passing:**
   - *Observation:* `CompilationContext` uses strings and integers.
   - *Gap:* Memory leaks in Go often happen when subsets of large memory chunks (like a massive LLM trace) are sliced and retained. If a dynamically generated ephemeral `PromptAtom` takes a substring of a huge trace, the entire backing array stays in memory.
   - *Remediation Strategy:* Negative testing should verify that generating an atom from a substring uses `strings.Clone()` when large, preventing GC retention of entire session histories.

---

## Part 5: Negative Testing Expansion Code Implementation

### Section E: Testing Empty Slices vs Nil Slices
```go
func TestPromptAtom_Clone_NilVsEmpty(t *testing.T) {
	// Boundary Value: Nil vs Empty Slice Serialization
	atomEmpty := &PromptAtom{
		ID:               "empty/slice",
		OperationalModes: []string{}, // initialized but empty
		DependsOn:        nil,        // nil pointer
	}

	cloned := atomEmpty.Clone()

	// An empty slice should remain empty (not nil)
	if cloned.OperationalModes == nil {
		t.Errorf("Expected OperationalModes to be empty slice, got nil")
	}

	// A nil slice should remain nil
	if cloned.DependsOn != nil {
		t.Errorf("Expected DependsOn to be nil, got initialized slice")
	}

	// Compare JSON serialization outputs to prove API contracts are maintained
	emptyBytes, _ := json.Marshal(atomEmpty)
	clonedBytes, _ := json.Marshal(cloned)

	if string(emptyBytes) != string(clonedBytes) {
		t.Errorf("JSON serialization divergence due to slice pointer allocation rules")
	}
}
```

### Section F: Type Coercion & Zero-Byte Propagation
```go
func TestPromptAtom_TypeCoercion_ZeroBytes(t *testing.T) {
	// Testing Type Coercion and extremely malformed inputs
	invalidStr := "hello\x00world\xff"

	atom := NewPromptAtom("bad/atom", CategoryIdentity, invalidStr)

	// Ensure hashing does not panic on malformed UTF-8
	hash := HashContent(invalidStr)
	if hash == "" {
		t.Fatalf("Hash generated empty string for invalid UTF-8")
	}

	// Ensure Validation handles it
	// Although currently it will pass because it only checks length > 0,
	// this documents the boundary for future safety validators.
	err := atom.Validate()
	if err != nil {
		t.Fatalf("Validation rejected input too early without explicit rule")
	}
}
```

### Section G: Malformed Mangle Fact Assertion Test
```go
func TestPromptAtom_MalformedMangleFacts(t *testing.T) {
	// A category missing will result in "/" which may break Mangle parsing.
	atom := &PromptAtom{
		ID:          "bad/category",
		Category:    AtomCategory(""),
		Priority:    10,
		TokenCount:  10,
		IsMandatory: false,
	}

	fact := atom.ToFact()

	// fact.Args[1] will literally be "/"
	if fact.Args[1] != "/" {
		t.Errorf("Expected default coercion to '/', got %v", fact.Args[1])
	}

	// Ideally, this should be caught in a future Mangle layer,
	// but the Atom system should probably prevent it via Validate() enhancements.
}
```

### Section H: Deep Cycle Dependency Verification (Future Mangle Tests)
```go
func TestPromptAtom_DependencyCycle_ShouldBeCaughtByCompiler(t *testing.T) {
	atomA := &PromptAtom{
		ID:        "atomA",
		DependsOn: []string{"atomB"},
	}
	atomB := &PromptAtom{
		ID:        "atomB",
		DependsOn: []string{"atomC"},
	}
	atomC := &PromptAtom{
		ID:        "atomC",
		DependsOn: []string{"atomA"},
	}

	// If these atoms are fed into the JIT Compiler, the Stratification
	// analysis must fail or topological sort must panic gracefully.
	// This documents the exact gap in the current atom logic.
	_ = []*PromptAtom{atomA, atomB, atomC}
}
```

---

## Part 6: Performance Optimization Under Constraints

1. **JIT Hot Path Optimization:**
   The `prompt` package is on the extreme hot path of every Clean Loop execution. Every action the user takes recompiles the intent and re-checks `MatchesContext` across hundreds or thousands of atoms.
   - *Improvement:* `NormalizeSelectors` currently modifies slices in place. However, it's called multiple times during benchmarks and potentially runtime. We should introduce an `isNormalized bool` state variable to short-circuit repeated string manipulations on hot path atoms.

2. **Memory Allocation Overheads:**
   `Clone()` allocates memory for every slice within the PromptAtom. For an `EmbeddedCorpus` containing 10,000 atoms, creating copies for every thread-safe `GetByCategory` call can result in high heap allocation churn and trigger premature Garbage Collection cycles.
   - *Improvement:* Utilize `sync.Pool` for `PromptAtom` slices if dynamic manipulation becomes commonplace, or enforce complete immutability for the `EmbeddedCorpus` so that `GetByCategory` can return the original pointers safely without fear of state corruption, relying instead on Go's compiler to enforce read-only semantics where possible (though difficult with slices).

3. **String Builder for Hash Computation:**
   `HashContent` creates a `[]byte(content)` which is a complete memory allocation copy of the original string. If `content` is 50MB, this temporarily allocates another 50MB.
   - *Improvement:* For extremely large contexts, `HashContent` should utilize `io.WriteString` to a hash interface rather than forcing `[]byte()` casting overhead.
   ```go
   func HashContent(content string) string {
       h := sha256.New()
       io.WriteString(h, content) // Avoids byte slice allocation
       return hex.EncodeToString(h.Sum(nil))
   }
   ```

## Part 7: Final Reflections on High-Assurance Agents

codeNERD's reliance on logic programming via Mangle requires that the Go runtime boundary feeding into Mangle is flawless. Every single `PromptAtom` generated acts as an axiom within the logical proof the Mangle kernel attempts to construct.

If Boundary Value conditions (like null slices, unprintable characters, missing categories) are permitted into the EDB (Extensional Database) of the Mangle engine, the logic resolution can result in "Explosion" (everything is true), "Contradiction" (nothing is true), or non-terminating fixpoints (infinite recursion).

This negative testing analysis proves that while the system protects against "Happy Path" failures well, adversarial inputs from malformed AI LLM Piggyback Protocol returns or massive copy-pasted context windows can currently breach the structural integrity of the `PromptAtom` context generator.

The implemented `// TODO: TEST_GAP:` markers directly document these risks for future system hardening campaigns.

---

## Part 8: Expanded Boundary Considerations for Future Iterations

### 1. Zero-Padding and Decimal Conversions
When serializing context values, the JIT JIT Prompt Compiler currently treats everything as simple strings for pattern matching.
- **Gap:** In scenarios where atoms define floating point requirements (e.g. `Version` matching or token budget thresholds represented as strings), `0` and `0.0` or `0.00` might logically equate but fail standard string matching.
- **Remediation:** Introduce semantic typing or explicit coercion rules prior to `MatchContext` if numbers are introduced into selector lists.

### 2. Time Traveling Attacks (Temporal Facts)
codeNERD supports temporal fact logic via its git binding and history tracking.
- **Gap:** `PromptAtom.CreatedAt` is set to `time.Now()`. If a mock kernel injects atoms from a pre-recorded session (via `/load-session`), and these atoms are passed directly into the evaluator, the system might perceive them as dynamically newly generated.
- **Remediation:** Ensure `Clone()` accurately preserves temporal markers, and negative testing should specifically generate atoms with `time.Time{}` (the zero value) to see if serialization/deserialization into Mangle facts (which often convert time to Unix epochs) handles zero correctly without throwing `Out of Bounds` errors in SQLite or Mangle.

### 3. Extremely Nested JSON
If `Content` wraps deeply nested structured outputs (e.g., from an LLM Piggyback trace), and `EstimateTokens` attempts to blindly parse it as normal strings, the token counting might be mildly inaccurate. However, more dangerously, if `Content` is piped to a UI component (like Bubbletea) without sanitization, deeply nested brackets might cause rendering engines to pause or crash.
- **Gap:** UI / Content rendering mismatch.
- **Remediation:** Negative tests should feed atoms with 10,000+ nested `[{[{...}]}]` into the `Content` property and ensure `PromptAtom.Validate()` still completes within micro-seconds.

### 4. Cross-Subagent Contamination via Embedded Corpus
The `EmbeddedCorpus` is loaded once and shared. If a bug is introduced in `prompt.go` where it accidentally modifies an atom's `Priority` dynamically (e.g. "We need this atom to take precedence right now"), that priority is permanently modified for *all* future sub-agents.
- **Gap:** Lack of immutability guarantees.
- **Remediation:** In Go, you cannot make fields `const`. The test suite should use reflection to assert that after 1000 randomized JIT compilations, the underlying `EmbeddedCorpus` memory addresses and values match a pristine baseline hash perfectly.

### 5. Final Code Quality Assurance Metric
By implementing all these tests, the `prompt` package's test coverage will not only reach high line-coverage but will achieve true *mutation-resilient* logic coverage. This is the difference between writing tests that prove the code runs and writing tests that prove the code cannot run incorrectly under adversarial pressure.

### 6. Extremely Long Atom Identifiers
Mangle facts often assume relatively short atom names (e.g. `atom123`).
- **Gap:** If `ID` strings exceed what the Mangle internal dictionary handles optimally (e.g., a 10MB string as an ID), does the system crash during `ToFact()` translation or during Mangle's internal String-to-Atom mapping?
- **Remediation:** Introduce negative tests that populate `PromptAtom.ID` with heavily massive, unique strings to see if the Mangle layer panics or truncates silently.

### 7. Overlapping Conflict and Dependency Maps
- **Gap:** Can an atom be both a dependency and a conflict?
- **Remediation:** A test should try setting `DependsOn: []string{"A"}` and `ConflictsWith: []string{"A"}` on the same PromptAtom. `Validate()` should catch this logical contradiction explicitly before compiling to Datalog.

### 8. Null characters in User Prompts
- **Gap:** When an LLM Piggyback generates prompt extensions containing `\0`, `HashContent` will process it, but if it gets rendered to standard out, it may cause early termination of strings in terminal interfaces.
- **Remediation:** Test rendering atom content containing `\0` and verify the output.

### 9. Token Budget Edge Cases
- **Gap:** When Token budgets approach `0` or exactly equal `1`, the behavior of estimation rounding `(len+3)/4` might cause atoms to be prematurely filtered or included when they shouldn't.
- **Remediation:** Explicit testing of atom selections where Budget strictly equals 0, 1, 2.

### 10. Conclusion and Forward Work
This comprehensive suite of edge cases defines the boundary parameterization necessary for stabilizing codeNERD's Prompt Architecture under highly adversarial conditions. By executing this plan, the JIT clean loop will become significantly more robust.

### 11. Concurrency During Corpus Load
- **Gap:** Loading the Embedded Corpus might happen simultaneously with compilation in rare hot-reloads.
- **Remediation:** Assure sync primitives lock properly during instantiation.

### 12. Validating Duplicate Categories
- **Gap:** Is it possible for atoms to declare invalid categories dynamically?
- **Remediation:** Strong type checking on atom definitions.

### 13. Deep Copy Performance
- **Gap:** Is Deep Copy performing slowly due to allocations?
- **Remediation:** Benchmark Clone with deep slices.

### 14. Reflection
The tests must not just run, but ensure structural safety.
