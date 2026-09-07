package session

import (
	"context"
	"fmt"

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
					result.BuildCheck = verifyBuild(ctx, workspace, nil)
				}
				if e.configSnapshot().VerifyTestsAfterEdits {
					result.TestCheck = verifyTests(ctx, workspace, packagesForPaths(result.WrittenPaths))
				}
			}
			current, currentErr := evidence.Snapshot(ctx, workspace)
			if err == nil && currentErr == nil && current == after && result.BuildCheck.Ran && result.BuildCheck.OK && result.TestCheck.Ran && result.TestCheck.OK {
				result.ChecksSnapshot = current
				result.ChangeStage = "checks_passed"
			}
			if (result.BuildCheck.Ran && !result.BuildCheck.OK) || (result.TestCheck.Ran && !result.TestCheck.OK) {
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
		result.Response += fmt.Sprintf("\n\nEvidence: %s. Requested behavior remains unverified (no acceptance contract).", stage)
	}
}
