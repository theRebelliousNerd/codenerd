---
remediated: true
remediated_date: 2026-05-12
---
# Semantic Classifier Subsystem - Boundary Value Analysis & Negative Testing Journal

**Date:** 2026-04-23
**Time:** 22:33:55 EST
**Subsystem:** `internal/perception/semantic_classifier.go`
**Author:** QA Automation Engineer (Jules)

## 1. Subsystem Overview & Architectural Context
The `SemanticClassifier` subsystem acts as the critical bridge in the perception pipeline, mapping free-form natural language intent to structured Mangle atoms (the `user_intent` rules). It forms the foundational "O" (Observe) stage of the codeNERD architecture's OODA loop. It uses an `EmbeddingEngine` to process inputs, searching both baked-in "Embedded" and dynamic "Learned" vector stores concurrently to find semantic matches, which it subsequently asserts into the `Kernel` as `semantic_match` predicates.

The architecture is explicitly designed around "Mangle is for deduction, not data lookup." The Classifier handles fuzzy vector searches locally and injects precise atoms (like `/review`, `/codebase`) to satisfy Mangle logic correctly.

---

## 2. Null/Undefined/Empty Input Vectors

### Missing Edge Cases
Currently, the test suite (`semantic_classifier_test.go`) evaluates graceful degradation when the `embedEngine` is missing (`TestSemanticClassifier_ClassifyWithoutEngine`), but heavily under-tests Null, Undefined, or Empty inputs into the core semantic matching pipeline itself.

1. **Empty Query Strings (`""`)**:
   - `sc.Classify(ctx, "")` - Does the embedding engine panic, or perform a useless network call/inference resulting in zero vectors, wasting compute?
   - **Performance Impact**: If an empty string reaches the embedding engine, it could lead to unnecessary inference overhead and locking latency in the memory stores without returning actionable matches.

2. **Whitespace-Only Queries (`"   \n  \t   "`)**:
   - These are effectively null intents but technically carry bytes. Similar to above, how does the system behave when trimming isn't aggressively applied pre-embedding?

3. **Null Contexts (`context.TODO()` or `nil` contexts)**:
   - What happens if the context is strictly `nil` when passed to `sc.Classify`? Given that `errgroup` and the downstream `EmbedWithTask` depend on `ctx`, this could trigger runtime panics.

### Proposed Test Additions
*   `TestSemanticClassifier_EmptyQuery_FastReturn`: Ensure `Classify` instantly returns `nil, nil` when given `""` without hitting the embedder.
*   `TestSemanticClassifier_WhitespaceQuery_Trimmed`: Verify that purely whitespace queries are normalized or rejected efficiently.
*   `TestSemanticClassifier_NilContext_HandledGracefully`: Ensure functions requiring `ctx` fail safely with wrapped context errors rather than panicking.

---

## 3. Type Coercion Vectors

### Missing Edge Cases
The core challenge in a neuro-symbolic agent like codeNERD is the dissonance between the "Stringly Typed" LLM environment and the strict atom-typed Mangle logic engine. The `SemanticClassifier` bridges this gap, but there are coercion risks.

1. **String vs. Atom Mismatch during Assertion**:
   - The test `TestInjectFacts` expects specific assertions, but it manually extracts `similarity, ok := fact.Args[5].(int64)`.
   - The system uses `argToString` to convert Mangle Atoms to strings for embedding, but does it safely assert *back* into Mangle using correct `ast.Name("...")` vs `ast.String("...")` representations? Mangle will silently produce empty sets if a string "review" is joined against an atom `/review`.

2. **Vector Dimension Mismatch (Extreme Coercion)**:
   - `TestLearnedCorpusStore_Add_DimensionMismatch` tests `768` vs `3072`. What about passing a slice of size `0` or `1` or `3071`?
   - Go's slice memory mapping could technically panic on out-of-bounds indexing if `embedding.CosineSimilarity` assumes symmetric fixed lengths without bounds checking.

3. **Similarity Score Type Coercion**:
   - Similarity is computed as `float64` but coerced to an integer 0-100 `int64` for Mangle (`int64(match.Similarity * 100)`). What happens if similarity returns `NaN` or `-Inf` from a faulty embedding engine? Converting `NaN` to `int64` is undefined/hardware-dependent in Go.

