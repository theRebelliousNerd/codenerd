# Quality Assurance Journal
**Date/Time:** 2026-03-08 01:11:39 EST
**Subsystem:** JIT Prompt Compiler - Token Budget Manager (`internal/prompt/budget.go`)

## Overview
The Token Budget Manager is a critical component within the codeNERD JIT clean loop. It operates at the heart of the Prompt Compiler, allocating token budgets across different atom categories based on available context windows and semantic priority. Given the dynamic nature of JIT sub-agent generation and modular tool registries, this subsystem frequently encounters varying context constraints, diverse token counts, and priority collisions.

This journal entry details boundary value analysis, negative testing vectors, and architectural observations for `TokenBudgetManager.Fit()` and related structures. The goal is to fortify this subsystem against edge cases that could otherwise lead to integer overflow, runtime panics, or starvation of essential context.

## Edge Case Analysis

### 1. Null / Undefined / Empty Inputs

**Vector Analysis:**
The current implementation of `Fit` contains basic empty checks but lacks deeper validation:
```go
if len(atoms) == 0 {
    return nil, nil
}
```
However, the test suite does not sufficiently exercise scenarios involving malformed or partially instantiated arrays.

**Identified Gaps:**

1.  **`nil` Element in Slice:** Passing `[]*OrderedAtom{nil}` into `Fit()`. The code immediately accesses `sortedAtoms[i].Atom.Category`, which will cause a nil pointer dereference and crash the compilation loop. This is a critical vulnerability if upstream selectors fail to filter nulls.
    *   *Remediation needed:* Add a fast-fail or skip-loop condition for `oa == nil`.

2.  **`nil` Atom in Element:** Passing `[]*OrderedAtom{{Atom: nil}}`. Similar to the above, accessing `oa.Atom.Category` or `oa.Atom.TokenCount` will panic. The system assumes a valid `*PromptAtom` pointer is always present.
    *   *Remediation needed:* Add a fast-fail or skip-loop condition for `oa.Atom == nil`.

3.  **Empty `Content` Fields:** An atom may have an empty `ContentConcise` but a valid `TokenCount`. The logic handles this gracefully by checking `!= ""`, but we should explicitly test combinations of `TokenCount > 0` and empty contents to ensure no zero-length strings bypass logic checks or cause unintended minification behaviors.
    *   *Remediation needed:* Write test cases feeding atoms with mismatched TokenCount and empty Content fields.

4.  **Missing Categories in Map:** A budget map with missing mandatory categories. If a mandatory category is omitted from the `budgets` map but atoms of that category are passed, the priority defaults to `int(PriorityConditional) + 1` (very low priority). However, `oa.Atom.IsMandatory` will forcefully include it later. The interaction between unbudgeted categories and the mandatory flag requires rigorous testing to ensure it doesn't destabilize the `availableBudget` calculation.
    *   *Remediation needed:* Verify behavior of unbudgeted mandatory atoms.

5.  **Nil Slices in Report Generation:** The `GenerateReport` method iterates over atoms. If `atoms` is nil or contains nil pointers, it will panic when calculating used tokens.
    *   *Remediation needed:* Add null checks within the reporting loop.

6.  **Zero Budget Inputs:** `Fit()` with `totalBudget == 0`. It errors if `availableBudget <= 0`. If `reservedHeadroom` is modified to `0` and `totalBudget` is `0`, what happens?
    *   *Remediation needed:* Ensure zero or negative budget values don't trigger zero-allocation infinite loops downstream.

### 2. Type Coercion / Invalid Data

**Vector Analysis:**
While Go's strong typing prevents literal string-to-int coercion errors at compile time, logical "coercion" of invalid semantic data happens when external systems (e.g., SQLite DB rows, manual tool edits, or LLM transducers) feed malformed integer values into the system.

**Identified Gaps:**

7.  **Negative Token Counts:** If an `OrderedAtom` has a `TokenCount` of `-5000` (e.g., due to a database parsing error or a flawed token estimation function), the logic `catTokens += tokens` and `usedTokens += tokens` will *subtract* from the used tokens. This artificially inflates the remaining budget, potentially allowing unbounded inclusion of subsequent atoms and causing downstream LLM API calls to fail due to exceeding hardware context limits.
    *   *Remediation needed:* Reject or floor negative TokenCounts to 0.

8.  **Fractional Token Calculations:** While `TokenCount` is an integer, `BasePercent` is a float. If `BasePercent` calculations result in precision loss (e.g., `0.333333 * totalBudget`), the sum of allocated tokens might slightly under-allocate the available budget. Tests are needed to ensure truncation during proportional allocation doesn't systematically lose tokens across many categories.
    *   *Remediation needed:* Write tests verifying the sum of float-calculated allocations doesn't drift significantly.

9.  **Invalid Budget Priority Values:** If a priority is cast from an invalid integer (`BudgetPriority(99)`), `getPriority` will handle it, but it might sort unpredictably. Tests should confirm behavior when enum bounds are violated.
    *   *Remediation needed:* Add tests enforcing expected enum behavior.

10. **Malformed IDs:** What if an Atom's ID is empty or contains non-UTF8 binary data? Does the report generation or logging choke on it?
    *   *Remediation needed:* Feed malformed string data into the Fit/Report lifecycle.

### 3. User Request Extremes

**Vector Analysis:**
codeNERD supports "brownfield requests to work on 50 million line monorepos." This translates to extreme constraints on token budgeting, where the context must be heavily compressed or carefully selected.

