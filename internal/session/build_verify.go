package session

import (
	"codenerd/internal/broker"
	"codenerd/internal/build"
	"codenerd/internal/config"
	"codenerd/internal/gates"
	jitconfig "codenerd/internal/jit/config"
	"codenerd/internal/logging"
	"codenerd/internal/testfacts"
	"codenerd/internal/tools"
	"codenerd/internal/types"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// ErrVerificationFailed marks post-edit verification failures (build, test,
// or critic uplift re-verification) so callers can distinguish broken edits
// from LLM generation failures. Wrap every verification-gate error with it
// via fmt.Errorf("%w: ...", ErrVerificationFailed, ...) so errors.Is reports
// true while the compiler/test output detail is preserved in the message.
var ErrVerificationFailed = errors.New("post-edit verification failed")

// Post-edit build verification.
//
// codeNERD edited cmd/nerd/cmd_instruction.go, produced four compile errors —
// an unused import, a duplicated block redeclaring variables with :=, and two
// calls to helper functions it never wrote — then asserted
// task_status(/manual_instruction, /complete) and exited 0. A single `go build`
// would have caught all four.
//
// Nothing verified write-tool output. internal/core/self_healing.go defined a
// SelfHealer with HandleValidationFailure / retryAction / rollbackAction /
// escalateToUser and had zero production callers anywhere in the repo, and was
// deleted once this file superseded it; ValidatorRegistry survived only in
// comments describing what it would dispatch on. The machinery to check the
// work existed and was wired to nothing.
//
// Detection alone would only convert a false success into an honest failure.
// The point of this file is the repair round: the compiler's own errors go back
// to the model as a tool-result turn, so the agent fixes its mistake rather
// than handing back broken code. That is the difference between an agent that
// writes plausible code and one that can finish a job.

// BuildVerification is the outcome of compiling the workspace after edits.
// Outcome is the authoritative verdict; Ran and OK stay as derived
// compatibility (Ran = the command executed, OK = passed). A skipped or
// indeterminate verification is NOT a pass, and gates must branch on
// Verdict, never on OK alone.
type BuildVerification struct {
	// Ran is true when the verification command executed, even if it
	// produced no verdict (timeout, cancellation). It is false only when
	// nothing ran: skipped for lack of workspace, toolchain, or packages.
	Ran bool

	// OK is true only when the build actually succeeded.
	OK bool

	// Output is the compiler's stderr/stdout. Empty on success and on runs
	// that produced no text (skips, pre-start cancels).
	Output string

	// Diagnostics is every file:line the build named
	// (gates.ParseGoDiagnostics). Nil means the output was not parsed (a
	// hand-built verification); the gate then parses Output. A non-nil
	// empty slice means the failure named no file. Line and column stay
	// here so a reader of the verification can name the site; the fact
	// the policy joins is the file and the package.
	Diagnostics []gates.GoDiagnostic

	// Duration is how long the build took.
	Duration time.Duration

	// Outcome is the explicit verdict: passed, failed, skipped,
	// indeterminate (budget exhausted), or canceled.
	Outcome VerifyOutcome

	// Command is the argv executed, for provenance. Nil when nothing ran.
	Command []string

	// Reason explains a non-pass outcome without overloading Output.
	Reason string

	// Repair is the episode record when a repair loop ran for this gate:
	// outcome, cost, per-attempt evidence, edited files, follow-ups. Nil
	// when the gate passed (or was skipped/canceled) without repair.
	Repair *RepairRecord

	// VetMeasured is set by verifyVet when the run was conclusive: exit 0,
	// or a non-zero exit that named findings. VetNow is that run's findings.
	// VetBeforeKnown is true when the pre-turn vet also finished; VetBefore
	// is what it reported, empty when it was clean. A finding's identity is
	// its file and message, not its line.
	VetMeasured    bool
	VetBeforeKnown bool
	VetNow         []vetDiagnostic
	VetBefore      []vetDiagnostic
}

// Verdict returns the authoritative outcome, deriving one for hand-built
// structs that predate the Outcome field.
func (v BuildVerification) Verdict() VerifyOutcome {
	if v.Outcome != "" {
		return v.Outcome
	}
	switch {
	case v.Ran && v.OK:
		return VerifyPassed
	case v.Ran:
		return VerifyFailed
	default:
		return VerifySkipped
	}
}

// locatedDiagnostics is the file:line list the build gate asserts. A
// verification verifyBuild filled already carries it. A hand-built failure
// does not, so its Output is parsed once here; empty output names nothing.
func (v BuildVerification) locatedDiagnostics(workspace string) []gates.GoDiagnostic {
	if v.Diagnostics != nil {
		return v.Diagnostics
	}
	if strings.TrimSpace(v.Output) == "" {
		return nil
	}
	return gates.ParseGoDiagnostics(workspace, v.Output)
}

// workspaceForVerification resolves the directory the verification build runs
// in, falling back to workspace discovery when the config does not carry one.
//
// The fallback is not belt-and-braces, it is the difference between a live gate
// and a dead one. Only the system factory sets ExecutorConfig.WorkspaceRoot;
// both campaign executors (cmd/nerd/cmd_campaign.go) construct an Executor and
// never call SetConfig at all, so they inherit DefaultExecutorConfig with an
// empty root — and campaigns are where the write volume is. With a bare field
// read, verifyBuild would return Ran=false for every campaign edit and the
// whole mechanism would report "verification skipped" forever while looking
// enabled. That is the exact shape of the dormant-wiring defect this codebase
// keeps producing, so the gate resolves its own workspace rather than trusting
// every future construction site to remember a field.
func (e *Executor) workspaceForVerification() string {
	if ws := strings.TrimSpace(e.configSnapshot().WorkspaceRoot); ws != "" {
		root, _ := tools.CanonicalWorkspaceRoot(ws)
		return root
	}
	if root, err := config.FindWorkspaceRoot(); err == nil && strings.TrimSpace(root) != "" {
		canonical, _ := tools.CanonicalWorkspaceRoot(root)
		return canonical
	}
	return ""
}

// verifyBuild compiles the workspace and reports whether it still builds.
//
// Uses build.GetBuildEnv so the verification inherits the same CGO_CFLAGS the
// project needs (this repo does not compile without
// -I<workspace>/sqlite_headers). A verification that fails for want of the
// build environment would send the agent chasing phantom errors, which is
// worse than not verifying at all.
func verifyBuild(ctx context.Context, workspace string, userCfg *config.UserConfig) BuildVerification {
	start := time.Now()
	command := []string{"go", "build", "./..."}

	if strings.TrimSpace(workspace) == "" {
		return BuildVerification{Outcome: VerifySkipped, Reason: "empty workspace path", Duration: time.Since(start)}
	}
	if _, err := verifyLookPath("go"); err != nil {
		logging.Get(logging.CategorySession).Warn(
			"build verification skipped: no Go toolchain on PATH (%v)", err)
		return BuildVerification{Outcome: VerifySkipped, Reason: "no Go toolchain on PATH", Duration: time.Since(start)}
	}

	out, outcome, reason := runVerificationCommand(ctx, workspace, build.GetBuildEnv(userCfg, workspace), buildVerifyTimeout, command[0], command[1:], verifyBuildRunner)
	elapsed := time.Since(start)

	switch outcome {
	case VerifyPassed:
		logging.SessionDebug("build verification passed in %s", elapsed.Round(time.Millisecond))
		return BuildVerification{Ran: true, OK: true, Outcome: VerifyPassed, Command: command, Duration: elapsed}
	case VerifyFailed:
		text := strings.TrimSpace(string(out))
		if text == "" {
			text = reason
		}
		// The compiler output goes back whole. A repair prompt built from the
		// first 6000 characters was a repair of the errors that happened to sort
		// first; if the whole log does not fit the window, the broker refuses the
		// request and says so instead.
		logging.Get(logging.CategorySession).Warn(
			"build verification FAILED in %s:\n%s", elapsed.Round(time.Millisecond), text)
		// A non-nil slice, empty when nothing was located. Nil would mean
		// "not parsed", and a later sync would parse the text a second time.
		diags := gates.ParseGoDiagnostics(workspace, text)
		if diags == nil {
			diags = []gates.GoDiagnostic{}
		}
		return BuildVerification{Ran: true, OK: false, Output: text, Diagnostics: diags, Outcome: VerifyFailed, Command: command, Reason: reason, Duration: elapsed}
	case VerifyCanceled:
		logging.Get(logging.CategorySession).Warn("build verification canceled: %s", reason)
		return BuildVerification{Ran: len(out) > 0, Output: strings.TrimSpace(string(out)), Outcome: VerifyCanceled, Command: command, Reason: reason, Duration: elapsed}
	default: // VerifyIndeterminate
		// A timeout is not evidence the code is broken — but it is not
		// evidence of recovery either. Report it as indeterminate with
		// whatever the compiler had printed, so gates retain what they knew
		// instead of minting a pass from silence.
		logging.Get(logging.CategorySession).Warn(
			"build verification timed out after %s; recovery not verified", buildVerifyTimeout)
		return BuildVerification{Ran: true, Output: strings.TrimSpace(string(out)), Outcome: VerifyIndeterminate, Command: command, Reason: reason, Duration: elapsed}
	}
}

// verifyAndRepairBuild compiles the workspace after a turn's edits and, if the
// build is broken, runs a bounded repair episode with the compiler's own
// output in hand.
//
// This used to be exactly one round, on the theory that a model that cannot
// fix its own breakage with the errors in front of it will not fix it on the
// fourth attempt either. C1 keeps the hard bound but makes it a loop: the
// attempt ceiling (RepairMaxAttempts) makes iteration safe where an unbounded
// loop was not, and real failures — a fix that
// addresses the first error but exposes the second — need more than one shot.
// If the budget exhausts, the turn fails loudly with the errors, the cost
// ledger, and follow-ups, rather than reporting the success that started this
// whole problem.
//
// Returns the model's post-repair response when a repair happened, nil when no
// repair was needed, and an error when the build is still broken. When a
// recheck produces no verdict (timeout), the original failure is retained on
// the result and the turn completes unverified with no error —
// closeChangeEvidence arbitrates the final workspace with a fresh check.
func (e *Executor) verifyAndRepairBuild(
	ctx context.Context,
	trp types.ToolResultsProvider,
	systemPrompt string,
	history []types.Message,
	current *types.LLMToolResponse,
	toolDefs []types.ToolDefinition,
	cfg *jitconfig.EffectiveAgentRuntimeConfig,
	result *ExecutionResult,
) (*types.LLMToolResponse, []string, error) {
	if !e.configSnapshot().VerifyBuildAfterEdits || result == nil {
		return nil, nil, nil
	}

	workspace := e.workspaceForVerification()
	verification := verifyBuild(ctx, workspace, nil)
	result.BuildCheck = verification
	switch verification.Verdict() {
	case VerifyPassed, VerifySkipped:
		return nil, nil, nil
	case VerifyCanceled:
		return nil, nil, fmt.Errorf("build verification canceled: %w", context.Canceled)
	case VerifyIndeterminate:
		// No known failure: nothing to repair, and a timeout is not
		// evidence of breakage. The turn completes unverified;
		// closeChangeEvidence arbitrates the final workspace.
		logging.Get(logging.CategorySession).Warn(
			"Build verification timed out on the initial check; turn completes unverified")
		return nil, nil, nil
	}

	// A failure policy does not charge to this turn is not this turn's to
	// repair (dogfood runs 4 and 5: go build ./... failed in another
	// agent's half-written package). The command stays whole-workspace;
	// attribution is the gate's. An unlocatable failure (no file:line) is
	// charged, so the repair still runs: fail closed. With no kernel there
	// is nothing to ask, and the raw exit is charged the same way.
	if !e.buildGateRed(ctx, result.turnAtom(), result) {
		logging.Get(logging.CategorySession).Warn(
			"build is broken outside this turn (%s); no repair attempted",
			strings.Join(result.BuildForeignPackages, ", "))
		return nil, nil, nil
	}

	if trp == nil {
		return nil, nil, fmt.Errorf(
			"%w: edits broke the build and no repair is possible (client cannot accept tool results):\n%s",
			ErrVerificationFailed, verification.Output)
	}

	// The prompt names the pre-gate write set, the one WithTurnWriteSet froze
	// in verifyCompletedToolTurn before the rounds map. A round invoked with
	// no snapshot names WrittenPaths as they are now. Later attempts must
	// not rename the set to the files the episode has created since.
	repairWriteSet := namedWriteSet(ctx, result)
	spec := repairSpec{
		kind:         "build",
		brokenPhrase: "edits broke the build",
		// The round is shown the turn's own edits (N26): a compiler error
		// names a line, and what put it there is the diff.
		promptFor: func(seed string) string {
			return buildRepairPrompt(seed, repairWriteSet) + turnDiffSection(workspace, result.WrittenPaths, result.PreWriteContents, e.configSnapshot().repairDiffBudget())
		},
		recheck: func(epCtx context.Context) (bool, repairFailure, VerifyOutcome) {
			r := verifyBuild(epCtx, workspace, nil)
			// Only affirmative verdicts move the check: an indeterminate
			// recheck leaves the original failure standing.
			if r.Verdict() == VerifyPassed || r.Verdict() == VerifyFailed {
				result.BuildCheck = r
			}
			// A build that still exits non-zero, but only in packages this
			// turn cannot have broken, is not this round's failure. The raw
			// exit stays on BuildCheck. Same shape as testGatePassed: the
			// suite can stay red while the turn is not charged.
			if r.Verdict() == VerifyPassed || (r.Verdict() == VerifyFailed && !e.buildGateRed(epCtx, result.turnAtom(), result)) {
				return true, repairFailure{}, r.Verdict()
			}
			return false, repairFailure{Output: r.Output}, r.Verdict()
		},
		followups: func() []string {
			return repairFollowups(workspace, nil, result, "build")
		},
	}
	repaired, repairErrs, rec, err := e.repairLoop(ctx, trp, systemPrompt, &history, toolDefs, cfg, result, verification.Output, spec)
	result.BuildCheck.Repair = rec
	if err != nil {
		return nil, repairErrs, err
	}
	return repaired, repairErrs, nil
}

// verifyAndRepairTests runs the tests for the packages this turn touched and,
// when they fail, runs a bounded repair episode with the test output in
// hand — the same contract as verifyAndRepairBuild, one level up.
//
// Compiling is a low bar. A turn can write code that builds cleanly, was never
// executed once, and still be reported as complete. This closes that gap.
//
// It also reports production Go written without a test alongside it. That is a
// warning, not a failure: a turn can legitimately edit an existing file whose
// tests were written long ago, and failing it would make the gate wrong more
// often than right. The gap is surfaced so the caller can name it.
func (e *Executor) verifyAndRepairTests(
	ctx context.Context,
	trp types.ToolResultsProvider,
	systemPrompt string,
	history []types.Message,
	toolDefs []types.ToolDefinition,
	cfg *jitconfig.EffectiveAgentRuntimeConfig,
	result *ExecutionResult,
) (*types.LLMToolResponse, []string, error) {
	if !e.configSnapshot().VerifyTestsAfterEdits || result == nil {
		return nil, nil, nil
	}

	workspace := e.workspaceForVerification()

	if untested := untestedWithoutCoverageOnDisk(workspace, result.WrittenPaths); len(untested) > 0 {
		logging.Get(logging.CategorySession).Warn(
			"Turn wrote production Go with no test alongside it: %s", strings.Join(untested, ", "))
		result.UntestedPaths = untested
	}

	verification, uncovered := gateTests(ctx, workspace, result, true)
	uncovered = narrowToChangedLines(workspace, result, uncovered)
	result.TestCheck = verification

	// Coverage is reported whether or not the tests passed. Green tests over
	// code that was never executed is the precise false success this signal
	// exists to expose — `go test` cannot tell the two apart, only the profile
	// can.
	if len(uncovered) > 0 {
		result.UncoveredBlocks = uncovered
		logging.Get(logging.CategorySession).Warn(
			"Turn wrote %d block(s) of Go that no test executes: %s",
			len(uncovered), summarizeUncovered(uncovered))
	}

	switch verification.Verdict() {
	case VerifyCanceled:
		return nil, nil, fmt.Errorf("test verification canceled: %w", context.Canceled)
	case VerifyIndeterminate:
		// No known failure: nothing to repair, and a timeout is not
		// evidence of breakage. The turn completes unverified;
		// closeChangeEvidence arbitrates the final workspace.
		logging.Get(logging.CategorySession).Warn(
			"Test verification timed out on the initial check; turn completes unverified")
		return nil, nil, nil
	}
	// Passed, skipped, or a failure policy does not charge to the turn
	// (every named failure also failed before it). The outcome stays the
	// run's own exit.
	if !e.testGateRed(result.turnAtom(), result) {
		return nil, nil, nil
	}

	// A run that compiled nothing and located nothing in the turn's files
	// is not this turn's to fix (dogfood run 4: the repair spent all its
	// attempts on another lane's breakage). Name it and stop without
	// burning repair attempts; the error fails the turn the way a give-up
	// does, because the gate as measured does not pass. "Located" is a
	// testfacts BuildFailure row with a non-empty File, and every such row
	// outside the owned paths: the pre-gate set plus files this turn has
	// written since (a failure in a file an earlier repair created is still
	// the turn's). An empty File, or any Failure row, keeps the repair.
	// The build path locates compiler output the same way (buildGateRed).
	// A failure only in packages this turn cannot have broken does not
	// repair; an unlocated build failure still does, fail closed, and the
	// guard repairRound installs keeps those writes inside the set.
	owned := turnOwnedPaths(ctx, result)
	if foreign, outside := testBuildFailuresOutsideWriteSet(result.TestCheck.Result, owned, workspace); outside {
		logging.Get(logging.CategorySession).Warn(
			"Tests failed to compile in files outside this turn's write set (%s); not this turn's to fix, no repair attempted",
			strings.Join(foreign, ", "))
		return nil, nil, fmt.Errorf(
			"%w: tests failed to compile in files outside this turn's write set (%s); not this turn's to fix, so no repair was attempted (write set: %s). Last failure:\n%s",
			ErrVerificationFailed, strings.Join(foreign, ", "), strings.Join(owned, ", "), verification.Output)
	}

	logging.Get(logging.CategorySession).Warn(
		"Edits broke the tests; giving the model one repair round with the test output")

	if trp == nil {
		return nil, nil, fmt.Errorf(
			"%w: edits broke the tests and no repair is possible (client cannot accept tool results):\n%s",
			ErrVerificationFailed, verification.Output)
	}

	spec := repairSpec{
		kind:         "tests",
		brokenPhrase: "edits broke the tests",
		// The prompt carries the failing tests themselves (N24): this round
		// closes the read tools after its first round that writes nothing, so
		// a model that has not already read the test it broke cannot.
		promptFor: func(seed string) string {
			// repairSpec.promptFor threads the seed as text. The Result is
			// the test check's when this seed is that check's Output. A
			// build recheck's seed is compiler text and has no Result;
			// the previous run's tests would name the wrong files.
			var res *testfacts.Result
			if result.TestCheck.Output == seed {
				res = result.TestCheck.Result
			}
			return testRepairPrompt(seed, failingTestSection(workspace, res, result.WrittenPaths)) +
				turnDiffSection(workspace, result.WrittenPaths, result.PreWriteContents, e.configSnapshot().repairDiffBudget())
		},
		// A test repair can break the build, so re-check both, cheapest
		// first. Only an affirmative failure verdict fails here: a recheck
		// that produced no verdict cannot prove the repair broke anything.
		recheck: func(epCtx context.Context) (bool, repairFailure, VerifyOutcome) {
			if rb := verifyBuild(epCtx, workspace, nil); rb.Verdict() == VerifyFailed {
				result.BuildCheck = rb
				// An attributed build break fails this repair. A failure
				// only in packages this turn cannot have broken does not:
				// the test repair is not asked to edit them.
				if e.buildGateRed(epCtx, result.turnAtom(), result) {
					// testRepairPrompt reads a compile failure out of the output
					// itself, so this round keeps its own prompt either way.
					return false, repairFailure{Output: rb.Output}, VerifyFailed
				}
			} else if rb.Verdict() == VerifyCanceled {
				return false, repairFailure{}, VerifyCanceled
			}
			rt, _ := gateTests(epCtx, workspace, result, false)
			if rt.Verdict() == VerifyPassed || rt.Verdict() == VerifyFailed {
				result.TestCheck = rt
			}
			if e.testGatePassed(result.turnAtom(), result) {
				return true, repairFailure{}, VerifyPassed
			}
			return false, repairFailure{Output: rt.Output}, rt.Verdict()
		},
		followups: func() []string {
			runnable, _ := splitTagGatedPackages(workspace, packagesForPaths(result.WrittenPaths))
			return repairFollowups(workspace, runnable, result, "tests")
		},
	}
	repaired, repairErrs, rec, err := e.repairLoop(ctx, trp, systemPrompt, &history, toolDefs, cfg, result, verification.Output, spec)
	result.TestCheck.Repair = rec
	if err != nil {
		return nil, repairErrs, err
	}
	return repaired, repairErrs, nil
}

// gateTests runs the post-edit test gate over the packages the turn wrote:
// go test (with coverage and baseline attribution) on packages the default
// tags can build, go vet -tags on packages whose files are all tag-gated, and
// then -- when those pass -- the tests of the packages that import them
// (N25, importer_packages.go), because a contract the turn changed is kept by
// its callers and not by itself.
func gateTests(ctx context.Context, workspace string, result *ExecutionResult, withCoverage bool) (TestVerification, []UncoveredBlock) {
	v, uncovered := gateOwnTests(ctx, workspace, result, withCoverage)
	// Importers still run when the turn's own named failures all predate it,
	// so a new importer failure is measured. That comparison does not set
	// the outcome: the /test rule does, from the run this returns.
	if v.Verdict() != VerifyPassed && !failuresAllPredate(v) {
		return v, uncovered
	}
	return mergeImporterVerdict(v, verifyImporters(ctx, workspace, result)), uncovered
}

// mergeImporterVerdict is what the gate reports once the turn's own packages
// have passed and the importer check has run.
//
// Only an affirmative pass, or a check that had nothing to run, leaves the
// turn's own pass standing. Anything else is the gate's answer, including a
// check that produced no verdict: ladder run R1-18 (2026-09-19) hit the
// four-minute verification budget on six importer packages and still printed
// "build ok | tests ok", because the old form propagated VerifyFailed alone and
// everything else fell through to the pass measured on other packages. A
// timeout is not proof that the importers passed.
func mergeImporterVerdict(own, imp TestVerification) TestVerification {
	switch imp.Verdict() {
	case VerifyPassed, VerifySkipped:
		return own
	default:
		return imp
	}
}

// gateOwnTests is the gate over the turn's own packages.
func gateOwnTests(ctx context.Context, workspace string, result *ExecutionResult, withCoverage bool) (TestVerification, []UncoveredBlock) {
	workspace = goWorkspace(workspace)
	runnable, gated := splitTagGatedPackages(workspace, packagesForPaths(result.WrittenPaths))
	if len(gated) > 0 {
		if failed, ok := vetTagGatedPackages(ctx, workspace, gated); !ok {
			return failed, nil
		}
	}
	if len(runnable) == 0 {
		return TestVerification{Outcome: VerifySkipped, Reason: "only tag-gated packages; compile-checked with go vet"}, nil
	}
	if withCoverage {
		v, uncovered := verifyTestsWithCoverage(ctx, workspace, runnable, result.WrittenPaths)
		v = attributeTestFailures(ctx, workspace, runnable, result.WrittenPaths, result.PreWriteContents, v)
		return withWrittenTagGatedTests(ctx, workspace, result.WrittenPaths, v), uncovered
	}
	v := attributeTestFailures(ctx, workspace, runnable, result.WrittenPaths, result.PreWriteContents, verifyTests(ctx, workspace, runnable))
	return withWrittenTagGatedTests(ctx, workspace, result.WrittenPaths, v), nil
}

// testBuildFailed reports whether a `go test -json` run compiled nothing:
// compiler diagnostics, or a package verdict of build-failed. A package
// whose build constraints exclude every file fails with FailedBuild set
// (observed 2026-09-28), which is that verdict; the "[setup failed]" line
// is the same event's text and is not scanned. No test ran, so there is
// no test verdict to honour.
func testBuildFailed(res *testfacts.Result) bool {
	if res == nil {
		return false
	}
	if len(res.BuildFailures) > 0 || res.Status == testfacts.StatusBuildFailed {
		return true
	}
	for _, p := range res.Packages {
		if p.Status == testfacts.StatusBuildFailed {
			return true
		}
	}
	return false
}

// testBuildFailuresOutsideWriteSet reports whether every located failure in
// a test run sits outside the turn's write set: compiler diagnostics in
// files the turn never wrote, and no test failure reported anywhere. A
// failing test's file is where the failure is reported, not where the fix
// goes -- the fix is usually the turn's own production code -- so any
// Failure row keeps the repair; only a run that compiled nothing and
// located nothing in the set is outside it. An unlocated diagnostic (no
// file) also keeps the repair: what cannot be located cannot be charged
// elsewhere. It returns the foreign files for the report.
func testBuildFailuresOutsideWriteSet(res *testfacts.Result, writeSet []string, workspace string) ([]string, bool) {
	if res == nil || len(res.BuildFailures) == 0 || len(res.Failures) != 0 {
		return nil, false
	}
	// The resolved root, as the gates run under it: the go tool may spell
	// an absolute path through an alias of the workspace (a symlink, a
	// short name), and keyed against any other spelling an in-set file
	// reads as foreign (go_paths.go).
	workspace = goWorkspace(workspace)
	inSet := func(file string) bool {
		file = workspaceFile(workspace, file)
		for _, w := range writeSet {
			// EqualFold, not ==: a case-variant spelling of a set file is
			// not positive evidence of a foreign file, and the skip fires
			// only on positive evidence.
			if strings.EqualFold(file, workspaceFile(workspace, w)) {
				return true
			}
		}
		return false
	}
	var foreign []string
	for _, bf := range res.BuildFailures {
		if strings.TrimSpace(bf.File) == "" {
			return nil, false
		}
		if inSet(bf.File) {
			return nil, false
		}
		foreign = append(foreign, workspaceFile(workspace, bf.File))
	}
	return foreign, true
}

// testRepairPrompt is the turn handed back to the model when its edits broke
// the tests.
//
// It forbids the cheapest way out. An agent told only "the tests fail" will
// often delete or weaken the assertion, which turns red green while destroying
// the thing that made the suite worth running. The failing test is the
// specification until proven otherwise.
// withSource is the failing tests' source when the round has it (N24). The
// closing sentence depends on it: telling a model to read the failing test
// while the round has closed the read tools is an instruction it cannot
// follow, and R1-12 spent three attempts on recall_context trying.
func testRepairPrompt(testOutput, withSource string) string {
	// testOutput is text because repairSpec.promptFor (repair_loop.go)
	// threads the seed string, not the Result. A `go test` compile
	// failure's seed is the Summary, whose build-failed lines are the
	// diagnostics; a later `go build` recheck's seed is the compiler's
	// own text and has no Result. Gates that hold the Result use
	// testBuildFailed.
	if summaryShowsBuildFailure(testOutput) {
		return testCompileRepairPrompt(testOutput)
	}
	closing := "Read the failing test and the code under test before editing either."
	if withSource != "" {
		closing = "The failing tests are below as they are on disk; read them there, not with a tool, " +
			"and edit the code under test."
	}
	return "Your edits compile but the tests fail. This is the test output:\n\n" +
		"```\n" + testOutput + "\n```\n\n" +
		"Fix the code so these tests pass, then stop. Do not explain, do not summarise, " +
		"and do not report success — the tests will be run again.\n\n" +
		"Do NOT delete the failing test, weaken its assertion, skip it, or change what it " +
		"expects in order to make it pass. The test states the required behaviour; your code " +
		"does not meet it yet. If — and only if — you can show the test itself asserts something " +
		"incorrect, say so explicitly and explain why before changing it.\n\n" +
		closing + withSource
}
func testCompileRepairPrompt(output string) string {
	return "The tests do not compile, so no test ran. This is the compiler's output:\n\n```\n" + output + "\n```\n\n" +
		"Each error names a file and line. When the file is a _test.go file, the test file is what is wrong: fix its imports, identifiers and types so it compiles against the code as it is. " +
		"Do NOT add, alias or re-export declarations in non-test code to make a test compile, and do NOT create new non-test files for it — that bends working code around a broken test. " +
		"Never delete, rename or comment out a test to make the package compile — fix its imports, identifiers and types instead; a test you just added is part of the deliverable. " +
		"When the error is in a non-test file, fix that file. Fix every error above using the edit tools, then stop; the tests will be run again."
}

// buildRepairPrompt is the turn handed back to the model when its edits broke
// the build.
//
// It states the failure as fact and asks for a fix, rather than asking whether
// one is needed — the compiler has already decided. It also names the specific
// mistakes seen in the live failure, because those are the ones an editing
// agent actually makes: a stale import left behind, a block pasted twice, and a
// call to a helper that was planned but never written. And it names the
// write set: failures outside it are not the turn's to fix (dogfood run 5),
// and the episode's guard refuses those edits, so the prompt says so up
// front instead of letting the model burn calls on them.
func buildRepairPrompt(compilerOutput string, writeSet []string) string {
	set := strings.Join(writeSet, ", ")
	if set == "" {
		set = "(none yet)"
	}
	return "Your edits do not compile. This is the compiler's output:\n\n" +
		"```\n" + compilerOutput + "\n```\n\n" +
		"Fix every error above using the edit tools, then stop. Do not explain, do not " +
		"summarise, and do not report success — the build will be checked again.\n\n" +
		"Check specifically for the mistakes that produce these errors:\n" +
		"  - an import added for code you did not end up writing (\"imported and not used\")\n" +
		"  - a block inserted twice, re-declaring variables with := (\"no new variables on left side of :=\")\n" +
		"  - a call to a helper function you planned but never wrote (\"undefined: ...\")\n" +
		"The compiler names the file and line of each error; edit those lines, but only in your write set: " + set + ". " +
		"Failures outside that set are not this turn's to fix: do not edit those files, and say which errors are outside it."
}

// repairRound sends one repair prompt through the working request path and
// runs the attempt's rounds, executing each response's batch and appending
// that round's assistant message and tool results to history so the next
// round sees them. How many rounds an attempt gets is the working policy's
// (working_set.mg), not a count: the attempt runs under the /repair regime,
// each round's progress is reported to the policy, and the attempt ends at
// its first successful write (the write hands control back to repairLoop,
// which rechecks before another call is spent), when the model answers with
// no tool calls, or when the policy derives a working_stop or a
// working_finalize. A six-call ceiling stood here until sweep finding F10.
//
// One round of reading is the diagnosis (F-REPAIR-1: an attempt whose only
// call spends itself reading cannot converge on even a one-line compile
// error); the policy then closes reading, working_regime(/commit) under
// /repair, and the failing prompt is re-sent with the regime text so the next
// call carries both. Under commit the read tools are withheld from the
// catalog and a read asked for anyway is answered with the regime. Reading
// stays closed to the end of the attempt: the recheck repairLoop runs after
// it is the verification that reopens it, not one the model runs itself. An
// attempt repairLoop starts closed (commit: an earlier attempt of the episode
// made no edit, repair_closed) runs under /commit from its first round.
//
// A repair needs the working policy: outside a working loop it refuses, as
// the tool loop does, rather than run rounds nothing can end.
//
// It returns the last response (its Usage holds the sum across every round, so
// the attempt's cost counts each call rather than only the last one), the
// number of model calls actually completed, the tool calls of every round
// (repair_loop's toolRunsFor needs the calls of all rounds), the collected
// repair errors, every round's tool results in order, and whether any round
// wrote.
func (e *Executor) repairRound(
	ctx context.Context,
	trp types.ToolResultsProvider,
	systemPrompt string,
	history *[]types.Message,
	toolDefs []types.ToolDefinition,
	cfg *jitconfig.EffectiveAgentRuntimeConfig,
	result *ExecutionResult,
	prompt string,
	commit bool,
) (*types.LLMToolResponse, int, [][]types.ToolCall, []string, []types.ToolResult, bool, error) {
	// Every repair attempt, coverage and pinning included (they share this
	// round), may write only the turn's files. The refusal names the
	// pre-gate set; a file the turn has written since, and a path that does
	// not exist yet, stay writable. A create cannot destroy another agent's
	// bytes, and those rounds write their tests inside the episode.
	ctx = withRepairWriteGuard(ctx, e.workspaceForVerification(), result)
	ctx = broker.WithPhase(ctx, broker.PhaseRepair)
	loop := activeWorkingLoop(ctx)
	if loop == nil {
		return nil, 0, nil, nil, nil, false, errors.New(
			"repair requires a working continuation policy and this turn has none " +
				"(no workspace root, so no working set could be built)")
	}
	threshold, err := loop.set.RepeatThreshold(ctx)
	if err != nil {
		return nil, 0, nil, nil, nil, false, fmt.Errorf("working continuation policy: %w", err)
	}
	meter := newWorkingMeter(threshold)
	previousRegime := loop.regime
	defer func() { loop.regime = previousRegime }()
	loop.regime = repairRegime
	if commit {
		loop.regime = commitRegime
	}

	*history = append(*history, types.Message{Role: "user", Text: prompt})
	var repairErrs []string
	var toolResults []types.ToolResult
	var allCalls [][]types.ToolCall
	var last *types.LLMToolResponse
	llmCalls := 0
	wrote := false
	failedRounds := 0
	for {
		before := 0
		if result != nil {
			before = result.SuccessfulWriteTools
		}
		repaired, err := e.completeWithWorkingContext(ctx, trp, systemPrompt, *history, toolDefs)
		if err != nil {
			return nil, llmCalls, allCalls, repairErrs, toolResults, wrote, err
		}
		llmCalls++
		if repaired == nil {
			break
		}
		if last != nil {
			// The caller charges the attempt for the returned response's
			// usage, so fold the earlier rounds' accumulated tokens into
			// the newest response before it becomes the one returned.
			repaired.Usage.InputTokens += last.Usage.InputTokens
			repaired.Usage.OutputTokens += last.Usage.OutputTokens
			repaired.Usage.TotalTokens += last.Usage.TotalTokens
			repaired.Usage.ThinkingTokens += last.Usage.ThinkingTokens
			repaired.Usage.CachedContentTokens += last.Usage.CachedContentTokens
		}
		last = repaired
		if len(repaired.ToolCalls) == 0 {
			break
		}
		allCalls = append(allCalls, repaired.ToolCalls)
		results, errs := e.executeToolBatch(ctx, repaired.ToolCalls, cfg, result)
		repairErrs = append(repairErrs, errs...)
		toolResults = append(toolResults, results...)
		meter.observe(repaired.ToolCalls, results)
		failedRounds++
		for _, r := range results {
			if !r.IsError {
				failedRounds = 0
				break
			}
		}
		wrote = result != nil && result.SuccessfulWriteTools > before
		if wrote {
			*history = append(*history,
				types.AssistantMessageFrom(repaired),
				types.Message{Role: "user", ToolResults: results})
			break
		}

		// A round that did not write: the policy says whether the attempt
		// goes on, and under which regime.
		progress := meter.workingProgress(true, failedRounds)
		progress.Regime = loop.regime
		decision, policyErr := loop.set.Continue(ctx, progress)
		if policyErr != nil {
			return last, llmCalls, allCalls, repairErrs, toolResults, wrote, fmt.Errorf("working continuation policy: %w", policyErr)
		}
		if decision.Nudge != "" {
			results = appendWorkingNudge(results, workingNudgeText(decision.Nudge, progress))
		}
		*history = append(*history,
			types.AssistantMessageFrom(repaired),
			types.Message{Role: "user", ToolResults: results})
		if !decision.Continue || decision.Finalize != "" {
			reason := decision.Stop
			if reason == "" {
				reason = decision.Finalize
			}
			logging.Get(logging.CategorySession).Warn(
				"Repair attempt ended by the working policy (%s) after %d model call(s) without an edit", reason, llmCalls)
			break
		}
		if decision.Regime == commitRegime {
			loop.regime = commitRegime
		}
		if loop.regime == commitRegime {
			// Freshly closed, or already closed where this round executed
			// tools without writing: carry the failing output plus the regime
			// text into the next call.
			*history = append(*history, types.Message{Role: "user", Text: withRegimePrompt(prompt, commitRegime)})
		}
	}
	return last, llmCalls, allCalls, repairErrs, toolResults, wrote, nil
}

// verifyAndUpliftWithCritic runs one adversarial review of the code this turn
// wrote and, when it reports something worth acting on, gives the model one
// round to respond. The round's edits count; its words do not: the answer the
// turn surfaces is the one the loop produced, so a narrated reaction to the
// review never replaces it (observed 2026-09-11: "Tackling the review's
// coverage gaps — inspecting the runner to judge each finding" was a turn's
// whole answer).
//
// This is the third question, after "does it compile" and "do the tests pass":
// is it actually right. A turn can satisfy both mechanical gates and still ship
// a logic error, a race, or a broken contract — the compiler and the test runner
// only check what they were told to check.
//
// It is deliberately the weakest gate of the three, and it can NEVER fail a
// turn. The other two gates are backed by a compiler and a test runner, which
// do not have opinions. This one is backed by a model reviewing another model's
// work, and it will sometimes be confidently wrong. A critic that can fail a
// turn on a hallucinated defect is worse than no critic, because the cost lands
// on correct code. So: findings become one advisory round, never an error.
//
// Only high and medium findings trigger the round (turn_needs_uplift,
// coder_safety.mg). Low-severity output from a reviewer asked to look hard at
// code is mostly style, and paying a full model round for it on every write
// turn is how a useful signal turns into a tax.
func (e *Executor) verifyAndUpliftWithCritic(
	ctx context.Context,
	trp types.ToolResultsProvider,
	systemPrompt string,
	history []types.Message,
	toolDefs []types.ToolDefinition,
	cfg *jitconfig.EffectiveAgentRuntimeConfig,
	result *ExecutionResult,
) ([]string, error) {
	if !e.configSnapshot().CriticReviewAfterEdits || result == nil {
		return nil, nil
	}

	workspace := e.workspaceForVerification()
	files := readWrittenFilesForReview(workspace, result.WrittenPaths)
	if len(files) == 0 {
		return nil, nil
	}

	client := e.criticClient()
	if client == nil {
		return nil, nil
	}

	// Ground the reviewer in tool output before asking for its opinion. A
	// reviewer with concrete diagnostics to check against reviews better than
	// one given only source, and gopls reports a class of defect the compiler
	// is deliberately silent about.
	grounding := summarizeUncovered(result.UncoveredBlocks)
	if diags := goplsDiagnostics(ctx, workspace, result.WrittenPaths); diags != "" {
		result.StaticDiagnostics = diags
		logging.Get(logging.CategorySession).Warn("gopls reported diagnostics on this turn's files:\n%s", diags)
		if grounding != "" {
			grounding += "\n\n"
		}
		grounding += "Static analysis (gopls) reported:\n" + diags
	}
	if diags := lspDiagnostics(ctx, workspace, result.WrittenPaths); diags != "" {
		if result.StaticDiagnostics != "" {
			result.StaticDiagnostics += "\n"
		}
		result.StaticDiagnostics += diags
		logging.Get(logging.CategorySession).Warn("language servers reported diagnostics on this turn's files:\n%s", diags)
		if grounding != "" {
			grounding += "\n\n"
		}
		grounding += "Static analysis (language servers) reported:\n" + diags
	}

	// Each file is reviewed as the change this turn made to it: the changed
	// regions, the lines it removed, and the windows a finding must fall in.
	views := make(map[string]string, len(files))
	removals := make(map[string]string, len(files))
	windows := make(map[string][]criticSpan, len(files))
	for path, content := range files {
		before := ""
		if pre, ok := result.PreWriteContents[canonicalizeWrittenPath(path, workspace)]; ok {
			before = pre.Content
		}
		changed := changedSpans(before, content)
		if len(changed) == 0 {
			continue // written back as it was: nothing to review
		}
		if before != "" {
			if r := turnRemovals(before, content); r != "" {
				removals[path] = r
			}
		}
		views[path], windows[path] = criticFileView(path, content, changed)
	}
	if len(views) == 0 {
		return nil, nil
	}
	request := ""
	if loop := activeWorkingLoop(ctx); loop != nil {
		request = loop.task
	}
	prompt := buildCriticPrompt(views, removals, grounding, request)

	// The review is bounded as every request is: by the caller's context and
	// the client's own request bound (the HTTP client each provider is built
	// with). It once carried a 3-minute clock of its own, set when a review
	// hung for twenty minutes and the client had no bound; the client has one
	// now, and the clock cut real
	// reviews -- the planner slot takes 2 to 3 minutes on a change, and R1-8's
	// review of a defective change was abandoned at 3 (06-unattended-hardening,
	// H1). A review that fails for any reason is abandoned and the turn
	// proceeds without it, which is what "advisory" means.
	// The review is the critic's own inference, not a round of the turn: its
	// spend belongs in the critic account, where a change to the critic can
	// be measured, not folded into the session total.
	response, err := client.CompleteWithSystem(broker.WithPurpose(ctx, broker.PurposeCritic), criticSystemPrompt, prompt)
	if err != nil {
		// The critic is advisory. A failed review is a missing opinion, not a
		// failed turn.
		logging.Get(logging.CategorySession).Warn("adversarial review failed (%v); turn continues", err)
		return nil, nil
	}

	findings := parseCriticFindings(response)
	result.CriticFindings = findings

	// One line carrying every gate's verdict. Reaching this point means the
	// build and tests already passed — the two hard gates return early
	// otherwise — so those are true by construction here.
	logging.Get(logging.CategorySession).Info("turn signals: %s",
		SummarizeTurnSignals(true, true, len(result.UncoveredBlocks), len(findings)))

	if len(findings) == 0 {
		return nil, nil
	}

	onChange, offChange := findingsOnChange(findings, windows)
	if len(offChange) > 0 {
		logging.Get(logging.CategorySession).Info(
			"Adversarial review: %d finding(s) cite code this turn did not change; not charged to it", len(offChange))
	}
	worth := e.actionableCriticFindings(result.turnAtom(), onChange)
	logging.Get(logging.CategorySession).Warn(
		"Adversarial review reported %d finding(s), %d worth acting on", len(findings), len(worth))
	if len(worth) == 0 {
		return nil, nil
	}
	if trp == nil {
		logging.Get(logging.CategorySession).Warn(
			"Adversarial review found %d actionable item(s), but this provider has no repair channel", len(worth))
		return nil, nil
	}

	history = append(history, types.Message{Role: "user", Text: formatUpliftPrompt(worth)})
	uplifted, err := e.completeWithWorkingContext(broker.WithPhase(ctx, broker.PhaseUplift), trp, systemPrompt, history, toolDefs)
	if err != nil {
		logging.Get(logging.CategorySession).Warn("uplift round failed (%v); turn continues", err)
		return nil, nil
	}

	var upliftErrs []string
	if uplifted != nil && len(uplifted.ToolCalls) > 0 {
		snap := snapshotTurnFiles(workspace, result)
		// Unguarded on purpose: the critic uplift answers findings about
		// the turn's files but is not a repair episode, and the deleted-
		// test round exists to catch what it removes
		// (TestCriticUplift_ADeletedTestIsHandedBack). Confining it to the
		// write set is a contract change for its own task, not this one.
		_, errs := e.executeToolBatch(ctx, uplifted.ToolCalls, cfg, result)
		upliftErrs = append(upliftErrs, errs...)
		return upliftErrs, recheckUplift(ctx, workspace, result, snap)
	}
	return upliftErrs, nil
}

// recheckUplift measures the uplift round's edits. They answer to the
// compiler and the test runner like every other edit, and they come after the
// test gate's pass, so that pass no longer describes this workspace.
//
// The critic's opinion is advisory, and acting on it must not take down a
// change that was green without it: an uplift that breaks the build or the
// tests is undone, the verdicts it would have invalidated describe the
// workspace again, and the finding stays on the result. Until 2026-09-19 a
// broken uplift failed the turn -- the one place left where the "never fails
// a turn" gate could. Only an undo that cannot be carried out still does,
// because then the break is in the workspace.
//
// An uplift that passes refreshes the build, the tests and the coverage the
// forcing rounds after it start from, so its own new code is covered, vetted
// and inventoried like the rest of the turn's.
func recheckUplift(ctx context.Context, workspace string, result *ExecutionResult, snap turnFiles) error {
	// Only Go writes answer to the Go compiler and test runner. A turn that
	// wrote other languages (the critic reviews those too) is measured after
	// this round by its workspace's own test gate, /test_run, which the kernel
	// orders after the critic; `go build ./...` in a workspace with no go.mod
	// would fail and undo every uplift the review asked for.
	if !wroteGo(result.WrittenPaths) {
		return nil
	}
	build := verifyBuild(ctx, workspace, nil)
	if build.Verdict() == VerifyCanceled {
		return fmt.Errorf("uplift build re-verification canceled: %w", context.Canceled)
	}
	var tests TestVerification
	var uncovered []UncoveredBlock
	broke, failure := "", build.Output
	if build.Verdict() == VerifyFailed {
		broke = "build"
	} else {
		// The same helper the test gate uses, with coverage: a tag-gated
		// package is compile-checked, not failed, and the blocks the uplift
		// left unexecuted reach the coverage round.
		tests, uncovered = gateTests(ctx, workspace, result, true)
		if tests.Verdict() == VerifyCanceled {
			return fmt.Errorf("uplift test re-verification canceled: %w", context.Canceled)
		}
		if tests.Verdict() == VerifyFailed {
			broke, failure = "tests", tests.Output
		}
	}
	if broke != "" {
		restored, conflicts, err := snap.restore(workspace, result)
		// A conflicted undo is a failed undo: the break the uplift left is
		// still in the workspace, so the turn fails loudly below with the
		// paths named instead of claiming the uplift was undone.
		if err == nil && len(conflicts) > 0 {
			err = fmt.Errorf("%s changed since the turn last wrote them and were left as found", strings.Join(conflicts, ", "))
		}
		if err != nil {
			build.Repair = inheritRepair(build.Verdict(), result.BuildCheck.Repair)
			result.BuildCheck = build
			if broke == "tests" {
				tests.Repair = inheritRepair(tests.Verdict(), result.TestCheck.Repair)
				result.TestCheck = tests
			}
			return fmt.Errorf("%w: the adversarial review's uplift round broke the %s and could not be undone (%v):\n%s",
				ErrVerificationFailed, broke, err, failure)
		}
		logging.Get(logging.CategorySession).Warn(
			"The adversarial review's uplift round broke the %s; restored %s as the review found them. Output:\n%s",
			broke, strings.Join(restored, ", "), failure)
		return nil
	}
	build.Repair = inheritRepair(build.Verdict(), result.BuildCheck.Repair)
	result.BuildCheck = build
	if build.Verdict() == VerifyIndeterminate {
		logging.Get(logging.CategorySession).Warn(
			"Uplift build re-verification timed out; prior pass invalidated, recovery NOT verified")
	}
	tests.Repair = inheritRepair(tests.Verdict(), result.TestCheck.Repair)
	result.TestCheck = tests
	result.UncoveredBlocks = narrowToChangedLines(workspace, result, uncovered)
	result.UntestedPaths = untestedWithoutCoverageOnDisk(workspace, result.WrittenPaths)
	if tests.Verdict() == VerifyIndeterminate {
		logging.Get(logging.CategorySession).Warn(
			"Uplift test re-verification timed out; prior pass invalidated, recovery NOT verified")
	}
	return nil
}

// wroteGo reports whether any written path is a Go file.
func wroteGo(paths []string) bool {
	for _, p := range paths {
		if strings.EqualFold(filepath.Ext(strings.TrimSpace(p)), ".go") {
			return true
		}
	}
	return false
}

// criticSystemPrompt keeps the reviewer in the one role that makes it useful.
const criticSystemPrompt = "You are a rigorous, adversarial code reviewer. You report only " +
	"defects you can point to in the code you were given. You have no incentive to find " +
	"something: reporting nothing when the code is sound is a correct and valued outcome, " +
	"and inventing a defect to appear useful is the worst thing you can do."

// criticClient picks the model that reviews the turn.
//
// The planner slot when one is configured, on the theory that finding a bug
// someone else missed is the reasoning-heavy job in this loop, and falling back
// to the same client that wrote the code otherwise. Reviewing your own work
// with your own weights is a weak check, but it is not a useless one, and it is
// what is available when only one slot is configured.
func (e *Executor) criticClient() types.LLMClient {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.plannerClient != nil {
		return e.plannerClient
	}
	return e.llmClient
}
