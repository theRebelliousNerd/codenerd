---
doc-class: shipped-with-future
subsystem: features
implementation-status: partial
last-verified: 2026-09-26
verified-against: 34634770970153e78c1e250fdab7abd888dcce6f
supersedes: []
---

# features: gap analysis (current vs target)

> **Status, 2026-09-26 (lane B build-out).** The matrix rows below are kept as written — observed 2026-09-21 true-then against `34634770970153e78c1e250fdab7abd888dcce6f`; their `features.go` anchors predate `Misconfigurations` and have shifted (see `Docs/architecture/features/TODO.md:10-27` status for current anchors and closed log). History preserved, never deleted.

This file answers the third question for `internal/features`: what the
distance is between what runs today and the finished behaviour. Each row
is a unit of work a campaign could pick up, with a command-checkable exit.

IDs are stable (`GAP-FEAT-01..05`). A closed gap would stay in the table,
marked closed with the commit that closed it; no gap is deleted. IDs and
exits are fixed in `Docs/architecture/features/01-VISION.md:176-216` —
this file adds current/target/severity/phase/deps rows and cannot redefine
them.

ADR slot (observed 2026-09-21): no `adr/` directory and no `adr/ADR-NNN-*.md` files exist yet.
`DEC-FEAT-01..10` in `Docs/architecture/features/RISK-REGISTER-AND-DECISION-LOG.md:52-63`
are a decision log with witnesses, not `adr/` files. Creating the GAP-FEAT-01/02
ADRs is the work itself, traced by `TODO-FEAT-01a/02a` via
`test -f adr/ADR-NNN-*.md` exits in `Docs/architecture/features/TODO.md:49-101`;
no ADR is claimed to exist here.
Correction 2026-09-26: the `no adr/` clause above is superseded — `Docs/architecture/features/adr/ADR-001-features-scope.md:1-175` exists with five decisions D1-D5 and witnesses per `Docs/journeys/09-architecture-doc-standard.md:82-88`; `DEC-FEAT-01..10` remain decision log, not `adr/` files.

Evidence base (all read 2026-09-21, verified against
`34634770970153e78c1e250fdab7abd888dcce6f`):
`Docs/architecture/features/01-VISION.md:176-216` fixes IDs and exits;
`Docs/architecture/features/05-CAPABILITY-SPEC.md:425-431` owns
GAP-FEAT-01/02/04-lifecycle; `Docs/architecture/features/06-CAPABILITY-SPEC.md:218-231`
owns GAP-FEAT-03/04-eval/05;
`Docs/architecture/features/WIRING-AND-NOT-BUILT.md:15-85` reachable vs
`:87-109` exists-but-uncalled vs `:111-136` assumed;
`Docs/architecture/features/02-CURRENT-STATE.md:278-325` shipped reachable;
`Docs/architecture/features/04-PRINCIPLES-AND-CONSTRAINTS.md:15-183` P1-P10;
`agents.md:7,9,27-29,43-53,55-63` vision sentences. Currency: 2026-09-21
rewrite, supersedes July corpus per `agents.md:60-63` — July docs are never
evidence.

## Gap matrix

