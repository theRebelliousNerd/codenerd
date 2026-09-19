package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	internalbuild "codenerd/internal/build"
	jitconfig "codenerd/internal/jit/config"
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// The two gates in this file turn evidence the harness already measured into
// evidence the verdict reads, and give the model the round to act on it first.
// Observed 2026-09-19 (ladder run R1-2): a nerd fix turn wrote 27 blocks of Go
// that no test executes and an unreachable statement go vet reports. The
// session log named the blocks, gopls named the statement to an advisory critic
// that timed out, and the turn was recorded /done. The agent is not asked for
// the tests that harden the change; the harness makes it write them.

// vetFinding matches one positioned go vet diagnostic: path:line:col: message.
var vetFinding = regexp.MustCompile(`^(.+\.go):\d+:\d+: `)

// vetFindingsInFiles keeps the diagnostics go vet printed for files the turn
// wrote. Vet runs package-wide and prints paths relative to the workspace,
// backslashed on Windows; a finding in a file the turn did not touch is not
// this turn's evidence.
func vetFindingsInFiles(output string, written []string) []string {
	var own []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		m := vetFinding.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		file := NormalizeCoverPath(m[1])
		for _, w := range written {
			if nw := NormalizeCoverPath(w); nw != "" && (file == nw || strings.HasSuffix(file, "/"+nw) || strings.HasSuffix(nw, "/"+file)) {
				own = append(own, line)
				break
			}
		}
	}
	return own
}

// verifyVet runs go vet over the untagged packages the turn wrote and judges
// the result on the turn's own files. Tag-gated packages are vetted by the
// test gate with their tags already (gateTests).
func verifyVet(ctx context.Context, workspace string, written []string) BuildVerification {
	start := time.Now()
	runnable, _ := splitTagGatedPackages(workspace, packagesForPaths(written))
	if len(runnable) == 0 {
		return BuildVerification{Outcome: VerifySkipped, Reason: "no untagged Go package written", Duration: time.Since(start)}
	}
	command := append([]string{"go", "vet"}, runnable...)
	out, outcome, reason := runVerificationCommand(ctx, workspace, internalbuild.GetBuildEnv(nil, workspace), buildVerifyTimeout, command[0], command[1:], verifyBuildRunner)
	elapsed := time.Since(start)
	switch outcome {
	case VerifyPassed:
		return BuildVerification{Ran: true, OK: true, Outcome: VerifyPassed, Command: command, Duration: elapsed}
	case VerifyFailed:
		own := vetFindingsInFiles(string(out), written)
		if len(own) == 0 {
			// Vet failed, but on nothing this turn wrote: the finding is the
			// workspace's, and it is not this turn's to answer for.
			logging.Get(logging.CategorySession).Warn(
				"go vet reported findings only in files this turn did not write:\n%s", strings.TrimSpace(string(out)))
			return BuildVerification{Ran: true, OK: true, Outcome: VerifyPassed, Command: command, Reason: "findings only in files the turn did not write", Duration: elapsed}
		}
		return BuildVerification{Ran: true, Output: strings.Join(own, "\n"), Outcome: VerifyFailed, Command: command, Reason: reason, Duration: elapsed}
	default:
		return BuildVerification{Ran: len(out) > 0, Output: strings.TrimSpace(string(out)), Outcome: outcome, Command: command, Reason: reason, Duration: elapsed}
	}
}

func vetRepairPrompt(findings string) string {
	return "go vet reports these problems in the files you changed:\n\n```\n" + findings + "\n```\n\n" +
		"Fix each one in the code, then stop; go vet will be run again. Do not silence a finding " +
		"with a directive or by moving the code out of the file: the finding describes a defect " +
		"(unreachable code, a wrong format verb, a copied lock), and the fix is to remove the defect."
}

