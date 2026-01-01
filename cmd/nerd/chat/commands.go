// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains command handling for the chat interface.
//
// File Index (modularized):
//
//	commands.go            - Main command dispatcher (handleCommand switch)
//	commands_help.go       - Help text constants (helpCommandText)
//	commands_tools.go      - Tool/status helpers (buildStatusReport, handleCleanupToolsCommand)
//	commands_evolution.go  - Prompt Evolution helpers (renderEvolutionStats, runEvolutionCycle)
//
// Command Categories (within handleCommand switch):
//
//	Session:    /quit, /exit, /continue, /usage, /clear, /reset, /new-session, /sessions
//	Help:       /help, /status
//	Init:       /init, /scan, /refresh-docs, /scan-path, /scan-dir
//	Config:     /config, /embedding
//	Files:      /read, /mkdir, /write, /search, /patch, /edit, /append, /pick
//	Agents:     /define-agent, /northstar, /learn, /agents, /spawn, /ingest
//	Analysis:   /review, /security, /analyze, /test, /fix, /refactor
//	Campaigns:  /legislate, /clarify, /launchcampaign, /campaign
//	Query:      /query, /why, /logic, /glassbox, /transparency, /shadow, /whatif
//	Review:     /approve, /reject-finding, /accept-finding, /review-accuracy
//	Tools:      /tool, /jit, /cleanup-tools
//	Evolution:  /evolve, /evolution-stats, /evolved-atoms, /promote-atom, /reject-atom, /strategies
package chat

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"codenerd/cmd/nerd/ui"
	"codenerd/internal/campaign"
	"codenerd/internal/config"
	nerdinit "codenerd/internal/init"
	"codenerd/internal/perception"
	"codenerd/internal/transparency"

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

	case "/continue", "/resume":
		// Resume from paused continuation state
		if len(m.pendingSubtasks) > 0 {
			next := m.pendingSubtasks[0]
			m.pendingSubtasks = m.pendingSubtasks[1:]
			m.isLoading = true
			m.isInterrupted = false
			// Clear interrupt fact from kernel
			if m.kernel != nil {
				_ = m.kernel.Retract("interrupt_requested")
			}
			m.statusMessage = fmt.Sprintf("[%d/%d] %s", m.continuationStep, m.continuationTotal, next.Description)
			m.textarea.Reset()
			return m, tea.Batch(
				m.spinner.Tick,
				m.executeSubtask(next.ID, next.Description, next.ShardType),
			)
		}
		// No pending subtasks
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: "No pending tasks to continue. Start a new task.",
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil

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

	case "/reset":
		// POWER-USER-FEATURE: Reset kernel facts while keeping policy
		// This clears the working memory but preserves learned rules and schemas
		if m.kernel != nil {
			m.kernel.Reset()
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: "Kernel reset. Facts cleared, policy and schemas retained.",
				Time:    time.Now(),
			})
		} else {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: "No kernel attached - nothing to reset.",
				Time:    time.Now(),
			})
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
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

	case "/load-session":
		// Load a specific session by ID: /load-session <session-id>
		if len(parts) < 2 {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: "Usage: `/load-session <session-id>`\n\nUse `/sessions` to see available sessions.",
				Time:    time.Now(),
			})
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			m.textarea.Reset()
			return m, nil
		}
		sessionID := parts[1]
		return m.loadSelectedSession(sessionID)

	case "/help":
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: helpCommandText, // Defined in commands_help.go
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

	case "/reflection":
		content := m.renderReflectionStatus()
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: content,
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil

	case "/knowledge":
		if len(parts) == 1 {
			if len(m.knowledgeHistory) > 0 {
				limit := 5
				recent := make([]KnowledgeResult, 0, limit)
				for i := len(m.knowledgeHistory) - 1; i >= 0 && len(recent) < limit; i-- {
					recent = append(recent, m.knowledgeHistory[i])
				}

				var sb strings.Builder
				sb.WriteString("## Recent Knowledge Requests\n\n")
				for i, kr := range recent {
					query := kr.Query
					if strings.TrimSpace(query) == "" {
						query = "(no query)"
					}
					specialist := kr.Specialist
					if strings.TrimSpace(specialist) == "" {
						specialist = "specialist"
					}
					timestamp := kr.Timestamp
					timeLabel := "-"
					if !timestamp.IsZero() {
						timeLabel = timestamp.Format("15:04:05")
					}
					sb.WriteString(fmt.Sprintf("%d. [%s] %s — %s\n", i+1, specialist, query, timeLabel))
				}
				sb.WriteString("\nUse `/knowledge <n>` to view the full response.")

				m.history = append(m.history, Message{
					Role:    "assistant",
					Content: sb.String(),
					Time:    time.Now(),
				})
			} else if m.localDB != nil {
				atoms, err := m.localDB.GetKnowledgeAtomsByPrefix("session/")
				if err != nil || len(atoms) == 0 {
					m.history = append(m.history, Message{
						Role:    "assistant",
						Content: "No persisted knowledge entries found.",
						Time:    time.Now(),
					})
				} else {
					sort.Slice(atoms, func(i, j int) bool {
						return atoms[i].CreatedAt.After(atoms[j].CreatedAt)
					})
					limit := 5
					if len(atoms) < limit {
						limit = len(atoms)
					}
					var sb strings.Builder
					sb.WriteString("## Recent Knowledge Entries (Persisted)\n\n")
					for i := 0; i < limit; i++ {
						atom := atoms[i]
						concept := atom.Concept
						if strings.TrimSpace(concept) == "" {
							concept = "(unknown concept)"
						}
						timeLabel := atom.CreatedAt.Format("2006-01-02 15:04:05")
						sb.WriteString(fmt.Sprintf("%d. %s — %s\n", i+1, concept, timeLabel))
					}
					sb.WriteString("\nUse `/knowledge <n>` to view the full response.")
					m.history = append(m.history, Message{
						Role:    "assistant",
						Content: sb.String(),
						Time:    time.Now(),
					})
				}
			} else {
				m.history = append(m.history, Message{
					Role:    "assistant",
					Content: "No knowledge history or database available.",
					Time:    time.Now(),
				})
			}

			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			m.textarea.Reset()
			return m, nil
		}

		if parts[1] == "search" {
			if len(parts) < 3 {
				m.history = append(m.history, Message{
					Role:    "assistant",
					Content: "Usage: `/knowledge search <query>`",
					Time:    time.Now(),
				})
			} else if m.localDB == nil {
				m.history = append(m.history, Message{
					Role:    "assistant",
					Content: "No knowledge database available.",
					Time:    time.Now(),
				})
			} else {
				query := strings.Join(parts[2:], " ")
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				atoms, err := m.localDB.SearchKnowledgeAtomsSemantic(ctx, query, 5)
				if err != nil || len(atoms) == 0 {
					m.history = append(m.history, Message{
						Role:    "assistant",
						Content: "No matching knowledge entries found.",
						Time:    time.Now(),
					})
				} else {
					var sb strings.Builder
					sb.WriteString(fmt.Sprintf("## Knowledge Search Results (%d)\n\n", len(atoms)))
					for i, atom := range atoms {
						concept := atom.Concept
						if strings.TrimSpace(concept) == "" {
							concept = "(unknown concept)"
						}
						sb.WriteString(fmt.Sprintf("### %d. %s\n\n", i+1, concept))
						sb.WriteString(atom.Content)
						sb.WriteString("\n\n")
					}
					m.history = append(m.history, Message{
						Role:    "assistant",
						Content: strings.TrimSpace(sb.String()),
						Time:    time.Now(),
					})
				}
			}

			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			m.textarea.Reset()
			return m, nil
		}

		if idx, err := strconv.Atoi(parts[1]); err == nil {
			if idx < 1 {
				m.history = append(m.history, Message{
					Role:    "assistant",
					Content: "Usage: `/knowledge <n>` (n starts at 1).",
					Time:    time.Now(),
				})
			} else if len(m.knowledgeHistory) > 0 {
				recent := make([]KnowledgeResult, 0, len(m.knowledgeHistory))
				for i := len(m.knowledgeHistory) - 1; i >= 0; i-- {
					recent = append(recent, m.knowledgeHistory[i])
				}
				if idx > len(recent) {
					m.history = append(m.history, Message{
						Role:    "assistant",
						Content: fmt.Sprintf("Only %d knowledge entries available.", len(recent)),
						Time:    time.Now(),
					})
				} else {
					content := formatKnowledgeResults([]KnowledgeResult{recent[idx-1]})
					m.history = append(m.history, Message{
						Role:    "assistant",
						Content: content,
						Time:    time.Now(),
					})
				}
			} else if m.localDB != nil {
				atoms, err := m.localDB.GetKnowledgeAtomsByPrefix("session/")
				if err != nil || len(atoms) == 0 {
					m.history = append(m.history, Message{
						Role:    "assistant",
						Content: "No persisted knowledge entries found.",
						Time:    time.Now(),
					})
				} else {
					sort.Slice(atoms, func(i, j int) bool {
						return atoms[i].CreatedAt.After(atoms[j].CreatedAt)
					})
					if idx > len(atoms) {
						m.history = append(m.history, Message{
							Role:    "assistant",
							Content: fmt.Sprintf("Only %d persisted knowledge entries available.", len(atoms)),
							Time:    time.Now(),
						})
					} else {
						atom := atoms[idx-1]
						var sb strings.Builder
						sb.WriteString("## Persisted Knowledge Entry\n\n")
						sb.WriteString(fmt.Sprintf("**Concept:** %s\n\n", atom.Concept))
						sb.WriteString(fmt.Sprintf("**Created:** %s\n\n", atom.CreatedAt.Format(time.RFC3339)))
						sb.WriteString(atom.Content)
						m.history = append(m.history, Message{
							Role:    "assistant",
							Content: sb.String(),
							Time:    time.Now(),
						})
					}
				}
			} else {
				m.history = append(m.history, Message{
					Role:    "assistant",
					Content: "No knowledge history or database available.",
					Time:    time.Now(),
				})
			}

			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			m.textarea.Reset()
			return m, nil
		}

		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: "Usage: `/knowledge`, `/knowledge <n>`, or `/knowledge search <query>`",
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
		deep := false
		for _, part := range parts[1:] {
			if part == "--deep" || part == "-d" {
				deep = true
				break
			}
		}
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: "Scanning workspace...",
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		m.isLoading = true
		return m, tea.Batch(m.spinner.Tick, m.runScan(deep))

	case "/refresh-docs", "/scan-docs":
		// Refresh strategic knowledge by re-scanning and processing documentation
		// Uses Mangle tracking to only process new/changed docs
		force := false
		for _, part := range parts[1:] {
			if part == "--force" || part == "-f" {
				force = true
				break
			}
		}
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: "Scanning documentation for updates...\n\nThis will:\n- Discover new/changed docs\n- Use LLM to filter for relevance\n- Extract knowledge atoms incrementally\n- Update the strategic knowledge base",
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		m.isLoading = true
		return m, tea.Batch(m.spinner.Tick, m.runDocRefresh(force))

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
  /embedding reembed                    - Force re-embed all .nerd + internal DBs (vectors + prompt atoms)`,
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
						Content: "Re-embedding all databases (vectors + prompt atoms)... this may take a while.",
						Time:    time.Now(),
					})
					m.viewport.SetContent(m.renderHistory())
					m.viewport.GotoBottom()
					m.textarea.Reset()
					m.isLoading = true
					return m, tea.Batch(m.spinner.Tick, m.runReembedAllDBs())
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
				clarification, err := m.stageLearningCandidateFromFact(fact, criticManualLearnReason)
				if err != nil {
					return responseMsg(fmt.Sprintf("Learning candidate staging failed: %v", err))
				}
				return clarification
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

	case "/alignment", "/align":
		// Run on-demand alignment check against project vision
		subject := "Current session state"
		if len(parts) > 1 {
			subject = strings.Join(parts[1:], " ")
		}
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: "Running Northstar alignment check...",
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		m.isLoading = true
		return m, tea.Batch(m.spinner.Tick, m.runAlignmentCheck(subject))

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

	case "/ingest":
		if len(parts) < 3 {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: "Usage: `/ingest <agent> <path>`\n\nExample: `/ingest mangleexpert .claude/skills/mangle-programming/references`",
				Time:    time.Now(),
			})
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			m.textarea.Reset()
			return m, nil
		}

		agentName := parts[1]
		docPath := strings.Join(parts[2:], " ")
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: fmt.Sprintf("Ingesting documents into %s: %s", agentName, docPath),
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		m.isLoading = true
		return m, tea.Batch(m.spinner.Tick, m.ingestAgentDocs(agentName, docPath))

	case "/review":
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
			m.history = append(m.history, Message{
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
			Content: fmt.Sprintf("Running test task: %s (with specialist matching)", task),
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		m.isLoading = true
		// Use specialist-aware spawning for /test
		return m, tea.Batch(m.spinner.Tick, m.spawnShardWithSpecialists("/test", "tester", task, target))

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
				Content: fmt.Sprintf("Attempting to fix: %s (with specialist matching)", target),
				Time:    time.Now(),
			})
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			m.textarea.Reset()
			m.isLoading = true
			// Use specialist-aware spawning for /fix
			return m, tea.Batch(m.spinner.Tick, m.spawnShardWithSpecialists("/fix", "coder", task, target))
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
				Content: fmt.Sprintf("Refactoring: %s (with specialist matching)", target),
				Time:    time.Now(),
			})
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			m.textarea.Reset()
			m.isLoading = true
			// Use specialist-aware spawning for /refactor
			return m, tea.Batch(m.spinner.Tick, m.spawnShardWithSpecialists("/refactor", "coder", task, target))
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
				Content: "Usage: `/why <fact>` - Explains why a fact was derived\n\nExamples:\n- `/why next_action` - Explain why an action was chosen\n- `/why permitted` - Explain what's permitted\n- `/why user_intent` - Show how input was interpreted",
				Time:    time.Now(),
			})
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			m.textarea.Reset()
			return m, nil
		}
		fact := strings.Join(parts[1:], " ")
		m.isLoading = true
		m.statusMessage = "Tracing derivation..."
		return m, tea.Batch(m.spinner.Tick, m.fetchTraceForWhy(fact))

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

	case "/glassbox":
		// Glass Box debug mode - inline system visibility
		var response string
		if len(parts) > 1 {
			switch parts[1] {
			case "status":
				response = m.glassBoxStatus()
			case "verbose":
				response = m.toggleGlassBoxVerbose()
			default:
				// Try as category toggle
				response = m.toggleGlassBoxCategory(parts[1])
			}
		} else {
			// Toggle Glass Box mode
			response = m.toggleGlassBox()
		}
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: response,
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		// Start listening for events if enabled
		if m.glassBoxEnabled {
			return m, m.listenGlassBoxEvents()
		}
		return m, nil

	case "/transparency":
		// Toggle or set transparency mode
		var status string
		if m.transparencyMgr == nil {
			// Initialize on first use if not set up
			m.transparencyMgr = transparency.NewTransparencyManager(m.Config.Transparency)
		}

		if len(parts) > 1 {
			switch parts[1] {
			case "on":
				m.transparencyMgr.Enable()
				status = "Transparency mode **enabled**. You'll now see:\n" +
					"- Shard execution phases\n" +
					"- Safety gate explanations\n" +
					"- Verbose error context"
			case "off":
				m.transparencyMgr.Disable()
				status = "Transparency mode **disabled**."
			default:
				status = "Usage: `/transparency [on|off]`\n\nToggles visibility into codeNERD's internal operations."
			}
		} else {
			// Toggle
			newState := m.transparencyMgr.Toggle()
			if newState {
				status = "Transparency mode **enabled**."
			} else {
				status = "Transparency mode **disabled**."
			}
		}

		// Also show current status if enabled
		if m.transparencyMgr.IsEnabled() {
			status += "\n\n" + m.transparencyMgr.GetStatus()
		}

		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: status,
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
				Content: "Usage: `/campaign <start|assault|status|pause|resume|list> [args]`",
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
			case "assault":
				m.history = append(m.history, Message{
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

	case "/cleanup-tools":
		// Tool execution cleanup command
		content := m.handleCleanupToolsCommand(parts[1:])
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: content,
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil

	// =============================================================================
	// PROMPT EVOLUTION COMMANDS (System Prompt Learning)
	// =============================================================================

	case "/evolve":
		// Trigger manual evolution cycle
		if m.promptEvolver == nil {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: "Prompt Evolution system not initialized.\n\nEnable it in config.",
				Time:    time.Now(),
			})
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			m.textarea.Reset()
			return m, nil
		}
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: "Running evolution cycle...",
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.isLoading = true
		m.textarea.Reset()
		return m, tea.Batch(m.spinner.Tick, m.runEvolutionCycle())

	case "/evolution-stats":
		content := m.renderEvolutionStats()
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: content,
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil

	case "/evolved-atoms":
		content := m.renderEvolvedAtoms()
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: content,
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil

	case "/strategies":
		content := m.renderStrategies()
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: content,
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil

	case "/promote-atom":
		if len(parts) < 2 {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: "Usage: `/promote-atom <atom-id>`",
				Time:    time.Now(),
			})
		} else if m.promptEvolver == nil {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: "Prompt Evolution system not initialized.",
				Time:    time.Now(),
			})
		} else {
			atomID := parts[1]
			if err := m.promptEvolver.PromoteAtom(atomID); err != nil {
				m.history = append(m.history, Message{
					Role:    "assistant",
					Content: fmt.Sprintf("Failed to promote atom: %v", err),
					Time:    time.Now(),
				})
			} else {
				m.history = append(m.history, Message{
					Role:    "assistant",
					Content: fmt.Sprintf("Atom `%s` promoted to corpus.", atomID),
					Time:    time.Now(),
				})
			}
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil

	case "/reject-atom":
		if len(parts) < 2 {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: "Usage: `/reject-atom <atom-id>`",
				Time:    time.Now(),
			})
		} else if m.promptEvolver == nil {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: "Prompt Evolution system not initialized.",
				Time:    time.Now(),
			})
		} else {
			atomID := parts[1]
			if err := m.promptEvolver.RejectAtom(atomID); err != nil {
				m.history = append(m.history, Message{
					Role:    "assistant",
					Content: fmt.Sprintf("Failed to reject atom: %v", err),
					Time:    time.Now(),
				})
			} else {
				m.history = append(m.history, Message{
					Role:    "assistant",
					Content: fmt.Sprintf("Atom `%s` rejected.", atomID),
					Time:    time.Now(),
				})
			}
		}
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

