# Root-Cause Mandate: No Band-Aids Allowed

> **CRITICAL**: When stress testing reveals broken artifacts that codeNERD created (invalid Mangle rules, corrupted facts, malformed configs), the artifact is NOT the bug - it is a SYMPTOM of a deeper systemic failure. Deleting, commenting out, or patching the artifact is strictly forbidden.

## The Principle

When codeNERD creates invalid artifacts during stress testing:

| DO NOT (Band-Aid) | DO (Root-Cause Fix) |
|-------------------|---------------------|
| Comment out the broken learned rule | Investigate why autopoiesis generated an invalid rule |
| Delete the corrupted fact from the store | Find why the fact was written incorrectly |
| Manually fix the malformed config | Trace the config generation code path |
| Ignore and retry the test | Understand the failure mode completely |

## Investigation Protocol

When a stress test reveals codeNERD-created artifacts that break the system:

1. **Freeze the scene** - Do NOT modify or delete the broken artifact yet
2. **Identify the generation source** - Which subsystem created this artifact?
   - Learned rules → `internal/autopoiesis/` (Ouroboros, learning loops)
   - Facts → `internal/core/kernel.go` (fact assertion paths)
   - Tools → `internal/shards/tool_generator/` (Ouroboros tool gen)
   - Configs → Check all writers to `.nerd/` directory
3. **Trace the creation path** - How did invalid data get through?
   - Missing validation at creation time?
   - Race condition during concurrent writes?
   - Incomplete schema enforcement?
4. **Design the systemic fix** - Options include:
   - **Validation at creation** - Prevent invalid artifacts from being written
   - **Self-healing mechanism** - Detect and auto-repair/remove invalid artifacts
   - **Schema enforcement** - Tighten constraints so invalid states are impossible
5. **Implement and re-stress** - Fix the generation pipeline, then re-run the stress test

## Anti-Pattern Catalog

### Category A: Deletion & Suppression

| Anti-Pattern | Why It's Wrong | What To Do Instead |
|--------------|----------------|-------------------|
| Comment out broken code | Hides the symptom, generation bug remains | Find and fix the code that generated the broken code |
| Delete problematic artifact files | Next run will recreate them broken again | Fix the writer, not the written |
| Add `// nolint` or ignore directives | Silences linters that caught real bugs | Fix the underlying issue the linter detected |
| Remove failing test cases | Tests were right, implementation is wrong | Fix implementation to pass the test |
| Delete log lines that show errors | Blinds future debugging | Fix the error source |

### Category B: Defensive Wrapping

| Anti-Pattern | Why It's Wrong | What To Do Instead |
|--------------|----------------|-------------------|
| Add nil checks everywhere | Papers over nil propagation from source | Find where nil originates and prevent it |
| Wrap in `recover()` at top level | Panics indicate logic bugs, not runtime noise | Fix the panic source |
| Add `if err != nil { return nil, nil }` | Swallows errors, breaks callers | Propagate errors properly, fix root cause |
| Add retry loops around flaky operations | Retries hide race conditions and resource issues | Fix the flakiness at source |
| Increase timeouts to "fix" slowness | Slowness indicates performance bug | Profile and fix the performance issue |

### Category C: Limit Manipulation

| Anti-Pattern | Why It's Wrong | What To Do Instead |
|--------------|----------------|-------------------|
| Lower resource limits to prevent OOM | Hides memory leak or unbounded growth | Fix the leak or add proper bounds |
| Increase gas limit to fix derivation timeout | Infinite recursion or exponential rules exist | Fix the Mangle rules causing explosion |
| Reduce concurrency to hide race conditions | Race still exists, just triggers less often | Fix the race with proper synchronization |
| Increase queue sizes to prevent overflow | Producer outpacing consumer indicates design flaw | Add backpressure or fix throughput mismatch |
| Disable features that stress test broke | Feature is broken, not optional | Fix the feature |

### Category D: Special-Casing

