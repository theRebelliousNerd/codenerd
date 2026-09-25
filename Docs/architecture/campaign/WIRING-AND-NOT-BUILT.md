# campaign wiring — what runs, what is dormant, what was never built

**Question answered here:** what can I rely on, what exists but is unproven,
and what the design assumes that the code does not do?
(What the package is → [README.md](README.md). How it works → [INTERNALS.md](INTERNALS.md).)

**Re-verified 2026-09-25 (lane A build-out)**; first written 2026-09-20 against
`refs/heads/main`. Every item of the 2026-09-20 "unverified" and "not built"
lists was traced; each is kept below with what the code does today.

## Wired and reachable

- **Task execution requires `TaskExecutor`.** The config field
  (`internal/campaign/orchestrator_types.go`) is mandatory; `ShardManager` is
  monitoring only. With no executor, tasks cannot run and both reviewer
  checkpoints fail closed rather than pass (`internal/campaign/checkpoint.go`,
  `runShardValidationCheckpoint`).
- **Risk preflight blocks protected-surface campaigns without reviewers.**
  If targets touch the protected roots and no advisory board (or no northstar
  observer) is configured, preflight returns blocked before any gate runs
  (`internal/campaign/risk_scoring.go`, `runRiskPreflight`).
- **Checkpoints that cannot verify report failure.** Shard validation and the
  nemesis gauntlet fail closed without a task executor; manual review
  escalates to shard validation instead of consulting a human
  (`checkpoint.go:253-264`).
- **An unknown verification method fails closed.** The `default` arm of
  `CheckpointRunner.Run` returns failed with an error
  (`checkpoint.go:95-103`), and the policy blocks a campaign whose phase names
  a method the runner cannot run (`campaign_blocked(C, /unverifiable_objective)`).
  Pinned by `TestCheckpointRunner_AnUnknownMethodFailsClosed`,
  `TestPolicy_APhaseWithAnUnknownMethodBlocksTheCampaign` and
  `TestPolicy_TheKnownVerificationMethodsAreTheOnesRunRuns`
  (`checkpoint_verdict_test.go`).
- **A test run leaves a host witness.** `executeTestRunTask` clears
  `Task.TestWitness` when an attempt starts
  (`orchestrator_task_handlers.go:836`) and sets it only after the check ran
  through the gated VirtualStore (`run_tests`) and the workspace snapshot was
  unchanged across the run (`:891-893`). Pinned by
  `TestTestRunRequiresHostWitness` (`task_effect_contract_test.go:24`). It is
  persisted in `campaign.json` as the run's receipt; no code reads it back.
- **Write sets are enforced at dispatch and at the moment of each write.**
  `acquireWriteSetLease` (`orchestrator_tasks.go:459`) serializes tasks over
  `resolveTaskWriteSet`; `spawnTask` puts a write guard on every turn
  (`orchestrator_task_handlers.go:48` → `session.WithWriteGuard`), which the
  executor consults before any write-mutation tool runs
  (`internal/session/executor_tools.go:976`): an undeclared path is leased for
  the task at the moment of the write and a path another task holds is
  refused. A failed attempt's writes are undone before its leases go back
  (`orchestrator_attempt_writes.go`). Pinned by
  `TestWriteGuard_HoldsAnUndeclaredPathForTheAttemptAndRefusesAHeldOne`
  (`orchestrator_write_guard_test.go:14`). `Task.DeterministicWriteSet`
  (`types.go:818`) feeds the task's facts, the replanner and risk scoring.
- **Recursive sweeps have two drivers.** `nerd campaign recurse`
  (`cmd/nerd/cmd_campaign_recurse.go:173`, registered in `cmd/nerd/main.go`)
  runs a `RecurseRunner`; chat `/recurse` and `/campaign recurse` run a
  `RecurseLoop` and chain waves (`cmd/nerd/chat/campaign_recurse.go:146`,
  `:252`).
- **The bounding counters are incremented.** `Phase.CheckpointFailures`
  by `incrementCheckpointFailures` (`orchestrator_tasks.go:22`, called at
  `:155`) and mirrored to the kernel; reset on resume
  (`orchestrator_resume.go`). `Task.ReplannedAtCap` is set when the attempt-cap
  replan runs (`orchestrator_failure.go:101`). Pinned by
  `TestRunPhase_WhenCheckpointFails_ShouldNotCompletePhase`,
  `TestIncrementCheckpointFailures_BoundsAndIsolation` and
  `TestAttemptCap_TriggersReplanBeforeBlock`.
- **Degraded plans are labelled.** An empty LLM plan becomes a scaffold with
  `PlanDegraded` set, persisted across resume so it cannot be mistaken for a
  real plan.
- **Journal verify/replay is operator-grade.** List, verify, replay, and render
  all exist (`internal/campaign/journal_ops.go`) and back the
  `nerd campaign journal` CLI surface.
- **Risk auto-wiring is on in production.** The zero-config path forces
  `EnableRiskAutoWiring` on (`orchestrator_init.go:334-340`) unless a caller
  sets task-level risk overrides, and no production caller does. The
  "everything off" path (`risk_scoring.go:286-303`) is reachable only
  programmatically, and it emits `EventRiskGateSkipped`.

## Not built, by decision

- **No interactive human review.** `/manual_review` never consults a person;
  it escalates to a reviewer shard and says so in the checkpoint details
  (`checkpoint.go:253-264`). Declined: campaigns run unattended and the
  decomposer picks `/manual_review` for most phases, so a human wait would
  stall them; adding one is a product decision. Any process that assumes "a
  human approved this phase" is assuming something the code does not do.
- **Below-threshold scores run no strict gates** (`risk_scoring.go:346`,
  audited as `EventRiskGateSkipped`). That is the risk-scoring design;
  protected surfaces are gated before the score is read.
- **The completion sweep covers only the workspace root.** It moves only root
  files absent from `RootBaseline` and undeclared by every task
  (`sweepUndeclaredRootWrites`, `orchestrator_tasks.go:843`). Declined: write
  sets are inferred, so a file a task created under `internal/` may be the
  work itself; moving nested files would move legitimate output
  (`snapshotWorkspaceRoot`, `:744`, records the reasoning).

## Fixed in this pass (2026-09-25)

- `TestTaskEvidence_ATasksOwnOutputIsNotItsEvidence` and
  `TestTaskEvidence_ADocumentTheBriefNamesArrives` failed on Linux: they
  hand-wrote a Windows-lowercased write set (`docs/readme.md` for
  `Docs/README.md`), which names the same file only on a case-insensitive
  filesystem. Production folds write-set case on Windows only
  (`normalizeAbsolutePath`, `write_set_lock_manager.go`), and `outputFiles`
  identifies a task's own output by `os.SameFile`, correctly. The fixtures now
  build the write set with the production normalizer (`storedWriteSet`); no
  assertion changed. Commit `bdd2983`.

## What the old documents got wrong

- **19 markdown files for one package.** The old directory (`00-`–`12-`
  numbered files plus `IMPLEMENTED_SPEC.md`, `OPEN-QUESTIONS.md`, `TODO.md`,
  `_progress.md`) was replaced by three files with one question each.
- **Stale scale in the in-package map.** `internal/campaign/README.md` claims
  "49 non-test sources (~22.2k lines), 59 test files" and per-file sizes that
  no longer hold. The module map itself is accurate.
- **A plan shown at "50% confidence" may be a scaffold.** Any document that
  presents a degraded plan as a real one repeats the failure `PlanDegraded`
  exists to prevent.
- **This file's 2026-09-20 version** listed four "unverified" items that were
  wired and tested, and "unknown verification methods pass", which had been
  fixed.
