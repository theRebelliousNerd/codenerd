# Pre-Implementation Phase Checklist (codeNERD)

Any unchecked box halts the run with a journal failure entry.

## Phase 0 — North-Star Interview

- [ ] `.arch-propose/north-star/<feature>.md` exists and non-empty
- [ ] Captures: problem, placement (new vs extend), success criteria, out-of-scope, tier estimate
- [ ] Names primary package + whether Mangle/VirtualStore/shards/prompt atoms are in scope
- [ ] Expand mode: preserves vs supersedes called out

## Phase 1 — Parallel Research

All four dossiers >500 words:

- [ ] `.arch-propose/research/internal/<feature>-<date>.md` (≥3 real file:line)
- [ ] `.arch-propose/research/literature/<feature>-<date>.md` (≥3 named sources)
- [ ] `.arch-propose/research/convergent/<feature>-<date>.md` (≥1 extend-existing option)
- [ ] `.arch-propose/research/divergent/<feature>-<date>.md` (≥1 cross-domain analogy with structural map)

## Phase 2 — Synthesis

- [ ] `.arch-propose/candidates/<feature>.md` with 2–3 candidates
- [ ] At least one absorption / extend-existing candidate when viable
- [ ] Every candidate has codeNERD hard-gate fields (package, Mangle, VS, shards, prompts, safety, fact-flow)
- [ ] No time/cost estimates

## Phase 3 — Interrogation

- [ ] `.arch-propose/interrogations/<feature>.md` with ≥2 rounds
- [ ] Fail-closed READY / NEEDS_WORK verdict line present
- [ ] READY only if hard gates satisfied

## Phase 3.5 — Judge

- [ ] `.arch-propose/decision/<feature>.md` selects winning candidate with rationale

## Phase 4 — Synthetic Audit

- [ ] `.arch-propose/audit/<feature>.code-audit.md` follows template
- [ ] Synthetic banner present; adjacent citations real

## Phase 5 — Corpus Generation

- [ ] Foundation 00–04 + IMPLEMENTED_SPEC written under `Docs/architecture/<feature>/`
- [ ] TESTING-STRATEGY.md + ECOSYSTEM-IMPACT.md present (tier ≥2)
- [ ] Pre-implementation banners present
- [ ] No invented current-state behavior

## Phase 6 — Governance

- [ ] README, TODO, OPEN-QUESTIONS, _progress
- [ ] Docs/architecture/INDEX.md Proposed entry
- [ ] Journal + SEED: lines written
