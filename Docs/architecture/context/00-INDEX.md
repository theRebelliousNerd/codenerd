---
doc-class: governance
subsystem: context
implementation-status: not-applicable
last-verified: 2026-09-21
verified-against: ea90cc63
supersedes: []
---

# 00 — Index — Docs/architecture/context

_Governance map for this directory. Per `Docs/journeys/09-architecture-doc-standard.md:50-52`: read order, one line per file saying what it answers + when to read it, and a grounded-vs-hypothesized section._

- Last-verified: 2026-09-21 (except as noted per file).
- Verified-against: `ea90cc63`.
- ID ownership: `03-GAP-ANALYSIS.md:33-39` owns `GAP-CTX-01..07`; `01-VISION.md:237-244` numbering is SUPERSEDED (`03:26-39`).

## 1. Read order (one line per file)

| # | File | What it answers — when to read it |
|---|------|-----------------------------------|
| `README.md` | What this directory is and where the truth lives — read first |
| 2 | `00-INDEX.md` (this file) | In what order to read everything else and what is grounded vs hypothesized — read second; owns the order + §2 split. |
| 3 | `01-VISION.md` | What the five-verb dream wants (relevance, retention, eviction, retrieval, ordering) — read before shipped so aspiration is not mistaken for today; gap IDs at `01:237-244` are SUPERSEDED. |
| 4 | `02-CURRENT-STATE.md` + `IMPLEMENTED_SPEC.md` (as a pair) | What runs today, file-by-file, every claim cited — read together; when they disagree with anything else, IMPLEMENTED_SPEC wins. |
| 5 | `WIRING-AND-NOT-BUILT.md` | What is wired/reachable vs exists-but-uncalled vs assumed-not-done — read before specs so gap exits are gated correctly (`03:53-92`). |
| 6 | `05-RELEVANCE-AND-RETRIEVAL.md` | How relevance + retrieval should work (`T-REL-01..03`/`T-RET-01..03` at `05:174-183`) — read as the spec that owns `GAP-CTX-01..03` (`05:187-195`). |
| 7 | `06-RETENTION-EVICTION-ORDERING.md` | How retention/eviction/ordering should work (`T-RTN`/`T-EVC`/`T-ORD` at `06:282-290`) — read as the spec that owns `GAP-CTX-04..07` (`06:294-305`). |
| 8 | `03-GAP-ANALYSIS.md` | What to build next (`GAP-CTX-01..07` at `03:43-51`, phase order `03:94-101`) — read after both specs; it is the only ID owner. |
| 9 | `04-PRINCIPLES-AND-CONSTRAINTS.md` | What constraints (P1-P12) any change must respect — read before writing code. |
| 10 | `corpus.toml` | Predicate-priority data, not docs — `verified_on 2026-07-13` is stale vs 2026-09-21 docs; read as data input to `computeBaseScore`. |

## 2. Grounded vs hypothesized

### 2a. GROUNDED — element-verified + body-read (cite freely)

- All function ranges cited in `IMPLEMENTED_SPEC.md:61-80` + `02-CURRENT-STATE.md` file sections: verified via `package_outline internal/context` (410 rows) per `IMPLEMENTED_SPEC.md:26-27`. Includes `Select working_set.go:308-512`; `ActivationEngine activation.go:31-85/356-717` entries; `BuildContext compressor.go:645-742`; `getCoreFacts :745-771`; `types.go:19-175`; `serializer.go:23-390/594-658`; `tokens.go:38-280`; `feedback_store.go:25-400`; `working_store.go:27-239`; `compressor_turns.go:30/202/241/247/297/556/611/674/715-736`; `compressor_metrics.go:22/145/242/360/548/671/704/717-772`; `activation_scoring.go:17-698` ranges.
- Body-read inner lines (fresh, task_aab9612b_2_0 §§1-8): `Select` control facts `:323-324`, 64-cap check `:346`, `working_revision :361`, `code_defines/code_element :365-371`, `Candidates(256) :373-376`, `working_recent :377-385`, missing-recents `:390-407`, asserts `:408-420`, `ReplaceControlFacts :430` + rationale `:421-429`, sort `:446-451`, `should_include_context :455-462`, `charBudget/8 :471`, omit `:480-508`; `FilterByThreshold :415-432` vs `SelectWithinBudgetPreFiltered :469-483` + contract `:460-468`; `factKey :533-535`, `sortScoredFactsDesc :539-546`; `BuildContext` query `:688`, fallback `:690-712`, core facts `:719-720`, `Build :725-731`; safety list `:759` + Warn-continue `:752-765`; `GetContextString :774-784`; `DefaultConfig ActivationThreshold 105.0 :59-63`; `TokenCounter/Ratio/CountFact :38-98`; `TokenBudget/Allocate :184-280`; `StoreFeedback :118-209` + `computePredicateScore :255-341`; `WorkingStore Search/Save/Candidates/Read :106-239`; `ContextBlockBuilder :612-658` (Correction 2: `Build` body is `:626-658`).
- Shipped gaps current-state cells in `03:43-51` — each is a `shipped` claim cited to the above; target-state cells point at `05`/`06`.

