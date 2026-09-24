---
doc-class: governance
subsystem: features
implementation-status: not-applicable
last-verified: 2026-09-21
verified-against: 34634770970153e78c1e250fdab7abd888dcce6f
supersedes: []
---

# features: risk register and decision log

This file answers one question: what could break the shipped
`internal/features` behaviour, and what decisions are already recorded
against that breakage. It does not answer what runs today (that is
`Docs/architecture/features/02-CURRENT-STATE.md:1-342`), what the finished
state looks like (that is
`Docs/architecture/features/01-VISION.md:1-268`), how lifecycle versus
evaluation are specced (those are
`Docs/architecture/features/05-CAPABILITY-SPEC.md:10-21` and
`Docs/architecture/features/06-CAPABILITY-SPEC.md:10-21`), or what is still
undecided (that is
`Docs/architecture/features/OPEN-QUESTIONS.md:1-198`). Nothing below is
written as a design question: questions live in `OPEN-QUESTIONS.md`, gaps
with exits live in `Docs/architecture/features/01-VISION.md:185-216`,
and anything undecided about provenance wiring, boot constructors, schema
keys, the reserved fence, or warn-only placement is deliberately absent
here. Risks below are failures of shipped invariants; decisions below are
already taken and fenced by tests.

## Risk register

Each risk names likelihood, consequence, and what would retire it. Likelihood
is judged against the code read 2026-09-21; retirement is command-checkable.

| Risk ID | Risk (shipped invariant at stake) | Likelihood | Consequence | What retires it |
|---|---|---|---|---|
| R-FEAT-01 | Flight recorder turned on where it should stay off, aborting the process under load. Guard today is default-off in func `features.DefaultFeaturesConfig` (`internal/features/features.go:156-168`) plus the strict parse in func `features.envBool` (`internal/features/features.go:444-458`), because the tracer region allocator can abort via `throw(traceRegion)` (`internal/features/features.go:460-470`; accessor `internal/features/features.go:471-474`; gate `cmd/nerd/main.go:395` in `main.main`). | low — default off and stray values are no-override | high — process abort under long-running load | Keep the conservative default and the strict parse; proving tests `TestDefaultFeaturesConfig` (`internal/features/features_defaults_test.go:5-22`) and `TestResolveBoolPrecedence` (`internal/features/features_test.go:12-67`). Command: `go test ./internal/features/...`. |
| R-FEAT-02 | Per-shard facts flipped without resolving the audit, silently deleting cross-domain derivations. Guard today is default-off with `FullyEnabledFeaturesConfig` keeping `PerShardFacts:false` per the 2026-08-15 audit (`internal/features/features.go:175-196`; accessor `internal/features/features.go:509-512`; reader `internal/core/cortex_kernel.go:119` in `core.NewCortexKernel`). | low — both constructors keep it off | high — dispatch-only router with no cross-shard joins drops derivations silently | Keep the opt-in; proving tests `TestPerShardFacts_ShouldRemainOptInEvenWhenFullyEnabled` (`internal/features/schema_test.go:72-96`) and `TestPerShardFacts_ShouldStillHonourAnExplicitOptIn` (`internal/features/schema_test.go:101-120`). Command: `go test ./internal/features/...`. |
| R-FEAT-03 | Fast taxonomy defaulting on, so verification skips itself on ordinary runs. Guard today is default-off with the tool reading the registry (`cmd/tools/verify_taxonomy/main.go:17` in `main.main`; accessor `internal/features/features.go:535-538`; rationale `internal/features/features.go:527-538`). | low — wired-OFF end state | medium — scenario sweep skipped, false confidence | Keep the wired-OFF default; proving command: `go test ./internal/features/...` plus tool run with and without `CODENERD_TAXONOMY_FAST`. |
| R-FEAT-04 | Caller mutates the pointer `Active()` returns or caches a resolved value across `SetActive`, breaking the no-snapshot contract. Contract today is copy-on-install plus load-per-call under `var features.active atomic.Pointer[FeaturesConfig]` (`internal/features/features.go:221`; installer `internal/features/features.go:233-241`, copy at `internal/features/features.go:238-240`; reader `internal/features/features.go:408`; per-call load `internal/features/features.go:419-434` at line 428). | medium — the easiest mistake a new caller makes | medium — stale reads or data race on hot paths | Keep copy-plus-atomic discipline; proving tests `TestSetActiveCopySemantics` (`internal/features/features_test.go:112-126`), `TestSetActive_ShouldCopyTheConfig` (`internal/features/resolved_test.go:213-226`), `TestSetActive_ConcurrentWithReads` (`internal/features/resolved_test.go:189-209`). Command: `go test ./internal/features/...`. |
| R-FEAT-05 | New flag hard-codes its name, env var, or default outside the tables, so views and schema drift from the accessors. Guard today is `var features.boolFlags` (`internal/features/features.go:276-291`) plus `var features.intFlags` (`internal/features/features.go:296-303`) as the single source of truth, with the add-a-flag lockstep (`internal/features/features.go:46-54`). | medium — the common contribution error | medium — CLI, chat, and schema disagree with what the binary honours | Keep tables as SST; proving tests `TestConfigSchemaJSON_ShouldListEveryRecognisedKey` (`internal/features/schema_test.go:11-22`), `TestConfigSchemaJSON_ShouldNameEveryEnvVar` (`internal/features/schema_test.go:24-40`), `TestResolved_ShouldMatchAccessors` (`internal/features/resolved_test.go:140-169`). Command: `go test ./internal/features/...`. |
| R-FEAT-06 | Effective state rendered from raw config fields instead of resolved values, hiding env overrides (the fixed `Summary` bug). Guard today is func `features.Summary` (`internal/features/features.go:374-391`) resolving via func `features.Resolved` (`internal/features/features.go:308-328`), with the prior bug documented (`internal/features/features.go:365-373`). | low — fixed and pinned | medium — operator debugs a value the binary never honoured | Keep resolved-not-raw; proving tests `TestSummary_ShouldBeSingleLineAndPointerFree` (`internal/features/resolved_test.go:172-186`) and `TestSummaryRendersBoolPointersAsValues` (`internal/features/features_test.go:159-243`). Command: `go test ./internal/features/...`. |
| R-FEAT-07 | Legacy alias added casually or removed prematurely, changing behaviour silently. Guard today is the P9 removal criterion (`internal/features/features.go:38-44`): legacy names go away only after a release ships with func `features.Deprecations` (`internal/features/features.go:341-363`) surfaced at boot (`internal/config/user_config.go:576-578`) and operator docs are fixed. Canonical shape is `CODENERD_` plus upper-cased key (`internal/features/features.go:22-36`). | low — four known legacy names, ratcheted by test | medium — renamed export silently stops or starts working | Keep canonical-first with old name demoted, never deleted in the same change; proving tests `TestEnvMigration_EveryCanonicalVarShouldUseTheCodenerdPrefix` (`internal/features/migration_test.go:155-167`) and `TestEnvMigration_LegacyVarsShouldBeTheKnownFour` (`internal/features/migration_test.go:172-198`). Command: `go test ./internal/features/...`. |
| R-FEAT-08 | Gate reads before install, so a default runs where config should have. Guard today is eager config load before any gate (`cmd/nerd/main.go:365-378` in `main.main`) with install plus boot log in func `config.LoadUserConfig` (`internal/config/user_config.go:567` installs via func `features.SetActive`, `internal/config/user_config.go:571` logs via func `features.Summary`). | low — boot order is explicit | medium — first-run behaviour disagrees with the config file | Keep eager load; proving test `TestLoadUserConfig_InstallsFeaturesIntoRegistry` (`internal/features/config_roundtrip_test.go:48-111`). Command: `go test ./internal/features/... ./internal/config/...`. |

