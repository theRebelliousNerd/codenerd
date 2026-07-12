package chat

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"codenerd/cmd/nerd/ui"
	"codenerd/internal/campaign"
	"codenerd/internal/config"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/store"

	tea "github.com/charmbracelet/bubbletea"
)

// handleConfigCommand handles the corresponding chat slash-command.
func (m Model) handleConfigCommand(input string, parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 2 {
		m = m.addMessage(Message{
			Role: "assistant",
			Content: `Configuration commands:

| Command | Description |
|---------|-------------|
| /config wizard | Full interactive configuration dialogue |
| /config set-key <key> | Set API key |
| /config set-theme <theme> | Set theme (light/dark) |
| /config engine [api\|claude-cli\|codex-cli\|xai-oauth] | Set LLM engine |
| /config show | Show current configuration |`,
			Time: time.Now(),
		})
	} else if parts[1] == "wizard" {
		// Enter config wizard mode
		m.setInputMode(InputModeConfigWizard)
		m.configWizard = NewConfigWizard()
		m.textarea.Placeholder = "Press Enter to start..."
		m = m.addMessage(Message{
			Role: "assistant",
			Content: `## codeNERD Configuration Wizard

This wizard will guide you through configuring:

1. **LLM Provider** - Choose between Z.AI, Anthropic, OpenAI, Gemini, or xAI
2. **Model Selection** - Pick the model for your provider
3. **Per-Shard Config** - Customize settings for coder, tester, reviewer, researcher
4. **Embedding Engine** - Configure Ollama or GenAI for semantic search
5. **Context Window** - Set token limits and compression settings
6. **Resource Limits** - Configure concurrent shards and memory

Press **Enter** to begin...`,
			Time: time.Now(),
		})
	} else if parts[1] == "show" {
		// Show current configuration
		content := m.renderCurrentConfig()
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: content,
			Time:    time.Now(),
		})
	} else if parts[1] == "set-key" {
		// API keys are now provider-specific - guide user to use wizard or edit config directly
		m = m.addMessage(Message{
			Role: "assistant",
			Content: "API keys are now **provider-specific**. To update your API key:\n\n" +
				"1. Run `/config wizard` to reconfigure all settings\n" +
				"2. Or edit `.nerd/config.json` directly with the appropriate key:\n" +
				"   - `zai_api_key` for Z.AI\n" +
				"   - `anthropic_api_key` for Anthropic/Claude\n" +
				"   - `openai_api_key` for OpenAI\n" +
				"   - `gemini_api_key` for Google Gemini\n" +
				"   - `xai_api_key` for xAI/Grok\n" +
				"   - `openrouter_api_key` for OpenRouter",
			Time: time.Now(),
		})
	} else if parts[1] == "set-theme" && len(parts) >= 3 {
		theme := parts[2]
		if theme == "dark" || theme == "light" {
			m.Config.Theme = theme
			// Load current config, update theme, and save
			cfg, _ := config.GlobalConfig()
			if cfg == nil {
				cfg = config.DefaultUserConfig()
			}
			cfg.Theme = theme
			if err := cfg.Save(config.DefaultUserConfigPath()); err != nil {
				logging.Routing("[commands] failed to save config: %v", err)
			}
			// Apply theme
			if theme == "dark" {
				m.styles = ui.NewStyles(ui.DarkTheme())
			} else {
				m.styles = ui.DefaultStyles()
			}
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: fmt.Sprintf("Theme set to: %s", theme),
				Time:    time.Now(),
			})
		} else {
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: "Invalid theme. Use 'light' or 'dark'.",
				Time:    time.Now(),
			})
		}
	} else if parts[1] == "model" {
		cfg := m.Config
		if cfg == nil {
			cfg = config.DefaultUserConfig()
		}
		activeProvider, _ := cfg.GetActiveProvider()
		available := ProviderModels[activeProvider]

		if len(parts) == 2 {
			// Show current model and available models
			currentModel := cfg.Model
			if currentModel == "" {
				currentModel = getModelRecursive(m.client)
			}
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("Active LLM provider: **%s**\n", activeProvider))
			sb.WriteString(fmt.Sprintf("Current active model: **%s**\n\n", currentModel))
			sb.WriteString("Available models for this provider:\n")
			for _, mName := range available {
				if mName == currentModel {
					sb.WriteString(fmt.Sprintf("- **%s** (currently active)\n", mName))
				} else {
					sb.WriteString(fmt.Sprintf("- `%s`\n", mName))
				}
			}
			sb.WriteString("\nTo change model, run: `/config model <model-name>`\n")
			sb.WriteString("Or run `/model <model-name>` to switch instantly.")
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: sb.String(),
				Time:    time.Now(),
			})
		} else {
			// Set model
			newModel := parts[2]
			// Validate model
			valid := slices.Contains(available, newModel)
			if !valid {
				// Also support matching without provider prefix for openrouter
				if activeProvider == "openrouter" {
					for _, mName := range available {
						if strings.HasSuffix(mName, "/"+newModel) || mName == newModel {
							newModel = mName
							valid = true
							break
						}
					}
				}
			}
			if !valid {
				m = m.addMessage(Message{
					Role:    "assistant",
					Content: fmt.Sprintf("Error: `%s` is not a recognized model for provider `%s`.\nUse `/config model` to see list of valid models.", newModel, activeProvider),
					Time:    time.Now(),
				})
			} else {
				cfg.Model = newModel
				if err := cfg.Save(config.DefaultUserConfigPath()); err != nil {
					m = m.addMessage(Message{
						Role:    "assistant",
						Content: fmt.Sprintf("Error saving config: %s", err.Error()),
						Time:    time.Now(),
					})
				} else {
					m.Config = cfg
					// Set the model on the active client if possible
					setModelRecursive(m.client, newModel)
					m = m.addMessage(Message{
						Role:    "assistant",
						Content: fmt.Sprintf("✓ Active model for **%s** set to: **%s**", activeProvider, newModel),
						Time:    time.Now(),
					})
				}
			}
		}
	} else if parts[1] == "engine" {
		// Engine configuration for CLI backends
		cfg, _ := config.GlobalConfig()
		if cfg == nil {
			cfg = config.DefaultUserConfig()
		}

		if len(parts) == 2 {
			// Show current engine
			engine := cfg.GetEngine()
			var engineDesc string
			switch engine {
			case "claude-cli":
				cliCfg := cfg.GetClaudeCLIConfig()
				engineDesc = fmt.Sprintf("**Claude Code CLI** (model: %s, timeout: %ds)", cliCfg.Model, cliCfg.Timeout)
			case "codex-cli":
				cliCfg := cfg.GetCodexCLIConfig()
				skillEnabled := false
				schemaEnabled := false
				if cliCfg.SkillEnabled != nil {
					skillEnabled = *cliCfg.SkillEnabled
				}
				if cliCfg.EnableOutputSchema != nil {
					schemaEnabled = *cliCfg.EnableOutputSchema
				}
				engineDesc = fmt.Sprintf(
					"**Codex CLI** (model: %s, sandbox: %s, timeout: %ds, skill: %s, skill_enabled: %t, schema_mode: %t, max_concurrent_calls: %d, effective_scheduler_ceiling: %d)",
					cliCfg.Model,
					cliCfg.Sandbox,
					cliCfg.Timeout,
					cliCfg.SkillName,
					skillEnabled,
					schemaEnabled,
					cliCfg.MaxConcurrentCalls,
					cfg.GetEffectiveMaxConcurrentAPICalls(),
				)
			case "xai-oauth":
				oauthCfg := cfg.GetXAIOAuthConfig()
				engineDesc = fmt.Sprintf(
					"**SuperGrok OAuth** (model: %s, timeout: %ds, max_concurrent_calls: %d, effective_scheduler_ceiling: %d)",
					oauthCfg.Model,
					oauthCfg.Timeout,
					oauthCfg.MaxConcurrentCalls,
					cfg.GetEffectiveMaxConcurrentAPICalls(),
				)
			default:
				provider, _ := cfg.GetActiveProvider()
				engineDesc = fmt.Sprintf("**API** (provider: %s)", provider)
			}
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: fmt.Sprintf("Current engine: %s\n\n%s\n\nAvailable engines:\n- `api` - HTTP API (default)\n- `claude-cli` - Claude Code CLI (subscription)\n- `codex-cli` - Codex CLI (ChatGPT subscription)\n- `xai-oauth` - SuperGrok OAuth (SuperGrok / X Premium+ subscription)", engine, engineDesc),
				Time:    time.Now(),
			})
		} else {
			// Set engine
			newEngine := parts[2]
			if err := cfg.SetEngine(newEngine); err != nil {
				m = m.addMessage(Message{
					Role:    "assistant",
					Content: fmt.Sprintf("Error: %s", err.Error()),
					Time:    time.Now(),
				})
			} else {
				if err := cfg.Save(config.DefaultUserConfigPath()); err != nil {
					m = m.addMessage(Message{
						Role:    "assistant",
						Content: fmt.Sprintf("Error saving config: %s", err.Error()),
						Time:    time.Now(),
					})
				} else {
					m.Config = cfg
					m = m.addMessage(Message{
						Role:    "assistant",
						Content: fmt.Sprintf("Engine set to: **%s**\n\nRestart codeNERD for changes to take effect.", newEngine),
						Time:    time.Now(),
					})
				}
			}
		}
	}
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil

}

