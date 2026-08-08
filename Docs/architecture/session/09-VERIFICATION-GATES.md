# 09 — Verification Gates: post-edit checks

> Last verified: 2026-08-08 — sources: `internal/session/build_verify.go`, `internal/session/test_verify.go`, `internal/session/coverage_profile.go`, `internal/session/lsp_diagnostics.go`, `internal/session/critic.go`, `internal/session/executor.go`, `internal/session/executor_tools.go`

## 1. Overview — where the gates run

Post-edit verification runs inside `Executor.runToolLoop` (`internal/session/executor_tools.go`) after the model returns a final answer with no further tool calls. The ordering is fixed and load-bearing:

```
runToolLoop final answer with 0 tool calls
  → verifyAndRepairBuild   (gate 1: build)
  → verifyAndRepairTests   (gate 2: test + coverage side-signal)
  → verifyAndUpliftWithCritic (gate 3+4: coverage already collected, critic advisory)
  → return
```

Each gate is skipped entirely when no write-mutation succeeded or no `.go` file was touched. The `ExecutionResult` fields `SuccessfulWriteTools` and `WrittenPaths` (populated in `executeToolBatch`) are the shared guard for this check.

```
touchedGoFiles(paths) == false  →  skip
SuccessfulWriteTools == 0       →  skip
```

Gates 1 and 2 can fail a turn. Gates 3 (coverage) and 4 (critic) are advisory and never fail a turn on their own.

---

## 2. Gate 1 — Build

| Item | Fact |
|------|------|
| **Purpose** | Prove the workspace still compiles after the turn's edits. A turn that writes non-compiling Go and reports success is the false-success this gate exists to prevent. |
| **Primary function** | `func (e *Executor) verifyAndRepairBuild(ctx, trp, systemPrompt, history, current, toolDefs, cfg, result) (*LLMToolResponse, []string, error)` in `internal/session/build_verify.go` |
| **Inner checker** | `func verifyBuild(ctx, workspace, userCfg) BuildVerification` — runs `go build ./...` with `build.GetBuildEnv` so `CGO_CFLAGS` (e.g. `-I<workspace>/sqlite_headers`) is inherited |
| **Helpers** | `func touchedGoFiles(paths []string) bool`, `func (e *Executor) workspaceForVerification() string` (falls back to `config.FindWorkspaceRoot` when `ExecutorConfig.WorkspaceRoot` is empty), `func buildRepairPrompt(compilerOutput string) string` |
| **Outcome type** | `type BuildVerification struct { Ran bool; OK bool; Output string; Duration time.Duration }` |
| **Constants** | `buildVerifyTimeout = 4 * time.Minute`, `buildVerifyMaxOutput = 6000` (truncated output is appended with `... (compiler output truncated)`) |
| **Can fail a turn** | **Yes** |

**When it fails:**

1. `verifyAndRepairBuild` checks `e.config.VerifyBuildAfterEdits`; if false, returns immediately (gate disabled).
2. Runs `verifyBuild`. If `!Ran || OK`, returns no repair (nothing to fix or nothing to check).
3. Otherwise logs `Edits broke the build; giving the model one repair round with the compiler output`.
4. If `trp == nil` (client cannot accept tool results), returns `fmt.Errorf("edits broke the build and no repair is possible...")` — turn fails without a repair attempt.
5. Appends `buildRepairPrompt(verification.Output)` as a `user` message to `history` and calls `trp.CompleteWithToolResults`.
6. Executes any repair `ToolCalls` via `e.executeToolBatch` (repair errors are collected but do not short-circuit).
7. Re-runs `verifyBuild`. If `recheck.Ran && !recheck.OK`, returns `fmt.Errorf("edits broke the build and the repair round did not fix it...")` — **turn fails**. Otherwise returns the repaired response.

Timeout handling: if `buildCtx.Err() != nil` after `CombinedOutput`, `verifyBuild` returns `Ran: false` (treated as not-run, never as pass or fail) with a warning log.

---

## 3. Gate 2 — Test

