# tools — Invariants and Gates

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/tools/` (25 non-test .go, 21 tests, 0 .mg)**


## Invariants

1. Package code under `internal/tools/` remains the source of truth over this corpus.
2. Actions affecting the system must remain compatible with `permitted(...)` / default deny.
3. New Mangle predicates require `Decl` before use; negation safety and stratification hold.
4. LLM-facing behavior prefers JIT prompt atoms over ad-hoc monologue prompts.
5. Concurrent paths honor context cancellation and error wrapping (Go architect norms).

## Verification gates

| Gate | Command / check |
|------|-----------------|
| Unit/race | `go test -race ./internal/tools/...` |
| Build (sqlite-vec) | `$env:CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers"; go build -o nerd.exe ./cmd/nerd` |
| Path existence | Every cited path under `internal/` or `cmd/` must resolve |
| Surface registry | `python .agents/skills/corpus-build/scripts/verify_surfaces.py` when wiring claimed |

## Safety notes

- Dreamer / constitutional policy live primarily in core; consumers must not bypass gates.
- Do not delete passing tests to satisfy corpus status rows.
