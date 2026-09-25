# core: wiring and what is NOT built

Re-verified 2026-09-25 (lane A build-out) against the working tree; first
written 2026-09-21 against `3463477` from `kernel_facts.go`, `kernel_eval.go`,
`kernel_provenance.go`, `kernel_validation.go`, `kernel_sysfacts.go`,
`virtual_store.go`, `virtual_store_routing.go`, and the boot-guard audit in
`Docs/journeys/01-docs-audit.md`. Each item of the 2026-09-21 list is kept
below with what the code does today.

## Wired and reachable

- Action routing evaluates the boot guard first
  (`virtual_store_routing.go:41-47`), then Dreamer simulation, constitution,
  allowlists, and validators. Default is deny: nothing routes until policy
  derives `permitted`.
- **Two boot guards, and the router's names itself.** `VirtualStore`'s guard
  (`DisableBootGuard` / `IsBootGuardActive`, `virtual_store.go:395-408`)
  refuses with `boot guard active: action routing blocked until first user
  interaction` (`virtual_store_routing.go:46`), so a denied route does say
  which gate denied it. Every cortex host releases it at boot
  (`internal/system/factory.go:1420`), and the one-shot CLI paths release it
  again explicitly. The sibling `ExecutivePolicyShard` guard is released only
  by chat, on the first input (`cmd/nerd/chat/process.go:135`, via
  `ShardManager.DisableExecutiveBootGuard`). Outside chat it stays engaged and
  the executive suppresses the actions it derives
  (`internal/shards/system/executive.go:583-585`, debug log); those hosts query
  and route `next_action` themselves (`cmd/nerd/cmd_instruction.go`), so the
  executive is passive there rather than silently broken.
- The fact path is one chain: `Assert` validates (`ValidateFact`),
  canonicalizes (`canonFact`), dedups (`addFactIfNewLockedErr`), and every atom
  that reaches the store passes Decl-directed coercion
  (`coerceAtomToDeclLocked`, `kernel_fact_decl.go:46`); `rebuildProgram` then
  `evaluate` derive consequences; `Query` reads them back.
- **Numbers are coerced at the source boundary, not by convention.** An
  integral float in a `/number` slot is narrowed to `int64`; a fractional,
  NaN, infinite or out-of-range one is refused for that fact, with the
  predicate and argument named (`kernel_fact_decl.go:46-98`), instead of
  aborting the fixpoint.
- `Explain` is reachable: chat `/explain` turns provenance on, forces a
  re-evaluation and asks for proofs
  (`cmd/nerd/chat/commands_handlers_misc.go:100-116`); `CortexKernel`
  enables it on every shard (`cortex_kernel_inspect.go:88`).
- Kernel lifecycle helpers have production callers: `Reset`
  (`cmd/nerd/chat/commands_handlers_files.go:23`, `cmd/nerd/cmd_campaign.go:1066`,
  `internal/system/factory_adapters.go:404`) and `Clone`
  (`internal/core/dreamer.go:295`, `internal/system/factory_adapters.go:99`,
  `internal/init/jit_integration.go:138`, `shadow_mode.go`).
- **Learned rules cannot widen `permitted` or write a host witness.**
  `RealKernel.ValidateLearnedRule` (`kernel_validation.go:20`) validates with
  the protection its program derives now (`learnedHeadProtectionLocked`,
  `kernel_validation.go:79`): the grant path (`mangle.GrantPathOf`) and
  `hostWitnessPredicates` (`mangle_updates.go:271`), the one set the
  control-packet gate `predicateAllowed` (`mangle_updates.go:307`) also
  refuses. The boot heal of `learned.mg` (`validateLearnedRulesContent`) uses
  the same protection. See `Docs/architecture/mangle/WIRING-AND-NOT-BUILT.md`.
- Git state reaches policy through `UpdateSystemFacts` / `parseGitStatus`
  (`kernel_sysfacts.go`).

## Exists but nothing in production calls

- `Clear` (`kernel_eval.go:407`) does exactly what `Reset` (`:415`) does and
  has no production caller. Declined: removing the duplicate is a maintainer
  call.
- `GetStartupValidationResult` (`kernel_validation.go:517`): no consumer. It
  re-reads `learned.mg` and re-validates it (now with the grant path and host
  witnesses) without healing. Declined: the boot heal already reports every
  invalid rule at Warn and comments it out on disk, and choosing a surface for
  the summary (status line, `check-mangle`) is a UI decision.

## Assumed by the design, not done by the code

- `rebuildProgram` concatenates schemas, policy and learned rules and does not
  resolve conflicting `Decl`s by precedence. By design: Mangle's analyzer
  rejects a predicate declared twice, so the kernel fails closed at boot (with
  the debug dump) rather than picking a winner, and the embedded corpus is
  pinned by `TestSchemas_NoPredicateIsDeclaredTwice` and
  `TestPolicy_DeclsDoNotCollideWithSchemas`
  (`internal/core/defaults/schema_duplicate_decl_test.go:54`, `:101`).

## Deliberately not built

- No metrics exporter among the kernel symbols: counters exist where the
  subsystems keep them (routing, sharding, scheduling), but nothing in the
  `kernel_*.go` surface exports them to an external telemetry system.
- No runtime rule repair: `healLearnedRules` (`kernel_validation.go:107`) runs
  once at load. A rule that degrades mid-run is not re-validated or healed;
  a new one is validated at admission (`HotLoadLearnedRule`), grant path
  included.

## Closed in this pass (2026-09-25)

| Item (2026-09-21 wording) | Verdict | Evidence |
|---|---|---|
| `Explain` unpopulated in normal runs | stale | chat `/explain`, `commands_handlers_misc.go:100-116` |
| `Clear` / `Reset` / `Clone` no production caller | stale for `Reset`, `Clone`; `Clear` is a duplicate (declined) | callers above |
| Numbers int64 "by convention plus scrubbing" | stale | `coerceAtomToDeclLocked`, `kernel_fact_decl.go:46` |
| Host silence when a guard is never released; no guard diagnostics | stale for `VirtualStore` (its error names the guard; every cortex boot releases it); the executive guard is passive outside chat by design | `virtual_store_routing.go:46`, `factory.go:1420` |
| Learned rules could promote into the grant path and host witnesses | built | `internal/core/learned_grant_path_test.go` |