| Item | Fact |
|------|------|
| **Purpose** | Prove the tests for exactly the packages the turn touched still pass. Compiling is a low bar; this is the second false-success (green build, never executed). Also surfaces the weak signal "production Go written with no test alongside it" as a warning. |
| **Primary function** | `func (e *Executor) verifyAndRepairTests(ctx, trp, systemPrompt, history, toolDefs, cfg, result) (*LLMToolResponse, []string, error)` in `internal/session/build_verify.go` (defined there, documented as `test_verify.go` gate) |
| **Inner checkers** | `func verifyTests(ctx, workspace, packages, extraArgs...) TestVerification` and `func verifyTestsWithCoverage(ctx, workspace, packages, writtenPaths) (TestVerification, []UncoveredBlock)` in `internal/session/test_verify.go` / `internal/session/coverage_profile.go` |
| **Helpers** | `func packagesForPaths(paths []string) []string`, `func untestedGoFiles(paths []string) []string`, `func untestedWithoutCoverageOnDisk(workspace string, paths []string) []string`, `func packageHasTestFile(dir string) bool`, `func DeduplicatePreservingOrder`, `func TrimGoExtension`, `func testRepairPrompt(testOutput string) string` |
| **Outcome type** | `type TestVerification struct { Ran bool; OK bool; Output string; Duration time.Duration }` |
| **Constants** | `testVerifyTimeout = 4 * time.Minute`, `testVerifyMaxOutput = 6000` |
| **Can fail a turn** | **Yes** |

**When it fails:**

1. Checks `e.config.VerifyTestsAfterEdits`; if false, returns.
2. Guard: `SuccessfulWriteTools == 0` or `!touchedGoFiles(...)` → skip.
3. Computes `packages := packagesForPaths(result.WrittenPaths)` (workspace-relative `.go` paths → `go test` package patterns like `./internal/session`, deduplicated, sorted).
4. Calls `untestedWithoutCoverageOnDisk(workspace, result.WrittenPaths)`; if non-empty, sets `result.UntestedPaths` and logs a warning. This is **warning only, never a failure** — editing a long-tested file without rewriting its test file is legitimate.
5. Calls `verifyTestsWithCoverage` (single `go test -covermode=set -coverprofile=<tmp>` invocation that yields both pass/fail and uncovered blocks — see Gate 3).
6. If `!Ran || OK`, returns (coverage blocks, if any, are still stored on `result.UncoveredBlocks` even on success).
7. Otherwise logs `Edits broke the tests; giving the model one repair round with the test output`, appends `testRepairPrompt(verification.Output)`, calls `trp.CompleteWithToolResults`, executes repair tool calls.
8. Re-checks build first (`verifyBuild`): if the test repair broke the build, returns build error and fails the turn.
9. Re-runs `verifyTests(ctx, workspace, packagesForPaths(result.WrittenPaths))`. If `recheck.Ran && !recheck.OK`, returns `fmt.Errorf("edits broke the tests and the repair round did not fix them...")` — **turn fails**.

`verifyTests` timeout handling mirrors the build gate: `buildCtx.Err() != nil` → `Ran: false`, not a failure.

---

## 4. Gate 3 — Coverage

| Item | Fact |
|------|------|
| **Purpose** | Fine-grained signal `go test` cannot give: which blocks in files the turn actually wrote were never executed, even when tests are green. An added function with an added test file that never calls it is green but uncovered. |
| **Primary functions** | `func verifyTestsWithCoverage(ctx, workspace, packages, writtenPaths) (TestVerification, []UncoveredBlock)` and `func uncoveredWrittenCode(ctx, workspace, packages, writtenPaths) ([]UncoveredBlock, error)` in `internal/session/coverage_profile.go`; `func parseCoverProfile(r io.Reader, writtenFiles []string) ([]UncoveredBlock, error)` (pure predicate) |
| **Helpers** | `func NormalizeCoverPath(p string) string` (forward slashes, strip leading `./`), `func summarizeUncovered(blocks []UncoveredBlock) string` (caps at 8 entries) |
| **Outcome type** | `type UncoveredBlock struct { File string; StartLine int; EndLine int; NumStmts int }` — `File` is the import-qualified path as it appears in the profile |
| **Constants** | `coverVerifyTimeout = 4 * time.Minute` |
| **Can fail a turn** | **No — advisory only** |

**What it does:**

1. The executor's test gate calls `verifyTestsWithCoverage` instead of two separate invocations. That function creates a temp file, runs `verifyTests(ctx, workspace, packages, "-covermode=set", "-coverprofile="+path)`, and always returns the `TestVerification` verdict.
2. If `!verification.Ran`, coverage is `nil` (no signal).
3. Otherwise it opens the temp profile and calls `parseCoverProfile(f, writtenPaths)`. `parseCoverProfile` requires a `mode: ...` first line, then expects lines of form `file.go:start.col,end.col numStmts count`; it returns an error on any malformed line. Only blocks with `count == 0` whose slash-normalised `File` has a suffix matching a `NormalizeCoverPath`-normalised entry in `writtenFiles` are kept.
4. If the profile cannot be opened or parsed, the error is logged at `Warn`/`Debug` and `verifyTestsWithCoverage` returns `(verification, nil)` — **the test verdict stands, coverage is simply empty**.
5. `uncoveredWrittenCode` is a standalone entry point with the same suffix-matching logic, used independently. It runs `go test -covermode=set -coverprofile=<tmp>`, handles timeout as `nil, nil`, handles missing `go` toolchain as `nil, nil`, and returns `nil, nil` for empty workspace or empty package list (unknown, not covered).
6. When blocks are found, the test gate stores them in `result.UncoveredBlocks`, logs `Turn wrote N block(s) of Go that no test executes: <summary>`, and passes `summarizeUncovered(uncovered)` into Gate 4's prompt as `uncoveredSummary`.

