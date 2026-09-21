---
doc-class: shipped
subsystem: context
implementation-status: shipped
last-verified: 2026-09-21
verified-against: 456e521
supersedes: []
---

# IMPLEMENTED_SPEC — `internal/context` (authoritative shipped record)

> This file answers one question: what `internal/context` builds and reaches
> today. Every code claim cites a repo-relative path plus a symbol plus a
> line, verified against the working tree (element index this turn; body reads
> per §11). **On any disagreement between this file and any other document in
> `Docs/architecture/context/` (`README.md`, `INTERNALS.md`,
> `WIRING-AND-NOT-BUILT.md`, `09-MANGLE-SURFACE.md`, `01-VISION.md`,
> `04-PRINCIPLES-AND-CONSTRAINTS.md`, `05-RELEVANCE-AND-RETRIEVAL.md`,
> `06-RETENTION-EVICTION-ORDERING.md`), this file wins.**
> Planned behaviour lives only in `01-VISION.md`,
> `05-RELEVANCE-AND-RETRIEVAL.md`, and `06-RETENTION-EVICTION-ORDERING.md`
> (all `planned`); this file makes no planned claims.

## 0. Verification basis and corrections

- Element-index verification this turn covers every symbol range cited below
  (`package_outline internal/context`, 410 rows, complete).
- Body-read verification this turn (campaign task `task_aab9612b_2_0`) covers:
  `WorkingSet.Select` (`internal/context/working_set.go:308-512`);
  `ActivationEngine` methods `internal/context/activation.go:356-717`;
  `Compressor` tail `internal/context/compressor.go:520-785`;
  `CompressorConfig`/`CompressedContext` (`internal/context/types.go:19-175`);
  `FactSerializer` full span (`internal/context/serializer.go:1-390`,
  `359-659`); `TokenCounter`/`TokenBudget` head
  (`internal/context/tokens.go:1-280`); `ContextFeedbackStore` head
  (`internal/context/feedback_store.go:1-400`); `WorkingStore` full
  (`internal/context/working_store.go:1-240`).
- Carried seam reads (campaign task `task_aab9612b_1_0`, body-verified in that
  task, not re-opened this turn) cover inner lines of
  `internal/context/activation_scoring.go:17-698`,
  `internal/context/compressor_turns.go:30-736`, and
  `internal/context/compressor_metrics.go:360-772`. Inner-line numbers for
  those three files are cited from that artifact and marked `[seam]`; all
  function ranges are element-verified this turn.
- **Correction 1:** `WorkingSet.Select` is
  `internal/context/working_set.go:308-512`
  (`WorkingSet.Select`, element-verified). Claims in `README.md` and
  `INTERNALS.md` attributing it to `:297` are stale and lose to this file.
- **Correction 2:** `ContextBlockBuilder` spans
  `internal/context/serializer.go:612-658` as struct plus constructor plus
  `Build` (`ContextBlockBuilder`, `internal/context/serializer.go:612-615`;
  `NewContextBlockBuilder`, `internal/context/serializer.go:618-623`;
  `Build`, `internal/context/serializer.go:626-658`). A bare `:612-658`
  header for `Build` alone is off by ~14 lines.
- **Correction 3:** `TokenCounter.Confidence` is
  `internal/context/tokens.go:58-60` and `Ratio` is
  `internal/context/tokens.go:63` (`TokenCounter.Confidence`,
  `TokenCounter.Ratio`, element-verified); the `:55-63` approximation is
  ~3-line drift.

## 1. Package shape

`internal/context` holds two bounded-context loops plus their scoring,
accounting, serialization, storage, and learning support. File by file
(each row names a symbol and line drawn from inside the file):

