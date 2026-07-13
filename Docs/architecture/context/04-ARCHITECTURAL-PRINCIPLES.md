# 04 — Architectural Principles: Context

> Last verified against codebase: 2026-07-13  
> Binding principles specific to `internal/context`  
> Status: Living Reference Document

These principles govern design and review of changes in this package. They are **package-local** expressions of the codeNERD north star.

## P1 — Atoms are the memory; text is the UI

Surface chat is for humans. Long-horizon state is **Mangle facts**.  
`CompressedTurn` must not reintroduce surface response text into the durable window.

**Evidence:** `types.go` `CompressedTurn` comment; `ProcessTurn` builds atom-only turns.

## P2 — Select before packing

Never dump all facts into the atom reserve. Always **score → threshold → budget**.

**Evidence:** `FilterByThreshold` before selection; `SelectWithinBudget` re-applies threshold defensively.

## P3 — Core reserve is constitutional

Permission and safety predicates ship in the core section even when activation is cold.

**Evidence:** `getCoreFacts` queries `permitted`, `dangerous_action`, `admin_override`, `security_violation`, `block_commit`.

## P4 — Kernel may override heuristics; Go may not override kernel policy

When `should_include_context` yields usable priorities, prefer them. Go heuristics are the fallback executive for ranking, not a second constitution.

**Evidence:** `BuildContext` C1+C4 branch; `ScoreFactsWithKernelOverride`.

## P5 — Thresholds beat pure recency

A fact that is merely recent is not automatically relevant. Default threshold 105 requires relevance beyond base+recency.

**Evidence:** `DefaultConfig` comments on `ActivationThreshold`.

## P6 — Caps and clamps prevent score weapons

No single component may dominate the window via uncapped boosts.

**Evidence:** Dependency cap 40, campaign 60, issue 100, back-ref 70; keyword weight clamp [0,1].

## P7 — Budget is hard at the total limit

Soft reject per category; **error** when total budget already exceeded before build.

**Evidence:** `ErrContextWindowExceeded`, `CheckTotalBudget` in `BuildContext`.

## P8 — Compression is utilization-driven

Do not compress because “N turns elapsed.” Compress when `Utilization >= CompressionThreshold`.

**Evidence:** `shouldCompress` → `TokenBudget.ShouldCompress`.

## P9 — Context setters are derived from kernel truth when possible

Campaign/issue/back-ref contexts should refresh from QueryAll, not only manual chat hooks.

**Evidence:** `refreshActivationContextsLocked`.

## P10 — Concurrent readers/writers must not race maps

Activation maps are shared across save/turn paths; lock every mutation and rebuild graphs under write lock.

**Evidence:** `ActivationEngine.mu`; race test; comment on historical concurrent map crash.

## P11 — Corpus is priority SSOT; hardcoded maps are fallback only

`PredicatePriorities` in config is deprecated for primary use.

**Evidence:** `types.go` deprecation comments; `LoadPrioritiesFromCorpus`.

## P12 — Learning is third-loop, never silent override of safety

Feedback can nudge ±20 activation points after enough samples; it cannot remove core safety facts from `getCoreFacts`.

**Evidence:** feedback scaling in `computeFeedbackScore`; core path independent of feedback store.
