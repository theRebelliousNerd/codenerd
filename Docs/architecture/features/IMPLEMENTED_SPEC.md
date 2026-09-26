---
doc-class: shipped
subsystem: features
implementation-status: shipped
last-verified: 2026-09-21
verified-against: 77fb2f6e1c47f7a06856be57b9fd8d0eb19fe150c9d9b8dce8362121aba9d143
supersedes: []
---

# features — IMPLEMENTED_SPEC (authoritative shipped record)

> This file wins on disagreement. When `README.md`, `INTERNALS.md`,
> `WIRING-AND-NOT-BUILT.md`, or `Docs/architecture/shards/` disagree with this
> file about what `internal/features` does today, this file is correct.
> All claims below were verified against code at `verified-against` above.
> Plan-layer intent lives in `01-VISION.md` / `05-*` specs, never here.

## 1. Package role (leaf toggle registry)

`internal/features` is a leaf-level toggle registry so `internal/core` can read
flags without an import cycle with `internal/config`
(`internal/features/features.go:1-13`).

- Layering: `internal/config` loads `.nerd/config.json` then calls
  `features.SetActive` (`internal/features/features.go:233-241`,
  installed at `internal/config/user_config.go:567`); `internal/core`,
  `internal/observability`, `internal/world` read via `features.Is*` /
  `features.Fast*` accessors (`internal/features/features.go:419-434`).
- Constraint: depends on nothing inside codeNERD
  (`internal/features/features.go:12-13`).
- Files: impl (2): `internal/features/features.go`,
  `internal/features/schema.go` (`internal/features/schema.go:21-52`,
  `internal/features/schema.go:56-65`); tests (6):
  `internal/features/config_roundtrip_test.go`,
  `internal/features/features_defaults_test.go`,
  `internal/features/features_test.go`,
  `internal/features/migration_test.go`,
  `internal/features/resolved_test.go`,
  `internal/features/schema_test.go`.

## 2. Config shape (`FeaturesConfig`)

`struct features.FeaturesConfig` (`internal/features/features.go:69-138`) carries
the on-disk JSON shape. All bools are `*bool` to distinguish absent vs `false`
(`internal/features/features.go:64-68`):

| Field | JSON key | Env (canonical / legacy) | Lines |
|---|---|---|---|
| `FlightRecorder` | `flight_recorder` | `CODENERD_FLIGHT_RECORDER` / `NERD_FLIGHTREC` | `internal/features/features.go:73` |
| `Provenance` | `provenance` | `CODENERD_PROVENANCE` only | `internal/features/features.go:79` |
| `SystemShards` | `system_shards` | `CODENERD_SYSTEM_SHARDS` | `internal/features/features.go:86` |
| `PerShardFacts` | `per_shard_facts` | `CODENERD_PER_SHARD_FACTS` | `internal/features/features.go:95` |
| `DarkMode` | `dark_mode` | `CODENERD_DARK_MODE` | `internal/features/features.go:99` |
| `SkipOnboarding` | `skip_onboarding` | `CODENERD_SKIP_ONBOARDING` / `NERD_SKIP_ONBOARDING` | `internal/features/features.go:103` |
| `TaxonomyFast` | `taxonomy_fast` | `CODENERD_TAXONOMY_FAST` | `internal/features/features.go:110` |
| `PromptEvolution` | `prompt_evolution` | `CODENERD_PROMPT_EVOLUTION` | `internal/features/features.go:127` |
| `FastScanWorkers int` | `fast_scan_workers` | `CODENERD_FAST_SCAN_WORKERS` / `NERD_FAST_SCAN_WORKERS`, 0=default | `internal/features/features.go:132` |
| `FastASTMaxBytes int64` | `fast_ast_max_bytes` | `CODENERD_FAST_AST_MAX_BYTES` / `NERD_FAST_AST_MAX_BYTES`, 0=default | `internal/features/features.go:137` |

Single source of truth for the 8 bools is `var features.boolFlags`
(`internal/features/features.go:276-291`); for the 2 ints
`var features.intFlags` (`internal/features/features.go:296-303`).

