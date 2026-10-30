package session

import (
	"context"
	"fmt"
	"strings"

	"codenerd/internal/observation"
	"codenerd/internal/types"
)

// ObservedTaskExecutor is the optional half of TaskExecutor: the same task run,
// but returning what the executor already knows about it instead of only the
// prose.
//
// It is an extension interface rather than a method on TaskExecutor because
// every consumer that wants the prose is correct to want the prose — the chat
// surface prints it to the user, the campaign's durable artifact IS it — and
// widening the interface would make five test doubles implement a method they
// have no answer for. A consumer that wants the structure type-asserts for
// this, which is the same idiom types.ModelIdentifier and types.ThinkingProvider
// already use for the same reason.
//
// The wire it opens is the point of it. ExecutionResult computes WrittenPaths,
// BuildCheck, TestCheck, UntestedPaths and CriticFindings on every turn, and
// SubAgent.execute returned result.Response and discarded all of it — so a
// parent asking "what did that agent change, and did it build" had to guess
// from prose that the runtime had already answered exactly.
type ObservedTaskExecutor interface {
	ExecuteObserved(ctx context.Context, req TaskRequest) (observation.Return, error)

	// ExecuteObservedWithContext is the observed twin of
	// TaskExecutor.ExecuteWithContext. It exists because the caller that most
	// needs the structure — the chat's delegation and continuation path — is
	// also the one that must pass a session context and a priority, and a
	// consumer forced to choose between the two would take the string.
	ExecuteObservedWithContext(ctx context.Context, req TaskRequest, sessionCtx *types.SessionContext, priority types.SpawnPriority) (observation.Return, error)
}

// observedReturn builds the raw observation from what the executor recorded.
//
// It reads only fields the runtime OBSERVED. Nothing here is inferred from the
// response text: that is the projection's job, and mixing the two would make
// "the runtime watched this build fail" indistinguishable from "the agent said
// its build failed" at the one place the difference decides whether the parent
// re-verifies.
func observedReturn(agent, task string, res *ExecutionResult) observation.Return {
	out := observation.Return{
		Agent: strings.TrimSpace(agent),
		Task:  strings.TrimSpace(task),
	}
	if res == nil {
		return out
	}

	out.Output = res.Response
	out.Duration = res.Duration
	if res.Error != nil {
		out.Failure = res.Error.Error()
	}

	// The kernel's verdict and the change stage are the two things the
	// executor already decided and every consumer downstream used to
	// re-invent by reading the prose. They cross here because this is the
	// only place both ends are in scope.
	out.Outcome = string(res.TurnOutcome)
	out.Missing = append(out.Missing, res.MissingEvidence...)
	out.Stage = res.ChangeStage
	out.Report = reportReadingOf(res)
	out.Snapshot = res.ChecksSnapshot
	if res.Acceptance != nil && res.Acceptance.Status == "verified" && res.Acceptance.After != "" {
		out.Snapshot = res.Acceptance.After
	}
	if res.Acceptance != nil {
		out.Acceptance = &observation.Acceptance{
			Status:   res.Acceptance.Status,
			Contract: res.Acceptance.ContractID,
			Summary:  res.Acceptance.Summary(),
		}
	}
	out.Untested = append(out.Untested, res.UntestedPaths...)

	// WrittenPaths is the write set of successful mutations, which is what
	// "changed artifacts" means when it is a fact rather than a claim.
	out.Changed = append(out.Changed, res.WrittenPaths...)
	out.Effects = projectReceipts(res.ToolReceipts)

	if res.BuildCheck.Ran || res.BuildCheck.OK || res.BuildCheck.Output != "" || res.BuildCheck.Outcome != "" {
		out.Build = &observation.Verification{
			Kind:    "build",
			Source:  observation.SourceObserved,
			Ran:     res.BuildCheck.Ran,
			OK:      res.BuildCheck.OK,
			Outcome: string(res.BuildCheck.Verdict()),
			Detail:  firstLine(res.BuildCheck.Output),
		}
	}
	// The observed suite is the turn's own run and the importer run
	// together (suiteExit): an own pass over a red or unfinished importer
	// run is not "tests OK". The charge is still the derived /test gate;
	// this reports what ran and how it exited.
	if res.TestCheck.Ran || res.TestCheck.OK || res.TestCheck.Output != "" || res.TestCheck.Outcome != "" || res.ImporterCheck.Ran {
		exit := suiteExit(res)
		out.Tests = &observation.Verification{
			Kind:    "tests",
			Source:  observation.SourceObserved,
			Ran:     res.TestCheck.Ran || res.ImporterCheck.Ran,
			OK:      exit == VerifyPassed,
			Outcome: string(exit),
			Detail:  firstLine(suiteFailureText(res)),
		}
	}

	for _, f := range res.CriticFindings {
		out.Findings = append(out.Findings, observation.Finding{
			File:     f.File,
			Line:     f.Line,
			Severity: f.Severity,
			Message:  f.Claim,
		})
	}

	// Untested and uncovered are the two shapes of "this turn went green
	// without proving anything", and both are advisory in the executor. They
	// are uncertainty rather than findings precisely because of that: a parent
	// that treats them as defects will send a fixer after code that is fine,
	// and a parent that never sees them will believe a write set was covered.
	if len(res.UntestedPaths) > 0 {
		out.Notes = append(out.Notes,
			"written with no test file alongside: "+strings.Join(res.UntestedPaths, " "))
	}
	if n := len(res.UncoveredBlocks); n > 0 {
		out.Notes = append(out.Notes,
			fmt.Sprintf("%d block(s) in this turn's own files executed by no test", n))
	}
	if d := strings.TrimSpace(res.StaticDiagnostics); d != "" {
		out.Notes = append(out.Notes, "static analysis reported: "+firstLine(d))
	}
	// The tool failures of a turn that ended in an error are part of why it
	// did. A turn that recovered owes none of them to its parent: the closing
	// evidence already answered them.
	if res.Error != nil {
		for _, failure := range res.ToolErrors {
			out.Notes = append(out.Notes, "tool call failed: "+failure)
		}
	}
	return out
}

