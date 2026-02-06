package campaign

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/tools/research"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Replanner handles campaign replanning when things go wrong.
// It uses LLM + Mangle collaboration to adapt the plan.
type Replanner struct {
	kernel         core.Kernel
	llmClient      perception.LLMClient
	promptProvider PromptProvider // Optional JIT prompt provider

	// Gemini advanced features (nil if not Gemini or features unavailable)
	grounding *research.GroundingHelper // Google Search / URL Context grounding
	thinking  *research.ThinkingHelper  // Thinking mode metadata capture
}

// NewReplanner creates a new replanner.
func NewReplanner(kernel core.Kernel, llmClient perception.LLMClient) *Replanner {
	r := &Replanner{
		kernel:         kernel,
		llmClient:      llmClient,
		promptProvider: NewStaticPromptProvider(), // Default to static prompts
	}

	// Initialize Gemini advanced features helpers
	if llmClient != nil {
		r.grounding = research.NewGroundingHelper(llmClient)
		r.thinking = research.NewThinkingHelper(llmClient)

		// Enable Google Search grounding for research-intensive replanning
		if r.grounding.IsGroundingAvailable() {
			r.grounding.EnableGoogleSearch()
			logging.CampaignDebug("Gemini grounding enabled for replanner (Google Search active)")
		}
	}

	return r
}

// SetPromptProvider sets the PromptProvider for JIT-compiled prompts.
// This allows using JIT-compiled prompts from the articulation package.
// If not set, static prompts will be used.
func (r *Replanner) SetPromptProvider(provider PromptProvider) {
	r.promptProvider = provider
}

// completeWithGrounding performs an LLM completion with grounding if available.
// Falls back to standard Complete if grounding is not available.
func (r *Replanner) completeWithGrounding(ctx context.Context, prompt string) (string, error) {
	if r.grounding != nil && r.grounding.IsGroundingAvailable() {
		response, sources, err := r.grounding.CompleteWithGrounding(ctx, prompt)
		if err != nil {
			return "", err
		}
		if len(sources) > 0 {
			logging.CampaignDebug("Replanner LLM call grounded with %d sources", len(sources))
		}
		// Capture thinking metadata after grounded call
		if r.thinking != nil {
			r.thinking.CaptureThinkingMetadata()
		}
		return response, nil
	}
	// Fall back to standard completion
	if r.llmClient == nil {
		return "", fmt.Errorf("LLM client not available for fallback completion")
	}
	return r.llmClient.Complete(ctx, prompt)
}

// ReplanReason represents why a replan was triggered.
type ReplanReason string

const (
	ReplanTaskFailed       ReplanReason = "/task_failed"
	ReplanNewRequirement   ReplanReason = "/new_requirement"
	ReplanUserFeedback     ReplanReason = "/user_feedback"
	ReplanDependencyChange ReplanReason = "/dependency_change"
	ReplanBlocked          ReplanReason = "/blocked"
)

// ReplanResult represents the outcome of replanning.
type ReplanResult struct {
	Success       bool
	ChangeSummary string
	AddedTasks    []Task
	RemovedTasks  []string
	ModifiedTasks []Task
	NewPhases     []Phase
}

