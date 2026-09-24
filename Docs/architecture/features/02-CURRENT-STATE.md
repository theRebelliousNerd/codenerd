---
doc-class: shipped
subsystem: features
implementation-status: shipped
last-verified: 2026-09-21
verified-against: 34634770970153e78c1e250fdab7abd888dcce6f
supersedes: []
---

# features: current state (shipped)

One question: what in `internal/features` is built and reachable today.
Every claim below cites Go read this pass. Reachability means a caller
verified from Go here; anything not re-verified this pass is deferred to
`WIRING-AND-NOT-BUILT.md`, not repeated as fact.

Read from `internal/features/features.go`,
`internal/features/schema.go`, `internal/config/user_config.go:518-599`,
`cmd/nerd/main.go:355-425`, `cmd/nerd/cmd_features.go`,
`cmd/nerd/chat/commands_handlers_features.go`,
`internal/world/scanner_config.go`, `internal/core/cortex_kernel.go:90-130`,
`internal/ux/migration.go:183-217`,
`internal/core/kernel_provenance.go:40-62`,
`cmd/tools/verify_taxonomy/main.go:1-35`, and
`internal/config/removed_keys.go`.

## Package role

`internal/features` is a leaf-level toggle registry so `internal/core` can
read flags without an import cycle with `internal/config`
(`internal/features/features.go:1-13`). It depends on nothing inside
codeNERD (`internal/features/features.go:12-13`). The flow is
`internal/config` loads `.nerd/config.json` then calls
`features.SetActive` (`internal/features/features.go:233-241`); leaf
packages (`internal/core`, `internal/observability`, `internal/world`)
read forever through the `Is*` / `Fast*` accessors
(`internal/features/features.go:471-576`).

## File: `internal/features/features.go` (implementation)

### Config shape

`struct features.FeaturesConfig` (`internal/features/features.go:69-138`)
is the 10-field on-disk JSON shape. All booleans are `*bool` to
distinguish absent from `false` (`internal/features/features.go:64-68`):

- `FlightRecorder *bool` `json:flight_recorder`
  (`internal/features/features.go:73`), env `CODENERD_FLIGHT_RECORDER`,
  legacy `NERD_FLIGHTREC`.
- `Provenance *bool` `json:provenance`
  (`internal/features/features.go:79`), env `CODENERD_PROVENANCE` only.
- `SystemShards *bool` `json:system_shards`
  (`internal/features/features.go:86`), env `CODENERD_SYSTEM_SHARDS`.
- `PerShardFacts *bool` `json:per_shard_facts`
  (`internal/features/features.go:95`), env `CODENERD_PER_SHARD_FACTS`.
- `DarkMode *bool` `json:dark_mode`
  (`internal/features/features.go:99`), env `CODENERD_DARK_MODE`.
- `SkipOnboarding *bool` `json:skip_onboarding`
  (`internal/features/features.go:103`), env `CODENERD_SKIP_ONBOARDING`,
  legacy `NERD_SKIP_ONBOARDING`.
- `TaxonomyFast *bool` `json:taxonomy_fast`
  (`internal/features/features.go:110`), env `CODENERD_TAXONOMY_FAST`.
- `PromptEvolution *bool` `json:prompt_evolution`
  (`internal/features/features.go:127`), env `CODENERD_PROMPT_EVOLUTION`.
- `FastScanWorkers int` `json:fast_scan_workers`
  (`internal/features/features.go:132`), env
  `CODENERD_FAST_SCAN_WORKERS`, legacy `NERD_FAST_SCAN_WORKERS`, 0 means
  call-site default.
- `FastASTMaxBytes int64` `json:fast_ast_max_bytes`
  (`internal/features/features.go:137`), env
  `CODENERD_FAST_AST_MAX_BYTES`, legacy `NERD_FAST_AST_MAX_BYTES`, 0 means
  call-site default.

### Constructors

`func features.DefaultFeaturesConfig`
(`internal/features/features.go:156-168`) returns the conservative
compile-time defaults used when no config and no env are present: all
`false` except `SystemShards:true`. The comment states the rationale:
Provenance allocates per-derivation and FlightRecorder can abort via
`throw(traceRegion)` (`internal/features/features.go:140-151`,
`internal/features/features.go:460-467`).

`func features.FullyEnabledFeaturesConfig`
(`internal/features/features.go:202-215`) is what `nerd init` writes: all
`true` except `PerShardFacts:false` and `PromptEvolution:false`.
PerShardFacts stays false per the 2026-08-15 audit recorded in the comment:
`ShardFactRouter` is dispatch-only with no cross-shard joins
(`internal/features/features.go:175-196`). PromptEvolution stays false for
cost (`internal/features/features.go:112-127` in the
`DefaultFeaturesConfig` rationale block and
`internal/features/features.go:540-552`).

