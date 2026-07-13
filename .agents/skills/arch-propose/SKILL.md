---
name: arch-propose
description: >
  Research-driven architecture corpus generator for pre-implementation codeNERD features. Takes a feature
  idea, runs 4 parallel scouts (internal, literature, convergent, divergent), synthesizes
  architectural candidates, stress-tests via Socratic interrogation, and generates a 22+ doc
  architecture corpus in `Docs/architecture/<feature>/` marked as Pre-Implementation. Also handles
  partial-corpus expansion via `--expand`. Use when asked to design/investigate a new feature
  or subsystem, or to expand an existing pre-implementation corpus. Do NOT use for code-first doc
  generation on existing subsystems (use `spec-doc-sprint`), algorithm reformulation (use `mangle-programming`),
  or spec-to-code implementation (use `corpus-build`).
metadata:
  version: 2.0.0
  author: codeNERD (full ecosystem port from Vectryx; mutated for codeNERD)
  last-verified: 2026-07-13
---

# Arch-Propose — Pre-Implementation Architecture Corpus Generator

> **codeNERD full ecosystem port.** Entire skill tree copied from Vectryx then mutated file-by-file. Agent fleet lives under `.grok/agents/` and `.claude/agents/` with bound `skills:` frontmatter. Architecture corpora: `Docs/architecture/`. North star: LLM creative / Mangle executive; `permitted(...)` default deny; JIT prompt atoms.


Research-driven sibling to `spec-doc-sprint` (which fills product Specs from living code).
`/arch-propose` starts from a feature idea with little/no code, runs research scouts, and
produces a deep architecture corpus under `Docs/architecture/<feature>/`.

**Pipeline position**: `arch-propose` → (human decides to build) → `corpus-build` → `spec-doc-sprint` (once code exists) → `nerd-evolve`

## Core Loop

```
NorthStarInterview(inline) -> Research(4 parallel scouts) -> Synthesize(2-3 candidates)
  -> SchemaValidate(hard gate: package-scope + RBW + integration surface + planned inventory + invariants + gates)
  -> Interrogate(requirements-interrogator, max 3 rounds, fail-closed verdict regex) -> Judge -> SyntheticAuditor
  -> CorpusDiff + Renumbering Map (--expand only) -> CorpusGeneration (4 parallel writers:
       arch-writer + cross-cutting-analyst + test-strategist + ecosystem-mapper)
  -> Governance (TODO / OPEN-QUESTIONS / CLAUDE / README / _progress) -> Docs/architecture/INDEX patch
  -> Compliance Gate (Pre-Impl markers + no-time-estimates grep) -> Journal
```

## Output Corpus Contents

Per Tier 3 (22+ files), the slash command produces:

**Numbered (template-driven)**: 5 foundation (00-04) + 1 IMPLEMENTED_SPEC + 2-10 deep-dives (05-NN, by tier) + 10 cross-cutting (NN+1 to NN+10 in fixed order)

**Un-numbered (governance + pre-implementation-specific)**:
- `TODO.md`, `OPEN-QUESTIONS.md`, `CLAUDE.md`, `README.md`, `_progress.md`
- `TESTING-STRATEGY.md` — deep test plan from `arch-propose-test-strategist`
- `ECOSYSTEM-IMPACT.md` — full ripple map + implementer checklist from `arch-propose-ecosystem-mapper`

**Tier 1+2 only**: `adr/` directory with ADRs (the candidate-selection ADR is auto-generated).

## Entry Points

Invoke via `/arch-propose <feature-name> [flags]`. See `.claude/commands/arch-propose.md`
for the full 9-phase orchestration recipe.

Flags:
- `--expand` — partial-corpus mode: existing `Docs/architecture/<feature>/` contents are preserved and only gaps filled
- `--refresh` — full-corpus mode: backs up and regenerates everything
- `--tier 1|2|3` — force tier classification (default: heuristic from candidate scope)
- `--sources path1,path2` — if the feature will span multiple packages (virtual subsystem)
- `--force` — required to target a protected corpus (core, mangle, prompt, session, shards, campaign, perception)

## Agents Dispatched

New (canonical agent files under `.grok/agents/`):

