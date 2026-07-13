# init — Invariants and Gates

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/init/` (complete internal coverage)
> **Implementation: `internal/init/` — 16 non-test .go, 7 tests, 1 .mg**


## Invariants

1. Source under `internal/init/` is authoritative over this corpus.
2. System actions remain compatible with `permitted(...)` / default deny.
3. New Mangle predicates require `Decl`; safe negation; stratification.
4. LLM-facing changes prefer prompt atoms (JIT) over ad-hoc prose.
5. Go: context-first I/O, wrapped errors, race-safe concurrency.

## Gates

| Gate | Check |
|------|-------|
| Tests | `go test ./internal/init/...` |
| Race (when concurrent) | `go test -race ./internal/init/...` |
| Binary (if CLI-impacting) | CGO sqlite-vec build of `./cmd/nerd` |
| Path existence | All cited `internal/` paths resolve |
| Surfaces | `validate_architecture_corpora.py` + optional `verify_surfaces.py` |
