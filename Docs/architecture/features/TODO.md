---
doc-class: governance
subsystem: features
implementation-status: not-applicable
last-verified: 2026-09-26
verified-against: 34634770970153e78c1e250fdab7abd888dcce6f
supersedes: []
---

> **Status, 2026-09-25 (lane B build-out).** Every item below is closed.
> The item bodies are kept as written; their `features.go` anchors predate
> `Misconfigurations` and have shifted (current: `IsProvenanceEnabled`
> `internal/features/features.go:530`, `Misconfigurations` `:379`,
> `DefaultFeaturesConfig` `:156`, `FullyEnabledFeaturesConfig` `:202`,
> `boolFlags` `:276`; `ConfigSchemaKeys` `internal/features/schema.go:56`).
> Decisions were recorded in the commits and in this table rather than in
> separate ADR files: each has a test as its witness, which is what the
> exits ask for.
> Correction 2026-09-26: the `rather than in separate ADR files` clause above is superseded — `Docs/architecture/features/adr/ADR-001-features-scope.md:1-175` exists with five decisions D1-D5 and witnesses per `Docs/journeys/09-architecture-doc-standard.md:82-88`; the commit-plus-table record above remains true, the ADR file is added, not substituted. Further correction 2026-09-26 per `Docs/architecture/features/00-INDEX.md:60`: lane B wired D1/D3/D5 since (`internal/system/factory.go:1207` provenance reader, `cmd/nerd/cmd_features.go:41` schema-keys reader, `internal/config/user_config.go:601` misconfiguration warn); the never-built / `accepted-not-implemented` clause is superseded — updated ADR text wins, history preserved, never deleted.
>
> | Item | Resolution | Evidence |
> |---|---|---|
> | TODO-FEAT-01a / 01b | closed `df4a9d2`: **wired**, option (a) | `NewDomainCortex` enables recording in every shard when `features.IsProvenanceEnabled()` holds, before the first evaluation (`internal/system/factory.go:1207`). Measured on the default corpus: 79 ms per evaluation off, 81 ms on; the buffer holds only the latest pass, so no reason remained to leave the flag unread. `TestNewDomainCortex_HonorsTheProvenanceFlag` fails without the wiring |
> | TODO-FEAT-02a / 02b | closed `791e821` | one boot truth: a no-config boot resolves every flag to the table's default, which `DefaultFeaturesConfig` states as a struct, and `nerd init` seeds `FullyEnabledFeaturesConfig` via `config.DefaultUserConfig` (`internal/config/user_config.go:1514`). Pinned by `TestBootTruth_ANoConfigBootResolvesToDefaultFeaturesConfig` and `TestBootTruth_InitSeedsFullyEnabledFeaturesConfig` (`internal/features/boot_truth_test.go`) |
> | TODO-FEAT-03 | closed `791e821`: surfaced, option (a) | `nerd features --schema --json` prints `ConfigSchemaKeys()` as a JSON array (`cmd/nerd/cmd_features.go:41`); `TestFeaturesCmd_WhenSchemaJSONRequested_ShouldEmitTheKeys` |
> | TODO-FEAT-04a / 04b | closed `df4a9d2` | `TestEveryFlagHasAProductionReaderOrIsReserved` (`internal/features/flag_readers_test.go`) walks the module and requires a production reader outside `internal/features` for every flag (the `nerd features` / `/features` inspection surfaces do not count), or a `reservedFlags` entry with its decision; `reservedFlags` is empty. It logs the reader table (e.g. `provenance -> internal/system/factory.go:1207`, `fast_scan_workers -> internal/world/scanner_config.go:31`) and failed before `df4a9d2`, naming provenance |
> | TODO-FEAT-05 | closed `791e821` | `features.Misconfigurations()` names every feature env var set to a value the registry refuses; `LoadUserConfig` warns with each at boot (`internal/config/user_config.go:601`), and `nerd features` / `/features` print them. The no-flip guarantee is unchanged. `TestMisconfigurations_WhenAnEnvValueDoesNotParse_ShouldReportItAndStillIgnoreIt`, `TestFeaturesCmd_WhenAnEnvValueIsRefused_ShouldWarn` |

# features: TODO (build queue)

This file answers one question for `internal/features`: what leaf work
remains, and how its completion is checked. It builds nothing new on its
own — every item traces to a gap ID fixed in
`Docs/architecture/features/01-VISION.md:176-216`, with full
current/target/severity/phase/deps rows in
`Docs/architecture/features/03-GAP-ANALYSIS.md:36-59`.
IDs and exits are fixed there; this file cannot redefine them.

