# 09 — Verification Gates: post-edit checks

> Last verified: 2026-08-08 — sources: `internal/session/build_verify.go`, `internal/session/test_verify.go`, `internal/session/coverage_profile.go`, `internal/session/lsp_diagnostics.go`, `internal/session/critic.go`, `internal/session/executor.go`, `internal/session/executor_tools.go`

## 1. Overview — where the gates run

Post-edit rounds are selected by `turn_next_round` in
`internal/core/defaults/policy/turn_rounds.mg`, not a fixed Go schedule.

### Fix verification contract

During `/fix`, the working loop uses covering tests and edited-package tests;
repository-wide test, build and vet requests are withheld until closure.
The final whole-suite gate runs only after all repair/review rounds finish and
current narrow tests pass. A completed whole-suite measurement is reused at the
same workspace snapshot, including failures; a changed covered input requires
fresh narrow checks before another final run. Missing, canceled or stale
measurements never become a passing gate. Coverage, failure attribution and
caller-supplied acceptance obligations remain independent requirements.

Narrow evidence also identifies the exact written-path set and baseline
preimages: adding an already-existing tagged test to that set must run its
obligation even when workspace bytes are unchanged. Closure checks these current
narrow obligations before reusing a final-suite measurement; obligation-only
changes do not authorize another unchanged whole-suite process.

---

## 2. Gate 1 — Build

| Item | Fact |
|------|------|
| **Purpose** | Prove the workspace still compiles after the turn's edits. A turn that writes non-compiling Go and reports success is the false-success this gate exists to prevent. |
| **Primary function** | `func (e *Executor) verifyAndRepairBuild(ctx, trp, systemPrompt, history, current, toolDefs, cfg, result) (*LLMToolResponse, []string, error)` in `internal/session/build_verify.go` |
| **Inner checker** | `func verifyBuild(ctx, workspace, userCfg) BuildVerification` — runs `go build ./...` with `build.GetBuildEnv` so `CGO_CFLAGS` (e.g. `-I<workspace>/sqlite_headers`) is inherited |
| **Helpers** | `func touchedGoFiles(paths []string) bool`, `func (e *Executor) workspaceForVerification() string` (falls back to `config.FindWorkspaceRoot` when `ExecutorConfig.WorkspaceRoot` is empty), `func buildRepairPrompt(compilerOutput string) string` |
| **Outcome type** | `type BuildVerification struct { Ran bool; OK bool; Output string; Duration time.Duration; Outcome VerifyOutcome; Command []string; Reason string }` — `Outcome` (`passed`/`failed`/`skipped`/`indeterminate`/`canceled`) is authoritative; `Ran`/`OK` are derived compatibility |
| **Constants** | `buildVerifyTimeout = 4 * time.Minute` (a var, so tests can shrink it; cold builds take tens of seconds, hence the generous ceiling). Compiler output goes back whole — there is no output cap. |
| **Can fail a turn** | **Yes** |

**When it fails:**

1. `verifyAndRepairBuild` checks a coherent config snapshot's `VerifyBuildAfterEdits`; if false, returns immediately (gate disabled).
2. Runs `verifyBuild` and branches on `Verdict()`: `Passed`/`Skipped` return no repair (nothing to fix or nothing to check); `Canceled` aborts the turn; `Indeterminate` completes unverified with no error (a timeout is not evidence of breakage); only `Failed` proceeds to repair.
3. Otherwise logs `Edits broke the build; giving the model one repair round with the compiler output`.
4. If `trp == nil` (client cannot accept tool results), returns `fmt.Errorf("edits broke the build and no repair is possible...")` — turn fails without a repair attempt.
5. Appends `buildRepairPrompt(verification.Output)` as a `user` message to `history` and calls `trp.CompleteWithToolResults`.
6. Executes any repair `ToolCalls` via `e.executeToolBatch` (repair errors are collected but do not short-circuit).
7. Re-runs `verifyBuild` and branches on the recheck verdict: `Passed` clears the failure, stores the fresh pass, and returns the repaired response; `Failed` stores the fresh failure and returns `fmt.Errorf("edits broke the build and the repair round did not fix it...")` — **turn fails**; `Canceled` aborts the turn; `Indeterminate`/`Skipped` retains the ORIGINAL failure on the result and completes unverified with no error — `closeChangeEvidence` then arbitrates the final workspace with a fresh check, and only its affirmative pass clears the retained failure.

