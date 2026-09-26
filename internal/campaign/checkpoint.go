package campaign

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"codenerd/internal/build"
	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/session"
	"codenerd/internal/tactile"
	"codenerd/internal/testoutput"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// CheckpointRunner runs verification checkpoints for phases.
type CheckpointRunner struct {
	executor     tactile.Executor
	taskExecutor session.TaskExecutor
	workspace    string
	kernel       core.Kernel
	// commandTimeout bounds a checkpoint's build or test command:
	// campaign.checkpoint_command_timeout (SetCommandTimeout).
	commandTimeout time.Duration
}

// NewCheckpointRunner creates a new checkpoint runner. The kernel is where a
// review checkpoint's verdict is settled (checkpoint_verdict_outcome): a runner
// without one can run builds and tests, and fails every review closed.
func NewCheckpointRunner(executor tactile.Executor, taskExecutor session.TaskExecutor, workspace string, kernel core.Kernel) *CheckpointRunner {
	cr := &CheckpointRunner{
		executor:     executor,
		taskExecutor: taskExecutor,
		workspace:    workspace,
		kernel:       kernel,
	}
	// The config's default until the orchestrator sets the user's: one source.
	if def, err := config.DefaultCampaignConfig().Resolve(); err == nil {
		cr.commandTimeout = def.CheckpointCommandTimeout
	}
	return cr
}

// SetCommandTimeout takes the bound on a checkpoint's build or test command
// from the campaign policy.
func (cr *CheckpointRunner) SetCommandTimeout(d time.Duration) {
	if cr == nil {
		return
	}
	cr.commandTimeout = d
}

// spawnTask is the unified entry point for task execution.
func (cr *CheckpointRunner) spawnTask(ctx context.Context, intent string, task string) (string, error) {
	if cr.taskExecutor == nil {
		return "", fmt.Errorf("taskExecutor not initialized")
	}
	req := session.TaskRequest{
		IntentVerb: intent,
		Task:       task,
	}
	return cr.taskExecutor.Execute(ctx, req)
}

// Run executes a checkpoint based on the verification method.
func (cr *CheckpointRunner) Run(ctx context.Context, phase *Phase, method VerificationMethod) (passed bool, details string, err error) {
	phaseName := ""
	if phase != nil {
		phaseName = phase.Name
	}
	logging.Campaign("CheckpointRunner.Run: executing method=%s phase=%s", method, phaseName)

	switch method {
	case VerifyTestsPass:
		passed, details, err = cr.runTestsCheckpoint(ctx)
	case VerifyBuilds:
		passed, details, err = cr.runBuildCheckpoint(ctx)
	case VerifyManualReview:
		passed, details, err = cr.runManualReviewCheckpoint(ctx, phase)
	case VerifyShardValidate:
		passed, details, err = cr.runShardValidationCheckpoint(ctx, phase)
	case VerifyNemesisGauntlet:
		passed, details, err = cr.runNemesisGauntletCheckpoint(ctx, phase)
	case VerifyNone:
		logging.CampaignDebug("CheckpointRunner.Run: no verification required for phase=%s", phaseName)
		return true, "No verification required", nil
	default:
		// Fail closed: a method this runner cannot run has not been checked,
		// and "we did not check" must never read as "passed" (it used to:
		// "Unknown verification method, skipping"). The policy names such a
		// phase too: campaign_blocked(C, /unverifiable_objective).
		logging.CampaignWarn("CheckpointRunner.Run: unknown verification method=%s; the phase cannot be verified", method)
		return false, fmt.Sprintf("Unknown verification method %s: it cannot be run, so the phase is unverified", method),
			fmt.Errorf("unknown verification method %q", method)
	}

	if err != nil {
		logging.CampaignError("CheckpointRunner.Run: method=%s phase=%s error: %v", method, phaseName, err)
	} else if passed {
		logging.Campaign("CheckpointRunner.Run: method=%s phase=%s PASSED", method, phaseName)
	} else {
		logging.CampaignWarn("CheckpointRunner.Run: method=%s phase=%s FAILED", method, phaseName)
	}
	return passed, details, err
}

