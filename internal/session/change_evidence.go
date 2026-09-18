package session

import (
	"context"
	"fmt"
	"strings"

	"codenerd/internal/evidence"
)

// closeChangeEvidence runs after every model repair/critic edit. Later edits
// invalidate earlier green checks before the turn is allowed to complete.
func (e *Executor) closeChangeEvidence(ctx context.Context, result *ExecutionResult, before string) error {
	if result == nil {
		return nil
	}
	workspace := e.workspaceForVerification()
	if result.SuccessfulWriteTools > 0 {
		result.ChangeStage = "artifact_changed"
		if touchedGoFiles(result.WrittenPaths) {
			after, err := evidence.Snapshot(ctx, workspace)
			if err == nil && after != before {
				if e.configSnapshot().VerifyBuildAfterEdits {
					fresh := verifyBuild(ctx, workspace, nil)
					fresh.Repair = inheritRepair(fresh.Verdict(), result.BuildCheck.Repair)
					result.BuildCheck = fresh
				}
				if e.configSnapshot().VerifyTestsAfterEdits {
					// Must stay the same helper the post-edit gate uses (gateTests),
					// or a tag-gated package fails the turn twice over.
					fresh, _ := gateTests(ctx, workspace, result, false)
					fresh.Repair = inheritRepair(fresh.Verdict(), result.TestCheck.Repair)
					result.TestCheck = fresh
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
				return fmt.Errorf("%w: final workspace failed mechanical checks", ErrVerificationFailed)
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

func (e *Executor) appendEvidenceReport(ctx context.Context, result *ExecutionResult) {
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
		result.Response += "\n\n" + result.Acceptance.Summary()
	} else if result.SuccessfulWriteTools > 0 {
		if result.ChangeStage == "checks_passed" {
			current, err := evidence.Snapshot(ctx, e.workspaceForVerification())
			if err != nil || current != result.ChecksSnapshot {
				result.ChangeStage = "artifact_changed"
				result.ChecksSnapshot = ""
			}
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
		result.Response += fmt.Sprintf("\n\nWrote %d file(s): %s\nEvidence: %s. Requested behavior remains unverified (no acceptance contract).", len(result.WrittenPaths), written, stage)
	}
}