// verifyAndRepairVet runs the vet gate after the tests pass and, when the
// turn's own files have findings, gives the model repair rounds with them.
// A round that does not converge leaves the finding to the verdict
// (turn_missing_evidence /vet_not_clean); it does not fail the turn.
func (e *Executor) verifyAndRepairVet(
	ctx context.Context,
	trp types.ToolResultsProvider,
	systemPrompt string,
	history []types.Message,
	toolDefs []types.ToolDefinition,
	cfg *jitconfig.EffectiveAgentRuntimeConfig,
	result *ExecutionResult,
) (*types.LLMToolResponse, []string, error) {
	if result == nil || result.SuccessfulWriteTools == 0 || !touchedGoFiles(result.WrittenPaths) {
		return nil, nil, nil
	}
	if !e.configSnapshot().VerifyBuildAfterEdits {
		return nil, nil, nil
	}
	workspace := e.workspaceForVerification()
	result.VetCheck = verifyVet(ctx, workspace, result.WrittenPaths)
	if result.VetCheck.Verdict() != VerifyFailed || trp == nil {
		return nil, nil, nil
	}
	logging.Get(logging.CategorySession).Warn("go vet rejects this turn's files; giving the model repair rounds:\n%s", result.VetCheck.Output)
	spec := repairSpec{
		kind:         "vet",
		brokenPhrase: "go vet reports problems in the files this turn changed",
		promptFor:    vetRepairPrompt,
		recheck: func(epCtx context.Context) (bool, string, VerifyOutcome) {
			v := verifyVet(epCtx, workspace, result.WrittenPaths)
			if v.Verdict() == VerifyPassed || v.Verdict() == VerifyFailed {
				result.VetCheck = v
			}
			return v.Verdict() == VerifyPassed, v.Output, v.Verdict()
		},
		followups: func() []string {
			return []string{"go vet " + strings.Join(packagesForPaths(result.WrittenPaths), " ")}
		},
	}
	repaired, repairErrs, rec, err := e.repairLoop(ctx, trp, systemPrompt, &history, toolDefs, cfg, result, result.VetCheck.Output, spec)
	result.VetCheck.Repair = rec
	return repaired, repairErrs, settleForcingRepair(err)
}

// narrowToChangedLines keeps the uncovered blocks that overlap lines the turn
// changed. The profile is file-level, so without this a one-line edit in a
// large file reports every uncovered block of the file as code the turn wrote
// (observed 2026-09-11: 59 blocks for one line); a file with no pre-write
// snapshot, which the turn created, keeps all its blocks.
func narrowToChangedLines(workspace string, result *ExecutionResult, uncovered []UncoveredBlock) []UncoveredBlock {
	if len(uncovered) == 0 || len(result.PreWriteContents) == 0 {
		return uncovered
	}
	changed := make(map[string][]LineRange, len(result.PreWriteContents))
	for path, before := range result.PreWriteContents {
		data, readErr := os.ReadFile(filepath.Join(workspace, filepath.FromSlash(path)))
		if readErr != nil {
			continue
		}
		changed[path] = changedLines(before, string(data))
	}
	return blocksInChangedLines(uncovered, changed)
}

// uncoveredList names every block, one per line, by the path the turn wrote.
// summarizeUncovered caps its list for the log; a repair built from a capped
// list repairs the blocks that happened to sort first.
func uncoveredList(blocks []UncoveredBlock, written []string) string {
	var b strings.Builder
	for _, block := range blocks {
		path := NormalizeCoverPath(block.File)
		for _, w := range written {
			if nw := NormalizeCoverPath(w); nw != "" && strings.HasSuffix(path, nw) {
				path = nw
				break
			}
		}
		fmt.Fprintf(&b, "%s:%d-%d (%d statement(s))\n", path, block.StartLine, block.EndLine, block.NumStmts)
	}
	return strings.TrimRight(b.String(), "\n")
}

func coverageRepairPrompt(blocks string) string {
	return "The tests pass, but no test executes these lines of code you changed:\n\n```\n" + blocks + "\n```\n\n" +
		"Write or extend tests that run each of them and assert what it does -- the behaviour the " +
		"change was made for, and the error and edge branches it added. Put them in the package's " +
		"_test.go files. Do not change the production code to make these lines unreachable or to " +
		"remove them; they are the change. The tests will be run again with coverage."
}

