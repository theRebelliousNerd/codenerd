# codeNERD Architecture Corpus Shape

Target root: `Docs/architecture/<feature>/`

This is **pre-implementation / design corpus**, distinct from product specs in
`Docs/Spec/internal/<pkg>/` (filled by `spec-doc-sprint` once code exists).

## Numbered Foundation (required)

1. `00-ALIGNMENT-VISION-REVIEW.md` — alignment to codeNERD north star (LLM creative / Mangle executive)
2. `01-DOMAIN-MODEL.md` — Go types + Mangle Decl plan + fact flow
3. `02-CURRENT-STATE-<FEATURE>.md` — adjacent existing code only
4. `03-GAP-ANALYSIS-<FEATURE>.md` — target vs nothing
5. `04-INVARIANTS-AND-GATES.md` — safety, stratification, race, CGO/build gates
6. `IMPLEMENTED_SPEC.md` — living target spec (0% until code ships)

## Deep-Dives (tier-scaled)

Examples: `05-MANGLE-POLICY.md`, `06-VIRTUALSTORE-ROUTES.md`, `07-SHARD-LIFECYCLE.md`,
`08-PROMPT-ATOMS-JIT.md`, `09-PERCEPTION-ARTICULATION.md`, `10-CAMPAIGN-ORCHESTRATION.md`.

## Cross-Cutting (large tier / always for kernel-touching features)

Suggested fixed order after last deep-dive:

1. `NN-DEPENDENCY-MAP.md`
2. `NN-CROSS-SYSTEM-WIRING.md`
3. `NN-CONSTITUTIONAL-SAFETY.md`
4. `NN-MANGLE-SURFACE.md`
5. `NN-PROMPT-JIT-SURFACE.md`
6. `NN-TELEMETRY-LOGGING.md`
7. `NN-TESTING-ALIGNMENT.md`
8. `NN-CONFIG-AND-CLI.md`
9. `NN-SESSION-EXECUTOR-IMPACT.md`
10. `NN-CAMPAIGN-IMPACT.md` (or N/A with reason)

## Un-numbered

`README.md`, `TODO.md`, `OPEN-QUESTIONS.md`, `_progress.md`,
`TESTING-STRATEGY.md`, `ECOSYSTEM-IMPACT.md`.

## Index

Register under `Docs/architecture/INDEX.md` in a **Proposed** section until code lands.
Do not claim Tier completeness in `Docs/Spec/INDEX.md` until `spec-doc-sprint` is run on real code.
