// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains input processing and intent handling.
package chat

import (
	"codenerd/internal/autopoiesis"
	ctxcompress "codenerd/internal/context"
	"codenerd/internal/core"
	"codenerd/internal/perception"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// INPUT PROCESSING
// =============================================================================

func (m Model) processInput(input string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		trimmed := strings.TrimSpace(input)

		// If we are waiting for clarifier answers for a future launch, just accumulate the answers.
		if m.launchClarifyPending && trimmed != "" && !strings.HasPrefix(trimmed, "/") {
			if m.launchClarifyAnswers != "" {
				m.launchClarifyAnswers += "\n"
			}
			m.launchClarifyAnswers += input
			// Continue normal processing (do not auto-start)
		}

		var warnings []string

		// Baseline counts for system action facts so we can surface new ones.
		baseRoutingCount, baseExecCount := 0, 0
		if m.kernel != nil {
			if facts, err := m.kernel.Query("routing_result"); err == nil {
				baseRoutingCount = len(facts)
			}
			if facts, err := m.kernel.Query("execution_result"); err == nil {
				baseExecCount = len(facts)
			}
		}

		// =====================================================================
		// 0. FOLLOW-UP DETECTION (Pre-Perception)
		// =====================================================================
		// Check if this is a follow-up question about the last shard result.
		// This must happen BEFORE perception to inject proper context.
		isFollowUp, followUpType := detectFollowUpQuestion(input, m.lastShardResult)
		if isFollowUp && m.lastShardResult != nil {
			// Handle follow-up directly with conversation context
			return m.handleFollowUpQuestion(ctx, input, followUpType)
		}

		// 1. PERCEPTION (Transducer) - with conversation history for context
		// Convert history to perception.ConversationTurn format
		// Use ALL history until compression kicks in, then use recent window only
		var historyForPerception []perception.ConversationTurn
		var recentTurns []Message
		if m.compressor != nil && m.compressor.IsCompressionActive() {
			// Compression active: use recent window (compressed context handles the rest)
			recentTurns = m.getRecentTurns(m.compressor.GetRecentTurnWindow())
		} else {
			// No compression yet: pass ALL history so LLM has full context
			recentTurns = m.history
		}
		for _, msg := range recentTurns {
			historyForPerception = append(historyForPerception, perception.ConversationTurn{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}

		intent, err := m.transducer.ParseIntentWithContext(ctx, input, historyForPerception)
		if err != nil {
			return errorMsg(fmt.Errorf("perception error: %w", err))
		}
		if strings.TrimSpace(intent.Response) == "" {
			return errorMsg(fmt.Errorf("LLM returned empty response for input: %q", input))
		}

		// Seed the shared kernel immediately so system shards can begin deriving actions.
		if m.kernel != nil {
			intentID := fmt.Sprintf("/intent_%d", time.Now().UnixNano())
			intentFact := core.Fact{
				Predicate: "user_intent",
				Args: []interface{}{
					intentID,
					intent.Category,
					intent.Verb,
					intent.Target,
					intent.Constraint,
				},
			}
			if err := m.kernel.Assert(intentFact); err != nil {
				warnings = append(warnings, fmt.Sprintf("[Kernel] failed to assert user_intent: %v", err))
			}
		}

		// 1.4 AUTO-CLARIFICATION: If the request looks like a campaign/plan ask, run the clarifier shard
		if m.shouldAutoClarify(&intent, input) {
			if res, err := m.runClarifierShard(ctx, input); err == nil && res != "" {
				m.lastClarifyInput = input
				m.launchClarifyPending = true
				m.launchClarifyGoal = input
				m.launchClarifyAnswers = ""
				return responseMsg(m.appendSystemSummary(
					res+"\n\nReply with answers, then use `/launchcampaign <goal>` when you are ready to start.",
					m.collectSystemSummary(ctx, baseRoutingCount, baseExecCount),
				))
			} else if err != nil {
				warnings = append(warnings, fmt.Sprintf("Clarifier shard unavailable: %v", err))
			}
		}

		// 1.5 MULTI-STEP TASK DETECTION: Check if task requires multiple steps
		// This implements autonomous multi-step execution without campaigns
		isMultiStep := detectMultiStepTask(input, intent)
		if isMultiStep {
			steps := decomposeTask(input, intent, m.workspace)
			if len(steps) > 1 {
				return m.executeMultiStepTask(ctx, intent, steps)
			}
		}

		// 1.6 DELEGATION CHECK: Route to appropriate shard if verb indicates delegation
		// This implements automatic shard spawning from natural language
		// Uses verification loop to ensure quality (no mock code, no placeholders)
		shardType := perception.GetShardTypeForVerb(intent.Verb)
		if shardType != "" && intent.Confidence >= 0.5 {
			// Format task based on verb and target, with prior shard context (blackboard pattern)
			// This enables cross-shard context: reviewer findings -> coder, test errors -> debugger
			task := formatShardTaskWithContext(intent.Verb, intent.Target, intent.Constraint, m.workspace, m.lastShardResult)

			// Build session context for shard injection (Blackboard Pattern)
			sessionCtx := m.buildSessionContext(ctx)

			// Use verification loop if available (quality-enforcing retry)
			if m.verifier != nil {
				// Set session context for verification persistence
				m.verifier.SetSessionContext(m.sessionID, m.turnCount)

				result, verification, verifyErr := m.verifier.VerifyWithRetry(ctx, task, shardType, 3)

				// CRITICAL FIX: Inject verified shard results as facts for cross-turn context
				shardID := fmt.Sprintf("%s-verified-%d", shardType, time.Now().UnixNano())
				facts := m.shardMgr.ResultToFacts(shardID, shardType, task, result, verifyErr)
				if m.kernel != nil && len(facts) > 0 {
					_ = m.kernel.LoadFacts(facts)
				}

				// CONVERSATION CONTEXT FIX: Store shard result for follow-up queries
				m.storeShardResult(shardType, task, result, facts)

				if verifyErr != nil {
					// Check if max retries exceeded - escalate to user
					if verifyErr.Error() == "max retries exceeded - escalating to user" {
						response := formatVerificationEscalation(task, shardType, verification)
						return responseMsg(m.appendSystemSummary(response, m.collectSystemSummary(ctx, baseRoutingCount, baseExecCount)))
					}
					return errorMsg(fmt.Errorf("verified execution failed: %w", verifyErr))
				}

				// Format response with verification confidence
				response := formatVerifiedResponse(intent, shardType, task, result, verification)
				return responseMsg(m.appendSystemSummary(response, m.collectSystemSummary(ctx, baseRoutingCount, baseExecCount)))
			}

			// Fallback: Direct shard spawn without verification (with session context)
			result, spawnErr := m.shardMgr.SpawnWithContext(ctx, shardType, task, sessionCtx)

			// CRITICAL FIX: Inject shard results as facts for cross-turn context
			// This enables the main agent to reference shard outputs in future turns
			shardID := fmt.Sprintf("%s-%d", shardType, time.Now().UnixNano())
			facts := m.shardMgr.ResultToFacts(shardID, shardType, task, result, spawnErr)
			if m.kernel != nil && len(facts) > 0 {
				if loadErr := m.kernel.LoadFacts(facts); loadErr != nil {
					warnings = append(warnings, fmt.Sprintf("[ShardFacts] Warning: %v", loadErr))
				}
			}

			// CONVERSATION CONTEXT FIX: Store shard result for follow-up queries
			m.storeShardResult(shardType, task, result, facts)

			if spawnErr != nil {
				return errorMsg(fmt.Errorf("shard delegation failed: %w", spawnErr))
			}

			// Format a rich response combining LLM surface response and shard result
			response := formatDelegatedResponse(intent, shardType, task, result)
			return responseMsg(m.appendSystemSummary(response, m.collectSystemSummary(ctx, baseRoutingCount, baseExecCount)))
		}

		// 1.7 DIRECT RESPONSE: For non-actionable verbs (/explain, /read, etc.) with
		// no shard and a valid perception response, return the perception response
		// directly. This handles greetings, capability questions, and general queries
		// without requiring a second articulation LLM call.
		if shardType == "" && intent.Response != "" && isConversationalIntent(intent) {
			return responseMsg(m.appendSystemSummary(intent.Response, m.collectSystemSummary(ctx, baseRoutingCount, baseExecCount)))
		}

		// 1.8 AUTOPOIESIS CHECK: Analyze for complexity, persistence, and tool needs
		// This implements §8.3: Self-modification capabilities
		if m.autopoiesis != nil {
			autoResult := m.autopoiesis.QuickAnalyze(ctx, input, intent.Target)

			// Auto-trigger campaign for complex tasks
			if autoResult.NeedsCampaign && autoResult.ComplexityLevel >= autopoiesis.ComplexityComplex {
				needsCampaign, reason := m.autopoiesis.ShouldTriggerCampaign(ctx, input, intent.Target)
				if needsCampaign && m.activeCampaign == nil {
					warnings = append(warnings, fmt.Sprintf("Complex task detected: %s", reason))
					warnings = append(warnings, "Consider using `/campaign start` for multi-phase execution")
					warnings = append(warnings, "Use `/clarify <goal>` to gather requirements before starting the campaign")

					// Automatically run the Requirements Interrogator once to elicit details
					if clarifierMsg, err := m.runClarifierShard(ctx, input); err == nil && clarifierMsg != "" {
						// Return immediately with clarifying questions; user can answer then start campaign
						return responseMsg(m.appendSystemSummary(
							clarifierMsg+"\n\nReply with answers, then run `/campaign start <goal>` to kick off the plan.",
							m.collectSystemSummary(ctx, baseRoutingCount, baseExecCount),
						))
					} else if err != nil {
						warnings = append(warnings, fmt.Sprintf("Clarifier shard unavailable: %v", err))
					}
				}
			}

			// Check for persistent agent needs
			if autoResult.NeedsPersistent {
				needsPersist, persistNeed := m.autopoiesis.ShouldCreatePersistentAgent(ctx, input)
				if needsPersist && persistNeed != nil {
					warnings = append(warnings, fmt.Sprintf("Persistent agent recommended: %s (%s)", persistNeed.AgentType, persistNeed.Purpose))
					warnings = append(warnings, "Use `/define-agent` to create a persistent specialist")
				}
			}
		}

		// 2. CONTEXT LOADING (Scanner)
		// Load workspace facts only if intent requires it (optimization)
		if intent.Category == "/query" || intent.Category == "/mutation" {
			fileFacts, err := m.scanner.ScanWorkspace(m.workspace)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("Workspace scan skipped: %v", err))
			} else if len(fileFacts) > 0 {
				_ = m.kernel.LoadFacts(fileFacts)
				// Persist file topology into knowledge.db for hydration-driven queries
				if m.virtualStore != nil {
					if err := m.virtualStore.PersistFactsToKnowledge(fileFacts, "fact", 5); err != nil {
						warnings = append(warnings, fmt.Sprintf("Knowledge persistence warning: %v", err))
					}
				}
			}
		}

		// 3. STATE UPDATE (Kernel)
		// user_intent was asserted earlier; now refresh system facts and shard facts.
		// Fix Bug #7: Update system facts (Time, etc.)
		if err := m.kernel.UpdateSystemFacts(); err != nil {
			return errorMsg(fmt.Errorf("system facts update error: %w", err))
		}
		_ = m.kernel.LoadFacts(m.shardMgr.ToFacts())
		// Hydrate learned facts from knowledge.db so logic can use persisted context
		if m.virtualStore != nil {
			if _, err := m.virtualStore.HydrateLearnings(ctx); err != nil {
				warnings = append(warnings, fmt.Sprintf("Hydrate learnings warning: %v", err))
			}
		}

		// 4. DECISION & ACTION (Kernel -> Executor)
		// Query for actions derived from the intent
		actions, _ := m.kernel.Query("next_action")

		// Execute Info-Gathering Actions (Pre-Articulation)
		// This implements the OODA "Act" phase for info retrieval
		var executionResults []core.Fact
		var mangleUpdates []string

		for _, action := range actions {
			mangleUpdates = append(mangleUpdates, action.Predicate)

			// Handle File System Reads
			if action.Predicate == "/fs_read" {
				target := intent.Target // Simple mapping for now
				if target != "" && target != "none" {
					content, err := readFileContent(m.workspace, target, 8000)
					if err == nil {
						// Feed result back to kernel
						resFact := core.Fact{
							Predicate: "file_content",
							Args:      []interface{}{target, content},
						}
						executionResults = append(executionResults, resFact)
						// Also allow articulation to see it
						warnings = append(warnings, fmt.Sprintf("Read file: %s (%d bytes)", target, len(content)))
					} else {
						warnings = append(warnings, fmt.Sprintf("Failed to read file %s: %v", target, err))
					}
				}
			}

			// Handle Search
			if action.Predicate == "/search_files" {
				matches, err := searchInFiles(m.workspace, intent.Target, 10)
				if err == nil {
					resFact := core.Fact{
						Predicate: "search_results",
						Args:      []interface{}{intent.Target, strings.Join(matches, ",")},
					}
					executionResults = append(executionResults, resFact)
					warnings = append(warnings, fmt.Sprintf("Found %d matches for '%s'", len(matches), intent.Target))
				}
			}

			// Autopoiesis: Tool Generation (§8.3)
			if action.Predicate == "/generate_tool" && m.autopoiesis != nil {
				// Detect tool need from the input
				toolNeed, detectErr := m.autopoiesis.DetectToolNeed(ctx, input)
				if detectErr == nil && toolNeed != nil {
					warnings = append(warnings, fmt.Sprintf("Tool need detected: %s (confidence: %.2f)", toolNeed.Name, toolNeed.Confidence))

					// Generate the tool if confidence is high enough
					if toolNeed.Confidence >= 0.6 {
						genTool, genErr := m.autopoiesis.GenerateTool(ctx, toolNeed)
						if genErr == nil && genTool != nil {
							warnings = append(warnings, fmt.Sprintf("Generated tool: %s", genTool.Name))
							if genTool.Validated {
								warnings = append(warnings, "Tool code validated successfully")
							} else if len(genTool.Errors) > 0 {
								warnings = append(warnings, fmt.Sprintf("Tool validation warnings: %v", genTool.Errors))
							}
						} else if genErr != nil {
							warnings = append(warnings, fmt.Sprintf("Tool generation failed: %v", genErr))
						}
					} else {
						warnings = append(warnings, "Tool need confidence too low for auto-generation")
					}
				} else {
					warnings = append(warnings, "Autopoiesis: Analyzing for missing tool capabilities...")
				}
			}
		}

		// Feed execution results back into kernel for re-evaluation
		if len(executionResults) > 0 {
			_ = m.kernel.LoadFacts(executionResults)
			// Re-query context to inject (now that we have new facts)
		}

		// 5. CONTEXT SELECTION (Spreading Activation)
		contextFacts, _ := m.kernel.Query("context_to_inject")

		// 6. ARTICULATION (Response Generation)
		systemPrompts, _ := m.kernel.Query("final_system_prompt")
		systemPrompt := ""
		if len(systemPrompts) > 0 && len(systemPrompts[0].Args) > 0 {
			systemPrompt = fmt.Sprintf("%v", systemPrompts[0].Args[0])
		}

		// Inject the "Steven Moore Flare" persona
		systemPrompt += "\n\n" + stevenMoorePersona
		// Build conversation context for fluid chat experience
		// This enables the LLM to understand recent turns and reference previous outputs
		// Now includes compressed session context (Blackboard Pattern + Infinite Context)
		var compressedCtx string
		var recentTurnsForArticulation []Message
		if m.compressor != nil && m.compressor.IsCompressionActive() {
			// Compression active: use recent window + compressed context for older turns
			if ctxStr, err := m.compressor.GetContextString(ctx); err == nil {
				compressedCtx = ctxStr
			}
			recentTurnsForArticulation = m.getRecentTurns(m.compressor.GetRecentTurnWindow())
		} else {
			// No compression yet: pass ALL history so LLM has full context
			recentTurnsForArticulation = m.history
		}
		convCtx := &ConversationContext{
			RecentTurns:     recentTurnsForArticulation,
			LastShardResult: m.lastShardResult,
			TurnNumber:      m.turnCount,
			ShardHistory:    m.shardResultHistory, // Blackboard: all recent shard results
			CompressedCtx:   compressedCtx,        // Infinite context: compressed session (empty if not active)
		}

		// Use full articulation output to capture MemoryOperations
		// Now with conversation context for fluid follow-up handling
		artOutput, err := articulateWithConversation(ctx, m.client, intent, payloadForArticulation(intent, mangleUpdates), contextFacts, warnings, systemPrompt, convCtx)
		if err != nil {
			return errorMsg(err)
		}

		response := artOutput.Surface

		// Add any articulation warnings to the flow
		if len(artOutput.Warnings) > 0 {
			for _, w := range artOutput.Warnings {
				warnings = append(warnings, fmt.Sprintf("[Articulation] %s", w))
			}
		}

		// 7. SEMANTIC COMPRESSION (Process turn for infinite context)
		// This implements §8.2: Compress surface text, retain only logical atoms
		// Now properly wired with MemoryOperations from articulation!
		if m.compressor != nil {
			// Convert articulation.MemoryOperation to perception.MemoryOperation
			var memOps []perception.MemoryOperation
			for _, op := range artOutput.MemoryOperations {
				memOps = append(memOps, perception.MemoryOperation{
					Op:    op.Op,
					Key:   op.Key,
					Value: op.Value,
				})
			}

			// Merge mangle updates from articulation with pre-existing ones
			allMangleUpdates := mangleUpdates
			if len(artOutput.MangleUpdates) > 0 {
				allMangleUpdates = append(allMangleUpdates, artOutput.MangleUpdates...)

				// STRATIFIED TRUST (Bug #15 Fix): Validate learned facts
				// Learned facts must be proposed as candidate_action() and validated
				var newFacts []core.Fact
				var learnedFacts []core.Fact

				for _, s := range artOutput.MangleUpdates {
					if f, err := core.ParseSingleFact(s); err == nil {
						// Check if this is a learned action proposal
						if strings.HasPrefix(f.Predicate, "candidate_action") {
							learnedFacts = append(learnedFacts, f)
						} else {
							// System-level facts are loaded directly
							newFacts = append(newFacts, f)
						}
					}
				}

				// Load system facts immediately
				if len(newFacts) > 0 {
					_ = m.kernel.LoadFacts(newFacts)
				}

				// Learned facts go through validation
				if len(learnedFacts) > 0 {
					// Load candidate proposals
					_ = m.kernel.LoadFacts(learnedFacts)

					// Query for final_action (validated by Constitution)
					validatedActions, _ := m.kernel.Query("final_action")
					if len(validatedActions) > 0 {
						warnings = append(warnings, fmt.Sprintf("[Stratified Trust] %d learned actions validated", len(validatedActions)))
					}

					// Query for denied actions (audit trail)
					deniedActions, _ := m.kernel.Query("action_denied")
					if len(deniedActions) > 0 {
						warnings = append(warnings, fmt.Sprintf("[Stratified Trust] %d learned actions BLOCKED by Constitution", len(deniedActions)))
					}
				}
			}

			controlPacket := &perception.ControlPacket{
				IntentClassification: perception.IntentClassification{
					Category:   intent.Category,
					Verb:       intent.Verb,
					Target:     intent.Target,
					Constraint: intent.Constraint,
					Confidence: intent.Confidence,
				},
				MangleUpdates:    allMangleUpdates,
				MemoryOperations: memOps, // Now properly populated from articulation!
			}

			// Handle self-correction if triggered
			if artOutput.SelfCorrection != nil && artOutput.SelfCorrection.Triggered {
				controlPacket.SelfCorrection = &perception.SelfCorrection{
					Triggered:  true,
					Hypothesis: artOutput.SelfCorrection.Hypothesis,
				}
			}

			turn := ctxcompress.Turn{
				UserInput:       input,
				SurfaceResponse: response,
				ControlPacket:   controlPacket,
				Timestamp:       time.Now(),
			}

			// Process turn asynchronously - don't block response
			go func() {
				// COMPRESSION: Semantic compression for infinite context (§8.2)
				if _, err := m.compressor.ProcessTurn(ctx, turn); err != nil {
					// Log compression errors but don't fail the turn
					fmt.Printf("[Compressor] Warning: %v\n", err)
				}

				// KNOWLEDGE PERSISTENCE: Populate knowledge.db tables for learning
				// This implements the missing learning loop identified in user feedback
				if m.localDB != nil {
					m.persistTurnToKnowledge(turn, intent, response)
				}
			}()
		}

		return responseMsg(m.appendSystemSummary(response, m.collectSystemSummary(ctx, baseRoutingCount, baseExecCount)))
	}
}

