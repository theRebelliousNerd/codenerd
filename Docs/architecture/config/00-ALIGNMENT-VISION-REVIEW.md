# config — Alignment & Vision Review

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/config/` (complete internal coverage)
> **Implementation: `internal/config/` — 17 non-test .go, 5 tests, 0 .mg**


## North-star fit

codeNERD: LLM creative center; Mangle kernel executive. Package role:

**Configuration loading, engines, limits, user and memory config**

| Dimension | Score (0–5) | Notes |
|-----------|-------------|-------|
| Creative/executive split | 3 | Relative to fact-flow spine |
| Fact-flow placement | 3 | See domain model |
| Constitutional safety | 3 | permitted / safety surfaces |
| JIT / atom discipline | 2 | prompt atoms |
| Observability | 3 | logs/metrics |
| Test grounding | 2 | 5 tests / 17 src |

## Verdict

Living package under `internal/config/`. Full corpus is **code-grounded**, not pre-implementation fiction.
