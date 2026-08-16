# TODO — `internal/types`

> Last verified: **2026-08-16**  
> Items are **package evolution backlog**, not incomplete documentation. The 2026-08-15 pass cleared
> P0–P2 and the P3 examples item in code; see `_progress.md`.

## P0 — Safety / correctness

- [x] Audit hot assert paths for remaining bare `Args[i].(T)` and `%v` dumps; migrate to `Extract*` / typed construction  
      Done as an enforced sweep, not prose: `internal/types/fact_conventions_guard_test.go` walks every
      `.go` file in the repo, cross-references literal `Fact{…}` constructions against the `bound [...]`
      declarations in `internal/core/defaults/*.mg`, and fails on anything not in its documented baseline.
      Three rules: Decl conformance, no `%v`-rendered fact arguments, no `MangleAtom` type assertion on
      query results. **Findings are listed in the baselines with reasons; every one lives outside
      `internal/types` and is reported to the owning package.** `types.Atom(s)` was added so the fix at
      each `/name` site is one call rather than a remembered naming rule, and the guard's failure
      message names the right constructor for each slot kind.
- [x] Ensure all production kernels used with multi-op updates implement `KernelTransactor` (mocks too)  
      `internal/types/kernel_transactor_guard_test.go` finds every `types.Kernel` implementation in the
      repo (by marker method set) and requires `Transaction()`, with a baseline naming the 3 production
      forwarding adapters and 14 test doubles that lack it. `types.TransactorOf` added for the
      non-panicking check; `NewKernelTx`'s panic now names the concrete type and the one-line fix.

## P1 — API consolidation

- [x] Plan deprecation path: `KernelInterface` / `KernelFact` → full `Kernel` + adapters only at edges  
      Plan is written on `KernelInterface` in `types.go` (3 steps, one per release cycle). **Step 1 is
      done**: `KernelFact` is now `= Fact`, so the two APIs speak one fact type and the bridge's
      per-result slice copies are identity conversions. Step 2 (autopoiesis moves to `Kernel`) edits
      another package and is left for its owner; the import-graph blocker recorded in Q1 does not exist.
- [x] Decide: typed context keys for spawn priority / model capability (match session key pattern)  
      Decided and implemented in `internal/types/ctxkeys.go`: private zero-width struct keys with
      `WithSpawnPriority` / `WithModelCapability` / `WithModelName` and matching `…FromContext` readers.
      Setters dual-write the legacy string key and readers fall back to it, so the migration is safe in
      either order while `api_scheduler`, `session` and the perception clients still read the raw key.
- [x] Add container (`map`/`slice`) ToAtom table tests  
      `internal/types/container_toatom_test.go`. Also pins the float rendering asymmetry: mangle-go
      renders `Float64(2.0)` as the text `2`, which re-parses as `int64` — `Fact.String` uses `%f` and is
      therefore safe, and there is now a test that fails if that verb is "simplified".

## P2 — Hygiene

- [x] Consider nested sub-structs if `SessionContext` gains more sections (keep field groups navigable)  
      Decided: stays flat. Rationale recorded on the struct (Q4).
- [x] Optional test helper: `MockKernel` implementing `Kernel` + `KernelTransactor` for shared unit tests  
      `internal/types/typestest` — a sibling package, so no cycle. Carries compile-time assertions for
      both interfaces and rejects facts `ToAtom` would reject.
- [x] Document VirtualStore expansion policy in code comment when next method is added  
      Written now rather than "when": ≥2 external consumers before a method is added (Q6).

## P3 — DX

