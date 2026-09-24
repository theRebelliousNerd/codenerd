---
doc-class: shipped-with-future
subsystem: features
implementation-status: partial
last-verified: 2026-09-21
verified-against: 34634770970153e78c1e250fdab7abd888dcce6f
supersedes: []
---

# 05 — Capability spec: lifecycle (install and resolve)

> First capability spec for `internal/features`. Lifecycle means install and
> resolve: `FeaturesConfig` shape, defaults, `SetActive` / `Active`, the
> `boolFlags` / `intFlags` tables, `resolveBool` / `envBool` / `envInt`
> precedence, and the `Is*` / `Fast*` accessors. Evaluation (turning resolved
> state into inspectable views: `Resolved`, `Summary`, `Deprecations`, schema)
> belongs to `06-*` and is not repeated here.
> Shipped claims below cite code read 2026-09-21; planned behaviour is marked
> planned and never stated as shipped.

## 1. Capability in one paragraph

Given a file config, environment overrides, and compile-time defaults,
lifecycle answers "what value does this flag have right now" on every call
with no snapshot to go stale: `internal/config` installs once via
`features.SetActive`, any leaf package reads forever via `features.Is*` /
`features.Fast*`, and precedence is always canonical env, then legacy env,
then active config, then default. The finished behaviour is that every flag
has exactly one table row, one accessor, and one default that agree by
construction; every installed value is visible to subsequent reads; no stray
export ever flips a bit; and no flag parses and resolves while nothing reads
it without an explicit reserved mark.

## 2. Data shapes (shipped)

All shapes live in `internal/features`; all citations are symbols observed at
`verified-against`.

- `struct features.FeaturesConfig` (`internal/features/features.go:69-138`)
  is the 10-field on-disk JSON shape. All booleans are `*bool` to distinguish
  absent from `false` (`internal/features/features.go:64-68`):
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
    (`internal/features/features.go:132`), env `CODENERD_FAST_SCAN_WORKERS`,
    legacy `NERD_FAST_SCAN_WORKERS`, 0 means call-site default.
  - `FastASTMaxBytes int64` `json:fast_ast_max_bytes`
    (`internal/features/features.go:137`), env `CODENERD_FAST_AST_MAX_BYTES`,
    legacy `NERD_FAST_AST_MAX_BYTES`, 0 means call-site default.
- `func features.DefaultFeaturesConfig`
  (`internal/features/features.go:156-168`) returns the conservative
  compile-time defaults used when no config and no env are present: all
  `false` except `SystemShards:true`. Rationale is load-bearing: Provenance
  allocates per-derivation and FlightRecorder can abort via
  `throw(traceRegion)` (`internal/features/features.go:140-151`,
  `internal/features/features.go:460-467`).
- `func features.FullyEnabledFeaturesConfig`
  (`internal/features/features.go:202-215`) is what `nerd init` writes: all
  `true` except `PerShardFacts:false` and `PromptEvolution:false`.
  PerShardFacts stays false per the 2026-08-15 audit recorded in the comment:
  `ShardFactRouter` is dispatch-only with no cross-shard joins
  (`internal/features/features.go:175-196`). PromptEvolution stays false for
  cost (`internal/features/features.go:112-127` and
  `internal/features/features.go:540-552`).
- `var features.active atomic.Pointer[FeaturesConfig]`
  (`internal/features/features.go:221`) holds the installed config for
  wait-free reads on hot paths.
- `func features.SetActive` (`internal/features/features.go:233-241`)
  installs the config parsed from `.nerd/config.json`; it copies the struct
  so callers cannot mutate the live pointer, and `nil` resets to defaults.
  The caller must log via `Summary()` because the leaf cannot import logging
  (`internal/features/features.go:227-232`).
- `func features.Active` (`internal/features/features.go:408`) returns
  `active.Load()`, nil when unset; callers must not mutate the result.
  Production code reaches flags through the `Is*` / `Fast*` accessors, not
  through `Active()` directly.
