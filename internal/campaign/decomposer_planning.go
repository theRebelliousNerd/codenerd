package campaign

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/store"
	"codenerd/internal/types"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (d *Decomposer) completePlanWithSchemaOrFallback(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if d.llmClient == nil {
		return "", fmt.Errorf("%w: decomposer requires llm client", ErrNilDependency)
	}

	// This reply is unmarshalled straight into RawPlan, so it must not be
	// constrained to the Piggyback envelope. Clients otherwise decide by
	// searching the prompt for "control_packet" — which the planner prompt
	// contains precisely because it FORBIDS the envelope. The schema won that
	// argument every time: the model returned a fully-formed envelope with empty
	// fields, RawPlan unmarshalled it successfully with zero phases, and the
	// campaign silently ran a generic placeholder. Declare the contract instead
	// of letting a substring search infer it.
	ctx = types.WithStructuredOutputOnly(ctx)

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

// parseRawPlanResponse extracts a plan from the model's reply.
//
// The phases check is the important part. encoding/json ignores unknown fields,
// so unmarshalling a Piggyback envelope into RawPlan SUCCEEDS: every field is
// unknown, nothing is populated, err is nil, and the caller receives a
// structurally valid plan with no phases in it. Three live campaigns died that
// way — the parse reported success, the caller substituted a generic scaffold,
// and the CLI printed a confidence figure for a plan the model never produced.
//
// A plan with no phases is not a plan, so it is now treated as a parse miss and
// the recovery paths run.
func parseRawPlanResponse(resp string) (*RawPlan, error) {
	clean := cleanJSONResponse(resp)

	var plan RawPlan
	firstErr := json.Unmarshal([]byte(clean), &plan)
	if firstErr == nil && len(plan.Phases) > 0 {
		return &plan, nil
	}

	// Providers that wrap the payload under "plan".
	var wrapped struct {
		Plan RawPlan `json:"plan"`
	}
	if err := json.Unmarshal([]byte(clean), &wrapped); err == nil && len(wrapped.Plan.Phases) > 0 {
		return &wrapped.Plan, nil
	}

	// A model that has been told about the Piggyback envelope elsewhere in its
	// context will sometimes wrap the plan in one. The plan is right there;
	// refusing to read it costs an entire campaign. Recovery, not license — the
	// prompt still asks for the bare schema.
	var envelope struct {
		ControlPacket struct {
			Plan RawPlan `json:"plan"`
		} `json:"control_packet"`
		Plan RawPlan `json:"plan"`
	}
	if err := json.Unmarshal([]byte(clean), &envelope); err == nil {
		if len(envelope.ControlPacket.Plan.Phases) > 0 {
			return &envelope.ControlPacket.Plan, nil
		}
		if len(envelope.Plan.Phases) > 0 {
			return &envelope.Plan, nil
		}
	}

	if firstErr != nil {
		return nil, firstErr
	}
	if strings.Contains(clean, "control_packet") {
		return nil, fmt.Errorf("%w: the model returned a Piggyback control_packet instead of a plan. "+
			"Check that the planner compile is structured-output-only (prompt.IsStructuredOutputOnly)",
			errPlanHasNoPhases)
	}
	return nil, errPlanHasNoPhases
}

// errPlanHasNoPhases marks a response that was valid JSON but carried no phases.
//
// It is deliberately distinct from a JSON syntax error. Unparseable output means
// the model is broken and the command should fail; a well-formed answer with an
// empty phases array is the model saying "I have nothing", which degrades to the
// scaffold so `nerd campaign start` still produces something the user can see
// and reject. Collapsing the two turned an existing degrade path into a hard
// failure.
var errPlanHasNoPhases = errors.New("response parsed as JSON but contained no phases")

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
		// The example must show phases POPULATED. This retry prompt used to
		// print `"phases": []` as the "required structure", so a model that had
		// just failed to produce phases was handed an example with none in it
		// and obligingly returned none again.
		retryPrompt := fmt.Sprintf(`Your previous response could not be used as a plan. Output ONLY a JSON object.

Do NOT wrap it in "control_packet". Do NOT include "tool_requests" or
"surface_response". "phases" must contain at least one real phase.

Required structure:
{
  "title": "REPLACE_WITH_ACTUAL_CAMPAIGN_TITLE",
  "confidence": 0.9,
  "phases": [
    {
      "name": "REPLACE_WITH_PHASE_NAME",
      "order": 0,
      "category": "/scaffold",
      "description": "REPLACE_WITH_WHAT_THIS_PHASE_ACCOMPLISHES",
      "tasks": [
        {"description": "REPLACE_WITH_A_SPECIFIC_ACTIONABLE_TASK", "type": "/file_create", "order": 0}
      ]
    }
  ]
}

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

			// A well-formed answer with no phases degrades to the scaffold,
			// which is now loud (WARN + Degraded + CLI banner) so the user can
			// see it and decide. Genuinely unparseable output is a broken model
			// and fails the command.
			if errors.Is(retryErr, errPlanHasNoPhases) {
				logging.Get(logging.CategoryCampaign).Warn(
					"Planner returned no phases twice (%v); falling through to the degraded scaffold", retryErr)
				return d.normalizeRawPlanFromLLM(&RawPlan{}, req)
			}
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
		// Scoped per phase: see the duplicate-suppression note below.
		seenTaskKeys := make(map[string]bool, len(rawPhase.Tasks))
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
			// Use globalTaskIDMap for cross-phase references. Deduplicate
			// explicit mappings while preserving stable order so an explicit
			// index repeated by the model does not produce duplicate IDs, and
			// so later research inheritance can check the same set.
			seenCtx := make(map[string]struct{}, len(rawTask.ContextFrom))
			for _, ctxIdx := range rawTask.ContextFrom {
				if ctxTaskID, ok := globalTaskIDMap[ctxIdx]; ok {
					if _, exists := seenCtx[ctxTaskID]; exists {
						continue
					}
					task.ContextFrom = append(task.ContextFrom, ctxTaskID)
					seenCtx[ctxTaskID] = struct{}{}
					logging.CampaignDebug("Task %s will receive context from task %s (global index %d)", taskID, ctxTaskID, ctxIdx)
				} else {
					logging.Get(logging.CategoryCampaign).Warn("Task %s references unknown context source index %d", taskID, ctxIdx)
				}
			}

			// Deterministic research handoff: if the model omitted context_from
			// but the task directly depends on a prior research task, inherit it.
			// orchestrator_task_results.go only injects ContextFrom, so without
			// this the coder never sees discovery artifacts (campaign 46015b77
			// invented a 480-line isolated subsystem). Preserve explicit
			// mappings, deduplicate while preserving stable order, and never
			// allow current/self/forward references or prose inference.
			for _, depIdx := range rawTask.DependsOn {
				if depIdx < 0 || depIdx >= j {
					continue
				}
				if depIdx >= len(rawPhase.Tasks) {
					continue
				}
				depRaw := rawPhase.Tasks[depIdx]
				depType := normalizeTaskType(depRaw.Type, defaultTaskTypeForCategory(phaseCategory))
				if depType != TaskTypeResearch {
					continue
				}
				if depTaskID, ok := taskIDMap[depIdx]; ok {
					if _, exists := seenCtx[depTaskID]; exists {
						continue
					}
					task.ContextFrom = append(task.ContextFrom, depTaskID)
					seenCtx[depTaskID] = struct{}{}
					logging.CampaignDebug("Inherited research context for task %s from dependency %s (research) via depends_on %d", taskID, depTaskID, depIdx)
					logging.Campaign("Task %s inherits research context from %s", taskID, depTaskID)
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

			// Reconcile a mistyped file task with its write set so a planning
			// error cannot block a whole campaign at run time, and retype a
			// pathless mutating task to /research.
			//
			// Observed live 2026-09-04 on campaign a36d036c: the decomposer
			// emitted "Add turn_cost/6 Decl to schemas" as /file_modify with a
			// write set containing only a path that did not exist. The coder
			// correctly created the file, and validateFileModifyOutcome
			// correctly refused it ("modified no pre-existing file in its
			// declared write set"); the diagnostic-repro retries re-ran the
			// identical mistyped task until the phase blocked with
			// /all_tasks_blocked. Check the type against the filesystem here,
			// at plan time, when the fix is a retype instead of a failure.
			//
			// Pathless file-task defense (campaign 855d7bcf, 2026-09-04): a
			// mutating file task with no artifact path, no write-set entry, and
			// no extractable description path has nowhere to write. Dispatched
			// as a file task it either hollow-succeeds (empty target stats the
			// workspace directory) or fails forever in the fallback until
			// /all_tasks_blocked. Retype it to /research so it runs as
			// analytical work with a durable artifact instead of blocking the
			// phase. Recorded the same way as reconcileTaskTypeWithWriteSet.
			//
			// Shared with the rolling-wave replanner via
			// applyTaskTypeDefenses so plan-time and re-plan-time behavior
			// stay identical.
			oldTaskType := task.Type
			if changed, reason := applyTaskTypeDefenses(d.workspace, &task); changed {
				logging.CampaignWarn("Retyped task %s from %s to %s: %s",
					taskID, oldTaskType, task.Type, reason)
			}

			// Drop tasks the planner emitted twice within one phase.
			//
			// Observed live 2026-08-08 on campaign fc6472c2: phase 1 came back
			// with four tasks that were two tasks, each duplicated —
			// "Modify internal/session/gate_names.go to add a doc comment"
			// twice and "Run go test ./internal/session" twice. The IDs were
			// distinct, so nothing downstream saw a duplicate; the orchestrator
			// simply ran the same work again, and the two test tasks were
			// in_progress concurrently.
			//
			// This is planner variance rather than a structural bug, which is
			// exactly why it is worth handling here: an LLM will occasionally
			// repeat itself, and every repetition is a full task's worth of
			// model calls and wall-clock spent to reach a state already reached.
			// Comparing normalized descriptions is deterministic and cheap.
			//
			// Deliberately scoped to within a phase. The same description in a
			// LATER phase can be legitimate — verify, change, verify again — and
			// suppressing that would silently delete real work.
			if key := normalizedTaskKey(task.Description); key != "" {
				if seenTaskKeys[key] {
					logging.CampaignWarn(
						"Planner emitted a duplicate task in phase %s; dropping it: %.80q",
						phase.ID, task.Description)
					continue
				}
				seenTaskKeys[key] = true
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

// normalizedTaskKey reduces a task description to a comparison key for
// duplicate suppression: lowercased, punctuation-insensitive, whitespace
// collapsed.
//
// Conservative on purpose. It only catches a planner repeating itself in
// substantially the same words; two genuinely different tasks will not collide,
// and the cost of a miss is the duplicate work we have today rather than a
// deleted task.
func normalizedTaskKey(desc string) string {
	var b strings.Builder
	lastSpace := true
	for _, r := range strings.ToLower(strings.TrimSpace(desc)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '/', r == '.', r == '_', r == '-':
			b.WriteRune(r)
			lastSpace = false
		default:
			if !lastSpace {
				b.WriteRune(' ')
				lastSpace = true
			}
		}
	}
	key := strings.TrimSpace(b.String())

	// Path characters are kept above so ./internal/session stays distinct from
	// ./internal/core, but that also preserves sentence-ending punctuation and
	// turns "..." into a key. Trim them at the edges, then require the key to
	// contain something substantive — a description with no letters or digits
	// cannot be compared, and the caller must let it through rather than
	// suppress a task it failed to parse.
	key = strings.Trim(key, "./_- ")
	if !strings.ContainsFunc(key, func(r rune) bool {
		return (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
	}) {
		return ""
	}
	return key
}

// applyTaskTypeDefenses performs the plan-time task-type defenses shared by
// the decomposer (buildCampaign) and the rolling-wave replanner
// (RefineNextPhase): first the write-set reconciliation, then the pathless
// file-task retype. It mutates task.Type in place and reports whether a
// retype happened and why.
//
// At most one retype fires per call in practice: the reconciliation only
// fires on a non-empty write set, while the pathless retype requires an
// empty one, so a single changed/reason pair describes the outcome. Pure:
// no logging here — the caller logs the retype.
func applyTaskTypeDefenses(workspace string, task *Task) (changed bool, reason string) {
	if task == nil {
		return false, ""
	}
	if c, r := reconcileTaskTypeWithWriteSet(workspace, task); c {
		changed, reason = true, r
	}
	// Pathless file-task defense (campaign 855d7bcf, 2026-09-04): a
	// mutating file task with no artifact path, no write-set entry, and
	// no extractable description path has nowhere to write. Dispatched
	// as a file task it either hollow-succeeds (empty target stats the
	// workspace directory) or fails forever in the fallback until
	// /all_tasks_blocked. Retype it to /research so it runs as
	// analytical work with a durable artifact instead of blocking the
	// phase. Recorded the same way as reconcileTaskTypeWithWriteSet.
	if isMutatingTaskType(task.Type) {
		hasArtifactPath := false
		for _, a := range task.Artifacts {
			if strings.TrimSpace(a.Path) != "" {
				hasArtifactPath = true
				break
			}
		}
		hasWriteSetPath := false
		for _, p := range task.WriteSet {
			if strings.TrimSpace(p) != "" {
				hasWriteSetPath = true
				break
			}
		}
		if !hasArtifactPath && !hasWriteSetPath && extractPathFromDescription(task.Description) == "" {
			task.Type = TaskTypeResearch
			return true, "no artifact, write set, or extractable path"
		}
	}
	return changed, reason
}

// reconcileTaskTypeWithWriteSet corrects a mistyped file task against the
// reconcileTaskTypeWithWriteSet corrects a mistyped file task against the
// filesystem at plan time. It mutates task.Type in place and reports whether
// a retype happened and why.
//
// A /file_modify whose write set contains no existing file is invalid by the
// orchestrator's own transaction rules (validateFileModifyOutcome refuses it
// with "modified no pre-existing file in its declared write set"), so retype
// it to /file_create. Symmetrically, a /file_create whose write set already
// exists on disk is really a modification, so retype it to /file_modify.
//
// Only exact paths participate: if any write-set entry contains glob
// metacharacters (see containsGlobMeta), or the write set is empty, the task
// is left unchanged. Relative paths resolve against workspace; absolute paths
// are stated directly. Pure: no logging here — the caller logs the retype.
func reconcileTaskTypeWithWriteSet(workspace string, task *Task) (changed bool, reason string) {
	if task == nil {
		return false, ""
	}
	if len(task.WriteSet) == 0 {
		return false, ""
	}
	for _, p := range task.WriteSet {
		if containsGlobMeta(p) {
			return false, ""
		}
	}
	pathExists := func(p string) bool {
		q := p
		if !filepath.IsAbs(q) && workspace != "" {
			q = filepath.Join(workspace, q)
		}
		_, err := os.Stat(q)
		return err == nil
	}
	switch task.Type {
	case TaskTypeFileModify:
		for _, p := range task.WriteSet {
			if pathExists(p) {
				return false, ""
			}
		}
		task.Type = TaskTypeFileCreate
		return true, "no write-set path exists"
	case TaskTypeFileCreate:
		for _, p := range task.WriteSet {
			if !pathExists(p) {
				return false, ""
			}
		}
		task.Type = TaskTypeFileModify
		return true, "every write-set path already exists"
	default:
		return false, ""
	}
}