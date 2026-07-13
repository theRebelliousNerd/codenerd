---
name: arch-propose
description: >
  Research-driven architecture corpus generator for pre-implementation codeNERD
  features. Takes a feature idea, runs 4 parallel scouts (internal, literature,
  convergent, divergent), synthesizes architectural candidates, stress-tests via
  Socratic interrogation, and writes a pre-implementation architecture corpus under
  Docs/architecture/<feature>/. Use for design/investigate new subsystem/feature,
  or --expand an existing pre-impl corpus. Do NOT use for code-first docs on existing
  packages (use spec-doc-sprint), Mangle-only rule work (use mangle-programming),
  or spec-to-code implementation (use corpus-build).
metadata:
  version: 1.0.0
  author: codeNERD (ported from Vectryx arch-propose)
  last-verified: 2026-07-13
---

# Arch-Propose — Pre-Implementation Architecture Corpus (codeNERD)

Research-first pipeline for features that do not have code yet. Sibling of
`spec-doc-sprint` (which fills `Docs/Spec/` from existing source). This skill starts
from an idea, researches, and emits a **target-state** corpus with honest
pre-implementation markers.

**Code-grounded mode (living packages):** when `internal/<pkg>/` or `cmd/...` already
exists, do **not** zero IMPLEMENTED_SPEC to 0% or claim “no code exists”. Inventory real
files with path citations, mark status Realized, and describe gaps honestly. Use pure
pre-impl markers only for greenfield targets.

**Pipeline**: `arch-propose` → (human decides) → `corpus-build` → `spec-doc-sprint` / live code → optional `nerd-evolve`

## Core Loop

```
NorthStarInterview -> Research(4 parallel scouts) -> Synthesize(2-3 candidates)
  -> SchemaValidate(hard gates) -> Interrogate(max 3 rounds) -> Judge
  -> SyntheticAuditor -> CorpusGeneration (writers) -> Governance docs
  -> INDEX patch -> Compliance Gate -> Journal
```

## codeNERD Hard Gates (Phase 2.5)

Every candidate MUST specify:

| Field | Why |
|-------|-----|
| **primary package** | e.g. `internal/<feature>/` or absorption into an existing package |
| **Mangle surface** | new `Decl`s, policy predicates, or "none — pure Go runtime" |
| **VirtualStore / tools** | new routes, tools, or none |
| **shard impact** | Type A/B/U/S, registration points, or none |
| **prompt atoms** | new atoms under `internal/prompt/atoms/` or none |
| **constitutional safety** | how `permitted(...)` / danger classes apply |
| **fact flow fit** | where it sits in user_intent → next_action → VirtualStore → articulation |
| **extend-existing option** | at least one candidate must absorb into an existing subsystem when viable |

Reject candidates missing required fields. Do **not** invent time/cost/sprint estimates —
ordering + gates only.

## Output Layout

Primary corpus root: `Docs/architecture/<feature>/`

| Artifact | Notes |
|----------|-------|
| `00-ALIGNMENT-VISION-REVIEW.md` | North-star alignment; scores 0/5 if greenfield |
| `01-DOMAIN-MODEL.md` | Types, facts, Mangle schema plan |
| `02-CURRENT-STATE-<FEATURE>.md` | Adjacent reality only — never invent living code |
| `03-GAP-ANALYSIS-<FEATURE>.md` | Target vs nothing roadmap |
| `04-INVARIANTS-AND-GATES.md` | Safety, stratification, build/test gates |
| `IMPLEMENTED_SPEC.md` | Target spec; all rows 0% pre-impl |
| `05-NN-*.md` | Deep-dives (tier-dependent count) |
| Cross-cutting set | Dependency, wiring, testing, telemetry, prompt/JIT, Mangle, safety |
| `TESTING-STRATEGY.md` | From test-strategist agent |
| `ECOSYSTEM-IMPACT.md` | Ripple map + implementer punchlist |
| `TODO.md`, `OPEN-QUESTIONS.md`, `README.md`, `_progress.md` | Governance |

Tier heuristic: small = foundation + IMPLEMENTED_SPEC + 2 deep-dives; medium + more deep-dives;
large = full cross-cutting set. Force with `--tier 1|2|3`.

Honesty rules: `references/pre-implementation-markers.md`.

## Entry Points

Triggers: "propose architecture for X", "arch-propose X", "design subsystem X",
"investigate feature X before coding", `--expand` on existing `Docs/architecture/<feature>/`.

Flags (orchestrator interprets):

- `--expand` — fill gaps; preserve existing files
- `--refresh` — backup then regenerate
- `--tier 1|2|3` — force depth
- `--sources path1,path2` — multi-package virtual subsystem
- `--force` — required for protected cores (`core`, `mangle`, `prompt`, `session`, `shards`)

## Agents Dispatched

| Agent | Role |
|-------|------|
| `arch-propose-scout-internal` | Reuse map across `internal/`, `cmd/`, `Docs/Spec/` |
| `arch-propose-scout-literature` | External SOTA / papers / agent systems |
| `arch-propose-scout-convergent` | Extend-existing matrix against codeNERD working map |
| `arch-propose-scout-divergent` | Cross-domain analogies + wildcard candidate |
| `arch-propose-synthesizer` | 2–3 ranked candidates incl. absorption option |
| `arch-propose-auditor` | Synthetic `.code-audit.md` for writers |
| `arch-propose-test-strategist` | `TESTING-STRATEGY.md` |
| `arch-propose-ecosystem-mapper` | `ECOSYSTEM-IMPACT.md` |
| `plan` / `wiring-auditor` | Optional reuse for interrogation support |

Interrogation may use main-agent Socratic rounds or a `plan` subagent if no dedicated
requirements-interrogator is loaded. Fail-closed: no READY verdict → do not generate corpus.

Agent definitions: `.grok/agents/arch-propose-*.md`.

## Scratch Workspace (gitignored)

```
.arch-propose/
  north-star/<feature>.md
  research/{internal,literature,convergent,divergent}/<feature>-<date>.md
  candidates/<feature>.md
  interrogations/<feature>.md
  decision/<feature>.md
  audit/<feature>.code-audit.md
  backups/<feature>/<date>/
  journal/<date>_<feature>.md
```

## Cross-Pollination Seeds

Journal must end with extractable seeds:

- `SEED:subsystem-X:<insight>`
- `SEED:reuse:<pattern>`
- `SEED:gap:<capability>`
- `SEED:mangle:<predicate-or-policy-insight>`
- `SEED:prompt:<atom-or-jit-insight>`

## Relationship to Other Skills

| Skill | When instead |
|-------|----------------|
| `spec-doc-sprint` | Code already exists; fill `Docs/Spec/` |
| `corpus-build` | Corpus exists; implement code from it |
| `codenerd-builder` | Implement core architecture without full propose→build fleet |
| `integration-auditor` | Audit live wiring without building |
| `mangle-programming` | Pure Mangle rule work |
| `prompt-architect` | Pure prompt-atom work |
| `nerd-evolve` | Evolve existing implementation |

## References

- `references/pre-implementation-markers.md`
- `references/synthetic-audit-template.md`
- `references/pre-implementation-phase-checklist.md`
- `references/codenerd-corpus-shape.md`
