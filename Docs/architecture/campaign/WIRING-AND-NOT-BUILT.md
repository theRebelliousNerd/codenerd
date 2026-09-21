# campaign wiring — what runs, what is dormant, what was never built

**Question answered here:** what can I rely on, what exists but is unproven,
and what the design assumes that the code does not do?
(What the package is → [README.md](README.md). How it works → [INTERNALS.md](INTERNALS.md).)

**Verified 2026-09-20 against the working tree at `refs/heads/main`.**

## Wired and reachable

- **Task execution requires `TaskExecutor`.** The config field
  (`internal/campaign/orchestrator_types.go:134`) is mandatory; `ShardManager`
  (`:135`) is monitoring only. With no executor, tasks cannot run and both
  reviewer checkpoints fail closed rather than pass
  (`internal/campaign/checkpoint.go:261`, `:349`). The events channel that
  surfaces all of this is on the config at
  (`internal/campaign/orchestrator_types.go:101`).
- **Risk preflight blocks protected-surface campaigns without reviewers.**
  If targets touch the protected roots and no advisory board (or no northstar
  observer) is configured, preflight returns blocked before any gate runs
  (`internal/campaign/risk_scoring.go:248`, `:267`).
- **Checkpoints that cannot verify report failure.** Shard validation and the
  nemesis gauntlet fail closed without a task executor; manual review escalates
  to shard validation instead of consulting a human
  (`internal/campaign/checkpoint.go:246`, `:260`, `:348`). A fabricated audit
  that "passed" five unverified phases is what these comments cite as the
  incident that motivated the design.
- **Degraded plans are labelled.** An empty LLM plan becomes a scaffold with
  `PlanDegraded` set (`internal/campaign/types.go:186`), persisted across
  resume so it cannot be mistaken for a real plan.
- **Journal verify/replay is operator-grade.** List, verify, replay, and render
  all exist (`internal/campaign/journal_ops.go:96`, `:208`, `:293`, `:353`,
  `:390`) and back the `nerd campaign journal` CLI surface.

## Exists but unverified in this pass (present, not proven here)

- `Task.TestWitness` (`internal/campaign/types.go:286`) — the struct field
  exists; the site that populates it was not traced in this pass. Grep for
  writers before relying on it.
- `Task.DeterministicWriteSet` (`internal/campaign/types.go:730`) and
  `internal/campaign/write_set_lock_manager.go` — the allow-list field
  (`internal/campaign/types.go:308`) and helper exist; end-to-end enforcement
  through the task handlers was not traced here.
- `RecurseLoop` / `RecurseRunner` (`internal/campaign/recurse_runner.go:46`,
  `:117`) with `Campaign.RecurseID` / `RecurseWave`
  (`internal/campaign/types.go:175`) — the wave-sweep machinery exists; its
  driver wiring was not traced in this pass.
- `Phase.CheckpointFailures` (`internal/campaign/types.go:260`) and
  `Task.ReplannedAtCap` (`:335`) — the bounding counters are declared on the
  model with their intent documented; the loop sites that increment them were
  not re-verified here.

## Not built (assumed by the design, absent from the code)

- **No interactive human review exists.** `/manual_review` never consults a
  person; non-interactive runs escalate to a reviewer shard
  (`internal/campaign/checkpoint.go:246`). Any process that assumes "a human
  approved this phase" is assuming something the code cannot do.
- **Unknown verification methods pass.** The `default` arm of the checkpoint
  dispatcher returns pass-with-`skipping` (`internal/campaign/checkpoint.go:93`).
  A misspelled method degrades to no verification, silently.
- **Risk gating can be entirely off.** With auto-wiring disabled, auto mode, and
  no campaign override, preflight returns `Allowed: true` with no gates run
  (`internal/campaign/risk_scoring.go:286`). Below-threshold scores likewise run
  no strict gates (`:344`).
- **The completion sweep covers only the workspace root.** It moves only files
  absent from `RootBaseline` (`internal/campaign/types.go:197`), which records
  root filenames — scratch nested in subdirectories is out of scope
  (`internal/campaign/orchestrator_execution.go:198`).

## What the old documents got wrong

The previous corpus was not re-read for claims (its errors are what a rewrite
launders forward), but structural defects verified against the tree:

- **19 markdown files for one package.** The defect this rewrite fixes: the old
  directory (`00-`–`12-` numbered files plus `IMPLEMENTED_SPEC.md`,
  `OPEN-QUESTIONS.md`, `TODO.md`, `_progress.md`) has been replaced by three
  files with one question each. Two inbound links to the removed
  `IMPLEMENTED_SPEC.md` were repointed: `internal/campaign/README.md` and
  `Docs/architecture/cli/INTERNALS.md`.
- **Stale scale in the in-package map.** `internal/campaign/README.md` claims
  "49 non-test sources (~22.2k lines), 59 test files" and per-file sizes
  (`replan.go` "1201", `risk_scoring.go` "1169", `decomposer.go` "1079").
  The tree holds 32 non-test `.go` files and 17 test files; the three named
  files are 1342, 1181, and 1124 lines. The module map itself is accurate and
  was reused for the README.md map above.
- **A plan shown at "50% confidence" may be a scaffold.** Any document that
  presents a degraded plan as a real one repeats the failure `PlanDegraded`
  (`internal/campaign/types.go:186`) exists to prevent.
