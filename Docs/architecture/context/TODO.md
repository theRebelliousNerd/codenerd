---
doc-class: governance
subsystem: context
implementation-status: not-applicable
last-verified: 2026-09-21
verified-against: 456e521
supersedes: []
---

# TODO — build queue: `internal/context` (leaf work only)

> Governance for `internal/context`. This file answers one question only:
> what to build next, as leaf tasks. It makes no `shipped` claim of its
> own; every current-state cell is a `shipped` claim cited like one
> (repo-relative path plus symbol plus line, verified against the working
> tree via `02-CURRENT-STATE.md`, `IMPLEMENTED_SPEC.md:1-526`, and
> `WIRING-AND-NOT-BUILT.md:1-101`, all re-verified 2026-09-21 against
> `456e521`). Every target-state cell points at the spec that details it
> (`05-RELEVANCE-AND-RETRIEVAL.md:1-204`,
> `06-RETENTION-EVICTION-ORDERING.md:1-317`). Gap IDs are owned by
> `03-GAP-ANALYSIS.md:33-39` (`GAP-CTX-01..07`); the `01-VISION.md:237-244`
> numbering is SUPERSEDED (`03:26-39`) and is never used here.
> **On any disagreement about shipped behaviour, `IMPLEMENTED_SPEC.md` wins**
> (`IMPLEMENTED_SPEC.md:15-19`, `03:21-22`).
> Phase order is `03:94-101`. Severities and phases are plan input, not
> shipped fact (`00-INDEX.md:41`).

Rules for this queue (per `Docs/journeys/09-architecture-doc-standard.md:63`):

- Leaf work only: each item is one checkable unit — one proving test
  (`T-REL`/`T-RET`/`T-RTN`/`T-EVC`/`T-ORD`) or one wiring verification.
  No epics ("derive relevance"), no adjectives ("robust", "complete").
- Every item traces to exactly one `GAP-CTX-NN` exit in `03:43-51`.
  An item with no gap ID does not belong here.
- §0 items are not separate gaps. They are the wiring qualifiers from
  `03:53-92` (via `WIRING-AND-NOT-BUILT.md:45-100`) that gate crediting a
  gap exit. A gap is not done until its gating §0 item is done.
- A closed item stays in this file, marked closed with the commit that
  closed it; it is never deleted (mirrors `03:103-104`).

## 0. Wiring prerequisites (gate exits, not gaps)

| TODO | Gates gap exit(s) | Leaf task (one verification) | Done when |
|---|---|---|---|
| TODO-CTX-00A | GAP-CTX-01, GAP-CTX-02 | Resolve `working_set.mg` load site: the file exists (182 lines under `internal/context/`, contradicting the old "no `.mg`" claim) but whether the engine loads it in compilation scope is unverified (`03:73-79`, `WIRING-AND-NOT-BUILT.md:68-73`). Determine where `working_selected` / `should_include_context` (`internal/context/working_set.go:433-462`) are decided (`WIRING-AND-NOT-BUILT.md:98-100`). | Load site named with a citing test or code path; derivation exits for GAP-01/02 are not credited until then |
| TODO-CTX-00B | GAP-CTX-02, GAP-CTX-04, GAP-CTX-05 | Trace the live production driver for the turn path: `ProcessTurn` (`internal/context/compressor_turns.go:30-184`) is `exists-but-uncalled` (16-row caller index, all `*_test.go`, zero production callers, `03:58-61`); `GetContextString` (`internal/context/compressor.go:774-784`) is test-only (`03:61-63`); `Select` (`internal/context/working_set.go:308-512`) has no traced production driver (`03:63-66`); `Continue` (`internal/context/working_set.go:175-243`) with `TranscriptRounds` (`internal/context/working_set.go:254-266`), `SectionCeiling` (`internal/context/working_set.go:271-283`), `RepeatThreshold` (`internal/context/working_set.go:291-303`) is dormant (`03:66-71`). Name the live driver. | Live driver traced and cited; GAP-02/04/05 exits are not credited until then |

## 1. P1 — derivation + funnel + leaf (GAP-CTX-01, 02, 03, 07)

