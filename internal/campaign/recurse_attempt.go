package campaign

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/types"

	"github.com/google/uuid"
)

// recurseEvidenceLimit bounds the gate output an attempt's task carries. The
// tail is kept: test runners and compilers put the verdict and the failure's
// detail last.
const recurseEvidenceLimit = 12000

// RecurseFixPromptPhase and RecurseImprovePromptPhase are the JIT
// campaign_phase values that select the recurse attempt's instruction atoms
// (internal/prompt/atoms/campaign/recurse_attempt.yaml). The task string
// carries the finding and the numbers; the atom carries how to approach the
// work and what done means. The two sides name the same phase and cannot
// import each other.
const (
	RecurseFixPromptPhase     = "/recurse_fix"
	RecurseImprovePromptPhase = "/recurse_improve"
)

// RecurseAttemptCampaign is one recurse attempt as a campaign: one phase for
// the node, one task carrying the finding, the gate output that shows it, and
// the command that witnesses the fix. Every campaign door (the CLI, chat) runs
// an attempt the same way it runs any campaign.
//
// A fix attempt carries its finding's own check as the campaign's acceptance
// witness (R7): the attempt is done when the gate that reported the finding
// passes, not when the model says so. The witness runs through the existing
// acceptance engine -- settleAcceptance runs it once every phase is done and
// appends remediation turns until it passes or the round budget is spent --
// so a still-failing check gets further turns inside the attempt instead of
// costing the loop a whole cycle to learn the finding is still open. Without
// it settleAcceptance returns satisfied with no witness declared and the
// attempt completes on the model's word; the ratchet still reverts it a
// moment later, but the turn that could have fixed it is already over. The
// phase's own checkpoint stays VerifyNone: the witness is what verifies the
// phase, the same shape as an acceptance remediation.
//
// An attempt with no check declares no witness. That is every improvement:
// there is no single gate to re-run, only metrics the ratchet reads after
// the attempt, so completion stays the phases' as before.
func RecurseAttemptCampaign(workspace string, a RecurseAttempt) *Campaign {
	now := time.Now()
	campaignID := fmt.Sprintf("/campaign_%s", uuid.New().String()[:8])
	slug := sanitizeCampaignID(campaignID)
	phaseID := fmt.Sprintf("/phase_%s_0", campaignID[10:])
	scope := strings.Join(a.Node.Paths, ", ")
	if scope == "" {
		scope = "the whole tree"
	}
	title := fmt.Sprintf("Recurse cycle %d: %s: %s", a.Cycle, a.Node.ID, oneLine(a.Finding.Message))
	goal := fmt.Sprintf("Fix %s in %s so that `%s` passes, without making any other gate worse.", a.Finding.Target, a.Node.Title, strings.Join(a.Check, " "))
	objective := fmt.Sprintf("Fix %s (%s)", a.Finding.Target, a.Finding.Gate)
	promptPhase := RecurseFixPromptPhase
	task := recurseAttemptTask(a, scope)
	if a.Angle != "" {
		title = fmt.Sprintf("Recurse cycle %d: %s: %s", a.Cycle, a.Node.ID, a.Angle)
		goal = fmt.Sprintf("%s %s, moving a measured metric without making any gate or other metric worse.", strings.ToUpper(a.Angle[:1])+a.Angle[1:], a.Node.Title)
		objective = fmt.Sprintf("%s %s", a.Angle, a.Node.ID)
		promptPhase = RecurseImprovePromptPhase
		task = recurseImproveTask(a, scope)
	}

	c := &Campaign{
		ID:              campaignID,
		Type:            CampaignTypeRecurse,
		Title:           title,
		Goal:            goal,
		SourceMaterial:  []string{},
		KnowledgeBase:   filepath.Join(workspace, ".nerd", "campaigns", slug, "knowledge.db"),
		Status:          StatusActive,
		CreatedAt:       now,
		UpdatedAt:       now,
		Confidence:      1.0,
		ContextProfiles: buildContextProfiles(campaignID),
		RecurseWave:     a.Pass,
		PromptPhase:     promptPhase,
		TotalPhases:     1,
		TotalTasks:      1,
	}
	// The finding's own check is the acceptance witness: a.Check is the gate
	// command as run, placeholders expanded (recurse_cycle.sourceOf hands the
	// reporting run's Argv to the attempt). OKExitCodes is that gate's pass
	// rule, copied beside the command: the acceptance engine and the gate
	// that produced the finding then agree on which exits pass. Both slices
	// are copied because the attempt outlives the loop's gate state, and the
	// engine appends rounds beside the command.
	if len(a.Check) > 0 {
		c.Acceptance = &Acceptance{
			Command:     append([]string(nil), a.Check...),
			OKExitCodes: append([]int(nil), a.OKExitCodes...),
		}
	}
	c.Phases = []Phase{{
		ID:             phaseID,
		CampaignID:     campaignID,
		Name:           fmt.Sprintf("recurse:%s", a.Node.ID),
		Order:          0,
		Category:       "/recurse",
		Status:         PhasePending,
		ContextProfile: c.ContextProfiles[0].ID,
		Objectives: []PhaseObjective{{
			Type:               ObjectiveModify,
			Description:        objective,
			VerificationMethod: VerifyNone,
		}},
		EstimatedTasks:      1,
		EstimatedComplexity: "/medium",
		Tasks: []Task{{
			ID:          fmt.Sprintf("/task_%s_0_0", campaignID[10:]),
			PhaseID:     phaseID,
			Description: task,
			Status:      TaskPending,
			Type:        TaskTypeFileModify,
			PlannedType: TaskTypeFileModify,
			Priority:    PriorityHigh,
			WriteSet:    a.Node.Paths,
		}},
		Checkpoints: []Checkpoint{{Type: string(VerifyNone)}},
	}}
	return c
}