Evidence base (all read 2026-09-21, verified against
`34634770970153e78c1e250fdab7abd888dcce6f`):
`Docs/architecture/features/01-VISION.md:176-216` fixes IDs and exits;
`Docs/architecture/features/05-CAPABILITY-SPEC.md:425-431` owns
GAP-FEAT-01/02/04-lifecycle;
`Docs/architecture/features/06-CAPABILITY-SPEC.md:218-231` owns
GAP-FEAT-03/04-eval/05;
`Docs/architecture/features/WIRING-AND-NOT-BUILT.md:15-85` reachable vs
`:87-109` exists-but-uncalled vs `:111-136` assumed;
`Docs/architecture/features/02-CURRENT-STATE.md:278-325` shipped reachable;
`Docs/architecture/features/04-PRINCIPLES-AND-CONSTRAINTS.md:15-183` P1-P10;
`agents.md:7,9,27-29,43-53,55-63` vision sentences. Currency: 2026-09-21
rewrite, supersedes July corpus per `agents.md:60-63` — July docs are never
evidence.

Rules for this queue (per `Docs/journeys/09-architecture-doc-standard.md:63`):

- Leaf work only. Each item is one reviewable change with one command exit.
- Each item names its gap (`GAP-FEAT-01..05`). Work with no gap ID does not
  belong here.
- Exit criteria are command-checkable: a `go test` that passes, a deletion
  commit with zero remaining refs, or an ADR whose witness resolves.
  "Improved", "robust" and "complete" are not exits.
- Closed items stay marked closed with the commit that closed them; no item
  is deleted.

All items are Phase 3. Severities and blocking deps are owned by the gap
matrix; repeated here only as pointers.

## TODO-FEAT-01a — Decide provenance flag fate (ADR draft)

- Traces to: `GAP-FEAT-01` (lifecycle: provenance decided, high, Phase 3)
  fixed in `Docs/architecture/features/01-VISION.md:185-190`, row in
  `Docs/architecture/features/03-GAP-ANALYSIS.md:40`, specced in
  `Docs/architecture/features/05-CAPABILITY-SPEC.md:113-119` plus
  `:299-302`.
- Seam: `func features.IsProvenanceEnabled`
  (`internal/features/features.go:479-482`) exists-but-uncalled vs method
  `(*RealKernel).IsProvenanceEnabled`
  (`internal/core/kernel_provenance.go:50-54`); the one chat caller uses the
  kernel method, not the flag, in `chat.Model.handleExplainCommand`
  (`cmd/nerd/chat/commands_handlers_misc.go:101`).
- Leaf work: draft `adr/ADR-NNN-provenance-decided.md` choosing exactly one
  of (a) wire the flag to the kernel proof path read at
  `chat.Model.handleExplainCommand`
  (`cmd/nerd/chat/commands_handlers_misc.go:101`), or (b) mark the flag
  reserved by decision with a fencing test. Needs kernel proof-path owner
  agreement on wiring vs reserved (blocking dep owned by gap row).
- Exit: `test -f adr/ADR-NNN-provenance-decided.md` shows the ADR with a named witness (reserved mark plus fencing test, or wiring test hook), per the GAP-FEAT-01 exit, with `go test ./internal/features/...` still passing. No code change required for this item alone.

## TODO-FEAT-01b — Implement provenance decision with test

- Traces to: `GAP-FEAT-01`, same sources as TODO-FEAT-01a.
- Seam: same as TODO-FEAT-01a — `func features.IsProvenanceEnabled`
  (`internal/features/features.go:479-482`) vs
  `chat.Model.handleExplainCommand`
  (`cmd/nerd/chat/commands_handlers_misc.go:101`).
- Leaf work: implement the decided path — either a test asserting the flag
  flips the kernel proof path read at `chat.Model.handleExplainCommand`
  (`cmd/nerd/chat/commands_handlers_misc.go:101`), or the reserved mark plus
  fencing test from the TODO-FEAT-01a ADR.
- Exit: `go test ./internal/features/... ./internal/core/... ./cmd/nerd/...`
  passes with the new test, or with the ADR witness resolving.

## TODO-FEAT-02a — ADR one boot truth

- Traces to: `GAP-FEAT-02` (lifecycle: one boot truth, medium, Phase 3)
  fixed in `Docs/architecture/features/01-VISION.md:191-198`, row in
  `Docs/architecture/features/03-GAP-ANALYSIS.md:41`, specced in
  `Docs/architecture/features/05-CAPABILITY-SPEC.md:66-80`.
- Seam: `func features.DefaultFeaturesConfig`
  (`internal/features/features.go:156-168`) test-only vs
  `func features.FullyEnabledFeaturesConfig`
  (`internal/features/features.go:202-215`) reachable via
  `func config.DefaultUserConfig`
  (`internal/config/user_config.go:1494`); reset via
  `func features.SetActive` (`internal/features/features.go:233-241`, nil
  resets to defaults).
