---
surface: "Context_Activation_Pipeline"
mode: "pipeline"
subsystems_tested: ["context", "core", "session", "mangle", "virtualstore"]
blast_radius: "critical"
remediated: false
---

# Spreading Activation (Context) ↔ Kernel ↔ Session Pipeline Analysis

## 1. System Interaction Map

The Spreading Activation pipeline connects the user's initial intent (Session/Perception) to the Mangle Kernel's logic-driven fact store, and finally pages the selected context back to the LLM.

**Key Interactions and Cross-Boundary Calls:**
1. **Perception -> Kernel:** `transducer.ParseIntentWithContext()` produces `user_intent/5` facts, which are injected into the Kernel via `kernel.Assert(facts)`.
2. **Session -> Context:** The Session executor calls `context.NewActivationEngine(config)` at initialization.
3. **Context -> Kernel:** `ActivationEngine.ScoreFacts(facts, currentIntent)` receives all active EDB facts from the Kernel.
4. **Kernel -> Context (Graph Building):** `buildSymbolGraphLocked(facts)` iterates over `dependency_link/3` and `symbol_graph/2` facts to build the in-memory dependency graph.
5. **Context -> Session (Context Paging):** The engine returns a sorted `[]ScoredFact`. The Session orchestrator calls `context_pager.go` to compress and truncate these facts based on the token budget before feeding them to the JIT Prompt Compiler.
6. **Session -> Context (Feedback):** Following a turn, `feedbackStore.Record(...)` is invoked to update the predicate usefulness scores.
7. **VirtualStore -> Kernel -> Context:** When a tool (e.g., `graph_query_result`) produces new relationships, the VirtualStore injects them as virtual facts. If `Mangle` derives new rules, they cascade into `ScoreFacts`.

## 2. Contract Analysis

The implicit contracts that govern this pipeline are numerous and fragile:

- **The Atom/String Dissonance Contract:** The Kernel schema defines `dependency_link` arguments as Atoms (`ast.Name`). However, `ScoreFacts` blindly casts `f.Args[0].(string)`. If the Kernel sends Mangle Strings instead of Mangle Atoms (due to a permissive rule), the cast in Go might fail or succeed with quotes attached, breaking graph traversal.
- **The Immutability Contract:** `ScoreFacts` receives a slice of `core.Fact` from the Kernel. The Kernel assumes this slice is read-only. If `ActivationEngine` sorts or mutates this slice in place, it corrupts the Kernel's internal query caches and breaks fixpoint evaluation for subsequent Mangle queries.
- **The Graph Reset Contract:** `buildSymbolGraphLocked` promises to rebuild the graph cleanly on each score to avoid unbounded map growth. However, it explicitly merges `preservedDeps` (edges added via `AddDependency`). If `AddDependency` is called concurrently with `ScoreFacts`, the map read/write race will crash the Go runtime.
- **The Fail-Closed Constraint:** If `ScoreFacts` panics, the Session Executor must catch it, log a diagnostic fact, and proceed with a degraded context. It must not crash the main orchestrator loop, which would abandon all active Campaign phases.

## 3. Failure Mode Enumeration

### Temporal Failures
1. **Graph Traversal Timeout:** `ScoreFacts` enters an infinite loop due to circular dependencies in `preservedDeps`, exceeding the 50ms budget.
2. **Context Cancellation Mid-Scoring:** A user aborts the request, cancelling the context. If `ScoreFacts` ignores the context, it burns CPU and delays goroutine cleanup.
3. **Session Desync:** `MarkNewFacts` runs after `ClearState` due to asynchronous tool results arriving late, resulting in phantom facts in the next turn.

### Semantic Failures
4. **Mangle Type Confusion:** `dependency_link(/a, /b, "path")` vs `dependency_link("a", "b", "path")`. The engine fails to match strings, returning 0 active context facts.
5. **Phase-Aware Poisoning:** `CampaignActivationContext` contains massive `PhaseGoals` strings that overwhelm the activation score heuristics, causing every fact to score 100.
6. **Priority Fallback Failure:** `SetCorpusPriorities` is provided a malformed map, overriding the safe `config.PredicatePriorities` with negative scores.

