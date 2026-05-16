package autopoiesis

import (
	"context"
	"fmt"
	"time"
)

// =============================================================================
// ANALYSIS AND ACTION EXECUTION
// =============================================================================

// Analyze performs complete autopoiesis analysis on user input
func (o *Orchestrator) Analyze(ctx context.Context, input string, target string) (*AnalysisResult, error) {
	result := &AnalysisResult{
		AnalyzedAt: time.Now(),
		InputHash:  hashString(input),
		Actions:    []AutopoiesisAction{},
	}

	// 1. Analyze complexity
	if o.config.EnableLLM {
		complexity, err := o.complexity.AnalyzeWithLLM(ctx, input)
		if err != nil {
			// Fall back to heuristic
			complexity = o.complexity.Analyze(ctx, input, target)
		}
		result.Complexity = complexity
	} else {
		result.Complexity = o.complexity.Analyze(ctx, input, target)
	}

	result.NeedsCampaign = result.Complexity.NeedsCampaign
	result.SuggestedPhases = result.Complexity.SuggestedPhases

	// Add campaign action if needed
	if result.NeedsCampaign {
		result.Actions = append(result.Actions, AutopoiesisAction{
			Type:        ActionStartCampaign,
			Priority:    result.Complexity.Score,
			Description: fmt.Sprintf("Start campaign with %d phases", len(result.Complexity.SuggestedPhases)),
			Payload: CampaignPayload{
				Phases:         result.Complexity.SuggestedPhases,
				EstimatedFiles: result.Complexity.EstimatedFiles,
				Reasons:        result.Complexity.Reasons,
			},
		})
	}

	// 2. Analyze persistence needs
	if o.config.EnableLLM {
		persistence, err := o.persistence.AnalyzeWithLLM(ctx, input)
		if err != nil {
			persistence = o.persistence.Analyze(ctx, input)
		}
		result.Persistence = persistence
	} else {
		result.Persistence = o.persistence.Analyze(ctx, input)
	}

	result.NeedsPersistent = result.Persistence.NeedsPersistent

	// Create agent specs for persistence needs
	for _, need := range result.Persistence.Needs {
		if need.Confidence >= o.config.MinConfidence {
			spec, err := o.agentCreate.CreateFromNeed(ctx, need)
			if err != nil {
				continue
			}
			result.SuggestedAgents = append(result.SuggestedAgents, *spec)

			result.Actions = append(result.Actions, AutopoiesisAction{
				Type:        ActionCreateAgent,
				Priority:    need.Confidence,
				Description: fmt.Sprintf("Create persistent %s agent", need.AgentType),
				Payload:     spec,
			})
		}
	}

	// 3. Tool need detection (only if task seems to need new capability)
	if o.config.EnableToolGeneration && shouldCheckToolNeed(input) {
		toolNeed, err := o.toolGen.DetectToolNeed(ctx, input, "")
		if err == nil && toolNeed != nil && o.shouldGenerateToolNeed(toolNeed) {
			result.ToolNeeds = append(result.ToolNeeds, *toolNeed)

			result.Actions = append(result.Actions, AutopoiesisAction{
				Type:        ActionGenerateTool,
				Priority:    toolNeed.Priority,
				Description: fmt.Sprintf("Generate tool: %s", toolNeed.Name),
				Payload:     toolNeed,
			})
		}
	}

	// Sort actions by priority
	sortActionsByPriority(result.Actions)

	return result, nil
}

// ExecuteAction executes a single autopoiesis action
func (o *Orchestrator) ExecuteAction(ctx context.Context, action AutopoiesisAction) error {
	switch action.Type {
	case ActionGenerateTool:
		return o.executeToolGeneration(ctx, action)
	case ActionCreateAgent:
		return o.executeAgentCreation(ctx, action)
	case ActionStartCampaign:
		// Campaign starting is handled by the campaign orchestrator
		return nil
	default:
		return fmt.Errorf("unknown action type: %v", action.Type)
	}
}

