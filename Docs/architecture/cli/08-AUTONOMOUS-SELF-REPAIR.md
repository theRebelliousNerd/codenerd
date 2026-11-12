---
doc-class: shipped-with-future
subsystem: cli
implementation-status: partial
last-verified: 2026-11-12
verified-against: f8bbd6ba0ae0daca62073170d4cc0a94cfb1a359
supersedes: []
---

# Autonomous self-repair

While the `debug_self` logging key is on, codeNERD fixes and speeds up its own source with no
one asking, restarts onto the fix, carries on with what it was doing, and judges the fix
from the new process's own evidence. The kernel decides what is due; the chat executes.

## Target behaviour

1. **Trigger.** `self_repair_due(Target, Lane)` is derived
   (`internal/core/defaults/policy/self_repair.mg:229-230`). The chat asks at a turn
   boundary and on an idle timer, never through perception.
2. **Two lanes.** `/repair`: every hot problem of the session's problems log that is not
   held. `/optimize`, once nothing is left to repair: the recurring operation that cost
   this run the most time (count times mean), from this process's own audit log. Time
   spent waiting on a model, the network or a browser is excluded. Only timings taken
   after the startup scan count (`self_timing_settled`): boot-time evaluations of the
   whole world model are not the steady-state cost, and the baseline and the verdict
   after the restart use the same settled mean.
3. **The user comes first.** Nothing starts while a turn runs or is queued, held work or
   an offered step waits, an approval is open, a campaign or `/recurse` runs, a restart is
   pending, or restored work has not resumed. A self-repair follows the work mode: only
   yolo runs the fix unattended.
4. **Lifecycle.** `being_fixed` -> `pending_restart` -> `verifying` -> `verified` or
   `regressed`, plus `failed`, `stopped` and `no_defect`, persisted in
   `.nerd/self_repair/ledger.json` so it survives the re-exec. A target that failed
   the `self_repair_max_failures` logging key times is left. A retry is told what earlier attempts
   came to.
5. **Self-checked install.** The rebuild goes to a side file; it must start (`--help`),
   its changed `.mg` files must pass `check-mangle` and its changed packages `go vet`,
   and only then is it renamed over the running binary. A failed build or check is logged
   as an ERROR (so it becomes a `self_problem`) and the old binary stays.
6. **Resuming restart.** Running background jobs (with follow-up and lane) and queued
   turns are written to the restore state (`Resume`) and started again after the startup
   scan of the new process.
7. **Verdict before the fix.** A finished diagnosis is judged per target by Jev
   (`JudgeDiagnosis`, `internal/turnjudge/verdict.go:56`; switch `diagnosis_verdict`
   under `typesafe`). The kernel derives `self_diagnosis_fix_due(Job)`
   (`internal/core/defaults/policy/self_diagnosis.mg:67-68`): true when any target is a
   fix target (yes or unsettled), or when the job has no verdict because Jev is off or
   the call failed. A target at or below the no threshold settles as `no_defect`
   (`self_diagnosis.mg:58`): no fix turn, not a failure, and held until its count grows
   by the configured regrowth factor over what the diagnosis saw
   (`self_repair_evidence_regrown`, `self_repair.mg:147`). With nothing due the user sees
   one line saying the diagnosis found nothing of codeNERD's to fix.
8. **The fix step is a chain step.** It runs with a preset intent (`/fix`, or `/refactor`
   on the optimize lane) aimed at the files the diagnosis cites (`fixStep`,
   `cmd/nerd/chat/self_diagnosis_verdict.go:280`). It is never perceived again and never
   decomposed into a plan (`intent_signal(/chain_step)`,
   `internal/core/defaults/policy/routing_arbitration.mg:236`), and in yolo it is not held
   on the conversation's brief (`:270`). A queued chain step survives a restart as a step
   and gives way to the user's queued messages. A fix turn the user stops settles as
   `stopped`, not `failed`.