- Leaf work: draft `adr/ADR-NNN-boot-truth.md` naming which constructor fresh
  boot uses. No deletion without this ADR (audit-before-delete blocking dep
  owned by gap row).
- Exit: `test -f adr/ADR-NNN-boot-truth.md` shows the ADR with a witness naming the boot constructor, with `go test ./internal/features/... ./internal/config/...` still passing.

## TODO-FEAT-02b — Pin boot constructor with test

- Traces to: `GAP-FEAT-02`, same sources as TODO-FEAT-02a.
- Seam: same constructors — `func features.DefaultFeaturesConfig`
  (`internal/features/features.go:156-168`) vs
  `func features.FullyEnabledFeaturesConfig`
  (`internal/features/features.go:202-215`) via
  `func config.DefaultUserConfig`
  (`internal/config/user_config.go:1494`) and `func features.SetActive`
  (`internal/features/features.go:233-241`).
- Leaf work: add a test pinning the ADR-named constructor through
  `func config.DefaultUserConfig`
  (`internal/config/user_config.go:1494`) and the `func features.SetActive`
  (`internal/features/features.go:233-241`) reset path.
- Exit: `go test ./internal/features/... ./internal/config/...` passes with
  the pinning test.

## TODO-FEAT-03 — Surface or retire schema Keys

- Traces to: `GAP-FEAT-03` (evaluation: machine-enumerable keys, medium,
  Phase 3) fixed in `Docs/architecture/features/01-VISION.md:199-204`, row
  in `Docs/architecture/features/03-GAP-ANALYSIS.md:42`, specced in
  `Docs/architecture/features/06-CAPABILITY-SPEC.md:57-68`.
- Seam: `func features.ConfigSchemaKeys`
  (`internal/features/schema.go:56-65`) exists-but-uncalled vs
  `func features.ConfigSchemaJSON` (`internal/features/schema.go:21-52`)
  reachable via `main.featuresCmd`
  (`cmd/nerd/cmd_features.go:37` in `RunE`).
- Leaf work: first re-run full-repo `callers_of ConfigSchemaKeys` (searched-set
  no-caller is not unwired); then exactly one of (a) add a `--keys` (or
  equivalent) surface with a test driving it through
  `func features.ConfigSchemaKeys`
  (`internal/features/schema.go:56-65`), or (b) land a deletion commit
  removing `func features.ConfigSchemaKeys`
  (`internal/features/schema.go:56-65`) plus its tests with zero remaining
  refs while `func features.ConfigSchemaJSON`
  (`internal/features/schema.go:21-52`) keeps its reachable test via
  `main.featuresCmd` (`cmd/nerd/cmd_features.go:37`).
- Exit: `go test ./internal/features/... ./cmd/nerd/...` passes with either
  the new surface test or the deletion commit.

## TODO-FEAT-04a — Full-repo callers_of pass per registry accessor

- Traces to: `GAP-FEAT-04` (fenced registry, medium, Phase 3) fixed in
  `Docs/architecture/features/01-VISION.md:205-210`, row in
  `Docs/architecture/features/03-GAP-ANALYSIS.md:43`, lifecycle half owned by
  `Docs/architecture/features/05-CAPABILITY-SPEC.md:308-313`, evaluation
  gate owned by `Docs/architecture/features/06-CAPABILITY-SPEC.md:145-150`.
- Seam: `var features.boolFlags`
  (`internal/features/features.go:276-291`) table to enumerate, pointer read
  `func features.Active` (`internal/features/features.go:408`), per-call
  `func features.resolveBool` (`internal/features/features.go:419-434`).
- Leaf work: run full-repo `callers_of` for each accessor behind
  `var features.boolFlags` (`internal/features/features.go:276-291`) and
  record reader symbol plus line per flag, or explicit reserved candidate.
  Searched-set no-caller is not unwired — only a full-repo pass counts
  (blocking dep owned by gap row). Confidence 0.7 pending this pass — do not
  promote before it lands.
- Exit: full-repo `callers_of` evidence recorded as a table (in the TODO-FEAT-04b test or its commit message) listing one production reader symbol plus line per flag, or a reserved witness, with `go test ./internal/features/...` passing unchanged.

## TODO-FEAT-04b — Enumeration gate reaches zero unmarked

- Traces to: `GAP-FEAT-04`, same sources as TODO-FEAT-04a.
- Seam: same — `var features.boolFlags`
  (`internal/features/features.go:276-291`) plus
  `func features.resolveBool` (`internal/features/features.go:419-434`).
- Leaf work: add an enumeration test proving every row of
  `var features.boolFlags` (`internal/features/features.go:276-291`) has a
  production reader or an explicit reserved mark — reserved means parses plus
  resolves but no production reader by explicit decision, fenced by test.
  Depends on TODO-FEAT-04a and the lifecycle ADR for reserved meaning.