- `var features.boolFlags` (`internal/features/features.go:276-291`) is the
  single source of truth for the 8 booleans (name, canonical env, legacy env,
  getter, default). `var features.intFlags`
  (`internal/features/features.go:296-303`) is the same for the 2 integer
  overrides.
- `func features.resolveBool` (`internal/features/features.go:419-434`)
  implements canonical-env to legacy-env to active-config to default, in that
  order. The legacy name is read only when the canonical one is absent or
  unparseable (`internal/features/features.go:416-418`).
- `func features.envBool` (`internal/features/features.go:444-458`) accepts
  only `1` / `true` and `0` / `false` case-insensitive
  (`internal/features/features.go:451-454`), else nil ("no override",
  `internal/features/features.go:456-457`), so a stray export never flips a
  bit.
- `func features.envInt` (`internal/features/features.go:581-595`) prefers
  canonical then legacy; non-numeric or non-positive values are no override.
  `func features.parseInt64` (`internal/features/features.go:597-609`)
  accepts digits only. `var features.errBadInt`,
  `type features.featuresErr`, and `func features.newErr`
  (`internal/features/features.go:611-616`) are the integer-parse sentinel.
- Accessors (all resolve through `resolveBool` / `envInt`):
  - `func features.IsFlightRecorderEnabled`
    (`internal/features/features.go:471-474`): default OFF,
    `CODENERD_FLIGHT_RECORDER` / `NERD_FLIGHTREC`.
  - `func features.IsProvenanceEnabled`
    (`internal/features/features.go:479-482`): default OFF,
    `CODENERD_PROVENANCE`, gates the DerivationRecorder for `/explain`.
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
    (`internal/features/features.go:557-565`): env, then active, then 0.
  - `func features.FastASTMaxBytes`
    (`internal/features/features.go:568-576`): env, then active, then 0.
- Adding a flag means three things in lockstep
  (`internal/features/features.go:46-54`): field on `FeaturesConfig`, public
  `IsXXX` helper with env/config/default precedence, default in
  `DefaultFeaturesConfig`.

Evaluation dependency (anchor only, spec lives in `06-*`):
`type features.Source string` (`internal/features/features.go:244`) with
`SourceEnv` / `SourceLegacyEnv` / `SourceConfig` / `SourceDefault`
(`internal/features/features.go:248`,
`internal/features/features.go:252`,
`internal/features/features.go:254`,
`internal/features/features.go:256`), `struct features.Flag`
(`internal/features/features.go:261-268`),
`func features.Resolved` (`internal/features/features.go:308-328`),
`func features.Deprecations` (`internal/features/features.go:341-363`),
`func features.Summary` (`internal/features/features.go:374-391`).

## 3. Go / Mangle split

Today this capability is Go-only, by constraint. `internal/features` exists
as its own package so low-level subsystems can read flags without an import
cycle with `internal/config` (`internal/features/features.go:1-13`,
layering rule at `internal/features/features.go:7-11`). The package depends
on nothing inside codeNERD (`internal/features/features.go:12-13`) and
imports only stdlib (`internal/features/features.go:57-62`: `fmt`, `os`,
`strings`, `sync/atomic`).

There are no Mangle predicates for lifecycle today. The planned direction
(Rule 2: planned, anchored, not asserted as shipped) is that the two future
signals below — "provenance flag flips the kernel proof path" and "every
flag has a reader or an explicit reserved mark" — become derived obligations
(a predicate that derives, a gate that reaches zero) rather than conventions
in Go. No predicate is named here as shipped; see §8 for the
command-checkable exits that would precede any predicate.

## 4. Current seams (where this capability attaches)

Shipped, each with an exact prod caller verified at `verified-against`
(full sets in `Docs/architecture/features/WIRING-AND-NOT-BUILT.md:15-85`):

