# internal/features

Leaf registry for every codeNERD feature toggle. Precedence is
env → legacy-env → config → default, resolved per call — there is no
snapshot, so a late `SetActive` applies to subsequent reads.

Verified 2026-09-21 against commit `34634770970153e78c1e250fdab7abd888dcce6f` (`main`). The package is
`internal/features/features.go` (616 lines) plus `internal/features/schema.go`
and six `*_test.go` files.

## Resolution in one paragraph

Each boolean accessor calls `resolveBool`
(`internal/features/features.go:419-434`): the canonical `CODENERD_*` env var
wins, then the legacy `NERD_*` name if one exists, then the active
`FeaturesConfig` installed by `SetActive`, then the compile-time default.
`envBool` (`internal/features/features.go:444-458`) accepts only `1`/`true`
and `0`/`false` (case-insensitive); anything else — including a typo — is
silently not an override. Integer tunables follow the same rule via `envInt`
(`internal/features/features.go:581-595`): non-numeric or non-positive values
are ignored. `Active()` (`internal/features/features.go:405-408`) returns the
installed config or nil; callers must not mutate it.

## Flags

| Accessor | Canonical env (legacy) | Default | Lines |
|---|---|---|---|
| `IsFlightRecorderEnabled` | `CODENERD_FLIGHT_RECORDER` (`NERD_FLIGHTREC`) | false | `internal/features/features.go:471-474` |
| `IsProvenanceEnabled` | `CODENERD_PROVENANCE` (none) | false | `internal/features/features.go:479-482` |
| `IsSystemShardsEnabled` | `CODENERD_SYSTEM_SHARDS` (none) | true | `internal/features/features.go:498-501` |
| `IsPerShardFactsEnabled` | `CODENERD_PER_SHARD_FACTS` (none) | false | `internal/features/features.go:509-512` |
| `IsDarkModeEnabled` | `CODENERD_DARK_MODE` (none) | false | `internal/features/features.go:516-519` |
| `IsOnboardingSkipped` | `CODENERD_SKIP_ONBOARDING` (`NERD_SKIP_ONBOARDING`) | false | `internal/features/features.go:522-525` |
| `IsTaxonomyFastEnabled` | `CODENERD_TAXONOMY_FAST` (none) | false | `internal/features/features.go:535-538` |
| `IsPromptEvolutionEnabled` | `CODENERD_PROMPT_EVOLUTION` (none) | false | `internal/features/features.go:549-552` |
| `FastScanWorkers` / `FastASTMaxBytes` | canonical + legacy pair in the `intFlags` table | 0 (= call-site default) | `internal/features/features.go:557-576` |

Call-site defaults for the tunables live with the caller: workers default to
`max(min(NumCPU,20),4)` and the AST cutoff to 2 MiB
(`internal/world/scanner_config.go:29-38`).

## Install and inspect

- `internal/config/user_config.go:561` installs the file config with
  `features.SetActive(cfg.Features)` (nil resets to defaults); the next two
  lines log `features.Summary()` and warn on `features.Deprecations()`
  (`internal/config/user_config.go:565-572`).
- `cmd/nerd/main.go:363-370` eagerly loads that config before any
  feature-gated boot check reads the registry.
- `nerd features` (`cmd/nerd/cmd_features.go:18-32`) prints every flag with
  the value in force and the source that decided it, from `features.Resolved()`
  (`cmd/nerd/cmd_features.go:44-45`); `--json` adds the tunables and
  deprecations (`cmd/nerd/cmd_features.go:47-56`), `--schema` prints the
  config-block schema from `features.ConfigSchemaJSON()`
  (`cmd/nerd/cmd_features.go:37`).
- The chat `/features` report reads the same three surfaces —
  `features.Resolved()`, `features.Deprecations()`, `features.Summary()` —
  plus the two tunables (`cmd/nerd/chat/commands_handlers_features.go`).

## Further reading

- `INTERNALS.md` — how a flag gets its value (registry, precedence, inspectors).
- `WIRING-AND-NOT-BUILT.md` — what is wired and reachable, what exists but has
  no verified caller, what the design assumes that the code does not do, and
  what the old corpus got wrong.
