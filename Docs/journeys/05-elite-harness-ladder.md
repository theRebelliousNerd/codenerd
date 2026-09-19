# 05 — The elite-harness ladder

Steve's goal (2026-09-19): codeNERD dogfoods itself until the codebase is error free, and climbs
a ladder of milestones that would let it be called an elite coding harness: 1, 2, 5, 10 and 50
file edits from a single prompt on a real problem or optimisation quest; a rewrite of every doc in
`Docs/architecture` that codeNERD does itself, to a standard its reviewer is impressed by; and
recursive, incremental improvement of the whole codebase.

This file is the program of record: the rungs, what counts as passing one, the error-free gates,
the protocol for a run, and the status of each rung with the run that moved it.

## Status

- last updated: 2026-09-19 07:20
- **R0 gates passed** on `10223378`: three consecutive uncached runs (06:21-06:36), each G1 build,
  G2 `go vet -tags sqlite_vec ./...` and G3 `go test -count=1 ./...` green, 89 of 89 packages, 0
  cached, 4.8-4.9 min. The earlier attempts ran `go test ./...` with the test cache, so a "green"
  run re-reported old passes for every package whose tests touch no files -- a timing flake there
  could never show; they do not count. R0's other clause (a run's verdict matches its evidence) is
  reviewed on every codeNERD run: true for R1-3 and R1-4 (R1-4 failed and said so).
- current rung: R1 -- streak 0: R1-3 landed, R1-4 was refused by the removed-tests guard. Next: the
  same brief again on `279b73fe`, where that guard hands the deleted tests back.