- Install: `config.LoadUserConfig` calls `features.SetActive(cfg.Features)`
  (`internal/config/user_config.go:567` in `config.LoadUserConfig`,
  `internal/features/features.go:233-241`), then logs `features.Summary()`
  (`internal/config/user_config.go:571`) and warns on each of
  `features.Deprecations()` (`internal/config/user_config.go:576-578`).
- Seed config: `config.DefaultUserConfig` returns
  `features.FullyEnabledFeaturesConfig()` (`internal/config/user_config.go:1494`;
  `internal/features/features.go:202-215`) — what `nerd init` writes.
- Boot order: `main` loads config eagerly precisely so the registry is
  populated before any gate reads it (`cmd/nerd/main.go:365-378`).
- Flight recorder: `main` starts the tracer ring only when
  `features.IsFlightRecorderEnabled()` holds and the invocation is not a
  campaign (`cmd/nerd/main.go:395`; accessor
  `internal/features/features.go:471-474`).
- Onboarding: `ux.ShouldShowOnboarding` returns false when
  `features.IsOnboardingSkipped()` holds, before touching the workspace
  (`internal/ux/migration.go:192` in `ux.ShouldShowOnboarding`;
  `internal/ux/migration.go:190-208`; accessor
  `internal/features/features.go:522-525`).
- Scan tunables: `world.DefaultScannerConfig` takes workers and the AST
  cutoff from `features.FastScanWorkers()` / `features.FastASTMaxBytes()`,
  defaulting to `max(min(NumCPU,20),4)` / 2 MiB when they return 0
  (`internal/world/scanner_config.go:31,36` in
  `world.DefaultScannerConfig`; `internal/world/scanner_config.go:29-38`;
  accessors `internal/features/features.go:557-565` and
  `internal/features/features.go:568-576`).
- Kernel router construction: `core.NewCortexKernel` builds the
  `ShardFactRouter` only when `features.IsPerShardFactsEnabled()` holds
  (`internal/core/cortex_kernel.go:119` in `core.NewCortexKernel`;
  `internal/core/cortex_kernel.go:97-123`; accessor
  `internal/features/features.go:509-512`). Second prod reader:
  `system.initKernel` (`internal/system/factory.go:1180`).
- System-shard master switch: `system.initShardManagement` reads
  `features.IsSystemShardsEnabled()` (`internal/system/factory.go:1895`;
  accessor `internal/features/features.go:498-501`).
- Dark mode: `ui.detectTheme` returns `DarkTheme()` when
  `features.IsDarkModeEnabled()` holds (`cmd/nerd/ui/styles.go:293`;
  accessor `internal/features/features.go:516-519`).
- Fast taxonomy: `cmd/tools/verify_taxonomy/main.go` short-circuits the
  scenario sweep when `features.IsTaxonomyFastEnabled()` holds
  (`cmd/tools/verify_taxonomy/main.go:17` in `main.main`; accessor
  `internal/features/features.go:535-538`).
- Prompt evolution: `system.initLearningLoop` gates the background cycle on
  `features.IsPromptEvolutionEnabled()`
  (`internal/system/factory_learning.go:144`) and
  `system.Cortex.runEvolutionCycle` re-checks it
  (`internal/system/factory_learning.go:320`; accessor
  `internal/features/features.go:549-552`). Recording is unconditional; the
  flag gates only who pushes the button
  (`internal/features/features.go:540-548`). Former assumed seam
  `cmd/nerd/cmd_systems.go:12` resolved to reachable via
  `cmd/nerd/cmd_systems.go:300` inside var `main.autopoiesisStatusCmd`
  (`cmd/nerd/cmd_systems.go:257-353`).

Exists-but-uncalled in prod (shipped code, test-only callers — the gaps in
§8):

