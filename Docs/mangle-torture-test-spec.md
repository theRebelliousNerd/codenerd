# Mangle Engine Torture Test Specification

**File:** `internal/mangle/torture_test.go`
**Tests:** 119
**Lines:** ~2840
**Run command:**
```bash
CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers" go test ./internal/mangle/... -run TestTorture -count=1 -race -timeout 120s
```

## Design Philosophy

Modeled after GCC's `gcc.c-torture/{compile,execute}` and `gcc.dg/noncompile` test suites. Every test is deterministic and requires no external resources (no LLM calls, no network, no filesystem). Tests are designed to catch regressions in the Mangle engine wrapper (`internal/mangle/engine.go`), the differential engine (`differential.go`), schema validation (`schema_validator.go`), and supporting data structures.

All tests run clean with `-race` to detect data races in concurrent code paths.

## Test Matrix

| # | Category | Count | Convention | Purpose |
|---|----------|-------|------------|---------|
| 1 | Parse | 20 | `TestTorture_Parse_*` | Programs that MUST parse without error |
| 2 | Eval | 25 | `TestTorture_Eval_*` | Programs that MUST derive expected facts |
| 3 | Error | 7 | `TestTorture_Error_*` | Programs that MUST fail at parse/analysis/stratification |
| 4 | Lifecycle | 11 | `TestTorture_Lifecycle_*` | Engine state machine invariants |
| 5 | Differential | 11 | `TestTorture_Differential_*` | Incremental evaluation engine |
| 6 | SchemaValidator | 14 | `TestTorture_SchemaValidator_*` | Schema drift prevention + safety |
| 7 | TypeSystem | 8 | `TestTorture_TypeSystem_*` | Boundary value handling |
| 8 | Concurrency | 6 | `TestTorture_Concurrency_*` | Race detector targets |
| 9 | Engine | 6 | `TestTorture_Engine_*` | Engine wrapper edge cases |
| 10 | ChainedFactStore | 7 | `TestTorture_ChainedFactStore_*` | Overlay/base store pattern |
| 11 | KnowledgeGraph | 2 | `TestTorture_KnowledgeGraph_*` | Stratum layer invariants |
| 12 | FactStoreProxy | 2 | `TestTorture_FactStoreProxy_*` | Lazy predicate loading |

---

## Category 1: Parse Torture (20 tests)

Programs that MUST parse without error. Validates that the `parse.Unit()` pipeline accepts all valid Mangle syntax forms.

| Test | What It Validates |
|------|-------------------|
| `Parse_EmptySchema` | Empty input is a valid program (GCC parallel: empty translation unit) |
| `Parse_CommentOnly` | Comment-only files parse cleanly |
| `Parse_SingleDecl` | Minimal `Decl simple(X).` declaration |
| `Parse_DeclWithBound` | Bound type annotations: `bound [/string, /number, /float64]` |
| `Parse_MultipleBoundAlternatives` | Union type signatures: `bound [...] bound [...]` |
| `Parse_DeclWithMode` | Mode descriptors: `descr [mode("-", "-")]` |
| `Parse_GroundFact` | Ground fact assertion: `color(/red).` |
| `Parse_NameConstants` | All valid `/atom` forms including underscores and digits |
| `Parse_SimpleRule` | Basic recursive rule (ancestor/parent) |
| `Parse_NegationInRule` | Stratified negation: `!admin(X)` syntax |
| `Parse_Comparison` | Comparison operators in rule body: `V >= 90` |
| `Parse_AggregationPipeline` | Full `|> do fn:group_by(), let Total = fn:Sum(Amount).` pipeline |
| `Parse_CountAggregation` | `fn:Count()` aggregation (note: parses but requires lowercase for analysis) |
| `Parse_CollectAggregation` | `fn:collect(Tag)` list collection |
| `Parse_AnonymousVariable` | Anonymous variable `_` in rule body |
| `Parse_ArithmeticBuiltins` | `fn:plus(X, 1)` arithmetic expressions |
| `Parse_MultiClauseUnion` | Multiple rules for same head = UNION semantics |
| `Parse_DeeplyNestedProgram` | Large program with 50 declarations + 20 chain rules |
| `Parse_UnicodeInStrings` | Unicode string literals: `"Hello, 世界"`, `"café"`, `"αβγ"` |
| `Parse_MixedTypesInDecl` | All 5 bound types in one declaration |

