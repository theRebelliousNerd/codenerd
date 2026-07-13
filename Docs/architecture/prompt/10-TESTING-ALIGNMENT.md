# prompt — Testing Alignment

> Last verified: **2026-07-13**

## Commands

```powershell
# Unit + gap tests for package
go test ./internal/prompt/...

# Subpackage
go test ./internal/prompt/sync/...

# Consumer packages
go test ./internal/session/... ./internal/articulation/...

# E2E (heavier; needs env/CGO for some)
$env:CGO_CFLAGS = "-IC:/CodeProjects/codeNERD/sqlite_headers"
go test ./tests/e2e/ -run Prompt -count=1
```

## Package test inventory (by concern)

| File pattern | Focus |
|--------------|-------|
| `compiler_test.go`, `compiler_gaps_test.go`, `compiler_boundary_test.go` | Compile pipeline, edges |
| `compiler_kernel_atoms_test.go` | Kernel-injected atoms |
| `selector_test.go`, `selector_gaps_test.go` | Skeleton/flesh |
| `budget_test.go` | Fit, mandatory caps, modes |
| `resolver_test.go`, `resolver_gaps_test.go` | Order, cycles |
| `assembler_test.go`, `assembler_gaps_test.go` | Order, templates |
| `atoms_test.go`, `atoms_verification_test.go` | Matching, categories |
| `context_test.go`, `context_hash_test.go` | Hash/facts stability |
| `loader_test.go`, `loader_yaml_fields_test.go`, `loader_embedding.go` tests | YAML/SQL |
| `embedded_test.go` | go:embed corpus load |
| `config_factory_test.go`, `config_registry_test.go`, `config_generation_test.go` | ConfigAtoms |
| `prompt_gaps_test.go` | Cross-component gaps |
| `debugging_atoms_test.go`, `refactoring_atoms_test.go` | Corpus content presence |
| `specialist_benchmark_test.go`, `default_corpus_benchmark_test.go`, `loader_bench_test.go`, `minify_whitespace_perf_test.go` | Perf |
| `sync/synchronizer_test.go` | Agent sync |
| `query_expansion_test.go` | Expansion |
| `predicate_selector_test.go` | Predicate selection |

## E2E coverage (sample)

| Test package file | Asserts |
|-------------------|---------|
| `prompt_compiler_llm_integration_test.go` | Compile ↔ LLM |
| `promptcompiler_llmclient_integration_test.go` | Client boundary |
| `jit_kernel_context_cleanup_test.go` | Retract hygiene |
| `session_clean_loop_integration_test.go` | Full loop |
| `specialist_config_boundary_test.go` | consult/general config |
| `SessionExecutor_VirtualStore_Kernel_integration_test.go` | Executive chain |

## Coverage strengths

- Pipeline unit tests with mocks for kernel/vector.  
- Explicit gap tests for partial construction / nil budget manager paths.  
- Hash tests for cache stability.  
- Atom verification for critical embedded content (debugging/refactoring).  

## Coverage gaps

| Gap | Risk |
|-----|------|
| Full Mangle rule regression with real kernel + large corpus | Selection drift vs unit mocks |
| Cache key incompleteness scenarios | Stale prompt after tool_nudge flag |
| Dual ConfigAtom catalog parity | Tool allowlist drift |
| Evolved atom hot-reload under concurrent Compile | Race/stale |
| Vector timeout degradation golden tests | Flesh empty unexpectedly |

## Alignment to principles

| Principle | Test support |
|-----------|--------------|
| Skeleton critical | Selector tests with nil kernel |
| Budget absolute | Budget mandatory oversize tests |
| Atoms first | debugging/refactoring atom presence tests |
| Wiring | E2E session/compiler tests |

## Pre-handoff checklist for prompt changes

1. `go test ./internal/prompt/...`  
2. If Mangle rules touched (core defaults): run mangle check + JIT e2e cleanup test.  
3. If atoms YAML changed: consider regenerating `prompt_corpus.db`.  
4. If ConfigAtoms changed: grep tool registration for name match.  
5. If selector fact format changed: run kernel integration tests.
