// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains input processing and intent handling.
//
// # File Index
//
// Input Processing:
//   - process.go            - Main processInput(), processInputWithKnowledge(), seed functions
//
// Dream State:
//   - process_dream.go      - handleDreamState(), formatDreamStateResponse(), system delegation
//
// Follow-Up Detection:
//   - process_follow_up.go  - detectFollowUpQuestion(), handleFollowUpQuestion(), formatFinding()
//
// Continuation Protocol:
//   - process_continuation.go - checkContinuation(), executeSubtask(), isMutationOperation()
//
// Knowledge Handling:
//   - process_knowledge.go  - handleKnowledgeRequests(), matchSpecialistForQuery()
//
// Background Sync:
//   - process_sync.go       - checkWorkspaceSync()
package chat

import (
	"codenerd/internal/autopoiesis"
	"codenerd/internal/campaign"
	"codenerd/internal/config"
	ctxcompress "codenerd/internal/context"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/retrieval"
	"codenerd/internal/transparency"
	"codenerd/internal/types"
	"codenerd/internal/usage"
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// INPUT PROCESSING
// =============================================================================

func (m Model) processInput(input string) tea.Cmd {
	return func() tea.Msg {
		// Panic recovery: this closure runs as a fire-and-forget goroutine in the
		// Bubbletea runtime. An unrecovered panic here kills the entire process.
		defer func() {
			if r := recover(); r != nil {
				logging.API("PANIC in processInput (recovered): %v", r)
			}
		}()

		// Guard: ensure transducer is initialized
		if m.transducer == nil {
			return errorMsg(fmt.Errorf("system not ready: transducer not initialized (boot may still be in progress)"))
		}
		if m.client == nil {
			return errorMsg(fmt.Errorf("system not ready: LLM client not initialized"))
		}

		// Use shutdownCtx as parent so Ctrl+X cancels in-flight LLM calls cleanly
		baseCtx := m.shutdownCtx
		if baseCtx == nil {
			baseCtx = context.Background()
		}
		ctx, cancel := context.WithTimeout(baseCtx, config.GetLLMTimeouts().OODALoopTimeout)
		if m.usageTracker != nil {
			ctx = usage.NewContext(ctx, m.usageTracker)
		}
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
		workspaceScanned := false

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
		m.ReportStatus("Perception: parsing intent...")

		// Disable boot guards on first user interaction.
		// This signals that the system is ready for normal operation and allows
		// action execution (prevents startup action spam from session rehydration).
		if m.shardMgr != nil {
			m.shardMgr.DisableExecutiveBootGuard()
		}
		if m.virtualStore != nil {
			m.virtualStore.DisableBootGuard()
		}

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

		intentHandledBySystem := false
		var intent perception.Intent
		var err error

		// Prefer the always-on PerceptionFirewall system shard when available.
		// This ensures Perception emits canonical facts (user_intent, processed_intent,
		// focus_resolution, ambiguity_flag) into the shared kernel and uses JIT prompts.
		if m.shardMgr != nil {
			if shard, ok := m.shardMgr.GetRunningShardByConfigName("perception_firewall"); ok {
				type perceiver interface {
					Perceive(ctx context.Context, input string, history []perception.ConversationTurn) (perception.Intent, error)
				}
				if pf, ok := shard.(perceiver); ok {
					intent, err = pf.Perceive(ctx, input, historyForPerception)
					intentHandledBySystem = err == nil
				}
			}
		}

		// Fallback: direct transducer parsing (legacy / when system shards are disabled).
		if !intentHandledBySystem {
			intent, err = m.transducer.ParseIntentWithContext(ctx, input, historyForPerception)
		}
		if err != nil {
			return errorMsg(fmt.Errorf("perception error: %w", err))
		}
		if strings.TrimSpace(intent.Response) == "" {
			warnings = append(warnings, "Perception response was empty; falling back to articulation")
		}
		m.ReportStatus(fmt.Sprintf("Orient: %s", intent.Verb))

		// Glass Box: Emit perception event
		if m.glassBoxEventBus != nil && m.glassBoxEnabled {
			summary := fmt.Sprintf("Intent: %s%s → %s (%.0f%%)",
				intent.Category, intent.Verb, truncateSummary(intent.Target, 40), intent.Confidence*100)
			details := fmt.Sprintf("Category: %s\nVerb: %s\nTarget: %s\nConstraint: %s",
				intent.Category, intent.Verb, intent.Target, intent.Constraint)
			m.glassBoxEventBus.Emit(transparency.GlassBoxEvent{
				Timestamp: time.Now(),
				Category:  transparency.CategoryPerception,
				Summary:   summary,
				Details:   details,
				TurnID:    m.turnCount,
			})
		}

		// Seed the shared kernel immediately so system shards can begin deriving actions.
		if m.kernel != nil {
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

			// Only assert user_intent ourselves if the PerceptionFirewall shard didn't already do it.
			if !intentHandledBySystem {
				// Use a stable ID so the kernel doesn't accumulate historical intents.
				intentID := "/current_intent"
				_ = m.kernel.RetractFact(core.Fact{Predicate: "user_intent", Args: []interface{}{intentID}})
				_ = m.kernel.RetractFact(core.Fact{Predicate: "processed_intent", Args: []interface{}{intentID}})
				_ = m.kernel.RetractFact(core.Fact{Predicate: "executive_processed_intent", Args: []interface{}{intentID}})
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
				_ = m.kernel.Assert(core.Fact{Predicate: "processed_intent", Args: []interface{}{intentID}})

				// Glass Box: Emit kernel event
				if m.glassBoxEventBus != nil && m.glassBoxEnabled {
					m.glassBoxEventBus.Emit(transparency.GlassBoxEvent{
						Timestamp: time.Now(),
						Category:  transparency.CategoryKernel,
						Summary:   fmt.Sprintf("Asserted: user_intent(%s, %s, %s)", intent.Category, intent.Verb, truncateSummary(intent.Target, 30)),
						TurnID:    m.turnCount,
					})
				}
			} else {
				// Glass Box: Emit kernel event (handled by system shard)
				if m.glassBoxEventBus != nil && m.glassBoxEnabled {
					m.glassBoxEventBus.Emit(transparency.GlassBoxEvent{
						Timestamp: time.Now(),
						Category:  transparency.CategoryKernel,
						Summary:   fmt.Sprintf("Intent handled by PerceptionFirewall: %s%s", intent.Category, intent.Verb),
						TurnID:    m.turnCount,
					})
				}
			}

			// If this is an issue-driven request, seed issue facts for activation and JIT selection.
			m.seedIssueFacts(intent, input)

			// GAP-002 FIX: Seed campaign facts if there's an active campaign.
			// This enables the activation engine and JIT compiler to be campaign-aware.
			m.seedCampaignFacts()
		}

		if reflection := m.performReflection(ctx, input, intent); reflection != nil {
			m.lastReflection = reflection
			if len(reflection.Warnings) > 0 {
				warnings = append(warnings, reflection.Warnings...)
			}
		} else {
			m.lastReflection = nil
		}

		// 1.3.1 MEMORY OPERATIONS: Process promote_to_long_term, forget, etc.
		// This enables "remember X" and "learn that Y" instructions
		if len(intent.MemoryOperations) > 0 && m.localDB != nil {
			for _, memOp := range intent.MemoryOperations {
				switch memOp.Op {
				case "promote_to_long_term":
					if err := m.localDB.StoreFact(memOp.Key, []interface{}{memOp.Value}, "learned", 10); err != nil {
						warnings = append(warnings, fmt.Sprintf("[Memory] failed to store: %v", err))
					}
				case "forget":
					if m.kernel != nil {
						m.kernel.Retract(memOp.Key)
					}
				}
			}
		}

		// 1.3.2 DREAM STATE: Multi-agent simulation/learning mode
		// When user asks "what if", "imagine", "hypothetically" - consult all shards without executing
		if intent.Verb == "/dream" {
			m.ReportStatus("Dream: consulting shards...")
			return m.handleDreamState(ctx, intent, input)
		}

		// 1.3.3 ASSAULT CAMPAIGN: Auto-start adversarial assault campaigns from natural language.
		// Example: "run an assault campaign on internal/core"
		if args, ok := assaultArgsFromNaturalLanguage(m.workspace, input, intent); ok {
			m.ReportStatus("Assault: starting campaign...")
			cmd := m.startAssaultCampaign(args)
			return cmd()
		}

		// 1.4 AUTO-CLARIFICATION: If the request looks like a campaign/plan ask, run the clarifier shard
		if m.shouldAutoClarify(&intent, input) {
			m.ReportStatus("Clarifier: generating questions...")
			if res, err := m.runClarifierShard(ctx, input); err == nil && res != "" {
				surface := m.appendSystemSummary(
					res+"\n\nReply with answers, then use `/launchcampaign <goal>` when you are ready to start.",
					m.collectSystemSummary(ctx, baseRoutingCount, baseExecCount),
				)
				return assistantMsg{
					Surface: surface,
					ClarifyUpdate: &ClarifyUpdate{
						LastClarifyInput:     input,
						LaunchClarifyPending: true,
						LaunchClarifyGoal:    input,
						LaunchClarifyAnswers: "",
					},
				}
			} else if err != nil {
				warnings = append(warnings, fmt.Sprintf("Clarifier shard unavailable: %v", err))
			}
		}

		// 1.4.1 GENERAL CLARIFICATION: Guard ambiguous intents before delegation.
		if question, options, ok := m.shouldClarifyFromKernel(&intent, input); ok {
			return clarificationMsg{
				Question:      question,
				Options:       options,
				Context:       input,
				PendingIntent: &intent,
			}
		}

		// 1.4.2 FALLBACK CLARIFICATION: Heuristic-only check if kernel has no question.
		if m.shouldClarifyIntent(&intent, input) {
			m.ReportStatus("Clarifier: resolving ambiguity...")
			if res, err := m.runClarifierShard(ctx, input); err == nil && res != "" {
				return clarificationMsg{
					Question:      res,
					Options:       []string{},
					Context:       input,
					PendingIntent: &intent,
				}
			} else if err != nil {
				warnings = append(warnings, fmt.Sprintf("Clarifier shard unavailable: %v", err))
			}
		}

		// 1.5 MULTI-STEP TASK DETECTION: Check if task requires multiple steps
		// This implements autonomous multi-step execution without campaigns
		isMultiStep := detectMultiStepTask(input, intent)
		if isMultiStep {
			m.ReportStatus("Multi-step: decomposing task...")
			steps := decomposeTask(input, intent, m.workspace)
			if len(steps) > 1 {
				cmd := m.executeMultiStepTask(ctx, intent, input, steps)
				return cmd()
			}
		}

		// 1.6 DELEGATION CHECK: Route to appropriate shard if verb indicates delegation
		// This implements automatic shard spawning from natural language
		// Uses verification loop to ensure quality (no mock code, no placeholders)
		shardType := perception.GetShardTypeForVerb(intent.Verb)

		// Glass Box: Emit routing decision
		if m.glassBoxEventBus != nil && m.glassBoxEnabled {
			if shardType != "" {
				m.glassBoxEventBus.Emit(transparency.GlassBoxEvent{
					Timestamp: time.Now(),
					Category:  transparency.CategoryKernel,
					Summary:   fmt.Sprintf("Routing: %s → %s shard (confidence: %.0f%%)", intent.Verb, shardType, intent.Confidence*100),
					TurnID:    m.turnCount,
				})
			} else {
				m.glassBoxEventBus.Emit(transparency.GlassBoxEvent{
					Timestamp: time.Now(),
					Category:  transparency.CategoryKernel,
					Summary:   fmt.Sprintf("Routing: %s → no shard delegation", intent.Verb),
					TurnID:    m.turnCount,
				})
			}
		}

		if shardType != "" && intent.Confidence >= 0.5 {
			if m.needsWorkspaceScanForDelegation(intent) && !workspaceScanned {
				workspaceScanned = m.loadWorkspaceFacts(ctx, intent, &warnings)
			}
			m.ReportStatus(fmt.Sprintf("Act: delegating to %s...", shardType))
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

				srPayload := &ShardResultPayload{
					ShardType: shardType,
					Task:      task,
					Result:    result,
					Facts:     facts,
				}

				if verifyErr != nil {
					// Check if max retries exceeded - escalate to user
					if verifyErr.Error() == "max retries exceeded - escalating to user" {
						response := formatVerificationEscalation(task, shardType, verification)
						return assistantMsg{
							Surface:     m.appendSystemSummary(response, m.collectSystemSummary(ctx, baseRoutingCount, baseExecCount)),
							ShardResult: srPayload,
						}
					}
					return errorMsg(fmt.Errorf("verified execution failed: %w", verifyErr))
				}

				// Format response with verification confidence
				response := formatVerifiedResponse(intent, shardType, task, result, verification)
				return assistantMsg{
					Surface:     m.appendSystemSummary(response, m.collectSystemSummary(ctx, baseRoutingCount, baseExecCount)),
					ShardResult: srPayload,
				}
			}

			// Shard spawn with queue backpressure management (user-initiated = high priority)
			// Glass Box: Emit shard spawn event
			if m.glassBoxEventBus != nil && m.glassBoxEnabled {
				m.glassBoxEventBus.Emit(transparency.GlassBoxEvent{
					Timestamp: time.Now(),
					Category:  transparency.CategoryShard,
					Summary:   fmt.Sprintf("Spawning: %s (task: %s)", shardType, truncateSummary(task, 40)),
					Source:    shardType,
					TurnID:    m.turnCount,
				})
			}

			result, spawnErr := m.spawnTaskWithContext(ctx, shardType, task, sessionCtx, types.PriorityHigh)

			// Glass Box: Emit shard completion event
			if m.glassBoxEventBus != nil && m.glassBoxEnabled {
				status := "completed"
				if spawnErr != nil {
					status = "failed"
				}
				m.glassBoxEventBus.Emit(transparency.GlassBoxEvent{
					Timestamp: time.Now(),
					Category:  transparency.CategoryShard,
					Summary:   fmt.Sprintf("Shard %s: %s (result: %d chars)", shardType, status, len(result)),
					Source:    shardType,
					TurnID:    m.turnCount,
				})

				// Glass Box: Emit JIT compilation stats if available
				if m.jitCompiler != nil {
					if jitResult := m.jitCompiler.GetLastResult(); jitResult != nil && jitResult.Stats != nil {
						stats := jitResult.Stats
						m.glassBoxEventBus.Emit(transparency.GlassBoxEvent{
							Timestamp: time.Now(),
							Category:  transparency.CategoryJIT,
							Summary:   fmt.Sprintf("JIT: %d atoms (%d skel + %d flesh), %d/%d tokens (%.0f%%)", stats.AtomsSelected, stats.SkeletonAtoms, stats.FleshAtoms, stats.TokensUsed, stats.TokenBudget, stats.BudgetUtilization*100),
							Duration:  stats.Duration,
							TurnID:    m.turnCount,
						})
					}
				}
			}

			// CRITICAL FIX: Inject shard results as facts for cross-turn context
			// This enables the main agent to reference shard outputs in future turns
			shardID := fmt.Sprintf("%s-%d", shardType, time.Now().UnixNano())
			facts := m.shardMgr.ResultToFacts(shardID, shardType, task, result, spawnErr)
			if m.kernel != nil && len(facts) > 0 {
				if loadErr := m.kernel.LoadFacts(facts); loadErr != nil {
					warnings = append(warnings, fmt.Sprintf("[ShardFacts] Warning: %v", loadErr))
				}
			}

			srPayload := &ShardResultPayload{
				ShardType: shardType,
				Task:      task,
				Result:    result,
				Facts:     facts,
			}

			if spawnErr != nil {
				return errorMsg(fmt.Errorf("shard delegation failed: %w", spawnErr))
			}

			// 1.6.1 AUTO-INTERPRETATION: For Reviewer/Tester shards, explain the findings
			if shardType == "reviewer" || shardType == "tester" {
				// 1.6.2 REVIEWER VALIDATION CHECK: Self-correction feedback loop
				// Check if the review shows signs of inaccuracy and flag for user attention
				var validationWarning string
				if shardType == "reviewer" {
					if m.shardMgr.CheckReviewNeedsValidation(shardID) {
						reasons := m.shardMgr.GetReviewSuspectReasons(shardID)
						if len(reasons) > 0 {
							validationWarning = "**Review Validation Alert**: This review may contain inaccuracies.\n"
							validationWarning += "Reasons: " + strings.Join(reasons, ", ") + "\n"
							validationWarning += "Please verify findings before acting on them. "
							validationWarning += "Use `/reject-finding <file>:<line> <reason>` to help the system learn from mistakes."
							warnings = append(warnings, "Review flagged for validation: "+strings.Join(reasons, ", "))
						}
					}
				}

				response := m.formatInterpretedResult(ctx, input, shardType, task, result, validationWarning)
				surface := m.appendSystemSummary(response, m.collectSystemSummary(ctx, baseRoutingCount, baseExecCount))

				// CONTINUATION PROTOCOL: Check for pending subtasks before returning
				if cont := m.checkContinuation(shardType, task, result); cont != nil {
					return continuationInitMsg{
						completedSurface: surface,
						firstResult:      srPayload,
						next:             *cont,
						totalSteps:       0,
					}
				}

				return assistantMsg{
					Surface:     surface,
					ShardResult: srPayload,
				}
			}

			// Format a rich response combining LLM surface response and shard result
			response := formatDelegatedResponse(intent, shardType, task, result)
			surface := m.appendSystemSummary(response, m.collectSystemSummary(ctx, baseRoutingCount, baseExecCount))

			// CONTINUATION PROTOCOL: Check for pending subtasks before returning
			if cont := m.checkContinuation(shardType, task, result); cont != nil {
				return continuationInitMsg{
					completedSurface: surface,
					firstResult:      srPayload,
					next:             *cont,
					totalSteps:       0,
				}
			}

			return assistantMsg{
				Surface:     surface,
				ShardResult: srPayload,
			}
		}

		// 1.7 STATS: Deterministic codebase/file statistics (no LLM follow-up).
		if shardType == "" && intent.Verb == "/stats" {
			m.ReportStatus("Stats: computing...")
			statsResp, err := m.handleStatsIntent(ctx, intent)
			if err != nil {
				return responseMsg(m.appendSystemSummary(
					fmt.Sprintf("Stats error: %v", err),
					m.collectSystemSummary(ctx, baseRoutingCount, baseExecCount),
				))
			}
			return responseMsg(m.appendSystemSummary(statsResp, m.collectSystemSummary(ctx, baseRoutingCount, baseExecCount)))
		}

		// 1.7 DIRECT RESPONSE: For non-actionable verbs (/explain, /read, etc.) with
		// no shard and a valid perception response, return the perception response
		// directly. This handles greetings, capability questions, and general queries
		// without requiring a second articulation LLM call.
		if shardType == "" && intent.Response != "" && isConversationalIntent(intent) {
			// Glass Box: Emit direct response path
			if m.glassBoxEventBus != nil && m.glassBoxEnabled {
				m.glassBoxEventBus.Emit(transparency.GlassBoxEvent{
					Timestamp: time.Now(),
					Category:  transparency.CategoryControl,
					Summary:   fmt.Sprintf("Direct response: %s (bypassing articulation)", intent.Verb),
					Details:   "Conversational intent handled directly from perception without full articulation pass",
					TurnID:    m.turnCount,
				})
			}
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
		// Load workspace facts only if intent requires it (optimization).
		// Use incremental scan to avoid reparsing unchanged repos.
		if !workspaceScanned {
			workspaceScanned = m.loadWorkspaceFacts(ctx, intent, &warnings)
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
			if _, err := m.virtualStore.HydrateSessionContext(ctx, m.sessionID, input, m.collectTraceShardTypes()); err != nil {
				warnings = append(warnings, fmt.Sprintf("Hydrate session context warning: %v", err))
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
			if actionKey == "search_files" {
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
			if actionKey == "generate_tool" && m.autopoiesis != nil {
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

		// 4.5 SYSTEM ACTION HANDLING: Surface kernel-driven delegations and execution results.
		if msg := m.handleSystemDelegations(ctx, input, intent, baseRoutingCount, baseExecCount); msg != nil {
			return msg
		}

		// 5. CONTEXT SELECTION (Spreading Activation)
		contextFacts, _ := m.kernel.Query("context_to_inject")

		// 6. ARTICULATION (Response Generation)
		systemPrompts, _ := m.kernel.Query("final_system_prompt")
		systemPrompt := ""
		if len(systemPrompts) > 0 && len(systemPrompts[0].Args) > 0 {
			systemPrompt = types.ExtractString(systemPrompts[0].Args[0])
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

		// Add grounding sources as a "Sources:" section for transparency
		// This shows users what URLs were used to ground the response (Google Search / URL Context)
		if len(artOutput.GroundingSources) > 0 {
			response += "\n\n**Sources:**\n"
			for _, src := range artOutput.GroundingSources {
				response += fmt.Sprintf("- %s\n", src)
			}
		}

		// =====================================================================
		// CONTEXT FEEDBACK STORAGE (Third Feedback Loop)
		// =====================================================================
		// Store LLM's feedback on which context facts were useful vs noise.
		// This feeds into the ActivationEngine to improve future context selection.
		if artOutput.ContextFeedback != nil && m.feedbackStore != nil {
			// GAP-016 FIX: Include JIT manifest hash for context learning correlation
			manifestHash := ""
			if m.jitCompiler != nil {
				if jitResult := m.jitCompiler.GetLastResult(); jitResult != nil && jitResult.Manifest != nil {
					manifestHash = jitResult.Manifest.ContextHash
				}
			}

			if err := m.feedbackStore.StoreFeedback(
				m.turnCount,
				manifestHash,
				artOutput.ContextFeedback.OverallUsefulness,
				intent.Verb,
				artOutput.ContextFeedback.HelpfulFacts,
				artOutput.ContextFeedback.NoiseFacts,
			); err != nil {
				logging.Get(logging.CategoryContext).Warn("Failed to store context feedback: %v", err)
			} else {
				logging.ContextDebug("Stored context feedback: usefulness=%.2f, helpful=%d, noise=%d, hash=%s",
					artOutput.ContextFeedback.OverallUsefulness,
					len(artOutput.ContextFeedback.HelpfulFacts),
					len(artOutput.ContextFeedback.NoiseFacts),
					manifestHash)
			}
		}

		// =====================================================================
		// KNOWLEDGE REQUEST HANDLING (LLM-First Knowledge Discovery)
		// =====================================================================
		// If the LLM requested knowledge from specialists, gather it and re-process.
		// This enables the LLM to proactively research topics it doesn't know.
		// Guard: Don't re-request knowledge if we already have pending results (prevents infinite loop).
		if len(artOutput.KnowledgeRequests) > 0 && !m.awaitingKnowledge && len(m.pendingKnowledge) == 0 {
			logging.Get(logging.CategoryContext).Info(
				"LLM requested knowledge: %d specialists to consult",
				len(artOutput.KnowledgeRequests),
			)
			return m.handleKnowledgeRequests(ctx, artOutput.KnowledgeRequests, input, response)
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

			// Glass Box: Emit control packet event
			if m.glassBoxEventBus != nil && m.glassBoxEnabled {
				summary := fmt.Sprintf("Control: %d mangle updates, %d memory ops",
					len(controlPacket.MangleUpdates), len(controlPacket.MemoryOperations))
				var details strings.Builder
				if len(controlPacket.MangleUpdates) > 0 {
					details.WriteString("Mangle Updates:\n")
					for _, u := range controlPacket.MangleUpdates {
						details.WriteString("  " + truncateSummary(u, 60) + "\n")
					}
				}
				if len(controlPacket.MemoryOperations) > 0 {
					details.WriteString("Memory Operations:\n")
					for _, op := range controlPacket.MemoryOperations {
						details.WriteString(fmt.Sprintf("  %s: %s\n", op.Op, op.Key))
					}
				}
				if controlPacket.SelfCorrection != nil && controlPacket.SelfCorrection.Triggered {
					summary += " [SELF-CORRECTION]"
					details.WriteString("Self-Correction: " + truncateSummary(controlPacket.SelfCorrection.Hypothesis, 100) + "\n")
				}
				m.glassBoxEventBus.Emit(transparency.GlassBoxEvent{
					Timestamp: time.Now(),
					Category:  transparency.CategoryControl,
					Summary:   summary,
					Details:   details.String(),
					TurnID:    m.turnCount,
				})
			}

			turn := ctxcompress.Turn{
				Number:          m.turnCount, // zero-based turn numbering
				Role:            "assistant",
				UserInput:       input,
				SurfaceResponse: response,
				ControlPacket:   controlPacket,
				Timestamp:       time.Now(),
			}

			// Process turn asynchronously - don't block response
			if m.goroutineWg != nil {
				m.goroutineWg.Add(1)
			}
			go func(t ctxcompress.Turn, capturedIntent perception.Intent, capturedResponse string) {
				if m.goroutineWg != nil {
					defer m.goroutineWg.Done()
				}
				// Panic recovery for background compression goroutine
				defer func() {
					if r := recover(); r != nil {
						logging.API("PANIC in compression goroutine (recovered): %v", r)
					}
				}()

				// Use a shutdown-scoped context so compression can finish after the main turn ctx is canceled.
				baseCtx := m.shutdownCtx
				if baseCtx == nil {
					baseCtx = context.Background()
				}
				compressCtx, cancel := context.WithTimeout(baseCtx, 2*time.Minute)
				defer cancel()

				// COMPRESSION: Semantic compression for infinite context (§8.2)
				if _, err := m.compressor.ProcessTurn(compressCtx, t); err != nil {
					// Log compression errors but don't fail the turn
					fmt.Printf("[Compressor] Warning: %v\n", err)
				}

				// KNOWLEDGE PERSISTENCE: Populate knowledge.db tables for learning
				// This implements the missing learning loop identified in user feedback
				if m.localDB != nil {
					m.persistTurnToKnowledge(t, capturedIntent, capturedResponse)
				}
			}(turn, intent, response)
		}

		return responseMsg(m.appendSystemSummary(response, m.collectSystemSummary(ctx, baseRoutingCount, baseExecCount)))
	}
}

// processInputWithKnowledge re-processes input after knowledge gathering.
// The gathered knowledge is already stored in m.pendingKnowledge and will be
// injected into SessionContext by buildSessionContext(), making it available
// to the articulation layer for synthesizing an informed response.
func (m Model) processInputWithKnowledge(input string) tea.Cmd {
	return func() tea.Msg {
		// Add context about gathered knowledge to the input
		var knowledgeSummary strings.Builder
		if len(m.pendingKnowledge) > 0 {
			knowledgeSummary.WriteString("\n\n---\n**Gathered Knowledge from Specialists:**\n\n")
			for _, kr := range m.pendingKnowledge {
				if kr.Error == nil && kr.Response != "" {
					knowledgeSummary.WriteString(fmt.Sprintf("### From %s\n", kr.Specialist))
					knowledgeSummary.WriteString(fmt.Sprintf("**Query:** %s\n\n", kr.Query))
					knowledgeSummary.WriteString(kr.Response)
					knowledgeSummary.WriteString("\n\n")
				}
			}
			knowledgeSummary.WriteString("---\n\n")
			knowledgeSummary.WriteString("When responding:\n")
			knowledgeSummary.WriteString("1. Provide a short **Knowledge Summary** section that synthesizes the gathered specialist responses.\n")
			knowledgeSummary.WriteString("2. Then answer the user's question.\n")
		}

		// Augment the input with gathered knowledge context
		enrichedInput := input + knowledgeSummary.String()

		// Re-run the full processing pipeline with enriched context
		// processInput will detect that pendingKnowledge is non-empty and
		// skip further knowledge gathering (preventing infinite loops)
		return m.processInput(enrichedInput)()
	}
}

// seedIssueFacts extracts issue text + keywords from user input and asserts issue_* facts.
// This drives issue-aware spreading activation and prompt atom selection.
func (m *Model) seedIssueFacts(intent perception.Intent, rawInput string) {
	if m.kernel == nil {
		return
	}

	// Only seed for verbs that are typically issue-driven.
	switch intent.Verb {
	case "/fix", "/debug", "/review", "/security":
	default:
		return
	}

	issueText := strings.TrimSpace(rawInput)
	if issueText == "" {
		return
	}

	// Keep the stored issue text bounded to avoid EDB bloat.
	const maxIssueChars = 4000
	if len(issueText) > maxIssueChars {
		issueText = issueText[:maxIssueChars]
	}

	issueID := fmt.Sprintf("/issue_%d", time.Now().UnixNano())
	keywords := retrieval.ExtractKeywords(issueText)

	facts := make([]core.Fact, 0, 1+len(keywords.Weights)+len(keywords.MentionedFiles))
	facts = append(facts, core.Fact{
		Predicate: "issue_text",
		Args:      []interface{}{issueID, issueText},
	})

	for kw, weight := range keywords.Weights {
		if strings.TrimSpace(kw) == "" {
			continue
		}
		facts = append(facts, core.Fact{
			Predicate: "issue_keyword",
			Args:      []interface{}{issueID, kw, weight},
		})
	}

	for _, file := range keywords.MentionedFiles {
		if strings.TrimSpace(file) == "" {
			continue
		}
		facts = append(facts, core.Fact{
			Predicate: "file_mentioned",
			Args:      []interface{}{file, issueID},
		})
	}

	// GAP-017 FIX: Assert tiered_context_file facts for issue-driven file relevance
	// Tier 1: Directly mentioned files (highest relevance)
	// This enables the activation engine to boost these files in context selection
	for i, file := range keywords.MentionedFiles {
		if strings.TrimSpace(file) == "" {
			continue
		}
		// tiered_context_file(IssueID, File, Tier, Relevance, TokenCount)
		// Tier 1 for directly mentioned, relevance decreases by position
		relevance := 1.0 - (float64(i) * 0.1)
		if relevance < 0.5 {
			relevance = 0.5
		}
		facts = append(facts, core.Fact{
			Predicate: "tiered_context_file",
			Args:      []interface{}{issueID, file, "/tier1", relevance, 0},
		})
	}

	_ = m.kernel.LoadFacts(facts)
}

// seedCampaignFacts asserts campaign context facts for spreading activation and JIT selection.
// GAP-002 FIX: This enables the activation engine and JIT compiler to be campaign-aware.
func (m *Model) seedCampaignFacts() {
	if m.kernel == nil || m.activeCampaign == nil {
		return
	}

	c := m.activeCampaign
	facts := make([]core.Fact, 0, 10)

	// current_campaign(CampaignID)
	facts = append(facts, core.Fact{
		Predicate: "current_campaign",
		Args:      []interface{}{c.ID},
	})

	// Find current phase (first non-completed phase)
	var currentPhase *campaign.Phase
	for i := range c.Phases {
		if c.Phases[i].Status != campaign.PhaseCompleted {
			currentPhase = &c.Phases[i]
			break
		}
	}

	if currentPhase != nil {
		// current_phase(PhaseID)
		facts = append(facts, core.Fact{
			Predicate: "current_phase",
			Args:      []interface{}{currentPhase.ID},
		})

		// phase_objective(PhaseID, ObjectiveIndex, Description)
		for i, obj := range currentPhase.Objectives {
			objID := fmt.Sprintf("/obj_%s_%d", currentPhase.ID, i)
			facts = append(facts, core.Fact{
				Predicate: "phase_objective",
				Args:      []interface{}{currentPhase.ID, objID, obj.Description},
			})
		}

		// Find next pending task
		for _, task := range currentPhase.Tasks {
			if task.Status == campaign.TaskPending || task.Status == campaign.TaskInProgress {
				// next_campaign_task(TaskID)
				facts = append(facts, core.Fact{
					Predicate: "next_campaign_task",
					Args:      []interface{}{task.ID},
				})

				// task_artifact(TaskID, ArtifactType, Path)
				for _, artifact := range task.Artifacts {
					facts = append(facts, core.Fact{
						Predicate: "task_artifact",
						Args:      []interface{}{task.ID, artifact.Type, artifact.Path},
					})
				}
				break // Only the next task
			}
		}
	}

	_ = m.kernel.LoadFacts(facts)
}

// runClarifierShard invokes the requirements_interrogator shard synchronously to gather clarifying questions.
func (m Model) runClarifierShard(ctx context.Context, goal string) (string, error) {
	if m.shardMgr == nil && m.taskExecutor == nil {
		return "", fmt.Errorf("no executor available: both taskExecutor and shardMgr are nil")
	}

	cctx, cancel := context.WithTimeout(ctx, config.GetLLMTimeouts().ArticulationTimeout)
	defer cancel()

	result, err := m.spawnTask(cctx, "requirements_interrogator", goal)
	if err != nil {
		return "", err
	}
	return result, nil
}
