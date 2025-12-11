---
name: stress-tester
description: Live stress testing of codeNERD via CLI. Use when testing system stability, finding panics, edge cases, and failure modes across all 25+ subsystems. Includes comprehensive multi-minute workflows with conservative, aggressive, chaos, and hybrid severity levels.
---

# Stress Tester

## Overview

Live stress testing skill for codeNERD that systematically pushes all subsystems to their limits via CLI commands. Unlike unit tests, these are extensive end-to-end scenarios designed to find panics, race conditions, resource exhaustion, and edge cases across the entire system.

**When to use:**

- Pre-release stability verification
- After major architectural changes
- Debugging intermittent failures
- Validating resource limits
- Finding panic vectors

## Quick Start

### 1. Build codeNERD

```powershell
$env:CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers"; go build ./cmd/nerd
```

### 2. Clear Logs

```bash
rm .nerd/logs/*
```

### 3. Pick a Workflow

Start with a conservative test from any category:

| Quick Test | Category | Duration |
|------------|----------|----------|
| [queue-saturation.md](references/workflows/01-kernel-core/queue-saturation.md) | Kernel | 10-15 min |
| [intent-fuzzing.md](references/workflows/02-perception-articulation/intent-fuzzing.md) | Perception | 15-20 min |
| [shard-explosion.md](references/workflows/03-shards-campaigns/shard-explosion.md) | Shards | 15-25 min |

### 4. Analyze Results

```bash
python .claude/skills/stress-tester/scripts/analyze_stress_logs.py
```

### 5. Remember that if the system bugs out, an entire combined mangle schema which combines all .mg files will dump to C:\CodeProjects\codeNERD\debug_program_ERROR.mg... also, there are between 5000-10000 facts in the kernal, set long timeouts

## Workflow Catalog

### 01-kernel-core (4 workflows)

Tests the Mangle kernel, SpawnQueue, and core runtime.

| Workflow | What It Stresses | Duration |
|----------|------------------|----------|
| [queue-saturation.md](references/workflows/01-kernel-core/queue-saturation.md) | SpawnQueue backpressure with 100+ spawn requests | 10-25 min |
| [mangle-explosion.md](references/workflows/01-kernel-core/mangle-explosion.md) | Cyclic rules + large EDB causing derivation explosion | 15-30 min |
| [memory-pressure.md](references/workflows/01-kernel-core/memory-pressure.md) | Load 250k facts, trigger emergency compression | 20-40 min |
| [concurrent-derivations.md](references/workflows/01-kernel-core/concurrent-derivations.md) | 4 shards querying kernel simultaneously | 10-20 min |

### 02-perception-articulation (3 workflows)

Tests NL parsing, intent classification, and response formatting.

| Workflow | What It Stresses | Duration |
|----------|------------------|----------|
| [intent-fuzzing.md](references/workflows/02-perception-articulation/intent-fuzzing.md) | Malformed NL inputs, adversarial strings, edge case verbs | 15-25 min |
| [piggyback-corruption.md](references/workflows/02-perception-articulation/piggyback-corruption.md) | Truncated JSON, invalid ControlPackets | 10-20 min |
| [taxonomy-exhaustion.md](references/workflows/02-perception-articulation/taxonomy-exhaustion.md) | Every verb in corpus + unknown verbs | 15-25 min |

### 03-shards-campaigns (4 workflows)

Tests shard lifecycle, campaigns, and TDD loops.

| Workflow | What It Stresses | Duration |
|----------|------------------|----------|
| [campaign-marathon.md](references/workflows/03-shards-campaigns/campaign-marathon.md) | 50-phase campaign with 500 tasks | 45-90 min |
| [shard-explosion.md](references/workflows/03-shards-campaigns/shard-explosion.md) | Spawn all shard types rapidly | 15-25 min |
| [tdd-infinite-loop.md](references/workflows/03-shards-campaigns/tdd-infinite-loop.md) | Test that always fails, repair loop stress | 20-30 min |
| [reviewer-finding-explosion.md](references/workflows/03-shards-campaigns/reviewer-finding-explosion.md) | Large codebase with 1000+ issues | 20-30 min |

### 04-autopoiesis-ouroboros (3 workflows)

Tests self-modification, tool generation, and adversarial testing.

| Workflow | What It Stresses | Duration |
|----------|------------------|----------|
| [tool-generation-nesting.md](references/workflows/04-autopoiesis-ouroboros/tool-generation-nesting.md) | Tool that generates tool that generates tool | 20-35 min |
| [thunderdome-battle.md](references/workflows/04-autopoiesis-ouroboros/thunderdome-battle.md) | 100 attack vectors against generated tools | 25-40 min |
| [safety-checker-bypass.md](references/workflows/04-autopoiesis-ouroboros/safety-checker-bypass.md) | Forbidden imports, dangerous operations | 15-25 min |

