# S23 — the differential evaluation path is unsound under negation and aggregation; delete it

## Status

- last updated: 2026-09-18
- branch point: `0b5c69d0` (this worktree is 79 commits behind `dogfood/c2-closure`; see Open (d))
- branch: `worktree-agent-a74051ff86f826174`
- done:
  - unsoundness pinned by a test that fails on the branch point and passes after
  - `evaluateDiffLocked` and the whole diff-engine apparatus deleted from `internal/core`
  - `features.DiffEval` / `IsDiffEvalEnabled` / `CODENERD_DIFF_EVAL` deleted; a config still
    carrying `features.diff_eval` now fails to load by name
  - ouroboros repointed to a fresh, soundly-driven `mangle.Engine`; a real gate bug fixed
  - `internal/mangle/differential.go` deleted (no remaining non-test caller)
  - `go build ./...`, `go vet`, `go test` green on every touched package
- open: six items below — one config edit the merger must apply, two rebase follow-ups, and
  three findings this seam uncovered but deliberately did not fix

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

### A correction to the ouroboros analysis, found while testing

The ouroboros path was **not producing wrong answers at runtime**, by accident. `mangle.Engine.ToggleAutoEval` sets `e.autoEval`, not `e.config.AutoEval` (`engine.go:172-176`), and `NewDifferentialEngine` copies `base.config` (`differential.go:261-270` at the branch point). `NewOuroborosLoop` builds its engine with `engineConfig.AutoEval = false` (`ouroboros.go:257-259`) and only calls `ToggleAutoEval(true)` afterwards, so `de.config.AutoEval` was **false** and every `ApplyDelta` / `AddFactIncremental` returned before evaluating. The simulation's answers came entirely from `DifferentialEngine.Query`'s top-down `EvalQuery` over an EDB-only store — which is sound.

So the accumulation bug was one `ToggleAutoEval`-vs-`config.AutoEval` line away from being live inside the self-modification gate, and was not live. That does not change the decision (the machinery is deleted either way), but it does mean the ouroboros change is a **soundness-preserving repoint** rather than a fix to a live wrong answer. The log says so rather than claiming a verdict change that did not happen.

### The bug the repoint did expose

`DifferentialEngine.Query` (deleted, `differential.go:717-830` at the branch point) passed `shape.atom` to `GetFacts` and then emitted **every** row `EvalQuery` produced, with no filter on the query's constant. `Engine.Query` does filter (`engine.go:951-954`, `factstore.Matches` + `repeatedVariablesAgree`, added by `42eafd37`). Repointing therefore turned the gate from "accept if any `valid_transition` row exists" into a real check — and the real check failed, because `ouroboros.go` built the query as `fmt.Sprintf("valid_transition(%s)", nextStepID)`. `nextStepID` is `/step_<name>_next`, which in query position parses as a **/name** constant, while `state`/`proposed` are `bound [/string]` (`schemas_state.mg:27-29`) so the stored argument is a **/string**. Verified with a throwaway probe: `factstore.Matches` correctly reports `false` for `ast.Constant{Type:1 /*string*/}` against `ast.Constant{Type:0 /*name*/}` with the same symbol.

Fixed to `fmt.Sprintf("valid_transition(%q)", nextStepID)`. Net effect: for four months the Phase 3 stability gate on self-generated tools never looked at which transition it was approving.

## Tests

### Fail-before proof (the S23 behavioural change)

`internal/core/kernel_eval_soundness_test.go`, run against the branch point `0b5c69d0` with `CODENERD_DIFF_EVAL=1`:

```
--- FAIL: TestKernelEvalUnDerivesUnderNegation (0.62s)
    kernel_eval_soundness_test.go:55: after discharge: s23_open = [s23_open(/o1).], want no facts —
      a derived fact whose negated premise became true was never retracted
--- FAIL: TestKernelEvalReplacesAggregateResult (0.62s)
    kernel_eval_soundness_test.go:113: two items: s23_item_count = [1 2], want exactly [2] —
      the previous aggregate result was added to rather than replaced
```

Same code with `CODENERD_DIFF_EVAL=0`: both PASS. After the deletion there is no env var and no second path, so the file carries no `t.Setenv` — it pins the behaviour on the only path there is.

### Fail-before proof (the ouroboros repoint)

`internal/autopoiesis/ouroboros_simulation_test.go`
`TestSimulateTransition_GateIsScopedToThisProposalAndIsolated` **passes against the branch point too**, and that is honest: as established above, the old path answered soundly by accident. It is a *forward* pin, and the fail condition was verified by regressing the one thing it guards — changing `%q` back to `%s` in the gate query:

```
--- FAIL: TestSimulateTransition_GateIsScopedToThisProposalAndIsolated (0.01s)
    ouroboros_simulation_test.go:41: simulation rejected a monotonic 0.00 -> 0.90 transition:
      transition rejected by Mangle (unstable): stability 0.90 < threshold
```

(The whole-tree signal was the same: `TestOuroboros_WhenFirstProposalIsUnsafe_ShouldRegenerateAndSurviveThunderdome` failed at the simulation stage until the query constant was fixed.)

A second candidate test — "a proposal whose stability degrades must be rejected" — was written and **deleted**, because it does not hold and the reason is a rule defect, not a repoint defect. See Open (a).

### Pass-after

