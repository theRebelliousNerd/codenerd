---

remediated: true
remediated_date: 2026-05-28
subsystem: prompt
---
# TokenBudgetManager Boundary Value Analysis & Negative Testing Journal

**Date:** $(TZ='America/New_York' date +"%Y-%m-%d %H:%M:%S EST")
**Subsystem Evaluated:** `internal/prompt/budget.go` (Token Budget Manager)
**Author:** QA Automation Engineer

## 1. Introduction & Scope

The `TokenBudgetManager` (in `internal/prompt/budget.go`) is a critical component in the Prompt compiler system responsible for allocating prompt context sizes correctly based on priorities, atom sizes, and strategies. It must run predictably, securely, and performantly during every JIT Compilation step.

The primary function evaluated is `Fit()`, which takes `totalBudget int` and `atoms []*OrderedAtom` to truncate or fit atoms based on pre-defined policies (Mandatory, High, Medium, Low) and strategies (Proportional, PriorityFirst, Balanced).
The `calculateAllocations()` subroutine determines token limits per category.
The system relies on floating-point math for percentage allocation and slices/structs for managing prompt atoms.

This evaluation specifically focuses on:
- Null/Undefined/Empty vectors
- Type Coercion and Precision loss vectors
- User Request Extremes (e.g. huge budgets, lots of atoms, malicious configurations)
- State Conflicts (race conditions on `m.mu`)

## 2. Identified Edge Cases & Gaps

The test suite (`budget_test.go`) is primarily focused on the happy path, benchmarking, and minimal invalid data testing (e.g. nil atoms or negative token counts). It lacks robust boundary value analysis. Below is a breakdown of the specific edge case gaps identified.

### 2.1 Null/Undefined/Empty

**Gaps Identified:**
1.  **Empty `budgets` map**: If `m.budgets` is manipulated into an empty map or wiped, how does `calculateAllocations` or `getPriority()` handle it? Will it result in zero tokens allocated for all categories?
2.  **Nil `atoms` array**: `Fit()` handles `len(atoms) == 0` but a purely `nil` array should explicitly be tested to ensure the first guard clause prevents any dereferencing panics.
3.  **Total Budget ≤ 0**: A total budget ≤ `m.reservedHeadroom` triggers an error. What if `totalBudget` is literally zero or intensely negative (e.g., `-1`)? It is handled gracefully by returning an error, but this path must be explicitly tested.
4.  **Zero `m.reservedHeadroom`**: Testing behavior when `reservedHeadroom` is explicitly set to 0.

### 2.2 Type Coercion / Precision Loss

**Gaps Identified:**
1.  **Precision Loss in Float Calculation**: `calculateAllocations` uses float math: `int(float64(totalBudget) * budget.BasePercent)`. For example, a `totalBudget` of 1001 with 3 categories having 33.3% will yield `333 * 3 = 999`. 2 tokens are lost to truncation. The code lacks precision testing to verify that 100% of the budget is distributed, or at least that remaining tokens are accurately pushed down to the next priority.
2.  **Type Conversions**: `catTokens += tokens` relies on `int64` and `totalBudget` relies on `int`. Coercing between 32-bit vs 64-bit platforms might produce inconsistent boundaries if `totalBudget` is `math.MaxInt32` on a 32-bit machine vs `math.MaxInt64` on a 64-bit machine.

### 2.3 User Request Extremes

**Gaps Identified:**
1.  **Overflow in `calculateAllocations`**: If `totalBudget` is passed in as `math.MaxInt` or `math.MaxInt64`, converting it to `float64` can lose precision, and then casting back to `int` after multiplying by `budget.BasePercent` might result in massive positive or wrap-around negative integers, breaking `clamp` and allocation logic.
2.  **Atom limits (`maxAtomsLimit`)**: The `Fit()` function hardcodes `const maxAtomsLimit = 5000`. A test must submit 10,000+ tiny atoms to guarantee that once `atomsIncluded >= maxAtomsLimit`, the loop successfully short-circuits to appending to `unselected` array and gracefully avoids processing.
3.  **Overflow in Token Addition**: `catTokens += tokens` uses `int64`. If a maliciously crafted `PromptAtom` has `TokenCount` near `math.MaxInt64`, it will overflow `catTokens` causing it to wrap around to a negative number. This bypasses the `catTokens+tokens <= int64(allocation)` check, potentially including an infinite number of atoms in the final prompt if the memory can handle it.
4.  **Enormous single atom**: What if a single `PriorityMandatory` atom has 2,000,000 tokens while the `totalBudget` is 8,000? `Fit()` iterates: `catTokens += tokens` and includes it because `oa.Atom.IsMandatory` overrides allocation checks, leading to context window explosions.