// handleEmbeddingCommand handles the corresponding chat slash-command.
func (m Model) handleEmbeddingCommand(input string, parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 2 {
		m = m.addMessage(Message{
			Role: "assistant",
			Content: `Embedding commands:
  /embedding set <provider> [api-key]  - Set embedding provider (ollama or genai)
  /embedding stats                      - Show embedding statistics
  /embedding reembed                    - Force re-embed all .nerd + internal DBs (vectors + prompt atoms)`,
			Time: time.Now(),
		})
	} else {
		switch parts[1] {
		case "set":
			if len(parts) < 3 {
				m = m.addMessage(Message{
					Role:    "assistant",
					Content: "Usage: /embedding set <ollama|genai> [api-key]",
					Time:    time.Now(),
				})
			} else {
				provider := parts[2]
				cfg, _ := config.GlobalConfig()
				if cfg == nil {
					cfg = config.DefaultUserConfig()
				}
				if cfg.Embedding == nil {
					cfg.Embedding = &config.EmbeddingConfig{}
				}
				cfg.Embedding.Provider = provider
				if provider == "ollama" {
					cfg.Embedding.OllamaEndpoint = "http://localhost:11434"
					cfg.Embedding.OllamaModel = "embeddinggemma:300m"
				} else if provider == "genai" && len(parts) >= 4 {
					cfg.Embedding.GenAIAPIKey = parts[3]
					cfg.Embedding.GenAIModel = "gemini-embedding-001"
				}
				if err := cfg.Save(config.DefaultUserConfigPath()); err != nil {
					m = m.addMessage(Message{
						Role:    "assistant",
						Content: fmt.Sprintf("Failed to save config: %v", err),
						Time:    time.Now(),
					})
				} else {
					m = m.addMessage(Message{
						Role:    "assistant",
						Content: fmt.Sprintf("✓ Embedding provider set to: %s\nRestart to apply changes.", provider),
						Time:    time.Now(),
					})
				}
			}
		case "stats":
			if m.localDB != nil {
				stats, err := m.localDB.GetVectorStats()
				if err != nil {
					m = m.addMessage(Message{
						Role:    "assistant",
						Content: fmt.Sprintf("Failed to get stats: %v", err),
						Time:    time.Now(),
					})
				} else {
					m = m.addMessage(Message{
						Role: "assistant",
						Content: fmt.Sprintf(`Embedding Statistics:
  Total Vectors: %v
  With Embeddings: %v
  Without Embeddings: %v
  Engine: %v
  Dimensions: %v`,
							stats["total_vectors"],
							stats["with_embeddings"],
							stats["without_embeddings"],
							stats["embedding_engine"],
							stats["embedding_dimensions"]),
						Time: time.Now(),
					})
				}
			} else {
				m = m.addMessage(Message{
					Role:    "assistant",
					Content: "No knowledge database available.",
					Time:    time.Now(),
				})
			}
		case "reembed":
			if m.localDB != nil {
				m = m.addMessage(Message{
					Role:    "assistant",
					Content: "Re-embedding all databases (vectors + prompt atoms)... this may take a while.",
					Time:    time.Now(),
				})
				m.viewport.SetContent(m.renderHistory())
				m.viewport.GotoBottom()
				m.textarea.Reset()
				m.isLoading = true
				return m, tea.Batch(m.spinner.Tick, m.runReembedAllDBs())
			} else {
				m = m.addMessage(Message{
					Role:    "assistant",
					Content: "No knowledge database available.",
					Time:    time.Now(),
				})
			}
		default:
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: "Unknown embedding command. Use /embedding for help.",
				Time:    time.Now(),
			})
		}
	}
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil

}

