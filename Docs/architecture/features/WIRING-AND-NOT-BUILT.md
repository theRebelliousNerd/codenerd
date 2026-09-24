# features: wiring and what is NOT built

Verified 2026-09-21 against commit `34634770970153e78c1e250fdab7abd888dcce6f` (`main`). Read from `internal/features/features.go`,
`internal/features/schema.go`, `internal/config/user_config.go`, `cmd/nerd/main.go`, `cmd/nerd/cmd_features.go`,
`cmd/nerd/chat/commands_handlers_features.go`,
`cmd/nerd/chat/commands_handlers_misc.go`, `cmd/nerd/ui/styles.go`,
`cmd/tools/verify_taxonomy/main.go`, `internal/core/cortex_kernel.go`,
`internal/core/kernel_provenance.go`, `internal/ux/migration.go`,
`internal/world/scanner_config.go`, and `internal/config/removed_keys.go`.
Evidence base: `task_7b853890_2_0` (grep imports vs exact callers), `task_7b853890_1_0` (deep read),
`task_7b853890_1_1` (docs-vs-code verdict). Factory call sites below
(`internal/system/factory.go`, `internal/system/factory_learning.go`,
`internal/config/user_config.go:1494`) are per `task_7b853890_2_0` exact-caller sets.

## Wired and reachable (caller verified from Go)

- Install: `LoadUserConfig` calls `features.SetActive(cfg.Features)`
  (`internal/config/user_config.go:567` in `config.LoadUserConfig`,
  `internal/features/features.go:233-241`), then logs `features.Summary()`
  (`internal/config/user_config.go:571`) and warns on each of
  `features.Deprecations()` (`internal/config/user_config.go:576-578`).
  Removed keys are rejected before the strict decode, so a stale `features` key
  fails with a named reason
  (`internal/config/user_config.go:534-536`; `internal/config/removed_keys.go`).
- Seed config: `config.DefaultUserConfig` returns
  `features.FullyEnabledFeaturesConfig()` (`internal/config/user_config.go:1494`;
  `internal/features/features.go:202-215`) — what `nerd init` writes.
- Boot order: `main` loads config eagerly precisely so the registry is
  populated before any gate reads it (`cmd/nerd/main.go:365-378`).
- Flight recorder: `main` starts the tracer ring only when
  `features.IsFlightRecorderEnabled()` holds and the invocation is not a
  campaign (`cmd/nerd/main.go:395`). Default off is a safety property, not a
  preference — the tracer's region allocator can abort the process under
  long-running load (`internal/features/features.go:460-474`;
  accessor `internal/features/features.go:471-474`).
- Onboarding: `ShouldShowOnboarding` returns false when
  `features.IsOnboardingSkipped()` holds, before touching the workspace
  (`internal/ux/migration.go:192` in `ux.ShouldShowOnboarding`;
  `internal/ux/migration.go:190-208`; accessor `internal/features/features.go:522-525`).
- Scan tunables: `DefaultScannerConfig` takes workers and the AST cutoff from
  `features.FastScanWorkers()` / `features.FastASTMaxBytes()`, defaulting to
  `max(min(NumCPU,20),4)` / 2 MiB when they return 0
  (`internal/world/scanner_config.go:31,36` in `world.DefaultScannerConfig`;
  `internal/world/scanner_config.go:29-38`; accessors
  `internal/features/features.go:557-565` and `internal/features/features.go:568-576`).
- Kernel router construction: `NewCortexKernel` builds the `ShardFactRouter`
  only when `features.IsPerShardFactsEnabled()` holds; otherwise the field
  stays nil and the legacy single-store path runs byte-for-byte
  (`internal/core/cortex_kernel.go:119` in `core.NewCortexKernel`;
  `internal/core/cortex_kernel.go:97-123`; accessor `internal/features/features.go:509-512`).
  Second prod reader: `system.initKernel` (`internal/system/factory.go:1180`).
- System-shard master switch: `system.initShardManagement` reads
  `features.IsSystemShardsEnabled()` (`internal/system/factory.go:1895`;
  accessor `internal/features/features.go:498-501`). This closes the former
  "no caller" gap noted at `internal/features/features.go:489-491`.
- Dark mode: `ui.detectTheme` returns `DarkTheme()` when
  `features.IsDarkModeEnabled()` holds (`cmd/nerd/ui/styles.go:293`;
  accessor `internal/features/features.go:516-519`).
- Fast taxonomy: `cmd/tools/verify_taxonomy/main.go` short-circuits the
  scenario sweep when `features.IsTaxonomyFastEnabled()` holds
  (`cmd/tools/verify_taxonomy/main.go:17` in `main.main`;
  accessor `internal/features/features.go:535-538`). Default off is load-bearing:
  the fast path skips verification, so default-on would make the tool skip
  itself on ordinary runs (`internal/features/features.go:527-538`).
- Prompt evolution: `system.initLearningLoop` gates the background cycle on
  `features.IsPromptEvolutionEnabled()` (`internal/system/factory_learning.go:144`)
  and `system.Cortex.runEvolutionCycle` re-checks it
  (`internal/system/factory_learning.go:320`;
  accessor `internal/features/features.go:549-552`). Recording is unconditional;
  the flag gates only who pushes the button (`internal/features/features.go:540-548`).
- Inspection, CLI: `nerd features` renders `features.Resolved()` as a table
  (`cmd/nerd/cmd_features.go:44` in the `RunE` closure) plus both tunables
  (`cmd/nerd/cmd_features.go:52-53,72-73`) plus deprecation warnings
  (`cmd/nerd/cmd_features.go:45,78-83`); `--schema` prints
  `features.ConfigSchemaJSON()` (`cmd/nerd/cmd_features.go:37`;
  `internal/features/schema.go:21-52`).
