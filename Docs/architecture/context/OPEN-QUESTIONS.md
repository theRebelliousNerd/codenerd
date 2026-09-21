---
doc-class: governance
subsystem: context
implementation-status: not-applicable
last-verified: 2026-09-21
verified-against: 456e521
supersedes: []
---

# Open questions and tripwire invariants — `internal/context`

> This file answers the governance question: what is still undecided, and what
> must a future author preserve while deciding it. It makes no `shipped` claim
> of its own; every code cell is cited like one (repo-relative path plus symbol
> plus line, verified against the working tree via `02-CURRENT-STATE.md:1-496`,
> `IMPLEMENTED_SPEC.md:1-526`, and `WIRING-AND-NOT-BUILT.md:1-101`, all
> re-verified 2026-09-21 against `456e521`). Target-state cells point at the
> spec that details them (`05-RELEVANCE-AND-RETRIEVAL.md:1-204`,
> `06-RETENTION-EVICTION-ORDERING.md:1-317`). Gap IDs are owned by
> `03-GAP-ANALYSIS.md:33-39` (`GAP-CTX-01..07` as defined in `05:187-195` and
> `06:294-305`); the `01-VISION.md:237-244` numbering is superseded and is not
> cited as a queue. Principles are owned by
> `04-PRINCIPLES-AND-CONSTRAINTS.md:1-240` (`P1-P12`). For what runs today see
> `02-CURRENT-STATE.md` and `IMPLEMENTED_SPEC.md`. For the dream see
> `01-VISION.md`, `05-RELEVANCE-AND-RETRIEVAL.md`, and
> `06-RETENTION-EVICTION-ORDERING.md`. **On any disagreement about shipped
> behaviour, `IMPLEMENTED_SPEC.md:15-19` wins.**

## 0. How to read this file

- §1 lists **open questions** (`OQ-CTX-NN`): design decisions with no owner or
  no evidence yet. Each names the seam, the options, what evidence would close
  it, and which gap exit it gates.
- §2 lists **tripwire invariants** (`I-01..I-12`, from `04` `P1-P12`): standing
  constraints a future change must preserve. Each names the code witness and
  the proving-test witness that fails if the invariant is broken.
- §3 lists **re-read obligations**: `[upstream]` / `[seam]` claims that must be
  re-verified from code before any shipped document cites them.

## 1. Open questions

### OQ-CTX-01 — Who owns the live turn path: `ProcessTurn` or the working loop?

- Seam: `ProcessTurn` (`internal/context/compressor_turns.go:30`) is
  `exists-but-uncalled` — 16-row caller index, all `*_test.go`, zero
  production callers (`WIRING-AND-NOT-BUILT.md:45-49`, `03-GAP-ANALYSIS.md:53-92`).
  `GetContextString` (`internal/context/compressor.go:774`) is test-only
  (`WIRING:50-52`). `Select` (`internal/context/working_set.go:308`) has no
  traced production driver — same-name `browser`/`prompt` hits are false
  positives, session-loop driver untraced (`WIRING:53-58`). `Continue`
  (`internal/context/working_set.go:175`), `TranscriptRounds` (`:254`),
  `SectionCeiling` (`:271`), `RepeatThreshold` (`:291`) are dormant
  (`WIRING:59-64`).
- Options: (a) the working loop (`beginWorkingLoop` → `WorkingSet` + recall
  tool hypothesis) is the live path and `ProcessTurn` is legacy; (b)
  `ProcessTurn` is the intended path and needs wiring.
- Closes when: a production-driver trace names the caller of `Select` or
  `ProcessTurn` outside `*_test.go`.
- Gates: crediting any `GAP-CTX-02` / `GAP-CTX-04` / `GAP-CTX-05` exit
  (`T-RET-01`, `T-RTN-01`, `T-EVC-01/02`). Do not mark those gaps done until
  this trace exists.

### OQ-CTX-02 — Where does `working_set.mg` load, and in what scope?

- Seam: `internal/context/working_set.mg` EXISTS (182 lines), contradicting the
  pre-rewrite claim that the package held no `.mg` file
  (`WIRING:68-73`, `03:73-79`). Whether the engine loads it in compilation
  scope is unverified. `working_selected` / `should_include_context` rules are
  decided where it loads (`WIRING:98-100`).
- Options: (a) it loads in kernel compilation scope and its rules gate
  `Select`; (b) it is inert data and the Go-side queries decide alone.
- Closes when: the load site is cited (path + symbol + line) and the
  duplicate-`Decl` boot check passes (project rule: one `Decl` per arity).
- Gates: `GAP-CTX-01` / `GAP-CTX-02` derivation claims (`T-REL-01..03`,
  `T-RET-01`).

### OQ-CTX-03 — `note` persists and recalls, or the schema narrows?

