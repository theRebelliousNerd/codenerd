package campaign

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/session"
	"codenerd/internal/tactile"
)

// spawnTask is the unified entry point for task execution.
// It uses TaskExecutor when available, falling back to ShardManager.
//
// Migration helper: replaces direct shardMgr.Spawn() calls.
// Once all consumers migrate, ShardManager can be removed.
func (o *Orchestrator) spawnTask(ctx context.Context, shardType string, task string) (string, error) {
	o.mu.RLock()
	te := o.taskExecutor
	sm := o.shardMgr
	o.mu.RUnlock()

	// Prefer TaskExecutor when available
	if te != nil {
		intent := session.LegacyShardNameToIntent(shardType)
		logging.CampaignDebug("spawnTask: using TaskExecutor (intent=%s) for %s", intent, shardType)
		return te.Execute(ctx, intent, task)
	}

	// Fall back to ShardManager
	if sm != nil {
		logging.CampaignDebug("spawnTask: using ShardManager for %s", shardType)
		return sm.Spawn(ctx, shardType, task)
	}

	return "", fmt.Errorf("no executor available: both TaskExecutor and ShardManager are nil")
}

// executeTask executes a single task.
func (o *Orchestrator) executeTask(ctx context.Context, task *Task) (any, error) {
	logging.CampaignDebug("Executing task %s with type %s, shard=%s", task.ID, task.Type, task.Shard)

	// Update task status
	o.updateTaskStatus(task, TaskInProgress)

	// If task has explicit shard specified, use generic shard routing with context injection
	if task.Shard != "" {
		logging.CampaignDebug("Using explicit shard routing: %s", task.Shard)
		return o.executeWithExplicitShard(ctx, task)
	}

	// Fallback to type-based routing for backward compatibility
	switch task.Type {
	case TaskTypeAssaultDiscover:
		logging.CampaignDebug("Delegating to assault discover handler")
		return o.executeAssaultDiscoverTask(ctx, task)
	case TaskTypeAssaultBatch:
		logging.CampaignDebug("Delegating to assault batch handler")
		return o.executeAssaultBatchTask(ctx, task)
	case TaskTypeAssaultTriage:
		logging.CampaignDebug("Delegating to assault triage handler")
		return o.executeAssaultTriageTask(ctx, task)
	case TaskTypeResearch:
		logging.CampaignDebug("Delegating to research task handler")
		return o.executeResearchTask(ctx, task)
	case TaskTypeFileCreate, TaskTypeFileModify:
		logging.CampaignDebug("Delegating to file task handler")
		return o.executeFileTask(ctx, task)
	case TaskTypeTestWrite:
		logging.CampaignDebug("Delegating to test write handler")
		return o.executeTestWriteTask(ctx, task)
	case TaskTypeTestRun:
		logging.CampaignDebug("Delegating to test run handler")
		return o.executeTestRunTask(ctx, task)
	case TaskTypeVerify:
		logging.CampaignDebug("Delegating to verify handler")
		return o.executeVerifyTask(ctx, task)
	case TaskTypeShardSpawn:
		logging.CampaignDebug("Delegating to shard spawn handler")
		return o.executeShardSpawnTask(ctx, task)
	case TaskTypeRefactor:
		logging.CampaignDebug("Delegating to refactor handler")
		return o.executeRefactorTask(ctx, task)
	case TaskTypeIntegrate:
		logging.CampaignDebug("Delegating to integrate handler")
		return o.executeIntegrateTask(ctx, task)
	case TaskTypeDocument:
		logging.CampaignDebug("Delegating to document handler")
		return o.executeDocumentTask(ctx, task)
	case TaskTypeToolCreate:
		logging.CampaignDebug("Delegating to tool create handler (Ouroboros)")
		return o.executeToolCreateTask(ctx, task)
	case TaskTypeCampaignRef:
		logging.CampaignDebug("Delegating to sub-campaign handler")
		return o.executeCampaignRefTask(ctx, task)
	default:
		logging.CampaignDebug("Using generic task handler for type: %s", task.Type)
		return o.executeGenericTask(ctx, task)
	}
}