### Ordering Failures
7. **Fact Retraction Race:** The Kernel retracts a fact while `ScoreFacts` is concurrently iterating over the slice (if the slice is not properly cloned by the Kernel).
8. **Feedback Loop Inversion:** The Session records feedback for an intent *before* `ScoreFacts` processes it, causing the intent to use future knowledge to score past facts.

### Partial Failures
9. **Corrupt VirtualStore Output:** `graph_query_result` returns 99% valid facts and 1 fact with missing arity. `ScoreFacts` panics on `f.Args[2]` index out of bounds.

### State Corruption Failures
10. **Map Concurrent Write Panic:** `Session.GetSessionStats()` reads maps while `ScoreFacts` rebuilds them.
11. **Slice Backing Array Overwrite:** Engine modifies `facts[i].Score` directly on the Kernel's EDB slice.

## 4. Adversarial Scenario Design

1. **Scenario: Mangle Type Disjoint Validation (Contract Violation)**
   - *Mechanism:* Inject `dependency_link` with `ast.String` instead of `ast.Name`.
   - *Expected:* Context engine should detect type mismatch and either coerce or reject safely without panic.
   - *Severity:* P1

2. **Scenario: Missing Arity on Virtual Fact (Contract Violation)**
   - *Mechanism:* VirtualStore injects `dependency_link(A, B)` (arity 2 instead of 3).
   - *Expected:* `buildSymbolGraphLocked` bounds checks arguments and safely ignores malformed facts.
   - *Severity:* P0

3. **Scenario: Concurrent State Reset (Contract Violation)**
   - *Mechanism:* Fire `ClearState()` concurrently while `ScoreFacts()` is processing 10,000 facts.
   - *Expected:* Mutex prevents partial graph builds; `ScoreFacts` either completes before clear or uses clean state.
   - *Severity:* P0

4. **Scenario: The Back-Reference Ghost Fact (Contract Violation)**
   - *Mechanism:* Provide a `BackReferenceActivationContext` for facts retracted by the Kernel in the previous turn.
   - *Expected:* Engine handles missing facts gracefully without nil pointer dereference.
   - *Severity:* P2

5. **Scenario: In-Place EDB Slice Mutation (State Corruption)**
   - *Mechanism:* Check if `ScoreFacts` mutates the backing array of the provided fact slice.
   - *Expected:* Kernel's internal representation remains pristine. Engine must allocate a new `[]ScoredFact` slice.
   - *Severity:* P0

6. **Scenario: `GetSessionStats` RLock Race (State Corruption)**
   - *Mechanism:* Rapidly poll `GetSessionStats` while `ScoreFacts` mutates maps.
   - *Expected:* `RLock` / `Lock` prevents panic. Statistics remain consistent.
   - *Severity:* P1

7. **Scenario: Cross-Session Ghosting (State Corruption)**
   - *Mechanism:* Do not call `NewSession()`. Inject facts from Session A, run Session B.
   - *Expected:* `sessionFacts` leaks. Test verifies the leak mechanism.
   - *Severity:* P2

8. **Scenario: The 100k Graph Bomb (Resource Exhaustion)**
   - *Mechanism:* Inject 100,000 interconnected `dependency_link` facts.
   - *Expected:* `ScoreFacts` completes within reasonable bounds or truncates appropriately. No OOM.
   - *Severity:* P1

9. **Scenario: Infinite Dependency Cycle (Resource Exhaustion)**
   - *Mechanism:* `A -> B -> C -> A` injected via VirtualStore.
   - *Expected:* Spreading activation decays or halts; no infinite recursion or stack overflow.
   - *Severity:* P1

10. **Scenario: Cancellation During Activation (Temporal Failure)**
    - *Mechanism:* Cancel the context exactly mid-way through `ScoreFacts`.
    - *Expected:* The engine halts processing and returns an error or partial list immediately.
    - *Severity:* P2

