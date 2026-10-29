# store — Testing Alignment

> Last verified: **2026-07-13**  
> Package: `internal/store/`

## Commands

```powershell
# Full package
go test ./internal/store/...

# With race (where CGO allows)
go test -race ./internal/store/...

# Benchmarks (sample)
go test ./internal/store/ -bench=. -run=^$

# App binary path that exercises store at runtime
$env:CGO_CFLAGS = "-IC:/CodeProjects/codeNERD/sqlite_headers"
go build -o nerd.exe ./cmd/nerd
```

## Existing coverage map

| Area | Test files (representative) | Quality |
|------|----------------------------|---------|
| Cold / archival | `archival_test.go`, `cold_storage_integration_test.go`, `local_cold_extra_test.go` | Strong |
| Graph | `local_graph_test.go`, `*_extra_test.go`, `*_integration_test.go`, benchmarks | Strong |
| Session | `local_session_test.go`, integration, extra | Strong |
| Vector / semantic | `vector_store_test.go`, batch/search/brute/boundary/e2e/extra, benchmarks | Strong |
| Knowledge | `local_knowledge_*`, benchmarks | Good |
| Prompt | `local_prompt_extra_test.go` | Moderate |
| World | `local_world_extra_test.go` | Moderate |
| Verification | `local_verification_extra_test.go` | Moderate |
| Traces | `trace_store_test.go`, integration, extra | Strong |
| Learning | `learning_candidates_test.go`, reflection paths | Good |
| Tools | `tool_store_test.go`, cleanup tests | Strong |
| Migrations | `migrations_test.go`, benchmarks | Strong |
| Fact codec | `fact_codec_test.go` | Strong |
| Reflection | worker/search/reembed tests | Good |
| Reembed all | `reembed_all_test.go` | Good |
| Mocks | `mocks_test.go` embedding engines | Support |

## What tests prove

1. Cold lifecycle happy path, archive, purge, maintenance, concurrency suite.
2. Graph persistence, hydration, concurrent access.
3. Vector paths including fallbacks and batching.
4. Schema migrations apply cleanly.
5. Trace store write/read and integration with LocalStore wrapper.

## Gaps

| Gap | Risk | Suggestion |
|-----|------|------------|
| ANN drift reconcile under failure injection | Search quality silent miss | Test forced vec insert failure → detect/heal |
| Multi-DB reembed under partial failure | Ops | Expand `ReembedAllDBsForce` failure taxonomy tests |
| Prompt atom polymorphism selectors | JIT correctness | More selector JSON round-trip tests |
| World fingerprint conflict | Stale facts | Concurrency + fingerprint race tests |
| Cross-package wiring | Boot regressions | Rely on system/core integration tests outside this package |

## Alignment with principles

Tests should keep:

- No requirement that constitutional policy is enforced inside store.
- Explicit coverage of keyword vs semantic vs ANN ladders when engines are mocked.
- Temp dirs for DBs; never touch developer `.nerd/knowledge.db` in unit tests.

## CI notes

sqlite-vec behavior differs with build tags. CI should document whether `sqlite_vec` is enabled; tests that require ANN should skip or soft-assert when extension missing (existing patterns use detection).
