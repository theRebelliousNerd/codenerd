// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains command handling for the chat interface.
package chat

import (
	"codenerd/cmd/nerd/ui"
	"codenerd/internal/campaign"
	"codenerd/internal/config"
	nerdinit "codenerd/internal/init"
	"codenerd/internal/perception"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// COMMAND HANDLING
// =============================================================================
// handleCommand processes all /command inputs from the user.
// Commands are organized by category: session, config, shard, query, campaign.

func (m Model) handleCommand(input string) (tea.Model, tea.Cmd) {
	parts := strings.Fields(input)
	cmd := parts[0]

	switch cmd {
	case "/quit", "/exit", "/q":
		return m, tea.Quit

	case "/usage":
		m.viewMode = UsageView
		m.usagePage.SetSize(m.width, m.height)
		m.usagePage.UpdateContent()
		return m, nil

	case "/clear":
		m.history = []Message{}
		m.viewport.SetContent("")
		m.textarea.Reset()
		// Save empty history
		m.saveSessionState()
		return m, nil

	case "/new-session":
		// Start a completely new session with fresh ID
		m.history = []Message{}
		m.sessionID = fmt.Sprintf("sess_%d", time.Now().UnixNano())
		m.turnCount = 0
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: fmt.Sprintf("Started new session: `%s`\n\nPrevious history saved.", m.sessionID),
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		m.saveSessionState()
		return m, nil

	case "/sessions":
		// List available sessions
		sessions, err := nerdinit.ListSessionHistories(m.workspace)
		if err != nil || len(sessions) == 0 {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: "No saved sessions found.",
				Time:    time.Now(),
			})
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			m.textarea.Reset()
			return m, nil
		}

		// Populate interactive list
		var items []list.Item
		for _, sess := range sessions {
			desc := "Session History"
			if sess == m.sessionID {
				desc = "Current Session"
			}
			// Use session ID as date for now, or parse it if it's a timestamp
			items = append(items, sessionItem{id: sess, date: sess, desc: desc})
		}

		m.list.SetItems(items)
		m.list.Title = "Select a Session to Load"
		m.viewMode = ListView // Switch to List View
		m.textarea.Reset()
		return m, nil

	case "/help":
		help := `## Available Commands

| Command | Description |
|---------|-------------|
| /help | Show this help message |
| /clear | Clear chat history |
| /new-session | Start a fresh session (preserves old) |
| /sessions | List saved sessions |
| /status | Show system status (includes tools) |
| /init | Initialize codeNERD in the workspace |
| /init --force | Reinitialize (preserves learned preferences) |
| /scan | Refresh codebase index without full reinit |
| /config wizard | Full interactive configuration dialogue |
| /config show | Show current configuration |
| /config set-key <key> | Set API key |
| /config set-theme <theme> | Set theme (light/dark) |
| /read <path> | Read file contents |
| /mkdir <path> | Create a directory |
| /write <path> <content> | Write content to file |
| /search <pattern> | Search for pattern in files |
| /patch | Enter patch ingestion mode |
| /edit <path> | Edit a file |

| /append <path> | Append to a file |
| /pick | Open file picker to read a file |
| /define-agent | Define a new specialist agent |
| /agents | List defined agents |
| /spawn <type> <task> | Spawn a shard agent |
| /legislate <constraint> | Synthesize & ratify a safety rule |
| /review [path] [--andEnhance] | Code review (--andEnhance for creative suggestions) |
| /security [path] | Security analysis |
| /analyze [path] | Complexity analysis |
| /clarify <goal> | Socratic requirements interrogation |
| /test [target] | Generate/run tests |
| /fix <issue> | Fix an issue |
| /refactor <target> | Refactor code |
| /northstar | Define project vision & specification |
| /vision | Alias for /northstar |
| /spec | Alias for /northstar |
| /query <predicate> | Query Mangle facts |
| /why <fact> | Explain why a fact was derived |
| /logic | Show logic pane content |
| /shadow | Run shadow mode simulation |
| /whatif <change> | Counterfactual query |
| /approve | Approve pending changes |
| /reject-finding <file>:<line> <reason> | Mark finding as false positive |
| /accept-finding <file>:<line> | Confirm finding is valid |
| /review-accuracy | Show review accuracy report |
| /campaign start <goal> | Start multi-phase campaign |
| /campaign status | Show campaign status |
| /campaign pause | Pause current campaign |
| /campaign resume | Resume paused campaign |
| /campaign list | List all campaigns |
| /launchcampaign <goal> | Clarify and auto-start a hands-free campaign |

### Tool Management (Autopoiesis)

| Command | Description |
|---------|-------------|
| /tool list | List all generated tools |
| /tool run <name> <input> | Execute a generated tool |
| /tool info <name> | Show details about a tool |
| /tool generate <description> | Generate a new tool via Ouroboros Loop |

Note: Tools are generated automatically when capabilities are missing,
or you can create them on-demand with /tool generate.

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| Ctrl+L | Toggle logic pane |
| Ctrl+G | Cycle pane modes |
| Ctrl+R | Toggle pane focus |
| Ctrl+P | Toggle campaign panel |
| Ctrl+C | Exit |
`
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: help,
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil

	case "/status":
		// Show system status
		status := m.buildStatusReport()
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: status,
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil

	case "/legislate":
		if len(parts) < 2 {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: "Usage: `/legislate <constraint>`\n\nExample: `/legislate Stop using fmt.Printf; use log.Info instead.`",
				Time:    time.Now(),
			})
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			m.textarea.Reset()
			return m, nil
		}
		task := strings.TrimSpace(strings.TrimPrefix(input, "/legislate"))
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: "Legislator engaged. Compiling and ratifying rule...",
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, tea.Batch(m.spinner.Tick, m.spawnShard("legislator", task))

	case "/clarify":
		if len(parts) < 2 {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: "Usage: `/clarify <goal>`\n\nExample: `/clarify build a campaign to harden auth`",
				Time:    time.Now(),
			})
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			m.textarea.Reset()
			return m, nil
		}
		task := strings.TrimSpace(strings.TrimPrefix(input, "/clarify"))
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: "Requirements Interrogator engaged. Drafting clarifying questions...",
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, tea.Batch(m.spinner.Tick, m.spawnShard("requirements_interrogator", task))

	case "/launchcampaign":
		if len(parts) < 2 {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: "Usage: `/launchcampaign <goal>`\n\nThis will run clarifications, then auto-start a campaign hands-free if possible.",
				Time:    time.Now(),
			})
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			m.textarea.Reset()
			return m, nil
		}
		goal := strings.TrimSpace(strings.TrimPrefix(input, "/launchcampaign"))
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: "Launching auto-campaign: running clarifier and then starting the campaign...",
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, tea.Batch(m.spinner.Tick, m.runLaunchCampaign(goal))

	case "/init":
		// Check for --force flag
		forceInit := false
		for _, part := range parts[1:] {
			if part == "--force" || part == "-f" {
				forceInit = true
				break
			}
		}

		// Check if already initialized and not forcing
		if nerdinit.IsInitialized(m.workspace) && !forceInit {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: "Workspace already initialized. Use `/init --force` to reinitialize.",
				Time:    time.Now(),
			})
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			m.textarea.Reset()
			return m, nil
		}

		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: "Initializing codeNERD... This may take a few minutes for research and agent creation.",
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		m.isLoading = true
		return m, tea.Batch(m.spinner.Tick, m.runInitialization(forceInit))
	case "/scan":
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: "Scanning workspace...",
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		m.isLoading = true
		return m, tea.Batch(m.spinner.Tick, m.runScan())

	case "/scan-path":
		if len(parts) < 2 {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: "Usage: /scan-path <file1>[,<file2>...]",
				Time:    time.Now(),
			})
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			m.textarea.Reset()
			return m, nil
		}
		targets := strings.Split(parts[1], ",")
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: fmt.Sprintf("Scanning %d path(s)...", len(targets)),
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		m.isLoading = true
		return m, tea.Batch(m.spinner.Tick, m.runPartialScan(targets))

	case "/scan-dir":
		if len(parts) < 2 {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: "Usage: /scan-dir <directory>",
				Time:    time.Now(),
			})
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			m.textarea.Reset()
			return m, nil
		}
		dir := parts[1]
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: fmt.Sprintf("Scanning directory: %s", dir),
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		m.isLoading = true
		return m, tea.Batch(m.spinner.Tick, m.runDirScan(dir))

	case "/config":
		if len(parts) < 2 {
			m.history = append(m.history, Message{
				Role: "assistant",
				Content: `Configuration commands:

| Command | Description |
|---------|-------------|
| /config wizard | Full interactive configuration dialogue |
| /config set-key <key> | Set API key |
| /config set-theme <theme> | Set theme (light/dark) |
| /config engine [api\|claude-cli\|codex-cli] | Set LLM engine |
| /config show | Show current configuration |`,
				Time: time.Now(),
			})
		} else if parts[1] == "wizard" {
			// Enter config wizard mode
			m.awaitingConfigWizard = true
			m.configWizard = NewConfigWizard()
			m.textarea.Placeholder = "Press Enter to start..."
			m.history = append(m.history, Message{
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
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: content,
				Time:    time.Now(),
			})
		} else if parts[1] == "set-key" {
			// API keys are now provider-specific - guide user to use wizard or edit config directly
			m.history = append(m.history, Message{
				Role:    "assistant",
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
				_ = cfg.Save(config.DefaultUserConfigPath())
				// Apply theme
				if theme == "dark" {
					m.styles = ui.NewStyles(ui.DarkTheme())
				} else {
					m.styles = ui.DefaultStyles()
				}
				m.history = append(m.history, Message{
					Role:    "assistant",
					Content: fmt.Sprintf("Theme set to: %s", theme),
					Time:    time.Now(),
				})
			} else {
				m.history = append(m.history, Message{
					Role:    "assistant",
					Content: "Invalid theme. Use 'light' or 'dark'.",
					Time:    time.Now(),
				})
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
					engineDesc = fmt.Sprintf("**Codex CLI** (model: %s, sandbox: %s, timeout: %ds)", cliCfg.Model, cliCfg.Sandbox, cliCfg.Timeout)
				default:
					provider, _ := cfg.GetActiveProvider()
					engineDesc = fmt.Sprintf("**API** (provider: %s)", provider)
				}
				m.history = append(m.history, Message{
					Role:    "assistant",
					Content: fmt.Sprintf("Current engine: %s\n\n%s\n\nAvailable engines:\n- `api` - HTTP API (default)\n- `claude-cli` - Claude Code CLI (subscription)\n- `codex-cli` - Codex CLI (ChatGPT subscription)", engine, engineDesc),
					Time:    time.Now(),
				})
			} else {
				// Set engine
				newEngine := parts[2]
				if err := cfg.SetEngine(newEngine); err != nil {
					m.history = append(m.history, Message{
						Role:    "assistant",
						Content: fmt.Sprintf("Error: %s", err.Error()),
						Time:    time.Now(),
					})
				} else {
					if err := cfg.Save(config.DefaultUserConfigPath()); err != nil {
						m.history = append(m.history, Message{
							Role:    "assistant",
							Content: fmt.Sprintf("Error saving config: %s", err.Error()),
							Time:    time.Now(),
						})
					} else {
						m.Config = cfg
						m.history = append(m.history, Message{
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

	case "/embedding":
		if len(parts) < 2 {
			m.history = append(m.history, Message{
				Role: "assistant",
				Content: `Embedding commands:
  /embedding set <provider> [api-key]  - Set embedding provider (ollama or genai)
  /embedding stats                      - Show embedding statistics
  /embedding reembed                    - Re-generate all embeddings`,
				Time: time.Now(),
			})
		} else {
			switch parts[1] {
			case "set":
				if len(parts) < 3 {
					m.history = append(m.history, Message{
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
						cfg.Embedding.OllamaModel = "embeddinggemma"
					} else if provider == "genai" && len(parts) >= 4 {
						cfg.Embedding.GenAIAPIKey = parts[3]
						cfg.Embedding.GenAIModel = "gemini-embedding-001"
					}
					if err := cfg.Save(config.DefaultUserConfigPath()); err != nil {
						m.history = append(m.history, Message{
							Role:    "assistant",
							Content: fmt.Sprintf("Failed to save config: %v", err),
							Time:    time.Now(),
						})
					} else {
						m.history = append(m.history, Message{
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
						m.history = append(m.history, Message{
							Role:    "assistant",
							Content: fmt.Sprintf("Failed to get stats: %v", err),
							Time:    time.Now(),
						})
					} else {
						m.history = append(m.history, Message{
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
					m.history = append(m.history, Message{
						Role:    "assistant",
						Content: "No knowledge database available.",
						Time:    time.Now(),
					})
				}
			case "reembed":
				if m.localDB != nil {
					m.history = append(m.history, Message{
						Role:    "assistant",
						Content: "Re-embedding all vectors... (this may take a moment)",
						Time:    time.Now(),
					})
					ctx := context.Background()
					if err := m.localDB.ReembedAllVectors(ctx); err != nil {
						m.history = append(m.history, Message{
							Role:    "assistant",
							Content: fmt.Sprintf("Re-embedding failed: %v", err),
							Time:    time.Now(),
						})
					} else {
						stats, _ := m.localDB.GetVectorStats()
						m.history = append(m.history, Message{
							Role:    "assistant",
							Content: fmt.Sprintf("✓ Re-embedding complete! Vectors with embeddings: %v", stats["with_embeddings"]),
							Time:    time.Now(),
						})
					}
				} else {
					m.history = append(m.history, Message{
						Role:    "assistant",
						Content: "No knowledge database available.",
						Time:    time.Now(),
					})
				}
			default:
				m.history = append(m.history, Message{
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

	case "/read":
		if len(parts) < 2 {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: "Usage: `/read <path>`",
				Time:    time.Now(),
			})
		} else {
			path := parts[1]
			content, err := readFileContent(m.workspace, path, 16000)
			if err != nil {
				m.history = append(m.history, Message{
					Role:    "assistant",
					Content: fmt.Sprintf("Failed to read file: %v", err),
					Time:    time.Now(),
				})
			} else {
				m.history = append(m.history, Message{
					Role:    "assistant",
					Content: fmt.Sprintf("**Contents of %s:**\n\n```\n%s\n```", path, content),
					Time:    time.Now(),
				})
			}
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil

	case "/mkdir":
		if len(parts) < 2 {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: "Usage: `/mkdir <path>`",
				Time:    time.Now(),
			})
		} else {
			path := parts[1]
			if err := makeDir(m.workspace, path); err != nil {
				m.history = append(m.history, Message{
					Role:    "assistant",
					Content: fmt.Sprintf("Failed to create directory: %v", err),
					Time:    time.Now(),
				})
			} else {
				m.history = append(m.history, Message{
					Role:    "assistant",
					Content: fmt.Sprintf("Created directory: %s", path),
					Time:    time.Now(),
				})
			}
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil

	case "/write":
		if len(parts) < 3 {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: "Usage: `/write <path> <content>`",
				Time:    time.Now(),
			})
		} else {
			path := parts[1]
			content := strings.Join(parts[2:], " ")
			if err := writeFileContent(m.workspace, path, content); err != nil {
				m.history = append(m.history, Message{
					Role:    "assistant",
					Content: fmt.Sprintf("Failed to write file: %v", err),
					Time:    time.Now(),
				})
			} else {
				m.history = append(m.history, Message{
					Role:    "assistant",
					Content: fmt.Sprintf("Wrote to file: %s", path),
					Time:    time.Now(),
				})
			}
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil

	case "/search":
		if len(parts) < 2 {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: "Usage: `/search <pattern>`",
				Time:    time.Now(),
			})
		} else {
			pattern := strings.Join(parts[1:], " ")
			matches, err := searchInFiles(m.workspace, pattern, 20)
			if err != nil {
				m.history = append(m.history, Message{
					Role:    "assistant",
					Content: fmt.Sprintf("Search failed: %v", err),
					Time:    time.Now(),
				})
			} else if len(matches) == 0 {
				m.history = append(m.history, Message{
					Role:    "assistant",
					Content: fmt.Sprintf("No matches found for: %s", pattern),
					Time:    time.Now(),
				})
			} else {
				var sb strings.Builder
				sb.WriteString(fmt.Sprintf("**Found %d matches for '%s':**\n\n", len(matches), pattern))
				for _, match := range matches {
					sb.WriteString(fmt.Sprintf("- %s\n", match))
				}
				m.history = append(m.history, Message{
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

	case "/patch":
		m.awaitingPatch = true
		m.pendingPatchLines = nil
		m.textarea.Placeholder = "Paste patch lines (type --END-- when done)..."
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: "Patch mode enabled. Paste your patch line by line, then type `--END--` to apply.",
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil

	case "/edit":
		if len(parts) < 2 {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: "Usage: `/edit <path>` - Opens file for inline editing",
				Time:    time.Now(),
			})
		} else {
			path := parts[1]
			content, err := readFileContent(m.workspace, path, 16000)
			if err != nil {
				m.history = append(m.history, Message{
					Role:    "assistant",
					Content: fmt.Sprintf("Failed to read file for editing: %v", err),
					Time:    time.Now(),
				})
			} else {
				m.history = append(m.history, Message{
					Role:    "assistant",
					Content: fmt.Sprintf("**Editing %s:**\n\n```\n%s\n```\n\nUse `/write %s <new content>` to save changes.", path, content, path),
					Time:    time.Now(),
				})
			}
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil

	case "/append":
		if len(parts) < 3 {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: "Usage: `/append <path> <content>`",
				Time:    time.Now(),
			})
		} else {
			path := parts[1]
			content := strings.Join(parts[2:], " ")
			if err := appendFileContent(m.workspace, path, content+"\n"); err != nil {
				m.history = append(m.history, Message{
					Role:    "assistant",
					Content: fmt.Sprintf("Failed to append to file: %v", err),
					Time:    time.Now(),
				})
			} else {
				m.history = append(m.history, Message{
					Role:    "assistant",
					Content: fmt.Sprintf("Appended to file: %s", path),
					Time:    time.Now(),
				})
			}
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil

	case "/pick":
		m.viewMode = FilePickerView
		m.textarea.Reset()
		return m, m.filepicker.Init()

	case "/define-agent", "/agent":
		// Enter agent definition wizard
		m.awaitingAgentDefinition = true
		m.agentWizard = &AgentWizardState{Step: 0} // Start at step 0 (Name)
		m.textarea.Placeholder = "Enter agent name (e.g., 'RustExpert')..."
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: "**Agent Creation Wizard**\n\nLet's define a new specialist agent.\n\n**Step 1:** What should we name this agent? (Alphanumeric, e.g., `RustExpert`, `SecurityAuditor`)",
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil

	case "/northstar", "/vision", "/spec":
		// Enter Northstar definition wizard - project vision and specification
		m.awaitingNorthstar = true

		// Check for existing northstar session
		existingWizard, hasExisting := loadExistingNorthstar(m.workspace)

		if hasExisting && existingWizard.Mission != "" {
			m.northstarWizard = existingWizard
			m.northstarWizard.Phase = NorthstarSummary // Jump to summary for review
			m.textarea.Placeholder = "resume / new / edit..."
			m.history = append(m.history, Message{
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
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: getNorthstarWelcomeMessage(),
				Time:    time.Now(),
			})
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil

	case "/learn":
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: "Invoking Meta-Cognitive Supervisor (The Critic)... Analyzing recent turns for learning opportunities.",
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()

		return m, func() tea.Msg {
			// Trigger the Ouroboros Loop
			// We need to fetch recent traces. Since we don't have direct access to ReasoningTraces in the Model struct
			// (we only have Message history), we should ideally rely on the TraceStore if available.
			// But for now, we can construct traces from the session history or rely on the TaxonomyEngine's access if we passed it in.

			// Actually, perception.SharedTaxonomy is available globally.
			// And we have m.client.

			// We need to convert m.history (Message) to perception.ReasoningTrace for the learner
			var traces []perception.ReasoningTrace
			for _, msg := range m.history {
				// Convert TUI message to Trace format (simplified)
				t := perception.ReasoningTrace{
					UserPrompt: "...", // We don't have perfect mapping here, simplified for now
					Response:   msg.Content,
					Success:    true, // Assumed
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
			return responseMsg(fmt.Sprintf("Successfully learned and crystallized new pattern:\n```\n%s\n```", fact))
		}

	case "/agents":
		// List defined agents
		agents := m.loadType3Agents()
		if len(agents) == 0 {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: "No agents defined yet. Use `/define-agent` to create one, or run `/init` to auto-create agents.",
				Time:    time.Now(),
			})
		} else {
			var sb strings.Builder
			sb.WriteString("## Defined Agents\n\n")
			sb.WriteString("| Name | Type | KB Size | Status |\n")
			sb.WriteString("|------|------|---------|--------|\n")
			for _, agent := range agents {
				sb.WriteString(fmt.Sprintf("| %s | %s | %d | %s |\n", agent.Name, agent.Type, agent.KBSize, agent.Status))
			}
			sb.WriteString("\n*Use `/spawn <name> <task>` to spawn an agent*")
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: sb.String(),
				Time:    time.Now(),
			})
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil

	case "/spawn":
		if len(parts) < 3 {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: "Usage: `/spawn <type> <task>`\n\nTypes: coder, researcher, reviewer, tester, or a defined agent name",
				Time:    time.Now(),
			})
		} else {
			shardType := parts[1]
			task := strings.Join(parts[2:], " ")
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: fmt.Sprintf("Spawning %s shard for: %s", shardType, task),
				Time:    time.Now(),
			})
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			m.textarea.Reset()
			m.isLoading = true
			return m, tea.Batch(m.spinner.Tick, m.spawnShard(shardType, task))
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil

	case "/review":
		target := "."
		enableEnhancement := false

		// Parse args for target and --andEnhance flag
		for _, arg := range parts[1:] {
			if arg == "--andEnhance" || arg == "--enhance" {
				enableEnhancement = true
			} else if !strings.HasPrefix(arg, "-") {
				target = arg
			}
		}

		// Check if multi-shard review is available (has registered specialists)
		registry := m.loadAgentRegistry()
		if registry != nil && len(registry.Agents) > 0 {
			// Use multi-shard orchestrated review
			msg := fmt.Sprintf("Running multi-shard review on: %s (with specialists)", target)
			if enableEnhancement {
				msg += " with creative enhancement"
			}
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: msg,
				Time:    time.Now(),
			})
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			m.textarea.Reset()
			m.isLoading = true
			return m, tea.Batch(m.spinner.Tick, m.spawnMultiShardReview(target))
		}

		// Fallback to single ReviewerShard
		task := formatShardTask("/review", target, "", m.workspace)
		// Append --andEnhance flag if requested
		if enableEnhancement {
			task += " --andEnhance"
		}
		msg := fmt.Sprintf("Running code review on: %s", target)
		if enableEnhancement {
			msg += " with creative enhancement (Steps 8-12)"
		}
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: msg,
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		m.isLoading = true
		return m, tea.Batch(m.spinner.Tick, m.spawnShard("reviewer", task))

	case "/security":
		target := "."
		if len(parts) > 1 {
			target = parts[1]
		}
		task := formatShardTask("/security", target, "", m.workspace)
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: fmt.Sprintf("Running security analysis on: %s", target),
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		m.isLoading = true
		return m, tea.Batch(m.spinner.Tick, m.spawnShard("reviewer", task))

	case "/analyze":
		target := "."
		if len(parts) > 1 {
			target = parts[1]
		}
		task := formatShardTask("/analyze", target, "", m.workspace)
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: fmt.Sprintf("Running complexity analysis on: %s", target),
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		m.isLoading = true
		return m, tea.Batch(m.spinner.Tick, m.spawnShard("reviewer", task))

	case "/test":
		target := "run"
		if len(parts) > 1 {
			target = strings.Join(parts[1:], " ")
		}
		task := formatShardTask("/test", target, "", m.workspace)
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: fmt.Sprintf("Running test task: %s", task),
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		m.isLoading = true
		return m, tea.Batch(m.spinner.Tick, m.spawnShard("tester", task))

	case "/fix":
		if len(parts) < 2 {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: "Usage: `/fix <issue description>`",
				Time:    time.Now(),
			})
		} else {
			target := strings.Join(parts[1:], " ")
			task := formatShardTask("/fix", target, "", m.workspace)
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: fmt.Sprintf("Attempting to fix: %s", target),
				Time:    time.Now(),
			})
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			m.textarea.Reset()
			m.isLoading = true
			return m, tea.Batch(m.spinner.Tick, m.spawnShard("coder", task))
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil

	case "/refactor":
		if len(parts) < 2 {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: "Usage: `/refactor <target>`",
				Time:    time.Now(),
			})
		} else {
			target := strings.Join(parts[1:], " ")
			task := formatShardTask("/refactor", target, "", m.workspace)
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: fmt.Sprintf("Refactoring: %s", target),
				Time:    time.Now(),
			})
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			m.textarea.Reset()
			m.isLoading = true
			return m, tea.Batch(m.spinner.Tick, m.spawnShard("coder", task))
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil

	case "/query":
		if len(parts) < 2 {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: "Usage: `/query <predicate>`",
				Time:    time.Now(),
			})
		} else {
			predicate := parts[1]
			facts, err := m.kernel.Query(predicate)
			if err != nil {
				m.history = append(m.history, Message{
					Role:    "assistant",
					Content: fmt.Sprintf("Query failed: %v", err),
					Time:    time.Now(),
				})
			} else if len(facts) == 0 {
				m.history = append(m.history, Message{
					Role:    "assistant",
					Content: fmt.Sprintf("No facts found for predicate: %s", predicate),
					Time:    time.Now(),
				})
			} else {
				var sb strings.Builder
				sb.WriteString(fmt.Sprintf("**Query results for `%s`:**\n\n", predicate))
				for _, fact := range facts {
					sb.WriteString(fmt.Sprintf("- %s\n", fact.String()))
				}
				m.history = append(m.history, Message{
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

	case "/why":
		if len(parts) < 2 {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: "Usage: `/why <fact>` - Explains why a fact was derived",
				Time:    time.Now(),
			})
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			m.textarea.Reset()
			return m, nil
		} else {
			fact := strings.Join(parts[1:], " ")
			// Use fetchTrace to populate the logic pane
			return m, m.fetchTrace(fact)
		}

	case "/logic":
		// Show current logic pane content
		var sb strings.Builder
		sb.WriteString("## Logic Pane Content\n\n")

		// Get recent facts
		facts, _ := m.kernel.Query("*")
		if len(facts) > 0 {
			sb.WriteString("### Recent Facts\n\n")
			for i, fact := range facts {
				if i >= 20 {
					sb.WriteString(fmt.Sprintf("... and %d more\n", len(facts)-20))
					break
				}
				sb.WriteString(fmt.Sprintf("- %s\n", fact.String()))
			}
		}

		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: sb.String(),
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil

	case "/shadow":
		if len(parts) < 2 {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: "Usage: `/shadow <action>` - Run a shadow mode simulation",
				Time:    time.Now(),
			})
		} else {
			action := strings.Join(parts[1:], " ")
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: fmt.Sprintf("Running shadow simulation for: %s", action),
				Time:    time.Now(),
			})
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			m.textarea.Reset()
			m.isLoading = true
			return m, tea.Batch(m.spinner.Tick, m.runShadowSimulation(action))
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil

	case "/whatif":
		if len(parts) < 2 {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: "Usage: `/whatif <change>` - Run a counterfactual query",
				Time:    time.Now(),
			})
		} else {
			change := strings.Join(parts[1:], " ")
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: fmt.Sprintf("Running counterfactual analysis for: %s", change),
				Time:    time.Now(),
			})
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			m.textarea.Reset()
			m.isLoading = true
			return m, tea.Batch(m.spinner.Tick, m.runWhatIfQuery(change))
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil

	case "/approve":
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: "Approval noted. Proceeding with pending changes.",
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil

	case "/reject-finding":
		// Reviewer feedback: mark a finding as false positive
		// Usage: /reject-finding <file>:<line> <reason>
		if len(parts) < 3 {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: "Usage: `/reject-finding <file>:<line> <reason>`\nExample: `/reject-finding internal/core/kernel.go:42 function exists in sibling file`",
				Time:    time.Now(),
			})
		} else {
			location := parts[1]
			reason := strings.Join(parts[2:], " ")

			// Parse file:line
			colonIdx := strings.LastIndex(location, ":")
			if colonIdx == -1 {
				m.history = append(m.history, Message{
					Role:    "assistant",
					Content: "Invalid format. Use `<file>:<line>` (e.g., `kernel.go:42`)",
					Time:    time.Now(),
				})
			} else {
				file := location[:colonIdx]
				lineStr := location[colonIdx+1:]
				var line int
				fmt.Sscanf(lineStr, "%d", &line)

				// Use lastShardResult to get review ID (generate from turn number)
				reviewID := "unknown"
				if m.lastShardResult != nil && m.lastShardResult.ShardType == "reviewer" {
					reviewID = fmt.Sprintf("review-%d-%d", m.lastShardResult.TurnNumber, m.lastShardResult.Timestamp.Unix())
				}

				// Record the rejection
				m.shardMgr.RejectReviewFinding(reviewID, file, line, reason)

				m.history = append(m.history, Message{
					Role:    "assistant",
					Content: fmt.Sprintf("✓ Rejected finding at `%s:%d`\nReason: %s\n\nThe system will learn from this feedback to avoid similar false positives.", file, line, reason),
					Time:    time.Now(),
				})
			}
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil

	case "/accept-finding":
		// Reviewer feedback: confirm a finding is valid
		// Usage: /accept-finding <file>:<line>
		if len(parts) < 2 {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: "Usage: `/accept-finding <file>:<line>`\nExample: `/accept-finding internal/core/kernel.go:42`",
				Time:    time.Now(),
			})
		} else {
			location := parts[1]

			// Parse file:line
			colonIdx := strings.LastIndex(location, ":")
			if colonIdx == -1 {
				m.history = append(m.history, Message{
					Role:    "assistant",
					Content: "Invalid format. Use `<file>:<line>` (e.g., `kernel.go:42`)",
					Time:    time.Now(),
				})
			} else {
				file := location[:colonIdx]
				lineStr := location[colonIdx+1:]
				var line int
				fmt.Sscanf(lineStr, "%d", &line)

				// Use lastShardResult to get review ID (generate from turn number)
				reviewID := "unknown"
				if m.lastShardResult != nil && m.lastShardResult.ShardType == "reviewer" {
					reviewID = fmt.Sprintf("review-%d-%d", m.lastShardResult.TurnNumber, m.lastShardResult.Timestamp.Unix())
				}

				// Record the acceptance
				m.shardMgr.AcceptReviewFinding(reviewID, file, line)

				m.history = append(m.history, Message{
					Role:    "assistant",
					Content: fmt.Sprintf("✓ Accepted finding at `%s:%d`\n\nThis helps validate the reviewer's accuracy.", file, line),
					Time:    time.Now(),
				})
			}
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil

	case "/review-accuracy":
		// Show accuracy report for the last review
		reviewID := "unknown"
		if m.lastShardResult != nil && m.lastShardResult.ShardType == "reviewer" {
			reviewID = fmt.Sprintf("review-%d-%d", m.lastShardResult.TurnNumber, m.lastShardResult.Timestamp.Unix())
		}
		report := m.shardMgr.GetReviewAccuracyReport(reviewID)
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: fmt.Sprintf("## Review Accuracy Report\n\n%s", report),
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil

	case "/campaign":
		if len(parts) < 2 {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: "Usage: `/campaign <start|status|pause|resume|list> [args]`",
				Time:    time.Now(),
			})
		} else {
			subCmd := parts[1]
			switch subCmd {
			case "start":
				if len(parts) < 3 {
					m.history = append(m.history, Message{
						Role:    "assistant",
						Content: "Usage: `/campaign start <goal>`",
						Time:    time.Now(),
					})
				} else {
					goal := strings.Join(parts[2:], " ")
					m.history = append(m.history, Message{
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
			case "status":
				content := m.renderCampaignStatus()
				m.history = append(m.history, Message{
					Role:    "assistant",
					Content: content,
					Time:    time.Now(),
				})
			case "pause":
				if m.activeCampaign != nil {
					m.activeCampaign.Status = campaign.StatusPaused
					m.history = append(m.history, Message{
						Role:    "assistant",
						Content: "Campaign paused.",
						Time:    time.Now(),
					})
				} else {
					m.history = append(m.history, Message{
						Role:    "assistant",
						Content: "No active campaign to pause.",
						Time:    time.Now(),
					})
				}
			case "resume":
				if m.activeCampaign != nil && m.activeCampaign.Status == campaign.StatusPaused {
					m.history = append(m.history, Message{
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
					m.history = append(m.history, Message{
						Role:    "assistant",
						Content: "No paused campaign to resume.",
						Time:    time.Now(),
					})
				}
			case "list":
				content := m.renderCampaignList()
				m.history = append(m.history, Message{
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

	case "/tool":
		if len(parts) < 2 {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: "Usage: `/tool <list|run|info|generate> [args]`\n\n- `/tool list` - List all generated tools\n- `/tool run <name> <input>` - Execute a tool\n- `/tool info <name>` - Show tool details\n- `/tool generate <description>` - Generate a new tool",
				Time:    time.Now(),
			})
		} else {
			subCmd := parts[1]
			switch subCmd {
			case "list":
				content := m.renderToolList()
				m.history = append(m.history, Message{
					Role:    "assistant",
					Content: content,
					Time:    time.Now(),
				})
			case "run":
				if len(parts) < 3 {
					m.history = append(m.history, Message{
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
					m.history = append(m.history, Message{
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
					m.history = append(m.history, Message{
						Role:    "assistant",
						Content: "Usage: `/tool info <name>`",
						Time:    time.Now(),
					})
				} else {
					toolName := parts[2]
					content := m.renderToolInfo(toolName)
					m.history = append(m.history, Message{
						Role:    "assistant",
						Content: content,
						Time:    time.Now(),
					})
				}
			case "generate":
				if len(parts) < 3 {
					m.history = append(m.history, Message{
						Role:    "assistant",
						Content: "Usage: `/tool generate <description>`\n\nExample: `/tool generate a tool that validates JSON syntax`",
						Time:    time.Now(),
					})
				} else {
					description := strings.Join(parts[2:], " ")
					m.history = append(m.history, Message{
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
				m.history = append(m.history, Message{
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

	case "/jit":
		// JIT Prompt Compiler inspector
		content := m.renderJITStatus()
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: content,
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil

	default:
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: fmt.Sprintf("Unknown command: %s. Type `/help` for available commands.", cmd),
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil
	}
}

// buildStatusReport builds a status report for /status command
func (m Model) buildStatusReport() string {
	var sb strings.Builder
	sb.WriteString("## System Status\n\n")

	sb.WriteString("### Session\n")
	sb.WriteString(fmt.Sprintf("- Session ID: `%s`\n", m.sessionID))
	sb.WriteString(fmt.Sprintf("- Turn Count: %d\n", m.turnCount))
	sb.WriteString(fmt.Sprintf("- Workspace: %s\n", m.workspace))
	sb.WriteString("\n")

	sb.WriteString("### Components\n")
	sb.WriteString("- Kernel: Active\n")
	sb.WriteString("- Transducer: Active\n")
	sb.WriteString("- Shard Manager: Active\n")
	sb.WriteString("- Dreamer: Precog safety enabled\n")
	sb.WriteString("- Legislator: Available via `/legislate`\n")
	sb.WriteString("- Requirements Interrogator: Available via `/clarify`\n")
	if m.activeCampaign != nil {
		sb.WriteString(fmt.Sprintf("- Active Campaign: %s\n", m.activeCampaign.Goal))
	}
	if m.autopoiesis != nil {
		sb.WriteString("- Autopoiesis: Active\n")
	}
	sb.WriteString("\n")

	// Query fact counts
	facts, _ := m.kernel.Query("*")
	sb.WriteString("### Kernel State\n")
	sb.WriteString(fmt.Sprintf("- Total Facts: %d\n", len(facts)))

	// List registered shards
	sb.WriteString("\n### Registered Shards\n")
	sb.WriteString("- coder\n")
	sb.WriteString("- reviewer\n")
	sb.WriteString("- tester\n")
	sb.WriteString("- researcher\n")
	sb.WriteString("- legislator\n")
	sb.WriteString("- requirements_interrogator\n")

	// List generated tools
	if m.autopoiesis != nil {
		tools := m.autopoiesis.ListTools()
		sb.WriteString("\n### Generated Tools\n")
		if len(tools) == 0 {
			sb.WriteString("- No tools generated yet\n")
			sb.WriteString("- Tools are created on-demand when capabilities are missing\n")
			sb.WriteString("- Use `/tool generate <description>` to create a tool\n")
		} else {
			sb.WriteString(fmt.Sprintf("- Total Tools: %d\n", len(tools)))
			sb.WriteString("- Recent Tools:\n")
			count := 0
			for _, tool := range tools {
				if count >= 5 {
					sb.WriteString(fmt.Sprintf("  ... and %d more (use `/tool list` for full list)\n", len(tools)-5))
					break
				}
				sb.WriteString(fmt.Sprintf("  - `%s`: %d executions\n", tool.Name, tool.ExecuteCount))
				count++
			}
		}
	}

	return sb.String()
}