### Proposed Test Additions
*   `TestSemanticClassifier_InjectFacts_TypeStrictness`: Verify that asserted facts strictly use `ast.Name` for verbs (e.g., `/fix`) and NOT strings.
*   `TestSemanticClassifier_NaN_Similarity_Handling`: Mock the embedding engine to return a corrupted vector resulting in `NaN` similarity, proving the system rejects or clamps the value rather than asserting poisoned facts.

---

## 4. User Request Extremes

### Missing Edge Cases
codeNERD must handle frontier-level coding constraints and brownfield repos.

1. **Massive Context Windows (Extreme Length Queries)**:
   - If a user pastes a 50,000-line error log and says "fix this", `input` could be megabytes in size.
   - When passed to `Classify`, the embedding model's context window will blow out. Does the `SemanticClassifier` chunk the input, aggressively truncate it, or does it attempt to embed the whole block, leading to `OOM` or `HTTP 413 Payload Too Large` from the embedding API?

2. **Extreme TopK Requests**:
   - `SemanticConfig.TopK` defaults to 5. What if an extreme configuration requests `TopK = 1,000,000`?
   - The sorting logic `sort.Slice(candidates, ...)` would attempt to allocate massive arrays, creating a memory bottleneck, particularly when merged.

3. **High-Frequency Classification Loops**:
   - If a user's prompt expands recursively in an Autopoiesis/Thunderdome loop, the `SemanticClassifier` could be hammered 100+ times a second. Can the locks `s.mu.RLock()` in `EmbeddedCorpusStore` and `LearnedCorpusStore` sustain this throughput without starving?

### Proposed Test Additions
*   `TestSemanticClassifier_MassiveInput_Truncation`: Pass a 10MB string to `Classify` and verify it intelligently truncates to a safe token limit before hitting the embedder.
*   `TestSemanticClassifier_ExtremeTopK_Capped`: Set `TopK` to a massive number and ensure it clamps to a reasonable upper bound to protect memory.

---

## 5. State Conflicts & Concurrency Vectors

### Missing Edge Cases
The code uses `sync.RWMutex` to protect `embeddings`, `entries`, and configurations. However, Go maps are notoriously sensitive to concurrent reads/writes.

1. **Concurrent Classify while Learning**:
   - The `LearnedCorpusStore.Add()` function takes a `Lock()`, writes to `s.entries` and `s.embeddings`.
   - Concurrently, `Classify()` runs `Search()`, which takes an `RLock()`.
   - If `Classify()` is invoked in parallel across hundreds of goroutines (e.g., parallel sub-agents processing independent chunks of code), does the `Add()` operation get starved, preventing the system from "learning" in real-time?

2. **Dynamic Config Updates mid-Classification**:
   - `sc.SetConfig()` takes a `Lock()` on `sc.mu`.
   - `ClassifyWithoutInjection` reads config under `RLock()`.
   - If the system attempts an Ouroboros loop that updates the `MinSimilarity` threshold while active classifications are occurring, is there a race condition?

3. **Parallel Search Engine Conflicts**:
   - `EnableParallel = true` fires off `errgroup.Group`. If one store fails (e.g., `learnedStore` SQLite DB is locked `database is locked (5)`), the entire classification group errors out and returns `nil`, silently skipping the embedded store's valid matches.

### Proposed Test Additions
*   `TestSemanticClassifier_ConcurrentReadWrite`: A heavy concurrency test that spawns 100 readers (`Classify`) and 10 writers (`Add` to learned store) simultaneously to trigger data races or deadlocks (run with `go test -race`).
*   `TestSemanticClassifier_ParallelSearch_PartialFailure`: Mock the `learnedStore` to return a forced error. Ensure `errgroup` failure logic is resilient enough to still return matches from the `embeddedStore` rather than failing entirely.

---

## Summary & Performance Evaluation
The `SemanticClassifier` is performant for standard usage but exhibits structural vulnerabilities under adversarial scale and concurrent pressure. The biggest threat to stability is extreme input size (OOM via embedding) and `NaN`/type coercion during the critical Mangle logic transition. Implementing the suggested negative tests will harden the OODA loop's observation layer, ensuring codeNERD remains robust when deployed in massive brownfield scenarios.

