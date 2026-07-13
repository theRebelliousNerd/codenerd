# prompt — Testing Alignment

> Last verified: **2026-07-13**

## Commands

```powershell
# Package, sync, and validator routes
go test -count=1 -timeout=240s ./internal/prompt ./internal/prompt/sync ./cmd/tools/validate_prompt_atoms

# Checked-in atom contract: warnings fail the gate
go run ./cmd/tools/validate_prompt_atoms -root internal/prompt/atoms -fail-on-warn

# Production adapter cache/isolation race gate
go test -race -count=1 -timeout=240s ./internal/system -run 'TestKernelAdapter_(CompilationScopesIsolateConcurrentPrompts|CompilationScopeDoesNotLeakOnBudgetError|CompilationScopeDoesNotLeakOnCancellation|RetryContextBypassesPreRetryCache)$'

# Consumer packages
go test ./internal/session/... ./internal/articulation/...

# E2E (heavier; needs env/CGO for some)
$env:CGO_CFLAGS = "-IC:/CodeProjects/codeNERD/sqlite_headers"
go test ./tests/e2e/ -run Prompt -count=1
```

## Package test inventory (by concern)

| File pattern | Focus |
|--------------|-------|
| `compiler_test.go`, `compiler_gaps_test.go`, `compiler_boundary_test.go`, `compiler_scope_test.go` | Compile pipeline, edges, panic-safe scope close |
| `compiler_kernel_atoms_test.go` | Kernel-injected atoms |
| `selector_test.go`, `selector_gaps_test.go` | Skeleton/flesh |
| `budget_test.go` | Fit, mandatory caps, modes |
| `resolver_test.go`, `resolver_gaps_test.go` | Order, cycles |
| `assembler_test.go`, `assembler_gaps_test.go` | Order, templates |
| `atoms_test.go`, `atoms_verification_test.go`, `atom_schema_test.go` | Matching, categories, strict schema and migrations |
| `context_test.go`, `context_hash_test.go` | Hash/facts stability |
| `loader_test.go`, `loader_yaml_fields_test.go`, `loader_embedding.go` tests | YAML/SQL |
| `embedded_test.go` | go:embed corpus load |
| `config_factory_test.go`, `config_registry_test.go`, `config_generation_test.go` | ConfigAtoms |
| `prompt_gaps_test.go` | Cross-component gaps |
| `debugging_atoms_test.go`, `refactoring_atoms_test.go` | Corpus content presence |
| `specialist_benchmark_test.go`, `default_corpus_benchmark_test.go`, `loader_bench_test.go`, `minify_whitespace_perf_test.go` | Perf |
| `sync/synchronizer_test.go` | Agent sync, prune, transactional invalid-document failure |
| `cmd/tools/validate_prompt_atoms/corpus_parity_test.go` | Validator/filesystem/embedded ordered 888-ID parity |
| `internal/system/prompt_kernel_scope_test.go` | Production cloned-kernel isolation, cleanup, retry cache identity |
| `query_expansion_test.go` | Expansion |
| `predicate_selector_test.go` | Predicate selection |

## E2E coverage (sample)

| Test package file | Asserts |
|-------------------|---------|
| `prompt_compiler_llm_integration_test.go` | Compile ↔ LLM |
| `promptcompiler_llmclient_integration_test.go` | Client boundary |
| `jit_kernel_context_cleanup_test.go` | Legacy retract hygiene |
| `internal/system/prompt_kernel_scope_test.go` | Production `KernelAdapter` scope isolation and cleanup |
| `session_clean_loop_integration_test.go` | Full loop |
| `specialist_config_boundary_test.go` | consult/general config |
| `SessionExecutor_VirtualStore_Kernel_integration_test.go` | Executive chain |

## Coverage strengths

- The package currently exposes 231 listed tests across 34 root test files, plus
  four listed `sync` tests.
- Pipeline unit tests with mocks for kernel/vector.  
- Explicit gap tests for partial construction / nil budget manager paths.  
- Hash tests cover retry/tools, budgets/search, world fields, set canonicalization,
  and caller non-mutation.
- Atom verification for critical embedded content (debugging/refactoring).  
- Strict parser tests reject unknown fields and whole invalid sequences; parity
  tests pin all 888 ordered atom IDs.
- Focused production adapter tests pass under `-race` for concurrent mixed
  contexts, budget failure, cancellation, and retry cache separation.

## Coverage gaps

| Gap | Risk |
|-----|------|
| Full Mangle rule regression with real kernel + large corpus | Selection drift vs unit mocks |
| External adapters without `KernelScopeProvider` | Compatibility compiles may share facts unless adapter retraction/serialization is guaranteed |
| Dual ConfigAtom catalog parity | Tool allowlist drift |
| Evolved atom hot-reload under concurrent Compile | Race/stale |
| Vector timeout degradation golden tests | Flesh empty unexpectedly |
| Any `Fuzz*` entry point | Parser, loader, resolver, and context boundaries lack mutation pressure |

`internal/system/prompt_kernel_scope_test.go` proves the production adapter seam
with a real kernel clone. The older E2E retraction test remains useful only for
compatibility adapters. The full prompt/sync race gate is green after the shared
test kernel was made concurrency-safe.

## Alignment to principles

| Principle | Test support |
|-----------|--------------|
| Skeleton critical | Selector tests with nil kernel |
| Budget absolute | Budget mandatory oversize tests |
| Atoms first | debugging/refactoring atom presence tests |
| Wiring | E2E session/compiler tests |

## Pre-handoff checklist for prompt changes

1. `go test -count=1 ./internal/prompt ./internal/prompt/sync ./cmd/tools/validate_prompt_atoms`
2. Run `go run ./cmd/tools/validate_prompt_atoms -root internal/prompt/atoms -fail-on-warn`
   and require a clean 888-atom result before treating the contract as release-ready.
3. If Mangle rules touched (core defaults): run mangle check + JIT E2E cleanup
   test, and include the production adapter when changing fact lifecycle.
4. If atoms YAML changed: consider regenerating `prompt_corpus.db`.
5. If ConfigAtoms changed: grep tool registration for name match.
6. If selector fact format changed: run kernel integration tests.