### Coverage Rationale
Covers the full syntactic surface area of Mangle: declarations, facts, rules, negation, aggregation, built-in functions, anonymous variables, and unicode. The "deeply nested" test validates the parser doesn't stack-overflow on realistic program sizes.

---

## Category 2: Eval Torture (25 tests)

Programs that MUST parse, evaluate via stratified bottom-up fixpoint, and produce exactly the expected derived facts. This is the largest category because correct inference is the core contract of the Mangle engine.

| Test | What It Validates |
|------|-------------------|
| `Eval_TransitiveClosure` | Classic TC: 3 edges derive 6 reachable pairs |
| `Eval_DiamondDependency` | Diamond graph: A->B, A->C, B->D, C->D; 5 unique pairs, (A,D) deduplicated |
| `Eval_NegationSafe` | Safe negation: 3 users, 1 admin -> 2 regular_users. Alice excluded |
| `Eval_NegationEmptyBase` | Negation over empty predicate -> all candidates selected |
| `Eval_DoubleNegation` | Two-stratum negation chain: `!rejected(X)` -> `confirmed(X)` |
| `Eval_ComparisonOperators` | `>=`, `<`, `=` with int64 values. Subtests: pass/fail/perfect |
| `Eval_Arithmetic` | `fn:mult(X, 2)` and `fn:plus(X, 10)` including zero and negative inputs |
| `Eval_ConstantRule` | Rule producing constant `/yes` from zero-arity trigger |
| `Eval_MultiRuleUnion` | Two rules for `combined/1` with overlapping inputs -> 3 deduplicated facts |
| `Eval_DeepRecursion` | 100-node chain (99 edges). Verifies engine handles deep recursion without stack overflow |
| `Eval_SelfLoop` | Self-edge: `edge("a", "a")`. Reachable should contain `("a","a")` (no infinite loop) |
| `Eval_MutualRecursion` | Two predicates referencing each other without negation |
| `Eval_NameConstantUnification` | `/atom` constants unify correctly (vs string confusion bug) |
| `Eval_BoundedRecursion` | `fn:plus(D1, 1)` with `D < 5` depth limit. Verifies termination |
| `Eval_ClosedWorldAssumption` | `!known(X)` derives `unknown(X)` for unmatched candidates |
| `Eval_FactDeduplication` | Same fact added twice -> only 1 stored |
| `Eval_MultiJoinRule` | 3-way join: `assigned(P,T), project_lang(P,L), lang_tool(L,Tool)` |
| `Eval_AggregationCount` | `fn:count()` over 3 items = 3 (**lowercase only** -- uppercase parses but fails analysis) |
| `Eval_AggregationSum` | `fn:sum(Amount)` over [10, 20, 30] = 60 |
| `Eval_AggregationGroupBySum` | `fn:group_by(Region)` + `fn:sum(Amount)`: north=300, south=150 |
| `Eval_AggregationGroupByCount` | `fn:group_by(Category)` + `fn:count()`: click=3, scroll=2 |
| `Eval_AggregationMinMax` | `fn:min()` and `fn:max()` per player group |
| `Eval_AggregationCollect` | `fn:collect(Tag)` groups tags into lists per item |
| `Eval_AggregationEmptyInput` | Aggregation over empty set: no crash, 0 or no results |
| `Eval_AggregationWithFilter` | Filter (`Amount > 50`) before aggregation pipeline |

### Coverage Rationale
Tests 8 of the 8 aggregation functions documented in AGENTS.md. The aggregation tests specifically exercise the `|> do fn:group_by(), let X = fn:*()` pipeline syntax that is the #2 most common AI hallucination point. The casing bug (uppercase parses but fails analysis) is explicitly documented in the count test comment.

---

## Category 3: Error Torture (7 tests)

Programs that MUST be rejected. Validates that the engine correctly rejects invalid programs at the appropriate stage (parse, analysis, or stratification).

