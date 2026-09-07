
# Quality Assurance Journal: Boundary Value Analysis and Negative Testing
## Subsystem: internal/usage/pricing.go
## Date: 2026-09-01_23-40-05_EST

### Executive Summary

As a QA Automation Engineer specializing in Boundary Value Analysis (BVA) and Negative Testing, this journal documents a deep, exhaustive architectural and testing review of the `internal/usage/pricing.go` subsystem within codenerd. This subsystem is a highly critical module responsible for resolving API models to their respective token pricing and calculating aggregate costs for large-scale language model inference requests across various providers (OpenAI, Anthropic, Google, etc.). Given its direct impact on financial reporting, token budgeting, and cost limits, it must be highly robust against non-standard inputs.

The current test suite (`internal/usage/pricing_test.go`) demonstrates a solid grasp of happy-path scenarios. It validates basic substring matching for model resolution, simple case normalization, and standard integer cost calculations for millions of tokens. However, applying a strict Negative Testing methodology reveals significant and potentially dangerous blind spots in the test coverage regarding Null/Empty cases, extreme Type Coercion boundaries (integer to float), User Request Extremes (denial of service vectors), and Concurrency/State Conflicts.

This document details these gaps exhaustively, proposes explicit test vectors for each, writes out the actual proposed test code to be implemented, and evaluates whether the current Go implementation in `pricing.go` is structurally performant enough to handle these edge cases or if architectural changes are fundamentally required to protect the codenerd platform.

---

### 1. Vector: Null/Undefined/Empty and Near-Empty Inputs

#### 1.1 Current State Analysis
The tests currently check `""` (empty string) as an input to `LookupPrice` and expect it to return `false` for finding a price. The `normalizeModelName` function explicitly checks for an empty string after calling `strings.TrimSpace`.

#### 1.2 Identified Edge Cases and Missing Coverage
- **Whitespace Variations and Trimming Deficiencies:** `normalizeModelName` calls `strings.TrimSpace`, but tests only verify `"  gpt-4o  "`. What about input strings composed entirely of various whitespace characters (tabs, newlines, zero-width spaces, non-breaking spaces)? Are there any whitespace characters in the Unicode standard that `strings.TrimSpace` misses which might bypass the empty string check?
  - Test Input: `"\t\n\r"`
  - Test Input: `"​"` (Zero-width space)
  - Test Input: `"\u00A0"` (Non-breaking space)
- **Null-Byte Injection:** Go strings can contain null bytes. If a model string from an external API payload includes `\x00`, does `normalizeModelName` handle it gracefully? While Go can handle null bytes in strings, operations like `strings.TrimSpace` might not strip them, and subsequent logging or SQL operations (like tracking costs in SQLite) might truncate the string at the null byte, leading to ghost data or index mismatches.
  - Test Input: `"gpt-4o\x00-mini"`
  - Test Input: `"\x00"`
- **Zero Token Counts and Semantic Inversions:** `EstimateCost` handles 0, but tests do not explicitly check negative counts (which should technically be impossible or rejected, but are valid `int64` inputs to the function signature).
  - Test Input: `EstimateCost("gpt-4o", -100, -500)` -> This will result in negative cost. Is this intended (e.g., for refunds or error compensation logic in codenerd), or a logic error?
  - Test Input: `EstimateCost("gpt-4o", 0, -1)` -> Asymmetric negative counts.

#### 1.3 Proposed Test Implementation
```go
// Proposed Test for Vector 1
func TestLookupPrice_ExtremeWhitespaceAndNull(t *testing.T) {
    t.Parallel()
    bizarreInputs := []string{
        "\t\n\r",
        "\u200B",
        "\u00A0",
        "\x00",
        "gpt-4o\x00-mini",
    }
    for _, input := range bizarreInputs {
        _, ok := LookupPrice(input)
        if ok && input != "gpt-4o\x00-mini" {
            t.Errorf("Expected whitespace/null to fail lookup: %q", input)
        }
    }
}

func TestEstimateCost_NegativeTokenSemantic(t *testing.T) {
    t.Parallel()
    // Mathematically this works, but semantically, does the system expect this?
    cost, ok := EstimateCost("gpt-4o", -1_000_000, 0)
    if !ok {
        t.Fatal("gpt-4o should be priced")
    }
    if cost >= 0 {
        t.Errorf("Expected negative token count to yield negative cost, got %f", cost)
    }
}
```