### Active registry

`var features.active atomic.Pointer[FeaturesConfig]`
(`internal/features/features.go:221`) holds the installed config for
wait-free reads on hot paths.

`func features.SetActive` (`internal/features/features.go:233-241`)
installs the config parsed from `.nerd/config.json`; it copies the struct
so callers cannot mutate the live pointer, and `nil` resets to defaults.
The caller must log via `Summary()` because the leaf cannot import
logging.

`func features.Active` (`internal/features/features.go:408`) returns
`active.Load()`, nil when unset; callers must not mutate the result.
Production code reaches flags through the `Is*` / `Fast*` accessors, not
through `Active()` directly.

### Resolution model

`type features.Source string` (`internal/features/features.go:244`) with
`SourceEnv=env` (`internal/features/features.go:248`),
`SourceLegacyEnv=legacy-env` (`internal/features/features.go:252`),
`SourceConfig=config` (`internal/features/features.go:254`), and
`SourceDefault=default` (`internal/features/features.go:256`).

`struct features.Flag` (`internal/features/features.go:261-268`) carries
`Name, EnvVar, LegacyEnvVar, Value, Source, Default`.

`var features.boolFlags` (`internal/features/features.go:276-291`) is the
single source of truth for the 8 booleans (name, canonical env, legacy
env, getter, default). `var features.intFlags`
(`internal/features/features.go:296-303`) is the same for the 2 integer
overrides.

`func features.resolveBool` (`internal/features/features.go:419-434`)
implements canonical-env to legacy-env to active-config to default, in
that order. `func features.envBool`
(`internal/features/features.go:444-458`) accepts only `1` / `true` and
`0` / `false` case-insensitive, else nil, so a stray export never flips a
bit. `func features.envInt` (`internal/features/features.go:581-595`)
prefers canonical then legacy; non-numeric or non-positive values are no
override. `func features.parseInt64`
(`internal/features/features.go:597-609`) accepts digits only.
`var features.errBadInt`, `type features.featuresErr`, and
`func features.newErr` (`internal/features/features.go:611-616`) are the
integer-parse sentinel.

### Inspectors

`func features.Resolved` (`internal/features/features.go:308-328`)
returns every boolean as the accessors resolve it, with the winning
source.

`func features.Deprecations` (`internal/features/features.go:341-363`)
reports legacy `NERD_*` variables in use that have a canonical
replacement, including shadowed ones the operator needs to hear about.

`func features.Summary` (`internal/features/features.go:374-391`)
renders the single-line resolved-values log (`name=value(source)` for
non-default, plus `fast_scan_workers` and `fast_ast_max_bytes`). It fixes
the prior raw-config bug described in its own comment
(`internal/features/features.go:365-373`).

`func features.boolPtrString` (`internal/features/features.go:395-403`)
is the nil-to-`unset` log helper.

### Accessors (all resolve through `resolveBool` / `envInt`)

- `func features.IsFlightRecorderEnabled`
  (`internal/features/features.go:471-474`): default OFF, `CODENERD_FLIGHT_RECORDER` /
  `NERD_FLIGHTREC`.
- `func features.IsProvenanceEnabled`
  (`internal/features/features.go:479-482`): default OFF, `CODENERD_PROVENANCE`,
  gates the DerivationRecorder for `/explain`.
- `func features.IsSystemShardsEnabled`
  (`internal/features/features.go:498-501`): default ON,
  `CODENERD_SYSTEM_SHARDS`; the per-shard disable is the
  `--disable-system-shard` flag, not env
  (`internal/features/features.go:484-497`).
- `func features.IsPerShardFactsEnabled`
  (`internal/features/features.go:509-512`): default OFF,
  `CODENERD_PER_SHARD_FACTS`.
- `func features.IsDarkModeEnabled`
  (`internal/features/features.go:516-519`): `CODENERD_DARK_MODE`.
- `func features.IsOnboardingSkipped`
  (`internal/features/features.go:522-525`): `CODENERD_SKIP_ONBOARDING` /
  `NERD_SKIP_ONBOARDING`.
- `func features.IsTaxonomyFastEnabled`
  (`internal/features/features.go:535-538`): default OFF,
  `CODENERD_TAXONOMY_FAST`, skips the scenario sweep.
