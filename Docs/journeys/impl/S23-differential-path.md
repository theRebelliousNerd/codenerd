# S23 — the differential evaluation path is unsound under negation and aggregation; delete it

## Status

- last updated: 2026-09-18
- branch point: `0b5c69d0`
- branch: `worktree-agent-a74051ff86f826174`
- done: log created; unsoundness pinned by a failing test (fail-before proven)
- open: deletions, flag removal, ouroboros decision, config note

## Evidence

### The unsoundness

`internal/mangle/differential.go:445-464` — `DifferentialEngine.ApplyAtomDelta`, unified fast path:

```go
if de.unifiedStore != nil {
    changed := false
    for _, atom := range atoms {
        if de.unifiedStore.Add(atom) { changed = true }
    }
    if !changed || !de.config.AutoEval { return nil }
    if _, err := mengine.EvalStratifiedProgramWithStats(
        de.programInfo, de.unifiedStrata, de.unifiedPredToStratum, de.unifiedStore,
        de.evalOptions()...,
    ); err != nil { return err }
    return nil
}
```

`de.unifiedStore` is created once in `EnableUnifiedFastPath` (`differential.go:398`) and never
cleared. It accumulates EDB *and* IDB facts across calls — the comment at `:449-452` says so
outright ("The unified store accumulates EDB + IDB across calls so the engine's seminaive
evaluator can skip already-derived facts"). Semi-naive bottom-up evaluation over a store that
already holds previously derived facts is monotone: it can only add. So nothing is ever
un-derived.

The legacy per-stratum path (`differential.go:466-511`) has the same property by construction:
each `de.strataStores[s].store` is only ever `Add`-ed to, and `ChainedFactStore{base, overlay}`
is an overlay over those retained stores.

`internal/core/kernel_eval.go:216-228` routes `RealKernel.evaluate()` into that engine when
`features.IsDiffEvalEnabled()` is true, the kernel has no proof recorder
(`kernel_eval.go:216`), no external predicates (`hasExternalPredicatesLocked`,
`kernel_eval.go:237-251`), and `len(k.facts) <= differentialFactCeiling` (10000,
`kernel_types.go:161`). `buildDiffEngineLocked` opts the kernel into the unified fast path at
`kernel_eval.go:531`.

### The probe (new test, `internal/core/kernel_eval_soundness_test.go`)

With `CODENERD_DIFF_EVAL=1` against the branch point (`0b5c69d0`):

```
--- FAIL: TestKernelEvalUnDerivesUnderNegation (0.62s)
    kernel_eval_soundness_test.go:55: after discharge: s23_open = [s23_open(/o1).], want no facts
--- FAIL: TestKernelEvalReplacesAggregateResult (0.62s)
    kernel_eval_soundness_test.go:113: two items: s23_item_count = [1 2], want exactly [2]
```

With `CODENERD_DIFF_EVAL=0` (full path, `evaluateFullLocked`, `kernel_eval.go:256-380`, which
builds a fresh store from `k.cachedAtoms` every evaluate) both pass.

So: `s23_open(X) :- s23_raised(X), !s23_discharged(X).` stays derived after `s23_discharged(/o1)`
is asserted, and `fn:count()` reports **both** 1 and 2 simultaneously. Every downstream
`Count >= N` rule fires on the stale value.

## Production exposure

_pending_

## Deletions

_pending_

## Ouroboros decision

_pending_

## Config change required

_pending_

## Tests

_pending_

## Open

- (everything)
