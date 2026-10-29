---
doc-class: governance
subsystem: diff
implementation-status: not-applicable
last-verified: 2026-09-21
verified-against: 231cfa7
supersedes: []
---

# 00-INDEX — internal/diff

Map of `Docs/architecture/diff/`. Start here after `README.md`. On any
disagreement about what the code does today, `IMPLEMENTED_SPEC.md` wins.
Gap work is ordered by `03-GAP-ANALYSIS.md` (GAP-DIFF-01..07), not by the
shorthand exits in `01-VISION.md` nor the queue order in `TODO.md`.

## Read order (one line per file)

1. `README.md` (governance, 55 lines) — entry paragraph: what this directory is; read first, then this index.
2. `00-INDEX.md` (governance, this file) — read-order map plus grounded-vs-hypothesized section; read second.
3. `01-VISION.md` (north-star, planned, 78 lines) — the finished behaviour the package builds toward and why it matters to `agents.md:43-46`; read for why, never as evidence of built behaviour.
4. `02-CURRENT-STATE.md` (shipped, 169 lines) — what is built and reachable today, file by file (`internal/diff/diff.go`, `internal/diff/cache.go`, `internal/session/turn_diff.go`, `cmd/nerd/ui/diffview.go`); read for cited current truth.
5. `IMPLEMENTED_SPEC.md` (shipped, 177 lines) — authoritative record of shipped behaviour; read as the tie-breaker on any conflict.
6. `INTERNALS.md` (deep-dive, 66 lines) — pipeline stages and cache design behind `func ComputeDiff` (`internal/diff/diff.go:243-307`); read for how the engine works.
7. `WIRING-AND-NOT-BUILT.md` (shipped, 106 lines) — what is wired and reachable versus what exists but nothing calls; read for what runs versus what only exists.
8. `03-GAP-ANALYSIS.md` (shipped-with-future, partial, 125 lines) — gap matrix GAP-DIFF-01..07 with machine-checkable exits; read as the R7 work source before any build decision.
9. `04-PRINCIPLES-AND-CONSTRAINTS.md` (governance, 109 lines) — numbered constraints P1-P7 any change must respect, each with code or ruling source; read before modifying the package.
10. `05-prompt-budget.md` (shipped-with-future, target-state, 279 lines) — capability spec for GAP-DIFF-04 bounded repair-prompt diffs, anchored at `func renderFileDiff` (`internal/session/turn_diff.go:55-76`); read when working the budget gap.
11. `06-CALLER-INTEGRATION.md` (shipped-with-future, partial, 369 lines) — how the engine attaches at its two seams, `diff.ComputeDiff` (`internal/session/turn_diff.go:62`) and `var uiDiffEngine` (`cmd/nerd/ui/diffview.go:907`); read when touching either caller.
12. `RISK-REGISTER-AND-DECISION-LOG.md` (governance, 79 lines) — risks R1-R7 plus R8a/R8b footguns with retirement tests, decisions D1-D11 with witnesses; read for what a change must not worsen.
13. `OPEN-QUESTIONS.md` (governance, 296 lines) — unresolved questions OQ-DIFF-01..07 plus tripwires T1-T9 a future author must preserve; read for what is still undecided.
14. `TODO.md` (governance, 183 lines) — leaf build queue TODO-DIFF-* tracing to GAP-DIFF-01..07, Phase 1 > 2 > 3; read last, when picking up work.

Note: `adr/ADR-NNN-<slug>.md` does not exist in this directory (verified by
listing 2026-09-21). ADR slugs proposed in prior research (engine choice,
bounded cost, binary short-circuit, cache design, trust-by-default, header
ownership, word spans, one-engine-per-site, turn-sees-own-edits, lifecycle,
prompt budget) are inputs to future `adr/` files, not evidence of files.

## Grounded vs hypothesized

Grounded (claims the code bears out today, each cited with repo-relative path
plus symbol and line read 2026-09-21): `02-CURRENT-STATE.md`,
`IMPLEMENTED_SPEC.md`, `INTERNALS.md`, `WIRING-AND-NOT-BUILT.md`, the
`04-PRINCIPLES-AND-CONSTRAINTS.md` constraints P1-P7, the
`RISK-REGISTER-AND-DECISION-LOG.md` decisions D1-D11, and the current-state
half (seams, tables, cited ranges) of `03-GAP-ANALYSIS.md`,
`05-prompt-budget.md`, and `06-CALLER-INTEGRATION.md`.

Hypothesized (plan-layer descriptions of what does not exist, allowed only
because their front-matter says `planned`, `target-state`, or `partial`):
`01-VISION.md` finished behaviour (5 items, cites no code by rule); the target
states and exit criteria GAP-DIFF-01..07 in `03-GAP-ANALYSIS.md`; the Target,
Predicates, Failure-modes-planned, and Proving-tests-planned sections of
`05-prompt-budget.md` (proposed `TurnDiffBudget`/`Outcome` and
`turn_diff_budget/3`, future tense throughout); the planned data flow,
failure modes FM-01..04, and proving tests PT-01..07 in
`06-CALLER-INTEGRATION.md`; the `TODO-DIFF-*` queue in `TODO.md`; the
OQ-DIFF-01..07 questions and T1-T9 tripwires in `OPEN-QUESTIONS.md`; and any
ADR slug named above or in `OPEN-QUESTIONS.md`/`RISK-REGISTER-AND-DECISION-LOG.md`
until its witness file is created under `adr/`.
