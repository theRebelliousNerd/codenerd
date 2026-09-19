# 08 — The deductive-engineering thesis (2026-09-19): what it asks, what the code shows, where it goes

Steve shared an external thesis on 2026-09-19, *From Coding Agent to Deductive Engineering System*
(56 pages, written against `02e4c8dd`; his downloads, `codeNERD_Deductive_Engineering_Thesis.pdf`).
It stays where he put it. This file is the tracked record: its claims in one line each, what was
checked against the code here, and how each one is routed into the ladder (05), the hardening
program (06) and the audit queue (07).

It is an input to be judged, like the original bet. Nothing below is adopted because the thesis
says so; a row is adopted when the mechanism it names is found in the code.

## Status

- last updated: 2026-09-19 12:40
- verified here: Case A (below), the empty-selection testing obligation (= 07 N07), progress
  without run clocks (landed before the thesis arrived: `4316415f`, `02e4c8dd`, `f91c39b6`,
  `856ff1fe`).
- next: Case A as a hand-built slice after the 07 hand queue's F3/F5 -- it is small, it is the
  thesis's own recommended first cut, and it sits upstream of every CodeDOM-backed read.

## The thesis in one paragraph

The harness should stop being an agent that happens to use a kernel and become a compiler from
obligations to work: a demand compiler that knows what a task needs, acquires it with a
**completeness receipt** (what was asked, what was found, what failed, at which revision), packs it
into proof-directed context, and edits through a patch IR whose **semantic read set** says which
facts the patch depended on, so a later change to any of them invalidates the patch's evidence.
Memory is **dependency-certified**: a stored fact carries what it was derived from and dies with it.
Repair is **residual**: what the gates reject becomes the next obligation, not a retry of the whole.

## Claims, checked and routed

| # | Thesis idea | What the code shows (2026-09-19) | Route |
|---|---|---|---|
| T1 | Acquisition without a receipt is silent loss | **Case A, verified**: `HolographicCodeScope.ensureDeepFacts` (`internal/system/holographic_code_scope.go:111-177`) has six silent exits -- a failed deep scan logs Warn and returns; `RetractExactFactsBatch` and `LoadFacts` errors are discarded in both branches (`_ =`); a failed `os.Stat` or `MapFileAs` is `continue`; `Open`/`Refresh` return nil regardless. And in the store-less branch the cache records the new fingerprint even when `LoadFacts` failed, so the kernel stays without that file's facts until the file changes again | hand slice, after F3/F5: `ensureDeepFacts` returns a receipt (requested / loaded / failed per path, revision), `Open`/`Refresh` surface it, the cache advances only on a load that succeeded, and the receipt is asserted as facts policy can read |
| T2 | An empty selection must not satisfy a testing obligation | same mechanism as 07 **N07** (`run_impacted_tests` by name alone) | 07 hand queue (N07) |
| T3 | Patch IR with semantic read sets; evidence dies with its inputs | the gates certify the write set a turn declared, and evidence is keyed by verb, not execution (07 **F1**, **F3**, **F4**) | F1/F3/F4 are the first steps; a read set on the patch is R2+ design |
| T4 | Progress without arbitrary cutoffs | done before it arrived: every run-level clock is gone; progress stops are `working_stop` and the repair-attempt bound | landed (06 H1) |
| T5 | Dependency-certified memory | stale evidence surviving a source change is the open S24 seam (session-global `build_state`/`test_state`) and 07 **N11/N12** | R2 |
| T6 | Residual repair synthesis | the repair loop hands back the gate's findings, but its ledger error path drops completed work (06 finding) | 06 queue |
| T7 | Semantic capsules / proof-directed context packets | the JIT needs are kernel-derived (`jit_needs.mg`) but `final_injectable` has no consumer and `context_budget` no assertor (S12) | S12, then R3 |
| T8 | Semantic edit lenses over CodeDOM | 07 **N08** (the registered reader is per-line regex) and **N09** (first same-named element wins) must hold first | N09 brief now; N08 at R2 |

## Why Case A first

It is the thesis's recommended first vertical slice, it is one file, and it is upstream of the
world model the vision says replaces grepping: a CodeDOM read the kernel cannot distinguish from a
silent failure is a context delivery that lies. The receipt is also the smallest honest instance of
T1, T3 and T5 at once -- an acquisition that says what it got, at which revision, and what it did
not get.
