---
doc-class: shipped
subsystem: context
implementation-status: shipped
last-verified: 2026-09-21
verified-against: working-tree 2026-09-21 (commit hash unavailable from tool surface)
supersedes: []
---

# 02-CURRENT-STATE — internal/context shipped behavior

File-by-file record of what `internal/context` does today. Every code claim
cites a repo-relative path plus symbol plus line span. Line spans come from
the workspace symbol index re-read 2026-09-21, cross-checked against the
Phase 2 current-truth re-verification
(`.nerd/campaigns/aab9612b/artifacts/task_aab9612b_2_0.md`).

Scope: the 11 non-test `.go` source files under `internal/context/`.
Test files (`*_test.go`) are excluded from shipped-behavior claims.
This file makes no wiring claims (who calls what) and no `.mg` claims:
the `working_set.mg` existence contradiction and the `ProcessTurn`
zero-non-test-callers hypothesis from Phase 2 §10 are unresolved and
therefore not stated here.

## activation.go — activation engine shell

- Engine and context state live in `internal/context/activation.go:31-85`
  (`ActivationEngine`), `internal/context/activation.go:88-95`
  (`CampaignActivationContext`), `internal/context/activation.go:107-143`
  (`IssueActivationContext`), and `internal/context/activation.go:151-176`
  (`BackReferenceActivationContext`).
- Construction is `internal/context/activation.go:179-192`
  (`NewActivationEngine`); context setters are
  `internal/context/activation.go:195-199` (`SetCampaignContext`),
  `internal/context/activation.go:202-206` (`ClearCampaignContext`),
  `internal/context/activation.go:210-214` (`SetIssueContext`),
  `internal/context/activation.go:217-221` (`ClearIssueContext`),
  `internal/context/activation.go:225-229` (`SetBackReferenceContext`),
  and `internal/context/activation.go:232-236` (`ClearBackReferenceContext`).
- Corpus priorities enter through `internal/context/activation.go:241-245`
  (`SetCorpusPriorities`) and `internal/context/activation.go:249-261`
  (`LoadPrioritiesFromCorpus`); the feedback store is wired by
  `internal/context/activation.go:266-270` (`SetFeedbackStore`).
- Scoring entry points are `internal/context/activation.go:278-285`
  (`ScoreFacts`) and `internal/context/activation.go:289-347`
  (`scoreFactsLocked`); symbol relationships come from
  `internal/context/activation.go:356-393` (`buildSymbolGraphLocked`,
  consumes `dependency_link` and `symbol_graph` facts).
- Selection is `internal/context/activation.go:415-432`
  (`FilterByThreshold`, keeps `Score >= threshold`),
  `internal/context/activation.go:437-458` (`SelectWithinBudget`,
  threshold-first), and `internal/context/activation.go:469-483`
  (`SelectWithinBudgetPreFiltered`, no threshold — the path the
  kernel-override selection uses so a shipped threshold of 105 does not
  prune kernel priorities at or below 100).
- Focus tracking is `internal/context/activation.go:486-490`
  (`UpdateFocusedPaths`) with the locked implementation at
  `internal/context/activation.go:493-507`
  (`updateFocusedPathsLocked`); fact identity for recency/dependency maps
  is `internal/context/activation.go:533-535` (`factKey`, `Fact.String()`).
- Ordering and priority helpers are `internal/context/activation.go:539-546`
  (`sortScoredFactsDesc`), `internal/context/activation.go:549-555`
  (`extractPredicate`), and `internal/context/activation.go:559-571`
  (`lookupPriority`, corpus then config then default 50).
- Intent and spread entry points are
  `internal/context/activation.go:579-591` (`ApplyIntentActivation`),
  `internal/context/activation.go:594-606` (`GetHighActivationFacts`,
  apply plus select), and `internal/context/activation.go:610-671`
  (`SpreadFromSeeds`, spread factor `0.5*0.7^d`).