Timeout handling: if the run exhausts `buildVerifyTimeout`, `verifyBuild` returns `Ran: true, OK: false, Outcome: indeterminate` with whatever partial output the compiler had printed — never a pass, and never overwriting a known failure.

---

## 3. Gate 2 — Test

| Item | Fact |
|------|------|
| **Purpose** | Prove the tests for exactly the packages the turn touched still pass. Compiling is a low bar; this is the second false-success (green build, never executed). Also surfaces the weak signal "production Go written with no test alongside it" as a warning. |
| **Primary function** | `func (e *Executor) verifyAndRepairTests(ctx, trp, systemPrompt, history, toolDefs, cfg, result) (*LLMToolResponse, []string, error)` in `internal/session/build_verify.go` (defined there, documented as `test_verify.go` gate) |
| **Inner checkers** | `func verifyTests(ctx, workspace, packages, extraArgs...) TestVerification` and `func verifyTestsWithCoverage(ctx, workspace, packages, writtenPaths) (TestVerification, []UncoveredBlock)` in `internal/session/test_verify.go` / `internal/session/coverage_profile.go` |
| **Helpers** | `func packagesForPaths(paths []string) []string`, `func untestedGoFiles(paths []string) []string`, `func untestedWithoutCoverageOnDisk(workspace string, paths []string) []string`, `func packageHasTestFile(dir string) bool`, `func DeduplicatePreservingOrder`, `func TrimGoExtension`, `func testRepairPrompt(testOutput string) string` |
| **Outcome type** | `type TestVerification struct { Ran bool; OK bool; Output string; Duration time.Duration; Outcome VerifyOutcome; Command []string; Reason string }` — same `Outcome` authority as the build gate |
| **Constants** | `testVerifyTimeout = 4 * time.Minute` (a var, so tests can shrink it). Test output goes back whole — there is no output cap. |
| **Can fail a turn** | **Yes** |

**When it fails:**

1. Checks a coherent config snapshot's `VerifyTestsAfterEdits`; if false, returns.
2. Guard: `SuccessfulWriteTools == 0` or `!touchedGoFiles(...)` → skip.
3. Computes `packages := packagesForPaths(result.WrittenPaths)` (workspace-relative `.go` paths → `go test` package patterns like `./internal/session`, deduplicated, sorted).
4. Calls `untestedWithoutCoverageOnDisk(workspace, result.WrittenPaths)`; if non-empty, sets `result.UntestedPaths` and logs a warning. This is **warning only, never a failure** — editing a long-tested file without rewriting its test file is legitimate.
5. Calls `verifyTestsWithCoverage` (single `go test -covermode=set -coverprofile=<tmp>` invocation that yields both pass/fail and uncovered blocks — see Gate 3).
6. Branches on the verdict: `Passed`/`Skipped` return (coverage blocks, if any, are still stored on `result.UncoveredBlocks` even on success); `Canceled` aborts the turn; `Indeterminate` completes unverified with no error; only `Failed` proceeds to repair.
7. Otherwise logs `Edits broke the tests; giving the model one repair round with the test output`, appends `testRepairPrompt(verification.Output)`, calls `trp.CompleteWithToolResults`, executes repair tool calls.
8. Re-checks build first (`verifyBuild`): if the test repair broke the build, returns build error and fails the turn.
9. Re-runs `verifyTests(ctx, workspace, packagesForPaths(result.WrittenPaths))` and branches like the build gate: `Passed` clears and stores; `Failed` stores and fails the turn with `fmt.Errorf("edits broke the tests and the repair round did not fix them...")`; `Canceled` aborts; `Indeterminate`/`Skipped` retains the ORIGINAL failure and completes unverified for `closeChangeEvidence` to arbitrate.

`verifyTests` timeout handling mirrors the build gate: budget exhaustion → `Ran: true, OK: false, Outcome: indeterminate` with partial output — never a pass, never a failure, never overwriting a known failure.

---

## 4. Gate 3 — Coverage

