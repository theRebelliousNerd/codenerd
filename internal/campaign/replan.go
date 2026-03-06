package campaign

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/tools/research"
	"codenerd/internal/types"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Replanner handles campaign replanning when things go wrong.
// It uses LLM + Mangle collaboration to adapt the plan.
type Replanner struct {
	mu             sync.Mutex
	kernel         core.Kernel
	llmClient      perception.LLMClient
	promptProvider PromptProvider // Optional JIT prompt provider

	// Gemini advanced features (nil if not Gemini or features unavailable)
	grounding *research.GroundingHelper // Google Search / URL Context grounding
	thinking  *research.ThinkingHelper  // Thinking mode metadata capture
}

const (
	maxReplanContextTasks    = 25
	maxReplanAttemptsPerTask = 3
	maxReplanContextText     = 400
	maxReplanContextChars    = 16000
)

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
	if provider == nil {
		r.promptProvider = NewStaticPromptProvider()
		return
	}

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
		return "", fmt.Errorf("%w: replanner requires llm client", ErrNilDependency)
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

func truncateForPrompt(text string, maxLen int) string {
	text = strings.TrimSpace(text)
	if maxLen <= 0 || len(text) <= maxLen {
		return text
	}
	suffix := "... [truncated]"
	if maxLen <= len(suffix) {
		return text[:maxLen]
	}
	return text[:maxLen-len(suffix)] + suffix
}

func appendReplanContextLine(sb *strings.Builder, line string) bool {
	if line == "" {
		return true
	}

	if sb.Len() >= maxReplanContextChars {
		return false
	}

	remaining := maxReplanContextChars - sb.Len()
	if len(line) > remaining {
		if remaining <= 0 {
			return false
		}
		line = truncateForPrompt(line, remaining)
	}
	sb.WriteString(line)
	return sb.Len() < maxReplanContextChars
}

