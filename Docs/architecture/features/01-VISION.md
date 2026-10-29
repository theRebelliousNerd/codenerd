---
doc-class: north-star
subsystem: features
implementation-status: target-state
last-verified: 2026-09-21
verified-against: 34634770970153e78c1e250fdab7abd888dcce6f
supersedes: []
---

# features: vision (finished behaviour)

This file describes what the finished `internal/features` package would
exhibit. Nothing in it claims the code does this today. Present-tense
claims about today live in the shipped layer for this package;
everything below is conditional until its gap row closes.

## Finished behaviour

A finished features registry would exhibit five properties:

1. **No silent flags.** Every entry in the boolean table would either have
   a production reader or carry an explicit `reserved` mark that a command
   can check. There would be no third state where a flag parses, resolves,
   and nothing reads it without anyone noticing.

2. **Provenance would be decided.** The `provenance` toggle would either
   gate the kernel proof path that `/explain` reads, or it would be marked
   reserved by decision with a witness — it would not linger as a
   parses-but-nothing-reads-it flag.

3. **One boot truth.** Fresh boot and `nerd init` output would agree on
   which constructor is the source of truth, recorded in a decision with a
   witness. Two constructors with one production reader would no longer be
   the steady state.

4. **Schema fully reachable or retired.** The generated schema's machine
   half would either be surfaced (`--keys`, completion, or equivalent) or
   deleted in a commit. A generated accessor with only test callers would
   no longer persist indefinitely.

5. **Misconfiguration visible without weakening safety.** A typo'd or
   unparseable canonical env value would still flip nothing (stray exports
   never flip a bit), but the operator would hear about it through the
   existing report path rather than running as `false` in silence.

## Why it matters to the north star

Each finished property serves a named sentence from codeNERD's north star.
A capability that serves none of those sentences would be a spec for a
different project.

- **No silent flags** serves NS-1 (`agents.md:7`): "The LLM handles problem
  solving, synthesis, and insight; the deterministic Mangle layer handles
  planning, memory, orchestration, and safety." An unfenced Go table where
  convention decides what a flag means is executive state held in Go, not
  a derived obligation. A fenced registry — every flag read or explicitly
  reserved — moves the decision from convention to something the harness
  can check. Seam: var `features.boolFlags`
  (`internal/features/features.go:276-291`) resolved per call by func
  `features.resolveBool` (`internal/features/features.go:419-434`).

- **No silent flags + provenance decided** serve NS-2 (`agents.md:9`):
  "This repo exists to make that split real in production: creative power
  with deterministic safety, long-horizon context without prompt drift, and
  parallel specialists whose behavior is grounded by logic rather than
  luck." A flag that gates `DerivationRecorder` for `/explain` but has no
  production reader is silent executive state: the safety-relevant knob
  exists and nothing turns it. Wiring it advances deterministic safety;
  explicitly reserving it stops pretending it does. Seam: func
  `features.IsProvenanceEnabled`
  (`internal/features/features.go:479-482`) vs method
  `(k *RealKernel).IsProvenanceEnabled`
  (`internal/core/kernel_provenance.go:50-54`) read at
  `cmd/nerd/chat/commands_handlers_misc.go:101`, method
  `chat.Model.handleExplainCommand`.

- **Schema reachable** serves the tools sentence (`agents.md:43-46`):
  "Tools exist for exactly three things: condense the search space, reduce
  the turns to complete the task, and offload cognition to deterministic
  code... A tool that does none of these is cruft." Generated schema that
  cannot drift is offload-to-deterministic-code. A `Keys` accessor with no
  surface is either a missing tool (wire it) or cruft (delete it) — the
  finished state refuses the middle. Seam: func
  `features.ConfigSchemaKeys` (`internal/features/schema.go:56-65`) vs func
  `features.ConfigSchemaJSON` (`internal/features/schema.go:21-52`)
  reachable via `cmd/nerd/cmd_features.go:37` in var `main.featuresCmd`
  `RunE` closure (`cmd/nerd/cmd_features.go:18-86`).

- **Schema reachable + provenance decided** serve the harness-decides
  sentence (`agents.md:27-29`): "Mangle — a deductive database programming
  language, not Datalog — plus JIT prompt compilation is meant to be the
  replacement for subagents and skills: the harness decides, not the
  model's discretion." Inspection surfaces (`nerd features`, `/features`)
  that render resolved values plus sources are the harness showing its
  work; an inspector nobody can reach from production is discretionary
  surface, not harness surface. Seam: func `features.Resolved`
  (`internal/features/features.go:308-328`) rendered at
  `cmd/nerd/cmd_features.go:44` plus tunables at
  `cmd/nerd/cmd_features.go:52-53`.

