---
doc-class: shipped-with-future
subsystem: context
implementation-status: partial
last-verified: 2026-09-21
verified-against: ea90cc63
supersedes: []
---

# 03 — Gap analysis: `internal/context` (shipped vs planned)

> This file answers question 3 only: the distance between what runs today
> and what the vision plans. Current-state cells are `shipped` claims and are
> cited like ones (repo-relative path plus symbol plus line, verified against
> the working tree via `02-CURRENT-STATE.md`, `05-RELEVANCE-AND-RETRIEVAL.md`,
> `06-RETENTION-EVICTION-ORDERING.md`, and `WIRING-AND-NOT-BUILT.md`, all
> re-verified 2026-09-21). Target-state cells point at the spec file that
> details them. For what runs today see `02-CURRENT-STATE.md` and
> `IMPLEMENTED_SPEC.md`. For the dream see `01-VISION.md`,
> `05-RELEVANCE-AND-RETRIEVAL.md`, and `06-RETENTION-EVICTION-ORDERING.md`.
> **On any disagreement with another file about shipped behaviour,
> `IMPLEMENTED_SPEC.md` wins.**

## 0. ID ownership (collision resolved here)

- `01-VISION.md:237-244` defines `GAP-CTX-01..06` (relevance, retention,
  eviction, retrieval, ordering, `note`) with one-line exits.
- `05-RELEVANCE-AND-RETRIEVAL.md:187-195` defines `GAP-CTX-01..03`
  (relevance, funnel, revision) with `T-REL`/`T-RET` exits.
- `06-RETENTION-EVICTION-ORDERING.md:294-305` defines `GAP-CTX-04..07`
  (retention, eviction-fold, recoverable+ordering, `note`) with
  `T-RTN`/`T-EVC`/`T-ORD` exits.
- **This file owns one numbering: `GAP-CTX-01..07` as defined in `05` §8 and
  `06` §8.** That numbering is the build queue because it carries precise
  proving tests. Mapping to the vision one-liners: VISION-01→GAP-CTX-01,
  VISION-02→GAP-CTX-04, VISION-03→GAP-CTX-05+GAP-CTX-06 (recoverability leg),
  VISION-04→GAP-CTX-02, VISION-05→GAP-CTX-06 (ordering leg),
  VISION-06→GAP-CTX-07. The `01-VISION.md:237-244` IDs are superseded by this
  table and are not a second queue.

## 1. Gap matrix

