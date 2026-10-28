package chat

import (
	"context"
	"fmt"
	"strings"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/turnjudge"
	"codenerd/internal/types"
	"codenerd/internal/usage"
)

// Jev judgments for a chat turn (internal/turnjudge, policy/jev_evidence.mg).
// They are asked in parallel with perception and land in the kernel before
// routing as evidence the rules may use to ask first, recognise a go-ahead,
// close an answered brief question, start the self-diagnosis chain, stop
// background work, or add a restriction. Routing waits for them, so the call
// is bounded by typesafe.turn_judgment_timeout: at worst a turn waits that
// long past perception, and a call still out then is no evidence. With Jev
// off, or its call failed, the turn routes exactly as it would have without
// it. Every answered call is billed to the usage tracker (jevContext).

// jevContext carries the session's usage tracker and identity, so a Jev
// call's tokens are billed wherever it runs: the turn, a side turn, a
// background review, the self-diagnosis chain.
func (m Model) jevContext(ctx context.Context) context.Context {
	if m.usageTracker != nil && usage.FromContext(ctx) == nil {
		ctx = usage.NewContext(ctx, m.usageTracker)
	}
	if usage.SessionIDFromContext(ctx) == "" && m.sessionID != "" {
		ctx = usage.WithSessionID(ctx, m.sessionID)
	}
	return ctx
}

// judgedTurn is the outcome of one turn's judgments.
type judgedTurn struct {
	result *turnjudge.Result
	err    error
	took   time.Duration
}

// startTurnJudgments asks Jev about the turn in the background and returns
// the channel its outcome arrives on; nil when Jev is off.
func (m Model) startTurnJudgments(ctx context.Context, input string, history []perception.ConversationTurn) <-chan judgedTurn {
	uses := m.Config.TypeSafe.Uses()
	if m.jev == nil || !uses.TurnJudgments {
		return nil
	}
	in := turnjudge.Input{Message: input, Brief: m.currentBrief(), HeldWork: describeHeldWork(m.pendingWork), RecentTurns: uses.RecentTurns}
	for _, h := range history {
		if strings.TrimSpace(h.Content) == "" {
			continue
		}
		in.Recent = append(in.Recent, turnjudge.Turn{From: h.Role, Text: h.Content})
	}
	out := make(chan judgedTurn, 1)
	jctx, cancel := context.WithTimeout(m.jevContext(ctx), uses.TurnJudgmentTimeout)
	go func() {
		defer cancel()
		began := time.Now()
		res, err := turnjudge.Judge(jctx, m.jev, in)
		out <- judgedTurn{result: res, err: err, took: time.Since(began)}
	}()
	return out
}

// recordTurnJudgments waits for the judgments and replaces last turn's
// evidence with them. A failure leaves no evidence, never a guess.
func (m Model) recordTurnJudgments(ctx context.Context, judged <-chan judgedTurn) {
	if m.kernel == nil {
		return
	}
	for _, pred := range turnjudge.Predicates {
		_ = m.kernel.Retract(pred)
	}
	if judged == nil {
		return
	}
	var j judgedTurn
	waitStart := time.Now()
	select {
	case j = <-judged:
	case <-ctx.Done():
		logging.Get(logging.CategoryPerception).Warn("jev: turn ended before the judgments arrived: %v", ctx.Err())
		return
	}
	// waited is what Jev cost the turn's latency: the time routing sat on the
	// judgments after perception was done.
	waited := time.Since(waitStart).Round(time.Millisecond)
	log := logging.Get(logging.CategoryPerception)
	if j.err != nil {
		log.Warn("jev: no evidence this turn (call %s, routing waited %s): %v", j.took.Round(time.Millisecond), waited, j.err)
		return
	}
	if err := config.EnsureParams(m.kernel, m.Config.TypeSafe.Params()); err != nil {
		log.Warn("jev: thresholds not asserted, evidence dropped: %v", err)
		return
	}
	if err := m.kernel.AssertBatch(j.result.Facts()); err != nil {
		log.Error("jev: assert judgments: %v", err)
		return
	}
	log.Info("jev: %s in %s (routing waited %s)", j.result.Summary(), j.took.Round(time.Millisecond), waited)
}

