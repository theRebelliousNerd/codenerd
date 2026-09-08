package campaign

import (
	"fmt"
	"strings"
)

func requiresFileTarget(kind TaskType) bool {
	switch kind {
	case TaskTypeFileCreate, TaskTypeFileModify, TaskTypeTestWrite, TaskTypeDocument, TaskTypeRefactor, TaskTypeIntegrate:
		return true
	}
	return false
}

// validateTaskEffect uses declared task kinds, never words in model prose.
func validateTaskEffect(task *Task) error {
	if task == nil {
		return fmt.Errorf("nil task")
	}
	if task.PlannedType != "" && isMutatingTaskType(task.PlannedType) && !isMutatingTaskType(task.Type) {
		return fmt.Errorf("task %s downgraded its planned %s effect to %s", task.ID, task.PlannedType, task.Type)
	}
	if task.PlannedType == TaskTypeTestRun && task.Type != TaskTypeTestRun {
		return fmt.Errorf("task %s lost its planned test execution obligation", task.ID)
	}
	hasTarget := extractPathFromDescription(task.Description) != ""
	for _, path := range task.WriteSet {
		hasTarget = hasTarget || strings.TrimSpace(path) != ""
	}
	for _, artifact := range task.Artifacts {
		hasTarget = hasTarget || strings.TrimSpace(artifact.Path) != ""
	}
	if requiresFileTarget(task.Type) && !hasTarget {
		return fmt.Errorf("task %s requires a declared file target or write set; refine the plan without changing its execution obligation", task.ID)
	}
	return nil
}

func taskEffectIssues(c *Campaign) []PlanValidationIssue {
	var issues []PlanValidationIssue
	for _, p := range c.Phases {
		for n := range p.Tasks {
			if err := validateTaskEffect(&p.Tasks[n]); err != nil {
				issues = append(issues, PlanValidationIssue{CampaignID: c.ID, IssueType: "/ambiguous_goal", Description: err.Error()})
			}
		}
	}
	return issues
}

// A check placed after edits must await those edits. This conservative default
// can later be narrowed using verified dependency information.
func orderVerificationAfterEdits(phase *Phase) {
	var edits []string
	for n := range phase.Tasks {
		t := &phase.Tasks[n]
		if t.Type == TaskTypeTestRun || t.Type == TaskTypeVerify {
			seen := map[string]bool{}
			for _, id := range t.DependsOn {
				seen[id] = true
			}
			for _, id := range edits {
				if !seen[id] {
					t.DependsOn = append(t.DependsOn, id)
					seen[id] = true
				}
			}
		}
		if isMutatingTaskType(t.Type) {
			edits = append(edits, t.ID)
		}
	}
}

// Refiner output must retain the same target representation as decomposition.
func applyRefinedTargets(workspace string, task *Task, artifacts, writes []string) {
	if artifacts != nil {
		task.Artifacts = nil
		for _, path := range artifacts {
			if strings.TrimSpace(path) != "" {
				task.Artifacts = append(task.Artifacts, TaskArtifact{Type: "/source_file", Path: path})
			}
		}
	}
	if writes != nil {
		task.WriteSet = normalizeWriteSetPaths(workspace, writes)
	}
	if len(task.WriteSet) == 0 && isMutatingTaskType(task.Type) {
		var paths []string
		for _, a := range task.Artifacts {
			paths = append(paths, a.Path)
		}
		if len(paths) == 0 {
			if path := extractPathFromDescription(task.Description); path != "" {
				paths = append(paths, path)
			}
		}
		task.WriteSet = normalizeWriteSetPaths(workspace, paths)
	}
}