// firstLine is the one line of a verification's output worth carrying into a
// projection. The rest of a compiler's or a runner's output is in the
// transcript behind the handle, where it costs nothing until it is asked for.
func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

// projectReceipts carries every tool-effect receipt to the parent, with
// content identities and no content.
func projectReceipts(receipts []ToolEffectReceipt) []observation.EffectReceipt {
	if len(receipts) == 0 {
		return nil
	}
	out := make([]observation.EffectReceipt, 0, len(receipts))
	for _, r := range receipts {
		er := observation.EffectReceipt{
			Call:         r.CallID,
			Tool:         r.Tool,
			Seq:          r.Seq,
			Validation:   string(r.Validation.State),
			Reason:       r.Validation.Reason,
			HandlerError: r.HandlerError,
			Recovery:     r.Recovery,
			Closure:      r.Closure,
		}
		for _, eff := range r.Effects {
			er.Effects = append(er.Effects, observation.ResourceEffect{
				Path: eff.Path, Before: eff.Before, After: eff.After, State: string(eff.State),
			})
		}
		out = append(out, er)
	}
	return out
}

// reportReadingOf is Jev's last reading of the turn's final report
// (turn_report_reading), carried so a caller that re-dispatches an unverified
// task can tell a report that names an obstacle from one that only stopped.
// The kernel cleared the turn's own row; the executor kept the fact.
func reportReadingOf(res *ExecutionResult) *observation.ReportReading {
	if res == nil || res.reportReading == nil {
		return nil
	}
	unfinished, okU := types.ArgInt64(*res.reportReading, 1)
	blocked, okB := types.ArgInt64(*res.reportReading, 2)
	if !okU || !okB {
		return nil
	}
	return &observation.ReportReading{Unfinished: unfinished, Blocked: blocked}
}