11. **Scenario: Spreading Activation Budget Exceeded (Temporal Failure)**
    - *Mechanism:* Block the Go scheduler to simulate a 10-second `ScoreFacts` execution.
    - *Expected:* The caller (Session) applies a timeout and proceeds without optimal context rather than hanging.
    - *Severity:* P1

12. **Scenario: Recency Decay Desync (Temporal Failure)**
    - *Mechanism:* Fast-forward the mock clock by 24 hours between `MarkNewFacts` and `ScoreFacts`.
    - *Expected:* Recency score drops to exactly 0, but facts are not dropped unless explicitly retracted.
    - *Severity:* P2

13. **Scenario: Cascading Panic on Nil Intent (Cascading Failure)**
    - *Mechanism:* Pass `nil` for `currentIntent` to `ScoreFacts`.
    - *Expected:* Engine handles missing intent without panic, applying default scores.
    - *Severity:* P0

14. **Scenario: Panic Recovery in Pager (Cascading Failure)**
    - *Mechanism:* Force a panic inside `buildSymbolGraphLocked`.
    - *Expected:* The panic is caught (either by engine or session pager) and does not kill the Orhcestrator.
    - *Severity:* P1

15. **Scenario: Malformed Corpus Priorities (Cascading Failure)**
    - *Mechanism:* `SetCorpusPriorities` loads `{ "dependency_link": -999999 }`.
    - *Expected:* Facts sink to bottom, but engine continues to operate and downstream LLM survives with empty context.
    - *Severity:* P2

16. **Scenario: The Perfect Pipeline (End-to-End Data Integrity)**
    - *Mechanism:* `user_intent` -> VirtualStore graph output -> `ScoreFacts` -> Top 1 Context.
    - *Expected:* Fact survives intact and unmodified from origin to top-ranked output.
    - *Severity:* P3

## 5. Detailed Architectural Deep Dive: Spreading Activation

The logic-driven context mechanism operates on the assumption that the `Mangle` engine provides an eventually consistent, sound fixpoint graph. However, the Context `ActivationEngine` uses an imperative graph traversal (`buildSymbolGraphLocked`) over the declarative facts. This creates a severe impedance mismatch.

### Subsystem Boundaries Exposed
1. **The Mangle Tuple to Go Map Conversion**
   Mangle rules (like `schemas_analysis.mg`) define facts as ordered tuples: `dependency_link(Caller, Callee, Path)`. When `kernel.Assert` executes, these are ingested into an internal trie. When `ScoreFacts` runs, it must unpack `f.Args[0]` and `f.Args[1]`.
   - *Risk:* If the arity of `dependency_link` is relaxed in a later `Mangle` update (e.g., adding a 4th argument for weight), `ScoreFacts` ignores it. If it shrinks, `ScoreFacts` panics.
2. **The Session to Context Lifecycle**
   The Session orchestrator treats context as ephemeral per-turn, while `ActivationEngine` maintains a stateful `factTimestamps` map for recency decay.
   - *Risk:* If `NewSession` is skipped (e.g., in subagent delegation loops), the recency decay is never reset. Old facts accumulate artificially high scores.
3. **The Pager to Token Budget Manager Constraint**
   `ScoreFacts` returns thousands of scored facts. The pager must serialize these into prompt tokens.
   - *Risk:* If the budget is exact, facts that serialize to multi-line strings can straddle the token boundary. The tokenizer truncates the string, resulting in syntactically invalid Mangle atoms in the JIT prompt (e.g., `file_content(/path, "partia...)`).

### Cascading Failure Blast Radius Analysis

Let's examine what happens when the TokenBudgetManager aggressively truncates `Context` facts.
1. `ScoreFacts` identifies `file_content` as highest priority (score 100).
2. `file_content` contains a 5,000 token payload.
3. The Pager allows it, but it hits the TokenBudget limit.
4. The Pager truncates the string mid-byte.
5. The LLM receives invalid JSON or unclosed strings in the prompt.
6. The LLM ignores the corrupted context and hallucinates a tool call.
7. The `VirtualStore` rejects the hallucinated tool call.
8. The `TDDLoop` escalates the rejection.
9. Because the context is permanently poisoned (the same file will be pulled next turn), the loop exhausts retries.
10. The `Session` aborts the `Task`, failing the `Campaign` Phase.