// verifyAndRepairCoverage runs after the tests pass. When code the turn
// changed is executed by no test, the model gets repair rounds to write the
// tests that run it; what is still unexecuted afterwards is left to the
// verdict (turn_missing_evidence /changed_code_unexecuted), not failed.
func (e *Executor) verifyAndRepairCoverage(
	ctx context.Context,
	trp types.ToolResultsProvider,
	systemPrompt string,
	history []types.Message,
	toolDefs []types.ToolDefinition,
	cfg *jitconfig.EffectiveAgentRuntimeConfig,
	result *ExecutionResult,
) (*types.LLMToolResponse, []string, error) {
	if result == nil || len(result.UncoveredBlocks) == 0 || result.TestCheck.Verdict() != VerifyPassed {
		return nil, nil, nil
	}
	workspace := e.workspaceForVerification()
	if result.TestCheck.Repair != nil {
		// A test repair ran after the blocks were measured and may have
		// changed them: measure again before asking for anything.
		v, uncovered := gateTests(ctx, workspace, result, true)
		if v.Verdict() != VerifyPassed {
			return nil, nil, nil
		}
		result.UncoveredBlocks = narrowToChangedLines(workspace, result, uncovered)
		if len(result.UncoveredBlocks) == 0 {
			return nil, nil, nil
		}
	}
	if trp == nil {
		return nil, nil, nil
	}
	logging.Get(logging.CategorySession).Warn(
		"Code this turn changed is executed by no test (%d block(s)); giving the model repair rounds to write the tests", len(result.UncoveredBlocks))
	spec := repairSpec{
		kind:         "coverage",
		brokenPhrase: "code this turn changed is executed by no test",
		promptFor:    coverageRepairPrompt,
		recheck: func(epCtx context.Context) (bool, string, VerifyOutcome) {
			v, uncovered := gateTests(epCtx, workspace, result, true)
			if v.Verdict() == VerifyPassed || v.Verdict() == VerifyFailed {
				// The test gate's own repair record stays with it; this
				// round's record is its own.
				v.Repair = result.TestCheck.Repair
				result.TestCheck = v
			}
			if v.Verdict() != VerifyPassed {
				// The new tests broke something: the failure is what the next
				// round works from, and the close re-runs the gates regardless.
				return false, v.Output, v.Verdict()
			}
			result.UncoveredBlocks = narrowToChangedLines(workspace, result, uncovered)
			if len(result.UncoveredBlocks) == 0 {
				return true, "", VerifyPassed
			}
			return false, uncoveredList(result.UncoveredBlocks, result.WrittenPaths), VerifyFailed
		},
		followups: func() []string {
			runnable, _ := splitTagGatedPackages(workspace, packagesForPaths(result.WrittenPaths))
			return repairFollowups(workspace, runnable, result, "coverage")
		},
	}
	repaired, repairErrs, rec, err := e.repairLoop(ctx, trp, systemPrompt, &history, toolDefs, cfg, result, uncoveredList(result.UncoveredBlocks, result.WrittenPaths), spec)
	if rec != nil && !rec.Passed {
		logging.Get(logging.CategorySession).Warn(
			"Coverage repair left %d block(s) executed by no test; the verdict names them", len(result.UncoveredBlocks))
	}
	return repaired, repairErrs, settleForcingRepair(err)
}

// removedTestsRepairPrompt asks for the tests the turn deleted back. The seed
// is either the tests still missing, with their source, or the failing run of
// the restored tests.
func removedTestsRepairPrompt(seed string) string {
	return "This turn deleted tests that existed before it, and no test of the same name exists anywhere " +
		"in the workspace now. A test is a contract the system already had: put each one back in the " +
		"file it came from. If your change altered the behaviour a test pins on purpose, keep the test " +
		"and change its assertions to the new behaviour -- never delete it. The tests are run again " +
		"afterwards.\n\n" + seed
}