// handleLearnCommand handles the corresponding chat slash-command.
func (m Model) handleLearnCommand(input string, parts []string) (tea.Model, tea.Cmd) {
	if len(parts) > 1 && (parts[1] == "list" || parts[1] == "candidates") {
		if m.localDB == nil {
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: "No local database available.",
				Time:    time.Now(),
			})
		} else {
			candidates, err := m.localDB.ListLearningCandidates("", 50)
			if err != nil {
				m = m.addMessage(Message{
					Role:    "assistant",
					Content: fmt.Sprintf("Failed to list candidates: %v", err),
					Time:    time.Now(),
				})
			} else if len(candidates) == 0 {
				m = m.addMessage(Message{
					Role:    "assistant",
					Content: "No learning candidates found.",
					Time:    time.Now(),
				})
			} else {
				var sb strings.Builder
				sb.WriteString("## Staging Learning Candidates\n\n")
				sb.WriteString("| ID | Phrase | Verb | Target | Status | Count |\n")
				sb.WriteString("|----|--------|------|--------|--------|-------|\n")
				for _, c := range candidates {
					sb.WriteString(fmt.Sprintf("| %d | `%s` | `%s` | `%s` | **%s** | %d |\n",
						c.ID, c.Phrase, c.Verb, c.Target, c.Status, c.Count))
				}
				sb.WriteString("\n*Confirm or reject candidates using `/learn confirm <id>` or `/learn reject <id>`*")
				m = m.addMessage(Message{
					Role:    "assistant",
					Content: sb.String(),
					Time:    time.Now(),
				})
			}
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil
	}

	if len(parts) > 1 && parts[1] == "confirm" {
		if len(parts) < 3 {
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: "Usage: `/learn confirm <id>`",
				Time:    time.Now(),
			})
		} else {
			var id int64
			if _, err := fmt.Sscanf(parts[2], "%d", &id); err != nil {
				m = m.addMessage(Message{
					Role:    "assistant",
					Content: "Invalid candidate ID. Must be an integer.",
					Time:    time.Now(),
				})
			} else if m.localDB == nil {
				m = m.addMessage(Message{
					Role:    "assistant",
					Content: "No local database available.",
					Time:    time.Now(),
				})
			} else {
				cands, err := m.localDB.ListLearningCandidates("", 100)
				var targetCand *store.LearningCandidate
				if err == nil {
					for _, c := range cands {
						if c.ID == id {
							targetCand = &c
							break
						}
					}
				}
				if targetCand == nil {
					m = m.addMessage(Message{
						Role:    "assistant",
						Content: fmt.Sprintf("Candidate ID %d not found.", id),
						Time:    time.Now(),
					})
				} else {
					err := m.localDB.ConfirmLearningCandidate(id)
					if err == nil && perception.SharedTaxonomy != nil {
						verb := normalizeVerbAtom(targetCand.Verb)
						fact := fmt.Sprintf(
							`learned_exemplar("%s", %s, "%s", "", 0.90).`,
							escapeMangleString(targetCand.Phrase),
							verb,
							escapeMangleString(targetCand.Target),
						)
						_ = perception.SharedTaxonomy.PersistLearnedFact(fact)
					}
					if err != nil {
						m = m.addMessage(Message{
							Role:    "assistant",
							Content: fmt.Sprintf("Failed to confirm candidate: %v", err),
							Time:    time.Now(),
						})
					} else {
						m = m.addMessage(Message{
							Role: "assistant",
							Content: fmt.Sprintf("✓ Confirmed learning candidate %d: `%s` -> `%s` successfully!",
								id, targetCand.Phrase, targetCand.Verb),
							Time: time.Now(),
						})
					}
				}
			}
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil
	}

	if len(parts) > 1 && parts[1] == "reject" {
		if len(parts) < 3 {
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: "Usage: `/learn reject <id>`",
				Time:    time.Now(),
			})
		} else {
			var id int64
			if _, err := fmt.Sscanf(parts[2], "%d", &id); err != nil {
				m = m.addMessage(Message{
					Role:    "assistant",
					Content: "Invalid candidate ID. Must be an integer.",
					Time:    time.Now(),
				})
			} else if m.localDB == nil {
				m = m.addMessage(Message{
					Role:    "assistant",
					Content: "No local database available.",
					Time:    time.Now(),
				})
			} else {
				err := m.localDB.RejectLearningCandidate(id)
				if err != nil {
					m = m.addMessage(Message{
						Role:    "assistant",
						Content: fmt.Sprintf("Failed to reject candidate: %v", err),
						Time:    time.Now(),
					})
				} else {
					m = m.addMessage(Message{
						Role:    "assistant",
						Content: fmt.Sprintf("✓ Rejected learning candidate %d successfully.", id),
						Time:    time.Now(),
					})
				}
			}
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil
	}

	m = m.addMessage(Message{
		Role:    "assistant",
		Content: "Invoking Meta-Cognitive Supervisor (The Critic)... Analyzing recent turns for learning opportunities.",
		Time:    time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()

	return m, func() tea.Msg {
		// Trigger the Ouroboros Loop
		var traces []perception.ReasoningTrace
		for _, msg := range m.history {
			t := perception.ReasoningTrace{
				UserPrompt: "...",
				Response:   msg.Content,
				Success:    true,
			}
			if msg.Role == "user" {
				t.UserPrompt = msg.Content
			}
			traces = append(traces, t)
		}

		perception.SharedTaxonomy.SetClient(m.client)
		perception.SharedTaxonomy.SetWorkspace(m.workspace) // Ensure .nerd paths resolve correctly
		fact, err := perception.SharedTaxonomy.LearnFromInteraction(context.Background(), traces)
		if err != nil {
			return responseMsg(fmt.Sprintf("Learning failed: %v", err))
		}
		if fact == "" {
			return responseMsg("No new patterns detected in recent interactions.")
		}
		clarification, err := m.stageLearningCandidateFromFact(fact, criticManualLearnReason)
		if err != nil {
			return responseMsg(fmt.Sprintf("Learning candidate staging failed: %v", err))
		}
		return clarification
	}

}

// handleCampaignCommand handles the corresponding chat slash-command.
func (m Model) handleCampaignCommand(input string, parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 2 {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: "Usage: `/campaign <start|assault|status|pause|resume|list> [args]`",
			Time:    time.Now(),
		})
	} else {
		subCmd := parts[1]
		switch subCmd {
		case "start":
			if len(parts) < 3 {
				m = m.addMessage(Message{
					Role:    "assistant",
					Content: "Usage: `/campaign start <goal>`",
					Time:    time.Now(),
				})
			} else {
				goal := strings.Join(parts[2:], " ")
				m = m.addMessage(Message{
					Role:    "assistant",
					Content: fmt.Sprintf("Starting campaign for: %s", goal),
					Time:    time.Now(),
				})
				m.viewport.SetContent(m.renderHistory())
				m.viewport.GotoBottom()
				m.textarea.Reset()
				m.isLoading = true
				return m, tea.Batch(m.spinner.Tick, m.startCampaign(goal))
			}
		case "assault":
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: "Starting adversarial assault campaign...",
				Time:    time.Now(),
			})
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			m.textarea.Reset()
			m.isLoading = true
			return m, tea.Batch(m.spinner.Tick, m.startAssaultCampaign(parts[2:]))
		case "status":
			content := m.renderCampaignStatus()
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: content,
				Time:    time.Now(),
			})
		case "pause":
			if m.activeCampaign != nil {
				m.activeCampaign.Status = campaign.StatusPaused
				m = m.addMessage(Message{
					Role:    "assistant",
					Content: "Campaign paused.",
					Time:    time.Now(),
				})
			} else {
				m = m.addMessage(Message{
					Role:    "assistant",
					Content: "No active campaign to pause.",
					Time:    time.Now(),
				})
			}
		case "resume":
			if m.activeCampaign != nil && m.activeCampaign.Status == campaign.StatusPaused {
				m = m.addMessage(Message{
					Role:    "assistant",
					Content: "Resuming campaign...",
					Time:    time.Now(),
				})
				m.viewport.SetContent(m.renderHistory())
				m.viewport.GotoBottom()
				m.textarea.Reset()
				m.isLoading = true
				return m, tea.Batch(m.spinner.Tick, m.resumeCampaign())
			} else {
				m = m.addMessage(Message{
					Role:    "assistant",
					Content: "No paused campaign to resume.",
					Time:    time.Now(),
				})
			}
		case "list":
			content := m.renderCampaignList()
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: content,
				Time:    time.Now(),
			})
		}
	}
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil

}