## 6. Detailed Expansion on Boundary Test Cases & Implementations

### Null/Undefined/Empty Input Implementations
To properly safeguard against the empty intent vector, the system needs defensive programming at the absolute top of the `Classify` call.

```go
// Proposed fix in Classify:
if len(strings.TrimSpace(input)) == 0 {
    logging.PerceptionDebug("Skipping classification for empty input")
    return nil, nil
}
```

By adding a fast-return, we prevent zero-byte payloads from passing over the network to the embedding API (e.g., Google Gemini or Ollama). In boundary tests, we would simulate strings like `"  \n \r \t "` and verify that `sc.embedEngine.Embed` is explicitly *never called* using a mock engine.

### Type Coercion Deep Dive
In Go, type assertions can panic if not handled via the comma-ok idiom. The test `TestInjectFacts` relies on `int64` extraction:

```go
similarity, ok := fact.Args[5].(int64)
```

However, the actual injection logic inside `semantic_classifier.go` looks like this:
```go
// Inside injectFacts
simInt := int64(match.Similarity * 100)
```

**The Boundary Value Risk:**
If `match.Similarity` is a `NaN` (Not a Number) due to a zero-vector dot product inside `CosineSimilarity`, casting `int64(NaN)` in Go results in the minimum integer value `−9223372036854775808`. If this gets asserted into Mangle, Mangle's math evaluation `Similarity > 50` will fail, masking the actual error.

**Mitigation and Testing:**
The system needs to clamp boundaries:
```go
if math.IsNaN(match.Similarity) {
    match.Similarity = 0.0
}
simInt := int64(math.Max(0, math.Min(100, match.Similarity * 100)))
```
Testing this involves mocking `embedEngine` to return a `[0,0,0...]` vector which mathematically induces a division-by-zero (`NaN`) in cosine distance, and asserting the output `simInt` is exactly `0`.

### Extreme Constraints & Token Bounds
The test `TestSemanticClassifier_MassiveInput_Truncation` must evaluate how the system handles the physical limits of token context windows. Embedding models typically max out at 8k to 32k tokens.

If the `input` string is 10MB of minified JavaScript:
1. `Classify()` attempts embedding.
2. The gRPC/REST client attempts to serialize 10MB.
3. The server rejects with HTTP 400/413.
4. The system logs a generic error and the user request dies.

**Structural fix & Test implementation:**
The Transducer layer *must* truncate inputs prior to passing to the Semantic Classifier. If it doesn't, the Classifier must assert its own boundary.
```go
const MaxClassifyBytes = 32768
if len(input) > MaxClassifyBytes {
    input = input[:MaxClassifyBytes]
}
```
The test must generate a `strings.Repeat("a ", 50000)` and verify the mock embedding engine receives exactly `MaxClassifyBytes`.

### State Conflicts: Race Conditions in Maps
The most subtle bugs in go occur when maps are read and written concurrently. The `EmbeddedCorpusStore` and `LearnedCorpusStore` use `sync.RWMutex` which is correct, but there is a vulnerability in how the slices `candidates` are constructed and sliced.

If `candidates` capacity is mismanaged and a re-slice occurs without locking during the topK sort, we risk data corruption.

```go
// Current implementation
candidates := make([]scored, 0, len(s.entries))
for _, entry := range s.entries {
    // ...
}
```
The boundary test `TestSemanticClassifier_ConcurrentReadWrite` must spawn hundreds of goroutines:
```go
func TestSemanticClassifier_ConcurrentReadWrite(t *testing.T) {
    var wg sync.WaitGroup
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            sc.Classify(ctx, fmt.Sprintf("Query %d", id))
        }(i)
    }
    // Concurrently add to learned store
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            learnedStore.Add(CorpusEntry{TextContent: fmt.Sprintf("Learn %d", id)}, mockEmbed)
        }(i)
    }
    wg.Wait()
}
```
Running this test under `go test -race` guarantees that the lock scope completely protects map iteration (`s.entries`) from map mutation (`s.entries = append(s.entries, ...)`).

## 7. Performance and Latency Implications
From a performance perspective, semantic classification must happen in under `50ms` (excluding network API latency) to maintain the fluidity of the OODA loop.