| Item | Fact |
|------|------|
| **Purpose** | Fine-grained signal `go test` cannot give: which blocks in files the turn actually wrote were never executed, even when tests are green. An added function with an added test file that never calls it is green but uncovered. |
| **Primary functions** | `verifyTestsWithCoverage(ctx, workspace, packages, writtenPaths)` and `parseCoverBlocks(r, workspace, writtenFiles)` in `internal/session/coverage_profile.go`; `newCoverSourceResolver` in `internal/session/coverage_source.go` |
| **Helpers** | `func NormalizeCoverPath(p string) string` (forward slashes, strip leading `./`), `func summarizeUncovered(blocks []UncoveredBlock) string` (caps at 8 entries) |
| **Outcome type** | `UncoveredBlock` carries import-qualified `File`, resolved workspace `Source`, statement range, `NumStmts`, and execution `Count` |
| **Execution bound** | Shares the test runner's context and verification bound; coverage does not launch a second timed test process |
| **Completion effect** | A passing test verdict stands, but a changed element without execution evidence can withhold `/done` and leave the turn `/unverified` |

**What it does:**

1. The executor's test gate calls `verifyTestsWithCoverage` instead of two separate invocations. That function creates a temp file, runs `verifyTests(ctx, workspace, packages, "-covermode=set", "-coverprofile="+path)`, and always returns the `TestVerification` verdict.
2. If `!verification.Ran`, coverage is `nil` (no signal).
3. Otherwise it opens the temp profile and calls `parseCoverBlocks(f, workspace, writtenPaths)`. The parser requires a `mode: ...` first line, then expects lines of form `file.go:start.col,end.col numStmts count`; it returns an error on malformed input. It retains both executed and unexecuted statement blocks belonging to written files, carrying each resolved source identity.
4. If the profile cannot be opened or parsed, the error is logged at `Warn`/`Debug` and `verifyTestsWithCoverage` returns `(verification, nil)` — **the test verdict stands; execution evidence is unknown**.
5. `elementUncoveredRefs` records which changed elements were measured and which had no executed statement block. `narrowToChangedLines` then retains unexecuted changed-line blocks for file-level debt. The executed blocks must survive until element measurement; dropping them earlier would discard the proof that a function ran.
6. The executor asserts turn-scoped evidence for the Mangle witness rules. A green test run can meet a changed element's witness only when its own statement blocks executed; another file's block cannot satisfy that obligation.

**Module-source identity contract:** a profile file belongs to the written path
whose nearest module path, joined to that file's module-relative path, matches
exactly. Standard, quoted, and parenthesized Go module declarations must produce
the same identity; comments cannot become part of it. `modulePath` reads the
complete manifest using `modfile.ParseLax`, also used by retrieval, so unrelated
future directives are tolerated and malformed or duplicate module declarations
cannot invent an exact identity. Missing, unreadable, or malformed manifests keep
the existing whole-component suffix fallback, choosing the most specific written
path. A nested module owns its own files; parent and foreign module identities
cannot claim them.

Regression evidence: `TestCoverageModulePath` checks declaration syntax and
malformed inputs; `TestCoverSource_ParenthesizedModule` checks exact and nested
source identity. `TestWitnessStep6_ParenthesizedModuleExecutionEvidence` runs real
Go tests with coverage through turn evidence and Mangle verdict derivation:
executing every changed element permits `/done`, while a green run missing one
changed element remains `/unverified` and names only that element.

---

## 5. Gate 4 — Adversarial critic (with gopls grounding)

