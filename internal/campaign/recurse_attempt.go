package campaign

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"codenerd/internal/core"

	"github.com/google/uuid"
)

// recurseEvidenceLimit bounds the gate output an attempt's task carries. The
// tail is kept: test runners and compilers put the verdict and the failure's
// detail last.
const recurseEvidenceLimit = 12000

// RecurseAttemptCampaign is one recurse attempt as a campaign: one phase for
// the node, one task carrying the finding, the gate output that shows it, and
// the command that witnesses the fix. Every campaign door (the CLI, chat) runs
// an attempt the same way it runs any campaign.
//
// The phase's own checkpoint is VerifyNone on purpose. The loop re-runs the
// gates after the attempt and the kernel judges it (recurse_ratchet); a second
// verdict from inside the attempt would be one the loop never reads.
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
	task := recurseAttemptTask(a, scope)
	if a.Angle != "" {
		title = fmt.Sprintf("Recurse cycle %d: %s: %s", a.Cycle, a.Node.ID, a.Angle)
		goal = fmt.Sprintf("%s %s, moving a measured metric without making any gate or other metric worse.", strings.ToUpper(a.Angle[:1])+a.Angle[1:], a.Node.Title)
		objective = fmt.Sprintf("%s %s", a.Angle, a.Node.ID)
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
		TotalPhases:     1,
		TotalTasks:      1,
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

// recurseAttemptTask is the task the model is handed: what failed, where,
// the evidence, the check, and how the change will be judged.
func recurseAttemptTask(a RecurseAttempt, scope string) string {
	evidence := a.Evidence
	if len(evidence) > recurseEvidenceLimit {
		evidence = "...\n" + evidence[len(evidence)-recurseEvidenceLimit:]
	}
	var b strings.Builder
	fmt.Fprintf(&b, "FIX (recurse pass %d, cycle %d) in %s (%s).\n\n", a.Pass, a.Cycle, a.Node.Title, scope)
	fmt.Fprintf(&b, "The workspace's %s gate %s reports:\n  %s\n  target: %s\n\n", a.Finding.Kind, a.Finding.Gate, a.Finding.Message, a.Finding.Target)
	if len(a.Check) > 0 {
		fmt.Fprintf(&b, "Acceptance: `%s` passes and no longer reports this.\n", strings.Join(a.Check, " "))
	}
	b.WriteString("After this task the loop re-runs this gate and the workspace's build and lint gates. The change is kept only if this finding is gone and no gate is worse; otherwise every write is reverted.\n")
	b.WriteString("Fix the cause with the smallest change that does it. Do not weaken, skip or delete a test or a check to make it pass.\n")
	if evidence != "" {
		fmt.Fprintf(&b, "\nGate output:\n```\n%s\n```\n", strings.TrimRight(evidence, "\n"))
	}
	return b.String()
}

// recurseImproveTask is an improvement attempt's task: the angle, what it must
// move, where the numbers stand, and how the change will be judged.
func recurseImproveTask(a RecurseAttempt, scope string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s (recurse pass %d, cycle %d) %s (%s).\n\n", strings.ToUpper(a.Angle), a.Pass, a.Cycle, a.Node.Title, scope)
	switch a.Angle {
	case "stabilize":
		b.WriteString("Find behaviour in this node that no test pins yet and add tests that pin it; fix any flaky test you find. Kept only if the workspace's test count rises.\n")
	case "harden":
		b.WriteString("Find error paths, edge cases and unvalidated inputs in this node that no test exercises. Add tests for them and fix what they expose, so failures fail closed with honest errors. Kept only if the node's coverage or the workspace's test count rises.\n")
	case "simplify":
		b.WriteString("Remove dead code, duplication and needless complexity in this node without changing its behaviour. Kept only if the node's source lines drop while every test still passes.\n")
	case "extend":
		b.WriteString("Add one capability this node is missing -- one its own docs, its TODOs or the north star below ask for -- with a test that proves it. Kept only if the workspace's test count rises.\n")
	default:
		fmt.Fprintf(&b, "Improve this node from the %s angle.\n", a.Angle)
	}
	b.WriteString("Every gate must stay at least as green, no test may be removed, and the node's coverage may not drop; a change that moves no metric, or makes anything worse, is reverted in full.\n")
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
	return b.String()
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
