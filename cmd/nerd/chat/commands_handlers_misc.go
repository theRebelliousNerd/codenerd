package chat

import (
	"fmt"
	"strings"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/transparency"

	tea "github.com/charmbracelet/bubbletea"
)

// handleQueryCommand handles the corresponding chat slash-command.
func (m Model) handleQueryCommand(input string, parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 2 {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: "Usage: `/query <predicate>`",
			Time:    time.Now(),
		})
	} else {
		predicate := parts[1]
		facts, err := m.kernel.Query(predicate)
		if err != nil {
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: fmt.Sprintf("Query failed: %v", err),
				Time:    time.Now(),
			})
		} else if len(facts) == 0 {
			m = m.addMessage(Message{
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

// handleExplainCommand handles the corresponding chat slash-command.
func (m Model) handleExplainCommand(input string, parts []string) (tea.Model, tea.Cmd) {
	// Full provenance via the Codeberg mangle-go fork's
	// DerivationRecorder (more accurate than /why for rules using
	// let-transforms or aggregations). On first use, enables
	// provenance + forces a re-evaluation so the recorder catches
	// the current pass; subsequent /explain calls reuse the
	// installed recorder.
	if len(parts) < 2 {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: "Usage: `/explain <ground-fact>` - Full-provenance proof tree (handles let-transforms + aggregations).\n\nThe fact must be ground (no variables), e.g.:\n- `/explain next_action(/generate_tool)`\n- `/explain permitted(/edit, \"main.go\")`\n\nOn first use this enables kernel provenance recording and forces a re-evaluation; following calls reuse the installed recorder.",
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil
	}
	goal := strings.Join(parts[1:], " ")
	if m.kernel == nil {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: fmt.Sprintf("## /explain %s\n\n_error: kernel not available in this session_\n", goal),
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil
	}
	// Enable provenance if it's off and force a re-eval so the
	// recorder catches the current store.
	newlyEnabled := false
	if !m.kernel.IsProvenanceEnabled() {
		m.kernel.EnableProvenance()
		newlyEnabled = true
		if evalErr := m.kernel.Evaluate(); evalErr != nil {
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: fmt.Sprintf("## /explain %s\n\n_error: forced re-evaluation failed: %v_\n", goal, evalErr),
				Time:    time.Now(),
			})
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			m.textarea.Reset()
			return m, nil
		}
	}
	proofs, err := m.kernel.Explain(goal, core.ExplainOptions{MaxProofs: 3, MaxDepth: 32})
	msg := explainCommandReply(goal, proofs, err)
	if newlyEnabled {
		msg.Content += "\n_provenance recording enabled for this session — disable with `/explain-off`._\n"
	}
	m = m.addMessage(msg)
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil

}

// handleTransparencyCommand handles the corresponding chat slash-command.
func (m Model) handleTransparencyCommand(input string, parts []string) (tea.Model, tea.Cmd) {
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

// handleRejectFindingCommand handles the corresponding chat slash-command.
func (m Model) handleRejectFindingCommand(input string, parts []string) (tea.Model, tea.Cmd) {
	// Reviewer feedback: mark a finding as false positive
	// Usage: /reject-finding <file>:<line> <reason>
	if len(parts) < 3 {
		m = m.addMessage(Message{
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
			m = m.addMessage(Message{
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

			m = m.addMessage(Message{
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

}

// handleAcceptFindingCommand handles the corresponding chat slash-command.
func (m Model) handleAcceptFindingCommand(input string, parts []string) (tea.Model, tea.Cmd) {
	// Reviewer feedback: confirm a finding is valid
	// Usage: /accept-finding <file>:<line>
	if len(parts) < 2 {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: "Usage: `/accept-finding <file>:<line>`\nExample: `/accept-finding internal/core/kernel.go:42`",
			Time:    time.Now(),
		})
	} else {
		location := parts[1]

		// Parse file:line
		colonIdx := strings.LastIndex(location, ":")
		if colonIdx == -1 {
			m = m.addMessage(Message{
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

			m = m.addMessage(Message{
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

}

// handlePromoteAtomCommand handles the corresponding chat slash-command.
func (m Model) handlePromoteAtomCommand(input string, parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 2 {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: "Usage: `/promote-atom <atom-id>`",
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
		if err := m.promptEvolver.PromoteAtom(atomID); err != nil {
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: fmt.Sprintf("Failed to promote atom: %v", err),
				Time:    time.Now(),
			})
		} else if err := m.refreshEvolvedAtomsInCompiler(); err != nil {
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: fmt.Sprintf("Atom `%s` promoted, but JIT refresh failed: %v", atomID, err),
				Time:    time.Now(),
			})
		} else {
			m = m.addMessage(Message{
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

}

// handleCmdContinue handles the corresponding chat slash-command.
func (m Model) handleCmdContinue(input string, parts []string) (tea.Model, tea.Cmd) {
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
}

// handleCmdUsage handles the corresponding chat slash-command.
func (m Model) handleCmdUsage(input string, parts []string) (tea.Model, tea.Cmd) {
	m.viewMode = UsageView
	m.usagePage.SetSize(m.width, m.height)
	m.usagePage.UpdateContent()
	return m, nil
}

// handleCmdClear handles the corresponding chat slash-command.
func (m Model) handleCmdClear(input string, parts []string) (tea.Model, tea.Cmd) {
	m.history = []Message{}
	m.viewport.SetContent("")
	m.textarea.Reset()
	// Save empty history
	m.saveSessionState()
	return m, nil
}

// handleCmdBrowser handles the /browser slash command — a plain status report, not a picker.
func (m Model) handleCmdBrowser(input string, parts []string) (tea.Model, tea.Cmd) {
	var content string
	if m.browserMgr == nil {
		content = "Browser automation is not running in this session — it starts on first use."
	} else {
		sessions := m.browserMgr.ListSessions()
		if len(sessions) == 0 {
			content = "Browser is running — no open sessions."
		} else {
			var sb strings.Builder
			defaultID := m.browserMgr.DefaultSessionID()
			sb.WriteString(fmt.Sprintf("Browser sessions (%d):\n", len(sessions)))
			for _, s := range sessions {
				markers := ""
				if s.ID == defaultID && defaultID != "" {
					markers += " [default]"
				}
				if s.Isolated {
					markers += " [isolated]"
				}
				status := s.Status
				if strings.TrimSpace(status) == "" {
					status = "unknown"
				}
				url := s.URL
				if url == "" {
					url = "-"
				} else {
					url = browserTruncate(url, 60)
				}
				title := s.Title
				if title == "" {
					title = "-"
				} else {
					title = browserTruncate(title, 40)
				}
				age := "unknown"
				if !s.LastActive.IsZero() {
					age = time.Since(s.LastActive).Truncate(time.Second).String() + " ago"
				}
				sb.WriteString(fmt.Sprintf("- %s%s status=%s url=%s title=\"%s\" %s\n", s.ID, markers, status, url, title, age))
			}
			content = strings.TrimSuffix(sb.String(), "\n")
		}
	}
	m = m.addMessage(Message{Role: "assistant", Content: content, Time: time.Now()})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}

func browserTruncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}