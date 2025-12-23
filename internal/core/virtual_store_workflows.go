package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/tactile"
)

// =============================================================================
// TDD LOOP ACTION HANDLERS
// =============================================================================

// handleReadErrorLog reads test/build error logs from the last execution.
func (v *VirtualStore) handleReadErrorLog(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	logging.VirtualStoreDebug("handleReadErrorLog: target=%s", req.Target)

	// Target specifies the log type (test, build, or file path)
	logType := req.Target
	if logType == "" {
		logType = "test"
	}

	// Try to read from common log locations
	var logContent string
	var logPath string

	switch logType {
	case "test":
		logPath = filepath.Join(v.workingDir, ".nerd", "logs", "test.log")
	case "build":
		logPath = filepath.Join(v.workingDir, ".nerd", "logs", "build.log")
	default:
		logPath = v.resolvePath(logType)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		// Return empty result if log not found, not an error
		return ActionResult{
			Success: true,
			Output:  "",
			FactsToAdd: []Fact{
				{Predicate: "error_log_empty", Args: []interface{}{logType}},
			},
		}, nil
	}
	logContent = string(data)

	return ActionResult{
		Success: true,
		Output:  logContent,
		Metadata: map[string]interface{}{
			"log_type": logType,
			"log_path": logPath,
			"size":     len(logContent),
		},
		FactsToAdd: []Fact{
			{Predicate: "error_log_read", Args: []interface{}{logType, len(logContent)}},
			// Fix 15.4: Assert test_state transition
			{Predicate: "test_state", Args: []interface{}{"/log_read"}},
		},
	}, nil
}

// handleAnalyzeRootCause signals the kernel to analyze root cause of a failure.
func (v *VirtualStore) handleAnalyzeRootCause(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	logging.VirtualStoreDebug("handleAnalyzeRootCause: target=%s", req.Target)

	// This is a signal action - the actual analysis is done by the LLM
	// We inject facts to indicate the analysis should proceed
	errorContext := req.Target
	if errorContext == "" {
		errorContext = "unknown"
	}

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Root cause analysis requested for: %s", errorContext),
		FactsToAdd: []Fact{
			{Predicate: "analyzing_root_cause", Args: []interface{}{errorContext}},
			{Predicate: "tdd_phase", Args: []interface{}{"/analyze"}},
		},
	}, nil
}

// handleGeneratePatch signals the kernel that a patch should be generated.
func (v *VirtualStore) handleGeneratePatch(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	logging.VirtualStoreDebug("handleGeneratePatch: target=%s", req.Target)

	// Signal action for patch generation
	targetFile := req.Target
	patchDesc, _ := req.Payload["description"].(string)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Patch generation requested for: %s", targetFile),
		FactsToAdd: []Fact{
			{Predicate: "generating_patch", Args: []interface{}{targetFile, patchDesc}},
			{Predicate: "tdd_phase", Args: []interface{}{"/patch"}},
		},
	}, nil
}

// handleEscalateToUser escalates an issue to the user for intervention.
func (v *VirtualStore) handleEscalateToUser(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	reason := req.Target
	logging.VirtualStore("Escalating to user: %s", reason)

	return ActionResult{
		Success: false, // Escalation means current task cannot proceed
		Output:  fmt.Sprintf("ESCALATION REQUIRED: %s", reason),
		Error:   "USER_INTERVENTION_REQUIRED",
		FactsToAdd: []Fact{
			{Predicate: "escalated_to_user", Args: []interface{}{reason}},
			{Predicate: "task_blocked", Args: []interface{}{reason}},
		},
	}, nil
}

// handleComplete marks the current task as complete.
func (v *VirtualStore) handleComplete(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	taskID := req.Target
	summary, _ := req.Payload["summary"].(string)

	logging.VirtualStore("Task completed: %s", taskID)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Task %s completed: %s", taskID, summary),
		FactsToAdd: []Fact{
			{Predicate: "task_completed", Args: []interface{}{taskID, summary}},
			{Predicate: "completion_signal", Args: []interface{}{taskID}},
		},
	}, nil
}

