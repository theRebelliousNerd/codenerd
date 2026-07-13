# config — Architecture Corpus

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/config/` (17 non-test .go, 4 tests, 0 .mg)**


## Role

Config loading, engines, limits, user config

## Source location

- Primary: `internal/config/`
- Non-test Go files: **17**
- Test files: **4**
- Mangle sources: **0**
- Tier (dark-factory): **2**
- Estimated implementation completeness (heuristic): **70%**

## Documents

| Doc | Purpose |
|-----|---------|
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment |
| [01-DOMAIN-MODEL.md](01-DOMAIN-MODEL.md) | Types, facts, models |
| [02-CURRENT-STATE-CONFIG.md](02-CURRENT-STATE-CONFIG.md) | Honest living inventory |
| [03-GAP-ANALYSIS-CONFIG.md](03-GAP-ANALYSIS-CONFIG.md) | Gaps vs north star |
| [04-INVARIANTS-AND-GATES.md](04-INVARIANTS-AND-GATES.md) | Safety and verification gates |
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | Status + target surface |
| [TODO.md](TODO.md) | Open work |
| [_progress.md](_progress.md) | Generation progress |

## How to verify

```powershell
# package tests (when applicable)
go test ./internal/config/...
```
