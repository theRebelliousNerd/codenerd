# mangle — Alignment & Vision Review

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/mangle/` (complete internal coverage)
> **Implementation: `internal/mangle/` — 21 non-test .go, 39 tests, 1 .mg**


## North-star fit

codeNERD: LLM creative center; Mangle kernel executive. Package role:

**Mangle engine bindings, differential evaluation, generation feedback**

| Dimension | Score (0–5) | Notes |
|-----------|-------------|-------|
| Creative/executive split | 5 | Relative to fact-flow spine |
| Fact-flow placement | 5 | See domain model |
| Constitutional safety | 5 | permitted / safety surfaces |
| JIT / atom discipline | 2 | prompt atoms |
| Observability | 3 | logs/metrics |
| Test grounding | 4 | 39 tests / 21 src |

## Verdict

Living package under `internal/mangle/`. Full corpus is **code-grounded**, not pre-implementation fiction.