// runTestsCheckpoint runs tests and checks if they pass.
func (cr *CheckpointRunner) runTestsCheckpoint(ctx context.Context) (bool, string, error) {
	// Detect project type and run appropriate test command
	testCmdStr := cr.detectTestCommand()
	logging.CampaignDebug("runTestsCheckpoint: detected command=%s workspace=%s", testCmdStr, cr.workspace)
	isGoTest := strings.HasPrefix(testCmdStr, "go test")
	isNpmTest := strings.HasPrefix(testCmdStr, "npm test")
	if isGoTest && !strings.Contains(testCmdStr, "-json") {
		testCmdStr = testCmdStr + " -json"
	}
	if isNpmTest && !strings.Contains(testCmdStr, "--") {
		// Try to request JSON where supported (e.g., jest). This is best-effort.
		testCmdStr = testCmdStr + " -- --json --outputFile=.nerd/npm-test.json"
	}
	// Workspace build tags: without them the gate judges a different build
	// than dev/CI in tag-gated trees. No-op elsewhere.
	if tags := build.TestTagsForWorkspace(cr.workspace); len(tags) > 0 && strings.HasPrefix(testCmdStr, "go test") {
		testCmdStr = "go test " + strings.Join(tags, " ") + strings.TrimPrefix(testCmdStr, "go test")
	}
	parts := strings.Fields(testCmdStr)

	cmd := tactile.Command{
		Binary:           parts[0],
		Arguments:        parts[1:],
		WorkingDirectory: cr.workspace,
		// Build env carries the CGO flags the tagged build needs; without
		// them the tag above would turn the gate red on missing headers.
		Environment: build.GetBuildEnv(nil, cr.workspace),
		Limits: &tactile.ResourceLimits{
			TimeoutMs: cr.commandTimeout.Milliseconds(),
		},
	}

	res, err := cr.executor.Execute(ctx, cmd)
	output := ""
	if res != nil {
		output = res.Output()
	}
	if err != nil {
		// Test failures return non-zero exit code
		return false, fmt.Sprintf("Error running tests: %v\n%s", err, output), nil
	}

	// Check for non-zero exit code which definitely indicates failure
	// DirectExecutor returns Success=true but ExitCode!=0 for command failures
	hasExitError := res != nil && res.ExitCode != 0

	// Count passed/failed from output
	if isGoTest {
		passedCount, failedCount, duration := cr.parseGoTestJSON(output)
		if failedCount > 0 || hasExitError {
			// If we detected exit error but no failed tests counted, likely a build error or catastrophic failure
			if failedCount == 0 && hasExitError {
				return false, fmt.Sprintf("Tests failed (exit code %d) - likely build error\n%s", res.ExitCode, output), nil
			}
			return false, fmt.Sprintf("Tests: %d passed, %d failed (%.2fs)\n%s", passedCount, failedCount, duration.Seconds(), output), nil
		}
		return true, fmt.Sprintf("All %d tests passed (%.2fs)", passedCount, duration.Seconds()), nil
	}

	if isNpmTest {
		passedCount, failedCount := cr.parseTestOutput(output)
		// Also try to read the JSON file if it exists
		jsonPath := filepath.Join(cr.workspace, ".nerd", "npm-test.json")
		if data, err := os.ReadFile(jsonPath); err == nil {
			p, f := cr.parseJestJSON(data)
			if p+f > 0 {
				passedCount, failedCount = p, f
			}
		}
		if failedCount > 0 || hasExitError {
			return false, fmt.Sprintf("Tests: %d passed, %d failed\n%s", passedCount, failedCount, output), nil
		}
		return true, fmt.Sprintf("All %d tests passed", passedCount), nil
	}

	passedCount, failedCount := cr.parseTestOutput(output)
	if failedCount > 0 || hasExitError {
		return false, fmt.Sprintf("Tests: %d passed, %d failed\n%s", passedCount, failedCount, output), nil
	}
	return true, fmt.Sprintf("All %d tests passed", passedCount), nil
}

// runBuildCheckpoint runs the build and checks if it succeeds.
func (cr *CheckpointRunner) runBuildCheckpoint(ctx context.Context) (bool, string, error) {
	buildCmdStr := cr.detectBuildCommand()
	logging.CampaignDebug("runBuildCheckpoint: detected command=%s workspace=%s", buildCmdStr, cr.workspace)
	parts := strings.Fields(buildCmdStr)

	cmd := tactile.Command{
		Binary:           parts[0],
		Arguments:        parts[1:],
		WorkingDirectory: cr.workspace,
		Limits: &tactile.ResourceLimits{
			TimeoutMs: cr.commandTimeout.Milliseconds(),
		},
	}

	res, err := cr.executor.Execute(ctx, cmd)
	output := ""
	if res != nil {
		output = res.Output()
	}
	if err != nil {
		logging.CampaignWarn("runBuildCheckpoint: build failed: %v (output_len=%d)", err, len(output))
		return false, fmt.Sprintf("Build failed:\n%s", output), nil
	}

	if res != nil && res.ExitCode != 0 {
		logging.CampaignWarn("runBuildCheckpoint: build failed with exit code %d", res.ExitCode)
		return false, fmt.Sprintf("Build failed (exit code %d):\n%s", res.ExitCode, output), nil
	}

	logging.CampaignDebug("runBuildCheckpoint: build succeeded")
	return true, "Build succeeded", nil
}