9. **Safety.** While a fix turn runs, `self_repair_fixing()` makes the constitution, the
   safety and permission policies, the write guards, the approval and work-mode code, the
   self-repair trigger itself and `nerd.md` `project_forbidden_path`
   (`self_repair.mg:279-303`). The executor's write gate and `coder_edit_barred` both
   refuse them.

10. **A fix that is not installed is undone.** In yolo the fix chain runs under a write
    journal (`WriteJournal`, `internal/session/write_journal.go:53`) and a fix window
    (`self_repair_fix_window`). When the chain stops, and before any queued user turn,
    `settleFixTurn` (`cmd/nerd/chat/self_repair_fix_turn.go:98`) restores every file a
    fix wrote if that fix did not end built, self-checked and installed. A file another
    writer changed since is left as found and named. The attempt's diff and its failing
    tests are kept under `.nerd/self_repair/attempts/`, the ledger names them, and a
    retry is told what was tried and which tests failed. A user's own turn, or a fix the
    user approved in another work mode, is never undone.
11. **A failed attempt's record is not a target.** A problem logged only inside fix
    windows is derived as the attempt's own record (`self_problem_from_repair`,
    `self_repair.mg:178`) and held. A turn's verdict on the workspace (verification
    failed, repair rounds, a gate's FAILED line) is logged at INFO: the turn already
    reports it, and it describes the edits, not a fault in codeNERD.

## Go / Mangle split

| Decision | Where |
|---|---|
| What is due, on which lane, by which persona | `self_repair.mg` (`self_repair_due`, `self_repair_shard`) |
| Slot, cooldown, failure limit, hold | `self_repair.mg` (`self_repair_slot_open`, `self_repair_held`) |
| Verdict after the restart | `self_repair.mg` (`self_repair_verified`, `self_repair_regressed`) |
| Whether the fix turn runs, and for which targets | `self_diagnosis.mg`, `jev_evidence.mg` (`self_diagnosis_fix_due`, `self_diagnosis_no_defect`) |
| Which problems are an attempt's own record | `self_repair.mg` (`self_problem_from_repair`) over per-second occurrences (`self_problem_at`) and fix windows |
| Undoing a fix that was not installed | Go: `WriteJournal` and `settleFixTurn` |
| What a fix turn may not edit | `self_repair.mg` -> `project_forbidden_path` |
| Measuring: problems log, timings, ledger, clock | `internal/problemlog/repair.go`, `timing.go` |
| Asking, starting the job, recording outcomes | `cmd/nerd/chat/self_repair.go` |
| Idleness (is anything of the user's in the way) | `cmd/nerd/chat/self_repair.go:149` (Go: UI state) |
| Build, self-check, install, resume | `cmd/nerd/chat/self_restart.go:378`, `:448` |

Knobs live in the logging section of `.nerd/config.json`: whether self-repair is disabled, its cooldown,
failure limit, job limit, verify window, poll interval and self-check timeout, and the optimize
lane's minimum count (`SelfOptimizeMinCount`, `internal/config/logging.go:55`, and the fields beside it).

## Not built

- The yolo fix -> real rebuild -> exec -> resume -> verify path is proven with fakes
  (TestSelfRepair_WalksTriggerJobFixRebuildRestoreResumeVerify); see GAP-CLI-SELF-REPAIR-06.
- A running fix turn is not interrupted mid-edit; it can only be stopped, which records it
  as stopped. A pause point between tool calls inside the executor is not built
  (GAP-CLI-SELF-REPAIR-08).
- The write journal and an open fix window do not survive a crash mid-fix: a window found
  open at load is closed at its start, and the edits stay (GAP-CLI-SELF-REPAIR-09).
- An installed fix that regresses after the restart is recorded as regressed but its
  edits are not taken back out.
- Task text for the diagnosis and fix steps is Go constants, not prompt atoms.
- Each idle poll re-asserts the clock and ledger facts and re-evaluates.
