# browser — Failure Modes

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/browser/` (complete internal coverage)
> **Implementation: `internal/browser/` — 3 non-test .go, 6 tests, 0 .mg**


## Generic failure classes for `internal/browser/`

| Mode | Symptoms | Mitigation |
|------|----------|------------|
| Missing wiring | Feature code exists but never runs | Grep registration / VirtualStore / CLI hooks |
| Kernel policy deny | Action blocked | Check `permitted` derivation and policy corpus |
| Mangle load failure | Boot dump `debug_program_ERROR.mg` | Decl, safety, stratification |
| LLM/client failure | Perception/articulation errors | Client factory, config engines |
| Store/IO failure | Persist errors | Context cancel, wrap errors, sqlite pragmas |
| Race/leak | Flaky tests, hung sessions | `-race`, goroutine lifecycle |

## Package-specific note

Browser automation / Rod session management and honeypot surfaces

Revisit this file after incidents; attach real log paths under `.nerd/logs/` when available.
