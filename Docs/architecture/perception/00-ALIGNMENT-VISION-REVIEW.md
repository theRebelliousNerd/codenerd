# perception — Alignment & Vision Review

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/perception/` (complete internal coverage)
> **Implementation: `internal/perception/` — 50 non-test .go, 48 tests, 0 .mg**


## North-star fit

codeNERD: LLM creative center; Mangle kernel executive. Package role:

**NL→atoms transduction, semantic classification, LLM clients**

| Dimension | Score (0–5) | Notes |
|-----------|-------------|-------|
| Creative/executive split | 5 | Relative to fact-flow spine |
| Fact-flow placement | 5 | See domain model |
| Constitutional safety | 3 | permitted / safety surfaces |
| JIT / atom discipline | 2 | prompt atoms |
| Observability | 3 | logs/metrics |
| Test grounding | 4 | 48 tests / 50 src |

## Verdict

Living package under `internal/perception/`. Full corpus is **code-grounded**, not pre-implementation fiction.