// Replan adapts the campaign plan based on current state and failures.
// failedTaskID is optional; if provided, replanning is scoped to that task's subtree.
func (r *Replanner) Replan(ctx context.Context, campaign *Campaign, failedTaskID string) error {
	// 1. Gather context about what went wrong
	failedTasks := r.getFailedTasks(campaign)
	blockedTasks := r.getBlockedTasks(campaign)
	replanTriggers := r.getReplanTriggers(campaign.ID)

	if len(failedTasks) == 0 && len(blockedTasks) == 0 && len(replanTriggers) == 0 && failedTaskID == "" {
		return nil // Nothing to replan
	}

	// 2. Build context for LLM
	// If failedTaskID is set, we could filter context to relevant subtree,
	// but for now we provide full context for global consistency.
	contextSummary := r.buildReplanContext(campaign, failedTasks, blockedTasks, replanTriggers)

	// 3. Ask LLM to propose fixes
	fixes, err := r.proposeReplans(ctx, campaign, contextSummary, failedTaskID)
	if err != nil {
		return fmt.Errorf("failed to propose replans: %w", err)
	}

	// 4. Apply fixes
	if err := r.applyFixes(campaign, fixes); err != nil {
		return fmt.Errorf("failed to apply fixes: %w", err)
	}

	// 5. Record revision
	campaign.RevisionNumber++
	campaign.LastRevision = fixes.ChangeSummary

	// 6. Reload campaign into kernel
	facts := campaign.ToFacts()
	if err := r.kernel.LoadFacts(facts); err != nil {
		return fmt.Errorf("failed to reload campaign: %w", err)
	}

	// 7. Record plan revision
	r.kernel.Assert(core.Fact{
		Predicate: "plan_revision",
		Args:      []interface{}{campaign.ID, campaign.RevisionNumber, fixes.ChangeSummary, time.Now().Unix()},
	})

	return nil
}

// ReplanForNewRequirement adds tasks for a new requirement.
func (r *Replanner) ReplanForNewRequirement(ctx context.Context, campaign *Campaign, requirement string) error {
	// 1. Trigger replan
	r.kernel.Assert(core.Fact{
		Predicate: "replan_trigger",
		Args:      []interface{}{campaign.ID, "/new_requirement", time.Now().Unix()},
	})

	// 2. Ask LLM to incorporate new requirement
	prompt := fmt.Sprintf(`A campaign is in progress and a new requirement has been added.

Current Campaign: %s
Current Phase: %d of %d
Completed Tasks: %d of %d

New Requirement: %s

Determine what new tasks need to be added and which phase they belong to.
Output JSON:
{
  "new_tasks": [
    {"phase_order": 0, "description": "...", "type": "/file_create|/file_modify|/test_write", "priority": "/high"}
  ],
  "modified_tasks": [
    {"task_id": "existing_id", "new_description": "...", "reason": "..."}
  ],
  "summary": "Brief explanation of changes"
}

JSON only:`, campaign.Title, campaign.CompletedPhases, campaign.TotalPhases, campaign.CompletedTasks, campaign.TotalTasks, requirement)

	resp, err := r.completeWithGrounding(ctx, prompt)
	if err != nil {
		return err
	}

	// 3. Parse and apply changes
	resp = cleanJSONResponse(resp)
	var changes struct {
		NewTasks []struct {
			PhaseOrder  int    `json:"phase_order"`
			Description string `json:"description"`
			Type        string `json:"type"`
			Priority    string `json:"priority"`
		} `json:"new_tasks"`
		ModifiedTasks []struct {
			TaskID         string `json:"task_id"`
			NewDescription string `json:"new_description"`
			Reason         string `json:"reason"`
		} `json:"modified_tasks"`
		Summary string `json:"summary"`
	}

	if err := json.Unmarshal([]byte(resp), &changes); err != nil {
		return fmt.Errorf("failed to parse replan response: %w", err)
	}

	// 4. Add new tasks
	for _, newTask := range changes.NewTasks {
		if newTask.PhaseOrder >= 0 && newTask.PhaseOrder < len(campaign.Phases) {
			phase := &campaign.Phases[newTask.PhaseOrder]

			// Derive a stable slug from campaign ID without assuming prefix length.
			campaignSlug := strings.TrimPrefix(campaign.ID, "/campaign_")
			if campaignSlug == campaign.ID {
				campaignSlug = sanitizeCampaignID(campaign.ID)
			}
			taskID := fmt.Sprintf("/task_%s_%d_%d", campaignSlug, newTask.PhaseOrder, len(phase.Tasks))

			task := Task{
				ID:          taskID,
				PhaseID:     phase.ID,
				Description: newTask.Description,
				Status:      TaskPending,
				Type:        TaskType(newTask.Type),
				Priority:    TaskPriority(newTask.Priority),
			}

			phase.Tasks = append(phase.Tasks, task)
			campaign.TotalTasks++

			// Add to kernel
			r.kernel.LoadFacts(task.ToFacts())
		}
	}

	// 5. Modify existing tasks
	for _, mod := range changes.ModifiedTasks {
		for i := range campaign.Phases {
			for j := range campaign.Phases[i].Tasks {
				if campaign.Phases[i].Tasks[j].ID == mod.TaskID {
					campaign.Phases[i].Tasks[j].Description = mod.NewDescription
					// Update in kernel
					r.kernel.Assert(core.Fact{
						Predicate: "campaign_task",
						Args: []interface{}{
							mod.TaskID,
							campaign.Phases[i].ID,
							mod.NewDescription,
							string(campaign.Phases[i].Tasks[j].Status),
							string(campaign.Phases[i].Tasks[j].Type),
						},
					})
					break
				}
			}
		}
	}

	// 6. Record revision
	campaign.RevisionNumber++
	campaign.LastRevision = changes.Summary

	return nil
}