// handleInterrogative enters interrogative mode for clarification.
func (v *VirtualStore) handleInterrogative(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	question := req.Target
	options, _ := req.Payload["options"].([]interface{})

	logging.VirtualStoreDebug("Entering interrogative mode: %s", question)

	return ActionResult{
		Success: false, // Needs user response
		Output:  question,
		Error:   "CLARIFICATION_NEEDED",
		Metadata: map[string]interface{}{
			"question": question,
			"options":  options,
			"mode":     "interrogative",
		},
		FactsToAdd: []Fact{
			{Predicate: "awaiting_clarification", Args: []interface{}{question}},
			{Predicate: "interrogative_mode", Args: []interface{}{true}},
		},
	}, nil
}

// handleResumeTask resumes a previously paused task.
func (v *VirtualStore) handleResumeTask(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	taskID := req.Target
	logging.VirtualStore("Resuming task: %s", taskID)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Resuming task: %s", taskID),
		FactsToAdd: []Fact{
			{Predicate: "task_resumed", Args: []interface{}{taskID}},
			{Predicate: "active_task", Args: []interface{}{taskID}},
		},
	}, nil
}

// handleRefreshShardContext marks stale shard context atoms as refreshed.
// This prevents policy from repeatedly emitting /refresh_shard_context for the same stale atoms.
func (v *VirtualStore) handleRefreshShardContext(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	v.mu.RLock()
	k := v.kernel
	v.mu.RUnlock()

	if k == nil {
		return ActionResult{Success: false, Error: "kernel not attached"}, nil
	}

	stale, err := k.Query("context_stale")
	if err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	shardFilter := strings.TrimSpace(req.Target)
	now := time.Now().Unix()

	facts := make([]Fact, 0, len(stale))
	refreshed := 0
	for _, f := range stale {
		if len(f.Args) < 2 {
			continue
		}
		shardID := fmt.Sprintf("%v", f.Args[0])
		atom := fmt.Sprintf("%v", f.Args[1])
		if shardFilter != "" && shardID != shardFilter {
			continue
		}
		facts = append(facts, Fact{
			Predicate: "shard_context_refreshed",
			Args:      []interface{}{shardID, atom, now},
		})
		refreshed++
	}

	out := fmt.Sprintf("Refreshed %d stale shard context atoms", refreshed)
	if shardFilter != "" {
		out = fmt.Sprintf("Refreshed %d stale shard context atoms for %s", refreshed, shardFilter)
	}
	if refreshed == 0 {
		out = "No stale shard context atoms detected"
		if shardFilter != "" {
			out = fmt.Sprintf("No stale shard context atoms detected for %s", shardFilter)
		}
	}

	return ActionResult{
		Success:    true,
		Output:     out,
		FactsToAdd: facts,
	}, nil
}

// =============================================================================
// OUROBOROS ACTION HANDLERS
// =============================================================================

// handleGenerateTool generates a new tool via the Ouroboros pipeline.
func (v *VirtualStore) handleGenerateTool(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	v.mu.RLock()
	generator := v.toolGenerator
	v.mu.RUnlock()

	if generator == nil {
		return ActionResult{
			Success: false,
			Error:   "tool generator not configured",
			FactsToAdd: []Fact{
				{Predicate: "tool_generation_failed", Args: []interface{}{req.Target, "no_generator"}},
			},
		}, nil
	}

	toolName := req.Target
	purpose, _ := req.Payload["purpose"].(string)
	code, _ := req.Payload["code"].(string)
	confidence, _ := req.Payload["confidence"].(float64)
	priority, _ := req.Payload["priority"].(float64)
	isDiagnostic, _ := req.Payload["is_diagnostic"].(bool)

	if confidence == 0 {
		confidence = 0.8
	}
	if priority == 0 {
		priority = 5.0
	}

	logging.VirtualStore("Generating tool: %s (purpose=%s)", toolName, purpose)

	success, registeredName, binaryPath, errMsg := generator.GenerateToolFromCode(
		ctx, toolName, purpose, code, confidence, priority, isDiagnostic,
	)

	if !success {
		return ActionResult{
			Success: false,
			Error:   errMsg,
			FactsToAdd: []Fact{
				{Predicate: "tool_generation_failed", Args: []interface{}{toolName, errMsg}},
			},
		}, nil
	}

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Tool %s generated at %s", registeredName, binaryPath),
		Metadata: map[string]interface{}{
			"tool_name":   registeredName,
			"binary_path": binaryPath,
		},
		FactsToAdd: []Fact{
			{Predicate: "tool_generated", Args: []interface{}{registeredName, binaryPath}},
			{Predicate: "tool_available", Args: []interface{}{registeredName}},
		},
	}, nil
}

