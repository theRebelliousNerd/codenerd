# core — Alignment & Vision Review

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/core/` (complete internal coverage)
> **Implementation: `internal/core/` — 78 non-test .go, 107 tests, 129 .mg**


## North-star fit

codeNERD: LLM creative center; Mangle kernel executive. Package role:

**Mangle kernel, VirtualStore, Dreamer, facts, API scheduler, shard manager plumbing**

| Dimension | Score (0–5) | Notes |
|-----------|-------------|-------|
| Creative/executive split | 5 | Relative to fact-flow spine |
| Fact-flow placement | 5 | See domain model |
| Constitutional safety | 5 | permitted / safety surfaces |
| JIT / atom discipline | 2 | prompt atoms |
| Observability | 3 | logs/metrics |
| Test grounding | 4 | 107 tests / 78 src |

## Verdict

Living package under `internal/core/`. Full corpus is **code-grounded**, not pre-implementation fiction.
