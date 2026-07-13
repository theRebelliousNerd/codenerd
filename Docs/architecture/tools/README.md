# tools — Architecture Corpus

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/tools/` (25 non-test .go, 21 tests, 0 .mg)**


## Role

Tool registry and research/tool integrations

## Source location

- Primary: `internal/tools/`
- Non-test Go files: **25**
- Test files: **21**
- Mangle sources: **0**
- Tier (dark-factory): **2**
- Estimated implementation completeness (heuristic): **85%**

## Documents

| Doc | Purpose |
|-----|---------|
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment |
| [01-DOMAIN-MODEL.md](01-DOMAIN-MODEL.md) | Types, facts, models |
| [02-CURRENT-STATE-TOOLS.md](02-CURRENT-STATE-TOOLS.md) | Honest living inventory |
| [03-GAP-ANALYSIS-TOOLS.md](03-GAP-ANALYSIS-TOOLS.md) | Gaps vs north star |
| [04-INVARIANTS-AND-GATES.md](04-INVARIANTS-AND-GATES.md) | Safety and verification gates |
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | Status + target surface |
| [TODO.md](TODO.md) | Open work |
| [_progress.md](_progress.md) | Generation progress |

## How to verify

```powershell
# package tests (when applicable)
go test ./internal/tools/...
```