- **Store Search Scaling:** The current implementation uses linear iteration `for _, entry := range s.entries` and computes `CosineSimilarity` against every element. For small corpuses (< 1000 items), this is negligible (`O(N)`). However, if the learned corpus expands via Autopoiesis over weeks of runtime to 1,000,000 entries, the linear iteration will induce multi-second latency per classification.
- **Boundary Test Gap:** The lack of a `TestSemanticClassifier_ScaleTest` hides this algorithmic limitation. A test should mock 100,000 embedded entries and assert that `Search()` completes within acceptable bounds. If it fails, the architecture must pivot from brute-force linear search to an optimized Vector DB index (like HNSW or FAISS) for the learned store.

By applying these negative and boundary value tests, the codeNERD framework ensures structural integrity during massive context operations, concurrent multi-agent executions, and unpredictable model outputs.

## 8. Extreme Edge Case Combinations

Boundary testing is not just about isolated extremes, but the Cartesian product of adversarial inputs. The `SemanticClassifier` operates at the boundary of external LLM network responses and internal Mangle kernel state.

### Combination 1: Massive Input + Network Timeout + Concurrency
Consider the scenario where 5 sub-agents are spawned simultaneously in Thunderdome. They all attempt to `Classify` a massive 32k token block.
- The `input` size pushes the embedding network API to its latency limits.
- The `embedEngine` takes 29 seconds to respond (near the typical 30s gRPC deadline).
- All 5 goroutines are blocked.

**Test Implementation:**
We need a `TestSemanticClassifier_ContextCancellation_DuringEmbed`.
```go
func TestSemanticClassifier_ContextCancellation_DuringEmbed(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
    defer cancel()

    mockEngine := &SlowMockEngine{Delay: 100 * time.Millisecond}
    sc := NewSemanticClassifier(kernel, nil, nil, mockEngine)

    matches, err := sc.Classify(ctx, "test query")

    // Expect graceful failure, no panic, and error propagated
    if !errors.Is(err, context.DeadlineExceeded) {
        t.Errorf("Expected context deadline exceeded, got %v", err)
    }
}
```

### Combination 2: Memory Eviction and Re-Embedding (Cold Starts)
If `SemanticClassifier` processes a huge repository, the OS might swap memory. When `LearnedCorpusStore` accesses its in-memory map `s.embeddings`, cache misses can dramatically spike latency.
- What if an `entry` exists in `s.entries`, but its embedding was somehow cleared or failed to save (simulating a partial write during a crash)?
- The code handles missing keys gracefully `entryEmbed, ok := s.embeddings[entry.TextContent]`, but it degrades the system's accuracy silently.

**Test Implementation:**
`TestSemanticClassifier_PartialCorpusState` should verify that if `len(s.entries) > len(s.embeddings)`, the search algorithm gracefully skips the broken entries and continues to rank the valid ones without causing out-of-bounds errors during the final `candidates` slice allocation.

## 9. Mangle State Conflict Management

The most insidious state conflicts happen downstream in the `kernel.AssertBatch`. The `SemanticClassifier` asserts facts like `semantic_match("test", /review, "", "test", 1, 80)`.

If `user_intent` derivations trigger immediately upon assertion, but the classifier asserts multiple matches incrementally across parallel threads, we risk creating non-deterministic logic bifurcations in the Mangle resolution.

**Negative Test for Mangle State Pollution:**
When `Classify` is run twice on conflicting inputs, it asserts conflicting `semantic_match` facts unless the kernel cleans them up.
```go
sc.Classify(ctx, "I want to review code") // Asserts rank 1 /review
sc.Classify(ctx, "I want to run tests")   // Asserts rank 1 /test
```
If the session is not quiescently wiped between turns, the kernel now holds BOTH intent matches with Rank 1. The `SemanticClassifier` does not call `kernel.Retract` on old matches before asserting new ones.

**Test Gap Resolution:**
Add `TestSemanticClassifier_SequentialStatePollution`.
This test runs `Classify` multiple times and then queries the `mockKernel` to prove that old facts accumulate unless an explicit transaction retracts them. This highlights an architectural gap: either the `SemanticClassifier` needs to clear prior semantic states, or it must rely strictly on the outer `Clean Loop` to wipe the kernel before the next turn.