- State copy semantics are `internal/context/activation.go:683-687`
  (`GetState`), `internal/context/activation.go:691-695` (`SetState`),
  and `internal/context/activation.go:699-717`
  (`cloneActivationState`, deep-copies slices and active intent).
- Session lifecycle is `internal/context/activation.go:720-734`
  (`ClearState`), `internal/context/activation.go:737-747`
  (`MarkNewFacts`), `internal/context/activation.go:751-772`
  (`DecayRecency`), `internal/context/activation.go:775-781`
  (`NewSession`), and `internal/context/activation.go:784-797`
  (`GetSessionStats`).

## activation_scoring.go — nine-component heuristic scorer

- The component vector is `internal/context/activation_scoring.go:17-27`
  (`scoreComponents`: base, recency, relevance, dependency, campaign,
  session, issue, feedback, backReference) summed by
  `internal/context/activation_scoring.go:29-31` (`Total`).
- Aggregation is `internal/context/activation_scoring.go:39-51`
  (`computeScore`); base priority is
  `internal/context/activation_scoring.go:59-71` (`computeBaseScore`:
  corpus map, then config `PredicatePriorities`, then default 50.0).
- Recency bias is `internal/context/activation_scoring.go:75-99`
  (`computeRecencyScore`, under-1-minute +50, under-5-minute +30,
  under-30-minute +10, keyed by `factKey`); intent relevance is
  `internal/context/activation_scoring.go:102-221`
  (`computeRelevanceScore`: intent-target substring +40, focused path +30,
  focused symbol +20, verb-by-predicate table).
- Graph and scope boosts are `internal/context/activation_scoring.go:225-264`
  (`computeDependencyScore`, 30 percent forward inherit, +5 per reverse
  dependent, cap 40), `internal/context/activation_scoring.go:268-330`
  (`computeCampaignScore`, cap 60),
  `internal/context/activation_scoring.go:333-342`
  (`computeSessionScore`, +15 on session facts),
  `internal/context/activation_scoring.go:347-449`
  (`computeIssueScore`, cap 100 with tier boosts),
  `internal/context/activation_scoring.go:454-474`
  (`computeFeedbackScore`, usefulness scaled by 20), and
  `internal/context/activation_scoring.go:479-559`
  (`computeBackReferenceScore`, cap 70: referenced turn +50, topic +30,
  file +20, symbol +25, error +35, strength multiplier).
- Fact-argument coercion (MangleAtom and MangleString normalization,
  int/int64/float64 and numeric-string drift, absent-versus-zero
  distinction) is `internal/context/activation_scoring.go:566-577`
  (`factArgAsString`), `internal/context/activation_scoring.go:584-590`
  (`factArgAsInt`), `internal/context/activation_scoring.go:595-600`
  (`factArgAt`), `internal/context/activation_scoring.go:606-622`
  (`factArgToInt`), `internal/context/activation_scoring.go:626-632`
  (`factStringAt`), and `internal/context/activation_scoring.go:636-642`
  (`factTurnIDAt`).
- The kernel-substitution seam is
  `internal/context/activation_scoring.go:653-698`
  (`ScoreFactsWithKernelOverride`): an empty kernel-score map falls back
  to `ScoreFacts`, a kernel hit takes precedence, a kernel miss keeps the
  Go component breakdown, and the result keeps the shared sort contract.

## compressor.go — compressor shell and context build

- The compressor type and constructors are
  `internal/context/compressor.go:29-64` (`Compressor`),
  `internal/context/compressor.go:67-92` (`NewCompressor`),
  `internal/context/compressor.go:563-584` (`NewCompressorWithConfig`),
  `internal/context/compressor.go:588-612` (`NewCompressorWithParams`),
  and `internal/context/compressor.go:615-628`
  (`newCompressorWithCompressorConfig`).