| TODO | Gap | Leaf task | Exit (machine-checkable) | Depends on |
|---|---|---|---|---|
| TODO-CTX-01A | GAP-CTX-01 | Kernel-precedence fixture: `ScoreFactsWithKernelOverride` (`internal/context/activation_scoring.go:653-698`) with a non-empty fixture map derives the fixture's expected order; Go-only `ScoreFacts` (`internal/context/activation.go:278-285`) on the same fixture may differ. Seam: nine-component Go sum `computeScore` (`internal/context/activation_scoring.go:39-51` over `scoreComponents`, `internal/context/activation_scoring.go:17-27`, summed by `Total`, `internal/context/activation_scoring.go:29-31`); Go keeps `factKey` (`internal/context/activation.go:533-535`) + stable sort `sortScoredFactsDesc` (`internal/context/activation.go:539-546`). Spec `05:28-42`, `05:101-137`. | `T-REL-01` passes (`05:178`) | TODO-CTX-00A (derivation decided where the `.mg` loads) |
| TODO-CTX-01B | GAP-CTX-01 | Miss-fallback fixture: kernel map missing one key → that fact carries the Go component breakdown + `fallback` flag; hit facts keep kernel relative order. Same seam as 01A. Spec `05:101-137`. | `T-REL-02` passes (`05:179`) | TODO-CTX-01A |
| TODO-CTX-01C | GAP-CTX-01 | Pre-filtered budget fixture: kernel-scored fixture including low-magnitude kernel priorities survives `SelectWithinBudgetPreFiltered` (`internal/context/activation.go:469-483`); documents why `FilterByThreshold` (`internal/context/activation.go:415-432`, contract `internal/context/activation.go:460-468`) is not used for kernel sets — shipped threshold 105 (`internal/context/types.go:59-63`) prunes kernel priorities ≤100. Spec `05:122-137`. NOTE: `03:45` lists GAP-01 exit as `T-REL-01` + `T-REL-02` only; this item covers the `05:180`-defined remainder of the same seam. | `T-REL-03` passes (`05:180`) | TODO-CTX-01A |
| TODO-CTX-02A | GAP-CTX-02 | Funnel-contract fixture: `Select` (`internal/context/working_set.go:308-512`) on a seeded world returns `should_include_context`-gated entities (`internal/context/working_set.go:455-462` via `buildKernelDerivedContext`, `internal/context/working_set.go:473`) within `charBudget`, with control facts (`internal/context/working_set.go:323-324`), two-hop `dependency_link` traversal capped at 64 entities (`internal/context/working_set.go:325-359`), `Candidates` at 256 (`internal/context/working_set.go:373-376`); every omitted body carries a recoverable reference. Silent far-context drop past two hops / 64 entities just `continue`s (`internal/context/working_set.go:325-359`, `WIRING-AND-NOT-BUILT.md:77-79`). Spec `05:32-40`, `05:122-137`. | `T-RET-01` passes (`05:181`) | GAP-CTX-01 (inclusion needs derived scores); TODO-CTX-00A, TODO-CTX-00B |
| TODO-CTX-02B | GAP-CTX-02 | Funnel recoverability round-trip: each omitted reference from TODO-CTX-02A resolves through the recall path (`WorkingStore.Read`, `internal/context/working_store.go:219-239`; `RecallContextTool`, `internal/tools/core/context_recall.go:13-80`) to the full body. Spec `05:183`. | `T-RET-03` passes (`05:183`) | TODO-CTX-02A |
| TODO-CTX-03A | GAP-CTX-03 | Revision-freshness fixture: entity whose `Revision` content identity (`internal/context/working_set.go:116-134`) changed between seed and select is refetched, never served stale; per-entity `working_revision` (`internal/context/working_set.go:361`) joined before `should_include_context`. No shipped claim that `Select` joins revision today (`03:47`). Spec `05:159-162`, `05:182`. | `T-RET-02` passes (`05:182`) | TODO-CTX-02A (same funnel) |
| TODO-CTX-07A | GAP-CTX-07 | `note` closure: EITHER `note` persists and recalls through the store, OR packet validation rejects `note` at the boundary. Seam: `processMemoryOperation` (`internal/context/compressor_turns.go:202-236`) — `promote_to_long_term`, `forget`, `store_vector` shipped; `note` accepted-by-schema but Warn-dropped (`internal/context/compressor_turns.go:226-231`); unknown ops Warn-drop (`internal/context/compressor_turns.go:232-235`). Spec `06:105-116`, `06:262-265`, `06:304-305`. | Round-trip test passes OR boundary rejects `note` (`06:304-305`) | None; schema owner decides persist-vs-narrow. Parallelizable P1 leaf |

## 2. P2 — retention then eviction (GAP-CTX-04, then GAP-CTX-05)

