---
phase: "05_report"
next: "DONE"
---

# Phase 05: Report and Learn

## Objective

Return a compact, auditable migration report and record any repeatable learning.

## Report Contract

Name only what matters:

- source surfaces reviewed
- target surfaces changed
- classifications used
- skills updated
- agents updated
- hooks, commands, rules, prompts, plugins, or config touched
- memory roots repointed to `.claude/agent-memory/<name>/`
- validation commands run
- unsupported gaps and preserved evidence
- intentional exceptions

## Learning Contract

Update `references/journal.md` when a run exposes a repeatable failure mode,
surface mismatch, stale docs issue, or validation gap. Include:

- date
- observed failure mode
- correction added
- evidence path or command

## Gate

Do not say the migration is complete unless the report ties every changed file
back to the ledger and names the validation that ran.
