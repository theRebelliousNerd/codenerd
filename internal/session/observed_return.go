package session

import (
	"context"
	"fmt"
	"strings"

	"codenerd/internal/observation"
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

	// WrittenPaths is the write set of successful mutations, which is what
	// "changed artifacts" means when it is a fact rather than a claim.
	out.Changed = append(out.Changed, res.WrittenPaths...)

	if res.BuildCheck.Ran || res.BuildCheck.OK || res.BuildCheck.Output != "" {
		out.Build = &observation.Verification{
			Kind:   "build",
			Source: observation.SourceObserved,
			Ran:    res.BuildCheck.Ran,
			OK:     res.BuildCheck.OK,
			Detail: firstLine(res.BuildCheck.Output),
		}
	}
	if res.TestCheck.Ran || res.TestCheck.OK || res.TestCheck.Output != "" {
		out.Tests = &observation.Verification{
			Kind:   "tests",
			Source: observation.SourceObserved,
			Ran:    res.TestCheck.Ran,
			OK:     res.TestCheck.OK,
			Detail: firstLine(res.TestCheck.Output),
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