| file | what it holds (verified symbols) |
|---|---|
| `internal/context/types.go` | `CompressorConfig` (`internal/context/types.go:19-40`); `DefaultConfig` (`internal/context/types.go:44-130`); `NewConfigWithBudget` (`internal/context/types.go:135-147`); `CompressedContext` (`internal/context/types.go:155-175`); `CompressedTurn` (`internal/context/types.go:179-198`); `TokenUsage` (`internal/context/types.go:201-212`); `ScoredFact` (`internal/context/types.go:219-233`); `SelectionMode` (`internal/context/types.go:237`) with `SelectionKernel` (`internal/context/types.go:241`) / `SelectionGoFallback` (`internal/context/types.go:243`); `SelectionStats` (`internal/context/types.go:260-276`) with `KernelInclusionRate` (`internal/context/types.go:280-286`); `ActivationState` (`internal/context/types.go:289-305`); `Turn` (`internal/context/types.go:312-326`); `TurnResult` (`internal/context/types.go:329-341`); `HistorySegment` (`internal/context/types.go:348-378`); `RollingSummary` (`internal/context/types.go:381-402`); `CompressedState` (`internal/context/types.go:409-428`) |
| `internal/context/activation.go` | `ActivationEngine` (`internal/context/activation.go:31-85`); `NewActivationEngine` (`internal/context/activation.go:179-192`); `ScoreFacts` (`internal/context/activation.go:278-285`); `buildSymbolGraphLocked` (`internal/context/activation.go:356-393`); `FilterByThreshold` (`internal/context/activation.go:415-432`); `SelectWithinBudget` (`internal/context/activation.go:437-458`); `SelectWithinBudgetPreFiltered` (`internal/context/activation.go:469-483`); `factKey` (`internal/context/activation.go:533-535`); `sortScoredFactsDesc` (`internal/context/activation.go:539-546`); `ApplyIntentActivation` (`internal/context/activation.go:579-591`); `GetHighActivationFacts` (`internal/context/activation.go:594-606`); `SpreadFromSeeds` (`internal/context/activation.go:610-671`); `GetState`/`SetState` (`internal/context/activation.go:683-695`) |
| `internal/context/activation_scoring.go` | `scoreComponents` (`internal/context/activation_scoring.go:17-27`) with `Total` (`internal/context/activation_scoring.go:29-31`); `computeScore` (`internal/context/activation_scoring.go:39-51`); `computeBaseScore` (`internal/context/activation_scoring.go:59-71`); `computeRecencyScore` (`internal/context/activation_scoring.go:75-99`); `computeRelevanceScore` (`internal/context/activation_scoring.go:102-221`); `computeDependencyScore` (`internal/context/activation_scoring.go:225-264`); `computeCampaignScore` (`internal/context/activation_scoring.go:268-330`); `computeSessionScore` (`internal/context/activation_scoring.go:333-342`); `computeIssueScore` (`internal/context/activation_scoring.go:347-449`); `computeFeedbackScore` (`internal/context/activation_scoring.go:454-474`); `computeBackReferenceScore` (`internal/context/activation_scoring.go:479-559`); `ScoreFactsWithKernelOverride` (`internal/context/activation_scoring.go:653-698`) |
| `internal/context/compressor.go` | `Compressor` (`internal/context/compressor.go:29-64`); `NewCompressor` (`internal/context/compressor.go:67-92`); `NewCompressorWithConfig` (`internal/context/compressor.go:563-584`); `NewCompressorWithParams` (`internal/context/compressor.go:588-612`); `recentWindow` (`internal/context/compressor.go:638-640`); `BuildContext` (`internal/context/compressor.go:645-742`); `getCoreFacts` (`internal/context/compressor.go:745-771`); `GetContextString` (`internal/context/compressor.go:774-784`) |
| `internal/context/compressor_turns.go` | `ProcessTurn` (`internal/context/compressor_turns.go:30-184`); `processMemoryOperation` (`internal/context/compressor_turns.go:202-236`); `shouldCompress` (`internal/context/compressor_turns.go:241-243`); `recalcBudget` (`internal/context/compressor_turns.go:247-294`); `compress` (`internal/context/compressor_turns.go:297-401`); `rebuildRollingSummaryText` (`internal/context/compressor_turns.go:556-605`); `mergeOldestSegments` (`internal/context/compressor_turns.go:611-671`); `renderRollingSummaryText` (`internal/context/compressor_turns.go:674-701`); `pruneRecentTurns` (`internal/context/compressor_turns.go:715-736`) |
| `internal/context/compressor_metrics.go` | `GetMetrics` (`internal/context/compressor_metrics.go:22-49`); `GetSelectionStats` (`internal/context/compressor_metrics.go:145-149`); `GetState`/`LoadState` (`internal/context/compressor_metrics.go:242-317`); `collectKeyAtoms` (`internal/context/compressor_metrics.go:360-400`); `trimToTokens` (`internal/context/compressor_metrics.go:410-435`); `buildKernelDerivedContext` (`internal/context/compressor_metrics.go:548-659`); `assertTurnAgeCategories` (`internal/context/compressor_metrics.go:671-700`); `turnMaskID` (`internal/context/compressor_metrics.go:704-706`); `maskedObservationTurns` (`internal/context/compressor_metrics.go:717-772`) |
| `internal/context/working_set.go` | `WorkingSet` (`internal/context/working_set.go:33-40`); `NewWorkingSet` (`internal/context/working_set.go:42-78`); `Recall` (`internal/context/working_set.go:88-103`); `Revision` (`internal/context/working_set.go:116-134`); `Continue` (`internal/context/working_set.go:175-243`); `TranscriptRounds` (`internal/context/working_set.go:254-266`); `SectionCeiling` (`internal/context/working_set.go:271-283`); `RepeatThreshold` (`internal/context/working_set.go:291-303`); `Select` (`internal/context/working_set.go:308-512`) |
| `internal/context/working_store.go` | `WorkingRecord` (`internal/context/working_store.go:27-38`); `WorkingStore` (`internal/context/working_store.go:47-50`); `OpenWorkingStore` (`internal/context/working_store.go:52-84`); `Search` (`internal/context/working_store.go:106-134`); `Save` (`internal/context/working_store.go:136-149`); `Candidates` (`internal/context/working_store.go:152-175`); `Read` (`internal/context/working_store.go:219-239`) |
| `internal/context/serializer.go` | `FactSerializer` (`internal/context/serializer.go:23-32`); `SerializeFacts` (`internal/context/serializer.go:91-100`); `renderFact` (`internal/context/serializer.go:119-125`); `truncateFact` (`internal/context/serializer.go:180-201`); `SerializeCompressedContext` (`internal/context/serializer.go:251-298`); `ExtractAtomsFromControlPacket` (`internal/context/serializer.go:321-355`); `ParseMangleAtom` (`internal/context/serializer.go:359-390`); `ContextBlockBuilder.Build` (`internal/context/serializer.go:626-658`) |
| `internal/context/tokens.go` | `TokenCounter` (`internal/context/tokens.go:38-40`) with `CountString` (`internal/context/tokens.go:48-53`) and `CountFact` (`internal/context/tokens.go:66-98`); `TokenBudget` (`internal/context/tokens.go:184-200`) with `Allocate` (`internal/context/tokens.go:228-280`), `ShouldCompress` (`internal/context/tokens.go:378-393`), and `SetUsage` (`internal/context/tokens.go:424-432`) |
| `internal/context/feedback_store.go` | `ContextFeedbackStore` (`internal/context/feedback_store.go:25-38`); `NewContextFeedbackStore` (`internal/context/feedback_store.go:63-83`); `StoreFeedback` (`internal/context/feedback_store.go:118-209`); `GetPredicateUsefulness` (`internal/context/feedback_store.go:213-230`); `computePredicateScore` (`internal/context/feedback_store.go:255-341`); `CollectFeedbackStats` (`internal/context/feedback_store.go:506-541`) |

