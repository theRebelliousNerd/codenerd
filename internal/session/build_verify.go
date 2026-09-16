package session

import (
	"codenerd/internal/build"
	"codenerd/internal/config"
	jitconfig "codenerd/internal/jit/config"
	"codenerd/internal/logging"
	"codenerd/internal/tools"
	"codenerd/internal/types"
	"context"
	"errors"
	"fmt"
	"os"
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
// Nothing verified write-tool output. internal/core/self_healing.go defines a
// SelfHealer with HandleValidationFailure / retryAction / rollbackAction /
// escalateToUser and has zero production callers anywhere in the repo;
// ValidatorRegistry survives only in comments describing what it would dispatch
// on. The machinery to check the work existed and was wired to nothing.
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

	// Output is the compiler's stderr/stdout, truncated. Empty on success
	// and on runs that produced no text (skips, pre-start cancels).
	Output string

	// Duration is how long the build took.
	Duration time.Duration

	// Outcome is the explicit verdict: passed, failed, skipped,
	// indeterminate (budget exhausted), or canceled.
	Outcome VerifyOutcome

	// Command is the argv executed, for provenance. Nil when nothing ran.
	Command []string

	// Reason explains a non-pass outcome without overloading Output.
	Reason string
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

// touchedGoFiles reports whether any successful write-mutation touched a .go
// file. Verification is pointless — and expensive — for a turn that only wrote
// markdown.
func touchedGoFiles(paths []string) bool {
	for _, p := range paths {
		if strings.HasSuffix(strings.ToLower(strings.TrimSpace(p)), ".go") {
			return true
		}
	}
	return false
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
		return BuildVerification{Ran: true, OK: false, Output: text, Outcome: VerifyFailed, Command: command, Reason: reason, Duration: elapsed}
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
// build is broken, gives the model exactly one round to fix it with the
// compiler's own output in hand.
//
// One round, not a loop: a model that cannot fix its own syntax with the errors
// in front of it will not fix it on the fourth attempt either, and an unbounded
// repair loop is how an unattended run burns a budget going nowhere. If the
// second build still fails, the turn fails — loudly, with the errors — rather
// than reporting the success that started this whole problem.
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
	if !e.configSnapshot().VerifyBuildAfterEdits {
		return nil, nil, nil
	}
	// Nothing was written, or nothing written was Go: no compile to run.
	if result == nil || result.SuccessfulWriteTools == 0 || !touchedGoFiles(result.WrittenPaths) {
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

	logging.Get(logging.CategorySession).Warn(
		"Edits broke the build; giving the model one repair round with the compiler output")

	if trp == nil {
		return nil, nil, fmt.Errorf(
			"%w: edits broke the build and no repair is possible (client cannot accept tool results):\n%s",
			ErrVerificationFailed, verification.Output)
	}

	repaired, repairErrs, wrote, err := e.repairRound(ctx, trp, systemPrompt, &history, toolDefs, cfg, result,
		buildRepairPrompt(verification.Output), false)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"%w: edits broke the build and the repair round failed (%v). Compiler output:\n%s",
			ErrVerificationFailed, err, verification.Output)
	}

	recheck := verifyBuild(ctx, workspace, nil)
	if recheck.Verdict() == VerifyFailed && !wrote {
		// The round read instead of editing. Observed 2026-09-11: handed
		// `"context" imported and not used` with the line number, the model
		// spent its round on twenty reads and no edit, and the turn ended on
		// a broken build. One more round, with the compiler output again and
		// reading closed: the output names the lines, and what the first
		// round read is in the transcript.
		logging.Get(logging.CategorySession).Warn(
			"Repair round read without editing; one more under the commit regime")
		second, errs, _, err := e.repairRound(ctx, trp, systemPrompt, &history, toolDefs, cfg, result,
			buildRepairPrompt(recheck.Output)+"\n\n"+workingRegimeText(commitRegime), true)
		repairErrs = append(repairErrs, errs...)
		if err != nil {
			return nil, repairErrs, fmt.Errorf(
				"%w: edits broke the build and the second repair round failed (%v). Compiler output:\n%s",
				ErrVerificationFailed, err, recheck.Output)
		}
		repaired = second
		recheck = verifyBuild(ctx, workspace, nil)
	}
	switch recheck.Verdict() {
	case VerifyPassed:
		// An affirmative pass on the final workspace clears the failure.
		result.BuildCheck = recheck
		logging.Get(logging.CategorySession).Info("Build repaired successfully after one round")
		return repaired, repairErrs, nil
	case VerifyFailed:
		result.BuildCheck = recheck
		return nil, repairErrs, fmt.Errorf(
			"%w: edits broke the build and the repair round did not fix it. Compiler output:\n%s",
			ErrVerificationFailed, recheck.Output)
	case VerifyCanceled:
		return nil, repairErrs, fmt.Errorf("build re-verification canceled: %w", context.Canceled)
	default: // VerifyIndeterminate, VerifySkipped
		// The recheck produced no verdict: the original failure stands until
		// an affirmative pass clears it. The turn completes unverified — a
		// timeout is not proof of recovery — and closeChangeEvidence gets
		// the final word on the workspace with a fresh check.
		logging.Get(logging.CategorySession).Warn(
			"Build re-verification produced no verdict (%s); original failure retained, recovery NOT verified",
			recheck.Verdict())
		return repaired, repairErrs, nil
	}
}

