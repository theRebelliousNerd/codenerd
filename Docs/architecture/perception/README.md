# perception — Architecture Corpus

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/perception/` (49 non-test .go, 47 tests, 0 .mg)**


## Role

NL→atoms transduction, semantic classifier, LLM clients

## Source location

- Primary: `internal/perception/`
- Non-test Go files: **49**
- Test files: **47**
- Mangle sources: **0**
- Tier (dark-factory): **3**
- Estimated implementation completeness (heuristic): **85%**

## Documents

| Doc | Purpose |
|-----|---------|
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment |
| [01-DOMAIN-MODEL.md](01-DOMAIN-MODEL.md) | Types, facts, models |
| [02-CURRENT-STATE-PERCEPTION.md](02-CURRENT-STATE-PERCEPTION.md) | Honest living inventory |
| [03-GAP-ANALYSIS-PERCEPTION.md](03-GAP-ANALYSIS-PERCEPTION.md) | Gaps vs north star |
| [04-INVARIANTS-AND-GATES.md](04-INVARIANTS-AND-GATES.md) | Safety and verification gates |
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | Status + target surface |
| [TODO.md](TODO.md) | Open work |
| [_progress.md](_progress.md) | Generation progress |

## How to verify

```powershell
# package tests (when applicable)
go test ./internal/perception/...
```