**Identified Gaps:**

11. **Massive Token Counts (Integer Overflow):** An atom representing a massive file might legitimately have `TokenCount = math.MaxInt64` (or `math.MaxInt`). Summing these in `catTokens += tokens` could overflow the integer, resulting in a negative value. This bypasses the `catTokens+tokens <= allocation` check, incorrectly including massive files and subsequently crashing the memory allocator or the LLM request layer.
    *   *Remediation needed:* Implement overflow-safe addition or explicit bounds checking.

12. **Massive Atom Arrays:** The `Fit` function pre-allocates arrays and uses a nested structure to try Standard/Concise/Min formats. Testing the subsystem with an input array of 1,000,000 `OrderedAtom` elements is necessary to benchmark CPU and memory scaling, especially the `sort.Slice` and the internal copy operations (`make([]*OrderedAtom, len(atoms))`).
    *   *Remediation needed:* Add aggressive benchmarking tests for ultra-large array inputs.

13. **Impossible Mandatory Budgets:** What happens if the sum of `TokenCount` for `IsMandatory=true` atoms drastically exceeds `totalBudget`? The code forcefully includes them: `if oa.Atom.IsMandatory { ... catTokens += tokens; usedTokens += tokens; continue }`. This means `usedTokens` can greatly exceed `totalBudget`, making `remaining` negative in the second pass. The second pass handles `remaining > 0`, but the final `GenerateReport` might report negative remaining budgets. We need explicit tests to verify how the system reacts to negative remaining budgets and if it correctly flags the `OverBudgetAmount` without underflowing unsigned integer conversions elsewhere.
    *   *Remediation needed:* Test scenarios where mandatory atoms exceed total budget by orders of magnitude.

14. **Extreme Number of Categories:** While the system currently defines a fixed set of categories, if a sub-agent introduces 10,000 custom categories via prompt atoms, the `presentCategories` map allocation and subsequent iterations could become a performance bottleneck.
    *   *Remediation needed:* Test map scaling with thousands of dynamically injected categories.

15. **Maximum Headroom Configurations:** What if `reservedHeadroom` is configured higher than the available system memory or the maximum conceivable token limit of an LLM?
    *   *Remediation needed:* Add validation bounds on SetReservedHeadroom.

### 4. State Conflicts / Concurrency

**Vector Analysis:**
The Token Budget Manager maintains internal state (`budgets`, `strategy`, `reservedHeadroom`). In a highly concurrent environment like codeNERD, where multiple sub-agents might be spun up simultaneously or configurations hot-swapped via the ConfigFactory, shared mutable state is dangerous.

**Identified Gaps:**

16. **Race Conditions on Budget Map:** The `TokenBudgetManager` uses a standard Go map (`map[AtomCategory]CategoryBudget`). If one goroutine calls `SetCategoryBudget` (e.g., a self-modifying Ouroboros loop tweaking priorities based on failure rates) while another goroutine is executing `Fit()`, a concurrent map read/write panic will occur. Maps in Go are not safe for concurrent access.
    *   *Remediation needed:* Implement `sync.RWMutex` around map accesses.

17. **Race Conditions on Strategy:** Modifying `strategy` or `reservedHeadroom` concurrently with `Fit()` or `calculateAllocations()` could lead to unpredictable allocation logic midway through execution. A context switch between `availableBudget := totalBudget - m.reservedHeadroom` and the allocation loop could use mismatched parameters.
    *   *Remediation needed:* Wrap state mutations in thread-safe locks.

18. **Shared Instances vs. Per-Request Instances:** Currently, it appears a new manager might be instantiated per compilation (via `NewTokenBudgetManager`). However, if an architectural refactor introduces a shared "GlobalBudgetManager" for a specific sub-agent pool, these race conditions will manifest. The test suite lacks parallel execution tests (`t.Parallel()`) to verify thread safety guarantees for the methods.
    *   *Remediation needed:* Write `t.Parallel()` tests firing setters and getters concurrently.

19. **Sort Non-Determinism on Ties:** The tie-breaker sorting logic in `Fit()` falls back to `catI < catJ`. If multiple atoms within the same category have the exact same score, `sort.Slice` (which is not stable) will order them non-deterministically. This leads to flaky tests and unpredictable budget eviction across runs.
    *   *Remediation needed:* Use `sort.SliceStable` or add an `ID` tie-breaker to the sorting function.

## Architectural Assessment & Performance Notes

Is the `TokenBudgetManager` performant enough to handle these edge cases?

1.  **Memory Allocations:**
    The `Fit` function makes a defensive copy of the array:
    ```go
    sortedAtoms := make([]*OrderedAtom, len(atoms))
    copy(sortedAtoms, atoms)
    ```
    For large monorepo arrays (e.g., millions of atoms), this O(N) allocation could contribute to OOM events. However, the system relies on `selector.go` to filter candidate atoms *before* budgeting, so the array size shouldn't reach millions in practice. Still, for safety, replacing the defensive copy with an in-place sort (if the caller guarantees they don't need the original order) or using an index array could improve memory efficiency and reduce garbage collection pressure.
    Furthermore, `result = make([]*OrderedAtom, 0, len(atoms))` pre-allocates the worst-case scenario. This is generally good for reducing reallocation overhead, but if `len(atoms)` is massive and only 5% fit, it wastes memory.
    *   *Assessment: Moderately Performant. Vulnerable to memory spikes if the pre-selector fails to cull inputs.*