| Package | Result |
|---|---|
| `go build ./...` | clean |
| `go vet ./internal/core ./internal/features ./internal/config ./internal/mangle ./internal/autopoiesis` | clean |
| `go test ./internal/core` | ok (252 s) |
| `go test ./internal/features` | ok |
| `go test ./internal/config` | ok |
| `go test ./internal/mangle` | ok |
| `go test ./internal/autopoiesis` | ok (19.7 s) |

One flake seen on the first full `./internal/autopoiesis` run and not on the rerun:
`TestOuroborosLoop_HotReload_LockedBinary` ("tool execution canceled: context deadline
exceeded"). It compiles a Go binary and gives it a 2 s `ExecuteTimeout` for a 300 ms sleep
while the rest of the package (Thunderdome, other compile-heavy tests) runs alongside — a
wall-clock test under machine load. It passes alone and on a repeat full run, and it exercises
`ExecuteTool`/`hotReload`, not `simulateTransition` or any engine this seam touched.

## Residual references (grep, case-insensitive, after the work)

`diff_eval` / `DIFF_EVAL` / `DiffEval` in code: **only** `internal/config/removed_keys.go` and its test, which is the rejection itself. No `CODENERD_DIFF_EVAL` anywhere.

`DifferentialEngine` in code: three comments that name it as history, all accurate —
`internal/autopoiesis/ouroboros.go:772` (why the gate query needs `%q`),
`internal/config/user_config.go:277` → corrected, it no longer lists it,
`internal/mangle/engine_harden_step2c_regression_test.go:22` → corrected.

Left alone deliberately:

- `Docs/architecture/**` — 26 files mention the flag. `CLAUDE.md` says this corpus was generated by a weak model, is stale or never-true, and must never be cited as evidence; S10 replaces it. Rewriting it here would dignify it.
- `.claude/skills/codenerd-dogfood/references/component-ledger.md` and `subsystem-pass-2026-09-04.md` — a dated ledger of what was measured then. Editing the record of a past measurement would be falsifying it; the ledger entry for this change is the merger's to add.
- `Docs/audits/MANGLE_FACT_APIS_INVENTORY.md:285` — same, a dated audit.

## Open

(a) **`schemas_state.mg`'s `valid_transition` does not constrain `Curr`.** Both rules
(`schemas_state.mg:144-157`) quantify `state(Curr, ...)` with nothing tying `Curr` to the step
being left, so `Curr = Next` satisfies `NextEff >= CurrEff` reflexively and **every** proposed
step derives `valid_transition`. The Phase 3 stability gate is therefore still vacuous, now for
a policy reason rather than a query reason. Out of scope for S23 (it changes self-modification
safety semantics and needs its own decision), but it is the next thing to fix in this area, and
it is why the degraded-stability test was deleted rather than left failing.

(b) **`mangle.Engine` has the same monotone-store property as the thing deleted here.**
`evalWithGasLimit` (`engine.go:218-236`) runs `EvalStratifiedProgramWithStats` over the retained
`e.store`, so any caller that adds facts and re-evaluates accumulates IDB exactly as the diff
path did. `ReplaceControlFacts` (`engine.go:572-604`) documents this and works around it by
clearing every rule-head predicate first — that is the only sound re-evaluation entry point on
`Engine`. The ouroboros repoint sidesteps it (fresh engine, one batch, one evaluation), but the
general hazard is live for every other `Engine` caller and deserves its own seam.

(c) **`ToggleAutoEval` does not change `config.AutoEval`.** `engine.go:172-176` sets `e.autoEval`
while `e.config.AutoEval` keeps its construction-time value, and anything that copies `e.config`
(as `NewDifferentialEngine` did) reads the stale one. One field, two truths. Small, and exactly
the class of thing that made the ouroboros behaviour so hard to reason about.

(d) **Two brief items refer to code this worktree cannot see.** It was created from
`origin/main` (`0b5c69d0`), 79 commits behind `dogfood/c2-closure`:
  - `cmd/nerd/chat/verdict_from_evidence_test.go` does not exist here; its
    `t.Setenv("CODENERD_DIFF_EVAL", "0")` line (added today at `097d59ac`) must be **deleted by
    the merger** when this branch is rebased onto the tip — the variable no longer exists.
  - `internal/config/limits.go`'s `rejectRemovedCoreLimitKeys` (S3, `cb6dab78`) is not here
    either, so the by-name rejection was built fresh as `internal/config/removed_keys.go`
    rather than mirrored. On rebase, consider folding the two into one place; they are the same
    idea applied to `core_limits` and `features`.
  - `.claude/rules/nerd-config-schema.md` does not exist on this base, so its `features` row
    was not updated. The merger should drop `diff_eval` from it if it names the key.

(e) **`.nerd/config.json` in the main checkout must have `"diff_eval": true` removed** before
anything boots. See `## Config change required`. Not done here on purpose — the file is the
user's and this worktree has no copy.

(f) **Cost.** `BenchmarkProductionWorldDelta` (`internal/core/kernel_eval_large_test.go`) now
measures the single remaining path on a 48K-fact world. It was not run in this session (it is a
long benchmark and no baseline existed on the sound path to compare against). If the world-shard
evaluate is too slow after this, that number is where to start — and the answer is a *sound*
incremental evaluator (clear IDB, then re-derive), not the one that was deleted.
