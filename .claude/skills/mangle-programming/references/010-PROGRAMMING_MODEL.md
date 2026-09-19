# 010 — The programming model: seven patterns, distilled

Source of record: `Docs/journeys/M2-mangle-programming-model.md` (the design, with a runnable `.mg`
per pattern) and `Docs/journeys/M3-mangle-verification.md` (each pattern run against the pinned
engine, with a numbered refutation list). This file is the working summary: what each pattern is
for, its shape, and what verification changed. When the two disagree with this file, they win;
when they disagree with the code, the code wins.

## Conventions every pattern follows

1. One `Decl` per predicate, in the file that owns it, `bound [...]` on every column, before use.
   A second `Decl` of the same predicate anywhere in the loaded unit is a hard error (M3: the first
   draft's seven pattern files declared six predicates twice and refused to load together).
2. Identity columns are strings in the exact shape their producer emits; enumerations are `/name`
   atoms; counts, percentages and scores are int64 numbers (a float aborts the fixpoint).
3. Witnesses are named for what happened (`tool_ran`, `test_run`, `turn_gate`); judgements for the
   decision (`inject`, `test_obligation`, `turn_verified`); thresholds are `*_rounds/1` or
   `*_threshold/2` facts.
4. Anything mounted from a store is revision-keyed: the engine caches an external's answer for the
   whole evaluation, and stale evidence must not survive a source change.
5. Negate only a projection whose every argument is bound (see 020, item 1).
6. One `|> do fn:group_by` per rule; the zero case is a separate negation rule; no `_` in the body.
7. Selective, bound atoms first: bodies run as nested loops in written order.
8. "Must not fire twice" is a merge predicate or a `_superseded` projection, never Go
   retract-before-assert discipline.

## A. Mounting knowledge as virtual predicates

**For:** letting rules read SQLite, vector search, CodeDOM and history without loading them.
**Shape:** `Decl p(...) external() ...` with a Go callback (`internal/core/external_predicates.go`).
**Verified:** works when every input-mode argument is a literal constant in the rule text.
**Refuted on this engine:** joining a variable bound by an earlier atom into an external's input
panics (`engine/topdown.go:99`). Until that is fixed upstream, the working forms are (a) Go fills
the constant and calls `kernel.Query`, (b) Go asserts the small, relevant slice as EDB facts for
the turn and retracts it at turn close. An all-output mount that can be empty should return a
sentinel row, or the engine re-calls it (the cache engages only on a non-empty answer).
**Do not** bulk-load a store at boot: 307,455 knowledge-graph rows filled the EDB ceiling on
2026-09-18 and every turn ran with zero tools.

## B. Derived context injection

**For:** what enters the window, when, why, for which agent, in what order.
**Shape:** candidates (`injectable_context`, `prompt_atom`) x budget facts (`context_budget/2`) ->
`final_injectable`; ordering by a derived slot.
**Verified, with a cost:** the pairwise "above me" ranking is quadratic in candidates per agent:
0.46 s / 1.5 s / 5.8 s for 200 / 400 / 800 candidates. Scope candidates per agent first, or use a
per-priority-tier prefix sum (linear).
**Changed by M3:** `fn:collect` returns duplicates in unstable order; use `fn:collect_distinct`.
**Live (2026-09-18): needs are derived, atoms are served on the need.** `policy/jit_needs.mg`
declares `target_need(Language, Need)`; the executor queries it with the target's language bound
at every compile boundary (turn and planned step) and passes the result as
`CompilationContext.DerivedNeeds`; atoms declare `world_states: [<need>]` and the selector serves
them fail-closed. The per-step compile is the "when", the kernel's derivation the "what".
**Still open:** `final_injectable` has no Go consumer and `context_budget/2` no production
assertor (seam S12); the other world states are still Go booleans (the assembler's
`> 10 impacted files` and `> 20 unstaged` thresholds) and belong in `jit_needs.mg` as rules.

## C. Derived test obligations

**For:** the tests that harden a change exist because the harness derives that they must.
**Shape:** `changed_symbol` + `behavior_touched` -> `test_obligation(Task, Target, /write|/rerun)`.
**Changed by M3:** obligations must carry a task column (without it every task inherits every
obligation); "green now" needs a failure projection at the current revision (a pass and a fail at
one revision both looked green); adding the test turns `/write` into `/rerun`, it does not close
the obligation.
**Live slice:** `turn_untested(Verb, Path)` asserted by the executor, `turn_verified` withheld,
`turn_missing_evidence(Verb, /tests_not_written)` named (`policy/coder_safety.mg`).

## D. Completion as an obligation fixpoint

**For:** the agent stops when no obligation derives, not when it says it is done.
**Shape:** `pending_*` raised from evidence; discharged by the acting shard's `shard_result` for the
same description; `has_pending_subtask` is the open set; one retry per incomplete step.
**Changed by M3:** a lifetime failure count is not a stall detector (failures at rounds 1, 5 and 9
with progress between them still tripped it): window over the last K rounds.
**Live:** `policy/codedom_continuation.mg`; the working policy's stop, finalize and nudge rules in
`internal/context/working_set.mg` (a second engine).

## E. Blast-radius edits as derived plans

**For:** a change that touches many sites is planned by rules and carried out by a tool.
**Changed by M3:** a plan site must carry its change (ref + op): joining sites to changes on the
task alone attached a signature spec to rename sites when a task had two changes. Count sites over
a projection (`site_ref(T, Ref)`), not over rows that include a tier column.

## F. Provenance

**For:** "why was this injected / decided". The pinned engine ships `DerivationRecorder`,
`MemoryRecorder`, `BuildFromRecording` and the `mgwhy` tool; the corpus uses none of them yet.
**Changed by M3:** a rule's identity is the hash of the analysed, rewritten clause, not of the text
in the policy file (negations reordered, comparisons as `:ge(...)`), and a wildcard in a premise
drops that premise from the proof.

## G. Verdicts as facts

**For:** the turn's outcome is derived from gate evidence. **Live:** `turn_gate(Verb, Gate,
Verdict)` -> `turn_build_green|red`, `turn_tests_green|red` -> `turn_verified` -> `turn_done`;
`turn_unverified` names what is missing through `turn_missing_evidence/2`.
**Changed by M3:** a verification-only turn needs its own stage (a coder that only re-ran tests
was `/nothing_done`); claim rules need a shard-type guard (a reviewer's `/done` was always an
unverified claim).
**Learned in production (2026-09-18):** read THIS turn's gate, not a session-global
(`test_state/1` has four producers); close the turn on every path, including an errored one; and a
green gate is only green if the tests the turn wrote were compiled (build tags).

## Composition costs

Strata add: the seven toy patterns as one unit cost 52 strata, the exact sum. `current_rev/1`
asserted by several patterns with different values widens every revision join; make it one row
with `fundep`/`merge` or an invariant rule that reports the violation as a fact.
