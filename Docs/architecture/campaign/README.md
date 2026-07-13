# campaign — Architecture Corpus

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/campaign/` (44 non-test .go, 29 tests, 1 .mg)**


## Role

Multi-phase goal orchestration and context paging

## Source location

- Primary: `internal/campaign/`
- Non-test Go files: **44**
- Test files: **29**
- Mangle sources: **1**
- Tier (dark-factory): **2**
- Estimated implementation completeness (heuristic): **85%**

## Documents

| Doc | Purpose |
|-----|---------|
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment |
| [01-DOMAIN-MODEL.md](01-DOMAIN-MODEL.md) | Types, facts, models |
| [02-CURRENT-STATE-CAMPAIGN.md](02-CURRENT-STATE-CAMPAIGN.md) | Honest living inventory |
| [03-GAP-ANALYSIS-CAMPAIGN.md](03-GAP-ANALYSIS-CAMPAIGN.md) | Gaps vs north star |
| [04-INVARIANTS-AND-GATES.md](04-INVARIANTS-AND-GATES.md) | Safety and verification gates |
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | Status + target surface |
| [TODO.md](TODO.md) | Open work |
| [_progress.md](_progress.md) | Generation progress |

## How to verify

```powershell
# package tests (when applicable)
go test ./internal/campaign/...
```