| Gap ID | Capability | Current state (shipped, cited) | Target state (planned, points at spec) | Severity | Phase | Blocking dependencies | Exit criteria (machine-checkable) |
|---|---|---|---|---|---|---|---|
| GAP-CTX-01 | Relevance derived | Nine-component Go sum `computeScore` (`internal/context/activation_scoring.go:39-51` over `scoreComponents`, `internal/context/activation_scoring.go:17-27`, summed by `Total`, `internal/context/activation_scoring.go:29-31`); Go-only entry `ScoreFacts` (`internal/context/activation.go:278-285`); override seam `ScoreFactsWithKernelOverride` (`internal/context/activation_scoring.go:653-698`) falls back to Go scoring on an empty kernel map | Kernel derives `context_score/2` + `should_include_context/2`; Go keeps `factKey` (`internal/context/activation.go:533-535`) + stable sort `sortScoredFactsDesc` (`internal/context/activation.go:539-546`). See `05-RELEVANCE-AND-RETRIEVAL.md:28-42` (finished behaviour), `:101-137` (shapes, Go/Mangle split, fallback rule) | High — core Go→kernel drift named in `01-VISION.md:42-44` | P1 (first derivation; ordering depends on it) | None inside package; predicate definitions live in core defaults (this package owns call sites only) | `T-REL-01` + `T-REL-02` pass (`05-RELEVANCE-AND-RETRIEVAL.md:178-179`): non-empty fixture map derives the expected order; a miss carries the Go component breakdown + `fallback` flag while hits keep kernel relative order |
| GAP-CTX-02 | Kernel-gated retrieval funnel | Live funnel `Select` (`internal/context/working_set.go:308-512`) with control facts, two-hop `dependency_link` traversal capped at 64 entities, `Candidates` at 256, and `should_include_context` consulted (`internal/context/working_set.go:455-462` via `buildKernelDerivedContext`, `internal/context/working_set.go:473`); threshold split `FilterByThreshold` (`internal/context/activation.go:415-432`) vs `SelectWithinBudgetPreFiltered` (`internal/context/activation.go:469-483`); far context past two hops / 64 entities just `continue`s with no overflow signal (`internal/context/working_set.go:325-359`, per `WIRING-AND-NOT-BUILT.md:77-79`) | Same funnel where `should_include_context/2` gates inclusion (not a consulted signal) and every omit carries a recoverable reference. See `05-RELEVANCE-AND-RETRIEVAL.md:32-40` (funnel contract, omission-with-reference) and `:122-137` (split) | High — the production retrieval path; silent far-context drop is documented loss | P1 (with 01; proves funnel before blend) | GAP-CTX-01 (inclusion needs derived scores); `WorkingStore.Read` (`internal/context/working_store.go:219-239`) + `RecallContextTool` (`internal/tools/core/context_recall.go:13-80`) for the recovery leg | `T-RET-01` + `T-RET-03` pass (`05-RELEVANCE-AND-RETRIEVAL.md:181,183`): `Select` on a seeded world returns gated entities within `charBudget`, every omitted body carries a reference, and the reference resolves through the recall path to the full body |
| GAP-CTX-03 | Revision-joined retrieval (stale must not survive) | `Revision` content identity (`internal/context/working_set.go:116-134`) + per-entity `working_revision` (`internal/context/working_set.go:361`) exist; no shipped claim that `Select` (`internal/context/working_set.go:308-512`) joins revision before inclusion | Retrieval joins `Revision` before `should_include_context`; a revision mismatch forces refetch, never silent reuse. See `05-RELEVANCE-AND-RETRIEVAL.md:159-162` (failure mode 6) and `:182` (`T-RET-02`) | High — North Star explicit: "stale evidence must not survive a source change" (`agents.md:17-24`) | P1 (freshness gate ships with funnel) | GAP-CTX-02 (same funnel) | `T-RET-02` passes (`05-RELEVANCE-AND-RETRIEVAL.md:182`): an entity whose `Revision` changed between seed and select is refetched, never served stale |
| GAP-CTX-04 | Retention policy derived (`must_retain/1`) | Retention is token math: `recalcBudget` (`internal/context/compressor_turns.go:247-294`) fills `AtomReserve` via `GetHighActivationFacts` (`internal/context/activation.go:594-606`) and records usage via `SetUsage` (`internal/context/tokens.go:424-432`); safety list hardcoded in `getCoreFacts` (`internal/context/compressor.go:745-771`; predicates `permitted, dangerous_action, admin_override, security_violation, block_commit` at `internal/context/compressor.go:759`) with Warn-and-continue (`internal/context/compressor.go:761-766`), never a silent empty safety block | `must_retain/1` derives the retained set; reserves stay kernel-gated while token math (`TokenBudget`, `internal/context/tokens.go:184-200`; `SetUsage`, `internal/context/tokens.go:424-432`) stays as the forcing constraint. See `06-RETENTION-EVICTION-ORDERING.md:34-37` (finished behaviour) and `:219-234` (split, fallback rule) | High — safety retention bypasses eviction; the hardcoded list is constitution-adjacent | P2 (after funnel; needs budget math frozen) | `TokenBudget` + `SetUsage` stay as forcing budgets (Go side of `06:219-224` split) | `T-RTN-01` + `T-RTN-02` pass (`06-RETENTION-EVICTION-ORDERING.md:284-285`): fixture with a failing safety-predicate query still Warns and keeps the remainder while a `must_retain/1` fixture derives exactly the retained set; `shouldCompress` (`internal/context/compressor_turns.go:241-243` over `ShouldCompress`, `internal/context/tokens.go:378-393`) fires on budget threshold only, with no turn-count trigger |
| GAP-CTX-05 | Kernel-gated eviction + fold accounting | Trigger purely budget: `shouldCompress` (`internal/context/compressor_turns.go:241-243`) returns `budget.ShouldCompress()`; fold `compress` (`internal/context/compressor_turns.go:297-401`) with `rebuildRollingSummaryText` (`internal/context/compressor_turns.go:556-605`), `mergeOldestSegments` (`internal/context/compressor_turns.go:611-671`) with a 64-atom cap, and `collectKeyAtoms` (`internal/context/compressor_metrics.go:360-400`); exemplar only: `assertTurnAgeCategories` (`internal/context/compressor_metrics.go:671-700`) + `maskedObservationTurns` (`internal/context/compressor_metrics.go:717-772`); last-resort net `pruneRecentTurns` (`internal/context/compressor_turns.go:715-736`, `2*window` bound, Warn-drop) | Freeze the `should_mask_observation/1` contract (`/old`+`/ancient` masked, `preserve` holds); add planned `should_evict/2` with reason; merges preserve turn coverage, masked counts, original tokens, and summed `DroppedAtoms`. See `06-RETENTION-EVICTION-ORDERING.md:38-43` (finished behaviour), `:148-163` (masking exemplar), `:236-254` (failure modes 2-4) | High — eviction correctness + silent-loss risk (merge caps, prune Warn-drop) | P2 (exemplar first, then generalize) | GAP-CTX-04 (retention decides what eviction may touch); `turnMaskID` Go/Mangle contract (`internal/context/compressor_metrics.go:704-706`) | `T-EVC-01` + `T-EVC-02` pass (`06-RETENTION-EVICTION-ORDERING.md:286-287`): fixture `/old`+`/ancient` turns masked exactly with `MaskedTurns` counted; `compress` + `mergeOldestSegments` preserves coverage, masked counts, original tokens, and summed `DroppedAtoms`, none reset to zero |
| GAP-CTX-06 | Recoverable eviction + derived ordering | Per-round eviction whole-unit with an unproven ref: over-budget bodies become `Omitted` with a `[body outside active budget; recover with recall_context]` reference (`internal/context/working_set.go:480-508`; the `recall_context` reference is the seam, not the proof); ordering Go-owned: `Build` fixed order core→atoms→history→recent (`internal/context/serializer.go:626-658` via `ContextBlockBuilder`, `internal/context/serializer.go:612-615`, and `NewContextBlockBuilder`, `internal/context/serializer.go:618-623`), `Select` sort priority-desc/Step-desc (`internal/context/working_set.go:446-451`), stable sort `sortScoredFactsDesc` (`internal/context/activation.go:539-546`) | Every evicted body resolves via recall; kernel derives `context_position/2` and Go keeps stable sort. Prompt-layer order (`internal/prompt/assembler.go`, `internal/prompt/resolver.go`) is an out-of-package neighbour, not covered here. See `06-RETENTION-EVICTION-ORDERING.md:44-52` (finished behaviour) and `:255-261` (failure modes 5-6) | High (recoverability — explicit North Star) / Medium (ordering — lost-in-the-middle avoidance) | P3 (last; depends on 01+02+05) | GAP-CTX-01 (scores), GAP-CTX-02 (funnel omit), GAP-CTX-05 (mask), recall path (`WorkingStore.Read`, `internal/context/working_store.go:219-239`) | `T-EVC-03` + `T-ORD-01` + `T-ORD-02` pass (`06-RETENTION-EVICTION-ORDERING.md:288-290`): every `working_set.go:480-508` omit resolves to the full body; a `context_position/2` fixture pins placement through the stable-sort contract and an input shuffle does not change the derived order; empty-kernel positions fall back to Go order with exactly one `heuristic_ordered` record |
| GAP-CTX-07 | `note` memory op closed | `processMemoryOperation` (`internal/context/compressor_turns.go:202-236`): `promote_to_long_term`, `forget`, `store_vector` shipped; `note` accepted-by-schema but Warn-dropped (`internal/context/compressor_turns.go:226-231`); unknown ops Warn-drop (`internal/context/compressor_turns.go:232-235`) | `note` persists and recalls, OR the schema narrows to the implemented operations. See `06-RETENTION-EVICTION-ORDERING.md:105-116` (seam) and `:262-265` (failure mode 7); mirrors `01-VISION.md:189-195` | Medium — accept-and-drop is a contract violation; low blast radius | P1 (leaf work, parallelizable) | None; the schema owner decides persist-vs-narrow | Round-trip test passes OR the boundary rejects `note` (`06-RETENTION-EVICTION-ORDERING.md:304-305`): `note` persists and recalls through the store, or packet validation rejects `note` |

