---
name: arch-propose
description: >
  Research-driven architecture corpus generator for pre-implementation codeNERD
  features. Runs parallel internal, literature, convergent, and divergent scouts;
  synthesizes and interrogates competing designs; selects a candidate; and writes
  an evidence-grounded feature corpus under Docs/architecture/.
  Use for new subsystem design, architecture exploration, or expanding an
  incomplete pre-implementation corpus. Do not use for code-first documentation
  of an existing implementation (use spec-doc-sprint) or spec-to-code work (use
  corpus-build).
metadata:
  version: 3.0.0
  author: codeNERD
  last-verified: 2026-07-13
---

# Arch Propose

Design codeNERD features before implementation without inventing current-state
evidence.

The LLM fleet proposes and synthesizes. Repository evidence, architecture
invariants, Mangle safety, and explicit decision gates constrain the result.

## Orchestration contract (Grok / multi-harness)

**Source of this skill text:** one-way import from sibling Codex CLI under
`.codex/skills/arch-propose/` (read-only — Grok never edits `.codex/`).
This copy lives under `.agents/skills/` and `.grok/skills/`.

This skill explicitly requests subagents for non-trivial proposals.

1. Keep the root agent responsible for the north star, phase gates, final
   decision, numbering, and cross-document consistency.
2. Spawn four independent scouts in parallel.
3. Wait for all four dossiers before synthesis.
4. Keep subagent depth = 1; scouts never spawn their own agents.
5. Run writers in parallel only after the candidate and audit gates pass.
6. Treat every generated document as a durable repository artifact, not chat
   output.

### Grok agent names (spawn these; definitions in `.grok/agents/`)

| Role | Grok agent file |
|---|---|
| Internal scout | `arch-propose-scout-internal` |
| Literature scout | `arch-propose-scout-literature` |
| Convergent scout | `arch-propose-scout-convergent` |
| Divergent scout | `arch-propose-scout-divergent` |
| Synthesizer | `arch-propose-synthesizer` |
| Requirements interrogator | `requirements-interrogator` |
| Auditor | `arch-propose-auditor` |
| Foundation writer | `arch-writer` |
| Cross-cutting | `cross-cutting-analyst` |
| Test strategy | `arch-propose-test-strategist` |
| Ecosystem map | `arch-propose-ecosystem-mapper` |

Codex CLI uses underscore registry keys in `.codex/config.toml` (sibling only).
Do not edit that tree from Grok.

## Modes

- `arch-propose <feature>`: create a new pre-implementation corpus.
- `arch-propose <feature> --expand`: preserve existing files and fill verified
  gaps.
- `arch-propose <feature> --refresh`: regenerate only after recording a backup
  and a file-by-file replacement map.
- `--tier 1|2|3`: force scope tier.
- `--sources <paths>`: provide likely source or integration surfaces.
- `--force`: required for protected core surfaces such as core, mangle,
  prompt, session, shards, perception, and campaign.

## Required codeNERD invariants

Every candidate must address:

- LLM creative center; Mangle executive control
- `permitted(...)` default-deny authorization
- Decl-before-use, safe negation, stratification, and typed predicate contracts
- JIT prompt atoms for new LLM-facing behavior
- user input -> perception -> `user_intent` -> `next_action` -> execution ->
  articulation fact flow
- wiring through the actual lifecycle and registry surfaces
- bounded context and memory behavior
- cancellation, recovery, telemetry, and testability
- extend-existing analysis before introducing a parallel subsystem

## Pipeline

### Phase 0: North star

Capture a short north-star record under
`.arch-propose/north-star/<feature>.md`:

- problem and users
- desired outcome
- non-goals
- hard constraints
- likely packages and integrations
- success evidence
- risk tolerance
- whether any partial code or corpus already exists

Do not stall on low-risk ambiguity. Record a reasonable assumption and continue.

### Phase 1: Parallel research

Dispatch four scouts:

