# codeNERD corpus-build design of record

## Objective

Consume one architecture corpus, audit it against the current Go/Mangle runtime, choose BUILD/EVOLVE/PIVOT, and close the accepted gap through a dependency-ordered Codex fleet. The architecture north star is invariant: the LLM creates and synthesizes; Mangle executes planning, policy, safety, and state decisions.

## Inputs

- `Docs/architecture/<feature>/`
- current source, tests, runtime registrations, and repository status
- root and scoped `AGENTS.md`
- optional user priority constraints

## Phases

1. **Anchor and preflight** — record branch/worktree state, concurrent edits, exact corpus, and live commands.
2. **Read** — `corpus-reader` creates a requirement/evidence matrix without mutation.
3. **Judge** — `corpus-judge` classifies each requirement as built, partial, missing, obsolete, contradictory, or unverifiable and chooses BUILD/EVOLVE/PIVOT.
4. **Contract** — pin interfaces, Mangle declarations, prompt atom IDs, storage schemas, contested files, and acceptance oracles.
5. **Packetize** — emit work units defined by `01-work-unit-types.md` and validate the DAG.
6. **Build** — dispatch non-overlapping worker lanes; no recursive delegation.
7. **Wire** — apply registration intents serially and prove the perception → kernel → VirtualStore → articulation route.
8. **Assure** — critic, defense, consumables, targeted tests, then `go test ./...` when feasible.
9. **Reconcile** — doc-auditor updates the corpus from evidence and preserves unresolved residuals.

## Artifacts

```text
.corpus-build/
  runs/<run-id>/manifest.json
  runs/<run-id>/requirements.json
  runs/<run-id>/plan.json
  runs/<run-id>/results.json
  intents/*.json
  ledger/fleet_events.jsonl
  skips.jsonl
```

Usage fields are recorded only when Codex supplies them in a hook payload. Duration and command results are measured; price and token estimates are prohibited.

## Human checkpoints

- BUILD/EVOLVE/PIVOT judgment when it changes the requested architecture.
- Scope expansion into destructive migration, external publishing, or unrelated product repair.
- Final acceptance when residuals materially change the spec promise.

## Failure handoff

Workers get a bounded local repair cycle. Exhausted failures become deterministic packets under `.quality_assurance/remediation/` compatible with `Docs/jules-patch-remediation-prompt.md`. There is no assumed internal remediation runtime or automatic external dispatch.

## Completion

The run closes only when accepted requirements map to verified artifacts or named residuals, applicable surfaces have evidence, constitutional and JIT invariants hold, repo-level failures are classified, and corpus status matches current reality.