// handleOuroborosDetect detects tool needs based on task context.
func (v *VirtualStore) handleOuroborosDetect(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	logging.VirtualStoreDebug("Ouroboros detect: %s", req.Target)

	// Detection is a signal action - the LLM identifies tool needs
	taskContext := req.Target

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Tool detection initiated for: %s", taskContext),
		FactsToAdd: []Fact{
			{Predicate: "ouroboros_phase", Args: []interface{}{"/detect"}},
			{Predicate: "tool_detection_context", Args: []interface{}{taskContext}},
		},
	}, nil
}

// handleOuroborosGenerate generates tool code.
func (v *VirtualStore) handleOuroborosGenerate(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	toolName := req.Target
	logging.VirtualStoreDebug("Ouroboros generate: %s", toolName)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Tool code generation initiated for: %s", toolName),
		FactsToAdd: []Fact{
			{Predicate: "ouroboros_phase", Args: []interface{}{"/generate"}},
			{Predicate: "tool_generating", Args: []interface{}{toolName}},
		},
	}, nil
}

// handleOuroborosCompile compiles a generated tool.
func (v *VirtualStore) handleOuroborosCompile(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	toolName := req.Target
	sourcePath, _ := req.Payload["source_path"].(string)

	logging.VirtualStore("Ouroboros compile: %s from %s", toolName, sourcePath)

	// Compile the tool
	if sourcePath == "" {
		sourcePath = filepath.Join(v.workingDir, ".nerd", "tools", toolName+".go")
	}

	outputPath := filepath.Join(v.workingDir, ".nerd", "tools", ".compiled", toolName)

	cmd := tactile.ShellCommand{
		Binary:           "go",
		Arguments:        []string{"build", "-o", outputPath, sourcePath},
		WorkingDirectory: v.workingDir,
		TimeoutSeconds:   60,
		EnvironmentVars:  v.getAllowedEnv(),
	}

	output, err := v.executor.Execute(ctx, cmd)
	if err != nil {
		return ActionResult{
			Success: false,
			Output:  output,
			Error:   err.Error(),
			FactsToAdd: []Fact{
				{Predicate: "ouroboros_compile_failed", Args: []interface{}{toolName, err.Error()}},
			},
		}, nil
	}

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Tool %s compiled to %s", toolName, outputPath),
		Metadata: map[string]interface{}{
			"tool_name":   toolName,
			"binary_path": outputPath,
		},
		FactsToAdd: []Fact{
			{Predicate: "ouroboros_phase", Args: []interface{}{"/compiled"}},
			{Predicate: "tool_compiled", Args: []interface{}{toolName, outputPath}},
		},
	}, nil
}