| Agent | Focus |
|---|---|
| `arch-propose-scout-internal` | Live code, reusable mechanisms, dormant wiring, adjacent corpora |
| `arch-propose-scout-literature` | Primary papers, standards, official docs, comparable systems |
| `arch-propose-scout-convergent` | Smallest design that extends existing codeNERD architecture |
| `arch-propose-scout-divergent` | Cross-domain alternatives and one bounded wildcard |

Require file/symbol citations for repository claims and links for external
claims. Store dossiers under `.arch-propose/research/`.

### Phase 2: Synthesize candidates

Dispatch `arch-propose-synthesizer` after all dossiers exist.

Produce two or three candidates, including at least one extend-existing option.
For each candidate specify:

- component and package boundaries
- predicate/data contracts
- fact-flow integration
- JIT prompt impact
- permission and safety model
- lifecycle wiring
- state ownership and persistence
- failure/recovery semantics
- observability
- migration and compatibility
- test strategy
- rejected alternatives
- explicit unknowns

Reject a candidate that lacks concrete integration surfaces or falsifiable
acceptance gates.

### Phase 3: Interrogate and decide

Dispatch `requirements-interrogator` for up to three bounded rounds. It must
probe contradictions, not merely add questions.

The root agent then selects a winner using:

- architectural fit
- reuse leverage
- deterministic safety
- implementation complexity
- failure containment
- testability
- operational visibility
- reversibility

Write `.arch-propose/decision/<feature>.md` with the selected candidate,
rejected options, evidence, and remaining open questions.

### Phase 4: Synthetic audit

Dispatch `arch-propose-auditor`.

For greenfield work, the audit explicitly separates:

- verified adjacent current state
- planned feature state
- assumptions
- absent code
- evidence-backed reuse opportunities

For `--expand`, compare every existing corpus claim with live source and mark
keep, revise, replace, or unresolved.

Gate: the audit follows
`references/synthetic-audit-template.md` and contains no invented file:line
citation.

### Phase 5: Generate the corpus

After the decision and audit pass, dispatch writers with disjoint ownership:

- `arch-writer`: foundation, target architecture, data/predicate model, runtime
  flow, failure model, and IMPLEMENTED_SPEC
- `cross-cutting-analyst`: safety, wiring, JIT prompt, configuration,
  observability, and operational cross-cuts
- `arch-propose-test-strategist`: TESTING-STRATEGY.md
- `arch-propose-ecosystem-mapper`: ECOSYSTEM-IMPACT.md

The minimum corpus manifest is defined in
`references/orchestration.md`. Tier 1 and Tier 2 proposals may add deeper
design docs and ADRs; never pad a corpus with empty documents to hit a count.

### Phase 6: Governance and compliance

Create or update:

- `README.md`
- `TODO.md`
- `OPEN-QUESTIONS.md`
- `_progress.md`
- `IMPLEMENTED_SPEC.md`
- `TESTING-STRATEGY.md`
- `ECOSYSTEM-IMPACT.md`
- `Docs/architecture/INDEX.md`

Apply `references/pre-implementation-markers.md` and run the checklist in
`references/pre-implementation-phase-checklist.md`.

A pre-implementation corpus must never claim shipped code, passing tests,
runtime metrics, or wired behavior that has not been observed.

### Phase 7: Journal and handoff

Write `.arch-propose/journal/<date>_<feature>.md` containing:

- proposal decision
- decisive evidence
- unresolved questions
- generated file manifest
- compliance results
- reusable seeds
- recommended `corpus-build` entry conditions

Keep the user report short and point to exact artifact paths.

## Model and sandbox guidance

On Grok: use high-effort agents for synthesis/interrogation/writing; read-only
capability for scouts and auditors unless they own a dossier path. Writers own
explicit output files under `Docs/architecture/` and `.arch-propose/`.

Sibling Codex CLI keeps its own model catalog and agent TOMLs under `.codex/`
— leave that tree alone.

## References

- `references/orchestration.md`
- `references/pre-implementation-markers.md`
- `references/pre-implementation-phase-checklist.md`
- `references/synthetic-audit-template.md`
