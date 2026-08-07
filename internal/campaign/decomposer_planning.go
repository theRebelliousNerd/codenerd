package campaign

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/store"
	"codenerd/internal/types"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func (d *Decomposer) completePlanWithSchemaOrFallback(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if d.llmClient == nil {
		return "", fmt.Errorf("%w: decomposer requires llm client", ErrNilDependency)
	}
	if schemaClient, ok := core.AsSchemaCapable(d.llmClient); ok {
		resp, err := schemaClient.CompleteWithSchema(ctx, systemPrompt, userPrompt, planResponseSchema)
		if err == nil {
			return resp, nil
		}
		logging.CampaignDebug("Schema-constrained plan call failed, falling back to non-schema call: %v", err)
	}

	if systemClient, ok := d.llmClient.(interface {
		CompleteWithSystem(ctx context.Context, system, user string) (string, error)
	}); ok {
		return systemClient.CompleteWithSystem(ctx, systemPrompt, userPrompt)
	}
	return d.llmClient.Complete(ctx, systemPrompt+"\n\n"+userPrompt)
}

func parseRawPlanResponse(resp string) (*RawPlan, error) {
	clean := cleanJSONResponse(resp)

	var plan RawPlan
	if err := json.Unmarshal([]byte(clean), &plan); err == nil {
		return &plan, nil
	} else {
		// Compatibility fallback for providers that wrap payload under "plan".
		var wrapped struct {
			Plan RawPlan `json:"plan"`
		}
		if wrapErr := json.Unmarshal([]byte(clean), &wrapped); wrapErr == nil &&
			(strings.TrimSpace(wrapped.Plan.Title) != "" || len(wrapped.Plan.Phases) > 0) {
			return &wrapped.Plan, nil
		}
		return nil, err
	}
}

// llmProposePlan asks LLM to propose a plan structure using retrieved context.
func (d *Decomposer) buildPlanProposalContext(ctx context.Context, campaignID string, req DecomposeRequest, kbPath string, files []FileMetadata, requirements []Requirement) string {
	var contextBuilder strings.Builder

	// Add goal
	contextBuilder.WriteString(fmt.Sprintf("GOAL: %s\n\n", req.Goal))

	// Add campaign type context
	contextBuilder.WriteString(fmt.Sprintf("CAMPAIGN TYPE: %s\n\n", req.CampaignType))

	// Add user hints
	if len(req.UserHints) > 0 {
		contextBuilder.WriteString("USER HINTS:\n")
		for _, hint := range req.UserHints {
			contextBuilder.WriteString(fmt.Sprintf("- %s\n", hint))
		}
		contextBuilder.WriteString("\n")
	}

	// Add requirements summary
	if len(requirements) > 0 {
		contextBuilder.WriteString("EXTRACTED REQUIREMENTS:\n")
		for _, r := range requirements {
			contextBuilder.WriteString(fmt.Sprintf("- [%s] %s (Priority: %s)\n", r.ID, r.Description, r.Priority))
		}
		contextBuilder.WriteString("\n")
	}

	// Add strict build taxonomy guidance
	contextBuilder.WriteString(TaxonomyLogic)
	contextBuilder.WriteString("\n\n")

	// Add source metadata
	if len(files) > 0 {
		contextBuilder.WriteString("SOURCE DOCUMENTS (metadata):\n")
		for _, f := range files {
			contextBuilder.WriteString(fmt.Sprintf("- %s (%s, %d bytes, modified %s)\n", f.Path, f.Type, f.SizeBytes, f.ModifiedAt.Format(time.RFC3339)))
		}
		contextBuilder.WriteString("\n")
	}

	// Add topology hints derived from document layers
	if topo := d.topologyContextSummary(); topo != "" {
		contextBuilder.WriteString("TOPOLOGY HINTS:\n")
		contextBuilder.WriteString(topo)
		contextBuilder.WriteString("\n\n")
	}

	// Add intelligence report context (from Step 0)
	if d.lastIntelligence != nil {
		intelContext := d.formatIntelligenceContext(d.lastIntelligence)
		if intelContext != "" {
			contextBuilder.WriteString(intelContext)
			contextBuilder.WriteString("\n")
			logging.Campaign("Intelligence context injected into LLM prompt")
		}
	}

	// Add available shards for shard-aware planning
	if d.shardLister != nil {
		shards := d.shardLister.ListAvailableShards()
		if shardList := formatShardList(shards); shardList != "" {
			contextBuilder.WriteString(shardList)
			contextBuilder.WriteString("\n")
			logging.Campaign("Shard-aware planning: injected %d available shards into prompt", len(shards))
		}
	} else {
		logging.Campaign("Shard-aware planning: shardLister is nil, skipping shard injection")
	}

	// Retrieve goal-focused snippets for context
	if kbPath != "" {
		if ls, err := store.NewLocalStore(kbPath); err == nil {
			defer ls.Close()
			entries, _ := ls.VectorRecallSemanticFiltered(ctx, req.Goal, 6, "campaign_id", campaignID)
			if len(entries) == 0 {
				entries, _ = ls.VectorRecallSemantic(ctx, req.Goal, 6)
			}
			if len(entries) > 0 {
				contextBuilder.WriteString("RETRIEVED SNIPPETS:\n")
				for idx, e := range entries {
					path := ""
					if p, ok := e.Metadata["path"].(string); ok {
						path = p
					}
					contextBuilder.WriteString(fmt.Sprintf("--- Snippet %d (%s) ---\n%s\n", idx+1, path, e.Content))
				}
				contextBuilder.WriteString("\n")
			}
		}
	}

	return contextBuilder.String()
}