// handleOuroborosRegister registers a compiled tool.
func (v *VirtualStore) handleOuroborosRegister(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	toolName := req.Target
	binaryPath, _ := req.Payload["binary_path"].(string)
	shardAffinity, _ := req.Payload["shard_affinity"].(string)

	logging.VirtualStore("Ouroboros register: %s at %s", toolName, binaryPath)

	if shardAffinity == "" {
		shardAffinity = "coder"
	}

	if err := v.RegisterTool(toolName, binaryPath, shardAffinity); err != nil {
		return ActionResult{
			Success: false,
			Error:   err.Error(),
			FactsToAdd: []Fact{
				{Predicate: "ouroboros_register_failed", Args: []interface{}{toolName, err.Error()}},
			},
		}, nil
	}

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Tool %s registered", toolName),
		FactsToAdd: []Fact{
			{Predicate: "ouroboros_phase", Args: []interface{}{"/registered"}},
			{Predicate: "tool_registered", Args: []interface{}{toolName, binaryPath}},
			{Predicate: "tool_available", Args: []interface{}{toolName}},
		},
	}, nil
}

// handleRefineTool refines an existing tool.
func (v *VirtualStore) handleRefineTool(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	toolName := req.Target
	feedback, _ := req.Payload["feedback"].(string)

	logging.VirtualStoreDebug("Refine tool: %s", toolName)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Tool refinement initiated for: %s", toolName),
		FactsToAdd: []Fact{
			{Predicate: "tool_refining", Args: []interface{}{toolName, feedback}},
			{Predicate: "ouroboros_phase", Args: []interface{}{"/refine"}},
		},
	}, nil
}

// =============================================================================
// CAMPAIGN ACTION HANDLERS
// =============================================================================

// handleCampaignClarify requests clarification for a campaign goal.
func (v *VirtualStore) handleCampaignClarify(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	question := req.Target
	campaignID, _ := req.Payload["campaign_id"].(string)

	logging.VirtualStoreDebug("Campaign clarify: %s", question)

	return ActionResult{
		Success: false, // Needs user input
		Output:  question,
		Error:   "CAMPAIGN_CLARIFICATION_NEEDED",
		Metadata: map[string]interface{}{
			"campaign_id": campaignID,
			"question":    question,
		},
		FactsToAdd: []Fact{
			{Predicate: "campaign_awaiting_clarification", Args: []interface{}{campaignID, question}},
		},
	}, nil
}

// handleCampaignCreateFile creates a file as part of a campaign.
func (v *VirtualStore) handleCampaignCreateFile(ctx context.Context, req ActionRequest) (ActionResult, error) {
	// Delegate to write file handler
	return v.handleWriteFile(ctx, req)
}

// handleCampaignModifyFile modifies a file as part of a campaign.
func (v *VirtualStore) handleCampaignModifyFile(ctx context.Context, req ActionRequest) (ActionResult, error) {
	// Delegate to edit file handler
	return v.handleEditFile(ctx, req)
}

// handleCampaignWriteTest writes a test file as part of a campaign.
func (v *VirtualStore) handleCampaignWriteTest(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	// Similar to write file but adds test-specific facts
	result, err := v.handleWriteFile(ctx, req)
	if err != nil {
		return result, err
	}

	if result.Success {
		result.FactsToAdd = append(result.FactsToAdd, Fact{
			Predicate: "test_written",
			Args:      []interface{}{req.Target},
		})
	}

	return result, nil
}

// handleCampaignRunTest runs tests as part of a campaign.
func (v *VirtualStore) handleCampaignRunTest(ctx context.Context, req ActionRequest) (ActionResult, error) {
	// Delegate to run tests handler
	return v.handleRunTests(ctx, req)
}

// handleCampaignResearch performs research as part of a campaign.
func (v *VirtualStore) handleCampaignResearch(ctx context.Context, req ActionRequest) (ActionResult, error) {
	// Delegate to research handler
	return v.handleResearch(ctx, req)
}

// handleCampaignVerify verifies a campaign step.
func (v *VirtualStore) handleCampaignVerify(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	stepID := req.Target
	campaignID, _ := req.Payload["campaign_id"].(string)

	logging.VirtualStoreDebug("Campaign verify step: %s", stepID)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Verifying campaign step: %s", stepID),
		FactsToAdd: []Fact{
			{Predicate: "campaign_step_verifying", Args: []interface{}{campaignID, stepID}},
			{Predicate: "campaign_phase", Args: []interface{}{"/verify"}},
		},
	}, nil
}