// runManualReviewCheckpoint escalates to automated verification in non-interactive mode.
//
// Manual review requires a human. In non-interactive mode there is no human
// to consult, so this checkpoint cannot verify by asking a reviewer.
// Previously it returned PASSED with a "skipped" note, which violated the core
// invariant: a checkpoint that did not verify must never report PASSED. That
// made "we did not check" indistinguishable from "we checked and it was fine" —
// the single most dangerous answer a verification gate can give, and one that
// survived precisely because it was silent (see the fail-closed comment on
// runShardValidationCheckpoint ten lines below). A fabricated audit campaign
// completed with 5/5 phases PASSED while citing symbols that do not exist, with
// every /manual_review gate logging "Checkpoint PASSED" having verified nothing.
//
// Rather than simply returning false — which would block every campaign whose
// decomposer chose /manual_review, which is most of them — this checkpoint
// escalates to the verification that can actually run:
// cr.runShardValidationCheckpoint(ctx, phase), which spawns a reviewer shard and
// inspects the phase's objectives and completed tasks. That function already
// fails closed when cr.taskExecutor is nil, so the unverifiable case is handled
// correctly without new logic. The returned details are prefixed so the
// escalation is visible in the log and in the persisted checkpoint record.
func (cr *CheckpointRunner) runManualReviewCheckpoint(ctx context.Context, phase *Phase) (bool, string, error) {
	// Check for cancellation
	select {
	case <-ctx.Done():
		return false, "", ctx.Err()
	default:
	}

	passed, details, err := cr.runShardValidationCheckpoint(ctx, phase)
	prefix := fmt.Sprintf("Manual review requested for phase '%s' but no human was present (non-interactive mode); escalated to shard validation: ", phase.Name)
	return passed, prefix + details, err
}

// runShardValidationCheckpoint spawns a reviewer shard to validate the phase.
func (cr *CheckpointRunner) runShardValidationCheckpoint(ctx context.Context, phase *Phase) (bool, string, error) {
	if cr.taskExecutor == nil {
		// Fail closed. This used to return PASS, which made "we did not check"
		// indistinguishable from "we checked and it was fine" — the single most
		// dangerous answer a verification gate can give, and one that survived
		// precisely because it was silent. A checkpoint that cannot run has not
		// been satisfied, so it does not pass.
		logging.CampaignWarn("runShardValidationCheckpoint: no task executor for phase=%s; failing the checkpoint because it cannot be verified", phase.Name)
		return false, "Shard validation could not run — no task executor is wired into this orchestrator, so the phase is unverified. Construct the orchestrator with OrchestratorConfig.TaskExecutor.", nil
	}

	logging.Campaign("runShardValidationCheckpoint: spawning reviewer shard for phase=%s", phase.Name)

	// Build a review prompt based on phase objectives and completed tasks
	var reviewPrompt strings.Builder
	reviewPrompt.WriteString("Review the following phase completion for quality and correctness:\n\n")
	reviewPrompt.WriteString(fmt.Sprintf("Phase: %s\n\n", phase.Name))

	reviewPrompt.WriteString("Objectives:\n")
	for _, obj := range phase.Objectives {
		reviewPrompt.WriteString(fmt.Sprintf("- %s\n", obj.Description))
	}

	reviewPrompt.WriteString("\nCompleted Tasks:\n")
	for _, task := range phase.Tasks {
		if task.Status == TaskCompleted {
			reviewPrompt.WriteString(fmt.Sprintf("- [DONE] %s\n", task.Description))
			if len(task.Artifacts) > 0 {
				reviewPrompt.WriteString(fmt.Sprintf("  Artifacts: %v\n", task.Artifacts))
			}
		}
	}

	key := verdictKey(phase)
	reviewPrompt.WriteString("\nYour response MUST be a JSON control-packet carrying exactly one checkpoint_verdict/4 fact in control_packet.mangle_updates:\n")
	reviewPrompt.WriteString("checkpoint_verdict(\"PhaseKey\", Verdict, \"reason\", Confidence).\n")
	reviewPrompt.WriteString(fmt.Sprintf("PhaseKey must be exactly %q. ", key))
	reviewPrompt.WriteString("Verdict must be /pass (objectives met) or /fail (objectives not met). Reason is a short human-readable justification. Confidence is an integer percent 0-100.\n")
	reviewPrompt.WriteString("The atom must end with a period; it is asserted into the kernel as a fact.\n")
	reviewPrompt.WriteString("Example: {\"control_packet\": {\"mangle_updates\": [\"checkpoint_verdict(\\\"my-phase\\\", /pass, \\\"all objectives met\\\", 95).\"]}, \"surface_response\": \"...\"}.\n")
	reviewPrompt.WriteString("Free-text PASS/FAIL is not accepted; only checkpoint_verdict/4 decides.")

	// Retract any pre-existing verdict for this phase before spawning the
	// reviewer so a task shard cannot pre-approve its own phase. Best effort.
	if cr != nil && cr.kernel != nil && phase != nil {
		_ = cr.kernel.RetractFact(core.Fact{Predicate: "checkpoint_verdict", Args: []any{key}})
		logging.CampaignDebug("runShardValidationCheckpoint: retracted stale checkpoint_verdict for phase=%s before spawn", key)
	}

	// Spawn reviewer intent
	result, err := cr.spawnTask(ctx, "/review", reviewPrompt.String())
	if err != nil {
		logging.CampaignError("runShardValidationCheckpoint: reviewer shard failed for phase=%s: %v", phase.Name, err)
		return false, fmt.Sprintf("Reviewer shard failed: %v", err), err
	}

	return settledCheckpointResult(cr.kernel, key, fmt.Sprintf("%v", result), "Review")
}