func (d *Decomposer) executePlanProposalWithRetry(ctx context.Context, req DecomposeRequest, contextPreview string, userPrompt string, systemPrompt string) (*RawPlan, error) {
	logging.CampaignDebug("Sending plan proposal request to LLM (prompt length=%d)", len(userPrompt))

	resp, llmErr := d.completePlanWithSchemaOrFallback(ctx, systemPrompt, userPrompt)
	if llmErr != nil {
		logging.Get(logging.CategoryCampaign).Error("LLM plan proposal failed: %v", llmErr)
		return nil, llmErr
	}
	logging.CampaignDebug("LLM response received (length=%d)", len(resp))

	// Parse response with retry on failure
	plan, parseErr := parseRawPlanResponse(resp)
	if parseErr != nil {
		// Log the raw response for debugging
		rawPreview := resp
		if len(rawPreview) > 500 {
			rawPreview = rawPreview[:500]
		}
		logging.CampaignDebug("Raw response (first 500 chars): %s", rawPreview)
		logging.Get(logging.CategoryCampaign).Error("Failed to parse plan JSON: %v", parseErr)

		// Retry with stronger enforcement
		logging.Campaign("Retrying plan proposal with JSON enforcement")
		if len(contextPreview) > 2000 {
			contextPreview = contextPreview[:2000]
		}
		retryPrompt := fmt.Sprintf(`The previous response was not valid JSON. Output ONLY a JSON object.

Required structure:
{"title": "REPLACE_WITH_ACTUAL_CAMPAIGN_TITLE", "confidence": 0.9, "phases": []}

Goal: %s
Context: %s

Output ONLY the JSON:`, req.Goal, contextPreview)

		retrySystemPrompt := "You output ONLY valid JSON matching the campaign plan schema."
		resp, llmErr = d.completePlanWithSchemaOrFallback(ctx, retrySystemPrompt, retryPrompt)
		if llmErr != nil {
			return nil, fmt.Errorf("retry failed: %w", llmErr)
		}
		plan, retryErr := parseRawPlanResponse(resp)
		if retryErr != nil {
			retryPreview := resp
			if len(retryPreview) > 500 {
				retryPreview = retryPreview[:500]
			}
			logging.CampaignDebug("Retry raw response (first 500 chars): %s", retryPreview)
			return nil, fmt.Errorf("failed to parse plan JSON after retry: %w", retryErr)
		}
		return d.normalizeRawPlanFromLLM(plan, req)
	}

	return d.normalizeRawPlanFromLLM(plan, req)
}