// RefineNextPhase performs rolling-wave planning: after completing a phase, we
// refresh the next phase based on the latest artifacts and failures.
func (r *Replanner) RefineNextPhase(ctx context.Context, campaign *Campaign, completedPhase *Phase) error {
	if campaign == nil || completedPhase == nil {
		return nil
	}

	// Identify the next phase by order
	var nextPhase *Phase
	for i := range campaign.Phases {
		if campaign.Phases[i].Order == completedPhase.Order+1 {
			nextPhase = &campaign.Phases[i]
			break
		}
	}
	if nextPhase == nil {
		return nil // No further phases to refine
	}

	// Summaries for prompt
	var completedTasksSummary strings.Builder
	for _, t := range completedPhase.Tasks {
		completedTasksSummary.WriteString(fmt.Sprintf("- %s [%s]\n", t.Description, t.Status))
	}

	var upcomingTasks strings.Builder
	for _, t := range nextPhase.Tasks {
		upcomingTasks.WriteString(fmt.Sprintf("- %s (%s)\n", t.Description, t.Type))
	}

	// Get Replanner prompt (JIT or static)
	replannerPrompt, err := r.promptProvider.GetPrompt(ctx, RoleReplanner, campaign.ID)
	if err != nil {
		logging.Get(logging.CategoryCampaign).Warn("Failed to get Replanner prompt, using fallback: %v", err)
		replannerPrompt = ReplannerLogic
	}

	prompt := fmt.Sprintf(`%s

Campaign Goal: %s
Completed Phase: %s (order %d)
Completed Tasks:
%s

Upcoming Phase: %s (order %d)
Current Tasks:
%s

Return JSON only:
{
  "tasks": [
    {"task_id": "existing-id or empty for new", "description": "...", "type": "/file_modify|/file_create|/test_run|/research|/verify|/document|/refactor|/integrate", "priority": "/high|/normal|/low|/critical", "action": "update|add|remove"}
  ],
  "summary": "one-line change summary"
}`, replannerPrompt, campaign.Goal, completedPhase.Name, completedPhase.Order, completedTasksSummary.String(), nextPhase.Name, nextPhase.Order, upcomingTasks.String())

	resp, err := r.completeWithGrounding(ctx, prompt)
	if err != nil {
		return err
	}

	resp = cleanJSONResponse(resp)
	var changes struct {
		Tasks []struct {
			TaskID      string `json:"task_id"`
			Description string `json:"description"`
			Type        string `json:"type"`
			Priority    string `json:"priority"`
			Action      string `json:"action"`
		} `json:"tasks"`
		Summary string `json:"summary"`
	}

	// Try parsing as object first, then fall back to array
	if err := json.Unmarshal([]byte(resp), &changes); err != nil {
		// LLM might have returned just an array instead of {tasks: [], summary: ""}
		var tasksOnly []struct {
			TaskID      string `json:"task_id"`
			Description string `json:"description"`
			Type        string `json:"type"`
			Priority    string `json:"priority"`
			Action      string `json:"action"`
		}
		if arrErr := json.Unmarshal([]byte(resp), &tasksOnly); arrErr != nil {
			logging.Get(logging.CategoryCampaign).Error("RefineNextPhase: failed to parse refinement response as object or array: object_err=%v, array_err=%v, response=%s", err, arrErr, resp[:min(500, len(resp))])
			return fmt.Errorf("failed to parse refinement response: %w (also tried array: %v)", err, arrErr)
		}
		// Successfully parsed as array - convert to expected format
		changes.Tasks = tasksOnly
		changes.Summary = "Rolling-wave refinement (tasks only)"
		logging.CampaignDebug("RefineNextPhase: parsed %d tasks from array response", len(tasksOnly))
	}

	// Apply changes
	for _, t := range changes.Tasks {
		action := strings.ToLower(strings.TrimSpace(t.Action))
		switch action {
		case "remove":
			for i := range nextPhase.Tasks {
				if nextPhase.Tasks[i].ID == t.TaskID {
					nextPhase.Tasks = append(nextPhase.Tasks[:i], nextPhase.Tasks[i+1:]...)
					break
				}
			}
		case "add":
			newID := t.TaskID
			if newID == "" {
				newID = fmt.Sprintf("/task_%s_%d_%d", campaign.ID[10:], nextPhase.Order, len(nextPhase.Tasks))
			}
			task := Task{
				ID:          newID,
				PhaseID:     nextPhase.ID,
				Description: t.Description,
				Status:      TaskPending,
				Type:        TaskType(t.Type),
				Priority:    TaskPriority(defaultPriority(t.Priority)),
			}
			nextPhase.Tasks = append(nextPhase.Tasks, task)
		default: // update
			updated := false
			for i := range nextPhase.Tasks {
				if nextPhase.Tasks[i].ID == t.TaskID {
					if t.Description != "" {
						nextPhase.Tasks[i].Description = t.Description
					}
					if t.Type != "" {
						nextPhase.Tasks[i].Type = TaskType(t.Type)
					}
					if t.Priority != "" {
						nextPhase.Tasks[i].Priority = TaskPriority(defaultPriority(t.Priority))
					}
					updated = true
					break
				}
			}
			if !updated && t.Description != "" {
				newID := t.TaskID
				if newID == "" {
					newID = fmt.Sprintf("/task_%s_%d_%d", campaign.ID[10:], nextPhase.Order, len(nextPhase.Tasks))
				}
				nextPhase.Tasks = append(nextPhase.Tasks, Task{
					ID:          newID,
					PhaseID:     nextPhase.ID,
					Description: t.Description,
					Status:      TaskPending,
					Type:        TaskType(t.Type),
					Priority:    TaskPriority(defaultPriority(t.Priority)),
				})
			}
		}
	}

	// Recompute totals
	var totalTasks, completedTasks int
	for i := range campaign.Phases {
		for _, t := range campaign.Phases[i].Tasks {
			totalTasks++
			if t.Status == TaskCompleted || t.Status == TaskSkipped {
				completedTasks++
			}
		}
	}
	campaign.TotalTasks = totalTasks
	campaign.CompletedTasks = completedTasks
	campaign.RevisionNumber++
	campaign.LastRevision = changes.Summary

	// Refresh facts in kernel
	if err := r.kernel.LoadFacts(campaign.ToFacts()); err != nil {
		return err
	}
	r.kernel.Assert(core.Fact{
		Predicate: "plan_revision",
		Args:      []interface{}{campaign.ID, campaign.RevisionNumber, changes.Summary, time.Now().Unix()},
	})

	return nil
}

