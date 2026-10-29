---
doc-class: north-star
subsystem: context
implementation-status: planned
last-verified: 2026-09-21
verified-against: 456e521
supersedes: []
---

# 05 — Relevance and retrieval (planned spec)

This file answers one question: what the finished relevance + retrieval
capability would exhibit, and which seam in today's code it would attach to.
It makes no shipped claims. Every code anchor below names a symbol that was
re-verified against the working tree this turn via element index; inner-line
details quoted from Phase 1 seam research are marked `[upstream]` until
re-read.

North Star trace: `agents.md:17-24` sentence 2 — "Mangle manages the active
working context throughout execution: relevance, retention, eviction,
retrieval, and ordering. A growing tool transcript with occasional
summarization does not fulfill this design." This spec covers two of the five
verbs (relevance, retrieval). Retention, eviction, and ordering belong in
sibling `05-…` specs, not here.

## 1. Finished behaviour (target state, planned)

In the finished package:

1. What enters the window would be a derived decision, not a Go heuristic
   sum. The kernel would derive `should_include_context/2` and
   `context_score/2` per fact; Go would keep keying, sorting, and rendering.
2. Where context comes from would be one live retrieval funnel with a stable
   contract: control facts → bounded dependency expansion → candidate fetch →
   kernel-gated inclusion → budgeted rendering with a recoverable reference
   for anything omitted.
3. Any omitted body would carry a recoverable reference (the
   `recall_context` seam), so eviction at the prompt layer would not be loss
   at the session layer. Recoverability itself is proven by a separate
   retrieval-round-trip test, not asserted here.

Present tense in this section describes the plan only.

## 2. Anchor seams in today's code (cited)

### 2.1 Relevance scorer — the heuristic this spec would replace

- `internal/context/activation_scoring.go:17-27` `scoreComponents` (nine
  fields) + `:29-31` `Total()` sum — verified this turn.
- `internal/context/activation_scoring.go:39-51` `computeScore` aggregates
  all nine components — verified this turn.
- `internal/context/activation_scoring.go:59-71` `computeBaseScore`,
  `:75-99` `computeRecencyScore`, `:102-221` `computeRelevanceScore`,
  `:225-264` `computeDependencyScore`, `:268-330` `computeCampaignScore`,
  `:333-342` `computeSessionScore`, `:347-449` `computeIssueScore`,
  `:454-474` `computeFeedbackScore`, `:479-559`
  `computeBackReferenceScore` — ranges verified this turn; inner weights
  (e.g. recency `<1m +50`, relevance verb×predicate table, dependency 30%
  inherit, campaign cap, issue tier boosts, back-reference cap)
  `[upstream: task_aab9612b_1_0]` and not re-asserted here.
- `internal/context/activation_scoring.go:653-698`
  `ScoreFactsWithKernelOverride` — verified this turn. Upstream describes it
  as the substitution point: empty `kernelScores` falls back to Go scoring;
  a hit takes kernel precedence; a miss falls back with component breakdown;
  output keeps the same-sort contract via `sortScoredFactsDesc`
  (`internal/context/activation.go:539-546`, verified this turn).

### 2.2 Threshold and budget selection

- `internal/context/activation.go:415-432` `FilterByThreshold`
  (score-gated filter) — verified this turn.
- `internal/context/activation.go:469-483`
  `SelectWithinBudgetPreFiltered` (budget selection without threshold) —
  verified this turn. Upstream reports this variant exists because a shipped
  threshold prunes kernel-scale priorities; the exact threshold constant is
  `[upstream]` until the body is re-read — this spec does not assert its
  value.
- `internal/context/activation.go:594-606` `GetHighActivationFacts`
  (apply + select) and `:610-671` `SpreadFromSeeds` (bounded spread) —
  verified this turn.
- `internal/context/activation.go:356-393` `buildSymbolGraphLocked`
  (consumes dependency/symbol edges) and `:278-285` `ScoreFacts` (Go-only
  entry) — verified this turn.

