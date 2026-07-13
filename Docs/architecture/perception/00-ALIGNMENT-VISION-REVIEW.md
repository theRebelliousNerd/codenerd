# perception — Alignment & Vision Review

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/perception/` (49 non-test .go, 47 tests, 0 .mg)**


## North-star fit

codeNERD separates **LLM creativity** from **Mangle executive control**. This package contributes:

**NL→atoms transduction, semantic classifier, LLM clients**

| Dimension | Score (0–5) | Evidence |
|-----------|-------------|----------|
| Creative/executive split | 4 | Package role relative to fact-flow spine |
| Fact-flow placement | 5 | See 01-DOMAIN-MODEL |
| Constitutional safety | 3 | permitted / policy surfaces |
| JIT / atom discipline | 2 | prompt atoms vs ad-hoc prompts |
| Observability | 3 | logging / transparency hooks |
| Test grounding | 4 | 47 tests vs 49 sources |

## Overall

Living package under `internal/perception/`. Corpus is **code-grounded**, not pre-implementation fiction.