### Extended Threat Modeling: Adversarial Interactions

- **Threat Agent:** A malicious external workspace file.
- **Vector:** The VirtualStore `file_content` extraction.
- **Exploit:** A file contains text crafted to look like `dependency_link` declarations. If the extraction pipeline accidentally parses file contents as atoms, it poisons the Mangle EDB.
- **Impact:** The Spreading Activation engine builds a graph directed by the malicious file, hiding real context and forcing the LLM to only see the adversary's chosen files.

### Failure Scenario: The Ouroboros Retraction Race
When the `Kernel` retracts a fact, it relies on slice filtering. If `ScoreFacts` has cached the slice pointer, it might read retracted facts.
- *Condition:* A `TDDLoop` retracts a failed test state exactly as `ScoreFacts` begins traversal.
- *Result:* `ScoreFacts` elevates the failed test state to the LLM. The LLM attempts to fix a test that has already been fixed, applying a patch that breaks the build again.

### Deep Evaluation: IssueContext Activation
When an issue is provided (e.g. `GH-1234`), the `ActivationEngine` uses `IssueActivationContext`.
- **The Dependency Neighbor Contract:** Tier 3 relevance boosts imports and dependencies of Tier 1 files.
- *Risk:* If a highly connected utility file (`utils.go`) is mentioned in the issue, Tier 3 boosting pulls in the entire codebase. This effectively flattens the scoring curve, making Spreading Activation behave like a random selection. The TokenBudgetManager will blindly truncate the results, often dropping the most critical Tier 1 files.

### Deep Evaluation: BackReference Ghost Facts
- **Mechanism:** `BackReferenceActivationContext` targets facts that were present in previous turns.
- *Risk:* The Kernel is monotonic during a turn, but mutable across turns. If the `VirtualStore` retracts a fact (e.g., a file is deleted or a process exits), but the user refers back to it ("what did that process output?"), the `ActivationEngine` attempts to boost a fact that no longer exists in the `[]core.Fact` EDB.
- *Failure Mode:* The engine either panics trying to find the missing fact, or assigns a zero score, causing the LLM to hallucinate the historical state instead of admitting it's gone.

### Deep Evaluation: The Feedback Store Inversion
- **Mechanism:** The `Session` records feedback on which predicates were useful for a given intent. The `ActivationEngine` reads this via `feedbackStore`.
- *Risk:* The feedback store is updated *after* a turn completes. If a subagent spawns a concurrent sub-task, both tasks might read stale feedback, or write conflicting feedback.
- *Failure Mode:* Thread contention on the feedback store maps can cause priority inversion, where explicitly downvoted predicates (e.g., `build_error` after a successful build) are still artificially boosted by concurrent readers.

### Deep Evaluation: The Symbol Graph Cache Invalidation
- **Mechanism:** `buildSymbolGraphLocked` rebuilds the graph, but preserves explicitly added dependencies (`preservedDeps`).
- *Risk:* There is no TTL or eviction policy for `preservedDeps`.
- *Failure Mode:* In a long-running `Campaign` phase (e.g., thousands of test-fix loops), the `preservedDeps` map grows unboundedly. The 50ms Spreading Activation budget is consumed entirely by map merging, causing the pipeline to timeout.

### Deep Evaluation: JIT Compiler Boundary
- **Mechanism:** `context_pager.go` compresses the `[]ScoredFact` into a text block for the JIT compiler.
- *Risk:* The `ToAtom()` or `String()` representations of facts can contain unescaped quotes or newlines if the `VirtualStore` didn't sanitize them.
- *Failure Mode:* The JIT compiler constructs a Mangle program containing syntax errors. The Mangle parser fails, and the entire `Session` crashes out with a fatal `ErrParseFailure`.

