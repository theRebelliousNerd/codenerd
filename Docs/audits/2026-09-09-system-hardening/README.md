# System hardening — 2026-09-09

A pass across every subsystem asking one question per component: **is it built,
is it wired, is it called at the right time, and does what it puts in front of
the model earn its tokens?**

`go build ./...` and `go vet ./...` were already clean at the branch point. Not
one defect here is a compile defect.

## Read in this order

| Document | What it holds |
|---|---|
| [00-FINDINGS.md](00-FINDINGS.md) | Round 1: wiring, context, safety gates, the three budgets. |
| [01-CONTEXT-PIPELINE.md](01-CONTEXT-PIPELINE.md) | What reaches the model on a file-targeted turn, and why each section earns its place. |
| [02-PROMPT-BUDGET.md](02-PROMPT-BUDGET.md) | Every source of text that can reach the prompt, with its cap. |
| [03-STUBS-AND-SWALLOWED-ERRORS.md](03-STUBS-AND-SWALLOWED-ERRORS.md) | Live stubs reporting success they did not earn, and failures that were invisible. |
| [05-OPEN-ITEMS.md](05-OPEN-ITEMS.md) | Found, verified, deliberately not fixed — with the reason for each. |

## The deepest one

`core.Kernel.Assert` returned `nil` for a fact the kernel had **thrown away**.

`addFactIfNewLocked` answered one bool for two opposite outcomes — "already
present" (a no-op, fine) and "rejected, and never will be present" — and
`Assert` read it as the former. So a rejected fact reported success to its
caller and simply was not there afterwards, and **every**
`if err := kernel.Assert(f); err != nil` guard in the tree was ornamental,
including the ones added earlier in this same pass.

Confirmed on the pre-fix code:

```
Assert(dream_preference("likes tabs", 0.85))  ->  err = <nil>, rows after = 0
```

Not a corner case. `coerceAtomToDeclLocked` *has* to refuse a fractional float
in a `/number` slot, because this Mangle fork compares int64 only and one such
fact aborts the whole fixpoint. `DreamRouter` asserted exactly that shape for
all three Dream State learning predicates, then set `Success: true` and marked
the learning `Persisted` so it was never retried. Three predicates with a
`Decl`, a Go producer, and no rows, since the day they were added.

This is why round 2 had to happen before its own targets could be fixed: you
cannot handle an error that is never returned.

## The one pattern

Almost every defect was the same shape, in two languages:

> **Something exists, is tested in isolation, and never runs.**
>
> Both halves look correct on their own. Nothing fails. No test goes red.

| | Mangle | Go |
|---|---|---|
| The defect | a predicate declared and read by rules, produced by nothing | a function defined and exported, called by nothing |
| Found | `modified_function`, `user_{accepted,rejected}_finding`, `atom_selector` | `BuildWithImpactPriorities`, `RegisterTestImpactProvider`, `IsSystemShardsEnabled`, `RegisterFileValidators`, … |
| Now gated by | `TestStarvedPredicateBudget` | `scripts/deadcode-budget.sh` |
| Baseline | 65 | 869 |

Neither baseline is a target of zero. Both fail on drift **in either
direction** — new entries, and entries that got wired without leaving the list —
so the numbers stay measurements instead of rotting into files nobody trusts.

**The gates are the durable part of this work.** The individual fixes close
today's instances; the gates are what stop tomorrow's from being invisible.

## The headline defect

The holographic "impact-prioritized callers" feature was built end to end and
executed **zero times** in production. Three links were cut at once:

1. Nothing produced `modified_function`. Its only Go references were an
   allow-list entry letting *the model* volunteer the fact, and a shard's
   owned-predicate list. The kernel — the executive, in a design whose stated
   premise is that logic determines reality and the model merely describes it —
   was waiting to be told which function had changed about an edit it had just
   performed itself.
2. `BuildWithImpactPriorities` had no production caller: 730 lines, test-only.
3. So `PromptSection`'s prioritized branch was unreachable, and every turn fell
   through to an unordered list of caller names.

And a fourth, found only by an end-to-end test after the first three were fixed:
the producer emitted a bare `Target` where the call graph holds
`impactdemo.Target`. Every fact correct, the join silently empty. No unit test
could catch it — each side was internally consistent.

## Round 2: success that was not earned

| Defect | What it reported | What happened |
|---|---|---|
| `Kernel.Assert` | success | fact rejected and dropped |
| `swebench_evaluate` | `Success: true`, environment `/evaluating` | no tests run, no verdict fact produced, ever |
| `delete_lines` validator | `Verified: true` | branch unreachable (`.(int)` against JSON `float64`), and vacuous when reached |
| `ValidateProgram` | validated | real analysis computed and discarded; a line split stood in |
| `matchSpecialistsForReview` | a review | always empty; and the formatter dropped Files and Knowledge anyway |
| Shadow Mode effect asserts | `IsSafe` | the evidence against the action was lost, so nothing objected |
| `DreamRouter` learnings | `Persisted` | rejected by the kernel, never retried |

## Measured

| | Before | After |
|---|---|---|
| `PromptSection` on `internal/core`, per LLM turn | 49.4 ms · 12.6 MB · 296,445 allocs | **1.3 ms · 426 KB · 1,163 allocs** |
| Impact-prioritized callers in the prompt | branch unreachable | derived on every CodeDOM edit |
| Direct-caller priority as rendered | `MINIMAL` | `CRITICAL` |
| `run_impacted_tests` / `get_impacted_tests` | error on every call | real results |
| Thunderdome gate | always passed | passes only on a surviving battle |
| Shadow-kernel safety queries | fail open | fail closed, reason named |
| CodeDOM predicates retracted per scope change | 20 of 52 | 52 of 52 |
| Signature slots spent on the target's own package | 0 for a marker file | ranked, 3 tiers + centrality |

## Verification

Every fix carries a regression test that names the defect it pins. Two are
whole-system:

- `TestImpactChain_EndToEndThroughVirtualStore` — a real `edit_element` through
  a real kernel, asserting the fact, `impact_caller`, `impact_graph`,
  `context_priority_file`, and the rendered section.
- `TestCodeDOMScopePredicates_CoverEveryEmittedPredicate` — parses fixtures
  through the real parser factory and fails if any emitter lacks a retraction.

`go build ./...`, `go vet ./...`, `go test ./...`, and the repo's own
`audit_test_bodies` gate all pass.
