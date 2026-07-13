# 10 — Testing Alignment: mangle

> Last verified: 2026-07-13

## Commands

```powershell
# Entire mangle tree
go test ./internal/mangle/...

# Subpackages
go test ./internal/mangle/feedback/...
go test ./internal/mangle/synth/...
go test ./internal/mangle/transpiler/...

# Stress / race (when investigating parse or concurrent access)
go test ./internal/mangle -race -run 'Parse|Concurrent' -count=1

# Kernel differential integration
go test ./internal/core -run 'Diff|Differential' -count=1

# Policy corpus via Engine
go test ./internal/core/defaults/policy/... -count=1
```

## Test map

### Engine (`engine_test.go`)

| Area | Coverage |
|------|----------|
| NewEngine / DefaultConfig | Smoke |
| LoadSchemaString, AddFact(s), Query, GetFacts | Happy path |
| Clear, stats, ToggleAutoEval, PushFact, QueryFacts | API surface |
| Fact limits, derived gas | Resource bounds |
| ConcurrentAccess | Mutex behavior |
| Float/string coercion, unicode, invalid UTF-8 | Encoding edges |
| Batch atomicity / partial failure | Error handling |
| Zero/negative timeouts and limits | Config edges |

### Differential (`differential_test.go`, `fact_store_proxy_test.go`)

| Area | Coverage |
|------|----------|
| NewDifferentialEngine | Requires schema |
| Stratification / incremental | Core behavior |
| Snapshot isolation | Copy semantics |
| Lazy loading / proxy | Virtual predicates |
| KnowledgeGraph | Construction |

### Validation corpus (`mangle_validation_test.go`)

**Critical:** loads real schemas/policy/shard `.mg` sources:

- Parses schemas + policy alone and combined.
- Duplicate Decl detection.
- Domain GL files (coder, tester, reviewer, chaos).
- TDD repair, spreading activation, safe negation, delegate_task.
- Impact analysis transitive closure.
- **Constitutional safety** rules.
- Campaign phase eligibility.
- Engine with real policy; world model; block commit; user intent; symbol graph; focus; autopoiesis; strategy selection.

This suite is the package’s **integration bar** against the living policy corpus.

### Grammar / GCD

- `grammar_test.go`, `grammar_helpers_test.go`, `grammar_argtype_test.go`, `grammar_internal_test.go` (benchmarks), `grammar_fuzz_test.go` (`FuzzParseAtom`).

### Parse lock

- Concurrent ParseUnit / ParseAtom / mixed / error paths.

### Feedback

- Loop success/retry, budget, CanRetryPrompt, classifier, pre-validator, prompt builder, normalize, JIT selector tests, benchmarks.

### Synth / transpiler

- Compile, decode (including from-response), validate, schema, sanitizer atoms.

### Other

- `lsp_test`, `proof_tree_test`, `schema_validator_test`, `inspect_test`, `intent_*_test`, `torture_test`, `simd_intersect_test`, `verification_logic_test`.

## Gaps

| Gap | Priority |
|-----|----------|
| Explicit test that Sanitizer under `-race` with concurrent Engine.LoadSchema is safe after ParseUnit migration | P0 after fix |
| Diff path asserts gas limit option once forwarded | P1 |
| intent_routing.mg loaded in runtime boot test | P2 |
| End-to-end CLI mangle-check in CI documented here as optional (slow) | P3 |
| Unified vs legacy path behavioral parity matrix | P1 |

## Alignment with principles

| Principle | Test evidence |
|-----------|---------------|
| Decl before use | Engine undeclared predicate tests; schema validator |
| Forbidden heads | Constitutional / validation suite |
| Gas limits | `TestDerivedFactsGasLimit`, fact limit tests |
| Concurrent parse | `parse_lock_test` |
| Feedback termination | budget tests in feedback package |

## Recommendation for handoff

Before claiming mangle changes green:

1. `go test ./internal/mangle/...`
2. If touching eval/diff: `go test ./internal/core -run Diff -count=1`
3. If touching Decl/forbidden: relevant cases in `mangle_validation_test.go` or schema_validator tests
