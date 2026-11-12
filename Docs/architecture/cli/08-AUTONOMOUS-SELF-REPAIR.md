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
   (`internal/core/defaults/policy/self_repair.mg:156-157`). The chat asks at a turn
   boundary and on an idle timer, never through perception.
2. **Two lanes.** `/repair`: every hot problem of the session's problems log that is not
   held. `/optimize`, once nothing is left to repair: the recurring operation that cost
   this run the most time (count times mean), from this process's own audit log. Time
   spent waiting on a model, the network or a browser is excluded.
3. **The user comes first.** Nothing starts while a turn runs or is queued, held work or
   an offered step waits, an approval is open, a campaign or `/recurse` runs, a restart is
   pending, or restored work has not resumed. A self-repair follows the work mode: only
   yolo runs the fix unattended.
4. **Lifecycle.** `being_fixed` -> `pending_restart` -> `verifying` -> `verified` or
   `regressed`, plus `failed` and `stopped`, persisted in
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
7. **Safety.** While a fix turn runs, `self_repair_fixing()` makes the constitution, the
   safety and permission policies, the write guards, the approval and work-mode code, the
   self-repair trigger itself and `nerd.md` `project_forbidden_path`
   (`self_repair.mg:205-229`). The executor's write gate and `coder_edit_barred` both
   refuse them.

## Go / Mangle split

| Decision | Where |
|---|---|
| What is due, on which lane, by which persona | `self_repair.mg` (`self_repair_due`, `self_repair_shard`) |
| Slot, cooldown, failure limit, hold | `self_repair.mg` (`self_repair_slot_open`, `self_repair_held`) |
| Verdict after the restart | `self_repair.mg` (`self_repair_verified`, `self_repair_regressed`) |
| What a fix turn may not edit | `self_repair.mg` -> `project_forbidden_path` |
| Measuring: problems log, timings, ledger, clock | `internal/problemlog/repair.go`, `timing.go` |
| Asking, starting the job, recording outcomes | `cmd/nerd/chat/self_repair.go` |
| Idleness (is anything of the user's in the way) | `cmd/nerd/chat/self_repair.go:141` (Go: UI state) |
| Build, self-check, install, resume | `cmd/nerd/chat/self_restart.go:373`, `:439` |

Knobs live in the logging section of `.nerd/config.json`: whether self-repair is disabled, its cooldown,
failure limit, job limit, verify window, poll interval and self-check timeout, and the optimize
lane's minimum count (`SelfOptimizeMinCount`, `internal/config/logging.go:55`, and the fields beside it).

## Not built

- The yolo fix -> real rebuild -> exec -> resume -> verify path is proven with fakes
  (`TestSelfRepair_WalksTriggerJobFixRebuildRestoreResumeVerify`); see GAP-CLI-SELF-REPAIR-06.
- Once a fix turn starts it holds the turn slot; a user message queues behind it.
- Task text for the diagnosis and fix steps is Go constants, not prompt atoms.
- Each idle poll re-asserts the clock and ledger facts and re-evaluates.
