# sqlpragmas — Failure Modes

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/sqlpragmas/` (complete internal coverage)
> **Implementation: `internal/sqlpragmas/` — 1 non-test .go, 2 tests, 0 .mg**


## Generic failure classes for `internal/sqlpragmas/`

| Mode | Symptoms | Mitigation |
|------|----------|------------|
| Missing wiring | Feature code exists but never runs | Grep registration / VirtualStore / CLI hooks |
| Kernel policy deny | Action blocked | Check `permitted` derivation and policy corpus |
| Mangle load failure | Boot dump `debug_program_ERROR.mg` | Decl, safety, stratification |
| LLM/client failure | Perception/articulation errors | Client factory, config engines |
| Store/IO failure | Persist errors | Context cancel, wrap errors, sqlite pragmas |
| Race/leak | Flaky tests, hung sessions | `-race`, goroutine lifecycle |

## Package-specific note

SQLite pragma helpers for safe DB open

Revisit this file after incidents; attach real log paths under `.nerd/logs/` when available.