- Seam: `processMemoryOperation`
  (`internal/context/compressor_turns.go:202`) accepts `note` by schema but
  Warn-drops it (`:226-231`); unknown ops Warn-drop (`:232-235`)
  (`IMPLEMENTED_SPEC.md:241-248`).
- Options: (a) `note` persists through the store and recalls (round-trip);
  (b) packet validation rejects `note` and the schema narrows to
  `promote_to_long_term` / `forget` / `store_vector`.
- Closes when: round-trip test passes OR boundary rejection lands
  (`06:304-305`; mirrors `01-VISION.md:244`).
- Gates: `GAP-CTX-07` (parallel leaf, no other deps).

### OQ-CTX-04 — Do kernel facts bypass the budget, by design or by accident?

- Seam: `buildKernelDerivedContext`
  (`internal/context/compressor_metrics.go:548`) "never filters or reorders"
  kernel facts (`:549-551`, `WIRING:80-83`). `Select`
  (`internal/context/working_set.go:308`) budgets `Candidates(256)` at `:373`
  and per-fact `charBudget/8` at `:471`, but kernel-derived context built at
  `:473` is not re-budgeted on that path.
- Options: (a) bypass is intended (kernel facts are authoritative); (b) bypass
  is a hole and kernel facts must pass the same `charBudget` gate.
- Closes when: `T-RET-01` funnel contract (`05:181`) pins whether gated
  entities stay within `charBudget`.
- Gates: `GAP-CTX-02`.

### OQ-CTX-05 — Is best-effort persistence acceptable for memory ops?

- Seam: `ProcessTurn` persist path discards errors
  (`internal/context/compressor_turns.go:168-181`, `WIRING:84-86`);
  `promote_to_long_term` / `store_vector` failures Warn without persisting
  (`:204-225`).
- Options: (a) accept and document best-effort; (b) propagate errors or retry
  so `GAP-CTX-07` round-trip is honest.
- Closes when: `GAP-CTX-07` round-trip-or-narrow witness runs green.
- Gates: `GAP-CTX-07`.

### OQ-CTX-06 — What are the true nine-scorer weights?

- Seam: `computeScore` (`internal/context/activation_scoring.go:39`) sums nine
  components (`:17-27`); inner weights (`:59-559`: recency `<1m+50/<5m+30/<30m+10`,
  relevance verb×predicate table, dependency 30%/cap-40, campaign cap-60,
  issue cap-100 + tiers, feedback ×20, backref cap-70) are carried as
  `[upstream: task_aab9612b_1_0]` per `05:57-60`, `02:87-109` — ranges
  element-verified, weights NOT re-read (`00-INDEX.md:37`).
- Closes when: `internal/context/activation_scoring.go:59-559` is body-read
  and each weight is cited to its line. Highest-priority remaining read
  (task_aab9612b_2_0 §9).
- Gates: `GAP-CTX-01` spec precision (`T-REL-01/02` fixtures).

### OQ-CTX-07 — Which eviction numbers are load-bearing?

- Seam: `compressor_turns.go` / `compressor_metrics.go` inner lines finer than
  function range — cutoff (`internal/context/compressor_turns.go:305`),
  `collectKeyAtoms(64)` (`:309`), age assert (`:318`), ratio (`:336-354`),
  merge 64-atom cap (`internal/context/compressor_turns.go:641`),
  mask queries (`internal/context/compressor_metrics.go:729` + `:739-751`),
  refuse-on-drift (`:762-766`) — carried as `[seam]` per
  `IMPLEMENTED_SPEC.md:38-44`; `05:93-95` + `06:§2` mark them `[upstream]`
  until re-read (`00-INDEX.md:38`).
- Closes when: each line above is body-read and cited.
- Gates: `GAP-CTX-05` / `GAP-CTX-06` exits (`T-EVC-01/02/03`).

- **OQ-CTX-08 (citations to deleted pre-rewrite drafts).** Two files predating
  the rewrite were deleted 2026-09-21 per standard Rule 3 (nothing may point at
  a removed file). Their directory pointers were removed at the same time, and
  every true claim they held that the code bears out is cited where it is now
  verified (see the `09`/`INTERNALS` rows above and `IMPLEMENTED_SPEC.md`
  Correction 1, §9, and §10). There is no remaining citation debt: no document
  in this directory points at a file that no longer exists.

## 2. Tripwire invariants (preserve these; each has a witness that fails if broken)

- I-01 Kernel decides, Go computes (P1): `BuildContext`
  (`internal/context/compressor.go:688`, `:695`) + `Select`
  (`internal/context/working_set.go:433`, `:455`) query
  `should_include_context` — witness: kernel-gated fixture changes selection
  with Go inputs fixed (`GAP-CTX-02` `T-RET-01`, `05:181`).
- I-02 Fallback never overrides (P2): `ScoreFactsWithKernelOverride`
  (`internal/context/activation_scoring.go:653`) hit-precedence + miss-breakdown
  + same-sort — witnesses `T-REL-01/02` (`05:178-179`).