## 2. Wiring qualifiers that gate the exits (from `WIRING-AND-NOT-BUILT.md`)

These are not separate gaps. They are conditions a gap cannot be marked
done until resolved:

- **Turn-path resolution (gates any turn-path exit):** `ProcessTurn`
  (`internal/context/compressor_turns.go:30-184`) is `exists-but-uncalled` —
  16-row caller index, all `*_test.go`, zero production callers
  (`WIRING-AND-NOT-BUILT.md:45-49`). `GetContextString`
  (`internal/context/compressor.go:774-784`) is test-only
  (`WIRING-AND-NOT-BUILT.md:50-52`). `Select`
  (`internal/context/working_set.go:308-512`) has no traced production driver
  (same-name `browser`/`prompt` hits are false positives; session-loop driver
  untraced, `WIRING-AND-NOT-BUILT.md:53-58`). `Continue`
  (`internal/context/working_set.go:175-243`) with `TranscriptRounds`
  (`internal/context/working_set.go:254-266`), `SectionCeiling`
  (`internal/context/working_set.go:271-283`), and `RepeatThreshold`
  (`internal/context/working_set.go:291-303`) is dormant
  (`WIRING-AND-NOT-BUILT.md:59-64`). Do not mark GAP-CTX-02/04/05 done until
  the live driver is traced.