- `func features.IsProvenanceEnabled`
  (`internal/features/features.go:479-482`) — zero prod callers; only
  `internal/features/*_test.go` + `config_roundtrip_test.go`. This is a
  different symbol from the kernel method
  `(*RealKernel).IsProvenanceEnabled`
  (`internal/core/kernel_provenance.go:50-54`). The one chat caller observed
  uses the kernel's, not the flag's
  (`cmd/nerd/chat/commands_handlers_misc.go:101` in
  `chat.Model.handleExplainCommand`).
- `func features.Active` (`internal/features/features.go:408`) — 1 exact
  caller total: `internal/features/features_test.go:122` in
  `TestSetActiveCopySemantics`. By design: prod goes via the `Is*` / `Fast*`
  accessors, never `Active()` directly.
- `func features.DefaultFeaturesConfig`
  (`internal/features/features.go:156-168`) — 1 caller:
  `internal/features/features_defaults_test.go:6`. Conservative
  compile-time defaults, currently unreferenced in prod
  (`config.DefaultUserConfig` uses `FullyEnabledFeaturesConfig`).

## 5. Failure modes (shipped handling + planned hardening)

- **Stale snapshot.** Prevented shipped: `var features.active`
  (`internal/features/features.go:221`) holds one `*FeaturesConfig`;
  `SetActive` (`internal/features/features.go:233-241`) copies the struct
  (`internal/features/features.go:238-240`) and swaps the pointer, and every
  accessor dereferences it on every call
  (`internal/features/features.go:419-434`, load at line 428) — there is no
  snapshot to go stale, so a late install changes what subsequent reads
  return. Ruling: callers must not mutate the pointer `Active()` returns,
  and must not cache a resolved value across a `SetActive`.
- **Stray export flips a bit.** Prevented shipped: `envBool`
  (`internal/features/features.go:444-458`) trims whitespace and accepts
  exactly `1`/`0` and case-insensitive `true`/`false`; anything else returns
  nil ("no override"). `envInt` applies the same discipline to numbers: only
  positive digit strings count (`internal/features/features.go:581-595` via
  `parseInt64`, `internal/features/features.go:597-609`). Ruling:
  misconfiguration is silent by design at this layer — never coerce an
  unrecognized value to true.
- **Absent-vs-false collapse.** Prevented shipped: every boolean field is
  `*bool` so "user wrote `false`" is distinguishable from "key absent, use
  the default" (`internal/features/features.go:64-68`). The integer
  overrides are zero-valued instead: zero means "call site picks"
  (`internal/features/schema.go:38-39`); `FastScanWorkers`
  (`internal/features/features.go:557-565`) and `FastASTMaxBytes`
  (`internal/features/features.go:568-576`) return env, then active, then
  `0`. Ruling: never change a `*bool` field to plain `bool`, and never
  interpret `0` as a configured count or limit.
- **Dangerous or expensive defaults on.** Prevented shipped:
  `DefaultFeaturesConfig` (`internal/features/features.go:156-168`) is
  intentionally conservative — all false except `SystemShards:true` — and
  `FullyEnabledFeaturesConfig`
  (`internal/features/features.go:202-215`) keeps `PerShardFacts:false` and
  `PromptEvolution:false` for the audited reasons in §2. Ruling: a new flag
  that allocates, traces, spends tokens, or skips verification defaults off.
- **Table drift.** Prevented shipped: `boolFlags` / `intFlags`
  (`internal/features/features.go:276-291`,
  `internal/features/features.go:296-303`) tie each name, env pair, getter,
  and default together so views cannot drift from the accessors. Ruling:
  never hard-code a flag name, env var, or default outside the tables.
- **Silent recognised-but-unused flag (planned GAP-FEAT-01).**
  `IsProvenanceEnabled` parses and resolves but nothing in prod reads it.
  Either it gates the kernel proof path or it is marked reserved by
  decision. Exit in §8.
- **Two boot truths (planned GAP-FEAT-02).** Two constructors, one prod
  reader (`FullyEnabledFeaturesConfig` via `config.DefaultUserConfig` at
  `internal/config/user_config.go:1494`; `DefaultFeaturesConfig` test-only).
  Needs a decision naming the fresh-boot source of truth, not deletion
  without audit. Exit in §8.
