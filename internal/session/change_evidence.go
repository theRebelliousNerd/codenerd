package session

import (
	"context"
	"fmt"
	"strings"

	"codenerd/internal/evidence"
)

// failedChecksSummary names what the final re-verification found failing: the
// build's error lines and the tests that fail. The turn's error is what the
// user and the verdict read; "failed mechanical checks" alone (ladder run
// R1-4b) sent them to the session log to find out which.
func failedChecksSummary(result *ExecutionResult) string {
	var parts []string
	if result.BuildCheck.Verdict() == VerifyFailed {
		part := "build fails"
		var errs []string
		for _, line := range strings.Split(result.BuildCheck.Output, "\n") {
			if line = strings.TrimSpace(line); vetFinding.MatchString(line) {
				errs = append(errs, line)
			}
		}
		if len(errs) > 0 {
			part += ": " + strings.Join(errs, "; ")
		} else if reason := strings.TrimSpace(result.BuildCheck.Reason); reason != "" {
			part += ": " + reason
		}
		parts = append(parts, part)
	}
	if result.TestCheck.Verdict() == VerifyFailed {
		part := "tests fail"
		if names := topLevelFailedTests(result.TestCheck.Output); len(names) > 0 {
			part += ": " + strings.Join(names, ", ")
		} else if reason := strings.TrimSpace(result.TestCheck.Reason); reason != "" {
			part += ": " + reason
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, "; ")
}

// closeChangeEvidence runs after the turn's last writer -- every repair round
// and the critic's uplift -- and binds the evidence the verdict reads to the
// workspace as it is now. A gate's verdict describes the revision it measured;
// an edit after it turns that verdict into a statement about a workspace that
// no longer exists. When anything changed after the gates began, or a change
// cannot be ruled out, every gate the verdict reads is measured again here
// (remeasureGates), with no model in the loop: build, tests, coverage, vet and
// the test inventory then all describe one revision.
//
// External audit F3 (2026-09-19): this refreshed the build and the tests only,
// and dropped the tests' coverage, so a later round could add a branch no test
// runs, a vet finding, or delete a passing test, and the turn kept the earlier
// clean coverage, vet and inventory verdicts -- or kept an earlier debt a
// later round had paid.
func (e *Executor) closeChangeEvidence(ctx context.Context, result *ExecutionResult, before string) error {
	if result == nil {
		return nil
	}
	workspace := e.workspaceForVerification()
	if result.SuccessfulWriteTools > 0 {
		result.ChangeStage = "artifact_changed"
		if touchedGoFiles(result.WrittenPaths) {
			after, err := evidence.Snapshot(ctx, workspace)
			if err != nil || before == "" || after != before {
				if err := e.remeasureGates(ctx, workspace, result); err != nil {
					return err
				}
			}
			current, currentErr := evidence.Snapshot(ctx, workspace)
			if err == nil && currentErr == nil && current == after &&
				result.BuildCheck.Verdict() == VerifyPassed && result.TestCheck.Verdict() == VerifyPassed {
				result.ChecksSnapshot = current
				result.ChangeStage = "checks_passed"
			}
			// Only an affirmative failure fails the turn. A timeout or cancel is
			// not proof of broken code: the stage stays artifact_changed and the
			// turn is labeled unverified rather than failed.
			if result.BuildCheck.Verdict() == VerifyFailed || result.TestCheck.Verdict() == VerifyFailed {
				return fmt.Errorf("%w: final workspace failed mechanical checks: %s", ErrVerificationFailed, failedChecksSummary(result))
			}
		}
	}
	if result.acceptanceTransaction != nil {
		report := result.acceptanceTransaction.Verify(ctx)
		result.Acceptance = &report
		if report.Status == "verified" {
			result.ChangeStage = "behavior_verified"
		} else {
			return fmt.Errorf("%w: %s", ErrVerificationFailed, report.Summary())
		}
	}
	return nil
}

// remeasureGates measures every gate the verdict reads on the workspace as it
// is now, in the order the post-edit gates run. Nothing here asks the model
// anything or edits a file: the rounds are over, and this is what they left.
// A deleted test fails the turn, as it does in its own round; the other gates
// leave their findings on the result for the verdict to read.
func (e *Executor) remeasureGates(ctx context.Context, workspace string, result *ExecutionResult) error {
	cfg := e.configSnapshot()
	if cfg.VerifyBuildAfterEdits {
		fresh := verifyBuild(ctx, workspace, nil)
		fresh.Repair = inheritRepair(fresh.Verdict(), result.BuildCheck.Repair)
		result.BuildCheck = fresh
	}
	if cfg.VerifyTestsAfterEdits {
		result.UntestedPaths = untestedWithoutCoverageOnDisk(workspace, result.WrittenPaths)
		// Must stay the same helper the post-edit gate uses (gateTests), or a
		// tag-gated package fails the turn twice over. With coverage: the
		// blocks no test runs are this revision's too, whichever way they
		// moved -- a later round can add one or pay one off.
		fresh, uncovered := gateTests(ctx, workspace, result, true)
		fresh.Repair = inheritRepair(fresh.Verdict(), result.TestCheck.Repair)
		result.TestCheck = fresh
		result.UncoveredBlocks = narrowToChangedLines(workspace, result, uncovered)
	}
	if cfg.VerifyBuildAfterEdits {
		fresh := verifyVet(ctx, workspace, result.WrittenPaths, result.PreWriteContents)
		fresh.Repair = inheritRepair(fresh.Verdict(), result.VetCheck.Repair)
		result.VetCheck = fresh
	}
	if removed := removedTestFunctions(workspace, result.WrittenPaths, result.PreWriteContents); len(removed) > 0 {
		return removedTestsError(removed)
	}
	return nil
}

// surfaceToolErrors records unrecovered tool failures as the turn's error.
//
// A tool error is an event inside the turn, not a verdict on it: the model may
// run a build, see it fail, fix the code, and run it green. What decides
// whether the failure survived is the CLOSING evidence, so that is what this
// asks.
//
// It used to ask whether the final response was empty. Live (2026-09-18 00:05,
// pid 054828, `nerd fix`): run_build failed mid-turn, the repair loop
// converged over two attempts, and the post-edit build and tests both passed —
// and the command still exited non-zero, because the response WAS empty. The
// working policy had closed the turn before the model wrote its last sentence,
// and appendEvidenceReport then glued the runtime's own "Evidence:
// checks_passed" onto that empty string, so the user was shown a green
// evidence line beside a failed exit. turn_cost recorded outcome=/failed with
// every gate green.
func surfaceToolErrors(result *ExecutionResult, toolErrs []string) {
	if result == nil || len(toolErrs) == 0 {
		return
	}
	if turnRecoveredFromToolErrors(result) {
		return
	}
	result.Error = fmt.Errorf("tool execution failed: %s", strings.Join(toolErrs, "; "))
}

// turnRecoveredFromToolErrors reports whether the closing evidence answers the
// tool errors this turn hit on the way there.
//
// The order is the point. An affirmative gate failure is the tool error
// confirmed; a gate that ran and passed is direct evidence about the final
// workspace and outranks anything that failed earlier in the turn; and only
// when no gate ran at all does the model's own closing answer decide, because
// then it is the only signal there is.
func turnRecoveredFromToolErrors(result *ExecutionResult) bool {
	if result == nil {
		return false
	}
	if result.BuildCheck.Verdict() == VerifyFailed || result.TestCheck.Verdict() == VerifyFailed {
		return false
	}
	if result.BuildCheck.Verdict() == VerifyPassed || result.TestCheck.Verdict() == VerifyPassed {
		return true
	}
	return strings.TrimSpace(result.Response) != ""
}

// closeAcceptanceEvidence verifies, persists and arbitrates the acceptance
// report, and re-arbitrates the checks snapshot for a write turn with no
// contract. It runs BEFORE the hollow gate because it can set result.Error,
// and an error already on the result outranks the hollow reason when the turn
// closes (checkHollowSuccess runs on every path; its verdict is /failed for an
// errored turn).
//
// It used to also write the closing sentence, and that is why the sentence was
// a constant: at this point in the turn the kernel has not been asked anything
// yet — assertTurnEvidence and the verdict read both happen inside
// checkHollowSuccess, afterwards — so the only thing this function could
// truthfully say was "unverified", and it said it unconditionally. The sentence
// now lives in appendEvidenceSummary, which runs after the verdict.
func (e *Executor) closeAcceptanceEvidence(ctx context.Context, result *ExecutionResult) {
	if result == nil {
		return
	}
	if result.acceptanceTransaction != nil && result.Acceptance == nil {
		r := result.acceptanceTransaction.Unverified("turn did not reach acceptance verification")
		result.Acceptance = &r
	}
	if result.Acceptance != nil {
		current, err := evidence.Snapshot(ctx, e.workspaceForVerification())
		if err != nil || current != result.Acceptance.After {
			result.Acceptance.Status = "unverified"
			result.Acceptance.Unknown = append(result.Acceptance.Unknown, "workspace changed after acceptance verification")
			result.TurnOutcome = "/unverified"
		}
		if _, err := evidence.Persist(e.workspaceForVerification(), *result.Acceptance); err != nil {
			result.Acceptance.Status = "unverified"
			result.Acceptance.Unknown = append(result.Acceptance.Unknown, "could not persist acceptance report: "+err.Error())
		}
		if result.Acceptance.Status != "verified" && result.Error == nil {
			result.Error = fmt.Errorf("%w: acceptance remains unverified", ErrVerificationFailed)
		}
		return
	}
	if result.SuccessfulWriteTools > 0 && result.ChangeStage == "checks_passed" {
		current, err := evidence.Snapshot(ctx, e.workspaceForVerification())
		if err != nil || current != result.ChecksSnapshot {
			result.ChangeStage = "artifact_changed"
			result.ChecksSnapshot = ""
		}
	}
}

// appendEvidenceSummary writes the turn's closing evidence sentence. It runs
// after checkHollowSuccess, so result.TurnOutcome and result.MissingEvidence
// carry the kernel's verdict and the sentence is a function of it.
//
// Before this seam the sentence was the constant "Evidence: <stage>. Requested
// behavior remains unverified (no acceptance contract)." on every write turn.
// It was accidentally true — turn_done needed a contract, so nothing without
// one ever WAS verified — and it would have become a lie the moment the corpus
// could verify a turn from its gates. There are three states now, and the
// sentence names which one the turn reached and, when it did not reach done,
// which evidence the kernel found missing.
func (e *Executor) appendEvidenceSummary(result *ExecutionResult) {
	if result == nil {
		return
	}
	// Verified by contract: the report states its own status, its snapshot and
	// every obligation it ran, which is strictly more than a sentence could.
	if result.Acceptance != nil {
		result.Response += "\n\n" + result.Acceptance.Summary()
		return
	}
	if result.SuccessfulWriteTools == 0 {
		return
	}
	stage := result.ChangeStage
	if stage == "" {
		stage = "artifact_changed"
	}
	written := "none"
	if len(result.WrittenPaths) > 0 {
		if len(result.WrittenPaths) > 10 {
			written = strings.Join(result.WrittenPaths[:10], ", ") + fmt.Sprintf(" and %d more", len(result.WrittenPaths)-10)
		} else {
			written = strings.Join(result.WrittenPaths, ", ")
		}
	}
	result.Response += fmt.Sprintf("\n\nWrote %d file(s): %s\nEvidence: %s. %s",
		len(result.WrittenPaths), written, stage, verdictSentence(result))
}

// verdictSentence states the turn's verification in one sentence, derived from
// the outcome the kernel produced. No branch here decides anything: each one
// spells out an atom captureTurnOutcome already recorded.
//
// There is no acceptance branch. A turn with a report never reaches this —
// appendEvidenceSummary prints Report.Summary() and returns — and a branch that
// cannot be reached is a second answer to a question already answered, which is
// how two spellings of "verified by contract" would drift apart.
func verdictSentence(result *ExecutionResult) string {
	if result == nil {
		return "Unverified: the turn produced no verdict."
	}
	switch result.TurnOutcome {
	case "/done":
		return "Verified by evidence: the build and the tests were both measured green after the final edit."
	case "/hollow":
		return "Not verified: the turn claimed work it has no evidence for."
	case "/failed":
		return "Failed: the workspace did not pass its mechanical checks."
	case "/unverified":
		if clause := DescribeMissingEvidence(result.MissingEvidence); clause != "" {
			return "Unverified: " + clause + "."
		}
		return "Unverified: the evidence this turn owed was not produced."
	default:
		return "Unverified: the turn produced no verdict."
	}
}