### 05-world-context (3 workflows)

Tests filesystem scanning, context building, and impact analysis.

| Workflow | What It Stresses | Duration |
|----------|------------------|----------|
| [large-codebase-scan.md](references/workflows/05-world-context/large-codebase-scan.md) | 10k+ files, symlink loops, deep nesting | 25-40 min |
| [context-compression.md](references/workflows/05-world-context/context-compression.md) | 100+ turn conversation, emergency compression | 20-30 min |
| [holographic-impact.md](references/workflows/05-world-context/holographic-impact.md) | Impact analysis on massive change set | 20-30 min |

### 06-advanced-features (3 workflows)

Tests dream state, shadow mode, and browser automation.

| Workflow | What It Stresses | Duration |
|----------|------------------|----------|
| [dream-state-load.md](references/workflows/06-advanced-features/dream-state-load.md) | 4 consultants x 100 perspectives | 25-40 min |
| [shadow-mode-stress.md](references/workflows/06-advanced-features/shadow-mode-stress.md) | Complex action simulation with rollback | 15-25 min |
| [browser-automation.md](references/workflows/06-advanced-features/browser-automation.md) | 50 concurrent page fetches via rod | 25-40 min |

### 07-full-system-chaos (3 workflows)

Tests system-wide stability under extreme conditions.

| Workflow | What It Stresses | Duration |
|----------|------------------|----------|
| [everything-at-once.md](references/workflows/07-full-system-chaos/everything-at-once.md) | All subsystems stressed simultaneously | 60-120 min |
| [long-running-session.md](references/workflows/07-full-system-chaos/long-running-session.md) | 2+ hour session stability | 120+ min |
| [recovery-after-panic.md](references/workflows/07-full-system-chaos/recovery-after-panic.md) | Force panic, verify recovery | 20-30 min |

### 08-hybrid-integration (4 workflows)

Tests cross-subsystem integration under load.

| Workflow | What It Stresses | Duration |
|----------|------------------|----------|
| [perception-to-campaign.md](references/workflows/08-hybrid-integration/perception-to-campaign.md) | NL input through full campaign execution | 25-40 min |
| [research-to-coder-to-tester.md](references/workflows/08-hybrid-integration/research-to-coder-to-tester.md) | Full shard handoff pipeline | 30-45 min |
| [ouroboros-thunderdome-nemesis.md](references/workflows/08-hybrid-integration/ouroboros-thunderdome-nemesis.md) | Adversarial tool evolution loop | 35-50 min |
| [full-ooda-loop-stress.md](references/workflows/08-hybrid-integration/full-ooda-loop-stress.md) | Complete OODA cycle under pressure | 40-60 min |

## Severity Levels

Each workflow supports 4 severity levels:

| Level | Description | Use When |
|-------|-------------|----------|
| **Conservative** | Stay within configured limits, test edge cases | Regular CI/CD, smoke testing |
| **Aggressive** | Approach/exceed limits, stress resources | Pre-release validation |
| **Chaos** | Random inputs, race conditions, resource exhaustion | Finding unknown failure modes |
| **Hybrid** | Multiple subsystems stressed simultaneously | Integration validation |

## Log Analysis Integration

After any stress test, analyze logs using the integrated log-analyzer:

```bash
# Quick analysis
python .claude/skills/stress-tester/scripts/analyze_stress_logs.py

# Verbose with custom output
python .claude/skills/stress-tester/scripts/analyze_stress_logs.py -v -o report.md

# Manual query with logquery
cd .claude/skills/log-analyzer/scripts
python parse_log.py .nerd/logs/* --no-schema | grep "^log_entry" > /tmp/stress.mg
cd logquery
./logquery.exe /tmp/stress.mg --builtin errors
./logquery.exe /tmp/stress.mg --builtin kernel-errors
```

### Custom Stress Queries

The skill includes [stress_queries.mg](assets/stress_queries.mg) with predicates for:

- `panic_detected/3` - Panic events with stack traces
- `nil_pointer_error/3` - Nil pointer dereferences
- `oom_event/3` - Out of memory events
- `timeout_event/3` - Operation timeouts
- `queue_full/3` - Queue saturation events
- `gas_limit_hit/3` - Mangle gas limit exceeded
- `critical_issue/3` - Any critical failure

## Test Fixtures

### Mangle Stress Files

