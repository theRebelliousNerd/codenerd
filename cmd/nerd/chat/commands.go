// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains command handling for the chat interface.
package chat

import (
	"codenerd/cmd/nerd/config"
	"codenerd/cmd/nerd/ui"
	"codenerd/internal/campaign"
	nerdinit "codenerd/internal/init"
	"codenerd/internal/perception"
	"context"
	"fmt"
	"strings"
	"time"

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

	case "/clear":
		m.history = []Message{}
		m.viewport.SetContent("")
		m.textinput.Reset()
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
		m.textinput.Reset()
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
		} else {
			var sb strings.Builder
			sb.WriteString("## Saved Sessions\n\n")
			for _, sess := range sessions {
				current := ""
				if sess == m.sessionID {
					current = " *(current)*"
				}
				sb.WriteString(fmt.Sprintf("- `%s`%s\n", sess, current))
			}
			sb.WriteString("\n*Use `/load-session <id>` to restore a session*")
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: sb.String(),
				Time:    time.Now(),
			})
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textinput.Reset()
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
| /define-agent | Define a new specialist agent |
| /agents | List defined agents |
| /spawn <type> <task> | Spawn a shard agent |
| /legislate <constraint> | Synthesize & ratify a safety rule |
| /review [path] | Code review (current dir or specified) |
| /security [path] | Security analysis |
| /analyze [path] | Complexity analysis |
| /test [target] | Generate/run tests |
| /fix <issue> | Fix an issue |
| /refactor <target> | Refactor code |
| /query <predicate> | Query Mangle facts |
| /why <fact> | Explain why a fact was derived |
| /logic | Show logic pane content |
| /shadow | Run shadow mode simulation |
| /whatif <change> | Counterfactual query |
| /approve | Approve pending changes |
| /campaign start <goal> | Start multi-phase campaign |
| /campaign status | Show campaign status |
| /campaign pause | Pause current campaign |
| /campaign resume | Resume paused campaign |
| /campaign list | List all campaigns |

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
		m.textinput.Reset()
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
		m.textinput.Reset()
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
			m.textinput.Reset()
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
		m.textinput.Reset()
		return m, tea.Batch(m.spinner.Tick, m.spawnShard("legislator", task))

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
			m.textinput.Reset()
			return m, nil
		}

		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: "Initializing codeNERD... This may take a few minutes for research and agent creation.",
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textinput.Reset()
		m.isLoading = true
		return m, tea.Batch(m.spinner.Tick, m.runInit())

	case "/scan":
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: "Scanning workspace...",
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textinput.Reset()
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
			m.textinput.Reset()
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
		m.textinput.Reset()
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
			m.textinput.Reset()
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
		m.textinput.Reset()
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
| /config show | Show current configuration |`,
				Time: time.Now(),
			})
		} else if parts[1] == "wizard" {
			// Enter config wizard mode
			m.awaitingConfigWizard = true
			m.configWizard = NewConfigWizard()
			m.textinput.Placeholder = "Press Enter to start..."
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
		} else if parts[1] == "set-key" && len(parts) >= 3 {
			newKey := parts[2]
			m.Config.APIKey = newKey
			// Load current config, update API key, and save
			cfg, _ := config.Load()
			cfg.APIKey = newKey
			if err := config.Save(cfg); err != nil {
				m.history = append(m.history, Message{
					Role:    "assistant",
					Content: fmt.Sprintf("Failed to save API key: %v", err),
					Time:    time.Now(),
				})
			} else {
				m.history = append(m.history, Message{
					Role:    "assistant",
					Content: "API key updated successfully.",
					Time:    time.Now(),
				})
			}
		} else if parts[1] == "set-theme" && len(parts) >= 3 {
			theme := parts[2]
			if theme == "dark" || theme == "light" {
				m.Config.Theme = theme
				// Load current config, update theme, and save
				cfg, _ := config.Load()
				cfg.Theme = theme
				_ = config.Save(cfg)
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
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textinput.Reset()
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
					cfg, _ := config.Load()
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
					if err := config.Save(cfg); err != nil {
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
		m.textinput.Reset()
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
		m.textinput.Reset()
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
		m.textinput.Reset()
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
		m.textinput.Reset()
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
		m.textinput.Reset()
		return m, nil

	case "/patch":
		m.awaitingPatch = true
		m.pendingPatchLines = nil
		m.textinput.Placeholder = "Paste patch lines (type --END-- when done)..."
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: "Patch mode enabled. Paste your patch line by line, then type `--END--` to apply.",
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textinput.Reset()
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
		m.textinput.Reset()
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
		m.textinput.Reset()
		return m, nil

	case "/define-agent", "/agent":
		// Enter agent definition wizard
		m.awaitingAgentDefinition = true
		m.agentWizard = &AgentWizardState{Step: 0} // Start at step 0 (Name)
		m.textinput.Placeholder = "Enter agent name (e.g., 'RustExpert')..."
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: "**Agent Creation Wizard**\n\nLet's define a new specialist agent.\n\n**Step 1:** What should we name this agent? (Alphanumeric, e.g., `RustExpert`, `SecurityAuditor`)",
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textinput.Reset()
		return m, nil

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
		m.textinput.Reset()
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
			m.textinput.Reset()
			m.isLoading = true
			return m, tea.Batch(m.spinner.Tick, m.spawnShard(shardType, task))
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textinput.Reset()
		return m, nil

	case "/review":
		target := "."
		if len(parts) > 1 {
			target = parts[1]
		}
		task := formatShardTask("/review", target, "", m.workspace)
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: fmt.Sprintf("Running code review on: %s", target),
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textinput.Reset()
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
		m.textinput.Reset()
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
		m.textinput.Reset()
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
		m.textinput.Reset()
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
			m.textinput.Reset()
			m.isLoading = true
			return m, tea.Batch(m.spinner.Tick, m.spawnShard("coder", task))
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textinput.Reset()
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
			m.textinput.Reset()
			m.isLoading = true
			return m, tea.Batch(m.spinner.Tick, m.spawnShard("coder", task))
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textinput.Reset()
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
		m.textinput.Reset()
		return m, nil

	case "/why":
		if len(parts) < 2 {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: "Usage: `/why <fact>` - Explains why a fact was derived",
				Time:    time.Now(),
			})
		} else {
			fact := strings.Join(parts[1:], " ")
			explanation := m.buildDerivationTrace(fact)
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: explanation,
				Time:    time.Now(),
			})
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textinput.Reset()
		return m, nil

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
		m.textinput.Reset()
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
			m.textinput.Reset()
			m.isLoading = true
			return m, tea.Batch(m.spinner.Tick, m.runShadowSimulation(action))
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textinput.Reset()
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
			m.textinput.Reset()
			m.isLoading = true
			return m, tea.Batch(m.spinner.Tick, m.runWhatIfQuery(change))
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textinput.Reset()
		return m, nil

	case "/approve":
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: "Approval noted. Proceeding with pending changes.",
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textinput.Reset()
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
					m.textinput.Reset()
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
					m.textinput.Reset()
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
		m.textinput.Reset()
		return m, nil

	case "/learn":
		if len(parts) < 3 {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: "Usage: `/learn <verb> <synonym>`\nExample: `/learn /test verify`",
				Time:    time.Now(),
			})
		} else {
			verb := parts[1]
			synonym := strings.Join(parts[2:], " ")

			// Use the shared taxonomy engine
			if perception.SharedTaxonomy != nil {
				if err := perception.SharedTaxonomy.LearnSynonym(verb, synonym); err != nil {
					m.history = append(m.history, Message{
						Role:    "assistant",
						Content: fmt.Sprintf("Failed to learn synonym: %v", err),
						Time:    time.Now(),
					})
				} else {
					m.history = append(m.history, Message{
						Role:    "assistant",
						Content: fmt.Sprintf("✓ Learned that **%q** implies **%s**.\nThis knowledge has been persisted to the graph and applied immediately.", synonym, verb),
						Time:    time.Now(),
					})
				}
			} else {
				m.history = append(m.history, Message{
					Role:    "assistant",
					Content: "Taxonomy engine is not available.",
					Time:    time.Now(),
				})
			}
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textinput.Reset()
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
					m.textinput.Reset()
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
					m.textinput.Reset()
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
		m.textinput.Reset()
		return m, nil

	default:
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: fmt.Sprintf("Unknown command: %s. Type `/help` for available commands.", cmd),
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textinput.Reset()
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