- Activation contexts refresh under lock via
  `internal/context/compressor.go:97-120`
  (`refreshActivationContextsLocked`),
  `internal/context/compressor.go:122-239`
  (`refreshCampaignContextLocked`),
  `internal/context/compressor.go:241-412`
  (`refreshIssueContextLocked`), and
  `internal/context/compressor.go:414-518`
  (`refreshBackReferenceContextLocked`).
- Session and learning wiring are `internal/context/compressor.go:522-529`
  (`SetSessionID`), `internal/context/compressor.go:532-539`
  (`GetSessionID`), `internal/context/compressor.go:543-550`
  (`SetFeedbackStore`), and `internal/context/compressor.go:555-560`
  (`GetFeedbackStats`).
- The recent-turn window clamp is `internal/context/compressor.go:638-640`
  (`recentWindow`, clamped at zero).
- The LLM-facing build is `internal/context/compressor.go:645-742`
  (`BuildContext`): kernel-first selection with a Go-fallback path, core
  facts prepended, block assembly through the context-block builder, and
  usage accounting after the build.
- Safety retention is `internal/context/compressor.go:745-771`
  (`getCoreFacts`): queries the `permitted`, `dangerous_action`,
  `admin_override`, `security_violation`, and `block_commit` predicates
  and warns-and-continues on query errors rather than silently emitting
  an empty safety block.
- The serialized-string entry point is
  `internal/context/compressor.go:774-784` (`GetContextString`).

## compressor_metrics.go — budgets, state, kernel-derived context, masking

- Budget and compression telemetry are
  `internal/context/compressor_metrics.go:22-49` (`GetMetrics`),
  `internal/context/compressor_metrics.go:52-60`
  (`GetCompressionRatio`),
  `internal/context/compressor_metrics.go:64-71`
  (`GetBudgetUtilization`),
  `internal/context/compressor_metrics.go:75-82` (`GetBudgetUsage`),
  `internal/context/compressor_metrics.go:94-98` (`RefreshBudget`),
  and `internal/context/compressor_metrics.go:104-121`
  (`IsCompressionActive`).
- Selection-path accounting is
  `internal/context/compressor_metrics.go:125-139`
  (`recordSelectionLocked`),
  `internal/context/compressor_metrics.go:145-149`
  (`GetSelectionStats`), and
  `internal/context/compressor_metrics.go:152-154`
  (`GetRecentTurnWindow`).
- Persisted-state handling is
  `internal/context/compressor_metrics.go:163-192`
  (`buildStateLocked`),
  `internal/context/compressor_metrics.go:196-203`
  (`cloneRollingSummary`),
  `internal/context/compressor_metrics.go:207-226`
  (`cloneCompressedTurns`),
  `internal/context/compressor_metrics.go:229-239` (`cloneFacts`),
  `internal/context/compressor_metrics.go:242-251` (`GetState`),
  `internal/context/compressor_metrics.go:254-317` (`LoadState`),
  `internal/context/compressor_metrics.go:320-336` (`Reset`),
  and `internal/context/compressor_metrics.go:339-349`
  (`countOriginalTokens`).
- Key-atom extraction for persistence is
  `internal/context/compressor_metrics.go:360-400` (`collectKeyAtoms`,
  bounded per-turn and overall counts with a dropped count returned for
  rendering).
- String truncation under a token budget is
  `internal/context/compressor_metrics.go:410-435` (`trimToTokens`) and
  `internal/context/compressor_metrics.go:438-456`
  (`trimBodyToTokens`); corpus priority loading is
  `internal/context/compressor_metrics.go:460-465`
  (`LoadPrioritiesFromCorpus`).
- Score inspection helpers are
  `internal/context/compressor_metrics.go:470-505`
  (`GetActivationScores`) and
  `internal/context/compressor_metrics.go:509-520`
  (`GetHighActivationFactKeys`).
- The kernel-derived context path is
  `internal/context/compressor_metrics.go:548-659`
  (`buildKernelDerivedContext`).