- **Unfenced names (planned GAP-FEAT-04, lifecycle half).** Names are
  trusted, not fenced: nothing stops two packages reading the same flag to
  mean different things, and nothing warns when a flag has no reader — the
  three uncalled symbols above are silent, not errors. The lifecycle half is
  what "reserved" means for resolution; the evaluation half (enumeration
  gate) belongs to `06-*`.

## 6. Proving tests (command-checkable)

Run with `CGO_CFLAGS=-IC:/CodeProjects/codeNERD/sqlite_headers`:

- Precedence: `TestResolveBoolPrecedence`
  (`internal/features/features_test.go:12-67`),
  `TestPerShardFactsPrecedence`
  (`internal/features/features_test.go:74-92`),
  `TestResolved_PrecedenceMatrix`
  (`internal/features/resolved_test.go:11-102`),
  `TestResolved_ShouldMatchAccessors`
  (`internal/features/resolved_test.go:140-169`).
- Install and copy semantics: `TestSetActiveCopySemantics`
  (`internal/features/features_test.go:112-126`),
  `TestSetActive_ShouldCopyTheConfig`
  (`internal/features/resolved_test.go:213-226`),
  `TestSetActive_ConcurrentWithReads`
  (`internal/features/resolved_test.go:189-209`),
  `TestLoadUserConfig_InstallsFeaturesIntoRegistry`
  (`internal/features/config_roundtrip_test.go:48-111`),
  `TestEnvOverridesActiveConfig`
  (`internal/features/config_roundtrip_test.go:117-139`),
  `TestFullyEnabledConfigRoundTrip`
  (`internal/features/config_roundtrip_test.go:145-163`).
- Numeric overrides: `TestNumericAccessors`
  (`internal/features/features_test.go:131-152`),
  `TestNumericOverrides`
  (`internal/features/config_roundtrip_test.go:169-188`),
  `TestFastScanWorkers_ShouldDualReadTheLegacyVar`
  (`internal/features/migration_test.go:126-137`).
- Defaults and parse sentinel: `TestDefaultFeaturesConfig`
  (`internal/features/features_defaults_test.go:5-22`),
  `TestParseInt64`
  (`internal/features/features_defaults_test.go:24-38`),
  `TestFeaturesErrError`
  (`internal/features/features_defaults_test.go:40-44`),
  `TestPerShardFacts_ShouldRemainOptInEvenWhenFullyEnabled`
  (`internal/features/schema_test.go:72-96`),
  `TestPerShardFacts_ShouldStillHonourAnExplicitOptIn`
  (`internal/features/schema_test.go:101-120`).
- Migration matrix: `TestEnvMigration_WhenOnlyTheLegacyVarIsSet_ShouldStillHonourIt`
  (`internal/features/migration_test.go:12-22`),
  `TestEnvMigration_WhenBothVarsAreSet_ShouldPreferTheCanonicalOne`
  (`internal/features/migration_test.go:24-32`),
  `TestEnvMigration_EveryCanonicalVarShouldUseTheCodenerdPrefix`
  (`internal/features/migration_test.go:155-167`),
  `TestEnvMigration_LegacyVarsShouldBeTheKnownFour`
  (`internal/features/migration_test.go:172-198`),
  `TestSystemShardsLegacyEnvIgnored`
  (`internal/features/features_test.go:98-108`).
- Command: `go test ./internal/features/... ./internal/config/... ./cmd/nerd/...`

Planned exits in §8 each name the new test or gate that would fail today.

## 7. North-star trace

Which sentence of `agents.md` this capability serves, and why it is not a
spec for a different project:

- Deterministic safety over luck (NS-2, `agents.md:9`): "This repo exists to
  make that split real in production: creative power with deterministic
  safety, long-horizon context without prompt drift, and parallel specialists
  whose behavior is grounded by logic rather than luck." A flag that gates
  `DerivationRecorder` for `/explain` but has no production reader is silent
  executive state: the safety-relevant knob exists and nothing turns it.
  Wiring it advances deterministic safety; explicitly reserving it stops
  pretending it does. Seam: func `features.IsProvenanceEnabled`
  (`internal/features/features.go:479-482`) vs method
  `(k *RealKernel).IsProvenanceEnabled`
  (`internal/core/kernel_provenance.go:50-54`) read at
  `cmd/nerd/chat/commands_handlers_misc.go:101`, method
  `chat.Model.handleExplainCommand`.
- Harness decides (V-decides, `agents.md:27-29`): "Mangle — a deductive
  database programming language, not Datalog — plus JIT prompt compilation is
  meant to be the replacement for subagents and skills: the harness decides,
  not the model's discretion." Install-then-read-forever with per-call
  resolution (`internal/features/features.go:233-241`,
  `internal/features/features.go:419-434`) is the harness showing one
  resolved truth at every surface; two competing boot constructors with no
  recorded decision is discretionary surface. Seam: func
  `features.DefaultFeaturesConfig`
  (`internal/features/features.go:156-168`) vs func
  `features.FullyEnabledFeaturesConfig`
  (`internal/features/features.go:202-215`) via func
  `config.DefaultUserConfig` (`internal/config/user_config.go:1494`).
- Clean fixpoint (V-fixpoint + V-derived, `agents.md:47-53`): "\"Clean
  fixpoint,\" not \"clean loop.\" Go is ... the FFI, the drivers, the tools.
  The executive decisions ... are the fixpoint of the kernel over the facts.
  The drift to hunt is decisions computed in Go instead of derived." and
  "what is scored is whether a decision is derived and whether an obligation
  is forced." Unfenced flag names and stray-export handling by convention are
  decisions computed in Go. The finished state forces them: enumerate every
  flag against its readers, fail loudly on drift, never flip on garbage.
  Seam: var `features.boolFlags`
  (`internal/features/features.go:276-291`) resolved per call by func
  `features.resolveBool` (`internal/features/features.go:419-434`).
- Pressure that never lets go (V-pressure, `agents.md:55-59`): "The north
  star is a final state... the harness continuously derives the distance ...
  and applies that distance as pressure on every agent... pressure never lets
  go." A parses-but-nothing-reads-it flag hides the distance between what
  the operator configured and what runs. The fence (GAP-FEAT-04) refuses to
  hide it. Seam: pointer read at
  `internal/features/features.go:408`, func `features.Active`, and strict
  parse at `internal/features/features.go:444-458`, func `features.envBool`.

Out of scope for this spec: resolved views, boot log shape, deprecation
report shape, and generated schema belong to the evaluation spec
(`Docs/architecture/features/06-CAPABILITY-SPEC.md:10-19` scope statement)
and its principles P8, P10. This file duplicates none of that reasoning.

## 8. Gaps closed by this spec (buildable exits)