// recurseAttemptTask is the data the model is handed: what failed, where,
// the evidence, and the check. How to approach the fix and what done means
// are the campaign/recurse/fix atom, selected by RecurseFixPromptPhase.
func recurseAttemptTask(a RecurseAttempt, scope string) string {
	evidence := a.Evidence
	if len(evidence) > recurseEvidenceLimit {
		evidence = "...\n" + evidence[len(evidence)-recurseEvidenceLimit:]
	}
	var b strings.Builder
	fmt.Fprintf(&b, "FIX (recurse pass %d, cycle %d) in %s (%s).\n\n", a.Pass, a.Cycle, a.Node.Title, scope)
	fmt.Fprintf(&b, "The workspace's %s gate %s reports:\n  %s\n  target: %s\n\n", a.Finding.Kind, a.Finding.Gate, a.Finding.Message, a.Finding.Target)
	if len(a.Check) > 0 {
		fmt.Fprintf(&b, "Acceptance: `%s`\n", strings.Join(a.Check, " "))
	}
	if evidence != "" {
		fmt.Fprintf(&b, "\nGate output:\n```\n%s\n```\n", strings.TrimRight(evidence, "\n"))
	}
	writePriorAttempts(&b, a.Prior)
	return b.String()
}

// writePriorAttempts records what earlier attempts at the same work tried
// and why each was reverted, newest first, within recurseEvidenceLimit.
// The instruction not to repeat them is in the recurse attempt atoms; a
// retry that is not shown the diff makes the same change again, and the
// loop then stops the finding as stalled after two identical failures.
func writePriorAttempts(b *strings.Builder, prior []PriorAttempt) {
	if len(prior) == 0 {
		return
	}
	budget := recurseEvidenceLimit
	for i := len(prior) - 1; i >= 0 && budget > 0; i-- {
		p := prior[i]
		fmt.Fprintf(b, "\nCycle %d, reverted: %s\n", p.Cycle, p.Why)
		tried := p.Tried
		if len(tried) > budget {
			tried = tried[:budget] + "\n... (truncated)"
		}
		budget -= len(tried)
		if strings.TrimSpace(tried) != "" {
			fmt.Fprintf(b, "It changed:\n```diff\n%s\n```\n", strings.TrimRight(tried, "\n"))
		}
	}
}

// recurseImproveTask is an improvement attempt's data: the angle, where
// the numbers stand, and the north star an extend attempt draws on. What
// each angle must do is the campaign/recurse/improve atom, selected by
// RecurseImprovePromptPhase.
func recurseImproveTask(a RecurseAttempt, scope string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s (recurse pass %d, cycle %d) %s (%s).\n\n", strings.ToUpper(a.Angle), a.Pass, a.Cycle, a.Node.Title, scope)
	if len(a.Metrics) > 0 {
		b.WriteString("\nWhere the numbers stand now:\n")
		for _, name := range sortedMetricNames(a.Metrics) {
			v := a.Metrics[name]
			if name == "coverage" {
				fmt.Fprintf(&b, "  %s: %d.%02d%%\n", name, v/100, v%100)
				continue
			}
			fmt.Fprintf(&b, "  %s: %d\n", name, v)
		}
	}
	if a.NorthStar != "" && a.Angle == "extend" {
		fmt.Fprintf(&b, "\nNorth star:\n%s\n", a.NorthStar)
	}
	writePriorAttempts(&b, a.Prior)
	return b.String()
}

// withPromptPhase puts the campaign's JIT phase on the turn context the
// session executor compiles from. jit_compiler.mg treats /phase as a regime
// dimension: an atom that declares campaign_phases is excluded unless
// current_context(/phase, Tag) matches. An empty phase leaves the context
// untouched, including a nil one. An existing session context is copied so
// the phase does not drop the rest of the turn's session state.
func withPromptPhase(ctx context.Context, phase string) context.Context {
	phase = strings.TrimSpace(phase)
	if phase == "" {
		return ctx
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var sc types.SessionContext
	if base := types.GetSessionContext(ctx); base != nil {
		sc = *base
	}
	sc.CampaignActive = true
	sc.CampaignPhase = phase
	return types.WithSessionContext(ctx, &sc)
}

// ReleaseRecurseAttempt drops what an attempt's campaign left behind once it
// has run: its facts in the kernel and its files under
// .nerd/campaigns (the snapshot, its journal, its knowledge base). The recurse
// journal and the git history are the record of an attempt; a forever loop
// that also kept every attempt's campaign would grow without bound.
func ReleaseRecurseAttempt(workspace string, kernel core.Kernel, c *Campaign) error {
	if c == nil {
		return nil
	}
	var firstErr error
	if kernel != nil {
		if err := retractCampaignFacts(kernel, c); err != nil {
			firstErr = fmt.Errorf("recurse: retract attempt campaign facts: %w", err)
		}
	}
	slug := sanitizeCampaignID(c.ID)
	if slug == "" {
		return firstErr
	}
	matches, err := filepath.Glob(filepath.Join(workspace, ".nerd", "campaigns", slug+"*"))
	if err != nil && firstErr == nil {
		firstErr = err
	}
	for _, m := range matches {
		if err := os.RemoveAll(m); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("recurse: remove %s: %w", m, err)
		}
	}
	return firstErr
}