### 2.1 Compile-time defaults

- `func features.DefaultFeaturesConfig`
  (`internal/features/features.go:156-168`): conservative defaults when no
  config + no env — all `false` except `SystemShards:true`. Rationale:
  Provenance allocates per-derivation, FlightRecorder can OOM via
  `throw(traceRegion)` (`internal/features/features.go:140-151`,
  `internal/features/features.go:460-467`).
- `func features.FullyEnabledFeaturesConfig`
  (`internal/features/features.go:202-215`): what `nerd init` writes — all
  `true` except `PerShardFacts:false`, `PromptEvolution:false`. PerShardFacts
  stays false per 2026-08-15 audit: `ShardFactRouter` is dispatch-only, no
  cross-shard joins (`internal/features/features.go:175-196`). PromptEvolution
  stays false for cost (`internal/features/features.go:112-127`).

Shipped wiring: `FullyEnabledFeaturesConfig` is called by prod
`config.DefaultUserConfig` (`internal/config/user_config.go:1494`);
`DefaultFeaturesConfig` has no prod caller — sole exact caller is
`internal/features/features_defaults_test.go:6`
(`features.TestDefaultFeaturesConfig`). It is exists-but-uncalled, not wired.

## 3. Active registry (no snapshots)

- `var features.active atomic.Pointer[FeaturesConfig]`
  (`internal/features/features.go:221`) — wait-free reads for hot paths.
- `func features.SetActive` (`internal/features/features.go:233-241`) —
  installs config from `.nerd/config.json`; copies struct; `nil` resets to
  defaults. Caller must log via `Summary()` (leaf cannot import logging).
  Sole prod installer is `config.LoadUserConfig`
  (`internal/config/user_config.go:567`); test helper
  `system.bootKernelForShardModeTest`
  (`internal/system/factory_kernel_shards_test.go:35-36`); remaining 70+ sites
  are tests.
- `func features.Active` (`internal/features/features.go:408`) — returns
  `active.Load()`, nil if unset; caller must not mutate. Exactly 1 caller
  total: `internal/features/features_test.go:122`
  (`features.TestSetActiveCopySemantics`). Prod code never calls `Active()`
  directly by design — it goes via `Is*` / `Fast*` accessors.

## 4. Resolution model (four layers + strict parsing)

- `type features.Source string` (`internal/features/features.go:244`) +
  consts `SourceEnv=env` (`internal/features/features.go:248`),
  `SourceLegacyEnv=legacy-env` (`internal/features/features.go:252`),
  `SourceConfig=config` (`internal/features/features.go:254`),
  `SourceDefault=default` (`internal/features/features.go:256`).
- `struct features.Flag` (`internal/features/features.go:261-268`) —
  `Name, EnvVar, LegacyEnvVar, Value, Source, Default`.
- `func features.resolveBool` (`internal/features/features.go:419-434`) —
  precedence canonical-env → legacy-env → active → default. Every accessor
  resolves via line 428.
- `func features.envBool` (`internal/features/features.go:444-458`) —
  `1/true` (case-insensitive) → true, `0/false` → false, else nil (stray export
  never flips the bit).
- `func features.envInt` (`internal/features/features.go:581-595`) —
  canonical preferred, then legacy; non-numeric / non-positive = no override.
  `func features.parseInt64` (`internal/features/features.go:597-609`) —
  digits-only.
- `func features.Resolved` (`internal/features/features.go:308-328`) — every
  bool as accessors resolve it, plus winning source. Prod callers:
  `chat.renderFeaturesReport`
  (`cmd/nerd/chat/commands_handlers_features.go:19`) and
  `features.Summary` (`internal/features/features.go:376`); tests mirror it
  (`cmd/nerd/cmd_features_test.go:33,70-71`).