// verifyAndRepairTests runs the tests for the packages this turn touched and,
// when they fail, gives the model one repair round with the test output in
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
	if !e.configSnapshot().VerifyTestsAfterEdits {
		return nil, nil, nil
	}
	if result == nil || result.SuccessfulWriteTools == 0 || !touchedGoFiles(result.WrittenPaths) {
		return nil, nil, nil
	}

	workspace := e.workspaceForVerification()
	packages := packagesForPaths(result.WrittenPaths)

	if untested := untestedWithoutCoverageOnDisk(workspace, result.WrittenPaths); len(untested) > 0 {
		logging.Get(logging.CategorySession).Warn(
			"Turn wrote production Go with no test alongside it: %s", strings.Join(untested, ", "))
		result.UntestedPaths = untested
	}

	verification, uncovered := verifyTestsWithCoverage(ctx, workspace, packages, result.WrittenPaths)

	// The profile is file-level, so without this a one-line edit in a large
	// file reports every uncovered block of the file as code the turn wrote
	// (observed 2026-09-11: 59 blocks for one line); a file with no pre-write
	// snapshot keeps all its blocks.
	if len(uncovered) > 0 && len(result.PreWriteContents) > 0 {
		changed := make(map[string][]LineRange, len(result.PreWriteContents))
		for path, before := range result.PreWriteContents {
			data, readErr := os.ReadFile(filepath.Join(workspace, filepath.FromSlash(path)))
			if readErr != nil {
				continue
			}
			changed[path] = changedLines(before, string(data))
		}
		uncovered = blocksInChangedLines(uncovered, changed)
	}
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
	case VerifyPassed, VerifySkipped:
		return nil, nil, nil
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

	logging.Get(logging.CategorySession).Warn(
		"Edits broke the tests; giving the model one repair round with the test output")

	if trp == nil {
		return nil, nil, fmt.Errorf(
			"%w: edits broke the tests and no repair is possible (client cannot accept tool results):\n%s",
			ErrVerificationFailed, verification.Output)
	}

	repaired, repairErrs, wrote, err := e.repairRound(ctx, trp, systemPrompt, &history, toolDefs, cfg, result,
		testRepairPrompt(verification.Output), false)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"%w: edits broke the tests and the repair round failed (%v). Test output:\n%s",
			ErrVerificationFailed, err, verification.Output)
	}

	// A test repair can break the build, so re-check both, cheapest first.
	// Only an affirmative failure verdict fails here: a recheck that
	// produced no verdict cannot prove the repair broke anything.
	if recheckBuild := verifyBuild(ctx, workspace, nil); recheckBuild.Verdict() == VerifyFailed {
		result.BuildCheck = recheckBuild
		return nil, repairErrs, fmt.Errorf(
			"%w: the test repair round broke the build. Compiler output:\n%s",
			ErrVerificationFailed, recheckBuild.Output)
	} else if recheckBuild.Verdict() == VerifyCanceled {
		return nil, repairErrs, fmt.Errorf("build re-verification canceled: %w", context.Canceled)
	}
	recheck := verifyTests(ctx, workspace, packagesForPaths(result.WrittenPaths))
	if recheck.Verdict() == VerifyFailed && !wrote {
		// Same escalation as the build repair: a round that only read gets
		// one more with reading closed.
		logging.Get(logging.CategorySession).Warn(
			"Test repair round read without editing; one more under the commit regime")
		second, errs, _, err := e.repairRound(ctx, trp, systemPrompt, &history, toolDefs, cfg, result,
			testRepairPrompt(recheck.Output)+"\n\n"+workingRegimeText(commitRegime), true)
		repairErrs = append(repairErrs, errs...)
		if err != nil {
			return nil, repairErrs, fmt.Errorf(
				"%w: edits broke the tests and the second repair round failed (%v). Test output:\n%s",
				ErrVerificationFailed, err, recheck.Output)
		}
		repaired = second
		if recheckBuild := verifyBuild(ctx, workspace, nil); recheckBuild.Verdict() == VerifyFailed {
			result.BuildCheck = recheckBuild
			return nil, repairErrs, fmt.Errorf(
				"%w: the test repair round broke the build. Compiler output:\n%s",
				ErrVerificationFailed, recheckBuild.Output)
		} else if recheckBuild.Verdict() == VerifyCanceled {
			return nil, repairErrs, fmt.Errorf("build re-verification canceled: %w", context.Canceled)
		}
		recheck = verifyTests(ctx, workspace, packagesForPaths(result.WrittenPaths))
	}
	switch recheck.Verdict() {
	case VerifyPassed:
		// An affirmative pass on the final workspace clears the failure.
		result.TestCheck = recheck
		logging.Get(logging.CategorySession).Info("Tests repaired successfully after one round")
		return repaired, repairErrs, nil
	case VerifyFailed:
		result.TestCheck = recheck
		return nil, repairErrs, fmt.Errorf(
			"%w: edits broke the tests and the repair round did not fix them. Test output:\n%s",
			ErrVerificationFailed, recheck.Output)
	case VerifyCanceled:
		return nil, repairErrs, fmt.Errorf("test re-verification canceled: %w", context.Canceled)
	default: // VerifyIndeterminate, VerifySkipped
		// The recheck produced no verdict: the original failure stands until
		// an affirmative pass clears it. The turn completes unverified — a
		// timeout is not proof of recovery — and closeChangeEvidence gets
		// the final word on the workspace with a fresh check.
		logging.Get(logging.CategorySession).Warn(
			"Test re-verification produced no verdict (%s); original failure retained, recovery NOT verified",
			recheck.Verdict())
		return repaired, repairErrs, nil
	}
}