- Kernel-gated observation masking is
  `internal/context/compressor_metrics.go:671-700`
  (`assertTurnAgeCategories`, recent/mid/old/ancient categories asserted
  without a trailing period),
  `internal/context/compressor_metrics.go:704-706` (`turnMaskID`, the
  `turn_<n>` identifier Go and Mangle must agree on), and
  `internal/context/compressor_metrics.go:717-772`
  (`maskedObservationTurns`, mask query plus preserve-reasoning safety
  net with refuse-to-mask on drift).

## compressor_turns.go — turn loop, compression, history rendering

- Turn intake is `internal/context/compressor_turns.go:30-184`
  (`ProcessTurn`): batch assertion with per-atom fallback, new-fact
  marking, activation-context refresh, memory operations, turn shaping
  with intent/focus/result split, sliding window, budget recompute,
  budget-gated compression, pruning, and best-effort persistence.
- Long-term retention operations are
  `internal/context/compressor_turns.go:202-236`
  (`processMemoryOperation`): promote persists a preference fact, forget
  retracts, vector store delegates, and unimplemented operations are
  warn-dropped rather than silently ignored.
- Budget-driven triggers are
  `internal/context/compressor_turns.go:241-243` (`shouldCompress`,
  delegates to the token budget) and
  `internal/context/compressor_turns.go:247-294` (`recalcBudget`,
  recomputes core, atom, history, recent, and working usage).
- Compression is `internal/context/compressor_turns.go:297-401`
  (`compress`): window check, cutoff selection, bounded key-atom
  collection, age-category assertion, masked observation turns, masked
  summary generation, target-ratio enforcement, segmentation, rolling
  totals, summary-text rebuild, fresh-slice removal, and recency decay.
- Summarization variants are
  `internal/context/compressor_turns.go:404-442`
  (`generateSummary`, LLM-backed),
  `internal/context/compressor_turns.go:456-498`
  (`generateObservationMaskedSummary`, masking-aware segment summary),
  and `internal/context/compressor_turns.go:501-517`
  (`generateSimpleSummary`, non-LLM fallback).
- Result-atom bounding is `internal/context/compressor_turns.go:524`
  (`maxSummaryResultAtoms`) with the writer at
  `internal/context/compressor_turns.go:533-545`
  (`writeCappedResultAtoms`).
- History rendering is
  `internal/context/compressor_turns.go:556-605`
  (`rebuildRollingSummaryText`, history-reserve gate with oldest-half
  merge and single-segment floor trim),
  `internal/context/compressor_turns.go:611-671`
  (`mergeOldestSegments`, folds the oldest segments with a 64-atom cap),
  and `internal/context/compressor_turns.go:674-701`
  (`renderRollingSummaryText`, header plus turn range plus key atoms
  plus truncation notice).
- The window safety net is `internal/context/compressor_turns.go:715-736`
  (`pruneRecentTurns`, compresses overflow before pruning and warns on
  last-resort drops).

## feedback_store.go — third-loop learning store

- Shapes are `internal/context/feedback_store.go:25-38`
  (`ContextFeedbackStore`), `internal/context/feedback_store.go:41-49`
  (`StoredFeedback`), `internal/context/feedback_store.go:52-59`
  (`PredicateFeedback`), and `internal/context/feedback_store.go:488-502`
  (`FeedbackStats`).
- Lifecycle is `internal/context/feedback_store.go:63-83`
  (`NewContextFeedbackStore`, minimum samples and decay defaults),
  `internal/context/feedback_store.go:86-115` (`initSchema`), and
  `internal/context/feedback_store.go:471-476` (`Close`).
- Writes are `internal/context/feedback_store.go:118-209`
  (`StoreFeedback`, clamps usefulness into range, drops blank predicate
  names, invalidates the cached scores).