// settledCheckpointResult turns the kernel's derived verdict for key into the
// checkpoint's result. A missing or malformed checkpoint_verdict/4 fails
// closed: no derived outcome is not a pass.
func settledCheckpointResult(kernel core.Kernel, key, raw, label string) (bool, string, error) {
	verdict, ok := settleCheckpointVerdict(kernel, key, raw)
	if !ok {
		logging.CampaignWarn("%s verdict could not be determined for phase=%s; failing closed", label, key)
		return false, fmt.Sprintf("%s verdict could not be determined (missing or malformed checkpoint_verdict/4 for phase %q): the reviewer's control packet carried no checkpoint_verdict/4 for this phase:\n%s", label, key, raw), nil
	}
	logging.Campaign("%s verdict for phase=%s: %s", label, key, verdict.outcome)
	details := verdict.describe(label)
	if !verdict.passed() {
		// The verdict's reason is one line; the reviewer's report names the
		// files and lines. Until 2026-09-26 only the line was kept, so the
		// remediation it briefs could not see which files were at fault
		// (campaign 7b853890: "missing front-matter", in files it never named).
		if report := reviewerReport(raw); report != "" {
			details += "\n\nThe reviewer's report:\n" + report
		}
	}
	return verdict.passed(), details, nil
}

// reviewerReport is what a checkpoint's reviewer said: the envelope's
// surface_response when its reply is the control-packet envelope, the reply
// itself otherwise.
func reviewerReport(raw string) string {
	var env checkpointEnvelope
	if err := json.Unmarshal([]byte(raw), &env); err == nil {
		return strings.TrimSpace(env.Surface)
	}
	return strings.TrimSpace(raw)
}