func (d *Decomposer) llmProposePlan(ctx context.Context, campaignID string, req DecomposeRequest, kbPath string, files []FileMetadata, requirements []Requirement) (*RawPlan, error) {
	timer := logging.StartTimer(logging.CategoryCampaign, "llmProposePlan")
	defer timer.Stop()

	logging.Campaign("Requesting LLM plan proposal")
	logging.CampaignDebug("Context: files=%d, requirements=%d, hints=%d",
		len(files), len(requirements), len(req.UserHints))

	contextStr := d.buildPlanProposalContext(ctx, campaignID, req, kbPath, files, requirements)

	// Get Planner prompt (JIT or static)
	plannerPrompt, err := d.promptProvider.GetPrompt(ctx, RolePlanner, campaignID)
	if err != nil {
		logging.CampaignDebug("Failed to get Planner prompt, using fallback: %v", err)
		plannerPrompt = PlannerLogic
	}

	// System prompt with JSON enforcement.
	systemPrompt := `You are a Campaign Planner. Output only a valid JSON object representing the campaign plan.

CRITICAL: Your response MUST be valid JSON matching this schema:
{
  "title": "Campaign Title",
  "confidence": 0.9,
  "phases": [{"name": "Phase 1", "order": 0, "category": "/scaffold", "description": "...", "tasks": [...]}]
}

Do NOT use markdown. Do NOT include text outside the JSON object.`

	userPrompt := fmt.Sprintf(`%s

%s

Output the JSON plan now:`, plannerPrompt, contextStr)

	return d.executePlanProposalWithRetry(ctx, req, contextStr, userPrompt, systemPrompt)
}

func (d *Decomposer) normalizeRawPlanFromLLM(plan *RawPlan, req DecomposeRequest) (*RawPlan, error) {
	if plan == nil {
		return nil, fmt.Errorf("nil plan")
	}
	// Validate and fix plan title if it's a placeholder or empty
	if plan.Title == "" || plan.Title == "string" || plan.Title == "REPLACE_WITH_ACTUAL_CAMPAIGN_TITLE" {
		// Extract title from goal: use first sentence or first 60 chars
		title := req.Goal
		if idx := strings.Index(title, "."); idx > 0 && idx < 80 {
			title = title[:idx]
		} else if idx := strings.Index(title, ":"); idx > 0 && idx < 60 {
			title = title[:idx]
		} else if len(title) > 60 {
			title = title[:60]
		}
		plan.Title = strings.TrimSpace(title)
		logging.Campaign("Fixed placeholder title to: %s", plan.Title)
	}

	// If no phases were generated, create a fallback scaffolding phase.
	//
	// This is a DEGRADED plan, not a plan. It is three generic tasks with the
	// goal string pasted into each description, and it cannot express anything
	// the goal actually asked for. It was previously logged at Campaign level
	// and then reported to the user as "Campaign Plan ... Confidence: 50%" —
	// indistinguishable from a real plan.
	//
	// Live consequence: a goal naming six specific documents to write reached
	// here because the planner prompt carried both the plan schema and the
	// Piggyback envelope protocol (see prompt.IsStructuredOutputOnly), so the
	// model returned a control_packet and this parser saw no phases. The
	// campaign then executed the scaffold, produced two files nobody asked for,
	// and reported phases completed. Warn loudly enough that the next silent
	// decomposition failure is visible in the log by itself.
	if len(plan.Phases) == 0 {
		logging.Get(logging.CategoryCampaign).Warn(
			"DECOMPOSITION FAILED: the model returned no phases, so this campaign is running a generic "+
				"three-task scaffold that cannot satisfy the goal. Check the planner response in _llm_io.log "+
				"for an output-contract mismatch. Goal: %.120s", req.Goal)
		plan.Degraded = true
		goalPreview := req.Goal
		if len(goalPreview) > 100 {
			goalPreview = goalPreview[:100] + "..."
		}
		plan.Phases = []RawPhase{
			{
				Name:        "Research & Planning",
				Order:       0,
				Category:    "/research",
				Description: "Analyze requirements and existing codebase",
				Tasks: []RawTask{
					{Description: "Understand the full scope of: " + goalPreview, Type: "/research", Order: 0},
				},
			},
			{
				Name:        "Implementation",
				Order:       1,
				Category:    "/scaffold",
				Description: "Implement the core functionality",
				Tasks: []RawTask{
					{Description: "Build the main components for: " + goalPreview, Type: "/code", Order: 0},
				},
			},
			{
				Name:        "Testing & Review",
				Order:       2,
				Category:    "/test",
				Description: "Validate the implementation",
				Tasks: []RawTask{
					{Description: "Ensure quality for: " + goalPreview, Type: "/test", Order: 0},
				},
			},
		}
		plan.Confidence = 0.5 // Lower confidence for fallback plan
	}

	logging.Campaign("Plan proposed: %s (confidence=%.2f, phases=%d)", plan.Title, plan.Confidence, len(plan.Phases))
	for i, phase := range plan.Phases {
		logging.CampaignDebug("  Phase %d: %s (category=%s, tasks=%d)",
			i, phase.Name, phase.Category, len(phase.Tasks))
	}

	return plan, nil
}