- `func features.IsPromptEvolutionEnabled`
  (`internal/features/features.go:549-552`): default OFF for cost,
  `CODENERD_PROMPT_EVOLUTION`.
- `func features.FastScanWorkers`
  (`internal/features/features.go:557-565`): 0 means call-site default.
- `func features.FastASTMaxBytes`
  (`internal/features/features.go:568-576`): 0 means call-site default.

## File: `internal/features/schema.go` (implementation)

`func features.ConfigSchemaJSON` (`internal/features/schema.go:21-52`)
builds the documented `features`-block JSON snippet from
`boolFlags`/`intFlags`, with `//` comments, so it cannot drift from the
accessors. `func features.ConfigSchemaKeys`
(`internal/features/schema.go:56-65`) returns every recognised key.
`func features.envHint` (`internal/features/schema.go:67-72`) formats the
env hint per key.

## Files: tests (what each locks in)

Six `*_test.go` files pin the registry; no production behavior ships from
them, but they are the executable contract for the above:

- `internal/features/config_roundtrip_test.go`: `resetActive`
  (`internal/features/config_roundtrip_test.go:24-28`) and `writeUserConfig`
  (`internal/features/config_roundtrip_test.go:33-41`) support three
  round-trips: `TestLoadUserConfig_InstallsFeaturesIntoRegistry`
  (`internal/features/config_roundtrip_test.go:48-111`),
  `TestEnvOverridesActiveConfig`
  (`internal/features/config_roundtrip_test.go:117-139`),
  `TestFullyEnabledConfigRoundTrip`
  (`internal/features/config_roundtrip_test.go:145-163`), plus integer
  overrides in `TestNumericOverrides`
  (`internal/features/config_roundtrip_test.go:169-188`).
- `internal/features/features_defaults_test.go`:
  `TestDefaultFeaturesConfig`
  (`internal/features/features_defaults_test.go:5-22`),
  `TestParseInt64`
  (`internal/features/features_defaults_test.go:24-38`), and
  `TestFeaturesErrError`
  (`internal/features/features_defaults_test.go:40-44`).
- `internal/features/features_test.go`:
  `TestResolveBoolPrecedence`
  (`internal/features/features_test.go:12-67`),
  `TestPerShardFactsPrecedence`
  (`internal/features/features_test.go:74-92`),
  `TestSystemShardsLegacyEnvIgnored`
  (`internal/features/features_test.go:98-108`),
  `TestSetActiveCopySemantics`
  (`internal/features/features_test.go:112-126`),
  `TestNumericAccessors`
  (`internal/features/features_test.go:131-152`),
  `TestSummaryRendersBoolPointersAsValues`
  (`internal/features/features_test.go:159-243`), and `TestBoolPtrString`
  (`internal/features/features_test.go:246-257`).
- `internal/features/migration_test.go`: the `NERD_*` to `CODENERD_*`
  migration matrix, including
  `TestEnvMigration_WhenOnlyTheLegacyVarIsSet_ShouldStillHonourIt`
  (`internal/features/migration_test.go:12-22`),
  `TestEnvMigration_WhenBothVarsAreSet_ShouldPreferTheCanonicalOne`
  (`internal/features/migration_test.go:24-32`),
  `TestResolved_WhenALegacyVarDecides_ShouldReportLegacyEnvSource`
  (`internal/features/migration_test.go:57-71`),
  `TestDeprecations_WhenALegacyVarIsSet_ShouldNameTheReplacement`
  (`internal/features/migration_test.go:91-105`),
  `TestFastScanWorkers_ShouldDualReadTheLegacyVar`
  (`internal/features/migration_test.go:126-137`), the canonical-prefix
  ratchet `TestEnvMigration_EveryCanonicalVarShouldUseTheCodenerdPrefix`
  (`internal/features/migration_test.go:155-167`), and the scope pin
  `TestEnvMigration_LegacyVarsShouldBeTheKnownFour`
  (`internal/features/migration_test.go:172-198`).
- `internal/features/resolved_test.go`:
  `TestResolved_PrecedenceMatrix`
  (`internal/features/resolved_test.go:11-102`),
  `TestResolved_ShouldMatchAccessors`
  (`internal/features/resolved_test.go:140-169`),
  `TestSummary_ShouldBeSingleLineAndPointerFree`
  (`internal/features/resolved_test.go:172-186`),
  `TestSetActive_ConcurrentWithReads`
  (`internal/features/resolved_test.go:189-209`), and
  `TestSetActive_ShouldCopyTheConfig`
  (`internal/features/resolved_test.go:213-226`).