// executeWithExplicitShard handles tasks with explicitly specified shard routing.
// This enables the campaign system to call ANY shard at will with context injection.
func (o *Orchestrator) executeWithExplicitShard(ctx context.Context, task *Task) (any, error) {
	shardType := task.Shard
	logging.Campaign("Executing task %s with explicit shard: %s", task.ID, shardType)

	// Build input with context injection from dependent tasks AND specialist knowledge
	input := o.buildTaskInputWithSpecialistKnowledge(ctx, task, shardType)
	logging.CampaignDebug("Built shard input (%d bytes) for task %s", len(input), task.ID)

	// Spawn the shard via unified spawnTask
	result, err := o.spawnTask(ctx, shardType, input)
	if err != nil {
		logging.Get(logging.CategoryCampaign).Error("Shard %s failed for task %s: %v", shardType, task.ID, err)
		return nil, fmt.Errorf("shard %s failed: %w", shardType, err)
	}

	logging.CampaignDebug("Shard %s completed for task %s, result_len=%d", shardType, task.ID, len(result))

	return map[string]interface{}{
		"shard":  shardType,
		"result": result,
		"task":   task.ID,
	}, nil
}

// executeResearchTask spawns a researcher shard.
func (o *Orchestrator) executeResearchTask(ctx context.Context, task *Task) (any, error) {
	logging.CampaignDebug("Spawning researcher shard for task %s", task.ID)
	result, err := o.spawnTask(ctx, "researcher", task.Description)
	if err != nil {
		logging.Get(logging.CategoryCampaign).Error("Researcher shard failed for task %s: %v", task.ID, err)
		return nil, err
	}
	logging.CampaignDebug("Researcher shard completed for task %s", task.ID)
	return map[string]interface{}{"research_result": result}, nil
}

// executeFileTask creates or modifies a file using the Coder shard.
func (o *Orchestrator) executeFileTask(ctx context.Context, task *Task) (any, error) {
	// Get target path from artifacts
	var targetPath string
	if len(task.Artifacts) > 0 {
		targetPath = task.Artifacts[0].Path
	}
	logging.CampaignDebug("Executing file task %s: path=%s", task.ID, targetPath)

	// Build task string for coder shard
	// NOTE: Don't use "instruction:<value>" format because strings.Fields() splits on spaces,
	// causing multi-word instructions to be truncated. Use simpler format where bare words
	// are joined into the instruction by parseTask.
	action := "create"
	if task.Type == TaskTypeFileModify {
		action = "modify"
	}
	shardTask := fmt.Sprintf("%s file:%s %s", action, targetPath, task.Description)
	logging.CampaignDebug("Spawning coder shard: action=%s, path=%s, task=%s", action, targetPath, shardTask)

	// Delegate to coder shard
	result, err := o.spawnTask(ctx, "coder", shardTask)
	if err != nil {
		logging.Get(logging.CategoryCampaign).Warn("Coder shard failed for task %s, using fallback: %v", task.ID, err)
		// Fallback to direct LLM if shard fails
		return o.executeFileTaskFallback(ctx, task, targetPath)
	}

	logging.CampaignDebug("Coder shard completed for task %s, result_len=%d", task.ID, len(result))

	// CRITICAL: Verify file was actually written
	// Shards may return successfully without calling write_file tool
	fullPath := filepath.Join(o.workspace, targetPath)
	if _, statErr := os.Stat(fullPath); os.IsNotExist(statErr) {
		logging.Get(logging.CategoryCampaign).Warn("Coder shard returned but file not created: %s, using fallback", fullPath)
		// Shard didn't write file - fall back to direct LLM
		return o.executeFileTaskFallback(ctx, task, targetPath)
	}

	logging.Campaign("File verified after shard execution: %s", fullPath)
	return map[string]interface{}{"coder_result": result, "path": targetPath}, nil
}