#### 1.4 Performance & System Capability Analysis
The `pricing.go` system is highly capable of handling these strings from a performance perspective. Go's string manipulation (`strings.TrimSpace`, `strings.ToLower`) is allocation-efficient for small strings. Null bytes are treated as standard characters in Go, so they won't cause C-style string termination crashes in the pricing logic itself.
Negative token counts will correctly output a negative float64. The system is performant, but semantically, allowing negative tokens might corrupt aggregate billing if not explicitly guarded at the caller level. The `pricing.go` module itself should arguably not enforce positive counts if it's meant to be a pure calculator, but the tests should verify the mathematical correctness of negative inputs.

---

### 2. Vector: Type Coercion, Numeric Boundaries, and Max Int

#### 2.1 Current State Analysis
Token counts are passed as `int64` to `Cost(input, output int64) float64`. Tests check basic ranges like `1_000_000`.

#### 2.2 Identified Edge Cases and Missing Coverage
- **Integer Overflow to Float Precision Loss:** A 64-bit integer can hold up to `9,223,372,036,854,775,807`. When passed to `Cost`, these integers are cast to `float64`. Float64 only has 53 bits of precision, meaning `int64` values above ~`9 x 10^15` will lose precision during the cast.
  - Test Input: `math.MaxInt64` tokens for input.
  - Test Input: `math.MaxInt64` tokens for output.
- **NaN / Infinity Propagation:** What happens if `math.MaxInt64` is multiplied by an extremely high price? Does it result in `+Inf`? Does the calling tracking system gracefully handle `+Inf` costs?
  - Test Input: `Cost(math.MaxInt64, math.MaxInt64)`
  - Test Input: `Cost(math.MinInt64, math.MinInt64)` -> `-Inf`?
- **Extreme Fractional Token Calculations (Underflow):** If token counts are very small (e.g., 1 token) and the price is very small (e.g., $0.075 per 1M), the cost is $0.000000075. Does the float64 representation suffer from underflow or rounding errors if summed millions of times in the tracking layer?
  - Test Input: `Cost(1, 0)` with the cheapest model.
  - Test Input: `Cost(0, 1)` with the cheapest model.

#### 2.3 Proposed Test Implementation
```go
// Proposed Test for Vector 2
func TestPrice_Cost_MaxIntOverflow(t *testing.T) {
    t.Parallel()
    p := Price{InputPerMTok: 30.00, OutputPerMTok: 60.00} // Expensive model

    // Testing precision loss bounds
    cost1 := p.Cost(math.MaxInt64, 0)
    cost2 := p.Cost(math.MaxInt64-100, 0) // May equate to cost1 due to 53-bit mantissa

    if math.IsInf(cost1, 0) {
        t.Log("Warning: MaxInt64 tokens results in Infinity cost.")
    }

    // Testing underflow limits
    cheap := Price{InputPerMTok: 0.02}
    microCost := cheap.Cost(1, 0)
    if microCost == 0 {
        t.Error("Cost underflowed to zero")
    }
}
```

#### 2.4 Performance & System Capability Analysis
The `Cost` function is a single mathematical expression: `(float64(input)/1e6)*p.InputPerMTok + (float64(output)/1e6)*p.OutputPerMTok`. It executes in ~1ns. It is exceptionally performant. However, the system's *precision* capability is bounded by IEEE-754 float64 rules. For typical LLM usage (even billions of tokens), float64 precision is completely adequate to track USD values accurately. The boundary case of `MaxInt64` losing precision is a known architectural trade-off of mixing int64 counters with float64 currencies, but it's unlikely to be hit in reality without malicious input injection into the token counters. Testing these bounds ensures the system doesn't panic on overflow.

---

### 3. Vector: User Request Extremes and Malformed Input

#### 3.1 Current State Analysis
`normalizeModelName` tries to handle provider prefixes (e.g., `"openai/"`, `"anthropic."`).