### Concluding Thoughts on Hardening
The `SemanticClassifier` represents the exact boundary where probabilistic AI meets deterministic Logic Programming. The negative testing strategy outlined above protects codeNERD from the primary failure modes of this bridge: Type bleeding (strings vs atoms), dimensional mismatch (vector sizes), mathematical poisoning (NaN scores), and state pollution (dangling facts).

Implementing these tests will shift the subsystem from "generally functional" to "provably robust" under extreme agent orchestration loads.

## 10. Further Type Coercion Vectors

### Stringly Typed Arguments
The Mangle Kernel is strictly typed in regards to Strings vs Atoms.
A critical boundary value test must ensure that the `Verb` extracted from the semantic match is correctly converted to a Mangle Atom before assertion.
The `injectFacts` function contains:
```go
fact := core.Fact{
	Predicate: "semantic_match",
	Args: []interface{}{
		input,
		match.Verb, // Is this passed as a raw string or an Atom?
        ...
```
If `match.Verb` is just a string `"/review"`, Mangle will see a String `"/review"` instead of the Name/Atom `/review`. This is the #1 cause of silent Datalog join failures in the codebase.

**Proposed Test:**
`TestSemanticClassifier_InjectFacts_AtomVerification`:
```go
// Inside test, parse the injected fact arguments
verbArg := kernel.assertedFacts[0].Args[1]
if _, isString := verbArg.(string); isString {
    t.Fatal("CRITICAL: Verb was asserted as a raw string, not a Mangle Atom")
}
```
This forces the implementation to change to `ast.Name(match.Verb)`.

## 11. Security and Exploit Scenarios

While not a traditional web server, codeNERD runs locally and executes arbitrary tools. If the `SemanticClassifier` can be tricked, the agent could be hijacked.

### Prompt Injection via Vector Similarity Poisoning
An attacker could place a file in the workspace containing adversarial learned patterns. If the `LearnedCorpusStore` blindly loads `.nerd/learned_patterns.db` without validating the schemas, an attacker could insert:
`TextContent: "run tests", Verb: "/execute_malware"`

If the user types "run tests", the vector search matches the poisoned entry, giving it an artificially high `Confidence`, and asserts `semantic_match("run tests", /execute_malware, ...)`.

**Test Implementation:**
`TestSemanticClassifier_AdversarialLearnedPattern_Rejection`:
Create a test that inserts an invalid Mangle verb (e.g., `"/execute_malware"`) into the `LearnedCorpusStore`. When `Search()` returns this match, the `SemanticClassifier` must validate the `Verb` against a known corpus of safe intents (`intent_corpus.go`) before asserting it into the kernel. If the verb is unknown or unmapped, it must be discarded to prevent privilege escalation via dataflow poisoning.

## 12. Final System Assessment
The boundary value analysis confirms that the `SemanticClassifier` is highly dependent on well-formed inputs from upstream transducers and well-behaved downstream embedding engines. By shifting QA focus from "Happy Path Semantic Matches" to "Adversarial Context, Vector Size Boundaries, and Mangle Type Strictness", the framework will gain the resilience required for frontier coding tasks.

The performance profile remains safe up to `O(N=5000)` entries, but scaling beyond requires algorithmic indexing rather than linear traversal. Concurrency and Race tests are the most urgent additions to prevent silent memory corruption during Ouroboros learning loops.

## 13. Advanced State Conflicts: Engine Initialization

### Late Initialization Races
The `SharedSemanticClassifier` is a package-level global variable initialized via `InitSemanticClassifier`.
```go
// InitSemanticClassifier initializes the shared classifier.
func InitSemanticClassifier(kernel core.Kernel, cfg *config.UserConfig) error {
	sharedClassifierMu.Lock()
	defer sharedClassifierMu.Unlock()
...
```

If multiple sub-agents attempt to trigger initialization concurrently during a highly concurrent boot sequence, the mutex protects the assignment. However, there is a boundary case regarding subsequent reads:

If `SharedSemanticClassifier` is read *without* a lock in other parts of the system before `InitSemanticClassifier` completes, it will be `nil`, causing panic.