## 2. Configuration and context shapes

- `CompressorConfig` (`internal/context/types.go:19-40`) carries total and
  reserve budgets, window sizes, thresholds, and ratios. Shipped defaults come
  from `DefaultConfig` (`internal/context/types.go:44-130`), including
  `ActivationThreshold 105.0` with the base-plus-recency comment
  (`internal/context/types.go:59-63`, body-read). Budget-derived configs come
  from `NewConfigWithBudget` (`internal/context/types.go:135-147`).
- `CompressedContext` (`internal/context/types.go:155-175`) is the minimal
  LLM-call context; `CompressedTurn` (`internal/context/types.go:179-198`)
  is one turn with surface text removed; `TokenUsage`
  (`internal/context/types.go:201-212`) tracks per-component allocation;
  `ScoredFact` (`internal/context/types.go:219-233`) pairs a fact with its
  activation score.
- Selection provenance is a shipped shape: `SelectionMode`
  (`internal/context/types.go:237`) with `SelectionKernel`
  (`internal/context/types.go:241`) and `SelectionGoFallback`
  (`internal/context/types.go:243`), reason constants
  (`internal/context/types.go:251-254`), `SelectionStats`
  (`internal/context/types.go:260-276`) and its `KernelInclusionRate`
  (`internal/context/types.go:280-286`). The kernel-vs-Go split is assertable,
  not log-only.
- Persistence shape is `CompressedState`
  (`internal/context/types.go:409-428`) over `RollingSummary`
  (`internal/context/types.go:381-402`) and `HistorySegment`
  (`internal/context/types.go:348-378`), with turn inputs `Turn`
  (`internal/context/types.go:312-326`) and `TurnResult`
  (`internal/context/types.go:329-341`) and engine state `ActivationState`
  (`internal/context/types.go:289-305`).

## 3. Relevance scoring (Go heuristic with kernel override)

- The engine is `ActivationEngine`
  (`internal/context/activation.go:31-85`), built by `NewActivationEngine`
  (`internal/context/activation.go:179-192`). Campaign, issue, and
  back-reference contexts are settable and clearable
  (`SetCampaignContext`/`ClearCampaignContext`,
  `internal/context/activation.go:195-206`; `SetIssueContext`/
  `ClearIssueContext`, `internal/context/activation.go:210-221`;
  `SetBackReferenceContext`/`ClearBackReferenceContext`,
  `internal/context/activation.go:225-236`).
- Go-only scoring entry is `ScoreFacts`
  (`internal/context/activation.go:278-285`) via `scoreFactsLocked`
  (`internal/context/activation.go:289-347`). Symbol edges come from
  `buildSymbolGraphLocked` (`internal/context/activation.go:356-393`),
  which consumes dependency and symbol edges.