- `internal/features/schema_test.go`:
  `TestConfigSchemaJSON_ShouldListEveryRecognisedKey`
  (`internal/features/schema_test.go:11-22`),
  `TestConfigSchemaJSON_ShouldNameEveryEnvVar`
  (`internal/features/schema_test.go:24-40`),
  `TestConfigSchemaKeys_ShouldMatchTheJSONTags`
  (`internal/features/schema_test.go:44-63`),
  `TestPerShardFacts_ShouldRemainOptInEvenWhenFullyEnabled`
  (`internal/features/schema_test.go:72-96`), and
  `TestPerShardFacts_ShouldStillHonourAnExplicitOptIn`
  (`internal/features/schema_test.go:101-120`).

## Reachable today (caller verified from Go this pass)

- Install: `LoadUserConfig` calls `features.SetActive(cfg.Features)`
  (`internal/config/user_config.go:567`), logs `features.Summary()`
  (`internal/config/user_config.go:571`), and warns on each of
  `features.Deprecations()` (`internal/config/user_config.go:576`).
  `LoadUserConfig` itself (`internal/config/user_config.go:518-599`)
  rejects removed keys before the strict decode.
- Boot order: `main` eagerly loads config so the registry is populated
  before any gate reads it (`cmd/nerd/main.go:365-378`).
- Flight recorder: `main` starts the trace ring only when
  `features.IsFlightRecorderEnabled()` holds and the invocation is not a
  campaign (`cmd/nerd/main.go:395`).
- Inspection: `nerd features` renders `features.Resolved()`
  (`cmd/nerd/cmd_features.go:44`), `features.Deprecations()`
  (`cmd/nerd/cmd_features.go:45`), both tunables
  (`cmd/nerd/cmd_features.go:52-53`), and `--schema` prints
  `features.ConfigSchemaJSON()` (`cmd/nerd/cmd_features.go:37`). Chat
  `/features` renders the same three surfaces via
  `features.Resolved()`
  (`cmd/nerd/chat/commands_handlers_features.go:19`), the tunables
  (`cmd/nerd/chat/commands_handlers_features.go:36-38`),
  `features.Deprecations()`
  (`cmd/nerd/chat/commands_handlers_features.go:40`), and
  `features.Summary()`
  (`cmd/nerd/chat/commands_handlers_features.go:49`).
- Scan tunables: `world.DefaultScannerConfig`
  (`internal/world/scanner_config.go:29-38`) takes workers from
  `features.FastScanWorkers()` (`internal/world/scanner_config.go:31`)
  and the AST cutoff from `features.FastASTMaxBytes()`
  (`internal/world/scanner_config.go:36`), defaulting to
  `max(min(NumCPU,20),4)` workers and 2 MiB when they return 0.
- Router construction: `core.NewCortexKernel`
  (`internal/core/cortex_kernel.go:111-122`) builds the `ShardFactRouter`
  only when `features.IsPerShardFactsEnabled()` holds
  (`internal/core/cortex_kernel.go:119`); otherwise the field stays nil
  and the legacy single-store path runs.
- Onboarding: `ux.ShouldShowOnboarding`
  (`internal/ux/migration.go:190-208`) returns false when
  `features.IsOnboardingSkipped()` holds
  (`internal/ux/migration.go:192`), before touching the workspace.
- Taxonomy fast path: `verify_taxonomy` reads the flag through the
  registry via `features.IsTaxonomyFastEnabled()`
  (`cmd/tools/verify_taxonomy/main.go:17`) rather than raw env.
- Provenance symbol distinction: `features.IsProvenanceEnabled` is a
  package function (`internal/features/features.go:479-482`), a different
  symbol from the kernel method `(*RealKernel).IsProvenanceEnabled`
  (`internal/core/kernel_provenance.go:49-54`).

## Removed, not shipped

`features.diff_eval` is gone: the differential-evaluation path was deleted
and the kernel always rebuilds from the EDB, so a config still carrying
the key fails the load with a named reason
(`internal/config/removed_keys.go:24-31`).

## Not claimed here

Prod callers for `IsSystemShardsEnabled`, `IsDarkModeEnabled`,
`IsPromptEvolutionEnabled`, `DefaultFeaturesConfig`, and
`FullyEnabledFeaturesConfig`, and the zero-prod-caller status of
`IsProvenanceEnabled`, were not re-verified from Go this pass and are not
asserted here. See `WIRING-AND-NOT-BUILT.md` for the scoped search and the
hypothesized Phase-1 importers awaiting a full-repo `callers_of` check.
