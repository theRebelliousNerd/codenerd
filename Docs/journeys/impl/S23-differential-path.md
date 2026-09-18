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

`.nerd/config.json` in the main checkout sets `"features": {"diff_eval": true}`, so the gate was
ON in production. A kernel took the differential path whenever it had no proof recorder, no
external predicates, and ≤ 10000 facts. External predicates come from a VirtualStore
(`hasExternalPredicatesLocked`), so the chat/session main kernel took the full path — but every
kernel built by `core.NewRealKernel()` with no virtual store took the differential path:

| Call site | What it is |
|---|---|
| `internal/shards/system/constitution.go:295` | ConstitutionGate's own kernel — the safety-enforcement loop |
| `internal/shards/system/executive.go:441` | Executive shard |
| `internal/shards/system/legislator.go:186` | Legislator shard |
| `internal/shards/system/perception.go:424`, `:495` | Perception shard |
| `internal/shards/system/planner.go:173` | Planner shard |
| `internal/shards/system/router.go:218` | Router shard |
| `internal/shards/system/world_model.go:196` | World-model shard |
| `internal/core/kernel_shard.go:65-79` | every domain `KernelShard` |
| `internal/core/rule_court.go:71` | the learned-rule sandbox |
| `cmd/nerd/cmd_query.go:146,:215`, `cmd_retrieve.go:102`, `dom_cmd.go:115,:257,:477`, `cmd_snapshot.go:88,:190`, `cmd_mcp_select.go:94`, `cmd_init_scan.go:397`, `internal/init/initializer.go:324` | CLI entry points |

The shipped policy corpus that these kernels evaluate contains 399 negated premises and 42 `|>`
transforms (grep over `internal/core/defaults/`), including the constitution. The
ConstitutionGate — the shard that derives `permitted(...)` — was running on an evaluator that
cannot un-derive.

`internal/core/kernel_eval_demote_test.go` (now deleted) recorded the other half: on a 48K-fact
store a one-fact delta took 91 s on the differential path, which is why the demotion machinery
(`diffPathDemoted`, `diffDemoteThreshold`, `differentialFactCeiling`) existed at all. It was
both unsound and, at scale, slower than the thing it replaced.

The M3 study (`Docs/journeys/M3-mangle-verification.md`, item 23 / open item (d)) had already
flagged the rule: any host that re-evaluates on a retained store must clear IDB predicates first.

## Deletions

### `internal/core` (commit 2)

- `kernel_eval.go`
  - `diffEvalEnabled()` and the `codenerd/internal/features` + `codenerd/internal/mangle`
    imports.
  - `evaluateDiffLocked`, `buildDiffEngineLocked`, `diffEngineConfigLocked`,
    `factsToAtomsLocked`, `copyDiffStoreToKernelLocked`, `invalidateDiffEngineLocked`,
    `hasExternalPredicatesLocked` (existed only to keep the diff path away from external
    predicates) — 183 lines.
  - `evaluateFullLocked` was merged into `evaluate()`: with one path left, "full" named a
    contrast that no longer exists. The comment on `evaluate()` now states the invariant
    (rebuild from the EDB every call) and why.
  - `clearFactsLocked(reason string)` → `clearFactsLocked()`; the reason existed only for the
    invalidation log. Call sites in `Clear()`/`Reset()` updated.
  - `ClearSchemas()` and `rebuild()` lost their invalidation calls; `rebuild()`'s comment now
    explains what actually makes a retract visible (dropping `cachedAtoms`).
  - `Clone()` lost the `diffPathDemoted` copy.
- `kernel_types.go` — the whole "Differential evaluation (Task #10)" field block
  (`diffEngine`, `diffMangleEngine`, `dirtyStrata`, `factsSinceLastEval`, `diffPathDemoted`),
  the constants `differentialFactCeiling` and `diffDemoteThreshold`, and
  `EvaluationStats.Mode` / `.DeltaFacts` / `.DemotionReason`. `EvaluationStats` keeps
  `InputFacts` + `Duration` (honest cost telemetry, still read by the world benchmark).
- `kernel_facts.go` — the per-fact delta buffer in `addFactIfNewLocked` and
  `markStratumDirtyLocked`.
- Tests: `kernel_eval_demote_test.go` and `kernel_features_test.go` deleted outright (both
  exist only to pin diff-path behaviour). `kernel_eval_test.go` lost
  `TestKernelDifferentialEval` and `BenchmarkKernelDifferentialEval`;
  `TestKernelEval_ZeroConfigDerivedFactLimitParity` became
  `TestKernelEval_ZeroConfigDerivedFactLimit`, keeping the half that pins the gas limit.
  `kernel_eval_uplift_test.go` lost `TestEval_DiffFullParity` and its helpers; the Clone,
  Clear/Reset and ClearSchemas tests were kept.
  `kernel_eval_large_test.go`: `TestLargeWorldDeltaUsesFullEvaluatorAndPreservesResults` →
  `TestLargeWorldDeltaPreservesResults` (kept: 10001 facts, delta not lost, retraction lands;
  dropped: the `LastEvaluation().Mode` assertion) and `BenchmarkProductionWorldDelta` became
  single-mode — it now measures what a 48K-fact world evaluate costs on the only path.
  `kernel_indexed_store_test.go` lost its `t.Setenv("CODENERD_DIFF_EVAL", "0")` line.

`go build`, `go vet` and `go test ./internal/core` green (252 s).

## Ouroboros decision

_pending_

## Config change required

_pending_

## Tests

_pending_

## Open

- (everything)