// handleCampaignDocument creates documentation as part of a campaign.
func (v *VirtualStore) handleCampaignDocument(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	docTarget := req.Target
	content, _ := req.Payload["content"].(string)

	logging.VirtualStoreDebug("Campaign document: %s", docTarget)

	// If content provided, write it
	if content != "" {
		req.Payload["content"] = content
		return v.handleWriteFile(ctx, req)
	}

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Documentation requested for: %s", docTarget),
		FactsToAdd: []Fact{
			{Predicate: "campaign_documenting", Args: []interface{}{docTarget}},
			{Predicate: "campaign_phase", Args: []interface{}{"/document"}},
		},
	}, nil
}

// handleCampaignRefactor performs refactoring as part of a campaign.
func (v *VirtualStore) handleCampaignRefactor(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	target := req.Target
	refactorType, _ := req.Payload["refactor_type"].(string)

	logging.VirtualStoreDebug("Campaign refactor: %s (%s)", target, refactorType)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Refactoring initiated for: %s", target),
		FactsToAdd: []Fact{
			{Predicate: "campaign_refactoring", Args: []interface{}{target, refactorType}},
			{Predicate: "campaign_phase", Args: []interface{}{"/refactor"}},
		},
	}, nil
}

// handleCampaignIntegrate performs integration as part of a campaign.
func (v *VirtualStore) handleCampaignIntegrate(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	target := req.Target
	campaignID, _ := req.Payload["campaign_id"].(string)

	logging.VirtualStoreDebug("Campaign integrate: %s", target)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Integration step for: %s", target),
		FactsToAdd: []Fact{
			{Predicate: "campaign_integrating", Args: []interface{}{campaignID, target}},
			{Predicate: "campaign_phase", Args: []interface{}{"/integrate"}},
		},
	}, nil
}

// handleCampaignComplete marks a campaign as complete.
func (v *VirtualStore) handleCampaignComplete(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	campaignID := req.Target
	summary, _ := req.Payload["summary"].(string)

	logging.VirtualStore("Campaign completed: %s", campaignID)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Campaign %s completed: %s", campaignID, summary),
		FactsToAdd: []Fact{
			{Predicate: "campaign_completed", Args: []interface{}{campaignID, summary}},
			{Predicate: "campaign_phase", Args: []interface{}{"/complete"}},
		},
	}, nil
}

// handleCampaignFinalVerify performs final verification of a campaign.
func (v *VirtualStore) handleCampaignFinalVerify(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	campaignID := req.Target

	logging.VirtualStore("Campaign final verification: %s", campaignID)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Final verification for campaign: %s", campaignID),
		FactsToAdd: []Fact{
			{Predicate: "campaign_final_verifying", Args: []interface{}{campaignID}},
			{Predicate: "campaign_phase", Args: []interface{}{"/final_verify"}},
		},
	}, nil
}

// handleCampaignCleanup performs cleanup after a campaign.
func (v *VirtualStore) handleCampaignCleanup(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	campaignID := req.Target

	logging.VirtualStore("Campaign cleanup: %s", campaignID)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Cleanup completed for campaign: %s", campaignID),
		FactsToAdd: []Fact{
			{Predicate: "campaign_cleaned_up", Args: []interface{}{campaignID}},
			{Predicate: "campaign_phase", Args: []interface{}{"/cleanup"}},
		},
	}, nil
}

// handleArchiveCampaign archives a completed campaign.
func (v *VirtualStore) handleArchiveCampaign(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	campaignID := req.Target

	logging.VirtualStore("Archiving campaign: %s", campaignID)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Campaign %s archived", campaignID),
		FactsToAdd: []Fact{
			{Predicate: "campaign_archived", Args: []interface{}{campaignID}},
		},
	}, nil
}

// handleShowCampaignStatus shows the current status of a campaign.
func (v *VirtualStore) handleShowCampaignStatus(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	campaignID := req.Target

	logging.VirtualStoreDebug("Show campaign status: %s", campaignID)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Showing status for campaign: %s", campaignID),
		FactsToAdd: []Fact{
			{Predicate: "campaign_status_requested", Args: []interface{}{campaignID}},
		},
	}, nil
}