// buildCampaign converts a RawPlan to a Campaign.
func (d *Decomposer) buildCampaign(campaignID string, req DecomposeRequest, plan *RawPlan) *Campaign {
	logging.CampaignDebug("Building campaign structure from raw plan")
	logging.CampaignDebug("Raw plan: title=%s, confidence=%.2f, phases=%d", plan.Title, plan.Confidence, len(plan.Phases))

	now := time.Now()

	campaign := &Campaign{
		ID:              campaignID,
		Type:            req.CampaignType,
		Title:           plan.Title,
		Goal:            req.Goal,
		SourceMaterial:  req.SourcePaths,
		Status:          StatusValidating,
		CreatedAt:       now,
		UpdatedAt:       now,
		Confidence:      plan.Confidence,
		PlanDegraded:    plan.Degraded,
		ContextBudget:   req.ContextBudget,
		Phases:          make([]Phase, 0),
		ContextProfiles: make([]ContextProfile, 0),
	}

	// Build phases
	slug := campaignSlug(campaignID)
	phaseIDMap := make(map[int]string)      // Map phase order -> phaseID
	globalTaskIDMap := make(map[int]string) // Map global task index -> taskID (for cross-phase context_from)
	globalTaskIndex := 0                    // Running counter for global task indices
	for i, rawPhase := range plan.Phases {
		phaseID := fmt.Sprintf("/phase_%s_%d", slug, i)
		phaseIDMap[i] = phaseID
		phaseOrder := rawPhase.Order
		if phaseOrder == 0 {
			phaseOrder = i
		}
		logging.CampaignDebug("Building phase %d: %s (category=%s, tasks=%d, deps=%v)",
			i, rawPhase.Name, rawPhase.Category, len(rawPhase.Tasks), rawPhase.DependsOn)

		// Create context profile
		profileID := fmt.Sprintf("/profile_%s_%d", slug, i)
		contextProfile := ContextProfile{
			ID:              profileID,
			RequiredSchemas: []string{"file_topology", "symbol_graph", "diagnostic"},
			RequiredTools:   rawPhase.RequiredTools,
			FocusPatterns:   rawPhase.FocusPatterns,
		}

		campaign.ContextProfiles = append(campaign.ContextProfiles, contextProfile)

		phaseCategory := normalizeCategory(rawPhase.Category)

		phase := Phase{
			ID:             phaseID,
			CampaignID:     campaignID,
			Name:           rawPhase.Name,
			Order:          phaseOrder,
			Category:       phaseCategory,
			Status:         PhasePending,
			ContextProfile: profileID,
			Objectives: []PhaseObjective{{
				Type:               normalizeObjectiveType(rawPhase.ObjectiveType, defaultObjectiveTypeForCategory(phaseCategory)),
				Description:        rawPhase.Description,
				VerificationMethod: normalizeVerificationMethod(rawPhase.VerificationMethod),
			}},
			EstimatedTasks:      len(rawPhase.Tasks),
			EstimatedComplexity: normalizeComplexity(rawPhase.Complexity),
			Tasks:               make([]Task, 0),
		}

		// Build dependencies
		for _, depIdx := range rawPhase.DependsOn {
			if depPhaseID, ok := phaseIDMap[depIdx]; ok {
				phase.Dependencies = append(phase.Dependencies, PhaseDependency{
					DependsOnPhaseID: depPhaseID,
					Type:             DepHard,
				})
				logging.CampaignDebug("Phase %s depends on %s (hard dependency)", phaseID, depPhaseID)
			} else {
				logging.Get(logging.CategoryCampaign).Warn("Phase %s references unknown dependency index %d", phaseID, depIdx)
			}
		}

		// Build tasks
		taskIDMap := make(map[int]string) // Phase-local map for depends_on
		logging.CampaignDebug("Building %d tasks for phase %s", len(rawPhase.Tasks), phaseID)
		for j, rawTask := range rawPhase.Tasks {
			taskID := fmt.Sprintf("/task_%s_%d_%d", slug, i, j)
			taskIDMap[j] = taskID
			globalTaskIDMap[globalTaskIndex] = taskID // Track global index for cross-phase context_from
			globalTaskIndex++
			orderIndex := j
			if rawTask.Order > 0 {
				orderIndex = rawTask.Order
			}
			logging.CampaignDebug("Task %d: type=%s, priority=%s, artifacts=%d, deps=%v",
				j, rawTask.Type, rawTask.Priority, len(rawTask.Artifacts), rawTask.DependsOn)

			task := Task{
				ID:          taskID,
				PhaseID:     phaseID,
				Description: rawTask.Description,
				Status:      TaskPending,
				Type:        normalizeTaskType(rawTask.Type, defaultTaskTypeForCategory(phaseCategory)),
				Priority:    normalizeTaskPriority(rawTask.Priority),
				Order:       orderIndex,
				DependsOn:   make([]string, 0),
				Artifacts:   make([]TaskArtifact, 0),
				WriteSet:    normalizeWriteSetPaths(d.workspace, rawTask.WriteSet),
				// Shard routing fields (explicit shard selection)
				Shard:       rawTask.Shard,
				ShardInput:  rawTask.ShardInput,
				ContextFrom: make([]string, 0),
			}
			// Task dependencies
			for _, depIdx := range rawTask.DependsOn {
				if depTaskID, ok := taskIDMap[depIdx]; ok {
					task.DependsOn = append(task.DependsOn, depTaskID)
					logging.CampaignDebug("Task %s depends on task %s", taskID, depTaskID)
				} else {
					logging.Get(logging.CategoryCampaign).Warn("Task %s references unknown dependency index %d", taskID, depIdx)
				}
			}

			// Context injection references (for shard-aware planning)
			// Use globalTaskIDMap for cross-phase references
			for _, ctxIdx := range rawTask.ContextFrom {
				if ctxTaskID, ok := globalTaskIDMap[ctxIdx]; ok {
					task.ContextFrom = append(task.ContextFrom, ctxTaskID)
					logging.CampaignDebug("Task %s will receive context from task %s (global index %d)", taskID, ctxTaskID, ctxIdx)
				} else {
					logging.Get(logging.CategoryCampaign).Warn("Task %s references unknown context source index %d", taskID, ctxIdx)
				}
			}

			// Log explicit shard routing if present
			if task.Shard != "" {
				logging.CampaignDebug("Task %s has explicit shard routing: %s", taskID, task.Shard)
			}

			// Artifacts
			for _, artifactPath := range rawTask.Artifacts {
				artifactType := "/source_file"
				if strings.Contains(artifactPath, "_test") || strings.Contains(artifactPath, "test_") {
					artifactType = "/test_file"
				}
				normalizedPath := sanitizeTaskArtifactPath(d.workspace, artifactPath)
				if normalizedPath == "" {
					logging.Get(logging.CategoryCampaign).Warn("Dropping unsafe artifact path %q for task %s", artifactPath, taskID)
					continue
				}
				task.Artifacts = append(task.Artifacts, TaskArtifact{
					Type: artifactType,
					Path: normalizedPath,
				})
			}

			// Mutating tasks default to artifact-backed write sets if explicit write_set was omitted.
			if len(task.WriteSet) == 0 && isMutatingTaskType(task.Type) {
				inferredWriteSet := make([]string, 0, len(task.Artifacts)+1)
				for _, a := range task.Artifacts {
					if a.Path != "" {
						inferredWriteSet = append(inferredWriteSet, a.Path)
					}
				}
				if len(inferredWriteSet) == 0 {
					if inferred := extractPathFromDescription(task.Description); inferred != "" {
						inferredWriteSet = append(inferredWriteSet, inferred)
					}
				}
				task.WriteSet = normalizeWriteSetPaths(d.workspace, inferredWriteSet)
			}

			phase.Tasks = append(phase.Tasks, task)
			campaign.TotalTasks++
		}

		campaign.Phases = append(campaign.Phases, phase)
		campaign.TotalPhases++
		logging.CampaignDebug("Added phase %s with %d tasks", phase.ID, len(phase.Tasks))
	}

	logging.Campaign("Campaign structure built: phases=%d, totalTasks=%d", campaign.TotalPhases, campaign.TotalTasks)
	return campaign
}

