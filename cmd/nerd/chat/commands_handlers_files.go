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
	nerdinit "codenerd/internal/init"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleCmdReset(input string, parts []string) (tea.Model, tea.Cmd) {
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
}

// handleCmdModel handles the corresponding chat slash-command.
func (m Model) handleCmdModel(input string, parts []string) (tea.Model, tea.Cmd) {
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
}

// handleCmdNewSession handles the corresponding chat slash-command.
func (m Model) handleCmdNewSession(input string, parts []string) (tea.Model, tea.Cmd) {
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
}

// handleCmdSessions handles the corresponding chat slash-command.
func (m Model) handleCmdSessions(input string, parts []string) (tea.Model, tea.Cmd) {
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
}

// handleCmdLoadSession handles the corresponding chat slash-command.
func (m Model) handleCmdLoadSession(input string, parts []string) (tea.Model, tea.Cmd) {
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
}

// handleCmdHelp handles the corresponding chat slash-command.
func (m Model) handleCmdHelp(input string, parts []string) (tea.Model, tea.Cmd) {
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
}

// handleCmdStatus handles the corresponding chat slash-command.
func (m Model) handleCmdStatus(input string, parts []string) (tea.Model, tea.Cmd) {
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
}

// handleCmdReflection handles the corresponding chat slash-command.
func (m Model) handleCmdReflection(input string, parts []string) (tea.Model, tea.Cmd) {
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
}

// handleCmdKnowledge handles the corresponding chat slash-command.
func (m Model) handleCmdKnowledge(input string, parts []string) (tea.Model, tea.Cmd) {
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
}

// handleCmdLegislate handles the corresponding chat slash-command.