// runNemesisGauntletCheckpoint spawns the Nemesis shard to perform adversarial review.
// The verdict is structured: only a well-formed checkpoint_verdict/4 for this
// phase in control_packet.mangle_updates decides; prose is inert and a
// missing or malformed verdict fails closed.
func (cr *CheckpointRunner) runNemesisGauntletCheckpoint(ctx context.Context, phase *Phase) (bool, string, error) {
	if cr.taskExecutor == nil {
		// Fail closed, as in runShardValidationCheckpoint. An assault campaign
		// exists to be adversarially verified; reporting that it survived a
		// gauntlet that never ran is the most misleading result this
		// orchestrator can produce.
		logging.CampaignWarn("runNemesisGauntletCheckpoint: no task executor; failing the checkpoint because the adversarial gauntlet cannot run")
		return false, "Nemesis gauntlet could not run — no task executor is wired into this orchestrator, so no adversarial verification was performed. Construct the orchestrator with OrchestratorConfig.TaskExecutor.", nil
	}

	phaseName := ""
	if phase != nil {
		phaseName = phase.Name
	}
	key := verdictKey(phase)
	logging.Campaign("runNemesisGauntletCheckpoint: spawning nemesis shard for phase=%s", phaseName)

	target := cr.workspace
	// Prefer a phase-specific target if artifacts exist.
	if phase != nil {
		for _, task := range phase.Tasks {
			for _, artifact := range task.Artifacts {
				if artifact.Path != "" {
					target = artifact.Path
					break
				}
			}
			if target != cr.workspace {
				break
			}
		}
	}

	var nemesisPrompt strings.Builder
	nemesisPrompt.WriteString("Perform an adversarial review of the following phase and target:\n\n")
	nemesisPrompt.WriteString(fmt.Sprintf("Phase: %s\n", phaseName))
	nemesisPrompt.WriteString(fmt.Sprintf("Target: %s\n\n", target))
	if phase != nil {
		nemesisPrompt.WriteString("Objectives:\n")
		for _, obj := range phase.Objectives {
			nemesisPrompt.WriteString(fmt.Sprintf("- %s\n", obj.Description))
		}
		nemesisPrompt.WriteString("\nCompleted Tasks:\n")
		for _, task := range phase.Tasks {
			if task.Status == TaskCompleted {
				nemesisPrompt.WriteString(fmt.Sprintf("- [DONE] %s\n", task.Description))
				if len(task.Artifacts) > 0 {
					nemesisPrompt.WriteString(fmt.Sprintf("  Artifacts: %v\n", task.Artifacts))
				}
			}
		}
		nemesisPrompt.WriteString("\n")
	}
	nemesisPrompt.WriteString("Attempt to break the implementation: find vulnerabilities, logic errors, and unhandled edge cases.\n")
	nemesisPrompt.WriteString("\nYour response MUST be a JSON control-packet carrying exactly one checkpoint_verdict/4 fact in control_packet.mangle_updates:\n")
	nemesisPrompt.WriteString("checkpoint_verdict(\"PhaseKey\", Verdict, \"reason\", Confidence).\n")
	nemesisPrompt.WriteString(fmt.Sprintf("PhaseKey must be exactly %q. ", key))
	nemesisPrompt.WriteString("Verdict must be /pass (survived the gauntlet, no exploitable weaknesses found) or /fail (gauntlet broke the implementation). Reason is a short human-readable justification. Confidence is an integer percent 0-100.\n")
	nemesisPrompt.WriteString("The atom must end with a period; it is asserted into the kernel as a fact.\n")
	nemesisPrompt.WriteString("Example: {\"control_packet\": {\"mangle_updates\": [\"checkpoint_verdict(\\\"my-phase\\\", /pass, \\\"no weaknesses found\\\", 95).\"]}, \"surface_response\": \"...\"}.\n")
	nemesisPrompt.WriteString("Free-text PASS/FAIL is not accepted; only checkpoint_verdict/4 decides.")

	// Retract any pre-existing verdict for this phase before spawning the
	// nemesis so a task shard cannot pre-approve its own phase. Best effort.
	if cr != nil && cr.kernel != nil {
		_ = cr.kernel.RetractFact(core.Fact{Predicate: "checkpoint_verdict", Args: []any{key}})
		logging.CampaignDebug("runNemesisGauntletCheckpoint: retracted stale checkpoint_verdict for phase=%s before spawn", key)
	}

	logging.CampaignDebug("runNemesisGauntletCheckpoint: target=%s", target)
	result, err := cr.spawnTask(ctx, "/nemesis", nemesisPrompt.String())
	if err != nil {
		logging.CampaignError("runNemesisGauntletCheckpoint: nemesis shard failed for phase=%s: %v", phaseName, err)
		return false, fmt.Sprintf("Nemesis shard failed: %v", err), err
	}

	return settledCheckpointResult(cr.kernel, key, fmt.Sprintf("%v", result), "Nemesis gauntlet")
}

// detectTestCommand delegates to the canonical tools.TestCommandForDir
// projection (internal/tools/framework.go, mirroring the policy
// test_framework/1 + test_command/1 facts) and falls back to the Go default
// on unknown workspaces.
func (cr *CheckpointRunner) detectTestCommand() string {
	if cmd, ok := tools.TestCommandForDir(cr.workspace); ok {
		return cmd
	}
	return tools.DefaultTestCommand
}

// detectBuildCommand delegates to the canonical tools.BuildCommandForDir
// projection (mirroring the policy build_command/1 facts) and falls back to
// the Go default on unknown workspaces.
func (cr *CheckpointRunner) detectBuildCommand() string {
	if cmd, ok := tools.BuildCommandForDir(cr.workspace); ok {
		return cmd
	}
	return tools.DefaultBuildCommand
}