// handleToolCommand handles the corresponding chat slash-command.
func (m Model) handleToolCommand(input string, parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 2 {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: "Usage: `/tool <list|run|info|generate> [args]`\n\n- `/tool list` - List all generated tools\n- `/tool run <name> <input>` - Execute a tool\n- `/tool info <name>` - Show tool details\n- `/tool generate <description>` - Generate a new tool",
			Time:    time.Now(),
		})
	} else {
		subCmd := parts[1]
		switch subCmd {
		case "list":
			content := m.renderToolList()
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: content,
				Time:    time.Now(),
			})
		case "run":
			if len(parts) < 3 {
				m = m.addMessage(Message{
					Role:    "assistant",
					Content: "Usage: `/tool run <name> [input]`",
					Time:    time.Now(),
				})
			} else {
				toolName := parts[2]
				toolInput := ""
				if len(parts) > 3 {
					toolInput = strings.Join(parts[3:], " ")
				}
				m = m.addMessage(Message{
					Role:    "assistant",
					Content: fmt.Sprintf("Executing tool `%s`...", toolName),
					Time:    time.Now(),
				})
				m.viewport.SetContent(m.renderHistory())
				m.viewport.GotoBottom()
				m.textarea.Reset()
				m.isLoading = true
				return m, tea.Batch(m.spinner.Tick, m.runTool(toolName, toolInput))
			}
		case "info":
			if len(parts) < 3 {
				m = m.addMessage(Message{
					Role:    "assistant",
					Content: "Usage: `/tool info <name>`",
					Time:    time.Now(),
				})
			} else {
				toolName := parts[2]
				content := m.renderToolInfo(toolName)
				m = m.addMessage(Message{
					Role:    "assistant",
					Content: content,
					Time:    time.Now(),
				})
			}
		case "generate":
			if len(parts) < 3 {
				m = m.addMessage(Message{
					Role:    "assistant",
					Content: "Usage: `/tool generate <description>`\n\nExample: `/tool generate a tool that validates JSON syntax`",
					Time:    time.Now(),
				})
			} else {
				description := strings.Join(parts[2:], " ")
				m = m.addMessage(Message{
					Role:    "assistant",
					Content: fmt.Sprintf("Generating tool from description: %s\n\nThis will use the Ouroboros Loop to create, compile, and register the tool.", description),
					Time:    time.Now(),
				})
				m.viewport.SetContent(m.renderHistory())
				m.viewport.GotoBottom()
				m.textarea.Reset()
				m.isLoading = true
				return m, tea.Batch(m.spinner.Tick, m.generateTool(description))
			}
		default:
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: fmt.Sprintf("Unknown tool subcommand: %s. Use list, run, info, or generate.", subCmd),
				Time:    time.Now(),
			})
		}
	}
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil

}