- The nine-component sum is `computeScore`
  (`internal/context/activation_scoring.go:39-51`) over `scoreComponents`
  (`internal/context/activation_scoring.go:17-27`) summed by `Total`
  (`internal/context/activation_scoring.go:29-31`): `computeBaseScore`
  (`internal/context/activation_scoring.go:59-71`), `computeRecencyScore`
  (`internal/context/activation_scoring.go:75-99`), `computeRelevanceScore`
  (`internal/context/activation_scoring.go:102-221`), `computeDependencyScore`
  (`internal/context/activation_scoring.go:225-264`), `computeCampaignScore`
  (`internal/context/activation_scoring.go:268-330`),
  `computeSessionScore` (`internal/context/activation_scoring.go:333-342`),
  `computeIssueScore` (`internal/context/activation_scoring.go:347-449`),
  `computeFeedbackScore`
  (`internal/context/activation_scoring.go:454-474`), and
  `computeBackReferenceScore`
  (`internal/context/activation_scoring.go:479-559`). Normalisation helpers
  `factArgAsString` (`internal/context/activation_scoring.go:566-577`)
  through `factTurnIDAt` (`internal/context/activation_scoring.go:636-642`)
  own the Mangle-atom/int-drift coercion.
- Threshold and budget selection are three distinct shipped paths, and they
  must not be conflated: `FilterByThreshold`
  (`internal/context/activation.go:415-432`) gates on score versus threshold;
  `SelectWithinBudget` (`internal/context/activation.go:437-458`) filters
  first (threshold-first, defensive); `SelectWithinBudgetPreFiltered`
  (`internal/context/activation.go:469-483`) performs no threshold gate and is
  the required path for kernel-priority sets that the shipped 105.0 default
  would otherwise prune (contract comment `[seam]` at
  `internal/context/activation.go:460-468`).
- Kernel substitution point is `ScoreFactsWithKernelOverride`
  (`internal/context/activation_scoring.go:653-698`): empty kernel map falls
  back to Go scoring `[seam]` (`internal/context/activation_scoring.go:654`);
  a kernel hit takes precedence `[seam]`
  (`internal/context/activation_scoring.go:668-673`); a miss falls back with
  component breakdown `[seam]`
  (`internal/context/activation_scoring.go:676-690`); output keeps the
  `ScoreFacts`-identical sort contract via `sortScoredFactsDesc`
  (`internal/context/activation.go:539-546`) `[seam]`
  (`internal/context/activation_scoring.go:693-696`).
- Convenience and spread: `ApplyIntentActivation`
  (`internal/context/activation.go:579-591`); `GetHighActivationFacts`
  (`internal/context/activation.go:594-606`) equals apply-plus-select;
  `SpreadFromSeeds` (`internal/context/activation.go:610-671`) spreads with
  the `0.5*0.7^d` decay `[seam]`
  (`internal/context/activation_scoring.go:656` analogue at
  `internal/context/activation.go:656`).
- Identity and ordering primitives: `factKey`
  (`internal/context/activation.go:533-535`) is the fact string;
  `extractPredicate` (`internal/context/activation.go:549-555`) reads the
  predicate; `lookupPriority` (`internal/context/activation.go:559-571`)
  resolves corpus then config then 50; focused paths update via
  `UpdateFocusedPaths` (`internal/context/activation.go:486-490`) and
  `updateFocusedPathsLocked` (`internal/context/activation.go:493-507`).

## 4. Compressor: context assembly, turns, and eviction folds

- The compressor is `Compressor`
  (`internal/context/compressor.go:29-64`), built by `NewCompressor`
  (`internal/context/compressor.go:67-92`), `NewCompressorWithConfig`
  (`internal/context/compressor.go:563-584`), `NewCompressorWithParams`
  (`internal/context/compressor.go:588-612`), or the internal
  `newCompressorWithCompressorConfig`
  (`internal/context/compressor.go:615-628`). The recent-turn window is
  clamped at zero by `recentWindow`
  (`internal/context/compressor.go:638-640`).
- Assembly order is kernel-first: `BuildContext`
  (`internal/context/compressor.go:645-742`) queries `should_include_context`
  (query site body-read at `internal/context/compressor.go:688`), builds
  kernel-derived context first via `buildKernelDerivedContext`
  (`internal/context/compressor_metrics.go:548-659`), falls back to
  `GetHighActivationFacts` only when the kernel path yields nothing
  (body-read `internal/context/compressor.go:690-712`), adds core facts
  (body-read `internal/context/compressor.go:719-720`), assembles through
  `ContextBlockBuilder.Build` (`internal/context/serializer.go:626-658`,
  call body-read at `internal/context/compressor.go:725-731`), and records
  usage (body-read `internal/context/compressor.go:734`). Surfaced string
  form is `GetContextString` (`internal/context/compressor.go:774-784`).
- Safety retention is hardcoded and fail-loud: `getCoreFacts`
  (`internal/context/compressor.go:745-771`) queries the retention list at
  `internal/context/compressor.go:759` (`permitted, dangerous_action,
  admin_override, security_violation, block_commit`) and on per-predicate
  error warns and continues, never returning a silent empty safety block
  (body-read `internal/context/compressor.go:752-765`).