#### 3.2 Identified Edge Cases and Missing Coverage
- **Extreme Length Strings (DoS Vector):** A user might attempt a Denial of Service by sending an incredibly long string (e.g., a 100MB string) as the model name.
  - Test Input: `strings.Repeat("a", 10_000_000)`
- **Deep Path Traversal / Provider Spoofing:** `normalizeModelName` uses `LastIndex(name, "/")`. What if the name is an absolute path or has massive depth?
  - Test Input: `"/openai/anthropic.google./bedrock./gpt-4o"`
  - Test Input: `strings.Repeat("a/", 1000) + "gpt-4o"`
- **Unicode Normalization & Case Folding:** `strings.ToLower` in Go handles basic casing, but does not handle full Unicode case folding (e.g., Turkish 'i', or certain ligatures). If an engine sends a bizarre Unicode representation of "GPT-4o", it will fail to match the table.
  - Test Input: `"ſ"` (Latin small letter long s, which lowercases to 's', but might not match literal 's' if not fully folded).
  - Test Input: `"GPT-4Ö"` (O with diaeresis).
- **Missing Version Suffix Edge Cases:** The code strips `...-v1:0` bedrock-style version suffixes using `strings.Index(name, ":")`. What if the colon is the first character?
  - Test Input: `":gpt-4o"`
  - Test Input: `":"`

#### 3.3 Proposed Test Implementation
```go
// Proposed Test for Vector 3
func TestNormalizeModelName_DenialOfServiceVectors(t *testing.T) {
    t.Parallel()

    // Memory and CPU exhaustion vector
    if !testing.Short() {
        hugeString := strings.Repeat("a", 10_000_000) // 10MB
        got := normalizeModelName(hugeString)
        if len(got) == 0 {
            t.Error("Expected to process large string, but got empty")
        }
    }

    // Path traversal vector
    traversal := strings.Repeat("a/", 1000) + "gpt-4o"
    if got := normalizeModelName(traversal); got != "gpt-4o" {
        t.Errorf("Expected gpt-4o, got %q", got)
    }

    // Weird edge cases
    if got := normalizeModelName(":"); got != "" {
        t.Errorf("Expected empty string for ':', got %q", got)
    }
}
```

#### 3.4 Performance & System Capability Analysis
This is where the system has a potential vulnerability. If `normalizeModelName` is fed a massive 100MB string, `strings.ToLower` will allocate a new 100MB string. `strings.TrimSpace` might allocate. `strings.LastIndex` will scan the 100MB string. While Go's garbage collector is fast, doing this concurrently on a server could cause a quick Out-Of-Memory (OOM) panic. The system needs a strict, early length bound on model names (e.g., rejecting any name > 256 characters) before performing string allocations. The string scanning operations (`Index`, `LastIndex`) are O(N) relative to string length, making extreme lengths a CPU exhaustion vector as well.

---

### 4. Vector: State Conflicts, Race Conditions, and Concurrency

#### 4.1 Current State Analysis
The system uses a `sync.RWMutex` (`priceMu`) to protect `priceTable` and `priceKeysMemo` during `LookupPrice` and `RegisterPrice`.

#### 4.2 Identified Edge Cases and Missing Coverage
- **Concurrent Registration and Lookup:** `RegisterPrice` clears `priceKeysMemo` and rebuilds the `priceTable`. If a million goroutines are calling `LookupPrice` while another goroutine spam-calls `RegisterPrice`, does the system stall due to lock contention?
- **Memoization Race Condition:** The `priceKeys` function has a double-checked locking pattern. The tests do not run `RegisterPrice` and `LookupPrice` in parallel using `t.Run` and goroutines to explicitly trigger race detection (`-race`) on this specific memoization logic.
- **Map iteration non-determinism during sorting:** When `priceKeys` rebuilds the keys, it iterates over `priceTable` map. Go map iteration is intentionally non-deterministic. The subsequent `sort.Slice` uses a stable comparison, but if two prefixes have the exact same length and alphabetical ordering (impossible for identical string keys, but theoretically relevant if the sort logic changes), the order could be unstable. The current sort logic is `len(keys[i]) > len(keys[j])` followed by alphabetical. This is safe, but untested under concurrent mutation.