- `func features.Deprecations` (`internal/features/features.go:341-363`) —
  legacy `NERD_*` in use with canonical replacement; reports even shadowed
  ones. Prod callers: `config.LoadUserConfig`
  (`internal/config/user_config.go:576`) and `chat.renderFeaturesReport`
  (`cmd/nerd/chat/commands_handlers_features.go:40`).
- `func features.Misconfigurations` (`internal/features/features.go:379`) —
  every feature env var set to a value the registry refuses (unparseable canonical value is no override, still reported). Warn-only: preserves the no-flip guarantee. Prod callers: `config.LoadUserConfig` warns with each at boot (`internal/config/user_config.go:612`), `nerd features` prints them (`cmd/nerd/cmd_features.go:52`), `/features` prints them (`cmd/nerd/chat/commands_handlers_features.go:46`). Lane-B wiring since 2026-09-25 (D5).
- `func features.Summary` (`internal/features/features.go:374-391`) —
  single-line resolved values, `name=value(source)` for non-default +
  `fast_scan_workers`, `fast_ast_max_bytes`.
  Prod callers: `chat.renderFeaturesReport`
  (`cmd/nerd/chat/commands_handlers_features.go:49`) and
  `config.LoadUserConfig` boot log (`internal/config/user_config.go:571`).
- `func features.boolPtrString` (`internal/features/features.go:395-403`) —
  unexported log helper, nil → `unset`.
- `var features.errBadInt`, `type features.featuresErr`,
  `func features.newErr` (`internal/features/features.go:611-616`) — sentinel
  for int parse.

## 5. Accessors — behavior + exact prod wiring

| Accessor | Default | Env | Prod caller(s) (exact) |
|---|---|---|---|
| `func features.IsFlightRecorderEnabled` (`internal/features/features.go:471-474`) | OFF | `CODENERD_FLIGHT_RECORDER` / `NERD_FLIGHTREC` | `main.main` (`cmd/nerd/main.go:395`) gates trace ring buffer + watchdog; only prod caller |
| `func features.IsProvenanceEnabled` (`internal/features/features.go:479-482`) | OFF | `CODENERD_PROVENANCE` | Wired: `system.NewDomainCortex` enables derivation recording in every shard when it holds, before the first evaluation (`internal/system/factory.go:1223` `if features.IsProvenanceEnabled()` → `cortex.EnableProvenance()`). Different symbol from `(*RealKernel).IsProvenanceEnabled` method (`internal/core/kernel_provenance.go:49-54`). History: observed 2026-09-21 true-then zero prod callers (only `internal/features/*_test.go` + `config_roundtrip_test.go`); lane-B wiring since 2026-09-25 supersedes that clause — history preserved, never deleted |
| `func features.IsSystemShardsEnabled` (`internal/features/features.go:498-501`) | ON | `CODENERD_SYSTEM_SHARDS` | `system.initShardManagement` (`internal/system/factory.go:1895`); only prod caller. Per-shard disable is `--disable-system-shard` flag, not env (`internal/features/features.go:484-497`) |
| `func features.IsPerShardFactsEnabled` (`internal/features/features.go:509-512`) | OFF | `CODENERD_PER_SHARD_FACTS` | `core.NewCortexKernel` (`internal/core/cortex_kernel.go:119`) + `system.initKernel` (`internal/system/factory.go:1180`) |
| `func features.IsDarkModeEnabled` (`internal/features/features.go:516-519`) | OFF | `CODENERD_DARK_MODE` | `ui.detectTheme` (`cmd/nerd/ui/styles.go:293`) |
| `func features.IsOnboardingSkipped` (`internal/features/features.go:522-525`) | OFF | `CODENERD_SKIP_ONBOARDING` / `NERD_SKIP_ONBOARDING` | `ux.ShouldShowOnboarding` (`internal/ux/migration.go:192`); early-false (`internal/ux/migration.go:190-208`) |
| `func features.IsTaxonomyFastEnabled` (`internal/features/features.go:535-538`) | OFF, skips scenario sweep | `CODENERD_TAXONOMY_FAST` | `main.main` of verify_taxonomy (`cmd/tools/verify_taxonomy/main.go:17`) |
| `func features.IsPromptEvolutionEnabled` (`internal/features/features.go:549-552`) | OFF (cost) | `CODENERD_PROMPT_EVOLUTION` | `system.initLearningLoop` (`internal/system/factory_learning.go:144`) + `system.Cortex.runEvolutionCycle` (`internal/system/factory_learning.go:320`) |
| `func features.FastScanWorkers` (`internal/features/features.go:557-565`) | 0=default | `CODENERD_FAST_SCAN_WORKERS` / legacy | `world.DefaultScannerConfig` (`internal/world/scanner_config.go:31`) + `chat.renderFeaturesReport` (`cmd/nerd/chat/commands_handlers_features.go:36`) + `features.Summary` (`internal/features/features.go:387`) |
| `func features.FastASTMaxBytes` (`internal/features/features.go:568-576`) | 0=default | `CODENERD_FAST_AST_MAX_BYTES` / legacy | `world.DefaultScannerConfig` (`internal/world/scanner_config.go:36`) + `chat.renderFeaturesReport` (`cmd/nerd/chat/commands_handlers_features.go:38`) + `features.Summary` (`internal/features/features.go:388`) |