// executeFileTaskFallback uses direct LLM when shard is unavailable.
func (o *Orchestrator) executeFileTaskFallback(ctx context.Context, task *Task, targetPath string) (any, error) {
	logging.CampaignDebug("Executing file task fallback for %s via direct LLM", task.ID)

	// If no target path, try to extract from task description or fail
	if targetPath == "" {
		// Try to extract path from description (look for common patterns)
		targetPath = extractPathFromDescription(task.Description)
		if targetPath == "" {
			logging.Get(logging.CategoryCampaign).Error("No target path for file task %s and could not extract from description", task.ID)
			return nil, fmt.Errorf("no target path specified for file task %s", task.ID)
		}
		logging.CampaignDebug("Extracted target path from description: %s", targetPath)
	}

	prompt := fmt.Sprintf(`Generate the following file:
Task: %s
Target Path: %s

Output ONLY the file content, no explanation or markdown fences:`, task.Description, targetPath)

	content, err := o.llmClient.Complete(ctx, prompt)
	if err != nil {
		logging.Get(logging.CategoryCampaign).Error("LLM file generation failed for task %s: %v", task.ID, err)
		return nil, err
	}

	// Extract code block from LLM response (removes reasoning traces and markdown fences)
	lang := getLangFromPath(targetPath)
	content = extractCodeBlock(content, lang)
	logging.CampaignDebug("Extracted code block for %s (lang=%s, %d bytes)", targetPath, lang, len(content))

	fullPath := filepath.Join(o.workspace, targetPath)
	logging.CampaignDebug("Writing generated file: %s (%d bytes)", fullPath, len(content))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		logging.Get(logging.CategoryCampaign).Error("Failed to create directory for %s: %v", fullPath, err)
		return nil, err
	}

	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		logging.Get(logging.CategoryCampaign).Error("Failed to write file %s: %v", fullPath, err)
		return nil, err
	}

	logging.CampaignDebug("File fallback completed: %s", fullPath)
	return map[string]interface{}{"path": fullPath, "size": len(content)}, nil
}

// executeTestWriteTask writes tests for existing code using the Tester shard.
func (o *Orchestrator) executeTestWriteTask(ctx context.Context, task *Task) (any, error) {
	// Get target file from artifacts
	var targetPath string
	if len(task.Artifacts) > 0 {
		targetPath = task.Artifacts[0].Path
	}
	logging.CampaignDebug("Executing test write task %s: target=%s", task.ID, targetPath)

	// Build task string for tester shard
	shardTask := fmt.Sprintf("generate_tests file:%s", targetPath)
	logging.CampaignDebug("Spawning tester shard for test generation")

	// Delegate to tester shard
	result, err := o.spawnTask(ctx, "tester", shardTask)
	if err != nil {
		logging.Get(logging.CategoryCampaign).Warn("Tester shard failed for test write task %s, falling back to coder: %v", task.ID, err)
		// Fallback to coder shard for test generation
		return o.executeFileTask(ctx, task)
	}

	logging.CampaignDebug("Test write task completed: %s", task.ID)
	return map[string]interface{}{"tester_result": result, "target": targetPath}, nil
}