### 2b. HYPOTHESIZED / UNVERIFIED — mark `[upstream]`/`[seam]`, do not cite as shipped

- Nine-scorer inner weights (`activation_scoring.go:59-559`: recency `<1m+50/<5m+30/<30m+10`, relevance verb×predicate table, dependency 30%/cap-40, campaign cap-60, issue cap-100 + tiers, feedback ×20, backref cap-70): ranges verified, weights carried as `[upstream: task_aab9612b_1_0]` per `05:57-60`, `02:87-109`. Highest-priority remaining read (task_2_0 §9).
- `compressor_turns.go` / `compressor_metrics.go` inner lines finer than function range (cutoff `:305-306`, keyAtoms64 `:309`, age assert `:318`, ratio `:336-354`, merge cap `:641-643`, mask queries `:729/:739-751`, refuse-on-drift `:762-766`): carried as `[seam]` per `IMPLEMENTED_SPEC.md:38-44`; `05:93-95` + `06:§2` explicitly mark them `[upstream]` until re-read.
- Wiring qualifiers gating exits (`03:53-92`, from `WIRING:45-100`): `ProcessTurn` exists-but-uncalled (16-row caller index, all `*_test.go`); `GetContextString` test-only; `Select` production driver untraced (browser/prompt hits are false positives); `Continue`/`TranscriptRounds`/`SectionCeiling`/`RepeatThreshold` dormant; `working_set.mg` load site unverified; kernel-facts-bypass-budget, best-effort persistence with discarded errors, `UnmarshalCompressedState` JSON-only, `Confidence()/Ratio()` uncalled in-package, feedback half-wired. None are separate gaps — all gate crediting `GAP-CTX-01/02/04/05` exits.
- Two pre-rewrite files (a 16-line Mangle-surface pointer and a working-set deep dive) were deleted under Rule 3 (`Docs/journeys/09-architecture-doc-standard.md:154-160`) after their surviving claims were absorbed into `04`, `02` and `IMPLEMENTED_SPEC.md`. What they got wrong is recorded where it matters: `Select` is at `internal/context/working_set.go:308-512` (`IMPLEMENTED_SPEC.md:45-48`), and `internal/context/working_set.mg` exists (`WIRING-AND-NOT-BUILT.md:68-73`). `corpus.toml` still says `verified_on 2026-07-13`. The shipped layer is written from code per Rule 4 (`Docs/journeys/09-architecture-doc-standard.md:162-170`).
- Severities/phases/dependencies in `03:43-51`: INFERRED (labeled as such in task_aab9612b_3_0); High on 01-06, Medium on 07 + ordering leg. Treat as plan input, not shipped fact.

## 3. How to cite this directory

- Shipped claims: `IMPLEMENTED_SPEC.md` + `02-CURRENT-STATE.md` ranges in §2a, or the code path+symbol+line directly (standard Rule 1/1a).
- Planned claims: `01`/`05`/`06` + `GAP-CTX-NN` in `03`, with proving test (`T-REL`/`T-RET`/`T-RTN`/`T-EVC`/`T-ORD`).
- Do not cite `README.md` inner lines or `[upstream]`/`[seam]` weights as shipped until re-read.
