package session

import (
	"context"
	"fmt"
	"strings"

	"codenerd/internal/broker"
	"codenerd/internal/logging"
)

// Admission against interest.
//
// A model's claim that it finished is not evidence, which is why the gates
// exist. Its admission that it did not finish is: nothing rewards a model for
// saying it left work undone, so when its own report says so the report is
// believed. N41 (Docs/journeys/05-elite-harness-ladder.md, R5-3) closed /done
// one paragraph after "I cannot execute the remaining 177 deletions ...
// Requesting re-invocation": every gate was green, because the eleven
// deletions it made were correct, and nothing read the report.
//
// Reading prose for meaning is classification, and classification belongs to
// a model (a phrase scan over the response is the text match the executive
// literal budget forbids). So a model reads the report and answers one word;
// the kernel decides what the answer means for the verdict
// (turn_self_reported_incomplete in coder_safety.mg). The answer can only
// withhold /done: an unavailable or unparseable reading asserts nothing.

// finalReportAuditPrompt is the audit's message: the task and the report.
// The question -- does the report itself admit unfinished work -- and the
// one-word reply contract are the auditor's role prompt
// (atoms/system/final_report_auditor.yaml). Caveats and suggested follow-ups
// are not admissions: the question is only whether the report says the
// requested work was not all done. Nor is "I could not run the check":
// verification is what the gates and the campaign's checks do after the turn.
// Until 2026-09-26 the question also asked whether the work "was not
// verified", and campaign 7b853890's acceptance fix, done and honest about the
// checker it had no tool for, was read INCOMPLETE and rolled back.
func finalReportAuditPrompt(task, report string) string {
	var b strings.Builder
	if strings.TrimSpace(task) != "" {
		b.WriteString("Task:\n")
		b.WriteString(task)
		b.WriteString("\n\n")
	}
	b.WriteString("Final report:\n")
	b.WriteString(report)
	b.WriteString("\n")
	return b.String()
}

// auditFinalReport asks the main model whether the turn's final report admits
// the work is unfinished, and records a yes on the result for the verdict.
// input is the turn's request: the question is whether the report admits the
// *requested* work is unfinished, so the classifier reads both. It used to
// look the request up on the working loop, which lives on a context the tool
// loop derives and drops before the turn closes -- the audit never saw a task
// (campaign 7b853890, 2026-09-26: no "Task:" in any of its prompts).
//
// Only turns that wrote, or whose intent owed a write, are read: the admission
// is about requested work, and a question answered with "that cannot be
// determined from the code" is an answer, not unfinished work.
//
// The returned error is the audit's own failure to be asked: its role prompt
// did not compile. A model that fails to answer is not an error; the verdict
// is taken without it.
func (e *Executor) auditFinalReport(ctx context.Context, input string, result *ExecutionResult) error {
	if result == nil || result.Error != nil || !e.configSnapshot().AuditFinalReport {
		return nil
	}
	report := strings.TrimSpace(result.Response)
	if report == "" || (result.WriteToolEffects == 0 && !e.writeOrientedIntent(result.Intent.Verb)) {
		return nil
	}
	e.mu.RLock()
	client := e.llmClient
	e.mu.RUnlock()
	if client == nil {
		return nil
	}
	task := input
	if len(task) > criticMaxFileBytes {
		// Marked, not silent: the auditor must know it read the head of the
		// request only.
		task = task[:criticMaxFileBytes] + "\n... (task truncated for the audit)"
	}
	if len(report) > criticMaxFileBytes {
		// The admission, when there is one, is in the closing paragraphs.
		report = report[len(report)-criticMaxFileBytes:]
	}
	// The audit can withhold /done; without its role prompt it cannot be
	// asked, and a turn that skipped it would be read as one it passed. The
	// missing prompt is the turn's error.
	system, err := e.rolePrompt(ctx, finalReportAuditorRole)
	if err != nil {
		return fmt.Errorf("final-report audit: %w", err)
	}
	ctx = broker.WithPhase(ctx, broker.PhaseAdmissionAudit)
	answer, err := client.CompleteWithSystem(ctx, system, finalReportAuditPrompt(task, report))
	if err != nil {
		logging.Get(logging.CategorySession).Warn("final-report audit unavailable (%v); the verdict is taken without it", err)
		return nil
	}
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(answer)), "INCOMPLETE") {
		result.SelfReportedIncomplete = true
		logging.Get(logging.CategorySession).Warn("the turn's own report says the work is not finished; /done is withheld")
	}
	return nil
}