| Anti-Pattern | Why It's Wrong | What To Do Instead |
|--------------|----------------|-------------------|
| Add `if specificValue { skip }` | Creates tech debt, doesn't fix root cause | Understand why that value is problematic, fix generally |
| Hardcode values that "work" | Config/dynamic loading is broken | Fix the loading mechanism |
| Add special error handlers for one case | Indicates misunderstood error contract | Fix error handling architecture |
| Create "safe" wrapper for one callsite | Other callsites still vulnerable | Fix the unsafe function itself |
| Add migration code for bad data | More bad data will be created | Fix the data creation, migrate once |

### Category E: Mangle-Specific Anti-Patterns

| Anti-Pattern | Why It's Wrong | What To Do Instead |
|--------------|----------------|-------------------|
| Add `Decl` for undeclared predicate in generated rule | Generation shouldn't create undeclared predicates | Add validation to rule generator |
| Manually edit `.mg` files | Will be overwritten or regenerated wrong | Fix the Go code that writes `.mg` files |
| Comment out stratification-violating rules | Rule generator has logic bug | Fix rule generation to produce valid strata |
| Reduce fact count to avoid gas limit | Unbounded fact growth is the bug | Add fact lifecycle management |
| Disable learned rules entirely | Learning system has validation gap | Add validation to learning pipeline |

### Category F: Go/Concurrency Anti-Patterns

| Anti-Pattern | Why It's Wrong | What To Do Instead |
|--------------|----------------|-------------------|
| Add `sync.Mutex` to hide race condition | Mutex doesn't fix race, may cause deadlock | Redesign data flow to eliminate race |
| Ignore context cancellation | Goroutines leak, resources held indefinitely | Propagate context, respect cancellation |
| Add `go func()` without lifecycle management | Orphan goroutines accumulate | Use WaitGroup or errgroup, track lifecycle |
| `defer recover()` in every function | Hides bugs, makes debugging impossible | Fix panic sources, recover only at boundaries |
| Use `time.Sleep` for synchronization | Race condition still exists | Use proper sync primitives (chan, WaitGroup) |

## Root-Cause Investigation Methodology

### Step 1: Classify the Failure Mode

| Failure Type | Key Question | Investigation Focus |
|--------------|--------------|---------------------|
| **Panic** | What nil/bounds/assert triggered? | Stack trace → function → how did bad state arrive? |
| **Deadlock** | What goroutines are blocked? | `pprof` → lock ordering → who holds what? |
| **Memory Leak** | What's growing unbounded? | Heap profile → retention path → who's holding refs? |
| **Data Corruption** | What invariant was violated? | Last valid state → first invalid → what changed it? |
| **Performance Degradation** | What's taking time? | CPU profile → hot paths → algorithmic complexity? |
| **Resource Exhaustion** | What's not being released? | Resource tracking → lifecycle → missing cleanup? |

### Step 2: Trace the Causal Chain

```text
SYMPTOM: Panic in kernel.Query()
    ↑
PROXIMATE CAUSE: Nil pointer in fact.Args[0]
    ↑
INTERMEDIATE: Fact created with empty args slice
    ↑
ROOT CAUSE: ToAtom() in learner.go doesn't validate args before creating fact
    ↑
SYSTEMIC FIX: Add validation in ToAtom(), add schema constraint for min args
```

**Always trace back to the EARLIEST point where the bug could have been prevented.**

### Step 3: Apply the Five Whys

Example from stress test failure:

1. **Why did the kernel panic?** → Nil pointer in Query result
2. **Why was the result nil?** → Predicate had no matching facts
3. **Why were there no matching facts?** → Facts used wrong predicate name
4. **Why was the predicate name wrong?** → Learner used string interpolation instead of atom
5. **Why did learner use string interpolation?** → No type enforcement in rule template

**ROOT CAUSE:** Rule template system allows strings where atoms are required.
**FIX:** Type-safe template API that only accepts atom types for predicate positions.

### Step 4: Verify Fix Completeness

Before closing the investigation, verify:

- [ ] **Root cause identified** - Not just proximate cause
- [ ] **Fix prevents recurrence** - Same bug cannot happen again
- [ ] **Similar vectors checked** - Other code paths with same pattern reviewed
- [ ] **Regression test added** - Stress test that would catch this
- [ ] **Self-healing considered** - Can system detect and recover if similar issue occurs?

