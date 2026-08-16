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
- [x] When dual Kernel APIs collapse, delete obsolete aliases after one release cycle
      Closed 2026-08-16. Every shim named in this item is deleted: `KernelFact` (the alias), `Fact.ToFact` (the identity), `KernelInterface`, and `core.AutopoiesisBridge`. The legacy `CtxKey*` constants' readers were migrated to the typed accessors in the same pass, which was this item's other stated precondition.
      1. Step 3a deleted `types.KernelInterface` and `core.AutopoiesisBridge`. Step 2 had already left both with zero production consumers; every remaining call site was a test. The five `AutopoiesisBridge` tests were deleted with the type they exercised, and `query_pattern_test.go`, which reached the kernel only through the bridge, now calls the `RealKernel` method directly with its assertions unchanged.
      2. Step 3b/3c removed the local alias in `internal/autopoiesis`, then `KernelFact` and `Fact.ToFact` themselves across the seven files that actually referenced the type: `internal/core/kernel_utils.go` plus tests in `internal/autopoiesis`, `internal/core`, `internal/types` and `tests/e2e`.
      3. The change was behaviour-preserving by construction: `KernelFact` was declared `= Fact`, an alias rather than a distinct type, and `ToFact` was `return f`. Every substitution is the same value under a different spelling, and the compiler had to agree.
      4. Four tests named `TestKernelFactToFact*` were deleted rather than ported; each existed solely to assert that the alias aliases and the identity is identical, so there was nothing left to pin. Tests that merely used `ToFact` to obtain a `Fact` kept every assertion and now use the value directly.
      5. Hazard for the archive, because it is the trap this change was most likely to fall into: `intent.ToFact()`, `focus.ToFact()`, `diag.ToFact()`, `patch.ToFact()`, `atom.ToFact()` and `resolution.ToFact()` are DIFFERENT methods on other domain types that genuinely convert to a `Fact`. Only calls whose receiver is already a `Fact` were the shim. A blanket find-and-replace would have broken real conversions.
      6. The "after one release cycle" gate in the original item text was never real: the repository has zero git tags, so there is no cadence to wait for, and the revertibility the staging was protecting is provided by each step being its own commit.
      7. Verified against the integration-tagged build as well as the default one, since `tests/e2e` is where an earlier step's regression hid.

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
