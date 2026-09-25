package session

import (
	"context"
	"strings"
)

// IssueRetriever runs the kernel-gated retrieval pass for one task turn:
// the kernel decides whether the turn retrieves (issue_retrieval_wanted) and
// which retrieved files the model is handed (retrieval_brief_file), and the
// retriever returns the rendered brief ("" for none) and a release that
// retracts every fact the pass asserted. retrieval.TaskRetriever is the
// production implementation; the interface keeps the executor from owning a
// filesystem scanner.
//
// Until 2026-09-25 only the chat seeded issue facts, and only for the chat
// compressor: a `nerd fix`, a delegated task and a campaign task never ran a
// pass, and no path handed what a pass found to a model.
type IssueRetriever interface {
	Retrieve(ctx context.Context, intentID, taskText string) (brief string, release func())
}

// SetIssueRetriever attaches the per-turn retrieval pass. Nil turns it off.
func (e *Executor) SetIssueRetriever(r IssueRetriever) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.issueRetriever = r
}

// HasIssueRetriever reports whether the per-turn retrieval pass is attached,
// so boot wiring can be asserted from outside the package.
func (e *Executor) HasIssueRetriever() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.issueRetriever != nil
}

type retrievalBriefKey struct{}

// retrieveForTurn runs the pass for intentID and carries the brief on the
// turn's context, where the working loop picks it up for its anchor. The
// returned release must run when the turn ends.
func (e *Executor) retrieveForTurn(ctx context.Context, intentID, input string) (context.Context, func()) {
	e.mu.RLock()
	r := e.issueRetriever
	e.mu.RUnlock()
	if r == nil {
		return ctx, func() {}
	}
	brief, release := r.Retrieve(ctx, intentID, input)
	if release == nil {
		release = func() {}
	}
	if strings.TrimSpace(brief) == "" {
		return ctx, release
	}
	return context.WithValue(ctx, retrievalBriefKey{}, brief), release
}

// retrievalBrief is the turn's retrieval brief, or "".
func retrievalBrief(ctx context.Context) string {
	brief, _ := ctx.Value(retrievalBriefKey{}).(string)
	return brief
}

// withRetrievalBrief appends the turn's brief to a task text: the working
// loop's anchor, or the single request a client without a message channel
// gets. It rides the task rather than the system prompt for the reason the
// focus view does -- it is evidence for this task, not instructions -- and it
// is part of the anchor, so a ledger compaction does not take it.
func withRetrievalBrief(ctx context.Context, task string) string {
	brief := retrievalBrief(ctx)
	if brief == "" {
		return task
	}
	return task + "\n\n" + brief
}
