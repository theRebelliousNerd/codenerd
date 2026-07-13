# retrieval — Testing Alignment

> Last verified: **2026-07-13**

## 1. Commands

```powershell
# Default unit/package tests (excludes //go:build integration)
go test ./internal/retrieval/...

# Race detector (cache + search concurrency)
go test -race ./internal/retrieval/...

# Integration suite
go test -tags=integration ./internal/retrieval/...

# Benchmarks
go test -bench=BenchmarkExtractKeywords -benchmem ./internal/retrieval/

# Verbose single test
go test ./internal/retrieval/ -run TestFindRelevantFiles_EndToEnd -v
```

No CGO / sqlite headers required for this package.

## 2. Test file map

| File | Build tag | Focus |
|------|-----------|-------|
| `sparse_test.go` | default | Extract weights/order, path normalize, cache TTL/evict, parseRipgrep counts, RankFiles tiers, empty/null/ReDoS, cancel, huge parse, concurrency |
| `sparse_search_test.go` | default | ScanBuffer, alphanumeric/boundary helpers, AllKeywords, SearchKeywords, FindRelevantFiles e2e on temp dirs |
| `sparse_bench_test.go` | default | `BenchmarkExtractKeywords` |
| `tiered_context_test.go` | default | Cache clear, extract mentioned, BuildContext e2e |
| `tiered_context_coverage_test.go` | default | Config defaults, GetFilesByTier/Top/Paths, LoadContent, findFile/import/symbol helpers |
| `sparse_integration_test.go` | `integration` | testify suite: search, no match, find relevant, exclusions, multi-file tree, cancel hang guard |

## 3. Coverage character

### Well covered

- Keyword extraction golden paths (files, errors, funcs, methods, quoted)
- Ranking score order + tier-1 mentioned boost
- Cache TTL, eviction, clear, concurrent Get/Set
- ScanBuffer edge cases (empty kw, multi match, not found)
- Word boundary positive/negative
- Context cancel/timeout on searchSingleKeyword
- Tiered builder helpers and LoadContent byte budget
- Integration exclusions + cancel hang (with tag)

### Thin / missing

| Gap | Notes |
|-----|-------|
| SIMD scanner path | Not exercised unless `-tags=simd` on amd64 |
| Embedding T4 | Placeholder; no fake embedding engine tests |
| Real multi-GB repo perf | Only unit-scale temp trees |
| Soft-error aggregation in SearchKeywords | Errors logged; hard to assert |
| Chat seed integration | Lives outside package; no retrieval package test for LoadFacts mapping |
| Go/TS import resolution | N/A until implemented |
| Binary/large file behavior | No max-size policy to test |

## 4. Remediated gap comments

`sparse_test.go` documents a “Marathon 36” remediation list (empty inputs, malformed colons, concurrency, huge output, cancel, ReDoS, null bytes, empty workDir, case weights). Corresponding tests exist in the same file.

## 5. Alignment to principles

| Principle | Test support |
|-----------|--------------|
| P5 Bounded work | timeout/cancel tests |
| P6 Word boundary | unit tests |
| P8 Cache safety | clone + concurrency |
| P9 Scanner parity | generic path well tested; SIMD not CI-default |

## 6. Suggested additions (backlog)

1. Table test for `determineTier` score boundaries (2.0 / 1.0).  
2. `SearchKeywords` with one keyword failing timeout while another succeeds — assert soft-fail aggregation.  
3. `BuildContext` with only Python imports across packages.  
4. Cross-package test: seed facts from ExtractKeywords match Decl arity.  
5. Optional simd build job.

## 7. CI expectation

Treat `go test ./internal/retrieval/...` as **required** for retrieval changes. Integration tag optional nightly / heavy job.
