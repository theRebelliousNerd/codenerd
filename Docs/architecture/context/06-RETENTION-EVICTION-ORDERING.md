---
doc-class: north-star
subsystem: context
implementation-status: planned
last-verified: 2026-09-21
verified-against: 456e521
supersedes: []
---

# 06 — Retention, eviction, ordering (planned spec)

This file answers one question: what the finished retention + eviction +
ordering capability would exhibit, and which seam in today's code it would
attach to. It makes no shipped claims. Every code anchor below names a
symbol that was re-verified against the working tree this turn via element
index, plus a body read where the range is quoted; inner-line details quoted
from Phase 1 seam research are marked `[upstream]` until re-read.

North Star trace: `agents.md:17-24` sentence 2 — "Mangle manages the active
working context throughout execution: relevance, retention, eviction,
retrieval, and ordering. A growing tool transcript with occasional
summarization does not fulfill this design. Evicted context must remain
recoverable; stale evidence must not survive a source change as current
truth." This spec covers three of the five verbs (retention, eviction,
ordering). Relevance and retrieval belong in sibling `05-RELEVANCE-AND-
RETRIEVAL.md`, not here.

Present tense in this section describes the plan only.

## 1. Finished behaviour (target state, planned)

In the finished package:

1. What stays in the window would be a derived decision, not token math.
   The kernel would derive `must_retain/1` per predicate (today's hardcoded
   safety list) and retention reserves would be kernel-gated, not Go
   constants.
2. What leaves the window would be a derived decision with a recoverable
   reference. The kernel would derive `should_mask_observation/1` (already
   done for turns — the exemplar) and, planned, `should_evict/2` for atoms;
   every evicted body would carry a reference resolvable through the recall
   path, so eviction at the prompt layer would not be loss at the session
   layer.
3. Where a fact sits in the window would be a derived decision. Vision
   (`agents.md:40-42`): "Lost-in-the-middle is eliminated by context
   compression, pruning and ordering — where a fact sits in the window is a
   decision." The kernel would derive `context_position/2` per fact; Go
   would keep keying, stable sorting, and rendering.
4. Compression would fold turns into segments without silent loss: turn
   coverage, masked counts, original-token counts, and dropped-atom counts
   are preserved across merges, and every cap that sheds atoms announces
   itself where the model reads the segment.

## 2. Anchor seams in today's code (cited)

### 2.1 Retention accounting — retention is token math today

- `internal/context/compressor_turns.go:247-294` `recalcBudget` — verified
  this turn (body read). Gathers core facts, scores atoms via
  `GetHighActivationFacts(allFacts, intent, AtomReserve)`, builds the
  context block, and records usage via `SetUsage`. Retention window =
  `recentWindow()` + `AtomReserve` fill; no kernel predicate gates
  retention here.
- `internal/context/tokens.go:424-432` `SetUsage` — verified this turn
  (element index). The sanctioned absolute setter for budget usage.
- `internal/context/tokens.go:184-200` `TokenBudget` struct — verified this
  turn (element index). Mutex-guarded `used{core,atoms,history,recent,
  working}`.
- `internal/context/tokens.go:228-280` `Allocate` — verified this turn
  (element index). Per-category allocation with reserve mapping
  `[upstream: task_aab9612b_1_0 §2a]`.
- `internal/context/tokens.go:378-393` `ShouldCompress` — verified this
  turn (element index). The budget predicate behind the trigger.
- `internal/context/compressor_turns.go:241-243` `shouldCompress` —
  verified this turn (body read). Purely token-budget driven
  (`budget.ShouldCompress()`), not turn-count driven.

### 2.2 Safety retention — always kept, hardcoded list

- `internal/context/compressor.go:745-771` `getCoreFacts` — verified this
  turn (body read). Queries `permitted, dangerous_action, admin_override,
  security_violation, block_commit` (`:759`); errors Warn-and-continue, never
  a silent empty safety block (`:761-766`).