### Deep Evaluation: Activation Threshold Sensitivity
- **Mechanism:** The `ActivationEngine` filters out facts below a certain threshold.
- *Risk:* The threshold is statically configured in `CompressorConfig`. In dynamic scenarios, such as exploring a large legacy codebase, the baseline activation of all facts might drop below this threshold due to extreme fan-out.
- *Failure Mode:* The LLM receives 0 facts despite the graph containing millions of relevant nodes, causing a complete loss of grounding.

### Deep Evaluation: VirtualStore Fact Retraction Delay
- **Mechanism:** Virtual facts are evaluated lazily.
- *Risk:* If a file is deleted on disk, the `VirtualStore` might still return the cached `file_content` fact if the cache isn't properly invalidated.
- *Failure Mode:* The `ActivationEngine` boosts a non-existent file. The LLM attempts to parse or edit it, leading to a `404 Not Found` equivalent error deep in the tactile execution phase, wasting expensive API tokens.

### Deep Evaluation: Multi-Agent Context Bleed
- **Mechanism:** The `Session` manages isolation, but the `Kernel` is shared.
- *Risk:* If a subagent leaks a temporary `user_intent` fact (e.g., missing a `Retract` call), the `ActivationEngine` will incorporate it into the scoring for all subsequent agents.
- *Failure Mode:* A C++ expert agent starts receiving activation boosts for Python files because a previous Python agent's intent was left in the EDB.

### Deep Evaluation: The Token Reserve Overlap
- **Mechanism:** `TokenBudget` divides capacity into `CoreReserve`, `AtomReserve`, etc.
- *Risk:* The `ActivationEngine` does not know which reserve a fact belongs to when scoring it.
- *Failure Mode:* If 90% of the highly scored facts belong to the `WorkingReserve`, but the `AtomReserve` is the only one with capacity, the high-value facts are dropped, and low-value atoms fill the prompt.

### Deep Evaluation: Campaign Risk Scoring Interference
- **Mechanism:** The `Campaign` orchestrator evaluates risk gates before execution.
- *Risk:* The risk evaluation uses the *unfiltered* EDB, while the LLM uses the *filtered* context.
- *Failure Mode:* A risk gate blocks execution based on a critical fact (e.g., `security_violation`), but that fact scored below the activation threshold and was hidden from the LLM. The LLM cannot understand why it is blocked and enters an infinite retry loop trying the same action.

### Deep Evaluation: Tactile Executor Desync
- **Mechanism:** The `TactileExecutor` mutates the filesystem, which should implicitly update the `VirtualStore` and subsequently the `ActivationEngine`.
- *Risk:* Filesystem operations are asynchronous. If the `Session` triggers `ScoreFacts` immediately after a file write, the `VirtualStore` might still be reading stale disk caches.
- *Failure Mode:* The `ActivationEngine` scores the old file content. The LLM verifies the change failed, and enters a recursive repair loop, ultimately mangling the file further.

### Deep Evaluation: Semantic Compressor Artifacts
- **Mechanism:** The `SemanticCompressor` summarizes old context to save tokens.
- *Risk:* The compressor output is injected back into the context graph as a new synthetic fact.
- *Failure Mode:* If the `ActivationEngine` assigns a high score to both the original raw facts (if they haven't decayed enough) and the compressed summary, the LLM receives duplicate, conflicting information, causing hallucinated diffs.

### Deep Evaluation: Shadow Mode Telemetry Leakage
- **Mechanism:** In shadow mode, the system runs a parallel evaluation without taking action.
- *Risk:* Facts generated during shadow mode evaluation (e.g., predicted actions) are injected into the kernel.
- *Failure Mode:* The `ActivationEngine` does not differentiate between shadow facts and real facts. If a shadow evaluation is interrupted, its predicted actions are scored highly in the real run, tricking the real LLM into executing hallucinated, untested actions.

### Conclusion on Remediation Strategy
The boundary between imperative Go state (`ActivationEngine.symbolGraph`) and declarative Mangle state (`core.Kernel.EDB`) is fundamentally leaky. We must enforce deep copies of fact arguments at the boundary, rather than passing raw pointers or slices, and introduce explicit arity/type checks in `buildSymbolGraphLocked` before map insertion.
