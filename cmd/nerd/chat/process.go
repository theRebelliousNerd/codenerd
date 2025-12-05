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

		// 1.5 DELEGATION CHECK: Route to appropriate shard if verb indicates delegation
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

		// 1.6 AUTOPOIESIS CHECK: Analyze for complexity, persistence, and tool needs
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

		response, err := articulateWithContext(ctx, m.client, intent, payloadForArticulation(intent, mangleUpdates), contextFacts, warnings, systemPrompt)
		if err != nil {
			return errorMsg(err)
		}

		// 7. SEMANTIC COMPRESSION (Process turn for infinite context)
		// This implements §8.2: Compress surface text, retain only logical atoms
		if m.compressor != nil {
			controlPacket := &perception.ControlPacket{
				IntentClassification: perception.IntentClassification{
					Category:   intent.Category,
					Verb:       intent.Verb,
					Target:     intent.Target,
					Constraint: intent.Constraint,
					Confidence: intent.Confidence,
				},
				MangleUpdates:    mangleUpdates,
				MemoryOperations: nil, // Populated by articulation layer in future
			}
			turn := ctxcompress.Turn{
				UserInput:       input,
				SurfaceResponse: response,
				ControlPacket:   controlPacket,
				Timestamp:       time.Now(),
			}
			// Process turn asynchronously - don't block response
			go func() {
				if _, err := m.compressor.ProcessTurn(ctx, turn); err != nil {
					// Log compression errors but don't fail the turn
					fmt.Printf("[Compressor] Warning: %v\n", err)
				}
			}()
		}

		return responseMsg(response)
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
