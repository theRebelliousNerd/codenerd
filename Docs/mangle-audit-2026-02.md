# Mangle Integration Audit & Enhancement Report

**Date:** February 17, 2026
**Scope:** `internal/mangle/`, `internal/core/`, `internal/types/`, `internal/perception/`, `internal/prompt/`
**Mangle Version Pinned:** `v0.4.0` (go.mod line 14)
**Upstream HEAD:** `29970168` (Feb 11, 2026) -- significantly ahead of v0.4.0

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Upstream Mangle: What We're Missing](#2-upstream-mangle-what-were-missing)
3. [Critical: Data Fidelity Issues](#3-critical-data-fidelity-issues)
4. [Critical: Schema & Type Safety](#4-critical-schema--type-safety)
5. [Critical: Prompt Atom Accuracy](#5-critical-prompt-atom-accuracy)
6. [High: Error Swallowing on Kernel Mutations](#6-high-error-swallowing-on-kernel-mutations)
7. [High: Stale Fact Accumulation](#7-high-stale-fact-accumulation)
8. [High: Perception Transducer Accuracy](#8-high-perception-transducer-accuracy)
9. [High: JIT Compilation Gaps](#9-high-jit-compilation-gaps)
10. [High: Virtual Predicate Architecture](#10-high-virtual-predicate-architecture)
11. [Medium: Code Quality Issues](#11-medium-code-quality-issues)
12. [Medium: Concurrency & Safety](#12-medium-concurrency--safety)
13. [Low: Housekeeping](#13-low-housekeeping)
14. [Upgrade Path & Prioritized Action Plan](#14-upgrade-path--prioritized-action-plan)

---

## 1. Executive Summary

This audit identifies **5 critical**, **14 high**, **10 medium**, and **6 low** priority issues across the Mangle integration layer. The most impactful cluster is the **float/fact fidelity + schema drift + prompt atom inaccuracy** combination: Go values are lossy-converted to Mangle facts, the schemas don't enforce types, and the prompt atoms teach the LLM wrong Mangle syntax. This means the logic kernel -- codeNERD's "executive brain" -- operates on imprecise data with unvalidated schemas, while the LLM receives contradictory instructions about how to interact with it.

### Impact on Agent Effectiveness

| Issue Cluster | Agent Impact |
|---------------|-------------|
| Float precision loss | Confidence scores, test coverage, code churn metrics silently corrupted. Policy rules comparing thresholds produce wrong decisions. |
| `fmt.Sprintf("%v")` fact extraction | Atom/string/number type information destroyed on round-trip. Equality comparisons between facts fail unpredictably. |
| Schema type annotations missing | No compile-time enforcement. Wrong-typed facts silently accepted. Mangle cannot catch bugs in rules. |
| Prompt atom wrong syntax | LLM generates invalid Mangle code. Feedback loops waste tokens on syntax errors that are our fault, not the LLM's. |
| Error swallowing on kernel mutations | Kernel operates on stale/incomplete state. `next_action` derivation based on wrong facts. Agent takes wrong actions. |
| Stale fact accumulation | Kernel fact store grows unboundedly. Old `test_state`, `heartbeat`, `diagnostic` facts cause phantom rule firings. |
| Perception `/explain` fallback | Unrecognized verbs silently become "explain". Agent explains code instead of performing requested action. |

---

## 2. Upstream Mangle: What We're Missing

The upstream `google/mangle` repository has landed major features since v0.4.0 that are directly relevant to codeNERD:

### 2.1 Time & Duration Built-in Types (Feb 2, 2026)
Commit `dd902ac0` -- +1,763 lines across 12 files.

Native `TimeType` and `DurationType` constants (int64 nanoseconds since Unix epoch). 30+ new built-in functions:
- Time: `fn:time:now`, `fn:time:add`, `fn:time:sub`, `fn:time:format`, `fn:time:parse_rfc3339`, `fn:time:year/month/day/hour/minute/second`, `fn:time:trunc`, `fn:time:from_unix_nanos`, `fn:time:to_unix_nanos`
- Duration: `fn:duration:from_hours/minutes/seconds/nanos`, `fn:duration:add`, `fn:duration:mult`
- Comparisons: `:time:lt/le/gt/ge`, `:duration:lt/le/gt/ge`

**codeNERD impact:** We currently store timestamps as `int64` numbers. With native time types, session timing, campaign phase tracking, heartbeat monitoring, and TDD cycle timing could all use proper temporal semantics with built-in comparison predicates.

### 2.2 Full DatalogMTL Temporal Reasoning (Feb 9, 2026)
Commit `77dd1714` -- +10,940 lines across 50 files.

Facts can have validity intervals via `@[start, end]` syntax:
```mangle
team_member(/alice, /engineering)@[2020-01-01, 2023-06-15].
active(/alice)@[2024-01-01, now].
```

Temporal operators for past/future reasoning:
- `<-[0d, 30d]` -- "was true at some point in the past 30 days"
- `[-[0d, 30d]` -- "was continuously true for last 30 days"
- `<+[0d, 7d]` -- "will be true at some point in next 7 days"

Allen's Interval Relations (`:interval:before`, `:interval:overlaps`, `:interval:during`, etc.), interval coalescing, `TemporalStore` with configurable limits, `CheckTemporalRecursion` static analysis.

**codeNERD impact:** This is transformative for:
- **Shard lifecycle tracking:** `shard_active(/coder_1)@[spawn_time, completion_time].` instead of assert/retract pairs that can leak
- **Session context windows:** `context_relevant(/file, /symbol)@[T, now].` with automatic expiry
- **Campaign phase management:** Phase validity intervals with overlap detection
- **Heartbeat monitoring:** `system_heartbeat(/shard_id)@[now].` with `<-[30s]` lookback queries instead of accumulating facts

### 2.3 External Predicates API with Filter Pushdown (Sep-Oct 2025)
Commits `ea0330ca`, `3596fd99` -- proper `func(query engine.Query, cb func(engine.Fact)) error` signature with arbitrary filter pushdown.

**codeNERD impact:** Our `virtualFactStore` wrapper manually intercepts `GetFacts()` calls and does its own matching with `factstore.Matches()`. The native API would let the Mangle engine push bound arguments directly to handlers, reducing result sets before they enter the evaluation loop.

### 2.4 Rust Architecture with WASM + Pluggable Storage (Jan 14, 2026)
Commit `5ae68167` -- +10,162 lines. Server Mode (WASM via wasmtime) and Edge Mode (pure Rust interpreter) with pluggable `Store`/`Host` traits.

**codeNERD impact:** Relevant for NERDide (browser-based IDE). Mangle logic could run client-side via WASM.

### 2.5 Deprecated API: `EvalProgram`
The old `EvalProgram` function is deprecated in favor of `EvalStratifiedProgramWithStats`. Our codebase exclusively uses `EvalProgramWithStats` (4 call sites) -- **this is correct and up to date**.

### 2.6 Other v0.4.0 Features We Should Leverage
- **Aggregation with non-reducer functions**: Normal functions can now be called in aggregation pipelines
- **Map reducer**: `fn:collect_map` for building maps in aggregation
- **Duplicate declaration checking**: Static analysis catches redeclared predicates

---

## 3. Critical: Data Fidelity Issues

### 3.1 Float-to-Integer Coercion with Ambiguous Scaling

The `0.0-1.0 → 0-100` scaling pattern is duplicated in **three files** and has a silent correctness bug.

**Locations:**
- `internal/types/types.go:141-158`
- `internal/mangle/engine.go:644-658`
- `internal/core/kernel_facts.go:186-191`

```go
case float64:
    if v >= 0.0 && v <= 1.0 {
        terms = append(terms, ast.Number(int64(v*100)))  // 0.85 → 85
    } else {
        terms = append(terms, ast.Number(int64(v)))       // 42.7 → 42 (TRUNCATION)
    }
```

**Problems:**
- **Ambiguity at boundary:** `1.0` becomes `100`, but `1.01` becomes `1`. A 99x cliff.
- **Precision loss:** `3.14` → `3`, `99.5` → `99`. Any Mangle rule comparing these produces wrong results.
- **Semantic ambiguity:** `1.0` could be a confidence score (should scale to 100) or a literal number (should stay 1). No way to distinguish.
- **Duplication:** Three separate implementations that must stay in sync.

**Recommendation:** Mangle v0.4.0 supports `ast.Float64()` natively. Add a `float64` case that preserves the value using `ast.Float64(v)`. If integer comparison is needed in specific rules, make the scaling explicit in the Mangle rules, not silently in Go.

### 3.2 Pervasive `fmt.Sprintf("%v")` Lossy Fact Extraction

There are **100+ instances** of `fmt.Sprintf("%v", fact.Args[N])` used to extract values from Mangle facts. This pattern destroys type information.

**Key locations:**
- `internal/session/executor.go:582,583,851,857,863`
- `internal/core/virtual_store.go:474,857,1050,1055,1064`
- `internal/campaign/decomposer.go:1032,1098,1145,1758-1760`
- `internal/core/trace.go:120-147`

**Problem A:** When `fact.Args[0]` is a Mangle atom `/readFile`, `fmt.Sprintf("%v")` produces the string `/readFile` -- but downstream code then does string comparisons that may or may not expect the leading `/`.

**Problem B:** In `trace.go:123`, fact args are *compared* using `%v` formatting:
```go
if fmt.Sprintf("%v", d.Args[0]) == fmt.Sprintf("%v", fact.Args[0]) {
```
This only works if both args produce identical string representations -- fragile when comparing atoms vs strings vs numbers.

**Problem C:** Default fallback in `types.go:166`:
```go
default:
    terms = append(terms, ast.String(fmt.Sprintf("%v", v)))
```
Any Go value that doesn't match the handled types (structs, maps, slices) gets serialized as `ast.String("{map[key:value]}")` -- a string that can never be matched or queried correctly.

**Recommendation:** Create a centralized `func ExtractString(arg ast.BaseTerm) string` and `func ExtractInt64(arg ast.BaseTerm) (int64, bool)` utility that properly handles `ast.Constant` types by checking `c.Type` and using the appropriate accessor (`c.Symbol`, `c.NumberValue()`, `c.Float64Value()`, `c.TimeValue()`, etc.).

### 3.3 Missing Float Handling in Graph Virtual Store

**File:** `internal/core/virtual_store_graph.go:73-94`

```go
func goToMangleTerm(val interface{}) (ast.BaseTerm, error) {
    switch v := val.(type) {
    case int:
        return ast.Number(int64(v)), nil
    // ... no float64 case at all!
    default:
        return ast.String(fmt.Sprintf("%v", v)), nil  // float64 becomes string "3.14"
    }
}
```

Float values from the knowledge graph fall through to `fmt.Sprintf` and become Mangle strings. Any rule using numeric comparisons on graph data will silently fail.

---

## 4. Critical: Schema & Type Safety

### 4.1 Schema Declarations Lack Type Annotations

**Files:** `internal/core/defaults/schemas_intent.mg`, `schemas_state.mg`, and all other `.mg` schema files.

```mangle
# Current (untyped):
Decl user_intent(ID, Category, Verb, Target, Constraint).
Decl state(StepID, Stability, Loc).

# Correct (typed):
Decl user_intent(ID.Type<n>, Category.Type<n>, Verb.Type<n>, Target.Type<string>, Constraint.Type<string>).
```

All 87+ `Decl` statements use untyped arguments. This means:
- The Mangle engine performs no type validation on facts
- Wrong-typed facts are silently accepted (string where atom expected)
- The powerful type inference system in `analysis.AnalyzeOneUnit` cannot catch type errors in rules
- This directly contradicts the AGENTS.md documentation which extensively describes `.Type<>` syntax

### 4.2 Schema Validator Discards Analysis Results

**File:** `internal/mangle/schema_validator.go:327-328`

```go
programInfo, _ := analysis.AnalyzeOneUnit(parsed, decls)
_ = programInfo  // Analyzed ProgramInfo is never used
```

The schema validator parses and analyzes rules but then throws away the `ProgramInfo`. It catches parse errors but misses analysis-level issues like type mismatches, stratification failures, and arity violations.

### 4.3 `factToAtomLocked` Missing New Type Bounds

**File:** `internal/mangle/engine.go:566-575`

The type-bound switch in `factToAtomLocked` only handles `/name`, `/string`, `/number`, `/bytes`. It does not handle:
- `/time` (`ast.TimeType`) -- added upstream Feb 2, 2026
- `/duration` (`ast.DurationType`) -- added upstream Feb 2, 2026
- `/float` (`ast.Float64Type`) -- already available in v0.4.0

---

## 5. Critical: Prompt Atom Accuracy

These issues directly cause the LLM to generate invalid Mangle code, wasting feedback loop tokens and producing incorrect logic.

### 5.1 Wrong Mangle `Decl` Syntax in Prompt Atoms

**File:** `internal/prompt/atoms/language/mangle.yaml:193-199`

```yaml
Decl user_intent(category: /category, verb: /verb,
                 target: String, constraint: String, mode: /mode).
Decl file_modified(path: String, lines: Number).
Decl hypothesis(type: /hypothesis_type, file: String,
                line: Number, confidence: Number).
```

This uses **Souffleundefined-style** declaration syntax (`name: type`) instead of Mangle's `.Type<>` syntax. It also adds a `mode` argument to `user_intent` that doesn't exist in the actual schema. The LLM will copy this invalid syntax, producing Mangle code that fails to parse.

**Correct syntax:**
```mangle
Decl user_intent(ID.Type<n>, Category.Type<n>, Verb.Type<n>, Target.Type<string>, Constraint.Type<string>).
```

### 5.2 Conflicting Predicate Arities Across Atoms

Multiple prompt atoms define the same predicates with different arities:

| Predicate | Atom File | Arity | Args |
|-----------|-----------|-------|------|
| `user_intent` | `language/mangle.yaml:161` | 5 | `Category, Verb, Target, Constraint, Mode` |
| `user_intent` | Schema `schemas_intent.mg:17` | 5 | `ID, Category, Verb, Target, Constraint` |
| `next_action` | `system/executive.yaml:39` | 4 | `ShardID, ActionVerb, Target, Args` |
| `next_action` | `language/mangle.yaml:164` | 1 | `Action` |
| `permitted` | Identity atom | 1 | `Action` |
| `permitted` | `identity/legislator.yaml:79` | 3 | `ActionType, Target, Payload` |
| `shard_executed` | `mangle/patterns/existence.yaml:519` | 3 | `ShardId, TaskId, Status` |
| `shard_executed` | `AGENTS.md` documentation | 4 | (documented as `/4`) |

When the JIT compiler selects different combinations of these atoms, the LLM receives contradictory instructions about predicate signatures. It may generate rules with the wrong number of arguments, causing silent failures (facts asserted but never matching any rule).

### 5.3 Perception Atom Verb Categories Don't Match Code

**File:** `internal/prompt/atoms/system/perception.yaml:38-40`

```yaml
- Query: explain, describe, search, find, show, list, analyze
- Mutation: fix, refactor, create, delete, implement, add, modify
- Instruction: run, test, build, deploy, review, debug
```

But `understanding_adapter.go` maps:
- `review` → category `/query` (not "Instruction" as atom says)
- `verify` → verb `/test` but category `/mutation` (testing is not a mutation)
- `chat` → verb `/greet` (not in any category in the atom)

The LLM's understanding of intent classification differs from the code's actual mapping.

---

## 6. High: Error Swallowing on Kernel Mutations

28 instances of `_ = err` on kernel operations found in `internal/core/`. These are not benign -- if kernel fact assertion or retraction fails, downstream policy evaluation operates on incomplete or stale state.

### 6.1 Session Context Hydration

**File:** `internal/core/virtual_store_predicates.go:554-582`

```go
_ = kernel.Retract("session_turn")
_ = kernel.Retract("similar_content")
_ = kernel.Retract("reasoning_trace")
// ...
_ = kernel.LoadFacts(turns)
_ = kernel.LoadFacts(matches)
_ = kernel.LoadFacts(traces)
```

If `LoadFacts` fails silently, the kernel operates with stale or missing session context. The agent makes decisions based on incomplete information -- e.g., not seeing the most recent turn, missing relevant code snippets, or lacking reasoning traces.

### 6.2 TDD State Transitions

**File:** `internal/core/tdd_loop.go:219-226`

```go
_ = t.kernel.Assert(Fact{Predicate: "test_state", Args: []interface{}{"/red"}})
_ = t.kernel.Assert(Fact{Predicate: "retry_count", Args: []interface{}{int64(3)}})
```

If state assertion fails, the TDD loop's Mangle-derived `next_action` operates on stale `test_state` facts. The agent may try to "fix" code that has already passed tests, or skip refactoring because it still thinks tests are failing.

### 6.3 Shard Manager Tool Routing

**File:** `internal/core/shard_manager_tools.go:148-172`

```go
_ = sm.kernel.Retract("current_shard_type")
_ = sm.kernel.Retract("current_intent")
_ = sm.kernel.Retract("current_time")
// ... followed by 4 more _ = sm.kernel.Assert(...)
```

### 6.4 System Heartbeats

**File:** `internal/core/system_shard.go:421` (via agents.go)

```go
_ = s.kernel.Assert(types.Fact{
    Predicate: "system_heartbeat",
    Args:      []interface{}{s.id, tick.Unix()},
})
```

If heartbeat assertion fails repeatedly, the kernel never knows system shards are alive. Policy rules depending on heartbeats derive incorrect conclusions.

**Recommendation:** At minimum, log errors at Warning level. For critical path mutations (session context, TDD state, tool routing), return the error to the caller and let them decide whether to proceed with degraded state or fail fast.

---

## 7. High: Stale Fact Accumulation

### 7.1 Only 2 Predicates Have Time-Based Pruning

**File:** `internal/core/virtual_store.go:1421-1423`

```go
prune("execution_result", 4, now.Add(-15*time.Minute).Unix())
prune("shard_context_refreshed", 2, now.Add(-60*time.Minute).Unix())
```

All other predicates that accumulate per-turn or per-action facts are **never pruned**:

| Predicate | Assertion Rate | Retraction | Problem |
|-----------|---------------|------------|---------|
| `system_heartbeat` | Every 10s per system shard | Never | ~360 facts/hour/shard |
| `test_state` | Every TDD state change | Never | Old states coexist with current |
| `retry_count` | Every retry | Never | Old counts accumulate |
| `diagnostic` | From build output parsing | Never | Old diagnostics persist after fixes |
| `active_shard` | On shard spawn | On completion only | Leaked if shard panics |

### 7.2 TDD State Not Retracted Before Assert

**File:** `internal/core/tdd_loop.go:219-226`

Each TDD state transition asserts a new `test_state` fact but **never retracts the old one**. After cycling Red → Green → Refactor, the kernel contains `test_state(/red)`, `test_state(/green)`, `test_state(/refactor)` simultaneously. Any policy rule like:
```mangle
next_action(/fix_code) :- test_state(/red), ...
```
Would fire even when the current state is Green.

### 7.3 Session Turn Retraction is Partial and Error-Swallowed

**File:** `internal/core/virtual_store_predicates.go:554-556`

Retracts 3 predicates per turn with swallowed errors. If `Retract` fails (concurrent modification), old facts persist alongside new ones, causing duplicate or conflicting session data.

### 7.4 JIT Context Facts Never Retracted

**File:** `internal/prompt/compiler.go:465-474`

Context facts are asserted to the kernel for Mangle-based atom selection, but there is **no corresponding retraction** after compilation. Facts from previous compilations accumulate, potentially causing stale context to influence future atom selection.

**Recommendation:** Implement a fact lifecycle system:
1. **Temporal facts** (when upstream is adopted): Use `@[now, now+TTL]` intervals with automatic expiry
2. **Until then:** Tag facts with a generation counter. Before evaluation, retract all facts from previous generation. This is the "retract-before-assert" pattern.
3. **For heartbeats:** Retract old heartbeat before asserting new one, or use a ring buffer approach (keep last N).

---

## 8. High: Perception Transducer Accuracy

### 8.1 Default Fallback to `/explain` Silently Swallows Unrecognized Intents

**File:** `internal/perception/understanding_adapter.go:259-261`

```go
default:
    return "/explain" // Safe fallback
```

Any verb not in the hardcoded `case` list silently maps to `/explain`. Common verbs that would be misclassified: "migrate", "optimize", "document", "benchmark", "profile", "audit", "scaffold", "lint", "format", "publish", "release", "merge", "rebase", "stash", "cherry-pick".

The agent would explain code instead of performing the requested action, with no indication to the user that their intent was misunderstood.

### 8.2 `chat` Maps to `/greet` with No Handler

**File:** `internal/perception/understanding_adapter.go:256`

```go
case "chat":
    return "/greet"
```

The verb `/greet` doesn't appear in any schema declaration or policy rule. Conversational messages produce a `user_intent` with verb `/greet` that no Mangle rule matches, resulting in no `next_action` being derived. The agent would appear to ignore the user.

### 8.3 VerbCorpus Catastrophic Fallback

**File:** `internal/perception/transducer.go:58-74`

If `SharedTaxonomy` fails to initialize, the entire verb corpus is reduced to a single entry: `/explain`. Every user input would be classified as "explain" -- the agent would be completely unable to perform mutations, tests, reviews, or other operations. The error is logged but the system continues in this severely degraded state without user notification.

### 8.4 `verify` Categorized as `/mutation`

**File:** `internal/perception/understanding_adapter.go:234-235, 276`

"Verify" maps to verb `/test` but category `/mutation`. Testing is typically read-only. The constitutional safety gate may apply mutation-level restrictions to what should be a read-only test operation.

---

## 9. High: JIT Compilation Gaps

### 9.1 Failed Atom Collection Produces Generic Prompt

**File:** `internal/prompt/compiler.go:736-741`

```go
if c.projectDB != nil {
    projectAtoms, err := c.loadAtomsFromDB(ctx, c.projectDB)
    if err != nil {
        logging.Get(logging.CategoryJIT).Warn("Failed to load project atoms: %v", err)
        // Continues without project atoms!
    }
}
```

If the project database is temporarily unavailable, the compiler silently produces a prompt with **no project-specific atoms**. The LLM receives a generic prompt with no domain knowledge about the current project -- file structure, coding conventions, architecture decisions, etc. This could cause the agent to produce code that doesn't match project conventions or misunderstands the codebase structure.

### 9.2 Cache May Return Stale Compiled Prompts

**File:** `internal/prompt/compiler.go:421`

```go
cacheKey := cc.Hash()
```

If `cc.Hash()` doesn't account for all relevant context (kernel-injected atoms, dynamic knowledge atoms), the cache may return a stale compiled prompt. Kernel-injected atoms are collected *after* the cache check (lines 479-486), so changes in kernel state between compilations may not invalidate the cache.

### 9.3 Context Fact Leakage Between Compilations

(See Section 7.4) Context facts asserted for one compilation persist and influence the next.

---

## 10. High: Virtual Predicate Architecture

### 10.1 Custom Wrapper vs Native External Predicates API

**File:** `internal/core/virtual_fact_store.go:61-86`

The virtual predicate system is a hand-rolled `factstore.FactStore` wrapper with a manual dispatch table and `factstore.Matches()` filtering. Mangle v0.4.0 introduced proper external predicates with filter pushdown.

**Current architecture:**
```
Mangle Engine → GetFacts() → virtualFactStore → manual Matches() → VirtualStore.Get() → handler
```

**Native architecture (v0.4.0+):**
```
Mangle Engine → External Predicate callback(query, resultCallback) → handler with bound args
```

**Benefits of migration:**
- Engine pushes bound arguments to handlers, reducing result sets
- Proper integration with stratification
- Eliminates manual `virtualPredicateHandlers` map
- Eliminates manual `factstore.Matches()` post-filtering
- Filter pushdown for complex queries

### 10.2 Virtual Predicate Error Fallthrough

**File:** `internal/core/virtual_fact_store.go:65-83`

When a virtual predicate query fails, the error is logged but execution falls through to the base store:
```go
if err != nil {
    logging.Get(logging.CategoryVirtualStore).Error(...)
    // Continue to base store - this is intentional fallback.
}
```

If the base store happens to contain stale facts for that predicate (from a previous session or evaluation), those stale facts will be returned instead of the error being propagated. The kernel would silently use outdated data.

---

## 11. Medium: Code Quality Issues

### 11.1 `fmt.Fprintf(os.Stderr)` Instead of Logger

**File:** `internal/mangle/engine.go:535`

```go
fmt.Fprintf(os.Stderr, "warning: fact store is %.1f%% of configured capacity (%d / %d)\n",
    utilization*100, e.factCount, e.config.FactLimit)
```

Bypasses the `go.uber.org/zap` logging infrastructure used everywhere else. Should use `logging.Get(logging.CategoryMangle).Warn(...)`.

### 11.2 Hardcoded Predicate Specs in Grammar

**File:** `internal/mangle/grammar.go:87-235` and `grammar.go:309-393`

~250 lines of hardcoded predicate specifications and name constants that duplicate schema declarations. These will drift out of sync as predicates are added. Should be generated from `ProgramInfo.Decls` at schema load time.

### 11.3 Duplicate `ReplaceFactsForFile*` Methods

**File:** `internal/mangle/engine.go:410-489`

Two nearly identical ~40-line methods differing only by whether `contentHash` is computed internally or passed in. Extract common logic.

### 11.4 `isBuiltin` Map Recreated Per Call

**File:** `internal/mangle/schema_validator.go:339-351`

Map literal created inside `isBuiltin()` on every invocation. Move to package-level `var`.

### 11.5 Custom `min()` Shadows Go 1.21+ Builtin

**File:** `internal/mangle/lsp.go:1049-1054`

```go
func min(a, b int) int { ... }
```

Go 1.24 (per `go.mod:3`) has a built-in `min()`. Delete the custom function.

### 11.6 Regex Not Precompiled

- `internal/mangle/grammar.go:672` -- `isNumeric` uses `regexp.MatchString` on every call
- `internal/mangle/feedback/prompt_builder.go:414` -- `factPattern` compiled inside function

Both should be package-level `var regex = regexp.MustCompile(...)`.

### 11.7 LSP Error Handling: Ignored JSON Unmarshal

**File:** `internal/mangle/lsp.go:857,871,887,914`

Multiple `json.Unmarshal` calls have error returns ignored. Malformed LSP messages cause silent failures.

### 11.8 Redundant Gas Limit System

**File:** `internal/mangle/engine.go:198-231`

The `Engine.Evaluate()` has its own gas limit system that manually counts facts before/after evaluation. But `kernel_eval.go:147` uses the built-in `engine.WithCreatedFactLimit(derivedFactLimit)`. The manual counting is redundant with the built-in mechanism.

### 11.9 Deprecated `convertValueToBaseTerm` Still Present

**File:** `internal/mangle/engine.go:693-696`

Marked deprecated but still defined. If nothing calls it, remove it.

### 11.10 Differential Engine: Naive 2-Stratum Stratification

**File:** `internal/mangle/differential.go:336-416`

80+ lines of design comments explaining that the `DifferentialEngine` only supports 2 strata (EDB=0, IDB=1). Cannot handle predicates that depend on negation of other IDB predicates. The upstream engine handles multi-stratum evaluation properly.

---

## 12. Medium: Concurrency & Safety

### 12.1 Lock-Release-Use Pattern

**File:** `internal/core/virtual_store.go` (lines 621-623, 638-640, 694-696, 715-717, 733-737, 777-780)

Multiple places acquire RLock, read a field, release the lock, then use the value. If another goroutine modifies the value between unlock and use, behavior is incorrect.

```go
v.mu.RLock()
value := v.someField
v.mu.RUnlock()
// ... use value without lock protection -- TOCTOU race ...
```

### 12.2 Non-Atomic Retract+Assert Sequences

**File:** `internal/core/shard_manager_tools.go:148-172`

```go
_ = sm.kernel.Retract("current_shard_type")   // Step 1
_ = sm.kernel.Retract("current_intent")       // Step 2
_ = sm.kernel.Assert(...)                      // Step 3
```

These are individually locked but not atomic as a group. Another goroutine could `Query` between steps 1 and 3, seeing an inconsistent state where `current_shard_type` is retracted but `current_intent` is not yet.

### 12.3 Goroutines Launched Without Context

**File:** `internal/core/spawn_queue.go:170,192`

Workers launched via `go sq.worker(i)` without `context.Context`. If `Stop()` is never called (panic path), workers leak.

### 12.4 Context Accepted But Not Propagated

**File:** `internal/core/virtual_store_predicates.go:533`

`HydrateSessionContext` accepts `ctx` but never passes it to child calls (`QuerySession`, `RecallSimilar`, `kernel.LoadFacts`). Cancelled contexts don't stop expensive database queries.

---

## 13. Low: Housekeeping

### 13.1 11 TODO Test Gaps

**File:** `internal/mangle/engine_test.go:309-362`

11 `// TODO: TEST_GAP:` comments for unimplemented tests: nil arguments, float coercion boundaries, string/atom ambiguity, fact limit enforcement, concurrent access, batch atomicity, unicode identifiers, float discontinuity.

### 13.2 Repeated Error Message Strings

**File:** `internal/mangle/engine.go:167,345,384,414,456`

`"no schemas loaded; call LoadSchema first"` appears at 5 call sites. Extract to `var errNoSchemas = errors.New(...)`.

### 13.3 Hardcoded EDB/IDB Predicate Sets in Proof Tree

**File:** `internal/mangle/proof_tree.go:217-248`

Two hardcoded maps classify predicates as EDB or IDB. Should be derived from `ProgramInfo.IdbPredicates`.

### 13.4 LSP `os.Exit(0)` on Exit

**File:** `internal/mangle/lsp.go:934`

Abrupt process exit without cleanup on LSP "exit" notification. Should allow deferred functions to run.

### 13.5 `AGENTS.md` for `internal/mangle/` References Nonexistent Functions

The AGENTS.md suggests `engine.EvalProgramNaive` and `engine.Run()` as valid entry points. Both are either deprecated or nonexistent. The AGENTS.md should be updated to reflect the actual API: `engine.EvalProgramWithStats`.

### 13.6 Proof Tree Hardcoded Predicate Sets

**File:** `internal/mangle/proof_tree.go:217-248`

Accept `*analysis.ProgramInfo` in the constructor and derive EDB/IDB classifications dynamically instead of maintaining hardcoded maps.

---

## 14. Upgrade Path & Prioritized Action Plan

### Phase 1: Immediate Fixes (High Impact, Low Effort)

| # | Issue | Files | Effort | Impact |
|---|-------|-------|--------|--------|
| 1 | Float coercion: add `ast.Float64()` path | `types.go`, `engine.go`, `kernel_facts.go` | 2h | Eliminates data loss for all numeric values |
| 2 | Add `time.Time`/`time.Duration` to `ToAtom()` | `types.go` | 1h | Enables temporal fact assertions |
| 3 | Add `/time`, `/duration`, `/float` to type bound switch | `engine.go:566-575` | 30m | Completes type mapping |
| 4 | Fix prompt atom `Decl` syntax | `language/mangle.yaml` | 1h | LLM generates valid Mangle code |
| 5 | Reconcile predicate arities in prompt atoms | Multiple yaml files | 2h | Consistent instructions to LLM |
| 6 | Replace `fmt.Fprintf(os.Stderr)` with logger | `engine.go:535` | 5m | Logging consistency |
| 7 | Delete custom `min()` | `lsp.go:1049-1054` | 1m | Go 1.24 compatibility |
| 8 | Precompile regexes | `grammar.go:672`, `prompt_builder.go:414` | 10m | Performance |
| 9 | Add `float64` case to `goToMangleTerm` | `virtual_store_graph.go:73-94` | 15m | Graph floats handled correctly |

### Phase 2: Error Handling & Fact Lifecycle (High Impact, Medium Effort)

| # | Issue | Files | Effort | Impact |
|---|-------|-------|--------|--------|
| 10 | Log errors on kernel mutations (replace `_ = err`) | 28 sites across `internal/core/` | 3h | Visibility into kernel state failures |
| 11 | Retract-before-assert pattern for TDD state | `tdd_loop.go` | 1h | Correct `next_action` derivation |
| 12 | Add pruning for heartbeats, diagnostics, test_state | `virtual_store.go`, `tdd_loop.go` | 3h | Prevents fact store bloat |
| 13 | Retract JIT context facts after compilation | `compiler.go` | 1h | Clean atom selection state |
| 14 | Create centralized fact arg extraction utilities | New file in `internal/types/` | 4h | Replaces 100+ `fmt.Sprintf("%v")` calls |
| 15 | Add type annotations to schema `.mg` files | All `defaults/*.mg` | 4h | Enables Mangle type checking |

### Phase 3: Architecture Improvements (High Impact, High Effort)

| # | Issue | Files | Effort | Impact |
|---|-------|-------|--------|--------|
| 16 | Update Mangle dependency to latest HEAD | `go.mod` | 4h | Unlocks temporal, time/duration, external predicates |
| 17 | Migrate virtual predicates to native external predicates API | `virtual_fact_store.go`, `virtual_store.go` | 2d | Filter pushdown, proper stratification |
| 18 | Implement temporal fact lifecycle (when upstream adopted) | `kernel_facts.go`, policy `.mg` files | 3d | Automatic fact expiry, interval-based queries |
| 19 | Fix perception fallback chain | `understanding_adapter.go`, `transducer.go` | 1d | Correct intent classification for uncommon verbs |
| 20 | Generate predicate specs from schema declarations | `grammar.go` | 1d | Eliminates schema drift |
| 21 | Atomic retract+assert operations | `kernel_facts.go` | 1d | Eliminates query-between-mutation races |

### Phase 4: Cleanup (Low Impact, Low Effort)

| # | Issue | Files | Effort |
|---|-------|-------|--------|
| 22 | Deduplicate `ReplaceFactsForFile*` | `engine.go` | 30m |
| 23 | Move `isBuiltin` map to package level | `schema_validator.go` | 10m |
| 24 | Use `WithCreatedFactLimit` in Engine | `engine.go` | 30m |
| 25 | Remove deprecated `convertValueToBaseTerm` | `engine.go` | 5m |
| 26 | Extract repeated error strings | `engine.go` | 15m |
| 27 | Fix LSP JSON unmarshal error handling | `lsp.go` | 30m |
| 28 | Fill 11 test gaps | `engine_test.go` | 4h |
| 29 | Update AGENTS.md to reference correct API | `internal/mangle/AGENTS.md` | 30m |
| 30 | Derive proof tree predicate sets from ProgramInfo | `proof_tree.go` | 1h |

---

## Appendix A: File Inventory

### `internal/mangle/` (36 files, ~400KB)

| File | Lines | Purpose |
|------|-------|---------|
| `engine.go` | ~880 | Main Mangle engine: fact store, query execution, schema loading, gas limits |
| `lsp.go` | ~920 | LSP server for `.mg` files: diagnostics, completion, go-to-definition, hover |
| `grammar.go` | ~700 | Grammar-Constrained Decoding: AtomValidator, PredicateSpec, RepairLoop |
| `differential.go` | ~600 | DifferentialEngine: incremental eval, stratum caching, COW snapshots, ChainedFactStore |
| `proof_tree.go` | ~400 | Proof tree tracer: derivation traces, ASCII/JSON rendering |
| `schema_validator.go` | ~380 | Schema drift prevention: validates rules use only declared predicates |
| `intent_routing.mg` | ~500 | Mangle rules for intent routing, persona selection, tool selection, TDD workflow |
| `campaign_intelligence.mg` | ~600 | Campaign intelligence predicates: churn analysis, safety, impact analysis |
| `engine_test.go` | ~370 | Engine tests |
| `schema_validator_test.go` | ~300 | Schema validator tests |
| `differential_test.go` | ~200 | Differential engine tests |
| `mangle_validation_test.go` | ~1000 | Comprehensive validation tests |
| `feedback/loop.go` | ~500 | FeedbackLoop: validate-retry cycle for LLM-generated Mangle with JIT predicate selection |
| `feedback/prompt_builder.go` | ~450 | Builds feedback prompts for LLM retry, extracts rules from responses |
| `feedback/pre_validator.go` | ~400 | Fast regex pre-validation catching atom/string confusion, aggregation errors |
| `feedback/types.go` | ~250 | ErrorCategory, ValidationError, RetryConfig, ValidationBudget, SynthMode |
| `feedback/error_classifier.go` | ~250 | Classifies Mangle compiler errors into structured categories |
| `feedback/normalize.go` | ~50 | NormalizeRuleInput: fixes Prolog negation, backslash escapes |
| `synth/compile.go` | ~350 | JSON spec -> AST clause compilation with parse+analysis validation |
| `synth/validate.go` | ~300 | Spec validation: predicates, variables, types, transforms |
| `synth/schema.go` | ~170 | JSON Schema generation for MangleSynth format |
| `synth/decoder.go` | ~120 | Decodes MangleSynth JSON from LLM responses |
| `synth/spec.go` | ~100 | Type definitions: Spec, ClauseSpec, AtomSpec, ExprSpec |
| `transpiler/sanitizer.go` | ~350 | Sanitizer: atom interning, aggregation repair, safety injection |

### Mangle Integration Points Across Codebase

| Metric | Count |
|--------|-------|
| Files importing `google/mangle` | 30 |
| Distinct sub-packages imported | 8 (`analysis`, `ast`, `builtin`, `engine`, `factstore`, `packages`, `parse`, `unionfind`) |
| `EvalProgramWithStats` call sites | 4 |
| `NewSimpleInMemoryStore` call sites | 13 |
| `kernel.Query()` call sites | 60+ |
| Virtual predicates registered | 10 |
| `parse.Unit()` call sites | 20+ |

## Appendix B: Upstream Mangle Release Timeline

| Release/Commit | Date | Key Features |
|----------------|------|-------------|
| v0.1.0 | Dec 2023 | First tagged release |
| v0.2.0 | Aug 2024 | Lattice support, `mg` interpreter |
| v0.3.0 | May 2025 | `.Type<>` syntax, `⟸` arrow syntax |
| v0.4.0 | Nov 2025 | External predicates, aggregation enhancements, map reducer **(currently pinned)** |
| `3c74c284` | Sep 2025 | Copybara import with internal improvements |
| `ea0330ca` | Sep 2025 | External predicates support |
| `3596fd99` | Sep 2025 | Arbitrary filter pushdown in external predicates |
| `a77833b8` | Sep 2025 | Non-reducer functions in aggregations |
| `3c1ec603` | Sep 2025 | Deprecate `EvalProgram` in favor of `EvalStratifiedProgramWithStats` |
| `36f2233f` | Dec 2025 | UTF-8 validation in AST serde |
| `5ae68167` | Jan 2026 | Rust architecture overhaul: WASM, pluggable storage |
| `dd902ac0` | Feb 2026 | Time & Duration built-in types (+30 functions) |
| `77dd1714` | Feb 2026 | Full DatalogMTL temporal reasoning |
| `29970168` | Feb 2026 | Latest (no public description) |