// handleShowCampaignProgress shows the progress of a campaign.
func (v *VirtualStore) handleShowCampaignProgress(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	campaignID := req.Target

	logging.VirtualStoreDebug("Show campaign progress: %s", campaignID)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Showing progress for campaign: %s", campaignID),
		FactsToAdd: []Fact{
			{Predicate: "campaign_progress_requested", Args: []interface{}{campaignID}},
		},
	}, nil
}

// handleAskCampaignInterrupt handles campaign interrupt requests.
func (v *VirtualStore) handleAskCampaignInterrupt(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	campaignID := req.Target
	reason, _ := req.Payload["reason"].(string)

	logging.VirtualStore("Campaign interrupt requested: %s - %s", campaignID, reason)

	return ActionResult{
		Success: false,
		Output:  fmt.Sprintf("Campaign %s interrupt requested: %s", campaignID, reason),
		Error:   "CAMPAIGN_INTERRUPT_REQUESTED",
		FactsToAdd: []Fact{
			{Predicate: "campaign_interrupt_requested", Args: []interface{}{campaignID, reason}},
		},
	}, nil
}

// handleRunPhaseCheckpoint runs a checkpoint for the current phase.
func (v *VirtualStore) handleRunPhaseCheckpoint(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	phaseID := req.Target
	campaignID, _ := req.Payload["campaign_id"].(string)

	logging.VirtualStoreDebug("Phase checkpoint: %s in campaign %s", phaseID, campaignID)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Checkpoint for phase: %s", phaseID),
		FactsToAdd: []Fact{
			{Predicate: "phase_checkpoint", Args: []interface{}{campaignID, phaseID}},
		},
	}, nil
}

// handlePauseAndReplan pauses and replans the current campaign.
func (v *VirtualStore) handlePauseAndReplan(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	campaignID := req.Target
	reason, _ := req.Payload["reason"].(string)

	logging.VirtualStore("Pause and replan: %s - %s", campaignID, reason)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Campaign %s paused for replanning: %s", campaignID, reason),
		FactsToAdd: []Fact{
			{Predicate: "campaign_paused", Args: []interface{}{campaignID, reason}},
			{Predicate: "campaign_replanning", Args: []interface{}{campaignID}},
		},
	}, nil
}

// =============================================================================
// CONTEXT MANAGEMENT ACTION HANDLERS
// =============================================================================

// handleCompressContext compresses the current context.
func (v *VirtualStore) handleCompressContext(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	reason := req.Target
	targetRatio, _ := req.Payload["ratio"].(float64)
	if targetRatio == 0 {
		targetRatio = 0.5
	}

	logging.VirtualStore("Context compression requested: ratio=%.2f", targetRatio)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Context compression initiated (target ratio: %.2f)", targetRatio),
		FactsToAdd: []Fact{
			{Predicate: "context_compressing", Args: []interface{}{reason, targetRatio}},
			{Predicate: "compression_requested", Args: []interface{}{"/normal"}},
		},
	}, nil
}

// handleEmergencyCompress performs emergency context compression.
func (v *VirtualStore) handleEmergencyCompress(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	logging.VirtualStore("Emergency context compression requested")

	return ActionResult{
		Success: true,
		Output:  "Emergency context compression initiated",
		FactsToAdd: []Fact{
			{Predicate: "context_compressing", Args: []interface{}{"emergency", 0.25}},
			{Predicate: "compression_requested", Args: []interface{}{"/emergency"}},
		},
	}, nil
}

