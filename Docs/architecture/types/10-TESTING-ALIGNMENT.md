# 10 — Testing Alignment: `internal/types`

> Last verified: **2026-08-15**

## 1. Commands

```powershell
go test ./internal/types/...
go test ./internal/types/ -count=1 -v
go test ./internal/types/ -cover

# the two repo-wide ratchets (they walk every .go file and the .mg corpus)
go test ./internal/types/ -run 'TestFactConventions|TestKernelTransactor' -v
```

Related implementer tests (not in package, but exercise contracts):

```powershell
go test ./internal/core/ -run 'Transaction|CortexKernel|ToAtom|Fact' -count=1
go test ./internal/persist/factsnap/... -count=1
```

## 2. Existing test inventory

| File | Focus | Strength |
|------|-------|----------|
| `types_test.go` | Name validation, Fact.String, ToAtom core, KernelFact.ToFact | Core path |
| `types_comprehensive_test.go` | Full ToAtom matrix (strings, names, numbers, floats, bool, time, duration, nil, unknown, mixed); String edge cases; session context helpers; ArgName/ArgFloat64 | **Primary** |
| `extract_test.go` | All Extract*/Arg*/StripAtomPrefix tables | Strong |
| `container_toatom_test.go` | map/slice JSON encoding, nil-vs-empty, named error for unencodable contents | Strong |
| `mangle_roundtrip_external_test.go` | `package types_test`; every arg type's `Fact.String` must re-parse as Mangle (uses `mangle.ParseUnit`, the serialized parser) | Strong |
| `mangle_string_test.go` | `MangleString` vs `MangleAtom` vs shape inference | Strong |
| `ctxkeys_test.go` | Typed key set/read, legacy string fallback, foreign-key collision | Strong |
| `example_test.go` | Executed godoc examples for ToAtom / NewKernelTx / TransactorOf | Doc + test |
| `fact_conventions_guard_test.go` | **Repo-wide** Decl conformance, `%v` fact args, `MangleAtom` asserts on query results | Ratchet |
| `kernel_transactor_guard_test.go` | **Repo-wide** `KernelTransactor` conformance for every `types.Kernel` impl | Ratchet |
| `typestest/mockkernel_test.go` | Transaction buffering/atomicity, `NewKernelTx` panic path | Strong |
| `shard_test.go` | `SpawnPriority.String` only | Thin |

## 3. Coverage map vs surface

| Surface | Covered in-package? | Notes |
|---------|---------------------|-------|
| `ToAtom` typed args | **Yes** | Comprehensive |
| `ToAtom` nil/unknown | **Yes** | Explicit error tests |
| `ToAtom` containers (JSON) | **Yes** | Tables for map/slice/nil/unencodable + JSON round trip |
| Name heuristic / file ext | **Yes** | Valid/invalid tables |
| `Fact.String` | **Yes** | Multiple cases |
| Extract helpers | **Yes** | Good tables |
| Session context ctx helpers | **Yes** | Set/get/nil |
| `KernelFact.ToFact` | **Yes** | Basic + multi-arg |
| `SpawnPriority.String` | **Yes** | Including unknown |
| `NewKernelTx` panic path | **Yes** | `typestest` uses an embedded-interface kernel with no `Transaction()` |
| Typed context keys | **Yes** | Typed + legacy reads, empty/absent semantics |
| `Fact.String` ↔ Mangle parser parity | **Yes** | Every supported arg type must re-parse |
| Interface method sets | **No** | Compile-time `var _` in implementers |
| SessionContext field semantics | **No** | Integration elsewhere |
| Optional LLM interfaces | **No** | Provider packages |

## 4. Gaps and recommended tests

| Priority | Test idea | Status |
|----------|-----------|--------|
| P1 | `ToAtom` for `map[string]any` / `[]string` JSON round-trip shape | **Done** (`container_toatom_test.go`) |
| P1 | `NewKernelTx` with mock non-transactor → panic | **Done** (`typestest/mockkernel_test.go`) |
| P2 | `ExtractBool` `"true"`/`"false"` without slash | Partially in `extract_test.go` |
| P2 | Hierarchical name `/a/b` vs deep path `/a/b/c/d` dedicated cases | Overlap exists |
| P3 | Golden test: `Fact.String` vs `ToAtom` consistency for shared samples | Partly done — both now JSON-encode containers; a shared golden table is still open |

## 5. Testing principles for this package

1. Prefer table-driven pure tests — no kernel boot required.
2. When changing `ToAtom`, add a failing case first for the poison class you fixed.
3. Do not require network or CGO for `types` tests.
4. Interface compliance belongs next to implementers (`var _ types.X = (*T)(nil)`). Where that
   cannot reach — `types` can never name its own implementers — a repo-wide AST ratchet stands in,
   with an explicit baseline that records WHY each existing violation survives.
5. Parsing Mangle inside a test means `mangle.ParseUnit`, never `parse.Unit`: the upstream ANTLR
   prediction cache is process-global and mutated while parsing. That forces such tests into the
   external `types_test` package, since `internal/mangle` imports `internal/types`.

## 6. Alignment with north star

Tests protect **executive integrity** (fact encoding), not LLM creativity. That is correctly prioritized: conversion and extract tests dominate; no flaky LLM tests in this package.