2.  **Algorithm Complexity:**
    The core algorithm relies on `sort.Slice`, which is O(N log N). The subsequent iteration over `sortedAtoms` is O(N). The performance is highly adequate for the expected operational envelope (usually < 10,000 atoms).
    The map lookups for `presentCategories` and allocations are O(1) amortized, which is efficient.
    *   *Assessment: Highly Performant computationally.*

3.  **Concurrency Weakness:**
    The lack of a `sync.RWMutex` protecting the `budgets` map and other state variables (`strategy`, `reservedHeadroom`) is a critical vulnerability if instances of `TokenBudgetManager` are shared across threads. Given the `atomic.Pointer` usage seen elsewhere in `compiler.go` for JIT observation state, the budget manager should ideally adopt similar lock-free or read-write lock patterns if it ever escapes the bounds of a single compilation request.
    *   *Assessment: Not Performant under shared concurrent load. Safe only if strictly instantiated per-request.*

4.  **Safety Mechanisms vs. Silent Failures:**
    The use of `clamp` is robust against negative token allocations in the proportion calculations. However, the system lacks robust defenses against *overflows* (from summing massive tokens) or *underflows* (from negative token inputs on atoms). A malicious or corrupted atom with `TokenCount = -999999` will silently subvert the entire budget constraint system.
    *   *Assessment: Brittle. Logic relies on upstream validation which may not exist.*

5.  **String Concatenation in Logging:**
    The logging statement inside the tight loop:
    ```go
    logging.Get(logging.CategoryContext).Debug("Category %s: allocated %d tokens, used %d tokens", cat, allocation, catTokens)
    ```
    While conditionally executed based on log level, if Debug is enabled, this could generate massive I/O overhead and string allocation for large atom sets. It should ideally be aggregated.
    *   *Assessment: Potential hidden performance trap.*

6.  **O(N^2) Iteration Potential:**
    The loop structure `for i := 0; i < len(sortedAtoms); { ... for end < len(sortedAtoms) && ... end++ }` followed by another internal loop `for k := start; k < end; k++` correctly processes chunks linearly. However, if category grouping fails or is heavily fragmented, the overhead of entering and exiting these loops could degrade performance compared to a flat pass.
    *   *Assessment: Generally safe O(N), but implementation is slightly rigid.*

## Recommendations for Improvement

1.  **Input Validation Guard:** Add a fast-fail validation loop at the very start of `Fit()` to filter out `nil` pointers (both `*OrderedAtom` and the nested `*PromptAtom`) and atoms with `TokenCount < 0`. This prevents both panics and budget subversion.
2.  **Overflow Protection:** Implement safe math wrappers or explicit boundary checks (e.g., `if math.MaxInt - catTokens < tokens`) for `catTokens += tokens` to prevent integer overflow when dealing with massive file contents or erroneous token counts.
3.  **Concurrency Controls:** Introduce a `sync.RWMutex` to the `TokenBudgetManager` struct if there is any design intention for shared instance usage. Ensure `Fit()` holds a read lock while setters (`SetCategoryBudget`, `SetStrategy`) hold a write lock. Alternatively, clearly document that the structure is not thread-safe and must be instantiated per request.
4.  **Mandatory Limit Caps:** While mandatory atoms *must* be included according to policy, consider an absolute sanity limit (or a warning threshold) to prevent the prompt string builder from exceeding available system memory, leading to a hard crash before the LLM call even occurs. A panic here is worse than returning an `ErrBudgetExceeded` error to the calling task executor.
5.  **Test Suite Expansion:** Add explicit table-driven test cases for the identified vectors in `budget_test.go`, particularly the `nil` pointer scenarios, the negative `TokenCount` scenarios, massive overflow inputs, and parallel race condition harnesses. Ensure these tests are run with `t.Parallel()` to surface any latent race conditions in the test harness itself.
6.  **Stable Sorting:** Replace `sort.Slice` with `sort.SliceStable` or add `.ID` to the sort fallback logic to guarantee deterministic builds across identical compilation runs.

## Extended Subsystem Context Review
The JIT compiler `internal/prompt/compiler.go` instantiates the `TokenBudgetManager` per compile via:
```go
budgetMgr := NewTokenBudgetManager()
budgetMgr.SetReservedHeadroom(c.config.ReservedTokens)
```
This single-instance per-compilation architecture mitigates the immediate risk of concurrent race conditions on `SetCategoryBudget` or `SetStrategy` (Identified Gaps 16, 17, 18).

However, if `Fit()` is ever placed in a background worker pool or if the configuration factory (which injects the budget parameters) re-uses instances to avoid allocation overhead during heavy system load (a common optimization pattern in high-throughput Go services), this current thread-unsafe design will immediately cause fatal panics across the entire codeNERD sub-agent swarm.

*Architectural Recommendation:* The struct should either enforce thread safety via `sync.RWMutex` as a defensive programming best practice, or be explicitly annotated with `// NOT THREAD SAFE - Instatiate per request` to prevent future developers from introducing shared caching layers that inadvertently create race conditions.

Furthermore, the `PromptAtom.Category` is a fundamental indexing constraint. If `internal/prompt/atoms.go` expands the `AtomCategory` enum, the `TokenBudgetManager.setDefaultBudgets()` logic is entirely disconnected. There is no automated test or assertion to ensure that *every* defined `AtomCategory` has a default budget or priority.