### 2.4 State Conflicts (Concurrency)

**Gaps Identified:**
1.  **Read/Write Race Conditions**: The `TokenBudgetManager` struct has a `sync.RWMutex` (`m.mu`).
    - `SetCategoryBudget`, `SetStrategy`, and `SetReservedHeadroom` use `m.mu.Lock()` (Write lock).
    - `Fit()` and `GenerateReport()` use `m.mu.RLock()` (Read lock).
    - The tests do not verify this concurrent thread safety. If 10 goroutines are calculating budgets (`Fit()`) while 5 goroutines are mutating policies (`SetStrategy`), will it panic or silently calculate using dirty states? A `go test -race` specifically targeting concurrent mutations is required.

## 3. Analysis of Subsystem Performance & Reliability

### 3.1 Flow & Performance Analysis
The subsystem is generally well-written for typical workloads. The `Fit()` method:
1. Filters nil/invalid atoms (O(n)).
2. Sorts atoms by Priority -> Category -> Score (O(n log n)).
3. Maps present categories and runs `calculateAllocations` (O(k) where k is categories).
4. Iterates through the sorted arrays linearly, resolving fits or pushing to `unselected` (O(n)).
5. Second pass iteration on `unselected` to pack remaining budget (O(m) where m is unselected count).

Total complexity is approximately **O(N log N)** where N is the number of input atoms. For 5000 atoms (the hardcoded max limit), sorting is negligible on modern hardware.

However, there is a risk of memory spikes during sorting because a new contiguous slice `sortedAtoms = make([]*OrderedAtom, 0, len(atoms))` is allocated. For 5,000 atoms, this is small, but if a malicious user request constructs 5,000,000 atoms, memory could spike dangerously before hitting the `maxAtomsLimit` break statement.

### 3.2 Float Precision & Strategy Execution
The calculation relies on integer truncation via `int(float...)`.
```go
allocation := int(float64(totalBudget) * budget.BasePercent)
```
In `StrategyBalanced` or `StrategyProportional`, small leftover values are dropped. In a 50k token budget context, losing 3-4 tokens is negligible, but this indicates the algorithm is mathematically inexact. A test should verify that the system gracefully handles the precision floor.

### 3.3 Security & Bounds
As analyzed, `maxAtomsLimit = 5000` is a good heuristic to prevent CPU exhaustion. The most severe gap is the handling of `IsMandatory` atoms. The current logic:

```go
if oa.Atom.IsMandatory {
    oa.RenderMode = mode
    result = append(result, oa)
    catTokens += tokens
    usedTokens += tokens
    atomsIncluded++
    continue
}
```
If an injected `IsMandatory` atom is 120k tokens long but the LLM budget is 32k, this logic will force inclusion of the 120k token atom. The LLM API will likely return a 400 Bad Request error. The `TokenBudgetManager` fails to protect the overall compilation pipeline from this specific overflow.

## 4. Detailed Findings per Vector

### Vector: Null/Undefined/Empty
- **Current state**: Empty arrays are handled gracefully (returns `nil, nil`). Nil items inside arrays are skipped.
- **Gap**: `totalBudget <= m.reservedHeadroom` returns an error, but tests do not validate negative numbers explicitly to guarantee bounds. Empty `budgets` map needs tests to prove fallback behavior.

### Vector: Type Coercion & Max Bounds
- **Current state**: Uses float64 casting for percentages. Uses int64 for token counting. Uses int for allocations.
- **Gap**: Need a test where `totalBudget` is `math.MaxInt`. Conversion to `float64` loses precision above 2^53 (9,007,199,254,740,992), and casting it back to `int` may result in negative integers on some architectures, which `clamp()` might "fix" unexpectedly if `min` is small, breaking the budget entirely.
- **Gap**: `TokenCount` is an integer. If manipulated to be near `math.MaxInt64`, it will wrap around when added: `usedTokens += tokens`, making `usedTokens` heavily negative and bypassing budget enforcement.