- **`working_set.mg` load site (gates GAP-CTX-01/02 derivation claims):** the
  file exists (182 lines) under `internal/context/`, contradicting the old
  "no `.mg`" claim; whether the engine loads it in compilation scope is
  unverified (`WIRING-AND-NOT-BUILT.md:68-73`). The
  `working_selected`/`should_include_context` rules
  (`internal/context/working_set.go:433-462`) are decided where it loads
  (`WIRING-AND-NOT-BUILT.md:98-100`).
- **Assumed-not-done caveats carried into the exits:** kernel facts bypass
  budget selection (`buildKernelDerivedContext` "never filters or reorders",
  `internal/context/compressor_metrics.go:548-659`,
  `WIRING-AND-NOT-BUILT.md:80-83`); persistence best-effort with discarded
  errors (`internal/context/compressor_turns.go:168-181`,
  `WIRING-AND-NOT-BUILT.md:84-86`); `UnmarshalCompressedState` JSON-only, no
  version/migration (`internal/context/serializer.go:599-605`,
  `WIRING-AND-NOT-BUILT.md:87-89`); `Confidence()`/`Ratio()`
  (`internal/context/tokens.go:58-60,63`) uncalled in-package
  (`WIRING-AND-NOT-BUILT.md:90-92`); feedback half-wired (`SetFeedbackStore`,
  `internal/context/compressor.go:543-550`, plus `computeFeedbackScore`,
  `internal/context/activation_scoring.go:454-474`, end-to-end untraced,
  `WIRING-AND-NOT-BUILT.md:93-97`).

## 3. Build queue (phase order)

1. **P1 — derivation + funnel + leaf:** GAP-CTX-01 (relevance), GAP-CTX-02
   (funnel), GAP-CTX-03 (revision freshness), GAP-CTX-07 (`note`, parallel).
2. **P2 — retention then eviction:** GAP-CTX-04 (`must_retain/1`), then
   GAP-CTX-05 (mask freeze + `should_evict/2` + fold accounting).
3. **P3 — recoverability + ordering:** GAP-CTX-06 (round-trip proof +
   `context_position/2`).

A closed gap stays in the §1 table, marked closed with the commit that closed
it; it is never deleted. R7 recursion picks its next item from this table.