Behavioral notes that are load-bearing:

- `DefaultScannerConfig` takes `FastScanWorkers/FastASTMaxBytes`, defaulting
  to `max(min(NumCPU,20),4)` / 2 MiB on 0
  (`internal/world/scanner_config.go:29-38`).
- `NewCortexKernel` builds `ShardFactRouter` only when
  `IsPerShardFactsEnabled()` (`internal/core/cortex_kernel.go:97-123`).
- `main` starts the flight-recorder ring only when
  `IsFlightRecorderEnabled()` and not campaign (`cmd/nerd/main.go:395`),
  default OFF with OOM rationale
  (`internal/features/features.go:460-474`).

## 6. Install + inspect surfaces (wired, reachable)

- `LoadUserConfig` does `SetActive` → `Summary` → `Deprecations` loop
  (`internal/config/user_config.go:567`,
  `internal/config/user_config.go:571`,
  `internal/config/user_config.go:576-578`) and warns refused env values via `Misconfigurations()` (`internal/config/user_config.go:612`, lane-B D5 since 2026-09-25).
- `main` eager-loads before gates (`cmd/nerd/main.go:365-378`).
- `nerd features` renders `Resolved()` + tunables + deprecations
  (`cmd/nerd/cmd_features.go:44-83`) and refused env values (`cmd/nerd/cmd_features.go:52`).
- Chat `/features` renders the same surfaces plus refused env values
  (`cmd/nerd/chat/commands_handlers_features.go:18-50`, misconfigurations at `:46`).

## 7. Schema (generated snippet, not valid JSON)

- `func features.ConfigSchemaJSON` (`internal/features/schema.go:21-52`) —
  built from `boolFlags/intFlags` with `//` comments; documented snippet, not
  parseable JSON. Wired: `nerd features --schema` prints it (`cmd/nerd/cmd_features.go:43`).
- `func features.ConfigSchemaKeys` (`internal/features/schema.go:56-65`) —
  every recognised key of the `features` block. Wired: `nerd features --schema --json` prints it as a JSON array (`cmd/nerd/cmd_features.go:41`).
- Shipped status: both wired in prod (lane-B wiring since 2026-09-25 supersedes the prior exists-but-uncalled clause — history preserved, never deleted). History: observed 2026-09-21 true-then both test-only — exact callers of
  `ConfigSchemaJSON` were 3 test sites
  (`cmd/nerd/cmd_features_test.go:49`,
  `internal/features/schema_test.go:12`,
  `internal/features/schema_test.go:25`); exact callers of
  `ConfigSchemaKeys` were 2 test sites
  (`internal/features/schema_test.go:14`,
  `internal/features/schema_test.go:54`).

## 8. Discarded claims (do not re-add without code)