| Test | Rejection Stage | What It Validates |
|------|-----------------|-------------------|
| `Error_SyntaxError` | Parse | Missing period on `Decl foo(X)` |
| `Error_InvalidAtomSyntax` | Parse | Uppercase predicate name `Decl Valid(X).` |
| `Error_UnstratifiableNegation` | Stratification | Negative cycle: `winning -> losing -> NOT winning` |
| `Error_ArityMismatchInRule` | Analysis | Binary predicate used with 1 arg: `binary(X)` |
| `Error_UndeclaredPredInRule` | Analysis | Rule body uses undeclared predicate |
| `Error_UnsafeHeadVariable` | Analysis | Head variable `Y` not bound in body |
| `Error_UnsafeNegation` | Analysis | Self-negation: `bad(X) :- !bad(X).` (X unbound) |

### Coverage Rationale
Covers the top 4 safety violations from AGENTS.md Section 150: syntax errors, stratification cycles, arity mismatches, and unsafe variable binding. These are the errors that "compile successfully but return empty results" or crash at runtime if not caught.

---

## Category 4: Lifecycle Torture (11 tests)

Engine state machine invariants. Validates the correct ordering and behavior of `NewEngine`, `LoadSchemaString`, `AddFact`, `GetFacts`, `Clear`, `Reset`, and `Close`.

| Test | Invariant |
|------|-----------|
| `Lifecycle_AddFactBeforeSchema` | AddFact without schema -> error (not panic) |
| `Lifecycle_QueryBeforeSchema` | GetFacts without schema -> error |
| `Lifecycle_ClearPreservesSchema` | Clear() removes facts but preserves schema declarations |
| `Lifecycle_ResetWipesSchema` | Reset() removes both facts and schema |
| `Lifecycle_ResetThenReload` | Reset -> LoadSchemaString(new) -> AddFact works |
| `Lifecycle_DoubleClear` | Two consecutive Clear() calls -> no panic |
| `Lifecycle_CloseAndReuse` | Close() -> operations should fail gracefully |
| `Lifecycle_IncrementalSchemaLoading` | Multiple LoadSchemaString calls accumulate predicates |
| `Lifecycle_SchemaAfterFacts` | Loading new schema after facts clears stale data |
| `Lifecycle_GetStatsEmpty` | GetStats() on empty engine returns valid struct |
| `Lifecycle_QueryImmediatelyAfterClear` | Clear -> query returns 0 facts (not stale) |

### Coverage Rationale
The engine is called from multiple goroutines during boot, shard execution, and fact management. These tests ensure that ordering violations produce errors rather than panics or corrupted state.

---

## Category 5: Differential Engine Torture (11 tests)

Incremental evaluation engine (`differential.go`). Tests snapshot isolation, delta application, and concurrent operations.

| Test | What It Validates |
|------|-------------------|
| `Differential_EmptySnapshot` | Snapshot of empty engine is non-nil |
| `Differential_RequiresSchema` | Creating DifferentialEngine without schema -> error |
| `Differential_SnapshotIsolation` | Snapshot + base engine are independent |
| `Differential_ApplyDelta` | Single fact applied via delta is stored in strata |
| `Differential_ApplyDeltaEmpty` | Empty delta (nil and []) -> no error |
| `Differential_AddFactIncremental` | Convenience wrapper AddFactIncremental works |
| `Differential_QueryWithDeclaredMode` | Query with mode descriptor returns bindings |
| `Differential_QueryUndeclaredPredicate` | Query for undeclared predicate -> error |
| `Differential_SnapshotMutationIsolation` | Facts added after snapshot are NOT visible in snapshot |
| `Differential_MultipleDeltas` | 10 sequential deltas -> all facts stored |
| `Differential_RegisterVirtualPredicate` | Virtual predicate wraps base store with FactStoreProxy |

### Coverage Rationale
The differential engine is used for speculative evaluation (Shadow Mode) and incremental fact updates. Snapshot isolation is critical for correctness -- a snapshot must be a frozen view that doesn't see subsequent mutations.

---

## Category 6: Schema Validator Torture (14 tests)

Schema drift prevention, forbidden head detection, and arity enforcement.