// executeToolGeneration generates and registers a new tool
func (o *Orchestrator) executeToolGeneration(ctx context.Context, action AutopoiesisAction) error {
	need, ok := action.Payload.(*ToolNeed)
	if !ok {
		return fmt.Errorf("invalid payload for tool generation")
	}

	// Generate the tool
	tool, err := o.toolGen.GenerateTool(ctx, need)
	if err != nil {
		return fmt.Errorf("failed to generate tool: %w", err)
	}

	// Write to disk
	if err := o.toolGen.WriteTool(tool); err != nil {
		return fmt.Errorf("failed to write tool: %w", err)
	}

	// Register in memory
	if err := o.toolGen.RegisterTool(tool); err != nil {
		return fmt.Errorf("failed to register tool: %w", err)
	}

	// Update throttling counters on success.
	o.mu.Lock()
	o.toolsGenerated++
	o.lastToolGen = time.Now()
	o.mu.Unlock()

	return nil
}

// =============================================================================
// QUICK ANALYSIS (for real-time use in processInput)
// =============================================================================

// QuickAnalyze performs fast analysis without LLM calls
func (o *Orchestrator) QuickAnalyze(ctx context.Context, input string, target string) QuickResult {
	result := QuickResult{}

	// Quick complexity check (heuristic only)
	complexity := o.complexity.Analyze(ctx, input, target)
	result.ComplexityLevel = complexity.Level
	result.NeedsCampaign = complexity.NeedsCampaign

	// Enhance with code element awareness from kernel
	elementCount := o.QueryCodeElementCount()
	filesInScope := o.QueryFilesInScope()

	// If many elements are in scope, the task might be more complex
	if elementCount > 20 && result.ComplexityLevel < ComplexityComplex {
		result.ComplexityLevel = ComplexityComplex
		result.NeedsCampaign = true
	}

	// If many files in scope, consider complexity
	if filesInScope > 5 && result.ComplexityLevel < ComplexityModerate {
		result.ComplexityLevel = ComplexityModerate
	}

	// Quick persistence check (heuristic only)
	persistence := o.persistence.Analyze(ctx, input)
	result.NeedsPersistent = persistence.NeedsPersistent

	// Determine top action
	if result.NeedsCampaign {
		result.TopAction = &AutopoiesisAction{
			Type:        ActionStartCampaign,
			Priority:    complexity.Score,
			Description: "Complex task - recommend campaign",
		}
	} else if result.NeedsPersistent && len(persistence.Needs) > 0 {
		result.TopAction = &AutopoiesisAction{
			Type:        ActionCreateAgent,
			Priority:    persistence.Needs[0].Confidence,
			Description: "Persistent agent recommended",
		}
	}

	return result
}

// ShouldTriggerCampaign is a quick check for campaign needs
func (o *Orchestrator) ShouldTriggerCampaign(ctx context.Context, input string, target string) (bool, string) {
	complexity := o.complexity.Analyze(ctx, input, target)

	if !complexity.NeedsCampaign {
		return false, ""
	}

	// Build reason string
	reason := fmt.Sprintf("Complexity: %s (score: %.2f). ", complexityLevelString(complexity.Level), complexity.Score)
	if len(complexity.SuggestedPhases) > 0 {
		reason += fmt.Sprintf("Suggested phases: %v. ", complexity.SuggestedPhases)
	}
	if len(complexity.Reasons) > 0 {
		reason += fmt.Sprintf("Reasons: %v", complexity.Reasons)
	}

	return true, reason
}

// ShouldCreatePersistentAgent is a quick check for persistence needs
func (o *Orchestrator) ShouldCreatePersistentAgent(ctx context.Context, input string) (bool, *PersistenceNeed) {
	persistence := o.persistence.Analyze(ctx, input)

	if !persistence.NeedsPersistent || len(persistence.Needs) == 0 {
		return false, nil
	}

	// Return highest confidence need
	var best *PersistenceNeed
	for i := range persistence.Needs {
		if best == nil || persistence.Needs[i].Confidence > best.Confidence {
			best = &persistence.Needs[i]
		}
	}

	return true, best
}