// executeTestRunTask runs tests using the Tester shard.
func (o *Orchestrator) executeTestRunTask(ctx context.Context, task *Task) (any, error) {
	// Get target from artifacts or use default
	target := "./..."
	if len(task.Artifacts) > 0 {
		target = task.Artifacts[0].Path
	}
	logging.CampaignDebug("Executing test run task %s: target=%s", task.ID, target)

	// Build task string for tester shard
	shardTask := fmt.Sprintf("run_tests package:%s", target)
	logging.CampaignDebug("Spawning tester shard for test execution")

	// Delegate to tester shard
	result, err := o.spawnTask(ctx, "tester", shardTask)
	if err != nil {
		logging.Get(logging.CategoryCampaign).Warn("Tester shard failed for test run task %s, using direct execution: %v", task.ID, err)
		// Fallback to direct execution
		cmd := tactile.Command{
			Binary:           "go",
			Arguments:        []string{"test", target},
			WorkingDirectory: o.workspace,
			Limits: &tactile.ResourceLimits{
				TimeoutMs: 300 * 1000,
			},
		}
		logging.CampaignDebug("Executing tests directly via tactile: go test %s", target)
		res, execErr := o.executor.Execute(ctx, cmd)
		output := ""
		if res != nil {
			output = res.Output()
		}
		if execErr != nil {
			logging.Get(logging.CategoryCampaign).Error("Test execution failed: %v", execErr)
			return map[string]interface{}{"output": output, "passed": false}, execErr
		}
		logging.Campaign("Tests passed via direct execution")
		return map[string]interface{}{"output": output, "passed": true}, nil
	}

	logging.CampaignDebug("Test run task completed: %s", task.ID)
	return map[string]interface{}{"tester_result": result, "target": target}, nil
}

// executeVerifyTask runs verification (build, lint, etc.).
func (o *Orchestrator) executeVerifyTask(ctx context.Context, task *Task) (any, error) {
	logging.CampaignDebug("Executing verify task %s: go build ./...", task.ID)
	// Run build verification for this task
	cmd := tactile.Command{
		Binary:           "go",
		Arguments:        []string{"build", "./..."},
		WorkingDirectory: o.workspace,
		Limits: &tactile.ResourceLimits{
			TimeoutMs: 300 * 1000, // 5 minutes
		},
	}
	res, err := o.executor.Execute(ctx, cmd)
	output := ""
	if res != nil {
		output = res.Output()
	}
	if err != nil {
		logging.Get(logging.CategoryCampaign).Error("Verify task %s failed: %v", task.ID, err)
		return map[string]interface{}{
			"task_id":  task.ID,
			"output":   output,
			"verified": false,
		}, err
	}
	logging.Campaign("Verify task %s passed", task.ID)
	return map[string]interface{}{
		"task_id":  task.ID,
		"output":   output,
		"verified": true,
	}, nil
}

// executeShardSpawnTask spawns a specialized shard.
func (o *Orchestrator) executeShardSpawnTask(ctx context.Context, task *Task) (any, error) {
	// Extract shard type from description
	shardType := "coder" // Default
	logging.CampaignDebug("Executing shard spawn task %s: type=%s", task.ID, shardType)
	result, err := o.spawnTask(ctx, shardType, task.Description)
	if err != nil {
		logging.Get(logging.CategoryCampaign).Error("Shard spawn task %s failed: %v", task.ID, err)
		return nil, err
	}
	logging.CampaignDebug("Shard spawn task completed: %s", task.ID)
	return map[string]interface{}{"shard_result": result}, nil
}

// executeRefactorTask refactors existing code using the Coder shard.
func (o *Orchestrator) executeRefactorTask(ctx context.Context, task *Task) (any, error) {
	// Get target files from artifacts
	var targetPath string
	if len(task.Artifacts) > 0 {
		targetPath = task.Artifacts[0].Path
	}
	logging.CampaignDebug("Executing refactor task %s: path=%s", task.ID, targetPath)

	// Build task string for coder shard
	shardTask := fmt.Sprintf("refactor file:%s instruction:%s", targetPath, task.Description)
	logging.CampaignDebug("Spawning coder shard for refactoring")

	// Delegate to coder shard
	result, err := o.spawnTask(ctx, "coder", shardTask)
	if err != nil {
		logging.Get(logging.CategoryCampaign).Warn("Refactor shard failed for task %s, falling back to file task: %v", task.ID, err)
		// Fallback to generic file task
		return o.executeFileTask(ctx, task)
	}

	logging.CampaignDebug("Refactor task completed: %s", task.ID)
	return map[string]interface{}{"coder_result": result, "path": targetPath}, nil
}

// executeIntegrateTask integrates components.
func (o *Orchestrator) executeIntegrateTask(ctx context.Context, task *Task) (any, error) {
	logging.CampaignDebug("Executing integrate task %s via file task", task.ID)
	return o.executeFileTask(ctx, task)
}

