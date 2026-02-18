// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains the Continuation Protocol for multi-step task execution.
package chat

import (
	"context"
	"fmt"
	"strings"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/types"

	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// CONTINUATION PROTOCOL - Multi-Step Task Execution
// =============================================================================

// checkContinuation checks if there are pending subtasks after shard execution.
// Returns the next continueMsg if more work is needed, nil otherwise.
// This is called from processInput after each shard completes.
func (m *Model) checkContinuation(shardType, task, result string) *continueMsg {
	// Inject result facts to enable continuation derivation
	m.injectShardResultFacts(shardType, task, result, nil)

	if m.kernel == nil {
		return nil
	}

	// Query for continuation signal
	shouldContinue, _ := m.kernel.Query("should_auto_continue")
	pending, _ := m.kernel.Query("has_pending_subtask")

	if len(shouldContinue) > 0 && len(pending) > 0 {
		// Extract first pending subtask
		subtask := pending[0]
		if len(subtask.Args) >= 3 {
			nextID, _ := subtask.Args[0].(string)
			nextDesc, _ := subtask.Args[1].(string)
			nextShard := types.StripAtomPrefix(types.ExtractString(subtask.Args[2]))

			// Check if this is a mutation operation
			isMutation := isMutationOperation(nextShard)
			return &continueMsg{
				subtaskID:   nextID,
				description: nextDesc,
				shardType:   nextShard,
				isMutation:  isMutation,
			}
		}
	}

	return nil
}

// executeSubtask executes a single subtask and checks for continuation.
// Returns either a continueMsg (more work) or continuationDoneMsg (complete).
func (m Model) executeSubtask(subtaskID, description, shardType string) tea.Cmd {
	return func() tea.Msg {
		// Use per-shard execution timeout from config when available.
		timeout := config.GetLLMTimeouts().ShardExecutionTimeout
		if m.Config != nil {
			profile := m.Config.GetShardProfile(shardType)
			if profile.MaxExecutionTimeSec > 0 {
				timeout = time.Duration(profile.MaxExecutionTimeSec) * time.Second
			}
		}

		// Log start of subtask execution
		logging.Get(logging.CategoryRouting).Info("Executing subtask %s (%s) with shard %s", subtaskID, description, shardType)

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		// Check for interrupt before starting
		if m.isInterrupted {
			return continuationDoneMsg{
				stepCount: m.continuationStep,
				summary:   "Stopped by user (Ctrl+X)",
			}
		}

		// Build session context for shard execution
		sessionCtx := m.buildSessionContext(ctx)

		// Execute the shard with queue backpressure (user-initiated = high priority)
		result, err := m.spawnTaskWithContext(ctx, shardType, description, sessionCtx, types.PriorityHigh)

		// Inject result facts into kernel
		m.injectShardResultFacts(shardType, description, result, err)

		payload := &ShardResultPayload{
			ShardType: shardType,
			Task:      description,
			Result:    result,
			Facts:     nil,
		}

		if err != nil {
			return continuationDoneMsg{
				stepCount:            m.continuationStep,
				summary:              fmt.Sprintf("Step %d failed: %v", m.continuationStep, err),
				completedShardResult: payload,
			}
		}

		// Check for more pending subtasks from kernel
		if m.kernel != nil {
			// Query for continuation signal
			shouldContinue, _ := m.kernel.Query("should_auto_continue")
			pending, _ := m.kernel.Query("has_pending_subtask")

			if len(shouldContinue) > 0 && len(pending) > 0 {
				// Extract first pending subtask
				subtask := pending[0]
				if len(subtask.Args) >= 3 {
					nextID, _ := subtask.Args[0].(string)
					nextDesc, _ := subtask.Args[1].(string)
					nextShard := types.StripAtomPrefix(types.ExtractString(subtask.Args[2]))

					// Check if this is a mutation operation
					isMutation := isMutationOperation(nextShard)

					// Update total steps if we discover more (send to Update thread)
					newTotal := m.continuationTotal
					if m.continuationStep >= m.continuationTotal {
						newTotal = m.continuationStep + 1
					}

					return continueMsg{
						subtaskID:            nextID,
						description:          nextDesc,
						shardType:            nextShard,
						isMutation:           isMutation,
						totalSteps:           newTotal,
						completedShardResult: payload,
					}
				}
			}
		}

		// No more continuation - we're done
		return continuationDoneMsg{
			stepCount:            m.continuationStep,
			summary:              fmt.Sprintf("Completed %d steps successfully.", m.continuationStep),
			completedShardResult: payload,
		}
	}
}

// injectShardResultFacts injects shard execution results into the kernel.
// This enables the continuation protocol to detect follow-up work.
func (m *Model) injectShardResultFacts(shardType, task, result string, err error) {
	if m.kernel == nil {
		return
	}

	taskID := fmt.Sprintf("task_%d", time.Now().UnixNano())
	status := "/complete"
	if err != nil {
		status = "/failed"
	}

	// Determine result status based on content
	if strings.Contains(result, "TODO") || strings.Contains(result, "FIXME") {
		status = "/incomplete"
	}
	if shardType == "coder" && !strings.Contains(strings.ToLower(result), "test") {
		status = "/code_generated" // May need tests
	}

	// Assert shard_result fact
	resultFact := core.Fact{
		Predicate: "shard_result",
		Args: []interface{}{
			taskID,
			status,
			"/" + shardType,
			task,
			truncateSummary(result, 200),
		},
	}
	_ = m.kernel.Assert(resultFact)

	// For coder shards that generated code, assert pending test need
	if status == "/code_generated" {
		testFact := core.Fact{
			Predicate: "pending_test",
			Args: []interface{}{
				fmt.Sprintf("test_%d", time.Now().UnixNano()),
				fmt.Sprintf("Write tests for: %s", truncateSummary(task, 100)),
			},
		}
		_ = m.kernel.Assert(testFact)
	}

	// For reviewer shards that found issues, assert pending fix need
	if shardType == "reviewer" && strings.Contains(strings.ToLower(result), "issue") {
		reviewFact := core.Fact{
			Predicate: "pending_review",
			Args: []interface{}{
				fmt.Sprintf("review_%d", time.Now().UnixNano()),
				fmt.Sprintf("Fix issues found in: %s", truncateSummary(task, 100)),
			},
		}
		_ = m.kernel.Assert(reviewFact)
	}
}

// isMutationOperation returns true if the shard type performs mutations.
// Used by Breakpoint mode to pause before write/run operations.
func isMutationOperation(shardType string) bool {
	mutationShards := map[string]bool{
		"coder":            true,  // Writes code
		"tool_generator":   true,  // Creates tools
		"tester":           false, // Just runs tests (read-only)
		"reviewer":         false, // Just analyzes (read-only)
		"researcher":       false, // Just gathers info (read-only)
		"debugger":         true,  // May fix bugs
		"security_auditor": false, // Just analyzes
	}
	if isMutation, exists := mutationShards[shardType]; exists {
		return isMutation
	}
	// Default: assume mutation if unknown
	return true
}

// truncateSummary truncates a string for fact storage
func truncateSummary(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