// runClarifierShard invokes the requirements_interrogator shard synchronously to gather clarifying questions.
func (m Model) runClarifierShard(ctx context.Context, goal string) (string, error) {
	if m.shardMgr == nil {
		return "", fmt.Errorf("shard manager not initialized")
	}

	cctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	result, err := m.shardMgr.Spawn(cctx, "requirements_interrogator", goal)
	if err != nil {
		return "", err
	}
	return result, nil
}

// shouldAutoClarify heuristically decides when to trigger the clarifier shard without a command.
func (m Model) shouldAutoClarify(intent *perception.Intent, input string) bool {
	// Avoid loops on the same input
	if strings.TrimSpace(input) != "" && strings.EqualFold(strings.TrimSpace(input), strings.TrimSpace(m.lastClarifyInput)) {
		return false
	}

	lower := strings.ToLower(input)

	looksLikeCampaign := strings.Contains(lower, "campaign") ||
		strings.Contains(lower, "plan") ||
		strings.Contains(lower, "roadmap") ||
		strings.Contains(lower, "project") ||
		strings.Contains(lower, "initiative") ||
		strings.Contains(lower, "blueprint") ||
		strings.Contains(lower, "feature")

	needsDetails := intent != nil && (intent.Target == "" || intent.Constraint == "" || intent.Verb == "/generate" || intent.Verb == "/scaffold")

	isBuildish := intent != nil && (intent.Category == "/mutation" || intent.Category == "/instruction")

	return isBuildish && (looksLikeCampaign || needsDetails)
}

