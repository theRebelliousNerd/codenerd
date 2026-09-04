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

	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/session"
	"codenerd/internal/tactile"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)


// CheckpointRunner runs verification checkpoints for phases.
type CheckpointRunner struct {
	executor     tactile.Executor
	taskExecutor session.TaskExecutor
	workspace    string
	kernel       core.Kernel
}

// NewCheckpointRunner creates a new checkpoint runner.
// The kernel is optional for backward compatibility: production callers pass
// o.kernel so structured checkpoint_verdict/4 facts asserted by the session
// executor (control_packet.mangle_updates) can be read on the live path,
// where TaskExecutor.Execute returns only the surface_response. Callers that
// omit it get a runner that falls back to parsing a raw envelope string.
func NewCheckpointRunner(executor tactile.Executor, taskExecutor session.TaskExecutor, workspace string, kernels ...core.Kernel) *CheckpointRunner {
	var k core.Kernel
	if len(kernels) > 0 {
		k = kernels[0]
	}
	return &CheckpointRunner{
		executor:     executor,
		taskExecutor: taskExecutor,
		workspace:    workspace,
		kernel:       k,
	}
}

// SetKernel wires the kernel used for structured verdict lookup after
// construction. Prefer passing the kernel to NewCheckpointRunner.
func (cr *CheckpointRunner) SetKernel(k core.Kernel) {
	if cr == nil {
		return
	}
	cr.kernel = k
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
		logging.CampaignWarn("CheckpointRunner.Run: unknown verification method=%s, skipping", method)
		return true, "Unknown verification method, skipping", nil
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
	parts := strings.Fields(testCmdStr)

	cmd := tactile.Command{
		Binary:           parts[0],
		Arguments:        parts[1:],
		WorkingDirectory: cr.workspace,
		Limits: &tactile.ResourceLimits{
			TimeoutMs: 600 * 1000, // 10 minutes
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
			TimeoutMs: 600 * 1000, // 10 minutes
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

	reviewPrompt.WriteString("\nYour response MUST be a JSON control-packet carrying exactly one checkpoint_verdict/4 fact in control_packet.mangle_updates:\n")
	reviewPrompt.WriteString("checkpoint_verdict(\"PhaseName\", Verdict, \"reason\", Confidence).\n")
	reviewPrompt.WriteString(fmt.Sprintf("PhaseName must be exactly %q. ", phase.Name))
	reviewPrompt.WriteString("Verdict must be /pass (objectives met) or /fail (objectives not met). Reason is a short human-readable justification. Confidence is an integer percent 0-100.\n")
	reviewPrompt.WriteString("Example: {\"control_packet\": {\"mangle_updates\": [\"checkpoint_verdict(\\\"my-phase\\\", /pass, \\\"all objectives met\\\", 95)\"]}, \"surface_response\": \"...\"}.\n")
	reviewPrompt.WriteString("Free-text PASS/FAIL is not accepted; only checkpoint_verdict/4 decides.")

	// Spawn reviewer intent
	result, err := cr.spawnTask(ctx, "/review", reviewPrompt.String())
	if err != nil {
		logging.CampaignError("runShardValidationCheckpoint: reviewer shard failed for phase=%s: %v", phase.Name, err)
		return false, fmt.Sprintf("Reviewer shard failed: %v", err), err
	}

	// Structured verdict: the reviewer's control packet reaches the KERNEL
	// (mangle_updates are asserted by the session executor), not the returned
	// string. Query the kernel first; fall back to parsing the returned
	// string as a raw envelope for executors that return it verbatim.
	// Anything without a well-formed checkpoint_verdict/4 for this phase
	// fails closed.
	if passed, reason, ok := cr.lookupKernelVerdict(phase.Name); ok {
		if passed {
			logging.Campaign("runShardValidationCheckpoint: reviewer approved phase=%s", phase.Name)
			return true, fmt.Sprintf("Review passed: %s", reason), nil
		}
		logging.CampaignWarn("runShardValidationCheckpoint: reviewer found issues in phase=%s", phase.Name)
		return false, fmt.Sprintf("Review failed: %s", reason), nil
	}
	resultStr := fmt.Sprintf("%v", result)

	passed, reason, ok := parseCheckpointVerdict(resultStr, phase.Name)
	if !ok {
		logging.CampaignWarn("runShardValidationCheckpoint: reviewer verdict could not be determined for phase=%s; failing closed", phase.Name)
		return false, fmt.Sprintf("Review verdict could not be determined (missing or malformed checkpoint_verdict/4 for phase %q): %s", phase.Name, resultStr), nil
	}
	if passed {
		logging.Campaign("runShardValidationCheckpoint: reviewer approved phase=%s", phase.Name)
		return true, fmt.Sprintf("Review passed: %s", reason), nil
	}
	logging.CampaignWarn("runShardValidationCheckpoint: reviewer found issues in phase=%s", phase.Name)
	return false, fmt.Sprintf("Review failed: %s", reason), nil
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
	nemesisPrompt.WriteString("checkpoint_verdict(\"PhaseName\", Verdict, \"reason\", Confidence).\n")
	nemesisPrompt.WriteString(fmt.Sprintf("PhaseName must be exactly %q. ", phaseName))
	nemesisPrompt.WriteString("Verdict must be /pass (survived the gauntlet, no exploitable weaknesses found) or /fail (gauntlet broke the implementation). Reason is a short human-readable justification. Confidence is an integer percent 0-100.\n")
	nemesisPrompt.WriteString("Example: {\"control_packet\": {\"mangle_updates\": [\"checkpoint_verdict(\\\"my-phase\\\", /pass, \\\"no weaknesses found\\\", 95)\"]}, \"surface_response\": \"...\"}.\n")
	nemesisPrompt.WriteString("Free-text PASS/FAIL is not accepted; only checkpoint_verdict/4 decides.")

	logging.CampaignDebug("runNemesisGauntletCheckpoint: target=%s", target)
	result, err := cr.spawnTask(ctx, "/nemesis", nemesisPrompt.String())
	if err != nil {
		logging.CampaignError("runNemesisGauntletCheckpoint: nemesis shard failed for phase=%s: %v", phaseName, err)
		return false, fmt.Sprintf("Nemesis shard failed: %v", err), err
	}

	// Structured verdict: the reviewer's control packet reaches the KERNEL
	// (mangle_updates are asserted by the session executor), not the returned
	// string. Query the kernel first; fall back to parsing the returned
	// string as a raw envelope for executors that return it verbatim.
	// Anything without a well-formed checkpoint_verdict/4 for this phase
	// fails closed.
	if passed, reason, ok := cr.lookupKernelVerdict(phaseName); ok {
		if passed {
			logging.Campaign("runNemesisGauntletCheckpoint: phase=%s survived nemesis gauntlet", phaseName)
			return true, fmt.Sprintf("Nemesis gauntlet passed: %s", reason), nil
		}
		logging.CampaignWarn("runNemesisGauntletCheckpoint: nemesis found vulnerabilities in phase=%s", phaseName)
		return false, fmt.Sprintf("Nemesis gauntlet failed: %s", reason), nil
	}
	resultStr := fmt.Sprintf("%v", result)

	passed, reason, ok := parseCheckpointVerdict(resultStr, phaseName)
	if !ok {
		logging.CampaignWarn("runNemesisGauntletCheckpoint: nemesis verdict could not be determined for phase=%s; failing closed", phaseName)
		return false, fmt.Sprintf("Nemesis verdict could not be determined (missing or malformed checkpoint_verdict/4 for phase %q): %s", phaseName, resultStr), nil
	}
	if passed {
		logging.Campaign("runNemesisGauntletCheckpoint: phase=%s survived nemesis gauntlet", phaseName)
		return true, fmt.Sprintf("Nemesis gauntlet passed: %s", reason), nil
	}
	logging.CampaignWarn("runNemesisGauntletCheckpoint: nemesis found vulnerabilities in phase=%s", phaseName)
	return false, fmt.Sprintf("Nemesis gauntlet failed: %s", reason), nil
}

// detectTestCommand delegates to the canonical tools.TestCommandForDir
// projection (internal/tools/framework.go, mirroring the policy
// test_framework/1 + test_command/1 facts) and falls back to the Go default
// on unknown workspaces.
func (r *CheckpointRunner) detectTestCommand() string {
	if cmd, ok := tools.TestCommandForDir(r.workspace); ok {
		return cmd
	}
	return tools.DefaultTestCommand
}

// detectBuildCommand delegates to the canonical tools.BuildCommandForDir
// projection (mirroring the policy build_command/1 facts) and falls back to
// the Go default on unknown workspaces.
func (r *CheckpointRunner) detectBuildCommand() string {
	if cmd, ok := tools.BuildCommandForDir(r.workspace); ok {
		return cmd
	}
	return tools.DefaultBuildCommand
}

// parseTestOutput parses test output to count passed/failed tests.
func (cr *CheckpointRunner) parseTestOutput(output string) (passed, failed int) {
	lines := strings.SplitSeq(output, "\n")

	for line := range lines {
		lower := strings.ToLower(line)

		// Go test output
		if strings.Contains(lower, "--- pass") {
			passed++
		} else if strings.Contains(lower, "--- fail") {
			failed++
		}

		// Generic patterns
		if strings.Contains(lower, "passed") || strings.Contains(lower, "ok") {
			// Try to extract number
			// This is a rough heuristic
		}
		if strings.Contains(lower, "failed") || strings.Contains(lower, "error") {
			failed++
		}
	}

	// If we couldn't parse, assume 1 passed if no failures
	if passed == 0 && failed == 0 {
		passed = 1
	}

	return passed, failed
}

// parseGoTestJSON parses go test -json output for pass/fail counts.
func (cr *CheckpointRunner) parseGoTestJSON(output string) (passed, failed int, duration time.Duration) {
	type goTestEvent struct {
		Action  string  `json:"Action"`
		Test    string  `json:"Test"`
		Elapsed float64 `json:"Elapsed"`
	}

	dec := json.NewDecoder(strings.NewReader(output))
	for dec.More() {
		var evt goTestEvent
		if err := dec.Decode(&evt); err != nil {
			// Fall back to heuristic if JSON framing breaks
			p, f := cr.parseTestOutput(output)
			return p, f, 0
		}
		switch evt.Action {
		case "pass":
			if evt.Test != "" {
				passed++
				duration += time.Duration(evt.Elapsed * float64(time.Second))
			}
		case "fail":
			if evt.Test != "" {
				failed++
				duration += time.Duration(evt.Elapsed * float64(time.Second))
			} else {
				// package-level failure
				failed++
			}
		}
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

// fileExists checks if a file exists in the workspace.
func fileExists(workspace, file string) bool {
	path := filepath.Join(workspace, file)
	if _, err := os.Stat(path); err == nil {
		return true
	}
	return false
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

// parseCheckpointVerdict extracts the structured reviewer verdict for the
// given phase from a JSON control-packet envelope. Only
// control_packet.mangle_updates entries that are well-formed
// checkpoint_verdict/4 facts decide; free text, bare atoms outside the
// envelope, and prose PASS/FAIL are inert.
//
// Returns (passed, reason, ok): ok is false when no well-formed verdict for
// this phase is present, in which case the caller must fail closed.
func parseCheckpointVerdict(resultStr, phaseName string) (bool, string, bool) {
	var env checkpointEnvelope
	if err := json.Unmarshal([]byte(resultStr), &env); err != nil {
		return false, "", false
	}

	for _, update := range env.Control.MangleUpdates {
		gotPhase, verdict, reason, ok := parseCheckpointVerdictAtom(update)
		if !ok {
			continue
		}
		if gotPhase != phaseName {
			continue
		}
		switch verdict {
		case "pass":
			return true, reason, true
		case "fail":
			return false, reason, true
		default:
			continue
		}
	}
	return false, "", false
}

// parseCheckpointVerdictAtom parses a single mangle_updates entry as a
// structured checkpoint_verdict/4 fact:
//
//	checkpoint_verdict("Phase", /pass|/fail, "details", confidence)
//
// The entry must be exactly the atom (modulo surrounding whitespace); the
// atom is never searched for inside a larger string. Verdict accepts /pass,
// "pass" or 'pass' spellings so JSON-quoted atoms still parse. Confidence
// must be numeric but is otherwise ignored; the verdict atom alone decides.
// Phase is returned verbatim so the caller can require it to match the
// checkpoint's phase name.
func parseCheckpointVerdictAtom(atom string) (phase, verdict, reason string, ok bool) {
	trimmed := strings.TrimSpace(atom)
	const prefix = "checkpoint_verdict("
	if !strings.HasPrefix(trimmed, prefix) || !strings.HasSuffix(trimmed, ")") {
		return "", "", "", false
	}
	inner := trimmed[len(prefix) : len(trimmed)-1]
	parts := splitTopLevelCommas(inner)
	if len(parts) != 4 {
		return "", "", "", false
	}

	phasePart := strings.TrimSpace(parts[0])
	phaseUnquoted, err := strconv.Unquote(phasePart)
	if err != nil {
		return "", "", "", false
	}

	verdictPart := strings.TrimSpace(parts[1])
	verdictPart = strings.Trim(verdictPart, `"'`)
	verdictPart = strings.TrimSpace(verdictPart)
	verdictPart = strings.TrimPrefix(verdictPart, "/")
	verdictNorm := strings.ToLower(strings.TrimSpace(verdictPart))
	if verdictNorm != "pass" && verdictNorm != "fail" {
		return "", "", "", false
	}

	reasonPart := strings.TrimSpace(parts[2])
	reasonUnquoted, err := strconv.Unquote(reasonPart)
	if err != nil {
		return "", "", "", false
	}

	confidencePart := strings.TrimSpace(parts[3])
	if _, err := strconv.ParseFloat(confidencePart, 64); err != nil {
		return "", "", "", false
	}

	return phaseUnquoted, verdictNorm, reasonUnquoted, true
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


// lookupKernelVerdict returns the structured reviewer verdict for phaseName
// from the kernel, where the live session executor asserts control-packet
// mangle_updates (see internal/session/executor.go processMangleUpdatesFromEnvelope).
// The TaskExecutor.Execute string is only the surface_response, so parsing it
// can never see the verdict on the live path.
//
// Returns (passed, reason, ok): ok is false when no well-formed
// checkpoint_verdict/4 for this phase exists in the kernel, in which case the
// caller falls back to parseCheckpointVerdict for raw-envelope executors and
// otherwise fails closed. A matching fact is retracted after reading so a
// later phase cannot inherit a stale verdict.
func (cr *CheckpointRunner) lookupKernelVerdict(phaseName string) (bool, string, bool) {
	if cr == nil || cr.kernel == nil {
		return false, "", false
	}
	return lookupCheckpointVerdictInKernel(cr.kernel, phaseName)
}

// lookupCheckpointVerdictInKernel is the shared kernel lookup used by the
// CheckpointRunner checkpoints and the assault nemesis stage (which reaches
// the kernel via o.kernel rather than a runner).
func lookupCheckpointVerdictInKernel(kernel core.Kernel, phaseName string) (bool, string, bool) {
	if kernel == nil {
		return false, "", false
	}
	facts, err := kernel.Query("checkpoint_verdict")
	if err != nil {
		return false, "", false
	}
	matched := false
	for _, f := range facts {
		if f.Predicate != "checkpoint_verdict" || len(f.Args) != 4 {
			continue
		}
		if types.ExtractString(f.Args[0]) != phaseName {
			continue
		}
		matched = true
	}
	if !matched {
		return false, "", false
	}
	// Retract all facts for this phase so a later phase cannot inherit a
	// stale verdict. Best effort: a retraction failure must not fail the
	// checkpoint itself. RetractFact matches predicate + first arg, which is
	// exactly the per-phase scope needed here.
	_ = kernel.RetractFact(core.Fact{Predicate: "checkpoint_verdict", Args: []any{phaseName}})
	for _, f := range facts {
		if f.Predicate != "checkpoint_verdict" || len(f.Args) != 4 {
			continue
		}
		if types.ExtractString(f.Args[0]) != phaseName {
			continue
		}
		verdictRaw := types.ExtractString(f.Args[1])
		verdict := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(verdictRaw), "/")))
		verdict = strings.Trim(verdict, `"'`)
		verdict = strings.TrimSpace(verdict)
		if verdict != "pass" && verdict != "fail" {
			continue
		}
		reason := types.ExtractString(f.Args[2])
		// Confidence (Args[3]) is an integer percent 0-100 per the Decl.
		// The Decl already enforces /number; the verdict alone decides here.
		if verdict == "pass" {
			return true, reason, true
		}
		return false, reason, true
	}
	return false, "", false
}