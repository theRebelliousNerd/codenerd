---
doc-class: governance
subsystem: features
implementation-status: not-applicable
last-verified: 2026-09-21
verified-against: 34634770970153e78c1e250fdab7abd888dcce6f
supersedes: []
---

# features: open questions

This file answers one question: what about `internal/features` is still
undecided, and what standing invariants must a future author preserve while
deciding it. It does not answer what is built (that is
`Docs/architecture/features/02-CURRENT-STATE.md:1-342`), what the finished
state looks like (that is
`Docs/architecture/features/01-VISION.md:1-268`), how lifecycle versus
evaluation are specced (those are
`Docs/architecture/features/05-CAPABILITY-SPEC.md:10-21` and
`Docs/architecture/features/06-CAPABILITY-SPEC.md:10-21`). Risks and recorded
decisions live in
`Docs/architecture/features/RISK-REGISTER-AND-DECISION-LOG.md:30-64`, and
nothing below is written as a risk.
Each question below names exactly one primary scope file with symbol and
line; related files appear only under "Traces to", never as additional
scopes. Each question traces to a buildable gap ID fixed in
`Docs/architecture/features/01-VISION.md:185-216` or to a cited seam in
today's code.

## Q1 — Does the provenance flag gate the kernel proof path, or is it reserved?

- **Scope:** `internal/features/features.go`, func `features.IsProvenanceEnabled`
  (`internal/features/features.go:479-482`).
- **Question:** should the flag flip the kernel proof path that `/explain`
  reads, or should it be marked reserved by decision?
- **Traces to:** GAP-FEAT-01 (`Docs/architecture/features/01-VISION.md:185-190`).
  Seam is the flag versus the kernel method
  `(k *RealKernel).IsProvenanceEnabled`
  (`internal/core/kernel_provenance.go:50-54`), of which the one observed
  chat caller reads the kernel's, not the flag's, at method
  `chat.Model.handleExplainCommand`
  (`cmd/nerd/chat/commands_handlers_misc.go:101`).
- **Why unresolved:** the flag parses and resolves today with zero production
  readers (test-only callers in `internal/features/*_test.go` plus
  `config_roundtrip_test.go` per
  `Docs/architecture/features/WIRING-AND-NOT-BUILT.md:92-98`); the safety
  knob exists and nothing turns it.
- **What closes it:** the GAP-FEAT-01 exit — a test asserting the flag flips
  the kernel proof path, or an ADR marking the flag reserved with a fencing
  witness — owned by the lifecycle spec
  (`Docs/architecture/features/05-CAPABILITY-SPEC.md:299-302`).

## Q2 — Which constructor is the fresh-boot source of truth?

- **Scope:** `internal/features/features.go`, func
  `features.DefaultFeaturesConfig`
  (`internal/features/features.go:156-168`).
- **Question:** for a fresh boot with no config and no env, is the source of
  truth the conservative defaults or what `nerd init` writes?
- **Traces to:** GAP-FEAT-02 (`Docs/architecture/features/01-VISION.md:191-198`).
  Seam is the constructor pair: func `features.DefaultFeaturesConfig`
  (`internal/features/features.go:156-168`) versus func
  `features.FullyEnabledFeaturesConfig`
  (`internal/features/features.go:202-215`), installed as what `nerd init`
  writes via func `config.DefaultUserConfig`
  (`internal/config/user_config.go:1494`), with the reset path in func
  `features.SetActive` (`internal/features/features.go:233-241`).
- **Why unresolved:** two constructors, one production reader
  (`FullyEnabledFeaturesConfig` via `config.DefaultUserConfig`); the other
  is test-only. Deleting either without a decision violates the
  audit-before-delete rule.
- **What closes it:** the GAP-FEAT-02 exit — an ADR naming the boot
  constructor with a witness plus a test pinning it through
  `config.DefaultUserConfig` and the `features.SetActive` reset path.

## Q3 — Does the machine-readable keys accessor earn a surface, or is it deleted?

- **Scope:** `internal/features/schema.go`, func
  `features.ConfigSchemaKeys` (`internal/features/schema.go:56-65`).
- **Question:** should the keys list gain a CLI or completion surface, or
  should it be removed in a commit?
- **Traces to:** GAP-FEAT-03 (`Docs/architecture/features/01-VISION.md:199-204`).
  Seam is the generated pair: func `features.ConfigSchemaKeys`
  (`internal/features/schema.go:56-65`) versus func
  `features.ConfigSchemaJSON` (`internal/features/schema.go:21-52`), the
  latter reachable via var `main.featuresCmd` `RunE` closure at
  `cmd/nerd/cmd_features.go:37` (`cmd/nerd/cmd_features.go:18-86`).
- **Why unresolved:** the keys accessor has only test callers
  (`internal/features/schema_test.go:14,54`); the JSON half is reachable.
  A generated accessor with no reader is either a missing tool or cruft.
- **What closes it:** the GAP-FEAT-03 exit — a surface test asserting the new
  output lists every key, or a deletion commit with no remaining references
  — owned by the evaluation spec
  (`Docs/architecture/features/06-CAPABILITY-SPEC.md:220-224`).

## Q4 — What does "reserved" mean, and how is the fence checked?

- **Scope:** `internal/features/features.go`, var `features.boolFlags`
  (`internal/features/features.go:276-291`).
- **Question:** what mark makes an unread flag explicitly reserved rather
  than silently uncalled, and what command-checkable gate enforces it?
- **Traces to:** GAP-FEAT-04 (`Docs/architecture/features/01-VISION.md:205-210`).
  Seam is the table resolved per call by func `features.resolveBool`
  (`internal/features/features.go:419-434`), read via the pointer in func
  `features.Active` (`internal/features/features.go:408`).
