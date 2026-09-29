---
doc-class: governance
subsystem: workspace
implementation-status: target-state
last-verified: 2026-09-29
verified-against: e056692c
supersedes: []
---

# RISK-REGISTER-AND-DECISION-LOG — Workspace Membership

Risks of moving every workspace walker onto one git-backed membership authority
(`internal/workspace`, planned), and the decisions taken so far. Nothing here has shipped;
see `IMPLEMENTED_SPEC.md`.

## Risks

| ID | Risk | Likelihood | Consequence | What retires it |
|---|---|---|---|---|
| RISK-WS-01 | A file created after the `git ls-files` snapshot is invisible to walkers until the next refresh | medium | the agent cannot find a file it just wrote | a path outside the snapshot is decided by batched `git check-ignore -z --stdin` and cached; witness test `TestMembership_DynamicCheckIgnore` |
| RISK-WS-02 | `git` is absent or the workspace is not a git work tree | medium | membership silently degrades | explicit fallback to `world.ignore_patterns` semantics, reported in scan output; witness: a non-git temp-dir test |
| RISK-WS-03 | A tracked tree is huge but unwanted (seed data, generated fixtures) | high on foreign repos | scans and embeddings dominated by data, not code | `world.ignore_patterns` applies on top of git truth as the user's extra excludes, and the orientation discernment overlay (see `Docs/architecture/orient/08-DISCERNMENT-AND-QUESTIONS.md`) assigns a treatment per tree |
| RISK-WS-04 | Repointing ~35 walkers leaves one on its private list | high | two truths about what the workspace is | `WIRING-AND-NOT-BUILT.md` is the repoint checklist; exit: a repository-wide search finds no private ignore-name set in a production walker |
| RISK-WS-05 | Submodules and nested repositories | low | members counted twice or not at all | membership follows git's own output for the parent repository; behaviour documented in `05-MEMBERSHIP-SPEC.md` |
| RISK-WS-06 | Cost of `git ls-files` on every scan tick for a large repository | low | slower ticks | the snapshot is cached per canonical root and refreshed when `.git/index` or `HEAD` changes or on an explicit `Refresh` |

## Decision log

| Date | Decision | Where |
|---|---|---|
| 2026-09-29 | git's own view (`git ls-files -z -co --exclude-standard` + batched `check-ignore`) is the membership floor; no gitignore parser is vendored | `adr/ADR-001-git-ls-files-and-check-ignore-as-membership-authority.md` |
| 2026-09-29 | `world.ignore_patterns` becomes extra exclusions on top of git truth, not a replacement | `05-MEMBERSHIP-SPEC.md` |
| 2026-09-29 | Only `.git` and `.nerd` are always excluded; no hidden-directory special cases | `05-MEMBERSHIP-SPEC.md` |
