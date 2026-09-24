---
doc-class: governance
subsystem: features
implementation-status: accepted-not-implemented
last-verified: 2026-09-21
verified-against: 34634770970153e78c1e250fdab7abd888dcce6f
supersedes: []
---

# ADR-001: features scope decisions

Scope gate for `internal/features`. Each decision below has context, decision,
consequences, and a witness (test, symbol, predicate, file) per
`Docs/journeys/09-architecture-doc-standard.md:82-88`. Status is derived from
whether the witness resolves, never asserted. All five subjects are never built
at `verified-against`, so every decision and this file are
`accepted-not-implemented`.

## D1 — Provenance flag stays unwired until wired or reserved (GAP-FEAT-01)

Context: `func features.IsProvenanceEnabled`
(`internal/features/features.go:479-482`) exists-but-uncalled in prod (zero prod
callers; only `internal/features/*_test.go` plus `config_roundtrip_test.go`). It
is a distinct symbol from method `(k *RealKernel).IsProvenanceEnabled`
(`internal/core/kernel_provenance.go:50-54`). The one chat caller reads the
kernel, not the flag: method `chat.Model.handleExplainCommand`
(`cmd/nerd/chat/commands_handlers_misc.go:101`).

Decision: Accept the gap as open. Either wire the flag to flip the kernel proof
path read at `cmd/nerd/chat/commands_handlers_misc.go:101`, or mark it reserved
by decision with a fencing test. No code change in this ADR.

Consequences: Until the witness resolves, nothing may cite the flag as gating
`DerivationRecorder` for `/explain`. Wiring advances deterministic safety;
reserving stops pretending.

Witness:

- test: none resolving today (planned `TestProvenanceFlag_WiresToKernel`; closest existing `TestEnvOverridesActiveConfig` at `internal/features/config_roundtrip_test.go:117-139` does not touch provenance wiring)
- symbol: `func features.IsProvenanceEnabled` (`internal/features/features.go:479-482`)
- predicate: none (Go-only today; no Mangle predicate names shipped lifecycle wiring per `Docs/architecture/features/06-CAPABILITY-SPEC.md:77-89`)
- file: `cmd/nerd/chat/commands_handlers_misc.go:101` (`chat.Model.handleExplainCommand` reads kernel method, not flag)

Trace: GAP-FEAT-01 in `Docs/architecture/features/05-CAPABILITY-SPEC.md:299-302`;
vision NS-2 (`agents.md:9` deterministic safety) + NS-1 (`agents.md:7`).

Status: accepted-not-implemented (witness does not resolve).

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

Witness:

- test: `TestDefaultFeaturesConfig` (`internal/features/features_defaults_test.go:5-22`) pins defaults only; fresh-boot pinning test (`TestFreshBoot_UsesSingleSourceOfTruth`, planned) missing
- symbol: `func features.DefaultFeaturesConfig` (`internal/features/features.go:156-168`) vs `func features.FullyEnabledFeaturesConfig` (`internal/features/features.go:202-215`)
- predicate: none (Go-only)
- file: `internal/config/user_config.go:1494` (`func config.DefaultUserConfig`)

Trace: GAP-FEAT-02 in `Docs/architecture/features/05-CAPABILITY-SPEC.md:303-307`;
vision V-decides (`agents.md:27-29` harness decides) + V-fixpoint/V-derived
(`agents.md:47-53`).

Status: accepted-not-implemented (pinning witness does not resolve).

## D3 — Machine-enumerable keys surface or delete (GAP-FEAT-03)

Context: `func features.ConfigSchemaKeys`
(`internal/features/schema.go:56-65`) exists-but-uncalled: 2 callers both in
`internal/features/schema_test.go:14,54`, no prod caller. Contrast reachable
`func features.ConfigSchemaJSON` (`internal/features/schema.go:21-52`) via
`nerd features --schema` (`cmd/nerd/cmd_features.go:37` in `main.featuresCmd`
RunE).

Decision: Accept the gap as open. Either surface keys (CLI flag or completion
reading the same `boolFlags`/`intFlags` tables at
`internal/features/features.go:276-291` and
`internal/features/features.go:296-303`) or delete `ConfigSchemaKeys` plus its
tests with no remaining refs.

Consequences: A generated accessor with no reader stays cruft-vs-offload open
question until the witness resolves. No surface added and nothing deleted here.

Witness:

- test: `TestConfigSchemaKeys_ShouldMatchTheJSONTags` (`internal/features/schema_test.go:44-63`) pins keys-to-tags only; surface test (`TestKeysSurface_ListsEveryKey`, planned) missing
- symbol: `func features.ConfigSchemaKeys` (`internal/features/schema.go:56-65`)
- predicate: none (Go-only)
- file: `cmd/nerd/cmd_features.go:37` (`main.featuresCmd` RunE prints `features.ConfigSchemaJSON()`)

Trace: GAP-FEAT-03 in `Docs/architecture/features/06-CAPABILITY-SPEC.md:141-144`;
vision V-tools (`agents.md:43-46`).

Status: accepted-not-implemented (surface-or-delete witness does not resolve).

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

Witness:

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

Context: Typo'd canonical env is no override, silent: `func features.envBool`
(`internal/features/features.go:444-458` via
`internal/features/features.go:456-457`) plus `func features.envInt`
(`internal/features/features.go:581-595`); only shadowed legacy warns via
`func features.Deprecations` (`internal/features/features.go:341-363`). For
example `CODENERD_DARK_MODE=yes` runs false silent. Callers own logging per
`internal/features/features.go:33-36` plus P8.

Decision: Accept the gap as open. Caller-layer warn-only log for unparseable
canonical values, preserving the no-flip guarantee. Changing default-off safety
itself needs a separate ADR and is out of scope.

Consequences: Silence stays the default operator outcome until the witness warns
while preserving P4 no-flip and P7 conservative defaults.

Witness:

- test: `TestDeprecations_WhenALegacyVarIsSet_ShouldNameTheReplacement` (`internal/features/migration_test.go:91-105`) pins legacy-shadow warnings only; canonical warn-only test (`TestWarnOnly_UnparseableCanonicalWarnsWithoutFlip`, planned) missing
- symbol: `func features.envBool` (`internal/features/features.go:444-458`)
- predicate: none (Go-only; caller owns emission)
- file: `internal/features/features.go:341-363` (`func features.Deprecations`)

Trace: GAP-FEAT-05 in `Docs/architecture/features/06-CAPABILITY-SPEC.md:151-160`;
vision V-pressure (`agents.md:55-59`).

Status: accepted-not-implemented (warn-only witness does not resolve).
