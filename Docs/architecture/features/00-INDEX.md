---
doc-class: governance
subsystem: features
implementation-status: not-applicable
last-verified: 2026-09-24
verified-against: 34634770970153e78c1e250fdab7abd888dcce6f
supersedes: []
---

# features: index (read order)

This index is the read order for the `internal/features` architecture
directory. Start at `README.md` for the one-paragraph summary, then read in
slot order below. One line per file: what question it answers and when to
read it.

## Read order

- `README.md` — what this directory is, in a paragraph; read first, then come here for order.
- `00-INDEX.md` (this file) — read order plus which files describe today vs plan; read second, or whenever lost.
- `01-VISION.md` — the behaviour the finished package exhibits and why it matters to the north star; read before any plan file to know the dream.
- `02-CURRENT-STATE.md` — what is built and reachable today, file by file, every claim cited; read before judging any gap or spec claim.
- `03-GAP-ANALYSIS.md` — the gap matrix: distance between shipped and finished, each row with a checkable exit; read when picking work.
- `04-PRINCIPLES-AND-CONSTRAINTS.md` — numbered principles any change to this package must respect; read before writing code.
- `05-CAPABILITY-SPEC.md` — lifecycle capability spec: install and resolve (`FeaturesConfig`, defaults, `SetActive`, resolution precedence); read when building boot-truth or resolution work.
- `06-CAPABILITY-SPEC.md` — evaluation capability spec: turning resolved values into decisions (enumeration gate, warn-only misconfiguration); read when building evaluation work, not lifecycle.
- `IMPLEMENTED_SPEC.md` — the authoritative record of shipped behaviour; read when two files disagree, this one wins.
- `WIRING-AND-NOT-BUILT.md` — what is wired and reachable, what exists but nothing calls, what the design assumes the code does not do; read when asking "does this run?".
- `INTERNALS.md` — how a flag gets its value (registry, precedence, inspectors); read for the mechanism behind the current-state claims.
- `adr/ADR-001-features-scope.md` — scope-gate decisions with witnesses; read when changing scope, boot truth, reserved meaning, or misconfiguration signalling.
- `RISK-REGISTER-AND-DECISION-LOG.md` — risks with likelihood, consequence, and what would retire them, plus recorded decisions; read before taking on a gap.
- `OPEN-QUESTIONS.md` — unresolved design questions and standing invariants a future author must preserve; read before answering a question someone already framed.
- `TODO.md` — the build queue: leaf work only, each item traceable to a gap ID; read when ready to build.
- `corpus.toml` — machine-readable entrypoint and source roots; read when tooling needs the directory pointer.

## Grounded vs hypothesized

Grounded files describe the code as it runs today. Hypothesized files
describe what does not exist yet and say so at the top. Governance files
describe neither; they record process, decisions, risks, questions, and work
queue. Do not read a hypothesized file as evidence that something is built.

### Grounded (shipped — describes today)

- `02-CURRENT-STATE.md` (`doc-class: shipped`, `implementation-status: shipped`) — every claim cited to code read this pass.
- `IMPLEMENTED_SPEC.md` (`doc-class: shipped`, `implementation-status: shipped`) — authoritative. On any disagreement with another file about what `internal/features` does today, this file wins (`IMPLEMENTED_SPEC.md:12-15`).
- `WIRING-AND-NOT-BUILT.md` — shipped-layer content: wired-and-reachable vs exists-but-uncalled, with caller evidence. Note: carries a `Verified 2026-09-21 against commit 3463477…` line (`WIRING-AND-NOT-BUILT.md:3-9`) instead of front-matter.
- `INTERNALS.md` — shipped-layer explainer: how a flag resolves. Note: carries a `Verified 2026-09-21 against commit 3463477…` line (`INTERNALS.md:3-5`) instead of front-matter.

### Hypothesized (plan — describes what is not built yet)

- `01-VISION.md` (`doc-class: north-star`, `implementation-status: target-state`) — finished behaviour; states nothing in it claims the code does this today (`01-VISION.md:12-15`).
- `03-GAP-ANALYSIS.md` (`doc-class: shipped-with-future`, `implementation-status: partial`) — mixed by design: current-state half cited like shipped, target-state half points at the spec that details it.
- `05-CAPABILITY-SPEC.md` (`doc-class: shipped-with-future`, `implementation-status: partial`) — lifecycle spec; defers evaluation to `06`.
- `06-CAPABILITY-SPEC.md` (`doc-class: shipped-with-future`, `implementation-status: partial`) — evaluation spec; defers lifecycle to `05`.

### Governance (process — neither shipped nor plan)

- `README.md`, `00-INDEX.md`, `04-PRINCIPLES-AND-CONSTRAINTS.md`, `RISK-REGISTER-AND-DECISION-LOG.md`, `OPEN-QUESTIONS.md`, `TODO.md` (`doc-class: governance`, `implementation-status: not-applicable`).
- `adr/ADR-001-features-scope.md` (`doc-class: governance`, `implementation-status: accepted-not-implemented`) — all five subjects never built; status derived from witnesses, never asserted.
- `corpus.toml` — machine pointer (`entrypoint = "README.md"`). Note: its `implemented_spec = "README.md"` entry (`corpus.toml:6`) predates the shipped record; the authoritative record per the standard is `IMPLEMENTED_SPEC.md`.
