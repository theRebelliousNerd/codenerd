---
doc-class: governance
subsystem: features
implementation-status: partial
last-verified: 2026-09-26
verified-against: 34634770970153e78c1e250fdab7abd888dcce6f
supersedes: []
---

# ADR-001: features scope decisions

Scope gate for `internal/features`. Each decision below has context, decision,
consequences, and a witness (test, symbol, predicate, file) per
`Docs/journeys/09-architecture-doc-standard.md:82-88`. Status is derived from
whether the witness resolves, never asserted. History (observed 2026-09-21 true-then at `verified-against`): all five subjects were never built, so every decision and this file read `accepted-not-implemented` — preserved, not deleted. Current truth 2026-09-26, supersedes that reading for D1/D3/D5: lane B wired D1/D3/D5 since (`internal/system/factory.go:1223` provenance reader, `cmd/nerd/cmd_features.go:41` schema-keys reader, `internal/config/user_config.go:612` misconfiguration warn); D1/D3/D5 are implemented below, D2/D4 remain `accepted-not-implemented`; file-level status is therefore `partial`.

## D1 — Provenance flag wired from boot (GAP-FEAT-01)

Context: History (observed 2026-09-21 true-then): `func features.IsProvenanceEnabled` (`internal/features/features.go:479-482`) exists-but-uncalled in prod (zero prod callers; only `internal/features/*_test.go` plus `config_roundtrip_test.go`) — preserved, not deleted. Current truth 2026-09-26, supersedes the exists-but-uncalled clause: lane B wired the flag since (`internal/system/factory.go:1223` `if features.IsProvenanceEnabled()` enables derivation recording in every shard before the first evaluation, `df4a9d2`). It remains a distinct symbol from method `(k *RealKernel).IsProvenanceEnabled` (`internal/core/kernel_provenance.go:50-54`); the chat caller at `cmd/nerd/chat/commands_handlers_misc.go:101` (`chat.Model.handleExplainCommand`) is now the fallback when the flag is off, not the sole reader.

Decision: History: accept the gap as open — either wire the flag to flip the kernel proof path read at `cmd/nerd/chat/commands_handlers_misc.go:101`, or mark it reserved by decision with a fencing test; no code change in this ADR — preserved. Current: wired option (a) landed (`df4a9d2`): `NewDomainCortex` enables recording in every shard when `features.IsProvenanceEnabled()` holds, before the first evaluation (`internal/system/factory.go:1223`).

Consequences: History: until the witness resolves, nothing may cite the flag as gating `DerivationRecorder` for `/explain`; wiring advances deterministic safety, reserving stops pretending — preserved. Current: witness resolves (below), so the flag may now be cited as gating derivation recording from boot.

**Witness:** test:TestNewDomainCortex_HonorsTheProvenanceFlag

- test (history): none resolving 2026-09-21 (planned `TestProvenanceFlag_WiresToKernel`; closest existing `TestEnvOverridesActiveConfig` at `internal/features/config_roundtrip_test.go:117-139` did not touch provenance wiring) — preserved. Current (resolves): `TestNewDomainCortex_HonorsTheProvenanceFlag` fails without the wiring (`df4a9d2`)
- symbol: `func features.IsProvenanceEnabled` (`internal/features/features.go:479-482`)
- predicate: none (Go-only; no Mangle predicate names shipped lifecycle wiring per `Docs/architecture/features/06-CAPABILITY-SPEC.md:77-89`)
- file (history): `cmd/nerd/chat/commands_handlers_misc.go:101` (`chat.Model.handleExplainCommand` reads kernel method, not flag) — preserved. Current (resolves): `internal/system/factory.go:1223` (`NewDomainCortex` provenance reader)

Trace: GAP-FEAT-01 in `Docs/architecture/features/05-CAPABILITY-SPEC.md:299-302`;
vision NS-2 (`agents.md:9` deterministic safety) + NS-1 (`agents.md:7`).

Status: implemented (witness resolves 2026-09-26 via `internal/system/factory.go:1223`; supersedes the 2026-09-21 `accepted-not-implemented` reading — history preserved, never deleted).

## D2 — One fresh-boot source of truth (GAP-FEAT-02)

Context: Two constructors, one prod reader. `func features.DefaultFeaturesConfig`
(`internal/features/features.go:156-168`) is test-only (1 caller
`internal/features/features_defaults_test.go:6` in
`TestDefaultFeaturesConfig` at `internal/features/features_defaults_test.go:5-22`).
`func features.FullyEnabledFeaturesConfig`
(`internal/features/features.go:202-215`) is reachable via
`func config.DefaultUserConfig` (`internal/config/user_config.go:1494`). Reset
`func features.SetActive` (`internal/features/features.go:233-241`, nil resets to
defaults) is installed by `func config.LoadUserConfig`
(`internal/config/user_config.go:567`).

