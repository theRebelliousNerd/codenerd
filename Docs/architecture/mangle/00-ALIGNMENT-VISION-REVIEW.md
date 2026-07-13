# mangle — Alignment & Vision Review

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/mangle/` (21 non-test .go, 39 tests, 1 .mg)**


## North-star fit

codeNERD separates **LLM creativity** from **Mangle executive control**. This package contributes:

**Mangle engine bindings, differential evaluation, feedback loops**

| Dimension | Score (0–5) | Evidence |
|-----------|-------------|----------|
| Creative/executive split | 4 | Package role relative to fact-flow spine |
| Fact-flow placement | 3 | See 01-DOMAIN-MODEL |
| Constitutional safety | 5 | permitted / policy surfaces |
| JIT / atom discipline | 2 | prompt atoms vs ad-hoc prompts |
| Observability | 3 | logging / transparency hooks |
| Test grounding | 4 | 39 tests vs 21 sources |

## Overall

Living package under `internal/mangle/`. Corpus is **code-grounded**, not pre-implementation fiction.
