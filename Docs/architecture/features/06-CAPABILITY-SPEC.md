---
doc-class: shipped-with-future
subsystem: features
implementation-status: partial
last-verified: 2026-09-21
verified-against: 34634770970153e78c1e250fdab7abd888dcce6f
supersedes: []
---

# 06 — Capability spec: evaluation and inspection surfaces

> Second capability spec for `internal/features`. Lifecycle (install and
> resolve: `SetActive`, `resolveBool`, `envBool`/`envInt`, defaults,
> `boolFlags`/`intFlags` tables, `FeaturesConfig` shape) belongs to `05-*`
> and is not repeated here. This file specs only evaluation: turning resolved
> flag state into inspectable views (`Resolved`, `Summary`, `Deprecations`,
> schema) and surfacing those views at boot, CLI, and chat.
> Shipped claims below cite code read 2026-09-21; planned behaviour is marked
> planned and never stated as shipped.

## 1. Capability in one paragraph

Given the active registry installed by lifecycle, evaluation answers "what is
in force and why" in one call: every boolean with its winning source, a
single-line boot log, legacy-shadow warnings, and a generated config snippet
plus key list. The finished behaviour is that any operator surface (`nerd
features`, chat `/features`, boot log) renders the same resolved truth with no
transcription drift, every recognised key is enumerable by machine, and any
flag with no production reader or any unparseable canonical value is an
explicit, command-checkable signal — not silence.

## 2. Data shapes (shipped)

All shapes live in `internal/features`; all citations are symbols observed at
`verified-against`.

- `type features.Source string` (`internal/features/features.go:244`) with
  `SourceEnv=env` (`internal/features/features.go:248`),
  `SourceLegacyEnv=legacy-env` (`internal/features/features.go:252`),
  `SourceConfig=config` (`internal/features/features.go:254`),
  `SourceDefault=default` (`internal/features/features.go:256`).
- `struct features.Flag` (`internal/features/features.go:261-268`) carries
  `Name, EnvVar, LegacyEnvVar, Value, Source, Default` — the resolved record.
- `func features.Resolved` (`internal/features/features.go:308-328`) returns
  one `Flag` per boolean, resolved exactly as the accessors resolve it.
- `func features.Summary` (`internal/features/features.go:374-391`) renders
  the single-line resolved-values log (`name=value(source)` for non-default
  plus `fast_scan_workers` and `fast_ast_max_bytes`); it resolves via
  `features.Resolved` (`internal/features/features.go:376`) and appends the
  tunables (`internal/features/features.go:387-388`). Prior raw-config bug is
  documented in its own comment (`internal/features/features.go:365-373`).
- `func features.Deprecations` (`internal/features/features.go:341-363`)
  returns strings naming legacy `NERD_*` variables in use that have a canonical
  replacement, including shadowed ones.
- `func features.boolPtrString` (`internal/features/features.go:395-403`) is
  the nil-to-`unset` log helper.
- `func features.ConfigSchemaJSON` (`internal/features/schema.go:21-52`)
  builds the documented `features`-block snippet from `boolFlags`/`intFlags`
  with `//` comments, so it cannot drift from the accessors. The result is
  deliberately not strict JSON (`internal/features/schema.go:18-20`).
- `func features.ConfigSchemaKeys` (`internal/features/schema.go:56-65`)
  returns every recognised key of the `features` block.
- `func features.envHint` (`internal/features/schema.go:67-72`) formats the
  env hint per key.
- Single source of truth the views read: `var features.boolFlags`
  (`internal/features/features.go:276-291`) for the 8 booleans and
  `var features.intFlags` (`internal/features/features.go:296-303`) for the 2
  integer overrides.

Lifecycle dependency (anchor only, spec lives in `05-*`):
`var features.active atomic.Pointer[FeaturesConfig]`
(`internal/features/features.go:221`) installed by
`func features.SetActive` (`internal/features/features.go:233-241`).

## 3. Go / Mangle split

Today this capability is Go-only, by constraint. `internal/features` depends on
nothing inside codeNERD (`internal/features/features.go:12-13`) and must not
import logging, so it returns strings the caller surfaces
(`internal/features/features.go:33-36`): `Summary` and `Deprecations` are pure
views over the registry, and the caller owns emission.