func defaultPriority(p string) string {
	if p == "" {
		return string(PriorityNormal)
	}
	return p
}

// getFailedTasks returns all failed tasks in the campaign.
func (r *Replanner) getFailedTasks(campaign *Campaign) []Task {
	var failed []Task
	for _, phase := range campaign.Phases {
		for _, task := range phase.Tasks {
			if task.Status == TaskFailed {
				failed = append(failed, task)
			}
		}
	}
	return failed
}

// getBlockedTasks returns all blocked tasks in the campaign.
func (r *Replanner) getBlockedTasks(campaign *Campaign) []Task {
	var blocked []Task
	for _, phase := range campaign.Phases {
		for _, task := range phase.Tasks {
			if task.Status == TaskBlocked {
				blocked = append(blocked, task)
			}
		}
	}
	return blocked
}

// getReplanTriggers returns replan triggers from the kernel.
func (r *Replanner) getReplanTriggers(campaignID string) []ReplanTrigger {
	facts, err := r.kernel.Query("replan_trigger")
	if err != nil {
		return nil
	}

	var triggers []ReplanTrigger
	for _, fact := range facts {
		if len(fact.Args) >= 3 {
			if fmt.Sprintf("%v", fact.Args[0]) == campaignID {
				ts := int64(0)
				if t, ok := fact.Args[2].(int64); ok {
					ts = t
				}
				triggers = append(triggers, ReplanTrigger{
					CampaignID:  campaignID,
					Reason:      fmt.Sprintf("%v", fact.Args[1]),
					TriggeredAt: time.Unix(ts, 0),
				})
			}
		}
	}
	return triggers
}

