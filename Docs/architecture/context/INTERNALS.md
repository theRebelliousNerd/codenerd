# context internals

> Verified 2026-09-20 against `456e521` (`main`).

One question: how do the two loops work?

## Turn loop

`Compressor.ProcessTurn` (`internal/context/compressor_turns.go:30`) runs
ten steps, all under one mutex (`compressor_turns.go:34-35`):

1. Extract atoms from the turn's control packet, plus any pre-extracted
   atoms (`compressor_turns.go:42-56`).
2. Commit to the kernel via `AssertBatch`, falling back to per-atom
   `Assert` so one bad fact does not block the rest
   (`compressor_turns.go:58-75`); mark them new for recency scoring.
3. Apply memory operations (`processMemoryOperation`,
   `compressor_turns.go:84-90,202`).
4. Build the compressed turn: intent atom, focus atoms, result atoms,
   mangle updates — and no surface text
   (`compressor_turns.go:94-116`).
5. Append to the sliding window; trigger compression past the budget
   threshold (`compressor_turns.go:119-145`).
6. Prune old turns (`pruneRecentTurns`, `compressor_turns.go:148-156`).
7. Persist state and activation analytics best-effort — both store errors
   are discarded (`compressor_turns.go:168-181`).

## Selection loop

`WorkingSet.Select` (`internal/context/working_set.go:297`) fills one
prompt section within `charBudget`:

1. Observations already in the transcript are excluded outright
   (`working_set.go:294-306`).
2. It asserts `user_intent` and `focus_resolution`, then walks dependency
   links two hops out, keeping at most 64 entities
   (`working_set.go:312-348`).
3. It pulls code definitions for those entities, fetches candidate
   observations (`store.Candidates`), and unions in recent observations
   from outside the slice so the focus file cannot evict working memory
   (`working_set.go:349-396`).
4. It replaces the engine's control facts and queries
   `working_selected(ID, Priority)` (`working_set.go:419-434`).
5. Kernel-derived facts take at most one eighth of the budget
   (`working_set.go:460-468`); observation bodies fill the rest in
   priority order, and anything over budget is reported omitted, never
   truncated (`working_set.go:469-497`).

Loop-control helpers on the same type — `Continue`, `TranscriptRounds`,
`SectionCeiling`, `RepeatThreshold`
(`internal/context/working_set.go:170-292`) — are not consulted by
`Select` itself.

## Scoring, counting, serializing

- Scoring: nine `compute*Score` methods summed by `computeScore`
  (`internal/context/activation_scoring.go:39-559`), constructed with
  `NewActivationEngine` (`internal/context/activation.go:179`).
- Counting: `TokenCounter` delegates text-to-token conversion to the
  broker and keeps only structural arithmetic locally; the old private
  chars-per-token constant is gone
  (`internal/context/tokens.go:17-53`). Budgets live in `TokenBudget`
  (`internal/context/tokens.go:184`), built by `NewTokenBudget`
  (`internal/context/tokens.go:203`).
- Serializing: `FactSerializer` groups by predicate, caps every fact at
  120 characters (`renderFact`/`truncateFact`,
  `internal/context/serializer.go:118-125,180`), prefers corpus order
  with a hardcoded fallback (`getSortOrder`,
  `internal/context/serializer.go:81-88`). State round-trips through
  `MarshalCompressedState` / `UnmarshalCompressedState`
  (`internal/context/serializer.go:594-605`); prompt assembly through
  `ContextBlockBuilder.Build`
  (`internal/context/serializer.go:612-658`).
- Feedback: `NewContextFeedbackStore`
  (`internal/context/feedback_store.go:63`) persists per-predicate
  usefulness (`StoreFeedback`, `GetPredicateUsefulness`,
  `GetTopHelpfulPredicates`/`GetTopNoisePredicates`,
  `internal/context/feedback_store.go:118-411`), gated by `MinSamples`
  (`internal/context/feedback_store.go:482`).
