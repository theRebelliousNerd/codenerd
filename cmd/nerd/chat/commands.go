// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains command handling for the chat interface.
//
// File Index (modularized):
//
//	commands.go            - Main command dispatcher (handleCommand switch)
//	help_renderer.go       - Experience-level aware /help rendering
//	command_categories.go  - /help command registry (single source of truth)
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
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/core"
	nerdinit "codenerd/internal/init"
	"codenerd/internal/logging"
	"codenerd/internal/perception"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// COMMAND HANDLING
// =============================================================================
// handleCommand processes all /command inputs from the user.
// Commands are organized by category: session, config, shard, query, campaign.

func (m Model) handleCommand(input string) (tea.Model, tea.Cmd) {
	// Sanitize: strip null bytes and ANSI escapes before processing
	input = sanitizeCommandInput(input)
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return m, nil
	}
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
				if err := m.kernel.Retract("interrupt_requested"); err != nil {
					logging.Kernel("[commands] failed to retract interrupt_requested: %v", err)
				}
			}
			m.statusMessage = fmt.Sprintf("[%d/%d] %s", m.continuationStep, m.continuationTotal, next.Description)
			m.textarea.Reset()
			return m, tea.Batch(
				m.spinner.Tick,
				m.executeSubtask(next.ID, next.Description, next.ShardType),
			)
		}
		// No pending subtasks
		m = m.addMessage(Message{
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
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: "Kernel reset. Facts cleared, policy and schemas retained.",
				Time:    time.Now(),
			})
		} else {
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: "No kernel attached - nothing to reset.",
				Time:    time.Now(),
			})
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil

	case "/model":
		cfg := m.Config
		if cfg == nil {
			cfg = config.DefaultUserConfig()
		}
		activeProvider, _ := cfg.GetActiveProvider()
		available := ProviderModels[activeProvider]

		if len(parts) < 2 {
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
			sb.WriteString("\nTo change model, run: `/model <model-name>`")
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: sb.String(),
				Time:    time.Now(),
			})
		} else {
			newModel := parts[1]
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
					Content: fmt.Sprintf("Error: `%s` is not a recognized model for provider `%s`.\nUse `/model` to see list of valid models.", newModel, activeProvider),
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
						Content: fmt.Sprintf("✓ Switched active model for **%s** to: **%s**", activeProvider, newModel),
						Time:    time.Now(),
					})
				}
			}
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
		m = m.addMessage(Message{
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
			m = m.addMessage(Message{
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
			m = m.addMessage(Message{
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

	case "/help", "/h", "/?":
		// Progressive help system (experience-level aware).
		// Supports: /help, /help all, /help advanced, /help <command>
		arg := ""
		if len(parts) > 1 {
			arg = strings.Join(parts[1:], " ")
		}
		renderer := NewHelpRenderer(m.workspace)
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: renderer.RenderHelp(arg),
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil

	case "/status":
		// Show system status
		status := m.buildStatusReport()
		m = m.addMessage(Message{
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
		m = m.addMessage(Message{
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

				m = m.addMessage(Message{
					Role:    "assistant",
					Content: sb.String(),
					Time:    time.Now(),
				})
			} else if m.localDB != nil {
				atoms, err := m.localDB.GetKnowledgeAtomsByPrefix("session/")
				if err != nil || len(atoms) == 0 {
					m = m.addMessage(Message{
						Role:    "assistant",
						Content: "No persisted knowledge entries found.",
						Time:    time.Now(),
					})
				} else {
					sort.Slice(atoms, func(i, j int) bool {
						return atoms[i].CreatedAt.After(atoms[j].CreatedAt)
					})
					limit := min(len(atoms), 5)
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
					m = m.addMessage(Message{
						Role:    "assistant",
						Content: sb.String(),
						Time:    time.Now(),
					})
				}
			} else {
				m = m.addMessage(Message{
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
				m = m.addMessage(Message{
					Role:    "assistant",
					Content: "Usage: `/knowledge search <query>`",
					Time:    time.Now(),
				})
			} else if m.localDB == nil {
				m = m.addMessage(Message{
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
					m = m.addMessage(Message{
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
					m = m.addMessage(Message{
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
				m = m.addMessage(Message{
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
					m = m.addMessage(Message{
						Role:    "assistant",
						Content: fmt.Sprintf("Only %d knowledge entries available.", len(recent)),
						Time:    time.Now(),
					})
				} else {
					content := formatKnowledgeResults([]KnowledgeResult{recent[idx-1]})
					m = m.addMessage(Message{
						Role:    "assistant",
						Content: content,
						Time:    time.Now(),
					})
				}
			} else if m.localDB != nil {
				atoms, err := m.localDB.GetKnowledgeAtomsByPrefix("session/")
				if err != nil || len(atoms) == 0 {
					m = m.addMessage(Message{
						Role:    "assistant",
						Content: "No persisted knowledge entries found.",
						Time:    time.Now(),
					})
				} else {
					sort.Slice(atoms, func(i, j int) bool {
						return atoms[i].CreatedAt.After(atoms[j].CreatedAt)
					})
					if idx > len(atoms) {
						m = m.addMessage(Message{
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
						m = m.addMessage(Message{
							Role:    "assistant",
							Content: sb.String(),
							Time:    time.Now(),
						})
					}
				}
			} else {
				m = m.addMessage(Message{
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

		m = m.addMessage(Message{
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
			m = m.addMessage(Message{
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
		m = m.addMessage(Message{
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
			m = m.addMessage(Message{
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
		m = m.addMessage(Message{
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
			m = m.addMessage(Message{
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
		m = m.addMessage(Message{
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
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: "Workspace already initialized. Use `/init --force` to reinitialize.",
				Time:    time.Now(),
			})
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			m.textarea.Reset()
			return m, nil
		}

		m = m.addMessage(Message{
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
		m = m.addMessage(Message{
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
		m = m.addMessage(Message{
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
			m = m.addMessage(Message{
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
		m = m.addMessage(Message{
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
			m = m.addMessage(Message{
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
		m = m.addMessage(Message{
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
		return m.handleConfigCommand(input, parts)
	case "/embedding":
		return m.handleEmbeddingCommand(input, parts)
	case "/read":
		if len(parts) < 2 {
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: "Usage: `/read <path>`",
				Time:    time.Now(),
			})
		} else {
			path := parts[1]
			content, err := readFileContent(m.workspace, path, 16000)
			if err != nil {
				m = m.addMessage(Message{
					Role:    "assistant",
					Content: fmt.Sprintf("Failed to read file: %v", err),
					Time:    time.Now(),
				})
			} else {
				m = m.addMessage(Message{
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
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: "Usage: `/mkdir <path>`",
				Time:    time.Now(),
			})
		} else {
			path := parts[1]
			if err := makeDir(m.workspace, path); err != nil {
				m = m.addMessage(Message{
					Role:    "assistant",
					Content: fmt.Sprintf("Failed to create directory: %v", err),
					Time:    time.Now(),
				})
			} else {
				m = m.addMessage(Message{
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
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: "Usage: `/write <path> <content>`",
				Time:    time.Now(),
			})
		} else {
			path := parts[1]
			content := strings.Join(parts[2:], " ")
			if err := writeFileContent(m.workspace, path, content); err != nil {
				m = m.addMessage(Message{
					Role:    "assistant",
					Content: fmt.Sprintf("Failed to write file: %v", err),
					Time:    time.Now(),
				})
			} else {
				m = m.addMessage(Message{
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
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: "Usage: `/search <pattern>`",
				Time:    time.Now(),
			})
		} else {
			pattern := strings.Join(parts[1:], " ")
			matches, err := searchInFiles(m.workspace, pattern, 20)
			if err != nil {
				m = m.addMessage(Message{
					Role:    "assistant",
					Content: fmt.Sprintf("Search failed: %v", err),
					Time:    time.Now(),
				})
			} else if len(matches) == 0 {
				m = m.addMessage(Message{
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

	case "/patch":
		m.inputMode = InputModePatch
		m.pendingPatchLines = nil
		m.textarea.Placeholder = "Paste patch lines (type --END-- when done)..."
		m = m.addMessage(Message{
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
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: "Usage: `/edit <path>` - Opens file for inline editing",
				Time:    time.Now(),
			})
		} else {
			path := parts[1]
			content, err := readFileContent(m.workspace, path, 16000)
			if err != nil {
				m = m.addMessage(Message{
					Role:    "assistant",
					Content: fmt.Sprintf("Failed to read file for editing: %v", err),
					Time:    time.Now(),
				})
			} else {
				m = m.addMessage(Message{
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
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: "Usage: `/append <path> <content>`",
				Time:    time.Now(),
			})
		} else {
			path := parts[1]
			content := strings.Join(parts[2:], " ")
			if err := appendFileContent(m.workspace, path, content+"\n"); err != nil {
				m = m.addMessage(Message{
					Role:    "assistant",
					Content: fmt.Sprintf("Failed to append to file: %v", err),
					Time:    time.Now(),
				})
			} else {
				m = m.addMessage(Message{
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
		m.setInputMode(InputModeAgentWizard)
		m.agentWizard = &AgentWizardState{Step: 0} // Start at step 0 (Name)
		m.textarea.Placeholder = "Enter agent name (e.g., 'RustExpert')..."
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: "**Agent Creation Wizard**\n\nLet's define a new specialist agent.\n\n**Step 1:** What should we name this agent? (Alphanumeric, e.g., `RustExpert`, `SecurityAuditor`)",
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil

	case "/northstar", "/vision", "/spec":
		return m.handleNorthstarCommand(input, parts)
	case "/learn":
		return m.handleLearnCommand(input, parts)
	case "/agents":
		// List defined agents
		agents := m.loadType3Agents()
		if len(agents) == 0 {
			m = m.addMessage(Message{
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
			m = m.addMessage(Message{
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
		m = m.addMessage(Message{
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
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: "Usage: `/spawn <type> <task>`\n\nTypes: coder, researcher, reviewer, tester, or a defined agent name",
				Time:    time.Now(),
			})
		} else {
			shardType := parts[1]
			task := strings.Join(parts[2:], " ")
			m = m.addMessage(Message{
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
			m = m.addMessage(Message{
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
		m = m.addMessage(Message{
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
		return m.handleReviewCommand(input, parts)
	case "/security":
		target := "."
		if len(parts) > 1 {
			target = parts[1]
		}
		task := formatShardTask("/security", target, "", m.workspace)
		m = m.addMessage(Message{
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
		m = m.addMessage(Message{
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
		m = m.addMessage(Message{
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
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: "Usage: `/fix <issue description>`",
				Time:    time.Now(),
			})
		} else {
			target := strings.Join(parts[1:], " ")
			task := formatShardTask("/fix", target, "", m.workspace)
			m = m.addMessage(Message{
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
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: "Usage: `/refactor <target>`",
				Time:    time.Now(),
			})
		} else {
			target := strings.Join(parts[1:], " ")
			task := formatShardTask("/refactor", target, "", m.workspace)
			m = m.addMessage(Message{
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
		return m.handleQueryCommand(input, parts)
	case "/why":
		if len(parts) < 2 {
			m = m.addMessage(Message{
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

	case "/explain":
		return m.handleExplainCommand(input, parts)
	case "/explain-off":
		if m.kernel == nil {
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: "_error: kernel not available_\n",
				Time:    time.Now(),
			})
		} else {
			m.kernel.DisableProvenance()
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: "_Provenance recording disabled._\n",
				Time:    time.Now(),
			})
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
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

		m = m.addMessage(Message{
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
		m = m.addMessage(Message{
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
		return m.handleTransparencyCommand(input, parts)
	case "/shadow":
		if len(parts) < 2 {
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: "Usage: `/shadow <action>` - Run a shadow mode simulation",
				Time:    time.Now(),
			})
		} else {
			action := strings.Join(parts[1:], " ")
			m = m.addMessage(Message{
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
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: "Usage: `/whatif <change>` - Run a counterfactual query",
				Time:    time.Now(),
			})
		} else {
			change := strings.Join(parts[1:], " ")
			m = m.addMessage(Message{
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
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: "Approval noted. Proceeding with pending changes.",
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil

	case "/reject-finding":
		return m.handleRejectFindingCommand(input, parts)
	case "/accept-finding":
		return m.handleAcceptFindingCommand(input, parts)
	case "/review-accuracy":
		// Show accuracy report for the last review
		reviewID := "unknown"
		if m.lastShardResult != nil && m.lastShardResult.ShardType == "reviewer" {
			reviewID = fmt.Sprintf("review-%d-%d", m.lastShardResult.TurnNumber, m.lastShardResult.Timestamp.Unix())
		}
		report := m.shardMgr.GetReviewAccuracyReport(reviewID)
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: fmt.Sprintf("## Review Accuracy Report\n\n%s", report),
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil

	case "/campaign":
		return m.handleCampaignCommand(input, parts)
	case "/tool":
		return m.handleToolCommand(input, parts)
	case "/jit":
		// JIT Prompt Compiler inspector
		content := m.renderJITStatus()
		m = m.addMessage(Message{
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
		m = m.addMessage(Message{
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
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: "Prompt Evolution system not initialized.\n\nEnable it in config.",
				Time:    time.Now(),
			})
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			m.textarea.Reset()
			return m, nil
		}
		m = m.addMessage(Message{
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
		m = m.addMessage(Message{
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
		m = m.addMessage(Message{
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
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: content,
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil

	case "/promote-atom":
		return m.handlePromoteAtomCommand(input, parts)
	case "/reject-atom":
		if len(parts) < 2 {
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: "Usage: `/reject-atom <atom-id>`",
				Time:    time.Now(),
			})
		} else if m.promptEvolver == nil {
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: "Prompt Evolution system not initialized.",
				Time:    time.Now(),
			})
		} else {
			atomID := parts[1]
			if err := m.promptEvolver.RejectAtom(atomID); err != nil {
				m = m.addMessage(Message{
					Role:    "assistant",
					Content: fmt.Sprintf("Failed to reject atom: %v", err),
					Time:    time.Now(),
				})
			} else if err := m.refreshEvolvedAtomsInCompiler(); err != nil {
				m = m.addMessage(Message{
					Role:    "assistant",
					Content: fmt.Sprintf("Atom `%s` rejected, but JIT refresh failed: %v", atomID, err),
					Time:    time.Now(),
				})
			} else {
				m = m.addMessage(Message{
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
		m = m.addMessage(Message{
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

func setModelRecursive(client perception.LLMClient, model string) {
	if client == nil {
		return
	}
	if setter, ok := client.(interface{ SetModel(string) }); ok {
		setter.SetModel(model)
	}
	if sched, ok := client.(*core.ScheduledLLMCall); ok {
		setModelRecursive(sched.Client, model)
	}
	if tc, ok := client.(*perception.TracingLLMClient); ok {
		setModelRecursive(tc.GetUnderlying(), model)
	}
}

func getModelRecursive(client perception.LLMClient) string {
	if client == nil {
		return ""
	}
	if getter, ok := client.(interface{ GetModel() string }); ok {
		return getter.GetModel()
	}
	if sched, ok := client.(*core.ScheduledLLMCall); ok {
		return getModelRecursive(sched.Client)
	}
	if tc, ok := client.(*perception.TracingLLMClient); ok {
		return getModelRecursive(tc.GetUnderlying())
	}
	return ""
}