## Extended Examples

### Example: Invalid Learned Rule

**Symptom:** Stress test causes panic due to undeclared predicate in `learned.mg`

**Wrong Response (FORBIDDEN):**
```bash
# Just commenting out the broken rule in learned.mg
```

**Correct Response:**
1. Identify source: Rule came from autopoiesis learning system
2. Trace path: `internal/autopoiesis/learner.go` → `LearnPattern()` → writes to `learned.mg`
3. Root cause: No validation that predicates in learned rules are declared in schema
4. Systemic fix options:
   - Add `ValidateRule()` before writing to `learned.mg`
   - Create predicate whitelist for learnable patterns
   - Add schema-check pass after rule generation
   - Implement self-healing: kernel detects undeclared predicates on load, quarantines invalid rules

### Example: Corrupted Scan Cache

**Symptom:** `scan.mg` contains duplicate facts causing stratification errors

**Root-Cause Response:**
1. **Classify:** Data corruption - duplicate facts violate uniqueness invariant
2. **Trace chain:**
   - Duplicates exist in scan.mg
   - Scanner appends without checking existing facts
   - Concurrent scans can race and double-write
   - Scanner lacks file locking
3. **Five whys:**
   - Why duplicates? → Append without dedup
   - Why no dedup? → Assumed single-writer
   - Why multiple writers? → Concurrent scan requests
   - Why concurrent? → No scan lock
   - Why no lock? → Scanner designed for single-threaded use, now called from parallel shards
4. **Systemic fix:**
   - Add file locking to scanner writes
   - Add dedup pass before write
   - Consider: fact-level idempotency (same fact asserted twice = no-op)
   - Self-healing: Kernel detects duplicates on load, dedupes automatically

## The Healing Hierarchy

When designing fixes, prefer higher levels of the hierarchy:

```text
Level 5: IMPOSSIBLE   - Invalid state cannot be represented (type system, schema)
Level 4: PREVENTED    - Invalid state rejected at creation (validation)
Level 3: DETECTED     - Invalid state caught at load/startup (self-check)
Level 2: RECOVERED    - Invalid state found at runtime, auto-healed (self-healing)
Level 1: LOGGED       - Invalid state found, reported for manual fix (alert)
Level 0: SILENT FAIL  - Invalid state causes undefined behavior (BUG)
```

**Always aim for the highest feasible level.** Level 5 (impossible) is best.

## Red Flags That Indicate Band-Aid Thinking

When reviewing fixes for stress test failures, reject any PR that:

1. **Modifies artifacts instead of generators** - Editing `.mg`, `.json`, or generated code files
2. **Adds defensive checks without tracing origin** - Nil checks, empty checks without finding source
3. **Increases limits/timeouts** - Without profiling and fixing root cause
4. **Adds special cases** - `if x == brokenValue { handleSpecially }`
5. **Disables or comments out code** - Instead of fixing it
6. **Uses words like "workaround", "temporary", "hack"** - These become permanent
7. **Doesn't include regression test** - Same bug will return
8. **Fixes consumer instead of producer** - Error handling in caller instead of fixing callee
9. **Adds logging without fixing** - "Let's log this and see" is not a fix
10. **Changes behavior only under stress conditions** - `if underLoad { differentPath }` hides the bug

## Subsystems That Create Artifacts

| Subsystem | Artifacts Created | Validation Point |
|-----------|-------------------|------------------|
| Autopoiesis Learner | `learned.mg` rules | `internal/autopoiesis/learner.go` |
| Ouroboros Tool Gen | `.nerd/tools/*.go` | `internal/shards/tool_generator/` |
| Memory Persistence | `.nerd/memory/*.db` | `internal/store/` |
| Config Writers | `.nerd/config.json` | Various |
| Scan Cache | `.nerd/mangle/scan.mg` | `internal/world/scanner.go` |
| Fact Recorder | `.nerd/mangle/*.mg` | `internal/core/fact_recorder.go` |

When stress testing reveals failures in any of these, the fix lives in the **creation code**, not in the **created artifact**.