#### 4.3 Proposed Test Implementation
```go
// Proposed Test for Vector 4
func TestPricing_ConcurrencyStress(t *testing.T) {
    // Do not run parallel as it mutates global state, but use internal concurrency
    var wg sync.WaitGroup

    // 10 writers rapidly registering prices
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            for j := 0; j < 1000; j++ {
                modelName := fmt.Sprintf("dynamic-model-%d-%d", id, j)
                RegisterPrice(modelName, Price{InputPerMTok: 1.0, OutputPerMTok: 2.0})
            }
        }(i)
    }

    // 100 readers rapidly looking up prices
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for j := 0; j < 1000; j++ {
                // Read both known and unknown models to trigger memoization
                LookupPrice("gpt-4o")
                LookupPrice("dynamic-model-0-500")
            }
        }()
    }

    wg.Wait()
}
```

#### 4.4 Performance & System Capability Analysis
The read-write mutex pattern is generally performant for read-heavy workloads (which pricing lookups are). However, `RegisterPrice` takes an exclusive write lock (`priceMu.Lock()`) and triggers a slice rebuild (`priceKeysMemo = nil`, then the next read rebuilds the slice and sorts it). Sorting strings inside a global mutex lock is an anti-pattern for extreme high-concurrency systems. If `RegisterPrice` is called frequently (e.g., dynamic bidding for spot-instance LLMs), the `sort.Slice` operation will block all cost estimations globally. The system is performant for *static* prices but fundamentally unsuited for high-frequency dynamic price updates.

---

### Deep Architectural Impact Analysis

The `internal/usage/pricing.go` module is a fundamental dependency for the `usage_tracker.go`. Any performance degradation or panic in `LookupPrice` directly halts all usage tracking, which in turn could fail API requests if token budgets cannot be verified.

**1. The `normalizeModelName` Allocation Trap:**
The current implementation of `normalizeModelName` is:
```go
func normalizeModelName(model string) string {
	name := strings.ToLower(strings.TrimSpace(model))
    // ...
}
```
This eagerly allocates a new string. In a highly distributed environment where `LookupPrice` is called per-event, this generates significant garbage. For standard strings (10-30 bytes), this is fine. For our malicious 100MB string vector, this is fatal.
*Recommendation:* Implement an upfront length check:
```go
if len(model) > 256 {
    return "" // Or handle as an error
}
```

**2. The Double-Checked Locking in `priceKeys`:**
```go
func priceKeys() []string {
	priceMu.RLock()
	if priceKeysMemo != nil {
		defer priceMu.RUnlock()
		return priceKeysMemo
	}
	priceMu.RUnlock()
    // ...
}
```
While technically correct in Go, the `defer priceMu.RUnlock()` inside the `if` block forces the defer to be evaluated on return. In highly performance-sensitive paths, manual `Unlock()` before return avoids the slight defer overhead. However, the bigger issue is that if 1000 goroutines miss the read lock simultaneously because `RegisterPrice` was just called, they will all queue up on `priceMu.Lock()`. The first one rebuilds the slice. The remaining 999 will acquire the write lock sequentially, see `priceKeysMemo != nil`, and return. This causes a massive latency spike ("thundering herd" on the lock).
*Recommendation:* This is acceptable for rare `RegisterPrice` calls, but if dynamic pricing is expected, a lock-free structure (like `atomic.Pointer` wrapping an immutable struct containing both the map and the sorted keys) would be vastly superior and immune to this latency spike.

**3. Precision Loss in Accumulators:**
While `Cost` handles the math correctly, the `usage_tracker.go` adds these float64 values repeatedly:
```go
stats.TotalProject.Cost += eventCost
```
Adding millions of tiny float64 values can lead to precision loss (catastrophic cancellation or absorption) where `large_float + tiny_float == large_float`.
*Recommendation:* While testing `Cost` boundaries is important, the architectural fix for precision loss in financial systems is typically integer arithmetic (e.g., tracking cost in micro-cents). For this module's stated purpose ("order-of-magnitude answer to 'what did this session cost', not an invoice"), float64 is acceptable, but this assumption must be explicitly verified in the tests via boundary values.

---

### Conclusion and Final Quality Assurance Sign-Off

