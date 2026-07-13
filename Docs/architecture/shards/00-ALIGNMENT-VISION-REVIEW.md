# shards — Alignment & Vision Review

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/shards/` (complete internal coverage)
> **Implementation: `internal/shards/` — 18 non-test .go, 24 tests, 1 .mg**


## North-star fit

codeNERD: LLM creative center; Mangle kernel executive. Package role:

**Domain and system shard implementations + registration**

| Dimension | Score (0–5) | Notes |
|-----------|-------------|-------|
| Creative/executive split | 3 | Relative to fact-flow spine |
| Fact-flow placement | 5 | See domain model |
| Constitutional safety | 3 | permitted / safety surfaces |
| JIT / atom discipline | 2 | prompt atoms |
| Observability | 3 | logs/metrics |
| Test grounding | 4 | 24 tests / 18 src |

## Verdict

Living package under `internal/shards/`. Full corpus is **code-grounded**, not pre-implementation fiction.