- [cyclic_rules.mg](assets/cyclic_rules.mg) - Rules causing derivation explosion
- [stress_queries.mg](assets/stress_queries.mg) - Log analysis queries

### Input Generators

- [generate_large_project.py](scripts/fixtures/generate_large_project.py) - Creates synthetic Go projects
- [malformed_inputs.py](scripts/fixtures/malformed_inputs.py) - Generates fuzzing payloads

### Malformed Data

- [malformed_piggyback.json](assets/malformed_piggyback.json) - Invalid JSON for articulation testing

## Reference Documentation

- [subsystem-stress-points.md](references/subsystem-stress-points.md) - All 25+ subsystems with failure modes
- [panic-catalog.md](references/panic-catalog.md) - Known panic vectors with triggers
- [resource-limits.md](references/resource-limits.md) - Config limits and safe/dangerous values

## Success Criteria

Every stress test should verify:

- [ ] No panics in logs (`grep -i "panic" .nerd/logs/*.log`)
- [ ] Memory stayed within limits
- [ ] All commands completed
- [ ] No orphaned goroutines
- [ ] Data integrity maintained
- [ ] Recovery after any failures

## Common Failure Patterns

| Pattern | Symptom | Check |
|---------|---------|-------|
| Queue saturation | "queue full" errors | `spawn_queue_depth` predicate |
| Gas exhaustion | "gas limit" errors | `gas_limit_hit` query |
| Memory pressure | OOM or slowdown | `memory_usage` predicate |
| Derivation explosion | Long delays, high CPU | `derived_fact_count` query |
| Panic | Process crash | Log files for stack trace |

## CRITICAL: Artifacts Are Symptoms, Not Causes

> **NO BAND-AIDS ALLOWED.** When stress testing reveals broken artifacts that codeNERD created (invalid Mangle rules, corrupted facts, malformed configs), the artifact is NOT the bug - it is a SYMPTOM of a deeper systemic failure. Deleting, commenting out, or patching the artifact is strictly forbidden.

### The Root-Cause Mandate

When codeNERD creates invalid artifacts during stress testing:

| DO NOT (Band-Aid) | DO (Root-Cause Fix) |
|-------------------|---------------------|
| Comment out the broken learned rule | Investigate why autopoiesis generated an invalid rule |
| Delete the corrupted fact from the store | Find why the fact was written incorrectly |
| Manually fix the malformed config | Trace the config generation code path |
| Ignore and retry the test | Understand the failure mode completely |

### Investigation Protocol

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

### Example: Invalid Learned Rule

**Symptom:** Stress test causes panic due to undeclared predicate in `learned.mg`

**Wrong Response:**