// handleCreateCheckpoint creates a checkpoint of the current state.
func (v *VirtualStore) handleCreateCheckpoint(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	checkpointName := req.Target
	if checkpointName == "" {
		checkpointName = fmt.Sprintf("checkpoint_%d", time.Now().Unix())
	}

	logging.VirtualStore("Creating checkpoint: %s", checkpointName)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Checkpoint created: %s", checkpointName),
		Metadata: map[string]interface{}{
			"checkpoint_name": checkpointName,
			"timestamp":       time.Now().Unix(),
		},
		FactsToAdd: []Fact{
			{Predicate: "checkpoint_created", Args: []interface{}{checkpointName, time.Now().Unix()}},
		},
	}, nil
}

// =============================================================================
// INVESTIGATION/ANALYSIS ACTION HANDLERS
// =============================================================================

// handleInvestigateAnomaly investigates an anomaly.
func (v *VirtualStore) handleInvestigateAnomaly(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	anomalyDesc := req.Target
	severity, _ := req.Payload["severity"].(string)
	if severity == "" {
		severity = "medium"
	}

	logging.VirtualStore("Investigating anomaly: %s (severity=%s)", anomalyDesc, severity)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Investigating anomaly: %s", anomalyDesc),
		FactsToAdd: []Fact{
			{Predicate: "anomaly_investigating", Args: []interface{}{anomalyDesc, severity}},
			{Predicate: "investigation_phase", Args: []interface{}{"/anomaly"}},
		},
	}, nil
}

// handleInvestigateSystemic investigates a systemic issue.
func (v *VirtualStore) handleInvestigateSystemic(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	issueDesc := req.Target

	logging.VirtualStore("Investigating systemic issue: %s", issueDesc)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Investigating systemic issue: %s", issueDesc),
		FactsToAdd: []Fact{
			{Predicate: "systemic_investigating", Args: []interface{}{issueDesc}},
			{Predicate: "investigation_phase", Args: []interface{}{"/systemic"}},
		},
	}, nil
}

// handleUpdateWorldModel updates the world model.
func (v *VirtualStore) handleUpdateWorldModel(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	updateType := req.Target
	scope, _ := req.Payload["scope"].(string)

	logging.VirtualStoreDebug("Updating world model: type=%s, scope=%s", updateType, scope)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("World model update: %s", updateType),
		FactsToAdd: []Fact{
			{Predicate: "world_model_updating", Args: []interface{}{updateType, scope}},
		},
	}, nil
}

// =============================================================================
// CORRECTIVE ACTION HANDLERS
// =============================================================================

// handleCorrectiveResearch performs research to correct an issue.
func (v *VirtualStore) handleCorrectiveResearch(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	topic := req.Target
	issueType, _ := req.Payload["issue_type"].(string)

	logging.VirtualStoreDebug("Corrective research: %s (issue=%s)", topic, issueType)

	// If scraper is available, delegate to research handler
	scraper := v.GetMCPClient("scraper")

	if scraper != nil {
		return v.handleResearch(ctx, req)
	}

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Corrective research initiated for: %s", topic),
		FactsToAdd: []Fact{
			{Predicate: "corrective_researching", Args: []interface{}{topic, issueType}},
			{Predicate: "corrective_phase", Args: []interface{}{"/research"}},
		},
	}, nil
}

// handleCorrectiveDocs creates corrective documentation.
func (v *VirtualStore) handleCorrectiveDocs(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	docTarget := req.Target

	logging.VirtualStoreDebug("Corrective documentation: %s", docTarget)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Corrective documentation initiated for: %s", docTarget),
		FactsToAdd: []Fact{
			{Predicate: "corrective_documenting", Args: []interface{}{docTarget}},
			{Predicate: "corrective_phase", Args: []interface{}{"/docs"}},
		},
	}, nil
}

// handleCorrectiveDecompose decomposes a problem for correction.
func (v *VirtualStore) handleCorrectiveDecompose(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	problem := req.Target

	logging.VirtualStoreDebug("Corrective decompose: %s", problem)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Decomposing problem for correction: %s", problem),
		FactsToAdd: []Fact{
			{Predicate: "corrective_decomposing", Args: []interface{}{problem}},
			{Predicate: "corrective_phase", Args: []interface{}{"/decompose"}},
		},
	}, nil
}