| Test | What It Validates |
|------|-------------------|
| `SchemaValidator_ForbiddenHead` | 4 protected predicates (permitted, safe_action, admin_override, pending_action) rejected in learned rules |
| `SchemaValidator_ArityMismatch` | CheckArity(pair, 3) fails when pair is declared with arity 2 |
| `SchemaValidator_UnknownPredicateArity` | Unknown predicate returns arity -1, CheckArity passes (no data = no check) |
| `SchemaValidator_HotLoadEmpty` | HotLoadRule("") -> error |
| `SchemaValidator_HotLoadNoPeriod` | HotLoadRule without trailing period auto-appends period |
| `SchemaValidator_UndefinedBodyPredicate` | Rule using undeclared body predicate -> rejected |
| `SchemaValidator_ValidProgram` | ValidateProgram with well-formed rules + Decls -> passes |
| `SchemaValidator_AllForbiddenHeads` | Iterates ALL entries in `forbiddenLearnedHeads` map -- exhaustive coverage |
| `SchemaValidator_IsDeclared` | IsDeclared returns true/false correctly for known/unknown predicates |
| `SchemaValidator_MultiRuleValidation` | ValidateRules: 1 valid + 1 invalid -> exactly 1 error |
| `SchemaValidator_LearnedFromFile` | Predicates in head of learned rules are implicitly declared |
| `SchemaValidator_CommentAndBlankLines` | Empty strings and comments are valid (no-op, not error) |
| `SchemaValidator_SetPredicateArity` | SetPredicateArity -> GetArity -> CheckArity round-trip |
| `SchemaValidator_GetDeclaredPredicates` | Returns all 3 declared predicates, no extras |

### Coverage Rationale
The schema validator is the Constitutional Gate's first line of defense. Forbidden head predicates (`permitted`, `safe_action`, etc.) MUST be rejected when submitted as "learned rules" to prevent policy injection attacks. The `AllForbiddenHeads` test exhaustively iterates the forbidden map to catch any additions that lack test coverage.

---

## Category 7: Type System Torture (8 tests)

Boundary value handling for Mangle's type system. Ensures the engine doesn't panic on edge-case values.

| Test | Boundary Value |
|------|----------------|
| `TypeSystem_NaN` | `math.NaN()` -- store or fail gracefully, never panic |
| `TypeSystem_Infinity` | `+Inf` and `-Inf` -- store or fail gracefully |
| `TypeSystem_MaxInt64` | `MaxInt64`, `MinInt64`, `0`, `-1` -- all store correctly |
| `TypeSystem_BoolValues` | `true` and `false` stored as 2 distinct facts |
| `TypeSystem_IntCoercion` | Go `int` (native) coerced to `int64` without error |
| `TypeSystem_EmptyString` | Empty string `""` is a valid fact argument |
| `TypeSystem_ZeroArityFact` | Zero-arity `Decl flag().` + `AddFact("flag")` |
| `TypeSystem_DuplicateFactDedup` | Same fact added twice -> only 1 stored (set semantics) |

### Coverage Rationale
These values are supplied by Go code (transducers, virtual stores) and could panic the engine if type conversion is incorrect. NaN and Infinity are particularly dangerous because they can break comparison operators and aggregation functions.

---

## Category 8: Concurrency Torture (6 tests)

Race detector targets. All designed to be run with `-race` flag. Tests concurrent access patterns that occur during normal codeNERD operation (multiple shards querying the kernel simultaneously).

| Test | Concurrent Pattern |
|------|-------------------|
| `Concurrency_AddFactAndQuery` | 5 writers (100 facts each) + 3 readers (50 queries each) simultaneously |
| `Concurrency_ConcurrentSchemaLoad` | 5 goroutines loading different schemas concurrently |
| `Concurrency_ClearDuringAddFact` | Writer racing with 5 Clear() calls |
| `Concurrency_QueryWithTimeout` | 10 goroutines querying with 2s context timeout |
| `Concurrency_ConcurrentQueryAndRecompute` | 5 readers + 3 recompute goroutines against 50 seeded facts |
| `Concurrency_DifferentialDeltaConcurrent` | 5 delta writers + 3 snapshot-takers on differential engine |

### Coverage Rationale
The Mangle engine is protected by `sync.RWMutex`. These tests validate that the lock discipline is correct under contention. The `ClearDuringAddFact` test is specifically designed to catch a common mutex ordering bug where Clear() invalidates state that AddFact() is reading.