- landed since R1-2: the forcing gate (`fd3c1d99`: changed code no test executes, and `go vet`
  findings in the turn's own files, are verdict evidence with a repair round first -- R1-2 had
  passed as done over both); two more load flakes (`10223378`: the watcher debounce test, and the
  boot test now names whatever still holds the workspace after `Close` instead of failing only in
  TempDir's cleanup); the removed-tests guard's repair round (`279b73fe`: a turn that deleted
  tests is handed each one's source to put back and the tests are rerun, where it used to fail on
  the spot -- R1-4's blocker)
- open: the harness defects below (C1-C4, V1); G6 needs a toolchain download
  (asked); codeNERD cannot run the gates it will be asked to clear -- no typed tool runs `go vet`,
  staticcheck or golangci-lint for the model (the forcing gate runs vet itself, the model cannot),
  and `nerd fix --acceptance` takes failing tests, not a gate. G4/G5 items are therefore not rung-1
  probes (criterion 5), and R5 (clear a class of findings) and R7 (measure the gates, pick the
  next item) need that tool first.

### Harness defects to fix before R2

C1-C5 from the R1-2 campaign comparison (`campaign_1284b6bb`); V1 found while fixing C5:

- **C1 targets fixed before research.** Every write set was decided when the plan was made, before
  phase 0's research ran. Phase 2's task targets `internal/cli/check-mangle.go`, a path that does
  not exist; the command lives in `cmd/nerd/cmd_mangle_check.go`, which phase 0 found. Nothing
  carries research results back into the tasks that follow it.
- **C2 a missing modify target becomes a create.** `reconcileTaskTypeWithWriteSet`
  (`internal/campaign/decomposer_planning.go`) retypes a `/file_modify` whose write set does not
  exist to `/file_create` (planned `/file_modify`, executed `/file_create`), so a wrong guess turns
  into a new file at the wrong path instead of a plan error.
- **C3 a retry drops its reason.** The retry re-spawned the same create task without the failure
  that stopped it, and the create-only fallback refused because the file now existed.
- **C4 rollback covers the declared write set, not the attempt's writes.** It removed the created
  file and kept the coder's edit to `cmd_mangle_check.go` -- outside the set -- which imported it,
  so the tree stopped building. `TestRollback_KeepsFilesOutsideTheWriteSet` pins today's
  behaviour; a fix has to tell the attempt's own writes from anyone else's. The campaign cannot
  today: it spawns through `TaskExecutor.Execute`, which returns the shard's prose, while the
  typed return (`observation.Return`, on `session.TaskResult.Observed`) already carries the
  attempt's write set (`Changed`), its own error (`Failure`) and the kernel's verdict (`Outcome`).
  C3 and C4 share that root: the typed result (S1) stops one seam short of the campaign.
- **C5 a gated coder turn has a way around its gate.** When the coder shard fails a `/file_create`
  task for any reason but a cancel -- including the test obligation ("hollow success blocked: turn
  created Go source ... without a test file") -- `executeFileTaskFallback`
  (`internal/campaign/orchestrator_task_handlers.go`) asks the model to "Generate the following
  file" and writes the answer through the VirtualStore: permission and write validation apply, the
  turn's obligations (tests, coverage, vet, verdict) do not. In R1-2 it was refused only because the
  blocked turn had already written the file; C2's retype is what made it reachable. The completion
  gate's reach -- hand-built, not a codeNERD brief. Fixing it found the bypass latent: the fallback
  asserted a `pending_action` payload (`{"content_bytes":N}`) the kernel's permission check, which
  matches the exact routed payload, never matches, so every fallback write was refused -- the
  document deliverables it exists for (F-DOC-1) included -- after the model had generated them.
  Correcting the payload alone would have opened the bypass; the fix lands both: documents only,
  the canonical payload, and every refusal leading with the coder's reason.
- **V1 the VirtualStore does not write what it is given.** `handleWriteFile` passes every
  `write_file` through `extractCodeBlockForFile`, which trims the content and, for a Go file,
  drops everything before the first `package ` -- a `//go:build` line or a licence header. The
  store's own `FileWriteValidator` then reports "content hash mismatch" for any content that ends
  in a newline (measured: the campaign's degraded-generation placeholder). The coder's tool loop
  writes through `internal/tools/core` and is unaffected; the store's route serves the campaign
  fallback, the system router, `cmd_instruction.go`, `pending_action.go` and the interactive gate.
  A multi-file fix (the handler and whatever its callers rely on it to strip): an R2 candidate for
  codeNERD.

## What counts as a landing

A landing is one prompt to codeNERD (`nerd fix "<brief>"`, or one `nerd chat` request) on a real
problem in this repository, whose result is kept. All of these hold:

1. **The brief is a symptom, not a diagnosis.** It quotes what was observed (an error, a log line,
   a failing check, a measured cost) and what should hold instead. It does not name the cause or,
   where the symptom allows, the file.
2. **codeNERD alone edits the tree.** No hand edit to the diff before it is kept. A trivial
   post-edit (formatting, a comment) is recorded as such and caps the landing at "assisted".
3. **The change is proven.** Tests that fail before and pass after (the reviewer checks the
   "fail before" by reverting the fix), and `go build ./...`, `go vet ./...` and `go test ./...`
   green after.
4. **The verdict is truthful.** codeNERD's own result says done only when its evidence shows it:
   no "done" over a test that never ran, no success line over a failed gate.
5. **The failure is reproducible with codeNERD's own tools** (a command it may run), or the brief
   carries an acceptance contract. A failure only the reviewer can reproduce (R1-1 needed 32 busy
   loops) cannot be told red from green inside the run, so it is not a rung-1 probe.
6. **The file count is the fix's, not the noise's.** Files edited to fix the problem, tests
   included, count; generated files and formatting churn do not.
7. **The review finds no defect the change introduces** (added 2026-09-19 07:20, before R1-4's
   rerun). The reviewer measures the change beyond its own tests -- planted errors, the inputs next
   to the ones the tests use -- and a behaviour the change makes worse than before fails the
   landing even when every test and gate is green. Nits (style, a missed reuse) are recorded and
   do not. R1-3 met it; R1-4's fix would not have: a malformed Decl planted in `task_stage.mg` was
   reported in that file alone before (measured, beside the three rejections the brief is about)
   and in 84 files after, none of them naming it.

Every run, landed or not, is recorded in the dogfood ledger
(`.claude/skills/codenerd-dogfood/references/component-ledger.md`) with the brief, minutes, tool
calls, tokens, files, what landed and what it missed. A run that fails for a reason in the
harness (a tool that refuses a correct edit, context that never reached the model, a verdict that
lies) is followed by fixing that reason, test first, and the same brief is run again: the blocker
is the finding.

## The rungs

| Rung | Pass when | Why it is on the ladder |
|---|---|---|
| R0 Stable ground | the gates G1-G3 are green on `main` for three suite runs in a row, and a run's verdict matches its evidence | nothing above it can be measured on sand |
| R1 One file | three consecutive landings, each fixing a real problem whose fix is one file (plus its test) | the brief shape works when the cause is local |
| R2 Two files | three consecutive landings where the fix spans two files (a producer and its consumer, a rule and its Go assertor) | the agent follows a cause across a seam |
| R3 Five files | two landings of five-file fixes or features | planned multi-step work, context served per step |
| R4 Ten files | two landings of ten-file changes (an API migration, a cross-cutting fix) carried out through CodeDOM multi-file edits, not hand-edited file by file | a change with a blast radius is executed by a tool |
| R5 Fifty files | one landing of a fifty-file change from a single prompt: an optimisation quest or a sweep that clears a whole class of gate findings | long-horizon work without drift |
| R6 The architecture docs | codeNERD rewrites every document in `Docs/architecture` from the code (never from the old docs), every claim citing a path and symbol that exist, links and symbols verified by a checker, and the reviewer, reading a sample of each package's docs against the code, finds nothing false and nothing important missing | the harness can hold a whole codebase in view and write the truth about it |
| R7 Recursion | `nerd campaign recurse` (or its successor) runs unattended for N cycles: measures the gates, picks the next failing item, fixes it, proves it, and each cycle leaves the gates strictly better and nothing worse | it improves itself |

Rungs are climbed in order; a later rung's run may happen earlier as a probe, but it does not
count until the rungs below it are passed.

## The error-free gates

"Error free" means every gate at zero on `main`, each measured by a command anyone can run:

| Gate | Command | Baseline 2026-09-19 |
|---|---|---|
| G1 build | `go build ./...` | 0 errors |
| G2 vet | `go vet -tags sqlite_vec ./...` (every package) | 0 |
| G3 tests | `go test -count=1 ./...`, three consecutive green runs (no flakes; uncached, or a run re-reports old passes) | 88 of 89 on `f26140e1`: `TestRunToolLoop_ReservesTimeForFinalVerdict` fails 1 in ~6 runs alone ("compile working context: context deadline exceeded") |
| G4 staticcheck | `staticcheck -tags sqlite_vec ./...` | 103: 95 unused (U1000; 24 of them in `cmd/nerd/cmd_campaign.go`), 3 SA4000, 2 SA5011, 1 SA4023, 1 SA9003 |
| G5 golangci-lint | `golangci-lint run --build-tags sqlite_vec --max-issues-per-linter=0 --max-same-issues=0 ./...` (default linters; without the two caps it prints 169 and hides the rest) | 3,286: errcheck 1,851 (1,176 in tests), staticcheck 1,319 (1,190 are QF1012 `WriteString(fmt.Sprintf(...))`), unused 95, ineffassign 16, govet 5 (`reflect.Ptr`) |
| G6 vulnerabilities | `govulncheck -tags sqlite_vec ./...` | 11 reachable: 8 in the standard library (go1.26.4, fixed in 1.26.6), 2 in `google.golang.org/grpc` v1.81.1, 1 in `golang.org/x/text` v0.37.0 |
| G7 policy corpus | `nerd check-mangle internal/core/defaults/*.mg internal/core/defaults/policy/*.mg internal/core/defaults/schema/*.mg` | 3 of 135 files: each uses a predicate declared in a sibling file (`reviewer.mg`, `policy/task_stage.mg`, `policy/schemas_perception_latency.mg`) that the checker does not preload, though the kernel loads them together -- the checker disagrees with the kernel. 0 wildcard negations in the corpus |
| G8 atom corpus | Mangle examples in `internal/prompt/atoms` that fail to load unmarked (`TestAtomCorpus_MangleExamplesTheEngineRejectsDoNotGrow`) | 344 at `f26140e1` (888 on 2026-09-18, 654 after `86e461f2`) |
| G9 the corpus guards | `go test ./internal/prompt/ -run TestEmbeddedCorpus` | green |

A gate that is not zero is a backlog of real problems: it is where R1-R5 briefs come from.
Sized to the rungs, the backlog on `f26140e1` offers: single-file items (the final-verdict flake,
the checker's preload, SA2001's empty critical section, the SA5011 test that dereferences after
`t.Error`); a five-file class (govet's `reflect.Ptr`, SA1012's nil contexts); a ten-file class
(ST1005's 20 capitalised error strings); and the fifty-file quest (QF1012's 1,190 sites, a change a
tool should carry out). The 95 unused functions are not a deletion list: each is first audited
for the wiring it was meant to have (repo contract).

## Which entry point for which rung

| Entry point | What it is | Used for |
|---|---|---|
| `nerd fix "<brief>"` | one turn: the executor's tool loop, with planned steps compiled per step | R1, and as the baseline an R2 campaign is compared against |
| `nerd campaign start "<goal>" --type remediation\|feature\|migration` | a short campaign: the goal decomposed into phases and tasks, each verified before the next | R2-R5, R6 (one campaign per package's docs) |
| `nerd campaign recurse --waves N --subsystem S --angles A` | waves of campaigns over the subsystem DAG, each wave led by what failed last | R7 |

Why both: a single fix turn is the shortest loop to measure, but a change that spans files is what
a campaign exists for (decompose, execute, verify per phase). Where a brief could go either way,
it is run both ways and the ledger records which landed and at what cost.

## Protocol for one run

1. Pick a real problem from a gate's backlog or from the study's seams, sized to the rung.
2. Write the brief: the symptom, the evidence, what should hold. Save it as a file.
3. Rebuild `nerd.exe` from `main` (the prompt atoms are embedded: a corpus fix only reaches the
   agent after a rebuild). Nothing else edits the tree while the run is in flight.
4. Run the entry point for the rung and log it (minutes, tool calls, model calls and tokens from
   `.nerd/logs`; for a campaign, `nerd campaign journal` and `status`).
5. Review the diff against the landing criteria; keep or revert; run the suite; commit with
   "via nerd fix" and the brief's name.
6. Ledger entry. If the harness blocked it: fix the blocker test-first and rerun the brief.

## Runs

(Newest last.)

| date | rung | brief | entry | outcome | minutes | tool calls | files | commit |
|---|---|---|---|---|---|---|---|---|
| 2026-09-19 02:26 | R1 | R1-1 final-verdict flake | nerd fix | not landed: `working_stop(/read_only_stall)` after 24 rounds, nothing written. Harness blocker: the working window dropped every file but the last one read (fixed by hand, `7856287c`) | 6.1 | 27 | 0 | -- |
| 2026-09-19 02:50 | R1 | R1-1 again, fixed window | nerd fix | not landed, reverted: `/done` over a change that still fails 40 of 40 under load; the runtime edit broke tool-use/result pairing; its test edit (an assertion drop) was refused and never redone. The failure is load-dependent and codeNERD's tools cannot reproduce it -- a poor rung-1 probe; fixed by hand for R0 | 15.6 | 46 | 2 | reverted |
| 2026-09-19 03:09 | R1 | R1-2 check-mangle in kernel context | nerd fix | not landed, reverted: the corpus passes (135/135) and real errors still fail, but `go vet` unreachable code, an unused function, every corpus error misattributed to every file, and no test. Its `run_build` replaced the running nerd.exe (became R1-3) | 25.3 | 108 | 2 | reverted |
| 2026-09-19 03:38 | R1 | R1-2, same brief | campaign `--type remediation` | not landed: timed out at 60 min in phase 3 of 4 with the tree not building. Plan targets guessed before research, modify retyped to create, a retry without its reason, a rollback scoped to declared targets | 60.1 | -- | 1 + a doc | reverted |
| 2026-09-19 04:41 | R1 | R1-3 run_build leaves artifacts | nerd fix | **landed**: fix + a test that fails before and passes after; found and fixed its own regression from a full-suite run | 23.9 | 52 | 2 | `df4a3063` |
| 2026-09-19 06:37 | R1 | R1-2 again (R1-4), forcing-gate binary | nerd fix | not landed, reverted: every gate held -- build, tests, the new coverage round (converged in two attempts), vet -- then the removed-tests guard refused the turn: a whole-file `write_file` of the test file dropped three existing tests that still pass against the new code. The guard was the one gate with no round (fixed by hand, `279b73fe`). The fix: corpus 135/135 and rule errors attributed right, but one malformed Decl cascades into 84 misattributed errors | 14.2 | 63 | 2 | reverted |