// parseTestOutput counts passed and failed tests in a runner's plain output.
//
// The parser itself lives in internal/testoutput, because the chat model needs
// the same answer to summarize a tester shard and had grown its own version
// that could not produce one. This keeps the checkpoint runner's convention:
// unreadable output is optimistically one pass, so a runner the parser cannot
// read does not manufacture a failure that fails a campaign phase.
func (cr *CheckpointRunner) parseTestOutput(output string) (passed, failed int) {
	return testoutput.ParseOptimistic(output)
}

// parseGoTestJSON parses go test -json output for pass/fail counts.
// parseGoTestJSON counts `go test -json` results line by line.
//
// The stream is not pure JSON: the toolchain interleaves plain-text lines
// (`# pkg` build headers, `FAIL\tpkg [build failed]`, vet output). The old
// decoder abandoned JSON at the first such line and fed the whole stream to
// the text heuristic, whose "failed"/"error" substring rule then counted
// every test whose name contains "Error" — campaign 149c512d saw
// "3,584 failed (0.00s)" from a stream holding two real failures, and the
// phase advanced UNVERIFIED on that number. Now: skip non-JSON lines, count
// test-level results, count a package-level fail only when that package
// reported no failing test (build/setup failures), take duration from the
// package events (their Elapsed is the package wall time), and use the
// heuristic only when the stream carried no JSON at all.
func (cr *CheckpointRunner) parseGoTestJSON(output string) (passed, failed int, duration time.Duration) {
	type goTestEvent struct {
		Action  string  `json:"Action"`
		Package string  `json:"Package"`
		Test    string  `json:"Test"`
		Elapsed float64 `json:"Elapsed"`
	}

	sawJSON := false
	pkgHadTestFailure := map[string]bool{}
	pkgFailed := map[string]bool{}
	var pkgDuration, testDuration time.Duration
	for line := range strings.SplitSeq(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "{") {
			continue
		}
		var evt goTestEvent
		if err := json.Unmarshal([]byte(trimmed), &evt); err != nil {
			continue
		}
		sawJSON = true
		elapsed := time.Duration(evt.Elapsed * float64(time.Second))
		switch evt.Action {
		case "pass":
			if evt.Test != "" {
				passed++
				testDuration += elapsed
			} else {
				pkgDuration += elapsed
			}
		case "fail":
			if evt.Test != "" {
				failed++
				testDuration += elapsed
				pkgHadTestFailure[evt.Package] = true
			} else {
				pkgFailed[evt.Package] = true
				pkgDuration += elapsed
			}
		}
	}
	if !sawJSON {
		p, f := cr.parseTestOutput(output)
		return p, f, 0
	}
	for pkg := range pkgFailed {
		if !pkgHadTestFailure[pkg] {
			failed++ // build or setup failure: the package failed without running a test
		}
	}
	// Package events carry wall time; a stream with only test events (older
	// callers, partial captures) still reports the summed test time.
	duration = pkgDuration
	if duration == 0 {
		duration = testDuration
	}
	return passed, failed, duration
}