Dream seam: retention policy = which predicates bypass eviction. Today the
list is hardcoded at `:759`; dream = a `must_retain/1` derivation the list
is read from.

### 2.3 Turn shape — what a retained turn carries

- `internal/context/compressor_turns.go:30-184` `ProcessTurn` — verified
  this turn (body read). Ten steps: `AssertBatch` + per-atom fallback
  (`:61-73`); `MarkNewFacts` (`:78`); `refreshActivationContextsLocked`
  (`:81`); memory ops (`:84-90`); `CompressedTurn` with Intent/Focus/Result
  split on `user_intent`/`focus_resolution` (`:94-110`); carry of
  `MangleUpdates`/`MemoryOperations` (`:113-116`); sliding window
  (`:119-127`); `recalcBudget` (`:130`); budget-gated `compress`
  (`:136-145`); prune (`:148-156`); best-effort persist + top-50 hot-fact
  log with `_ =` discards (`:168-181`).
- Upstream wiring finding `[upstream: task_aab9612b_1_0 §2c]`: `ProcessTurn`
  has zero non-test callers (16-row `callers_of`: def + 15 test hits). This
  spec therefore marks `ProcessTurn` as `exists-but-uncalled`, not the live
  turn path; the live path is the `WorkingSet` + recall-tool loop.

### 2.4 Long-term retention ops

- `internal/context/compressor_turns.go:202-236` `processMemoryOperation` —
  verified this turn (body read). `promote_to_long_term → StoreFact(…
  "preference", 10)` (`:204-212`, fail Warns, not persisted);
  `forget → Retract` (`:213-216`); `store_vector → StoreVector`
  (`:217-225`, fail Warns, not recallable); `note` accepted-by-schema but
  **not implemented**, Warn-drop (`:226-231`); default Warn-drop
  (`:232-235`).

Dream seam: `note` + unknown-op handling; exit = `note` persists and
recalls, or the schema narrows to implemented ops.

### 2.5 Eviction compressor — the fold path

- `internal/context/compressor_turns.go:297-401` `compress` — verified this
  turn (body read). Window check (`:298-302`); cutoff (`:305-306`);
  `collectKeyAtoms(…, 64)` (`:309`); `assertTurnAgeCategories` (`:318`);
  `maskedObservationTurns` (`:319`); `generateObservationMaskedSummary`
  (`:321`); ratio enforcement via `TargetCompressionRatio` (`:336-354`,
  atom-serialization fallback `:342-348` vs trim `:350-353`); segment fill
  (`:359-371`, shape below); rolling totals (`:374-380`);
  `rebuildRollingSummaryText` (`:388`); fresh-slice removal so the backing
  array does not pin compressed turns (`:390-394`); `DecayRecency(30m)`
  (`:398`).
- `internal/context/compressor_metrics.go:360-400` `collectKeyAtoms` —
  verified this turn (element index; shape `[upstream:
  task_aab9612b_1_0 §3c]`): per-turn result-atom cap, overall `limit`,
  dropped-count returned for rendering, dedup-is-not-drop.
- `internal/context/compressor_turns.go:556-605`
  `rebuildRollingSummaryText` — verified this turn (element index; inner
  lines `[upstream: task_aab9612b_1_0 §3c]`): `HistoryReserve` gate, oldest-
  half merge, single-segment floor trim, `DroppedAtoms` accounting, totals
  recompute.
- `internal/context/compressor_turns.go:611-671` `mergeOldestSegments` —
  verified this turn (element index; inner lines `[upstream:
  task_aab9612b_1_0 §3c]`): preserves turn coverage / masked counts /
  original tokens, 64-atom cap, halving re-trim, totals recompute.
- `internal/context/compressor_turns.go:674-701`
  `renderRollingSummaryText` — verified this turn (element index; inner
  lines `[upstream: task_aab9612b_1_0 §3c]`): header, `## Turns Start-End`,
  `# Key Atoms`, `TruncationNotice` where the model reads the segment.

