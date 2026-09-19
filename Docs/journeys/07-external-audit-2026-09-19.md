# 07 — External audit, 2026-09-19: findings, verification and routing

Two external audits of `40c6f61a` reached Steve on 2026-09-19 (his downloads, `AUDIT.md` and
`AUDIT (1).md`, with `probe-output.txt`): a seven-finding capability and wiring audit (F1-F7) and
an additive "wave 2" of sixteen (N01-N16). Both are source-anchored; wave 1 reproduced three
findings and wave 2 eight mechanisms with isolated Go probes. This file is the tracked record:
what each finding claims, whether it was checked against the code here, where it is routed, and
what landed. The audits themselves stay where Steve put them.

Their acceptance criterion, adopted as ours: evidence cannot be borrowed, dropped, used after
becoming stale, or detached from the mutations it is supposed to describe.

## Status

- last updated: 2026-09-19 13:50
- landed: F7 (`247a2402`), F6 (`ac5c9b03`), F3 (`8dd5ef48`), F5 (`a6d4e572`), F1 (`662699eb`),
  N03 (`90100128`), N07 (`0b0a4d49`), N02 (`8c906461`), N01 (`b41565cf`), F2 with C3 (`6df972de`),
  F4's rollback half with C4 (`ad91108d`).
- next, hand-built: F4's isolation half (a lease taken at write time, not plan time) before any R2+
  campaign run. N01 follow-on: a forcing round for `/test_run` -- a turn that owes a test run and
  never started one ends `/unverified` with the run named, and nothing yet sends it back to run it.
- next, as codeNERD ladder briefs (one or two files, tool code, symptom and evidence in hand):
  N04 (reproduced 13:30: a partial second write leaves `b.txt` truncated after the rollback),
  N10, N09, N05, N06, N14.

## Found here while working the audit

- **L1 the VirtualStore's `run_tests` and `build` handlers run a free command string.**
  `handleRunTests`/`handleBuildProject` (`core/virtual_store_actions.go`) take `command` from the
  action payload (or a command-looking target) and run it through `cmd.exe /C` or `bash -c`. The
  model-facing `run_tests`/`run_build` tools are typed and refuse a `command`; this path is reached
  by kernel-routed actions. Whether any producer lets model text into that payload is unaudited --
  safety-gate territory, hand-built.

## Routing rule


Hand-built: completion-gate, verdict, rollback and scheduling-authority logic (the model does not
widen the rule that constrains it), and anything that stops codeNERD from running. Everything else
that is one or two files with the evidence in hand is a ladder brief for codeNERD -- these are
real defects in its own tools, found by someone else, which is the best rung-1 material the ladder
has had. Larger items wait for their rung.

## Findings

"Verified" means the claimed mechanism was found in the code at HEAD on 2026-09-19 (a file:line
read, or a run where one is named). It is not a reproduction unless the row says so.

