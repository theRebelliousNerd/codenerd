# shards — Architecture Corpus

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/shards/` (18 non-test .go, 24 tests, 1 .mg)**


## Role

Domain/system shard implementations and registration

## Source location

- Primary: `internal/shards/`
- Non-test Go files: **18**
- Test files: **24**
- Mangle sources: **1**
- Tier (dark-factory): **3**
- Estimated implementation completeness (heuristic): **90%**

## Documents

| Doc | Purpose |
|-----|---------|
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment |
| [01-DOMAIN-MODEL.md](01-DOMAIN-MODEL.md) | Types, facts, models |
| [02-CURRENT-STATE-SHARDS.md](02-CURRENT-STATE-SHARDS.md) | Honest living inventory |
| [03-GAP-ANALYSIS-SHARDS.md](03-GAP-ANALYSIS-SHARDS.md) | Gaps vs north star |
| [04-INVARIANTS-AND-GATES.md](04-INVARIANTS-AND-GATES.md) | Safety and verification gates |
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | Status + target surface |
| [TODO.md](TODO.md) | Open work |
| [_progress.md](_progress.md) | Generation progress |

## How to verify

```powershell
# package tests (when applicable)
go test ./internal/shards/...
```