```bash
# FORBIDDEN - This is a band-aid
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

### Self-Healing Checklist

For any artifact-creation subsystem, verify these safeguards exist:

- [ ] **Creation-time validation** - Invalid data rejected before write
- [ ] **Load-time validation** - Invalid data detected on system startup
- [ ] **Runtime detection** - Invalid data caught during execution
- [ ] **Quarantine mechanism** - Invalid data isolated, not deleted (for debugging)
- [ ] **Audit trail** - Log showing what created the artifact and when

### Subsystems That Create Artifacts

| Subsystem | Artifacts Created | Validation Point |
|-----------|-------------------|------------------|
| Autopoiesis Learner | `learned.mg` rules | `internal/autopoiesis/learner.go` |
| Ouroboros Tool Gen | `.nerd/tools/*.go` | `internal/shards/tool_generator/` |
| Memory Persistence | `.nerd/memory/*.db` | `internal/store/` |
| Config Writers | `.nerd/config.json` | Various |
| Scan Cache | `.nerd/mangle/scan.mg` | `internal/world/scanner.go` |
| Fact Recorder | `.nerd/mangle/*.mg` | `internal/core/fact_recorder.go` |

When stress testing reveals failures in any of these, the fix lives in the **creation code**, not in the **created artifact**.

### Comprehensive Anti-Pattern Catalog

These are the band-aid fixes that AI agents commonly attempt. **ALL ARE FORBIDDEN.**

#### Category A: Deletion & Suppression

| Anti-Pattern | Why It's Wrong | What To Do Instead |
|--------------|----------------|-------------------|
| Comment out broken code | Hides the symptom, generation bug remains | Find and fix the code that generated the broken code |
| Delete problematic artifact files | Next run will recreate them broken again | Fix the writer, not the written |
| Add `// nolint` or ignore directives | Silences linters that caught real bugs | Fix the underlying issue the linter detected |
| Remove failing test cases | Tests were right, implementation is wrong | Fix implementation to pass the test |
| Delete log lines that show errors | Blinds future debugging | Fix the error source |

#### Category B: Defensive Wrapping

| Anti-Pattern | Why It's Wrong | What To Do Instead |
|--------------|----------------|-------------------|
| Add nil checks everywhere | Papers over nil propagation from source | Find where nil originates and prevent it |
| Wrap in `recover()` at top level | Panics indicate logic bugs, not runtime noise | Fix the panic source |
| Add `if err != nil { return nil, nil }` | Swallows errors, breaks callers | Propagate errors properly, fix root cause |
| Add retry loops around flaky operations | Retries hide race conditions and resource issues | Fix the flakiness at source |
| Increase timeouts to "fix" slowness | Slowness indicates performance bug | Profile and fix the performance issue |

#### Category C: Limit Manipulation

| Anti-Pattern | Why It's Wrong | What To Do Instead |
|--------------|----------------|-------------------|
| Lower resource limits to prevent OOM | Hides memory leak or unbounded growth | Fix the leak or add proper bounds |
| Increase gas limit to fix derivation timeout | Infinite recursion or exponential rules exist | Fix the Mangle rules causing explosion |
| Reduce concurrency to hide race conditions | Race still exists, just triggers less often | Fix the race with proper synchronization |
| Increase queue sizes to prevent overflow | Producer outpacing consumer indicates design flaw | Add backpressure or fix throughput mismatch |
| Disable features that stress test broke | Feature is broken, not optional | Fix the feature |

#### Category D: Special-Casing

| Anti-Pattern | Why It's Wrong | What To Do Instead |
|--------------|----------------|-------------------|
| Add `if specificValue { skip }` | Creates tech debt, doesn't fix root cause | Understand why that value is problematic, fix generally |
| Hardcode values that "work" | Config/dynamic loading is broken | Fix the loading mechanism |
| Add special error handlers for one case | Indicates misunderstood error contract | Fix error handling architecture |
| Create "safe" wrapper for one callsite | Other callsites still vulnerable | Fix the unsafe function itself |
| Add migration code for bad data | More bad data will be created | Fix the data creation, migrate once |

#### Category E: Mangle-Specific Anti-Patterns

| Anti-Pattern | Why It's Wrong | What To Do Instead |
|--------------|----------------|-------------------|
| Add `Decl` for undeclared predicate in generated rule | Generation shouldn't create undeclared predicates | Add validation to rule generator |
| Manually edit `.mg` files | Will be overwritten or regenerated wrong | Fix the Go code that writes `.mg` files |
| Comment out stratification-violating rules | Rule generator has logic bug | Fix rule generation to produce valid strata |
| Reduce fact count to avoid gas limit | Unbounded fact growth is the bug | Add fact lifecycle management |
| Disable learned rules entirely | Learning system has validation gap | Add validation to learning pipeline |

#### Category F: Go/Concurrency Anti-Patterns

| Anti-Pattern | Why It's Wrong | What To Do Instead |
|--------------|----------------|-------------------|
| Add `sync.Mutex` to hide race condition | Mutex doesn't fix race, may cause deadlock | Redesign data flow to eliminate race |
| Ignore context cancellation | Goroutines leak, resources held indefinitely | Propagate context, respect cancellation |
| Add `go func()` without lifecycle management | Orphan goroutines accumulate | Use WaitGroup or errgroup, track lifecycle |
| `defer recover()` in every function | Hides bugs, makes debugging impossible | Fix panic sources, recover only at boundaries |
| Use `time.Sleep` for synchronization | Race condition still exists | Use proper sync primitives (chan, WaitGroup) |

### Root-Cause Investigation Methodology

When stress testing reveals a failure, apply these diagnostic patterns IN ORDER:

#### Step 1: Classify the Failure Mode

| Failure Type | Key Question | Investigation Focus |
|--------------|--------------|---------------------|
| **Panic** | What nil/bounds/assert triggered? | Stack trace → function → how did bad state arrive? |
| **Deadlock** | What goroutines are blocked? | `pprof` → lock ordering → who holds what? |
| **Memory Leak** | What's growing unbounded? | Heap profile → retention path → who's holding refs? |
| **Data Corruption** | What invariant was violated? | Last valid state → first invalid → what changed it? |
| **Performance Degradation** | What's taking time? | CPU profile → hot paths → algorithmic complexity? |
| **Resource Exhaustion** | What's not being released? | Resource tracking → lifecycle → missing cleanup? |

#### Step 2: Trace the Causal Chain

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

#### Step 3: Apply the Five Whys

Example from stress test failure:

1. **Why did the kernel panic?** → Nil pointer in Query result
2. **Why was the result nil?** → Predicate had no matching facts
3. **Why were there no matching facts?** → Facts used wrong predicate name
4. **Why was the predicate name wrong?** → Learner used string interpolation instead of atom
5. **Why did learner use string interpolation?** → No type enforcement in rule template

**ROOT CAUSE:** Rule template system allows strings where atoms are required.
**FIX:** Type-safe template API that only accepts atom types for predicate positions.

#### Step 4: Verify Fix Completeness

Before closing the investigation, verify:

- [ ] **Root cause identified** - Not just proximate cause
- [ ] **Fix prevents recurrence** - Same bug cannot happen again
- [ ] **Similar vectors checked** - Other code paths with same pattern reviewed
- [ ] **Regression test added** - Stress test that would catch this
- [ ] **Self-healing considered** - Can system detect and recover if similar issue occurs?

### Extended Examples

#### Example 2: Corrupted Scan Cache

**Symptom:** `scan.mg` contains duplicate facts causing stratification errors

**Band-Aid Response (FORBIDDEN):**

```bash
# Just delete and rescan
rm .nerd/mangle/scan.mg
./nerd.exe scan
```

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

#### Example 3: Tool Generation Infinite Loop

**Symptom:** Ouroboros generates tool that immediately triggers Ouroboros again

**Band-Aid Response (FORBIDDEN):**

```go
// Just add a recursion limit
if depth > 5 {
    return errors.New("too deep")
}
```

**Root-Cause Response:**

1. **Classify:** Logic bug - recursion termination condition missing
2. **Trace chain:**
   - Tool generates → needs capability → triggers Ouroboros → generates tool → ...
   - Capability check doesn't recognize newly-generated tool
   - Tool registry not updated synchronously
   - Ouroboros doesn't check "am I already generating this?"
3. **Five whys:**
   - Why infinite? → No cycle detection
   - Why no cycle? → Assumed tools wouldn't trigger generation
   - Why do they? → Capability gap in generated tool
   - Why gap? → Tool template incomplete
   - Why incomplete? → Template validation doesn't verify capability coverage
4. **Systemic fix:**
   - Add generation cycle detection (track in-flight tool names)
   - Validate generated tool has all required capabilities
   - Synchronously update registry before returning
   - Add generation provenance tracking (tool X generated by Ouroboros for capability Y)

#### Example 4: Memory Pressure from Fact Accumulation

**Symptom:** RAM usage grows unbounded during long session, eventually OOM

**Band-Aid Response (FORBIDDEN):**

```go
// Just clear facts periodically
if len(facts) > 100000 {
    facts = facts[:50000]  // Keep recent half
}
```

**Root-Cause Response:**

1. **Classify:** Resource leak - facts created but never retired
2. **Trace chain:**
   - Every query adds derived facts
   - Derived facts persist in working memory
   - No fact retirement policy
   - Session grows monotonically
3. **Five whys:**
   - Why OOM? → Unbounded fact growth
   - Why unbounded? → No retirement policy
   - Why no retirement? → Originally designed for short sessions
   - Why long sessions now? → Architecture evolved, memory model didn't
   - Why wasn't it updated? → No memory pressure tests until now
4. **Systemic fix:**
   - Implement fact lifetime tiers (session, turn, derived)
   - Add LRU eviction for derived facts
   - Compress old facts to cold storage
   - Add memory pressure monitoring with emergency compression
   - Self-healing: Automatic tier demotion under pressure

### Red Flags That Indicate Band-Aid Thinking

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

### The Healing Hierarchy

When designing fixes, prefer higher levels of the hierarchy:

```text
Level 5: IMPOSSIBLE   - Invalid state cannot be represented (type system, schema)
Level 4: PREVENTED    - Invalid state rejected at creation (validation)
Level 3: DETECTED     - Invalid state caught at load/startup (self-check)
Level 2: RECOVERED    - Invalid state found at runtime, auto-healed (self-healing)
Level 1: LOGGED       - Invalid state found, reported for manual fix (alert)
Level 0: SILENT FAIL  - Invalid state causes undefined behavior (BUG)
```

**Always aim for the highest feasible level.** Level 5 (impossible) is best - if Mangle's type system prevents a predicate from having wrong arity, that bug class is eliminated forever.

### Post-Fix Verification

After implementing a root-cause fix:

1. **Re-run the original stress test** - Must pass now
2. **Run at higher severity** - If you ran conservative, run aggressive
3. **Run related workflows** - If you fixed kernel, run all kernel tests
4. **Check for regressions** - Run full test suite
5. **Document the fix** - Add to [panic-catalog.md](references/panic-catalog.md) if novel failure mode