The `internal/usage/pricing.go` module is logically sound for standard operations but lacks test coverage and defensive programming against extreme boundary values and negative inputs.

To harden this system, the following actions must be taken by the engineering team:
1. Implement test cases for negative token counts and evaluate if `EstimateCost` should panic, clamp to zero, or return an error for negative inputs.
2. Implement test cases for massive token counts (`math.MaxInt64`) to document expected precision loss behavior.
3. Add a strict length bound check at the beginning of `normalizeModelName` to prevent allocation-based DOS attacks from malicious extreme-length model strings.
4. Add a concurrent stress test that runs `RegisterPrice` and `LookupPrice` in tight parallel loops with the `-race` flag enabled to guarantee the memoization lock holds under extreme contention.
5. Add test cases for whitespace-only strings and null-byte injection to ensure `normalizeModelName` behaves predictably.
6. Add test cases for extreme path traversal spoofing (e.g., `strings.Repeat("a/", 1000) + "gpt-4o"`).

### Addendum: Mangle System Evaluation Context

To fully evaluate the codenerd context based on `.claude/skills/stress-tester/SKILL.md` and related documentation, we must also consider how these pricing failures map into the larger Mangle evaluation engine context.

If `pricing.go` were to fail catastrophically (e.g., an OOM from the `normalizeModelName` vector), it would register as a `Level 0: SILENT FAIL - Invalid state causes undefined behavior (BUG)` according to the Mangle self-healing hierarchy.

To improve this:
- **Level 3: DETECTED:** We should add internal assertions (self-checks) validating that strings do not exceed length limits before processing.
- **Level 4: PREVENTED:** The inputs from Mangle facts `usage_event(T, Model, Cost)` should have schema-level type enforcement where `Model` string length is constrained by the Mangle JIT compiler prior to Go execution.
- **Level 5: IMPOSSIBLE:** If we can bind models directly to enums rather than unbounded strings, the string parsing problem disappears entirely. However, dynamic routing via `RegisterPrice` requires string matching.

These recommendations represent the shift from reactive band-aid bug fixing to architectural root-cause remediation demanded by the codenerd engineering standards.

### Addendum 2: Mangle Fact Database Integration and Billing Ghost States

In the codenerd ecosystem, Mangle acts as a declarative fixpoint engine that evaluates monotonic facts. The interaction between the imperative `pricing.go` system and the declarative Mangle fact store introduces subtle boundary value vulnerabilities that are not immediately obvious when unit testing the Go code in isolation.

#### The "Ghost Fact" Vulnerability
Mangle's evaluation is monotonic and stateful. Reusing a store across tests or sessions leads to "ghost facts" from previous runs contaminating the current fixpoint. When `usage_tracker.go` writes cost data based on `EstimateCost`, it implicitly relies on `pricing.go` returning deterministic results.

Consider a scenario where `RegisterPrice` is called mid-session, mutating the `priceTable`.
1. Fact 1 is asserted: `usage_event(session1, "gpt-4o", 100, 100)`. `pricing.go` evaluates this at $2.50 / $10.00.
2. `RegisterPrice("gpt-4o", Price{0,0})` is called, zeroing the cost due to an administrative override.
3. Fact 2 is asserted: `usage_event(session1, "gpt-4o", 50, 50)`. `pricing.go` evaluates this at $0.00.

In an imperative system, this is a standard temporal state change. In Mangle's monotonic logic, if rules are written to aggregate costs over a session (`total_cost(S, Sum) :- ...`), the engine might re-evaluate or join these facts in unpredictable orders. If the cost calculation is pushed down into a Mangle rule that calls a Go function bound to `pricing.go`, the changing underlying state violates Mangle's pure function assumptions.

**Boundary Test Requirement:**
Tests must verify the behavior of `RegisterPrice` when interfaced with Mangle. The factstore must be reinstantiated cleanly (`factstore.NewSimpleInMemoryStore()`) inside the test loop to ensure idempotency. The tests must prove that a `RegisterPrice` mutation does not retroactively alter the evaluation of already asserted usage facts.