| Gap ID | Capability | Current state (shipped, cited) | Target state (spec) | Severity | Phase | Blocking dependencies | Exit criteria (command-checkable) |
|---|---|---|---|---|---|---|---|
| GAP-FEAT-01 (C1) | lifecycle: provenance decided | `func features.IsProvenanceEnabled` (`internal/features/features.go:479-482`) exists-but-uncalled: zero prod callers, only `internal/features/*_test.go` plus round-trip coverage; distinct symbol from `(*RealKernel).IsProvenanceEnabled` (`internal/core/kernel_provenance.go:50-54`); the one chat caller uses the kernel method, not the flag (`cmd/nerd/chat/commands_handlers_misc.go:101` in `chat.Model.handleExplainCommand`). Source: `Docs/architecture/features/05-CAPABILITY-SPEC.md:240-248`, `Docs/architecture/features/WIRING-AND-NOT-BUILT.md:92-98`, `Docs/architecture/features/01-VISION.md:139-146`. | The flag seam at `internal/features/features.go:479-482` would attach to the kernel proof path read at `cmd/nerd/chat/commands_handlers_misc.go:101` per `Docs/architecture/features/05-CAPABILITY-SPEC.md:113-119` plus `:299-302`, or the flag would carry an explicit reserved mark by decision with a witness. | high | 3 | kernel proof-path owner agrees on wiring vs reserved | Either a test asserting the `features.IsProvenanceEnabled` (`internal/features/features.go:479-482`) flag flips the kernel proof path read at `cmd/nerd/chat/commands_handlers_misc.go:101`, or an ADR marking the flag reserved whose witness (reserved mark plus fencing test) resolves; `go test ./internal/features/... ./internal/core/... ./cmd/nerd/...` passes. Fixed in `Docs/architecture/features/01-VISION.md:185-190`. |
| GAP-FEAT-02 (C2) | lifecycle: one boot truth | Two constructors, one prod reader: `func features.DefaultFeaturesConfig` (`internal/features/features.go:156-168`) is test-only (sole caller `internal/features/features_defaults_test.go:6`) while `func features.FullyEnabledFeaturesConfig` (`internal/features/features.go:202-215`) is reachable via `func config.DefaultUserConfig` (`internal/config/user_config.go:1494`); reset `func features.SetActive` (`internal/features/features.go:233-241`, nil resets to defaults) is the installer. Source: `Docs/architecture/features/05-CAPABILITY-SPEC.md:253-257`, `Docs/architecture/features/WIRING-AND-NOT-BUILT.md:102-105`, `Docs/architecture/features/02-CURRENT-STATE.md:76-92`. | The seam pair `internal/features/features.go:156-168` vs `:202-215` via `internal/config/user_config.go:1494` would resolve to a single named fresh-boot source of truth with a pinning test per `Docs/architecture/features/05-CAPABILITY-SPEC.md:66-80`. | medium | 3 | audit-before-delete per live rule; no deletion without ADR | An ADR with a witness naming which constructor fresh boot uses, plus a test pinning that constructor through `config.DefaultUserConfig` (`internal/config/user_config.go:1494`) and the `features.SetActive` reset path (`internal/features/features.go:233-241`), with `go test ./internal/features/... ./internal/config/...` passing. Fixed in `Docs/architecture/features/01-VISION.md:191-198`. |
| GAP-FEAT-03 (C3) | evaluation: machine-enumerable keys | `func features.ConfigSchemaKeys` (`internal/features/schema.go:56-65`) exists-but-uncalled: 2 callers both in `internal/features/schema_test.go:14,54`, no prod caller; contrast reachable `func features.ConfigSchemaJSON` (`internal/features/schema.go:21-52`) via `nerd features --schema` (`cmd/nerd/cmd_features.go:37` in `main.featuresCmd` RunE). Source: `Docs/architecture/features/06-CAPABILITY-SPEC.md:115-120`, `Docs/architecture/features/WIRING-AND-NOT-BUILT.md:106-109`. | The seam at `internal/features/schema.go:56-65` would either gain a surface (CLI flag or completion reading the same tables) or be removed per `Docs/architecture/features/06-CAPABILITY-SPEC.md:57-68`, while `internal/features/schema.go:21-52` would keep its reachable path via `cmd/nerd/cmd_features.go:37`. | medium | 3 | full-repo `callers_of ConfigSchemaKeys` re-check before deletion | `go test ./internal/features/... ./cmd/nerd/...` passes with either a new surface test asserting CLI or completion lists every key, or a commit deleting `ConfigSchemaKeys` plus its tests with no remaining refs. Fixed in `Docs/architecture/features/01-VISION.md:199-204`. |
| GAP-FEAT-04 (C5) | fenced registry — lifecycle half plus evaluation half (joint) | Names trusted, not fenced: `var features.boolFlags` (`internal/features/features.go:276-291`) is the table to enumerate, pointer read `func features.Active` (`internal/features/features.go:408`), per-call `func features.resolveBool` (`internal/features/features.go:419-434`); uncalled symbols stay silent, not errors. Source: `Docs/architecture/features/05-CAPABILITY-SPEC.md:308-313`, `Docs/architecture/features/06-CAPABILITY-SPEC.md:145-150`, `Docs/architecture/features/WIRING-AND-NOT-BUILT.md:128-132`. | Every row of `internal/features/features.go:276-291` would have a prod reader or an explicit reserved mark per `Docs/architecture/features/05-CAPABILITY-SPEC.md:308-313` (what reserved means for resolution) and `Docs/architecture/features/06-CAPABILITY-SPEC.md:145-150` (enumeration gate); reserved would mean parses plus resolves but no prod reader by explicit decision fenced by test. | medium | 3 | full-repo `callers_of` for each accessor (searched-set no-caller is not unwired); lifecycle ADR for reserved meaning; evaluation gate owned by `06-CAPABILITY-SPEC.md` | A command-checkable test enumerating every `boolFlags` row (`internal/features/features.go:276-291`) and proving each has a production reader symbol plus line or a reserved witness — zero unmarked — reached only after a full-repo `callers_of` pass. Fixed in `Docs/architecture/features/01-VISION.md:205-210`. Confidence 0.7 pending full-repo pass — do not promote. |
| GAP-FEAT-05 (C6) | evaluation: warn-only misconfiguration | Typo'd canonical env is no override, silent: `func features.envBool` (`internal/features/features.go:444-458` via `:456-457`) plus `func features.envInt` (`internal/features/features.go:581-595`); only shadowed legacy warns via `func features.Deprecations` (`internal/features/features.go:341-363`). For example `CODENERD_DARK_MODE=yes` resolves false silent. Source: `Docs/architecture/features/06-CAPABILITY-SPEC.md:151-160`, `Docs/architecture/features/WIRING-AND-NOT-BUILT.md:133-136`, P4 `Docs/architecture/features/04-PRINCIPLES-AND-CONSTRAINTS.md:57-69`. | The caller layer around `internal/features/features.go:444-458` via `internal/features/features.go:419-434` would emit a warn-only log for an unparseable canonical value while preserving no-flip per `Docs/architecture/features/06-CAPABILITY-SPEC.md:151-160`; the default-off safety itself would stay unchanged. | low | 3 | ADR if default-off safety itself changed (out of scope); caller owns logging per `internal/features/features.go:33-36` plus P8 | A new test asserting `CODENERD_DARK_MODE=yes` warns at the caller, resolves to no-override, and `go test ./internal/features/... ./internal/config/...` passes. Fixed in `Docs/architecture/features/01-VISION.md:211-216`. Preserves P4/P7. |