There are no Mangle predicates for evaluation today. The planned direction
(Rule 2: planned, anchored, not asserted as shipped) is that the two future
signals below — "every flag has a reader or an explicit reserved mark" and
"unparseable canonical env warned without flipping" — become derived
obligations (a predicate that derives, a gate that reaches zero) rather than
conventions in Go. No predicate is named here as shipped; see §8 for the
command-checkable exits that would precede any predicate.

## 4. Current seams (where this capability attaches)

Shipped, each with an exact prod caller verified at `verified-against`:

- Boot: `config.LoadUserConfig` logs `features.Summary()`
  (`internal/config/user_config.go:571`) and warns on each of
  `features.Deprecations()` (`internal/config/user_config.go:576-578`) after
  `features.SetActive(cfg.Features)` (`internal/config/user_config.go:567`).
- CLI: `nerd features` renders `features.Resolved()` as a table
  (`cmd/nerd/cmd_features.go:44` in the `RunE` closure) plus both tunables
  (`cmd/nerd/cmd_features.go:52-53`) plus deprecation warnings
  (`cmd/nerd/cmd_features.go:45`), and `--schema` prints
  `features.ConfigSchemaJSON()` (`cmd/nerd/cmd_features.go:37`;
  snippet built at `internal/features/schema.go:21-52`).
- Chat: the `/features` report renders `features.Resolved()`
  (`cmd/nerd/chat/commands_handlers_features.go:19` in
  `chat.renderFeaturesReport`), `features.Deprecations()`
  (`cmd/nerd/chat/commands_handlers_features.go:40`), `features.Summary()`
  (`cmd/nerd/chat/commands_handlers_features.go:49`), and the tunables
  (`cmd/nerd/chat/commands_handlers_features.go:36,38`).
- Self-reference: `features.Summary` resolves via `features.Resolved`
  (`internal/features/features.go:376` in `features.Summary`;
  `internal/features/features.go:374-391`).

Exists-but-uncalled in prod (shipped code, test-only callers — the gaps in §8):

- `func features.ConfigSchemaKeys` (`internal/features/schema.go:56-65`) — 2
  callers, both `internal/features/schema_test.go:14,54`. No prod caller;
  contrast `ConfigSchemaJSON`, which is reachable via `nerd features --schema`
  (`cmd/nerd/cmd_features.go:37`).
- The four-uncalled set the fence would enumerate is documented in
  `WIRING-AND-NOT-BUILT.md`; only the evaluation-visible member
  (`ConfigSchemaKeys`) is specced here. Flag-wiring members
  (`IsProvenanceEnabled` at `internal/features/features.go:479-482`,
  `DefaultFeaturesConfig` at `internal/features/features.go:156-168`,
  `Active` at `internal/features/features.go:408`) belong to the lifecycle
  spec and are named here only to exclude them.

## 5. Failure modes (shipped handling + planned hardening)

- **Drift between accessors and docs.** Handled shipped: schema and views read
  `boolFlags`/`intFlags` (`internal/features/features.go:276-291`,
  `internal/features/features.go:296-303`), so a new flag without a table row
  cannot render. Ruling: never hard-code a flag name, env var, or default
  outside the tables.
- **Raw config rendered as effective state.** Fixed shipped: `Summary`
  (`internal/features/features.go:374-391`) reports resolved values, not raw
  `FeaturesConfig` fields; the prior version printed `*bool` fields and hid env
  overrides (`internal/features/features.go:365-373`). Ruling: never render raw
  fields as effective state.
- **Silent recognised-but-unused surface (planned GAP-FEAT-03).**
  `ConfigSchemaKeys` has no prod reader today. Either it gains a surface or it
  is deleted; a generated accessor with no reader is the exact cruft-vs-offload
  question. Exit in §8.
- **Silent unread flags (planned GAP-FEAT-04, evaluation half).** Nothing stops
  two packages reading the same flag to mean different things, and nothing warns
  when a flag has no reader — uncalled symbols are silent, not errors. The
  evaluation half is a command-checkable enumeration: every `boolFlags` row has
  a prod reader or an explicit reserved mark. The lifecycle half (what
  "reserved" means for resolution) belongs to `05-*`.
