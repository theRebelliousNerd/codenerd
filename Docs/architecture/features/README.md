---
doc-class: governance
subsystem: features
implementation-status: not-applicable
last-verified: 2026-09-26
verified-against: 34634770970153e78c1e250fdab7abd888dcce6f
supersedes: []
---

# features

`internal/features` is the leaf toggle registry every codeNERD feature flag
reads through, kept leaf so `internal/core` can read flags without an import
cycle with `internal/config` (`internal/features/features.go:1-13`). Precedence
is canonical `CODENERD_*` env, then legacy `NERD_*` env where one exists, then
the active `FeaturesConfig` installed by `func features.SetActive`
(`internal/features/features.go:233-241`), then the compile-time default,
resolved per call by `func features.resolveBool`
(`internal/features/features.go:419-434`) — there is no snapshot, so a late
`SetActive` applies to subsequent reads. The package is
`internal/features/features.go` plus `internal/features/schema.go` with
`func features.ConfigSchemaJSON` (`internal/features/schema.go:21-52`), and six
`*_test.go` files pin the registry.

## Read order

Full read order lives in `00-INDEX.md`; when in doubt read in slot order:

- `00-INDEX.md` — read order, one line per file saying what it answers and when to read it.
- `01-VISION.md` — the behaviour the finished package exhibits and why it matters to the north star.
- `02-CURRENT-STATE.md` — what is built and reachable today, file by file, every claim cited.
- `03-GAP-ANALYSIS.md` — the gap matrix; distance between shipped and finished, with checkable exits.
- `04-PRINCIPLES-AND-CONSTRAINTS.md` — numbered principles any change to this package must respect.
- `05-CAPABILITY-SPEC.md` — lifecycle capability spec: boot truth, resolution, and reserved meaning.
- `06-CAPABILITY-SPEC.md` — evaluation capability spec: enumeration gate and warn-only misconfiguration.
- `IMPLEMENTED_SPEC.md` — the authoritative record of shipped behaviour; wins on disagreement.
- `WIRING-AND-NOT-BUILT.md` — what is wired and reachable, what exists but nothing calls, and what the design assumes that the code does not do.
- `INTERNALS.md` — how a flag gets its value (registry, precedence, inspectors).
- `RISK-REGISTER-AND-DECISION-LOG.md` — risks with likelihood, consequence, and what would retire them.
- `OPEN-QUESTIONS.md` — unresolved design questions and standing invariants a future author must preserve.
- `TODO.md` — the build queue: leaf work only, each item traceable to a gap ID.
- `adr/ADR-001-features-scope.md` — five scope decisions D1-D5 (`GAP-FEAT-01..05`), each with context, decision, consequence, and witness; history (true 2026-09-21 at `verified-against 3463477…`): the ADR read `accepted-not-implemented` with witnesses not resolving — preserved, not deleted. Current truth 2026-09-26, supersedes that reading for D1/D3/D5: lane B wired D1/D3/D5 since (`internal/system/factory.go:1223` provenance reader, `cmd/nerd/cmd_features.go:41` schema-keys reader, `internal/config/user_config.go:612` misconfiguration warn) per `TODO.md:21-27` closed log and `WIRING-AND-NOT-BUILT.md` "Wired since 2026-09-25"; do not cite the 2026-09-21 never-built reading as current for D1/D3/D5.
- `corpus.toml` — machine-readable entrypoint (`entrypoint = "README.md"`) and source roots.