// Replan adapts the campaign plan based on current state and failures.
// failedTaskID is optional; if provided, replanning is scoped to that task's subtree.
func (r *Replanner) Replan(ctx context.Context, campaign *Campaign, failedTaskID string) error {
	if campaign == nil {
		return ErrNilCampaign
	}
	if r.kernel == nil {
		return ErrNilKernel
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	// 1. Gather context about what went wrong
	failedTasks := r.getFailedTasks(campaign)
	blockedTasks := r.getBlockedTasks(campaign)
	replanTriggers := r.getReplanTriggers(campaign.ID)

	if len(failedTasks) == 0 && len(blockedTasks) == 0 && len(replanTriggers) == 0 && failedTaskID == "" {
		return nil // Nothing to replan
	}
	if r.llmClient == nil {
		return fmt.Errorf("%w: replanner requires llm client", ErrNilDependency)
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
	workingCampaign, err := cloneCampaign(campaign)
	if err != nil {
		return err
	}
	if err := r.applyFixes(workingCampaign, fixes); err != nil {
		return fmt.Errorf("failed to apply fixes: %w", err)
	}

	// 5. Record revision
	workingCampaign.RevisionNumber++
	workingCampaign.LastRevision = strings.TrimSpace(fixes.ChangeSummary)
	if workingCampaign.LastRevision == "" {
		workingCampaign.LastRevision = "Applied replan updates"
	}

	// 6. Reload campaign into kernel
	if err := syncCampaignFacts(r.kernel, campaign, workingCampaign, workingCampaign.LastRevision); err != nil {
		return fmt.Errorf("failed to reload campaign: %w", err)
	}

	*campaign = *workingCampaign

	return nil
}

// ReplanForNewRequirement adds tasks for a new requirement.
func (r *Replanner) ReplanForNewRequirement(ctx context.Context, campaign *Campaign, requirement string) error {
	if campaign == nil {
		return ErrNilCampaign
	}
	requirement = strings.TrimSpace(requirement)
	if requirement == "" {
		return ErrEmptyRequirement
	}
	if r.kernel == nil {
		return ErrNilKernel
	}
	if r.llmClient == nil {
		return fmt.Errorf("%w: replanner requires llm client", ErrNilDependency)
	}
	r.mu.Lock()
	defer r.mu.Unlock()

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

	workingCampaign, err := cloneCampaign(campaign)
	if err != nil {
		return err
	}

	// 4. Add new tasks
	for _, newTask := range changes.NewTasks {
		if newTask.PhaseOrder >= 0 && newTask.PhaseOrder < len(workingCampaign.Phases) {
			phase := &workingCampaign.Phases[newTask.PhaseOrder]
			taskID := fmt.Sprintf("/task_%s_%d_%d", campaignSlug(workingCampaign.ID), newTask.PhaseOrder, len(phase.Tasks))

			task := Task{
				ID:          taskID,
				PhaseID:     phase.ID,
				Description: newTask.Description,
				Status:      TaskPending,
				Type:        normalizeTaskType(newTask.Type, defaultTaskTypeForCategory(phase.Category)),
				Priority:    normalizeTaskPriority(newTask.Priority),
				Order:       len(phase.Tasks),
			}
			if strings.TrimSpace(task.Description) == "" {
				continue
			}

			phase.Tasks = append(phase.Tasks, task)
			workingCampaign.TotalTasks++
		}
	}

	// 5. Modify existing tasks
	for _, mod := range changes.ModifiedTasks {
		for i := range workingCampaign.Phases {
			for j := range workingCampaign.Phases[i].Tasks {
				if workingCampaign.Phases[i].Tasks[j].ID == mod.TaskID {
					if desc := strings.TrimSpace(mod.NewDescription); desc != "" {
						workingCampaign.Phases[i].Tasks[j].Description = desc
					}
					break
				}
			}
		}
	}

	// 6. Record revision
	workingCampaign.RevisionNumber++
	workingCampaign.LastRevision = strings.TrimSpace(changes.Summary)
	if workingCampaign.LastRevision == "" {
		workingCampaign.LastRevision = "Applied new requirement updates"
	}

	if err := syncCampaignFacts(r.kernel, campaign, workingCampaign, workingCampaign.LastRevision); err != nil {
		return fmt.Errorf("failed to reload campaign after new requirement: %w", err)
	}
	if err := r.kernel.Assert(core.Fact{
		Predicate: "replan_trigger",
		Args:      []interface{}{workingCampaign.ID, "/new_requirement", time.Now().Unix()},
	}); err != nil {
		logging.Get(logging.CategoryCampaign).Warn("ReplanForNewRequirement: failed to assert replan_trigger: %v", err)
	}

	*campaign = *workingCampaign

	return nil
}

// RefineNextPhase performs rolling-wave planning: after completing a phase, we
// refresh the next phase based on the latest artifacts and failures.
func (r *Replanner) RefineNextPhase(ctx context.Context, campaign *Campaign, completedPhase *Phase) error {
	if campaign == nil || completedPhase == nil {
		return nil
	}
	if r.kernel == nil {
		return ErrNilKernel
	}
	if r.llmClient == nil {
		return fmt.Errorf("%w: replanner requires llm client", ErrNilDependency)
	}
	r.mu.Lock()
	defer r.mu.Unlock()

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

	workingCampaign, err := cloneCampaign(campaign)
	if err != nil {
		return err
	}
	var workingNextPhase *Phase
	for i := range workingCampaign.Phases {
		if workingCampaign.Phases[i].Order == completedPhase.Order+1 {
			workingNextPhase = &workingCampaign.Phases[i]
			break
		}
	}
	if workingNextPhase == nil {
		return nil
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
			for i := range workingNextPhase.Tasks {
				if workingNextPhase.Tasks[i].ID == t.TaskID {
					workingNextPhase.Tasks = append(workingNextPhase.Tasks[:i], workingNextPhase.Tasks[i+1:]...)
					break
				}
			}
		case "add":
			newID := t.TaskID
			if newID == "" {
				newID = fmt.Sprintf("/task_%s_%d_%d", campaignSlug(campaign.ID), workingNextPhase.Order, len(workingNextPhase.Tasks))
			}
			task := Task{
				ID:          newID,
				PhaseID:     workingNextPhase.ID,
				Description: t.Description,
				Status:      TaskPending,
				Type:        normalizeTaskType(t.Type, defaultTaskTypeForCategory(workingNextPhase.Category)),
				Priority:    normalizeTaskPriority(t.Priority),
				Order:       len(workingNextPhase.Tasks),
			}
			workingNextPhase.Tasks = append(workingNextPhase.Tasks, task)
		default: // update
			updated := false
			for i := range workingNextPhase.Tasks {
				if workingNextPhase.Tasks[i].ID == t.TaskID {
					if t.Description != "" {
						workingNextPhase.Tasks[i].Description = t.Description
					}
					if t.Type != "" {
						workingNextPhase.Tasks[i].Type = normalizeTaskType(t.Type, workingNextPhase.Tasks[i].Type)
					}
					if t.Priority != "" {
						workingNextPhase.Tasks[i].Priority = normalizeTaskPriority(t.Priority)
					}
					updated = true
					break
				}
			}
			if !updated && t.Description != "" {
				newID := t.TaskID
				if newID == "" {
					newID = fmt.Sprintf("/task_%s_%d_%d", campaignSlug(campaign.ID), workingNextPhase.Order, len(workingNextPhase.Tasks))
				}
				workingNextPhase.Tasks = append(workingNextPhase.Tasks, Task{
					ID:          newID,
					PhaseID:     workingNextPhase.ID,
					Description: t.Description,
					Status:      TaskPending,
					Type:        normalizeTaskType(t.Type, defaultTaskTypeForCategory(workingNextPhase.Category)),
					Priority:    normalizeTaskPriority(t.Priority),
					Order:       len(workingNextPhase.Tasks),
				})
			}
		}
	}

	// Recompute totals
	var totalTasks, completedTasks int
	for i := range workingCampaign.Phases {
		for _, t := range workingCampaign.Phases[i].Tasks {
			totalTasks++
			if t.Status == TaskCompleted || t.Status == TaskSkipped {
				completedTasks++
			}
		}
	}
	workingCampaign.TotalTasks = totalTasks
	workingCampaign.CompletedTasks = completedTasks
	workingCampaign.RevisionNumber++
	workingCampaign.LastRevision = strings.TrimSpace(changes.Summary)
	if workingCampaign.LastRevision == "" {
		workingCampaign.LastRevision = "Rolling-wave refinement"
	}

	// Refresh facts in kernel
	if err := syncCampaignFacts(r.kernel, campaign, workingCampaign, workingCampaign.LastRevision); err != nil {
		return err
	}

	*campaign = *workingCampaign

	return nil
}

func defaultPriority(p string) string {
	return string(normalizeTaskPriority(p))
}

// getFailedTasks returns all failed tasks in the campaign.
func (r *Replanner) getFailedTasks(campaign *Campaign) []Task {
	if campaign == nil {
		return nil
	}
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
	if campaign == nil {
		return nil
	}
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
	if r.kernel == nil {
		return nil
	}
	facts, err := r.kernel.Query("replan_trigger")
	if err != nil {
		return nil
	}

	var triggers []ReplanTrigger
	for _, fact := range facts {
		if len(fact.Args) >= 3 {
			if types.ExtractString(fact.Args[0]) == campaignID {
				ts := int64(0)
				if t, ok := fact.Args[2].(int64); ok {
					ts = t
				}
				triggers = append(triggers, ReplanTrigger{
					CampaignID:  campaignID,
					Reason:      types.ExtractString(fact.Args[1]),
					TriggeredAt: time.Unix(ts, 0),
				})
			}
		}
	}
	return triggers
}

// buildReplanContext builds a context string for the LLM.
func (r *Replanner) buildReplanContext(campaign *Campaign, failedTasks, blockedTasks []Task, triggers []ReplanTrigger) string {
	if campaign == nil {
		return ""
	}
	var sb strings.Builder

	appendReplanContextLine(&sb, fmt.Sprintf("Campaign: %s\n", truncateForPrompt(campaign.Title, 200)))
	appendReplanContextLine(&sb, fmt.Sprintf("Status: %s\n", campaign.Status))
	appendReplanContextLine(&sb, fmt.Sprintf("Progress: %d/%d phases, %d/%d tasks\n\n", campaign.CompletedPhases, campaign.TotalPhases, campaign.CompletedTasks, campaign.TotalTasks))

	if len(failedTasks) > 0 {
		if !appendReplanContextLine(&sb, "Failed Tasks:\n") {
			return sb.String()
		}
		for idx, task := range failedTasks {
			if idx >= maxReplanContextTasks {
				appendReplanContextLine(&sb, fmt.Sprintf("... %d additional failed tasks omitted\n", len(failedTasks)-idx))
				break
			}
			if !appendReplanContextLine(&sb, fmt.Sprintf("- [%s] %s\n", task.ID, truncateForPrompt(task.Description, 240))) {
				return sb.String()
			}
			if task.LastError != "" {
				if !appendReplanContextLine(&sb, fmt.Sprintf("  Error: %q\n", truncateForPrompt(task.LastError, maxReplanContextText))) {
					return sb.String()
				}
			}
			start := 0
			if len(task.Attempts) > maxReplanAttemptsPerTask {
				start = len(task.Attempts) - maxReplanAttemptsPerTask
			}
			for _, attempt := range task.Attempts[start:] {
				if !appendReplanContextLine(&sb, fmt.Sprintf("  Attempt %d: %s - %q\n", attempt.Number, attempt.Outcome, truncateForPrompt(attempt.Error, maxReplanContextText))) {
					return sb.String()
				}
			}
		}
		if !appendReplanContextLine(&sb, "\n") {
			return sb.String()
		}
	}

	if len(blockedTasks) > 0 {
		if !appendReplanContextLine(&sb, "Blocked Tasks:\n") {
			return sb.String()
		}
		for idx, task := range blockedTasks {
			if idx >= maxReplanContextTasks {
				appendReplanContextLine(&sb, fmt.Sprintf("... %d additional blocked tasks omitted\n", len(blockedTasks)-idx))
				break
			}
			if !appendReplanContextLine(&sb, fmt.Sprintf("- [%s] %s (depends on: %v)\n", task.ID, truncateForPrompt(task.Description, 240), task.DependsOn)) {
				return sb.String()
			}
		}
		if !appendReplanContextLine(&sb, "\n") {
			return sb.String()
		}
	}

	if len(triggers) > 0 {
		if !appendReplanContextLine(&sb, "Replan Triggers:\n") {
			return sb.String()
		}
		for idx, trigger := range triggers {
			if idx >= maxReplanContextTasks {
				appendReplanContextLine(&sb, fmt.Sprintf("... %d additional triggers omitted\n", len(triggers)-idx))
				break
			}
			if !appendReplanContextLine(&sb, fmt.Sprintf("- %s at %s\n", trigger.Reason, trigger.TriggeredAt.Format(time.RFC3339))) {
				return sb.String()
			}
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
			Type:        normalizeTaskType(add.Type, TaskTypeFileModify),
			Priority:    normalizeTaskPriority(add.Priority),
			Status:      TaskPending,
		})
	}

	return result, nil
}

// applyFixes applies the replan fixes to the campaign.
func (r *Replanner) applyFixes(campaign *Campaign, fixes *ReplanResult) error {
	if campaign == nil {
		return ErrNilCampaign
	}
	if fixes == nil {
		return nil
	}

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
				taskID := fmt.Sprintf("/task_%s_%d_%d", campaignSlug(campaign.ID), i, len(campaign.Phases[i].Tasks))
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
	if r.kernel == nil {
		return ErrNilKernel
	}
	return r.kernel.RetractFact(core.Fact{
		Predicate: "replan_trigger",
		Args:      []interface{}{campaignID},
	})
}