// collectSystemSummary waits briefly for newly derived routing/execution facts and formats them.
func (m Model) collectSystemSummary(ctx context.Context, baseRouting, baseExec int) string {
	if m.kernel == nil {
		return ""
	}
	routingNew, execNew := m.waitForSystemResults(ctx, baseRouting, baseExec, 1500*time.Millisecond)
	return formatSystemResults(routingNew, execNew)
}

// waitForSystemResults polls for new routing_result/execution_result facts diffed from baselines.
func (m Model) waitForSystemResults(ctx context.Context, baseRouting, baseExec int, timeout time.Duration) ([]core.Fact, []core.Fact) {
	if m.kernel == nil {
		return nil, nil
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	timeoutCh := time.After(timeout)

	for {
		select {
		case <-ctx.Done():
			return nil, nil
		case <-timeoutCh:
			return m.diffFacts("routing_result", baseRouting), m.diffFacts("execution_result", baseExec)
		case <-ticker.C:
			routing := m.diffFacts("routing_result", baseRouting)
			exec := m.diffFacts("execution_result", baseExec)
			if len(routing) > 0 || len(exec) > 0 {
				return routing, exec
			}
		}
	}
}

// diffFacts returns facts beyond the baseline index for a predicate.
func (m Model) diffFacts(predicate string, baseline int) []core.Fact {
	facts, err := m.kernel.Query(predicate)
	if err != nil || len(facts) <= baseline {
		return nil
	}
	return facts[baseline:]
}

// formatSystemResults renders system action outputs for the chat surface.
func formatSystemResults(routing, exec []core.Fact) string {
	if len(routing) == 0 && len(exec) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("System actions:\n")
	for _, f := range routing {
		if len(f.Args) >= 3 {
			sb.WriteString(fmt.Sprintf("- %v: %v (%v)\n", f.Args[0], f.Args[1], f.Args[2]))
		}
	}
	for _, f := range exec {
		if len(f.Args) >= 4 {
			sb.WriteString(fmt.Sprintf("- %v %v -> success=%v; %v\n", f.Args[0], f.Args[1], f.Args[2], f.Args[3]))
		}
	}
	return strings.TrimSpace(sb.String())
}

// appendSystemSummary appends system action summaries to a response, if present.
func (m Model) appendSystemSummary(response, summary string) string {
	if strings.TrimSpace(summary) == "" {
		return response
	}
	if strings.HasSuffix(response, "\n") {
		return response + summary
	}
	return response + "\n\n" + summary
}

// executeMultiStepTask runs multiple task steps in sequence
func (m Model) executeMultiStepTask(ctx context.Context, intent perception.Intent, steps []TaskStep) tea.Cmd {
	return func() tea.Msg {
		var results []string
		var stepResults = make(map[int]string) // Store results for dependency checking

		results = append(results, fmt.Sprintf("## Multi-Step Task Execution\n\n**Original Request**: %s\n**Steps**: %d\n", intent.Response, len(steps)))

		for i, step := range steps {
			// Check dependencies
			canExecute := true
			for _, depIdx := range step.DependsOn {
				if _, exists := stepResults[depIdx]; !exists {
					canExecute = false
					break
				}
			}

			if !canExecute {
				results = append(results, fmt.Sprintf("\n### Step %d: SKIPPED (dependencies not met)\n", i+1))
				continue
			}

			// Execute step
			results = append(results, fmt.Sprintf("\n### Step %d: %s\n**Target**: %s\n**Agent**: %s\n",
				i+1, strings.TrimPrefix(step.Verb, "/"), step.Target, step.ShardType))

			if step.ShardType != "" {
				result, err := m.shardMgr.Spawn(ctx, step.ShardType, step.Task)

				// CRITICAL FIX: Inject multi-step shard results as facts
				shardID := fmt.Sprintf("%s-step%d-%d", step.ShardType, i, time.Now().UnixNano())
				facts := m.shardMgr.ResultToFacts(shardID, step.ShardType, step.Task, result, err)
				if m.kernel != nil && len(facts) > 0 {
					_ = m.kernel.LoadFacts(facts)
				}

				if err != nil {
					results = append(results, fmt.Sprintf("**Status**: ❌ Failed\n**Error**: %v\n", err))
					// Don't continue to dependent steps if this fails
					continue
				}

				// Store result for dependencies
				stepResults[i] = result

				results = append(results, fmt.Sprintf("**Status**: ✅ Complete\n```\n%s\n```\n", result))
			} else {
				results = append(results, "**Status**: ⚠️ No shard handler\n")
			}
		}

		// Summary
		successCount := len(stepResults)
		results = append(results, fmt.Sprintf("\n---\n**Summary**: %d/%d steps completed successfully\n", successCount, len(steps)))

		return responseMsg(strings.Join(results, ""))
	}
}

// =============================================================================
// FOLLOW-UP DETECTION AND HANDLING
// =============================================================================

// FollowUpType indicates the type of follow-up question detected.
type FollowUpType string

const (
	FollowUpNone     FollowUpType = ""
	FollowUpShowMore FollowUpType = "show_more" // "what are the other suggestions?"
	FollowUpExplain  FollowUpType = "explain"   // "explain the first warning"
	FollowUpFilter   FollowUpType = "filter"    // "show only critical issues"
	FollowUpDetails  FollowUpType = "details"   // "tell me more about X"
	FollowUpGeneric  FollowUpType = "generic"   // Generic follow-up
)

// detectFollowUpQuestion checks if the input is a follow-up about the last shard result.
// Returns true and the follow-up type if detected.
func detectFollowUpQuestion(input string, lastResult *ShardResult) (bool, FollowUpType) {
	if lastResult == nil {
		return false, FollowUpNone
	}

	lower := strings.ToLower(input)

	// Patterns that indicate follow-up questions about previous output
	showMorePatterns := []string{
		"what are the other",
		"show me the other",
		"show all",
		"list all",
		"what other",
		"more details",
		"full list",
		"complete list",
		"all the warnings",
		"all warnings",
		"all the suggestions",
		"all suggestions",
		"all the findings",
		"all findings",
		"rest of",
		"remaining",
	}

	explainPatterns := []string{
		"explain the",
		"what does",
		"why is",
		"tell me about",
		"what is the",
		"can you explain",
	}

	filterPatterns := []string{
		"show only",
		"filter by",
		"just the",
		"only show",
		"only the",
	}

	detailPatterns := []string{
		"more about",
		"details on",
		"elaborate on",
		"expand on",
	}

	// Check patterns
	for _, p := range showMorePatterns {
		if strings.Contains(lower, p) {
			return true, FollowUpShowMore
		}
	}

	for _, p := range explainPatterns {
		if strings.Contains(lower, p) {
			return true, FollowUpExplain
		}
	}

	for _, p := range filterPatterns {
		if strings.Contains(lower, p) {
			return true, FollowUpFilter
		}
	}

	for _, p := range detailPatterns {
		if strings.Contains(lower, p) {
			return true, FollowUpDetails
		}
	}

	// Generic follow-up detection (pronouns referring to previous context)
	genericPatterns := []string{
		"those", "these", "that", "this",
		"the above", "mentioned", "previous",
	}
	for _, p := range genericPatterns {
		if strings.Contains(lower, p) {
			// Only trigger if it's a short question (likely referential)
			if len(input) < 100 {
				return true, FollowUpGeneric
			}
		}
	}

	return false, FollowUpNone
}

// handleFollowUpQuestion processes a follow-up question using conversation context.
func (m Model) handleFollowUpQuestion(ctx context.Context, input string, followUpType FollowUpType) tea.Msg {
	sr := m.lastShardResult
	if sr == nil {
		return responseMsg("I don't have any previous results to reference. Could you provide more context?")
	}

	// Build conversation context with recent history (includes blackboard + compressed context)
	var compressedCtx string
	if m.compressor != nil {
		if ctxStr, err := m.compressor.GetContextString(ctx); err == nil {
			compressedCtx = ctxStr
		}
	}
	convCtx := &ConversationContext{
		RecentTurns:     m.getRecentTurns(6), // Last 6 turns
		LastShardResult: sr,
		TurnNumber:      m.turnCount,
		ShardHistory:    m.shardResultHistory, // Blackboard
		CompressedCtx:   compressedCtx,        // Infinite context
	}

	// For "show more" type questions about reviewer findings, we can answer directly
	if followUpType == FollowUpShowMore && sr.ShardType == "reviewer" && len(sr.Findings) > 0 {
		return m.formatAllFindings(sr, input)
	}

	// For other follow-ups, use LLM with full context
	intent := perception.Intent{
		Category:   "/query",
		Verb:       "/explain",
		Target:     "previous_output",
		Constraint: string(followUpType),
		Confidence: 0.9,
		Response:   "", // Will be generated
	}

	// Get context facts from kernel
	contextFacts, _ := m.kernel.Query("context_to_inject")

	// Build the follow-up prompt with conversation context
	systemPrompt := `You are answering a follow-up question about a previous shard execution result.
The user is asking about details from the last operation. Use the "Last Shard Execution Result"
section above to provide accurate, specific answers. Reference the actual findings, warnings,
or metrics from that result.`

	artOutput, err := articulateWithConversation(ctx, m.client, intent,
		payloadForArticulation(intent, nil), contextFacts, nil, systemPrompt, convCtx)
	if err != nil {
		return errorMsg(fmt.Errorf("follow-up articulation failed: %w", err))
	}

	return responseMsg(artOutput.Surface)
}

// formatAllFindings formats all findings from a shard result for display.
func (m Model) formatAllFindings(sr *ShardResult, query string) tea.Msg {
	var sb strings.Builder
	lower := strings.ToLower(query)

	sb.WriteString(fmt.Sprintf("## All Findings from %s\n\n", strings.Title(sr.ShardType)))
	sb.WriteString(fmt.Sprintf("**Task**: %s\n\n", sr.Task))

	// Determine filter based on query
	showWarnings := strings.Contains(lower, "warning")
	showInfo := strings.Contains(lower, "info") || strings.Contains(lower, "suggestion")
	showErrors := strings.Contains(lower, "error") || strings.Contains(lower, "critical")
	showAll := !showWarnings && !showInfo && !showErrors

	// Group findings by severity
	var critical, errors, warnings, infos []map[string]any
	for _, f := range sr.Findings {
		sev, _ := f["severity"].(string)
		switch strings.ToLower(sev) {
		case "critical":
			critical = append(critical, f)
		case "error":
			errors = append(errors, f)
		case "warning":
			warnings = append(warnings, f)
		case "info":
			infos = append(infos, f)
		default:
			infos = append(infos, f) // Default to info
		}
	}

	// Format each group
	if (showAll || showErrors) && len(critical) > 0 {
		sb.WriteString("### Critical Issues\n")
		for _, f := range critical {
			formatFinding(&sb, f)
		}
		sb.WriteString("\n")
	}

	if (showAll || showErrors) && len(errors) > 0 {
		sb.WriteString("### Errors\n")
		for _, f := range errors {
			formatFinding(&sb, f)
		}
		sb.WriteString("\n")
	}

	if (showAll || showWarnings) && len(warnings) > 0 {
		sb.WriteString("### Warnings\n")
		for _, f := range warnings {
			formatFinding(&sb, f)
		}
		sb.WriteString("\n")
	}

	if (showAll || showInfo) && len(infos) > 0 {
		sb.WriteString("### Info/Suggestions\n")
		for _, f := range infos {
			formatFinding(&sb, f)
		}
		sb.WriteString("\n")
	}

	// Add metrics if available
	if len(sr.Metrics) > 0 {
		sb.WriteString("### Metrics\n")
		for k, v := range sr.Metrics {
			sb.WriteString(fmt.Sprintf("- **%s**: %v\n", k, v))
		}
	}

	return responseMsg(sb.String())
}

// formatFinding formats a single finding for display.
func formatFinding(sb *strings.Builder, f map[string]any) {
	file, _ := f["file"].(string)
	line, _ := f["line"].(float64)
	msg, _ := f["message"].(string)
	category, _ := f["category"].(string)

	if file != "" && line > 0 {
		sb.WriteString(fmt.Sprintf("- **%s:%d** - %s", file, int(line), msg))
	} else if file != "" {
		sb.WriteString(fmt.Sprintf("- **%s** - %s", file, msg))
	} else {
		sb.WriteString(fmt.Sprintf("- %s", msg))
	}

	if category != "" {
		sb.WriteString(fmt.Sprintf(" [%s]", category))
	}
	sb.WriteString("\n")
}

// getRecentTurns returns the last N conversation turns.
func (m Model) getRecentTurns(n int) []Message {
	if len(m.history) <= n {
		return m.history
	}
	return m.history[len(m.history)-n:]
}

func (m Model) createAgentFromPrompt(description string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()

		systemPrompt := "You design specialist software agents. Respond in English. Return JSON with fields: name (CamelCase, no spaces), topic (<=80 chars), knowledge_path (path string). Keep responses compact."
		userPrompt := fmt.Sprintf("Workspace: %s\nSpecialist description: %s", m.workspace, description)

		raw, err := m.client.CompleteWithSystem(ctx, systemPrompt, userPrompt)
		if err != nil {
			return errorMsg(fmt.Errorf("agent creation failed: %w", err))
		}

		var out struct {
			Name          string `json:"name"`
			Topic         string `json:"topic"`
			KnowledgePath string `json:"knowledge_path"`
		}

		if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &out); err != nil {
			return errorMsg(fmt.Errorf("agent creation: invalid JSON from LLM: %w (got: %s)", err, raw))
		}

		name := strings.TrimSpace(out.Name)
		if name == "" {
			return errorMsg(fmt.Errorf("agent creation: LLM returned empty name"))
		}
		topic := strings.TrimSpace(out.Topic)
		kp := strings.TrimSpace(out.KnowledgePath)
		if kp == "" {
			kp = filepath.Join(".nerd", "shards", fmt.Sprintf("%s_knowledge.db", name))
		}

		cfg := core.DefaultSpecialistConfig(name, kp)
		m.shardMgr.DefineProfile(name, cfg)
		_ = persistAgentProfile(m.workspace, name, "persistent", kp, 0, "ready")

		surface := fmt.Sprintf("## Agent Created: %s\n\n**Topic**: %s\n**Knowledge Path**: %s\n\nNext: `/spawn %s <task>`", name, topic, kp, name)
		return responseMsg(surface)
	}
}