| TODO | Gap | Leaf task | Exit (machine-checkable) | Depends on |
|---|---|---|---|---|
| TODO-CTX-04A | GAP-CTX-04 | Safety-retention fixture: `getCoreFacts` (`internal/context/compressor.go:745-771`) on a fixture with a failing safety-predicate query still Warns and keeps the remainder (`internal/context/compressor.go:761-766`); a `must_retain/1` fixture derives exactly the retained set, replacing the hardcoded list (`permitted, dangerous_action, admin_override, security_violation, block_commit` at `internal/context/compressor.go:759`). Token math (`TokenBudget`, `internal/context/tokens.go:184-200`; `SetUsage`, `internal/context/tokens.go:424-432`) stays as the forcing constraint. Spec `06:34-37`, `06:219-234`. | `T-RTN-01` passes (`06:284`) | Token math frozen; TODO-CTX-00B (turn-path driver) |
| TODO-CTX-04B | GAP-CTX-04 | Budget-trigger fixture: `recalcBudget` (`internal/context/compressor_turns.go:247-294`, fills `AtomReserve` via `GetHighActivationFacts`, `internal/context/activation.go:594-606`) + `shouldCompress` (`internal/context/compressor_turns.go:241-243` over `ShouldCompress`, `internal/context/tokens.go:378-393`) on a fixture crossing the threshold triggers `compress`; below threshold it does not — no turn-count trigger exists. Spec `06:284-285`. | `T-RTN-02` passes (`06:285`) | TODO-CTX-04A |
| TODO-CTX-05A | GAP-CTX-05 | Masking-exemplar fixture: kernel marks `/old`+`/ancient` fixture turns via `should_mask_observation` while `preserve` holds; `maskedObservationTurns` (`internal/context/compressor_metrics.go:717-772`) masks exactly those turns and `MaskedTurns` is counted in the segment. Exemplar seam: `assertTurnAgeCategories` (`internal/context/compressor_metrics.go:671-700`) + `turnMaskID` Go/Mangle contract (`internal/context/compressor_metrics.go:704-706`). Spec `06:38-43`, `06:148-163`. | `T-EVC-01` passes (`06:286`) | GAP-CTX-04 (retention decides what eviction may touch); TODO-CTX-00B |
| TODO-CTX-05B | GAP-CTX-05 | Fold-accounting fixture: `compress` (`internal/context/compressor_turns.go:297-401`) + `mergeOldestSegments` (`internal/context/compressor_turns.go:611-671`, 64-atom cap) with `rebuildRollingSummaryText` (`internal/context/compressor_turns.go:556-605`) and `collectKeyAtoms` (`internal/context/compressor_metrics.go:360-400`): merged segment preserves turn coverage, masked counts, original tokens, and summed `DroppedAtoms` — none reset to zero. Last-resort net `pruneRecentTurns` (`internal/context/compressor_turns.go:715-736`, `2*window` bound, Warn-drop) out of scope for this item. Spec `06:236-254`. | `T-EVC-02` passes (`06:287`) | TODO-CTX-05A (exemplar first, then generalize) |

## 3. P3 — recoverability + ordering (GAP-CTX-06, last)

| TODO | Gap | Leaf task | Exit (machine-checkable) | Depends on |
|---|---|---|---|---|
| TODO-CTX-06A | GAP-CTX-06 | Eviction round-trip: every body omitted at `internal/context/working_set.go:480-508` (over-budget bodies become `Omitted` with a `[body outside active budget; recover with recall_context]` reference, `internal/context/working_set.go:496-502` — the seam, not the proof) carries a `recall_context` reference, and the reference resolves through `WorkingStore.Read` (`internal/context/working_store.go:219-239`) to the full body. Spec `06:44-52`, `06:255-261`. | `T-EVC-03` passes (`06:288`) | GAP-CTX-01 (scores), GAP-CTX-02 (funnel omit), GAP-CTX-05 (mask) |
| TODO-CTX-06B | GAP-CTX-06 | Derived-ordering fixture: `context_position/2` fixture derives expected placement through the stable-sort contract (`sortScoredFactsDesc`, `internal/context/activation.go:539-546`; `Select` sort priority-desc/Step-desc, `internal/context/working_set.go:446-451`; fixed `Build` order core→atoms→history→recent via `ContextBlockBuilder`, `internal/context/serializer.go:612-615`, `NewContextBlockBuilder`, `internal/context/serializer.go:618-623`, `Build`, `internal/context/serializer.go:626-658`). Go-only order may differ — the test pins the kernel order. Prompt-layer order (`internal/prompt/assembler.go`, `internal/prompt/resolver.go`) is an out-of-package neighbour, not covered here. Spec `06:44-52`, `06:179-193`. | `T-ORD-01` passes (`06:289`) | GAP-CTX-01; TODO-CTX-06A |
| TODO-CTX-06C | GAP-CTX-06 | Heuristic-marking fixture: empty kernel positions → Go order applies and the turn carries exactly one `heuristic_ordered` record; no silent mixing of derived and heuristic order. Spec `06:275`, `06:290`. | `T-ORD-02` passes (`06:290`) | TODO-CTX-06B |

## 4. Closed log (stays here, never deleted)

_None closed yet. When a TODO passes its exit, mark it closed with the
commit hash; keep the row. The corresponding `03:43-51` gap row is marked
closed with the same commit, never deleted._
