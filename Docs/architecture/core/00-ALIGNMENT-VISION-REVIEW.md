# core — Alignment & Vision Review

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/core/` (78 non-test .go, 107 tests, 129 .mg)**


## North-star fit

codeNERD separates **LLM creativity** from **Mangle executive control**. This package contributes:

**Mangle kernel, VirtualStore, Dreamer, fact store, shard manager plumbing**

| Dimension | Score (0–5) | Evidence |
|-----------|-------------|----------|
| Creative/executive split | 4 | Package role relative to fact-flow spine |
| Fact-flow placement | 5 | See 01-DOMAIN-MODEL |
| Constitutional safety | 5 | permitted / policy surfaces |
| JIT / atom discipline | 2 | prompt atoms vs ad-hoc prompts |
| Observability | 4 | logging / transparency hooks |
| Test grounding | 4 | 107 tests vs 78 sources |

## Overall

Living package under `internal/core/`. Corpus is **code-grounded**, not pre-implementation fiction.