- Reads are `internal/context/feedback_store.go:213-230`
  (`GetPredicateUsefulness`),
  `internal/context/feedback_store.go:234-251`
  (`GetPredicateUsefulnessForIntent`),
  `internal/context/feedback_store.go:255-341`
  (`computePredicateScore`, decayed weighted score with a minimum-sample
  gate and a no-rows guard),
  `internal/context/feedback_store.go:344-372`
  (`GetPredicateFeedback`),
  `internal/context/feedback_store.go:375-408`
  (`GetTopHelpfulPredicates`),
  `internal/context/feedback_store.go:411-444`
  (`GetTopNoisePredicates`), and
  `internal/context/feedback_store.go:447-457` (`GetOverallStats`).
- Helpers are `internal/context/feedback_store.go:460-468`
  (`dropEmptyPredicates`),
  `internal/context/feedback_store.go:482-484` (`MinSamples`), and
  `internal/context/feedback_store.go:506-541`
  (`CollectFeedbackStats`, tolerates a nil or failing store).

## serializer.go — Mangle text in and out

- The serializer type and options are
  `internal/context/serializer.go:23-32` (`FactSerializer`),
  `internal/context/serializer.go:35-41` (`NewFactSerializer`),
  `internal/context/serializer.go:44-47` (`WithComments`),
  `internal/context/serializer.go:50-53` (`WithGrouping`),
  `internal/context/serializer.go:57-60` (`SetCorpusOrder`),
  `internal/context/serializer.go:64-77`
  (`LoadSerializationOrderFromCorpus`), and
  `internal/context/serializer.go:81-88` (`getSortOrder`, corpus order
  first, hardcoded fallback second).
- Fact rendering is `internal/context/serializer.go:91-100`
  (`SerializeFacts`), `internal/context/serializer.go:109-116`
  (`serializeFlat`, per-fact line bound on both flat and grouped paths),
  `internal/context/serializer.go:119-125` (`renderFact`, collapses
  over-long arguments), and `internal/context/serializer.go:128-166`
  (`serializeGrouped`).
- Display truncation is `internal/context/serializer.go:169`
  (`maxFactArgChars`, 47-character argument cap) with the implementation
  at `internal/context/serializer.go:180-201` (`truncateFact`,
  `ClampInline` marker).
- Context-block rendering is `internal/context/serializer.go:204-216`
  (`SerializeScoredFacts`, optional score annotations),
  `internal/context/serializer.go:219-248`
  (`SerializeCompressedTurn`),
  `internal/context/serializer.go:251-298`
  (`SerializeCompressedContext`), bounded by
  `internal/context/serializer.go:313` (`maxContextBlockChars`, 64 KiB,
  a report-not-gate bound).
- Control-packet intake is `internal/context/serializer.go:321-355`
  (`ExtractAtomsFromControlPacket`) and
  `internal/context/serializer.go:359-390` (`ParseMangleAtom`).
- Atom grammar internals are `internal/context/serializer.go:393-413`
  (`parseArgs`), `internal/context/serializer.go:419-469`
  (`splitArgs`, quote- and paren-aware),
  `internal/context/serializer.go:472-507` (`parseArgValue`),
  `internal/context/serializer.go:511-527` (`unescapeQuoted`), and
  `internal/context/serializer.go:530-549` (`formatArg`).
- Predicate ordering fallback is `internal/context/serializer.go:555-574`
  (`fallbackPredicateOrder`) with the lookup at
  `internal/context/serializer.go:582-587` (`predicateSortOrder`).
- State transport is `internal/context/serializer.go:594-596`
  (`MarshalCompressedState`, JSON) and
  `internal/context/serializer.go:599-605`
  (`UnmarshalCompressedState`, JSON with validation-only semantics).
- Block assembly is `internal/context/serializer.go:612-615`
  (`ContextBlockBuilder`), `internal/context/serializer.go:618-623`
  (`NewContextBlockBuilder`), and
  `internal/context/serializer.go:626-658` (`Build`, core facts plus
  scored context atoms plus history summary plus recent turns).

## tokens.go — counting and budget enforcement