// executeDocumentTask generates documentation.
func (o *Orchestrator) executeDocumentTask(ctx context.Context, task *Task) (any, error) {
	logging.CampaignDebug("Executing document task %s via file task", task.ID)
	return o.executeFileTask(ctx, task)
}

// executeToolCreateTask triggers tool generation via kernel-mediated autopoiesis.
// It asserts missing_tool_for fact to the kernel, which derives delegate_task(/tool_generator, ...).
// The autopoiesis orchestrator listens for these derived facts and generates the tool.
func (o *Orchestrator) executeToolCreateTask(ctx context.Context, task *Task) (any, error) {
	logging.Campaign("Executing tool create task %s (Ouroboros)", task.ID)
	// Extract tool capability from task description or artifacts
	// For tool creation, the Path field contains the tool/capability name
	capability := task.Description
	if len(task.Artifacts) > 0 && task.Artifacts[0].Path != "" {
		capability = task.Artifacts[0].Path
	}
	logging.CampaignDebug("Tool capability requested: %s", capability)

	// Generate intent ID for this tool creation request
	intentID := fmt.Sprintf("campaign_%s_task_%s", o.campaign.ID, task.ID)
	logging.CampaignDebug("Tool creation intent ID: %s", intentID)

	// Assert missing_tool_for to kernel - this triggers the policy rules:
	// 1. delegate_task(/tool_generator, Cap, /pending) derives
	// 2. next_action(/generate_tool) derives
	// 3. Autopoiesis orchestrator picks up the delegation
	err := o.kernel.Assert(core.Fact{
		Predicate: "missing_tool_for",
		Args:      []interface{}{intentID, capability},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to assert missing_tool_for: %w", err)
	}

	// Also assert goal_requires so the policy can derive properly
	err = o.kernel.Assert(core.Fact{
		Predicate: "goal_requires",
		Args:      []interface{}{o.campaign.Goal, capability},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to assert goal_requires: %w", err)
	}

	// Emit event for visibility
	o.emitEvent("tool_generation_requested", "", task.ID, capability, map[string]interface{}{
		"intent_id":  intentID,
		"capability": capability,
	})

	// Poll for tool_ready or tool_registered fact (with timeout)
	timeout := time.After(30 * time.Minute)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			// Tool generation timed out - return partial success
			// The tool may still be generating in the background
			return map[string]interface{}{
				"status":     "pending",
				"capability": capability,
				"message":    "tool generation initiated but not yet complete",
			}, nil
		case <-ticker.C:
			// Check if tool is now registered
			facts, err := o.kernel.Query("tool_registered")
			if err == nil {
				for _, fact := range facts {
					if len(fact.Args) > 0 {
						if toolName, ok := fact.Args[0].(string); ok && toolName == capability {
							return map[string]interface{}{
								"status":     "complete",
								"capability": capability,
								"tool_name":  toolName,
							}, nil
						}
					}
				}
			}

			// Also check has_capability
			capFacts, capErr := o.kernel.Query("has_capability")
			if capErr == nil {
				for _, fact := range capFacts {
					if len(fact.Args) > 0 {
						if cap, ok := fact.Args[0].(string); ok && cap == capability {
							return map[string]interface{}{
								"status":     "complete",
								"capability": capability,
							}, nil
						}
					}
				}
			}
		}
	}
}