- **Silent misconfiguration (planned GAP-FEAT-05, warn-only).**
  `func features.envBool` (`internal/features/features.go:444-458`) returns nil
  ("no override") for anything but `1`/`0`/`true`/`false`, and
  `func features.envInt` (`internal/features/features.go:581-595`) ignores
  non-numeric/non-positive values, so `CODENERD_DARK_MODE=yes` runs as false
  with no message; only shadowed legacy vars warn via `Deprecations()`
  (`internal/features/features.go:341-363`). Planned hardening is warn-only at
  the caller layer (never inside this leaf): log the unparseable canonical
  value while preserving the no-flip guarantee. Changing default-off safety
  itself needs an ADR and is out of scope here.

## 6. Proving tests (command-checkable)

Run with `CGO_CFLAGS=-IC:/CodeProjects/codeNERD/sqlite_headers`:

- Views match accessors: `TestResolved_ShouldMatchAccessors`
  (`internal/features/resolved_test.go:140-169`), `TestResolved_PrecedenceMatrix`
  (`internal/features/resolved_test.go:11-102`).
- Single-line, pointer-free summary:
  `TestSummary_ShouldBeSingleLineAndPointerFree`
  (`internal/features/resolved_test.go:172-186`),
  `TestSummaryRendersBoolPointersAsValues`
  (`internal/features/features_test.go:159-243`).
- Deprecation surfaces: `TestDeprecations_WhenALegacyVarIsSet_ShouldNameTheReplacement`
  (`internal/features/migration_test.go:91-105`),
  `TestResolved_WhenALegacyVarDecides_ShouldReportLegacyEnvSource`
  (`internal/features/migration_test.go:57-71`).
- Schema generated from tables, keys match tags:
  `TestConfigSchemaJSON_ShouldListEveryRecognisedKey`
  (`internal/features/schema_test.go:11-22`),
  `TestConfigSchemaJSON_ShouldNameEveryEnvVar`
  (`internal/features/schema_test.go:24-40`),
  `TestConfigSchemaKeys_ShouldMatchTheJSONTags`
  (`internal/features/schema_test.go:44-63`).
- Surfaces: `cmd/nerd/cmd_features_test.go` (CLI table, tunables,
  deprecations, `--schema` snippet) and
  `cmd/nerd/chat/commands_handlers_features_test.go` (chat report).
- Command: `go test ./internal/features/... ./internal/config/... ./cmd/nerd/...`

Planned exits in §8 each name the new test or gate that would fail today.

## 7. North-star trace

Which sentence of `agents.md` this capability serves, and why it is not a spec
for a different project:

- Deterministic safety over luck (NS-2, `agents.md:9`): one resolved truth rendered
  identically at boot, CLI, and chat removes the class of bug where the
  operator debugs a flag the binary never honoured. Generated-from-tables views
  (`internal/features/schema.go:21-52` from `internal/features/features.go:276-291`)
  are offload-to-deterministic-code: the snippet cannot say what the accessors
  do not do.
- Pressure that never lets go (V-pressure, `agents.md:55-59`): the fence (GAP-FEAT-04) and the
  warn-only misconfiguration signal (GAP-FEAT-05) turn "distance from finished"
  into a checkable gate — unmarked unread flags reach zero; unparseable
  canonical values warn while preserving no-flip. Silence stops being the
  default outcome for an operator error.
- Tools that earn their keep (V-tools, `agents.md:43-46`): `ConfigSchemaKeys`
  (`internal/features/schema.go:56-65`) either gains a CLI/completion surface
  that condenses the search space (GAP-FEAT-03) or is deleted as cruft. A
  generated accessor with no reader does not survive on intent.

Out of scope for this spec: resolution precedence, env strictness rationale,
and constructor fate belong to the lifecycle spec
(`Docs/architecture/features/05-CAPABILITY-SPEC.md:10-21` scope statement) and its principles
P2–P4, P6–P7. This file duplicates none of that reasoning.

## 8. Gaps closed by this spec (buildable exits)

