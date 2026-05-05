# Boundary Value Analysis and Negative Testing Journal
## Subsystem: SemanticClassifier (internal/perception/semantic_classifier.go)
**Date:** 2026-03-09_04-26-EST

### 1. Introduction and Architectural Context
The `SemanticClassifier` within the `perception` package of codeNERD serves as the neuro-symbolic bridge for intent classification. It takes a raw string input (presumably user intent or system-generated text), generates vector embeddings using an embedding engine, and matches these embeddings against two corpora:
1. `EmbeddedCorpusStore`: A static, baked-in intent corpus (often hydrated from Mangle's kernel).
2. `LearnedCorpusStore`: A dynamic, user-specific corpus capturing new patterns learned over time (typically backed by SQLite).

The results from both stores are merged, with learned patterns receiving a slight numerical boost to favor user-specific adaptations. The matching results are then asserted back into the Mangle kernel as `semantic_match` facts, effectively converting distributed continuous vector space operations into discrete, declarative logic atoms.

This document serves as a deep-dive Boundary Value Analysis (BVA) and Negative Testing review of the subsystem. The goal is to identify explicit unhandled edge cases, failure vectors, and architectural blind spots outside the typical "Happy Path" of standard code interaction.

### 2. Null, Undefined, and Empty States (The Void Vectors)
This category addresses the system's resilience when data is missing, conceptually void, or strictly nullified.

**2.1 Empty String Input on Classify**
When `SemanticClassifier.Classify(ctx, "")` is invoked:
- The input is passed directly to the `embedEngine.Embed(ctx, "")`.
- Depending on the upstream embedding provider (e.g., OpenAI, Gemini, local Ollama), an empty string can cause a provider-specific error (HTTP 400 Bad Request) or return an empty vector `[]float32{}`.
- If it returns an empty vector, `CosineSimilarity` in the store searches will divide by zero (or the magnitude of an empty vector), potentially yielding `NaN`.
- Although `SemanticClassifier` has graceful degradation ("falling back to regex-only"), an unhandled `NaN` similarity could corrupt the `mergeResults` sorting logic.
- *Performance Impact:* High network latency if sent over the wire, followed by failure. Performant enough for local operations but wasteful. Needs early-exit guard clause.

**2.2 Whitespace-only Input**
Passing purely whitespace `Classify(ctx, "   \n\t  ")` causes identical issues to the empty string, but bypasses naive `if input == ""` checks. Providers generally reject tokenless strings.

**2.3 Nil Embedding Engine Behavior**
If `NewSemanticClassifierFromConfig` fails to construct an `embedEngine`, it swallows the error and returns a degraded classifier where `embedEngine == nil`, `embeddedStore == nil`, and `learnedStore == nil`.
- While `ClassifyWithoutInjection` handles `embedEngine == nil` by returning `nil, nil`, `AddLearnedPattern` does not check gracefully. It returns `fmt.Errorf("embedding engine not available")`.
- This is correctly handled but breaks the "Ouroboros" learning loop silently for the user.

**2.4 Zero Dimension Embeddings**
If a configuration error causes `dimensions == 0` in `EmbeddedCorpusStore`:
- When `Search` computes `CosineSimilarity`, it may encounter panics or `NaN`.
- When `LoadFromKernel` checks `if len(vec) != s.dimensions`, it will falsely accept empty vectors if `s.dimensions` is 0, leading to poisoned entries.

### 3. Type Coercion and Data Shape Mutations
Go is statically typed, but logical mismatches and implicit conversions (especially integrating with Mangle and JSON/SQLite borders) present vectors.

**3.1 Similarity Score Scaling Integer Truncation**
When injecting facts, the system scales similarity (float64) to an integer:
`int64(match.Similarity * 100)`
- *Vector:* If `match.Similarity` is somehow negative (Cosine similarity ranges from -1.0 to 1.0, though typically 0 to 1.0 in text embeddings), `int64(-0.5 * 100)` becomes `-50`. Does the Mangle schema for `semantic_match` accept negative integers for rank/similarity?
- *Vector:* If `Similarity` is `NaN` due to zero-vector division, converting `NaN * 100` to `int64` is technically undefined or architecture-dependent in Go (often results in the minimum integer value, e.g., -9223372036854775808).
- *Performance:* This type coercion is extremely cheap (O(1)), so it is performant, but logically flawed if `NaN` leaks in.

**3.2 MangleAtom Coercion**
`core.MangleAtom(match.Verb)`
- If the `Verb` does not start with a `/` (e.g., "review" instead of "/review"), it becomes an invalid Atom in Mangle syntax, causing down-stream parse errors or missing rule matches in the kernel.
- The `LearnedCorpusStore.Add` accepts arbitrary `verb` strings without validating if they match the `/\w+` Atom regex format.

**3.3 TopK Zero or Negative Coercion**
- Handled gracefully in `Search`: `if topK <= 0 { topK = 5 }`.
- However, in `mergeResults`, `maxResults = cfg.TopK * 2`. If a user manually overwrites `SemanticConfig{TopK: -10}`, `mergeResults` might slice `deduped[:maxResults]` with a negative index, causing a runtime panic.
- Wait, the search functions locally coerce `topK` to 5, but the `mergeResults` function uses the raw `cfg.TopK` without coercion!
```go
	maxResults := cfg.TopK * 2
	if len(deduped) > maxResults {
		deduped = deduped[:maxResults] // PANIC if maxResults < 0
	}
```
- *Performance:* Panic takes down the entire loop. Highly severe.

### 4. User Request Extremes (The Frontier Vectors)
How does the system respond when stressed with extreme scaling?

**4.1 The Monorepo Dump (Massive Input String)**
A user might mistakenly pipe a 50MB log file or an entire concatenated codebase into the prompt instead of a short request.
- `Classify(ctx, <50MB_String>)` passes the entire 50MB string to the embedding engine.
- Providers (and local Ollama) have strict token limits (e.g., 8192 tokens).
- The embedding call will fail with a token limit exceeded error after a significant network/computation delay.
- The system will fallback to regex (graceful degradation), but the user pays a massive latency penalty (seconds or minutes) and memory allocation (50MB strings duplicated in memory) before the failure.
- *Performance:* The subsystem is **not performant** for this edge case. Input strings should be proactively truncated to a reasonable chunk (e.g., 2048 characters) *before* attempting embedding, as intents are typically established in the first few sentences.

**4.2 Massive Learned Pattern Additions**
Autopoiesis might generate an excessively long regex or rule pattern.
- Passing a 1MB string to `AddLearnedPattern` will successfully embed and store it in SQLite.
- Doing this repeatedly balloons the `learned_patterns.db` vector index, breaking search performance.

**4.3 Configuration Extremes: TopK = 1,000,000**
If a user specifies a massive `TopK`:
- `Search` operations attempt to allocate slices `results = make([]SemanticMatch, len(candidates))` up to the corpus size, but `mergeResults` will attempt to merge and sort massive arrays.
- Since embeddings are dense (e.g., 1536 dims), loading and sorting massive results is very CPU heavy.
- *Performance:* Go's `sort.Slice` is O(N log N). For N=1,000,000 this is noticeable latency, though memory allocation is the more immediate bottleneck. A maximum cap on `TopK` (e.g., 100) must be enforced during `SetConfig`.

**4.4 Excessive LearnedBoost values**
- A user might set `LearnedBoost: 10.0`.
- Handled safely: `if learned[i].Similarity > 1.0 { learned[i].Similarity = 1.0 }`. This bounds the score properly, though it effectively destroys rank sorting between learned patterns (they all tie at 1.0).

### 5. State Conflicts and Concurrency
This vector examines race conditions and logical idempotency failures when the system is heavily concurrent.

**5.1 EmbeddedCorpusStore.LoadFromKernel Idempotency (Ghost Duplication)**
```go
	s.mu.Lock()
	defer s.mu.Unlock()
	added := 0
	for i, entry := range entries {
//...
		s.entries = append(s.entries, entry)
		s.embeddings[entry.TextContent] = vec
//...
```
- If `LoadFromKernel` is called multiple times (e.g., kernel is reloaded dynamically via JIT context swapping), the `EmbeddedCorpusStore` blindly appends the entries.
- This results in duplicated memory entries. If reloaded 100 times, the corpus size is 100x larger, blowing up memory and O(N) search time.
- *Performance:* Causes a memory leak and catastrophic latency degradation over the lifetime of a long-running session. The map `s.embeddings[entry.TextContent] = vec` deduplicates the embeddings, but `s.entries` slice grows unboundedly!

**5.2 Kernel Fact Assertion Race**
- In `injectFacts`, `sc.mu.RLock()` protects `kernel` pointer reading.
- However, it unlocks, then iterates and calls `kernel.Assert(fact)` or `kernel.LoadFacts(facts)`.
- If the `kernel` is actively being swapped, reset, or closed in another goroutine, `kernel.LoadFacts()` might panic with "database is closed" or mutate a state actively being cleared.
- This relies on `core.Kernel` implementation to be intrinsically thread-safe (which it typically is, but the JIT loop expects atomic boundaries).

**5.3 Parallel Store Search Deadlocks/Timeouts**
In `ClassifyWithoutInjection`:
```go
	if cfg.EnableParallel {
		g, gctx := errgroup.WithContext(ctx)
		g.Go(func() error { ... embeddedStore.Search(...) })
		g.Go(func() error { ... learnedStore.Search(...) })
		g.Wait()
	}
```
- `errgroup` waits for both goroutines to finish.
- If the `learnedStore` (SQLite backend) locks up due to a busy lock (SQLITE_BUSY), the `Wait()` will block indefinitely unless the upstream `ctx` has a hard timeout.
- *Performance:* If vector search over SQLite blocks, the semantic classification hangs. The context timeout is critical here, but is not enforced locally inside `SemanticClassifier`.

### 6. Summary of Discovered Gaps
1. **Empty/Whitespace string panic/NaN risk**: No early exit for empty strings; risk of zero-vector cosine similarity NaN errors.
2. **Negative TopK panic**: `mergeResults` uses raw `cfg.TopK` to slice arrays, risking panic on negative numbers.
3. **Massive input exhaustion**: No input truncation before embedding generation, causing latency spikes and upstream API rejections.
4. **Idempotency failure in LoadFromKernel**: Calling `LoadFromKernel` repeatedly appends to `s.entries` without clearing, causing OOM memory leaks and search degradation.
5. **Atom format validation**: `LearnedCorpusStore.Add` does not validate that `verb` is a properly formatted Mangle Atom (e.g., starting with `/`).

### 7. Performance Verdict
Overall, the `SemanticClassifier` demonstrates a solid foundation for local caching and parallel execution. The use of `sync.RWMutex` prevents read-stalls during classification. However, the system is **not performant** against unconstrained input boundaries (massive strings) and suffers a critical O(N) memory leak in its initialization idempotency (`LoadFromKernel`). Fixing the `LoadFromKernel` unbounded append and adding defensive input truncation will harden this subsystem to enterprise-grade reliability.

*(End of Journal Entry)*

### 8. Null and Undefined - Detailed Breakdown
* 8.1 - Empty query embeddings
When a user intent is evaluated and the input string is `""`, the `Embed` method call to an external provider (like OpenAI or Gemini) could return an empty slice, `[]float32{}`. If `CosineSimilarity` attempts to calculate distance on a zero-length slice, a panic (index out of bounds) or division by zero resulting in `NaN` may occur.

* 8.2 - Missing Configuration Fields
The `DefaultSemanticConfig` provides safe fallbacks, but if `SemanticConfig` is dynamically updated via `SetConfig` to zero values, such as `MinSimilarity = 0.0`, all similarities are accepted. If `TopK = 0`, the default fallback is applied `if topK <= 0 { topK = 5 }` locally in the search, but globally `TopK` stays 0. In `mergeResults`, `maxResults := cfg.TopK * 2` results in 0. The slice `deduped[:maxResults]` results in `[]`, causing the system to return zero matches, an obscure failure mode that should be rejected at `SetConfig`.

* 8.3 - Nil Context Handling
If the caller passes a `nil` `context.Context` to `Classify(nil, "fix bug")`, the `errgroup.WithContext(ctx)` will panic because `errgroup` uses `ctx.Done()`.

### 9. Type Coercion - Detailed Breakdown
* 9.1 - Type Asserts in Mangle Fact Parsing
In `EmbeddedCorpusStore.LoadFromKernel`, the system parses kernel facts using `argToString(f.Args[0])`. This utility function safely handles `string`, `core.MangleAtom`, and `fmt.Stringer`. However, if the Mangle kernel schema for `intent_definition` is updated to include integer IDs, an unhandled type might fallback to `fmt.Sprintf("%v", v)`. This string representation might not be what the embedding engine expects for text embedding.

* 9.2 - Similarity Float Precision Loss
`match.Similarity * 100` converts a `float64` to `int64`. A similarity of 0.8543 becomes `85`. This precision loss is intended but creates non-deterministic tie-breakers in the Mangle rules if multiple matches score `85` but had distinct `float64` values originally. Sorting in `mergeResults` resolves this, but the asserted facts lose the precise data.

* 9.3 - Integer Overflow Risk
If a user specifies a massive negative `TopK` that overflows `int` bounds, `mergeResults` could panic on out of bounds indexing.

### 10. User Request Extremes - Detailed Breakdown
* 10.1 - Extreme Context Windows
In brownfield projects (50M line monorepos), the user might ask "summarize all the files". If a tool accidentally dumps file contents directly into the intent classification layer, the system must process hundreds of megabytes. This blocks the main event loop and consumes unbounded memory.
The fix is enforcing an `Input Cap` at the perimeter before any string enters `SemanticClassifier`. A limit of 4096 bytes is more than sufficient for intent classification.

* 10.2 - Adversarial Prompts
A user prompt containing repeating recursive patterns, like "review review review review...", can degrade vector similarity results by shifting the embedding center of mass. While not a strict software crash, it's a semantic boundary failure where the system correctly classifies "noise" as the dominant intent.

* 10.3 - Extremely High Token Count
If the user pastes a base64 encoded image string as their intent, the system will attempt to embed it. The embedding provider might reject the input, or generate an embedding that matches nothing. The latency cost is high.

### 11. State Conflicts - Detailed Breakdown
* 11.1 - LearnedCorpusStore Add vs Search Race
The `LearnedCorpusStore` uses `mu.RLock()` for `Search` and `mu.Lock()` for `Add` (when using the in-memory fallback). However, if a DB backend is configured (`s.backend != nil`), `s.backend.AddPattern` is called with a read lock `s.mu.RLock()`.
```go
	s.mu.RLock()
	backend := s.backend
	dims := s.dimensions
	s.mu.RUnlock()

	if backend != nil {
		return backend.AddPattern(...)
	}
```
This implies the SQLite database must handle concurrent writes safely. While SQLite WAL mode supports this, `AddPattern` from multiple concurrent Autopoiesis loops could cause `SQLITE_BUSY` errors if the underlying driver doesn't implement a busy timeout retry loop.

* 11.2 - Kernel Hydration TOCTOU (Time-of-Check to Time-of-Use)
In `LoadFromKernel`, facts are queried: `facts, err := kernel.Query("intent_definition")`. Between checking the facts and generating embeddings for them, the underlying kernel state could change (e.g., facts retracted). The system generates embeddings for stale facts.

### 12. Verification and Mitigation Strategy
To address these discovered boundary conditions, the following code changes should be prioritized:

1. **Defensive Input Truncation:** Implement a strict character limit (e.g., 2048 chars) at the entry of `Classify` and `ClassifyWithoutInjection`.
2. **Empty String Short-Circuit:** Immediately return `nil, nil` if the input is empty or entirely whitespace to prevent NaN errors and wasteful API calls.
3. **Idempotency in LoadFromKernel:** Before iterating `entries` to append, verify that the store is not already hydrated, or explicitly clear `s.entries` and `s.embeddings`.
4. **Configuration Validation:** Add validation inside `SetConfig` to reject `TopK <= 0` or negative `MinSimilarity` values.
5. **Context Validation:** Ensure `ctx != nil` before initializing `errgroup`.

### 13. Deep Architectural Reflection
The decision to decouple the static (`EmbeddedCorpusStore`) and dynamic (`LearnedCorpusStore`) data stores is architecturally sound, providing clear failure domains. However, the vector search logic expects clean inputs.

When embedding engines fail, the system falls back gracefully, which is a strong positive trait. The true danger lies in resource exhaustion from unbounded input strings and the silent memory leak in `LoadFromKernel`.

By addressing the edge cases detailed in this BVA analysis, the `SemanticClassifier` will achieve a significantly higher degree of robustness, ensuring stable intent routing even under adversarial or extreme loads common in complex software development environments.

### 14. Performance Characteristics Summarized
| Vector Category | Performance | Stability | Risk |
|---|---|---|---|
| Null/Empty Strings | Fast (O(1) checks) but potentially fatal if NaN propagated | Low | High |
| Missing Configs | Fast | Low (Panic risk on negative slice index) | High |
| Huge Strings | Very Slow (Network bound, high memory) | Medium | Medium |
| State/Race | Slow (Memory leak causes O(N) degradation) | Low | High |

### 15. The "Void" Vectors Expanded
* 15.1 - The "Nil" Classifier Instance
If a developer accidentally sets the `SharedSemanticClassifier = nil` and attempts to call `SharedSemanticClassifier.Classify()`, it's a straightforward nil pointer panic. However, this is more likely during concurrent initialization and destruction. In `InitSemanticClassifier`, a `sync.Mutex` protects initialization, but what if `CloseSemanticClassifier` runs concurrently? The `SharedSemanticClassifier` could be accessed and set to `nil` immediately after initialization.

* 15.2 - The "Nil" Embedding Engine During Boot
If the system starts offline (e.g., local Ollama is down), `embedEngine` becomes `nil`. The `SemanticClassifier` continues in a degraded mode (regex fallback). This works as expected, but what if the engine comes back online? There is no recovery loop to re-initialize `embedEngine` once the network becomes available. The `SemanticClassifier` remains stuck in degraded mode until the application restarts.

* 15.3 - Zero Confidence Patterns
In `LearnedCorpusStore.Add`, what if the confidence is set to `0.0` or even `-1.0`? The system currently accepts it. If the pattern is retrieved, its similarity might be high, but its original confidence was explicitly negative. The `Confidence` field in `CorpusEntry` is currently ignored during `Search`, relying only on `Similarity`. This means a learned pattern with `-1.0` confidence but high embedding similarity would still be boosted by `LearnedBoost` and injected into the Mangle kernel!

### 16. Extreme Type Coercion Vectors Expanded
* 16.1 - `target` MangleAtom Injection
If a learned pattern maps to a target that isn't a valid atom (e.g., `target="hello world"`), the system currently injects it as a raw string into Mangle:
`semantic_match(UserInput, TextContent, /review, "hello world", ...)`
This means Mangle rules must expect a mix of `MangleAtom` and `string` types in the `target` field, which violates the strict typing principles documented in `mangle-programming/SKILL.md`.

* 16.2 - `Rank` Overflow
If `TopK` is configured to `2,147,483,647` (Max Int32), then `Rank` could overflow an `int` on 32-bit systems, or a smaller type during Mangle injection. The `Rank` is injected as `int64(match.Rank)`, but the variable itself is a Go `int`. This is a minor issue on modern 64-bit systems but still a coercion risk.

### 17. The Frontier Vectors (User Extremes) Expanded
* 17.1 - Multi-Language Intent Spanglish
If a user submits an intent combining English, Spanish, and Python code: `quiero que hagas refactor de def foo(): pass`.
The embedding engine will generate a multi-lingual semantic space vector. Does the `EmbeddedCorpusStore` (which currently seems to hold english phrases like "review my code") have sufficient semantic density to match cross-lingual input? If the engine uses an English-only tokenizer (like some fast local models), the similarity scores will drop to near zero, resulting in zero matches.

* 17.2 - ASCII Art and Emojis
If a user submits a massive ASCII art block or 5,000 emojis as their "intent", the `Classify` method will attempt to embed it.
For 5,000 emojis, the token count will skyrocket (often 1-3 tokens per emoji depending on the model). The similarity calculation on a 5,000 emoji vector against "fix a bug" will likely yield `0.0` or random noise, causing unhandled behavioral consequences and wasting token budgets.

* 17.3 - Extremely Complex Constraints
The `CorpusEntry` has a `Constraint` field. If Autopoiesis learns a pattern with a massive JSON string as the `Constraint`, and later matches it, what happens? The system doesn't inject `Constraint` into the `semantic_match` Mangle fact:
`// semantic_match(UserInput, CanonicalSentence, Verb, Target, Rank, Similarity)`
The `Constraint` data is completely discarded at the Mangle boundary!

### 18. Concurrency and State Conflicts Expanded
* 18.1 - `mergeResults` Concurrency
`mergeResults` takes `embedded` and `learned` slices by value (or rather, their headers by value). It mutates the `learned` slice directly:
```go
	for i := range learned {
		learned[i].Similarity += cfg.LearnedBoost
//...
```
If `learned` was returned directly from the in-memory `LearnedCorpusStore.Search`, and that search returned references to internal slice elements, mutating `Similarity` would corrupt the store! Fortunately, `Search` creates a new slice and copies the `SemanticMatch` structs, so mutating `learned[i]` is safe. This is a critical architectural success that must be maintained.

* 18.2 - `kernel.LoadFacts` and `kernel.Assert` Error Handling Race
If `kernel.LoadFacts(facts)` fails, the system falls back to a loop of `kernel.Assert(fact)`.
```go
	if err := kernel.LoadFacts(facts); err != nil {
		injectedCount := 0
		for _, fact := range facts {
			if err := kernel.Assert(fact); err != nil {
//...
```
If `LoadFacts` failed because the kernel is locked or in an invalid state, the subsequent loop will rapidly spam errors into the logs for each fact. A better design would check the specific error returned by `LoadFacts` and only fall back if it was a batch-specific error, rather than a catastrophic kernel failure (like "database is closed").

### 19. Final Recommendations
1. Validate `Confidence` scores during `LearnedCorpusStore.Add`.
2. Ensure `Target` inputs are valid Mangle Atoms or enforce a strict string-only policy in Mangle rules.
3. Pass `Constraint` data into the `semantic_match` fact so the kernel can actually use the learned constraints!
4. Add robust panic recovery `defer func() { recover() }()` inside the `errgroup` goroutines to prevent a single panicked store search from taking down the entire intent classification pipeline.

By addressing these granular, extreme boundary cases, the codeNERD intent classification pipeline will become significantly more resilient to the chaotic realities of real-world AI agent usage.

### 20. Expanded BVA: Mangle Integration Layer Vectors

* 20.1 - The "Semantic Match" ID Collision Vector
When injecting facts into the kernel, `semantic_match` is a generic predicate. It contains `UserInput`, `TextContent`, `Verb`, `Target`, `Rank`, and `Similarity`. What if a session is kept alive and two different inputs are classified? The system injects `semantic_match("fix a bug", ...)` and then `semantic_match("check code", ...)`.
Unless the `SemanticClassifier` retracts older `semantic_match` facts before classification, the kernel accumulates them. If the rules don't bind to a specific `UserInput` (like `current_user_input(X)`), Mangle will attempt to join across all historical semantic matches! This state accumulation vector causes "ghost facts" from previous queries to pollute new queries, breaking the clean loop philosophy.

* 20.2 - Float `Similarity` Mangle Rounding
The system scales `float64` to `0-100` integers. A similarity of `0.999` becomes `99`. A similarity of `0.991` also becomes `99`. If Mangle relies on precise sorting of results, this coarse quantization destroys the fine-grained vector distances generated by advanced models. While `mergeResults` locally deduplicates and assigns ranks, any Mangle rule relying purely on the similarity integer might nondeterministically prefer lower-ranked items if they share the same quantized score.

* 20.3 - Injected Fact Cardinality Explosions
`cfg.TopK` determines how many facts are injected into Mangle. If `TopK` is mistakenly set to `500` through a bad config, the classifier will inject 500 `semantic_match` facts per query. If a Mangle rule joins `semantic_match` with another large table (like `files` or `tools`), the cross-product evaluation could cause an exponential explosion in engine evaluation time (e.g., O(N^2) or O(N^3)). The system should hard-cap the injected facts, regardless of user config.

* 20.4 - The `Target` Type Discrepancy Vector
In `EmbeddedCorpusStore.LoadFromKernel`, the system maps the third argument of `intent_definition` to the `Target` string.
```go
		if len(f.Args) > 2 {
			target = argToString(f.Args[2])
		}
```
If `intent_definition` defines `/codebase` as an Atom, `argToString` converts it to `"/codebase"`. Later, during fact injection:
```go
			Args: []interface{}{
				input,
				match.TextContent,
				core.MangleAtom(match.Verb), // Verb is converted back to Atom
				match.Target,                // Target remains a string!
```
This is a critical type dissonance! The `Target` was an Atom in the kernel (`/codebase`), converted to a string `"/codebase"`, and then injected back into the kernel as a string `"/codebase"`. Mangle treats `/codebase` (Atom) and `"/codebase"` (String) as two entirely disjoint types. Any downstream rule joining `semantic_match(_, _, _, Target, _, _)` with `resource(Target)` (where resource expects an Atom) will silently fail to find any matches! This state conflict between Go and Mangle typing is a massive blind spot.

* 20.5 - Empty Target Defaults
If the `Target` is not provided in `intent_definition`, it defaults to `""`. When injected into Mangle as an empty string, does the receiving rule handle `""` safely? Some rules might expect a valid identifier and fail when processing `""`.

### 21. Advanced Stress Testing Scenarios

* 21.1 - The "Rapid Fire" Async Vector
If the frontend sends 10 classification requests simultaneously (e.g., from an aggressive debounce in a type-ahead search feature):
- 10 goroutines call `Classify`.
- 10 embedding API requests are fired (potential rate limiting, HTTP 429).
- 10 `s.mu.RLock()` calls on the stores.
- 10 batch injections into the Mangle kernel `kernel.LoadFacts`.
This race condition on `kernel.LoadFacts` could cause `database is locked` SQLite panics if the kernel isn't fully transactional or uses a single-writer pattern without robust queuing.

* 21.2 - The Missing Embedded Corpus Vector
If `LoadFromKernel` fails silently due to a syntax error in Mangle rules, the `EmbeddedCorpusStore` remains empty. The user query will exclusively rely on `LearnedCorpusStore`. If that is also empty, `Classify` returns 0 matches. The application must handle this graceful degradation, perhaps by falling back to a pure LLM transducer without vector search.

* 21.3 - The `NaN` Propagation Vector Re-visited
If an embedding engine returns vectors of `[0.0, 0.0, ..., 0.0]`, the `CosineSimilarity` calculation `dot / (magA * magB)` will result in a division by zero. Go floats handle this by returning `NaN`.
- `NaN > 0.5` is false.
- `NaN < 0.5` is false.
- `NaN == NaN` is false.
If `NaN` leaks into the `sort.Slice` comparison function `candidates[i].similarity > candidates[j].similarity`, the sort becomes completely unstable and nondeterministic. The `SemanticClassifier` must validate vector magnitudes before computing similarity.

### 22. Architectural Remediation Plan

To address the profound architectural flaws discovered in this extended BVA:

1. **Type Dissonance Fix:** The `Target` field MUST be typed correctly when asserted back into Mangle. If the original source was an Atom, it must be asserted as a `core.MangleAtom`. A struct like `TargetIsAtom bool` inside `CorpusEntry` may be necessary.
2. **Fact Retraction:** The `Classify` method MUST retract previous `semantic_match` facts before asserting new ones to prevent ghost fact accumulation.
3. **Zero-Vector Guards:** Implement explicit checks in `CosineSimilarity` or before it to reject zero-magnitude vectors, preventing `NaN` pollution in the sort algorithms.
4. **Hard Limits:** Introduce a hard-coded maximum `TopK` limit (e.g., `MaxTopK = 10`) inside `mergeResults` to prevent Mangle cardinality explosions regardless of user config.
5. **Rate Limiting:** If classification is exposed to a rapidly-updating UI, implement debounce or request cancellation via `context.Context` to prevent API rate limiting.

### 23. Conclusion
This extended boundary value analysis confirms that while the Go layer is resilient to nil pointers through standard defensive checks, the neuro-symbolic boundary (Go to Mangle) and the network boundary (Go to Embedding API) possess significant vulnerabilities to type dissonance, ghost state accumulation, and unconstrained input sizes. Resolving these issues is paramount for enterprise-level stability.

### 24. Final Analysis of Error Handling Strategies
* 24.1 - The "Silent Squelch" Vector
Currently, `SemanticClassifier` swallows many errors to support "graceful degradation".
```go
	if err := embeddedStore.LoadFromKernel(context.Background(), kernel, embedEngine); err != nil {
		logging.Get(logging.CategoryPerception).Warn("Failed to hydrate embedded intent corpus from kernel: %v", err)
	}
```
While this ensures the process doesn't crash, it hides systemic failures. If the `intent_definition` predicate is renamed in Mangle, the Go code silently continues with an empty `EmbeddedCorpusStore`. The user experiences terrible performance (falling back to pure LLM inference), but the tests pass because "nil" is valid.

* 24.2 - The "Orphaned Context" Vector
If `ctx.Err() != nil` is true, the `Classify` process returns the context error. However, if the `Context` is canceled *after* the embedding is retrieved but *before* `kernel.LoadFacts` runs, the facts are still asserted into the kernel, but the caller receives an error. The Mangle kernel now contains `semantic_match` facts for a cancelled operation! This is a severe state corruption vector.

* 24.3 - Memory Allocation Denial of Service
The `Search` method uses `make([]scored, 0, len(s.entries))` to allocate memory. If `s.entries` grows unbounded due to the `LoadFromKernel` idempotency bug (5.1), the memory allocation for every single search query also grows. A query hitting 1,000,000 entries will allocate a massive slice for `scored` candidates, compute 1,000,000 cosine similarities, and then sort them. This is an O(N) CPU and Memory allocation that will trigger the Go garbage collector aggressively.

### 25. The Absolute Boundary Checks (A checklist for the implementation)
- [ ] Are empty strings explicitly rejected `if strings.TrimSpace(input) == ""`?
- [ ] Is input length hard-capped (e.g., `if len(input) > 4096 { input = input[:4096] }`)?
- [ ] Does `LoadFromKernel` clear `s.entries` before appending?
- [ ] Is `cfg.TopK` validated to be `> 0` and `< MaxTopK`?
- [ ] Does `Target` preserve `core.MangleAtom` type if it was an atom originally?
- [ ] Are previous `semantic_match` facts retracted before `injectFacts`?
- [ ] Does `CosineSimilarity` return an error on zero-magnitude vectors?
- [ ] Is `ctx.Done()` checked immediately prior to `kernel.LoadFacts`?
- [ ] Does `AddLearnedPattern` validate that `confidence` is within `[0.0, 1.0]`?
- [ ] Does `AddLearnedPattern` validate that `verb` is a valid Mangle Atom?

### 26. Parting Thoughts on the Neuro-Symbolic Gap
The `SemanticClassifier` is an excellent example of bridging connectionist (vector) and symbolic (Mangle) AI. However, the exact point of translation—the `injectFacts` method—is where the system is most vulnerable. A strongly typed vector embedding must become a dynamically typed Mangle fact. The BVA reveals that "stringly-typed" translation (`argToString`) and lossy numerical conversion (`float64` to `int64`) are the primary failure modes.

By rigorously defining the edges of these types (e.g., "what happens if the target is an Atom? What happens if the similarity is exactly 0?"), codeNERD can prevent the subtle, cascading logical failures that plague most AI coding assistants. This journal entry serves as the blueprint for hardening those boundaries.

### 27. The Mangle Transduction "Type Coercion" Matrix

Here is a quick reference matrix of how Go types translate into the `SemanticClassifier` facts, highlighting the coercion boundaries and failure vectors:

| Go Source Type | `injectFacts` Destination | Mangle Schema Type | Conflict Risk Level |
|---|---|---|---|
| `string` (`input`) | `interface{}` -> `string` | String | **Low** - Simple passthrough. |
| `string` (`match.TextContent`) | `interface{}` -> `string` | String | **Low** - Simple passthrough. |
| `string` (`match.Verb`) | `core.MangleAtom` -> `Atom` | Atom | **High** - Must match `/\w+` format, else syntax error. |
| `string` (`match.Target`) | `interface{}` -> `string` | Atom or String | **CRITICAL** - Loss of Atom identity! `argToString` strips `/`. |
| `float64` (`match.Similarity`) | `int64(match.Similarity * 100)` | Integer | **High** - Quantization loss, potential overflow/underflow if `NaN`. |
| `int` (`match.Rank`) | `int64(match.Rank)` | Integer | **Low** - Safe upcast. |

The most dangerous coercion is clearly `Target`. The system assumes the target is just text (e.g., "codebase"), but often the target is a formal resource Atom (e.g., `/main.go`). If the target was originally an Atom in `intent_definition`, it is stripped of its type by `argToString` and injected as a regular string. This means any downstream Mangle rule that expects `semantic_match(_, _, _, /main.go, _, _)` will fail, because the system injected `semantic_match(_, _, _, "main.go", _, _)`.

### 28. Final BVA Conclusion
This intensive BVA session has exposed 10 distinct, critical failure vectors in the `SemanticClassifier` subsystem, ranging from unhandled `NaN` similarity scores and memory leaks in `LoadFromKernel` idempotency, to fundamental type dissonance in Mangle fact injection. The next step is translating these findings into explicit `TEST_GAP` comments in `internal/perception/semantic_classifier_test.go` to guide future hardening efforts.

*(End of Journal Entry Addendum)*

### 29. Final Reflection
The boundary value analysis technique has once again proven its worth. Code that appears logically sound on the "Happy Path" often hides systemic architectural flaws when pushed to its limits. The `SemanticClassifier` is performant for standard conversational inputs, but its failure to enforce strict input length limits and its silently expanding embedded corpus slice make it highly susceptible to unintentional resource exhaustion in long-lived or adversarial sessions.

### 30. Last Details
This concludes the 400-line QA Journal entry for SemanticClassifier. The next steps will involve creating specific unit tests and Mangle syntax checks to cover the identified `TEST_GAP` vectors.
