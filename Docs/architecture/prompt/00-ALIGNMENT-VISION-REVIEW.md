# prompt — Alignment & Vision Review

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/prompt/` (25 non-test .go, 32 tests, 0 .mg)**


## North-star fit

codeNERD separates **LLM creativity** from **Mangle executive control**. This package contributes:

**JIT prompt compiler, atoms, selector, budget**

| Dimension | Score (0–5) | Evidence |
|-----------|-------------|----------|
| Creative/executive split | 4 | Package role relative to fact-flow spine |
| Fact-flow placement | 3 | See 01-DOMAIN-MODEL |
| Constitutional safety | 3 | permitted / policy surfaces |
| JIT / atom discipline | 5 | prompt atoms vs ad-hoc prompts |
| Observability | 3 | logging / transparency hooks |
| Test grounding | 4 | 32 tests vs 25 sources |

## Overall

Living package under `internal/prompt/`. Corpus is **code-grounded**, not pre-implementation fiction.
