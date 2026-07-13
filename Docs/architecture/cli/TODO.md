# CLI — TODO

> Living backlog for `cmd/nerd`. Prefer gates over dates.

## P0

- [ ] Keep panic recovery on all async chat paths that can throw
- [ ] Never introduce policy bypass flags
- [ ] Ensure `--workspace` honored for any new persistence

## P1

- [ ] Table-driven tests for `processInput` slash vs NL routing matrix
- [ ] Boot failure injection tests for `session_boot` / shared boot
- [ ] Publish and maintain Cobra↔slash parity matrix (close or document gaps)
- [ ] Audit JIT-only domain shard paths after each major chat refactor

## P2

- [ ] Mechanical modularization of largest chat files (>800 LOC) with golden tests
- [ ] Structured boot spans instead of only printf logStep
- [ ] Help snapshot test for `rootCmd` command names

## P3

- [ ] UI performance pass on splitpane + diffview large payloads
- [ ] Align embedding status UX across cmd, slash, and config wizard

## Done (recent corpus work)

- [x] Replace thin auto inventory CLI corpus with deep-dive architecture set (2026-07-13)