// handleNorthstarCommand handles the corresponding chat slash-command.
func (m Model) handleNorthstarCommand(input string, parts []string) (tea.Model, tea.Cmd) {
	// Enter Northstar definition wizard - project vision and specification
	m.setInputMode(InputModeNorthstar)

	// Check for existing northstar session
	existingWizard, hasExisting := loadExistingNorthstar(m.workspace)

	if hasExisting && existingWizard.Mission != "" {
		m.northstarWizard = existingWizard
		m.northstarWizard.Phase = NorthstarSummary // Jump to summary for review
		m.textarea.Placeholder = "resume / new / edit..."
		m = m.addMessage(Message{
			Role: "assistant",
			Content: fmt.Sprintf(`# 🌟 Existing Northstar Found

**Mission:** %s

**Problem:** %s

You have an existing Northstar definition. What would you like to do?

- **resume** - Continue from where you left off
- **new** - Start fresh (existing will be overwritten)
- **edit** - Review and edit the current definition`, existingWizard.Mission, truncateWithEllipsis(existingWizard.Problem, 100)),
			Time: time.Now(),
		})
	} else {
		m.northstarWizard = NewNorthstarWizard()
		m.textarea.Placeholder = "yes / no..."
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: getNorthstarWelcomeMessage(),
			Time:    time.Now(),
		})
	}
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil

}

