# features: wiring and what is NOT built

Verified 2026-09-21 against commit `34634770970153e78c1e250fdab7abd888dcce6f` (`main`). Read from `internal/features/features.go`,
`internal/config/user_config.go`, `cmd/nerd/main.go`, `cmd/nerd/cmd_features.go`,
`cmd/nerd/chat/commands_handlers_features.go`,
`cmd/nerd/chat/commands_handlers_misc.go`, `internal/core/cortex_kernel.go`,
`internal/core/kernel_provenance.go`, `internal/ux/migration.go`,
`internal/world/scanner_config.go`, and `internal/config/removed_keys.go`.

## Wired and reachable (caller verified from Go)

- Install: `LoadUserConfig` calls `features.SetActive(cfg.Features)` after a
  successful parse, then logs `features.Summary()` and warns on each of
  `features.Deprecations()`
  (`internal/config/user_config.go:561-572`). Removed keys are rejected before
  the strict decode, so a stale `features` key fails with a named reason
  (`internal/config/user_config.go:541-543`; `internal/config/removed_keys.go`).
- Boot order: `main` loads config eagerly precisely so the registry is
  populated before any gate reads it (`cmd/nerd/main.go:363-370`).
- Flight recorder: `main` starts the tracer ring only when
  `features.IsFlightRecorderEnabled()` holds and the invocation is not a
  campaign (`cmd/nerd/main.go:387`). Default off is a safety property, not a
  preference — the tracer's region allocator can abort the process under
  long-running load (`internal/features/features.go:460-474`).
- Onboarding: `ShouldShowOnboarding` returns false when
  `features.IsOnboardingSkipped()` holds, before touching the workspace
  (`internal/ux/migration.go:190-208`).
- Scan tunables: `DefaultScannerConfig` takes workers and the AST cutoff from
  `features.FastScanWorkers()` / `features.FastASTMaxBytes()`, defaulting to
  `max(min(NumCPU,20),4)` / 2 MiB when they return 0
  (`internal/world/scanner_config.go:29-38`).
- Kernel router construction: `NewCortexKernel` builds the `ShardFactRouter`
  only when `features.IsPerShardFactsEnabled()` holds; otherwise the field
  stays nil and the legacy single-store path runs byte-for-byte
  (`internal/core/cortex_kernel.go:97-123`).
- Inspection: `nerd features` renders `features.Resolved()` as a table plus
  both tunables plus deprecation warnings (`cmd/nerd/cmd_features.go:44-83`);
  the chat `/features` report renders `Resolved()`, `Deprecations()`,
  `Summary()`, and the tunables
  (`cmd/nerd/chat/commands_handlers_features.go`).

## Exists but no caller verified this pass

No production caller was found in the searched set for: `IsProvenanceEnabled`,
`IsSystemShardsEnabled`, `IsDarkModeEnabled`, `IsPromptEvolutionEnabled`,
`DefaultFeaturesConfig`, or `FullyEnabledFeaturesConfig`. Notes on three:

- `features.IsProvenanceEnabled` is a different symbol from the kernel method
  `(*RealKernel).IsProvenanceEnabled`
  (`internal/core/kernel_provenance.go:49-54`). The one chat caller observed
  uses the kernel's, not the flag's
  (`cmd/nerd/chat/commands_handlers_misc.go:101`).
- `IsSystemShardsEnabled` is claimed as the master switch for system-shard
  boot by `Docs/architecture/shards/` (`07-DEPENDENCY-MAP.md:77`,
  `08-WIRING-AND-INTEGRATION.md:59`) — a doc claim, not re-verified from Go
  here. The accessor's own comment flags its history: it once had no caller at
  all, and a legacy `NERD_DISABLE_SYSTEM_SHARDS` mechanism described in older
  prose appears in no `.go` file (`internal/features/features.go:483-501`).
- `IsPerShardFactsEnabled` past construction is bounded by the router's own
  limits: it dispatches single predicates and does not evaluate joins across
  shards (`internal/features/features.go:503-512`).

## Assumed by the design, not done by the code

- Names are trusted, not fenced. Nothing stops two packages reading the same
  flag to mean different things, and nothing warns when a flag has no reader —
  the five unverified accessors above are silent, not errors.
- Misconfiguration is silent by design. A typo'd env value is "no override"
  (`internal/features/features.go:436-458`), so `CODENERD_DARK_MODE=yes` runs
  as false with no message; only shadowed legacy vars get a warning, via
  `Deprecations()`.
- The fast-taxonomy default is load-bearing. The accessor defaults off because
  the fast path skips the scenario sweep entirely; defaulting on would make
  verification skip itself on ordinary runs
  (`internal/features/features.go:527-538`).
- Prompt evolution records unconditionally and gates only who pushes the
  button: the flag defaults off for API-budget reasons while `/evolve` stays
  available by hand (`internal/features/features.go:540-552`).

## What the old corpus got WRONG

The previous 18-file corpus was replaced without reading it for claims; these
are divergences between its text (as grepped for filenames and line anchors
only) and the code above:

- Stale line anchors: its state table placed the accessors around `~L237-285`
  (`02-CURRENT-STATE.md:47-53`); they are at `471-552` today.
- `features.diff_eval` / `IsDiffEvalEnabled`: the key was removed — the
  differential-evaluation path was deleted and the kernel always rebuilds from
  the EDB (`internal/config/removed_keys.go:24-31`). Any page still
  configuring it describes a knob that fails the load.
- Taxonomy wiring contradicts itself: one page marks "wire the tool to the
  accessor" done (`TODO.md:8`) while another says the tool reads raw env and
  never touches the registry (`IMPLEMENTED_SPEC.md:95,239`). The accessor's
  comment describes the wired end state with the default flipped to off
  (`internal/features/features.go:527-538`).
- The `NERD_DISABLE_SYSTEM_SHARDS` mechanism: repeated across the old wiring
  pages, absent from every `.go` file per the accessor comment
  (`internal/features/features.go:493-497`).
- `cmd/nerd/main.go` anchors drifted 4–8 lines early in downstream prose
  (config load cited `355-361`, actual `363-370`; flight gate cited as a
  range starting `377`, actual `387`) — already recorded at
  `Docs/journeys/00-journey-map.md:926`.