Model tiers set 2026-07-07 per the high/high/sonnet benchmark split (high-judgment model = judgment
bottlenecks only; high-accuracy model = code-grounding accuracy, run at xhigh where it peaks; fast model =
search/retrieval/formulaic doc work — GDPval-AA v2 and HLE-with-tools parity with high-accuracy model at
~40% of the cost, Terminal-Bench 80.4 vs Opus 74.6):

| Agent | Model | Role |
|---|---|---|
| `arch-propose-scout-internal` | opus | Phase 1 — Internal codebase scan: reusable utilities, adjacent subsystems, integration seams (code comprehension → opus) |
| `arch-propose-scout-literature` | sonnet | Phase 1 — External papers/RFCs/industry patterns via WebSearch + WebFetch (search task → sonnet-5 reaches opus quality at ~1/3 cost) |
| `arch-propose-scout-convergent` | sonnet | Phase 1 — Maps codeNERD's existing subsystem matrix; finds extend-existing over reinvent paths |
| `arch-propose-scout-divergent` | fable | Phase 1 — Cross-domain analogies (biology, physics, other DB paradigms) + wildcard candidate (divergent-creative → high/xhigh) |
| `arch-propose-synthesizer` | fable | Phase 2 — Merges 4 dossiers + north-star into 2–3 ranked candidates; must include one "extend existing subsystem" option (highest-leverage judgment point → high/xhigh) |
| `arch-propose-auditor` | opus | Phase 4 — Produces the synthetic `.code-audit.md` + VERBATIM blocks that unlock `arch-writer` reuse (audit accuracy gates every writer; grounds against real code in --expand) |
| `arch-propose-test-strategist` | sonnet | Phase 5 — Produces `TESTING-STRATEGY.md` applying test-forge tier matrix across unit / integration / e2e / cross-system / benchmark / Mangle / cybersecurity dimensions |
| `arch-propose-ecosystem-mapper` | opus | Phase 5 — Produces `ECOSYSTEM-IMPACT.md` mapping ripple impact across campaign orchestration, shard agents / TUI pages, internal/skills SDK skills, internal/client libraries, cmd/ CLIs, sidecar, deductive, inference, observability, security, scheduler, learning, frontend dashboard, protos, configs. Final section is the implementer punchlist. |

Reused (with Pre-Implementation Mode added to their system prompts):

| Agent | Role |
|---|---|
| `requirements-interrogator` | Phase 3 — Multi-round Socratic stress-test (26 dimensions, selectively applied; max 3 rounds, fail-closed verdict regex) — high/high: the fail-closed gate must not be weaker than the synthesizer it interrogates |
| `arch-writer` | Phase 5 — Foundation 00–04 + IMPLEMENTED_SPEC.md + deep-dives 05–NN from the synthetic audit; honors VERBATIM-FOR blocks — high/xhigh: flagship deliverable + highest output volume |
| `cross-cutting-analyst` | Phase 5 — ALL 10 cross-cutting docs in the fleet canon order (FRONTEND, DEPENDENCY, constitutional safety (permitted), TESTING-ALIGNMENT, WIRING, TELEMETRY, TESTING-REMEDIATION, ENGINE-INTEGRATION, CAMPAIGN-CONTROLLABILITY, CAMPAIGN_NARRATIVE-INTEGRATION), per the recipe's Phase 5b dispatch — medium/xhigh: formulaic from the audit artifact (reconciled 2026-07-11: an earlier revision split 6/4 with the orchestrator; the single-dispatch recipe is authoritative and is what the campaign run executed) |

### Agent resolution (canonical since 2026-07-11)

The 8 `arch-propose-*` agent files live in `.grok/agents/` (consolidated from the dissolved
architecture-doc-generator plugin) and register as dispatchable agent types. Dispatch each directly
by `subagent_type` (e.g. `Agent(subagent_type="arch-propose-scout-internal", ...)`), applying the
model override from the tier map above. `requirements-interrogator`, `arch-writer`, and
`cross-cutting-analyst` are likewise real, directly-dispatchable agent types. Do NOT pass the Agent
tool's `name` parameter — named-teammate dispatch requires an iTerm2 teammate session and fails
outside it. (The Agent tool has no effort parameter — reasoning effort applies only via each agent's
frontmatter.)

