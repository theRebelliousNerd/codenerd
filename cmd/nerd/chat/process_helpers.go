package chat

import (
	"context"
	"fmt"
	"strings"

	"codenerd/internal/core"
	"codenerd/internal/perception"
)

func (m Model) clearStaleKernelFacts() {
	// CONTINUATION PROTOCOL: Clean up stale continuation facts from previous turns
	// This prevents old pending_test/pending_review facts from triggering false continuations
	_ = m.kernel.Retract("shard_result")
	_ = m.kernel.Retract("pending_test")
	_ = m.kernel.Retract("pending_review")
	_ = m.kernel.Retract("interrupt_requested")

	// STALE ACTION CLEANUP: Clear action pipeline facts from previous turns/sessions
	// These facts accumulate and cause misleading "System actions" displays for greetings
	// and other conversational intents that don't actually trigger shard work.
	_ = m.kernel.Retract("execution_result")
	_ = m.kernel.Retract("routing_result")
	_ = m.kernel.Retract("pending_action")
	_ = m.kernel.Retract("delegate_task")
	_ = m.kernel.Retract("trace_recall_result")
	_ = m.kernel.Retract("learning_recall_result")
	// Step 4: clear the prior turn's delegation decision input so stale
	// confidence cannot leak into this turn's should_delegate gate.
	_ = m.kernel.Retract("delegation_candidate")
	// Step 5: clear the prior turn's multi-step signals.
	_ = m.kernel.Retract("multi_step_signal")
	// Routing arbitration: clear the prior turn's perception signals so
	// a stale /is_question cannot flip this turn's route_decision.
	_ = m.kernel.Retract("intent_signal")

	// NERD-EVOLVE-START: P3_routing_assertion
	// Retract per-turn perception routing facts so stale values from the
	// previous turn cannot influence C1/C4 composition in this turn.
	// These are reasserted by LLMTransducer.assertRoutingFacts() during perception.
	_ = m.kernel.Retract("current_understanding")
	_ = m.kernel.Retract("llm_suggested_mode")
	_ = m.kernel.Retract("candidate_mode")
	_ = m.kernel.Retract("best_candidate_priority")
	_ = m.kernel.Retract("derived_mode")
	_ = m.kernel.Retract("derived_primary_shard")
	_ = m.kernel.Retract("derived_context_priority")
	_ = m.kernel.Retract("derived_tool_priority")
	// NERD-EVOLVE-END: P3_routing_assertion

}

func (m Model) processMemoryOperations(intent perception.Intent, warnings *[]string) {
	// 1.3.1 MEMORY OPERATIONS: Process promote_to_long_term, forget, etc.
	// This enables "remember X" and "learn that Y" instructions
	if len(intent.MemoryOperations) > 0 && m.localDB != nil {
		for _, memOp := range intent.MemoryOperations {
			switch memOp.Op {
			case "promote_to_long_term":
				if err := m.localDB.StoreFact(memOp.Key, []any{memOp.Value}, "learned", 10); err != nil {
					*warnings = append(*warnings, fmt.Sprintf("[Memory] failed to store: %v", err))
				}
			case "forget":
				if m.kernel != nil {
					m.kernel.Retract(memOp.Key)
				}
			}
		}
	}
}

func (m Model) executeInfoGatheringActions(ctx context.Context, input string, intent perception.Intent, actions []core.Fact, warnings *[]string) ([]core.Fact, []string) {
	// Execute Info-Gathering Actions (Pre-Articulation)
	// This implements the OODA "Act" phase for info retrieval
	var executionResults []core.Fact
	var mangleUpdates []string

	for _, action := range actions {
		actionName := nextActionName(action)
		if actionName != "" {
			mangleUpdates = append(mangleUpdates, actionName)
		}
		actionKey := normalizeActionType(actionName)

		// Handle File System Reads
		if actionKey == "fs_read" {
			target := intent.Target // Simple mapping for now
			if target != "" && target != "none" {
				content, err := readFileContent(m.workspace, target, 8000)
				if err == nil {
					// Feed result back to kernel
					resFact := core.Fact{
						Predicate: "file_content",
						Args:      []any{target, content},
					}
					executionResults = append(executionResults, resFact)
					// Also allow articulation to see it
					*warnings = append(*warnings, fmt.Sprintf("Read file: %s (%d bytes)", target, len(content)))
				} else {
					*warnings = append(*warnings, fmt.Sprintf("Failed to read file %s: %v", target, err))
				}
			}
		}

		// Handle Search
		if actionKey == "search_files" {
			matches, err := searchInFiles(m.workspace, intent.Target, 10)
			if err == nil {
				resFact := core.Fact{
					Predicate: "search_results",
					Args:      []any{intent.Target, strings.Join(matches, ",")},
				}
				executionResults = append(executionResults, resFact)
				*warnings = append(*warnings, fmt.Sprintf("Found %d matches for '%s'", len(matches), intent.Target))
			}
		}

		// Autopoiesis: Tool Generation (§8.3)
		if actionKey == "generate_tool" && m.autopoiesis != nil {
			// Detect tool need from the input
			toolNeed, detectErr := m.autopoiesis.DetectToolNeed(ctx, input)
			if detectErr == nil && toolNeed != nil {
				*warnings = append(*warnings, fmt.Sprintf("Tool need detected: %s (confidence: %.2f)", toolNeed.Name, toolNeed.Confidence))

				// Generate the tool if confidence is high enough
				if toolNeed.Confidence >= 0.6 {
					genTool, genErr := m.autopoiesis.GenerateTool(ctx, toolNeed)
					if genErr == nil && genTool != nil {
						*warnings = append(*warnings, fmt.Sprintf("Generated tool: %s", genTool.Name))
						if genTool.Validated {
							*warnings = append(*warnings, "Tool code validated successfully")
						} else if len(genTool.Errors) > 0 {
							*warnings = append(*warnings, fmt.Sprintf("Tool validation warnings: %v", genTool.Errors))
						}
					} else if genErr != nil {
						*warnings = append(*warnings, fmt.Sprintf("Tool generation failed: %v", genErr))
					}
				} else {
					*warnings = append(*warnings, "Tool need confidence too low for auto-generation")
				}
			} else {
				*warnings = append(*warnings, "Autopoiesis: Analyzing for missing tool capabilities...")
			}
		}
	}

	return executionResults, mangleUpdates
}
