# 05 — The elite-harness ladder

Steve's goal (2026-09-19): codeNERD dogfoods itself until the codebase is error free, and climbs
a ladder of milestones that would let it be called an elite coding harness: 1, 2, 5, 10 and 50
file edits from a single prompt on a real problem or optimisation quest; a rewrite of every doc in
`Docs/architecture` that codeNERD does itself, to a standard its reviewer is impressed by; and
recursive, incremental improvement of the whole codebase.

This file is the program of record: the rungs, what counts as passing one, the error-free gates,
the protocol for a run, and the status of each rung with the run that moved it.

## Status

- last updated: 2026-09-19 02:40 (gate baseline measured on `f26140e1`)
- current rung: R1 (single-file landings happen, not yet three in a row)
- open: the first R1 run (brief R1-1, the final-verdict flake); G6 needs a toolchain download (asked)

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
5. **The file count is the fix's, not the noise's.** Files edited to fix the problem, tests
   included, count; generated files and formatting churn do not.

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
| G3 tests | `go test ./...`, three consecutive green runs (no flakes) | 88 of 89 on `f26140e1`: `TestRunToolLoop_ReservesTimeForFinalVerdict` fails 1 in ~6 runs alone ("compile working context: context deadline exceeded") |
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

(Newest last. Each line: date, rung, brief, outcome, minutes, calls, files, commit or revert.)