- **Why unresolved:** names are trusted, not fenced
  (`Docs/architecture/features/WIRING-AND-NOT-BUILT.md:130-132`); uncalled
  symbols are silent, not errors. The lifecycle half (what "reserved" means
  for resolution) and the evaluation half (enumeration gate) split across
  `Docs/architecture/features/05-CAPABILITY-SPEC.md:308-313` and
  `Docs/architecture/features/06-CAPABILITY-SPEC.md:145-150`.
- **What closes it:** the GAP-FEAT-04 exit — an enumeration test proving
  every `boolFlags` row has a production reader or an explicit reserved
  mark, reaching zero unmarked, only after a full-repo `callers_of` pass.

## Q5 — Where does the warn-only misconfiguration signal live?

- **Scope:** `internal/features/features.go`, func `features.envBool`
  (`internal/features/features.go:444-458`).
- **Question:** which caller-layer surface warns on an unparseable canonical
  env value while preserving the no-flip guarantee, given the leaf cannot
  log?
- **Traces to:** GAP-FEAT-05 (`Docs/architecture/features/01-VISION.md:211-216`).
  Seam is the strict parse in func `features.envBool`
  (`internal/features/features.go:444-458`) read through func
  `features.resolveBool` (`internal/features/features.go:419-434`), with
  integer discipline in func `features.envInt`
  (`internal/features/features.go:581-595`) via func
  `features.parseInt64` (`internal/features/features.go:597-609`), and the
  existing legacy-shadow reporter func `features.Deprecations`
  (`internal/features/features.go:341-363`).
- **Why unresolved:** a typo'd canonical value is no override, silent, by
  design; only shadowed legacy vars warn today. Warn-only hardening must
  live at the caller per the leaf contract
  (`internal/features/features.go:33-36`), without weakening default-off
  safety (`Docs/architecture/features/04-PRINCIPLES-AND-CONSTRAINTS.md:57-69`).
- **What closes it:** the GAP-FEAT-05 exit — a test proving an unparseable
  canonical value warns at the caller, flips nothing, and the suite passes.

## Tripwires (standing invariants — do not break while answering the above)

- **T1 — Leaf stays leaf.** Add no `codenerd/internal/*` import to
  `internal/features` (`internal/features/features.go:12-13`); stdlib only
  (`internal/features/features.go:57-62`). Ruling from
  `Docs/architecture/features/04-PRINCIPLES-AND-CONSTRAINTS.md:15-26`.
- **T2 — Stray exports never flip a bit.** Keep the strict parse in func
  `features.envBool` (`internal/features/features.go:444-458`) and func
  `features.envInt` (`internal/features/features.go:581-595`); Q5 may add a
  warning at the caller but must not coerce garbage to true.
- **T3 — Canonical wins, layers in order.** Keep func
  `features.resolveBool` (`internal/features/features.go:419-434`) reading
  canonical, then legacy only when canonical is absent or unparseable
  (`internal/features/features.go:416-418`), then active, then default.
- **T4 — Absent is not false; zero is not a value.** Keep booleans `*bool`
  (`internal/features/features.go:64-68`); keep tunables returning `0` for
  call-site default in func `features.FastScanWorkers`
  (`internal/features/features.go:557-565`) and func
  `features.FastASTMaxBytes` (`internal/features/features.go:568-576`),
  defaulted by func `world.DefaultScannerConfig`
  (`internal/world/scanner_config.go:29-38`).
- **T5 — Dangerous or expensive stays off.** Keep func
  `features.DefaultFeaturesConfig`
  (`internal/features/features.go:156-168`) conservative and func
  `features.FullyEnabledFeaturesConfig`
  (`internal/features/features.go:202-215`) keeping `PerShardFacts:false`
  and `PromptEvolution:false` per the audit
  (`internal/features/features.go:175-196`) until Q2/Q4 resolve otherwise.
- **T6 — Show resolved, not raw; report, don't log.** Keep func
  `features.Summary` (`internal/features/features.go:374-391`) resolving via
  func `features.Resolved` (`internal/features/features.go:308-328`) rather
  than printing raw fields (prior bug
  `internal/features/features.go:365-373`); keep func
  `features.Deprecations` (`internal/features/features.go:341-363`)
  returning strings for the caller contract
  (`internal/features/features.go:227-232`).

## Not open questions (stay discarded)

- `diff_eval`: key removed; configuring it fails load via var
  `config.removedFeatureKeys` (`internal/config/removed_keys.go:24-31`).
- `NERD_DISABLE_SYSTEM_SHARDS`: absent from every `.go` file per the comment
  preceding func `features.IsSystemShardsEnabled`
  (`internal/features/features.go:493-497`).
- Taxonomy contradiction: resolved to wired-OFF via func `main.main`
  (`cmd/tools/verify_taxonomy/main.go:17`) plus func
  `features.IsTaxonomyFastEnabled`
  (`internal/features/features.go:535-538`).
- Former assumed seam `cmd/nerd/cmd_systems.go:12`: resolved to reachable
  via `cmd/nerd/cmd_systems.go:300` inside var
  `main.autopoiesisStatusCmd` (`cmd/nerd/cmd_systems.go:257-353`); pattern
  for Rule 1a done right, not a question.
- Anchor drifts (behaviour true, lines stale): install via func
  `features.SetActive` (`internal/config/user_config.go:567`), log via func
  `features.Summary` (`internal/config/user_config.go:571`), warn via func
  `features.Deprecations` (`internal/config/user_config.go:576-578`); flight
  gate (`cmd/nerd/main.go:395`); precedence
  (`internal/features/features.go:419-434`). Fix lines, open nothing.