### 2.3 Live retrieval funnel

- `internal/context/working_set.go:308-512` `Select` — verified this turn.
  Upstream body-verified shape (control facts → two-hop `dependency_link`
  traversal capped at 64 entities → per-entity revision → `code_defines` /
  `code_element` queries → bounded candidates → kernel-gated inclusion →
  budgeted rendering) is `[upstream: task_aab9612b_1_0 §4a]`; this spec
  cites only the verified range and does not re-assert inner lines.
- `internal/context/compressor_turns.go:247-294` `recalcBudget` — verified
  this turn. Upstream reports it calls `GetHighActivationFacts` to refill the
  atom reserve; inner call line `[upstream]`.
- `internal/context/compressor_turns.go:30-184` `ProcessTurn` — verified
  this turn. Upstream wiring finding: zero non-test callers (exists-but-
  uncalled); the live path is the `WorkingSet` + recall-tool loop. This spec
  therefore anchors retrieval to `Select`, not to `ProcessTurn`.

## 3. Data shapes (target)

### 3.1 Inputs (Go-owned, passed to kernel)

| shape | fields | source seam |
|---|---|---|
| `FactView` | predicate string, arg vector (strings + ints normalised), turn id, fact key (`factKey`, `internal/context/activation.go:533-535`, verified) | `activation_scoring.go:566-642` normalisation helpers (ranges verified; int/int64/float64 drift handling `[upstream]`) |
| `IntentView` | verb atom (`/fix`, `/refactor`, `/research`, …), target string, focus paths/symbols | relevance range `:102-221` (verb×predicate table it would replace, `[upstream]` for rows) |
| `BudgetView` | char/token budget, already-committed cost, per-entity cost | `Select` signature `working_set.go:308` (`charBudget int`, verified); token reserves live in the compressor (retention spec, not here) |

### 3.2 Outputs (kernel-derived)

| predicate | arity | meaning (planned) |
|---|---|---|
| `context_score/2` | (FactKey, Score) | kernel's replacement for the nine-component `Total()`; Go keeps sort stability only |
| `should_include_context/2` | (Entity, Priority) | inclusion decision the funnel must honour before budget rendering; already queried by the funnel per upstream `working_set.go:433-462` `[upstream]` — this spec would promote it from consulted signal to gating decision |
| `working_selected/2` | (ID, Priority) | selected-set record for trace (see §6) |

Scores would be `/number` integers (project rule: numbers-are-int64; scale
ratios before they reach a fact). No float64 score fact may enter the kernel.

## 4. Go / Mangle split (planned)

| concern | Go | Mangle |
|---|---|---|
| keying / identity | `factKey` (`activation.go:533-535`) stays in Go | — |
| candidate generation | bounded traversal + fetch (`Select` range above) stays in Go; caps (64 entities, candidate limit) stay in Go as forcing budgets | — |
| scoring | fallback scorer stays in Go for empty-kernel and miss cases (`ScoreFactsWithKernelOverride` contract) | `context_score/2` decides on hit |
| inclusion | budget rendering + omission-with-reference stays in Go | `should_include_context/2` gates inclusion |
| ordering within budget | stable sort (`sortScoredFactsDesc`, `activation.go:539-546`) stays in Go | score values come from kernel |
| trace | Go emits selected/omitted records | kernel derivations recorded as witness (see §6) |

Fallback rule (planned): empty kernel map → Go scores; kernel hit →
kernel score wins; kernel miss → Go fallback with component breakdown
preserved for trace. The rule already exists as code shape at
`activation_scoring.go:653-698`; this spec would freeze its semantics and
add the proving tests in §7.

## 5. Failure modes (planned handling)

1. **Kernel silent (empty map).** Fall back to Go scoring; emit a trace
   record marking the turn as heuristic-scored. Never render an empty window
   without a record.