## Vision trace (each gap serves a named `agents.md` sentence)

- GAP-FEAT-01 serves NS-2 (`agents.md:9` deterministic safety) plus NS-1 (`agents.md:7`): the silent executive-state seam `internal/features/features.go:479-482` vs `internal/core/kernel_provenance.go:50-54` read at `cmd/nerd/chat/commands_handlers_misc.go:101` would either advance safety by wiring or stop pretending by reserving. Per `Docs/architecture/features/01-VISION.md:62-75`, `Docs/architecture/features/05-CAPABILITY-SPEC.md:374-386`.
- GAP-FEAT-02 serves V-decides (`agents.md:27-29` harness decides) plus V-fixpoint/V-derived (`agents.md:47-53`): the two-boot-truths seam `internal/features/features.go:156-168` vs `:202-215` via `internal/config/user_config.go:1494` would no longer leave the source of truth to discretion. Per `Docs/architecture/features/01-VISION.md:101-114`, `Docs/architecture/features/05-CAPABILITY-SPEC.md:387-399`.
- GAP-FEAT-03 serves V-tools (`agents.md:43-46`): the generated-keys seam `internal/features/schema.go:56-65` vs `:21-52` reachable via `cmd/nerd/cmd_features.go:37` would either gain a tool that condenses search space or be retired as cruft. Per `Docs/architecture/features/01-VISION.md:77-87`, `Docs/architecture/features/06-CAPABILITY-SPEC.md:208-211`.
- GAP-FEAT-04 serves NS-1 (`agents.md:7`) plus V-fixpoint/V-derived (`agents.md:47-53`) plus V-pressure (`agents.md:55-59`): the unfenced-names seam `internal/features/features.go:276-291` plus `internal/features/features.go:419-434` plus `internal/features/features.go:408` would move from convention in Go to derived and forced obligation. Per `Docs/architecture/features/01-VISION.md:52-60`, `Docs/architecture/features/05-CAPABILITY-SPEC.md:400-418`.
- GAP-FEAT-05 serves V-pressure (`agents.md:55-59`): the silent seam `internal/features/features.go:444-458` via `internal/features/features.go:419-434` with warnings via `internal/features/features.go:341-363` would surface distance while preserving P4 no-flip. Per `Docs/architecture/features/01-VISION.md:116-126`, `Docs/architecture/features/06-CAPABILITY-SPEC.md:203-207`. Changing default-off itself would need an ADR (out of scope).

## Non-gaps (do not revive)

- C4 `cmd/nerd/cmd_systems.go:12` assumed is now RESOLVED reachable via `cmd/nerd/cmd_systems.go:300` calling `features.IsPromptEvolutionEnabled` (`internal/features/features.go:549-552`) inside `main.autopoiesisStatusCmd` (`cmd/nerd/cmd_systems.go:257-353`). Worked Rule 1a example alongside `cmd/nerd/cmd_features.go:8` resolved via `:37`. Per `Docs/architecture/features/01-VISION.md:218-226`.
- D1 `diff_eval`: removed (`internal/config/removed_keys.go:24-31`) — configuring it fails load. D2 `NERD_DISABLE_SYSTEM_SHARDS`: absent from every `.go` (`internal/features/features.go:493-497` comment). D3 taxonomy contradiction: resolved wired-OFF (`cmd/tools/verify_taxonomy/main.go:17` plus `internal/features/features.go:527-538`). D4 anchor drifts: behaviour true, lines tighten to `internal/config/user_config.go:567/571/576`, `cmd/nerd/main.go:395`, `internal/features/features.go:419-434`. Per `Docs/architecture/features/01-VISION.md:228-254`, `Docs/architecture/features/05-CAPABILITY-SPEC.md:457-461`.
- Principles preserved: P1 leaf stdlib-only, P2 tables single source of truth, P3 canonical-wins, P4 never-flip-on-garbage, P5 atomic no-snapshot, P6 absent-is-not-false, P7 conservative defaults, P8 report-do-not-log plus show-resolved, P9 legacy removal criterion, P10 generated schema. Per `Docs/architecture/features/04-PRINCIPLES-AND-CONSTRAINTS.md:15-183`, `Docs/architecture/features/01-VISION.md:256-267`.
