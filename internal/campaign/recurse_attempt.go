package campaign

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

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

	c := &Campaign{
		ID:              campaignID,
		Type:            CampaignTypeRecurse,
		Title:           title,
		Goal:            fmt.Sprintf("Fix %s in %s so that `%s` passes, without making any other gate worse.", a.Finding.Target, a.Node.Title, strings.Join(a.Check, " ")),
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
			Description:        fmt.Sprintf("Fix %s (%s)", a.Finding.Target, a.Finding.Gate),
			VerificationMethod: VerifyNone,
		}},
		EstimatedTasks:      1,
		EstimatedComplexity: "/medium",
		Tasks: []Task{{
			ID:          fmt.Sprintf("/task_%s_0_0", campaignID[10:]),
			PhaseID:     phaseID,
			Description: recurseAttemptTask(a, scope),
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
