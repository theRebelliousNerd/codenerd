# regression — Architecture Corpus

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/regression/` (complete internal coverage)
> **Implementation: `internal/regression/` — 1 non-test .go, 1 tests, 0 .mg**


## Role

Regression harness utilities

## Source location

- Primary: `internal/regression/` (**1:1 package root**)
- Non-test Go files: **1**
- Test files: **1**
- Mangle sources: **0**
- Tier: **2** (full foundation always; higher tier adds more cross-cuts)
- Heuristic implementation completeness: **90%**

## Full document set

| Doc | Purpose |
|-----|---------|
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment |
| [01-DOMAIN-MODEL.md](01-DOMAIN-MODEL.md) | Types, funcs, models |
| [02-CURRENT-STATE-REGRESSION.md](02-CURRENT-STATE-REGRESSION.md) | Living inventory |
| [03-GAP-ANALYSIS-REGRESSION.md](03-GAP-ANALYSIS-REGRESSION.md) | Gaps vs north star |
| [04-INVARIANTS-AND-GATES.md](04-INVARIANTS-AND-GATES.md) | Safety + verify gates |
| [05-CROSS-SYSTEM-WIRING.md](05-CROSS-SYSTEM-WIRING.md) | Integration surfaces |
| [06-TESTING-STRATEGY.md](06-TESTING-STRATEGY.md) | Test plan from inventory |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Import/dependency notes |
| [08-FAILURE-MODES.md](08-FAILURE-MODES.md) | Failure / risk surface |
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | Status + public surface |
| [TODO.md](TODO.md) | Open work |
| [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) | Open design questions |
| [_progress.md](_progress.md) | Generation progress |

## Verify

```powershell
go test ./internal/regression/...
```
