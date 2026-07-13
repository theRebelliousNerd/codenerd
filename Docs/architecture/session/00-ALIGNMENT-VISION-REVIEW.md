# session — Alignment & Vision Review

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/session/` (complete internal coverage)
> **Implementation: `internal/session/` — 6 non-test .go, 14 tests, 0 .mg**


## North-star fit

codeNERD: LLM creative center; Mangle kernel executive. Package role:

**Session execution loop and clean executor**

| Dimension | Score (0–5) | Notes |
|-----------|-------------|-------|
| Creative/executive split | 3 | Relative to fact-flow spine |
| Fact-flow placement | 5 | See domain model |
| Constitutional safety | 3 | permitted / safety surfaces |
| JIT / atom discipline | 2 | prompt atoms |
| Observability | 3 | logs/metrics |
| Test grounding | 4 | 14 tests / 6 src |

## Verdict

Living package under `internal/session/`. Full corpus is **code-grounded**, not pre-implementation fiction.