| Item | Fact |
|------|------|
| **Purpose** | Catch defects the compiler and test runner are silent about: logic errors, correctness bugs, security issues, data races, contract violations. Backed by an LLM reviewer, so it is inherently fallible. |
| **Primary function** | `func (e *Executor) verifyAndUpliftWithCritic(ctx, trp, systemPrompt, history, toolDefs, cfg, result) (*LLMToolResponse, []string)` in `internal/session/build_verify.go` |
| **Pure helpers** | `func buildCriticPrompt(writtenFiles map[string]string, removals map[string]string, uncoveredSummary, request string) string`, `func changedSpans(before, after string) []criticSpan`, `func criticFileView(path, content string, changed []criticSpan) (string, []criticSpan)`, `func findingsOnChange(findings []CriticFinding, windows map[string][]criticSpan) (kept, dropped []CriticFinding)`, `func parseCriticFindings(response string) []CriticFinding`, `func findingsWorthUplift(findings []CriticFinding) []CriticFinding`, `func CriticSeverityRank(sev string) int`, `func readWrittenFilesForReview(workspace string, writtenPaths []string) map[string]string`, `func formatUpliftPrompt(findings []CriticFinding) string` in `internal/session/critic.go` |
| **Grounding helpers** | `func goplsDiagnostics(ctx, workspace, writtenPaths) string` and `func keepDiagnosticLines(raw string) string` (Go), and `func lspDiagnostics(ctx, workspace, writtenPaths) string` (Python via `pyright-langserver`, TypeScript/JavaScript via `typescript-language-server`, over `internal/world/lsp`'s client), all in `internal/session/lsp_diagnostics.go` |
| **Outcome type** | `type CriticFinding struct { File string; Line int; Severity string; Claim string }` — Severity normalised to `high`/`medium`/`low` |
| **Constants** | `criticMaxFileBytes = 24000`, `criticMaxFiles = 6`, `goplsTimeout = 90 * time.Second`, `goplsMaxOutput = 4000`, `goplsMaxFiles = 8`, `criticTimeout = 3 * time.Minute`, `criticUpliftTimeout = 5 * time.Minute`, `criticSystemPrompt` (rigorous adversarial reviewer prompt) |
| **Can fail a turn** | **No — advisory only, by design** (`ExecutorConfig.CriticReviewAfterEdits` doc: "this gate can never fail a turn") |

**When it finds something:**

1. `verifyAndUpliftWithCritic` checks a coherent config snapshot's `CriticReviewAfterEdits`; if false, returns immediately.
2. Guard: the round is owed for a Go write or any other non-doc write (`turn_round_owed(Turn, /critic)` in `policy/turn_rounds.mg`); it returns without a call when `readWrittenFilesForReview` returns empty (no readable non-test source files, caps applied).
3. `readWrittenFilesForReview` loads at most `criticMaxFiles` written non-test source files, whole (`reviewableSource`: Go, Mangle, Python, TypeScript/JavaScript, Rust, Java, Kotlin, Ruby, PHP, C#, C/C++, Swift, Scala). Tests are excluded by each language's convention (`_test.go`, `test_*.py`, `*.test.ts`, `*.spec.*`, `tests/` directories, ...). Until 2026-09-25 only Go was reviewed.

    Each file is then reviewed as the change the turn made to it:
    - `changedSpans` diffs it against its preimage (`PreWriteContents`); a file with none is new, and all of it changed. A file written back as it was is not reviewed.
    - `reviewWindows` widens each changed span to its enclosing code element and pads it.
    - `criticFileView` shows a file within `criticMaxFileBytes` whole, and a larger one as its changed windows only. Either way the lines are numbered.

    Until 2026-09-26 every file was cut at its first `criticMaxFileBytes` bytes, so an edit past that point was reviewed by a critic that could not see it.
4. Optionally collects `goplsDiagnostics` — runs `gopls check` on at most `goplsMaxFiles` written files, filtered through `keepDiagnosticLines` (regex `^.+:\d+:\d+(-\d+)?: .+$`) to drop operational chatter like missing `%AppData%`. Absent `gopls` or timeout returns `""` (silence, not an error). Output is capped at `goplsMaxOutput` and handed to the critic prompt as grounding; the critic is explicitly told to treat tool output as evidence.
5. Builds the prompt with `buildCriticPrompt`: files in fenced blocks, sorted keys, and the request (the working loop's task), with an instruction to judge the change against it. Then an optional uncovered summary, instructions to emit `FINDING file.go:123 severity: claim` or `NO FINDINGS`, and to treat inventing a finding as worse than finding nothing). The uncovered-summary section instructs the reviewer to report uncovered blocks worth testing at `medium` severity.
6. Calls the critic model via `e.criticClient().CompleteWithSystem` bounded by `criticTimeout` (3m). If the call times out or errors, the gate is abandoned and the turn proceeds without a review — a failed review is "a missing opinion, not a hard failure".
7. Parses with `parseCriticFindings`: `NO FINDINGS` on any line → `nil`; otherwise regex `^FINDING\s+(\S+):(\d+)\s+(\w+):\s*(.+)$` per line, severity must be `high`/`medium`/`low` (case-insensitive), non-matching lines silently skipped.
8. Drops, from the uplift, findings that cite a line outside the changed windows of their file (`findingsOnChange`). They are still recorded. R1-11 reported 3 of its 4 findings in untouched code. Then filters with `findingsWorthUplift` (keeps only `high`/`medium`; `low` is noise). If `nil`, stores `result.CriticFindings` (if any original findings existed) and returns.
9. If worthy findings remain, stores all findings in `result.CriticFindings`, appends `formatUpliftPrompt(worth)` as a `user` message, and runs one advisory `CompleteWithToolResults` bounded by `criticUpliftTimeout` (5m). The uplift prompt requires the model to either fix each finding or state plainly why the finding is wrong — forcing a fix for a hallucinated finding is explicitly forbidden.
10. Executes any uplift tool calls, then re-runs `verifyBuild` and `verifyTests` and stores both re-verdicts (the uplift edits came after the gates' passes, so those passes no longer describe the workspace). A `Failed` re-verdict returns an error and **the turn fails**; a `Passed` one refreshes the stored pass; an `Indeterminate` one invalidates the stale pass to indeterminate with no error (the turn completes unverified); `Canceled` aborts the turn.

    This is the one case where the critic gate can fail a turn, and it is not an exception to "advisory". The critic's *opinion* is advisory: a hallucinated finding must never fail anything. Its *edits* are not privileged — they answer to the compiler and the test runner like any other edit. Acting on a wrong finding and breaking the build is a real break, whoever suggested it.

    > **Provenance:** this step did not exist when this document was first written. The original draft asserted it did, which was a fabricated claim — and checking it exposed a real hole: the uplift round makes edits *after* gates 1 and 2 have already run, so its output was the only code in the loop shipping unverified. The code was changed to match the description rather than the description trimmed to match the code.

`goplsDiagnostics` and `lspDiagnostics` are not separate numbered gates; they are the static-analysis grounding fed into Gate 4. Without the server on `PATH` the critic runs without it and behaviour is identical to before the signal existed. `lspDiagnostics` reports errors and warnings only, bounded by `goplsTimeout` and `goplsMaxFiles`; a server that never publishes for a file costs the rest of the budget, not the turn.

`recheckUplift` re-verifies an uplift with the Go build and tests only when the turn wrote Go (`wroteGo`). A non-Go uplift is measured by the workspace's own test gate, `/test_run`, which the kernel orders after the critic; `go build ./...` in a workspace with no `go.mod` would fail and undo every uplift.

---

## 6. The Outcome convention — only an affirmative pass is a pass

Both `BuildVerification` and `TestVerification` carry an explicit `Outcome` verdict (`passed`/`failed`/`skipped`/`indeterminate`/`canceled`) alongside the older `Ran`/`OK` pair, which stays as derived compatibility (`Ran` = the command executed, `OK` = passed). Gates branch on `Verdict()`, which returns `Outcome` when set and otherwise derives it from `Ran`/`OK` for hand-built structs:

```go
type BuildVerification struct {
    Ran      bool          // true when the command executed, even without a verdict
    OK       bool          // true only when the build actually succeeded
    Output   string
    Duration time.Duration
    Outcome  VerifyOutcome // authoritative: passed, failed, skipped, indeterminate, canceled
    Command  []string      // argv executed, for provenance; nil when nothing ran
    Reason   string        // why, for non-pass outcomes; Output stays empty on skips
}
type TestVerification struct {
    // Same shape as BuildVerification.
}
```

Coverage follows the same discipline with a different shape: `verifyTestsWithCoverage` returns `(verification, nil)` and `uncoveredWrittenCode` / `parseCoverProfile` callers treat `(nil, nil)` as "no signal, not nothing uncovered". The comment in `coverage_profile.go` states it explicitly: "Absence of a profile is 'unknown', never 'covered'".

`Outcome == skipped` (`Ran == false`) occurs when:

- no successful write-mutation touched a `.go` file (`touchedGoFiles` / `SuccessfulWriteTools` guard),
- workspace is empty (no `WorkspaceRoot` and `FindWorkspaceRoot` failed),
- no `go` toolchain on `PATH` (toolchain probe fails),
- verification disabled via `ExecutorConfig` flag,
- no packages to verify (test gate only).

A subprocess that exhausts its budget is NOT a skip: it reports `Ran: true, OK: false, Outcome: indeterminate` with partial output. Operator cancellation reports `Outcome: canceled`. Callers branch on `Verdict()` — only `Passed` is a pass, only `Failed` is a fail, and a known failure is retained until an affirmative pass on the final workspace clears it. Nothing indeterminate is ever reported as success. A gate that reports "verification skipped" while looking enabled is described in `workspaceForVerification`'s comment as the dormant-wiring defect this codebase keeps producing.

The subprocess runs under an independent budget rooted at `Background`, supervised so an expired parent (turn) deadline does not abort it while explicit operator cancellation kills it promptly — `runVerificationCommand` in `internal/session/verify_outcome.go`. The runner, toolchain probe, and budgets are package-level seams (`verifyBuildRunner`, `verifyTestRunner`, `verifyLookPath`, `buildVerifyTimeout`, `testVerifyTimeout`) so timeout / missing-toolchain / cancel tests run in milliseconds without real subprocesses.

---

## 6a. Repair rounds: write boundary and suite recheck (as built 2026-10-28)

Every repair attempt runs under `withRepairWriteGuard` (`internal/session/write_guard.go`), installed per attempt in `repairRound` (`build_verify.go`). The guard admits the turn's pre-gate write set, every file the turn has written since, and any path that does not exist yet; an existing file outside that set is refused with the set named.

The rounds whose obligation is a test -- `coverage`, `pinning` and `removed_tests` (`repairSpec.writesTests`) -- also admit every existing `_test.go` file in the directory of a Go file the turn wrote (`testBesideWrittenGo`). Their prompts send the model to "the package's _test.go files"; before this the guard refused those files and the coverage round could not write the test its own gate demanded. The failure-repair rounds (`build`, `tests`, `vet`, `test_run`) keep the narrow set: editing another test to turn the suite green is what the boundary exists to stop.

The same three rounds ask one question after an attempt: `recheckOwnTests` (`change_gates.go`) runs the turn's own packages again and reads the derived `/test` gate (`testGatePassed`), not the process exit. A failure that also fails before the turn's edits is not charged to the turn by the test gate, so it is not charged by these rounds either. Before this the `removed_tests` round read the raw exit: with the deleted test restored on the first attempt, it told the model three times that 16 pre-existing failures were its edit and gave up at the attempt cap after ~1M input tokens (problems log 2026-10-28). Pinned by `TestRemovedTestsRound_APreExistingFailureDoesNotKeepTheRoundOpen` and `TestRepairWriteGuard_TestRoundsMayExtendThePackagesTests`.

A round's last response becomes the turn's answer only when it carries text (`verifyCompletedToolTurn`, `executor_tools.go`). An attempt ends at its write, usually a response of tool calls with no text; that used to replace the model's own answer, and articulation then logged "Fallback parse: empty response with no Piggyback JSON" for the empty one (problems log 2026-10-28, after the pinning and coverage rounds). Pinned by `TestRepairLoop_AToolOnlyRepairKeepsTheTurnsAnswer`.

The Dreamer's refusal to delete a file under a `nerd.md` `critical:` prefix (`critical_path_missing`) agrees with these rounds: deleting a test file that existed before the turn is what `removed_tests` would then ask the model to undo.

---

## 6b. The `/unwired` round: new code no production path reaches (as built 2026-10-28)

A turn could add a function nothing in production calls and still read verified: a coder added `RepairExistingPromptAtoms` to `internal/init/agents.go`, ran it once from a temporary test, deleted the test and reported done. The `/unwired` round (`round_order` 9, after `/removed_tests`, before `/test_run`; owed by every Go write) measures and pushes back.

- Measure (`internal/session/unwired_measure.go`): the functions and methods the turn added to non-test Go files (the pin gate's `fileUnits`, token-compared against the preimage), as structure-index refs. Per function: `turn_new_func(Turn, Ref, Exposure)` (`/api` when exported from a package with no `internal` segment and not `main`), `turn_new_func_entry` (main in package main, `//go:linkname`, `//go:wasmexport`, cgo `//export`), `turn_new_func_generated` (`ast.IsGenerated`), `turn_func_use(Turn, Ref, From, /production|/test)` from the index's `Uses` with the declaration holding each use, and for a method `turn_method_mentioned` from `NameMentions` (`internal/world/structure_mentions.go`): its name outside declarations, so an interface it satisfies or a call through an interface value counts. Any index deficit asserts `turn_new_func_unobserved`. Each run replaces the previous measurement.
- Decide (`policy/turn_unwired.mg`): `turn_func_reached` is a closure. A production use from code the turn did not add reaches the function; a use from new code reaches it only when that code is reached, exempt or settled; a test reaches nothing. Exempt: `/api`, generated, unobserved. `turn_func_unwired_open` is unreached, not exempt and not settled. Open code derives `has_red_gate` and `turn_missing_evidence(/code_unwired)`, so the turn is not verified.
- Push back (`internal/session/unwired_round.go`): while `turn_unwired_pushback` holds, the model gets the open refs (marked "only tests call it" where `turn_func_test_only`) and three answers: wire it, delete it, or reply `unused <ref>: <reason>`. Jev reads each reason (`learnjudge.JudgeUnusedReason`); a caller that exists today, or a task that asks for the function uncalled, at or above `jev_yes_percent` settles it. No judge or a failed call records `turn_unused_reason_unread`, which settles it as stated and is logged: a failed call is never a "no". An attempt that writes nothing asserts `turn_unwired_idle` and ends the round. A write re-arms the gate rounds; `/unwired` is in `gateRounds`, so a `/finish` write re-arms it. The push-back's words are appended to the turn's answer. With no continuation channel the open code stands, and the verdict says so.
- Rulings: functions and methods only (an unused type or constant is out of scope); a method is reached by name (conservative: a common name such as `String` always counts); a reflection-only or linkname caller is the model's reason to give; a function only a test calls is unwired (it belongs in the test file).
- Tests: `TestUnwiredPolicy_TruthTable`, `TestUnwiredMeasure_NewFunctionsAndTheirUses`, `TestUnwiredRound_UnreachedCodeIsPushedBack` (production chain with real `go test`; the two reason cases use live Jev), `TestStructureIndexNameMentions`, and the schedule tests in `post_edit_rounds_test.go`.

---

## 7. Configuration and result fields the gates use

### 7.1 `ExecutorConfig` — `internal/session/executor.go`

```go
type ExecutorConfig struct {
    MaxToolCalls           int
    MaxToolIterations      int
    ToolTimeout            time.Duration
    FinalAnswerReserve     time.Duration // conclusion window before the outer deadline; default 5m
    EnableSafetyGate       bool
    TokenBudget            int
    VerifyBuildAfterEdits  bool   // gate 1 — default true
    VerifyTestsAfterEdits  bool   // gate 2 — default true
    CriticReviewAfterEdits bool   // gate 4 — default true, never fails a turn
    WorkspaceRoot          string // verification dir; empty → auto-discover via workspaceForVerification
}
func DefaultExecutorConfig() ExecutorConfig // VerifyBuildAfterEdits=true, VerifyTestsAfterEdits=true, CriticReviewAfterEdits=true
```

- `VerifyBuildAfterEdits` controls `verifyAndRepairBuild`.
- `VerifyTestsAfterEdits` controls `verifyAndRepairTests`.
- `CriticReviewAfterEdits` controls `verifyAndUpliftWithCritic` (advisory only).
- `WorkspaceRoot` is the directory `verifyBuild` / `verifyTests` / `goplsDiagnostics` / `readWrittenFilesForReview` run in. When empty the executor resolves it itself rather than silently skipping verification.

Coverage has no separate config flag; it piggybacks on `VerifyTestsAfterEdits` via `verifyTestsWithCoverage`'s single `go test -coverprofile` invocation.

### 7.2 `ExecutionResult` — `internal/session/executor.go`

```go
type ExecutionResult struct {
    Response            string
    Intent              perception.Intent
    ToolCallsExecuted   int
    SuccessfulToolCalls int              // successful execution, independent of tool kind
    SuccessfulWriteTools int              // gate guard: >0 required
    WrittenPaths        []string         // gate input: which packages/files to check
    UntestedPaths       []string         // gate 2 warning: production Go with no test alongside it
    UncoveredBlocks     []UncoveredBlock // gate 3 signal: blocks in written files with count==0
    CriticFindings      []CriticFinding  // gate 4 signal: advisory, low severity included but not uplifted
    StaticDiagnostics   string           // gate 4 grounding: filtered gopls output, empty if gopls absent
    Duration            time.Duration
    Error               error            // set when gate 1 or 2 fails after its repair round
}
```

- `SuccessfulWriteTools` / `WrittenPaths` are written by `executeToolBatch` (only for `isWriteMutationTool` calls) and read by all three gate entry points to decide whether to run at all.
- `UntestedPaths` is set by `verifyAndRepairTests` via `untestedWithoutCoverageOnDisk` as a warning.
- `UncoveredBlocks` is set by `verifyAndRepairTests` via `verifyTestsWithCoverage`/`parseCoverProfile` and summarised into Gate 4's prompt.
- `CriticFindings` and `StaticDiagnostics` are set by `verifyAndUpliftWithCritic` / `goplsDiagnostics`; they are advisory and never set `Error`.

---

## 8. Verification order and references

Execution order inside `runToolLoop` (verified in `internal/session/executor_tools.go`):

1. Build — cheapest, most precise error. Test output on uncompilable packages wraps compiler errors in test noise, so build runs first.
2. Test + coverage — single `go test -coverprofile` call for both signals; `UncoveredBlocks` collected regardless of pass/fail; `UntestedPaths` warned.
3. Critic — receives file contents, `summarizeUncovered` output, and `goplsDiagnostics` as evidence; high/medium findings get one `formatUpliftPrompt` round, bounded by `criticTimeout` / `criticUpliftTimeout`.

**Uncertainty note:** This document only states facts verified by reading the listed source files. Behaviour described as "verified live on 2026-08-08" in source comments is reported as the source reports it, not independently re-measured here. Any detail not present in the five source files plus `executor.go`/`executor_tools.go`/`gate_names.go` is intentionally omitted.

## References

- `internal/session/build_verify.go` — `BuildVerification`, `verifyBuild`, `verifyAndRepairBuild`, `verifyAndRepairTests`, `verifyAndUpliftWithCritic`, `criticTimeout`, `criticUpliftTimeout`, `buildRepairPrompt`, `testRepairPrompt`
- `internal/session/verify_outcome.go` — `VerifyOutcome`, `runVerificationCommand`, `verifyBuildRunner`, `verifyTestRunner`, `verifyLookPath`, `buildVerifyTimeout`, `testVerifyTimeout`
- `internal/session/test_verify.go` — `TestVerification`, `verifyTests`, `packagesForPaths`, `untestedGoFiles`, `untestedWithoutCoverageOnDisk`, `DeduplicatePreservingOrder`, `TrimGoExtension`
- `internal/session/coverage_profile.go` — `UncoveredBlock`, `parseCoverProfile`, `verifyTestsWithCoverage`, `uncoveredWrittenCode`, `NormalizeCoverPath`, `summarizeUncovered`
- `internal/session/lsp_diagnostics.go` — `goplsDiagnostics`, `keepDiagnosticLines`, `diagnosticLineRe`
- `internal/session/critic.go` — `CriticFinding`, `buildCriticPrompt`, `parseCriticFindings`, `findingsWorthUplift`, `CriticSeverityRank`, `readWrittenFilesForReview`, `formatUpliftPrompt`
- `internal/session/executor.go` — `ExecutorConfig`, `ExecutionResult`, `DefaultExecutorConfig`
- `internal/session/executor_tools.go` — `runToolLoop` gate sequencing, `executeToolBatch` population of `SuccessfulWriteTools`/`WrittenPaths`

## Declared check exit semantics

Campaign check receipts keep their observed exit and invocation-specific allowed
exits. Host facts project these with the turn sequence. Mangle derives membership,
then applies the existing latest-after-final-write rule for `/check`. Missing exit
lists mean zero only; execution errors cannot pass. Live check progress reads the
same receipt judgment. Check passes never satisfy test execution or coverage gates.
