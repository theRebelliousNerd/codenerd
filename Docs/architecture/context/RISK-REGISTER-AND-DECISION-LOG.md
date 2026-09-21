---
doc-class: governance
subsystem: context
implementation-status: not-applicable
last-verified: 2026-09-21
verified-against: 456e521
supersedes: []
---

# Risk register and decision log — `internal/context`

> This file answers question 4 only for `internal/context`: why the package is
> shaped this way — what can go wrong, what retires each risk, and which
> decision (ADR) owns it. It makes no `shipped` claim of its own; every
> current-state cell is cited like one (repo-relative path plus symbol plus
> line, verified against the working tree via `02-CURRENT-STATE.md:1-496`,
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

- Each risk has an ID (`R-CTX-NN`), a likelihood, a consequence, and a
  retirement condition. Retirement is machine-checkable: a proving test
  passes, a predicate derives, or a gate reaches zero — never "improved".
- Each risk traces to one gap in `03-GAP-ANALYSIS.md:43-51`, one principle in
  `04-PRINCIPLES-AND-CONSTRAINTS.md`, and one decision in §3 below.
- Severities, phases, and likelihoods are plan input (inferred from the gap
  matrix and the wiring qualifiers in `03-GAP-ANALYSIS.md:53-92`), not shipped
  fact. Likelihood rationale names the code seam that makes the failure
  reachable today.
- A retired risk stays in the table, marked retired with the commit that
  retired it; it is never deleted (same rule as gaps,
  `Docs/journeys/09-architecture-doc-standard.md:76-77`).

## 1. Risk register (summary)

| Risk ID | Risk (one line) | Maps to | Likelihood / Consequence | Retires when | ADR pointer (§3) |
|---|---|---|---|---|---|
| R-CTX-01 | Silent far-context drop past 2 hops / 64 entities | `GAP-CTX-02`, P10 (`04:191-206`) | High / High | `T-RET-01` passes (`05:181`) + overflow signal added | ADR-002 (proposed, funnel-as-live-path) |
| R-CTX-02 | Stale evidence served as current (revision not joined) | `GAP-CTX-03`, P10 (`04:191-206`) | Medium / High | `T-RET-02` passes (`05:182`) | ADR-002 (proposed) |
| R-CTX-03 | Safety retention silently empty | `GAP-CTX-04`, P4 (`04:71-83`) | Low / Critical | `T-RTN-01` passes (`06:284`) + fail-closed empty-set witness | ADR-004 (proposed, safety-retention) |
| R-CTX-04 | Merge / prune silent loss (`DroppedAtoms` reset, Warn-drop) | `GAP-CTX-05`, P6 (`04:104-129`) | Medium / High | `T-EVC-01` + `T-EVC-02` pass (`06:286-287`) | ADR-004 (proposed, eviction-accounting leg) |
| R-CTX-05 | Evicted body unrecoverable (`recall_context` seam, not proof) | `GAP-CTX-06`, P7 (`04:131-147`) | Medium / High | `T-EVC-03` passes (`06:288`) | ADR-002 (proposed, recovery leg) |
| R-CTX-06 | Kernel / Go scale confusion (threshold 105 prunes kernel priorities) | `GAP-CTX-01`, P2-P3 (`04:36-69`) | High / Medium | `T-REL-01` + `T-REL-02` pass (`05:178-179`) + numbers-are-int64 guard | ADR-001 (proposed, kernel-precedence) |
| R-CTX-07 | Turn-path mis-credit (exits credited to uncalled / test-only / driverless path) | `03:53-92` wiring qualifier, P12 (`04:226-239`) | High / Medium | Live driver traced before GAP-CTX-02/04/05 marked done | ADR-002 (proposed, driver-trace witness) |
| R-CTX-08 | Derivation-rule orphan (`working_set.mg` exists, load site unverified) | `GAP-CTX-01/02`, P1 (`04:17-34`) | Medium / High | Load site verified + duplicate-Decl boot check | ADR-001 (proposed, derivation-scope leg) |
| R-CTX-09 | Accept-and-drop protocol (`note` + unknown ops Warn-dropped; best-effort persist) | `GAP-CTX-07`, P11 (`04:208-224`) | High / Low | Round-trip persists OR schema narrows (`06:304-305`) | ADR-003 (proposed, `note`-persist-vs-narrow) |

## 2. Risks in detail

### R-CTX-01 — Silent far-context drop

- **Description.** The live retrieval funnel silently skips context past two
  dependency hops or 64 entities. No overflow signal, no omit record, no
  recovery reference — the context is simply absent from the next window.
- **Evidence (shipped).** `WorkingSet.Select`
  (`internal/context/working_set.go:308`) walks `dependency_link` capped at 64
  entities (`internal/context/working_set.go:325-359`, cap check at
  `internal/context/working_set.go:346`) and just `continue`s past the bound
  with no signal (`WIRING-AND-NOT-BUILT.md:77-79`). Current-state owner:
  `03-GAP-ANALYSIS.md:46` (GAP-CTX-02 row).
- **Likelihood High.** This is the production retrieval path; every select
  over a world larger than 64 reachable entities exercises the `continue`.
- **Consequence High.** Lost context with no signal violates the funnel
  contract in `05-RELEVANCE-AND-RETRIEVAL.md:32-40` (omission-with-reference).
- **Retirement.** `T-RET-01` passes (`05-RELEVANCE-AND-RETRIEVAL.md:181`):
  `Select` (`internal/context/working_set.go:308`) on a seeded world returns
  gated entities within `charBudget`, every omitted body carries a reference,
  and the reference resolves through `WorkingStore.Read`
  (`internal/context/working_store.go:219`) via `RecallContextTool`
  (`internal/tools/core/context_recall.go:13`); plus an overflow signal (count
  or flag) so the drop is observable. Gated by the §2 wiring qualifier:
  do not credit until the live `Select` driver is traced (`03:58-72`).
- **ADR pointer.** ADR-002 (proposed) — the funnel decision must name the live
  driver as its witness before this risk can retire.

### R-CTX-02 — Stale evidence served as current

- **Description.** An entity whose source changed between seed and select can
  be served with its old body because retrieval does not join revision before
  inclusion.
- **Evidence (shipped).** `Revision` content identity exists
  (`internal/context/working_set.go:116` on `Revision`) and per-entity
  `working_revision` is asserted each round
  (`internal/context/working_set.go:361` inside `Select`,
  `internal/context/working_set.go:308`); no shipped claim joins revision
  before `should_include_context` (`internal/context/working_set.go:455`).
  North Star sentence served: `agents.md:17-24` ("stale evidence must not
  survive a source change as current truth"). Owner: `03:47` (GAP-CTX-03).
- **Likelihood Medium.** Requires a source change inside the select window;
  the join is missing on every select, so the window is always open.
- **Consequence High.** Stale-as-current is the explicit North Star failure.
- **Retirement.** `T-RET-02` passes (`05:182`): an entity whose `Revision`
  changed between seed and select is refetched, never served stale.
  Witnesses P10 (`04:191-206`) and P5 replace-not-accumulate
  (`internal/context/working_set.go:430` `ReplaceControlFacts`,
  `04:85-102`).
- **ADR pointer.** ADR-002 (proposed) — freshness ships with the funnel.

### R-CTX-03 — Safety retention silently empty

- **Description.** The constitution-adjacent safety block could build empty
  (or partial without notice) if the safety-predicate query fails and the
  Warn-and-continue net is removed, weakened, or bypassed by a derived
  retention path that forgets the fail-closed rule.
- **Evidence (shipped).** `Compressor.getCoreFacts`
  (`internal/context/compressor.go:745`) queries the hardcoded list
  (`internal/context/compressor.go:759`
  `permitted, dangerous_action, admin_override, security_violation,
  block_commit`) and on error warns and continues
  (`internal/context/compressor.go:761-765`), never silent
  (`04:71-83` P4). The list is hardcoded — the derivation
  `must_retain/1` does not exist yet (GAP-CTX-04, `03:48`).
- **Likelihood Low.** The Warn-continue net holds today; failure needs a query
  error plus a future refactor that drops the net.
- **Consequence Critical.** An empty safety block removes the constitutional
  core from the window.
- **Retirement.** `T-RTN-01` passes (`06:284`): a fixture with a failing
  safety-predicate query still Warns and keeps the remainder, and a
  `must_retain/1` fixture derives exactly the retained set; plus a
  fail-closed empty-set witness (a query that returns nothing still builds a
  loud, non-empty safety block or refuses the build).
- **ADR pointer.** ADR-004 (proposed, safety-retention) — freezes the
  fail-closed rule before `must_retain/1` replaces the hardcoded list.

### R-CTX-04 — Merge / prune silent loss

- **Description.** Folding history can lose atoms without accounting: the
  64-atom merge cap, the last-resort prune Warn-drop, or a totals recompute
  that resets `DroppedAtoms` to zero.
- **Evidence (shipped).** `Compressor.compress`
  (`internal/context/compressor_turns.go:297`), `Compressor.mergeOldestSegments`
  (`internal/context/compressor_turns.go:611`, 64-atom cap), and
  `Compressor.pruneRecentTurns` (`internal/context/compressor_turns.go:715`,
  `2*window` bound at `internal/context/compressor_turns.go:716`, warned drop
  at `internal/context/compressor_turns.go:728-734`); exemplar masking only
  via `Compressor.assertTurnAgeCategories`
  (`internal/context/compressor_metrics.go:671`) +
  `Compressor.maskedObservationTurns`
  (`internal/context/compressor_metrics.go:717`). Owner: `03:49` (GAP-CTX-05).
- **Likelihood Medium.** Requires budget pressure deep enough to fold and
  merge; the caps execute on every fold past the bound.
- **Consequence High.** Silent loss breaks the eviction-correctness promise
  (P6, `04:104-129`).
- **Retirement.** `T-EVC-01` + `T-EVC-02` pass (`06:286-287`): fixture
  `/old`+`/ancient` turns masked exactly with `MaskedTurns` counted, and
  `compress` + `mergeOldestSegments` preserves turn coverage, masked counts,
  original tokens, and summed `DroppedAtoms` — none reset to zero.
- **ADR pointer.** ADR-004 (proposed, eviction-accounting leg).

### R-CTX-05 — Evicted body unrecoverable

- **Description.** A per-round omit carries a `recall_context` reference, but
  the round-trip from reference back to full body is unproven — the seam
  exists, the proof does not.
- **Evidence (shipped).** Over-budget omits with a
  `[body outside active budget; recover with recall_context]` reference
  (`internal/context/working_set.go:496-502` inside `Select`,
  `internal/context/working_set.go:308`); whole-body read via
  `WorkingStore.Read` (`internal/context/working_store.go:219`); tool path
  via `RecallContextTool` (`internal/tools/core/context_recall.go:13`).
  The reference is the seam, not the proof (P7, `04:131-147`). Owner:
  `03:50` (GAP-CTX-06 recoverability leg).
- **Likelihood Medium.** Requires an over-budget round followed by a recall;
  the omit path executes, the resolve path is untraced end-to-end.
- **Consequence High.** Unrecoverable eviction is the explicit North Star
  failure (`agents.md:17-24`, tail).
- **Retirement.** `T-EVC-03` passes (`06:288`): every
  `internal/context/working_set.go:480-508` omit resolves to the full body
  through the recall path.
- **ADR pointer.** ADR-002 (proposed, recovery leg) — the funnel decision
  must include the round-trip witness.

### R-CTX-06 — Kernel / Go scale confusion

- **Description.** Kernel priorities (bounded roughly 60–100) routed through
  the Go threshold gate (default 105.0) are all pruned: the kernel speaks and
  the funnel hears nothing. A parallel drift risk is numeric: a float64 score
  fact entering the kernel aborts the fixpoint (project rule
  `numbers-are-int64`), while Go scores are float64 throughout.
- **Evidence (shipped).** `ActivationEngine.FilterByThreshold`
  (`internal/context/activation.go:415`, gate at
  `internal/context/activation.go:421`) vs
  `ActivationEngine.SelectWithinBudgetPreFiltered`
  (`internal/context/activation.go:469`, no gate, contract at
  `internal/context/activation.go:460-468`); shipped default
  `ActivationThreshold 105.0` (`internal/context/types.go:63` inside
  `DefaultConfig`, `internal/context/types.go:44`); override seam
  `ActivationEngine.ScoreFactsWithKernelOverride`
  (`internal/context/activation_scoring.go:653`, fallback at
  `internal/context/activation_scoring.go:654-656`). Owner: `03:45`
  (GAP-CTX-01).
- **Likelihood High.** The override seam is live (`BuildContext`,
  `internal/context/compressor.go:645`, fallback at
  `internal/context/compressor.go:707-712`); one wrong call-site routes
  kernel sets through the threshold.
- **Consequence Medium.** Total prune is loud (empty selection), but a partial
  prune silently reorders.
- **Retirement.** `T-REL-01` + `T-REL-02` pass (`05:178-179`) with kernel sets
  routed only through the pre-filtered path (`T-REL-03`, `05:180`), plus the
  numbers-are-int64 guard: no float64 score / eviction / ordering fact enters
  the kernel (project rule, `05:119-120`, `06:215-217`).
- **ADR pointer.** ADR-001 (proposed, kernel-precedence) — freezes
  hit-precedence, miss-breakdown, and the pre-filtered-path rule.

### R-CTX-07 — Turn-path mis-credit

- **Description.** Build credit goes to the wrong turn path: exits marked done
  against `ProcessTurn` (exists-but-uncalled), `GetContextString`
  (test-only), or `Select` (driver untraced) without tracing the path that
  actually runs in production.
- **Evidence (shipped).** `Compressor.ProcessTurn`
  (`internal/context/compressor_turns.go:30`) has zero production callers
  (16-row caller index, all `*_test.go`, `WIRING-AND-NOT-BUILT.md:45-49`);
  `Compressor.GetContextString` (`internal/context/compressor.go:774`) is
  test-only (`WIRING-AND-NOT-BUILT.md:50-52`); `WorkingSet.Select`
  (`internal/context/working_set.go:308`) has no traced production driver
  (`WIRING-AND-NOT-BUILT.md:53-58`); `WorkingSet.Continue`
  (`internal/context/working_set.go:175`) with `TranscriptRounds`
  (`internal/context/working_set.go:254`), `SectionCeiling`
  (`internal/context/working_set.go:271`), `RepeatThreshold`
  (`internal/context/working_set.go:291`) is dormant
  (`WIRING-AND-NOT-BUILT.md:59-64`). Gate owner: `03:53-92` §2.
- **Likelihood High.** Any campaign picking up GAP-CTX-02/04/05 without reading
  §2 hits this.
- **Consequence Medium.** Wasted build plus false-done: the gap closes on
  paper while the live path still drops, stalls, or serves stale.
- **Retirement.** The live `Select` / working-loop driver is traced
  (production caller named with symbol and line) before GAP-CTX-02/04/05 may
  be marked done. Standing open question until then (see §4).
- **ADR pointer.** ADR-002 (proposed) — its witness IS the driver trace.

### R-CTX-08 — Derivation-rule orphan

- **Description.** The Mangle rules behind `working_selected` /
  `should_include_context` live in `working_set.mg` (182 lines), but the load
  site — whether the engine compiles that file in scope — is unverified. The
  derivation can be orphaned: rules exist, nothing loads them, Go fallback
  silently decides everything.
- **Evidence (shipped).** `working_selected(ID, Priority)` query
  (`internal/context/working_set.go:433`) and
  `should_include_context(Entity, Priority)` decision
  (`internal/context/working_set.go:455`) via `buildKernelDerivedContext`
  (`internal/context/working_set.go:473`); the 182-line `working_set.mg`
  exists under `internal/context/`, contradicting the old "no `.mg`" claim;
  load scope unverified (`WIRING-AND-NOT-BUILT.md:68-73`,
  `WIRING-AND-NOT-BUILT.md:98-100`). Gates GAP-CTX-01/02 (`03:73-79`).
  Project rule at stake: every predicate needs one Decl per arity, never two
  (duplicate takes the kernel down at boot).
- **Likelihood Medium.** Existence is confirmed, loading is not; one scope
  misconfiguration orphans the rules.
- **Consequence High.** Silent fallback to Go-only scoring while the docs
  claim kernel derivation (P1, `04:17-34`).
- **Retirement.** Load site verified (file and line where the engine compiles
  `working_set.mg` in scope, named with symbol and line) plus a
  duplicate-Decl boot check proving one Decl per arity.
- **ADR pointer.** ADR-001 (proposed, derivation-scope leg).

### R-CTX-09 — Accept-and-drop protocol

- **Description.** The packet schema admits operations the switch does not
  implement (`note`), and the store persists best-effort with discarded
  errors — the caller is told "accepted" while the fact is dropped.
- **Evidence (shipped).** `Compressor.processMemoryOperation`
  (`internal/context/compressor_turns.go:202`): `note` accepted-by-schema but
  Warn-dropped (`internal/context/compressor_turns.go:226-231`), unknown ops
  Warn-drop (`internal/context/compressor_turns.go:232-235`); best-effort
  persist with discarded errors (`internal/context/compressor_turns.go:168-181`
  inside `ProcessTurn`, `internal/context/compressor_turns.go:30`).
  Owner: `03:51` (GAP-CTX-07). Principle: P11 (`04:208-224`).
- **Likelihood High.** Every `note` op exercises the Warn-drop today.
- **Consequence Low.** Contract violation with low blast radius (single memory
  op, warned not silent).
- **Retirement.** Either leg of `06:304-305`: `note` round-trips (persists and
  recalls through the store) OR the boundary rejects `note` (schema narrows
  to implemented ops). Parallel leaf, no dependencies.
- **ADR pointer.** ADR-003 (proposed, `note`-persist-vs-narrow).

## 3. Decision log (ADR pointers)

> No `adr/` directory exists under `Docs/architecture/context/` as of
> 2026-09-21 (verified by directory listing; required slot per
> `Docs/journeys/09-architecture-doc-standard.md:60`). Every entry below is
> therefore **proposed**, not accepted. Status is derived from its witness
> per `Docs/journeys/09-architecture-doc-standard.md:82-88`: if the witness
> does not resolve, the status is `accepted-not-implemented` (or
> `proposed` where no decision has been recorded). Creating the `adr/`
> files is the governance TODO; this log is the pointer layer until then.

| ADR (proposed) | Decision (one line) | Context | Witness (must resolve) | Status today | Retires |
|---|---|---|---|---|---|
| ADR-001 kernel-precedence | Kernel hit wins; miss falls back with Go breakdown; empty map → Go; kernel sets use the pre-filtered path only | Override seam `ActivationEngine.ScoreFactsWithKernelOverride` (`internal/context/activation_scoring.go:653`, P2 `04:36-52`, P3 `04:54-69`) | `T-REL-01` + `T-REL-02` (`05:178-179`) + `T-REL-03` (`05:180`); derivation scope: `working_set.mg` load site + one-Decl-per-arity boot check | `accepted-not-implemented` — seam exists, frozen semantics do not | R-CTX-06, R-CTX-08; unblocks GAP-CTX-01 |
| ADR-002 funnel-as-live-path | `WorkingSet.Select` (`internal/context/working_set.go:308`) is the live retrieval funnel (vs exists-but-uncalled `ProcessTurn`); every omit carries a recoverable ref; revision is joined before inclusion | Funnel `Select` (`internal/context/working_set.go:308-512`), revision (`internal/context/working_set.go:361`), omit-with-ref (`internal/context/working_set.go:496-502`), recall via `WorkingStore.Read` (`internal/context/working_store.go:219`) + `RecallContextTool` (`internal/tools/core/context_recall.go:13`); qualifiers `03:53-92` | Production-driver trace (file + symbol + line of the live `Select` caller); then `T-RET-01` (`05:181`), `T-RET-03` (`05:183`), `T-RET-02` (`05:182`), `T-EVC-03` (`06:288`) | `proposed` — cannot be Accepted until the driver is traced | R-CTX-01, R-CTX-02, R-CTX-05, R-CTX-07; unblocks GAP-CTX-02/03/06 credit |
| ADR-003 `note`-persist-vs-narrow | `note` persists and recalls through the store, OR the packet schema narrows to implemented ops | Accept-and-drop seam `Compressor.processMemoryOperation` (`internal/context/compressor_turns.go:202`, `note` at `internal/context/compressor_turns.go:226-231`), P11 (`04:208-224`) | Round-trip test OR boundary rejection (`06:304-305`; mirrors `01-VISION.md:189-195`) | `accepted-not-implemented` — `note` Warn-drops today | R-CTX-09; unblocks GAP-CTX-07 |
| ADR-004 safety-retention + eviction-accounting (proposed next) | Safety retention derives `must_retain/1` fail-closed; eviction folds preserve coverage / masked counts / original tokens / summed `DroppedAtoms` | Safety list `Compressor.getCoreFacts` (`internal/context/compressor.go:745`, list at `internal/context/compressor.go:759`), fold `Compressor.compress` (`internal/context/compressor_turns.go:297`) + `Compressor.mergeOldestSegments` (`internal/context/compressor_turns.go:611`) + `Compressor.pruneRecentTurns` (`internal/context/compressor_turns.go:715`), P4 (`04:71-83`), P6 (`04:104-129`) | `T-RTN-01` + `T-RTN-02` (`06:284-285`); `T-EVC-01` + `T-EVC-02` (`06:286-287`) | `proposed` — hardcoded list + Warn-continue is the only net today | R-CTX-03, R-CTX-04; unblocks GAP-CTX-04/05 |

## 4. Standing open questions (risk-adjacent, owned by `OPEN-QUESTIONS.md`)

- Live turn-path owner: `ProcessTurn`
  (`internal/context/compressor_turns.go:30`) vs the working loop feeding
  `Select` (`internal/context/working_set.go:308`) — gates R-CTX-07 and all
  turn-path credit (`03:58-72`).
- `working_set.mg` load scope — gates R-CTX-08 (`03:73-79`).
- `note` persist-vs-narrow owner (schema owner decides) — gates R-CTX-09
  (`03:51`, `06:304-305`).

## 5. Maintenance

- Re-verify against the working tree whenever any cited Go symbol moves; the
  authoritative shipped record is `IMPLEMENTED_SPEC.md:15-19`.
- Claims the pre-rewrite documents made that the code does not bear out, kept
  here so they are not reintroduced: that the package holds no `.mg` file
  (`internal/context/working_set.mg` exists, `WIRING-AND-NOT-BUILT.md:68-73`),
  and that `Select` sits at line 297 (it is
  `internal/context/working_set.go:308-512` per `IMPLEMENTED_SPEC.md:45-48`).
  `corpus.toml:1-8` `verified_on 2026-07-13` is stale vs 2026-09-21 docs.