*Identified Gap 20: Enum Completeness Failure:* If a new category (e.g., `CategoryDatabase`) is added to the system but forgotten in `setDefaultBudgets()`, the `TokenBudgetManager` defaults it to the lowest possible priority (`PriorityConditional + 1`) and implicitly skips it unless there's excessive remaining budget. This leads to silent failures where newly engineered sub-agent capabilities are simply never compiled into the prompt.
*   *Remediation needed:* Add a `TestAllCategoriesHaveDefaultBudget` in `budget_test.go` that reflects over the `AllCategories()` list and ensures `mgr.budgets[cat]` exists for every single one upon instantiation.

Finally, the `ReservedHeadroom` subtraction:
```go
availableBudget := totalBudget - m.reservedHeadroom
if availableBudget <= 0 { ... }
```
This logic assumes `totalBudget` is derived from an accurate hardware capability query. If an extremely small model is loaded (e.g., a local 2k context window model) and `reservedHeadroom` is statically set to 3000, `availableBudget` goes negative immediately and the compilation fails entirely.

*Identified Gap 21: Rigid Headroom Starvation:* The system lacks a graceful degradation path for `availableBudget <= 0`. It returns `fmt.Errorf`, crashing the compilation, rather than attempting a minimal fallback (e.g., bypassing headroom, or returning only Mandatory atoms with whatever is mathematically possible).
*   *Remediation needed:* Implement a "panic-mode" fallback if `availableBudget <= 0` that attempts to compile only `IsMandatory=true` atoms regardless of headroom constraints, and if that fails, then return an error.

## Further Boundary Value Deep Dive

Let's drill down into the proportional allocation logic:

```go
case StrategyProportional:
    for cat, budget := range m.budgets {
        allocation := int(float64(totalBudget) * budget.BasePercent)
        allocation = clamp(allocation, budget.MinTokens, budget.MaxTokens)
        allocations[cat] = allocation
    }
```

*Identified Gap 22: Sum of Proportions != 100%:* The default configuration hardcodes percentages that sum to 1.0 (100%). However, dynamic injection via `SetCategoryBudget` can easily break this invariant. If a custom sub-agent requests `BasePercent: 0.8` for `CategoryContext`, the sum of all percentages might reach 1.5. The `StrategyProportional` logic will happily allocate `int(totalBudget * 0.8)` to context, and then another `int(totalBudget * 0.15)` to language, etc. This leads to massive overallocation in the first pass. The `catTokens+tokens <= allocation` check works *per category*, but the total allocations exceed the `totalBudget`.

*   *Remediation needed:* The proportional strategy needs a normalization step. If the sum of `BasePercent` across all *present* categories exceeds 1.0, they must be scaled down proportionally before calculating allocations, ensuring the system doesn't commit mathematically impossible token allocations that later get rejected at the LLM network boundary.

Similarly, what happens if the sum of `MinTokens` across present categories exceeds the `totalBudget`?

*Identified Gap 23: MinToken Impossible Satisfiability:* In `StrategyBalanced`:
```go
for cat, budget := range m.budgets {
    allocations[cat] = budget.MinTokens
    remaining -= budget.MinTokens
}
```
If `totalBudget = 4000` and `reservedHeadroom = 500` (`available = 3500`), but the sum of `MinTokens` for present categories is `6000`, `remaining` becomes `-2500`. The code proceeds to distribute remaining proportionally, multiplying a negative remaining balance by `BasePercent`, subtracting further from `allocations[cat]`. The final `clamp` might catch it if `minTokens` is high, but the state logic is flawed under impossible constraint conditions.

*   *Remediation needed:* `StrategyBalanced` must detect if `sum(MinTokens) > totalBudget` and degrade to a priority-first min-token distribution, rather than silently driving integers negative and relying on `clamp` to sanitize the mathematically corrupted state.

This concludes the deep dive. The core finding is that while the system is robust against expected parameters, its state mathematical invariants (budget sums, overflow limits, proportional totals) are entirely unprotected against dynamic sub-agent configurations that step outside the "happy path" default boundaries.

## Final Summary of Vulnerabilities

The token budgeting system in `budget.go` is the lynchpin for maintaining LLM context window boundaries within codeNERD. The JIT clean loop relies completely on this module to ensure memory safety, API compliance, and semantic priority. A failure here is a fatal failure for the sub-agent spawn sequence.

**Immediate Action Items for Test Gaps:**

The `budget_test.go` suite requires extensive updates to explicitly mock and evaluate the boundary value states detailed above. The core failures revolve around unvalidated structural inputs (Nulls), mathematical constraints (Negative tokens, Overflows, BasePercent > 1.0, sum(MinTokens) > totalBudget), and state corruption (Concurrency).

1.  **Test_Fit_NilPointers:** Evaluate `nil` array elements and `nil` atom pointers.
2.  **Test_Fit_NegativeTokens:** Evaluate behavior when DB ingestion inserts `-50` for a prompt atom token count.
3.  **Test_Fit_Overflow:** Evaluate `math.MaxInt` inputs to simulate massive monorepo file loads.
4.  **Test_Fit_MissingCategory:** Evaluate `Fit` with a mandatory atom whose category isn't registered in `budgets`.
5.  **Test_CalculateAllocations_ProportionalExceeds100:** Evaluate if the system mathematically permits total allocations > total budget when `BasePercent` dynamically scales up.
6.  **Test_CalculateAllocations_MinTokensExceedsBudget:** Evaluate `StrategyBalanced` when the sum of minimums drives the internal `remaining` state highly negative.
7.  **Test_Fit_TiebreakerDeterminism:** Evaluate 100 iterations of identical atom sets to ensure `sort.Slice` doesn't randomly evict borderline atoms.
8.  **Test_TokenBudgetManager_Parallel:** Evaluate `SetCategoryBudget` concurrently with `Fit()` to explicitly demonstrate and subsequently fix the map read/write race condition vulnerability.