- Counting is `internal/context/tokens.go:38-40` (`TokenCounter`),
  `internal/context/tokens.go:43-45` (`NewTokenCounter`),
  `internal/context/tokens.go:48-53` (`CountString`),
  `internal/context/tokens.go:58-60` (`Confidence`),
  `internal/context/tokens.go:63` (`Ratio`),
  `internal/context/tokens.go:66-98` (`CountFact`, with a dedicated
  MangleAtom case), `internal/context/tokens.go:101-107`
  (`CountFacts`), `internal/context/tokens.go:110-116`
  (`CountScoredFacts`), `internal/context/tokens.go:119-144`
  (`CountTurn`), `internal/context/tokens.go:147-153` (`CountTurns`),
  and `internal/context/tokens.go:156-168`
  (`CountCompressedContext`).
- The window-exceeded signal is `internal/context/tokens.go:175`
  (`ErrContextWindowExceeded`).
- The budget type is `internal/context/tokens.go:184-200`
  (`TokenBudget`, mutex-guarded per-category usage) with construction at
  `internal/context/tokens.go:203-209` (`NewTokenBudget`, hard
  enforcement on by default).
- Enforcement toggles are `internal/context/tokens.go:213-217`
  (`SetHardEnforcement`) and `internal/context/tokens.go:220-224`
  (`IsHardEnforcementEnabled`).
- Allocation and accounting are `internal/context/tokens.go:228-280`
  (`Allocate`, core to core reserve, atoms to atom reserve, history to
  history reserve, recent nested inside history, working to working
  reserve), `internal/context/tokens.go:284-290`
  (`AllocateWithError`), `internal/context/tokens.go:294-305`
  (`CheckTotalBudget`), `internal/context/tokens.go:309-321`
  (`MustFitWithinBudget`), `internal/context/tokens.go:324-342`
  (`Release`), `internal/context/tokens.go:345-349` (`TotalUsed`),
  `internal/context/tokens.go:352-354` (`totalUsedLocked`),
  `internal/context/tokens.go:357-361` (`Available`),
  `internal/context/tokens.go:368-375` (`Utilization`),
  `internal/context/tokens.go:378-393` (`ShouldCompress`),
  `internal/context/tokens.go:396-408` (`GetUsage`),
  `internal/context/tokens.go:411-419` (`Reset`), and
  `internal/context/tokens.go:424-432` (`SetUsage`, the sanctioned
  absolute setter, negative inputs clamped).
- The ratio estimator is `internal/context/tokens.go:439-449`
  (`EstimateCompressionRatio`).

## types.go — configuration and state shapes

- Configuration is `internal/context/types.go:19-40`
  (`CompressorConfig`), `internal/context/types.go:44-130`
  (`DefaultConfig`, activation threshold 105.0 with the base-plus-recency
  rationale), and `internal/context/types.go:135-147`
  (`NewConfigWithBudget`).
- Context shapes are `internal/context/types.go:155-175`
  (`CompressedContext`), `internal/context/types.go:179-198`
  (`CompressedTurn`, surface text removed),
  `internal/context/types.go:201-212` (`TokenUsage`),
  `internal/context/types.go:219-233` (`ScoredFact`),
  `internal/context/types.go:289-305` (`ActivationState`),
  `internal/context/types.go:312-326` (`Turn`),
  `internal/context/types.go:329-341` (`TurnResult`),
  `internal/context/types.go:348-378` (`HistorySegment`),
  `internal/context/types.go:381-402` (`RollingSummary`), and
  `internal/context/types.go:409-428` (`CompressedState`).
- Selection provenance is `internal/context/types.go:237`
  (`SelectionMode`) with `internal/context/types.go:241`
  (`SelectionKernel`), `internal/context/types.go:243`
  (`SelectionGoFallback`), and the reason constants at
  `internal/context/types.go:251-254`; the counters are
  `internal/context/types.go:260-276` (`SelectionStats`) with the rate
  helper at `internal/context/types.go:280-286`
  (`KernelInclusionRate`).