Coverage never turns a passing turn into a failing one.

---

## 5. Gate 4 — Adversarial critic (with gopls grounding)

| Item | Fact |
|------|------|
| **Purpose** | Catch defects the compiler and test runner are silent about: logic errors, correctness bugs, security issues, data races, contract violations. Backed by an LLM reviewer, so it is inherently fallible. |
| **Primary function** | `func (e *Executor) verifyAndUpliftWithCritic(ctx, trp, systemPrompt, history, toolDefs, cfg, result) (*LLMToolResponse, []string)` in `internal/session/build_verify.go` |
| **Pure helpers** | `func buildCriticPrompt(writtenFiles map[string]string, uncoveredSummary string) string`, `func parseCriticFindings(response string) []CriticFinding`, `func findingsWorthUplift(findings []CriticFinding) []CriticFinding`, `func CriticSeverityRank(sev string) int`, `func readWrittenFilesForReview(workspace string, writtenPaths []string) map[string]string`, `func formatUpliftPrompt(findings []CriticFinding) string` in `internal/session/critic.go` |
| **Grounding helper** | `func goplsDiagnostics(ctx, workspace, writtenPaths) string` and `func keepDiagnosticLines(raw string) string` in `internal/session/lsp_diagnostics.go` |
| **Outcome type** | `type CriticFinding struct { File string; Line int; Severity string; Claim string }` — Severity normalised to `high`/`medium`/`low` |
| **Constants** | `criticMaxFileBytes = 24000`, `criticMaxFiles = 6`, `goplsTimeout = 90 * time.Second`, `goplsMaxOutput = 4000`, `goplsMaxFiles = 8`, `criticTimeout = 3 * time.Minute`, `criticUpliftTimeout = 5 * time.Minute`, `criticSystemPrompt` (rigorous adversarial reviewer prompt) |
| **Can fail a turn** | **No — advisory only, by design** (`ExecutorConfig.CriticReviewAfterEdits` doc: "this gate can never fail a turn") |

**When it finds something:**

1. `verifyAndUpliftWithCritic` checks `e.config.CriticReviewAfterEdits`; if false, returns immediately.
2. Guard: same `SuccessfulWriteTools` / `touchedGoFiles` checks as above; also skips when `readWrittenFilesForReview` returns empty (no readable non-test `.go` files, caps applied).
3. `readWrittenFilesForReview` loads at most `criticMaxFiles` written non-test `.go` files, newest-written first, truncating any file over `criticMaxFileBytes` with `// ... (truncated for review)`. Test files (`_test.go`) are intentionally excluded.
4. Optionally collects `goplsDiagnostics` — runs `gopls check` on at most `goplsMaxFiles` written files, filtered through `keepDiagnosticLines` (regex `^.+:\d+:\d+(-\d+)?: .+$`) to drop operational chatter like missing `%AppData%`. Absent `gopls` or timeout returns `""` (silence, not an error). Output is capped at `goplsMaxOutput` and handed to the critic prompt as grounding; the critic is explicitly told to treat tool output as evidence.
5. Builds the prompt with `buildCriticPrompt` (files in fenced blocks, sorted keys, optional uncovered summary, instructions to emit `FINDING file.go:123 severity: claim` or `NO FINDINGS`, and to treat inventing a finding as worse than finding nothing). The uncovered-summary section instructs the reviewer to report uncovered blocks worth testing at `medium` severity.
6. Calls the critic model via `e.criticClient().CompleteWithSystem` bounded by `criticTimeout` (3m). If the call times out or errors, the gate is abandoned and the turn proceeds without a review — a failed review is "a missing opinion, not a hard failure".
7. Parses with `parseCriticFindings`: `NO FINDINGS` on any line → `nil`; otherwise regex `^FINDING\s+(\S+):(\d+)\s+(\w+):\s*(.+)$` per line, severity must be `high`/`medium`/`low` (case-insensitive), non-matching lines silently skipped.
8. Filters with `findingsWorthUplift` (keeps only `high`/`medium`; `low` is noise). If `nil`, stores `result.CriticFindings` (if any original findings existed) and returns.
9. If worthy findings remain, stores all findings in `result.CriticFindings`, appends `formatUpliftPrompt(worth)` as a `user` message, and runs one advisory `CompleteWithToolResults` bounded by `criticUpliftTimeout` (5m). The uplift prompt requires the model to either fix each finding or state plainly why the finding is wrong — forcing a fix for a hallucinated finding is explicitly forbidden.
10. Executes any uplift tool calls, then re-runs `verifyBuild` and `verifyTests`. If either fails, `verifyAndUpliftWithCritic` returns an error and **the turn fails**.

    This is the one case where the critic gate can fail a turn, and it is not an exception to "advisory". The critic's *opinion* is advisory: a hallucinated finding must never fail anything. Its *edits* are not privileged — they answer to the compiler and the test runner like any other edit. Acting on a wrong finding and breaking the build is a real break, whoever suggested it.

    > **Provenance:** this step did not exist when this document was first written. The original draft asserted it did, which was a fabricated claim — and checking it exposed a real hole: the uplift round makes edits *after* gates 1 and 2 have already run, so its output was the only code in the loop shipping unverified. The code was changed to match the description rather than the description trimmed to match the code.

