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

		var warnings []string

		// 1. PERCEPTION (Transducer)
		intent, err := m.transducer.ParseIntent(ctx, input)
		if err != nil {
			return errorMsg(fmt.Errorf("perception error: %w", err))
		}
		if strings.TrimSpace(intent.Response) == "" {
			return errorMsg(fmt.Errorf("LLM returned empty response for input: %q", input))
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
		shardType := perception.GetShardTypeForVerb(intent.Verb)
		if shardType != "" && intent.Confidence >= 0.5 {
			// Format task based on verb and target
			task := formatShardTask(intent.Verb, intent.Target, intent.Constraint, m.workspace)

			// Spawn the shard and return its result
			result, spawnErr := m.shardMgr.Spawn(ctx, shardType, task)
			if spawnErr != nil {
				return errorMsg(fmt.Errorf("shard delegation failed: %w", spawnErr))
			}

			// Format a rich response combining LLM surface response and shard result
			response := formatDelegatedResponse(intent, shardType, task, result)
			return responseMsg(response)
		}

		// 1.7 DIRECT RESPONSE: For non-actionable verbs (/explain, /read, etc.) with
		// no shard and a valid perception response, return the perception response
		// directly. This handles greetings, capability questions, and general queries
		// without requiring a second articulation LLM call.
		if shardType == "" && intent.Response != "" && isConversationalIntent(intent) {
			return responseMsg(intent.Response)
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
			}
		}

		// 3. STATE UPDATE (Kernel)
		if err := m.kernel.LoadFacts([]core.Fact{intent.ToFact()}); err != nil {
			return errorMsg(fmt.Errorf("kernel load error: %w", err))
		}
		// Fix Bug #7: Update system facts (Time, etc.)
		if err := m.kernel.UpdateSystemFacts(); err != nil {
			return errorMsg(fmt.Errorf("system facts update error: %w", err))
		}
		_ = m.kernel.LoadFacts(m.shardMgr.ToFacts())

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

		// Use full articulation output to capture MemoryOperations
		artOutput, err := articulateWithContextFull(ctx, m.client, intent, payloadForArticulation(intent, mangleUpdates), contextFacts, warnings, systemPrompt)
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

		return responseMsg(response)
	}
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
				if err != nil {
					results = append(results, fmt.Sprintf("**Status**: ❌ Failed\n**Error**: %v\n", err))
					// Don't continue to dependent steps if this fails
					continue
				}

				// Store result for dependencies
				stepResults[i] = result

				// Truncate very long results
				if len(result) > 1000 {
					result = result[:1000] + "\n... (truncated)"
				}

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