// parseJestJSON parses a Jest-style JSON report if available.
func (cr *CheckpointRunner) parseJestJSON(data []byte) (passed, failed int) {
	var report struct {
		NumPassedTests int `json:"numPassedTests"`
		NumFailedTests int `json:"numFailedTests"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		return 0, 0
	}
	return report.NumPassedTests, report.NumFailedTests
}

// RunAll runs all checkpoints for a phase.
func (cr *CheckpointRunner) RunAll(ctx context.Context, phase *Phase) ([]Checkpoint, error) {
	checkpoints := make([]Checkpoint, 0)

	for _, obj := range phase.Objectives {
		if obj.VerificationMethod == VerifyNone {
			continue
		}

		passed, details, err := cr.Run(ctx, phase, obj.VerificationMethod)
		if err != nil {
			return checkpoints, err
		}

		checkpoints = append(checkpoints, Checkpoint{
			Type:      string(obj.VerificationMethod),
			Passed:    passed,
			Details:   details,
			Timestamp: time.Now(),
		})
	}

	return checkpoints, nil
}

// RunQuick runs a quick sanity check (build only).
func (cr *CheckpointRunner) RunQuick(ctx context.Context) (bool, string, error) {
	return cr.runBuildCheckpoint(ctx)
}

// checkpointEnvelope is the minimal subset of the reviewer control-packet
// needed here: the mangle_updates list that carries checkpoint_verdict/4.
// A local struct avoids importing articulation (and any import cycle) for
// what is deliberately a tiny, stable contract.
type checkpointEnvelope struct {
	Control struct {
		MangleUpdates []string `json:"mangle_updates"`
	} `json:"control_packet"`
	Surface string `json:"surface_response"`
}

// verdictKey is how a checkpoint's reviewer names the phase in its
// checkpoint_verdict/4: the phase ID without its leading slash. Names are not
// unique (two phases may both be "Verification"), and a quoted string that
// starts with a slash never matches the name constant Go asserts for an ID,
// so the key is the ID the model can write as a plain string. A phase with no
// ID (hand-built in tests) falls back to its name.
func verdictKey(phase *Phase) string {
	if phase == nil {
		return ""
	}
	if id := strings.TrimPrefix(strings.TrimSpace(phase.ID), "/"); id != "" {
		return id
	}
	return phase.Name
}

// parseCheckpointVerdictFacts extracts the checkpoint_verdict/4 facts for key
// from a JSON control-packet envelope, for executors that return the envelope
// verbatim instead of asserting it. Only control_packet.mangle_updates entries
// that are well-formed checkpoint_verdict/4 atoms count; free text, bare atoms
// outside the envelope and prose PASS/FAIL are inert. It decides nothing: the
// facts go into the kernel, which derives the outcome.
func parseCheckpointVerdictFacts(resultStr, key string) []core.Fact {
	var env checkpointEnvelope
	if err := json.Unmarshal([]byte(resultStr), &env); err != nil {
		return nil
	}
	var facts []core.Fact
	for _, update := range env.Control.MangleUpdates {
		gotKey, verdict, reason, confidence, ok := parseCheckpointVerdictAtom(update)
		if !ok || gotKey != key {
			continue
		}
		facts = append(facts, core.Fact{
			Predicate: "checkpoint_verdict",
			Args:      []any{gotKey, types.MangleAtom("/" + verdict), reason, confidence},
		})
	}
	return facts
}

// parseCheckpointVerdictAtom parses a single mangle_updates entry as a
// structured checkpoint_verdict/4 fact:
//
//	checkpoint_verdict("Key", /pass|/fail, "details", confidence)
//
// The entry must be exactly the atom (modulo surrounding whitespace); the
// atom is never searched for inside a larger string. Verdict accepts /pass,
// "pass" or 'pass' spellings so JSON-quoted atoms still parse. Confidence must
// be an integer percent: the kernel compares integers only, and the policy
// weighs it (checkpoint_verdict_outcome). The key is returned verbatim so the
// caller can require it to match the checkpoint's.
func parseCheckpointVerdictAtom(atom string) (key, verdict, reason string, confidence int64, ok bool) {
	trimmed := strings.TrimSpace(atom)
	trimmed = strings.TrimSuffix(trimmed, ".")
	trimmed = strings.TrimSpace(trimmed)
	const prefix = "checkpoint_verdict("
	if !strings.HasPrefix(trimmed, prefix) || !strings.HasSuffix(trimmed, ")") {
		return "", "", "", 0, false
	}
	inner := trimmed[len(prefix) : len(trimmed)-1]
	parts := splitTopLevelCommas(inner)
	if len(parts) != 4 {
		return "", "", "", 0, false
	}

	keyUnquoted, err := strconv.Unquote(strings.TrimSpace(parts[0]))
	if err != nil {
		return "", "", "", 0, false
	}

	verdictPart := strings.TrimSpace(parts[1])
	verdictPart = strings.Trim(verdictPart, `"'`)
	verdictPart = strings.TrimSpace(verdictPart)
	verdictPart = strings.TrimPrefix(verdictPart, "/")
	verdictNorm := strings.ToLower(strings.TrimSpace(verdictPart))
	if verdictNorm != "pass" && verdictNorm != "fail" {
		return "", "", "", 0, false
	}

	reasonUnquoted, err := strconv.Unquote(strings.TrimSpace(parts[2]))
	if err != nil {
		return "", "", "", 0, false
	}

	conf, err := strconv.ParseInt(strings.TrimSpace(parts[3]), 10, 64)
	if err != nil {
		return "", "", "", 0, false
	}

	return keyUnquoted, verdictNorm, reasonUnquoted, conf, true
}

// splitTopLevelCommas splits s on commas that are not inside single or
// double quotes and honors backslash escapes so a quoted reason may contain
// commas and escaped quotes.
func splitTopLevelCommas(s string) []string {
	var parts []string
	var cur strings.Builder
	var inSingle, inDouble, escaped bool
	for _, r := range s {
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped = false
		case r == '\\':
			cur.WriteRune(r)
			if inSingle || inDouble {
				escaped = true
			}
		case r == '"' && !inSingle:
			cur.WriteRune(r)
			inDouble = !inDouble
		case r == '\'' && !inDouble:
			cur.WriteRune(r)
			inSingle = !inSingle
		case r == ',' && !inSingle && !inDouble:
			parts = append(parts, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	parts = append(parts, cur.String())
	return parts
}

// checkpointVerdictOutcome is what the kernel derived from a reviewer's
// checkpoint_verdict rows (checkpoint_verdict_outcome).
type checkpointVerdictOutcome struct {
	outcome string // "/pass", "/fail" or "/inconclusive"; "" when none derived
	reason  string
}

func (v checkpointVerdictOutcome) passed() bool { return v.outcome == "/pass" }

// describe renders the outcome for the checkpoint record, with prefix naming
// the checkpoint ("Review", "Nemesis gauntlet").
func (v checkpointVerdictOutcome) describe(prefix string) string {
	switch v.outcome {
	case "/pass":
		return fmt.Sprintf("%s passed: %s", prefix, v.reason)
	case "/fail":
		return fmt.Sprintf("%s failed: %s", prefix, v.reason)
	default:
		return fmt.Sprintf("%s inconclusive: the reviewer passed it below campaign.checkpoint_min_confidence, which does not pass: %s", prefix, v.reason)
	}
}

// settleCheckpointVerdict reads the verdict the kernel derives for key. The
// live path's reviewer asserts its checkpoint_verdict/4 through its control
// packet (the session executor asserts mangle_updates); an executor that
// returns the envelope verbatim has its facts asserted here from raw. Either
// way the outcome is derived by the policy -- any /fail fails, a /pass passes
// only at the configured confidence -- and read here, never decided in Go.
// The key's rows are retracted after reading so a later checkpoint cannot
// inherit them. ok is false when no outcome derived: the caller fails closed.
func settleCheckpointVerdict(kernel core.Kernel, key, raw string) (checkpointVerdictOutcome, bool) {
	if kernel == nil || key == "" {
		return checkpointVerdictOutcome{}, false
	}
	rows := verdictRows(kernel, key)
	if len(rows) == 0 {
		for _, f := range parseCheckpointVerdictFacts(raw, key) {
			if err := kernel.Assert(f); err != nil {
				logging.CampaignWarn("checkpoint verdict for %s from the reviewer's envelope was not asserted: %v", key, err)
			}
		}
		rows = verdictRows(kernel, key)
	}
	defer func() {
		_ = kernel.RetractFact(core.Fact{Predicate: "checkpoint_verdict", Args: []any{key}})
	}()

	facts, err := kernel.Query("checkpoint_verdict_outcome")
	if err != nil {
		logging.CampaignWarn("checkpoint verdict for %s: query checkpoint_verdict_outcome: %v", key, err)
		return checkpointVerdictOutcome{}, false
	}
	var out checkpointVerdictOutcome
	for _, f := range facts {
		if len(f.Args) == 2 && types.ExtractString(f.Args[0]) == key {
			out.outcome = types.ExtractString(f.Args[1])
		}
	}
	if out.outcome == "" {
		return checkpointVerdictOutcome{}, false
	}
	// The reason quoted is one the outcome rests on: a failing row's for
	// /fail, a passing row's otherwise, with its confidence.
	want := "/pass"
	if out.outcome == "/fail" {
		want = "/fail"
	}
	for _, r := range rows {
		if r.verdict == want {
			out.reason = fmt.Sprintf("%s (confidence %d)", r.reason, r.confidence)
			break
		}
	}
	return out, true
}

type verdictRow struct {
	verdict    string
	reason     string
	confidence int64
}

// verdictRows lists the checkpoint_verdict rows the kernel holds for key.
func verdictRows(kernel core.Kernel, key string) []verdictRow {
	facts, err := kernel.Query("checkpoint_verdict")
	if err != nil {
		return nil
	}
	var rows []verdictRow
	for _, f := range facts {
		if f.Predicate != "checkpoint_verdict" || len(f.Args) != 4 || types.ExtractString(f.Args[0]) != key {
			continue
		}
		conf, _ := f.Args[3].(int64)
		rows = append(rows, verdictRow{
			verdict:    types.ExtractString(f.Args[1]),
			reason:     types.ExtractString(f.Args[2]),
			confidence: conf,
		})
	}
	return rows
}