- **No silent flags + one boot truth** serve the fixpoint sentences
  (`agents.md:47-53`): "\"Clean fixpoint,\" not \"clean loop.\" Go is ...
  the FFI, the drivers, the tools. The executive decisions ... are the
  fixpoint of the kernel over the facts. The drift to hunt is decisions
  computed in Go instead of derived." and "what is scored is whether a
  decision is derived and whether an obligation is forced." Unfenced flag
  names and two competing boot constructors are decisions computed in Go
  by convention. The finished state forces them: enumerate every flag
  against its readers, name the boot constructor, fail loudly on drift.
  Seam: func `features.DefaultFeaturesConfig`
  (`internal/features/features.go:156-168`) vs func
  `features.FullyEnabledFeaturesConfig`
  (`internal/features/features.go:202-215`) via func
  `config.DefaultUserConfig` (`internal/config/user_config.go:1494`).

- **Misconfiguration visible** serves the pressure sentence
  (`agents.md:55-59`): "The north star is a final state... the harness
  continuously derives the distance ... and applies that distance as
  pressure on every agent... pressure never lets go." Silent
  `CODENERD_DARK_MODE=yes`-means-false hides the distance between what the
  operator asked and what runs. Warn-only surfacing preserves the safety
  property (never flip on garbage) while refusing to hide the distance.
  Seam: func `features.envBool` (`internal/features/features.go:444-458`)
  through func `features.resolveBool`
  (`internal/features/features.go:419-434`), warnings via func
  `features.Deprecations` (`internal/features/features.go:341-363`).

- **This file obeys the read-code sentence** (`agents.md:60-63`):
  "`Docs/architecture/` (July 2026) is not the original and not
  authoritative... never cite it as evidence of anything. Read the code."
  Every seam below is drawn from Go read this campaign, not from the July
  corpus. Discards at the foot stay discarded for the same reason.

## Seams this vision would attach to (today's code)

These are the attachment points in today's tree where the finished
behaviour would land. All paths are repo-relative with symbol and line.

- `internal/features/features.go:479-482`, func
  `features.IsProvenanceEnabled` — the undecided flag (default OFF,
  `CODENERD_PROVENANCE` only). Distinct symbol from the kernel method
  `internal/core/kernel_provenance.go:50-54`, method
  `(k *RealKernel).IsProvenanceEnabled`, which is what the one chat caller
  reads (`cmd/nerd/chat/commands_handlers_misc.go:101`, method
  `chat.Model.handleExplainCommand` calling
  `m.kernel.IsProvenanceEnabled()`).
- `internal/features/features.go:156-168`, func
  `features.DefaultFeaturesConfig` (conservative compile-time defaults)
  vs `internal/features/features.go:202-215`, func
  `features.FullyEnabledFeaturesConfig` (what `nerd init` writes, via
  `internal/config/user_config.go:1494`, func `config.DefaultUserConfig`).
  Reset path `internal/features/features.go:233-241`, func
  `features.SetActive` (`nil` resets to defaults).
- `internal/features/schema.go:56-65`, func `features.ConfigSchemaKeys`
  (test-only today) vs `internal/features/schema.go:21-52`, func
  `features.ConfigSchemaJSON` (reachable via `cmd/nerd/cmd_features.go:37`
  in var `main.featuresCmd` `RunE` closure, `cmd/nerd/cmd_features.go:18-86`).
- `internal/features/features.go:276-291`, var `features.boolFlags`
  (single source of truth for the 8 bools) resolved per call by
  `internal/features/features.go:419-434`, func `features.resolveBool`;
  pointer read at `internal/features/features.go:408`, func
  `features.Active`.
- `internal/features/features.go:444-458`, func `features.envBool`
  (strict parse: only `1`/`true`, `0`/`false`) plus
  `internal/features/features.go:581-595`, func `features.envInt`, read
  through `internal/features/features.go:419-434`, func
  `features.resolveBool`; shadowed-legacy warnings via
  `internal/features/features.go:341-363`, func `features.Deprecations`.
- `internal/features/features.go:549-552`, func
  `features.IsPromptEvolutionEnabled`, read at
  `cmd/nerd/cmd_systems.go:300` inside var
  `main.autopoiesisStatusCmd` (`cmd/nerd/cmd_systems.go:257-353`) — the
  former assumed seam, now the pattern for what "resolved to reachable"
  looks like.

## Buildable gaps (from judged candidates C1–C6)

Prior ambitions below are the Rule-2 plan candidates judged in
`.nerd/campaigns/7b853890/artifacts/task_7b853890_3_0.md:19-47` (C1–C6)
against the sentences above. Each ends in a command-checkable exit. Full
rows (current/target/severity/phase) belong in the gap matrix for this
package; this file fixes the IDs and exits so that table cannot redefine
them.

- **GAP-FEAT-01 (C1 — provenance decided).** Exit: either a test asserting
  the `features.IsProvenanceEnabled`
  (`internal/features/features.go:479-482`) flag flips the kernel proof
  path read at `cmd/nerd/chat/commands_handlers_misc.go:101`, or an ADR
  marking the flag reserved whose witness (reserved mark + fencing test)
  resolves.