// buildReplanContext builds a context string for the LLM.
func (r *Replanner) buildReplanContext(campaign *Campaign, failedTasks, blockedTasks []Task, triggers []ReplanTrigger) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Campaign: %s\n", campaign.Title))
	sb.WriteString(fmt.Sprintf("Status: %s\n", campaign.Status))
	sb.WriteString(fmt.Sprintf("Progress: %d/%d phases, %d/%d tasks\n\n", campaign.CompletedPhases, campaign.TotalPhases, campaign.CompletedTasks, campaign.TotalTasks))

	if len(failedTasks) > 0 {
		sb.WriteString("Failed Tasks:\n")
		for _, task := range failedTasks {
			sb.WriteString(fmt.Sprintf("- [%s] %s\n", task.ID, task.Description))
			if task.LastError != "" {
				sb.WriteString(fmt.Sprintf("  Error: %s\n", task.LastError))
			}
			for _, attempt := range task.Attempts {
				sb.WriteString(fmt.Sprintf("  Attempt %d: %s - %s\n", attempt.Number, attempt.Outcome, attempt.Error))
			}
		}
		sb.WriteString("\n")
	}

	if len(blockedTasks) > 0 {
		sb.WriteString("Blocked Tasks:\n")
		for _, task := range blockedTasks {
			sb.WriteString(fmt.Sprintf("- [%s] %s (depends on: %v)\n", task.ID, task.Description, task.DependsOn))
		}
		sb.WriteString("\n")
	}

	if len(triggers) > 0 {
		sb.WriteString("Replan Triggers:\n")
		for _, trigger := range triggers {
			sb.WriteString(fmt.Sprintf("- %s at %s\n", trigger.Reason, trigger.TriggeredAt.Format(time.RFC3339)))
		}
	}

	return sb.String()
}