- Turn intake is `ProcessTurn`
  (`internal/context/compressor_turns.go:30-184`): per-atom assert with
  fallback `[seam]` (`internal/context/compressor_turns.go:61-73`), recency
  marking, activation-context refresh, memory ops, Intent/Focus/Result split,
  sliding window, budget recompute, budget-gated compression, prune, and
  best-effort persist with discarded errors `[seam]`
  (`internal/context/compressor_turns.go:168-181`). Wiring status (live path
  versus exists-but-uncalled) is owned by `WIRING-AND-NOT-BUILT.md`; this
  spec asserts existence and shape only.
- Budget recompute is `recalcBudget`
  (`internal/context/compressor_turns.go:247-294`), which refills the atom
  reserve through `GetHighActivationFacts` and records usage atomically via
  `SetUsage` (`internal/context/tokens.go:424-432`, call `[seam]` at
  `internal/context/compressor_turns.go:290`). The trigger is purely
  budget-driven: `shouldCompress`
  (`internal/context/compressor_turns.go:241-243`) returns
  `budget.ShouldCompress()`.
- The fold path is `compress`
  (`internal/context/compressor_turns.go:297-401`) with window check, cutoff,
  64-atom key collection, age-category assert, masked summary, ratio
  enforcement, segment fill, rolling totals, summary rebuild, fresh-slice
  removal, and 30-minute recency decay `[seam]`. History rendering under
  reserve is `rebuildRollingSummaryText`
  (`internal/context/compressor_turns.go:556-605`),
  `mergeOldestSegments` (`internal/context/compressor_turns.go:611-671`)
  with its 64-atom cap `[seam]`, and `renderRollingSummaryText`
  (`internal/context/compressor_turns.go:674-701`). Key-atom extraction is
  `collectKeyAtoms` (`internal/context/compressor_metrics.go:360-400`) with
  per-turn and overall caps and a returned dropped count.
- The last-resort net is `pruneRecentTurns`
  (`internal/context/compressor_turns.go:715-736`): bound `2*window` `[seam]`,
  compress-before-prune, warned drop on a fresh slice. It bounds memory; it
  is not the normal eviction path.
- Long-term ops are `processMemoryOperation`
  (`internal/context/compressor_turns.go:202-236`): `promote_to_long_term`
  stores a preference (failure warns) `[seam]`; `forget` retracts `[seam]`;
  `store_vector` stores a vector (failure warns, not recallable) `[seam]`;
  `note` is accepted by schema but not implemented and is Warn-dropped
  (body-read-adjacent `[seam]` at
  `internal/context/compressor_turns.go:226-231`); unknown ops Warn-drop
  `[seam]`.
- Kernel-gated masking is shipped: `assertTurnAgeCategories`
  (`internal/context/compressor_metrics.go:671-700`) writes
  `/recent <= 3`, `/mid <= 8`, `/old <= 15`, else `/ancient` `[seam]`;
  `turnMaskID` (`internal/context/compressor_metrics.go:704-706`) names
  `turn_<n>` (Go/Mangle contract); `maskedObservationTurns`
  (`internal/context/compressor_metrics.go:717-772`) reads
  `should_mask_observation` with a `should_preserve_reasoning` safety net and
  refuses to mask on drift `[seam]`.
- Metrics, selection accounting, and persistence: `GetMetrics`
  (`internal/context/compressor_metrics.go:22-49`),
  `GetCompressionRatio` (`internal/context/compressor_metrics.go:52-60`),
  `GetBudgetUtilization` (`internal/context/compressor_metrics.go:64-71`),
  `GetBudgetUsage` (`internal/context/compressor_metrics.go:75-82`),
  `RefreshBudget` (`internal/context/compressor_metrics.go:94-98`),
  `IsCompressionActive` (`internal/context/compressor_metrics.go:104-121`),
  `GetSelectionStats` (`internal/context/compressor_metrics.go:145-149`),
  `GetRecentTurnWindow` (`internal/context/compressor_metrics.go:152-154`),
  `GetState`/`LoadState` (`internal/context/compressor_metrics.go:242-317`),
  `Reset` (`internal/context/compressor_metrics.go:320-336`),
  `GetActivationScores` (`internal/context/compressor_metrics.go:470-505`),
  and `GetHighActivationFactKeys`
  (`internal/context/compressor_metrics.go:509-520`). Session binding is
  `SetSessionID`/`GetSessionID`
  (`internal/context/compressor.go:522-539`); feedback wiring is
  `SetFeedbackStore` (`internal/context/compressor.go:543-550`) with
  `GetFeedbackStats` (`internal/context/compressor.go:555-560`).

## 5. Working set and working store (bounded retrieval funnel)

- Scope and handles: `WorkingSet`
  (`internal/context/working_set.go:33-40`) built by `NewWorkingSet`
  (`internal/context/working_set.go:42-78`) over `WorkingWorld`
  (`internal/context/working_set.go:27-29`); policy handle
  `workingSetPolicy` (`internal/context/working_set.go:25`). Rounds report
  `WorkingProgress` (`internal/context/working_set.go:145-157`) and receive
  `WorkingDecision` (`internal/context/working_set.go:163-171`) through
  `Continue` (`internal/context/working_set.go:175-243`).
