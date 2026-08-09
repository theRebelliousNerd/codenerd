package session

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"codenerd/internal/build"
	"codenerd/internal/config"
	jitconfig "codenerd/internal/jit/config"
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

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

// buildVerifyTimeout bounds the verification build. A cold `go build ./...` on
// this repo takes tens of seconds; the ceiling is generous because a verify
// that times out reports a false alarm, which is worse than a slow one.
const buildVerifyTimeout = 4 * time.Minute

// buildVerifyMaxOutput caps how much compiler output is fed back to the model.
// Go reports errors newest-package-first and a broken edit usually produces a
// handful; a runaway cascade would otherwise blow the context budget the repair
// round needs.
const buildVerifyMaxOutput = 6000

// BuildVerification is the outcome of compiling the workspace after edits.
type BuildVerification struct {
	// Ran is false when verification was skipped (no Go files touched, no Go
	// toolchain, verification disabled). A skipped verification is NOT a pass
	// and must never be reported as one.
	Ran bool

	// OK is true only when the build actually succeeded.
	OK bool

	// Output is the compiler's stderr/stdout, truncated. Empty on success.
	Output string

	// Duration is how long the build took.
	Duration time.Duration
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
		return ws
	}
	if root, err := config.FindWorkspaceRoot(); err == nil && strings.TrimSpace(root) != "" {
		return root
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

	if strings.TrimSpace(workspace) == "" {
		return BuildVerification{Ran: false}
	}
	if _, err := exec.LookPath("go"); err != nil {
		logging.Get(logging.CategorySession).Warn(
			"build verification skipped: no Go toolchain on PATH (%v)", err)
		return BuildVerification{Ran: false}
	}

	// Bound the build independently of the turn's remaining budget: a verify
	// that inherits an almost-expired context reports a spurious failure.
	buildCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), buildVerifyTimeout)
	defer cancel()

	cmd := exec.CommandContext(buildCtx, "go", "build", "./...")
	cmd.Dir = workspace
	cmd.Env = build.GetBuildEnv(userCfg, workspace)

	out, err := cmd.CombinedOutput()
	elapsed := time.Since(start)

	if err == nil {
		logging.SessionDebug("build verification passed in %s", elapsed.Round(time.Millisecond))
		return BuildVerification{Ran: true, OK: true, Duration: elapsed}
	}

	if buildCtx.Err() != nil {
		// A timeout is not evidence the code is broken. Report it as "did not
		// run" so the turn is not failed on a verification that never finished.
		logging.Get(logging.CategorySession).Warn(
			"build verification timed out after %s; treating as not run", buildVerifyTimeout)
		return BuildVerification{Ran: false, Duration: elapsed}
	}

	text := strings.TrimSpace(string(out))
	if text == "" {
		text = err.Error()
	}
	if len(text) > buildVerifyMaxOutput {
		text = text[:buildVerifyMaxOutput] + "\n... (compiler output truncated)"
	}

	logging.Get(logging.CategorySession).Warn(
		"build verification FAILED in %s:\n%s", elapsed.Round(time.Millisecond), text)

	return BuildVerification{Ran: true, OK: false, Output: text, Duration: elapsed}
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
// repair was needed, and an error when the build is still broken.
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
	if !verification.Ran || verification.OK {
		return nil, nil, nil
	}

	logging.Get(logging.CategorySession).Warn(
		"Edits broke the build; giving the model one repair round with the compiler output")

	if trp == nil {
		return nil, nil, fmt.Errorf(
			"edits broke the build and no repair is possible (client cannot accept tool results):\n%s",
			verification.Output)
	}

	history = append(history, types.Message{Role: "user", Text: buildRepairPrompt(verification.Output)})

	repaired, err := trp.CompleteWithToolResults(ctx, systemPrompt, history, toolDefs)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"edits broke the build and the repair round failed (%v). Compiler output:\n%s", err, verification.Output)
	}

	var repairErrs []string
	if repaired != nil && len(repaired.ToolCalls) > 0 {
		_, errs := e.executeToolBatch(ctx, repaired.ToolCalls, cfg, result)
		repairErrs = append(repairErrs, errs...)
	}

	recheck := verifyBuild(ctx, workspace, nil)
	if recheck.Ran && !recheck.OK {
		return nil, repairErrs, fmt.Errorf(
			"edits broke the build and the repair round did not fix it. Compiler output:\n%s", recheck.Output)
	}

	logging.Get(logging.CategorySession).Info("Build repaired successfully after one round")
	return repaired, repairErrs, nil
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

	if !verification.Ran || verification.OK {
		return nil, nil, nil
	}

	logging.Get(logging.CategorySession).Warn(
		"Edits broke the tests; giving the model one repair round with the test output")

	if trp == nil {
		return nil, nil, fmt.Errorf(
			"edits broke the tests and no repair is possible (client cannot accept tool results):\n%s",
			verification.Output)
	}

	history = append(history, types.Message{Role: "user", Text: testRepairPrompt(verification.Output)})

	repaired, err := trp.CompleteWithToolResults(ctx, systemPrompt, history, toolDefs)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"edits broke the tests and the repair round failed (%v). Test output:\n%s", err, verification.Output)
	}

	var repairErrs []string
	if repaired != nil && len(repaired.ToolCalls) > 0 {
		_, errs := e.executeToolBatch(ctx, repaired.ToolCalls, cfg, result)
		repairErrs = append(repairErrs, errs...)
	}

	// A test repair can break the build, so re-check both, cheapest first.
	if recheckBuild := verifyBuild(ctx, workspace, nil); recheckBuild.Ran && !recheckBuild.OK {
		return nil, repairErrs, fmt.Errorf(
			"the test repair round broke the build. Compiler output:\n%s", recheckBuild.Output)
	}
	recheck := verifyTests(ctx, workspace, packagesForPaths(result.WrittenPaths))
	if recheck.Ran && !recheck.OK {
		return nil, repairErrs, fmt.Errorf(
			"edits broke the tests and the repair round did not fix them. Test output:\n%s", recheck.Output)
	}

	logging.Get(logging.CategorySession).Info("Tests repaired successfully after one round")
	return repaired, repairErrs, nil
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
		"Read the file around each reported line before editing it."
}