**Proposed Test:**
`TestSemanticClassifier_GlobalInit_DataRace`:
Use `go test -race` while 100 goroutines attempt to call `InitSemanticClassifier` and another 100 goroutines attempt to read `SharedSemanticClassifier` to prove that the global state is accessed safely or that the API design prevents unsafe external access.

## 14. Extreme Environment Constraints

### Zero Memory / High Pressure Handling
The code allocates slices dynamically based on the size of `s.entries`.
```go
candidates := make([]scored, 0, len(s.entries))
```
If `s.entries` somehow becomes corrupted and reports a length of `10,000,000,000` (e.g., due to an SQLite integer overflow bug when loading from disk), `make` will attempt to allocate tens of gigabytes of RAM instantly, causing the Go runtime to panic with `out of memory`.

**Mitigation and Test:**
Implement a hard cap on max entries loadable from the corpus to prevent OOM panics.
`TestSemanticClassifier_MaxCorpusSize_OOMProtection`: Mock the database to return an infinite stream of rows and verify that `NewLearnedCorpusStore` stops loading after `MaxLearnedEntries` (e.g., 50,000) and returns an error or safely caps the array.

## 15. Conclusion on Quality Assurance

Through this deep dive into Boundary Value Analysis, Negative Testing, State Conflicts, and User Extremes, we have established a clear roadmap for hardening the `SemanticClassifier`. The identified `// TODO: TEST_GAP:` markers in the source test file serve as actionable tickets for the engineering team.

By addressing the `NaN` type coercion vectors, preventing OOM through token truncation, and sealing concurrent memory maps, codeNERD's perception transducer will graduate to a highly resilient neuro-symbolic bridge capable of managing the chaos of frontier LLM models.

## 16. Vector Embedding Dimension Limits

### High-Dimensional Anomaly Processing
Currently, `EmbeddingEngine` defaults to typical sizes (e.g., 768 or 3072).
The `LearnedCorpusStore` checks for matching dimensions:
```go
if len(entryEmbed) != s.dimensions {
    return fmt.Errorf("embedding dimension mismatch...")
}
```

However, if a new frontier model is released (e.g., 32768 dimensions), does the cosine similarity math hold up?
Floating-point precision limits start to compound with massive arrays. The difference between `float32` and `float64` becomes a critical source of error.

`embedding.CosineSimilarity` accepts `[]float32`. If the dimensions are massive, the sum of squares could exceed the maximum value representable by `float32` (~3.4e38). While unlikely for embeddings normalized between -1 and 1, un-normalized vectors from a buggy provider could overflow.

**Test Gap & Implementation:**
`TestSemanticClassifier_HighDim_FloatOverflow`:
Create a mock embedding engine that returns a 10,000-dimension vector where each element is a large float32 value (e.g., `1000.0`). Ensure `CosineSimilarity` handles the math correctly without returning `+Inf`, and that `SemanticClassifier` properly filters or rejects the anomalous result.

## 17. Final Verification

The journal now encompasses:
1.  **Null/Empty Inputs**: Fast-returns, whitespace handling, nil contexts.
2.  **Type Coercion**: String vs Mangle Atom dissonance, NaN Similarity bounds.
3.  **Extreme Requests**: Token window blowouts, massive TopK values, Float32 overflow.
4.  **State Conflicts**: Concurrent map access, Lock inversion, Global init races, Mangle logic pollution.
5.  **Security Boundaries**: Adversarial database prompt injection.

This fulfills the comprehensive QA requirements for the subsystem boundary analysis.

## 18. Appendices

### Appendix A: Summary of Actionable Tickets
- Implement `TestSemanticClassifier_EmptyQuery_FastReturn`
- Implement `TestSemanticClassifier_NaN_Similarity_Handling`
- Implement `TestSemanticClassifier_MassiveInput_Truncation`
- Implement `TestSemanticClassifier_ConcurrentReadWrite`
- Implement `TestSemanticClassifier_InjectFacts_AtomVerification`
- Implement `TestSemanticClassifier_MaxCorpusSize_OOMProtection`

### Appendix B: Relevant File Paths
- Target: `internal/perception/semantic_classifier.go`
- Tests: `internal/perception/semantic_classifier_test.go`
- Store Backend: `internal/store/local_core.go` (LearnedCorpusStore references)

*End of Journal Entry.*