- The live funnel is `Select` (`internal/context/working_set.go:308-512`,
  body-read): control facts `user_intent`/`focus_resolution`
  (body-read `internal/context/working_set.go:323-324`); two-hop
  `dependency_link` traversal capped at 64 entities
  (body-read `internal/context/working_set.go:325-359`, cap check at
  `internal/context/working_set.go:346`); per-entity `working_revision`
  (body-read `internal/context/working_set.go:361`); `code_defines` /
  `code_element` queries (body-read `internal/context/working_set.go:365-371`);
  `Candidates` at 256 (body-read `internal/context/working_set.go:373-376`)
  with recent-as-working-memory priority (body-read
  `internal/context/working_set.go:377-385`); missing-recents fetch
  (body-read `internal/context/working_set.go:390-407`);
  `working_observation`/`digest`/`span` asserts plus `working_recent` and
  `working_in_transcript` (body-read
  `internal/context/working_set.go:408-420`); control-fact replacement via
  `ReplaceControlFacts` (call at `internal/context/working_set.go:430`,
  replace-not-accumulate rationale body-read at
  `internal/context/working_set.go:421-429`); `working_selected(ID,Priority)`
  query with priority-desc/Step-desc sort and
  `should_include_context(Entity,Priority)` decisions (body-read
  `internal/context/working_set.go:433-462`); kernel-derived build with a
  one-eighth fact share (body-read `internal/context/working_set.go:463-473`,
  `buildKernelDerivedContext` at `internal/context/working_set.go:473`).
- Per-round eviction is whole-unit with a named recovery (body-read
  `internal/context/working_set.go:480-508`): in-transcript skip
  (`internal/context/working_set.go:481-483`); zero priority omits
  (`internal/context/working_set.go:484-486`); whole-body read
  (`WorkingStore.Read`, `internal/context/working_store.go:219-239`, called
  whole at `internal/context/working_set.go:491`); over-budget omits with a
  `[body outside active budget; recover with recall_context]` reference
  (`internal/context/working_set.go:496-502`). The reference is the seam, not
  the proof, of recoverability.
- Loop policy bounds: `TranscriptRounds`
  (`internal/context/working_set.go:254-266`), `SectionCeiling`
  (`internal/context/working_set.go:271-283`), `RepeatThreshold`
  (`internal/context/working_set.go:291-303`), with the selection result
  `WorkingSelection` (`internal/context/working_set.go:136-141`).
- Revision is content identity: `Revision`
  (`internal/context/working_set.go:116-134`) invalidates views on
  uncommitted edits; `Entity` (`internal/context/working_set.go:107-113`)
  names the recording file; `Recall`
  (`internal/context/working_set.go:88-103`) pages an archived body from a
  character offset; `Search` (`internal/context/working_set.go:82-84`)
  delegates to the store.
- Cold storage is `WorkingStore`
  (`internal/context/working_store.go:47-50`), opened by `OpenWorkingStore`
  (`internal/context/working_store.go:52-84`) and closed by `Close`
  (`internal/context/working_store.go:86`): `WorkingRecord`
  (`internal/context/working_store.go:27-38`) with `workingDigest`
  (`internal/context/working_store.go:40-43`); `Save`
  (`internal/context/working_store.go:136-149`) refuses empty IDs
  (body-read `internal/context/working_store.go:140-142`) and fills the
  digest (body-read `internal/context/working_store.go:143-145`);
  `WorkingSearchHit` (`internal/context/working_store.go:92-102`) omits the
  body by design; `Search` (`internal/context/working_store.go:106-134`) is
  literal, paginated, and scope-local; `Candidates`
  (`internal/context/working_store.go:152-175`) rejects non-positive limits
  (body-read `internal/context/working_store.go:159-161`) and performs no
  body IO; `Records` (`internal/context/working_store.go:181-197`) is
  metadata-only; `Read` (`internal/context/working_store.go:219-239`) pages
  from an offset, reads to end on non-positive limit (body-read
  `internal/context/working_store.go:223-225`), and reports no-rows
  (body-read `internal/context/working_store.go:231-236`).

## 6. Serialization and the context block

- `FactSerializer` (`internal/context/serializer.go:23-32`) built by
  `NewFactSerializer` (`internal/context/serializer.go:35-41`), with
  `WithComments` (`internal/context/serializer.go:44-47`), `WithGrouping`
  (`internal/context/serializer.go:50-53`), `SetCorpusOrder`
  (`internal/context/serializer.go:57-60`), and
  `LoadSerializationOrderFromCorpus`
  (`internal/context/serializer.go:64-77`). Sort order resolves corpus then
  hardcoded via `getSortOrder` (`internal/context/serializer.go:81-88`).
- Flat and grouped paths are both length-bounded: `SerializeFacts`
  (`internal/context/serializer.go:91-100`), `serializeFlat`
  (`internal/context/serializer.go:109-116`), `renderFact`
  (`internal/context/serializer.go:119-125`), `serializeGrouped`
  (`internal/context/serializer.go:128-166`). Long arguments collapse via
  `truncateFact` (`internal/context/serializer.go:180-201`) under the
  47-character argument cap (body-read `internal/context/serializer.go:169`).
