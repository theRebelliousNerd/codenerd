# prompt — Architecture Corpus

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/prompt/` (25 non-test .go, 32 tests, 0 .mg)**


## Role

JIT prompt compiler, atoms, selector, budget

## Source location

- Primary: `internal/prompt/`
- Non-test Go files: **25**
- Test files: **32**
- Mangle sources: **0**
- Tier (dark-factory): **3**
- Estimated implementation completeness (heuristic): **90%**

## Documents

| Doc | Purpose |
|-----|---------|
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment |
| [01-DOMAIN-MODEL.md](01-DOMAIN-MODEL.md) | Types, facts, models |
| [02-CURRENT-STATE-PROMPT.md](02-CURRENT-STATE-PROMPT.md) | Honest living inventory |
| [03-GAP-ANALYSIS-PROMPT.md](03-GAP-ANALYSIS-PROMPT.md) | Gaps vs north star |
| [04-INVARIANTS-AND-GATES.md](04-INVARIANTS-AND-GATES.md) | Safety and verification gates |
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | Status + target surface |
| [TODO.md](TODO.md) | Open work |
| [_progress.md](_progress.md) | Generation progress |

## How to verify

```powershell
# package tests (when applicable)
go test ./internal/prompt/...
```