### 2.6 Kernel-gated observation masking (C3) — the exemplar

- `internal/context/compressor_metrics.go:671-700`
  `assertTurnAgeCategories` — verified this turn (element index; categories
  `/recent ≤3 /mid ≤8 /old ≤15 /ancient` `[upstream:
  task_aab9612b_1_0 §3d]`).
- `internal/context/compressor_metrics.go:704-706` `turnMaskID` — verified
  this turn (element index). Go/Mangle id contract (`turn_<n>`)
  `[upstream: task_aab9612b_1_0 §3d]`.
- `internal/context/compressor_metrics.go:717-772`
  `maskedObservationTurns` — verified this turn (element index; inner lines
  `[upstream: task_aab9612b_1_0 §3d]`): reads `should_mask_observation`
  back out of the kernel and obeys it, with a `should_preserve_reasoning`
  safety net and refuse-to-mask on drift.

Eviction is already kernel-gated here — the exemplar for the other verbs.

### 2.7 Window safety net + per-round eviction

- `internal/context/compressor_turns.go:715-736` `pruneRecentTurns` —
  verified this turn (element index; inner lines `[upstream:
  task_aab9612b_1_0 §3b]`): `maxTurns = 2*window`, overflow compresses
  before prune, last-resort drop Warns with a fresh slice.
- `internal/context/working_set.go:480-508` per-round eviction — verified
  this turn (earlier body read in session context): `inTranscript` skip,
  `priorities==0 → Omitted`, over-budget → Omitted + `[body outside active
  budget; recover with recall_context]` reference. The `recall_context`
  reference is the seam, not the proof, of recoverability.

### 2.8 Ordering today — Go owns it entirely

- `internal/context/compressor.go:725-731` `BuildContext` assembly order —
  verified this turn (body read): `builder.Build(coreFacts, scoredFacts,
  rollingSummary.Text, recentTurns, turnNumber)` — core → atoms → history →
  recent. Fixed positional contract; no kernel input to placement.
- `internal/context/working_set.go:446-451` selection sort — verified this
  turn (earlier body read in session context): priority desc, Step desc
  tie-break.
- `internal/context/activation.go:539-546` `sortScoredFactsDesc`
  `[upstream: task_aab9612b_1_0 §1a]` — the stable-sort contract kernel
  scores must preserve.
- Neighbour, out of this file: prompt-layer ordering
  (`internal/prompt/assembler.go` Head/Tail category order,
  `internal/prompt/resolver.go` topological sort) belongs to the prompt
  package's spec, not here; this spec covers ordering of context content
  (facts, segments, turns), not prompt atoms.

## 3. Data shapes (target)

### 3.1 Inputs (Go-owned, passed to kernel)

| shape | fields | source seam |
|---|---|---|
| `RetentionView` | predicate name, reserve name (`core`/`atoms`/`history`/`recent`/`working`), current usage, reserve size | `TokenBudget` (`tokens.go:184-200`, verified); `SetUsage` (`tokens.go:424-432`, verified) |
| `EvictionCandidate` | fact key (`factKey`, `internal/context/activation.go:533-535` `[upstream]`), turn id (`turnMaskID`, `compressor_metrics.go:704-706`, verified), age category (`/recent`/`/mid`/`/old`/`/ancient`), size in tokens | `assertTurnAgeCategories` (`compressor_metrics.go:671-700`, verified) |
| `OrderView` | fact key, score + score source (kernel vs fallback), segment/turn position | `sortScoredFactsDesc` (`activation.go:539-546` `[upstream]`); `Build` order (`compressor.go:725-731`, verified) |
| `HistorySegment` | `ID, StartTurn, EndTurn, Summary, KeyAtoms, OriginalTokens, CompressedTokens, CompressionRatio, CompressedAt, MaskedTurns, DroppedAtoms` | `compress` segment fill (`compressor_turns.go:359-371`, verified) |

### 3.2 Outputs (kernel-derived)