- Block shapes: `SerializeScoredFacts`
  (`internal/context/serializer.go:204-216`); `SerializeCompressedTurn`
  (`internal/context/serializer.go:219-248`); `SerializeCompressedContext`
  (`internal/context/serializer.go:251-298`) under `maxContextBlockChars`
  (`internal/context/serializer.go:313`, 64 KiB body-read at
  `internal/context/serializer.go:308-314`), where the budget is a report,
  not a gate (comment body-read at
  `internal/context/serializer.go:300-307`).
- Control-packet ingress: `ExtractAtomsFromControlPacket`
  (`internal/context/serializer.go:321-355`) is the packet front door;
  `ParseMangleAtom` (`internal/context/serializer.go:359-390`) parses one
  atom; `parseArgs` (`internal/context/serializer.go:393-413`), `splitArgs`
  (`internal/context/serializer.go:419-469`), `parseArgValue`
  (`internal/context/serializer.go:472-507`), `unescapeQuoted`
  (`internal/context/serializer.go:511-527`), and `formatArg`
  (`internal/context/serializer.go:530-549`) own the comma/quote/nesting and
  formatting rules; `fallbackPredicateOrder`
  (`internal/context/serializer.go:555-574`) and `predicateSortOrder`
  (`internal/context/serializer.go:582-587`) own the hardcoded order.
- State codecs validate JSON only: `MarshalCompressedState`
  (`internal/context/serializer.go:594-596`) and
  `UnmarshalCompressedState`
  (`internal/context/serializer.go:599-605`).
- Assembly order is a fixed positional contract: `ContextBlockBuilder`
  (`internal/context/serializer.go:612-615`) via `NewContextBlockBuilder`
  (`internal/context/serializer.go:618-623`) and `Build`
  (`internal/context/serializer.go:626-658`) renders core facts, scored
  atoms, history summary, then recent turns.

## 7. Token accounting (constraint, not policy)

- Counting is broker-ratio based: `TokenCounter`
  (`internal/context/tokens.go:38-40`) via `NewTokenCounter`
  (`internal/context/tokens.go:43-45`); `CountString`
  (`internal/context/tokens.go:48-53`); `Confidence`
  (`internal/context/tokens.go:58-60`); `Ratio`
  (`internal/context/tokens.go:63`); `CountFact`
  (`internal/context/tokens.go:66-98`) with its `MangleAtom` case
  (body-read `internal/context/tokens.go:83-87`); `CountFacts`
  (`internal/context/tokens.go:101-107`); `CountScoredFacts`
  (`internal/context/tokens.go:110-116`); `CountTurn`
  (`internal/context/tokens.go:119-144`); `CountTurns`
  (`internal/context/tokens.go:147-153`); `CountCompressedContext`
  (`internal/context/tokens.go:156-168`); `EstimateCompressionRatio`
  (`internal/context/tokens.go:439-449`). Overflow signals
  `ErrContextWindowExceeded` (`internal/context/tokens.go:175`).
- Budgeting is mutex-guarded: `TokenBudget`
  (`internal/context/tokens.go:184-200`) via `NewTokenBudget`
  (`internal/context/tokens.go:203-209`, hard enforcement defaults on);
  `Allocate` (`internal/context/tokens.go:228-280`) maps
  core-to-`CoreReserve`, atoms-to-`AtomReserve`,
  history-to-`HistoryReserve`, recent-to-nested-history, and
  working-to-`WorkingReserve` (body-read
  `internal/context/tokens.go:237-273`); `AllocateWithError`
  (`internal/context/tokens.go:284-290`); `CheckTotalBudget`
  (`internal/context/tokens.go:294-305`); `MustFitWithinBudget`
  (`internal/context/tokens.go:309-321`); `Release`
  (`internal/context/tokens.go:324-342`); `TotalUsed`
  (`internal/context/tokens.go:345-349`); `Available`
  (`internal/context/tokens.go:357-361`); `Utilization`
  (`internal/context/tokens.go:368-375`); `ShouldCompress`
  (`internal/context/tokens.go:378-393`); `GetUsage`
  (`internal/context/tokens.go:396-408`); `Reset`
  (`internal/context/tokens.go:411-419`); `SetUsage`
  (`internal/context/tokens.go:424-432`) is the only sanctioned absolute
  setter.

## 8. Feedback learning loop (third loop)

- Store: `ContextFeedbackStore`
  (`internal/context/feedback_store.go:25-38`) via
  `NewContextFeedbackStore` (`internal/context/feedback_store.go:63-83`)
  with a 10-sample minimum (body-read
  `internal/context/feedback_store.go:73`) and a 7-day half-life (body-read
  `internal/context/feedback_store.go:74`); schema in `initSchema`
  (`internal/context/feedback_store.go:86-115`); `Close`
  (`internal/context/feedback_store.go:471-476`); `MinSamples`
  (`internal/context/feedback_store.go:482-484`).