`goplsDiagnostics` is not a separate numbered gate; it is the static-analysis grounding signal fed into Gate 4. Without `gopls` on `PATH` the critic runs without it and behaviour is identical to before the signal existed.

---

## 6. The Ran / OK convention — a skipped verification is never a pass

Both `BuildVerification` and `TestVerification` split "did we run?" from "did we pass?":

```go
type BuildVerification struct {
    Ran      bool          // false means skipped/unknown — NOT a pass
    OK       bool          // true only when the build actually succeeded
    Output   string
    Duration time.Duration
}
type TestVerification struct {
    Ran      bool
    OK       bool
    Output   string
    Duration time.Duration
}
```

Coverage follows the same discipline with a different shape: `verifyTestsWithCoverage` returns `(verification, nil)` and `uncoveredWrittenCode` / `parseCoverProfile` callers treat `(nil, nil)` as "no signal, not nothing uncovered". The comment in `coverage_profile.go` states it explicitly: "Absence of a profile is 'unknown', never 'covered'".

`Ran == false` occurs when:

- no successful write-mutation touched a `.go` file (`touchedGoFiles` / `SuccessfulWriteTools` guard),
- workspace is empty (no `WorkspaceRoot` and `FindWorkspaceRoot` failed),
- no `go` toolchain on `PATH` (`exec.LookPath("go")` fails),
- verification disabled via `ExecutorConfig` flag,
- the verification subprocess timed out (`buildVerifyTimeout` / `testVerifyTimeout` / `coverVerifyTimeout`).

All of these paths log at `Warn` or `Debug` and return without touching `result.Error`. Callers test `if !verification.Ran || verification.OK { return }` — only `Ran && OK` is a pass, only `Ran && !OK` is a fail, everything else is unknown and must never be reported as success. A gate that reports "verification skipped" while looking enabled is described in `workspaceForVerification`'s comment as the dormant-wiring defect this codebase keeps producing.

---

## 7. Configuration and result fields the gates use

### 7.1 `ExecutorConfig` — `internal/session/executor.go`

```go
type ExecutorConfig struct {
    MaxToolCalls           int
    MaxToolIterations      int
    ToolTimeout            time.Duration
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
- `internal/session/test_verify.go` — `TestVerification`, `verifyTests`, `packagesForPaths`, `untestedGoFiles`, `untestedWithoutCoverageOnDisk`, `DeduplicatePreservingOrder`, `TrimGoExtension`
- `internal/session/coverage_profile.go` — `UncoveredBlock`, `parseCoverProfile`, `verifyTestsWithCoverage`, `uncoveredWrittenCode`, `NormalizeCoverPath`, `summarizeUncovered`
- `internal/session/lsp_diagnostics.go` — `goplsDiagnostics`, `keepDiagnosticLines`, `diagnosticLineRe`
- `internal/session/critic.go` — `CriticFinding`, `buildCriticPrompt`, `parseCriticFindings`, `findingsWorthUplift`, `CriticSeverityRank`, `readWrittenFilesForReview`, `formatUpliftPrompt`
- `internal/session/executor.go` — `ExecutorConfig`, `ExecutionResult`, `DefaultExecutorConfig`
- `internal/session/executor_tools.go` — `runToolLoop` gate sequencing, `executeToolBatch` population of `SuccessfulWriteTools`/`WrittenPaths`