// handleReviewCommand handles the corresponding chat slash-command.
func (m Model) handleReviewCommand(input string, parts []string) (tea.Model, tea.Cmd) {
	target := "."
	opts := reviewCommandOptions{}

	// Parse args for target and flags (case-insensitive)
	for _, arg := range parts[1:] {
		if strings.HasPrefix(arg, "--") {
			lower := strings.ToLower(arg)
			switch lower {
			case "--andenhance", "--and-enhance", "--enhance":
				opts.EnableEnhancement = true
			default:
				opts.PassThroughFlags = append(opts.PassThroughFlags, arg)
			}
			continue
		}

		if strings.HasPrefix(arg, "-") {
			// Preserve unknown short flags for shards to interpret
			opts.PassThroughFlags = append(opts.PassThroughFlags, arg)
			continue
		}

		// Bare argument - treat as target path
		target = arg
	}

	// Check if multi-shard review is available (has registered specialists)
	registry := m.loadAgentRegistry()
	if registry != nil && len(registry.Agents) > 0 {
		// Use multi-shard orchestrated review
		msg := fmt.Sprintf("Running multi-shard review on: %s (with specialists)", target)
		if opts.EnableEnhancement {
			msg += " with creative enhancement"
		}
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: msg,
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		m.isLoading = true
		return m, tea.Batch(m.spinner.Tick, m.spawnMultiShardReview(target, opts))
	}

	// Fallback to single ReviewerShard
	task := formatShardTask("/review", target, "", m.workspace)
	// Append --andEnhance flag if requested
	if opts.EnableEnhancement {
		task += " --andEnhance"
	}
	if len(opts.PassThroughFlags) > 0 {
		task += " " + strings.Join(opts.PassThroughFlags, " ")
	}
	msg := fmt.Sprintf("Running code review on: %s", target)
	if opts.EnableEnhancement {
		msg += " with creative enhancement (Steps 8-12)"
	}
	m = m.addMessage(Message{
		Role:    "assistant",
		Content: msg,
		Time:    time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	m.isLoading = true
	return m, tea.Batch(m.spinner.Tick, m.spawnShard("reviewer", task))

}