// verifyAndUpliftWithCritic runs one adversarial review of the code this turn
// wrote and, when it reports something worth acting on, gives the model one
// round to respond.
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
) (*types.LLMToolResponse, []string, error) {
	if !e.configSnapshot().CriticReviewAfterEdits {
		return nil, nil, nil
	}
	if result == nil || result.SuccessfulWriteTools == 0 || !touchedGoFiles(result.WrittenPaths) {
		return nil, nil, nil
	}

	workspace := e.workspaceForVerification()
	files := readWrittenFilesForReview(workspace, result.WrittenPaths)
	if len(files) == 0 {
		return nil, nil, nil
	}

	client := e.criticClient()
	if client == nil {
		return nil, nil, nil
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
		return nil, nil, nil
	}

	findings := parseCriticFindings(response)
	result.CriticFindings = findings

	// One line carrying every gate's verdict. Reaching this point means the
	// build and tests already passed — the two hard gates return early
	// otherwise — so those are true by construction here.
	logging.Get(logging.CategorySession).Info("turn signals: %s",
		SummarizeTurnSignals(true, true, len(result.UncoveredBlocks), len(findings)))

	if len(findings) == 0 {
		return nil, nil, nil
	}

	worth := findingsWorthUplift(findings)
	logging.Get(logging.CategorySession).Warn(
		"Adversarial review reported %d finding(s), %d worth acting on", len(findings), len(worth))
	if len(worth) == 0 {
		return nil, nil, nil
	}
	if trp == nil {
		logging.Get(logging.CategorySession).Warn(
			"Adversarial review found %d actionable item(s), but this provider has no repair channel", len(worth))
		return nil, nil, nil
	}

	history = append(history, types.Message{Role: "user", Text: formatUpliftPrompt(worth)})
	upliftCtx, cancelUplift := context.WithTimeout(ctx, criticUpliftTimeout)
	defer cancelUplift()

	uplifted, err := trp.CompleteWithToolResults(upliftCtx, systemPrompt, history, toolDefs)
	if err != nil {
		logging.Get(logging.CategorySession).Warn("uplift round failed (%v); turn continues", err)
		return nil, nil, nil
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
		if verification := verifyBuild(ctx, workspace, nil); verification.Ran && !verification.OK {
			return uplifted, upliftErrs, fmt.Errorf(
				"the adversarial review's uplift round broke the build. Compiler output:\n%s",
				verification.Output)
		}
		if tv := verifyTests(ctx, workspace, packagesForPaths(result.WrittenPaths)); tv.Ran && !tv.OK {
			return uplifted, upliftErrs, fmt.Errorf(
				"the adversarial review's uplift round broke the tests. Test output:\n%s", tv.Output)
		}
	}
	return uplifted, upliftErrs, nil
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