Decision: Accept the gap as open. Name which constructor fresh boot uses in a
follow-up witness test pinning that constructor through
`config.DefaultUserConfig` (`internal/config/user_config.go:1494`) and the
`features.SetActive` reset path. No deletion without ADR per audit-before-delete.

Consequences: Two boot truths remain discretionary surface until the witness pins
one. No constructor is deleted by this ADR.

**Witness:** test:TestFreshBoot_UsesSingleSourceOfTruth

- test: `TestDefaultFeaturesConfig` (`internal/features/features_defaults_test.go:5-22`) pins defaults only; fresh-boot pinning test (`TestFreshBoot_UsesSingleSourceOfTruth`, planned) missing
- symbol: `func features.DefaultFeaturesConfig` (`internal/features/features.go:156-168`) vs `func features.FullyEnabledFeaturesConfig` (`internal/features/features.go:202-215`)
- predicate: none (Go-only)
- file: `internal/config/user_config.go:1494` (`func config.DefaultUserConfig`)

Trace: GAP-FEAT-02 in `Docs/architecture/features/05-CAPABILITY-SPEC.md:303-307`;
vision V-decides (`agents.md:27-29` harness decides) + V-fixpoint/V-derived
(`agents.md:47-53`).

Status: accepted-not-implemented (pinning witness does not resolve).

## D3 — Machine-enumerable keys surface or delete (GAP-FEAT-03)

Context: History (observed 2026-09-21 true-then): `func features.ConfigSchemaKeys` (`internal/features/schema.go:56-65`) exists-but-uncalled: 2 callers both in `internal/features/schema_test.go:14,54`, no prod caller — preserved, not deleted. Current truth 2026-09-26, supersedes the no-prod-caller clause: lane B surfaced keys since (`cmd/nerd/cmd_features.go:41` prints `features.ConfigSchemaKeys()` as a JSON array, `791e821`). Contrast remains and now both halves are reachable: `func features.ConfigSchemaJSON` (`internal/features/schema.go:21-52`) via `nerd features --schema` (`cmd/nerd/cmd_features.go:37` in `main.featuresCmd` RunE).

Decision: History: accept the gap as open — either surface keys (CLI flag or completion reading the same `boolFlags`/`intFlags` tables at `internal/features/features.go:276-291` and `internal/features/features.go:296-303`) or delete `ConfigSchemaKeys` plus its tests with no remaining refs — preserved, not deleted. Current: surfaced option (a) landed (`791e821`): `nerd features --schema --json` prints `ConfigSchemaKeys()` (`cmd/nerd/cmd_features.go:41`).

Consequences: History: a generated accessor with no reader stays cruft-vs-offload open question until the witness resolves; no surface added and nothing deleted here — preserved. Current: witness resolves (below), so the accessor is surfaced, not cruft.

**Witness:** test:TestFeaturesCmd_WhenSchemaJSONRequested_ShouldEmitTheKeys

- test (history): `TestConfigSchemaKeys_ShouldMatchTheJSONTags` (`internal/features/schema_test.go:44-63`) pins keys-to-tags only; surface test (`TestKeysSurface_ListsEveryKey`, planned) missing 2026-09-21 — preserved. Current (resolves): `TestFeaturesCmd_WhenSchemaJSONRequested_ShouldEmitTheKeys` (`791e821`)
- symbol: `func features.ConfigSchemaKeys` (`internal/features/schema.go:56-65`)
- predicate: none (Go-only)
- file (history): `cmd/nerd/cmd_features.go:37` (`main.featuresCmd` RunE prints `features.ConfigSchemaJSON()`) — preserved. Current (resolves): `cmd/nerd/cmd_features.go:41` (`main.featuresCmd` RunE prints `features.ConfigSchemaKeys()` when `--json`)

Trace: GAP-FEAT-03 in `Docs/architecture/features/06-CAPABILITY-SPEC.md:141-144`;
vision V-tools (`agents.md:43-46`).

Status: implemented (surface witness resolves 2026-09-26 via `cmd/nerd/cmd_features.go:41`; supersedes the 2026-09-21 `accepted-not-implemented` reading — history preserved, never deleted).

## D4 — Fenced registry: every flag has a reader or a reserved mark (GAP-FEAT-04, joint lifecycle + evaluation)

Context: Names trusted, not fenced. `var features.boolFlags`
(`internal/features/features.go:276-291`) is the table to enumerate; pointer read
`func features.Active` (`internal/features/features.go:408`); per-call
`func features.resolveBool` (`internal/features/features.go:419-434`). Uncalled
symbols are silent, not errors. Lifecycle half defines what reserved means for
resolution (`Docs/architecture/features/05-CAPABILITY-SPEC.md:308-313`);
evaluation half is the enumeration gate
(`Docs/architecture/features/06-CAPABILITY-SPEC.md:145-150`).

