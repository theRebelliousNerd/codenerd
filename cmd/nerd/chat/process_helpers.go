package chat

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"codenerd/internal/autopoiesis"
	memorycontext "codenerd/internal/context"
	"codenerd/internal/core"
	"codenerd/internal/perception"
	shardsystem "codenerd/internal/shards/system"
	"codenerd/internal/types"
)

func (m Model) clearStaleKernelFacts() {
	// CONTINUATION PROTOCOL: Clean up stale continuation facts from previous turns
	// This prevents old pending_test/pending_review facts from triggering false continuations
	_ = m.kernel.Retract("shard_result")
	_ = m.kernel.Retract("pending_test")
	_ = m.kernel.Retract("pending_fix")
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

	// The perception routing facts are retired BEFORE perception runs, by
	// clearStaleRoutingFacts: clearing them here, after perception, retracted
	// the ones the transducer had just published for this turn.

}

// clearStaleRoutingFacts retires the previous turn's perception routing facts.
// It runs before perception, never after: LLMTransducer.assertRoutingFacts
// publishes current_understanding, the suggested and derived mode, the primary
// shard and the context and tool priorities for THIS turn during perception, so
// a clear that followed it (clearStaleKernelFacts did, until 2026-10-28)
// removed exactly what the transducer had just asserted. Retiring first means
// a turn whose perception publishes nothing still cannot inherit last turn's.
func (m Model) clearStaleRoutingFacts() {
	if m.kernel == nil {
		return
	}
	for _, pred := range []string{
		"current_understanding", "llm_suggested_mode", "candidate_mode",
		"best_candidate_priority", "derived_mode", "derived_primary_shard",
		"derived_context_priority", "derived_tool_priority",
	} {
		_ = m.kernel.Retract(pred)
	}
}

func (m Model) processMemoryOperations(intent perception.Intent, warnings *[]string) {
	// 1.3.1 MEMORY OPERATIONS: the person's "remember X" and "forget Y". They
	// go through the same scoped implementation a control packet's operations
	// do, so a remembered preference is stored as the type hydration loads and
	// a forget names the memory it addressed. A person is told what became of
	// each: a failure by its notice, a forget by its receipt.
	for i, d := range memorycontext.ApplyMemoryOperations(m.memoryKernel(), m.memoryStore(), intent.MemoryOperations) {
		if notice := d.Notice(); notice != "" {
			*warnings = append(*warnings, notice)
		} else if intent.MemoryOperations[i].Op == "forget" && d.Receipt != "" {
			*warnings = append(*warnings, "[Memory] "+d.Receipt)
		}
	}
}

func (m Model) executeInfoGatheringActions(ctx context.Context, input string, intent perception.Intent, actions []core.Fact, warnings *[]string) ([]core.Fact, []string, []evidenceObservation) {
	// Execute Info-Gathering Actions (Pre-Articulation)
	// This implements the OODA "Act" phase for info retrieval
	var executionResults []core.Fact
	var mangleUpdates []string
	var evidence []evidenceObservation

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
				obs := readEvidenceFile(m.workspace, target, evidenceReadBudget())
				evidence = append(evidence, obs)
				switch obs.Coverage {
				case coverageWhole:
					// Only a whole file is the kernel's file_content: a prefix
					// stored as the file would make every later reader of the
					// fact (has_file_content, edit preconditions) a prefix too.
					executionResults = append(executionResults, core.Fact{
						Predicate: "file_content",
						Args:      []any{target, obs.Body},
					})
					*warnings = append(*warnings, fmt.Sprintf("Read file: %s (%d bytes)", target, obs.SourceBytes))
				case coverageSpan:
					*warnings = append(*warnings, fmt.Sprintf("Read file: %s (only the first %d of %d bytes were read; the rest was not)", target, obs.ReturnedBytes, obs.SourceBytes))
				default:
					*warnings = append(*warnings, fmt.Sprintf("Failed to read file %s: %s", target, obs.Diagnostic))
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
			} else if errors.Is(detectErr, autopoiesis.ErrGenerationRestricted) {
				// The request restricts changes; generating a tool writes and
				// compiles source, so it is not done for this request.
				*warnings = append(*warnings, detectErr.Error())
			} else {
				*warnings = append(*warnings, "Autopoiesis: Analyzing for missing tool capabilities...")
			}
		}
	}

	return executionResults, mangleUpdates, evidence
}

// assertDirectIntent publishes a turn's intent when no PerceptionFirewall did:
// a fallback transducer read it, or an approved proposal reuses its held
// intent. The previous turn's perception facts go with its intent first: an
// approved run reaches here without the firewall, and the "go" turn's
// intent_unmapped for "approve" otherwise routed the approved run to clarify.
// It returns the warnings for the turn.
func (m Model) assertDirectIntent(intent perception.Intent) []string {
	var warnings []string
	// A stable ID, so the kernel does not accumulate historical intents.
	intentID := "/current_intent"
	tx := types.NewKernelTx(m.kernel)
	shardsystem.RetractPerceptionEphemera(tx, intentID)
	if err := tx.Commit(); err != nil {
		warnings = append(warnings, fmt.Sprintf("[Kernel] failed to clear the previous turn's perception facts: %v", err))
	}
	intentFact := core.Fact{
		Predicate: "user_intent",
		Args:      []any{types.MangleAtom(intentID), types.MangleAtom(intent.Category), types.MangleAtom(intent.Verb), intent.Target, intent.Constraint},
	}
	if err := m.kernel.Assert(intentFact); err != nil {
		warnings = append(warnings, fmt.Sprintf("[Kernel] failed to assert user_intent: %v", err))
	}
	_ = m.kernel.Assert(core.Fact{Predicate: "processed_intent", Args: []any{types.MangleAtom(intentID)}})
	// The firewall publishes what happened when a model read this turn; this
	// path does it here, so both paths give the kernel the same outcome
	// (policy/perception_outcome.mg).
	if err := perception.PublishClassificationOutcome(m.kernel, intentID, intent.Outcome); err != nil {
		warnings = append(warnings, fmt.Sprintf("[Kernel] failed to publish the classification outcome: %v", err))
	}
	return warnings
}

// detachFromTurn is the context for work that outlives the turn that started
// it (the streamed reply): it keeps the turn's values (usage tracker, session
// id, the chat's session-log scope and turn, the turn's restriction) and drops
// only the turn's cancel, while shutdown still cancels it. A fresh context lost
// every value, and the main agent's streamed replies filed under
// unattributed/. release cancels it and detaches it from shutdown.
func detachFromTurn(turnCtx, shutdownCtx context.Context) (ctx context.Context, release func()) {
	ctx, cancel := context.WithCancel(context.WithoutCancel(turnCtx))
	stop := func() bool { return false }
	if shutdownCtx != nil {
		stop = context.AfterFunc(shutdownCtx, cancel)
	}
	return ctx, func() {
		stop()
		cancel()
	}
}