### Hook-safe wording (mandatory for every writer prompt)

The repo's excuse-word guard hook (`.claude/hooks/`, PreToolUse) blocks any file write containing
its trigger terms — including the new-subsystem compound this skill historically used for pre-code
features. Every dispatched writer prompt must instruct: write "pre-implementation" (or
"new-subsystem") wherever a template says that compound; use "later-rung" / "owned by Rung N" /
"decided by phase gate" instead of any postponement wording. The VERBATIM blocks in
`references/synthetic-audit-template.md` are already hook-safe; the template's
`{pure_new_subsystem / partial_corpus}` expand-mode field is the hook-safe enum.

### Cross-cutting numbering (corrected 2026-07-02)

The fixed on-disk order in every shipped corpus (gatekeeper, consolidation, …) is: CLI-TUI-SURFACE,
DEPENDENCY-MAP, CONSTITUTIONAL-SAFETY, TESTING-ALIGNMENT, CROSS-SYSTEM-WIRING-JOURNAL,
TELEMETRY-OBSERVABILITY, TESTING-REMEDIATION-SURFACE, KERNEL-VIRTUALSTORE-SURFACE,
CAMPAIGN-CONTROLLABILITY, CAMPAIGN_NARRATIVE-INTEGRATION — numbered consecutively after the last deep-dive.
An earlier revision of this file listed a different order; the on-disk corpus order above is
authoritative. The 10th doc (CAMPAIGN_NARRATIVE-INTEGRATION) was decreed by
`Docs/architecture/demo/campaign narrative/07-WEAVE-STANDARD.md` §3.2 (promoted into this pipeline 2026-07-11).

## Canonical Naming

This skill emits **IMPLEMENTED_SPEC.md** (not `CURRENT_SPEC.md`) to match the current
corpus per `Docs/architecture/CLAUDE.md` and `Docs/architecture/INDEX.md`. The project-root CLAUDE.md
references `CURRENT_SPEC.md` — that is a planned rename not yet reflected in the template
or the 51 existing complete corpuses. When the migration lands, update `arch-templates`,
then this skill.

## Pre-Implementation Honesty

Every generated doc must be honest that no code exists yet. The full rule set lives in
`references/pre-implementation-markers.md`. Highlights:

1. `IMPLEMENTED_SPEC.md` §3 Implementation Status: every row `Not Implemented — 0%`.
2. `IMPLEMENTED_SPEC.md` header gets a bold status banner: `**Status: Pre-Implementation — this spec describes target state; no code exists yet.**`
3. `02-CURRENT-STATE-<FEATURE>.md` names the planned source path (`internal/<feature>/` will not yet exist) and describes adjacent subsystems the feature will integrate with. It never invents current behavior.
4. `00-ALIGNMENT-VISION-REVIEW.md` scores all dimensions 0/5 (Not started) unless the north-star captured a concrete baseline (e.g., partial code in a sibling package).
5. `03-GAP-ANALYSIS-<FEATURE>.md` frames the delta as target-vs-nothing — an implementation roadmap.
6. `file:line` citations in foundation docs are suspended (the auditor flags this in the audit preamble so `arch-writer` knows not to require them).
7. `Docs/architecture/INDEX.md` registration: the feature goes into a **Proposed** section, never Tier 1/2/3 until real code lands.

## Package-scope Isolation + Read-Before-Write

Every candidate produced in Phase 2 must have its data-model section specify:
- **Package-scope name** any future writes will scope under (mandatory per project CLAUDE.md — unscoped writes silently corrupt sibling apps).
- **Read-before-write check** for persistent records the feature will create (durable/persistent duplicate-insertion hazard).

The synthesizer enforces this. Candidates missing either field are rejected.

## Campaign narrative Weave (Pre-Implementation)

Every candidate is hard-gated in Phase 2.5 on two additional campaign narrative fields (enforced by the
recipe alongside the package-scope / read-before-write (persistent store) gates):