| ID | Audit priority | Finding | Verified here | Route | Status |
|---|---|---|---|---|---|
| F1 | P1 | Transient turn facts keyed by verb, not execution: `turn_evidence(Verb,…)`, `turn_gate(Verb,…)`, unkeyed `hollow_success` on the kernel `CloneForTask` shares; cleanup can sweep another executor's facts | reproduced on `a6d4e572`: B closed `/done` on A's test gate; B's cleanup took A's verdict; an early-return turn swept A's `turn_evidence` | hand | **landed `662699eb`** (every verdict relation keyed by a minted turn atom; cleanup retracts only what the turn asserted; created files are the executor's record, not `created_source`/`test_file_for` in the shared kernel; the legacy claim predicates deleted) |
| F2 | P1 | Campaign `spawnTask` takes the string route and drops the typed outcome, so `/unverified` with a nil error completes the task (the ladder's C3/C4) | known (C3/C4) | hand, before R2+ campaigns | **landed `6df972de`**, with C3 (the campaign reads the observed return and fails any outcome but `/done` by its missing evidence; the retry carries the last failed attempt's error; a prose-only executor is refused at construction) |
| F3 | P1 | Gate order build→tests→coverage→vet→removed-tests→**critic**→close: the critic edits after the last guard, and the final closure refreshes tests only (`gateTests(…, false)`, second return discarded) | seven regressions fail on `a9534981` | hand | **landed `8dd5ef48`** (the closure re-measures build, tests with coverage, vet and the test inventory at the final revision; the critic runs before the forcing rounds; a broken uplift is undone) |
| F4 | P1 | Declared write sets lock and snapshot; actual writes outside them are neither isolated nor rolled back (the ladder's C4) | known (C4); reproduced on `6df972de`: a retry found the failed attempt's import in a file outside the set | hand, before R2+ campaigns | rollback half **landed `ad91108d`** (the observed return carries each write's preimage and postimage; a failed attempt's writes are undone compare-and-swap, never over a later writer's). Isolation half open: leases cover declared paths only |
| F5 | P1 | `go vet` findings in files the turn did not write become `VerifyPassed`; a cause in `state.go` (a mutex added) diagnosed in `use.go` is discarded | the audit's case passed vet on `8dd5ef48` | hand | **landed `a6d4e572`** (findings are the difference from the same packages vetted before the turn, by file and message; no baseline charges every finding; a failure naming nothing is indeterminate). Reverse dependencies are not vetted, as they are not tested -- with N05 |
| F6 | P1 | A deleted test is excused when any test anywhere shares its name | `workspaceTestNames`; fails on `247a2402` as the audit said | hand | **landed `ac5c9b03`** (a test moves only within its package directory, among files the default build includes; a copy behind `//go:build ignore` is a removal) |
| F7 | P2 | Rollback deletes a pre-existing empty file (`exists = pre != ""`) | reproduced on `f91c39b6` | hand | **landed `247a2402`** (also: unreadable ≠ absent in the snapshot, the current-state read and the test-baseline overlay; an undo that fails stays written) |
| N01 | P1 | Build/test gates run only when a `.go` file was written; policy demands them for every write, so non-Go work has no affirmative completion path | `build_verify.go:222,297,602`; reproduced on `4b68a04f`: a `.md`-only turn derived no `turn_done` | hand (completion contract) | **landed `b41565cf`** (what a write owes is policy, by extension: Go owes the executor's gates, documentation nothing mechanical, anything else a passing test run after the last write -- `/test_run`, from N07's receipts) |
| N02 | P1 | `getEligibleTasks` falls back to an in-memory scan whenever the kernel's answer is empty -- including a valid "wait" | `orchestrator_phases.go:94-98`; the fallback checks dependencies only, so it schedules tasks `eligible_task` withholds for write-set conflicts, ordering or backoff. Reproduced on `0b0a4d49`: a pending task whose write target an in-progress task holds was scheduled | hand (scheduling authority) | **landed `8c906461`** (the kernel is the only scheduler; a kernel holding none of the phase's tasks is reloaded and asked again. Removing the scan exposed two defects it hid, both fixed: a priority-inversion deadlock in `has_earlier_task`, and a campaign clock nothing moved, so a retry stayed in backoff for the run -- the orchestrator now feeds `current_time` at most once a second) |
| N03 | P1 | Three failed phase checkpoints call `completePhase`: `/completed`, progress, a success observation | `orchestrator_tasks.go:144-152`; the old test pinned `/completed` | hand (completion) | **landed `90100128`** (the phase closes `/unverified`: hard dependents blocked, the campaign blocked on it by name, the observer told failure, a resume re-arms the checkpoint) |

| N04 | P1 | `apply_edits` rollback visits `succeeded` only; a target whose write failed part-way is left corrupt | `apply_edits.go:332-379` | codeNERD brief | open |
| N05 | P1 | Impacted-test selection follows two dependency edges, not a fixed point | `test_dependency.go` ("simplified - could query kernel for full transitivity") | codeNERD brief | open |
| N06 | P2 | Coverage-gap report counts production callers as test coverage | not yet checked | codeNERD brief | open |
| N07 | P1 | `run_impacted_tests` is a test execution by name alone; a dry run or an empty selection mints executed-test evidence | reproduced on `90100128`: a dry run counted `SuccessfulTestTools = 1` | hand (evidence) | **landed `0b0a4d49`** (a test run is a receipt the process-starting code records -- typed runner, `runGoTests`, shell test commands, the VirtualStore handler; the counter is `TestRunCalls`) |
| N08 | P1 | The registered CodeDOM reader is per-line regex: misses generic funcs and `async def`, invents declarations inside raw strings | not yet checked | R2 (route readers to the parser) | open |
| N09 | P2 | `get_element` returns the first same-named element; `A.Close` vs `B.Close` silently resolves to one | `elements.go` first match | codeNERD brief | open |
| N10 | P2 | The edit delimiter guard counts only the replaced fragment, so a valid edit inside a multiline literal is refused | `lines.go:171-179`; R1-4d's log has one such refusal (08:34:57) to classify | codeNERD brief | open |
| N11 | P1 | A failed reread supersedes the successful observation of the same revision | not yet checked | R2 (working policy + store) | open |
| N12 | P1 | Aggregate observations are stamped with the focused file's revision | not yet checked | R2 | open |
| N13 | P1 | The working-context archive scope is random per executor (`rand.Text()`), so a resumed executor cannot reopen it | `working_context.go:125-126` | R2 | open |
| N14 | P2 | Co-use statistics settle every nil-error turn as success, `/unverified` included | `executor.go:913-917` | codeNERD brief | open |
| N15 | P2 | The impacted-test provider is process-global, last workspace wins | `run_impacted_tests.go:63-98` | R2 | open |
| N16 | P2 | A contained symlink stops snapshot certification (fail-closed, a capability limit) | `change.go:173-174` | decision (Steve) | open |
