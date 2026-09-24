package campaign

import (
	"fmt"
	"sort"
	"strings"

	"codenerd/internal/logging"
)

// appendCheckpointRemediation gives a phase whose checkpoint failed the work
// its checkpoint found missing: one task, briefed with the checkpoint's
// findings whole and the phase's objectives, scoped to what the phase's tasks
// write. It is what the policy's /replan move does (phase_ckpt_move,
// campaign_decisions.mg); the phase stays open, runs the task, and its
// checkpoint runs again.
//
// Until 2026-09-24 the /replan move called the replanner with an empty reason
// and a bare /checkpoint_failed trigger, so the findings never reached anyone.
// Measured live on campaign 7b853890: the phase-6 reviewer named the exact
// defects the grader reports ("2 files miss front-matter and adr-slot claim
// unresolved"), the replan added nothing, the phase re-checked unchanged work
// 24 seconds later, and after three attempts it closed unverified.
//
// The findings are the symptom and the whole brief, as the acceptance
// command's output is for acceptance remediation: the task is told what the
// checkpoint saw, not how to fix it.
func (o *Orchestrator) appendCheckpointRemediation(phaseID string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	c := o.campaign
	var phase *Phase
	for i := range c.Phases {
		if c.Phases[i].ID == phaseID {
			phase = &c.Phases[i]
			break
		}
	}
	if phase == nil {
		return fmt.Errorf("checkpoint remediation: phase %s is not in the campaign", phaseID)
	}
	var failed *Checkpoint
	for i := len(phase.Checkpoints) - 1; i >= 0; i-- {
		if !phase.Checkpoints[i].Passed {
			failed = &phase.Checkpoints[i]
			break
		}
	}
	if failed == nil {
		return fmt.Errorf("checkpoint remediation: phase %s has no failed checkpoint", phaseID)
	}

	previous, err := cloneCampaign(c)
	if err != nil {
		return fmt.Errorf("checkpoint remediation: %w", err)
	}

	writeSet := phaseWriteSet(phase)
	if len(writeSet) == 0 {
		// A file task with no declared target is refused before it runs
		// (validateTaskEffect); appending one would block the phase.
		return fmt.Errorf("checkpoint remediation: phase %s declares no file its tasks write, so there is no scope to remediate", phaseID)
	}
	scope := strings.Join(writeSet, ", ")
	var objectives strings.Builder
	for _, obj := range phase.Objectives {
		fmt.Fprintf(&objectives, "- %s\n", obj.Description)
	}
	round := len(phase.Checkpoints)
	description := fmt.Sprintf(
		"The checkpoint of phase %q failed (%s, round %d). Every task of the phase is done and the phase is not.\n\n"+
			"What the checkpoint found:\n\n%s\n\nThe phase's objectives:\n\n%s\n"+
			"Fix what the findings name, in %s. The checkpoint runs again when this task is done. "+
			"Do not delete content or weaken a true statement to get there.",
		phase.Name, failed.Type, round, indentBlock(failed.Details), objectives.String(), scope)

	suffix := strings.TrimPrefix(c.ID, "/campaign_")
	phase.Tasks = append(phase.Tasks, Task{
		ID:          fmt.Sprintf("/task_%s_%d_ckpt_%d", suffix, phase.Order, round),
		PhaseID:     phase.ID,
		Description: description,
		Status:      TaskPending,
		Type:        TaskTypeFileModify,
		PlannedType: TaskTypeFileModify,
		Priority:    PriorityHigh,
		WriteSet:    writeSet,
	})
	c.TotalTasks++

	if err := syncCampaignFacts(o.kernel, previous, c, fmt.Sprintf("phase %s checkpoint round %d failed: remediation task appended", phase.ID, round)); err != nil {
		return fmt.Errorf("checkpoint remediation: %w", err)
	}
	o.persistCampaign(fmt.Sprintf("checkpoint remediation %s %d", phase.ID, round))
	logging.Campaign("Checkpoint remediation appended to %s (round %d, scope: %s)", phase.ID, round, scope)
	return nil
}

// phaseWriteSet is every path the phase's tasks declared they write -- their
// write sets and their artifacts, the targets validateTaskEffect accepts --
// deduplicated and sorted: a checkpoint remediation's scope.
func phaseWriteSet(p *Phase) []string {
	seen := map[string]bool{}
	var out []string
	add := func(path string) {
		if path = strings.TrimSpace(path); path != "" && !seen[path] {
			seen[path] = true
			out = append(out, path)
		}
	}
	for j := range p.Tasks {
		for _, path := range p.Tasks[j].WriteSet {
			add(path)
		}
		for _, artifact := range p.Tasks[j].Artifacts {
			add(artifact.Path)
		}
	}
	sort.Strings(out)
	return out
}
