package campaign

import (
	"codenerd/internal/logging"
	"strings"
)

// storeTaskResult stores a task's result for context injection into dependent tasks.
func (o *Orchestrator) storeTaskResult(taskID, result string) {
	// Compute which results are still needed by pending/active tasks.
	needed := o.computeNeededResultIDs()

	o.resultsMu.Lock()
	defer o.resultsMu.Unlock()
	// Truncate if too large (keep first 10KB for context injection)
	if len(result) > 10240 {
		result = result[:10240] + "\n... [truncated]"
	}

	// Maintain insertion/LRU order
	if _, exists := o.taskResults[taskID]; exists {
		for i, id := range o.taskResultOrder {
			if id == taskID {
				o.taskResultOrder = append(o.taskResultOrder[:i], o.taskResultOrder[i+1:]...)
				break
			}
		}
	}
	o.taskResultOrder = append(o.taskResultOrder, taskID)

	o.taskResults[taskID] = result
	logging.CampaignDebug("Stored result for task %s (%d bytes)", taskID, len(result))

	// Prune cache if needed.
	limit := o.config.TaskResultCacheLimit
	if limit <= 0 {
		limit = 100
	}
	if len(o.taskResultOrder) > limit {
		pruned := 0
		rotations := 0
		for len(o.taskResultOrder) > limit && rotations < len(o.taskResultOrder) {
			oldest := o.taskResultOrder[0]
			o.taskResultOrder = o.taskResultOrder[1:]
			if needed[oldest] {
				// Keep needed results by rotating to the back.
				o.taskResultOrder = append(o.taskResultOrder, oldest)
			} else {
				delete(o.taskResults, oldest)
				pruned++
			}
			rotations++
		}
		if pruned > 0 {
			logging.CampaignDebug("Pruned %d task results (limit=%d)", pruned, limit)
		}
	}
}

// computeNeededResultIDs returns the set of task IDs whose results are referenced
// by pending/in-progress/blocked tasks via ContextFrom.
func (o *Orchestrator) computeNeededResultIDs() map[string]bool {
	needed := make(map[string]bool)
	o.mu.RLock()
	defer o.mu.RUnlock()
	if o.campaign == nil {
		return needed
	}
	for _, phase := range o.campaign.Phases {
		for _, task := range phase.Tasks {
			if task.Status == TaskPending || task.Status == TaskInProgress || task.Status == TaskBlocked {
				for _, dep := range task.ContextFrom {
					needed[dep] = true
				}
			}
		}
	}
	return needed
}

// getTaskResult retrieves a stored task result for context injection.
func (o *Orchestrator) getTaskResult(taskID string) (string, bool) {
	o.resultsMu.RLock()
	defer o.resultsMu.RUnlock()
	result, ok := o.taskResults[taskID]
	return result, ok
}

// buildTaskInput constructs the input for a shard by combining the task's
// ShardInput/Description with context from dependent tasks.
func (o *Orchestrator) buildTaskInput(task *Task) string {
	if task == nil {
		return ""
	}
	// Start with explicit shard input if provided, otherwise use description
	input := task.ShardInput
	if input == "" {
		input = task.Description
	}

	// Inject context from dependent tasks specified in ContextFrom
	if len(task.ContextFrom) > 0 {
		type contextData struct {
			depID  string
			result string
		}

		contexts := make([]contextData, 0, len(task.ContextFrom))
		var totalLen int = len(input)

		for _, depID := range task.ContextFrom {
			if result, ok := o.getTaskResult(depID); ok && result != "" {
				contexts = append(contexts, contextData{depID, result})
				totalLen += 29 + len(depID) + len(result) // length of "\n\n=== CONTEXT FROM TASK  ===\n" is 29
				logging.CampaignDebug("Injected context from task %s (%d bytes)", depID, len(result))
			}
		}

		if len(contexts) > 0 {
			var builder strings.Builder
			builder.Grow(totalLen)
			builder.WriteString(input)

			for _, ctx := range contexts {
				builder.WriteString("\n\n=== CONTEXT FROM TASK ")
				builder.WriteString(ctx.depID)
				builder.WriteString(" ===\n")
				builder.WriteString(ctx.result)
			}
			input = builder.String()
		}
	}

	// Holographic context: every shard input carries the durable upstream
	// findings (phase dependencies + earlier same-phase tasks), bounded so a
	// long campaign cannot blow the prompt. Prompt-only: persistTaskOutputArtifact
	// stores the shard's result, never this input, so the section is not
	// re-persisted as the task's own artifact.
	if upstream := o.upstreamArtifactContext(task); upstream != "" {
		if input != "" {
			input += "\n\n"
		}
		input += upstream
	}

	return input
}

// inferShardFromTaskType maps a TaskType to its default shard for backward compatibility.
// Tasks with explicit Shard fields bypass this inference.
func inferShardFromTaskType(taskType TaskType) string {
	switch taskType {
	case TaskTypeFileCreate, TaskTypeFileModify, TaskTypeRefactor, TaskTypeDocument, TaskTypeIntegrate:
		return "coder"
	case TaskTypeTestWrite, TaskTypeTestRun:
		return "tester"
	case TaskTypeResearch:
		return "researcher"
	case TaskTypeVerify:
		return "reviewer"
	default:
		return "coder" // Default to coder for unknown types
	}
}