// verifyAndRepairRemovedTests runs after the other gates, so a suite made
// green by losing a contract is still caught. A turn that deleted tests which
// existed before it -- and exist nowhere in the workspace now -- gets repair
// rounds in which it is handed each one's source as the turn found it, and
// the restored tests are run again. A turn that still deletes them fails: a
// test is fixed by fixing the code, not by deleting it. Until 2026-09-19 the
// first deletion failed the turn with no round (ladder run R1-4: a whole-file
// rewrite dropped three tests that still passed against the new code, and
// fourteen minutes of otherwise green work were refused).
func (e *Executor) verifyAndRepairRemovedTests(
	ctx context.Context,
	trp types.ToolResultsProvider,
	systemPrompt string,
	history []types.Message,
	toolDefs []types.ToolDefinition,
	cfg *jitconfig.EffectiveAgentRuntimeConfig,
	result *ExecutionResult,
) (*types.LLMToolResponse, []string, error) {
	if result == nil || result.SuccessfulWriteTools == 0 || len(result.PreWriteContents) == 0 {
		return nil, nil, nil
	}
	workspace := e.workspaceForVerification()
	removed := removedTestFunctions(workspace, result.WrittenPaths, result.PreWriteContents)
	if len(removed) == 0 {
		return nil, nil, nil
	}
	stillRemoved := func() error {
		return fmt.Errorf("%w: turn removed test(s) without replacing them: %s; a failing test is fixed by fixing the code, not by deleting the test",
			ErrVerificationFailed, strings.Join(removed, ", "))
	}
	if trp == nil {
		return nil, nil, stillRemoved()
	}
	missing := func() string {
		return "Deleted by this turn, as they were before it:\n\n" + removedTestListing(removed, result.PreWriteContents)
	}
	logging.Get(logging.CategorySession).Warn(
		"This turn deleted %d test(s) that existed before it; giving the model repair rounds to restore them: %s",
		len(removed), strings.Join(removed, ", "))
	spec := repairSpec{
		kind:         "removed_tests",
		brokenPhrase: "the turn deleted tests that existed before it",
		promptFor:    removedTestsRepairPrompt,
		recheck: func(epCtx context.Context) (bool, string, VerifyOutcome) {
			removed = removedTestFunctions(workspace, result.WrittenPaths, result.PreWriteContents)
			if len(removed) > 0 {
				return false, missing(), VerifyFailed
			}
			v, _ := gateTests(epCtx, workspace, result, false)
			if v.Verdict() == VerifyPassed || v.Verdict() == VerifyFailed {
				// The test gate's own repair record stays with it.
				v.Repair = result.TestCheck.Repair
				result.TestCheck = v
			}
			if v.Verdict() != VerifyPassed {
				return false, "The deleted tests are back, and the tests fail:\n\n```\n" + v.Output + "\n```", v.Verdict()
			}
			return true, "", VerifyPassed
		},
		followups: func() []string {
			if len(removed) > 0 {
				return []string{"restore " + strings.Join(removed, ", ")}
			}
			runnable, _ := splitTagGatedPackages(workspace, packagesForPaths(result.WrittenPaths))
			return repairFollowups(workspace, runnable, result, "tests")
		},
	}
	repaired, repairErrs, _, err := e.repairLoop(ctx, trp, systemPrompt, &history, toolDefs, cfg, result, missing(), spec)
	if err != nil && errors.Is(err, ErrVerificationFailed) && len(removed) > 0 {
		return nil, repairErrs, stillRemoved()
	}
	return repaired, repairErrs, err
}

// settleForcingRepair decides what a forcing round's error means for the
// turn. Running out of attempts leaves the debt on the result, where the
// kernel's verdict names it; a cancel is a cancel.
func settleForcingRepair(err error) error {
	if err == nil || errors.Is(err, context.Canceled) {
		return err
	}
	if errors.Is(err, ErrVerificationFailed) {
		logging.Get(logging.CategorySession).Warn("forcing round did not converge; the verdict carries what is missing: %v", err)
		return nil
	}
	return err
}
