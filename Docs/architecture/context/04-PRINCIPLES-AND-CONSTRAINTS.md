---
doc-class: governance
subsystem: context
implementation-status: not-applicable
last-verified: 2026-09-21
verified-against: ea90cc63
supersedes: []
---

# 04 — Principles and Constraints (context)

Governance for `internal/context`. Each principle is normative for any change
to this package and carries the code or ruling it comes from. Go citations
give repo-relative path plus symbol plus line, verified this turn against the
workspace unless marked [upstream]. Ruling citations give document plus line.

## P1. The kernel decides what enters the window; Go computes

Relevance, retention, eviction, retrieval, and ordering are kernel
derivations, not Go heuristics. Go is the FFI, the drivers, and the tools;
the executive decisions are the fixpoint of the kernel over the facts.

- Ruling: `agents.md:11-15` (LLM as creative center; logic as executive),
  `agents.md:17-24` sentence 2 ("Mangle manages the active working context
  throughout execution: relevance, retention, eviction, retrieval, and
  ordering"), `agents.md:47-50` ("Clean fixpoint, not clean loop").
- Code: `Compressor.BuildContext` (`internal/context/compressor.go:645`)
  queries `should_include_context` (`internal/context/compressor.go:688`)
  and builds kernel-derived context first
  (`internal/context/compressor.go:695` `buildKernelDerivedContext`);
  `WorkingSet.Select` (`internal/context/working_set.go:308`) queries
  `working_selected(ID, Priority)` (`internal/context/working_set.go:433`)
  and `should_include_context(Entity, Priority)`
  (`internal/context/working_set.go:455`).

## P2. The Go scorer is the fallback, never the override

The nine-component Go heuristic may fill in where the kernel is silent. It
must never outrank a kernel score on the same fact.

- Code: `ActivationEngine.ScoreFactsWithKernelOverride`
  (`internal/context/activation_scoring.go:653`): empty `kernelScores`
  returns Go scoring (`internal/context/activation_scoring.go:654-656`);
  a kernel hit takes precedence
  (`internal/context/activation_scoring.go:668-673`); a miss falls back to
  `computeScore` with a component breakdown
  (`internal/context/activation_scoring.go:676-690`); output keeps the
  `ScoreFacts`-identical sort contract
  (`internal/context/activation_scoring.go:693-696`).
- Code: `Compressor.BuildContext` (`internal/context/compressor.go:645`)
  uses `GetHighActivationFacts` only when the kernel path yields nothing
  (`internal/context/compressor.go:707-712`).

## P3. Never route kernel priorities through the Go threshold gate

Kernel priorities and Go heuristic scores live on different scales. The
shipped Go threshold prunes every kernel-priority score, so kernel-derived
context must use the pre-filtered budget selector.

- Code: `ActivationEngine.FilterByThreshold`
  (`internal/context/activation.go:415`) gates on
  `Score >= threshold` (`internal/context/activation.go:421`);
  `ActivationEngine.SelectWithinBudget`
  (`internal/context/activation.go:437`) always filters first
  (`internal/context/activation.go:440`);
  `ActivationEngine.SelectWithinBudgetPreFiltered`
  (`internal/context/activation.go:469`) performs no threshold gate, per
  its contract (`internal/context/activation.go:460-468`: shipped default
  threshold 105.0 prunes kernel priorities bounded roughly 60–100).

## P4. Safety facts are retained fail-loud, never silent-empty

The constitutional core block must never be built empty because a query
failed. A failed safety-predicate query warns and continues; it does not
produce a context without safety facts.

- Code: `Compressor.getCoreFacts`
  (`internal/context/compressor.go:745`) queries the hardcoded retention
  list (`internal/context/compressor.go:759`
  `permitted, dangerous_action, admin_override, security_violation,
  block_commit`) and on error warns and continues
  (`internal/context/compressor.go:761-765`), per the comment at
  (`internal/context/compressor.go:752-758`).

## P5. Control and revision state is replaced, not accumulated

Evaluation is monotone, so stale control facts never age out on their own.
Any per-round assertion must replace the previous round's facts, and every
entity carries a fresh revision each round.

- Code: `WorkingSet.Select` (`internal/context/working_set.go:308`)
  asserts per-entity `working_revision`
  (`internal/context/working_set.go:360-361`) and replaces control facts
  via `ReplaceControlFacts` (`internal/context/working_set.go:430`); the
  replace-not-accumulate comment
  (`internal/context/working_set.go:421-429`) records the 2026-09-11
  stale-revision incident where accumulated revisions made every later
  observation derive stale and nothing was ever selected again.
- Code: `Compressor.recalcBudget`
  (`internal/context/compressor_turns.go:247`) recomputes usage and records
  it atomically via `SetUsage`
  (`internal/context/compressor_turns.go:290`).

## P6. Turns leave only through compression

Retention pressure is token-budget driven, not turn-count driven. Window
overflow compresses first; an uncompressed drop is a warned last resort,
never the normal path.

- Code: `Compressor.shouldCompress`
  (`internal/context/compressor_turns.go:241`) returns
  `budget.ShouldCompress()` (`internal/context/compressor_turns.go:242`),
  purely budget driven (`internal/context/compressor_turns.go:238-240`).
- Code: `Compressor.pruneRecentTurns`
  (`internal/context/compressor_turns.go:715`) keeps `2 * window`
  (`internal/context/compressor_turns.go:716`), compresses on overflow
  (`internal/context/compressor_turns.go:721-724`), and only then bounds
  memory with a warned drop
  (`internal/context/compressor_turns.go:728-734`); the comment
  (`internal/context/compressor_turns.go:703-714`) records the prior
  reslice-drop that deleted turns without folding ("forget everything
  older than two windows").
- Code: `Compressor.compress`
  (`internal/context/compressor_turns.go:297`) and
  `Compressor.mergeOldestSegments`
  (`internal/context/compressor_turns.go:611`), which preserves turn
  coverage and dropped-atom accounting across folds
  (`internal/context/compressor_turns.go:619-625`,
  `internal/context/compressor_turns.go:630-635`).

## P7. Eviction is whole-unit, and the omitted unit names its recovery

Never cut a unit mid-body. An observation that does not fit is omitted whole
and the omitted slot carries the recovery reference; a zero-priority record
is omitted; in-transcript records are skipped, not re-emitted.

- Code: `WorkingSet.Select` (`internal/context/working_set.go:308`):
  in-transcript skip (`internal/context/working_set.go:481-483`);
  `priorities == 0` omits (`internal/context/working_set.go:484-486`);
  the whole body is read (`internal/context/working_set.go:491`
  `Read(ctx, r.ID, 0, 0)` per the comment at
  `internal/context/working_set.go:488-490`); over-budget omits with a
  `[body outside active budget; recover with recall_context]` reference
  (`internal/context/working_set.go:496-502`).
- Ruling: `agents.md:17-24` sentence 2, tail ("Evicted context must remain
  recoverable"); the `recall_context` reference above is the seam, not the
  proof of recoverability.

## P8. Observation masking is kernel-gated with a preserve safety net

Go asserts turn ages; Mangle decides what may be masked. A turn marked for
masking but not for reasoning preservation is refused, not masked — rule
drift fails closed.

- Code: `Compressor.assertTurnAgeCategories`
  (`internal/context/compressor_metrics.go:671`) with thresholds
  `/recent <= 3`, `/mid <= 8`, `/old <= 15`, else `/ancient`
  (`internal/context/compressor_metrics.go:681-689`) and assertion
  (`internal/context/compressor_metrics.go:696`).
- Code: `turnMaskID` (`internal/context/compressor_metrics.go:704`) — Go
  and Mangle must agree on the `turn_<n>` shape
  (`internal/context/compressor_metrics.go:702-706`).
- Code: `Compressor.maskedObservationTurns`
  (`internal/context/compressor_metrics.go:717`) queries
  `should_mask_observation` (`internal/context/compressor_metrics.go:729`)
  and `should_preserve_reasoning`
  (`internal/context/compressor_metrics.go:739-751`), refusing to mask on
  drift (`internal/context/compressor_metrics.go:762-766`).

## P9. Ordering is a derived decision with stable tie-breaks

Where a fact sits in the window is a decision. Kernel priority orders;
Go tie-breaks are deterministic and documented so the same kernel state
renders the same block.

- Ruling: `agents.md:40-42` ("Lost-in-the-middle is eliminated by context
  compression, pruning and ordering — where a fact sits in the window is a
  decision").
- Code: `ScoreFactsWithKernelOverride`
  (`internal/context/activation_scoring.go:653`) sorts identically to
  `ScoreFacts` (`internal/context/activation_scoring.go:693-696`);
  `WorkingSet.Select` sorts priority desc, Step desc
  (`internal/context/working_set.go:446-451`); world facts are capped at a
  one-eighth share with focus-entity-first fill
  (`internal/context/working_set.go:463-473`);
  `renderRollingSummaryText`
  (`internal/context/compressor_turns.go:674`) renders header, per-segment
  `Turns Start-End`, summary, key atoms, truncation notice
  (`internal/context/compressor_turns.go:676-696`) in turn order.

## P10. Stale evidence must not survive a source change as current truth

A revision change retires the evidence keyed to the old revision. Selection
reads revisions, never bare entity names.

- Ruling: `agents.md:17-24` sentence 2, tail ("stale evidence must not
  survive a source change as current truth").
- Code: `WorkingSet.Select` (`internal/context/working_set.go:308`)
  asserts `working_revision` per entity
  (`internal/context/working_set.go:360-361`), walks at most two
  dependency hops capped at 64 entities
  (`internal/context/working_set.go:325-359`, cap at
  `internal/context/working_set.go:346`), pulls at most 256 candidates
  (`internal/context/working_set.go:373`), and treats recent observations
  as working memory whatever file they came from
  (`internal/context/working_set.go:377-385`).

## P11. Unimplemented protocol surface warns; nothing drops silently

A memory operation the schema admits but the switch does not implement is a
gap, and the code must read as a gap. Persistence failures warn with the key;
they are best-effort, not durable, and must not masquerade as durable.

- Code: `Compressor.processMemoryOperation`
  (`internal/context/compressor_turns.go:202`): `promote_to_long_term`
  failure warns (`internal/context/compressor_turns.go:208-211`);
  `store_vector` failure warns
  (`internal/context/compressor_turns.go:221-224`); `note` is accepted by
  schema, not implemented, Warn-dropped
  (`internal/context/compressor_turns.go:226-231`); unknown ops
  Warn-drop (`internal/context/compressor_turns.go:232-234`).
- Code: `Compressor.ProcessTurn`
  (`internal/context/compressor_turns.go:30`) persists best-effort with
  discarded errors (`internal/context/compressor_turns.go:168-181`).

## P12. Audit wiring before deleting; write shipped prose from the code

This tree carries partially wired features and dormant integration points.
Apparently-unused code is investigated (callers, tests, history) before it
is removed, and shipped documentation is derived from opened bodies, with
every Go file cited by symbol and line.

- Ruling: `agents.md:96` ("Always look for wiring gaps before deleting
  'unused' code"); `Docs/journeys/09-architecture-doc-standard.md:162-170`
  (Rule 4: shipped layer written from the code, never from previous docs);
  `Docs/journeys/09-architecture-doc-standard.md:124-136` (Rule 1a: every
  named Go file carries a symbol and a line);
  `Docs/journeys/09-architecture-doc-standard.md:154-160` (Rule 3: one
  question per file, replaced docs deleted not stubbed).
