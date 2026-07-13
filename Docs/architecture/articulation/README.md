# articulation — Architecture Corpus

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/articulation/` (8 non-test .go, 7 tests, 0 .mg)**


## Role

Atoms→NL, Piggyback emitter, prompt assembly bridge

## Source location

- Primary: `internal/articulation/`
- Non-test Go files: **8**
- Test files: **7**
- Mangle sources: **0**
- Tier (dark-factory): **2**
- Estimated implementation completeness (heuristic): **85%**

## Documents

| Doc | Purpose |
|-----|---------|
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment |
| [01-DOMAIN-MODEL.md](01-DOMAIN-MODEL.md) | Types, facts, models |
| [02-CURRENT-STATE-ARTICULATION.md](02-CURRENT-STATE-ARTICULATION.md) | Honest living inventory |
| [03-GAP-ANALYSIS-ARTICULATION.md](03-GAP-ANALYSIS-ARTICULATION.md) | Gaps vs north star |
| [04-INVARIANTS-AND-GATES.md](04-INVARIANTS-AND-GATES.md) | Safety and verification gates |
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | Status + target surface |
| [TODO.md](TODO.md) | Open work |
| [_progress.md](_progress.md) | Generation progress |

## How to verify

```powershell
# package tests (when applicable)
go test ./internal/articulation/...
```