| predicate | arity | meaning (planned) |
|---|---|---|
| `must_retain/1` | (Predicate) | predicate bypasses eviction; replaces the hardcoded list at `compressor.go:759` |
| `should_mask_observation/1` | (TurnID) | already derived today (C3); this spec freezes its contract: `/old`+`/ancient` masked, `preserve` holds |
| `should_evict/2` | (FactKey, Reason) | planned per-atom eviction decision with a machine-readable reason (`over-budget` \| `superseded` \| `stale-revision`) |
| `context_position/2` | (FactKey, Ordinal) | kernel's placement decision; Go keeps stable sort only |

Scores and ordinals would be `/number` integers (project rule:
numbers-are-int64; scale ratios before they reach a fact). No float64
eviction or ordering fact may enter the kernel.

## 4. Go / Mangle split (planned)

| concern | Go | Mangle |
|---|---|---|
| budget math | `TokenCounter`/`TokenBudget` counting, reserves, `SetUsage` stay in Go as forcing budgets | — |
| retention policy | hardcoded list at `compressor.go:759` becomes a read of the derivation | `must_retain/1` decides what bypasses eviction |
| fold mechanics | cutoff, `collectKeyAtoms` caps, segment fill, fresh-slice removal, merge/halve/trim stay in Go | — |
| eviction decision | `shouldCompress` trigger stays in Go (budget forcing function) | `should_mask_observation/1` (today) + `should_evict/2` (planned) decide what leaves |
| ordering within budget | stable sort (`sortScoredFactsDesc`, `working_set.go:446-451`) stays in Go | `context_position/2` values come from kernel |
| rendering | segment text, `# Key Atoms`, `TruncationNotice`, omission-with-reference stay in Go | — |
| trace | Go emits retained/evicted/ordered records | kernel derivations recorded as witness (see §6) |

Fallback rule (planned): kernel silent on an atom → Go budget rules apply
and the record is marked `heuristic_evicted`; kernel decision present → it
wins and the record carries the deriving predicate. No silent mixing: a
reviewer can tell derived placement from Go placement per fact.

## 5. Failure modes (planned handling)

1. **Safety list silently empty.** `getCoreFacts` already Warns-and-
   continues per predicate (`compressor.go:761-766`, verified). Planned:
   add a `must_retain` derivation witness — a render with zero safety facts
   fails closed unless the kernel positively derives the empty set.
2. **Merge resets loss accounting.** `mergeOldestSegments` preserves
   coverage/masked/original counts and adds per-segment `DroppedAtoms`
   `[upstream: task_aab9612b_1_0 §3c]`. Any future merge path must preserve
   all three counters; a merge that resets any of them to zero fails the
   §7 accounting test.
3. **Window overflow deletes without folding.** `pruneRecentTurns` compresses
   before prune with a Warn-and-drop last resort (`compressor_turns.go:
   715-736`, element-verified). The last-resort drop must stay Warn-level
   and counted; silent drops are a spec violation.
4. **Masking drifts from kernel truth.** `maskedObservationTurns`
   refuses-to-mask on drift `[upstream: task_aab9612b_1_0 §3d]`. Planned:
   any Go/Mangle `turnMaskID` disagreement fails the §7 masking test rather
   than masking the wrong turn.
5. **Eviction without recovery.** Any body evicted for budget (per-round
   omit at `working_set.go:480-508`, or segment trim) must carry its
   recoverable reference; an evict without a reference is a spec violation
   and fails the §7 round-trip test.
6. **Ordering silently heuristic.** When the kernel supplied no positions,
   Go order applies but the turn is recorded `heuristic_ordered`. A trace
   that cannot distinguish derived order from Go order fails closed.
7. **`note` accepted but dropped.** `processMemoryOperation` Warn-drops
   `note` (`compressor_turns.go:226-231`, verified). Either implement
   persistence + recall or narrow the schema to implemented ops; the
   current accept-and-drop shape must not survive.

## 6. Trace (what would prove what stayed, what left, why)

