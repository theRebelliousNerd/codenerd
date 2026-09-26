package session

import (
	"context"
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

// finalReportAuditSystem is the classifier's instruction.
const finalReportAuditSystem = "You read an agent's final report on a task and answer with exactly one word: INCOMPLETE or COMPLETE."

// finalReportAuditPrompt asks whether the report itself admits unfinished work.
// Caveats and suggested follow-ups are not admissions: the question is only
// whether the report says the requested work was not all done. Nor is "I
// could not run the check": verification is what the gates and the
// campaign's checks do after the turn. Until 2026-09-26 the question also
// asked whether the work "was not verified", and campaign 7b853890's
// acceptance fix, done and honest about the checker it had no tool for, was
// read INCOMPLETE and rolled back.
func finalReportAuditPrompt(task, report string) string {
	var b strings.Builder
	if strings.TrimSpace(task) != "" {
		b.WriteString("Task:\n")
		b.WriteString(task)
		b.WriteString("\n\n")
	}
	b.WriteString("Final report:\n")
	b.WriteString(report)
	b.WriteString("\n\nDoes the report itself say that part of the requested work was not done, could not be finished, " +
		"or was left for another run? Answer INCOMPLETE if it says so. " +
		"Answer COMPLETE if it reports the work as done, including when it adds caveats, says it could not run a check itself, " +
		"or suggests optional follow-up work.")
	return b.String()
}

// auditFinalReport asks the main model whether the turn's final report admits
// the work is unfinished, and records a yes on the result for the verdict.
//
// Only turns that wrote, or whose intent owed a write, are read: the admission
// is about requested work, and a question answered with "that cannot be
// determined from the code" is an answer, not unfinished work.
func (e *Executor) auditFinalReport(ctx context.Context, result *ExecutionResult) {
	if result == nil || result.Error != nil || !e.configSnapshot().AuditFinalReport {
		return
	}
	report := strings.TrimSpace(result.Response)
	if report == "" || (result.SuccessfulWriteTools == 0 && !e.writeOrientedIntent(result.Intent.Verb)) {
		return
	}
	e.mu.RLock()
	client := e.llmClient
	e.mu.RUnlock()
	if client == nil {
		return
	}
	task := ""
	if loop := activeWorkingLoop(ctx); loop != nil {
		task = loop.task
	}
	if len(task) > criticMaxFileBytes {
		task = task[:criticMaxFileBytes]
	}
	if len(report) > criticMaxFileBytes {
		// The admission, when there is one, is in the closing paragraphs.
		report = report[len(report)-criticMaxFileBytes:]
	}
	ctx = broker.WithPhase(ctx, broker.PhaseAdmissionAudit)
	answer, err := client.CompleteWithSystem(ctx, finalReportAuditSystem, finalReportAuditPrompt(task, report))
	if err != nil {
		logging.Get(logging.CategorySession).Warn("final-report audit unavailable (%v); the verdict is taken without it", err)
		return
	}
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(answer)), "INCOMPLETE") {
		result.SelfReportedIncomplete = true
		logging.Get(logging.CategorySession).Warn("the turn's own report says the work is not finished; /done is withheld")
	}
}