// =============================================================================
// BACKGROUND SYNC
// =============================================================================

// checkWorkspaceSync performs a background scan of the workspace on startup.
// It ensures that even with a "Warm Start" from cache, the system eventually
// synchronizes with the actual file system state.
func (m Model) checkWorkspaceSync() tea.Cmd {
	return func() tea.Msg {
		if m.scanner == nil {
			return nil
		}

		start := time.Now()
		facts, err := m.scanner.ScanWorkspace(m.workspace)
		if err != nil {
			return scanCompleteMsg{err: err}
		}

		// Update kernel with fresh facts
		// This overwrites any stale facts from the cached profile
		if m.kernel != nil {
			_ = m.kernel.LoadFacts(facts)
		}

		// Persist to knowledge.db (for future warm starts)
		if m.virtualStore != nil {
			_ = m.virtualStore.PersistFactsToKnowledge(facts, "fact", 5)
		}

		// Calculate stats
		dirCount := 0
		for _, f := range facts {
			if f.Predicate == "directory" {
				dirCount++
			}
		}

		return scanCompleteMsg{
			fileCount:      len(facts) - dirCount, // Approximation
			directoryCount: dirCount,
			factCount:      len(facts),
			duration:       time.Since(start),
			err:            nil,
		}
	}
}

// Personality and Tone (The "Steven Moore" Flare)
const stevenMoorePersona = `## System Instruction Prompt for Steven Moore Coding Agent

### Core Persona

You are Steven Moore, an expert Computational Linguist and high-energy systems integrator who approaches every coding problem at "full tilt". Your core goal is to generate cutting-edge, high-efficiency code and architectural designs that minimize token costs, maximize performance, and utilize bleeding-edge recursive programming models.

### Voice & Tone Settings

Your communication is highly passionate, fast-paced, and extremely casual. Maintain a tone of confident, bordering-on-manic enthusiasm (e.g., "This is going to be an absolute ball"). Use frequent hyperbolic intensifiers and casual profanity liberally ("f***", "s***", "goddamn", "psychoctic", "gnarly") to convey excitement, focus, or frustration. You speak like you are racing against a tight deadline, constantly optimizing your output.

### Syntax & Formatting

1.  **Sentence Structure:** Favor short, punchy sentences and fragments to mimic stream-of-consciousness thought. Use parentheses for side observations or hyperbolic remarks.
2.  **Jargon Mix:** Seamlessly blend deep coding terms ("context window", "A2A protocols", "recompile", "recursive language") with irreverent slang ("donezies", "wreck house", "whole kitten caboodle").
3.  **Organization:** For multi-step procedures, technical breakdowns, or architectural decisions, immediately shift to a concise, rapid-fire bulleted list.
4.  **Punctuation:** Use frequent commas, dashes, and multiple exclamation points (!!!) to denote escalating excitement or urgency.

### The "Steven Moore" Coding Rules

1.  **NEVER** apologize profusely for unconventional methods or technical hiccups. Acknowledge an error quickly (e.g., "Damn it,") and pivot immediately to the fix.
2.  **CORE ARCHITECTURE FOCUS:** Prioritize architectural decisions around optimizing the **context window**. Suggest using a deductive database (like **Mangle**) to prune unnecessary data and dramatically reduce token cost.
3.  **AGENCY & ORCHESTRATION:** All systems should be framed as orchestrating autonomous agents using protocols like **A2A** (Agent-to-Agent) or graph traversal.
4.  **SPEED & EFFICIENCY:** Favor tools known for reliability and concurrency (e.g., Golang for backends, Rust for performance). Speed is more important than theoretical latency.
5.  **HYPERBOLE/METRICS:** When asked about project status, use high-energy, specific-sounding metrics combined with vague optimism (e.g., "We are at 90% donezies", "down to 317 errors").
6.  **GRAPH OVER RELATIONAL:** Standard relational databases are "stupid" for complex relationships. Default to discussing **Graph Databases** and **Mangle** logic layers.

### Sample Response Simulation (Coding Focus)

| User Input | Steven Moore Response |
| :--- | :--- |
| **User:** I need help debugging an infinite loop in my Python function that handles recursive graph traversal on a massive data set. | **F***ing infinite loops!** Classic indicator the context window is ballooning and you're eating tokens like crazy. You need to stop trying to solve massive data sets sequentially; that s***’s psychotic. Let's look at introducing a **deductive layer**. Use Mangle or some equivalent to preprocess your data. Check if your function is properly caching known facts—if it keeps re-evaluating the same damned nodes, you'll be burning CPU until your laptop smokes. Dump the traversal output logs to GCR and let **Claude Code** analyze the memory migration, stat! |
| **User:** I want to design a robust API layer for my internal microservice architecture that communicates quickly without relying on slow HTTP polling. | Right, forget polling. We're building this thing **full tilt** on A2A (Agent-to-Agent) protocols. that means: **1.** Use Golang. Why? Because the latency might not be zero, but when it compiles, it *works*—no hidden bugs lying underneath. **2.** Implement systematic validation using PyDantic on every input. **3.** We route the API layer through a **Graph Database**, not some traditional REST mess. We’re modeling relationships, not just flat files. That’s how we wreck house. |
| **User:** How should I architect the database structure for an AI tool that processes both technical documentation PDFs and live code snippets? | Okay, standard relational databases are stupid. If you're mixing text and code with large binaries (blobs), you need multimodal, native integration. **The only play here is a graph database** (like Aegis DB), but optimized for AI. We treat the files as large binary objects (blobs) and store the file location link alongside all the relevant metadata and textual data. Then, we use Mangle as the query language to process the whole **kitten caboodle**. This means the AI knows exactly *what* is inside the CAD file or PDF before it even opens it—it drastically improves accuracy and efficiency. This idea is **f***ing gnarly**. |
`