- Retained set: predicate + `must_retain` witness (or `heuristic_retained`
  when the kernel was silent).
- Evicted set: fact key / turn id + reason (`over-budget` | `superseded` |
  `stale-revision` | `masked`) + recoverable reference. Evicted without
  reference fails closed.
- Ordering: `context_position` per fact + score source (kernel vs
  fallback); heuristic turns carry one `heuristic_ordered` record.
- Fold accounting: per segment, `OriginalTokens` / `CompressedTokens` /
  `MaskedTurns` / `DroppedAtoms`, with merge-preservation checkable across
  `mergeOldestSegments` runs.

## 7. Proving tests (machine-checkable exits)

| id | proves | exit criterion |
|---|---|---|
| `T-RTN-01` | safety retention derived | `getCoreFacts` (`compressor.go:745-771`) on a fixture with a failing safety-predicate query still Warns and keeps the remaining predicates; a `must_retain/1` fixture derives exactly the retained set |
| `T-RTN-02` | budget trigger honesty | `recalcBudget` (`compressor_turns.go:247-294`) + `shouldCompress` (`:241-243`) on a fixture crossing the threshold triggers `compress`; below threshold it does not — no turn-count trigger exists |
| `T-EVC-01` | kernel-gated masking | kernel marks `/old`+`/ancient` fixture turns via `should_mask_observation` while `preserve` holds; `maskedObservationTurns` (`compressor_metrics.go:717-772`) masks exactly those turns and `MaskedTurns` is counted in the segment |
| `T-EVC-02` | fold accounting preserved | `compress` (`compressor_turns.go:297-401`) + `mergeOldestSegments` (`:611-671`) on a fixture: merged segment preserves turn coverage, masked counts, original tokens, and summed `DroppedAtoms` — none reset to zero |
| `T-EVC-03` | eviction recoverable | every body omitted at `working_set.go:480-508` carries a `recall_context` reference, and the reference resolves to the full body through the recall path |
| `T-ORD-01` | derived ordering | `context_position/2` fixture derives expected placement through the stable sort contract (`activation.go:539-546`); Go-only order on the same fixture may differ — the test pins the kernel order |
| `T-ORD-02` | heuristic marking | empty kernel positions → Go order applies and the turn carries exactly one `heuristic_ordered` record; no silent mixing |

## 8. Gap IDs (for `03-GAP-ANALYSIS.md`)

- `GAP-CTX-04` — kernel retention derivation (`must_retain/1`) replacing
  the hardcoded list at `compressor.go:759`. Exit: `T-RTN-01` passes.
- `GAP-CTX-05` — kernel-gated eviction with fold accounting
  (`should_mask_observation/1` frozen, `should_evict/2` planned;
  `compress`, `mergeOldestSegments`, `pruneRecentTurns` as seams). Exit:
  `T-EVC-01` + `T-EVC-02` pass.
- `GAP-CTX-06` — recoverable eviction + derived ordering
  (`context_position/2`; omission-with-reference at `working_set.go:
  480-508`). Exit: `T-EVC-03` + `T-ORD-01` + `T-ORD-02` pass.
- `GAP-CTX-07` — `note` memory op persists and recalls, or the protocol
  schema narrows to implemented ops (`compressor_turns.go:202-236`).
  Exit: `note` round-trips, or the schema rejects `note` at the boundary.

## 9. Non-goals (explicitly out of this file)

Relevance scoring (`computeScore` nine-component heuristic,
`ScoreFactsWithKernelOverride`), the retrieval funnel (`WorkingSet.Select`
traversal/candidates/inclusion), prompt-layer Fit/Shed budgets, prompt-atom
ordering (`internal/prompt/assembler.go`, `internal/prompt/resolver.go`),
and cross-turn `ProcessTurn`-as-live-path claims (disavowed in §2.3) belong
in their own specs. `ProcessTurn` (`compressor_turns.go:30-184`) internals
are cited here only as the retention shape and the exists-but-uncalled
wiring fact.