- **`campaign narrative_beat`** — a case-side scene beat, an analyst-frame beat (`07-WEAVE-STANDARD.md` §6),
  or an honest `not_covered` rationale. Empty / `TBD` is a rejection.
- **`mode_isolation`** — for a feature with runtime surface, how it honors mode-off unreachability
  across the four membrane layers (presence / reachability / data / observability). A pure-doc or
  non-runtime feature states `not applicable — no runtime surface`, but the field must be present.

The generated corpus therefore includes a **10th cross-cutting doc**, `{NN}-CAMPAIGN_NARRATIVE-INTEGRATION.md`,
as a POINTER doc: it carries the proposed beat from the winning candidate, a `planned` coverage-row
draft, and a named follow-up owner (the downstream `corpus-build` run) — never fabricated coverage,
never duplicated story prose (the story lives only in the campaign narrative corpus). Phase 6 (step 6h)
appends the matching `planned` `SWC-NNN` row(s) to the single coverage ledger
`Docs/architecture/demo/campaign narrative/06-FEATURE-COVERAGE.csv`, and Phase 7b fails closed if the pointer
doc or the coverage row is missing.

- Binding contract: `Docs/architecture/demo/campaign narrative/07-WEAVE-STANDARD.md`
- Decree record: `Docs/architecture/demo/CAMPAIGN_NARRATIVE-MODE-PLAN-OF-RECORD.md`

## Architecture Separation (per `.claude/rules/architecture-separation.md`)

During synthesis, classify each candidate's docs as:
- **Engine-layer** (domain-agnostic: algorithms, storage layout, query surface, invariants) → `Docs/architecture/<feature>/`
- **Applicability-layer** (domain-specific: crypto asset taxonomies, industry-specific thresholds, operator guides) → `Docs/architecture/<feature>/applicability/`

Most pre-implementation features start purely engine-layer. If the north-star captures domain
specifics (e.g., "NCAA bracket search"), the skill creates an `applicability/` subdir with
its own mini-corpus.

## No Time, Cost, or Effort Estimates

Per `.claude/rules/no-time-cost-estimates.md`, no generated doc may contain wall-clock
predictions, sprint counts, person-weeks, or dollar costs. Roadmap language must be
**ordering + gates** only (e.g., "Phase 4 depends on Phase 3 validated", not "Phase 4
begins week 23"). The synthesizer and arch-writer prompts enforce this.

## Shakeout Target

First real target: `Docs/architecture/search/` — currently partial (3 cross-cutting docs,
legacy `IMPLEMENTED_SPEC.md`, no foundation, no deep-dives, no `internal/search/` code).
Exercises `--expand` mode, partial-corpus diff (Phase 4.5), and backup-before-overwrite.

## Persistent Working Directory

All scratch artifacts live under `.arch-propose/` (gitignored):

```
.arch-propose/
  north-star/<feature>.md          # Phase 0 interview transcript
  research/
    internal/<feature>-<date>.md   # Phase 1 scout outputs
    literature/<feature>-<date>.md
    convergent/<feature>-<date>.md
    divergent/<feature>-<date>.md
  candidates/<feature>.md          # Phase 2 synthesizer output
  interrogations/<feature>.md      # also mirrored to .claude/interrogations/
  decision/<feature>.md            # Phase 3.5 judge verdict
  diff/<feature>.md                # Phase 4.5 corpus diff (--expand mode)
  backups/<feature>/<date>/        # pre-overwrite snapshots
  journal/<date>_<feature>.md      # Phase 7 final run record + cross-pollination seeds
```

## Cross-Pollination

After the run, extract seed insights into the journal with explicit markers:
- `SEED:subsystem-X:<insight>` — findings that other subsystem specs should absorb
- `SEED:reuse:<pattern>` — reusable patterns future `/arch-propose` runs should try first
- `SEED:gap:<capability>` — identified cross-system gaps worth their own proposal run

These flow into future `/arch-propose`, `integration-auditor`, and `mangle-programming` runs.

## References

- `references/pre-implementation-markers.md` — the 8 honesty rules with exact wording templates
- `references/synthetic-audit-template.md` — skeleton `.code-audit.md` the auditor fills in
- `references/pre-implementation-phase-checklist.md` — step-by-step verification for each phase