Decision: Accept the gap as open. Enumeration gate — every `boolFlags` row has a
prod reader (reader symbol plus line) or an explicit reserved mark fenced by a
test. Reserved means parses plus resolves but has no prod reader by explicit
decision.

Consequences: Unmarked unread flags remain convention in Go, not derived/forced
obligation, until the gate reaches zero unmarked.

**Witness:** test:TestRegistryFence_ZeroUnmarked

- test: `TestResolved_ShouldMatchAccessors` (`internal/features/resolved_test.go:140-169`) pins views match accessors; enumeration gate test (`TestRegistryFence_ZeroUnmarked`, planned) missing
- symbol: `var features.boolFlags` (`internal/features/features.go:276-291`) + `func features.resolveBool` (`internal/features/features.go:419-434`)
- predicate: none (planned derived obligation, not shipped; see `Docs/architecture/features/06-CAPABILITY-SPEC.md:83-89`)
- file: `internal/features/features.go:276-291`

Trace: GAP-FEAT-04 in `Docs/architecture/features/05-CAPABILITY-SPEC.md:308-313`
+ `Docs/architecture/features/06-CAPABILITY-SPEC.md:145-150`; vision NS-1
(`agents.md:7`) + V-fixpoint/V-derived (`agents.md:47-53`) + V-pressure
(`agents.md:55-59`).

Status: accepted-not-implemented (gate does not reach zero; confidence 0.7 pending
full-repo `callers_of` pass — do not promote).

## D5 — Warn-only misconfiguration signal (GAP-FEAT-05)

Context: History (observed 2026-09-21 true-then): typo'd canonical env is no override, silent: `func features.envBool` (`internal/features/features.go:444-458` via `internal/features/features.go:456-457`) plus `func features.envInt` (`internal/features/features.go:581-595`); only shadowed legacy warns via `func features.Deprecations` (`internal/features/features.go:341-363`). For example `CODENERD_DARK_MODE=yes` runs false silent. Callers own logging per `internal/features/features.go:33-36` plus P8 — preserved, not deleted. Current truth 2026-09-26, supersedes the silent clause while preserving no-flip: lane B reports refused values since (`internal/features/features.go:379` `features.Misconfigurations()`, warned at boot by `LoadUserConfig` at `internal/config/user_config.go:612`, `791e821`).

Decision: History: accept the gap as open — caller-layer warn-only log for unparseable canonical values, preserving the no-flip guarantee; changing default-off safety itself needs a separate ADR and is out of scope — preserved, not deleted. Current: warn-only path landed (`791e821`): `LoadUserConfig` warns with each misconfiguration at boot (`internal/config/user_config.go:612`); the no-flip guarantee is unchanged.

Consequences: History: silence stays the default operator outcome until the witness warns while preserving P4 no-flip and P7 conservative defaults — preserved. Current: witness resolves (below), so a refused canonical value now warns instead of running silent.

**Witness:** test:TestMisconfigurations_WhenAnEnvValueDoesNotParse_ShouldReportItAndStillIgnoreIt

- test (history): `TestDeprecations_WhenALegacyVarIsSet_ShouldNameTheReplacement` (`internal/features/migration_test.go:91-105`) pins legacy-shadow warnings only; canonical warn-only test (`TestWarnOnly_UnparseableCanonicalWarnsWithoutFlip`, planned) missing 2026-09-21 — preserved. Current (resolves): `TestMisconfigurations_WhenAnEnvValueDoesNotParse_ShouldReportItAndStillIgnoreIt`, `TestFeaturesCmd_WhenAnEnvValueIsRefused_ShouldWarn` (`791e821`)
- symbol (history): `func features.envBool` (`internal/features/features.go:444-458`) — preserved. Current: plus `func features.Misconfigurations` (`internal/features/features.go:379`)
- predicate: none (Go-only; caller owns emission)
- file (history): `internal/features/features.go:341-363` (`func features.Deprecations`) — preserved. Current (resolves): `internal/config/user_config.go:612` (boot warn of each `features.Misconfigurations()`), `cmd/nerd/cmd_features.go:52` (misconfigurations surface)

Trace: GAP-FEAT-05 in `Docs/architecture/features/06-CAPABILITY-SPEC.md:151-160`;
vision V-pressure (`agents.md:55-59`).

Status: implemented (warn-only witness resolves 2026-09-26 via `internal/config/user_config.go:612`; supersedes the 2026-09-21 `accepted-not-implemented` reading — history preserved, never deleted).
