# session — Architecture Corpus

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/session/` (6 non-test .go, 14 tests, 0 .mg)**


## Role

Clean execution loop / session executor

## Source location

- Primary: `internal/session/`
- Non-test Go files: **6**
- Test files: **14**
- Mangle sources: **0**
- Tier (dark-factory): **2**
- Estimated implementation completeness (heuristic): **90%**

## Documents

| Doc | Purpose |
|-----|---------|
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment |
| [01-DOMAIN-MODEL.md](01-DOMAIN-MODEL.md) | Types, facts, models |
| [02-CURRENT-STATE-SESSION.md](02-CURRENT-STATE-SESSION.md) | Honest living inventory |
| [03-GAP-ANALYSIS-SESSION.md](03-GAP-ANALYSIS-SESSION.md) | Gaps vs north star |
| [04-INVARIANTS-AND-GATES.md](04-INVARIANTS-AND-GATES.md) | Safety and verification gates |
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | Status + target surface |
| [TODO.md](TODO.md) | Open work |
| [_progress.md](_progress.md) | Generation progress |

## How to verify

```powershell
# package tests (when applicable)
go test ./internal/session/...
```
