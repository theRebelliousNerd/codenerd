# articulation — Alignment & Vision Review

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/articulation/` (8 non-test .go, 7 tests, 0 .mg)**


## North-star fit

codeNERD separates **LLM creativity** from **Mangle executive control**. This package contributes:

**Atoms→NL, Piggyback emitter, prompt assembly bridge**

| Dimension | Score (0–5) | Evidence |
|-----------|-------------|----------|
| Creative/executive split | 4 | Package role relative to fact-flow spine |
| Fact-flow placement | 3 | See 01-DOMAIN-MODEL |
| Constitutional safety | 3 | permitted / policy surfaces |
| JIT / atom discipline | 4 | prompt atoms vs ad-hoc prompts |
| Observability | 3 | logging / transparency hooks |
| Test grounding | 4 | 7 tests vs 8 sources |

## Overall

Living package under `internal/articulation/`. Corpus is **code-grounded**, not pre-implementation fiction.