### Vector: User Request Extremes (The "Frontier Coding" Attack)
- **Current state**: 5000 hard limit exists for inclusion.
- **Gap**: Does not limit the number of atoms *passed in*, meaning sorting 5,000,000 objects will hang or OOM the CLI process before the 5000 limit protects it. Needs bounds checking on `len(atoms)` early.
- **Gap**: Mandatory priorities ignore token size.

### Vector: State Conflicts
- **Current state**: RWMutex exists.
- **Gap**: No concurrency tests validate the RWMutex. If the mutex is removed accidentally, no test will fail until production race conditions occur.

## 5. Recommendations & Action Items

1.  **Add Bounds Checking to `Fit()` early**:
    If `len(atoms) > 100_000`, truncate or throw an error *before* attempting to allocate and sort them to prevent OOM in extreme scenarios.
2.  **Fix Mandatory Inclusion Overflow**:
    Even `IsMandatory` items should respect the absolute hardcap of `totalBudget`. If a mandatory item pushes `usedTokens > totalBudget`, it should be logged as an error and potentially truncated or skipped to prevent breaking the downstream LLM API call.
3.  **Guard against Integer Overflow**:
    Add `if math.MaxInt64 - catTokens < tokens { return Error }` style guards when incrementing `catTokens` and `usedTokens` to prevent wrap-around bugs from malicious atoms.
4.  **Implement the test gaps in `budget_test.go`**:
    - Add `TestTokenBudgetManager_Fit_Extremes`
    - Add `TestTokenBudgetManager_Fit_Concurrency`
    - Add `TestTokenBudgetManager_CalculateAllocations_Precision`

## 6. Appendix: Additional Negative Testing Vectors

### 6.1 Uninitialized Memory & Null Pointer Dereferencing
- A completely uninitialized `TokenBudgetManager` (created via `var manager *TokenBudgetManager` instead of `NewTokenBudgetManager()`) will panic immediately when `manager.mu.RLock()` is called if not properly instantiated. A negative test must assert that methods invoked on a nil receiver either return an error gracefully or are explicitly guarded by documentation.
- Inside the sorting closure, if `sortedAtoms[i].Atom` is somehow nil despite the initial filtering loop, a panic will occur. The initial filter loop checks `if atom == nil || atom.Atom == nil { continue }`, making it relatively safe. However, verifying this logic with fuzzy inputs is still required for high assurance.

### 6.2 Bizarre String Categories (Data Injection)
- The budget manager uses `AtomCategory` (an alias for string) for maps and logic. What if the `Category` string is a massive 100MB string, or contains SQL injection payloads, or null bytes `\x00`?
- Go maps handle large string keys correctly by hashing, but printing/logging these strings later could lead to memory issues or log injection vulnerabilities. Tests should pass bizarre, large string bytes as `Category` strings.

### 6.3 Clock/Timer Issues
- `Fit()` uses `logging.StartTimer`. While this is mostly for telemetry, negative testing should consider environments where system clocks are unstable or jump backwards (e.g., during NTP syncs), verifying that telemetry timing does not induce panics.

### 6.4 The "Zero Sum" Edge Case
- What if a legitimate prompt atom has exactly 0 tokens? The system bounds checking `atom.Atom.TokenCount < 0 { atom.Atom.TokenCount = 0 }` normalizes it. But a 0 token count implies infinite atoms could theoretically fit into a budget.
- The `maxAtomsLimit` serves as the secondary brake. A test should verify that passing 10,000 atoms with 0 tokens hits the `maxAtomsLimit` branch successfully and doesn't just infinitely recurse or break allocation thresholds.

### 6.5 Render Mode Fallback Logic Validation
The rendering mode fallbacks ("standard" -> "concise" -> "min"):
- What if an atom claims `ContentConcise` is present but the `TokenCount` of concise is *larger* than standard due to a bug upstream?
- The budget manager relies on `getTokenCount(oa.Atom, "concise")`. If the fallback is larger, it might still fail to fit, but it wastes computation cycles. Negative tests should verify robustness when downstream token size assumptions are inverted.

