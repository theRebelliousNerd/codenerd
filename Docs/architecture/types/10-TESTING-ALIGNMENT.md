# 10 — Testing Alignment: `internal/types`

> Last verified: **2026-07-13**

## 1. Commands

```powershell
go test ./internal/types/...
go test ./internal/types/ -count=1 -v
go test ./internal/types/ -cover
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
| `shard_test.go` | `SpawnPriority.String` only | Thin |

## 3. Coverage map vs surface

| Surface | Covered in-package? | Notes |
|---------|---------------------|-------|
| `ToAtom` typed args | **Yes** | Comprehensive |
| `ToAtom` nil/unknown | **Yes** | Explicit error tests |
| `ToAtom` containers (JSON) | **Partial** | Logic present; less table coverage than scalars |
| Name heuristic / file ext | **Yes** | Valid/invalid tables |
| `Fact.String` | **Yes** | Multiple cases |
| Extract helpers | **Yes** | Good tables |
| Session context ctx helpers | **Yes** | Set/get/nil |
| `KernelFact.ToFact` | **Yes** | Basic + multi-arg |
| `SpawnPriority.String` | **Yes** | Including unknown |
| `NewKernelTx` panic path | **No** (in-package) | Covered via core tests / manual |
| Interface method sets | **No** | Compile-time `var _` in implementers |
| SessionContext field semantics | **No** | Integration elsewhere |
| Optional LLM interfaces | **No** | Provider packages |

## 4. Gaps and recommended tests

| Priority | Test idea |
|----------|-----------|
| P1 | `ToAtom` for `map[string]any` / `[]string` JSON round-trip shape |
| P1 | `NewKernelTx` with mock non-transactor → panic (or document exclusive core coverage) |
| P2 | `ExtractBool` `"true"`/`"false"` without slash (already partially in extract_test) |
| P2 | Hierarchical name `/a/b` vs deep path `/a/b/c/d` dedicated cases (overlap exists) |
| P3 | Golden test: Fact.String vs ToAtom consistency for shared samples |

## 5. Testing principles for this package

1. Prefer table-driven pure tests — no kernel boot required.
2. When changing `ToAtom`, add a failing case first for the poison class you fixed.
3. Do not require network or CGO for `types` tests.
4. Interface compliance belongs next to implementers (`var _ types.X = (*T)(nil)`).

## 6. Alignment with north star

Tests protect **executive integrity** (fact encoding), not LLM creativity. That is correctly prioritized: conversion and extract tests dominate; no flaky LLM tests in this package.