- [x] Optional package-level godoc examples for `ToAtom` and `NewKernelTx` — `internal/types/example_test.go`
- [ ] When dual Kernel APIs collapse, delete obsolete aliases after one release cycle
      _Updated 2026-08-16 — accurate and actionable; replaces the earlier "next release cycle" blocker, which was wrong (see point 1):_
      1. **No "one release cycle" gate.** The repository has zero git tags, so there is no release cadence to wait for. The property that staging actually buys — each step independently revertible — is provided by each step being its own commit. The previous note cited the cycle as the blocker; that was incorrect.
      2. **DONE — step 3a: `types.KernelInterface` and `core.AutopoiesisBridge` deleted.** Step 2 had left both without a single production consumer; every remaining call site was a `_test.go` file. The five `AutopoiesisBridge` tests were deleted (they exercised a type that is gone) and `internal/core/query_pattern_test.go`, which went through the bridge only to reach the kernel, now calls the `RealKernel` method directly with its assertions unchanged.
      3. **DONE — `internal/autopoiesis` alias removed.** The package no longer carries its own `type KernelFact = types.KernelFact`; it names `types.Fact` directly.
      4. **STILL OPEN — `types.KernelFact` and `Fact.ToFact` (22 files, measured 2026-08-16).** Cannot be deleted until remaining consumers move to `types.Fact`. That is 22 files, not the handful the original note implied, and it includes production code — not just tests: `internal/browser/kernel_bridge.go`; `internal/context/compressor.go`, `compressor_metrics.go`, `types.go`; `internal/northstar/guardian.go`; `internal/system/factory.go`; `internal/core/kernel_utils.go`; `cmd/nerd/chat/northstar_persistence.go`; plus test files in `internal/core`, `internal/types`, `internal/northstar`, `internal/browser`, `internal/context`, `cmd/nerd/chat` and `tests/e2e`. Several of those packages declare their own local `KernelFact` alias on top of `types'`, so the migration is per-package rather than a single find-and-replace.
      5. **Subtle — `Fact.ToFact` false-friend hazard.** `Fact.ToFact` is an identity shim (`func (f Fact) ToFact() Fact { return f }`) and its call sites look identical to unrelated methods that must NOT be touched: `intent.ToFact()`, `focus.ToFact()`, `diag.ToFact()`, `patch.ToFact()`, `atom.ToFact()` and `resolution.ToFact()` are DIFFERENT methods on different domain types that genuinely convert to a `Fact`. Only calls whose receiver is already a `Fact` are the shim — there are six, in `internal/core/kernel_utils.go` and the `internal/types` tests.
      6. **Remaining work:** Migrate those 22 files from `KernelFact` to `Fact` package by package, replace the six identity `ToFact` calls with the value itself, then delete `type KernelFact = Fact` and `func (f Fact) ToFact() Fact` from `internal/types/types.go`. Every substitution is behaviour-preserving by construction, because `KernelFact` is an alias rather than a distinct type.

## Done (recent, evidenced in code)

- [x] Remove silent ToAtom stringification of unknown/pointer types
- [x] Remove non-atomic NewKernelTx fallback (panic)
- [x] Extract* helpers for fact args
- [x] Optional LLM capability interfaces (grounding, thinking, piggyback, …)
- [x] GraphQuery / VirtualStore cycle-break markers
- [x] `TransparencyManager` / `ShardPhase` / `OperationRecord` moved here so ShardManager can report
      operator visibility without importing `internal/transparency` (`transparency.go`)

## Reported to other packages (found by the P0 sweep, not fixed here)

These are live findings from the enforced audit. They are baselined in
`fact_conventions_guard_test.go` with the same reasons, so they cannot silently grow.

| Site | Finding |
|---|---|
| `internal/core/kernel_query.go` | `git_state(Attribute, …)` is declared `/name`; all four attributes are asserted as quoted strings. The reader (`cmd/nerd/chat/model_session_context.go`) matches the string, so **writer and reader must be fixed together**. |
| `internal/core/virtual_store_file_actions.go` | `edit_failed` / `delete_blocked` reason arg declared `/name`, asserted as `"pattern_not_found"` / `"no_confirmation"`. No policy reads them yet — the first one written would never fire. |
| `cmd/nerd/chat/campaign.go` | `campaign_intent_capture` autonomy arg declared `/name`, asserted as `"hands_free"`. |
| `internal/campaign/types.go` | `task_error` ErrorType declared `/name`, asserted as `"execution_error"`. |
| `internal/shards/system/router.go` | `routing_error` ActionType declared `/name`, asserted as `"internal_error"` (×2). |
| `cmd/nerd/chat/model_update.go` | `continuation_step` / `max_continuation_steps` are `/number` but passed `float64`; survives only because `RealKernel.coerceAtomToDeclLocked` narrows whole floats. |
| `internal/core/shadow_mode.go` | `simulated_effect` arg 2 built with `fmt.Sprintf("%v", effect.Args)` — renders a `[]any` as `[a b c]`. Should use the JSON container path. |
| `cmd/nerd/chat/helpers.go`, `helpers_scan.go`, `cmd/nerd/cmd_init_scan.go`, `internal/world/incremental_scan.go`, `internal/world/persist.go` | `fact.Args[i].(MangleAtom)` on **kernel query results**, which never carry `MangleAtom` (both readback paths return `NameType` as a plain `string`). These branches can never be taken: the workspace summary renders with no language/framework and `/scan --deep` finds no Go files. Use `ExtractName` / `ArgName`. |
| `cmd/nerd/chat`, `cmd/nerd`, `internal/system` | `sessionKernelAdapter` / `campaignKernelAdapter` wrap a `*core.RealKernel` and forward 13 Kernel methods but not `Transaction()`, so `types.NewKernelTx` reached through them panics. |