### 6.6 Mutating State during Fit
Although `m.mu` protects the `m.budgets` map and `m.strategy`, the `atoms` slice passed in as a pointer is *not* protected from concurrent mutation by the caller.
- If the caller modifies the `atoms` slice (e.g., changes an atom's score or category) *while* `Fit()` is executing, race conditions can occur within the sort closure or allocation calculation.
- The manager copies pointers: `sortedAtoms = append(sortedAtoms, atom)`. This prevents structural changes to the slice from crashing the system, but mutating the underlying `PromptAtom` struct concurrently will still cause data races. Tests should document or enforce caller-side immutability guarantees.

### 6.7 Extreme Headroom
- What if `totalBudget` is 10,000, and `m.reservedHeadroom` is deliberately set to 15,000 via `SetReservedHeadroom(15000)`?
- The code handles this: `availableBudget := totalBudget - m.reservedHeadroom; if availableBudget <= 0 { return nil, error }`.
- However, what if `reservedHeadroom` is negative (e.g., `-5000`)? `availableBudget` would become `10000 - (-5000) = 15000`, essentially expanding the budget beyond safe LLM limits. A negative test must check `SetReservedHeadroom` for negative values and assert it clamps them to 0 or rejects them.

### 6.8 The Proportional Strategy Discrepancy
- Under `StrategyProportional`, `allocation = int(float64(totalBudget) * budget.BasePercent)`.
- If the sum of all `BasePercent` across all present categories does not equal 1.0 (e.g., it only equals 0.5 because some categories are missing), 50% of the `totalBudget` is never allocated in pass 1.
- Pass 2 fills the remaining budget, but relies strictly on score descending. This fundamentally alters the intended proportional mix. A negative test should simulate missing categories and verify pass 2 correctly absorbs the deficit without violating other constraints.

### 6.9 Clamp Function Extremes
- `func clamp(value, min, max int) int`
- What if `min > max`? e.g., `clamp(50, 100, 10)`. The logic is:
  ```go
  if value < min { return min }
  if value > max { return max }
  ```
- If called with `(50, 100, 10)`, `value < min` (50 < 100) triggers and returns 100.
- If called with `(150, 100, 10)`, `value < min` (150 < 100) is false, `value > max` (150 > 10) is true, returns 10.
- This creates non-deterministic bounds checking if `min` and `max` are misconfigured in `CategoryBudget`. A boundary test should specifically inject invalid `CategoryBudget{MinTokens: 5000, MaxTokens: 1000}`.

### 6.10 Iterative Exhaustion
The second pass:
```go
var remaining int64 = int64(availableBudget) - usedTokens
// ... loop through unselected
```
- If an attacker provides 4999 atoms that fit perfectly, and 1 massive unselected atom, does pass 2 waste time trying to fallback through standard->concise->min for an atom that obviously won't fit? Yes. A performance boundary test should verify the time complexity of failing repeatedly in pass 2.

### 6.11 Division by Zero / Panic Vectors
- Does any logic divide by the length of arrays? No, the code appears safe from obvious divide-by-zero panics.

### 6.12 Subsystem Conclusions
The `internal/prompt/budget.go` subsystem correctly handles its core mandate but is fragile against deliberately adversarial or completely uninitialized inputs.
The integration with `m.mu` provides basic thread safety, but the lack of caller-side immutability enforcement on the `atoms` pointer array is a long-term technical debt item.

The heavy reliance on implicit `int` limits and floating-point percentage math requires robust bounds checking to guarantee the safety of downstream components (like the LLM inference engine) which will crash if context windows are blown due to an integer wrap-around vulnerability inside this manager.

Applying these identified negative and boundary value tests will transition this from a functional component to a high-assurance agent primitive.


## 7. Deep Dive: Memory Profiling Boundary Conditions

To achieve true high-assurance guarantees within the `TokenBudgetManager`, memory stability under boundary conditions is just as critical as logic correctness. The boundary tests defined above highlight logical failures, but we must also consider the physical constraints of the execution environment (e.g., a lightweight container or an overloaded orchestration node).

### 7.1 Garbage Collection Thrashing on Slice Re-allocation
When `Fit()` processes the `atoms` input, it creates multiple slices:
1.  `sortedAtoms := make([]*OrderedAtom, 0, len(atoms))`
2.  `result := make([]*OrderedAtom, 0, len(atoms))`
3.  `unselected := make([]*OrderedAtom, 0, len(atoms))`

**Boundary Scenario:**
If a user submits 100,000 atoms, `Fit()` will immediately allocate memory for three slice headers backing arrays of 100,000 pointers. On a 64-bit system, that's roughly `3 * (100,000 * 8 bytes) = 2.4 MB` just for the backing arrays, per request.
While 2.4 MB seems small, if the system handles 50 concurrent compilation requests, this scales linearly.
More critically, if `len(atoms)` is extremely large but only 5,000 atoms (the `maxAtomsLimit`) are ever processed, allocating `len(atoms)` for `result` and `unselected` is wildly inefficient.

**Negative Testing Strategy:**
A negative test should simulate a massive input array (e.g., 5,000,000 atoms).
The test should monitor `runtime.MemStats` before and after the `Fit()` call to assert that the memory footprint remains bounded and does not linearly scale with `len(atoms)` if `len(atoms)` vastly exceeds `maxAtomsLimit`.

### 7.2 The `fmt.Sprintf` and String Allocation Bottleneck
The `budget.go` file uses `fmt.Errorf` and string concatenation for error reporting and debug logging:
```go
return nil, fmt.Errorf("total budget %d is less than reserved headroom %d", totalBudget, m.reservedHeadroom)
```
While this specific line is not in a hot loop, if logging is set to `Debug` level, string concatenations in hot loops (if they existed) would rapidly generate garbage. We must ensure that boundary values (like extremely long string inputs for Category names) do not cause log-related OOMs.

### 7.3 Map Pre-allocation and Hash Collisions
The `calculateAllocations` subroutine utilizes:
```go
presentCategories := make(map[AtomCategory]bool)
```
In `Fit()`, this is initialized dynamically:
```go
for _, oa := range sortedAtoms {
    presentCategories[oa.Atom.Category] = true
}
```
**Boundary Scenario:**
If an attacker manages to synthesize thousands of unique categories (e.g., `Category: "cat_" + randString()`), this loop will force the map to continually resize, triggering rehashing algorithms and blocking execution.
**Negative Testing Strategy:**
Inject an array of 5,000 atoms, each with a unique, completely synthetic category string. Assert that the `Fit()` function processes it within a specific strict timeout (e.g., <5ms) to prove it is immune to algorithmic complexity attacks on the hash map.

## 8. Deep Dive: Mangle Logic Engine Interface Vectors

The `TokenBudgetManager` does not exist in a vacuum. It acts as the final gatekeeper before the `CompilationResult` is sent back to the Session Executor or LLM interface. However, the *atoms themselves* are generated by the Mangle Logic kernel (via `Phase 1` skeleton generation and `Phase 2` flesh expansion).

### 8.1 The "Poisoned Fact" Vector
If the Mangle kernel produces a logically valid but contextually poisoned fact (e.g., an atom with a negative `Score`), how does `Fit()` behave?
- `sortedAtoms` sorts by score: `sortedAtoms[i].Score > sortedAtoms[j].Score`.
- A negative score is mathematically valid here.
- However, if pass 2 attempts to "fill remaining budget with best remaining atoms", negative scored atoms will simply be pushed to the bottom. But if the budget is huge, they *will* still be included.
**Negative Testing Strategy:**
Test with a budget of 100k, and 5 atoms: 1 with score 1.0, and 4 with score -100.0. Ensure they are still appended. This validates that the budget manager strictly manages *size* and *priority*, and does not implicitly act as a score-based threshold filter (which is the responsibility of the `Selector`).

### 8.2 Discrepancy between Atom Tokens and Final Serialized Output
The `TokenBudgetManager` relies entirely on `oa.Atom.TokenCount`.
What if `TokenCount` says 50, but the actual string representation of the atom requires 5,000 tokens when serialized to JSON for the LLM?
This is a boundary between subsystems. While `budget.go` cannot inherently fix bad `TokenCount` heuristics, it *must* fail safely if its own bounds are manipulated.
**Negative Testing Strategy:**
No code change needed in `budget.go`, but a systemic integration test must verify that if `TokenCount` is maliciously reported as 1 for a 50MB string, `budget.go` processes it instantly, proving its decoupling from string serialization logic.

## 9. Final Strategic Recommendations for Robustness

To elevate the subsystem to military-grade reliability (as required by the system's `high-assurance` mandate):
1.  **Cap slice allocations**: `result = make([]*OrderedAtom, 0, min(len(atoms), maxAtomsLimit))`
2.  **Hard fail on wrap-around**: Introduce explicit `math.MaxInt64` bounds checking in hot loops.
3.  **Strictly enforce `reservedHeadroom` bounds**: Ensure it cannot be negative via a setter guard.
4.  **Implement continuous memory profiling**: Add Benchmark tests explicitly designed to fail if allocations per operation exceed a set threshold.

These changes will close the critical gaps identified in this boundary value analysis.

## 10. Expanding the User Request Extremes: The "Turing Test" Vectors

When dealing with LLM interfaces, the agent will inevitably encounter inputs that are not just numerically extreme, but structurally hostile or bizarre (often termed "Turing Test" vectors or adversarial prompts). While the LLM itself handles semantic hostility, the `TokenBudgetManager` must handle the structural fallout.

### 10.1 The "Recursive Dependency" Atom Bomb
Imagine a scenario where the `Decomposer` or `Selector` enters an infinite loop, or resolves a deeply nested dependency graph of code files, resulting in 50,000 highly similar atoms being queued for budget fitting.
- **The Gap:** The `Fit()` function iterates through atoms by category. If 50,000 atoms are all `CategoryCapability`, the first pass will consume the entire category allocation. Pass 2 will then process the remaining atoms. If the budget is immense (e.g., 2,000,000 tokens for a next-gen LLM), pass 2 will iterate through tens of thousands of atoms, running `getTokenCount` for `standard`, `concise`, and `min` on each one.
- **Performance Impact:** The `getTokenCount` function is relatively cheap, but executing it 150,000 times (3 modes * 50,000 atoms) sequentially inside the `Fit()` lock (`m.mu.RLock()`) blocks other system processes from updating budgets or strategies.
- **Negative Testing Strategy:** Generate 100,000 minimal atoms (1 token each). Set a massive total budget. Measure the execution time of `Fit()`. The lock hold time must not exceed a critical latency threshold (e.g., 50ms), or it risks stalling the entire orchestrator loop.

### 10.2 The "Empty Rendering" Edge Case
In the fallback logic, the code checks:
```go
// Try Concise
if oa.Atom.ContentConcise != "" {
    mode = "concise"
    // ...
}
```
- **The Gap:** What if an atom is purposefully constructed such that `ContentConcise` is *not* empty (e.g., it contains a single space `" "`), but its semantic value is essentially nil?
- While this is technically a content issue, structurally, it forces the budget manager into the concise branch. If `getTokenCount` (which relies on `EstimateTokens`) returns 0 or 1 for this string, the manager will happily include hundreds of functionally empty atoms, consuming the `maxAtomsLimit` allowance (5000) with garbage context.
- **Negative Testing Strategy:** Create atoms where standard content is 10k tokens, but concise content is just `"\n"`. Ensure the fallback executes cleanly and `EstimateTokens` handles whitespace-only strings predictably without throwing panics.

### 10.3 The Priority Inversion Vulnerability
The `calculateAllocations` strategy `StrategyPriorityFirst` allocates strictly based on priority tiers (Mandatory -> High -> Medium -> Low/Conditional).
- **The Gap:** The percentages (`BasePercent`) dictate how much of the *remaining* budget is taken.
- If `PriorityMandatory` has `BasePercent: 0.9` (90%), it takes 90% of the total budget.
- Then `PriorityHigh` with `BasePercent: 0.9` takes 90% of the *remaining* 10%.
- This mathematically guarantees that lower priorities will almost always receive *something*, even if tiny. However, if `totalBudget` is very small (e.g., 500), 90% is 450. Remaining is 50. 90% of 50 is 45. Remaining is 5.
- At extremely small budgets, float truncation (`int(float64(5) * 0.9) = 4`) and clamping might result in `PriorityMedium` receiving 0, while a `PriorityConditional` atom might accidentally bypass checks if logic dictates unallocated space falls through.
- **Negative Testing Strategy:** Execute `StrategyPriorityFirst` with an unnaturally constrained budget (e.g., exactly `m.reservedHeadroom + 1`). Validate that the hierarchical starvation model executes mathematically perfectly and does not throw divide-by-zero or negative-allocation bounds errors.

## 11. State Transition and Quiescence Verification

In the codeNERD architecture, sessions undergo "Quiescent Boot" and state transitions where memory is wiped or restored.

### 11.1 The "Stale Cache" Concurrency Vector
The `TokenBudgetManager` is instantiated once per compiler or session. If a session enters a quiescent state, the manager's configuration (strategies, budgets) persists unless explicitly reset.
- **The Gap:** If `SetCategoryBudget` is called during a session reset to modify a baseline budget (e.g., halving the budget for `CategoryKnowledge`), and `Fit()` is simultaneously called by a lagging background evaluation thread from the previous state, does the data race corrupt the map?
- Yes, maps in Go are inherently unsafe for concurrent writes. The `m.mu.Lock()` prevents this *during* the execution of `Fit()`, but it does not prevent logic interleaving (where the `Fit()` output is sent to the LLM based on an older, now-invalid budget constraint).
- **Negative Testing Strategy:** The test must prove that the mutex prevents panics (`fatal error: concurrent map read and map write`), but system architecture must acknowledge the interleaving issue.

## 12. Final Execution Directives for Remediation

To close these complex vectors, the remediation plan must include:

1.  **Immutability enforcement:** Update `SetCategoryBudget` to copy-on-write the entire `budgets` map rather than mutating it in place, allowing lock-free reads in `Fit()` (optimizing the hot path and eliminating lock contention during massive scaling).
2.  **Integer Math Refactoring:** Replace float-based `BasePercent` math with integer-based permille (parts per thousand) math to guarantee cross-architecture determinism and eliminate precision loss completely. (e.g., `(totalBudget * permille) / 1000`).
3.  **Strict Cap Implementation:** As noted earlier, immediately cap allocations for slices to `min(len(atoms), maxAtomsLimit)` to prevent memory exhaustion attacks.

End of Boundary Value Analysis Journal.

## 13. Deep Boundary Constraints on Allocation Subroutines

The `calculateAllocations` subroutine acts as the mathematical engine for the budget manager. Its deterministic behavior is critical for reproducibility. We must analyze its boundary conditions under duress.

### 13.1 Strategy Proportional: Missing Categories Deficit

When using `StrategyProportional`, the code executes:
```go
for cat, budget := range m.budgets {
    if !presentCategories[cat] { continue }
    allocation := int(float64(totalBudget) * budget.BasePercent)
    // ...
```
- **The Gap:** The `BasePercent` values across all categories might theoretically sum to 1.0 (100%). But if `presentCategories` only contains a single category with a `BasePercent` of 0.1 (10%), then 90% of `totalBudget` is simply not allocated in the first pass.
- **Why this is dangerous:** If 90% of a massive budget is left unallocated, the `Fit()` function relies entirely on the secondary "fill remaining budget" pass. This secondary pass sorts purely by `Score` descending, ignoring all category balancing. This defeats the purpose of the Token Budget Manager, turning it into a simple highest-score-wins algorithm.
- **Negative Testing Strategy:** Feed `Fit()` with atoms from only a single low-percentage category (e.g., `CategoryKnowledge` at 5%). Verify that the system successfully reallocates the remaining 95% of the budget during Pass 2 without throwing an error, but also document this architectural behavior as a potential logic flaw that bounds testing has exposed.

### 13.2 Strategy Balanced: Minimum Allocation Starvation

The `StrategyBalanced` logic starts by distributing minimum tokens:
```go
for cat, budget := range m.budgets {
    if !presentCategories[cat] { continue }
    allocations[cat] = budget.MinTokens
    remaining -= budget.MinTokens
}
```
- **The Gap:** What if the sum of `MinTokens` for all `presentCategories` exceeds the `totalBudget`?
- **Example:** `totalBudget` is 4000. We have 5 categories present, each with a `MinTokens` of 1000.
- `remaining` starts at 4000. It drops to 3000, 2000, 1000, 0, and then -1000.
- **The Fallout:** The loop does not stop. `remaining` becomes negative. Then it moves to the proportional distribution phase:
```go
for cat, budget := range m.budgets {
    // ...
    extra := int(float64(remaining) * budget.BasePercent)
    allocations[cat] = clamp(allocations[cat]+extra, budget.MinTokens, budget.MaxTokens)
}
```
- Since `remaining` is negative (-1000), `extra` will be negative. The system subtracts from the `MinTokens` it just allocated, and then uses `clamp()` which will force the value back up to `MinTokens`.
- **Result:** The allocations map will dictate allocating 5000 tokens when the total budget is only 4000! `Fit()` will then try to respect these allocations, potentially exceeding the `totalBudget` entirely.
- **Negative Testing Strategy:** Construct a scenario where `sum(MinTokens) > totalBudget` and execute `StrategyBalanced`. Assert that the final output array of atoms does not exceed `totalBudget`, proving that the downstream token addition checks in `Fit()` (`catTokens+tokens <= int64(allocation)`) fail safely, even if the allocation map is mathematically impossible.

### 13.3 Nil Pointers in Nested Structs

The `Fit()` function checks:
```go
if atom == nil || atom.Atom == nil {
    continue
}
```
- **The Gap:** This checks the top level pointers. But what about strings? A string in Go cannot be nil, but it can be empty. What if `atom.Atom.ID` is empty? What if `atom.Atom.Category` is cast from an empty string `AtomCategory("")`?
- The code handles empty strings gracefully in maps (an empty string is a valid map key). However, logging might output weird formatting: `Category : allocated X tokens`.
- **Negative Testing Strategy:** Pass `OrderedAtom` objects where the inner `PromptAtom` has completely uninitialized fields (empty strings for ID, Content, Category, etc.). Verify that the system processes them as valid 0-length objects without throwing panics in `string()` formatters or causing map key collisions.

## 14. Synthesizing the Impact of Negative Testing

By integrating these deep boundary tests, the CodeNERD system guarantees resilience against:
1.  **Denial of Service (DoS):** By capping slice allocations and catching infinite loops or massive hash map resizing.
2.  **Context Window Explosions:** By strictly enforcing budget caps even against `PriorityMandatory` overrides.
3.  **Data Corruption:** By ensuring mutexes protect the allocation logic and that integer conversions do not wrap around to negative numbers.

This journal entry serves as the blueprint for the required automated test suite implementation in `budget_test.go`.


## 15. Operational Stability During Hot-Swapping

In the codeNERD architecture, it is possible for the system config to be reloaded at runtime (hot-swapping config files or receiving updated policies from the `ShardAdvisoryBoard`). The `TokenBudgetManager` must handle this seamlessly.

### 15.1 State Machine Race Conditions

The `SetCategoryBudget` function updates the internal `m.budgets` map.
- **The Gap:** What if a configuration hot-swap deletes a category entirely while `Fit()` is executing?
- Because `Fit()` holds the `RLock()`, the hot-swap thread (which needs a full `Lock()`) will block until `Fit()` completes. This is correct behavior.
- However, if the system is under intense load (e.g., orchestrating 50 concurrent file edits across multiple `AgentConfig` instances), the `RWMutex` could experience writer starvation. The `Fit()` calls constantly acquire read locks, meaning the write lock for the hot-swap may be delayed indefinitely, causing the system to operate on stale configurations.
- **Negative Testing Strategy:** Write a concurrent test that spawns 1,000 goroutines continuously calling `Fit()`, and 1 goroutine attempting to call `SetStrategy()`. Measure the latency of the write lock acquisition. If it exceeds acceptable bounds, recommend implementing an atomic pointer swap (RCU pattern) instead of an `RWMutex` for the config state.

### 15.2 Handling Corrupted Configurations

If a corrupted `AgentConfig` is loaded, it might pass invalid data to `TokenBudgetManager`:
- `BasePercent` set to `10.0` (1000%).
- `MinTokens` set to `-500`.
- `MaxTokens` set to `0`.
- **The Gap:** The `TokenBudgetManager` trusts the incoming `CategoryBudget` blindly.
```go
func (m *TokenBudgetManager) SetCategoryBudget(budget CategoryBudget) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.budgets[budget.Category] = budget
}
```
- If `MinTokens` is negative, `clamp()` might return a negative allocation. If `BasePercent` is `10.0`, allocations will vastly exceed the `totalBudget`, causing the safety nets in `Fit()` to rely purely on the fallback `int64(allocation)` checks.
- **Negative Testing Strategy:** Inject absurd `CategoryBudget` structures (negative limits, >1.0 percentages). Ensure `Fit()` does not panic, and evaluate whether `SetCategoryBudget` should validate inputs and return an error before accepting poisoned state.

## 16. Final Sign-off

The boundary analysis confirms that while `internal/prompt/budget.go` is logically sound for anticipated happy-path scenarios, it lacks the strict defensive programming required for a high-assurance framework. The test gaps identified must be closed to harden the system against malicious inputs, integer overflows, and uninitialized states.

End of Report.