## working_set.go — live retrieval funnel and loop policy

- The task-private policy handle is `internal/context/working_set.go:25`
  (`workingSetPolicy`), `internal/context/working_set.go:27-29`
  (`WorkingWorld`), `internal/context/working_set.go:33-40`
  (`WorkingSet`), and `internal/context/working_set.go:42-78`
  (`NewWorkingSet`).
- Cold-storage delegation is `internal/context/working_set.go:80`
  (`Close`), `internal/context/working_set.go:81` (`Save`), and
  `internal/context/working_set.go:82-84` (`Search`).
- Recall and identity are `internal/context/working_set.go:88-103`
  (`Recall`, paged body reads from a character offset),
  `internal/context/working_set.go:107-113` (`Entity`, file name for an
  archived observation), and `internal/context/working_set.go:116-134`
  (`Revision`, content identity so uncommitted edits invalidate views).
- Loop-protocol shapes are `internal/context/working_set.go:136-141`
  (`WorkingSelection`), `internal/context/working_set.go:145-157`
  (`WorkingProgress`), and `internal/context/working_set.go:163-171`
  (`WorkingDecision`).
- Policy gates are `internal/context/working_set.go:175-243`
  (`Continue`), `internal/context/working_set.go:254-266`
  (`TranscriptRounds`), `internal/context/working_set.go:271-283`
  (`SectionCeiling`), and `internal/context/working_set.go:291-303`
  (`RepeatThreshold`).
- Round selection is `internal/context/working_set.go:308-512`
  (`Select`): control facts, bounded two-hop dependency traversal (64
  entities), per-entity revision, code-definition queries, bounded
  store candidates, recent observations as working memory, asserted
  observation/digest/span facts, replace-not-accumulate control facts,
  priority-ordered kernel inclusion decisions, kernel-derived context
  build under a character budget, and per-round eviction with
  transcript-skip, zero-priority omit, whole-body reads, and
  over-budget omit carrying a `recall_context` recovery reference.

## working_store.go — durable cold storage for observations

- Shapes and identity are `internal/context/working_store.go:27-38`
  (`WorkingRecord`), `internal/context/working_store.go:40-43`
  (`workingDigest`, hex-encoded SHA-256), and
  `internal/context/working_store.go:47-50` (`WorkingStore`, scope in
  every lookup).
- Lifecycle is `internal/context/working_store.go:52-84`
  (`OpenWorkingStore`) and `internal/context/working_store.go:86`
  (`Close`).
- Search returns handles, not bodies, at
  `internal/context/working_store.go:92-102`
  (`WorkingSearchHit`, metadata plus body size) with the implementation
  at `internal/context/working_store.go:106-134` (`Search`, literal and
  paginated, scope-local).
- Writes and metadata reads are
  `internal/context/working_store.go:136-149` (`Save`, refuses empty
  IDs and fills the digest),
  `internal/context/working_store.go:152-175` (`Candidates`, metadata
  for the current dependency slice, no body I/O, rejects non-positive
  limits), `internal/context/working_store.go:181-197` (`Records`,
  metadata for named observations, no body I/O), and
  `internal/context/working_store.go:200-211`
  (`scanWorkingMetadata`, the shared metadata column reader).
- Body recovery is `internal/context/working_store.go:219-239`
  (`Read`, page from a character offset, non-positive limit reads to
  end, explicit no-rows message).

## Deliberately not claimed here

- Whether `Compressor.ProcessTurn`
  (`internal/context/compressor_turns.go:30-184`) is on the live turn
  path or exists-but-uncalled: caller analysis is a wiring claim and
  belongs in `WIRING-AND-NOT-BUILT.md`, not here.
- Whether a `working_set.mg` policy file exists under
  `internal/context/`: the inventory contradiction is unresolved, so no
  shipped claim either way is made in this file.
- Import edges into `internal/session/` and `cmd/nerd/` callers: not
  re-verified this turn, so not repeated as shipped fact.