- Inspection, chat: the `/features` report renders `features.Resolved()`
  (`cmd/nerd/chat/commands_handlers_features.go:19` in `chat.renderFeaturesReport`),
  `features.Deprecations()` (`cmd/nerd/chat/commands_handlers_features.go:40`),
  `features.Summary()` (`cmd/nerd/chat/commands_handlers_features.go:49`), and the
  tunables (`cmd/nerd/chat/commands_handlers_features.go:36,38`).
- Self-reference: `features.Summary` resolves via `features.Resolved`
  (`internal/features/features.go:376` in `features.Summary`;
  `internal/features/features.go:374-391`) and appends the tunables
  (`internal/features/features.go:387-388`).

## Exists but uncalled in prod (test-only)

No production caller in the index for these four. Each is real code with
test-only callers — not dead text, but not reachable from any prod entry path:

- `features.IsProvenanceEnabled` (`internal/features/features.go:479-482`) —
  zero prod callers; only `internal/features/*_test.go` + `config_roundtrip_test.go`.
  This is a different symbol from the kernel method
  `(*RealKernel).IsProvenanceEnabled` (`internal/core/kernel_provenance.go:49-54`).
  The one chat caller observed uses the kernel's, not the flag's
  (`cmd/nerd/chat/commands_handlers_misc.go:101`). Exists-but-uncalled wiring gap:
  either wire the flag to the kernel or mark it reserved.
- `features.Active` (`internal/features/features.go:408`) — 1 exact caller total:
  `internal/features/features_test.go:122` in `TestSetActiveCopySemantics`.
  By design: prod goes via the `Is*`/`Fast*` accessors, never `Active()` directly.
- `features.DefaultFeaturesConfig` (`internal/features/features.go:156-168`) —
  1 caller: `internal/features/features_defaults_test.go:6`. Conservative
  compile-time defaults, currently unreferenced in prod (`DefaultUserConfig`
  uses `FullyEnabledFeaturesConfig`).
- `features.ConfigSchemaKeys` (`internal/features/schema.go:56-65`) — 2 callers,
  both `internal/features/schema_test.go:14,54`. No prod caller; contrast
  `ConfigSchemaJSON`, which IS reachable via `nerd features --schema`
  (`cmd/nerd/cmd_features.go:37`).

## Assumed seams (imports without exact caller — not wiring evidence)

- `cmd/nerd/cmd_systems.go:12` imports `codenerd/internal/features` but has zero
  exact hits in any `features.*` callers set this round. Assumed systems surface
  until a `read_file cmd/nerd/cmd_systems.go` + `callers_of` re-check names the
  call site. Do not cite the import line as wiring.
- `cmd/nerd/cmd_features.go:8` was in this category in an earlier pass (index
  showed zero exact hits) but is now RESOLVED as reachable: the file calls
  `Resolved`, `Deprecations`, `ConfigSchemaJSON`, `FastScanWorkers`,
  `FastASTMaxBytes` at `cmd/nerd/cmd_features.go:37,44-45,52-53,72-73,80`.
  Kept here only to record that the earlier "no edge" reading was index staleness,
  not absence.
- `Docs/architecture/shards/` `07-DEPENDENCY-MAP.md:77` /
  `08-WIRING-AND-INTEGRATION.md:59` claim `IsSystemShardsEnabled` as the master
  switch — now re-verified from Go at `internal/system/factory.go:1895`, so the
  claim is corroborated, not merely a doc claim.

## Assumed by the design, not done by the code

- Names are trusted, not fenced. Nothing stops two packages reading the same
  flag to mean different things, and nothing warns when a flag has no reader —
  the four uncalled symbols above are silent, not errors.
- Misconfiguration is silent by design. A typo'd env value is "no override"
  (`internal/features/features.go:436-458`), so `CODENERD_DARK_MODE=yes` runs
  as false with no message; only shadowed legacy vars get a warning, via
  `Deprecations()` (`internal/features/features.go:341-363`).

## What the old corpus got WRONG

The previous 18-file corpus was replaced without reading it for claims; these
are divergences between its text (as grepped for filenames and line anchors
only) and the code above:

- Stale line anchors: its state table placed the accessors around `~L237-285`
  (`02-CURRENT-STATE.md:47-53`); they are at `471-576` today
  (`internal/features/features.go:471-576`).
- `features.diff_eval` / `IsDiffEvalEnabled`: the key was removed — the
  differential-evaluation path was deleted and the kernel always rebuilds from
  the EDB (`internal/config/removed_keys.go:24-31`). Any page still
  configuring it describes a knob that fails the load.
- Taxonomy wiring contradiction resolved: one page marked "wire the tool to the
  accessor" done (`TODO.md:8`) while another said the tool reads raw env and
  never touches the registry (`IMPLEMENTED_SPEC.md:95,239`). The tool now reads
  the registry (`cmd/tools/verify_taxonomy/main.go:17`) with the default flipped
  to off (`internal/features/features.go:527-538`) — wired end state.
- The `NERD_DISABLE_SYSTEM_SHARDS` mechanism: repeated across the old wiring
  pages, absent from every `.go` file per the accessor comment
  (`internal/features/features.go:493-497`).
- `cmd/nerd/main.go` anchors drifted 4–8 lines early in downstream prose
  (config load cited `355-361`, actual `365-378`; flight gate cited `387`,
  actual `395`) — already recorded at
  `Docs/journeys/00-journey-map.md:926`.