| Gap ID | Capability | Current state (shipped, cited) | Target state (this spec) | Severity | Phase | Blocking dependencies | Exit criteria |
|---|---|---|---|---|---|---|---|
| GAP-FEAT-03 | evaluation: machine-enumerable keys | `ConfigSchemaKeys` (`internal/features/schema.go:56-65`) is exists-but-uncalled: 2 call sites both in `internal/features/schema_test.go` (`:14` in `TestConfigSchemaJSON_ShouldListEveryRecognisedKey` at `internal/features/schema_test.go:11-22`, `:54` in `TestConfigSchemaKeys_ShouldMatchTheJSONTags` at `internal/features/schema_test.go:44-63`), no prod caller; contrast reachable `ConfigSchemaJSON` via `nerd features --schema` (`cmd/nerd/cmd_features.go:37` in `main.featuresCmd` RunE). Drift note: `Docs/architecture/features/IMPLEMENTED_SPEC.md:191-199` still lists both schema funcs as exists-but-uncalled; code wins — `ConfigSchemaJSON` is reachable, only `ConfigSchemaKeys` is uncalled | §2 + §4: keys surface (CLI flag or completion) reading the same tables, or deletion | medium | 3 | full-repo `callers_of ConfigSchemaKeys` re-check | `go test ./internal/features/... ./cmd/nerd/...` passes with either a new surface test asserting the CLI/completion output lists every key, or a commit deleting `ConfigSchemaKeys` and its tests with no remaining references |
| GAP-FEAT-04 | evaluation: fenced registry (reader enumeration; evaluation half) | names trusted, not fenced: uncalled symbols silent, not errors; `boolFlags` (`internal/features/features.go:276-291`) is the table to enumerate | §5: command-checkable test enumerating every `boolFlags` row has a prod reader or an explicit reserved mark | medium | 3 | full-repo `callers_of` for each accessor; lifecycle ADR for what "reserved" means (owned by `Docs/architecture/features/05-CAPABILITY-SPEC.md`) | gate reaches zero unmarked: the enumeration test passes and lists reader symbol+line per flag or its reserved witness |
| GAP-FEAT-05 | evaluation: warn-only misconfiguration signal | typo'd canonical env is no override, silent (`internal/features/features.go:444-458` via `internal/features/features.go:456-457`); only shadowed legacy warns via `Deprecations()` (`internal/features/features.go:341-363`) | §5: caller-layer warn-only log for unparseable canonical env, preserving no-flip | low | 3 | ADR if default-off safety itself changes (out of scope); caller owns logging per `internal/features/features.go:33-36` | new test asserting `CODENERD_DARK_MODE=yes` (or equivalent) warns at the caller, resolves to no-override, and `go test ./internal/features/... ./internal/config/...` passes |

Closed gaps stay in `Docs/architecture/features/03-GAP-ANALYSIS.md` marked
closed with the closing commit; they are never deleted.
`IsProvenanceEnabled` wiring (GAP-FEAT-01) and `DefaultFeaturesConfig` vs
`FullyEnabled` fate (GAP-FEAT-02) are lifecycle gaps owned by the lifecycle spec
(`Docs/architecture/features/05-CAPABILITY-SPEC.md:10-21` scope statement) and
are not closed here.

## 9. Non-goals (owned elsewhere, explicitly not this file)

- Resolution order, strict parsing, defaults, and constructors: lifecycle docs
  (`Docs/architecture/features/README.md:1-66`,
  `Docs/architecture/features/02-CURRENT-STATE.md`,
  `Docs/architecture/features/04-PRINCIPLES-AND-CONSTRAINTS.md`,
  `Docs/architecture/features/WIRING-AND-NOT-BUILT.md:1-163`).
- Kernel provenance method vs flag symbol distinction
  (`internal/core/kernel_provenance.go:50-54` vs
  `internal/features/features.go:479-482`): recorded here only as an exclusion
  boundary; wiring fate belongs to lifecycle docs, not this evaluation spec.
- Cross-cutting kernel evaluation model: lives in exactly one cross-cutting
  document per standard Rule 6
  (`Docs/journeys/09-architecture-doc-standard.md:178-182`), cited — not
  repeated — by dependents.
  repeated — by dependents.