#### Type-Strict AST Dissonance (Atom vs String)
Mangle treats `/gpt-4o` (an Atom) and `"gpt-4o"` (a String) as disjoint types. `pricing.go` operates exclusively on Go strings. If the usage tracking system serializes Mangle Atoms into Go strings and passes them to `LookupPrice`, there is a boundary where type coercion errors can occur.

If an operator writes a Mangle rule:
```mangle
track_usage(Session, /gpt-4o, Tokens).
```
And the Go integration layer passes `"/gpt-4o"` (literal string containing a forward slash) to `LookupPrice`, the `normalizeModelName` function will strip everything before the last slash.
```go
if i := strings.LastIndex(name, "/"); i >= 0 {
    name = name[i+1:]
}
```
`"/gpt-4o"` becomes `"gpt-4o"`, and it miraculously works!

However, if the Mangle rule used:
```mangle
track_usage(Session, "gpt-4o", Tokens).
```
This is a Mangle String, and it also works. This accidental correctness masks a severe boundary flaw. If Mangle introduces a new type, or if a user accidentally creates a composite Atom like `/providers/openai/gpt-4o`, `normalizeModelName` will strip `/providers/openai/` and match `gpt-4o`. This seems helpful, but it means the pricing module is guessing the intent of the Mangle engine rather than strictly validating types.

**Boundary Test Requirement:**
The test suite must explicitly assert how `normalizeModelName` reacts to Mangle AST serializations. It must be tested with inputs generated by `ast.Name("gpt-4o").String()` and `ast.String("gpt-4o").String()` to guarantee that the normalizer doesn't accidentally mangle (pun intended) the type indicators into invalid model names, or worse, hallucinate a match.

#### The "Forgotten Sender" and Context Cancellation
The `pricing_test.go` file lacks tests that simulate extreme system load where context cancellation is rapid. In `usage_tracker.go`, contexts are used heavily (`WithShardContext`). If a Mangle evaluation fails due to a stratification error (e.g., negation cycles like `p :- not p.`), the context is immediately cancelled.

If `RegisterPrice` is holding `priceMu.Lock()` and a massive garbage collection pause occurs, hundreds of usage tracking events might queue up on `priceMu.RLock()`. If their contexts cancel while waiting, the Go standard library `sync.RWMutex` does not support context-aware locking. The goroutines will block indefinitely until the write lock is released, even though the request is dead. This is a goroutine leak vector.

**Boundary Test Requirement:**
A stress test must simulate high-frequency `LookupPrice` calls with contexts that cancel after 1 nanosecond, while a slow `RegisterPrice` holds the write lock. We must use `goleak.VerifyNone(t)` to ensure that no goroutines are permanently blocked waiting for the `RWMutex` after their contexts have expired. This proves the system is resilient against the "Forgotten Sender" failure mode.

#### Golden File Testing for Aggregates
For complex aggregate calculations spanning multiple models and shards (as tested in `TestTrack_ShouldAccumulateCost`), hardcoding float64 assertions is brittle and ignores precision drift across architectures.

**Boundary Test Requirement:**
Implement Golden File testing for the output of `tr.Stats()`. Serialize the `Stats` struct to JSON after processing millions of boundary-value events (negative tokens, unpriced models, MaxInt limits) and compare it against a `.golden` file. This guarantees that complex derivation limits and rounding behaviors remain perfectly stable across refactors, catching subtle regressions in how `float64` boundaries interact.

### Addendum 3: Extended Performance Metrics and Threat Modeling

This section extends the architectural threat model for `pricing.go`, specifically focusing on how extreme boundary values impact the host environment (RAM, CPU, Goroutine limits).

#### CPU Exhaustion via Pathological Strings
The `normalizeModelName` function employs `strings.ToLower` and `strings.TrimSpace`. While Go's standard library is optimized, it relies on Unicode properties. A malicious actor could provide a string composed of millions of Unicode characters that require complex case-folding (e.g., the German double-s 'ß' which folds to 'ss', altering string length).

**Attack Vector:**
If codenerd exposes an API where users can specify their own custom model strings for bring-your-own-LLM features, they could inject a 10MB string of 'ß' characters.
1. `strings.TrimSpace` scans the 10MB string (O(N)).
2. `strings.ToLower` scans the string. Because 'ß' expands to 'ss', it cannot do an in-place modification or simple copy. It must allocate a 20MB buffer and perform complex Unicode boundary checks for every character.
3. The resulting 20MB string is then subjected to `LastIndex` and `Index` operations.