- Write path clamps and drops blanks: `StoreFeedback`
  (`internal/context/feedback_store.go:118-209`) clamps usefulness to
  [0,1] (body-read `internal/context/feedback_store.go:132-140`), drops
  empty predicate names (body-read
  `internal/context/feedback_store.go:143-144`), and invalidates the score
  cache (body-read `internal/context/feedback_store.go:201-203`).
- Read path: `GetPredicateUsefulness`
  (`internal/context/feedback_store.go:213-230`);
  `GetPredicateUsefulnessForIntent`
  (`internal/context/feedback_store.go:234-251`); `computePredicateScore`
  (`internal/context/feedback_store.go:255-341`) with decay (body-read
  `internal/context/feedback_store.go:310`), no-rows guard (body-read
  `internal/context/feedback_store.go:326-329`), and minimum-sample gate
  (body-read `internal/context/feedback_store.go:332-334`);
  `GetPredicateFeedback` (`internal/context/feedback_store.go:344-372`);
  `GetTopHelpfulPredicates` (`internal/context/feedback_store.go:375-400`);
  `GetTopNoisePredicates` (`internal/context/feedback_store.go:411-444`);
  `GetOverallStats` (`internal/context/feedback_store.go:447-457`);
  snapshot `FeedbackStats` (`internal/context/feedback_store.go:488-502`)
  via `CollectFeedbackStats`
  (`internal/context/feedback_store.go:506-541`). Feedback reaches scoring
  through `computeFeedbackScore`
  (`internal/context/activation_scoring.go:454-474`) after wiring through
  `SetFeedbackStore` (`internal/context/activation.go:266-270`).

## 9. Kernel interaction at the Go call sites

Shipped interaction only; predicate definitions live in core defaults and are
not redefined here. `09-MANGLE-SURFACE.md` pointer claims about
`schemas_context.mg` Decl lists and `context_compilation.mg` C1/C4/C3 rules
were not re-opened this turn and lose to this section where they differ.

- `BuildContext` (`internal/context/compressor.go:645-742`) queries
  `should_include_context` and prefers `buildKernelDerivedContext`
  (`internal/context/compressor_metrics.go:548-659`); Go scoring applies
  only on the empty/unresolved path.
- `Select` (`internal/context/working_set.go:308-512`) records
  `working_selected(ID,Priority)` and honours
  `should_include_context(Entity,Priority)` before budget rendering
  (body-read `internal/context/working_set.go:433-462`).
- Turn ages are asserted for the kernel by `assertTurnAgeCategories`
  (`internal/context/compressor_metrics.go:671-700`) and read back as
  `should_mask_observation` with the `should_preserve_reasoning` safety net
  in `maskedObservationTurns`
  (`internal/context/compressor_metrics.go:717-772`).
- No other kernel predicate is asserted or queried by this package beyond the
  sites above and the safety-predicate reads in `getCoreFacts`
  (`internal/context/compressor.go:745-771`).

## 10. What this spec does not claim

- Live-path wiring (which turn path production executes, which import edges
  hold, whether any `.mg` file lives under `internal/context/`, and whether
  `ProcessTurn` has non-test callers) is owned by
  `WIRING-AND-NOT-BUILT.md`, not by this file. This file asserts existence
  and in-package shape only.
- Vision, gap IDs, exit criteria, and capability specs are owned by
  `01-VISION.md`, `05-RELEVANCE-AND-RETRIEVAL.md`, and
  `06-RETENTION-EVICTION-ORDERING.md` (all `planned`). A `planned` claim that
  contradicts this file is a plan, not a description.
- There is no `02-CURRENT-STATE.md`, `03-GAP-ANALYSIS.md`,
  `IMPLEMENTED_SPEC.md` predecessor, or `05-INTERNAL-ARCHITECTURE.md` in this
  directory at this verification revision; links in `09-MANGLE-SURFACE.md`
  and `corpus.toml` naming those targets name files that do not exist and
  lose to this file until the targets are created.

## 11. Provenance (what was checked, what was carried)

- Function ranges: element-index verified this turn for every cited symbol.
- Inner lines without `[seam]`: body-read this turn under
  `task_aab9612b_2_0` (working set, activation head/tail, compressor tail,
  types head, serializer, tokens head, feedback head, working store).
- Inner lines marked `[seam]`: carried from the campaign seam map
  (`task_aab9612b_1_0`, body-verified in that task) for
  `activation_scoring.go`, `compressor_turns.go`, and
  `compressor_metrics.go` internals. Re-read before citing them at finer
  granularity than the function range.
- Unresolved at sign-off: `working_set.mg` existence (test comment at
  `working_repeat_threshold_test.go:15-29` names it; directory inventory
  claims no package-owned `.mg`); production import edges; `ProcessTurn`
  caller set. None of the three is asserted above.