// testRepairPrompt is the turn handed back to the model when its edits broke
// the tests.
//
// It forbids the cheapest way out. An agent told only "the tests fail" will
// often delete or weaken the assertion, which turns red green while destroying
// the thing that made the suite worth running. The failing test is the
// specification until proven otherwise.
func testRepairPrompt(testOutput string) string {
	return "Your edits compile but the tests fail. This is the test output:\n\n" +
		"```\n" + testOutput + "\n```\n\n" +
		"Fix the code so these tests pass, then stop. Do not explain, do not summarise, " +
		"and do not report success — the tests will be run again.\n\n" +
		"Do NOT delete the failing test, weaken its assertion, skip it, or change what it " +
		"expects in order to make it pass. The test states the required behaviour; your code " +
		"does not meet it yet. If — and only if — you can show the test itself asserts something " +
		"incorrect, say so explicitly and explain why before changing it.\n\n" +
		"Read the failing test and the code under test before editing either."
}

// buildRepairPrompt is the turn handed back to the model when its edits broke
// the build.
//
// It states the failure as fact and asks for a fix, rather than asking whether
// one is needed — the compiler has already decided. It also names the specific
// mistakes seen in the live failure, because those are the ones an editing
// agent actually makes: a stale import left behind, a block pasted twice, and a
// call to a helper that was planned but never written.
func buildRepairPrompt(compilerOutput string) string {
	return "Your edits do not compile. This is the compiler's output:\n\n" +
		"```\n" + compilerOutput + "\n```\n\n" +
		"Fix every error above using the edit tools, then stop. Do not explain, do not " +
		"summarise, and do not report success — the build will be checked again.\n\n" +
		"Check specifically for the mistakes that produce these errors:\n" +
		"  - an import added for code you did not end up writing (\"imported and not used\")\n" +
		"  - a block inserted twice, re-declaring variables with := (\"no new variables on left side of :=\")\n" +
		"  - a call to a helper function you planned but never wrote (\"undefined: ...\")\n" +
		"The compiler names the file and line of each error; edit those lines."
}