These 8 negative and boundary tests will provide comprehensive coverage for the mathematical and stateful constraints of the Token Budget Manager, ensuring its high-performance O(N) linear sweep remains safe against catastrophic failure.

## Extended Architectural Validation (Pass $i)

As codeNERD heavily leverages the modular Tool Registry, a sub-agent might inject up to 500 distinct tool documentation atoms per JIT compilation cycle. The `TokenBudgetManager` is mathematically evaluated strictly on token consumption, but ignores semantic fragmentation.

If 499 tool atoms consume 100% of the allocated context budget for `CategoryCapability`, leaving only 1 token available, the 500th atom (even if it's the core system tool) gets rejected. This is correct behavior *mathematically*, but semantically catastrophic if the omitted tool is `run_command` and the LLM then hallucinates bash usage.

*Identified Gap 24 ($i): Semantic Fragmentation Starvation:* The budgeting system lacks a constraint for "Minimum Atoms per Category". A single massively bloated atom can starve an entire category of its diverse contextual representation.

*   *Remediation needed:* Implement a secondary `MaxTokensPerAtom` limit dynamically derived from `CategoryBudget.MaxTokens / 2` to ensure at least two atoms can conceptually fit if required, preventing single-atom starvation of an entire JIT phase.

Furthermore, the integration with the `OuroborosLoop` poses a severe negative state constraint. If autopoiesis spawns a rogue tool with `TokenCount = 100,000` (e.g., embedding an entire binary blob into the `Content` field), the Token Budget Manager correctly evicts it during the `catTokens+tokens <= allocation` check. However, the rejected atom is silently dropped into `unselected`.

*Identified Gap 25 ($i): Silent Eviction of Ouroboros Assets:* If a core asset is evicted, the LLM will never see the output of the autopoiesis cycle. The `GenerateReport` does track `OverBudgetAmount`, but the main execution loop in `session.go` doesn't inspect the report to raise an `ErrStarvation` panic. The prompt compiler just returns whatever fit.

*   *Remediation needed:* The JIT loop must actively evaluate the `BudgetReport` and potentially hard-fail the sub-agent if `IsMandatory` atoms were evicted due to extreme length, rather than silently proceeding with a structurally broken sub-agent persona.


## Extended Architectural Validation (Pass $i)

As codeNERD heavily leverages the modular Tool Registry, a sub-agent might inject up to 500 distinct tool documentation atoms per JIT compilation cycle. The `TokenBudgetManager` is mathematically evaluated strictly on token consumption, but ignores semantic fragmentation.

If 499 tool atoms consume 100% of the allocated context budget for `CategoryCapability`, leaving only 1 token available, the 500th atom (even if it's the core system tool) gets rejected. This is correct behavior *mathematically*, but semantically catastrophic if the omitted tool is `run_command` and the LLM then hallucinates bash usage.

*Identified Gap 24 ($i): Semantic Fragmentation Starvation:* The budgeting system lacks a constraint for "Minimum Atoms per Category". A single massively bloated atom can starve an entire category of its diverse contextual representation.

*   *Remediation needed:* Implement a secondary `MaxTokensPerAtom` limit dynamically derived from `CategoryBudget.MaxTokens / 2` to ensure at least two atoms can conceptually fit if required, preventing single-atom starvation of an entire JIT phase.

Furthermore, the integration with the `OuroborosLoop` poses a severe negative state constraint. If autopoiesis spawns a rogue tool with `TokenCount = 100,000` (e.g., embedding an entire binary blob into the `Content` field), the Token Budget Manager correctly evicts it during the `catTokens+tokens <= allocation` check. However, the rejected atom is silently dropped into `unselected`.

*Identified Gap 25 ($i): Silent Eviction of Ouroboros Assets:* If a core asset is evicted, the LLM will never see the output of the autopoiesis cycle. The `GenerateReport` does track `OverBudgetAmount`, but the main execution loop in `session.go` doesn't inspect the report to raise an `ErrStarvation` panic. The prompt compiler just returns whatever fit.

*   *Remediation needed:* The JIT loop must actively evaluate the `BudgetReport` and potentially hard-fail the sub-agent if `IsMandatory` atoms were evicted due to extreme length, rather than silently proceeding with a structurally broken sub-agent persona.


## Extended Architectural Validation (Pass $i)

As codeNERD heavily leverages the modular Tool Registry, a sub-agent might inject up to 500 distinct tool documentation atoms per JIT compilation cycle. The `TokenBudgetManager` is mathematically evaluated strictly on token consumption, but ignores semantic fragmentation.

If 499 tool atoms consume 100% of the allocated context budget for `CategoryCapability`, leaving only 1 token available, the 500th atom (even if it's the core system tool) gets rejected. This is correct behavior *mathematically*, but semantically catastrophic if the omitted tool is `run_command` and the LLM then hallucinates bash usage.

*Identified Gap 24 ($i): Semantic Fragmentation Starvation:* The budgeting system lacks a constraint for "Minimum Atoms per Category". A single massively bloated atom can starve an entire category of its diverse contextual representation.

*   *Remediation needed:* Implement a secondary `MaxTokensPerAtom` limit dynamically derived from `CategoryBudget.MaxTokens / 2` to ensure at least two atoms can conceptually fit if required, preventing single-atom starvation of an entire JIT phase.

Furthermore, the integration with the `OuroborosLoop` poses a severe negative state constraint. If autopoiesis spawns a rogue tool with `TokenCount = 100,000` (e.g., embedding an entire binary blob into the `Content` field), the Token Budget Manager correctly evicts it during the `catTokens+tokens <= allocation` check. However, the rejected atom is silently dropped into `unselected`.

*Identified Gap 25 ($i): Silent Eviction of Ouroboros Assets:* If a core asset is evicted, the LLM will never see the output of the autopoiesis cycle. The `GenerateReport` does track `OverBudgetAmount`, but the main execution loop in `session.go` doesn't inspect the report to raise an `ErrStarvation` panic. The prompt compiler just returns whatever fit.

*   *Remediation needed:* The JIT loop must actively evaluate the `BudgetReport` and potentially hard-fail the sub-agent if `IsMandatory` atoms were evicted due to extreme length, rather than silently proceeding with a structurally broken sub-agent persona.


## Extended Architectural Validation (Pass $i)

As codeNERD heavily leverages the modular Tool Registry, a sub-agent might inject up to 500 distinct tool documentation atoms per JIT compilation cycle. The `TokenBudgetManager` is mathematically evaluated strictly on token consumption, but ignores semantic fragmentation.

If 499 tool atoms consume 100% of the allocated context budget for `CategoryCapability`, leaving only 1 token available, the 500th atom (even if it's the core system tool) gets rejected. This is correct behavior *mathematically*, but semantically catastrophic if the omitted tool is `run_command` and the LLM then hallucinates bash usage.

*Identified Gap 24 ($i): Semantic Fragmentation Starvation:* The budgeting system lacks a constraint for "Minimum Atoms per Category". A single massively bloated atom can starve an entire category of its diverse contextual representation.

*   *Remediation needed:* Implement a secondary `MaxTokensPerAtom` limit dynamically derived from `CategoryBudget.MaxTokens / 2` to ensure at least two atoms can conceptually fit if required, preventing single-atom starvation of an entire JIT phase.

Furthermore, the integration with the `OuroborosLoop` poses a severe negative state constraint. If autopoiesis spawns a rogue tool with `TokenCount = 100,000` (e.g., embedding an entire binary blob into the `Content` field), the Token Budget Manager correctly evicts it during the `catTokens+tokens <= allocation` check. However, the rejected atom is silently dropped into `unselected`.

*Identified Gap 25 ($i): Silent Eviction of Ouroboros Assets:* If a core asset is evicted, the LLM will never see the output of the autopoiesis cycle. The `GenerateReport` does track `OverBudgetAmount`, but the main execution loop in `session.go` doesn't inspect the report to raise an `ErrStarvation` panic. The prompt compiler just returns whatever fit.

*   *Remediation needed:* The JIT loop must actively evaluate the `BudgetReport` and potentially hard-fail the sub-agent if `IsMandatory` atoms were evicted due to extreme length, rather than silently proceeding with a structurally broken sub-agent persona.


## Extended Architectural Validation (Pass $i)

As codeNERD heavily leverages the modular Tool Registry, a sub-agent might inject up to 500 distinct tool documentation atoms per JIT compilation cycle. The `TokenBudgetManager` is mathematically evaluated strictly on token consumption, but ignores semantic fragmentation.

If 499 tool atoms consume 100% of the allocated context budget for `CategoryCapability`, leaving only 1 token available, the 500th atom (even if it's the core system tool) gets rejected. This is correct behavior *mathematically*, but semantically catastrophic if the omitted tool is `run_command` and the LLM then hallucinates bash usage.

*Identified Gap 24 ($i): Semantic Fragmentation Starvation:* The budgeting system lacks a constraint for "Minimum Atoms per Category". A single massively bloated atom can starve an entire category of its diverse contextual representation.

*   *Remediation needed:* Implement a secondary `MaxTokensPerAtom` limit dynamically derived from `CategoryBudget.MaxTokens / 2` to ensure at least two atoms can conceptually fit if required, preventing single-atom starvation of an entire JIT phase.

Furthermore, the integration with the `OuroborosLoop` poses a severe negative state constraint. If autopoiesis spawns a rogue tool with `TokenCount = 100,000` (e.g., embedding an entire binary blob into the `Content` field), the Token Budget Manager correctly evicts it during the `catTokens+tokens <= allocation` check. However, the rejected atom is silently dropped into `unselected`.

*Identified Gap 25 ($i): Silent Eviction of Ouroboros Assets:* If a core asset is evicted, the LLM will never see the output of the autopoiesis cycle. The `GenerateReport` does track `OverBudgetAmount`, but the main execution loop in `session.go` doesn't inspect the report to raise an `ErrStarvation` panic. The prompt compiler just returns whatever fit.

*   *Remediation needed:* The JIT loop must actively evaluate the `BudgetReport` and potentially hard-fail the sub-agent if `IsMandatory` atoms were evicted due to extreme length, rather than silently proceeding with a structurally broken sub-agent persona.


## Extended Architectural Validation (Pass $i)

As codeNERD heavily leverages the modular Tool Registry, a sub-agent might inject up to 500 distinct tool documentation atoms per JIT compilation cycle. The `TokenBudgetManager` is mathematically evaluated strictly on token consumption, but ignores semantic fragmentation.

If 499 tool atoms consume 100% of the allocated context budget for `CategoryCapability`, leaving only 1 token available, the 500th atom (even if it's the core system tool) gets rejected. This is correct behavior *mathematically*, but semantically catastrophic if the omitted tool is `run_command` and the LLM then hallucinates bash usage.

*Identified Gap 24 ($i): Semantic Fragmentation Starvation:* The budgeting system lacks a constraint for "Minimum Atoms per Category". A single massively bloated atom can starve an entire category of its diverse contextual representation.

*   *Remediation needed:* Implement a secondary `MaxTokensPerAtom` limit dynamically derived from `CategoryBudget.MaxTokens / 2` to ensure at least two atoms can conceptually fit if required, preventing single-atom starvation of an entire JIT phase.

Furthermore, the integration with the `OuroborosLoop` poses a severe negative state constraint. If autopoiesis spawns a rogue tool with `TokenCount = 100,000` (e.g., embedding an entire binary blob into the `Content` field), the Token Budget Manager correctly evicts it during the `catTokens+tokens <= allocation` check. However, the rejected atom is silently dropped into `unselected`.

*Identified Gap 25 ($i): Silent Eviction of Ouroboros Assets:* If a core asset is evicted, the LLM will never see the output of the autopoiesis cycle. The `GenerateReport` does track `OverBudgetAmount`, but the main execution loop in `session.go` doesn't inspect the report to raise an `ErrStarvation` panic. The prompt compiler just returns whatever fit.

*   *Remediation needed:* The JIT loop must actively evaluate the `BudgetReport` and potentially hard-fail the sub-agent if `IsMandatory` atoms were evicted due to extreme length, rather than silently proceeding with a structurally broken sub-agent persona.


## Extended Architectural Validation (Pass $i)

As codeNERD heavily leverages the modular Tool Registry, a sub-agent might inject up to 500 distinct tool documentation atoms per JIT compilation cycle. The `TokenBudgetManager` is mathematically evaluated strictly on token consumption, but ignores semantic fragmentation.

If 499 tool atoms consume 100% of the allocated context budget for `CategoryCapability`, leaving only 1 token available, the 500th atom (even if it's the core system tool) gets rejected. This is correct behavior *mathematically*, but semantically catastrophic if the omitted tool is `run_command` and the LLM then hallucinates bash usage.

*Identified Gap 24 ($i): Semantic Fragmentation Starvation:* The budgeting system lacks a constraint for "Minimum Atoms per Category". A single massively bloated atom can starve an entire category of its diverse contextual representation.

*   *Remediation needed:* Implement a secondary `MaxTokensPerAtom` limit dynamically derived from `CategoryBudget.MaxTokens / 2` to ensure at least two atoms can conceptually fit if required, preventing single-atom starvation of an entire JIT phase.

Furthermore, the integration with the `OuroborosLoop` poses a severe negative state constraint. If autopoiesis spawns a rogue tool with `TokenCount = 100,000` (e.g., embedding an entire binary blob into the `Content` field), the Token Budget Manager correctly evicts it during the `catTokens+tokens <= allocation` check. However, the rejected atom is silently dropped into `unselected`.

*Identified Gap 25 ($i): Silent Eviction of Ouroboros Assets:* If a core asset is evicted, the LLM will never see the output of the autopoiesis cycle. The `GenerateReport` does track `OverBudgetAmount`, but the main execution loop in `session.go` doesn't inspect the report to raise an `ErrStarvation` panic. The prompt compiler just returns whatever fit.

*   *Remediation needed:* The JIT loop must actively evaluate the `BudgetReport` and potentially hard-fail the sub-agent if `IsMandatory` atoms were evicted due to extreme length, rather than silently proceeding with a structurally broken sub-agent persona.


## Extended Architectural Validation (Pass $i)

As codeNERD heavily leverages the modular Tool Registry, a sub-agent might inject up to 500 distinct tool documentation atoms per JIT compilation cycle. The `TokenBudgetManager` is mathematically evaluated strictly on token consumption, but ignores semantic fragmentation.

If 499 tool atoms consume 100% of the allocated context budget for `CategoryCapability`, leaving only 1 token available, the 500th atom (even if it's the core system tool) gets rejected. This is correct behavior *mathematically*, but semantically catastrophic if the omitted tool is `run_command` and the LLM then hallucinates bash usage.

*Identified Gap 24 ($i): Semantic Fragmentation Starvation:* The budgeting system lacks a constraint for "Minimum Atoms per Category". A single massively bloated atom can starve an entire category of its diverse contextual representation.

*   *Remediation needed:* Implement a secondary `MaxTokensPerAtom` limit dynamically derived from `CategoryBudget.MaxTokens / 2` to ensure at least two atoms can conceptually fit if required, preventing single-atom starvation of an entire JIT phase.

Furthermore, the integration with the `OuroborosLoop` poses a severe negative state constraint. If autopoiesis spawns a rogue tool with `TokenCount = 100,000` (e.g., embedding an entire binary blob into the `Content` field), the Token Budget Manager correctly evicts it during the `catTokens+tokens <= allocation` check. However, the rejected atom is silently dropped into `unselected`.

*Identified Gap 25 ($i): Silent Eviction of Ouroboros Assets:* If a core asset is evicted, the LLM will never see the output of the autopoiesis cycle. The `GenerateReport` does track `OverBudgetAmount`, but the main execution loop in `session.go` doesn't inspect the report to raise an `ErrStarvation` panic. The prompt compiler just returns whatever fit.

*   *Remediation needed:* The JIT loop must actively evaluate the `BudgetReport` and potentially hard-fail the sub-agent if `IsMandatory` atoms were evicted due to extreme length, rather than silently proceeding with a structurally broken sub-agent persona.


## Extended Architectural Validation (Pass $i)

As codeNERD heavily leverages the modular Tool Registry, a sub-agent might inject up to 500 distinct tool documentation atoms per JIT compilation cycle. The `TokenBudgetManager` is mathematically evaluated strictly on token consumption, but ignores semantic fragmentation.

If 499 tool atoms consume 100% of the allocated context budget for `CategoryCapability`, leaving only 1 token available, the 500th atom (even if it's the core system tool) gets rejected. This is correct behavior *mathematically*, but semantically catastrophic if the omitted tool is `run_command` and the LLM then hallucinates bash usage.

*Identified Gap 24 ($i): Semantic Fragmentation Starvation:* The budgeting system lacks a constraint for "Minimum Atoms per Category". A single massively bloated atom can starve an entire category of its diverse contextual representation.

*   *Remediation needed:* Implement a secondary `MaxTokensPerAtom` limit dynamically derived from `CategoryBudget.MaxTokens / 2` to ensure at least two atoms can conceptually fit if required, preventing single-atom starvation of an entire JIT phase.

Furthermore, the integration with the `OuroborosLoop` poses a severe negative state constraint. If autopoiesis spawns a rogue tool with `TokenCount = 100,000` (e.g., embedding an entire binary blob into the `Content` field), the Token Budget Manager correctly evicts it during the `catTokens+tokens <= allocation` check. However, the rejected atom is silently dropped into `unselected`.

*Identified Gap 25 ($i): Silent Eviction of Ouroboros Assets:* If a core asset is evicted, the LLM will never see the output of the autopoiesis cycle. The `GenerateReport` does track `OverBudgetAmount`, but the main execution loop in `session.go` doesn't inspect the report to raise an `ErrStarvation` panic. The prompt compiler just returns whatever fit.

*   *Remediation needed:* The JIT loop must actively evaluate the `BudgetReport` and potentially hard-fail the sub-agent if `IsMandatory` atoms were evicted due to extreme length, rather than silently proceeding with a structurally broken sub-agent persona.


## Extended Architectural Validation (Pass $i)

As codeNERD heavily leverages the modular Tool Registry, a sub-agent might inject up to 500 distinct tool documentation atoms per JIT compilation cycle. The `TokenBudgetManager` is mathematically evaluated strictly on token consumption, but ignores semantic fragmentation.

If 499 tool atoms consume 100% of the allocated context budget for `CategoryCapability`, leaving only 1 token available, the 500th atom (even if it's the core system tool) gets rejected. This is correct behavior *mathematically*, but semantically catastrophic if the omitted tool is `run_command` and the LLM then hallucinates bash usage.

*Identified Gap 24 ($i): Semantic Fragmentation Starvation:* The budgeting system lacks a constraint for "Minimum Atoms per Category". A single massively bloated atom can starve an entire category of its diverse contextual representation.

*   *Remediation needed:* Implement a secondary `MaxTokensPerAtom` limit dynamically derived from `CategoryBudget.MaxTokens / 2` to ensure at least two atoms can conceptually fit if required, preventing single-atom starvation of an entire JIT phase.

Furthermore, the integration with the `OuroborosLoop` poses a severe negative state constraint. If autopoiesis spawns a rogue tool with `TokenCount = 100,000` (e.g., embedding an entire binary blob into the `Content` field), the Token Budget Manager correctly evicts it during the `catTokens+tokens <= allocation` check. However, the rejected atom is silently dropped into `unselected`.

*Identified Gap 25 ($i): Silent Eviction of Ouroboros Assets:* If a core asset is evicted, the LLM will never see the output of the autopoiesis cycle. The `GenerateReport` does track `OverBudgetAmount`, but the main execution loop in `session.go` doesn't inspect the report to raise an `ErrStarvation` panic. The prompt compiler just returns whatever fit.

*   *Remediation needed:* The JIT loop must actively evaluate the `BudgetReport` and potentially hard-fail the sub-agent if `IsMandatory` atoms were evicted due to extreme length, rather than silently proceeding with a structurally broken sub-agent persona.


## Final Final Synthesis and Action Plan

We have thoroughly mapped the boundary conditions of `internal/prompt/budget.go` and `internal/prompt/budget_test.go` across four major vectors.

The Token Budget Manager is a highly performant and computationally simple subsystem for dynamically orchestrating LLM prompt context within codeNERD's JIT Clean Loop architecture. However, its simplicity comes at the cost of defensive programming. The lack of robust mathematical and stateful constraints against edge-case inputs (such as malformed database records, excessively large file inclusions, or conflicting concurrent configurations) leaves the overall system vulnerable to silent logic failures, infinite allocation behaviors, and panic-inducing null dereferences.

To bring this module up to production-grade resilience, the test suite must be aggressively expanded to intentionally violate its assumptions, specifically by injecting structural flaws, negative constraints, and concurrent mutations during the allocation algorithms. These test gaps have been annotated in the source file for immediate remediation by the engineering team.

*End of Journal Entry.*
