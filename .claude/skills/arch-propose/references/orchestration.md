# Arch-Propose Orchestration

## Artifact layout

```text
.arch-propose/
  north-star/<feature>.md
  research/internal/<feature>.md
  research/literature/<feature>.md
  research/convergent/<feature>.md
  research/divergent/<feature>.md
  candidates/<feature>.md
  interrogations/<feature>.md
  decision/<feature>.md
  audits/<feature>.md
  diff/<feature>.md
  backups/<feature>/<date>/
  journal/<date>_<feature>.md
```

## Minimum corpus manifest

Generate the smallest complete set for the chosen design. The normal Tier 3
baseline is:

1. `00-NORTH-STAR.md`
2. `01-CURRENT-STATE-EVIDENCE.md`
3. `02-TARGET-ARCHITECTURE.md`
4. `03-GAP-ANALYSIS.md`
5. `04-INVARIANTS-AND-CONTRACTS.md`
6. `05-DATA-AND-PREDICATE-MODEL.md`
7. `06-FACT-FLOW-AND-LIFECYCLE.md`
8. `07-MANGLE-POLICY-AND-SAFETY.md`
9. `08-JIT-PROMPT-AND-AGENT-BEHAVIOR.md`
10. `09-PERSISTENCE-MEMORY-AND-RECOVERY.md`
11. `10-CROSS-SYSTEM-WIRING.md`
12. `11-OBSERVABILITY-AND-OPERATIONS.md`
13. `12-TESTING-AND-VALIDATION.md`
14. `13-MIGRATION-AND-COMPATIBILITY.md`
15. `IMPLEMENTED_SPEC.md`
16. `TESTING-STRATEGY.md`
17. `ECOSYSTEM-IMPACT.md`
18. `README.md`
19. `TODO.md`
20. `OPEN-QUESTIONS.md`
21. `_progress.md`

Add focused deep dives and ADRs only when the chosen design needs them.

## Parallel wave contract

### Research wave

Spawn the four scouts concurrently. Each writes only its assigned dossier.
Wait for all four before synthesis.

### Writer wave

After the candidate, interrogation, decision, and audit gates pass:

- arch_writer owns files 00 through 09 plus IMPLEMENTED_SPEC
- cross_cutting_analyst owns files 10 through 13
- arch_propose_test_strategist owns TESTING-STRATEGY
- arch_propose_ecosystem_mapper owns ECOSYSTEM-IMPACT
- the root agent owns governance files and final cross-document consistency

Do not let multiple agents edit the same file.

## Expand mode

1. Inventory current files and references.
2. Back up only files that will be replaced.
3. Produce a keep/revise/replace/add map.
4. Preserve useful content and provenance.
5. Renumber only when necessary.
6. Rewrite internal links after renumbering.
7. Verify every manifest entry and every local link.

## Protected surfaces

Require `--force` and a stronger interrogation gate for changes centered on:

- `internal/core/`
- `internal/mangle/`
- `internal/prompt/`
- `internal/session/`
- `internal/shards/`
- `internal/perception/`
- `internal/campaign/`

## Final compliance commands

Use PowerShell-safe commands on Windows:

```powershell
rg -n "Not Implemented|Pre-Implementation" Docs/architecture/<feature>
rg -n -i "fully implemented|production ready|all tests pass" Docs/architecture/<feature>
rg -n -i "weeks?|sprints?|person-days?|story points" Docs/architecture/<feature>
rg -n "TODO|TBD|OPEN QUESTION" Docs/architecture/<feature>
```

Interpret matches in context; do not use grep counts as semantic proof.