// repairRound sends one repair prompt through the working request path, runs
// the batch the model answers with, and reports whether that batch wrote
// anything. Under commit the read tools are withheld from the catalog and a
// read asked for anyway is answered with the regime (see working_regime); the
// first round is open, so a model that wants one look at the reported lines
// gets it, and only a round that read without editing is followed by a closed
// one. The round's own calls and results are appended to history so the next
// round sees them.
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
) (*types.LLMToolResponse, []string, bool, error) {
	*history = append(*history, types.Message{Role: "user", Text: prompt})
	if commit {
		defer e.enterCommitRegime(ctx)()
	}
	repaired, err := e.completeWithWorkingContext(ctx, trp, systemPrompt, *history, toolDefs)
	if err != nil {
		return nil, nil, false, err
	}
	before := 0
	if result != nil {
		before = result.SuccessfulWriteTools
	}
	var repairErrs []string
	if repaired != nil && len(repaired.ToolCalls) > 0 {
		results, errs := e.executeToolBatch(ctx, repaired.ToolCalls, cfg, result)
		repairErrs = append(repairErrs, errs...)
		*history = append(*history,
			types.Message{Role: "assistant", Text: repaired.Text, ToolCalls: repaired.ToolCalls},
			types.Message{Role: "user", ToolResults: results})
	}
	wrote := result != nil && result.SuccessfulWriteTools > before
	return repaired, repairErrs, wrote, nil
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
// Only high and medium findings trigger the round. Low-severity output from a
// reviewer asked to look hard at code is mostly style, and paying a full model
// round for it on every write turn is how a useful signal turns into a tax.
func (e *Executor) verifyAndUpliftWithCritic(
	ctx context.Context,
	trp types.ToolResultsProvider,
	systemPrompt string,
	history []types.Message,
	toolDefs []types.ToolDefinition,
	cfg *jitconfig.EffectiveAgentRuntimeConfig,
	result *ExecutionResult,
) ([]string, error) {
	if !e.configSnapshot().CriticReviewAfterEdits {
		return nil, nil
	}
	if result == nil || result.SuccessfulWriteTools == 0 || !touchedGoFiles(result.WrittenPaths) {
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

	prompt := buildCriticPrompt(files, grounding)

	// Bound the review independently of the turn.
	//
	// Observed live on the first run of this gate: the critic call started
	// (prompt_len=11407) and had not returned twenty minutes later, with no log
	// line after it — the turn had finished its work, passed the build and test
	// gates, and was then held open indefinitely by an advisory review. The
	// client's own timeout did not save it.
	//
	// An advisory gate that cannot fail a turn but CAN hang one is worse than no
	// gate at all: the failure is invisible and unbounded. A review that has not
	// come back in criticTimeout is abandoned and the turn proceeds without it,
	// which is precisely the behaviour "advisory" is supposed to mean.
	criticCtx, cancelCritic := context.WithTimeout(ctx, criticTimeout)
	defer cancelCritic()

	response, err := client.CompleteWithSystem(criticCtx, criticSystemPrompt, prompt)
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

	worth := findingsWorthUplift(findings)
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
	upliftCtx, cancelUplift := context.WithTimeout(ctx, criticUpliftTimeout)
	defer cancelUplift()

	uplifted, err := trp.CompleteWithToolResults(upliftCtx, systemPrompt, history, toolDefs)
	if err != nil {
		logging.Get(logging.CategorySession).Warn("uplift round failed (%v); turn continues", err)
		return nil, nil
	}

	var upliftErrs []string
	if uplifted != nil && len(uplifted.ToolCalls) > 0 {
		_, errs := e.executeToolBatch(ctx, uplifted.ToolCalls, cfg, result)
		upliftErrs = append(upliftErrs, errs...)

		// Re-verify. The uplift round makes real edits, and it runs AFTER the
		// build and test gates have already had their turn — so without this,
		// code written here is the only code in the whole loop that ships
		// unverified. That is precisely the false success the stack exists to
		// prevent, reintroduced at the last step.
		//
		// This does not contradict "the critic can never fail a turn". The
		// critic's OPINION is advisory: a hallucinated finding must not fail
		// anything. Its EDITS are not privileged — they answer to the compiler
		// and the test runner like every other edit. Acting on a wrong finding
		// and breaking the build is a real break, whoever suggested it.
		//
		// Every re-verdict is stored: the uplift edits came after the gates'
		// passes, so those passes no longer describe this workspace. A pass
		// refreshes them; a failure fails the turn; a timeout invalidates
		// them to indeterminate rather than leaving a stale green behind.
		// closeChangeEvidence then re-verifies the final workspace anyway.
		if verification := verifyBuild(ctx, workspace, nil); verification.Verdict() == VerifyFailed {
			result.BuildCheck = verification
			return upliftErrs, fmt.Errorf(
				"%w: the adversarial review's uplift round broke the build. Compiler output:\n%s",
				ErrVerificationFailed, verification.Output)
		} else if verification.Verdict() == VerifyCanceled {
			return upliftErrs, fmt.Errorf("uplift build re-verification canceled: %w", context.Canceled)
		} else {
			result.BuildCheck = verification
			if verification.Verdict() == VerifyIndeterminate {
				logging.Get(logging.CategorySession).Warn(
					"Uplift build re-verification timed out; prior pass invalidated, recovery NOT verified")
			}
		}
		if tv := verifyTests(ctx, workspace, packagesForPaths(result.WrittenPaths)); tv.Verdict() == VerifyFailed {
			result.TestCheck = tv
			return upliftErrs, fmt.Errorf(
				"%w: the adversarial review's uplift round broke the tests. Test output:\n%s",
				ErrVerificationFailed, tv.Output)
		} else if tv.Verdict() == VerifyCanceled {
			return upliftErrs, fmt.Errorf("uplift test re-verification canceled: %w", context.Canceled)
		} else {
			result.TestCheck = tv
			if tv.Verdict() == VerifyIndeterminate {
				logging.Get(logging.CategorySession).Warn(
					"Uplift test re-verification timed out; prior pass invalidated, recovery NOT verified")
			}
		}
	}
	return upliftErrs, nil
}

// criticTimeout bounds the adversarial review call.
//
// Three minutes is deliberately shorter than the client's own ceiling. The
// review is the least important of the three gates and the only one whose
// backend can stall without erroring; the cost of abandoning it is one missing
// opinion, while the cost of waiting is the whole turn.
const criticTimeout = 3 * time.Minute

// criticUpliftTimeout bounds the follow-up round for the same reason. It is
// longer than the review itself because this round makes real edits.
const criticUpliftTimeout = 5 * time.Minute

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