// executeCampaignRefTask handles a sub-campaign reference.
// Currently it validates the sub-campaign ID and logs the intent.
// In a full fractal implementation, this would spawn a child Orchestrator.
func (o *Orchestrator) executeCampaignRefTask(ctx context.Context, task *Task) (any, error) {
	logging.CampaignDebug("Executing campaign ref task %s", task.ID)
	if task.SubCampaignID == "" {
		logging.Get(logging.CategoryCampaign).Error("Task %s has type /campaign_ref but no sub_campaign_id", task.ID)
		return nil, fmt.Errorf("task %s has type /campaign_ref but no sub_campaign_id", task.ID)
	}

	logging.Campaign("Linking sub-campaign: %s", task.SubCampaignID)
	o.emitEvent("sub_campaign_referenced", "", task.ID, fmt.Sprintf("Linking sub-campaign %s", task.SubCampaignID), nil)

	// In the future, this would look like:
	// childOrch := NewOrchestrator(o.kernel, o.llmClient, ...)
	// childOrch.LoadCampaign(task.SubCampaignID)
	// err := childOrch.Run(ctx)

	// For now, we treat it as a pointer that is "satisfied" if the sub-campaign exists or is acknowledged.
	logging.CampaignDebug("Sub-campaign %s linked (fractal execution not yet implemented)", task.SubCampaignID)
	return map[string]interface{}{
		"sub_campaign_id": task.SubCampaignID,
		"status":          "linked",
	}, nil
}

// executeGenericTask runs a generic task via shard delegation.
func (o *Orchestrator) executeGenericTask(ctx context.Context, task *Task) (any, error) {
	logging.CampaignDebug("Executing generic task %s via coder shard", task.ID)
	result, err := o.spawnTask(ctx, "coder", task.Description)
	if err != nil {
		logging.Get(logging.CategoryCampaign).Error("Generic task %s failed: %v", task.ID, err)
		return nil, err
	}
	logging.CampaignDebug("Generic task completed: %s", task.ID)
	return map[string]interface{}{"result": result}, nil
}

// extractCodeBlock extracts code from LLM response that may contain markdown fences.
// Returns the code inside ```lang or ``` blocks, or the original text if no fences found.
func extractCodeBlock(text, lang string) string {
	// Look for ```lang or ``` blocks
	patterns := []string{
		"```" + lang + "\n",
		"```" + lang + "\r\n",
		"```\n",
		"```\r\n",
	}

	for _, pattern := range patterns {
		if idx := strings.Index(text, pattern); idx != -1 {
			start := idx + len(pattern)
			end := strings.Index(text[start:], "```")
			if end != -1 {
				return strings.TrimSpace(text[start : start+end])
			}
		}
	}

	// If no code block found, return the whole text (might be raw code)
	return strings.TrimSpace(text)
}

// getLangFromPath returns the language identifier for a file path.
func getLangFromPath(path string) string {
	ext := strings.TrimPrefix(filepath.Ext(path), ".")
	switch ext {
	case "go":
		return "go"
	case "ts", "tsx":
		return "typescript"
	case "js", "jsx":
		return "javascript"
	case "kt":
		return "kotlin"
	case "py":
		return "python"
	case "sql":
		return "sql"
	case "yaml", "yml":
		return "yaml"
	case "json":
		return "json"
	case "md":
		return "markdown"
	default:
		return ext
	}
}

// extractPathFromDescription attempts to extract a file path from a task description.
// Looks for common patterns like "Create internal/domain/foo.go" or "file: path/to/file.go"
func extractPathFromDescription(desc string) string {
	// Common path patterns in task descriptions
	patterns := []string{
		`(?i)create\s+(\S+\.\w+)`,           // "Create internal/domain/foo.go"
		`(?i)file[:\s]+(\S+\.\w+)`,          // "file: path/to/file.go"
		`(?i)(\S+/\S+\.\w+)`,                // Any path with / and extension
		`(?i)internal/\S+\.\w+`,             // internal/... paths
		`(?i)cmd/\S+\.\w+`,                  // cmd/... paths
		`(?i)pkg/\S+\.\w+`,                  // pkg/... paths
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(desc)
		if len(matches) > 1 {
			path := matches[1]
			// Validate it looks like a real path
			if strings.Contains(path, "/") && strings.Contains(path, ".") {
				return path
			}
		} else if len(matches) == 1 {
			path := matches[0]
			if strings.Contains(path, "/") && strings.Contains(path, ".") {
				return path
			}
		}
	}

	return ""
}