2. **Kernel partial (miss on some keys).** Per-fact fallback; hit facts keep
   kernel order relative to each other, miss facts follow with Go scores and
   a `fallback` flag. No silent mixing of scales.
3. **Scale drift (float vs int, absent vs zero).** Normalisation helpers
   (`activation_scoring.go:566-642`, ranges verified) would own coercion;
   absent never coerces to 0 without an explicit `missing` marker.
   `[upstream]` for current drift behaviour.
4. **Threshold vs kernel-priority mismatch.** Budget selection for
   kernel-scored sets must use the pre-filtered path
   (`activation.go:469-483`), never the threshold gate
   (`activation.go:415-432`), until the threshold story is re-verified.
   Exact constant `[upstream]`.
5. **Omission without recovery.** Any body omitted for budget must carry its
   recoverable reference; an omit without a reference is a spec violation
   and fails the §7 round-trip test.
6. **Stale evidence surviving a source change.** Retrieval must join
   per-entity revision (`WorkingSet.Revision`,
   `internal/context/working_set.go:116-134`, verified this turn) before
   inclusion; a revision mismatch forces refetch, never silent reuse.

## 6. Trace (what would prove what entered, what left, why)

- Selected set: `working_selected(ID, Priority)` + winning score source
  (kernel vs fallback) per fact.
- Omitted set: id + priority + reason (`over-budget` | `gated-out`) +
  recoverable reference. Omitted without reference fails closed.
- Heuristic turns: when the kernel map was empty, one record per turn
  (`heuristic_scored`) so a reviewer can distinguish derived order from Go
  order.

## 7. Proving tests (machine-checkable exits)

| id | proves | exit criterion |
|---|---|---|
| `T-REL-01` | kernel precedence | `ScoreFactsWithKernelOverride` with a non-empty fixture map derives the fixture's expected order; Go-only `ScoreFacts` (`activation.go:278-285`) on the same fixture may differ — the test pins the kernel order, not the heuristic |
| `T-REL-02` | miss fallback | kernel map missing one key → that fact carries Go breakdown + `fallback` flag; hit facts keep kernel relative order |
| `T-REL-03` | pre-filtered budget path | kernel-scored fixture including low-magnitude kernel priorities survives `SelectWithinBudgetPreFiltered` (`activation.go:469-483`); documents why `FilterByThreshold` (`:415-432`) is not used for kernel sets |
| `T-RET-01` | funnel contract | `Select` (`working_set.go:308-512`) on a seeded world returns `should_include_context`-gated entities within `charBudget`, and every omitted body carries a recoverable reference |
| `T-RET-02` | revision freshness | entity whose `Revision` (`working_set.go:116-134`) changed between seed and select is refetched, never served stale |
| `T-RET-03` | recoverability round-trip | omitted reference from `T-RET-01` resolves through the recall path to the full body (proves "evicted remains recoverable" for this funnel) |

## 8. Gap IDs (for `03-GAP-ANALYSIS.md`)

- `GAP-CTX-01` — kernel relevance derivation (`context_score/2`,
  `should_include_context/2`) with `ScoreFactsWithKernelOverride`
  (`activation_scoring.go:653-698`) as the seam. Exit: `T-REL-01` + `T-REL-02`
  pass.
- `GAP-CTX-02` — kernel-gated retrieval funnel on `Select`
  (`working_set.go:308-512`) with omission-with-reference. Exit: `T-RET-01` +
  `T-RET-03` pass.
- `GAP-CTX-03` — revision-joined retrieval (stale evidence must not survive
  a source change). Exit: `T-RET-02` passes.

## 9. Non-goals (explicitly out of this file)

Retention reserves, compression/eviction triggers, rolling-summary
rendering, prompt-layer Fit/Shed budgets, and cross-turn ordering belong in
their own `05-…` specs. `ProcessTurn`
(`compressor_turns.go:30-184`) internals belong in the retention/eviction
spec; it is cited here only to disavow it as the retrieval anchor.