**Impact:**
A single request consumes ~30MB of RAM and significant CPU time. 100 concurrent requests will consume 3GB of RAM and lock up the CPU threads, leading to a complete Denial of Service.

**Mitigation and Testing:**
The system MUST implement a hard limit on model string lengths at the API boundary, long before it reaches `pricing.go`. However, under Defense in Depth principles, `pricing.go` must also protect itself.
The tests must include a "Pathological Case Folding" benchmark using `testing.B` to quantify the CPU cost of these extreme Unicode strings, proving that the system degrades gracefully or rejects them outright.

#### Lock Contention Profiling
The `priceKeys` memoization strategy:
```go
priceMu.Lock()
defer priceMu.Unlock()
if priceKeysMemo != nil { return priceKeysMemo }
// ... rebuild ...
```
This is a standard pattern, but in a highly concurrent environment like codenerd processing millions of log streams, the read-write lock is a bottleneck.

**Profiling Requirement:**
Tests must be written using `go test -bench=. -benchtime=10s -cpuprofile=cpu.out -blockprofile=block.out`. The benchmark must simulate a 99% read / 1% write workload.
If the `block.out` profile shows significant time spent in `sync.(*RWMutex).RLock`, the architecture must be deemed insufficient for scale. The journal formally recommends migrating `priceTable` to an `atomic.Value` containing a read-only map (e.g., `map[string]Price`). When a price changes, a new map is allocated, copied, modified, and swapped atomically. This eliminates `RLock` contention entirely, providing wait-free lookups for the millions of usage tracking events, at the minor cost of allocations during rare `RegisterPrice` calls.

This concludes the rigorous Negative Testing and Boundary Value Analysis for the pricing subsystem.

### Addendum 4: Subsystem Interoperability and Mangle Rule Stratification

To satisfy the highest rigor of QA Automation, we must evaluate how `pricing.go` failures interact with codenerd's core logic solver, Mangle.

#### Stratification Errors in Pricing Logic
Mangle enforces stratification: rules cannot circularly depend on their own negation. If pricing logic were to be exported as a Mangle predicate (e.g., `price(Model, Cost)`), and a rule was written to override a price *if* the price was too high:
```mangle
override_price(Model, 0.0) :- price(Model, C), C > 10.0.
price(Model, C) :- override_price(Model, C).
```
This violates stratification and the analysis phase `analysis.Analyze(program)` will fail. However, because `RegisterPrice` allows imperative mutations outside of Mangle's knowledge, a Go routine could dynamically call `RegisterPrice` based on a Mangle output, creating a hidden, unstratified cycle that the engine cannot detect!

**Boundary Vector:**
If codenerd uses `RegisterPrice` as a feedback loop from an LLM response (e.g., the LLM negotiates a lower spot price), this creates a hidden recurrence relation.
The test suite must verify that the `usage_tracker` and `pricing` subsystems do not create unstable feedback loops when driven by external, non-deterministic agents.

#### The "Empty Result" Anomaly
In logic programming, bugs often manifest as zero results rather than panics. If `LookupPrice("unknown_model")` returns `false`, Mangle joins involving that model will silently fail (produce 0 tuples).
If a user submits an extreme vector like the 100MB string, and `normalizeModelName` decides to truncate it or return an empty string to protect memory, `LookupPrice` will return `false`.

**Impact:**
The system will safely protect its memory (no OOM), but the usage tracking rules in Mangle will silently drop the multi-million token cost event. The user gets free LLM compute because the security mechanism triggered an empty result in the logic engine.

**Testing Mandate:**
All negative tests for `pricing.go` that result in a "fail closed" state (returning `false` to protect the system) MUST be paired with an integration test in `usage_tracker_test.go` asserting that an explicitly unpriced model generates an `UnpricedTokens` metric rather than being silently dropped. The system must degrade to "known unknown" rather than "silent success".

This deep analysis ensures that `pricing.go` is hardened not just at the function level, but at the architectural system boundary.
