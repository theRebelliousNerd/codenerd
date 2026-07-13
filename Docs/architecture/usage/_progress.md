# usage — Corpus Progress

## 2026-07-13 — Full rebuild (SUBAGENT_INSTRUCTIONS)

- Re-read entire `internal/usage/` (2 src + 4 tests).
- Reverse-dep scan of `codenerd/internal/usage` across `cmd/` and `internal/`.
- Traced boot (`system/factory.go`), ZAI Track, shard `WithShardContext`, chat/CLI `NewContext`, UI page.
- Replaced thin auto-inventory stubs with **full new-named doc set** per rebuild contract:
  - README, IMPLEMENTED_SPEC
  - 00–12 series (vision, state, gaps, principles, architecture, API, deps, wiring, safety, tests, observability, failures)
  - TODO, OPEN-QUESTIONS, _progress
- No Go/Mangle/code changes.
- Flagship IMPLEMENTED_SPEC expanded with control-flow narrative, integration map, honesty about Events/Cost/ZAI-only producers.

## Status

| Item | State |
|------|-------|
| Research | Done |
| Full doc set | Done |
| Code changes | None (docs only) |