- I-03 Kernel sets use the pre-filtered path only (P3):
  `SelectWithinBudgetPreFiltered` (`internal/context/activation.go:469`)
  with contract at `:460-468`, never `FilterByThreshold` (`:415`) — witness
  `T-REL-03` (`05:180`). Rationale: shipped threshold `105.0`
  (`internal/context/types.go:63`) prunes kernel priorities ≤100.
- I-04 Safety never silent-empty (P4): `getCoreFacts`
  (`internal/context/compressor.go:752-765`) Warn-continues over the hardcoded
  list (`:759`: `permitted`, `dangerous_action`, `admin_override`,
  `security_violation`, `block_commit`) — witness `T-RTN-01` failing-query
  fixture (`06:284`, `GAP-CTX-04`).
- I-05 Replace, don't accumulate (P5): `ReplaceControlFacts`
  (`internal/context/working_set.go:430`) + rationale (`:421-429`) +
  `working_revision` (`:360-361`) per round (2026-09-11 stale-revision
  incident) — witness `T-RET-02` (`05:182`, `GAP-CTX-03`).
- I-06 Budget triggers, window folds first (P6): `shouldCompress`
  (`internal/context/compressor_turns.go:241`) → `ShouldCompress`;
  `pruneRecentTurns` (`:715-736`) compresses before pruning — witness
  `T-RTN-02` (`06:285`, `GAP-CTX-04`). No turn-count trigger.
- I-07 Whole-unit omit with recovery ref (P7):
  (`internal/context/working_set.go:481-502`) — over-budget bodies omit whole
  with `[body outside active budget; recover with recall_context]` — witness
  `T-EVC-03` (`06:288`, `GAP-CTX-06`).
- I-08 Masking kernel-gated, drift fails closed (P8):
  `assertTurnAgeCategories` (`internal/context/compressor_metrics.go:671`)
  + `turnMaskID` (`:704`, `turn_<n>` contract) +
  `maskedObservationTurns` (`:717-772`) refuse-on-drift (`:762-766`) —
  witness `T-EVC-01` (`06:286`, `GAP-CTX-05`). Freeze
  `should_mask_observation/1` contract first.
- I-09 Placement is derived, ties deterministic (P9): `sortScoredFactsDesc`
  (`internal/context/activation.go:539`) + `Select` sort
  (`internal/context/working_set.go:446`) + `Build`
  (`internal/context/serializer.go:725-731`, body `:626-658`) order — witnesses
  `T-ORD-01/02` (`06:289-290`, `GAP-CTX-06`) plus exactly-one
  `heuristic_ordered` record on empty kernel.
- I-10 No stale survival (P10): revision joined before inclusion —
  `working_revision` per entity (`internal/context/working_set.go:361`) +
  `Revision` content identity (`:116-134`) — witness `T-RET-02`
  (`GAP-CTX-03`). North Star sentence: stale evidence must not survive a
  source change.
- I-11 Warn, never silent drop (P11): `processMemoryOperation`
  (`internal/context/compressor_turns.go:202-236`) + best-effort persist
  (`:168-181`) — witness `GAP-CTX-07` round-trip-or-narrow (`06:304-305`).
- I-12 Numbers are int64 (project rule, `05:119-120` / `06:215-217`): no
  float64 score / eviction / ordering fact enters the kernel — witness: boot
  aborts on float64 (kernel fixpoint rule). Scale ratios with
  `types.PercentFromRatio` before they reach a fact.

## 3. Re-read obligations (do not cite as shipped until re-read)

- `internal/context/activation_scoring.go:59-559` inner weights (see
  OQ-CTX-06) — `[upstream: task_aab9612b_1_0]`.
- `internal/context/compressor_turns.go` inner lines finer than function range
  (cutoff, 64-cap, age assert, ratio enforcement, merge cap — see OQ-CTX-07) —
  `[seam]` per `IMPLEMENTED_SPEC.md:38-44`.
- `internal/context/compressor_metrics.go` inner lines (keyAtoms64, mask
  queries, refuse-on-drift — see OQ-CTX-07) — `[seam]`.
- Severities / phases / dependencies in `03:43-51` — INFERRED (labeled as such
  in task_aab9612b_3_0); High on 01-06, Medium on 07 + ordering leg. Plan
  input, not shipped fact.
- Import edges (`session/working_context.go`, `working_meter.go`,
  `cmd/nerd/chat/*`, `cmd_context_stats.go`) — hypothesis, needs grep.
- `README.md` inner lines and `corpus.toml verified_on` — still stale (see
  OQ-CTX-08); the two pre-rewrite drafts that carried the other stale lines
  were deleted per standard Rule 3. Shipped claims must be written from code
  per standard Rule 4 (`Docs/journeys/09-architecture-doc-standard.md:162-170`).