// validatePlan uses Mangle to validate the plan.
func (d *Decomposer) validatePlan(campaignID string) []PlanValidationIssue {
	logging.CampaignDebug("Validating plan via Mangle kernel")

	issues := make([]PlanValidationIssue, 0)

	// Let Mangle drive validation via validation_error facts
	facts, err := d.kernel.Query("validation_error")
	if err != nil {
		logging.CampaignDebug("No validation errors queried (or query failed): %v", err)
	} else {
		for _, fact := range facts {
			if len(fact.Args) >= 3 {
				phaseID := types.ExtractString(fact.Args[0])
				issueType := types.ExtractString(fact.Args[1])
				desc := types.ExtractString(fact.Args[2])
				issues = append(issues, PlanValidationIssue{
					CampaignID:  campaignID,
					IssueType:   issueType,
					Description: fmt.Sprintf("%s: %s", phaseID, desc),
				})
				logging.CampaignDebug("Validation issue: [%s] %s", issueType, desc)
			}
		}
	}

	logging.CampaignDebug("Validation complete: %d issues found", len(issues))
	return issues
}

// refinePlan asks LLM to refine the plan based on validation issues.
func (d *Decomposer) refinePlan(ctx context.Context, plan *RawPlan, issues []PlanValidationIssue) (*RawPlan, error) {
	if len(issues) == 0 {
		return plan, nil
	}

	logging.Campaign("Refining plan to fix %d validation issues", len(issues))
	timer := logging.StartTimer(logging.CategoryCampaign, "refinePlan")
	defer timer.Stop()

	// Build issues summary
	var issuesSummary strings.Builder
	for _, issue := range issues {
		issuesSummary.WriteString(fmt.Sprintf("- [%s] %s\n", issue.IssueType, issue.Description))
	}

	// Serialize current plan
	planJSON, _ := json.MarshalIndent(plan, "", "  ")

	prompt := fmt.Sprintf(`The following plan has validation issues that need to be fixed:

CURRENT PLAN:
%s

ISSUES:
%s

Please fix these issues and output the corrected plan as JSON.
- For circular dependencies: adjust phase order or dependencies
- For unreachable tasks: add missing task definitions or fix dependency references

Output ONLY valid JSON with the same structure as the input:`, string(planJSON), issuesSummary.String())

	logging.CampaignDebug("Sending refinement request to LLM")
	// Use grounding for plan refinement (may benefit from up-to-date best practices)
	resp, err := d.completeWithGrounding(ctx, prompt)
	if err != nil {
		logging.Get(logging.CategoryCampaign).Error("LLM refinement failed: %v", err)
		return nil, err
	}

	// Parse response
	resp = cleanJSONResponse(resp)
	var refinedPlan RawPlan
	if err := json.Unmarshal([]byte(resp), &refinedPlan); err != nil {
		logging.Get(logging.CategoryCampaign).Error("Failed to parse refined plan: %v", err)
		return nil, fmt.Errorf("failed to parse refined plan: %w", err)
	}

	logging.Campaign("Plan refined successfully: %s (phases=%d)", refinedPlan.Title, len(refinedPlan.Phases))
	return &refinedPlan, nil
}