- `diff_eval` is removed with reason
  (`internal/config/removed_keys.go:24-31`) — any doc claiming it as a live
  feature flag is stale.
- `NERD_DISABLE_SYSTEM_SHARDS` appears in no `.go` per accessor comment
  (`internal/features/features.go:493-497`) — the canonical switch is
  `IsSystemShardsEnabled` + `--disable-system-shard`.
- Taxonomy TODO-vs-SPEC contradiction is resolved to wired-OFF
  (`internal/features/features.go:527-538`).
- July-2026 `Docs/architecture/` corpus anchors at `~L237-285` for accessors
  are stale vs today `471-552`; `user_config.go:561-572` anchors drift to
  `567/571/576`; `main.go:387` flight gate drifts to `395`;
  `main.go:363-370` load drifts to `365-378`. Behaviors held; lines moved.

## 9. Wired vs exists-but-uncalled vs assumed-but-not-done

- **Wired/reachable:** `SetActive`, `Resolved`, `Summary`, `Deprecations`, `Misconfigurations` (warned at boot `internal/config/user_config.go:612`, printed by `nerd features` `cmd/nerd/cmd_features.go:52` and `/features`),
  `IsFlightRecorderEnabled`, `IsProvenanceEnabled` (via `internal/system/factory.go:1223`), `IsSystemShardsEnabled`,
  `IsPerShardFactsEnabled`, `IsDarkModeEnabled`, `IsOnboardingSkipped`,
  `IsTaxonomyFastEnabled`, `IsPromptEvolutionEnabled`, `FastScanWorkers`,
  `FastASTMaxBytes`, `ConfigSchemaJSON` (via `cmd/nerd/cmd_features.go:43`), `ConfigSchemaKeys` (via `cmd/nerd/cmd_features.go:41`), `FullyEnabledFeaturesConfig` (via
  `internal/config/user_config.go:1494`). Each row in §5 names its prod
  caller; lane-B wiring since 2026-09-25 supersedes the prior exists-but-uncalled clauses for `IsProvenanceEnabled`, `ConfigSchemaJSON`, and `ConfigSchemaKeys` — history preserved, never deleted.
- **Exists-but-uncalled:** `Active`
  (`internal/features/features.go:408`, 1 caller
  `internal/features/features_test.go:122`);
  `DefaultFeaturesConfig`
  (`internal/features/features.go:156-168`, 1 caller
  `internal/features/features_defaults_test.go:6`).
  These are deliberate (leaf read discipline, init-only defaults) unless a gap ID promotes one to wired. History: `IsProvenanceEnabled`, `ConfigSchemaJSON`, and `ConfigSchemaKeys` were exists-but-uncalled observed 2026-09-21 true-then; that clause is superseded by the wired entries above.
- **Assumed-but-not-done:** nothing in this package. Cross-package claims
  that `Docs/architecture/shards/07-DEPENDENCY-MAP.md:77` or
  `08-WIRING-AND-INTEGRATION.md:59` wire the master switch are doc claims,
  not re-verified from Go here — the verified switch caller is
  `internal/system/factory.go:1895`.

## 10. Proving tests (command-checkable)

- Precedence + migration: `internal/features/features_test.go`,
  `internal/features/migration_test.go`,
  `internal/features/resolved_test.go` (canonical → legacy → active →
  default; `envBool`/`envInt` strictness).
- Round-trip + install: `internal/features/config_roundtrip_test.go`
  (incl. `TestLoadUserConfig_InstallsFeaturesIntoRegistry`,
  `TestFullyEnabledConfigRoundTrip`, `TestNumericOverrides`).
- Defaults + schema: `internal/features/features_defaults_test.go`,
  `internal/features/schema_test.go`.
- Surfaces: `cmd/nerd/cmd_features_test.go`,
  `cmd/nerd/chat/commands_handlers_features_test.go`.
- Run: `go test ./internal/features/... ./internal/config/... ./cmd/nerd/...`
  with `CGO_CFLAGS=-IC:/CodeProjects/codeNERD/sqlite_headers`.
