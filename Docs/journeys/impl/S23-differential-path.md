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

### `internal/features` + `internal/config` (commit 3)

- `features.go`: the `DiffEval` field, `IsDiffEvalEnabled`, the `boolFlags`
  `{"diff_eval", "CODENERD_DIFF_EVAL", ...}` row, and its entries in
  `DefaultFeaturesConfig` / `FullyEnabledFeaturesConfig`. Package-doc and `envBool`
  examples that used `CODENERD_DIFF_EVAL` as the illustrative variable now use
  `CODENERD_DARK_MODE`. `ConfigSchemaJSON` / `ConfigSchemaKeys` are generated from
  `boolFlags`, so `nerd features --schema` stops advertising the key automatically.
- Tests: `features_test.go` (`TestResolveBoolPrecedence`, `TestSetActiveCopySemantics`,
  `TestSummaryRendersBoolPointersAsValues`), `config_roundtrip_test.go`
  (`TestEnvOverridesActiveConfig` and friends) and `resolved_test.go` used `diff_eval`
  as the *subject* of tests that are really about `resolveBool`, `SetActive` snapshotting
  and `Summary` formatting. They are repointed to `provenance` (same default, no legacy
  env var) rather than deleted — the behaviour they pin is not the flag's.
  `features_defaults_test.go` and `schema_test.go` lost the `DiffEval` entries from
  their expectation maps.
- **New**: `internal/config/removed_keys.go` — `rejectRemovedKeys(data []byte) error`,
  called from `LoadUserConfig` *before* `decodeStrictJSON`. The strict decoder already
  refuses the file, but its message ("unknown field \"diff_eval\"") reads like a typo;
  the named rejection says the key is gone and why. Pinned by
  `internal/config/removed_keys_test.go` (rejects `features.diff_eval`, still accepts
  a live `features` block).

`go build ./...`, `go vet` and `go test ./internal/features ./internal/config` green.

## Ouroboros decision

**Repointed, and `DifferentialEngine` deleted.** The ouroboros program is not monotone.

`internal/autopoiesis/ouroboros.go:674-757` (`simulateTransition`) built a
`DifferentialEngine` over `o.engine`, whose only loaded program is
`internal/core/defaults/schemas_state.mg` (`ouroboros.go:302-305`). That file contains
four negated premises:

- `schemas_state.mg:107` `cumulative_penalty(StepID, 20) :- has_panic_penalty(StepID), !has_retry_penalty(StepID).`
- `schemas_state.mg:112` `cumulative_penalty(StepID, 10) :- has_retry_penalty(StepID), !has_panic_penalty(StepID).`
- `schemas_state.mg:117` `cumulative_penalty(StepID, 0) :- state(StepID, _, _), !has_penalty(StepID).`
- `schemas_state.mg:151` `valid_transition(Next) :- state(Curr, CurrStability, _), proposed(Next), state(Next, NextStability, _), !has_effective_stability(Curr), NextStability >= CurrStability.`

and `simulateTransition` fed the engine six facts one at a time with AutoEval on
(`AddFactIncremental`, `ouroboros.go:701-731`), so each negated premise was evaluated
against a partial EDB and every conclusion drawn from it was retained. Concretely: when
`proposed(nextStepID)` lands, `base_stability(nextStepID, ...)` has not been asserted yet,
so `has_effective_stability(nextStepID)` is false and the fallback rule fires with
`Curr = Next = nextStepID` (`NextStability >= CurrStability` is trivially true). One fact
later that premise becomes true, and the `valid_transition(nextStepID)` derived from it is
never retracted. `ouroboros.go:746-755` reads exactly that predicate to decide whether a
self-generated tool is committed.

So the consumer has the same bug, and the brief's condition is met. The repoint:
`simulateTransition` now builds its own `mangle.Engine` with `AutoEval` **off**, loads the
same `schemas_state.mg`, asserts all six facts in one `AddFacts` batch, and evaluates once.
A single stratified pass over a store that holds only EDB facts is sound, and a fresh engine
per simulation preserves the isolation the DifferentialEngine was there for (simulation
facts must not reach `o.engine`'s real state machine).

`internal/mangle/differential.go` (907 lines) is then deleted outright: `DifferentialEngine`,
`ChainedFactStore`, `FactStoreProxy`, `KnowledgeGraph` and `computeStrata` had no other
non-test caller in the tree.

## Config change required

`C:\CodeProjects\codeNERD\.nerd\config.json` in the MAIN checkout has
`"features": { "diff_eval": true, ... }`. It is the user's live config and this worktree
has no copy, so it is NOT touched here. The merger applies exactly one Edit:

- **Remove the line** `    "diff_eval": true,` from inside the `"features"` object
  (preserve the comma placement of whatever key follows/precedes it).

Until that edit lands, `nerd` will refuse to boot with:

```
config .nerd/config.json: features.diff_eval is no longer a supported key: the
differential evaluation path was deleted: ...
```

That failure is deliberate (`internal/config/removed_keys.go`): a removed toggle is a
behaviour change, and a silently-ignored key would leave the operator believing they
still control something.

## Tests

_pending_

## Open

- (everything)