- **GAP-FEAT-02 (C2 — one boot truth).** Exit: an ADR with a witness naming
  which of `features.DefaultFeaturesConfig`
  (`internal/features/features.go:156-168`) vs
  `features.FullyEnabledFeaturesConfig`
  (`internal/features/features.go:202-215`) fresh boot uses, plus a test
  pinning that constructor through `config.DefaultUserConfig`
  (`internal/config/user_config.go:1494`) and the `features.SetActive`
  (`internal/features/features.go:233-241`) reset path.
- **GAP-FEAT-03 (C3 — schema Keys surfaced or retired).** Exit: a test
  driving a `--keys` (or equivalent) surface through
  `features.ConfigSchemaKeys` (`internal/features/schema.go:56-65`), or a
  deletion commit removing it while `features.ConfigSchemaJSON`
  (`internal/features/schema.go:21-52`) keeps its reachable test via
  `cmd/nerd/cmd_features.go:37`.
- **GAP-FEAT-04 (C5 — fenced registry).** Exit: a command-checkable test
  enumerating every row of `features.boolFlags`
  (`internal/features/features.go:276-291`) and proving each has a
  production reader or an explicit reserved mark — zero unmarked — reached
  only after a full-repo `callers_of` pass (searched-set no-caller is not
  unwired).
- **GAP-FEAT-05 (C6 — warn-only misconfiguration).** Exit: a test proving
  an unparseable canonical env value still flips nothing (the
  `features.envBool` `internal/features/features.go:444-458` no-override
  guarantee holds) while producing a warning through the
  `features.Deprecations`-style report path
  (`internal/features/features.go:341-363`).

C4 (promote `cmd/nerd/cmd_systems.go` from assumed to wired) is not a gap:
fresh read closed it — `cmd/nerd/cmd_systems.go:300` calls
`features.IsPromptEvolutionEnabled`
(`internal/features/features.go:549-552`). It stays as the worked example
of Rule 1a done right (import alone was never evidence; the exact call
is), alongside the already-resolved import at `cmd/nerd/cmd_features.go:8`
(resolved as reachable via func `features.ConfigSchemaJSON()` at
`cmd/nerd/cmd_features.go:37` in var `main.featuresCmd` `RunE` closure,
`cmd/nerd/cmd_features.go:18-86`).

## What this vision does not revive

The following stay discarded per the code reads behind
`Docs/architecture/features/WIRING-AND-NOT-BUILT.md:87-163` and
`.nerd/campaigns/7b853890/artifacts/task_7b853890_3_0.md:49-53`. They are
not gaps and get no IDs.

- `diff_eval` / `IsDiffEvalEnabled`: key removed; configuring it fails load
  (`internal/config/removed_keys.go:24-31`, var
  `config.removedFeatureKeys`). Keep discarded.
- `NERD_DISABLE_SYSTEM_SHARDS`: absent from every `.go` file per the
  comment preceding func `features.IsSystemShardsEnabled`
  (`internal/features/features.go:498-501`, comment at
  `internal/features/features.go:493-497`). Keep discarded.
- Taxonomy wiring contradiction: resolved to wired-OFF
  (`cmd/tools/verify_taxonomy/main.go:17`, func `main.main`, plus func
  `features.IsTaxonomyFastEnabled`
  (`internal/features/features.go:535-538`)). Not a candidate.
- Anchor drifts (behaviour true, line fixes only): func
  `config.LoadUserConfig` installs via `features.SetActive`
  (`internal/config/user_config.go:567`), logs via `features.Summary()`
  (`internal/config/user_config.go:571`), warns via
  `features.Deprecations()` (`internal/config/user_config.go:576-578`) vs
  older `561-572`; func `main.main` flight gate via
  `features.IsFlightRecorderEnabled()` (`cmd/nerd/main.go:395`) vs older
  `387`; func `features.resolveBool`
  (`internal/features/features.go:419-434`) vs older `410-434`. No gap.

## Principles this vision preserves

From `Docs/architecture/features/04-PRINCIPLES-AND-CONSTRAINTS.md:15-183`
(P1 leaf through P10 generated schema): P1 (leaf package, stdlib only),
P2 (tables are the single source of truth), P3 (four-layer precedence,
canonical wins), P4 (stray exports never flip a bit — C6 warn-only must
not weaken this), P5 (no snapshots, atomic pointer), P6
(absent-is-not-false / zero-is-not-a-value), P7 (conservative defaults —
dangerous or expensive stays off), P8 (report, don't log; show resolved,
not raw), P9 (legacy names are migration debt with a removal criterion),
P10 (schema is generated, not transcribed). Any gap closure that violates
one of these is not the finished state described here.