| Gap ID | Capability | Current state (shipped, cited) | Target state (this spec) | Severity | Phase | Blocking dependencies | Exit criteria |
|---|---|---|---|---|---|---|---|
| GAP-FEAT-01 | lifecycle: provenance decided | `features.IsProvenanceEnabled` (`internal/features/features.go:479-482`) is exists-but-uncalled: zero prod callers, only `internal/features/*_test.go` + `config_roundtrip_test.go`; distinct symbol from kernel method `(*RealKernel).IsProvenanceEnabled` (`internal/core/kernel_provenance.go:50-54`); the one chat caller uses the kernel's, not the flag's (`cmd/nerd/chat/commands_handlers_misc.go:101`) | §2 + §5: flag flips the kernel proof path, or flag marked reserved by decision with witness | high | 3 | kernel proof-path owner agrees on wiring vs reserved | `go test ./internal/features/... ./internal/core/... ./cmd/nerd/...` passes with either a new test asserting the flag flips the kernel proof path read at `cmd/nerd/chat/commands_handlers_misc.go:101`, or an ADR marking the flag reserved whose witness (reserved mark + fencing test) resolves |
| GAP-FEAT-02 | lifecycle: one boot truth | two constructors, one prod reader: `features.DefaultFeaturesConfig` (`internal/features/features.go:156-168`) test-only (1 caller at `internal/features/features_defaults_test.go:6`) vs `features.FullyEnabledFeaturesConfig` (`internal/features/features.go:202-215`) reachable via `config.DefaultUserConfig` (`internal/config/user_config.go:1494`); reset path `features.SetActive` (`internal/features/features.go:233-241`, nil resets to defaults) | §2: ADR naming the fresh-boot source of truth plus pinning test | medium | 3 | audit-before-delete per live rule; no code deletion without ADR | ADR with witness naming which constructor fresh boot uses, plus a test pinning that constructor through `config.DefaultUserConfig` (`internal/config/user_config.go:1494`) and the `features.SetActive` reset path, with `go test ./internal/features/... ./internal/config/...` passing |
| GAP-FEAT-04 | lifecycle: fenced registry (reserved meaning; lifecycle half) | names trusted, not fenced: uncalled symbols silent, not errors; `boolFlags` (`internal/features/features.go:276-291`) is the table to enumerate, pointer read at `features.Active` (`internal/features/features.go:408`), resolution per call at `features.resolveBool` (`internal/features/features.go:419-434`) | §5: what "reserved" means for resolution defined; enumeration gate owned jointly with `06-*` | medium | 3 | full-repo `callers_of` for each accessor (searched-set no-caller is not unwired); evaluation enumeration gate owned by `Docs/architecture/features/06-CAPABILITY-SPEC.md` | gate reaches zero unmarked: the enumeration test passes and lists reader symbol+line per flag or its reserved witness, with `reserved` defined here as "parses and resolves but no prod reader by explicit decision, fenced by test" |

Closed gaps stay in `Docs/architecture/features/03-GAP-ANALYSIS.md` marked
closed with the closing commit; they are never deleted. Schema keys surface
(GAP-FEAT-03) and warn-only misconfiguration (GAP-FEAT-05) are evaluation
gaps owned by the evaluation spec
(`Docs/architecture/features/06-CAPABILITY-SPEC.md:218-231`) and are not
closed here.

## 9. Non-goals (owned elsewhere, explicitly not this file)

- Resolved views, boot log, deprecation report, generated schema: evaluation
  docs (`Docs/architecture/features/06-CAPABILITY-SPEC.md`,
  `Docs/architecture/features/README.md:1-66`,
  `Docs/architecture/features/02-CURRENT-STATE.md`,
  `Docs/architecture/features/04-PRINCIPLES-AND-CONSTRAINTS.md`,
  `Docs/architecture/features/WIRING-AND-NOT-BUILT.md:1-163`).
- Legacy migration removal criterion
  (`internal/features/features.go:38-44`) and canonical-prefix discipline
  (`internal/features/features.go:22-36`): governed by principles P9
  (`Docs/architecture/features/04-PRINCIPLES-AND-CONSTRAINTS.md:156-170`);
  this file changes neither the criterion nor any alias.
- Cross-cutting kernel evaluation model: lives in exactly one cross-cutting
  document per standard Rule 6
  (`Docs/journeys/09-architecture-doc-standard.md:178-182`), cited — not
  repeated — by dependents.
- Discards stay discarded: `diff_eval` removal
  (`internal/config/removed_keys.go:24-31`), `NERD_DISABLE_SYSTEM_SHARDS`
  absence (`internal/features/features.go:493-497`), taxonomy wired-OFF
  (`cmd/tools/verify_taxonomy/main.go:17` plus
  `internal/features/features.go:527-538`). Not gaps, no IDs.