// describeHeldWork is the held work as a person would read it.
func describeHeldWork(w *pendingWork) string {
	if w == nil {
		return ""
	}
	var b strings.Builder
	if verb := strings.TrimPrefix(w.Intent.Verb, "/"); verb != "" {
		fmt.Fprintf(&b, "%s %s", verb, strings.TrimSpace(w.Intent.Target))
		if r := strings.TrimSpace(w.Route.Shard); r != "" {
			fmt.Fprintf(&b, " (by the %s)", r)
		}
		b.WriteString(", requested as: ")
	}
	b.WriteString(strings.TrimSpace(w.Input))
	return b.String()
}

// stopRequestedBackground cancels running background work when the kernel
// derives that the user asked to stop it (stop_background_requested,
// jev_evidence.mg). It returns the jobs it stopped.
func (m Model) stopRequestedBackground() []backgroundJob {
	if m.kernel == nil {
		return nil
	}
	rows, err := m.kernel.Query("stop_background_requested")
	if err != nil || len(rows) == 0 {
		return nil
	}
	return m.background.cancelAll()
}

// relevanceUse is Jev's Tier 2 relevance judging for the chat's retrieval
// seed (turnjudge.RelevanceFor); its Judge is nil when Jev or the use is off.
func (m Model) relevanceUse() turnjudge.RelevanceUse {
	return turnjudge.RelevanceFor(m.jev, m.Config.TypeSafe)
}

// reviewerShard is the shard whose findings are checked against their code.
const reviewerShard = "reviewer"

// checkReviewFindings asks Jev whether each reviewer finding is visible in the
// code it cites (turnjudge.CheckFindings), records the verdicts for the kernel
// (review_finding_check), and returns a note naming each finding the kernel
// derives as refuted; "" when none is, or Jev is off, or the check failed.
func (m Model) checkReviewFindings(ctx context.Context, reviewID, result string) string {
	uses := m.Config.TypeSafe.Uses()
	if m.jev == nil || m.kernel == nil || !uses.ReviewChecks {
		return ""
	}
	var findings []turnjudge.Finding
	for _, f := range extractFindings(result) {
		file, _ := f["file"].(string)
		line, _ := f["line"].(int)
		msg, _ := f["message"].(string)
		findings = append(findings, turnjudge.Finding{File: file, Line: line, Message: msg})
	}
	log := logging.Get(logging.CategoryShards)
	verdicts, err := turnjudge.CheckFindings(m.jevContext(ctx), m.jev, m.workspace, findings,
		turnjudge.FindingOptions{BatchSize: uses.BatchSize, ContextLines: uses.FindingContextLines})
	if err != nil {
		log.Warn("jev: review %s findings unchecked: %v", reviewID, err)
		return ""
	}
	if len(verdicts) == 0 {
		return ""
	}
	if err := config.EnsureParams(m.kernel, m.Config.TypeSafe.Params()); err != nil {
		log.Warn("jev: thresholds not asserted, review %s unchecked: %v", reviewID, err)
		return ""
	}
	if err := m.kernel.AssertBatch(turnjudge.FindingFacts(reviewID, verdicts)); err != nil {
		log.Error("jev: assert review %s verdicts: %v", reviewID, err)
		return ""
	}
	refuted, err := turnjudge.Refuted(m.kernel, reviewID, verdicts)
	if err != nil {
		log.Warn("jev: review %s refutations unread: %v", reviewID, err)
		return ""
	}
	var b strings.Builder
	for _, v := range refuted {
		fmt.Fprintf(&b, "\n- Not visible in %s:%d (Jev %d%%): %s", v.File, v.Line, types.PercentFromRatio(v.Probability), v.Message)
	}
	if b.Len() == 0 {
		return ""
	}
	return "\n\n**Findings Jev could not see in the code they cite** (verify before acting):" + b.String()
}

// jevRestriction is the restriction the kernel derives from Jev's judgments
// (jev_restriction, policy/jev_evidence.mg); zero when there is none.
func (m Model) jevRestriction() perception.Restriction {
	if m.kernel == nil {
		return perception.Restriction{}
	}
	rows, err := m.kernel.Query("jev_restriction")
	if err != nil {
		return perception.Restriction{}
	}
	var kinds []perception.RestrictionKind
	for _, r := range rows {
		if len(r.Args) == 1 {
			kinds = append(kinds, perception.RestrictionKind(strings.TrimPrefix(types.ExtractString(r.Args[0]), "/")))
		}
	}
	return perception.RestrictionOf("Jev read a limit on this request", kinds...)
}