// proposeReplans asks LLM to propose fixes for the campaign.
func (r *Replanner) proposeReplans(ctx context.Context, campaign *Campaign, context string, scopeTaskID string) (*ReplanResult, error) {
	// Include campaign metadata for better LLM context
	prompt := fmt.Sprintf(`A campaign needs replanning. Analyze the issues and propose fixes.
Scope: %s

Campaign Metadata:
Campaign: %s (ID: %s)
Goal: %s
Phases: %d, Completed Tasks: %d/%d

Current State:
%s

For each failed/blocked task, determine:
1. Should we retry with modified approach?
2. Should we skip this task?
3. Should we add prerequisite tasks?
4. Should we modify dependencies?

Output JSON:
{
  "success": true,
  "change_summary": "Brief description of changes",
  "retry_tasks": [
    {"task_id": "...", "new_approach": "Modified description/approach"}
  ],
  "skip_tasks": ["task_id1", "task_id2"],
  "add_tasks": [
    {"phase_id": "...", "description": "...", "type": "/file_create", "priority": "/high", "before_task": "task_id"}
  ],
  "modify_dependencies": [
    {"task_id": "...", "remove_deps": ["dep_id"], "add_deps": ["dep_id"]}
  ]
}

	JSON only:`,
		scopeTaskID,
		campaign.Title, campaign.ID, campaign.Goal,

		len(campaign.Phases), campaign.CompletedTasks, campaign.TotalTasks,
		context)

	resp, err := r.completeWithGrounding(ctx, prompt)
	if err != nil {
		return nil, err
	}

	// Parse response
	resp = cleanJSONResponse(resp)
	var parsed struct {
		Success       bool   `json:"success"`
		ChangeSummary string `json:"change_summary"`
		RetryTasks    []struct {
			TaskID      string `json:"task_id"`
			NewApproach string `json:"new_approach"`
		} `json:"retry_tasks"`
		SkipTasks []string `json:"skip_tasks"`
		AddTasks  []struct {
			PhaseID     string `json:"phase_id"`
			Description string `json:"description"`
			Type        string `json:"type"`
			Priority    string `json:"priority"`
			BeforeTask  string `json:"before_task"`
		} `json:"add_tasks"`
		ModifyDependencies []struct {
			TaskID     string   `json:"task_id"`
			RemoveDeps []string `json:"remove_deps"`
			AddDeps    []string `json:"add_deps"`
		} `json:"modify_dependencies"`
	}

	if err := json.Unmarshal([]byte(resp), &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse replan response: %w", err)
	}

	result := &ReplanResult{
		Success:       parsed.Success,
		ChangeSummary: parsed.ChangeSummary,
		RemovedTasks:  parsed.SkipTasks,
	}

	// Build modified tasks
	for _, retry := range parsed.RetryTasks {
		result.ModifiedTasks = append(result.ModifiedTasks, Task{
			ID:          retry.TaskID,
			Description: retry.NewApproach,
			Status:      TaskPending, // Reset to pending
		})
	}

	// Build added tasks
	for _, add := range parsed.AddTasks {
		result.AddedTasks = append(result.AddedTasks, Task{
			PhaseID:     add.PhaseID,
			Description: add.Description,
			Type:        TaskType(add.Type),
			Priority:    TaskPriority(add.Priority),
			Status:      TaskPending,
		})
	}

	return result, nil
}

// applyFixes applies the replan fixes to the campaign.
func (r *Replanner) applyFixes(campaign *Campaign, fixes *ReplanResult) error {
	// 1. Skip tasks
	for _, skipID := range fixes.RemovedTasks {
		for i := range campaign.Phases {
			for j := range campaign.Phases[i].Tasks {
				if campaign.Phases[i].Tasks[j].ID == skipID {
					campaign.Phases[i].Tasks[j].Status = TaskSkipped
					break
				}
			}
		}
	}

	// 2. Modify tasks
	for _, mod := range fixes.ModifiedTasks {
		for i := range campaign.Phases {
			for j := range campaign.Phases[i].Tasks {
				if campaign.Phases[i].Tasks[j].ID == mod.ID {
					campaign.Phases[i].Tasks[j].Description = mod.Description
					campaign.Phases[i].Tasks[j].Status = TaskPending
					campaign.Phases[i].Tasks[j].Attempts = nil // Reset attempts
					campaign.Phases[i].Tasks[j].LastError = ""
					break
				}
			}
		}
	}

	// 3. Add tasks
	for _, add := range fixes.AddedTasks {
		for i := range campaign.Phases {
			if campaign.Phases[i].ID == add.PhaseID {
				taskID := fmt.Sprintf("/task_%s_%d_%d", campaign.ID[10:], i, len(campaign.Phases[i].Tasks))
				newTask := add
				newTask.ID = taskID
				campaign.Phases[i].Tasks = append(campaign.Phases[i].Tasks, newTask)
				campaign.TotalTasks++
				break
			}
		}
	}

	return nil
}

// ClearReplanTriggers clears all replan triggers for a campaign.
func (r *Replanner) ClearReplanTriggers(campaignID string) error {
	return r.kernel.RetractFact(core.Fact{
		Predicate: "replan_trigger",
		Args:      []interface{}{campaignID},
	})
}
