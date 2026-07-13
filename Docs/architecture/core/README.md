# core — Architecture Corpus

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/core/` (78 non-test .go, 107 tests, 129 .mg)**


## Role

Mangle kernel, VirtualStore, Dreamer, fact store, shard manager plumbing

## Source location

- Primary: `internal/core/`
- Non-test Go files: **78**
- Test files: **107**
- Mangle sources: **129**
- Tier (dark-factory): **3**
- Estimated implementation completeness (heuristic): **88%**

## Documents

| Doc | Purpose |
|-----|---------|
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment |
| [01-DOMAIN-MODEL.md](01-DOMAIN-MODEL.md) | Types, facts, models |
| [02-CURRENT-STATE-CORE.md](02-CURRENT-STATE-CORE.md) | Honest living inventory |
| [03-GAP-ANALYSIS-CORE.md](03-GAP-ANALYSIS-CORE.md) | Gaps vs north star |
| [04-INVARIANTS-AND-GATES.md](04-INVARIANTS-AND-GATES.md) | Safety and verification gates |
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | Status + target surface |
| [TODO.md](TODO.md) | Open work |
| [_progress.md](_progress.md) | Generation progress |

## How to verify

```powershell
# package tests (when applicable)
go test ./internal/core/...
```
