# cli — Architecture Corpus

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `cmd/nerd/` (113 non-test .go, 55 tests, 2 .mg)**


## Role

CLI entrypoints, chat TUI, campaign and system commands

## Source location

- Primary: `cmd/nerd/`
- Non-test Go files: **113**
- Test files: **55**
- Mangle sources: **2**
- Tier (dark-factory): **3**
- Estimated implementation completeness (heuristic): **70%**

## Documents

| Doc | Purpose |
|-----|---------|
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment |
| [01-DOMAIN-MODEL.md](01-DOMAIN-MODEL.md) | Types, facts, models |
| [02-CURRENT-STATE-CLI.md](02-CURRENT-STATE-CLI.md) | Honest living inventory |
| [03-GAP-ANALYSIS-CLI.md](03-GAP-ANALYSIS-CLI.md) | Gaps vs north star |
| [04-INVARIANTS-AND-GATES.md](04-INVARIANTS-AND-GATES.md) | Safety and verification gates |
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | Status + target surface |
| [TODO.md](TODO.md) | Open work |
| [_progress.md](_progress.md) | Generation progress |

## How to verify

```powershell
# package tests (when applicable)
go test ./cmd/nerd/...
```