- Exit: `go test ./internal/features/...` passes with the enumeration test proving zero unmarked flags.

## TODO-FEAT-05 — Warn-only misconfiguration test

- Traces to: `GAP-FEAT-05` (evaluation: warn-only misconfiguration, low,
  Phase 3) fixed in `Docs/architecture/features/01-VISION.md:211-216`, row
  in `Docs/architecture/features/03-GAP-ANALYSIS.md:44`, specced in
  `Docs/architecture/features/06-CAPABILITY-SPEC.md:151-160`.
- Seam: `func features.envBool` (`internal/features/features.go:444-458`) plus
  `func features.envInt` (`internal/features/features.go:581-595`); only
  shadowed legacy warns via `func features.Deprecations`
  (`internal/features/features.go:341-363`); resolution via
  `func features.resolveBool` (`internal/features/features.go:419-434`).
  Caller owns logging per the report-don't-log contract
  (`internal/features/features.go:33-36`) plus P8 in
  `Docs/architecture/features/04-PRINCIPLES-AND-CONSTRAINTS.md:138-154`.
- Leaf work: add a caller-layer warn-only path plus test proving e.g.
  `CODENERD_DARK_MODE=yes` warns at the caller, resolves to no-override
  (the `func features.envBool`
  (`internal/features/features.go:444-458`) no-flip guarantee holds), and
  preserves P4/P7. Changing default-off safety itself needs its own ADR and
  is out of scope.
- Exit: `go test ./internal/features/... ./internal/config/...` passes with
  the new warn-only test.

## Closed log

- TODO-FEAT-01a, 01b — `df4a9d2` — wired; `TestNewDomainCortex_HonorsTheProvenanceFlag`.
- TODO-FEAT-02a, 02b — `791e821` — `TestBootTruth_*` (`internal/features/boot_truth_test.go`).
- TODO-FEAT-03 — `791e821` — `nerd features --schema --json`;
  `TestFeaturesCmd_WhenSchemaJSONRequested_ShouldEmitTheKeys`.
- TODO-FEAT-04a, 04b — `df4a9d2` — `TestEveryFlagHasAProductionReaderOrIsReserved`.
- TODO-FEAT-05 — `791e821` — `TestMisconfigurations_*`,
  `TestFeaturesCmd_WhenAnEnvValueIsRefused_ShouldWarn`.

## Non-goals (do not revive as TODO)

Per `Docs/architecture/features/01-VISION.md:218-254` and
`.nerd/campaigns/7b853890/artifacts/task_7b853890_3_0.md:49-53`:

- C4 promote `cmd/nerd/cmd_systems.go` from assumed to wired — already
  resolved: `var main.autopoiesisStatusCmd` (`cmd/nerd/cmd_systems.go:257-353`)
  calls `func features.IsPromptEvolutionEnabled`
  (`internal/features/features.go:549-552`), alongside
  `func features.ConfigSchemaJSON` (`internal/features/schema.go:21-52`) reachable
  via `var main.featuresCmd` (`cmd/nerd/cmd_features.go:18-86`). Stays as the worked
  Rule 1a example, not work.
- `diff_eval`: key removed — configuring it fails load via
  `var config.removedFeatureKeys`
  (`internal/config/removed_keys.go:24-31`). Keep discarded.
- `NERD_DISABLE_SYSTEM_SHARDS`: absent from every `.go` file per the comment
  at `internal/features/features.go:493-497` preceding
  `func features.IsSystemShardsEnabled`
  (`internal/features/features.go:498-501`). Keep discarded.
- Taxonomy wiring contradiction: resolved to wired-OFF via `func main.main`
  (`cmd/tools/verify_taxonomy/main.go:17`) plus
  `func features.IsTaxonomyFastEnabled`
  (`internal/features/features.go:535-538`). Not work.
- Anchor drifts (behaviour true, line fixes only): `func config.LoadUserConfig`
  (`internal/config/user_config.go:567`) installs via `func features.SetActive`
  (`internal/features/features.go:233-241`), `func config.LoadUserConfig`
  (`internal/config/user_config.go:571`) logs via `func features.Summary`
  (`internal/features/features.go:374-391`), `func config.LoadUserConfig`
  (`internal/config/user_config.go:576`) warns via
  `func features.Deprecations` (`internal/features/features.go:341-363`);
  `func main.main` (`cmd/nerd/main.go:395`) flight gate via
  `func features.IsFlightRecorderEnabled`
  (`internal/features/features.go:471-474`); `func features.resolveBool`
  (`internal/features/features.go:419-434`). No gap, no TODO.