---

## Category 9: Engine Wrapper Torture (6 tests)

Edge cases specific to the codeNERD engine wrapper (`engine.go`) rather than upstream Mangle.

| Test | What It Validates |
|------|-------------------|
| `Engine_RecomputeWithoutSchema` | RecomputeRules() without schema -> error |
| `Engine_ToggleAutoEval` | AutoEval ON: facts auto-derive. OFF: manual RecomputeRules needed |
| `Engine_QueryEmpty` | Query on engine with schema but no facts -> 0 bindings |
| `Engine_QueryWithLeadingQuestion` | Query with `?` prefix is handled (Mangle convention) |
| `Engine_DerivedFactCount` | GetDerivedFactCount/ResetDerivedFactCount counter works |
| `Engine_QueryFactsFiltered` | QueryFacts(predicate, firstArg) returns filtered results |

---

## Category 10: ChainedFactStore Torture (7 tests)

Overlay/base store pattern used for differential evaluation and sandboxed fact management.

| Test | What It Validates |
|------|-------------------|
| `ChainedFactStore_BasicOverlay` | GetFacts returns facts from both base and overlay |
| `ChainedFactStore_Contains` | Contains checks both base and overlay, returns false for missing |
| `ChainedFactStore_ListPredicates` | 3 predicates across base+overlay -> 3 unique (deduplicated) |
| `ChainedFactStore_EstimateFactCount` | 5 base + 3 overlay = 8 estimated |
| `ChainedFactStore_Merge` | Merge copies source facts into overlay |
| `ChainedFactStore_MultipleBases` | 2 base stores + overlay -> all 3 sources accessible |
| `ChainedFactStore_AddGoesToOverlay` | Add() writes to overlay, not base (immutability) |

---

## Category 11: KnowledgeGraph Torture (2 tests)

Stratum layer invariants for the in-memory knowledge graph.

| Test | What It Validates |
|------|-------------------|
| `KnowledgeGraph_NewIsEmpty` | New graph is non-nil, not frozen, 0 facts |
| `KnowledgeGraph_AddAndRetrieve` | Add fact -> Contains returns true, count = 1 |

---

## Category 12: FactStoreProxy Torture (2 tests)

Lazy predicate loading via registered loader functions (used for virtual predicates like `file_content`).

| Test | What It Validates |
|------|-------------------|
| `FactStoreProxy_LazyLoading` | Registered loader called exactly once on first query |
| `FactStoreProxy_NoLoaderForPredicate` | Facts added directly work without a registered loader |

---

## Relationship to AGENTS.md Section 150 (AI Failure Modes)

The torture tests directly validate defenses against the documented AI hallucination patterns:

| AGENTS.md Failure Mode | Torture Test Coverage |
|------------------------|----------------------|
| 1. Atom vs String confusion | `Eval_NameConstantUnification` |
| 2. Aggregation syntax errors | 8 `Eval_Aggregation*` tests |
| 3. Type declaration syntax | 4 `Parse_Decl*` tests |
| 4. Unsafe variables in negation | `Error_UnsafeNegation`, `Error_UnsafeHeadVariable` |
| 5. Stratification violations | `Error_UnstratifiableNegation` |
| 6. Infinite recursion | `Eval_DeepRecursion`, `Eval_BoundedRecursion`, `Eval_SelfLoop` |
| 7. Cartesian product explosions | `Eval_MultiJoinRule` (validates correct join, not perf) |
| 10. Closed World Assumption | `Eval_ClosedWorldAssumption` |
| 11. Mangle as HashMap anti-pattern | Not directly tested (architectural, not engine-level) |

## Known Gaps (Future Work)

- No tests for `fn:concat`, `fn:append`, `fn:pair`, `fn:map_get` built-in functions
- No tests for `:match_field` / `:match_entry` struct access
- No tests for `fn:len` on lists
- No negative test for float64 comparison bug (comparisons only work on NumberType)
- KnowledgeGraph and FactStoreProxy categories have minimal coverage (2 tests each)
- No fuzz testing (could generate random Mangle programs and assert no panics)