## Decision log

Recorded decisions — context, decision, witness. Status is derived from
whether the witness resolves, never asserted. Undecided items are not here;
see `Docs/architecture/features/OPEN-QUESTIONS.md:30-138` (Q1–Q5).

| Decision ID | Context and decision | Witness (resolves today) |
|---|---|---|
| DEC-FEAT-01 | Leaf stays leaf: `internal/features` exists so low-level subsystems read flags without an import cycle (`internal/features/features.go:1-13`, layering rule `internal/features/features.go:7-11`). Decision: stdlib only, no `codenerd/internal/*` import, no logging inside (imports `internal/features/features.go:57-62`: `fmt`, `os`, `strings`, `sync/atomic`). | Import list at `internal/features/features.go:57-62`; ruling `Docs/architecture/features/04-PRINCIPLES-AND-CONSTRAINTS.md:15-26`. Violated by any new internal import. |
| DEC-FEAT-02 | Stray exports never flip a bit: strict parse accepts only `1`/`0`/`true`/`false` (func `features.envBool` at `internal/features/features.go:444-458` via `internal/features/features.go:451-457`); numbers accept only positive digit strings (func `features.envInt` at `internal/features/features.go:581-595` via func `features.parseInt64` at `internal/features/features.go:597-609`). | `TestResolveBoolPrecedence` (`internal/features/features_test.go:12-67`), `TestParseInt64` (`internal/features/features_defaults_test.go:24-38`); ruling `Docs/architecture/features/04-PRINCIPLES-AND-CONSTRAINTS.md:57-69`. |
| DEC-FEAT-03 | Canonical wins in fixed order: func `features.resolveBool` (`internal/features/features.go:419-434`) reads canonical, then legacy only when canonical is absent or unparseable (`internal/features/features.go:416-418`), then active, then default; func `features.Resolved` reports the winner (`internal/features/features.go:308-328`). | `TestResolved_PrecedenceMatrix` (`internal/features/resolved_test.go:11-102`), `TestEnvMigration_WhenBothVarsAreSet_ShouldPreferTheCanonicalOne` (`internal/features/migration_test.go:24-32`); ruling `Docs/architecture/features/04-PRINCIPLES-AND-CONSTRAINTS.md:43-55`. |
| DEC-FEAT-04 | Absent is not false; zero is not a value: booleans are `*bool` (`internal/features/features.go:64-68`); tunables return `0` for call-site default, defaulted by func `world.DefaultScannerConfig` (`internal/world/scanner_config.go:29-38`; workers `internal/world/scanner_config.go:30-33`, cutoff `internal/world/scanner_config.go:35-38`). | `TestNumericAccessors` (`internal/features/features_test.go:131-152`); ruling `Docs/architecture/features/04-PRINCIPLES-AND-CONSTRAINTS.md:86-101`. |
| DEC-FEAT-05 | Dangerous or expensive stays off: func `features.DefaultFeaturesConfig` (`internal/features/features.go:156-168`) is all-false except `SystemShards:true`; func `features.FullyEnabledFeaturesConfig` (`internal/features/features.go:202-215`) keeps `PerShardFacts:false` and `PromptEvolution:false` per the audit (`internal/features/features.go:175-196`). | `TestDefaultFeaturesConfig` (`internal/features/features_defaults_test.go:5-22`) plus the schema opt-in tests at `internal/features/schema_test.go:72-120`; ruling `Docs/architecture/features/04-PRINCIPLES-AND-CONSTRAINTS.md:103-136`. |
| DEC-FEAT-06 | Report, don't log; show resolved, not raw: leaf returns strings, caller surfaces them (`internal/features/features.go:33-36`; caller-log contract `internal/features/features.go:227-232`); shadowed legacy still reported (`internal/features/features.go:338-340`). | `TestDeprecations_WhenALegacyVarIsSet_ShouldNameTheReplacement` (`internal/features/migration_test.go:91-105`) plus the summary tests at `internal/features/resolved_test.go:172-186`; ruling `Docs/architecture/features/04-PRINCIPLES-AND-CONSTRAINTS.md:138-154`. |
| DEC-FEAT-07 | Schema is generated, not transcribed: func `features.ConfigSchemaJSON` (`internal/features/schema.go:21-52`) feeds `nerd features --schema` (`cmd/nerd/cmd_features.go:37`); func `features.ConfigSchemaKeys` (`internal/features/schema.go:56-65`) stays test-only today, so no machine surface is claimed here. | Schema tests `internal/features/schema_test.go:11-63`; ruling `Docs/architecture/features/04-PRINCIPLES-AND-CONSTRAINTS.md:172-183`. |
| DEC-FEAT-08 | `diff_eval` stays removed: the differential-evaluation path was deleted and the kernel always rebuilds from the EDB; configuring the key fails load with a named reason via var `config.removedFeatureKeys` (`internal/config/removed_keys.go:24-31`). | Configuring `diff_eval` fails load; recorded `Docs/architecture/features/WIRING-AND-NOT-BUILT.md:147-150` and `Docs/architecture/features/01-VISION.md:235-237`. Do not revive. |
| DEC-FEAT-09 | Taxonomy fast stays wired-OFF: the tool reads the registry rather than raw env (func `main.main` at `cmd/tools/verify_taxonomy/main.go:17`) with default off (`internal/features/features.go:527-538`). | Tool plus accessor cited above; recorded `Docs/architecture/features/WIRING-AND-NOT-BUILT.md:151-155` and `Docs/architecture/features/01-VISION.md:242-245`. Do not revive the contradiction. |
| DEC-FEAT-10 | `NERD_DISABLE_SYSTEM_SHARDS` stays absent: no such mechanism exists in any `.go` file per the comment preceding func `features.IsSystemShardsEnabled` (`internal/features/features.go:493-497`; accessor `internal/features/features.go:498-501`). | Absence per the cited comment; recorded `Docs/architecture/features/WIRING-AND-NOT-BUILT.md:156-158` and `Docs/architecture/features/01-VISION.md:238-241`. Do not revive. |

## Explicit non-overlap

- Design questions Q1–Q5 and tripwires T1–T6 belong to
  `Docs/architecture/features/OPEN-QUESTIONS.md:30-175` and are not restated
  here, even where they share a seam (for example the strict parse at
  `internal/features/features.go:444-458` appears here only as decided
  behaviour R-FEAT-01/DEC-FEAT-02, never as where a future warning should
  live).
- Gap IDs and exits GAP-FEAT-01..05 are fixed in
  `Docs/architecture/features/01-VISION.md:185-216`, with rows in
  `Docs/architecture/features/03-GAP-ANALYSIS.md:36-59`; this file adds no gap,
  renames none, and redefines no exit.
- Shipped behaviour claims belong to
  `Docs/architecture/features/02-CURRENT-STATE.md:1-342` and
  `Docs/architecture/features/WIRING-AND-NOT-BUILT.md:15-136`; this file cites
  those seams only as risk context or decision witnesses.
